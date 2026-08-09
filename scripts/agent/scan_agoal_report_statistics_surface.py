#!/usr/bin/env python3
"""Audit Agoal report-statistics discovery and per-terminal rollout."""

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
    return subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True, check=False, timeout=timeout)


def parse_object(text: str) -> dict[str, Any] | None:
    try:
        value = json.loads(text)
    except json.JSONDecodeError:
        return None
    return value if isinstance(value, dict) else None


def canonical_path(tool: dict[str, Any]) -> str | None:
    identity = tool.get("identity") if isinstance(tool.get("identity"), dict) else {}
    value = tool.get("canonical_path") or identity.get("canonical_path")
    return value if isinstance(value, str) else None


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--live", action="store_true")
    parser.add_argument("--phase", choices=("dual", "active"), default="dual")
    args = parser.parse_args()
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    focused = run([
        "go", "test", "-count=1", "./internal/helpers", "./internal/cli",
        "-run", "TestAgoalReportStatistics|TestReviewedRuntimeSchemaExclusions|TestSchemaCatalogDeliveryCompleteness",
    ], env, 900)
    focused_ok = focused.returncode == 0
    checks.append(("Contract/Safety/投影与 exclusion 回归", focused_ok, f"rc={focused.returncode}"))
    if not focused_ok:
        findings.append("focused Agoal report-statistics tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-agoal-report-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env, 900)
        build_ok = build.returncode == 0
        checks.append(("临时构建当前源码", build_ok, f"rc={build.returncode}"))
        if not build_ok:
            findings.append("current source build failed")
        else:
            help_result = run([str(binary), "agoal", "report", "list-statistics", "--help"], env)
            help_ok = help_result.returncode == 0 and "--keyword" in help_result.stdout and "--request-id" in help_result.stdout
            checks.append(("无业务参数 Help 发现", help_ok, f"rc={help_result.returncode}, canonical_flags={'yes' if help_ok else 'no'}"))
            if not help_ok:
                findings.append("Agoal report-statistics Help is not discoverable")

            schema_result = run([str(binary), "schema", "--cli-path", "agoal report list-statistics", "--format", "json"], env)
            schema = parse_object(schema_result.stdout) or {}
            params = schema.get("parameters") if isinstance(schema.get("parameters"), dict) else {}
            result_spec = schema.get("result") if isinstance(schema.get("result"), dict) else {}
            data_schema = result_spec.get("data_schema") if isinstance(result_spec.get("data_schema"), dict) else {}
            required = data_schema.get("required") if isinstance(data_schema.get("required"), list) else []
            schema_ok = (
                schema_result.returncode == 0
                and schema.get("canonical_path") == "agoal.report_list_statistics"
                and schema.get("effect") == "read"
                and schema.get("risk") == "low"
                and schema.get("confirmation") == "not_required"
                and schema.get("idempotency") == "idempotent"
                and set(params) == {"keyword", "request-id"}
                and params.get("keyword", {}).get("property") == "keyword"
                and params.get("request-id", {}).get("property") == "requestId"
                and result_spec.get("outcomes") == ["success", "failure"]
                and set(required) == {"reports", "reportCoverageKnown"}
            )
            checks.append(("Runtime Schema 从精确 exclusion 进入公开面", schema_ok, f"rc={schema_result.returncode}, parameters={len(params)}"))
            if not schema_ok:
                findings.append("Agoal report-statistics Runtime Schema is incomplete")

            all_result = run([str(binary), "schema", "--all", "--format", "json"], env)
            all_schema = parse_object(all_result.stdout) or {}
            products = all_schema.get("products") if isinstance(all_schema.get("products"), list) else []
            agoal_products = [product for product in products if isinstance(product, dict) and product.get("id") == "agoal"]
            tools = agoal_products[0].get("tools") if len(agoal_products) == 1 and isinstance(agoal_products[0].get("tools"), list) else []
            paths = {canonical_path(tool) for tool in tools if isinstance(tool, dict)}
            gradual_ok = all_result.returncode == 0 and paths == {"agoal.user_rules", "agoal.report_list_statistics"}
            checks.append(("Agoal 仍按叶渐进公开", gradual_ok, f"public_agoal_tools={len(tools)}"))
            if not gradual_ok:
                findings.append("Agoal admission was not limited to the two reviewed leaves")

            if args.live:
                live = run([str(binary), "agoal", "report", "list-statistics", "--format", "json"], env)
                payload = parse_object(live.stdout)
                if args.phase == "active":
                    data = payload.get("data") if isinstance(payload, dict) and isinstance(payload.get("data"), dict) else {}
                    meta = payload.get("meta") if isinstance(payload, dict) and isinstance(payload.get("meta"), dict) else {}
                    reports = data.get("reports") if isinstance(data.get("reports"), list) else None
                    stable = reports is not None and all(
                        isinstance(row, dict)
                        and isinstance(row.get("templateId"), str) and bool(row["templateId"].strip())
                        and all(isinstance(row.get(key), int) and not isinstance(row.get(key), bool) for key in ("onTime", "late", "notSubmitted", "remindSize"))
                        and row["remindSize"] == row["onTime"] + row["late"] + row["notSubmitted"]
                        for row in reports
                    )
                    live_ok = (
                        live.returncode == 0
                        and isinstance(payload, dict)
                        and set(payload).issubset({"ok", "outcome", "data", "meta", "dry_run", "_notice"})
                        and "contract_version" not in payload
                        and payload.get("ok") is True
                        and payload.get("outcome") == "success"
                        and data.get("reportCoverageKnown") is False
                        and meta.get("count") == len(reports or [])
                        and "pagination" not in meta
                        and stable
                        and not live.stderr.strip()
                    )
                else:
                    reports = payload.get("content") if isinstance(payload, dict) and isinstance(payload.get("content"), list) else None
                    stable = reports is not None and all(
                        isinstance(row, dict) and isinstance(row.get("templateId"), str) and bool(row["templateId"].strip())
                        for row in reports
                    )
                    live_ok = (
                        live.returncode == 0
                        and isinstance(payload, dict)
                        and payload.get("success") is True
                        and "ok" not in payload
                        and stable
                        and not live.stderr.strip()
                    )
                label = "真实只读统一统计投影" if args.phase == "active" else "真实只读 dual legacy 输出"
                checks.append((label, live_ok, f"rc={live.returncode}, reports={len(reports) if reports is not None else 'unknown'}, stable_ids={'yes' if stable else 'no'}"))
                if not live_ok:
                    findings.append(f"live Agoal report-statistics did not match the reviewed {args.phase} shape")
            else:
                checks.append(("真实只读统计响应", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# Agoal report list-statistics Agent {args.phase} 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 从当前源码临时构建并审阅 Help、Runtime Schema 与可选真实只读响应；模板 ID、标题、修改人、正文和原始 JSON 只在内存中处理，不写入本报告，也不接入 CI / policy。",
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
        "- 只发布后续 submit-detail 所需的稳定 templateId、规则摘要、三类统计计数与视图权限；HTML 正文、修改人标识和未审阅配置不进入 Agent 输出。",
        "- 上游没有提供总目录或分页终态证据，因此统一结果固定保留 `reportCoverageKnown:false`，且不生成 `meta.pagination`。",
        "- `agoal user objectives` 当前真实账号只观察到空数组，缺少非空行投影证据，继续保持精确 exclusion；没有为了追求 exclusion=0 而猜测返回形态。",
    ]
    if args.phase == "dual":
        lines.append("- 当前命令处于 dual_validate：同一次业务调用构造并校验统一结果，但外部 stdout 仍为 legacy JSON；Agent 不选择协议版本。")
    else:
        lines.append("- 当前命令处于 unified_active：普通 `--format json` 直接得到 `ok/outcome/data/meta`，不含协议选择参数或版本标记。")
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
