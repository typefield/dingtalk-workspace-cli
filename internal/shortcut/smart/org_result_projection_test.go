// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package smart

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"reflect"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

func TestOrgDepartmentProjectionFailsClosed(t *testing.T) {
	projected, err := shortcutOrgProjectDepartment(map[string]any{
		"result": map[string]any{"deptId": float64(7), "deptName": "Engineering", "memberCount": float64(3), "ignored": "x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]any{"deptId": float64(7), "deptName": "Engineering", "memberCount": int64(3)}; !reflect.DeepEqual(projected, want) {
		t.Fatalf("projected=%#v, want %#v", projected, want)
	}
	for name, payload := range map[string]any{
		"not object":    []any{},
		"bad result":    map[string]any{"result": []any{}},
		"display only":  map[string]any{"result": map[string]any{"deptName": "Engineering"}},
		"fractional id": map[string]any{"result": map[string]any{"deptId": 7.5}},
	} {
		t.Run(name, func(t *testing.T) {
			if value, err := shortcutOrgProjectDepartment(payload); value != nil || err == nil {
				t.Fatalf("value=%#v err=%v", value, err)
			}
		})
	}
}

type orgProjectionCaller struct {
	calls int
}

func (c *orgProjectionCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.calls++
	if product != "contact" || tool != "get_dept_info_by_dept_id" {
		return nil, stderrors.New("unexpected org route")
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: `{"success":true,"result":{"deptId":7,"deptName":"Engineering","memberCount":3}}`}}}, nil
}

func (c *orgProjectionCaller) Format() string { return "json" }
func (c *orgProjectionCaller) DryRun() bool   { return false }
func (c *orgProjectionCaller) Fields() string { return "" }
func (c *orgProjectionCaller) JQ() string     { return "" }

func TestOrgDualValidatePreservesLegacyTerminalBytes(t *testing.T) {
	if Org.OutputRollout != output.RolloutDualValidate {
		t.Fatalf("rollout=%q", Org.OutputRollout)
	}
	if Org.Contract.Result == nil || !reflect.DeepEqual(Org.Contract.Result.Outcomes, []contract.ResultOutcome{
		contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure,
	}) {
		t.Fatalf("result contract=%#v", Org.Contract.Result)
	}
	caller := &orgProjectionCaller{}
	helpers.InitDepsForTest(t, caller)
	declaration := Org
	declaration.Execute = func(rt *shortcut.RuntimeContext) error {
		return rt.CallMCP("get_dept_info_by_dept_id", map[string]any{"deptId": 7})
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
