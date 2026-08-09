# DWS 错误契约 Agent 扫描

扫描日期：2026-08-09

> 本报告由 Agent 读取当前 Go 源码生成，不是 CI 门禁；只保存 Markdown 证据，不生成或保存运行时 JSON fixture。

## 当前事实

- 已注册 descriptor：**105** 个；直接 `WithSubtype(...)` / 兼容桥 `WithStableSubtypeAndLegacyReason(...)` 调用点：**172** 个；间接映射调用点：**11** 个。
- `WithReason("…")` 的自由字面调用点：**0** 个；与已注册调用合计覆盖 **84** 个 subtype、**172** 个调用点。
- 直接构造 `ErrorInfo.Subtype`：**10** 个不同值，其中已登记 **10** 个、未登记 **0** 个。
- 动态 `WithReason(variable)` 调用：**0** 个。
- 至少一个调用点既没有邻近 `WithHint`、也没有 registry `DefaultHint` 的 subtype：**0** 个。
- 无法从同一局部构造窗口解析 Category 的 subtype：**0** 个。

所有字面和变量 `WithReason` 调用均已迁入受治理 registry 或兼容桥；间接 subtype 仍须由 Agent 审阅其有限映射与真实服务端终态，不能据此宣称写入已经验证。

## 源码 subtype 清单

