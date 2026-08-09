#!/usr/bin/env python3
"""Read-only live review for Todo aggregate unified results.

The probe builds the current source in a temporary directory, executes only
the two read-only aggregate shortcuts, and writes a redacted Markdown summary.
Raw Todo payloads are parsed in memory and are never persisted as fixtures.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from typing import Any


ROOT = Path(__file__).resolve().parents[2]
COMMANDS = ("+due-today", "+related-tasks")


def run_command(binary: Path, command: str) -> tuple[bool, int, str, int]:
    completed = subprocess.run(
        [str(binary), "todo", command, "--format", "json"],
        cwd=ROOT,
        text=True,
        capture_output=True,
        timeout=90,
    )
    try:
        payload: Any = json.loads(completed.stdout)
    except json.JSONDecodeError:
        return False, completed.returncode, "stdout 不是单一 JSON 对象", 0
    if not isinstance(payload, dict):
        return False, completed.returncode, "stdout 顶层不是对象", 0
    data = payload.get("data")
    meta = payload.get("meta")
    tasks = data.get("tasks") if isinstance(data, dict) else None
    count = data.get("count") if isinstance(data, dict) else None
    valid_ids = isinstance(tasks, list) and all(
        isinstance(item, dict)
        and isinstance(item.get("taskId"), str)
        and bool(item["taskId"].strip())
        for item in tasks
    )
    valid = (
        completed.returncode == 0
        and completed.stderr == ""
        and payload.get("ok") is True
        and payload.get("outcome") == "success"
        and "contract_version" not in payload
        and isinstance(data, dict)
        and data.get("pagination_known") is False
        and isinstance(count, int)
        and count == len(tasks) if isinstance(tasks, list) else False
    )
    valid = bool(
        valid
        and valid_ids
        and isinstance(meta, dict)
        and meta.get("count") == count
        and "pagination" not in meta
    )
    detail = (
        "统一 success；count 对齐；稳定 taskId；pagination_known=false，且未伪造 endpoint 耗尽"
        if valid
        else "结果信封、count、taskId 或分页边界不符合统一契约"
    )
    return valid, completed.returncode, detail, count if isinstance(count, int) else 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output path")
    args = parser.parse_args()

    env = os.environ.copy()
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    rows: list[tuple[str, bool, int, str, int]] = []
    with tempfile.TemporaryDirectory(prefix="dws-todo-live-agent-") as temp_dir:
        binary = Path(temp_dir) / "dws"
        build = subprocess.run(
            ["go", "build", "-o", str(binary), "./cmd"],
            cwd=ROOT,
            env=env,
            text=True,
            capture_output=True,
            timeout=180,
        )
        if build.returncode != 0:
            print(build.stderr, file=sys.stderr)
            return build.returncode
        for command in COMMANDS:
            ok, rc, detail, count = run_command(binary, command)
            rows.append((command, ok, rc, detail, count))

    passed = sum(1 for _, ok, _, _, _ in rows if ok)
    lines = [
        "# Todo 聚合统一结果真实只读 Agent 复验",
        "",
        f"扫描时间：{dt.datetime.now().astimezone().isoformat(timespec='seconds')}",
        "",
        "> 本扫描使用当前源码临时构建二进制，只执行 Todo 只读命令。原始响应只在内存解析，仓库仅保存脱敏 Markdown；不保存 JSON fixture。",
        "",
        "| 命令 | 结果 | rc | 投影条数 | 证据 |",
        "|---|---|---:|---:|---|",
    ]
    for command, ok, rc, detail, count in rows:
        lines.append(f"| `dws todo {command} --format json` | {'PASS' if ok else 'REVIEW'} | {rc} | {count} | {detail} |")
    lines.extend(
        [
            "",
            f"结果：**{passed}/{len(rows)} PASS**。",
            "",
            "## 边界",
            "",
            "- 本证据证明当前账号下两个读取入口能投影真实响应并直接产生统一结果；不证明 Todo 服务端短页等于权威终页。",
            "- 因上游没有可信 continuation，结果必须保留 `pagination_known:false`，且不得输出 `meta.pagination.endpoint_exhausted:true`。",
            "- 本扫描不创建、修改、完成或删除待办，也不证明其他账号、组织权限或后端覆盖率。",
            "",
        ]
    )
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text("\n".join(lines), encoding="utf-8")
    return 0 if passed == len(rows) else 1


if __name__ == "__main__":
    raise SystemExit(main())
