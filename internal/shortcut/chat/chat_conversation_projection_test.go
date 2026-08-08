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

package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/jsonutil"
	frameworkoutput "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	shortcutcore "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestConversationListPaginationRolloutStartsWithDualValidation(t *testing.T) {
	if ConversationList.OutputRollout != frameworkoutput.RolloutDualValidate {
		t.Fatalf("conversation-list rollout = %q, want dual_validate", ConversationList.OutputRollout)
	}
}

func runConversationListUnifiedResult(t *testing.T, fake *larkAlignmentCaller, args ...string) (map[string]any, int) {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := ConversationList
	declaration.OutputRollout = frameworkoutput.RolloutUnifiedActive
	cmd := corecmd.New(shortcutcore.FromShortcut(declaration))
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

func TestConversationListUnifiedPromotionEvidence(t *testing.T) {
	t.Run("local page limit remains resumable success", func(t *testing.T) {
		envelope, exitCode := runConversationListUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/list_all_conversations": `{"result":{"conversationList":[{"openConversationId":"cid-1","title":"一"}],"hasMore":true,"nextCursor":2}}`,
		}}, "--page-all", "--page-limit", "1")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		pagination := envelope["meta"].(map[string]any)["pagination"].(map[string]any)
		if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "2" {
			t.Fatalf("pagination = %#v", pagination)
		}
	})

	t.Run("unknown list container fails closed", func(t *testing.T) {
		envelope, exitCode := runConversationListUnifiedResult(t, &larkAlignmentCaller{responses: map[string]string{
			"im/list_all_conversations": `{"result":{"unexpected":true},"hasMore":false}`,
		}})
		if exitCode != 1 || envelope["ok"] != false || envelope["outcome"] != "failure" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		failure := envelope["error"].(map[string]any)
		if failure["subtype"] != "projection_unknown" {
			t.Fatalf("failure = %#v", failure)
		}
	})

	t.Run("later read failure preserves first page", func(t *testing.T) {
		envelope, exitCode := runConversationListUnifiedResult(t, &larkAlignmentCaller{
			sequenceResponses: map[string][]string{
				"im/list_all_conversations": {`{"result":{"conversationList":[{"openConversationId":"cid-1","title":"一"}],"hasMore":true,"nextCursor":2}}`},
			},
			failProductToolAt: map[string]int{"im/list_all_conversations": 2},
		}, "--page-all")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("envelope=%#v exit=%d", envelope, exitCode)
		}
		data := envelope["data"].(map[string]any)
		if len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("partial data = %#v", data)
		}
	})
}

func TestCrossPlatformCoverageConversationListTopProjectNormalizesAndFiltersType(t *testing.T) {
	data := map[string]any{
		"result": map[string]any{
			"items": []any{
				map[string]any{
					"openConversationId": "cid-direct",
					"title":              "张三",
					"singleChat":         true,
				},
				map[string]any{
					"openConversationId": "cid-group",
					"title":              "项目群",
					"singleChat":         false,
				},
				map[string]any{
					"openConversationId": "cid-legacy",
					"title":              "旧版单聊",
					"conversationType":   "P2P",
				},
				map[string]any{
					"openConversationId": "cid-unknown",
					"title":              "未知类型",
				},
			},
		},
	}

	all := conversationListTopProject(data)
	if len(all) != 4 {
		t.Fatalf("all conversations = %#v", all)
	}
	if got := all[0]["conversationType"]; got != "direct" {
		t.Errorf("singleChat=true type = %#v, want direct", got)
	}
	if got := all[1]["conversationType"]; got != "group" {
		t.Errorf("singleChat=false type = %#v, want group", got)
	}
	if got := all[2]["conversationType"]; got != "direct" {
		t.Errorf("legacy P2P type = %#v, want direct", got)
	}
	if _, ok := all[3]["conversationType"]; ok {
		t.Errorf("unknown type was fabricated: %#v", all[3])
	}

	if got, want := conversationListTopFilter(all, "group"), []map[string]any{all[1]}; !reflect.DeepEqual(got, want) {
		t.Errorf("group filter = %#v, want %#v", got, want)
	}
	if got, want := conversationListTopFilter(all, "direct"), []map[string]any{all[0], all[2]}; !reflect.DeepEqual(got, want) {
		t.Errorf("direct filter = %#v, want %#v", got, want)
	}
	if got := conversationListTopFilter(all, "all"); !reflect.DeepEqual(got, all) {
		t.Errorf("all filter changed rows: %#v", got)
	}
}

