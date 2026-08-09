#!/usr/bin/env python3
"""Agent probe for schedule and vacation read-export script contracts."""

from __future__ import annotations

import argparse
import contextlib
import importlib.util
import io
import json
import subprocess
import sys
from datetime import date
from pathlib import Path
from types import ModuleType
from typing import Any
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
ROOTS = (
    ("mono", ROOT / "skills/mono/scripts"),
    ("multi", ROOT / "skills/multi/dingtalk-misc/scripts"),
)


def load(label: str, path: Path) -> ModuleType:
    for name in ("attendance_report_common", "_runtime"):
        sys.modules.pop(name, None)
    sys.path.insert(0, str(path.parent))
    try:
        spec = importlib.util.spec_from_file_location(f"{path.stem}_{label}", path)
        if spec is None or spec.loader is None:
            raise RuntimeError(f"cannot load {path}")
        module = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = module
        spec.loader.exec_module(module)
        return module
    finally:
        sys.path.pop(0)


def run_main(module: ModuleType, argv: list[str]) -> tuple[int, dict[str, Any]]:
    stdout = io.StringIO()
    stderr = io.StringIO()
    with (
        patch.object(sys, "argv", [str(module.__file__), *argv]),
        contextlib.redirect_stdout(stdout),
        contextlib.redirect_stderr(stderr),
    ):
        rc = module.run_main(module.main)
    return rc, json.loads(stdout.getvalue())


def schedule_final(module: ModuleType, state: str) -> tuple[int, dict[str, Any], int]:
    writes = 0

    def fake_fetch(_users: list[str], _start: str, _end: str, stats: Any) -> list[dict]:
        if state == "failure":
            stats.record_failure(
                "schedule-batch:1",
                module.DwsCallError(
                    "timeout",
                    error_info={"type": "network", "subtype": "timeout", "message": "timeout"},
                ),
            )
            return []
        stats.record_success("schedule-batch:1", item_count=1)
        if state == "partial":
            stats.record_failure(
                "schedule-batch:2",
                module.DwsCallError("denied", is_permission_error=True),
            )
        return [{"userId": "u1", "workDate": "2026-08-01", "classId": 1, "className": "Day"}]

    def fake_write(*_args: Any, **_kwargs: Any) -> None:
        nonlocal writes
        writes += 1

    with (
        patch.object(module, "fetch_all_schedules", side_effect=fake_fetch),
        patch.object(module, "build_class_name_map", return_value={1: "Day"}),
        patch.object(module, "resolve_user_names", return_value={"u1": "User"}),
        patch.object(module, "build_schedule_table", return_value=(["姓名"], [["User"]])),
        patch.object(module, "write_excel", side_effect=fake_write),
    ):
        rc, payload = run_main(module, [
            "--users", "u1", "--start", "2026-08-01", "--end", "2026-08-02",
            "--output", "schedule.xlsx", "--format", "json",
        ])
    return rc, payload, writes


