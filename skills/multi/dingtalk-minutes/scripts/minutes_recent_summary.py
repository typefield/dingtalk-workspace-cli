#!/usr/bin/env python3
"""获取最近 N 条听记的 AI 摘要，并诚实保留逐条读取结果。"""

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

from minutes_list_parse import project_summary_text, project_uuid_title_pairs, unwrap_child_data
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


def render_markdown(records: list[dict[str, str]], failed_ids: set[str]) -> str:
    lines = [f"# 最近 {len(records) + len(failed_ids)} 条听记摘要\n"]
    for index, record in enumerate(records, 1):
        lines.append(f"## {index}. {record['title']}\n")
        lines.append(f"{record['summary'] or '(暂无摘要)'}\n")
    if failed_ids:
        lines.append("## 未完成\n")
        lines.extend(f"- `{identifier}`：摘要读取失败\n" for identifier in sorted(failed_ids))
    return '\n'.join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description='获取最近听记的 AI 摘要')
    parser.add_argument('--max', type=int, default=5, help='获取条数 (默认 5)')
    parser.add_argument('--output', default='', help='输出到 Markdown 文件')
    add_contract_flags(parser)
    args = parser.parse_args()
    if args.max <= 0:
        return failure(args.format, '--max 必须大于 0')

    print('🎙️ 获取听记列表...', file=sys.stderr)
    list_result = run_dws([
        'minutes', 'list', 'mine', '--limit', str(args.max), '--format', 'json',
    ], dry_run=args.dry_run)
    if args.dry_run:
        run_dws([
            'minutes', 'get', 'summary', '--id', '<TASK_UUID>', '--format', 'json',
        ], dry_run=True)
        return emit(
            fmt=args.format,
            outcome='success',
            data={'limit': args.max, 'output': args.output or None},
            dry_run=True,
            text='[dry-run] 将读取听记列表并逐条获取摘要',
        )

    child_meta: list[dict[str, Any]] = []
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
    if not pairs:
        return emit(
            fmt=args.format,
            outcome='success',
            data={'count': 0, 'items': []},
            meta={'children': child_meta} if child_meta else None,
            text='暂无听记',
        )

    records: list[dict[str, str]] = []
    succeeded: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    for index, (uuid, title) in enumerate(pairs, 1):
        print(f"  [{index}/{len(pairs)}] 获取摘要: {title}", file=sys.stderr)
        summary_result = run_dws([
            'minutes', 'get', 'summary', '--id', uuid, '--format', 'json',
        ])
        summary_meta = child_meta_entry(f'summary:{uuid}', summary_result)
        if summary_meta:
            child_meta.append(summary_meta)
        if summary_result.state != 'success':
            failed.append({'id': uuid, 'error': summary_result.error or {'type': 'api', 'message': '听记摘要读取失败。'}})
            continue
        summary_text, summary_error = project_summary_text(summary_result.payload)
        if summary_error:
            failed.append({'id': uuid, 'error': summary_error})
            continue
        record = {'id': uuid, 'title': title, 'summary': summary_text}
        records.append(record)
        succeeded.append(record)

    result_data = batch_data(succeeded=succeeded, failed=failed)
    outcome = batch_outcome(result_data)
    if outcome != 'success':
        result_data.update({'count': len(records), 'items': records})
    if outcome == 'failure':
        return emit(
            fmt=args.format,
            outcome='failure',
            data=result_data,
            error=failed[0]['error'],
            meta={'children': child_meta} if child_meta else None,
            text='所有听记摘要均读取失败',
        )

    full_output = render_markdown(records, {entry['id'] for entry in failed})
    if outcome == 'partial_failure':
        if args.format == 'text':
            if args.output:
                Path(args.output).write_text(full_output, encoding='utf-8')
                print(f"\n✓ 已输出部分结果到 {args.output}", file=sys.stderr)
            else:
                print('\n' + full_output)
        return emit(
            fmt=args.format,
            outcome='partial_failure',
            data=result_data,
            meta={'children': child_meta} if child_meta else None,
            text='警告：部分听记摘要读取失败；已保留成功项。',
        )

    if args.format != 'text':
        return emit(
            fmt=args.format,
            outcome='success',
            data={'count': len(records), 'items': records},
            meta={'children': child_meta} if child_meta else None,
        )
    if args.output:
        Path(args.output).write_text(full_output, encoding='utf-8')
        print(f"\n✓ 已输出到 {args.output}", file=sys.stderr)
    else:
        print('\n' + full_output)
    return 0


if __name__ == '__main__':
    raise SystemExit(run_main(main))
