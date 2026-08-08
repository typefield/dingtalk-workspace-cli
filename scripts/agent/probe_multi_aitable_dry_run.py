#!/usr/bin/env python3
"""Agent-side side-effect probe for Multi AITable write wrappers.

The probe uses temporary local fixtures and a sentinel ``dws`` executable. A
passing result means the script accepted dry-run, produced a structured plan,
and did not invoke the sentinel or perform an OSS upload. No fixture is kept
in the repository.
"""

from __future__ import annotations

import json
import os
import subprocess
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
BASE = "ABCdef12"
TABLE = "ABCdef13"


def run(script: Path, args: list[str], env: dict[str, str]) -> tuple[int, str, str]:
    p = subprocess.run(
        ["python3", str(script), *args],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=30,
    )
    return p.returncode, p.stdout, p.stderr


def assert_plan(name: str, script: Path, args: list[str], env: dict[str, str]) -> None:
    rc, stdout, stderr = run(script, args + ["--dry-run"], env)
    if rc != 0:
        raise AssertionError(f"{name}: rc={rc}, stderr={stderr[:300]}")
    try:
        payload = json.loads(stdout)
    except json.JSONDecodeError as exc:
        raise AssertionError(f"{name}: stdout is not JSON: {stdout[:300]}") from exc
    if payload.get("ok") is not True or payload.get("dry_run") is not True:
        raise AssertionError(f"{name}: missing dry-run contract: {payload!r}")


def main() -> int:
    with tempfile.TemporaryDirectory(prefix="dws-multi-dry-run-") as directory:
        tmp = Path(directory)
        bin_dir = tmp / "bin"
        bin_dir.mkdir()
        sentinel = tmp / "unexpected-dws-call"
        fake = bin_dir / "dws"
        fake.write_text(f"#!/bin/sh\ntouch '{sentinel}'\nexit 99\n", encoding="utf-8")
        fake.chmod(0o755)

        csv = tmp / "records.csv"
        csv.write_text("a,b\n1,2\n", encoding="utf-8")
        fields = tmp / "fields.json"
        fields.write_text('[{"fieldName":"A","type":"text"}]', encoding="utf-8")
        attachment = tmp / "attachment.txt"
        attachment.write_text("x", encoding="utf-8")

        env = os.environ.copy()
        env["PATH"] = f"{bin_dir}{os.pathsep}{env.get('PATH', '')}"
        env["OPENCLAW_WORKSPACE"] = str(tmp)
        checks = [
            (
                "aitable_import_via_task",
                ROOT / "skills/multi/dingtalk-aitable/scripts/aitable_import_via_task.py",
                [BASE, str(csv), "--dws", str(fake)],
            ),
            (
                "bulk_add_fields",
                ROOT / "skills/multi/dingtalk-aitable/scripts/bulk_add_fields.py",
                [BASE, TABLE, str(fields)],
            ),
            (
                "import_records",
                ROOT / "skills/multi/dingtalk-aitable/scripts/import_records.py",
                [BASE, TABLE, str(csv)],
            ),
            (
                "upload_attachment",
                ROOT / "skills/multi/dingtalk-aitable/scripts/upload_attachment.py",
                [BASE, str(attachment)],
            ),
        ]
        for name, script, args in checks:
            assert_plan(name, script, args, env)
        if sentinel.exists():
            raise AssertionError("dry-run invoked the sentinel dws executable")

    print("multi AITable dry-run probe: 4/4 passed; no dws or OSS write observed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
