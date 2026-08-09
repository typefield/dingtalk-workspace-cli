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

func validAgoalContractFieldsResponse() map[string]any {
	return map[string]any{
		"code": nil, "message": nil, "requestId": "request-1", "success": true,
		"content": []any{
			map[string]any{
				"active": true, "category": "BASE", "code": "field_code_1",
				"forceActive": false, "forceRequired": false, "id": "field-1",
				"required": true, "scheme": map[string]any{"width": float64(120)},
				"source": nil, "title": "字段一", "type": "TEXT",
			},
			map[string]any{
				"active": true, "category": "OBJECTIVE", "code": "field_code_2",
				"forceActive": true, "forceRequired": true, "id": "field-2",
				"required": true, "scheme": map[string]any{"format": "percent", "width": float64(160)},
				"source": nil, "title": "字段二", "type": "NUMBER",
			},
		},
	}
}

func TestAgoalContractFieldsProjectionPublishesStableDefinitionsOnly(t *testing.T) {
	data, meta, err := projectAgoalContractFields(validAgoalContractFieldsResponse())
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if data["fieldCoverageKnown"] != false {
		t.Fatalf("coverage = %#v", data["fieldCoverageKnown"])
	}
	fields, ok := data["fields"].([]map[string]any)
	if !ok || len(fields) != 2 {
		t.Fatalf("fields = %#v", data["fields"])
	}
	if fields[0]["fieldId"] != "field-1" || fields[0]["code"] != "field_code_1" || fields[0]["required"] != true {
		t.Fatalf("field = %#v", fields[0])
	}
	for _, hidden := range []string{"scheme", "source"} {
		if _, exists := fields[0][hidden]; exists {
			t.Fatalf("presentation/opaque field %q leaked: %#v", hidden, fields[0])
		}
	}
	if meta == nil || meta.Count == nil || *meta.Count != 2 || meta.Pagination != nil {
		t.Fatalf("meta = %#v", meta)
	}
}

func TestAgoalContractFieldsProjectionKeepsKnownEmptyDistinctFromUnknown(t *testing.T) {
	raw := validAgoalContractFieldsResponse()
	raw["content"] = []any{}
	data, meta, err := projectAgoalContractFields(raw)
	if err != nil {
		t.Fatalf("known empty: %v", err)
	}
	if len(data["fields"].([]map[string]any)) != 0 || meta.Count == nil || *meta.Count != 0 {
		t.Fatalf("data/meta = %#v %#v", data, meta)
	}
	delete(raw, "content")
	assertAgoalContractFieldsProjectionUnknown(t, func() error {
		_, _, err := projectAgoalContractFields(raw)
		return err
	}())
}

func TestAgoalContractFieldsProjectionRejectsDriftAndAmbiguity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown top field", func(raw map[string]any) { raw["complete"] = true }},
		{"wrong success type", func(raw map[string]any) { raw["success"] = "true" }},
		{"unknown row field", func(raw map[string]any) { contractFieldRow(raw, 0)["description"] = "new" }},
		{"duplicate id", func(raw map[string]any) { contractFieldRow(raw, 1)["id"] = contractFieldRow(raw, 0)["id"] }},
		{"duplicate code", func(raw map[string]any) { contractFieldRow(raw, 1)["code"] = contractFieldRow(raw, 0)["code"] }},
		{"empty title", func(raw map[string]any) { contractFieldRow(raw, 0)["title"] = " " }},
		{"string boolean", func(raw map[string]any) { contractFieldRow(raw, 0)["required"] = "false" }},
		{"non-null source", func(raw map[string]any) { contractFieldRow(raw, 0)["source"] = "custom" }},
		{"missing scheme width", func(raw map[string]any) { contractFieldRow(raw, 0)["scheme"] = map[string]any{} }},
		{"fractional scheme width", func(raw map[string]any) { contractFieldRow(raw, 0)["scheme"] = map[string]any{"width": 1.5} }},
		{"unknown scheme key", func(raw map[string]any) { contractFieldRow(raw, 0)["scheme"].(map[string]any)["color"] = "red" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := validAgoalContractFieldsResponse()
			tc.mutate(raw)
			_, _, err := projectAgoalContractFields(raw)
			assertAgoalContractFieldsProjectionUnknown(t, err)
		})
	}
}

