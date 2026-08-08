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
    "AITable": ROOT / "skills" / "multi" / "dingtalk-aitable" / "scripts",
    "Todo": ROOT / "skills" / "multi" / "dingtalk-todo" / "scripts",
    "Misc": ROOT / "skills" / "multi" / "dingtalk-misc" / "scripts",
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
    for name, directory in RUNTIMES.items():
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

        partial = run(directory, "\n".join([
            f"import sys; sys.path.insert(0, {str(directory)!r})",
            "import _runtime",
            "data = _runtime.batch_data(succeeded=[{'id': 'ok'}], unknown=[{'id': 'maybe', 'reason': 'ambiguous'}], total=2)",
            "raise SystemExit(_runtime.emit(fmt='json', outcome=_runtime.batch_outcome(data), data=data))",
        ]))
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
        "# Multi 本地结果边界 Agent 语义对拍", "",
        "各运行时仅在临时 Python 子进程中加载；本报告不保存 JSON fixture，不调用 dws，也不替代真实服务端终态验证。", "",
        "| 检查 | 结果 | 证据 |", "|---|---|---|",
    ]
    for check, status, detail in checks:
        lines.append("| {} | {} | {} |".format(check, status, detail.replace("|", "\\|")))
    lines.extend(["", f"结论：**{passed}/{len(checks)} PASS**。", "", "范围：横向验证异常边界、历史字符串布尔失败分类和批量部分成功的机器契约；业务写入、分页和服务端终态仍由各产品的 Agent 探针与真实环境证据负责。", ""])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(checks) else 1


if __name__ == "__main__":
    raise SystemExit(main())