| subtype | 治理状态 | 调用点 | 推断 Category | 有效 hint | actions | retryable 值 | retry-after | execution-started | 例子 |
|---|---|---:|---|:---:|:---:|:---:|:---:|:---:|---|
| `ambiguous_command_fallback` | registered 1 | 1 | `validation` | yes | yes | none | no | no | `internal/pipeline/command_fallback.go:154` |
| `at_me_incomplete` | registered 1 | 1 | `api` | yes | no | true | no | yes | `internal/shortcut/smart/at_me.go:484` |
| `attachment_tokens_unavailable` | registered 2 | 2 | `validation` | yes | no | none | no | yes | `internal/shortcut/aitable/attachment_composite.go:114`<br>`internal/shortcut/aitable/attachment_composite.go:216` |
| `auth_refresh_failed` | registered 1 | 1 | `auth` | yes | no | none | no | no | `internal/app/auth_refresh_retry.go:151` |
| `batch_write_failed` | registered 1 | 1 | `api` | yes | yes | false | no | yes | `internal/shortcut/chat/batch_write.go:80` |
| `business_error` | registered 1 | 1 | `api` | yes | no | none | no | no | `internal/errors/pat.go:295` |
| `chat_list_all_incomplete` | registered 1 | 1 | `api` | yes | no | mixed | no | yes | `internal/shortcut/chat/chat_group.go:1200` |
| `chat_list_incomplete` | registered 1 | 1 | `api` | yes | no | true | no | yes | `internal/shortcut/chat/lark_alignment.go:1246` |
| `chat_messages_incomplete` | registered 1 | 1 | `api` | yes | no | true | no | yes | `internal/shortcut/smart/chat_messages.go:820` |
| `chat_search_incomplete` | registered 1 | 1 | `api` | yes | no | true | no | yes | `internal/shortcut/chat/chat_group.go:426` |
| `confirmation_required` | registered 7 | 7 | `validation` | yes | yes | none | no | no | `internal/app/plugin_commands.go:759`<br>`internal/corecmd/corecmd.go:1206`<br>`internal/helpers/chat/toolbar_remove_custom.go:51` … |
| `discovery_request_build_failed` | registered 1 | 1 | `discovery` | yes | no | none | no | yes | `internal/transport/client.go:602` |
| `doc_download_preflight_failed` | registered 2 | 2 | `api` | yes | yes | none | no | no | `internal/app/doc_download_preflight.go:62`<br>`internal/app/doc_download_preflight.go:72` |
| `doc_grant_permission_partial_failure` | registered 1 | 1 | `api` | yes | yes | false | no | yes | `internal/shortcut/smart/doc_access.go:448` |
| `doc_share_message_failed` | registered 1 | 1 | `api` | yes | yes | false | no | yes | `internal/shortcut/smart/doc_access.go:523` |
| `download_output_unavailable` | registered 1 | 1 | `internal` | yes | no | none | no | yes | `internal/helpers/drive.go:708` |
| `download_size_mismatch` | registered 1 | 1 | `api` | yes | yes | true | no | yes | `internal/helpers/drive.go:721` |
| `empty_tool_response` | registered 1 | 1 | `api` | yes | no | false | no | no | `internal/shortcut/runner.go:282` |
| `endpoint_not_resolved` | registered 1 | 1 | `api` | yes | yes | none | no | no | `internal/app/runner.go:493` |
| `flag_list_incomplete` | registered 1 | 1 | `api` | yes | no | true | no | yes | `internal/shortcut/chat/lark_alignment.go:762` |
| `formula_errors_found` | registered 1 | 1 | `validation` | yes | no | none | no | no | `internal/helpers/sheet_formula_verify.go:143` |
| `gateway_auth_expired` | registered 2 | 2 | `auth` | yes | yes | none | no | no | `internal/errors/pat.go:251`<br>`internal/errors/pat.go:275` |
| `id_intersection` | registered 1 | 1 | `validation` | yes | yes | none | no | no | `internal/helpers/chat/toolbar_sort.go:59` |
| `input_read_failed` | registered 2 | 2 | `validation` | yes | no | none | no | no | `internal/helpers/sheet_formula_verify.go:212`<br>`internal/helpers/sheet_formula_verify.go:221` |
| `invalid_agent_code` | registered 2 | 2 | `validation` | yes | no | none | no | no | `internal/pat/chmod.go:181`<br>`internal/pat/chmod.go:186` |
| `invalid_agent_host` | registered 1 | 1 | `validation` | yes | no | none | no | no | `internal/app/agent_host.go:68` |
| `invalid_agent_product` | registered 1 | 1 | `validation` | yes | no | none | no | no | `internal/app/agent_product.go:46` |
| `invalid_aitable_url` | registered 1 | 1 | `validation` | yes | no | none | no | yes | `internal/shortcut/aitabletarget/resolver.go:391` |
| `invalid_argument` | registered 10 | 10 | `validation` | yes | no | none | no | no | `internal/helpers/calendar.go:25`<br>`internal/helpers/ding.go:38`<br>`internal/helpers/ding.go:62` … |
| `invalid_flag_value` | registered 14 | 14 | `validation` | yes | yes | none | no | no | `internal/helpers/chat.go:81`<br>`internal/helpers/chat.go:90`<br>`internal/helpers/chat.go:105` … |
| `invalid_json_input` | registered 1 | 1 | `validation` | yes | no | none | no | no | `internal/helpers/sheet_formula_verify.go:230` |
| `key_value_conflict` | registered 1 | 1 | `validation` | yes | no | none | no | yes | `internal/shortcut/aitable/record_upsert_by_key.go:99` |
| `mcp_tool_error` | registered 1 | 1 | `api` | yes | no | none | no | no | `internal/app/runner.go:855` |
| `messages_list_direct_incomplete` | registered 1 | 1 | `api` | yes | no | true | no | yes | `internal/shortcut/chat/chat_message.go:648` |
| `missing_required_flags` | registered 22 | 22 | `validation` | yes | yes | none | no | no | `internal/corecmd/corecmd.go:680`<br>`internal/helpers/chat/deps.go:123`<br>`internal/helpers/chat/toolbar_helpers.go:78` … |
| `missing_target` | registered 1 | 1 | `validation` | yes | no | none | no | yes | `internal/shortcut/targetresolver/resolver.go:310` |
| `my_groups_incomplete` | registered 1 | 1 | `api` | yes | no | true | no | yes | `internal/shortcut/smart/my_groups.go:269` |
| `not_authenticated` | registered 4 | 4 | `auth` | yes | yes | none | no | no | `internal/app/runner.go:611`<br>`internal/app/runner.go:904`<br>`internal/app/skill_command.go:651` … |
| `not_configured` | registered 1 | 1 | `auth` | yes | yes | none | no | no | `internal/errors/pat.go:283` |
| `pagination_inconsistent` | registered 6 | 6 | `api` | yes | no | mixed | no | yes | `internal/helpers/doc.go:115`<br>`internal/helpers/helpers.go:627`<br>`internal/shortcut/drive/drive.go:950` … |
| `parameter_conflict` | registered 1 | 1 | `validation` | yes | no | none | no | no | `internal/app/root.go:347` |
| `partial_failure` | registered 1 | 1 | `api` | yes | yes | false | no | no | `internal/app/event_personal_command.go:1196` |
| `pat_auth_cancelled` | registered 1 | 1 | `auth` | yes | no | none | no | no | `internal/app/pat_auth_retry.go:696` |
| `pat_auth_expired` | registered 1 | 1 | `auth` | yes | no | none | no | no | `internal/app/pat_auth_retry.go:688` |
| `pat_auth_rejected` | registered 1 | 1 | `auth` | yes | no | none | no | no | `internal/app/pat_auth_retry.go:680` |
| `pat_auth_timeout` | registered 1 | 1 | `auth` | yes | yes | none | no | no | `internal/app/pat_auth_retry.go:349` |
| `pat_batch_requires_yes` | registered 1 | 1 | `validation` | yes | yes | none | no | no | `internal/pat/chmod.go:497` |
| `personal_subscription_auth` | registered 1 | 1 | `auth` | yes | no | none | no | no | `internal/app/event_personal_attempts.go:564` |
| `personal_subscription_guard_failed` | registered 1 | 1 | `internal` | yes | no | false | no | no | `internal/app/event_personal_attempts.go:612` |
| `personal_subscription_invalid` | registered 1 | 1 | `validation` | yes | no | false | no | no | `internal/app/event_personal_attempts.go:626` |
| `personal_subscription_rejected` | registered 1 | 1 | `validation` | yes | no | none | no | no | `internal/app/event_personal_attempts.go:570` |
| `personal_subscription_unverified` | registered 1 | 1 | `api` | yes | no | none | no | no | `internal/app/event_personal_attempts.go:575` |
| `plugin_input_schema_invalid` | registered 1 | 1 | `validation` | yes | no | none | no | no | `internal/app/plugin_input_schema.go:120` |
| `plugin_tool_not_found` | registered 1 | 1 | `validation` | yes | no | none | no | no | `internal/app/runner.go:833` |
| `projection_unknown` | registered 19 | 19 | `api` | yes | no | false | no | no | `internal/shortcut/calendar/calendar.go:967`<br>`internal/shortcut/chat/chat_group.go:1284`<br>`internal/shortcut/contact/contact.go:295` … |
| `raw_api_credentials_required` | registered 1 | 1 | `auth` | yes | yes | false | no | yes | `internal/app/api_command.go:316` |
| `resolution_ambiguous` | registered 1 | 1 | `validation` | yes | no | false | no | yes | `internal/shortcut/targetresolver/resolver.go:662` |
| `resolution_batch_failed` | registered 1 | 1 | `validation` | yes | no | false | no | yes | `internal/shortcut/targetresolver/resolver.go:723` |
| `resolution_incomplete` | registered 2 | 2 | `api` | yes | no | true | no | yes | `internal/shortcut/targetresolver/resolver.go:686`<br>`internal/shortcut/targetresolver/resolver.go:710` |
| `resolution_not_found` | registered 1 | 1 | `validation` | yes | no | false | no | yes | `internal/shortcut/targetresolver/resolver.go:651` |
| `skill_download_info_unavailable` | registered 1 | 1 | `api` | yes | no | none | no | no | `internal/app/skill_command.go:584` |
| `stdio_error` | registered 1 | 1 | `api` | yes | no | none | no | no | `internal/app/runner.go:847` |
| `stdio_initialize_error` | registered 1 | 1 | `api` | yes | no | none | no | no | `internal/app/runner.go:817` |
| `stdio_tools_list_error` | registered 1 | 1 | `api` | yes | no | none | no | no | `internal/app/runner.go:826` |
| `system_busy` | registered 1 | 1 | `api` | yes | yes | false | no | yes | `internal/helpers/chat/toolbar_helpers.go:89` |
| `target_ambiguous` | registered 2 | 2 | `validation` | yes | no | none | no | yes | `internal/shortcut/aitable/record_upsert_by_key.go:250`<br>`internal/shortcut/aitable/view_preset.go:66` |
| `target_arguments_conflict` | registered 1 | 1 | `validation` | yes | no | none | no | yes | `internal/shortcut/targetresolver/resolver.go:297` |
| `target_incomplete` | registered 2 | 2 | `api` | yes | no | true | no | yes | `internal/shortcut/aitable/record_upsert_by_key.go:230`<br>`internal/shortcut/aitabletarget/resolver.go:423` |
| `target_invalid_response` | registered 3 | 3 | `api` | yes | no | false | no | yes | `internal/shortcut/aitable/record_upsert_by_key.go:222`<br>`internal/shortcut/aitable/record_upsert_by_key.go:243`<br>`internal/shortcut/aitabletarget/resolver.go:438` |
| `target_not_found` | registered 1 | 1 | `validation` | yes | no | none | no | yes | `internal/shortcut/aitable/workflow_deploy.go:64` |
| `target_type_conflict` | registered 1 | 1 | `validation` | yes | no | none | no | yes | `internal/shortcut/aitable/view_preset.go:79` |
| `target_type_mismatch` | registered 1 | 1 | `validation` | yes | no | none | no | yes | `internal/shortcut/targetresolver/resolver.go:182` |
| `target_verification_failed` | registered 1 | 1 | `api` | yes | no | false | no | yes | `internal/shortcut/aitable/url_resolve.go:128` |
| `thread_context_missing` | registered 1 | 1 | `api` | yes | no | none | no | no | `internal/shortcut/smart/thread_replies.go:266` |
| `thread_replies_incomplete` | registered 1 | 1 | `api` | yes | no | true | no | yes | `internal/shortcut/smart/thread_replies.go:555` |
| `thread_root_message_not_found` | registered 1 | 1 | `api` | yes | no | none | no | no | `internal/shortcut/smart/thread_replies.go:255` |
| `tool_request_build_failed` | registered 1 | 1 | `api` | yes | no | none | no | yes | `internal/transport/client.go:597` |
| `unknown_flag` | registered 2 | 2 | `validation` | yes | yes | none | no | no | `internal/app/root.go:436`<br>`internal/app/root.go:450` |
| `unknown_shortcut` | registered 1 | 1 | `validation` | yes | yes | none | no | no | `internal/pipeline/command_resolution.go:67` |
| `unknown_subcommand` | registered 1 | 1 | `validation` | yes | yes | none | no | no | `internal/pipeline/command_resolution.go:77` |
| `unsupported_alidoc_extension` | registered 1 | 1 | `validation` | yes | yes | none | no | no | `internal/app/doc_download_preflight.go:105` |
| `unsupported_format` | registered 1 | 1 | `validation` | yes | no | none | no | no | `internal/app/root.go:619` |
| `upstream_unclassified` | registered 2 | 2 | `api` | yes | yes | none | no | yes | `internal/app/server_failure_classifier.go:100`<br>`internal/transport/client.go:556` |
| `version_not_found` | registered 2 | 2 | `validation` | yes | yes | none | no | no | `internal/helpers/doc.go:4084`<br>`internal/helpers/sheet_version.go:149` |

