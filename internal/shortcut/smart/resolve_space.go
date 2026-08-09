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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

const (
	resolveSpacePageSize = "50"
	resolveSpaceMaxPages = 100
)

// ResolveSpace resolves one accessible organization Wiki space by exhausting
// the authoritative list endpoint and matching names locally. The former
// implementation used search_wikiSpaces, whose live response exposes no
// continuation or coverage fact; treating one search hit as globally unique
// was therefore not defensible. Exhausting list_wikiSpaces gives the command a
// concrete endpoint boundary before it publishes resolved=true.
//
//	dws wiki +resolve-space --name 产品文档
var ResolveSpace = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "wiki",
	Command:       "+resolve-space",
	Product:       "wiki",
	Description:   "按名称从完整组织知识库目录解析出唯一 spaceId（只读）",
	Intent: "当你只知道某个组织知识空间的名称（或名称里的关键词）、想把它解析成可直接用于后续工具的 spaceId 时使用；" +
		"CLI 会逐页读取当前身份可访问的组织知识库，只有观察到目录 endpoint 耗尽后才在本地按名称筛选。" +
		"如果完整目录中只命中一个知识空间就返回它的 spaceId；多个候选会全部列出让你消歧，绝不替你猜；一个都没有则提示未找到。" +
		"这是纯只读操作，不修改任何知识空间，也不把搜索索引的一页结果扩大为全局唯一。",
	Risk: shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "wiki",
			Name:           "shortcut_resolve_space",
			CanonicalPath:  "wiki.shortcut_resolve_space",
			CLIPath:        "wiki +resolve-space",
			PrimaryCLIPath: "wiki +resolve-space",
		},
		Description: "按名称从完整组织知识库目录解析出唯一 spaceId（只读）",
		Result: &contract.ResultSpec{
			Outcomes: []contract.ResultOutcome{
				contract.ResultOutcomeSuccess,
				contract.ResultOutcomeFailure,
			},
			DataSchema: json.RawMessage(`{"type":"object","properties":{"resolved":{"type":"boolean"},"count":{"type":"integer","minimum":1},"candidates":{"type":"array","minItems":1,"items":{"type":"object","properties":{"spaceId":{"type":"string"},"name":{"type":"string"}},"required":["spaceId","name"],"additionalProperties":false}}},"required":["resolved","count","candidates"],"additionalProperties":false}`),
			NDJSON: &contract.ResultNDJSONSpec{
				RecordPath:   "candidates",
				RecordSchema: json.RawMessage(`{"type":"object","properties":{"spaceId":{"type":"string"},"name":{"type":"string"}},"required":["spaceId","name"],"additionalProperties":false}`),
			},
		},
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed read-only resolver: the CLI exhausts wiki/list_wikiSpaces with a bounded pagination ledger, validates the reviewed response shape, and resolves names only after endpoint exhaustion.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "按名称从完整组织知识库目录解析出唯一 spaceId（只读）",
			UseWhen: []string{
				"当你只知道某个组织知识空间的名称（或名称里的关键词）、想把它解析成可直接用于后续工具的 spaceId 时使用；CLI 会逐页读取当前身份可访问的组织知识库，只有观察到目录 endpoint 耗尽后才在本地按名称筛选。如果完整目录中只命中一个知识空间就返回它的 spaceId；多个候选会全部列出让你消歧，绝不替你猜；一个都没有则提示未找到。这是纯只读操作，不修改任何知识空间，也不把搜索索引的一页结果扩大为全局唯一。",
			},
			AvoidWhen: []string{
				"已经知道 spaceId 时直接使用它；需要个人知识库时使用 wiki +space-list --type myWikiSpace；需要原始服务端搜索响应时使用 wiki space search",
			},
			Examples: []string{"dws wiki +resolve-space --name 产品文档 --format json"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "name", Type: shortcut.FlagString, Desc: "要搜索的组织知识空间名称关键词（必填）", Required: true},
	},
	Tips: []string{
		`dws wiki +resolve-space --name 产品文档 --format json`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		query := strings.TrimSpace(rt.Str("name"))
		if query == "" {
			return apperrors.NewValidation("--name 不能为空或只包含空白字符")
		}

		candidates, ledger, err := collectResolveSpaceCandidates(rt, query)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			return apperrors.NewValidation("完整组织知识库目录中没有找到名称包含 " + query + " 的知识空间")
		}

		legacy := map[string]any{
			"resolved":   len(candidates) == 1,
			"count":      len(candidates),
			"candidates": candidates,
		}
		if len(candidates) == 1 {
			legacy = map[string]any{
				"resolved": true,
				"spaceId":  candidates[0]["spaceId"],
				"name":     candidates[0]["name"],
			}
		}
		return rt.OutputResult(legacy, resolveSpaceSuccess(candidates, ledger))
	},
}

