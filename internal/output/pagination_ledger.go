// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package output

import (
	"errors"
	"fmt"
	"strings"
)

// ErrPaginationInconsistent identifies contradictory or unsafe pagination
// evidence. Product adapters should map it to the governed
// pagination_inconsistent subtype; the output package deliberately does not
// import product/API error classifiers.
var ErrPaginationInconsistent = errors.New("output: pagination evidence is inconsistent")

// PageState is the framework's internal view of the last observed pagination
// boundary. Unknown is deliberately not serialized as endpoint_exhausted=false:
// that wire state requires a usable continuation token.
type PageState string

const (
	PageStateInitial      PageState = "initial"
	PageStateContinuation PageState = "continuation"
	PageStateExhausted    PageState = "exhausted"
	PageStateUnknown      PageState = "unknown"
	PageStateInterrupted  PageState = "interrupted"
)

// PageEvidence is the product adapter's normalized evidence for one successful
// page. HasMore must be nil when the upstream did not provide authoritative
// evidence. A non-empty NextToken remains sufficient evidence that the endpoint
// is resumable, but never proves index or business-data completeness.
type PageEvidence struct {
	Cursor    string
	Items     int
	Data      any
	HasMore   *bool
	NextToken string
}

// PageRecord is an immutable-by-copy audit record for one attempted page.
// Status is success, failed, or unknown. Error is present only for failed;
// Reason is present only for unknown.
type PageRecord struct {
	Page   int
	Cursor string
	Items  int
	Data   any
	Status string
	Error  *ErrorInfo
	Reason string
}

const (
	pageStatusSuccess = "success"
	pageStatusFailed  = "failed"
	pageStatusUnknown = "unknown"
)

// PageLedger records pagination facts without interpreting an upstream API's
// field names. It is intentionally stateful only within one command execution.
type PageLedger struct {
	maxPages    int
	records     []PageRecord
	seenCursors map[string]struct{}
	state       PageState
	nextToken   string
	stopReason  string
}

// NewPageLedger creates a bounded ledger. maxPages is a safety budget, not a
// promise that the framework will automatically fetch pages.
func NewPageLedger(maxPages int) (*PageLedger, error) {
	if maxPages < 1 {
		return nil, paginationInvariant("max_pages must be at least 1")
	}
	return &PageLedger{
		maxPages:    maxPages,
		records:     make([]PageRecord, 0, maxPages),
		seenCursors: make(map[string]struct{}, maxPages),
		state:       PageStateInitial,
	}, nil
}

// ObservePage records one successfully decoded page and derives its pagination
// boundary from normalized evidence. It rejects contradictory evidence before
// any unified result can be emitted.
func (l *PageLedger) ObservePage(e PageEvidence) error {
	if l == nil {
		return paginationInvariant("nil PageLedger")
	}
	if e.Items < 0 {
		return paginationInvariant("page item count must not be negative")
	}
	if len(l.successfulRecords()) >= l.maxPages {
		return paginationInvariant("page budget %d exceeded", l.maxPages)
	}
	if err := l.validateNextAttempt(e.Cursor); err != nil {
		return err
	}

	cursor := strings.TrimSpace(e.Cursor)
	token := strings.TrimSpace(e.NextToken)
	if _, duplicate := l.seenCursors[cursor]; duplicate {
		return paginationInvariant("cursor %q was already fetched", cursor)
	}

	switch {
	case e.HasMore != nil && *e.HasMore && token == "":
		return paginationInvariant("has_more=true requires a non-empty next token")
	case e.HasMore != nil && !*e.HasMore && token != "":
		return paginationInvariant("has_more=false must not carry next token %q", token)
	case e.HasMore != nil && *e.HasMore:
		if _, seen := l.seenCursors[token]; seen || token == cursor {
			return paginationInvariant("next token %q does not advance", token)
		}
		l.state = PageStateContinuation
		l.nextToken = token
	case e.HasMore != nil:
		l.state = PageStateExhausted
		l.nextToken = ""
	case token != "":
		if _, seen := l.seenCursors[token]; seen || token == cursor {
			return paginationInvariant("next token %q does not advance", token)
		}
		l.state = PageStateContinuation
		l.nextToken = token
	default:
		l.state = PageStateUnknown
		l.nextToken = ""
	}

	l.seenCursors[cursor] = struct{}{}
	l.records = append(l.records, PageRecord{
		Page:   len(l.records) + 1,
		Cursor: cursor,
		Items:  e.Items,
		Data:   cloneResultData(e.Data),
		Status: pageStatusSuccess,
	})
	return nil
}

