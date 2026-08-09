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
ACTIVE_READ_ROUTES = {
    "todo +related-tasks": ROOT / "internal/shortcut/smart/related_tasks.go",
    "todo +due-today": ROOT / "internal/shortcut/smart/due_today.go",
}
LEGACY_READ_ROUTES = {
    "todo +created-todos": ROOT / "internal/shortcut/smart/created_todos.go",
    "todo +overdue": ROOT / "internal/shortcut/smart/overdue.go",
}
WRITE_PREFLIGHT = ROOT / "internal/shortcut/smart/todo_done.go"
PATTERN = r"TestShortcutTodoCards|TestCreatedTodosRejectsUnknownTodoProjection|TestTodoAggregateShortcutsHaveReadOnlyContracts|TestTodoAggregateRolloutIsUnifiedActive|TestTodoAggregateUnifiedSuccessKeepsPaginationBoundaryHonest|TestRelatedTasksDualValidatePreservesLegacyBytes|TestRelatedTasksDualValidatePreservesLegacyFailureWithoutPayload|TestRelatedTasksUnifiedResultPreservesEarlierPagesOnLaterFailure|TestTodoAggregateLegacyFacadeRemainsFailClosedForWritePreflight"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path, help="Markdown evidence output path")
    args = parser.parse_args()

    shared = SHARED.read_text(encoding="utf-8")
    shared_ok = "cards, err := shortcutTodoCards(data)" in shared
    partial_reader_ok = "type todoCardsRead struct" in shared and "func todoAggregateResult(" in shared
    missing_active_routes = [
        route for route, path in ACTIVE_READ_ROUTES.items()
        if "shortcutReadAllTodoCards(rt," not in path.read_text(encoding="utf-8")
    ]
    missing_legacy_routes = [
        route for route, path in LEGACY_READ_ROUTES.items()
        if "shortcutListAllTodoCards(rt," not in path.read_text(encoding="utf-8")
    ]
    write_fail_closed = "shortcutListAllTodoCards(rt," in WRITE_PREFLIGHT.read_text(encoding="utf-8")
    env = os.environ.copy()
    env.setdefault("DWS_PACKAGE_VERSION", "0.0.0-agent-review")
    command = ["go", "test", "-count=1", "./internal/shortcut/smart", "-run", PATTERN, "-v"]
    result = subprocess.run(command, cwd=ROOT, env=env, text=True, capture_output=True)
    passed = result.returncode == 0 and shared_ok and partial_reader_ok and not missing_active_routes and not missing_legacy_routes and write_fail_closed

    transcript = (result.stdout + result.stderr).strip()
    if len(transcript) > 3000:
        transcript = transcript[-3000:]
    missing_active = ", ".join(f"`{route}`" for route in missing_active_routes) or "无"
    missing_legacy = ", ".join(f"`{route}`" for route in missing_legacy_routes) or "无"
    report = f"""# Todo 聚合快捷命令投影 — Agent review

扫描时间：{dt.datetime.now().astimezone().isoformat(timespec="seconds")}

> 本扫描由 Agent 在当前工作树运行。它仅以内存 Go 测试和源码入口关系作取证，输出 Markdown；不是 CI / policy gate，且不保存响应 JSON fixture。

## Result: {"PASS" if passed else "REVIEW"}

- 共享分页器先验证 `todoCards`：`{"yes" if shared_ok else "no"}`
- 已激活统一结果读取入口：**{len(ACTIVE_READ_ROUTES) - len(missing_active_routes)}/{len(ACTIVE_READ_ROUTES)}**
- 未使用保留页读取器的 active 入口：{missing_active}
- 未迁读取入口保留 fail-closed facade：**{len(LEGACY_READ_ROUTES) - len(missing_legacy_routes)}/{len(LEGACY_READ_ROUTES)}**
- 缺少 legacy fail-closed facade 的未迁读取入口：{missing_legacy}
- 读取层保留后续页失败前的已读页：`{"yes" if partial_reader_ok else "no"}`
- `+todo-done` 写前定位仍使用 fail-closed facade：`{"yes" if write_fail_closed else "no"}`
- 焦点测试退出码：`{result.returncode}`

## Required behavior

1. 只有明确的 `todoCards: []` 可表达已知空列表。
2. 缺少容器、容器非数组、非对象行或缺稳定 `taskId` 必须 fail-closed 为不可重试的 `api/projection_unknown`，阶段为 `response_projection`。
3. `todo +todo-done` 必须在这种投影失败时停止，不能把“未找到待办”升级成写操作。
4. `+related-tasks`、`+due-today`、`+created-todos`、`+overdue` 不得将未知上游形状表达成空集合。
5. `+related-tasks`、`+due-today` 的普通 `--format json` 必须直接输出统一结果；后续页失败须以 `partial_failure`/rc=7 保留已读页。
6. Todo 服务没有权威 continuation 事实时，统一数据必须标 `pagination_known:false`，不得声称 endpoint 已耗尽。

## Focused test transcript

```text
{transcript or "(no output)"}
```

## Boundary

本地证据证明读取候选能保留后续页失败前的页，并且写前定位仍 fail-closed；它不证明 Todo 服务端短页等于终态。真实分页、组织权限和写后终态仍须用隔离账号单独由 Agent 取证。
"""
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
