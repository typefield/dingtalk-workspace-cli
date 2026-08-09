#!/usr/bin/env python3
"""Agent probe for OA pending-list and per-instance detail aggregation."""

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
    ('mono-oa', ROOT / 'skills/mono/scripts/oa_pending_review.py'),
    ('multi-oa', ROOT / 'skills/multi/dingtalk-misc/scripts/oa_pending_review.py'),
)

FAKE_DWS = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
mode = os.environ['DWS_OA_PROBE_MODE']
marker = pathlib.Path(os.environ['DWS_OA_PROBE_MARKER'])
marker.write_text(marker.read_text() + 'call\n' if marker.exists() else 'call\n')
args = sys.argv[1:]
def emit(value): print(json.dumps(value, ensure_ascii=False))
if args[:3] == ['oa', 'approval', 'list-pending']:
    if mode == 'list_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'auth', 'message': 'login'}}); raise SystemExit(3)
    if mode == 'list_unknown':
        emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}}); raise SystemExit(0)
    rows = [] if mode == 'empty' else [
        {'processInstanceId': 'p1', 'title': 'A', 'status': 'RUNNING', 'createTime': 1},
        {'processInstanceId': 'p2', 'title': 'B', 'status': 'RUNNING', 'createTime': 2},
    ]
    if mode == 'missing_id': rows = [{'title': 'No ID'}]
    emit({'ok': True, 'outcome': 'success', 'data': {'processInstanceList': rows}, 'meta': {'step': 'list'}}); raise SystemExit(0)
if args[:3] == ['oa', 'approval', 'detail']:
    pid = args[args.index('--instance-id') + 1]
    if mode in {'detail_partial', 'detail_projection'} and pid == 'p2':
        if mode == 'detail_partial':
            emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'authorization', 'message': 'denied'}}); raise SystemExit(3)
        emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}}); raise SystemExit(0)
    if mode == 'all_detail_failure':
        emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'network', 'message': 'timeout'}}); raise SystemExit(4)
    emit({'ok': True, 'outcome': 'success', 'data': {'formComponentValues': [{'name': 'Reason', 'value': pid}]}, 'meta': {'step': pid}}); raise SystemExit(0)
