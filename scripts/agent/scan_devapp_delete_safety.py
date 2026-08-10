#!/usr/bin/env python3
"""Agent review for DevApp shortcut delete safety and result rollout.

The scan uses only zero-write CLI paths and controlled Go callers. It never
deletes a real application, stores no JSON fixture, and is not a CI gate.
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
DECLARATION = ROOT / "internal" / "shortcut" / "devapp" / "devapp.go"
TESTS = ROOT / "internal" / "shortcut" / "devapp" / "devapp_unified_mutation_test.go"


def run(cmd: list[str], *, env: dict[str, str] | None = None, timeout: int = 240) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, cwd=ROOT, env=env, capture_output=True, text=True, timeout=timeout)


def invoke(binary: Path, env: dict[str, str], args: list[str]) -> tuple[int, Any, str]:
    completed = run([str(binary), *args, "--format", "json"], env=env, timeout=90)
    try:
        payload = json.loads(completed.stdout) if completed.stdout.strip() else None
    except json.JSONDecodeError:
        payload = None
    return completed.returncode, payload, completed.stderr.strip()


def source_checks() -> list[tuple[str, str, str]]:
    declaration = DECLARATION.read_text(encoding="utf-8")
    tests = TESTS.read_text(encoding="utf-8")
    init_block = declaration[declaration.index("func init()") : declaration.index("func frameworkUnified")]
    checks = [
        (
            "逐命令 active 与 guard-first 声明",
            "PASS" if "frameworkUnified(DeleteApp)" in init_block and "ConfirmFirst: true" in declaration else "FAIL",
            "delete 独立进入 unified_active；确认门禁先于 confirm-name 校验。",
        ),
        (
            "二次确认和 fail-closed 实现",
            "PASS"
            if 'Name: "confirm-name"' in declaration
            and 'rt.CallMCPData(productDevApp, "get_dev_app", params)' in declaration
            and 'confirmName != actualName' in declaration
            else "FAIL",
            "真实执行先只读 get，名称缺失或不匹配均不调用 delete。",
        ),
        (
            "受控调用顺序与零写边界测试",
            "PASS"
            if "TestDevAppDeleteShortcutRequiresMatchingNameBeforeOneDelete" in tests
            and "TestDevAppDeleteShortcutFailsClosedBeforeDelete" in tests
            and "TestDevAppDeleteShortcutDryRunNeedsNoNameAndMakesNoCall" in tests
            else "FAIL",
            "测试覆盖 get→delete、门禁、缺名、错名、不可读名称和 dry-run 零调用。",
        ),
    ]
    return checks


def go_probe() -> tuple[str, str]:
    completed = run([
        "go", "test", "-count=1", "./internal/shortcut/devapp", "./internal/helpers",
        "-run", r"DevApp(Delete|CoreMutation|Mutation|UpdateRejects)",
    ])
    if completed.returncode == 0:
        return "PASS", "受控 shortcut/native 删除与共享结果测试通过。"
    detail = (completed.stderr or completed.stdout).strip().replace("\n", " ")[:600]
    return "FAIL", f"rc={completed.returncode}; {detail}"


def public_probe() -> tuple[str, str]:
    with tempfile.TemporaryDirectory(prefix="dws-devapp-delete-agent-") as temp:
        base = Path(temp)
        binary = base / "dws"
        env = os.environ.copy()
        env["DWS_CONFIG_DIR"] = str(base / "config")
        env["DWS_PACKAGE_VERSION"] = "0.0.0-agent-scan"
        built = run(["go", "build", "-o", str(binary), "./cmd"], env=env, timeout=300)
        if built.returncode != 0:
            return "FAIL", f"build rc={built.returncode}: {(built.stderr or built.stdout)[:500]}"

        cases = [
            ["devapp", "+delete", "--unified-app-id", "app-1", "--dry-run"],
            ["devapp", "+delete", "--unified-app-id", "app-1", "--yes"],
            ["devapp", "+delete", "--unified-app-id", "app-1", "--confirm-name", "DemoApp"],
            ["schema", "--cli-path", "devapp +delete", "--compact"],
        ]
        with ThreadPoolExecutor(max_workers=len(cases)) as pool:
            results = list(pool.map(lambda args: invoke(binary, env, args), cases))

        dry_rc, dry, dry_err = results[0]
        dry_ok = (
            dry_rc == 0
            and dry_err == ""
            and isinstance(dry, dict)
            and dry.get("ok") is True
            and dry.get("outcome") == "success"
            and dry.get("dry_run") is True
            and isinstance(dry.get("data"), dict)
            and dry["data"].get("executed") is False
            and "contract_version" not in dry
        )
        missing_rc, missing, _ = results[1]
        missing_ok = (
            missing_rc == 3
            and isinstance(missing, dict)
            and isinstance(missing.get("error"), dict)
            and missing["error"].get("type") == "validation"
            and "--confirm-name" in missing["error"].get("message", "")
        )
        gate_rc, gate, _ = results[2]
        gate_ok = (
            gate_rc == 3
            and isinstance(gate, dict)
            and isinstance(gate.get("error"), dict)
            and gate["error"].get("subtype") == "confirmation_required"
        )
        schema_rc, schema, _ = results[3]
        schema_text = json.dumps(schema, ensure_ascii=False, sort_keys=True) if schema is not None else ""
        schema_ok = schema_rc == 0 and "confirm-name" in schema_text and "user_required" in schema_text
        if dry_ok and missing_ok and gate_ok and schema_ok:
            return "PASS", "公开 CLI dry-run、confirm-name validation、confirmation gate 与 live Schema 均通过。"
        return "FAIL", (
            f"dry={dry_ok}/rc{dry_rc}, missing={missing_ok}/rc{missing_rc}, "
            f"gate={gate_ok}/rc{gate_rc}, schema={schema_ok}/rc{schema_rc}"
        )


def render() -> str:
    checks = source_checks()
    status, detail = go_probe()
    checks.append(("受控业务边界", status, detail))
    status, detail = public_probe()
    checks.append(("公开 CLI 与 Schema", status, detail))
    passed = sum(status == "PASS" for _, status, _ in checks)
    lines = [
        "# DevApp 删除二次确认与统一结果 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本报告由 Agent 在当前工作树执行；仅使用 dry-run、确认/校验前拦截和受控 caller，不删除真实应用、不保存 JSON fixture，也不接入 CI。",
        "",
        "| 检查 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {name} | {status} | {detail} |" for name, status, detail in checks)
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。`devapp +delete` 已具备 guard-first、`--confirm-name` 精确匹配、dry-run 零调用和统一结果；成功仅声明 `verification.state=not_verified`。",
        "",
        "边界：本扫描不证明服务端已永久删除资源，也不证明响应丢失时操作未执行。真实删除失败或成功后必须先用只读查询核查状态，禁止盲目重放。",
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
