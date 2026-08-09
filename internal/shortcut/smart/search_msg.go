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

package smart

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	chatshortcut "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/chat"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/chatmsg"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/targetresolver"
)

const searchMsgUseWhen = "当你要按关键词、发送者、@对象、会话、消息类型或机器人来源组合搜索 IM 消息时使用；会话与发送者过滤使用稳定 ID。默认查询近 7 天，也可指定精确起止时间。" +
	"--page-all 会连续拉取游标页，默认再按消息 ID 分批富化详情；任何续页或富化失败都会保留已取得结果并返回逐项失败 ledger。" +
	"meta.pagination.endpoint_exhausted 只表示服务端游标耗尽；indexCoverageKnown 为 false 时，空结果不等于业务数据确定不存在。" +
	"--download-resources 使用安全本地路径、默认不覆盖和原子落盘。"

// SearchMsg is the semantic message-search entry point. It exposes the native
// IM search dimensions, can exhaust cursor pagination, and enriches sparse
// search hits through list_messages_by_ids in chunks of 50. A later-page or
// enrichment failure never turns a partial result into a false success. The
// output separates endpoint pagination exhaustion from unknown search-index
// coverage and carries an explicit failure ledger.
var SearchMsg = shortcut.Shortcut{
	// This is the default Agent route for multi-dimensional IM search. Its
	// pagination and failure ledger have an activated, single machine contract:
	// callers keep using --format json; they never select an output generation.
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "chat",
	Command:       "+search-msg",
	Product:       "im",
	Description:   "按稳定 ID 和内容等条件跨会话搜索消息，可全量翻页并批量富化",
	Intent:        searchMsgUseWhen,
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "chat",
			Name:           "shortcut_search_msg",
			CanonicalPath:  "chat.shortcut_search_msg",
			CLIPath:        "chat +search-msg",
			PrimaryCLIPath: "chat +search-msg",
		},
		Description: "按稳定 ID 和内容等条件跨会话搜索消息，可全量翻页并批量富化",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed search adapter: it combines filters, cursor pagination, batched mget enrichment, stable projection, completeness accounting, and optional safe resource downloads.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "按稳定 ID 和内容等条件跨会话搜索消息，可全量翻页并批量富化",
			UseWhen:      []string{searchMsgUseWhen},
			AvoidWhen:    []string{"只想读取一个已知会话的连续历史时使用 +chat-messages；已有精确消息 ID 时使用 +messages-mget"},
			Examples: []string{
				"dws chat +search-msg --query \"周报\" --senders <openDingTalkId> --days 3 --page-all",
				"dws chat +search-msg --group <openConversationId> --message-type file --download-resources --output-dir ./downloads",
			},
		},
	},
	Flags: append([]shortcut.Flag{
		{Name: "query", Type: shortcut.FlagString, Desc: "搜索关键词"},
		{Name: "keyword", Type: shortcut.FlagString, Desc: "--query 的别名", Hidden: true},
		{Name: "text", Type: shortcut.FlagString, Desc: "--query 的兼容别名", Hidden: true},
		{Name: "text-query", Type: shortcut.FlagString, Desc: "--query 的兼容别名", Hidden: true},
		{Name: "group", Type: shortcut.FlagString, Desc: "单个会话 openConversationId"},
		{Name: "conversation-id", Type: shortcut.FlagString, Desc: "--group 的别名", Hidden: true},
		{Name: "id", Type: shortcut.FlagString, Desc: "--group 的别名", Hidden: true},
		{Name: "groups", Type: shortcut.FlagStringSlice, Desc: "多个会话 openConversationId"},
		{Name: "chat-id", Type: shortcut.FlagStringSlice, Desc: "--groups 的 lark-cli 对齐别名"},
		{Name: "chat-query", Type: shortcut.FlagStringSlice, Desc: "按群名唯一解析会话过滤条件（可选，可重复或逗号分隔）"},
		{Name: "senders", Type: shortcut.FlagStringSlice, Desc: "发送者 userId/openDingTalkId 列表"},
		{Name: "sender", Type: shortcut.FlagStringSlice, Desc: "--senders 的 lark-cli 对齐别名"},
		{Name: "sender-query", Type: shortcut.FlagStringSlice, Desc: "按姓名唯一解析发送者过滤条件（可选，可重复或逗号分隔）"},
		{Name: "at-me", Type: shortcut.FlagBool, Desc: "只搜索 @我 的消息"},
		{Name: "is-at-me", Type: shortcut.FlagBool, Desc: "--at-me 的 lark-cli 对齐别名"},
		{Name: "at-ids", Type: shortcut.FlagStringSlice, Desc: "@对象 userId/openDingTalkId 列表"},
		{Name: "message-type", Type: shortcut.FlagString, Desc: "下层消息类型过滤值（以当前 IM Schema 为准）"},
		{Name: "only-robot", Type: shortcut.FlagBool, Desc: "只搜索机器人消息"},
		{Name: "conversation-type", Type: shortcut.FlagString, Desc: "下层会话类型过滤值（以当前 IM Schema 为准）"},
		{Name: "chat-type", Type: shortcut.FlagString, Desc: "--conversation-type 的 lark-cli 对齐别名"},
		{Name: "days", Type: shortcut.FlagInt, Desc: "默认时间窗的回溯天数", Default: "7"},
		{Name: "start", Type: shortcut.FlagString, Desc: "精确开始时间（RFC3339，需与 --end 一起传；也支持 --end-time）"},
		{Name: "start-time", Type: shortcut.FlagString, Desc: "--start 的 lark-cli 对齐别名（RFC3339，需与 --end 一起传；也支持 --end-time）"},
		{Name: "end", Type: shortcut.FlagString, Desc: "精确结束时间（RFC3339，需与 --start 一起传；也支持 --start-time）"},
		{Name: "end-time", Type: shortcut.FlagString, Desc: "--end 的 lark-cli 对齐别名（RFC3339，需与 --start 一起传；也支持 --start-time）"},
		{Name: "order", Type: shortcut.FlagString, Enum: []string{"asc", "desc"}, Desc: "按消息创建时间稳定排列输出 asc/desc（可选，默认 desc）"},
		{Name: "sort", Type: shortcut.FlagString, Enum: []string{"asc", "desc"}, Desc: "--order 的 lark-cli 对齐别名（可选）"},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "每页返回数量（1-100）", Default: "100"},
		{Name: "page-size", Type: shortcut.FlagInt, Desc: "--limit 的 lark-cli 对齐别名（1-100）"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标，翻页传上次 meta.pagination.next_token", Default: "0"},
		{Name: "page-token", Type: shortcut.FlagString, Desc: "--cursor 的 lark-cli 对齐别名"},
		{Name: "page-all", Type: shortcut.FlagBool, Desc: "自动连续拉取所有游标页"},
		{Name: "page-limit", Type: shortcut.FlagInt, Desc: "--page-all 的最大页数（1-40）", Default: "20"},
		{Name: "no-enrich", Type: shortcut.FlagBool, Desc: "不再按消息 ID 批量查询完整详情"},
		{Name: "no-reactions", Type: shortcut.FlagBool, Desc: "不输出命中消息的 reaction（默认输出）"},
	}, chatshortcut.MessageResourceDownloadFlags()...),
	Constraints: append([]shortcut.Constraint{
		{
			Kind:        shortcut.ConstraintAtLeastOne,
			Flags:       []string{"query", "keyword", "text", "text-query", "group", "conversation-id", "id", "groups", "chat-id", "chat-query", "senders", "sender", "sender-query", "at-me", "is-at-me", "at-ids", "message-type", "only-robot", "conversation-type", "chat-type"},
			Description: "至少指定一个内容、身份、会话或消息类型过滤条件",
		},
		{
			Kind:        shortcut.ConstraintCustom,
			Flags:       []string{"start", "start-time"},
			Description: "需与 --end 一起传",
		},
		{
			Kind:        shortcut.ConstraintCustom,
			Flags:       []string{"end", "end-time"},
			Description: "需与 --start 一起传",
		},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"groups", "chat-id"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"start", "start-time"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"end", "end-time"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"order", "sort"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"senders", "sender"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"at-me", "is-at-me"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"conversation-type", "chat-type"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"query", "keyword", "text", "text-query"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"limit", "page-size"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"cursor", "page-token"}},
	}, chatshortcut.MessageResourceDownloadConstraints()...),
	Tips: []string{
		`dws chat +search-msg --group <openConversationId> --query "changefree"`,
		`dws chat +search-msg --senders <openDingTalkId> --at-me --days 3 --page-all`,
	},
	Validate: validateSearchMsgWithResources,
	Execute: func(rt *shortcut.RuntimeContext) error {
		params, resolvedFilters, err := searchMsgParams(rt)
		if err != nil {
			return err
		}

		pageLimit := 1
		if rt.Bool("page-all") {
			pageLimit = rt.Int("page-limit")
		}
		pageLedger, err := output.NewPageLedger(pageLimit)
		if err != nil {
			return apperrors.NewInternal("初始化消息搜索分页账本失败", apperrors.WithCause(err))
		}
		rollout := output.CommandRollout(rt.Command())
		usesUnified := output.UsesUnifiedResult(rt.Command())
		shadowsUnified := usesUnified || rollout == output.RolloutDualValidate
		cursor := rt.StrFirst("page-token", "cursor")
		messages := make([]map[string]any, 0)
		seen := map[string]bool{}
		failures := make([]map[string]any, 0)
		pagesFetched := 0
		resultPartial := false
		endpointExhausted := false
		hasMore := false
		nextCursor := ""
		paginationKnown := true

		for pagesFetched < pageLimit {
			params["cursor"] = cursor
			data, callErr := rt.CallMCPData("im", "search_messages", params)
			if callErr != nil {
				if shadowsUnified {
					if recordErr := pageLedger.RecordFailure(cursor, searchMsgReadFailureInfo(callErr)); recordErr != nil {
						return apperrors.NewInternal("记录消息搜索读取失败状态失败", apperrors.WithCause(recordErr))
					}
				}
				if pagesFetched == 0 {
					if usesUnified {
						return searchMsgOutputResult(rt, nil, pageLedger)
					}
					if rollout == output.RolloutDualValidate {
						if resultErr := searchMsgValidateShadowResult(nil, pageLedger, rt.DryRun()); resultErr != nil {
							return resultErr
						}
					}
					return callErr
				}
				failures = append(failures, map[string]any{
					"stage":  "search-page",
					"cursor": cursor,
					"error":  callErr.Error(),
				})
				resultPartial = true
				break
			}
			pagesFetched++
			for _, message := range searchMsgItems(data) {
				messageID := strings.TrimSpace(fmt.Sprint(searchMsgMessageID(message)))
				if messageID != "" && messageID != "<nil>" {
					if seen[messageID] {
						continue
					}
					seen[messageID] = true
				}
				messages = append(messages, message)
			}

			var unifiedNextCursor string
			var unifiedTerminal bool
			if shadowsUnified {
				unifiedNextCursor, unifiedTerminal, err = observeSearchMsgUnifiedPage(pageLedger, cursor, data, !rt.Bool("no-reactions"))
				if err != nil {
					return apperrors.NewInternal("记录消息搜索分页证据失败", apperrors.WithCause(err))
				}
				if usesUnified && unifiedTerminal {
					break
				}
			}

			page := chatmsg.Pagination(data)
			hasMoreValue, hasMoreKnown := page["hasMore"].(bool)
			nextCursor = searchMsgCursorText(page["nextCursor"])
			if !hasMoreKnown {
				if nextCursor != "" && nextCursor != "<nil>" {
					hasMoreValue = true
				} else {
					if usesUnified {
						// A single page remains truthful, but does not prove endpoint
						// exhaustion. A caller that explicitly requested page-all gets
						// a partial result with the missing boundary recorded below.
						if rt.Bool("page-all") {
							if recordErr := pageLedger.RecordBoundaryFailure(searchMsgPaginationFailureInfo("下层未返回 hasMore 或 nextCursor，无法继续全量分页")); recordErr != nil {
								return apperrors.NewInternal("记录消息搜索未知分页边界失败", apperrors.WithCause(recordErr))
							}
						}
						break
					}
					failures = append(failures, map[string]any{
						"stage": "search-pagination",
						"error": "下层未返回 hasMore 或 nextCursor，无法证明结果完整",
					})
					paginationKnown = false
					resultPartial = true
					break
				}
			}
			hasMore = hasMoreValue
			if !hasMore {
				endpointExhausted = true
			}
			if !rt.Bool("page-all") || !hasMore {
				break
			}
			if nextCursor == "" || nextCursor == "<nil>" || nextCursor == cursor {
				if usesUnified {
					// The observer already recorded a typed boundary failure while
					// preserving this successfully decoded page.
					break
				}
				failures = append(failures, map[string]any{
					"stage": "search-page",
					"error": "下层返回 hasMore=true，但缺少可继续且会前进的 nextCursor",
				})
				resultPartial = true
				break
			}
			if usesUnified && unifiedNextCursor != "" {
				cursor = unifiedNextCursor
				continue
			}
			cursor = nextCursor
		}
		if rt.Bool("page-all") && hasMore && pagesFetched == pageLimit {
			if !usesUnified {
				failures = append(failures, map[string]any{
					"stage": "search-page-limit",
					"error": fmt.Sprintf("达到 --page-limit=%d，仍有更多结果", pageLimit),
				})
				resultPartial = true
			}
		}

		enrichedCount := 0
		if !rt.Bool("no-enrich") && len(messages) > 0 {
			var enrichFailures []map[string]any
			var enrichFailureInfos []*output.ErrorInfo
			messages, enrichedCount, enrichFailures, enrichFailureInfos = enrichSearchMessages(rt, messages)
			failures = append(failures, enrichFailures...)
			if len(enrichFailures) > 0 {
				resultPartial = true
				if shadowsUnified {
					if len(enrichFailureInfos) != len(enrichFailures) {
						return apperrors.NewInternal("消息搜索富化失败明细与类型化结果数量不一致")
					}
					for _, failureInfo := range enrichFailureInfos {
						if recordErr := pageLedger.RecordPostPageFailure(failureInfo); recordErr != nil {
							return apperrors.NewInternal("记录消息搜索富化失败状态失败", apperrors.WithCause(recordErr))
						}
					}
				}
			}
		}

		order := strings.ToLower(strings.TrimSpace(rt.StrFirst("order", "sort")))
		if order == "" {
			order = "desc"
		}
		sortMessagesByCreateTimeStable(messages, order)
		results := make([]map[string]any, 0, len(messages))
		for _, m := range messages {
			results = append(results, searchMsgProjectWithReactions(m, !rt.Bool("no-reactions")))
		}
		payload := map[string]any{
			"count":              len(results),
			"messages":           results,
			"pagesFetched":       pagesFetched,
			"enrichedCount":      enrichedCount,
			"endpointExhausted":  endpointExhausted,
			"indexCoverageKnown": false,
			"coverageScope":      "server_search_index",
			"partial":            resultPartial,
			"hasMore":            hasMore,
			"nextCursor":         "",
			"paginationKnown":    paginationKnown,
			"failedCount":        len(failures),
			"failures":           failures,
			"queryRange":         searchMessageQueryRange(params, order),
		}
		if len(resolvedFilters.Senders) > 0 {
			payload["resolvedFilters"] = resolvedFilters
		}
		if hasMore && nextCursor != "" && nextCursor != "<nil>" {
			payload["nextCursor"] = nextCursor
		}
		if rt.Bool("download-resources") {
			resourceLedger := chatshortcut.DownloadMessageResources(rt, messages, "")
			chatshortcut.AttachMessageResourceDownloads(payload, resourceLedger)
			if shadowsUnified {
				if failureInfo := searchMsgResourceDownloadFailureInfo(resourceLedger); failureInfo != nil {
					if recordErr := pageLedger.RecordPostPageFailure(failureInfo); recordErr != nil {
						return apperrors.NewInternal("记录消息搜索资源下载失败状态失败", apperrors.WithCause(recordErr))
					}
				}
			}
		}
		if usesUnified {
			return searchMsgOutputResult(rt, payload, pageLedger)
		}
		if rollout == output.RolloutDualValidate {
			if resultErr := searchMsgValidateShadowResult(payload, pageLedger, rt.DryRun()); resultErr != nil {
				return resultErr
			}
		}
		return rt.Output(payload)
	},
}

