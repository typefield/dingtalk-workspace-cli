#!/usr/bin/env python3
"""Audit AITable base-get without persisting Base names, IDs, or JSON."""

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
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, timeout=timeout, check=False)


def object_json(text: str) -> dict[str, Any] | None:
    try:
        value = json.loads(text)
    except json.JSONDecodeError:
        return None
    return value if isinstance(value, dict) else None


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

    focused = run(["go", "test", "-count=1", "./internal/shortcut/aitable", "-run", "TestBaseGetProjection"] , env)
    focused_ok = focused.returncode == 0
    checks.append(("严格目录投影回归", focused_ok, f"rc={focused.returncode}"))
    if not focused_ok:
        findings.append("focused base-get projection tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-aitable-base-get-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env)
        build_ok = build.returncode == 0
        checks.append(("临时构建当前源码", build_ok, f"rc={build.returncode}"))
        if not build_ok:
            findings.append("current source build failed")
        elif args.live:
            try:
                listed = run([str(binary), "aitable", "+base-list", "--limit", "1", "--format", "json"], env, 180)
                list_payload = object_json(listed.stdout) or {}
                bases = ((list_payload.get("data") or {}).get("bases") or [])
                if listed.returncode != 0 or not isinstance(bases, list) or not bases or not isinstance(bases[0], dict):
                    raise ValueError("no live Base candidate")
                base_id = bases[0].get("baseId")
                if not isinstance(base_id, str) or not base_id.strip():
                    raise ValueError("candidate has no stable baseId")

                result = run([str(binary), "aitable", "+base-get", "--base-id", base_id, "--format", "json"], env, 180)
                payload = object_json(result.stdout)
                if result.returncode == 0 and isinstance(payload, dict):
                    if args.expected == "dual":
                        data = payload.get("data")
                        wire_ok = "ok" not in payload and payload.get("success") is True and isinstance(data, dict)
                    else:
                        data = payload.get("data")
                        wire_ok = payload.get("ok") is True and payload.get("outcome") == "success" and isinstance(data, dict)
                    tables = data.get("tables") if isinstance(data, dict) else None
                    dashboards = data.get("dashboards") if isinstance(data, dict) else None
                    documents = data.get("documents") if isinstance(data, dict) else None
                    live_ok = (
                        wire_ok
                        and isinstance(tables, list)
                        and isinstance(dashboards, list)
                        and isinstance(documents, list)
                        and not result.stderr.strip()
                        and "contract_version" not in payload
                    )
                    summary = (
                        f"rc=0, wire={args.expected}, tables=array:{isinstance(tables, list)}, "
                        f"dashboards=array:{isinstance(dashboards, list)}, documents=array:{isinstance(documents, list)}"
                    )
                else:
                    error_payload = object_json(result.stderr) or object_json(result.stdout) or {}
                    error = error_payload.get("error") if isinstance(error_payload.get("error"), dict) else error_payload
                    subtype = error.get("subtype") or error.get("reason") if isinstance(error, dict) else None
                    live_ok = result.returncode != 0 and subtype == "projection_unknown"
                    summary = f"rc=nonzero, subtype={subtype or 'missing'}, fail_closed={'yes' if live_ok else 'no'}"
                checks.append(("真实 Base 目录投影或未知形状 fail-closed", live_ok, summary))
                if not live_ok:
                    findings.append("live base-get neither projected the reviewed shape nor failed closed")
            except (AttributeError, IndexError, TypeError, ValueError, subprocess.TimeoutExpired) as error:
                findings.append(f"live base-get structural probe failed: {type(error).__name__}")
        else:
            checks.append(("真实 Base 目录投影或未知形状 fail-closed", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# aitable +base-get {args.expected} Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 当前源码临时构建；Base 名称、baseId 与原始 JSON 只在内存中用于结构审阅。本文件仅保存脱敏字段类型与计数，不接入 CI / policy。",
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
        "- Base 目录只发布已取证的稳定资源 ID/名称；请求 ID 不一致、重复 ID、字段漂移与未取证的非空 documents 均失败关闭。",
        "- `inventoryCoverageKnown:false` 明确表示这份响应不能扩大为用户所有 Base 或所有业务资源的权威清单。",
        "- dual 阶段只影子构建统一结果，外部仍是旧业务 JSON；Agent 不传协议选择参数。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
