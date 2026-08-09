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

// ResolveTable: resolve a 数据表 (table) inside one Base by name keyword into a
// single tableId.
//
// This is the table-level analogue of "resolve a Base by name". Because there is
// no server tool that searches tables by name, it lists every table in the Base
// via get_tables (baseId ← --base, verbatim from list_tables / helpers
// tableGetCmd) and then matches --name locally:
//   - project each table to {tableId, name} — field parsing is defensive
//     (multiple candidate keys);
//   - filter locally by a case-insensitive substring match on name;
//   - exactly one match → return {resolved:true, tableId, name, base};
//     multiple matches → return {resolved:false, count, candidates} and let the
//     caller pick (never guesses);
//     zero matches → report a validation error instead of an empty raw dump.
//
// Read-only: it only lists and reshapes, never mutates any table.
//
//	dws aitable +resolve-table --base B --name 任务
var ResolveTable = shortcut.Shortcut{
	OutputRollout: output.RolloutDualValidate,
	Service:       "aitable",
	Command:       "+resolve-table",
	Product:       "aitable",
	Description:   "在某个多维表 Base 的完整表目录内按名称解析唯一 tableId（只读）",
	Intent: "当你已经知道某个多维表 Base 的 baseId、又只记得里面某张数据表(table)的名称、" +
		"想把它解析成可直接用于后续工具的 tableId 时使用；" +
		"内部先列出全部数据表并优先做大小写不敏感的精确名称匹配，只有显式 --fuzzy 才允许包含匹配。" +
		"0 个或多个候选都会以结构化错误失败并返回候选，绝不替你猜选。" +
		"这是纯只读操作，只做列举、本地匹配与投影，不会创建、修改或删除任何数据表。",
	Risk: shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "aitable",
			Name:           "shortcut_resolve_table",
			CanonicalPath:  "aitable.shortcut_resolve_table",
			CLIPath:        "aitable +resolve-table",
			PrimaryCLIPath: "aitable +resolve-table",
		},
		Description: "在某个多维表 Base 的完整表目录内按名称解析唯一 tableId（只读）",
		Result: &contract.ResultSpec{
			Outcomes: []contract.ResultOutcome{
				contract.ResultOutcomeSuccess,
				contract.ResultOutcomeFailure,
			},
			DataSchema: json.RawMessage(`{"type":"object","properties":{"resolved":{"type":"boolean","const":true},"count":{"type":"integer","const":1},"candidates":{"type":"array","minItems":1,"maxItems":1,"items":{"type":"object","properties":{"tableId":{"type":"string","minLength":1},"tableName":{"type":"string","minLength":1}},"required":["tableId","tableName"],"additionalProperties":false}}},"required":["resolved","count","candidates"],"additionalProperties":false}`),
			NDJSON: &contract.ResultNDJSONSpec{
				RecordPath:   "candidates",
				RecordSchema: json.RawMessage(`{"type":"object","properties":{"tableId":{"type":"string","minLength":1},"tableName":{"type":"string","minLength":1}},"required":["tableId","tableName"],"additionalProperties":false}`),
			},
		},
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed read-only resolver: the CLI strictly validates the complete non-paginated get_tables directory, then performs explicit exact/fuzzy matching without guessing among candidates.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "在某个多维表 Base 内按名称解析出唯一的数据表 tableId（只读）",
			UseWhen:      []string{"当你已经知道某个多维表 Base 的 baseId、又只记得里面某张数据表(table)的名称、想把它解析成可直接用于后续工具的 tableId 时使用；内部先列出全部数据表并优先做大小写不敏感的精确名称匹配，只有显式 --fuzzy 才允许包含匹配。0 个或多个候选都会以结构化错误失败并返回候选，绝不替你猜选。这是纯只读操作，只做列举、本地匹配与投影，不会创建、修改或删除任何数据表。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws aitable +resolve-table --base B --name 任务"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "base", Type: shortcut.FlagString, Desc: "Base ID（要在其内解析数据表的多维表）", Required: true},
		{Name: "name", Type: shortcut.FlagString, Desc: "要解析的数据表名称", Required: true},
		{Name: "fuzzy", Type: shortcut.FlagBool, Default: "false", Desc: "精确名称无结果时允许包含匹配"},
	},
	Tips: []string{
		`dws aitable +resolve-table --base B --name 任务`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		resolution, err := aitabletarget.ResolveTableName(rt, rt.Str("base"), rt.Str("name"), rt.Bool("fuzzy"))
		if err != nil {
			return err
		}
		legacy := map[string]any{
			"resolved":  true,
			"status":    resolution.Status,
			"matchType": resolution.MatchType,
			"tableId":   resolution.Selected.ID,
			"name":      resolution.Selected.Name,
			"base":      rt.Str("base"),
		}
		candidate := map[string]any{
			"tableId":   resolution.Selected.ID,
			"tableName": resolution.Selected.Name,
		}
		data := map[string]any{
			"resolved":   true,
			"count":      1,
			"candidates": []map[string]any{candidate},
		}
		return rt.OutputResult(legacy, output.Success(data, output.WithMeta(&output.Meta{
			Count: output.NewCount(1),
		})))
	},
}

func init() {
	shortcut.Register(ResolveTable)
}
