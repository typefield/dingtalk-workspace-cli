#!/usr/bin/env python3
"""Read-only Agent preflight for the unresolved approval-create evaluation item.

This intentionally stops before creating an approval instance.  It checks
whether the current identity can discover a *plausibly isolated* template and
records only aggregate facts in Markdown.  A name match is never authority to
write: a human must still verify the template owner, fields, cleanup route and
business impact before a separately authorised live create/revoke exercise.

The probe is deliberately not a CI gate and does not store any JSON response
fixture, process code, template name, user ID, or form value.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
from pathlib import Path
import re
import subprocess
import sys
from typing import Any


TEST_NAME = re.compile(r"test|测试|sandbox|沙箱|demo|演示", re.IGNORECASE)


def render(*, status: str, facts: list[str], next_step: str) -> str:
    lines = [
        "# Approval 真实提单 Agent 只读预检",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本探针只读取当前身份可见的审批模板；不创建、不撤销审批，不保存 JSON、模板名称、processCode 或表单值，也不接入 CI。",
        "",
        "## 结果",
        "",
        f"**{status}**",
        "",
        "## 可观测事实",
        "",
    ]
    lines.extend(f"- {fact}" for fact in facts)
    lines.extend(["", "## 下一步", "", next_step, ""])
    return "\n".join(lines)


def form_rows(payload: dict[str, Any]) -> tuple[int, int | None, int] | None:
    """Return listed rows, declared total when meaningful, and candidate count."""

    if payload.get("success") is not True:
        return None
    result = payload.get("result")
    if not isinstance(result, dict):
        return None
    rows = result.get("processCodeList")
    total = result.get("totalCount")
    if not isinstance(rows, list):
        return None
    # The OA service uses -1 as an "unknown total" sentinel for some list
    # responses.  It is useful pagination evidence, but it must not be shown
    # as a negative number of templates or treated as an exhausted directory.
    known_total = total if isinstance(total, int) and total >= 0 else None
    candidates = 0
    for row in rows:
        if not isinstance(row, dict):
            continue
        name = row.get("processName")
        directory = row.get("dirName")
        searchable = " ".join(value for value in (name, directory) if isinstance(value, str))
        if TEST_NAME.search(searchable):
            candidates += 1
    return len(rows), known_total, candidates


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws", help="dws executable to query (default: dws)")
    parser.add_argument("--limit", type=int, default=100, help="maximum visible templates to inspect (1..100)")
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    if not 1 <= args.limit <= 100:
        parser.error("--limit must be within 1..100")

    command = [args.dws, "oa", "approval", "list-forms", "--limit", str(args.limit), "--format", "json"]
    try:
        completed = subprocess.run(command, text=True, capture_output=True, check=False)
    except OSError as exc:
        report = render(
            status="REVIEW：未能启动 CLI",
            facts=[f"启动只读模板查询失败：{type(exc).__name__}"],
            next_step="修复本地 CLI/认证后重新运行本探针；不要据此尝试创建审批。",
        )
    else:
        if completed.returncode != 0:
            report = render(
                status="REVIEW：只读模板查询未成功",
                facts=[f"`oa approval list-forms` exit code 为 {completed.returncode}。"],
                next_step="先解决认证、权限或服务端错误，再重新运行本探针；不要以失败输出猜测模板是否存在。",
            )
        else:
            try:
                payload = json.loads(completed.stdout)
            except json.JSONDecodeError:
                payload = None
            observed = form_rows(payload) if isinstance(payload, dict) else None
            if observed is None:
                report = render(
                    status="REVIEW：模板响应形状不可安全判定",
                    facts=["CLI 返回 0，但没有识别到 `success:true` 与 `result.processCodeList[]` 的完整形状。"],
                    next_step="先人工检查公开响应契约或修复投影；不要把未知形状当作“没有模板”或“可以创建”。",
                )
            else:
                listed, total, candidates = observed
                if candidates:
                    status = "REVIEW：发现名称上疑似隔离模板的候选"
                    next_step = (
                        "由模板所有者人工确认候选确为隔离测试模板，并确认可撤销/清理；随后先跑 "
                        "`form-schema`、`forecast-process` 与 `create-instance --dry-run`。仅在得到明确写入授权后，"
                        "才可单独执行一次 create 并记录实例清理证据。"
                    )
                else:
                    status = "REVIEW：没有名称可识别的隔离模板候选"
                    next_step = (
                        "向审批模板所有者申请一个隔离、可清理的测试模板；不要对普通业务模板执行 create-instance。"
                    )
                report = render(
                    status=status,
                    facts=[
                        (
                            f"当前身份可见模板总数：{total}；本次返回并审阅的模板数：{listed}。"
                            if total is not None
                            else f"服务端未提供可信模板总数；本次返回并审阅的模板数：{listed}。"
                        ),
                        f"按名称/目录的测试关键词匹配候选数：{candidates}。关键词只用于缩小人工核验范围，不代表写入授权。",
                    ],
                    next_step=next_step,
                )

    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        sys.stdout.write(report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
