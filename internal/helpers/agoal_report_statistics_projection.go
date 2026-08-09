// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/spf13/cobra"
)

const agoalReportStatisticsTool = "list_report_statistics"

// runAgoalReportStatistics executes the MCP operation exactly once. During
// dual validation the historical renderer remains byte-for-byte authoritative;
// unified_active stores the already validated projection for the root emitter.
func runAgoalReportStatistics(cmd *cobra.Command, toolArgs map[string]any) error {
	if deps.Caller.DryRun() {
		result := output.Success(map[string]any{
			"tool":        agoalReportStatisticsTool,
			"arguments":   toolArgs,
			"executed":    false,
			"readPreview": true,
		}, output.WithDryRun())
		if output.UsesUnifiedResult(cmd) {
			return output.StoreResult(cmd.Context(), result)
		}
		if output.CommandRollout(cmd) == output.RolloutDualValidate {
			if err := output.ValidateResult(result); err != nil {
				return err
			}
		}
		return callMCPTool(agoalReportStatisticsTool, toolArgs)
	}

	payload, err := CallMCPToolPayloadOnServer(cmd.Context(), "agoal", agoalReportStatisticsTool, toolArgs)
	if err != nil {
		return err
	}
	data, meta, err := projectAgoalReportStatistics(payload.Data)
	if err != nil {
		return err
	}
	result := output.Success(data, output.WithMeta(meta))
	if output.UsesUnifiedResult(cmd) {
		return output.StoreResult(cmd.Context(), result)
	}
	if output.CommandRollout(cmd) == output.RolloutDualValidate {
		if err := output.ValidateResult(result); err != nil {
			return err
		}
	}
	return WriteMCPToolPayloadLegacy("agoal", agoalReportStatisticsTool, payload)
}

func projectAgoalReportStatistics(raw any) (map[string]any, *output.Meta, error) {
	root, ok := raw.(map[string]any)
	if !ok {
		return nil, nil, agoalReportProjectionUnknown(fmt.Sprintf("response must be an object, got %T", raw))
	}
	if err := agoalReportExactKeys("response", root, "code", "content", "message", "requestId", "success"); err != nil {
		return nil, nil, err
	}
	if success, ok := root["success"].(bool); !ok || !success {
		return nil, nil, agoalReportProjectionUnknown("response.success must be boolean true")
	}
	if !agoalNilOrZero(root["code"]) {
		return nil, nil, agoalReportProjectionUnknown(fmt.Sprintf("response.code must be null or integer zero, got %T", root["code"]))
	}
	if root["message"] != nil {
		if _, ok := root["message"].(string); !ok {
			return nil, nil, agoalReportProjectionUnknown(fmt.Sprintf("response.message must be null or string, got %T", root["message"]))
		}
	}
	if _, err := agoalReportRequiredString("response.requestId", root["requestId"]); err != nil {
		return nil, nil, err
	}
	reports, err := projectAgoalReportRows(root["content"])
	if err != nil {
		return nil, nil, err
	}
	return map[string]any{
		"reports":             reports,
		"reportCoverageKnown": false,
	}, &output.Meta{Count: output.NewCount(len(reports))}, nil
}

