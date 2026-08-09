#!/usr/bin/env python3
"""Read-only live review for Wiki node-list continuation and empty folders.

All workspace/node identifiers and titles stay in memory.  The Markdown output
contains only counts and contract verdicts; no response JSON is persisted and
the probe is deliberately not wired into CI.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
from pathlib import Path
import subprocess
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


def first_workspace(payload: Any) -> str | None:
    if not isinstance(payload, dict):
        return None
    data = payload.get("data") if isinstance(payload.get("data"), dict) else payload
    spaces = data.get("spaces") if isinstance(data, dict) else None
    if not isinstance(spaces, list):
        return None
    for row in spaces:
        value = row.get("workspaceId") if isinstance(row, dict) else None
        if isinstance(value, str) and value.strip():
            return value
    return None


def node_page(payload: Any) -> tuple[list[dict[str, Any]], dict[str, Any]] | None:
    if not isinstance(payload, dict) or payload.get("ok") is not True or payload.get("outcome") != "success":
        return None
    data = payload.get("data")
    meta = payload.get("meta")
    if not isinstance(data, dict) or not isinstance(meta, dict):
        return None
    nodes = data.get("nodes")
    pagination = meta.get("pagination")
    if (
        not isinstance(nodes, list)
        or not all(isinstance(row, dict) for row in nodes)
        or not isinstance(meta.get("count"), int)
        or meta["count"] != len(nodes)
        or not isinstance(pagination, dict)
        or not isinstance(pagination.get("endpoint_exhausted"), bool)
    ):
        return None
    for row in nodes:
        node_id = row.get("nodeId")
        if not isinstance(node_id, str) or not node_id.strip():
            return None
    return nodes, pagination


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws")
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--folder-probes", type=int, default=20)
    args = parser.parse_args()

    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    rc, spaces = call(args.dws, ["wiki", "+space-list", "--limit", "1"])
    workspace = first_workspace(spaces) if rc == 0 else None
    checks.append(("可见知识库发现", bool(workspace), "workspace=yes" if workspace else f"rc={rc}"))
    if not workspace:
        findings.append("no readable workspace was available")
    else:
        rc, first = call(args.dws, ["wiki", "+node-list", "--workspace", workspace, "--limit", "1"])
        first_page = node_page(first) if rc == 0 else None
        continuation = None
        if first_page:
            _, pagination = first_page
            continuation = pagination.get("next_token")
        first_ok = (
            first_page is not None
            and first_page[1].get("endpoint_exhausted") is False
            and isinstance(continuation, str)
            and bool(continuation)
        )
        checks.append((
            "真实首屏续页证据",
            first_ok,
            "count=1, endpoint_exhausted=false, next_token=yes" if first_ok else f"rc={rc}",
        ))
        if not first_ok:
            findings.append("first page did not expose a usable continuation token")
        else:
            rc, second = call(args.dws, [
                "wiki", "+node-list", "--workspace", workspace,
                "--limit", "1", "--cursor", continuation,
            ])
            second_page = node_page(second) if rc == 0 else None
            first_ids = {row["nodeId"] for row in first_page[0]}
            second_ids = {row["nodeId"] for row in second_page[0]} if second_page else set()
            second_ok = second_page is not None and len(second_ids) == 1 and first_ids.isdisjoint(second_ids)
            checks.append((
                "真实第二页可恢复且不重复首屏",
                second_ok,
                "count=1, stable_ids=yes, pages_disjoint=yes" if second_ok else f"rc={rc}",
            ))
            if not second_ok:
                findings.append("continuation request did not produce a distinct valid second page")

        rc, root = call(args.dws, ["wiki", "+node-list", "--workspace", workspace, "--limit", "50"])
        root_page = node_page(root) if rc == 0 else None
        folder_ids = []
        if root_page:
            for row in root_page[0]:
                node_type = row.get("type")
                if isinstance(node_type, str) and "folder" in node_type.lower():
                    folder_ids.append(row["nodeId"])
        empty_found = False
        probes = 0
        for folder_id in folder_ids[: max(args.folder_probes, 0)]:
            probes += 1
            folder_rc, folder = call(args.dws, [
                "wiki", "+node-list", "--workspace", workspace,
                "--folder", folder_id, "--limit", "1",
            ])
            folder_page = node_page(folder) if folder_rc == 0 else None
            if folder_page and not folder_page[0] and folder_page[1].get("endpoint_exhausted") is True:
                empty_found = True
                break
        checks.append((
            "真实空文件夹终页",
            empty_found,
            f"folder_candidates={len(folder_ids)}, bounded_probes={probes}, empty_found={'yes' if empty_found else 'no'}",
        ))
        if not empty_found:
            findings.append("bounded live scope did not contain a readable empty folder")

    lines = [
        "# Wiki node-list 分页边界 Agent 实测",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 仅执行只读空间/节点列表调用。知识库、节点和标题标识只在内存中使用；报告不保存原始 JSON，也不接入 CI。",
        "",
        f"## Result: {'PASS' if not findings else 'REVIEW'}",
        "",
        "| 检查项 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {name} | {'PASS' if ok else 'REVIEW'} | `{evidence}` |" for name, ok, evidence in checks)
    lines += [
        "",
        "## 结论",
        "",
        "- `hasMore:true + nextPageToken` 必须投影为 `endpoint_exhausted:false + next_token`；Agent 使用返回 token 恢复下一页，不猜测游标字段。",
        "- 每页任一行缺稳定 string `nodeId` 时必须整体 fail-closed，不能静默丢行后仍返回 success。",
        "- 空目录只有在真实空数组且 endpoint 明确耗尽时成立；没有空文件夹 fixture 只记为未验证，不构造资源或扩大结论。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if not findings else 1


if __name__ == "__main__":
    raise SystemExit(main())
