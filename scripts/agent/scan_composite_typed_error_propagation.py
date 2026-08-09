#!/usr/bin/env python3
"""Agent-only review of typed errors retained by composite read commands.

This scan is intentionally an evidence-producing Agent review.  It is not a
CI/policy gate and writes Markdown only; it neither stores JSON fixtures nor
contacts a remote service.
"""

from __future__ import annotations

import argparse
import datetime as dt
import os
from pathlib import Path
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
HELPER = ROOT / "internal/shortcut/error_info.go"
CALLERS = {
    "minutes +detail": (ROOT / "internal/shortcut/smart/minutes_detail.go", 1),
    "chat +chat-messages": (ROOT / "internal/shortcut/smart/chat_messages.go", 1),
    "chat +thread-replies": (ROOT / "internal/shortcut/smart/thread_replies.go", 1),
    "chat +my-groups candidate": (ROOT / "internal/shortcut/smart/my_groups.go", 1),
    "chat +at-me": (ROOT / "internal/shortcut/smart/at_me.go", 1),
    "chat +search-msg read/enrichment": (ROOT / "internal/shortcut/smart/search_msg.go", 2),
    "todo aggregate reads": (ROOT / "internal/shortcut/smart/todo_shared.go", 1),
    "chat +chat-search / +chat-list-all": (ROOT / "internal/shortcut/chat/chat_group.go", 2),
    "chat +conversation-list": (ROOT / "internal/shortcut/chat/chat_conversation.go", 1),
    "chat +flag-list": (ROOT / "internal/shortcut/chat/lark_alignment.go", 1),
}
RESOURCE_CALLERS = {
    "chat +chat-messages resource download": ROOT / "internal/shortcut/smart/chat_messages.go",
    "chat +thread-replies resource download": ROOT / "internal/shortcut/smart/thread_replies.go",
    "chat +at-me resource download": ROOT / "internal/shortcut/smart/at_me.go",
    "chat +search-msg resource download": ROOT / "internal/shortcut/smart/search_msg.go",
}
TEST_PATTERN = r"TestPreserveTypedErrorInfo|TestMinutesDetailPreservesTyped|TestChatMessagesUnifiedPaginationOutcomes|TestChatMessagesPostProcessingFailuresPreserveTypedRecoveryFacts|TestSearchMsgUnifiedPaginationOutcomes|TestCompositeReadFailuresPreserveTypedRecoveryFacts|TestChatCompositeReadFailuresPreserveTypedRecoveryFacts|TestCrossPlatformCoverageMessageResourceFailuresPreserveTypedRecoveryFacts"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output path")
    args = parser.parse_args()

    helper = HELPER.read_text(encoding="utf-8")
    helper_keeps_facts = all(
        token in helper
        for token in (
            "func PreserveTypedErrorInfo",
            "typed.Category",
            "typed.StableSubtype",
            "typed.RetryableSet",
            "apperrors.LookupSubtype",
            "typed.Actions",
            "mergeErrorDetails",
        )
    )
    callers = {
        label: path.read_text(encoding="utf-8").count("shortcut.PreserveTypedErrorInfo(info, err)") >= expected
        for label, (path, expected) in CALLERS.items()
    }
    resource_callers = {
        label: "DownloadMessageResourcesWithFailureInfo" in path.read_text(encoding="utf-8")
        for label, path in RESOURCE_CALLERS.items()
    }
    chat_messages_source = (ROOT / "internal/shortcut/smart/chat_messages.go").read_text(encoding="utf-8")
    post_processing_uses_shared_projection = all(
        token in chat_messages_source
        for token in (
            "shortcut.PreserveTypedErrorInfo(info, err)",
            "shortcut.PreserveTypedErrorInfo(info, filter.err)",
        )
    )

    environment = os.environ.copy()
    environment.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    result = subprocess.run(
        [
            "go",
            "test",
            "-count=1",
            "./internal/shortcut",
            "./internal/shortcut/smart",
            "./internal/shortcut/chat",
            "-run",
            TEST_PATTERN,
            "-v",
        ],
        cwd=ROOT,
        env=environment,
        text=True,
        capture_output=True,
    )
    passed = (
        helper_keeps_facts
        and all(callers.values())
        and all(resource_callers.values())
        and post_processing_uses_shared_projection
        and result.returncode == 0
    )
    transcript = (result.stdout + result.stderr).strip()
    if len(transcript) > 8000:
        transcript = transcript[-8000:]
    caller_lines = "\n".join(
        f"- `{label}` uses the shared typed-error projection: **{'yes' if active else 'no'}**"
        for label, active in callers.items()
    )
    resource_caller_lines = "\n".join(
        f"- `{label}` records per-resource typed failures: **{'yes' if active else 'no'}**"
        for label, active in resource_callers.items()
    )

    report = f"""# 复合读取类型化错误保真 — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本扫描由 Agent 在当前工作树运行。它结合源码关系与内存 Go 测试生成 Markdown 证据；不是 CI / policy gate，不保存服务端响应或 JSON fixture。

## Result: {"PASS" if passed else "REVIEW"}

- 共享投影器保留 category / subtype / hint / actions / retry safety 并合并上下文：**{'yes' if helper_keeps_facts else 'no'}**
{caller_lines}
{resource_caller_lines}
- `chat +chat-messages` export and sender-resolution failures use the shared projection: **{'yes' if post_processing_uses_shared_projection else 'no'}**
- 焦点测试：`{TEST_PATTERN}`
- 测试退出码：`{result.returncode}`

## Required behavior

1. 复合读取可以添加命令自己的失败页、artifact 或恢复上下文，但不得把下游 `auth`、`validation`、`projection` 等 typed error 改写为笼统的 `api + retryable:true`。
2. 明确的 `retryable:false` 必须保留；仅没有分类的幂等读取错误才能采用读路径的默认重试建议。
   已登记 subtype 必须服从 registry 的 retry policy；`RetryNever` 不得继承外层读取的 `retryable:true`。
3. 聚合层与下游错误的 details 若同名，两个事实必须同时保留，不能静默覆盖。
4. 富化等批量后处理必须按失败批次保留独立 typed error，不能把多个不同原因压成一个自由字符串。
5. 资源下载必须保持 legacy `failures[].error` 字符串 wire，同时给统一结果按失败资源提供独立 typed error；权限、参数、投影和传输错误不得合并成一个通用 API 失败。
6. 发送者解析和本地导出等读后处理失败也必须保留下游 actions、retry policy 与 details，不能维护第二套不完整的字段拷贝逻辑。
7. 此扫描不证明真实服务端读取成功、权限正确或资源终态；只证明本地错误投影契约。

## Focused test transcript

```text
{transcript or "(no output)"}
```
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
