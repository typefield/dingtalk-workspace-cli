#!/usr/bin/env python3
"""
查看我今天/本周/指定日期的考勤记录（自动获取 userId）

用法:
    python attendance_my_record.py               # 今天
    python attendance_my_record.py today          # 今天
    python attendance_my_record.py 2026-03-10     # 指定日期
    python attendance_my_record.py --dry-run      # 仅显示命令
"""

import sys
import json
import subprocess
import re
import argparse
from datetime import datetime
from typing import List, Any, Optional

from _runtime import add_contract_flags, emit, failure, run_main

DATE_PATTERN = re.compile(r'^\d{4}-\d{2}-\d{2}$')


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


def get_my_user_id(dry_run: bool = False) -> Optional[str]:
    data = run_dws([
        'contact', 'user', 'get-self', '--format', 'json',
    ], dry_run=dry_run)
    if dry_run:
        return '<MY_USER_ID>'
    if not data or not isinstance(data, dict):
        return None
    # 兼容两种结构:
    # 1) 顶层直接给 userId
    # 2) {result: [{orgEmployeeModel: {userId}}]} 包裹
    uid = data.get('userId') or data.get('userid')
    if uid:
        return uid
    inner = data.get('result')
    if isinstance(inner, dict):
        inner = [inner]
    if isinstance(inner, list):
        for item in inner:
            if not isinstance(item, dict):
                continue
            emp = item.get('orgEmployeeModel')
            if isinstance(emp, dict) and emp.get('userId'):
                return emp['userId']
            if item.get('userId'):
                return item['userId']
    return None


def main() -> int:
    parser = argparse.ArgumentParser(description='查看我的考勤记录')
    parser.add_argument('date', nargs='?', default='today',
                        help='today 或 YYYY-MM-DD')
    add_contract_flags(parser)
    args = parser.parse_args()
    dry_run = args.dry_run

    date_str = args.date
    if date_str == 'today':
        date_str = datetime.now().strftime('%Y-%m-%d')
    elif not DATE_PATTERN.match(date_str):
        return failure(args.format, '日期必须为 today 或 YYYY-MM-DD')

    print('🔍 获取当前用户信息...', file=sys.stderr)
    user_id = get_my_user_id(dry_run=dry_run)
    if not user_id and not dry_run:
        return failure(args.format, '无法获取当前用户 ID')

    print(f'📊 查询 {date_str} 考勤记录...\n', file=sys.stderr)
    data = run_dws([
        'attendance', 'record', 'get',
        '--user', user_id or '<MY_USER_ID>',
        '--date', date_str,
        '--format', 'json',
    ], dry_run=dry_run)

    if dry_run:
        return emit(fmt=args.format, outcome='success', data={
            'date': date_str, 'userId': user_id,
        }, dry_run=True, text='[dry-run] 将查询当前用户考勤记录')
    if not data:
        return emit(fmt=args.format, outcome='success', data={
            'date': date_str, 'items': [], 'count': 0,
        }) if args.format != 'text' else 0

    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data={
            'date': date_str, 'records': data,
        })
    print(f"📋 考勤记录 ({date_str})")
    print('=' * 40)
    print(json.dumps(data, ensure_ascii=False, indent=2))
    return 0


if __name__ == '__main__':
    sys.exit(run_main(main))
