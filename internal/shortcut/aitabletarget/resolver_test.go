// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package aitabletarget

import (
	"errors"
	"reflect"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

type resolverStep struct {
	data map[string]any
	err  error
}

type resolverReader struct {
	steps  []resolverStep
	calls  []map[string]any
	tools  []string
	server []string
}

func reviewedBaseSearchPage(rows []any, hasMore bool, nextCursor string) map[string]any {
	return map[string]any{
		"success":                true,
		"status":                 "success",
		"summary":                "reviewed search page",
		"data":                   map[string]any{"bases": rows, "hasMore": hasMore, "nextCursor": nextCursor},
		"error":                  map[string]any{},
		"meta":                   map[string]any{},
		"hasMore":                hasMore,
		"nextCursor":             nextCursor,
		"paginationKnown":        true,
		"endpointExhausted":      !hasMore,
		"sourceKind":             "name_search_index",
		"authoritativeInventory": false,
		"inventoryCoverageKnown": false,
		"indexCoverageKnown":     false,
	}
}

func (r *resolverReader) CallMCPData(product, tool string, params map[string]any) (map[string]any, error) {
	cloned := make(map[string]any, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	r.calls = append(r.calls, cloned)
	r.tools = append(r.tools, tool)
	r.server = append(r.server, product)
	index := len(r.calls) - 1
	if index >= len(r.steps) {
		return nil, errors.New("unexpected resolver call")
	}
	return r.steps[index].data, r.steps[index].err
}

func TestCrossPlatformCoverageParseAITableURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want Target
	}{
		{
			name: "base",
			url:  "https://alidocs.dingtalk.com/i/nodes/base-1",
			want: Target{SchemaVersion: "aitable.target.v1", Source: "url", Kind: "base", BaseID: "base-1"},
		},
		{
			name: "table and view nested",
			url:  "https://alidocs.dingtalk.com/i/nodes/base-1?iframeQuery=sheetId%3Dtable-1%26viewId%3Dview-1",
			want: Target{SchemaVersion: "aitable.target.v1", Source: "url", Kind: "view", BaseID: "base-1", TableID: "table-1", ViewID: "view-1"},
		},
		{
			name: "double encoded iframe",
			url:  "https://alidocs.dingtalk.com/i/nodes/base-1?iframeQuery=sheetId%253Dtable-1%2526recordId%253Drecord-1",
			want: Target{SchemaVersion: "aitable.target.v1", Source: "url", Kind: "record", BaseID: "base-1", TableID: "table-1", RecordID: "record-1"},
		},
		{
			name: "direct table query",
			url:  "https://alidocs.dingtalk.com/i/nodes/base-1?tableId=table-1",
			want: Target{SchemaVersion: "aitable.target.v1", Source: "url", Kind: "table", BaseID: "base-1", TableID: "table-1"},
		},
		{
			name: "unrelated iframe query stays base",
			url:  "https://alidocs.dingtalk.com/i/nodes/base-1?iframeQuery=preview%3Dcompact",
			want: Target{SchemaVersion: "aitable.target.v1", Source: "url", Kind: "base", BaseID: "base-1"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseURL(test.url)
			if err != nil {
				t.Fatalf("ParseURL() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ParseURL() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestCrossPlatformCoverageParseAITableURLRejectsUnsafeOrAmbiguousInput(t *testing.T) {
	for _, raw := range []string{
		"",
		"http://alidocs.dingtalk.com/i/nodes/base",
		"https://@alidocs.dingtalk.com/i/nodes/base",
		"https://example.com/i/nodes/base",
		"https://alidocs.dingtalk.com/i/nodes/",
		"https://alidocs.dingtalk.com/i/nodes/base/extra",
		"https://alidocs.dingtalk.com/i/nodes/base?viewId=view",
		"https://alidocs.dingtalk.com/i/nodes/base?recordId=record",
		"https://alidocs.dingtalk.com/i/nodes/base?sheetId=one&tableId=two",
	} {
		if _, err := ParseURL(raw); err == nil {
			t.Errorf("ParseURL(%q) succeeded, want error", raw)
		}
	}
}

func TestCrossPlatformCoverageResolveBaseNameExactPaginationE2E(t *testing.T) {
	reader := &resolverReader{steps: []resolverStep{
		{data: reviewedBaseSearchPage([]any{map[string]any{"baseId": "b1", "baseName": "项目管理归档"}}, true, "next")},
		{data: reviewedBaseSearchPage([]any{map[string]any{"baseId": "b2", "baseName": "项目管理"}}, false, "")},
	}}
	got, ledger, err := ResolveBaseNameWithEvidence(reader, "项目管理", false)
	if err != nil {
		t.Fatalf("ResolveBaseName() error = %v", err)
	}
	if got.MatchType != "exact" || got.Selected.ID != "b2" {
		t.Fatalf("resolution = %#v", got)
	}
	if len(reader.calls) != 2 || reader.calls[1]["cursor"] != "next" {
		t.Fatalf("pagination calls = %#v", reader.calls)
	}
	page, err := ledger.Pagination()
	if err != nil || !page.EndpointExhausted || page.Pages != 2 || page.Items != 2 || page.NextToken != "" {
		t.Fatalf("pagination evidence = %#v, %v", page, err)
	}
}

func TestCrossPlatformCoverageResolveNameFuzzyIsExplicitAndAmbiguityFails(t *testing.T) {
	data := reviewedBaseSearchPage([]any{
		map[string]any{"baseId": "b1", "baseName": "项目管理归档"},
		map[string]any{"baseId": "b2", "baseName": "财务"},
	}, false, "")
	reader := &resolverReader{steps: []resolverStep{{data: data}}}
	if _, err := ResolveBaseName(reader, "项目", false); errorReason(err) != "target_not_found" {
		t.Fatalf("fuzzy-disabled error = %v", err)
	}
	reader = &resolverReader{steps: []resolverStep{{data: data}}}
	got, err := ResolveBaseName(reader, "项目", true)
	if err != nil || got.MatchType != "fuzzy" || got.Selected.ID != "b1" {
		t.Fatalf("fuzzy resolution = %#v, %v", got, err)
	}

	reader = &resolverReader{steps: []resolverStep{{data: reviewedBaseSearchPage([]any{
		map[string]any{"baseId": "b1", "baseName": "项目"},
		map[string]any{"baseId": "b2", "baseName": "项目"},
	}, false, "")}}}
	if _, err := ResolveBaseName(reader, "项目", false); errorReason(err) != "target_ambiguous" {
		t.Fatalf("ambiguous error = %v", err)
	}
}

func TestCrossPlatformCoverageResolveNameDistinguishesEmptyFromUnknown(t *testing.T) {
	reader := &resolverReader{steps: []resolverStep{{data: reviewedBaseSearchPage([]any{}, false, "")}}}
	if _, err := ResolveBaseName(reader, "missing", false); errorReason(err) != "target_not_found" {
		t.Fatalf("legal empty list error = %v", err)
	} else {
		var typed *apperrors.Error
		if !errors.As(err, &typed) || typed.Hint == "" || typed.Details["coverage_scope"] != "name_search_index" || typed.Details["index_coverage_known"] != false {
			t.Fatalf("empty search lost index boundary: %#v", err)
		}
	}
	reader = &resolverReader{steps: []resolverStep{{data: map[string]any{"success": true}}}}
	if _, err := ResolveBaseName(reader, "missing", false); errorReason(err) != "projection_unknown" {
		t.Fatalf("unknown response error = %v", err)
	}
	reader = &resolverReader{steps: []resolverStep{{data: reviewedBaseSearchPage([]any{}, true, "")}}}
	if _, err := ResolveBaseName(reader, "missing", false); errorReason(err) != "pagination_inconsistent" {
		t.Fatalf("incomplete response error = %v", err)
	}
	for name, data := range map[string]map[string]any{
		"wrong collection type": reviewedBaseSearchPage(nil, false, ""),
		"scalar candidate":      reviewedBaseSearchPage([]any{"bad"}, false, ""),
		"candidate missing id":  reviewedBaseSearchPage([]any{map[string]any{"baseName": "missing"}}, false, ""),
	} {
		if name == "wrong collection type" {
			data["data"].(map[string]any)["bases"] = map[string]any{}
		}
		t.Run(name, func(t *testing.T) {
			reader := &resolverReader{steps: []resolverStep{{data: data}}}
			if _, err := ResolveBaseName(reader, "missing", false); errorReason(err) != "projection_unknown" {
				t.Fatalf("malformed candidate response error = %v", err)
			}
		})
	}
}

func TestCrossPlatformCoverageProjectBaseSearchPageRejectsUnreviewedShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{name: "missing success", mutate: func(page map[string]any) { delete(page, "success") }, want: "projection_unknown"},
		{name: "business failure", mutate: func(page map[string]any) { page["success"] = false }, want: "projection_unknown"},
		{name: "unknown top-level field", mutate: func(page map[string]any) { page["guessed"] = true }, want: "projection_unknown"},
		{name: "status wrong type", mutate: func(page map[string]any) { page["status"] = 1 }, want: "projection_unknown"},
		{name: "meta wrong type", mutate: func(page map[string]any) { page["meta"] = "bad" }, want: "projection_unknown"},
		{name: "wrong source kind", mutate: func(page map[string]any) { page["sourceKind"] = "recently_accessed" }, want: "projection_unknown"},
		{name: "coverage overclaim", mutate: func(page map[string]any) { page["indexCoverageKnown"] = true }, want: "projection_unknown"},
		{name: "unknown data field", mutate: func(page map[string]any) { page["data"].(map[string]any)["items"] = []any{} }, want: "projection_unknown"},
		{name: "generic identifier alias", mutate: func(page map[string]any) {
			page["data"].(map[string]any)["bases"] = []any{map[string]any{"id": "b1", "name": "项目"}}
		}, want: "projection_unknown"},
		{name: "unknown row field", mutate: func(page map[string]any) {
			page["data"].(map[string]any)["bases"] = []any{map[string]any{"baseId": "b1", "baseName": "项目", "title": "项目"}}
		}, want: "projection_unknown"},
		{name: "duplicate row identifier", mutate: func(page map[string]any) {
			page["data"].(map[string]any)["bases"] = []any{
				map[string]any{"baseId": "b1", "baseName": "项目"},
				map[string]any{"baseId": "b1", "baseName": "项目副本"},
			}
		}, want: "projection_unknown"},
		{name: "missing pagination evidence", mutate: func(page map[string]any) {
			delete(page, "hasMore")
			delete(page["data"].(map[string]any), "hasMore")
		}, want: "pagination_inconsistent"},
		{name: "conflicting has more", mutate: func(page map[string]any) {
			page["hasMore"] = true
			page["endpointExhausted"] = false
		}, want: "pagination_inconsistent"},
		{name: "conflicting cursor", mutate: func(page map[string]any) {
			page["nextCursor"] = "outer"
			page["data"].(map[string]any)["nextCursor"] = "inner"
		}, want: "pagination_inconsistent"},
		{name: "open page without cursor", mutate: func(page map[string]any) {
			page["hasMore"] = true
			page["data"].(map[string]any)["hasMore"] = true
			page["endpointExhausted"] = false
		}, want: "pagination_inconsistent"},
		{name: "terminal page with cursor", mutate: func(page map[string]any) {
			page["nextCursor"] = "next"
			page["data"].(map[string]any)["nextCursor"] = "next"
		}, want: "pagination_inconsistent"},
		{name: "pagination unknown", mutate: func(page map[string]any) { page["paginationKnown"] = false }, want: "pagination_inconsistent"},
		{name: "conflicting exhausted declaration", mutate: func(page map[string]any) { page["endpointExhausted"] = false }, want: "pagination_inconsistent"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := reviewedBaseSearchPage([]any{map[string]any{"baseId": "b1", "baseName": "项目"}}, false, "")
			test.mutate(page)
			_, err := projectBaseSearchPage(page)
			if errorReason(err) != test.want {
				t.Fatalf("projectBaseSearchPage() error = %v, want %s", err, test.want)
			}
		})
	}
}

func TestCrossPlatformCoverageResolveBaseRejectsCrossPageDuplicateIdentity(t *testing.T) {
	reader := &resolverReader{steps: []resolverStep{
		{data: reviewedBaseSearchPage([]any{map[string]any{"baseId": "b1", "baseName": "项目"}}, true, "next")},
		{data: reviewedBaseSearchPage([]any{map[string]any{"baseId": "b1", "baseName": "项目副本"}}, false, "")},
	}}
	if _, err := ResolveBaseName(reader, "项目", false); errorReason(err) != "projection_unknown" {
		t.Fatalf("cross-page duplicate error = %v", err)
	}
}

func TestCrossPlatformCoverageResolveTableNameE2E(t *testing.T) {
	reader := &resolverReader{steps: []resolverStep{{data: map[string]any{"success": true, "data": map[string]any{"tables": []any{
		map[string]any{"tableId": "t1", "tableName": "任务"},
		map[string]any{"tableId": "t2", "tableName": "任务归档"},
	}}}}}}
	got, err := ResolveTableName(reader, "base", "任务", false)
	if err != nil || got.Selected.ID != "t1" || got.MatchType != "exact" {
		t.Fatalf("table resolution = %#v, %v", got, err)
	}
	if reader.server[0] != "aitable" || reader.tools[0] != "get_tables" || reader.calls[0]["baseId"] != "base" {
		t.Fatalf("table resolution call = %v/%v %#v", reader.server, reader.tools, reader.calls)
	}
}

func TestCrossPlatformCoverageResolveTableNameMalformedCandidateIsInvalidResponse(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"explicit empty is not found": {"success": true, "data": map[string]any{"tables": []any{}}},
		"scalar candidate":            {"success": true, "data": map[string]any{"tables": []any{1}}},
		"candidate missing name":      {"success": true, "data": map[string]any{"tables": []any{map[string]any{"tableId": "t1"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			reader := &resolverReader{steps: []resolverStep{{data: data}}}
			_, err := ResolveTableName(reader, "base", "missing", false)
			want := "projection_unknown"
			if name == "explicit empty is not found" {
				want = "target_not_found"
			}
			if errorReason(err) != want {
				t.Fatalf("resolution error = %v, want reason %s", err, want)
			}
		})
	}
}

func TestCrossPlatformCoverageListTablesStrictProjection(t *testing.T) {
	reader := &resolverReader{steps: []resolverStep{{data: map[string]any{
		"success": true,
		"status":  "success",
		"summary": "one table",
		"error":   map[string]any{},
		"meta":    map[string]any{},
		"data": map[string]any{"tables": []any{
			map[string]any{"tableId": "t1", "tableName": "任务", "fields": []any{}, "views": []any{}},
		}},
	}}}}
	got, err := ListTables(reader, "base")
	if err != nil || !reflect.DeepEqual(got, []Candidate{{ID: "t1", Name: "任务"}}) {
		t.Fatalf("ListTables() = %#v, %v", got, err)
	}
	if len(reader.calls) != 1 || reader.tools[0] != "get_tables" || len(reader.calls[0]) != 1 || reader.calls[0]["baseId"] != "base" {
		t.Fatalf("ListTables call = %v %#v", reader.tools, reader.calls)
	}
}

func TestCrossPlatformCoverageListTablesRejectsGuessesAndMalformedRows(t *testing.T) {
	tests := map[string]map[string]any{
		"legacy guessed top-level list": {"tables": []any{}},
		"unknown top-level key":         {"success": true, "data": map[string]any{"tables": []any{}}, "hasMore": false},
		"unknown data container":        {"success": true, "data": map[string]any{"items": []any{}}},
		"generic id alias":              {"success": true, "data": map[string]any{"tables": []any{map[string]any{"id": "t1", "name": "任务"}}}},
		"duplicate stable id": {"success": true, "data": map[string]any{"tables": []any{
			map[string]any{"tableId": "t1", "tableName": "任务"},
			map[string]any{"tableId": "t1", "tableName": "任务副本"},
		}}},
		"wrong child collection": {"success": true, "data": map[string]any{"tables": []any{
			map[string]any{"tableId": "t1", "tableName": "任务", "views": map[string]any{}},
		}}},
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			reader := &resolverReader{steps: []resolverStep{{data: data}}}
			if _, err := ListTables(reader, "base"); errorReason(err) != "projection_unknown" {
				t.Fatalf("ListTables error = %v", err)
			}
		})
	}
}

func errorReason(err error) string {
	var typed *apperrors.Error
	if errors.As(err, &typed) {
		return typed.Reason
	}
	return ""
}
