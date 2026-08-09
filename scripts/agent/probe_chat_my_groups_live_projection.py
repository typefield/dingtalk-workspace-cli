#!/usr/bin/env python3
"""Read-only Agent probe for the current user's My Groups projection.

The probe keeps group names and conversation IDs in memory. Its Markdown
evidence records only aggregate counts and output-contract facts, and it is
not a CI gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
from pathlib import Path
import subprocess
import sys
from typing import Any


def render(status: str, facts: list[str], boundary: str) -> str:
    lines = [
        "# Chat my-groups 投影 Agent 实测",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本探针只读取当前用户已加入群的一页；群名、会话 ID 与原始 JSON 都不落盘。它是 Agent 审阅证据，不接入 CI。",
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


def call(dws: str) -> tuple[int, Any | None, bool]:
    completed = subprocess.run(
        [dws, "chat", "+my-groups", "--limit", "1", "--format", "json"],
        text=True,
        capture_output=True,
        check=False,
    )
    try:
        payload = json.loads(completed.stdout)
    except json.JSONDecodeError:
        payload = None
    return completed.returncode, payload, not completed.stderr.strip()


def validate(payload: Any, rc: int, stderr_empty: bool) -> tuple[bool, list[str], str]:
    if rc != 0 or not isinstance(payload, dict):
        return (
            False,
            [f"命令未返回可解析成功 JSON（exit code={rc}）。"],
            "保留服务端或本地错误；不要把读取失败解释为没有群。",
        )
    groups = payload.get("groups")
    count = payload.get("count")
    has_more = payload.get("hasMore")
    next_cursor = payload.get("nextCursor")
    rows_ok = isinstance(groups, list) and all(
        isinstance(row, dict)
        and isinstance(row.get("conversationId"), str)
        and bool(row["conversationId"].strip())
        for row in groups
    )
    continuation_ok = isinstance(has_more, bool) and (
        not has_more or (next_cursor is not None and str(next_cursor).strip() not in {"", "0"})
    )
    passed = (
        isinstance(groups, list)
        and isinstance(count, int)
        and count == len(groups)
        and rows_ok
        and continuation_ok
        and stderr_empty
    )
    facts = [
        "当前命令仍为 legacy JSON（无 `ok/outcome` 信封）；这次扫描不把它误报为 unified active。",
        f"投影群数与 `count` 一致：{len(groups) if isinstance(groups, list) else 'unknown'}。",
        f"所有投影群均有稳定 `conversationId`：{str(rows_ok).lower()}。",
        f"单页续页事实可判定：`hasMore` 为 bool，continuation 自洽：{str(continuation_ok).lower()}。",
        f"stderr 为空：{str(stderr_empty).lower()}。",
    ]
    boundary = (
        "本次仅验证一个真实正常单页；未验证空列表、第二页、游标冲突或网关异常形状。"
        "未知容器、非法行或缺 stable conversationId 的 fail-closed 行为由专项单元回归覆盖。"
    )
    return passed, facts, boundary


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws", help="dws executable to query (default: dws)")
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    try:
        rc, payload, stderr_empty = call(args.dws)
        passed, facts, boundary = validate(payload, rc, stderr_empty)
        text = render("PASS" if passed else "REVIEW：投影不满足安全条件", facts, boundary)
    except OSError as exc:
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
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
