// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package smart

import (
	"bytes"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestResolveTableDualValidatePreservesLegacyProjection(t *testing.T) {
	if ResolveTable.OutputRollout != output.RolloutDualValidate {
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
	for _, want := range []string{`"resolved": true`, `"status": "resolved"`, `"matchType": "exact"`, `"tableId": "t1"`, `"name": "任务"`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("legacy output missing %s: %s", want, out)
		}
	}
	if bytes.Contains([]byte(out), []byte(`"ok"`)) || bytes.Contains([]byte(out), []byte(`"outcome"`)) {
		t.Fatalf("dual validation changed legacy wire: %s", out)
	}
}
