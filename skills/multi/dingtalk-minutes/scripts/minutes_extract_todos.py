#!/usr/bin/env python3
"""从一条或最近 N 条听记提取待办，并保留逐条读取结果。"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any


_scripts_dir = Path(__file__).resolve().parent
_shared_scripts = _scripts_dir.parents[1] / 'dingtalk-shared' / 'scripts'
for _path in (_scripts_dir, _shared_scripts):
    if str(_path) not in sys.path:
        sys.path.insert(0, str(_path))

from minutes_list_parse import project_todo_items, project_uuid_title_pairs, unwrap_child_data
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


def run_dws(args: list[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def child_meta_entry(identifier: str, result: ChildDWSResult) -> dict[str, Any] | None:
    return {'id': identifier, 'meta': result.meta} if result.meta else None


def main() -> int:
    parser = argparse.ArgumentParser(description='从听记中提取待办事项')
    parser.add_argument('--max', type=int, default=5)
    parser.add_argument('--id', default='', help='指定听记 UUID')
    add_contract_flags(parser)
    args = parser.parse_args()
    if args.max <= 0:
        return failure(args.format, '--max 必须大于 0')

    child_meta: list[dict[str, Any]] = []
    if args.id:
        pairs = [(args.id, args.id)]
        if args.dry_run:
            run_dws([
                'minutes', 'get', 'todos', '--id', args.id, '--format', 'json',
            ], dry_run=True)
            return emit(
                fmt=args.format,
                outcome='success',
                data={'ids': [args.id]},
                dry_run=True,
                text='[dry-run] 将读取指定听记并提取待办',
            )
    else:
        print('🎙️ 获取听记列表...', file=sys.stderr)
        list_result = run_dws([
            'minutes', 'list', 'mine', '--limit', str(args.max), '--format', 'json',
        ], dry_run=args.dry_run)
        if args.dry_run:
            run_dws([
                'minutes', 'get', 'todos', '--id', '<TASK_UUID>', '--format', 'json',
            ], dry_run=True)
            return emit(
                fmt=args.format,
                outcome='success',
                data={'limit': args.max},
                dry_run=True,
                text='[dry-run] 将读取听记列表并提取待办',
            )
        list_meta = child_meta_entry('minutes:list', list_result)
        if list_meta:
            child_meta.append(list_meta)
        if list_result.state != 'success':
            return emit(
                fmt=args.format,
                outcome='failure',
                error=list_result.error or {'type': 'api', 'message': '听记列表查询失败。'},
                meta={'children': child_meta} if child_meta else None,
                text='听记列表查询失败',
            )
        pairs, projection_error = project_uuid_title_pairs(unwrap_child_data(list_result.payload))
        if projection_error:
            return emit(
                fmt=args.format,
                outcome='failure',
                error=projection_error,
                meta={'children': child_meta} if child_meta else None,
                text='听记列表响应无法可靠解析',
            )

    all_todos: list[dict[str, str]] = []
    succeeded: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    for uuid, title in pairs:
        print(f"  提取待办: {title}", file=sys.stderr)
        todos_result = run_dws([
            'minutes', 'get', 'todos', '--id', uuid, '--format', 'json',
        ])
        todos_meta = child_meta_entry(f'todos:{uuid}', todos_result)
        if todos_meta:
            child_meta.append(todos_meta)
        if todos_result.state != 'success':
            failed.append({'id': uuid, 'error': todos_result.error or {'type': 'api', 'message': '听记待办读取失败。'}})
            continue
        items, todos_error = project_todo_items(todos_result.payload)
        if todos_error:
            failed.append({'id': uuid, 'error': todos_error})
            continue
        for item in items:
            item['_source'] = title
        all_todos.extend(items)
        succeeded.append({'id': uuid, 'title': title, 'items': items})

    result_data = batch_data(succeeded=succeeded, failed=failed)
    outcome = batch_outcome(result_data)
    records = list(all_todos)
    if outcome != 'success':
        result_data.update({'count': len(records), 'items': records})
    if outcome == 'failure':
        return emit(
            fmt=args.format,
            outcome='failure',
            data=result_data,
            error=failed[0]['error'],
            meta={'children': child_meta} if child_meta else None,
            text='所有听记待办均读取失败',
        )
    if outcome == 'partial_failure' and args.format != 'text':
        return emit(
            fmt=args.format,
            outcome='partial_failure',
            data=result_data,
            meta={'children': child_meta} if child_meta else None,
        )
    if args.format != 'text':
        return emit(
            fmt=args.format,
            outcome='success',
            data={'count': len(records), 'items': records},
            meta={'children': child_meta} if child_meta else None,
        )

    print('\n📋 听记待办汇总')
    print('=' * 50)
    if not records:
        print('  ✅ 暂无待办事项')
    else:
        for item in records:
            print(f"  • {item['content']}")
            if item.get('_source'):
                print(f"    来自: {item['_source']}")
        print(f"\n合计: {len(records)} 条待办")
    if outcome == 'partial_failure':
        return emit(
            fmt=args.format,
            outcome='partial_failure',
            data=result_data,
            meta={'children': child_meta} if child_meta else None,
            text='警告：部分听记待办读取失败；已保留成功项。',
        )
    return 0


if __name__ == '__main__':
    raise SystemExit(run_main(main))
