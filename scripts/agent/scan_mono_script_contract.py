#!/usr/bin/env python3
"""Agent-side semantic scan for the Mono Skill script interface.

This deliberately emits Markdown rather than JSON.  It is an evidence collector for
an Agent/reviewer, not a CI gate and not a replacement for side-effect testing.
"""

from __future__ import annotations

import argparse
import ast
import os
from pathlib import Path
import subprocess
import sys
from datetime import date
import re


def is_agent_entry(path: Path) -> bool:
    try:
        tree = ast.parse(path.read_text(encoding="utf-8"))
    except (OSError, SyntaxError):
        return False
    for node in ast.walk(tree):
        if not isinstance(node, ast.If) or not isinstance(node.test, ast.Compare):
            continue
        left = node.test.left
        if not isinstance(left, ast.Name) or left.id != "__name__":
            continue
        if any(isinstance(value, ast.Constant) and value.value == "__main__"
               for value in node.test.comparators):
            return True
    return False


def md_cell(value: str) -> str:
    return value.replace("|", "\\|").replace("\n", " ").strip()


def scan(root: Path) -> str:
    script_dir = root / "skills" / "mono" / "scripts"
    files = sorted(script_dir.glob("*.py"))
    entries = [path for path in files if is_agent_entry(path)]
    internal = [path for path in files if path not in entries]
    rows: list[tuple[str, int, bool, bool, str]] = []
    env = os.environ.copy()
    env.setdefault("PYTHONNOUSERSITE", "1")
    for path in entries:
        try:
            result = subprocess.run(
                [sys.executable, str(path), "--help"],
                cwd=root,
                env=env,
                capture_output=True,
                text=True,
                timeout=30,
            )
            help_text = result.stdout + result.stderr
            rows.append((path.name, result.returncode,
                         "--dry-run" in help_text,
                         "--format" in help_text,
                         "PASS" if result.returncode == 0 else "FAIL"))
        except subprocess.TimeoutExpired:
            rows.append((path.name, 124, False, False, "TIMEOUT"))
        except OSError as exc:
            rows.append((path.name, 127, False, False, f"ERROR: {exc}"))

    dry_count = sum(row[2] for row in rows)
    format_count = sum(row[3] for row in rows)
    help_failures = [row for row in rows if row[1] != 0]
    rfc_path = root / "docs" / "rfcs" / "0002-mono-skill-script-interface.md"
    rfc_text = rfc_path.read_text(encoding="utf-8") if rfc_path.exists() else ""
    rfc_claims = {
        "files": re.search(r"\| Python 文件 \| (\d+)", rfc_text),
        "entries": re.search(r"\| Agent 入口 \| (\d+)", rfc_text),
        "internal": re.search(r"\| 内部模块 \| (\d+)", rfc_text),
        "dry": re.search(r"\| Help 暴露 `--dry-run` \| (\d+)", rfc_text),
        "format": re.search(r"\| Help 暴露 `--format` \| (\d+)", rfc_text),
        "help_failures": re.search(r"\| Help 非零 \| (\d+)", rfc_text),
    }
    actuals = {
        "files": len(files), "entries": len(entries), "internal": len(internal),
        "dry": dry_count, "format": format_count,
        "help_failures": len(help_failures),
    }
    rfc_mismatches = []
    for key, match in rfc_claims.items():
        if not match:
            rfc_mismatches.append(f"{key}: RFC row missing")
        elif int(match.group(1)) != actuals[key]:
            rfc_mismatches.append(
                f"{key}: RFC={match.group(1)} actual={actuals[key]}"
            )
    lines = [
        "# Mono Skill 脚本契约 Agent 扫描",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本报告由 Agent 执行生成，仅记录 Help 可观测事实。dry-run 副作用需要受控 child-runner 或真实环境另行证明；本报告不会把未验证项写成通过，也不保存 JSON fixture。",
        "",
        "## 统计口径",
        "",
        "| 指标 | 数量 | 说明 |",
        "|---|---:|---|",
        f"| Python 文件 | {len(files)} | `skills/mono/scripts/*.py` 全部文件 |",
        f"| Agent 入口 | {len(entries)} | 含 `if __name__ == \\\"__main__\\\"` |",
        f"| 内部模块 | {len(internal)} | {', '.join(path.name for path in internal)} |",
        f"| Help 暴露 `--dry-run` | {dry_count}/{len(entries)} | 逐入口运行 `--help` |",
        f"| Help 暴露 `--format` | {format_count}/{len(entries)} | 逐入口运行 `--help` |",
        f"| Help 非零 | {len(help_failures)} | 退出码非 0 的入口 |",
        "",
        "## RFC 对账",
        "",
        f"对账文件：`{rfc_path.relative_to(root) if rfc_path.exists() else rfc_path}`",
        f"状态：{'PASS' if not rfc_mismatches else 'DRIFT'}",
    ]
    if rfc_mismatches:
        lines.extend(f"- {item}" for item in rfc_mismatches)
    lines += [
        "",
        "## 入口明细",
        "",
        "| 脚本 | help rc | dry-run | format | 状态 |",
        "|---|---:|:---:|:---:|---|",
    ]
    for name, rc, has_dry, has_format, status in rows:
        lines.append(f"| `{name}` | {rc} | {'yes' if has_dry else 'no'} | {'yes' if has_format else 'no'} | {status} |")
    lines += [
        "",
        "## 尚未由本扫描证明的事项",
        "",
        "- **UNVERIFIED**：`--dry-run` 是否对每个入口实现零远端写入、零本地写入；需要受控 child-runner、临时 HOME 和写请求计数器。",
        "- **UNVERIFIED**：`--format json` 是否在成功、失败、部分成功和不确定结果下都保持单一可解析 stdout；需要注入式执行夹具。",
        "- **UNVERIFIED**：脚本内部调用 `dws` 的 `remote_reads`、分页和重试语义；不能仅凭 flags 或源码字符串判定。",
        "",
        "## 结论",
        "",
        f"当前 {len(entries)} 个 Agent 入口的 Help 可观测性为 {dry_count}/{len(entries)} dry-run、{format_count}/{len(entries)} format、{len(help_failures)} 个 Help 非零；副作用和结果契约仍需单独实测。",
    ]
    return "\n".join(lines) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    parser.add_argument("--strict-rfc", action="store_true",
                        help="when set, return non-zero if RFC statistics drift")
    args = parser.parse_args()
    report = scan(args.root.resolve())
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        sys.stdout.write(report)
    if args.strict_rfc:
        # Re-run the cheap report check without parsing the Markdown output.  The
        # report itself remains the source of evidence; strict mode is an Agent
        # review aid, not a CI hook.
        files = sorted((args.root.resolve() / "skills" / "mono" / "scripts").glob("*.py"))
        entries = [path for path in files if is_agent_entry(path)]
        rfc = args.root.resolve() / "docs" / "rfcs" / "0002-mono-skill-script-interface.md"
        text = rfc.read_text(encoding="utf-8") if rfc.exists() else ""
        rows = []
        env = os.environ.copy()
        env.setdefault("PYTHONNOUSERSITE", "1")
        for path in entries:
            result = subprocess.run([sys.executable, str(path), "--help"], cwd=args.root.resolve(),
                                    env=env, capture_output=True, text=True, timeout=30)
            text_out = result.stdout + result.stderr
            rows.append((result.returncode, "--dry-run" in text_out, "--format" in text_out))
        expected = [len(files), len(entries), len(files) - len(entries),
                    sum(row[1] for row in rows), sum(row[2] for row in rows),
                    sum(row[0] != 0 for row in rows)]
        patterns = [r"\| Python 文件 \| (\d+)", r"\| Agent 入口 \| (\d+)",
                    r"\| 内部模块 \| (\d+)", r"\| Help 暴露 `--dry-run` \| (\d+)",
                    r"\| Help 暴露 `--format` \| (\d+)", r"\| Help 非零 \| (\d+)"]
        if not rfc.exists() or any(not re.search(p, text) or int(re.search(p, text).group(1)) != value
                                   for p, value in zip(patterns, expected)):
            return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
