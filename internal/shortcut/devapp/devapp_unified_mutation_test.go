// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package devapp

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type devAppMutationCaller struct {
	calls   int
	product string
	tool    string
	params  map[string]any
}

func (c *devAppMutationCaller) CallTool(_ context.Context, product, tool string, params map[string]any) (*edition.ToolResult, error) {
	c.calls++
	c.product, c.tool = product, tool
	c.params = make(map[string]any, len(params))
	for key, value := range params {
		c.params[key] = value
	}
	result := map[string]any{"unifiedAppId": "app-1", "status": "accepted"}
	encoded, err := json.Marshal(map[string]any{"success": true, "result": result})
	if err != nil {
		return nil, err
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: string(encoded)}}}, nil
}

func (*devAppMutationCaller) Format() string { return "json" }
func (*devAppMutationCaller) DryRun() bool   { return false }
func (*devAppMutationCaller) Fields() string { return "" }
func (*devAppMutationCaller) JQ() string     { return "" }

func executeDevAppMutation(t *testing.T, declaration shortcut.Shortcut, caller *devAppMutationCaller, args ...string) map[string]any {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(frameworkUnified(declaration)))
	cmd.PersistentFlags().String("format", "json", "")
	cmd.PersistentFlags().Bool("yes", false, "")
	cmd.PersistentFlags().Bool("dry-run", false, "")
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append([]string{"--format", "json"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	exitCode, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted || exitCode != 0 {
		t.Fatalf("emit: code=%d emitted=%v err=%v", exitCode, emitted, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	return envelope
}

func TestDevAppCoreMutationShortcutsEmitOneHonestUnifiedResult(t *testing.T) {
	tests := []struct {
		name        string
		declaration shortcut.Shortcut
		tool        string
		args        []string
		params      map[string]any
	}{
		{name: "create", declaration: CreateApp, tool: "create_dev_app", args: []string{"--name", "Demo", "--yes"}, params: map[string]any{"name": "Demo"}},
		{name: "update", declaration: UpdateApp, tool: "update_dev_app", args: []string{"--unified-app-id", "app-1", "--name", "Renamed", "--yes"}, params: map[string]any{"unifiedAppId": "app-1", "name": "Renamed"}},
		{name: "enable", declaration: EnableApp, tool: "enable_dev_app", args: []string{"--unified-app-id", "app-1", "--yes"}, params: map[string]any{"unifiedAppId": "app-1"}},
		{name: "disable", declaration: DisableApp, tool: "disable_dev_app", args: []string{"--unified-app-id", "app-1", "--yes"}, params: map[string]any{"unifiedAppId": "app-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &devAppMutationCaller{}
			envelope := executeDevAppMutation(t, tt.declaration, caller, tt.args...)
			if caller.calls != 1 || caller.product != productDevApp || caller.tool != tt.tool || !reflect.DeepEqual(caller.params, tt.params) {
				t.Fatalf("calls=%d route=%s/%s params=%#v", caller.calls, caller.product, caller.tool, caller.params)
			}
			if envelope["ok"] != true || envelope["outcome"] != "success" {
				t.Fatalf("envelope=%#v", envelope)
			}
			if _, found := envelope["contract_version"]; found {
				t.Fatalf("removed version marker leaked: %#v", envelope)
			}
			data, ok := envelope["data"].(map[string]any)
			if !ok {
				t.Fatalf("data=%#v", envelope["data"])
			}
			verification, ok := data["verification"].(map[string]any)
			if !ok || verification["state"] != "not_verified" || verification["reason"] == "" {
				t.Fatalf("verification=%#v", data["verification"])
			}
			if tt.tool == "enable_dev_app" && data["enabled"] != true {
				t.Fatalf("enable projection=%#v", data)
			}
			if tt.tool == "disable_dev_app" && data["disabled"] != true {
				t.Fatalf("disable projection=%#v", data)
			}
		})
	}
}

func TestDevAppCoreMutationShortcutContractsAreActive(t *testing.T) {
	registered := make(map[string]shortcut.Shortcut)
	for _, item := range shortcut.All() {
		if item.Service == "devapp" {
			registered[item.Command] = item
		}
	}
	for _, name := range []string{"+create", "+update", "+enable", "+disable", "+delete"} {
		item := registered[name]
		if item.OutputRollout != output.RolloutUnifiedActive || item.Contract.Result == nil {
			t.Fatalf("%s rollout/result = %q/%#v", name, item.OutputRollout, item.Contract.Result)
		}
		wantOutcomes := []contract.ResultOutcome{
			contract.ResultOutcomeSuccess,
			contract.ResultOutcomePending,
			contract.ResultOutcomePartialFailure,
			contract.ResultOutcomeFailure,
		}
		if !reflect.DeepEqual(item.Contract.Result.Outcomes, wantOutcomes) {
			t.Fatalf("%s outcomes=%#v", name, item.Contract.Result.Outcomes)
		}
		if item.Safety.Risk != "high" || item.Safety.Confirmation != "user_required" {
			t.Fatalf("%s safety=%#v", name, item.Safety)
		}
	}
	if item := registered["+delete"]; !item.ConfirmFirst {
		t.Fatal("delete must preserve guard-first confirmation ordering")
	}
}

type devAppDeleteCaller struct {
	actualName string
	tools      []string
}

func (c *devAppDeleteCaller) CallTool(_ context.Context, _, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.tools = append(c.tools, tool)
	result := map[string]any{"unifiedAppId": "app-1", "deleted": true}
	if tool == "get_dev_app" {
		result = map[string]any{}
		if c.actualName != "" {
			result["name"] = c.actualName
		}
	}
	encoded, err := json.Marshal(map[string]any{"success": true, "result": result})
	if err != nil {
		return nil, err
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: string(encoded)}}}, nil
}

func (*devAppDeleteCaller) Format() string { return "json" }
func (*devAppDeleteCaller) DryRun() bool   { return false }
func (*devAppDeleteCaller) Fields() string { return "" }
func (*devAppDeleteCaller) JQ() string     { return "" }

func executeDevAppDelete(t *testing.T, caller *devAppDeleteCaller, args ...string) (map[string]any, error) {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(frameworkUnified(DeleteApp)))
	cmd.PersistentFlags().String("format", "json", "")
	cmd.PersistentFlags().Bool("yes", false, "")
	cmd.PersistentFlags().Bool("dry-run", false, "")
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(append([]string{"--format", "json"}, args...))
	if err := cmd.Execute(); err != nil {
		return nil, err
	}
	_, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted {
		t.Fatalf("emit: emitted=%v err=%v", emitted, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	return envelope, nil
}

func TestDevAppDeleteShortcutRequiresMatchingNameBeforeOneDelete(t *testing.T) {
	caller := &devAppDeleteCaller{actualName: "DemoApp"}
	envelope, err := executeDevAppDelete(t, caller,
		"--unified-app-id", "app-1", "--confirm-name", "DemoApp", "--yes")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !reflect.DeepEqual(caller.tools, []string{"get_dev_app", "delete_dev_app"}) {
		t.Fatalf("tools=%#v", caller.tools)
	}
	data, _ := envelope["data"].(map[string]any)
	verification, _ := data["verification"].(map[string]any)
	if envelope["ok"] != true || envelope["outcome"] != "success" || verification["state"] != "not_verified" {
		t.Fatalf("envelope=%#v", envelope)
	}
}

func TestDevAppDeleteShortcutFailsClosedBeforeDelete(t *testing.T) {
	tests := []struct {
		name       string
		actualName string
		args       []string
		wantCalls  []string
		wantText   string
	}{
		{name: "missing yes is guard first", args: []string{"--unified-app-id", "app-1"}, wantText: "确认"},
		{name: "missing confirm name", args: []string{"--unified-app-id", "app-1", "--yes"}, wantText: "--confirm-name"},
		{name: "name mismatch", actualName: "RealApp", args: []string{"--unified-app-id", "app-1", "--confirm-name", "Wrong", "--yes"}, wantCalls: []string{"get_dev_app"}, wantText: "名称不匹配"},
		{name: "name unavailable", args: []string{"--unified-app-id", "app-1", "--confirm-name", "DemoApp", "--yes"}, wantCalls: []string{"get_dev_app"}, wantText: "无法读取应用名"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &devAppDeleteCaller{actualName: tt.actualName}
			_, err := executeDevAppDelete(t, caller, tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("error=%v, want %q", err, tt.wantText)
			}
			if !reflect.DeepEqual(caller.tools, tt.wantCalls) {
				t.Fatalf("tools=%#v, want %#v", caller.tools, tt.wantCalls)
			}
		})
	}
}

func TestDevAppDeleteShortcutDryRunNeedsNoNameAndMakesNoCall(t *testing.T) {
	caller := &devAppDeleteCaller{}
	envelope, err := executeDevAppDelete(t, caller, "--unified-app-id", "app-1", "--dry-run")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(caller.tools) != 0 || envelope["ok"] != true || envelope["dry_run"] != true {
		t.Fatalf("tools=%#v envelope=%#v", caller.tools, envelope)
	}
}

func TestDevAppUpdateRejectsEmptyMutationBeforeBusinessCall(t *testing.T) {
	caller := &devAppMutationCaller{}
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(frameworkUnified(UpdateApp)))
	cmd.PersistentFlags().String("format", "json", "")
	cmd.PersistentFlags().Bool("yes", false, "")
	cmd.PersistentFlags().Bool("dry-run", false, "")
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--format", "json", "--unified-app-id", "app-1", "--yes"})
	err := cmd.Execute()
	var typed *apperrors.Error
	if !stderrors.As(err, &typed) || typed.Category != apperrors.CategoryValidation {
		t.Fatalf("error=%#v", err)
	}
	if caller.calls != 0 {
		t.Fatalf("business calls=%d, want 0", caller.calls)
	}
}

func TestDevAppMutationMapperRejectsNonObjectSuccessAsUnknownEffect(t *testing.T) {
	result := helpers.DevAppCommandResultFromPayload("create_dev_app", "opaque-success", false)
	if result.Outcome() != output.OutcomeFailure {
		t.Fatalf("outcome=%q", result.Outcome())
	}
	envelope, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatalf("EnvelopeFromResult: %v", err)
	}
	if envelope.Error == nil || envelope.Error.Subtype != string(apperrors.SubtypeProjectionUnknown) ||
		envelope.Error.ExecutionStarted == nil || !*envelope.Error.ExecutionStarted || envelope.Error.Retryable {
		t.Fatalf("error=%#v", envelope.Error)
	}
}
