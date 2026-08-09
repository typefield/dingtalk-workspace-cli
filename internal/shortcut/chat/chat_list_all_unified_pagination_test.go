// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	shortcutcore "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestChatListAllRolloutIsDualValidate(t *testing.T) {
	if ChatListAll.OutputRollout != output.RolloutDualValidate {
		t.Fatalf("chat-list-all rollout=%q, want dual_validate", ChatListAll.OutputRollout)
	}
}

func TestChatListAllDualValidatePreservesLegacyPayload(t *testing.T) {
	t.Run("single page", func(t *testing.T) {
		response := `{"result":{"groups":[{"openConversationId":"g1","title":"群一"}],"hasMore":false}}`
		legacy := runChatListAllLegacyBytes(t, &larkAlignmentCaller{responses: map[string]string{"im/list_my_groups_pagination": response}}, output.RolloutLegacyOnly)
		dual := runChatListAllLegacyBytes(t, &larkAlignmentCaller{responses: map[string]string{"im/list_my_groups_pagination": response}}, output.RolloutDualValidate)
		if !bytes.Equal(legacy, dual) {
			t.Fatalf("dual validation changed legacy stdout\nlegacy=%s\ndual=%s", legacy, dual)
		}
	})

	t.Run("page all", func(t *testing.T) {
		responses := []string{
			`{"result":{"groups":[{"openConversationId":"g1","title":"群一"}],"hasMore":true,"nextCursor":"c2"}}`,
			`{"result":{"groups":[{"openConversationId":"g2","title":"群二"}],"hasMore":false}}`,
		}
		legacy := runChatListAllLegacyBytes(t, &larkAlignmentCaller{sequenceResponses: map[string][]string{"im/list_my_groups_pagination": responses}}, output.RolloutLegacyOnly, "--page-all", "--page-limit", "2")
		dual := runChatListAllLegacyBytes(t, &larkAlignmentCaller{sequenceResponses: map[string][]string{"im/list_my_groups_pagination": responses}}, output.RolloutDualValidate, "--page-all", "--page-limit", "2")
		if !bytes.Equal(legacy, dual) {
			t.Fatalf("dual validation changed page-all legacy stdout\nlegacy=%s\ndual=%s", legacy, dual)
		}
	})
}

func runChatListAllLegacyBytes(t *testing.T, fake *larkAlignmentCaller, rollout output.RolloutState, args ...string) []byte {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := ChatListAll
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

func runChatListAllUnifiedResult(t *testing.T, fake *larkAlignmentCaller, args ...string) (map[string]any, int) {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := ChatListAll
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
		t.Fatalf("decode active envelope: %v\n%s", err, stdout.String())
	}
	if _, present := envelope["contract_version"]; present {
		t.Fatalf("unified envelope must not expose a protocol version: %#v", envelope)
	}
	return envelope, exitCode
}

func TestChatListAllPromotableUnifiedPaginationOutcomes(t *testing.T) {
	t.Run("terminal page uses framework pagination", func(t *testing.T) {
		envelope, exitCode := runChatListAllUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/list_my_groups_pagination": `{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":false,"nextCursor":0}}`,
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
	})

	t.Run("unknown single-page boundary does not claim exhaustion", func(t *testing.T) {
		envelope, exitCode := runChatListAllUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/list_my_groups_pagination": `{"result":{"groups":[{"openConversationId":"g1"}]}}`,
		}}, "--format", "json")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("unknown-boundary envelope=%#v exit=%d", envelope, exitCode)
		}
		meta, _ := envelope["meta"].(map[string]any)
		if meta != nil {
			if _, present := meta["pagination"]; present {
				t.Fatalf("unknown boundary invented pagination=%#v", meta)
			}
		}
		data := envelope["data"].(map[string]any)
		if data["pagination_known"] != false {
			t.Fatalf("unknown boundary data=%#v", data)
		}
	})

	t.Run("contradictory terminal cursor is partial", func(t *testing.T) {
		envelope, exitCode := runChatListAllUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/list_my_groups_pagination": `{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":false,"nextCursor":"c2"}}`,
		}}, "--format", "json")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("contradictory envelope=%#v exit=%d", envelope, exitCode)
		}
	})

	t.Run("later read failure keeps prior page as partial", func(t *testing.T) {
		envelope, exitCode := runChatListAllUnifiedResult(t, &larkAlignmentCaller{
			sequenceResponses: map[string][]string{
				"im/list_my_groups_pagination": {`{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true,"nextCursor":"c2"}}`},
			},
			failProductToolAt: map[string]int{"im/list_my_groups_pagination": 2},
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

func TestChatListAllFailsClosedWithoutStableConversationID(t *testing.T) {
	_, err := chatListAllProject(map[string]any{
		"result": map[string]any{"groups": []any{map[string]any{"title": "展示名"}}},
	})
	if err == nil {
		t.Fatal("display-only group unexpectedly projected")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error=%T %#v; want non-retryable projection_unknown", err, err)
	}
}
