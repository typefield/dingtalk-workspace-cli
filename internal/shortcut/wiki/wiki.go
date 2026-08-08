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

// Package wiki declares high-fidelity shortcuts for the DingTalk wiki
// (knowledge base) service: space management, member management and node
// management. Tool names and parameters mirror internal/helpers/wiki.go.
package wiki

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

// ── space (知识库) ────────────────────────────────────────────

// SpaceCreate → create_wikiSpace
// SpaceGet → get_wikiSpace
// SpaceList → list_wikiSpaces
var SpaceList = shortcut.Shortcut{
	Service:     "wiki",
	Command:     "+space-list",
	Product:     "wiki",
	Description: "列出组织 / 个人知识库",
	Intent:      "当你想浏览自己有权限访问的知识库、拿到目标知识库的 workspaceId 却不确定具体名称时使用；可按类型（组织知识库或我的知识库）分页列出，返回知识库列表，是定位知识库的常用入口。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "wiki",
			Name:           "shortcut_space_list",
			CanonicalPath:  "wiki.shortcut_space_list",
			CLIPath:        "wiki +space-list",
			PrimaryCLIPath: "wiki +space-list",
		},
		Description: "列出组织 / 个人知识库",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns type validation, cursor pagination and stable wiki-space projection; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "列出组织 / 个人知识库",
			UseWhen:      []string{"当你想浏览自己有权限访问的知识库、拿到目标知识库的 workspaceId 却不确定具体名称时使用；可按类型（组织知识库或我的知识库）分页列出，返回知识库列表，是定位知识库的常用入口。"},
			AvoidWhen:    []string{"已经知道知识库名称并需要精确定位时，优先使用 wiki +resolve-space；需要原始响应时改用对应原子查询"},
			Examples:     []string{"dws wiki +space-list", "dws wiki +space-list --type myWikiSpace"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "type", Type: shortcut.FlagString, Default: "orgWikiSpace", Desc: "知识库类型: orgWikiSpace(默认) / myWikiSpace", Enum: []string{"orgWikiSpace", "myWikiSpace"}},
		{Name: "limit", Type: shortcut.FlagString, Desc: "每页数量 1-50 (默认 20)"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标 (首页留空)"},
	},
	Tips: []string{
		`dws wiki +space-list`,
		`dws wiki +space-list --type myWikiSpace`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{}
		if rt.Changed("type") {
			params["wikiSpaceType"] = rt.Str("type")
		}
		if rt.Changed("limit") {
			params["pageSize"] = rt.Str("limit")
		}
		if rt.Changed("cursor") {
			params["pageToken"] = rt.Str("cursor")
		}
		data, err := rt.CallMCPData("wiki", "list_wikiSpaces", params)
		if err != nil {
			return err
		}
		spaces := spaceListProject(data)
		return rt.Output(map[string]any{"count": len(spaces), "spaces": spaces})
	},
}

// spaceListProject reshapes list_wikiSpaces into a clean space list
// ({workspaceId, name, description, createTime}) — output-projection fidelity
// for clean output. The list container and per-item field names are probed defensively
// across candidate keys, so an unrecognized shape yields an empty list.
func spaceListProject(data map[string]any) []map[string]any {
	raw := wikiSpaceRawList(data)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := map[string]any{}
		if v := wikiSpaceFirst(m, "workspaceId", "workspace_id", "spaceId", "space_id", "id"); v != nil {
			row["workspaceId"] = v
		}
		if v := wikiSpaceFirst(m, "name", "title", "spaceName"); v != nil {
			row["name"] = v
		}
		if v := wikiSpaceFirst(m, "description", "desc"); v != nil {
			row["description"] = v
		}
		if v := wikiSpaceFirst(m, "createTime", "create_time", "gmtCreate", "createdAt"); v != nil {
			row["createTime"] = v
		}
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	return out
}

// wikiSpaceRawList locates the space array across candidate container keys,
// tolerating a nested {result|data:{list|items|spaces}} wrapper.
func wikiSpaceRawList(data map[string]any) []any {
	// list_wikiSpaces / search_wikiSpaces nest the space list under
	// result.wikiSpaces (or a top-level wikiSpaces once unwrapped); "wikiSpaces"
	// MUST be probed or +space-list / +space-search silently return empty.
	for _, k := range []string{"result", "data", "list", "items", "wikiSpaces", "spaces", "workspaces"} {
		if arr, ok := data[k].([]any); ok {
			return arr
		}
		if inner, ok := data[k].(map[string]any); ok {
			for _, ik := range []string{"list", "items", "wikiSpaces", "spaces", "workspaces", "result", "data"} {
				if arr, ok := inner[ik].([]any); ok {
					return arr
				}
			}
		}
	}
	return nil
}

