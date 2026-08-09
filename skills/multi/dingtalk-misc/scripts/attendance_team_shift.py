#!/usr/bin/env python3
"""查询团队成员班次；严格校验请求范围和返回投影。"""

from __future__ import annotations

import argparse
import json
from datetime import date, datetime, timedelta
from typing import Any, Optional

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main


def run_dws(args: list[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def get_week_range() -> tuple[str, str]:
    today = date.today()
    monday = today - timedelta(days=today.weekday())
    friday = monday + timedelta(days=4)
    return monday.isoformat(), friday.isoformat()


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


def project_shifts(payload: Any) -> tuple[list[dict[str, Any]], Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    if isinstance(value, list):
        rows = value
    elif isinstance(value, dict):
        container = value['result'] if isinstance(value.get('result'), dict) else value
        if isinstance(value.get('result'), list):
            rows = value['result']
        elif isinstance(container.get('items'), list):
            rows = container['items']
        elif isinstance(container.get('shiftList'), list):
            rows = container['shiftList']
        elif isinstance(container.get('result'), list):
            rows = container['result']
        else:
            return [], projection_error('班次响应缺少已知 items[]/shiftList[]/result[]。')
    else:
        return [], projection_error('班次响应不是可识别的列表对象。')
    shifts: list[dict[str, Any]] = []
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            return [], projection_error(f'班次结果第 {index + 1} 项不是对象。')
        user_id = row.get('userId') or row.get('userid')
        if not isinstance(user_id, str) or not user_id.strip():
            return [], projection_error(f'班次结果第 {index + 1} 项缺少稳定 userId。')
        item = dict(row)
        item['userId'] = user_id.strip()
        shifts.append(item)
    return shifts, None


def parse_day(value: str, flag: str) -> tuple[Optional[date], Optional[str]]:
    try:
        return datetime.strptime(value, '%Y-%m-%d').date(), None
    except ValueError:
        return None, f'{flag} 必须是有效的 YYYY-MM-DD'


def main() -> int:
    parser = argparse.ArgumentParser(description='查询团队成员排班和出勤统计')
    parser.add_argument('--users', required=True, help='用户 ID 列表，逗号分隔')
    monday, friday = get_week_range()
    parser.add_argument('--start', '--from', dest='from_date', default=monday, help='开始日期 YYYY-MM-DD')
    parser.add_argument('--end', '--to', dest='to_date', default=friday, help='结束日期 YYYY-MM-DD')
    add_contract_flags(parser)
    args = parser.parse_args()

    users = list(dict.fromkeys(value.strip() for value in args.users.split(',') if value.strip()))
    if not users:
        return failure(args.format, '--users 至少需要一个非空 userId')
    if len(users) > 50:
        return failure(args.format, '最多查询 50 人')
    start, start_error = parse_day(args.from_date, '--start')
    end, end_error = parse_day(args.to_date, '--end')
    if start_error or start is None:
        return failure(args.format, start_error or '开始日期无效')
    if end_error or end is None:
        return failure(args.format, end_error or '结束日期无效')
    if end < start:
        return failure(args.format, '--end 不能早于 --start')
    if (end - start).days > 6:
        return failure(args.format, '班次查询日期范围不能超过 7 天')

    if args.dry_run:
        return emit(
            fmt=args.format,
            outcome='success',
            data={'users': users, 'start': start.isoformat(), 'end': end.isoformat()},
            dry_run=True,
            text='[dry-run] 将查询团队班次；不会启动子 dws 进程。',
        )

    result = run_dws([
        'attendance', 'shift', 'list',
        '--users', ','.join(users), '--start', start.isoformat(), '--end', end.isoformat(),
        '--format', 'json',
    ])
    meta = {'children': [{'id': 'attendance:shift-list', 'meta': result.meta}]} if result.meta else None
    if result.state != 'success':
        return emit(
            fmt=args.format,
            outcome='failure',
            error=result.error or projection_error('团队班次查询失败。'),
            meta=meta,
            text='团队班次查询失败',
        )
    shifts, shift_error = project_shifts(result.payload)
    if shift_error:
        return emit(
            fmt=args.format,
            outcome='failure',
            error=shift_error,
            meta=meta,
            text='团队班次响应无法可靠解析',
        )
    data = {
        'users': users,
        'start': start.isoformat(),
        'end': end.isoformat(),
        'items': shifts,
        'count': len(shifts),
        'coverage': {'scope': 'requested_users_date_range', 'complete': True},
    }
    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data=data, meta=meta)
    return emit(
        fmt=args.format,
        outcome='success',
        data=data,
        meta=meta,
        text=json.dumps(shifts, ensure_ascii=False, indent=2) if shifts else '未返回团队班次记录',
    )


if __name__ == '__main__':
    raise SystemExit(run_main(main))
