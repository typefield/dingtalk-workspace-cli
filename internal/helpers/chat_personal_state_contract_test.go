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

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestChatPersonalStateCommandsPublishReviewedContracts(t *testing.T) {
	tests := []struct {
		path       []string
		canonical  string
		rpc        string
		risk       string
		parameters map[string]bool
	}{
		{[]string{"mark-unread"}, "chat.mark_conversation_unread", "mark_conversation_unread", "low", map[string]bool{"conversation-id": true}},
		{[]string{"clear-red-point"}, "chat.clear_conversation_red_point", "clear_conversation_red_point", "low", map[string]bool{"conversation-id": true}},
		{[]string{"clear-all-red-point"}, "chat.clear_all_red_point", "clear_all_red_point", "medium", map[string]bool{}},
		{[]string{"mark-read"}, "chat.mark_message_read", "mark_message_read", "low", map[string]bool{"conversation-id": true, "message-id": true}},
		{[]string{"group", "update-alias"}, "chat.update_user_group_alias", "update_user_group_alias", "low", map[string]bool{"alias-title": true, "group": true}},
		{[]string{"hide"}, "chat.hide_conversation", "hide_conversation", "low", map[string]bool{"conversation-id": true}},
		{[]string{"mute-at-all"}, "chat.update_at_all_notification_off", "update_at_all_notification_off", "low", map[string]bool{"conversation-id": true, "off": false}},
		{[]string{"mute-red-envelope"}, "chat.update_red_env_notification_off", "update_red_env_notification_off", "low", map[string]bool{"conversation-id": true, "off": false}},
	}

	root := newChatCommand()
	for _, test := range tests {
		t.Run(strings.Join(test.path, "_"), func(t *testing.T) {
			cmd, remaining, err := root.Find(test.path)
			if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
				t.Fatalf("chat %v is not an exact runnable leaf: cmd=%v remaining=%v err=%v", test.path, cmd, remaining, err)
			}
			final, ok := contractfinal.RuntimeContractFinal(cmd)
			if !ok {
				t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
			}
			wantCLIPath := "chat " + strings.Join(test.path, " ")
			if final.Identity == nil || final.Identity.CanonicalPath != test.canonical || final.Identity.CLIPath != wantCLIPath || final.Identity.PrimaryCLIPath != wantCLIPath {
				t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
			}
			if final.Safety == nil || final.Safety.Effect != "write" || final.Safety.Risk != test.risk || final.Safety.Confirmation != "not_required" || final.Safety.Idempotency != "idempotent" {
				t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
			}
			if final.Interface == nil || final.Interface.Mode != "composite" || final.Interface.Availability != "available" || final.Interface.Ref != nil || !strings.Contains(final.Interface.Reason, "im/"+test.rpc) || !strings.Contains(final.Interface.Reason, "absent from the pinned MCP metadata snapshot") {
				t.Fatalf("%s interface=%#v", cmd.CommandPath(), final.Interface)
			}
			if final.DryRun == nil || final.DryRun.PreviewKind != contract.DryRunPreviewRequest || final.DryRun.RemoteReads {
				t.Fatalf("%s dry_run=%#v", cmd.CommandPath(), final.DryRun)
			}
			if final.Selection == nil || strings.TrimSpace(final.Selection.AgentSummary) == "" || len(final.Selection.UseWhen) == 0 || len(final.Selection.AvoidWhen) == 0 || len(final.Selection.Examples) == 0 {
				t.Fatalf("%s selection=%#v", cmd.CommandPath(), final.Selection)
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

func TestChatPersonalStateCommandsRouteCanonicalArgumentsOnce(t *testing.T) {
	tests := []struct {
		name string
		args []string
		rpc  string
		want map[string]any
	}{
		{"mark_unread", []string{"mark-unread", "--conversation-id", " cid-1 "}, "mark_conversation_unread", map[string]any{"openConversationId": "cid-1"}},
		{"clear_red", []string{"clear-red-point", "--conversation-id", " cid-2 "}, "clear_conversation_red_point", map[string]any{"openConversationId": "cid-2"}},
		{"clear_all_red", []string{"clear-all-red-point"}, "clear_all_red_point", map[string]any{}},
		{"mark_read", []string{"mark-read", "--conversation-id", " cid-3 ", "--message-id", " msg-3 "}, "mark_message_read", map[string]any{"openConversationId": "cid-3", "openMessageId": "msg-3"}},
		{"update_alias", []string{"group", "update-alias", "--group", " cid-4 ", "--alias-title", " 项目群 "}, "update_user_group_alias", map[string]any{"openConversationId": "cid-4", "aliasTitle": "项目群"}},
		{"hide", []string{"hide", "--conversation-id", " cid-5 "}, "hide_conversation", map[string]any{"openConversationId": "cid-5"}},
		{"mute_at_all", []string{"mute-at-all", "--conversation-id", "cid-6"}, "update_at_all_notification_off", map[string]any{"openConversationId": "cid-6", "mute": true}},
		{"unmute_at_all", []string{"mute-at-all", "--conversation-id", "cid-6", "--off"}, "update_at_all_notification_off", map[string]any{"openConversationId": "cid-6", "mute": false}},
		{"mute_red_envelope", []string{"mute-red-envelope", "--conversation-id", "cid-7"}, "update_red_env_notification_off", map[string]any{"openConversationId": "cid-7", "mute": true}},
		{"unmute_red_envelope", []string{"mute-red-envelope", "--conversation-id", "cid-7", "--off"}, "update_red_env_notification_off", map[string]any{"openConversationId": "cid-7", "mute": false}},
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

func TestChatPersonalStateCompatibilityAliasesRemainExecutableButUnpublished(t *testing.T) {
	tests := []struct {
		args []string
		rpc  string
		want map[string]any
	}{
		{[]string{"mark-unread", "--id", "cid-alias"}, "mark_conversation_unread", map[string]any{"openConversationId": "cid-alias"}},
		{[]string{"mute-red-envelope", "--chat", "cid-chat"}, "update_red_env_notification_off", map[string]any{"openConversationId": "cid-chat", "mute": true}},
	}
	for _, test := range tests {
		caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{}`}}}
		if err := runChatCoverageCommand(t, caller, test.args...); err != nil {
			t.Fatalf("execute %v: %v", test.args, err)
		}
		if caller.calls != 1 || caller.tool != test.rpc || !reflect.DeepEqual(caller.args, test.want) {
			t.Fatalf("execute %v call=(count=%d tool=%q args=%#v)", test.args, caller.calls, caller.tool, caller.args)
		}
	}
}

func TestChatPersonalStateCommandsRejectBlankInputsBeforeRemoteCall(t *testing.T) {
	for _, args := range [][]string{
		{"mark-unread", "--conversation-id", " "},
		{"clear-red-point", "--conversation-id", " "},
		{"mark-read", "--conversation-id", " ", "--message-id", "msg-1"},
		{"mark-read", "--conversation-id", "cid-1", "--message-id", " "},
		{"group", "update-alias", "--group", " ", "--alias-title", "项目群"},
		{"group", "update-alias", "--group", "cid-1", "--alias-title", " "},
		{"hide", "--conversation-id", " "},
		{"mute-at-all", "--conversation-id", " "},
		{"mute-red-envelope", "--conversation-id", " "},
	} {
		caller := &scriptedToolCaller{}
		err := runChatCoverageCommand(t, caller, args...)
		if err == nil || apperrors.ExitCode(err) != apperrors.ExitCodeValidation {
			t.Fatalf("execute %v error=%v exit=%d, want validation", args, err, apperrors.ExitCode(err))
		}
		if caller.calls != 0 {
			t.Fatalf("execute %v made %d remote calls before validation", args, caller.calls)
		}
	}
}

func TestChatPersonalStateCommandsDryRunDoesNotCallRemote(t *testing.T) {
	for _, args := range [][]string{
		{"mark-unread", "--conversation-id", "cid-1"},
		{"clear-red-point", "--conversation-id", "cid-1"},
		{"clear-all-red-point"},
		{"mark-read", "--conversation-id", "cid-1", "--message-id", "msg-1"},
		{"group", "update-alias", "--group", "cid-1", "--alias-title", "项目群"},
		{"hide", "--conversation-id", "cid-1"},
		{"mute-at-all", "--conversation-id", "cid-1"},
		{"mute-red-envelope", "--conversation-id", "cid-1"},
	} {
		caller := &scriptedToolCaller{dry: true}
		if err := runChatCoverageCommand(t, caller, args...); err != nil {
			t.Fatalf("dry-run %v: %v", args, err)
		}
		if caller.calls != 0 {
			t.Fatalf("dry-run %v made %d remote calls", args, caller.calls)
		}
	}
}
