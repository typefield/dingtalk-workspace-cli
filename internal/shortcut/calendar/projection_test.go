package calendar

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

type calendarListProject func(map[string]any) ([]map[string]any, error)

func TestCalendarProjectionSeparatesKnownEmptyFromUnknown(t *testing.T) {
	standard := map[string]calendarListProject{
		"events":      eventListProject,
		"attendees":   attendeeListProject,
		"rooms":       roomSearchProject,
		"room groups": roomGroupsProject,
		"book search": bookSearchProject,
	}
	for name, project := range standard {
		t.Run(name+" known empty", func(t *testing.T) {
			rows, err := project(map[string]any{"items": []any{}})
			if err != nil || rows == nil || len(rows) != 0 {
				t.Fatalf("known empty = %#v, %v; want non-nil empty list", rows, err)
			}
		})
		t.Run(name+" unknown container", func(t *testing.T) {
			_, err := project(map[string]any{"unexpected": []any{}})
			assertCalendarProjectionUnknown(t, err)
		})
		t.Run(name+" malformed row", func(t *testing.T) {
			_, err := project(map[string]any{"items": []any{"opaque"}})
			assertCalendarProjectionUnknown(t, err)
		})
		t.Run(name+" unknown row", func(t *testing.T) {
			_, err := project(map[string]any{"items": []any{map[string]any{"opaque": true}}})
			assertCalendarProjectionUnknown(t, err)
		})
	}

	t.Run("book list known empty", func(t *testing.T) {
		rows, err := bookListProject(map[string]any{"result": []any{}})
		if err != nil || rows == nil || len(rows) != 0 {
			t.Fatalf("known empty = %#v, %v; want non-nil empty list", rows, err)
		}
	})
	t.Run("book list unknown", func(t *testing.T) {
		_, err := bookListProject(map[string]any{"items": []any{}})
		assertCalendarProjectionUnknown(t, err)
	})
}

func TestCalendarProjectedReadShortcutsAreUnifiedActive(t *testing.T) {
	for name, rollout := range map[string]output.RolloutState{
		"agenda":        EventList.OutputRollout,
		"attendee list": AttendeeList.OutputRollout,
		"room search":   RoomSearch.OutputRollout,
		"room groups":   RoomGroups.OutputRollout,
		"book list":     BookList.OutputRollout,
		"book search":   BookSearch.OutputRollout,
	} {
		if rollout != output.RolloutUnifiedActive {
			t.Fatalf("%s rollout=%q, want unified_active", name, rollout)
		}
	}
}

func TestCalendarProjectionAcceptsKnownNestedContainers(t *testing.T) {
	tests := []struct {
		name    string
		project calendarListProject
		data    map[string]any
	}{
		{"events", eventListProject, map[string]any{"result": map[string]any{"events": []any{map[string]any{"eventId": "e1"}}}}},
		{"attendees", attendeeListProject, map[string]any{"data": map[string]any{"attendees": []any{map[string]any{"userId": "u1"}}}}},
		{"rooms", roomSearchProject, map[string]any{"result": map[string]any{"rooms": []any{map[string]any{"roomId": "r1"}}}}},
		{"room groups", roomGroupsProject, map[string]any{"result": map[string]any{"groupList": []any{map[string]any{"groupId": "g1"}}}}},
		{"book search", bookSearchProject, map[string]any{"data": map[string]any{"calendars": []any{map[string]any{"calendarId": "c1"}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows, err := test.project(test.data)
			if err != nil || len(rows) != 1 {
				t.Fatalf("nested projection = %#v, %v; want one row", rows, err)
			}
		})
	}
}

func TestCalendarProjectionRejectsDisplayOnlyRows(t *testing.T) {
	tests := []struct {
		name    string
		project calendarListProject
		data    map[string]any
	}{
		{"events", eventListProject, map[string]any{"items": []any{map[string]any{"summary": "展示名"}}}},
		{"attendees", attendeeListProject, map[string]any{"items": []any{map[string]any{"displayName": "展示名"}}}},
		{"rooms", roomSearchProject, map[string]any{"items": []any{map[string]any{"roomName": "展示名"}}}},
		{"room groups", roomGroupsProject, map[string]any{"items": []any{map[string]any{"groupName": "展示名"}}}},
		{"book search", bookSearchProject, map[string]any{"items": []any{map[string]any{"summary": "展示名"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.project(test.data)
			assertCalendarProjectionUnknown(t, err)
		})
	}

	_, err := bookListProject(map[string]any{"result": []any{map[string]any{"summary": "展示名"}}})
	assertCalendarProjectionUnknown(t, err)
}

func assertCalendarProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
