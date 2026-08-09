#!/usr/bin/env python3
"""Read-only Agent probe for both active joined-group directory commands.

The probe keeps group names and conversation IDs in memory. Its Markdown
evidence records only aggregate counts and output-contract facts; it never
persists runtime JSON and is not a CI gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
from pathlib import Path
import subprocess
import sys
from typing import Any


COMMANDS = ("+my-groups", "+chat-list-all")


def render(status: str, facts: list[str], boundary: str) -> str:
    lines = [
        "# Chat 群目录统一结果 Agent 实测",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本探针只读当前用户已加入群的一页；群名、群 ID 与原始 JSON 都不落盘。它是 Agent 审阅证据，不接入 CI。",
        "",
        "## 结果",
        "",
        f"**{status}**",
        "",
        "## 可观测事实",
        "",
    ]
    lines.extend(f"- {fact}" for fact in facts)
    lines.extend(["", "## 边界", "", boundary, ""])
    return "\n".join(lines)


def call(dws: str, command: str) -> tuple[int, Any | None, bool]:
    completed = subprocess.run(
        [dws, "chat", command, "--limit", "1", "--format", "json"],
        text=True,
        capture_output=True,
        check=False,
    )
    try:
        payload = json.loads(completed.stdout)
    except json.JSONDecodeError:
        payload = None
    return completed.returncode, payload, not completed.stderr.strip()


def validate(command: str, payload: Any, rc: int, stderr_empty: bool) -> tuple[bool, list[str]]:
    if rc != 0 or not isinstance(payload, dict):
        return False, [f"`chat {command}` 未返回可解析成功 JSON（exit code={rc}）。"]
    data = payload.get("data")
    meta = payload.get("meta")
    groups = data.get("groups") if isinstance(data, dict) else None
    count = meta.get("count") if isinstance(meta, dict) else None
    pagination = meta.get("pagination") if isinstance(meta, dict) else None
    rows_ok = isinstance(groups, list) and all(
        isinstance(row, dict)
        and isinstance(row.get("openConversationId"), str)
        and bool(row["openConversationId"].strip())
        for row in groups
    )
    continuation_ok = (
        isinstance(pagination, dict)
        and pagination.get("endpoint_exhausted") is False
        and isinstance(pagination.get("next_token"), str)
        and bool(pagination["next_token"].strip())
    )
    passed = (
        payload.get("ok") is True
        and payload.get("outcome") == "success"
        and "contract_version" not in payload
        and isinstance(count, int)
        and count == len(groups or [])
        and rows_ok
        and continuation_ok
        and stderr_empty
        and isinstance(data, dict)
        and all(key not in data for key in ("complete", "hasMore", "nextCursor", "stopReason", "pagesFetched"))
    )
    facts = [
        f"`chat {command}` 直接返回统一 `ok/outcome/data/meta`：{str(payload.get('ok') is True and payload.get('outcome') == 'success').lower()}。",
        f"投影数量与 `meta.count` 一致：{len(groups) if isinstance(groups, list) else 'unknown'}。",
        f"所有投影群均有稳定 `openConversationId`：{str(rows_ok).lower()}。",
        f"真实单页明确可续：`endpoint_exhausted:false + next_token`：{str(continuation_ok).lower()}。",
        f"未暴露协议版本或 legacy 分页字段，stderr 为空：{str('contract_version' not in payload and stderr_empty).lower()}。",
    ]
    return passed, facts


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws", help="dws executable to query (default: dws)")
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    all_passed = True
    facts: list[str] = []
    try:
        for command in COMMANDS:
            rc, payload, stderr_empty = call(args.dws, command)
            passed, command_facts = validate(command, payload, rc, stderr_empty)
            all_passed = all_passed and passed
            facts.extend(command_facts)
        boundary = (
            "本次真实账号只验证两个入口的正常可续单页；未证明空群、完整跨页、游标冲突、网关异常形状或租户目录覆盖范围。"
            "这些边界继续由 fixture 和后续隔离账号复验覆盖；endpoint exhausted 只表示该接口分页耗尽。"
        )
        text = render("PASS" if all_passed else "REVIEW：统一群目录结果不满足安全条件", facts, boundary)
    except OSError as exc:
        all_passed = False
        text = render(
            "REVIEW：CLI 未启动",
            [f"启动失败：{type(exc).__name__}。"],
            "修复本地 CLI 或认证后重跑；不作群目录结论。",
        )
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(text, encoding="utf-8")
    else:
        sys.stdout.write(text)
    return 0 if all_passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
