#!/usr/bin/env python3
"""Collect Agent evidence for Mono attendance schedule import semantics.

The probe uses an isolated fake ``dws`` only to observe local orchestration.
It does not contact a tenant and its Markdown report is evidence for review,
not a CI gate or proof that a real schedule was persisted.
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
SCRIPT = ROOT / "skills" / "mono" / "scripts" / "attendance_schedule_import.py"
SCHEDULES = json.dumps([
    {"userId": "user-probe", "workDate": "2026-08-08", "classId": 101, "isRest": "N"},
], ensure_ascii=False)


def _single_json(process: subprocess.CompletedProcess[str]) -> tuple[bool, dict[str, Any] | None, str]:
    lines = [line for line in process.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return False, None, f"stdout lines={len(lines)}"
    try:
        payload = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return False, None, f"invalid JSON: {exc}"
    if not isinstance(payload, dict):
        return False, None, "result is not an object"
    return True, payload, "ok"


def _write_fake_dws(path: Path) -> None:
    path.write_text(
        """#!/usr/bin/env python3
import json, os, sys
from pathlib import Path

calls = Path(os.environ['MONO_SCHEDULE_PROBE_CALLS'])
with calls.open('a', encoding='utf-8') as handle:
    handle.write(json.dumps(sys.argv[1:], ensure_ascii=False) + '\\n')
args = sys.argv[1:]
if args[:3] == ['attendance', 'group', 'get']:
    print(json.dumps({'success': True, 'result': {'groupVO': {'type': 'TURN', 'name': 'Probe Group', 'classIds': [101]}}}))
elif args[:3] == ['contact', 'user', 'get']:
    print(json.dumps({'success': True, 'result': [{'userid': 'user-probe', 'name': 'Probe User'}]}))
elif args[:3] == ['attendance', 'class', 'search']:
    print(json.dumps({'success': True, 'result': {'items': [{'classId': 101, 'className': '早班'}]}}))
elif args[:3] == ['attendance', 'schedule', 'import']:
    if os.environ.get('MONO_SCHEDULE_PROBE_IMPORT') == 'ambiguous_failure':
        print(json.dumps({'success': False, 'error': {'type': 'api', 'message': 'gateway response lost', 'retryable': True}}))
        raise SystemExit(1)
    print(json.dumps({'success': True, 'result': {'requestId': 'schedule-request-probe'}}))
else:
    print(json.dumps({'success': False, 'error': {'type': 'validation', 'message': 'unexpected child command'}}))
    raise SystemExit(2)
