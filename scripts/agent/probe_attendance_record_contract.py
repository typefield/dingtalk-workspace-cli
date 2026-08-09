#!/usr/bin/env python3
"""Agent probe for Mono/Multi attendance record report result truthfulness."""

from __future__ import annotations

import argparse
import contextlib
import importlib.util
import io
import json
import subprocess
import sys
from argparse import Namespace
from datetime import date
from pathlib import Path
from types import ModuleType
from typing import Any
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
ENTRIES = (
    ("mono", ROOT / "skills/mono/scripts/attendance_report_record.py"),
    ("multi", ROOT / "skills/multi/dingtalk-misc/scripts/attendance_report_record.py"),
)


def load(label: str, path: Path) -> ModuleType:
    for name in ("attendance_report_common", "_runtime"):
        sys.modules.pop(name, None)
    sys.path.insert(0, str(path.parent))
    try:
        spec = importlib.util.spec_from_file_location(f"attendance_record_{label}", path)
        if spec is None or spec.loader is None:
            raise RuntimeError(f"cannot load {path}")
        module = importlib.util.module_from_spec(spec)
        sys.modules[spec.name] = module
        spec.loader.exec_module(module)
        return module
    finally:
        sys.path.pop(0)


def args_for(output: str = "record.xlsx") -> Namespace:
    return Namespace(
        type="trip", users="u1", start="2026-08-01", end="2026-08-02",
        out=output, format="json", dry_run=False,
    )


def execute_main(module: ModuleType, *, partial: bool, fail_all: bool) -> tuple[int, dict[str, Any], int]:
    writes = 0

    def fake_approve(_users: list[str], _kind: str, _start: str, _end: str, stats: Any) -> list[dict]:
        if fail_all:
            stats.record_failure(
                "approve-list:batch:1",
                module.DwsCallError(
                    "timeout",
                    error_info={"type": "network", "subtype": "timeout", "message": "timeout"},
                ),
            )
            return []
        stats.record_success("approve-list:batch:1", item_count=1)
        if partial:
            stats.record_failure(
                "approve-list:batch:2",
                module.DwsCallError(
                    "denied",
                    is_permission_error=True,
                ),
            )
        return [{"userId": "u1", "originId": "p1", "tagName": "出差"}]

    def fake_write(*_args: Any, **_kwargs: Any) -> None:
        nonlocal writes
        writes += 1

    stdout = io.StringIO()
    stderr = io.StringIO()
    row = ["value"] * len(module.COLUMNS["trip"])
    with (
        patch.object(module, "parse_args", return_value=args_for()),
        patch.object(module, "fetch_approve_list", side_effect=fake_approve),
        patch.object(module, "resolve_user_names", return_value={"u1": "User"}),
        patch.object(module, "resolve_user_info", return_value={}),
        patch.object(module, "fetch_user_group_map", return_value={}),
        patch.object(module, "parse_trip_from_approve_record", return_value=row),
        patch.object(module, "write_excel", side_effect=fake_write),
    ):
        with (
            patch.object(sys, "argv", [str(module.__file__), "--format", "json"]),
            contextlib.redirect_stdout(stdout),
            contextlib.redirect_stderr(stderr),
        ):
            rc = module.run_main(module.main)
    payload = json.loads(stdout.getvalue())
    return rc, payload, writes


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    checks: list[tuple[str, bool, str]] = []

    for label, path in ENTRIES:
        help_run = subprocess.run(
            [sys.executable, str(path), "--help"], capture_output=True, text=True,
        )
        checks.append((
            f"{label}: Help 可发现 format/dry-run",
            help_run.returncode == 0 and "--format" in help_run.stdout and "--dry-run" in help_run.stdout,
            f"rc={help_run.returncode}",
        ))
        module = load(label, path)

        stats = module.CallStats()
        with patch.object(module, "run_dws", return_value=[]):
            records = module.fetch_approve_list(["u1"], "leave", "2026-08-01", "2026-08-02", stats)
        checks.append((
            f"{label}: 明确空审批列表是成功空结果",
            records == [] and len(stats.succeeded) == 1 and not stats.failed,
            f"succeeded={len(stats.succeeded)} failed={len(stats.failed)}",
        ))

        stats = module.CallStats()
        with patch.object(module, "run_dws", return_value={"unexpected": []}):
            records = module.fetch_approve_list(["u1"], "leave", "2026-08-01", "2026-08-02", stats)
        checks.append((
            f"{label}: 未知审批列表投影不伪装为空",
            records == [] and not stats.succeeded
            and stats.failed[0]["error"].get("subtype") == "projection_unknown",
            f"succeeded={len(stats.succeeded)} failed={len(stats.failed)}",
        ))

        stats = module.CallStats()
        denied = module.DwsCallError("denied", is_permission_error=True)
        with patch.object(module, "run_dws", side_effect=denied):
            records = module.fetch_approve_list(["u1"], "leave", "2026-08-01", "2026-08-02", stats)
        checks.append((
            f"{label}: 权限失败保留 authorization",
            records == [] and stats.failed[0]["error"].get("type") == "authorization",
            stats.failed[0]["error"].get("type", "missing"),
        ))

        stats = module.CallStats()
        with patch.object(module, "run_dws", return_value=[]):
            detail = module.fetch_detail("p1", stats)
        checks.append((
            f"{label}: 非对象审批详情成为 projection failure",
            detail is None and stats.failed[0]["error"].get("subtype") == "projection_unknown",
            stats.failed[0]["error"].get("subtype", "missing"),
        ))

        rc, payload, writes = execute_main(module, partial=False, fail_all=False)
        checks.append((
            f"{label}: 完整结果保持 success 与旧 data 形状",
            rc == 0 and payload.get("outcome") == "success" and writes == 1
            and payload.get("data", {}).get("rowCount") == 1
            and "succeeded" not in payload.get("data", {}),
            f"rc={rc} writes={writes} outcome={payload.get('outcome')}",
        ))

        rc, payload, writes = execute_main(module, partial=True, fail_all=False)
        checks.append((
            f"{label}: 部分读取保留文件并返回 rc7 ledger",
            rc == 7 and payload.get("outcome") == "partial_failure" and writes == 1
            and bool(payload.get("data", {}).get("succeeded"))
            and bool(payload.get("data", {}).get("failed")),
            f"rc={rc} writes={writes} outcome={payload.get('outcome')}",
        ))

        rc, payload, writes = execute_main(module, partial=False, fail_all=True)
        checks.append((
            f"{label}: 全批失败不写文件并返回 typed failure",
            rc == 1 and payload.get("outcome") == "failure" and writes == 0
            and payload.get("error", {}).get("subtype") == "timeout",
            f"rc={rc} writes={writes} outcome={payload.get('outcome')}",
        ))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        "# 考勤记录报表结果契约 Agent 探针",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 受控注入 Mono/Multi 的列表、详情与最终写入边界；不保存 JSON fixture，不创建 Excel，不证明真实审批/考勤权限或后端终态。",
        "",
        "| 检查 | 结果 | 证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |" for name, ok, detail in checks)
    lines.extend([
        "", f"结论：**{passed}/{len(checks)} PASS**。", "",
        "范围：证明入口发现、已知空/未知投影、权限分类、完整 success、partial/rc7 和全失败零本地写；真实服务端与 Excel 内容仍需隔离/live evidence。", "",
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