def vacation_final(module: ModuleType, state: str) -> tuple[int, dict[str, Any], int]:
    writes = 0

    def fake_types(_inspect: bool, stats: Any) -> list[dict[str, str]]:
        stats.record_success("vacation-types", item_count=1)
        return [{"code": "annual", "name": "年假", "unit": "day", "source": ""}]

    def fake_balances(_users: list[str], _types: list[dict[str, str]], _inspect: bool, stats: Any) -> list[dict[str, Any]]:
        if state == "failure":
            stats.record_failure(
                "vacation-balance:annual:batch:1",
                module.cmn.DwsCallError(
                    "timeout",
                    error_info={"type": "network", "subtype": "timeout", "message": "timeout"},
                ),
            )
            return []
        stats.record_success("vacation-balance:annual:batch:1", item_count=1)
        if state == "partial":
            stats.record_failure(
                "vacation-balance:annual:batch:2",
                module.cmn.DwsCallError("denied", is_permission_error=True),
            )
        return [{"userId": "u1", "leaveCode": "annual", "leaveName": "年假", "balance": 1}]

    def fake_write(*_args: Any, **_kwargs: Any) -> None:
        nonlocal writes
        writes += 1

    with (
        patch.object(module.cmn, "resolve_users_from_input", return_value=["u1"]),
        patch.object(module, "query_leave_types", side_effect=fake_types),
        patch.object(module, "query_balance_records", side_effect=fake_balances),
        patch.object(module, "build_leave_columns", return_value=[{"code": "annual", "name": "年假"}]),
        patch.object(module.cmn, "resolve_user_info", return_value={}),
        patch.object(module, "build_balance_index", return_value={}),
        patch.object(module, "build_user_extra_index", return_value={}),
        patch.object(module, "build_headers", return_value=["姓名", "年假"]),
        patch.object(module, "build_rows", return_value=[["User", 1]]),
        patch.object(module.cmn, "write_excel", side_effect=fake_write),
    ):
        rc, payload = run_main(module, [
            "--users", "u1", "--out", "vacation.xlsx", "--format", "json",
        ])
    return rc, payload, writes


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    checks: list[tuple[str, bool, str]] = []

    for label, root in ROOTS:
        schedule_path = root / "attendance_schedule_export.py"
        vacation_path = root / "attendance_vacation_balance.py"
        for kind, path in (("schedule", schedule_path), ("vacation", vacation_path)):
            help_run = subprocess.run([sys.executable, str(path), "--help"], capture_output=True, text=True)
            checks.append((
                f"{label}-{kind}: Help 可发现 format/dry-run",
                help_run.returncode == 0 and "--format" in help_run.stdout and "--dry-run" in help_run.stdout,
                f"rc={help_run.returncode}",
            ))

        schedule = load(f"{label}_schedule", schedule_path)
        stats = schedule.CallStats()
        with patch.object(schedule, "run_dws", return_value=[]):
            rows = schedule.fetch_schedule_batch(["u1"], "2026-08-01", "2026-08-02", stats, "page:1")
        checks.append((
            f"{label}-schedule: 明确空数组是成功空结果",
            rows == [] and len(stats.succeeded) == 1 and not stats.failed,
            f"succeeded={len(stats.succeeded)} failed={len(stats.failed)}",
        ))
        stats = schedule.CallStats()
        with patch.object(schedule, "run_dws", return_value={"unexpected": []}):
            rows = schedule.fetch_schedule_batch(["u1"], "2026-08-01", "2026-08-02", stats, "page:1")
        checks.append((
            f"{label}-schedule: 未知投影不伪装空排班",
            rows == [] and stats.failed[0]["error"].get("subtype") == "projection_unknown",
            stats.failed[0]["error"].get("subtype", "missing"),
        ))

        for state, expected_rc, expected_outcome, expected_writes in (
            ("success", 0, "success", 1),
            ("partial", 7, "partial_failure", 1),
            ("failure", 1, "failure", 0),
        ):
            rc, payload, writes = schedule_final(schedule, state)
            checks.append((
                f"{label}-schedule: {state} 结果与写入边界",
                rc == expected_rc and payload.get("outcome") == expected_outcome and writes == expected_writes,
                f"rc={rc} outcome={payload.get('outcome')} writes={writes}",
            ))

        vacation = load(f"{label}_vacation", vacation_path)
        stats = vacation.cmn.CallStats()
        with patch.object(vacation.cmn, "run_dws", return_value=[]):
            types = vacation.query_leave_types(False, stats)
        checks.append((
            f"{label}-vacation: 明确空规则数组是成功空结果",
            types == [] and len(stats.succeeded) == 1 and not stats.failed,
            f"succeeded={len(stats.succeeded)} failed={len(stats.failed)}",
        ))
        stats = vacation.cmn.CallStats()
        with patch.object(vacation.cmn, "run_dws", return_value={"unexpected": []}):
            types = vacation.query_leave_types(False, stats)
        checks.append((
            f"{label}-vacation: 未知规则投影不伪装为空",
            types == [] and stats.failed[0]["error"].get("subtype") == "projection_unknown",
            stats.failed[0]["error"].get("subtype", "missing"),
        ))

        for state, expected_rc, expected_outcome, expected_writes in (
            ("success", 0, "success", 1),
            ("partial", 7, "partial_failure", 1),
            ("failure", 1, "failure", 0),
        ):
            rc, payload, writes = vacation_final(vacation, state)
            checks.append((
                f"{label}-vacation: {state} 结果与写入边界",
                rc == expected_rc and payload.get("outcome") == expected_outcome and writes == expected_writes,
                f"rc={rc} outcome={payload.get('outcome')} writes={writes}",
            ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        "# 考勤只读导出脚本结果契约 Agent 探针", "", f"扫描日期：{date.today().isoformat()}", "",
        "> 受控注入排班与假期余额 Mono/Multi 入口；不保存 JSON fixture，不创建 Excel，不证明真实考勤权限或后端终态。",
        "", "| 检查 | 结果 | 证据 |", "|---|---|---|",
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        "", f"结论：**{passed}/{len(checks)} PASS**。", "",
        "范围：证明 Help、已知空/未知投影、success/partial/failure、0/7/1 退出码和本地写入边界；真实数据内容与覆盖仍需隔离/live evidence。", "",
    ])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(checks) else 1


if __name__ == "__main__":
    raise SystemExit(main())
