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
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestChatSharedContentWritesPublishReviewedContracts(t *testing.T) {
	tests := []struct {
		name        string
		path        []string
		canonical   string
		rpc         string
		idempotency string
		parameters  map[string]bool
	}{
		{"set_top", []string{"message", "set-top-msg"}, "chat.set_top_message", "set_top_message", "idempotent", map[string]bool{"msg-id": true, "open-conversation-id": true}},
		{"unset_top", []string{"message", "unset-top-msg"}, "chat.unset_top_message", "unset_top_message", "idempotent", map[string]bool{"msg-id": true, "open-conversation-id": true}},
		{"notice_create", []string{"group", "notice", "create"}, "chat.create_group_notice", "create_group_notice", "non_idempotent", map[string]bool{"content": true, "group": true, "run-at": false, "send-ding": false, "sticky": false}},
		{"notice_edit", []string{"group", "notice", "edit"}, "chat.edit_group_notice", "edit_group_notice", "unknown", map[string]bool{"content": true, "group": true, "notice-id": true, "send-ding": false, "sticky": false}},
		{"share_invite", []string{"group", "share-invite"}, "chat.share_group_invite_url", "share_group_invite_url", "non_idempotent", map[string]bool{"expires-seconds": false, "receiver": false, "source": true, "target": false, "uuid": false}},
	}

	root := newChatCommand()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
			if final.Safety == nil || final.Safety.Effect != "write" || final.Safety.Risk != "medium" || final.Safety.Confirmation != "user_required" || final.Safety.Idempotency != test.idempotency {
				t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
			}
			if !HasContractConfirmSafety(cmd) {
				t.Fatalf("%s does not install the declared confirmation gate", cmd.CommandPath())
			}
			if final.Interface == nil || final.Interface.Mode != contract.InterfaceModeMCP || final.Interface.Availability != contract.InterfaceAvailable || final.Interface.Ref == nil || final.Interface.Ref.ProductID != "im" || final.Interface.Ref.RPCName != test.rpc {
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

func TestChatSharedContentWritesRouteNormalizedArgumentsExactlyOnce(t *testing.T) {
	createContent := "  # 公告\n内容  "
	editContent := "  更新后的完整正文\n  "
	tests := []struct {
		name string
		args []string
		rpc  string
		want map[string]any
	}{
		{"set_top", []string{"message", "set-top-msg", "--open-conversation-id", " cid-1 ", "--msg-id", " msg-1 ", "--yes"}, "set_top_message", map[string]any{"openConversationId": "cid-1", "openMessageId": "msg-1"}},
		{"unset_top", []string{"message", "unset-top-msg", "--open-conversation-id", " cid-2 ", "--msg-id", " msg-2 ", "--yes"}, "unset_top_message", map[string]any{"openConversationId": "cid-2", "openMessageId": "msg-2"}},
		{"notice_create", []string{"group", "notice", "create", "--group", " cid-3 ", "--content", createContent, "--sticky", "--send-ding", "--run-at", " 2026-07-03T09:00:00+08:00 ", "--yes"}, "create_group_notice", map[string]any{"openConversationId": "cid-3", "content": createContent, "sticky": true, "sendDing": true, "scheduled": true, "runAtText": "2026-07-03T09:00:00+08:00"}},
		{"notice_edit", []string{"group", "notice", "edit", "--group", " cid-4 ", "--notice-id", " notice-4 ", "--content", editContent, "--sticky", "--send-ding", "--yes"}, "edit_group_notice", map[string]any{"openConversationId": "cid-4", "dataId": "notice-4", "content": editContent, "sticky": true, "sendDing": true}},
		{"share_target", []string{"group", "share-invite", "--source", " source-5 ", "--target", " target-5 ", "--expires-seconds", "0", "--uuid", " key-5 ", "--yes"}, "share_group_invite_url", map[string]any{"sourceOpenConversationId": "source-5", "targetOpenConversationId": "target-5", "expiresSeconds": int64(0), "uuid": "key-5"}},
		{"share_receiver", []string{"group", "share-invite", "--source", " source-6 ", "--receiver", " user-6 ", "--yes"}, "share_group_invite_url", map[string]any{"sourceOpenConversationId": "source-6", "receiverOpenDingTalkId": "user-6"}},
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

func TestChatSharedContentWritesRejectInvalidInputBeforeConfirmationOrRemoteCall(t *testing.T) {
	for _, args := range [][]string{
		{"message", "set-top-msg", "--open-conversation-id", " ", "--msg-id", "msg-1"},
		{"message", "set-top-msg", "--open-conversation-id", "cid-1", "--msg-id", " "},
		{"message", "unset-top-msg", "--open-conversation-id", " ", "--msg-id", "msg-1"},
		{"group", "notice", "create", "--group", " ", "--content", "公告"},
		{"group", "notice", "create", "--group", "cid-1", "--content", " \n\t "},
		{"group", "notice", "create", "--group", "cid-1", "--content", "公告", "--run-at", "tomorrow morning"},
		{"group", "notice", "edit", "--group", "cid-1", "--notice-id", " ", "--content", "公告"},
		{"group", "notice", "edit", "--group", "cid-1", "--notice-id", "notice-1", "--content", " "},
		{"group", "share-invite", "--source", " ", "--target", "target-1"},
		{"group", "share-invite", "--source", "source-1"},
		{"group", "share-invite", "--source", "source-1", "--target", "target-1", "--receiver", "user-1"},
		{"group", "share-invite", "--source", "source-1", "--target", "target-1", "--expires-seconds", "-1"},
	} {
		caller := &scriptedToolCaller{}
		err := runChatCoverageCommand(t, caller, args...)
		if err == nil || apperrors.ExitCode(err) != apperrors.ExitCodeValidation {
			t.Fatalf("execute %v error=%T %v exit=%d, want typed validation", args, err, err, apperrors.ExitCode(err))
		}
		var typed *apperrors.Error
		if !errors.As(err, &typed) || typed.Reason == "confirmation_required" {
			t.Fatalf("execute %v error=%T %v, want input validation before confirmation", args, err, err)
		}
		if caller.calls != 0 {
			t.Fatalf("execute %v made %d remote calls before validation", args, caller.calls)
		}
	}
}

func TestChatSharedContentWritesRequireConfirmation(t *testing.T) {
	for _, args := range [][]string{
		{"message", "set-top-msg", "--open-conversation-id", "cid-1", "--msg-id", "msg-1"},
		{"message", "unset-top-msg", "--open-conversation-id", "cid-1", "--msg-id", "msg-1"},
		{"group", "notice", "create", "--group", "cid-1", "--content", "公告"},
		{"group", "notice", "edit", "--group", "cid-1", "--notice-id", "notice-1", "--content", "完整正文"},
		{"group", "share-invite", "--source", "source-1", "--target", "target-1"},
	} {
		caller := &scriptedToolCaller{}
		err := runChatCoverageCommand(t, caller, args...)
		var typed *apperrors.Error
		if !errors.As(err, &typed) || typed.Category != apperrors.CategoryValidation || typed.Reason != "confirmation_required" {
			t.Fatalf("execute %v error=%T %v, want confirmation_required", args, err, err)
		}
		if caller.calls != 0 {
			t.Fatalf("execute %v made %d remote calls before confirmation", args, caller.calls)
		}
	}
}

func TestChatSharedContentWritesDryRunNeverCallsRemoteOrRequiresYes(t *testing.T) {
	for _, args := range [][]string{
		{"message", "set-top-msg", "--open-conversation-id", "cid-1", "--msg-id", "msg-1", "--dry-run"},
		{"message", "unset-top-msg", "--open-conversation-id", "cid-1", "--msg-id", "msg-1", "--dry-run"},
		{"group", "notice", "create", "--group", "cid-1", "--content", "公告", "--dry-run"},
		{"group", "notice", "edit", "--group", "cid-1", "--notice-id", "notice-1", "--content", "完整正文", "--dry-run"},
		{"group", "share-invite", "--source", "source-1", "--target", "target-1", "--dry-run"},
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
