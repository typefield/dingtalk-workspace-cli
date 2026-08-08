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

package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/spf13/cobra"
)

// TestReportEntryListResolveListSnakeShape guards against projection-data-loss:
// get_received_report_list / get_send_report_list nest the list under
// result.report_list (snake_case); the resolver must probe "report_list" or
// +inbox-list / +outbox-list silently return empty despite the backend having
// reports.
func TestReportEntryListResolveListSnakeShape(t *testing.T) {
	const raw = `{"result":{"report_list":[
		{"reportId":"r1","templateName":"daily report"},
		{"reportId":"r2","templateName":"weekly report"}
	]}}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	got, known := reportEntryListResolveList(data)
	if !known || len(got) != 2 {
		t.Fatalf("lower/upper mismatch: result.report_list has 2 entries, resolver returned %d", len(got))
	}
}

func TestReportListProjectionSeparatesKnownEmptyFromUnknown(t *testing.T) {
	reports, err := reportEntryListProject(map[string]any{"report_list": []any{}})
	if err != nil {
		t.Fatalf("known empty report list returned error: %v", err)
	}
	if reports == nil || len(reports) != 0 {
		t.Fatalf("known empty report list = %#v, want non-nil empty list", reports)
	}

	_, err = reportEntryListProject(map[string]any{"unexpected": []any{}})
	assertReportProjectionUnknown(t, err)
}

func TestReportListProjectionRejectsUnknownRows(t *testing.T) {
	_, err := reportEntryListProject(map[string]any{"items": []any{"opaque"}})
	assertReportProjectionUnknown(t, err)

	_, err = reportEntryListProject(map[string]any{"items": []any{map[string]any{"opaque": true}}})
	assertReportProjectionUnknown(t, err)
}

func TestReportListProjectionAcceptsNestedKnownContainer(t *testing.T) {
	reports, err := reportEntryListProject(map[string]any{
		"result": map[string]any{"report_list": []any{map[string]any{"reportId": "r1", "creatorName": "Alice"}}},
	})
	if err != nil {
		t.Fatalf("nested report list returned error: %v", err)
	}
	if len(reports) != 1 || reports[0]["reportId"] != "r1" || reports[0]["creatorName"] != "Alice" {
		t.Fatalf("nested report projection = %#v", reports)
	}
}

func TestReportListPaginationMapsContinuationAndTerminalPages(t *testing.T) {
	page, known, err := reportListPagination(map[string]any{
		"result": map[string]any{"hasMore": true, "nextCursor": float64(42)},
	})
	if err != nil || !known || page.EndpointExhausted || page.NextToken != "42" {
		t.Fatalf("continuation pagination = %#v, known=%v, err=%v", page, known, err)
	}

	page, known, err = reportListPagination(map[string]any{"hasMore": false})
	if err != nil || !known || !page.EndpointExhausted || page.NextToken != "" {
		t.Fatalf("terminal pagination = %#v, known=%v, err=%v", page, known, err)
	}
	for _, sentinel := range []any{0, float64(0), "0"} {
		page, known, err = reportListPagination(map[string]any{"hasMore": false, "nextCursor": sentinel})
		if err != nil || !known || !page.EndpointExhausted || page.NextToken != "" {
			t.Fatalf("terminal pagination sentinel=%#v => %#v, known=%v, err=%v", sentinel, page, known, err)
		}
	}
	_, _, err = reportListPagination(map[string]any{"hasMore": true, "nextCursor": 0})
	assertReportPaginationError(t, err)

	payload, result, err := reportListResult(map[string]any{"hasMore": true, "nextCursor": "next"}, []map[string]any{})
	if err != nil || payload["hasMore"] != true || payload["nextCursor"] != "next" || result.Outcome() != output.OutcomeSuccess {
		t.Fatalf("pagination result = payload=%#v outcome=%v err=%v", payload, result.Outcome(), err)
	}
	env, err := output.EnvelopeFromResult(result)
	if err != nil || env.Meta == nil || env.Meta.Count == nil || *env.Meta.Count != 0 || env.Meta.Pagination == nil || env.Meta.Pagination.EndpointExhausted || env.Meta.Pagination.NextToken != "next" {
		t.Fatalf("unified pagination envelope = %#v, err=%v", env, err)
	}

	var stdout bytes.Buffer
	cmd := &cobra.Command{Use: "report"}
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.Flags().String("format", "json", "")
	if _, err := output.EmitResult(cmd, result); err != nil {
		t.Fatalf("emit unified result: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &wire); err != nil {
		t.Fatalf("unmarshal unified wire: %v; output=%q", err, stdout.String())
	}
	if _, present := wire["contract_version"]; present || wire["outcome"] != "success" || wire["ok"] != true {
		t.Fatalf("unexpected unified wire = %#v", wire)
	}
}

func TestReportListPaginationRejectsAmbiguousOrContradictoryEvidence(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"continuation without cursor": {"hasMore": true},
		"terminal with cursor":        {"hasMore": false, "nextCursor": "next"},
		"cursor without has more":     {"nextCursor": "next"},
		"nested contradiction": {
			"hasMore": false,
			"result":  map[string]any{"hasMore": true, "nextCursor": "next"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := reportListPagination(data)
			assertReportPaginationError(t, err)
		})
	}
}

func TestReportListShortcutsAreV2ActiveOnlyAfterPaginationProjection(t *testing.T) {
	for name, declaration := range map[string]struct {
		rollout output.RolloutState
	}{
		"inbox":  {rollout: InboxList.OutputRollout},
		"outbox": {rollout: OutboxList.OutputRollout},
	} {
		if declaration.rollout != output.RolloutUnifiedActive {
			t.Errorf("%s rollout = %s, want unified_active", name, declaration.rollout)
		}
	}
}

func assertReportProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}

func assertReportPaginationError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("pagination unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "pagination_inconsistent" || typed.Retryable {
		t.Fatalf("pagination error = %T %#v, want non-retryable pagination_inconsistent", err, err)
	}
}
