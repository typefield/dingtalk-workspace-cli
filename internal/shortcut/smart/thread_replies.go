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
	"math"
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
)

const (
	threadRepliesDefaultPageLimit = 50
	threadRepliesHardPageLimit    = 500
)

// ThreadReplies: fetch every reply under one topic ("话题") message and print a
// clean projected list (speaker / text / time) instead of a raw dump.
//
// Steps:
//  1. accept either --group plus --thread-id/--topic-id, or resolve a root
//     --message-id to its conversation/thread context through list_messages_by_ids;
//  2. call list_topic_replies (chat server), optionally with startTime=--time
//     and pageSize=--limit;
//  3. defensively unwrap the reply list (multiple candidate container keys) and
//     project each reply to {sender, text, createTime} tolerating field aliases;
//  4. print via rt.Output as {replies, count} so it honours --format/--jq/--fields.
//
// The default path only reads and reshapes topic replies;
// --download-resources additionally writes resource files locally.
//
//	dws chat +thread-replies --group <openconversationId> --thread-id <threadId>
var ThreadReplies = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "chat",
	Command:       "+thread-replies",
	Product:       "chat",
	Description:   "按主消息 ID 或 thread/topic ID 分页读取话题回复，支持完整排序与有界自动翻页",
	Intent: "当你已经拿到某个群里一条「话题消息」的主消息 ID 或 threadId/topicId、想快速看这条话题下的回复（谁在什么时间回复了什么），" +
		"而不想拿到一大坨原始消息字段时使用；可直接传 --message-id 自动只读解析会话和 thread，也可传 --group 配合 --thread-id（兼容 --topic-id），" +
		"可选 --time 指定手工续页边界、--limit/--page-size 指定每页条数；--page-all 沿下层毫秒级 nextCursor 自动续页，--page-limit 保持有界；--order/--sort 支持 desc，asc 需与 --page-all 一起使用，" +
		"结果会保留明确的续页凭据、失败或未知分页边界；不能把页数截断或空回复误解为完整结果，再在本地投影出每条回复的发言人、文本和回复时间。" +
		"默认只读且不会发送或修改任何消息；--download-resources 使用工作目录内安全路径、默认不覆盖和原子落盘，按既有安全下载约定无需交互确认。",
	Risk: shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "chat",
			Name:           "shortcut_thread_replies",
			CanonicalPath:  "chat.shortcut_thread_replies",
			CLIPath:        "chat +thread-replies",
			PrimaryCLIPath: "chat +thread-replies",
		},
		Description: "按主消息 ID 或 thread/topic ID 分页读取话题回复，支持完整排序与有界自动翻页",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed thread reader adapter: it accepts a root message ID or threadId/topicId, reads lower topic replies, projects stable message fields, orders complete results, and optionally downloads reply resources safely.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "按主消息 ID 或 thread/topic ID 分页读取话题回复，支持完整排序与有界自动翻页",
			UseWhen: []string{"当你已经拿到某个群里一条「话题消息」的主消息 ID 或 threadId/topicId、想快速看这条话题下的回复（谁在什么时间回复了什么），" +
				"而不想拿到一大坨原始消息字段时使用；可直接传 --message-id 自动只读解析会话和 thread，也可传 --group 配合 --thread-id（兼容 --topic-id），" +
				"可选 --time 指定手工续页边界、--limit/--page-size 指定每页条数；--page-all 沿下层毫秒级 nextCursor 自动续页，--page-limit 保持有界；--order/--sort 支持 desc，asc 需与 --page-all 一起使用，" +
				"结果会保留明确的续页凭据、失败或未知分页边界；不能把页数截断或空回复误解为完整结果，再在本地投影出每条回复的发言人、文本和回复时间。" +
				"默认只读且不会发送或修改任何消息；--download-resources 使用工作目录内安全路径、默认不覆盖和原子落盘，按既有安全下载约定无需交互确认。"},
			AvoidWhen: []string{"要回复 Thread 或发送新回复时不要使用此读取入口；当前没有经过验证的 thread writer Shortcut"},
			Examples: []string{
				"dws chat +thread-replies --message-id <rootOpenMessageId> --page-all --order asc",
				"dws chat +thread-replies --group <openConversationId> --thread-id <threadId>",
			},
		},
	},
	Flags: append([]shortcut.Flag{
		{Name: "group", Type: shortcut.FlagString, Desc: "群会话 ID；--thread-id/--topic-id 必须同时提供 --group；--group 与 --message-id 解析出的 conversationId 必须匹配"},
		{Name: "message-id", Type: shortcut.FlagString, Desc: "话题主消息 openMessageId；自动只读解析 conversationId 和 threadId；--group 与 --message-id 解析出的 conversationId 必须匹配"},
		{Name: "thread-id", Type: shortcut.FlagString, Desc: "话题/线程 ID（可直接使用消息列表返回的 threadId）；--thread-id/--topic-id 必须同时提供 --group"},
		{Name: "topic-id", Type: shortcut.FlagString, Desc: "--thread-id 的兼容别名；--thread-id/--topic-id 必须同时提供 --group"},
		{Name: "time", Type: shortcut.FlagString, Desc: "起始时间，如 \"2025-03-01 00:00:00\"；--time 必须是 RFC3339、YYYY-MM-DD HH:mm:ss 或 YYYY-MM-DD（可选）"},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "每页拉取的回复条数；--limit 必须大于 0"},
		{Name: "page-size", Type: shortcut.FlagInt, Desc: "--limit 的公开兼容别名；必须大于 0"},
		{Name: "page-all", Type: shortcut.FlagBool, Desc: "沿下层毫秒级 nextCursor 自动读取后续页；--page-limit 仅与 --page-all 一起使用且范围 1-500；asc 必须与 --page-all 一起使用"},
		{Name: "page-limit", Type: shortcut.FlagInt, Default: "50", Desc: "--page-limit 仅与 --page-all 一起使用且范围 1-500"},
		{Name: "order", Type: shortcut.FlagString, Enum: []string{"asc", "desc"}, Desc: "回复输出顺序 asc/desc（可选，默认 desc；asc 必须与 --page-all 一起使用）"},
		{Name: "sort", Type: shortcut.FlagString, Enum: []string{"asc", "desc"}, Desc: "--order 的 lark-cli 对齐别名（可选；asc 必须与 --page-all 一起使用）"},
		{Name: "no-reactions", Type: shortcut.FlagBool, Desc: "不输出回复 reaction（默认输出）"},
	}, chatshortcut.MessageResourceDownloadFlags()...),
	Constraints: append([]shortcut.Constraint{
		{
			Kind:        shortcut.ConstraintExactlyOne,
			Flags:       []string{"message-id", "thread-id", "topic-id"},
			Description: "--message-id、--thread-id 与兼容参数 --topic-id 必须且只能指定一个",
		},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"limit", "page-size"}},
		{Kind: shortcut.ConstraintMutuallyExclusive, Flags: []string{"order", "sort"}},
		{Kind: shortcut.ConstraintCustom, Flags: []string{"group", "thread-id", "topic-id"}, Description: "--thread-id/--topic-id 必须同时提供 --group"},
		{Kind: shortcut.ConstraintCustom, Flags: []string{"group", "message-id"}, Description: "--group 与 --message-id 解析出的 conversationId 必须匹配"},
		{Kind: shortcut.ConstraintCustom, Flags: []string{"time"}, Description: "--time 必须是 RFC3339、YYYY-MM-DD HH:mm:ss 或 YYYY-MM-DD"},
		{Kind: shortcut.ConstraintCustom, Flags: []string{"limit", "page-size"}, Description: "显式页大小必须大于 0"},
		{Kind: shortcut.ConstraintCustom, Flags: []string{"page-all", "page-limit"}, Description: "--page-limit 仅与 --page-all 一起使用且范围 1-500"},
		{Kind: shortcut.ConstraintCustom, Flags: []string{"order", "sort", "page-all"}, Description: "asc 必须与 --page-all 一起使用"},
	}, chatshortcut.MessageResourceDownloadConstraints()...),
	Tips: []string{
		`dws chat +thread-replies --message-id <rootOpenMessageId> --page-all --order asc`,
		`dws chat +thread-replies --group <openconversationId> --thread-id <threadId>`,
		`dws chat +thread-replies --group <openconversationId> --thread-id <threadId> --time "2025-03-01 00:00:00" --limit 20`,
		`dws chat +thread-replies --group <openconversationId> --thread-id <threadId> --page-size 50 --page-all --page-limit 20`,
	},
	Validate: validateThreadReplies,
	Execute:  executeThreadReplies,
}

