# 复合读取类型化错误保真 — Agent review

扫描时间：2026-08-09T17:32:38+08:00

> 本扫描由 Agent 在当前工作树运行。它结合源码关系与内存 Go 测试生成 Markdown 证据；不是 CI / policy gate，不保存服务端响应或 JSON fixture。

## Result: PASS

- 共享投影器保留 category / subtype / hint / actions / retry safety 并合并上下文：**yes**
- `minutes +detail` uses the shared typed-error projection: **yes**
- `chat +chat-messages` uses the shared typed-error projection: **yes**
- `chat +thread-replies` uses the shared typed-error projection: **yes**
- `chat +my-groups candidate` uses the shared typed-error projection: **yes**
- `chat +at-me` uses the shared typed-error projection: **yes**
- `chat +search-msg read/enrichment` uses the shared typed-error projection: **yes**
- `todo aggregate reads` uses the shared typed-error projection: **yes**
- `chat +chat-search / +chat-list-all` uses the shared typed-error projection: **yes**
- `chat +conversation-list` uses the shared typed-error projection: **yes**
- `chat +flag-list` uses the shared typed-error projection: **yes**
- 焦点测试：`TestPreserveTypedErrorInfo|TestMinutesDetailPreservesTyped|TestChatMessagesUnifiedPaginationOutcomes|TestSearchMsgUnifiedPaginationOutcomes|TestCompositeReadFailuresPreserveTypedRecoveryFacts|TestChatCompositeReadFailuresPreserveTypedRecoveryFacts`
- 测试退出码：`0`

## Required behavior

1. 复合读取可以添加命令自己的失败页、artifact 或恢复上下文，但不得把下游 `auth`、`validation`、`projection` 等 typed error 改写为笼统的 `api + retryable:true`。
2. 明确的 `retryable:false` 必须保留；仅没有分类的幂等读取错误才能采用读路径的默认重试建议。
   已登记 subtype 必须服从 registry 的 retry policy；`RetryNever` 不得继承外层读取的 `retryable:true`。
3. 聚合层与下游错误的 details 若同名，两个事实必须同时保留，不能静默覆盖。
4. 富化等批量后处理必须按失败批次保留独立 typed error，不能把多个不同原因压成一个自由字符串。
5. 此扫描不证明真实服务端读取成功、权限正确或资源终态；只证明本地错误投影契约。

## Focused test transcript