// searchMsgOutputResult turns the one internal page ledger into the framework
// result. It is intentionally the only activated exit: error, partial, and
// pagination results therefore have the same stdout/exit-code contract.
func searchMsgOutputResult(rt *shortcut.RuntimeContext, payload map[string]any, pageLedger *output.PageLedger) error {
	result, err := searchMsgUnifiedResult(pageLedger, payload, rt.DryRun())
	if err != nil {
		return apperrors.NewInternal("生成消息搜索统一分页结果失败", apperrors.WithCause(err))
	}
	return rt.OutputResult(payload, result)
}

func searchMsgValidateShadowResult(payload map[string]any, pageLedger *output.PageLedger, dryRun bool) error {
	result, err := searchMsgUnifiedResult(pageLedger, payload, dryRun)
	if err != nil {
		return apperrors.NewInternal("生成消息搜索影子分页结果失败", apperrors.WithCause(err))
	}
	if err := output.ValidateResult(result); err != nil {
		return apperrors.NewInternal("消息搜索影子结果违反统一契约", apperrors.WithCause(err))
	}
	return nil
}

// searchMsgUnifiedResult exposes only business data in data. Legacy transport
// fields such as contractVersion/partial/hasMore/nextCursor are deliberately
// not copied: the framework owns outcome and meta.pagination.
func searchMsgUnifiedResult(pageLedger *output.PageLedger, payload map[string]any, dryRun bool) (output.CommandResult, error) {
	if pageLedger == nil {
		return nil, fmt.Errorf("missing pagination ledger")
	}
	data := map[string]any{}
	if payload != nil {
		for _, key := range []string{"messages", "count", "enrichedCount", "indexCoverageKnown", "coverageScope", "queryRange", "resolvedFilters", "resourceDownloads"} {
			if value, ok := payload[key]; ok {
				data[key] = value
			}
		}
	}
	if pageLedger.State() == output.PageStateUnknown {
		data["pagination_known"] = false
	}
	options := make([]output.ResultOption, 0, 1)
	if dryRun {
		options = append(options, output.WithDryRun())
	}
	return pageLedger.Result(data, options...)
}

