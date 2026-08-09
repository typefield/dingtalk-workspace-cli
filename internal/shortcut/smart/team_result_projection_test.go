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

func TestTeamUnifiedResultUsesCanonicalMemberProjection(t *testing.T) {
	if Team.OutputRollout != output.RolloutUnifiedActive {
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
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout, stderr bytes.Buffer
	helpers.GetFormatter().SetWriters(&stdout, &stderr)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--name", "fixture", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	code, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted || code != 0 || caller.calls != 1 || stderr.Len() != 0 {
		t.Fatalf("code=%d emitted=%v calls=%d stderr=%q err=%v", code, emitted, caller.calls, stderr.String(), err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope["ok"] != true || envelope["outcome"] != "success" {
		t.Fatalf("envelope=%#v err=%v output=%q", envelope, err, stdout.String())
	}
	data, ok := envelope["data"].(map[string]any)
	members, membersOK := data["members"].([]any)
	meta, metaOK := envelope["meta"].(map[string]any)
	if !ok || !membersOK || !metaOK || data["count"] != float64(1) || meta["count"] != float64(1) || len(members) != 1 {
		t.Fatalf("envelope=%#v", envelope)
	}
	member, _ := members[0].(map[string]any)
	if member["userId"] != "u1" || member["name"] != "Alice" || len(member) != 2 {
		t.Fatalf("member=%#v", member)
	}
	if _, exists := envelope["contract_version"]; exists {
		t.Fatalf("removed version marker leaked: %#v", envelope)
	}
}
