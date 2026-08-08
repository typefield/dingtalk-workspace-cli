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
	frameworkoutput "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	shortcutcore "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// These tests promote a copied declaration only. Production remains
// dual_validate until an Agent has reviewed the real command surface and the
// local export/resource-writing modes have their own active-contract rules.
func runChatMessagesUnifiedResult(t *testing.T, fake *chatMessagesPagingCaller, args ...string) (map[string]any, int) {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := ChatMessages
	declaration.OutputRollout = frameworkoutput.RolloutUnifiedActive
	cmd := corecmd.New(shortcutcore.FromShortcut(declaration))
	// The application root owns this persistent flag. Mount it here too so the
	// promotion probe exercises the public Agent form: --format json.
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

func runChatMessagesLegacyBytes(t *testing.T, fake *chatMessagesPagingCaller, rollout frameworkoutput.RolloutState, args ...string) []byte {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := ChatMessages
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

func TestChatMessagesDualValidateKeepsLegacyBytes(t *testing.T) {
	response := `{"result":{"hasMore":false,"messages":[{"openMessageId":"m1","createTime":"2026-08-06 21:28:39"}]}}`
	args := []string{"--conversation-id", "cid"}
	legacy := runChatMessagesLegacyBytes(t, &chatMessagesPagingCaller{responses: []string{response}}, frameworkoutput.RolloutLegacyOnly, args...)
	dual := runChatMessagesLegacyBytes(t, &chatMessagesPagingCaller{responses: []string{response}}, frameworkoutput.RolloutDualValidate, args...)
	if !bytes.Equal(legacy, dual) {
		t.Fatalf("dual validation changed legacy stdout\nlegacy=%s\ndual=%s", legacy, dual)
	}
}

func TestChatMessagesUnifiedPaginationOutcomes(t *testing.T) {
	baseArgs := []string{"--format", "json", "--conversation-id", "cid", "--page-all"}
	t.Run("terminal zero cursor is exhausted success", func(t *testing.T) {
		envelope, exitCode := runChatMessagesUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":false,"nextCursor":0,"messages":[]}}`,
		}}, "--format", "json", "--conversation-id", "cid")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("terminal envelope=%#v exit=%d", envelope, exitCode)
		}
		pagination := envelope["meta"].(map[string]any)["pagination"].(map[string]any)
		if pagination["endpoint_exhausted"] != true {
			t.Fatalf("terminal pagination=%#v", pagination)
		}
		if _, present := pagination["next_token"]; present {
			t.Fatalf("terminal pagination unexpectedly has a continuation=%#v", pagination)
		}
	})

	t.Run("continuation is resumable success", func(t *testing.T) {
		envelope, exitCode := runChatMessagesUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":true,"nextCursor":1786022919361,"messages":[{"openMessageId":"m1","createTime":"2026-08-06 21:28:39"}]}}`,
		}}, append(baseArgs, "--page-limit", "1")...)
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("continuation envelope=%#v exit=%d", envelope, exitCode)
		}
		meta, _ := envelope["meta"].(map[string]any)
		pagination, _ := meta["pagination"].(map[string]any)
		if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "1786022919361" || pagination["pages"] != float64(1) {
			t.Fatalf("continuation pagination=%#v", pagination)
		}
	})

	t.Run("missing endpoint evidence is not exhaustion", func(t *testing.T) {
		envelope, exitCode := runChatMessagesUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"messages":[{"openMessageId":"m1","createTime":"2026-08-06 21:28:39"}]}}`,
		}}, "--format", "json", "--conversation-id", "cid")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("unknown envelope=%#v exit=%d", envelope, exitCode)
		}
		if meta, _ := envelope["meta"].(map[string]any); meta != nil && meta["pagination"] != nil {
			t.Fatalf("unknown endpoint evidence must not emit pagination meta: %#v", envelope)
		}
		data, _ := envelope["data"].(map[string]any)
		if data["pagination_known"] != false {
			t.Fatalf("unknown endpoint evidence=%#v", envelope)
		}
	})

	t.Run("later read failure preserves successful pages", func(t *testing.T) {
		envelope, exitCode := runChatMessagesUnifiedResult(t, &chatMessagesPagingCaller{
			responses: []string{`{"result":{"hasMore":true,"nextCursor":1786022919361,"messages":[{"openMessageId":"m1","createTime":"2026-08-06 21:28:39"}]}}`},
			failAt:    2,
		}, baseArgs...)
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("partial envelope=%#v exit=%d", envelope, exitCode)
		}
		data, _ := envelope["data"].(map[string]any)
		if len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("partial details=%#v", data)
		}
	})

	t.Run("terminal flag with a cursor is partial failure", func(t *testing.T) {
		envelope, exitCode := runChatMessagesUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":false,"nextCursor":1786022919361,"messages":[{"openMessageId":"m1","createTime":"2026-08-06 21:28:39"}]}}`,
		}}, baseArgs...)
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("contradictory cursor envelope=%#v exit=%d", envelope, exitCode)
		}
		data, _ := envelope["data"].(map[string]any)
		failed, _ := data["failed"].([]any)
		if len(failed) != 1 || failed[0].(map[string]any)["error"].(map[string]any)["subtype"] != "pagination_inconsistent" {
			t.Fatalf("contradictory cursor details=%#v", data)
		}
	})

	t.Run("unprojectable page is typed failure", func(t *testing.T) {
		envelope, exitCode := runChatMessagesUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":false,"messages":[{}]}}`,
		}}, baseArgs...)
		if exitCode != 1 || envelope["ok"] != false || envelope["outcome"] != "failure" {
			t.Fatalf("projection envelope=%#v exit=%d", envelope, exitCode)
		}
		errorInfo, _ := envelope["error"].(map[string]any)
		if errorInfo["subtype"] != "projection_unknown" {
			t.Fatalf("projection error=%#v", errorInfo)
		}
	})

	t.Run("requested resource failure is partial rather than read success", func(t *testing.T) {
		envelope, exitCode := runChatMessagesUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			"{\"result\":{\"hasMore\":false,\"messages\":[{\"openMessageId\":\"m1\",\"createTime\":\"2026-08-06 21:28:39\",\"content\":\"{\\\"mediaId\\\":\\\"media-1\\\"}\"}]}}",
		}}, "--format", "json", "--conversation-id", "cid", "--download-resources")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("resource partial envelope=%#v exit=%d", envelope, exitCode)
		}
		data, _ := envelope["data"].(map[string]any)
		if len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("resource partial details=%#v", data)
		}
		failure := data["failed"].([]any)[0].(map[string]any)["error"].(map[string]any)
		details, _ := failure["details"].(map[string]any)
		resourceLedger, _ := details["resource_downloads"].(map[string]any)
		if resourceLedger["failedCount"] != float64(1) {
			t.Fatalf("resource ledger=%#v", resourceLedger)
		}
	})

	t.Run("requested local export failure is partial rather than read success", func(t *testing.T) {
		originalWrite := writeChatMessagesExportJSON
		writeChatMessagesExportJSON = func(string, bool, any) (string, int, error) {
			return "", 0, errors.New("fixture local export failure")
		}
		t.Cleanup(func() { writeChatMessagesExportJSON = originalWrite })

		envelope, exitCode := runChatMessagesUnifiedResult(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":false,"messages":[{"openMessageId":"m1","createTime":"2026-08-06 21:28:39"}]}}`,
		}}, "--format", "json", "--conversation-id", "cid", "--output", "exports/messages.json")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("export partial envelope=%#v exit=%d", envelope, exitCode)
		}
		data, _ := envelope["data"].(map[string]any)
		if len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("export partial details=%#v", data)
		}
		failure := data["failed"].([]any)[0].(map[string]any)["error"].(map[string]any)
		if failure["operation"] != "chat/message_export" || failure["origin"] != "local_file" {
			t.Fatalf("export failure=%#v", failure)
		}
		details, _ := failure["details"].(map[string]any)
		export, _ := details["export"].(map[string]any)
		if export["local_path"] != "exports/messages.json" || export["overwrite"] != false {
			t.Fatalf("export failure details=%#v", details)
		}
	})
}