// RecordFailure records a page whose request or projection returned a typed
// failure. It does not convert server retry advice into safe replay advice.
func (l *PageLedger) RecordFailure(cursor string, info *ErrorInfo) error {
	if l == nil {
		return paginationInvariant("nil PageLedger")
	}
	if info == nil {
		return paginationInvariant("failed page requires a typed error")
	}
	if err := info.Validate(); err != nil {
		return paginationInvariant("failed page error is invalid: %v", err)
	}
	if err := l.validateNextAttempt(cursor); err != nil {
		return err
	}
	l.records = append(l.records, PageRecord{
		Page:   len(l.records) + 1,
		Cursor: strings.TrimSpace(cursor),
		Status: pageStatusFailed,
		Error:  cloneErrorInfo(info),
	})
	l.state = PageStateInterrupted
	l.nextToken = ""
	return nil
}

// RecordUnknown records a later page whose terminal state cannot be confirmed.
// With no successful page, callers must instead return an ordinary typed
// failure: partial_failure is forbidden when succeeded is empty.
func (l *PageLedger) RecordUnknown(cursor, reason string) error {
	if l == nil {
		return paginationInvariant("nil PageLedger")
	}
	if len(l.successfulRecords()) == 0 {
		return paginationInvariant("unknown page requires at least one successful page")
	}
	if strings.TrimSpace(reason) == "" {
		return paginationInvariant("unknown page requires a reason")
	}
	if err := l.validateNextAttempt(cursor); err != nil {
		return err
	}
	l.records = append(l.records, PageRecord{
		Page:   len(l.records) + 1,
		Cursor: strings.TrimSpace(cursor),
		Status: pageStatusUnknown,
		Reason: strings.TrimSpace(reason),
	})
	l.state = PageStateInterrupted
	l.nextToken = ""
	return nil
}

// SetStopReason records an informational local stop reason such as single_page
// or page_limit. It is intentionally not part of the public Pagination wire.
func (l *PageLedger) SetStopReason(reason string) {
	if l != nil {
		l.stopReason = strings.TrimSpace(reason)
	}
}

func (l *PageLedger) State() PageState {
	if l == nil {
		return PageStateInitial
	}
	return l.state
}

func (l *PageLedger) NextToken() string {
	if l == nil || l.state != PageStateContinuation {
		return ""
	}
	return l.nextToken
}

func (l *PageLedger) StopReason() string {
	if l == nil {
		return ""
	}
	return l.stopReason
}

func (l *PageLedger) Pages() int {
	if l == nil {
		return 0
	}
	return len(l.successfulRecords())
}

func (l *PageLedger) Items() int {
	if l == nil {
		return 0
	}
	total := 0
	for _, record := range l.records {
		if record.Status == pageStatusSuccess {
			total += record.Items
		}
	}
	return total
}

// Records returns a defensive copy suitable for diagnostics and tests.
func (l *PageLedger) Records() []PageRecord {
	if l == nil {
		return nil
	}
	out := make([]PageRecord, len(l.records))
	for i, record := range l.records {
		out[i] = record
		out[i].Data = cloneResultData(record.Data)
		out[i].Error = cloneErrorInfo(record.Error)
	}
	return out
}

// Pagination returns authoritative wire metadata. Unknown and interrupted
// states return nil rather than inventing endpoint_exhausted.
func (l *PageLedger) Pagination() (*Pagination, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	var page *Pagination
	var err error
	switch l.state {
	case PageStateExhausted:
		page, err = NewPagination(true, "")
	case PageStateContinuation:
		page, err = NewPagination(false, l.nextToken)
	case PageStateUnknown, PageStateInterrupted:
		return nil, nil
	default:
		return nil, paginationInvariant("no page evidence was observed")
	}
	if err != nil {
		return nil, err
	}
	page.Pages = l.Pages()
	page.Items = l.Items()
	return page, nil
}

