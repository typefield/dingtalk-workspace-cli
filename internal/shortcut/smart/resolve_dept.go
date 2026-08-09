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
	"math"
	"strconv"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// ResolveDept: resolve a department by name keyword into a single deptId.
//
// This is the department-level analogue of "resolve a user by name". It searches
// departments by name and disambiguates:
//   - search departments via search_dept_by_keyword (mirrors helpers contact
//     dept search, MCP arg "query" ← --name);
//   - project each candidate to {deptId, name} — field parsing is defensive
//     (multiple candidate keys);
//   - exactly one match → return {resolved:true, deptId, name};
//     multiple matches → return {resolved:false, count, candidates} and let
//     the caller pick (never guesses);
//     zero matches → report a validation error instead of an empty raw dump.
//
// Read-only: it only searches and reshapes, never mutates anything. Unlike
// +dept-members it stops at the deptId and does NOT list members.
//
//	dws contact +resolve-dept --name 技术部
var ResolveDept = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "contact",
	Command:       "+resolve-dept",
	Product:       "contact",
	Description:   "按名称搜索部门并解析出唯一 deptId（只读）",
	Intent: "当你只知道某个部门的名称（或名称里的关键词）、想把它解析成可直接用于后续工具的 deptId 时使用；" +
		"内部按 --name 关键词调用 search_dept_by_keyword 搜索部门，再在本地投影出每个候选的 deptId 和 name。" +
		"如果只命中一个部门就直接返回它的 deptId；如果命中多个则列出全部候选让你消歧，绝不替你瞎猜；如果一个都没命中则提示未找到。" +
		"这是纯只读操作，只做搜索与本地投影，不会修改任何部门，也不会列出部门成员。",
	Risk: shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "contact",
			Name:           "shortcut_resolve_dept",
			CanonicalPath:  "contact.shortcut_resolve_dept",
			CLIPath:        "contact +resolve-dept",
			PrimaryCLIPath: "contact +resolve-dept",
		},
		Description: "按名称搜索部门并解析出唯一 deptId（只读）",
		Result: &contract.ResultSpec{
			Outcomes: []contract.ResultOutcome{
				contract.ResultOutcomeSuccess,
				contract.ResultOutcomeFailure,
			},
			DataSchema: json.RawMessage(`{"type":"object","properties":{"resolved":{"type":"boolean"},"count":{"type":"integer","minimum":1},"candidates":{"type":"array","minItems":1,"items":{"type":"object","properties":{"deptId":{"type":"string"},"name":{"type":"string"}},"required":["deptId","name"],"additionalProperties":false}}},"required":["resolved","count","candidates"],"additionalProperties":false}`),
			NDJSON:     &contract.ResultNDJSONSpec{RecordPath: "candidates", RecordSchema: json.RawMessage(`{"type":"object","properties":{"deptId":{"type":"string"},"name":{"type":"string"}},"required":["deptId","name"],"additionalProperties":false}`)},
		},
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "按名称搜索部门并解析出唯一 deptId（只读）",
			UseWhen:      []string{"当你只知道某个部门的名称（或名称里的关键词）、想把它解析成可直接用于后续工具的 deptId 时使用；内部按 --name 关键词调用 search_dept_by_keyword 搜索部门，再在本地投影出每个候选的 deptId 和 name。如果只命中一个部门就直接返回它的 deptId；如果命中多个则列出全部候选让你消歧，绝不替你瞎猜；如果一个都没命中则提示未找到。这是纯只读操作，只做搜索与本地投影，不会修改任何部门，也不会列出部门成员。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws contact +resolve-dept --name 技术部"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "name", Type: shortcut.FlagString, Desc: "要搜索的部门名称关键词（必填）", Required: true},
	},
	Tips: []string{
		`dws contact +resolve-dept --name 技术部`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		// Search departments by name. tool "search_dept_by_keyword" + arg
		// "query" are taken verbatim from helpers contact dept search
		// (callMCPTool → server contact).
		data, err := rt.CallMCPData("contact", "search_dept_by_keyword", map[string]any{
			"query": rt.Str("name"),
		})
		if err != nil {
			return err
		}

		// Project the complete terminal search page. Unknown containers, invalid
		// rows and contradictory pagination are not equivalent to zero matches.
		candidates, err := resolveDeptCandidates(data)
		if err != nil {
			return err
		}

		switch len(candidates) {
		case 0:
			return apperrors.NewValidation("没有找到名称包含 " + rt.Str("name") + " 的部门")
		case 1:
			legacy := map[string]any{
				"resolved": true,
				"deptId":   candidates[0]["deptId"],
				"name":     candidates[0]["name"],
			}
			return rt.OutputResult(legacy, resolveDeptSuccess(candidates))
		default:
			legacy := map[string]any{
				"resolved":   false,
				"count":      len(candidates),
				"candidates": candidates,
			}
			return rt.OutputResult(legacy, resolveDeptSuccess(candidates))
		}
	},
}

