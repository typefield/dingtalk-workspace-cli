#!/usr/bin/env python3
"""Agent audit for contact +lookup full-profile result without storing profile data."""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
TOP_LEVEL_DATA = {"userId", "name", "email", "mobile", "jobNumber", "title", "isAdmin", "organization", "departments", "positions", "labels"}


def run(command: list[str], env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout, check=False)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--live", action="store_true")
    args = parser.parse_args()
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    focused = run(["go", "test", "-count=1", "./internal/shortcut/smart", "-run", "TestLookup(FullProfileProjectionAndFailClosedBoundary|UnifiedResultPreservesReviewedFullProfile)"], env)
    ok = focused.returncode == 0
    checks.append(("完整投影与 fail-closed 回归", ok, f"rc={focused.returncode}"))
    if not ok:
        findings.append("focused full-profile tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-contact-lookup-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env)
        ok = build.returncode == 0
        checks.append(("临时构建当前源码", ok, f"rc={build.returncode}"))
        if not ok:
            findings.append("current source build failed")
        else:
            schema = run([str(binary), "schema", "--cli-path", "contact +lookup", "--format", "json"], env)
            schema_ok = False
            try:
                result = (json.loads(schema.stdout).get("result") or {})
                properties = set(((result.get("data_schema") or {}).get("properties") or {}))
                sensitive = set(result.get("sensitive_paths") or [])
                schema_ok = schema.returncode == 0 and properties == TOP_LEVEL_DATA and len(sensitive) == 5
            except (json.JSONDecodeError, AttributeError, TypeError):
                pass
            checks.append(("Runtime Schema 覆盖完整资料与敏感路径", schema_ok, f"properties={len(properties) if 'properties' in locals() else 0}, sensitive={len(sensitive) if 'sensitive' in locals() else 0}, rc={schema.returncode}"))
            if not schema_ok:
                findings.append("contact +lookup Runtime Schema is incomplete")

            if args.live:
                try:
                    me = run([str(binary), "contact", "+me", "--format", "json"], env, 90)
                    name = (json.loads(me.stdout).get("data") or {}).get("name")
                    if me.returncode != 0 or not isinstance(name, str) or not name.strip():
                        raise ValueError("current-user result has no in-memory name")
                    lookup = run([str(binary), "contact", "+lookup", "--name", name, "--format", "json"], env, 90)
                    payload = json.loads(lookup.stdout)
                    data = payload.get("data") if isinstance(payload, dict) else None
                    departments = data.get("departments") if isinstance(data, dict) else None
                    positions = data.get("positions") if isinstance(data, dict) else None
                    labels = data.get("labels") if isinstance(data, dict) else None
                    stable_depts = isinstance(departments, list) and all(isinstance(row, dict) and row.get("id") not in (None, "") for row in departments)
                    stable_positions = isinstance(positions, list) and all(isinstance(row, dict) and row.get("departmentId") not in (None, "") for row in positions)
                    stable_labels = isinstance(labels, list) and all(isinstance(row, dict) and row.get("id") not in (None, "") for row in labels)
                    live_ok = (
                        lookup.returncode == 0
                        and not lookup.stderr.strip()
                        and payload.get("ok") is True
                        and payload.get("outcome") == "success"
                        and "contract_version" not in payload
                        and isinstance(data, dict)
                        and set(data) <= TOP_LEVEL_DATA
                        and isinstance(data.get("userId"), str)
                        and bool(data["userId"].strip())
                        and stable_depts and stable_positions and stable_labels
                        and "orgEmployeeModel" not in data
                    )
                    summary = (
                        f"rc={lookup.returncode}, fields={len(data) if isinstance(data, dict) else 0}, "
                        f"departments={len(departments) if isinstance(departments, list) else -1}, "
                        f"positions={len(positions) if isinstance(positions, list) else -1}, "
                        f"labels={len(labels) if isinstance(labels, list) else -1}, stable_ids={'yes' if live_ok else 'no'}"
                    )
                    checks.append(("真实当前用户完整资料结构对拍", live_ok, summary))
                    if not live_ok:
                        findings.append("live contact +lookup result does not match the reviewed full projection")
                except (json.JSONDecodeError, AttributeError, TypeError, ValueError) as error:
                    findings.append(f"live contact +lookup structural probe failed: {error}")
            else:
                checks.append(("真实当前用户完整资料结构对拍", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        "# contact +lookup 完整资料统一结果 Agent 审阅", "", f"扫描日期：{date.today().isoformat()}", "",
        "> 当前源码临时构建；资料 JSON 只在内存解析。本文件不保存姓名、联系方式、工号、userId、部门/职位/标签内容或原始响应，也不接入 CI / policy。", "",
        f"## Result: {'PASS' if passed else 'REVIEW'}", "", "| 检查项 | 结果 | 脱敏证据 |", "|---|---|---|",
    ]
    lines.extend(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{summary}` |" for label, ok, summary in checks)
    lines += [
        "", "## 结论", "",
        "- `contact +lookup --format json` 返回统一 `ok/outcome/data`，不含协议选择参数、版本标记或原始 `orgEmployeeModel` 包装。",
        "- 投影保留身份、组织、部门、职位、标签与联系方式，不以精简字段换取统一输出。",
        "- 唯一用户、稳定 userId、三类数组及逐项稳定 ID 是成功前提；未知字段、类型漂移和别名冲突均 fail-closed。",
        "- 当前用户成功只证明本次响应可完整投影，不证明其他用户、非空标签、权限受限或后端新增字段。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
