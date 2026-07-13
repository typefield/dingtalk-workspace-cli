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

package usage

import (
	"os"
	"testing"
)

func TestEnabledToggle(t *testing.T) {
	for _, v := range []string{"", "0", "false", "off", "NO", "garbage"} {
		t.Setenv("DWS_USAGE_TRACKING", v)
		if Enabled() {
			t.Errorf("DWS_USAGE_TRACKING=%q should keep tracking OFF (opt-in default)", v)
		}
	}
	for _, v := range []string{"1", "true", "on", "YES"} {
		t.Setenv("DWS_USAGE_TRACKING", v)
		if !Enabled() {
			t.Errorf("DWS_USAGE_TRACKING=%q should enable tracking", v)
		}
	}
}

func TestSampleArgsRedaction(t *testing.T) {
	args := map[string]any{
		"open_conversation_id": "cid_abc123",                   // ID-like → kept
		"page":                 20,                             // number → kept
		"has_read":             true,                           // bool → kept
		"text":                 "hi there 你好",                  // sensitive key → dropped
		"keyword":              "cid_abc123",                   // sensitive key → dropped even if ID-like
		"note":                 "a long free text with spaces", // whitespace → dropped
		"name":                 "Alice",                        // short content → dropped
		"fileName":             "roadmap.md",                   // short content → dropped
		"originalText":         "Q2",                           // short content → dropped
		"replacedText":         "第二季度",                         // short content → dropped
		"clientId":             "oauth-client",                 // credential metadata → dropped
		"authCode":             "one-time-code",                // credential → dropped
		"amount":               1000,                           // unknown numeric user data → dropped
		"tags":                 []string{"a", "b"},             // composite → dropped
	}
	got := sampleArgs(args)
	wantKept := map[string]string{"open_conversation_id": "cid_abc123", "page": "20", "has_read": "true"}
	for k, v := range wantKept {
		if got[k] != v {
			t.Errorf("expected %s=%q kept, got %q", k, v, got[k])
		}
	}
	for _, k := range []string{
		"text", "keyword", "note", "name", "fileName", "originalText",
		"replacedText", "clientId", "authCode", "amount", "tags",
	} {
		if _, ok := got[k]; ok {
			t.Errorf("expected %s to be redacted/dropped, but it was recorded", k)
		}
	}
}

func TestAggregateRequiresSampleOnEveryOccurrenceForFixedArg(t *testing.T) {
	recs := []Record{
		{Product: "chat", Tool: "send", ArgKeys: []string{"conversationId"}, SampleArgs: map[string]string{"conversationId": "cid_x"}},
		{Product: "chat", Tool: "send", ArgKeys: []string{"conversationId"}},
	}
	groups := Aggregate(recs)
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	if _, fixed := groups[0].FixedArgs["conversationId"]; fixed {
		t.Fatalf("partially sampled value must not become fixed: %#v", groups[0].FixedArgs)
	}
}

func TestAppendAndAggregate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DWS_CONFIG_DIR", dir)
	t.Setenv("DWS_USAGE_TRACKING", "1")

	// Same shape, same fixed conversation id → should group with a fixed arg.
	for i := 0; i < 3; i++ {
		Append("chat", "send_message", map[string]any{
			"open_conversation_id": "cid_x", "text": "msg" + string(rune('0'+i)),
		}, true, false)
	}
	// Different tool.
	Append("todo", "get_user_todos_in_current_org", map[string]any{"pageNum": "1"}, true, false)

	// Dry-run must be skipped.
	Append("chat", "send_message", map[string]any{"open_conversation_id": "cid_x"}, true, true)

	recs, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 4 {
		t.Fatalf("expected 4 records (dry-run skipped), got %d", len(recs))
	}

	groups := Aggregate(recs)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	top := groups[0]
	if top.Tool != "send_message" || top.Count != 3 {
		t.Fatalf("top group = %s x%d, want send_message x3", top.Tool, top.Count)
	}
	if top.FixedArgs["open_conversation_id"] != "cid_x" {
		t.Errorf("expected fixed open_conversation_id=cid_x, got %v", top.FixedArgs)
	}
	if _, leaked := top.FixedArgs["text"]; leaked {
		t.Error("free-text 'text' must never appear in fixed args")
	}

	if err := Purge(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(LogPath()); !os.IsNotExist(err) {
		t.Error("Purge should remove the log")
	}
}

func TestDisabledSkipsWrite(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DWS_CONFIG_DIR", dir)
	t.Setenv("DWS_USAGE_TRACKING", "0")
	Append("chat", "send_message", map[string]any{"x": "y"}, true, false)
	if recs, _ := Read(); len(recs) != 0 {
		t.Errorf("disabled tracking must not write, got %d records", len(recs))
	}
}
