#!/usr/bin/env python3
"""递归列出钉盘目录树，并诚实表达分页与局部读取失败。"""

from __future__ import annotations

import argparse
import sys
from collections import deque
from dataclasses import dataclass, field
from typing import Any, Optional

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


PAGE_SIZE = 50
MAX_PAGES_PER_FOLDER = 50
MAX_ITEMS = 2000


@dataclass
class FolderWork:
    folder_id: str
    depth: int
    children: list[dict[str, Any]]
    cursor: str = ''
    page: int = 1
    seen_cursors: set[str] = field(default_factory=set)


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


def projection_error(message: str, subtype: str = 'projection_unknown') -> dict[str, Any]:
    return {'type': 'api', 'subtype': subtype, 'message': message}


def is_folder(item: dict[str, Any]) -> bool:
    value = item.get('type') if item.get('type') is not None else item.get('dentryType')
    return str(value).lower() in {'folder', 'directory', '1'}


def stable_file_id(item: dict[str, Any]) -> Optional[str]:
    value = item.get('fileId') or item.get('dentryUuid')
    return value.strip() if isinstance(value, str) and value.strip() else None


def project_page(
    payload: Any,
) -> tuple[list[dict[str, Any]], str, Optional[bool], Optional[dict[str, Any]]]:
    """Project one drive list page without interpreting unknown shapes as empty."""
    value = unwrap_unified(payload)
    if isinstance(value, list):
        rows = value
        container: dict[str, Any] = {}
    elif isinstance(value, dict):
        container = value
        if isinstance(container.get('result'), dict):
            inner = container['result']
            if any(isinstance(inner.get(key), list) for key in ('items', 'dentryList')):
                container = inner
        rows = container.get('items')
        if not isinstance(rows, list):
            rows = container.get('dentryList')
        if not isinstance(rows, list):
            return [], '', None, projection_error('drive list 响应缺少已知 items[]/dentryList[]。')
    else:
        return [], '', None, projection_error('drive list 响应不是可识别的列表对象。')

    items: list[dict[str, Any]] = []
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            return [], '', None, projection_error(f'drive list 第 {index + 1} 项不是对象。')
        item = dict(row)
        name = item.get('name') if item.get('name') is not None else item.get('fileName')
        if not isinstance(name, str) or not name.strip():
            return [], '', None, projection_error(f'drive list 第 {index + 1} 项缺少名称。')
        file_id = stable_file_id(item)
        if file_id is None:
            return [], '', None, projection_error(f'drive list 第 {index + 1} 项缺少稳定 fileId/dentryUuid。')
        item['fileId'] = file_id
        item['name'] = name
        items.append(item)

    token_value = container.get('nextToken') if isinstance(container, dict) else None
    if token_value is None and isinstance(container, dict):
        token_value = container.get('nextCursor')
    if token_value is not None and not isinstance(token_value, str):
        return items, '', None, projection_error('drive list 分页游标不是字符串。', 'pagination_inconsistent')
    token = token_value.strip() if isinstance(token_value, str) else ''
    has_more_value = container.get('hasMore') if isinstance(container, dict) else None
    if has_more_value is not None and not isinstance(has_more_value, bool):
        return items, token, None, projection_error('drive list hasMore 不是 boolean。', 'pagination_inconsistent')
    if has_more_value is True and not token:
        return items, '', True, projection_error(
            'drive list 声明 hasMore=true 但未返回续页游标。', 'pagination_inconsistent',
        )
    return items, token, has_more_value, None


