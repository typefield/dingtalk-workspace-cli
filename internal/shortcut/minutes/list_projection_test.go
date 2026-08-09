// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package minutes

import (
	"encoding/json"
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

// TestCallListProjectItemListShape guards against projection-data-loss end to
// end: list_by_keyword_and_time_range nests the minutes under result.itemList,
// so the resolver must probe "itemList", and each projected row must carry the
// real taskUuid that record-control commands consume. minutesId is deliberately
// not treated as a taskUuid (see callListProject).
func TestCallListProjectItemListShape(t *testing.T) {
	const raw = `{"result":{"itemList":[
		{"taskUuid":"uuid-1","title":"weekly sync"},
		{"task_uuid":"uuid-2","title":"design review"}
	]}}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	got, err := callListProject(data)
	if err != nil {
		t.Fatalf("projection returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("lower/upper mismatch: itemList has 2 entries, projection returned %d", len(got))
	}
	if got[0]["taskUuid"] != "uuid-1" || got[1]["taskUuid"] != "uuid-2" {
		t.Fatalf("taskUuid not projected from taskUuid/task_uuid keys: %v", got)
	}
}

func TestCallListProjectSeparatesKnownEmptyFromUnknown(t *testing.T) {
	minutes, err := callListProject(map[string]any{"items": []any{}})
	if err != nil || minutes == nil || len(minutes) != 0 {
		t.Fatalf("known empty = %#v, %v; want non-nil empty list", minutes, err)
	}

	for name, data := range map[string]map[string]any{
		"unknown container": {"unexpected": []any{}},
		"malformed row":     {"items": []any{"opaque"}},
		"unknown row":       {"items": []any{map[string]any{"opaque": true}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := callListProject(data)
			assertMinutesProjectionUnknown(t, err)
		})
	}
}

func TestCallListProjectRejectsDisplayOnlyMinute(t *testing.T) {
	_, err := callListProject(map[string]any{"items": []any{map[string]any{"title": "仅展示"}}})
	assertMinutesProjectionUnknown(t, err)
}

func TestMinutesListShortcutsAreUnifiedActive(t *testing.T) {
	for name, rollout := range map[string]output.RolloutState{
		"mine":   ListMine.OutputRollout,
		"shared": ListShared.OutputRollout,
		"all":    ListAll.OutputRollout,
	} {
		if rollout != output.RolloutUnifiedActive {
			t.Fatalf("%s rollout=%q, want unified_active", name, rollout)
		}
	}
}

func assertMinutesProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
