#!/usr/bin/env python3
"""Batch-approve or reject pending OA instances with explicit result truth.

Use ``--yes`` only after the user has confirmed the affected instances and the
approval action.  A successful approve/reject response means the request was
accepted; it is deliberately not represented as a verified business terminal
state because an instance can advance asynchronously.
"""

from __future__ import annotations

import argparse
import sys
from datetime import datetime, timedelta
from typing import Any, Mapping, Optional

from _runtime import ChildDWSResult, add_contract_flags, batch_data, batch_outcome, emit, failure, run_child_dws, run_main


def to_iso(value: datetime) -> str:
    return value.strftime("%Y-%m-%dT%H:%M:%S+08:00")


def child_data(payload: Any) -> Any:
    if not isinstance(payload, Mapping):
        return payload
    data = payload.get("data")
    if isinstance(data, Mapping) and "result" in data:
        return data["result"]
    return data if data is not None else payload.get("result", payload)


def declared_list(payload: Any, *keys: str) -> Optional[list[Any]]:
    data = child_data(payload)
    if isinstance(data, list):
        return data
    if not isinstance(data, Mapping):
        return None
    for key in keys:
        value = data.get(key)
        if isinstance(value, list):
            return value
    return None


def instance_id(item: Any) -> Optional[str]:
    if not isinstance(item, Mapping):
        return None
    for key in ("processInstanceId", "instanceId", "id"):
        value = item.get(key)
        if isinstance(value, str) and value:
            return value
    return None


def task_id(item: Any) -> Optional[str]:
    if isinstance(item, str) and item:
        return item
    if not isinstance(item, Mapping):
        return None
    for key in ("taskId", "id"):
        value = item.get(key)
        if isinstance(value, (str, int)) and str(value):
            return str(value)
    return None


def read_pending(days: int) -> ChildDWSResult:
    now = datetime.now()
    return run_child_dws([
        "oa", "approval", "list-pending", "--start", to_iso(now - timedelta(days=days)),
        "--end", to_iso(now), "--format", "json",
    ], timeout=60)


def read_tasks(instance: str) -> ChildDWSResult:
    return run_child_dws(["oa", "approval", "tasks", "--instance-id", instance, "--format", "json"], timeout=60)


def apply_action(action: str, instance: str, task: str, remark: str) -> ChildDWSResult:
    command = ["oa", "approval", action, "--instance-id", instance, "--task-id", task, "--yes", "--format", "json"]
    if remark:
        command.extend(["--remark", remark])
    return run_child_dws(command, timeout=60)


def read_detail(instance: str) -> ChildDWSResult:
    return run_child_dws(["oa", "approval", "detail", "--instance-id", instance, "--format", "json"], timeout=60)


