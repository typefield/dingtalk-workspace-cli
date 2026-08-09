#!/usr/bin/env python3
"""Agent semantic scan for MCP server-failure recovery guidance.

This collector verifies the distinction between a pre-execution metadata error
and a tools/call network error whose business effect is unknown. It is not a CI
gate, does not call a DingTalk tenant, and produces Markdown evidence only.
"""

from __future__ import annotations

import argparse
from datetime import date
import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[2]
CLASSIFIER = ROOT / "internal" / "app" / "server_failure_classifier.go"
TEST = ROOT / "internal" / "app" / "server_failure_classifier_test.go"
RUNNER = ROOT / "internal" / "app" / "runner.go"


def source_checks() -> list[tuple[str, str, str]]:
    classifier = CLASSIFIER.read_text(encoding="utf-8")
    test = TEST.read_text(encoding="utf-8")
    runner = RUNNER.read_text(encoding="utf-8")
    return [
        (
            "标准 tools/call MCP / 业务失败进入统一分类器",
            "PASS"
            if runner.count("newServerFailureAPIError(") >= 2
            else "FAIL",
            "MCP isError 与成功传输中的业务 error 不能退回自由文本错误。",
        ),
        (
            "仅 queryToolMeta 前置失败允许安全重试",
            "PASS"
            if "classified.safeToReplay = true" in classifier
            and 'classified.stage = "tool_metadata_lookup"' in classifier
            else "FAIL",
            "已知业务工具尚未执行时，服务端 retryable 才是可采纳的恢复建议。",
        ),
        (
            "不确定网络失败不把服务端 retryable 升格为安全重放",
            "PASS"
            if "!classified.safeToReplay" in classifier
            and "先核对目标状态" in classifier
            else "FAIL",
            "执行状态未知时保留诊断，但让 Agent 先核对，避免重复写入。",
        ),
        (
            "本地 fixture 覆盖两类恢复分支",
            "PASS"
            if "TestCrossPlatformCoverageServerFailureClassifierBackendMetadataUnavailable" in test
            and "TestCrossPlatformCoverageServerFailureClassifierAmbiguousNetworkFailureDoesNotPermitReplay" in test
            else "FAIL",
            "同时验证 metadata retry 与 ambiguous write 的 no-retry 语义。",
        ),
    ]


def run_probe() -> tuple[str, str]:
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    command = [
        "go",
        "test",
        "-count=1",
        "./internal/app",
        "-run",
        r"TestCrossPlatformCoverage(ServerFailureClassifier|ExecuteInvocationClassifiesObservedMCPMetadataFailure)$",
    ]
    completed = subprocess.run(
        command, cwd=ROOT, env=environment, capture_output=True, text=True, timeout=180
    )
    detail = f"rc={completed.returncode}"
    if completed.returncode != 0:
        detail += "; " + (completed.stderr or completed.stdout).strip().replace("\n", " ")[:500]
    return ("PASS" if completed.returncode == 0 else "FAIL"), detail


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; defaults to stdout")
    args = parser.parse_args()
    checks = source_checks()
    status, detail = run_probe()
    checks.append(("fixture-backed recovery semantics", status, detail))
    passed = sum(result == "PASS" for _, result, _ in checks)
    lines = [
        "# MCP 服务端失败恢复 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 在当前工作树执行；它仅验证 runtime 分类器和本地 HTTP fixture，不调用真实 DingTalk，也不保存运行时 JSON fixture。",
        "",
        "| 检查 | 结果 | 证据 |",
        "|---|---|---|",
    ]
    for name, result, detail in checks:
        lines.append(f"| {name} | {result} | {detail.replace('|', ' / ')} |")
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。MCP metadata lookup 已明确失败且未执行业务工具时，Agent 可按 `retryable:true` 重试；其他网络错误即使服务端提示 retryable，仍视为业务效果未知，必须先核对目标状态。",
        "",
        "未验证：真实网关的每个 error code 是否准确标注前置/执行中阶段，以及真实写请求的最终效果；这些需要隔离账号与故障注入环境复验。",
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
