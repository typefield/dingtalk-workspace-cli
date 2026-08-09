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
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestSearchFilesResultPaginationContract(t *testing.T) {
	files := []map[string]any{{"dentryId": "f1", "name": "report"}}
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
		known   bool
		exhaust bool
		token   string
	}{
		{
			name:    "continuation is resumable",
			data:    map[string]any{"hasMore": true, "nextCursor": "c2"},
			known:   true,
			exhaust: false,
			token:   "c2",
		},
		{
			name:    "absent pagination facts remain unknown",
			data:    map[string]any{},
			known:   false,
			exhaust: false,
		},
		{
			name:    "cursor without has more fails closed",
			data:    map[string]any{"nextCursor": "c2"},
			wantErr: true,
		},
		{
			name:    "continuation without cursor fails closed",
			data:    map[string]any{"hasMore": true},
			wantErr: true,
		},
		{
			name:    "terminal with cursor fails closed",
			data:    map[string]any{"hasMore": false, "nextCursor": "c2"},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, result, err := searchFilesResult(tc.data, files)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want pagination error")
				}
				return
			}
			if err != nil {
				t.Fatalf("searchFilesResult: %v", err)
			}
			if payload["pagination_known"] != tc.known {
				t.Fatalf("pagination_known=%#v, want %v", payload["pagination_known"], tc.known)
			}
			env, err := output.EnvelopeFromResult(result)
			if err != nil {
				t.Fatalf("EnvelopeFromResult: %v", err)
			}
			if env.Meta == nil || env.Meta.Count == nil || *env.Meta.Count != 1 {
				t.Fatalf("count meta=%#v, want 1", env.Meta)
			}
			if !tc.known {
				if env.Meta.Pagination != nil {
					t.Fatalf("unknown pagination should omit meta.pagination: %#v", env.Meta)
				}
				return
			}
			page := env.Meta.Pagination
			if page == nil || page.EndpointExhausted != tc.exhaust || page.NextToken != tc.token {
				t.Fatalf("pagination=%#v, want exhausted=%v token=%q", page, tc.exhaust, tc.token)
			}
		})
	}
}

func TestListFilesResultPaginationAndScopeContract(t *testing.T) {
	files := []map[string]any{{"dentryId": "f1", "name": "report"}}
	tests := []struct {
		name    string
		data    map[string]any
		wantErr bool
		known   bool
		exhaust bool
		token   string
	}{
		{
			name:    "continuation is resumable",
			data:    map[string]any{"hasMore": true, "nextCursor": "c2"},
			known:   true,
			exhaust: false,
			token:   "c2",
		},
		{
			name:    "terminal page is endpoint exhausted",
			data:    map[string]any{"hasMore": false},
			known:   true,
			exhaust: true,
		},
		{
			name:  "absent pagination facts remain unknown",
			data:  map[string]any{},
			known: false,
		},
		{
			name:    "cursor without has more fails closed",
			data:    map[string]any{"nextCursor": "c2"},
			wantErr: true,
		},
		{
			name:    "continuation without cursor fails closed",
			data:    map[string]any{"hasMore": true},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, result, err := listFilesResult(tc.data, files)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want pagination error")
				}
				return
			}
			if err != nil {
				t.Fatalf("listFilesResult: %v", err)
			}
			if payload["pagination_known"] != tc.known {
				t.Fatalf("pagination_known=%#v, want %v", payload["pagination_known"], tc.known)
			}
			if payload["inventory_scope"] != "requested_location" {
				t.Fatalf("inventory_scope=%#v, want requested_location", payload["inventory_scope"])
			}
			env, err := output.EnvelopeFromResult(result)
			if err != nil {
				t.Fatalf("EnvelopeFromResult: %v", err)
			}
			if env.Meta == nil || env.Meta.Count == nil || *env.Meta.Count != 1 {
				t.Fatalf("count meta=%#v, want 1", env.Meta)
			}
			if !tc.known {
				if env.Meta.Pagination != nil {
					t.Fatalf("unknown pagination should omit meta.pagination: %#v", env.Meta)
				}
				return
			}
			page := env.Meta.Pagination
			if page == nil || page.EndpointExhausted != tc.exhaust || page.NextToken != tc.token {
				t.Fatalf("pagination=%#v, want exhausted=%v token=%q", page, tc.exhaust, tc.token)
			}
		})
	}
}

func TestSearchFilesProjectRetainsStableTargets(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]any
		wantKey string
		wantErr bool
	}{
		{
			name:    "file has dentry id",
			data:    map[string]any{"items": []any{map[string]any{"dentryUuid": "f1", "fileName": "report"}}},
			wantKey: "dentryId",
		},
		{
			name:    "space has space id",
			data:    map[string]any{"items": []any{map[string]any{"spaceId": "s1", "name": "team space"}}},
			wantKey: "spaceId",
		},
		{
			name:    "row without target id fails closed",
			data:    map[string]any{"items": []any{map[string]any{"name": "untargetable"}}},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files, err := searchFilesProject(tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want projection error")
				}
				return
			}
			if err != nil {
				t.Fatalf("searchFilesProject: %v", err)
			}
			if len(files) != 1 || files[0][tc.wantKey] == nil {
				t.Fatalf("projected files=%#v, want stable %s", files, tc.wantKey)
			}
		})
	}
}

func TestSearchRolloutIsUnifiedActive(t *testing.T) {
	if Search.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("search rollout=%q, want unified_active", Search.OutputRollout)
	}
}

func TestListRolloutIsDualValidate(t *testing.T) {
	if List.OutputRollout != output.RolloutDualValidate {
		t.Fatalf("list rollout=%q, want dual_validate", List.OutputRollout)
	}
}
