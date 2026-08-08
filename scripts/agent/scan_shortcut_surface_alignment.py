#!/usr/bin/env python3
"""Agent audit for runtime shortcut, catalog, and Skill-surface alignment.

The scan executes the current source tree in mock mode and compares the
in-memory runtime public set with the committed public catalog and the Mono
Skill overview. It deliberately does not write the command's JSON response or
any fixture to the repository; only a human-readable Markdown audit is saved.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
from collections import Counter
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
CATALOG = ROOT / "docs" / "shortcut-public-catalog.json"
MONO_SKILL = ROOT / "skills" / "mono" / "SKILL.md"


def runtime_public() -> set[tuple[str, str]]:
    proc = subprocess.run(
        ["go", "run", "./cmd", "shortcut", "list", "--all", "--mock", "--format", "json"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.returncode != 0:
        raise RuntimeError(f"runtime shortcut list failed (rc={proc.returncode}): {proc.stderr.strip()}")
    try:
        payload = json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"runtime shortcut list returned invalid JSON: {exc}") from exc
    return {
        (str(row.get("service") or ""), str(row.get("command") or ""))
        for row in payload.get("shortcuts", [])
        if row.get("public") is True
    }


def catalog_public() -> set[tuple[str, str]]:
    payload = json.loads(CATALOG.read_text(encoding="utf-8"))
    return {
        (str(row.get("service") or ""), str(row.get("command") or ""))
        for row in payload.get("results", [])
    }


def mono_counts() -> dict[str, int]:
    text = MONO_SKILL.read_text(encoding="utf-8")
    start = text.index("<!-- VISIBLE_SHORTCUTS_OVERVIEW_START -->")
    end = text.index("<!-- VISIBLE_SHORTCUTS_OVERVIEW_END -->", start)
    block = text[start:end]
    return {
        service: int(count)
        for service, count in re.findall(r"\| `([^`]+)` \| (\d+) \|", block)
    }


def render(output: Path) -> int:
    errors: list[str] = []
    try:
        runtime = runtime_public()
        catalog = catalog_public()
        skill = mono_counts()
    except Exception as exc:  # noqa: BLE001 - audit must leave evidence on failure
        output.write_text(
            "# Shortcut surface alignment Agent scan\n\n"
            f"- generated_at: `{datetime.now().isoformat(timespec='seconds')}`\n"
            f"- result: **FAIL**\n- error: `{exc}`\n",
            encoding="utf-8",
        )
        return 1

    runtime_only = sorted(runtime - catalog)
    catalog_only = sorted(catalog - runtime)
    if runtime_only:
        errors.append(f"runtime public entries missing from catalog: {runtime_only}")
    if catalog_only:
        errors.append(f"catalog entries missing at runtime: {catalog_only}")

    expected_counts = Counter(service for service, _ in catalog)
    skill_only = sorted(set(skill) - set(expected_counts))
    missing_skill = sorted(set(expected_counts) - set(skill))
    if skill_only:
        errors.append(f"Skill-only services: {skill_only}")
    if missing_skill:
        errors.append(f"catalog services missing from Skill overview: {missing_skill}")
    for service, expected in sorted(expected_counts.items()):
        actual = skill.get(service)
        if actual != expected:
            errors.append(f"{service}: Skill count={actual!r}, catalog count={expected}")

    result = "PASS" if not errors else "FAIL"
    lines = [
        "# Shortcut surface alignment Agent scan",
        "",
        f"- generated_at: `{datetime.now().isoformat(timespec='seconds')}`",
        "- source: current `go run ./cmd shortcut list --all --mock --format json`",
        "- fixture policy: runtime JSON is held in memory and not saved; this file is Markdown evidence only",
        f"- result: **{result}**",
        "",
        "| surface | count |",
        "|---|---:|",
        f"| runtime public | {len(runtime)} |",
        f"| committed catalog | {len(catalog)} |",
        f"| Mono Skill total | {sum(skill.values())} |",
        "",
        "| service | catalog | Skill |",
        "|---|---:|---:|",
    ]
    for service in sorted(expected_counts):
        lines.append(f"| `{service}` | {expected_counts[service]} | {skill.get(service, 'MISSING')} |")
    lines.extend(["", "## Findings", ""])
    lines.extend(f"- **FAIL**: {error}" for error in errors)
    if not errors:
        lines.append("- runtime public set, committed catalog, and Skill counts are identical")
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if not errors else 1


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, required=True, help="Markdown evidence path")
    args = parser.parse_args()
    return render(args.output)


if __name__ == "__main__":
    raise SystemExit(main())
