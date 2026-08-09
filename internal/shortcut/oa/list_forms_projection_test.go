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

package oa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type oaDiscoveryCaller struct {
	product string
	tool    string
	texts   map[string]string
}

func (c *oaDiscoveryCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.product, c.tool = product, tool
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: c.texts[tool]}}}, nil
}

func (*oaDiscoveryCaller) Format() string { return "json" }
func (*oaDiscoveryCaller) DryRun() bool   { return false }
func (*oaDiscoveryCaller) Fields() string { return "" }
func (*oaDiscoveryCaller) JQ() string     { return "" }

// TestListFormsProjectProcessCodeListShape guards against the projection-data-loss
// class: list_user_visible_process nests the forms under result.processCodeList.
// The resolver MUST probe that exact key — otherwise the whole list silently
// projects to empty (exit 0, no error envelope) while the backend has data.
func TestListFormsProjectProcessCodeListShape(t *testing.T) {
	// Faithful list_user_visible_process shape (as returned by the backend).
	const raw = `{"result":{"processCodeList":[
		{"processCode":"PROC-1","processName":"leave","dirName":"attendance"},
		{"processCode":"PROC-2","processName":"check-in fix","dirName":"attendance"},
		{"processCode":"PROC-3","processName":"overtime","dirName":"attendance"}
	],"totalCount":-1},"success":true}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	forms, err := listFormsProject(data)
	if err != nil {
		t.Fatalf("projection returned error: %v", err)
	}
	if len(forms) != 3 {
		t.Fatalf("lower/upper mismatch: 3 forms in backend, projection returned %d (forms=%v)", len(forms), forms)
	}
	for _, f := range forms {
		if f["processCode"] == nil || f["name"] == nil {
			t.Fatalf("projected form missing processCode/name: %v", f)
		}
	}
}

// TestListFormsProjectBareArrayShape ensures a bare top-level array still works.
func TestListFormsProjectBareArrayShape(t *testing.T) {
	const raw = `{"result":[
		{"processCode":"PROC-9","processName":"generic approval"}
	]}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	forms, err := listFormsProject(data)
	if err != nil {
		t.Fatalf("bare array projection returned error: %v", err)
	}
	if len(forms) != 1 {
		t.Fatalf("bare array shape: want 1 form, got %d (%v)", len(forms), forms)
	}
}