func validateThreadReplies(rt *shortcut.RuntimeContext) error {
	if err := chatshortcut.ValidateMessageResourceDownload(rt); err != nil {
		return err
	}
	for _, name := range []string{"limit", "page-size"} {
		if rt.Changed(name) && rt.Int(name) <= 0 {
			return localChatOptionError("invalid_page_size", "+thread-replies 的 --"+name+" 必须大于 0", "--"+name)
		}
	}
	if value := strings.TrimSpace(rt.Str("time")); value != "" && !validChatTime(value) {
		return localChatOptionError("invalid_time_boundary", "+thread-replies 的 --time 格式无效", "--time")
	}
	if strings.TrimSpace(rt.Str("message-id")) == "" && strings.TrimSpace(rt.Str("group")) == "" {
		return apperrors.NewValidation("--thread-id/--topic-id 必须同时提供 --group")
	}
	if threadRepliesOrder(rt) == "asc" && !rt.Bool("page-all") {
		return apperrors.NewValidation("asc 必须与 --page-all 一起使用")
	}
	if !rt.Bool("page-all") && rt.Changed("page-limit") {
		return apperrors.NewValidation("--page-limit 仅与 --page-all 一起使用")
	}
	if rt.Bool("page-all") {
		if pageLimit := rt.Int("page-limit"); pageLimit < 1 || pageLimit > threadRepliesHardPageLimit {
			return apperrors.NewValidation("--page-limit 必须在 1-500 之间")
		}
	}
	return nil
}

