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

func lookupFixture() map[string]any {
	return map[string]any{"success": true, "result": []any{map[string]any{
		"isAdmin": false,
		"orgEmployeeModel": map[string]any{
			"orgUserId": "u1", "orgUserName": "Alice", "orgAuthEmail": "a@example.test",
			"jobNumber": "J1", "orgId": float64(9), "orgName": "Example Org", "orgTitle": "Engineer",
			"orgMasterUserId": "m1", "orgMasterDisplayName": "Manager",
			"depts":     []any{map[string]any{"deptId": float64(7), "deptName": "Engineering", "deptPathName": "Example/Engineering"}},
			"positions": []any{map[string]any{"deptId": float64(7), "isMain": true, "managerDisplayName": "Manager", "managerStaffId": "m1", "title": "Engineer", "workStation": "Hangzhou"}},
			"labels":    []any{map[string]any{"labelId": float64(3), "labelName": "Reviewer"}},
		},
	}}}
}

func TestLookupFullProfileProjectionAndFailClosedBoundary(t *testing.T) {
	profile, err := lookupProjectProfile(lookupFixture())
	if err != nil {
		t.Fatal(err)
	}
	if profile["userId"] != "u1" || profile["name"] != "Alice" || profile["title"] != "Engineer" || profile["isAdmin"] != false {
		t.Fatalf("profile=%#v", profile)
	}
	if len(profile["departments"].([]map[string]any)) != 1 || len(profile["positions"].([]map[string]any)) != 1 || len(profile["labels"].([]map[string]any)) != 1 {
		t.Fatalf("nested profile=%#v", profile)
	}
	for name, mutate := range map[string]func(map[string]any){
		"multiple users": func(root map[string]any) { root["result"] = append(root["result"].([]any), map[string]any{}) },
		"unknown model field": func(root map[string]any) {
			root["result"].([]any)[0].(map[string]any)["orgEmployeeModel"].(map[string]any)["newSecret"] = "x"
		},
		"missing departments": func(root map[string]any) {
			delete(root["result"].([]any)[0].(map[string]any)["orgEmployeeModel"].(map[string]any), "depts")
		},
		"bad position": func(root map[string]any) {
			root["result"].([]any)[0].(map[string]any)["orgEmployeeModel"].(map[string]any)["positions"] = []any{map[string]any{"title": "Engineer"}}
		},
		"user alias conflict": func(root map[string]any) {
			root["result"].([]any)[0].(map[string]any)["orgEmployeeModel"].(map[string]any)["userId"] = "u2"
		},
		"string success": func(root map[string]any) { root["success"] = "true" },
		"blank department id": func(root map[string]any) {
			root["result"].([]any)[0].(map[string]any)["orgEmployeeModel"].(map[string]any)["depts"].([]any)[0].(map[string]any)["deptId"] = "  "
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := lookupFixture()
			mutate(fixture)
			if projected, err := lookupProjectProfile(fixture); projected != nil || err == nil {
				t.Fatalf("projected=%#v err=%v", projected, err)
			}
		})
	}
}

func TestLookupProjectionTreatsNullOptionalOrganizationIDAsAbsent(t *testing.T) {
	fixture := lookupFixture()
	model := fixture["result"].([]any)[0].(map[string]any)["orgEmployeeModel"].(map[string]any)
	model["orgId"] = nil
	profile, err := lookupProjectProfile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	organization, ok := profile["organization"].(map[string]any)
	if !ok || organization["name"] != "Example Org" {
		t.Fatalf("organization=%#v", profile["organization"])
	}
	if _, exists := organization["id"]; exists {
		t.Fatalf("null optional orgId must be omitted: %#v", organization)
	}
}

type lookupProjectionCaller struct{ calls int }

func (c *lookupProjectionCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.calls++
	if product != "contact" || tool != "get_user_info_by_user_ids" {
		return nil, stderrors.New("unexpected lookup route")
	}
	raw, _ := json.Marshal(lookupFixture())
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: string(raw)}}}, nil
}

func (c *lookupProjectionCaller) Format() string { return "json" }
func (c *lookupProjectionCaller) DryRun() bool   { return false }
func (c *lookupProjectionCaller) Fields() string { return "" }
func (c *lookupProjectionCaller) JQ() string     { return "" }

func TestLookupUnifiedResultPreservesReviewedFullProfile(t *testing.T) {
	if Lookup.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("rollout=%q", Lookup.OutputRollout)
	}
	if Lookup.Contract.Result == nil || len(Lookup.Contract.Result.SensitivePaths) != 5 {
		t.Fatalf("result contract=%#v", Lookup.Contract.Result)
	}
	caller := &lookupProjectionCaller{}
	helpers.InitDepsForTest(t, caller)
	declaration := Lookup
	declaration.Execute = func(rt *shortcut.RuntimeContext) error {
		return rt.CallMCP("get_user_info_by_user_ids", map[string]any{"user_id_list": []string{"u1"}})
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
		t.Fatalf("envelope=%#v err=%v", envelope, err)
	}
	if _, exists := envelope["contract_version"]; exists {
		t.Fatalf("removed version marker leaked: %#v", envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok || data["userId"] != "u1" || data["name"] != "Alice" || data["jobNumber"] != "J1" || len(data) != 10 {
		t.Fatalf("data=%#v", envelope["data"])
	}
	if _, leaked := data["orgEmployeeModel"]; leaked {
		t.Fatalf("raw wrapper leaked: %#v", data)
	}
	for _, key := range []string{"departments", "positions", "labels"} {
		rows, ok := data[key].([]any)
		if !ok || len(rows) != 1 {
			t.Fatalf("%s=%#v", key, data[key])
		}
	}
}
