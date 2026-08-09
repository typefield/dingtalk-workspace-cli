#!/usr/bin/env python3
"""PII-safe live shape audit for ``contact +list-roles``.

The shortcut deliberately fails closed when the service response cannot be
projected.  This probe enables the opt-in lower-layer diagnostic, holds that
response in memory, and writes only structural keys/types/cardinalities to a
Markdown report.  It never writes lower-layer JSON, role names, role IDs or
member data to disk.  It is a manual Agent tool, not CI.
"""

from __future__ import annotations

import argparse
from collections.abc import Mapping
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import sys
from typing import Any


def shape(value: Any, depth: int = 0) -> str:
    """Return a value-free structural signature with bounded recursion."""

    if depth >= 6:
        return "…"
    if isinstance(value, Mapping):
        keys = sorted(str(key) for key in value)
        children = ", ".join(f"{key}:{shape(value[key], depth + 1)}" for key in keys[:12])
        suffix = ", …" if len(keys) > 12 else ""
        return "{" + children + suffix + "}"
    if isinstance(value, list):
        if not value:
            return "[]"
        signatures = sorted({shape(item, depth + 1) for item in value[:20]})
        suffix = ", …" if len(value) > 20 else ""
        return f"array(len={len(value)}; item=" + " | ".join(signatures[:4]) + suffix + ")"
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "bool"
    if isinstance(value, (int, float)) and not isinstance(value, bool):
        return "number"
    if isinstance(value, str):
        return "string"
    return type(value).__name__


def raw_records(stderr: str) -> list[tuple[str, str, Any]]:
    records: list[tuple[str, str, Any]] = []
    for line in stderr.splitlines():
        if not line.startswith("DWSRAW\t"):
            continue
        fields = line.split("\t", 3)
        if len(fields) != 4:
            continue
        try:
            records.append((fields[1], fields[2], json.loads(fields[3])))
        except json.JSONDecodeError:
            records.append((fields[1], fields[2], None))
    return records


def paired_empty_label(row: Any) -> bool:
    if not isinstance(row, Mapping):
        return False
    id_keys = ("labelId", "label_id", "id")
    name_keys = ("labelName", "label_name", "name")
    present_id = any(key in row for key in id_keys)
    present_name = any(key in row for key in name_keys)

    def blank(keys: tuple[str, ...]) -> bool:
        for key in keys:
            value = row.get(key)
            if value is None:
                continue
            if isinstance(value, str) and not value.strip():
                continue
            return False
        return True

    return present_id and present_name and blank(id_keys) and blank(name_keys)


def grouped_role_counts(raw: Any) -> tuple[int, int] | None:
    """Return lower label rows and paired-empty placeholders without values."""

    if not isinstance(raw, Mapping) or not isinstance(raw.get("result"), list):
        return None
    total = 0
    placeholders = 0
    for group in raw["result"]:
        if not isinstance(group, Mapping) or not isinstance(group.get("labels"), list):
            return None
        for label in group["labels"]:
            total += 1
            if paired_empty_label(label):
                placeholders += 1
    return total, placeholders


def render(status: str, facts: list[str], next_step: str) -> str:
    lines = [
        "# Contact roles 投影形状 Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本探针只读调用 `contact +list-roles`，原始下层响应仅在进程内解析；报告不保存角色名称、ID、成员资料或 JSON fixture，也不接入 CI。",
        "",
        "## 结果",
        "",
        f"**{status}**",
        "",
        "## 结构事实",
        "",
    ]
    lines.extend(f"- {fact}" for fact in facts)
    lines.extend(["", "## 下一步", "", next_step, ""])
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws", help="dws executable to query (default: dws)")
    parser.add_argument("--output", type=Path, help="write Markdown report; default stdout")
    args = parser.parse_args()

    env = os.environ.copy()
    env["DWS_DUMP_RAW"] = "1"
    try:
        completed = subprocess.run(
            [args.dws, "contact", "+list-roles", "--format", "json"],
            text=True,
            capture_output=True,
            env=env,
            check=False,
        )
    except OSError as exc:
        text = render("REVIEW：CLI 未启动", [f"启动失败：{type(exc).__name__}。"], "修复本地 CLI/认证后重新运行；不要据此猜测角色是否为空。")
    else:
        try:
            upper = json.loads(completed.stdout)
        except json.JSONDecodeError:
            upper = None
        records = raw_records(completed.stderr)
        contact_records = [(tool, raw) for server, tool, raw in records if server == "contact"]
        facts = [f"命令 exit code：{completed.returncode}；捕获的 contact 下层响应数：{len(contact_records)}。"]
        if isinstance(upper, dict):
            facts.append(
                "上层结果："
                + ("统一 success。" if upper.get("ok") is True and upper.get("outcome") == "success" else "未被误报为统一 success。")
            )
        else:
            facts.append("上层 stdout 不是可解析 JSON；不能建立业务结论。")
        if not contact_records:
            text = render("REVIEW：未捕获下层响应", facts, "检查本地构建是否支持 DWS_DUMP_RAW，并保留 fail-closed 行为。")
        else:
            tool, raw = contact_records[-1]
            facts.append(f"下层工具：`contact/{tool}`；仅记录结构签名：`{shape(raw)}`。")
            lower_counts = grouped_role_counts(raw)
            upper_roles = upper.get("data", {}).get("roles") if isinstance(upper, dict) and isinstance(upper.get("data"), dict) else None
            if lower_counts is not None:
                total, placeholders = lower_counts
                facts.append(f"下层标签行数：{total}；成对空占位行数：{placeholders}；可投影角色预期数：{total - placeholders}。")
            if isinstance(upper_roles, list):
                facts.append(f"上层已投影角色数：{len(upper_roles)}。")
            if isinstance(upper, dict) and upper.get("ok") is False and upper.get("error", {}).get("subtype") == "projection_unknown":
                text = render(
                    "REVIEW：真实响应尚未被角色投影支持，但已 fail-closed",
                    facts,
                    "依据上述无值结构补充 projector 与同形状测试；在投影完整前继续返回 projection_unknown，禁止降级为空列表。",
                )
            elif lower_counts is not None and isinstance(upper_roles, list) and len(upper_roles) != lower_counts[0] - lower_counts[1]:
                text = render(
                    "REVIEW：角色投影计数与下层可用行不一致",
                    facts,
                    "保持 fail-closed 并检查下层分组/占位规则；禁止静默丢弃或补造角色。",
                )
            else:
                text = render(
                    "PASS：当前响应未触发角色投影未知错误",
                    facts,
                    "仍需验证分组、空列表和权限受限形状；当前一次成功不代表组织角色目录完整。",
                )

    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(text, encoding="utf-8")
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
