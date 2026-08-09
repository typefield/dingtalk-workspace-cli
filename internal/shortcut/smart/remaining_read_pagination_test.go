// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package smart

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
)

func TestCrossPlatformCoverageAtMePageAllUsesOpaqueCursorAndDeduplicates(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"conversationMessagesList":[{"openConversationId":"cid","title":"群","messages":[{"openMessageId":"m2"},{"openMessageId":"m1"}]}],"hasMore":true,"nextCursor":"cursor-2"}}`,
		`{"result":{"conversationMessagesList":[{"openConversationId":"cid","title":"群","messages":[{"openMessageId":"m1"},{"openMessageId":"m0"}]}],"hasMore":false}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+at-me", "--limit", "2", "--page-all", "--page-limit", "5"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.args) != 2 || caller.args[0]["cursor"] != "0" || caller.args[1]["cursor"] != "cursor-2" {
		t.Fatalf("calls = %#v", caller.args)
	}
	var envelope map[string]any
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	payload := atMeSuccessData(t, envelope)
	pagination := atMePagination(t, envelope)
	if payload["count"] != float64(3) || pagination["pages"] != float64(2) || pagination["endpoint_exhausted"] != true {
		t.Fatalf("envelope = %#v", envelope)
	}
	if len(payload["items"].([]any)) != 3 {
		t.Fatalf("compatibility items = %#v", payload["items"])
	}
}

