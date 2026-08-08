#!/usr/bin/env python3
"""Run the Skill-facing Agent audits and write one Markdown evidence report.

This is intentionally an Agent/reviewer tool, not a CI gate.  It invokes the
existing semantic probes against the current source tree and stores only their
human-readable Markdown/text evidence; no JSON fixture or generated runtime
catalog is written.
"""

from __future__ import annotations

import argparse
from datetime import date
from pathlib import Path
import subprocess
import sys
import tempfile


def run(root: Path, command: list[str]) -> tuple[int, str]:
    result = subprocess.run(
        command,
        cwd=root,
        capture_output=True,
        text=True,
        timeout=900,
    )
    output = (result.stdout or "") + (result.stderr or "")
    return result.returncode, output.rstrip()


def redact_temp_path(output: str, temp_dir: Path) -> str:
    """Keep checked-in evidence stable across runs.

    The child probes need a real temporary directory, but its host-specific
    absolute path is not evidence and causes needless Markdown churn.  Replace
    only this run's directory; leave all other command/output text intact.
    """

    return output.replace(str(temp_dir), "<agent-temp>")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output", type=Path, required=True,
        help="Markdown evidence path; no JSON output is produced",
    )
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]

    with tempfile.TemporaryDirectory(prefix="dws-skill-audit-") as temp_dir:
        shortcut_report = Path(temp_dir) / "shortcut-surface.md"
        probes = [
            (
                "Mono 脚本 Help/Skill 参数对账",
                [sys.executable, "scripts/agent/scan_mono_script_contract.py",
                 "--strict-rfc", "--strict-flags"],
            ),
            (
                "Multi 脚本 Help/Skill 参数对账",
                [sys.executable, "scripts/agent/scan_multi_script_contract.py"],
            ),
            (
                "Shortcut 运行时/目录/Skill 集合对账",
                [sys.executable, "scripts/agent/scan_shortcut_surface_alignment.py",
                 "--output", str(shortcut_report)],
            ),
            (
                "Shortcut exclusion 逐条审阅队列",
                [sys.executable, "scripts/agent/scan_shortcut_exclusions.py",
                 "--output", str(Path(temp_dir) / "shortcut-exclusions.md")],
            ),
            (
                "Skill CLI 路径/参数逐条对拍",
                ["go", "run", "./scripts/policy/skill-command-check"],
            ),
            (
                "Skill 隐藏兼容 flag Agent 审阅",
                ["go", "run", "./scripts/policy/skill-command-check", "--agent-semantic"],
            ),
        ]

        sections: list[str] = [
            "# Skill 契约 Agent 总审计",
            "",
            f"扫描日期：{date.today().isoformat()}",
            "",
            "> 本报告由 Agent 在当前源码上执行。它是评测证据，不是 CI 门禁；只保存 Markdown/text，不保存 JSON fixture。",
            "",
            "## 结果摘要",
            "",
            "| 审计项 | 结果 |",
            "|---|---|",
        ]
        results: list[tuple[str, int, str]] = []
        for title, command in probes:
            rc, output = run(root, command)
            if title.startswith("Shortcut"):
                generated = shortcut_report if "集合" in title else Path(temp_dir) / "shortcut-exclusions.md"
                if generated.exists():
                    output = generated.read_text(encoding="utf-8")
            results.append((title, rc, output))
            sections.append(f"| {title} | {'PASS' if rc == 0 else f'REVIEW (rc={rc})'} |")

        sections += ["", "## 原始 Agent 证据", ""]
        for (title, command), (_, rc, output) in zip(probes, results):
            sections += [
                f"### {title}",
                "",
                f"命令：`{' '.join(command)}`".replace(str(temp_dir), "<agent-temp>"),
                "",
                "```text",
                redact_temp_path(output, temp_dir),
                "```",
                "",
            ]

        sections += [
            "## 解释边界",
            "",
            "- Help 对账只证明参数可发现，不证明业务执行安全。",
            "- CLI 路径/参数对拍只证明当前公开 Help 接受文档中的 flags；隐藏兼容别名是否应继续教学，仍需 Agent 语义审阅。",
            "- 隐藏兼容 flag 审阅只把正向示例列为 REVIEW，不删除兼容 alias，也不作为 CI 阻断；应由 Agent 决定改 canonical 参数或保留历史说明。",
            "- dry-run 仍需由受控 child-runner、临时 HOME 和写请求计数器证明零写入。",
            "- 集合对账只证明 Runtime、目录和 Skill 不漂移，不证明后端数据真实存在。",
        ]
        final_report = "\n".join(sections) + "\n"
        final_rc = 0 if all(rc == 0 for _, rc, _ in results) else 1
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(final_report, encoding="utf-8")
    return final_rc


if __name__ == "__main__":
    raise SystemExit(main())
