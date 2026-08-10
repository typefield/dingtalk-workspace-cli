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

// Package doc declares the high-fidelity `dws doc +<command>` shortcuts.
// Tool names and parameter keys are lifted verbatim from
// internal/helpers/doc.go (the single source of truth for DingTalk MCP tools).
package doc

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

const (
	productDoc     = "doc"
	productComment = "doc-comment"
)

// ── 文档浏览 / 读取 ──────────────────────────────────────────

var Search = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "doc",
	Command:       "+search",
	Product:       productDoc,
	Description:   "按关键词搜索有权限的文档 (不传则返回最近访问)",
	Intent:        "当你只记得文档的标题或主题词、需要先定位到某篇钉钉文档拿到它的 nodeId/URL 以便后续阅读或编辑时使用；可按关键词、扩展名、创建/访问时间、创建者等条件过滤，不传关键词则返回最近访问的文档，返回匹配的文档列表。",
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_search",
			CanonicalPath:  "doc.shortcut_search",
			CLIPath:        "doc +search",
			PrimaryCLIPath: "doc +search",
		},
		Description: "按关键词搜索有权限的文档 (不传则返回最近访问)",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "按关键词搜索有权限的文档 (不传则返回最近访问)",
			UseWhen:      []string{"当你只记得文档的标题或主题词、需要先定位到某篇钉钉文档拿到它的 nodeId/URL 以便后续阅读或编辑时使用；可按关键词、扩展名、创建/访问时间、创建者等条件过滤，不传关键词则返回最近访问的文档，返回匹配的文档列表。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples: []string{
				"dws doc +search --query \"会议纪要\"",
				"dws doc +search --extensions pdf,docx",
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "query", Type: shortcut.FlagString, Desc: "搜索关键词，不传返回最近访问的文档"},
		{Name: "extensions", Type: shortcut.FlagStringSlice, Desc: "按文件扩展名过滤 (如 adoc,axls,pdf)"},
		{Name: "created-from", Type: shortcut.FlagInt, Desc: "创建时间起始 (毫秒时间戳)；创建时间必须为非负毫秒时间戳，且起始时间不得晚于截止时间"},
		{Name: "created-to", Type: shortcut.FlagInt, Desc: "创建时间截止 (毫秒时间戳)；创建时间必须为非负毫秒时间戳，且起始时间不得晚于截止时间"},
		{Name: "visited-from", Type: shortcut.FlagInt, Desc: "访问时间起始 (毫秒时间戳)；访问时间必须为非负毫秒时间戳，且起始时间不得晚于截止时间"},
		{Name: "visited-to", Type: shortcut.FlagInt, Desc: "访问时间截止 (毫秒时间戳)；访问时间必须为非负毫秒时间戳，且起始时间不得晚于截止时间"},
		{Name: "creator-uids", Type: shortcut.FlagStringSlice, Desc: "按创建者用户 ID 过滤"},
		{Name: "editor-uids", Type: shortcut.FlagStringSlice, Desc: "按编辑者用户 ID 过滤"},
		{Name: "mentioned-uids", Type: shortcut.FlagStringSlice, Desc: "按 @提及的用户 ID 过滤"},
		{Name: "workspace-ids", Type: shortcut.FlagStringSlice, Desc: "按知识库 ID 过滤"},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "每页数量 (默认 10)；显式 --limit 必须在 1-30 之间"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标 (上次结果的 nextPageToken)"},
	},
	Constraints: []shortcut.Constraint{
		{Kind: shortcut.ConstraintCustom, Flags: []string{"limit"}, Description: "显式 --limit 必须在 1-30 之间"},
		{Kind: shortcut.ConstraintCustom, Flags: []string{"created-from", "created-to"}, Description: "创建时间必须为非负毫秒时间戳，且起始时间不得晚于截止时间"},
		{Kind: shortcut.ConstraintCustom, Flags: []string{"visited-from", "visited-to"}, Description: "访问时间必须为非负毫秒时间戳，且起始时间不得晚于截止时间"},
	},
	Tips: []string{`dws doc +search --query "会议纪要"`, `dws doc +search --extensions pdf,docx`},
	Validate: func(rt *shortcut.RuntimeContext) error {
		if err := docValidateLimit(rt, "limit", 30); err != nil {
			return err
		}
		if err := docValidateTimeRange(rt, "created-from", "created-to"); err != nil {
			return err
		}
		return docValidateTimeRange(rt, "visited-from", "visited-to")
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{}
		if v := rt.Str("query"); v != "" {
			params["keyword"] = v
		}
		if rt.Changed("extensions") {
			params["extensions"] = rt.StrSlice("extensions")
		}
		if rt.Changed("created-from") {
			params["createdTimeFrom"] = rt.Int("created-from")
		}
		if rt.Changed("created-to") {
			params["createdTimeTo"] = rt.Int("created-to")
		}
		if rt.Changed("visited-from") {
			params["visitedTimeFrom"] = rt.Int("visited-from")
		}
		if rt.Changed("visited-to") {
			params["visitedTimeTo"] = rt.Int("visited-to")
		}
		if rt.Changed("creator-uids") {
			params["creatorUserIds"] = rt.StrSlice("creator-uids")
		}
		if rt.Changed("editor-uids") {
			params["editorUserIds"] = rt.StrSlice("editor-uids")
		}
		if rt.Changed("mentioned-uids") {
			params["mentionedUserIds"] = rt.StrSlice("mentioned-uids")
		}
		if rt.Changed("workspace-ids") {
			params["workspaceIds"] = rt.StrSlice("workspace-ids")
		}
		if rt.Changed("limit") {
			params["pageSize"] = rt.Int("limit")
		}
		if rt.Changed("cursor") {
			params["pageToken"] = rt.Str("cursor")
		}
		data, err := rt.CallMCPData(productDoc, "search_documents", params)
		if err != nil {
			return err
		}
		docs, err := searchDocsProject(data)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"count":                len(docs),
			"documents":            docs,
			"index_coverage_known": false,
		}
		return docListOutput(rt, data, payload, len(docs))
	},
}

