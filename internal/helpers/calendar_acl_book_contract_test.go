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

func executeReviewedCalendarCommand(t *testing.T, caller *scriptedToolCaller, args ...string) error {
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
	os.Args = []string{"dws", "calendar"}
	root := newCalendarCommand()
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

func TestCalendarACLAndBookUpdatePublishReviewedContracts(t *testing.T) {
	tests := []struct {
		path       []string
		canonical  string
		effect     string
		risk       string
		rpc        string
		parameters map[string]bool
	}{
		{[]string{"acl", "add"}, "calendar.add_acl", "write", "high", "add_acl", map[string]bool{"no-notification": false, "privilege": true, "user": true}},
		{[]string{"acl", "delete"}, "calendar.delete_acl", "destructive", "high", "delete_acl", map[string]bool{"acl-id": true}},
		{[]string{"book", "update"}, "calendar.update_calendar", "write", "medium", "update_calendar", map[string]bool{"desc": false, "id": true, "summary": false}},
	}

	root := newCalendarCommand()
	for _, test := range tests {
		t.Run(strings.Join(test.path, "_"), func(t *testing.T) {
			cmd, remaining, err := root.Find(test.path)
			if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
				t.Fatalf("calendar %v is not an exact runnable leaf: cmd=%v remaining=%v err=%v", test.path, cmd, remaining, err)
			}
			final, ok := contractfinal.RuntimeContractFinal(cmd)
			if !ok {
				t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
			}
			wantCLIPath := "calendar " + strings.Join(test.path, " ")
			if final.Identity == nil || final.Identity.CanonicalPath != test.canonical || final.Identity.CLIPath != wantCLIPath || final.Identity.PrimaryCLIPath != wantCLIPath {
				t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
			}
			if final.Safety == nil || final.Safety.Effect != test.effect || final.Safety.Risk != test.risk || final.Safety.Confirmation != "user_required" || final.Safety.Idempotency != "unknown" {
				t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
			}
			if final.DryRun == nil || final.DryRun.PreviewKind != contract.DryRunPreviewRequest || final.DryRun.RemoteReads {
				t.Fatalf("%s dry_run=%#v", cmd.CommandPath(), final.DryRun)
			}
			if final.Interface == nil || final.Interface.Mode != "mcp" || final.Interface.Ref == nil || final.Interface.Ref.ProductID != "calendar" || final.Interface.Ref.RPCName != test.rpc {
				t.Fatalf("%s interface=%#v", cmd.CommandPath(), final.Interface)
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

func TestCalendarACLAndBookUpdateRequireConfirmationBeforeCall(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{"acl_add", []string{"acl", "add", "--user", "user-1", "--privilege", "reader"}},
		{"acl_delete", []string{"acl", "delete", "--acl-id", "acl-1"}},
		{"book_update", []string{"book", "update", "--id", "calendar-1", "--summary", "新标题"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := executeReviewedCalendarCommand(t, caller, test.args...)
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

func TestCalendarACLAndBookUpdateRouteCanonicalArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		tool string
		want map[string]any
	}{
		{"acl_add", []string{"acl", "add", "--user", " user-1 ", "--privilege", "WRITER", "--no-notification", "--yes"}, "add_acl", map[string]any{"userId": "user-1", "privilege": "writer", "sendNotification": false}},
		{"acl_delete", []string{"acl", "delete", "--acl-id", " acl-1 ", "--yes"}, "delete_acl", map[string]any{"aclId": "acl-1"}},
		{"book_update", []string{"book", "update", "--id", " calendar-1 ", "--summary", " 新标题 ", "--desc", " 新描述 ", "--yes"}, "update_calendar", map[string]any{"calendarId": "calendar-1", "summary": "新标题", "description": "新描述"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{}`}}}
			if err := executeReviewedCalendarCommand(t, caller, test.args...); err != nil {
				t.Fatalf("execute %v: %v", test.args, err)
			}
			if caller.calls != 1 || caller.server != "calendar" || caller.tool != test.tool || !reflect.DeepEqual(caller.args, test.want) {
				t.Fatalf("call=(count=%d server=%q tool=%q args=%#v), want calendar/%s %#v", caller.calls, caller.server, caller.tool, caller.args, test.tool, test.want)
			}
		})
	}
}

func TestCalendarACLAndBookUpdateRejectInvalidInputsBeforeCall(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{"acl_privilege", []string{"acl", "add", "--user", "user-1", "--privilege", "owner", "--yes"}},
		{"acl_empty_user", []string{"acl", "add", "--user", " ", "--privilege", "reader", "--yes"}},
		{"acl_empty_id", []string{"acl", "delete", "--acl-id", " ", "--yes"}},
		{"book_primary", []string{"book", "update", "--id", "primary", "--summary", "新标题", "--yes"}},
		{"book_no_change", []string{"book", "update", "--id", "calendar-1", "--yes"}},
		{"book_blank_change", []string{"book", "update", "--id", "calendar-1", "--summary", " ", "--yes"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := executeReviewedCalendarCommand(t, caller, test.args...)
			if err == nil || apperrors.ExitCode(err) != apperrors.ExitCodeValidation {
				t.Fatalf("execute %v error=%v exit=%d, want validation", test.args, err, apperrors.ExitCode(err))
			}
			if caller.calls != 0 {
				t.Fatalf("execute %v made %d calls before validation", test.args, caller.calls)
			}
		})
	}
}

func TestCalendarACLAndBookUpdateDryRunDoesNotCallRemote(t *testing.T) {
	for _, args := range [][]string{
		{"acl", "add", "--user", "user-1", "--privilege", "reader", "--yes"},
		{"acl", "delete", "--acl-id", "acl-1", "--yes"},
		{"book", "update", "--id", "calendar-1", "--summary", "新标题", "--yes"},
	} {
		caller := &scriptedToolCaller{dry: true}
		if err := executeReviewedCalendarCommand(t, caller, args...); err != nil {
			t.Fatalf("dry-run %v: %v", args, err)
		}
		if caller.calls != 0 {
			t.Fatalf("dry-run %v made %d remote calls", args, caller.calls)
		}
	}
}
