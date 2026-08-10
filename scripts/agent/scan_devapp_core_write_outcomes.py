#!/usr/bin/env python3
"""Agent semantic scan for the first DevApp shortcut write rollout.

The scan builds the current CLI in a temporary directory and executes only
zero-write previews or pre-business validation/confirmation failures. Normal
write results are exercised through the injected Go caller seam, so no real
DevApp is created, updated, enabled or disabled. It writes Markdown evidence
only; it is not a CI gate and stores no JSON fixture.
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
DECLARATIONS = ROOT / "internal" / "shortcut" / "devapp" / "devapp.go"
MAPPER = ROOT / "internal" / "helpers" / "devapp_mutation_result.go"
TESTS = ROOT / "internal" / "shortcut" / "devapp" / "devapp_unified_mutation_test.go"


def source_checks() -> list[tuple[str, str, str]]:
    declarations = DECLARATIONS.read_text(encoding="utf-8")
    mapper = MAPPER.read_text(encoding="utf-8")
    tests = TESTS.read_text(encoding="utf-8")
    init_block = declarations[declarations.index("func init()") : declarations.index("func frameworkUnified")]
    active = all(
        f"frameworkUnified({name})" in init_block
        for name in ("CreateApp", "UpdateApp", "EnableApp", "DisableApp")
    )
    delete_dual = "frameworkDualValidate(DeleteApp)" in init_block
    return [
        (
            "四条核心写 shortcut 独立 active",
            "PASS" if active else "FAIL",
            "create/update/enable/disable 由发布声明选择统一结果；Agent 只传 --format json。",
        ),
        (
            "delete 未越过二次确认差距",
            "PASS" if delete_dual else "FAIL",
            "delete 继续 dual_validate，待补齐原子入口已有的 confirm-name 防误删后再晋级。",
        ),
        (
            "API success 不伪装写后终态",
            "PASS"
            if '"state":  "not_verified"' in mapper
            and "read-after-write terminal-state check" in mapper
            else "FAIL",
            "成功 data 显式携带 verification.state=not_verified；非对象响应 fail-closed。",
        ),
        (
            "单次业务调用与投影矩阵",
            "PASS"
            if "TestDevAppCoreMutationShortcutsEmitOneHonestUnifiedResult" in tests
            and "caller.calls != 1" in tests
            and "TestDevAppMutationMapperRejectsNonObjectSuccessAsUnknownEffect" in tests
            else "FAIL",
            "受控 caller 对四条写命令验证一次调用、统一 success、未知投影与无版本标记。",
        ),
    ]


def run_go_probe() -> tuple[str, str]:
    env = os.environ.copy()
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-scan")
    completed = subprocess.run(
        [
            "go", "test", "-count=1", "./internal/shortcut/devapp", "./internal/helpers",
            "-run", r"DevApp(CoreMutation|Mutation|UpdateRejects|SharedResult)",
        ],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=240,
    )
    if completed.returncode == 0:
        return "PASS", "受控 caller/mapping 测试通过；写工具恰好调用一次。"
    detail = (completed.stderr or completed.stdout).strip().replace("\n", " ")[:600]
    return "FAIL", f"rc={completed.returncode}; {detail}"


def invoke(binary: Path, env: dict[str, str], args: list[str]) -> tuple[int, Any, str]:
    completed = subprocess.run(
        [str(binary), *args, "--format", "json"],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=90,
    )
    try:
        payload = json.loads(completed.stdout) if completed.stdout.strip() else None
    except json.JSONDecodeError:
        payload = None
    return completed.returncode, payload, completed.stderr.strip()


def is_preview(payload: Any, tool: str) -> bool:
    return (
        isinstance(payload, dict)
        and payload.get("ok") is True
        and payload.get("outcome") == "success"
        and payload.get("dry_run") is True
        and "contract_version" not in payload
        and isinstance(payload.get("data"), dict)
        and payload["data"].get("executed") is False
        and payload["data"].get("tool") == tool
    )


def run_public_probe() -> tuple[str, str]:
    with tempfile.TemporaryDirectory(prefix="dws-devapp-write-agent-") as temp:
        temp_path = Path(temp)
        binary = temp_path / "dws"
        env = os.environ.copy()
        env["DWS_CONFIG_DIR"] = str(temp_path / "config")
        env["DWS_PACKAGE_VERSION"] = "0.0.0-agent-scan"
        built = subprocess.run(
            ["go", "build", "-o", str(binary), "./cmd"],
            cwd=ROOT,
            env=env,
            capture_output=True,
            text=True,
            timeout=300,
        )
        if built.returncode != 0:
            return "FAIL", f"build rc={built.returncode}: {(built.stderr or built.stdout)[:500]}"

        cases = [
            ("create", ["devapp", "+create", "--name", "Demo", "--dry-run"], "create_dev_app"),
            ("update", ["devapp", "+update", "--unified-app-id", "app-1", "--name", "Renamed", "--dry-run"], "update_dev_app"),
            ("enable", ["devapp", "+enable", "--unified-app-id", "app-1", "--dry-run"], "enable_dev_app"),
            ("disable", ["devapp", "+disable", "--unified-app-id", "app-1", "--dry-run"], "disable_dev_app"),
            ("update-empty", ["devapp", "+update", "--unified-app-id", "app-1", "--yes"], ""),
            ("create-gate", ["devapp", "+create", "--name", "Demo"], ""),
        ]
        with ThreadPoolExecutor(max_workers=len(cases)) as pool:
            results = list(pool.map(lambda case: invoke(binary, env, case[1]), cases))

        for (name, _, tool), (rc, payload, stderr) in zip(cases[:4], results[:4]):
            if rc != 0 or stderr or not is_preview(payload, tool):
                return "FAIL", f"{name}: rc={rc}, stderr={stderr[:120]!r}, payload={payload!r}"
        empty_rc, empty_payload, _ = results[4]
        if not (
            empty_rc == 3
            and isinstance(empty_payload, dict)
            and empty_payload.get("ok") is False
            and empty_payload.get("outcome") == "failure"
            and isinstance(empty_payload.get("error"), dict)
            and empty_payload["error"].get("type") == "validation"
            and "contract_version" not in empty_payload
        ):
            return "FAIL", f"update-empty: rc={empty_rc}, payload={empty_payload!r}"
        gate_rc, gate_payload, _ = results[5]
        if not (
            gate_rc == 3
            and isinstance(gate_payload, dict)
            and isinstance(gate_payload.get("error"), dict)
            and gate_payload["error"].get("subtype") == "confirmation_required"
            and "contract_version" not in gate_payload
        ):
            return "FAIL", f"create-gate: rc={gate_rc}, payload={gate_payload!r}"
        return "PASS", "4 条零写 preview、update 零字段 validation、create 确认门禁均为单一统一信封。"


def render() -> str:
    checks = source_checks()
    fixture_status, fixture_detail = run_go_probe()
    checks.append(("受控成功/未知/调用次数矩阵", fixture_status, fixture_detail))
    public_status, public_detail = run_public_probe()
    checks.append(("公开 CLI 零写与门禁边界", public_status, public_detail))
    passed = sum(status == "PASS" for _, status, _ in checks)
    lines = [
        "# DevApp 核心写 shortcut 统一结果 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> Agent 在当前工作树执行扫描；只运行零写入 dry-run、确认/校验前拦截和受控 caller。报告不保存 JSON fixture，不调用真实写接口，也不接入 CI。",
        "",
        "| 检查 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    for name, status, detail in checks:
        lines.append(f"| {name} | {status} | {detail} |")
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。四条核心写 shortcut 现直接使用统一结果；请求成功只声明 `verification.state=not_verified`，不把 API success 扩大为应用状态已生效。",
        "",
        "未验证：真实租户创建、更新、启停后的应用终态，以及响应丢失时服务端是否已经执行。`devapp +delete` 继续保持 dual，必须先补齐 `--confirm-name` 二次防误删语义。",
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
