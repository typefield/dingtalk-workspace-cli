#!/usr/bin/env python3
"""Agent probe for Calendar agenda and free-slot fail-closed semantics."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tempfile
from datetime import date
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
AGENDA = (
    ('mono-agenda', ROOT / 'skills/mono/scripts/calendar_today_agenda.py'),
    ('multi-agenda', ROOT / 'skills/multi/dingtalk-calendar/scripts/calendar_today_agenda.py'),
)
FREE = (
    ('mono-free', ROOT / 'skills/mono/scripts/calendar_free_slot_finder.py'),
    ('multi-free', ROOT / 'skills/multi/dingtalk-calendar/scripts/calendar_free_slot_finder.py'),
)

FAKE_DWS = r'''#!/usr/bin/env python3
import json
import os
import pathlib
import sys

mode = os.environ['DWS_CALENDAR_PROBE_MODE']
marker = pathlib.Path(os.environ['DWS_CALENDAR_PROBE_MARKER'])
marker.write_text(marker.read_text() + 'call\n' if marker.exists() else 'call\n')
args = sys.argv[1:]

def emit(value):
    print(json.dumps(value, ensure_ascii=False))

if mode == 'child_failure':
    emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'authorization', 'message': 'denied'}, 'meta': {'step': 'probe'}})
    raise SystemExit(3)

if args[:3] == ['calendar', 'event', 'list']:
    if mode == 'unknown':
        emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}, 'meta': {'step': 'agenda'}})
    elif mode == 'malformed':
        emit({'ok': True, 'outcome': 'success', 'data': {'events': [{'summary': 'A', 'start': 1, 'end': {}}]}, 'meta': {'step': 'agenda'}})
    else:
        events = [] if mode == 'empty' else [{
            'summary': 'A',
            'start': {'dateTime': '2026-08-09T10:00:00+08:00'},
            'end': {'dateTime': '2026-08-09T11:00:00+08:00'},
            'location': {'displayName': 'Room'},
        }]
        emit({'ok': True, 'outcome': 'success', 'data': {'events': events}, 'meta': {'step': 'agenda'}})
    raise SystemExit(0)

if args[:3] == ['calendar', 'busy', 'search']:
    rows = [
        {'userId': 'u1', 'scheduleItems': []},
        {'userId': 'u2', 'scheduleItems': []},
    ]
    if mode == 'missing_user':
        rows = rows[:1]
    elif mode == 'busy':
        rows[0]['scheduleItems'] = [{
            'start': '2026-08-09T10:00:00+08:00',
            'end': '2026-08-09T11:00:00+08:00',
            'status': 'BUSY',
        }]
    elif mode == 'malformed':
        rows[0]['scheduleItems'] = [{'start': 'bad', 'end': 'also-bad'}]
    elif mode == 'unknown':
        emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}, 'meta': {'step': 'busy'}})
        raise SystemExit(0)
    emit({'ok': True, 'outcome': 'success', 'data': rows, 'meta': {'step': 'busy'}})
    raise SystemExit(0)

raise SystemExit(9)
'''


def parse_result(completed: subprocess.CompletedProcess[str]) -> dict[str, Any] | None:
    lines = [line for line in completed.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None
    try:
        value = json.loads(lines[0])
    except json.JSONDecodeError:
        return None
    return value if isinstance(value, dict) else None


def run(script: Path, mode: str, fake_dir: Path, marker: Path, *script_args: str) -> subprocess.CompletedProcess[str]:
    marker.unlink(missing_ok=True)
    env = os.environ.copy()
    env['PATH'] = f"{fake_dir}{os.pathsep}{env.get('PATH', '')}"
    env['DWS_CALENDAR_PROBE_MODE'] = mode
    env['DWS_CALENDAR_PROBE_MARKER'] = str(marker)
    return subprocess.run(
        [sys.executable, str(script), *script_args, '--format', 'json'],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=30,
    )


def calls(marker: Path) -> int:
    return len(marker.read_text().splitlines()) if marker.exists() else 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path)
    args = parser.parse_args()
    checks: list[tuple[str, bool, str]] = []
    with tempfile.TemporaryDirectory(prefix='dws-calendar-read-') as temp:
        fake_dir = Path(temp)
        fake = fake_dir / 'dws'
        fake.write_text(FAKE_DWS, encoding='utf-8')
        fake.chmod(0o755)
        marker = fake_dir / 'calls'

        for label, script in AGENDA:
            success = run(script, 'success', fake_dir, marker, 'today')
            payload = parse_result(success)
            checks.append((
                f'{label}: 已知 event 结构与 meta',
                success.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('count') == 1
                and payload.get('meta', {}).get('children', [{}])[0].get('id') == 'event:list',
                f'rc={success.returncode}',
            ))
            empty = run(script, 'empty', fake_dir, marker, 'today')
            payload = parse_result(empty)
            checks.append((
                f'{label}: 已知空 events 才返回空日程',
                empty.returncode == 0 and payload is not None and payload.get('data', {}).get('items') == [],
                f'rc={empty.returncode}',
            ))
            failure = run(script, 'child_failure', fake_dir, marker, 'today')
            payload = parse_result(failure)
            checks.append((
                f'{label}: child failure 不伪装空日程',
                failure.returncode == 1 and payload is not None and payload.get('error', {}).get('type') == 'authorization',
                f'rc={failure.returncode}',
            ))
            unknown = run(script, 'unknown', fake_dir, marker, 'today')
            payload = parse_result(unknown)
            checks.append((
                f'{label}: 未知形状 fail-closed',
                unknown.returncode == 1 and payload is not None and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={unknown.returncode}',
            ))
            malformed = run(script, 'malformed', fake_dir, marker, 'today')
            payload = parse_result(malformed)
            checks.append((
                f'{label}: 非法时间不静默丢项',
                malformed.returncode == 1 and payload is not None and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={malformed.returncode}',
            ))
            dry = run(script, 'success', fake_dir, marker, 'today', '--dry-run')
            payload = parse_result(dry)
            checks.append((
                f'{label}: dry-run 零 child 进程',
                dry.returncode == 0 and payload is not None and payload.get('dry_run') is True and calls(marker) == 0,
                f'rc={dry.returncode}',
            ))

        free_args = ('--users', 'u1,u2', '--date', '2026-08-09', '--duration', '60')
        for label, script in FREE:
            success = run(script, 'success', fake_dir, marker, *free_args)
            payload = parse_result(success)
            checks.append((
                f'{label}: 全参与人覆盖后才推荐',
                success.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('coverage', {}).get('complete') is True
                and len(payload.get('data', {}).get('slots', [])) == 1,
                f'rc={success.returncode}',
            ))
            busy = run(script, 'busy', fake_dir, marker, *free_args)
            payload = parse_result(busy)
            checks.append((
                f'{label}: 忙时段参与计算',
                busy.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('busyCount') == 1
                and len(payload.get('data', {}).get('slots', [])) == 2,
                f'rc={busy.returncode}',
            ))
            missing = run(script, 'missing_user', fake_dir, marker, *free_args)
            payload = parse_result(missing)
            checks.append((
                f'{label}: 缺参与人覆盖拒绝推荐',
                missing.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'coverage_unknown'
                and 'slots' not in payload.get('data', {}),
                f'rc={missing.returncode}',
            ))
            malformed = run(script, 'malformed', fake_dir, marker, *free_args)
            payload = parse_result(malformed)
            checks.append((
                f'{label}: 非法忙时段拒绝全天空闲',
                malformed.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={malformed.returncode}',
            ))
            failure = run(script, 'child_failure', fake_dir, marker, *free_args)
            payload = parse_result(failure)
            checks.append((
                f'{label}: child failure 保留 typed error',
                failure.returncode == 1 and payload is not None and payload.get('error', {}).get('type') == 'authorization',
                f'rc={failure.returncode}',
            ))
            dry = run(script, 'success', fake_dir, marker, *free_args, '--dry-run')
            payload = parse_result(dry)
            checks.append((
                f'{label}: dry-run 零 child 进程',
                dry.returncode == 0 and payload is not None and payload.get('dry_run') is True and calls(marker) == 0,
                f'rc={dry.returncode}',
            ))
            invalid = run(script, 'success', fake_dir, marker, '--users', 'u1,u2', '--date', '2026-08-09', '--start-hour', '18', '--end-hour', '9')
            payload = parse_result(invalid)
            checks.append((
                f'{label}: 非法工作时间调用前校验',
                invalid.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'validation'
                and calls(marker) == 0,
                f'rc={invalid.returncode}',
            ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        '# Calendar 只读结果真实性 Agent 探针',
        '',
        f'扫描日期：{date.today().isoformat()}',
        '',
        '> 临时假 dws 对拍 Mono/Multi 日程与忙闲入口；不保存 JSON fixture，不证明真实日历覆盖。',
        '',
        '| 检查 | 结果 | 证据 |',
        '|---|---|---|',
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        '',
        f'结论：**{passed}/{len(checks)} PASS**。',
        '',
        '范围：证明已知空、typed failure、投影漂移、参与人覆盖、时间校验、meta 与 dry-run 的本地编排；真实日历权限、数据覆盖和服务端终态仍需 live evidence。',
        '',
    ])
    report = '\n'.join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding='utf-8')
    else:
        print(report)
    return 0 if passed == len(checks) else 1


if __name__ == '__main__':
    raise SystemExit(main())
