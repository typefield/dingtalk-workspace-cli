// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestDevAppListResultSpecMatchesSharedProjection(t *testing.T) {
	tests := []struct {
		tool    string
		dataKey string
	}{
		{tool: devAppListTool, dataKey: "apps"},
		{tool: devAppPermissionListTool, dataKey: "permissions"},
		{tool: devAppEventListTool, dataKey: "events"},
		{tool: devAppVersionListTool, dataKey: "versions"},
	}
	for _, tt := range tests {
		t.Run(tt.dataKey, func(t *testing.T) {
			result := DevAppListResultSpec(tt.tool)
			if result == nil || result.NDJSON == nil || result.NDJSON.RecordPath != tt.dataKey {
				t.Fatalf("result spec = %#v", result)
			}
			if result.Pagination != nil {
				t.Fatalf("business result must not redeclare meta pagination: %#v", result.Pagination)
			}
			if got, want := result.Outcomes, []contract.ResultOutcome{
				contract.ResultOutcomeSuccess,
				contract.ResultOutcomeFailure,
			}; !reflect.DeepEqual(got, want) {
				t.Fatalf("outcomes = %#v, want %#v", got, want)
			}
			var schema struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			}
			if err := json.Unmarshal(result.DataSchema, &schema); err != nil {
				t.Fatalf("decode data schema: %v", err)
			}
			if len(schema.Properties) != 1 || schema.Properties[tt.dataKey] == nil ||
				!reflect.DeepEqual(schema.Required, []string{tt.dataKey}) {
				t.Fatalf("schema properties=%v required=%v", schema.Properties, schema.Required)
			}

			page, problem, handled := ProjectDevAppListPage(tt.tool, map[string]any{
				"items": []any{}, "hasMore": false,
			})
			if !handled || problem != nil || page == nil || len(page.Data) != 1 {
				t.Fatalf("projection: handled=%v problem=%+v page=%#v", handled, problem, page)
			}
			if _, ok := page.Data[tt.dataKey]; !ok {
				t.Fatalf("declared key %q missing from active data %#v", tt.dataKey, page.Data)
			}
		})
	}
	if result := DevAppListResultSpec("not-a-list-tool"); result != nil {
		t.Fatalf("unsupported tool unexpectedly has result spec: %#v", result)
	}
}

