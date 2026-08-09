#!/usr/bin/env python3
"""
查询今天/明天/本周未完成的待办并汇总输出

用法:
    python todo_daily_summary.py              # 默认查今天
    python todo_daily_summary.py today        # 今天的待办
    python todo_daily_summary.py tomorrow     # 明天的待办
    python todo_daily_summary.py week         # 本周的待办
    python todo_daily_summary.py --dry-run    # 仅显示将执行的命令
"""

import sys
import argparse
from datetime import datetime, timedelta
from typing import List, Dict, Any, Optional

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

PRIORITY_MAP = {10: '低', 20: '普通', 30: '较高', 40: '紧急'}
PAGE_SIZE = 50
MAX_PAGES = 10


def run_dws(args: List[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def child_data(result: ChildDWSResult) -> Any:
    """Unwrap unified child data while preserving legacy bare payloads."""
    payload = result.payload
    if (
        isinstance(payload, dict)
        and isinstance(payload.get('ok'), bool)
        and isinstance(payload.get('outcome'), str)
        and 'data' in payload
    ):
        return payload['data']
    return payload


def get_date_range(scope: str):
    now = datetime.now()
    today_start = now.replace(hour=0, minute=0, second=0, microsecond=0)
    if scope == 'today':
        return today_start, today_start + timedelta(days=1)
    elif scope == 'tomorrow':
        tmr = today_start + timedelta(days=1)
        return tmr, tmr + timedelta(days=1)
    elif scope == 'week':
        week_start = today_start - timedelta(days=today_start.weekday())
        return week_start, week_start + timedelta(days=7)
    return today_start, today_start + timedelta(days=1)


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


def fetch_all_todos(dry_run: bool = False) -> Dict[str, Any]:
    """Fetch pages without turning an interrupted traversal into completeness."""
    all_todos: List[Dict[str, Any]] = []
    succeeded: List[Dict[str, Any]] = []
    failed: List[Dict[str, Any]] = []
    unknown: List[Dict[str, Any]] = []
    child_meta: List[Dict[str, Any]] = []
    for page in range(1, MAX_PAGES + 1):
        result = run_dws([
            'todo', 'task', 'list',
            '--page', str(page),
            '--size', str(PAGE_SIZE),
            '--status', 'false',
            '--format', 'json',
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


def format_priority(p) -> str:
    try:
        return PRIORITY_MAP.get(int(p), str(p))
    except (ValueError, TypeError):
        return '普通'


def format_due(due_ms) -> str:
    if not due_ms:
        return '无截止时间'
    try:
        dt = datetime.fromtimestamp(int(due_ms) / 1000)
        return dt.strftime('%Y-%m-%d %H:%M')
    except (ValueError, TypeError, OSError):
        return str(due_ms)


def filter_by_due(
    todos: List[Dict[str, Any]], start: datetime, end: datetime,
) -> List[Dict[str, Any]]:
    start_ms = int(start.timestamp() * 1000)
    end_ms = int(end.timestamp() * 1000)
    result = []
    for t in todos:
        due = t.get('dueTime') or t.get('due')
        if not due:
            result.append(t)
            continue
        try:
            due_val = int(due)
            if start_ms <= due_val < end_ms:
                result.append(t)
        except (ValueError, TypeError):
            result.append(t)
    return result


def print_summary(
    todos: List[Dict[str, Any]], scope: str,
    start: datetime, end: datetime,
):
    scope_label = {
        'today': '今天', 'tomorrow': '明天', 'week': '本周',
    }.get(scope, scope)
    print(f"\n📋 {scope_label}未完成待办 "
          f"({start.strftime('%m-%d')} ~ {end.strftime('%m-%d')})")
    print('=' * 50)
    if not todos:
        print('  ✅ 暂无待办，轻松一下！')
        return
    urgent = [t for t in todos if format_priority(
        t.get('priority')) == '紧急']
    if urgent:
        print(f"\n🔴 紧急 ({len(urgent)} 条)")
        for t in urgent:
            title = t.get('subject') or t.get('title', '无标题')
            print(f"  • {title}  ⏰ {format_due(t.get('dueTime'))}")
    normal = [t for t in todos if t not in urgent]
    if normal:
        print(f"\n📌 其他 ({len(normal)} 条)")
        for t in normal:
            title = t.get('subject') or t.get('title', '无标题')
            pri = format_priority(t.get('priority'))
            print(f"  • [{pri}] {title}  ⏰ {format_due(t.get('dueTime'))}")
    print(f"\n合计: {len(todos)} 条待办")


def main() -> int:
    parser = argparse.ArgumentParser(description='汇总日期范围内未完成待办')
    parser.add_argument('scope', nargs='?', default='today',
                        choices=['today', 'tomorrow', 'week'])
    add_contract_flags(parser)
    args = parser.parse_args()
    dry_run = args.dry_run
    scope = args.scope
    if scope not in ('today', 'tomorrow', 'week'):
        return failure(args.format, f'不支持的范围: {scope}')
    start, end = get_date_range(scope)
    fetched = fetch_all_todos(dry_run=dry_run)
    if dry_run:
        return emit(fmt=args.format, outcome='success', data={
            'scope': scope, 'start': start.isoformat(), 'end': end.isoformat(),
        }, dry_run=True, text='[dry-run] 将查询未完成待办并按截止时间筛选')
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
                'message': first.get('reason', '待办查询失败，无法给出汇总结论。'),
            },
            meta={'children': fetched['meta']} if fetched['meta'] else None,
            text='待办查询失败，无法给出汇总结论',
        )
    filtered = filter_by_due(todos, start, end)
    items = [{
        'title': t.get('subject') or t.get('title', '无标题'),
        'priority': format_priority(t.get('priority')),
        'due': format_due(t.get('dueTime') or t.get('due')),
    } for t in filtered]
    if outcome == 'partial_failure':
        result_data.update({
            'scope': scope,
            'count': len(items),
            'items': items,
        })
        if args.format == 'text':
            print_summary(filtered, scope, start, end)
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
            data={'scope': scope, 'count': len(items), 'items': items},
            meta={'children': fetched['meta']} if fetched['meta'] else None,
        )
    print_summary(filtered, scope, start, end)
    return 0


if __name__ == '__main__':
    sys.exit(run_main(main))
