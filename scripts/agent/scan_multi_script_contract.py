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


def documented_script_flag_mismatches(files: list[Path]) -> list[str]:
    """Check flags in positive Python-script recipes against that script Help."""
    by_name = {path.name: path for path in files}
    pattern = re.compile(r"\bpython3?\s+(?:scripts/)?([^\s`|()]+\.py)([^\n`)]*)")
    ignored = ("禁止", "错误", "不存在", "不要", "废弃", "反例")
    mismatches: list[str] = []
    cache: dict[Path, str] = {}
    for markdown in (ROOT / "skills" / "multi").rglob("*.md"):
        text = markdown.read_text(encoding="utf-8", errors="ignore")
        for line_no, line in enumerate(text.splitlines(), 1):
            if any(marker in line for marker in ignored):
                continue
            match = pattern.search(line)
            if not match:
                continue
            script = by_name.get(Path(match.group(1)).name)
            if script is None:
                continue
            help_text = cache.get(script)
            if help_text is None:
                _, help_text = help_probe(script)
                cache[script] = help_text
            flags = re.findall(r"(?<![\w-])(--[a-z][a-z0-9-]*)", match.group(2))
            for flag in sorted(set(flags)):
                if flag not in help_text:
                    mismatches.append(
                        f"{markdown.relative_to(ROOT)}:{line_no}: {script.name} {flag}"
                    )
    return mismatches


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
    mismatches = documented_script_flag_mismatches(entries)
    print(f"\nDocumented Python-script flag mismatches: {len(mismatches)}")
    for mismatch in mismatches:
        print(f"- {mismatch}")
    return 0 if all(rc == 0 for _, rc, _, _ in rows) and not mismatches else 1


if __name__ == "__main__":
    raise SystemExit(main())
