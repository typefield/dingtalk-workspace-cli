package todo

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestGetMyTasksProjectSeparatesKnownEmptyFromUnknown(t *testing.T) {
	cards, err := getMyTasksProject(map[string]any{
		"result": map[string]any{"todoCards": []any{}},
	})
	if err != nil || cards == nil || len(cards) != 0 {
		t.Fatalf("known empty = %#v, %v; want non-nil empty list", cards, err)
	}

	for name, data := range map[string]map[string]any{
		"unknown container": {"result": map[string]any{"items": []any{}}},
		"malformed row":     {"result": map[string]any{"todoCards": []any{"opaque"}}},
		"unknown row":       {"result": map[string]any{"todoCards": []any{map[string]any{"opaque": true}}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := getMyTasksProject(data)
			assertTodoProjectionUnknown(t, err)
		})
	}
}

func TestGetMyTasksProjectAcceptsKnownCards(t *testing.T) {
	cards, err := getMyTasksProject(map[string]any{
		"result": map[string]any{"todoCards": []any{map[string]any{
			"taskId": "task-1", "subject": "Review report",
		}}},
	})
	if err != nil {
		t.Fatalf("known card projection returned error: %v", err)
	}
	if len(cards) != 1 || cards[0]["taskId"] != "task-1" {
		t.Fatalf("projected cards = %#v", cards)
	}
}

func assertTodoProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
