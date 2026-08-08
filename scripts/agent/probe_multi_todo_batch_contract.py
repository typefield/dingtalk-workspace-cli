#!/usr/bin/env python3
"""Agent semantic probe for the Multi Todo batch-create script.

It is intentionally not a CI gate.  Fake commands and inputs live only in a
temporary directory; an optional report is Markdown evidence, never a stored
JSON fixture or a substitute for real DingTalk terminal verification.
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
SCRIPT = ROOT / "skills" / "multi" / "dingtalk-todo" / "scripts" / "todo_batch_create.py"


def execute(args: list[str], env: dict[str, str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(SCRIPT), *args], cwd=ROOT, env=env, capture_output=True, text=True, timeout=30)


def one_envelope(proc: subprocess.CompletedProcess[str]) -> tuple[dict[str, Any] | None, str]:
    lines = [line for line in proc.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None, f"stdout lines={len(lines)}"
    try:
        value = json.loads(lines[0])
    except json.JSONDecodeError as exc:
        return None, f"invalid JSON: {exc}"
    return (value, "ok") if isinstance(value, dict) else (None, "result is not an object")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    outcomes: list[tuple[str, str, str]] = []
    with tempfile.TemporaryDirectory(prefix="dws-multi-todo-probe-") as directory:
        tmp = Path(directory)
        bin_dir = tmp / "bin"
        bin_dir.mkdir()
        sentinel = tmp / "unexpected-dws-call"
        fake = bin_dir / "dws"
        fake.write_text(f"#!/bin/sh\ntouch '{sentinel}'\nexit 99\n", encoding="utf-8")
        fake.chmod(0o755)
        env = {**os.environ, "PATH": f"{bin_dir}{os.pathsep}{os.environ.get('PATH', '')}"}

        valid = tmp / "todos.json"
        valid.write_text('[{"title":"first","executors":"user_a"},{"title":"second","executors":"user_b"}]', encoding="utf-8")
        invalid = tmp / "invalid.json"
        invalid.write_text('[{"title":"bad","executors":["user_a"]}]', encoding="utf-8")

        help_proc = execute(["--help"], env)
        help_ok = help_proc.returncode == 0 and "--format {text,json,ndjson}" in help_proc.stdout and "--dry-run" in help_proc.stdout
        outcomes.append(("脚本 Help 可发现 format/dry-run", "PASS" if help_ok else "FAIL", f"rc={help_proc.returncode}"))

        dry_run = execute([str(valid), "--dry-run", "--format", "json"], env)
        payload, detail = one_envelope(dry_run)
        dry_run_ok = dry_run.returncode == 0 and payload is not None and payload.get("ok") is True and payload.get("outcome") == "success" and payload.get("dry_run") is True
        outcomes.append(("dry-run 不调用 dws 且返回单信封", "PASS" if dry_run_ok and not sentinel.exists() else "FAIL", f"rc={dry_run.returncode}; {detail}; sentinel={sentinel.exists()}"))

        type_error = execute([str(invalid), "--format", "json"], env)
        payload, detail = one_envelope(type_error)
        type_error_ok = type_error.returncode == 1 and payload is not None and payload.get("ok") is False and payload.get("outcome") == "failure" and payload.get("error", {}).get("type") == "validation" and "Traceback" not in type_error.stderr
        outcomes.append(("executors 类型错误不泄漏 traceback", "PASS" if type_error_ok else "FAIL", f"rc={type_error.returncode}; {detail}"))

        batch_bin = tmp / "batch-bin"
        batch_bin.mkdir()
        counter = tmp / "calls"
        batch_dws = batch_bin / "dws"
        batch_dws.write_text(
            "#!/usr/bin/env python3\n"
            "import json, os, sys\n"
            "counter = os.environ['PROBE_TODO_COUNTER']\n"
            "try:\n  count = int(open(counter, encoding='utf-8').read())\nexcept FileNotFoundError:\n  count = 0\n"
            "count += 1\nopen(counter, 'w', encoding='utf-8').write(str(count))\n"
            "if sys.argv[1:4] == ['todo', 'task', 'get']:\n  print(json.dumps({'ok': True, 'data': {'taskId': 'task_1'}}))\n"
            "elif count == 1:\n  print(json.dumps({'ok': True, 'data': {'todoTaskId': 'task_1'}, 'meta': {'request': 'accepted'}}))\n"
            "else:\n  print(json.dumps({'success': False, 'error': {'type': 'api', 'message': 'ambiguous task create'}}))\n",
            encoding="utf-8",
        )
        batch_dws.chmod(0o755)
        partial = execute([str(valid), "--format", "json"], {**env, "PATH": f"{batch_bin}{os.pathsep}{env['PATH']}", "PROBE_TODO_COUNTER": str(counter)})
        payload, detail = one_envelope(partial)
        partial_ok = (
            partial.returncode == 7 and payload is not None and payload.get("ok") is False
            and payload.get("outcome") == "partial_failure"
            and payload.get("data", {}).get("succeeded", [{}])[0].get("id") == "1"
            and payload.get("data", {}).get("unknown", [{}])[0].get("id") == "2"
            and payload.get("data", {}).get("succeeded", [{}])[0].get("verification", {}).get("state") == "verified"
        )
        outcomes.append(("逐项写入保留成功、未知与未验证事实", "PASS" if partial_ok else "FAIL", f"rc={partial.returncode}; {detail}"))

    passed = sum(1 for _, status, _ in outcomes if status == "PASS")
    lines = [
        "# Multi Todo 批量创建 Agent 语义探针", "",
        "临时 child runner 与输入只在本次执行期间存在；本报告不保存 JSON fixture，也不证明真实服务端终态。", "",
        "| 检查 | 结果 | 证据 |", "|---|---|---|",
    ]
    for name, status, detail in outcomes:
        lines.append("| {} | {} | {} |".format(name, status, detail.replace("|", "\\|")))
    lines.extend(["", f"结论：**{passed}/{len(outcomes)} PASS**。", "", "范围：验证可发现性、机器错误边界、零写预览及批量三通道表达；非 dry-run 的成功项只有在 `task get` 回读后才标为 `verification.state=verified`。", ""])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(outcomes) else 1


if __name__ == "__main__":
    raise SystemExit(main())
