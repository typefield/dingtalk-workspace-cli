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
	stderrors "errors"
	"math"
	"os"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	frameworkoutput "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

// TestProjectChatMessageExpandsForwarded guards that a forwarded chat record
// ("聊天记录") exposes its nested messages under "forwarded" instead of
// collapsing to the lossy top-level "[卡片]" summary, recursing through nested
// forwards, and that the string-"null" sender is nulled out. The per-field
// behaviour (sender/text/encryption) is covered in the chatmsg package tests.
func TestCrossPlatformCoverageProjectChatMessageExpandsForwarded(t *testing.T) {
	row := projectChatMessage(map[string]any{
		"sender":             "hugozhu",
		"openMessageId":      "msg-top",
		"openConversationId": "cid-top",
		"openConvThreadId":   "thread-top",
		"msgType":            "text",
		"content":            "hugozhu与opencode-agent的聊天记录\nopencode-agent:[卡片]",
		"createTime":         "2026-07-20 21:41:21",
		"emotionReplyList": []any{
			map[string]any{"emoji": "赞", "replyUsers": []any{"D1"}},
		},
		"forwardMessages": []any{
			map[string]any{"sender": "null", "content": "读下冬翔发给我的最近两条消息", "createTime": "2026-07-20 09:30:33"},
			map[string]any{"sender": "冬翔", "content": "W29 工作总结", "createTime": "2026-07-19 23:35:40",
				// nested forward inside a forward — must expand recursively.
				"forwardMessages": []any{
					map[string]any{"sender": "念晨", "content": "收到", "createTime": "2026-07-19 23:36:00"},
				},
			},
		},
	})

	if row["sender"] != "hugozhu" {
		t.Fatalf("top sender = %v, want hugozhu", row["sender"])
	}
	if row["messageId"] != "msg-top" || row["conversationId"] != "cid-top" || row["messageType"] != "text" {
		t.Errorf("stable identity = %#v", row)
	}
	if row["threadId"] != "thread-top" {
		t.Errorf("thread identity = %#v", row)
	}
	if reactions, ok := row["reactions"].(map[string]any); !ok || len(reactions) == 0 {
		t.Errorf("reactions = %#v", row["reactions"])
	}
	forwarded, ok := row["forwarded"].([]map[string]any)
	if !ok || len(forwarded) != 2 {
		t.Fatalf("forwarded = %#v, want 2 entries", row["forwarded"])
	}
	if forwarded[0]["sender"] != nil {
		t.Errorf("forwarded[0].sender = %v, want nil (string \"null\")", forwarded[0]["sender"])
	}
	if forwarded[0]["text"] != "读下冬翔发给我的最近两条消息" {
		t.Errorf("forwarded[0].text = %v", forwarded[0]["text"])
	}
	nested, ok := forwarded[1]["forwarded"].([]map[string]any)
	if !ok || len(nested) != 1 || nested[0]["sender"] != "念晨" {
		t.Errorf("nested forwarded = %#v, want 1 entry from 念晨", forwarded[1]["forwarded"])
	}

	// A plain message must not grow a "forwarded" key.
	plain := projectChatMessage(map[string]any{"sender": "念晨", "content": "hi", "createTime": "t"})
	if _, has := plain["forwarded"]; has {
		t.Errorf("plain message unexpectedly has forwarded key: %#v", plain)
	}

	withoutReactions := projectChatMessageWithReactions(map[string]any{
		"content": "hi",
		"emotionReplyList": []any{
			map[string]any{"emoji": "赞", "replyUsers": []any{"D1"}},
		},
	}, false)
	if _, has := withoutReactions["reactions"]; has {
		t.Errorf("no-reactions projection leaked reactions: %#v", withoutReactions)
	}
}

type chatMessagesPagingCaller struct {
	responses []string
	args      []map[string]any
	failAt    int
}

type chatMessagesFailWriter struct{}

func (chatMessagesFailWriter) Write([]byte) (int, error) {
	return 0, stderrors.New("fixture output failure")
}