// searchDocsProject reshapes the raw search_documents response into a clean
// document list ({nodeId, name, docType, url, creatorId, modifiedTime}) —
// clean output projection. Candidate aliases are accepted, while an unknown
// container or row fails closed rather than becoming a successful empty search.
func searchDocsProject(data map[string]any) ([]map[string]any, error) {
	raw, known := docResolveList(data)
	if !known {
		return nil, docProjectionUnknown("无法识别 search_documents 返回的文档列表容器")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, docProjectionUnknown("文档搜索结果包含无法识别的条目")
		}
		nodeID, present, err := docStringField(m, "nodeId", "nodeId", "node_id", "id", "docId", "doc_id")
		if err != nil {
			return nil, err
		}
		if !present || strings.TrimSpace(nodeID) == "" {
			return nil, docProjectionUnknown("文档搜索结果条目缺少可用于后续操作的稳定 nodeId")
		}
		row := map[string]any{"nodeId": nodeID}
		if v, ok, err := docStringField(m, "name", "name", "title", "docName", "fileName"); err != nil {
			return nil, err
		} else if ok {
			row["name"] = v
		}
		if v, ok, err := docStringField(m, "docType", "docType", "doc_type", "type", "extension", "fileType", "nodeType", "contentType"); err != nil {
			return nil, err
		} else if ok {
			row["docType"] = v
		}
		if v, ok, err := docStringField(m, "url", "url", "docUrl", "nodeUrl", "webUrl"); err != nil {
			return nil, err
		} else if ok {
			row["url"] = v
		}
		if v, ok, err := docStringField(m, "creatorId", "creatorId", "creatorUserId", "creator_user_id", "creatorUid", "creator"); err != nil {
			return nil, err
		} else if ok {
			row["creatorId"] = v
		}
		if v, ok, err := docTimestampField(m, "modifiedTime", "modifiedTime", "gmtModified", "visitedTime", "lastEditTime", "updateTime", "modifyTime"); err != nil {
			return nil, err
		} else if ok {
			row["modifiedTime"] = v
		}
		out = append(out, row)
	}
	return out, nil
}

// docResolveList locates the list payload inside a doc-service response,
// tolerating a bare top-level array or nesting under common envelope keys, and
// optionally one level deeper inside a result/data container.
func docResolveList(data map[string]any) ([]any, bool) {
	if data == nil {
		return nil, false
	}
	for _, key := range []string{"nodes", "documents", "list", "items", "result", "data", "records"} {
		v, ok := data[key]
		if !ok {
			continue
		}
		if arr, ok := v.([]any); ok {
			return arr, true
		}
		if inner, ok := v.(map[string]any); ok {
			for _, ik := range []string{"nodes", "documents", "list", "items", "records", "result", "data"} {
				if arr, ok := inner[ik].([]any); ok {
					return arr, true
				}
			}
		}
	}
	return nil, false
}

var List = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "doc",
	Command:       "+list",
	Product:       productDoc,
	Description:   "列出文件夹、知识库或默认文档根目录下的直接子节点",
	Intent:        "当你想浏览某个文档文件夹、知识库或默认文档根目录下面直接包含的文档与子文件夹（不递归深层）以便逐层导航时使用；folder 优先于 workspace，二者都不传时浏览默认根目录。",
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_list",
			CanonicalPath:  "doc.shortcut_list",
			CLIPath:        "doc +list",
			PrimaryCLIPath: "doc +list",
		},
		Description: "列出文件夹、知识库或默认文档根目录下的直接子节点",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "列出文件夹、知识库或默认文档根目录下的直接子节点",
			UseWhen:      []string{"当你想浏览某个文档文件夹、知识库或默认文档根目录下面直接包含的文档与子文件夹（不递归深层）以便逐层导航时使用；folder 优先于 workspace，二者都不传时浏览默认根目录。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples: []string{
				"dws doc +list",
				"dws doc +list --folder DOC_FOLDER_NODE_ID",
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "folder", Type: shortcut.FlagString, Desc: "文档文件夹 nodeId 或 alidocs 文件夹 URL"},
		{Name: "workspace", Type: shortcut.FlagString, Desc: "知识库 ID"},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "每页数量 (默认 50)；显式 --limit 必须在 1-50 之间"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标 (上次结果的 nextPageToken)"},
	},
	Constraints: []shortcut.Constraint{
		{Kind: shortcut.ConstraintCustom, Flags: []string{"limit"}, Description: "显式 --limit 必须在 1-50 之间"},
	},
	Tips: []string{`dws doc +list`, `dws doc +list --folder DOC_FOLDER_NODE_ID`, `dws doc +list --workspace WS_ID --limit 20`},
	Validate: func(rt *shortcut.RuntimeContext) error {
		return docValidateLimit(rt, "limit", 50)
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{}
		if rt.Changed("folder") {
			params["folderId"] = rt.Str("folder")
		}
		if rt.Changed("workspace") {
			params["workspaceId"] = rt.Str("workspace")
		}
		if rt.Changed("limit") {
			params["pageSize"] = rt.Int("limit")
		}
		if rt.Changed("cursor") {
			params["pageToken"] = rt.Str("cursor")
		}
		data, err := rt.CallMCPData(productDoc, "list_nodes", params)
		if err != nil {
			return err
		}
		nodes, err := listNodesProject(data)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"count":           len(nodes),
			"nodes":           nodes,
			"inventory_scope": "requested_location",
		}
		return docListOutput(rt, data, payload, len(nodes))
	},
}

