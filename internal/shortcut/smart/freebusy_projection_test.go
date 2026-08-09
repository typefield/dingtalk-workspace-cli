package smart

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestFreebusySlotsProjectAcceptsExplicitEmptyAndNestedResult(t *testing.T) {
	slots, err := freebusySlotsProject(map[string]any{
		"result": []any{map[string]any{"scheduleItems": []any{}}},
	})
	if err != nil {
		t.Fatalf("explicit empty busy list: %v", err)
	}
	if slots == nil || len(slots) != 0 {
		t.Fatalf("empty busy slots=%#v, want non-nil empty list", slots)
	}

	slots, err = freebusySlotsProject(map[string]any{
		"data": map[string]any{
			"result": []any{map[string]any{
				"scheduleItems": []any{map[string]any{
					"start": map[string]any{"dateTime": "2026-08-09T09:00:00+08:00"},
					"end":   map[string]any{"dateTime": "2026-08-09T10:00:00+08:00"},
				}},
			}},
		},
	})
	if err != nil || len(slots) != 1 || slots[0]["start"] != "2026-08-09T09:00:00+08:00" {
		t.Fatalf("nested busy slots=%#v err=%v", slots, err)
	}
}

func TestFreebusySlotsProjectFailsClosedOnUnknownOrMalformedData(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"unknown container":   {"result": map[string]any{"notice": "unrecognized"}},
		"non-object entry":    {"result": []any{"invalid"}},
		"missing schedule":    {"result": []any{map[string]any{}}},
		"bad schedule type":   {"result": []any{map[string]any{"scheduleItems": "invalid"}}},
		"non-object interval": {"result": []any{map[string]any{"scheduleItems": []any{"invalid"}}}},
		"missing interval end": {"result": []any{map[string]any{
			"scheduleItems": []any{map[string]any{"start": "2026-08-09T09:00:00+08:00"}},
		}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := freebusySlotsProject(data)
			assertCalendarProjectionUnknown(t, err)
		})
	}
}

func TestFreebusyShortcutsAreUnifiedActive(t *testing.T) {
	for name, rollout := range map[string]output.RolloutState{
		"free":    FreeBusy.OutputRollout,
		"my-free": MyFree.OutputRollout,
	} {
		if rollout != output.RolloutUnifiedActive {
			t.Errorf("%s rollout=%q, want unified_active", name, rollout)
		}
	}
}
