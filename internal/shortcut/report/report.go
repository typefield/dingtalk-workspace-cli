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

// Package report declares declarative shortcuts for the DingTalk
// "报告/日志" (OA 周报应用) MCP tools. Each shortcut is a thin wrapper over a
// single MCP tool call; tool names and parameter keys mirror the DWS report
// helper (internal/helpers/report.go) verbatim.
package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// reportParseISOMillis parses an ISO-8601 timestamp (e.g. 2026-03-10T00:00:00+08:00)
// into Unix milliseconds, matching the helper's parseISOTimeToMillis behavior.
func reportParseISOMillis(name, s string) (int64, error) {
	s = strings.TrimSpace(s)
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, fmt.Errorf("--%s 时间格式无效，需 ISO-8601（如 2026-03-10T00:00:00+08:00）: %v", name, err)
	}
	return t.UnixMilli(), nil
}

// TemplateList lists the report templates available to the current user.
// TemplateGet reads a single report template's field definitions by name.
// InboxList lists reports received by the current user within a time window.
var InboxList = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "report",
	Command:       "+inbox-list",
	Product:       "report",
	Description:   "列出我收到的日报（按时间范围分页）",
	Intent:        "当你要查看下属或同事发给自己的日报周报、想在某个时间段内浏览或审阅收到的汇报时使用；输入起止时间（ISO-8601），可按发送人 staffId 过滤，分页返回收到的日报列表及其 reportId，供后续 +entry-get 读正文。",
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "report",
			Name:           "shortcut_inbox_list",
			CanonicalPath:  "report.shortcut_inbox_list",
			CLIPath:        "report +inbox-list",
			PrimaryCLIPath: "report +inbox-list",
		},
		Description: "列出我收到的日报（按时间范围分页）",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "列出我收到的日报（按时间范围分页）",
			UseWhen:      []string{"当你要查看下属或同事发给自己的日报周报、想在某个时间段内浏览或审阅收到的汇报时使用；输入起止时间（ISO-8601），可按发送人 staffId 过滤，分页返回收到的日报列表及其 reportId，供后续 +entry-get 读正文。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws report +inbox-list --start \"2026-03-10T00:00:00+08:00\" --end \"2026-03-10T23:59:59+08:00\" --cursor 0 --size 20"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "start", Type: shortcut.FlagString, Desc: "开始时间 ISO-8601 (如 2026-03-10T00:00:00+08:00)", Required: true},
		{Name: "end", Type: shortcut.FlagString, Desc: "结束时间 ISO-8601 (如 2026-03-10T23:59:59+08:00)", Required: true},
		{Name: "cursor", Type: shortcut.FlagInt, Default: "0", Desc: "分页游标，首次传 0"},
		{Name: "size", Type: shortcut.FlagInt, Default: "20", Desc: "每页条数，最大 20"},
		{Name: "sender-user-ids", Type: shortcut.FlagStringSlice, Desc: "发送人 staffId 列表，逗号分隔，过滤指定发送人"},
	},
	Tips: []string{
		`dws report +inbox-list --start "2026-03-10T00:00:00+08:00" --end "2026-03-10T23:59:59+08:00" --cursor 0 --size 20`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		startMs, err := reportParseISOMillis("start", rt.Str("start"))
		if err != nil {
			return err
		}
		endMs, err := reportParseISOMillis("end", rt.Str("end"))
		if err != nil {
			return err
		}
		params := map[string]any{
			"startTime": startMs,
			"endTime":   endMs,
			"cursor":    rt.Int("cursor"),
			"size":      rt.Int("size"),
		}
		if rt.Changed("sender-user-ids") {
			params["senderUserIds"] = rt.StrSlice("sender-user-ids")
		}
		data, err := rt.CallMCPData("report", "get_received_report_list", params)
		if err != nil {
			return err
		}
		reports, err := reportEntryListProject(data)
		if err != nil {
			return err
		}
		payload, result, err := reportListResult(data, reports)
		if err != nil {
			return err
		}
		return rt.OutputResult(payload, result)
	},
}

// reportEntryListProject reshapes the raw report-list responses
// (get_received_report_list / get_send_report_list) into a clean, stable list
// of report summaries — the output-projection fidelity the framework applies to
// every list command. Both the list container and the per-item field names are
// probed defensively across candidate keys so the projection tolerates
// response-shape drift. A known empty list remains a successful empty result;
// an unknown container or row is a typed error rather than a fabricated empty
// list. This distinction is essential for Agents: "no reports" must never
// mean "the response could not be understood".
func reportEntryListProject(data map[string]any) ([]map[string]any, error) {
	raw, known := reportEntryListResolveList(data)
	if !known {
		return nil, reportProjectionUnknown("报告列表响应缺少可识别的列表容器")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, reportProjectionUnknown("报告列表包含无法识别的条目")
		}
		row := map[string]any{}
		if v, ok := reportEntryListFirst(m, "reportId", "report_id", "id"); ok {
			row["reportId"] = v
		}
		if v, ok := reportEntryListFirst(m, "templateName", "template_name", "reportTemplateName"); ok {
			row["templateName"] = v
		}
		if v, ok := reportEntryListFirst(m, "creatorName", "creator_name", "creatorUserName", "senderName", "authorName"); ok {
			row["creatorName"] = v
		}
		if v, ok := reportEntryListFirst(m, "creatorUserId", "creator_user_id", "creatorId", "senderUserId", "userId"); ok {
			row["creatorUserId"] = v
		}
		if v, ok := reportEntryListFirst(m, "createTime", "create_time", "gmtCreate", "sendTime"); ok {
			row["createTime"] = v
		}
		if v, ok := reportEntryListFirst(m, "modifiedTime", "modified_time", "gmtModified", "modifyTime"); ok {
			row["modifiedTime"] = v
		}
		if len(row) == 0 {
			return nil, reportProjectionUnknown("报告列表条目缺少可识别字段")
		}
		out = append(out, row)
	}
	return out, nil
}