// TestOAInstanceResolveListValuesShape guards the shared resolver behind
// +list-pending / +list-executed / +list-submitted / +list-cc. Those approval
// instance tools nest the list under result.values; the resolver must probe
// "values" or all four commands silently return empty despite backend records.
func TestOAInstanceResolveListValuesShape(t *testing.T) {
	const raw = `{"result":{"hasMore":false,"values":[
		{"processInstanceId":"i-1","title":"user leave"},
		{"processInstanceId":"i-2","title":"user reimbursement"}
	]}}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	got, known := oaInstanceResolveList(data)
	if !known || len(got) != 2 {
		t.Fatalf("lower/upper mismatch: result.values has 2 entries, resolver returned %d", len(got))
	}
	// End-to-end through a real projection that uses the shared resolver.
	instances, err := listSubmittedProject(data)
	if err != nil {
		t.Fatalf("listSubmittedProject returned error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("listSubmittedProject: want 2, got %d (%v)", len(instances), instances)
	}
}

func TestOAListProjectionSeparatesKnownEmptyFromUnknown(t *testing.T) {
	for name, project := range map[string]func(map[string]any) ([]map[string]any, error){
		"forms":     listFormsProject,
		"search":    searchFormsProject,
		"pending":   listPendingProject,
		"executed":  listExecutedProject,
		"submitted": listSubmittedProject,
		"cc":        listCcProject,
	} {
		t.Run(name+" known empty", func(t *testing.T) {
			rows, err := project(map[string]any{"items": []any{}})
			if err != nil || rows == nil || len(rows) != 0 {
				t.Fatalf("known empty = %#v, %v; want non-nil empty list", rows, err)
			}
		})
		t.Run(name+" unknown", func(t *testing.T) {
			_, err := project(map[string]any{"unexpected": []any{}})
			assertOAProjectionUnknown(t, err)
		})
		t.Run(name+" malformed row", func(t *testing.T) {
			_, err := project(map[string]any{"items": []any{"opaque"}})
			assertOAProjectionUnknown(t, err)
		})
	}
}

func TestOAListProjectionRejectsDisplayOnlyRows(t *testing.T) {
	for name, project := range map[string]func(map[string]any) ([]map[string]any, error){
		"forms":  listFormsProject,
		"search": searchFormsProject,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := project(map[string]any{"items": []any{map[string]any{"name": "Leave"}}})
			assertOAProjectionUnknown(t, err)
		})
	}
	for name, project := range map[string]func(map[string]any) ([]map[string]any, error){
		"pending":   listPendingProject,
		"executed":  listExecutedProject,
		"submitted": listSubmittedProject,
		"cc":        listCcProject,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := project(map[string]any{"items": []any{map[string]any{"title": "Reimbursement"}}})
			assertOAProjectionUnknown(t, err)
		})
	}
}

func TestOAApprovalDiscoveryUsesUnifiedOutput(t *testing.T) {
	for name, declaration := range map[string]shortcut.Shortcut{
		"list-pending":   ListPending,
		"list-forms":     ListForms,
		"search-forms":   SearchForms,
		"list-executed":  ListExecuted,
		"list-submitted": ListSubmitted,
		"list-cc":        ListCc,
	} {
		if declaration.OutputRollout != output.RolloutUnifiedActive {
			t.Fatalf("%s rollout = %q, want unified active", name, declaration.OutputRollout)
		}
	}
}

func TestOAApprovalDiscoveryUnifiedOutputHasOneMachineEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		decl    shortcut.Shortcut
		tool    string
		text    string
		itemKey string
		args    []string
	}{
		{
			name:    "pending",
			decl:    ListPending,
			tool:    "list_pending_approvals",
			text:    `{"result":{"values":[{"processInstanceId":"instance-1","title":"Leave"}]}}`,
			itemKey: "instances",
			args:    []string{"--start", "1", "--end", "2"},
		},
		{
			name:    "forms",
			decl:    ListForms,
			tool:    "list_user_visible_process",
			text:    `{"result":{"processCodeList":[{"processCode":"PROC-1","name":"Leave"}]}}`,
			itemKey: "forms",
		},
		{
			name:    "search forms",
			decl:    SearchForms,
			tool:    "search_form",
			text:    `{"result":{"forms":[{"processCode":"PROC-1","name":"Leave"}]}}`,
			itemKey: "forms",
			args:    []string{"--query", "Leave"},
		},
		{
			name:    "executed",
			decl:    ListExecuted,
			tool:    "get_done_tasks",
			text:    `{"result":{"values":[{"processInstanceId":"instance-1","title":"Leave"}]}}`,
			itemKey: "instances",
		},
		{
			name:    "submitted",
			decl:    ListSubmitted,
			tool:    "get_submitted_instances",
			text:    `{"result":{"values":[{"processInstanceId":"instance-1","title":"Leave"}]}}`,
			itemKey: "instances",
		},
		{
			name:    "cc",
			decl:    ListCc,
			tool:    "get_noticed_instances",
			text:    `{"result":{"values":[{"processInstanceId":"instance-1","title":"Leave"}]}}`,
			itemKey: "instances",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &oaDiscoveryCaller{texts: map[string]string{tc.tool: tc.text}}
			helpers.InitDeps(caller)
			cmd := corecmd.New(shortcut.FromShortcut(tc.decl))
			cmd.PersistentFlags().String("format", "json", "")
			ctx, _ := output.WithResultStore(context.Background())
			cmd.SetContext(ctx)
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(append(tc.args, "--format", "json"))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			exitCode, emitted, err := output.EmitStoredResult(cmd)
			if err != nil || !emitted || exitCode != 0 {
				t.Fatalf("emit: code=%d emitted=%v err=%v", exitCode, emitted, err)
			}
			if caller.product != "oa" || caller.tool != tc.tool {
				t.Fatalf("route = %s/%s, want oa/%s", caller.product, caller.tool, tc.tool)
			}
			var envelope map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("decode output: %v\n%s", err, stdout.String())
			}
			if envelope["ok"] != true || envelope["outcome"] != "success" {
				t.Fatalf("envelope = %#v", envelope)
			}
			if _, leaked := envelope["contract_version"]; leaked {
				t.Fatalf("result leaked removed version marker: %#v", envelope)
			}
			data := envelope["data"].(map[string]any)
			if data["count"] != float64(1) || data["pagination_known"] != false || len(data[tc.itemKey].([]any)) != 1 {
				t.Fatalf("data = %#v", data)
			}
			if envelope["meta"].(map[string]any)["count"] != float64(1) {
				t.Fatalf("meta = %#v", envelope["meta"])
			}
		})
	}
}

func assertOAProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
