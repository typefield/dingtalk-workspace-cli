// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package devapp

import (
	stderrors "errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
)

func TestDevAppSharedListProjectionPreservesPaginationEvidence(t *testing.T) {
	tests := []struct {
		name      string
		source    map[string]any
		exhausted bool
		token     string
	}{
		{name: "top level", source: map[string]any{"items": []any{}, "hasMore": true, "nextCursor": "next-1"}, token: "next-1"},
		{name: "nested result", source: map[string]any{"result": map[string]any{"items": []any{}, "hasMore": false}}, exhausted: true},
		{name: "nested page info", source: map[string]any{"items": []any{}, "data": map[string]any{"pageInfo": map[string]any{"hasMore": true, "nextCursor": "next-2"}}}, token: "next-2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, problem, handled := helpers.ProjectDevAppListPage("list_dev_app", tt.source)
			if !handled || problem != nil || page == nil {
				t.Fatalf("projection: handled=%v problem=%+v page=%#v", handled, problem, page)
			}
			if page.Meta == nil || page.Meta.Pagination == nil ||
				page.Meta.Pagination.EndpointExhausted != tt.exhausted || page.Meta.Pagination.NextToken != tt.token {
				t.Fatalf("pagination=%+v", page.Meta)
			}
			if _, leaked := page.Data["hasMore"]; leaked {
				t.Fatalf("active data leaked pagination: %#v", page.Data)
			}
		})
	}
}

func TestDevAppSharedListProjectionRejectsInvalidPaginationEvidence(t *testing.T) {
	tests := []struct {
		name    string
		source  map[string]any
		subtype apperrors.Subtype
	}{
		{name: "has more is not boolean", source: map[string]any{"items": []any{}, "hasMore": "true", "nextCursor": "next-1"}, subtype: apperrors.SubtypePaginationInvalid},
		{name: "cursor is not string", source: map[string]any{"items": []any{}, "hasMore": true, "nextCursor": 7}, subtype: apperrors.SubtypePaginationIncomplete},
		{name: "has more conflicts", source: map[string]any{"items": []any{}, "hasMore": true, "result": map[string]any{"hasMore": false, "nextCursor": "next-1"}}, subtype: apperrors.SubtypePaginationConflict},
		{name: "cursor conflicts", source: map[string]any{"items": []any{}, "hasMore": true, "nextCursor": "next-1", "result": map[string]any{"hasMore": true, "nextCursor": "next-2"}}, subtype: apperrors.SubtypePaginationConflict},
		{name: "nonfinal omits cursor", source: map[string]any{"items": []any{}, "hasMore": true}, subtype: apperrors.SubtypePaginationIncomplete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, problem, handled := helpers.ProjectDevAppListPage("list_dev_app", tt.source)
			if !handled || page != nil || problem == nil || problem.Type != "validation" || problem.Subtype != string(tt.subtype) {
				t.Fatalf("projection: handled=%v page=%#v problem=%+v", handled, page, problem)
			}
			var typed *apperrors.Error
			if err := helpers.DevAppListProjectionError(problem); !stderrors.As(err, &typed) ||
				typed.Category != apperrors.CategoryValidation || typed.StableSubtype != string(tt.subtype) ||
				typed.FailureStage != "response_projection" {
				t.Fatalf("typed error=%#v", typed)
			}
		})
	}
}

func TestDevAppSharedListProjectionSeparatesKnownEmptyFromUnknown(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		dataKey string
		stable  map[string]any
	}{
		{name: "apps", tool: "list_dev_app", dataKey: "apps", stable: map[string]any{"unifiedAppId": "app-1", "name": "Example"}},
		{name: "permissions", tool: "list_dev_app_permissions", dataKey: "permissions", stable: map[string]any{"scopeValue": "contact:user.base:read", "scopeName": "Read users"}},
		{name: "events", tool: "list_dev_app_events", dataKey: "events", stable: map[string]any{"eventCode": "chat_add_member", "eventName": "Member added"}},
		{name: "versions", tool: "list_dev_app_versions", dataKey: "versions", stable: map[string]any{"versionId": "version-1", "version": "1.0.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, problem, handled := helpers.ProjectDevAppListPage(tt.tool, map[string]any{"result": map[string]any{"items": []any{}, "hasMore": false}})
			items, ok := page.Data[tt.dataKey].([]map[string]any)
			if !handled || problem != nil || !ok || items == nil || len(items) != 0 || page.Meta.Count == nil || *page.Meta.Count != 0 {
				t.Fatalf("known empty: handled=%v problem=%+v page=%#v", handled, problem, page)
			}
			page, problem, _ = helpers.ProjectDevAppListPage(tt.tool, map[string]any{"items": []any{tt.stable}, "hasMore": false})
			items, ok = page.Data[tt.dataKey].([]map[string]any)
			if problem != nil || !ok || len(items) != 1 {
				t.Fatalf("valid projection: problem=%+v page=%#v", problem, page)
			}
			for name, payload := range map[string]map[string]any{
				"unknown container": {"result": map[string]any{"status": "ok"}},
				"not an array":      {"result": map[string]any{"items": "not-an-array"}},
				"malformed row":     {"items": []any{"opaque"}},
				"display only row":  {"items": []any{map[string]any{"name": "display only"}}},
			} {
				t.Run(name, func(t *testing.T) {
					page, problem, _ := helpers.ProjectDevAppListPage(tt.tool, payload)
					if page != nil || problem == nil || problem.Type != "api" || problem.Subtype != string(apperrors.SubtypeProjectionUnknown) {
						t.Fatalf("page=%#v problem=%+v", page, problem)
					}
				})
			}
		})
	}
}

func TestDevAppSharedListProjectionTreatsTerminalCursorAsNonActionable(t *testing.T) {
	page, problem, handled := helpers.ProjectDevAppListPage("list_dev_app", map[string]any{
		"items": []any{}, "hasMore": false, "nextCursor": "terminal-position",
	})
	if !handled || problem != nil || page.Meta == nil || page.Meta.Pagination == nil ||
		!page.Meta.Pagination.EndpointExhausted || page.Meta.Pagination.NextToken != "" {
		t.Fatalf("handled=%v problem=%+v page=%#v", handled, problem, page)
	}
	if _, leaked := page.LegacyPage["nextCursor"]; leaked {
		t.Fatalf("terminal cursor leaked into legacy clean projection: %#v", page.LegacyPage)
	}
}
