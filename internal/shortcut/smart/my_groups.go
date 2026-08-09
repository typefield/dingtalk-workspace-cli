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

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/chatmsg"
)

const (
	myGroupsDefaultPageLimit = 50
	myGroupsHardPageLimit    = 500
)

// MyGroups: list the groups I've joined and project just the key fields
// (会话id / 名称 / 群主 / 人数 / 类型) into a clean, composed payload — instead of
// paging through `chat group list-all` and squinting at the raw MCP response.
//
// Steps:
//  1. list my groups via list_my_groups_pagination (im server); param `limit`
//     is copied verbatim from chat.go's `chat group list-all` call site.
//  2. defensively project each group's key fields (field names probed across
//     several candidate keys, since the gateway shape isn't guaranteed);
//  3. optionally keep only groups whose type matches --type (Go-side filter —
//     the underlying tool has no server-side type parameter).
//
// Read-only: it never modifies any group or membership.
//
//	dws chat +my-groups
//	dws chat +my-groups --type group
var MyGroups = shortcut.Shortcut{
	// The existing payload remains the external result while the exact same
	// read is projected into the PageLedger candidate. A real single-page
	// observation exists, but empty/continuation/partial paths still need more
	// tenant evidence before this public shortcut may become unified_active.
	OutputRollout: output.RolloutDualValidate,
	Service:       "chat",
	Command:       "+my-groups",
	Product:       "chat",
	Description:   "列出我加入的群，可按类型过滤并投影关键字段",
	Intent: "当你想快速看一眼自己都加入了哪些群、以及每个群的会话ID、名称、群主和人数，而不想翻分页或盯着原始返回时使用；" +
		"内部分页拉取你加入的群列表，把每个群防御式地投影成 会话id / 名称 / 群主 / 人数 / 类型 等关键字段，输出成干净的结果。" +
		"可选 --type 在本地按群类型过滤（底层接口本身不带类型参数，故为客户端过滤）。这是只读操作，不会改动任何群或成员关系。",
	Risk: shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "chat",
			Name:           "shortcut_my_groups",
			CanonicalPath:  "chat.shortcut_my_groups",
			CLIPath:        "chat +my-groups",
			PrimaryCLIPath: "chat +my-groups",
		},
		Description: "列出我加入的群，可按类型过滤并投影关键字段",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "列出我加入的群，可按类型过滤并投影关键字段",
			UseWhen:      []string{"当你想快速看一眼自己都加入了哪些群、以及每个群的会话ID、名称、群主和人数，而不想翻分页或盯着原始返回时使用；内部分页拉取你加入的群列表，把每个群防御式地投影成 会话id / 名称 / 群主 / 人数 / 类型 等关键字段，输出成干净的结果。可选 --type 在本地按群类型过滤（底层接口本身不带类型参数，故为客户端过滤）。这是只读操作，不会改动任何群或成员关系。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples: []string{
				"dws chat +my-groups",
				"dws chat +my-groups --type group",
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "type", Type: shortcut.FlagString, Desc: "按群类型过滤（可选，如返回中的 groupType/conversationType，大小写不敏感）", Required: false},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "每页返回数量（默认 200）；--limit 必须在 1-200 之间", Default: "200"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标，翻页传上次的 nextCursor"},
		{Name: "page-all", Type: shortcut.FlagBool, Desc: "沿 nextCursor 自动读取全部已加入群；--page-limit 仅与 --page-all 一起使用且范围 1-500"},
		{Name: "page-limit", Type: shortcut.FlagInt, Default: "50", Desc: "--page-limit 仅与 --page-all 一起使用且范围 1-500"},
	},
	Constraints: []shortcut.Constraint{
		{Kind: shortcut.ConstraintCustom, Flags: []string{"limit"}, Description: "--limit 必须在 1-200 之间"},
		{Kind: shortcut.ConstraintCustom, Flags: []string{"page-all", "page-limit"}, Description: "--page-limit 仅与 --page-all 一起使用且范围 1-500"},
	},
	Tips: []string{
		`dws chat +my-groups`,
		`dws chat +my-groups --type group`,
		`dws chat +my-groups --page-all --page-limit 50`,
	},
	Validate: validateMyGroups,
	Execute:  executeMyGroups,
}