func decodeChatMessagesEnvelope(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode unified chat-messages envelope: %v\n%s", err, raw)
	}
	if _, present := envelope["contract_version"]; present {
		t.Fatalf("chat-messages must not expose a protocol selector: %#v", envelope)
	}
	return envelope
}

func chatMessagesSuccessData(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	if envelope["ok"] != true || envelope["outcome"] != "success" {
		t.Fatalf("expected unified success, got %#v", envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("unified success missing data: %#v", envelope)
	}
	return data
}

func chatMessagesPagination(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	meta, ok := envelope["meta"].(map[string]any)
	if !ok {
		t.Fatalf("unified result missing meta: %#v", envelope)
	}
	pagination, ok := meta["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("unified result missing pagination: %#v", envelope)
	}
	return pagination
}

func chatMessagesPartialData(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	if envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
		t.Fatalf("expected unified partial_failure, got %#v", envelope)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("unified partial_failure missing data: %#v", envelope)
	}
	return data
}

// atMe uses the same active-result shape as the other migrated message reads,
// but keeps a dedicated helper so the old compatibility tests cannot
// accidentally reintroduce legacy complete/nextCursor assertions.
func atMeSuccessData(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	return chatMessagesSuccessData(t, envelope)
}

func atMePartialData(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	return chatMessagesPartialData(t, envelope)
}

func atMePagination(t *testing.T, envelope map[string]any) map[string]any {
	t.Helper()
	return chatMessagesPagination(t, envelope)
}

func (c *chatMessagesPagingCaller) CallTool(
	_ context.Context,
	_, _ string,
	args map[string]any,
) (*edition.ToolResult, error) {
	copied := make(map[string]any, len(args))
	for key, value := range args {
		copied[key] = value
	}
	c.args = append(c.args, copied)
	index := len(c.args) - 1
	if c.failAt > 0 && len(c.args) == c.failAt {
		return nil, stderrors.New("fixture read failure")
	}
	if index >= len(c.responses) {
		index = len(c.responses) - 1
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{
		Type: "text",
		Text: c.responses[index],
	}}}, nil
}

func (c *chatMessagesPagingCaller) CallReadTool(
	ctx context.Context,
	product, tool string,
	args map[string]any,
) (*edition.ToolResult, error) {
	return c.CallTool(ctx, product, tool, args)
}

func (*chatMessagesPagingCaller) Format() string { return "json" }
func (*chatMessagesPagingCaller) DryRun() bool   { return false }
func (*chatMessagesPagingCaller) Fields() string { return "" }
func (*chatMessagesPagingCaller) JQ() string     { return "" }

