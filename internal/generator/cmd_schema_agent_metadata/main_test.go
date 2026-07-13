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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/generator/agentmetadata"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/generator/outputguard"
)

func TestLoadEffectiveCommandRegistryProjectionReconcilesAliases(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	registry, err := loadEffectiveCommandRegistryProjection(root, "internal/cli/schema_command_registry.json", true)
	if err != nil {
		t.Fatalf("loadEffectiveCommandRegistryProjection() error = %v", err)
	}
	if registry.ToolCount == 0 || !registry.ProductIDs["calendar"] {
		t.Fatalf("registry = %#v", registry)
	}
	if got := registry.ToolPaths["aitable record list"]; got != "aitable record query" {
		t.Fatalf("alias primary path = %q, want aitable record query", got)
	}
	if got := registry.ToolPaths["aitable.query_records"]; got != "aitable record query" {
		t.Fatalf("canonical primary path = %q, want aitable record query", got)
	}
	if registry.Hash == "" {
		t.Fatalf("registry hash is empty: %#v", registry)
	}
}

func TestLoadEffectiveCommandRegistryProjectionRejectsCompatibilityDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema_command_registry.json")
	if err := os.WriteFile(path, []byte(`{"$schema":"./schema_command_registry.schema.json","version":1,"products":[{"id":"sample","tools":[{"canonical_path":"sample.run","cli_path":"sample run"}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadEffectiveCommandRegistryProjection(".", path, true)
	if err == nil || !strings.Contains(err.Error(), "disagrees with the embedded") {
		t.Fatalf("compatibility drift error = %v", err)
	}
	if _, err := loadEffectiveCommandRegistryProjection(".", "", false); err == nil || !strings.Contains(err.Error(), "cannot be disabled") {
		t.Fatalf("disabled registry validation error = %v", err)
	}
}

func TestProjectEffectiveCommandRegistryKeepsManualOnlyCommand(t *testing.T) {
	effective := cli.EffectiveCommandRegistry{Commands: []cli.CommandSpec{
		{
			CanonicalPath:  "base.get_item",
			PrimaryCLIPath: "base item get",
			Visibility:     cli.SchemaVisibilityPublic,
			Source:         "reviewed_command_registry",
		},
		{
			CanonicalPath:  "helper.add_item",
			PrimaryCLIPath: "helper item add",
			Visibility:     cli.SchemaVisibilityPublic,
			Source:         "reviewed_manual_hint",
		},
	}}

	projection := projectEffectiveCommandRegistry(effective)
	if projection.ToolCount != 2 {
		t.Fatalf("ToolCount = %d, want 2", projection.ToolCount)
	}
	if got := projection.ToolPaths["helper.add_item"]; got != "helper item add" {
		t.Fatalf("manual-only canonical projection = %q", got)
	}
	if !projection.ProductIDs["helper"] {
		t.Fatal("manual-only product was dropped from Agent metadata projection")
	}
}

func TestWriteMetadataDirectorySplitsDomains(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stale.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("WriteFile(stale) error = %v", err)
	}
	metadata := agentmetadata.File{
		Version:    1,
		SourceHash: "sha256:test",
		Products: map[string]agentmetadata.ProductMetadata{
			"calendar": {UseWhen: []string{"日程"}},
			"contact":  {UseWhen: []string{"联系人"}},
		},
		Tools: map[string]agentmetadata.ToolMetadata{
			"calendar event create": {Effect: "write"},
			"contact user get-self": {Effect: "read"},
		},
	}
	if err := writeMetadataDirectory(dir, metadata); err != nil {
		t.Fatalf("writeMetadataDirectory() error = %v", err)
	}

	indexData, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("ReadFile(index) error = %v", err)
	}
	var index agentMetadataIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		t.Fatalf("json.Unmarshal(index) error = %v", err)
	}
	if got := index.Domains; len(got) != 2 || got[0] != "calendar" || got[1] != "contact" {
		t.Fatalf("domains = %#v", got)
	}

	calendarData, err := os.ReadFile(filepath.Join(dir, "calendar.json"))
	if err != nil {
		t.Fatalf("ReadFile(calendar) error = %v", err)
	}
	var calendar agentMetadataDomain
	if err := json.Unmarshal(calendarData, &calendar); err != nil {
		t.Fatalf("json.Unmarshal(calendar) error = %v", err)
	}
	if calendar.ProductID != "calendar" || len(calendar.Tools) != 1 || calendar.Tools["calendar event create"].Effect != "write" {
		t.Fatalf("calendar metadata = %#v", calendar)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected stale metadata file: %v", err)
	}
}

func TestValidateManualHintsOutputIsolationRejectsOverlaps(t *testing.T) {
	root := t.TempDir()
	manualRelative := filepath.Join("internal", "cli", "schema_manual_hints.json")
	manualPath := filepath.Join(root, manualRelative)
	if err := os.MkdirAll(filepath.Dir(manualPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const manualContents = `{"reviewed":true}`
	if err := os.WriteFile(manualPath, []byte(manualContents), 0o644); err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(filepath.Dir(manualPath), "stale.json")
	if err := os.WriteFile(stalePath, []byte(`{"stale":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	fileAlias := filepath.Join(root, "manual-file-alias.json")
	if err := os.Symlink(manualPath, fileAlias); err != nil {
		t.Fatal(err)
	}
	directoryAlias := filepath.Join(root, "manual-directory-alias")
	if err := os.Symlink(filepath.Dir(manualPath), directoryAlias); err != nil {
		t.Fatal(err)
	}
	hardLink := filepath.Join(root, "manual-hard-link.json")
	if err := os.Link(manualPath, hardLink); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		output      string
		outputDir   string
		auditOutput string
		want        string
	}{
		{
			name:   "clean absolute output path",
			output: filepath.Join(filepath.Dir(manualPath), "..", "cli", filepath.Base(manualPath)),
			want:   "-output",
		},
		{
			name:        "audit output",
			auditOutput: manualPath,
			want:        "-audit-output",
		},
		{
			name:      "output directory contains source",
			outputDir: filepath.Dir(manualPath),
			want:      "contains reviewed manual",
		},
		{
			name:   "file symlink",
			output: fileAlias,
			want:   "-output",
		},
		{
			name:      "directory symlink",
			outputDir: directoryAlias,
			want:      "contains reviewed manual",
		},
		{
			name:   "hard link",
			output: hardLink,
			want:   "-output",
		},
		{
			name:   "directory passed as file output",
			output: filepath.Dir(manualPath),
			want:   "contains reviewed manual",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateManualHintsOutputIsolation(root, manualRelative, tt.output, tt.outputDir, tt.auditOutput)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateManualHintsOutputIsolation() error = %v, want %q", err, tt.want)
			}
		})
	}

	manualAfter, err := os.ReadFile(manualPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(manualAfter) != manualContents {
		t.Fatalf("manual source changed: %q", manualAfter)
	}
	if staleAfter, err := os.ReadFile(stalePath); err != nil || string(staleAfter) != `{"stale":true}` {
		t.Fatalf("stale output was touched before preflight completed: contents=%q err=%v", staleAfter, err)
	}
}

