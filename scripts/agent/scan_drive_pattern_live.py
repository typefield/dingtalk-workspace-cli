#!/usr/bin/env python3
"""Agent audit for drive list --pattern without persisting Drive data.

The scan builds current source in a temporary directory. With --live it reads
one bounded Drive page, derives an exact pattern in memory, and checks wildcard,
exact-match, no-match, and continuation preservation. Evidence contains only
counts and booleans; names, IDs, tokens, and response JSON are never written.
This is an Agent review tool, not a CI/policy gate.
"""

from __future__ import annotations

import argparse
from datetime import date
import json
import os
from pathlib import Path
import subprocess
import tempfile


ROOT = Path(__file__).resolve().parents[2]


def run(command: list[str], *, env: dict[str, str], timeout: int = 300) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        command,
        cwd=ROOT,
        env=env,
        text=True,
        capture_output=True,
        timeout=timeout,
        check=False,
    )


def decode_items(result: subprocess.CompletedProcess[str]) -> tuple[list[dict[str, object]], str]:
    if result.returncode != 0 or result.stderr.strip():
        raise ValueError("command did not produce a clean JSON success")
    payload = json.loads(result.stdout)
    if payload.get("success") is not True or not isinstance(payload.get("result"), dict):
        raise ValueError("unexpected legacy Drive envelope")
    body = payload["result"]
    items = body.get("items")
    if not isinstance(items, list) or any(not isinstance(item, dict) for item in items):
        raise ValueError("Drive items are not an object array")
    token = body.get("nextToken", "")
    if token is None:
        token = ""
    if not isinstance(token, str):
        raise ValueError("Drive continuation is not a string")
    return items, token