func TestDevAppSharedResultMapperClassifiesServiceOutcomes(t *testing.T) {
	t.Run("normalized success", func(t *testing.T) {
		result := DevAppCommandResultFromPayload("", map[string]any{
			"content": map[string]any{"success": true, "result": map[string]any{"id": "a"}},
		}, false)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := env.Data.(map[string]any)
		if env.Outcome != output.OutcomeSuccess || data["id"] != "a" {
			t.Fatalf("success envelope=%+v data=%#v", env, env.Data)
		}
	})

	t.Run("string false is still a failure", func(t *testing.T) {
		result := DevAppCommandResultFromPayload("", map[string]any{
			"success":  "false",
			"errorMsg": "rejected",
		}, false)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Outcome != output.OutcomeFailure || result.ExitCode() == 0 {
			t.Fatalf("string-false envelope=%+v rc=%d", env, result.ExitCode())
		}
	})

	t.Run("string true service wrapper is normalized", func(t *testing.T) {
		result := DevAppCommandResultFromPayload("", map[string]any{
			"content": map[string]any{
				"success": "true",
				"result":  map[string]any{"id": "string-true"},
			},
		}, false)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := env.Data.(map[string]any)
		if env.Outcome != output.OutcomeSuccess || data["id"] != "string-true" {
			t.Fatalf("string-true envelope=%+v data=%#v", env, env.Data)
		}
	})

	t.Run("pending approval", func(t *testing.T) {
		result := DevAppCommandResultFromPayload("", map[string]any{
			"versionStatus": "AUDIT", "versionId": "v1", "unifiedAppId": "u1",
		}, false)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Outcome != output.OutcomePending || env.Meta == nil || env.Meta.Operation == nil || env.Meta.Operation.NextCommand == "" {
			t.Fatalf("pending envelope=%+v", env)
		}
	})

	t.Run("approval selection uses tool normalization", func(t *testing.T) {
		result := DevAppCommandResultFromPayload(devAppVersionPublishTool, map[string]any{
			"approvalMode": "SELECT_APPROVER",
			"unifiedAppId": "u1",
			"versionId":    "v1",
			"approvalCandidates": []any{
				map[string]any{"userId": "user-1", "name": "Alice"},
			},
		}, false)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Outcome != output.OutcomePending || env.Meta == nil || env.Meta.Operation == nil {
			t.Fatalf("approval envelope=%+v", env)
		}
		if env.Meta.Operation.State != "waiting_for_approver_selection" || env.Meta.Operation.NextCommand == "" {
			t.Fatalf("approval operation=%+v", env.Meta.Operation)
		}
	})

	t.Run("approval precheck preserves params and stays terminal", func(t *testing.T) {
		result := DevAppCommandResultFromPayload(devAppVersionPublishTool, map[string]any{
			"approvalMode": "SELECT_APPROVER",
			"unifiedAppId": "u1",
			"versionId":    "v1",
			"approvalCandidates": []any{
				map[string]any{"userId": "user-1", "name": "Alice"},
			},
		}, false, map[string]any{"precheckOnly": true, "unifiedAppId": "u1", "versionId": "v1"})
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Outcome != output.OutcomeSuccess || result.ExitCode() != 0 {
			t.Fatalf("precheck envelope=%+v rc=%d, want terminal success", env, result.ExitCode())
		}
	})

	t.Run("pagination", func(t *testing.T) {
		result := DevAppCommandResultFromPayload("", map[string]any{
			"items": []any{}, "hasMore": true, "nextCursor": "next",
		}, false)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Meta == nil || env.Meta.Pagination == nil || env.Meta.Pagination.EndpointExhausted || env.Meta.Pagination.NextToken != "next" {
			t.Fatalf("pagination envelope=%+v", env)
		}
	})

	t.Run("terminal position cursor is not a continuation", func(t *testing.T) {
		result := DevAppCommandResultFromPayload("", map[string]any{
			"items": []any{}, "hasMore": false, "nextCursor": "terminal-position",
		}, false)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Outcome != output.OutcomeSuccess || result.ExitCode() != 0 || env.Meta == nil ||
			env.Meta.Pagination == nil || !env.Meta.Pagination.EndpointExhausted ||
			env.Meta.Pagination.NextToken != "" {
			t.Fatalf("terminal pagination envelope=%+v rc=%d", env, result.ExitCode())
		}
	})

	t.Run("non-final page without cursor is a failure", func(t *testing.T) {
		result := DevAppCommandResultFromPayload("", map[string]any{
			"items": []any{}, "hasMore": true,
		}, false)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Outcome != output.OutcomeFailure || result.ExitCode() == 0 || env.Error == nil ||
			env.Error.Subtype != "pagination_incomplete" || env.Error.Hint == "" ||
			env.Error.Operation != "devapp.pagination_projection" {
			t.Fatalf("incomplete pagination envelope=%+v rc=%d", env, result.ExitCode())
		}
	})

	t.Run("partial", func(t *testing.T) {
		result := DevAppCommandResultFromPayload("", map[string]any{
			"multiProfile": true,
			"profiles": []any{
				map[string]any{"selector": "a", "ok": true, "result": map[string]any{"id": "r1"}},
				map[string]any{"selector": "b", "ok": false, "error": map[string]any{"category": "api", "reason": "rejected", "message": "bad"}},
			},
		}, false)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Outcome != output.OutcomePartialFailure || result.ExitCode() != 7 {
			t.Fatalf("partial outcome=%s rc=%d", env.Outcome, result.ExitCode())
		}
	})

	t.Run("dry run is completed preview", func(t *testing.T) {
		result := DevAppCommandResultFromPayload("", map[string]any{"tool": "x"}, true)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Outcome != output.OutcomeSuccess || !env.DryRun {
			t.Fatalf("dry-run envelope=%+v", env)
		}
	})
}

func TestDevAppNativeAndShortcutListToolsShareProjectedResult(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		dataKey string
		item    map[string]any
	}{
		{name: "apps", tool: devAppListTool, dataKey: "apps", item: map[string]any{"unifiedAppId": "app-1", "name": "Example"}},
		{name: "permissions", tool: devAppPermissionListTool, dataKey: "permissions", item: map[string]any{"scopeValue": "contact:user.base:read", "scopeName": "Read users"}},
		{name: "events", tool: devAppEventListTool, dataKey: "events", item: map[string]any{"eventCode": "chat_add_member", "eventName": "Member added"}},
		{name: "versions", tool: devAppVersionListTool, dataKey: "versions", item: map[string]any{"versionId": "version-1", "version": "1.0.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := map[string]any{
				"success": true,
				"result": map[string]any{
					"items":      []any{tt.item},
					"hasMore":    true,
					"nextCursor": "cursor-next",
				},
			}
			result := DevAppCommandResultFromPayload(tt.tool, payload, false)
			env, err := output.EnvelopeFromResult(result)
			if err != nil {
				t.Fatal(err)
			}
			if env.Outcome != output.OutcomeSuccess || result.ExitCode() != 0 {
				t.Fatalf("envelope=%+v rc=%d", env, result.ExitCode())
			}
			data, ok := env.Data.(map[string]any)
			if !ok {
				t.Fatalf("data=%#v", env.Data)
			}
			for _, legacyKey := range []string{"count", "hasMore", "nextCursor", "items"} {
				if _, leaked := data[legacyKey]; leaked {
					t.Fatalf("legacy key %q leaked into active data: %#v", legacyKey, data)
				}
			}
			items, ok := data[tt.dataKey].([]map[string]any)
			if !ok || len(items) != 1 {
				t.Fatalf("%s=%#v", tt.dataKey, data[tt.dataKey])
			}
			if env.Meta == nil || env.Meta.Count == nil || *env.Meta.Count != 1 ||
				env.Meta.Pagination == nil || env.Meta.Pagination.EndpointExhausted ||
				env.Meta.Pagination.NextToken != "cursor-next" {
				t.Fatalf("meta=%+v", env.Meta)
			}
		})
	}
}