def child_meta(identifier: str, result: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {'id': identifier, 'meta': result.meta} if result.meta else None


def error_entry(identifier: str, error: dict[str, Any]) -> dict[str, Any]:
    return {'id': identifier, 'error': error}


def unknown_entry(identifier: str, error: dict[str, Any]) -> dict[str, Any]:
    return {'id': identifier, 'reason': error.get('message', '读取结果未知。'), 'error': error}


def fetch_tree(
    root_folder: str,
    max_depth: int,
) -> tuple[str, dict[str, Any], Optional[dict[str, Any]], Optional[dict[str, Any]]]:
    root_items: list[dict[str, Any]] = []
    queue: deque[FolderWork] = deque([FolderWork(root_folder, 0, root_items)])
    succeeded: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    unknown: list[dict[str, Any]] = []
    meta_children: list[dict[str, Any]] = []
    total_items = 0
    terminal_error: Optional[dict[str, Any]] = None

    while queue:
        work = queue.popleft()
        page_id = f"folder:{work.folder_id or 'root'}:page:{work.page}"
        args = ['drive', 'list', '--limit', str(PAGE_SIZE), '--format', 'json']
        if work.folder_id:
            args.extend(['--folder', work.folder_id])
        if work.cursor:
            args.extend(['--cursor', work.cursor])
        result = run_dws(args)
        meta_entry = child_meta(page_id, result)
        if meta_entry:
            meta_children.append(meta_entry)
        if result.state != 'success':
            error = result.error or projection_error('drive list 未返回终态成功。')
            terminal_error = terminal_error or error
            if result.state == 'failed':
                failed.append(error_entry(page_id, error))
            else:
                unknown.append(unknown_entry(page_id, error))
            continue

        items, next_token, _, page_error = project_page(result.payload)
        if page_error and not items:
            terminal_error = terminal_error or page_error
            failed.append(error_entry(page_id, page_error))
            continue
        if total_items + len(items) > MAX_ITEMS:
            retained = max(0, MAX_ITEMS - total_items)
            items = items[:retained]
            limit_error = projection_error(
                f'目录树超过全局 {MAX_ITEMS} 项安全上限；当前结果不完整。',
                'pagination_limit_reached',
            )
            terminal_error = terminal_error or limit_error
            unknown.append(unknown_entry(f'folder:{work.folder_id or "root"}:remaining', limit_error))
            next_token = ''

        for item in items:
            node = dict(item)
            work.children.append(node)
            total_items += 1
            if is_folder(node) and work.depth < max_depth:
                children: list[dict[str, Any]] = []
                node['children'] = children
                queue.append(FolderWork(node['fileId'], work.depth + 1, children))

        succeeded.append({
            'id': page_id,
            'folder': work.folder_id,
            'page': work.page,
            'item_count': len(items),
        })
        if page_error:
            terminal_error = terminal_error or page_error
            failed.append(error_entry(f'{page_id}:continuation', page_error))
            continue
        if not next_token:
            continue
        if next_token == work.cursor or next_token in work.seen_cursors:
            cursor_error = projection_error(
                f'目录 {work.folder_id or "root"} 返回重复分页游标。',
                'pagination_inconsistent',
            )
            terminal_error = terminal_error or cursor_error
            failed.append(error_entry(f'folder:{work.folder_id or "root"}:cursor', cursor_error))
            continue
        if work.page >= MAX_PAGES_PER_FOLDER:
            limit_error = projection_error(
                f'目录 {work.folder_id or "root"} 超过 {MAX_PAGES_PER_FOLDER} 页安全上限。',
                'pagination_limit_reached',
            )
            terminal_error = terminal_error or limit_error
            unknown.append(unknown_entry(f'folder:{work.folder_id or "root"}:remaining', limit_error))
            continue
        work.seen_cursors.add(next_token)
        work.cursor = next_token
        work.page += 1
        queue.appendleft(work)

    coverage_complete = not failed and not unknown
    common = {
        'folder': root_folder,
        'depth': max_depth,
        'items': root_items,
        'count': total_items,
        'coverage': {
            'scope': 'requested_tree_depth',
            'requested_depth': max_depth,
            'complete': coverage_complete,
        },
    }
    result_data = batch_data(
        succeeded=succeeded,
        failed=failed,
        unknown=unknown,
        **common,
    )
    outcome = batch_outcome(result_data)
    meta: dict[str, Any] = {
        'pagination': {'endpoint_exhausted': coverage_complete},
    }
    if meta_children:
        meta['children'] = meta_children
    if outcome == 'success':
        return outcome, common, meta, None
    return outcome, result_data, meta, terminal_error


def format_size(item: dict[str, Any]) -> str:
    value = item.get('size') if item.get('size') is not None else item.get('fileSize')
    if is_folder(item) or isinstance(value, bool) or not isinstance(value, (int, float)) or value <= 0:
        return ''
    if value > 1024 * 1024:
        return f' ({value / 1024 / 1024:.1f}MB)'
    if value > 1024:
        return f' ({value / 1024:.1f}KB)'
    return f' ({int(value)}B)'


def render_tree(items: list[dict[str, Any]], prefix: str = '') -> list[str]:
    lines: list[str] = []
    for index, item in enumerate(items):
        is_last = index == len(items) - 1
        lines.append(
            f"{prefix}{'└── ' if is_last else '├── '}{'📁' if is_folder(item) else '📄'} "
            f"{item['name']}{format_size(item)}"
        )
        children = item.get('children')
        if isinstance(children, list):
            lines.extend(render_tree(children, prefix + ('    ' if is_last else '│   ')))
    return lines


def main() -> int:
    parser = argparse.ArgumentParser(description='递归列出钉盘目录树')
    parser.add_argument('--folder', default='', help='起始目录 ID（drive list 返回的 fileId）')
    parser.add_argument('--depth', type=int, default=1, help='递归深度（1..5，默认 1）')
    # Retain the old Multi spelling as a hidden compatibility alias. Agent docs
    # and Help expose only the canonical --folder contract.
    parser.add_argument('--parent-id', dest='folder', help=argparse.SUPPRESS)
    add_contract_flags(parser)
    args = parser.parse_args()
    if args.depth < 1 or args.depth > 5:
        return failure(args.format, '--depth 必须在 1..5 之间')

    if args.dry_run:
        return emit(
            fmt=args.format,
            outcome='success',
            data={
                'folder': args.folder,
                'depth': args.depth,
                'page_size': PAGE_SIZE,
                'max_items': MAX_ITEMS,
                'plan': '逐目录分页读取并递归到请求深度',
            },
            dry_run=True,
            text='[dry-run] 将逐目录分页读取钉盘并递归展开；不会启动子 dws 进程。',
        )

    outcome, data, meta, error = fetch_tree(args.folder, args.depth)
    if args.format != 'text':
        return emit(fmt=args.format, outcome=outcome, data=data, error=error if outcome == 'failure' else None, meta=meta)

    lines = [f"📁 {args.folder or '我的文件'}", *render_tree(data.get('items', []))]
    lines.append(f"共 {data.get('count', 0)} 个项目（请求深度 {args.depth}）")
    return emit(
        fmt=args.format,
        outcome=outcome,
        data=data,
        error=error if outcome == 'failure' else None,
        meta=meta,
        text='\n'.join(lines) if outcome == 'success' else '\n'.join(lines + ['警告：目录树读取不完整。']),
    )


if __name__ == '__main__':
    raise SystemExit(run_main(main))