func executeThreadReplies(rt *shortcut.RuntimeContext) error {
	target, err := resolveThreadRepliesTarget(rt)
	if err != nil {
		return err
	}
	params := map[string]any{
		"openconversationId": target.conversationID,
		"topicId":            target.threadID,
		"forward":            false,
	}
	if value := strings.TrimSpace(rt.Str("time")); value != "" {
		params["startTime"] = value
	}
	if pageSize := rt.IntFirst("limit", "page-size"); pageSize > 0 {
		params["pageSize"] = pageSize
	}

	var payload map[string]any
	var items []map[string]any
	var pageLedger *output.PageLedger
	err = nil
	if rt.Bool("page-all") {
		payload, items, pageLedger, err = collectAllThreadReplies(rt, params)
	} else {
		payload, items, pageLedger, err = collectOneThreadRepliesPage(rt, params)
	}
	order := threadRepliesOrder(rt)
	applyThreadRepliesResultContract(payload, items, target, order)
	if err == nil && payload != nil && rt.Bool("download-resources") {
		resourceLedger, resourceFailures := chatshortcut.DownloadMessageResourcesWithFailureInfo(rt, items, target.conversationID)
		chatshortcut.AttachMessageResourceDownloads(payload, resourceLedger)
		for _, failureInfo := range resourceFailures {
			if recordErr := pageLedger.RecordPostPageFailure(failureInfo); recordErr != nil {
				return apperrors.NewInternal("记录话题回复资源下载失败状态失败", apperrors.WithCause(recordErr))
			}
		}
		if len(resourceFailures) > 0 {
			pageLedger.SetStopReason("resource_download_failure")
		}
	}
	result, resultErr := threadRepliesUnifiedResult(pageLedger, payload, rt.DryRun())
	if resultErr != nil {
		return apperrors.NewInternal("生成话题回复统一分页结果失败", apperrors.WithCause(resultErr))
	}
	if err != nil {
		// An activated command emits its stored unified failure/partial result;
		// returning the legacy error afterwards would create a second outcome.
		if output.UsesUnifiedResult(rt.Command()) {
			return rt.OutputResult(payload, result)
		}
		if payload != nil {
			if outputErr := rt.OutputResult(payload, result); outputErr != nil {
				return outputErr
			}
		}
		return err
	}
	return rt.OutputResult(payload, result)
}

type threadRepliesTarget struct {
	conversationID        string
	threadID              string
	resolvedFromMessageID string
}

