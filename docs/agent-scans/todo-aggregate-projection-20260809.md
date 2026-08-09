# Todo 聚合快捷命令投影 — Agent review

扫描时间：2026-08-09T15:03:09+08:00

> 本扫描由 Agent 在当前工作树运行。它仅以内存 Go 测试和源码入口关系作取证，输出 Markdown；不是 CI / policy gate，且不保存响应 JSON fixture。

## Result: PASS

- 共享分页器先验证 `todoCards`：`yes`
- 已复核公共聚合入口：**5/5**
- 缺少共享分页器的入口：无
- 焦点测试退出码：`0`

## Required behavior

1. 只有明确的 `todoCards: []` 可表达已知空列表。
2. 缺少容器、容器非数组、非对象行或缺稳定 `taskId` 必须 fail-closed 为不可重试的 `api/projection_unknown`，阶段为 `response_projection`。
3. `todo +todo-done` 必须在这种投影失败时停止，不能把“未找到待办”升级成写操作。
4. `+related-tasks`、`+due-today`、`+created-todos`、`+overdue` 不得将未知上游形状表达成空集合。

## Focused test transcript

```text
=== RUN   TestCreatedTodosRejectsUnknownTodoProjection
--- PASS: TestCreatedTodosRejectsUnknownTodoProjection (0.00s)
=== RUN   TestTodoAggregateShortcutsHaveReadOnlyContracts
=== RUN   TestTodoAggregateShortcutsHaveReadOnlyContracts/related
=== RUN   TestTodoAggregateShortcutsHaveReadOnlyContracts/due-today
--- PASS: TestTodoAggregateShortcutsHaveReadOnlyContracts (0.00s)
    --- PASS: TestTodoAggregateShortcutsHaveReadOnlyContracts/related (0.00s)
    --- PASS: TestTodoAggregateShortcutsHaveReadOnlyContracts/due-today (0.00s)
=== RUN   TestShortcutTodoCardsUnwrapsNestedResult
--- PASS: TestShortcutTodoCardsUnwrapsNestedResult (0.00s)
=== RUN   TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows
=== RUN   TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows/missing_container
=== RUN   TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows/container_is_not_array
=== RUN   TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows/row_is_not_object
=== RUN   TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows/row_lacks_task_id
--- PASS: TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows (0.00s)
    --- PASS: TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows/missing_container (0.00s)
    --- PASS: TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows/container_is_not_array (0.00s)
    --- PASS: TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows/row_is_not_object (0.00s)
    --- PASS: TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows/row_lacks_task_id (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/smart	0.447s
```

## Boundary

本地证据仅证明 CLI 不会将异常响应形状伪装为“没有待办”。真实分页、组织权限和写后终态仍须用隔离账号单独由 Agent 取证。