// Result maps the ledger to the framework's single CommandResult. Aggregate
// data remains product-owned. When a later page failed or became unknown, page
// summaries become the partial result units so already-read business data is
// preserved and the Agent can resume without replaying successful pages.
func (l *PageLedger) Result(data any, opts ...ResultOption) (CommandResult, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	succeeded := l.successfulRecords()
	failed := make([]PartialFailedEntry, 0)
	unknown := make([]PartialUnknownEntry, 0)
	for _, record := range l.records {
		switch record.Status {
		case pageStatusFailed:
			failed = append(failed, PartialFailedEntry{ID: pageRecordID(record.Page), Error: cloneErrorInfo(record.Error)})
		case pageStatusUnknown:
			unknown = append(unknown, PartialUnknownEntry{ID: pageRecordID(record.Page), Reason: record.Reason})
		}
	}

	if len(failed)+len(unknown) > 0 {
		if len(succeeded) == 0 {
			if len(failed) == 1 && len(unknown) == 0 {
				return Failure(failed[0].Error, opts...), nil
			}
			return nil, paginationInvariant("interrupted pagination without a successful page requires one ordinary typed failure")
		}
		pageSummaries := make([]any, 0, len(succeeded))
		for _, record := range succeeded {
			summary := map[string]any{
				"id":    pageRecordID(record.Page),
				"page":  record.Page,
				"items": record.Items,
			}
			if record.Cursor != "" {
				summary["cursor"] = record.Cursor
			}
			if record.Data != nil {
				summary["data"] = cloneResultData(record.Data)
			}
			pageSummaries = append(pageSummaries, summary)
		}
		partial, err := NewPartialData(len(pageSummaries)+len(failed)+len(unknown), pageSummaries, failed, unknown)
		if err != nil {
			return nil, err
		}
		return Partial(partial, opts...), nil
	}

	page, err := l.Pagination()
	if err != nil {
		return nil, err
	}
	if page != nil {
		opts = append(opts, ResultOption{apply: func(env *Envelope) {
			if env.Meta == nil {
				env.Meta = &Meta{}
			}
			env.Meta.Pagination = page
		}})
	}
	return Success(data, opts...), nil
}

// Validate protects the framework boundary even if a caller retains and
// mutates a ledger between observations.
func (l *PageLedger) Validate() error {
	if l == nil {
		return paginationInvariant("nil PageLedger")
	}
	if l.maxPages < 1 {
		return paginationInvariant("max_pages must be at least 1")
	}
	if len(l.successfulRecords()) > l.maxPages {
		return paginationInvariant("page budget %d exceeded", l.maxPages)
	}
	if len(l.records) == 0 {
		return paginationInvariant("no page evidence was observed")
	}
	if l.state == PageStateContinuation && strings.TrimSpace(l.nextToken) == "" {
		return paginationInvariant("continuation state requires next token")
	}
	if l.state != PageStateContinuation && strings.TrimSpace(l.nextToken) != "" {
		return paginationInvariant("state %q must not retain next token", l.state)
	}
	for i, record := range l.records {
		if record.Page != i+1 {
			return paginationInvariant("record index %d has non-sequential page number %d", i, record.Page)
		}
		switch record.Status {
		case pageStatusSuccess:
			if record.Items < 0 {
				return paginationInvariant("page %d has negative item count", record.Page)
			}
		case pageStatusFailed:
			if record.Error == nil {
				return paginationInvariant("failed page %d has no typed error", record.Page)
			}
			if err := record.Error.Validate(); err != nil {
				return paginationInvariant("failed page %d has invalid typed error: %v", record.Page, err)
			}
		case pageStatusUnknown:
			if strings.TrimSpace(record.Reason) == "" {
				return paginationInvariant("unknown page %d has no reason", record.Page)
			}
		default:
			return paginationInvariant("page %d has invalid status %q", record.Page, record.Status)
		}
	}
	return nil
}

func (l *PageLedger) validateNextAttempt(cursor string) error {
	switch l.state {
	case PageStateInitial:
		return nil
	case PageStateContinuation:
		if strings.TrimSpace(cursor) != l.nextToken {
			return paginationInvariant("next page cursor %q does not match continuation token %q", strings.TrimSpace(cursor), l.nextToken)
		}
		return nil
	case PageStateExhausted:
		return paginationInvariant("cannot fetch another page after endpoint exhaustion")
	case PageStateUnknown:
		return paginationInvariant("cannot fetch another page without continuation evidence")
	case PageStateInterrupted:
		return paginationInvariant("cannot fetch another page after interruption")
	default:
		return paginationInvariant("invalid ledger state %q", l.state)
	}
}

func (l *PageLedger) successfulRecords() []PageRecord {
	if l == nil {
		return nil
	}
	out := make([]PageRecord, 0, len(l.records))
	for _, record := range l.records {
		if record.Status == pageStatusSuccess {
			out = append(out, record)
		}
	}
	return out
}

func pageRecordID(page int) string { return fmt.Sprintf("page:%d", page) }

func paginationInvariant(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrPaginationInconsistent, fmt.Sprintf(format, args...))
}
