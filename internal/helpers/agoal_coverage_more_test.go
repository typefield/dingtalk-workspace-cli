package helpers

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
)

func TestAgoalUserRulesPublishesReviewedAgentContract(t *testing.T) {
	root := newAgoalCommand()
	cmd, remaining, err := root.Find([]string{"user", "rules"})
	if err != nil || cmd == nil || !cmd.Runnable() || len(remaining) != 0 {
		t.Fatalf("user rules is not an exact runnable leaf: cmd=%v remaining=%v err=%v", cmd, remaining, err)
	}
	final, ok := contractfinal.RuntimeContractFinal(cmd)
	if !ok {
		t.Fatal("agoal user rules has no ContractFinal")
	}
	if final.Identity == nil || final.Identity.CanonicalPath != "agoal.user_rules" || final.Identity.CLIPath != "agoal user rules" {
		t.Fatalf("identity = %#v", final.Identity)
	}
	if final.Safety == nil || final.Safety.Effect != "read" || final.Safety.Risk != "low" || final.Safety.Confirmation != "not_required" || final.Safety.Idempotency != "idempotent" {
		t.Fatalf("safety = %#v", final.Safety)
	}
	if final.Interface == nil || final.Interface.Mode != "mcp" || final.Interface.Availability != "available" || final.Interface.Ref == nil || final.Interface.Ref.ProductID != "agoal" || final.Interface.Ref.RPCName != "get_user_rules" {
		t.Fatalf("interface = %#v", final.Interface)
	}
	if final.Selection == nil || strings.TrimSpace(final.Selection.AgentSummary) == "" || len(final.Selection.UseWhen) == 0 || len(final.Selection.AvoidWhen) == 0 || len(final.Selection.Examples) != 2 {
		t.Fatalf("selection = %#v", final.Selection)
	}
	got := map[string]string{}
	for _, parameter := range final.Parameters {
		got[parameter.Name] = parameter.Property
	}
	if want := map[string]string{"user-id": "dingUserId", "request-id": "requestId"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parameters = %#v, want %#v", got, want)
	}
}

func TestAgoalUserRulesRoutesExactlyOnce(t *testing.T) {
	caller := &scriptedToolCaller{format: "json"}
	installScriptedCaller(t, caller)
	if err := executeFilterCoverage(t, newAgoalCommand(), "user", "rules", "--user-id", "user-1", "--request-id", "request-1"); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if caller.calls != 1 || caller.tool != "get_user_rules" {
		t.Fatalf("calls/tool = %d/%q", caller.calls, caller.tool)
	}
	if want := map[string]any{"dingUserId": "user-1", "requestId": "request-1"}; !reflect.DeepEqual(caller.args, want) {
		t.Fatalf("args = %#v, want %#v", caller.args, want)
	}
}

func TestCrossPlatformCoverageAgoalUpdateRemainingCoverage(t *testing.T) {
	installScriptedCaller(t, &scriptedToolCaller{dry: true})
	if err := executeFilterCoverage(t, newAgoalCommand(),
		"strategy", "update", "--profile-id", "profile", "--content", `[]`, "--request-id", "request",
	); err != nil {
		t.Fatalf("strategy update: %v", err)
	}
	if err := executeFilterCoverage(t, newAgoalCommand(),
		"contract", "update", "--contract-id", "contract", "--dimensions", `[]`,
		"--request-id", "request", "--audit-config", `{}`, "--objective-template", `{}`,
	); err != nil {
		t.Fatalf("contract update: %v", err)
	}
	if err := executeFilterCoverage(t, newAgoalCommand(),
		"scorecard", "update", "--dept-id", "dept", "--selected-time", "bad", "--id", "card",
		"--tracking-period-type", "MONTHLY", "--content", `[]`,
	); err == nil {
		t.Fatal("invalid scorecard time returned nil")
	}
	if err := executeFilterCoverage(t, newAgoalCommand(),
		"scorecard", "update", "--dept-id", "dept", "--selected-time", "2026-01-01T00:00:00", "--id", "card",
		"--tracking-period-type", "MONTHLY", "--content", `[]`, "--request-id", "request",
	); err != nil {
		t.Fatalf("scorecard update: %v", err)
	}
}

func TestCrossPlatformCoverageAgoalLocationFallbackCoverage(t *testing.T) {
	original := agoalLoadLocation
	agoalLoadLocation = func(string) (*time.Location, error) { return nil, errors.New("zoneinfo unavailable") }
	t.Cleanup(func() { agoalLoadLocation = original })
	if got := shanghaiLocation(); got == nil || got.String() != "CST" {
		t.Fatalf("fallback location = %v", got)
	}
}