// resolveDeptCandidates projects the one reviewed gateway response shape. The
// command resolves uniqueness, so it may only succeed after observing an
// explicit terminal page whose total agrees with every projected row.
func resolveDeptCandidates(data map[string]any) ([]map[string]any, error) {
	if data == nil {
		return nil, resolveDeptProjectionError("部门搜索响应不是 JSON 对象")
	}
	if err := rejectUnknownKeys(data, "部门搜索顶层", "success", "deptList", "hasMore", "totalCount"); err != nil {
		return nil, resolveDeptProjectionError(err.Error())
	}
	success, ok := data["success"].(bool)
	if !ok {
		return nil, resolveDeptProjectionError("部门搜索 success 不是布尔值")
	}
	if !success {
		return nil, resolveDeptProjectionError("部门搜索业务结果明确失败")
	}
	hasMore, ok := data["hasMore"].(bool)
	if !ok {
		return nil, resolveDeptPaginationError("部门搜索 hasMore 缺失或不是布尔值", nil)
	}
	rows, ok := data["deptList"].([]any)
	if !ok {
		return nil, resolveDeptProjectionError("部门搜索 deptList 缺失或不是数组")
	}
	total, ok := resolveDeptInteger(data["totalCount"])
	if !ok || total < 0 {
		return nil, resolveDeptPaginationError("部门搜索 totalCount 缺失或不是非负整数", map[string]any{"projected_count": len(rows)})
	}
	if hasMore {
		return nil, resolveDeptPaginationError("部门搜索仍有下一页但没有可恢复游标，无法判断唯一部门", map[string]any{"projected_count": len(rows), "total_count": total})
	}
	if total != int64(len(rows)) {
		return nil, resolveDeptPaginationError("部门搜索终页 totalCount 与候选条数不一致", map[string]any{"projected_count": len(rows), "total_count": total})
	}

	candidates := make([]map[string]any, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for index, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, resolveDeptProjectionError(fmt.Sprintf("deptList[%d] 不是对象", index))
		}
		if err := rejectUnknownKeys(row, fmt.Sprintf("deptList[%d]", index), "deptId", "deptName"); err != nil {
			return nil, resolveDeptProjectionError(err.Error())
		}
		deptID, ok := resolveDeptStableID(row["deptId"])
		if !ok {
			return nil, resolveDeptProjectionError(fmt.Sprintf("deptList[%d] 缺少稳定 deptId", index))
		}
		if _, duplicate := seen[deptID]; duplicate {
			return nil, resolveDeptProjectionError(fmt.Sprintf("deptList[%d] deptId 重复", index))
		}
		seen[deptID] = struct{}{}
		name, ok := row["deptName"].(string)
		name = strings.TrimSpace(stripHighlightTags(name))
		if !ok || name == "" {
			return nil, resolveDeptProjectionError(fmt.Sprintf("deptList[%d] 缺少可消歧部门名称", index))
		}
		candidates = append(candidates, map[string]any{"deptId": deptID, "name": name})
	}
	return candidates, nil
}

func resolveDeptStableID(raw any) (string, bool) {
	var value int64
	switch typed := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		parsed, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			return "", false
		}
		value = parsed
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || math.Trunc(typed) != typed || math.Abs(typed) > 1<<53 {
			return "", false
		}
		value = int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return "", false
		}
		value = parsed
	case int:
		value = int64(typed)
	case int32:
		value = int64(typed)
	case int64:
		value = typed
	default:
		return "", false
	}
	// The search API uses -1 as the enterprise-root sentinel, while all
	// downstream department commands require the canonical root ID 1.
	if value == -1 {
		value = 1
	}
	if value <= 0 {
		return "", false
	}
	return strconv.FormatInt(value, 10), true
}

func resolveDeptInteger(raw any) (int64, bool) {
	switch typed := raw.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) || math.Trunc(typed) != typed || math.Abs(typed) > 1<<53 {
			return 0, false
		}
		return int64(typed), true
	case json.Number:
		value, err := typed.Int64()
		return value, err == nil
	default:
		return 0, false
	}
}

func resolveDeptSuccess(candidates []map[string]any) output.CommandResult {
	page, _ := output.NewPagination(true, "")
	page.Pages = 1
	page.Items = len(candidates)
	return output.Success(map[string]any{
		"resolved":   len(candidates) == 1,
		"count":      len(candidates),
		"candidates": candidates,
	}, output.WithMeta(&output.Meta{Count: output.NewCount(len(candidates)), Pagination: page}))
}

func resolveDeptProjectionError(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithOperation("contact/search_dept_by_keyword"),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
		apperrors.WithHint("不要把未知部门响应、非法候选或缺失 ID 的条目压成未找到；保留脱敏结构后修复投影。"),
	)
}

func resolveDeptPaginationError(message string, details map[string]any) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypePaginationInconsistent),
		apperrors.WithOperation("contact/search_dept_by_keyword"),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("pagination_projection"),
		apperrors.WithRetryable(false),
		apperrors.WithDetails(details),
		apperrors.WithHint("不要把当前候选当成完整搜索结果；改用 contact dept search 核查分页证据。"),
	)
}

// stripHighlightTags removes the <red>/</red> (and similar simple) highlight
// tags the DingTalk search gateway injects around matched keywords.
func stripHighlightTags(s string) string {
	for _, tag := range []string{"<red>", "</red>", "<b>", "</b>", "<em>", "</em>"} {
		s = strings.ReplaceAll(s, tag, "")
	}
	return s
}

func init() {
	shortcut.Register(ResolveDept)
}
