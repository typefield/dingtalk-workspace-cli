// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package chat

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

func TestFlagListPaginationRolloutIsUnifiedActive(t *testing.T) {
	if FlagList.OutputRollout != frameworkoutput.RolloutUnifiedActive {
		t.Fatalf("flag-list rollout = %q, want unified_active", FlagList.OutputRollout)
	}
}

func runFlagListUnifiedResult(t *testing.T, fake *larkAlignmentCaller, args ...string) (map[string]any, int) {
	t.Helper()
	helpers.InitDeps(fake)
	cmd := corecmd.New(shortcutcore.FromShortcut(FlagList))
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

func TestFlagListUnifiedPromotionEvidence(t *testing.T) {
	t.Run("page limit exposes a resumable endpoint", func(t *testing.T) {
		envelope, exitCode := runFlagListUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/list_message_favorites": `{"result":{"items":[{"openMessageId":"msg-1"}],"hasMore":true,"nextCursor":9}}`,
		}}, "--format", "json", "--page-all", "--page-limit", "1")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		pagination := envelope["meta"].(map[string]any)["pagination"].(map[string]any)
		if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "9" || pagination["pages"] != float64(1) {
			t.Fatalf("pagination=%#v", pagination)
		}
	})

	t.Run("unknown legacy boundary does not assert endpoint completion", func(t *testing.T) {
		envelope, exitCode := runFlagListUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/list_message_favorites": `{"result":{"items":[]}}`,
		}}, "--format", "json")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		if _, present := envelope["meta"]; present {
			t.Fatalf("unknown pagination must omit meta.pagination: %#v", envelope)
		}
		data := envelope["data"].(map[string]any)
		if data["pagination_known"] != false {
			t.Fatalf("unknown pagination data=%#v", data)
		}
	})

	t.Run("later read failure retains completed page", func(t *testing.T) {
		envelope, exitCode := runFlagListUnifiedResult(t, &larkAlignmentCaller{
			sequenceResponses: map[string][]string{
				"im/list_message_favorites": {`{"result":{"items":[{"openMessageId":"msg-1"}],"hasMore":true,"nextCursor":9}}`},
			},
			failProductToolAt: map[string]int{"im/list_message_favorites": 2},
		}, "--format", "json", "--page-all")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		data := envelope["data"].(map[string]any)
		if data["total"] != float64(2) || len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("partial data=%#v", data)
		}
	})
}

func TestCrossPlatformCoverageFlagListDryRunStopsBeforeRead(t *testing.T) {
	fake := &larkAlignmentCaller{dryRun: true}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{
		"chat", "+flag-list", "--page-size", "20", "--cursor", "0", "--dry-run",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("flag-list dry-run made lower calls: %#v", fake.calls)
	}
}