// listNodesProject reshapes the raw list_nodes response into a clean child-node
// list ({nodeId, name, nodeType, url}) — clean output projection.
// The list container and per-item aliases are probed defensively. Unknown
// response shapes are surfaced as projection failures, never as an empty folder.
func listNodesProject(data map[string]any) ([]map[string]any, error) {
	raw, known := docResolveList(data)
	if !known {
		return nil, docProjectionUnknown("无法识别 list_nodes 返回的子节点列表容器")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, docProjectionUnknown("子节点列表包含无法识别的条目")
		}
		nodeID, present, err := docStringField(m, "nodeId", "nodeId", "node_id", "id", "docId", "doc_id")
		if err != nil {
			return nil, err
		}
		if !present || strings.TrimSpace(nodeID) == "" {
			return nil, docProjectionUnknown("子节点条目缺少可用于后续操作的稳定 nodeId")
		}
		row := map[string]any{"nodeId": nodeID}
		if v, ok, err := docStringField(m, "name", "name", "title", "nodeName", "fileName"); err != nil {
			return nil, err
		} else if ok {
			row["name"] = v
		}
		if v, ok, err := docStringField(m, "nodeType", "nodeType", "node_type", "docType", "type", "extension"); err != nil {
			return nil, err
		} else if ok {
			row["nodeType"] = v
		}
		if v, ok, err := docStringField(m, "url", "url", "nodeUrl", "docUrl", "webUrl"); err != nil {
			return nil, err
		} else if ok {
			row["url"] = v
		}
		if v, ok, err := docBoolField(m, "hasChildren", "hasChildren", "has_children"); err != nil {
			return nil, err
		} else if ok {
			row["hasChildren"] = v
		}
		if v, ok, err := docStringField(m, "workspaceId", "workspaceId", "workspace_id"); err != nil {
			return nil, err
		} else if ok {
			row["workspaceId"] = v
		}
		out = append(out, row)
	}
	return out, nil
}

func docProjectionUnknown(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

// docListOutput emits the one active result contract for the two document
// discovery commands. Pagination is published only when the service supplied
// an authoritative, internally consistent hasMore/nextPageToken pair.
func docListOutput(rt *shortcut.RuntimeContext, raw, payload map[string]any, count int) error {
	page, known, err := docDiscoveryPagination(raw)
	if err != nil {
		return err
	}
	payload["pagination_known"] = known
	meta := &output.Meta{Count: output.NewCount(count)}
	if known {
		meta.Pagination = page
	}
	return rt.OutputResult(payload, output.Success(payload, output.WithMeta(meta)))
}

func docDiscoveryPagination(data map[string]any) (*output.Pagination, bool, error) {
	var selected *output.Pagination
	for _, scope := range docPaginationScopes(data) {
		page, present, err := docPaginationFromScope(scope)
		if err != nil {
			return nil, false, err
		}
		if !present {
			continue
		}
		if selected != nil && (selected.EndpointExhausted != page.EndpointExhausted || selected.NextToken != page.NextToken) {
			return nil, false, docPaginationError("文档发现响应的分页字段在嵌套容器间互相矛盾")
		}
		selected = page
	}
	if selected == nil {
		return nil, false, nil
	}
	return selected, true, nil
}

func docPaginationScopes(data map[string]any) []map[string]any {
	if data == nil {
		return nil
	}
	scopes := []map[string]any{data}
	for _, key := range []string{"result", "data"} {
		if inner, ok := data[key].(map[string]any); ok {
			scopes = append(scopes, inner)
		}
	}
	return scopes
}

func docPaginationFromScope(scope map[string]any) (*output.Pagination, bool, error) {
	if scope == nil {
		return nil, false, nil
	}
	more, hasMore, err := docPaginationBoolAliases(scope, "hasMore", "has_more")
	if err != nil {
		return nil, false, err
	}
	cursor, hasCursor, err := docPaginationCursorAliases(scope,
		"nextPageToken", "next_page_token", "nextCursor", "next_cursor",
		"nextToken", "next_token", "pageToken", "page_token")
	if err != nil {
		return nil, false, err
	}
	if !hasMore && !hasCursor {
		return nil, false, nil
	}
	if !hasMore {
		return nil, false, docPaginationError("文档发现响应携带续页游标但缺少 hasMore")
	}
	page, err := output.NewPagination(!more, cursor)
	if err != nil {
		return nil, false, docPaginationError(err.Error())
	}
	return page, true, nil
}

func docPaginationBoolAliases(scope map[string]any, keys ...string) (bool, bool, error) {
	var selected bool
	present := false
	for _, key := range keys {
		value, ok := scope[key]
		if !ok {
			continue
		}
		flag, ok := value.(bool)
		if !ok {
			return false, false, docPaginationError("文档发现响应的 hasMore 必须是布尔值")
		}
		if present && selected != flag {
			return false, false, docPaginationError("文档发现响应的 hasMore 别名互相矛盾")
		}
		selected, present = flag, true
	}
	return selected, present, nil
}

func docPaginationCursorAliases(scope map[string]any, keys ...string) (string, bool, error) {
	selected := ""
	present := false
	for _, key := range keys {
		value, ok := scope[key]
		if !ok {
			continue
		}
		cursor, err := docPaginationToken(value, true)
		if err != nil {
			return "", false, err
		}
		if present && selected != cursor {
			return "", false, docPaginationError("文档发现响应的续页游标别名互相矛盾")
		}
		selected, present = cursor, true
	}
	return selected, present, nil
}

func docPaginationToken(value any, present bool) (string, error) {
	if !present || value == nil {
		return "", nil
	}
	switch typed := value.(type) {
	case string:
		cursor := strings.TrimSpace(typed)
		if cursor == "0" || cursor == "$" {
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
		return docPaginationFloatToken(float64(typed))
	case float64:
		return docPaginationFloatToken(typed)
	default:
		return "", docPaginationError(fmt.Sprintf("文档发现响应的续页游标类型无效: %T", value))
	}
}

func docPaginationFloatToken(value float64) (string, error) {
	const maxExactJSONInteger = float64(1<<53 - 1)
	if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) || math.Abs(value) > maxExactJSONInteger {
		return "", docPaginationError("文档发现响应的续页游标必须是字符串或整数")
	}
	if value == 0 {
		return "", nil
	}
	return strconv.FormatInt(int64(value), 10), nil
}

func docPaginationError(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypePaginationInconsistent),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

func docStringField(m map[string]any, outputName string, keys ...string) (string, bool, error) {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return "", false, docProjectionUnknown(fmt.Sprintf("文档发现结果字段 %s 必须是字符串", outputName))
		}
		return text, true, nil
	}
	return "", false, nil
}