func resolveThreadRepliesTarget(rt *shortcut.RuntimeContext) (threadRepliesTarget, error) {
	messageID := strings.TrimSpace(rt.Str("message-id"))
	if messageID == "" {
		return threadRepliesTarget{
			conversationID: strings.TrimSpace(rt.Str("group")),
			threadID:       strings.TrimSpace(rt.StrFirst("thread-id", "topic-id")),
		}, nil
	}

	data, err := rt.CallMCPData("im", "list_messages_by_ids", map[string]any{
		"openMsgIds": []string{messageID},
	})
	if err != nil {
		return threadRepliesTarget{}, err
	}
	var matched map[string]any
	for _, item := range threadReplyItems(data) {
		if threadRepliesString(chatmsg.MessageID(item)) == messageID {
			matched = item
			break
		}
	}
	if matched == nil {
		return threadRepliesTarget{}, apperrors.NewAPI(
			"未找到指定的话题主消息，无法解析 thread",
			apperrors.WithOperation("im/list_messages_by_ids"),
			apperrors.WithSubtype(apperrors.SubtypeThreadRootMessageNotFound),
			apperrors.WithOrigin("mcp_gateway"),
			apperrors.WithHint("请确认 --message-id 是可读取的话题主消息 openMessageId"),
		)
	}
	conversationID := threadRepliesString(chatmsg.ConversationID(matched))
	threadID := threadRepliesString(chatmsg.ThreadID(matched))
	if conversationID == "" || threadID == "" {
		return threadRepliesTarget{}, apperrors.NewAPI(
			"消息详情未返回 conversationId/threadId，无法读取话题回复",
			apperrors.WithOperation("im/list_messages_by_ids"),
			apperrors.WithSubtype(apperrors.SubtypeThreadContextMissing),
			apperrors.WithOrigin("mcp_gateway"),
			apperrors.WithHint("请确认 --message-id 指向话题主消息，或改用 --group 与 --thread-id"),
		)
	}
	if explicitGroup := strings.TrimSpace(rt.Str("group")); explicitGroup != "" && explicitGroup != conversationID {
		return threadRepliesTarget{}, apperrors.NewValidation("--group 与 --message-id 解析出的 conversationId 不匹配")
	}
	return threadRepliesTarget{
		conversationID:        conversationID,
		threadID:              threadID,
		resolvedFromMessageID: messageID,
	}, nil
}

func threadRepliesOrder(rt *shortcut.RuntimeContext) string {
	if order := strings.ToLower(strings.TrimSpace(rt.StrFirst("order", "sort"))); order != "" {
		return order
	}
	return "desc"
}

func applyThreadRepliesResultContract(payload map[string]any, items []map[string]any, target threadRepliesTarget, order string) {
	if payload == nil {
		return
	}
	if order == "asc" {
		reverseThreadReplyMaps(items)
		if replies, ok := payload["replies"].([]map[string]any); ok {
			reverseThreadReplyMaps(replies)
		}
	}
	payload["order"] = order
	if payload["complete"] == true {
		payload["orderScope"] = "complete_result"
	} else {
		payload["orderScope"] = "fetched_pages"
	}
	payload["conversationId"] = target.conversationID
	payload["threadId"] = target.threadID
	if target.resolvedFromMessageID != "" {
		payload["resolvedFromMessageId"] = target.resolvedFromMessageID
	}
}

func reverseThreadReplyMaps(items []map[string]any) {
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
}