```text
=== RUN   TestPreserveTypedErrorInfoKeepsTypedRecoveryAndCompositeContext
--- PASS: TestPreserveTypedErrorInfoKeepsTypedRecoveryAndCompositeContext (0.00s)
=== RUN   TestPreserveTypedErrorInfoRejectsPlainPartialErrorType
--- PASS: TestPreserveTypedErrorInfoRejectsPlainPartialErrorType (0.00s)
=== RUN   TestPreserveTypedErrorInfoDoesNotInheritRetryForGovernedNeverRetryErrors
=== RUN   TestPreserveTypedErrorInfoDoesNotInheritRetryForGovernedNeverRetryErrors/auth_category
=== RUN   TestPreserveTypedErrorInfoDoesNotInheritRetryForGovernedNeverRetryErrors/projection_subtype
--- PASS: TestPreserveTypedErrorInfoDoesNotInheritRetryForGovernedNeverRetryErrors (0.00s)
    --- PASS: TestPreserveTypedErrorInfoDoesNotInheritRetryForGovernedNeverRetryErrors/auth_category (0.00s)
    --- PASS: TestPreserveTypedErrorInfoDoesNotInheritRetryForGovernedNeverRetryErrors/projection_subtype (0.00s)
=== RUN   TestPreserveTypedErrorInfoKeepsReviewedIdempotentReadDefault
--- PASS: TestPreserveTypedErrorInfoKeepsReviewedIdempotentReadDefault (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut	0.380s
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
=== RUN   TestSearchMsgUnifiedPaginationOutcomes
=== RUN   TestSearchMsgUnifiedPaginationOutcomes/terminal_cursor_is_exhausted_and_legacy_fields_do_not_leak
=== RUN   TestSearchMsgUnifiedPaginationOutcomes/local_page_budget_exposes_a_usable_continuation_without_false_failure
=== RUN   TestSearchMsgUnifiedPaginationOutcomes/single_page_without_endpoint_evidence_stays_successful_but_unknown
=== RUN   TestSearchMsgUnifiedPaginationOutcomes/page-all_without_endpoint_evidence_is_partial
=== RUN   TestSearchMsgUnifiedPaginationOutcomes/later_read_failure_retains_successful_page
=== RUN   TestSearchMsgUnifiedPaginationOutcomes/typed_enrichment_failure_remains_a_typed_partial_item
=== RUN   TestSearchMsgUnifiedPaginationOutcomes/missing_enrichment_rows_are_projection_partial_items
=== RUN   TestSearchMsgUnifiedPaginationOutcomes/unknown_response_shape_is_typed_failure_rather_than_empty_success
--- PASS: TestSearchMsgUnifiedPaginationOutcomes (0.00s)
    --- PASS: TestSearchMsgUnifiedPaginationOutcomes/terminal_cursor_is_exhausted_and_legacy_fields_do_not_leak (0.00s)
    --- PASS: TestSearchMsgUnifiedPaginationOutcomes/local_page_budget_exposes_a_usable_continuation_without_false_failure (0.00s)
    --- PASS: TestSearchMsgUnifiedPaginationOutcomes/single_page_without_endpoint_evidence_stays_successful_but_unknown (0.00s)
    --- PASS: TestSearchMsgUnifiedPaginationOutcomes/page-all_without_endpoint_evidence_is_partial (0.00s)
    --- PASS: TestSearchMsgUnifiedPaginationOutcomes/later_read_failure_retains_successful_page (0.00s)
    --- PASS: TestSearchMsgUnifiedPaginationOutcomes/typed_enrichment_failure_remains_a_typed_partial_item (0.00s)
    --- PASS: TestSearchMsgUnifiedPaginationOutcomes/missing_enrichment_rows_are_projection_partial_items (0.00s)
    --- PASS: TestSearchMsgUnifiedPaginationOutcomes/unknown_response_shape_is_typed_failure_rather_than_empty_success (0.00s)
=== RUN   TestCompositeReadFailuresPreserveTypedRecoveryFacts
=== RUN   TestCompositeReadFailuresPreserveTypedRecoveryFacts/todo_aggregate
=== RUN   TestCompositeReadFailuresPreserveTypedRecoveryFacts/thread_replies
=== RUN   TestCompositeReadFailuresPreserveTypedRecoveryFacts/my_groups
=== RUN   TestCompositeReadFailuresPreserveTypedRecoveryFacts/at_me
=== RUN   TestCompositeReadFailuresPreserveTypedRecoveryFacts/search_message
--- PASS: TestCompositeReadFailuresPreserveTypedRecoveryFacts (0.00s)
    --- PASS: TestCompositeReadFailuresPreserveTypedRecoveryFacts/todo_aggregate (0.00s)
    --- PASS: TestCompositeReadFailuresPreserveTypedRecoveryFacts/thread_replies (0.00s)
    --- PASS: TestCompositeReadFailuresPreserveTypedRecoveryFacts/my_groups (0.00s)
    --- PASS: TestCompositeReadFailuresPreserveTypedRecoveryFacts/at_me (0.00s)
    --- PASS: TestCompositeReadFailuresPreserveTypedRecoveryFacts/search_message (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/smart	0.655s
=== RUN   TestChatCompositeReadFailuresPreserveTypedRecoveryFacts
=== RUN   TestChatCompositeReadFailuresPreserveTypedRecoveryFacts/joined_groups
=== RUN   TestChatCompositeReadFailuresPreserveTypedRecoveryFacts/conversations
=== RUN   TestChatCompositeReadFailuresPreserveTypedRecoveryFacts/favorites
=== RUN   TestChatCompositeReadFailuresPreserveTypedRecoveryFacts/group_search
--- PASS: TestChatCompositeReadFailuresPreserveTypedRecoveryFacts (0.00s)
    --- PASS: TestChatCompositeReadFailuresPreserveTypedRecoveryFacts/joined_groups (0.00s)
    --- PASS: TestChatCompositeReadFailuresPreserveTypedRecoveryFacts/conversations (0.00s)
    --- PASS: TestChatCompositeReadFailuresPreserveTypedRecoveryFacts/favorites (0.00s)
    --- PASS: TestChatCompositeReadFailuresPreserveTypedRecoveryFacts/group_search (0.00s)
PASS
ok  	github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/chat	0.878s
```
