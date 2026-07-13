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

package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestManualSchemaHintsIncludeExistingLeafAndOverrideParameters(t *testing.T) {
	root, leaf := manualSchemaHintTestTree()
	description := "Reviewed query text"
	property := "queryText"
	interfaceType := "object"
	required := true
	requiredWhen := "mode is advanced"
	snapshot := ManualSchemaHintSnapshot{
		Schema:  manualSchemaHintSchemaRef,
		Version: manualSchemaHintVersion,
		Commands: []ManualSchemaCommandHint{{
			CLIPath:       "sample item search",
			CanonicalPath: "sample.search_items",
			Reason:        "Reviewed public helper",
			Reviewed:      true,
			Parameters: map[string]ManualSchemaParameterHint{
				"query": {
					Description:   &description,
					Property:      &property,
					InterfaceType: &interfaceType,
					Required:      &required,
					RequiredWhen:  &requiredWhen,
				},
			},
		}},
	}

	report, err := applyManualSchemaHints(root, snapshot)
	if err != nil {
		t.Fatalf("applyManualSchemaHints() error = %v", err)
	}
	if len(report.Commands) != 1 || report.Commands[0] != "sample item search" {
		t.Fatalf("report.Commands = %#v", report.Commands)
	}
	productID, toolName, reason, ok := runtimeManualSchemaIdentity(leaf)
	if !ok || productID != "sample" || toolName != "search_items" || reason != "Reviewed public helper" {
		t.Fatalf("Schema identity = %s.%s (%s)", productID, toolName, reason)
	}

	payload, err := runtimeSchemaPayloadForTest(root, []string{"sample.search_items"})
	if err != nil {
		t.Fatalf("runtimeSchemaPayloadForTest() error = %v", err)
	}
	parameters := schemaMap(payload["parameters"])
	query := parameters["query"]
	if query["description"] != description || query["property"] != property || query["interface_type"] != interfaceType || query["required"] != true || query["required_when"] != requiredWhen {
		t.Fatalf("query parameter = %#v", query)
	}
	if leaf.Flags().Lookup("query").Usage != "Original query text" {
		t.Fatalf("human Schema hint changed Cobra help: %q", leaf.Flags().Lookup("query").Usage)
	}
	if len(leaf.Flags().Lookup("query").Annotations[cobra.BashCompOneRequiredFlag]) != 0 {
		t.Fatal("manual Schema hint changed Cobra execution validation")
	}
}

