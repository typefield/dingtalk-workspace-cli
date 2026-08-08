// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	frameworkoutput "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	shortcutcore "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestChatSearchPaginationRolloutIsUnifiedActive(t *testing.T) {
	if ChatSearch.OutputRollout != frameworkoutput.RolloutUnifiedActive {
		t.Fatalf("chat-search rollout = %q, want unified_active", ChatSearch.OutputRollout)
	}
}

func runChatSearchUnifiedResult(t *testing.T, fake *larkAlignmentCaller, args ...string) (map[string]any, int) {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := ChatSearch
	declaration.OutputRollout = frameworkoutput.RolloutUnifiedActive
	cmd := corecmd.New(shortcutcore.FromShortcut(declaration))
	// The application root owns this persistent flag. Mount it here too so the
	// promotion proof exercises the public Agent form: --format json.
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
		t.Fatalf("decode active envelope: %v\n%s", err, stdout.String())
	}
	if _, leaked := envelope["contract_version"]; leaked {
		t.Fatalf("active envelope exposed removed contract_version: %#v", envelope)
	}
	return envelope, exitCode
}

func TestChatSearchUnifiedPromotionEvidence(t *testing.T) {
	t.Run("continuation exposes narrow endpoint metadata", func(t *testing.T) {
		envelope, exitCode := runChatSearchUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/search_groups": `{"result":{"groups":[{"openConversationId":"cid-1","title":"项目群"}],"hasMore":true,"nextCursor":"cursor-2"}}`,
		}}, "--format", "json", "--query", "项目")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		meta := envelope["meta"].(map[string]any)
		pagination := meta["pagination"].(map[string]any)
		if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "cursor-2" || pagination["pages"] != float64(1) {
			t.Fatalf("pagination = %#v", pagination)
		}
	})

	t.Run("unknown legacy boundary omits pagination assertion", func(t *testing.T) {
		envelope, exitCode := runChatSearchUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/search_groups": `{"result":[]}`,
		}}, "--format", "json", "--query", "不存在")
		if exitCode != 0 || envelope["outcome"] != "success" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		if _, present := envelope["meta"]; present {
			t.Fatalf("unknown pagination must omit meta.pagination: %#v", envelope)
		}
		data := envelope["data"].(map[string]any)
		if data["pagination_known"] != false {
			t.Fatalf("unknown pagination data = %#v", data)
		}
	})

	t.Run("later page failure keeps successful page", func(t *testing.T) {
		envelope, exitCode := runChatSearchUnifiedResult(t, &larkAlignmentCaller{
			sequenceResponses: map[string][]string{
				"im/search_groups": {`{"result":{"groups":[{"openConversationId":"cid-1","title":"项目群"}],"hasMore":true,"nextCursor":"cursor-2"}}`},
			},
			failProductToolAt: map[string]int{"im/search_groups": 2},
		}, "--format", "json", "--query", "项目", "--page-all")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		data := envelope["data"].(map[string]any)
		if data["total"] != float64(2) || len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("partial data = %#v", data)
		}
	})

	t.Run("contradictory cursor is partial rather than endpoint completion", func(t *testing.T) {
		envelope, exitCode := runChatSearchUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/search_groups": `{"result":{"groups":[],"hasMore":false,"nextCursor":"ghost"}}`,
		}}, "--format", "json", "--query", "项目")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		data := envelope["data"].(map[string]any)
		failed := data["failed"].([]any)
		if len(failed) != 1 || failed[0].(map[string]any)["error"].(map[string]any)["subtype"] != "pagination_inconsistent" {
			t.Fatalf("partial data=%#v", data)
		}
	})

	t.Run("full maximum-window probe is partial rather than complete", func(t *testing.T) {
		groups := make([]map[string]any, chatSearchMaxWindowSize)
		for index := range groups {
			groups[index] = map[string]any{"openConversationId": fmt.Sprintf("probe-%d", index)}
		}
		second, err := json.Marshal(map[string]any{"result": map[string]any{"groups": groups, "hasMore": false}})
		if err != nil {
			t.Fatal(err)
		}
		envelope, exitCode := runChatSearchUnifiedResult(t, &larkAlignmentCaller{sequenceResponses: map[string][]string{
			"im/search_groups": {
				`{"result":{"groups":[{"openConversationId":"first"}],"hasMore":false}}`,
				string(second),
			},
		}}, "--format", "json", "--query", "项目", "--page-size", "1", "--page-all")
		if exitCode != 7 || envelope["outcome"] != "partial_failure" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
	})
}

