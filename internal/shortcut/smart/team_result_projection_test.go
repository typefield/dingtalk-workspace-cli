// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package smart

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type teamProjectionCaller struct {
	calls int
}

func (c *teamProjectionCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.calls++
	if product != "contact" || tool != "get_dept_members_by_deptId" {
		return nil, stderrors.New("unexpected team route")
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: `{"success":true,"deptUserList":[{"userInfo":{"userId":"u1","name":"Alice"}}]}`}}}, nil
}

func (c *teamProjectionCaller) Format() string { return "json" }
func (c *teamProjectionCaller) DryRun() bool   { return false }
func (c *teamProjectionCaller) Fields() string { return "" }
func (c *teamProjectionCaller) JQ() string     { return "" }

func TestTeamDualValidatePreservesLegacyTerminalBytes(t *testing.T) {
	if Team.OutputRollout != output.RolloutDualValidate {
		t.Fatalf("rollout=%q", Team.OutputRollout)
	}
	if Team.Contract.Result == nil || Team.Contract.Result.NDJSON == nil || Team.Contract.Result.NDJSON.RecordPath != "members" {
		t.Fatalf("result contract=%#v", Team.Contract.Result)
	}
	caller := &teamProjectionCaller{}
	helpers.InitDepsForTest(t, caller)
	declaration := Team
	declaration.Execute = func(rt *shortcut.RuntimeContext) error {
		return rt.CallMCP("get_dept_members_by_deptId", map[string]any{"deptIds": []string{"7"}})
	}
	cmd := corecmd.New(shortcut.FromShortcut(declaration))
	cmd.PersistentFlags().String("format", "json", "")
	cmd.SetContext(context.Background())
	var stdout, stderr bytes.Buffer
	helpers.GetFormatter().SetWriters(&stdout, &stderr)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--name", "fixture", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if caller.calls != 1 || stderr.Len() != 0 {
		t.Fatalf("calls=%d stderr=%q", caller.calls, stderr.String())
	}
	var legacy map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &legacy); err != nil || legacy["success"] != true {
		t.Fatalf("legacy=%#v err=%v output=%q", legacy, err, stdout.String())
	}
	if _, exists := legacy["ok"]; exists {
		t.Fatalf("dual stage leaked unified output: %#v", legacy)
	}
}
