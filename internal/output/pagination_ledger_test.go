// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package output

import (
	"errors"
	"testing"
)

func boolEvidence(value bool) *bool { return &value }

func TestPageLedgerExhaustedAndContinuationMetadata(t *testing.T) {
	t.Run("exhausted", func(t *testing.T) {
		ledger, err := NewPageLedger(3)
		if err != nil {
			t.Fatal(err)
		}
		if err := ledger.ObservePage(PageEvidence{Cursor: "0", Items: 2, Data: []any{"a", "b"}, HasMore: boolEvidence(false)}); err != nil {
			t.Fatal(err)
		}
		result, err := ledger.Result(map[string]any{"items": []any{"a", "b"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateResult(result); err != nil {
			t.Fatal(err)
		}
		env := result.envelope()
		if env.Meta == nil || env.Meta.Pagination == nil || !env.Meta.Pagination.EndpointExhausted {
			t.Fatalf("pagination = %#v, want exhausted", env.Meta)
		}
		if env.Meta.Pagination.Pages != 1 || env.Meta.Pagination.Items != 2 || env.Meta.Pagination.NextToken != "" {
			t.Fatalf("pagination = %#v", env.Meta.Pagination)
		}
	})

	t.Run("continuation", func(t *testing.T) {
		ledger, _ := NewPageLedger(3)
		if err := ledger.ObservePage(PageEvidence{Cursor: "0", Items: 1, HasMore: boolEvidence(true), NextToken: "2"}); err != nil {
			t.Fatal(err)
		}
		result, err := ledger.Result([]any{"a"})
		if err != nil {
			t.Fatal(err)
		}
		env := result.envelope()
		if env.Meta == nil || env.Meta.Pagination == nil || env.Meta.Pagination.EndpointExhausted || env.Meta.Pagination.NextToken != "2" {
			t.Fatalf("pagination = %#v, want resumable token 2", env.Meta)
		}
	})
}

func TestPageLedgerUnknownOmitsPagination(t *testing.T) {
	ledger, _ := NewPageLedger(2)
	if err := ledger.ObservePage(PageEvidence{Cursor: "0", Items: 1}); err != nil {
		t.Fatal(err)
	}
	if ledger.State() != PageStateUnknown {
		t.Fatalf("state = %q, want unknown", ledger.State())
	}
	result, err := ledger.Result(map[string]any{"pagination_known": false})
	if err != nil {
		t.Fatal(err)
	}
	env := result.envelope()
	if env.Meta != nil && env.Meta.Pagination != nil {
		t.Fatalf("unknown evidence emitted pagination: %#v", env.Meta.Pagination)
	}
}

func TestPageLedgerRejectsContradictionsAndCursorLoops(t *testing.T) {
	tests := []struct {
		name     string
		evidence PageEvidence
	}{
		{name: "has more without token", evidence: PageEvidence{Cursor: "0", HasMore: boolEvidence(true)}},
		{name: "exhausted with token", evidence: PageEvidence{Cursor: "0", HasMore: boolEvidence(false), NextToken: "2"}},
		{name: "stalled token", evidence: PageEvidence{Cursor: "2", HasMore: boolEvidence(true), NextToken: "2"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ledger, _ := NewPageLedger(2)
			err := ledger.ObservePage(tc.evidence)
			if !errors.Is(err, ErrPaginationInconsistent) {
				t.Fatalf("error = %v, want ErrPaginationInconsistent", err)
			}
		})
	}

	ledger, _ := NewPageLedger(3)
	if err := ledger.ObservePage(PageEvidence{Cursor: "0", HasMore: boolEvidence(true), NextToken: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.ObservePage(PageEvidence{Cursor: "3", HasMore: boolEvidence(false)}); !errors.Is(err, ErrPaginationInconsistent) {
		t.Fatalf("mismatched continuation error = %v", err)
	}
}

func TestPageLedgerLaterFailureProducesTypedPartial(t *testing.T) {
	ledger, _ := NewPageLedger(3)
	if err := ledger.ObservePage(PageEvidence{
		Cursor: "0", Items: 2, Data: []any{"a", "b"}, HasMore: boolEvidence(true), NextToken: "2",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.RecordFailure("2", &ErrorInfo{Type: "network", Subtype: "upstream_unavailable", Message: "page read failed"}); err != nil {
		t.Fatal(err)
	}
	result, err := ledger.Result(map[string]any{"items": []any{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome() != OutcomePartialFailure || result.ExitCode() == 0 {
		t.Fatalf("result = outcome %q rc %d", result.Outcome(), result.ExitCode())
	}
	if err := ValidateResult(result); err != nil {
		t.Fatal(err)
	}
	env := result.envelope()
	partial, ok := env.Data.(*PartialData)
	if !ok || partial.Total != 2 || len(partial.Succeeded) != 1 || len(partial.Failed) != 1 || len(partial.Unknown) != 0 {
		t.Fatalf("partial data = %#v", env.Data)
	}
	if partial.Failed[0].ID != "page:2" || partial.Failed[0].Error == nil {
		t.Fatalf("failed entry = %#v", partial.Failed[0])
	}
	if env.Meta != nil && env.Meta.Pagination != nil {
		t.Fatalf("interrupted pagination must not claim endpoint state: %#v", env.Meta.Pagination)
	}
}

func TestPageLedgerFirstFailureIsOrdinaryFailure(t *testing.T) {
	ledger, _ := NewPageLedger(2)
	if err := ledger.RecordFailure("0", &ErrorInfo{Type: "api", Subtype: "rate_limit", Message: "slow down"}); err != nil {
		t.Fatal(err)
	}
	result, err := ledger.Result(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome() != OutcomeFailure || result.ExitCode() == 0 {
		t.Fatalf("result = outcome %q rc %d", result.Outcome(), result.ExitCode())
	}
	if err := ValidateResult(result); err != nil {
		t.Fatal(err)
	}
}

func TestPageLedgerUnknownLaterPageProducesPartialUnknown(t *testing.T) {
	ledger, _ := NewPageLedger(3)
	if err := ledger.ObservePage(PageEvidence{Cursor: "0", Items: 1, Data: []any{"a"}, HasMore: boolEvidence(true), NextToken: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.RecordUnknown("2", "response received but terminal state is unknown"); err != nil {
		t.Fatal(err)
	}
	result, err := ledger.Result(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateResult(result); err != nil {
		t.Fatal(err)
	}
	partial := result.envelope().Data.(*PartialData)
	if len(partial.Unknown) != 1 || partial.Unknown[0].ID != "page:2" {
		t.Fatalf("unknown = %#v", partial.Unknown)
	}
}

func TestPageLedgerBoundaryFailurePreservesCurrentPage(t *testing.T) {
	ledger, _ := NewPageLedger(2)
	if err := ledger.ObservePage(PageEvidence{Cursor: "0", Items: 2, Data: []any{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.RecordBoundaryFailure(&ErrorInfo{
		Type: "api", Subtype: "pagination_inconsistent", Message: "full page omitted pagination evidence",
	}); err != nil {
		t.Fatal(err)
	}
	result, err := ledger.Result(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateResult(result); err != nil {
		t.Fatal(err)
	}
	partial := result.envelope().Data.(*PartialData)
	if len(partial.Succeeded) != 1 || len(partial.Failed) != 1 {
		t.Fatalf("partial = %#v", partial)
	}
	if partial.Failed[0].ID != "page:2" || partial.Failed[0].Error.Subtype != "pagination_inconsistent" {
		t.Fatalf("boundary failure = %#v", partial.Failed[0])
	}
}

func TestPageLedgerPostPageFailureCanInterruptAnExhaustedPage(t *testing.T) {
	ledger, _ := NewPageLedger(2)
	if err := ledger.ObservePage(PageEvidence{Cursor: "0", Items: 1, Data: []any{"a"}, HasMore: boolEvidence(false)}); err != nil {
		t.Fatal(err)
	}
	if err := ledger.RecordPostPageFailure(&ErrorInfo{
		Type: "api", Subtype: "projection_unknown", Message: "range projection failed",
	}); err != nil {
		t.Fatal(err)
	}
	result, err := ledger.Result(nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome() != OutcomePartialFailure || result.ExitCode() == 0 {
		t.Fatalf("result = outcome %q rc %d", result.Outcome(), result.ExitCode())
	}
	if err := ValidateResult(result); err != nil {
		t.Fatal(err)
	}
	partial := result.envelope().Data.(*PartialData)
	if len(partial.Succeeded) != 1 || len(partial.Failed) != 1 || partial.Failed[0].Error.Subtype != "projection_unknown" {
		t.Fatalf("partial = %#v", partial)
	}
}

func TestPageLedgerRecordsAreDefensiveCopies(t *testing.T) {
	payload := map[string]any{"items": []string{"a"}}
	ledger, _ := NewPageLedger(1)
	if err := ledger.ObservePage(PageEvidence{Cursor: "0", Items: 1, Data: payload, HasMore: boolEvidence(false)}); err != nil {
		t.Fatal(err)
	}
	payload["items"].([]string)[0] = "mutated"
	records := ledger.Records()
	got := records[0].Data.(map[string]any)["items"].([]string)[0]
	if got != "a" {
		t.Fatalf("stored page data mutated through caller: %q", got)
	}
	records[0].Data.(map[string]any)["items"].([]string)[0] = "again"
	got = ledger.Records()[0].Data.(map[string]any)["items"].([]string)[0]
	if got != "a" {
		t.Fatalf("stored page data mutated through Records copy: %q", got)
	}
}
