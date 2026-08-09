package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func validAgoalUserRulesResponse() map[string]any {
	return map[string]any{
		"code": nil, "message": nil, "requestId": "request-1", "success": true,
		"content": map[string]any{
			"preference": map[string]any{"perfTaskId": nil, "periodId": "period-current", "ruleId": "rule-1"},
			"rules": []any{map[string]any{
				"canRelatedUsers": true,
				"category":        "OBJECTIVE",
				"history":         false,
				"id":              "rule-1",
				"lastModified":    float64(1720000000000),
				"matchedCount":    float64(2),
				"perfTaskFilter": map[string]any{
					"currentPerfTasks": []any{},
					"historyPerfTasks": []any{},
				},
				"periodFilter": map[string]any{
					"currentPeriods": []any{map[string]any{
						"endDate": float64(1729999999999), "id": "period-current", "nameCn": "当前周期", "nameEN": "Current", "startDate": float64(1720000000000),
					}},
					"defaultPeriodIds": []any{"period-current"},
					"historyPeriods": []any{map[string]any{
						"endDate": float64(1719999999999), "id": "period-history", "nameCn": "历史周期", "nameEN": "History", "startDate": float64(1710000000000),
					}},
					"lastObjectivePeriodId": "period-current",
					"preferPeriodIds":       []any{"period-current"},
				},
				"reviewTag":      "NONE",
				"ruleContent":    nil,
				"ruleName":       "年度目标",
				"status":         "ACTIVE",
				"type":           "RULE",
				"weightCheckTag": false,
			}},
		},
	}
}

func TestAgoalUserRulesProjectionPublishesStableRuleAndPeriodHandles(t *testing.T) {
	data, meta, err := projectAgoalUserRules(validAgoalUserRulesResponse())
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if data["ruleCoverageKnown"] != false {
		t.Fatalf("coverage = %#v", data["ruleCoverageKnown"])
	}
	rules, ok := data["rules"].([]map[string]any)
	if !ok || len(rules) != 1 || rules[0]["ruleId"] != "rule-1" {
		t.Fatalf("rules = %#v", data["rules"])
	}
	periods := rules[0]["periods"].(map[string]any)
	current := periods["current"].([]map[string]any)
	if len(current) != 1 || current[0]["periodId"] != "period-current" || current[0]["nameEn"] != "Current" {
		t.Fatalf("current periods = %#v", current)
	}
	if got := periods["preferredPeriodIds"]; !reflect.DeepEqual(got, []string{"period-current"}) {
		t.Fatalf("preferred ids = %#v", got)
	}
	if meta == nil || meta.Count == nil || *meta.Count != 1 || meta.Pagination != nil {
		t.Fatalf("meta = %#v", meta)
	}
}

func TestAgoalUserRulesProjectionKeepsKnownEmptyDistinctFromUnknown(t *testing.T) {
	raw := validAgoalUserRulesResponse()
	raw["content"].(map[string]any)["rules"] = []any{}
	data, meta, err := projectAgoalUserRules(raw)
	if err != nil {
		t.Fatalf("known empty: %v", err)
	}
	if len(data["rules"].([]map[string]any)) != 0 || meta.Count == nil || *meta.Count != 0 {
		t.Fatalf("known empty data/meta = %#v %#v", data, meta)
	}
	delete(raw["content"].(map[string]any), "rules")
	assertAgoalRulesProjectionUnknown(t, func() error {
		_, _, err := projectAgoalUserRules(raw)
		return err
	}())
}

func TestAgoalUserRulesProjectionNormalizesEmptyOptionalLastPeriod(t *testing.T) {
	raw := validAgoalUserRulesResponse()
	rule := raw["content"].(map[string]any)["rules"].([]any)[0].(map[string]any)
	rule["periodFilter"].(map[string]any)["lastObjectivePeriodId"] = ""
	data, _, err := projectAgoalUserRules(raw)
	if err != nil {
		t.Fatalf("project empty optional id: %v", err)
	}
	periods := data["rules"].([]map[string]any)[0]["periods"].(map[string]any)
	if _, exists := periods["lastObjectivePeriodId"]; exists {
		t.Fatalf("empty optional id was published: %#v", periods)
	}
}

func TestAgoalUserRulesProjectionRejectsUnreviewedOrContradictoryShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown top field", func(raw map[string]any) { raw["complete"] = true }},
		{"wrong success type", func(raw map[string]any) { raw["success"] = "true" }},
		{"duplicate rule id", func(raw map[string]any) {
			rules := raw["content"].(map[string]any)["rules"].([]any)
			raw["content"].(map[string]any)["rules"] = append(rules, rules[0])
		}},
		{"fractional matched count", func(raw map[string]any) {
			raw["content"].(map[string]any)["rules"].([]any)[0].(map[string]any)["matchedCount"] = 1.5
		}},
		{"unknown period reference", func(raw map[string]any) {
			rule := raw["content"].(map[string]any)["rules"].([]any)[0].(map[string]any)
			rule["periodFilter"].(map[string]any)["preferPeriodIds"] = []any{"not-observed"}
		}},
		{"nonempty performance tasks", func(raw map[string]any) {
			rule := raw["content"].(map[string]any)["rules"].([]any)[0].(map[string]any)
			rule["perfTaskFilter"].(map[string]any)["currentPerfTasks"] = []any{map[string]any{"id": "unreviewed"}}
		}},
		{"nonnull rule content", func(raw map[string]any) {
			raw["content"].(map[string]any)["rules"].([]any)[0].(map[string]any)["ruleContent"] = map[string]any{}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := validAgoalUserRulesResponse()
			tc.mutate(raw)
			_, _, err := projectAgoalUserRules(raw)
			assertAgoalRulesProjectionUnknown(t, err)
		})
	}
}

func TestAgoalUserRulesDualValidatePreservesLegacyJSONExactlyOnce(t *testing.T) {
	raw, err := json.Marshal(validAgoalUserRulesResponse())
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	run := func(state output.RolloutState) (string, int) {
		caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: string(raw)}}}
		installScriptedCaller(t, caller)
		root := newAgoalCommand()
		leaf, remaining, err := root.Find([]string{"user", "rules"})
		if err != nil || len(remaining) != 0 {
			t.Fatalf("find leaf: remaining=%v err=%v", remaining, err)
		}
		output.SetCommandRollout(leaf, state)
		var stdout bytes.Buffer
		deps.Out.w = &stdout
		if err := executeFilterCoverage(t, root, "user", "rules"); err != nil {
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

func TestAgoalUserRulesResultContractMatchesProjection(t *testing.T) {
	spec := agoalUserRulesResultSpec()
	if spec == nil || spec.NDJSON == nil || spec.NDJSON.RecordPath != "rules" {
		t.Fatalf("result spec = %#v", spec)
	}
	if _, err := contract.NormalizeResultSpec(spec, "agoal.user_rules"); err != nil {
		t.Fatalf("normalize result: %v", err)
	}
}

func assertAgoalRulesProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
