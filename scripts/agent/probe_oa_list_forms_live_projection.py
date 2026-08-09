#!/usr/bin/env python3
"""Read-only Agent verification for the OA unified list-forms projection.

The upstream OA form directory may return ``totalCount=-1``.  That is an
unknown-total sentinel, not evidence that the current response exhausted the
directory.  This probe verifies the public shortcut keeps that distinction in
its unified result without storing template payloads or JSON fixtures.

It is intentionally a manually run Agent probe, not a CI/policy gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
from pathlib import Path
import subprocess
import sys
from typing import Any


def report(status: str, facts: list[str], next_step: str) -> str:
    lines = [
        "# OA list-forms 统一分页投影 Agent 实测",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本探针只读调用 `oa +list-forms`；只记录结构、计数与分页事实，不保存模板、processCode 或运行 JSON，也不接入 CI。",
        "",
        "## 结果",
        "",
        f"**{status}**",
        "",
        "## 可观测事实",
        "",
    ]
    lines.extend(f"- {fact}" for fact in facts)
    lines.extend(["", "## 边界", "", next_step, ""])
    return "\n".join(lines)


def validate(payload: Any) -> tuple[bool, list[str], str]:
    if not isinstance(payload, dict):
        return False, ["顶层不是对象。"], "保持 fail-closed；先检查 CLI 输出和认证状态。"
    data = payload.get("data")
    meta = payload.get("meta")
    if payload.get("ok") is not True or payload.get("outcome") != "success":
        return False, ["读取未返回 `ok:true` / `outcome:success`。"], "先保留服务端错误或认证事实，不要把失败当空目录。"
    if not isinstance(data, dict) or not isinstance(meta, dict):
        return False, ["统一信封缺少对象类型的 `data` 或 `meta`。"], "修复投影后重新实测。"
    forms = data.get("forms")
    count = meta.get("count")
    pagination_known = data.get("pagination_known")
    if not isinstance(forms, list) or not isinstance(count, int) or count != len(forms):
        return False, ["`data.forms` 与 `meta.count` 不构成可核对的列表事实。"], "不要据此声明发现结果完整。"
    if pagination_known is not False or "pagination" in meta:
        return False, ["服务端总数未知时没有明确保留 `pagination_known:false`。"], "修复为未知分页语义；不得伪造 endpoint exhausted。"
    return (
        True,
        [
            "统一信封为 `ok:true`、`outcome:success`，且 `data`/`meta` 均为对象。",
            f"已投影表单数与 `meta.count` 一致：{count}。",
            "上游未给可信总数或续页事实时，输出 `data.pagination_known:false`，且没有伪造 `meta.pagination`。",
        ],
        "本次只证明当前身份的一条正常只读响应。仍需真实复验空结果、续页、响应形状漂移和权限失败；不能把当前页或空结果扩大为完整审批目录。",
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws", help="dws executable to query (default: dws)")
    parser.add_argument("--limit", type=int, default=100, help="requested forms per page (1..100)")
    parser.add_argument("--output", type=Path, help="write Markdown report; default stdout")
    args = parser.parse_args()
    if not 1 <= args.limit <= 100:
        parser.error("--limit must be within 1..100")

    command = [args.dws, "oa", "+list-forms", "--limit", str(args.limit), "--format", "json"]
    try:
        completed = subprocess.run(command, text=True, capture_output=True, check=False)
    except OSError as exc:
        text = report("REVIEW：CLI 未启动", [f"启动失败：{type(exc).__name__}。"], "修复本地 CLI/认证后重跑；不作目录结论。")
    else:
        if completed.returncode != 0:
            text = report("REVIEW：只读调用失败", [f"`oa +list-forms` exit code 为 {completed.returncode}。"], "保留错误分类和恢复线索后重跑；不把失败当空结果。")
        else:
            try:
                payload = json.loads(completed.stdout)
            except json.JSONDecodeError:
                payload = None
            passed, facts, boundary = validate(payload)
            text = report("PASS" if passed else "REVIEW：统一投影不满足未知分页契约", facts, boundary)

    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(text, encoding="utf-8")
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
