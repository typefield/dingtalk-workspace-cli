#!/usr/bin/env python3
"""Review Agoal objectives/submit-detail inputs and bounded live shapes.

Identity and business values stay in memory.  The checked-in artifact contains
only counts, field names, and machine-contract verdicts; no response JSON is
persisted and this scanner is not a CI gate.
"""

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
    return subprocess.run(
        command, cwd=ROOT, env=env, text=True, capture_output=True,
        check=False, timeout=timeout,
    )


def parse_json(text: str) -> Any:
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return None


def current_user_id(binary: Path, env: dict[str, str]) -> str | None:
    result = run([str(binary), "profile", "list", "--format", "json"], env)
    payload = parse_json(result.stdout)
    if result.returncode != 0 or not isinstance(payload, dict):
        return None
    current = payload.get("currentProfile")
    profiles = payload.get("profiles")
    if not isinstance(current, str) or not isinstance(profiles, list):
        return None
    matches = [
        row.get("userId") for row in profiles
        if isinstance(row, dict) and row.get("profile") == current
    ]
    return matches[0] if len(matches) == 1 and isinstance(matches[0], str) and matches[0].strip() else None


def active_rules(binary: Path, env: dict[str, str], user_id: str) -> tuple[str | None, list[str]]:
    result = run([
        str(binary), "agoal", "user", "rules", "--user-id", user_id, "--format", "json",
    ], env)
    payload = parse_json(result.stdout)
    data = payload.get("data") if isinstance(payload, dict) else None
    rules = data.get("rules") if isinstance(data, dict) else None
    if result.returncode != 0 or not isinstance(rules, list) or not rules or not isinstance(rules[0], dict):
        return None, []
    rule_id = rules[0].get("ruleId")
    periods = rules[0].get("periods")
    if not isinstance(rule_id, str) or not rule_id.strip() or not isinstance(periods, dict):
        return None, []
    rows: list[Any] = []
    for key in ("current", "history"):
        value = periods.get(key)
        if isinstance(value, list):
            rows.extend(value)
    ids: list[str] = []
    for row in rows:
        value = row.get("periodId") if isinstance(row, dict) else None
        if isinstance(value, str) and value.strip() and value not in ids:
            ids.append(value)
    return rule_id, ids


def first_template(binary: Path, env: dict[str, str]) -> tuple[str | None, dict[str, int]]:
    result = run([str(binary), "agoal", "report", "list-statistics", "--format", "json"], env)
    payload = parse_json(result.stdout)
    data = payload.get("data") if isinstance(payload, dict) else None
    rows = data.get("reports") if isinstance(data, dict) else None
    if result.returncode != 0 or not isinstance(rows, list):
        return None, {}
    for row in rows:
        template_id = row.get("templateId") if isinstance(row, dict) else None
        if not isinstance(template_id, str) or not template_id.strip():
            continue
        counts: dict[str, int] = {}
        for state, key in (("ON_TIME", "onTime"), ("LATE", "late"), ("NOT_SUBMITTED", "notSubmitted")):
            value = row.get(key)
            if isinstance(value, int) and not isinstance(value, bool):
                counts[state] = value
        return template_id, counts
    return None, {}


