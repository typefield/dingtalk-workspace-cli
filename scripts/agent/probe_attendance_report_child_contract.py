#!/usr/bin/env python3
"""Agent probe for attendance report child-DWS envelope compatibility."""

from __future__ import annotations

import argparse
import importlib.util
import json
import subprocess
import sys
from datetime import date
from pathlib import Path
from types import ModuleType
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
MODULES = (
    ('mono-report-common', ROOT / 'skills/mono/scripts/attendance_report_common.py'),
    ('multi-report-common', ROOT / 'skills/multi/dingtalk-misc/scripts/attendance_report_common.py'),
)


def load(label: str, path: Path) -> ModuleType:
    spec = importlib.util.spec_from_file_location(label.replace('-', '_'), path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f'cannot load {path}')
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def completed(payload: Any, rc: int = 0, stderr: str = '') -> subprocess.CompletedProcess[str]:
    stdout = payload if isinstance(payload, str) else json.dumps(payload, ensure_ascii=False)
    return subprocess.CompletedProcess(['dws'], rc, stdout=stdout, stderr=stderr)


def invoke(module: ModuleType, result: subprocess.CompletedProcess[str]) -> tuple[Any, Exception | None]:
    original = module.subprocess.run
    module.subprocess.run = lambda *args, **kwargs: result
    try:
        return module.run_dws(['attendance', 'report', 'query-data']), None
    except Exception as exc:  # noqa: BLE001 - probe records the contract error.
        return None, exc
    finally:
        module.subprocess.run = original


def error_info(error: Exception | None) -> dict[str, Any]:
    value = getattr(error, 'error_info', None)
    return value if isinstance(value, dict) else {}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path)
    args = parser.parse_args()
    checks: list[tuple[str, bool, str]] = []
    for label, path in MODULES:
        module = load(label, path)
        value, error = invoke(module, completed({'ok': True, 'outcome': 'success', 'data': {'items': [{'id': '1'}]}}))
        checks.append((f'{label}: 统一成功信封解到 data', error is None and value == {'items': [{'id': '1'}]}, type(error).__name__ if error else 'ok'))
        value, error = invoke(module, completed({'success': True, 'result': {'items': []}}))
        checks.append((f'{label}: 旧成功信封兼容', error is None and value == {'items': []}, type(error).__name__ if error else 'ok'))
        value, error = invoke(module, completed({'items': [{'id': 'bare'}]}))
        checks.append((f'{label}: bare 业务 JSON 兼容', error is None and value == {'items': [{'id': 'bare'}]}, type(error).__name__ if error else 'ok'))

        _, error = invoke(module, completed({'ok': False, 'outcome': 'failure', 'error': {'type': 'authorization', 'subtype': 'scope_missing', 'message': 'denied'}}, 4))
        checks.append((
            f'{label}: 非零统一错误保留 typed 信息',
            isinstance(error, module.DwsCallError)
            and error_info(error).get('subtype') == 'scope_missing'
            and getattr(error, 'is_permission_error', False),
            error_info(error).get('subtype', type(error).__name__ if error else 'none'),
        ))
        _, error = invoke(module, completed({'ok': False, 'outcome': 'partial_failure', 'data': {'succeeded': [], 'failed': []}}, 7))
        checks.append((
            f'{label}: child partial 不伪装完整数据',
            isinstance(error, module.DwsCallError) and error_info(error).get('subtype') == 'child_partial_failure',
            error_info(error).get('subtype', type(error).__name__ if error else 'none'),
        ))
        _, error = invoke(module, completed({'ok': True, 'outcome': 'pending', 'meta': {'operation': {'id': 't1'}}}))
        checks.append((
            f'{label}: child pending 不伪装终态数据',
            isinstance(error, module.DwsCallError) and error_info(error).get('subtype') == 'operation_pending',
            error_info(error).get('subtype', type(error).__name__ if error else 'none'),
        ))
        _, error = invoke(module, completed({'success': 'false', 'result': []}))
        checks.append((
            f'{label}: 字符串 success 被拒绝',
            isinstance(error, module.DwsCallError) and error_info(error).get('subtype') == 'untyped_status',
            error_info(error).get('subtype', type(error).__name__ if error else 'none'),
        ))
        _, error = invoke(module, completed({'ok': True, 'outcome': 'failure', 'error': {'type': 'api'}}))
        checks.append((
            f'{label}: 矛盾 ok/outcome 被拒绝',
            isinstance(error, module.DwsCallError) and error_info(error).get('subtype') == 'untyped_status',
            error_info(error).get('subtype', type(error).__name__ if error else 'none'),
        ))
        _, error = invoke(module, completed({'ok': True, 'outcome': 'success', 'data': []}, 5))
        checks.append((
            f'{label}: 非零退出与成功信封矛盾被拒绝',
            isinstance(error, module.DwsCallError) and error_info(error).get('subtype') == 'exit_outcome_inconsistent',
            error_info(error).get('subtype', type(error).__name__ if error else 'none'),
        ))
        _, error = invoke(module, completed('not-json'))
        checks.append((f'{label}: rc0 非 JSON 被拒绝', isinstance(error, module.DwsCallError), type(error).__name__ if error else 'none'))
        _, error = invoke(module, completed('not-json', 4, 'forbidden'))
        checks.append((
            f'{label}: 非零文本错误保留权限分类',
            isinstance(error, module.DwsCallError) and getattr(error, 'is_permission_error', False),
            type(error).__name__ if error else 'none',
        ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        '# 考勤报表 child 输出信封兼容 Agent 探针',
        '',
        f'扫描日期：{date.today().isoformat()}',
        '',
        '> 直接注入受控 subprocess 结果对拍 Mono/Multi 公共模块；不保存 JSON fixture，不调用 dws，不证明真实考勤数据或报表终态。',
        '',
        '| 检查 | 结果 | 证据 |',
        '|---|---|---|',
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        '',
        f'结论：**{passed}/{len(checks)} PASS**。',
        '',
        '范围：证明统一/旧/bare child 结果解包、typed error、pending/partial、字符串布尔值和退出码矛盾；各报表的逐批 partial ledger 仍需单独收敛。',
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
