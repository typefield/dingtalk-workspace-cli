#!/usr/bin/env python3
"""Bounded live review for excluded Agoal strategy/contract list commands.

The scanner keeps user, department, strategy, and contract identifiers only in
memory.  It writes shape/count evidence as Markdown and never stores response
JSON.  It is an Agent review tool, not a CI gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile
from typing import Any


ROOT = Path(__file__).resolve().parents[2]


def run(command: list[str], env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        command,
        cwd=ROOT,
        env=env,
        text=True,
        capture_output=True,
        check=False,
        timeout=timeout,
    )


def parse_json(text: str) -> Any:
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return None


def current_user_id(binary: Path, env: dict[str, str]) -> str | None:
    result = run([str(binary), "profile", "list", "--format", "json"], env)
    payload = parse_json(result.stdout)
    if result.returncode != 0 or not isinstance(payload, dict):
        return None
    current = payload.get("currentProfile")
    profiles = payload.get("profiles")
    if not isinstance(current, str) or not isinstance(profiles, list):
        return None
    matches = [
        row.get("userId")
        for row in profiles
        if isinstance(row, dict) and row.get("profile") == current
    ]
    return matches[0] if len(matches) == 1 and isinstance(matches[0], str) and matches[0].strip() else None


def department_ids(binary: Path, env: dict[str, str], limit: int) -> list[str]:
    result = run([
        str(binary), "contact", "+list-sub-depts", "--dept", "1", "--format", "json",
    ], env)
    payload = parse_json(result.stdout)
    data = payload.get("data") if isinstance(payload, dict) else None
    rows = data.get("depts") if isinstance(data, dict) else None
    if result.returncode != 0 or not isinstance(rows, list):
        return []
    values: list[str] = []
    for row in rows:
        value = row.get("deptId") if isinstance(row, dict) else None
        if value is None:
            continue
        text = str(value).strip()
        if text and text not in values:
            values.append(text)
        if len(values) >= max(limit, 0):
            break
    return values


def inspect_command(
    binary: Path,
    env: dict[str, str],
    command_group: str,
    scopes: list[tuple[str, str]],
) -> dict[str, Any]:
    stats: dict[str, Any] = {
        "sampled": 0,
        "empty": 0,
        "nonempty": 0,
        "null_success": 0,
        "typed_failure": 0,
        "unexpected": 0,
        "row_keys": set(),
    }
    for scope_type, scope_id in scopes:
        result = run([
            str(binary), "agoal", command_group, "list",
            "--scope-type", scope_type,
            "--scope-id", scope_id,
            "--format", "json",
        ], env)
        stats["sampled"] += 1
        payload = parse_json(result.stdout)
        if result.returncode == 0:
            if payload is None and result.stdout.strip() == "null":
                stats["null_success"] += 1
                continue
            if (
                isinstance(payload, dict)
                and payload.get("success") is True
                and isinstance(payload.get("content"), list)
            ):
                rows = payload["content"]
                if not rows:
                    stats["empty"] += 1
                    continue
                if all(isinstance(row, dict) for row in rows):
                    stats["nonempty"] += 1
                    for row in rows:
                        stats["row_keys"].update(row.keys())
                    continue
            stats["unexpected"] += 1
            continue
        error_payload = parse_json(result.stderr)
        error = error_payload.get("error") if isinstance(error_payload, dict) else None
        if isinstance(error, dict) and isinstance(error.get("type"), str):
            stats["typed_failure"] += 1
        else:
            stats["unexpected"] += 1
    stats["row_keys"] = sorted(stats["row_keys"])
    return stats


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--live", action="store_true")
    parser.add_argument("--sample-limit", type=int, default=10)
    args = parser.parse_args()
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    focused = run([
        "go", "test", "-count=1", "./internal/helpers", "./internal/cli",
        "-run", "TestAgoal|TestReviewedRuntimeSchemaExclusions|TestSchemaCatalogDeliveryCompleteness",
    ], env, 900)
    checks.append(("Agoal 与精确 exclusion 回归", focused.returncode == 0, f"rc={focused.returncode}"))
    if focused.returncode != 0:
        findings.append("focused Agoal/exclusion tests failed")

    results: dict[str, dict[str, Any]] = {}
    with tempfile.TemporaryDirectory(prefix="dws-agoal-list-boundary-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env, 900)
        checks.append(("临时构建当前源码", build.returncode == 0, f"rc={build.returncode}"))
        if build.returncode != 0:
            findings.append("current source build failed")
        else:
            help_ok = True
            for group in ("strategy", "contract"):
                help_result = run([str(binary), "agoal", group, "list", "--help"], env)
                help_ok = help_ok and help_result.returncode == 0 and all(
                    flag in help_result.stdout for flag in ("--scope-type", "--scope-id", "--request-id")
                )
            checks.append(("两条兼容命令 Help 可发现", help_ok, "canonical_flags=yes" if help_ok else "shape mismatch"))
            if not help_ok:
                findings.append("strategy/contract list Help is not discoverable")

            invalid_ok = True
            for group in ("strategy", "contract"):
                invalid = run([
                    str(binary), "agoal", group, "list",
                    "--scope-type", "organization", "--scope-id", "1", "--format", "json",
                ], env)
                invalid_payload = parse_json(invalid.stderr)
                invalid_error = invalid_payload.get("error") if isinstance(invalid_payload, dict) else None
                invalid_ok = invalid_ok and (
                    invalid.returncode == 3
                    and not invalid.stdout.strip()
                    and isinstance(invalid_error, dict)
                    and invalid_error.get("type") == "validation"
                    and invalid_error.get("subtype") == "invalid_flag_value"
                )
            checks.append((
                "非法 scope-type 在远端调用前 fail-closed",
                invalid_ok,
                "two commands typed validation rc=3" if invalid_ok else "machine error mismatch",
            ))
            if not invalid_ok:
                findings.append("invalid strategy/contract scope type did not fail as typed local validation")

            if args.live:
                user_id = current_user_id(binary, env)
                depts = department_ids(binary, env, args.sample_limit)
                scopes: list[tuple[str, str]] = []
                if user_id:
                    scopes.append(("PERSONAL", user_id))
                scopes.append(("DEPT", "1"))
                scopes.extend(("DEPT", dept_id) for dept_id in depts)
                checks.append((
                    "脱敏抽样范围可建立",
                    bool(user_id) and bool(depts),
                    f"personal={'yes' if user_id else 'no'}, root=1, child_samples={len(depts)}",
                ))
                if not user_id or not depts:
                    findings.append("could not establish the bounded personal/department sample")
                for group in ("strategy", "contract"):
                    results[group] = inspect_command(binary, env, group, scopes)
                    stats = results[group]
                    safe = stats["null_success"] == 0 and stats["unexpected"] == 0
                    checks.append((
                        f"{group} list 有界真实响应",
                        safe,
                        (
                            f"sampled={stats['sampled']}, empty={stats['empty']}, "
                            f"nonempty={stats['nonempty']}, typed_failure={stats['typed_failure']}, "
                            f"null_success={stats['null_success']}, unexpected={stats['unexpected']}"
                        ),
                    ))
                    if not safe:
                        findings.append(f"{group} list returned null-success or an unrecognized machine result")
            else:
                checks.append(("有界真实响应", True, "SKIPPED（未传 --live）"))

    lines = [
        "# Agoal strategy/contract list Agent 边界审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 当前用户、部门和业务对象 ID 仅在 Agent 内存中使用；报告只保存响应形状与计数，不保存原始 JSON，也不接入 CI / policy。",
        "",
        f"## Result: {'PASS' if not findings else 'REVIEW'}",
        "",
        "| 检查项 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{evidence}` |" for label, ok, evidence in checks)
    if results:
        lines += ["", "## 非空行字段证据", ""]
        for group in ("strategy", "contract"):
            keys = results.get(group, {}).get("row_keys") or []
            lines.append(f"- `{group} list`: " + (", ".join(f"`{key}`" for key in keys) if keys else "UNVERIFIED（未观察到非空行）"))
    lines += [
        "",
        "## 结论",
        "",
        "- 空数组只证明本次所选身份与范围没有返回行，不能扩大成组织不存在战略解码或经营合约。",
        "- `--scope-type` 只允许 DEPT/PERSONAL；大小写在本地归一，其他值在任何业务调用前以 `validation/invalid_flag_value`、rc=3 拒绝。",
        "- 未取得非空行时无法验证稳定业务 ID、嵌套类型和 detail/update 所需上下文，因此两条命令继续保持精确 exclusion。",
        "- 一旦有界抽样观察到非空行，应基于字段证据定义严格 ResultSpec，并按单命令 `legacy_only → dual_validate → unified_active` 迁移。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if not findings else 1


if __name__ == "__main__":
    raise SystemExit(main())
