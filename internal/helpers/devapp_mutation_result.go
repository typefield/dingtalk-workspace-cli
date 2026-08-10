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
				"requested":{
					"type":"object",
					"properties":{
						"unifiedAppId":{"type":"string"},
						"memberType":{"type":"string"},
						"userIds":{"type":"array","items":{"type":"string"}},
						"count":{"type":"integer","minimum":1}
					},
					"required":["unifiedAppId","memberType","userIds","count"],
					"additionalProperties":false
				},
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

func devAppMutationSuccessData(tool string, data any, params map[string]any) (any, *output.ErrorInfo) {
	switch strings.TrimSpace(tool) {
	case devAppCreateTool, devAppUpdateTool, devAppEnableTool, devAppDisableTool, devAppDeleteTool,
		devAppMemberAddTool, devAppMemberRemoveTool:
	default:
		return data, nil
	}
	object, ok := data.(map[string]any)
	if !ok {
		return nil, devAppMutationProjectionUnknown(tool, "returned no object result")
	}
	projected := make(map[string]any, len(object)+1)
	for key, value := range object {
		projected[key] = value
	}
	reason := "the helper response was not followed by a read-after-write terminal-state check"
	if tool == devAppMemberAddTool || tool == devAppMemberRemoveTool {
		requested, ok := devAppMemberMutationRequest(params)
		if !ok {
			return nil, devAppMutationProjectionUnknown(tool, "lost the requested member identifiers")
		}
		projected["requested"] = requested
		reason = "the aggregate helper acknowledgement was not followed by a member-list readback; requested userIds are not per-user success claims"
	}
	projected["verification"] = map[string]any{
		"state":  "not_verified",
		"reason": reason,
	}
	return projected, nil
}

func devAppMutationProjectionUnknown(tool, detail string) *output.ErrorInfo {
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypeProjectionUnknown),
		Message:          fmt.Sprintf("%s %s; the requested application change cannot be verified", tool, detail),
		Hint:             "不要盲目重试写操作；先用 devapp +get、devapp +member-list 或对应 dev app 查询命令核查目标状态。",
		Operation:        "devapp.mutation_projection",
		Stage:            "response_projection",
		ExecutionStarted: &started,
	}
}

func devAppMemberMutationRequest(params map[string]any) (map[string]any, bool) {
	appID, _ := params["unifiedAppId"].(string)
	memberType, _ := params["memberType"].(string)
	appID = strings.TrimSpace(appID)
	memberType = strings.TrimSpace(memberType)
	userIDs := make([]string, 0)
	switch values := params["userIds"].(type) {
	case []string:
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				userIDs = append(userIDs, value)
			}
		}
	case []any:
		for _, raw := range values {
			value, ok := raw.(string)
			if !ok {
				return nil, false
			}
			if value = strings.TrimSpace(value); value != "" {
				userIDs = append(userIDs, value)
			}
		}
	}
	if appID == "" || memberType == "" || len(userIDs) == 0 {
		return nil, false
	}
	return map[string]any{
		"unifiedAppId": appID,
		"memberType":   memberType,
		"userIds":      userIDs,
		"count":        len(userIDs),
	}, true
}
