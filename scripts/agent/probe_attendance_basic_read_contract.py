#!/usr/bin/env python3
"""Agent probe for personal attendance and team-shift read scripts."""

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
MY_SCRIPTS = (
    ('mono-my-record', ROOT / 'skills/mono/scripts/attendance_my_record.py'),
    ('multi-my-record', ROOT / 'skills/multi/dingtalk-misc/scripts/attendance_my_record.py'),
)
SHIFT_SCRIPTS = (
    ('mono-team-shift', ROOT / 'skills/mono/scripts/attendance_team_shift.py'),
    ('multi-team-shift', ROOT / 'skills/multi/dingtalk-misc/scripts/attendance_team_shift.py'),
)

FAKE_DWS = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
mode = os.environ['DWS_ATTENDANCE_BASIC_MODE']
marker = pathlib.Path(os.environ['DWS_ATTENDANCE_BASIC_MARKER'])
marker.write_text(marker.read_text() + 'call\n' if marker.exists() else 'call\n')
args = sys.argv[1:]
def emit(value): print(json.dumps(value, ensure_ascii=False))
if args[:3] == ['contact', 'user', 'get-self']:
    if mode == 'self_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'auth', 'message': 'login'}}); raise SystemExit(3)
    if mode == 'self_unknown':
        emit({'ok': True, 'outcome': 'success', 'data': {'result': [{'name': 'No ID'}]}}); raise SystemExit(0)
    emit({'ok': True, 'outcome': 'success', 'data': {'result': [{'orgEmployeeModel': {'userId': 'me-1'}}]}, 'meta': {'step': 'self'}}); raise SystemExit(0)
if args[:3] == ['attendance', 'record', 'get']:
    if mode == 'record_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'network', 'message': 'timeout'}}); raise SystemExit(4)
    if mode == 'record_unknown':
        emit({'ok': True, 'outcome': 'success', 'data': {'result': {'unexpected': True}}}); raise SystemExit(0)
    if mode == 'record_empty':
        emit({'ok': True, 'outcome': 'success', 'data': {'result': None}, 'meta': {'step': 'record'}}); raise SystemExit(0)
    emit({'ok': True, 'outcome': 'success', 'data': {'result': {'userId': 'me-1', 'isHasSchedule': True, 'recordList': []}}, 'meta': {'step': 'record'}}); raise SystemExit(0)
if args[:3] == ['attendance', 'shift', 'list']:
    if mode == 'shift_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'authorization', 'message': 'denied'}}); raise SystemExit(4)
    if mode == 'shift_unknown':
        emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}}); raise SystemExit(0)
    if mode == 'shift_missing_id':
        emit({'ok': True, 'outcome': 'success', 'data': {'items': [{'workDate': '2026-08-03'}]}}); raise SystemExit(0)
    rows = [] if mode == 'shift_empty' else [{'userId': 'u1', 'workDate': '2026-08-03', 'isRest': False}]
    emit({'ok': True, 'outcome': 'success', 'data': {'items': rows}, 'meta': {'step': 'shift'}}); raise SystemExit(0)