func validateMyGroups(rt *shortcut.RuntimeContext) error {
	if limit := rt.Int("limit"); limit < 1 || limit > 200 {
		return apperrors.NewValidation("--limit 必须在 1-200 之间")
	}
	if !rt.Bool("page-all") && rt.Changed("page-limit") {
		return apperrors.NewValidation("--page-limit 仅与 --page-all 一起使用")
	}
	if rt.Bool("page-all") {
		if limit := rt.Int("page-limit"); limit < 1 || limit > myGroupsHardPageLimit {
			return apperrors.NewValidation("--page-limit 必须在 1-500 之间")
		}
	}
	return nil
}

func executeMyGroups(rt *shortcut.RuntimeContext) error {
	// Step 1 — list my groups. `limit` mirrors chat.go's list_my_groups_pagination
	// call site (chat group list-all); pass a generous page size.
	params := map[string]any{"limit": rt.Int("limit")}
	if cursor := strings.TrimSpace(rt.Str("cursor")); cursor != "" && cursor != "0" {
		params["cursor"] = cursor
	}
	pageLimit := 1
	if rt.Bool("page-all") {
		pageLimit = defaultChatPageLimit(rt.Int("page-limit"), myGroupsDefaultPageLimit)
	}
	pageLedger, err := output.NewPageLedger(pageLimit)
	if err != nil {
		return apperrors.NewInternal("初始化我的群分页账本失败", apperrors.WithCause(err))
	}
	if rt.Bool("page-all") {
		payload, legacyErr := readAllMyGroups(rt, params, pageLedger)
		result, resultErr := myGroupsUnifiedResult(pageLedger, payload, rt.DryRun())
		if resultErr != nil {
			return apperrors.NewInternal("生成我的群统一分页候选结果失败", apperrors.WithCause(resultErr))
		}
		if payload == nil {
			// Legacy's first-page error has no stdout payload. Dual validation must
			// still validate the candidate but must not invent legacy output.
			if output.UsesUnifiedResult(rt.Command()) {
				return rt.OutputResult(nil, result)
			}
			if err := output.ValidateResult(result); err != nil {
				return apperrors.NewInternal("我的群影子结果违反统一契约", apperrors.WithCause(err))
			}
			return legacyErr
		}
		if output.UsesUnifiedResult(rt.Command()) {
			// An activated command emits the candidate exactly once. In
			// particular, do not return the legacy aggregate error after storing
			// a partial result: Cobra would otherwise emit a second failure.
			return rt.OutputResult(payload, result)
		}
		if outputErr := rt.OutputResult(payload, result); outputErr != nil {
			return outputErr
		}
		return legacyErr
	}
	data, err := rt.CallMCPData("im", "list_my_groups_pagination", params)
	if err != nil {
		if recordErr := pageLedger.RecordFailure(myGroupsCursorString(params["cursor"]), myGroupsReadFailureInfo(err)); recordErr != nil {
			return apperrors.NewInternal("记录我的群读取失败状态失败", apperrors.WithCause(recordErr))
		}
		result, resultErr := myGroupsUnifiedResult(pageLedger, nil, rt.DryRun())
		if resultErr != nil {
			return apperrors.NewInternal("生成我的群读取失败候选结果失败", apperrors.WithCause(resultErr))
		}
		if output.UsesUnifiedResult(rt.Command()) {
			return rt.OutputResult(nil, result)
		}
		if validationErr := output.ValidateResult(result); validationErr != nil {
			return apperrors.NewInternal("我的群影子结果违反统一契约", apperrors.WithCause(validationErr))
		}
		return err
	}
	groups, err := myGroupsExtract(data)
	if err != nil {
		if recordErr := pageLedger.RecordFailure(myGroupsCursorString(params["cursor"]), myGroupsProjectionFailureInfo(err)); recordErr != nil {
			return apperrors.NewInternal("记录我的群投影失败状态失败", apperrors.WithCause(recordErr))
		}
		result, resultErr := myGroupsUnifiedResult(pageLedger, nil, rt.DryRun())
		if resultErr != nil {
			return apperrors.NewInternal("生成我的群投影失败候选结果失败", apperrors.WithCause(resultErr))
		}
		if output.UsesUnifiedResult(rt.Command()) {
			return rt.OutputResult(nil, result)
		}
		if validationErr := output.ValidateResult(result); validationErr != nil {
			return apperrors.NewInternal("我的群影子结果违反统一契约", apperrors.WithCause(validationErr))
		}
		return err
	}
	if err := observeMyGroupsPage(pageLedger, myGroupsCursorString(params["cursor"]), data, myGroupsPayload(rt, groups), false); err != nil {
		return apperrors.NewInternal("记录我的群分页证据失败", apperrors.WithCause(err))
	}
	payload := myGroupsPayload(rt, groups)
	chatmsg.ApplyPagination(payload, data)
	payload["pagesFetched"] = 1
	if payload["complete"] == true {
		payload["stopReason"] = "source_complete"
	} else {
		payload["stopReason"] = "single_page"
	}
	result, resultErr := myGroupsUnifiedResult(pageLedger, payload, rt.DryRun())
	if resultErr != nil {
		return apperrors.NewInternal("生成我的群统一分页候选结果失败", apperrors.WithCause(resultErr))
	}
	return rt.OutputResult(payload, result)
}

