package smart

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestMinutesSearchProjectDistinguishesKnownEmptyFromUnknown(t *testing.T) {
	minutes, err := minutesSearchProject(map[string]any{
		"result": map[string]any{"list": []any{}},
	})
	if err != nil {
		t.Fatalf("known empty minute search: %v", err)
	}
	if minutes == nil || len(minutes) != 0 {
		t.Fatalf("known empty minutes=%#v, want non-nil empty list", minutes)
	}

	minutes, err = minutesSearchProject(map[string]any{
		"data": []any{map[string]any{
			"taskUuid": "minute-1",
			"title":    "周会",
		}},
	})
	if err != nil || len(minutes) != 1 || minutes[0]["task_uuid"] != "minute-1" {
		t.Fatalf("nested minute projection=%#v err=%v", minutes, err)
	}

	minutes, err = minutesSearchProject(map[string]any{
		"result": map[string]any{"itemList": []any{map[string]any{
			"taskUuid": "minute-item-list",
			"title":    "周会",
		}}},
	})
	if err != nil || len(minutes) != 1 || minutes[0]["task_uuid"] != "minute-item-list" {
		t.Fatalf("result.itemList projection=%#v err=%v", minutes, err)
	}
}

func TestMinutesSearchProjectRejectsUntargetableData(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"unknown container": {"result": map[string]any{"notice": "unrecognized"}},
		"malformed row":     {"result": []any{"invalid"}},
		"missing task uuid": {"result": []any{map[string]any{"title": "display-only"}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := minutesSearchProject(data)
			assertMinutesSearchProjectionUnknown(t, err)
		})
	}
}

func TestMinutesSearchIsUnifiedActive(t *testing.T) {
	if MinutesSearch.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("minutes-search rollout=%q, want unified_active", MinutesSearch.OutputRollout)
	}
}

func assertMinutesSearchProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want projection error")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error=%T %#v, want non-retryable projection_unknown", err, err)
	}
}