func searchMsgReadFailureInfo(err error) *output.ErrorInfo {
	message := "消息搜索分页读取失败"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	info := &output.ErrorInfo{
		Type:             "api",
		Message:          message,
		Hint:             "从失败游标继续；不要重放已成功读取的页面。",
		Operation:        "im/search_messages",
		Origin:           "mcp_gateway",
		Stage:            "pagination_read",
		ExecutionStarted: &started,
		Retryable:        true, // this command only performs idempotent reads
	}
	return shortcut.PreserveTypedErrorInfo(info, err)
}

func searchMsgPaginationFailureInfo(message string) *output.ErrorInfo {
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypePaginationInconsistent),
		Message:          strings.TrimSpace(message),
		Hint:             "保留已读取页面；不要把当前结果解释为 endpoint 已耗尽。",
		Operation:        "im/search_messages",
		Origin:           "mcp_gateway",
		Stage:            "pagination_projection",
		ExecutionStarted: &started,
	}
}

func searchMsgProjectionFailureInfo(err error) *output.ErrorInfo {
	message := "消息搜索响应无法可靠投影"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypeProjectionUnknown),
		Message:          message,
		Hint:             "检查服务端返回结构；不要把未知响应形状解释为空搜索结果。",
		Operation:        "im/search_messages",
		Origin:           "mcp_gateway",
		Stage:            "projection",
		ExecutionStarted: &started,
	}
}