func TestManualSchemaHintsRejectInvalidOrStaleInputs(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ManualSchemaCommandHint)
		wantErr string
	}{
		{name: "missing command", mutate: func(h *ManualSchemaCommandHint) { h.CLIPath = "sample item missing" }, wantErr: "does not resolve"},
		{name: "wildcard command", mutate: func(h *ManualSchemaCommandHint) { h.CLIPath = "sample item *" }, wantErr: "invalid exact cli_path"},
		{name: "not reviewed", mutate: func(h *ManualSchemaCommandHint) { h.Reviewed = false }, wantErr: "not reviewed"},
		{name: "missing reason", mutate: func(h *ManualSchemaCommandHint) { h.Reason = "" }, wantErr: "has no reason"},
		{name: "bad canonical", mutate: func(h *ManualSchemaCommandHint) { h.CanonicalPath = "sample" }, wantErr: "invalid canonical_path"},
		{name: "missing flag", mutate: func(h *ManualSchemaCommandHint) {
			h.Parameters = map[string]ManualSchemaParameterHint{"missing": {Required: boolPointer(true)}}
		}, wantErr: "missing flag --missing"},
		{name: "invalid interface type", mutate: func(h *ManualSchemaCommandHint) {
			h.Parameters = map[string]ManualSchemaParameterHint{"query": {InterfaceType: stringPointer("made-up")}}
		}, wantErr: "unsupported interface_type"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, _ := manualSchemaHintTestTree()
			hint := ManualSchemaCommandHint{
				CLIPath:       "sample item search",
				CanonicalPath: "sample.search_items",
				Reason:        "Reviewed public helper",
				Reviewed:      true,
			}
			test.mutate(&hint)
			_, err := applyManualSchemaHints(root, ManualSchemaHintSnapshot{
				Schema:   manualSchemaHintSchemaRef,
				Version:  manualSchemaHintVersion,
				Commands: []ManualSchemaCommandHint{hint},
			})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestManualSchemaHintsRejectCanonicalConflict(t *testing.T) {
	root, leaf := manualSchemaHintTestTree()
	AttachRuntimeSchema(leaf, "sample", "existing", "test")
	_, err := applyManualSchemaHints(root, ManualSchemaHintSnapshot{
		Schema:  manualSchemaHintSchemaRef,
		Version: manualSchemaHintVersion,
		Commands: []ManualSchemaCommandHint{{
			CLIPath:       "sample item search",
			CanonicalPath: "sample.replacement",
			Reason:        "Reviewed public helper",
			Reviewed:      true,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "conflicts with existing canonical path") {
		t.Fatalf("error = %v", err)
	}
}

func TestManualSchemaHintsRejectAmbiguousCobraAlias(t *testing.T) {
	root, search := manualSchemaHintTestTree()
	search.Aliases = []string{"find"}
	list := &cobra.Command{Use: "list", Aliases: []string{"find"}, RunE: func(*cobra.Command, []string) error { return nil }}
	search.Parent().AddCommand(list)

	_, err := applyManualSchemaHints(root, ManualSchemaHintSnapshot{
		Schema:  manualSchemaHintSchemaRef,
		Version: manualSchemaHintVersion,
		Commands: []ManualSchemaCommandHint{{
			CLIPath:       "sample item find",
			CanonicalPath: "sample.search_items",
			Reason:        "Ambiguous alias fixture",
			Reviewed:      true,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), `Cobra alias segment "find" is ambiguous`) {
		t.Fatalf("error = %v", err)
	}
}

func TestManualSchemaHintsResolveReviewedCobraAliasExactly(t *testing.T) {
	root := commandRegistryTestRoot("aitable record query")
	exactSchemaCommand(root, "aitable record").Aliases = []string{"records"}
	leaf := exactSchemaCommand(root, "aitable record query")

	report, err := applyManualSchemaHints(root, ManualSchemaHintSnapshot{
		Schema:  manualSchemaHintSchemaRef,
		Version: manualSchemaHintVersion,
		Commands: []ManualSchemaCommandHint{{
			CLIPath:       "aitable records query",
			CanonicalPath: "aitable.query_records",
			Reason:        "Reviewed ancestor Cobra alias",
			Reviewed:      true,
		}},
	})
	if err != nil {
		t.Fatalf("applyManualSchemaHints() error = %v", err)
	}
	if got := strings.Join(report.Commands, ","); got != "aitable records query" {
		t.Fatalf("report commands = %q", got)
	}
	productID, toolName, _, ok := runtimeManualSchemaIdentity(leaf)
	if !ok || productID+"."+toolName != "aitable.query_records" {
		t.Fatalf("manual identity = %s.%s, ok=%v", productID, toolName, ok)
	}
}

func TestDecodeManualSchemaHintsRejectsUnknownFields(t *testing.T) {
	_, err := decodeManualSchemaHints([]byte(`{"$schema":"./schema_manual_hints.schema.json","version":1,"commands":[],"allow_virtual_commands":true}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeManualSchemaHintsRequiresDiscoverableSchema(t *testing.T) {
	_, err := decodeManualSchemaHints([]byte(`{"version":1,"commands":[]}`))
	if err == nil || !strings.Contains(err.Error(), "must declare $schema") {
		t.Fatalf("error = %v", err)
	}
}

func TestDecodeManualSchemaHintsRequiresAgentHints(t *testing.T) {
	_, err := decodeManualSchemaHints([]byte(`{"$schema":"./schema_manual_hints.schema.json","version":1,"commands":[]}`))
	if err == nil || !strings.Contains(err.Error(), "agent_hints.revisions must not be empty") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateManualAgentHintSetRequiresReviewedExactCoverage(t *testing.T) {
	hints := manualAgentHintSetFixture()
	if err := ValidateManualAgentHintSet(hints, map[string]bool{"sample": true}, map[string]bool{"sample.search_items": true}); err != nil {
		t.Fatalf("ValidateManualAgentHintSet() error = %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*ManualAgentHintSet)
		wantErr string
	}{
		{
			name: "unknown revision",
			mutate: func(hints *ManualAgentHintSet) {
				tool := hints.Tools["sample.search_items"]
				tool.Revision = "missing"
				hints.Tools["sample.search_items"] = tool
			},
			wantErr: "unknown revision",
		},
		{
			name: "unreviewed",
			mutate: func(hints *ManualAgentHintSet) {
				tool := hints.Tools["sample.search_items"]
				tool.Reviewed = false
				hints.Tools["sample.search_items"] = tool
			},
			wantErr: "must be reviewed",
		},
		{
			name: "missing examples",
			mutate: func(hints *ManualAgentHintSet) {
				tool := hints.Tools["sample.search_items"]
				tool.Examples = nil
				hints.Tools["sample.search_items"] = tool
			},
			wantErr: "requires non-empty examples",
		},
		{
			name: "confirmation bypass",
			mutate: func(hints *ManualAgentHintSet) {
				tool := hints.Tools["sample.search_items"]
				tool.Examples = []string{"dws sample item search --query x --yes"}
				hints.Tools["sample.search_items"] = tool
			},
			wantErr: "must not bypass confirmation",
		},
		{
			name: "missing Registry tool",
			mutate: func(hints *ManualAgentHintSet) {
				delete(hints.Tools, "sample.search_items")
			},
			wantErr: "missing=[sample.search_items]",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := manualAgentHintSetFixture()
			test.mutate(&candidate)
			err := ValidateManualAgentHintSet(candidate, map[string]bool{"sample": true}, map[string]bool{"sample.search_items": true})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestDecodeManualSchemaHintsRejectsUnknownManualAgentField(t *testing.T) {
	_, err := decodeManualSchemaHints([]byte(`{
  "$schema":"./schema_manual_hints.schema.json",
  "version":1,
  "commands":[],
  "agent_hints":{
    "revisions":{"v1":{"generated_by":"ai","model":"gpt-5","prompt_version":"v1","reason":"fixture"}},
    "products":{},
    "tools":{},
    "allow_generated_overwrite":true
  }
}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateManualAgentHintExamplesUsesBoundCobraContract(t *testing.T) {
	root, leaf := manualSchemaHintTestTree()
	root.PersistentFlags().String("format", "json", "output format")
	alias := &cobra.Command{Use: "find", RunE: func(*cobra.Command, []string) error { return nil }}
	alias.Flags().StringP("legacy-query", "l", "", "compatibility query")
	leaf.Parent().AddCommand(alias)
	boundSpec := BoundCommandSpec{
		CommandSpec: CommandSpec{
			CanonicalPath:  "sample.search_items",
			PrimaryCLIPath: "sample item search",
			Aliases:        []string{"sample item find"},
		},
		PrimaryCommand: leaf,
		AliasCommands: []BoundAlias{{
			Path:    "sample item find",
			Command: alias,
			Kind:    AliasKindCompatibilityLeaf,
		}},
	}
	bound := BoundCommandRegistry{
		Commands:    []BoundCommandSpec{boundSpec},
		ByCanonical: map[string]BoundCommandSpec{"sample.search_items": boundSpec},
		ByCLIPath: map[string]BoundCommandSpec{
			"sample item search": boundSpec,
			"sample item find":   boundSpec,
		},
	}
	hints := manualAgentHintSetFixture()
	tool := hints.Tools["sample.search_items"]
	tool.Examples = []string{"dws sample item find -l <item-id> --format json"}
	hints.Tools["sample.search_items"] = tool
	if err := ValidateManualAgentHintExamples(bound, hints); err != nil {
		t.Fatalf("ValidateManualAgentHintExamples() error = %v", err)
	}

	tool.Examples = []string{"dws sample item search -q 'value with spaces' --format=json"}
	hints.Tools["sample.search_items"] = tool
	if err := ValidateManualAgentHintExamples(bound, hints); err != nil {
		t.Fatalf("primary shorthand example error = %v", err)
	}

	tool.Examples = []string{"dws sample item search -l value"}
	hints.Tools["sample.search_items"] = tool
	if err := ValidateManualAgentHintExamples(bound, hints); err == nil || !strings.Contains(err.Error(), "unknown shorthand flag -l") {
		t.Fatalf("alias-only shorthand on primary error = %v", err)
	}

	tool.Examples = []string{"dws sample item search --invented value"}
	hints.Tools["sample.search_items"] = tool
	if err := ValidateManualAgentHintExamples(bound, hints); err == nil || !strings.Contains(err.Error(), "unknown flag --invented") {
		t.Fatalf("unknown flag error = %v", err)
	}

	tool.Examples = []string{"dws sample item list --query value"}
	hints.Tools["sample.search_items"] = tool
	if err := ValidateManualAgentHintExamples(bound, hints); err == nil || !strings.Contains(err.Error(), "does not use its reviewed") {
		t.Fatalf("wrong path error = %v", err)
	}
}

func TestValidateManualAgentHintExamplesRejectsUnsafeOrNonExecutingArgv(t *testing.T) {
	_, leaf := manualSchemaHintTestTree()
	boundSpec := BoundCommandSpec{
		CommandSpec: CommandSpec{
			CanonicalPath:  "sample.search_items",
			PrimaryCLIPath: "sample item search",
		},
		PrimaryCommand: leaf,
	}
	bound := BoundCommandRegistry{
		Commands:    []BoundCommandSpec{boundSpec},
		ByCanonical: map[string]BoundCommandSpec{"sample.search_items": boundSpec},
		ByCLIPath:   map[string]BoundCommandSpec{"sample item search": boundSpec},
	}
	tests := []struct {
		name    string
		example string
		wantErr string
	}{
		{name: "quoted operators are data", example: `dws sample item search --query "A && B > C"`},
		{name: "quoted multiline value is one argument", example: "dws sample item search --query '{\n\"key\": \"value\"\n}'"},
		{name: "placeholder is data", example: `dws sample item search --query <item-id>,<other_id>`},
		{name: "attached shorthand value", example: `dws sample item search -qvalue`},
		{name: "chaining", example: `dws sample item search --query x && dws sample item search --query y`, wantErr: "shell operator"},
		{name: "unquoted newline", example: "dws sample item search --query x\ndws sample item search --query y", wantErr: "unquoted newline shell operator"},
		{name: "semicolon", example: `dws sample item search --query x; dws sample item search --query y`, wantErr: "shell operator"},
		{name: "pipe", example: `dws sample item search --query x | tee out`, wantErr: "shell operator"},
		{name: "redirect", example: `dws sample item search --query x > out`, wantErr: "shell operator"},
		{name: "input redirect", example: `dws sample item search < input`, wantErr: "redirection operator"},
		{name: "command substitution", example: `dws sample item search --query $(whoami)`, wantErr: "shell expansion"},
		{name: "backticks", example: "dws sample item search --query `whoami`", wantErr: "shell expansion"},
		{name: "argument terminator", example: `dws sample item search -- --query x`, wantErr: "argument terminator"},
		{name: "help long", example: `dws sample item search --help`, wantErr: "not only --help"},
		{name: "help shorthand", example: `dws sample item search -h`, wantErr: "not only -h"},
		{name: "unknown shorthand", example: `dws sample item search -x value`, wantErr: "unknown shorthand flag -x"},
		{name: "unterminated quote", example: `dws sample item search --query "value`, wantErr: "unterminated quoted value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hints := manualAgentHintSetFixture()
			tool := hints.Tools["sample.search_items"]
			tool.Examples = []string{test.example}
			hints.Tools["sample.search_items"] = tool
			err := ValidateManualAgentHintExamples(bound, hints)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateManualAgentHintExamples() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func manualAgentHintSetFixture() ManualAgentHintSet {
	const revision = "ai-agent-contract-v1"
	return ManualAgentHintSet{
		Revisions: map[string]ManualAgentHintRevision{
			revision: {
				GeneratedBy:   "ai",
				Model:         "gpt-5",
				PromptVersion: "v1",
				Reason:        "Generate the reviewed fixture",
			},
		},
		Products: map[string]ManualAgentProductHint{
			"sample": {
				AgentSummary: "Manage sample items",
				UseWhen:      []string{"A sample item must be managed"},
				AvoidWhen:    []string{"The target is not a sample item"},
				Reviewed:     true,
				Revision:     revision,
				Reason:       "Reviewed product routing",
				Evidence:     []string{"sample product reference"},
			},
		},
		Tools: map[string]ManualAgentToolHint{
			"sample.search_items": {
				AgentSummary: "Search sample items",
				UseWhen:      []string{"Existing sample items must be found"},
				AvoidWhen:    []string{"A sample item must be created"},
				Examples:     []string{"dws sample item search --query value"},
				Reviewed:     true,
				Revision:     revision,
				Reason:       "Reviewed tool selection",
				Evidence:     []string{"dws sample item search --help"},
			},
		},
	}
}

func manualSchemaHintTestTree() (*cobra.Command, *cobra.Command) {
	root := &cobra.Command{Use: "dws"}
	product := &cobra.Command{Use: "sample"}
	group := &cobra.Command{Use: "item"}
	leaf := &cobra.Command{Use: "search", RunE: func(*cobra.Command, []string) error { return nil }}
	leaf.Flags().StringP("query", "q", "", "Original query text")
	group.AddCommand(leaf)
	product.AddCommand(group)
	root.AddCommand(product)
	return root, leaf
}

func boolPointer(value bool) *bool {
	return &value
}

func stringPointer(value string) *string {
	return &value
}
