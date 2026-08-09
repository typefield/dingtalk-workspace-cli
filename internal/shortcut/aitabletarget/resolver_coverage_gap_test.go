// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package aitabletarget

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageParseAITableURLRemainingEdges(t *testing.T) {
	for _, raw := range []string{
		"https://alidocs.dingtalk.com/i/nodes/base%2Fchild",
		"https://alidocs.dingtalk.com/i/nodes/base?iframeQuery=%25",
		"https://alidocs.dingtalk.com/i/nodes/base?viewId=one&viewId=two&sheetId=table",
		"https://alidocs.dingtalk.com/i/nodes/base?recordId=one&recordId=two&sheetId=table",
	} {
		if _, err := ParseURL(raw); err == nil {
			t.Fatalf("ParseURL(%q) succeeded", raw)
		}
	}
	if _, err := parseIframeQuery("%"); err == nil {
		t.Fatal("invalid iframe escape succeeded")
	}
	if values, err := parseIframeQuery("preview"); err != nil || !values.Has("preview") {
		t.Fatalf("plain iframe query = %#v, %v", values, err)
	}
	if _, err := parseIframeQuery("key=%zz"); err == nil {
		t.Fatal("invalid iframe query value succeeded")
	}
	values := url.Values{"id": {" ", "bad/id"}}
	if _, err := uniqueQueryID(values, "id"); err == nil {
		t.Fatal("invalid query ID succeeded")
	}
	if validID("ok\x7fno") {
		t.Fatal("control character ID succeeded")
	}
}

func TestCrossPlatformCoverageResolveBaseRemainingFailures(t *testing.T) {
	if _, err := ResolveBaseName(&resolverReader{}, " ", false); err == nil {
		t.Fatal("empty base name succeeded")
	}
	reader := &resolverReader{steps: []resolverStep{{err: errors.New("search failed")}}}
	if _, err := ResolveBaseName(reader, "name", false); err == nil {
		t.Fatal("search error was swallowed")
	}
	reader = &resolverReader{steps: []resolverStep{
		{data: reviewedBaseSearchPage([]any{}, true, "same")},
		{data: reviewedBaseSearchPage([]any{}, true, "same")},
	}}
	if _, err := ResolveBaseName(reader, "name", false); errorReason(err) != "pagination_inconsistent" {
		t.Fatalf("cursor cycle = %v", err)
	}

	steps := make([]resolverStep, maxResolutionPages)
	for index := range steps {
		steps[index] = resolverStep{data: reviewedBaseSearchPage([]any{}, true, fmt.Sprintf("cursor-%d", index))}
	}
	reader = &resolverReader{steps: steps}
	if _, err := ResolveBaseName(reader, "name", false); errorReason(err) != "target_incomplete" || len(reader.calls) != maxResolutionPages {
		t.Fatalf("page bound = err:%v calls:%d", err, len(reader.calls))
	}
}

func TestCrossPlatformCoverageResolveTableRemainingFailures(t *testing.T) {
	for _, pair := range [][2]string{{"bad/id", "name"}, {"base", " "}} {
		if _, err := ResolveTableName(&resolverReader{}, pair[0], pair[1], false); err == nil {
			t.Fatalf("invalid table args %#v succeeded", pair)
		}
	}
	reader := &resolverReader{steps: []resolverStep{{err: errors.New("tables failed")}}}
	if _, err := ResolveTableName(reader, "base", "name", false); err == nil {
		t.Fatal("table error was swallowed")
	}
	reader = &resolverReader{steps: []resolverStep{{data: map[string]any{"success": true, "data": map[string]any{"tables": "bad"}}}}}
	if _, err := ResolveTableName(reader, "base", "name", false); errorReason(err) != "projection_unknown" {
		t.Fatalf("invalid table list = %v", err)
	}
	reader = &resolverReader{steps: []resolverStep{{data: map[string]any{"success": true}}}}
	if _, err := ResolveTableName(reader, "base", "name", false); errorReason(err) != "projection_unknown" {
		t.Fatalf("missing table list = %v", err)
	}
}

func TestCrossPlatformCoverageResolverDedupeForIncompleteEvidence(t *testing.T) {
	got := dedupe([]Candidate{{}, {ID: "one", Name: "first"}, {ID: "one", Name: "duplicate"}, {ID: "two", Name: "second"}})
	if len(got) != 2 || strings.Join([]string{got[0].ID, got[1].ID}, ",") != "one,two" {
		t.Fatalf("dedupe = %#v", got)
	}
}
