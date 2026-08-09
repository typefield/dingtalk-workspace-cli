// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/spf13/cobra"
)

func validAgoalObjTemplateListResponse(page, pageSize, total float64) map[string]any {
	return map[string]any{
		"code": nil, "message": nil, "requestId": "request-1", "success": true,
		"content": map[string]any{
			"page": page, "pageSize": pageSize, "totalCount": total,
			"result": []any{map[string]any{
				"computeByWeight":   true,
				"creator":           map[string]any{"id": "staff-1", "name": "Reviewer", "dingUserId": nil},
				"dimensionWeight":   true,
				"dimensions":        []any{map[string]any{"title": "摘要不发布", "weight": float64(100)}},
				"id":                "template-1",
				"objectiveCategory": "PBC",
				"objectiveWeight":   false,
				"status":            "INIT",
				"title":             "研发模板",
				"type":              "MANUAL",
			}},
		},
	}
}

func TestAgoalObjTemplateListProjectionBuildsResumableAndTerminalPages(t *testing.T) {
	first := validAgoalObjTemplateListResponse(1, 1, 2)
	data, meta, err := projectAgoalObjTemplateList(first)
	if err != nil {
		t.Fatalf("project first page: %v", err)
	}
	if data["authoritativeInventory"] != false || data["inventoryCoverageKnown"] != false || data["totalCount"] != int64(2) {
		t.Fatalf("data = %#v", data)
	}
	templates, ok := data["templates"].([]map[string]any)
	if !ok || len(templates) != 1 || templates[0]["templateId"] != "template-1" {
		t.Fatalf("templates = %#v", data["templates"])
	}
	for _, hidden := range []string{"creator", "dimensions"} {
		if _, exposed := templates[0][hidden]; exposed {
			t.Fatalf("opaque field %q was exposed: %#v", hidden, templates[0])
		}
	}
	if meta == nil || meta.Count == nil || *meta.Count != 1 || meta.Pagination == nil || meta.Pagination.EndpointExhausted || meta.Pagination.NextToken != "2" || meta.Pagination.Pages != 1 || meta.Pagination.Items != 1 {
		t.Fatalf("first meta = %#v", meta)
	}

	second := validAgoalObjTemplateListResponse(2, 1, 2)
	data, meta, err = projectAgoalObjTemplateList(second)
	if err != nil {
		t.Fatalf("project second page: %v", err)
	}
	if meta.Pagination == nil || !meta.Pagination.EndpointExhausted || meta.Pagination.NextToken != "" {
		t.Fatalf("terminal meta = %#v", meta)
	}
}

func TestAgoalObjTemplateListProjectionKeepsKnownEmptyDistinctFromUnknown(t *testing.T) {
	raw := validAgoalObjTemplateListResponse(1, 20, 0)
	raw["content"].(map[string]any)["result"] = []any{}
	data, meta, err := projectAgoalObjTemplateList(raw)
	if err != nil {
		t.Fatalf("known empty: %v", err)
	}
	if len(data["templates"].([]map[string]any)) != 0 || meta.Count == nil || *meta.Count != 0 || !meta.Pagination.EndpointExhausted {
		t.Fatalf("known empty data/meta = %#v %#v", data, meta)
	}
	delete(raw["content"].(map[string]any), "result")
	assertAgoalObjTemplateProjectionUnknown(t, func() error {
		_, _, err := projectAgoalObjTemplateList(raw)
		return err
	}())
}

func TestAgoalObjTemplateListProjectionRejectsDriftAndPaginationContradictions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown top field", func(raw map[string]any) { raw["hasMore"] = false }},
		{"fractional page", func(raw map[string]any) { raw["content"].(map[string]any)["page"] = 1.5 }},
		{"zero page size", func(raw map[string]any) { raw["content"].(map[string]any)["pageSize"] = float64(0) }},
		{"short nonterminal page", func(raw map[string]any) {
			content := raw["content"].(map[string]any)
			content["pageSize"] = float64(2)
			content["totalCount"] = float64(3)
		}},
		{"duplicate template id", func(raw map[string]any) {
			content := raw["content"].(map[string]any)
			row := content["result"].([]any)[0]
			content["result"] = []any{row, row}
			content["pageSize"] = float64(2)
			content["totalCount"] = float64(2)
		}},
		{"missing stable id", func(raw map[string]any) { objTemplateRow(raw)["id"] = "" }},
		{"unknown row field", func(raw map[string]any) { objTemplateRow(raw)["newField"] = true }},
		{"wrong boolean", func(raw map[string]any) { objTemplateRow(raw)["objectiveWeight"] = "false" }},
		{"wrong creator shape", func(raw map[string]any) { objTemplateRow(raw)["creator"] = []any{} }},
		{"wrong dimensions shape", func(raw map[string]any) { objTemplateRow(raw)["dimensions"] = map[string]any{} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := validAgoalObjTemplateListResponse(1, 1, 2)
			tc.mutate(raw)
			_, _, err := projectAgoalObjTemplateList(raw)
			assertAgoalObjTemplateProjectionUnknown(t, err)
		})
	}
}

