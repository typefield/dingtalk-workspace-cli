// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package devapp

import (
	stderrors "errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestDevAppProjectedListsPreservePaginationEvidence(t *testing.T) {
	tests := []struct {
		name   string
		source map[string]any
	}{
		{name: "top level", source: map[string]any{"hasMore": true, "nextCursor": "next-1"}},
		{name: "nested result", source: map[string]any{"result": map[string]any{"hasMore": false}}},
		{name: "nested page info", source: map[string]any{"data": map[string]any{"pageInfo": map[string]any{"hasMore": true, "nextCursor": "next-2"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projected, err := projectDevAppPage(tt.source, map[string]any{"count": 1, "items": []any{map[string]any{"id": "item-1"}}})
			if err != nil {
				t.Fatalf("projectDevAppPage() error = %v", err)
			}
			if got, ok := projected["hasMore"].(bool); !ok {
				t.Fatalf("hasMore missing from projection: %#v", projected)
			} else if got != (tt.name != "nested result") {
				t.Fatalf("hasMore=%v, want fixture value", got)
			}
			if tt.name != "nested result" {
				if projected["nextCursor"] == nil {
					t.Fatalf("nextCursor missing from projection: %#v", projected)
				}
			} else if _, exists := projected["nextCursor"]; exists {
				t.Fatalf("unexpected nextCursor for exhausted page: %#v", projected)
			}
		})
	}
}

func TestDevAppProjectedListsRejectInvalidPaginationEvidence(t *testing.T) {
	tests := []struct {
		name    string
		source  map[string]any
		subtype apperrors.Subtype
	}{
		{
			name:    "has more is not boolean",
			source:  map[string]any{"hasMore": "true", "nextCursor": "next-1"},
			subtype: apperrors.SubtypePaginationInvalid,
		},
		{
			name:    "cursor is not string",
			source:  map[string]any{"hasMore": true, "nextCursor": 7},
			subtype: apperrors.SubtypePaginationIncomplete,
		},
		{
			name:    "has more conflicts across envelopes",
			source:  map[string]any{"hasMore": true, "result": map[string]any{"hasMore": false, "nextCursor": "next-1"}},
			subtype: apperrors.SubtypePaginationConflict,
		},
		{
			name:    "cursor conflicts across envelopes",
			source:  map[string]any{"hasMore": true, "nextCursor": "next-1", "result": map[string]any{"hasMore": true, "nextCursor": "next-2"}},
			subtype: apperrors.SubtypePaginationConflict,
		},
		{
			name:    "nonfinal page omits cursor",
			source:  map[string]any{"hasMore": true},
			subtype: apperrors.SubtypePaginationIncomplete,
		},
		{
			name:    "exhausted page carries cursor",
			source:  map[string]any{"hasMore": false, "nextCursor": "next-1"},
			subtype: apperrors.SubtypePaginationConflict,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projected, err := projectDevAppPage(tt.source, map[string]any{"items": []any{}})
			if projected != nil {
				t.Fatalf("projectDevAppPage() projection = %#v, want nil on malformed evidence", projected)
			}
			var typed *apperrors.Error
			if !stderrors.As(err, &typed) {
				t.Fatalf("projectDevAppPage() error = %T %v, want *errors.Error", err, err)
			}
			if typed.Category != apperrors.CategoryValidation || typed.StableSubtype != string(tt.subtype) {
				t.Fatalf("typed error = %#v, want validation/%s", typed, tt.subtype)
			}
			if typed.FailureStage != "response_projection" || !typed.RetryableSet || typed.Retryable {
				t.Fatalf("typed error must be non-retryable response projection failure: %#v", typed)
			}
		})
	}
}

func TestDevAppShortcutsRollOutPerTerminalCommand(t *testing.T) {
	active := map[string]bool{
		"+get": true, "+credentials-get": true, "+webapp-get": true,
		"+robot-get": true, "+version-get": true,
		"+version-check-approval": true, "+version-status": true,
	}
	seen := map[string]bool{}
	for _, item := range shortcut.All() {
		if item.Service != productDevApp {
			continue
		}
		seen[item.Command] = true
		want := output.RolloutDualValidate
		if active[item.Command] {
			want = output.RolloutUnifiedActive
		}
		if item.OutputRollout != want {
			t.Errorf("%s rollout=%s, want %s", item.Command, item.OutputRollout, want)
		}
		if active[item.Command] && item.Contract.Identity.CanonicalPath == "" {
			t.Errorf("active shortcut %s has no complete Contract identity", item.Command)
		}
		if active[item.Command] && !shortcut.InPublicCatalog(item.Service, item.Command) {
			t.Errorf("active shortcut %s is not reachable from the public Agent catalog", item.Command)
		}
		if item.Command == "+credentials-get" {
			if item.Contract.Result == nil || len(item.Contract.Result.SensitivePaths) == 0 {
				t.Error("credentials shortcut must declare sensitive output paths")
			}
		}
	}
	for name := range active {
		if !seen[name] {
			t.Errorf("active pilot shortcut %s is not registered", name)
		}
	}
	for _, paginated := range []string{"+list", "+permission-list", "+event-list", "+version-list"} {
		if !seen[paginated] {
			t.Errorf("paginated shortcut %s is not registered", paginated)
		}
	}
}