func TestCrossPlatformCoverageFlagListPageAllUsesNumericCursorAndDeduplicates(t *testing.T) {
	fake := &larkAlignmentCaller{sequenceResponses: map[string][]string{
		"im/list_message_favorites": {
			`{"result":{"items":[{"openMessageId":"msg-1","openConversationId":"cid-1","summary":"一"}],"hasMore":true,"nextCursor":7}}`,
			`{"result":{"items":[{"openMessageId":"msg-1","openConversationId":"cid-1","summary":"重复"},{"openMessageId":"msg-2","openConversationId":"cid-2","summary":"二"}],"hasMore":false}}`,
		},
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+flag-list", "--page-size", "1", "--page-all", "--page-limit", "5"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 2 || fake.calls[0].args["cursor"] != 0 || fake.calls[0].args["size"] != "1" || fake.calls[1].args["cursor"] != 7 {
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
	if data["count"] != float64(2) || len(data["items"].([]any)) != 2 {
		t.Fatalf("data = %#v", data)
	}
	pagination := payload["meta"].(map[string]any)["pagination"].(map[string]any)
	if pagination["endpoint_exhausted"] != true || pagination["pages"] != float64(2) || pagination["items"] != float64(2) {
		t.Fatalf("pagination = %#v", pagination)
	}
}

func TestCrossPlatformCoverageFlagListPageTokenAndPageLimit(t *testing.T) {
	fake := &larkAlignmentCaller{responses: map[string]string{
		"im/list_message_favorites": `{"result":{"items":[{"openMessageId":"msg-2"}],"hasMore":true,"nextCursor":9}}`,
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+flag-list", "--page-token", "7", "--page-all", "--page-limit", "1"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 || fake.calls[0].args["cursor"] != 7 {
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
	if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "9" || pagination["pages"] != float64(1) {
		t.Fatalf("pagination = %#v", pagination)
	}
}

func TestCrossPlatformCoverageFlagListFailureModes(t *testing.T) {
	t.Run("later read failure keeps partial result", func(t *testing.T) {
		fake := &larkAlignmentCaller{
			sequenceResponses: map[string][]string{
				"im/list_message_favorites": {`{"result":{"items":[{"openMessageId":"msg-1"}],"hasMore":true,"nextCursor":7}}`},
			},
			failProductToolAt: map[string]int{"im/list_message_favorites": 2},
		}
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"chat", "+flag-list", "--page-all"})
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
		if data["total"] != float64(2) || len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("partial data = %#v", data)
		}
	})

	t.Run("stalled numeric cursor fails closed", func(t *testing.T) {
		fake := &larkAlignmentCaller{responses: map[string]string{
			"im/list_message_favorites": `{"result":{"items":[{"openMessageId":"msg-1"}],"hasMore":true,"nextCursor":7}}`,
		}}
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"chat", "+flag-list", "--cursor", "7", "--page-all"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var envelope map[string]any
		if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope["outcome"] != "partial_failure" {
			t.Fatalf("stalled cursor envelope = %#v", envelope)
		}
	})

	t.Run("full legacy page without pagination fails closed", func(t *testing.T) {
		fake := &larkAlignmentCaller{responses: map[string]string{
			"im/list_message_favorites": `{"result":{"items":[{"openMessageId":"msg-1"}]}}`,
		}}
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs([]string{"chat", "+flag-list", "--page-size", "1"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var envelope map[string]any
		if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope["outcome"] != "partial_failure" {
			t.Fatalf("unknown boundary envelope = %#v", envelope)
		}
	})
}

func TestCrossPlatformCoverageFlagListPaginationValidation(t *testing.T) {
	for _, args := range [][]string{
		{"--page-size", "0"},
		{"--page-size", "31"},
		{"--size", "31"},
		{"--page-token", "not-a-number"},
		{"--page-limit", "2"},
		{"--page-all", "--page-limit", "0"},
		{"--page-all", "--page-limit", "501"},
		{"--cursor", "1", "--page-token", "2"},
	} {
		helpers.InitDeps(&larkAlignmentCaller{})
		root := newPlatformCoverageRoot()
		root.SetArgs(append([]string{"chat", "+flag-list"}, args...))
		if err := root.Execute(); err == nil {
			t.Fatalf("invalid args succeeded: %v", args)
		}
	}
	if _, err := flagListNextCursor(float64(1.5)); err == nil {
		t.Fatal("fractional cursor unexpectedly accepted")
	}
	if _, err := flagListNextCursor(struct{}{}); err == nil {
		t.Fatal("unsupported cursor unexpectedly accepted")
	}
}

func TestCrossPlatformCoverageFlagListAdditionalEdges(t *testing.T) {
	run := func(t *testing.T, fake *larkAlignmentCaller, args ...string) (map[string]any, error) {
		t.Helper()
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetArgs(append([]string{"chat", "+flag-list"}, args...))
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

	for _, token := range []string{"", "0"} {
		payload, err := run(t, &larkAlignmentCaller{responses: map[string]string{
			"im/list_message_favorites": `{"result":{"items":[],"hasMore":false}}`,
		}}, "--page-token="+token)
		pagination := payload["meta"].(map[string]any)["pagination"].(map[string]any)
		if err != nil || payload["outcome"] != "success" || pagination["endpoint_exhausted"] != true {
			t.Fatalf("token=%q payload=%#v err=%v", token, payload, err)
		}
	}

	t.Run("first read failure", func(t *testing.T) {
		_, err := run(t, &larkAlignmentCaller{failProductToolAt: map[string]int{"im/list_message_favorites": 1}})
		if err == nil {
			t.Fatal("first read failure unexpectedly succeeded")
		}
	})

	t.Run("cursor without hasMore continues", func(t *testing.T) {
		payload, err := run(t, &larkAlignmentCaller{sequenceResponses: map[string][]string{
			"im/list_message_favorites": {
				`{"result":{"items":[{"openMessageId":"m1"}],"nextCursor":2}}`,
				`{"result":{"items":[],"hasMore":false}}`,
			},
		}}, "--page-all")
		pagination := payload["meta"].(map[string]any)["pagination"].(map[string]any)
		if err != nil || payload["outcome"] != "success" || pagination["pages"] != float64(2) || pagination["endpoint_exhausted"] != true {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	for _, tc := range []struct {
		name        string
		response    string
		wantOutcome string
	}{
		{name: "single page continuation", response: `{"result":{"items":[{"openMessageId":"m1"}],"hasMore":true,"nextCursor":2}}`, wantOutcome: "success"},
		{name: "missing continuation", response: `{"result":{"items":[{"openMessageId":"m1"}],"hasMore":true}}`, wantOutcome: "partial_failure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := run(t, &larkAlignmentCaller{responses: map[string]string{"im/list_message_favorites": tc.response}})
			if err != nil || payload["outcome"] != tc.wantOutcome {
				t.Fatalf("payload=%#v err=%v", payload, err)
			}
			if tc.wantOutcome == "success" {
				pagination := payload["meta"].(map[string]any)["pagination"].(map[string]any)
				if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "2" {
					t.Fatalf("pagination=%#v", pagination)
				}
			}
		})
	}

	t.Run("output errors propagate", func(t *testing.T) {
		fake := &larkAlignmentCaller{responses: map[string]string{
			"im/list_message_favorites": `{"result":{"items":[],"hasMore":false}}`,
		}}
		helpers.InitDeps(fake)
		root := newPlatformCoverageRoot()
		root.SetOut(chatOutputErrorWriter{err: errors.New("fixture output")})
		root.SetArgs([]string{"chat", "+flag-list"})
		if err := root.Execute(); err == nil {
			t.Fatal("output error was swallowed")
		}
	})

	if got := flagListItems(nil); len(got) != 0 {
		t.Fatalf("nil items = %#v", got)
	}
	if got := flagListItems(map[string]any{"result": map[string]any{"items": []any{"invalid", map[string]any{"id": "m1"}}}}); len(got) != 1 {
		t.Fatalf("projected items = %#v", got)
	}
	if got := firstNonEmptyMapString(map[string]any{}, "missing"); got != "" {
		t.Fatalf("missing identity = %q", got)
	}
}

func TestCrossPlatformCoverageFlagListCursorTypeEdges(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		ok    bool
	}{
		{name: "nil", value: nil, ok: true},
		{name: "int", value: int(1), ok: true},
		{name: "negative int", value: int(-1)},
		{name: "int64", value: int64(2), ok: true},
		{name: "negative int64", value: int64(-1)},
		{name: "float64", value: float64(3), ok: true},
		{name: "negative float64", value: float64(-1)},
		{name: "json number", value: json.Number("4"), ok: true},
		{name: "invalid json number", value: json.Number("bad")},
		{name: "empty string", value: "", ok: true},
		{name: "string", value: "5", ok: true},
		{name: "invalid string", value: "bad"},
		{name: "unsupported", value: struct{}{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := flagListNextCursor(tc.value)
			if (err == nil) != tc.ok {
				t.Fatalf("value=%#v err=%v", tc.value, err)
			}
		})
	}
}
