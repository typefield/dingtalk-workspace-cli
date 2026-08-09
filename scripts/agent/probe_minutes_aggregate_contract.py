#!/usr/bin/env python3
"""Agent probe for Mono/Multi Minutes aggregate result truthfulness."""

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
    ("mono-summary", ROOT / "skills/mono/scripts/minutes_recent_summary.py"),
    ("mono-todos", ROOT / "skills/mono/scripts/minutes_extract_todos.py"),
    ("multi-summary", ROOT / "skills/multi/dingtalk-minutes/scripts/minutes_recent_summary.py"),
    ("multi-todos", ROOT / "skills/multi/dingtalk-minutes/scripts/minutes_extract_todos.py"),
)


FAKE_DWS = r'''#!/usr/bin/env python3
import json
import os
import pathlib
import sys

marker = pathlib.Path(os.environ["DWS_PROBE_MARKER"])
marker.write_text(marker.read_text() + "call\n" if marker.exists() else "call\n")
args = sys.argv[1:]
mode = os.environ["DWS_PROBE_MODE"]

def envelope(data, name):
    print(json.dumps({"ok": True, "outcome": "success", "data": data, "meta": {"probe": name}}))

if args[:3] == ["minutes", "list", "mine"]:
    if mode == "malformed_list":
        envelope({"itemList": [{"id": "display-only"}]}, "list")
    else:
        envelope({"itemList": [
            {"taskUuid": "minute-a", "title": "A"},
            {"taskUuid": "minute-b", "title": "B"},
        ]}, "list")
    raise SystemExit(0)

identifier = args[args.index("--id") + 1]
if mode == "partial" and identifier == "minute-b":
    print(json.dumps({
        "ok": False,
        "outcome": "failure",
        "error": {"type": "auth", "subtype": "probe_denied", "message": "denied"},
        "meta": {"probe": identifier},
    }))
    raise SystemExit(2)

if args[:3] == ["minutes", "get", "summary"]:
    if mode == "projection_partial" and identifier == "minute-b":
        envelope({"result": {"unexpected": True}}, identifier)
    else:
        envelope({"result": {"fullSummary": f"summary {identifier}"}}, identifier)
    raise SystemExit(0)

if args[:3] == ["minutes", "get", "todos"]:
    if mode == "projection_partial" and identifier == "minute-b":
        envelope({"result": {"unexpected": True}}, identifier)
    else:
        envelope({"result": {"dingtalkTodoList": [{"title": f"todo {identifier}"}]}}, identifier)
    raise SystemExit(0)

raise SystemExit(9)
'''


def parse_result(completed: subprocess.CompletedProcess[str]) -> dict[str, Any] | None:
    lines = [line for line in completed.stdout.splitlines() if line.strip()]
    if len(lines) != 1:
        return None
    try:
        payload = json.loads(lines[0])
    except json.JSONDecodeError:
        return None
    return payload if isinstance(payload, dict) else None


def run_script(
    script: Path,
    mode: str,
    fake_dir: Path,
    marker: Path,
    *,
    fmt: str = "json",
    extra: tuple[str, ...] = (),
) -> subprocess.CompletedProcess[str]:
    marker.unlink(missing_ok=True)
    env = os.environ.copy()
    env["PATH"] = f"{fake_dir}{os.pathsep}{env.get('PATH', '')}"
    env["DWS_PROBE_MODE"] = mode
    env["DWS_PROBE_MARKER"] = str(marker)
    return subprocess.run(
        [sys.executable, str(script), "--max", "2", *extra, "--format", fmt],
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

    with tempfile.TemporaryDirectory(prefix="dws-minutes-aggregate-") as temp_name:
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
                and payload.get("data", {}).get("count") == 2
                and len(payload.get("meta", {}).get("children", [])) == 3
            )
            checks.append((f"{label}: 全成功保留 child meta", success_ok, f"rc={success.returncode}"))

            partial = run_script(script, "partial", fake_dir, marker)
            payload = parse_result(partial)
            data = payload.get("data", {}) if payload else {}
            partial_ok = (
                partial.returncode == 7
                and payload is not None
                and payload.get("outcome") == "partial_failure"
                and data.get("succeeded", [{}])[0].get("id") == "minute-a"
                and data.get("failed", [{}])[0].get("id") == "minute-b"
                and data.get("failed", [{}])[0].get("error", {}).get("type") == "auth"
            )
            checks.append((f"{label}: 单项失败保留成功项", partial_ok, f"rc={partial.returncode}"))

            projection = run_script(script, "projection_partial", fake_dir, marker)
            payload = parse_result(projection)
            data = payload.get("data", {}) if payload else {}
            projection_ok = (
                projection.returncode == 7
                and payload is not None
                and payload.get("outcome") == "partial_failure"
                and data.get("failed", [{}])[0].get("error", {}).get("subtype") == "projection_unknown"
            )
            checks.append((f"{label}: 单项投影漂移不伪装空内容", projection_ok, f"rc={projection.returncode}"))

            malformed = run_script(script, "malformed_list", fake_dir, marker)
            payload = parse_result(malformed)
            malformed_ok = (
                malformed.returncode == 1
                and payload is not None
                and payload.get("outcome") == "failure"
                and payload.get("error", {}).get("subtype") == "projection_unknown"
            )
            checks.append((f"{label}: display-only id 列表 fail-closed", malformed_ok, f"rc={malformed.returncode}"))

            partial_text = run_script(script, "partial", fake_dir, marker, fmt="text")
            text_ok = partial_text.returncode == 7 and "部分听记" in partial_text.stdout
            checks.append((f"{label}: text 模式保持 partial rc=7", text_ok, f"rc={partial_text.returncode}"))

            if label.endswith("todos"):
                dry = run_script(
                    script,
                    "success",
                    fake_dir,
                    marker,
                    extra=("--id", "minute-a", "--dry-run"),
                )
                payload = parse_result(dry)
                dry_ok = (
                    dry.returncode == 0
                    and payload is not None
                    and payload.get("dry_run") is True
                    and not marker.exists()
                )
                checks.append((f"{label}: 指定 ID dry-run 零 child 调用", dry_ok, f"rc={dry.returncode}"))

    passed = sum(ok for _, ok, _ in checks)
    lines = [
        "# Minutes 聚合脚本结果契约 Agent 探针",
        "",
        f"扫描日期：{date.today().isoformat()}",
        "",
        "> 以临时假 dws 对拍 Mono/Multi 四个入口；不保存 JSON fixture，不证明真实听记内容或服务端终态。",
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
        "范围：证明列表/逐项读取失败、投影漂移、partial/text 退出码、meta 与指定 ID dry-run 的本地编排；索引覆盖、分页耗尽和真实内容仍需 live evidence。",
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