func TestCrossPlatformCoverageAtMePageAllFailsClosedWithoutContinuation(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"conversationMessagesList":[{"messages":[{"openMessageId":"m1"}]}],"hasMore":true}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+at-me", "--page-all"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	payload := atMePartialData(t, envelope)
	failed := payload["failed"].([]any)
	if len(failed) != 1 || failed[0].(map[string]any)["error"].(map[string]any)["subtype"] != "pagination_inconsistent" {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestCrossPlatformCoverageAtMePageAllContinuesAcrossEmptyIntermediatePage(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"conversationMessagesList":[],"hasMore":true,"nextCursor":"cursor-2"}}`,
		`{"result":{"conversationMessagesList":[{"messages":[{"openMessageId":"m1"}]}],"hasMore":false}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+at-me", "--page-all", "--page-limit", "5"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	payload := atMeSuccessData(t, envelope)
	pagination := atMePagination(t, envelope)
	if payload["count"] != float64(1) || pagination["pages"] != float64(2) || pagination["endpoint_exhausted"] != true {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestCrossPlatformCoverageMyGroupsPageAllUsesNumericCursorAndFiltersAfterMerge(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"groups":[{"openConversationId":"g1","title":"群一","groupType":"group"}],"hasMore":true,"nextCursor":88}}`,
		`{"result":{"groups":[{"openConversationId":"g1","title":"重复群","groupType":"group"},{"openConversationId":"g2","title":"单聊","groupType":"p2p"}],"hasMore":false}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+my-groups", "--type", "group", "--page-all", "--page-limit", "5"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.args) != 2 || caller.args[1]["cursor"] != float64(88) {
		t.Fatalf("calls = %#v", caller.args)
	}
	var envelope map[string]any
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data, _ := envelope["data"].(map[string]any)
	meta, _ := envelope["meta"].(map[string]any)
	pagination, _ := meta["pagination"].(map[string]any)
	if envelope["ok"] != true || envelope["outcome"] != "success" ||
		data["count"] != float64(1) || len(data["groups"].([]any)) != 1 ||
		pagination["pages"] != float64(2) || pagination["endpoint_exhausted"] != true {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestCrossPlatformCoverageRemainingReadPaginationValidation(t *testing.T) {
	for _, args := range [][]string{
		{"chat", "+at-me", "--page-limit", "2"},
		{"chat", "+at-me", "--page-all", "--page-limit", "501"},
		{"chat", "+my-groups", "--limit", "201"},
		{"chat", "+my-groups", "--page-limit", "2"},
		{"chat", "+my-groups", "--page-all", "--page-limit", "501"},
	} {
		helpers.InitDeps(&chatMessagesPagingCaller{})
		root := newPlatformCoverageRoot()
		root.SetArgs(args)
		if err := root.Execute(); err == nil {
			t.Fatalf("invalid args succeeded: %v", args)
		}
	}
}

func TestMyGroupsProjectionFailsClosedOnUnknownOrUnaddressableRows(t *testing.T) {
	tests := []struct {
		name      string
		response  map[string]any
		wantCount int
		wantError bool
	}{
		{
			name:      "known empty group list",
			response:  map[string]any{"result": map[string]any{"groups": []any{}}},
			wantCount: 0,
		},
		{
			name:      "unknown list container",
			response:  map[string]any{"result": map[string]any{"rows": []any{}}},
			wantError: true,
		},
		{
			name:      "non object list row",
			response:  map[string]any{"result": map[string]any{"groups": []any{"not-a-group"}}},
			wantError: true,
		},
		{
			name:      "row without stable conversation id",
			response:  map[string]any{"result": map[string]any{"groups": []any{map[string]any{"title": "display-only"}}}},
			wantError: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groups, err := myGroupsExtract(tc.response)
			if (err != nil) != tc.wantError {
				t.Fatalf("myGroupsExtract() error = %v, wantError=%v", err, tc.wantError)
			}
			if err == nil && len(groups) != tc.wantCount {
				t.Fatalf("myGroupsExtract() count = %d, want %d", len(groups), tc.wantCount)
			}
		})
	}
}

func TestMyGroupsLaterProjectionFailureKeepsPriorPageAndDisablesReplay(t *testing.T) {
	payload, err := func() (map[string]any, error) {
		helper := &chatMessagesPagingCaller{responses: []string{
			`{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true,"nextCursor":2}}`,
			`{"result":{"groups":[{"title":"display-only"}],"hasMore":false}}`,
		}}
		helpers.InitDeps(helper)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"chat", "+my-groups", "--page-all", "--page-limit", "5"})
		err := root.Execute()
		var payload map[string]any
		if decodeErr := json.Unmarshal(output.Bytes(), &payload); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		return payload, err
	}()
	data, _ := payload["data"].(map[string]any)
	if err != nil || payload["ok"] != false || payload["outcome"] != "partial_failure" ||
		len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestCrossPlatformCoverageMyGroupsAdditionalEdges(t *testing.T) {
	run := func(t *testing.T, caller *chatMessagesPagingCaller, args ...string) (map[string]any, error) {
		t.Helper()
		helpers.InitDeps(caller)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs(append([]string{"chat", "+my-groups"}, args...))
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

	t.Run("single-page outcomes", func(t *testing.T) {
		payload, err := run(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"groups":[],"hasMore":false}}`,
		}}, "--cursor", "cursor-1")
		meta, _ := payload["meta"].(map[string]any)
		pagination, _ := meta["pagination"].(map[string]any)
		if err != nil || payload["ok"] != true || payload["outcome"] != "success" || pagination["endpoint_exhausted"] != true {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
		payload, err = run(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true,"nextCursor":2}}`,
		}})
		meta, _ = payload["meta"].(map[string]any)
		pagination, _ = meta["pagination"].(map[string]any)
		if err != nil || payload["ok"] != true || payload["outcome"] != "success" ||
			pagination["endpoint_exhausted"] != false || pagination["next_token"] != "2" {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
		payload, err = run(t, &chatMessagesPagingCaller{failAt: 1})
		if err != nil || payload["ok"] != false || payload["outcome"] != "failure" {
			t.Fatalf("single-page failure payload=%#v err=%v", payload, err)
		}
	})

	t.Run("all-page failures", func(t *testing.T) {
		payload, err := run(t, &chatMessagesPagingCaller{failAt: 1}, "--page-all")
		if err != nil || payload["ok"] != false || payload["outcome"] != "failure" {
			t.Fatalf("first-page failure payload=%#v err=%v", payload, err)
		}
		payload, err = run(t, &chatMessagesPagingCaller{
			responses: []string{`{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true,"nextCursor":2}}`},
			failAt:    2,
		}, "--page-all")
		data, _ := payload["data"].(map[string]any)
		if err != nil || payload["ok"] != false || payload["outcome"] != "partial_failure" ||
			len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	for _, tc := range []struct {
		name      string
		responses []string
		args      []string
		outcome   string
		checkPage bool
	}{
		{name: "unknown pagination", responses: []string{`{"result":{"groups":[]}}`}, args: []string{"--page-all"}, outcome: "partial_failure"},
		{name: "missing continuation", responses: []string{`{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true}}`}, args: []string{"--page-all"}, outcome: "partial_failure"},
		{name: "stalled continuation", responses: []string{
			`{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true,"nextCursor":2}}`,
			`{"result":{"groups":[{"openConversationId":"g2"}],"hasMore":true,"nextCursor":2}}`,
		}, args: []string{"--page-all"}, outcome: "partial_failure"},
		{name: "page limit", responses: []string{`{"result":{"groups":[{"openConversationId":"g1"}],"hasMore":true,"nextCursor":2}}`}, args: []string{"--page-all", "--page-limit", "1"}, outcome: "success", checkPage: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := run(t, &chatMessagesPagingCaller{responses: tc.responses}, tc.args...)
			if err != nil || payload["outcome"] != tc.outcome {
				t.Fatalf("payload=%#v err=%v", payload, err)
			}
			if tc.checkPage {
				meta, _ := payload["meta"].(map[string]any)
				pagination, _ := meta["pagination"].(map[string]any)
				if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "2" {
					t.Fatalf("page-limit pagination=%#v", pagination)
				}
			}
		})
	}

	t.Run("output failures", func(t *testing.T) {
		helpers.InitDeps(&chatMessagesPagingCaller{responses: []string{`{"result":{"groups":[],"hasMore":false}}`}})
		for _, args := range [][]string{{"chat", "+my-groups"}, {"chat", "+my-groups", "--page-all"}} {
			root := newPlatformCoverageRoot()
			root.SetOut(chatMessagesFailWriter{})
			root.SetArgs(args)
			if err := root.Execute(); err == nil {
				t.Fatalf("output failure swallowed for %v", args)
			}
		}
	})

	for _, value := range []any{nil, "", "0", "<nil>", 0} {
		if got := myGroupsCursorString(value); got != "" {
			t.Fatalf("cursor %v = %q", value, got)
		}
	}
}

