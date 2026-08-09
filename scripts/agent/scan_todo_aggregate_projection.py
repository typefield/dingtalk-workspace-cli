#!/usr/bin/env python3
"""Agent-only review of Todo smart-shortcut aggregate projection.

The review deliberately produces Markdown evidence rather than a CI gate or a
saved response fixture. It confirms that every public aggregate route shares
the strict todoCards projection and runs its in-memory regression cases.
"""

from __future__ import annotations

import argparse
import datetime as dt
import os
from pathlib import Path
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
SHARED = ROOT / "internal/shortcut/smart/todo_shared.go"
ROUTES = {
    "todo +related-tasks": ROOT / "internal/shortcut/smart/related_tasks.go",
    "todo +due-today": ROOT / "internal/shortcut/smart/due_today.go",
    "todo +created-todos": ROOT / "internal/shortcut/smart/created_todos.go",
    "todo +overdue": ROOT / "internal/shortcut/smart/overdue.go",
    "todo +todo-done": ROOT / "internal/shortcut/smart/todo_done.go",
}
PATTERN = r"TestShortcutTodoCards|TestCreatedTodosRejectsUnknownTodoProjection|TestTodoAggregateShortcutsHaveReadOnlyContracts"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output path")
    args = parser.parse_args()

    shared = SHARED.read_text(encoding="utf-8")
    shared_ok = "cards, err := shortcutTodoCards(data)" in shared
    missing_routes = [
        route for route, path in ROUTES.items()
        if "shortcutListAllTodoCards(rt," not in path.read_text(encoding="utf-8")
    ]
    env = os.environ.copy()
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    command = ["go", "test", "-count=1", "./internal/shortcut/smart", "-run", PATTERN, "-v"]
    result = subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True)
    passed = result.returncode == 0 and shared_ok and not missing_routes

    transcript = (result.stdout + result.stderr).strip()
    if len(transcript) > 3000:
        transcript = transcript[-3000:]
    missing = ", ".join(f"`{route}`" for route in missing_routes) or "无"
    report = f"""# Todo 聚合快捷命令投影 — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本扫描由 Agent 在当前工作树运行。它仅以内存 Go 测试和源码入口关系作取证，输出 Markdown；不是 CI / policy gate，且不保存响应 JSON fixture。

## Result: {"PASS" if passed else "REVIEW"}

- 共享分页器先验证 `todoCards`：`{"yes" if shared_ok else "no"}`
- 已复核公共聚合入口：**{len(ROUTES) - len(missing_routes)}/{len(ROUTES)}**
- 缺少共享分页器的入口：{missing}
- 焦点测试退出码：`{result.returncode}`

## Required behavior

1. 只有明确的 `todoCards: []` 可表达已知空列表。
2. 缺少容器、容器非数组、非对象行或缺稳定 `taskId` 必须 fail-closed 为不可重试的 `api/projection_unknown`，阶段为 `response_projection`。
3. `todo +todo-done` 必须在这种投影失败时停止，不能把“未找到待办”升级成写操作。
4. `+related-tasks`、`+due-today`、`+created-todos`、`+overdue` 不得将未知上游形状表达成空集合。

## Focused test transcript

```text
{transcript or "(no output)"}
```

## Boundary

本地证据仅证明 CLI 不会将异常响应形状伪装为“没有待办”。真实分页、组织权限和写后终态仍须用隔离账号单独由 Agent 取证。
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
