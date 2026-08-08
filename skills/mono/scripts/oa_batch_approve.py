#!/usr/bin/env python3
"""
批量同意/拒绝待审批项（含安全确认）

用法:
    python oa_batch_approve.py --action approve --days 7
    python oa_batch_approve.py --action reject --remark "不符合要求"
    python oa_batch_approve.py --action approve --instance-ids id1,id2
    python oa_batch_approve.py --dry-run --action approve
"""

import sys
import argparse
from datetime import datetime, timedelta
from typing import List, Any, Optional

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


def run_dws(
    args: List[str], dry_run: bool = False,
) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def child_data(payload: Any) -> Any:
    """Unwrap both legacy payloads and the unified CLI envelope."""
    if isinstance(payload, dict):
        return payload.get('data', payload.get('result', payload))
    return payload


def list_items(payload: Any, *keys: str) -> Optional[list[Any]]:
    """Return a declared list or None when the child projection is ambiguous."""
    data = child_data(payload)
    if isinstance(data, list):
        return data
    if not isinstance(data, dict):
        return None
    for key in keys:
        value = data.get(key)
        if isinstance(value, list):
            return value
    return None


def to_iso(dt: datetime) -> str:
    return dt.strftime('%Y-%m-%dT%H:%M:%S+08:00')