// wikiSpaceFirst returns the first present value among candidate keys.
func wikiSpaceFirst(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

// SpaceSearch → search_wikiSpaces
var SpaceSearch = shortcut.Shortcut{
	Service:     "wiki",
	Command:     "+space-search",
	Product:     "wiki",
	Description: "搜索知识库",
	Intent:      "当你只记得知识库名称的部分关键词、想快速按名称定位某个知识库时使用；输入关键词返回匹配的知识库列表，比逐页 +space-list 更快找到目标 workspaceId。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "wiki",
			Name:           "shortcut_space_search",
			CanonicalPath:  "wiki.shortcut_space_search",
			CLIPath:        "wiki +space-search",
			PrimaryCLIPath: "wiki +space-search",
		},
		Description: "搜索知识库",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "搜索知识库",
			UseWhen:      []string{"当你只记得知识库名称的部分关键词、想快速按名称定位某个知识库时使用；输入关键词返回匹配的知识库列表，比逐页 +space-list 更快找到目标 workspaceId。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws wiki +space-search --query \"产品文档\""},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "query", Type: shortcut.FlagString, Desc: "搜索关键词", Required: true},
		{Name: "limit", Type: shortcut.FlagString, Desc: "返回数量 1-20 (默认 10)"},
	},
	Tips: []string{`dws wiki +space-search --query "产品文档"`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"keyword": rt.Str("query")}
		if rt.Changed("limit") {
			params["pageSize"] = rt.Str("limit")
		}
		data, err := rt.CallMCPData("wiki", "search_wikiSpaces", params)
		if err != nil {
			return err
		}
		spaces := spaceListProject(data)
		return rt.Output(map[string]any{"count": len(spaces), "spaces": spaces})
	},
}

// SpaceDelete → delete_wikiSpace
// ── member (知识库成员) ───────────────────────────────────────

// MemberAdd → add_member
// MemberUpdate → update_member
// MemberList → list_member
// MemberRemove → remove_member
// ── node (知识库节点，路由到 doc MCP server) ──────────────────

// NodeList → list_nodes (doc)
var NodeList = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "wiki",
	Command:       "+node-list",
	Product:       "doc",
	Description:   "列出知识库节点",
	Intent:        "当你要浏览某个知识库的目录结构、查看某文件夹下有哪些文档/子文件夹并拿到它们的 nodeId 时使用；输入 workspace（可选父节点 folder），分页返回该层级的节点列表，是逐层进入知识库定位文档的常用方式。",
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "wiki",
			Name:           "shortcut_node_list",
			CanonicalPath:  "wiki.shortcut_node_list",
			CLIPath:        "wiki +node-list",
			PrimaryCLIPath: "wiki +node-list",
		},
		Description: "列出知识库节点",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the CLI routes the Wiki-facing command to doc/list_nodes and owns validation, pagination truth, output projection, and safety; the complete command contract is not represented by one pinned Wiki interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "列出知识库节点",
			UseWhen:      []string{"当你要浏览某个知识库的目录结构、查看某文件夹下有哪些文档/子文件夹并拿到它们的 nodeId 时使用；输入 workspace（可选父节点 folder），分页返回该层级的节点列表，是逐层进入知识库定位文档的常用方式。"},
			AvoidWhen:    []string{"只知道知识库名称时先用 wiki +resolve-space；要按关键词跨目录搜索文档时使用 wiki node search"},
			Examples:     []string{`dws wiki +node-list --workspace <workspaceId> --folder <parentNodeId>`},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "workspace", Type: shortcut.FlagString, Desc: "知识库 ID", Required: true},
		{Name: "folder", Type: shortcut.FlagString, Desc: "父节点 nodeId (不传则列出根目录)"},
		{Name: "limit", Type: shortcut.FlagInt, Desc: "每页数量 (默认 50，最大 50)"},
		{Name: "cursor", Type: shortcut.FlagString, Desc: "分页游标"},
	},
	Tips: []string{`dws wiki +node-list --workspace <workspaceId> --folder <parentNodeId>`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{"workspaceId": rt.Str("workspace")}
		if rt.Changed("folder") {
			params["folderId"] = rt.Str("folder")
		}
		if rt.Changed("limit") {
			params["pageSize"] = rt.Int("limit")
		}
		if rt.Changed("cursor") {
			params["pageToken"] = rt.Str("cursor")
		}
		data, err := rt.CallMCPData("doc", "list_nodes", params)
		if err != nil {
			return err
		}
		nodes, projectionKnown := nodeListProjectWithStatus(data)
		if !projectionKnown {
			return apperrors.NewAPI("知识库节点响应结构无法识别，拒绝将未知数据投影为空列表",
				apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
				apperrors.WithHint("请使用 --verbose 或 DWS_DUMP_RAW=1 记录脱敏响应形状后提交问题"))
		}
		payload, result, err := nodeListResult(data, nodes)
		if err != nil {
			return err
		}
		return rt.OutputResult(payload, result)
	},
}

