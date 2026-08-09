// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package shortcut

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/cobra"
)

type dualValidateCaller struct {
	format string
	dryRun bool
	calls  int
	text   string
	result *edition.ToolResult
}

func (c *dualValidateCaller) CallTool(context.Context, string, string, map[string]any) (*edition.ToolResult, error) {
	c.calls++
	if c.result != nil {
		return c.result, nil
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: c.text}}}, nil
}

func (c *dualValidateCaller) Format() string { return c.format }
func (c *dualValidateCaller) DryRun() bool   { return c.dryRun }
func (c *dualValidateCaller) Fields() string { return "" }
func (c *dualValidateCaller) JQ() string     { return "" }

func runShortcutAtRollout(t *testing.T, caller *dualValidateCaller, rollout output.RolloutState, args ...string) (string, string) {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	var stdout, stderr bytes.Buffer
	helpers.GetFormatter().SetWriters(&stdout, &stderr)

	root := &cobra.Command{Use: "dws", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().String("format", "json", "")
	root.PersistentFlags().Bool("dry-run", false, "")
	cmd := mount(Shortcut{
		Service:       "sample",
		Command:       "+compat",
		OutputRollout: rollout,
		Execute: func(rt *RuntimeContext) error {
			return rt.CallMCP("get_sample", map[string]any{"id": "1"})
		},
	})
	cmd.SetOut(&stdout)
	root.AddCommand(cmd)
	root.SetArgs(append([]string{cmd.Name()}, args...))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	return stdout.String(), stderr.String()
}

func runDualValidateShortcut(t *testing.T, caller *dualValidateCaller, args ...string) (string, string) {
	t.Helper()
	return runShortcutAtRollout(t, caller, output.RolloutDualValidate, args...)
}

func TestDualValidatePreservesLegacyRawBytesWithOneBusinessCall(t *testing.T) {
	caller := &dualValidateCaller{format: "raw", text: `{"url":"https://example.test/?a=1&b=2"}`}
	got, _ := runDualValidateShortcut(t, caller, "--format", "raw")
	if want := caller.text + "\n"; got != want {
		t.Fatalf("raw output = %q, want legacy bytes %q", got, want)
	}
	if caller.calls != 1 {
		t.Fatalf("business calls = %d, want 1", caller.calls)
	}
}

func TestDualValidatePreservesLegacyPlainTextFormat(t *testing.T) {
	caller := &dualValidateCaller{format: "table", text: "legacy plain text"}
	got, _ := runDualValidateShortcut(t, caller, "--format", "table")
	if got != "legacy plain text\n" {
		t.Fatalf("table/plain output = %q", got)
	}
	if caller.calls != 1 {
		t.Fatalf("business calls = %d, want 1", caller.calls)
	}
}

func TestDualValidateDoesNotJSONQuoteLegacyPlainText(t *testing.T) {
	caller := &dualValidateCaller{format: "json", text: "legacy plain text"}
	got, _ := runDualValidateShortcut(t, caller, "--format", "json")
	if got != "legacy plain text\n" {
		t.Fatalf("json/plain output = %q", got)
	}
	if caller.calls != 1 {
		t.Fatalf("business calls = %d, want 1", caller.calls)
	}
}

func TestDualValidatePreservesLegacyDryRunWithoutBusinessCall(t *testing.T) {
	caller := &dualValidateCaller{format: "table", dryRun: true}
	stdout, stderr := runDualValidateShortcut(t, caller, "--format", "table", "--dry-run")
	if stdout != "" || !strings.Contains(stderr, "Tool") || !strings.Contains(stderr, "get_sample") || !strings.Contains(stderr, "Arguments") {
		t.Fatalf("dry-run output lost legacy presentation: stdout=%q stderr=%q", stdout, stderr)
	}
	if caller.calls != 0 {
		t.Fatalf("dry-run business calls = %d, want 0", caller.calls)
	}
}

func TestDualValidateStructuredJSONIsByteIdenticalToLegacy(t *testing.T) {
	text := `{"url":"https://example.test/?a=1&b=<tag>","label":"A > B"}`
	legacyCaller := &dualValidateCaller{format: "json", text: text}
	legacy, legacyErr := runShortcutAtRollout(t, legacyCaller, output.RolloutLegacyOnly, "--format", "json")
	dualCaller := &dualValidateCaller{format: "json", text: text}
	dual, dualErr := runDualValidateShortcut(t, dualCaller, "--format", "json")
	if legacyErr != dualErr {
		t.Fatalf("stderr differs: legacy=%q dual=%q", legacyErr, dualErr)
	}
	if legacy != dual {
		t.Fatalf("dual JSON changed legacy bytes:\nlegacy=%q\ndual=%q", legacy, dual)
	}
	if !strings.Contains(legacy, `\u0026`) || !strings.Contains(legacy, `\u003c`) || !strings.Contains(legacy, `\u003e`) {
		t.Fatalf("legacy fixture did not exercise HTML escaping: %q", legacy)
	}
	if legacyCaller.calls != 1 || dualCaller.calls != 1 {
		t.Fatalf("business calls legacy=%d dual=%d, want exactly one each", legacyCaller.calls, dualCaller.calls)
	}
}

func TestDualValidateNoTextResponseUsesLegacyFallback(t *testing.T) {
	result := &edition.ToolResult{Content: []edition.ContentBlock{{Type: "resource"}}}
	legacyCaller := &dualValidateCaller{format: "json", result: result}
	legacy, _ := runShortcutAtRollout(t, legacyCaller, output.RolloutLegacyOnly, "--format", "json")
	dualCaller := &dualValidateCaller{format: "json", result: result}
	dual, _ := runDualValidateShortcut(t, dualCaller, "--format", "json")
	if legacy != dual {
		t.Fatalf("dual no-text fallback changed legacy bytes:\nlegacy=%q\ndual=%q", legacy, dual)
	}
	if legacyCaller.calls != 1 || dualCaller.calls != 1 {
		t.Fatalf("business calls legacy=%d dual=%d, want exactly one each", legacyCaller.calls, dualCaller.calls)
	}
}

func TestDualValidateEmptyTextResponseUsesLegacyRawFallback(t *testing.T) {
	result := &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: ""}}}
	legacyCaller := &dualValidateCaller{format: "json", result: result}
	legacy, _ := runShortcutAtRollout(t, legacyCaller, output.RolloutLegacyOnly, "--format", "json")
	dualCaller := &dualValidateCaller{format: "json", result: result}
	dual, _ := runDualValidateShortcut(t, dualCaller, "--format", "json")
	if legacy != dual || legacy != "\n" {
		t.Fatalf("dual empty-text fallback changed legacy bytes: legacy=%q dual=%q", legacy, dual)
	}
	if legacyCaller.calls != 1 || dualCaller.calls != 1 {
		t.Fatalf("business calls legacy=%d dual=%d, want exactly one each", legacyCaller.calls, dualCaller.calls)
	}
}
