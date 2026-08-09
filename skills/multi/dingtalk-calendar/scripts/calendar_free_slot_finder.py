#!/usr/bin/env python3
"""查询多人共同空闲；参与人覆盖或忙闲投影未知时拒绝推荐。"""

from __future__ import annotations

import argparse
import sys
from datetime import datetime, timezone, timedelta
from pathlib import Path
from typing import Any, Optional

_SHARED_RUNTIME = Path(__file__).resolve().parents[2] / 'dingtalk-shared' / 'scripts'
sys.path.insert(0, str(_SHARED_RUNTIME))

from _runtime import ChildDWSResult, add_contract_flags, emit, failure, run_child_dws, run_main


TZ = timezone(timedelta(hours=8))


def run_dws(args: list[str], dry_run: bool = False) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def fmt_iso(value: datetime) -> str:
    return value.strftime('%Y-%m-%dT%H:%M:%S+08:00')


def projection_error(message: str, subtype: str = 'projection_unknown') -> dict[str, Any]:
    return {'type': 'api', 'subtype': subtype, 'message': message}


def unwrap_unified(payload: Any) -> Any:
    if (
        isinstance(payload, dict)
        and isinstance(payload.get('ok'), bool)
        and isinstance(payload.get('outcome'), str)
        and 'data' in payload
    ):
        return payload['data']
    return payload


def parse_time(value: Any) -> Optional[datetime]:
    if isinstance(value, dict):
        value = value.get('dateTime') or value.get('date')
    if not isinstance(value, str) or not value.strip():
        return None
    try:
        result = datetime.fromisoformat(value.replace('Z', '+00:00'))
    except ValueError:
        return None
    return result.replace(tzinfo=TZ) if result.tzinfo is None else result.astimezone(TZ)


def interval_from_item(item: Any, index: int) -> tuple[Optional[tuple[datetime, datetime]], Optional[dict[str, Any]]]:
    if not isinstance(item, dict):
        return None, projection_error(f'忙闲第 {index} 项不是对象。')
    if str(item.get('status') or '').upper() == 'FREE':
        return None, None
    start = parse_time(item.get('startTime') or item.get('start'))
    end = parse_time(item.get('endTime') or item.get('end'))
    if start is None or end is None:
        return None, projection_error(f'忙闲第 {index} 项缺少可解析的开始/结束时间。')
    if end <= start:
        return None, projection_error(f'忙闲第 {index} 项结束时间不晚于开始时间。')
    return (start, end), None


def project_busy_intervals(
    payload: Any,
    expected_users: list[str],
) -> tuple[list[tuple[datetime, datetime]], Optional[dict[str, Any]]]:
    """Accept known combined or per-user shapes and prove participant coverage."""
    value = unwrap_unified(payload)
    if isinstance(value, dict) and 'result' in value:
        value = value['result']

    rows: list[Any]
    coverage: Optional[set[str]] = None
    if isinstance(value, list):
        rows = value
        declared = {
            str(row.get('userId')).strip()
            for row in rows
            if isinstance(row, dict) and isinstance(row.get('userId'), str) and row.get('userId').strip()
        }
        if declared:
            coverage = declared
    elif isinstance(value, dict):
        if not value and expected_users:
            return [], projection_error('忙闲响应为空对象，无法证明参与人覆盖。', 'coverage_unknown')
        rows = []
        coverage = set()
        for user_id, row in value.items():
            if user_id in expected_users:
                coverage.add(user_id)
            rows.append(row)
    else:
        return [], projection_error('忙闲响应不是已知列表或用户映射。')

    if coverage is not None:
        missing = [user for user in expected_users if user not in coverage]
        if missing:
            return [], projection_error(
                f"忙闲响应缺少参与人覆盖：{', '.join(missing)}。",
                'coverage_unknown',
            )

    items: list[Any] = []
    for index, row in enumerate(rows, 1):
        if isinstance(row, list):
            items.extend(row)
            continue
        if not isinstance(row, dict):
            return [], projection_error(f'忙闲响应第 {index} 行类型不可识别。')
        if isinstance(row.get('scheduleItems'), list):
            items.extend(row['scheduleItems'])
        elif isinstance(row.get('busyTimes'), list):
            items.extend(row['busyTimes'])
        elif row.get('startTime') is not None or row.get('start') is not None:
            items.append(row)
        elif row.get('userId') is not None:
            return [], projection_error(f'参与人 {row.get("userId")} 缺少 scheduleItems/busyTimes。')
        elif not row:
            continue
        else:
            return [], projection_error(f'忙闲响应第 {index} 行缺少已知时间数组。')

    intervals: list[tuple[datetime, datetime]] = []
    for index, item in enumerate(items, 1):
        interval, error = interval_from_item(item, index)
        if error:
            return [], error
        if interval is not None:
            intervals.append(interval)
    return intervals, None