// collectResolveSpaceCandidates exhausts the reviewed organization-space list
// endpoint. Every successful page is recorded in the shared PageLedger so a
// missing, contradictory, repeated, or non-advancing cursor can never become
// a successful unique resolution.
func collectResolveSpaceCandidates(rt *shortcut.RuntimeContext, query string) ([]map[string]any, *output.PageLedger, error) {
	ledger, err := output.NewPageLedger(resolveSpaceMaxPages)
	if err != nil {
		return nil, nil, err
	}
	wanted := strings.ToLower(strings.TrimSpace(query))
	candidates := make([]map[string]any, 0)
	seenIDs := make(map[string]struct{})
	cursor := ""

	for pageNumber := 0; pageNumber < resolveSpaceMaxPages; pageNumber++ {
		params := map[string]any{
			"wikiSpaceType": "orgWikiSpace",
			"pageSize":      resolveSpacePageSize,
		}
		if cursor != "" {
			params["pageToken"] = cursor
		}
		data, callErr := rt.CallMCPData("wiki", "list_wikiSpaces", params)
		if callErr != nil {
			return nil, ledger, callErr
		}
		page, projectErr := projectResolveSpacePage(data)
		if projectErr != nil {
			return nil, ledger, projectErr
		}
		hasMore := page.hasMore
		if observeErr := ledger.ObservePage(output.PageEvidence{
			Cursor:    cursor,
			NextToken: page.nextToken,
			HasMore:   &hasMore,
			Items:     len(page.spaces),
		}); observeErr != nil {
			return nil, ledger, resolveSpacePaginationError(observeErr.Error(), map[string]any{
				"pages_scanned": ledger.Pages(),
			})
		}
		for _, space := range page.spaces {
			spaceID := space["spaceId"].(string)
			if _, duplicate := seenIDs[spaceID]; duplicate {
				return nil, ledger, resolveSpaceProjectionError("组织知识库目录包含重复 workspaceId")
			}
			seenIDs[spaceID] = struct{}{}
			if strings.Contains(strings.ToLower(space["name"].(string)), wanted) {
				candidates = append(candidates, space)
			}
		}
		if !page.hasMore {
			return candidates, ledger, nil
		}
		cursor = page.nextToken
	}

	return nil, ledger, resolveSpacePaginationError("组织知识库目录超过安全分页上限，无法证明唯一性", map[string]any{
		"max_pages":     resolveSpaceMaxPages,
		"pages_scanned": ledger.Pages(),
	})
}

type resolveSpacePage struct {
	spaces    []map[string]any
	hasMore   bool
	nextToken string
}

