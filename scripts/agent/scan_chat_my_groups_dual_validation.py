#!/usr/bin/env python3
"""Agent review for chat +my-groups' active unified pagination contract.

This is an evidence collector, not a CI gate. It reads declarations and runs
fixture-backed local tests only. It writes Markdown, never runtime JSON or
real group identities.
"""

from __future__ import annotations

import argparse
from datetime import date
import os
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
DECLARATION = ROOT / "internal" / "shortcut" / "smart" / "my_groups.go"
TEST = ROOT / "internal" / "shortcut" / "smart" / "my_groups_unified_pagination_test.go"


def source_checks() -> list[tuple[str, str, str]]:
    declaration = DECLARATION.read_text(encoding="utf-8")
    test = TEST.read_text(encoding="utf-8")
    match = re.search(r"var MyGroups = shortcut\.Shortcut\{(?P<body>.*?)^\}", declaration, flags=re.DOTALL | re.MULTILINE)
    body = match.group("body") if match else ""
    return [
        (
            "单命令进入 unified_active，未让 Agent 选择协议",
            "PASS" if "OutputRollout: output.RolloutUnifiedActive" in body else "FAIL",
            "公开调用只使用 `--format json`；每次调用只有一个 active 结果契约。",
        ),
        (
            "同一次读取进入 PageLedger 候选结果",
            "PASS" if "observeMyGroupsPage(pageLedger" in declaration and "myGroupsUnifiedResult(pageLedger" in declaration else "FAIL",
            "候选输出从实际读取派生，不得为双 renderer 重新执行业务请求。",
        ),
        (
            "完整性和部分失败由框架语义表达",
            "PASS" if "pageLedger.RecordFailure" in declaration and "pageLedger.RecordBoundaryFailure" in declaration else "FAIL",
            "后续页失败保留已读页；游标矛盾不宣称 endpoint exhausted。",
        ),
        (
            "迁移测试仍锁定 legacy/dual 字节结果",
            "PASS" if "TestMyGroupsDualValidatePreservesLegacyPayload" in test else "FAIL",
            "迁移期不要求下游 Agent 或脚本改 argv / 改解析器。",
        ),
        (
            "active 路径不泄漏版本或 legacy 分页字段",
            "PASS" if "TestMyGroupsUnifiedPaginationOutcomes" in test and 'envelope["contract_version"]' in test else "FAIL",
            "active probe 只用 `--format json`，检查 partial_failure/rc=7。",
        ),
        (
            "active 群句柄与 IM 后续命令对齐",
            "PASS" if "myGroupsUnifiedGroups" in declaration and 'group["openConversationId"]' in test else "FAIL",
            "兼容期仍保留 legacy conversationId；统一候选只发布可直接传入 IM 后续命令的 openConversationId。",
        ),
    ]


def run_probe() -> tuple[str, str]:
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    command = [
        "go", "test", "-count=1", "./internal/shortcut/smart",
        "-run", r"TestMyGroups(RolloutIsUnifiedActive|DualValidatePreservesLegacyPayload|UnifiedPaginationOutcomes|CandidatePaginationContract|CandidateKeepsFirstPageOnLaterFailure)$",
    ]
    completed = subprocess.run(command, cwd=ROOT, env=environment, capture_output=True, text=True, timeout=180)
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
    checks.append(("fixture-backed dual/active 候选语义", status, detail))
    passed = sum(status == "PASS" for _, status, _ in checks)

    lines = [
        "# Chat `+my-groups` 统一分页 Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本审只读取当前声明并运行本地 fixture；不调用 DingTalk 租户、不保存群名、conversationId 或 JSON fixture，也不是 CI / policy gate。",
        "",
        "| 检查 | 结果 | 证据 |",
        "|---|---|---|",
    ]
    for name, check_status, detail in checks:
        lines.append(f"| {name} | {check_status} | {detail.replace('|', ' / ')} |")
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。`chat +my-groups` 已进入 unified_active：普通 `--format json` 直接由 PageLedger 表达 endpoint 耗尽、可续页、未知边界与后续页 `partial_failure`。Agent 不传协议版本或 rollout 参数。",
        "",
        "边界：真实账号已观察到可续页形状，但空群、完整跨页、游标冲突、可见范围和网关异常形状仍由 fixture 或后续隔离账号复验覆盖；active 的 endpoint exhaustion 不代表租户目录完整。",
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
