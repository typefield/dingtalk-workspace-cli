#!/usr/bin/env python3
"""Audit Agoal user-rules discovery and per-terminal output rollout."""

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


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--live", action="store_true")
    parser.add_argument(
        "--phase", choices=("surface", "dual", "active"), default="surface",
        help="expected public result: surface/dual keep legacy bytes; active uses the unified envelope",
    )
    args = parser.parse_args()
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    focused = run([
        "go", "test", "-count=1", "./internal/helpers", "./internal/cli",
        "-run", "TestAgoalUserRules|TestReviewedRuntimeSchemaExclusions|TestSchemaCatalogDeliveryCompleteness",
    ], env, 900)
    ok = focused.returncode == 0
    checks.append(("Contract/Safety/Schema completeness 回归", ok, f"rc={focused.returncode}"))
    if not ok:
        findings.append("focused Agoal admission tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-agoal-rules-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env, 900)
        build_ok = build.returncode == 0
        checks.append(("临时构建当前源码", build_ok, f"rc={build.returncode}"))
        if not build_ok:
            findings.append("current source build failed")
        else:
            help_result = run([str(binary), "agoal", "user", "rules", "--help"], env)
            help_ok = help_result.returncode == 0 and "--user-id" in help_result.stdout and "--request-id" in help_result.stdout
            checks.append(("无业务参数 Help 发现", help_ok, f"rc={help_result.returncode}, canonical_flags={'yes' if help_ok else 'no'}"))
            if not help_ok:
                findings.append("Agoal user rules Help is not discoverable")

            schema_result = run([str(binary), "schema", "--cli-path", "agoal user rules", "--format", "json"], env)
            schema = parse_object(schema_result.stdout) or {}
            params = schema.get("parameters") if isinstance(schema.get("parameters"), dict) else {}
            schema_ok = (
                schema_result.returncode == 0
                and schema.get("canonical_path") == "agoal.user_rules"
                and schema.get("effect") == "read"
                and schema.get("risk") == "low"
                and schema.get("confirmation") == "not_required"
                and schema.get("idempotency") == "idempotent"
                and set(params) == {"user-id", "request-id"}
                and params.get("user-id", {}).get("property") == "dingUserId"
                and params.get("request-id", {}).get("property") == "requestId"
            )
            if args.phase != "surface":
                result_spec = schema.get("result") if isinstance(schema.get("result"), dict) else {}
                outcomes = result_spec.get("outcomes") if isinstance(result_spec.get("outcomes"), list) else []
                data_schema = result_spec.get("data_schema") if isinstance(result_spec.get("data_schema"), dict) else {}
                required = data_schema.get("required") if isinstance(data_schema.get("required"), list) else []
                schema_ok = schema_ok and outcomes == ["success", "failure"] and set(required) == {
                    "rules", "preference", "ruleCoverageKnown",
                }
            checks.append(("Runtime Schema 从 exclusion 进入公开面", schema_ok, f"rc={schema_result.returncode}, parameters={len(params)}"))
            if not schema_ok:
                findings.append("Agoal user rules Runtime Schema is incomplete")

            all_schema_result = run([str(binary), "schema", "--all", "--format", "json"], env)
            all_schema = parse_object(all_schema_result.stdout) or {}
            products = all_schema.get("products") if isinstance(all_schema.get("products"), list) else []
            agoal_products = [product for product in products if isinstance(product, dict) and product.get("id") == "agoal"]
            agoal_tools = agoal_products[0].get("tools") if len(agoal_products) == 1 and isinstance(agoal_products[0].get("tools"), list) else []
            gradual_ok = (
                all_schema_result.returncode == 0
                and len(agoal_products) == 1
                and len(agoal_tools) == 1
                and isinstance(agoal_tools[0], dict)
                and (agoal_tools[0].get("canonical_path") == "agoal.user_rules" or (agoal_tools[0].get("identity") or {}).get("canonical_path") == "agoal.user_rules")
            )
            checks.append(("Agoal 按叶渐进公开而非整域放开", gradual_ok, f"public_agoal_tools={len(agoal_tools)}"))
            if not gradual_ok:
                findings.append("Agoal admission was not limited to the reviewed leaf")

            if args.live:
                live = run([str(binary), "agoal", "user", "rules", "--format", "json"], env, 300)
                payload = parse_object(live.stdout)
                if args.phase == "active":
                    data = payload.get("data") if isinstance(payload, dict) and isinstance(payload.get("data"), dict) else {}
                    meta = payload.get("meta") if isinstance(payload, dict) and isinstance(payload.get("meta"), dict) else {}
                    rules = data.get("rules") if isinstance(data.get("rules"), list) else None
                    stable = bool(rules is not None and all(
                        isinstance(row, dict)
                        and isinstance(row.get("ruleId"), str) and row.get("ruleId", "").strip()
                        and isinstance(row.get("periods"), dict)
                        and all(
                            isinstance(period, dict)
                            and isinstance(period.get("periodId"), str)
                            and period.get("periodId", "").strip()
                            for key in ("current", "history")
                            for period in row["periods"].get(key, [])
                        )
                        for row in rules
                    ))
                    live_ok = (
                        live.returncode == 0
                        and isinstance(payload, dict)
                        and set(payload).issubset({"ok", "outcome", "data", "meta", "dry_run", "_notice"})
                        and "contract_version" not in payload
                        and payload.get("ok") is True
                        and payload.get("outcome") == "success"
                        and data.get("ruleCoverageKnown") is False
                        and meta.get("count") == len(rules or [])
                        and "pagination" not in meta
                        and stable
                        and not live.stderr.strip()
                    )
                else:
                    content = payload.get("content") if isinstance(payload, dict) and isinstance(payload.get("content"), dict) else {}
                    rules = content.get("rules") if isinstance(content.get("rules"), list) else None
                    stable = bool(rules is not None and all(isinstance(row, dict) and isinstance(row.get("id"), str) and row.get("id", "").strip() for row in rules))
                    live_ok = (
                        live.returncode == 0
                        and isinstance(payload, dict)
                        and payload.get("success") is True
                        and "ok" not in payload
                        and stable
                        and not live.stderr.strip()
                    )
                label = "当前用户真实只读统一投影" if args.phase == "active" else "当前用户真实只读 legacy 字节"
                checks.append((label, live_ok, f"rc={live.returncode}, rules={len(rules) if rules is not None else 'unknown'}, stable_ids={'yes' if stable else 'no'}"))
                if not live_ok:
                    findings.append(f"live Agoal user rules did not match the reviewed {args.phase} shape")
            else:
                checks.append(("当前用户真实只读规则发现", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# Agoal user rules Agent {args.phase} 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 从当前源码临时构建并审阅 Help、Runtime Schema 与可选真实只读响应；用户 ID、规则 ID、周期 ID、名称和原始 JSON 仅在内存中处理，不写入本报告，也不接入 CI / policy。",
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
        "- `agoal user rules` 是 Agoal 整域 exclusion 中首个逐叶完成 Contract、read/low Safety、参数映射和真实只读取证的命令；其余 Agoal 叶仍保持 exclusion，不批量放开。",
        "- 这次只证明当前用户规则响应可被读取并含稳定规则 ID，不证明 Agoal 全域权限、规则覆盖或目标完成情况。",
    ]
    if args.phase == "surface":
        lines.append("- 当前业务输出仍为 legacy JSON；本次关闭的是 Agent 发现面 exclusion，不把它误写成统一输出迁移已经完成。")
    elif args.phase == "dual":
        lines.append("- 当前命令处于 dual_validate：同一次业务调用会严格构造并验证统一结果，但外部 stdout 仍为 legacy JSON；Agent 不选择协议版本。")
    else:
        lines.append("- 当前命令处于 unified_active：普通 `--format json` 直接得到 `ok/outcome/data/meta`，不含协议选择参数或版本标记；`ruleCoverageKnown:false` 且不伪造分页终态。")
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
