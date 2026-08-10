#!/usr/bin/env python3
"""Agent review for DevApp member mutation safety and gradual output rollout.

The scan only executes dry-run previews and controlled Go callers. It never
adds or removes a real member, stores no JSON fixture, and is not a CI gate.
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
MAPPER = ROOT / "internal" / "helpers" / "devapp_mutation_result.go"
TESTS = ROOT / "internal" / "shortcut" / "devapp" / "devapp_unified_mutation_test.go"


def run(command: list[str], *, env: dict[str, str] | None = None, timeout: int = 240) -> subprocess.CompletedProcess[str]:
    return subprocess.run(command, cwd=ROOT, env=env, capture_output=True, text=True, timeout=timeout)


def source_checks() -> list[tuple[str, str, str]]:
    shortcuts = SHORTCUTS.read_text(encoding="utf-8")
    native = NATIVE.read_text(encoding="utf-8")
    mapper = MAPPER.read_text(encoding="utf-8")
    tests = TESTS.read_text(encoding="utf-8")
    init_block = shortcuts[shortcuts.index("func init()") : shortcuts.index("func frameworkUnified")]
    return [
        (
            "两条旧 Shortcut 保持逐命令 dual_validate",
            "PASS" if all(f"frameworkDualValidate({name})" in init_block for name in ("MemberAdd", "MemberRemove")) else "FAIL",
            "没有用户选择协议；真实 helper 响应取证前仍输出 legacy，shadow 构造统一结果。",
        ),
        (
            "两套入口共用成员写结果契约",
            "PASS"
            if shortcuts.count("Result:      helpers.DevAppMutationResultSpec()") >= 7
            and native.count("Result:      DevAppMutationResultSpec()") >= 7
            else "FAIL",
            "dev 与 devapp 均声明 success/pending/partial/failure 的同源 ResultSpec。",
        ),
        (
            "请求事实不冒充逐成员成功",
            "PASS"
            if 'projected["requested"] = requested' in mapper
            and "not per-user success claims" in mapper
            and '"state":  "not_verified"' in mapper
            else "FAIL",
            "成功保留上游对象，同时把 userIds 放在 requested 下并标 verification:not_verified。",
        ),
        (
            "投影未知 fail-closed",
            "PASS"
            if "lost the requested member identifiers" in mapper
            and "returned no object result" in mapper
            and "ExecutionStarted: &started" in mapper
            else "FAIL",
            "非对象响应或请求身份丢失均为 projection_unknown，且禁止盲目重试。",
        ),
        (
            "安全等级两入口对齐",
            "PASS"
            if 'Safety:  devAppSafetyDestructive()' in native
            and shortcuts.count('Effect: "destructive", Risk: "high"') >= 2
            and "TestDevAppMemberMutationContractsRemainDualUntilLiveResponseEvidence" in tests
            else "FAIL",
            "添加为 write/high；移除为 destructive/high；两者都 user_required、idempotency unknown。",
        ),
    ]


def go_probe() -> tuple[str, str]:
    completed = run([
        "go", "test", "-count=1", "./internal/helpers", "./internal/shortcut/devapp", "./internal/app",
        "-run", r"DevApp(Member|CoreMutation|MutationMapper|WriteGuardRequiresFinalSchema)",
    ])
    if completed.returncode == 0:
        return "PASS", "受控对象/非对象响应、一次调用、请求投影和最终 Schema Safety 测试通过。"
    detail = (completed.stderr or completed.stdout).strip().replace("\n", " ")[:600]
    return "FAIL", f"rc={completed.returncode}; {detail}"


def decode(completed: subprocess.CompletedProcess[str]) -> Any:
    try:
        return json.loads(completed.stdout)
    except json.JSONDecodeError:
        return None


def public_dry_run_probe() -> tuple[str, str]:
    with tempfile.TemporaryDirectory(prefix="dws-devapp-member-agent-") as temp:
        base = Path(temp)
        binary = base / "dws"
        env = os.environ.copy()
        env["DWS_CONFIG_DIR"] = str(base / "config")
        env["DWS_PACKAGE_VERSION"] = "0.0.0-agent-scan"
        built = run(["go", "build", "-o", str(binary), "./cmd"], env=env, timeout=300)
        if built.returncode != 0:
            return "FAIL", f"build rc={built.returncode}: {(built.stderr or built.stdout)[:500]}"

        cases = []
        for verb in ("add", "remove"):
            cases.append(("native", verb, [
                str(binary), "dev", "app", "member", verb,
                "--unified-app-id", "app-1", "--user-ids", "u1,u2",
                "--member-type", "DEVELOPER", "--dry-run", "--format", "json",
            ]))
            cases.append(("shortcut", verb, [
                str(binary), "devapp", f"+member-{verb}",
                "--unified-app-id", "app-1", "--user-ids", "u1,u2",
                "--member-type", "DEVELOPER", "--dry-run", "--format", "json",
            ]))
        with ThreadPoolExecutor(max_workers=4) as pool:
            results = list(pool.map(lambda item: run(item[2], env=env, timeout=90), cases))

        for (surface, verb, _), completed in zip(cases, results):
            payload = decode(completed)
            if completed.returncode != 0 or completed.stderr.strip() or not isinstance(payload, dict):
                return "FAIL", f"{surface}/{verb}: rc={completed.returncode}, stderr={completed.stderr[:120]!r}"
            if "contract_version" in payload:
                return "FAIL", f"{surface}/{verb}: removed version marker leaked"
            if surface == "native":
                if not (payload.get("ok") is True and payload.get("outcome") == "success" and payload.get("dry_run") is True):
                    return "FAIL", f"native/{verb}: payload={payload!r}"
            elif not (payload.get("dry_run") is True and payload.get("executed") is False and payload.get("tool") == f"{verb}_dev_app_members"):
                return "FAIL", f"shortcut/{verb}: legacy payload={payload!r}"

        schema_wants = {
            "dev app member add": ("write", "high"),
            "dev app member remove": ("destructive", "high"),
            "devapp +member-add": ("write", "high"),
            "devapp +member-remove": ("destructive", "high"),
        }
        for cli_path, (effect, risk) in schema_wants.items():
            completed = run(
                [str(binary), "schema", "--cli-path", cli_path, "--format", "json"],
                env=env,
                timeout=90,
            )
            payload = decode(completed)
            result = payload.get("result") if isinstance(payload, dict) else None
            if not (
                completed.returncode == 0
                and isinstance(payload, dict)
                and payload.get("effect") == effect
                and payload.get("risk") == risk
                and payload.get("confirmation") == "user_required"
                and isinstance(result, dict)
                and result.get("outcomes") == ["success", "pending", "partial_failure", "failure"]
                and "requested" in (result.get("data_schema") or {}).get("properties", {})
            ):
                return "FAIL", f"schema {cli_path}: rc={completed.returncode}, payload={payload!r}"
        return "PASS", "四条公开 dry-run 均零写：native 为统一信封，dual Shortcut 逐字保留 legacy JSON；live Schema 的 Safety/ResultSpec 同源且无版本标记。"


def render() -> str:
    checks = source_checks()
    status, detail = go_probe()
    checks.append(("受控结果与 Safety 矩阵", status, detail))
    status, detail = public_dry_run_probe()
    checks.append(("公开 CLI 渐进输出与零写边界", status, detail))
    passed = sum(status == "PASS" for _, status, _ in checks)
    lines = [
        "# DevApp 成员写入结果与渐进 rollout Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本报告由 Agent 在当前工作树执行；仅使用 dry-run 和受控 caller，不添加/移除真实成员、不保存 JSON fixture，也不接入 CI。",
        "",
        "| 检查 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {name} | {status} | {detail} |" for name, status, detail in checks)
    lines += [
        "",
        f"结论：**{passed}/{len(checks)} PASS**。两套入口已共用诚实的成员写结果模型；请求列表只作为 `requested` 事实，不能当作逐成员已生效证明。",
        "",
        "边界：官方仓库和本地评测证据都没有 dingtalk-dev helper 的真实成功响应样本，因此两个既有 Shortcut 继续 `dual_validate`。取得脱敏真实对象响应并完成成员列表回读前，不晋级 active、不宣称逐成员终态。",
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
