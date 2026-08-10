package doc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type docDiscoveryCaller struct {
	product string
	tool    string
	text    string
	calls   int
	args    map[string]any
}

func (c *docDiscoveryCaller) CallTool(_ context.Context, product, tool string, args map[string]any) (*edition.ToolResult, error) {
	c.calls++
	c.product, c.tool, c.args = product, tool, args
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: c.text}}}, nil
}

func (*docDiscoveryCaller) Format() string { return "json" }
func (*docDiscoveryCaller) DryRun() bool   { return false }
func (*docDiscoveryCaller) Fields() string { return "" }
func (*docDiscoveryCaller) JQ() string     { return "" }

func TestDocProjectionPreservesKnownEmptyContainers(t *testing.T) {
	docs, err := searchDocsProject(map[string]any{"documents": []any{}})
	if err != nil {
		t.Fatalf("known empty search response returned error: %v", err)
	}
	if docs == nil || len(docs) != 0 {
		t.Fatalf("known empty search response = %#v, want non-nil empty list", docs)
	}

	nodes, err := listNodesProject(map[string]any{"nodes": []any{}})
	if err != nil {
		t.Fatalf("known empty node response returned error: %v", err)
	}
	if nodes == nil || len(nodes) != 0 {
		t.Fatalf("known empty node response = %#v, want non-nil empty list", nodes)
	}
}

func TestDocProjectionRejectsUnknownShapeAndRow(t *testing.T) {
	_, err := searchDocsProject(map[string]any{"result": map[string]any{"unexpected": []any{}}})
	assertDocProjectionUnknown(t, err)

	_, err = listNodesProject(map[string]any{"nodes": []any{map[string]any{"opaque": true}}})
	assertDocProjectionUnknown(t, err)

	_, err = searchDocsProject(map[string]any{"documents": []any{map[string]any{"title": "display only"}}})
	assertDocProjectionUnknown(t, err)

	_, err = listNodesProject(map[string]any{"nodes": []any{map[string]any{"nodeId": " "}}})
	assertDocProjectionUnknown(t, err)
}

func TestDocDiscoveryShortcutsUseUnifiedOutput(t *testing.T) {
	if Search.OutputRollout != output.RolloutUnifiedActive || List.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("doc discovery rollout = %q/%q, want unified active", Search.OutputRollout, List.OutputRollout)
	}
}

func TestDocDiscoveryUnifiedOutputHasOneMachineEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		decl    shortcut.Shortcut
		tool    string
		payload string
		items   string
	}{
		{
			name:    "search",
			decl:    Search,
			tool:    "search_documents",
			payload: `{"documents":[{"nodeId":"doc-1","title":"Design"}],"hasMore":true,"nextPageToken":"page-2"}`,
			items:   "documents",
		},
		{
			name:    "list",
			decl:    List,
			tool:    "list_nodes",
			payload: `{"nodes":[{"nodeId":"node-1","nodeName":"Folder"}],"hasMore":true,"nextPageToken":"page-2"}`,
			items:   "nodes",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &docDiscoveryCaller{text: tc.payload}
			helpers.InitDeps(caller)
			cmd := corecmd.New(shortcut.FromShortcut(tc.decl))
			cmd.PersistentFlags().String("format", "json", "")
			ctx, _ := output.WithResultStore(context.Background())
			cmd.SetContext(ctx)
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--format", "json"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			exitCode, emitted, err := output.EmitStoredResult(cmd)
			if err != nil || !emitted || exitCode != 0 {
				t.Fatalf("emit: code=%d emitted=%v err=%v", exitCode, emitted, err)
			}
			if caller.product != productDoc || caller.tool != tc.tool {
				t.Fatalf("route = %s/%s, want %s/%s", caller.product, caller.tool, productDoc, tc.tool)
			}
			var envelope map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("decode output: %v\n%s", err, stdout.String())
			}
			if envelope["ok"] != true || envelope["outcome"] != "success" {
				t.Fatalf("envelope = %#v", envelope)
			}
			if _, leaked := envelope["contract_version"]; leaked {
				t.Fatalf("result leaked removed version marker: %#v", envelope)
			}
			data := envelope["data"].(map[string]any)
			if data["count"] != float64(1) || data["pagination_known"] != true || len(data[tc.items].([]any)) != 1 {
				t.Fatalf("data = %#v", data)
			}
			meta := envelope["meta"].(map[string]any)
			page := meta["pagination"].(map[string]any)
			if meta["count"] != float64(1) || page["endpoint_exhausted"] != false || page["next_token"] != "page-2" {
				t.Fatalf("meta = %#v", envelope["meta"])
			}
			if tc.name == "search" && data["index_coverage_known"] != false {
				t.Fatalf("search data widened index coverage: %#v", data)
			}
			if tc.name == "list" && data["inventory_scope"] != "requested_location" {
				t.Fatalf("list data missing bounded inventory scope: %#v", data)
			}
		})
	}
}

