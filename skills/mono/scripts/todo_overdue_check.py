#!/usr/bin/env python3
"""
扫描已过截止时间但未完成的待办，输出逾期清单

用法:
    python todo_overdue_check.py
    python todo_overdue_check.py --dry-run
"""

import sys
from datetime import datetime
from typing import List, Dict, Any, Optional

import argparse
from _runtime import (
    ChildDWSResult,
    add_contract_flags,
    batch_data,
    batch_outcome,
    emit,
    run_child_dws,
    run_main,
)

PAGE_SIZE = 50
MAX_PAGES = 10
PRIORITY_MAP = {10: '低', 20: '普通', 30: '较高', 40: '紧急'}


def run_dws(args: List[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def child_data(result: ChildDWSResult) -> Any:
    payload = result.payload
    if (
        isinstance(payload, dict)
        and isinstance(payload.get('ok'), bool)
        and isinstance(payload.get('outcome'), str)
        and 'data' in payload
    ):
        return payload['data']
    return payload


def extract_todo_cards(data: Any) -> Optional[List[Dict[str, Any]]]:
    """兼容两种结构: 顶层 todoCards / {result: {todoCards: [...]}}。
    返回 None 表示结构无法识别或调用失败。"""
    if isinstance(data, list):
        return data
    if not isinstance(data, dict):
        return None
    if data.get('success') is False:
        print(f"错误：todo 查询失败: "
              f"{data.get('errorMsg') or data.get('errorCode') or data}",
              file=sys.stderr)
        return None
    items = data.get('todoCards')
    if items is None:
        inner = data.get('result')
        if isinstance(inner, dict):
            items = inner.get('todoCards')
        elif isinstance(inner, list):
            items = inner
    return items if isinstance(items, list) else None


def fetch_all_undone(dry_run: bool = False) -> Dict[str, Any]:
    """Fetch pages without claiming completeness after an interrupted read."""
    all_todos: List[Dict[str, Any]] = []
    succeeded: List[Dict[str, Any]] = []
    failed: List[Dict[str, Any]] = []
    unknown: List[Dict[str, Any]] = []
    child_meta: List[Dict[str, Any]] = []
    for page in range(1, MAX_PAGES + 1):
        result = run_dws([
            'todo', 'task', 'list',
            '--page', str(page), '--size', str(PAGE_SIZE),
            '--status', 'false', '--format', 'json',
        ], dry_run=dry_run)
        if dry_run:
            return {
                'items': [], 'succeeded': [], 'failed': [], 'unknown': [],
                'meta': [],
            }
        page_id = f'page:{page}'
        if result.meta:
            child_meta.append({'id': page_id, 'meta': result.meta})
        if result.state != 'success':
            error = result.error or {
                'type': 'api',
                'message': '待办分页读取未返回终态成功。',
            }
            if result.state == 'failed':
                failed.append({'id': page_id, 'error': error})
            else:
                unknown.append({
                    'id': page_id,
                    'reason': '待办分页读取结果未知；不得把已读页面当作完整集合。',
                    'error': error,
                })
            break
        data = child_data(result)
        items = extract_todo_cards(data)
        if items is None:
            failed.append({
                'id': page_id,
                'error': {
                    'type': 'api',
                    'subtype': 'projection_unknown',
                    'message': '待办分页响应缺少可识别的 todoCards 列表。',
                },
            })
            break
        succeeded.append({'id': page_id, 'count': len(items), 'items': items})
        if not items:
            break
        all_todos.extend(items)
        if len(items) < PAGE_SIZE:
            break
        if page == MAX_PAGES:
            unknown.append({
                'id': f'page:{page + 1}',
                'reason': f'已达到 {MAX_PAGES} 页安全上限，端点是否耗尽未知。',
            })
    return {
        'items': all_todos,
        'succeeded': succeeded,
        'failed': failed,
        'unknown': unknown,
        'meta': child_meta,
    }


def find_overdue(todos: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    now_ms = int(datetime.now().timestamp() * 1000)
    overdue = []
    for t in todos:
        due = t.get('dueTime') or t.get('due')
        if not due:
            continue
        try:
            if int(due) < now_ms:
                overdue.append(t)
        except (ValueError, TypeError):
            continue
    return overdue


def days_overdue(due_ms) -> int:
    now = datetime.now()
    try:
        due_dt = datetime.fromtimestamp(int(due_ms) / 1000)
        return max(0, (now - due_dt).days)
    except (ValueError, TypeError, OSError):
        return 0


def main() -> int:
    parser = argparse.ArgumentParser(description='扫描逾期待办')
    add_contract_flags(parser)
    args = parser.parse_args()
    dry_run = args.dry_run
    fetched = fetch_all_undone(dry_run=dry_run)
    if dry_run:
        return emit(fmt=args.format, outcome='success', data={
            'plan': 'list unfinished tasks and filter overdue items',
        }, dry_run=True, text='[dry-run] 将查询未完成待办并筛选逾期项')
    todos = fetched['items']
    result_data = batch_data(
        succeeded=fetched['succeeded'],
        failed=fetched['failed'],
        unknown=fetched['unknown'],
    )
    outcome = batch_outcome(result_data)
    if outcome == 'failure':
        first = (fetched['failed'] or fetched['unknown'])[0]
        return emit(
            fmt=args.format,
            outcome='failure',
            data=result_data,
            error=first.get('error') or {
                'type': 'api',
                'message': first.get('reason', '待办查询失败，无法给出逾期结论。'),
            },
            meta={'children': fetched['meta']} if fetched['meta'] else None,
            text='待办查询失败，无法给出逾期结论',
        )

    overdue = find_overdue(todos)
    overdue.sort(
        key=lambda t: int(t.get('dueTime') or t.get('due', 0))
    )

    records = []
    for t in overdue:
        due = t.get('dueTime') or t.get('due')
        records.append({
            'title': t.get('subject') or t.get('title', '无标题'),
            'due': datetime.fromtimestamp(int(due) / 1000).strftime('%Y-%m-%d'),
            'daysOverdue': days_overdue(due),
            'priority': PRIORITY_MAP.get(int(t.get('priority', 20)), '普通'),
        })
    if outcome == 'partial_failure':
        result_data.update({'count': len(records), 'items': records})
        if args.format == 'text':
            print_overdue(records)
        return emit(
            fmt=args.format,
            outcome=outcome,
            data=result_data,
            meta={'children': fetched['meta']} if fetched['meta'] else None,
            text='警告：仅完成部分待办分页读取；请按失败或未知页面继续核查。',
        )
    if args.format != 'text':
        return emit(
            fmt=args.format,
            outcome='success',
            data={'count': len(records), 'items': records},
            meta={'children': fetched['meta']} if fetched['meta'] else None,
        )

    print_overdue(records)
    return 1 if overdue else 0


def print_overdue(records: List[Dict[str, Any]]) -> None:
    print(f"\n⏰ 逾期待办检查 ({datetime.now().strftime('%Y-%m-%d %H:%M')})")
    print('=' * 50)

    if not records:
        print('  ✅ 没有逾期待办，继续保持！')
        return

    for item in records:
        print(f"  🔴 [{item['priority']}] {item['title']}")
        print(f"     截止: {item['due']}  逾期: {item['daysOverdue']} 天")

    print(f"\n合计: {len(records)} 条逾期待办")


if __name__ == '__main__':
    sys.exit(run_main(main))