func TestCrossPlatformCoverageChatMessagesPageAllUsesTypedBoundaryAndDeduplicates(t *testing.T) {
	const cursorMillis int64 = 1767225600123
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"nextCursor":1767225600123,"messages":[{"openMessageId":"m2","createTime":"2026-01-02 00:00:00"},{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"}]}}`,
		`{"result":{"hasMore":false,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"},{"openMessageId":"m0","createTime":"2025-12-31 00:00:00"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid",
		"--time", "2026-01-03 00:00:00", "--page-all", "--page-limit", "5",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	wantBoundary := time.UnixMilli(cursorMillis).UTC().Format(time.RFC3339Nano)
	if len(caller.args) != 2 || caller.args[1]["time"] != wantBoundary {
		t.Fatalf("pagination calls = %#v", caller.args)
	}
	envelope := decodeChatMessagesEnvelope(t, output.Bytes())
	payload := chatMessagesSuccessData(t, envelope)
	pagination := chatMessagesPagination(t, envelope)
	if payload["count"] != float64(3) || pagination["endpoint_exhausted"] != true ||
		pagination["pages"] != float64(2) || pagination["items"] != float64(4) {
		t.Fatalf("all-page envelope = %#v", envelope)
	}
}

func TestCrossPlatformCoverageChatMessagesMillisecondCursorDoesNotSkipSameSecondRows(t *testing.T) {
	const cursorMillis int64 = 1785919699136
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"nextCursor":1785919699136,"messages":[{"openMessageId":"m4","createTime":"2026-08-05 16:48:19"},{"openMessageId":"m3","createTime":"2026-08-05 16:48:19"},{"openMessageId":"m2","createTime":"2026-08-05 16:48:19"}]}}`,
		`{"result":{"hasMore":false,"messages":[{"openMessageId":"m1","createTime":"2026-08-05 16:48:19"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid", "--time", "2026-08-05 16:49:00",
		"--page-size", "3", "--page-all", "--page-limit", "5",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	wantBoundary := time.UnixMilli(cursorMillis).UTC().Format(time.RFC3339Nano)
	if len(caller.args) != 2 || caller.args[1]["time"] != wantBoundary {
		t.Fatalf("pagination calls = %#v", caller.args)
	}
	envelope := decodeChatMessagesEnvelope(t, output.Bytes())
	payload := chatMessagesSuccessData(t, envelope)
	pagination := chatMessagesPagination(t, envelope)
	if payload["count"] != float64(4) || pagination["endpoint_exhausted"] != true || pagination["pages"] != float64(2) {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestCrossPlatformCoverageChatMessagesDescendingRangeStopsAtInclusiveStart(t *testing.T) {
	const cursorMillis int64 = 1767312000456
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"nextCursor":1767312000456,"messages":[{"openMessageId":"m3","createTime":"2026-01-03 00:00:00"},{"openMessageId":"m2","createTime":"2026-01-02 00:00:00"}]}}`,
		`{"result":{"hasMore":true,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 12:00:00"},{"openMessageId":"m0","createTime":"2026-01-01 11:59:59"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid",
		"--start", "2026-01-01 12:00:00", "--end", "2026-01-04 00:00:00",
		"--order", "desc", "--page-all", "--page-limit", "5",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	wantBoundary := time.UnixMilli(cursorMillis).UTC().Format(time.RFC3339Nano)
	if len(caller.args) != 2 || caller.args[0]["time"] != "2026-01-04 00:00:00" ||
		caller.args[0]["forward"] != false || caller.args[1]["time"] != wantBoundary {
		t.Fatalf("range calls = %#v", caller.args)
	}
	envelope := decodeChatMessagesEnvelope(t, output.Bytes())
	payload := chatMessagesSuccessData(t, envelope)
	if payload["count"] != float64(3) || payload["pagination_known"] != false {
		t.Fatalf("range envelope = %#v", envelope)
	}
	if meta, _ := envelope["meta"].(map[string]any); meta != nil && meta["pagination"] != nil {
		t.Fatalf("range completion must not claim endpoint exhaustion: %#v", envelope)
	}
	messages := payload["messages"].([]any)
	if messages[0].(map[string]any)["messageId"] != "m3" ||
		messages[2].(map[string]any)["messageId"] != "m1" {
		t.Fatalf("descending messages = %#v", messages)
	}
	rangeMeta := payload["queryRange"].(map[string]any)
	if rangeMeta["semantics"] != "[start,end)" || rangeMeta["order"] != "desc" {
		t.Fatalf("queryRange = %#v", rangeMeta)
	}
}

func TestCrossPlatformCoverageChatMessagesAscendingRangeStopsAtExclusiveEnd(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 12:00:00"},{"openMessageId":"m2","createTime":"2026-01-02 00:00:00"},{"openMessageId":"m-end","createTime":"2026-01-03 00:00:00"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid",
		"--start-time", "2026-01-01 12:00:00", "--end-time", "2026-01-03 00:00:00",
		"--sort", "asc", "--page-all",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.args) != 1 || caller.args[0]["time"] != "2026-01-01 12:00:00" ||
		caller.args[0]["forward"] != true {
		t.Fatalf("ascending call = %#v", caller.args)
	}
	envelope := decodeChatMessagesEnvelope(t, output.Bytes())
	payload := chatMessagesSuccessData(t, envelope)
	if payload["count"] != float64(2) || payload["pagination_known"] != false {
		t.Fatalf("ascending envelope = %#v", envelope)
	}
	if meta, _ := envelope["meta"].(map[string]any); meta != nil && meta["pagination"] != nil {
		t.Fatalf("range completion must not claim endpoint exhaustion: %#v", envelope)
	}
	messages := payload["messages"].([]any)
	if messages[0].(map[string]any)["messageId"] != "m1" ||
		messages[1].(map[string]any)["messageId"] != "m2" {
		t.Fatalf("ascending messages = %#v", messages)
	}
}

func TestCrossPlatformCoverageChatMessagesFirstReadFailureSkipsOptionalSenderResolution(t *testing.T) {
	caller := &chatMessagesPagingCaller{
		responses: []string{`{"result":[]}`},
		failAt:    1,
	}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid", "--page-all",
		"--sender-query", "张三",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("active first-page failure must emit its typed envelope: %v", err)
	}
	envelope := decodeChatMessagesEnvelope(t, output.Bytes())
	if envelope["ok"] != false || envelope["outcome"] != "failure" {
		t.Fatalf("first-page failure envelope=%#v", envelope)
	}
	if len(caller.args) != 1 {
		t.Fatalf("calls after first read failure = %#v, want only the message read", caller.args)
	}
}

func TestCrossPlatformCoverageChatMessagesRangeValidationStopsBeforeRead(t *testing.T) {
	for _, args := range [][]string{
		{"--start", "2026-01-02", "--end", "2026-01-01"},
		{"--end", "2026-01-02", "--order", "asc"},
		{"--time", "2026-01-02", "--start", "2026-01-01"},
		{"--direction", "newer", "--start", "2026-01-01"},
		{"--start", "2026-01-01", "--start-time", "2026-01-01"},
	} {
		caller := &chatMessagesPagingCaller{responses: []string{`{"result":{"hasMore":false,"messages":[]}}`}}
		helpers.InitDeps(caller)
		root := newPlatformCoverageRoot()
		root.SetArgs(append([]string{"chat", "+chat-messages", "--conversation-id", "cid"}, args...))
		if err := root.Execute(); err == nil {
			t.Errorf("invalid range succeeded: %v", args)
		}
		if len(caller.args) != 0 {
			t.Errorf("invalid range reached message read: %v calls=%#v", args, caller.args)
		}
	}
}

func TestCrossPlatformCoverageChatMessagesPageAllPublishesBoundedContinuation(t *testing.T) {
	const cursorMillis int64 = 1767225600123
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"nextCursor":1767225600123,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid",
		"--page-all", "--page-limit", "1",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	envelope := decodeChatMessagesEnvelope(t, output.Bytes())
	payload := chatMessagesSuccessData(t, envelope)
	pagination := chatMessagesPagination(t, envelope)
	if payload["count"] != float64(1) || pagination["endpoint_exhausted"] != false ||
		pagination["next_token"] != "1767225600123" || pagination["pages"] != float64(1) {
		t.Fatalf("bounded continuation envelope = %#v", envelope)
	}
}

func TestCrossPlatformCoverageChatMessagesPageAllFailsClosedOnStalledBoundary(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"messages":[{"openMessageId":"m1"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+chat-messages", "--conversation-id", "cid", "--page-all"})
	if err := root.Execute(); err != nil {
		t.Fatalf("active stalled boundary must emit partial_failure: %v", err)
	}
	payload := chatMessagesPartialData(t, decodeChatMessagesEnvelope(t, output.Bytes()))
	if len(payload["succeeded"].([]any)) != 1 || len(payload["failed"].([]any)) != 1 {
		t.Fatalf("stalled partial data = %#v", payload)
	}
	failure := payload["failed"].([]any)[0].(map[string]any)["error"].(map[string]any)
	if failure["subtype"] != "pagination_inconsistent" || failure["stage"] != "pagination_projection" {
		t.Fatalf("stalled partial failure = %#v", failure)
	}
}

func TestCrossPlatformCoverageChatMessagesFailedPageDoesNotExportPartialLedger(t *testing.T) {
	t.Chdir(t.TempDir())
	caller := &chatMessagesPagingCaller{
		responses: []string{
			`{"result":{"hasMore":true,"nextCursor":1767225600123,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"}]}}`,
		},
		failAt: 2,
	}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid", "--page-all",
		"--output", "exports/partial.json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("active later-page failure must emit partial_failure: %v", err)
	}
	if _, statErr := os.Lstat("exports/partial.json"); !os.IsNotExist(statErr) {
		t.Fatalf("partial export exists: %v", statErr)
	}
	payload := chatMessagesPartialData(t, decodeChatMessagesEnvelope(t, output.Bytes()))
	if len(payload["succeeded"].([]any)) != 1 || len(payload["failed"].([]any)) != 1 {
		t.Fatalf("failure ledger = %#v", payload)
	}
}

func TestCrossPlatformCoverageChatMessagesFailureLedgerOutputErrorIsNonZero(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"messages":[{"openMessageId":"m1"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	root.SetOut(chatMessagesFailWriter{})
	root.SetArgs([]string{"chat", "+chat-messages", "--conversation-id", "cid", "--page-all"})
	if err := root.Execute(); err == nil || err.Error() != "fixture output failure" {
		t.Fatalf("error = %v", err)
	}
}

func TestCrossPlatformCoverageChatMessagesExportIsAtomicAndNoClobber(t *testing.T) {
	t.Chdir(t.TempDir())
	newCaller := func() *chatMessagesPagingCaller {
		return &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":false,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"}]}}`,
		}}
	}
	run := func(overwrite bool) (map[string]any, error) {
		helpers.InitDeps(newCaller())
		root := newPlatformCoverageRoot()
		var output bytes.Buffer
		root.SetOut(&output)
		args := []string{"chat", "+chat-messages", "--conversation-id", "cid", "--page-all", "--output", "exports/messages.json"}
		if overwrite {
			args = append(args, "--overwrite")
		}
		root.SetArgs(args)
		err := root.Execute()
		return decodeChatMessagesEnvelope(t, output.Bytes()), err
	}
	first, err := run(false)
	if err != nil {
		t.Fatal(err)
	}
	chatMessagesSuccessData(t, first)
	raw, err := os.ReadFile("exports/messages.json")
	if err != nil {
		t.Fatal(err)
	}
	var exported map[string]any
	if err := json.Unmarshal(raw, &exported); err != nil {
		t.Fatal(err)
	}
	if exported["complete"] != true || exported["count"] != float64(1) {
		t.Fatalf("exported ledger = %#v", exported)
	}
	second, err := run(false)
	if err != nil {
		t.Fatalf("existing export must report a partial result, not return a raw error: %v", err)
	}
	partial := chatMessagesPartialData(t, second)
	if len(partial["succeeded"].([]any)) != 1 || len(partial["failed"].([]any)) != 1 {
		t.Fatalf("existing export partial=%#v", partial)
	}
	third, err := run(true)
	if err != nil {
		t.Fatal(err)
	}
	chatMessagesSuccessData(t, third)
}

func TestCrossPlatformCoverageChatMessagesExportRejectsNonJSONPlaceholder(t *testing.T) {
	t.Chdir(t.TempDir())
	helpers.InitDeps(&chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":false,"messages":[]}}`,
	}})
	root := newPlatformCoverageRoot()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid", "--output", "{}",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("non-JSON placeholder output unexpectedly succeeded")
	}
	if _, err := os.Lstat("{}"); !os.IsNotExist(err) {
		t.Fatalf("placeholder output was created: %v", err)
	}
}

