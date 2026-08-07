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
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

type reviewedProductReadCase struct {
	product       string
	build         func() *cobra.Command
	path          []string
	canonicalPath string
	rpc           string
	interfaceMode string
	parameters    map[string]bool
}

var reviewedDingOAReadCases = []reviewedProductReadCase{
	{"ding", newDingCommand, []string{"message", "list"}, "ding.list_ding_messages", "list_ding_messages", "composite", map[string]bool{"cursor": false, "type": false}},
	{"ding", newDingCommand, []string{"message", "receiver-status"}, "ding.list_ding_receiver_status", "list_ding_receiver_status", "composite", map[string]bool{"ding-id": true}},
	{"oa", newOaCommand, []string{"approval", "search-forms"}, "oa.search_form", "search_form", "mcp", map[string]bool{"query": true}},
	{"oa", newOaCommand, []string{"approval", "ding-info"}, "oa.oa_ding_user", "oa_ding_user", "mcp", map[string]bool{"task-id": true}},
}

func TestDingOAFormerReadExclusionsPublishReviewedAgentContracts(t *testing.T) {
	for _, test := range reviewedDingOAReadCases {
		t.Run(test.product+"_"+strings.Join(test.path, "_"), func(t *testing.T) {
			root := test.build()
			cmd, remaining, err := root.Find(test.path)
			if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
				t.Fatalf("%s %v is not a runnable exact leaf: cmd=%v remaining=%v err=%v", test.product, test.path, cmd, remaining, err)
			}
			final, ok := contractfinal.RuntimeContractFinal(cmd)
			if !ok {
				t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
			}
			wantCLIPath := test.product + " " + strings.Join(test.path, " ")
			if final.Identity == nil || final.Identity.CanonicalPath != test.canonicalPath || final.Identity.CLIPath != wantCLIPath || final.Identity.PrimaryCLIPath != wantCLIPath {
				t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
			}
			if final.Interface == nil || final.Interface.Mode != test.interfaceMode || final.Interface.Availability != "available" {
				t.Fatalf("%s interface=%#v", cmd.CommandPath(), final.Interface)
			}
			switch test.interfaceMode {
			case "mcp":
				if final.Interface.Ref == nil || final.Interface.Ref.ProductID != test.product || final.Interface.Ref.RPCName != test.rpc || final.Interface.Reason != "" {
					t.Fatalf("%s pinned interface=%#v", cmd.CommandPath(), final.Interface)
				}
			case "composite":
				if final.Interface.Ref != nil || !strings.Contains(final.Interface.Reason, "im/"+test.rpc) || !strings.Contains(final.Interface.Reason, "absent from the pinned MCP metadata snapshot") {
					t.Fatalf("%s unpinned interface=%#v", cmd.CommandPath(), final.Interface)
				}
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

func executeReviewedProductRead(t *testing.T, product string, build func() *cobra.Command, caller *scriptedToolCaller, args ...string) error {
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
	os.Args = []string{"dws", product}
	root := build()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	return root.ExecuteContext(context.Background())
}

func TestDingOAFormerReadExclusionsRouteOnceWithCanonicalArguments(t *testing.T) {
	tests := []struct {
		name    string
		product string
		build   func() *cobra.Command
		args    []string
		server  string
		rpc     string
		want    map[string]any
	}{
		{"ding_list", "ding", newDingCommand, []string{"message", "list", "--cursor", "7", "--type", "UNREAD"}, "im", "list_ding_messages", map[string]any{"cursor": int64(7), "type": "UNREAD"}},
		{"ding_receiver_status", "ding", newDingCommand, []string{"message", "receiver-status", "--ding-id", "ding-1"}, "im", "list_ding_receiver_status", map[string]any{"openDingId": "ding-1"}},
		{"oa_search_forms", "oa", newOaCommand, []string{"approval", "search-forms", "--query", "报销"}, "oa", "search_form", map[string]any{"query": "报销"}},
		{"oa_ding_info", "oa", newOaCommand, []string{"approval", "ding-info", "--task-id", "task-1"}, "oa", "oa_ding_user", map[string]any{"taskId": "task-1"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{}`}}}
			if err := executeReviewedProductRead(t, test.product, test.build, caller, test.args...); err != nil {
				t.Fatalf("execute %v: %v", test.args, err)
			}
			if caller.calls != 1 || caller.server != test.server || caller.tool != test.rpc || !reflect.DeepEqual(caller.args, test.want) {
				t.Fatalf("call=(count=%d server=%q tool=%q args=%#v), want %s/%s %#v", caller.calls, caller.server, caller.tool, caller.args, test.server, test.rpc, test.want)
			}
		})
	}
}

func TestDingOAFormerReadExclusionsRejectInvalidInputsBeforeRemoteCall(t *testing.T) {
	tests := []struct {
		name    string
		product string
		build   func() *cobra.Command
		args    []string
		typed   bool
	}{
		{"negative_cursor", "ding", newDingCommand, []string{"message", "list", "--cursor", "-1"}, true},
		{"unknown_ding_type", "ding", newDingCommand, []string{"message", "list", "--type", "UNKNOWN"}, true},
		{"missing_ding_id", "ding", newDingCommand, []string{"message", "receiver-status"}, false},
		{"missing_form_query", "oa", newOaCommand, []string{"approval", "search-forms"}, false},
		{"missing_task_id", "oa", newOaCommand, []string{"approval", "ding-info"}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := executeReviewedProductRead(t, test.product, test.build, caller, test.args...)
			if err == nil {
				t.Fatalf("execute %v succeeded, want validation failure", test.args)
			}
			// Value validation is owned by the helper and is typed here. Hard-required
			// flags are rejected earlier by Cobra; the production root normalizes that
			// pre-run error to the same validation/rc=3 envelope.
			if test.typed && apperrors.ExitCode(err) != apperrors.ExitCodeValidation {
				t.Fatalf("execute %v error=%v exit=%d, want typed validation", test.args, err, apperrors.ExitCode(err))
			}
			if caller.calls != 0 {
				t.Fatalf("execute %v made %d remote calls before validation", test.args, caller.calls)
			}
		})
	}
}