func projectAgoalReportRows(value any) ([]map[string]any, error) {
	rows, ok := value.([]any)
	if !ok {
		return nil, agoalReportProjectionUnknown(fmt.Sprintf("response.content must be an array, got %T", value))
	}
	out := make([]map[string]any, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for index, value := range rows {
		path := fmt.Sprintf("response.content[%d]", index)
		row, ok := value.(map[string]any)
		if !ok {
			return nil, agoalReportProjectionUnknown(path + " must be an object")
		}
		if err := agoalReportExactKeys(path, row,
			"allowTimeout", "content", "deadline", "enableStatistic", "lastModifiedFormat",
			"lastModifier", "late", "notSubmitted", "onTime", "preferTime", "remind",
			"remindSize", "reportType", "status", "templateId", "timeoutMinutes", "title",
			"viewPermission"); err != nil {
			return nil, err
		}
		templateID, err := agoalReportRequiredString(path+".templateId", row["templateId"])
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[templateID]; duplicate {
			return nil, agoalReportProjectionUnknown(fmt.Sprintf("response.content contains duplicate templateId %q", templateID))
		}
		seen[templateID] = struct{}{}
		projected := map[string]any{"templateId": templateID}
		for _, key := range []string{"title", "reportType", "status"} {
			text, err := agoalReportRequiredString(path+"."+key, row[key])
			if err != nil {
				return nil, err
			}
			projected[key] = text
		}
		for _, key := range []string{"allowTimeout", "enableStatistic"} {
			flag, ok := row[key].(bool)
			if !ok {
				return nil, agoalReportProjectionUnknown(fmt.Sprintf("%s.%s must be a boolean, got %T", path, key, row[key]))
			}
			projected[key] = flag
		}
		counts := make(map[string]int64, 5)
		for _, key := range []string{"onTime", "late", "notSubmitted", "remindSize", "timeoutMinutes"} {
			number, ok := agoalExactNonNegativeInteger(row[key])
			if !ok {
				return nil, agoalReportProjectionUnknown(fmt.Sprintf("%s.%s must be an exact non-negative integer", path, key))
			}
			counts[key] = number
			projected[key] = number
		}
		if counts["remindSize"] != counts["onTime"]+counts["late"]+counts["notSubmitted"] {
			return nil, agoalReportProjectionUnknown(path + ".remindSize must equal onTime + late + notSubmitted")
		}
		if err := validateAgoalReportOpaqueFields(path, row); err != nil {
			return nil, err
		}
		permission, err := projectAgoalReportViewPermission(path+".viewPermission", row["viewPermission"])
		if err != nil {
			return nil, err
		}
		projected["viewPermission"] = permission
		out = append(out, projected)
	}
	return out, nil
}

// validateAgoalReportOpaqueFields validates observed service fields that are
// intentionally not published. This keeps HTML and modifier identities out of
// the Agent result while still failing closed when their service shape drifts.
func validateAgoalReportOpaqueFields(path string, row map[string]any) error {
	if _, err := agoalReportRequiredString(path+".lastModifiedFormat", row["lastModifiedFormat"]); err != nil {
		return err
	}
	for _, key := range []string{"deadline", "preferTime", "remind"} {
		object, ok := row[key].(map[string]any)
		if !ok || len(object) != 0 {
			return agoalReportProjectionUnknown(fmt.Sprintf("%s.%s must be the reviewed empty object shape", path, key))
		}
	}
	modifier, ok := row["lastModifier"].(map[string]any)
	if !ok {
		return agoalReportProjectionUnknown(fmt.Sprintf("%s.lastModifier must be an object, got %T", path, row["lastModifier"]))
	}
	if err := agoalReportExactKeys(path+".lastModifier", modifier, "dingUserId", "id", "name", "workNo"); err != nil {
		return err
	}
	for _, key := range []string{"dingUserId", "id", "name"} {
		if _, err := agoalReportRequiredString(path+".lastModifier."+key, modifier[key]); err != nil {
			return err
		}
	}
	if modifier["workNo"] != nil {
		if _, ok := modifier["workNo"].(string); !ok {
			return agoalReportProjectionUnknown(path + ".lastModifier.workNo must be null or string")
		}
	}
	items, ok := row["content"].([]any)
	if !ok {
		return agoalReportProjectionUnknown(fmt.Sprintf("%s.content must be an array, got %T", path, row["content"]))
	}
	seen := make(map[string]struct{}, len(items))
	for index, value := range items {
		itemPath := fmt.Sprintf("%s.content[%d]", path, index)
		item, ok := value.(map[string]any)
		if !ok {
			return agoalReportProjectionUnknown(itemPath + " must be an object")
		}
		if err := agoalReportRequiredKeys(itemPath, item, "id", "name", "type"); err != nil {
			return err
		}
		id, err := agoalReportRequiredString(itemPath+".id", item["id"])
		if err != nil {
			return err
		}
		if _, duplicate := seen[id]; duplicate {
			return agoalReportProjectionUnknown(fmt.Sprintf("%s.content contains duplicate id %q", path, id))
		}
		seen[id] = struct{}{}
		if _, err := agoalReportRequiredString(itemPath+".name", item["name"]); err != nil {
			return err
		}
		typeName, err := agoalReportRequiredString(itemPath+".type", item["type"])
		if err != nil {
			return err
		}
		switch typeName {
		case "Objective":
			if err := agoalReportExactKeys(itemPath, item, "id", "name", "type"); err != nil {
				return err
			}
		case "Text":
			if err := agoalReportExactKeys(itemPath, item, "id", "name", "type", "value"); err != nil {
				return err
			}
			textValue, ok := item["value"].(map[string]any)
			if !ok {
				return agoalReportProjectionUnknown(itemPath + ".value must be an object for Text")
			}
			if err := agoalReportExactKeys(itemPath+".value", textValue, "asl", "editablePlaceholder", "html", "readonlyPlaceholder"); err != nil {
				return err
			}
			for _, key := range []string{"asl", "editablePlaceholder", "html", "readonlyPlaceholder"} {
				if _, ok := textValue[key].(string); !ok {
					return agoalReportProjectionUnknown(fmt.Sprintf("%s.value.%s must be a string", itemPath, key))
				}
			}
		default:
			return agoalReportProjectionUnknown(fmt.Sprintf("%s.type %q is not reviewed", itemPath, typeName))
		}
	}
	return nil
}

func projectAgoalReportViewPermission(path string, value any) (map[string]any, error) {
	permission, ok := value.(map[string]any)
	if !ok {
		return nil, agoalReportProjectionUnknown(fmt.Sprintf("%s must be an object, got %T", path, value))
	}
	if err := agoalReportExactKeys(path, permission, "deptReportLineManager", "sameDeptColleague", "userReportLineManager"); err != nil {
		return nil, err
	}
	out := make(map[string]any, 3)
	for _, key := range []string{"deptReportLineManager", "sameDeptColleague", "userReportLineManager"} {
		flag, ok := permission[key].(bool)
		if !ok {
			return nil, agoalReportProjectionUnknown(fmt.Sprintf("%s.%s must be a boolean, got %T", path, key, permission[key]))
		}
		out[key] = flag
	}
	return out, nil
}

func agoalReportExactKeys(path string, object map[string]any, keys ...string) error {
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
		if _, exists := object[key]; !exists {
			return agoalReportProjectionUnknown(fmt.Sprintf("%s is missing %q", path, key))
		}
	}
	for key := range object {
		if _, exists := allowed[key]; !exists {
			return agoalReportProjectionUnknown(fmt.Sprintf("%s contains unknown field %q", path, key))
		}
	}
	return nil
}