func myGroupsPayload(rt *shortcut.RuntimeContext, groups []map[string]any) map[string]any {
	typeFilter := strings.TrimSpace(rt.Str("type"))
	projected := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		row := myGroupsProject(group)
		if typeFilter != "" {
			groupType, _ := row["type"].(string)
			if !strings.EqualFold(strings.TrimSpace(groupType), typeFilter) {
				continue
			}
		}
		projected = append(projected, row)
	}
	return map[string]any{"count": len(projected), "groups": projected}
}

func readAllMyGroups(rt *shortcut.RuntimeContext, baseParams map[string]any, pageLedger *output.PageLedger) (map[string]any, error) {
	pageLimit := defaultChatPageLimit(rt.Int("page-limit"), myGroupsDefaultPageLimit)
	cursorValue := baseParams["cursor"]
	cursorKey := myGroupsCursorString(cursorValue)
	if cursorKey == "" {
		cursorKey = "0"
	}
	seenCursors := map[string]bool{cursorKey: true}
	seenGroups := map[string]bool{}
	allGroups := make([]map[string]any, 0)
	failures := make([]map[string]any, 0)
	pagesFetched := 0
	complete := false
	hasMore := false
	stopReason := "source_complete"
	truncatedByPageLimit := false
	projectionFailure := false
	var nextCursor any

	for pagesFetched < pageLimit {
		params := map[string]any{"limit": baseParams["limit"]}
		if cursorKey != "0" {
			params["cursor"] = cursorValue
		}
		data, err := rt.CallMCPData("im", "list_my_groups_pagination", params)
		if err != nil {
			if recordErr := pageLedger.RecordFailure(cursorKey, myGroupsReadFailureInfo(err)); recordErr != nil {
				return nil, apperrors.NewInternal("记录我的群读取失败状态失败", apperrors.WithCause(recordErr))
			}
			if pagesFetched == 0 {
				return nil, err
			}
			failures = append(failures, map[string]any{
				"page": pagesFetched + 1, "stage": "read", "cursor": cursorKey, "error": err.Error(),
			})
			stopReason = "read_failure"
			break
		}
		pagesFetched++
		pageGroups, projectErr := myGroupsExtract(data)
		if projectErr != nil {
			if recordErr := pageLedger.RecordFailure(cursorKey, myGroupsProjectionFailureInfo(projectErr)); recordErr != nil {
				return nil, apperrors.NewInternal("记录我的群投影失败状态失败", apperrors.WithCause(recordErr))
			}
			if pagesFetched == 1 && len(allGroups) == 0 {
				return nil, projectErr
			}
			projectionFailure = true
			failures = append(failures, map[string]any{
				"page": pagesFetched, "stage": "response_projection", "cursor": cursorKey, "error": projectErr.Error(),
			})
			stopReason = "projection_error"
			break
		}
		for _, group := range pageGroups {
			id := myGroupsStr(group, "openConversationId", "openConversationID", "conversationId", "openCid", "cid", "id")
			if id != "" && seenGroups[id] {
				continue
			}
			if id != "" {
				seenGroups[id] = true
			}
			allGroups = append(allGroups, group)
		}
		if observeErr := observeMyGroupsPage(pageLedger, cursorKey, data, myGroupsPayload(rt, pageGroups), true); observeErr != nil {
			return nil, apperrors.NewInternal("记录我的群分页证据失败", apperrors.WithCause(observeErr))
		}

		page := chatmsg.Pagination(data)
		pageHasMore, known := page["hasMore"].(bool)
		if !known {
			failures = append(failures, map[string]any{
				"page": pagesFetched, "stage": "pagination",
				"error": "我的群列表下层未返回可靠的 hasMore，无法证明结果完整",
			})
			stopReason = "pagination_error"
			break
		}
		hasMore = pageHasMore
		if !hasMore {
			complete = true
			nextCursor = nil
			stopReason = "source_complete"
			break
		}
		nextCursor = page["nextCursor"]
		nextKey := myGroupsCursorString(nextCursor)
		if nextKey == "" || seenCursors[nextKey] {
			failures = append(failures, map[string]any{
				"page": pagesFetched, "stage": "pagination",
				"error": "我的群列表下层返回 hasMore=true，但 nextCursor 缺失、无效或未前进",
			})
			stopReason = "pagination_error"
			break
		}
		seenCursors[nextKey] = true
		cursorKey = nextKey
		cursorValue = nextCursor
	}
	if !complete && hasMore && len(failures) == 0 && pagesFetched >= pageLimit {
		truncatedByPageLimit = true
		stopReason = "page_limit"
	}

	payload := myGroupsPayload(rt, allGroups)
	payload["pagesFetched"] = pagesFetched
	payload["paginationKnown"] = true
	payload["complete"] = complete && len(failures) == 0
	payload["hasMore"] = hasMore
	payload["stopReason"] = stopReason
	payload["truncatedByPageLimit"] = truncatedByPageLimit
	payload["failedCount"] = len(failures)
	payload["failures"] = failures
	payload["partial"] = len(failures) > 0 && len(allGroups) > 0
	if hasMore && nextCursor != nil {
		payload["nextCursor"] = nextCursor
	}
	if len(failures) == 0 {
		return payload, nil
	}
	options := []apperrors.Option{
		apperrors.WithOperation("im/list_my_groups_pagination"),
		apperrors.WithSubtype(apperrors.SubtypeMyGroupsIncomplete),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("pagination"),
		apperrors.WithExecutionStarted(true),
		apperrors.WithHint("请根据 failures 和 nextCursor 重试"),
	}
	if projectionFailure {
		options = append(options, apperrors.WithRetryable(false))
	} else {
		options = append(options, apperrors.WithRetryable(true))
	}
	return payload, apperrors.NewAPI(
		fmt.Sprintf("我的群列表分页未完成：成功读取 %d 页，存在 %d 个失败项", pagesFetched, len(failures)),
		options...,
	)
}

