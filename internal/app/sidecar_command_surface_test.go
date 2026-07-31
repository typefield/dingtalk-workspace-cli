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

package app

import (
	"sort"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/authsidecar"
)

// TestSidecarCommandClassificationCoversCommandTree is the reverse-completeness
// gate for sidecar mode: every executable top-level command must be reviewed as
// either allowed or denied. A new command must not become reachable in an
// untrusted sandbox just because nobody updated the sidecar allowlist.
func TestSidecarCommandClassificationCoversCommandTree(t *testing.T) {
	allowed, denied := authsidecar.SidecarCommandClassification()
	classified := make(map[string]string, len(allowed)+len(denied))
	for _, name := range allowed {
		classified[name] = "allowed"
	}
	for _, name := range denied {
		if previous, duplicate := classified[name]; duplicate {
			t.Fatalf("command %q is classified both %s and denied", name, previous)
		}
		classified[name] = "denied"
	}

	live := map[string]struct{}{"help": {}}
	for _, command := range NewRootCommand().Commands() {
		live[command.Name()] = struct{}{}
		for _, alias := range command.Aliases {
			// Cobra reports the canonical name in CommandPath(), so an alias
			// must never widen the sidecar surface.
			if _, ok := classified[alias]; ok {
				t.Errorf("alias %q of command %q must not be classified directly", alias, command.Name())
			}
		}
	}

	var unclassified, stale []string
	for name := range live {
		if _, ok := classified[name]; !ok {
			unclassified = append(unclassified, name)
		}
	}
	for name := range classified {
		if _, ok := live[name]; !ok {
			stale = append(stale, name)
		}
	}
	sort.Strings(unclassified)
	sort.Strings(stale)
	if len(unclassified) > 0 {
		t.Errorf("commands missing a reviewed sidecar classification: %v", unclassified)
	}
	if len(stale) > 0 {
		t.Errorf("sidecar classification references commands that no longer exist: %v", stale)
	}
}

func TestSidecarModeDeniesUnclassifiedCommandByDefault(t *testing.T) {
	t.Setenv("DWS_AUTH_MODE", "sidecar")
	if err := authsidecar.ValidateCommandPath("dws brand-new-command run"); err == nil {
		t.Fatal("an unclassified command was allowed in sidecar mode")
	}
	if err := authsidecar.ValidateCommandPath("dws doc get"); err != nil {
		t.Fatalf("allowlisted MCP command rejected: %v", err)
	}
}
