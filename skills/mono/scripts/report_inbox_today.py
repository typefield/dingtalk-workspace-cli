#!/usr/bin/env python3
"""查询收到的日志；分页与逐条详情失败都保留为结构化结果。"""

from __future__ import annotations

import argparse
import shlex
import sys
from datetime import datetime, timedelta
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


PAGE_SIZE = 20
MAX_PAGES = 100


def run_dws(args: list[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def iso_start(value: datetime) -> str:
    return value.strftime('%Y-%m-%dT00:00:00+08:00')


def iso_end(value: datetime) -> str:
    return value.strftime('%Y-%m-%dT23:59:59+08:00')


def unwrap_child_data(payload: Any) -> Any:
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


def report_id_from_command(command: Any) -> str:
    if not isinstance(command, str) or not command.strip():
        return ''
    try:
        parts = shlex.split(command)
    except ValueError:
        return ''
    if '--report-id' not in parts:
        return ''
    index = parts.index('--report-id')
    if index + 1 >= len(parts) or not parts[index + 1].strip():
        return ''
    return parts[index + 1].strip()


def project_list_page(payload: Any) -> tuple[list[dict[str, Any]], bool, Optional[Any], Optional[dict[str, Any]]]:
    """Project one known report page without inventing exhaustion or IDs."""
    body = unwrap_child_data(payload)
    if not isinstance(body, dict):
        return [], False, None, projection_error('日志列表响应不是对象。')
    items = body.get('result')
    commands = body.get('_internalDetailCommands', [])
    if not isinstance(items, list):
        return [], False, None, projection_error('日志列表响应缺少 result[]。')
    if not isinstance(commands, list):
        return [], False, None, projection_error('日志列表详情命令不是数组。')
    if commands and len(commands) != len(items):
        return [], False, None, projection_error(
            'result[] 与 _internalDetailCommands[] 无法逐项对应。',
        )
    if not isinstance(body.get('hasMore'), bool):
        return [], False, None, projection_error('日志列表缺少布尔 hasMore。', 'pagination_inconsistent')
    has_more = body['hasMore']
    next_cursor = body.get('nextCursor')
    if has_more and (next_cursor is None or next_cursor == ''):
        return [], False, None, projection_error('hasMore=true 但缺少 nextCursor。', 'pagination_inconsistent')

    records: list[dict[str, Any]] = []
    for index, item in enumerate(items):
        if not isinstance(item, dict):
            return [], False, None, projection_error(f'日志列表第 {index + 1} 项不是对象。')
        report_id = ''
        if commands:
            command_item = commands[index]
            if not isinstance(command_item, dict):
                return [], False, None, projection_error(f'第 {index + 1} 条详情命令不是对象。')
            report_id = report_id_from_command(command_item.get('command'))
            if not report_id:
                return [], False, None, projection_error(f'第 {index + 1} 条详情命令缺少 reportId。')
        records.append({
            'reportId': report_id,
            'title': str(item.get('标题') or '日志'),
            'sender': str(item.get('发送人') or '未知'),
            'date': str(item.get('日期') or ''),
            'status': str(item.get('状态') or ''),
            'link': str(item.get('钉钉链接') or ''),
        })
    return records, not has_more, next_cursor, None


def project_detail(payload: Any) -> tuple[list[dict[str, str]], Optional[dict[str, Any]]]:
    body = unwrap_child_data(payload)
    if not isinstance(body, dict):
        return [], projection_error('日志详情响应不是对象。')
    result = body.get('result', body)
    if not isinstance(result, dict):
        return [], projection_error('日志详情 result 不是对象。')
    raw = None
    for key in ('report_content', 'contents', 'reportContent', 'reportContents'):
        if key in result:
            raw = result[key]
            break
    if not isinstance(raw, list):
        return [], projection_error('日志详情缺少已知内容数组。')
    contents: list[dict[str, str]] = []
    for index, item in enumerate(raw):
        if not isinstance(item, dict):
            return [], projection_error(f'日志详情第 {index + 1} 项不是对象。')
        key = item.get('key') or item.get('title')
        value = item.get('value') or item.get('content') or item.get('text')
        if not isinstance(key, str) or not isinstance(value, str):
            return [], projection_error(f'日志详情第 {index + 1} 项缺少字符串 key/value。')
        contents.append({'key': key.strip(), 'value': value.strip()})
    return contents, None


def child_meta(identifier: str, result: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {'id': identifier, 'meta': result.meta} if result.meta else None


def main() -> int:
    parser = argparse.ArgumentParser(description='查看收到的日志')
    parser.add_argument('--days', type=int, default=1, help='查询天数 (默认 1，即今天)')
    parser.add_argument('--detail', action='store_true', help='额外拉取每条正文')
    add_contract_flags(parser)
    args = parser.parse_args()
    if args.days <= 0:
        return failure(args.format, '--days 必须大于 0')

    now = datetime.now()
    start = iso_start(now - timedelta(days=args.days - 1))
    end = iso_end(now)
    label = '今天' if args.days == 1 else f'最近 {args.days} 天'
    print(f'查看{label}收到的日志...', file=sys.stderr)

    if args.dry_run:
        run_dws([
            'report', 'inbox', 'list', '--start', start, '--end', end,
            '--cursor', '0', '--size', str(PAGE_SIZE), '--format', 'json',
        ], dry_run=True)
        if args.detail:
            run_dws([
                'report', 'entry', 'get', '--report-id', '<REPORT_ID>', '--format', 'json',
            ], dry_run=True)
        return emit(
            fmt=args.format,
            outcome='success',
            data={'days': args.days, 'detail': args.detail, 'start': start, 'end': end},
            dry_run=True,
            text='[dry-run] 将分页查询收到的日志并按需逐条读取正文',
        )

    records: list[dict[str, Any]] = []
    failed: list[dict[str, Any]] = []
    meta_children: list[dict[str, Any]] = []
    cursor: Any = 0
    seen: set[str] = set()
    endpoint_exhausted = False
    for page in range(1, MAX_PAGES + 1):
        cursor_key = str(cursor)
        if cursor_key in seen:
            failed.append({'id': f'page:{page}', 'error': projection_error('日志分页 cursor 出现循环。', 'pagination_inconsistent')})
            break
        seen.add(cursor_key)
        result = run_dws([
            'report', 'inbox', 'list', '--start', start, '--end', end,
            '--cursor', cursor_key, '--size', str(PAGE_SIZE), '--format', 'json',
        ])
        meta_entry = child_meta(f'list:{page}', result)
        if meta_entry:
            meta_children.append(meta_entry)
        if result.state != 'success':
            failed.append({'id': f'page:{page}', 'error': result.error or projection_error('日志列表读取失败。')})
            break
        page_records, exhausted, next_cursor, error = project_list_page(result.payload)
        if error:
            failed.append({'id': f'page:{page}', 'error': error})
            break
        records.extend(page_records)
        if exhausted:
            endpoint_exhausted = True
            break
        cursor = next_cursor
    else:
        failed.append({'id': f'page:{MAX_PAGES + 1}', 'error': projection_error(f'日志分页超过硬上限 {MAX_PAGES}。', 'pagination_inconsistent')})

    if not records and failed:
        return emit(
            fmt=args.format,
            outcome='failure',
            data={'count': 0, 'items': [], 'start': start, 'end': end},
            error=failed[0]['error'],
            meta={'children': meta_children, 'pagination': {'endpoint_exhausted': False}},
            text='日志列表读取失败',
        )

    succeeded: list[dict[str, Any]] = []
    if args.detail:
        for index, record in enumerate(records, 1):
            report_id = record['reportId']
            if not report_id:
                failed.append({'id': f'detail:{index}', 'error': projection_error('日志列表未提供可用 reportId，无法读取正文。')})
                continue
            result = run_dws([
                'report', 'entry', 'get', '--report-id', report_id, '--format', 'json',
            ])
            meta_entry = child_meta(f'detail:{report_id}', result)
            if meta_entry:
                meta_children.append(meta_entry)
            if result.state != 'success':
                failed.append({'id': report_id, 'error': result.error or projection_error('日志详情读取失败。')})
                continue
            contents, error = project_detail(result.payload)
            if error:
                failed.append({'id': report_id, 'error': error})
                continue
            record['detail'] = contents
            succeeded.append({'id': report_id, **record})
    else:
        succeeded = [
            {'id': record['reportId'] or f'row:{index}', **record}
            for index, record in enumerate(records, 1)
        ]

    result_data = batch_data(
        succeeded=succeeded,
        failed=failed,
        count=len(records),
        items=records,
        start=start,
        end=end,
    )
    outcome = batch_outcome(result_data)
    meta = {
        'children': meta_children,
        'pagination': {'endpoint_exhausted': endpoint_exhausted},
    }
    if outcome == 'failure':
        return emit(
            fmt=args.format,
            outcome='failure',
            data=result_data,
            error=failed[0]['error'],
            meta=meta,
            text='日志详情均读取失败',
        )
    if args.format != 'text':
        if outcome == 'success':
            data = {'count': len(records), 'items': records, 'start': start, 'end': end}
        else:
            data = result_data
        return emit(fmt=args.format, outcome=outcome, data=data, meta=meta)

    print(f'{label}日志 ({len(records)} 条)')
    print('=' * 50)
    for record in records:
        print(f"\n  {record['title']} - {record['sender']}")
        print(f"     时间: {record['date']}")
        if record['status']:
            print(f"     状态: {record['status']}")
        if record['link']:
            print(f"     链接: {record['link']}")
        for content in record.get('detail', [])[:3]:
            print(f"     {content['key']}: {content['value'][:60]}")
    if outcome == 'partial_failure':
        return emit(
            fmt=args.format,
            outcome='partial_failure',
            data=result_data,
            meta=meta,
            text='警告：日志列表或详情未完整读取；已保留成功项。',
        )
    return 0


if __name__ == '__main__':
    raise SystemExit(run_main(main))
