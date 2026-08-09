// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package smart

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	shortcutcore "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestMyGroupsRolloutIsDualValidate(t *testing.T) {
	if MyGroups.OutputRollout != output.RolloutDualValidate {
		t.Fatalf("my-groups rollout=%q, want dual_validate", MyGroups.OutputRollout)
	}
}

func TestMyGroupsDualValidatePreservesLegacyPayload(t *testing.T) {
	t.Run("single page", func(t *testing.T) {
		response := `{"result":{"groups":[{"openConversationId":"g1","title":"群一"}],"hasMore":false}}`
		legacyBytes := runMyGroupsLegacyBytes(t, &chatMessagesPagingCaller{responses: []string{response}}, output.RolloutLegacyOnly)
		dual := runMyGroupsLegacyBytes(t, &chatMessagesPagingCaller{responses: []string{response}}, output.RolloutDualValidate)
		if !bytes.Equal(legacyBytes, dual) {
			t.Fatalf("dual validation changed legacy stdout\nlegacy=%s\ndual=%s", legacyBytes, dual)
		}
	})

	t.Run("page all", func(t *testing.T) {
		responses := []string{
			`{"result":{"groups":[{"openConversationId":"g1","title":"群一"}],"hasMore":true,"nextCursor":"c2"}}`,
			`{"result":{"groups":[{"openConversationId":"g2","title":"群二"}],"hasMore":false}}`,
		}
		legacyBytes := runMyGroupsLegacyBytes(t, &chatMessagesPagingCaller{responses: responses}, output.RolloutLegacyOnly, "--page-all", "--page-limit", "2")
		dual := runMyGroupsLegacyBytes(t, &chatMessagesPagingCaller{responses: responses}, output.RolloutDualValidate, "--page-all", "--page-limit", "2")
		if !bytes.Equal(legacyBytes, dual) {
			t.Fatalf("dual validation changed page-all legacy stdout\nlegacy=%s\ndual=%s", legacyBytes, dual)
		}
	})

	helper := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"groups":[{"openConversationId":"g1","title":"群一"}],"hasMore":false}}`,
	}}
	helpers.InitDeps(helper)
	root := newPlatformCoverageRoot()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"chat", "+my-groups", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(helper.args) != 1 {
		t.Fatalf("calls=%d, want one execution", len(helper.args))
	}
	var legacy map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &legacy); err != nil {
		t.Fatalf("decode legacy payload: %v", err)
	}
	if _, exists := legacy["ok"]; exists {
		t.Fatalf("dual validation leaked unified envelope: %#v", legacy)
	}
	if legacy["count"] != float64(1) || legacy["complete"] != true || legacy["stopReason"] != "source_complete" {
		t.Fatalf("legacy payload changed: %#v", legacy)
	}
}

func runMyGroupsLegacyBytes(t *testing.T, fake *chatMessagesPagingCaller, rollout output.RolloutState, args ...string) []byte {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := MyGroups
	declaration.OutputRollout = rollout
	cmd := corecmd.New(shortcutcore.FromShortcut(declaration))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("legacy command execution failed: %v", err)
	}
	return stdout.Bytes()
}

func runMyGroupsUnifiedResult(t *testing.T, fake *chatMessagesPagingCaller, args ...string) (map[string]any, int) {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := MyGroups
	declaration.OutputRollout = output.RolloutUnifiedActive
	cmd := corecmd.New(shortcutcore.FromShortcut(declaration))
	cmd.PersistentFlags().String("format", "json", "")
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("active command execution failed: %v", err)
	}
	exitCode, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted {
		t.Fatalf("active result emission: code=%d emitted=%v err=%v", exitCode, emitted, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode unified envelope: %v\n%s", err, stdout.String())
	}
	if _, present := envelope["contract_version"]; present {
		t.Fatalf("unified envelope must not expose a protocol version: %#v", envelope)
	}
	return envelope, exitCode
}

func TestMyGroupsPromotableUnifiedPaginationOutcomes(t *testing.T) {
	t.Run("terminal page uses framework pagination", func(t *testing.T) {
		envelope, exitCode := runMyGroupsUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":false,"nextCursor":0}}`,
		}}, "--format", "json")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("terminal envelope=%#v exit=%d", envelope, exitCode)
		}
		data := envelope["data"].(map[string]any)
		for _, legacyKey := range []string{"complete", "hasMore", "nextCursor", "failures", "partial", "stopReason", "pagesFetched"} {
			if _, present := data[legacyKey]; present {
				t.Fatalf("legacy field %q leaked into unified data: %#v", legacyKey, data)
			}
		}
		pagination := envelope["meta"].(map[string]any)["pagination"].(map[string]any)
		if pagination["endpoint_exhausted"] != true {
			t.Fatalf("terminal pagination=%#v", pagination)
		}
		groups := data["groups"].([]any)
		group := groups[0].(map[string]any)
		if group["openConversationId"] != "g1" {
			t.Fatalf("unified group handle=%#v, want openConversationId", group)
		}
		if _, legacyName := group["conversationId"]; legacyName {
			t.Fatalf("unified group retained competing conversationId: %#v", group)
		}
	})

	t.Run("later read failure keeps prior page as partial", func(t *testing.T) {
		envelope, exitCode := runMyGroupsUnifiedResult(t, &chatMessagesPagingCaller{
			responses: []string{`{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true,"nextCursor":"c2"}}`},
			failAt:    2,
		}, "--format", "json", "--page-all")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("partial envelope=%#v exit=%d", envelope, exitCode)
		}
		data := envelope["data"].(map[string]any)
		if len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("partial channels=%#v", data)
		}
	})
}

