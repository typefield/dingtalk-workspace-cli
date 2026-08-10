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

// DevAppVersionCreateResultSpec describes the result that is safe for an
// Agent to consume after creating a version.  A returned versionId identifies
// the accepted resource; verification remains explicit because the command
// does not read the version back after the write.
func DevAppVersionCreateResultSpec() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{
			contract.ResultOutcomeSuccess,
			contract.ResultOutcomePending,
			contract.ResultOutcomePartialFailure,
			contract.ResultOutcomeFailure,
		},
		DataSchema: devAppVersionWriteDataSchema(),
	}
}

// DevAppVersionPublishResultSpec includes pending because a publish can stop
// for approver selection or enter an asynchronous approval workflow.
func DevAppVersionPublishResultSpec() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{
			contract.ResultOutcomeSuccess,
			contract.ResultOutcomePending,
			contract.ResultOutcomePartialFailure,
			contract.ResultOutcomeFailure,
		},
		DataSchema: devAppVersionWriteDataSchema(),
	}
}

// DevAppVersionPrecheckResultSpec is terminal: check-approval may describe a
// later user action, but the read-only precheck itself has completed.
func DevAppVersionPrecheckResultSpec() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{
			contract.ResultOutcomeSuccess,
			contract.ResultOutcomeFailure,
		},
		DataSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"requiresApproval":{"type":"boolean"},
				"publishable":{"type":"boolean"},
				"approvalMode":{"type":"string"},
				"approvalOptions":{"type":"array","items":{"type":"object"}},
				"approvalPromptText":{"type":"string"},
				"completionState":{"type":"string"},
				"requested":{
					"type":"object",
					"properties":{
						"operation":{"const":"check_approval"},
						"unifiedAppId":{"type":"string"},
						"versionId":{"type":"string"}
					},
					"required":["operation","unifiedAppId","versionId"],
					"additionalProperties":false
				}
			},
			"required":["requested"],
			"additionalProperties":true
		}`),
	}
}

func DevAppVersionStatusResultSpec() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{
			contract.ResultOutcomePending,
			contract.ResultOutcomeFailure,
			contract.ResultOutcomeSuccess,
		},
		DataSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"unifiedAppId":{"type":"string"},
				"versionId":{"type":"string"},
				"status":{"type":"string"},
				"versionStatus":{"type":"string"},
				"processStatus":{"type":"string"},
				"approvalStatus":{"type":"string"},
				"nextCommand":{"type":"string"}
			},
			"required":["versionId"],
			"additionalProperties":true
		}`),
	}
}

func devAppVersionWriteDataSchema() json.RawMessage {
	return json.RawMessage(`{
		"type":"object",
		"properties":{
			"unifiedAppId":{"type":"string"},
			"versionId":{"type":"string"},
			"versionStatus":{"type":"string"},
			"published":{"type":"boolean"},
			"approvalSubmitted":{"type":"boolean"},
			"processId":{"type":"string"},
			"processInstanceId":{"type":"string"},
			"completionState":{"type":"string"},
			"requested":{
				"type":"object",
				"properties":{
					"operation":{"type":"string","enum":["create_version","publish_version"]},
					"unifiedAppId":{"type":"string"},
					"versionId":{"type":"string"},
					"version":{"type":"string"},
					"desc":{"type":"string"},
					"approverUserId":{"type":"string"},
					"confirmedSensitive":{"type":"boolean"}
				},
				"required":["operation","unifiedAppId"],
				"additionalProperties":false
			},
			"verification":{
				"type":"object",
				"properties":{
					"state":{"const":"not_verified"},
					"reason":{"type":"string"},
					"next_command":{"type":"string"}
				},
				"required":["state","reason","next_command"],
				"additionalProperties":false
			}
		},
		"additionalProperties":true
	}`)
}

func devAppVersionSuccessData(tool string, data any, params map[string]any) (any, *output.ErrorInfo) {
	tool = strings.TrimSpace(tool)
	if tool != devAppVersionCreateTool && tool != devAppVersionPublishTool {
		return data, nil
	}
	object, ok := data.(map[string]any)
	if !ok {
		return nil, devAppVersionProjectionUnknown(tool, "returned no object result")
	}
	appID := devAppStringParam(params, "unifiedAppId")
	if appID == "" {
		return nil, devAppVersionProjectionUnknown(tool, "lost the requested unifiedAppId")
	}
	if responseAppID := devAppFirstContentString(object, "unifiedAppId"); responseAppID != "" && responseAppID != appID {
		return nil, devAppVersionProjectionUnknown(tool, "returned a different unifiedAppId")
	}

	if tool == devAppVersionPublishTool {
		if precheck, _ := params["precheckOnly"].(bool); precheck {
			return devAppVersionPrecheckSuccessData(object, params)
		}
		return devAppVersionPublishSuccessData(object, params)
	}
	return devAppVersionCreateSuccessData(object, params)
}