def main() -> int:
    parser = argparse.ArgumentParser(
        description='批量同意/拒绝审批'
    )
    parser.add_argument(
        '--action', required=True,
        choices=['approve', 'reject'], help='审批动作',
    )
    parser.add_argument(
        '--remark', default='', help='审批意见'
    )
    parser.add_argument('--days', type=int, default=7)
    parser.add_argument('--instance-ids', default='')
    parser.add_argument(
        '--yes', action='store_true', help='跳过确认'
    )
    add_contract_flags(parser)
    args = parser.parse_args()

    instance_ids: List[str] = []
    if args.instance_ids:
        instance_ids = [x.strip() for x in
                        args.instance_ids.split(',') if x.strip()]
    else:
        now = datetime.now()
        start = now - timedelta(days=args.days)
        pending = run_dws([
            'oa', 'approval', 'list-pending',
            '--start', to_iso(start),
            '--end', to_iso(now),
            '--format', 'json',
        ], dry_run=args.dry_run)
        if not args.dry_run:
            if pending.state != 'success':
                return emit(
                    fmt=args.format,
                    outcome='failure',
                    error=pending.error or {
                        'type': 'api',
                        'message': '读取待审批列表未得到可确认结果。',
                    },
                    meta={'children': [{'id': 'pending-list', 'meta': pending.meta}]} if pending.meta else None,
                    text='无法确认待审批列表，未执行审批动作',
                )
            items = list_items(pending.payload, 'processInstanceList', 'items')
            if items is None:
                return emit(
                    fmt=args.format,
                    outcome='failure',
                    error={
                        'type': 'api',
                        'message': '待审批列表返回形状未知，未执行审批动作。',
                    },
                    meta={'children': [{'id': 'pending-list', 'meta': pending.meta}]} if pending.meta else None,
                    text='待审批列表返回形状未知，未执行审批动作',
                )
            instance_ids = [
                item.get('processInstanceId') or item.get('id')
                for item in items
                if isinstance(item, dict)
                and (item.get('processInstanceId') or item.get('id'))
            ]

    if not instance_ids and not args.dry_run:
        return emit(fmt=args.format, outcome='success', data={
            'total': 0, 'succeeded': [], 'failed': [], 'unknown': [],
        }, text='✅ 没有待处理的审批')

    if args.dry_run and not args.instance_ids:
        return emit(fmt=args.format, outcome='success', data={
            'action': args.action, 'days': args.days,
            'plan': 'list pending approvals, resolve tasks, then apply action',
        }, dry_run=True, text='[dry-run] 将查询待审批项、解析 taskId 并执行审批动作')

    action_label = '同意' if args.action == 'approve' else '拒绝'
    count = len(instance_ids) if instance_ids else '?'
    print(f"\n⚠️  即将 {action_label} {count} 条审批", file=sys.stderr)
    if not args.yes and not args.dry_run:
        confirm = input('确认执行？(y/N): ').strip().lower()
        if confirm != 'y':
            return failure(args.format, '用户取消审批操作')

    succeeded_items: List[dict[str, Any]] = []
    failed_items: List[dict[str, Any]] = []
    unknown_items: List[dict[str, Any]] = []
    child_meta: List[dict[str, Any]] = []
    for i, inst_id in enumerate(instance_ids or ['<INST_ID>'], 1):
        tasks = run_dws([
            'oa', 'approval', 'tasks',
            '--instance-id', inst_id,
            '--format', 'json',
        ], dry_run=args.dry_run)

        if tasks.meta:
            child_meta.append({'id': f'{inst_id}:tasks', 'meta': tasks.meta})
        if tasks.state != 'success':
            if tasks.state == 'failed':
                failed_items.append({
                    'id': inst_id,
                    'error': tasks.error or {'type': 'api', 'message': '读取审批任务失败'},
                })
            else:
                unknown_items.append({
                    'id': inst_id,
                    'reason': '任务查询未返回终态结果；未尝试审批，请先核查审批任务。',
                    'error': tasks.error,
                })
            print(f"  ✗ [{i}/{count}] {inst_id}（未发送审批动作）", file=sys.stderr)
            continue

        task_ids = list_items(tasks.payload, 'tasks', 'items') if not args.dry_run else ['<TASK_ID>']
        task_id = (
            task_ids[0] if task_ids and isinstance(task_ids[0], str)
            else task_ids[0].get('taskId', '') if task_ids and isinstance(task_ids[0], dict)
            else ''
        )
        if not task_id:
            failed_items.append({
                'id': inst_id,
                'error': {
                    'type': 'precondition',
                    'message': '未找到可执行的审批 taskId；未发送审批动作。',
                },
            })
            print(f"  ✗ [{i}/{count}] {inst_id}（没有可执行任务）", file=sys.stderr)
            continue

        cmd_args = [
            'oa', 'approval', args.action,
            '--instance-id', inst_id,
            '--task-id', task_id or '<TASK_ID>',
            '--format', 'json',
        ]
        if args.remark:
            cmd_args.extend(['--remark', args.remark])

        result = run_dws(cmd_args, dry_run=args.dry_run)
        if result.meta:
            child_meta.append({'id': inst_id, 'meta': result.meta})
        if result.state == 'success':
            print(f"  ✓ [{i}/{count}] {inst_id} → {action_label}", file=sys.stderr)
            succeeded_items.append({'id': inst_id, 'data': result.payload})
        elif result.state == 'failed':
            failed_items.append({
                'id': inst_id,
                'error': result.error or {'type': 'api', 'message': '审批动作未执行'},
            })
            print(f"  ✗ [{i}/{count}] {inst_id}", file=sys.stderr)
        else:
            unknown_items.append({
                'id': inst_id,
                'reason': '审批请求未返回可确认终态；不要盲目重试，请先核查审批状态。',
                'error': result.error,
            })
            print(f"  ✗ [{i}/{count}] {inst_id}", file=sys.stderr)

    data = batch_data(
        succeeded=succeeded_items,
        failed=failed_items,
        unknown=unknown_items,
        total=len(instance_ids or ['<INST_ID>']),
        action=args.action,
    )
    outcome = batch_outcome(data)
    top_error = None
    if outcome == 'failure':
        top_error = (
            failed_items[0]['error'] if failed_items else
            {'type': 'api', 'message': '审批动作未获得任何可确认成功；请先核查 unknown 项。'}
        )
    return emit(
        fmt=args.format,
        outcome=outcome,
        data=data,
        error=top_error,
        meta={'children': child_meta} if child_meta else None,
        dry_run=args.dry_run,
        text=(
            f"\n完成: 成功 {len(succeeded_items)}, 明确失败 {len(failed_items)}, "
            f"结果未知 {len(unknown_items)}"
        ),
    )


if __name__ == '__main__':
    sys.exit(run_main(main))
