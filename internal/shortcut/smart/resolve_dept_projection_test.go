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

func resolveDeptFixture() map[string]any {
	return map[string]any{
		"success":    true,
		"hasMore":    false,
		"totalCount": float64(2),
		"deptList": []any{
			map[string]any{"deptId": float64(7), "deptName": "<red>Engineering</red>"},
			map[string]any{"deptId": float64(8), "deptName": "Platform"},
		},
	}
}

func TestResolveDeptProjectionRequiresCompleteStableCandidates(t *testing.T) {
	candidates, err := resolveDeptCandidates(resolveDeptFixture())
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || candidates[0]["deptId"] != "7" || candidates[0]["name"] != "Engineering" {
		t.Fatalf("candidates=%#v", candidates)
	}

	root := resolveDeptFixture()
	root["deptList"].([]any)[0].(map[string]any)["deptId"] = float64(-1)
	candidates, err = resolveDeptCandidates(root)
	if err != nil || candidates[0]["deptId"] != "1" {
		t.Fatalf("root sentinel projection=%#v err=%v", candidates, err)
	}
}

func TestResolveDeptProjectionFailsClosed(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"unknown container": func(root map[string]any) {
			delete(root, "deptList")
			root["items"] = []any{}
		},
		"non boolean success": func(root map[string]any) { root["success"] = "true" },
		"missing pagination":  func(root map[string]any) { delete(root, "hasMore") },
		"unfinished page":     func(root map[string]any) { root["hasMore"] = true },
		"total mismatch":      func(root map[string]any) { root["totalCount"] = float64(3) },
		"invalid row": func(root map[string]any) {
			root["deptList"].([]any)[0] = "not-an-object"
		},
		"unknown row field": func(root map[string]any) {
			root["deptList"].([]any)[0].(map[string]any)["parentId"] = float64(1)
		},
		"missing id": func(root map[string]any) {
			delete(root["deptList"].([]any)[0].(map[string]any), "deptId")
		},
		"fractional id": func(root map[string]any) {
			root["deptList"].([]any)[0].(map[string]any)["deptId"] = 7.5
		},
		"zero id": func(root map[string]any) {
			root["deptList"].([]any)[0].(map[string]any)["deptId"] = float64(0)
		},
		"missing name": func(root map[string]any) {
			delete(root["deptList"].([]any)[0].(map[string]any), "deptName")
		},
		"duplicate id": func(root map[string]any) {
			root["deptList"].([]any)[1].(map[string]any)["deptId"] = float64(7)
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := resolveDeptFixture()
			mutate(fixture)
			if candidates, err := resolveDeptCandidates(fixture); candidates != nil || err == nil {
				t.Fatalf("candidates=%#v err=%v", candidates, err)
			}
		})
	}
}

type resolveDeptProjectionCaller struct{ calls int }

func (c *resolveDeptProjectionCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.calls++
	if product != "contact" || tool != "search_dept_by_keyword" {
		return nil, stderrors.New("unexpected resolve-dept route")
	}
	raw, _ := json.Marshal(resolveDeptFixture())
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: string(raw)}}}, nil
}

func (c *resolveDeptProjectionCaller) Format() string { return "json" }
func (c *resolveDeptProjectionCaller) DryRun() bool   { return false }
func (c *resolveDeptProjectionCaller) Fields() string { return "" }
func (c *resolveDeptProjectionCaller) JQ() string     { return "" }

func TestResolveDeptUnifiedResultUsesCompleteCandidateProjection(t *testing.T) {
	if ResolveDept.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("rollout=%q", ResolveDept.OutputRollout)
	}
	if ResolveDept.Contract.Result == nil {
		t.Fatal("resolve-dept result contract is missing")
	}
	caller := &resolveDeptProjectionCaller{}
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(ResolveDept))
	cmd.PersistentFlags().String("format", "json", "")
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--name", "Engineering", "--format", "json"})
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
	if _, exists := envelope["contract_version"]; exists {
		t.Fatalf("removed version marker leaked: %#v", envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok || data["resolved"] != false || data["count"] != float64(2) {
		t.Fatalf("data=%#v", envelope["data"])
	}
	candidates, ok := data["candidates"].([]any)
	if !ok || len(candidates) != 2 || candidates[0].(map[string]any)["deptId"] != "7" {
		t.Fatalf("candidates=%#v", data["candidates"])
	}
	meta, ok := envelope["meta"].(map[string]any)
	if !ok || meta["count"] != float64(2) {
		t.Fatalf("meta=%#v", envelope["meta"])
	}
	pagination, ok := meta["pagination"].(map[string]any)
	if !ok || pagination["endpoint_exhausted"] != true || pagination["pages"] != float64(1) || pagination["items"] != float64(2) {
		t.Fatalf("pagination=%#v", meta["pagination"])
	}
}
