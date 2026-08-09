#!/usr/bin/env python3
"""Agent probe for Report inbox pagination and detail result truthfulness."""

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
    ('mono-inbox', ROOT / 'skills/mono/scripts/report_inbox_today.py'),
    ('mono-received', ROOT / 'skills/mono/scripts/report_received_today.py'),
    ('multi-misc-received', ROOT / 'skills/multi/dingtalk-misc/scripts/report_received_today.py'),
)

FAKE_DWS = r'''#!/usr/bin/env python3
import json
import os
import pathlib
import sys

args = sys.argv[1:]
mode = os.environ['DWS_REPORT_PROBE_MODE']
marker = pathlib.Path(os.environ['DWS_REPORT_PROBE_MARKER'])
marker.write_text(marker.read_text() + 'call\n' if marker.exists() else 'call\n')

def emit(value):
    print(json.dumps(value, ensure_ascii=False))

if args[:3] == ['report', 'inbox', 'list']:
    cursor = args[args.index('--cursor') + 1]
    if mode == 'first_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'auth', 'message': 'denied'}})
        raise SystemExit(3)
    if mode == 'second_failure' and cursor == 'next-1':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'network', 'message': 'timeout'}})
        raise SystemExit(4)
    if mode == 'missing_cursor':
        emit({'result': [], '_internalDetailCommands': [], 'hasMore': True})
        raise SystemExit(0)
    if cursor == '0':
        emit({
            'result': [{'标题': 'A', '发送人': 'Alice', '日期': '2026-08-09', '状态': '已读', '钉钉链接': 'https://a'}],
            '_internalDetailCommands': [{'command': 'dws report entry get --report-id report-a --format json'}],
            'hasMore': mode in {'two_pages', 'second_failure', 'detail_partial'},
            'nextCursor': 'next-1' if mode in {'two_pages', 'second_failure', 'detail_partial'} else '',
        })
    else:
        emit({
            'result': [{'标题': 'B', '发送人': 'Bob', '日期': '2026-08-08', '状态': '', '钉钉链接': 'https://b'}],
            '_internalDetailCommands': [{'command': 'dws report entry get --report-id report-b --format json'}],
            'hasMore': False,
            'nextCursor': '',
        })
    raise SystemExit(0)

if args[:3] == ['report', 'entry', 'get']:
    report_id = args[args.index('--report-id') + 1]
    if mode == 'detail_partial' and report_id == 'report-b':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'authorization', 'message': 'forbidden'}})
        raise SystemExit(3)
    emit({'result': {'report_content': [{'key': '进展', 'value': 'done ' + report_id}]}})
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
    env['DWS_REPORT_PROBE_MODE'] = mode
    env['DWS_REPORT_PROBE_MARKER'] = str(marker)
    return subprocess.run(
        [sys.executable, str(script), '--days', '2', *extra, '--format', 'json'],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=30,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path)
    args = parser.parse_args()
    checks: list[tuple[str, bool, str]] = []
    with tempfile.TemporaryDirectory(prefix='dws-report-aggregate-') as temp:
        fake_dir = Path(temp)
        fake = fake_dir / 'dws'
        fake.write_text(FAKE_DWS, encoding='utf-8')
        fake.chmod(0o755)
        marker = fake_dir / 'calls'

        for label, script in SCRIPTS:
            success = run(script, 'two_pages', fake_dir, marker)
            payload = parse_result(success)
            checks.append((
                f'{label}: 两页耗尽后才 success',
                success.returncode == 0
                and payload is not None
                and payload.get('outcome') == 'success'
                and payload.get('data', {}).get('count') == 2
                and payload.get('meta', {}).get('pagination', {}).get('endpoint_exhausted') is True,
                f'rc={success.returncode}',
            ))

            first = run(script, 'first_failure', fake_dir, marker)
            payload = parse_result(first)
            checks.append((
                f'{label}: 首页失败不伪装空列表',
                first.returncode == 1
                and payload is not None
                and payload.get('outcome') == 'failure'
                and payload.get('error', {}).get('type') == 'auth',
                f'rc={first.returncode}',
            ))

            later = run(script, 'second_failure', fake_dir, marker)
            payload = parse_result(later)
            data = payload.get('data', {}) if payload else {}
            checks.append((
                f'{label}: 后续页失败保留第一页',
                later.returncode == 7
                and payload is not None
                and payload.get('outcome') == 'partial_failure'
                and data.get('count') == 1
                and payload.get('meta', {}).get('pagination', {}).get('endpoint_exhausted') is False,
                f'rc={later.returncode}',
            ))

            inconsistent = run(script, 'missing_cursor', fake_dir, marker)
            payload = parse_result(inconsistent)
            checks.append((
                f'{label}: hasMore 无 cursor fail-closed',
                inconsistent.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'pagination_inconsistent',
                f'rc={inconsistent.returncode}',
            ))

            detail = run(script, 'detail_partial', fake_dir, marker, '--detail')
            payload = parse_result(detail)
            data = payload.get('data', {}) if payload else {}
            first_detail = data.get('items', [{}])[0].get('detail', []) if data else []
            checks.append((
                f'{label}: JSON detail 单项失败为 partial',
                detail.returncode == 7
                and payload is not None
                and payload.get('outcome') == 'partial_failure'
                and first_detail == [{'key': '进展', 'value': 'done report-a'}]
                and data.get('failed', [{}])[0].get('id') == 'report-b',
                f'rc={detail.returncode}',
            ))

            dry = run(script, 'two_pages', fake_dir, marker, '--detail', '--dry-run')
            payload = parse_result(dry)
            checks.append((
                f'{label}: dry-run 零 child 进程',
                dry.returncode == 0
                and payload is not None
                and payload.get('dry_run') is True
                and not marker.exists(),
                f'rc={dry.returncode}',
            ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        '# Report 聚合脚本结果契约 Agent 探针',
        '',
        f'扫描日期：{date.today().isoformat()}',
        '',
        '> 临时假 dws 验证两个兼容入口；不保存 JSON fixture，不证明真实日志可见性或服务端终态。',
        '',
        '| 检查 | 结果 | 证据 |',
        '|---|---|---|',
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        '',
        f'结论：**{passed}/{len(checks)} PASS**。',
        '',
        '范围：证明分页耗尽、前后页失败、分页矛盾、JSON detail partial 与 dry-run 本地边界；真实账号数据仍需 live evidence。',
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
