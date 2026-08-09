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
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// MinutesDetail: fetch several artifacts of ONE minute (听记) in a single command
// and print them as one projected bundle.
//
// dws exposes each artifact as its own atomic tool (get_minutes_basic_info /
// get_minutes_ai_summary / get_minutes_keywords / get_minutes_transcription /
// list_minutes_todos). To assemble a full picture a user otherwise has to call
// 4–5 commands and stitch the taskUuid through each. This shortcut fans them out
// for one taskUuid, tolerates partial failure (a failing artifact is recorded as
// an error string rather than aborting the whole bundle) and projects the result
// through rt.Output so it honours --format/--jq/--fields.
//
// --artifacts selects which artifacts to pull (default: all). Each tool's params
// mirror the helper call sites in internal/helpers/minutes.go: every one takes a
// single "taskUuid", and transcription additionally takes "direction".
//
//	dws minutes +detail --id <taskUuid>
//	dws minutes +detail --id <taskUuid> --artifacts summary,todos
//	dws minutes +detail --id <taskUuid> --direction 1
var MinutesDetail = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "minutes",
	Command:       "+detail",
	Product:       "minutes",
	Description:   "一条命令聚合取一条妙记（听记）的多项产物（基础信息/摘要/关键词/逐字稿/待办）",
	Intent: "当你已经有某条听记的 taskUuid，想在一次操作里同时拿到它的基础信息、AI 摘要、关键词、逐字稿和待办，而不想分别敲 4~5 个子命令再自己拼时使用；" +
		"内部按 --artifacts 选择要拉的产物（默认全部：basic/summary/keywords/transcript/todos），逐个调用对应的原子工具并聚合成一个结果，" +
		"某一项失败不会中断整体（会以错误字符串记录在该项下）。这是纯只读操作，不会修改听记；--direction 仅影响逐字稿排序（0=正序默认，1=倒序）。",
	Risk: shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "minutes",
			Name:           "shortcut_detail",
			CanonicalPath:  "minutes.shortcut_detail",
			CLIPath:        "minutes +detail",
			PrimaryCLIPath: "minutes +detail",
		},
		Description: "一条命令聚合取一条妙记（听记）的多项产物（基础信息/摘要/关键词/逐字稿/待办）",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "一条命令聚合取一条妙记（听记）的多项产物（基础信息/摘要/关键词/逐字稿/待办）",
			UseWhen:      []string{"当你已经有某条听记的 taskUuid，想在一次操作里同时拿到它的基础信息、AI 摘要、关键词、逐字稿和待办，而不想分别敲 4~5 个子命令再自己拼时使用；内部按 --artifacts 选择要拉的产物（默认全部：basic/summary/keywords/transcript/todos），逐个调用对应的原子工具并聚合成一个结果，某一项失败不会中断整体（会以错误字符串记录在该项下）。这是纯只读操作，不会修改听记；--direction 仅影响逐字稿排序（0=正序默认，1=倒序）。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples: []string{
				"dws minutes +detail --id <taskUuid>",
				"dws minutes +detail --id <taskUuid> --artifacts summary,todos",
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "id", Type: shortcut.FlagString, Desc: "听记 taskUuid（必填）", Required: true},
		{Name: "artifacts", Type: shortcut.FlagStringSlice, Desc: "要拉取的产物子集（默认全部）", Required: false, Enum: []string{"basic", "summary", "keywords", "transcript", "todos"}},
		{Name: "direction", Type: shortcut.FlagString, Desc: "逐字稿排序: 0=正序(默认), 1=倒序（可选）", Required: false, Enum: []string{"0", "1"}},
	},
	Tips: []string{
		`dws minutes +detail --id <taskUuid>`,
		`dws minutes +detail --id <taskUuid> --artifacts summary,todos`,
		`dws minutes +detail --id <taskUuid> --direction 1`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		taskUUID := rt.Str("id")
		direction := rt.Str("direction")
		if direction == "" {
			direction = "0"
		}

		// Resolve which artifacts to pull; default to all in a stable order.
		want := rt.StrSlice("artifacts")
		if len(want) == 0 {
			want = minutesArtifactOrder
		}

		artifacts := make([]minutesArtifactRead, 0, len(want))
		for _, raw := range want {
			name := strings.ToLower(strings.TrimSpace(raw))
			tool, ok := minutesArtifactTools[name]
			if !ok {
				return apperrors.NewValidation(fmt.Sprintf("不支持的听记产物 %q", raw))
			}
			params := map[string]any{"taskUuid": taskUUID}
			if name == "transcript" {
				params["direction"] = direction
			}
			data, err := rt.CallMCPData("minutes", tool, params)
			artifacts = append(artifacts, minutesArtifactRead{Name: name, Data: data, Err: err})
		}

		payload, result, legacyErr, err := minutesDetailResult(taskUUID, artifacts)
		if err != nil {
			return err
		}
		if result.Outcome() == output.OutcomePartialFailure {
			return rt.OutputPartial(result, legacyErr)
		}
		return rt.OutputResult(payload, result)
	},
}