def typed_invalid(result: subprocess.CompletedProcess[str]) -> bool:
    payload = parse_json(result.stderr)
    error = payload.get("error") if isinstance(payload, dict) else None
    return (
        result.returncode == 3
        and not result.stdout.strip()
        and isinstance(error, dict)
        and error.get("type") == "validation"
        and error.get("subtype") == "invalid_flag_value"
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--live", action="store_true")
    args = parser.parse_args()
    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []
    objective_keys: list[str] = []
    submit_shapes: dict[str, tuple[int, list[str], list[str]]] = {}

    focused = run([
        "go", "test", "-count=1", "./internal/helpers", "./internal/cli",
        "-run", "TestAgoal(UserObjectives|SubmitDetail)|TestReviewedRuntimeSchemaExclusions|TestSchemaCatalogDeliveryCompleteness",
    ], env, 900)
    checks.append(("输入边界与精确 exclusion 回归", focused.returncode == 0, f"rc={focused.returncode}"))
    if focused.returncode != 0:
        findings.append("focused objectives/submit-detail tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-agoal-objectives-submit-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env, 900)
        checks.append(("临时构建当前源码", build.returncode == 0, f"rc={build.returncode}"))
        if build.returncode != 0:
            findings.append("current source build failed")
        else:
            help_commands = [
                [str(binary), "agoal", "user", "objectives", "--help"],
                [str(binary), "agoal", "report", "submit-detail", "--help"],
            ]
            help_ok = all(run(command, env).returncode == 0 for command in help_commands)
            checks.append(("两条兼容命令 Help 可发现", help_ok, "rc=0" if help_ok else "help failed"))
            if not help_ok:
                findings.append("objectives/submit-detail Help is not discoverable")

            invalid_cases = [
                [str(binary), "agoal", "user", "objectives", "--user-id", "u", "--rule-id", "r", "--period-ids", ", ,", "--format", "json"],
                [str(binary), "agoal", "report", "submit-detail", "--template-id", "t", "--submit-state", "submitted", "--format", "json"],
                [str(binary), "agoal", "report", "submit-detail", "--template-id", "t", "--submit-state", "ON_TIME", "--query-date", "invalid", "--format", "json"],
                [str(binary), "agoal", "report", "submit-detail", "--template-id", "t", "--submit-state", "ON_TIME", "--page", "0", "--format", "json"],
                [str(binary), "agoal", "report", "submit-detail", "--template-id", "t", "--submit-state", "ON_TIME", "--page-size", "-1", "--format", "json"],
            ]
            invalid_ok = all(typed_invalid(run(command, env)) for command in invalid_cases)
            checks.append(("空周期/非法状态/日期/分页本地 fail-closed", invalid_ok, "5/5 typed validation rc=3" if invalid_ok else "machine error mismatch"))
            if not invalid_ok:
                findings.append("one or more invalid inputs escaped the typed local boundary")

            if args.live:
                user_id = current_user_id(binary, env)
                rule_id, period_ids = active_rules(binary, env, user_id) if user_id else (None, [])
                prerequisites = bool(user_id and rule_id and period_ids)
                checks.append(("目标读取前置事实可建立", prerequisites, f"user=yes, rule={'yes' if rule_id else 'no'}, periods={len(period_ids)}"))
                if prerequisites:
                    result = run([
                        str(binary), "agoal", "user", "objectives",
                        "--user-id", user_id or "",
                        "--rule-id", rule_id or "",
                        "--period-ids", ",".join(period_ids),
                        "--format", "json",
                    ], env)
                    payload = parse_json(result.stdout)
                    rows = payload.get("content") if isinstance(payload, dict) else None
                    objectives_ok = result.returncode == 0 and isinstance(rows, list) and all(isinstance(row, dict) for row in rows)
                    if objectives_ok:
                        objective_keys = sorted({key for row in rows for key in row})
                    checks.append(("已知规则周期的真实目标读取", objectives_ok, f"periods={len(period_ids)}, rows={len(rows) if isinstance(rows, list) else 'unknown'}"))
                    if not objectives_ok:
                        findings.append("live objectives result did not match the reviewed legacy container")
                else:
                    findings.append("could not establish a user/rule/period scope for objectives")

                template_id, statistic_counts = first_template(binary, env)
                checks.append(("提交详情前置 templateId 可建立", bool(template_id), f"template={'yes' if template_id else 'no'}"))
                if template_id:
                    query_date = f"{date.today().isoformat()}T00:00:00+08:00"
                    all_states_ok = True
                    for state in ("ON_TIME", "LATE", "NOT_SUBMITTED"):
                        result = run([
                            str(binary), "agoal", "report", "submit-detail",
                            "--template-id", template_id,
                            "--submit-state", state,
                            "--query-date", query_date,
                            "--page", "1", "--page-size", "20",
                            "--format", "json",
                        ], env)
                        payload = parse_json(result.stdout)
                        content = payload.get("content") if isinstance(payload, dict) else None
                        rows = content.get("result") if isinstance(content, dict) else None
                        state_ok = (
                            result.returncode == 0
                            and isinstance(content, dict)
                            and isinstance(rows, list)
                            and all(isinstance(row, dict) for row in rows)
                        )
                        all_states_ok = all_states_ok and state_ok
                        row_keys = sorted({key for row in rows for key in row}) if state_ok else []
                        user_keys = sorted({
                            key for row in rows
                            if isinstance(row.get("user"), dict)
                            for key in row["user"]
                        }) if state_ok else []
                        submit_shapes[state] = (len(rows) if state_ok else -1, row_keys, user_keys)
                    checks.append(("显式日期三状态真实提交详情", all_states_ok, "ON_TIME/LATE/NOT_SUBMITTED containers reviewed" if all_states_ok else "shape mismatch"))
                    if not all_states_ok:
                        findings.append("one or more live submit-detail states had an unrecognized shape")
                    detail_counts = {state: shape[0] for state, shape in submit_shapes.items() if shape[0] >= 0}
                    count_match = len(statistic_counts) == 3 and detail_counts == statistic_counts
                    checks.append((
                        "统计计数与显式日期详情计数一致",
                        count_match,
                        f"statistics={statistic_counts}, detail={detail_counts}",
                    ))
                    if not count_match:
                        findings.append("list-statistics and explicit-date submit-detail counts disagree")
                else:
                    findings.append("could not establish a report template for submit-detail")
            else:
                checks.append(("真实 objectives/submit-detail 响应", True, "SKIPPED（未传 --live）"))

    lines = [
        "# Agoal objectives / submit-detail Agent 边界审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 用户、规则、周期、模板和人员标识只在内存中使用；报告仅保存计数、字段名与机器契约结论，不保存原始 JSON，也不接入 CI / policy。",
        "",
        f"## Result: {'PASS' if not findings else 'REVIEW'}",
        "",
        "| 检查项 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{evidence}` |" for label, ok, evidence in checks)
    lines += ["", "## 非空字段证据", ""]
    lines.append("- `user objectives`: " + (", ".join(f"`{key}`" for key in objective_keys) if objective_keys else "UNVERIFIED（本次没有非空目标行）"))
    for state in ("ON_TIME", "LATE", "NOT_SUBMITTED"):
        count, row_keys, user_keys = submit_shapes.get(state, (-1, [], []))
        lines.append(
            f"- `submit-detail {state}`: rows={count if count >= 0 else 'UNVERIFIED'}; "
            f"row_keys={','.join(row_keys) if row_keys else 'UNVERIFIED'}; "
            f"user_keys={','.join(user_keys) if user_keys else 'UNVERIFIED'}"
        )
    lines += [
        "",
        "## 结论",
        "",
        "- `period-ids` 必须至少包含一个非空 ID；submit state、ISO 日期和显式页码均在任何业务调用前校验，失败为 `validation/invalid_flag_value`、rc=3。",
        "- 空目标或空提交行只证明本次明确身份、规则、周期、模板、日期与状态范围没有返回记录，不能扩大为组织业务不存在。",
        "- `list-statistics` 与显式日期 `submit-detail` 的三类计数必须分别记录；二者不一致时保持业务口径未知，不能用统计数覆盖详情或反向推断。",
        "- 缺少非空稳定字段或三状态覆盖时，两条命令继续保持精确 exclusion；不将 legacy passthrough 包装成统一结果。",
    ]
    if findings:
        lines += ["", "## Findings", ""] + [f"- {finding}" for finding in findings]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if not findings else 1


if __name__ == "__main__":
    raise SystemExit(main())
