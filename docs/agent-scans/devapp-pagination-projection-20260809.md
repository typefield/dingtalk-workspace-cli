# devapp shortcut pagination projection — Agent review

扫描时间：2026-08-09T15:10:56+08:00

> 这是 Agent 的语义审阅取证，不是 CI / policy gate。扫描只调用内存中的 Go 测试，并输出 Markdown；不会保存任何上游响应或 JSON fixture。

## Result: PASS

- 已审阅分页 shortcut：**4/4**
- 覆盖入口：`devapp +list`、`+permission-list`、`+event-list`、`+version-list`
- 焦点测试：`TestDevApp(ProjectedLists(PreservePaginationEvidence|RejectInvalidPaginationEvidence)|PaginatedShortcutsEmitUnifiedResumableResults)`
- 测试退出码：`0`

## Required behavior

1. 保留顶层及嵌套 `result/data/content/pageInfo/pagination` 中的有效 `hasMore` / `nextCursor`。
2. `hasMore` 非布尔值必须返回 `validation/pagination_invalid`，不能投影成空分页。
3. `nextCursor` 非字符串、`hasMore=true` 无游标必须返回 `validation/pagination_incomplete`。
4. 多层 `hasMore` 或非空 `nextCursor` 互相冲突、末页仍带游标必须返回 `validation/pagination_conflict`。
5. 上述失败均为 `response_projection`、不可安全重试；不得输出成功列表。
6. 只有经本项 Agent 审阅的四条 terminal command 才在原路径直接输出统一结果；调用者只传 `--format json`。
7. 非末页必须在 `meta.pagination` 中保留 `endpoint_exhausted:false` 与 `next_token`，且不输出任何协议版本标记。

## Source coverage

- PASS：四个列表 Shortcut 均先验证分页证据，再交给统一结果映射。
- PASS：四条已审阅的列表 Shortcut 均在原命令路径直接输出统一结果；没有公开版本/协议选择参数。


## Focused test transcript

```text
=== RUN   TestDevAppProjectedListsPreservePaginationEvidence
=== RUN   TestDevAppProjectedListsPreservePaginationEvidence/top_level
=== RUN   TestDevAppProjectedListsPreservePaginationEvidence/nested_result
=== RUN   TestDevAppProjectedListsPreservePaginationEvidence/nested_page_info
--- PASS: TestDevAppProjectedListsPreservePaginationEvidence (0.00s)
    --- PASS: TestDevAppProjectedListsPreservePaginationEvidence/top_level (0.00s)
    --- PASS: TestDevAppProjectedListsPreservePaginationEvidence/nested_result (0.00s)
    --- PASS: TestDevAppProjectedListsPreservePaginationEvidence/nested_page_info (0.00s)
=== RUN   TestDevAppProjectedListsRejectInvalidPaginationEvidence
=== RUN   TestDevAppProjectedListsRejectInvalidPaginationEvidence/has_more_is_not_boolean
=== RUN   TestDevAppProjectedListsRejectInvalidPaginationEvidence/cursor_is_not_string
=== RUN   TestDevAppProjectedListsRejectInvalidPaginationEvidence/has_more_conflicts_across_envelopes
=== RUN   TestDevAppProjectedListsRejectInvalidPaginationEvidence/cursor_conflicts_across_envelopes
=== RUN   TestDevAppProjectedListsRejectInvalidPaginationEvidence/nonfinal_page_omits_cursor
=== RUN   TestDevAppProjectedListsRejectInvalidPaginationEvidence/exhausted_page_carries_cursor
--- PASS: TestDevAppProjectedListsRejectInvalidPaginationEvidence (0.00s)
    --- PASS: TestDevAppProjectedListsRejectInvalidPaginationEvidence/has_more_is_not_boolean (0.00s)
    --- PASS: TestDevAppProjectedListsRejectInvalidPaginationEvidence/cursor_is_not_string (0.00s)
    --- PASS: TestDevAppProjectedListsRejectInvalidPaginationEvidence/has_more_conflicts_across_envelopes (0.00s)
    --- PASS: TestDevAppProjectedListsRejectInvalidPaginationEvidence/cursor_conflicts_across_envelopes (0.00s)
    --- PASS: TestDevAppProjectedListsRejectInvalidPaginationEvidence/nonfinal_page_omits_cursor (0.00s)
    --- PASS: TestDevAppProjectedListsRejectInvalidPaginationEvidence/exhausted_page_carries_cursor (0.00s)
=== RUN   TestDevAppPaginatedShortcutsEmitUnifiedResumableResults
=== RUN   TestDevAppPaginatedShortcutsEmitUnifiedResumableResults/apps
=== RUN   TestDevAppPaginatedShortcutsEmitUnifiedResumableResults/permissions
=== RUN   TestDevAppPaginatedShortcutsEmitUnifiedResumableResults/events
=== RUN   TestDevAppPaginatedShortcutsEmitUnifiedResumableResults/versions
--- PASS: TestDevAppPaginatedShortcutsEmitUnifiedResumableResults (0.00s)
    --- PASS: TestDevAppPaginatedShortcutsEmitUnifiedResumableResults/apps (0.00s)
    --- PASS: TestDevAppPaginatedShortcutsEmitUnifiedResumableResults/permissions (0.00s)
    --- PASS: TestDevAppPaginatedShortcutsEmitUnifiedResumableResults/events (0.00s)
    --- PASS: TestDevAppPaginatedShortcutsEmitUnifiedResumableResults/versions (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/devapp	0.372s
```

## Boundary

这证明本地投影不会再静默吞掉异常或矛盾的分页字段；真实 devapp 账号的续翻、服务端末页语义及 `dev ...` / `devapp +...` 的端到端对拍仍需单独 Agent 实测。
