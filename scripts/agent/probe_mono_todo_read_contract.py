#!/usr/bin/env python3
"""Agent probe for Mono/Multi Todo read pagination and child-result preservation."""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import tempfile
from datetime import date
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = (
    ("mono-daily", ROOT / "skills/mono/scripts/todo_daily_summary.py"),
    ("mono-overdue", ROOT / "skills/mono/scripts/todo_overdue_check.py"),
    ("multi-daily", ROOT / "skills/multi/dingtalk-todo/scripts/todo_daily_summary.py"),
    ("multi-overdue", ROOT / "skills/multi/dingtalk-todo/scripts/todo_overdue_check.py"),
)


FAKE_DWS = r'''#!/usr/bin/env python3
import json
import os
import pathlib
import sys

marker = pathlib.Path(os.environ["DWS_PROBE_MARKER"])
marker.write_text(marker.read_text() + "call\n" if marker.exists() else "call\n")
args = sys.argv[1:]
page = int(args[args.index("--page") + 1])
mode = os.environ["DWS_PROBE_MODE"]
meta = {"pagination": {"page": page, "source": "probe"}}

if mode == "first_failure" or (mode == "late_failure" and page == 2):
    print(json.dumps({
        "ok": False,
        "outcome": "failure",
        "error": {"type": "auth", "subtype": "probe_auth", "message": "probe denied"},
        "meta": meta,
    }))
    raise SystemExit(2)

count = 50 if mode == "cap" or (mode == "late_failure" and page == 1) else 1
items = [{"id": f"todo-{page}-{i}", "title": f"todo {i}"} for i in range(count)]
print(json.dumps({
    "ok": True,
    "outcome": "success",
    "data": {"todoCards": items},
    "meta": meta,
}))
'''


def parse_result(completed: subprocess.CompletedProcess[str]) -> dict[str, Any] | None:
    lines = [line for line in completed.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None
    try:
        value = json.loads(lines[0])
    except json.JSONDecodeError:
        return None
    return value if isinstance(value, dict) else None


def run_script(
    script: Path,
    mode: str,
    fake_dir: Path,
    marker: Path,
    *extra: str,
    fmt: str = "json",
) -> subprocess.CompletedProcess[str]:
    marker.unlink(missing_ok=True)
    env = os.environ.copy()
    env["PATH"] = f"{fake_dir}{os.pathsep}{env.get('PATH', '')}"
    env["DWS_PROBE_MODE"] = mode
    env["DWS_PROBE_MARKER"] = str(marker)
    return subprocess.run(
        [sys.executable, str(script), *extra, "--format", fmt],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=30,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    checks: list[tuple[str, bool, str]] = []

    with tempfile.TemporaryDirectory(prefix="dws-mono-todo-read-") as temp_name:
        fake_dir = Path(temp_name)
        fake = fake_dir / "dws"
        fake.write_text(FAKE_DWS, encoding="utf-8")
        fake.chmod(0o755)
        marker = fake_dir / "calls"

        for label, script in SCRIPTS:

            success = run_script(script, "success", fake_dir, marker)
            payload = parse_result(success)
            success_ok = (
                success.returncode == 0
                and payload is not None
                and payload.get("ok") is True
                and payload.get("outcome") == "success"
                and payload.get("meta", {}).get("children", [{}])[0].get("id") == "page:1"
            )
            checks.append((f"{label}: 短页成功保留 child meta", success_ok, f"rc={success.returncode}"))

            partial = run_script(script, "late_failure", fake_dir, marker)
            payload = parse_result(partial)
            data = payload.get("data", {}) if payload else {}
            partial_ok = (
                partial.returncode == 7
                and payload is not None
                and payload.get("ok") is False
                and payload.get("outcome") == "partial_failure"
                and data.get("succeeded", [{}])[0].get("id") == "page:1"
                and data.get("failed", [{}])[0].get("id") == "page:2"
                and data.get("failed", [{}])[0].get("error", {}).get("type") == "auth"
            )
            checks.append((f"{label}: 后续页失败不伪装完整成功", partial_ok, f"rc={partial.returncode}"))

            partial_text = run_script(
                script, "late_failure", fake_dir, marker, fmt="text",
            )
            partial_text_ok = (
                partial_text.returncode == 7
                and "仅完成部分待办分页读取" in partial_text.stdout
            )
            checks.append((f"{label}: text 模式保持 partial rc=7", partial_text_ok, f"rc={partial_text.returncode}"))

            capped = run_script(script, "cap", fake_dir, marker)
            payload = parse_result(capped)
            data = payload.get("data", {}) if payload else {}
            cap_ok = (
                capped.returncode == 7
                and payload is not None
                and payload.get("outcome") == "partial_failure"
                and data.get("unknown", [{}])[0].get("id") == "page:11"
            )
            checks.append((f"{label}: 达到硬上限不宣称完整", cap_ok, f"rc={capped.returncode}"))

            failure = run_script(script, "first_failure", fake_dir, marker)
            payload = parse_result(failure)
            failure_ok = (
                failure.returncode == 1
                and payload is not None
                and payload.get("ok") is False
                and payload.get("outcome") == "failure"
                and payload.get("error", {}).get("type") == "auth"
            )
            checks.append((f"{label}: 首页 typed failure 原样分类", failure_ok, f"rc={failure.returncode}"))

            dry = run_script(script, "success", fake_dir, marker, "--dry-run")
            payload = parse_result(dry)
            dry_ok = (
                dry.returncode == 0
                and payload is not None
                and payload.get("dry_run") is True
                and not marker.exists()
            )
            checks.append((f"{label}: dry-run 不启动 child dws", dry_ok, f"rc={dry.returncode}"))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        "# Mono/Multi Todo 只读分页结果契约 Agent 探针",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 使用临时假 dws 验证聚合边界；不保存 JSON fixture，也不替代真实服务端分页取证。",
        "",
        "| 检查 | 结果 | 证据 |",
        "|---|---|---|",
    ]
    lines.extend(
        f"| {name} | {'PASS' if ok else 'FAIL'} | {detail} |"
        for name, ok, detail in checks
    )
    lines.extend([
        "",
        f"结论：**{passed}/{len(checks)} PASS**。",
        "",
        "范围：证明 Mono/Multi 四个 Todo 汇总入口不会把后续页失败压成完整成功，并保留 child meta；端点真实耗尽、数据覆盖率与服务端终态仍需 live evidence。",
        "",
    ])
    report = "\n".join(lines)
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(report, encoding="utf-8")
    else:
        print(report)
    return 0 if passed == len(checks) else 1


if __name__ == "__main__":
    raise SystemExit(main())