func searchMsgEnrichmentReadFailureInfo(messageIDs []string, err error) *output.ErrorInfo {
	message := "消息搜索富化读取失败"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	info := &output.ErrorInfo{
		Type:             "api",
		Message:          message,
		Hint:             "保留搜索命中；只重新读取该失败批次的消息详情。",
		Operation:        "im/list_messages_by_ids",
		Origin:           "mcp_gateway",
		Stage:            "message_enrichment",
		ExecutionStarted: &started,
		Retryable:        true, // idempotent read
		Details: map[string]any{
			"message_ids": append([]string(nil), messageIDs...),
		},
	}
	return shortcut.PreserveTypedErrorInfo(info, err)
}

func searchMsgEnrichmentProjectionFailureInfo(messageIDs []string) *output.ErrorInfo {
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypeProjectionUnknown),
		Message:          "消息详情批量读取未返回全部请求消息",
		Hint:             "保留已富化消息；只核查并重新读取缺失的消息 ID。",
		Operation:        "im/list_messages_by_ids",
		Origin:           "mcp_gateway",
		Stage:            "message_enrichment_projection",
		ExecutionStarted: &started,
		Details: map[string]any{
			"missing_message_ids": append([]string(nil), messageIDs...),
		},
	}
}

func searchMsgResourceDownloadFailureInfo(ledger map[string]any) *output.ErrorInfo {
	failedCount := searchMsgLedgerCount(ledger["failedCount"])
	if failedCount == 0 {
		return nil
	}
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Message:          fmt.Sprintf("消息搜索资源下载失败：%d 个资源未完成", failedCount),
		Hint:             "保留已读取消息和已下载文件；查看失败资源后仅处理失败项。",
		Operation:        "chat/message_resource_download",
		Origin:           "local_resource_download",
		Stage:            "resource_download",
		ExecutionStarted: &started,
		Details: map[string]any{
			"resource_downloads": ledger,
		},
	}
}