type minutesArtifactRead struct {
	Name string
	Data map[string]any
	Err  error
}

// minutesDetailResult translates each independently executed read into the
// framework's terminal result model. A failed artifact must never be hidden in
// a successful bundle: retaining the successful artifacts alongside typed
// failed entries gives an Agent a precise, safe resume point.
func minutesDetailResult(taskUUID string, artifacts []minutesArtifactRead) (map[string]any, output.CommandResult, error, error) {
	succeeded := make([]any, 0, len(artifacts))
	failed := make([]output.PartialFailedEntry, 0, len(artifacts))
	bundle := make(map[string]any, len(artifacts)+1)
	bundle["task_uuid"] = taskUUID

	for _, artifact := range artifacts {
		name := strings.TrimSpace(artifact.Name)
		if name == "" {
			return nil, nil, nil, fmt.Errorf("minutes detail artifact is missing a stable name")
		}
		id := "artifact:" + name
		if artifact.Err != nil {
			failed = append(failed, output.PartialFailedEntry{
				ID:    id,
				Error: minutesArtifactFailureInfo(name, artifact.Err),
			})
			continue
		}
		entry := map[string]any{"id": id, "artifact": name, "data": artifact.Data}
		succeeded = append(succeeded, entry)
		bundle[name] = artifact.Data
	}

	if len(failed) == 0 {
		bundle["artifact_count"] = len(succeeded)
		return bundle, output.Success(bundle,
			output.WithMeta(&output.Meta{Count: output.NewCount(len(succeeded))}),
		), nil, nil
	}
	if len(succeeded) == 0 {
		// An all-failed fan-out is a terminal failure rather than
		// partial_failure (the latter requires at least one succeeded entry),
		// but it must not compress the individual typed artifact failures into
		// an unhelpful list of names.  The aggregate error remains API-shaped
		// for legacy compatibility while details preserve each actionable error
		// for the unified failure envelope.
		failedArtifacts := make([]map[string]any, 0, len(failed))
		for _, entry := range failed {
			failedArtifacts = append(failedArtifacts, map[string]any{
				"id":    entry.ID,
				"error": entry.Error,
			})
		}
		return nil, nil, nil, apperrors.NewAPI(
			"请求的妙记产物均未能读取",
			apperrors.WithOperation("minutes/detail"),
			apperrors.WithFailureStage("artifact_read"),
			apperrors.WithRetryable(true),
			apperrors.WithDetails(map[string]any{"task_uuid": taskUUID, "failed_artifacts": failedArtifacts}),
		)
	}

	partial, err := output.NewPartialData(len(succeeded)+len(failed), succeeded, failed, []output.PartialUnknownEntry{})
	if err != nil {
		return nil, nil, nil, err
	}
	return nil, output.Partial(partial), fmt.Errorf("妙记详情部分产物读取失败"), nil
}

func minutesArtifactFailureInfo(name string, err error) *output.ErrorInfo {
	message := fmt.Sprintf("读取妙记产物 %q 失败", name)
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	info := &output.ErrorInfo{
		Type:             "api",
		Message:          message,
		Hint:             "保留已读取产物；只重新读取失败的 artifact，不要把当前结果当作完整详情",
		Operation:        "minutes/detail",
		Origin:           "mcp_gateway",
		Stage:            "artifact_read",
		ExecutionStarted: &started,
		Retryable:        true,
	}

	// rt.CallMCPData already carries the repository's typed errors.  Do not
	// erase auth/validation/projection semantics merely because this is one
	// member of a composite read: that would make an Agent retry an
	// unretryable failed artifact or miss the recovery action supplied by the
	// lower layer.  A generic read error retains the conservative, safe-to-
	// retry default above; a typed error is authoritative for its category,
	// subtype and retry guidance.
	return shortcut.PreserveTypedErrorInfo(info, err)
}

// minutesArtifactTools maps the user-facing artifact name to the real MCP tool
// (ground truth: internal/helpers/minutes.go). Each tool takes a single
// "taskUuid"; transcript additionally takes "direction".
var minutesArtifactTools = map[string]string{
	"basic":      "get_minutes_basic_info",
	"summary":    "get_minutes_ai_summary",
	"keywords":   "get_minutes_keywords",
	"transcript": "get_minutes_transcription",
	"todos":      "list_minutes_todos",
}

// minutesArtifactOrder is the stable default fan-out order (map iteration is
// unordered, so the "all" case uses this slice).
var minutesArtifactOrder = []string{"basic", "summary", "keywords", "transcript", "todos"}

func init() {
	shortcut.Register(MinutesDetail)
}
