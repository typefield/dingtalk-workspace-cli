#!/usr/bin/env python3
"""Collect Markdown evidence for the offline unified-result surface.

This is an Agent review helper, not a CI or policy gate. It builds the current
tree into a temporary directory, runs the three local result-shape checkers on
safe offline samples, and writes only a human-readable Markdown report. The
shell helpers create any negative JSON samples in their own temporary
directory; no result JSON is saved in the repository.
"""

from __future__ import annotations

import argparse
import json
import os
from datetime import date
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]
CHECKS = (
    "check-stdout-json.sh",
    "check-string-bool.sh",
    "check-envelope-keys.sh",
)


def run(command: list[str], *, env: dict[str, str], timeout: int = 180) -> tuple[int, str]:
    try:
        completed = subprocess.run(
            command,
            cwd=ROOT,
            env=env,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        return 124, f"审计命令超时（{timeout}s）：{' '.join(command)}"
    except OSError as exc:
        return 125, f"无法启动审计命令（{type(exc).__name__}）：{' '.join(command)}"
    output = ((completed.stdout or "") + (completed.stderr or "")).strip()
    return completed.returncode, output


def fenced(text: str) -> str:
    return text if text else "(no stdout/stderr)"


def check_drive_download_version_dry_run(binary: Path, temp: Path, env: dict[str, str]) -> tuple[int, str]:
    """Exercise both historical-download spellings without touching the network.

    This is deliberately an Agent audit sample rather than a policy check.  The
    two CLI paths share implementation through a compatibility route, so both
    must preserve the same terminal JSON contract.  Only a compact structural
    summary is returned; the transient JSON wire is never persisted in the
    repository or copied into the Markdown evidence.
    """
    cases = (
        ("download-version", ["drive", "download-version", "--node", "agent-audit-node", "--version", "3"]),
        ("download --version alias", ["drive", "download", "--node", "agent-audit-node", "--version", "3"]),
    )
    summaries: list[str] = []
    for label, command in cases:
        destination = temp / ("history-" + label.replace(" ", "-") + ".bin")
        try:
            completed = subprocess.run(
                [str(binary), *command, "--output", str(destination), "--dry-run", "--format", "json"],
                cwd=ROOT,
                env=env,
                capture_output=True,
                text=True,
                timeout=90,
            )
        except subprocess.TimeoutExpired:
            return 124, f"{label}: dry-run timeout"
        except OSError as exc:
            return 125, f"{label}: cannot start binary ({type(exc).__name__})"

        try:
            payload = json.loads(completed.stdout)
        except json.JSONDecodeError:
            return 1, f"{label}: stdout is not one JSON result (rc={completed.returncode})"
        data = payload.get("data") if isinstance(payload, dict) else None
        valid = (
            completed.returncode == 0
            and not completed.stderr.strip()
            and isinstance(payload, dict)
            and payload.get("ok") is True
            and payload.get("outcome") == "success"
            and payload.get("dry_run") is True
            and "contract_version" not in payload
            and isinstance(data, dict)
            and data.get("executed") is False
            and data.get("operation") == "download_drive_file_version"
            and data.get("version") == 3
        )
        if not valid:
            return 1, f"{label}: result contract mismatch (rc={completed.returncode})"
        summaries.append(f"{label}: rc=0, one JSON success dry-run, version=3")
    return 0, "; ".join(summaries)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output",
        type=Path,
        required=True,
        help="Markdown evidence path; runtime JSON is never persisted",
    )
    args = parser.parse_args()

    sections = [
        "# 统一结果表面 Agent 审计",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 本报告由 Agent 在当前源码构建出的临时二进制上执行。它不是 CI / policy 门禁，只保存 Markdown 证据；所有 JSON 样本都只在临时目录存在。",
        "",
        "## 结果摘要",
        "",
        "| 项目 | 结果 |",
        "|---|---|",
    ]
    evidence: list[tuple[str, int, str]] = []

    with tempfile.TemporaryDirectory(prefix="dws-unified-result-agent-") as directory:
        temp = Path(directory)
        binary = temp / "dws"
        env = dict(os.environ)
        env["DWS_BIN"] = str(binary)
        env["DWS_SCAN_HOME"] = str(temp / "home")

        build_rc, build_output = run(
            ["go", "build", "-o", str(binary), "./cmd"], env=env, timeout=300
        )
        evidence.append(("临时构建当前二进制", build_rc, build_output))
        sections.append(f"| 临时构建当前二进制 | {'PASS' if build_rc == 0 else f'REVIEW (rc={build_rc})'} |")

        if build_rc == 0:
            history_rc, history_output = check_drive_download_version_dry_run(binary, temp, env)
            evidence.append(("drive 历史版本下载 dry-run 输出对拍", history_rc, history_output))
            sections.append(
                f"| drive 历史版本下载 dry-run 输出对拍 | {'PASS' if history_rc == 0 else f'REVIEW (rc={history_rc})'} |"
            )
            for check in CHECKS:
                script = ROOT / "scripts" / "policy" / check
                for label, command in (
                    (f"{check} self-test", [str(script), "--self-test"]),
                    (f"{check} offline surface", [str(script), "--scope", "dev"]),
                ):
                    rc, output = run(command, env=env)
                    evidence.append((label, rc, output))
                    sections.append(f"| {label} | {'PASS' if rc == 0 else f'REVIEW (rc={rc})'} |")
        else:
            sections.append("| 本轮表面检查 | REVIEW（临时二进制未能构建） |")

        sections += ["", "## 原始 Agent 证据", ""]
        for label, rc, output in evidence:
            sections += [
                f"### {label}",
                "",
                f"退出码：`{rc}`",
                "",
                "```text",
                fenced(output).replace(directory, "<agent-temp>"),
                "```",
                "",
            ]

    sections += [
        "## 审阅边界",
        "",
        "- 默认只运行离线、无登录、无网络、无副作用的 `dev` 样本；真实账号命令必须另行人工授权和取证。",
        "- `--self-test` 中的 `contract_version` 仅是预期被拒绝的临时负向样本，不代表 CLI 输出或保存的结果。",
        "- 本报告通过也只说明所列样本的 stdout 形状、布尔类型和顶层键符合契约；不能证明服务端终态、分页覆盖率或写入零副作用。",
        "- 检查器不得接入 `make policy` 或 CI；需要新证据时由 Agent 重跑本脚本。",
    ]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(sections) + "\n", encoding="utf-8")
    return 0 if all(rc == 0 for _, rc, _ in evidence) else 1


if __name__ == "__main__":
    raise SystemExit(main())
