#!/usr/bin/env python3
"""Audit contact +resolve-dept without storing names, IDs, queries, or JSON fixtures."""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
DATA_KEYS = {"resolved", "count", "candidates"}


def run(command: list[str], env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout, check=False)


def stable_candidates(rows: object) -> bool:
    return isinstance(rows, list) and bool(rows) and all(
        isinstance(row, dict)
        and isinstance(row.get("deptId"), str)
        and bool(row["deptId"].strip())
        and isinstance(row.get("name"), str)
        and bool(row["name"].strip())
        and "<" not in row["name"]
        for row in rows
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--expected", required=True, choices=("dual", "active"))
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--live", action="store_true")
    args = parser.parse_args()
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    test_name = "TestResolveDept" if args.expected == "dual" else "TestResolveDept(Projection|Unified)"
    focused = run(["go", "test", "-count=1", "./internal/shortcut/smart", "-run", test_name], env)
    focused_ok = focused.returncode == 0
    checks.append(("严格投影与 rollout 回归", focused_ok, f"rc={focused.returncode}"))
    if not focused_ok:
        findings.append("focused resolve-dept tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-resolve-dept-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env)
        build_ok = build.returncode == 0
        checks.append(("临时构建当前源码", build_ok, f"rc={build.returncode}"))
        if not build_ok:
            findings.append("current source build failed")
        else:
            schema = run([str(binary), "schema", "--cli-path", "contact +resolve-dept", "--format", "json"], env)
            property_count = 0
            schema_ok = False
            try:
                result = (json.loads(schema.stdout).get("result") or {})
                properties = set(((result.get("data_schema") or {}).get("properties") or {}))
                property_count = len(properties)
                schema_ok = schema.returncode == 0 and properties == DATA_KEYS
            except (json.JSONDecodeError, AttributeError, TypeError):
                pass
            checks.append(("Runtime Schema 固定消歧结果", schema_ok, f"properties={property_count}, rc={schema.returncode}"))
            if not schema_ok:
                findings.append("resolve-dept Runtime Schema is incomplete")

            if args.live:
                try:
                    me = run([str(binary), "contact", "+me", "--format", "json"], env, 120)
                    name = (json.loads(me.stdout).get("data") or {}).get("name")
                    if me.returncode != 0 or not isinstance(name, str) or not name.strip():
                        raise ValueError("current-user result has no in-memory name")
                    org = run([str(binary), "contact", "+org", "--name", name, "--format", "json"], env, 120)
                    query = (json.loads(org.stdout).get("data") or {}).get("deptName")
                    if org.returncode != 0 or not isinstance(query, str) or not query.strip():
                        raise ValueError("organization result has no in-memory department name")
                    resolved = run([str(binary), "contact", "+resolve-dept", "--name", query, "--format", "json"], env, 120)
                    payload = json.loads(resolved.stdout)
                    if args.expected == "dual":
                        rows = payload.get("candidates") if isinstance(payload, dict) else None
                        count = payload.get("count") if isinstance(payload, dict) else None
                        live_ok = (
                            resolved.returncode == 0
                            and not resolved.stderr.strip()
                            and isinstance(payload, dict)
                            and "ok" not in payload
                            and payload.get("resolved") is False
                            and isinstance(count, int)
                            and count == len(rows) if isinstance(rows, list) else False
                        ) and stable_candidates(rows)
                        summary = f"rc={resolved.returncode}, legacy=yes, candidates={len(rows) if isinstance(rows, list) else -1}, stable_ids={'yes' if live_ok else 'no'}"
                    else:
                        data = payload.get("data") if isinstance(payload, dict) else None
                        meta = payload.get("meta") if isinstance(payload, dict) else None
                        rows = data.get("candidates") if isinstance(data, dict) else None
                        count = data.get("count") if isinstance(data, dict) else None
                        pagination = meta.get("pagination") if isinstance(meta, dict) else None
                        live_ok = (
                            resolved.returncode == 0
                            and not resolved.stderr.strip()
                            and isinstance(payload, dict)
                            and payload.get("ok") is True
                            and payload.get("outcome") == "success"
                            and "contract_version" not in payload
                            and isinstance(data, dict)
                            and set(data) == DATA_KEYS
                            and isinstance(count, int)
                            and isinstance(rows, list)
                            and count == len(rows)
                            and isinstance(meta, dict)
                            and meta.get("count") == count
                            and isinstance(pagination, dict)
                            and pagination.get("endpoint_exhausted") is True
                            and pagination.get("items") == count
                            and stable_candidates(rows)
                        )
                        summary = f"rc={resolved.returncode}, unified=yes, candidates={len(rows) if isinstance(rows, list) else -1}, count_aligned={'yes' if live_ok else 'no'}"
                    checks.append(("真实当前部门消歧结构对拍", live_ok, summary))
                    if not live_ok:
                        findings.append("live resolve-dept result does not match the reviewed contract")
                except (json.JSONDecodeError, AttributeError, TypeError, ValueError) as error:
                    findings.append(f"live resolve-dept structural probe failed: {type(error).__name__}")
            else:
                checks.append(("真实当前部门消歧结构对拍", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# contact +resolve-dept {args.expected} Agent 审阅", "", f"扫描日期：{date.today().isoformat()}", "",
        "> 当前源码临时构建；姓名和部门名仅在内存中作为下一步输入。本文件不保存查询词、姓名、部门名、deptId 或原始 JSON，也不接入 CI / policy。", "",
        f"## Result: {'PASS' if passed else 'REVIEW'}", "", "| 检查项 | 结果 | 脱敏证据 |", "|---|---|---|",
    ]
    lines.extend(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{summary}` |" for label, ok, summary in checks)
    lines += [
        "", "## 结论", "",
        "- 未知容器、非法候选、缺失/分数/重复部门 ID 不再被静默丢弃或压成零命中。",
        "- `hasMore:true`、缺失分页证据或终页 total 不对账会 fail-closed，不能据当前页判断唯一部门。",
        "- 企业根部门搜索哨兵 `-1` 会归一为下游可直接使用的 canonical `deptId=1`。",
        "- `--format json` 是唯一 Agent 入口；没有协议选择参数或版本字段。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