func docBoolField(m map[string]any, outputName string, keys ...string) (bool, bool, error) {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		flag, ok := value.(bool)
		if !ok {
			return false, false, docProjectionUnknown(fmt.Sprintf("文档发现结果字段 %s 必须是布尔值", outputName))
		}
		return flag, true, nil
	}
	return false, false, nil
}

func docTimestampField(m map[string]any, outputName string, keys ...string) (any, bool, error) {
	for _, key := range keys {
		value, ok := m[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed, true, nil
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return typed, true, nil
		case float32:
			if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) || typed != float32(math.Trunc(float64(typed))) {
				return nil, false, docProjectionUnknown(fmt.Sprintf("文档发现结果字段 %s 必须是时间字符串或整数时间戳", outputName))
			}
			return typed, true, nil
		case float64:
			if math.IsNaN(typed) || math.IsInf(typed, 0) || typed != math.Trunc(typed) {
				return nil, false, docProjectionUnknown(fmt.Sprintf("文档发现结果字段 %s 必须是时间字符串或整数时间戳", outputName))
			}
			return typed, true, nil
		default:
			return nil, false, docProjectionUnknown(fmt.Sprintf("文档发现结果字段 %s 必须是时间字符串或整数时间戳", outputName))
		}
	}
	return nil, false, nil
}

func docValidateLimit(rt *shortcut.RuntimeContext, flag string, maximum int) error {
	if !rt.Changed(flag) {
		return nil
	}
	value := rt.Int(flag)
	if value >= 1 && value <= maximum {
		return nil
	}
	return apperrors.NewValidation(fmt.Sprintf("--%s 必须在 1-%d 之间", flag, maximum),
		apperrors.WithSubtype(apperrors.SubtypeInvalidFlagValue),
		apperrors.WithHint(fmt.Sprintf("省略 --%s 使用服务端默认值，或传入 1-%d 之间的整数。", flag, maximum)))
}

func docValidateTimeRange(rt *shortcut.RuntimeContext, fromFlag, toFlag string) error {
	if rt.Changed(fromFlag) && rt.Int(fromFlag) < 0 {
		return apperrors.NewValidation("--"+fromFlag+" 必须是非负毫秒时间戳",
			apperrors.WithSubtype(apperrors.SubtypeInvalidFlagValue))
	}
	if rt.Changed(toFlag) && rt.Int(toFlag) < 0 {
		return apperrors.NewValidation("--"+toFlag+" 必须是非负毫秒时间戳",
			apperrors.WithSubtype(apperrors.SubtypeInvalidFlagValue))
	}
	if rt.Changed(fromFlag) && rt.Changed(toFlag) && rt.Int(fromFlag) > rt.Int(toFlag) {
		return apperrors.NewValidation("--"+fromFlag+" 不得晚于 --"+toFlag,
			apperrors.WithSubtype(apperrors.SubtypeInvalidFlagValue))
	}
	return nil
}

// ── 文档创建 / 更新 ──────────────────────────────────────────

var Copy = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+copy",
	Product:     productDoc,
	Description: "复制文档/文件到指定文件夹或知识库",
	Intent:      "当你想保留原件、在另一个文件夹或知识库里生成一份文档/文件副本（例如以某篇文档为模板另存）时使用；输入源 node 与目标 folder/workspace，会实际创建一个副本。",
	Risk:        shortcut.RiskWrite,
	Safety: contract.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "user_required", Idempotency: "unknown",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_copy",
			CanonicalPath:  "doc.shortcut_copy",
			CLIPath:        "doc +copy",
			PrimaryCLIPath: "doc +copy",
		},
		Description: "复制文档/文件到指定文件夹或知识库",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "复制文档/文件到指定文件夹或知识库",
			UseWhen:      []string{"当你想保留原件、在另一个文件夹或知识库里生成一份文档/文件副本（例如以某篇文档为模板另存）时使用；输入源 node 与目标 folder/workspace，会实际创建一个副本。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +copy --node DOC_ID --folder TARGET_FOLDER_NODE_ID"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "文档/文件 ID 或 URL", Required: true},
		{Name: "folder", Type: shortcut.FlagString, Desc: "目标文档文件夹 nodeId 或 alidocs 文件夹 URL"},
		{Name: "workspace", Type: shortcut.FlagString, Desc: "目标知识库 ID"},
	},
	Tips: []string{`dws doc +copy --node DOC_ID --folder TARGET_FOLDER_NODE_ID`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"nodeId": rt.Str("node")}
		if rt.Changed("folder") {
			params["targetFolderId"] = rt.Str("folder")
		}
		if rt.Changed("workspace") {
			params["workspaceId"] = rt.Str("workspace")
		}
		return rt.CallMCP("copy_document", params)
	},
}

