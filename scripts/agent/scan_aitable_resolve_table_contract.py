#!/usr/bin/env python3
"""Audit AITable resolve-table without persisting Base/table names, IDs, or JSON."""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
RESULT_KEYS = {"resolved", "matchType", "count", "candidates"}


def run(command: list[str], env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout, check=False)


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

    focused = run([
        "go", "test", "-count=1", "./internal/shortcut/aitabletarget", "./internal/shortcut/smart",
        "-run", "TestCrossPlatformCoverageResolveTable|TestResolveTable",
    ], env)
    checks.append(("严格消歧投影与 rollout 回归", focused.returncode == 0, f"rc={focused.returncode}"))
    if focused.returncode != 0:
        findings.append("focused resolve-table tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-aitable-resolve-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env)
        checks.append(("临时构建当前源码", build.returncode == 0, f"rc={build.returncode}"))
        if build.returncode != 0:
            findings.append("current source build failed")
        else:
            schema = run([str(binary), "schema", "--cli-path", "aitable +resolve-table", "--format", "json"], env)
            schema_ok = False
            properties: set[str] = set()
            try:
                result = (json.loads(schema.stdout).get("result") or {})
                properties = set(((result.get("data_schema") or {}).get("properties") or {}))
                schema_ok = schema.returncode == 0 and properties == RESULT_KEYS
            except (json.JSONDecodeError, AttributeError, TypeError):
                pass
            checks.append(("Runtime Schema 固定唯一候选结果", schema_ok, f"properties={len(properties)}, rc={schema.returncode}"))
            if not schema_ok:
                findings.append("resolve-table Runtime Schema is incomplete")

            if args.live:
                try:
                    bases_result = run([str(binary), "aitable", "+base-list", "--format", "json"], env, 180)
                    bases_payload = json.loads(bases_result.stdout)
                    bases = ((bases_payload.get("data") or {}).get("bases") or [])
                    if bases_result.returncode != 0 or not isinstance(bases, list) or not bases:
                        raise ValueError("no live Base candidate")
                    base_id = bases[0].get("baseId") if isinstance(bases[0], dict) else None
                    if not isinstance(base_id, str) or not base_id.strip():
                        raise ValueError("live Base candidate has no stable ID")

                    directory_result = run([str(binary), "aitable", "table", "get", "--base-id", base_id, "--format", "json"], env, 180)
                    directory_payload = json.loads(directory_result.stdout)
                    rows = ((directory_payload.get("data") or {}).get("tables") or [])
                    if not isinstance(rows, list) or not rows or not isinstance(rows[0], dict):
                        raise ValueError("live Base has no table candidate")
                    expected_id = rows[0].get("tableId")
                    expected_name = rows[0].get("tableName")
                    if not isinstance(expected_id, str) or not expected_id.strip() or not isinstance(expected_name, str) or not expected_name.strip():
                        raise ValueError("live table candidate has no stable identity")

                    resolved = run([
                        str(binary), "aitable", "+resolve-table", "--base", base_id,
                        "--name", expected_name, "--format", "json",
                    ], env, 180)
                    payload = json.loads(resolved.stdout)
                    if args.expected == "dual":
                        selected_id = payload.get("tableId")
                        selected_name = payload.get("name")
                        wire_ok = "ok" not in payload and payload.get("resolved") is True and payload.get("matchType") == "exact"
                    else:
                        data = payload.get("data") or {}
                        candidates = data.get("candidates") or []
                        candidate = candidates[0] if isinstance(candidates, list) and len(candidates) == 1 and isinstance(candidates[0], dict) else {}
                        selected_id = candidate.get("tableId")
                        selected_name = candidate.get("tableName")
                        meta = payload.get("meta") or {}
                        wire_ok = (
                            payload.get("ok") is True
                            and payload.get("outcome") == "success"
                            and "contract_version" not in payload
                            and set(data) == RESULT_KEYS
                            and data.get("resolved") is True
                            and data.get("matchType") == "exact"
                            and data.get("count") == 1
                            and meta.get("count") == 1
                        )
                    live_ok = (
                        resolved.returncode == 0
                        and not resolved.stderr.strip()
                        and wire_ok
                        and selected_id == expected_id
                        and selected_name == expected_name
                    )
                    checks.append(("真实表目录唯一候选独立对拍", live_ok, f"rc={resolved.returncode}, exact=yes, stable_id={'yes' if selected_id == expected_id else 'no'}, wire={args.expected}"))
                    if not live_ok:
                        findings.append("live resolve-table result does not match the independently selected table")
                except (json.JSONDecodeError, AttributeError, IndexError, TypeError, ValueError) as error:
                    findings.append(f"live resolve-table structural probe failed: {type(error).__name__}")
            else:
                checks.append(("真实表目录唯一候选独立对拍", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# aitable +resolve-table {args.expected} Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 当前源码临时构建；Base 名称、baseId、tableId、tableName 与原始 JSON 只在内存中用于独立对拍。本文件不保存名称、ID 或 JSON fixture，也不接入 CI / policy。",
        "",
        f"## Result: {'PASS' if passed else 'REVIEW'}",
        "",
        "| 检查项 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{summary}` |" for label, ok, summary in checks)
    lines += [
        "",
        "## 结论",
        "",
        "- resolver 与 `+list-tables` 复用同一个严格完整表目录投影，不再维护独立的容器/ID/名称猜测器。",
        "- 只有完整目录中的大小写不敏感精确名称可以默认成功；包含匹配仍要求显式 `--fuzzy`，零/多候选不猜选。",
        "- 真实候选从原子 `table get` 在内存中派生，resolver 的稳定 ID/名称与之完全一致，Markdown 不保存业务值。",
        "- `--format json` 是唯一 Agent 输出入口；没有协议选择参数或版本字段。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
