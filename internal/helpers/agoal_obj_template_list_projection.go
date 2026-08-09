// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/spf13/cobra"
)

const agoalObjTemplateListTool = "list_obj_template"

func runAgoalObjTemplateList(cmd *cobra.Command, toolArgs map[string]any) error {
	if deps.Caller.DryRun() {
		result := output.Success(map[string]any{
			"tool":        agoalObjTemplateListTool,
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
		return callMCPTool(agoalObjTemplateListTool, toolArgs)
	}
	payload, err := CallMCPToolPayloadOnServer(cmd.Context(), "agoal", agoalObjTemplateListTool, toolArgs)
	if err != nil {
		return err
	}
	data, meta, err := projectAgoalObjTemplateList(payload.Data)
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
	return WriteMCPToolPayloadLegacy("agoal", agoalObjTemplateListTool, payload)
}

func projectAgoalObjTemplateList(raw any) (map[string]any, *output.Meta, error) {
	root, ok := raw.(map[string]any)
	if !ok {
		return nil, nil, agoalObjTemplateProjectionUnknown(fmt.Sprintf("response must be an object, got %T", raw))
	}
	if err := agoalObjTemplateExactKeys("response", root, "code", "content", "message", "requestId", "success"); err != nil {
		return nil, nil, err
	}
	if success, ok := root["success"].(bool); !ok || !success {
		return nil, nil, agoalObjTemplateProjectionUnknown("response.success must be boolean true")
	}
	if !agoalNilOrZero(root["code"]) {
		return nil, nil, agoalObjTemplateProjectionUnknown("response.code must be null or integer zero")
	}
	if root["message"] != nil {
		if _, ok := root["message"].(string); !ok {
			return nil, nil, agoalObjTemplateProjectionUnknown("response.message must be null or string")
		}
	}
	if _, err := agoalObjTemplateRequiredString("response.requestId", root["requestId"]); err != nil {
		return nil, nil, err
	}
	content, ok := root["content"].(map[string]any)
	if !ok {
		return nil, nil, agoalObjTemplateProjectionUnknown(fmt.Sprintf("response.content must be an object, got %T", root["content"]))
	}
	if err := agoalObjTemplateExactKeys("response.content", content, "page", "pageSize", "result", "totalCount"); err != nil {
		return nil, nil, err
	}
	page, ok := agoalExactNonNegativeInteger(content["page"])
	if !ok || page < 1 {
		return nil, nil, agoalObjTemplateProjectionUnknown("response.content.page must be an exact integer at least 1")
	}
	pageSize, ok := agoalExactNonNegativeInteger(content["pageSize"])
	if !ok || pageSize < 1 {
		return nil, nil, agoalObjTemplateProjectionUnknown("response.content.pageSize must be an exact integer at least 1")
	}
	totalCount, ok := agoalExactNonNegativeInteger(content["totalCount"])
	if !ok {
		return nil, nil, agoalObjTemplateProjectionUnknown("response.content.totalCount must be an exact non-negative integer")
	}
	templates, err := projectAgoalObjTemplateRows(content["result"])
	if err != nil {
		return nil, nil, err
	}
	if page-1 > (1<<53-1)/pageSize {
		return nil, nil, agoalObjTemplateProjectionUnknown("page offset exceeds exact integer range")
	}
	offset := (page - 1) * pageSize
	remaining := totalCount - offset
	if remaining < 0 {
		remaining = 0
	}
	expected := pageSize
	if remaining < expected {
		expected = remaining
	}
	if int64(len(templates)) != expected {
		return nil, nil, agoalObjTemplateProjectionUnknown(fmt.Sprintf("response.content.result count %d contradicts page/pageSize/totalCount (expected %d)", len(templates), expected))
	}
	exhausted := offset+int64(len(templates)) >= totalCount
	nextToken := ""
	if !exhausted {
		nextToken = strconv.FormatInt(page+1, 10)
	}
	pagination, err := output.NewPagination(exhausted, nextToken)
	if err != nil {
		return nil, nil, agoalObjTemplateProjectionUnknown(err.Error())
	}
	pagination.Pages = 1
	pagination.Items = len(templates)
	data := map[string]any{
		"templates":              templates,
		"totalCount":             totalCount,
		"authoritativeInventory": false,
		"inventoryCoverageKnown": false,
	}
	return data, &output.Meta{Count: output.NewCount(len(templates)), Pagination: pagination}, nil
}

func projectAgoalObjTemplateRows(value any) ([]map[string]any, error) {
	rows, ok := value.([]any)
	if !ok {
		return nil, agoalObjTemplateProjectionUnknown(fmt.Sprintf("response.content.result must be an array, got %T", value))
	}
	out := make([]map[string]any, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for index, value := range rows {
		path := fmt.Sprintf("response.content.result[%d]", index)
		row, ok := value.(map[string]any)
		if !ok {
			return nil, agoalObjTemplateProjectionUnknown(path + " must be an object")
		}
		if err := agoalObjTemplateExactKeys(path, row,
			"computeByWeight", "creator", "dimensionWeight", "dimensions", "id", "objectiveCategory",
			"objectiveWeight", "status", "title", "type"); err != nil {
			return nil, err
		}
		id, err := agoalObjTemplateRequiredString(path+".id", row["id"])
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, agoalObjTemplateProjectionUnknown(fmt.Sprintf("response.content.result contains duplicate id %q", id))
		}
		seen[id] = struct{}{}
		projected := map[string]any{"templateId": id}
		for _, key := range []string{"title", "objectiveCategory", "status", "type"} {
			text, err := agoalObjTemplateRequiredString(path+"."+key, row[key])
			if err != nil {
				return nil, err
			}
			projected[key] = text
		}
		for _, key := range []string{"computeByWeight", "dimensionWeight", "objectiveWeight"} {
			flag, ok := row[key].(bool)
			if !ok {
				return nil, agoalObjTemplateProjectionUnknown(fmt.Sprintf("%s.%s must be a boolean, got %T", path, key, row[key]))
			}
			projected[key] = flag
		}
		if _, ok := row["creator"].(map[string]any); !ok {
			return nil, agoalObjTemplateProjectionUnknown(fmt.Sprintf("%s.creator must be an object, got %T", path, row["creator"]))
		}
		if _, ok := row["dimensions"].([]any); !ok {
			return nil, agoalObjTemplateProjectionUnknown(fmt.Sprintf("%s.dimensions must be an array, got %T", path, row["dimensions"]))
		}
		out = append(out, projected)
	}
	return out, nil
}

func agoalObjTemplateExactKeys(path string, object map[string]any, keys ...string) error {
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
		if _, exists := object[key]; !exists {
			return agoalObjTemplateProjectionUnknown(fmt.Sprintf("%s is missing %q", path, key))
		}
	}
	for key := range object {
		if _, exists := allowed[key]; !exists {
			return agoalObjTemplateProjectionUnknown(fmt.Sprintf("%s contains unknown field %q", path, key))
		}
	}
	return nil
}

func agoalObjTemplateRequiredString(path string, value any) (string, error) {
	text, ok := value.(string)
	text = strings.TrimSpace(text)
	if !ok || text == "" {
		return "", agoalObjTemplateProjectionUnknown(fmt.Sprintf("%s must be a non-empty string", path))
	}
	return text, nil
}

func agoalObjTemplateProjectionUnknown(message string) error {
	return apperrors.NewAPI(agoalObjTemplateListTool+" response cannot be projected safely: "+message,
		apperrors.WithOperation(agoalObjTemplateListTool),
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

func agoalObjTemplateListResultSpec() *contract.ResultSpec {
	templateSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"templateId":        map[string]any{"type": "string", "minLength": 1},
			"title":             map[string]any{"type": "string", "minLength": 1},
			"objectiveCategory": map[string]any{"type": "string", "minLength": 1},
			"status":            map[string]any{"type": "string", "minLength": 1},
			"type":              map[string]any{"type": "string", "minLength": 1},
			"computeByWeight":   map[string]any{"type": "boolean"},
			"dimensionWeight":   map[string]any{"type": "boolean"},
			"objectiveWeight":   map[string]any{"type": "boolean"},
		},
		"required":             []string{"templateId", "title", "objectiveCategory", "status", "type", "computeByWeight", "dimensionWeight", "objectiveWeight"},
		"additionalProperties": false,
	}
	dataSchema, err := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"templates":              map[string]any{"type": "array", "items": templateSchema},
			"totalCount":             map[string]any{"type": "integer", "minimum": 0},
			"authoritativeInventory": map[string]any{"type": "boolean", "const": false},
			"inventoryCoverageKnown": map[string]any{"type": "boolean", "const": false},
		},
		"required":             []string{"templates", "totalCount", "authoritativeInventory", "inventoryCoverageKnown"},
		"additionalProperties": false,
	})
	if err != nil {
		panic(err)
	}
	recordSchema, err := json.Marshal(templateSchema)
	if err != nil {
		panic(err)
	}
	return &contract.ResultSpec{
		Outcomes:   []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: dataSchema,
		NDJSON:     &contract.ResultNDJSONSpec{RecordPath: "templates", RecordSchema: recordSchema},
	}
}
