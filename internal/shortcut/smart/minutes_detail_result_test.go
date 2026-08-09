package smart

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestMinutesDetailResultPreservesPartialArtifactFacts(t *testing.T) {
	payload, result, legacyErr, err := minutesDetailResult("minute-1", []minutesArtifactRead{
		{Name: "basic", Data: map[string]any{"title": "周会"}},
		{Name: "summary", Err: errors.New("gateway unavailable")},
	})
	if err != nil || payload != nil || legacyErr == nil {
		t.Fatalf("partial result payload=%#v legacyErr=%v err=%v", payload, legacyErr, err)
	}
	if result == nil || result.Outcome() != output.OutcomePartialFailure || result.ExitCode() != 7 {
		t.Fatalf("partial result=%#v, want partial_failure/7", result)
	}
	envelope := emitMinutesDetailResult(t, result)
	if envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
		t.Fatalf("partial envelope=%#v", envelope)
	}
	data, _ := envelope["data"].(map[string]any)
	succeeded, _ := data["succeeded"].([]any)
	failed, _ := data["failed"].([]any)
	if len(succeeded) != 1 || len(failed) != 1 {
		t.Fatalf("partial channels=%#v", data)
	}
	failedEntry, _ := failed[0].(map[string]any)
	failedError, _ := failedEntry["error"].(map[string]any)
	if failedEntry["id"] != "artifact:summary" || failedError["type"] != "api" {
		t.Fatalf("failed artifact=%#v", failedEntry)
	}
}

func TestMinutesDetailResultUsesSuccessOrFailureForTerminalCases(t *testing.T) {
	payload, result, legacyErr, err := minutesDetailResult("minute-1", []minutesArtifactRead{
		{Name: "basic", Data: map[string]any{"title": "周会"}},
		{Name: "keywords", Data: map[string]any{"items": []any{}}},
	})
	if err != nil || legacyErr != nil || result == nil || result.Outcome() != output.OutcomeSuccess {
		t.Fatalf("success classification payload=%#v result=%#v legacyErr=%v err=%v", payload, result, legacyErr, err)
	}
	if payload["artifact_count"] != 2 || payload["task_uuid"] != "minute-1" {
		t.Fatalf("success payload=%#v", payload)
	}

	_, result, legacyErr, err = minutesDetailResult("minute-1", []minutesArtifactRead{
		{Name: "basic", Err: errors.New("unavailable")},
	})
	if result != nil || legacyErr != nil || err == nil {
		t.Fatalf("all-failed result=%#v legacyErr=%v err=%v", result, legacyErr, err)
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Category != apperrors.CategoryAPI || typed.Operation != "minutes/detail" {
		t.Fatalf("all-failed error=%T %#v", err, err)
	}
	failed, _ := typed.Details["failed_artifacts"].([]map[string]any)
	if len(failed) != 1 || failed[0]["id"] != "artifact:basic" {
		t.Fatalf("all-failed details=%#v, want typed failed artifact", typed.Details)
	}
	failedInfo, _ := failed[0]["error"].(*output.ErrorInfo)
	if failedInfo == nil || failedInfo.Type != "api" || !failedInfo.Retryable {
		t.Fatalf("all-failed artifact error=%#v", failedInfo)
	}
}

func TestMinutesDetailPreservesTypedArtifactFailureGuidance(t *testing.T) {
	denied := apperrors.NewAuth("需要重新登录",
		apperrors.WithSubtype(apperrors.SubtypeUpstreamAuthenticationRequired),
		apperrors.WithHint("重新登录后只重试失败的 artifact。"),
		apperrors.WithActions("dws login"),
		apperrors.WithRetryable(false),
		apperrors.WithOperation("minutes/get_minutes_ai_summary"),
	)
	_, result, legacyErr, err := minutesDetailResult("minute-1", []minutesArtifactRead{
		{Name: "basic", Data: map[string]any{"title": "周会"}},
		{Name: "summary", Err: denied},
	})
	if err != nil || legacyErr == nil || result == nil || result.Outcome() != output.OutcomePartialFailure {
		t.Fatalf("typed partial result=%#v legacyErr=%v err=%v", result, legacyErr, err)
	}
	envelope := emitMinutesDetailResult(t, result)
	data, _ := envelope["data"].(map[string]any)
	failed, _ := data["failed"].([]any)
	if len(failed) != 1 {
		t.Fatalf("typed partial failed=%#v", data)
	}
	entry, _ := failed[0].(map[string]any)
	info, _ := entry["error"].(map[string]any)
	if info["type"] != "auth" || info["subtype"] != string(apperrors.SubtypeUpstreamAuthenticationRequired) {
		t.Fatalf("typed artifact category/subtype=%#v", info)
	}
	if _, present := info["retryable"]; present {
		t.Fatalf("non-retryable artifact must not serialize retryable=true: %#v", info)
	}
	if info["hint"] != "重新登录后只重试失败的 artifact。" || info["operation"] != "minutes/get_minutes_ai_summary" {
		t.Fatalf("typed artifact recovery guidance=%#v", info)
	}
}

func TestMinutesDetailIsUnifiedActive(t *testing.T) {
	if MinutesDetail.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("minutes detail rollout=%q, want unified_active", MinutesDetail.OutputRollout)
	}
}

func emitMinutesDetailResult(t *testing.T, result output.CommandResult) map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	exitCode, err := output.EmitResult(cmd, result)
	if err != nil || exitCode != result.ExitCode() || stderr.Len() != 0 {
		t.Fatalf("emit result exit=%d err=%v stderr=%q", exitCode, err, stderr.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode result stdout=%q err=%v", stdout.String(), err)
	}
	return envelope
}
