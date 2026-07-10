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
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSchemaCoversVisibleProductLeafCommands(t *testing.T) {
	root := NewRootCommand()
	visibleLeaves := visibleProductLeafPaths(root)

	schemaRoot := NewRootCommand()
	var out, errOut bytes.Buffer
	schemaRoot.SetOut(&out)
	schemaRoot.SetErr(&errOut)
	schemaRoot.SetArgs([]string{"schema", "--all", "--format", "json"})
	if err := schemaRoot.Execute(); err != nil {
		t.Fatalf("schema Execute() error = %v\nstderr:\n%s", err, errOut.String())
	}

	var payload struct {
		Products []struct {
			Tools []struct {
				CLIPath string   `json:"cli_path"`
				Aliases []string `json:"aliases"`
			} `json:"tools"`
		} `json:"products"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput:\n%s", err, out.String())
	}
	schemaPaths := map[string]bool{}
	for _, product := range payload.Products {
		for _, tool := range product.Tools {
			schemaPaths[tool.CLIPath] = true
			for _, alias := range tool.Aliases {
				schemaPaths[alias] = true
			}
		}
	}

	var missing []string
	for path := range visibleLeaves {
		if !schemaPaths[path] {
			missing = append(missing, path)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("visible product leaf commands missing schema (%d):\n%s", len(missing), strings.Join(missing, "\n"))
	}

	var extra []string
	for path := range schemaPaths {
		if !visibleLeaves[path] {
			extra = append(extra, path)
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Fatalf("schema paths not visible in product help (%d):\n%s", len(extra), strings.Join(extra, "\n"))
	}
}

func TestSchemaHidesChatFileUploadCompatibilityAliases(t *testing.T) {
	root := NewRootCommand()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"schema", "chat.upload_conversation_file", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("schema Execute() error = %v\nstderr:\n%s", err, errOut.String())
	}

	var payload struct {
		Parameters  map[string]json.RawMessage `json:"parameters"`
		Constraints struct {
			RequireOneOf [][]string `json:"require_one_of"`
		} `json:"constraints"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput:\n%s", err, out.String())
	}
	for _, name := range []string{"conversation-id", "id", "file-path"} {
		if _, ok := payload.Parameters[name]; ok {
			t.Fatalf("compatibility alias %q leaked into schema parameters", name)
		}
	}
	for _, name := range []string{"group", "user", "open-dingtalk-id", "file", "url"} {
		if _, ok := payload.Parameters[name]; !ok {
			t.Fatalf("canonical parameter %q missing from schema", name)
		}
	}
	wantGroups := map[string]bool{
		"group,user,open-dingtalk-id": true,
		"file,url":                    true,
	}
	for _, group := range payload.Constraints.RequireOneOf {
		delete(wantGroups, strings.Join(group, ","))
	}
	if len(wantGroups) != 0 {
		t.Fatalf("require_one_of missing groups: %#v", wantGroups)
	}
}

func TestSchemaProductIDsMatchRootHelpServices(t *testing.T) {
	root := NewRootCommand()
	helpProducts := map[string]bool{}
	for _, cmd := range visibleMCPRootCommands(root) {
		helpProducts[cmd.Name()] = true
	}

	schemaRoot := NewRootCommand()
	var out, errOut bytes.Buffer
	schemaRoot.SetOut(&out)
	schemaRoot.SetErr(&errOut)
	schemaRoot.SetArgs([]string{"schema", "--format", "json"})
	if err := schemaRoot.Execute(); err != nil {
		t.Fatalf("schema Execute() error = %v\nstderr:\n%s", err, errOut.String())
	}

	var payload struct {
		Products []struct {
			ID string `json:"id"`
		} `json:"products"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\noutput:\n%s", err, out.String())
	}
	schemaProducts := map[string]bool{}
	for _, product := range payload.Products {
		schemaProducts[product.ID] = true
	}

	var missing, extra []string
	for id := range helpProducts {
		if !schemaProducts[id] {
			missing = append(missing, id)
		}
	}
	for id := range schemaProducts {
		if !helpProducts[id] {
			extra = append(extra, id)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("schema products differ from root help services\nmissing from schema: %s\nextra in schema: %s", strings.Join(missing, ","), strings.Join(extra, ","))
	}
}

func visibleProductLeafPaths(root *cobra.Command) map[string]bool {
	out := map[string]bool{}
	for _, top := range root.Commands() {
		if !top.IsAvailableCommand() || top.Name() == "help" || schemaUtilityRoot(top.Name()) {
			continue
		}
		walkVisibleLeaves(top, func(leaf *cobra.Command) {
			if path := commandPathWithoutRoot(leaf); path != "" {
				out[path] = true
			}
		})
	}
	return out
}

func walkVisibleLeaves(cmd *cobra.Command, fn func(*cobra.Command)) {
	if cmd.Runnable() && !cmd.HasAvailableSubCommands() {
		fn(cmd)
		return
	}
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() || sub.Name() == "help" {
			continue
		}
		walkVisibleLeaves(sub, fn)
	}
}

func commandPathWithoutRoot(cmd *cobra.Command) string {
	var parts []string
	for c := cmd; c != nil && c.HasParent(); c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}
	return strings.Join(parts, " ")
}

func schemaUtilityRoot(name string) bool {
	switch name {
	case "api", "auth", "cache", "completion", "config", "doctor", "event", "help", "plugin", "recovery", "schema", "skill", "version":
		return true
	default:
		return false
	}
}
