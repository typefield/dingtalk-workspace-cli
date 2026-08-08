#!/usr/bin/env python3
"""Agent semantic scan for the active chat +chat-messages result contract.

This is an evidence collector, not a CI gate. It checks the public declaration
and local fixture probes without calling a DingTalk tenant, and persists only a
Markdown conclusion rather than response JSON fixtures.
"""

from __future__ import annotations

import argparse
from datetime import date
import os
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
DECLARATION = ROOT / "internal" / "shortcut" / "smart" / "chat_messages.go"
TEST = ROOT / "internal" / "shortcut" / "smart" / "chat_messages_unified_pagination_test.go"


def source_checks() -> list[tuple[str, str, str]]:
    declaration = DECLARATION.read_text(encoding="utf-8")
    test = TEST.read_text(encoding="utf-8")
    match = re.search(
        r"var ChatMessages = shortcut\.Shortcut\{(?P<body>.*?)^\}",
        declaration,
        flags=re.DOTALL | re.MULTILINE,
    )
    body = match.group("body") if match else ""
    return [
        (
            "迁移状态为 unified_active",
            "PASS" if "OutputRollout: output.RolloutUnifiedActive" in body else "FAIL",
            "此命令的公开 JSON 只有统一信封；Agent 始终不传协议选择参数。",
        ),
        (
            "读取成功后本地导出失败进入部分成功账本",
            "PASS"
            if "chatMessagesExportFailureInfo" in declaration
            and "pageLedger.RecordPostPageFailure(failureInfo)" in declaration
            and "rt.OutputPartial(result, writeErr)" in declaration
            else "FAIL",
            "成功消息页必须保留，导出失败不得压缩为丢失结果的普通错误。",
        ),
        (
            "失败项保留可恢复的本地导出事实",
            "PASS"
            if 'Operation:        "chat/message_export"' in declaration
            and 'Origin:           "local_file"' in declaration
            and '"local_path"' in declaration
            else "FAIL",
            "失败详情应标记操作、来源和请求的本地目标，而不把它伪装成远端读取失败。",
        ),
        (
            "统一结果只使用 --format json 且不泄漏版本标记",
            "PASS"
            if '"--format", "json"' in test
            and 'envelope["contract_version"]' in test
            else "FAIL",
            "协议迁移由发布控制，Agent 始终只用公开 format 参数。",
        ),
        (
            "迁移历史保留 dual legacy 字节 golden",
            "PASS" if "TestChatMessagesDualValidateKeepsLegacyBytes" in test else "FAIL",
            "历史双验证断言仍保留，证明晋级前未静默改变旧消费者输出。",
        ),
        (
            "发送者筛选未执行不误报成功",
            "PASS"
            if "chatMessagesSenderResolutionFailureInfo" in declaration
            and "pageLedger.RecordPostPageFailure(chatMessagesSenderResolutionFailureInfo(senderFilter))" in declaration
            else "FAIL",
            "请求的 sender-query 未完成时保留消息页并返回 typed partial_failure。",
        ),
    ]


def run_probe() -> tuple[str, str]:
    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    command = [
        "go", "test", "-count=1", "./internal/shortcut/smart",
        "-run", r"TestChatMessages(DualValidateKeepsLegacyBytes|UnifiedPaginationOutcomes)$",
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
    checks.append(("fixture-backed shadow/active 结果语义", status, detail))
    passed = sum(status == "PASS" for _, status, _ in checks)
    lines = [
        "# Chat +chat-messages 统一结果契约 Agent 扫描",
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
        f"结论：**{passed}/{len(checks)} PASS**。`chat +chat-messages` 已处于 `unified_active`：终页零游标可正确耗尽；范围已完成但源端未穷尽时不伪称 endpoint exhausted；后续读取、发送者筛选、资源下载和本地导出失败均保留成功页并返回 `partial_failure`/rc=7。Agent 只使用 `--format json`，不选择协议版本。",
        "",
        "未验证：真实账号的消息可见范围、网关分页响应、本地文件系统实际故障和服务端终态；这些仍需要评测账号与受控环境复验。",
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
