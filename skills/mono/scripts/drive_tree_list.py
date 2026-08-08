#!/usr/bin/env python3
"""
递归列出钉盘目录树结构（可指定深度）

用法:
    python drive_tree_list.py                # 列出根目录
    python drive_tree_list.py --depth 2      # 递归 2 层
    python drive_tree_list.py --folder <id>  # 指定目录 (传 drive list 返回的 fileId)
    python drive_tree_list.py --dry-run

说明:
    `dws drive list` 返回的每个 item 有两个 ID：
      - dentryId：纯数字串，`--folder` 不接受，别用它递归；
      - fileId：字母数字串，即 CLI 所称 dentryUuid，`--folder` 只认它。
    递归子目录必须用 fileId 作为 `--folder` 的值。
"""

import sys
import json
import subprocess
import argparse
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


def list_dir(
    folder: str = '', dry_run: bool = False,
) -> list:
    cmd_args = [
        'drive', 'list', '--limit', '50', '--format', 'json',
    ]
    if folder:
        cmd_args.extend(['--folder', folder])
    data = run_dws(cmd_args, dry_run=dry_run)
    if not data:
        return []
    if isinstance(data, list):
        return data
    if isinstance(data, dict):
        inner = data.get('result', data)
        if isinstance(inner, dict):
            return inner.get('items', inner.get('dentryList', []))
        if isinstance(inner, list):
            return inner
    return []


def print_tree(
    items: list, depth: int, max_depth: int,
    prefix: str = '', dry_run: bool = False,
):
    for i, item in enumerate(items):
        is_last = (i == len(items) - 1)
        connector = '└── ' if is_last else '├── '
        name = item.get('name') or item.get('fileName', '?')
        item_type = item.get('type') or item.get('dentryType', '')
        is_dir = str(item_type).lower() in (
            'folder', 'directory', '1',
        )
        icon = '📁' if is_dir else '📄'
        size_str = ''
        size = item.get('size') or item.get('fileSize')
        if size and not is_dir:
            size = int(size)
            if size > 1024 * 1024:
                size_str = f" ({size / 1024 / 1024:.1f}MB)"
            elif size > 1024:
                size_str = f" ({size / 1024:.1f}KB)"
            else:
                size_str = f" ({size}B)"

        print(f"{prefix}{connector}{icon} {name}{size_str}")

        if is_dir and depth < max_depth:
            child_prefix = prefix + ('    ' if is_last else '│   ')
            # `--folder` 只认 fileId (dentryUuid)，不认纯数字 dentryId
            folder_id = item.get('fileId', '')
            if folder_id:
                children = list_dir(folder_id, dry_run=dry_run)
                print_tree(
                    children, depth + 1, max_depth,
                    child_prefix, dry_run,
                )


def build_tree(items: list, depth: int, max_depth: int) -> list:
    """Build a machine-readable tree without emitting presentation text."""
    output = []
    for item in items:
        node = dict(item) if isinstance(item, dict) else {'value': item}
        item_type = node.get('type') or node.get('dentryType', '')
        is_dir = str(item_type).lower() in ('folder', 'directory', '1')
        folder_id = node.get('fileId', '')
        if is_dir and folder_id and depth < max_depth:
            node['children'] = build_tree(list_dir(folder_id), depth + 1, max_depth)
        output.append(node)
    return output


def main() -> int:
    parser = argparse.ArgumentParser(
        description='递归列出钉盘目录树'
    )
    parser.add_argument(
        '--folder', default='',
        help='起始目录 ID (传 drive list 返回的 fileId)',
    )
    parser.add_argument(
        '--depth', type=int, default=1,
        help='递归深度 (默认 1, 最大 5)',
    )
    add_contract_flags(parser)
    args = parser.parse_args()
    args.depth = min(args.depth, 5)

    root_name = args.folder or '我的文件'
    print(f"📁 {root_name}", file=sys.stderr)

    items = list_dir(args.folder, dry_run=args.dry_run)
    if args.dry_run:
        return emit(fmt=args.format, outcome='success', data={
            'folder': args.folder, 'depth': args.depth,
        }, dry_run=True, text='[dry-run] 将读取钉盘目录并递归展开')
    if not items:
        return emit(fmt=args.format, outcome='success', data={
            'folder': args.folder, 'items': [], 'count': 0,
        }) if args.format != 'text' else 0

    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data={
            'folder': args.folder, 'depth': args.depth,
            'items': build_tree(items, 0, args.depth),
            'count': len(items),
        })

    print_tree(items, 0, args.depth, '', args.dry_run)
    print(f"\n共 {len(items)} 个项目 (根目录)")
    return 0


if __name__ == '__main__':
    sys.exit(run_main(main))