func TestAgoalObjTemplateListDualValidatePreservesLegacyJSONExactlyOnce(t *testing.T) {
	raw, err := json.Marshal(validAgoalObjTemplateListResponse(1, 1, 2))
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	run := func(state output.RolloutState) (string, int) {
		caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: string(raw)}}}
		installScriptedCaller(t, caller)
		root := newAgoalCommand()
		leaf, remaining, err := root.Find([]string{"obj-template", "list"})
		if err != nil || len(remaining) != 0 {
			t.Fatalf("find leaf: remaining=%v err=%v", remaining, err)
		}
		output.SetCommandRollout(leaf, state)
		var stdout bytes.Buffer
		deps.Out.w = &stdout
		if err := executeFilterCoverage(t, root, "obj-template", "list", "--page", "1", "--page-size", "1"); err != nil {
			t.Fatalf("execute %s: %v", state, err)
		}
		return stdout.String(), caller.calls
	}
	legacy, legacyCalls := run(output.RolloutLegacyOnly)
	dual, dualCalls := run(output.RolloutDualValidate)
	if legacy != dual {
		t.Fatalf("dual changed legacy bytes:\nlegacy=%q\ndual=%q", legacy, dual)
	}
	if legacyCalls != 1 || dualCalls != 1 {
		t.Fatalf("business calls legacy/dual = %d/%d", legacyCalls, dualCalls)
	}
}

func TestAgoalObjTemplateListUnifiedActiveEmitsFrameworkPagination(t *testing.T) {
	raw, err := json.Marshal(validAgoalObjTemplateListResponse(1, 1, 2))
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: string(raw)}}}
	installScriptedCaller(t, caller)
	ctx, _ := output.WithResultStore(context.Background())
	root := &cobra.Command{Use: "dws", SilenceErrors: true, SilenceUsage: true}
	root.SetContext(ctx)
	root.PersistentFlags().String("format", "json", "")
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentPostRunE = func(cmd *cobra.Command, _ []string) error {
		_, _, err := output.EmitStoredResult(cmd)
		return err
	}
	agoal := newAgoalCommand()
	leaf, remaining, err := agoal.Find([]string{"obj-template", "list"})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("find leaf: remaining=%v err=%v", remaining, err)
	}
	output.SetCommandRollout(leaf, output.RolloutUnifiedActive)
	root.AddCommand(agoal)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"agoal", "obj-template", "list", "--page", "1", "--page-size", "1", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if caller.calls != 1 {
		t.Fatalf("business calls = %d, want 1", caller.calls)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode stdout %q: %v", stdout.String(), err)
	}
	if envelope["ok"] != true || envelope["outcome"] != "success" {
		t.Fatalf("envelope = %#v", envelope)
	}
	meta := envelope["meta"].(map[string]any)
	pagination := meta["pagination"].(map[string]any)
	if meta["count"] != float64(1) || pagination["endpoint_exhausted"] != false || pagination["next_token"] != "2" {
		t.Fatalf("meta = %#v", meta)
	}
}

func TestAgoalObjTemplateListPublishesReviewedAgentContract(t *testing.T) {
	root := newAgoalCommand()
	cmd, remaining, err := root.Find([]string{"obj-template", "list"})
	if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
		t.Fatalf("obj-template list is not an exact runnable leaf: cmd=%v remaining=%v err=%v", cmd, remaining, err)
	}
	final, ok := contractfinal.RuntimeContractFinal(cmd)
	if !ok {
		t.Fatal("agoal obj-template list has no ContractFinal")
	}
	if final.Identity == nil || final.Identity.CanonicalPath != "agoal.obj_template_list" || final.Identity.CLIPath != "agoal obj-template list" {
		t.Fatalf("identity = %#v", final.Identity)
	}
	if final.Safety == nil || final.Safety.Effect != "read" || final.Safety.Risk != "low" || final.Safety.Confirmation != "not_required" || final.Safety.Idempotency != "idempotent" {
		t.Fatalf("safety = %#v", final.Safety)
	}
	if final.Interface == nil || final.Interface.Ref == nil || final.Interface.Ref.RPCName != agoalObjTemplateListTool {
		t.Fatalf("interface = %#v", final.Interface)
	}
	if final.Selection == nil || strings.TrimSpace(final.Selection.AgentSummary) == "" || len(final.Selection.UseWhen) != 2 || len(final.Selection.AvoidWhen) != 2 || len(final.Selection.Examples) != 2 {
		t.Fatalf("selection = %#v", final.Selection)
	}
	got := map[string]string{}
	for _, parameter := range final.Parameters {
		got[parameter.Name] = parameter.Property
	}
	if want := map[string]string{"keyword": "keyword", "page": "page", "page-size": "pageSize", "request-id": "requestId"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parameters = %#v, want %#v", got, want)
	}
	if final.Result == nil || final.Result.NDJSON == nil || final.Result.NDJSON.RecordPath != "templates" {
		t.Fatalf("result = %#v", final.Result)
	}
	if _, err := contract.NormalizeResultSpec(final.Result, "agoal.obj_template_list"); err != nil {
		t.Fatalf("normalize result: %v", err)
	}
}

func TestAgoalObjTemplateListRejectsInvalidPageFlagsBeforeBusinessCall(t *testing.T) {
	for _, args := range [][]string{{"obj-template", "list", "--page", "0"}, {"obj-template", "list", "--page-size", "-1"}} {
		caller := &scriptedToolCaller{format: "json"}
		installScriptedCaller(t, caller)
		if err := executeFilterCoverage(t, newAgoalCommand(), args...); err == nil {
			t.Fatalf("args %v unexpectedly succeeded", args)
		}
		if caller.calls != 0 {
			t.Fatalf("args %v business calls = %d", args, caller.calls)
		}
	}
}

func objTemplateRow(raw map[string]any) map[string]any {
	return raw["content"].(map[string]any)["result"].([]any)[0].(map[string]any)
}

func assertAgoalObjTemplateProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
