package smart

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestFindRoomParseMillisAndValidation(t *testing.T) {
	start, err := findRoomParseMillis("start", "2026-03-10T14:00:00+08:00")
	if err != nil || start <= 0 {
		t.Fatalf("valid start: millis=%d err=%v", start, err)
	}
	if _, err := findRoomParseMillis("start", "not-a-time"); err == nil {
		t.Fatal("invalid timestamp must return typed validation error")
	}
}

func TestFindRoomProjectionNestedShape(t *testing.T) {
	data := map[string]any{
		"result": map[string]any{
			"rooms": []any{
				map[string]any{"roomId": "r1", "roomName": "A", "capacity": float64(8)},
			},
		},
	}
	rooms, err := findRoomProject(data)
	if err != nil {
		t.Fatalf("findRoomProject: %v", err)
	}
	if len(rooms) != 1 || rooms[0]["roomId"] != "r1" {
		t.Fatalf("nested room projection = %#v", rooms)
	}
}

func TestFindRoomProjectDistinguishesKnownEmptyFromUnknown(t *testing.T) {
	rooms, err := findRoomProject(map[string]any{"result": map[string]any{"rooms": []any{}}})
	if err != nil {
		t.Fatalf("known empty room result: %v", err)
	}
	if rooms == nil || len(rooms) != 0 {
		t.Fatalf("known empty rooms=%#v, want non-nil empty list", rooms)
	}

	_, err = findRoomProject(map[string]any{"result": map[string]any{"diagnostics": []any{}}})
	assertFindRoomProjectionUnknown(t, err)
}

func TestFindRoomProjectRejectsUntargetableRows(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"non-object": {"rooms": []any{"opaque"}},
		"missing id": {"rooms": []any{map[string]any{"roomName": "display-only"}}},
		"empty id":   {"rooms": []any{map[string]any{"roomName": "empty", "roomId": " "}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := findRoomProject(data)
			assertFindRoomProjectionUnknown(t, err)
		})
	}
}

func TestFindRoomRolloutIsUnifiedActive(t *testing.T) {
	if FindRoom.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("find-room rollout=%q, want unified_active", FindRoom.OutputRollout)
	}
}

func assertFindRoomProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want projection error")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error=%T %#v, want non-retryable projection_unknown", err, err)
	}
}