func chatMessagesRuntimeForTest(t *testing.T, values map[string]string) *shortcut.RuntimeContext {
	t.Helper()
	// These direct collector tests exercise the historical payload helper. Keep
	// that unit isolated from the production command's active result outlet;
	// external contract coverage is asserted above through the real root.
	declaration := ChatMessages
	declaration.OutputRollout = frameworkoutput.RolloutLegacyOnly
	cmd := corecmd.New(shortcut.FromShortcut(declaration))
	for name, value := range values {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set --%s=%q: %v", name, value, err)
		}
	}
	return shortcut.RuntimeContextForTest(cmd, ChatMessages)
}

func TestCrossPlatformCoverageChatMessagesAdditionalValidationAndHelpers(t *testing.T) {
	for _, values := range []map[string]string{
		{"time": "2026-01-01", "start": "2026-01-01"},
		{"direction": "older", "start": "2026-01-01"},
	} {
		if err := validateChatMessages(chatMessagesRuntimeForTest(t, values)); err == nil {
			t.Fatalf("direct validation unexpectedly accepted %#v", values)
		}
	}
	if _, err := resolveChatMessagesRequest(chatMessagesRuntimeForTest(t, map[string]string{
		"conversation-id": "cid", "start": "not-a-time",
	})); err == nil {
		t.Fatal("invalid direct request range unexpectedly succeeded")
	}

	helpers.InitDeps(&platformCoverageCaller{
		contactSearchResult: `{"result":[{"name":"测试用户甲"}]}`,
	})
	filter := resolveOptionalChatMessagesSenderFilter(chatMessagesRuntimeForTest(t, map[string]string{
		"sender-query": "测试用户甲",
	}))
	if filter.applied || filter.failure == nil {
		t.Fatalf("identity-free sender resolution = %#v", filter)
	}

	for _, tc := range []struct {
		name      string
		value     any
		wantError bool
	}{
		{name: "int", value: int(1234)},
		{name: "int32", value: int32(1234)},
		{name: "int64", value: int64(1234)},
		{name: "float32", value: float32(1024)},
		{name: "float64", value: float64(1234)},
		{name: "string", value: "1234"},
		{name: "float32 nan", value: float32(math.NaN()), wantError: true},
		{name: "float32 infinity", value: float32(math.Inf(1)), wantError: true},
		{name: "float32 max int64 boundary", value: float32(math.MaxInt64), wantError: true},
		{name: "float64 max int64 boundary", value: float64(math.MaxInt64), wantError: true},
		{name: "float64 fractional", value: 1.5, wantError: true},
		{name: "bad string", value: "opaque", wantError: true},
		{name: "missing", value: nil, wantError: true},
		{name: "zero", value: 0, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := chatMessagesNextCursorBoundary(tc.value)
			if (err != nil) != tc.wantError {
				t.Fatalf("value=%#v err=%v wantError=%v", tc.value, err, tc.wantError)
			}
		})
	}
}

