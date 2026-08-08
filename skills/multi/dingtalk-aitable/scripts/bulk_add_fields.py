#!/usr/bin/env python3
"""Batch-create AITable fields from a local JSON file.

The historical entry point and its default JSON output remain unchanged.  The
result boundary is new: a child call that cannot prove its terminal state is
reported as ``unknown`` instead of being collapsed into a boolean failure.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path
from typing import Any, Mapping, Optional

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main


MAX_FILE_SIZE = 10 * 1024 * 1024
RESOURCE_ID_PATTERN = re.compile(r"^[A-Za-z0-9_-]{8,128}$")
ALLOWED_FIELD_TYPES = {
    "text", "number", "singleSelect", "multipleSelect", "date", "currency",
    "user", "department", "group", "progress", "rating", "checkbox",
    "attachment", "url", "richText", "telephone", "email", "idCard",
    "barcode", "geolocation", "address", "primaryDoc", "formula",
    "unidirectionalLink", "bidirectionalLink", "lookup", "filterUp",
    "creator", "lastModifier", "createdTime", "lastModifiedTime",
}
FIELD_TYPE_ALIASES = {"phone": "telephone"}


def resolve_safe_path(path: str, allowed_root: Optional[str] = None) -> Path:
    root = Path(allowed_root or os.environ.get("OPENCLAW_WORKSPACE", os.getcwd())).resolve()
    target = Path(path).resolve() if Path(path).is_absolute() else (Path.cwd() / path).resolve()
    try:
        target.relative_to(root)
    except ValueError as exc:
        raise ValueError(f"路径超出允许范围：{path}（允许根目录：{root}）") from exc
    return target


def validate_resource_id(value: str) -> bool:
    return bool(value and RESOURCE_ID_PATTERN.match(value.strip()))


def normalize_field_config(field: Mapping[str, Any]) -> dict[str, Any]:
    normalized = dict(field)
    if "fieldName" not in normalized and "name" in normalized:
        normalized["fieldName"] = normalized.pop("name")
    field_type = normalized.get("type", "text")
    normalized["type"] = FIELD_TYPE_ALIASES.get(field_type, field_type)
    return normalized


def validate_field_config(value: Any) -> tuple[bool, str]:
    if not isinstance(value, Mapping):
        return False, "字段配置必须是对象"
    field = normalize_field_config(value)
    name = field.get("fieldName")
    if not isinstance(name, str) or not name.strip():
        return False, "fieldName 必须是非空字符串"
    field_type = field["type"]
    if field_type not in ALLOWED_FIELD_TYPES:
        return False, f"不支持的字段类型：{field_type}"
    config = field.get("config")
    if config is not None and not isinstance(config, Mapping):
        return False, "config 必须是对象"
    config = dict(config or {})
    if field_type in {"singleSelect", "multipleSelect"} and not isinstance(config.get("options"), list):
        return False, "singleSelect / multipleSelect 必须提供 config.options 数组"
    if field_type in {"unidirectionalLink", "bidirectionalLink"} and not validate_resource_id(str(config.get("linkedTableId") or "")):
        return False, "关联字段必须提供合法的 config.linkedTableId（目标 Table ID）"
    if field_type == "lookup":
        required = ("associateField", "valuesField", "aggregator")
        if any(not config.get(key) for key in required):
            return False, "lookup 必须提供 config.associateField、config.valuesField 与 config.aggregator"
    if field_type == "filterUp":
        required = ("targetSheet", "valuesField", "aggregator")
        if any(not config.get(key) for key in required) or not isinstance(config.get("filters"), list):
            return False, "filterUp 必须提供 targetSheet、filters、valuesField 与 aggregator"
    return True, ""


def load_fields(input_file: str) -> list[dict[str, Any]]:
    path = resolve_safe_path(input_file)
    if path.suffix.lower() != ".json":
        raise ValueError("只允许 .json 文件")
    if not path.exists() or not path.is_file():
        raise ValueError(f"文件不存在或不可读：{path}")
    if path.stat().st_size > MAX_FILE_SIZE:
        raise ValueError(f"文件过大：{path.stat().st_size:,} 字节（限制 {MAX_FILE_SIZE:,}）")
    try:
        with path.open("r", encoding="utf-8") as source:
            fields = json.load(source)
    except json.JSONDecodeError as exc:
        raise ValueError(f"JSON 格式无效：{exc}") from exc
    if not isinstance(fields, list) or not fields:
        raise ValueError("fields.json 必须是非空 JSON 数组")
    if len(fields) > 15:
        raise ValueError("单次最多创建 15 个字段，请拆分后重试")
    normalized: list[dict[str, Any]] = []
    for index, field in enumerate(fields, start=1):
        valid, reason = validate_field_config(field)
        if not valid:
            raise ValueError(f"字段 #{index} 配置无效：{reason}")
        value = normalize_field_config(field)
        item: dict[str, Any] = {"fieldName": value["fieldName"].strip(), "type": value["type"]}
        if value.get("config") is not None:
            item["config"] = value["config"]
        normalized.append(item)
    return normalized


def create_fields(base_id: str, table_id: str, fields: list[dict[str, Any]], *, dry_run: bool) -> ChildDWSResult:
    command = [
        "aitable", "field", "create", "--base-id", base_id, "--table-id", table_id,
        "--fields", json.dumps(fields, ensure_ascii=False), "--format", "json",
    ]
    return run_child_dws(command, dry_run=dry_run, timeout=60)


def main() -> int:
    parser = argparse.ArgumentParser(description="批量添加 AI 表格字段")
    parser.add_argument("base_id", help="目标 AI 表格 baseId")
    parser.add_argument("table_id", help="目标数据表 tableId")
    parser.add_argument("fields_file", help="字段定义 JSON 文件")
    # Preserve historical machine-readable JSON as the default.
    add_contract_flags(parser, default="json")
    args = parser.parse_args()
    if not validate_resource_id(args.base_id):
        return failure(args.format, "无效的 baseId 格式")
    if not validate_resource_id(args.table_id):
        return failure(args.format, "无效的 tableId 格式")
    try:
        fields = load_fields(args.fields_file)
    except ValueError as exc:
        return failure(args.format, str(exc))

    result = create_fields(args.base_id, args.table_id, fields, dry_run=args.dry_run)
    data: dict[str, Any] = {
        "baseId": args.base_id,
        "tableId": args.table_id,
        "input": str(Path(args.fields_file)),
        "fieldCount": len(fields),
    }
    if result.meta:
        data["child_meta"] = result.meta
    if result.state == "success":
        data["result"] = result.payload
        return emit(
            fmt=args.format, outcome="success", data=data, dry_run=args.dry_run,
            text="字段创建计划已生成" if args.dry_run else "字段创建完成",
        )
    data["execution_state"] = "unknown" if result.state == "unknown" else "not_executed"
    error = result.error or {"type": "api", "message": "字段创建未返回可确认终态"}
    print("字段创建终态未知；请先核查字段，避免重复创建。" if result.state == "unknown" else "字段创建在执行前失败。", file=sys.stderr)
    return emit(
        fmt=args.format, outcome="failure", data=data, error=error,
        text="字段创建未确认完成",
    )


if __name__ == "__main__":
    sys.exit(run_main(main, default_format="json"))