## 直接 ErrorInfo subtype

这类值绕过 `WithReason`；迁移到注册表时必须一并纳入，不能只扫描错误构造函数。

| subtype | registry | 位置 |
|---|---|---|
| `event_stop_unverified` | registered | `internal/app/event_personal_command.go:1155` |
| `invalid_success_type` | registered | `internal/shortcut/runner.go:385` |
| `pagination_conflict` | registered | `internal/helpers/devapp.go:2020` |
| `pagination_incomplete` | registered | `internal/helpers/devapp.go:2011`, `internal/helpers/devapp.go:2029` |
| `pagination_inconsistent` | registered | `internal/shortcut/chat/chat_conversation.go:551`, `internal/shortcut/chat/chat_group.go:459`, `internal/shortcut/chat/lark_alignment.go:795`, `internal/shortcut/smart/at_me.go:702`, `internal/shortcut/smart/chat_messages.go:1173` … |
| `pagination_invalid` | registered | `internal/helpers/devapp.go:2000` |
| `projection_unknown` | registered | `internal/shortcut/chat/chat_conversation.go:569`, `internal/shortcut/smart/at_me.go:711`, `internal/shortcut/smart/chat_messages.go:1191`, `internal/shortcut/smart/search_msg.go:451`, `internal/shortcut/smart/thread_replies.go:773` |
| `resolution_batch_failed` | registered | `internal/shortcut/smart/chat_messages.go:1078` |
| `skill_setup_failed` | registered | `internal/app/skill_setup.go:797` |
| `skill_setup_result_invalid` | registered | `internal/app/skill_setup.go:786` |

