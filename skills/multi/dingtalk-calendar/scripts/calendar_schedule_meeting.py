#!/usr/bin/env python3
"""Schedule a meeting without erasing partial calendar side effects.

Room discovery is a read-only precondition.  Creation, attendee addition and
room booking are separate writes so each step can be reported honestly.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections.abc import Mapping
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Optional


_SHARED_RUNTIME = Path(__file__).resolve().parents[2] / "dingtalk-shared" / "scripts"
sys.path.insert(0, str(_SHARED_RUNTIME))

from _runtime import (  # noqa: E402
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


def _normalize_time(raw: str) -> str:
    for fmt in ("%Y-%m-%dT%H:%M", "%Y-%m-%d %H:%M", "%Y-%m-%dT%H:%M:%S"):
        try:
            return datetime.strptime(raw, fmt).replace(tzinfo=TZ).isoformat(timespec="seconds")
        except ValueError:
            continue
    try:
        return datetime.fromisoformat(raw.replace("Z", "+00:00")).isoformat(timespec="seconds")
    except ValueError as exc:
        raise ValueError(f"无法解析时间：{raw}") from exc


def _parse_ids(raw: str, label: str) -> tuple[list[str], Optional[str]]:
    values = [value.strip() for value in raw.split(",") if value.strip()]
    if raw and not values:
        return [], f"{label} 不能只包含逗号或空白。"
    return list(dict.fromkeys(values)), None


def _unwrap(value: Any) -> Any:
    while isinstance(value, Mapping):
        for key in ("data", "result", "content"):
            nested = value.get(key)
            if isinstance(nested, (Mapping, list)):
                value = nested
                break
        else:
            return value
    return value


def _event_id(payload: Any) -> str:
    value = _unwrap(payload)
    if not isinstance(value, Mapping):
        return ""
    for key in ("eventId", "event_id", "id"):
        candidate = value.get(key)
        if isinstance(candidate, str) and candidate:
            return candidate
    return ""


def _rooms(payload: Any) -> tuple[Optional[list[dict[str, Any]]], str]:
    """Return None for an unprojectable room response; [] only for known empty."""
    value = _unwrap(payload)
    if isinstance(value, Mapping):
        for key in ("rooms", "items", "list"):
            if isinstance(value.get(key), list):
                value = value[key]
                break
        else:
            return None, "响应没有可识别的会议室列表。"
    if not isinstance(value, list):
        return None, "会议室响应不是列表。"
    valid: list[dict[str, Any]] = []
    malformed = False
    for item in value:
        if not isinstance(item, Mapping):
            malformed = True
            continue
        room_id = item.get("roomId") or item.get("room_id") or item.get("id")
        if isinstance(room_id, str) and room_id:
            valid.append(dict(item))
        else:
            malformed = True
    if valid:
        return valid, f"返回 {len(valid)} 个可预订会议室。"
    if malformed:
        return None, "会议室列表含无法识别 roomId 的条目。"
    return [], "当前范围和时段没有可预订会议室。"


def _meta(entry_id: str, child: ChildDWSResult) -> Optional[dict[str, Any]]:
    return {"id": entry_id, "meta": child.meta} if child.meta else None


def _unknown_error(child: ChildDWSResult, fallback: str) -> dict[str, Any]:
    error = dict(child.error or {"type": "api", "message": fallback})
    error.setdefault("hint", "请先回读日程状态；不要直接重复发起写入。")
    return error


def _verification(payload: Any, *, event_id: str, title: str, users: list[str], room_id: Optional[str]) -> bool:
    """Use only observed identifiers/text; absence produces not_verified."""
    try:
        text = json.dumps(payload, ensure_ascii=False)
    except (TypeError, ValueError):
        return False
    expected = [event_id, title, *users]
    if room_id:
        expected.append(room_id)
    return all(value in text for value in expected)


def main() -> int:
    parser = argparse.ArgumentParser(description="创建日程、添加参会人并可选预订会议室；发送前需确认。")
    parser.add_argument("--title", required=True, help="日程标题")
    parser.add_argument("--start", required=True, help="开始时间 ISO-8601 或 YYYY-MM-DDTHH:MM")
    parser.add_argument("--end", required=True, help="结束时间 ISO-8601 或 YYYY-MM-DDTHH:MM")
    parser.add_argument("--desc", default="", help="日程描述")
    parser.add_argument("--users", default="", help="参会人 userId，多个用逗号分隔")
    parser.add_argument("--book-room", action="store_true", help="先查询空闲会议室，再为创建的日程预订")
    parser.add_argument("--room-group-id", default="", help="允许查询的会议室 groupId，多个用逗号分隔")
    parser.add_argument("--yes", action="store_true", help="确认创建日程、邀请参会人和预订会议室")
    add_contract_flags(parser, default="json")
    args = parser.parse_args()

    if not args.title.strip():
        return failure(args.format, "--title 不能为空。")
    try:
        start = _normalize_time(args.start)
        end = _normalize_time(args.end)
        if datetime.fromisoformat(end) <= datetime.fromisoformat(start):
            return failure(args.format, "--end 必须晚于 --start。")
    except ValueError as exc:
        return failure(args.format, str(exc))
    users, error = _parse_ids(args.users, "--users")
    if error:
        return failure(args.format, error)
    groups, error = _parse_ids(args.room_group_id, "--room-group-id")
    if error:
        return failure(args.format, error)

    plan = {
        "operation": "calendar_schedule_meeting",
        "title": args.title,
        "start": start,
        "end": end,
        "participants": users,
        "book_room": args.book_room,
        "room_groups": groups,
        "steps": (["search available room"] if args.book_room else [])
        + ["create event"]
        + (["add participants"] if users else [])
        + (["reserve selected room"] if args.book_room else []),
        "verification": {"state": "not_applicable" if args.dry_run else "pending"},
    }
    if args.dry_run:
        return emit(fmt=args.format, outcome="success", data=plan, dry_run=True, text="预览：不会搜索会议室或创建/修改日程。")
    if not args.yes:
        return emit(
            fmt=args.format,
            outcome="failure",
            data={"operation": "calendar_schedule_meeting", "execution_state": "not_executed"},
            error={"type": "policy", "subtype": "confirmation_required", "message": "创建日程前需要显式确认。", "hint": "确认时间、参会人和会议室范围后使用 --yes 重新执行。"},
            text="错误：创建日程前需要显式确认。",
        )

    child_meta: list[dict[str, Any]] = []
    selected_room: Optional[dict[str, Any]] = None
    if args.book_room:
        read_failures: list[dict[str, Any]] = []
        read_unknown: list[dict[str, Any]] = []
        for group in groups or [""]:
            scope = group or "root"
            command = ["calendar", "room", "search", "--start", start, "--end", end, "--format", "json"]
            if group:
                command.extend(["--group-id", group])
            print(f"🏢 查询空闲会议室（{scope}）", file=sys.stderr)
            search = run_child_dws(command)
            if (entry := _meta(f"room:search:{scope}", search)):
                child_meta.append(entry)
            if search.state == "failed":
                read_failures.append({"id": f"room:search:{scope}", "error": search.error or {"type": "api", "message": "会议室查询被拒绝。"}})
                continue
            if search.state != "success":
                read_unknown.append({"id": f"room:search:{scope}", "reason": "会议室查询未返回可信结果。", "error": _unknown_error(search, "会议室查询未返回可信结果。")})
                continue
            rooms, reason = _rooms(search.payload)
            if rooms:
                selected_room = rooms[0]
                break
            if rooms is None:
                read_unknown.append({"id": f"room:search:{scope}", "reason": reason, "error": {"type": "api", "subtype": "room_projection_unknown", "message": reason}})
            else:
                read_failures.append({"id": f"room:search:{scope}", "error": {"type": "precondition", "message": reason}})
        if not selected_room:
            data = batch_data(succeeded=[], failed=read_failures, unknown=read_unknown, total=len(read_failures) + len(read_unknown), room_groups=groups)
            top_error = (read_unknown[0]["error"] if read_unknown else read_failures[0]["error"] if read_failures else {"type": "precondition", "message": "没有可预订会议室。"})
            return emit(
                fmt=args.format,
                outcome="failure",
                data={**data, "stage": "room_search", "execution_state": "not_executed"},
                error=top_error,
                meta={"children": child_meta} if child_meta else None,
                text="未找到可信的可预订会议室；未创建日程。",
            )

    create_command = ["calendar", "event", "create", "--title", args.title, "--start", start, "--end", end, "--format", "json"]
    if args.desc:
        create_command.extend(["--desc", args.desc])
    print("📅 创建日程", file=sys.stderr)
    created = run_child_dws(create_command)
    if (entry := _meta("event:create", created)):
        child_meta.append(entry)
    if created.state != "success":
        error = _unknown_error(created, "创建日程未返回可信结果。")
        channel = "failed" if created.state == "failed" else "unknown"
        entry: dict[str, Any] = {"id": "event:create", "error": error}
        if channel == "unknown":
            entry["reason"] = "创建日程未返回终态；日程可能已经创建。"
        return emit(
            fmt=args.format,
            outcome="failure",
            data=batch_data(total=1, **{channel: [entry]}),
            error=error,
            meta={"children": child_meta} if child_meta else None,
            text="创建日程未得到可信结果；请先核查。",
        )
    event_id = _event_id(created.payload)
    if not event_id:
        error = {"type": "api", "subtype": "event_create_missing_id", "message": "创建日程响应缺少 eventId。", "hint": "先查询日程确认是否已创建；不要直接重复创建。"}
        return emit(
            fmt=args.format,
            outcome="failure",
            data=batch_data(total=1, unknown=[{"id": "event:create", "reason": "创建请求成功但缺少可继续操作的 eventId。", "error": error}]),
            error=error,
            meta={"children": child_meta} if child_meta else None,
            text="创建响应缺少 eventId；请先核查。",
        )

    succeeded = [{"id": event_id, "operation": "event_create"}]
    failed: list[dict[str, Any]] = []
    unknown: list[dict[str, Any]] = []
    room_id = None
    room_name = None
    if users:
        print("👥 添加参会人", file=sys.stderr)
        attendees = run_child_dws(["calendar", "participant", "add", "--event", event_id, "--users", ",".join(users), "--format", "json"])
        if (entry := _meta(f"{event_id}:participants", attendees)):
            child_meta.append(entry)
        if attendees.state == "success":
            succeeded.append({"id": f"{event_id}:participants", "operation": "participant_add", "users": users})
        elif attendees.state == "failed":
            failed.append({"id": f"{event_id}:participants", "error": attendees.error or {"type": "api", "message": "添加参会人失败。"}})
        else:
            unknown.append({"id": f"{event_id}:participants", "reason": "添加参会人未返回终态。", "error": _unknown_error(attendees, "添加参会人未返回终态。")})
    if selected_room:
        room_id = str(selected_room.get("roomId") or selected_room.get("room_id") or selected_room.get("id"))
        room_name = selected_room.get("roomName") or selected_room.get("name")
        print("🏢 预订会议室", file=sys.stderr)
        booked = run_child_dws(["calendar", "room", "add", "--event", event_id, "--rooms", room_id, "--format", "json"])
        if (entry := _meta(f"{event_id}:room:{room_id}", booked)):
            child_meta.append(entry)
        if booked.state == "success":
            succeeded.append({"id": f"{event_id}:room:{room_id}", "operation": "room_add", "roomId": room_id, "name": room_name})
        elif booked.state == "failed":
            failed.append({"id": f"{event_id}:room:{room_id}", "error": booked.error or {"type": "api", "message": "预订会议室失败。"}})
        else:
            unknown.append({"id": f"{event_id}:room:{room_id}", "reason": "预订会议室未返回终态。", "error": _unknown_error(booked, "预订会议室未返回终态。")})

    data = batch_data(
        succeeded=succeeded,
        failed=failed,
        unknown=unknown,
        total=1 + (1 if users else 0) + (1 if selected_room else 0),
        eventId=event_id,
        participants=users,
        room={"id": room_id, "name": room_name} if room_id else None,
        start=start,
        end=end,
    )
    outcome = batch_outcome(data)
    if outcome != "success":
        top_error = failed[0]["error"] if failed else unknown[0]["error"]
        return emit(fmt=args.format, outcome=outcome, data=data, error=top_error, meta={"children": child_meta} if child_meta else None, text="日程已部分创建或后续结果未知；请先回读核查。")

    readback = run_child_dws(["calendar", "event", "get", "--id", event_id, "--format", "json"])
    if (entry := _meta(f"{event_id}:readback", readback)):
        child_meta.append(entry)
    verified = readback.state == "success" and _verification(readback.payload, event_id=event_id, title=args.title, users=users, room_id=room_id)
    data["verification"] = {
        "state": "verified" if verified else "not_verified",
        "method": "calendar_event_get" if readback.state == "success" else "readback_unavailable",
    }
    if not verified:
        data["verification"]["reason"] = "写请求均已接受，但 event get 未给出可完整对拍的日程、参会人与会议室事实。"
    return emit(
        fmt=args.format,
        outcome="success",
        data=data,
        meta={"children": child_meta} if child_meta else None,
        text="日程写请求已完成；请以 verification 状态判断是否已回读确认。",
    )


if __name__ == "__main__":
    raise SystemExit(run_main(main, default_format="json"))
