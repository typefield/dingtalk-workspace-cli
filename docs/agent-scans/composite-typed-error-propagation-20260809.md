# 复合读取类型化错误保真 — Agent review

扫描时间：2026-08-09T17:17:09+08:00

> 本扫描由 Agent 在当前工作树运行。它结合源码关系与内存 Go 测试生成 Markdown 证据；不是 CI / policy gate，不保存服务端响应或 JSON fixture。

## Result: PASS

- 共享投影器保留 category / subtype / hint / actions / retry safety 并合并上下文：**yes**
- `minutes +detail` uses the shared typed-error projection: **yes**
- `chat +chat-messages` uses the shared typed-error projection: **yes**
- 焦点测试：`TestPreserveTypedErrorInfo|TestMinutesDetailPreservesTyped|TestChatMessagesUnifiedPaginationOutcomes`
- 测试退出码：`0`

## Required behavior

1. 复合读取可以添加命令自己的失败页、artifact 或恢复上下文，但不得把下游 `auth`、`validation`、`projection` 等 typed error 改写为笼统的 `api + retryable:true`。
2. 明确的 `retryable:false` 必须保留；仅没有分类的幂等读取错误才能采用读路径的默认重试建议。
3. 聚合层与下游错误的 details 若同名，两个事实必须同时保留，不能静默覆盖。
4. 此扫描不证明真实服务端读取成功、权限正确或资源终态；只证明本地错误投影契约。

## Focused test transcript

```text
=== RUN   TestPreserveTypedErrorInfoKeepsTypedRecoveryAndCompositeContext
--- PASS: TestPreserveTypedErrorInfoKeepsTypedRecoveryAndCompositeContext (0.00s)
=== RUN   TestPreserveTypedErrorInfoRejectsPlainPartialErrorType
--- PASS: TestPreserveTypedErrorInfoRejectsPlainPartialErrorType (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut	0.406s
=== RUN   TestChatMessagesUnifiedPaginationOutcomes
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/terminal_zero_cursor_is_exhausted_success
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/continuation_is_resumable_success
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/missing_endpoint_evidence_is_not_exhaustion
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/completed_time_range_does_not_require_an_irrelevant_source_cursor
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/later_read_failure_preserves_successful_pages
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/later_typed_read_failure_preserves_recovery_guidance
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/terminal_flag_with_a_cursor_is_partial_failure
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/unprojectable_page_is_typed_failure
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/requested_resource_failure_is_partial_rather_than_read_success
=== RUN   TestChatMessagesUnifiedPaginationOutcomes/requested_local_export_failure_is_partial_rather_than_read_success
--- PASS: TestChatMessagesUnifiedPaginationOutcomes (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/terminal_zero_cursor_is_exhausted_success (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/continuation_is_resumable_success (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/missing_endpoint_evidence_is_not_exhaustion (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/completed_time_range_does_not_require_an_irrelevant_source_cursor (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/later_read_failure_preserves_successful_pages (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/later_typed_read_failure_preserves_recovery_guidance (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/terminal_flag_with_a_cursor_is_partial_failure (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/unprojectable_page_is_typed_failure (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/requested_resource_failure_is_partial_rather_than_read_success (0.00s)
    --- PASS: TestChatMessagesUnifiedPaginationOutcomes/requested_local_export_failure_is_partial_rather_than_read_success (0.00s)
=== RUN   TestMinutesDetailPreservesTypedArtifactFailureGuidance
--- PASS: TestMinutesDetailPreservesTypedArtifactFailureGuidance (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/smart	0.666s
```
