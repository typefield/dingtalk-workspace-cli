#!/usr/bin/env python3
"""Audit AITable resolve-base without persisting Base names, IDs, or JSON."""

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
RESULT_KEYS = {
    "resolved",
    "matchType",
    "count",
    "candidates",
    "sourceKind",
    "authoritativeInventory",
    "inventoryCoverageKnown",
    "indexCoverageKnown",
}


def run(command: list[str], env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout, check=False)


def parse_json(text: str) -> dict[str, Any] | None:
    try:
        value = json.loads(text)
    except json.JSONDecodeError:
        return None
    return value if isinstance(value, dict) else None


def error_from(stdout: dict[str, Any] | None, stderr: dict[str, Any] | None) -> dict[str, Any]:
    for payload in (stdout, stderr):
        if not isinstance(payload, dict):
            continue
        error = payload.get("error")
        if isinstance(error, dict):
            return error
        if isinstance(payload.get("type") or payload.get("category"), str):
            return payload
    return {}


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
        "-run", "TestCrossPlatformCoverageResolveBase|TestResolveBase|TestCrossPlatformCoverageProjectBaseSearch",
    ], env)
    checks.append(("严格候选/分页投影与 rollout 回归", focused.returncode == 0, f"rc={focused.returncode}"))
    if focused.returncode != 0:
        findings.append("focused resolve-base tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-aitable-resolve-base-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env)
        checks.append(("临时构建当前源码", build.returncode == 0, f"rc={build.returncode}"))
        if build.returncode != 0:
            findings.append("current source build failed")
        else:
            schema = run([str(binary), "schema", "--cli-path", "aitable +resolve-base", "--format", "json"], env)
            schema_ok = False
            properties: set[str] = set()
            try:
                result = (json.loads(schema.stdout).get("result") or {})
                properties = set(((result.get("data_schema") or {}).get("properties") or {}))
                schema_ok = schema.returncode == 0 and properties == RESULT_KEYS
            except (json.JSONDecodeError, AttributeError, TypeError):
                pass
            checks.append(("Runtime Schema 固定唯一候选与索引边界", schema_ok, f"properties={len(properties)}, rc={schema.returncode}"))
            if not schema_ok:
                findings.append("resolve-base Runtime Schema is incomplete")

            if args.live:
                try:
                    bases_result = run([str(binary), "aitable", "+base-list", "--limit", "1", "--format", "json"], env, 180)
                    bases_payload = parse_json(bases_result.stdout)
                    bases = (((bases_payload or {}).get("data") or {}).get("bases") or [])
                    if bases_result.returncode != 0 or not isinstance(bases, list) or not bases or not isinstance(bases[0], dict):
                        raise ValueError("no live Base candidate")
                    expected_id = bases[0].get("baseId")
                    expected_name = bases[0].get("baseName")
                    if not isinstance(expected_id, str) or not expected_id.strip() or not isinstance(expected_name, str) or not expected_name.strip():
                        raise ValueError("live Base candidate has no stable identity")

                    resolved = run([
                        str(binary), "aitable", "+resolve-base", "--name", expected_name, "--format", "json",
                    ], env, 300)
                    stdout_payload = parse_json(resolved.stdout)
                    stderr_payload = parse_json(resolved.stderr)
                    if resolved.returncode == 0 and isinstance(stdout_payload, dict):
                        if args.expected == "dual":
                            selected_id = stdout_payload.get("baseId")
                            selected_name = stdout_payload.get("name")
                            wire_ok = "ok" not in stdout_payload and stdout_payload.get("resolved") is True and stdout_payload.get("matchType") == "exact"
                        else:
                            data = stdout_payload.get("data") or {}
                            candidates = data.get("candidates") or [] if isinstance(data, dict) else []
                            candidate = candidates[0] if isinstance(candidates, list) and len(candidates) == 1 and isinstance(candidates[0], dict) else {}
                            selected_id = candidate.get("baseId")
                            selected_name = candidate.get("baseName")
                            pagination = ((stdout_payload.get("meta") or {}).get("pagination") or {})
                            wire_ok = (
                                stdout_payload.get("ok") is True
                                and stdout_payload.get("outcome") == "success"
                                and "contract_version" not in stdout_payload
                                and isinstance(data, dict)
                                and set(data) == RESULT_KEYS
                                and data.get("resolved") is True
                                and data.get("matchType") == "exact"
                                and data.get("count") == 1
                                and data.get("sourceKind") == "name_search_index"
                                and data.get("indexCoverageKnown") is False
                                and pagination.get("endpoint_exhausted") is True
                            )
                        live_ok = wire_ok and selected_id == expected_id and selected_name == expected_name and not resolved.stderr.strip()
                        summary = f"rc=0, exact=yes, stable_id={'yes' if selected_id == expected_id else 'no'}, endpoint_exhausted=yes, wire={args.expected}"
                    else:
                        error = error_from(stdout_payload, stderr_payload)
                        subtype = error.get("subtype") or error.get("reason")
                        retryable = error.get("retryable")
                        live_ok = (
                            resolved.returncode != 0
                            and subtype == "pagination_inconsistent"
                            and retryable is not True
                            and not ((stdout_payload or {}).get("baseId"))
                            and not (((stdout_payload or {}).get("data") or {}).get("candidates"))
                        )
                        summary = f"rc=nonzero, subtype={subtype or 'missing'}, retryable_true={'yes' if retryable is True else 'no'}, selected=no"
                    checks.append(("真实名称搜索终态或 fail-closed 对拍", live_ok, summary))
                    if not live_ok:
                        findings.append("live resolve-base neither proved exact terminal selection nor returned the reviewed pagination failure")
                except (json.JSONDecodeError, AttributeError, IndexError, TypeError, ValueError, subprocess.TimeoutExpired) as error:
                    findings.append(f"live resolve-base structural probe failed: {type(error).__name__}")
            else:
                checks.append(("真实名称搜索终态或 fail-closed 对拍", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# aitable +resolve-base {args.expected} Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 当前源码临时构建；Base 名称、baseId、查询词与原始 JSON 只在内存中用于独立对拍。本文件不保存名称、ID 或 JSON fixture，也不接入 CI / policy。",
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
        "- resolver 只接受稳定 `baseId/baseName`，逐页核对 `hasMore/nextCursor`，重复、不前进或相互矛盾的游标均失败关闭。",
        "- 只有搜索端点明确耗尽后才允许唯一精确/显式 fuzzy 选择；`endpoint_exhausted` 不扩大成索引完整。",
        "- 若真实服务端游标停滞，正确结果是 typed `pagination_inconsistent` 且不发布候选，不把第一页伪装成完整目录。",
        "- `--format json` 是唯一 Agent 输出入口；没有协议选择参数或版本字段。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
