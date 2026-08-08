#!/usr/bin/env python3
"""Agent semantic scan for message-history cursor precision boundaries.

This is an evidence collector, not a CI gate. It checks the source-level
normalizers and their local fixture probes without calling a DingTalk tenant.
Only Markdown evidence is persisted.
"""

from __future__ import annotations

import argparse
from datetime import date
import os
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[2]
CHAT_MESSAGES = ROOT / "internal" / "shortcut" / "smart" / "chat_messages.go"
THREAD_REPLIES = ROOT / "internal" / "shortcut" / "smart" / "thread_replies.go"
CHAT_MESSAGES_TEST = ROOT / "internal" / "shortcut" / "smart" / "chat_messages_test.go"
THREAD_REPLIES_TEST = ROOT / "internal" / "shortcut" / "smart" / "thread_replies_pagination_test.go"
THREAD_UNIFIED_TEST = ROOT / "internal" / "shortcut" / "smart" / "thread_replies_unified_pagination_test.go"


def source_checks() -> list[tuple[str, str, str]]:
    chat_messages = CHAT_MESSAGES.read_text(encoding="utf-8")
    thread_replies = THREAD_REPLIES.read_text(encoding="utf-8")
    chat_test = CHAT_MESSAGES_TEST.read_text(encoding="utf-8")
    thread_test = THREAD_REPLIES_TEST.read_text(encoding="utf-8")
    thread_unified_test = THREAD_UNIFIED_TEST.read_text(encoding="utf-8")
    strict_boundary = ">= float64(math.MaxInt64)"
    return [
        (
            "chat-messages 拒绝浮点 MaxInt64 精度边界",
            "PASS" if strict_boundary in chat_messages and "float64 max int64 boundary" in chat_test else "FAIL",
            "浮点 JSON number 在该边界会舍入，不能再转换为 int64 游标。",
        ),
        (
            "thread-replies 拒绝浮点 MaxInt64 精度边界",
            "PASS" if strict_boundary in thread_replies and "float64 max int64 boundary" in thread_test else "FAIL",
            "话题回复与消息历史使用同一 fail-closed 规则。",
        ),
        (
            "终页零游标仍被识别为 endpoint exhausted",
            "PASS" if "terminal zero cursor is exhausted success" in thread_unified_test else "FAIL",
            "修复异常浮点边界不能把 API 的 nextCursor=0 终态哨兵误判为失败。",
        ),
        (
            "Agent 公开调用仍为 --format json",
            "PASS" if '"--format", "json"' in thread_unified_test else "FAIL",
            "验证不引入协议版本或专用 cursor 参数。",
        ),
    ]


def run_probe() -> tuple[str, str]:
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    command = [
        "go", "test", "-count=1", "./internal/shortcut/smart",
        "-run", r"Test(CrossPlatformCoverage(ThreadRepliesNextCursorBoundaryValidation|ChatMessagesAdditionalValidationAndHelpers)|ThreadRepliesUnifiedPaginationOutcomes)$",
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
    checks.append(("fixture-backed precision/terminal semantics", status, detail))
    passed = sum(status == "PASS" for _, status, _ in checks)
    lines = [
        "# Chat 消息分页游标精度 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 在当前工作树执行此扫描；它只验证本地归一化逻辑和 fixture，不调用真实 DingTalk，也不保存 JSON fixture。",
        "",
        "| 检查 | 结果 | 证据 |",
        "|---|---|---|",
    ]
    for name, check_status, check_detail in checks:
        lines.append(f"| {name} | {check_status} | {check_detail.replace('|', ' / ')} |")
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。消息历史与话题回复均拒绝浮点 JSON number 无法精确表示的 `MaxInt64` 游标，避免生成错误的续页时间；`nextCursor=0` 仍只表示 endpoint 已耗尽。",
        "",
        "未验证：真实网关返回值的数值解码类型与服务端分页终态；这些仍需真实账号复验。",
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
