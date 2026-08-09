#!/usr/bin/env python3
"""Cross-Skill Agent semantic probe for Multi script result boundaries.

This is deliberately evidence rather than a CI gate.  Each probe starts a
short-lived Python process for the local runtime under test.  It writes only
Markdown when requested, never a JSON fixture; no DingTalk command is used.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import subprocess
import sys
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
RUNTIMES = {
    "AITable": (ROOT / "skills" / "multi" / "dingtalk-aitable" / "scripts", True),
    "Todo": (ROOT / "skills" / "multi" / "dingtalk-todo" / "scripts", True),
    "Misc": (ROOT / "skills" / "multi" / "dingtalk-misc" / "scripts", True),
    "Shared": (ROOT / "skills" / "multi" / "dingtalk-shared" / "scripts", True),
}


def run(directory: Path, code: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, "-c", code], cwd=ROOT, capture_output=True, text=True, timeout=30)


def exactly_one_envelope(value: str) -> dict[str, Any] | None:
    lines = [line for line in value.splitlines() if line.strip()]
    if len(lines) != 1:
        return None
    try:
        payload = json.loads(lines[0])
    except json.JSONDecodeError:
        return None
    return payload if isinstance(payload, dict) else None


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, help="write Markdown report; default is stdout")
    args = parser.parse_args()
    checks: list[tuple[str, str, str]] = []
    for name, (directory, has_batch_helpers) in RUNTIMES.items():
        exception = run(directory, "\n".join([
            "import sys",
            f"sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
            "sys.argv = ['probe', '--format', 'json']",
            "def boom(): raise AttributeError('probe secret')",
            "raise SystemExit(_runtime.run_main(boom))",
        ]))
        payload = exactly_one_envelope(exception.stdout)
        exception_ok = (
            exception.returncode == 1 and payload is not None
            and payload.get("ok") is False and payload.get("outcome") == "failure"
            and payload.get("error", {}).get("type") == "internal"
            and "Traceback" not in exception.stderr and "probe secret" not in exception.stderr
        )
        checks.append((f"{name}: 未捕获异常不泄漏 traceback", "PASS" if exception_ok else "FAIL", f"rc={exception.returncode}; single_envelope={payload is not None}"))

        child = run(directory, "\n".join([
            "import json, sys",
            f"sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
            "result = _runtime.run_child_dws(['-c', \"import json; print(json.dumps({'success': False, 'error': {'type': 'api', 'message': 'legacy false'}}))\"], executable=sys.executable)",
            "print(json.dumps({'state': result.state, 'error_type': (result.error or {}).get('type')}))",
        ]))
        try:
            child_payload = json.loads(child.stdout)
        except json.JSONDecodeError:
            child_payload = None
        child_ok = child.returncode == 0 and child_payload == {"state": "unknown", "error_type": "api"}
        checks.append((f"{name}: 旧 success:false 不按 truthiness 成功", "PASS" if child_ok else "FAIL", f"rc={child.returncode}; payload={child_payload}"))

        untyped = run(directory, "\n".join([
            "import json, sys",
            f"sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
            "result = _runtime.run_child_dws(['-c', \"import json; print(json.dumps({'success': 'false'}))\"], executable=sys.executable)",
            "print(json.dumps({'state': result.state, 'subtype': (result.error or {}).get('subtype')}))",
        ]))
        try:
            untyped_payload = json.loads(untyped.stdout)
        except json.JSONDecodeError:
            untyped_payload = None
        untyped_ok = untyped.returncode == 0 and untyped_payload == {"state": "unknown", "subtype": "untyped_status"}
        checks.append((f"{name}: success 字符串不伪装执行成功", "PASS" if untyped_ok else "FAIL", f"rc={untyped.returncode}; payload={untyped_payload}"))

        inconsistent = run(directory, "\n".join([
            "import json, sys",
            f"sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
            "result = _runtime.run_child_dws(['-c', \"import json; print(json.dumps({'ok': True, 'outcome': 'failure'}))\"], executable=sys.executable)",
            "print(json.dumps({'state': result.state, 'subtype': (result.error or {}).get('subtype')}))",
        ]))
        try:
            inconsistent_payload = json.loads(inconsistent.stdout)
        except json.JSONDecodeError:
            inconsistent_payload = None
        inconsistent_ok = inconsistent.returncode == 0 and inconsistent_payload == {"state": "unknown", "subtype": "untyped_status"}
        checks.append((f"{name}: 矛盾 ok/outcome 不伪装执行成功", "PASS" if inconsistent_ok else "FAIL", f"rc={inconsistent.returncode}; payload={inconsistent_payload}"))

        malformed = run(directory, "\n".join([
            "import json, sys",
            f"sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
            "result = _runtime.run_child_dws(['-c', \"import json; print(json.dumps({'ok': True, 'outcome': []}))\"], executable=sys.executable)",
            "print(json.dumps({'state': result.state, 'subtype': (result.error or {}).get('subtype')}))",
        ]))
        try:
            malformed_payload = json.loads(malformed.stdout)
        except json.JSONDecodeError:
            malformed_payload = None
        malformed_ok = malformed.returncode == 0 and malformed_payload == {"state": "unknown", "subtype": "untyped_status"}
        checks.append((f"{name}: 非字符串 outcome 不泄漏异常或伪装成功", "PASS" if malformed_ok else "FAIL", f"rc={malformed.returncode}; payload={malformed_payload}"))

        pending = run(directory, "\n".join([
            "import json, sys",
            f"sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
            "result = _runtime.run_child_dws(['-c', \"import json; print(json.dumps({'ok': True, 'outcome': 'pending', 'meta': {'operation': {'id': 'task-1'}}}))\"], executable=sys.executable)",
            "print(json.dumps({'state': result.state, 'subtype': (result.error or {}).get('subtype'), 'meta': result.meta}))",
        ]))
        try:
            pending_payload = json.loads(pending.stdout)
        except json.JSONDecodeError:
            pending_payload = None
        pending_ok = pending.returncode == 0 and pending_payload == {
            "state": "unknown",
            "subtype": "operation_pending",
            "meta": {"operation": {"id": "task-1"}},
        }
        checks.append((f"{name}: pending 不伪装终态成功且保留任务 meta", "PASS" if pending_ok else "FAIL", f"rc={pending.returncode}; payload={pending_payload}"))

        untyped_failure = run(directory, "\n".join([
            "import json, sys",
            f"sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
            "sys.argv = ['probe', '--format', 'json']",
            "raise SystemExit(_runtime.run_main(lambda: (print(json.dumps({'ok': False, 'outcome': 'failure'})), 1)[1]))",
        ]))
        untyped_failure_payload = exactly_one_envelope(untyped_failure.stdout)
        untyped_failure_ok = (
            untyped_failure.returncode == 1
            and untyped_failure_payload is not None
            and untyped_failure_payload.get("error", {}).get("type") == "internal"
            and untyped_failure_payload.get("error", {}).get("details", {}).get("violation") == "machine_stdout_contract"
        )
        checks.append((f"{name}: failure 缺 typed error 会被统一出口拒绝", "PASS" if untyped_failure_ok else "FAIL", f"rc={untyped_failure.returncode}; payload={untyped_failure_payload}"))

        malformed_metadata = run(directory, "\n".join([
            "import json, sys",
            f"sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
            "sys.argv = ['probe', '--format', 'json']",
            "raise SystemExit(_runtime.run_main(lambda: (print(json.dumps({'ok': True, 'outcome': 'success', 'meta': [], 'dry_run': 'true'})), 0)[1]))",
        ]))
        malformed_metadata_payload = exactly_one_envelope(malformed_metadata.stdout)
        malformed_metadata_ok = (
            malformed_metadata.returncode == 1
            and malformed_metadata_payload is not None
            and malformed_metadata_payload.get("error", {}).get("type") == "internal"
            and malformed_metadata_payload.get("error", {}).get("details", {}).get("violation") == "machine_stdout_contract"
        )
        checks.append((f"{name}: meta/dry_run 非法类型会被统一出口拒绝", "PASS" if malformed_metadata_ok else "FAIL", f"rc={malformed_metadata.returncode}; payload={malformed_metadata_payload}"))

        partial_lines = [
            f"import sys; sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
        ]
        if has_batch_helpers:
            partial_lines.extend([
                "data = _runtime.batch_data(succeeded=[{'id': 'ok'}], unknown=[{'id': 'maybe', 'reason': 'ambiguous'}], total=2)",
                "outcome = _runtime.batch_outcome(data)",
            ])
        else:
            partial_lines.extend([
                "data = {'total': 2, 'succeeded': [{'id': 'ok'}], 'failed': [], 'unknown': [{'id': 'maybe', 'reason': 'ambiguous'}]}",
                "outcome = 'partial_failure'",
            ])
        partial_lines.append("raise SystemExit(_runtime.emit(fmt='json', outcome=outcome, data=data))")
        partial = run(directory, "\n".join(partial_lines))
        payload = exactly_one_envelope(partial.stdout)
        partial_ok = (
            partial.returncode == 7 and payload is not None and payload.get("ok") is False
            and payload.get("outcome") == "partial_failure"
            and payload.get("data", {}).get("succeeded", [{}])[0].get("id") == "ok"
            and payload.get("data", {}).get("unknown", [{}])[0].get("id") == "maybe"
        )
        checks.append((f"{name}: partial_failure 保留三通道并返回 rc=7", "PASS" if partial_ok else "FAIL", f"rc={partial.returncode}; single_envelope={payload is not None}"))

    passed = sum(1 for _, status, _ in checks if status == "PASS")
    lines = [
        "# Multi 结果边界 Agent 语义对拍", "",
        "各运行时仅在临时 Python 子进程中加载；本报告不保存 JSON fixture，不调用 dws，也不替代真实服务端终态验证。", "",
        "| 检查 | 结果 | 证据 |", "|---|---|---|",
    ]
    for check, status, detail in checks:
        lines.append("| {} | {} | {} |".format(check, status, detail.replace("|", "\\|")))
    lines.extend(["", f"结论：**{passed}/{len(checks)} PASS**。", "", "范围：横向验证局部与 shared 运行时的异常边界、历史字符串布尔失败分类和 partial_failure/rc=7 机器契约；业务写入、分页和服务端终态仍由各产品的 Agent 探针与真实环境证据负责。", ""])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(checks) else 1


if __name__ == "__main__":
    raise SystemExit(main())
