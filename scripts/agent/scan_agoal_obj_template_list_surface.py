#!/usr/bin/env python3
"""Audit Agoal obj-template list discovery, pagination, and rollout."""

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


def stable_templates(rows: Any) -> bool:
    return isinstance(rows, list) and all(
        isinstance(row, dict)
        and isinstance(row.get("templateId"), str)
        and bool(row["templateId"].strip())
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
        "-run", "TestAgoalObjTemplateList|TestReviewedRuntimeSchemaExclusions|TestSchemaCatalogDeliveryCompleteness",
    ], env, 900)
    focused_ok = focused.returncode == 0
    checks.append(("Contract/Safety/分页投影与 exclusion 回归", focused_ok, f"rc={focused.returncode}"))
    if not focused_ok:
        findings.append("focused Agoal obj-template list tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-agoal-template-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env, 900)
        build_ok = build.returncode == 0
        checks.append(("临时构建当前源码", build_ok, f"rc={build.returncode}"))
        if not build_ok:
            findings.append("current source build failed")
        else:
            help_result = run([str(binary), "agoal", "obj-template", "list", "--help"], env)
            help_ok = help_result.returncode == 0 and all(flag in help_result.stdout for flag in ("--keyword", "--page", "--page-size", "--request-id"))
            checks.append(("无业务参数 Help 发现", help_ok, f"rc={help_result.returncode}, canonical_flags={'yes' if help_ok else 'no'}"))
            if not help_ok:
                findings.append("Agoal obj-template list Help is not discoverable")

            schema_result = run([str(binary), "schema", "--cli-path", "agoal obj-template list", "--format", "json"], env)
            schema = parse_object(schema_result.stdout) or {}
            params = schema.get("parameters") if isinstance(schema.get("parameters"), dict) else {}
            result_spec = schema.get("result") if isinstance(schema.get("result"), dict) else {}
            data_schema = result_spec.get("data_schema") if isinstance(result_spec.get("data_schema"), dict) else {}
            required = data_schema.get("required") if isinstance(data_schema.get("required"), list) else []
            schema_ok = (
                schema_result.returncode == 0
                and schema.get("canonical_path") == "agoal.obj_template_list"
                and schema.get("effect") == "read"
                and schema.get("risk") == "low"
                and schema.get("confirmation") == "not_required"
                and schema.get("idempotency") == "idempotent"
                and set(params) == {"keyword", "page", "page-size", "request-id"}
                and params.get("page-size", {}).get("property") == "pageSize"
                and result_spec.get("outcomes") == ["success", "failure"]
                and set(required) == {"templates", "totalCount", "authoritativeInventory", "inventoryCoverageKnown"}
            )
            checks.append(("Runtime Schema 从精确 exclusion 进入公开面", schema_ok, f"rc={schema_result.returncode}, parameters={len(params)}"))
            if not schema_ok:
                findings.append("Agoal obj-template list Runtime Schema is incomplete")

            all_result = run([str(binary), "schema", "--all", "--format", "json"], env)
            all_schema = parse_object(all_result.stdout) or {}
            products = all_schema.get("products") if isinstance(all_schema.get("products"), list) else []
            agoal_products = [product for product in products if isinstance(product, dict) and product.get("id") == "agoal"]
            tools = agoal_products[0].get("tools") if len(agoal_products) == 1 and isinstance(agoal_products[0].get("tools"), list) else []
            paths = {tool_path(tool) for tool in tools if isinstance(tool, dict)}
            expected = {"agoal.user_rules", "agoal.report_list_statistics", "agoal.obj_template_list"}
            gradual_ok = all_result.returncode == 0 and paths == expected
            checks.append(("Agoal 仍按叶渐进公开", gradual_ok, f"public_agoal_tools={len(tools)}"))
            if not gradual_ok:
                findings.append("Agoal admission was not limited to the three reviewed leaves")

            if args.live:
                pages: list[dict[str, Any]] = []
                live_ok = True
                for page_number in (1, 2):
                    live = run([str(binary), "agoal", "obj-template", "list", "--page", str(page_number), "--page-size", "20", "--format", "json"], env)
                    payload = parse_object(live.stdout)
                    page_fact: dict[str, Any] = {"rc": live.returncode, "stderr_empty": not live.stderr.strip()}
                    if args.phase == "active":
                        data = payload.get("data") if isinstance(payload, dict) and isinstance(payload.get("data"), dict) else {}
                        meta = payload.get("meta") if isinstance(payload, dict) and isinstance(payload.get("meta"), dict) else {}
                        pagination = meta.get("pagination") if isinstance(meta.get("pagination"), dict) else {}
                        rows = data.get("templates")
                        page_fact.update({"rows": rows, "data": data, "meta": meta, "pagination": pagination, "payload": payload})
                    else:
                        content = payload.get("content") if isinstance(payload, dict) and isinstance(payload.get("content"), dict) else {}
                        rows = content.get("result")
                        legacy_stable = isinstance(rows, list) and all(isinstance(row, dict) and isinstance(row.get("id"), str) and bool(row["id"].strip()) for row in rows)
                        page_fact.update({"rows": rows, "content": content, "payload": payload, "legacy_stable": legacy_stable})
                    pages.append(page_fact)

                if args.phase == "active":
                    first, second = pages
                    first_data, second_data = first["data"], second["data"]
                    first_meta, second_meta = first["meta"], second["meta"]
                    first_page, second_page = first["pagination"], second["pagination"]
                    live_ok = (
                        all(page["rc"] == 0 and page["stderr_empty"] and stable_templates(page["rows"]) for page in pages)
                        and all(isinstance(page["payload"], dict) and "contract_version" not in page["payload"] and page["payload"].get("ok") is True and page["payload"].get("outcome") == "success" for page in pages)
                        and len(first["rows"]) == 20 and len(second["rows"]) == 15
                        and first_data.get("totalCount") == 35 and second_data.get("totalCount") == 35
                        and first_data.get("authoritativeInventory") is False and first_data.get("inventoryCoverageKnown") is False
                        and first_meta.get("count") == 20 and second_meta.get("count") == 15
                        and first_page.get("endpoint_exhausted") is False and first_page.get("next_token") == "2"
                        and second_page.get("endpoint_exhausted") is True and "next_token" not in second_page
                    )
                else:
                    first, second = pages
                    first_content, second_content = first["content"], second["content"]
                    live_ok = (
                        all(page["rc"] == 0 and page["stderr_empty"] and page["legacy_stable"] for page in pages)
                        and all(isinstance(page["payload"], dict) and page["payload"].get("success") is True and "ok" not in page["payload"] for page in pages)
                        and len(first["rows"]) == 20 and len(second["rows"]) == 15
                        and first_content.get("page") == 1 and second_content.get("page") == 2
                        and first_content.get("pageSize") == 20 and second_content.get("pageSize") == 20
                        and first_content.get("totalCount") == 35 and second_content.get("totalCount") == 35
                    )
                label = "真实两页统一分页投影" if args.phase == "active" else "真实两页 dual legacy 输出"
                checks.append((label, live_ok, "pages=2, items=20+15, total=35, stable_ids=yes" if live_ok else "shape mismatch"))
                if not live_ok:
                    findings.append(f"live Agoal obj-template list did not match the reviewed {args.phase} shape")
            else:
                checks.append(("真实两页模板响应", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        f"# Agoal obj-template list Agent {args.phase} 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 从当前源码临时构建并审阅 Help、Runtime Schema 与可选真实两页响应；模板 ID、标题、创建人、维度内容和原始 JSON 只在内存中处理，不写入本报告，也不接入 CI / policy。",
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
        "- 结果只发布稳定 templateId、标题、类型、状态和三个权重开关；creator 与完整 dimensions 不进入 Agent 摘要投影。",
        "- 服务端 page/pageSize/totalCount 与当前页条数必须严格对账；非末页返回 `endpoint_exhausted:false + next_token`，末页才返回 `endpoint_exhausted:true`。",
        "- `authoritativeInventory:false` 与 `inventoryCoverageKnown:false` 防止把当前身份/关键词下的分页 endpoint 扩大成企业全部模板目录。",
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
