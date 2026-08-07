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
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestTodoFormerExclusionsPublishReviewedAgentContracts(t *testing.T) {
	root := newTodoCommand()
	tests := []struct {
		path          []string
		canonicalPath string
		effect        string
		risk          string
		confirmation  string
		parameters    map[string]string
		previewKind   string
	}{
		{
			path:          []string{"task", "list-sub"},
			canonicalPath: "todo.list_sub_tasks",
			effect:        "read",
			risk:          "low",
			confirmation:  "not_required",
			parameters:    map[string]string{"task-id": "todoSubTaskListRequest.taskId"},
		},
		{
			path:          []string{"task", "remove-attachment"},
			canonicalPath: "todo.remove_todo_attachment",
			effect:        "destructive",
			risk:          "high",
			confirmation:  "user_required",
			parameters: map[string]string{
				"task-id":       "todoAttachmentRemoveRequest.taskId",
				"attachment-id": "todoAttachmentRemoveRequest.attachmentId",
			},
			previewKind: "request",
		},
	}

	for _, test := range tests {
		cmd, _, err := root.Find(test.path)
		if err != nil || cmd == nil || !cmd.Runnable() {
			t.Fatalf("%v is not runnable: cmd=%v err=%v", test.path, cmd, err)
		}
		final, ok := contractfinal.RuntimeContractFinal(cmd)
		if !ok {
			t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
		}
		if final.Identity == nil || final.Identity.CanonicalPath != test.canonicalPath {
			t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
		}
		if final.Interface == nil || final.Interface.Mode != "composite" || final.Interface.Availability != "available" || strings.TrimSpace(final.Interface.Reason) == "" {
			t.Fatalf("%s interface=%#v", cmd.CommandPath(), final.Interface)
		}
		if final.Safety == nil || final.Safety.Effect != test.effect || final.Safety.Risk != test.risk || final.Safety.Confirmation != test.confirmation {
			t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
		}
		if test.previewKind == "" && final.DryRun != nil {
			t.Fatalf("%s dry_run=%#v, want absent", cmd.CommandPath(), final.DryRun)
		}
		if test.previewKind != "" && (final.DryRun == nil || final.DryRun.PreviewKind != test.previewKind || final.DryRun.RemoteReads) {
			t.Fatalf("%s dry_run=%#v, want preview_kind=%q without remote reads", cmd.CommandPath(), final.DryRun, test.previewKind)
		}
		got := make(map[string]string, len(final.Parameters))
		for _, parameter := range final.Parameters {
			if parameter.Required == nil || !*parameter.Required {
				t.Fatalf("%s parameter %s is not required", cmd.CommandPath(), parameter.Name)
			}
			got[parameter.Name] = parameter.Property
		}
		if len(got) != len(test.parameters) {
			t.Fatalf("%s parameters=%#v", cmd.CommandPath(), final.Parameters)
		}
		for name, property := range test.parameters {
			if got[name] != property {
				t.Fatalf("%s parameter %s property=%q, want %q", cmd.CommandPath(), name, got[name], property)
			}
		}
	}
}

func TestTodoRemoveAttachmentRejectsMissingAttachmentBeforeExecution(t *testing.T) {
	caller := &scriptedToolCaller{}
	err := executeTodoEdge(t, caller, "task", "remove-attachment", "--task-id", "task-1", "--yes")
	var typed *apperrors.Error
	if err == nil || !strings.Contains(err.Error(), "--attachment-id") || apperrors.ExitCode(err) != apperrors.ExitCodeValidation || !errors.As(err, &typed) || typed.Reason != "missing_required_flags" {
		t.Fatalf("error=%v, want missing --attachment-id", err)
	}
	if caller.calls != 0 {
		t.Fatalf("missing attachment id made %d remote calls", caller.calls)
	}
}