// reportEntryListResolveList locates the list payload inside the response,
// tolerating a bare top-level array or nesting one level under a common
// envelope key.
func reportEntryListResolveList(data map[string]any) ([]any, bool) {
	if data == nil {
		return nil, false
	}
	// get_received_report_list / get_send_report_list nest the list under
	// result.report_list (snake_case); "report_list" MUST be probed — the
	// camelCase "reportList" alone leaves +inbox-list / +outbox-list silently
	// empty despite the backend returning reports.
	for _, key := range []string{"result", "data", "list", "items", "report_list", "reportList", "records"} {
		v, ok := data[key]
		if !ok {
			continue
		}
		if arr, ok := v.([]any); ok {
			return arr, true
		}
		if inner, ok := v.(map[string]any); ok {
			for _, ik := range []string{"list", "items", "report_list", "reportList", "records", "result", "data"} {
				if arr, ok := inner[ik].([]any); ok {
					return arr, true
				}
			}
		}
	}
	return nil, false
}

func reportProjectionUnknown(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

// reportListResult preserves the API pagination signal in both the legacy
// payload and unified meta.pagination. An absent signal remains explicitly unknown;
// a contradictory or non-resumable signal fails closed rather than claiming a
// terminal page. Report cursors are numerically accepted by the command, but
// represented as strings in the unified contract so every resume handle has one
// stable wire type.
func reportListResult(data map[string]any, reports []map[string]any) (map[string]any, output.CommandResult, error) {
	payload := map[string]any{"count": len(reports), "reports": reports}
	page, known, err := reportListPagination(data)
	if err != nil {
		return nil, nil, err
	}
	meta := &output.Meta{Count: output.NewCount(len(reports))}
	payload["paginationKnown"] = known
	if known {
		meta.Pagination = page
		payload["hasMore"] = !page.EndpointExhausted
		payload["endpointExhausted"] = page.EndpointExhausted
		if page.NextToken != "" {
			payload["nextCursor"] = page.NextToken
		}
	}
	return payload, output.Success(payload, output.WithMeta(meta)), nil
}

func reportListPagination(data map[string]any) (*output.Pagination, bool, error) {
	if data == nil {
		return nil, false, nil
	}
	var selected *output.Pagination
	for _, scope := range reportPaginationScopes(data) {
		page, present, err := reportPaginationFromScope(scope)
		if err != nil {
			return nil, false, err
		}
		if !present {
			continue
		}
		if selected != nil && (selected.EndpointExhausted != page.EndpointExhausted || selected.NextToken != page.NextToken) {
			return nil, false, reportPaginationError("报告列表响应的分页字段在嵌套容器间互相矛盾")
		}
		selected = page
	}
	if selected == nil {
		return nil, false, nil
	}
	return selected, true, nil
}

func reportPaginationScopes(data map[string]any) []map[string]any {
	scopes := []map[string]any{data}
	for _, key := range []string{"result", "data"} {
		if nested, ok := data[key].(map[string]any); ok {
			scopes = append(scopes, nested)
		}
	}
	return scopes
}

func reportPaginationFromScope(scope map[string]any) (*output.Pagination, bool, error) {
	if scope == nil {
		return nil, false, nil
	}
	rawMore, hasMore := reportFirstPresent(scope, "hasMore", "has_more")
	rawCursor, hasCursor := reportFirstPresent(scope, "nextCursor", "next_cursor", "nextToken", "next_token", "cursor")
	if !hasMore && !hasCursor {
		return nil, false, nil
	}
	if !hasMore {
		return nil, false, reportPaginationError("报告列表返回 continuation cursor，但没有 hasMore")
	}
	more, ok := rawMore.(bool)
	if !ok {
		return nil, false, reportPaginationError("报告列表的 hasMore 必须是布尔值")
	}
	cursor, err := reportPaginationToken(rawCursor, hasCursor)
	if err != nil {
		return nil, false, err
	}
	page, err := output.NewPagination(!more, cursor)
	if err != nil {
		return nil, false, reportPaginationError(err.Error())
	}
	return page, true, nil
}

func reportFirstPresent(scope map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := scope[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func reportPaginationToken(value any, present bool) (string, error) {
	if !present || value == nil {
		return "", nil
	}
	switch typed := value.(type) {
	case string:
		cursor := strings.TrimSpace(typed)
		if cursor == "0" {
			return "", nil
		}
		return cursor, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		cursor := fmt.Sprint(typed)
		if cursor == "0" {
			return "", nil
		}
		return cursor, nil
	case float32:
		if typed != float32(int64(typed)) {
			return "", reportPaginationError("报告列表的 nextCursor 必须是整数或字符串")
		}
		cursor := fmt.Sprint(int64(typed))
		if cursor == "0" {
			return "", nil
		}
		return cursor, nil
	case float64:
		if typed != float64(int64(typed)) {
			return "", reportPaginationError("报告列表的 nextCursor 必须是整数或字符串")
		}
		cursor := fmt.Sprint(int64(typed))
		if cursor == "0" {
			return "", nil
		}
		return cursor, nil
	default:
		return "", reportPaginationError("报告列表的 nextCursor 必须是整数或字符串")
	}
}

func reportPaginationError(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypePaginationInconsistent),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

// reportEntryListFirst returns the first present candidate key's value.
func reportEntryListFirst(m map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	return nil, false
}

// OutboxList lists reports sent/created by the current user.
var OutboxList = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "report",
	Command:       "+outbox-list",
	Product:       "report",
	Description:   "列出我发出的日报（可选时间/模版名过滤）",
	Intent:        "当你要回顾自己写过、提交过的日报周报，比如确认某天是否已交、找回历史汇报内容或统计提交情况时使用；可按创建/修改时间范围和模版名过滤，分页返回自己发出的日报列表及 reportId，供后续 +entry-get 查看正文。",
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "report",
			Name:           "shortcut_outbox_list",
			CanonicalPath:  "report.shortcut_outbox_list",
			CLIPath:        "report +outbox-list",
			PrimaryCLIPath: "report +outbox-list",
		},
		Description: "列出我发出的日报（可选时间/模版名过滤）",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "列出我发出的日报（可选时间/模版名过滤）",
			UseWhen:      []string{"当你要回顾自己写过、提交过的日报周报，比如确认某天是否已交、找回历史汇报内容或统计提交情况时使用；可按创建/修改时间范围和模版名过滤，分页返回自己发出的日报列表及 reportId，供后续 +entry-get 查看正文。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples: []string{
				"dws report +outbox-list --cursor 0 --size 20",
				"dws report +outbox-list --cursor 0 --size 20 --template-name \"日报\"",
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "cursor", Type: shortcut.FlagInt, Default: "0", Desc: "分页游标，首次传 0"},
		{Name: "size", Type: shortcut.FlagInt, Default: "20", Desc: "每页条数，最大 20"},
		{Name: "start", Type: shortcut.FlagString, Desc: "创建开始时间 ISO-8601 (可选，服务端单次跨度上限 20 天)"},
		{Name: "end", Type: shortcut.FlagString, Desc: "创建结束时间 ISO-8601 (可选)"},
		{Name: "modified-start", Type: shortcut.FlagString, Desc: "修改开始时间 ISO-8601 (可选)"},
		{Name: "modified-end", Type: shortcut.FlagString, Desc: "修改结束时间 ISO-8601 (可选)"},
		{Name: "template-name", Type: shortcut.FlagString, Desc: "日志模板名称 (可选，不传查全部)"},
	},
	Tips: []string{
		`dws report +outbox-list --cursor 0 --size 20`,
		`dws report +outbox-list --cursor 0 --size 20 --template-name "日报"`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"cursor": rt.Int("cursor"),
			"size":   rt.Int("size"),
		}
		if rt.Changed("start") {
			ms, err := reportParseISOMillis("start", rt.Str("start"))
			if err != nil {
				return err
			}
			params["startTime"] = ms
		}
		if rt.Changed("end") {
			ms, err := reportParseISOMillis("end", rt.Str("end"))
			if err != nil {
				return err
			}
			params["endTime"] = ms
		}
		if rt.Changed("modified-start") {
			ms, err := reportParseISOMillis("modified-start", rt.Str("modified-start"))
			if err != nil {
				return err
			}
			params["modifiedStartTime"] = ms
		}
		if rt.Changed("modified-end") {
			ms, err := reportParseISOMillis("modified-end", rt.Str("modified-end"))
			if err != nil {
				return err
			}
			params["modifiedEndTime"] = ms
		}
		if rt.Changed("template-name") {
			params["report_template_name"] = rt.Str("template-name")
		}
		data, err := rt.CallMCPData("report", "get_send_report_list", params)
		if err != nil {
			return err
		}
		reports, err := reportEntryListProject(data)
		if err != nil {
			return err
		}
		payload, result, err := reportListResult(data, reports)
		if err != nil {
			return err
		}
		return rt.OutputResult(payload, result)
	},
}

// EntryGet reads the full body of a single report by id.
// EntryStats reads the read-receipt statistics of a single report.
// EntrySubmit submits a new report against a template.
func init() {
	shortcut.Register(
		InboxList,
		OutboxList,
	)
}
