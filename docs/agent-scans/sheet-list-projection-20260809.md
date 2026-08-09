# Sheet 工作表列表投影 — Agent review

扫描时间：2026-08-09T15:30:39+08:00

> 本扫描由 Agent 在当前工作树执行。它只检查源码声明并运行内存测试，产出 Markdown；不是 CI / policy gate，且不保存服务端响应或 JSON fixture。

## Result: PASS

- 普通 `dws sheet +list-sheets --format json` 已直接走统一结果：**yes**
- 仅接受明确 `sheetId` / `sheet_id`：**yes**
- 焦点测试：`TestListSheets(Project|UsesUnifiedOutput)`
- 测试退出码：`0`

## Required behavior

1. 明确空数组才表达成功空列表；未知容器、非法行、展示字段或仅有通用 `id` 都必须 fail-closed 为不可重试的 `api/projection_unknown`。
2. 成功 JSON 只有统一 `ok/outcome/data/meta` 语义，`data.count` 与 `meta.count` 对齐；不输出版本标记。
3. 上游未提供分页事实时不得伪造 endpoint 完整性。
4. 该变更只将经过 Agent 审阅的单条幂等读命令从 dual validation 迁入 active；调用者不选择协议版本。

## Focused test transcript

```text
=== RUN   TestListSheetsProjectPreservesKnownEmptyList
--- PASS: TestListSheetsProjectPreservesKnownEmptyList (0.00s)
=== RUN   TestListSheetsProjectRejectsUnknownContainer
--- PASS: TestListSheetsProjectRejectsUnknownContainer (0.00s)
=== RUN   TestListSheetsProjectRejectsUnknownRow
--- PASS: TestListSheetsProjectRejectsUnknownRow (0.00s)
=== RUN   TestListSheetsProjectRejectsDisplayOnlyRow
--- PASS: TestListSheetsProjectRejectsDisplayOnlyRow (0.00s)
=== RUN   TestListSheetsProjectRejectsGenericIDOnlyRow
--- PASS: TestListSheetsProjectRejectsGenericIDOnlyRow (0.00s)
=== RUN   TestListSheetsProjectSupportsNestedKnownContainer
--- PASS: TestListSheetsProjectSupportsNestedKnownContainer (0.00s)
=== RUN   TestListSheetsUsesUnifiedOutput
--- PASS: TestListSheetsUsesUnifiedOutput (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/sheet	0.451s
```

## Boundary

本地证据不能证明真实工作表的权限、空表、服务端嵌套形状或潜在分页行为；这些仍须用隔离文档由 Agent 单独取证。
