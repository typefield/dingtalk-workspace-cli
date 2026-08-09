#!/usr/bin/env python3
"""Agent review for deprecated doc file-management routes in Skill prose.

This is deliberately a reviewer aid rather than a CI rule.  It checks whether
the Mono Skill teaches a legacy ``dws doc`` file-management command as an
actionable route when the public replacement is ``dws drive``.  The scan writes
only Markdown evidence and never stores command-output JSON fixtures.
"""

from __future__ import annotations

import argparse
from datetime import date
from pathlib import Path
import re
import sys


ROOT = Path(__file__).resolve().parents[2]
SKILL_ROOTS = (ROOT / "skills" / "mono", ROOT / "skills" / "multi")
LEGACY = ("upload", "download", "copy", "move", "rename", "delete", "list", "search")
CANDIDATE = re.compile(r"(?<![\\w-])dws\\s+doc\\s+(" + "|".join(LEGACY) + r")\\b")
HISTORICAL = re.compile(r"弃用|已迁移|迁移|兼容|deprecated|历史|旧命令|不再作为|改用", re.IGNORECASE)


def is_explicitly_historical(lines: list[str], line_index: int) -> bool:
    """Use nearby prose rather than an allowlist of files.

    A comparison table may need to name the legacy command.  It is not a
    routing defect when the table or its heading clearly calls the spelling
    deprecated and gives the replacement.  A bare code-fence recipe remains a
    review finding.
    """

    context = " ".join(lines[max(0, line_index - 4) : line_index + 1])
    return bool(HISTORICAL.search(context))


def scan() -> list[tuple[Path, int, str, str]]:
    findings: list[tuple[Path, int, str, str]] = []
    for skill_root in SKILL_ROOTS:
        for path in sorted(skill_root.rglob("*.md")):
            lines = path.read_text(encoding="utf-8").splitlines()
            in_fence = False
            for index, line in enumerate(lines):
                if line.strip().startswith("```"):
                    in_fence = not in_fence
                    continue
                for match in CANDIDATE.finditer(line):
                    # A copyable code-fence command is an active recipe unless
                    # that exact local line calls it legacy.  A broad heading
                    # such as "compatibility" must not exempt a whole stale
                    # tutorial from review.
                    local_context = " ".join(lines[max(0, index - 1) : index + 1])
                    if (not in_fence and is_explicitly_historical(lines, index)) or (
                        in_fence and HISTORICAL.search(local_context)
                    ):
                        continue
                    findings.append((path.relative_to(ROOT), index + 1, match.group(0), line.strip()))
    return findings


def render(findings: list[tuple[Path, int, str, str]]) -> str:
    lines = [
        "# Skill 已迁移 Doc 文件管理路由 Agent 审阅",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本扫描检查 Agent 文档是否把旧 `dws doc` 文件管理入口当作正向路径。它是 Markdown 评测证据，不接入 CI，也不保存 JSON fixture。",
        "",
        "## 结论",
        "",
    ]
    if not findings:
        lines.extend([
        "**PASS**：Mono/Multi Skill 均未发现未标注为迁移/兼容的 `doc upload/download/copy/move/rename/delete/list/search` 正向路由。",
            "",
            "默认路由：文件发现、传输和节点管理使用 `dws drive`；文档正文读取、创建、块编辑、导出和媒体嵌入保留 `dws doc`。",
        ])
        return "\n".join(lines) + "\n"

    lines.extend([
        f"**REVIEW**：发现 {len(findings)} 条未明确标为历史兼容的旧路由。请改为对应的 `dws drive` 命令，或在相邻文字中说明它仅用于兼容。",
        "",
        "| 文件 | 行 | 旧路径 | 原文 |",
        "|---|---:|---|---|",
    ])
    for path, line, command, source in findings:
        safe = source.replace("|", "\\|")
        lines.append(f"| `{path}` | {line} | `{command}` | {safe} |")
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    report = render(scan())
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        sys.stdout.write(report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