var Move = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+move",
	Product:     productDoc,
	Description: "移动文档/文件到指定文件夹或知识库",
	Intent:      "当你要整理文档归属、把某篇文档/文件从当前位置挪到另一个文件夹或知识库（原位置不再保留）时使用；输入 node 与目标 folder/workspace，会实际改变文件的存放位置。",
	Risk:        shortcut.RiskWrite,
	Safety: contract.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "user_required", Idempotency: "unknown",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_move",
			CanonicalPath:  "doc.shortcut_move",
			CLIPath:        "doc +move",
			PrimaryCLIPath: "doc +move",
		},
		Description: "移动文档/文件到指定文件夹或知识库",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "移动文档/文件到指定文件夹或知识库",
			UseWhen:      []string{"当你要整理文档归属、把某篇文档/文件从当前位置挪到另一个文件夹或知识库（原位置不再保留）时使用；输入 node 与目标 folder/workspace，会实际改变文件的存放位置。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +move --node DOC_ID --folder TARGET_FOLDER_NODE_ID"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "文档/文件 ID 或 URL", Required: true},
		{Name: "folder", Type: shortcut.FlagString, Desc: "目标文档文件夹 nodeId 或 alidocs 文件夹 URL"},
		{Name: "workspace", Type: shortcut.FlagString, Desc: "目标知识库 ID"},
	},
	Tips: []string{`dws doc +move --node DOC_ID --folder TARGET_FOLDER_NODE_ID`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"nodeId": rt.Str("node")}
		if rt.Changed("folder") {
			params["targetFolderId"] = rt.Str("folder")
		}
		if rt.Changed("workspace") {
			params["workspaceId"] = rt.Str("workspace")
		}
		return rt.CallMCP("move_document", params)
	},
}

// ── 文件 / 文件夹 ────────────────────────────────────────────

// ── 块级编辑 ─────────────────────────────────────────────────

// ── 文档附件 ─────────────────────────────────────────────────

// ── 文档评论 (server: doc-comment) ───────────────────────────

var CommentList = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+comment-list",
	Product:     productComment,
	Description: "查询文档评论列表",
	Intent:      "当你想查看某篇文档上已有的评论、了解有哪些反馈或待处理意见（可按全文/划词、已解决/未解决过滤）时使用；输入 node，返回评论列表及其 commentKey 以便后续回复。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_comment_list",
			CanonicalPath:  "doc.shortcut_comment_list",
			CLIPath:        "doc +comment-list",
			PrimaryCLIPath: "doc +comment-list",
		},
		Description: "查询文档评论列表",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "查询文档评论列表",
			UseWhen:      []string{"当你想查看某篇文档上已有的评论、了解有哪些反馈或待处理意见（可按全文/划词、已解决/未解决过滤）时使用；输入 node，返回评论列表及其 commentKey 以便后续回复。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples: []string{
				"dws doc +comment-list --node DOC_ID",
				"dws doc +comment-list --node DOC_ID --type inline --resolve-status unresolved",
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "文档 ID 或 URL", Required: true},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "每页数量 (默认 50，最大 50)"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标 (上一页返回的 nextToken)"},
		{Name: "type", Type: shortcut.FlagString, Desc: "评论类型: global (全文) / inline (划词)", Enum: []string{"global", "inline"}},
		{Name: "resolve-status", Type: shortcut.FlagString, Desc: "解决状态: resolved / unresolved", Enum: []string{"resolved", "unresolved"}},
	},
	Tips: []string{`dws doc +comment-list --node DOC_ID`, `dws doc +comment-list --node DOC_ID --type inline --resolve-status unresolved`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"nodeId": rt.Str("node")}
		if rt.Changed("limit") {
			params["pageSize"] = rt.Int("limit")
		}
		if v := rt.Str("cursor"); v != "" {
			params["nextToken"] = v
		}
		if v := rt.Str("type"); v != "" {
			params["commentType"] = v
		}
		if v := rt.Str("resolve-status"); v != "" {
			params["resolveStatus"] = v
		}
		return rt.CallMCP("list_comments", params)
	},
}

var CommentCreate = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+comment-create",
	Product:     productComment,
	Description: "在文档上创建一条评论",
	Intent:      "当你想对整篇文档留一条全文评论、给出反馈或 @ 相关同事时使用；输入 node 与评论 content（可带 mention），会实际在文档上发布一条新评论。",
	Risk:        shortcut.RiskWrite,
	Safety: contract.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "user_required", Idempotency: "unknown",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_comment_create",
			CanonicalPath:  "doc.shortcut_comment_create",
			CLIPath:        "doc +comment-create",
			PrimaryCLIPath: "doc +comment-create",
		},
		Description: "在文档上创建一条评论",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "在文档上创建一条评论",
			UseWhen:      []string{"当你想对整篇文档留一条全文评论、给出反馈或 @ 相关同事时使用；输入 node 与评论 content（可带 mention），会实际在文档上发布一条新评论。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +comment-create --node DOC_ID --content \"这里需要修改\""},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "文档 ID 或 URL", Required: true},
		{Name: "content", Type: shortcut.FlagString, Desc: "评论文字内容 (纯文本)", Required: true},
		{Name: "mention", Type: shortcut.FlagStringSlice, Desc: "被 @ 的用户 uid 列表"},
	},
	Tips: []string{`dws doc +comment-create --node DOC_ID --content "这里需要修改"`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"nodeId":  rt.Str("node"),
			"content": rt.Str("content"),
		}
		if rt.Changed("mention") {
			params["mentionedUserIds"] = rt.StrSlice("mention")
		}
		return rt.CallMCP("create_comment", params)
	},
}

