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

package helpers

import "testing"

func TestNormalizeSkillName(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		" Create Plan ":               "create-plan",
		"implementation__strategy":    "implementation-strategy",
		"code-change verification":    "code-change-verification",
		"self---healing___executor  ": "self-healing-executor",
		"@#$%^":                       "",
		"":                            "",
	}

	for input, want := range cases {
		got := NormalizeSkillName(input)
		if got != want {
			t.Fatalf("NormalizeSkillName(%q) = %q, want %q", input, got, want)
		}
	}
}