func TestDocProjectionAcceptsNestedKnownContainers(t *testing.T) {
	docs, err := searchDocsProject(map[string]any{
		"result": map[string]any{"items": []any{map[string]any{"docId": "d1", "title": "Design"}}},
	})
	if err != nil {
		t.Fatalf("nested search response returned error: %v", err)
	}
	if len(docs) != 1 || docs[0]["nodeId"] != "d1" || docs[0]["name"] != "Design" {
		t.Fatalf("nested document projection = %#v", docs)
	}

	nodes, err := listNodesProject(map[string]any{
		"data": map[string]any{"nodes": []any{map[string]any{"nodeId": "n1", "nodeName": "Folder"}}},
	})
	if err != nil {
		t.Fatalf("nested list response returned error: %v", err)
	}
	if len(nodes) != 1 || nodes[0]["nodeId"] != "n1" || nodes[0]["name"] != "Folder" {
		t.Fatalf("nested node projection = %#v", nodes)
	}
}

func TestDocDiscoveryPaginationContract(t *testing.T) {
	tests := []struct {
		name          string
		data          map[string]any
		known         bool
		exhausted     bool
		nextToken     string
		wantErrorType string
	}{
		{name: "unknown", data: map[string]any{"nodes": []any{}}, known: false},
		{name: "terminal", data: map[string]any{"hasMore": false}, known: true, exhausted: true},
		{name: "continuation real service token", data: map[string]any{"hasMore": true, "nextPageToken": "pos:-1245174.5"}, known: true, nextToken: "pos:-1245174.5"},
		{name: "terminal numeric sentinel", data: map[string]any{"hasMore": false, "nextPageToken": float64(0)}, known: true, exhausted: true},
		{name: "continuation without token", data: map[string]any{"hasMore": true}, wantErrorType: "pagination_inconsistent"},
		{name: "terminal with token", data: map[string]any{"hasMore": false, "nextPageToken": "page-2"}, wantErrorType: "pagination_inconsistent"},
		{name: "token without has more", data: map[string]any{"nextPageToken": "page-2"}, wantErrorType: "pagination_inconsistent"},
		{name: "non boolean has more", data: map[string]any{"hasMore": "true", "nextPageToken": "page-2"}, wantErrorType: "pagination_inconsistent"},
		{name: "same scope has more aliases conflict", data: map[string]any{"hasMore": true, "has_more": false, "nextPageToken": "page-2"}, wantErrorType: "pagination_inconsistent"},
		{name: "same scope cursor aliases conflict", data: map[string]any{"hasMore": true, "nextPageToken": "page-2", "nextCursor": "page-3"}, wantErrorType: "pagination_inconsistent"},
		{name: "fractional numeric token", data: map[string]any{"hasMore": true, "nextPageToken": 1.5}, wantErrorType: "pagination_inconsistent"},
		{name: "unsafe json integer token", data: map[string]any{"hasMore": true, "nextPageToken": float64(1 << 54)}, wantErrorType: "pagination_inconsistent"},
		{name: "nested conflict", data: map[string]any{"hasMore": false, "result": map[string]any{"hasMore": true, "nextPageToken": "page-2"}}, wantErrorType: "pagination_inconsistent"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, known, err := docDiscoveryPagination(tc.data)
			if tc.wantErrorType != "" {
				assertDocTypedReason(t, err, tc.wantErrorType)
				return
			}
			if err != nil || known != tc.known {
				t.Fatalf("pagination = %#v known=%v err=%v", page, known, err)
			}
			if !known {
				if page != nil {
					t.Fatalf("unknown pagination returned page %#v", page)
				}
				return
			}
			if page == nil || page.EndpointExhausted != tc.exhausted || page.NextToken != tc.nextToken {
				t.Fatalf("pagination = %#v, want exhausted=%v token=%q", page, tc.exhausted, tc.nextToken)
			}
		})
	}
}

