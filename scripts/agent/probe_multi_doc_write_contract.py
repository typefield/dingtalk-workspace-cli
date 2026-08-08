#!/usr/bin/env python3
"""Agent semantic probe for the Multi document create/write script.

All input and fake child commands are temporary.  The optional artifact is a
Markdown evidence report, not a JSON fixture or proof of real doc terminal
state.
"""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "skills" / "multi" / "dingtalk-doc" / "scripts" / "doc_create_and_write.py"


def execute(args: list[str], env: dict[str, str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(SCRIPT), *args], cwd=ROOT, env=env, capture_output=True, text=True, timeout=30)


def envelope(proc: subprocess.CompletedProcess[str]) -> tuple[dict[str, Any] | None, str]:
    lines = [line for line in proc.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None, f"stdout lines={len(lines)}"
    try:
        value = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return None, f"invalid JSON: {exc}"
    return (value, "ok") if isinstance(value, dict) else (None, "result is not an object")


def write_dws(path: Path, mode: str) -> None:
    """Create a fake document service for success or second-chunk ambiguity."""
    path.write_text(
        "#!/usr/bin/env python3\n"
        "import json, os, sys\n"
        "argv = sys.argv[1:]\n"
        "counter_path = os.environ.get('PROBE_DOC_COUNTER')\n"
        "def count():\n"
        "  if not counter_path: return 0\n"
        "  try: value = int(open(counter_path, encoding='utf-8').read())\n"
        "  except FileNotFoundError: value = 0\n"
        "  value += 1\n"
        "  open(counter_path, 'w', encoding='utf-8').write(str(value))\n"
        "  return value\n"
        "if argv[:2] == ['doc', 'create']:\n"
        "  print(json.dumps({'ok': True, 'data': {'nodeId': 'node_1'}}))\n"
        "elif argv[:2] == ['doc', 'update']:\n"
        "  number = count()\n"
        f"  if {mode!r} == 'second_unknown' and number == 2:\n"
        "    print(json.dumps({'success': False, 'error': {'type': 'api', 'message': 'ambiguous update'}}))\n"
        "  else:\n"
        "    print(json.dumps({'ok': True, 'data': {'accepted': True}}))\n"
        "elif argv[:2] == ['doc', 'read']:\n"
        "  print(json.dumps({'ok': True, 'data': {'markdown': os.environ.get('PROBE_DOC_BODY', '')}}))\n"
        "else:\n"
        "  print(json.dumps({'ok': False, 'error': {'type': 'internal', 'message': 'unexpected command'}}))\n",
        encoding="utf-8",
    )
    path.chmod(0o755)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    outcomes: list[tuple[str, str, str]] = []
    with tempfile.TemporaryDirectory(prefix="dws-multi-doc-probe-") as directory:
        tmp = Path(directory)
        blocked_bin = tmp / "blocked-bin"
        blocked_bin.mkdir()
        sentinel = tmp / "unexpected-dws-call"
        blocked = blocked_bin / "dws"
        blocked.write_text(f"#!/bin/sh\ntouch '{sentinel}'\nexit 99\n", encoding="utf-8")
        blocked.chmod(0o755)
        base_env = {**os.environ, "PATH": f"{blocked_bin}{os.pathsep}{os.environ.get('PATH', '')}"}
        common = ["--name", "probe", "--content", "hello", "--format", "json"]

        help_proc = execute(["--help"], base_env)
        help_ok = help_proc.returncode == 0 and "--format {text,json,ndjson}" in help_proc.stdout and "--dry-run" in help_proc.stdout and "--yes" in help_proc.stdout
        outcomes.append(("脚本 Help 可发现 format/dry-run/yes", "PASS" if help_ok else "FAIL", f"rc={help_proc.returncode}"))

        dry_run = execute([*common, "--dry-run"], base_env)
        payload, detail = envelope(dry_run)
        dry_ok = dry_run.returncode == 0 and payload is not None and payload.get("dry_run") is True and not sentinel.exists()
        outcomes.append(("dry-run 零 child 调用且返回计划", "PASS" if dry_ok else "FAIL", f"rc={dry_run.returncode}; {detail}; sentinel={sentinel.exists()}"))

        no_yes = execute(common, base_env)
        payload, detail = envelope(no_yes)
        no_yes_ok = no_yes.returncode == 1 and payload is not None and payload.get("error", {}).get("subtype") == "confirmation_required" and not sentinel.exists()
        outcomes.append(("未确认创建写入 fail-closed", "PASS" if no_yes_ok else "FAIL", f"rc={no_yes.returncode}; {detail}; sentinel={sentinel.exists()}"))

        retry = execute([*common, "--yes", "--max-retries", "2"], base_env)
        payload, detail = envelope(retry)
        retry_ok = retry.returncode == 1 and payload is not None and payload.get("error", {}).get("type") == "validation" and not sentinel.exists()
        outcomes.append(("非幂等写入拒绝自动重试配置", "PASS" if retry_ok else "FAIL", f"rc={retry.returncode}; {detail}; sentinel={sentinel.exists()}"))

        success_bin = tmp / "success-bin"
        success_bin.mkdir()
        write_dws(success_bin / "dws", "success")
        success = execute(
            [*common, "--yes"],
            {**base_env, "PATH": f"{success_bin}{os.pathsep}{base_env['PATH']}", "PROBE_DOC_BODY": "hello"},
        )
        payload, detail = envelope(success)
        success_ok = success.returncode == 0 and payload is not None and payload.get("outcome") == "success" and payload.get("data", {}).get("verification", {}).get("state") == "verified"
        outcomes.append(("创建、写入、读回逐块对拍后才标 verified", "PASS" if success_ok else "FAIL", f"rc={success.returncode}; {detail}"))

        partial_bin = tmp / "partial-bin"
        partial_bin.mkdir()
        write_dws(partial_bin / "dws", "second_unknown")
        two_chunks = "a" * 30_000 + "\n" + "b" * 30_000
        counter = tmp / "updates"
        partial = execute(
            ["--name", "probe", "--content", two_chunks, "--yes", "--format", "json"],
            {**base_env, "PATH": f"{partial_bin}{os.pathsep}{base_env['PATH']}", "PROBE_DOC_COUNTER": str(counter), "PROBE_DOC_BODY": two_chunks},
        )
        payload, detail = envelope(partial)
        partial_ok = (
            partial.returncode == 7 and payload is not None and payload.get("outcome") == "partial_failure"
            and payload.get("data", {}).get("succeeded", [{}])[0].get("id") == "document:create"
            and payload.get("data", {}).get("unknown", [{}])[0].get("id") == "node_1:chunk:2"
            and counter.read_text(encoding="utf-8") == "2"
        )
        outcomes.append(("后续块未知保留前序成功且不重放", "PASS" if partial_ok else "FAIL", f"rc={partial.returncode}; {detail}"))

    passed = sum(1 for _, status, _ in outcomes if status == "PASS")
    lines = [
        "# Multi 文档创建写入 Agent 语义探针", "",
        "临时 child runner 和内容仅用于本次探针；报告不保存 JSON fixture，也不创建真实文档。", "",
        "| 检查 | 结果 | 证据 |", "|---|---|---|",
    ]
    for name, status, detail in outcomes:
        lines.append("| {} | {} | {} |".format(name, status, detail.replace("|", "\\|")))
    lines.extend(["", f"结论：**{passed}/{len(outcomes)} PASS**。", "", "范围：验证 Help、确认门禁、零写预览、禁止自动重放、逐块 partial 与读回表达；真实文档服务终态仍须隔离账号复验。", ""])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(outcomes) else 1


if __name__ == "__main__":
    raise SystemExit(main())
