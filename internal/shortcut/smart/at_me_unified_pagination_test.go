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
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	frameworkoutput "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	shortcutcore "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func runAtMeUnifiedResult(t *testing.T, fake *chatMessagesPagingCaller, args ...string) (map[string]any, int) {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := AtMe
	declaration.OutputRollout = frameworkoutput.RolloutUnifiedActive
	cmd := corecmd.New(shortcutcore.FromShortcut(declaration))
	cmd.PersistentFlags().String("format", "json", "")
	ctx, _ := frameworkoutput.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("active command execution failed: %v", err)
	}
	exitCode, emitted, err := frameworkoutput.EmitStoredResult(cmd)
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

func runAtMeLegacyBytes(t *testing.T, fake *chatMessagesPagingCaller, rollout frameworkoutput.RolloutState, args ...string) []byte {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := AtMe
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

func TestAtMeDualValidateKeepsLegacyBytes(t *testing.T) {
	response := `{"result":{"conversationMessagesList":[{"openConversationId":"cid","messages":[{"openMessageId":"m1"}]}],"hasMore":false}}`
	legacy := runAtMeLegacyBytes(t, &chatMessagesPagingCaller{responses: []string{response}}, frameworkoutput.RolloutLegacyOnly)
	dual := runAtMeLegacyBytes(t, &chatMessagesPagingCaller{responses: []string{response}}, frameworkoutput.RolloutDualValidate)
	if !bytes.Equal(legacy, dual) {
		t.Fatalf("dual validation changed legacy stdout\nlegacy=%s\ndual=%s", legacy, dual)
	}
}

func TestAtMeUnifiedPaginationOutcomes(t *testing.T) {
	baseArgs := []string{"--format", "json", "--page-all"}
	t.Run("terminal page is exhausted and legacy fields do not leak", func(t *testing.T) {
		envelope, exitCode := runAtMeUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"conversationMessagesList":[],"hasMore":false,"nextCursor":0}}`,
		}}, "--format", "json")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("terminal envelope=%#v exit=%d", envelope, exitCode)
		}
		data := atMeSuccessData(t, envelope)
		for _, legacyKey := range []string{"complete", "hasMore", "nextCursor", "failures", "partial", "stopReason", "pagesFetched"} {
			if _, present := data[legacyKey]; present {
				t.Fatalf("legacy pagination field %q leaked into active data: %#v", legacyKey, data)
			}
		}
		pagination := atMePagination(t, envelope)
		if pagination["endpoint_exhausted"] != true {
			t.Fatalf("terminal pagination=%#v", pagination)
		}
		if _, present := pagination["next_token"]; present {
			t.Fatalf("terminal pagination unexpectedly has a continuation=%#v", pagination)
		}
	})

	t.Run("page budget publishes a resumable continuation", func(t *testing.T) {
		envelope, exitCode := runAtMeUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"conversationMessagesList":[{"messages":[{"openMessageId":"m1"}]}],"hasMore":true,"nextCursor":"cursor-2"}}`,
		}}, append(baseArgs, "--page-limit", "1")...)
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("continuation envelope=%#v exit=%d", envelope, exitCode)
		}
		pagination := atMePagination(t, envelope)
		if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "cursor-2" || pagination["pages"] != float64(1) {
			t.Fatalf("continuation pagination=%#v", pagination)
		}
	})

	t.Run("single page without evidence remains successful but unknown", func(t *testing.T) {
		envelope, exitCode := runAtMeUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"conversationMessagesList":[]}}`,
		}}, "--format", "json")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("unknown evidence envelope=%#v exit=%d", envelope, exitCode)
		}
		if data := atMeSuccessData(t, envelope); data["pagination_known"] != false {
			t.Fatalf("unknown endpoint evidence=%#v", envelope)
		}
		if meta, _ := envelope["meta"].(map[string]any); meta != nil && meta["pagination"] != nil {
			t.Fatalf("unknown endpoint evidence must not emit pagination meta: %#v", envelope)
		}
	})

	t.Run("page all without endpoint evidence is partial", func(t *testing.T) {
		envelope, exitCode := runAtMeUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"conversationMessagesList":[]}}`,
		}}, baseArgs...)
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("unknown full scan envelope=%#v exit=%d", envelope, exitCode)
		}
		failed := atMePartialData(t, envelope)["failed"].([]any)
		if len(failed) != 1 || failed[0].(map[string]any)["error"].(map[string]any)["subtype"] != "pagination_inconsistent" {
			t.Fatalf("unknown full scan failed details=%#v", failed)
		}
	})

	t.Run("later read failure preserves the completed page", func(t *testing.T) {
		envelope, exitCode := runAtMeUnifiedResult(t, &chatMessagesPagingCaller{
			responses: []string{`{"result":{"conversationMessagesList":[{"messages":[{"openMessageId":"m1"}]}],"hasMore":true,"nextCursor":"cursor-2"}}`},
			failAt:    2,
		}, baseArgs...)
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("later failure envelope=%#v exit=%d", envelope, exitCode)
		}
		data := atMePartialData(t, envelope)
		if len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("later failure details=%#v", data)
		}
	})

	t.Run("unknown response shape is typed failure", func(t *testing.T) {
		envelope, exitCode := runAtMeUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":false,"conversationMessagesList":[{}]}}`,
		}}, "--format", "json")
		if exitCode != 1 || envelope["ok"] != false || envelope["outcome"] != "failure" {
			t.Fatalf("projection envelope=%#v exit=%d", envelope, exitCode)
		}
		if errorInfo := envelope["error"].(map[string]any); errorInfo["subtype"] != "projection_unknown" {
			t.Fatalf("projection error=%#v", errorInfo)
		}
	})
}
