#!/usr/bin/env python3
"""Agent semantic probe for the reviewed attendance schedule write workflow.

Temporary child responses prove only the script's local guardrails and result
semantics.  They do not prove a tenant accepted, applied, or persisted any
schedule record.  The output is Markdown evidence, never a saved JSON fixture.
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "skills" / "multi" / "dingtalk-misc" / "scripts" / "attendance_schedule_import.py"
SCHEDULES = '[{"userId":"user_001","workDate":"2026-08-10","classId":101,"isRest":"N"}]'


def _single_json(process: subprocess.CompletedProcess[str]) -> tuple[bool, dict[str, Any] | None, str]:
    lines = [line for line in process.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return False, None, f"stdout lines={len(lines)}"
    try:
        payload = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return False, None, f"invalid JSON: {exc}"
    return isinstance(payload, dict), payload if isinstance(payload, dict) else None, "ok"


def _fake_dws(path: Path) -> None:
    path.write_text(
        """#!/usr/bin/env python3
import json, os, sys
from pathlib import Path

calls = Path(os.environ['SCHEDULE_PROBE_CALLS'])
calls.write_text((calls.read_text() if calls.exists() else '') + ' '.join(sys.argv[1:]) + '\\n')
args = sys.argv[1:]
if args[:3] == ['attendance', 'group', 'get']:
    print(json.dumps({'success': True, 'result': {'groupVO': {'type': 'TURN', 'name': '研发排班组', 'classIds': [101]}}}))
elif args[:3] == ['attendance', 'class', 'search']:
    print(json.dumps({'success': True, 'result': [{'id': 101, 'name': '早班'}]}))
elif args[:3] == ['contact', 'user', 'get']:
    print(json.dumps({'success': True, 'result': [{'userid': 'user_001', 'name': '张三'}]}))
elif args[:3] == ['attendance', 'schedule', 'import']:
    if os.environ.get('SCHEDULE_PROBE_IMPORT') == 'fail':
        print(json.dumps({'success': False, 'error': {'message': 'gateway result lost'}}))
        raise SystemExit(1)
    print(json.dumps({'success': True, 'result': {'requestId': 'request-1'}}))
else:
    print(json.dumps({'success': False, 'error': {'message': 'unexpected child command'}}))
    raise SystemExit(1)
""",
        encoding="utf-8",
    )
    path.chmod(0o755)


def _run(temp_dir: Path, extra: list[str], *, import_behavior: str = "success") -> subprocess.CompletedProcess[str]:
    environment = os.environ.copy()
    environment.update({
        "SCHEDULE_PROBE_CALLS": str(temp_dir / "calls.log"),
        "SCHEDULE_PROBE_IMPORT": import_behavior,
        "PATH": f"{temp_dir}{os.pathsep}{environment.get('PATH', '')}",
    })
    return subprocess.run(
        [sys.executable, str(SCRIPT), "--group-id", "123", "--schedules", SCHEDULES, "--format", "json", *extra],
        cwd=temp_dir,
        capture_output=True,
        text=True,
        timeout=30,
        env=environment,
    )


def _record(rows: list[tuple[str, str, str]], name: str, passed: bool, detail: str) -> None:
    rows.append((name, "PASS" if passed else "FAIL", detail.replace("|", "\\|")))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown evidence; default stdout")
    args = parser.parse_args()
    rows: list[tuple[str, str, str]] = []

    with tempfile.TemporaryDirectory(prefix="dws-attendance-schedule-probe-") as temp_name:
        temp_dir = Path(temp_name)
        _fake_dws(temp_dir / "dws")
        help_process = subprocess.run([sys.executable, str(SCRIPT), "--help"], cwd=temp_dir, capture_output=True, text=True, timeout=30)
        _record(rows, "Help 可发现确认、格式与预览语义", help_process.returncode == 0 and "--yes" in help_process.stdout and "--format" in help_process.stdout and "--dry-run" in help_process.stdout, f"rc={help_process.returncode}")

        invalid = subprocess.run([sys.executable, str(SCRIPT), "--group-id", "123", "--schedules", '[{"userId":[],"workDate":"2026-08-10","classId":101,"isRest":"N"}]', "--format", "json"], cwd=temp_dir, capture_output=True, text=True, timeout=30)
        valid, payload, detail = _single_json(invalid)
        _record(rows, "类型错误在任何远端调用前返回 typed validation", valid and invalid.returncode == 1 and payload is not None and payload.get("error", {}).get("subtype") == "invalid_user_id", f"rc={invalid.returncode}; {detail}")

        dry = _run(temp_dir, ["--dry-run"])
        valid, payload, detail = _single_json(dry)
        dry_calls = (temp_dir / "calls.log").read_text(encoding="utf-8")
        _record(rows, "dry-run 仅做只读校验且不导入排班", valid and dry.returncode == 0 and payload is not None and payload.get("dry_run") is True and payload.get("data", {}).get("write") == "not_sent" and "attendance schedule import" not in dry_calls, f"rc={dry.returncode}; {detail}; child_reads={len(dry_calls.splitlines())}")

        no_yes = _run(temp_dir, [])
        valid, payload, detail = _single_json(no_yes)
        no_yes_calls = (temp_dir / "calls.log").read_text(encoding="utf-8")
        _record(rows, "未确认只返回 preview 与 confirmation_required", valid and no_yes.returncode == 1 and payload is not None and payload.get("error", {}).get("type") == "confirmation_required" and payload.get("data", {}).get("write") == "not_sent" and "attendance schedule import" not in no_yes_calls, f"rc={no_yes.returncode}; {detail}")

        accepted = _run(temp_dir, ["--yes"])
        valid, payload, detail = _single_json(accepted)
        _record(rows, "受确认写入只报告请求受理，不夸大为逐条终态", valid and accepted.returncode == 0 and payload is not None and payload.get("outcome") == "success" and payload.get("data", {}).get("request", {}).get("state") == "accepted" and payload.get("data", {}).get("verification", {}).get("state") == "not_verified", f"rc={accepted.returncode}; {detail}")

        failed = _run(temp_dir, ["--yes"], import_behavior="fail")
        valid, payload, detail = _single_json(failed)
        _record(rows, "写请求异常保留 execution_state=unknown 且不建议盲重试", valid and failed.returncode == 1 and payload is not None and payload.get("error", {}).get("subtype") == "schedule_import_unconfirmed" and payload.get("error", {}).get("details", {}).get("execution_state") == "unknown", f"rc={failed.returncode}; {detail}")

    passed = sum(status == "PASS" for _, status, _ in rows)
    report = "\n".join([
        "# Multi Attendance 排班导入 Agent 语义探针", "",
        "临时 child runner 验证脚本的确认门禁、只读预览、机器输出和终态诚实性；不证明真实租户排班已经生效。", "",
        "| 检查 | 结果 | 证据 |", "|---|---|---|",
        *(f"| {name} | {status} | {detail} |" for name, status, detail in rows),
        "", f"结论：**{passed}/{len(rows)} PASS**。", "",
        "范围：验证 Help、输入门禁、dry-run、确认、请求受理和不确定写入；真实权限、人员归属、排班覆盖和最终记录须由隔离组织复验。", "",
    ])
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(rows) else 1


if __name__ == "__main__":
    raise SystemExit(main())
