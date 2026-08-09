// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"errors"
	"reflect"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestAgoalListScopeTypeNormalizesBeforeExactlyOnceCall(t *testing.T) {
	tests := []struct {
		name string
		path []string
		tool string
	}{
		{name: "strategy", path: []string{"strategy", "list"}, tool: "list_strategy_decodings"},
		{name: "contract", path: []string{"contract", "list"}, tool: "list_op_contracts"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: `{"success":true,"content":[]}`}}}
			installScriptedCaller(t, caller)
			args := append(append([]string{}, test.path...),
				"--scope-type", " personal ", "--scope-id", "user-1", "--request-id", "request-1")
			if err := executeFilterCoverage(t, newAgoalCommand(), args...); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if caller.calls != 1 || caller.tool != test.tool {
				t.Fatalf("calls/tool = %d/%q, want 1/%q", caller.calls, caller.tool, test.tool)
			}
			want := map[string]any{
				"scopeType": "PERSONAL",
				"openId":    "user-1",
				"requestId": "request-1",
			}
			if !reflect.DeepEqual(caller.args, want) {
				t.Fatalf("args = %#v, want %#v", caller.args, want)
			}
		})
	}
}

func TestAgoalListScopeTypeFailsBeforeRemoteCall(t *testing.T) {
	for _, path := range [][]string{{"strategy", "list"}, {"contract", "list"}} {
		caller := &scriptedToolCaller{format: "json"}
		installScriptedCaller(t, caller)
		args := append(append([]string{}, path...), "--scope-type", "organization", "--scope-id", "1")
		err := executeFilterCoverage(t, newAgoalCommand(), args...)
		if err == nil {
			t.Fatalf("%v unexpectedly accepted an invalid scope type", path)
		}
		var typed *apperrors.Error
		if !errors.As(err, &typed) {
			t.Fatalf("%v error = %T, want typed validation", path, err)
		}
		if typed.Category != apperrors.CategoryValidation || typed.StableSubtype != string(apperrors.SubtypeInvalidFlagValue) || typed.ExitCode() != 3 {
			t.Fatalf("%v typed error = %#v", path, typed)
		}
		if caller.calls != 0 {
			t.Fatalf("%v issued %d remote calls after local validation failure", path, caller.calls)
		}
	}
}

func TestAgoalUserObjectivesValidatesPeriodIDsBeforeRemoteCall(t *testing.T) {
	caller := &scriptedToolCaller{format: "json"}
	installScriptedCaller(t, caller)
	err := executeFilterCoverage(t, newAgoalCommand(),
		"user", "objectives",
		"--user-id", "user-1",
		"--rule-id", "rule-1",
		"--period-ids", ", ,",
	)
	assertAgoalInvalidFlagError(t, err)
	if caller.calls != 0 {
		t.Fatalf("issued %d remote calls for an empty period ID list", caller.calls)
	}
}

func TestAgoalUserObjectivesProjectsTrimmedPeriodIDsExactlyOnce(t *testing.T) {
	caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: `{"success":true,"content":[]}`}}}
	installScriptedCaller(t, caller)
	if err := executeFilterCoverage(t, newAgoalCommand(),
		"user", "objectives",
		"--user-id", "user-1",
		"--rule-id", "rule-1",
		"--period-ids", " period-1, period-2 ",
		"--request-id", "request-1",
	); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := map[string]any{
		"dingUserId":      "user-1",
		"objectiveRuleId": "rule-1",
		"periodIds":       []string{"period-1", "period-2"},
		"requestId":       "request-1",
	}
	if caller.calls != 1 || caller.tool != "list_user_objectives" || !reflect.DeepEqual(caller.args, want) {
		t.Fatalf("calls/tool/args = %d/%q/%#v, want 1/list_user_objectives/%#v", caller.calls, caller.tool, caller.args, want)
	}
}

func TestAgoalSubmitDetailValidatesEnumsDatesAndPagesBeforeRemoteCall(t *testing.T) {
	tests := [][]string{
		{"--submit-state", "submitted"},
		{"--submit-state", "ON_TIME", "--query-date", "not-a-date"},
		{"--submit-state", "ON_TIME", "--page", "0"},
		{"--submit-state", "ON_TIME", "--page", "-1"},
		{"--submit-state", "ON_TIME", "--page-size", "0"},
		{"--submit-state", "ON_TIME", "--page-size", "-1"},
	}
	for _, extra := range tests {
		caller := &scriptedToolCaller{format: "json"}
		installScriptedCaller(t, caller)
		args := []string{"report", "submit-detail", "--template-id", "template-1"}
		args = append(args, extra...)
		err := executeFilterCoverage(t, newAgoalCommand(), args...)
		assertAgoalInvalidFlagError(t, err)
		if caller.calls != 0 {
			t.Fatalf("args %v issued %d remote calls", extra, caller.calls)
		}
	}
}

func TestAgoalSubmitDetailNormalizesStateAndMapsPagingExactlyOnce(t *testing.T) {
	caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: `{"success":true,"content":{"page":1,"pageSize":20,"result":[],"totalCount":0}}`}}}
	installScriptedCaller(t, caller)
	if err := executeFilterCoverage(t, newAgoalCommand(),
		"report", "submit-detail",
		"--template-id", "template-1",
		"--submit-state", " late ",
		"--query-date", "2026-06-18T00:00:00+08:00",
		"--page", "1",
		"--page-size", "20",
		"--keyword", "reviewer",
		"--request-id", "request-1",
	); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := map[string]any{
		"templateId":  "template-1",
		"submitState": "LATE",
		"queryDate":   "2026-06-18",
		"page":        1,
		"pageSize":    20,
		"keyword":     "reviewer",
		"requestId":   "request-1",
	}
	if caller.calls != 1 || caller.tool != "get_submit_detail" || !reflect.DeepEqual(caller.args, want) {
		t.Fatalf("calls/tool/args = %d/%q/%#v, want 1/get_submit_detail/%#v", caller.calls, caller.tool, caller.args, want)
	}
}

func assertAgoalInvalidFlagError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("invalid input unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) {
		t.Fatalf("error = %T, want typed validation", err)
	}
	if typed.Category != apperrors.CategoryValidation || typed.StableSubtype != string(apperrors.SubtypeInvalidFlagValue) || typed.ExitCode() != 3 {
		t.Fatalf("typed error = %#v", typed)
	}
}
