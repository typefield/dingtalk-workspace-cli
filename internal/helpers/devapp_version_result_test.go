// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestDevAppVersionResultSpecsSeparateTerminalAndPendingOperations(t *testing.T) {
	if got, want := DevAppVersionCreateResultSpec().Outcomes, []contract.ResultOutcome{
		contract.ResultOutcomeSuccess,
		contract.ResultOutcomePending,
		contract.ResultOutcomePartialFailure,
		contract.ResultOutcomeFailure,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("create outcomes=%#v, want %#v", got, want)
	}
	if got, want := DevAppVersionPublishResultSpec().Outcomes, []contract.ResultOutcome{
		contract.ResultOutcomeSuccess,
		contract.ResultOutcomePending,
		contract.ResultOutcomePartialFailure,
		contract.ResultOutcomeFailure,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("publish outcomes=%#v, want %#v", got, want)
	}
	if got, want := DevAppVersionPrecheckResultSpec().Outcomes, []contract.ResultOutcome{
		contract.ResultOutcomeSuccess,
		contract.ResultOutcomeFailure,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("precheck outcomes=%#v, want %#v", got, want)
	}
}

func TestDevAppVersionCreateRequiresStableIDAndDoesNotClaimReadback(t *testing.T) {
	params := map[string]any{
		"unifiedAppId": "app-1",
		"version":      "1.2.3",
		"desc":         "release",
	}
	result := DevAppCommandResultFromPayload(devAppVersionCreateTool, map[string]any{
		"unifiedAppId":  "app-1",
		"versionId":     "version-1",
		"versionStatus": "INIT",
	}, false, params)
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatal(err)
	}
	if env.Outcome != output.OutcomeSuccess || result.ExitCode() != 0 {
		t.Fatalf("envelope=%+v rc=%d", env, result.ExitCode())
	}
	data, _ := env.Data.(map[string]any)
	requested, _ := data["requested"].(map[string]any)
	verification, _ := data["verification"].(map[string]any)
	if requested["operation"] != "create_version" || requested["unifiedAppId"] != "app-1" ||
		requested["version"] != "1.2.3" || requested["desc"] != "release" {
		t.Fatalf("requested=%#v", requested)
	}
	if verification["state"] != "not_verified" || !strings.Contains(verification["next_command"].(string), "version-1") {
		t.Fatalf("verification=%#v", verification)
	}

	unknown := DevAppCommandResultFromPayload(devAppVersionCreateTool, map[string]any{
		"unifiedAppId": "app-1", "accepted": true,
	}, false, params)
	assertDevAppVersionProjectionUnknown(t, unknown, "stable versionId")
}

func TestDevAppVersionPublishClassifiesTerminalPendingAndUnknown(t *testing.T) {
	params := map[string]any{
		"unifiedAppId":       "app-1",
		"versionId":          "version-1",
		"precheckOnly":       false,
		"confirmedSensitive": true,
	}
	t.Run("direct release remains explicitly unverified", func(t *testing.T) {
		result := DevAppCommandResultFromPayload(devAppVersionPublishTool, map[string]any{
			"published": true, "versionStatus": "RELEASE",
		}, false, params)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := env.Data.(map[string]any)
		requested, _ := data["requested"].(map[string]any)
		verification, _ := data["verification"].(map[string]any)
		if env.Outcome != output.OutcomeSuccess || requested["operation"] != "publish_version" ||
			requested["versionId"] != "version-1" || requested["confirmedSensitive"] != true ||
			verification["state"] != "not_verified" {
			t.Fatalf("envelope=%+v data=%#v", env, data)
		}
	})

	t.Run("submitted approval is resumable with request identity", func(t *testing.T) {
		result := DevAppCommandResultFromPayload(devAppVersionPublishTool, map[string]any{
			"approvalSubmitted": true, "published": false,
		}, false, params)
		env, err := output.EnvelopeFromResult(result)
		if err != nil {
			t.Fatal(err)
		}
		if env.Outcome != output.OutcomePending || env.Meta == nil || env.Meta.Operation == nil ||
			env.Meta.Operation.ID != "version-1" || env.Meta.Operation.State != "approval_submitted" ||
			!strings.Contains(env.Meta.Operation.NextCommand, "--unified-app-id app-1") {
			t.Fatalf("envelope=%+v", env)
		}
	})

	t.Run("opaque acknowledgement is not terminal success", func(t *testing.T) {
		result := DevAppCommandResultFromPayload(devAppVersionPublishTool, map[string]any{
			"accepted": true,
		}, false, params)
		assertDevAppVersionProjectionUnknown(t, result, "no terminal publish evidence")
	})
}

func TestDevAppVersionStatusFailureAndPendingWinOverGenericStatus(t *testing.T) {
	t.Run("process failure wins", func(t *testing.T) {
		result := DevAppCommandResultFromPayload(devAppVersionPublishTool, map[string]any{
			"status": "SUCCESS", "versionStatus": "AUDIT", "processStatus": "FAIL",
		}, false, map[string]any{"unifiedAppId": "app-1", "versionId": "version-1"})
		if result.Outcome() != output.OutcomeFailure {
			t.Fatalf("outcome=%q", result.Outcome())
		}
	})
	t.Run("version audit wins over generic success", func(t *testing.T) {
		result := DevAppCommandResultFromPayload(devAppVersionPublishTool, map[string]any{
			"status": "SUCCESS", "versionStatus": "AUDIT",
		}, false, map[string]any{"unifiedAppId": "app-1", "versionId": "version-1"})
		if result.Outcome() != output.OutcomePending {
			t.Fatalf("outcome=%q", result.Outcome())
		}
	})
}

func TestDevAppVersionApprovalPrecheckIsTerminalButStructured(t *testing.T) {
	result := DevAppCommandResultFromPayload(devAppVersionPublishTool, map[string]any{
		"requiresApproval": false,
		"publishable":      true,
		"approvalMode":     "AUTOMATIC",
	}, false, map[string]any{
		"precheckOnly": true, "unifiedAppId": "app-1", "versionId": "version-1",
	})
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := env.Data.(map[string]any)
	requested, _ := data["requested"].(map[string]any)
	if env.Outcome != output.OutcomeSuccess || requested["operation"] != "check_approval" ||
		requested["versionId"] != "version-1" {
		t.Fatalf("envelope=%+v data=%#v", env, data)
	}
}

func assertDevAppVersionProjectionUnknown(t *testing.T, result output.CommandResult, messagePart string) {
	t.Helper()
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatal(err)
	}
	if env.Outcome != output.OutcomeFailure || env.Error == nil ||
		env.Error.Subtype != string(apperrors.SubtypeProjectionUnknown) ||
		env.Error.ExecutionStarted == nil || !*env.Error.ExecutionStarted || env.Error.Retryable ||
		!strings.Contains(env.Error.Message, messagePart) {
		t.Fatalf("envelope=%+v", env)
	}
}
