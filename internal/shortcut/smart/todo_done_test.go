// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package smart

import (
	stderrors "errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestShortcutTodoCardsUnwrapsNestedResult(t *testing.T) {
	data := map[string]any{
		"result": map[string]any{
			"result": map[string]any{
				"todoCards": []any{
					map[string]any{
						"subject": "DWS shortcut 真实测试待办",
						"taskId":  "55155814691",
					},
				},
			},
		},
	}

	cards, err := shortcutTodoCards(data)
	if err != nil {
		t.Fatalf("shortcutTodoCards() error = %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("len(cards)=%d, want 1", len(cards))
	}
	if got := cards[0]["subject"]; got != "DWS shortcut 真实测试待办" {
		t.Fatalf("subject=%v", got)
	}
	if got := cards[0]["taskId"]; got != "55155814691" {
		t.Fatalf("taskId=%v", got)
	}
}

func TestShortcutTodoCardsFailsClosedOnUnknownOrUnaddressableRows(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{name: "missing container", data: map[string]any{"result": map[string]any{}}},
		{name: "container is not array", data: map[string]any{"result": map[string]any{"todoCards": "not-an-array"}}},
		{name: "row is not object", data: map[string]any{"result": map[string]any{"todoCards": []any{"not-an-object"}}}},
		{name: "row lacks task id", data: map[string]any{"result": map[string]any{"todoCards": []any{map[string]any{"subject": "unaddressable"}}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cards, err := shortcutTodoCards(tt.data)
			if cards != nil {
				t.Fatalf("shortcutTodoCards() cards = %#v, want nil", cards)
			}
			var typed *apperrors.Error
			if !stderrors.As(err, &typed) {
				t.Fatalf("shortcutTodoCards() error = %T %v, want *errors.Error", err, err)
			}
			if typed.Category != apperrors.CategoryAPI || typed.StableSubtype != string(apperrors.SubtypeProjectionUnknown) {
				t.Fatalf("typed error = %#v, want api/projection_unknown", typed)
			}
			if typed.FailureStage != "response_projection" || !typed.RetryableSet || typed.Retryable {
				t.Fatalf("typed error must be non-retryable response projection failure: %#v", typed)
			}
		})
	}
}
