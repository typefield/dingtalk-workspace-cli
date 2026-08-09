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

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/aitabletarget"
)

// ResolveBase: resolve a 多维表 Base by name keyword into a single baseId.
//
// This is the Base-level analogue of "resolve a user by name". It searches
// Bases by name and disambiguates:
//   - search Bases via search_bases (mirrors helpers base search, MCP arg
//     "query" ← --name);
//   - accept only the reviewed {baseId, baseName} candidate projection;
//   - exhaust every coherent search page before selecting a unique match;
//   - exactly one match → return {resolved:true, baseId, name};
//     multiple matches → return {resolved:false, count, candidates} and let
//     the caller pick (never guesses);
//     zero matches → report a validation error instead of an empty raw dump.
//
// Read-only: it only searches and reshapes, never mutates any Base.
//
//	dws aitable +resolve-base --name 项目管理
var ResolveBase = shortcut.Shortcut{
	OutputRollout: output.RolloutDualValidate,
	Service:       "aitable",
	Command:       "+resolve-base",
	Product:       "aitable",
	Description:   "在名称搜索端点完整耗尽后解析唯一 Base（索引覆盖未知，只读）",
	Intent: "当你只知道某个多维表 Base 的名称、想把它解析成可直接用于后续工具的 baseId 时使用；" +
		"内部严格校验 search_bases 的稳定字段和分页证据，只有搜索端点明确耗尽后才做大小写不敏感的精确名称匹配；显式 --fuzzy 才允许关键词包含匹配。" +
		"0 个或多个候选都会以结构化错误失败并返回候选，绝不替你猜选。" +
		"名称搜索索引覆盖始终未知，因此空结果只表示索引当前没有候选，不证明业务上不存在该 Base。" +
		"这是纯只读操作，只做搜索与本地投影，不会修改任何 Base。",
	Risk: shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "aitable",
			Name:           "shortcut_resolve_base",
			CanonicalPath:  "aitable.shortcut_resolve_base",
			CLIPath:        "aitable +resolve-base",
			PrimaryCLIPath: "aitable +resolve-base",
		},
		Description: "在名称搜索端点完整耗尽后解析唯一 Base（索引覆盖未知，只读）",
		Result: &contract.ResultSpec{
			Outcomes: []contract.ResultOutcome{
				contract.ResultOutcomeSuccess,
				contract.ResultOutcomeFailure,
			},
			DataSchema: json.RawMessage(`{"type":"object","properties":{"resolved":{"type":"boolean","const":true},"matchType":{"type":"string","enum":["exact","fuzzy"]},"count":{"type":"integer","const":1},"candidates":{"type":"array","minItems":1,"maxItems":1,"items":{"type":"object","properties":{"baseId":{"type":"string","minLength":1},"baseName":{"type":"string","minLength":1}},"required":["baseId","baseName"],"additionalProperties":false}},"sourceKind":{"type":"string","const":"name_search_index"},"authoritativeInventory":{"type":"boolean","const":false},"inventoryCoverageKnown":{"type":"boolean","const":false},"indexCoverageKnown":{"type":"boolean","const":false}},"required":["resolved","matchType","count","candidates","sourceKind","authoritativeInventory","inventoryCoverageKnown","indexCoverageKnown"],"additionalProperties":false}`),
			NDJSON: &contract.ResultNDJSONSpec{
				RecordPath:   "candidates",
				RecordSchema: json.RawMessage(`{"type":"object","properties":{"baseId":{"type":"string","minLength":1},"baseName":{"type":"string","minLength":1}},"required":["baseId","baseName"],"additionalProperties":false}`),
			},
		},
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed read-only resolver: the CLI accepts only stable baseId/baseName rows, validates every cursor transition with a bounded page ledger, and selects only after the search endpoint is exhausted; it never claims the name index is authoritative.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "在名称搜索端点完整耗尽后解析唯一 Base（索引覆盖未知，只读）",
			UseWhen:      []string{"当你只知道某个 Base 的名称并接受名称索引覆盖未知这一边界时使用；命令会穷尽连贯分页并只返回唯一稳定 baseId/baseName，绝不猜测。"},
			AvoidWhen:    []string{"已有 URL/baseId 时直接使用确定性标识；需要证明 Base 不存在或枚举全部可访问 Base 时不要依赖名称索引"},
			Examples:     []string{"dws aitable +resolve-base --name 项目管理"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "name", Type: shortcut.FlagString, Desc: "要解析的 Base 名称", Required: true},
		{Name: "fuzzy", Type: shortcut.FlagBool, Default: "false", Desc: "精确名称无结果时允许包含匹配"},
	},
	Tips: []string{
		`dws aitable +resolve-base --name 项目管理`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		resolution, ledger, err := aitabletarget.ResolveBaseNameWithEvidence(rt, rt.Str("name"), rt.Bool("fuzzy"))
		if err != nil {
			return err
		}
		legacy := map[string]any{
			"resolved":  true,
			"status":    resolution.Status,
			"matchType": resolution.MatchType,
			"baseId":    resolution.Selected.ID,
			"name":      resolution.Selected.Name,
		}
		candidate := map[string]any{
			"baseId":   resolution.Selected.ID,
			"baseName": resolution.Selected.Name,
		}
		data := map[string]any{
			"resolved":               true,
			"matchType":              resolution.MatchType,
			"count":                  1,
			"candidates":             []map[string]any{candidate},
			"sourceKind":             "name_search_index",
			"authoritativeInventory": false,
			"inventoryCoverageKnown": false,
			"indexCoverageKnown":     false,
		}
		result, err := ledger.Result(data, output.WithMeta(&output.Meta{Count: output.NewCount(1)}))
		if err != nil {
			return err
		}
		return rt.OutputResult(legacy, result)
	},
}

func init() {
	shortcut.Register(ResolveBase)
}