// projectResolveSpacePage accepts only the live-reviewed list_wikiSpaces
// response. In particular, workspaceId is the stable service field; generic
// id/spaceId aliases are not accepted because they can represent another
// resource type.
func projectResolveSpacePage(data map[string]any) (resolveSpacePage, error) {
	if data == nil {
		return resolveSpacePage{}, resolveSpaceProjectionError("组织知识库列表响应不是 JSON 对象")
	}
	if err := rejectUnknownKeys(data, "组织知识库列表顶层", "success", "wikiSpaces", "hasMore", "nextPageToken", "logId"); err != nil {
		return resolveSpacePage{}, resolveSpaceProjectionError(err.Error())
	}
	success, ok := data["success"].(bool)
	if !ok {
		return resolveSpacePage{}, resolveSpaceProjectionError("组织知识库列表 success 缺失或不是布尔值")
	}
	if !success {
		return resolveSpacePage{}, resolveSpaceProjectionError("组织知识库列表业务结果明确失败")
	}
	if logID, exists := data["logId"]; exists {
		if _, ok := logID.(string); !ok {
			return resolveSpacePage{}, resolveSpaceProjectionError("组织知识库列表 logId 不是字符串")
		}
	}
	rows, ok := data["wikiSpaces"].([]any)
	if !ok {
		return resolveSpacePage{}, resolveSpaceProjectionError("组织知识库列表 wikiSpaces 缺失或不是数组")
	}
	hasMore, ok := data["hasMore"].(bool)
	if !ok {
		return resolveSpacePage{}, resolveSpacePaginationError("组织知识库列表 hasMore 缺失或不是布尔值", nil)
	}
	nextToken := ""
	if raw, exists := data["nextPageToken"]; exists {
		value, ok := raw.(string)
		if !ok {
			return resolveSpacePage{}, resolveSpacePaginationError("组织知识库列表 nextPageToken 不是字符串", nil)
		}
		nextToken = strings.TrimSpace(value)
	}
	if hasMore && nextToken == "" {
		return resolveSpacePage{}, resolveSpacePaginationError("组织知识库列表仍有下一页但缺少 nextPageToken", map[string]any{"projected_count": len(rows)})
	}
	if !hasMore && nextToken != "" {
		return resolveSpacePage{}, resolveSpacePaginationError("组织知识库列表已终结但仍携带 nextPageToken", map[string]any{"projected_count": len(rows)})
	}

	spaces := make([]map[string]any, 0, len(rows))
	for index, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return resolveSpacePage{}, resolveSpaceProjectionError(fmt.Sprintf("wikiSpaces[%d] 不是对象", index))
		}
		if err := rejectUnknownKeys(row, fmt.Sprintf("wikiSpaces[%d]", index), "workspaceId", "name", "description", "createTime", "updateTime", "spaceUrl"); err != nil {
			return resolveSpacePage{}, resolveSpaceProjectionError(err.Error())
		}
		spaceID, ok := row["workspaceId"].(string)
		spaceID = strings.TrimSpace(spaceID)
		if !ok || spaceID == "" {
			return resolveSpacePage{}, resolveSpaceProjectionError(fmt.Sprintf("wikiSpaces[%d] 缺少稳定 workspaceId", index))
		}
		name, ok := row["name"].(string)
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return resolveSpacePage{}, resolveSpaceProjectionError(fmt.Sprintf("wikiSpaces[%d] 缺少可消歧名称", index))
		}
		spaces = append(spaces, map[string]any{"spaceId": spaceID, "name": name})
	}
	return resolveSpacePage{spaces: spaces, hasMore: hasMore, nextToken: nextToken}, nil
}

func resolveSpaceSuccess(candidates []map[string]any, ledger *output.PageLedger) output.CommandResult {
	pagination, _ := ledger.Pagination()
	if pagination != nil {
		// Pagination.Items describes the records exposed by this resolver, while
		// Pages proves how many source pages were exhausted to establish it.
		pagination.Items = len(candidates)
	}
	return output.Success(map[string]any{
		"resolved":   len(candidates) == 1,
		"count":      len(candidates),
		"candidates": candidates,
	}, output.WithMeta(&output.Meta{
		Count:      output.NewCount(len(candidates)),
		Pagination: pagination,
	}))
}

func resolveSpaceProjectionError(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithOperation("wiki/list_wikiSpaces"),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
		apperrors.WithHint("不要把未知知识库响应、非法候选或缺失 workspaceId 的条目压成未找到；保留脱敏结构后修复投影。"),
	)
}

func resolveSpacePaginationError(message string, details map[string]any) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypePaginationInconsistent),
		apperrors.WithOperation("wiki/list_wikiSpaces"),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("pagination_projection"),
		apperrors.WithRetryable(false),
		apperrors.WithDetails(details),
		apperrors.WithHint("不要把未耗尽的组织知识库目录当成完整搜索结果；按 nextPageToken 继续读取或修复分页证据。"),
	)
}

func init() {
	shortcut.Register(ResolveSpace)
}
