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

package smart

import "testing"

func TestLatestMinutesTaskUUIDHandlesResultItemListAndKnownEmpty(t *testing.T) {
	uuid, err := latestMinutesTaskUUID(map[string]any{
		"result": map[string]any{"itemList": []any{
			map[string]any{"taskUuid": "older", "createTime": float64(10)},
			map[string]any{"taskUuid": "newer", "createTime": float64(20)},
		}},
	})
	if err != nil || uuid != "newer" {
		t.Fatalf("itemList uuid=%q err=%v, want newer", uuid, err)
	}

	uuid, err = latestMinutesTaskUUID(map[string]any{
		"result": map[string]any{"itemList": []any{}},
	})
	if err != nil || uuid != "" {
		t.Fatalf("known empty uuid=%q err=%v", uuid, err)
	}
}

func TestLatestMinutesTaskUUIDRejectsUnknownOrUntargetableResponse(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"unknown container": {"result": map[string]any{"notice": "unrecognized"}},
		"malformed row":     {"result": map[string]any{"itemList": []any{"invalid"}}},
		"missing task uuid": {"result": map[string]any{"itemList": []any{map[string]any{"title": "display-only"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := latestMinutesTaskUUID(data)
			assertMinutesSearchProjectionUnknown(t, err)
		})
	}
}
