# devapp shortcut pagination projection — Agent review

扫描时间：2026-08-09T15:19:09+08:00

> 这是 Agent 的语义审阅取证，不是 CI / policy gate。扫描只调用内存中的 Go 测试，并输出 Markdown；不会保存任何上游响应或 JSON fixture。

## Result: PASS

- 已审阅分页 shortcut：**4/4**
- 覆盖入口：`devapp +list`、`+permission-list`、`+event-list`、`+version-list`
- 焦点测试：`TestDevApp(ProjectedLists(PreservePaginationEvidence|RejectInvalidPaginationEvidence)|ListProjectionSeparatesKnownEmptyFromUnknown|PaginatedShortcutsEmitUnifiedResumableResults)`
- 测试退出码：`0`

## Required behavior

1. 保留顶层及嵌套 `result/data/content/pageInfo/pagination` 中的有效 `hasMore` / `nextCursor`。
2. `hasMore` 非布尔值必须返回 `validation/pagination_invalid`，不能投影成空分页。
3. `nextCursor` 非字符串、`hasMore=true` 无游标必须返回 `validation/pagination_incomplete`。
4. 多层 `hasMore` 或非空 `nextCursor` 互相冲突必须返回 `validation/pagination_conflict`；
   DevApp 实际终页会保留一个位置 cursor，`hasMore=false` 为权威终态，该 cursor 不得投影为 `next_token`。
5. 上述失败均为 `response_projection`、不可安全重试；不得输出成功列表。
6. 只有显式的空数组才可投影为成功空列表；缺容器、非数组、非法行或仅展示字段的行必须为 `api/projection_unknown`。
7. 只有经本项 Agent 审阅的四条 terminal command 才在原路径直接输出统一结果；调用者只传 `--format json`。
8. 非末页必须在 `meta.pagination` 中保留 `endpoint_exhausted:false` 与 `next_token`，且不输出任何协议版本标记。
9. 末页必须输出 `endpoint_exhausted:true`，即使原始响应携带位置 cursor，也不能诱导 Agent 继续请求。

## Source coverage

- PASS：四个列表 Shortcut 均先验证分页证据，再交给统一结果映射。
- PASS：四条已审阅的列表 Shortcut 均在原命令路径直接输出统一结果；没有公开版本/协议选择参数。

## Live pagination evidence

真实只读、脱敏探针确认：首屏 11 项，`hasMore=false` 且位置 cursor 非空；使用该 cursor
再次读取得到 0 项、同一 cursor、仍为 `hasMore=false`。修复后 `devapp +list` 输出
`success`、`endpoint_exhausted:true`，没有 `next_token` 或协议版本标记。探针不保存原始
JSON，也不打印应用或 cursor 值。


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
=== RUN   TestDevAppProjectedListsTreatTerminalCursorAsNonActionable
--- PASS: TestDevAppProjectedListsRejectInvalidPaginationEvidence (0.00s)
    --- PASS: TestDevAppProjectedListsRejectInvalidPaginationEvidence/has_more_is_not_boolean (0.00s)
    --- PASS: TestDevAppPr
... (middle transcript elided) ...
ctionSeparatesKnownEmptyFromUnknown/events/display_only_row (0.00s)
    --- PASS: TestDevAppListProjectionSeparatesKnownEmptyFromUnknown/versions (0.00s)
        --- PASS: TestDevAppListProjectionSeparatesKnownEmptyFromUnknown/versions/unknown_container (0.00s)
        --- PASS: TestDevAppListProjectionSeparatesKnownEmptyFromUnknown/versions/not_an_array (0.00s)
        --- PASS: TestDevAppListProjectionSeparatesKnownEmptyFromUnknown/versions/malformed_row (0.00s)
        --- PASS: TestDevAppListProjectionSeparatesKnownEmptyFromUnknown/versions/display_only_row (0.00s)
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
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/devapp	0.385s
```

## Boundary

这证明本地投影不会再静默吞掉异常分页字段，并已对拍一组真实终页位置 cursor；
非终页续翻、已知空、权限受限、其他列表接口和 `dev ...` / `devapp +...` 的完整端到端矩阵仍需单独 Agent 实测。
