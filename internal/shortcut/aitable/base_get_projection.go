// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package aitable

import (
	"fmt"
	"math"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

// baseGetProjection is deliberately narrower than the raw get_base response.
// It publishes only stable, Agent-reusable identifiers observed in the live
// contract. An unreviewed collection shape is an error rather than an empty
// list: dropping it would falsely claim a complete Base directory.
func baseGetProjection(raw map[string]any, requestedBaseID string) (map[string]any, *output.Meta, error) {
	data, err := baseGetData(raw)
	if err != nil {
		return nil, nil, err
	}
	baseID, err := requiredBaseGetString("data.baseId", data["baseId"])
	if err != nil {
		return nil, nil, err
	}
	if baseID != strings.TrimSpace(requestedBaseID) {
		return nil, nil, baseGetProjectionUnknown(fmt.Sprintf("data.baseId=%q does not match requested baseId", baseID))
	}
	baseName, err := requiredBaseGetString("data.baseName", data["baseName"])
	if err != nil {
		return nil, nil, err
	}
	tables, err := projectBaseGetTables(data["tables"])
	if err != nil {
		return nil, nil, err
	}
	dashboards, err := projectBaseGetDashboards(data["dashboards"])
	if err != nil {
		return nil, nil, err
	}
	documents, ok := data["documents"].([]any)
	if !ok {
		return nil, nil, baseGetProjectionUnknown(fmt.Sprintf("data.documents must be an array, got %T", data["documents"]))
	}
	if len(documents) != 0 {
		return nil, nil, baseGetProjectionUnknown("data.documents contains an unreviewed non-empty row shape")
	}

	count := len(tables) + len(dashboards)
	payload := map[string]any{
		"baseId":                 baseID,
		"baseName":               baseName,
		"tables":                 tables,
		"dashboards":             dashboards,
		"documents":              []map[string]any{},
		"counts":                 map[string]any{"tables": len(tables), "dashboards": len(dashboards), "documents": 0},
		"inventoryScope":         "base_resource_directory",
		"inventoryCoverageKnown": false,
	}
	return payload, &output.Meta{Count: output.NewCount(count)}, nil
}

func baseGetData(raw map[string]any) (map[string]any, error) {
	if raw == nil {
		return nil, baseGetProjectionUnknown("response is nil")
	}
	allowedTop := map[string]bool{"success": true, "status": true, "summary": true, "data": true, "error": true, "meta": true}
	for key := range raw {
		if !allowedTop[key] {
			return nil, baseGetProjectionUnknown(fmt.Sprintf("response contains unknown top-level field %q", key))
		}
	}
	success, ok := raw["success"].(bool)
	if !ok || !success {
		return nil, baseGetProjectionUnknown("response.success must be boolean true")
	}
	data, ok := raw["data"].(map[string]any)
	if !ok {
		return nil, baseGetProjectionUnknown(fmt.Sprintf("response.data must be an object, got %T", raw["data"]))
	}
	allowedData := map[string]bool{"baseId": true, "baseName": true, "tables": true, "dashboards": true, "documents": true}
	for key := range data {
		if !allowedData[key] {
			return nil, baseGetProjectionUnknown(fmt.Sprintf("response.data contains unknown field %q", key))
		}
	}
	for _, key := range []string{"baseId", "baseName", "tables", "dashboards", "documents"} {
		if _, exists := data[key]; !exists {
			return nil, baseGetProjectionUnknown(fmt.Sprintf("response.data is missing %q", key))
		}
	}
	return data, nil
}

func projectBaseGetTables(value any) ([]map[string]any, error) {
	rows, ok := value.([]any)
	if !ok {
		return nil, baseGetProjectionUnknown(fmt.Sprintf("data.tables must be an array, got %T", value))
	}
	out := make([]map[string]any, 0, len(rows))
	seen := map[string]struct{}{}
	for index, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			return nil, baseGetProjectionUnknown(fmt.Sprintf("data.tables[%d] must be an object", index))
		}
		if err := exactBaseGetKeys(fmt.Sprintf("data.tables[%d]", index), row, "tableId", "tableName"); err != nil {
			return nil, err
		}
		id, err := requiredBaseGetString(fmt.Sprintf("data.tables[%d].tableId", index), row["tableId"])
		if err != nil {
			return nil, err
		}
		name, err := requiredBaseGetString(fmt.Sprintf("data.tables[%d].tableName", index), row["tableName"])
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, baseGetProjectionUnknown(fmt.Sprintf("data.tables contains duplicate tableId %q", id))
		}
		seen[id] = struct{}{}
		out = append(out, map[string]any{"tableId": id, "tableName": name})
	}
	return out, nil
}

func projectBaseGetDashboards(value any) ([]map[string]any, error) {
	rows, ok := value.([]any)
	if !ok {
		return nil, baseGetProjectionUnknown(fmt.Sprintf("data.dashboards must be an array, got %T", value))
	}
	out := make([]map[string]any, 0, len(rows))
	seen := map[string]struct{}{}
	for index, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			return nil, baseGetProjectionUnknown(fmt.Sprintf("data.dashboards[%d] must be an object", index))
		}
		if err := exactBaseGetKeys(fmt.Sprintf("data.dashboards[%d]", index), row, "dashboardId", "dashboardName", "chartCount", "meta"); err != nil {
			return nil, err
		}
		id, err := requiredBaseGetString(fmt.Sprintf("data.dashboards[%d].dashboardId", index), row["dashboardId"])
		if err != nil {
			return nil, err
		}
		name, err := requiredBaseGetString(fmt.Sprintf("data.dashboards[%d].dashboardName", index), row["dashboardName"])
		if err != nil {
			return nil, err
		}
		count, ok := exactNonNegativeInt(row["chartCount"])
		if !ok {
			return nil, baseGetProjectionUnknown(fmt.Sprintf("data.dashboards[%d].chartCount must be a non-negative integer", index))
		}
		if _, ok := row["meta"].(map[string]any); !ok {
			return nil, baseGetProjectionUnknown(fmt.Sprintf("data.dashboards[%d].meta must be an object", index))
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, baseGetProjectionUnknown(fmt.Sprintf("data.dashboards contains duplicate dashboardId %q", id))
		}
		seen[id] = struct{}{}
		out = append(out, map[string]any{"dashboardId": id, "dashboardName": name, "chartCount": count})
	}
	return out, nil
}

func exactBaseGetKeys(path string, object map[string]any, keys ...string) error {
	allowed := make(map[string]bool, len(keys))
	for _, key := range keys {
		allowed[key] = true
		if _, exists := object[key]; !exists {
			return baseGetProjectionUnknown(fmt.Sprintf("%s is missing %q", path, key))
		}
	}
	for key := range object {
		if !allowed[key] {
			return baseGetProjectionUnknown(fmt.Sprintf("%s contains unknown field %q", path, key))
		}
	}
	return nil
}

func requiredBaseGetString(path string, value any) (string, error) {
	text, ok := value.(string)
	text = strings.TrimSpace(text)
	if !ok || text == "" {
		return "", baseGetProjectionUnknown(fmt.Sprintf("%s must be a non-empty string", path))
	}
	return text, nil
}

func exactNonNegativeInt(value any) (int, bool) {
	number, ok := value.(float64)
	if !ok || number < 0 || math.Trunc(number) != number || number > float64(math.MaxInt) {
		return 0, false
	}
	return int(number), true
}

func baseGetProjectionUnknown(message string) error {
	return apperrors.NewAPI("get_base response cannot be projected safely: "+message,
		apperrors.WithOperation("get_base"),
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithOrigin("mcp_gateway"),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}
