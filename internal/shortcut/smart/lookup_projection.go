// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package smart

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func lookupResultSpec() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{
			contract.ResultOutcomeSuccess,
			contract.ResultOutcomeFailure,
		},
		DataSchema: json.RawMessage(`{
  "type":"object",
  "properties":{
    "userId":{"type":"string"},
    "name":{"type":"string"},
    "email":{"type":"string"},
    "mobile":{"type":"string"},
    "jobNumber":{"type":"string"},
    "title":{"type":"string"},
    "isAdmin":{"type":"boolean"},
    "organization":{"type":"object","properties":{"id":{"type":["integer","string"]},"name":{"type":"string"},"masterUserId":{"type":"string"},"masterName":{"type":"string"}},"additionalProperties":false},
    "departments":{"type":"array","items":{"type":"object","properties":{"id":{"type":["integer","string"]},"name":{"type":"string"},"path":{"type":"string"}},"required":["id"],"additionalProperties":false}},
    "positions":{"type":"array","items":{"type":"object","properties":{"departmentId":{"type":["integer","string"]},"title":{"type":"string"},"workStation":{"type":"string"},"isMain":{"type":"boolean"},"managerUserId":{"type":"string"},"managerName":{"type":"string"}},"required":["departmentId"],"additionalProperties":false}},
    "labels":{"type":"array","items":{"type":"object","properties":{"id":{"type":["integer","string"]},"name":{"type":"string"}},"required":["id"],"additionalProperties":false}}
  },
  "required":["userId","departments","positions","labels"],
  "additionalProperties":false
}`),
		SensitivePaths: []string{
			"email", "mobile", "jobNumber", "organization.masterUserId", "positions.managerUserId",
		},
	}
}

func lookupCommandResult(tool string, payload any, _ map[string]any, dryRun bool) output.CommandResult {
	if dryRun {
		return output.Success(payload, output.WithDryRun())
	}
	if tool != "get_user_info_by_user_ids" {
		return lookupProjectionFailure("完整资料 shortcut 收到未审阅的终结工具响应")
	}
	profile, err := lookupProjectProfile(payload)
	if err != nil {
		return lookupProjectionFailure(err.Error())
	}
	return output.Success(profile)
}

func lookupProjectProfile(payload any) (map[string]any, error) {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("完整资料响应不是 JSON 对象")
	}
	if err := rejectUnknownKeys(root, "完整资料顶层", "success", "result"); err != nil {
		return nil, err
	}
	if rawSuccess, exists := root["success"]; exists {
		success, ok := rawSuccess.(bool)
		if !ok {
			return nil, fmt.Errorf("完整资料 success 不是布尔值")
		}
		if !success {
			return nil, fmt.Errorf("完整资料业务结果明确失败")
		}
	}
	rows, ok := root["result"].([]any)
	if !ok || len(rows) != 1 {
		return nil, fmt.Errorf("完整资料响应没有唯一用户记录")
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("完整资料用户记录不是对象")
	}
	if err := rejectUnknownKeys(row, "完整资料用户记录", "isAdmin", "orgEmployeeModel"); err != nil {
		return nil, err
	}
	model, ok := row["orgEmployeeModel"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("完整资料缺少 orgEmployeeModel")
	}
	if err := rejectUnknownKeys(model, "orgEmployeeModel",
		"depts", "jobNumber", "labels", "orgAuthEmail", "orgEmail", "email",
		"orgId", "orgMasterDisplayName", "orgMasterUserId", "orgName", "corpName",
		"orgTitle", "orgUserId", "orgUserName", "positions", "orgUserMobile",
		"stateMobile", "mobile", "userId", "userid", "staffId"); err != nil {
		return nil, err
	}

	userID, err := uniqueNonEmptyString(model, "orgUserId", "userId", "userid", "staffId")
	if err != nil || userID == "" {
		return nil, fmt.Errorf("完整资料缺少唯一稳定 userId")
	}
	out := map[string]any{"userId": userID}
	if err := copyStringAlias(out, "name", model, "orgUserName"); err != nil {
		return nil, err
	}
	if err := copyStringAlias(out, "email", model, "orgAuthEmail", "orgEmail", "email"); err != nil {
		return nil, err
	}
	if err := copyStringAlias(out, "mobile", model, "orgUserMobile", "stateMobile", "mobile"); err != nil {
		return nil, err
	}
	if err := copyStringAlias(out, "jobNumber", model, "jobNumber"); err != nil {
		return nil, err
	}
	if err := copyStringAlias(out, "title", model, "orgTitle"); err != nil {
		return nil, err
	}
	if raw, exists := row["isAdmin"]; exists {
		isAdmin, ok := raw.(bool)
		if !ok {
			return nil, fmt.Errorf("完整资料 isAdmin 不是布尔值")
		}
		out["isAdmin"] = isAdmin
	}

	organization, err := lookupProjectOrganization(model)
	if err != nil {
		return nil, err
	}
	if len(organization) > 0 {
		out["organization"] = organization
	}
	departments, err := lookupProjectDepartments(model["depts"])
	if err != nil {
		return nil, err
	}
	positions, err := lookupProjectPositions(model["positions"])
	if err != nil {
		return nil, err
	}
	labels, err := lookupProjectLabels(model["labels"])
	if err != nil {
		return nil, err
	}
	out["departments"] = departments
	out["positions"] = positions
	out["labels"] = labels
	return out, nil
}