func TestDocDiscoveryProjectionUsesTypedLiveFields(t *testing.T) {
	docs, err := searchDocsProject(map[string]any{"documents": []any{map[string]any{
		"nodeId":       "doc-1",
		"name":         "Design",
		"extension":    "adoc",
		"docUrl":       "https://example.invalid/doc",
		"creatorUid":   "user-1",
		"lastEditTime": nil,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0]["creatorId"] != "user-1" {
		t.Fatalf("search projection = %#v", docs)
	}
	if _, present := docs[0]["modifiedTime"]; present {
		t.Fatalf("null timestamp must be omitted, got %#v", docs[0])
	}

	nodes, err := listNodesProject(map[string]any{"nodes": []any{map[string]any{
		"nodeId":      "node-1",
		"name":        "Folder",
		"nodeType":    "folder",
		"hasChildren": true,
		"workspaceId": "workspace-1",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0]["hasChildren"] != true || nodes[0]["workspaceId"] != "workspace-1" {
		t.Fatalf("node projection = %#v", nodes)
	}

	_, err = searchDocsProject(map[string]any{"documents": []any{map[string]any{"nodeId": "doc-1", "name": map[string]any{}}}})
	assertDocProjectionUnknown(t, err)
	_, err = listNodesProject(map[string]any{"nodes": []any{map[string]any{"nodeId": "node-1", "hasChildren": "true"}}})
	assertDocProjectionUnknown(t, err)
}

func TestDocDiscoveryRejectsInvalidInputBeforeRemoteCall(t *testing.T) {
	tests := []struct {
		name string
		decl shortcut.Shortcut
		args []string
	}{
		{name: "search limit zero", decl: Search, args: []string{"--limit", "0"}},
		{name: "search limit too large", decl: Search, args: []string{"--limit", "31"}},
		{name: "list limit zero", decl: List, args: []string{"--limit", "0"}},
		{name: "list limit too large", decl: List, args: []string{"--limit", "51"}},
		{name: "negative created timestamp", decl: Search, args: []string{"--created-from", "-1"}},
		{name: "reversed created range", decl: Search, args: []string{"--created-from", "2", "--created-to", "1"}},
		{name: "reversed visited range", decl: Search, args: []string{"--visited-from", "2", "--visited-to", "1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &docDiscoveryCaller{text: `{}`}
			helpers.InitDeps(caller)
			cmd := corecmd.New(shortcut.FromShortcut(tc.decl))
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			assertDocTypedReason(t, err, "invalid_flag_value")
			if caller.calls != 0 {
				t.Fatalf("remote calls = %d, want 0", caller.calls)
			}
		})
	}
}

func assertDocProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	assertDocTypedReason(t, err, "projection_unknown")
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}

func assertDocTypedReason(t *testing.T, err error, reason string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected typed reason %q", reason)
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != reason {
		t.Fatalf("error = %T %#v, want reason %q", err, err, reason)
	}
}
