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

func validAgoalReportStatisticsResponse() map[string]any {
	return map[string]any{
		"code": nil, "message": nil, "requestId": "request-1", "success": true,
		"content": []any{map[string]any{
			"allowTimeout":       true,
			"content":            []any{map[string]any{"id": "field-1", "name": "正文", "type": "Text", "value": map[string]any{"asl": "", "editablePlaceholder": "", "html": "<p>hidden</p>", "readonlyPlaceholder": ""}}, map[string]any{"id": "field-2", "name": "目标", "type": "Objective"}},
			"deadline":           map[string]any{},
			"enableStatistic":    true,
			"lastModifiedFormat": "2026-08-10 10:00",
			"lastModifier":       map[string]any{"dingUserId": "user-1", "id": "staff-1", "name": "Reviewer", "workNo": nil},
			"late":               float64(1),
			"notSubmitted":       float64(2),
			"onTime":             float64(3),
			"preferTime":         map[string]any{},
			"remind":             map[string]any{},
			"remindSize":         float64(6),
			"reportType":         "weeklyReport",
			"status":             "ONLINE",
			"templateId":         "template-1",
			"timeoutMinutes":     float64(30),
			"title":              "研发周报",
			"viewPermission": map[string]any{
				"deptReportLineManager": true,
				"sameDeptColleague":     false,
				"userReportLineManager": true,
			},
		}},
	}
}

func TestAgoalReportStatisticsProjectionPublishesStableSummaryOnly(t *testing.T) {
	data, meta, err := projectAgoalReportStatistics(validAgoalReportStatisticsResponse())
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if data["reportCoverageKnown"] != false {
		t.Fatalf("coverage = %#v", data["reportCoverageKnown"])
	}
	reports, ok := data["reports"].([]map[string]any)
	if !ok || len(reports) != 1 {
		t.Fatalf("reports = %#v", data["reports"])
	}
	report := reports[0]
	if report["templateId"] != "template-1" || report["onTime"] != int64(3) || report["remindSize"] != int64(6) {
		t.Fatalf("report = %#v", report)
	}
	for _, hidden := range []string{"content", "lastModifier", "lastModifiedFormat", "deadline", "preferTime", "remind"} {
		if _, exposed := report[hidden]; exposed {
			t.Fatalf("sensitive/opaque field %q was exposed: %#v", hidden, report)
		}
	}
	if meta == nil || meta.Count == nil || *meta.Count != 1 || meta.Pagination != nil {
		t.Fatalf("meta = %#v", meta)
	}
}

func TestAgoalReportStatisticsProjectionKeepsKnownEmptyDistinctFromUnknown(t *testing.T) {
	raw := validAgoalReportStatisticsResponse()
	raw["content"] = []any{}
	data, meta, err := projectAgoalReportStatistics(raw)
	if err != nil {
		t.Fatalf("known empty: %v", err)
	}
	if len(data["reports"].([]map[string]any)) != 0 || meta.Count == nil || *meta.Count != 0 {
		t.Fatalf("known empty data/meta = %#v %#v", data, meta)
	}
	delete(raw, "content")
	assertAgoalReportProjectionUnknown(t, func() error {
		_, _, err := projectAgoalReportStatistics(raw)
		return err
	}())
}

func TestAgoalReportStatisticsProjectionRejectsUnreviewedOrContradictoryShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown top field", func(raw map[string]any) { raw["complete"] = true }},
		{"wrong success type", func(raw map[string]any) { raw["success"] = "true" }},
		{"duplicate template", func(raw map[string]any) {
			rows := raw["content"].([]any)
			raw["content"] = append(rows, rows[0])
		}},
		{"fractional count", func(raw map[string]any) { reportRow(raw)["onTime"] = 1.5 }},
		{"inconsistent total", func(raw map[string]any) { reportRow(raw)["remindSize"] = float64(7) }},
		{"unknown report field", func(raw map[string]any) { reportRow(raw)["hasMore"] = false }},
		{"nonempty opaque deadline", func(raw map[string]any) { reportRow(raw)["deadline"] = map[string]any{"day": 1} }},
		{"unknown content type", func(raw map[string]any) { reportContentItem(raw, 0)["type"] = "RichText" }},
		{"unexpected objective value", func(raw map[string]any) { reportContentItem(raw, 1)["value"] = nil }},
		{"duplicate content id", func(raw map[string]any) {
			items := reportRow(raw)["content"].([]any)
			reportContentItem(raw, 1)["id"] = items[0].(map[string]any)["id"]
		}},
		{"malformed permission", func(raw map[string]any) {
			reportRow(raw)["viewPermission"].(map[string]any)["sameDeptColleague"] = "false"
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := validAgoalReportStatisticsResponse()
			tc.mutate(raw)
			_, _, err := projectAgoalReportStatistics(raw)
			assertAgoalReportProjectionUnknown(t, err)
		})
	}
}

