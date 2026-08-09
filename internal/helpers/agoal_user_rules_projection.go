// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/spf13/cobra"
)

const agoalUserRulesTool = "get_user_rules"

// runAgoalUserRules is the per-terminal rollout seam for the first reviewed
// Agoal leaf. The MCP request is executed exactly once. dual_validate keeps
// the historical renderer byte-for-byte while validating the shadow unified
// result; unified_active stores that same result for the root lifecycle.
func runAgoalUserRules(cmd *cobra.Command, toolArgs map[string]any) error {
	if deps.Caller.DryRun() {
		result := output.Success(map[string]any{
			"tool":        agoalUserRulesTool,
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
		// Reuse the old dry-run renderer. callMCPTool observes Caller.DryRun and
		// therefore performs no business request.
		return callMCPTool(agoalUserRulesTool, toolArgs)
	}

	payload, err := CallMCPToolPayloadOnServer(cmd.Context(), "agoal", agoalUserRulesTool, toolArgs)
	if err != nil {
		return err
	}
	data, meta, err := projectAgoalUserRules(payload.Data)
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
	return WriteMCPToolPayloadLegacy("agoal", agoalUserRulesTool, payload)
}

func projectAgoalUserRules(raw any) (map[string]any, *output.Meta, error) {
	root, ok := raw.(map[string]any)
	if !ok {
		return nil, nil, agoalRulesProjectionUnknown(fmt.Sprintf("response must be an object, got %T", raw))
	}
	if err := agoalExactKeys("response", root, "code", "content", "message", "requestId", "success"); err != nil {
		return nil, nil, err
	}
	if success, ok := root["success"].(bool); !ok || !success {
		return nil, nil, agoalRulesProjectionUnknown("response.success must be boolean true")
	}
	if !agoalNilOrZero(root["code"]) {
		return nil, nil, agoalRulesProjectionUnknown(fmt.Sprintf("response.code must be null or integer zero, got %T", root["code"]))
	}
	if root["message"] != nil {
		if _, ok := root["message"].(string); !ok {
			return nil, nil, agoalRulesProjectionUnknown(fmt.Sprintf("response.message must be null or string, got %T", root["message"]))
		}
	}
	if _, err := agoalRequiredString("response.requestId", root["requestId"]); err != nil {
		return nil, nil, err
	}
	content, ok := root["content"].(map[string]any)
	if !ok {
		return nil, nil, agoalRulesProjectionUnknown(fmt.Sprintf("response.content must be an object, got %T", root["content"]))
	}
	if err := agoalExactKeys("response.content", content, "preference", "rules"); err != nil {
		return nil, nil, err
	}
	preference, err := projectAgoalPreference(content["preference"])
	if err != nil {
		return nil, nil, err
	}
	rules, err := projectAgoalRules(content["rules"])
	if err != nil {
		return nil, nil, err
	}
	data := map[string]any{
		"rules":             rules,
		"preference":        preference,
		"ruleCoverageKnown": false,
	}
	return data, &output.Meta{Count: output.NewCount(len(rules))}, nil
}

func projectAgoalPreference(value any) (map[string]any, error) {
	preference, ok := value.(map[string]any)
	if !ok {
		return nil, agoalRulesProjectionUnknown(fmt.Sprintf("content.preference must be an object, got %T", value))
	}
	if err := agoalExactKeys("content.preference", preference, "perfTaskId", "periodId", "ruleId"); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, key := range []string{"perfTaskId", "periodId", "ruleId"} {
		if preference[key] == nil {
			continue
		}
		text, err := agoalRequiredString("content.preference."+key, preference[key])
		if err != nil {
			return nil, err
		}
		out[key] = text
	}
	return out, nil
}

func projectAgoalRules(value any) ([]map[string]any, error) {
	rows, ok := value.([]any)
	if !ok {
		return nil, agoalRulesProjectionUnknown(fmt.Sprintf("content.rules must be an array, got %T", value))
	}
	out := make([]map[string]any, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for index, value := range rows {
		path := fmt.Sprintf("content.rules[%d]", index)
		row, ok := value.(map[string]any)
		if !ok {
			return nil, agoalRulesProjectionUnknown(path + " must be an object")
		}
		if err := agoalExactKeys(path, row,
			"canRelatedUsers", "category", "history", "id", "lastModified", "matchedCount",
			"perfTaskFilter", "periodFilter", "reviewTag", "ruleContent", "ruleName", "status",
			"type", "weightCheckTag"); err != nil {
			return nil, err
		}
		ruleID, err := agoalRequiredString(path+".id", row["id"])
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[ruleID]; duplicate {
			return nil, agoalRulesProjectionUnknown(fmt.Sprintf("content.rules contains duplicate id %q", ruleID))
		}
		seen[ruleID] = struct{}{}
		ruleName, err := agoalRequiredString(path+".ruleName", row["ruleName"])
		if err != nil {
			return nil, err
		}
		projected := map[string]any{"ruleId": ruleID, "ruleName": ruleName}
		for _, key := range []string{"category", "reviewTag", "status", "type"} {
			text, ok := row[key].(string)
			if !ok {
				return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s.%s must be a string, got %T", path, key, row[key]))
			}
			projected[key] = text
		}
		for _, key := range []string{"canRelatedUsers", "history", "weightCheckTag"} {
			flag, ok := row[key].(bool)
			if !ok {
				return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s.%s must be a boolean, got %T", path, key, row[key]))
			}
			projected[key] = flag
		}
		for _, key := range []string{"lastModified", "matchedCount"} {
			number, ok := agoalExactNonNegativeInteger(row[key])
			if !ok {
				return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s.%s must be an exact non-negative integer", path, key))
			}
			projected[key] = number
		}
		if row["ruleContent"] != nil {
			return nil, agoalRulesProjectionUnknown(path + ".ruleContent has an unreviewed non-null shape")
		}
		performanceTasks, err := projectAgoalPerformanceTasks(path+".perfTaskFilter", row["perfTaskFilter"])
		if err != nil {
			return nil, err
		}
		periods, err := projectAgoalPeriodFilter(path+".periodFilter", row["periodFilter"])
		if err != nil {
			return nil, err
		}
		projected["performanceTasks"] = performanceTasks
		projected["periods"] = periods
		out = append(out, projected)
	}
	return out, nil
}

func projectAgoalPerformanceTasks(path string, value any) (map[string]any, error) {
	filter, ok := value.(map[string]any)
	if !ok {
		return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s must be an object, got %T", path, value))
	}
	if err := agoalExactKeys(path, filter, "currentPerfTasks", "historyPerfTasks"); err != nil {
		return nil, err
	}
	out := map[string]any{"current": []map[string]any{}, "history": []map[string]any{}}
	for source, target := range map[string]string{"currentPerfTasks": "current", "historyPerfTasks": "history"} {
		rows, ok := filter[source].([]any)
		if !ok {
			return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s.%s must be an array, got %T", path, source, filter[source]))
		}
		if len(rows) != 0 {
			return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s.%s contains an unreviewed non-empty row shape", path, source))
		}
		out[target] = []map[string]any{}
	}
	return out, nil
}

func projectAgoalPeriodFilter(path string, value any) (map[string]any, error) {
	filter, ok := value.(map[string]any)
	if !ok {
		return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s must be an object, got %T", path, value))
	}
	if err := agoalExactKeys(path, filter,
		"currentPeriods", "defaultPeriodIds", "historyPeriods", "lastObjectivePeriodId", "preferPeriodIds"); err != nil {
		return nil, err
	}
	current, currentIDs, err := projectAgoalPeriods(path+".currentPeriods", filter["currentPeriods"])
	if err != nil {
		return nil, err
	}
	history, historyIDs, err := projectAgoalPeriods(path+".historyPeriods", filter["historyPeriods"])
	if err != nil {
		return nil, err
	}
	known := make(map[string]struct{}, len(currentIDs)+len(historyIDs))
	for id := range currentIDs {
		known[id] = struct{}{}
	}
	for id := range historyIDs {
		if _, duplicate := known[id]; duplicate {
			return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s repeats period id %q across current/history", path, id))
		}
		known[id] = struct{}{}
	}
	defaults, err := projectAgoalPeriodIDs(path+".defaultPeriodIds", filter["defaultPeriodIds"], known)
	if err != nil {
		return nil, err
	}
	preferred, err := projectAgoalPeriodIDs(path+".preferPeriodIds", filter["preferPeriodIds"], known)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"current":            current,
		"history":            history,
		"defaultPeriodIds":   defaults,
		"preferredPeriodIds": preferred,
	}
	if filter["lastObjectivePeriodId"] != nil {
		last, ok := filter["lastObjectivePeriodId"].(string)
		if !ok {
			return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s.lastObjectivePeriodId must be null or string, got %T", path, filter["lastObjectivePeriodId"]))
		}
		last = strings.TrimSpace(last)
		if last != "" {
			if _, exists := known[last]; !exists {
				return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s.lastObjectivePeriodId references unknown period %q", path, last))
			}
			out["lastObjectivePeriodId"] = last
		}
	}
	return out, nil
}

func projectAgoalPeriods(path string, value any) ([]map[string]any, map[string]struct{}, error) {
	rows, ok := value.([]any)
	if !ok {
		return nil, nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s must be an array, got %T", path, value))
	}
	out := make([]map[string]any, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for index, value := range rows {
		rowPath := fmt.Sprintf("%s[%d]", path, index)
		row, ok := value.(map[string]any)
		if !ok {
			return nil, nil, agoalRulesProjectionUnknown(rowPath + " must be an object")
		}
		if err := agoalExactKeys(rowPath, row, "endDate", "id", "nameCn", "nameEN", "startDate"); err != nil {
			return nil, nil, err
		}
		id, err := agoalRequiredString(rowPath+".id", row["id"])
		if err != nil {
			return nil, nil, err
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s contains duplicate id %q", path, id))
		}
		seen[id] = struct{}{}
		nameCN, err := agoalRequiredString(rowPath+".nameCn", row["nameCn"])
		if err != nil {
			return nil, nil, err
		}
		nameEN, ok := row["nameEN"].(string)
		if !ok {
			return nil, nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s.nameEN must be a string, got %T", rowPath, row["nameEN"]))
		}
		start, ok := agoalExactNonNegativeInteger(row["startDate"])
		if !ok {
			return nil, nil, agoalRulesProjectionUnknown(rowPath + ".startDate must be an exact non-negative integer")
		}
		end, ok := agoalExactNonNegativeInteger(row["endDate"])
		if !ok || end < start {
			return nil, nil, agoalRulesProjectionUnknown(rowPath + ".endDate must be an exact integer not earlier than startDate")
		}
		out = append(out, map[string]any{
			"periodId": id, "nameCn": nameCN, "nameEn": nameEN,
			"startDate": start, "endDate": end,
		})
	}
	return out, seen, nil
}

func projectAgoalPeriodIDs(path string, value any, known map[string]struct{}) ([]string, error) {
	rows, ok := value.([]any)
	if !ok {
		return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s must be an array, got %T", path, value))
	}
	out := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for index, value := range rows {
		id, err := agoalRequiredString(fmt.Sprintf("%s[%d]", path, index), value)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s contains duplicate id %q", path, id))
		}
		if _, exists := known[id]; !exists {
			return nil, agoalRulesProjectionUnknown(fmt.Sprintf("%s references unknown period %q", path, id))
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func agoalExactKeys(path string, object map[string]any, keys ...string) error {
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
		if _, exists := object[key]; !exists {
			return agoalRulesProjectionUnknown(fmt.Sprintf("%s is missing %q", path, key))
		}
	}
	for key := range object {
		if _, exists := allowed[key]; !exists {
			return agoalRulesProjectionUnknown(fmt.Sprintf("%s contains unknown field %q", path, key))
		}
	}
	return nil
}

func agoalRequiredString(path string, value any) (string, error) {
	text, ok := value.(string)
	text = strings.TrimSpace(text)
	if !ok || text == "" {
		return "", agoalRulesProjectionUnknown(fmt.Sprintf("%s must be a non-empty string", path))
	}
	return text, nil
}

func agoalExactNonNegativeInteger(value any) (int64, bool) {
	number, ok := value.(float64)
	if !ok || number < 0 || math.Trunc(number) != number || number > 1<<53-1 {
		return 0, false
	}
	return int64(number), true
}

func agoalNilOrZero(value any) bool {
	if value == nil {
		return true
	}
	number, ok := agoalExactNonNegativeInteger(value)
	return ok && number == 0
}

func agoalRulesProjectionUnknown(message string) error {
	return apperrors.NewAPI(agoalUserRulesTool+" response cannot be projected safely: "+message,
		apperrors.WithOperation(agoalUserRulesTool),
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

func agoalUserRulesResultSpec() *contract.ResultSpec {
	periodSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"periodId":  map[string]any{"type": "string", "minLength": 1},
			"nameCn":    map[string]any{"type": "string", "minLength": 1},
			"nameEn":    map[string]any{"type": "string"},
			"startDate": map[string]any{"type": "integer", "minimum": 0},
			"endDate":   map[string]any{"type": "integer", "minimum": 0},
		},
		"required":             []string{"periodId", "nameCn", "nameEn", "startDate", "endDate"},
		"additionalProperties": false,
	}
	ruleSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ruleId":          map[string]any{"type": "string", "minLength": 1},
			"ruleName":        map[string]any{"type": "string", "minLength": 1},
			"category":        map[string]any{"type": "string"},
			"reviewTag":       map[string]any{"type": "string"},
			"status":          map[string]any{"type": "string"},
			"type":            map[string]any{"type": "string"},
			"canRelatedUsers": map[string]any{"type": "boolean"},
			"history":         map[string]any{"type": "boolean"},
			"weightCheckTag":  map[string]any{"type": "boolean"},
			"lastModified":    map[string]any{"type": "integer", "minimum": 0},
			"matchedCount":    map[string]any{"type": "integer", "minimum": 0},
			"performanceTasks": map[string]any{
				"type": "object", "properties": map[string]any{
					"current": map[string]any{"type": "array", "maxItems": 0},
					"history": map[string]any{"type": "array", "maxItems": 0},
				}, "required": []string{"current", "history"}, "additionalProperties": false,
			},
			"periods": map[string]any{
				"type": "object", "properties": map[string]any{
					"current":               map[string]any{"type": "array", "items": periodSchema},
					"history":               map[string]any{"type": "array", "items": periodSchema},
					"defaultPeriodIds":      map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}},
					"preferredPeriodIds":    map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}},
					"lastObjectivePeriodId": map[string]any{"type": "string", "minLength": 1},
				}, "required": []string{"current", "history", "defaultPeriodIds", "preferredPeriodIds"}, "additionalProperties": false,
			},
		},
		"required":             []string{"ruleId", "ruleName", "category", "reviewTag", "status", "type", "canRelatedUsers", "history", "weightCheckTag", "lastModified", "matchedCount", "performanceTasks", "periods"},
		"additionalProperties": false,
	}
	dataSchema, err := json.Marshal(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"rules": map[string]any{"type": "array", "items": ruleSchema},
			"preference": map[string]any{
				"type": "object", "properties": map[string]any{
					"perfTaskId": map[string]any{"type": "string", "minLength": 1},
					"periodId":   map[string]any{"type": "string", "minLength": 1},
					"ruleId":     map[string]any{"type": "string", "minLength": 1},
				}, "additionalProperties": false,
			},
			"ruleCoverageKnown": map[string]any{"type": "boolean", "const": false},
		},
		"required":             []string{"rules", "preference", "ruleCoverageKnown"},
		"additionalProperties": false,
	})
	if err != nil {
		panic(err)
	}
	recordSchema, err := json.Marshal(ruleSchema)
	if err != nil {
		panic(err)
	}
	return &contract.ResultSpec{
		Outcomes:   []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: dataSchema,
		NDJSON:     &contract.ResultNDJSONSpec{RecordPath: "rules", RecordSchema: recordSchema},
	}
}
