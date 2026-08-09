package smart

import (
	"errors"
	"testing"
	"time"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestCalendarEventListProjectDistinguishesKnownEmptyFromUnknown(t *testing.T) {
	events, err := calendarEventListProject(map[string]any{
		"result": map[string]any{"events": []any{}},
	})
	if err != nil {
		t.Fatalf("known empty events: %v", err)
	}
	if events == nil || len(events) != 0 {
		t.Fatalf("known empty events=%#v, want non-nil empty list", events)
	}

	events, err = calendarEventListProject(map[string]any{
		"data": []any{map[string]any{"id": "evt-1"}},
	})
	if err != nil || len(events) != 1 || events[0]["id"] != "evt-1" {
		t.Fatalf("nested direct list events=%#v err=%v", events, err)
	}

	for name, data := range map[string]map[string]any{
		"unknown container": {"result": map[string]any{"notice": "unrecognized"}},
		"malformed row":     {"events": []any{"not-an-event"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := calendarEventListProject(data)
			assertCalendarProjectionUnknown(t, err)
		})
	}
}

func TestCalendarProjectEventsAndNextEventRejectUntargetableData(t *testing.T) {
	_, err := calendarProjectEvents(map[string]any{
		"events": []any{map[string]any{"summary": "display only"}},
	})
	assertCalendarProjectionUnknown(t, err)

	_, err = shortcutNextEventPick([]map[string]any{{"id": "evt-1"}}, time.Now())
	assertCalendarProjectionUnknown(t, err)
}

func TestCalendarDerivedReadShortcutsAreUnifiedActive(t *testing.T) {
	for name, rollout := range map[string]output.RolloutState{
		"today":      Today.OutputRollout,
		"tomorrow":   Tomorrow.OutputRollout,
		"week":       Week.OutputRollout,
		"next-event": NextEvent.OutputRollout,
		"free-slots": FreeSlots.OutputRollout,
		"conflicts":  Conflicts.OutputRollout,
	} {
		if rollout != output.RolloutUnifiedActive {
			t.Errorf("%s rollout=%q, want unified_active", name, rollout)
		}
	}
}

func assertCalendarProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want projection error")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error=%T %#v, want non-retryable projection_unknown", err, err)
	}
}
