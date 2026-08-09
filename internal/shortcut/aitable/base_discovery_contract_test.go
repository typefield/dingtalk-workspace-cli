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

package aitable

import (
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestBaseDiscoveryPayloadDoesNotClaimAuthoritativeInventory(t *testing.T) {
	bases := []map[string]any{{"baseId": "base-1", "baseName": "recent"}}
	payload, err := baseDiscoveryPayload("list_bases", map[string]any{
		"data": map[string]any{
			"hasMore":    false,
			"nextCursor": "",
		},
	}, bases, "recently_accessed")
	if err != nil {
		t.Fatal(err)
	}

	if payload["sourceKind"] != "recently_accessed" || payload["authoritativeInventory"] != false ||
		payload["inventoryCoverageKnown"] != false {
		t.Fatalf("inventory boundary = %#v", payload)
	}
	if payload["paginationKnown"] != true || payload["endpointExhausted"] != true || payload["hasMore"] != false {
		t.Fatalf("pagination facts = %#v", payload)
	}
	if _, exists := payload["complete"]; exists {
		t.Fatalf("base discovery must not publish semantically broad complete: %#v", payload)
	}
}

func TestBaseDiscoveryPayloadKeepsUnknownPaginationAndIndexCoverageHonest(t *testing.T) {
	payload, err := baseDiscoveryPayload("search_bases", map[string]any{
		"result": map[string]any{
			"bases": []any{},
		},
	}, []map[string]any{}, "name_search_index")
	if err != nil {
		t.Fatal(err)
	}

	if payload["paginationKnown"] != false || payload["indexCoverageKnown"] != false ||
		payload["authoritativeInventory"] != false {
		t.Fatalf("unknown discovery facts = %#v", payload)
	}
	if _, exists := payload["endpointExhausted"]; exists {
		t.Fatalf("unknown pagination must not claim exhaustion: %#v", payload)
	}
}

func TestBaseDiscoveryPayloadRejectsContradictoryOrUnresumablePagination(t *testing.T) {
	cases := []struct {
		name string
		data map[string]any
	}{
		{
			name: "open page missing cursor",
			data: map[string]any{"hasMore": true},
		},
		{
			name: "terminal page has continuation",
			data: map[string]any{"hasMore": false, "nextCursor": "next"},
		},
		{
			name: "outer and nested has more disagree",
			data: map[string]any{
				"hasMore":    true,
				"nextCursor": "outer-next",
				"data":       map[string]any{"hasMore": false},
			},
		},
		{
			name: "invalid has more type",
			data: map[string]any{"hasMore": "true", "nextCursor": "next"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := baseDiscoveryPayload("list_bases", tc.data, []map[string]any{}, "recently_accessed")
			typed, ok := err.(*apperrors.Error)
			if !ok || typed.StableSubtype != string(apperrors.SubtypePaginationInconsistent) || !typed.RetryableSet || typed.Retryable {
				t.Fatalf("error = %#v, want non-retryable pagination_inconsistent", err)
			}
		})
	}
}