func TestValidateManualHintsOutputIsolationAllowsSeparateTargets(t *testing.T) {
	root := t.TempDir()
	manualRelative := filepath.Join("inputs", "schema_manual_hints.json")
	manualPath := filepath.Join(root, manualRelative)
	if err := os.MkdirAll(filepath.Dir(manualPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manualPath, []byte(`{"reviewed":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	outputDirParent := filepath.Join(root, "real-output-parent")
	if err := os.MkdirAll(outputDirParent, 0o755); err != nil {
		t.Fatal(err)
	}
	outputDirAlias := filepath.Join(root, "output-parent-alias")
	if err := os.Symlink(outputDirParent, outputDirAlias); err != nil {
		t.Fatal(err)
	}
	err := validateManualHintsOutputIsolation(
		root,
		manualRelative,
		filepath.Join(root, "metadata.json"),
		filepath.Join(outputDirAlias, "split"),
		filepath.Join(root, "audit", "metadata-audit.json"),
	)
	if err != nil {
		t.Fatalf("validateManualHintsOutputIsolation() error = %v", err)
	}
}

func TestValidateAgentMetadataOutputIsolationProtectsAllSourceKinds(t *testing.T) {
	root := t.TempDir()
	skillPath := filepath.Join(root, "skills/mono/SKILL.md")
	hintsDir := filepath.Join(root, "internal/cli/schema_hints")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(hintsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, []byte("# Skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hintsDir, "reviewed.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	inputs := []outputguard.Input{
		{Name: "main Skill input", Path: "skills/mono/SKILL.md"},
		{Name: "structured hint input directory", Path: "internal/cli/schema_hints"},
	}
	for _, test := range []struct {
		name      string
		output    string
		outputDir string
		want      string
	}{
		{name: "skill file", output: skillPath, want: "main Skill input"},
		{name: "hint directory", outputDir: hintsDir, want: "structured hint input directory"},
		{name: "inside hint directory", output: filepath.Join(hintsDir, "generated.json"), want: "structured hint input directory"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateAgentMetadataOutputIsolation(root, inputs, test.output, test.outputDir, "")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateAgentMetadataOutputIsolation() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateManualAgentHintInputRejectsAlternateCopy(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, "internal/cli/schema_manual_hints.json")
	if err := os.MkdirAll(filepath.Dir(canonical), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"version":1,"commands":[],"agent_hints":{}}`)
	if err := os.WriteFile(canonical, body, 0o644); err != nil {
		t.Fatal(err)
	}
	alternate := filepath.Join(root, "alternate.json")
	if err := os.WriteFile(alternate, body, 0o644); err != nil {
		t.Fatal(err)
	}
	err := validateManualAgentHintInput(root, "alternate.json", commandRegistryProjection{})
	if err == nil || !strings.Contains(err.Error(), "must resolve to canonical reviewed source") {
		t.Fatalf("validateManualAgentHintInput() error = %v", err)
	}

	symlink := filepath.Join(root, "manual-link.json")
	if err := os.Symlink(canonical, symlink); err != nil {
		t.Fatal(err)
	}
	err = validateManualAgentHintInput(root, "manual-link.json", commandRegistryProjection{})
	if err == nil || strings.Contains(err.Error(), "must resolve to canonical reviewed source") {
		t.Fatalf("canonical symlink identity check error = %v", err)
	}
}

func TestValidateAgentMetadataOutputAllowlist(t *testing.T) {
	root := t.TempDir()
	canonicalDir := filepath.Join(root, "internal/cli/schema_agent_metadata")
	canonicalAudit := filepath.Join(root, "internal/cli/schema_agent_metadata_audit.json")
	if err := validateAgentMetadataOutputAllowlist(root, "", canonicalDir, canonicalAudit); err != nil {
		t.Fatalf("canonical outputs rejected: %v", err)
	}
	if err := validateAgentMetadataOutputAllowlist(root, "", filepath.Join(root, "internal/cli/schema_hints"), ""); err == nil || !strings.Contains(err.Error(), "not a canonical generated delivery target") {
		t.Fatalf("non-canonical repository output error = %v", err)
	}
	if err := validateAgentMetadataOutputAllowlist(root, "", filepath.Join(t.TempDir(), "metadata"), ""); err != nil {
		t.Fatalf("external temporary output rejected: %v", err)
	}
}
