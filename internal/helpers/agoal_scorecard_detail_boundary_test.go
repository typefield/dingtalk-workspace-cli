// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestAgoalScorecardDetailRejectsJSONNullInsteadOfRCZeroSuccess(t *testing.T) {
	caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: "null"}}}
	installScriptedCaller(t, caller)
	var stdout bytes.Buffer
	deps.Out.w = &stdout
	err := executeFilterCoverage(t, newAgoalCommand(),
		"scorecard", "detail",
		"--selected-time", "2026-08-01T00:00:00+08:00",
		"--dept-id", "1",
	)
	if err == nil {
		t.Fatal("JSON null unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("error = %T %#v, want non-retryable projection_unknown", err, err)
	}
	if caller.calls != 1 || caller.tool != agoalScorecardDetailTool {
		t.Fatalf("calls/tool = %d/%q", caller.calls, caller.tool)
	}
	if stdout.Len() != 0 {
		t.Fatalf("null response leaked success stdout: %q", stdout.String())
	}
}

func TestAgoalScorecardDetailPreservesNonNullLegacyPayloadExactlyOnce(t *testing.T) {
	const raw = `{"code":null,"content":{"content":[],"id":"scorecard-1"},"message":null,"requestId":"request-1","success":true}`
	wantArgs := map[string]any{
		"selectedTime": int64(1785513600000),
		"deptId":       "1",
		"requestId":    "request-2",
	}
	legacyCaller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: raw}}}
	installScriptedCaller(t, legacyCaller)
	var legacyStdout bytes.Buffer
	deps.Out.w = &legacyStdout
	if err := callMCPTool(agoalScorecardDetailTool, wantArgs); err != nil {
		t.Fatalf("legacy baseline: %v", err)
	}

	caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: raw}}}
	installScriptedCaller(t, caller)
	var stdout bytes.Buffer
	deps.Out.w = &stdout
	if err := executeFilterCoverage(t, newAgoalCommand(),
		"scorecard", "detail",
		"--selected-time", "2026-08-01T00:00:00+08:00",
		"--dept-id", "1",
		"--request-id", "request-2",
	); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if caller.calls != 1 || caller.tool != agoalScorecardDetailTool {
		t.Fatalf("calls/tool = %d/%q", caller.calls, caller.tool)
	}
	if !reflect.DeepEqual(caller.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", caller.args, wantArgs)
	}
	if legacyCaller.calls != 1 || stdout.String() != legacyStdout.String() {
		t.Fatalf("legacy stdout changed:\ngot = %q\nwant= %q", stdout.String(), legacyStdout.String())
	}
}
