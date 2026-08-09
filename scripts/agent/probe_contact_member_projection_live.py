#!/usr/bin/env python3
"""PII-safe read-only Agent probe for Contact role/department member projections.

The probe obtains one usable role ID in memory, then verifies role-member and
department-member shortcut results have a trustworthy list/count/stable-ID
shape.  It emits only aggregate counts; no role ID, department member data,
name, user ID or JSON fixture is written.  This is not a CI gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
from pathlib import Path
import subprocess
import sys
from typing import Any


def invoke(command: list[str]) -> tuple[int, Any | None]:
    completed = subprocess.run(command, text=True, capture_output=True, check=False)
    if completed.returncode != 0:
        return completed.returncode, None
    try:
        return 0, json.loads(completed.stdout)
    except json.JSONDecodeError:
        return 0, None


def first_role_id(payload: Any) -> str | None:
    if not isinstance(payload, dict) or not isinstance(payload.get("data"), dict):
        return None
    roles = payload["data"].get("roles")
    if not isinstance(roles, list):
        return None
    for role in roles:
        if not isinstance(role, dict):
            continue
        value = role.get("labelId")
        if isinstance(value, str) and value.strip():
            return value
        if isinstance(value, (int, float)) and not isinstance(value, bool):
            return str(value)
    return None


def validate_members(payload: Any) -> tuple[bool, int | None, str]:
    if not isinstance(payload, dict) or payload.get("ok") is not True or payload.get("outcome") != "success":
        return False, None, "未返回统一 success。"
    data = payload.get("data")
    meta = payload.get("meta")
    if not isinstance(data, dict) or not isinstance(meta, dict):
        return False, None, "缺少对象类型的 data/meta。"
    members = data.get("members")
    count = meta.get("count")
    if not isinstance(members, list) or not isinstance(count, int) or count != len(members):
        return False, None, "members 与 meta.count 不一致。"
    if not all(isinstance(row, dict) and isinstance(row.get("userId"), str) and row["userId"].strip() for row in members):
        return False, None, "存在缺少稳定 userId 的成员行。"
    return True, count, ""


def render(status: str, facts: list[str], boundary: str) -> str:
    lines = [
        "# Contact 成员投影 Agent 实测",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本探针仅做通讯录读取；角色 ID 仅在内存中用于一次角色成员查询。报告不保存角色、部门、用户、姓名或 JSON fixture，也不接入 CI。",
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


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dws", default="dws", help="dws executable to query (default: dws)")
    parser.add_argument("--dept-id", default="1", help="safe readable department ID to probe (default: 1)")
    parser.add_argument("--output", type=Path, help="write Markdown report; default stdout")
    args = parser.parse_args()

    try:
        rc, roles = invoke([args.dws, "contact", "+list-roles", "--format", "json"])
        role_id = first_role_id(roles) if rc == 0 else None
        if role_id is None:
            text = render("REVIEW：未获得可用角色 ID", ["角色读取失败、形状未知或为空。"], "先保留角色投影错误并修复；不要用猜测的角色 ID 查询成员。")
        else:
            role_rc, role_members = invoke([args.dws, "contact", "+list-role-members", "--id", role_id, "--format", "json"])
            dept_rc, dept_members = invoke([args.dws, "contact", "+list-dept-members", "--depts", args.dept_id, "--format", "json"])
            role_ok, role_count, role_error = validate_members(role_members) if role_rc == 0 else (False, None, f"exit code {role_rc}")
            dept_ok, dept_count, dept_error = validate_members(dept_members) if dept_rc == 0 else (False, None, f"exit code {dept_rc}")
            facts = [
                (f"角色成员：统一 success，成员数与 meta.count 一致为 {role_count}，每行均有稳定 userId。" if role_ok else f"角色成员投影未通过：{role_error}"),
                (f"部门成员：统一 success，成员数与 meta.count 一致为 {dept_count}，每行均有稳定 userId。" if dept_ok else f"部门成员投影未通过：{dept_error}"),
            ]
            text = render(
                "PASS" if role_ok and dept_ok else "REVIEW：至少一条成员投影未满足契约",
                facts,
                "本次只验证一个角色和一个部门的正常读取。仍需真实复验空列表、不同组织层级、权限受限与响应形状漂移；不可将当前结果扩张为全通讯录已完整覆盖。",
            )
    except OSError as exc:
        text = render("REVIEW：CLI 未启动", [f"启动失败：{type(exc).__name__}。"], "修复本地 CLI/认证后重新运行；不作成员目录结论。")

    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(text, encoding="utf-8")
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
