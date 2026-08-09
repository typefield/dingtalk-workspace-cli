// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package smart

import (
	"bytes"
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestListTablesDualValidatePreservesReviewedLegacyProjection(t *testing.T) {
	if ListTables.OutputRollout != output.RolloutDualValidate {
		t.Fatalf("rollout = %q", ListTables.OutputRollout)
	}
	if ListTables.Contract.Result == nil || len(ListTables.Contract.Result.DataSchema) == 0 {
		t.Fatal("list-tables result contract is missing")
	}
	caller := &aitableResolverCaller{text: `{"success":true,"status":"success","summary":"ok","data":{"tables":[{"tableId":"t1","tableName":"任务","views":[]}]},"error":{},"meta":{}}`}
	out, err := runAITableResolverCLI(t, caller, "aitable", "+list-tables", "--base", "base")
	if err != nil {
		t.Fatalf("list-tables error = %v", err)
	}
	if caller.calls != 1 || caller.tool != "get_tables" {
		t.Fatalf("calls=%d tool=%q", caller.calls, caller.tool)
	}
	for _, want := range []string{`"tables"`, `"tableId": "t1"`, `"tableName": "任务"`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("legacy output missing %s: %s", want, out)
		}
	}
	if bytes.Contains([]byte(out), []byte(`"ok"`)) || bytes.Contains([]byte(out), []byte(`"outcome"`)) {
		t.Fatalf("dual validation changed legacy wire: %s", out)
	}
}

func TestListTablesDualValidatePreservesHistoricalEmptyBytes(t *testing.T) {
	response := `{"success":true,"status":"success","summary":"empty","data":{"tables":[]},"error":{},"meta":{}}`
	caller := &aitableResolverCaller{text: response}
	out, err := runAITableResolverCLI(t, caller, "aitable", "+list-tables", "--base", "base")
	if err != nil {
		t.Fatalf("list-tables empty error = %v", err)
	}
	for _, want := range []string{`"success": true`, `"summary": "empty"`, `"tables": []`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("empty legacy output missing %s: %s", want, out)
		}
	}
}

func TestListTablesUnknownShapeFailsClosed(t *testing.T) {
	caller := &aitableResolverCaller{text: `{"success":true,"data":{"items":[]}}`}
	out, err := runAITableResolverCLI(t, caller, "aitable", "+list-tables", "--base", "base")
	if err == nil || out != "" {
		t.Fatalf("unknown response = output:%q err:%v", out, err)
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != string(apperrors.SubtypeProjectionUnknown) {
		t.Fatalf("unknown response error = %#v", err)
	}
}