func TestAgoalContractFieldsDualValidatePreservesLegacyJSONExactlyOnce(t *testing.T) {
	raw, err := json.Marshal(validAgoalContractFieldsResponse())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	run := func(state output.RolloutState) (string, int) {
		caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: string(raw)}}}
		installScriptedCaller(t, caller)
		root := newAgoalCommand()
		leaf, remaining, err := root.Find([]string{"contract", "fields"})
		if err != nil || len(remaining) != 0 {
			t.Fatalf("find leaf: remaining=%v err=%v", remaining, err)
		}
		output.SetCommandRollout(leaf, state)
		var stdout bytes.Buffer
		deps.Out.w = &stdout
		if err := executeFilterCoverage(t, root, "contract", "fields", "--request-id", "request-2"); err != nil {
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

func TestAgoalContractFieldsUnifiedActiveEmitsOneFrameworkResult(t *testing.T) {
	raw, err := json.Marshal(validAgoalContractFieldsResponse())
	if err != nil {
		t.Fatalf("marshal: %v", err)
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
	leaf, remaining, err := agoal.Find([]string{"contract", "fields"})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("find leaf: remaining=%v err=%v", remaining, err)
	}
	output.SetCommandRollout(leaf, output.RolloutUnifiedActive)
	root.AddCommand(agoal)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"agoal", "contract", "fields", "--format", "json"})
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
	if _, hasVersion := envelope["contract_version"]; hasVersion {
		t.Fatalf("version marker leaked: %#v", envelope)
	}
	data := envelope["data"].(map[string]any)
	meta := envelope["meta"].(map[string]any)
	if data["fieldCoverageKnown"] != false || meta["count"] != float64(2) {
		t.Fatalf("data/meta = %#v %#v", data, meta)
	}
}

func TestAgoalContractFieldsPublishesReviewedAgentContract(t *testing.T) {
	root := newAgoalCommand()
	cmd, remaining, err := root.Find([]string{"contract", "fields"})
	if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
		t.Fatalf("contract fields is not an exact leaf: cmd=%v remaining=%v err=%v", cmd, remaining, err)
	}
	final, ok := contractfinal.RuntimeContractFinal(cmd)
	if !ok {
		t.Fatal("agoal contract fields has no ContractFinal")
	}
	if final.Identity == nil || final.Identity.CanonicalPath != "agoal.contract_fields" || final.Identity.CLIPath != "agoal contract fields" {
		t.Fatalf("identity = %#v", final.Identity)
	}
	if final.Safety == nil || final.Safety.Effect != "read" || final.Safety.Risk != "low" || final.Safety.Confirmation != "not_required" || final.Safety.Idempotency != "idempotent" {
		t.Fatalf("safety = %#v", final.Safety)
	}
	if final.Interface == nil || final.Interface.Ref == nil || final.Interface.Ref.RPCName != agoalContractFieldsTool {
		t.Fatalf("interface = %#v", final.Interface)
	}
	if final.Selection == nil || strings.TrimSpace(final.Selection.AgentSummary) == "" || len(final.Selection.UseWhen) != 2 || len(final.Selection.AvoidWhen) != 2 || len(final.Selection.Examples) != 2 {
		t.Fatalf("selection = %#v", final.Selection)
	}
	if want := []contract.ParamDecl{{Name: "request-id", Property: "requestId"}}; !reflect.DeepEqual(final.Parameters, want) {
		t.Fatalf("parameters = %#v, want %#v", final.Parameters, want)
	}
	if final.Result == nil || final.Result.NDJSON == nil || final.Result.NDJSON.RecordPath != "fields" {
		t.Fatalf("result = %#v", final.Result)
	}
	if _, err := contract.NormalizeResultSpec(final.Result, "agoal.contract_fields"); err != nil {
		t.Fatalf("normalize result: %v", err)
	}
}

func TestAgoalContractFieldsRoutesRequestIDExactlyOnce(t *testing.T) {
	raw, err := json.Marshal(validAgoalContractFieldsResponse())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: string(raw)}}}
	installScriptedCaller(t, caller)
	if err := executeFilterCoverage(t, newAgoalCommand(), "contract", "fields", "--request-id", "request-2"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if caller.calls != 1 || caller.tool != agoalContractFieldsTool {
		t.Fatalf("calls/tool = %d/%q", caller.calls, caller.tool)
	}
	if want := map[string]any{"requestId": "request-2"}; !reflect.DeepEqual(caller.args, want) {
		t.Fatalf("args = %#v, want %#v", caller.args, want)
	}
}

func contractFieldRow(raw map[string]any, index int) map[string]any {
	return raw["content"].([]any)[index].(map[string]any)
}

func assertAgoalContractFieldsProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
