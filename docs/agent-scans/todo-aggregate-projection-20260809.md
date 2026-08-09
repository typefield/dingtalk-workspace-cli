# Todo 聚合快捷命令投影 — Agent review

扫描时间：2026-08-09T19:46:13+08:00

> 本扫描由 Agent 在当前工作树运行。它仅以内存 Go 测试和源码入口关系作取证，输出 Markdown；不是 CI / policy gate，且不保存响应 JSON fixture。

## Result: PASS

- 共享分页器先验证 `todoCards`：`yes`
- 已激活统一结果读取入口：**2/2**
- 未使用保留页读取器的 active 入口：无
- 未迁读取入口保留 fail-closed facade：**2/2**
- 缺少 legacy fail-closed facade 的未迁读取入口：无
- 读取层保留后续页失败前的已读页：`yes`
- `+todo-done` 写前定位仍使用 fail-closed facade：`yes`
- 焦点测试退出码：`0`

## Required behavior

1. 只有明确的 `todoCards: []` 可表达已知空列表。
2. 缺少容器、容器非数组、非对象行或缺稳定 `taskId` 必须 fail-closed 为不可重试的 `api/projection_unknown`，阶段为 `response_projection`。
3. `todo +todo-done` 必须在这种投影失败时停止，不能把“未找到待办”升级成写操作。
4. `+related-tasks`、`+due-today`、`+created-todos`、`+overdue` 不得将未知上游形状表达成空集合。
5. `+related-tasks`、`+due-today` 的普通 `--format json` 必须直接输出统一结果；后续页失败须以 `partial_failure`/rc=7 保留已读页。
6. Todo 服务没有权威 continuation 事实时，统一数据必须标 `pagination_known:false`，不得声称 endpoint 已耗尽。

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
=== RUN   TestTodoAggregateRolloutIsUnifiedActive
--- PASS: TestTodoAggregateRolloutIsUnifiedActive (0.00s)
=== RUN   TestTodoAggregateUnifiedSuccessKeepsPaginationBoundaryHonest
=== RUN   TestTodoAggregateUnifiedSuccessKeepsPaginationBoundaryHonest/related-tasks
=== RUN   TestTodoAggregateUnifiedSuccessKeepsPaginationBoundaryHonest/due-today
--- PASS: TestTodoAggregateUnifiedSuccessKeepsPaginationBoundaryHonest (0.00s)
    --- PASS: TestTodoAggregateUnifiedSuccessKeepsPaginationBoundaryHonest/related-tasks (0.00s)
    --- PASS: TestTodoAggregateUnifiedSuccessKeepsPaginationBoundaryHonest/due-today (0.00s)
=== RUN   TestRelatedTasksDualValidatePreservesLegacyBytes
--- PASS: TestRelatedTasksDualValidatePreservesLegacyBytes (0.00s)
=== RUN   TestRelatedTasksUnifiedResultPreservesEarlierPagesOnLaterFailure
--- PASS: TestRelatedTasksUnifiedResultPreservesEarlierPagesOnLaterFailure (0.00s)
=== RUN   TestRelatedTasksDualValidatePreservesLegacyFailureWithoutPayload
--- PASS: TestRelatedTasksDualValidatePreservesLegacyFailureWithoutPayload (0.00s)
=== RUN   TestTodoAggregateLegacyFacadeRemainsFailClosedForWritePreflight
--- PASS: TestTodoAggregateLegacyFacadeRemainsFailClosedForWritePreflight (0.00s)
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
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/smart	0.287s
```

## Boundary

本地证据证明读取候选能保留后续页失败前的页，并且写前定位仍 fail-closed；它不证明 Todo 服务端短页等于终态。真实分页、组织权限和写后终态仍须用隔离账号单独由 Agent 取证。