func TestCrossPlatformCoverageChatSearchDryRunStopsBeforeRead(t *testing.T) {
	fake := &larkAlignmentCaller{dryRun: true}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{
		"chat", "+chat-search", "--query", "项目", "--page-size", "20",
		"--cursor", "cursor-1", "--exclude-muted", "--dry-run",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("chat-search dry-run made lower calls: %#v", fake.calls)
	}
}

func TestCrossPlatformCoverageChatSearchPageAllUsesOpaqueCursorAndDeduplicates(t *testing.T) {
	fake := &larkAlignmentCaller{sequenceResponses: map[string][]string{
		"im/search_groups": {
			`{"result":{"groups":[{"openConversationId":"cid-1","title":"项目一群"}],"hasMore":true,"nextCursor":"cursor-2"}}`,
			`{"result":{"groups":[{"openConversationId":"cid-1","title":"重复项目群"},{"openConversationId":"cid-2","title":"项目二群"}],"hasMore":false}}`,
		},
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+chat-search", "--query", "项目", "--page-size", "1", "--page-all", "--page-limit", "5"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 2 || fake.calls[0].args["cursor"] != "0" || fake.calls[1].args["cursor"] != "cursor-2" {
		t.Fatalf("calls = %#v", fake.calls)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ok"] != true || payload["outcome"] != "success" {
		t.Fatalf("envelope = %#v", payload)
	}
	data := payload["data"].(map[string]any)
	if data["count"] != float64(2) {
		t.Fatalf("data = %#v", data)
	}
	chats := data["chats"].([]any)
	if chats[0].(map[string]any)["name"] != "项目一群" || chats[1].(map[string]any)["openConversationId"] != "cid-2" {
		t.Fatalf("chats = %#v", chats)
	}
	pagination := payload["meta"].(map[string]any)["pagination"].(map[string]any)
	if pagination["endpoint_exhausted"] != true || pagination["pages"] != float64(2) || pagination["items"] != float64(2) {
		t.Fatalf("pagination = %#v", pagination)
	}
}

func TestCrossPlatformCoverageChatSearchProbesMaximumWindowWhenBackendFalselyEndsFullFirstPage(t *testing.T) {
	fake := &larkAlignmentCaller{sequenceResponses: map[string][]string{
		"im/search_groups": {
			`{"result":{"groups":[{"openConversationId":"cid-1","title":"项目一群"}],"hasMore":false}}`,
			`{"result":{"groups":[{"openConversationId":"cid-1","title":"项目一群"},{"openConversationId":"cid-2","title":"项目二群"}],"hasMore":false}}`,
		},
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+chat-search", "--query", "项目", "--page-size", "1", "--page-all", "--page-limit", "5"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 2 || fake.calls[0].args["limit"] != 1 || fake.calls[1].args["limit"] != 100 {
		t.Fatalf("calls = %#v", fake.calls)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ok"] != true || payload["outcome"] != "success" {
		t.Fatalf("envelope = %#v", payload)
	}
	if _, present := payload["meta"]; present {
		t.Fatalf("maximum-window probe must not assert endpoint exhaustion: %#v", payload)
	}
	data := payload["data"].(map[string]any)
	if data["count"] != float64(2) || data["pagination_known"] != false {
		t.Fatalf("data = %#v", data)
	}
}

func TestCrossPlatformCoverageChatSearchPageTokenAndPageLimit(t *testing.T) {
	fake := &larkAlignmentCaller{responses: map[string]string{
		"im/search_groups": `{"result":{"groups":[{"openConversationId":"cid-2","title":"项目二群"}],"hasMore":true,"nextCursor":"cursor-3"}}`,
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+chat-search", "--query", "项目", "--page-token", "cursor-2", "--page-all", "--page-limit", "1"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 || fake.calls[0].args["cursor"] != "cursor-2" {
		t.Fatalf("calls = %#v", fake.calls)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["ok"] != true || payload["outcome"] != "success" {
		t.Fatalf("envelope = %#v", payload)
	}
	pagination := payload["meta"].(map[string]any)["pagination"].(map[string]any)
	if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "cursor-3" || pagination["pages"] != float64(1) {
		t.Fatalf("pagination = %#v", pagination)
	}
}

func TestCrossPlatformCoverageChatSearchFailureModes(t *testing.T) {
	t.Run("later read failure keeps partial result", func(t *testing.T) {
		fake := &larkAlignmentCaller{
			sequenceResponses: map[string][]string{
				"im/search_groups": {`{"result":{"groups":[{"openConversationId":"cid-1","title":"项目群"}],"hasMore":true,"nextCursor":"cursor-2"}}`},
			},
			failProductToolAt: map[string]int{"im/search_groups": 2},
		}
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"chat", "+chat-search", "--query", "项目", "--page-all"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["ok"] != false || payload["outcome"] != "partial_failure" {
			t.Fatalf("envelope = %#v", payload)
		}
		data := payload["data"].(map[string]any)
		if len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("partial data = %#v", data)
		}
	})

	t.Run("maximum window probe failure preserves provisional result", func(t *testing.T) {
		fake := &larkAlignmentCaller{
			sequenceResponses: map[string][]string{
				"im/search_groups": {`{"result":{"groups":[{"openConversationId":"cid-1","title":"项目群"}],"hasMore":false}}`},
			},
			failProductToolAt: map[string]int{"im/search_groups": 2},
		}
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"chat", "+chat-search", "--query", "项目", "--page-size", "1", "--page-all", "--page-limit", "5"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["ok"] != false || payload["outcome"] != "partial_failure" {
			t.Fatalf("envelope = %#v", payload)
		}
		data := payload["data"].(map[string]any)
		if len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("partial data = %#v", data)
		}
	})

	t.Run("full legacy page without pagination fails closed", func(t *testing.T) {
		fake := &larkAlignmentCaller{responses: map[string]string{
			"im/search_groups": `{"result":{"groups":[{"openConversationId":"cid-1","title":"项目群"}]}}`,
		}}
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"chat", "+chat-search", "--query", "项目", "--page-size", "1", "--page-all"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var envelope map[string]any
		if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope["outcome"] != "partial_failure" {
			t.Fatalf("missing pagination envelope = %#v", envelope)
		}
	})

	t.Run("short legacy page is bounded complete", func(t *testing.T) {
		fake := &larkAlignmentCaller{responses: map[string]string{"im/search_groups": `{"result":[]}`}}
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"chat", "+chat-search", "--query", "不存在"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["ok"] != true || payload["outcome"] != "success" {
			t.Fatalf("envelope = %#v", payload)
		}
		if _, present := payload["meta"]; present {
			t.Fatalf("unknown short-page boundary must omit pagination: %#v", payload)
		}
		if payload["data"].(map[string]any)["pagination_known"] != false {
			t.Fatalf("data = %#v", payload["data"])
		}
	})
}

func TestCrossPlatformCoverageChatSearchPaginationValidation(t *testing.T) {
	for _, args := range [][]string{
		{"--page-size", "0"},
		{"--page-limit", "2"},
		{"--page-all", "--page-limit", "0"},
		{"--page-all", "--page-limit", "501"},
		{"--cursor", "c1", "--page-token", "c2"},
	} {
		helpers.InitDeps(&larkAlignmentCaller{})
		root := newPlatformCoverageRoot()
		root.SetArgs(append([]string{"chat", "+chat-search", "--query", "项目"}, args...))
		if err := root.Execute(); err == nil {
			t.Fatalf("invalid args succeeded: %v", args)
		}
	}
}

func TestCrossPlatformCoverageChatSearchAdditionalPaginationEdges(t *testing.T) {
	run := func(t *testing.T, fake *larkAlignmentCaller, args ...string) (map[string]any, error) {
		t.Helper()
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs(append([]string{"chat", "+chat-search", "--query", "项目"}, args...))
		err := root.Execute()
		if output.Len() == 0 {
			return nil, err
		}
		var payload map[string]any
		if decodeErr := json.Unmarshal(output.Bytes(), &payload); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		return payload, err
	}

	t.Run("first read failure", func(t *testing.T) {
		_, err := run(t, &larkAlignmentCaller{failProductToolAt: map[string]int{"im/search_groups": 1}})
		if err == nil {
			t.Fatal("first read failure unexpectedly succeeded")
		}
	})

	t.Run("cursor without hasMore continues", func(t *testing.T) {
		payload, err := run(t, &larkAlignmentCaller{sequenceResponses: map[string][]string{
			"im/search_groups": {
				`{"result":{"groups":[{"openConversationId":"g1"}],"nextCursor":"next"}}`,
				`{"result":{"groups":[],"hasMore":false}}`,
			},
		}}, "--page-all")
		pagination := payload["meta"].(map[string]any)["pagination"].(map[string]any)
		if err != nil || payload["outcome"] != "success" || pagination["endpoint_exhausted"] != true || pagination["pages"] != float64(2) {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	t.Run("single full page is untrusted", func(t *testing.T) {
		payload, err := run(t, &larkAlignmentCaller{responses: map[string]string{
			"im/search_groups": `{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":false}}`,
		}}, "--page-size", "1")
		if err != nil || payload["outcome"] != "success" || payload["data"].(map[string]any)["pagination_known"] != false {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	t.Run("false hasMore with cursor fails closed", func(t *testing.T) {
		payload, err := run(t, &larkAlignmentCaller{responses: map[string]string{
			"im/search_groups": `{"result":{"groups":[],"hasMore":false,"nextCursor":"ghost"}}`,
		}})
		if err != nil || payload["outcome"] != "partial_failure" {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	t.Run("bounded probe reports page limit", func(t *testing.T) {
		payload, err := run(t, &larkAlignmentCaller{responses: map[string]string{
			"im/search_groups": `{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":false}}`,
		}}, "--page-size", "1", "--page-all", "--page-limit", "1")
		if err != nil || payload["outcome"] != "success" || payload["data"].(map[string]any)["pagination_known"] != false {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	t.Run("full maximum probe fails closed", func(t *testing.T) {
		groups := make([]map[string]any, chatSearchMaxWindowSize)
		for i := range groups {
			groups[i] = map[string]any{"openConversationId": fmt.Sprintf("probe-%d", i)}
		}
		second, marshalErr := json.Marshal(map[string]any{"result": map[string]any{"groups": groups, "hasMore": false}})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		payload, err := run(t, &larkAlignmentCaller{sequenceResponses: map[string][]string{
			"im/search_groups": {
				`{"result":{"groups":[{"openConversationId":"first"}],"hasMore":false}}`,
				string(second),
			},
		}}, "--page-size", "1", "--page-all")
		if err != nil || payload["outcome"] != "partial_failure" || len(payload["data"].(map[string]any)["failed"].([]any)) != 1 {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	for _, tc := range []struct {
		name        string
		response    string
		args        []string
		wantOutcome string
	}{
		{name: "missing continuation", response: `{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true}}`, wantOutcome: "partial_failure"},
		{name: "single page continuation", response: `{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true,"nextCursor":"next"}}`, wantOutcome: "success"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := run(t, &larkAlignmentCaller{responses: map[string]string{"im/search_groups": tc.response}, failProductToolAt: map[string]int{}}, tc.args...)
			if err != nil || payload["outcome"] != tc.wantOutcome {
				t.Fatalf("payload=%#v err=%v", payload, err)
			}
			if tc.wantOutcome == "success" {
				pagination := payload["meta"].(map[string]any)["pagination"].(map[string]any)
				if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "next" {
					t.Fatalf("pagination=%#v", pagination)
				}
			}
		})
	}

	t.Run("exclude muted reaches lower call and output errors propagate", func(t *testing.T) {
		fake := &larkAlignmentCaller{responses: map[string]string{
			"im/search_groups": `{"result":{"groups":[],"hasMore":false}}`,
		}}
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		root.SetOut(chatOutputErrorWriter{err: errors.New("fixture output")})
		root.SetArgs([]string{"chat", "+chat-search", "--query", "项目", "--exclude-muted"})
		if err := root.Execute(); err == nil || fake.calls[0].args["excludeMuted"] != true {
			t.Fatalf("calls=%#v err=%v", fake.calls, err)
		}
	})
}

func TestCrossPlatformCoverageChatSearchProjectionEdges(t *testing.T) {
	if got := chatSearchItems(nil); len(got) != 0 {
		t.Fatalf("nil items = %#v", got)
	}
	if got := chatSearchItems(map[string]any{"result": map[string]any{"unknown": true}}); len(got) != 0 {
		t.Fatalf("unknown items = %#v", got)
	}
	got := projectChatSearchItems([]any{
		"invalid",
		map[string]any{"conversationId": "g1", "conversationName": "项目群"},
	})
	if len(got) != 1 || got[0]["openConversationId"] != "g1" || got[0]["name"] != "项目群" {
		t.Fatalf("projected = %#v", got)
	}
	for _, value := range []any{nil, "", "0", "<nil>"} {
		if cursor := chatSearchCursorString(value); cursor != "" {
			t.Fatalf("cursor %v = %q", value, cursor)
		}
	}

	helpers.InitDeps(&larkAlignmentCaller{responses: map[string]string{
		"im/search_groups": `{"result":{"groups":[],"hasMore":false}}`,
	}})
	root := newPlatformCoverageRoot()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"chat", "+chat-search", "--query", "项目", "--cursor="})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}
