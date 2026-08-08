#!/usr/bin/env python3
"""
查询团队成员本周排班和出勤统计

用法:
    python attendance_team_shift.py --users userId1,userId2,userId3
    python attendance_team_shift.py --users userId1,userId2 \
        --from 2026-03-10 --to 2026-03-14
    python attendance_team_shift.py --users userId1 --dry-run
"""

import sys
import json
import subprocess
import argparse
from datetime import datetime, timedelta
from typing import List, Any, Optional

from _runtime import add_contract_flags, emit, failure, run_main


def run_dws(
    args: List[str], dry_run: bool = False,
) -> Optional[Any]:
    cmd = ['dws'] + args
    if dry_run:
        print(f"[dry-run] {' '.join(cmd)}", file=sys.stderr)
        return None
    try:
        result = subprocess.run(
            cmd, capture_output=True, text=True, timeout=60
        )
        if result.returncode != 0:
            print(f"错误：{result.stderr.strip()}", file=sys.stderr)
            return None
        return json.loads(result.stdout)
    except (subprocess.TimeoutExpired, json.JSONDecodeError,
            FileNotFoundError) as e:
        print(f"错误：{e}", file=sys.stderr)
        return None


def get_week_range():
    today = datetime.now()
    monday = today - timedelta(days=today.weekday())
    friday = monday + timedelta(days=4)
    return monday.strftime('%Y-%m-%d'), friday.strftime('%Y-%m-%d')


def main() -> int:
    parser = argparse.ArgumentParser(
        description='查询团队成员排班和出勤统计'
    )
    parser.add_argument(
        '--users', required=True, help='用户 ID 列表，逗号分隔'
    )
    mon, fri = get_week_range()
    parser.add_argument('--from', dest='from_date', default=mon)
    parser.add_argument('--to', dest='to_date', default=fri)
    add_contract_flags(parser)
    args = parser.parse_args()

    user_count = len(args.users.split(','))
    if user_count > 50:
        return failure(args.format, '最多查询 50 人')

    print(f"📊 团队排班查询 ({args.from_date} ~ {args.to_date})", file=sys.stderr)
    print(f"   人数: {user_count}", file=sys.stderr)
    print('=' * 50, file=sys.stderr)

    print('\n🔍 查询排班信息...', file=sys.stderr)
    data = run_dws([
        'attendance', 'shift', 'list',
        '--users', args.users,
        '--start', args.from_date,
        '--end', args.to_date,
        '--format', 'json',
    ], dry_run=args.dry_run)

    if args.dry_run:
        return emit(fmt=args.format, outcome='success', data={
            'users': args.users.split(','), 'start': args.from_date,
            'end': args.to_date,
        }, dry_run=True, text='[dry-run] 将查询团队排班信息')
    if not data:
        return emit(fmt=args.format, outcome='success', data={
            'users': args.users.split(','), 'items': [], 'count': 0,
        }) if args.format != 'text' else 0

    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data=data)
    print(json.dumps(data, ensure_ascii=False, indent=2))
    return 0


if __name__ == '__main__':
    sys.exit(run_main(main))
