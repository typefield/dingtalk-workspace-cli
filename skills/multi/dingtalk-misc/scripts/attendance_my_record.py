#!/usr/bin/env python3
"""查看当前用户指定日期的考勤记录，不把读取失败伪装为空。"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import date, datetime
from typing import Any, Optional

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main


RECORD_FIELDS = {
    'userId', 'userid', 'workDate', 'isHasSchedule', 'isRest', 'isUnSigned',
    'recordList', 'approveList', 'workOvertime', 'workTimeDesc',
}


def run_dws(args: list[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def unwrap_unified(payload: Any) -> Any:
    if (
        isinstance(payload, dict)
        and isinstance(payload.get('ok'), bool)
        and isinstance(payload.get('outcome'), str)
        and 'data' in payload
    ):
        return payload['data']
    return payload


def projection_error(message: str) -> dict[str, Any]:
    return {'type': 'api', 'subtype': 'projection_unknown', 'message': message}


def project_self_user_id(payload: Any) -> tuple[Optional[str], Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    candidates: list[Any]
    if isinstance(value, dict) and 'result' in value:
        inner = value['result']
        candidates = inner if isinstance(inner, list) else [inner]
    else:
        candidates = [value]
    for candidate in candidates:
        if not isinstance(candidate, dict):
            continue
        employee = candidate.get('orgEmployeeModel')
        sources = [candidate, employee] if isinstance(employee, dict) else [candidate]
        for source in sources:
            user_id = source.get('userId') or source.get('userid')
            if isinstance(user_id, str) and user_id.strip():
                return user_id.strip(), None
    return None, projection_error('当前用户响应缺少稳定 userId。')


def project_records(payload: Any) -> tuple[list[dict[str, Any]], Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    if isinstance(value, dict) and 'result' in value:
        value = value['result']
    if value is None:
        rows: list[Any] = []
    elif isinstance(value, list):
        rows = value
    elif isinstance(value, dict):
        rows = [value]
    else:
        return [], projection_error('考勤详情响应不是已知对象或数组。')
    records: list[dict[str, Any]] = []
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            return [], projection_error(f'考勤详情第 {index + 1} 项不是对象。')
        if not RECORD_FIELDS.intersection(row):
            return [], projection_error(f'考勤详情第 {index + 1} 项缺少已知业务字段。')
        records.append(dict(row))
    return records, None


def parse_date(value: str) -> tuple[Optional[str], Optional[str]]:
    if value == 'today':
        return date.today().isoformat(), None
    try:
        return datetime.strptime(value, '%Y-%m-%d').date().isoformat(), None
    except ValueError:
        return None, '日期必须为 today 或有效的 YYYY-MM-DD'


def child_meta(identifier: str, result: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {'id': identifier, 'meta': result.meta} if result.meta else None


def main() -> int:
    parser = argparse.ArgumentParser(description='查看我的考勤记录')
    parser.add_argument('date', nargs='?', default='today', help='today 或 YYYY-MM-DD')
    add_contract_flags(parser)
    args = parser.parse_args()
    date_str, date_error = parse_date(args.date)
    if date_error or date_str is None:
        return failure(args.format, date_error or '日期无效')

    if args.dry_run:
        return emit(
            fmt=args.format,
            outcome='success',
            data={
                'date': date_str,
                'plan': ['获取当前用户 userId', '按 userId 和日期查询个人考勤详情'],
            },
            dry_run=True,
            text='[dry-run] 将获取当前用户并查询指定日期考勤；不会启动子 dws 进程。',
        )

    meta_children: list[dict[str, Any]] = []
    print('🔍 获取当前用户信息...', file=sys.stderr)
    self_result = run_dws(['contact', 'user', 'get-self', '--format', 'json'])
    meta_entry = child_meta('contact:get-self', self_result)
    if meta_entry:
        meta_children.append(meta_entry)
    if self_result.state != 'success':
        return emit(
            fmt=args.format,
            outcome='failure',
            error=self_result.error or projection_error('当前用户查询失败。'),
            meta={'children': meta_children} if meta_children else None,
            text='当前用户查询失败',
        )
    user_id, user_error = project_self_user_id(self_result.payload)
    if user_error or user_id is None:
        return emit(
            fmt=args.format,
            outcome='failure',
            error=user_error or projection_error('当前用户 ID 不可识别。'),
            meta={'children': meta_children} if meta_children else None,
            text='当前用户响应无法可靠解析',
        )

    print(f'📊 查询 {date_str} 考勤记录...', file=sys.stderr)
    record_result = run_dws([
        'attendance', 'record', 'get', '--user', user_id, '--date', date_str, '--format', 'json',
    ])
    meta_entry = child_meta('attendance:record', record_result)
    if meta_entry:
        meta_children.append(meta_entry)
    meta = {'children': meta_children} if meta_children else None
    if record_result.state != 'success':
        return emit(
            fmt=args.format,
            outcome='failure',
            error=record_result.error or projection_error('个人考勤查询失败。'),
            meta=meta,
            text='个人考勤查询失败',
        )
    records, record_error = project_records(record_result.payload)
    if record_error:
        return emit(
            fmt=args.format,
            outcome='failure',
            error=record_error,
            meta=meta,
            text='个人考勤响应无法可靠解析',
        )

    data = {
        'date': date_str,
        'userId': user_id,
        'items': records,
        'count': len(records),
        'coverage': {'scope': 'requested_user_date', 'complete': True},
    }
    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data=data, meta=meta)
    return emit(
        fmt=args.format,
        outcome='success',
        data=data,
        meta=meta,
        text=(f'📋 考勤记录 ({date_str})\n' + json.dumps(records, ensure_ascii=False, indent=2))
        if records else f'📋 {date_str} 未返回考勤记录',
    )


if __name__ == '__main__':
    raise SystemExit(run_main(main))
