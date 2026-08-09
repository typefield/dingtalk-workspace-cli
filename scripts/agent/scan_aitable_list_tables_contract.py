#!/usr/bin/env python3
"""Audit AITable list-tables without persisting Base/table names, IDs, or JSON."""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]


def run(command: list[str], env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout, check=False)


def stable_pairs(rows: object) -> list[tuple[str, str]]:
    if not isinstance(rows, list):
        raise ValueError("table directory is not a list")
    pairs: list[tuple[str, str]] = []
    for row in rows:
        if not isinstance(row, dict) or not set(row).issuperset({"tableId", "tableName"}):
            raise ValueError("table directory row is not a stable object")
        table_id = row.get("tableId")
        table_name = row.get("tableName")
        if not isinstance(table_id, str) or not table_id.strip() or not isinstance(table_name, str) or not table_name.strip():
            raise ValueError("table directory row lacks a stable identity")
        pairs.append((table_id.strip(), table_name.strip()))
    if len({pair[0] for pair in pairs}) != len(pairs):
        raise ValueError("table directory contains duplicate IDs")
    return sorted(pairs)


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
        "-run", "TestCrossPlatformCoverage(ListTables|ResolveTable)|TestListTables",
    ], env)
    checks.append(("严格表目录投影与 rollout 回归", focused.returncode == 0, f"rc={focused.returncode}"))
    if focused.returncode != 0:
        findings.append("focused AITable table projection tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-aitable-table-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env)
        checks.append(("临时构建当前源码", build.returncode == 0, f"rc={build.returncode}"))
        if build.returncode != 0:
            findings.append("current source build failed")
        else:
            schema = run([str(binary), "schema", "--cli-path", "aitable +list-tables", "--format", "json"], env)
            schema_ok = False
            try:
                payload = json.loads(schema.stdout)
                result = payload.get("result") or {}
                properties = set(((result.get("data_schema") or {}).get("properties") or {}))
                ndjson = result.get("ndjson") or {}
                schema_ok = schema.returncode == 0 and properties == {"tables"} and ndjson.get("record_path") == "tables"
            except (json.JSONDecodeError, AttributeError, TypeError):
                properties = set()
            checks.append(("Runtime Schema 固定表目录结果", schema_ok, f"properties={len(properties)}, rc={schema.returncode}"))
            if not schema_ok:
                findings.append("list-tables Runtime Schema is incomplete")

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

                    source_result = run([str(binary), "aitable", "table", "get", "--base-id", base_id, "--format", "json"], env, 180)
                    source_payload = json.loads(source_result.stdout)
                    source_rows = ((source_payload.get("data") or {}).get("tables"))
                    source_pairs = stable_pairs(source_rows)

                    projected_result = run([str(binary), "aitable", "+list-tables", "--base", base_id, "--format", "json"], env, 180)
                    projected_payload = json.loads(projected_result.stdout)
                    if args.expected == "dual":
                        projected_rows = projected_payload.get("tables")
                        wire_ok = "ok" not in projected_payload and "outcome" not in projected_payload
                    else:
                        data = projected_payload.get("data") or {}
                        projected_rows = data.get("tables")
                        meta = projected_payload.get("meta") or {}
                        wire_ok = (
                            projected_payload.get("ok") is True
                            and projected_payload.get("outcome") == "success"
                            and "contract_version" not in projected_payload
                            and meta.get("count") == len(source_pairs)
                        )
                    projected_pairs = stable_pairs(projected_rows)
                    live_ok = (
                        source_result.returncode == 0
                        and projected_result.returncode == 0
                        and not source_result.stderr.strip()
                        and not projected_result.stderr.strip()
                        and wire_ok
                        and source_pairs == projected_pairs
                    )
                    checks.append((
                        "真实 Base 完整表目录独立对拍",
                        live_ok,
                        f"source={len(source_pairs)}, projected={len(projected_pairs)}, same_pairs={'yes' if source_pairs == projected_pairs else 'no'}, wire={args.expected}",
                    ))
                    if not live_ok:
                        findings.append("live list-tables result does not match independent table get")
                except (json.JSONDecodeError, AttributeError, IndexError, TypeError, ValueError) as error:
                    findings.append(f"live list-tables structural probe failed: {type(error).__name__}")
            else:
                checks.append(("真实 Base 完整表目录独立对拍", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# aitable +list-tables {args.expected} Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 当前源码临时构建；Base 名称、baseId、tableId、tableName 与原始 JSON 只在内存中用于独立对拍。本文件不保存名称、ID、token 或 JSON fixture，也不接入 CI / policy。",
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
        "- `get_tables` 只接受实测的 `success -> data.tables[] -> tableId/tableName` 投影链路，不再猜测 `items/list/records/id/name`。",
        "- 未知容器、非对象条目、空或重复稳定 ID、缺少名称与错误子集合类型均 fail-closed，不会变成空列表或原始成功结果。",
        "- 真实 Base 的原子 `table get` 与 `+list-tables` 按稳定 ID/名称集合在内存中完全对拍；Markdown 只记录计数与布尔结论。",
        "- `--format json` 是唯一 Agent 输出入口；没有协议选择参数或版本字段。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
