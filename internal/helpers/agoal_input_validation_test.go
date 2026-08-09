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
