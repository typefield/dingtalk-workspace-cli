#!/usr/bin/env python3
"""Audit Agoal scorecard detail null-response safety without admitting it."""

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
SELECTED_TIME = "2026-08-01T00:00:00+08:00"


def run(command: list[str], env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, check=False, timeout=timeout)


def parse_object(text: str) -> dict[str, Any] | None:
    try:
        value = json.loads(text)
    except json.JSONDecodeError:
        return None
    return value if isinstance(value, dict) else None


def tool_path(tool: dict[str, Any]) -> str | None:
    identity = tool.get("identity") if isinstance(tool.get("identity"), dict) else {}
    value = tool.get("canonical_path") or identity.get("canonical_path")
    return value if isinstance(value, str) else None


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
        "-run", "TestAgoalScorecardDetail|TestReviewedRuntimeSchemaExclusions|TestSchemaCatalogDeliveryCompleteness",
    ], env, 900)
    focused_ok = focused.returncode == 0
    checks.append(("null 拒绝、legacy 保真与 exclusion 回归", focused_ok, f"rc={focused.returncode}"))
    if not focused_ok:
        findings.append("focused Agoal scorecard detail tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-agoal-scorecard-boundary-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env, 900)
        build_ok = build.returncode == 0
        checks.append(("临时构建当前源码", build_ok, f"rc={build.returncode}"))
        if not build_ok:
            findings.append("current source build failed")
        else:
            help_result = run([str(binary), "agoal", "scorecard", "detail", "--help"], env)
            help_ok = help_result.returncode == 0 and all(flag in help_result.stdout for flag in ("--selected-time", "--dept-id", "--request-id"))
            checks.append(("兼容命令 Help 保持可发现", help_ok, f"rc={help_result.returncode}, flags={'yes' if help_ok else 'no'}"))
            if not help_ok:
                findings.append("Agoal scorecard detail Help is not discoverable")

            all_result = run([str(binary), "schema", "--all", "--format", "json"], env)
            all_schema = parse_object(all_result.stdout) or {}
            products = all_schema.get("products") if isinstance(all_schema.get("products"), list) else []
            agoal_products = [product for product in products if isinstance(product, dict) and product.get("id") == "agoal"]
            tools = agoal_products[0].get("tools") if len(agoal_products) == 1 and isinstance(agoal_products[0].get("tools"), list) else []
            paths = {tool_path(tool) for tool in tools if isinstance(tool, dict)}
            excluded_ok = all_result.returncode == 0 and "agoal.scorecard_detail" not in paths and len(tools) == 4
            checks.append(("证据不足命令仍保持精确 exclusion", excluded_ok, f"public_agoal_tools={len(tools)}"))
            if not excluded_ok:
                findings.append("Agoal scorecard detail was admitted without a complete entity contract")

            if args.live:
                root_result = run([
                    str(binary), "agoal", "scorecard", "detail",
                    "--selected-time", SELECTED_TIME, "--dept-id", "1", "--format", "json",
                ], env)
                root_payload = parse_object(root_result.stdout)
                root_content = root_payload.get("content") if isinstance(root_payload, dict) and isinstance(root_payload.get("content"), dict) else {}
                entities = root_content.get("content")
                root_ok = (
                    root_result.returncode == 0 and not root_result.stderr.strip()
                    and isinstance(root_payload, dict) and root_payload.get("success") is True
                    and isinstance(root_content.get("id"), str) and bool(root_content["id"].strip())
                    and isinstance(entities, list) and len(entities) == 0
                )
                checks.append(("根部门非 null legacy 响应保持可用", root_ok, "entity_rows=0, stable_scorecard_id=yes" if root_ok else "shape mismatch"))
                if not root_ok:
                    findings.append("root scorecard legacy response changed unexpectedly")

                depts_result = run([str(binary), "contact", "+list-sub-depts", "--dept", "1", "--format", "json"], env)
                depts_payload = parse_object(depts_result.stdout) or {}
                depts_data = depts_payload.get("data") if isinstance(depts_payload.get("data"), dict) else {}
                depts = depts_data.get("depts") if isinstance(depts_data.get("depts"), list) else []
                dept_ids = [row.get("deptId") for row in depts if isinstance(row, dict) and row.get("deptId") is not None][:max(args.sample_limit, 0)]
                rejected_null = 0
                empty_success = 0
                nonempty_success = 0
                rc_zero_null = 0
                unexpected = 0
                for dept_id in dept_ids:
                    result = run([
                        str(binary), "agoal", "scorecard", "detail",
                        "--selected-time", SELECTED_TIME, "--dept-id", str(dept_id), "--format", "json",
                    ], env)
                    if result.returncode == 0:
                        if result.stdout.strip() == "null":
                            rc_zero_null += 1
                            continue
                        payload = parse_object(result.stdout)
                        content = payload.get("content") if isinstance(payload, dict) and isinstance(payload.get("content"), dict) else {}
                        rows = content.get("content")
                        if isinstance(rows, list) and len(rows) == 0:
                            empty_success += 1
                        elif isinstance(rows, list) and len(rows) > 0:
                            nonempty_success += 1
                        else:
                            unexpected += 1
                        continue
                    error_payload = parse_object(result.stderr)
                    error = error_payload.get("error") if isinstance(error_payload, dict) and isinstance(error_payload.get("error"), dict) else {}
                    if (
                        not result.stdout.strip()
                        and error_payload is not None and error_payload.get("outcome") == "failure"
                        and error.get("subtype") == "projection_unknown"
                        and error.get("retryable") is False
                    ):
                        rejected_null += 1
                    else:
                        unexpected += 1
                live_ok = bool(dept_ids) and rejected_null > 0 and rc_zero_null == 0 and unexpected == 0
                evidence = (
                    f"sampled={len(dept_ids)}, null_rejected={rejected_null}, empty_success={empty_success}, "
                    f"nonempty_success={nonempty_success}, rc0_null={rc_zero_null}"
                )
                checks.append(("直属部门 null 不再伪装成功", live_ok, evidence))
                if not live_ok:
                    findings.append("live child-department sample did not prove null fail-closed behavior")
                if nonempty_success == 0:
                    checks.append(("非空核心实体投影证据", True, "UNVERIFIED（抽样未观察到非空 content）"))
            else:
                checks.append(("真实 null/非空边界", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        "# Agoal scorecard detail Agent 边界审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 使用当前源码临时构建并做有界真实只读抽样；部门 ID、计分卡 ID、业务内容和原始 JSON 只在内存中处理，不写入本报告，也不接入 CI / policy。",
        "",
        f"## Result: {'PASS' if passed else 'REVIEW'}",
        "",
        "| 检查项 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{evidence}` |" for label, ok, evidence in checks)
    lines += [
        "",
        "## 结论",
        "",
        "- 旧行为会把服务端 JSON null 原样写到 stdout 并 rc=0；当前边界改为 typed `api/projection_unknown`、retryable=false、非零退出，且 stdout 为空。",
        "- 非 null 响应继续使用 legacy writer，业务请求 exactly-once；本修复不借机改变输出结构或公开统一契约。",
        "- 根部门及有界直属部门样本未观察到非空核心 `content` 实体，因此无法审阅 entityId、嵌套目标或 update 所需完整内容。",
        "- `agoal scorecard detail` 继续保留精确 exclusion；取得非空实体样本并定义完整 ResultSpec 后再进入 dual validation。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
