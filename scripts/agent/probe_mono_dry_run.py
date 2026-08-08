#!/usr/bin/env python3
"""Run a small, controlled dry-run probe for high-risk Mono Skill writers.

The probe uses a fake ``dws`` executable and a temporary workspace.  A passing
result means the selected fixture did not attempt a child CLI call or mutate
the temporary workspace; it is not a proof for every possible business path.
The report is Markdown only.
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


CASES = (
    ("doc_create_and_write", ("doc_create_and_write.py", "--name", "Probe", "--content", "hello")),
    ("oa_batch_approve", ("oa_batch_approve.py", "--action", "approve")),
    ("todo_batch_create", ("todo_batch_create.py", "__TODOS__")),
    ("bulk_add_fields", ("bulk_add_fields.py", "base123", "table123", "__FIELDS__")),
    ("import_records", ("import_records.py", "base123", "table123", "__RECORDS__")),
    ("calendar_schedule_meeting", (
        "calendar_schedule_meeting.py", "--title", "Probe",
        "--start", "2030-01-01T10:00:00+08:00",
        "--end", "2030-01-01T11:00:00+08:00",
    )),
    ("upload_attachment", ("upload_attachment.py", "base12345", "__ATTACHMENT__")),
)


def _write_fake_dws(path: Path, log: Path) -> None:
    path.write_text(
        "#!/bin/sh\n"
        f"printf '%s\\n' \"$*\" >> {str(log)!r}\n"
        "printf '%s\\n' '{\"ok\":false,\"outcome\":\"failure\",\"error\":{\"type\":\"probe\"}}'\n"
        "exit 99\n",
        encoding="utf-8",
    )
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def _snapshot(root: Path, ignored: set[Path]) -> set[str]:
    return {
        str(path.relative_to(root))
        for path in root.rglob("*")
        if path.is_file() and path not in ignored
    }


def run(root: Path) -> str:
    script_dir = root / "skills" / "mono" / "scripts"
    with tempfile.TemporaryDirectory(prefix="dws-mono-dry-run-") as tmp_name:
        tmp = Path(tmp_name)
        bin_dir = tmp / "bin"
        bin_dir.mkdir()
        home = tmp / "home"
        home.mkdir()
        log = tmp / "dws-calls.log"
        fake_dws = bin_dir / "dws"
        _write_fake_dws(fake_dws, log)
        todos = tmp / "todos.json"
        todos.write_text('[{"title":"probe todo","executors":"user-1"}]', encoding="utf-8")
        fields = tmp / "fields.json"
        fields.write_text('[{"fieldName":"probe","type":"text"}]', encoding="utf-8")
        records = tmp / "records.json"
        records.write_text('[{"cells":{"fldprobe":"v"}}]', encoding="utf-8")
        attachment = tmp / "attachment.txt"
        attachment.write_text("probe attachment", encoding="utf-8")
        fixed = {
            "__TODOS__": str(todos),
            "__FIELDS__": str(fields),
            "__RECORDS__": str(records),
            "__ATTACHMENT__": str(attachment),
        }
        ignored = {todos, fields, records, attachment, fake_dws, log}
        env = os.environ.copy()
        env.update({
            "HOME": str(home),
            "OPENCLAW_WORKSPACE": str(tmp),
            "PATH": f"{bin_dir}{os.pathsep}{env.get('PATH', '')}",
            "PYTHONDONTWRITEBYTECODE": "1",
        })
        rows: list[tuple[str, int, bool, bool, str]] = []
        for name, raw_args in CASES:
            args = [fixed.get(arg, arg) for arg in raw_args]
            before = _snapshot(tmp, ignored)
            proc = subprocess.run(
                [sys.executable, str(script_dir / args[0]), *args[1:], "--dry-run", "--format", "json"],
                cwd=tmp,
                env=env,
                capture_output=True,
                text=True,
                timeout=60,
            )
            after = _snapshot(tmp, ignored)
            remote_attempted = log.exists() and bool(log.read_text(encoding="utf-8"))
            if log.exists():
                log.unlink()
            local_changed = before != after
            try:
                payload = json.loads(proc.stdout)
                json_ok = isinstance(payload, dict) and "ok" in payload and "outcome" in payload
            except json.JSONDecodeError:
                json_ok = False
            status = "PASS" if proc.returncode == 0 and not remote_attempted and not local_changed and json_ok else "FAIL"
            detail = []
            if remote_attempted:
                detail.append("remote-call")
            if local_changed:
                detail.append("local-write")
            if not json_ok:
                detail.append("invalid-json-envelope")
            rows.append((name, proc.returncode, remote_attempted, local_changed, status + (f" ({', '.join(detail)})" if detail else "")))

    lines = [
        "# Mono Skill 深层写脚本 dry-run 受控探针",
        "",
        "> Agent probe：使用临时 HOME/工作区和假的 `dws` 子进程。`PASS` 只证明下面这些固定 fixture 的 dry-run 没有远端调用、没有临时工作区写入且 stdout 是统一 JSON；不替代真实账号、异常分支和全量业务验证。",
        "",
        "| 脚本 | rc | 远端调用 | 临时区写入 | 结果 |",
        "|---|---:|:---:|:---:|---|",
    ]
    for name, rc, remote, local, status in rows:
        lines.append(f"| `{name}` | {rc} | {'yes' if remote else 'no'} | {'yes' if local else 'no'} | {status} |")
    passed = sum(status.startswith("PASS") for *_, status in rows)
    lines += [
        "",
        f"结论：{passed}/{len(rows)} 个受控 fixture 通过；剩余路径仍标记为 `UNVERIFIED`，不得据此扩大为全量安全承诺。",
    ]
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    report = run(args.root.resolve())
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        sys.stdout.write(report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
