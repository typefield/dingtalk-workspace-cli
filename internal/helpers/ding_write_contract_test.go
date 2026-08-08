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
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

type reviewedDingWriteCase struct {
	path          []string
	canonicalPath string
	effect        string
	risk          string
	rpc           string
	interfaceMode string
	parameters    map[string]bool
}

var reviewedDingWriteCases = []reviewedDingWriteCase{
	{[]string{"message", "send"}, "ding.send_ding_message", "write", "medium", "send_ding_message", "mcp", map[string]bool{"content": true, "robot-code": false, "type": false, "users": true}},
	{[]string{"message", "recall"}, "ding.recall_ding_message", "destructive", "high", "recall_ding_message", "mcp", map[string]bool{"id": true, "robot-code": false}},
	{[]string{"message", "send-personal"}, "ding.send_personal_ding", "write", "medium", "send_personal_ding", "composite", map[string]bool{"content": true, "type": false, "users": true, "uuid": false}},
	{[]string{"message", "send-by-message"}, "ding.send_ding_by_message", "write", "medium", "send_ding_by_message", "composite", map[string]bool{"group": true, "message-id": true, "type": false, "users": true, "uuid": false}},
	{[]string{"message", "recall-personal"}, "ding.recall_personal_ding", "destructive", "high", "recall_personal_ding", "composite", map[string]bool{"id": true}},
}