func searchMsgLedgerCount(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

// observeSearchMsgUnifiedPage converts IM's cursor fields into narrow framework
// pagination evidence. Its strict message projection deliberately distinguishes
// an empty, recognized list from an unknown response shape: the latter must
// never become a successful empty search result.
func observeSearchMsgUnifiedPage(
	pageLedger *output.PageLedger,
	cursor string,
	data map[string]any,
	includeReactions bool,
) (nextToken string, terminal bool, err error) {
	items, projectionErr := searchMsgItemsStrict(data)
	if projectionErr != nil {
		if recordErr := pageLedger.RecordFailure(cursor, searchMsgProjectionFailureInfo(projectionErr)); recordErr != nil {
			return "", true, recordErr
		}
		return "", true, nil
	}
	evidence := output.PageEvidence{
		Cursor: cursor,
		Items:  len(items),
		Data:   map[string]any{"messages": searchMsgProjectItems(items, includeReactions)},
	}
	page := chatmsg.Pagination(data)
	hasMore, known := page["hasMore"].(bool)
	if !known {
		token := searchMsgCursorText(page["nextCursor"])
		if token != "" {
			evidence.NextToken = token
			if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
				return "", true, searchMsgRecordUnifiedBoundary(pageLedger, evidence, observeErr.Error())
			}
			return token, false, nil
		}
		if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
			return "", true, observeErr
		}
		return "", false, nil
	}
	if !hasMore {
		if searchMsgCursorText(page["nextCursor"]) != "" {
			if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
				return "", true, searchMsgRecordUnifiedBoundary(pageLedger, evidence, observeErr.Error())
			}
			if recordErr := pageLedger.RecordBoundaryFailure(searchMsgPaginationFailureInfo("hasMore=false，但同时携带可用 nextCursor")); recordErr != nil {
				return "", true, recordErr
			}
			return "", true, nil
		}
		more := false
		evidence.HasMore = &more
		if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
			return "", false, observeErr
		}
		return "", false, nil
	}

	token := searchMsgCursorText(page["nextCursor"])
	if token == "" {
		if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
			return "", true, observeErr
		}
		if recordErr := pageLedger.RecordBoundaryFailure(searchMsgPaginationFailureInfo("hasMore=true，但缺少可继续的 nextCursor")); recordErr != nil {
			return "", true, recordErr
		}
		return "", true, nil
	}
	more := true
	evidence.HasMore = &more
	evidence.NextToken = token
	if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
		return "", true, searchMsgRecordUnifiedBoundary(pageLedger, evidence, observeErr.Error())
	}
	return token, false, nil
}

func searchMsgRecordUnifiedBoundary(pageLedger *output.PageLedger, evidence output.PageEvidence, message string) error {
	evidence.HasMore = nil
	evidence.NextToken = ""
	if err := pageLedger.ObservePage(evidence); err != nil {
		return err
	}
	return pageLedger.RecordBoundaryFailure(searchMsgPaginationFailureInfo(message))
}

func searchMsgCursorText(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" || text == "0" {
		return ""
	}
	return text
}

func searchMsgProjectItems(items []map[string]any, includeReactions bool) []map[string]any {
	projected := make([]map[string]any, 0, len(items))
	for _, item := range items {
		projected = append(projected, searchMsgProjectWithReactions(item, includeReactions))
	}
	return projected
}

func validateSearchMsgWithResources(rt *shortcut.RuntimeContext) error {
	if err := validateSearchMsg(rt); err != nil {
		return err
	}
	return chatshortcut.ValidateMessageResourceDownload(rt)
}

func validateSearchMsg(rt *shortcut.RuntimeContext) error {
	hasFilter := rt.StrFirst("query", "keyword", "text", "text-query", "group", "conversation-id", "id", "message-type", "conversation-type", "chat-type") != "" ||
		len(rt.StrSlice("groups")) > 0 ||
		len(rt.StrSlice("chat-id")) > 0 ||
		len(rt.StrSlice("chat-query")) > 0 ||
		len(rt.StrSlice("senders")) > 0 ||
		len(rt.StrSlice("sender")) > 0 ||
		len(rt.StrSlice("sender-query")) > 0 ||
		len(rt.StrSlice("at-ids")) > 0 ||
		rt.Bool("at-me") ||
		rt.Bool("is-at-me") ||
		rt.Bool("only-robot")
	if !hasFilter {
		return apperrors.NewValidation("至少指定一个过滤条件，例如 --query、--group、--senders、--at-me 或 --message-type")
	}
	if rt.Changed("start") != rt.Changed("end") {
		return apperrors.NewValidation("--start 与 --end 必须同时指定")
	}
	if days := rt.Int("days"); days < 1 || days > 3650 {
		return apperrors.NewValidation("--days 必须在 1-3650 之间")
	}
	limit := rt.IntFirst("limit", "page-size")
	if limit < 1 || limit > 100 {
		return apperrors.NewValidation("--limit 必须在 1-100 之间")
	}
	if pageLimit := rt.Int("page-limit"); pageLimit < 1 || pageLimit > 40 {
		return apperrors.NewValidation("--page-limit 必须在 1-40 之间")
	}
	return nil
}

type searchResolvedFilters struct {
	Senders []targetresolver.UserResolution `json:"senders,omitempty"`
}

