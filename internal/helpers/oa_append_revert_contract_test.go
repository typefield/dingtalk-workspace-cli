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
	"encoding/json"
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

func executeReviewedOACommand(t *testing.T, caller *scriptedToolCaller, args ...string) error {
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
	os.Args = []string{"dws", "oa"}
	root := newOaCommand()
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

func TestOAAppendAndRevertCommandsPublishReviewedContracts(t *testing.T) {
	tests := []struct {
		path       []string
		canonical  string
		effect     string
		risk       string
		confirm    string
		idempotent string
		rpc        string
		dryRun     bool
		parameters map[string]bool
	}{
		{[]string{"approval", "append-task"}, "oa.append_task", "write", "high", "user_required", "unknown", "append_task", true, map[string]bool{"activate-type": true, "agree-all": true, "appender-user-ids": true, "instance-id": true, "task-id": true, "type": true}},
		{[]string{"approval", "revert-activities"}, "oa.get_inst_revert_activities", "read", "low", "not_required", "idempotent", "get_inst_revert_activities", false, map[string]bool{"task-id": true}},
		{[]string{"approval", "revert-task"}, "oa.revert_task", "destructive", "high", "user_required", "unknown", "revert_task", true, map[string]bool{"action": true, "instance-id": true, "remark": false, "target-activity-id": true, "task-id": true}},
	}

	root := newOaCommand()
	for _, test := range tests {
		t.Run(strings.Join(test.path, "_"), func(t *testing.T) {
			cmd, remaining, err := root.Find(test.path)
			if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
				t.Fatalf("oa %v is not an exact runnable leaf: cmd=%v remaining=%v err=%v", test.path, cmd, remaining, err)
			}
			final, ok := contractfinal.RuntimeContractFinal(cmd)
			if !ok {
				t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
			}
			wantCLIPath := "oa " + strings.Join(test.path, " ")
			if final.Identity == nil || final.Identity.CanonicalPath != test.canonical || final.Identity.CLIPath != wantCLIPath || final.Identity.PrimaryCLIPath != wantCLIPath {
				t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
			}
			if final.Safety == nil || final.Safety.Effect != test.effect || final.Safety.Risk != test.risk || final.Safety.Confirmation != test.confirm || final.Safety.Idempotency != test.idempotent {
				t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
			}
			if (final.DryRun != nil) != test.dryRun || final.Interface == nil || final.Interface.Ref == nil || final.Interface.Ref.ProductID != "oa" || final.Interface.Ref.RPCName != test.rpc {
				t.Fatalf("%s dry_run=%#v interface=%#v", cmd.CommandPath(), final.DryRun, final.Interface)
			}
			if test.dryRun && (final.DryRun.PreviewKind != contract.DryRunPreviewRequest || final.DryRun.RemoteReads) {
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

func TestOAAppendAndRevertWritesRequireConfirmationBeforeCall(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{"append", []string{"approval", "append-task", "--instance-id", "instance-1", "--task-id", "12", "--type", "before", "--appender-user-ids", "user-1", "--activate-type", "ALL", "--agree-all", "true"}},
		{"revert", []string{"approval", "revert-task", "--instance-id", "instance-1", "--task-id", "12", "--target-activity-id", "activity-1", "--action", "REVERT_FOR_APPROVAL"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := executeReviewedOACommand(t, caller, test.args...)
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

func TestOAAppendAndRevertCommandsPreserveTypedArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		tool string
		want map[string]any
	}{
		{
			"append",
			[]string{"approval", "append-task", "--instance-id", "instance-1", "--task-id", "12", "--type", "PARALLEL", "--appender-user-ids", " user-1,user-2 ", "--activate-type", "one_by_one", "--agree-all", "false", "--yes"},
			"append_task",
			map[string]any{"processInstanceId": "instance-1", "taskId": "12", "type": "Parallel", "appenderUserIds": []string{"user-1", "user-2"}, "activateType": "ONE_BY_ONE", "agreeAll": false},
		},
		{
			"revert_activities",
			[]string{"approval", "revert-activities", "--task-id", "9007199254740993"},
			"get_inst_revert_activities",
			map[string]any{"taskId": json.Number("9007199254740993")},
		},
		{
			"revert_task",
			[]string{"approval", "revert-task", "--instance-id", "instance-1", "--task-id", "9007199254740993", "--target-activity-id", "activity-1", "--action", "revert_for_approval", "--remark", "重新审批", "--yes"},
			"revert_task",
			map[string]any{"RevertTaskRequest": map[string]any{"processInstanceId": "instance-1", "taskId": json.Number("9007199254740993"), "targetActivityId": "activity-1", "revertAction": "REVERT_FOR_APPROVAL", "remark": "重新审批"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{}`}}}
			if err := executeReviewedOACommand(t, caller, test.args...); err != nil {
				t.Fatalf("execute %v: %v", test.args, err)
			}
			if caller.calls != 1 || caller.server != "oa" || caller.tool != test.tool {
				t.Fatalf("call=(count=%d server=%q tool=%q), want oa/%s", caller.calls, caller.server, caller.tool, test.tool)
			}
			if test.name == "append" {
				if taskID, ok := caller.args["taskId"].(json.Number); !ok || taskID.String() != "12" {
					t.Fatalf("append taskId=%#v", caller.args["taskId"])
				}
				caller.args["taskId"] = "12"
			}
			if !reflect.DeepEqual(caller.args, test.want) {
				t.Fatalf("args=%#v, want %#v", caller.args, test.want)
			}
		})
	}
}

func TestOAAppendAndRevertRejectInvalidInputsBeforeCall(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{"append_type", []string{"approval", "append-task", "--instance-id", "instance-1", "--task-id", "12", "--type", "around", "--appender-user-ids", "user-1", "--activate-type", "ALL", "--agree-all", "true", "--yes"}},
		{"append_activation", []string{"approval", "append-task", "--instance-id", "instance-1", "--task-id", "12", "--type", "before", "--appender-user-ids", "user-1", "--activate-type", "RANDOM", "--agree-all", "true", "--yes"}},
		{"append_empty_users", []string{"approval", "append-task", "--instance-id", "instance-1", "--task-id", "12", "--type", "before", "--appender-user-ids", ",", "--activate-type", "ALL", "--agree-all", "true", "--yes"}},
		{"append_boolean", []string{"approval", "append-task", "--instance-id", "instance-1", "--task-id", "12", "--type", "before", "--appender-user-ids", "user-1", "--activate-type", "ALL", "--agree-all", "yes", "--yes"}},
		{"activities_nan", []string{"approval", "revert-activities", "--task-id", "NaN"}},
		{"revert_action", []string{"approval", "revert-task", "--instance-id", "instance-1", "--task-id", "12", "--target-activity-id", "activity-1", "--action", "RETRY", "--yes"}},
		{"revert_resubmit_target", []string{"approval", "revert-task", "--instance-id", "instance-1", "--task-id", "12", "--target-activity-id", "activity-1", "--action", "REVERT_FOR_RESUBMIT", "--yes"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := executeReviewedOACommand(t, caller, test.args...)
			if err == nil || apperrors.ExitCode(err) != apperrors.ExitCodeValidation {
				t.Fatalf("execute %v error=%v exit=%d, want validation", test.args, err, apperrors.ExitCode(err))
			}
			if caller.calls != 0 {
				t.Fatalf("execute %v made %d calls before validation", test.args, caller.calls)
			}
		})
	}
}

func TestOAAppendAndRevertDryRunDoesNotCallRemote(t *testing.T) {
	for _, args := range [][]string{
		{"approval", "append-task", "--instance-id", "instance-1", "--task-id", "12", "--type", "before", "--appender-user-ids", "user-1", "--activate-type", "ALL", "--agree-all", "true", "--yes"},
		{"approval", "revert-task", "--instance-id", "instance-1", "--task-id", "12", "--target-activity-id", "activity-1", "--action", "REVERT_FOR_APPROVAL", "--yes"},
	} {
		caller := &scriptedToolCaller{dry: true}
		if err := executeReviewedOACommand(t, caller, args...); err != nil {
			t.Fatalf("dry-run %v: %v", args, err)
		}
		if caller.calls != 0 {
			t.Fatalf("dry-run %v made %d remote calls", args, caller.calls)
		}
	}
}
