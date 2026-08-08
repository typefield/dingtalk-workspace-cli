#!/usr/bin/env python3
"""
在指定目录创建文档并写入 Markdown 内容（一键完成）

用法:
    python doc_create_and_write.py \
        --name "项目周报" \
        --content "# 本周总结\n\n## 完成事项\n- 任务A"

    python doc_create_and_write.py \
        --name "会议纪要" \
        --content-file notes.md

    python doc_create_and_write.py \
        --name "知识库文档" --content "# 内容" --folder FOLDER_ID

    python doc_create_and_write.py --name "test" --content "hello" --dry-run
"""

import sys
import argparse
from pathlib import Path
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
    if isinstance(payload, dict):
        return payload.get('data', payload.get('result', payload))
    return payload


def document_id(payload: Any) -> str:
    data = child_data(payload)
    if not isinstance(data, dict):
        return ''
    return str(data.get('nodeId') or data.get('dentryUuid') or data.get('id') or '')


def child_meta(entry_id: str, result: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {'id': entry_id, 'meta': result.meta} if result.meta else None


def main():
    parser = argparse.ArgumentParser(
        description='创建文档并写入内容'
    )
    parser.add_argument('--name', required=True, help='文档名称')
    parser.add_argument('--content', default='', help='Markdown 内容')
    parser.add_argument('--content-file', default='', help='内容文件')
    parser.add_argument('--folder', default='', help='目标文件夹 ID 或 URL')
    parser.add_argument('--workspace', default='', help='目标知识库 ID')
    parser.add_argument(
        '--mode', default='append', choices=['overwrite', 'append'],
        help='写入模式: overwrite=覆盖, append=追加 (默认 append)',
    )
    parser.add_argument(
        '--max-retries', type=int, default=3,
        help='兼容参数；为避免重复写入，文档写入不会自动重试',
    )
    add_contract_flags(parser)
    args = parser.parse_args()

    content = args.content
    if args.content_file:
        p = Path(args.content_file)
        if not p.exists():
            return failure(args.format, f"文件不存在: {p}")
        content = p.read_text(encoding='utf-8')
    if not content:
        return failure(args.format, '需要 --content 或 --content-file')
    chunk_size = 30000

    create_args = ['doc', 'create', '--name', args.name, '--format', 'json']
    if args.folder:
        create_args.extend(['--folder', args.folder])
    if args.workspace:
        create_args.extend(['--workspace', args.workspace])

    if args.max_retries > 1 and not args.dry_run:
        print(
            '⚠️  --max-retries 仅为兼容保留；为避免 append/overwrite 重复写入，脚本不会自动重放 doc update。',
            file=sys.stderr,
        )

    print(f'\n📝 创建文档: {args.name}', file=sys.stderr)
    create = run_dws(create_args, dry_run=args.dry_run)
    meta_entries: list[dict[str, Any]] = []
    if (entry := child_meta('document:create', create)):
        meta_entries.append(entry)
    if create.state != 'success':
        state_channel = 'failed' if create.state == 'failed' else 'unknown'
        create_error = create.error or {'type': 'api', 'message': '创建文档未获得终态结果'}
        channels: dict[str, list[dict[str, Any]]] = {
            'succeeded': [], 'failed': [], 'unknown': [],
        }
        if state_channel == 'failed':
            channels['failed'].append({'id': 'document:create', 'error': create_error})
        else:
            channels['unknown'].append({
                'id': 'document:create',
                'reason': '创建请求未返回可确认终态；文档可能已经创建，请先核查。',
                'error': create_error,
            })
        return emit(
            fmt=args.format,
            outcome='failure',
            data=batch_data(total=1, **channels, name=args.name),
            error=create_error,
            meta={'children': meta_entries} if meta_entries else None,
            text='创建文档未得到可确认结果',
            dry_run=args.dry_run,
        )

    node_id = document_id(create.payload)
    if not args.dry_run and not node_id:
        unknown = {
            'id': 'document:create',
            'reason': '创建调用成功但未返回文档 ID；文档是否已创建需要人工核查。',
            'error': {'type': 'api', 'message': '创建文档响应缺少 nodeId'},
        }
        return emit(
            fmt=args.format,
            outcome='failure',
            data=batch_data(total=1, unknown=[unknown], name=args.name),
            error=unknown['error'],
            meta={'children': meta_entries} if meta_entries else None,
            text='创建文档响应缺少 nodeId，请先核查是否已创建',
        )
    node_id = node_id or '<NODE_ID>'
    print(f"  ✓ 文档已创建 (ID: {node_id})", file=sys.stderr)

    chunks: list[str] = []
    pos = 0
    while pos < len(content):
        end = min(pos + chunk_size, len(content))
        if end < len(content):
            newline_pos = content.rfind('\n', pos, end)
            if newline_pos > pos:
                end = newline_pos + 1
        chunks.append(content[pos:end])
        pos = end
    total_chunks = len(chunks)
    if total_chunks == 1:
        print(f'\n✍️  写入内容 (模式: {"追加" if args.mode == "append" else "覆盖"}, {len(content)} 字符)...', file=sys.stderr)
    else:
        print(f'\n✍️  内容较长 ({len(content)} 字符), 分 {total_chunks} 块写入...', file=sys.stderr)

    succeeded_items: list[dict[str, Any]] = [
        {'id': 'document:create', 'nodeId': node_id, 'data': create.payload},
    ]
    failed_items: list[dict[str, Any]] = []
    unknown_items: list[dict[str, Any]] = []
    for index, chunk in enumerate(chunks):
        chunk_id = f'{node_id}:chunk:{index + 1}'
        chunk_mode = args.mode if index == 0 else 'append'
        write = run_dws([
            'doc', 'update',
            '--node', node_id,
            '--content', chunk,
            '--mode', chunk_mode,
            '--format', 'json',
        ], dry_run=args.dry_run)
        if (entry := child_meta(chunk_id, write)):
            meta_entries.append(entry)
        if write.state == 'success':
            succeeded_items.append({
                'id': chunk_id,
                'chunk': index + 1,
                'characters': len(chunk),
                'data': write.payload,
            })
            print(f"  ✓ 块 {index + 1}/{total_chunks} 已写入 ({len(chunk)} 字符)", file=sys.stderr)
            continue

        error = write.error or {'type': 'api', 'message': '文档写入未获得终态结果'}
        if write.state == 'failed':
            failed_items.append({'id': chunk_id, 'error': error})
        else:
            unknown_items.append({
                'id': chunk_id,
                'reason': '写入请求未返回可确认终态；该块可能已经写入，请先回读文档。',
                'error': error,
            })
        for remaining in range(index + 1, total_chunks):
            failed_items.append({
                'id': f'{node_id}:chunk:{remaining + 1}',
                'error': {
                    'type': 'precondition',
                    'message': '前一块未得到终态结果，后续块未执行。',
                },
            })
        print(f"\n❌ 块 {index + 1}/{total_chunks} 未得到可确认结果；停止后续写入", file=sys.stderr)
        break

    data = batch_data(
        succeeded=succeeded_items,
        failed=failed_items,
        unknown=unknown_items,
        total=1 + total_chunks,
        nodeId=node_id,
        characters=len(content),
        chunks=total_chunks,
    )
    outcome = batch_outcome(data)
    top_error = None
    if outcome == 'failure':
        top_error = (
            failed_items[0]['error'] if failed_items else
            {'type': 'api', 'message': '文档写入未获得任何可确认成功。'}
        )
    return emit(
        fmt=args.format,
        outcome=outcome,
        data=data,
        error=top_error,
        meta={'children': meta_entries} if meta_entries else None,
        text='\n✅ 完成!' if outcome == 'success' else '文档已部分写入或终态未知，请先回读核查',
        dry_run=args.dry_run,
    )


if __name__ == '__main__':
    sys.exit(run_main(main))
