# DWS 错误契约 Agent 扫描

扫描日期：2026-08-08

> 本报告由 Agent 读取当前 Go 源码生成，不是 CI 门禁；只保存 Markdown 证据，不生成或保存运行时 JSON fixture。

## 当前事实

- `WithReason("…")` 的字面 subtype：**79** 个；调用点：**159** 个。
- 直接构造 `ErrorInfo.Subtype`：**6** 个不同值。
- 动态 `WithReason(variable)` 调用：**16** 个。
- 至少一个调用点缺少邻近 `WithHint` 的 subtype：**30** 个。
- 无法从同一局部构造窗口解析 Category 的 subtype：**3** 个。

当前 DWS 没有 subtype 注册表；以上值均是自由字符串。这份扫描的用途是建立迁移基线，**不**把“出现过”误写成“已经 wire-stable”。

## 字面 subtype 清单

| subtype | 调用点 | 推断 Category | hint | actions | retryable | retry-after | execution-started | 例子 |
|---|---:|---|:---:|:---:|:---:|:---:|:---:|---|
| `ambiguous_command_fallback` | 1 | `validation` | yes | yes | no | no | no | `internal/pipeline/command_fallback.go:154` |
| `at_me_incomplete` | 1 | `api` | yes | no | yes | no | yes | `internal/shortcut/smart/at_me.go:327` |
| `attachment_tokens_unavailable` | 2 | `validation` | no | no | no | no | yes | `internal/shortcut/aitable/attachment_composite.go:114`<br>`internal/shortcut/aitable/attachment_composite.go:216` |
| `auth_refresh_failed` | 1 | `auth` | yes | no | no | no | no | `internal/app/auth_refresh_retry.go:151` |
| `batch_write_failed` | 1 | `api` | no | no | yes | no | yes | `internal/shortcut/chat/batch_write.go:80` |
| `business_error` | 1 | `api` | yes | no | no | no | no | `internal/errors/pat.go:293` |
| `chat_list_all_incomplete` | 1 | `unresolved` | yes | no | yes | no | yes | `internal/shortcut/chat/chat_group.go:1065` |
| `chat_list_incomplete` | 1 | `api` | yes | no | yes | no | yes | `internal/shortcut/chat/lark_alignment.go:1153` |
| `chat_messages_incomplete` | 1 | `api` | yes | no | yes | no | yes | `internal/shortcut/smart/chat_messages.go:674` |
| `chat_search_incomplete` | 1 | `api` | yes | no | yes | no | yes | `internal/shortcut/chat/chat_group.go:323` |
| `confirmation_required` | 9 | `unresolved, validation` | yes | yes | no | no | no | `internal/app/plugin_commands.go:759`<br>`internal/corecmd/corecmd.go:1206`<br>`internal/errors/errors.go:39` … |
| `doc_download_preflight_failed` | 2 | `api` | yes | yes | no | no | no | `internal/app/doc_download_preflight.go:62`<br>`internal/app/doc_download_preflight.go:72` |
| `doc_grant_permission_partial_failure` | 1 | `api` | no | yes | yes | no | yes | `internal/shortcut/smart/doc_access.go:448` |
| `doc_share_message_failed` | 1 | `api` | no | no | yes | no | yes | `internal/shortcut/smart/doc_access.go:523` |
| `download_output_unavailable` | 1 | `internal` | no | no | no | no | yes | `internal/helpers/drive.go:708` |
| `download_size_mismatch` | 1 | `api` | no | yes | yes | no | yes | `internal/helpers/drive.go:721` |
| `empty_tool_response` | 1 | `api` | no | no | yes | no | no | `internal/shortcut/runner.go:307` |
| `endpoint_not_resolved` | 1 | `api` | yes | yes | no | no | no | `internal/app/runner.go:493` |
| `flag_list_incomplete` | 1 | `api` | yes | no | yes | no | yes | `internal/shortcut/chat/lark_alignment.go:701` |
| `formula_errors_found` | 1 | `validation` | no | no | no | no | no | `internal/helpers/sheet_formula_verify.go:143` |
| `gateway_auth_expired` | 2 | `auth` | yes | yes | no | no | no | `internal/errors/pat.go:251`<br>`internal/errors/pat.go:274` |
| `id_intersection` | 1 | `validation` | yes | yes | no | no | no | `internal/helpers/chat/toolbar_sort.go:59` |
| `input_read_failed` | 2 | `validation` | no | no | no | no | no | `internal/helpers/sheet_formula_verify.go:212`<br>`internal/helpers/sheet_formula_verify.go:221` |
| `invalid_agent_code` | 2 | `validation` | no | no | no | no | no | `internal/pat/chmod.go:181`<br>`internal/pat/chmod.go:186` |
| `invalid_agent_host` | 1 | `validation` | no | no | no | no | no | `internal/app/agent_host.go:68` |
| `invalid_agent_product` | 1 | `validation` | no | no | no | no | no | `internal/app/agent_product.go:46` |
| `invalid_aitable_url` | 1 | `validation` | yes | no | no | no | yes | `internal/shortcut/aitabletarget/resolver.go:391` |
| `invalid_argument` | 10 | `validation` | yes | no | no | no | no | `internal/helpers/calendar.go:25`<br>`internal/helpers/ding.go:38`<br>`internal/helpers/ding.go:62` … |
| `invalid_flag_value` | 13 | `validation` | no | no | no | no | no | `internal/helpers/chat.go:81`<br>`internal/helpers/chat.go:90`<br>`internal/helpers/chat.go:105` … |
| `invalid_json_input` | 1 | `validation` | no | no | no | no | no | `internal/helpers/sheet_formula_verify.go:230` |
| `key_value_conflict` | 1 | `validation` | no | no | no | no | yes | `internal/shortcut/aitable/record_upsert_by_key.go:99` |
| `mcp_tool_error` | 1 | `api` | no | no | no | no | no | `internal/app/runner.go:855` |
| `messages_list_direct_incomplete` | 1 | `api` | yes | no | yes | no | yes | `internal/shortcut/chat/chat_message.go:648` |
| `missing_required_flag` | 4 | `validation` | yes | yes | no | no | no | `internal/helpers/chat/deps.go:123`<br>`internal/helpers/chat/toolbar_helpers.go:78`<br>`internal/helpers/doc.go:4067` … |
| `missing_required_flags` | 18 | `validation` | no | no | no | no | no | `internal/corecmd/corecmd.go:680`<br>`internal/helpers/chat.go:41`<br>`internal/helpers/chat.go:52` … |
| `missing_target` | 1 | `validation` | no | no | no | no | yes | `internal/shortcut/targetresolver/resolver.go:310` |
| `my_groups_incomplete` | 1 | `api` | yes | no | yes | no | yes | `internal/shortcut/smart/my_groups.go:269` |
| `not_authenticated` | 3 | `auth` | yes | yes | no | no | no | `internal/app/runner.go:611`<br>`internal/app/runner.go:904`<br>`internal/app/skill_command.go:646` |
| `not_configured` | 1 | `auth` | yes | yes | no | no | no | `internal/errors/pat.go:281` |
| `pagination_inconsistent` | 5 | `api` | yes | no | yes | no | yes | `internal/helpers/doc.go:115`<br>`internal/helpers/helpers.go:592`<br>`internal/shortcut/mail/pagination.go:124` … |
| `parameter_conflict` | 1 | `validation` | yes | no | no | no | no | `internal/app/root.go:344` |
| `partial_failure` | 1 | `api` | yes | yes | yes | no | no | `internal/app/event_personal_command.go:1137` |
| `pat_auth_cancelled` | 1 | `auth` | yes | no | no | no | no | `internal/app/pat_auth_retry.go:696` |
| `pat_auth_expired` | 1 | `auth` | yes | no | no | no | no | `internal/app/pat_auth_retry.go:688` |
| `pat_auth_rejected` | 1 | `auth` | yes | no | no | no | no | `internal/app/pat_auth_retry.go:680` |
| `pat_auth_timeout` | 1 | `auth` | yes | yes | no | no | no | `internal/app/pat_auth_retry.go:349` |
| `pat_batch_requires_yes` | 1 | `validation` | yes | yes | no | no | no | `internal/pat/chmod.go:497` |
| `personal_subscription_guard_failed` | 1 | `internal` | no | no | yes | no | no | `internal/app/event_personal_attempts.go:528` |
| `personal_subscription_invalid` | 1 | `validation` | no | no | yes | no | no | `internal/app/event_personal_attempts.go:541` |
| `plugin_input_schema_invalid` | 1 | `validation` | no | no | no | no | no | `internal/app/plugin_input_schema.go:120` |
| `plugin_tool_not_found` | 1 | `validation` | no | no | no | no | no | `internal/app/runner.go:833` |
| `projection_unknown` | 14 | `api` | yes | no | yes | no | no | `internal/shortcut/calendar/calendar.go:925`<br>`internal/shortcut/chat/chat_group.go:1149`<br>`internal/shortcut/contact/contact.go:285` … |
| `raw_api_credentials_required` | 1 | `auth` | yes | yes | yes | no | yes | `internal/app/api_command.go:316` |
| `request_build_failed` | 1 | `unresolved` | yes | no | no | no | yes | `internal/transport/client.go:588` |
| `resolution_ambiguous` | 1 | `validation` | yes | no | yes | no | yes | `internal/shortcut/targetresolver/resolver.go:662` |
| `resolution_batch_failed` | 1 | `validation` | yes | no | yes | no | yes | `internal/shortcut/targetresolver/resolver.go:723` |
| `resolution_incomplete` | 2 | `api` | yes | no | yes | no | yes | `internal/shortcut/targetresolver/resolver.go:686`<br>`internal/shortcut/targetresolver/resolver.go:710` |
| `resolution_not_found` | 1 | `validation` | yes | no | yes | no | yes | `internal/shortcut/targetresolver/resolver.go:651` |
| `stdio_error` | 1 | `api` | no | no | no | no | no | `internal/app/runner.go:847` |
| `stdio_initialize_error` | 1 | `api` | no | no | no | no | no | `internal/app/runner.go:817` |
| `stdio_tools_list_error` | 1 | `api` | no | no | no | no | no | `internal/app/runner.go:826` |
| `system_busy` | 1 | `validation` | yes | yes | no | no | no | `internal/helpers/chat/toolbar_helpers.go:89` |
| `target_ambiguous` | 2 | `validation` | no | no | no | no | yes | `internal/shortcut/aitable/record_upsert_by_key.go:250`<br>`internal/shortcut/aitable/view_preset.go:66` |
| `target_arguments_conflict` | 1 | `validation` | no | no | no | no | yes | `internal/shortcut/targetresolver/resolver.go:297` |
| `target_incomplete` | 2 | `api` | yes | no | yes | no | yes | `internal/shortcut/aitable/record_upsert_by_key.go:230`<br>`internal/shortcut/aitabletarget/resolver.go:415` |
| `target_invalid_response` | 3 | `api` | yes | no | yes | no | yes | `internal/shortcut/aitable/record_upsert_by_key.go:222`<br>`internal/shortcut/aitable/record_upsert_by_key.go:243`<br>`internal/shortcut/aitabletarget/resolver.go:430` |
| `target_not_found` | 1 | `validation` | no | no | no | no | yes | `internal/shortcut/aitable/workflow_deploy.go:64` |
| `target_type_conflict` | 1 | `validation` | no | no | no | no | yes | `internal/shortcut/aitable/view_preset.go:79` |
| `target_type_mismatch` | 1 | `validation` | yes | no | no | no | yes | `internal/shortcut/targetresolver/resolver.go:182` |
| `target_verification_failed` | 1 | `api` | yes | no | yes | no | yes | `internal/shortcut/aitable/url_resolve.go:128` |
| `thread_context_missing` | 1 | `api` | yes | no | no | no | no | `internal/shortcut/smart/thread_replies.go:249` |
| `thread_replies_incomplete` | 1 | `api` | yes | no | yes | no | yes | `internal/shortcut/smart/thread_replies.go:480` |
| `thread_root_message_not_found` | 1 | `api` | yes | no | no | no | no | `internal/shortcut/smart/thread_replies.go:238` |
| `unknown_flag` | 2 | `validation` | yes | yes | no | no | no | `internal/app/root.go:433`<br>`internal/app/root.go:447` |
| `unknown_shortcut` | 1 | `validation` | yes | yes | no | no | no | `internal/pipeline/command_resolution.go:67` |
| `unknown_subcommand` | 1 | `validation` | yes | yes | no | no | no | `internal/pipeline/command_resolution.go:77` |
| `unsupported_alidoc_extension` | 1 | `validation` | yes | yes | no | no | no | `internal/app/doc_download_preflight.go:105` |
| `unsupported_format` | 1 | `validation` | no | no | no | no | no | `internal/app/root.go:616` |
| `version_not_found` | 2 | `validation` | yes | yes | no | no | no | `internal/helpers/doc.go:4084`<br>`internal/helpers/sheet_version.go:149` |

