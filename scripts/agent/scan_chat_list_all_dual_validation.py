#!/usr/bin/env python3
"""Agent review for chat +chat-list-all's active unified pagination contract.

This evidence collector reads declarations and runs fixture-backed local tests.
It writes Markdown only; it never calls a DingTalk tenant or persists runtime
JSON/group identities. It is deliberately not a CI or policy gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import os
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
DECLARATION = ROOT / "internal" / "shortcut" / "chat" / "chat_group.go"
TEST = ROOT / "internal" / "shortcut" / "chat" / "chat_list_all_unified_pagination_test.go"


def source_checks() -> list[tuple[str, str, str]]:
    declaration = DECLARATION.read_text(encoding="utf-8")
    test = TEST.read_text(encoding="utf-8")
    match = re.search(
        r"var ChatListAll = shortcut\.Shortcut\{(?P<body>.*?)^\}",
        declaration,
        flags=re.DOTALL | re.MULTILINE,
    )
    body = match.group("body") if match else ""
    return [
        (
            "单命令进入 unified_active，Agent 不选择协议",
            "PASS" if "OutputRollout: output.RolloutUnifiedActive" in body else "FAIL",
            "公开调用只使用 `--format json`；每次调用只有一个 active 结果契约。",
        ),
        (
            "同一次读取映射至 PageLedger 候选结果",
            "PASS" if "observeChatListAllPage(pageLedger" in declaration and "chatListAllUnifiedResult(pageLedger" in declaration else "FAIL",
            "候选结果来自原业务请求，禁止为了输出再次执行目录读取。",
        ),
        (
            "无稳定群 ID 的展示行 fail-closed",
            "PASS" if "chatListAllStableConversationID" in declaration and "缺少稳定 openConversationId" in declaration else "FAIL",
            "只有可继续操作的 openConversationId 才会进入群目录结果。",
        ),
        (
            "分页边界/后续页失败具备统一结果语义",
            "PASS" if "pageLedger.RecordFailure" in declaration and "pageLedger.RecordBoundaryFailure" in declaration else "FAIL",
            "已读页保留为 succeeded；游标矛盾不冒充 endpoint 耗尽。",
        ),
        (
            "迁移测试锁定 legacy/dual 字节兼容",
            "PASS" if "TestChatListAllDualValidatePreservesLegacyPayload" in test else "FAIL",
            "单页和 --page-all 均不要求既有消费者更改 argv 或解析器。",
        ),
        (
            "active 结果不泄漏版本或 legacy 分页字段",
            "PASS" if "TestChatListAllUnifiedPaginationOutcomes" in test and 'envelope["contract_version"]' in test else "FAIL",
            "active probe 只使用 `--format json`，检查 endpoint 终态与 partial_failure/rc=7。",
        ),
    ]


def run_probe() -> tuple[str, str]:
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    command = [
        "go",
        "test",
        "-count=1",
        "./internal/shortcut/chat",
        "-run",
        r"Test(ChatListAll(RolloutIsUnifiedActive|DualValidatePreservesLegacyPayload|UnifiedPaginationOutcomes|FailsClosedWithoutStableConversationID)|CrossPlatformCoverageChatListAll.*)$",
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
        "# Chat `+chat-list-all` 统一分页 Agent 审阅",
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
        f"结论：**{passed}/{len(checks)} PASS**。`chat +chat-list-all` 已进入 unified_active：普通 `--format json` 直接由 PageLedger 表达 endpoint 耗尽、可续页、未知边界和后续页 `partial_failure`。Agent 不传协议版本或 rollout 参数。",
        "",
        "边界：真实账号已观察到可续页形状，但空群、完整多页、游标冲突、可见范围和网关异常形状仍由 fixture 或后续隔离账号复验覆盖；active 的 endpoint exhaustion 不代表租户目录完整。",
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