func threadRepliesString(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func collectOneThreadRepliesPage(rt *shortcut.RuntimeContext, params map[string]any) (map[string]any, []map[string]any, *output.PageLedger, error) {
	pageLedger, err := output.NewPageLedger(1)
	if err != nil {
		return nil, nil, nil, err
	}
	data, err := rt.CallMCPData("chat", "list_topic_replies", params)
	if err != nil {
		if recordErr := pageLedger.RecordFailure(threadRepliesInitialLedgerCursor(params), threadRepliesReadFailureInfo(err)); recordErr != nil {
			return nil, nil, nil, recordErr
		}
		return nil, nil, pageLedger, err
	}
	items := threadReplyItems(data)
	payload := newThreadRepliesPayload(items, !rt.Bool("no-reactions"))
	applyOneThreadRepliesPagination(payload, data)
	if payload["complete"] == true {
		payload["stopReason"] = "source_complete"
	} else if payload["failedCount"] != 0 {
		payload["stopReason"] = "pagination_error"
	} else {
		payload["stopReason"] = "single_page"
	}
	if _, _, observeErr := observeThreadRepliesUnifiedPage(
		pageLedger,
		threadRepliesInitialLedgerCursor(params),
		data,
		!rt.Bool("no-reactions"),
	); observeErr != nil {
		return nil, nil, nil, observeErr
	}
	pageLedger.SetStopReason(fmt.Sprint(payload["stopReason"]))
	return payload, items, pageLedger, nil
}

func applyOneThreadRepliesPagination(payload, data map[string]any) {
	payload["pagesFetched"] = 1
	page := chatmsg.Pagination(data)
	hasMore, hasMoreKnown := page["hasMore"].(bool)
	if !hasMoreKnown {
		payload["paginationKnown"] = false
		payload["failedCount"] = 1
		payload["failures"] = []map[string]any{{
			"stage": "pagination",
			"error": "下层未返回可靠的 hasMore，无法证明话题回复完整",
		}}
		return
	}
	payload["paginationKnown"] = true
	payload["hasMore"] = hasMore
	payload["complete"] = !hasMore
	if !hasMore {
		return
	}
	_, boundary, err := threadRepliesNextCursorBoundary(page["nextCursor"])
	if err != nil {
		payload["paginationKnown"] = false
		payload["failedCount"] = 1
		payload["failures"] = []map[string]any{{
			"stage": "pagination",
			"error": "hasMore=true 但 nextCursor 无效，无法安全续页: " + err.Error(),
		}}
		return
	}
	payload["nextPage"] = map[string]any{
		"direction":  "older",
		"time":       boundary,
		"nextCursor": page["nextCursor"],
	}
}

func collectAllThreadReplies(rt *shortcut.RuntimeContext, params map[string]any) (map[string]any, []map[string]any, *output.PageLedger, error) {
	pageLimit := defaultChatPageLimit(rt.Int("page-limit"), threadRepliesDefaultPageLimit)
	pageLedger, err := output.NewPageLedger(pageLimit)
	if err != nil {
		return nil, nil, nil, err
	}
	seenIDs := map[string]bool{}
	seenCursors := map[string]bool{}
	allItems := make([]map[string]any, 0)
	failures := make([]map[string]any, 0)
	pagesFetched := 0
	paginationKnown := true
	complete := false
	hasMore := false
	stopReason := "source_complete"
	truncatedByPageLimit := false
	var nextPage map[string]any
	ledgerCursor := threadRepliesInitialLedgerCursor(params)
	shadowInterrupted := false

	for pagesFetched < pageLimit {
		data, err := rt.CallMCPData("chat", "list_topic_replies", params)
		if err != nil {
			failures = append(failures, map[string]any{
				"page": pagesFetched + 1, "stage": "read", "error": err.Error(),
			})
			if !shadowInterrupted {
				if recordErr := pageLedger.RecordFailure(ledgerCursor, threadRepliesReadFailureInfo(err)); recordErr != nil {
					return nil, nil, nil, recordErr
				}
				shadowInterrupted = true
			}
			stopReason = "read_failure"
			break
		}
		pagesFetched++
		rawItems := threadReplyItems(data)
		for _, item := range rawItems {
			stableID := chatmsg.StableMessageID(item)
			if stableID != "" && seenIDs[stableID] {
				continue
			}
			if stableID != "" {
				seenIDs[stableID] = true
			}
			allItems = append(allItems, item)
		}
		if !shadowInterrupted {
			nextToken, terminal, observeErr := observeThreadRepliesUnifiedPage(
				pageLedger, ledgerCursor, data, !rt.Bool("no-reactions"),
			)
			if observeErr != nil {
				return nil, nil, nil, observeErr
			}
			if nextToken != "" {
				ledgerCursor = nextToken
			}
			shadowInterrupted = terminal
		}

		page := chatmsg.Pagination(data)
		pageHasMore, hasMoreKnown := page["hasMore"].(bool)
		if !hasMoreKnown {
			if !shadowInterrupted {
				if recordErr := pageLedger.RecordPostPageFailure(threadRepliesPaginationFailureInfo("下层未返回可靠的 hasMore，无法证明话题回复完整")); recordErr != nil {
					return nil, nil, nil, recordErr
				}
				shadowInterrupted = true
			}
			paginationKnown = false
			failures = append(failures, map[string]any{
				"page": pagesFetched, "stage": "pagination",
				"error": "下层未返回可靠的 hasMore，无法证明话题回复完整",
			})
			stopReason = "pagination_error"
			break
		}
		hasMore = pageHasMore
		if !pageHasMore {
			complete = true
			hasMore = false
			nextPage = nil
			stopReason = "source_complete"
			break
		}
		if len(rawItems) == 0 {
			if !shadowInterrupted {
				if recordErr := pageLedger.RecordPostPageFailure(threadRepliesPaginationFailureInfo("hasMore=true 但当前页没有回复，无法证明分页可以安全推进")); recordErr != nil {
					return nil, nil, nil, recordErr
				}
				shadowInterrupted = true
			}
			failures = append(failures, map[string]any{
				"page": pagesFetched, "stage": "pagination",
				"error": "下层返回 hasMore=true 但当前页没有回复，无法证明分页可以安全推进",
			})
			stopReason = "pagination_error"
			break
		}

		cursorKey, boundary, cursorErr := threadRepliesNextCursorBoundary(page["nextCursor"])
		if cursorErr != nil {
			failures = append(failures, map[string]any{
				"page": pagesFetched, "stage": "pagination",
				"error": "hasMore=true 但 nextCursor 无效，无法安全续页: " + cursorErr.Error(),
			})
			stopReason = "pagination_error"
			break
		}
		if seenCursors[cursorKey] {
			failures = append(failures, map[string]any{
				"page": pagesFetched, "stage": "pagination",
				"error": "hasMore=true 但 nextCursor 停滞",
			})
			stopReason = "pagination_error"
			break
		}
		seenCursors[cursorKey] = true
		nextPage = map[string]any{
			"direction":  "older",
			"time":       boundary,
			"nextCursor": page["nextCursor"],
		}
		params["startTime"] = boundary
		if shadowInterrupted && output.UsesUnifiedResult(rt.Command()) {
			// The legacy path may still have a continuation. Once active, an
			// unprojectable page or contradictory boundary is terminal.
			break
		}
	}

	if !complete && hasMore && len(failures) == 0 && pagesFetched >= pageLimit {
		truncatedByPageLimit = true
		stopReason = "page_limit"
	}
	payload := newThreadRepliesPayload(allItems, !rt.Bool("no-reactions"))
	payload["pagesFetched"] = pagesFetched
	payload["paginationKnown"] = paginationKnown
	payload["complete"] = complete && len(failures) == 0
	payload["hasMore"] = hasMore
	payload["stopReason"] = stopReason
	payload["truncatedByPageLimit"] = truncatedByPageLimit
	payload["failedCount"] = len(failures)
	payload["failures"] = failures
	payload["partial"] = len(failures) > 0 && len(allItems) > 0
	if hasMore && nextPage != nil {
		payload["nextPage"] = nextPage
	}
	if len(failures) > 0 {
		failureStage := "pagination"
		if stopReason == "read_failure" {
			failureStage = "read"
		}
		pageLedger.SetStopReason(stopReason)
		return payload, allItems, pageLedger, apperrors.NewAPI(
			fmt.Sprintf("话题回复全量读取未完成：%d 页成功，%d 个失败项", pagesFetched, len(failures)),
			apperrors.WithOperation("chat/list_topic_replies"),
			apperrors.WithSubtype(apperrors.SubtypeThreadRepliesIncomplete),
			apperrors.WithOrigin("mcp_gateway"),
			apperrors.WithFailureStage(failureStage),
			apperrors.WithExecutionStarted(true),
			apperrors.WithRetryable(true),
			apperrors.WithHint("请根据 failures 和 nextPage 重试"),
		)
	}
	pageLedger.SetStopReason(stopReason)
	return payload, allItems, pageLedger, nil
}

// threadRepliesUnifiedResult is deliberately built beside the legacy payload,
// not from it. The payload retains historical complete/hasMore/nextPage fields
// during dual validation; the PageLedger is the sole input to the unified
// pagination contract.
func threadRepliesUnifiedResult(pageLedger *output.PageLedger, payload map[string]any, dryRun bool) (output.CommandResult, error) {
	if pageLedger == nil {
		return nil, fmt.Errorf("missing pagination ledger")
	}
	data := map[string]any{}
	if payload != nil {
		for _, key := range []string{"replies", "count", "conversationId", "threadId", "resolvedFromMessageId", "order", "orderScope", "resourceDownloads"} {
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

// observeThreadRepliesUnifiedPage converts the service-specific reply page
// into the framework's narrow pagination evidence. It never changes legacy
// control flow: in dual validation it only records a shadow result. The bool
// return means the active unified command must stop because the current page
// cannot be projected or its continuation boundary is contradictory.
func observeThreadRepliesUnifiedPage(
	pageLedger *output.PageLedger,
	cursor string,
	data map[string]any,
	includeReactions bool,
) (nextToken string, terminal bool, err error) {
	items, projectionErr := threadReplyItemsStrict(data)
	if projectionErr != nil {
		if recordErr := pageLedger.RecordFailure(cursor, threadRepliesProjectionFailureInfo(projectionErr)); recordErr != nil {
			return "", true, recordErr
		}
		return "", true, nil
	}
	evidence := output.PageEvidence{
		Cursor: cursor,
		Items:  len(items),
		Data:   map[string]any{"replies": projectThreadReplyItems(items, includeReactions)},
	}
	page := chatmsg.Pagination(data)
	hasMore, known := page["hasMore"].(bool)
	if !known {
		if token, _, tokenErr := threadRepliesNextCursorBoundary(page["nextCursor"]); tokenErr == nil {
			evidence.NextToken = token
			if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
				return "", true, threadRepliesRecordUnifiedBoundary(pageLedger, evidence, observeErr.Error())
			}
			return token, false, nil
		}
		if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
			return "", false, observeErr
		}
		return "", false, nil
	}
	if !hasMore {
		if token, _, tokenErr := threadRepliesNextCursorBoundary(page["nextCursor"]); tokenErr == nil && token != "" {
			if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
				return "", true, threadRepliesRecordUnifiedBoundary(pageLedger, evidence, observeErr.Error())
			}
			if recordErr := pageLedger.RecordBoundaryFailure(threadRepliesPaginationFailureInfo("hasMore=false，但同时携带可用 nextCursor")); recordErr != nil {
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

	token, _, tokenErr := threadRepliesNextCursorBoundary(page["nextCursor"])
	if tokenErr != nil {
		if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
			return "", true, observeErr
		}
		if recordErr := pageLedger.RecordBoundaryFailure(threadRepliesPaginationFailureInfo("hasMore=true 但 nextCursor 无效：" + tokenErr.Error())); recordErr != nil {
			return "", true, recordErr
		}
		return "", true, nil
	}
	more := true
	evidence.HasMore = &more
	evidence.NextToken = token
	if observeErr := pageLedger.ObservePage(evidence); observeErr != nil {
		return "", true, threadRepliesRecordUnifiedBoundary(pageLedger, evidence, observeErr.Error())
	}
	return token, false, nil
}

// threadRepliesRecordUnifiedBoundary re-observes a decoded page without a
// continuation assertion before recording the unsafe boundary. This preserves
// the page in partial_failure rather than leaking an internal framework error
// when a cursor is stale, repeated, or otherwise fails ledger validation.
func threadRepliesRecordUnifiedBoundary(pageLedger *output.PageLedger, evidence output.PageEvidence, message string) error {
	evidence.HasMore = nil
	evidence.NextToken = ""
	if err := pageLedger.ObservePage(evidence); err != nil {
		return err
	}
	return pageLedger.RecordBoundaryFailure(threadRepliesPaginationFailureInfo(message))
}

func threadRepliesInitialLedgerCursor(params map[string]any) string {
	if params == nil {
		return "initial"
	}
	if start := strings.TrimSpace(fmt.Sprint(params["startTime"])); start != "" && start != "<nil>" {
		return "initial:" + start
	}
	return "initial"
}

func threadRepliesReadFailureInfo(err error) *output.ErrorInfo {
	message := "话题回复分页读取失败"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	info := &output.ErrorInfo{
		Type:             "api",
		Message:          message,
		Hint:             "从失败页边界继续；不要重放已经成功的页面",
		Operation:        "chat/list_topic_replies",
		Origin:           "mcp_gateway",
		Stage:            "pagination_read",
		ExecutionStarted: &started,
		Retryable:        true, // idempotent read
	}
	return shortcut.PreserveTypedErrorInfo(info, err)
}

func threadRepliesPaginationFailureInfo(message string) *output.ErrorInfo {
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypePaginationInconsistent),
		Message:          strings.TrimSpace(message),
		Hint:             "保留已读取页面；不要把当前结果解释为 endpoint 已耗尽",
		Operation:        "chat/list_topic_replies",
		Origin:           "mcp_gateway",
		Stage:            "pagination_projection",
		ExecutionStarted: &started,
	}
}

func threadRepliesProjectionFailureInfo(err error) *output.ErrorInfo {
	message := "话题回复响应无法可靠投影"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypeProjectionUnknown),
		Message:          message,
		Hint:             "保留原始响应证据并检查服务端返回结构；不要把空回复解释为确认无回复",
		Operation:        "chat/list_topic_replies",
		Origin:           "mcp_gateway",
		Stage:            "projection",
		ExecutionStarted: &started,
	}
}