func searchMsgParams(rt *shortcut.RuntimeContext) (map[string]any, searchResolvedFilters, error) {
	params := map[string]any{"limit": rt.IntFirst("limit", "page-size")}
	resolvedFilters := searchResolvedFilters{}
	if value := rt.StrFirst("query", "keyword", "text", "text-query"); value != "" {
		params["keyword"] = value
	}
	conversationIDs := append([]string{}, rt.StrSlice("groups")...)
	conversationIDs = append(conversationIDs, rt.StrSlice("chat-id")...)
	if value := rt.StrFirst("group", "conversation-id", "id"); value != "" {
		conversationIDs = append(conversationIDs, value)
	}
	if queries := rt.StrSlice("chat-query"); len(queries) > 0 {
		for _, query := range queries {
			resolved, err := targetresolver.ResolveChatTarget(rt, "", query)
			if err != nil {
				return nil, searchResolvedFilters{}, err
			}
			conversationIDs = append(conversationIDs, resolved.Selected.OpenConversationID)
		}
	}
	if values := uniqueSearchStrings(conversationIDs); len(values) > 0 {
		params["openConversationIds"] = values
	}
	senders := append([]string{}, rt.StrSlice("senders")...)
	senders = append(senders, rt.StrSlice("sender")...)
	if queries := rt.StrSlice("sender-query"); len(queries) > 0 {
		resolvedUsers, err := targetresolver.ResolveUsers(rt, queries, targetresolver.IdentityAny)
		if err != nil {
			return nil, searchResolvedFilters{}, err
		}
		for _, resolved := range resolvedUsers {
			resolvedFilters.Senders = append(resolvedFilters.Senders, resolved)
			identity := resolved.Selected.OpenDingTalkID
			if identity == "" {
				identity = resolved.Selected.UserID
			}
			senders = append(senders, identity)
		}
	}
	appendSearchActorIDs(params, senders, "senderUserIds", "senderOpenDingTakIds")
	appendSearchActorIDs(params, rt.StrSlice("at-ids"), "atUserIds", "atOpenDingTakIds")
	if rt.Bool("at-me") || rt.Bool("is-at-me") {
		params["atMe"] = true
	}
	if value := rt.Str("message-type"); value != "" {
		params["messageType"] = value
	}
	if rt.Changed("only-robot") {
		params["onlyRobotMessages"] = rt.Bool("only-robot")
	}
	if value := rt.StrFirst("conversation-type", "chat-type"); value != "" {
		params["searchConvType"] = value
	}

	startValue := rt.StrFirst("start", "start-time")
	endValue := rt.StrFirst("end", "end-time")
	if startValue != "" && endValue != "" {
		start, err := time.Parse(time.RFC3339, rt.Str("start"))
		if rt.Str("start") == "" {
			start, err = time.Parse(time.RFC3339, startValue)
		}
		if err != nil {
			return nil, searchResolvedFilters{}, apperrors.NewValidation(fmt.Sprintf("--start 必须是 RFC3339 时间: %v", err))
		}
		end, err := time.Parse(time.RFC3339, endValue)
		if err != nil {
			return nil, searchResolvedFilters{}, apperrors.NewValidation(fmt.Sprintf("--end 必须是 RFC3339 时间: %v", err))
		}
		if !end.After(start) {
			return nil, searchResolvedFilters{}, apperrors.NewValidation("--end 必须晚于 --start")
		}
		params["startTime"] = start.UnixMilli()
		params["endTime"] = end.UnixMilli()
	} else {
		now := time.Now()
		params["startTime"] = now.AddDate(0, 0, -rt.Int("days")).UnixMilli()
		params["endTime"] = now.UnixMilli()
	}
	return params, resolvedFilters, nil
}

func appendSearchActorIDs(params map[string]any, values []string, userKey, openIDKey string) {
	var userIDs, openIDs []string
	for _, value := range uniqueSearchStrings(values) {
		if len(value) > 0 && (value[0] == 'D' || value[0] == 'd') {
			openIDs = append(openIDs, value)
		} else {
			userIDs = append(userIDs, value)
		}
	}
	if len(userIDs) > 0 {
		params[userKey] = userIDs
	}
	if len(openIDs) > 0 {
		params[openIDKey] = openIDs
	}
}

func uniqueSearchStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func enrichSearchMessages(rt *shortcut.RuntimeContext, messages []map[string]any) ([]map[string]any, int, []map[string]any, []*output.ErrorInfo) {
	detailsByID := map[string]map[string]any{}
	failures := make([]map[string]any, 0)
	failureInfos := make([]*output.ErrorInfo, 0)
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		if id := strings.TrimSpace(fmt.Sprint(searchMsgMessageID(message))); id != "" && id != "<nil>" {
			ids = append(ids, id)
		}
	}
	ids = uniqueSearchStrings(ids)
	for start := 0; start < len(ids); start += 50 {
		end := start + 50
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		data, err := rt.CallMCPData("im", "list_messages_by_ids", map[string]any{"openMsgIds": chunk})
		if err != nil {
			failures = append(failures, map[string]any{
				"stage":      "message-enrichment",
				"messageIds": chunk,
				"error":      err.Error(),
			})
			failureInfos = append(failureInfos, searchMsgEnrichmentReadFailureInfo(chunk, err))
			continue
		}
		foundInChunk := map[string]bool{}
		for _, detail := range searchMsgItems(data) {
			if id := strings.TrimSpace(fmt.Sprint(searchMsgMessageID(detail))); id != "" && id != "<nil>" {
				detailsByID[id] = detail
				foundInChunk[id] = true
			}
		}
		missing := make([]string, 0)
		for _, id := range chunk {
			if !foundInChunk[id] {
				missing = append(missing, id)
			}
		}
		if len(missing) > 0 {
			failures = append(failures, map[string]any{
				"stage":             "message-enrichment",
				"missingMessageIds": missing,
				"error":             "mget 未返回全部请求消息",
			})
			failureInfos = append(failureInfos, searchMsgEnrichmentProjectionFailureInfo(missing))
		}
	}

	enriched := 0
	out := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		id := strings.TrimSpace(fmt.Sprint(searchMsgMessageID(message)))
		detail := detailsByID[id]
		if detail == nil {
			out = append(out, message)
			continue
		}
		merged := make(map[string]any, len(message)+len(detail))
		for key, value := range message {
			merged[key] = value
		}
		for key, value := range detail {
			merged[key] = value
		}
		out = append(out, merged)
		enriched++
	}
	return out, enriched, failures, failureInfos
}

