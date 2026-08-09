// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package smart

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestReportLatestProjectionDistinguishesEmptyFromUnknown(t *testing.T) {
	row, id, err := shortcutReportLatestPick(map[string]any{"result": map[string]any{"list": []any{}}})
	if err != nil || row != nil || id != "" {
		t.Fatalf("known empty row=%#v id=%q err=%v", row, id, err)
	}

	for name, payload := range map[string]map[string]any{
		"unknown container":    {"status": "ok"},
		"wrong container type": {"result": map[string]any{"list": "not-an-array"}},
		"non-object row":       {"result": map[string]any{"list": []any{"bad"}}},
		"missing stable id":    {"result": map[string]any{"list": []any{map[string]any{"title": "日报"}}}},
		"partial time coverage": {"result": map[string]any{"list": []any{
			map[string]any{"reportId": "r1", "createTime": float64(100)},
			map[string]any{"reportId": "r2"},
		}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := shortcutReportLatestPick(payload)
			assertReportLatestProjectionUnknown(t, err)
		})
	}
}

func TestReportLatestProjectionSelectsNewestStableEntry(t *testing.T) {
	newest := map[string]any{"reportId": "r-new", "createTime": "200"}
	row, id, err := shortcutReportLatestPick(map[string]any{"result": map[string]any{"list": []any{
		map[string]any{"reportId": "r-old", "createTime": float64(100)},
		newest,
	}}})
	if err != nil || id != "r-new" || row["reportId"] != "r-new" {
		t.Fatalf("newest row=%#v id=%q err=%v", row, id, err)
	}

	row, id, err = shortcutReportLatestPick(map[string]any{"list": []any{
		map[string]any{"reportId": "server-first"},
		map[string]any{"reportId": "server-second"},
	}})
	if err != nil || id != "server-first" || row["reportId"] != "server-first" {
		t.Fatalf("server-order row=%#v id=%q err=%v", row, id, err)
	}
}

func assertReportLatestProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed == nil {
		t.Fatalf("projection error=%T %v", err, err)
	}
	if typed.StableSubtype != string(apperrors.SubtypeProjectionUnknown) || typed.Retryable {
		t.Fatalf("projection error=%#v", typed)
	}
	if typed.Operation != "report/get_send_report_list" || typed.FailureStage != "latest_report_projection" {
		t.Fatalf("projection recovery context=%#v", typed)
	}
}
