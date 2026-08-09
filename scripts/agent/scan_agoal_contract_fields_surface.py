#!/usr/bin/env python3
"""Audit Agoal contract fields discovery, projection, and gradual rollout."""

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


def tool_path(tool: dict[str, Any]) -> str | None:
    identity = tool.get("identity") if isinstance(tool.get("identity"), dict) else {}
    value = tool.get("canonical_path") or identity.get("canonical_path")
    return value if isinstance(value, str) else None


def stable_projected_fields(rows: Any) -> bool:
    required = {
        "fieldId", "code", "title", "category", "type",
        "active", "required", "forceActive", "forceRequired",
    }
    return isinstance(rows, list) and len(rows) > 0 and all(
        isinstance(row, dict)
        and set(row) == required
        and all(isinstance(row.get(key), str) and bool(row[key].strip()) for key in ("fieldId", "code", "title", "category", "type"))
        and all(isinstance(row.get(key), bool) for key in ("active", "required", "forceActive", "forceRequired"))
        for row in rows
    )


def stable_legacy_fields(rows: Any) -> bool:
    return isinstance(rows, list) and len(rows) > 0 and all(
        isinstance(row, dict)
        and isinstance(row.get("id"), str) and bool(row["id"].strip())
        and isinstance(row.get("code"), str) and bool(row["code"].strip())
        and isinstance(row.get("title"), str) and bool(row["title"].strip())
        for row in rows
    )


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
        "-run", "TestAgoalContractFields|TestReviewedRuntimeSchemaExclusions|TestSchemaCatalogDeliveryCompleteness",
    ], env, 900)
    focused_ok = focused.returncode == 0
    checks.append(("Contract/Safety/严格投影与 exclusion 回归", focused_ok, f"rc={focused.returncode}"))
    if not focused_ok:
        findings.append("focused Agoal contract fields tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-agoal-contract-fields-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env, 900)
        build_ok = build.returncode == 0
        checks.append(("临时构建当前源码", build_ok, f"rc={build.returncode}"))
        if not build_ok:
            findings.append("current source build failed")
        else:
            help_result = run([str(binary), "agoal", "contract", "fields", "--help"], env)
            help_ok = help_result.returncode == 0 and "--request-id" in help_result.stdout
            checks.append(("无业务参数 Help 发现", help_ok, f"rc={help_result.returncode}, request_id={'yes' if help_ok else 'no'}"))
            if not help_ok:
                findings.append("Agoal contract fields Help is not discoverable")

            schema_result = run([str(binary), "schema", "--cli-path", "agoal contract fields", "--format", "json"], env)
            schema = parse_object(schema_result.stdout) or {}
            params = schema.get("parameters") if isinstance(schema.get("parameters"), dict) else {}
            result_spec = schema.get("result") if isinstance(schema.get("result"), dict) else {}
            data_schema = result_spec.get("data_schema") if isinstance(result_spec.get("data_schema"), dict) else {}
            required = data_schema.get("required") if isinstance(data_schema.get("required"), list) else []
            schema_ok = (
                schema_result.returncode == 0
                and schema.get("canonical_path") == "agoal.contract_fields"
                and schema.get("effect") == "read"
                and schema.get("risk") == "low"
                and schema.get("confirmation") == "not_required"
                and schema.get("idempotency") == "idempotent"
                and set(params) == {"request-id"}
                and params.get("request-id", {}).get("property") == "requestId"
                and result_spec.get("outcomes") == ["success", "failure"]
                and set(required) == {"fields", "fieldCoverageKnown"}
            )
            checks.append(("Runtime Schema 从精确 exclusion 进入公开面", schema_ok, f"rc={schema_result.returncode}, parameters={len(params)}"))
            if not schema_ok:
                findings.append("Agoal contract fields Runtime Schema is incomplete")

            all_result = run([str(binary), "schema", "--all", "--format", "json"], env)
            all_schema = parse_object(all_result.stdout) or {}
            products = all_schema.get("products") if isinstance(all_schema.get("products"), list) else []
            agoal_products = [product for product in products if isinstance(product, dict) and product.get("id") == "agoal"]
            tools = agoal_products[0].get("tools") if len(agoal_products) == 1 and isinstance(agoal_products[0].get("tools"), list) else []
            paths = {tool_path(tool) for tool in tools if isinstance(tool, dict)}
            expected = {"agoal.user_rules", "agoal.report_list_statistics", "agoal.obj_template_list", "agoal.contract_fields"}
            gradual_ok = all_result.returncode == 0 and paths == expected
            checks.append(("Agoal 仍按叶渐进公开", gradual_ok, f"public_agoal_tools={len(tools)}"))
            if not gradual_ok:
                findings.append("Agoal admission was not limited to the four reviewed leaves")

            if args.live:
                live = run([str(binary), "agoal", "contract", "fields", "--format", "json"], env)
                payload = parse_object(live.stdout)
                stderr_empty = not live.stderr.strip()
                if args.phase == "active":
                    data = payload.get("data") if isinstance(payload, dict) and isinstance(payload.get("data"), dict) else {}
                    meta = payload.get("meta") if isinstance(payload, dict) and isinstance(payload.get("meta"), dict) else {}
                    rows = data.get("fields")
                    ids = [row.get("fieldId") for row in rows] if isinstance(rows, list) and all(isinstance(row, dict) for row in rows) else []
                    codes = [row.get("code") for row in rows] if isinstance(rows, list) and all(isinstance(row, dict) for row in rows) else []
                    live_ok = (
                        live.returncode == 0 and stderr_empty
                        and isinstance(payload, dict) and payload.get("ok") is True and payload.get("outcome") == "success"
                        and "contract_version" not in payload
                        and stable_projected_fields(rows)
                        and len(set(ids)) == len(ids) and len(set(codes)) == len(codes)
                        and data.get("fieldCoverageKnown") is False
                        and meta.get("count") == len(rows)
                        and "pagination" not in meta
                    )
                else:
                    rows = payload.get("content") if isinstance(payload, dict) else None
                    ids = [row.get("id") for row in rows] if isinstance(rows, list) and all(isinstance(row, dict) for row in rows) else []
                    codes = [row.get("code") for row in rows] if isinstance(rows, list) and all(isinstance(row, dict) for row in rows) else []
                    live_ok = (
                        live.returncode == 0 and stderr_empty
                        and isinstance(payload, dict) and payload.get("success") is True and "ok" not in payload
                        and stable_legacy_fields(rows)
                        and len(set(ids)) == len(ids) and len(set(codes)) == len(codes)
                    )
                count = len(rows) if isinstance(rows, list) else 0
                label = "真实统一字段摘要" if args.phase == "active" else "真实 dual legacy 字段响应"
                checks.append((label, live_ok, f"fields={count}, unique_ids=yes, unique_codes=yes" if live_ok else "shape mismatch"))
                if not live_ok:
                    findings.append(f"live Agoal contract fields did not match the reviewed {args.phase} shape")
            else:
                checks.append(("真实非空字段响应", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# Agoal contract fields Agent {args.phase} 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 从当前源码临时构建并审阅 Help、Runtime Schema 与可选真实响应；字段 ID、code、标题和值只在内存中处理，不写入本报告，也不接入 CI / policy。",
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
        "- 结果只发布稳定 fieldId、code、标题、类别、类型和四个激活/必填布尔值；presentation-only scheme 与当前恒空 source 不进入 Agent 摘要。",
        "- 字段 ID/code 重复、未知字段、字符串布尔、非整数布局宽度或 source 形状漂移均 fail-closed 为不可重试的 projection_unknown。",
        "- `fieldCoverageKnown:false` 表示服务端没有给出分页或权威目录覆盖事实；空数组不能扩大为组织没有经营合约字段。",
    ]
    if args.phase == "dual":
        lines.append("- 当前处于 dual_validate：同一次业务调用校验统一投影，但 stdout 仍为 legacy JSON，Agent 不选择协议版本。")
    else:
        lines.append("- 当前处于 unified_active：普通 `--format json` 直接得到 `ok/outcome/data/meta`，不含协议选择参数或版本标记。")
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
