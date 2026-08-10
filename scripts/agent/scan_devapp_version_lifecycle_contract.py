#!/usr/bin/env python3
"""Agent review for DevApp version lifecycle truth and gradual rollout.

The scan only executes dry-run previews and controlled Go callers. It never
creates or publishes a real version, stores no JSON fixture, and is not a CI
gate.
"""

from __future__ import annotations

import argparse
from concurrent.futures import ThreadPoolExecutor
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SHORTCUTS = ROOT / "internal" / "shortcut" / "devapp" / "devapp.go"
NATIVE = ROOT / "internal" / "helpers" / "devapp.go"
MAPPER = ROOT / "internal" / "helpers" / "devapp_version_result.go"
HELPER_TESTS = ROOT / "internal" / "helpers" / "devapp_version_result_test.go"
SHORTCUT_TESTS = ROOT / "internal" / "shortcut" / "devapp" / "devapp_version_result_test.go"


def run(command: list[str], *, env: dict[str, str] | None = None, timeout: int = 240) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, capture_output=True, text=True, timeout=timeout)


def source_checks() -> list[tuple[str, str, str]]:
    shortcuts = SHORTCUTS.read_text(encoding="utf-8")
    native = NATIVE.read_text(encoding="utf-8")
    mapper = MAPPER.read_text(encoding="utf-8")
    helper_tests = HELPER_TESTS.read_text(encoding="utf-8")
    shortcut_tests = SHORTCUT_TESTS.read_text(encoding="utf-8")
    init_block = shortcuts[shortcuts.index("func init()") : shortcuts.index("func frameworkUnified")]
    return [
        (
            "两条版本写 Shortcut 保持逐命令 dual_validate",
            "PASS" if all(f"frameworkDualValidate({name})" in init_block for name in ("VersionCreate", "VersionPublish")) else "FAIL",
            "没有用户选择协议；真实写响应与回读证据齐全前，外部仍保持 legacy、内部 shadow 校验统一结果。",
        ),
        (
            "两套入口发布同源版本结果契约",
            "PASS"
            if shortcuts.count("helpers.DevAppVersionCreateResultSpec()") == 1
            and shortcuts.count("helpers.DevAppVersionPublishResultSpec()") == 1
            and shortcuts.count("helpers.DevAppVersionPrecheckResultSpec()") == 1
            and shortcuts.count("helpers.DevAppVersionStatusResultSpec()") == 1
            and native.count("DevAppVersionCreateResultSpec()") == 1
            and native.count("DevAppVersionPublishResultSpec()") == 1
            and native.count("DevAppVersionPrecheckResultSpec()") == 1
            and native.count("DevAppVersionStatusResultSpec()") == 1
            else "FAIL",
            "create/check-approval/publish/status 的 outcome 与 data schema 在 dev、devapp 两入口同源。",
        ),
        (
            "创建版本要求稳定 versionId 且不伪造回读",
            "PASS"
            if "did not return a stable versionId" in mapper
            and '"operation":    "create_version"' in mapper
            and '"state":        "not_verified"' in mapper
            else "FAIL",
            "只有稳定 versionId 才能成功；请求事实在 requested，下游继续使用明确的 status 回读命令。",
        ),
        (
            "发布区分终态、审批 pending 与未知效果",
            "PASS"
            if "approvalSubmitted" in mapper
            and "returned no terminal publish evidence" in mapper
            and 'versionStatus == "RELEASE"' in mapper
            and "DevAppVersionPublishResultSpec" in mapper
            else "FAIL",
            "审批提交/审核中为 pending；只有显式 RELEASE/GRAY/published 才进入 success，且仍标 not_verified。",
        ),
        (
            "冲突状态 fail-closed 并保留恢复路径",
            "PASS"
            if "devAppStatusValues" in native
            and 'case "FAIL", "FAILED", "EXPIRED"' in native
            and "firstDevAppParams" in native
            and "process failure wins" in helper_tests
            else "FAIL",
            "processStatus 失败优先于笼统 SUCCESS；pending 可从原请求补齐 versionId/unifiedAppId 和 next_command。",
        ),
        (
            "版本写 Safety 与 guard-first 对齐",
            "PASS"
            if shortcuts.count('Confirmation: "user_required", Idempotency: "unknown"') >= 10
            and shortcut_tests.count("ConfirmFirst") >= 1
            and "TestDevAppVersionShortcutContractsRemainDualUntilRealWriteEvidence" in shortcut_tests
            else "FAIL",
            "create/publish 均 write/high、user_required、idempotency unknown，确认发生在业务参数与调用之前。",
        ),
    ]


def go_probe() -> tuple[str, str]:
    completed = run([
        "go", "test", "-count=1", "./internal/helpers", "./internal/shortcut/devapp", "./internal/app",
        "-run", r"DevApp(Version|EnvelopeRegression|FamilyRead|AllLeavesStdout|SharedResult|WriteGuardRequiresFinalSchema)",
    ])
    if completed.returncode == 0:
        return "PASS", "受控创建/发布对象、审批 pending、冲突状态、未知 ACK、一次调用与最终 Schema 测试通过。"
    detail = (completed.stderr or completed.stdout).strip().replace("\n", " ")[:600]
    return "FAIL", f"rc={completed.returncode}; {detail}"