// observeMyGroupsPage converts the gateway's pagination fields into one
// PageLedger record. It deliberately keeps unknown single-page pagination as
// unknown rather than falsely calling it exhausted. A page-all request treats
// absent continuation evidence as a boundary failure because it cannot finish
// the requested traversal safely.
func observeMyGroupsPage(pageLedger *output.PageLedger, cursor string, data map[string]any, pageData map[string]any, requireContinuation bool) error {
	if pageLedger == nil {
		return fmt.Errorf("missing pagination ledger")
	}
	items, _ := pageData["count"].(int)
	evidence := output.PageEvidence{Cursor: cursor, Items: items, Data: pageData}
	page := chatmsg.Pagination(data)
	hasMore, known := page["hasMore"].(bool)
	nextToken := myGroupsCursorString(page["nextCursor"])

	if !known {
		if err := pageLedger.ObservePage(evidence); err != nil {
			return myGroupsObserveFailure(pageLedger, cursor, err)
		}
		if !requireContinuation {
			return nil
		}
		return pageLedger.RecordBoundaryFailure(myGroupsPaginationFailureInfo("下层未返回可靠的 hasMore，无法继续全量分页"))
	}
	if !hasMore {
		if nextToken != "" {
			// Preserve the legacy payload in dual validation, but make the
			// candidate an explicit partial result instead of declaring this
			// contradictory response endpoint-exhausted.
			if err := pageLedger.ObservePage(evidence); err != nil {
				return myGroupsObserveFailure(pageLedger, cursor, err)
			}
			return pageLedger.RecordBoundaryFailure(myGroupsPaginationFailureInfo("hasMore=false，但同时携带可用 nextCursor"))
		}
		more := false
		evidence.HasMore = &more
		if err := pageLedger.ObservePage(evidence); err != nil {
			return myGroupsObserveFailure(pageLedger, cursor, err)
		}
		return nil
	}
	if nextToken == "" {
		if err := pageLedger.ObservePage(evidence); err != nil {
			return myGroupsObserveFailure(pageLedger, cursor, err)
		}
		return pageLedger.RecordBoundaryFailure(myGroupsPaginationFailureInfo("hasMore=true，但缺少可继续的 nextCursor"))
	}
	more := true
	evidence.HasMore = &more
	evidence.NextToken = nextToken
	if err := pageLedger.ObservePage(evidence); err != nil {
		return myGroupsObserveFailure(pageLedger, cursor, err)
	}
	return nil
}