raise SystemExit(9)
'''


def parse(completed: subprocess.CompletedProcess[str]) -> dict[str, Any] | None:
    lines = [line for line in completed.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None
    try:
        value = json.loads(lines[0])
    except json.JSONDecodeError:
        return None
    return value if isinstance(value, dict) else None


def run(
    script: Path,
    mode: str,
    fake_dir: Path,
    marker: Path,
    *extra: str,
) -> subprocess.CompletedProcess[str]:
    marker.unlink(missing_ok=True)
    env = os.environ.copy()
    env['PATH'] = f"{fake_dir}{os.pathsep}{env.get('PATH', '')}"
    env['DWS_ATTENDANCE_BASIC_MODE'] = mode
    env['DWS_ATTENDANCE_BASIC_MARKER'] = str(marker)
    return subprocess.run(
        [sys.executable, str(script), *extra, '--format', 'json'],
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
    with tempfile.TemporaryDirectory(prefix='dws-attendance-basic-') as temp:
        fake_dir = Path(temp)
        fake = fake_dir / 'dws'
        fake.write_text(FAKE_DWS, encoding='utf-8')
        fake.chmod(0o755)
        marker = fake_dir / 'calls'

        for label, script in MY_SCRIPTS:
            success = run(script, 'record_success', fake_dir, marker, '2026-08-03')
            payload = parse(success)
            checks.append((
                f'{label}: 身份与考勤详情成功',
                success.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('userId') == 'me-1'
                and payload.get('data', {}).get('count') == 1
                and len(payload.get('meta', {}).get('children', [])) == 2
                and calls(marker) == 2,
                f'rc={success.returncode}, calls={calls(marker)}',
            ))
            empty = run(script, 'record_empty', fake_dir, marker, '2026-08-03')
            payload = parse(empty)
            checks.append((
                f'{label}: 服务端明确 null 才返回空',
                empty.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('items') == []
                and payload.get('data', {}).get('coverage', {}).get('complete') is True,
                f'rc={empty.returncode}',
            ))
            self_failure = run(script, 'self_failure', fake_dir, marker, '2026-08-03')
            payload = parse(self_failure)
            checks.append((
                f'{label}: 当前用户失败不变成 validation/空',
                self_failure.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'auth'
                and calls(marker) == 1,
                f'rc={self_failure.returncode}',
            ))
            self_unknown = run(script, 'self_unknown', fake_dir, marker, '2026-08-03')
            payload = parse(self_unknown)
            checks.append((
                f'{label}: 当前用户缺 userId fail-closed',
                self_unknown.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown'
                and calls(marker) == 1,
                f'rc={self_unknown.returncode}',
            ))
            record_failure = run(script, 'record_failure', fake_dir, marker, '2026-08-03')
            payload = parse(record_failure)
            checks.append((
                f'{label}: 考勤 child 失败不伪装空',
                record_failure.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'network'
                and calls(marker) == 2,
                f'rc={record_failure.returncode}',
            ))
            record_unknown = run(script, 'record_unknown', fake_dir, marker, '2026-08-03')
            payload = parse(record_unknown)
            checks.append((
                f'{label}: 考勤未知形状 fail-closed',
                record_unknown.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={record_unknown.returncode}',
            ))
            dry = run(script, 'record_success', fake_dir, marker, '2026-08-03', '--dry-run')
            payload = parse(dry)
            checks.append((
                f'{label}: dry-run 零 child 进程',
                dry.returncode == 0 and payload is not None and payload.get('dry_run') is True and calls(marker) == 0,
                f'rc={dry.returncode}, calls={calls(marker)}',
            ))
            invalid = run(script, 'record_success', fake_dir, marker, '2026-02-31')
            payload = parse(invalid)
            checks.append((
                f'{label}: 无效真实日期调用前拒绝',
                invalid.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'validation'
                and calls(marker) == 0,
                f'rc={invalid.returncode}',
            ))

        base_shift = ('--users', 'u1,u1', '--start', '2026-08-03', '--end', '2026-08-07')
        for label, script in SHIFT_SCRIPTS:
            success = run(script, 'shift_success', fake_dir, marker, *base_shift)
            payload = parse(success)
            checks.append((
                f'{label}: 有界班次成功且用户去重',
                success.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('users') == ['u1']
                and payload.get('data', {}).get('count') == 1
                and payload.get('data', {}).get('coverage', {}).get('complete') is True
                and len(payload.get('meta', {}).get('children', [])) == 1,
                f'rc={success.returncode}',
            ))
            empty = run(script, 'shift_empty', fake_dir, marker, *base_shift)
            payload = parse(empty)
            checks.append((
                f'{label}: 已知空 items 成功',
                empty.returncode == 0 and payload is not None and payload.get('data', {}).get('items') == [],
                f'rc={empty.returncode}',
            ))
            failed = run(script, 'shift_failure', fake_dir, marker, *base_shift)
            payload = parse(failed)
            checks.append((
                f'{label}: child 失败不伪装空',
                failed.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'authorization',
                f'rc={failed.returncode}',
            ))
            unknown = run(script, 'shift_unknown', fake_dir, marker, *base_shift)
            payload = parse(unknown)
            checks.append((
                f'{label}: 未知容器 fail-closed',
                unknown.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={unknown.returncode}',
            ))
            missing = run(script, 'shift_missing_id', fake_dir, marker, *base_shift)
            payload = parse(missing)
            checks.append((
                f'{label}: 缺 userId 不静默跳过',
                missing.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={missing.returncode}',
            ))
            dry = run(script, 'shift_success', fake_dir, marker, *base_shift, '--dry-run')
            payload = parse(dry)
            checks.append((
                f'{label}: dry-run 零 child 进程',
                dry.returncode == 0 and payload is not None and payload.get('dry_run') is True and calls(marker) == 0,
                f'rc={dry.returncode}, calls={calls(marker)}',
            ))
            invalid = run(
                script, 'shift_success', fake_dir, marker,
                '--users', 'u1', '--start', '2026-08-03', '--end', '2026-08-10',
            )
            payload = parse(invalid)
            checks.append((
                f'{label}: 超 7 天调用前拒绝',
                invalid.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'validation'
                and calls(marker) == 0,
                f'rc={invalid.returncode}',
            ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        '# 考勤基础只读脚本结果契约 Agent 探针',
        '',
        f'扫描日期：{date.today().isoformat()}',
        '',
        '> 临时假 dws 对拍 Mono/Multi 四入口；不保存 JSON fixture，不证明真实考勤权限、数据覆盖或服务端终态。',
        '',
        '| 检查 | 结果 | 证据 |',
        '|---|---|---|',
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        '',
        f'结论：**{passed}/{len(checks)} PASS**。',
        '',
        '范围：证明身份投影、个人考勤/团队班次已知空、child failure、投影漂移、参数校验、meta 和 dry-run；真实管理员权限与组织数据完整性仍需 live evidence。',
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
