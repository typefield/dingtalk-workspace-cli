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
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

func TestWhoamiProjectionRequiresStableCurrentUser(t *testing.T) {
	profile, err := whoamiProject(map[string]any{
		"result": []any{map[string]any{
			"orgEmployeeModel": map[string]any{
				"userId":        "user-1",
				"orgUserName":   "Example",
				"orgUserMobile": "13800000000",
				"depts": []any{
					map[string]any{"deptName": "Engineering"},
				},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile["userId"] != "user-1" || profile["name"] != "Example" ||
		profile["mobile"] != "13800000000" || profile["dept"] != "Engineering" {
		t.Fatalf("profile = %#v", profile)
	}

	for name, payload := range map[string]map[string]any{
		"empty response":       {},
		"known empty result":   {"result": []any{}},
		"display only profile": {"result": map[string]any{"name": "Display only"}},
		"multiple profiles": {"result": []any{
			map[string]any{"userId": "user-1"},
			map[string]any{"userId": "user-2"},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			projected, err := whoamiProject(payload)
			var typed *apperrors.Error
			if projected != nil || !stderrors.As(err, &typed) ||
				typed.Category != apperrors.CategoryAPI ||
				typed.StableSubtype != string(apperrors.SubtypeProjectionUnknown) ||
				typed.FailureStage != "response_projection" || typed.Retryable {
				t.Fatalf("projected=%#v error=%#v", projected, typed)
			}
		})
	}
}

type whoamiProjectionCaller struct {
	result string
	calls  int
}

func (c *whoamiProjectionCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.calls++
	if product != "contact" || tool != "get_current_user_profile" {
		return nil, stderrors.New("unexpected whoami route")
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: c.result}}}, nil
}

func (c *whoamiProjectionCaller) Format() string { return "json" }
func (c *whoamiProjectionCaller) DryRun() bool   { return false }
func (c *whoamiProjectionCaller) Fields() string { return "" }
func (c *whoamiProjectionCaller) JQ() string     { return "" }

func TestWhoamiDualValidatePreservesLegacyResultAndDeclaration(t *testing.T) {
	if Whoami.OutputRollout != output.RolloutDualValidate {
		t.Fatalf("rollout = %q", Whoami.OutputRollout)
	}
	result := Whoami.Contract.Result
	if result == nil || result.NDJSON != nil || result.Pagination != nil {
		t.Fatalf("result contract = %#v", result)
	}
	if got, want := result.Outcomes, []contract.ResultOutcome{
		contract.ResultOutcomeSuccess,
		contract.ResultOutcomeFailure,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("outcomes = %#v, want %#v", got, want)
	}
	if got, want := result.SensitivePaths, []string{"email", "mobile"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("sensitive paths = %#v, want %#v", got, want)
	}
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(result.DataSchema, &schema); err != nil || !reflect.DeepEqual(schema.Required, []string{"userId"}) {
		t.Fatalf("data schema required=%v err=%v", schema.Required, err)
	}

	caller := &whoamiProjectionCaller{result: `{"result":[{"orgEmployeeModel":{"userId":"user-1","orgUserName":"Example"}}]}`}
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(Whoami))
	cmd.PersistentFlags().String("format", "json", "")
	cmd.SetContext(context.Background())
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if caller.calls != 1 || stderr.Len() != 0 {
		t.Fatalf("calls=%d stderr=%q", caller.calls, stderr.String())
	}
	const want = "{\n  \"name\": \"Example\",\n  \"userId\": \"user-1\"\n}\n"
	if stdout.String() != want {
		t.Fatalf("legacy bytes changed:\n got: %q\nwant: %q", stdout.String(), want)
	}
	var legacy map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &legacy); err != nil || legacy["userId"] != "user-1" || legacy["name"] != "Example" {
		t.Fatalf("legacy result=%#v err=%v", legacy, err)
	}
	if _, exists := legacy["ok"]; exists {
		t.Fatalf("dual validation leaked unified envelope: %#v", legacy)
	}
}
