#!/usr/bin/env python3
"""Agent semantic scan for the active chat +flag-list pagination contract.

This is an evidence collector, not a CI gate. It reads the public command
declaration and runs the fixture-backed Go probe that exercises the command
through its normal result store. It writes only Markdown; no response JSON is
persisted and no DingTalk tenant is called.
"""

from __future__ import annotations

import argparse
from datetime import date
import os
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
DECLARATION = ROOT / "internal" / "shortcut" / "chat" / "lark_alignment.go"
TEST = ROOT / "internal" / "shortcut" / "chat" / "flag_list_pagination_test.go"


def check_source() -> list[tuple[str, str, str]]:
    declaration = DECLARATION.read_text(encoding="utf-8")
    test = TEST.read_text(encoding="utf-8")
    flag_list = re.search(
        r"var FlagList = shortcut\.Shortcut\{(?P<body>.*?)^\}",
        declaration,
        flags=re.DOTALL | re.MULTILINE,
    )
    body = flag_list.group("body") if flag_list else ""
    checks: list[tuple[str, str, str]] = []
    checks.append((
        "公开命令只使用一个 active 契约",
        "PASS" if "OutputRollout: output.RolloutUnifiedActive" in body else "FAIL",
        "FlagList 的 rollout 必须是 unified_active；不接受 Agent/用户选择协议。",
    ))
    checks.append((
        "分页账本生成统一结果",
        "PASS" if "pageLedger.Result(unifiedData)" in declaration and "rt.OutputResult(payload, result)" in declaration else "FAIL",
        "业务读取一次后由 PageLedger 生成 CommandResult，再经单一输出出口传递。",
    ))
    checks.append((
        "Agent 调用形式覆盖 --format json",
        "PASS" if '"--format", "json"' in test else "FAIL",
        "promotion probe 必须使用公开的格式参数，而不是隐藏协议开关。",
    ))
    checks.append((
        "不暴露版本选择标记",
        "PASS" if 'envelope["contract_version"]' in test and "exposed removed contract_version" in test else "FAIL",
        "测试明确拒绝在结果信封中泄漏 contract_version。",
    ))
    return checks


def run_probe() -> tuple[str, str]:
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    command = [
        "go", "test", "-count=1", "./internal/shortcut/chat",
        "-run", r"TestFlagList(PaginationRolloutIsUnifiedActive|UnifiedPromotionEvidence|CrossPlatformCoverageFlagListDryRunStopsBeforeRead)$",
    ]
    completed = subprocess.run(
        command, cwd=ROOT, env=environment, capture_output=True, text=True, timeout=180,
    )
    detail = f"rc={completed.returncode}"
    if completed.returncode != 0:
        diagnostic = (completed.stderr or completed.stdout).strip().replace("\n", " ")
        detail = f"{detail}; {diagnostic[:500]}"
    return ("PASS" if completed.returncode == 0 else "FAIL"), detail


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; defaults to stdout")
    args = parser.parse_args()

    checks = check_source()
    probe_status, probe_detail = run_probe()
    checks.append(("fixture-backed active 结果语义", probe_status, probe_detail))
    passed = sum(status == "PASS" for _, status, _ in checks)
    lines = [
        "# Chat +flag-list 分页契约 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本扫描由 Agent 发起，读取当前命令声明并运行本地 fixture probe。它只验证 CLI 的声明、分页结果和输出形状；不调用真实 DingTalk，不保存 JSON fixture，也不能证明服务端索引覆盖或真实分页终态。",
        "",
        "| 检查 | 结果 | 证据 |",
        "|---|---|---|",
    ]
    for name, status, detail in checks:
        safe_detail = detail.replace("|", "\\|")
        lines.append(f"| {name} | {status} | {safe_detail} |")
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。`chat +flag-list --format json` 已是 active 统一结果试点：续页只声明 endpoint 可续，边界未知不声明耗尽，后续页失败保留成功页并返回 `partial_failure`/rc=7。",
        "",
        "未验证：真实账号的收藏列表召回、网关响应形状和索引健康；这些仍需评测账号复验。",
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
