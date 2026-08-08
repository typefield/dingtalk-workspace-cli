// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

// Subtype is an approved machine-readable error reason. Only values present
// in the registry are stable Agent branch keys. The underlying Error.Reason
// remains a string during the gradual migration so legacy commands preserve
// their existing wire output.
type Subtype string

const (
	SubtypeMissingRequiredFlags               Subtype = "missing_required_flags"
	SubtypeInvalidFlagValue                   Subtype = "invalid_flag_value"
	SubtypeInvalidArgument                    Subtype = "invalid_argument"
	SubtypeUnknownFlag                        Subtype = "unknown_flag"
	SubtypeConfirmationRequired               Subtype = "confirmation_required"
	SubtypeRateLimit                          Subtype = "rate_limit"
	SubtypePaginationInconsistent             Subtype = "pagination_inconsistent"
	SubtypePaginationInvalid                  Subtype = "pagination_invalid"
	SubtypePaginationIncomplete               Subtype = "pagination_incomplete"
	SubtypePaginationConflict                 Subtype = "pagination_conflict"
	SubtypeChatSearchIncomplete               Subtype = "chat_search_incomplete"
	SubtypeChatListAllIncomplete              Subtype = "chat_list_all_incomplete"
	SubtypeFlagListIncomplete                 Subtype = "flag_list_incomplete"
	SubtypeChatListIncomplete                 Subtype = "chat_list_incomplete"
	SubtypeMessagesListDirectIncomplete       Subtype = "messages_list_direct_incomplete"
	SubtypeChatMessagesIncomplete             Subtype = "chat_messages_incomplete"
	SubtypeMyGroupsIncomplete                 Subtype = "my_groups_incomplete"
	SubtypeThreadRepliesIncomplete            Subtype = "thread_replies_incomplete"
	SubtypeProjectionUnknown                  Subtype = "projection_unknown"
	SubtypeInputReadFailed                    Subtype = "input_read_failed"
	SubtypeInvalidJSONInput                   Subtype = "invalid_json_input"
	SubtypeFormulaErrorsFound                 Subtype = "formula_errors_found"
	SubtypeDownloadOutputUnavailable          Subtype = "download_output_unavailable"
	SubtypeDownloadSizeMismatch               Subtype = "download_size_mismatch"
	SubtypeVersionNotFound                    Subtype = "version_not_found"
	SubtypeTargetTypeMismatch                 Subtype = "target_type_mismatch"
	SubtypeTargetArgumentsConflict            Subtype = "target_arguments_conflict"
	SubtypeMissingTarget                      Subtype = "missing_target"
	SubtypeResolutionNotFound                 Subtype = "resolution_not_found"
	SubtypeResolutionAmbiguous                Subtype = "resolution_ambiguous"
	SubtypeResolutionIncomplete               Subtype = "resolution_incomplete"
	SubtypeResolutionBatchFailed              Subtype = "resolution_batch_failed"
	SubtypeInvalidAITableURL                  Subtype = "invalid_aitable_url"
	SubtypeTargetNotFound                     Subtype = "target_not_found"
	SubtypeTargetAmbiguous                    Subtype = "target_ambiguous"
	SubtypeTargetTypeConflict                 Subtype = "target_type_conflict"
	SubtypeTargetIncomplete                   Subtype = "target_incomplete"
	SubtypeTargetInvalidResponse              Subtype = "target_invalid_response"
	SubtypeTargetVerificationFailed           Subtype = "target_verification_failed"
	SubtypeKeyValueConflict                   Subtype = "key_value_conflict"
	SubtypeAttachmentTokensUnavailable        Subtype = "attachment_tokens_unavailable"
	SubtypeUpstreamUnclassified               Subtype = "upstream_unclassified"
	SubtypeDiscoveryUpstreamUnclassified      Subtype = "discovery_upstream_unclassified"
	SubtypeUpstreamAuthenticationRequired     Subtype = "upstream_authentication_required"
	SubtypeUpstreamAuthorizationDenied        Subtype = "upstream_authorization_denied"
	SubtypeToolProtocolIncompatible           Subtype = "tool_protocol_incompatible"
	SubtypeBackendDependencyUnavailable       Subtype = "backend_dependency_unavailable"
	SubtypeUpstreamRequestRejected            Subtype = "upstream_request_rejected"
	SubtypeBlockedFlag                        Subtype = "blocked_flag"
	SubtypeAmbiguousFlag                      Subtype = "ambiguous_flag"
	SubtypeSkillDownloadInfoUnavailable       Subtype = "skill_download_info_unavailable"
	SubtypeDocCreateMissingNodeID             Subtype = "doc_create_missing_node_id"
	SubtypeDocCreateInitialContentFailed      Subtype = "doc_create_initial_content_failed"
	SubtypeDocCheckpointUpdateFailed          Subtype = "doc_checkpoint_update_failed"
	SubtypeDocCheckpointVerificationFailed    Subtype = "doc_checkpoint_verification_failed"
	SubtypeDocHistoryRevertVerificationFailed Subtype = "doc_history_revert_verification_failed"
	SubtypeEventStopUnverified                Subtype = "event_stop_unverified"
	SubtypeInvalidSuccessType                 Subtype = "invalid_success_type"
	SubtypeSkillSetupResultInvalid            Subtype = "skill_setup_result_invalid"
	SubtypeSkillSetupFailed                   Subtype = "skill_setup_failed"
	SubtypeBatchWriteFailed                   Subtype = "batch_write_failed"
	SubtypeDocGrantPermissionPartialFailure   Subtype = "doc_grant_permission_partial_failure"
	SubtypeDocShareMessageFailed              Subtype = "doc_share_message_failed"
	SubtypeStdioInitializeError               Subtype = "stdio_initialize_error"
	SubtypeStdioToolsListError                Subtype = "stdio_tools_list_error"
	SubtypeStdioError                         Subtype = "stdio_error"
	SubtypeMCPToolError                       Subtype = "mcp_tool_error"
	SubtypeEmptyToolResponse                  Subtype = "empty_tool_response"
	SubtypePluginToolNotFound                 Subtype = "plugin_tool_not_found"
	SubtypePluginInputSchemaInvalid           Subtype = "plugin_input_schema_invalid"
	SubtypeUnsupportedFormat                  Subtype = "unsupported_format"
	SubtypeInvalidAgentCode                   Subtype = "invalid_agent_code"
	SubtypeInvalidAgentHost                   Subtype = "invalid_agent_host"
	SubtypeInvalidAgentProduct                Subtype = "invalid_agent_product"
)

