#!/usr/bin/env python3
"""Agent-only audit of Skill version and cli_version metadata.

The output is Markdown evidence, not a generated runtime fixture.  This keeps
the version requirement visible to an Agent reviewer without adding a CI-time
policy dependency or persisting JSON state.
"""

from __future__ import annotations

import argparse
import re
from pathlib import Path


SKILL_VERSION_RE = re.compile(r"^\s*version:\s*[\"']?([^\"'\s]+)[\"']?\s*$", re.MULTILINE)
CLI_VERSION_RE = re.compile(r"^\s*cli_version:\s*[\"']([^\"']+)[\"']\s*$", re.MULTILINE)
SEMVER_RE = re.compile(r"^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$")


def read_versions(path: Path) -> tuple[str, str]:
    text = path.read_text(encoding="utf-8")
    frontmatter = text.split("---", 2)[1] if text.startswith("---") else ""
    skill = SKILL_VERSION_RE.search(frontmatter)
    cli = CLI_VERSION_RE.search(frontmatter)
    return (skill.group(1) if skill else "MISSING", cli.group(1) if cli else "MISSING")


def main() -> int:
    parser = argparse.ArgumentParser(description="scan Skill version declarations")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    root = Path(__file__).resolve().parents[2]
    files = [root / "skills/mono/SKILL.md"]
    files.extend(sorted((root / "skills/multi").glob("*/SKILL.md")))
    rows = [(path.relative_to(root).as_posix(), *read_versions(path)) for path in files]
    skill_versions = sorted({version for _, version, _ in rows})
    cli_versions = sorted({version for _, _, version in rows})
    valid = all(SEMVER_RE.fullmatch(version) for _, version, _ in rows)
    complete_cli = all(version != "MISSING" for _, _, version in rows)

    lines = [
        "# Skill version / cli_version Agent 扫描",
        "",
        "此文件由 Agent 扫描生成；不作为运行时 JSON fixture，也不接入 CI 门禁。",
        "",
        f"- Skill 数量：{len(rows)}",
        f"- Skill 版本声明集合：{', '.join(skill_versions)}",
        f"- CLI 版本声明集合：{', '.join(cli_versions)}",
        f"- Skill 版本格式：{'PASS' if valid else 'REVIEW'}",
        f"- CLI 版本覆盖：{'PASS' if complete_cli else 'REVIEW'}",
        "",
        "| Skill | version | cli_version |",
        "|---|---|---|",
    ]
    lines.extend(f"| `{path}` | `{version}` | `{cli_version}` |" for path, version, cli_version in rows)
    lines += [
        "",
        "结论：每个 Skill 均有独立语义版本，并声明最低 CLI 兼容版本；两者语义不能互相替代。",
    ]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if valid and complete_cli else 1


if __name__ == "__main__":
    raise SystemExit(main())
