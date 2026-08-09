#!/usr/bin/env python3
"""Agent probe for attendance report batch ledgers and write boundaries."""

from __future__ import annotations

import argparse
import importlib.util
import sys
from datetime import date, datetime
from pathlib import Path
from types import ModuleType
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
ENTRIES = (
    ('mono-checkin', ROOT / 'skills/mono/scripts/attendance_report_checkin.py', 'checkin'),
    ('mono-daily', ROOT / 'skills/mono/scripts/attendance_report_daily.py', 'query'),
    ('mono-monthly', ROOT / 'skills/mono/scripts/attendance_report_monthly.py', 'query'),
    ('mono-detail', ROOT / 'skills/mono/scripts/attendance_report_detail.py', 'detail'),
    ('multi-checkin', ROOT / 'skills/multi/dingtalk-misc/scripts/attendance_report_checkin.py', 'checkin'),
    ('multi-daily', ROOT / 'skills/multi/dingtalk-misc/scripts/attendance_report_daily.py', 'query'),
    ('multi-monthly', ROOT / 'skills/multi/dingtalk-misc/scripts/attendance_report_monthly.py', 'query'),
    ('multi-detail', ROOT / 'skills/multi/dingtalk-misc/scripts/attendance_report_detail.py', 'detail'),
)


def load(label: str, path: Path) -> ModuleType:
    for name in ('attendance_report_common', '_runtime'):
        sys.modules.pop(name, None)
    sys.path.insert(0, str(path.parent))
    try:
        spec = importlib.util.spec_from_file_location(label.replace('-', '_'), path)
        if spec is None or spec.loader is None:
            raise RuntimeError(f'cannot load {path}')
        module = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = module
        spec.loader.exec_module(module)
        return module
    finally:
        sys.path.pop(0)


def call_batch(module: ModuleType, kind: str, payload: Any) -> tuple[list[dict], Any]:
    cmn = module.cmn
    stats = cmn.CallStats(user_batches=1, date_slices=1)
    original = cmn.run_dws
    def fake_run_dws(_args: list[str]) -> Any:
        if isinstance(payload, Exception):
            raise payload
        return payload
    cmn.run_dws = fake_run_dws
    try:
        date_slice = cmn.DateSlice(datetime(2026, 8, 1), datetime(2026, 8, 1, 23, 59, 59))
        if kind == 'checkin':
            records = module.query_checkin_batch(['u1'], date_slice, 'corp', 'staff', stats)
        elif kind == 'query':
            records = module.query_one_batch(['u1'], ['c1'], date_slice, stats)
        else:
            records = module.query_check_records(['u1'], date_slice, stats)
        return records, stats
    finally:
        cmn.run_dws = original


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path)
    args = parser.parse_args()
    checks: list[tuple[str, bool, str]] = []

    for label, path, kind in ENTRIES:
        module = load(label, path)
        records, stats = call_batch(module, kind, [])
        checks.append((
            f'{label}: 明确空批次记成功而非失败',
            records == [] and len(stats.succeeded) == 1 and not stats.failed,
            f'succeeded={len(stats.succeeded)}, failed={len(stats.failed)}',
        ))
        records, stats = call_batch(module, kind, {'unexpected': []})
        checks.append((
            f'{label}: 未知投影记 typed failure',
            records == []
            and not stats.succeeded
            and len(stats.failed) == 1
            and stats.failed[0].get('error', {}).get('subtype') == 'projection_unknown',
            f'succeeded={len(stats.succeeded)}, failed={len(stats.failed)}',
        ))
        permission = module.cmn.DwsCallError('denied', is_permission_error=True)
        records, stats = call_batch(module, kind, permission)
        checks.append((
            f'{label}: 权限失败保留 authorization 类型',
            records == []
            and not stats.succeeded
            and len(stats.failed) == 1
            and stats.failed[0].get('error', {}).get('type') == 'authorization'
            and stats.failed[0].get('error', {}).get('subtype') == 'permission_denied',
            f'succeeded={len(stats.succeeded)}, failed={len(stats.failed)}',
        ))
        source = path.read_text(encoding='utf-8')
        result_pos = source.find('result_data =')
        failure_pos = source.find('if outcome == "failure":', result_pos)
        write_pos = source.find('cmn.write_excel', result_pos)
        checks.append((
            f'{label}: 全失败门禁位于本地写入前',
            result_pos >= 0 and failure_pos > result_pos and write_pos > failure_pos,
            f'failure_pos={failure_pos}, write_pos={write_pos}',
        ))
        checks.append((
            f'{label}: 终态使用公共 report_result',
            'cmn.report_result(stats,' in source and 'outcome="partial_failure"' not in source,
            'shared-helper',
        ))

    for label, path in (
        ('mono-common', ROOT / 'skills/mono/scripts/attendance_report_common.py'),
        ('multi-common', ROOT / 'skills/multi/dingtalk-misc/scripts/attendance_report_common.py'),
    ):
        module = load(label, path)
        stats = module.CallStats()
        stats.record_success('page:1', item_count=2)
        stats.record_failure(
            'page:2',
            module.DwsCallError(
                'timeout',
                error_info={'type': 'network', 'subtype': 'timeout', 'message': 'timeout'},
            ),
        )
        outcome, data, error = module.report_result(stats, {'rowCount': 2})
        checks.append((
            f'{label}: 混合批次为 partial ledger',
            outcome == 'partial_failure'
            and error is None
            and data.get('succeeded', [{}])[0].get('id') == 'page:1'
            and data.get('failed', [{}])[0].get('id') == 'page:2'
            and data.get('coverage', {}).get('complete') is False,
            f'outcome={outcome}',
        ))
        failed_stats = module.CallStats()
        failed_stats.record_failure(
            'page:1',
            module.DwsCallError(
                'denied',
                error_info={'type': 'authorization', 'subtype': 'scope_missing', 'message': 'denied'},
            ),
        )
        outcome, data, error = module.report_result(failed_stats, {'rowCount': 0})
        checks.append((
            f'{label}: 全批次失败为 typed failure',
            outcome == 'failure'
            and error is not None
            and error.get('subtype') == 'scope_missing'
            and not data.get('succeeded'),
            f'outcome={outcome}',
        ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        '# 考勤报表逐批 partial ledger Agent 探针',
        '',
        f'扫描日期：{date.today().isoformat()}',
        '',
        '> 受控注入逐个调用 Mono/Multi 八个报表入口的批次函数，并审阅本地写入门禁；不保存 JSON fixture，不创建 Excel，不证明真实考勤权限或服务端终态。',
        '',
        '| 检查 | 结果 | 证据 |',
        '|---|---|---|',
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        '',
        f'结论：**{passed}/{len(checks)} PASS**。',
        '',
        '范围：证明已知空、未知投影、逐批 ledger、partial/failure 推导及全失败写前门禁；真实 Excel 内容、图片下载与后端覆盖仍需隔离/live evidence。',
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
