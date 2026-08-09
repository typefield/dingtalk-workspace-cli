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

package drive

import (
	"encoding/json"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

// TestRecentListProjectResultWrapper guards against projection-data-loss:
// get_recent_list nests its payload under result.recentItems; the projection
// must descend into result or +recent silently returns empty despite backend
// records.
func TestRecentListProjectResultWrapper(t *testing.T) {
	const raw = `{"result":{"hasMore":true,"nextCursor":"c2","recentItems":[
		{"name":"weekly report","nodeType":"doc","nodeId":"n1","docUrl":"https://x/1"},
		{"name":"budget sheet","nodeType":"sheet","nodeId":"n2","docUrl":"https://x/2"}
	]}}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	payload, result, err := recentListResult(data)
	if err != nil {
		t.Fatalf("recentListResult: %v", err)
	}
	if got, _ := payload["count"].(int); got != 2 {
		t.Fatalf("lower/upper mismatch: result.recentItems has 2 entries, projection count=%v (%v)", payload["count"], payload)
	}
	if payload["pagination_known"] != true {
		t.Fatalf("pagination should be known: %#v", payload)
	}
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatalf("EnvelopeFromResult: %v", err)
	}
	if env.Meta == nil || env.Meta.Pagination == nil || env.Meta.Pagination.EndpointExhausted || env.Meta.Pagination.NextToken != "c2" {
		t.Fatalf("pagination fields lost from result wrapper: %#v", env)
	}
}

// TestRecentListProjectRejectsUnknownContainer ensures a missing recentItems
// container cannot turn into a fabricated successful empty list.
func TestRecentListProjectRejectsUnknownContainer(t *testing.T) {
	const raw = `{"result":{"totalCount":0},"success":true}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if _, _, err := recentListResult(data); err == nil {
		t.Fatal("unknown recentItems container should fail closed")
	}
}

// TestRecentListProjectTopLevel covers the already-unwrapped shape.
func TestRecentListProjectTopLevel(t *testing.T) {
	const raw = `{"recentItems":[{"name":"weekly report","nodeId":"n1"}],"hasMore":false}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	payload, result, err := recentListResult(data)
	if err != nil {
		t.Fatalf("recentListResult: %v", err)
	}
	if payload["count"].(int) != 1 {
		t.Fatalf("top-level recentItems: want count 1, got %v", payload["count"])
	}
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatalf("EnvelopeFromResult: %v", err)
	}
	if env.Meta == nil || env.Meta.Pagination == nil || !env.Meta.Pagination.EndpointExhausted || env.Meta.Pagination.NextToken != "" {
		t.Fatalf("terminal pagination = %#v", env)
	}
}

func TestRecentListResultPaginationContract(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
		known   bool
		exhaust bool
		token   string
	}{
		{
			name:    "pagination omitted stays unknown",
			data:    map[string]any{"recentItems": []any{}},
			known:   false,
			exhaust: false,
		},
		{
			name:    "zero cursor sentinel is terminal",
			data:    map[string]any{"recentItems": []any{}, "hasMore": false, "nextCursor": float64(0)},
			known:   true,
			exhaust: true,
		},
		{
			name:    "continuation is resumable",
			data:    map[string]any{"recentItems": []any{}, "hasMore": true, "nextCursor": float64(42)},
			known:   true,
			exhaust: false,
			token:   "42",
		},
		{
			name:    "token only continuation is resumable",
			data:    map[string]any{"recentItems": []any{}, "nextCursor": "next"},
			known:   true,
			exhaust: false,
			token:   "next",
		},
		{
			name:    "continuation without cursor fails closed",
			data:    map[string]any{"recentItems": []any{}, "hasMore": true},
			wantErr: true,
		},
		{
			name:    "terminal with continuation fails closed",
			data:    map[string]any{"recentItems": []any{}, "hasMore": false, "nextCursor": "next"},
			wantErr: true,
		},
		{
			name:    "nested contradiction fails closed",
			data:    map[string]any{"recentItems": []any{}, "hasMore": false, "result": map[string]any{"recentItems": []any{}, "hasMore": true, "nextCursor": "next"}},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, result, err := recentListResult(tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want pagination error")
				}
				return
			}
			if err != nil {
				t.Fatalf("recentListResult: %v", err)
			}
			if payload["pagination_known"] != tc.known {
				t.Fatalf("pagination_known=%#v, want %v", payload["pagination_known"], tc.known)
			}
			env, err := output.EnvelopeFromResult(result)
			if err != nil {
				t.Fatalf("EnvelopeFromResult: %v", err)
			}
			if !tc.known {
				if env.Meta == nil || env.Meta.Pagination != nil {
					t.Fatalf("unknown pagination meta = %#v", env.Meta)
				}
				return
			}
			page := env.Meta.Pagination
			if page.EndpointExhausted != tc.exhaust || page.NextToken != tc.token {
				t.Fatalf("pagination=%#v want exhausted=%v token=%q", page, tc.exhaust, tc.token)
			}
		})
	}
}

func TestRecentListResultRejectsMalformedRows(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"non array recent items": {"recentItems": map[string]any{}},
		"non object entry":       {"recentItems": []any{"not-a-document"}},
		"missing node id":        {"recentItems": []any{map[string]any{"name": "untargetable"}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := recentListResult(data); err == nil {
				t.Fatal("malformed recent item should fail closed")
			}
		})
	}
}

func TestRecentListRolloutIsUnifiedActive(t *testing.T) {
	if Recent.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("recent rollout=%q, want unified_active", Recent.OutputRollout)
	}
}
