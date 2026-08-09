#!/usr/bin/env python3
"""
假期余额 Excel 导出脚本。

[AI Agent 强制门禁] 调用本脚本前必须先阅读：
  references/attendance-vacation.md

本脚本负责：
  1. 通过 dws attendance vacation types 获取假期规则列表，用于确定列顺序
  2. 通过 dws attendance vacation balance 查询所有假期规则余额
  3. 通过 dws contact user get 解析姓名、部门等基础信息
  4. 生成横向宽表 Excel：每人一行，假期规则为动态列
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import datetime
from typing import Any

import attendance_report_common as cmn
from _runtime import add_contract_flags, emit, run_main

MAX_USERS_PER_BALANCE_BATCH = 20
BASE_HEADERS = ["姓名", "部门", "入职时间", "首次工作时间"]
USER_ID_KEYS = (
    "userId", "userid", "targetUserId", "targetUserID", "staffId", "staffID",
    "employeeId", "empId", "dingUserId",
)
LEAVE_CODE_KEYS = (
    "leaveCode", "leaveTypeCode", "quotaCode", "vacationCode", "bizType",
    "bizCode", "code", "id",
)
LEAVE_NAME_KEYS = (
    "leaveName", "leaveTypeName", "quotaName", "vacationName", "name",
    "title", "ruleName",
)
BALANCE_KEYS = (
    "balance", "balanceQuota", "remain", "remainQuota", "remainDuration",
    "restQuota", "availableBalance", "availableQuota", "quotaNumPerDay",
    "quotaNumPerHour", "quotaNum", "quota", "value", "leaveBalance",
    "leftQuota", "leftBalance",
)
MESSAGE_KEYS = ("message", "msg", "reason", "errorMessage", "errorMsg")
SOURCE_KEYS = ("source", "leaveSource", "ruleSource", "dataSource")
UNIT_KEYS = (
    "leaveViewUnit", "viewUnit", "displayUnit", "unit", "quotaUnit",
    "durationUnit", "timeUnit", "balanceUnit", "leaveUnit",
)
UNIT_LABELS = {
    "day": "天",
    "days": "天",
    "percent_day": "天",
    "hour": "小时",
    "hours": "小时",
    "minute": "分钟",
    "minutes": "分钟",
}
ENTRY_TIME_KEYS = (
    "entryTime", "entryDate", "hireDate", "joinDate", "employmentDate", "入职时间",
)
FIRST_WORK_TIME_KEYS = (
    "firstWorkTime", "firstWorkingTime", "firstWorkDate", "首次工作时间",
)
UNLIMITED_KEYS = (
    "unlimited", "isUnlimited", "unLimit", "unlimitedBalance", "notLimit",
)
NOT_APPLICABLE_KEYS = (
    "notApplicable", "notApply", "isNotApplicable", "invalid", "disable", "disabled",
)
VISIBLE_KEYS = ("visible", "visiable", "visibility", "isVisible", "isVisiable")
NO_BALANCE_MESSAGES = ("假期类型没有余额", "没有余额", "未设置假期余额")
NOT_APPLICABLE_MESSAGES = (
    "员工未设置首次参加工作时间",
    "未设置首次参加工作时间",
    "员工未设置入职时间",
    "未设置入职时间",
)
EXTERNAL_SOURCE = "external"
EXTERNAL_BALANCE_UNAVAILABLE_MESSAGE = "外部规则暂无余额，需通过接口初始化更新余额"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "导出假期余额 Excel。AI Agent 必须先读 "
            "references/attendance-vacation.md 再调用本脚本。"
        ),
    )
    parser.add_argument("--users", required=True, help="userId 或 deptId 列表，逗号分隔")
    parser.add_argument("--leave-keywords", default="", help="按假期名称关键词筛选列，逗号分隔；默认导出全部")
    parser.add_argument("--out", default="", help="输出 xlsx 文件名；不传则自动生成")
    parser.add_argument("--inspect", action="store_true", help="打印首条假期类型和余额原始结构到 stderr")
    add_contract_flags(parser)
    return parser.parse_args()


def first_nonempty(record: dict[str, Any], keys: tuple[str, ...]) -> Any:
    for key in keys:
        if key in record and record[key] not in (None, ""):
            return record[key]
    return None


def recursively_collect_dicts(payload: Any) -> list[dict[str, Any]]:
    if isinstance(payload, list):
        records: list[dict[str, Any]] = []
        for item in payload:
            records.extend(recursively_collect_dicts(item))
        return records
    if isinstance(payload, dict):
        if looks_like_business_record(payload):
            return [payload]
        direct_records = cmn.extract_records(payload)
        if direct_records:
            return direct_records
        records = []
        for value in payload.values():
            records.extend(recursively_collect_dicts(value))
        return records
    return []


def looks_like_business_record(record: dict[str, Any]) -> bool:
    candidate_key_groups = (
        USER_ID_KEYS,
        LEAVE_CODE_KEYS,
        LEAVE_NAME_KEYS,
        BALANCE_KEYS,
        ENTRY_TIME_KEYS,
        FIRST_WORK_TIME_KEYS,
    )
    return any(first_nonempty(record, keys) is not None for keys in candidate_key_groups)


def is_truthy_flag(value: Any) -> bool:
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return value != 0
    if isinstance(value, str):
        return value.strip().lower() in {"true", "1", "y", "yes", "是", "visible"}
    return False


def is_falsey_flag(value: Any) -> bool:
    if isinstance(value, bool):
        return not value
    if isinstance(value, (int, float)):
        return value == 0
    if isinstance(value, str):
        return value.strip().lower() in {"false", "0", "n", "no", "否", "invisible", "not_visible"}
    return False


def is_no_balance_message(message: Any) -> bool:
    return any(keyword in str(message) for keyword in NO_BALANCE_MESSAGES)


def is_not_applicable_message(message: Any) -> bool:
    return any(keyword in str(message) for keyword in NOT_APPLICABLE_MESSAGES)


def is_external_leave_type(leave_type: dict[str, str]) -> bool:
    return leave_type.get("source", "").strip().lower() == EXTERNAL_SOURCE


def normalize_leave_unit(value: Any) -> str:
    if value in (None, ""):
        return ""
    unit = str(value).strip()
    if not unit:
        return ""
    return UNIT_LABELS.get(unit.lower(), unit)


def format_date(value: Any) -> str:
    if value in (None, ""):
        return "未设置"
    if isinstance(value, (int, float)):
        timestamp = value / 1000 if value > 10_000_000_000 else value
        try:
            return datetime.fromtimestamp(timestamp).strftime("%Y-%m-%d")
        except (OverflowError, OSError, ValueError):
            return str(value)
    if isinstance(value, str):
        stripped = value.strip()
        if not stripped:
            return "未设置"
        for fmt in ("%Y-%m-%d %H:%M:%S", "%Y-%m-%dT%H:%M:%S", "%Y-%m-%d"):
            try:
                return datetime.strptime(stripped[:19], fmt).strftime("%Y-%m-%d")
            except ValueError:
                continue
        return stripped[:10] if len(stripped) >= 10 else stripped
    return str(value)


def format_balance_value(record: dict[str, Any]) -> Any:
    visible = first_nonempty(record, VISIBLE_KEYS)
    if visible is not None and is_falsey_flag(visible):
        return "不适用"

    message = first_nonempty(record, MESSAGE_KEYS)
    if message and is_no_balance_message(message):
        return "不限制余额"
    if message and is_not_applicable_message(message):
        return "不适用"

    if "hideQuota" in record and is_truthy_flag(record["hideQuota"]):
        return "不适用"

    for key in UNLIMITED_KEYS:
        if key in record and is_truthy_flag(record[key]):
            return "不限制余额"
    for key in NOT_APPLICABLE_KEYS:
        if key in record and is_truthy_flag(record[key]):
            return "不适用"

    value = first_nonempty(record, BALANCE_KEYS)
    if value in (None, ""):
        status = first_nonempty(record, ("status", "state", "balanceStatus", *MESSAGE_KEYS))
        return status or "不适用"

    if isinstance(value, str):
        stripped = value.strip()
        if stripped in {"UNLIMITED", "Unlimited", "不限", "不限制"}:
            return "不限制余额"
        if stripped in {"N/A", "NA", "NOT_APPLICABLE", "不适用"}:
            return "不适用"
        try:
            value = float(stripped)
        except ValueError:
            return stripped

    if isinstance(value, (int, float)):
        rounded = round(float(value), 2)
        return int(rounded) if rounded == int(rounded) else rounded
    return value


def normalize_leave_types(payload: Any) -> list[dict[str, str]]:
    raw_records = recursively_collect_dicts(payload)
    leave_types: list[dict[str, str]] = []
    seen: set[str] = set()
    for record in raw_records:
        code = first_nonempty(record, LEAVE_CODE_KEYS)
        name = first_nonempty(record, LEAVE_NAME_KEYS)
        if not code and not name:
            continue
        stable_key = str(code or name)
        if stable_key in seen:
            continue
        seen.add(stable_key)
        unit = normalize_leave_unit(first_nonempty(record, UNIT_KEYS))
        source = first_nonempty(record, SOURCE_KEYS)
        leave_types.append({
            "code": str(code or name),
            "name": str(name or code),
            "unit": unit,
            "source": str(source or ""),
        })
    return leave_types


def normalize_balance_records(payload: Any) -> list[dict[str, Any]]:
    raw_records = recursively_collect_dicts(payload)
    return [record for record in raw_records if first_nonempty(record, USER_ID_KEYS) or first_nonempty(record, LEAVE_CODE_KEYS) or first_nonempty(record, LEAVE_NAME_KEYS)]


def has_known_collection(payload: Any) -> bool:
    """Return whether the response explicitly contains a recognized list boundary."""
    if isinstance(payload, list):
        return True
    if not isinstance(payload, dict):
        return False
    for key in ("data", "result", "records", "list", "items", "values", "leaveTypes", "vacationTypes"):
        if key not in payload:
            continue
        value = payload[key]
        if isinstance(value, list) or (isinstance(value, dict) and has_known_collection(value)):
            return True
    return False


def projection_error(source: str) -> cmn.DwsCallError:
    return cmn.DwsCallError(
        f"{source} 响应缺少已知业务记录或列表边界。",
        error_info={
            "type": "api",
            "subtype": "projection_unknown",
            "message": f"{source} 响应缺少已知业务记录或列表边界。",
        },
    )


def query_leave_types(inspect: bool, stats: cmn.CallStats) -> list[dict[str, str]]:
    try:
        payload = cmn.run_dws(["attendance", "vacation", "types"])
    except cmn.DwsCallError as exc:
        stats.record_failure("vacation-types", exc)
        return []
    if inspect:
        records = recursively_collect_dicts(payload)
        cmn.log("[inspect] vacation types first record:\n" + json.dumps(records[:1], ensure_ascii=False, indent=2))
    leave_types = normalize_leave_types(payload)
    if not leave_types and not has_known_collection(payload):
        stats.record_failure("vacation-types", projection_error("attendance vacation types"))
        return []
    stats.record_success("vacation-types", item_count=len(leave_types))
    cmn.log(f"[types] 获取到 {len(leave_types)} 个假期规则")
    return leave_types


def extract_message(payload: Any) -> str:
    if isinstance(payload, dict):
        message = first_nonempty(payload, MESSAGE_KEYS)
        if message:
            return str(message)
        for value in payload.values():
            nested_message = extract_message(value)
            if nested_message:
                return nested_message
    if isinstance(payload, list):
        for item in payload:
            nested_message = extract_message(item)
            if nested_message:
                return nested_message
    return ""


def enrich_balance_record(record: dict[str, Any], leave_type: dict[str, str]) -> dict[str, Any]:
    enriched = dict(record)
    enriched.setdefault("leaveCode", leave_type["code"])
    enriched.setdefault("leaveName", leave_type["name"])
    if leave_type.get("unit"):
        enriched.setdefault("unit", leave_type["unit"])
    if leave_type.get("source"):
        enriched.setdefault("source", leave_type["source"])
    return enriched


def build_message_balance_records(
    batch: list[str],
    leave_type: dict[str, str],
    message: str,
) -> list[dict[str, Any]]:
    if not message:
        return []
    return [
        {
            "userId": user_id,
            "leaveCode": leave_type["code"],
            "leaveName": leave_type["name"],
            "unit": leave_type.get("unit") or "",
            "source": leave_type.get("source") or "",
            "message": message,
        }
        for user_id in batch
    ]


def query_balance_payload(batch: list[str], leave_code: str) -> Any:
    return cmn.run_dws([
        "attendance", "vacation", "balance",
        "--users", ",".join(batch),
        "--leave-code", leave_code,
    ])


def normalize_query_records(
    payload: Any,
    batch: list[str],
    leave_type: dict[str, str],
) -> list[dict[str, Any]]:
    records = [
        enrich_balance_record(record, leave_type)
        for record in normalize_balance_records(payload)
    ]
    if records:
        return records
    message_records = build_message_balance_records(batch, leave_type, extract_message(payload))
    if message_records:
        return message_records
    if has_known_collection(payload):
        return []
    raise projection_error("attendance vacation balance")


def query_single_user_after_batch_error(
    user_id: str,
    leave_type: dict[str, str],
    batch_error: cmn.DwsCallError,
) -> list[dict[str, Any]]:
    leave_code = leave_type["code"]
    try:
        payload = query_balance_payload([user_id], leave_code)
    except cmn.DwsCallError as error:
        if is_external_leave_type(leave_type) and not error.is_permission_error:
            return build_message_balance_records(
                [user_id],
                leave_type,
                EXTERNAL_BALANCE_UNAVAILABLE_MESSAGE,
            )
        if is_no_balance_message(error) or is_not_applicable_message(error):
            return build_message_balance_records([user_id], leave_type, str(error))
        raise
    records = normalize_query_records(payload, [user_id], leave_type)
    if records:
        return records
    return build_message_balance_records([user_id], leave_type, str(batch_error))


def query_balance_records(
    user_ids: list[str],
    leave_types: list[dict[str, str]],
    inspect: bool,
    stats: cmn.CallStats,
) -> list[dict[str, Any]]:
    all_records: list[dict[str, Any]] = []
    for leave_index, leave_type in enumerate(leave_types, start=1):
        leave_code = leave_type["code"]
        cmn.log(f"[balance] 查询假期规则 {leave_index}/{len(leave_types)}：{leave_type['name']}({leave_code})")
        for batch_index, batch in enumerate(cmn.chunk_users(user_ids, MAX_USERS_PER_BALANCE_BATCH), start=1):
            step_id = f"vacation-balance:{leave_code}:batch:{batch_index}"
            cmn.log(f"[balance] 查询第 {batch_index} 批，{len(batch)} 人")
            try:
                payload = query_balance_payload(batch, leave_code)
            except cmn.DwsCallError as error:
                if is_external_leave_type(leave_type) and not error.is_permission_error:
                    cmn.warn(
                        f"[balance] 外部假期规则 {leave_type['name']}({leave_code}) 查询失败，"
                        "按外部规则暂无余额处理"
                    )
                    records = build_message_balance_records(
                        batch,
                        leave_type,
                        EXTERNAL_BALANCE_UNAVAILABLE_MESSAGE,
                    )
                    all_records.extend(records)
                    stats.record_success(step_id, item_count=len(records))
                    continue
                if is_no_balance_message(error):
                    cmn.warn(
                        f"[balance] 假期规则 {leave_type['name']}({leave_code}) 没有余额，"
                        "按不限制余额处理"
                    )
                    records = build_message_balance_records(batch, leave_type, str(error))
                    all_records.extend(records)
                    stats.record_success(step_id, item_count=len(records))
                    continue
                if is_not_applicable_message(error):
                    cmn.warn(
                        f"[balance] 假期规则 {leave_type['name']}({leave_code}) 依赖员工时间字段，"
                        "改为逐个员工查询并将缺失配置的员工标为不适用"
                    )
                    fallback_records: list[dict[str, Any]] = []
                    fallback_failed = False
                    for user_id in batch:
                        try:
                            fallback_records.extend(query_single_user_after_batch_error(user_id, leave_type, error))
                        except cmn.DwsCallError as fallback_error:
                            fallback_failed = True
                            stats.record_failure(f"{step_id}:user:{user_id}", fallback_error)
                    all_records.extend(fallback_records)
                    if fallback_records:
                        stats.record_success(step_id, item_count=len(fallback_records))
                    elif not fallback_failed:
                        stats.record_success(step_id, item_count=0)
                    continue
                stats.record_failure(step_id, error)
                continue

            try:
                records = normalize_query_records(payload, batch, leave_type)
            except cmn.DwsCallError as error:
                stats.record_failure(step_id, error)
                continue
            stats.record_success(step_id, item_count=len(records))
            if inspect and leave_index == 1 and batch_index == 1:
                cmn.log("[inspect] vacation balance first record:\n" + json.dumps(records[:1], ensure_ascii=False, indent=2))
            all_records.extend(records)
    cmn.log(f"[balance] 获取到 {len(all_records)} 条余额记录")
    return all_records


def extract_user_id(record: dict[str, Any], fallback_users: list[str]) -> str:
    user_id = first_nonempty(record, USER_ID_KEYS)
    if user_id:
        return str(user_id)
    if len(fallback_users) == 1:
        return fallback_users[0]
    return ""


def build_leave_columns(
    leave_types: list[dict[str, str]],
    balance_records: list[dict[str, Any]],
    keywords: list[str],
) -> list[dict[str, str]]:
    columns: list[dict[str, str]] = []
    seen: set[str] = set()

    for leave_type in leave_types:
        code = leave_type["code"]
        name = leave_type["name"]
        if keywords and not any(keyword in name for keyword in keywords):
            continue
        seen.add(code)
        columns.append(leave_type)

    for record in balance_records:
        code = first_nonempty(record, LEAVE_CODE_KEYS)
        name = first_nonempty(record, LEAVE_NAME_KEYS)
        if not code and not name:
            continue
        code_str = str(code or name)
        name_str = str(name or code)
        if code_str in seen:
            continue
        if keywords and not any(keyword in name_str for keyword in keywords):
            continue
        seen.add(code_str)
        unit = normalize_leave_unit(first_nonempty(record, UNIT_KEYS))
        source = first_nonempty(record, SOURCE_KEYS)
        columns.append({"code": code_str, "name": name_str, "unit": unit, "source": str(source or "")})

    return columns


def build_balance_index(
    user_ids: list[str],
    balance_records: list[dict[str, Any]],
) -> dict[str, dict[str, Any]]:
    balance_index: dict[str, dict[str, Any]] = {user_id: {} for user_id in user_ids}
    for record in balance_records:
        user_id = extract_user_id(record, user_ids)
        code = first_nonempty(record, LEAVE_CODE_KEYS)
        name = first_nonempty(record, LEAVE_NAME_KEYS)
        if not user_id or (not code and not name):
            continue
        value = format_balance_value(record)
        if code:
            balance_index.setdefault(user_id, {})[str(code)] = value
        if name:
            balance_index.setdefault(user_id, {})[str(name)] = value
    return balance_index


def extract_user_extra(record: dict[str, Any]) -> dict[str, str]:
    return {
        "entry_time": format_date(first_nonempty(record, ENTRY_TIME_KEYS)),
        "first_work_time": format_date(first_nonempty(record, FIRST_WORK_TIME_KEYS)),
    }


def build_user_extra_index(
    user_ids: list[str],
    balance_records: list[dict[str, Any]],
) -> dict[str, dict[str, str]]:
    result = {
        user_id: {"entry_time": "未设置", "first_work_time": "未设置"}
        for user_id in user_ids
    }
    for record in balance_records:
        user_id = extract_user_id(record, user_ids)
        if not user_id:
            continue
        extra = extract_user_extra(record)
        current = result.setdefault(user_id, {"entry_time": "未设置", "first_work_time": "未设置"})
        if current["entry_time"] == "未设置" and extra["entry_time"] != "未设置":
            current["entry_time"] = extra["entry_time"]
        if current["first_work_time"] == "未设置" and extra["first_work_time"] != "未设置":
            current["first_work_time"] = extra["first_work_time"]
    return result


def build_headers(leave_columns: list[dict[str, str]]) -> list[str]:
    headers = BASE_HEADERS.copy()
    for leave_column in leave_columns:
        name = leave_column["name"]
        unit = leave_column.get("unit") or ""
        headers.append(f"{name}({unit})" if unit else name)
    return headers


def build_rows(
    user_ids: list[str],
    leave_columns: list[dict[str, str]],
    balance_index: dict[str, dict[str, Any]],
    user_extra_index: dict[str, dict[str, str]],
    user_info_map: dict[str, cmn.UserInfo],
) -> list[list[Any]]:
    rows: list[list[Any]] = []
    for user_id in user_ids:
        user_info = user_info_map.get(user_id, cmn.UserInfo(name=user_id))
        user_extra = user_extra_index.get(user_id, {})
        user_balances = balance_index.get(user_id, {})
        row: list[Any] = [
            user_info.name or user_id,
            user_info.dept_name,
            user_extra.get("entry_time") or "未设置",
            user_extra.get("first_work_time") or "未设置",
        ]
        for leave_column in leave_columns:
            row.append(
                user_balances.get(leave_column["code"], user_balances.get(leave_column["name"], "不适用"))
            )
        rows.append(row)
    return rows


def main() -> int:
    args = parse_args()
    raw_ids = [user_id.strip() for user_id in args.users.split(",") if user_id.strip()]
    if not raw_ids:
        cmn.error("--users 不能为空")
        return 2

    user_ids = cmn.resolve_users_from_input(raw_ids)
    if not user_ids:
        cmn.error("未能解析出任何有效员工 userId")
        return 2
    cmn.log(f"[users] 最终用户列表：{len(user_ids)} 人")

    keywords = [keyword.strip() for keyword in args.leave_keywords.split(",") if keyword.strip()]
    stats = cmn.CallStats()
    leave_types = query_leave_types(args.inspect, stats)
    if not leave_types:
        report = {
            "users": user_ids,
            "employeeCount": len(user_ids),
            "leaveColumnCount": 0,
            "rowCount": 0,
            "output": None,
            "keywords": keywords,
        }
        outcome, output_data, output_error = cmn.report_result(stats, report)
        if outcome == "partial_failure":
            outcome = "failure"
            output_error = stats.failed[0]["error"]
        return emit(
            fmt=args.format, outcome=outcome, data=output_data, error=output_error,
            dry_run=args.dry_run,
            text=("没有可导出的假期规则；未生成文件" if outcome == "success"
                  else "假期规则查询失败；未生成文件"),
        )
    balance_records = query_balance_records(user_ids, leave_types, args.inspect, stats)

    leave_columns = build_leave_columns(leave_types, balance_records, keywords)
    if not leave_columns:
        return emit(
            fmt=args.format,
            outcome="failure",
            error={
                "type": "validation",
                "subtype": "leave_columns_not_found",
                "message": "未匹配到任何假期规则列，请检查假期规则或 --leave-keywords 参数。",
            },
            text="未匹配到任何假期规则列，请检查假期规则或 --leave-keywords 参数。",
        )

    user_info_map = cmn.resolve_user_info(user_ids)
    balance_index = build_balance_index(user_ids, balance_records)
    user_extra_index = build_user_extra_index(user_ids, balance_records)
    headers = build_headers(leave_columns)
    rows = build_rows(user_ids, leave_columns, balance_index, user_extra_index, user_info_map)

    out_name = args.out or f"attendance_vacation_balance_{datetime.now().strftime('%Y%m%d_%H%M%S')}.xlsx"
    title = "假期余额列表"
    subtitle = f"报表生成时间：{datetime.now().strftime('%Y-%m-%d %H:%M')}；员工数：{len(user_ids)}；假期规则数：{len(leave_columns)}"

    result_data = {
        "users": user_ids,
        "employeeCount": len(user_ids),
        "leaveColumnCount": len(leave_columns),
        "rowCount": len(rows),
        "output": os.path.abspath(out_name),
        "keywords": keywords,
    }
    outcome, output_data, output_error = cmn.report_result(stats, result_data)
    balance_succeeded = any(
        str(item.get("id", "")).startswith("vacation-balance:")
        for item in stats.succeeded
    )
    balance_failed = any(
        str(item.get("id", "")).startswith("vacation-balance:")
        for item in stats.failed
    )
    if balance_failed and not balance_succeeded:
        outcome = "failure"
        output_error = next(
            item["error"] for item in stats.failed
            if str(item.get("id", "")).startswith("vacation-balance:")
        )
    if args.dry_run:
        preview = {**result_data, "write": False}
        if isinstance(output_data, dict) and "report" in output_data:
            output_data = {**output_data, "report": preview}
        else:
            output_data = preview
        return emit(
            fmt=args.format, outcome=outcome, data=output_data, error=output_error,
            dry_run=True,
            text=("[dry-run] 已完成远端只读查询和报表预览，不写入 Excel 文件"
                  if outcome == "success" else "[dry-run] 假期余额查询不完整；未写入 Excel 文件"),
        )

    if outcome == "failure":
        if isinstance(output_data, dict) and "report" in output_data:
            output_data = {**output_data, "report": {**result_data, "write": False}}
        return emit(
            fmt=args.format, outcome=outcome, data=output_data, error=output_error,
            text="假期余额批次全部失败；未写入 Excel 文件",
        )

    try:
        cmn.write_excel(
            out_name,
            headers,
            rows,
            sheet_name="假期余额",
            title=title,
            subtitle=subtitle,
        )
    except RuntimeError as error:
        cmn.error(str(error))
        return 1

    if args.format != "text":
        return emit(fmt=args.format, outcome=outcome, data=output_data, error=output_error)
    if outcome == "partial_failure":
        return emit(
            fmt=args.format, outcome=outcome, data=output_data,
            text=f"[警告] 假期余额表仅包含成功批次：{os.path.abspath(out_name)}",
        )
    print("✅ 假期余额 Excel 导出完成")
    print(f"- 输出文件：{os.path.abspath(out_name)}")
    print(f"- 员工数量：{len(user_ids)}")
    print(f"- 假期规则列数：{len(leave_columns)}")
    if keywords:
        print(f"- 假期筛选关键词：{','.join(keywords)}")
    print("- 说明：每名员工一行，假期规则横向展开；未设置假期余额显示“不限制余额”，hideQuota=true 显示“不适用”，余额为 0 时显示 0。")
    return 0


if __name__ == "__main__":
    sys.exit(run_main(main))
