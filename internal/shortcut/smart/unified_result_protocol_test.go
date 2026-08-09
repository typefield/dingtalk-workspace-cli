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

// assertNoLegacyProtocolMarker keeps a promoted result from accidentally
// carrying the old shortcut envelope discriminator inside its new data shape.
// A unified result has one framework contract; the legacy marker is neither
// business data nor a version selector for Agents.
func assertNoLegacyProtocolMarker(t *testing.T, value any) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "contractVersion" || key == "contract_version" {
				t.Fatalf("unified result leaked legacy protocol marker %q: %#v", key, value)
			}
			assertNoLegacyProtocolMarker(t, child)
		}
	case []any:
		for _, child := range typed {
			assertNoLegacyProtocolMarker(t, child)
		}
	}
}
