// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package aitable

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func validBaseGetResponse() map[string]any {
	return map[string]any{
		"success": true,
		"status":  "ok",
		"summary": "reviewed fixture",
		"error":   map[string]any{},
		"meta":    map[string]any{},
		"data": map[string]any{
			"baseId":   "base-1",
			"baseName": "Project",
			"tables": []any{
				map[string]any{"tableId": "table-1", "tableName": "Tasks"},
			},
			"dashboards": []any{
				map[string]any{"dashboardId": "dashboard-1", "dashboardName": "Overview", "chartCount": float64(2), "meta": map[string]any{}},
			},
			"documents": []any{},
		},
	}
}

func TestBaseGetProjectionPublishesOnlyStableDirectoryFields(t *testing.T) {
	payload, meta, err := baseGetProjection(validBaseGetResponse(), "base-1")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if payload["baseId"] != "base-1" || payload["baseName"] != "Project" || payload["inventoryCoverageKnown"] != false {
		t.Fatalf("payload = %#v", payload)
	}
	if got := payload["tables"].([]map[string]any); len(got) != 1 || got[0]["tableId"] != "table-1" {
		t.Fatalf("tables = %#v", got)
	}
	if got := payload["dashboards"].([]map[string]any); len(got) != 1 || got[0]["chartCount"] != 2 {
		t.Fatalf("dashboards = %#v", got)
	}
	if meta == nil || meta.Count == nil || *meta.Count != 2 || meta.Pagination != nil {
		t.Fatalf("meta = %#v", meta)
	}
	if BaseGet.OutputRollout != output.RolloutDualValidate {
		t.Fatalf("rollout = %q, want dual_validate", BaseGet.OutputRollout)
	}
}

func TestBaseGetProjectionRejectsUnreviewedOrContradictoryShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown top field", func(raw map[string]any) { raw["guessed"] = true }},
		{"unknown data field", func(raw map[string]any) { raw["data"].(map[string]any)["complete"] = true }},
		{"wrong base", func(raw map[string]any) { raw["data"].(map[string]any)["baseId"] = "other" }},
		{"missing table id", func(raw map[string]any) {
			delete(raw["data"].(map[string]any)["tables"].([]any)[0].(map[string]any), "tableId")
		}},
		{"duplicate table id", func(raw map[string]any) {
			data := raw["data"].(map[string]any)
			data["tables"] = append(data["tables"].([]any), map[string]any{"tableId": "table-1", "tableName": "Duplicate"})
		}},
		{"fractional chart count", func(raw map[string]any) {
			raw["data"].(map[string]any)["dashboards"].([]any)[0].(map[string]any)["chartCount"] = 1.5
		}},
		{"nonempty unreviewed documents", func(raw map[string]any) {
			raw["data"].(map[string]any)["documents"] = []any{map[string]any{"id": "unknown"}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := validBaseGetResponse()
			tc.mutate(raw)
			_, _, err := baseGetProjection(raw, "base-1")
			assertBaseGetProjectionUnknown(t, err)
		})
	}
}

func TestBaseGetProjectionKeepsKnownEmptyDistinctFromMissing(t *testing.T) {
	raw := validBaseGetResponse()
	data := raw["data"].(map[string]any)
	data["tables"] = []any{}
	data["dashboards"] = []any{}
	payload, meta, err := baseGetProjection(raw, "base-1")
	if err != nil {
		t.Fatalf("known empty: %v", err)
	}
	if len(payload["tables"].([]map[string]any)) != 0 || meta.Count == nil || *meta.Count != 0 {
		t.Fatalf("known empty payload/meta = %#v %#v", payload, meta)
	}

	delete(data, "tables")
	_, _, err = baseGetProjection(raw, "base-1")
	assertBaseGetProjectionUnknown(t, err)
}

func assertBaseGetProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
