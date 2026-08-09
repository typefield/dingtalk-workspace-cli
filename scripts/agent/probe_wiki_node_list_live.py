#!/usr/bin/env python3
"""Read-only Agent probe for the Wiki shortcut node-list normal path.

It discovers one currently visible workspace, reads only its root node page and
records aggregate result facts.  No workspace ID, node ID, node title or JSON
payload is persisted.  It is deliberately a manual Agent probe, not CI.
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
        "# Wiki node-list 统一结果 Agent 实测",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本探针先只读发现一个可见知识库，再只读其根目录；只记录计数和结果契约，不保存 workspaceId、nodeId、标题或运行 JSON，也不接入 CI。",
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


def call(command: list[str]) -> tuple[int, Any | None]:
    completed = subprocess.run(command, text=True, capture_output=True, check=False)
    if completed.returncode != 0:
        return completed.returncode, None
    try:
        return 0, json.loads(completed.stdout)
    except json.JSONDecodeError:
        return 0, None


def first_workspace_id(payload: Any) -> str | None:
    if not isinstance(payload, dict):
        return None
    # Space discovery remains legacy on some builds, while other builds put
    # the same list inside unified data.  Both are discovery-only inputs.
    spaces = payload.get("spaces")
    if not isinstance(spaces, list) and isinstance(payload.get("data"), dict):
        spaces = payload["data"].get("spaces")
    if not isinstance(spaces, list):
        return None
    for space in spaces:
        if isinstance(space, dict) and isinstance(space.get("workspaceId"), str) and space["workspaceId"].strip():
            return space["workspaceId"]
    return None


def validate_node_list(payload: Any) -> tuple[bool, list[str], str]:
    if not isinstance(payload, dict):
        return False, ["node-list 顶层不是 JSON 对象。"], "保留失败事实，不要把不可解析输出当作空目录。"
    data = payload.get("data")
    meta = payload.get("meta")
    if payload.get("ok") is not True or payload.get("outcome") != "success":
        return False, ["node-list 未返回 `ok:true` / `outcome:success`。"], "保留服务端错误；不要把失败当作目录为空。"
    if not isinstance(data, dict) or not isinstance(meta, dict):
        return False, ["统一信封缺少对象类型的 `data` 或 `meta`。"], "修复统一投影后重新实测。"
    nodes = data.get("nodes")
    count = meta.get("count")
    pagination = meta.get("pagination")
    if not isinstance(nodes, list) or not isinstance(count, int) or count != len(nodes):
        return False, ["`data.nodes` 与 `meta.count` 不构成可核对的列表事实。"], "不得把该响应表述为可信目录。"
    if data.get("paginationKnown") is not True or not isinstance(pagination, dict):
        return False, ["正常响应缺少可信分页事实。"], "保持未知分页而非猜测 endpoint 终态。"
    exhausted = pagination.get("endpoint_exhausted")
    has_more = data.get("hasMore")
    if not isinstance(exhausted, bool) or not isinstance(has_more, bool) or exhausted == has_more:
        return False, ["`endpoint_exhausted` 与 `hasMore` 不一致。"], "修复分页投影；二者不能同时表示还可续页或已耗尽。"
    if not all(isinstance(row, dict) and isinstance(row.get("nodeId"), str) and row["nodeId"].strip() for row in nodes):
        return False, ["存在无法继续操作的节点：缺少非空 string nodeId。"], "保持 fail-closed，不能把展示行交给 Agent 继续操作。"
    return (
        True,
        [
            "返回统一 `ok:true` / `outcome:success`，且 `data`/`meta` 均为对象。",
            f"根目录节点数与 `meta.count` 一致：{count}；所有返回节点均有非空 string nodeId。",
            f"分页事实自洽：`hasMore:{str(has_more).lower()}`，`endpoint_exhausted:{str(exhausted).lower()}`。",
        ],
        "本次仅验证一个可见知识库的正常根目录响应。仍需真实复验空目录、续页、嵌套分页和服务端异常形状；不能把这一页扩大为所有知识库目录均已验证。",
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws", help="dws executable to query (default: dws)")
    parser.add_argument("--output", type=Path, help="write Markdown report; default stdout")
    args = parser.parse_args()

    try:
        rc, spaces = call([args.dws, "wiki", "+space-list", "--limit", "1", "--format", "json"])
        workspace_id = first_workspace_id(spaces) if rc == 0 else None
        if workspace_id is None:
            text = render(
                "REVIEW：未发现可安全读取的知识库",
                ["空间发现未成功或未返回可用 workspaceId。"],
                "修复认证、权限或空间发现投影后再重跑；不要构造 workspaceId 调用 node-list。",
            )
        else:
            rc, nodes = call([args.dws, "wiki", "+node-list", "--workspace", workspace_id, "--limit", "50", "--format", "json"])
            if rc != 0:
                text = render("REVIEW：node-list 读取失败", [f"node-list exit code 为 {rc}。"], "保留服务端错误线索；不要将失败当作根目录为空。")
            else:
                passed, facts, boundary = validate_node_list(nodes)
                text = render("PASS" if passed else "REVIEW：node-list 投影不满足契约", facts, boundary)
    except OSError as exc:
        text = render("REVIEW：CLI 未启动", [f"启动失败：{type(exc).__name__}。"], "修复本地 CLI/认证后重跑；不作目录结论。")

    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(text, encoding="utf-8")
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
