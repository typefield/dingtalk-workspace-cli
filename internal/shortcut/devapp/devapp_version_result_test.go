// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package devapp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type devAppVersionCaller struct {
	payload map[string]any
	calls   int
	tool    string
	params  map[string]any
}

func (c *devAppVersionCaller) CallTool(_ context.Context, _, tool string, params map[string]any) (*edition.ToolResult, error) {
	c.calls++
	c.tool = tool
	c.params = make(map[string]any, len(params))
	for key, value := range params {
		c.params[key] = value
	}
	encoded, err := json.Marshal(map[string]any{"success": true, "result": c.payload})
	if err != nil {
		return nil, err
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: string(encoded)}}}, nil
}

func (*devAppVersionCaller) Format() string { return "json" }
func (*devAppVersionCaller) DryRun() bool   { return false }
func (*devAppVersionCaller) Fields() string { return "" }
func (*devAppVersionCaller) JQ() string     { return "" }

func executeDevAppVersionShortcut(t *testing.T, declaration shortcut.Shortcut, caller *devAppVersionCaller, args ...string) (map[string]any, int) {
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
	if err != nil || !emitted {
		t.Fatalf("emit: code=%d emitted=%v err=%v", exitCode, emitted, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	return envelope, exitCode
}

func TestDevAppVersionShortcutsShareTruthfulOutcomeMapper(t *testing.T) {
	t.Run("create returns stable identity but not readback proof", func(t *testing.T) {
		caller := &devAppVersionCaller{payload: map[string]any{
			"unifiedAppId": "app-1", "versionId": "version-1", "versionStatus": "INIT",
		}}
		envelope, exitCode := executeDevAppVersionShortcut(t, VersionCreate, caller,
			"--unified-app-id", "app-1", "--desc", "release", "--yes")
		data, _ := envelope["data"].(map[string]any)
		verification, _ := data["verification"].(map[string]any)
		if caller.calls != 1 || caller.tool != "create_dev_app_version" || exitCode != 0 ||
			envelope["outcome"] != "success" || verification["state"] != "not_verified" {
			t.Fatalf("calls=%d tool=%s rc=%d envelope=%#v", caller.calls, caller.tool, exitCode, envelope)
		}
	})

	t.Run("submitted approval is pending not success", func(t *testing.T) {
		caller := &devAppVersionCaller{payload: map[string]any{
			"approvalSubmitted": true, "published": false,
		}}
		envelope, exitCode := executeDevAppVersionShortcut(t, VersionPublish, caller,
			"--unified-app-id", "app-1", "--version-id", "version-1", "--yes")
		meta, _ := envelope["meta"].(map[string]any)
		operation, _ := meta["operation"].(map[string]any)
		if caller.calls != 1 || caller.tool != "publish_dev_app_version" || exitCode != 0 ||
			envelope["outcome"] != "pending" || operation["id"] != "version-1" || operation["next_command"] == "" {
			t.Fatalf("calls=%d tool=%s rc=%d envelope=%#v", caller.calls, caller.tool, exitCode, envelope)
		}
	})

	t.Run("ambiguous publish fails closed", func(t *testing.T) {
		caller := &devAppVersionCaller{payload: map[string]any{"accepted": true}}
		envelope, exitCode := executeDevAppVersionShortcut(t, VersionPublish, caller,
			"--unified-app-id", "app-1", "--version-id", "version-1", "--yes")
		errorData, _ := envelope["error"].(map[string]any)
		if caller.calls != 1 || exitCode == 0 || envelope["outcome"] != "failure" ||
			errorData["subtype"] != "projection_unknown" || errorData["retryable"] == true {
			t.Fatalf("calls=%d rc=%d envelope=%#v", caller.calls, exitCode, envelope)
		}
	})
}

func TestDevAppVersionShortcutContractsRemainDualUntilRealWriteEvidence(t *testing.T) {
	registered := make(map[string]shortcut.Shortcut)
	for _, item := range shortcut.All() {
		if item.Service == "devapp" {
			registered[item.Command] = item
		}
	}
	for _, name := range []string{"+version-create", "+version-publish"} {
		item := registered[name]
		if item.OutputRollout != output.RolloutDualValidate || item.Contract.Result == nil ||
			item.Safety.Effect != "write" || item.Safety.Risk != "high" ||
			item.Safety.Confirmation != "user_required" || item.Safety.Idempotency != "unknown" || !item.ConfirmFirst {
			t.Fatalf("%s rollout/safety/result/confirm=%q/%#v/%#v/%v", name, item.OutputRollout, item.Safety, item.Contract.Result, item.ConfirmFirst)
		}
	}
	for _, name := range []string{"+version-check-approval", "+version-status"} {
		item := registered[name]
		if item.OutputRollout != output.RolloutUnifiedActive || item.Contract.Result == nil {
			t.Fatalf("%s rollout/result=%q/%#v", name, item.OutputRollout, item.Contract.Result)
		}
	}
}