## 动态 subtype 构造

动态 `WithReason` 在注册表启用前必须人工审阅：要么映射到声明的稳定 subtype，要么按当前 Category 归入 `upstream_unclassified` / `discovery_upstream_unclassified` 并保留上游码/trace，不能把上游任意文本直接变成 Agent 分支键。

- 无

## 间接稳定 subtype 映射

这类调用使用有限映射函数而非字面量；Agent 必须阅读映射函数和对应测试，确认它没有把上游文本或任意数值重新拼进 subtype。

- internal/app/root.go:410: `apperrors.WithSubtype(subtype)`
- internal/shortcut/aitabletarget/resolver.go:410: `apperrors.WithSubtype(subtype)`
- internal/shortcut/doc/common.go:125: `apperrors.WithSubtype(subtype)`
- internal/transport/client.go:1033: `apperrors.WithSubtype(httpStatusSubtype(`
- internal/transport/client.go:1162: `apperrors.WithSubtype(jsonRPCSubtype(`
- internal/transport/client.go:461: `apperrors.WithSubtype(transportUpstreamSubtype(`
- internal/transport/client.go:504: `apperrors.WithSubtype(transportUpstreamSubtype(`
- internal/transport/client.go:525: `apperrors.WithSubtype(transportUpstreamSubtype(`
- internal/transport/client.go:544: `apperrors.WithSubtype(transportUpstreamSubtype(`
- internal/transport/client.go:671: `apperrors.WithSubtype(transportUpstreamSubtype(`
- internal/transport/client.go:705: `apperrors.WithSubtype(transportUpstreamSubtype(`

## Agent 审阅结论

1. 当前 `Category`/退出码、`hint/actions/retryable/retry_after_seconds` 已是可复用底座；表中的 retryable 值区分显式 `true` 与 `false`，不把“声明过字段”误读为“允许重试”。
2. 当前源码的字面与变量 `WithReason` 均已归入 registry 或兼容桥；下一步是审阅有限间接映射与实际恢复行为，而不是重复开展 reason 清零迁移。
3. 对写请求，`retryable:false` 或省略仍不等于“绝无副作用”；应以 execution state、幂等键和真实账号验证决定恢复流程。
4. 扫描只证明源码出现与邻近选项，不能证明服务端终态或 recovery action 在真实账号上可执行。