var CommentReply = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+comment-reply",
	Product:     productComment,
	Description: "回复文档中的一条评论",
	Intent:      "当你要针对某条已有评论进行回复、参与讨论或用表情贴图回应时使用；先从评论列表拿到 comment-key，再输入 node、comment-key 与 content（--emoji 则作为表情回复），会实际发布一条回复。",
	Risk:        shortcut.RiskWrite,
	Safety: contract.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "user_required", Idempotency: "unknown",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_comment_reply",
			CanonicalPath:  "doc.shortcut_comment_reply",
			CLIPath:        "doc +comment-reply",
			PrimaryCLIPath: "doc +comment-reply",
		},
		Description: "回复文档中的一条评论",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "回复文档中的一条评论",
			UseWhen:      []string{"当你要针对某条已有评论进行回复、参与讨论或用表情贴图回应时使用；先从评论列表拿到 comment-key，再输入 node、comment-key 与 content（--emoji 则作为表情回复），会实际发布一条回复。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +comment-reply --node DOC_ID --comment-key COMMENT_KEY --content \"同意\""},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "文档 ID 或 URL", Required: true},
		{Name: "content", Type: shortcut.FlagString, Desc: "回复文字内容 (表情回复时填表情名称)", Required: true},
		{Name: "comment-key", Type: shortcut.FlagString, Desc: "被回复评论的 commentKey (从 list/create 获取)", Required: true},
		{Name: "emoji", Type: shortcut.FlagBool, Desc: "作为表情贴图回复"},
		{Name: "mention", Type: shortcut.FlagStringSlice, Desc: "被 @ 的用户 uid 列表"},
	},
	Tips: []string{`dws doc +comment-reply --node DOC_ID --comment-key COMMENT_KEY --content "同意"`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"nodeId":          rt.Str("node"),
			"content":         rt.Str("content"),
			"replyCommentKey": rt.Str("comment-key"),
		}
		if rt.Bool("emoji") {
			params["emoji"] = true
		}
		if rt.Changed("mention") {
			params["mentionedUserIds"] = rt.StrSlice("mention")
		}
		return rt.CallMCP("reply_comment", params)
	},
}

var CommentCreateInline = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+comment-create-inline",
	Product:     productComment,
	Description: "在文档选中文本区域上创建划词评论",
	Intent:      "当你想针对文档里某段具体文字（而非整篇）留评论、做精确批注时使用；需先用 +block-list 定位块，再输入 node、block-id 及该块内的 start/end 字符偏移量，会实际在选中文本上创建划词评论。",
	Risk:        shortcut.RiskWrite,
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "文档 ID 或 URL", Required: true},
		{Name: "content", Type: shortcut.FlagString, Desc: "评论文字内容 (纯文本)", Required: true},
		{Name: "block-id", Type: shortcut.FlagString, Desc: "评论标记所在的块 ID (通过 +block-list 获取)", Required: true},
		{Name: "start", Type: shortcut.FlagInt, Desc: "块内文本起始字符偏移量 (从 0 开始)", Required: true},
		{Name: "end", Type: shortcut.FlagInt, Desc: "块内文本结束字符偏移量 (须大于 start)", Required: true},
		{Name: "selected-text", Type: shortcut.FlagString, Desc: "选中文本内容 (展示引用原文)"},
		{Name: "mention", Type: shortcut.FlagStringSlice, Desc: "被 @ 的用户 uid 列表"},
	},
	Tips: []string{`dws doc +comment-create-inline --node DOC_ID --block-id BLOCK_ID --start 0 --end 10 --content "这里需要修改"`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"nodeId":  rt.Str("node"),
			"content": rt.Str("content"),
			"blockId": rt.Str("block-id"),
			"start":   rt.Int("start"),
			"end":     rt.Int("end"),
		}
		if v := rt.Str("selected-text"); v != "" {
			params["selectedText"] = v
		}
		if rt.Changed("mention") {
			params["mentionedUserIds"] = rt.StrSlice("mention")
		}
		return rt.CallMCP("create_inline_comment", params)
	},
}

// ── 协作权限 ─────────────────────────────────────────────────

// ── 导出 ─────────────────────────────────────────────────────

var ExportSubmit = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+export-submit",
	Product:     productDoc,
	Description: "提交在线文档导出任务 (docx/markdown/pdf)，返回 jobId",
	Intent:      "当你想把在线文档导出成 docx/markdown/pdf 文件（例如离线保存或外发）时使用；这是异步任务的第一步，输入 node 与 export-format 提交导出，返回 jobId，随后用 +export-get 轮询结果。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_export_submit",
			CanonicalPath:  "doc.shortcut_export_submit",
			CLIPath:        "doc +export-submit",
			PrimaryCLIPath: "doc +export-submit",
		},
		Description: "提交在线文档导出任务 (docx/markdown/pdf)，返回 jobId",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "提交在线文档导出任务 (docx/markdown/pdf)，返回 jobId",
			UseWhen:      []string{"当你想把在线文档导出成 docx/markdown/pdf 文件（例如离线保存或外发）时使用；这是异步任务的第一步，输入 node 与 export-format 提交导出，返回 jobId，随后用 +export-get 轮询结果。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +export-submit --node DOC_ID --export-format markdown"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "要导出的文档 ID 或 URL", Required: true},
		{Name: "export-format", Type: shortcut.FlagString, Default: "docx", Desc: "导出格式", Enum: []string{"docx", "markdown", "pdf"}},
	},
	Tips: []string{`dws doc +export-submit --node DOC_ID --export-format markdown`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		format := rt.Str("export-format")
		if format == "" {
			format = "docx"
		}
		return rt.CallMCP("submit_export_job", map[string]any{
			"nodeId":       rt.Str("node"),
			"exportFormat": format,
		})
	},
}

