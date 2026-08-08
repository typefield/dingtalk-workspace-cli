#!/usr/bin/env python3
"""
一键创建日程 + 添加参与者 + 搜索并预定空闲会议室

用法:
    python calendar_schedule_meeting.py \
        --title "Q1 复盘会" \
        --start "2026-03-15T14:00" \
        --end "2026-03-15T15:00" \
        --users userId1,userId2 \
        --book-room

    python calendar_schedule_meeting.py --dry-run \
        --title "测试" --start "2026-03-15T14:00" --end "2026-03-15T15:00"
"""

import sys
import argparse
from datetime import datetime, timedelta, timezone
from typing import Any, Dict, List

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

TZ = timezone(timedelta(hours=8))


def run_dws(
    args: List[str], dry_run: bool = False,
) -> ChildDWSResult:
    return run_child_dws(args, dry_run=dry_run)


def child_data(payload: Any) -> Any:
    """Unwrap the current envelope and the historical result wrapper."""
    if isinstance(payload, dict):
        for key in ("data", "result"):
            if key in payload:
                return payload[key]
    return payload


def event_id_from(payload: Any) -> str:
    data = child_data(payload)
    if not isinstance(data, dict):
        return ""
    return str(data.get("eventId") or data.get("id") or "")


def child_meta(result: ChildDWSResult, entry_id: str) -> list[dict[str, Any]]:
    return [{"id": entry_id, "meta": result.meta}] if result.meta else []


def unknown_error(result: ChildDWSResult, message: str) -> dict[str, Any]:
    error = dict(result.error or {"type": "api", "message": message})
    error.setdefault("hint", "请先核查日程当前状态；不要盲目重发后续写操作。")
    return error


def normalize_time(time_str: str) -> str:
    for fmt in ('%Y-%m-%dT%H:%M', '%Y-%m-%d %H:%M',
                '%Y-%m-%dT%H:%M:%S'):
        try:
            dt = datetime.strptime(time_str, fmt)
            dt = dt.replace(tzinfo=TZ)
            return dt.strftime('%Y-%m-%dT%H:%M:%S+08:00')
        except ValueError:
            continue
    if '+' in time_str or time_str.endswith('Z'):
        return time_str
    raise ValueError(f"无法解析时间：{time_str}")