func TestCrossPlatformCoverageChatMessagesAdditionalCollectionEdges(t *testing.T) {
	runtimeWith := func(t *testing.T, caller *chatMessagesPagingCaller, values map[string]string) *shortcut.RuntimeContext {
		t.Helper()
		helpers.InitDeps(caller)
		return chatMessagesRuntimeForTest(t, values)
	}
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, dingTalkMessageLocation)
	configuredRange := chatMessageTimeRange{configured: true, start: &start, order: "desc"}

	t.Run("single-page range failures and terminal", func(t *testing.T) {
		caller := &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":true,"nextCursor":1234,"messages":[{"openMessageId":"bad","createTime":"invalid"}]}}`,
		}}
		payload, _, err := collectOneChatMessagesPage(
			runtimeWith(t, caller, nil),
			chatMessagesRequest{tool: "list_conversation_message_v2", params: map[string]any{}, direction: "older", timeRange: configuredRange},
		)
		if err != nil || payload["stopReason"] != "time_filter_error" || payload["queryRange"] == nil {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}

		caller = &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":true,"nextCursor":1234,"messages":[{"openMessageId":"old","createTime":"2026-01-01 00:00:00"}]}}`,
		}}
		payload, _, err = collectOneChatMessagesPage(
			runtimeWith(t, caller, nil),
			chatMessagesRequest{tool: "list_conversation_message_v2", params: map[string]any{}, direction: "older", timeRange: configuredRange},
		)
		if err != nil || payload["complete"] != true || payload["stopReason"] != "range_start" {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	t.Run("default page size and time-filter failure", func(t *testing.T) {
		caller := &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":false,"messages":[]}}`,
		}}
		payload, _, err := collectAllChatMessages(
			runtimeWith(t, caller, nil),
			chatMessagesRequest{tool: "list_conversation_message_v2", params: map[string]any{}, direction: "older"},
		)
		if err != nil || payload["complete"] != true || caller.args[0]["limit"] != chatMessagesAllPageSize {
			t.Fatalf("payload=%#v calls=%#v err=%v", payload, caller.args, err)
		}

		caller = &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":false,"messages":[{"openMessageId":"bad","createTime":"invalid"}]}}`,
		}}
		payload, _, err = collectAllChatMessages(
			runtimeWith(t, caller, nil),
			chatMessagesRequest{tool: "list_conversation_message_v2", params: map[string]any{}, direction: "older", timeRange: configuredRange},
		)
		if err == nil || payload["stopReason"] != "time_filter_error" {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	t.Run("terminal result limit and unsafe continuation", func(t *testing.T) {
		caller := &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":true,"messages":[{"openMessageId":"m2","createTime":"2026-01-03 00:00:00"},{"openMessageId":"m1","createTime":"2026-01-02 00:00:00"},{"openMessageId":"old","createTime":"2026-01-01 00:00:00"}]}}`,
		}}
		payload, _, err := collectAllChatMessages(
			runtimeWith(t, caller, map[string]string{"max-results": "1"}),
			chatMessagesRequest{tool: "list_conversation_message_v2", params: map[string]any{}, direction: "older", timeRange: configuredRange},
		)
		if err != nil || payload["truncatedByResultLimit"] != true || payload["stopReason"] != "result_limit" {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}

		caller = &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":true,"messages":[{"openMessageId":"m1","createTime":"2026-01-03 00:00:00"}]}}`,
		}}
		payload, _, err = collectAllChatMessages(
			runtimeWith(t, caller, map[string]string{"max-results": "1"}),
			chatMessagesRequest{tool: "list_conversation_message_v2", params: map[string]any{}, direction: "older"},
		)
		if err == nil || payload["stopReason"] != "pagination_error" {
			t.Fatalf("payload=%#v err=%v", payload, err)
		}
	})

	t.Run("stalled cursor", func(t *testing.T) {
		caller := &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":true,"nextCursor":1234,"messages":[{"openMessageId":"m2"}]}}`,
			`{"result":{"hasMore":true,"nextCursor":1234,"messages":[{"openMessageId":"m1"}]}}`,
		}}
		payload, _, err := collectAllChatMessages(
			runtimeWith(t, caller, nil),
			chatMessagesRequest{tool: "list_conversation_message_v2", params: map[string]any{}, direction: "older"},
		)
		if err == nil || payload["stopReason"] != "pagination_error" || len(caller.args) != 2 {
			t.Fatalf("payload=%#v calls=%#v err=%v", payload, caller.args, err)
		}
	})

	t.Run("first failure ledger output error", func(t *testing.T) {
		helpers.InitDeps(&chatMessagesPagingCaller{failAt: 1})
		root := newPlatformCoverageRoot()
		root.SetOut(chatMessagesFailWriter{})
		root.SetArgs([]string{"chat", "+chat-messages", "--conversation-id", "cid", "--page-all"})
		if err := root.Execute(); err == nil || err.Error() != "fixture output failure" {
			t.Fatalf("error=%v", err)
		}
	})
}
