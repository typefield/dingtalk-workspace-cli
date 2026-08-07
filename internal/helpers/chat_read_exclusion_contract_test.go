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

package helpers

import (
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

type reviewedChatReadCase struct {
	path          []string
	canonicalPath string
	rpc           string
	parameters    map[string]bool
}

var reviewedChatReadCases = []reviewedChatReadCase{
	{[]string{"group", "list-all"}, "chat.list_my_groups_pagination", "list_my_groups_pagination", map[string]bool{"cursor": false, "limit": false}},
	{[]string{"group", "list-join-validations"}, "chat.list_apply_join_group_records", "list_apply_join_group_records", map[string]bool{"cursor": false, "limit": false}},
	{[]string{"group", "members", "list-by-ids"}, "chat.list_group_member_by_ids", "list_group_member_by_ids", map[string]bool{"id": true, "users": true}},
	{[]string{"group", "notice", "get"}, "chat.get_group_notice", "get_group_notice", map[string]bool{"group": true, "notice-id": true}},
	{[]string{"group", "notice", "list"}, "chat.list_group_notices", "list_group_notices", map[string]bool{"cursor": false, "group": true, "limit": false, "scheduled": false}},
	{[]string{"list-all-conversations"}, "chat.list_all_conversations", "list_all_conversations", map[string]bool{"cursor": false, "exclude-muted": false, "limit": false}},
	{[]string{"message", "list-emotion-replies"}, "chat.list_message_emotion_replies", "list_message_emotion_replies", map[string]bool{"msg-ids": true}},
	{[]string{"text", "translate"}, "chat.translate", "translate", map[string]bool{"query": true, "to": true}},
}

func TestChatFormerReadExclusionsPublishReviewedAgentContracts(t *testing.T) {
	root := newChatCommand()
	for _, test := range reviewedChatReadCases {
		t.Run(strings.Join(test.path, "_"), func(t *testing.T) {
			cmd, remaining, err := root.Find(test.path)
			if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
				t.Fatalf("%v is not a runnable exact leaf: cmd=%v remaining=%v err=%v", test.path, cmd, remaining, err)
			}
			final, ok := contractfinal.RuntimeContractFinal(cmd)
			if !ok {
				t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
			}
			wantCLIPath := "chat " + strings.Join(test.path, " ")
			if final.Identity == nil || final.Identity.CanonicalPath != test.canonicalPath || final.Identity.CLIPath != wantCLIPath || final.Identity.PrimaryCLIPath != wantCLIPath {
				t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
			}
			if final.Interface == nil || final.Interface.Mode != "composite" || final.Interface.Availability != "available" || final.Interface.Ref != nil || !strings.Contains(final.Interface.Reason, "im/"+test.rpc) || !strings.Contains(final.Interface.Reason, "absent from the pinned MCP metadata snapshot") {
				t.Fatalf("%s interface=%#v", cmd.CommandPath(), final.Interface)
			}
			if final.Safety == nil || final.Safety.Effect != "read" || final.Safety.Risk != "low" || final.Safety.Confirmation != "not_required" || final.Safety.Idempotency != "idempotent" {
				t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
			}
			if final.Selection == nil || strings.TrimSpace(final.Selection.AgentSummary) == "" || len(final.Selection.UseWhen) == 0 || len(final.Selection.AvoidWhen) == 0 || len(final.Selection.Examples) == 0 {
				t.Fatalf("%s selection=%#v", cmd.CommandPath(), final.Selection)
			}
			if final.DryRun != nil {
				t.Fatalf("%s dry_run=%#v, read command must not advertise a write preview", cmd.CommandPath(), final.DryRun)
			}
			gotParameters := make(map[string]bool, len(final.Parameters))
			for _, parameter := range final.Parameters {
				gotParameters[parameter.Name] = parameter.Required != nil && *parameter.Required
			}
			if !reflect.DeepEqual(gotParameters, test.parameters) {
				t.Fatalf("%s parameters=%#v, want %#v", cmd.CommandPath(), final.Parameters, test.parameters)
			}
		})
	}
}

func TestChatFormerReadExclusionsRouteOnceWithCanonicalArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		rpc  string
		want map[string]any
	}{
		{"group_list_all", []string{"group", "list-all", "--limit", "25", "--cursor", "next-groups"}, "list_my_groups_pagination", map[string]any{"limit": 25, "cursor": "next-groups"}},
		{"join_validations", []string{"group", "list-join-validations", "--limit", "30", "--cursor", "next-joins"}, "list_apply_join_group_records", map[string]any{"limit": 30, "cursor": "next-joins"}},
		{"members_by_ids", []string{"group", "members", "list-by-ids", "--id", "cid-1", "--users", "D1,D2"}, "list_group_member_by_ids", map[string]any{"openConversationId": "cid-1", "memberOpenDingTalkIds": []string{"D1", "D2"}}},
		{"notice_get", []string{"group", "notice", "get", "--group", "cid-1", "--notice-id", "notice-1"}, "get_group_notice", map[string]any{"openConversationId": "cid-1", "dataId": "notice-1"}},
		{"notice_list", []string{"group", "notice", "list", "--group", "cid-1", "--limit", "20", "--cursor", "next-notices", "--scheduled"}, "list_group_notices", map[string]any{"openConversationId": "cid-1", "limit": 20, "cursor": "next-notices", "scheduled": true}},
		{"all_conversations", []string{"list-all-conversations", "--limit", "50", "--cursor", "7", "--exclude-muted"}, "list_all_conversations", map[string]any{"limit": 50, "cursor": int64(7), "excludeMuted": true}},
		{"emotion_replies", []string{"message", "list-emotion-replies", "--msg-ids", "m1,m2"}, "list_message_emotion_replies", map[string]any{"openMessageIds": []string{"m1", "m2"}}},
		{"translate", []string{"text", "translate", "--query", "hello", "--to", "zh_CN"}, "translate", map[string]any{"query": "hello", "to": "zh_CN"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{}`}}}
			if err := runChatCoverageCommand(t, caller, test.args...); err != nil {
				t.Fatalf("execute %v: %v", test.args, err)
			}
			if caller.calls != 1 || caller.server != "im" || caller.tool != test.rpc || !reflect.DeepEqual(caller.args, test.want) {
				t.Fatalf("call=(count=%d server=%q tool=%q args=%#v), want im/%s %#v", caller.calls, caller.server, caller.tool, caller.args, test.rpc, test.want)
			}
		})
	}
}

func TestChatFormerReadExclusionsRejectInvalidInputsBeforeRemoteCall(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"group_limit", []string{"group", "list-all", "--limit", "201"}},
		{"join_limit", []string{"group", "list-join-validations", "--limit", "51"}},
		{"notice_limit", []string{"group", "notice", "list", "--group", "cid-1", "--limit", "101"}},
		{"conversation_limit", []string{"list-all-conversations", "--limit", "0"}},
		{"empty_member_ids", []string{"group", "members", "list-by-ids", "--id", "cid-1", "--users", ","}},
		{"unsupported_language", []string{"text", "translate", "--query", "hello", "--to", "xx_XX"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := runChatCoverageCommand(t, caller, test.args...)
			if err == nil || apperrors.ExitCode(err) != apperrors.ExitCodeValidation {
				t.Fatalf("execute %v error=%v exit=%d, want validation", test.args, err, apperrors.ExitCode(err))
			}
			if caller.calls != 0 {
				t.Fatalf("execute %v made %d remote calls before validation", test.args, caller.calls)
			}
		})
	}
}
