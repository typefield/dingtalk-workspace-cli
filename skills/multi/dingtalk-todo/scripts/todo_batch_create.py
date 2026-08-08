#!/usr/bin/env python3
"""Batch-create Todo tasks from a JSON file without losing per-item truth."""

from __future__ import annotations

import argparse
import json
import re
import sys
from datetime import datetime
from pathlib import Path
from typing import Any, Mapping, Optional

from _runtime import ChildDWSResult, add_contract_flags, batch_data, batch_outcome, emit, failure, run_child_dws, run_main


ALLOWED_PRIORITIES = {10, 20, 30, 40}
DATE_PATTERN = re.compile(r"^\d{4}-\d{2}-\d{2}$")
MAX_FILE_SIZE = 10 * 1024 * 1024


def validate_todo(item: Any, index: int) -> tuple[bool, str]:
    if not isinstance(item, Mapping):
        return False, f"第 {index} 条待办必须是对象"
    if not isinstance(item.get("title"), str) or not item["title"].strip():
        return False, f"第 {index} 条待办缺少非空 title"
    if not isinstance(item.get("executors"), str) or not item["executors"].strip():
        return False, f"第 {index} 条待办缺少非空 executors"
    priority = item.get("priority")
    if priority is not None:
        try:
            valid_priority = int(priority) in ALLOWED_PRIORITIES
        except (TypeError, ValueError):
            valid_priority = False
        if not valid_priority:
            return False, f"第 {index} 条待办优先级无效：{priority}"
    recurrence = item.get("recurrence")
    if recurrence is not None and (not isinstance(recurrence, str) or not recurrence.strip()):
        return False, f"第 {index} 条待办 recurrence 必须是非空字符串"
    if recurrence and not item.get("due"):
        return False, f"第 {index} 条待办设置 recurrence 时必须提供 due"
    return True, ""


def parse_due(value: Any) -> Optional[str]:
    if value is None or value == "":
        return None
    source = str(value)
    if source.isdigit() and len(source) >= 10:
        return source
    if DATE_PATTERN.match(source):
        return str(int(datetime.strptime(source, "%Y-%m-%d").replace(hour=23, minute=59, second=59).timestamp() * 1000))
    raise ValueError(f"无法解析截止时间：{value}")


def load_todos(input_file: str) -> list[dict[str, Any]]:
    path = Path(input_file)
    if not path.exists() or not path.is_file():
        raise ValueError(f"文件不存在或不可读：{path}")
    if path.stat().st_size > MAX_FILE_SIZE:
        raise ValueError(f"文件过大（限制 {MAX_FILE_SIZE // 1024}KB）")
    try:
        with path.open("r", encoding="utf-8") as source:
            value = json.load(source)
    except json.JSONDecodeError as exc:
        raise ValueError(f"待办 JSON 格式无效：{exc}") from exc
    if not isinstance(value, list) or not value:
        raise ValueError("JSON 文件必须是非空数组")
    todos: list[dict[str, Any]] = []
    for index, item in enumerate(value, start=1):
        valid, reason = validate_todo(item, index)
        if not valid:
            raise ValueError(reason)
        todo = dict(item)
        todo["title"] = todo["title"].strip()
        todo["executors"] = todo["executors"].strip()
        todo["due"] = parse_due(todo.get("due"))
        todos.append(todo)
    return todos


def create_task(item: Mapping[str, Any], *, dry_run: bool) -> ChildDWSResult:
    command = ["todo", "task", "create", "--title", item["title"], "--executors", item["executors"], "--format", "json"]
    if item.get("priority") is not None:
        command.extend(["--priority", str(int(item["priority"]))])
    if item.get("due"):
        command.extend(["--due", item["due"]])
    if item.get("recurrence"):
        command.extend(["--recurrence", item["recurrence"].replace("\\n", "\n")])
    return run_child_dws(command, dry_run=dry_run, timeout=60)


def task_id_from_payload(payload: Any) -> Optional[str]:
    """Expose only an observed ID; never synthesize one from the input."""
    if not isinstance(payload, Mapping):
        return None
    for container in (payload, payload.get("data"), payload.get("result")):
        if not isinstance(container, Mapping):
            continue
        for key in ("taskId", "todoTaskId"):
            value = container.get(key)
            if isinstance(value, str) and value:
                return value
    return None


