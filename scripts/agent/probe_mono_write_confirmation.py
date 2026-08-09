#!/usr/bin/env python3
"""Agent probe for the Mono composite-write confirmation boundary.

The probe deliberately omits ``--yes`` from non-dry-run invocations.  It runs
each workflow inside a temporary directory with a sentinel ``dws`` executable
and records only Markdown evidence.  A PASS proves the local script gate
emits a typed policy failure before it starts a child CLI process or writes a
new local artifact; it does not prove a real tenant's write semantics.
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SCRIPT_DIR = ROOT / "skills" / "mono" / "scripts"


def _write_sentinel(path: Path, log: Path) -> None:
    path.write_text(
        "#!/usr/bin/env python3\n"
        "import pathlib, sys\n"
        f"pathlib.Path({str(log)!r}).write_text(' '.join(sys.argv[1:]), encoding='utf-8')\n"
        "print('{}')\n",
        encoding="utf-8",
    )
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def _files(root: Path) -> set[str]:
    return {
        str(path.relative_to(root))
        for path in root.rglob("*")
        if path.is_file()
    }


def _fixtures(root: Path) -> Path:
    fixtures = root / "fixtures"
    fixtures.mkdir()
    (fixtures / "fields.json").write_text(
        json.dumps([{"fieldName": "Owner", "type": "text"}]), encoding="utf-8"
    )
    (fixtures / "records.csv").write_text("Name\nAlice\n", encoding="utf-8")
    (fixtures / "todos.json").write_text(
        json.dumps([{"title": "probe", "executors": "user-probe"}]), encoding="utf-8"
    )
    (fixtures / "attachment.txt").write_text("probe", encoding="utf-8")
    return fixtures


def _cases(fixtures: Path) -> list[tuple[str, list[str]]]:
    return [
        ("doc_create_and_write", [
            "--name", "probe-doc", "--content", "hello", "--format", "json",
        ]),
        ("mail_send_with_cc", [
            "--to", "probe@example.com", "--subject", "probe", "--body", "body",
            "--format", "json",
        ]),
        ("calendar_schedule_meeting", [
            "--title", "probe", "--start", "2026-08-08T10:00",
            "--end", "2026-08-08T11:00", "--format", "json",
        ]),
        ("todo_batch_create", [str(fixtures / "todos.json"), "--format", "json"]),
        ("bulk_add_fields", [
            "base-probe", "table-probe", str(fixtures / "fields.json"), "--format", "json",
        ]),
        ("import_records", [
            "base-probe", "table-probe", str(fixtures / "records.csv"), "1", "--format", "json",
        ]),
        ("aitable_import_via_task", [
            "base-probe", str(fixtures / "records.csv"), "--format", "json",
        ]),
        ("upload_attachment", [
            "base-probe", str(fixtures / "attachment.txt"), "--format", "json",
        ]),
        ("oa_batch_approve", [
            "--action", "approve", "--instance-ids", "instance-probe", "--format", "json",
        ]),
    ]


def _valid_confirmation(proc: subprocess.CompletedProcess[str]) -> tuple[bool, str]:
    try:
        payload: Any = json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        return False, f"stdout 非单一 JSON：{exc}"
    valid = (
        proc.returncode == 1
        and isinstance(payload, dict)
        and payload.get("ok") is False
        and payload.get("outcome") == "failure"
        and isinstance(payload.get("error"), dict)
        and payload["error"].get("type") == "policy"
        and payload["error"].get("subtype") == "confirmation_required"
        and isinstance(payload.get("data"), dict)
        and payload["data"].get("execution_state") == "not_executed"
    )
    return valid, "ok" if valid else f"rc={proc.returncode}, payload={payload!r}"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()

    results: list[tuple[str, str, str]] = []
    with tempfile.TemporaryDirectory(prefix="dws-mono-confirmation-") as temp:
        sandbox = Path(temp)
        fixtures = _fixtures(sandbox)
        sentinel = sandbox / "dws"
        sentinel_log = sandbox / "sentinel.log"
        _write_sentinel(sentinel, sentinel_log)
        env = os.environ.copy()
        env["HOME"] = str(sandbox / "home")
        env["PYTHONDONTWRITEBYTECODE"] = "1"
        env["PATH"] = f"{sandbox}{os.pathsep}{env.get('PATH', '')}"

        for name, case_args in _cases(fixtures):
            before = _files(sandbox)
            proc = subprocess.run(
                [sys.executable, str(SCRIPT_DIR / f"{name}.py"), *case_args],
                cwd=sandbox,
                env=env,
                capture_output=True,
                text=True,
                timeout=60,
            )
            after = _files(sandbox)
            called = sentinel_log.exists()
            if called:
                sentinel_log.unlink()
            unexpected = sorted(after - before - {"sentinel.log"})
            valid, detail = _valid_confirmation(proc)
            if called or unexpected:
                valid = False
                detail = f"{detail}; child_called={called}, extra_files={unexpected}"
            results.append((name, "PASS" if valid else "FAIL", detail))

    lines = [
        "# Mono 复合写确认门禁 Agent 受控探针",
        "",
        "> 调用均省略 `--yes` 且不使用 `--dry-run`。临时 HOME、工作目录和 sentinel `dws` 用于证明本地脚本在 child CLI/本地写入前停止；仅保存 Markdown 证据，不替代真实租户验证。",
        "",
        "| 入口 | 结果 | 说明 |",
        "|---|---|---|",
    ]
    for name, status, detail in results:
        lines.append(f"| `{name}.py` | {status} | {detail} |")
    passed = sum(status == "PASS" for _, status, _ in results)
    lines.extend([
        "",
        f"结果：{passed}/{len(results)} 通过",
        "",
        "## 边界",
        "",
        "- PASS 表示缺少确认时本地脚本返回 `policy/confirmation_required`、`execution_state=not_executed`，且 probe 未观察到子 dws 调用或新增本地文件。",
        "- 不证明真实租户的权限、服务端写入终态或确认后的 exactly-once；这些仍需要隔离账号或受控后端证据。",
    ])
    report = "\n".join(lines) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        sys.stdout.write(report)
    return 0 if passed == len(results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