def confirmation_required(fmt: str, action: str, count: int) -> int:
    return emit(
        fmt=fmt,
        outcome="failure",
        error={
            "type": "policy",
            "subtype": "confirmation_required",
            "message": f"批量{action} {count} 条审批前需要用户确认；确认后追加 --yes。",
            "actions": ["向用户展示审批实例与影响", "取得明确同意后以相同参数追加 --yes"],
        },
        text=f"需要确认后才能批量{action}审批。",
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="批量同意/拒绝待审批项")
    parser.add_argument("--action", required=True, choices=("approve", "reject"), help="审批动作")
    parser.add_argument("--remark", default="", help="审批意见")
    parser.add_argument("--days", type=int, default=7, help="未显式指定实例时读取的待审批时间窗")
    parser.add_argument("--instance-ids", default="", help="逗号分隔的审批实例 ID")
    parser.add_argument("--yes", action="store_true", help="用户已明确确认批量审批动作")
    add_contract_flags(parser)
    args = parser.parse_args()
    if args.days <= 0:
        return failure(args.format, "days 必须大于 0")
    instances = [value.strip() for value in args.instance_ids.split(",") if value.strip()]

    if args.dry_run:
        return emit(
            fmt=args.format,
            outcome="success",
            data={
                "action": args.action,
                "instanceIds": instances,
                "days": args.days,
                "plan": "读取待审批实例、逐项解析唯一 taskId，再执行审批动作；不执行任何远端读写。",
            },
            dry_run=True,
            text="[dry-run] 将读取待审批项、解析 taskId 并执行审批动作。",
        )
    if not args.yes:
        return confirmation_required(args.format, "同意" if args.action == "approve" else "拒绝", len(instances) if instances else 0)

    children: list[dict[str, Any]] = []
    if not instances:
        pending = read_pending(args.days)
        if pending.meta:
            children.append({"id": "pending-list", "meta": pending.meta})
        if pending.state != "success":
            return emit(
                fmt=args.format,
                outcome="failure",
                error=pending.error or {"type": "api", "message": "读取待审批列表未得到可确认结果。"},
                meta={"children": children} if children else None,
                text="无法确认待审批列表，未执行审批动作。",
            )
        values = declared_list(pending.payload, "processInstanceList", "items")
        if values is None:
            return emit(
                fmt=args.format,
                outcome="failure",
                error={"type": "api", "message": "待审批列表返回形状未知，未执行审批动作。"},
                meta={"children": children} if children else None,
                text="待审批列表返回形状未知，未执行审批动作。",
            )
        unresolved = [index for index, value in enumerate(values, start=1) if instance_id(value) is None]
        if unresolved:
            return emit(
                fmt=args.format,
                outcome="failure",
                error={"type": "api", "message": "待审批列表包含没有稳定实例 ID 的条目，未执行审批动作。", "details": {"indexes": unresolved}},
                meta={"children": children} if children else None,
                text="待审批列表投影不完整，未执行审批动作。",
            )
        instances = [instance_id(value) for value in values if instance_id(value) is not None]
    if not instances:
        return emit(
            fmt=args.format,
            outcome="success",
            data=batch_data(succeeded=[], failed=[], unknown=[], total=0, action=args.action, verification={"state": "not_applicable", "reason": "empty_pending_list"}),
            meta={"children": children} if children else None,
            text="没有待处理的审批。",
        )

    succeeded: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    unknown: list[dict[str, Any]] = []
    for instance in instances:
        tasks = read_tasks(instance)
        if tasks.meta:
            children.append({"id": f"{instance}:tasks", "meta": tasks.meta})
        if tasks.state == "failed":
            failed.append({"id": instance, "error": tasks.error or {"type": "api", "message": "读取审批任务失败；未发送审批动作。"}})
            continue
        if tasks.state != "success":
            unknown.append({"id": instance, "reason": "任务查询未返回终态结果；未尝试审批，请先核查审批任务。", "error": tasks.error or {"type": "api", "message": "审批任务查询终态未知"}})
            continue
        task_values = declared_list(tasks.payload, "tasks", "items")
        task_ids = [value for value in (task_id(item) for item in task_values or []) if value]
        if task_values is None:
            failed.append({"id": instance, "error": {"type": "api", "message": "审批任务返回形状未知；未发送审批动作。"}})
            continue
        if len(task_ids) != 1:
            failed.append({"id": instance, "error": {"type": "precondition", "message": "需要恰好一个可执行审批 taskId；未任选任务执行。", "details": {"task_count": len(task_ids)}}})
            continue
        result = apply_action(args.action, instance, task_ids[0], args.remark)
        if result.meta:
            children.append({"id": f"{instance}:action", "meta": result.meta})
        if result.state == "failed":
            failed.append({"id": instance, "task_id": task_ids[0], "error": result.error or {"type": "api", "message": "审批动作未执行"}})
            continue
        if result.state != "success":
            unknown.append({"id": instance, "task_id": task_ids[0], "reason": "审批请求未返回可确认终态；不要盲目重试，请先核查审批状态。", "error": result.error or {"type": "api", "message": "审批动作终态未知"}})
            continue
        detail = read_detail(instance)
        if detail.meta:
            children.append({"id": f"{instance}:detail", "meta": detail.meta})
        verification: dict[str, Any] = {"state": "not_verified", "method": "approval_detail", "reason": "详情可读不等于能从通用响应证明当前审批动作已最终生效"}
        if detail.state == "success":
            verification["data"] = detail.payload
        else:
            verification["state"] = "verification_failed"
            verification["error"] = detail.error or {"type": "api", "message": "审批动作后无法读取实例详情"}
        succeeded.append({"id": instance, "task_id": task_ids[0], "action": args.action, "data": result.payload, "verification": verification})

    data = batch_data(
        succeeded=succeeded, failed=failed, unknown=unknown, total=len(instances), action=args.action,
        verification={"state": "per_item", "instruction": "succeeded[] 仅表示审批请求被接受；只有产品特定终态证据可确认审批最终结果"},
    )
    outcome = batch_outcome(data)
    top_error = failed[0]["error"] if outcome == "failure" and failed else ({"type": "api", "message": "没有审批实例获得可确认请求成功；请先核查 unknown 项。"} if outcome == "failure" else None)
    return emit(
        fmt=args.format, outcome=outcome, data=data, error=top_error,
        meta={"children": children} if children else None,
        text=f"批量审批：请求已接受 {len(succeeded)}，明确未执行 {len(failed)}，终态未知 {len(unknown)}。",
    )


if __name__ == "__main__":
    sys.exit(run_main(main))
