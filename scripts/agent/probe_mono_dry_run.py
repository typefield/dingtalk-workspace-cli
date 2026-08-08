#!/usr/bin/env python3
"""Agent-side dry-run probe for Mono's deep-gated write workflows.

The probe uses temporary fixtures and a sentinel ``dws`` executable.  A passing
case must emit exactly one JSON result, report ``dry_run=true``, and never invoke
the sentinel or create files beyond the fixture supplied by the probe.  This is
an Agent evidence collector, not a CI gate and it intentionally writes no JSON
fixture into the repository.
"""

from __future__ import annotations

import json
import argparse
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[2]
SCRIPT_DIR = ROOT / "skills" / "mono" / "scripts"


def _write_sentinel(path: Path, log: Path) -> None:
    path.write_text(
        "#!/usr/bin/env python3\n"
        "import os, pathlib, sys\n"
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


def _cases(root: Path) -> list[tuple[str, list[str]]]:
    fixtures = root / "fixtures"
    fixtures.mkdir()
    (fixtures / "fields.json").write_text(
        json.dumps([{"fieldName": "Owner", "type": "text"}]), encoding="utf-8"
    )
    (fixtures / "records.csv").write_text("Name\nAlice\n", encoding="utf-8")
    (fixtures / "todos.json").write_text(
        json.dumps([{"title": "probe", "executors": "user-probe"}]),
        encoding="utf-8",
    )
    (fixtures / "attachment.txt").write_text("probe", encoding="utf-8")
    return [
        ("doc_create_and_write", [
            "--name", "probe-doc", "--content", "hello",
            "--format", "json", "--dry-run",
        ]),
        ("oa_batch_approve", [
            "--action", "approve", "--instance-ids", "instance-probe",
            "--format", "json", "--dry-run",
        ]),
        ("todo_batch_create", [
            str(fixtures / "todos.json"), "--format", "json", "--dry-run",
        ]),
        ("bulk_add_fields", [
            "base-probe", "table-probe", str(fixtures / "fields.json"),
            "--format", "json", "--dry-run",
        ]),
        ("import_records", [
            "base-probe", "table-probe", str(fixtures / "records.csv"),
            "--format", "json", "--dry-run",
        ]),
        ("calendar_schedule_meeting", [
            "--title", "probe", "--start", "2026-08-08T10:00",
            "--end", "2026-08-08T11:00", "--format", "json", "--dry-run",
        ]),
        ("upload_attachment", [
            "base-probe", str(fixtures / "attachment.txt"),
            "--format", "json", "--dry-run",
        ]),
    ]


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    results: list[tuple[str, str, str]] = []
    with tempfile.TemporaryDirectory(prefix="dws-mono-dry-run-") as temp:
        sandbox = Path(temp)
        sentinel_log = sandbox / "sentinel.log"
        sentinel = sandbox / "dws"
        _write_sentinel(sentinel, sentinel_log)
        baseline = _files(sandbox)
        env = os.environ.copy()
        env["HOME"] = str(sandbox / "home")
        env["PYTHONDONTWRITEBYTECODE"] = "1"
        env["PATH"] = f"{sandbox}{os.pathsep}{env.get('PATH', '')}"
        for name, case_args in _cases(sandbox):
            before = _files(sandbox)
            command = [sys.executable, str(SCRIPT_DIR / f"{name}.py"), *case_args]
            proc = subprocess.run(
                command, cwd=sandbox, env=env, capture_output=True, text=True,
                timeout=60,
            )
            after = _files(sandbox)
            unexpected = sorted(after - before - {"sentinel.log"})
            sentinel_called = sentinel_log.exists()
            if sentinel_called:
                sentinel_log.unlink()
            try:
                payload = json.loads(proc.stdout)
                valid = (
                    proc.returncode == 0
                    and payload.get("ok") is True
                    and payload.get("outcome") == "success"
                    and payload.get("dry_run") is True
                    and not sentinel_called
                    and not unexpected
                )
                detail = "ok" if valid else (
                    f"rc={proc.returncode}, payload={payload!r}, "
                    f"sentinel={sentinel_called}, files={unexpected}"
                )
            except json.JSONDecodeError as exc:
                valid = False
                detail = f"rc={proc.returncode}, invalid stdout JSON: {exc}"
            results.append((name, "PASS" if valid else "FAIL", detail))

    lines = [
        "# Mono 深层 dry-run Agent 受控探针",
        "",
        "> 临时 HOME、临时工作目录和 sentinel dws；不保存 JSON fixture。PASS 表示单一 JSON stdout、dry_run=true、无 dws 调用、无额外本地文件。",
        "",
        "| 入口 | 结果 | 说明 |",
        "|---|---|---|",
    ]
    for name, status, detail in results:
        lines.append(f"| `{name}.py` | {status} | {detail} |")
    passed = sum(status == "PASS" for _, status, _ in results)
    lines.extend(["", f"结果：{passed}/{len(results)} 通过"])
    report = "\n".join(lines) + "\n"
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        sys.stdout.write(report)
    return 0 if passed == len(results) else 1


if __name__ == "__main__":
    raise SystemExit(main())
