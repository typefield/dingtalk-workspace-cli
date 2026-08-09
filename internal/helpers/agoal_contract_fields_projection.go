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

const agoalContractFieldsTool = "list_op_contract_fields"

// runAgoalContractFields executes the business request exactly once. During
// dual validation the historical writer remains authoritative while the same
// response is projected and validated for the unified contract.
func runAgoalContractFields(cmd *cobra.Command, toolArgs map[string]any) error {
	if deps.Caller.DryRun() {
		result := output.Success(map[string]any{
			"tool":        agoalContractFieldsTool,
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
		return callMCPTool(agoalContractFieldsTool, toolArgs)
	}

	payload, err := CallMCPToolPayloadOnServer(cmd.Context(), "agoal", agoalContractFieldsTool, toolArgs)
	if err != nil {
		return err
	}
	data, meta, err := projectAgoalContractFields(payload.Data)
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
	return WriteMCPToolPayloadLegacy("agoal", agoalContractFieldsTool, payload)
}

func projectAgoalContractFields(raw any) (map[string]any, *output.Meta, error) {
	root, ok := raw.(map[string]any)
	if !ok {
		return nil, nil, agoalContractFieldsProjectionUnknown(fmt.Sprintf("response must be an object, got %T", raw))
	}
	if err := agoalContractFieldsExactKeys("response", root, "code", "content", "message", "requestId", "success"); err != nil {
		return nil, nil, err
	}
	if success, ok := root["success"].(bool); !ok || !success {
		return nil, nil, agoalContractFieldsProjectionUnknown("response.success must be boolean true")
	}
	if !agoalNilOrZero(root["code"]) {
		return nil, nil, agoalContractFieldsProjectionUnknown("response.code must be null or integer zero")
	}
	if root["message"] != nil {
		if _, ok := root["message"].(string); !ok {
			return nil, nil, agoalContractFieldsProjectionUnknown("response.message must be null or string")
		}
	}
	if _, err := agoalContractFieldsRequiredString("response.requestId", root["requestId"]); err != nil {
		return nil, nil, err
	}
	fields, err := projectAgoalContractFieldRows(root["content"])
	if err != nil {
		return nil, nil, err
	}
	return map[string]any{
		"fields":             fields,
		"fieldCoverageKnown": false,
	}, &output.Meta{Count: output.NewCount(len(fields))}, nil
}

func projectAgoalContractFieldRows(value any) ([]map[string]any, error) {
	rows, ok := value.([]any)
	if !ok {
		return nil, agoalContractFieldsProjectionUnknown(fmt.Sprintf("response.content must be an array, got %T", value))
	}
	out := make([]map[string]any, 0, len(rows))
	seenIDs := make(map[string]struct{}, len(rows))
	seenCodes := make(map[string]struct{}, len(rows))
	for index, value := range rows {
		path := fmt.Sprintf("response.content[%d]", index)
		row, ok := value.(map[string]any)
		if !ok {
			return nil, agoalContractFieldsProjectionUnknown(path + " must be an object")
		}
		if err := agoalContractFieldsExactKeys(path, row,
			"active", "category", "code", "forceActive", "forceRequired", "id",
			"required", "scheme", "source", "title", "type"); err != nil {
			return nil, err
		}
		fieldID, err := agoalContractFieldsRequiredString(path+".id", row["id"])
		if err != nil {
			return nil, err
		}
		if _, duplicate := seenIDs[fieldID]; duplicate {
			return nil, agoalContractFieldsProjectionUnknown(fmt.Sprintf("response.content contains duplicate id %q", fieldID))
		}
		seenIDs[fieldID] = struct{}{}
		code, err := agoalContractFieldsRequiredString(path+".code", row["code"])
		if err != nil {
			return nil, err
		}
		if _, duplicate := seenCodes[code]; duplicate {
			return nil, agoalContractFieldsProjectionUnknown(fmt.Sprintf("response.content contains duplicate code %q", code))
		}
		seenCodes[code] = struct{}{}

		projected := map[string]any{"fieldId": fieldID, "code": code}
		for _, key := range []string{"title", "category", "type"} {
			text, err := agoalContractFieldsRequiredString(path+"."+key, row[key])
			if err != nil {
				return nil, err
			}
			projected[key] = text
		}
		for _, key := range []string{"active", "required", "forceActive", "forceRequired"} {
			flag, ok := row[key].(bool)
			if !ok {
				return nil, agoalContractFieldsProjectionUnknown(fmt.Sprintf("%s.%s must be a boolean, got %T", path, key, row[key]))
			}
			projected[key] = flag
		}
		if row["source"] != nil {
			return nil, agoalContractFieldsProjectionUnknown(path + ".source must be null in the reviewed shape")
		}
		if err := validateAgoalContractFieldScheme(path+".scheme", row["scheme"]); err != nil {
			return nil, err
		}
		out = append(out, projected)
	}
	return out, nil
}

// Scheme currently contains presentation-only width/format hints. Validate
// the observed shape to detect service drift, but do not expose those layout
// details as business field semantics.
func validateAgoalContractFieldScheme(path string, value any) error {
	scheme, ok := value.(map[string]any)
	if !ok {
		return agoalContractFieldsProjectionUnknown(fmt.Sprintf("%s must be an object, got %T", path, value))
	}
	if _, exists := scheme["width"]; !exists {
		return agoalContractFieldsProjectionUnknown(path + " is missing \"width\"")
	}
	for key := range scheme {
		if key != "width" && key != "format" {
			return agoalContractFieldsProjectionUnknown(fmt.Sprintf("%s contains unknown field %q", path, key))
		}
	}
	width, ok := agoalExactNonNegativeInteger(scheme["width"])
	if !ok || width < 1 {
		return agoalContractFieldsProjectionUnknown(path + ".width must be an exact positive integer")
	}
	if format, exists := scheme["format"]; exists {
		if _, err := agoalContractFieldsRequiredString(path+".format", format); err != nil {
			return err
		}
	}
	return nil
}

func agoalContractFieldsExactKeys(path string, object map[string]any, keys ...string) error {
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
		if _, exists := object[key]; !exists {
			return agoalContractFieldsProjectionUnknown(fmt.Sprintf("%s is missing %q", path, key))
		}
	}
	for key := range object {
		if _, exists := allowed[key]; !exists {
			return agoalContractFieldsProjectionUnknown(fmt.Sprintf("%s contains unknown field %q", path, key))
		}
	}
	return nil
}

func agoalContractFieldsRequiredString(path string, value any) (string, error) {
	text, ok := value.(string)
	text = strings.TrimSpace(text)
	if !ok || text == "" {
		return "", agoalContractFieldsProjectionUnknown(fmt.Sprintf("%s must be a non-empty string", path))
	}
	return text, nil
}

func agoalContractFieldsProjectionUnknown(message string) error {
	return apperrors.NewAPI(agoalContractFieldsTool+" response cannot be projected safely: "+message,
		apperrors.WithOperation(agoalContractFieldsTool),
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

func agoalContractFieldsResultSpec() *contract.ResultSpec {
	fieldSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fieldId":       map[string]any{"type": "string", "minLength": 1},
			"code":          map[string]any{"type": "string", "minLength": 1},
			"title":         map[string]any{"type": "string", "minLength": 1},
			"category":      map[string]any{"type": "string", "minLength": 1},
			"type":          map[string]any{"type": "string", "minLength": 1},
			"active":        map[string]any{"type": "boolean"},
			"required":      map[string]any{"type": "boolean"},
			"forceActive":   map[string]any{"type": "boolean"},
			"forceRequired": map[string]any{"type": "boolean"},
		},
		"required": []string{
			"fieldId", "code", "title", "category", "type",
			"active", "required", "forceActive", "forceRequired",
		},
		"additionalProperties": false,
	}
	dataSchema, err := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fields":             map[string]any{"type": "array", "items": fieldSchema},
			"fieldCoverageKnown": map[string]any{"type": "boolean", "const": false},
		},
		"required":             []string{"fields", "fieldCoverageKnown"},
		"additionalProperties": false,
	})
	if err != nil {
		panic(err)
	}
	recordSchema, err := json.Marshal(fieldSchema)
	if err != nil {
		panic(err)
	}
	return &contract.ResultSpec{
		Outcomes:   []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: dataSchema,
		NDJSON:     &contract.ResultNDJSONSpec{RecordPath: "fields", RecordSchema: recordSchema},
	}
}
