#!/usr/bin/env python3
"""Agent semantic scan for the unified, destructive `event stop` contract.

This is an evidence collector, not a CI gate. It checks the release-owned
rollout declaration plus local lifecycle fixtures. It never contacts a DingTalk
tenant and writes Markdown conclusions only; it does not preserve response JSON
fixtures.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[2]
EVENT_COMMAND = ROOT / "internal" / "app" / "event_command.go"
PERSONAL_COMMAND = ROOT / "internal" / "app" / "event_personal_command.go"
OUTCOME_TEST = ROOT / "internal" / "app" / "event_personal_outcome_test.go"
SAFETY_TEST = ROOT / "internal" / "app" / "event_stop_safety_test.go"


def source_checks() -> list[tuple[str, str, str]]:
    event = EVENT_COMMAND.read_text(encoding="utf-8")
    personal = PERSONAL_COMMAND.read_text(encoding="utf-8")
    outcome_test = OUTCOME_TEST.read_text(encoding="utf-8")
    safety_test = SAFETY_TEST.read_text(encoding="utf-8")
    return [
        (
            "迁移状态为 unified_active",
            "PASS" if "output.SetCommandRollout(cmd, output.RolloutUnifiedActive)" in event else "FAIL",
            "协议由发布声明决定；Agent 仅传 --format json，不选择版本。",
        ),
        (
            "dry-run 是完整 success 结果而非停机成功",
            "PASS"
            if "eventStopDryRunPayload" in event
            and "output.WithDryRun()" in event
            and "output.StoreResult(cmd.Context(), output.Success(" in event
            else "FAIL",
            "预览经统一 lifecycle 输出 dry_run:true，确认门禁前不调用取消路径。",
        ),
        (
            "已确认取消与未知后续状态保留三通道",
            "PASS"
            if "eventStopOutcomeResult" in personal
            and "output.Partial(partial)" in personal
            and "output.StoreResult(c.Context(), result)" in personal
            else "FAIL",
            "已取消订阅进入 succeeded[]；后续清理/控制面不确定进入 unknown[]，rc=7。",
        ),
        (
            "普通终态成功也走 StoreResult",
            "PASS"
            if "writeEventStopSuccess" in event
            and "writePersonalEventStopSuccess" in personal
            else "FAIL",
            "不会只统一失败、让成功继续手写 stdout。",
        ),
        (
            "结果声明覆盖 success/partial_failure/failure",
            "PASS"
            if "contract.ResultOutcomePartialFailure" in event
            and "contract.ResultOutcomeFailure" in event
            else "FAIL",
            "Schema/Skill 发现层可获得真实终态集合。",
        ),
        (
            "本地 fixture 覆盖 active 的 preview、success 与 partial",
            "PASS"
            if "TestEventStopUnifiedActiveStoresPartialResult" in outcome_test
            and "TestEventStopUnifiedActiveStoresTerminalSuccess" in outcome_test
            and "output.EmitStoredResult(cmd)" in outcome_test
            and "PersistentPostRunE" in safety_test
            else "FAIL",
            "测试根挂载真实 ResultStore/PostRun，覆盖 preview、success 与 partial，避免把 direct Cobra 调用误当生产生命周期。",
        ),
    ]


def run_fixture_probe() -> tuple[str, str]:
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    command = [
        "go",
        "test",
        "-count=1",
        "./internal/app",
        "-run",
        r"TestEventStop(UnifiedActiveStoresPartialResult|UnifiedActiveStoresTerminalSuccess|DryRunPrecedesConfirmationAndReturnsPreview|RequiresTypedConfirmationBeforeMutation|StartsUnifiedActive)$",
    ]
    completed = subprocess.run(
        command, cwd=ROOT, env=environment, capture_output=True, text=True, timeout=180
    )
    detail = f"rc={completed.returncode}"
    if completed.returncode != 0:
        detail += "; " + (completed.stderr or completed.stdout).strip().replace("\n", " ")[:500]
    return ("PASS" if completed.returncode == 0 else "FAIL"), detail


def run_cli_probe() -> tuple[str, str]:
    """Exercise safe public paths without retaining their JSON responses."""
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    base = ["go", "run", "./cmd", "--format", "json", "event", "stop", "sub-agent-scan"]
    preview = subprocess.run(
        [*base, "--dry-run"], cwd=ROOT, env=environment, capture_output=True, text=True, timeout=180
    )
    rejected = subprocess.run(
        base, cwd=ROOT, env=environment, capture_output=True, text=True, timeout=180
    )
    try:
        preview_body = json.loads(preview.stdout)
        rejected_body = json.loads(rejected.stdout)
    except json.JSONDecodeError as exc:
        return "FAIL", f"JSON decode failed: {exc}"
    preview_ok = (
        preview.returncode == 0
        and preview_body.get("ok") is True
        and preview_body.get("outcome") == "success"
        and preview_body.get("dry_run") is True
        and preview_body.get("data", {}).get("action") == "event.stop"
    )
    # `go run` itself returns 1 when its child returns 3. The stable process
    # result under test is the envelope's framework-derived exit_code.
    rejected_ok = (
        rejected_body.get("ok") is False
        and rejected_body.get("outcome") == "failure"
        and rejected_body.get("error", {}).get("type") == "validation"
        and rejected_body.get("error", {}).get("subtype") == "confirmation_required"
        and rejected_body.get("error", {}).get("exit_code") == 3
    )
    no_version = "contract_version" not in preview_body and "contract_version" not in rejected_body
    if preview_ok and rejected_ok and no_version:
        return "PASS", "dry-run success + no-confirm typed validation; no version marker"
    return "FAIL", "public dry-run or confirmation envelope violated the contract"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; defaults to stdout")
    args = parser.parse_args()
    checks = source_checks()
    status, detail = run_fixture_probe()
    checks.append(("fixture-backed active lifecycle", status, detail))
    status, detail = run_cli_probe()
    checks.append(("public CLI dry-run / confirmation boundary", status, detail))
    passed = sum(result == "PASS" for _, result, _ in checks)
    lines = [
        "# Event stop 统一结果契约 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 在当前工作树执行此扫描；它只验证声明、门禁和本地 lifecycle fixture，不调用真实 DingTalk，也不保存 JSON fixture。",
        "",
        "| 检查 | 结果 | 证据 |",
        "|---|---|---|",
    ]
    for name, result, detail in checks:
        lines.append(f"| {name} | {result} | {detail.replace('|', ' / ')} |")
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。`event stop` 已处于 `unified_active`：缺确认会在写入前失败；dry-run 是可解析预览；确认取消后若后续阶段不可确认，已完成项不会丢失而是输出 `partial_failure`/rc=7。Agent 只使用 `--format json`，不选择协议版本。",
        "",
        "未验证：真实订阅取消后的远端订阅、本地消费者进程和 run-state 三者是否最终一致；这仍需要隔离账号和受控进程环境复验，不能由本地 fixture 推断。",
        "",
    ]
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(checks) else 1


if __name__ == "__main__":
    raise SystemExit(main())