// searchMsgItems locates the message list inside a search_messages_by_keyword
// response, probing common container keys at the top level and nested under
// "result". Returns nil when no list is found.
func searchMsgItems(data map[string]any) []map[string]any {
	if data == nil {
		return nil
	}
	for _, root := range []map[string]any{data, searchMsgChildMap(data, "result")} {
		if root == nil {
			continue
		}
		if groups, ok := root["conversationMessagesList"].([]any); ok {
			return searchMsgFlattenGroups(groups)
		}
	}
	keys := []string{"list", "messages", "messageList", "items", "data", "records", "result"}
	for _, key := range keys {
		if arr, ok := data[key].([]any); ok {
			return searchMsgToMaps(arr)
		}
		if inner, ok := data[key].(map[string]any); ok {
			for _, k2 := range []string{"list", "messages", "messageList", "items", "data", "records"} {
				if arr, ok := inner[k2].([]any); ok {
					return searchMsgToMaps(arr)
				}
			}
		}
	}
	return nil
}

func searchMsgChildMap(data map[string]any, key string) map[string]any {
	if value, ok := data[key].(map[string]any); ok {
		return value
	}
	return nil
}

func searchMsgFlattenGroups(groups []any) []map[string]any {
	out := make([]map[string]any, 0)
	for _, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		messages, ok := group["messages"].([]any)
		if !ok {
			continue
		}
		conversationID := strings.TrimSpace(fmt.Sprint(group["openConversationId"]))
		conversationTitle := strings.TrimSpace(fmt.Sprint(group["title"]))
		singleChat, hasSingleChat := group["singleChat"]
		for _, rawMessage := range messages {
			message, ok := rawMessage.(map[string]any)
			if !ok {
				continue
			}
			item := make(map[string]any, len(message)+3)
			for key, value := range message {
				item[key] = value
			}
			if _, exists := item["openConversationId"]; !exists &&
				conversationID != "" && conversationID != "<nil>" {
				item["openConversationId"] = conversationID
			}
			if _, exists := item["conversationTitle"]; !exists &&
				conversationTitle != "" && conversationTitle != "<nil>" {
				item["conversationTitle"] = conversationTitle
			}
			if _, exists := item["singleChat"]; !exists && hasSingleChat {
				item["singleChat"] = singleChat
			}
			out = append(out, item)
		}
	}
	return out
}