func TestDingWriteCommandsPublishReviewedSafetyAndAgentContracts(t *testing.T) {
	root := newDingCommand()
	for _, test := range reviewedDingWriteCases {
		t.Run(strings.Join(test.path, "_"), func(t *testing.T) {
			cmd, remaining, err := root.Find(test.path)
			if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
				t.Fatalf("ding %v is not a runnable exact leaf: cmd=%v remaining=%v err=%v", test.path, cmd, remaining, err)
			}
			final, ok := contractfinal.RuntimeContractFinal(cmd)
			if !ok {
				t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
			}
			wantCLIPath := "ding " + strings.Join(test.path, " ")
			if final.Identity == nil || final.Identity.CanonicalPath != test.canonicalPath || final.Identity.CLIPath != wantCLIPath || final.Identity.PrimaryCLIPath != wantCLIPath {
				t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
			}
			if final.Safety == nil || final.Safety.Effect != test.effect || final.Safety.Risk != test.risk || final.Safety.Confirmation != "user_required" || final.Safety.Idempotency != "unknown" {
				t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
			}
			if final.DryRun == nil || final.DryRun.PreviewKind != contract.DryRunPreviewRequest || final.DryRun.RemoteReads {
				t.Fatalf("%s dry_run=%#v", cmd.CommandPath(), final.DryRun)
			}
			if final.Interface == nil || final.Interface.Mode != test.interfaceMode || final.Interface.Availability != "available" {
				t.Fatalf("%s interface=%#v", cmd.CommandPath(), final.Interface)
			}
			if test.interfaceMode == "mcp" {
				if final.Interface.Ref == nil || final.Interface.Ref.ProductID != "ding" || final.Interface.Ref.RPCName != test.rpc {
					t.Fatalf("%s pinned interface=%#v", cmd.CommandPath(), final.Interface)
				}
			} else if final.Interface.Ref != nil || !strings.Contains(final.Interface.Reason, "im/"+test.rpc) {
				t.Fatalf("%s unpinned interface=%#v", cmd.CommandPath(), final.Interface)
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

func executeReviewedDingWrite(t *testing.T, caller *scriptedToolCaller, args ...string) error {
	t.Helper()
	previousDeps := deps
	previousArgs := os.Args
	t.Cleanup(func() {
		deps = previousDeps
		os.Args = previousArgs
	})

	InitDeps(caller)
	deps.Out.w = io.Discard
	deps.Out.errW = io.Discard
	os.Args = []string{"dws", "ding"}
	root := newDingCommand()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetIn(strings.NewReader(""))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.PersistentFlags().Bool("yes", false, "confirm the operation")
	root.PersistentFlags().Bool("dry-run", false, "preview the operation")
	root.SetArgs(args)
	return root.ExecuteContext(context.Background())
}

func TestDingWriteCommandsRequireConfirmationBeforeRemoteCall(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"robot_send", []string{"message", "send", "--robot-code", "robot-1", "--users", "user-1", "--content", "hello"}},
		{"robot_recall", []string{"message", "recall", "--robot-code", "robot-1", "--id", "ding-1"}},
		{"personal_send", []string{"message", "send-personal", "--users", "open-1", "--content", "hello"}},
		{"message_send", []string{"message", "send-by-message", "--group", "cid-1", "--message-id", "msg-1", "--users", "open-1"}},
		{"personal_recall", []string{"message", "recall-personal", "--id", "ding-1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := executeReviewedDingWrite(t, caller, test.args...)
			if err == nil {
				t.Fatalf("execute %v succeeded without confirmation", test.args)
			}
			var typed *apperrors.Error
			if !errors.As(err, &typed) || typed.Category != apperrors.CategoryValidation || typed.Reason != "confirmation_required" {
				t.Fatalf("execute %v error=%T %v, want confirmation_required", test.args, err, err)
			}
			if caller.calls != 0 {
				t.Fatalf("execute %v made %d calls before confirmation", test.args, caller.calls)
			}
		})
	}
}

func TestDingWriteCommandsRouteOnceAfterExplicitConfirmation(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		server string
		rpc    string
		want   map[string]any
	}{
		{"robot_send", []string{"message", "send", "--robot-code", "robot-1", "--users", " user-1,user-2 ", "--content", "hello", "--type", "SMS", "--yes"}, "ding", "send_ding_message", map[string]any{"robotCode": "robot-1", "remindType": 2, "receiverUserIdList": []string{"user-1", "user-2"}, "content": "hello"}},
		{"robot_recall", []string{"message", "recall", "--robot-code", "robot-1", "--id", "ding-1", "--yes"}, "ding", "recall_ding_message", map[string]any{"robotCode": "robot-1", "openDingId": "ding-1"}},
		{"personal_send", []string{"message", "send-personal", "--users", "open-1,open-2", "--content", "hello", "--type", "CALL", "--uuid", "request-1", "--yes"}, "im", "send_personal_ding", map[string]any{"receiverOpenDingTalkIds": []string{"open-1", "open-2"}, "content": "hello", "remindType": "call", "uuid": "request-1"}},
		{"message_send", []string{"message", "send-by-message", "--group", "cid-1", "--message-id", "msg-1", "--users", "open-1", "--type", "APP", "--yes"}, "im", "send_ding_by_message", map[string]any{"openConversationId": "cid-1", "openMessageId": "msg-1", "receiverOpenDingTalkIds": []string{"open-1"}, "remindType": "APP"}},
		{"personal_recall", []string{"message", "recall-personal", "--id", "ding-1", "--yes"}, "im", "recall_personal_ding", map[string]any{"openDingId": "ding-1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{}`}}}
			if err := executeReviewedDingWrite(t, caller, test.args...); err != nil {
				t.Fatalf("execute %v: %v", test.args, err)
			}
			if caller.calls != 1 || caller.server != test.server || caller.tool != test.rpc || !reflect.DeepEqual(caller.args, test.want) {
				t.Fatalf("call=(count=%d server=%q tool=%q args=%#v), want %s/%s %#v", caller.calls, caller.server, caller.tool, caller.args, test.server, test.rpc, test.want)
			}
		})
	}
}

func TestDingSendCommandsRejectInvalidRecipientsAndTypesBeforeRemoteCall(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{"robot_empty_recipients", []string{"message", "send", "--robot-code", "robot-1", "--users", ",", "--content", "hello", "--yes"}},
		{"robot_unknown_type", []string{"message", "send", "--robot-code", "robot-1", "--users", "user-1", "--content", "hello", "--type", "fax", "--yes"}},
		{"personal_empty_recipients", []string{"message", "send-personal", "--users", ",", "--content", "hello", "--yes"}},
		{"message_unknown_type", []string{"message", "send-by-message", "--group", "cid-1", "--message-id", "msg-1", "--users", "open-1", "--type", "fax", "--yes"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := executeReviewedDingWrite(t, caller, test.args...)
			if err == nil || apperrors.ExitCode(err) != apperrors.ExitCodeValidation {
				t.Fatalf("execute %v error=%v exit=%d, want validation", test.args, err, apperrors.ExitCode(err))
			}
			if caller.calls != 0 {
				t.Fatalf("execute %v made %d calls before validation", test.args, caller.calls)
			}
		})
	}
}
