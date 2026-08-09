#!/usr/bin/env python3
"""Read-only Agent probe for AITable Base discovery output.

The probe records only aggregate result facts. Base names, IDs, and raw JSON
remain in memory; this is manual Agent evidence rather than a CI gate.
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
        "# AITable Base discovery Agent 实测",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本探针只读取当前用户最近访问 Base 的一页；不保存 Base 名称、ID 或原始 JSON，不接入 CI。",
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
        [dws, "aitable", "+base-list", "--limit", "1", "--format", "json"],
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
            "保留读取错误；不要将失败或不可解析输出解释为没有 Base。",
        )
    # Base discovery is deliberately in dual_validate. Public bytes must stay
    # historical while the framework validates the richer result in-process.
    bases = payload.get("bases")
    count = payload.get("count")
    rows_ok = isinstance(bases, list) and all(
        isinstance(row, dict)
        and isinstance(row.get("baseId"), str)
        and bool(row["baseId"].strip())
        for row in bases
    )
    known = payload.get("paginationKnown")
    paging_ok = isinstance(known, bool)
    if known is True:
        more = payload.get("hasMore")
        exhausted = payload.get("endpointExhausted")
        token = payload.get("nextCursor")
        paging_ok = (
            isinstance(more, bool)
            and isinstance(exhausted, bool)
            and exhausted is (not more)
            and (not more or (token is not None and str(token).strip() not in {"", "0"}))
            and (more or token is None or str(token).strip() in {"", "0"})
        )
    passed = (
        isinstance(bases, list)
        and isinstance(count, int)
        and count == len(bases)
        and rows_ok
        and payload.get("authoritativeInventory") is False
        and payload.get("inventoryCoverageKnown") is False
        and paging_ok
        and stderr_empty
        and "ok" not in payload
        and "outcome" not in payload
    )
    facts = [
        "当前命令处于 `dual_validate`：外部仍为 historical Base payload；统一结果只在进程内校验，未对 Agent 激活。",
        f"投影 Base 数与 `count` 一致：{len(bases) if isinstance(bases, list) else 'unknown'}。",
        f"所有投影 Base 均有稳定 `baseId`：{str(rows_ok).lower()}。",
        f"非权威目录边界：`authoritativeInventory:false`、`inventoryCoverageKnown:false`。",
        f"分页事实已知性为 {known!r}，已知时 hasMore/continuation 自洽：{str(paging_ok).lower()}。",
        f"stderr 为空：{str(stderr_empty).lower()}。",
    ]
    boundary = (
        "本次只验证一个真实正常单页，不证明最近访问列表等于所有可访问 Base，"
        "也不验证死条目、检索召回或服务端索引健康。分页矛盾/缺 continuation 的 fail-closed 行为由专项单元回归覆盖；"
        "在明确晋级 unified_active 前，Agent 继续按现有 payload 解析。"
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
        text = render("PASS" if passed else "REVIEW：Base 发现投影不满足安全条件", facts, boundary)
    except OSError as exc:
        text = render(
            "REVIEW：CLI 未启动",
            [f"启动失败：{type(exc).__name__}。"],
            "修复本地 CLI 或认证后重跑；不作 Base 目录结论。",
        )
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(text, encoding="utf-8")
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
