#!/usr/bin/env python3
"""
查看今天/明天/本周的日程安排

用法:
    python calendar_today_agenda.py              # 今天
    python calendar_today_agenda.py today        # 今天
    python calendar_today_agenda.py tomorrow     # 明天
    python calendar_today_agenda.py week         # 本周
    python calendar_today_agenda.py --dry-run    # 仅显示命令
"""

import sys
import json
import subprocess
import argparse
from datetime import datetime, timedelta, timezone
from typing import List, Dict, Any, Optional

from _runtime import add_contract_flags, emit, failure

TZ = timezone(timedelta(hours=8))


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


def get_range(scope: str):
    now = datetime.now(TZ)
    today = now.replace(hour=0, minute=0, second=0, microsecond=0)
    if scope == 'today':
        return today, today + timedelta(days=1)
    elif scope == 'tomorrow':
        t = today + timedelta(days=1)
        return t, t + timedelta(days=1)
    elif scope == 'week':
        ws = today - timedelta(days=today.weekday())
        return ws, ws + timedelta(days=7)
    return today, today + timedelta(days=1)


def fmt_iso(dt: datetime) -> str:
    return dt.strftime('%Y-%m-%dT%H:%M:%S+08:00')


def fmt_time(iso_str: str) -> str:
    if not iso_str:
        return '??:??'
    try:
        for fmt in ('%Y-%m-%dT%H:%M:%S%z', '%Y-%m-%dT%H:%M:%S'):
            try:
                dt = datetime.strptime(iso_str, fmt)
                return dt.strftime('%H:%M')
            except ValueError:
                continue
        return iso_str[:16]
    except Exception:
        return iso_str[:16]


def main() -> int:
    parser = argparse.ArgumentParser(description='查看日程安排')
    parser.add_argument('scope', nargs='?', default='today',
                        choices=['today', 'tomorrow', 'week'])
    add_contract_flags(parser)
    args = parser.parse_args()
    dry_run = args.dry_run
    scope = args.scope
    if scope not in ('today', 'tomorrow', 'week'):
        return failure(args.format, f'不支持的范围: {scope}')

    start, end = get_range(scope)
    data = run_dws([
        'calendar', 'event', 'list',
        '--start', fmt_iso(start),
        '--end', fmt_iso(end),
        '--format', 'json',
    ], dry_run=dry_run)
    if dry_run:
        return emit(fmt=args.format, outcome='success', data={
            'scope': scope, 'start': fmt_iso(start), 'end': fmt_iso(end),
        }, dry_run=True, text='[dry-run] 将查询日程列表')

    # 兼容两种结构: 顶层 events / {result: {events: [...]}}
    events = []
    if isinstance(data, list):
        events = data
    elif isinstance(data, dict):
        events = data.get('events')
        if events is None:
            inner = data.get('result')
            if isinstance(inner, list):
                events = inner
            elif isinstance(inner, dict):
                events = inner.get('events', [])
    if not isinstance(events, list):
        events = []

    label = {'today': '今天', 'tomorrow': '明天', 'week': '本周'
             }.get(scope, scope)
    records = []
    for event in events:
        title = event.get('summary') or event.get('title', '无标题')
        start_obj = event.get('start', {})
        end_obj = event.get('end', {})
        records.append({
            'title': title,
            'start': start_obj.get('dateTime', '') if isinstance(start_obj, dict) else str(start_obj),
            'end': end_obj.get('dateTime', '') if isinstance(end_obj, dict) else str(end_obj),
            'location': (event.get('location', {}).get('displayName', '')
                         if isinstance(event.get('location'), dict) else str(event.get('location', '') or '')),
        })
    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data={
            'scope': scope, 'count': len(records), 'items': records,
            'start': fmt_iso(start), 'end': fmt_iso(end),
        })

    print(f"\n📅 {label}日程 ({start.strftime('%m-%d')} ~ "
          f"{end.strftime('%m-%d')})")
    print('=' * 50)

    if not events:
        print('  ✅ 暂无日程，自由安排！')
        return 0

    for record in records:
        title = record['title']
        s = record['start']
        ed = record['end']
        start_t = fmt_time(
            s
        )
        end_t = fmt_time(
            ed
        )
        loc_str = record['location']
        line = f"  🕐 {start_t}-{end_t}  {title}"
        if loc_str:
            line += f"  📍{loc_str}"
        print(line)

    print(f"\n合计: {len(events)} 场日程")
    return 0


if __name__ == '__main__':
    sys.exit(main())
