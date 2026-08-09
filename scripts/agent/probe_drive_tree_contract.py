#!/usr/bin/env python3
"""Agent probe for the Mono/Multi Drive tree pagination contract."""

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
    ('mono-drive', ROOT / 'skills/mono/scripts/drive_tree_list.py'),
    ('multi-drive', ROOT / 'skills/multi/dingtalk-drive/scripts/drive_tree_list.py'),
)

FAKE_DWS = r'''#!/usr/bin/env python3
import json, os, pathlib, sys
mode = os.environ['DWS_DRIVE_PROBE_MODE']
marker = pathlib.Path(os.environ['DWS_DRIVE_PROBE_MARKER'])
marker.write_text(marker.read_text() + 'call\n' if marker.exists() else 'call\n')
args = sys.argv[1:]
folder = args[args.index('--folder') + 1] if '--folder' in args else ''
cursor = args[args.index('--cursor') + 1] if '--cursor' in args else ''
def emit(value): print(json.dumps(value, ensure_ascii=False))
def item(fid, name, kind='FILE'): return {'fileId': fid, 'name': name, 'type': kind}
if mode == 'first_failure':
    emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'auth', 'message': 'login'}}); raise SystemExit(3)
if mode == 'unknown_shape':
    emit({'ok': True, 'outcome': 'success', 'data': {'unexpected': []}}); raise SystemExit(0)
if mode == 'missing_id':
    emit({'ok': True, 'outcome': 'success', 'data': {'items': [{'name': 'orphan', 'type': 'FILE'}]}}); raise SystemExit(0)
if mode == 'empty':
    emit({'ok': True, 'outcome': 'success', 'data': {'items': []}, 'meta': {'source': 'empty'}}); raise SystemExit(0)
if mode == 'has_more_without_token':
    emit({'ok': True, 'outcome': 'success', 'data': {'items': [item('a', 'a.txt')], 'hasMore': True}}); raise SystemExit(0)
if mode == 'child_failure' and folder == 'dir1':
    emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'authorization', 'message': 'denied'}}); raise SystemExit(4)
if mode in {'later_failure', 'text_partial'} and cursor == 'p2':
    emit({'ok': False, 'outcome': 'failure', 'error': {'type': 'network', 'message': 'timeout'}}); raise SystemExit(4)
if mode == 'cursor_loop':
    if not cursor:
        emit({'ok': True, 'outcome': 'success', 'data': {'items': [item('a', 'a.txt')], 'nextToken': 'loop'}}); raise SystemExit(0)
    emit({'ok': True, 'outcome': 'success', 'data': {'items': [item('b', 'b.txt')], 'nextToken': 'loop'}}); raise SystemExit(0)
if mode == 'success':
    if folder == 'dir1':
        emit({'ok': True, 'outcome': 'success', 'data': {'dentryList': [item('inner', 'inside.txt')]}, 'meta': {'page': 'child'}}); raise SystemExit(0)
    if cursor == 'p2':
        emit({'ok': True, 'outcome': 'success', 'data': {'items': [item('root2', 'root2.txt')]}, 'meta': {'page': 2}}); raise SystemExit(0)
    emit({'ok': True, 'outcome': 'success', 'data': {'items': [item('dir1', 'docs', 'FOLDER')], 'nextToken': 'p2'}, 'meta': {'page': 1}}); raise SystemExit(0)
if mode == 'child_failure':
    emit({'ok': True, 'outcome': 'success', 'data': {'items': [item('dir1', 'docs', 'FOLDER')]}}); raise SystemExit(0)
if mode in {'later_failure', 'text_partial'}:
    emit({'ok': True, 'outcome': 'success', 'data': {'items': [item('root1', 'root1.txt')], 'nextToken': 'p2'}}); raise SystemExit(0)
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
    fmt: str = 'json',
) -> subprocess.CompletedProcess[str]:
    marker.unlink(missing_ok=True)
    env = os.environ.copy()
    env['PATH'] = f"{fake_dir}{os.pathsep}{env.get('PATH', '')}"
    env['DWS_DRIVE_PROBE_MODE'] = mode
    env['DWS_DRIVE_PROBE_MARKER'] = str(marker)
    return subprocess.run(
        [sys.executable, str(script), '--depth', '1', *extra, '--format', fmt],
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
    with tempfile.TemporaryDirectory(prefix='dws-drive-tree-') as temp:
        fake_dir = Path(temp)
        fake = fake_dir / 'dws'
        fake.write_text(FAKE_DWS, encoding='utf-8')
        fake.chmod(0o755)
        marker = fake_dir / 'calls'
        for label, script in SCRIPTS:
            success = run(script, 'success', fake_dir, marker)
            payload = parse(success)
            data = payload.get('data', {}) if payload else {}
            children = data.get('items', [{}])[0].get('children', []) if data.get('items') else []
            checks.append((
                f'{label}: 根与子目录逐页耗尽',
                success.returncode == 0
                and payload is not None
                and payload.get('outcome') == 'success'
                and data.get('count') == 3
                and len(children) == 1
                and payload.get('meta', {}).get('pagination', {}).get('endpoint_exhausted') is True
                and calls(marker) == 3,
                f'rc={success.returncode}, calls={calls(marker)}',
            ))
            empty = run(script, 'empty', fake_dir, marker)
            payload = parse(empty)
            checks.append((
                f'{label}: 已知空目录才返回成功空',
                empty.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('items') == []
                and payload.get('meta', {}).get('pagination', {}).get('endpoint_exhausted') is True,
                f'rc={empty.returncode}',
            ))
            first = run(script, 'first_failure', fake_dir, marker)
            payload = parse(first)
            checks.append((
                f'{label}: 首请求失败不伪装空目录',
                first.returncode == 1
                and payload is not None
                and payload.get('outcome') == 'failure'
                and payload.get('error', {}).get('type') == 'auth',
                f'rc={first.returncode}',
            ))
            later = run(script, 'later_failure', fake_dir, marker)
            payload = parse(later)
            data = payload.get('data', {}) if payload else {}
            checks.append((
                f'{label}: 后续页失败保留首批数据',
                later.returncode == 7
                and payload is not None
                and payload.get('outcome') == 'partial_failure'
                and data.get('items', [{}])[0].get('fileId') == 'root1'
                and data.get('unknown', [{}])[0].get('id') == 'folder:root:page:2'
                and payload.get('meta', {}).get('pagination', {}).get('endpoint_exhausted') is False,
                f'rc={later.returncode}',
            ))
            child = run(script, 'child_failure', fake_dir, marker)
            payload = parse(child)
            data = payload.get('data', {}) if payload else {}
            checks.append((
                f'{label}: 子目录失败保留根目录',
                child.returncode == 7
                and payload is not None
                and data.get('items', [{}])[0].get('fileId') == 'dir1'
                and data.get('failed', [{}])[0].get('id') == 'folder:dir1:page:1',
                f'rc={child.returncode}',
            ))
            unknown = run(script, 'unknown_shape', fake_dir, marker)
            payload = parse(unknown)
            checks.append((
                f'{label}: 未知列表形状 fail-closed',
                unknown.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={unknown.returncode}',
            ))
            missing = run(script, 'missing_id', fake_dir, marker)
            payload = parse(missing)
            checks.append((
                f'{label}: 缺稳定 fileId fail-closed',
                missing.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('subtype') == 'projection_unknown',
                f'rc={missing.returncode}',
            ))
            inconsistent = run(script, 'has_more_without_token', fake_dir, marker)
            payload = parse(inconsistent)
            data = payload.get('data', {}) if payload else {}
            checks.append((
                f'{label}: 缺续页游标保留当前页',
                inconsistent.returncode == 7
                and payload is not None
                and data.get('items', [{}])[0].get('fileId') == 'a'
                and data.get('failed', [{}])[0].get('error', {}).get('subtype') == 'pagination_inconsistent',
                f'rc={inconsistent.returncode}',
            ))
            loop = run(script, 'cursor_loop', fake_dir, marker)
            payload = parse(loop)
            checks.append((
                f'{label}: 重复游标停止且 partial',
                loop.returncode == 7
                and payload is not None
                and payload.get('data', {}).get('count') == 2
                and payload.get('data', {}).get('failed', [{}])[0].get('error', {}).get('subtype') == 'pagination_inconsistent',
                f'rc={loop.returncode}',
            ))
            dry = run(script, 'success', fake_dir, marker, '--dry-run')
            payload = parse(dry)
            checks.append((
                f'{label}: dry-run 零 child 进程',
                dry.returncode == 0
                and payload is not None
                and payload.get('dry_run') is True
                and calls(marker) == 0,
                f'rc={dry.returncode}, calls={calls(marker)}',
            ))
            alias = run(script, 'empty', fake_dir, marker, '--parent-id', 'legacy-folder')
            payload = parse(alias)
            checks.append((
                f'{label}: 旧 parent-id 仅作兼容别名',
                alias.returncode == 0
                and payload is not None
                and payload.get('data', {}).get('folder') == 'legacy-folder',
                f'rc={alias.returncode}',
            ))
            invalid = run(script, 'success', fake_dir, marker, '--depth', '6')
            payload = parse(invalid)
            checks.append((
                f'{label}: 非法 depth 调用前校验',
                invalid.returncode == 1
                and payload is not None
                and payload.get('error', {}).get('type') == 'validation'
                and calls(marker) == 0,
                f'rc={invalid.returncode}',
            ))
            text_partial = run(script, 'text_partial', fake_dir, marker, fmt='text')
            checks.append((
                f'{label}: text 与 JSON 使用同一次遍历结果',
                text_partial.returncode == 7
                and 'root1.txt' in text_partial.stdout
                and calls(marker) == 2,
                f'rc={text_partial.returncode}, calls={calls(marker)}',
            ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        '# Drive 目录树分页结果契约 Agent 探针',
        '',
        f'扫描日期：{date.today().isoformat()}',
        '',
        '> 临时假 dws 对拍 Mono/Multi 两入口；不保存 JSON fixture，不证明真实钉盘权限、目录规模或服务端终态。',
        '',
        '| 检查 | 结果 | 证据 |',
        '|---|---|---|',
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        '',
        f'结论：**{passed}/{len(checks)} PASS**。',
        '',
        '范围：证明逐目录分页、单次遍历、已知空、投影 fail-closed、游标异常、部分失败、dry-run 和兼容别名；真实账号下的目录可见范围仍需 live evidence。',
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
