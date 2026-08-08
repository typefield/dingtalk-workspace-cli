#!/usr/bin/env python3
"""Agent-side audit of the executable Python scripts shipped with multi Skills.

This intentionally records observable help and does not modify files or create
JSON fixtures.  A script is not required to expose both flags: fixed-output
utilities and helpers are reported separately from scripts that advertise a
script-level contract in their Skill documentation.
"""

from __future__ import annotations

import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = ROOT / "skills" / "multi"


def is_entry(path: Path) -> bool:
    return "if __name__" in path.read_text(encoding="utf-8", errors="ignore")


def help_probe(path: Path) -> tuple[int, str]:
    try:
        p = subprocess.run(
            ["python3", str(path), "--help"],
            cwd=ROOT,
            capture_output=True,
            text=True,
            timeout=20,
        )
        return p.returncode, (p.stdout or "") + (p.stderr or "")
    except Exception as exc:  # keep the audit report complete
        return 124, str(exc)


def main() -> int:
    files = sorted(SCRIPTS.glob("*/scripts/*.py"))
    entries = [p for p in files if is_entry(p)]
    rows = []
    for path in entries:
        rc, text = help_probe(path)
        rows.append((path.relative_to(ROOT), rc, "--dry-run" in text, "--format" in text))

    print(f"multi Python files: {len(files)}")
    print(f"Agent entries: {len(entries)}")
    print(f"Help nonzero: {sum(rc != 0 for _, rc, _, _ in rows)}")
    print(f"Help text mentions --dry-run: {sum(dry for _, _, dry, _ in rows)}/{len(rows)}")
    print(f"Help text mentions --format: {sum(fmt for _, _, _, fmt in rows)}/{len(rows)}")
    print("\nNonzero help:")
    for path, rc, dry, fmt in rows:
        if rc:
            print(f"- {path}: rc={rc}, dry_run={dry}, format={fmt}")
    print("\nEntries without both flags (review, not automatic failures):")
    for path, rc, dry, fmt in rows:
        if not (dry and fmt):
            print(f"- {path}: rc={rc}, dry_run={dry}, format={fmt}")
    return 0 if all(rc == 0 for _, rc, _, _ in rows) else 1


if __name__ == "__main__":
    raise SystemExit(main())