var ExportGet = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+export-get",
	Product:     productDoc,
	Description: "根据 jobId 查询文档导出任务结果",
	Intent:      "当你已用 +export-submit 提交了导出任务、想查询它是否完成并拿到导出文件的下载链接时使用；输入上一步返回的 job-id，返回任务状态与结果。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_export_get",
			CanonicalPath:  "doc.shortcut_export_get",
			CLIPath:        "doc +export-get",
			PrimaryCLIPath: "doc +export-get",
		},
		Description: "根据 jobId 查询文档导出任务结果",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "根据 jobId 查询文档导出任务结果",
			UseWhen:      []string{"当你已用 +export-submit 提交了导出任务、想查询它是否完成并拿到导出文件的下载链接时使用；输入上一步返回的 job-id，返回任务状态与结果。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +export-get --job-id JOB_ID"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "job-id", Type: shortcut.FlagString, Desc: "导出任务 ID", Required: true},
	},
	Tips: []string{`dws doc +export-get --job-id JOB_ID`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		return rt.CallMCP("query_export_job", map[string]any{"jobId": rt.Str("job-id")})
	},
}

// ── 历史版本 (server: doc) ───────────────────────────────────

var VersionSave = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+version-save",
	Product:     productDoc,
	Description: "手动保存文档版本快照",
	Intent:      "当你在做重大改动前后、想手动打一个可回滚的版本存档点时使用；输入 node，会实际为该文档保存一个当前内容的历史版本快照。",
	Risk:        shortcut.RiskWrite,
	Safety: contract.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "user_required", Idempotency: "unknown",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_version_save",
			CanonicalPath:  "doc.shortcut_version_save",
			CLIPath:        "doc +version-save",
			PrimaryCLIPath: "doc +version-save",
		},
		Description: "手动保存文档版本快照",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "手动保存文档版本快照",
			UseWhen:      []string{"当你在做重大改动前后、想手动打一个可回滚的版本存档点时使用；输入 node，会实际为该文档保存一个当前内容的历史版本快照。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +version-save --node DOC_ID"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "文档 ID 或 URL", Required: true},
	},
	Tips: []string{`dws doc +version-save --node DOC_ID`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		return rt.CallMCP("save_doc_version", map[string]any{"nodeId": rt.Str("node")})
	},
}

var VersionList = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+version-list",
	Product:     productDoc,
	Description: "查看文档历史版本列表",
	Intent:      "当你想查看某篇文档有哪些历史版本、以便挑一个版本号用于回滚时使用；输入 node，返回历史版本列表及其版本号。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_version_list",
			CanonicalPath:  "doc.shortcut_version_list",
			CLIPath:        "doc +version-list",
			PrimaryCLIPath: "doc +version-list",
		},
		Description: "查看文档历史版本列表",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "查看文档历史版本列表",
			UseWhen:      []string{"当你想查看某篇文档有哪些历史版本、以便挑一个版本号用于回滚时使用；输入 node，返回历史版本列表及其版本号。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +version-list --node DOC_ID"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "文档 ID 或 URL", Required: true},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "返回版本数量上限"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标"},
	},
	Tips: []string{`dws doc +version-list --node DOC_ID`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"nodeId": rt.Str("node")}
		if rt.Changed("limit") {
			params["maxResults"] = rt.Int("limit")
		}
		if v := rt.Str("cursor"); v != "" {
			params["nextCursor"] = v
		}
		return rt.CallMCP("list_doc_versions", params)
	},
}

var VersionRevert = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+version-revert",
	Product:     productDoc,
	Description: "回滚文档到指定历史版本",
	Intent:      "当文档被误改、你想把它整体恢复到某个历史版本时使用；先用 +version-list 找到目标版本号，再输入 node 与 version，会实际把文档内容覆盖回该版本，属于高风险写操作，需谨慎确认。",
	Risk:        shortcut.RiskHighWrite,
	Safety: contract.SafetySpec{
		Effect: "destructive", Risk: "high",
		Confirmation: "user_required", Idempotency: "unknown",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_version_revert",
			CanonicalPath:  "doc.shortcut_version_revert",
			CLIPath:        "doc +version-revert",
			PrimaryCLIPath: "doc +version-revert",
		},
		Description: "回滚文档到指定历史版本",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "回滚文档到指定历史版本",
			UseWhen:      []string{"当文档被误改、你想把它整体恢复到某个历史版本时使用；先用 +version-list 找到目标版本号，再输入 node 与 version，会实际把文档内容覆盖回该版本，属于高风险写操作，需谨慎确认。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +version-revert --node DOC_ID --version 3"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "node", Type: shortcut.FlagString, Desc: "文档 ID 或 URL", Required: true},
		{Name: "version", Type: shortcut.FlagInt, Desc: "目标版本号 (从 +version-list 获取)", Required: true},
	},
	Tips: []string{`dws doc +version-revert --node DOC_ID --version 3 --yes`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		return rt.CallMCP("revert_doc_version", map[string]any{
			"nodeId":  rt.Str("node"),
			"version": rt.Int("version"),
		})
	},
}

