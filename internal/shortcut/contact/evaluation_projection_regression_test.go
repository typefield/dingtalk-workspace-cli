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

package contact

import (
	"fmt"
	"testing"
)

// TestEvaluationProjectionCountsEightFiveOne locks the exact lower-layer
// cardinalities called out by the 2026-08 evaluation: eight roles, five role
// members and one department member. Fixtures are constructed as Go values so
// the repository does not need a persisted response JSON snapshot.
func TestEvaluationProjectionCountsEightFiveOne(t *testing.T) {
	rolesRaw := make([]any, 0, 8)
	for i := 1; i <= 8; i++ {
		rolesRaw = append(rolesRaw, map[string]any{
			"labelId": i,
			"name":    fmt.Sprintf("role-%d", i),
		})
	}
	roles, known := listRolesProjectWithStatus(map[string]any{
		"result": []any{map[string]any{"groupName": "default", "labels": rolesRaw}},
	})
	assertProjectionCount(t, "roles", roles, known, 8)

	roleMembersRaw := make([]any, 0, 5)
	for i := 1; i <= 5; i++ {
		roleMembersRaw = append(roleMembersRaw, projectedMemberFixture("role", i))
	}
	roleMembers, known := memberListProjectWithStatus(map[string]any{
		"labelUserList": roleMembersRaw,
	})
	assertProjectionCount(t, "role members", roleMembers, known, 5)

	deptMembers, known := memberListProjectWithStatus(map[string]any{
		"deptUserList": []any{projectedMemberFixture("dept", 1)},
	})
	assertProjectionCount(t, "department members", deptMembers, known, 1)
}

func TestProjectionStatusRejectsRecognizedContainerWithDroppedRows(t *testing.T) {
	if projected, known := listRolesProjectWithStatus(map[string]any{
		"result": []any{map[string]any{"unexpected": "role"}},
	}); known || len(projected) != 0 {
		t.Fatalf("unprojectable role rows = %#v, known=%v; want fail-closed", projected, known)
	}

	if projected, known := memberListProjectWithStatus(map[string]any{
		"members": []any{
			projectedMemberFixture("known", 1),
			map[string]any{"unexpected": "member"},
		},
	}); known || len(projected) != 1 {
		t.Fatalf("partially projected member rows = %#v, known=%v; want fail-closed", projected, known)
	}
}

func projectedMemberFixture(prefix string, index int) map[string]any {
	return map[string]any{"userInfo": map[string]any{
		"userId": fmt.Sprintf("%s-user-%d", prefix, index),
		"name":   fmt.Sprintf("%s-name-%d", prefix, index),
	}}
}

func assertProjectionCount(t *testing.T, name string, projected []map[string]any, known bool, want int) {
	t.Helper()
	if !known || len(projected) != want {
		t.Fatalf("%s projection count = %d, known=%v; want %d and known", name, len(projected), known, want)
	}
	for i, row := range projected {
		if row["labelId"] == nil && row["userId"] == nil {
			t.Fatalf("%s row %d has no stable id: %#v", name, i, row)
		}
	}
}