// threadRepliesNextCursorBoundary converts the authoritative millisecond
// cursor returned by list_topic_replies into the exact RFC3339 boundary that
// the same lower tool accepts as startTime. Message createTime is deliberately
// not used here: it is only second-precision and can skip replies when a page
// boundary splits several messages created within the same second.
func threadRepliesNextCursorBoundary(value any) (string, string, error) {
	var millis int64
	switch typed := value.(type) {
	case int:
		millis = int64(typed)
	case int32:
		millis = int64(typed)
	case int64:
		millis = typed
	case float32:
		asFloat := float64(typed)
		if math.IsNaN(asFloat) || math.IsInf(asFloat, 0) || asFloat <= 0 || math.Trunc(asFloat) != asFloat || asFloat >= float64(math.MaxInt64) {
			return "", "", fmt.Errorf("必须是正整数毫秒时间戳")
		}
		millis = int64(asFloat)
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed <= 0 || math.Trunc(typed) != typed || typed >= float64(math.MaxInt64) {
			return "", "", fmt.Errorf("必须是正整数毫秒时间戳")
		}
		millis = int64(typed)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return "", "", fmt.Errorf("必须是正整数毫秒时间戳")
		}
		millis = parsed
	default:
		return "", "", fmt.Errorf("缺少毫秒级分页游标")
	}
	if millis <= 0 {
		return "", "", fmt.Errorf("必须是正整数毫秒时间戳")
	}
	cursorKey := strconv.FormatInt(millis, 10)
	boundary := time.UnixMilli(millis).UTC().Format(time.RFC3339Nano)
	return cursorKey, boundary, nil
}

