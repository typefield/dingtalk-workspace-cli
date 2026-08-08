#!/usr/bin/env python3
"""
从 CSV / JSON 批量导入记录到钉钉 AI 表格（新版 schema）

用法:
    python import_records.py <baseId> <tableId> data.csv [batch_size]
    python import_records.py <baseId> <tableId> data.json [batch_size]

说明：
- CSV 表头默认视为 fieldId
- JSON 支持两种格式：
  1. [{"cells": {"fldxxx": "value"}}, ...]
  2. [{"fldxxx": "value"}, ...]  # 会自动包装成 cells
"""

import sys
import csv
import json
import os
import re
import argparse
from pathlib import Path
from typing import Union, List, Dict, Any, Optional, Tuple

from _runtime import (
    ChildDWSResult,
    add_contract_flags,
    batch_data,
    batch_outcome,
    emit,
    failure,
    run_child_dws,
    run_main,
)

JsonData = Union[List[Any], Dict[str, Any]]
RecordDict = Dict[str, str]

MAX_FILE_SIZE = 50 * 1024 * 1024
ALLOWED_CSV_EXTENSIONS = ['.csv']
ALLOWED_JSON_EXTENSIONS = ['.json']
RESOURCE_ID_PATTERN = re.compile(r'^[A-Za-z0-9_-]{6,128}$')
MAX_RECORDS_PER_BATCH = 100
DEFAULT_BATCH_SIZE = 50


def resolve_safe_path(
    path: str, allowed_root: Optional[str] = None
) -> Path:
    if allowed_root is None:
        allowed_root = os.environ.get('OPENCLAW_WORKSPACE', os.getcwd())
    allowed_root = Path(allowed_root).resolve()
    target_path = (
        Path(path).resolve()
        if Path(path).is_absolute()
        else (Path.cwd() / path).resolve()
    )
    try:
        target_path.relative_to(allowed_root)
        return target_path
    except ValueError:
        raise ValueError(
            f"路径超出允许范围：{path}\n"
            f"目标路径：{target_path}\n"
            f"允许根目录：{allowed_root}\n"
            f"提示：设置 OPENCLAW_WORKSPACE 环境变量或确保文件在工作目录内"
        )


def validate_resource_id(resource_id: str) -> bool:
    return bool(
        resource_id and RESOURCE_ID_PATTERN.match(resource_id.strip())
    )


def validate_file_extension(
    filename: str, allowed_extensions: list
) -> bool:
    return any(filename.lower().endswith(ext) for ext in allowed_extensions)


def safe_csv_load(
    file_path: Path, max_size: int = MAX_FILE_SIZE
) -> List[RecordDict]:
    file_size = file_path.stat().st_size
    if file_size > max_size:
        raise ValueError(
            f"文件过大：{file_size:,} 字节 (限制：{max_size:,} 字节)"
        )
    with open(file_path, 'r', encoding='utf-8', newline='') as f:
        return list(csv.DictReader(f))


def safe_json_load(
    file_path: Path, max_size: int = MAX_FILE_SIZE
) -> JsonData:
    file_size = file_path.stat().st_size
    if file_size > max_size:
        raise ValueError(
            f"文件过大：{file_size:,} 字节 (限制：{max_size:,} 字节)"
        )
    with open(file_path, 'r', encoding='utf-8') as f:
        return json.load(f)


def sanitize_record_value(
    value: Any,
) -> Optional[Union[str, int, float, bool, list, dict]]:
    if value is None:
        return None
    if isinstance(value, (bool, int, float, list, dict)):
        return value
    if not isinstance(value, str):
        return value
    if not value.strip():
        return None

    value = value.strip()
    if value.lower() == 'true':
        return True
    if value.lower() == 'false':
        return False

    try:
        if '.' in value:
            return float(value)
        return int(value)
    except ValueError:
        return value


def normalize_record(record: Dict[str, Any]) -> Dict[str, Any]:
    if 'cells' in record and isinstance(record['cells'], dict):
        cells = record['cells']
    else:
        cells = record
    normalized = {}
    for key, value in cells.items():
        sanitized = sanitize_record_value(value)
        if sanitized is not None:
            normalized[key] = sanitized
    return {'cells': normalized}


def validate_record(
    record: Dict[str, Any], headers: List[str]
) -> Tuple[bool, str]:
    if not isinstance(record, dict):
        return False, '记录必须是对象'
    normalized = normalize_record(record)
    cells = normalized.get('cells', {})
    if not cells or not isinstance(cells, dict):
        return False, '记录必须包含非空 cells 对象'
    return True, ''


def run_dws(args: List[str], dry_run: bool = False) -> ChildDWSResult:
    """Keep a child write's execution certainty instead of returning a bool."""
    return run_child_dws(args, dry_run=dry_run, timeout=120)


def import_from_csv(
    base_id: str, table_id: str, csv_file: str,
    batch_size: int = DEFAULT_BATCH_SIZE, dry_run: bool = False,
) -> Optional[List[Dict[str, Any]]]:
    try:
        safe_path = resolve_safe_path(csv_file)
    except ValueError as e:
        print(f"路径验证失败：{e}", file=sys.stderr)
        return None

    if not validate_file_extension(csv_file, ALLOWED_CSV_EXTENSIONS):
        print(f"错误：只允许 {', '.join(ALLOWED_CSV_EXTENSIONS)} 文件", file=sys.stderr)
        return None
    if not safe_path.exists():
        print(f"错误：文件不存在：{safe_path}", file=sys.stderr)
        return None

    try:
        rows = safe_csv_load(safe_path)
    except ValueError as e:
        print(f"错误：{e}", file=sys.stderr)
        return None
    except csv.Error as e:
        print(f"错误：CSV 格式无效：{e}", file=sys.stderr)
        return None

    if not rows:
        print('错误：CSV 文件为空或没有有效数据行', file=sys.stderr)
        return None

    records = [
        normalize_record(row)
        for row in rows
        if normalize_record(row)['cells']
    ]
    return records