## 直接 ErrorInfo subtype

这类值绕过 `WithReason`；迁移到注册表时必须一并纳入，不能只扫描错误构造函数。

| subtype | 位置 |
|---|---|
| `invalid_success_type` | `internal/shortcut/runner.go:383` |
| `pagination_conflict` | `internal/helpers/devapp.go:2016` |
| `pagination_incomplete` | `internal/helpers/devapp.go:2009`, `internal/helpers/devapp.go:2023` |
| `pagination_invalid` | `internal/helpers/devapp.go:2000` |
| `skill_setup_failed` | `internal/app/skill_setup.go:795` |
| `skill_setup_result_invalid` | `internal/app/skill_setup.go:785` |

## 动态 subtype 构造

动态构造在注册表启用前必须人工审阅：要么映射到声明的稳定 subtype，要么统一归入 `api/upstream_unclassified` 并保留上游码/trace，不能把上游任意文本直接变成 Agent 分支键。

- internal/app/event_personal_attempts.go:489: `apperrors.WithReason(reason)`
- internal/app/root.go:407: `apperrors.WithReason(reason)`
- internal/app/server_failure_classifier.go:81: `apperrors.WithReason(fallbackReason)`
- internal/app/server_failure_classifier.go:89: `apperrors.WithReason(classified.reason)`
- internal/app/skill_command.go:583: `apperrors.WithReason(downloadResp.ErrorCode)`
- internal/shortcut/doc/common.go:122: `apperrors.WithReason(reason)`
- internal/shortcut/smart/local_chat_validation.go:32: `apperrors.WithReason(reason)`
- internal/transport/client.go:1028: `apperrors.WithReason(fmt.Sprintf("http_%d", statusCode)`
- internal/transport/client.go:1155: `apperrors.WithReason(reason)`
- internal/transport/client.go:461: `apperrors.WithReason(reasonForMethod(request.Method, "response_read_failed")`
- internal/transport/client.go:504: `apperrors.WithReason(reasonForMethod(request.Method, "invalid_response")`
- internal/transport/client.go:525: `apperrors.WithReason(reasonForMethod(request.Method, "empty_result")`
- internal/transport/client.go:544: `apperrors.WithReason(reasonForMethod(request.Method, "result_decode_failed")`
- internal/transport/client.go:556: `apperrors.WithReason(reason)`
- internal/transport/client.go:668: `apperrors.WithReason(reason)`
- internal/transport/client.go:701: `apperrors.WithReason(reason)`

## Agent 审阅结论

1. 当前 `Category`/退出码、`hint/actions/retryable/retry_after_seconds` 已是可复用底座。
2. 不能直接宣布 subtype 已稳定：`WithReason(string)` 没有闭集，也没有逐 subtype 的恢复字段声明。
3. 下一步应先从本清单审定少量高频、跨产品 subtype（如 `missing_required_flags`、`unknown_flag`、`confirmation_required`、`rate_limit`、`pagination_inconsistent`、`projection_unknown`），建立注册表；不应一次性重命名现有 wire 字段或类别。
4. 扫描只证明源码出现与邻近选项，不能证明服务端终态或 recovery action 在真实账号上可执行。