func newThreadRepliesPayload(items []map[string]any, includeReactions bool) map[string]any {
	return map[string]any{
		"contractVersion": chatmsg.MessageListContractVersion,
		"replies":         projectThreadReplyItems(items, includeReactions),
		"count":           len(items),
		"pagesFetched":    0,
		"paginationKnown": false,
		"complete":        false,
		"hasMore":         false,
		"failedCount":     0,
		"failures":        []map[string]any{},
		"partial":         false,
	}
}

func projectThreadReplyItems(items []map[string]any, includeReactions bool) []map[string]any {
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		results = append(results, projectChatMessageWithReactions(item, includeReactions))
	}
	return results
}

// threadReplyItems defensively unwraps the reply list from the response,
// tolerating the common container keys and one level of nesting under a
// "result"/"data" wrapper.
func threadReplyItems(data map[string]any) []map[string]any {
	if data == nil {
		return nil
	}
	// Try the list keys directly on the response, then inside a wrapper.
	scopes := []map[string]any{data}
	for _, wrap := range []string{"result", "data"} {
		if inner, ok := data[wrap].(map[string]any); ok {
			scopes = append(scopes, inner)
		}
	}
	for _, scope := range scopes {
		for _, key := range []string{"replies", "list", "items", "messages", "records", "data", "result"} {
			if raw, ok := scope[key].([]any); ok {
				out := make([]map[string]any, 0, len(raw))
				for _, e := range raw {
					if m, ok := e.(map[string]any); ok {
						out = append(out, m)
					}
				}
				if len(out) > 0 {
					return out
				}
			}
		}
	}
	return nil
}

