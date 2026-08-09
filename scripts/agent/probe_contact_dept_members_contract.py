#!/usr/bin/env python3
"""Agent probe for department-search and per-department member aggregation."""

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
SCRIPTS = (
    ('mono-contact', ROOT / 'skills/mono/scripts/contact_dept_members.py'),
    ('multi-contact', ROOT / 'skills/multi/dingtalk-contact/scripts/contact_dept_members.py'),
)

FAKE_DWS = r'''#!/usr/bin/env python3
import json
import os
import pathlib
import sys

mode = os.environ['DWS_CONTACT_PROBE_MODE']
marker = pathlib.Path(os.environ['DWS_CONTACT_PROBE_MARKER'])
marker.write_text(marker.read_text() + 'call\n' if marker.exists() else 'call\n')
args = sys.argv[1:]

def emit(value):
    print(json.dumps(value, ensure_ascii=False))

if args[:3] == ['contact', 'dept', 'search']:
    if mode == 'search_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'auth', 'message': 'login'}})
        raise SystemExit(3)
    if mode == 'search_unknown':
        emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}, 'meta': {'step': 'search'}})
        raise SystemExit(0)
    if mode == 'missing_dept_id':
        rows = [{'deptName': 'No ID'}]
    elif mode == 'empty':
        rows = []
    else:
        rows = [
            {'deptId': 1, 'deptName': '<red>A</red>'},
            {'deptId': 2, 'deptName': 'B'},
        ]
    emit({'ok': True, 'outcome': 'success', 'data': {'deptList': rows}, 'meta': {'step': 'search'}})
    raise SystemExit(0)

if args[:3] == ['contact', 'dept', 'list-members']:
    dept = args[args.index('--depts') + 1]
    if mode in {'member_partial', 'member_projection'} and dept == '2':
        if mode == 'member_partial':
            emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'authorization', 'message': 'denied'}, 'meta': {'dept': dept}})
            raise SystemExit(3)
        emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}, 'meta': {'dept': dept}})
        raise SystemExit(0)
    if mode == 'all_member_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'network', 'message': 'timeout'}, 'meta': {'dept': dept}})
        raise SystemExit(4)
    rows = [] if mode == 'empty_members' else [{'userInfo': {'userId': 'user-' + dept, 'name': 'User ' + dept, 'title': 'Dev'}}]
    emit({'ok': True, 'outcome': 'success', 'data': {'deptUserList': rows}, 'meta': {'dept': dept}})
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


def run(script: Path, mode: str, fake_dir: Path, marker: Path, *extra: str) -> subprocess.CompletedProcess[str]:
    marker.unlink(missing_ok=True)
    env = os.environ.copy()
    env['PATH'] = f"{fake_dir}{os.pathsep}{env.get('PATH', '')}"
    env['DWS_CONTACT_PROBE_MODE'] = mode
    env['DWS_CONTACT_PROBE_MARKER'] = str(marker)
    return subprocess.run(
        [sys.executable, str(script), '--query', 'Engineering', *extra, '--format', 'json'],
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
    with tempfile.TemporaryDirectory(prefix='dws-contact-dept-') as temp:
        fake_dir = Path(temp)
        fake = fake_dir / 'dws'
        fake.write_text(FAKE_DWS, encoding='utf-8')
        fake.chmod(0o755)
        marker = fake_dir / 'calls'

        for label, script in SCRIPTS:
            success = run(script, 'success', fake_dir, marker)
            payload = parse_result(success)
            checks.append((
                f'{label}: 两部门成功与 child meta',
                success.returncode == 0
                and payload is not None
                and len(payload.get('data', {}).get('departments', [])) == 2
                and len(payload.get('meta', {}).get('children', [])) == 3
                and payload.get('data', {}).get('coverage', {}).get('complete') is False,
                f'rc={success.returncode}',
            ))
            empty = run(script, 'empty', fake_dir, marker)
            payload = parse_result(empty)
            checks.append((
                f'{label}: 已知空搜索结果成功',
                empty.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('departments') == []
                and calls(marker) == 1,
                f'rc={empty.returncode}',
            ))
            search_failure = run(script, 'search_failure', fake_dir, marker)
            payload = parse_result(search_failure)
            checks.append((
                f'{label}: 搜索 typed failure 原样分类',
                search_failure.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'auth'
                and calls(marker) == 1,
                f'rc={search_failure.returncode}',
            ))
            search_unknown = run(script, 'search_unknown', fake_dir, marker)
            payload = parse_result(search_unknown)
            checks.append((
                f'{label}: 搜索未知形状 fail-closed',
                search_unknown.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={search_unknown.returncode}',
            ))
            missing_id = run(script, 'missing_dept_id', fake_dir, marker)
            payload = parse_result(missing_id)
            checks.append((
                f'{label}: 缺 deptId 不静默跳过',
                missing_id.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown'
                and calls(marker) == 1,
                f'rc={missing_id.returncode}',
            ))
            partial = run(script, 'member_partial', fake_dir, marker)
            payload = parse_result(partial)
            data = payload.get('data', {}) if payload else {}
            checks.append((
                f'{label}: 单部门失败保留成功部门',
                partial.returncode == 7
                and payload is not None
                and payload.get('outcome') == 'partial_failure'
                and data.get('succeeded', [{}])[0].get('id') == '1'
                and data.get('failed', [{}])[0].get('id') == '2',
                f'rc={partial.returncode}',
            ))
            projection = run(script, 'member_projection', fake_dir, marker)
            payload = parse_result(projection)
            checks.append((
                f'{label}: 单部门投影漂移为 partial',
                projection.returncode == 7
                and payload is not None
                and payload.get('data', {}).get('failed', [{}])[0].get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={projection.returncode}',
            ))
            all_failed = run(script, 'all_member_failure', fake_dir, marker)
            payload = parse_result(all_failed)
            checks.append((
                f'{label}: 全部门失败为 failure',
                all_failed.returncode == 1
                and payload is not None
                and payload.get('outcome') == 'failure'
                and len(payload.get('data', {}).get('failed', [])) == 2,
                f'rc={all_failed.returncode}',
            ))
            empty_members = run(script, 'empty_members', fake_dir, marker)
            payload = parse_result(empty_members)
            departments = payload.get('data', {}).get('departments', []) if payload else []
            checks.append((
                f'{label}: 已知空成员数组保留部门',
                empty_members.returncode == 0
                and len(departments) == 2
                and departments[0].get('members') == [],
                f'rc={empty_members.returncode}',
            ))
            dry = run(script, 'success', fake_dir, marker, '--dry-run')
            payload = parse_result(dry)
            checks.append((
                f'{label}: dry-run 零 child 进程',
                dry.returncode == 0 and payload is not None and payload.get('dry_run') is True and calls(marker) == 0,
                f'rc={dry.returncode}',
            ))
            invalid = run(script, 'success', fake_dir, marker, '--query', '   ')
            payload = parse_result(invalid)
            checks.append((
                f'{label}: 空 query 调用前 validation',
                invalid.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'validation'
                and calls(marker) == 0,
                f'rc={invalid.returncode}',
            ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        '# Contact 部门成员聚合结果契约 Agent 探针',
        '',
        f'扫描日期：{date.today().isoformat()}',
        '',
        '> 临时假 dws 对拍 Mono/Multi 两入口；不保存 JSON fixture，不证明真实通讯录可见范围。',
        '',
        '| 检查 | 结果 | 证据 |',
        '|---|---|---|',
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        '',
        f'结论：**{passed}/{len(checks)} PASS**。',
        '',
        '范围：证明搜索、稳定 ID、逐部门 partial、投影漂移、已知空、meta、dry-run 与参数校验；通讯录权限、索引覆盖和跨层级完整性仍需 live evidence。',
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