func lookupProjectOrganization(model map[string]any) (map[string]any, error) {
	out := map[string]any{}
	if raw, exists := model["orgId"]; exists {
		if !lookupStableIdentifier(raw) {
			return nil, fmt.Errorf("组织 orgId 不是稳定标识")
		}
		out["id"] = raw
	}
	for target, keys := range map[string][]string{
		"name":         {"orgName", "corpName"},
		"masterUserId": {"orgMasterUserId"},
		"masterName":   {"orgMasterDisplayName"},
	} {
		if err := copyStringAlias(out, target, model, keys...); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func lookupProjectDepartments(raw any) ([]map[string]any, error) {
	rows, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("完整资料 depts 缺失或不是数组")
	}
	out := make([]map[string]any, 0, len(rows))
	for index, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("depts[%d] 不是对象", index)
		}
		if err := rejectUnknownKeys(row, fmt.Sprintf("depts[%d]", index), "deptId", "deptName", "deptPathName"); err != nil {
			return nil, err
		}
		id, exists := row["deptId"]
		if !exists || !lookupStableIdentifier(id) {
			return nil, fmt.Errorf("depts[%d] 缺少稳定 deptId", index)
		}
		projected := map[string]any{"id": id}
		if err := copyStringAlias(projected, "name", row, "deptName"); err != nil {
			return nil, err
		}
		if err := copyStringAlias(projected, "path", row, "deptPathName"); err != nil {
			return nil, err
		}
		out = append(out, projected)
	}
	return out, nil
}

func lookupProjectPositions(raw any) ([]map[string]any, error) {
	rows, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("完整资料 positions 缺失或不是数组")
	}
	out := make([]map[string]any, 0, len(rows))
	for index, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("positions[%d] 不是对象", index)
		}
		if err := rejectUnknownKeys(row, fmt.Sprintf("positions[%d]", index),
			"deptId", "isMain", "managerDisplayName", "managerStaffId", "title", "workStation"); err != nil {
			return nil, err
		}
		id, exists := row["deptId"]
		if !exists || !lookupStableIdentifier(id) {
			return nil, fmt.Errorf("positions[%d] 缺少稳定 deptId", index)
		}
		projected := map[string]any{"departmentId": id}
		for target, keys := range map[string][]string{
			"title":         {"title"},
			"workStation":   {"workStation"},
			"managerUserId": {"managerStaffId"},
			"managerName":   {"managerDisplayName"},
		} {
			if err := copyStringAlias(projected, target, row, keys...); err != nil {
				return nil, err
			}
		}
		if rawMain, exists := row["isMain"]; exists {
			isMain, ok := rawMain.(bool)
			if !ok {
				return nil, fmt.Errorf("positions[%d].isMain 不是布尔值", index)
			}
			projected["isMain"] = isMain
		}
		out = append(out, projected)
	}
	return out, nil
}

func lookupProjectLabels(raw any) ([]map[string]any, error) {
	rows, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("完整资料 labels 缺失或不是数组")
	}
	out := make([]map[string]any, 0, len(rows))
	for index, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("labels[%d] 不是对象", index)
		}
		if err := rejectUnknownKeys(row, fmt.Sprintf("labels[%d]", index), "labelId", "labelName", "name"); err != nil {
			return nil, err
		}
		id, exists := row["labelId"]
		if !exists || !lookupStableIdentifier(id) {
			return nil, fmt.Errorf("labels[%d] 缺少稳定 labelId", index)
		}
		projected := map[string]any{"id": id}
		if err := copyStringAlias(projected, "name", row, "labelName", "name"); err != nil {
			return nil, err
		}
		out = append(out, projected)
	}
	return out, nil
}

func copyStringAlias(target map[string]any, targetKey string, source map[string]any, keys ...string) error {
	value, err := uniqueNonEmptyString(source, keys...)
	if err != nil {
		return fmt.Errorf("%s: %w", targetKey, err)
	}
	if value != "" {
		target[targetKey] = value
	}
	return nil
}

func lookupStableIdentifier(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case int, int32, int64, uint, uint32, uint64:
		return true
	case float64:
		return typed == float64(int64(typed))
	case json.Number:
		_, err := typed.Int64()
		return err == nil
	default:
		return false
	}
}

func uniqueNonEmptyString(source map[string]any, keys ...string) (string, error) {
	var selected string
	for _, key := range keys {
		raw, exists := source[key]
		if !exists || raw == nil {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			return "", fmt.Errorf("%s 不是字符串", key)
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if selected != "" && selected != value {
			return "", fmt.Errorf("别名字段值冲突")
		}
		selected = value
	}
	return selected, nil
}

func rejectUnknownKeys(value map[string]any, context string, allowed ...string) error {
	known := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		known[key] = struct{}{}
	}
	unknown := make([]string, 0)
	for key := range value {
		if _, ok := known[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("%s 包含未审阅字段: %s", context, strings.Join(unknown, ","))
}

func lookupProjectionFailure(message string) output.CommandResult {
	return output.Failure(&output.ErrorInfo{
		Type:      "api",
		Subtype:   string(apperrors.SubtypeProjectionUnknown),
		Message:   message,
		Hint:      "不要丢弃未知资料字段或猜测用户/部门标识；保留脱敏字段名并扩展完整资料投影。",
		Operation: "contact/get_user_info_by_user_ids",
		Stage:     "response_projection",
	})
}
