#!/usr/bin/env python3
"""Agent probe for Mono/Multi unread-mail aggregate truthfulness."""

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
    ('mono-mail', ROOT / 'skills/mono/scripts/mail_unread_summary.py'),
    ('multi-mail', ROOT / 'skills/multi/dingtalk-mail/scripts/mail_unread_summary.py'),
)

FAKE_DWS = r'''#!/usr/bin/env python3
import json
import os
import pathlib
import sys

mode = os.environ['DWS_MAIL_PROBE_MODE']
marker = pathlib.Path(os.environ['DWS_MAIL_PROBE_MARKER'])
marker.write_text(marker.read_text() + 'call\n' if marker.exists() else 'call\n')
args = sys.argv[1:]

def emit(value):
    print(json.dumps(value, ensure_ascii=False))

if args[:3] == ['mail', 'mailbox', 'list']:
    if mode == 'mailbox_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'auth', 'message': 'login'}})
        raise SystemExit(3)
    if mode == 'mailbox_projection':
        emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}, 'meta': {'step': 'mailbox'}})
        raise SystemExit(0)
    emit({'ok': True, 'outcome': 'success', 'data': {'emailAccounts': [
        {'type': 'PERSONAL', 'email': 'personal@example.com'},
        {'type': 'ORG', 'email': 'org@example.com'},
    ]}, 'meta': {'step': 'mailbox'}})
    raise SystemExit(0)

if args[:3] == ['mail', 'message', 'search']:
    if mode == 'search_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'authorization', 'message': 'denied'}, 'meta': {'step': 'search'}})
        raise SystemExit(3)
    if mode == 'search_projection':
        emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}, 'meta': {'step': 'search'}})
        raise SystemExit(0)
    messages = [] if mode == 'empty' else [
        {'subject': 'A', 'from': {'name': 'Alice'}},
        {'subject': 'B', 'from': 'bob@example.com'},
    ]
    emit({'ok': True, 'outcome': 'success', 'data': {'messages': messages}, 'meta': {'step': 'search'}})
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
    env['DWS_MAIL_PROBE_MODE'] = mode
    env['DWS_MAIL_PROBE_MARKER'] = str(marker)
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
    with tempfile.TemporaryDirectory(prefix='dws-mail-unread-') as temp:
        fake_dir = Path(temp)
        fake = fake_dir / 'dws'
        fake.write_text(FAKE_DWS, encoding='utf-8')
        fake.chmod(0o755)
        marker = fake_dir / 'calls'

        for label, script in SCRIPTS:
            success = run(script, 'success', fake_dir, marker)
            payload = parse_result(success)
            checks.append((
                f'{label}: ORG 邮箱与消息成功投影',
                success.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('email') == 'org@example.com'
                and payload.get('data', {}).get('count') == 2
                and len(payload.get('meta', {}).get('children', [])) == 2,
                f'rc={success.returncode}',
            ))

            empty = run(script, 'empty', fake_dir, marker)
            payload = parse_result(empty)
            checks.append((
                f'{label}: 已知空列表才返回 count=0',
                empty.returncode == 0
                and payload is not None
                and payload.get('outcome') == 'success'
                and payload.get('data', {}).get('messages') == [],
                f'rc={empty.returncode}',
            ))

            mailbox_failure = run(script, 'mailbox_failure', fake_dir, marker)
            payload = parse_result(mailbox_failure)
            checks.append((
                f'{label}: 邮箱发现 typed failure 原样分类',
                mailbox_failure.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'auth'
                and calls(marker) == 1,
                f'rc={mailbox_failure.returncode}',
            ))

            mailbox_projection = run(script, 'mailbox_projection', fake_dir, marker)
            payload = parse_result(mailbox_projection)
            checks.append((
                f'{label}: 邮箱未知形状不伪装无邮箱',
                mailbox_projection.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown'
                and calls(marker) == 1,
                f'rc={mailbox_projection.returncode}',
            ))

            search_failure = run(script, 'search_failure', fake_dir, marker)
            payload = parse_result(search_failure)
            checks.append((
                f'{label}: 搜索失败不伪装空收件箱',
                search_failure.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'authorization'
                and calls(marker) == 2,
                f'rc={search_failure.returncode}',
            ))

            search_projection = run(script, 'search_projection', fake_dir, marker)
            payload = parse_result(search_projection)
            checks.append((
                f'{label}: 搜索未知形状 fail-closed',
                search_projection.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={search_projection.returncode}',
            ))

            dry = run(script, 'success', fake_dir, marker, '--dry-run')
            payload = parse_result(dry)
            checks.append((
                f'{label}: dry-run 零 child 进程',
                dry.returncode == 0
                and payload is not None
                and payload.get('dry_run') is True
                and calls(marker) == 0,
                f'rc={dry.returncode}',
            ))

            invalid = run(script, 'success', fake_dir, marker, '--limit', '0')
            payload = parse_result(invalid)
            checks.append((
                f'{label}: 非法 limit 写前/读前校验',
                invalid.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'validation'
                and calls(marker) == 0,
                f'rc={invalid.returncode}',
            ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        '# 未读邮件聚合结果契约 Agent 探针',
        '',
        f'扫描日期：{date.today().isoformat()}',
        '',
        '> 临时假 dws 对拍 Mono/Multi 两入口；不保存 JSON fixture，不证明真实邮箱索引或消息覆盖。',
        '',
        '| 检查 | 结果 | 证据 |',
        '|---|---|---|',
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        '',
        f'结论：**{passed}/{len(checks)} PASS**。',
        '',
        '范围：证明邮箱发现、搜索、投影、已知空、meta、dry-run 与参数校验的本地编排；真实邮箱权限、索引完整性和服务端终态仍需 live evidence。',
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
