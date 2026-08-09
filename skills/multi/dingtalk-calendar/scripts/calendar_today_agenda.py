#!/usr/bin/env python3
"""查看今天/明天/本周日程；未知响应不会伪装为空日程。"""

from __future__ import annotations

import argparse
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Optional

_SHARED_RUNTIME = Path(__file__).resolve().parents[2] / 'dingtalk-shared' / 'scripts'
sys.path.insert(0, str(_SHARED_RUNTIME))

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main


TZ = timezone(timedelta(hours=8))


def run_dws(args: list[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def get_range(scope: str) -> tuple[datetime, datetime]:
    now = datetime.now(TZ)
    today = now.replace(hour=0, minute=0, second=0, microsecond=0)
    if scope == 'tomorrow':
        value = today + timedelta(days=1)
        return value, value + timedelta(days=1)
    if scope == 'week':
        value = today - timedelta(days=today.weekday())
        return value, value + timedelta(days=7)
    return today, today + timedelta(days=1)


def fmt_iso(value: datetime) -> str:
    return value.strftime('%Y-%m-%dT%H:%M:%S+08:00')


def fmt_time(value: str) -> str:
    if not value:
        return '??:??'
    try:
        return datetime.fromisoformat(value.replace('Z', '+00:00')).strftime('%H:%M')
    except ValueError:
        return value[:16]


def projection_error(message: str) -> dict[str, Any]:
    return {'type': 'api', 'subtype': 'projection_unknown', 'message': message}


def unwrap_unified(payload: Any) -> Any:
    if (
        isinstance(payload, dict)
        and isinstance(payload.get('ok'), bool)
        and isinstance(payload.get('outcome'), str)
        and 'data' in payload
    ):
        return payload['data']
    return payload


def time_value(value: Any, label: str) -> tuple[str, Optional[dict[str, Any]]]:
    if isinstance(value, str):
        return value, None
    if isinstance(value, dict):
        candidate = value.get('dateTime') or value.get('date')
        if isinstance(candidate, str):
            return candidate, None
    return '', projection_error(f'日程 {label} 时间类型不可识别。')


def project_events(payload: Any) -> tuple[list[dict[str, str]], Optional[dict[str, Any]]]:
    value = unwrap_unified(payload)
    if isinstance(value, list):
        events = value
    elif isinstance(value, dict) and isinstance(value.get('events'), list):
        events = value['events']
    elif isinstance(value, dict) and isinstance(value.get('result'), list):
        events = value['result']
    elif (
        isinstance(value, dict)
        and isinstance(value.get('result'), dict)
        and isinstance(value['result'].get('events'), list)
    ):
        events = value['result']['events']
    else:
        return [], projection_error('日程列表响应缺少已知 events[]。')

    records: list[dict[str, str]] = []
    for index, event in enumerate(events):
        if not isinstance(event, dict):
            return [], projection_error(f'日程列表第 {index + 1} 项不是对象。')
        title = event.get('summary') or event.get('title') or '无标题'
        if not isinstance(title, str):
            return [], projection_error(f'日程列表第 {index + 1} 项标题不是字符串。')
        start, error = time_value(event.get('start'), '开始')
        if error:
            return [], error
        end, error = time_value(event.get('end'), '结束')
        if error:
            return [], error
        location = event.get('location', '')
        if isinstance(location, dict):
            location = location.get('displayName', '')
        if not isinstance(location, str):
            return [], projection_error(f'日程列表第 {index + 1} 项地点类型不可识别。')
        records.append({'title': title, 'start': start, 'end': end, 'location': location})
    return records, None


def main() -> int:
    parser = argparse.ArgumentParser(description='查看日程安排')
    parser.add_argument('scope', nargs='?', default='today', choices=['today', 'tomorrow', 'week'])
    add_contract_flags(parser)
    args = parser.parse_args()
    start, end = get_range(args.scope)
    if args.dry_run:
        run_dws([
            'calendar', 'event', 'list', '--start', fmt_iso(start), '--end', fmt_iso(end),
            '--format', 'json',
        ], dry_run=True)
        return emit(
            fmt=args.format,
            outcome='success',
            data={'scope': args.scope, 'start': fmt_iso(start), 'end': fmt_iso(end)},
            dry_run=True,
            text='[dry-run] 将查询日程列表',
        )

    result = run_dws([
        'calendar', 'event', 'list', '--start', fmt_iso(start), '--end', fmt_iso(end),
        '--format', 'json',
    ])
    meta = {'children': [{'id': 'event:list', 'meta': result.meta}]} if result.meta else None
    if result.state != 'success':
        return emit(
            fmt=args.format,
            outcome='failure',
            error=result.error or projection_error('日程列表查询失败。'),
            meta=meta,
            text='日程列表查询失败',
        )
    records, error = project_events(result.payload)
    if error:
        return emit(fmt=args.format, outcome='failure', error=error, meta=meta, text='日程列表响应无法可靠解析')

    data = {
        'scope': args.scope,
        'count': len(records),
        'items': records,
        'start': fmt_iso(start),
        'end': fmt_iso(end),
    }
    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data=data, meta=meta)

    label = {'today': '今天', 'tomorrow': '明天', 'week': '本周'}[args.scope]
    print(f"\n📅 {label}日程 ({start.strftime('%m-%d')} ~ {end.strftime('%m-%d')})")
    print('=' * 50)
    if not records:
        print('  ✅ 暂无日程，自由安排！')
        return 0
    for record in records:
        line = f"  🕐 {fmt_time(record['start'])}-{fmt_time(record['end'])}  {record['title']}"
        if record['location']:
            line += f"  📍{record['location']}"
        print(line)
    print(f"\n合计: {len(records)} 场日程")
    return 0


if __name__ == '__main__':
    raise SystemExit(run_main(main))
