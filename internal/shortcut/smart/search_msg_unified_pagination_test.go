// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package smart

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	frameworkoutput "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	shortcutcore "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func runSearchMsgUnifiedResult(t *testing.T, fake *searchMsgExecutionCaller, args ...string) (map[string]any, int) {
	t.Helper()
	helpers.InitDeps(fake)
	declaration := SearchMsg
	declaration.OutputRollout = frameworkoutput.RolloutUnifiedActive
	cmd := corecmd.New(shortcutcore.FromShortcut(declaration))
	cmd.PersistentFlags().String("format", "json", "")
	ctx, _ := frameworkoutput.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("active command execution failed: %v", err)
	}
	exitCode, emitted, err := frameworkoutput.EmitStoredResult(cmd)
	if err != nil || !emitted {
		t.Fatalf("active result emission: code=%d emitted=%v err=%v", exitCode, emitted, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode unified envelope: %v\n%s", err, stdout.String())
	}
	if _, present := envelope["contract_version"]; present {
		t.Fatalf("unified envelope must not expose a protocol version: %#v", envelope)
	}
	assertNoLegacyProtocolMarker(t, envelope)
	return envelope, exitCode
}

func TestSearchMsgUnifiedPaginationOutcomes(t *testing.T) {
	t.Run("terminal cursor is exhausted and legacy fields do not leak", func(t *testing.T) {
		envelope, exitCode := runSearchMsgUnifiedResult(t, &searchMsgExecutionCaller{
			firstResponse: `{"result":{"messages":[{"openMessageId":"m1","content":"hit"}],"hasMore":false,"nextCursor":0}}`,
		}, "--format", "json", "--query", "x", "--no-enrich")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("terminal envelope=%#v exit=%d", envelope, exitCode)
		}
		data := searchMsgSuccessData(t, envelope)
		if _, present := data["contractVersion"]; present {
			t.Fatalf("legacy contractVersion leaked into active data: %#v", data)
		}
		for _, legacyKey := range []string{"partial", "hasMore", "nextCursor", "endpointExhausted", "paginationKnown"} {
			if _, present := data[legacyKey]; present {
				t.Fatalf("legacy pagination field %q leaked into active data: %#v", legacyKey, data)
			}
		}
		pagination := envelope["meta"].(map[string]any)["pagination"].(map[string]any)
		if pagination["endpoint_exhausted"] != true {
			t.Fatalf("terminal pagination=%#v", pagination)
		}
		if _, present := pagination["next_token"]; present {
			t.Fatalf("terminal pagination unexpectedly has a continuation=%#v", pagination)
		}
	})

	t.Run("local page budget exposes a usable continuation without false failure", func(t *testing.T) {
		envelope, exitCode := runSearchMsgUnifiedResult(t, &searchMsgExecutionCaller{
			firstResponse: `{"result":{"messages":[{"openMessageId":"m1","content":"hit"}],"hasMore":true,"nextCursor":"c2"}}`,
		}, "--format", "json", "--query", "x", "--no-enrich", "--page-all", "--page-limit", "1")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("continuation envelope=%#v exit=%d", envelope, exitCode)
		}
		pagination := envelope["meta"].(map[string]any)["pagination"].(map[string]any)
		if pagination["endpoint_exhausted"] != false || pagination["next_token"] != "c2" || pagination["pages"] != float64(1) {
			t.Fatalf("continuation pagination=%#v", pagination)
		}
	})

	t.Run("single page without endpoint evidence stays successful but unknown", func(t *testing.T) {
		envelope, exitCode := runSearchMsgUnifiedResult(t, &searchMsgExecutionCaller{omitPagination: true}, "--format", "json", "--query", "x", "--no-enrich")
		if exitCode != 0 || envelope["ok"] != true || envelope["outcome"] != "success" {
			t.Fatalf("unknown evidence envelope=%#v exit=%d", envelope, exitCode)
		}
		data := searchMsgSuccessData(t, envelope)
		if data["pagination_known"] != false || data["indexCoverageKnown"] != false {
			t.Fatalf("unknown evidence data=%#v", data)
		}
		if meta, _ := envelope["meta"].(map[string]any); meta != nil && meta["pagination"] != nil {
			t.Fatalf("unknown endpoint evidence must not emit pagination meta: %#v", envelope)
		}
	})

	t.Run("page-all without endpoint evidence is partial", func(t *testing.T) {
		envelope, exitCode := runSearchMsgUnifiedResult(t, &searchMsgExecutionCaller{omitPagination: true}, "--format", "json", "--query", "x", "--no-enrich", "--page-all")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("unknown full scan envelope=%#v exit=%d", envelope, exitCode)
		}
		failed := envelope["data"].(map[string]any)["failed"].([]any)
		if len(failed) != 1 || failed[0].(map[string]any)["error"].(map[string]any)["subtype"] != "pagination_inconsistent" {
			t.Fatalf("unknown full scan failed details=%#v", failed)
		}
	})

	t.Run("later read failure retains successful page", func(t *testing.T) {
		envelope, exitCode := runSearchMsgUnifiedResult(t, &searchMsgExecutionCaller{failSecondPage: true}, "--format", "json", "--query", "x", "--no-enrich", "--page-all")
		if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
			t.Fatalf("later failure envelope=%#v exit=%d", envelope, exitCode)
		}
		data := envelope["data"].(map[string]any)
		if len(data["succeeded"].([]any)) != 1 || len(data["failed"].([]any)) != 1 {
			t.Fatalf("later failure details=%#v", data)
		}
	})

	t.Run("typed enrichment failure remains a typed partial item", func(t *testing.T) {
		denied := apperrors.NewAuth("需要重新登录",
			apperrors.WithSubtype(apperrors.SubtypeUpstreamAuthenticationRequired),
			apperrors.WithHint("重新登录后只重试消息详情富化。"),
			apperrors.WithActions("dws login"),
			apperrors.WithRetryable(false),
		)
		envelope, exitCode := runSearchMsgUnifiedResult(t, &searchMsgExecutionCaller{
			firstResponse: `{"result":{"messages":[{"openMessageId":"m1","content":"hit"}],"hasMore":false,"nextCursor":0}}`,
			enrichmentErr: denied,
		}, "--format", "json", "--query", "x")
		if exitCode != 7 || envelope["outcome"] != "partial_failure" {
			t.Fatalf("typed enrichment envelope=%#v exit=%d", envelope, exitCode)
		}
		data := envelope["data"].(map[string]any)
		failed := data["failed"].([]any)
		if len(failed) != 1 {
			t.Fatalf("typed enrichment failures=%#v", failed)
		}
		info := failed[0].(map[string]any)["error"].(map[string]any)
		if info["type"] != "auth" || info["subtype"] != string(apperrors.SubtypeUpstreamAuthenticationRequired) {
			t.Fatalf("typed enrichment error=%#v", info)
		}
		if _, present := info["retryable"]; present {
			t.Fatalf("retryable=false must not become retryable=true: %#v", info)
		}
		if info["hint"] != "重新登录后只重试消息详情富化。" {
			t.Fatalf("typed enrichment recovery=%#v", info)
		}
	})

	t.Run("missing enrichment rows are projection partial items", func(t *testing.T) {
		envelope, exitCode := runSearchMsgUnifiedResult(t, &searchMsgExecutionCaller{
			firstResponse: `{"result":{"messages":[{"openMessageId":"m1"},{"openMessageId":"m2"}],"hasMore":false,"nextCursor":0}}`,
			omitMgetItem:  true,
		}, "--format", "json", "--query", "x")
		if exitCode != 7 || envelope["outcome"] != "partial_failure" {
			t.Fatalf("missing enrichment envelope=%#v exit=%d", envelope, exitCode)
		}
		failed := envelope["data"].(map[string]any)["failed"].([]any)
		if len(failed) != 1 {
			t.Fatalf("missing enrichment failures=%#v", failed)
		}
		info := failed[0].(map[string]any)["error"].(map[string]any)
		if info["subtype"] != string(apperrors.SubtypeProjectionUnknown) || info["stage"] != "message_enrichment_projection" {
			t.Fatalf("missing enrichment projection=%#v", info)
		}
	})

	t.Run("unknown response shape is typed failure rather than empty success", func(t *testing.T) {
		envelope, exitCode := runSearchMsgUnifiedResult(t, &searchMsgExecutionCaller{
			firstResponse: `{"result":{"hasMore":false,"messages":[{}]}}`,
		}, "--format", "json", "--query", "x", "--no-enrich")
		if exitCode != 1 || envelope["ok"] != false || envelope["outcome"] != "failure" {
			t.Fatalf("projection envelope=%#v exit=%d", envelope, exitCode)
		}
		if errorInfo := envelope["error"].(map[string]any); errorInfo["subtype"] != "projection_unknown" {
			t.Fatalf("projection error=%#v", errorInfo)
		}
	})
}