// ── 模板 (server: doc) ───────────────────────────────────────

var TemplateList = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+template-list",
	Product:     productDoc,
	Description: "获取文档模板列表",
	Intent:      "当你想基于模板新建文档、需要先浏览可用的模板（自己的 MY 或公共 PUBLIC）并拿到 templateId 时使用；返回模板列表，随后可配合 +template-apply 套用。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_template_list",
			CanonicalPath:  "doc.shortcut_template_list",
			CLIPath:        "doc +template-list",
			PrimaryCLIPath: "doc +template-list",
		},
		Description: "获取文档模板列表",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "获取文档模板列表",
			UseWhen:      []string{"当你想基于模板新建文档、需要先浏览可用的模板（自己的 MY 或公共 PUBLIC）并拿到 templateId 时使用；返回模板列表，随后可配合 +template-apply 套用。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +template-list --source PUBLIC"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "source", Type: shortcut.FlagString, Desc: "模板来源: MY / PUBLIC (默认 MY)", Enum: []string{"MY", "PUBLIC"}},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "返回数量上限"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标"},
	},
	Tips: []string{`dws doc +template-list --source PUBLIC`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{}
		if v := rt.Str("source"); v != "" {
			params["templateSource"] = v
		}
		if rt.Changed("limit") {
			params["maxResults"] = rt.Int("limit")
		}
		if v := rt.Str("cursor"); v != "" {
			params["nextCursor"] = v
		}
		return rt.CallMCP("list_doc_templates", params)
	},
}

var TemplateSearch = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+template-search",
	Product:     productDoc,
	Description: "根据关键词搜索文档模板",
	Intent:      "当模板较多、你想按关键词（如“周报”“合同”）快速找到合适的模板并拿到 templateId 时使用；输入 query，返回匹配的模板列表，随后可配合 +template-apply 套用。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "doc",
			Name:           "shortcut_template_search",
			CanonicalPath:  "doc.shortcut_template_search",
			CLIPath:        "doc +template-search",
			PrimaryCLIPath: "doc +template-search",
		},
		Description: "根据关键词搜索文档模板",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "根据关键词搜索文档模板",
			UseWhen:      []string{"当模板较多、你想按关键词（如“周报”“合同”）快速找到合适的模板并拿到 templateId 时使用；输入 query，返回匹配的模板列表，随后可配合 +template-apply 套用。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws doc +template-search --query \"周报\""},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "query", Type: shortcut.FlagString, Desc: "搜索关键词", Required: true},
		{Name: "source", Type: shortcut.FlagString, Desc: "模板来源: MY / PUBLIC (默认 MY)", Enum: []string{"MY", "PUBLIC"}},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "返回数量上限"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标"},
	},
	Tips: []string{`dws doc +template-search --query "周报"`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"searchName": rt.Str("query")}
		if v := rt.Str("source"); v != "" {
			params["templateSource"] = v
		}
		if rt.Changed("limit") {
			params["maxResults"] = rt.Int("limit")
		}
		if v := rt.Str("cursor"); v != "" {
			params["nextCursor"] = v
		}
		return rt.CallMCP("search_doc_templates", params)
	},
}

var TemplateApply = shortcut.Shortcut{
	Service:     "doc",
	Command:     "+template-apply",
	Product:     productDoc,
	Description: "使用指定模板创建新文档",
	Intent:      "当你已选定某个模板、想据此快速生成一篇带预设结构的新文档时使用；输入 template-id（可选 name/folder/workspace），会实际按模板创建一篇新文档并返回其 ID。",
	Risk:        shortcut.RiskWrite,
	Flags: []shortcut.Flag{
		{Name: "template-id", Type: shortcut.FlagString, Desc: "模板 ID", Required: true},
		{Name: "name", Type: shortcut.FlagString, Desc: "新文档名称 (可选)"},
		{Name: "folder", Type: shortcut.FlagString, Desc: "目标文件夹 ID (可选)"},
		{Name: "workspace", Type: shortcut.FlagString, Desc: "知识库 ID (可选)"},
	},
	Tips: []string{`dws doc +template-apply --template-id TPL_ID --name "我的周报"`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"templateId": rt.Str("template-id")}
		if v := rt.Str("name"); v != "" {
			params["name"] = v
		}
		if rt.Changed("folder") {
			params["folderId"] = rt.Str("folder")
		}
		if rt.Changed("workspace") {
			params["workspaceId"] = rt.Str("workspace")
		}
		return rt.CallMCP("apply_doc_template", params)
	},
}

func init() {
	// Expert/recovery leaves remain callable without entering Agent discovery.
	CommentCreateInline.Contract = corecmd.ContractDecl{}
	TemplateApply.Contract = corecmd.ContractDecl{}
	canonicalizeHistoryShortcuts()
	canonicalizeCommentShortcuts()
	// The canonical history-revert declaration is assembled here before this
	// init registers it. Create/checkpoint-update are configured in their own
	// registration init so the registry never captures an earlier legacy copy.
	VersionRevert.Contract.DryRun = &contract.DryRunSpec{PreviewKind: "plan", RemoteReads: true}
	VersionRevert.Contract.Result = docOperationResultSpec()
	VersionRevert.OutputRollout = output.RolloutUnifiedActive
	shortcut.Register(
		Search,
		List,
		Copy,
		Move,
		CommentList,
		CommentCreate,
		CommentReply,
		CommentCreateInline,
		ExportSubmit,
		ExportGet,
		legacyVersionSave,
		legacyVersionList,
		legacyVersionRevert,
		VersionSave,
		VersionList,
		VersionRevert,
		TemplateList,
		TemplateSearch,
		TemplateApply,
	)
}