func TestAgoalReportStatisticsDualValidatePreservesLegacyJSONExactlyOnce(t *testing.T) {
	raw, err := json.Marshal(validAgoalReportStatisticsResponse())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	run := func(state output.RolloutState) (string, int) {
		caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: string(raw)}}}
		installScriptedCaller(t, caller)
		root := newAgoalCommand()
		leaf, remaining, err := root.Find([]string{"report", "list-statistics"})
		if err != nil || len(remaining) != 0 {
			t.Fatalf("find leaf: remaining=%v err=%v", remaining, err)
		}
		output.SetCommandRollout(leaf, state)
		var stdout bytes.Buffer
		deps.Out.w = &stdout
		if err := executeFilterCoverage(t, root, "report", "list-statistics", "--keyword", "weekly"); err != nil {
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

func TestAgoalReportStatisticsUnifiedActiveEmitsOneFrameworkResult(t *testing.T) {
	raw, err := json.Marshal(validAgoalReportStatisticsResponse())
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
	leaf, remaining, err := agoal.Find([]string{"report", "list-statistics"})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("find leaf: remaining=%v err=%v", remaining, err)
	}
	output.SetCommandRollout(leaf, output.RolloutUnifiedActive)
	root.AddCommand(agoal)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"agoal", "report", "list-statistics", "--format", "json"})
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
	data, ok := envelope["data"].(map[string]any)
	if !ok || data["reportCoverageKnown"] != false {
		t.Fatalf("data = %#v", envelope["data"])
	}
	meta, ok := envelope["meta"].(map[string]any)
	if !ok || meta["count"] != float64(1) {
		t.Fatalf("meta = %#v", envelope["meta"])
	}
}

func TestAgoalReportStatisticsResultContractMatchesProjection(t *testing.T) {
	spec := agoalReportStatisticsResultSpec()
	if spec == nil || spec.NDJSON == nil || spec.NDJSON.RecordPath != "reports" {
		t.Fatalf("result spec = %#v", spec)
	}
	if _, err := contract.NormalizeResultSpec(spec, "agoal.report_list_statistics"); err != nil {
		t.Fatalf("normalize result: %v", err)
	}
}

func TestAgoalReportStatisticsRoutesExpectedArgumentsExactlyOnce(t *testing.T) {
	raw, err := json.Marshal(validAgoalReportStatisticsResponse())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: string(raw)}}}
	installScriptedCaller(t, caller)
	root := newAgoalCommand()
	ctx, _ := output.WithResultStore(context.Background())
	root.SetContext(ctx)
	if err := executeFilterCoverage(t, root, "report", "list-statistics", "--keyword", "weekly", "--request-id", "request-2"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if caller.calls != 1 || caller.tool != agoalReportStatisticsTool {
		t.Fatalf("calls/tool = %d/%q", caller.calls, caller.tool)
	}
	if want := map[string]any{"keyword": "weekly", "requestId": "request-2"}; !reflect.DeepEqual(caller.args, want) {
		t.Fatalf("args = %#v, want %#v", caller.args, want)
	}
}

func TestAgoalReportStatisticsPublishesReviewedAgentContract(t *testing.T) {
	root := newAgoalCommand()
	cmd, remaining, err := root.Find([]string{"report", "list-statistics"})
	if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
		t.Fatalf("report list-statistics is not an exact runnable leaf: cmd=%v remaining=%v err=%v", cmd, remaining, err)
	}
	final, ok := contractfinal.RuntimeContractFinal(cmd)
	if !ok {
		t.Fatal("agoal report list-statistics has no ContractFinal")
	}
	if final.Identity == nil || final.Identity.CanonicalPath != "agoal.report_list_statistics" || final.Identity.CLIPath != "agoal report list-statistics" {
		t.Fatalf("identity = %#v", final.Identity)
	}
	if final.Safety == nil || final.Safety.Effect != "read" || final.Safety.Risk != "low" || final.Safety.Confirmation != "not_required" || final.Safety.Idempotency != "idempotent" {
		t.Fatalf("safety = %#v", final.Safety)
	}
	if final.Interface == nil || final.Interface.Mode != "mcp" || final.Interface.Availability != "available" || final.Interface.Ref == nil || final.Interface.Ref.ProductID != "agoal" || final.Interface.Ref.RPCName != agoalReportStatisticsTool {
		t.Fatalf("interface = %#v", final.Interface)
	}
	if final.Selection == nil || strings.TrimSpace(final.Selection.AgentSummary) == "" || len(final.Selection.UseWhen) != 2 || len(final.Selection.AvoidWhen) != 2 || len(final.Selection.Examples) != 2 {
		t.Fatalf("selection = %#v", final.Selection)
	}
	got := map[string]string{}
	for _, parameter := range final.Parameters {
		got[parameter.Name] = parameter.Property
	}
	if want := map[string]string{"keyword": "keyword", "request-id": "requestId"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parameters = %#v, want %#v", got, want)
	}
}

func reportRow(raw map[string]any) map[string]any {
	return raw["content"].([]any)[0].(map[string]any)
}

func reportContentItem(raw map[string]any, index int) map[string]any {
	return reportRow(raw)["content"].([]any)[index].(map[string]any)
}

func assertAgoalReportProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