def verify_task(task_id: str) -> ChildDWSResult:
    return run_child_dws(["todo", "task", "get", "--task-id", task_id, "--format", "json"], timeout=60)


def main() -> int:
    parser = argparse.ArgumentParser(description="从 JSON 文件批量创建待办")
    parser.add_argument("input", help="待办 JSON 文件")
    add_contract_flags(parser)
    args = parser.parse_args()
    try:
        todos = load_todos(args.input)
    except ValueError as exc:
        return failure(args.format, str(exc))
    succeeded: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    unknown: list[dict[str, Any]] = []
    records: list[dict[str, Any]] = []
    children: list[dict[str, Any]] = []
    for index, todo in enumerate(todos, start=1):
        entry_id = str(index)
        result = create_task(todo, dry_run=args.dry_run)
        record: dict[str, Any] = {"id": entry_id, "index": index, "title": todo["title"]}
        if result.meta:
            children.append({"id": entry_id, "meta": result.meta})
        if result.state == "success":
            task_id = task_id_from_payload(result.payload)
            if args.dry_run:
                record.update({"outcome": "success", "data": result.payload, "verification": {"state": "not_applicable", "reason": "dry_run"}})
                succeeded.append(dict(record))
            elif not task_id:
                reason = "创建请求已返回成功，但未提供可回读的 taskId；请先查询待办，禁止盲目重试。"
                error = {"type": "api", "message": "创建响应缺少任务 ID，无法验证终态"}
                record.update({"outcome": "unknown", "reason": reason, "error": error, "data": result.payload, "verification": {"state": "not_verified", "reason": "missing_task_id"}})
                unknown.append({"id": entry_id, "title": todo["title"], "reason": reason, "error": error})
            else:
                verification = verify_task(task_id)
                if verification.state == "success":
                    record.update({"outcome": "success", "task_id": task_id, "data": result.payload, "verification": {"state": "verified", "method": "task_get", "data": verification.payload}})
                    succeeded.append(dict(record))
                else:
                    reason = "创建请求已返回成功，但 task get 未能确认终态；请先核查待办，禁止盲目重试。"
                    error = verification.error or {"type": "api", "message": "待办创建后回读未返回可确认终态"}
                    record.update({"outcome": "unknown", "task_id": task_id, "reason": reason, "error": error, "data": result.payload, "verification": {"state": "verification_failed", "method": "task_get"}})
                    unknown.append({"id": entry_id, "title": todo["title"], "reason": reason, "error": error, "task_id": task_id})
        elif result.state == "failed":
            error = result.error or {"type": "api", "message": "待办创建在执行前失败"}
            record["outcome"] = "failure"
            record["error"] = error
            failed.append({"id": entry_id, "title": todo["title"], "error": error})
        else:
            error = result.error or {"type": "api", "message": "待办创建未返回可确认终态"}
            reason = "创建请求未返回可确认终态；请先查询待办，禁止盲目重试。"
            record.update({"outcome": "unknown", "reason": reason, "error": error})
            unknown.append({"id": entry_id, "title": todo["title"], "reason": reason, "error": error})
        records.append(record)
    data = batch_data(
        succeeded=succeeded, failed=failed, unknown=unknown, total=len(todos), items=records,
        verification={"state": "per_item", "method": "task_get", "instruction": "仅 verification.state=verified 的项可视为已回读"},
    )
    outcome = batch_outcome(data)
    top_error = failed[0]["error"] if outcome == "failure" and failed else ({"type": "api", "message": "没有待办获得可确认成功；请先核查未知项。"} if outcome == "failure" else None)
    return emit(
        fmt=args.format, outcome=outcome, data=data, error=top_error,
        meta={"children": children} if children else None, dry_run=args.dry_run, items=records,
        text=f"待办创建：成功 {len(succeeded)}，明确失败 {len(failed)}，终态未知 {len(unknown)}",
    )


if __name__ == "__main__":
    sys.exit(run_main(main))