// nodeListProject reshapes list_nodes into a clean node list (name/nodeId/type).
// Execute uses nodeListProjectWithStatus so an unrecognized response fails closed
// instead of being misreported as a valid empty list.
func nodeListProject(data map[string]any) []map[string]any {
	out, _ := nodeListProjectWithStatus(data)
	return out
}

func nodeListProjectWithStatus(data map[string]any) ([]map[string]any, bool) {
	raw, known := nodeListRawList(data)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := map[string]any{}
		if v := nodeListFirst(m, "name", "title", "nodeName"); v != nil {
			row["name"] = v
		}
		if v := nodeListFirst(m, "nodeId", "node_id", "id", "uuid", "dentryUuid"); v != nil {
			row["nodeId"] = v
		}
		if v := nodeListFirst(m, "type", "nodeType", "docType", "fileType"); v != nil {
			row["type"] = v
		}
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	return out, known
}

// nodeListRawList locates the node array across candidate container keys,
// tolerating a nested {result|data:{list|items|nodes}} wrapper.
func nodeListRawList(data map[string]any) ([]any, bool) {
	for _, k := range []string{"result", "data", "list", "items", "nodes"} {
		if arr, ok := data[k].([]any); ok {
			return arr, true
		}
		if inner, ok := data[k].(map[string]any); ok {
			for _, ik := range []string{"list", "items", "nodes", "result", "data"} {
				if arr, ok := inner[ik].([]any); ok {
					return arr, true
				}
			}
		}
	}
	return nil, false
}