func TestCrossPlatformCoverageConversationListTopRejectsInvalidType(t *testing.T) {
	fake := &platformCoverageCaller{}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{"chat", "+conversation-list-top", "--type", "bot"})
	if err := root.Execute(); err == nil {
		t.Fatal("invalid --type unexpectedly succeeded")
	}
	if fake.tool != "" {
		t.Fatalf("invalid --type reached lower tool %s/%s", fake.product, fake.tool)
	}
}

func TestCrossPlatformCoverageConversationListProjectUnwrapsGatewayTuple(t *testing.T) {
	data := map[string]any{
		"result": []any{
			[]any{map[string]any{"openConversationId": "cid-1", "title": "项目群"}},
			float64(2),
			true,
		},
	}
	if got := conversationListProject(data); len(got) != 1 || got[0]["openConversationId"] != "cid-1" {
		t.Fatalf("conversation tuple projection = %#v", got)
	}
	if got := conversationListTopProject(data); len(got) != 1 || got[0]["openConversationId"] != "cid-1" {
		t.Fatalf("top tuple projection = %#v", got)
	}
}

func TestCrossPlatformCoverageConversationListPageAllFollowsTypedCursor(t *testing.T) {
	fake := &larkAlignmentCaller{sequenceResponses: map[string][]string{
		"im/list_all_conversations": {
			`{"result":{"conversationList":[{"openConversationId":"cid-1","title":"一"}],"hasMore":true,"nextCursor":2}}`,
			`{"result":{"conversationList":[{"openConversationId":"cid-2","title":"二"}],"hasMore":false}}`,
		},
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+conversation-list", "--page-all"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 2 || fake.calls[1].args["cursor"] != int64(2) {
		t.Fatalf("calls = %#v", fake.calls)
	}
	want, err := jsonutil.MarshalIndent(map[string]any{
		"count": 2,
		"conversations": []map[string]any{
			{"openConversationId": "cid-1", "conversationName": "一"},
			{"openConversationId": "cid-2", "conversationName": "二"},
		},
		"pagesFetched":    2,
		"complete":        true,
		"hasMore":         false,
		"nextCursor":      int64(2),
		"paginationKnown": true,
		"failedCount":     0,
		"failures":        []map[string]any{},
		"partial":         false,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != string(want)+"\n" {
		t.Fatalf("dual_validate changed legacy bytes:\n%s\nwant:\n%s", got, string(want))
	}
}

func TestCrossPlatformCoverageConversationListSinglePagePreservesTypedCursor(t *testing.T) {
	fake := &larkAlignmentCaller{responses: map[string]string{
		"im/list_all_conversations": `{"result":{"conversationList":[{"openConversationId":"cid-1","title":"一"}],"hasMore":true,"nextCursor":2}}`,
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+conversation-list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls = %#v, want exactly one page", fake.calls)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["hasMore"] != true || payload["nextCursor"] != float64(2) {
		t.Fatalf("pagination payload = %#v", payload)
	}
}

func TestCrossPlatformCoverageConversationListDeduplicatesStableIDs(t *testing.T) {
	fake := &larkAlignmentCaller{responses: map[string]string{
		"im/list_all_conversations": `{"result":{"conversationList":[{"openConversationId":"cid-1","title":"一"},{"openConversationId":"cid-1","title":"重复"}],"hasMore":false}}`,
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+conversation-list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["count"] != float64(1) {
		t.Fatalf("deduplicated payload = %#v", payload)
	}
}

func TestConversationListStrictProjectionDistinguishesEmptyFromUnknown(t *testing.T) {
	if rows, err := conversationListProjectStrict(map[string]any{"result": map[string]any{"conversationList": []any{}}}); err != nil || len(rows) != 0 {
		t.Fatalf("known empty projection rows=%#v err=%v", rows, err)
	}
	for name, data := range map[string]map[string]any{
		"unknown container": {"result": map[string]any{"unexpected": true}},
		"invalid entry":     {"result": map[string]any{"conversationList": []any{"bad"}}},
		"missing stable id": {"result": map[string]any{"conversationList": []any{map[string]any{"title": "无ID"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := conversationListProjectStrict(data); err == nil {
				t.Fatal("strict projection unexpectedly succeeded")
			}
		})
	}
}
