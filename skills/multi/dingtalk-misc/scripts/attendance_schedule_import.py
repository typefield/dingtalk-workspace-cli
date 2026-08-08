#!/usr/bin/env python3
"""
考勤排班导入脚本

[AI Agent 强制门禁] 本脚本执行前必须先阅读：
   references/attendance-schedule.md

   排班工作流、参数校验、班次校验、回显确认等约束全部在
   attendance-schedule.md，禁止凭本脚本源码或 --help 自行组装命令。

职责：
  1. 二次校验考勤组类型（必须为 TURN 排班制）
  2. 二次校验班次 ID 在可用班次列表中
  3. 回显排班内容表格，等待用户确认
  4. 调用 dws attendance schedule import 执行排班
  5. 输出执行结果摘要

用法:
    python attendance_schedule_import.py \
        --group-id 123456 \
        --schedules '[{"userId":"u001","workDate":"2026-05-19","classId":789,"isRest":"N"}]' \
        --yes --format json
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime
from typing import Any

# 复用公共模块
from attendance_report_common import (
    run_dws,
    DwsCallError,
    extract_records,
    resolve_user_names,
    log,
    warn,
    error,
)
from _runtime import add_contract_flags, emit, run_main

DATE_FMT = "%Y-%m-%d"
DATETIME_FMT = "%Y-%m-%d %H:%M:%S"


class ScheduleError(Exception):
    """A typed, safe-to-render failure from the schedule preparation flow."""

    def __init__(self, error_type: str, subtype: str, message: str, *, details: Any = None) -> None:
        super().__init__(message)
        self.error_type = error_type
        self.subtype = subtype
        self.details = details

    def as_error(self) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "type": self.error_type,
            "subtype": self.subtype,
            "message": str(self),
        }
        if self.details is not None:
            payload["details"] = self.details
        return payload


def _validation(message: str, *, subtype: str = "invalid_schedule", details: Any = None) -> None:
    raise ScheduleError("validation", subtype, message, details=details)


# ─────────────────────────────────────────────────────────────────────────────
# 考勤组校验
# ─────────────────────────────────────────────────────────────────────────────

def _unwrap_group_vo(result: dict) -> dict:
    """从 group get 返回结构中提取 groupVO（type/name/classIds 等字段所在层）。

    group get 返回结构：{groupVO: {type, name, classIds, ...}, ...}
    filtered-get 返回结构可能直接是扁平的 {type, name, memberUsers, ...}
    """
    if not isinstance(result, dict):
        return result
    group_vo = result.get("groupVO")
    if isinstance(group_vo, dict) and group_vo.get("type"):
        return group_vo
    # 如果顶层已经有 type 字段，说明是扁平结构，直接返回
    if result.get("type"):
        return result
    # 兜底：尝试从所有 dict 类型的值中找包含 type 字段的
    for value in result.values():
        if isinstance(value, dict) and value.get("type"):
            return value
    return result


def validate_group_is_turn(group_id: int) -> dict:
    """校验考勤组存在且类型为 TURN（排班制），返回考勤组信息（groupVO 层级）。"""
    log(f"🔍 校验考勤组 {group_id} ...")

    # 优先用 group get 获取完整信息（含绑定班次列表）
    try:
        result = run_dws([
            "attendance", "group", "get",
            "--group-id", str(group_id),
        ])
    except DwsCallError:
        # 降级使用 filtered-get
        try:
            result = run_dws([
                "attendance", "group", "filtered-get",
                "--group-id", str(group_id),
            ])
        except DwsCallError as exc:
            raise ScheduleError(
                "api", "group_lookup_failed", f"查询考勤组失败：{exc}",
                details={"group_id": group_id},
            ) from exc

    if not result or not isinstance(result, dict):
        _validation(
            f"考勤组 {group_id} 不存在或返回数据异常",
            subtype="group_not_found_or_unreadable", details={"group_id": group_id},
        )

    # 关键：从 groupVO 中提取 type/name 等字段
    group_vo = _unwrap_group_vo(result)
    group_type = group_vo.get("type", "")
    group_name = group_vo.get("name", f"ID:{group_id}")

    if not group_type:
        raise ScheduleError(
            "api", "group_projection_unknown",
            f"未能从考勤组 {group_id} 返回数据中识别出类型字段",
            details={"group_id": group_id, "top_level_keys": sorted(result.keys())},
        )

    if group_type != "TURN":
        type_label = {"FIXED": "固定班制", "NONE": "自由工时"}.get(group_type, group_type)
        _validation(
            f"考勤组「{group_name}」类型为 {type_label}，不是排班制（TURN），无法执行排班操作",
            subtype="group_not_turn", details={"group_id": group_id, "group_type": group_type},
        )

    log(f"✅ 考勤组「{group_name}」确认为排班制")
    return group_vo


# ─────────────────────────────────────────────────────────────────────────────
# 班次校验
# ─────────────────────────────────────────────────────────────────────────────

def extract_group_bound_classes(group_info: dict) -> set[int]:
    """从考勤组详情中提取绑定的班次 ID 集合。

    兼容多种字段结构：
      - classIds: [int]                    — 班次 ID 数组
      - classes / selectedClass: [dict]    — 班次对象数组 (含 id/classId)
      - shiftVOList: [dict]                — 排班制特有，含 shiftSetting.shiftId
      - classNameIdMap: {name: id}         — 名称到 ID 映射
    """

    def _extract_from_obj(obj: dict) -> set[int]:
        """从单个 dict 层级中提取班次 ID。"""
        ids: set[int] = set()

        # 方式1: classIds / shiftIds 数组（最常见）
        for key in ("classIds", "shiftIds", "classIdList"):
            ids_list = obj.get(key)
            if isinstance(ids_list, list):
                for item in ids_list:
                    try:
                        ids.add(int(item))
                    except (ValueError, TypeError):
                        pass

        # 方式2: classes / selectedClass 对象数组
        for key in ("classes", "selectedClass"):
            classes = obj.get(key)
            if isinstance(classes, list):
                for item in classes:
                    if isinstance(item, dict):
                        class_id = item.get("id") or item.get("classId")
                        if class_id is not None:
                            ids.add(int(class_id))
                    elif isinstance(item, (int, str)):
                        try:
                            ids.add(int(item))
                        except (ValueError, TypeError):
                            pass

        # 方式3: shiftVOList — 排班制考勤组特有字段
        shift_vo_list = obj.get("shiftVOList")
        if isinstance(shift_vo_list, list):
            for shift_vo in shift_vo_list:
                if not isinstance(shift_vo, dict):
                    continue
                # shiftSetting.shiftId
                shift_setting = shift_vo.get("shiftSetting")
                if isinstance(shift_setting, dict):
                    shift_id = shift_setting.get("shiftId") or shift_setting.get("classId")
                    if shift_id is not None:
                        ids.add(int(shift_id))
                # 直接在 shiftVO 层级的 id/shiftId/classId
                for id_key in ("id", "shiftId", "classId"):
                    val = shift_vo.get(id_key)
                    if val is not None:
                        try:
                            ids.add(int(val))
                        except (ValueError, TypeError):
                            pass

        # 方式4: classNameIdMap {name: id}
        class_map = obj.get("classNameIdMap")
        if isinstance(class_map, dict):
            for _, class_id in class_map.items():
                try:
                    ids.add(int(class_id))
                except (ValueError, TypeError):
                    pass

        return ids

    # 优先从 groupVO 提取（group get 返回结构），兼容顶层扁平结构
    bound_ids: set[int] = set()

    group_vo = group_info.get("groupVO")
    if isinstance(group_vo, dict):
        bound_ids.update(_extract_from_obj(group_vo))

    # 同时从顶层提取（兼容 filtered-get 或已解包的结构）
    bound_ids.update(_extract_from_obj(group_info))

    return bound_ids


def extract_group_class_names(group_info: dict) -> dict[int, str]:
    """Read class labels from the reviewed group's own configuration.

    A global class search can expose classes belonging to other attendance
    groups.  It is therefore a fallback only when the group response itself
    contains no usable class binding.
    """
    names: dict[int, str] = {}
    candidates = [group_info]
    nested = group_info.get("groupVO") if isinstance(group_info, dict) else None
    if isinstance(nested, dict):
        candidates.append(nested)
    for candidate in candidates:
        for key in ("classes", "selectedClass"):
            values = candidate.get(key)
            if not isinstance(values, list):
                continue
            for value in values:
                if not isinstance(value, dict):
                    continue
                raw_id = value.get("id") or value.get("classId") or value.get("shiftId")
                label = value.get("name") or value.get("className") or value.get("shiftName")
                try:
                    if raw_id is not None and isinstance(label, str) and label:
                        names[int(raw_id)] = label
                except (TypeError, ValueError):
                    continue
        shifts = candidate.get("shiftVOList")
        if isinstance(shifts, list):
            for shift in shifts:
                if not isinstance(shift, dict):
                    continue
                settings = shift.get("shiftSetting")
                settings = settings if isinstance(settings, dict) else shift
                raw_id = settings.get("shiftId") or settings.get("classId") or shift.get("id")
                label = settings.get("shiftName") or settings.get("className") or settings.get("name")
                try:
                    if raw_id is not None and isinstance(label, str) and label:
                        names[int(raw_id)] = label
                except (TypeError, ValueError):
                    continue
        mapping = candidate.get("classNameIdMap")
        if isinstance(mapping, dict):
            for label, raw_id in mapping.items():
                try:
                    if isinstance(label, str) and label:
                        names[int(raw_id)] = label
                except (TypeError, ValueError):
                    continue
    return names


def fetch_all_classes() -> dict[int, str]:
    """获取全局所有班次，返回 {classId: className}，用于 ID→名称映射。"""
    log("🔍 获取班次名称映射 ...")
    all_classes: dict[int, str] = {}
    page_index = 1
    page_size = 200

    while True:
        try:
            result = run_dws([
                "attendance", "class", "search",
                "--page", str(page_index),
                "--limit", str(page_size),
            ])
        except DwsCallError as exc:
            raise ScheduleError("api", "class_lookup_failed", f"查询班次列表失败：{exc}") from exc

        records = extract_records(result) if result else []
        if not records:
            break

        for record in records:
            class_id = record.get("id") or record.get("classId")
            class_name = record.get("name") or record.get("className") or str(class_id)
            if class_id is not None:
                all_classes[int(class_id)] = class_name

        if len(records) < page_size:
            break
        page_index += 1

    log(f"✅ 获取到 {len(all_classes)} 个班次名称")
    return all_classes


def validate_class_ids(
    schedules: list[dict],
    group_bound_class_ids: set[int],
    all_classes: dict[int, str],
    group_name: str,
) -> None:
    """校验排班记录中的 classId 都在该考勤组绑定的班次中。

    如果考勤组未提取到绑定班次列表（可能是接口字段差异），
    则降级为全局班次校验并输出警告。
    """
    # 如果两个来源都无法获取到班次信息，跳过校验（排班导入接口本身有服务端校验）
    no_bound = len(group_bound_class_ids) == 0
    no_global = len(all_classes) == 0

    if no_bound and no_global:
        warn(f"无法获取考勤组绑定班次和全局班次列表，跳过班次校验（将依赖服务端校验）")
        return

    use_global_fallback = no_bound
    if use_global_fallback:
        warn(f"未能从考勤组「{group_name}」详情中提取绑定班次列表，降级为全局班次校验")
        check_set = set(all_classes.keys())
    else:
        check_set = group_bound_class_ids

    invalid_class_ids: set[int] = set()

    for schedule in schedules:
        is_rest = str(schedule.get("isRest", "N")).upper()
        if is_rest == "Y":
            continue
        class_id = int(schedule.get("classId", 0))
        if class_id != 0 and class_id not in check_set:
            invalid_class_ids.add(class_id)

    if invalid_class_ids:
        invalid_names = [all_classes.get(cid, f"ID:{cid}") for cid in sorted(invalid_class_ids)]
        if use_global_fallback:
            error(f"以下班次不在可用班次列表中: {', '.join(invalid_names)}")
        else:
            error(f"以下班次不属于考勤组「{group_name}」: {', '.join(invalid_names)}")
        log(f"「{group_name}」可用班次:")
        available_ids = check_set if not use_global_fallback else set(all_classes.keys())
        for cid in sorted(available_ids):
            cname = all_classes.get(cid, f"ID:{cid}")
            log(f"  - {cname} (ID: {cid})")
        _validation(
            (f"以下班次不在可用班次列表中: {', '.join(invalid_names)}" if use_global_fallback
             else f"以下班次不属于考勤组「{group_name}」: {', '.join(invalid_names)}"),
            subtype="class_not_available",
            details={"group_name": group_name, "invalid_class_ids": sorted(invalid_class_ids)},
        )


# ─────────────────────────────────────────────────────────────────────────────
# 日期格式标准化
# ─────────────────────────────────────────────────────────────────────────────

def normalize_work_date(work_date: Any) -> str:
    """将 workDate 统一转换为 yyyy-MM-dd HH:mm:ss 格式。"""
    if isinstance(work_date, (int, float)):
        timestamp = work_date / 1000 if work_date > 1e12 else work_date
        return datetime.fromtimestamp(timestamp).strftime(DATETIME_FMT)

    date_str = str(work_date).strip()

    for fmt in (DATETIME_FMT, DATE_FMT):
        try:
            parsed = datetime.strptime(date_str, fmt)
            return parsed.strftime(DATETIME_FMT)
        except ValueError:
            continue

    raise ValueError(f"无法解析日期格式: {work_date!r}，请使用 YYYY-MM-DD 格式")


# ─────────────────────────────────────────────────────────────────────────────
# 回显排班内容
# ─────────────────────────────────────────────────────────────────────────────

def schedule_preview(
    group_name: str,
    group_id: int,
    schedules: list[dict],
    available_classes: dict[int, str],
    user_names: dict[str, str],
) -> dict[str, Any]:
    """Build the reviewable schedule, including stable identifiers for Agents."""
    rows: list[dict[str, Any]] = []
    for schedule in sorted(schedules, key=lambda item: (str(item.get("userId", "")), str(item.get("workDate", "")))):
        user_id = str(schedule["userId"])
        class_id = int(schedule["classId"])
        is_rest = str(schedule["isRest"]).upper()
        rows.append({
            "user_id": user_id,
            "user_name": user_names.get(user_id, user_id),
            "work_date": str(schedule["workDate"])[:10],
            "class_id": class_id,
            "class_name": "休息" if is_rest == "Y" else available_classes.get(class_id, f"未知班次(ID:{class_id})"),
            "is_rest": is_rest == "Y",
        })
    dates = sorted({row["work_date"] for row in rows})
    return {
        "group": {"id": group_id, "name": group_name},
        "date_range": {"start": dates[0], "end": dates[-1]} if dates else None,
        "records": rows,
        "record_count": len(rows),
        "user_count": len({row["user_id"] for row in rows}),
    }


def print_schedule_preview(preview: dict[str, Any]) -> None:
    """Print a human review table; JSON callers receive the same content as data."""
    print("\n📋 排班确认")
    group = preview["group"]
    print(f"\n考勤组: {group['name']} (ID: {group['id']})")
    date_range = preview.get("date_range")
    if isinstance(date_range, dict):
        print(f"排班日期: {date_range['start']} ~ {date_range['end']}")

    print(f"\n{'员工姓名':<12} {'日期':<14} {'班次':<16} {'是否排休':<8}")
    print("-" * 54)

    for row in preview["records"]:
        print(f"{row['user_name']:<12} {row['work_date']:<14} {row['class_name']:<16} {'是' if row['is_rest'] else '否':<8}")

    print(f"\n共 {preview['record_count']} 条排班记录")


# ─────────────────────────────────────────────────────────────────────────────
# 执行排班
# ─────────────────────────────────────────────────────────────────────────────

def execute_schedule_import(group_id: int, schedules: list[dict]) -> Any:
    """Submit the write request without claiming that every row reached its final state."""
    log(f"🚀 正在执行排班导入 ({len(schedules)} 条记录) ...")

    schedules_json = json.dumps(schedules, ensure_ascii=False)

    try:
        result = run_dws([
            "attendance", "schedule", "import",
            "--groupId", str(group_id),
            "--scheduleVOS", schedules_json,
            "--yes",
        ])
    except DwsCallError as exc:
        error(f"排班导入请求没有得到可确认的成功响应: {exc}")
        raise ScheduleError(
            "authorization" if exc.is_permission_error else "api",
            "schedule_import_unconfirmed",
            "排班导入请求未得到可确认的成功响应；请先核查目标人员当日排班，勿直接重试。",
            details={"group_id": group_id, "records": len(schedules), "execution_state": "unknown"},
        ) from exc

    log("✅ 排班导入请求已成功返回；实际排班终态仍需查询确认")
    return result


# ─────────────────────────────────────────────────────────────────────────────
# 主流程
# ─────────────────────────────────────────────────────────────────────────────

def _parse_schedules(raw: str) -> list[dict[str, Any]]:
    try:
        schedules = json.loads(raw)
    except json.JSONDecodeError as exc:
        _validation(f"--schedules JSON 格式错误：{exc}", subtype="invalid_schedule_json")
    if not isinstance(schedules, list) or not schedules:
        _validation("--schedules 必须是非空 JSON 数组", subtype="invalid_schedule_list")

    required_fields = ("userId", "workDate", "classId", "isRest")
    normalized: list[dict[str, Any]] = []
    for index, schedule in enumerate(schedules):
        if not isinstance(schedule, dict):
            _validation(f"schedule[{index}] 必须是对象", subtype="invalid_schedule_entry", details={"index": index})
        missing = [field for field in required_fields if field not in schedule]
        if missing:
            _validation(
                f"schedule[{index}] 缺少必填字段：{', '.join(missing)}",
                subtype="missing_schedule_fields", details={"index": index, "missing": missing},
            )
        user_id = schedule["userId"]
        if not isinstance(user_id, str) or not user_id.strip():
            _validation(f"schedule[{index}].userId 必须是非空字符串", subtype="invalid_user_id", details={"index": index})
        try:
            class_id = int(schedule["classId"])
        except (TypeError, ValueError):
            _validation(f"schedule[{index}].classId 必须是整数", subtype="invalid_class_id", details={"index": index})
        is_rest = schedule["isRest"]
        if not isinstance(is_rest, str) or is_rest.upper() not in {"Y", "N"}:
            _validation(f"schedule[{index}].isRest 必须是 Y 或 N", subtype="invalid_is_rest", details={"index": index})
        try:
            work_date = normalize_work_date(schedule["workDate"])
        except (TypeError, ValueError) as exc:
            _validation(f"schedule[{index}] 日期格式错误：{exc}", subtype="invalid_work_date", details={"index": index})
        normalized.append({"userId": user_id.strip(), "workDate": work_date, "classId": class_id, "isRest": is_rest.upper()})
    return normalized


def main() -> int:
    """Run the reviewed write path and return one machine-readable result."""
    parser = argparse.ArgumentParser(
        description="考勤排班导入（含校验、预览、受确认写入）",
        epilog="执行前必须阅读 attendance-schedule.md；--dry-run 允许只读校验，绝不执行排班写入。",
    )
    parser.add_argument("--group-id", required=True, type=int, help="考勤组 ID（必须为排班制 TURN）")
    parser.add_argument("--schedules", required=True, help="排班记录 JSON 数组，含 userId/workDate/classId/isRest")
    parser.add_argument("--yes", action="store_true", help="用户已审阅预览并明确确认；缺失时不发送写入")
    parser.add_argument("--confirm", action="store_true", help=argparse.SUPPRESS)
    add_contract_flags(parser, default="json")
    args = parser.parse_args()

    try:
        schedules = _parse_schedules(args.schedules)
        group_info = validate_group_is_turn(args.group_id)
        group_name = str(group_info.get("name") or f"ID:{args.group_id}")
        user_ids = sorted({schedule["userId"] for schedule in schedules})
        user_names = resolve_user_names(user_ids)
        group_bound_class_ids = extract_group_bound_classes(group_info)
        all_classes = extract_group_class_names(group_info)
        if not group_bound_class_ids:
            # The group response has no binding evidence.  A global lookup is
            # only a last-resort label/validation source and is surfaced in
            # the validation warning below.
            all_classes = fetch_all_classes()
        validate_class_ids(schedules, group_bound_class_ids, all_classes, group_name)
        preview = schedule_preview(group_name, args.group_id, schedules, all_classes, user_names)
        if args.format == "text":
            print_schedule_preview(preview)

        if args.dry_run:
            return emit(
                fmt=args.format,
                outcome="success",
                data={"preview": preview, "remote_reads": "performed_for_validation", "write": "not_sent"},
                dry_run=True,
                text="[dry-run] 已完成只读校验与排班预览；未执行排班写入。",
            )
        if not (args.yes or args.confirm):
            return emit(
                fmt=args.format,
                outcome="failure",
                data={"preview": preview, "write": "not_sent"},
                error={
                    "type": "confirmation_required",
                    "subtype": "schedule_preview_requires_yes",
                    "message": "排班尚未执行；请向用户展示 preview 后，获得明确确认再使用 --yes。",
                },
                text="排班尚未执行：请向用户展示预览并获得确认后使用 --yes。",
            )

        execute_schedule_import(args.group_id, schedules)
        return emit(
            fmt=args.format,
            outcome="success",
            data={
                "preview": preview,
                "request": {"state": "accepted", "operation": "attendance_schedule_import"},
                "verification": {
                    "state": "not_verified",
                    "reason": "服务端未返回逐条终态；请使用 attendance schedule get 核查目标人员和日期。",
                },
            },
            text="排班导入请求已受理；请查询目标人员和日期核查最终排班。",
        )
    except ScheduleError as exc:
        return emit(fmt=args.format, outcome="failure", error=exc.as_error(), text=f"错误：{exc}")


if __name__ == "__main__":
    raise SystemExit(run_main(main, default_format="json"))
