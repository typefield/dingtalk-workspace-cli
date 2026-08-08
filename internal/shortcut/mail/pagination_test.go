package mail

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestMailPaginationProjectsContinuationAndTerminalEvidence(t *testing.T) {
	page, known, err := mailPagination(map[string]any{
		"result": map[string]any{"hasMore": true, "nextCursor": "cursor-2"},
	})
	if err != nil || !known || page.EndpointExhausted || page.NextToken != "cursor-2" {
		t.Fatalf("continuation = %#v, known=%v, err=%v", page, known, err)
	}

	for _, cursor := range []any{"", "$", "0", float64(0)} {
		page, known, err = mailPagination(map[string]any{"hasMore": false, "nextCursor": cursor})
		if err != nil || !known || !page.EndpointExhausted || page.NextToken != "" {
			t.Fatalf("terminal cursor=%#v => %#v, known=%v, err=%v", cursor, page, known, err)
		}
	}

	meta, err := mailListMeta(map[string]any{"hasMore": false}, 0)
	if err != nil || meta.Count == nil || *meta.Count != 0 || meta.Pagination == nil || !meta.Pagination.EndpointExhausted {
		t.Fatalf("list metadata = %#v, err=%v", meta, err)
	}
	if _, err := output.EnvelopeFromResult(output.Success(map[string]any{"threads": []any{}}, output.WithMeta(meta))); err != nil {
		t.Fatalf("metadata must form a valid unified result: %v", err)
	}
}

func TestMailPaginationDoesNotInventExhaustionWithoutEvidence(t *testing.T) {
	page, known, err := mailPagination(map[string]any{"result": map[string]any{"items": []any{}}})
	if err != nil || known || page != nil {
		t.Fatalf("missing evidence = %#v, known=%v, err=%v", page, known, err)
	}

	meta, err := mailListMeta(map[string]any{"items": []any{}}, 0)
	if err != nil || meta.Pagination != nil || meta.Count == nil || *meta.Count != 0 {
		t.Fatalf("missing-evidence metadata = %#v, err=%v", meta, err)
	}
}

func TestMailPaginationRejectsInconsistentEvidence(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"continuation without cursor": {"hasMore": true},
		"terminal with cursor":        {"hasMore": false, "nextCursor": "cursor-2"},
		"cursor without has more":     {"nextCursor": "cursor-2"},
		"invalid has more type":       {"hasMore": "true", "nextCursor": "cursor-2"},
		"invalid cursor type":         {"hasMore": true, "nextCursor": []any{"cursor-2"}},
		"nested contradiction": {
			"hasMore": false,
			"result":  map[string]any{"hasMore": true, "nextCursor": "cursor-2"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := mailPagination(data)
			assertMailPaginationError(t, err)
		})
	}
}

func TestMailPaginatedListsUseUnifiedResultAfterPaginationProjection(t *testing.T) {
	for name, rollout := range map[string]output.RolloutState{
		"thread-list":   ThreadList.OutputRollout,
		"user-search":   UserSearch.OutputRollout,
		"template-list": TemplateList.OutputRollout,
		"contact-list":  ContactList.OutputRollout,
	} {
		if rollout != output.RolloutUnifiedActive {
			t.Fatalf("%s rollout = %s, want unified_active", name, rollout)
		}
	}
}

func assertMailPaginationError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("pagination unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "pagination_inconsistent" || typed.Retryable {
		t.Fatalf("pagination error = %T %#v, want non-retryable pagination_inconsistent", err, err)
	}
}
