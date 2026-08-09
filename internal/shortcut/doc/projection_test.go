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
}

func (c *docDiscoveryCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.product, c.tool = product, tool
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
			payload: `{"result":{"items":[{"nodeId":"doc-1","title":"Design"}]}}`,
			items:   "documents",
		},
		{
			name:    "list",
			decl:    List,
			tool:    "list_nodes",
			payload: `{"result":{"nodes":[{"nodeId":"node-1","nodeName":"Folder"}]}}`,
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
			if data["count"] != float64(1) || data["pagination_known"] != false || len(data[tc.items].([]any)) != 1 {
				t.Fatalf("data = %#v", data)
			}
			if envelope["meta"].(map[string]any)["count"] != float64(1) {
				t.Fatalf("meta = %#v", envelope["meta"])
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

func assertDocProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
