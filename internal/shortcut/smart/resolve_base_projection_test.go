// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package smart

import (
	"bytes"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestResolveBaseUnifiedActiveEmitsReviewedResult(t *testing.T) {
	if ResolveBase.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("rollout = %q", ResolveBase.OutputRollout)
	}
	if ResolveBase.Contract.Result == nil || len(ResolveBase.Contract.Result.DataSchema) == 0 {
		t.Fatal("resolve-base result contract is missing")
	}
	caller := &aitableResolverCaller{text: reviewedBaseSearchJSON(t, []any{
		map[string]any{"baseId": "b1", "baseName": "项目"},
	}, false, "")}
	out, err := runAITableResolverCLI(t, caller, "aitable", "+resolve-base", "--name", "项目")
	if err != nil {
		t.Fatalf("resolve-base error = %v", err)
	}
	if caller.calls != 1 || caller.tool != "search_bases" {
		t.Fatalf("calls=%d tool=%q", caller.calls, caller.tool)
	}
	for _, want := range []string{`"ok": true`, `"outcome": "success"`, `"resolved": true`, `"matchType": "exact"`, `"count": 1`, `"candidates"`, `"baseId": "b1"`, `"baseName": "项目"`, `"sourceKind": "name_search_index"`, `"indexCoverageKnown": false`, `"endpoint_exhausted": true`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Fatalf("unified output missing %s: %s", want, out)
		}
	}
	for _, forbidden := range []string{`"contract_version"`, `"status"`, `"name":`} {
		if bytes.Contains([]byte(out), []byte(forbidden)) {
			t.Fatalf("active output leaked legacy/protocol field %s: %s", forbidden, out)
		}
	}
}