def main() -> int:
    parser = argparse.ArgumentParser(
        description='一键创建日程 + 添加参与者 + 预定会议室'
    )
    parser.add_argument('--title', required=True, help='日程标题')
    parser.add_argument('--start', required=True, help='开始时间')
    parser.add_argument('--end', required=True, help='结束时间')
    parser.add_argument('--desc', default='', help='日程描述')
    parser.add_argument('--users', default='', help='参与者 userId')
    parser.add_argument(
        '--book-room', action='store_true', help='自动预定会议室'
    )
    add_contract_flags(parser)
    args = parser.parse_args()

    try:
        start_iso = normalize_time(args.start)
        end_iso = normalize_time(args.end)
    except ValueError as e:
        return failure(args.format, str(e))

    if args.dry_run:
        plan = {
            "operation": "calendar_schedule_meeting",
            "title": args.title,
            "start": start_iso,
            "end": end_iso,
            "participants": [u for u in args.users.split(',') if u],
            "book_room": args.book_room,
            "steps": ["create event"]
            + (["add participants"] if args.users else [])
            + (["search and reserve room"] if args.book_room else []),
        }
        return emit(fmt=args.format, outcome='success', data=plan, dry_run=True, text='[dry-run] 将创建日程并执行请求的后续步骤')

    print('📅 创建日程...', file=sys.stderr)
    create_args = [
        'calendar', 'event', 'create',
        '--title', args.title,
        '--start', start_iso,
        '--end', end_iso,
        '--format', 'json',
    ]
    if args.desc:
        create_args.extend(['--desc', args.desc])

    result = run_dws(create_args)
    if result.state == 'failed':
        return emit(fmt=args.format, outcome='failure', error=result.error or {'type': 'api', 'message': '创建日程在执行前被拒绝。'}, meta={'children': child_meta(result, 'event:create')} if result.meta else None, text='创建日程失败')
    if result.state != 'success':
        return emit(
            fmt=args.format,
            outcome='failure',
            data={'operation': 'calendar.event.create', 'execution_state': 'unknown'},
            error=unknown_error(result, '创建日程未返回可确认终态。'),
            meta={'children': child_meta(result, 'event:create')} if result.meta else None,
            text='创建日程终态未知；请先核查后再决定是否重试。',
        )

    event_id = event_id_from(result.payload)
    if not event_id:
        return emit(
            fmt=args.format,
            outcome='failure',
            data={'operation': 'calendar.event.create', 'execution_state': 'unknown'},
            error={'type': 'api', 'message': '日程创建响应缺少 eventId；无法确认可继续操作的目标。', 'hint': '请先查询日程是否已创建；不要直接重复创建。'},
            meta={'children': child_meta(result, 'event:create')} if result.meta else None,
            text='日程创建响应缺少 eventId；请先核查。',
        )

    print(f"  ✓ 日程已创建 (eventId: {event_id})", file=sys.stderr)
    succeeded: list[dict[str, Any]] = [{'id': event_id, 'operation': 'event_create'}]
    failed: list[dict[str, Any]] = []
    unknown: list[dict[str, Any]] = []
    metas = child_meta(result, 'event:create')
    result_data: Dict[str, Any] = {'eventId': event_id, 'participants': [], 'room': None}

    if args.users:
        print('\n👥 添加参与者...', file=sys.stderr)
        r = run_dws([
            'calendar', 'participant', 'add',
            '--event', event_id,
            '--users', args.users,
            '--format', 'json',
        ])
        metas.extend(child_meta(r, 'event:participants'))
        if r.state == 'success':
            print(f"  ✓ 已添加参与者: {args.users}", file=sys.stderr)
            result_data['participants'] = [u for u in args.users.split(',') if u]
            succeeded.append({'id': f'{event_id}:participants', 'operation': 'participant_add'})
        elif r.state == 'failed':
            failed.append({'id': f'{event_id}:participants', 'error': r.error or {'type': 'api', 'message': '添加参会人失败'}})
        else:
            unknown.append({'id': f'{event_id}:participants', 'reason': '添加参会人未返回可确认终态；请先核查日程。', 'error': unknown_error(r, '添加参会人未返回可确认终态。')})

    if args.book_room:
        print('\n🏢 搜索空闲会议室...', file=sys.stderr)
        rooms_data = run_dws([
            'calendar', 'room', 'search',
            '--start', start_iso,
            '--end', end_iso,
            '--format', 'json',
        ])
        metas.extend(child_meta(rooms_data, 'room:search'))

        if rooms_data.state == 'success':
            data = child_data(rooms_data.payload)
            rooms = data if isinstance(data, list) else data.get('rooms', []) if isinstance(data, dict) else []
            if rooms:
                room = rooms[0]
                room_id = room.get('roomId') or room.get('id') if isinstance(room, dict) else None
                room_name = room.get('roomName') or room.get('name') if isinstance(room, dict) else None
                print(f"  找到空闲会议室: {room_name}", file=sys.stderr)
                if room_id:
                    r = run_dws([
                        'calendar', 'room', 'add',
                        '--event', event_id,
                        '--rooms', str(room_id),
                        '--format', 'json',
                    ])
                    metas.extend(child_meta(r, f'{event_id}:room:{room_id}'))
                    if r.state == 'success':
                        print(f"  ✓ 已预定: {room_name}", file=sys.stderr)
                        result_data['room'] = {'id': room_id, 'name': room_name}
                        succeeded.append({'id': f'{event_id}:room:{room_id}', 'operation': 'room_add'})
                    elif r.state == 'failed':
                        failed.append({'id': f'{event_id}:room:{room_id}', 'error': r.error or {'type': 'api', 'message': '预定会议室失败'}})
                    else:
                        unknown.append({'id': f'{event_id}:room:{room_id}', 'reason': '预定会议室未返回可确认终态；请先核查日程。', 'error': unknown_error(r, '预定会议室未返回可确认终态。')})
                else:
                    failed.append({'id': f'{event_id}:room', 'error': {'type': 'api', 'message': '会议室搜索结果缺少 roomId'}})
            else:
                print('  ⚠ 该时段无空闲会议室', file=sys.stderr)
                failed.append({'id': f'{event_id}:room', 'error': {'type': 'precondition', 'message': '该时段没有可预定的会议室'}})
        elif rooms_data.state == 'failed':
            failed.append({'id': f'{event_id}:room-search', 'error': rooms_data.error or {'type': 'api', 'message': '搜索会议室失败'}})
        else:
            unknown.append({'id': f'{event_id}:room-search', 'reason': '搜索会议室未返回可确认终态；请先核查日程。', 'error': unknown_error(rooms_data, '搜索会议室未返回可确认终态。')})

    data = batch_data(
        succeeded=succeeded,
        failed=failed,
        unknown=unknown,
        eventId=event_id,
        participants=result_data['participants'],
        room=result_data['room'],
        start=start_iso,
        end=end_iso,
        bookRoom=args.book_room,
    )
    outcome = batch_outcome(data)
    return emit(
        fmt=args.format,
        outcome=outcome,
        data=data,
        meta={'children': metas} if metas else None,
        text='\n✅ 完成!' if outcome == 'success' else '\n⚠️ 日程已创建，但后续步骤未全部确认。',
    )


if __name__ == '__main__':
    sys.exit(run_main(main))
