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

import "testing"

func TestBaseDiscoveryPayloadDoesNotClaimAuthoritativeInventory(t *testing.T) {
	bases := []map[string]any{{"baseId": "base-1", "baseName": "recent"}}
	payload := baseDiscoveryPayload(map[string]any{
		"data": map[string]any{
			"hasMore":    false,
			"nextCursor": "",
		},
	}, bases, "recently_accessed")

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
	payload := baseDiscoveryPayload(map[string]any{
		"result": map[string]any{
			"bases": []any{},
		},
	}, []map[string]any{}, "name_search_index")

	if payload["paginationKnown"] != false || payload["indexCoverageKnown"] != false ||
		payload["authoritativeInventory"] != false {
		t.Fatalf("unknown discovery facts = %#v", payload)
	}
	if _, exists := payload["endpointExhausted"]; exists {
		t.Fatalf("unknown pagination must not claim exhaustion: %#v", payload)
	}
}
