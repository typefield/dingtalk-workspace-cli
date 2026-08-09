// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package smart

import (
	"bytes"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestResolveTableUnifiedActiveEmitsReviewedProjection(t *testing.T) {
	if ResolveTable.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("rollout = %q", ResolveTable.OutputRollout)
	}
	if ResolveTable.Contract.Result == nil || len(ResolveTable.Contract.Result.DataSchema) == 0 {
		t.Fatal("resolve-table result contract is missing")
	}
	caller := &aitableResolverCaller{text: `{"success":true,"status":"success","summary":"ok","data":{"tables":[{"tableId":"t1","tableName":"任务","views":[]}]},"error":{},"meta":{}}`}
	out, err := runAITableResolverCLI(t, caller, "aitable", "+resolve-table", "--base", "base", "--name", "任务")
	if err != nil {
		t.Fatalf("resolve-table error = %v", err)
	}
	if caller.calls != 1 || caller.tool != "get_tables" {
		t.Fatalf("calls=%d tool=%q", caller.calls, caller.tool)
	}
	for _, want := range []string{`"ok": true`, `"outcome": "success"`, `"resolved": true`, `"matchType": "exact"`, `"count": 1`, `"candidates"`, `"tableId": "t1"`, `"tableName": "任务"`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("unified output missing %s: %s", want, out)
		}
	}
	if bytes.Contains([]byte(out), []byte(`"contract_version"`)) || bytes.Contains([]byte(out), []byte(`"status"`)) {
		t.Fatalf("active output leaked protocol or legacy-only fields: %s", out)
	}
}
