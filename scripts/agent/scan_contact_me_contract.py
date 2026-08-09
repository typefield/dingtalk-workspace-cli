#!/usr/bin/env python3
"""Agent audit for the contact +me result contract.

The scan builds the current source in a temporary directory, checks the live
Runtime Schema and focused Go tests, and optionally performs one read-only
profile call. Runtime JSON is parsed only in memory; the Markdown evidence
contains field names and booleans, never profile values or a JSON fixture.
This is an Agent review tool, not a CI/policy gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
EXPECTED_DATA_KEYS = {"userId", "name", "mobile", "email", "org", "dept"}


def run(command: list[str], *, env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        command,
        cwd=ROOT,
        env=env,
        text=True,
        capture_output=True,
        timeout=timeout,
        check=False,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence path")
    parser.add_argument("--live", action="store_true", help="perform one authenticated read-only contact +me call")
    args = parser.parse_args()

    findings: list[str] = []
    checks: list[tuple[str, bool, str]] = []
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")

    focused = run(
        [
            "go",
            "test",
            "-count=1",
            "./internal/shortcut/smart",
            "-run",
            "TestWhoami(ProjectionRequiresStableCurrentUser|UnifiedResultAndDeclaration)",
        ],
        env=env,
    )
    focused_ok = focused.returncode == 0
    checks.append(("焦点投影与统一结果测试", focused_ok, f"rc={focused.returncode}"))
    if not focused_ok:
        findings.append("focused whoami tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-contact-me-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env=env)
        build_ok = build.returncode == 0
        checks.append(("临时构建当前源码", build_ok, f"rc={build.returncode}"))
        if not build_ok:
            findings.append("current source build failed")
        else:
            help_result = run([str(binary), "contact", "+me", "--help"], env=env)
            help_ok = (
                help_result.returncode == 0
                and "--format" in help_result.stdout
                and "--output-contract" not in help_result.stdout
            )
            checks.append(("Help 只暴露统一 format 入口", help_ok, f"rc={help_result.returncode}"))
            if not help_ok:
                findings.append("contact +me Help does not expose the expected single format surface")

            schema_result = run(
                [str(binary), "schema", "--cli-path", "contact +me", "--format", "json"],
                env=env,
            )
            schema_ok = False
            schema_summary = f"rc={schema_result.returncode}"
            try:
                payload = json.loads(schema_result.stdout)
                result = payload.get("result") or {}
                data_schema = result.get("data_schema") or {}
                properties = set((data_schema.get("properties") or {}).keys())
                required = data_schema.get("required") or []
                sensitive = result.get("sensitive_paths") or []
                schema_ok = (
                    schema_result.returncode == 0
                    and properties == EXPECTED_DATA_KEYS
                    and required == ["userId"]
                    and sensitive == ["email", "mobile"]
                    and "pagination" not in result
                    and "ndjson" not in result
                )
                schema_summary = (
                    f"properties={len(properties)}, required=userId, "
                    f"sensitive=email/mobile, rc={schema_result.returncode}"
                )
            except (json.JSONDecodeError, AttributeError, TypeError):
                pass
            checks.append(("Runtime Schema 与 active data 对齐", schema_ok, schema_summary))
            if not schema_ok:
                findings.append("contact +me Runtime Schema does not match the reviewed projection")

            if args.live:
                live = run([str(binary), "contact", "+me", "--format", "json"], env=env, timeout=90)
                live_ok = False
                live_summary = f"rc={live.returncode}"
                try:
                    payload = json.loads(live.stdout)
                    data = payload.get("data") if isinstance(payload, dict) else None
                    live_ok = (
                        live.returncode == 0
                        and not live.stderr.strip()
                        and isinstance(payload, dict)
                        and set(payload) <= {"ok", "outcome", "data", "meta", "dry_run"}
                        and payload.get("ok") is True
                        and payload.get("outcome") == "success"
                        and "contract_version" not in payload
                        and isinstance(data, dict)
                        and isinstance(data.get("userId"), str)
                        and bool(data["userId"].strip())
                        and set(data) <= EXPECTED_DATA_KEYS
                    )
                    live_summary = (
                        f"rc={live.returncode}, one JSON success, data_fields={len(data) if isinstance(data, dict) else 0}, "
                        "stable_user_id=yes, stderr=empty"
                    )
                except (json.JSONDecodeError, AttributeError, TypeError, KeyError):
                    pass
                checks.append(("真实只读 contact +me 结构对拍", live_ok, live_summary))
                if not live_ok:
                    findings.append("authenticated read-only contact +me contract mismatch")
            else:
                checks.append(("真实只读 contact +me 结构对拍", True, "SKIPPED（未传 --live）"))

    passed = not findings
    lines = [
        "# contact +me 统一结果与投影 Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 当前源码临时构建；JSON 只在内存解析。本文件不保存姓名、手机号、邮箱、组织、部门、userId 或原始响应，也不接入 CI / policy。",
        "",
        f"## Result: {'PASS' if passed else 'REVIEW'}",
        "",
        "| 检查项 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    for label, ok, summary in checks:
        lines.append(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{summary}` |")
    lines += [
        "",
        "## 结论",
        "",
        "- 普通 `--format json` 直接返回 `ok/outcome/data`，没有协议版本标记或第二个选择参数。",
        "- `data.userId` 是成功结果的必需稳定句柄；空数组、多记录、display-only 或未知响应均 fail-closed 为 `api/projection_unknown`。",
        "- 手机号和邮箱在 ResultSpec 中声明为敏感路径；Schema 不承诺分页或 NDJSON。",
        "- 真实读取只证明当前身份和本次 endpoint 响应可投影，不证明组织目录覆盖、权限边界或资料长期完整。",
    ]
    if findings:
        lines += ["", "## Findings", ""]
        lines.extend(f"- {finding}" for finding in findings)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
