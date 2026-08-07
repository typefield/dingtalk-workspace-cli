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

func TestChatCategoryWriteCommandsPublishReviewedContracts(t *testing.T) {
	tests := []struct {
		path         string
		canonical    string
		rpc          string
		effect       string
		risk         string
		confirmation string
		idempotency  string
		parameters   map[string]bool
	}{
		{"create", "chat.create_conv_category", "create_conv_category", "write", "low", "not_required", "unknown", map[string]bool{"title": true}},
		{"delete", "chat.delete_conv_category", "delete_conv_category", "destructive", "high", "user_required", "unknown", map[string]bool{"category-id": true}},
		{"rename", "chat.rename_conv_category", "rename_conv_category", "write", "low", "not_required", "idempotent", map[string]bool{"category-id": true, "title": true}},
		{"add-conv", "chat.add_conv_to_categories", "add_conv_to_categories", "write", "low", "not_required", "idempotent", map[string]bool{"category-ids": true, "group": true}},
		{"remove-conv", "chat.remove_conv_from_categories", "remove_conv_from_categories", "write", "low", "not_required", "idempotent", map[string]bool{"category-ids": true, "group": true}},
	}

	root := newChatCommand()
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			cmd, remaining, err := root.Find([]string{"category", test.path})
			if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
				t.Fatalf("chat category %s is not an exact runnable leaf: cmd=%v remaining=%v err=%v", test.path, cmd, remaining, err)
			}
			final, ok := contractfinal.RuntimeContractFinal(cmd)
			if !ok {
				t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
			}
			wantCLIPath := "chat category " + test.path
			if final.Identity == nil || final.Identity.CanonicalPath != test.canonical || final.Identity.CLIPath != wantCLIPath || final.Identity.PrimaryCLIPath != wantCLIPath {
				t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
			}
			if final.Safety == nil || final.Safety.Effect != test.effect || final.Safety.Risk != test.risk || final.Safety.Confirmation != test.confirmation || final.Safety.Idempotency != test.idempotency {
				t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
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

func TestChatCategoryWriteCommandsRouteNormalizedArgumentsOnce(t *testing.T) {
	tests := []struct {
		name string
		args []string
		rpc  string
		want map[string]any
	}{
		{"create", []string{"category", "create", "--title", " 工作群 "}, "create_conv_category", map[string]any{"title": "工作群"}},
		{"delete", []string{"category", "delete", "--category-id", "7", "--yes"}, "delete_conv_category", map[string]any{"categoryId": int64(7)}},
		{"rename", []string{"category", "rename", "--category-id", "8", "--title", " 新名称 "}, "rename_conv_category", map[string]any{"categoryId": int64(8), "title": "新名称"}},
		{"add", []string{"category", "add-conv", "--group", " cid-add ", "--category-ids", "1, 2"}, "add_conv_to_categories", map[string]any{"openConversationId": "cid-add", "categoryIds": []int64{1, 2}}},
		{"remove", []string{"category", "remove-conv", "--group", " cid-remove ", "--category-ids", "3, 4"}, "remove_conv_from_categories", map[string]any{"openConversationId": "cid-remove", "categoryIds": []int64{3, 4}}},
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

func TestChatCategoryCompatibilityAliasesRemainExecutableButUnpublished(t *testing.T) {
	tests := []struct {
		args []string
		rpc  string
		want map[string]any
	}{
		{[]string{"category", "add-conv", "--conversation-id", "cid-alias", "--category-ids", "1"}, "add_conv_to_categories", map[string]any{"openConversationId": "cid-alias", "categoryIds": []int64{1}}},
		{[]string{"category", "remove-conv", "--id", "cid-id", "--category-ids", "2"}, "remove_conv_from_categories", map[string]any{"openConversationId": "cid-id", "categoryIds": []int64{2}}},
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

func TestChatCategoryWriteCommandsRejectInvalidInputBeforeRemoteCall(t *testing.T) {
	for _, args := range [][]string{
		{"category", "create", "--title", " "},
		{"category", "delete", "--category-id", "0", "--yes"},
		{"category", "delete", "--category-id", "-1", "--yes"},
		{"category", "rename", "--category-id", "-1", "--title", "新名称"},
		{"category", "rename", "--category-id", "1", "--title", " "},
		{"category", "add-conv", "--group", " ", "--category-ids", "1"},
		{"category", "add-conv", "--group", "cid", "--category-ids", " "},
		{"category", "add-conv", "--group", "cid", "--category-ids", "0,2"},
		{"category", "remove-conv", "--group", "cid", "--category-ids", "-1"},
		{"category", "remove-conv", "--group", "cid", "--category-ids", "not-a-number"},
	} {
		caller := &scriptedToolCaller{}
		err := runChatCoverageCommand(t, caller, args...)
		if err == nil || apperrors.ExitCode(err) != apperrors.ExitCodeValidation {
			t.Fatalf("execute %v error=%T %v exit=%d, want typed validation", args, err, err, apperrors.ExitCode(err))
		}
		if caller.calls != 0 {
			t.Fatalf("execute %v made %d remote calls before validation", args, caller.calls)
		}
	}
}

func TestChatCategoryDeleteRequiresConfirmationButDryRunDoesNot(t *testing.T) {
	caller := &scriptedToolCaller{}
	err := runChatCoverageCommand(t, caller, "category", "delete", "--category-id", "7")
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Category != apperrors.CategoryValidation || typed.Reason != "confirmation_required" {
		t.Fatalf("delete without --yes error=%T %v, want confirmation_required", err, err)
	}
	if caller.calls != 0 {
		t.Fatalf("delete without --yes made %d remote calls", caller.calls)
	}

	dryCaller := &scriptedToolCaller{dry: true}
	if err := runChatCoverageCommand(t, dryCaller, "category", "delete", "--category-id", "7", "--dry-run"); err != nil {
		t.Fatalf("delete dry-run without --yes: %v", err)
	}
	if dryCaller.calls != 0 {
		t.Fatalf("delete dry-run made %d remote calls", dryCaller.calls)
	}
}

func TestChatCategoryWriteCommandsDryRunNeverCallsRemote(t *testing.T) {
	for _, args := range [][]string{
		{"category", "create", "--title", "工作群", "--dry-run"},
		{"category", "delete", "--category-id", "7", "--dry-run"},
		{"category", "rename", "--category-id", "7", "--title", "新名称", "--dry-run"},
		{"category", "add-conv", "--group", "cid", "--category-ids", "1,2", "--dry-run"},
		{"category", "remove-conv", "--group", "cid", "--category-ids", "1,2", "--dry-run"},
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