// myGroupsObserveFailure attaches an invalid continuation to an already-read
// page as a typed partial boundary. The first page has no succeeded unit yet,
// so the caller must surface its typed framework error instead.
func myGroupsObserveFailure(pageLedger *output.PageLedger, cursor string, observeErr error) error {
	if pageLedger != nil && pageLedger.Pages() > 0 {
		if err := pageLedger.RecordPostPageFailure(myGroupsPaginationFailureInfo(observeErr.Error())); err == nil {
			return nil
		}
	}
	return observeErr
}

// myGroupsUnifiedResult is the candidate output contract. It excludes legacy
// fields such as complete/hasMore/nextCursor/failures: the framework owns
// outcome and endpoint pagination. PageLedger preserves each completed page
// when a later page fails, so the candidate can report partial_failure without
// asking an Agent to replay already-read groups.
func myGroupsUnifiedResult(pageLedger *output.PageLedger, payload map[string]any, dryRun bool) (output.CommandResult, error) {
	if pageLedger == nil {
		return nil, fmt.Errorf("missing pagination ledger")
	}
	data := map[string]any{}
	count := 0
	if payload != nil {
		if groups, ok := payload["groups"]; ok {
			data["groups"] = myGroupsUnifiedGroups(groups)
		}
		if value, ok := payload["count"]; ok {
			data["count"] = value
		}
		count, _ = payload["count"].(int)
	}
	if pageLedger.State() == output.PageStateUnknown {
		data["pagination_known"] = false
	}
	options := []output.ResultOption{output.WithMeta(&output.Meta{Count: output.NewCount(count)})}
	if dryRun {
		options = append(options, output.WithDryRun())
	}
	return pageLedger.Result(data, options...)
}

// myGroupsUnifiedGroups keeps the compatibility payload untouched while
// exposing the IM-wide canonical group handle in a future unified result.
// Other chat readers use openConversationId as the value accepted by follow-up
// commands; a bare display-oriented conversationId would keep two competing
// machine contracts alive after promotion.
func myGroupsUnifiedGroups(value any) any {
	groups, ok := value.([]map[string]any)
	if !ok {
		return value
	}
	canonical := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		row := make(map[string]any, len(group))
		for key, item := range group {
			row[key] = item
		}
		if conversationID, ok := row["conversationId"]; ok {
			row["openConversationId"] = conversationID
			delete(row, "conversationId")
		}
		canonical = append(canonical, row)
	}
	return canonical
}