def import_from_json(
    base_id: str, table_id: str, json_file: str,
    batch_size: int = DEFAULT_BATCH_SIZE, dry_run: bool = False,
) -> Optional[List[Dict[str, Any]]]:
    try:
        safe_path = resolve_safe_path(json_file)
    except ValueError as e:
        print(f"路径验证失败：{e}", file=sys.stderr)
        return None

    if not validate_file_extension(json_file, ALLOWED_JSON_EXTENSIONS):
        print(f"错误：只允许 {', '.join(ALLOWED_JSON_EXTENSIONS)} 文件", file=sys.stderr)
        return None
    if not safe_path.exists():
        print(f"错误：文件不存在：{safe_path}", file=sys.stderr)
        return None

    try:
        records = safe_json_load(safe_path)
    except ValueError as e:
        print(f"错误：{e}", file=sys.stderr)
        return None
    except json.JSONDecodeError as e:
        print(f"错误：JSON 格式无效：{e}", file=sys.stderr)
        return None

    if not isinstance(records, list) or not records:
        print('错误：JSON 文件必须是非空数组', file=sys.stderr)
        return None

    for i, record in enumerate(records):
        valid, error = validate_record(record, [])
        if not valid:
            print(f"错误：记录 #{i+1} 格式无效：{error}", file=sys.stderr)
            return None

    return [normalize_record(r) for r in records]


def import_records(
    base_id: str, table_id: str,
    records: List[Dict[str, Any]], batch_size: int, dry_run: bool = False,
) -> Optional[dict[str, Any]]:
    if batch_size <= 0:
        print('错误：batch_size 必须大于 0', file=sys.stderr)
        return None
    if batch_size > MAX_RECORDS_PER_BATCH:
        batch_size = MAX_RECORDS_PER_BATCH

    total_batches = (len(records) + batch_size - 1) // batch_size
    succeeded: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    unknown: list[dict[str, Any]] = []
    child_meta: list[dict[str, Any]] = []

    for i in range(0, len(records), batch_size):
        batch = records[i:i + batch_size]
        batch_num = (i // batch_size) + 1
        records_json = json.dumps(batch, ensure_ascii=False)
        result = run_dws([
            'aitable', 'record', 'create',
            '--base-id', base_id,
            '--table-id', table_id,
            '--records', records_json,
            '--format', 'json',
        ], dry_run=dry_run)
        entry_id = f"batch:{batch_num}"
        if result.meta:
            child_meta.append({'id': entry_id, 'meta': result.meta})
        if result.state == 'success':
            print(
                f"[{batch_num}/{total_batches}] "
                f"✓ {'计划导入' if dry_run else '已提交'} {len(batch)} 条记录",
                file=sys.stderr,
            )
            succeeded.append({'id': entry_id, 'record_count': len(batch)})
        elif result.state == 'failed':
            print(f"[{batch_num}/{total_batches}] ✗ 导入在执行前失败", file=sys.stderr)
            failed.append({
                'id': entry_id,
                'record_count': len(batch),
                'error': result.error or {'type': 'api', 'message': '导入批次失败'},
            })
        else:
            print(f"[{batch_num}/{total_batches}] ? 导入终态未知", file=sys.stderr)
            unknown.append({
                'id': entry_id,
                'record_count': len(batch),
                'reason': '导入批次未返回可确认终态；请先核查记录，避免重复导入。',
                'error': result.error or {'type': 'api', 'message': '导入批次未返回可确认终态'},
            })

    return batch_data(
        succeeded=succeeded,
        failed=failed,
        unknown=unknown,
        total=total_batches,
        baseId=base_id,
        tableId=table_id,
        recordCount=len(records),
        batchSize=batch_size,
        children=child_meta,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description='批量导入 AI 表格记录')
    parser.add_argument('base_id')
    parser.add_argument('table_id')
    parser.add_argument('input_file')
    parser.add_argument('batch_size', nargs='?', type=int, default=DEFAULT_BATCH_SIZE)
    add_contract_flags(parser)
    args = parser.parse_args()

    base_id = args.base_id
    table_id = args.table_id
    input_file = args.input_file
    batch_size = args.batch_size

    if not validate_resource_id(base_id):
        return failure(args.format, '无效的 baseId 格式')
    if not validate_resource_id(table_id):
        return failure(args.format, '无效的 tableId 格式')

    if input_file.lower().endswith('.csv'):
        records = import_from_csv(
            base_id, table_id, input_file, batch_size, args.dry_run
        )
    elif input_file.lower().endswith('.json'):
        records = import_from_json(
            base_id, table_id, input_file, batch_size, args.dry_run
        )
    else:
        return failure(args.format, '仅支持 .csv 或 .json 文件')

    if records is None:
        return failure(args.format, '记录导入失败')
    data = import_records(base_id, table_id, records, batch_size, args.dry_run)
    if data is None:
        return failure(args.format, '记录导入失败')
    data['input'] = str(Path(input_file))
    outcome = batch_outcome(data)
    top_error = None
    if outcome == 'failure':
        top_error = (data['failed'][0]['error'] if data['failed'] else
                     {'type': 'api', 'message': '没有任何批次得到可确认成功；请先核查记录。'})
    return emit(
        fmt=args.format,
        outcome=outcome,
        data=data,
        error=top_error,
        dry_run=args.dry_run,
        text='记录导入完成' if outcome == 'success' else '记录导入未全部确认完成',
    )


if __name__ == '__main__':
    sys.exit(run_main(main))