func TestMyGroupsCandidatePaginationContract(t *testing.T) {
	pageData := map[string]any{
		"count":  1,
		"groups": []map[string]any{{"conversationId": "g1"}},
	}
	tests := []struct {
		name    string
		data    map[string]any
		require bool
		state   output.PageState
		exhaust bool
		token   string
	}{
		{
			name:    "terminal endpoint is exhausted",
			data:    map[string]any{"result": map[string]any{"hasMore": false}},
			state:   output.PageStateExhausted,
			exhaust: true,
		},
		{
			name:  "single page without pagination remains unknown",
			data:  map[string]any{"result": map[string]any{}},
			state: output.PageStateUnknown,
		},
		{
			name:    "continuation carries token",
			data:    map[string]any{"result": map[string]any{"hasMore": true, "nextCursor": "c2"}},
			state:   output.PageStateContinuation,
			exhaust: false,
			token:   "c2",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ledger, err := output.NewPageLedger(1)
			if err != nil {
				t.Fatal(err)
			}
			if err := observeMyGroupsPage(ledger, "0", tc.data, pageData, tc.require); err != nil {
				t.Fatalf("observe: %v", err)
			}
			if ledger.State() != tc.state {
				t.Fatalf("state=%q, want %q", ledger.State(), tc.state)
			}
			result, err := myGroupsUnifiedResult(ledger, pageData, false)
			if err != nil {
				t.Fatalf("result: %v", err)
			}
			env, err := output.EnvelopeFromResult(result)
			if err != nil {
				t.Fatalf("EnvelopeFromResult: %v", err)
			}
			if tc.state == output.PageStateUnknown {
				if env.Meta == nil || env.Meta.Pagination != nil || env.Data.(map[string]any)["pagination_known"] != false {
					t.Fatalf("unknown pagination envelope=%#v", env)
				}
				return
			}
			page := env.Meta.Pagination
			if page == nil || page.EndpointExhausted != tc.exhaust || page.NextToken != tc.token {
				t.Fatalf("pagination=%#v want exhausted=%v token=%q", page, tc.exhaust, tc.token)
			}
		})
	}
}

func TestMyGroupsCandidateKeepsFirstPageOnLaterFailure(t *testing.T) {
	ledger, err := output.NewPageLedger(2)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{"result": map[string]any{"hasMore": true, "nextCursor": "c2"}}
	pageData := map[string]any{"count": 1, "groups": []map[string]any{{"conversationId": "g1"}}}
	if err := observeMyGroupsPage(ledger, "0", data, pageData, true); err != nil {
		t.Fatalf("observe first page: %v", err)
	}
	if err := ledger.RecordFailure("c2", myGroupsReadFailureInfo(errors.New("second page unavailable"))); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	result, err := myGroupsUnifiedResult(ledger, pageData, false)
	if err != nil {
		t.Fatalf("result: %v", err)
	}
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatalf("EnvelopeFromResult: %v", err)
	}
	if env.Outcome != output.OutcomePartialFailure || result.ExitCode() != 7 {
		t.Fatalf("partial envelope=%#v", env)
	}
	partial, ok := env.Data.(*output.PartialData)
	if !ok || len(partial.Succeeded) != 1 || len(partial.Failed) != 1 {
		t.Fatalf("partial data=%#v", env.Data)
	}
}
