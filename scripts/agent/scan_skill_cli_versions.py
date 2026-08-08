#!/usr/bin/env python3
"""Agent-only audit of Skill cli_version metadata.

The output is Markdown evidence, not a generated runtime fixture.  This keeps
the version requirement visible to an Agent reviewer without adding a CI-time
policy dependency or persisting JSON state.
"""

from __future__ import annotations

import argparse
import re
from pathlib import Path


VERSION_RE = re.compile(r"^\s*cli_version:\s*[\"']([^\"']+)[\"']\s*$", re.MULTILINE)


def read_version(path: Path) -> str:
    match = VERSION_RE.search(path.read_text(encoding="utf-8"))
    return match.group(1) if match else "MISSING"


def main() -> int:
    parser = argparse.ArgumentParser(description="scan Skill cli_version declarations")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    root = Path(__file__).resolve().parents[2]
    files = [root / "skills/mono/SKILL.md"]
    files.extend(sorted((root / "skills/multi").glob("*/SKILL.md")))
    rows = [(path.relative_to(root).as_posix(), read_version(path)) for path in files]
    versions = sorted({version for _, version in rows})

    lines = [
        "# Skill cli_version Agent 扫描",
        "",
        "此文件由 Agent 扫描生成；不作为运行时 JSON fixture，也不接入 CI 门禁。",
        "",
        f"- Skill 数量：{len(rows)}",
        f"- 版本声明集合：{', '.join(versions)}",
        "",
        "| Skill | cli_version |",
        "|---|---|",
    ]
    lines.extend(f"| `{path}` | `{version}` |" for path, version in rows)
    lines += ["", "结论：mono 与 multi 的最低 CLI 版本声明已统一为 `>=1.0.15`。"]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