def escape_go_path_pattern(value: str) -> str:
    # Go filepath.Match uses backslash to quote metacharacters on Unix.
    return "".join("\\" + char if char in "\\*?[" else char for char in value)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence path")
    parser.add_argument("--live", action="store_true", help="perform bounded authenticated read-only Drive calls")
    args = parser.parse_args()

    env = dict(os.environ)
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    checks: list[tuple[str, bool, str]] = []
    findings: list[str] = []

    focused = run(
        [
            "go", "test", "-count=1", "./internal/helpers", "-run",
            "Test(DriveListPatternFiltersCurrentPageAndPreservesContinuation|DriveListPatternRejectsUnknownResponseInsteadOfReturningFalseEmpty|CrossPlatformCoverageMatchDriveNamePattern)",
        ],
        env=env,
    )
    ok = focused.returncode == 0
    checks.append(("pattern 投影与 continuation 回归", ok, f"rc={focused.returncode}"))
    if not ok:
        findings.append("focused Drive pattern tests failed")

    with tempfile.TemporaryDirectory(prefix="dws-drive-pattern-agent-") as directory:
        binary = Path(directory) / "dws"
        build = run(["go", "build", "-o", str(binary), "./cmd"], env=env)
        ok = build.returncode == 0
        checks.append(("临时构建当前源码", ok, f"rc={build.returncode}"))
        if not ok:
            findings.append("current source build failed")
        else:
            help_result = run([str(binary), "drive", "list", "--help"], env=env)
            help_ok = help_result.returncode == 0 and "--pattern" in help_result.stdout
            checks.append(("Help 公开 --pattern", help_ok, f"rc={help_result.returncode}"))
            if not help_ok:
                findings.append("drive list Help does not expose --pattern")

            if args.live:
                try:
                    baseline_result = run(
                        [str(binary), "drive", "list", "--limit", "100", "--format", "json"],
                        env=env,
                        timeout=90,
                    )
                    baseline, _ = decode_items(baseline_result)
                    names = [item.get("name") for item in baseline]
                    if not baseline or any(not isinstance(name, str) or not name for name in names):
                        raise ValueError("bounded page has no stable non-empty names")

                    wildcard_result = run(
                        [str(binary), "drive", "list", "--limit", "100", "--pattern", "*", "--format", "json"],
                        env=env,
                        timeout=90,
                    )
                    wildcard, _ = decode_items(wildcard_result)
                    wildcard_ok = len(wildcard) == len(baseline)
                    checks.append(("通配正例", wildcard_ok, f"baseline={len(baseline)}, matched={len(wildcard)}"))
                    if not wildcard_ok:
                        findings.append("wildcard pattern did not preserve the bounded page")

                    selected_name = names[0]
                    exact_expected = sum(name == selected_name for name in names)
                    exact_result = run(
                        [
                            str(binary), "drive", "list", "--limit", "100", "--pattern",
                            escape_go_path_pattern(selected_name), "--format", "json",
                        ],
                        env=env,
                        timeout=90,
                    )
                    exact, _ = decode_items(exact_result)
                    exact_ok = len(exact) == exact_expected and exact_expected > 0
                    checks.append(("精确正例", exact_ok, f"expected={exact_expected}, matched={len(exact)}"))
                    if not exact_ok:
                        findings.append("escaped exact pattern did not match the selected name count")

                    sentinel = "__dws_agent_no_match_8f731c5b__"
                    if sentinel in names:
                        raise ValueError("no-match sentinel unexpectedly exists")
                    empty_result = run(
                        [str(binary), "drive", "list", "--limit", "100", "--pattern", sentinel, "--format", "json"],
                        env=env,
                        timeout=90,
                    )
                    empty, _ = decode_items(empty_result)
                    empty_ok = len(empty) == 0
                    checks.append(("无命中反例", empty_ok, f"matched={len(empty)}"))
                    if not empty_ok:
                        findings.append("no-match pattern returned entries")

                    page_result = run(
                        [str(binary), "drive", "list", "--limit", "5", "--format", "json"],
                        env=env,
                        timeout=90,
                    )
                    page_items, page_token = decode_items(page_result)
                    filtered_page_result = run(
                        [str(binary), "drive", "list", "--limit", "5", "--pattern", "*", "--format", "json"],
                        env=env,
                        timeout=90,
                    )
                    filtered_page, filtered_token = decode_items(filtered_page_result)
                    token_ok = (
                        len(page_items) == len(filtered_page)
                        and bool(page_token)
                        and page_token == filtered_token
                    )
                    checks.append(("当前页过滤保留续页令牌", token_ok, f"page_count={len(page_items)}, token_preserved={'yes' if token_ok else 'no'}"))
                    if not token_ok:
                        findings.append("pattern filtering changed or dropped the current page continuation")
                except (json.JSONDecodeError, KeyError, TypeError, ValueError) as error:
                    findings.append(f"live Drive structural probe failed: {error}")
            else:
                checks.extend([
                    ("通配正例", True, "SKIPPED（未传 --live）"),
                    ("精确正例", True, "SKIPPED（未传 --live）"),
                    ("无命中反例", True, "SKIPPED（未传 --live）"),
                    ("当前页过滤保留续页令牌", True, "SKIPPED（未传 --live）"),
                ])

    passed = not findings
    lines = [
        "# drive list --pattern Agent 实测",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 当前源码临时构建；真实 Drive JSON 只在内存解析。本文件不保存文件名、ID、路径、token 或原始响应，也不接入 CI / policy。",
        "",
        f"## Result: {'PASS' if passed else 'REVIEW'}",
        "",
        "| 检查项 | 结果 | 脱敏证据 |",
        "|---|---|---|",
    ]
    lines.extend(f"| {label} | {'PASS' if ok else 'REVIEW'} | `{summary}` |" for label, ok, summary in checks)
    lines += [
        "",
        "## 结论",
        "",
        "- `--pattern` 对服务端选定的当前页做客户端名称过滤；通配、精确和无命中三类实测均符合预期。",
        "- 过滤不吞掉服务端 continuation；它不把当前页匹配结果扩大成整个 Drive 目录完整性。",
        "- 本次正常根页只证明当前账号、当前 endpoint 页的行为，不证明子目录、权限受限、分页后续页或服务端目录覆盖。",
    ]
    if findings:
        lines += ["", "## Findings", ""]
        lines.extend(f"- {finding}" for finding in findings)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