// nodeListResult keeps established data fields for humans and legacy consumers,
// while putting count and endpoint pagination into the framework-owned meta
// channel. No pagination evidence means precisely that: the endpoint state is
// unknown, never guessed to be complete.
func nodeListResult(data map[string]any, nodes []map[string]any) (map[string]any, output.CommandResult, error) {
	payload := map[string]any{"count": len(nodes), "nodes": nodes}
	page, known, err := nodeListPagination(data)
	if err != nil {
		return nil, nil, err
	}
	meta := &output.Meta{Count: output.NewCount(len(nodes))}
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

// nodeListPagination accepts only an authoritative hasMore/nextCursor pair.
// Duplicate evidence in result/data wrappers must agree; a continuation token
// without hasMore, or a terminal page retaining a non-terminal cursor, is an
// upstream inconsistency rather than permission to fabricate a complete list.
func nodeListPagination(data map[string]any) (*output.Pagination, bool, error) {
	var selected *output.Pagination
	for _, scope := range nodeListPaginationScopes(data) {
		page, present, err := nodeListPaginationFromScope(scope)
		if err != nil {
			return nil, false, err
		}
		if !present {
			continue
		}
		if selected != nil && (selected.EndpointExhausted != page.EndpointExhausted || selected.NextToken != page.NextToken) {
			return nil, false, nodeListPaginationError("知识库节点分页字段在嵌套容器间互相矛盾")
		}
		selected = page
	}
	if selected == nil {
		return nil, false, nil
	}
	return selected, true, nil
}

func nodeListPaginationFromScope(scope map[string]any) (*output.Pagination, bool, error) {
	if scope == nil {
		return nil, false, nil
	}
	rawMore, hasMore := nodeListFirstPresent(scope, "hasMore", "has_more")
	rawCursor, hasCursor := nodeListFirstPresent(scope, "nextCursor", "next_cursor", "nextToken", "next_token", "pageToken", "page_token")
	if !hasMore && !hasCursor {
		return nil, false, nil
	}
	if !hasMore {
		return nil, false, nodeListPaginationError("知识库节点响应携带续页游标但缺少 hasMore")
	}
	more, ok := rawMore.(bool)
	if !ok {
		return nil, false, nodeListPaginationError("知识库节点的 hasMore 必须是布尔值")
	}
	cursor, err := nodeListPaginationToken(rawCursor, hasCursor)
	if err != nil {
		return nil, false, err
	}
	page, err := output.NewPagination(!more, cursor)
	if err != nil {
		return nil, false, nodeListPaginationError(err.Error())
	}
	return page, true, nil
}

func nodeListFirstPresent(scope map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := scope[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func nodeListPaginationToken(value any, present bool) (string, error) {
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
	case int:
		if typed == 0 {
			return "", nil
		}
		return strconv.Itoa(typed), nil
	case int64:
		if typed == 0 {
			return "", nil
		}
		return strconv.FormatInt(typed, 10), nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || typed != math.Trunc(typed) {
			return "", nodeListPaginationError("知识库节点的 nextCursor 必须是字符串或整数")
		}
		if typed == 0 {
			return "", nil
		}
		return strconv.FormatInt(int64(typed), 10), nil
	default:
		return "", nodeListPaginationError(fmt.Sprintf("知识库节点的 nextCursor 类型无效: %T", value))
	}
}

func nodeListPaginationError(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypePaginationInconsistent),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

func nodeListPaginationScopes(data map[string]any) []map[string]any {
	scopes := make([]map[string]any, 0, 3)
	if data == nil {
		return scopes
	}
	scopes = append(scopes, data)
	for _, key := range []string{"result", "data"} {
		if inner, ok := data[key].(map[string]any); ok {
			scopes = append(scopes, inner)
		}
	}
	return append(scopes, data)
}

// nodeListFirst returns the first present value among candidate keys.
func nodeListFirst(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

// NodeCreate → create_file (doc)
// NodeCopy → copy_document (doc)
var NodeCopy = shortcut.Shortcut{
	Service:     "wiki",
	Command:     "+node-copy",
	Product:     "doc",
	Description: "复制知识库节点",
	Intent:      "当你想基于已有文档/文件夹快速生成一份副本（如用模板起草新文档、留档备份）时使用；指定源 node 和目标 folder，会实际在知识库中复制出一个新节点，原节点保持不变。",
	Risk:        shortcut.RiskWrite,
	Flags: []shortcut.Flag{
		{Name: "workspace", Type: shortcut.FlagString, Desc: "知识库 ID", Required: true},
		{Name: "node", Type: shortcut.FlagString, Desc: "源节点 ID", Required: true},
		{Name: "folder", Type: shortcut.FlagString, Desc: "目标文件夹 nodeId (不传则复制到根目录)"},
	},
	Tips: []string{`dws wiki +node-copy --workspace <workspaceId> --node <nodeId> --folder <targetFolderId>`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"nodeId":      rt.Str("node"),
			"workspaceId": rt.Str("workspace"),
		}
		if rt.Changed("folder") {
			params["targetFolderId"] = rt.Str("folder")
		}
		return rt.CallMCP("copy_document", params)
	},
}

// NodeMove → move_document (doc)
var NodeMove = shortcut.Shortcut{
	Service:     "wiki",
	Command:     "+node-move",
	Product:     "doc",
	Description: "移动知识库节点",
	Intent:      "当你要重新整理知识库目录、把某个文档或文件夹从当前位置挪到另一个文件夹（或根目录）下时使用；指定源 node 和目标 folder，会实际改变该节点在知识库中的所属位置。",
	Risk:        shortcut.RiskWrite,
	Flags: []shortcut.Flag{
		{Name: "workspace", Type: shortcut.FlagString, Desc: "知识库 ID", Required: true},
		{Name: "node", Type: shortcut.FlagString, Desc: "源节点 ID", Required: true},
		{Name: "folder", Type: shortcut.FlagString, Desc: "目标文件夹 nodeId (不传则移动到根目录)"},
	},
	Tips: []string{`dws wiki +node-move --workspace <workspaceId> --node <nodeId> --folder <targetFolderId>`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"nodeId":      rt.Str("node"),
			"workspaceId": rt.Str("workspace"),
		}
		if rt.Changed("folder") {
			params["targetFolderId"] = rt.Str("folder")
		}
		return rt.CallMCP("move_document", params)
	},
}

// NodeDelete → delete_document (doc)
// NodeSearch → search_documents (doc)
func init() {
	shortcut.Register(
		SpaceList,
		SpaceSearch,
		NodeList,
		NodeCopy,
		NodeMove,
	)
}