raise SystemExit(9)
'''


def parse(completed: subprocess.CompletedProcess[str]) -> dict[str, Any] | None:
    lines = [line for line in completed.stdout.splitlines() if line.strip()]
    if len(lines) != 1: return None
    try: value = json.loads(lines[0])
    except json.JSONDecodeError: return None
    return value if isinstance(value, dict) else None


def run(script: Path, mode: str, fake_dir: Path, marker: Path, *extra: str) -> subprocess.CompletedProcess[str]:
    marker.unlink(missing_ok=True)
    env = os.environ.copy(); env['PATH'] = f"{fake_dir}{os.pathsep}{env.get('PATH', '')}"
    env['DWS_OA_PROBE_MODE'] = mode; env['DWS_OA_PROBE_MARKER'] = str(marker)
    return subprocess.run([sys.executable, str(script), *extra, '--format', 'json'], cwd=ROOT, env=env, capture_output=True, text=True, timeout=30)


def calls(marker: Path) -> int: return len(marker.read_text().splitlines()) if marker.exists() else 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__); parser.add_argument('--output', type=Path); args = parser.parse_args()
    checks: list[tuple[str, bool, str]] = []
    with tempfile.TemporaryDirectory(prefix='dws-oa-pending-') as temp:
        fake_dir = Path(temp); fake = fake_dir / 'dws'; fake.write_text(FAKE_DWS, encoding='utf-8'); fake.chmod(0o755); marker = fake_dir / 'calls'
        for label, script in SCRIPTS:
            success = run(script, 'success', fake_dir, marker); payload = parse(success)
            checks.append((f'{label}: 列表与两详情成功', success.returncode == 0 and payload is not None and payload.get('data', {}).get('count') == 2 and len(payload.get('meta', {}).get('children', [])) == 3, f'rc={success.returncode}'))
            empty = run(script, 'empty', fake_dir, marker); payload = parse(empty)
            checks.append((f'{label}: 已知空列表成功', empty.returncode == 0 and payload is not None and payload.get('data', {}).get('items') == [] and calls(marker) == 1, f'rc={empty.returncode}'))
            failure = run(script, 'list_failure', fake_dir, marker); payload = parse(failure)
            checks.append((f'{label}: 列表 typed failure', failure.returncode == 1 and payload is not None and payload.get('error', {}).get('type') == 'auth', f'rc={failure.returncode}'))
            unknown = run(script, 'list_unknown', fake_dir, marker); payload = parse(unknown)
            checks.append((f'{label}: 列表未知形状 fail-closed', unknown.returncode == 1 and payload is not None and payload.get('error', {}).get('subtype') == 'projection_unknown', f'rc={unknown.returncode}'))
            missing = run(script, 'missing_id', fake_dir, marker); payload = parse(missing)
            checks.append((f'{label}: 缺 processInstanceId 不跳过', missing.returncode == 1 and payload is not None and payload.get('error', {}).get('subtype') == 'projection_unknown' and calls(marker) == 1, f'rc={missing.returncode}'))
            partial = run(script, 'detail_partial', fake_dir, marker); payload = parse(partial); data = payload.get('data', {}) if payload else {}
            checks.append((f'{label}: 单详情失败保留成功项', partial.returncode == 7 and payload is not None and data.get('succeeded', [{}])[0].get('id') == 'p1' and data.get('failed', [{}])[0].get('id') == 'p2', f'rc={partial.returncode}'))
            projection = run(script, 'detail_projection', fake_dir, marker); payload = parse(projection)
            checks.append((f'{label}: 单详情投影漂移为 partial', projection.returncode == 7 and payload is not None and payload.get('data', {}).get('failed', [{}])[0].get('error', {}).get('subtype') == 'projection_unknown', f'rc={projection.returncode}'))
            all_failed = run(script, 'all_detail_failure', fake_dir, marker); payload = parse(all_failed)
            checks.append((f'{label}: 全详情失败为 failure', all_failed.returncode == 1 and payload is not None and len(payload.get('data', {}).get('failed', [])) == 2, f'rc={all_failed.returncode}'))
            dry = run(script, 'success', fake_dir, marker, '--dry-run'); payload = parse(dry)
            checks.append((f'{label}: dry-run 零 child 进程', dry.returncode == 0 and payload is not None and payload.get('dry_run') is True and calls(marker) == 0, f'rc={dry.returncode}'))
            invalid = run(script, 'success', fake_dir, marker, '--days', '0'); payload = parse(invalid)
            checks.append((f'{label}: 非法 days 调用前校验', invalid.returncode == 1 and payload is not None and payload.get('error', {}).get('type') == 'validation' and calls(marker) == 0, f'rc={invalid.returncode}'))
    passed = sum(ok for _, ok, _ in checks)
    lines = ['# OA 待审聚合结果契约 Agent 探针', '', f'扫描日期：{date.today().isoformat()}', '', '> 临时假 dws 对拍 Mono/Multi 两入口；不保存 JSON fixture，不证明真实审批权限或服务端终态。', '', '| 检查 | 结果 | 证据 |', '|---|---|---|']
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend(['', f'结论：**{passed}/{len(checks)} PASS**。', '', '范围：证明列表、稳定实例 ID、逐详情 partial、投影漂移、已知空、meta、dry-run 与参数校验；审批可见范围和真实状态仍需 live evidence。', ''])
    report = '\n'.join(lines)
    if args.output: args.output.parent.mkdir(parents=True, exist_ok=True); args.output.write_text(report, encoding='utf-8')
    else: print(report)
    return 0 if passed == len(checks) else 1


if __name__ == '__main__': raise SystemExit(main())
