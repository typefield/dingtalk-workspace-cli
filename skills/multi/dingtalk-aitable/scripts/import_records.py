#!/usr/bin/env python3
"""Batch-import CSV or JSON records into an AITable.

CSV headers are field IDs.  JSON accepts either ``[{"cells": {...}}]`` or
plain cell maps.  Every batch has a separate outcome so a later failure never
erases evidence that an earlier write was accepted.
"""

from __future__ import annotations

import argparse
import csv
import json
import os
import re
import sys
from pathlib import Path
from typing import Any, Mapping, Optional

from _runtime import batch_data, batch_outcome, add_contract_flags, emit, failure, run_child_dws, run_main


MAX_FILE_SIZE = 50 * 1024 * 1024
RESOURCE_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]{8,128}$")
MAX_RECORDS_PER_BATCH = 100
DEFAULT_BATCH_SIZE = 50


def resolve_safe_path(path: str, allowed_root: Optional[str] = None) -> Path:
    root = Path(allowed_root or os.environ.get("OPENCLAW_WORKSPACE", os.getcwd())).resolve()
    target = Path(path).resolve() if Path(path).is_absolute() else (Path.cwd() / path).resolve()
    try:
        target.relative_to(root)
    except ValueError as exc:
        raise ValueError(f"路径超出允许范围：{path}（允许根目录：{root}）") from exc
    return target


def validate_resource_id(resource_id: str) -> bool:
    return bool(resource_id and RESOURCE_ID_PATTERN.match(resource_id.strip()))


def sanitize_record_value(value: Any) -> Any:
    if value is None or isinstance(value, (bool, int, float, list, dict)):
        return value
    if not isinstance(value, str):
        return value
    value = value.strip()
    if not value:
        return None
    if value.lower() == "true":
        return True
    if value.lower() == "false":
        return False
    try:
        return float(value) if "." in value else int(value)
    except ValueError:
        return value


def normalize_record(record: Any) -> dict[str, Any]:
    if not isinstance(record, Mapping):
        raise ValueError("记录必须是对象")
    cells = record.get("cells") if isinstance(record.get("cells"), Mapping) else record
    normalized = {str(key): sanitized for key, value in cells.items() if (sanitized := sanitize_record_value(value)) is not None}
    if not normalized:
        raise ValueError("记录必须包含非空 cells 对象")
    return {"cells": normalized}


def load_records(input_file: str) -> list[dict[str, Any]]:
    path = resolve_safe_path(input_file)
    if not path.exists() or not path.is_file():
        raise ValueError(f"文件不存在或不可读：{path}")
    if path.stat().st_size > MAX_FILE_SIZE:
        raise ValueError(f"文件过大：{path.stat().st_size:,} 字节（限制 {MAX_FILE_SIZE:,}）")
    suffix = path.suffix.lower()
    if suffix == ".csv":
        try:
            with path.open("r", encoding="utf-8", newline="") as source:
                rows = list(csv.DictReader(source))
        except csv.Error as exc:
            raise ValueError(f"CSV 格式无效：{exc}") from exc
    elif suffix == ".json":
        try:
            with path.open("r", encoding="utf-8") as source:
                rows = json.load(source)
        except json.JSONDecodeError as exc:
            raise ValueError(f"JSON 格式无效：{exc}") from exc
    else:
        raise ValueError("仅支持 .csv 或 .json 文件")
    if not isinstance(rows, list) or not rows:
        raise ValueError("输入文件必须包含非空记录数组")
    records: list[dict[str, Any]] = []
    for index, row in enumerate(rows, start=1):
        try:
            records.append(normalize_record(row))
        except ValueError as exc:
            raise ValueError(f"记录 #{index} 格式无效：{exc}") from exc
    return records


def import_batches(
    base_id: str, table_id: str, records: list[dict[str, Any]], batch_size: int, *, dry_run: bool,
) -> dict[str, Any]:
    batch_size = min(batch_size, MAX_RECORDS_PER_BATCH)
    total_batches = (len(records) + batch_size - 1) // batch_size
    succeeded: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    unknown: list[dict[str, Any]] = []
    children: list[dict[str, Any]] = []

    for offset in range(0, len(records), batch_size):
        batch = records[offset:offset + batch_size]
        batch_number = offset // batch_size + 1
        entry_id = f"batch:{batch_number}"
        command = [
            "aitable", "record", "create", "--base-id", base_id, "--table-id", table_id,
            "--records", json.dumps(batch, ensure_ascii=False), "--format", "json",
        ]
        result = run_child_dws(command, dry_run=dry_run, timeout=120)
        if result.meta:
            children.append({"id": entry_id, "meta": result.meta})
        if result.state == "success":
            print(f"[{batch_number}/{total_batches}] ✓ {'计划导入' if dry_run else '已提交'} {len(batch)} 条记录", file=sys.stderr)
            succeeded.append({"id": entry_id, "record_count": len(batch)})
        elif result.state == "failed":
            print(f"[{batch_number}/{total_batches}] ✗ 导入在执行前失败", file=sys.stderr)
            failed.append({
                "id": entry_id,
                "record_count": len(batch),
                "error": result.error or {"type": "api", "message": "导入批次失败"},
            })
        else:
            print(f"[{batch_number}/{total_batches}] ? 导入终态未知", file=sys.stderr)
            unknown.append({
                "id": entry_id,
                "record_count": len(batch),
                "reason": "导入批次未返回可确认终态；请先核查记录，避免重复导入。",
                "error": result.error or {"type": "api", "message": "导入批次未返回可确认终态"},
            })

    return batch_data(
        succeeded=succeeded, failed=failed, unknown=unknown, total=total_batches,
        baseId=base_id, tableId=table_id, recordCount=len(records), batchSize=batch_size,
        children=children,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="从 CSV 或 JSON 批量导入 AI 表格记录")
    parser.add_argument("base_id", help="目标 AI 表格 baseId")
    parser.add_argument("table_id", help="目标数据表 tableId")
    parser.add_argument("input_file", help="CSV 或 JSON 文件")
    parser.add_argument("batch_size", nargs="?", type=int, default=DEFAULT_BATCH_SIZE, help="每批记录数，最大 100")
    # Preserve historical JSON output while adding an explicit text mode.
    add_contract_flags(parser, default="json")
    args = parser.parse_args()
    if not validate_resource_id(args.base_id):
        return failure(args.format, "无效的 baseId 格式")
    if not validate_resource_id(args.table_id):
        return failure(args.format, "无效的 tableId 格式")
    if args.batch_size <= 0:
        return failure(args.format, "batch_size 必须大于 0")
    try:
        records = load_records(args.input_file)
    except ValueError as exc:
        return failure(args.format, str(exc))

    data = import_batches(args.base_id, args.table_id, records, args.batch_size, dry_run=args.dry_run)
    data["input"] = str(Path(args.input_file))
    outcome = batch_outcome(data)
    top_error = None
    if outcome == "failure":
        top_error = data["failed"][0]["error"] if data["failed"] else {
            "type": "api", "message": "没有任何批次得到可确认成功；请先核查记录。",
        }
    return emit(
        fmt=args.format, outcome=outcome, data=data, error=top_error, dry_run=args.dry_run,
        text="记录导入完成" if outcome == "success" else "记录导入未全部确认完成",
    )


if __name__ == "__main__":
    sys.exit(run_main(main, default_format="json"))
