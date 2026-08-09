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
        "> 本探针只读取当前用户最近访问 Base 的一页，并在内存中取一个返回名称做搜索对拍；不保存 Base 名称、ID、查询词或原始 JSON，不接入 CI。",
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


def call(dws: str, command: str, extra: list[str]) -> tuple[int, Any | None, bool]:
    completed = subprocess.run(
        [dws, "aitable", command, *extra, "--format", "json"],
        text=True,
        capture_output=True,
        check=False,
    )
    try:
        payload = json.loads(completed.stdout)
    except json.JSONDecodeError:
        payload = None
    return completed.returncode, payload, not completed.stderr.strip()


def validate(payload: Any, rc: int, stderr_empty: bool, source: str) -> tuple[bool, dict[str, Any] | None, list[str]]:
    if rc != 0 or not isinstance(payload, dict):
        return False, None, [f"{source} 未返回可解析成功 JSON（exit code={rc}）。"]
    data = payload.get("data")
    meta = payload.get("meta")
    if not isinstance(data, dict) or not isinstance(meta, dict):
        return False, None, [f"{source} 缺少统一 data/meta 对象。"]
    bases = data.get("bases")
    count = data.get("count")
    rows_ok = isinstance(bases, list) and all(
        isinstance(row, dict)
        and isinstance(row.get("baseId"), str)
        and bool(row["baseId"].strip())
        for row in bases
    )
    known = data.get("paginationKnown")
    paging_ok = isinstance(known, bool)
    if known is True:
        pagination = meta.get("pagination")
        paging_ok = (
            isinstance(pagination, dict)
            and isinstance(pagination.get("endpoint_exhausted"), bool)
            and (
                pagination["endpoint_exhausted"] is True
                or isinstance(pagination.get("next_token"), str) and bool(pagination["next_token"].strip())
            )
        )
    elif known is False:
        paging_ok = "pagination" not in meta
    duplicate_paging = any(key in data for key in ("complete", "hasMore", "endpointExhausted", "nextCursor"))
    passed = (
        payload.get("ok") is True
        and payload.get("outcome") == "success"
        and "contract_version" not in payload
        and isinstance(bases, list)
        and isinstance(count, int)
        and count == len(bases)
        and rows_ok
        and data.get("sourceKind") == source
        and data.get("authoritativeInventory") is False
        and data.get("inventoryCoverageKnown") is False
        and (source != "name_search_index" or data.get("indexCoverageKnown") is False)
        and meta.get("count") == count
        and paging_ok
        and not duplicate_paging
        and stderr_empty
    )
    facts = [
        f"`{source}` 已返回统一 success，count={len(bases) if isinstance(bases, list) else 'unknown'}，稳定 baseId={str(rows_ok).lower()}。",
        f"`{source}` 保留非权威目录/覆盖未知边界：{str(data.get('authoritativeInventory') is False and data.get('inventoryCoverageKnown') is False).lower()}。",
        f"`{source}` 分页已知性为 {known!r}，只在 `meta.pagination` 表达续页事实：{str(paging_ok and not duplicate_paging).lower()}。",
        f"`{source}` stderr 为空：{str(stderr_empty).lower()}。",
    ]
    return passed, data, facts


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws", help="dws executable to query (default: dws)")
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    passed = False
    try:
        list_rc, list_payload, list_stderr_empty = call(args.dws, "+base-list", ["--limit", "1"])
        list_ok, list_data, list_facts = validate(list_payload, list_rc, list_stderr_empty, "recently_accessed")
        rows = list_data.get("bases") if isinstance(list_data, dict) else None
        query = rows[0].get("baseName") if isinstance(rows, list) and rows and isinstance(rows[0], dict) else None
        if isinstance(query, str) and query.strip():
            search_rc, search_payload, search_stderr_empty = call(args.dws, "+base-search", ["--query", query])
            search_ok, _, search_facts = validate(search_payload, search_rc, search_stderr_empty, "name_search_index")
        else:
            search_ok = False
            search_facts = ["最近访问结果未提供可用名称，未执行脱敏搜索对拍。"]
        passed = list_ok and search_ok
        facts = [*list_facts, *search_facts]
        boundary = (
            "本次只验证一组真实正常列表/搜索响应，不证明最近访问列表等于所有可访问 Base，"
            "也不验证死条目、搜索召回率或服务端索引健康。分页矛盾/缺 continuation 的 fail-closed 行为由专项回归覆盖；"
            "搜索零命中仍只能表示当前索引返回为空，不能扩大成业务上不存在。"
        )
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
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