def find_free_slots(
    day_start: datetime,
    day_end: datetime,
    busy: list[tuple[datetime, datetime]],
    duration_min: int,
) -> list[tuple[datetime, datetime]]:
    clipped = [
        (max(start, day_start), min(end, day_end))
        for start, end in busy
        if end > day_start and start < day_end
    ]
    merged: list[tuple[datetime, datetime]] = []
    for start, end in sorted(clipped, key=lambda value: value[0]):
        if merged and start <= merged[-1][1]:
            merged[-1] = (merged[-1][0], max(merged[-1][1], end))
        else:
            merged.append((start, end))
    free: list[tuple[datetime, datetime]] = []
    cursor = day_start
    for start, end in merged:
        if (start - cursor).total_seconds() / 60 >= duration_min:
            free.append((cursor, start))
        cursor = max(cursor, end)
    if (day_end - cursor).total_seconds() / 60 >= duration_min:
        free.append((cursor, day_end))
    return free


def main() -> int:
    parser = argparse.ArgumentParser(description='查询多人共同空闲时段')
    parser.add_argument('--users', required=True, help='用户 ID 列表，逗号分隔')
    parser.add_argument('--date', required=True, help='查询日期 YYYY-MM-DD')
    parser.add_argument('--duration', type=int, default=60, help='会议时长（分钟），默认 60')
    parser.add_argument('--start-hour', type=int, default=9, help='工作日开始小时，默认 9')
    parser.add_argument('--end-hour', type=int, default=18, help='工作日结束小时，默认 18')
    add_contract_flags(parser)
    args = parser.parse_args()

    users = list(dict.fromkeys(value.strip() for value in args.users.split(',') if value.strip()))
    if not users:
        return failure(args.format, '--users 必须至少包含一个用户 ID')
    if args.duration <= 0:
        return failure(args.format, '--duration 必须大于 0')
    if not 0 <= args.start_hour < args.end_hour <= 24:
        return failure(args.format, '工作时间必须满足 0 <= start-hour < end-hour <= 24')
    try:
        day = datetime.strptime(args.date, '%Y-%m-%d')
    except ValueError:
        return failure(args.format, '日期格式应为 YYYY-MM-DD')
    day_start = day.replace(hour=args.start_hour, tzinfo=TZ)
    day_end = day.replace(hour=0, tzinfo=TZ) + timedelta(hours=args.end_hour)

    if args.dry_run:
        run_dws([
            'calendar', 'busy', 'search', '--users', ','.join(users),
            '--start', fmt_iso(day_start), '--end', fmt_iso(day_end), '--format', 'json',
        ], dry_run=True)
        return emit(
            fmt=args.format,
            outcome='success',
            data={
                'date': args.date, 'users': users, 'duration': args.duration,
                'startHour': args.start_hour, 'endHour': args.end_hour,
            },
            dry_run=True,
            text='[dry-run] 将查询参与人忙闲并计算共同空闲时段',
        )

    result = run_dws([
        'calendar', 'busy', 'search', '--users', ','.join(users),
        '--start', fmt_iso(day_start), '--end', fmt_iso(day_end), '--format', 'json',
    ])
    meta = {'children': [{'id': 'busy:search', 'meta': result.meta}]} if result.meta else None
    if result.state != 'success':
        return emit(
            fmt=args.format,
            outcome='failure',
            error=result.error or projection_error('忙闲查询失败。'),
            meta=meta,
            text='忙闲查询失败，无法给出空闲时段结论',
        )
    busy, error = project_busy_intervals(result.payload, users)
    if error:
        return emit(
            fmt=args.format,
            outcome='failure',
            error=error,
            meta=meta,
            text='忙闲响应无法证明所有参与人的空闲状态',
        )
    free = find_free_slots(day_start, day_end, busy, args.duration)
    slots = [
        {'start': start.isoformat(), 'end': end.isoformat(), 'minutes': int((end - start).total_seconds() / 60)}
        for start, end in free
    ]
    data = {
        'date': args.date,
        'users': users,
        'duration': args.duration,
        'coverage': {'complete': True, 'users': users},
        'busyCount': len(busy),
        'slots': slots,
    }
    if args.format != 'text':
        return emit(fmt=args.format, outcome='success', data=data, meta=meta)

    print(f"\n🕐 空闲时段查询 ({args.date})")
    print(f'   参与人: {len(users)} 人')
    print(f'   会议时长: {args.duration} 分钟')
    print(f'   工作时间: {args.start_hour}:00 ~ {args.end_hour}:00')
    print(f'   已识别忙时段: {len(busy)} 个')
    print('=' * 50)
    if not free:
        print('  ❌ 该日无共同空闲时段')
        return 0
    print(f"\n✅ 找到 {len(free)} 个可用时段:\n")
    for index, (start, end) in enumerate(free, 1):
        minutes = int((end - start).total_seconds() / 60)
        label = '⭐ 推荐' if index == 1 else f'   备选{index - 1}'
        print(f"  {label}  {start.strftime('%H:%M')} ~ {end.strftime('%H:%M')}  ({minutes}分钟)")
    return 0


if __name__ == '__main__':
    raise SystemExit(run_main(main))