func myGroupsReadFailureInfo(err error) *output.ErrorInfo {
	message := "我的群列表分页读取失败"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	info := &output.ErrorInfo{
		Type:             "api",
		Message:          message,
		Hint:             "从失败游标继续；不要重放已成功读取的页面。",
		Operation:        "im/list_my_groups_pagination",
		Origin:           "mcp_gateway",
		Stage:            "pagination_read",
		ExecutionStarted: &started,
		Retryable:        true,
	}
	return shortcut.PreserveTypedErrorInfo(info, err)
}

func myGroupsPaginationFailureInfo(message string) *output.ErrorInfo {
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypePaginationInconsistent),
		Message:          strings.TrimSpace(message),
		Hint:             "保留已读取页面；不要把当前结果解释为 endpoint 已耗尽。",
		Operation:        "im/list_my_groups_pagination",
		Origin:           "mcp_gateway",
		Stage:            "pagination_projection",
		ExecutionStarted: &started,
	}
}

func myGroupsProjectionFailureInfo(err error) *output.ErrorInfo {
	message := "我的群列表响应无法可靠投影"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypeProjectionUnknown),
		Message:          message,
		Hint:             "检查服务端返回结构；不要把未知响应形状解释为空群列表。",
		Operation:        "im/list_my_groups_pagination",
		Origin:           "mcp_gateway",
		Stage:            "projection",
		ExecutionStarted: &started,
	}
}

func myGroupsCursorString(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" || text == "0" {
		return ""
	}
	return text
}

// myGroupsExtract walks a list_my_groups_pagination response and returns its
// group entries. The gateway wraps the list under one of several common
// container keys, so we probe them (and one nested level). A missing container
// or an item without a stable conversation ID is not an empty group list: the
// caller must fail closed instead of teaching an Agent that no target exists.
func myGroupsExtract(data map[string]any) ([]map[string]any, error) {
	for _, key := range []string{"result", "list", "groups", "groupList", "items", "data", "records", "conversations"} {
		if arr, ok := data[key].([]any); ok {
			return myGroupsToMaps(arr)
		}
		if inner, ok := data[key].(map[string]any); ok {
			for _, k2 := range []string{"list", "groups", "groupList", "items", "records", "result", "conversations"} {
				if arr, ok := inner[k2].([]any); ok {
					return myGroupsToMaps(arr)
				}
			}
		}
	}
	return nil, myGroupsProjectionUnknown("我的群列表响应缺少可识别的列表容器")
}

func myGroupsToMaps(arr []any) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, myGroupsProjectionUnknown("我的群列表包含无法识别的条目")
		}
		if myGroupsStr(m, "openConversationId", "openConversationID", "conversationId", "openCid", "cid", "id") == "" {
			return nil, myGroupsProjectionUnknown("我的群列表条目缺少稳定 conversationId")
		}
		out = append(out, m)
	}
	return out, nil
}

func myGroupsProjectionUnknown(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithOperation("im/list_my_groups_pagination"),
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

// myGroupsProject reshapes a single group into the projected key fields, probing
// multiple candidate keys for each because the response shape isn't guaranteed.
func myGroupsProject(m map[string]any) map[string]any {
	row := map[string]any{}
	if v := myGroupsStr(m, "openConversationId", "openConversationID", "conversationId", "openCid", "cid", "id"); v != "" {
		row["conversationId"] = v
	}
	if v := myGroupsStr(m, "name", "groupName", "title", "conversationName", "chatName"); v != "" {
		row["name"] = v
	}
	if v := myGroupsStr(m, "ownerUserId", "owner", "ownerId", "ownerOpenDingTalkId", "ownerOpenId", "groupOwnerId"); v != "" {
		row["owner"] = v
	}
	if v, ok := myGroupsInt(m, "memberCount", "memberNum", "memberSize", "userCount", "totalMember", "count"); ok {
		row["memberCount"] = v
	}
	if v := myGroupsStr(m, "groupType", "conversationType", "type", "chatType"); v != "" {
		row["type"] = v
	}
	return row
}

func myGroupsStr(m map[string]any, keys ...string) string {
	for _, key := range keys {
		switch v := m[key].(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(v)
		}
	}
	return ""
}

func myGroupsInt(m map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		switch v := m[key].(type) {
		case float64:
			return int64(v), true
		case string:
			if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func init() {
	shortcut.Register(MyGroups)
}