def decode(completed: subprocess.CompletedProcess[str]) -> Any:
    try:
        return json.loads(completed.stdout)
    except json.JSONDecodeError:
        return None


def public_dry_run_probe() -> tuple[str, str]:
    with tempfile.TemporaryDirectory(prefix="dws-devapp-version-agent-") as temp:
        base = Path(temp)
        binary = base / "dws"
        env = os.environ.copy()
        env["DWS_CONFIG_DIR"] = str(base / "config")
        env["DWS_PACKAGE_VERSION"] = "0.0.0-agent-scan"
        built = run(["go", "build", "-o", str(binary), "./cmd"], env=env, timeout=300)
        if built.returncode != 0:
            return "FAIL", f"build rc={built.returncode}: {(built.stderr or built.stdout)[:500]}"

        cases = [
            ("native/create", [str(binary), "dev", "app", "version", "create", "--unified-app-id", "app-1", "--desc", "release", "--dry-run", "--format", "json"]),
            ("native/publish", [str(binary), "dev", "app", "version", "publish", "--unified-app-id", "app-1", "--version-id", "version-1", "--dry-run", "--format", "json"]),
            ("shortcut/create", [str(binary), "devapp", "+version-create", "--unified-app-id", "app-1", "--desc", "release", "--dry-run", "--format", "json"]),
            ("shortcut/publish", [str(binary), "devapp", "+version-publish", "--unified-app-id", "app-1", "--version-id", "version-1", "--dry-run", "--format", "json"]),
        ]
        with ThreadPoolExecutor(max_workers=4) as pool:
            results = list(pool.map(lambda item: run(item[1], env=env, timeout=90), cases))
        for (name, _), completed in zip(cases, results):
            payload = decode(completed)
            if completed.returncode != 0 or completed.stderr.strip() or not isinstance(payload, dict):
                return "FAIL", f"{name}: rc={completed.returncode}, stderr={completed.stderr[:120]!r}"
            if "contract_version" in payload:
                return "FAIL", f"{name}: removed version marker leaked"
            if name.startswith("native/"):
                if not (payload.get("ok") is True and payload.get("outcome") == "success" and payload.get("dry_run") is True):
                    return "FAIL", f"{name}: payload={payload!r}"
            elif not (payload.get("dry_run") is True and payload.get("executed") is False):
                return "FAIL", f"{name}: legacy payload={payload!r}"

        schema_wants = {
            "dev app version create": ("write", ["success", "pending", "partial_failure", "failure"]),
            "dev app version check-approval": ("read", ["success", "failure"]),
            "dev app version publish": ("write", ["success", "pending", "partial_failure", "failure"]),
            "dev app version status": ("read", ["success", "pending", "failure"]),
            "devapp +version-create": ("write", ["success", "pending", "partial_failure", "failure"]),
            "devapp +version-check-approval": ("read", ["success", "failure"]),
            "devapp +version-publish": ("write", ["success", "pending", "partial_failure", "failure"]),
            "devapp +version-status": ("read", ["success", "pending", "failure"]),
        }
        for cli_path, (effect, outcomes) in schema_wants.items():
            completed = run([str(binary), "schema", "--cli-path", cli_path, "--format", "json"], env=env, timeout=90)
            payload = decode(completed)
            result = payload.get("result") if isinstance(payload, dict) else None
            if not (
                completed.returncode == 0
                and isinstance(payload, dict)
                and payload.get("effect") == effect
                and payload.get("confirmation") == ("user_required" if effect == "write" else "not_required")
                and isinstance(result, dict)
                and result.get("outcomes") == outcomes
            ):
                return "FAIL", f"schema {cli_path}: rc={completed.returncode}, payload={payload!r}"
        return "PASS", "四条 dry-run 均零写：native 为统一信封、dual Shortcut 保持 legacy；八条 live Schema 的 Safety/ResultSpec 同源且无版本标记。"


def render() -> str:
    checks = source_checks()
    status, detail = go_probe()
    checks.append(("受控版本状态与 Schema 矩阵", status, detail))
    status, detail = public_dry_run_probe()
    checks.append(("公开 CLI 渐进输出与零写边界", status, detail))
    passed = sum(status == "PASS" for _, status, _ in checks)
    lines = [
        "# DevApp 版本生命周期结果与渐进 rollout Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本报告由 Agent 在当前工作树执行；仅使用 dry-run 和受控 caller，不创建/发布真实版本、不保存 JSON fixture，也不接入 CI。",
        "",
        "| 检查 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {name} | {status} | {detail} |" for name, status, detail in checks)
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。版本创建不再接受无 `versionId` 的模糊 ACK；发布明确区分审批 pending、响应声称终态和未知效果。",
        "",
        "边界：当前没有隔离应用的真实创建/发布与回读证据。两条写 Shortcut 继续 `dual_validate` 且保持 exclusion；只有真实响应、审批链和 `version status/get` 回读闭环后才允许逐条晋级或公开。",
    ]
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown evidence; default stdout")
    args = parser.parse_args()
    report = render()
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report, end="")
    return 1 if "| FAIL |" in report else 0


if __name__ == "__main__":
    raise SystemExit(main())