// threadReplyItemsStrict is the unified projection boundary. The legacy
// projector intentionally tolerates an absent/invalid container as an empty
// reply list for compatibility; an activated Agent-facing command must not
// make that equivalence because it turns a response-shape failure into a false
// empty result.
func threadReplyItemsStrict(data map[string]any) ([]map[string]any, error) {
	if data == nil {
		return nil, fmt.Errorf("response is empty")
	}
	scopes := []map[string]any{data}
	for _, wrap := range []string{"result", "data"} {
		if inner, ok := data[wrap].(map[string]any); ok {
			scopes = append(scopes, inner)
		}
	}
	for _, scope := range scopes {
		for _, key := range []string{"replies", "list", "items", "messages", "records", "data", "result"} {
			raw, present := scope[key]
			if !present {
				continue
			}
			if _, wrapped := raw.(map[string]any); wrapped {
				continue
			}
			values, ok := raw.([]any)
			if !ok {
				return nil, fmt.Errorf("%s is not a reply list", key)
			}
			items := make([]map[string]any, 0, len(values))
			for index, value := range values {
				item, ok := value.(map[string]any)
				if !ok || item == nil {
					return nil, fmt.Errorf("%s[%d] is not an object", key, index)
				}
				if strings.TrimSpace(chatmsg.StableMessageID(item)) == "" {
					return nil, fmt.Errorf("%s[%d] has no stable message ID", key, index)
				}
				items = append(items, item)
			}
			return items, nil
		}
	}
	return nil, fmt.Errorf("response has no recognized reply list")
}

func init() {
	shortcut.Register(ThreadReplies)
}
