#!/usr/bin/env python3
"""Agent semantic probe for the Multi calendar scheduling script.

Temporary fake commands prove result classification and zero-write boundaries.
The Markdown report is evidence only; it does not create a real calendar event.
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "skills" / "multi" / "dingtalk-calendar" / "scripts" / "calendar_schedule_meeting.py"


def execute(args: list[str], env: dict[str, str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(SCRIPT), *args], cwd=ROOT, env=env, capture_output=True, text=True, timeout=30)


def envelope(proc: subprocess.CompletedProcess[str]) -> tuple[dict[str, Any] | None, str]:
    lines = [line for line in proc.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None, f"stdout lines={len(lines)}"
    try:
        value = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return None, f"invalid JSON: {exc}"
    return (value, "ok") if isinstance(value, dict) else (None, "result is not an object")


def write_dws(path: Path, behavior: str) -> None:
    path.write_text(
        "#!/usr/bin/env python3\n"
        "import json, os, sys\n"
        "argv = sys.argv[1:]\n"
        "log = os.environ.get('PROBE_CALENDAR_LOG')\n"
        "if log:\n"
        "  with open(log, 'a', encoding='utf-8') as out: out.write(' '.join(argv[:3]) + '\\n')\n"
        "if argv[:3] == ['calendar', 'room', 'search']:\n"
        f"  if {behavior!r} == 'no_room': print(json.dumps({{'ok': True, 'data': {{'rooms': []}}}}))\n"
        "  else: print(json.dumps({'ok': True, 'data': {'rooms': [{'roomId': 'room_1', 'roomName': 'Room one'}]}}))\n"
        "elif argv[:3] == ['calendar', 'event', 'create']:\n"
        "  print(json.dumps({'ok': True, 'data': {'eventId': 'event_1'}}))\n"
        "elif argv[:3] == ['calendar', 'participant', 'add']:\n"
        f"  if {behavior!r} == 'participant_unknown': print(json.dumps({{'success': False, 'error': {{'type': 'api', 'message': 'ambiguous attendee write'}}}}))\n"
        "  else: print(json.dumps({'ok': True, 'data': {'accepted': True}}))\n"
        "elif argv[:3] == ['calendar', 'room', 'add']:\n"
        "  print(json.dumps({'ok': True, 'data': {'accepted': True}}))\n"
        "elif argv[:3] == ['calendar', 'event', 'get']:\n"
        "  print(json.dumps({'ok': True, 'data': {'eventId': 'event_1', 'title': 'probe', 'attendees': ['user_1'], 'rooms': ['room_1']}}))\n"
        "else:\n"
        "  print(json.dumps({'ok': False, 'error': {'type': 'internal', 'message': 'unexpected command'}}))\n",
        encoding="utf-8",
    )
    path.chmod(0o755)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    outcomes: list[tuple[str, str, str]] = []
    with tempfile.TemporaryDirectory(prefix="dws-multi-calendar-probe-") as directory:
        tmp = Path(directory)
        blocked_bin = tmp / "blocked-bin"
        blocked_bin.mkdir()
        sentinel = tmp / "unexpected-dws-call"
        blocked = blocked_bin / "dws"
        blocked.write_text(f"#!/bin/sh\ntouch '{sentinel}'\nexit 99\n", encoding="utf-8")
        blocked.chmod(0o755)
        base_env = {**os.environ, "PATH": f"{blocked_bin}{os.pathsep}{os.environ.get('PATH', '')}"}
        common = ["--title", "probe", "--start", "2027-01-01T10:00", "--end", "2027-01-01T11:00", "--format", "json"]

        help_proc = execute(["--help"], base_env)
        help_ok = help_proc.returncode == 0 and "--format {text,json,ndjson}" in help_proc.stdout and "--dry-run" in help_proc.stdout and "--yes" in help_proc.stdout
        outcomes.append(("脚本 Help 可发现 format/dry-run/yes", "PASS" if help_ok else "FAIL", f"rc={help_proc.returncode}"))

        dry_run = execute([*common, "--book-room", "--dry-run"], base_env)
        payload, detail = envelope(dry_run)
        dry_ok = dry_run.returncode == 0 and payload is not None and payload.get("dry_run") is True and not sentinel.exists()
        outcomes.append(("dry-run 不搜索会议室也不写入", "PASS" if dry_ok else "FAIL", f"rc={dry_run.returncode}; {detail}; sentinel={sentinel.exists()}"))

        no_yes = execute(common, base_env)
        payload, detail = envelope(no_yes)
        no_yes_ok = no_yes.returncode == 1 and payload is not None and payload.get("error", {}).get("subtype") == "confirmation_required" and not sentinel.exists()
        outcomes.append(("未确认日程创建 fail-closed", "PASS" if no_yes_ok else "FAIL", f"rc={no_yes.returncode}; {detail}; sentinel={sentinel.exists()}"))

        invalid_range = execute(["--title", "probe", "--start", "2027-01-01T11:00", "--end", "2027-01-01T10:00", "--format", "json"], base_env)
        payload, detail = envelope(invalid_range)
        range_ok = invalid_range.returncode == 1 and payload is not None and payload.get("error", {}).get("type") == "validation" and not sentinel.exists()
        outcomes.append(("无效时间范围在任何调用前失败", "PASS" if range_ok else "FAIL", f"rc={invalid_range.returncode}; {detail}; sentinel={sentinel.exists()}"))

        no_room_bin = tmp / "no-room-bin"
        no_room_bin.mkdir()
        write_dws(no_room_bin / "dws", "no_room")
        no_room_log = tmp / "no-room.log"
        no_room = execute([*common, "--book-room", "--yes"], {**base_env, "PATH": f"{no_room_bin}{os.pathsep}{base_env['PATH']}", "PROBE_CALENDAR_LOG": str(no_room_log)})
        payload, detail = envelope(no_room)
        no_room_ok = no_room.returncode == 1 and payload is not None and payload.get("data", {}).get("execution_state") == "not_executed" and no_room_log.read_text(encoding="utf-8") == "calendar room search\n"
        outcomes.append(("无可订会议室时不创建半成品日程", "PASS" if no_room_ok else "FAIL", f"rc={no_room.returncode}; {detail}"))

        success_bin = tmp / "success-bin"
        success_bin.mkdir()
        write_dws(success_bin / "dws", "success")
        success = execute([*common, "--users", "user_1", "--book-room", "--room-group-id", "group_1", "--yes"], {**base_env, "PATH": f"{success_bin}{os.pathsep}{base_env['PATH']}"})
        payload, detail = envelope(success)
        success_ok = success.returncode == 0 and payload is not None and payload.get("outcome") == "success" and payload.get("data", {}).get("verification", {}).get("state") == "verified"
        outcomes.append(("建日程、加人、订房与回读均成功才标 verified", "PASS" if success_ok else "FAIL", f"rc={success.returncode}; {detail}"))

        partial_bin = tmp / "partial-bin"
        partial_bin.mkdir()
        write_dws(partial_bin / "dws", "participant_unknown")
        partial = execute([*common, "--users", "user_1", "--yes"], {**base_env, "PATH": f"{partial_bin}{os.pathsep}{base_env['PATH']}"})
        payload, detail = envelope(partial)
        partial_ok = partial.returncode == 7 and payload is not None and payload.get("outcome") == "partial_failure" and payload.get("data", {}).get("succeeded", [{}])[0].get("id") == "event_1" and payload.get("data", {}).get("unknown", [{}])[0].get("id") == "event_1:participants"
        outcomes.append(("参会人写入未知时保留已创建日程并返回 rc=7", "PASS" if partial_ok else "FAIL", f"rc={partial.returncode}; {detail}"))

    passed = sum(1 for _, status, _ in outcomes if status == "PASS")
    lines = [
        "# Multi 日程创建 Agent 语义探针", "",
        "临时 child runner 只验证脚本编排与结果表达；本报告不保存 JSON fixture，也不创建真实日程或预订会议室。", "",
        "| 检查 | 结果 | 证据 |", "|---|---|---|",
    ]
    for name, status, detail in outcomes:
        lines.append("| {} | {} | {} |".format(name, status, detail.replace("|", "\\|")))
    lines.extend(["", f"结论：**{passed}/{len(outcomes)} PASS**。", "", "范围：验证 Help、确认门禁、零写预览、会议室预检、分步 partial 与 event get 回读；真实日程、参会人和会议室终态仍须隔离账号复验。", ""])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(outcomes) else 1


if __name__ == "__main__":
    raise SystemExit(main())
