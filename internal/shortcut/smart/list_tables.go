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

// ListTables returns the complete stable table directory for one Base. The
// underlying get_tables call omits tableIds, which the reviewed endpoint
// contract defines as the full non-paginated directory. Unknown containers,
// invalid rows, duplicate IDs and missing stable fields fail closed instead of
// becoming an empty or raw successful payload.
//
//	dws aitable +list-tables --base B
var ListTables = shortcut.Shortcut{
	OutputRollout: output.RolloutDualValidate,
	Service:       "aitable",
	Command:       "+list-tables",
	Product:       "aitable",
	Description:   "列出某个多维表(base)里的完整数据表目录（只读，严格投影 tableId/tableName）",
	Intent: "当你已经知道某个多维表(base)的 baseId、想一步看清这个 base 下都有哪些数据表(table)、" +
		"拿到它们的稳定 tableId 和 tableName 以便后续查记录或改结构时使用；" +
		"内部调用 get_tables 且不传 tableIds，严格校验完整目录响应和每一条稳定身份。" +
		"未知结构、非法条目或重复 ID 会失败，不会被压成空列表或原始成功结果。" +
		"这是纯只读操作，只做列举与本地投影，不会创建、修改或删除任何表。",
	Risk: shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "aitable",
			Name:           "shortcut_list_tables",
			CanonicalPath:  "aitable.shortcut_list_tables",
			CLIPath:        "aitable +list-tables",
			PrimaryCLIPath: "aitable +list-tables",
		},
		Description: "列出某个多维表(base)里的完整数据表目录（只读，严格投影 tableId/tableName）",
		Result: &contract.ResultSpec{
			Outcomes: []contract.ResultOutcome{
				contract.ResultOutcomeSuccess,
				contract.ResultOutcomeFailure,
			},
			DataSchema: json.RawMessage(`{"type":"object","properties":{"tables":{"type":"array","items":{"type":"object","properties":{"tableId":{"type":"string","minLength":1},"tableName":{"type":"string","minLength":1}},"required":["tableId","tableName"],"additionalProperties":false}}},"required":["tables"],"additionalProperties":false}`),
			NDJSON: &contract.ResultNDJSONSpec{
				RecordPath:   "tables",
				RecordSchema: json.RawMessage(`{"type":"object","properties":{"tableId":{"type":"string","minLength":1},"tableName":{"type":"string","minLength":1}},"required":["tableId","tableName"],"additionalProperties":false}`),
			},
		},
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed read-only directory adapter: get_tables without tableIds returns the Base table directory; the CLI validates the live response envelope and every stable tableId/tableName before emitting it.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "列出某个 Base 的完整稳定数据表目录",
			UseWhen:      []string{"已经知道 baseId，需要完整列出该 Base 中可供后续命令使用的稳定 tableId 和 tableName 时"},
			AvoidWhen:    []string{"已经知道 tableId 时直接使用它；需要字段或视图详情时使用 aitable table get"},
			Examples:     []string{"dws aitable +list-tables --base B --format json"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "base", Type: shortcut.FlagString, Desc: "Base ID（要列出数据表的多维表）", Required: true},
	},
	Tips: []string{
		`dws aitable +list-tables --base B --format json`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		raw, candidates, err := aitabletarget.FetchTableDirectory(rt, rt.Str("base"))
		if err != nil {
			return err
		}
		tables := make([]map[string]any, 0, len(candidates))
		for _, candidate := range candidates {
			tables = append(tables, map[string]any{
				"tableId":   candidate.ID,
				"tableName": candidate.Name,
			})
		}
		payload := map[string]any{"tables": tables}
		legacyPayload := any(payload)
		if len(tables) == 0 {
			// Before unified activation, the historical implementation emitted
			// the raw get_tables payload when the projected list was empty. Keep
			// those bytes during dual validation; the shadow result still proves
			// the future stable empty directory.
			legacyPayload = raw
		}
		return rt.OutputResult(legacyPayload, output.Success(payload, output.WithMeta(&output.Meta{
			Count: output.NewCount(len(tables)),
		})))
	},
}

func init() {
	shortcut.Register(ListTables)
}
