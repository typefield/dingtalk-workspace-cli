#!/usr/bin/env python3
"""Agent semantic probe for the Multi OA batch approval script.

The probe is evidence, not a CI gate: fake child commands and inputs live in a
temporary directory.  Optional output is Markdown only; no JSON fixture is
kept and no real approval is created, approved, or rejected.
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
SCRIPT = ROOT / "skills" / "multi" / "dingtalk-misc" / "scripts" / "oa_batch_approve.py"


def execute(args: list[str], env: dict[str, str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(SCRIPT), *args], cwd=ROOT, env=env, capture_output=True, text=True, timeout=30)


def one_envelope(proc: subprocess.CompletedProcess[str]) -> tuple[dict[str, Any] | None, str]:
    lines = [line for line in proc.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None, f"stdout lines={len(lines)}"
    try:
        payload = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return None, f"invalid JSON: {exc}"
    return (payload, "ok") if isinstance(payload, dict) else (None, "result is not an object")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    outcomes: list[tuple[str, str, str]] = []
    with tempfile.TemporaryDirectory(prefix="dws-multi-oa-probe-") as directory:
        tmp = Path(directory)
        bin_dir = tmp / "bin"
        bin_dir.mkdir()
        sentinel = tmp / "unexpected-dws-call"
        fake = bin_dir / "dws"
        fake.write_text(f"#!/bin/sh\ntouch '{sentinel}'\nexit 99\n", encoding="utf-8")
        fake.chmod(0o755)
        env = {**os.environ, "PATH": f"{bin_dir}{os.pathsep}{os.environ.get('PATH', '')}"}

        help_proc = execute(["--help"], env)
        help_ok = help_proc.returncode == 0 and "--format {text,json,ndjson}" in help_proc.stdout and "--dry-run" in help_proc.stdout and "--yes" in help_proc.stdout
        outcomes.append(("脚本 Help 可发现 format/dry-run/yes", "PASS" if help_ok else "FAIL", f"rc={help_proc.returncode}"))

        dry_run = execute(["--action", "approve", "--instance-ids", "one", "--dry-run", "--format", "json"], env)
        payload, detail = one_envelope(dry_run)
        dry_run_ok = dry_run.returncode == 0 and payload is not None and payload.get("ok") is True and payload.get("dry_run") is True
        outcomes.append(("dry-run 零写入且输出单信封", "PASS" if dry_run_ok and not sentinel.exists() else "FAIL", f"rc={dry_run.returncode}; {detail}; sentinel={sentinel.exists()}"))

        no_yes = execute(["--action", "approve", "--instance-ids", "one", "--format", "json"], env)
        payload, detail = one_envelope(no_yes)
        no_yes_ok = no_yes.returncode == 1 and payload is not None and payload.get("error", {}).get("subtype") == "confirmation_required" and not sentinel.exists()
        outcomes.append(("非确认路径 fail-closed", "PASS" if no_yes_ok else "FAIL", f"rc={no_yes.returncode}; {detail}"))

        response_bin = tmp / "response-bin"
        response_bin.mkdir()
        response = response_bin / "dws"
        response.write_text(
            "#!/usr/bin/env python3\n"
            "import json, sys\n"
            "argv = sys.argv[1:]\n"
            "instance = argv[argv.index('--instance-id') + 1] if '--instance-id' in argv else ''\n"
            "if argv[:3] == ['oa', 'approval', 'tasks'] and instance == 'one':\n  print(json.dumps({'ok': True, 'data': {'tasks': [{'taskId': 'task_one'}]}}))\n"
            "elif argv[:3] == ['oa', 'approval', 'tasks'] and instance == 'two':\n  print(json.dumps({'success': False, 'error': {'type': 'api', 'message': 'ambiguous task lookup'}}))\n"
            "elif argv[:3] == ['oa', 'approval', 'detail']:\n  print(json.dumps({'ok': True, 'data': {'processInstanceId': instance, 'status': 'RUNNING'}}))\n"
            "elif argv[:3] == ['oa', 'approval', 'approve']:\n  print(json.dumps({'ok': True, 'data': {'accepted': True}, 'meta': {'request': 'accepted'}}))\n"
            "else:\n  print(json.dumps({'ok': False, 'error': {'type': 'internal', 'message': 'unexpected command'}}))\n",
            encoding="utf-8",
        )
        response.chmod(0o755)
        partial = execute(["--action", "approve", "--instance-ids", "one,two", "--yes", "--format", "json"], {**env, "PATH": f"{response_bin}{os.pathsep}{env['PATH']}"})
        payload, detail = one_envelope(partial)
        partial_ok = (
            partial.returncode == 7 and payload is not None and payload.get("ok") is False
            and payload.get("outcome") == "partial_failure"
            and payload.get("data", {}).get("succeeded", [{}])[0].get("id") == "one"
            and payload.get("data", {}).get("succeeded", [{}])[0].get("verification", {}).get("state") == "not_verified"
            and payload.get("data", {}).get("unknown", [{}])[0].get("id") == "two"
        )
        outcomes.append(("审批请求成功与任务查询未知不互相压缩", "PASS" if partial_ok else "FAIL", f"rc={partial.returncode}; {detail}"))

        ambiguous_bin = tmp / "ambiguous-bin"
        ambiguous_bin.mkdir()
        ambiguous = ambiguous_bin / "dws"
        ambiguous.write_text("#!/usr/bin/env python3\nimport json\nprint(json.dumps({'ok': True, 'data': {'tasks': [{'taskId': 'a'}, {'taskId': 'b'}]}}))\n", encoding="utf-8")
        ambiguous.chmod(0o755)
        ambiguous_proc = execute(["--action", "reject", "--instance-ids", "ambiguous", "--yes", "--format", "json"], {**env, "PATH": f"{ambiguous_bin}{os.pathsep}{env['PATH']}"})
        payload, detail = one_envelope(ambiguous_proc)
        ambiguous_ok = ambiguous_proc.returncode == 1 and payload is not None and payload.get("data", {}).get("failed", [{}])[0].get("error", {}).get("type") == "precondition"
        outcomes.append(("多个 taskId 不任选其一执行", "PASS" if ambiguous_ok else "FAIL", f"rc={ambiguous_proc.returncode}; {detail}"))

    passed = sum(1 for _, status, _ in outcomes if status == "PASS")
    lines = [
        "# Multi OA 批量审批 Agent 语义探针", "",
        "临时 child runner 仅用于本地分类与安全语义；本报告不保存 JSON fixture，也不证明真实审批终态。", "",
        "| 检查 | 结果 | 证据 |", "|---|---|---|",
    ]
    for name, status, detail in outcomes:
        lines.append("| {} | {} | {} |".format(name, status, detail.replace("|", "\\|")))
    lines.extend(["", f"结论：**{passed}/{len(outcomes)} PASS**。", "", "范围：验证 Help、确认门禁、零写预览、唯一 taskId 选择和部分结果表达；审批动作已受理与实例终态仍分层表达，真实审批终态须用隔离账号复验。", ""])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(outcomes) else 1


if __name__ == "__main__":
    raise SystemExit(main())
