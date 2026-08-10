// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

// DevAppMutationResultSpec is the shared Agent contract for the core DevApp
// lifecycle writes. A successful helper response only proves that the request
// returned successfully; these commands do not perform a read-after-write
// check. The active projector therefore adds verification.state=not_verified
// instead of claiming the application reached its requested terminal state.
//
// Partial and pending remain declared because the same helper can execute over
// multiple selected profiles, and future service responses can explicitly
// return a resumable non-terminal operation. The framework, not the command,
// still owns ok/outcome/error and the matching process exit code.
func DevAppMutationResultSpec() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{
			contract.ResultOutcomeSuccess,
			contract.ResultOutcomePending,
			contract.ResultOutcomePartialFailure,
			contract.ResultOutcomeFailure,
		},
		DataSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"unifiedAppId":{"type":"string"},
				"enabled":{"type":"boolean"},
				"disabled":{"type":"boolean"},
				"status":{"type":"string"},
				"versionStatus":{"type":"string"},
				"verification":{
					"type":"object",
					"properties":{
						"state":{"type":"string","enum":["not_verified"]},
						"reason":{"type":"string"}
					},
					"required":["state","reason"],
					"additionalProperties":false
				}
			},
			"additionalProperties":true
		}`),
	}
}

func devAppMutationSuccessData(tool string, data any) (any, *output.ErrorInfo) {
	switch strings.TrimSpace(tool) {
	case devAppCreateTool, devAppUpdateTool, devAppEnableTool, devAppDisableTool, devAppDeleteTool:
	default:
		return data, nil
	}
	object, ok := data.(map[string]any)
	if !ok {
		started := true
		return nil, &output.ErrorInfo{
			Type:             "api",
			Subtype:          string(apperrors.SubtypeProjectionUnknown),
			Message:          fmt.Sprintf("%s returned no object result; the requested application change cannot be verified", tool),
			Hint:             "不要盲目重试写操作；先用 devapp +get 或对应 dev app get 查询目标应用状态。",
			Operation:        "devapp.mutation_projection",
			Stage:            "response_projection",
			ExecutionStarted: &started,
		}
	}
	projected := make(map[string]any, len(object)+1)
	for key, value := range object {
		projected[key] = value
	}
	projected["verification"] = map[string]any{
		"state":  "not_verified",
		"reason": "the helper response was not followed by a read-after-write terminal-state check",
	}
	return projected, nil
}
