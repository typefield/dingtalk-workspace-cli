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
		{data: map[string]any{"data": map[string]any{
			"bases":      []any{map[string]any{"baseId": "b1", "baseName": "项目管理归档"}},
			"nextCursor": "next",
			"hasMore":    true,
		}}},
		{data: map[string]any{"data": map[string]any{
			"bases":   []any{map[string]any{"baseId": "b2", "baseName": "项目管理"}},
			"hasMore": false,
		}}},
	}}
	got, err := ResolveBaseName(reader, "项目管理", false)
	if err != nil {
		t.Fatalf("ResolveBaseName() error = %v", err)
	}
	if got.MatchType != "exact" || got.Selected.ID != "b2" {
		t.Fatalf("resolution = %#v", got)
	}
	if len(reader.calls) != 2 || reader.calls[1]["cursor"] != "next" {
		t.Fatalf("pagination calls = %#v", reader.calls)
	}
}

func TestCrossPlatformCoverageResolveNameFuzzyIsExplicitAndAmbiguityFails(t *testing.T) {
	data := map[string]any{"bases": []any{
		map[string]any{"baseId": "b1", "baseName": "项目管理归档"},
		map[string]any{"baseId": "b2", "baseName": "财务"},
	}}
	reader := &resolverReader{steps: []resolverStep{{data: data}}}
	if _, err := ResolveBaseName(reader, "项目", false); errorReason(err) != "target_not_found" {
		t.Fatalf("fuzzy-disabled error = %v", err)
	}
	reader = &resolverReader{steps: []resolverStep{{data: data}}}
	got, err := ResolveBaseName(reader, "项目", true)
	if err != nil || got.MatchType != "fuzzy" || got.Selected.ID != "b1" {
		t.Fatalf("fuzzy resolution = %#v, %v", got, err)
	}

	reader = &resolverReader{steps: []resolverStep{{data: map[string]any{"bases": []any{
		map[string]any{"baseId": "b1", "baseName": "项目"},
		map[string]any{"baseId": "b2", "baseName": "项目"},
	}}}}}
	if _, err := ResolveBaseName(reader, "项目", false); errorReason(err) != "target_ambiguous" {
		t.Fatalf("ambiguous error = %v", err)
	}
}

func TestCrossPlatformCoverageResolveNameDistinguishesEmptyFromUnknown(t *testing.T) {
	reader := &resolverReader{steps: []resolverStep{{data: map[string]any{"bases": []any{}}}}}
	if _, err := ResolveBaseName(reader, "missing", false); errorReason(err) != "target_not_found" {
		t.Fatalf("legal empty list error = %v", err)
	}
	reader = &resolverReader{steps: []resolverStep{{data: map[string]any{"success": true}}}}
	if _, err := ResolveBaseName(reader, "missing", false); errorReason(err) != "target_invalid_response" {
		t.Fatalf("unknown response error = %v", err)
	}
	reader = &resolverReader{steps: []resolverStep{{data: map[string]any{"bases": []any{}, "hasMore": true}}}}
	if _, err := ResolveBaseName(reader, "missing", false); errorReason(err) != "target_incomplete" {
		t.Fatalf("incomplete response error = %v", err)
	}
	for name, data := range map[string]map[string]any{
		"wrong collection type": {"bases": map[string]any{}},
		"scalar candidate":      {"bases": []any{"bad"}},
		"candidate missing id":  {"bases": []any{map[string]any{"baseName": "missing"}}},
	} {
		t.Run(name, func(t *testing.T) {
			reader := &resolverReader{steps: []resolverStep{{data: data}}}
			if _, err := ResolveBaseName(reader, "missing", false); errorReason(err) != "target_invalid_response" {
				t.Fatalf("malformed candidate response error = %v", err)
			}
		})
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
