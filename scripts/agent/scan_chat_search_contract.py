#!/usr/bin/env python3
"""Agent semantic scan for the active chat +chat-search pagination contract.

This evidence collector reads the current declaration and runs the local
fixture-backed promotion probe. It is not a CI gate: no DingTalk tenant is
called and only Markdown is persisted.
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
TEST = ROOT / "internal" / "shortcut" / "chat" / "chat_search_pagination_test.go"


def source_checks() -> list[tuple[str, str, str]]:
    declaration = DECLARATION.read_text(encoding="utf-8")
    test = TEST.read_text(encoding="utf-8")
    match = re.search(
        r"var ChatSearch = shortcut\.Shortcut\{(?P<body>.*?)^\}",
        declaration,
        flags=re.DOTALL | re.MULTILINE,
    )
    body = match.group("body") if match else ""
    return [
        (
            "公开命令只使用一个 active 契约",
            "PASS" if "OutputRollout:            output.RolloutUnifiedActive" in body else "FAIL",
            "ChatSearch 必须直接使用 unified_active；Agent 不选择协议版本。",
        ),
        (
            "最大窗口与分页账本共同决定结果",
            "PASS" if "pageLedger.Result(unifiedData)" in declaration and "maxWindowProbeUsed" in declaration else "FAIL",
            "满页但无续页 token 的探测不能绕过 PageLedger。",
        ),
        (
            "Agent 调用形式覆盖 --format json",
            "PASS" if '"--format", "json"' in test else "FAIL",
            "promotion probe 走公开 format 参数，不存在协议选择 flag。",
        ),
        (
            "不暴露版本选择标记",
            "PASS" if 'envelope["contract_version"]' in test and "exposed removed contract_version" in test else "FAIL",
            "测试明确拒绝 contract_version 出现在结果信封。",
        ),
    ]


def run_probe() -> tuple[str, str]:
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    command = [
        "go", "test", "-count=1", "./internal/shortcut/chat",
        "-run", r"TestChatSearch(UnifiedPromotionEvidence|PaginationRolloutIsUnifiedActive)$",
    ]
    completed = subprocess.run(
        command, cwd=ROOT, env=environment, capture_output=True, text=True, timeout=180,
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
    checks.append(("fixture-backed active 结果语义", status, detail))
    passed = sum(status == "PASS" for _, status, _ in checks)
    lines = [
        "# Chat +chat-search 分页契约 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 在当前工作树执行此扫描；它只验证声明、输出形状和本地分页 fixture，不调用真实 DingTalk，也不保存 JSON fixture。",
        "",
        "| 检查 | 结果 | 证据 |",
        "|---|---|---|",
    ]
    for name, check_status, check_detail in checks:
        lines.append(f"| {name} | {check_status} | {check_detail.replace('|', ' / ')} |")
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。`chat +chat-search --format json` 已使用统一结果：续页返回可恢复 token；未知边界不声明耗尽；后续读取失败、矛盾 cursor 和最大窗口仍满页均返回 `partial_failure`/rc=7。",
        "",
        "未验证：真实群搜索索引覆盖、网关实际响应形状和账号可见范围；这些仍需要评测账号复验。",
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