// RetryPolicy describes whether a descriptor can ever recommend replay. It
// does not cause the CLI to replay requests: retry decisions remain with the
// caller/Agent and must also consider idempotency and execution_started.
type RetryPolicy string

const (
	RetryNever              RetryPolicy = "never"
	RetryServerDirective    RetryPolicy = "server_directive"
	RetryIdempotentReadOnly RetryPolicy = "idempotent_read_only"
)

// SubtypeDescriptor is the registry entry for a public, stable subtype.
// Recovery text deliberately stays at the command boundary: a generic
// descriptor cannot safely invent a resource ID, credential scope, or action.
type SubtypeDescriptor struct {
	Subtype       Subtype
	Category      Category
	RetryPolicy   RetryPolicy
	RequireHint   bool
	RequireAction bool
	// DefaultHint is used only when a command has no more-specific recovery
	// hint. It must be safe without inventing resource IDs, credentials, or a
	// business-terminal result; command-local WithHint remains authoritative.
	DefaultHint string
	Description string
}

var subtypeRegistry = map[Subtype]SubtypeDescriptor{
	SubtypeMissingRequiredFlags: {
		Subtype:       SubtypeMissingRequiredFlags,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请补齐缺失的必填参数后重试；运行当前命令的 --help 查看参数说明。",
		Description:   "required command input is missing",
	},
	SubtypeInvalidFlagValue: {
		Subtype:       SubtypeInvalidFlagValue,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请根据当前命令的 --help 检查参数取值、格式和互斥关系，修正后再重试。",
		Description:   "a command flag value is invalid or conflicts with another flag",
	},
	SubtypeInvalidArgument: {
		Subtype:       SubtypeInvalidArgument,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请根据当前命令的 --help 检查输入参数和组合约束，修正后再重试。",
		Description:   "a command argument or local input combination is invalid",
	},
	SubtypeUnknownFlag: {
		Subtype:       SubtypeUnknownFlag,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请运行当前命令的 --help 查看可用参数，修正参数名后再重试。",
		Description:   "an unsupported command flag was supplied",
	},
	SubtypeConfirmationRequired: {
		Subtype:       SubtypeConfirmationRequired,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "请先使用 --dry-run 预览；获得用户确认后以相同参数追加 --yes。",
		Description:   "a protected write was stopped before request execution",
	},
	SubtypeRateLimit: {
		Subtype:       SubtypeRateLimit,
		Category:      CategoryAPI,
		RetryPolicy:   RetryServerDirective,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请按服务端给出的等待时间退避后重试；未提供时使用指数退避。",
		Description:   "the upstream service asked the caller to slow down",
	},
	SubtypePaginationInconsistent: {
		Subtype:       SubtypePaginationInconsistent,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请检查上游的分页游标和 hasMore 证据；确认前不要把结果当作完整。",
		Description:   "pagination evidence is incomplete or contradictory",
	},
	SubtypePaginationInvalid: {
		Subtype:       SubtypePaginationInvalid,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "上游分页字段类型无效；不要将结果当作完整列表，保留脱敏响应证据后排查上游。",
		Description:   "an upstream pagination field has an invalid local projection type",
	},
	SubtypePaginationIncomplete: {
		Subtype:       SubtypePaginationIncomplete,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "上游表示仍有下一页但没有可用游标；不要将结果当作完整列表，保留脱敏响应证据后排查上游。",
		Description:   "an upstream pagination response cannot be resumed safely",
	},
	SubtypePaginationConflict: {
		Subtype:       SubtypePaginationConflict,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "上游分页终态与续页游标相互矛盾；不要将结果当作完整列表，保留脱敏响应证据后排查上游。",
		Description:   "an upstream pagination response contains contradictory terminal and continuation evidence",
	},
	SubtypeChatSearchIncomplete: {
		Subtype:       SubtypeChatSearchIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留已读取结果和 failures，使用 nextCursor 继续读取；不要把当前结果当作完整搜索结果。",
		Description:   "a group search pagination read stopped before a terminal page was observed",
	},
	SubtypeChatListAllIncomplete: {
		Subtype:       SubtypeChatListAllIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留已读取结果和 failures，使用 nextCursor 继续读取；不要把当前群列表当作完整目录。",
		Description:   "a joined-group pagination read stopped before a terminal page was observed",
	},
	SubtypeFlagListIncomplete: {
		Subtype:       SubtypeFlagListIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留已读取结果和 failures，使用 nextCursor 继续读取；不要把当前收藏列表当作完整。",
		Description:   "a message-favorites pagination read stopped before a terminal page was observed",
	},
	SubtypeChatListIncomplete: {
		Subtype:       SubtypeChatListIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留已读取结果和 failures，使用 nextCursor 继续读取；不要把当前会话列表当作完整。",
		Description:   "a conversation pagination read stopped before a terminal page was observed",
	},
	SubtypeMessagesListDirectIncomplete: {
		Subtype:       SubtypeMessagesListDirectIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留已读取结果和 failures，使用 nextPage 继续读取；不要把当前单聊消息当作完整。",
		Description:   "a direct-message pagination read stopped before a terminal page was observed",
	},
	SubtypeChatMessagesIncomplete: {
		Subtype:       SubtypeChatMessagesIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留已读取结果和 failures，使用 nextPage 继续读取；不要把当前消息读取结果当作完整。",
		Description:   "a chat-message pagination read stopped before a terminal page was observed",
	},
	SubtypeMyGroupsIncomplete: {
		Subtype:       SubtypeMyGroupsIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留已读取结果和 failures，使用 nextCursor 继续读取；不要把当前我的群列表当作完整。",
		Description:   "a personal-group pagination read stopped before a terminal page was observed",
	},
	SubtypeThreadRepliesIncomplete: {
		Subtype:       SubtypeThreadRepliesIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留已读取结果和 failures，使用 nextPage 继续读取；不要把当前话题回复当作完整。",
		Description:   "a thread-replies pagination read stopped before a terminal page was observed",
	},
	SubtypeProjectionUnknown: {
		Subtype:       SubtypeProjectionUnknown,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请记录脱敏响应形状并提交诊断；不要将该结果当作空集合。",
		Description:   "an upstream response cannot be safely projected",
	},
	SubtypeInputReadFailed: {
		Subtype:       SubtypeInputReadFailed,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请检查输入文件路径、读取权限或 stdin 内容后重新执行。",
		Description:   "a local input file or standard input stream could not be read",
	},
	SubtypeInvalidJSONInput: {
		Subtype:       SubtypeInvalidJSONInput,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请修正输入 JSON 的语法和字段形状后重新执行。",
		Description:   "a local JSON input could not be parsed",
	},
	SubtypeFormulaErrorsFound: {
		Subtype:       SubtypeFormulaErrorsFound,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请根据 details 中的错误位置和类型修正公式后，再重新校验。",
		Description:   "formula verification completed and found formula errors",
	},
	SubtypeDownloadOutputUnavailable: {
		Subtype:       SubtypeDownloadOutputUnavailable,
		Category:      CategoryInternal,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请检查本地输出路径、磁盘空间和文件权限；确认文件状态后再决定是否重试。",
		Description:   "a completed download could not be inspected at its local output path",
	},
	SubtypeDownloadSizeMismatch: {
		Subtype:       SubtypeDownloadSizeMismatch,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保留当前文件供核查；确认服务端元数据后以新的输出路径重新下载。",
		Description:   "downloaded bytes do not match upstream file metadata",
	},
	SubtypeVersionNotFound: {
		Subtype:       SubtypeVersionNotFound,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请先查询可用历史版本，选择存在的版本号后再执行预览或回滚。",
		Description:   "the requested document or sheet version does not exist",
	},
	SubtypeTargetTypeMismatch: {
		Subtype:       SubtypeTargetTypeMismatch,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请使用当前命令要求的目标类型或稳定 ID；运行 leaf --help 查看参数说明。",
		Description:   "a target was supplied in a type not accepted by this command",
	},
	SubtypeTargetArgumentsConflict: {
		Subtype:       SubtypeTargetArgumentsConflict,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请只保留一种目标指定方式后重试，避免稳定 ID 与自然语言目标冲突。",
		Description:   "mutually exclusive target selectors were supplied together",
	},
	SubtypeMissingTarget: {
		Subtype:       SubtypeMissingTarget,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请提供目标的稳定 ID 或命令声明的自然语言查询参数后重试。",
		Description:   "no target selector was supplied",
	},
	SubtypeResolutionNotFound: {
		Subtype:       SubtypeResolutionNotFound,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请提供更完整的名称，或直接传入稳定 ID；不要猜测目标。",
		Description:   "deterministic target resolution found no usable candidate",
	},
	SubtypeResolutionAmbiguous: {
		Subtype:       SubtypeResolutionAmbiguous,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请让用户消歧或改用稳定 ID；禁止默认选择第一个候选。",
		Description:   "deterministic target resolution found multiple candidates",
	},
	SubtypeResolutionIncomplete: {
		Subtype:       SubtypeResolutionIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "候选集未证明完整；可重试只读解析，或改用稳定 ID，禁止据此执行写入。",
		Description:   "target-resolution pagination or source coverage was incomplete",
	},
	SubtypeResolutionBatchFailed: {
		Subtype:       SubtypeResolutionBatchFailed,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请逐项修正无法唯一解析的目标，或全部改用稳定 ID 后再执行。",
		Description:   "one or more batch target resolutions failed before execution",
	},
	SubtypeInvalidAITableURL: {
		Subtype:       SubtypeInvalidAITableURL,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请提供受支持的钉钉 AI 表格 HTTPS URL，并检查 base/table/view/record 标识符。",
		Description:   "an AI Table URL could not be parsed into stable target IDs",
	},
	SubtypeTargetNotFound: {
		Subtype:       SubtypeTargetNotFound,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请先查询或核对稳定目标 ID；确认资源存在后再执行写操作。",
		Description:   "a requested target was not identified by a local preflight",
	},
	SubtypeTargetAmbiguous: {
		Subtype:       SubtypeTargetAmbiguous,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "目标不是唯一匹配；请提供稳定 ID 或更精确条件，禁止默认选第一个。",
		Description:   "a target preflight found more than one matching resource",
	},
	SubtypeTargetTypeConflict: {
		Subtype:       SubtypeTargetTypeConflict,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请保持现有目标类型，或选择新目标后再执行；不要原地强制转换资源类型。",
		Description:   "an existing target has an incompatible immutable type",
	},
	SubtypeTargetIncomplete: {
		Subtype:       SubtypeTargetIncomplete,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "目标查询未证明完整；可重试只读预检，确认前不得继续写入。",
		Description:   "an upstream target query was incomplete and cannot prove uniqueness",
	},
	SubtypeTargetInvalidResponse: {
		Subtype:       SubtypeTargetInvalidResponse,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请记录脱敏响应形状并排查上游；不要把未知响应当作目标不存在。",
		Description:   "an upstream target response could not be safely projected",
	},
	SubtypeTargetVerificationFailed: {
		Subtype:       SubtypeTargetVerificationFailed,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "接口未证明目标存在；请核对稳定 ID 或稍后以只读方式重新验证。",
		Description:   "a read-back did not verify the requested target",
	},
	SubtypeKeyValueConflict: {
		Subtype:       SubtypeKeyValueConflict,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请使 cells 内的键字段值与唯一键参数一致后重试。",
		Description:   "a record key conflicts with the value embedded in cells",
	},
	SubtypeAttachmentTokensUnavailable: {
		Subtype:       SubtypeAttachmentTokensUnavailable,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "当前附件缺少安全覆盖写所需 token；请改用 replace 或先人工核查原附件。",
		Description:   "attachment read-back omitted tokens required for a safe replacement write",
	},
	SubtypeUpstreamUnclassified: {
		Subtype:       SubtypeUpstreamUnclassified,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "上游未提供可安全分类的失败；请保留 trace、HTTP/RPC 诊断并核查执行状态，禁止盲目重试写操作。",
		Description:   "an upstream tool-call failure whose detailed code is diagnostic rather than an Agent branch key",
	},
	SubtypeDiscoveryUpstreamUnclassified: {
		Subtype:       SubtypeDiscoveryUpstreamUnclassified,
		Category:      CategoryDiscovery,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "服务发现未获得可安全分类的上游响应；请保留 trace 并核对服务版本或网络后重试只读发现。",
		Description:   "a discovery-path upstream failure whose detailed code is diagnostic rather than an Agent branch key",
	},
	SubtypeUpstreamAuthenticationRequired: {
		Subtype:       SubtypeUpstreamAuthenticationRequired,
		Category:      CategoryAuth,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "上游要求重新认证；请检查登录状态、凭证和租户身份后重新执行。",
		Description:   "the upstream rejected the current authentication state",
	},
	SubtypeUpstreamAuthorizationDenied: {
		Subtype:       SubtypeUpstreamAuthorizationDenied,
		Category:      CategoryAuth,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "当前身份没有访问上游资源的权限；请核对授权范围或切换有权限的身份。",
		Description:   "the upstream denied authorization for the current identity",
	},
	SubtypeToolProtocolIncompatible: {
		Subtype:       SubtypeToolProtocolIncompatible,
		Category:      CategoryDiscovery,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "工具协议或服务版本不兼容；请核对工具名、服务版本或升级到兼容的 dws 版本。",
		Description:   "the upstream rejected the JSON-RPC tool protocol before a tool result was produced",
	},
	SubtypeBackendDependencyUnavailable: {
		Subtype:       SubtypeBackendDependencyUnavailable,
		Category:      CategoryAPI,
		RetryPolicy:   RetryServerDirective,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "MCP 后端依赖暂时不可用；保留 Trace ID，确认服务恢复后再决定是否重试。",
		Description:   "a declared MCP backend dependency was unavailable",
	},
	SubtypeUpstreamRequestRejected: {
		Subtype:       SubtypeUpstreamRequestRejected,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请求被上游拒绝；请核对当前 leaf Help、Schema 和稳定目标 ID 后重试。",
		Description:   "the upstream rejected a tool request without a more specific safe classification",
	},
	SubtypeBlockedFlag: {
		Subtype:       SubtypeBlockedFlag,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "该参数不能被自动归一化；请查看当前 leaf Help 并显式选择正确参数。",
		Description:   "a compatibility flag was intentionally blocked from unsafe automatic normalization",
	},
	SubtypeAmbiguousFlag: {
		Subtype:       SubtypeAmbiguousFlag,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "该参数在当前命令中存在歧义；请查看 leaf Help 并显式选择目标参数。",
		Description:   "a compatibility flag could not be normalized because multiple meanings are plausible",
	},
	SubtypeSkillDownloadInfoUnavailable: {
		Subtype:       SubtypeSkillDownloadInfoUnavailable,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "技能市场未返回可用下载信息；请保留上游错误码，确认网络和登录状态后重试读取。",
		Description:   "a read-only skill-market download-info lookup did not return a usable result",
	},
	SubtypeDocCreateMissingNodeID: {
		Subtype:       SubtypeDocCreateMissingNodeID,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "文档已创建但未返回稳定 node ID；请先定位已创建文档，不要重复创建。",
		Description:   "document creation completed but the response omitted the created node identity",
	},
	SubtypeDocCreateInitialContentFailed: {
		Subtype:       SubtypeDocCreateInitialContentFailed,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "文档已创建但初始内容写入失败；先检查已创建文档或补偿计划，不要重复创建。",
		Description:   "a follow-up initial-content write failed after document creation completed",
	},
	SubtypeDocCheckpointUpdateFailed: {
		Subtype:       SubtypeDocCheckpointUpdateFailed,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "更新前已保存恢复点；请检查已完成步骤并按补偿信息决定是否恢复。",
		Description:   "a checkpoint-protected document update failed after the checkpoint was saved",
	},
	SubtypeDocCheckpointVerificationFailed: {
		Subtype:       SubtypeDocCheckpointVerificationFailed,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "写入可能已完成但读回验证失败；请先检查文档或恢复点，不要盲目重试。",
		Description:   "a checkpoint-protected document update could not be verified after writing",
	},
	SubtypeDocHistoryRevertVerificationFailed: {
		Subtype:       SubtypeDocHistoryRevertVerificationFailed,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "版本回滚可能已完成但读回验证失败；请先检查当前文档，不要重复回滚。",
		Description:   "a document version revert could not be verified after the revert request completed",
	},
	SubtypeEventStopUnverified: {
		Subtype:       SubtypeEventStopUnverified,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "部分订阅可能已取消但停机终态未确认；先运行 dws event status 核对订阅和本地状态，不要盲目重复停止。",
		Description:   "an event-stop composite workflow did not produce a fully verifiable terminal state after at least one control-plane step",
	},
	SubtypeInvalidSuccessType: {
		Subtype:       SubtypeInvalidSuccessType,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "上游 success 字段不是布尔值；写操作先核查目标状态，读取操作保留脱敏响应后排查上游。",
		Description:   "an upstream shortcut response uses a non-boolean success field",
	},
	SubtypeSkillSetupResultInvalid: {
		Subtype:       SubtypeSkillSetupResultInvalid,
		Category:      CategoryInternal,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "Skill 安装结果违反三通道契约；先检查已列出的目标路径和诊断信息，再决定是否人工清理或重试。",
		Description:   "skill setup produced an invalid partial-result projection",
	},
	SubtypeSkillSetupFailed: {
		Subtype:       SubtypeSkillSetupFailed,
		Category:      CategoryInternal,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "Skill 安装未完成任何目标；先检查各目标路径，未知项可能已经发生文件变更。",
		Description:   "skill setup completed no target operation and retained diagnostic facts",
	},
	SubtypeBatchWriteFailed: {
		Subtype:       SubtypeBatchWriteFailed,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "批量写入可能已部分完成；先检查 succeeded 与 failures，只重试尚未确认的目标，不要整体重放。",
		Description:   "a batch mutation completed with one or more target failures",
	},
	SubtypeDocGrantPermissionPartialFailure: {
		Subtype:       SubtypeDocGrantPermissionPartialFailure,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "部分权限可能已写入；先检查当前权限并按 compensation 决定补偿或只重试未完成步骤。",
		Description:   "document permission grants completed before a later role update failed",
	},
	SubtypeDocShareMessageFailed: {
		Subtype:       SubtypeDocShareMessageFailed,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: true,
		DefaultHint:   "部分消息可能已经送达；先检查 recipients 台账，只重试未确认送达的接收人。",
		Description:   "document share-message delivery did not complete for every recipient",
	},
	SubtypeStdioInitializeError: {
		Subtype:       SubtypeStdioInitializeError,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "本地插件 MCP 初始化失败；检查插件进程、配置和日志，恢复后再重新发现或调用工具。",
		Description:   "a local stdio MCP server could not complete initialization",
	},
	SubtypeStdioToolsListError: {
		Subtype:       SubtypeStdioToolsListError,
		Category:      CategoryAPI,
		RetryPolicy:   RetryIdempotentReadOnly,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "本地插件 MCP 无法列出工具；检查插件进程、配置和日志，恢复后再重新发现工具。",
		Description:   "a local stdio MCP server could not list its declared tools",
	},
	SubtypeStdioError: {
		Subtype:       SubtypeStdioError,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "插件工具调用的远端终态未确认；写操作先核查目标状态，读取操作保留诊断后再决定是否重试。",
		Description:   "a local stdio MCP tool call ended without a trustworthy terminal result",
	},
	SubtypeMCPToolError: {
		Subtype:       SubtypeMCPToolError,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "MCP 工具返回错误；写操作先核查目标状态，读取操作检查参数和上游诊断后再决定是否重试。",
		Description:   "an MCP tool returned an explicit error result",
	},
	SubtypeEmptyToolResponse: {
		Subtype:       SubtypeEmptyToolResponse,
		Category:      CategoryAPI,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "写调用没有返回可验证业务结果；先核查目标状态，不要因空响应直接重放。",
		Description:   "a write-capable MCP tool returned no business result",
	},
	SubtypePluginToolNotFound: {
		Subtype:       SubtypePluginToolNotFound,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "插件未声明该工具；重新发现插件工具并核对当前命令、插件版本和输入 Schema。",
		Description:   "a requested plugin tool was absent from its declared tools/list response",
	},
	SubtypePluginInputSchemaInvalid: {
		Subtype:       SubtypePluginInputSchemaInvalid,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "插件输入不符合其声明的 Schema；核对字段名、类型和必填项后重新调用。",
		Description:   "plugin input could not be normalized or validated against the declared schema",
	},
	SubtypeUnsupportedFormat: {
		Subtype:       SubtypeUnsupportedFormat,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "请使用当前命令支持的输出格式；运行当前命令的 --help 查看可用格式。",
		Description:   "the requested output format is not supported by the command contract",
	},
	SubtypeInvalidAgentCode: {
		Subtype:       SubtypeInvalidAgentCode,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "将 --agentCode 或 DINGTALK_DWS_AGENTCODE 设置为 1–64 位的字母、数字、下划线或连字符后重试。",
		Description:   "the requested PAT agent code failed local syntax validation",
	},
	SubtypeInvalidAgentHost: {
		Subtype:       SubtypeInvalidAgentHost,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "将 DWS_AGENT_HOST 设置为 1–64 位小写字母、数字、下划线或连字符；不需要时请清空该环境变量。",
		Description:   "the caller-declared Agent host failed local syntax validation",
	},
	SubtypeInvalidAgentProduct: {
		Subtype:       SubtypeInvalidAgentProduct,
		Category:      CategoryValidation,
		RetryPolicy:   RetryNever,
		RequireHint:   true,
		RequireAction: false,
		DefaultHint:   "将 DWS_AGENT_PRODUCT 设置为 1–64 位字母、数字、下划线或连字符；不需要时请清空该环境变量。",
		Description:   "the caller-declared Agent product failed local syntax validation",
	},
}

// LookupSubtype returns the immutable descriptor for an approved subtype.
func LookupSubtype(subtype Subtype) (SubtypeDescriptor, bool) {
	descriptor, ok := subtypeRegistry[subtype]
	return descriptor, ok
}

// IsRegisteredSubtype reports whether a value is an approved stable subtype.
// It is intentionally not used to reject legacy free-form reasons at runtime;
// doing so before each command is migrated would be a wire-breaking change.
func IsRegisteredSubtype(subtype string) bool {
	_, ok := LookupSubtype(Subtype(subtype))
	return ok
}

// WithSubtype records a registered, stable subtype. New production code must
// prefer this over WithReason("literal"); WithReason remains solely for
// compatibility and for values still under Agent review. It deliberately keeps
// the existing string-valued Reason, Category, and exit-code semantics. A
// registered descriptor may add a safe fallback hint; that additive recovery
// guidance is intentional, and an adjacent or later WithHint always replaces
// it with command-specific advice.
func WithSubtype(subtype Subtype) Option {
	return func(err *Error) {
		err.Reason = string(subtype)
		if err.Hint != "" {
			return
		}
		if descriptor, ok := LookupSubtype(subtype); ok {
			err.Hint = descriptor.DefaultHint
		}
	}
}