func devAppVersionCreateSuccessData(object map[string]any, params map[string]any) (any, *output.ErrorInfo) {
	versionID := devAppFirstContentString(object, "versionId")
	if versionID == "" {
		return nil, devAppVersionProjectionUnknown(devAppVersionCreateTool, "did not return a stable versionId")
	}
	projected := cloneDevAppObject(object)
	requested := map[string]any{
		"operation":    "create_version",
		"unifiedAppId": devAppStringParam(params, "unifiedAppId"),
	}
	for _, key := range []string{"version", "desc"} {
		if value := devAppStringParam(params, key); value != "" {
			requested[key] = value
		}
	}
	projected["requested"] = requested
	projected["verification"] = map[string]any{
		"state":        "not_verified",
		"reason":       "the helper returned a versionId, but the created version was not read back",
		"next_command": devAppVersionStatusCommand(requested["unifiedAppId"].(string), versionID),
	}
	return projected, nil
}

func devAppVersionPrecheckSuccessData(object map[string]any, params map[string]any) (any, *output.ErrorInfo) {
	appID := devAppStringParam(params, "unifiedAppId")
	versionID := devAppStringParam(params, "versionId")
	if versionID == "" {
		return nil, devAppVersionProjectionUnknown(devAppVersionPublishTool, "lost the requested versionId during approval precheck")
	}
	if responseVersionID := devAppFirstContentString(object, "versionId"); responseVersionID != "" && responseVersionID != versionID {
		return nil, devAppVersionProjectionUnknown(devAppVersionPublishTool, "approval precheck returned a different versionId")
	}
	_, hasRequiresApproval := object["requiresApproval"].(bool)
	_, hasPublishable := object["publishable"].(bool)
	known := hasRequiresApproval || hasPublishable || devAppFirstContentString(object, "approvalMode", "completionState") != ""
	if !known {
		return nil, devAppVersionProjectionUnknown(devAppVersionPublishTool, "approval precheck returned no recognized decision fields")
	}
	projected := cloneDevAppObject(object)
	projected["requested"] = map[string]any{
		"operation":    "check_approval",
		"unifiedAppId": appID,
		"versionId":    versionID,
	}
	return projected, nil
}

func devAppVersionPublishSuccessData(object map[string]any, params map[string]any) (any, *output.ErrorInfo) {
	appID := devAppStringParam(params, "unifiedAppId")
	versionID := devAppStringParam(params, "versionId")
	if versionID == "" {
		return nil, devAppVersionProjectionUnknown(devAppVersionPublishTool, "lost the requested versionId")
	}
	if responseVersionID := devAppFirstContentString(object, "versionId"); responseVersionID != "" && responseVersionID != versionID {
		return nil, devAppVersionProjectionUnknown(devAppVersionPublishTool, "returned a different versionId")
	}
	published, publishedKnown := object["published"].(bool)
	versionStatus := strings.ToUpper(devAppFirstContentString(object, "versionStatus"))
	responseClaimsPublished := published || versionStatus == "RELEASE" || versionStatus == "GRAY"
	if !responseClaimsPublished {
		detail := "returned no terminal publish evidence"
		if publishedKnown && !published {
			detail = "explicitly reported published=false without a resumable operation"
		}
		return nil, devAppVersionProjectionUnknown(devAppVersionPublishTool, detail)
	}
	projected := cloneDevAppObject(object)
	requested := map[string]any{
		"operation":    "publish_version",
		"unifiedAppId": appID,
		"versionId":    versionID,
	}
	if value := devAppStringParam(params, "approverUserId"); value != "" {
		requested["approverUserId"] = value
	}
	if value, present := params["confirmedSensitive"].(bool); present {
		requested["confirmedSensitive"] = value
	}
	projected["requested"] = requested
	projected["verification"] = map[string]any{
		"state":        "not_verified",
		"reason":       "the publish response claimed a released or gray state, but version status was not read back",
		"next_command": devAppVersionStatusCommand(appID, versionID),
	}
	return projected, nil
}

func devAppVersionProjectionUnknown(tool, detail string) *output.ErrorInfo {
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypeProjectionUnknown),
		Message:          fmt.Sprintf("%s %s; the version operation state cannot be verified", tool, detail),
		Hint:             "不要盲目重试版本写操作；先用 version list/get/status 核查目标版本与审批状态，再决定是否继续。",
		Operation:        "devapp.version_projection",
		Stage:            "response_projection",
		ExecutionStarted: &started,
	}
}

func devAppVersionStatusCommand(appID, versionID string) string {
	return fmt.Sprintf("dws dev app version status --unified-app-id %s --version-id %s --format json", appID, versionID)
}

func devAppStringParam(params map[string]any, key string) string {
	value, _ := params[key].(string)
	return strings.TrimSpace(value)
}

func cloneDevAppObject(source map[string]any) map[string]any {
	cloned := make(map[string]any, len(source)+2)
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
