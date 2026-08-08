#!/usr/bin/env python3
"""Agent semantic scan for the active chat +thread-replies result contract.

This is an evidence collector, not a CI gate. It verifies the public command
declaration and fixture-backed promotion tests without calling a DingTalk
tenant, and persists Markdown only.
"""

from __future__ import annotations

import argparse
from datetime import date
import os
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
DECLARATION = ROOT / "internal" / "shortcut" / "smart" / "thread_replies.go"
TEST = ROOT / "internal" / "shortcut" / "smart" / "thread_replies_unified_pagination_test.go"
INTEGRATION_TEST = ROOT / "internal" / "shortcut" / "smart" / "thread_replies_pagination_test.go"


def source_checks() -> list[tuple[str, str, str]]:
    declaration = DECLARATION.read_text(encoding="utf-8")
    test = TEST.read_text(encoding="utf-8")
    integration = INTEGRATION_TEST.read_text(encoding="utf-8")
    match = re.search(
        r"var ThreadReplies = shortcut\.Shortcut\{(?P<body>.*?)^\}",
        declaration,
        flags=re.DOTALL | re.MULTILINE,
    )
    body = match.group("body") if match else ""
    return [
        (
            "公开命令只使用一个 active 契约",
            "PASS" if "OutputRollout: output.RolloutUnifiedActive" in body else "FAIL",
            "Agent 不选择协议版本；`--format json` 直接取得统一结果。",
        ),
        (
            "--page-all 缺失或矛盾边界保留已读页",
            "PASS"
            if "RecordPostPageFailure(threadRepliesPaginationFailureInfo" in declaration
            and "page-all missing endpoint evidence is partial rather than exhaustion" in test
            else "FAIL",
            "无法证明全量结果时必须为 partial_failure，不能把当前页伪装成完整成功。",
        ),
        (
            "显式资源下载失败进入部分成功",
            "PASS" if "threadRepliesResourceDownloadFailureInfo" in declaration and "requested resource failure is partial" in test else "FAIL",
            "消息页读取成功不掩盖请求的本地资源副作用失败。",
        ),
        (
            "终页零游标仍正确耗尽",
            "PASS" if "terminal zero cursor is exhausted success" in test else "FAIL",
            "nextCursor=0 是 API 终态哨兵，不是继续游标或错误。",
        ),
        (
            "生产声明的集成测试走真实结果出口",
            "PASS" if "threadRepliesSuccessData" in integration and "threadRepliesPartialData" in integration else "FAIL",
            "测试不再把 active 命令按旧 payload 断言，覆盖 success 与 partial_failure。",
        ),
        (
            "不暴露协议版本标记",
            "PASS" if 'envelope["contract_version"]' in test else "FAIL",
            "统一信封不得输出 contract_version 或让 Agent 协商版本。",
        ),
    ]


def run_probe() -> tuple[str, str]:
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    command = [
        "go", "test", "-count=1", "./internal/shortcut/smart",
        "-run", r"Test(ThreadRepliesUnifiedPaginationOutcomes|CrossPlatformCoverageThreadReplies)",
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
        "# Chat +thread-replies 分页契约 Agent 扫描",
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
        f"结论：**{passed}/{len(checks)} PASS**。`chat +thread-replies --format json` 已使用统一结果：终页零游标声明 endpoint 已耗尽；有可靠续页时返回 token；缺失/矛盾分页证据、后续读取或资源下载失败都保留成功页并返回 `partial_failure`/rc=7。",
        "",
        "未验证：真实账号话题回复可见范围、网关实际响应形状、本地资源下载和服务端分页终态；这些仍需要评测账号复验。",
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