""",
        encoding="utf-8",
    )
    path.chmod(0o755)


def _run(temp_dir: Path, extra: list[str], *, import_behavior: str = "success") -> subprocess.CompletedProcess[str]:
    environment = os.environ.copy()
    environment.update({
        "HOME": str(temp_dir / "home"),
        "PYTHONDONTWRITEBYTECODE": "1",
        "MONO_SCHEDULE_PROBE_CALLS": str(temp_dir / "calls.log"),
        "MONO_SCHEDULE_PROBE_IMPORT": import_behavior,
        "PATH": f"{temp_dir}{os.pathsep}{environment.get('PATH', '')}",
    })
    return subprocess.run(
        [
            sys.executable, str(SCRIPT), "--group-id", "123", "--schedules", SCHEDULES,
            "--format", "json", *extra,
        ],
        cwd=temp_dir,
        env=environment,
        capture_output=True,
        text=True,
        timeout=60,
    )


def _calls(path: Path) -> list[list[str]]:
    if not path.exists():
        return []
    return [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]


def _record(rows: list[tuple[str, str, str]], check: str, passed: bool, detail: str) -> None:
    rows.append((check, "PASS" if passed else "FAIL", detail.replace("|", "\\|")))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default stdout")
    args = parser.parse_args()
    rows: list[tuple[str, str, str]] = []

    with tempfile.TemporaryDirectory(prefix="dws-mono-schedule-probe-") as temp_name:
        temp_dir = Path(temp_name)
        _write_fake_dws(temp_dir / "dws")

        help_process = subprocess.run(
            [sys.executable, str(SCRIPT), "--help"], cwd=temp_dir,
            capture_output=True, text=True, timeout=30,
        )
        _record(
            rows,
            "Help 只公开脚本确认参数 --yes",
            help_process.returncode == 0
            and "--yes" in help_process.stdout
            and "--confirm" not in help_process.stdout
            and "--format" in help_process.stdout
            and "--dry-run" in help_process.stdout,
            f"rc={help_process.returncode}",
        )

        invalid = subprocess.run(
            [
                sys.executable, str(SCRIPT), "--group-id", "123",
                "--schedules", '[{"userId":[],"workDate":"2026-08-08","classId":101,"isRest":"N"}]',
                "--format", "json",
            ],
            cwd=temp_dir,
            capture_output=True,
            text=True,
            timeout=30,
        )
        valid, payload, detail = _single_json(invalid)
        _record(
            rows,
            "类型错误在任何 child 调用前返回 typed validation",
            valid and invalid.returncode == 1 and payload is not None
            and payload.get("error", {}).get("type") == "validation"
            and not _calls(temp_dir / "calls.log"),
            f"rc={invalid.returncode}; {detail}; child_calls={len(_calls(temp_dir / 'calls.log'))}",
        )

        no_yes = _run(temp_dir, [])
        valid, payload, detail = _single_json(no_yes)
        _record(
            rows,
            "缺确认在任何 child 调用前 fail-closed",
            valid and no_yes.returncode == 1 and payload is not None
            and payload.get("error", {}).get("type") == "policy"
            and payload.get("error", {}).get("subtype") == "confirmation_required"
            and payload.get("data", {}).get("execution_state") == "not_executed"
            and not _calls(temp_dir / "calls.log"),
            f"rc={no_yes.returncode}; {detail}; child_calls={len(_calls(temp_dir / 'calls.log'))}",
        )

        dry_run = _run(temp_dir, ["--dry-run"])
        valid, payload, detail = _single_json(dry_run)
        dry_calls = _calls(temp_dir / "calls.log")
        _record(
            rows,
            "dry-run 可做只读校验但不会导入排班",
            valid and dry_run.returncode == 0 and payload is not None
            and payload.get("dry_run") is True
            and not any(call[:3] == ["attendance", "schedule", "import"] for call in dry_calls),
            f"rc={dry_run.returncode}; {detail}; child_reads={len(dry_calls)}",
        )

        accepted = _run(temp_dir, ["--yes"])
        valid, payload, detail = _single_json(accepted)
        accepted_calls = _calls(temp_dir / "calls.log")
        import_calls = [call for call in accepted_calls if call[:3] == ["attendance", "schedule", "import"]]
        _record(
            rows,
            "确认后传底层 canonical --user-say-yes，且只报告请求受理",
            valid and accepted.returncode == 0 and payload is not None
            and len(import_calls) == 1
            and "--user-say-yes" in import_calls[0]
            and payload.get("data", {}).get("execution_state") == "accepted"
            and payload.get("data", {}).get("verification", {}).get("state") == "not_verified",
            f"rc={accepted.returncode}; {detail}; import_calls={len(import_calls)}",
        )

        ambiguous = _run(temp_dir, ["--yes"], import_behavior="ambiguous_failure")
        valid, payload, detail = _single_json(ambiguous)
        error = payload.get("error", {}) if payload else {}
        _record(
            rows,
            "写请求异常保留 unknown、禁止把 retryable 透传为重放许可",
            valid and ambiguous.returncode == 1 and payload is not None
            and payload.get("data", {}).get("execution_state") == "unknown"
            and error.get("subtype") == "schedule_import_unconfirmed"
            and "retryable" not in error,
            f"rc={ambiguous.returncode}; {detail}",
        )

    passed = sum(status == "PASS" for _, status, _ in rows)
    report = "\n".join([
        "# Mono 考勤排班导入 Agent 语义探针",
        "",
        "> 临时 child runner 只验证脚本的本地确认、参数委托与结果表达；不证明真实租户的权限、排班持久化或 exactly-once。仅保存 Markdown 证据。",
        "",
        "| 检查 | 结果 | 证据 |",
        "|---|---|---|",
        *(f"| {check} | {status} | {detail} |" for check, status, detail in rows),
        "",
        f"结论：**{passed}/{len(rows)} PASS**。",
        "",
        "边界：成功只表示 child API 接受请求，脚本明确标记 `verification.state=not_verified`；写请求异常只表示终态未知，Agent 必须先查询排班而不是重放导入。",
        "",
    ])
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(rows) else 1


if __name__ == "__main__":
    raise SystemExit(main())
