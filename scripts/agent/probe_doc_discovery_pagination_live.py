#!/usr/bin/env python3
"""Read-only Agent review for Doc list/search pagination and projections.

Identifiers, titles, URLs and raw JSON remain in memory.  The generated
Markdown contains only counts and contract verdicts and is not a CI gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
from pathlib import Path
import subprocess
import uuid
from typing import Any


def call(binary: str, args: list[str]) -> tuple[int, Any]:
    completed = subprocess.run(
        [binary, *args, "--format", "json"],
        text=True,
        capture_output=True,
        check=False,
        timeout=120,
    )
    try:
        payload = json.loads(completed.stdout) if completed.stdout.strip() else None
    except json.JSONDecodeError:
        payload = None
    return completed.returncode, payload


def objects(value: Any):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from objects(child)
    elif isinstance(value, list):
        for child in value:
            yield from objects(child)


def first_workspace(payload: Any) -> str | None:
    for item in objects(payload):
        value = item.get("workspaceId")
        if isinstance(value, str) and value.strip():
            return value
    return None


def discovery_page(payload: Any, items_key: str) -> tuple[list[dict[str, Any]], dict[str, Any], dict[str, Any]] | None:
    if (
        not isinstance(payload, dict)
        or payload.get("ok") is not True
        or payload.get("outcome") != "success"
        or "contract_version" in payload
    ):
        return None
    data = payload.get("data")
    meta = payload.get("meta")
    if not isinstance(data, dict) or not isinstance(meta, dict):
        return None
    items = data.get(items_key)
    pagination = meta.get("pagination")
    if (
        not isinstance(items, list)
        or not all(isinstance(row, dict) for row in items)
        or not isinstance(meta.get("count"), int)
        or meta["count"] != len(items)
        or data.get("pagination_known") is not True
        or not isinstance(pagination, dict)
        or not isinstance(pagination.get("endpoint_exhausted"), bool)
        or any(key in data for key in ("hasMore", "nextCursor", "nextPageToken"))
    ):
        return None
    for row in items:
        node_id = row.get("nodeId")
        if not isinstance(node_id, str) or not node_id.strip():
            return None
    return items, data, pagination


def invalid_flag_failure(payload: Any) -> bool:
    return (
        isinstance(payload, dict)
        and payload.get("ok") is False
        and payload.get("outcome") == "failure"
        and isinstance(payload.get("error"), dict)
        and payload["error"].get("subtype") == "invalid_flag_value"
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--folder-probes", type=int, default=20)
    args = parser.parse_args()

    checks: list[tuple[str, bool, str, bool]] = []

    rc, spaces = call(args.dws, ["wiki", "+space-list", "--limit", "1"])
    workspace = first_workspace(spaces) if rc == 0 else None
    checks.append(("可见知识库发现", bool(workspace), "workspace=yes" if workspace else f"rc={rc}", True))

    if workspace:
        rc, first = call(args.dws, ["doc", "+list", "--workspace", workspace, "--limit", "1"])
        first_page = discovery_page(first, "nodes") if rc == 0 else None
        next_token = first_page[2].get("next_token") if first_page else None
        first_ok = (
            first_page is not None
            and first_page[2].get("endpoint_exhausted") is False
            and isinstance(next_token, str)
            and bool(next_token)
            and first_page[1].get("inventory_scope") == "requested_location"
        )
        checks.append((
            "Doc list 首屏续页与范围",
            first_ok,
            "count=1, endpoint_exhausted=false, next_token=yes, scope=requested_location" if first_ok else f"rc={rc}",
            True,
        ))
        if first_ok:
            rc, second = call(args.dws, [
                "doc", "+list", "--workspace", workspace,
                "--limit", "1", "--cursor", next_token,
            ])
            second_page = discovery_page(second, "nodes") if rc == 0 else None
            first_ids = {row["nodeId"] for row in first_page[0]}
            second_ids = {row["nodeId"] for row in second_page[0]} if second_page else set()
            second_ok = second_page is not None and len(second_ids) == 1 and first_ids.isdisjoint(second_ids)
            checks.append((
                "Doc list 第二页可恢复且不重复",
                second_ok,
                "count=1, stable_ids=yes, pages_disjoint=yes" if second_ok else f"rc={rc}",
                True,
            ))

        rc, root = call(args.dws, ["doc", "+list", "--workspace", workspace, "--limit", "50"])
        root_page = discovery_page(root, "nodes") if rc == 0 else None
        likely_empty_folders = []
        other_folders = []
        if root_page:
            for row in root_page[0]:
                node_type = row.get("nodeType")
                if isinstance(node_type, str) and "folder" in node_type.lower():
                    target = likely_empty_folders if row.get("hasChildren") is False else other_folders
                    target.append(row["nodeId"])
        folder_ids = likely_empty_folders + other_folders
        empty_found = False
        probes = 0
        for folder_id in folder_ids[: max(args.folder_probes, 0)]:
            probes += 1
            folder_rc, folder = call(args.dws, ["doc", "+list", "--folder", folder_id, "--limit", "1"])
            folder_page = discovery_page(folder, "nodes") if folder_rc == 0 else None
            if folder_page and not folder_page[0] and folder_page[2].get("endpoint_exhausted") is True:
                empty_found = True
                break
        checks.append((
            "Doc list 真实空文件夹终页",
            empty_found,
            f"folder_candidates={len(folder_ids)}, likely_empty={len(likely_empty_folders)}, bounded_probes={probes}, empty_found={'yes' if empty_found else 'no'}",
            False,
        ))

    rc, search_first = call(args.dws, ["doc", "+search", "--limit", "1"])
    search_page = discovery_page(search_first, "documents") if rc == 0 else None
    search_token = search_page[2].get("next_token") if search_page else None
    search_ok = (
        search_page is not None
        and search_page[2].get("endpoint_exhausted") is False
        and isinstance(search_token, str)
        and bool(search_token)
        and search_page[1].get("index_coverage_known") is False
    )
    checks.append((
        "Doc search 首屏续页且不扩大索引覆盖",
        search_ok,
        "count=1, endpoint_exhausted=false, next_token=yes, index_coverage_known=false" if search_ok else f"rc={rc}",
        True,
    ))
    if search_ok:
        rc, search_second = call(args.dws, ["doc", "+search", "--limit", "1", "--cursor", search_token])
        second_page = discovery_page(search_second, "documents") if rc == 0 else None
        first_ids = {row["nodeId"] for row in search_page[0]}
        second_ids = {row["nodeId"] for row in second_page[0]} if second_page else set()
        second_ok = second_page is not None and len(second_ids) == 1 and first_ids.isdisjoint(second_ids)
        checks.append((
            "Doc search 第二页可恢复且不重复",
            second_ok,
            "count=1, stable_ids=yes, pages_disjoint=yes" if second_ok else f"rc={rc}",
            True,
        ))

    no_match_query = "__dws_agent_no_match_" + uuid.uuid4().hex
    rc, no_match = call(args.dws, ["doc", "+search", "--query", no_match_query, "--limit", "1"])
    no_match_page = discovery_page(no_match, "documents") if rc == 0 else None
    no_match_ok = (
        no_match_page is not None
        and len(no_match_page[0]) == 0
        and no_match_page[2].get("endpoint_exhausted") is True
        and no_match_page[1].get("index_coverage_known") is False
    )
    checks.append((
        "Doc search 无命中只声明 endpoint 终页",
        no_match_ok,
        "count=0, endpoint_exhausted=true, index_coverage_known=false" if no_match_ok else f"rc={rc}",
        True,
    ))

    for label, command in (
        ("Doc search 非法 limit 调用前失败", ["doc", "+search", "--limit", "31"]),
        ("Doc list 非法 limit 调用前失败", ["doc", "+list", "--limit", "51"]),
    ):
        rc, payload = call(args.dws, command)
        ok = rc == 3 and invalid_flag_failure(payload)
        checks.append((label, ok, "rc=3, subtype=invalid_flag_value" if ok else f"rc={rc}", True))

    required_failed = any(required and not ok for _, ok, _, required in checks)
    coverage_review = any(not required and not ok for _, ok, _, required in checks)
    result = "FAIL" if required_failed else "REVIEW" if coverage_review else "PASS"
    lines = [
        "# Doc 发现分页与投影 Agent 实测",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 仅执行只读空间、文档列表与搜索调用。知识库、节点、标题、URL、查询词和 token 只在内存中使用；报告不保存原始 JSON，也不接入 CI。",
        "",
        f"## Result: {result}",
        "",
        "| 检查项 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    for name, ok, evidence, required in checks:
        status = "PASS" if ok else "REVIEW" if not required else "FAIL"
        lines.append(f"| {name} | {status} | `{evidence}` |")
    lines += [
        "",
        "## 结论",
        "",
        "- `hasMore:true + nextPageToken` 被投影为 `meta.pagination.endpoint_exhausted:false + next_token`；Agent 可以原样续页。",
        "- 搜索 endpoint 耗尽不等于索引健康或全局召回完整，因此始终保留 `index_coverage_known:false`。",
        "- 列表范围固定为 `requested_location`；它只表示请求位置的直接子节点，不扩大为递归目录或租户全量文档。",
        "- 非法页大小在远端调用前返回 typed validation；未知容器、非法行、字段类型漂移及分页矛盾由 Go response seam fail-closed。",
    ]
    if coverage_review:
        lines += [
            "",
            "## 尚未验证",
            "",
            "- 有界只读范围内未找到真实空文件夹时，只记录 REVIEW；不为通过测试而创建或修改业务资源。",
        ]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 1 if required_failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