func TestCrossPlatformCoverageAtMeAdditionalEdges(t *testing.T) {
	run := func(t *testing.T, caller *chatMessagesPagingCaller, args ...string) (map[string]any, error) {
		t.Helper()
		helpers.InitDeps(caller)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs(append([]string{"chat", "+at-me"}, args...))
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

	t.Run("single page terminal and read failure", func(t *testing.T) {
		envelope, err := run(t, &chatMessagesPagingCaller{responses: []string{
			`{"result":{"conversationMessagesList":[],"hasMore":false}}`,
		}})
		if err != nil || atMePagination(t, envelope)["endpoint_exhausted"] != true {
			t.Fatalf("envelope=%#v err=%v", envelope, err)
		}
		envelope, err = run(t, &chatMessagesPagingCaller{failAt: 1})
		if err != nil || envelope["outcome"] != "failure" {
			t.Fatalf("first read failure envelope=%#v err=%v", envelope, err)
		}
	})

	t.Run("all-page read failures", func(t *testing.T) {
		envelope, err := run(t, &chatMessagesPagingCaller{failAt: 1}, "--page-all")
		if err != nil || envelope["outcome"] != "failure" {
			t.Fatalf("first all-page failure envelope=%#v err=%v", envelope, err)
		}
		envelope, err = run(t, &chatMessagesPagingCaller{
			responses: []string{`{"result":{"conversationMessagesList":[{"messages":[{"openMessageId":"m1"}]}],"hasMore":true,"nextCursor":"next"}}`},
			failAt:    2,
		}, "--page-all")
		if err != nil || envelope["outcome"] != "partial_failure" {
			t.Fatalf("partial envelope=%#v err=%v", envelope, err)
		}
	})

	for _, tc := range []struct {
		name          string
		responses     []string
		args          []string
		wantOutcome   string
		wantExhausted *bool
	}{
		{name: "unknown pagination", responses: []string{`{"result":{"conversationMessagesList":[]}}`}, args: []string{"--page-all"}, wantOutcome: "partial_failure"},
		{name: "page limit", responses: []string{`{"result":{"conversationMessagesList":[{"messages":[{"openMessageId":"m1"}]}],"hasMore":true,"nextCursor":"next"}}`}, args: []string{"--page-all", "--page-limit", "1"}, wantOutcome: "success"},
		{name: "empty cursor defaults", responses: []string{`{"result":{"conversationMessagesList":[],"hasMore":false}}`}, args: []string{"--cursor=", "--page-all"}, wantOutcome: "success"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envelope, err := run(t, &chatMessagesPagingCaller{responses: tc.responses}, tc.args...)
			if err != nil || envelope["outcome"] != tc.wantOutcome {
				t.Fatalf("envelope=%#v err=%v", envelope, err)
			}
			if tc.name == "page limit" && atMePagination(t, envelope)["next_token"] != "next" {
				t.Fatalf("continuation envelope=%#v", envelope)
			}
			if tc.name == "empty cursor defaults" && atMePagination(t, envelope)["endpoint_exhausted"] != true {
				t.Fatalf("terminal envelope=%#v", envelope)
			}
		})
	}

	t.Run("output errors propagate", func(t *testing.T) {
		helpers.InitDeps(&chatMessagesPagingCaller{responses: []string{
			`{"result":{"conversationMessagesList":[],"hasMore":false}}`,
		}})
		root := newPlatformCoverageRoot()
		root.SetOut(chatMessagesFailWriter{})
		root.SetArgs([]string{"chat", "+at-me"})
		if err := root.Execute(); err == nil {
			t.Fatal("output error was swallowed")
		}
	})

	for _, value := range []any{nil, "", "0", "<nil>"} {
		if got := atMeCursorString(value); got != "" {
			t.Fatalf("cursor %v = %q", value, got)
		}
	}
}
