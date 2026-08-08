#!/usr/bin/env python3
"""
查看待我审批列表 + 逐条显示详情（自动时间戳计算）

用法:
    python oa_pending_review.py                # 最近 7 天
    python oa_pending_review.py --days 30      # 最近 30 天
    python oa_pending_review.py --dry-run
"""

import sys
import json
import subprocess
import argparse
from datetime import datetime, timedelta
from typing import List, Any, Optional

from _runtime import add_contract_flags, emit, failure


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


def to_iso(dt: datetime) -> str:
    return dt.strftime('%Y-%m-%dT%H:%M:%S+08:00')


def main() -> int:
    parser = argparse.ArgumentParser(
        description='查看待我审批列表'
    )
    parser.add_argument(
        '--days', type=int, default=7, help='查询天数 (默认 7)'
    )
    add_contract_flags(parser)
    args = parser.parse_args()

    now = datetime.now()
    start = now - timedelta(days=args.days)

    print(f"📋 查询待审批 (最近 {args.days} 天)...", file=sys.stderr)
    data = run_dws([
        'oa', 'approval', 'list-pending',
        '--start', to_iso(start),
        '--end', to_iso(now),
        '--format', 'json',
    ], dry_run=args.dry_run)

    if args.dry_run:
        run_dws([
            'oa', 'approval', 'detail',
            '--instance-id', '<INSTANCE_ID>',
            '--format', 'json',
        ], dry_run=True)
        return emit(fmt=args.format, outcome='success', data={
            'days': args.days, 'start': to_iso(start), 'end': to_iso(now),
        }, dry_run=True, text='[dry-run] 将查询审批列表并读取详情')

    if not data:
        return failure(args.format, '审批列表查询失败')

    if isinstance(data, list):
        instances = data
    elif isinstance(data, dict):
        inner = data.get('result', data)
        if isinstance(inner, dict):
            instances = inner.get('processInstanceList',
                                  inner.get('items', []))
        elif isinstance(inner, list):
            instances = inner
        else:
            instances = []
    else:
        instances = []
    if not instances:
        return emit(fmt=args.format, outcome='success', data={
            'count': 0, 'items': [],
        }) if args.format != 'text' else 0

    records = []

    print(f"\n🔔 待审批列表 ({len(instances)} 条)")
    print('=' * 50)

    for i, inst in enumerate(instances, 1):
        if not isinstance(inst, dict):
            print(f"\n  [{i}] {inst}")
            continue
        inst_id = (inst.get('processInstanceId')
                   or inst.get('id', ''))
        title = inst.get('title') or inst.get('name', '无标题')
        status = inst.get('status') or inst.get('result', '')
        create_time = inst.get('createTime', '')
        if isinstance(create_time, (int, float)):
            create_time = datetime.fromtimestamp(
                create_time / 1000
            ).strftime('%Y-%m-%d %H:%M')

        records.append({'id': inst_id, 'title': title, 'status': status,
                        'createTime': create_time})
        print(f"\n  [{i}] {title}", file=sys.stderr)
        print(f"      状态: {status}  创建: {create_time}", file=sys.stderr)
        print(f"      ID: {inst_id}", file=sys.stderr)

        detail = run_dws([
            'oa', 'approval', 'detail',
            '--instance-id', inst_id,
            '--format', 'json',
        ])
        if detail and isinstance(detail, dict):
            forms = detail.get('formComponentValues', [])
            if forms:
                print(f"      --- 表单内容 ---", file=sys.stderr)
                for f in forms[:5]:
                    name = f.get('name', '')
                    value = f.get('value', '')
                    if value:
                        print(f"      {name}: {value[:60]}", file=sys.stderr)

    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data={
            'count': len(records), 'items': records,
        })
    return 0


if __name__ == '__main__':
    sys.exit(main())