func agoalReportRequiredKeys(path string, object map[string]any, keys ...string) error {
	for _, key := range keys {
		if _, exists := object[key]; !exists {
			return agoalReportProjectionUnknown(fmt.Sprintf("%s is missing %q", path, key))
		}
	}
	return nil
}

func agoalReportRequiredString(path string, value any) (string, error) {
	text, ok := value.(string)
	text = strings.TrimSpace(text)
	if !ok || text == "" {
		return "", agoalReportProjectionUnknown(fmt.Sprintf("%s must be a non-empty string", path))
	}
	return text, nil
}

func agoalReportProjectionUnknown(message string) error {
	return apperrors.NewAPI(agoalReportStatisticsTool+" response cannot be projected safely: "+message,
		apperrors.WithOperation(agoalReportStatisticsTool),
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

func agoalReportStatisticsResultSpec() *contract.ResultSpec {
	permissionSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"deptReportLineManager": map[string]any{"type": "boolean"},
			"sameDeptColleague":     map[string]any{"type": "boolean"},
			"userReportLineManager": map[string]any{"type": "boolean"},
		},
		"required":             []string{"deptReportLineManager", "sameDeptColleague", "userReportLineManager"},
		"additionalProperties": false,
	}
	reportSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"templateId":      map[string]any{"type": "string", "minLength": 1},
			"title":           map[string]any{"type": "string", "minLength": 1},
			"reportType":      map[string]any{"type": "string", "minLength": 1},
			"status":          map[string]any{"type": "string", "minLength": 1},
			"allowTimeout":    map[string]any{"type": "boolean"},
			"enableStatistic": map[string]any{"type": "boolean"},
			"onTime":          map[string]any{"type": "integer", "minimum": 0},
			"late":            map[string]any{"type": "integer", "minimum": 0},
			"notSubmitted":    map[string]any{"type": "integer", "minimum": 0},
			"remindSize":      map[string]any{"type": "integer", "minimum": 0},
			"timeoutMinutes":  map[string]any{"type": "integer", "minimum": 0},
			"viewPermission":  permissionSchema,
		},
		"required": []string{
			"templateId", "title", "reportType", "status", "allowTimeout", "enableStatistic",
			"onTime", "late", "notSubmitted", "remindSize", "timeoutMinutes", "viewPermission",
		},
		"additionalProperties": false,
	}
	dataSchema, err := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"reports":             map[string]any{"type": "array", "items": reportSchema},
			"reportCoverageKnown": map[string]any{"type": "boolean", "const": false},
		},
		"required":             []string{"reports", "reportCoverageKnown"},
		"additionalProperties": false,
	})
	if err != nil {
		panic(err)
	}
	recordSchema, err := json.Marshal(reportSchema)
	if err != nil {
		panic(err)
	}
	return &contract.ResultSpec{
		Outcomes:   []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: dataSchema,
		NDJSON:     &contract.ResultNDJSONSpec{RecordPath: "reports", RecordSchema: recordSchema},
	}
}