func searchMsgToMaps(arr []any) []map[string]any {
	out := make([]map[string]any, 0, len(arr))
	for _, it := range arr {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// searchMsgItemsStrict is the activated projection boundary. Legacy search
// output historically tolerated a missing or malformed list as an empty one;
// the Agent-facing result cannot make that equivalence because it converts a
// service-shape failure into a false "no messages" answer.
func searchMsgItemsStrict(data map[string]any) ([]map[string]any, error) {
	if data == nil {
		return nil, fmt.Errorf("response is empty")
	}
	scopes := []map[string]any{data}
	if result, ok := data["result"].(map[string]any); ok {
		scopes = append(scopes, result)
	}
	for _, scope := range scopes {
		if raw, present := scope["conversationMessagesList"]; present {
			groups, ok := raw.([]any)
			if !ok {
				return nil, fmt.Errorf("conversationMessagesList is not a list")
			}
			return searchMsgFlattenGroupsStrict(groups)
		}
		for _, key := range []string{"list", "messages", "messageList", "items", "data", "records", "result"} {
			raw, present := scope[key]
			if !present {
				continue
			}
			if _, wrapped := raw.(map[string]any); wrapped {
				continue
			}
			values, ok := raw.([]any)
			if !ok {
				return nil, fmt.Errorf("%s is not a message list", key)
			}
			return searchMsgValidateItems(values, key)
		}
	}
	return nil, fmt.Errorf("response has no recognized message list")
}

func searchMsgFlattenGroupsStrict(groups []any) ([]map[string]any, error) {
	out := make([]map[string]any, 0)
	for groupIndex, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok || group == nil {
			return nil, fmt.Errorf("conversationMessagesList[%d] is not an object", groupIndex)
		}
		rawMessages, present := group["messages"]
		if !present {
			return nil, fmt.Errorf("conversationMessagesList[%d] has no messages", groupIndex)
		}
		messages, ok := rawMessages.([]any)
		if !ok {
			return nil, fmt.Errorf("conversationMessagesList[%d].messages is not a list", groupIndex)
		}
		conversationID := searchMsgCursorText(group["openConversationId"])
		conversationTitle := strings.TrimSpace(fmt.Sprint(group["title"]))
		singleChat, hasSingleChat := group["singleChat"]
		for messageIndex, rawMessage := range messages {
			message, ok := rawMessage.(map[string]any)
			if !ok || message == nil {
				return nil, fmt.Errorf("conversationMessagesList[%d].messages[%d] is not an object", groupIndex, messageIndex)
			}
			if strings.TrimSpace(chatmsg.StableMessageID(message)) == "" {
				return nil, fmt.Errorf("conversationMessagesList[%d].messages[%d] has no stable message ID", groupIndex, messageIndex)
			}
			item := make(map[string]any, len(message)+3)
			for key, value := range message {
				item[key] = value
			}
			if _, exists := item["openConversationId"]; !exists && conversationID != "" {
				item["openConversationId"] = conversationID
			}
			if _, exists := item["conversationTitle"]; !exists && conversationTitle != "" && conversationTitle != "<nil>" {
				item["conversationTitle"] = conversationTitle
			}
			if _, exists := item["singleChat"]; !exists && hasSingleChat {
				item["singleChat"] = singleChat
			}
			out = append(out, item)
		}
	}
	return out, nil
}

func searchMsgValidateItems(values []any, label string) ([]map[string]any, error) {
	items := make([]map[string]any, 0, len(values))
	for index, value := range values {
		item, ok := value.(map[string]any)
		if !ok || item == nil {
			return nil, fmt.Errorf("%s[%d] is not an object", label, index)
		}
		if strings.TrimSpace(chatmsg.StableMessageID(item)) == "" {
			return nil, fmt.Errorf("%s[%d] has no stable message ID", label, index)
		}
		items = append(items, item)
	}
	return items, nil
}

// searchMsgProject reshapes one matched message into {sender, time, text,
// messageId}, running text through the shared chatmsg cleaning (card/auto-reply
// JSON → readable, ciphertext → marker) and recursively expanding any forwarded
// chat record under "forwarded".
func searchMsgProject(m map[string]any) map[string]any {
	return searchMsgProjectWithReactions(m, true)
}

func searchMsgProjectWithReactions(m map[string]any, includeReactions bool) map[string]any {
	row := chatmsg.ProjectMessageV1(m, includeReactions)
	// Preserve the established search aliases while also publishing the V1
	// canonical fields used by other message readers.
	row["sender"] = searchMsgSender(m)
	row["time"] = searchMsgTime(m)
	row["text"] = searchMsgCleanText(m)
	row["messageId"] = searchMsgMessageID(m)
	if forwarded := chatmsg.Forwarded(m, func(item map[string]any) map[string]any {
		return searchMsgProjectWithReactions(item, includeReactions)
	}); len(forwarded) > 0 {
		row["forwarded"] = forwarded
	}
	return row
}

// searchMsgCleanText runs searchMsgText's extraction through chatmsg.CleanText.
func searchMsgCleanText(m map[string]any) any {
	if s, ok := searchMsgText(m).(string); ok {
		return chatmsg.CleanText(s)
	}
	return searchMsgText(m)
}

// searchMsgSender reads a message's sender display name/id, tolerating the
// common sender keys the gateway may use (including a nested sender object). The
// literal string "null" (carried by forwarded sub-messages) and the empty string
// are both treated as absent so they never surface as the speaker.
func searchMsgSender(m map[string]any) any {
	norm := func(v any) string {
		if s := searchMsgString(v); s != "" && s != "null" {
			return s
		}
		return ""
	}
	for _, key := range []string{"senderName", "sender_name", "senderNick", "fromName", "senderStaffName"} {
		if v := norm(m[key]); v != "" {
			return v
		}
	}
	for _, key := range []string{"sender", "from", "senderUser"} {
		if nested, ok := m[key].(map[string]any); ok {
			for _, k2 := range []string{"name", "nick", "userName", "staffName", "displayName"} {
				if v := norm(nested[k2]); v != "" {
					return v
				}
			}
		}
		if v := norm(m[key]); v != "" {
			return v
		}
	}
	for _, key := range []string{"senderId", "sender_id", "senderUserId", "senderStaffId", "openDingTalkId"} {
		if v := norm(m[key]); v != "" {
			return v
		}
	}
	return nil
}

// searchMsgTime reads a message's send time, returning the raw value (usually
// epoch millis) under whichever candidate key is present.
func searchMsgTime(m map[string]any) any {
	for _, key := range []string{"createTime", "sendTime", "gmtCreate", "time", "msgTime", "createAt"} {
		if v, ok := m[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

// searchMsgText reads a message's textual content, tolerating flat text keys and
// a nested content/text object.
func searchMsgText(m map[string]any) any {
	for _, key := range []string{"text", "content", "msgContent", "message", "body"} {
		if v := searchMsgString(m[key]); v != "" {
			return v
		}
	}
	for _, key := range []string{"content", "text", "msg"} {
		if nested, ok := m[key].(map[string]any); ok {
			for _, k2 := range []string{"text", "content", "richText", "title"} {
				if v := searchMsgString(nested[k2]); v != "" {
					return v
				}
			}
		}
	}
	return nil
}

// searchMsgMessageID reads a message's identifier, tolerating the common id keys
// the gateway may use.
func searchMsgMessageID(m map[string]any) any {
	for _, key := range []string{"messageId", "message_id", "msgId", "msg_id", "openMessageId", "id"} {
		if v := searchMsgString(m[key]); v != "" {
			return v
		}
	}
	return nil
}

// searchMsgString coerces a scalar JSON value to a trimmed string, returning ""
// for nil / non-scalar / empty values.
func searchMsgString(v any) string {
	switch typed := v.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func init() {
	shortcut.Register(SearchMsg)
}
