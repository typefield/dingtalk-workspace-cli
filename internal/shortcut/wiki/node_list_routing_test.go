// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package wiki

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

type nodeListRoutingCaller struct {
	product string
	tool    string
	args    map[string]any
	calls   int
	text    string
}

func (c *nodeListRoutingCaller) CallTool(_ context.Context, product, tool string, args map[string]any) (*edition.ToolResult, error) {
	c.calls++
	c.product, c.tool, c.args = product, tool, args
	text := c.text
	if text == "" {
		text = `{"result":{"nodes":[{"name":"Guide","nodeId":"node-1","nodeType":"doc"}]}}`
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{
		Type: "text",
		Text: text,
	}}}, nil
}

func (*nodeListRoutingCaller) Format() string { return "json" }
func (*nodeListRoutingCaller) DryRun() bool   { return false }
func (*nodeListRoutingCaller) Fields() string { return "" }
func (*nodeListRoutingCaller) JQ() string     { return "" }

func TestNodeListPublishesReviewedReadContract(t *testing.T) {
	if NodeList.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("OutputRollout = %s, want unified_active", NodeList.OutputRollout)
	}
	if !shortcut.InPublicCatalog(NodeList.Service, NodeList.Command) {
		t.Fatal("wiki +node-list remains executable but absent from the public catalog")
	}
	if NodeList.Contract.Empty() {
		t.Fatal("wiki +node-list has no Agent contract")
	}
	safety := shortcut.EffectiveSafety(NodeList)
	if safety.Effect != "read" || safety.Risk != "low" ||
		safety.Confirmation != "not_required" || safety.Idempotency != "idempotent" {
		t.Fatalf("Safety = %#v, want read/low/not_required/idempotent", safety)
	}
}

func TestNodeListRoutesToDocServerAndProjectsNodes(t *testing.T) {
	caller := &nodeListRoutingCaller{}
	helpers.InitDeps(caller)
	cmd := corecmd.New(shortcut.FromShortcut(NodeList))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"--workspace", "workspace-1", "--folder", "folder-1", "--limit", "20"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, emitted, err := output.EmitStoredResult(cmd); err != nil || !emitted {
		t.Fatalf("emit unified result: emitted=%v err=%v", emitted, err)
	}
	if caller.product != "doc" || caller.tool != "list_nodes" {
		t.Fatalf("route = %s/%s, want doc/list_nodes", caller.product, caller.tool)
	}
	if caller.calls != 1 {
		t.Fatalf("calls = %d, want exactly one", caller.calls)
	}
	if caller.args["workspaceId"] != "workspace-1" || caller.args["folderId"] != "folder-1" || caller.args["pageSize"] != 20 {
		t.Fatalf("args = %#v", caller.args)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, stdout.String())
	}
	if payload["ok"] != true || payload["outcome"] != "success" {
		t.Fatalf("payload = %#v", payload)
	}
	if _, present := payload["contract_version"]; present {
		t.Fatalf("unified result must not carry a version marker: %#v", payload)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["count"] != float64(1) || data["paginationKnown"] != false {
		t.Fatalf("data = %#v", payload["data"])
	}
	meta, ok := payload["meta"].(map[string]any)
	if !ok || meta["count"] != float64(1) {
		t.Fatalf("meta = %#v", payload["meta"])
	}
}

func TestNodeListRejectsInvalidLimitBeforeRemoteCall(t *testing.T) {
	for _, limit := range []string{"0", "51", "-1"} {
		t.Run(limit, func(t *testing.T) {
			caller := &nodeListRoutingCaller{}
			helpers.InitDeps(caller)
			cmd := corecmd.New(shortcut.FromShortcut(NodeList))
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--workspace", "workspace-1", "--limit", limit})
			err := cmd.Execute()
			if err == nil {
				t.Fatal("invalid limit unexpectedly succeeded")
			}
			var appErr *apperrors.Error
			if !errors.As(err, &appErr) || appErr.StableSubtype != string(apperrors.SubtypeInvalidFlagValue) || appErr.ExitCode() != 3 {
				t.Fatalf("error = %#v, want validation/invalid_flag_value rc=3", err)
			}
			if caller.calls != 0 {
				t.Fatalf("remote calls = %d, want 0", caller.calls)
			}
		})
	}
}

func TestNodeListFailsClosedOnProjectionAndPaginationUncertainty(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		wantReason    string
		wantExhausted any
		wantCursor    string
	}{
		{
			name:          "terminal page",
			text:          `{"result":{"nodes":[],"hasMore":false}}`,
			wantExhausted: true,
		},
		{
			name:          "continuation page",
			text:          `{"result":{"nodes":[],"hasMore":true,"nextCursor":"page-2"}}`,
			wantExhausted: false,
			wantCursor:    "page-2",
		},
		{
			name:          "real service next page token",
			text:          `{"nodes":[],"hasMore":true,"nextPageToken":"pos:-1245174.5"}`,
			wantExhausted: false,
			wantCursor:    "pos:-1245174.5",
		},
		{
			name:       "unknown projection",
			text:       `{"result":{"unexpected":[]}}`,
			wantReason: "projection_unknown",
		},
		{
			name:       "non object row",
			text:       `{"nodes":["invalid"],"hasMore":false}`,
			wantReason: "projection_unknown",
		},
		{
			name:       "row without stable node id",
			text:       `{"nodes":[{"name":"display only"}],"hasMore":false}`,
			wantReason: "projection_unknown",
		},
		{
			name:       "row with non string node id",
			text:       `{"nodes":[{"nodeId":42}],"hasMore":false}`,
			wantReason: "projection_unknown",
		},
		{
			name:       "row with invalid presentation type",
			text:       `{"nodes":[{"nodeId":"node-1","name":{},"nodeType":"doc"}],"hasMore":false}`,
			wantReason: "projection_unknown",
		},
		{
			name:       "continuation without cursor",
			text:       `{"result":{"nodes":[],"hasMore":true}}`,
			wantReason: "pagination_inconsistent",
		},
		{
			name:       "terminal with cursor",
			text:       `{"result":{"nodes":[],"hasMore":false,"nextCursor":"contradiction"}}`,
			wantReason: "pagination_inconsistent",
		},
		{
			name:       "outer and nested pagination contradict",
			text:       `{"hasMore":false,"result":{"nodes":[],"hasMore":true,"nextCursor":"page-2"}}`,
			wantReason: "pagination_inconsistent",
		},
		{
			name:          "terminal numeric cursor sentinel",
			text:          `{"result":{"nodes":[],"hasMore":false,"nextCursor":0}}`,
			wantExhausted: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &nodeListRoutingCaller{text: test.text}
			helpers.InitDeps(caller)
			cmd := corecmd.New(shortcut.FromShortcut(NodeList))
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			ctx, _ := output.WithResultStore(context.Background())
			cmd.SetContext(ctx)
			cmd.SetArgs([]string{"--workspace", "workspace-1"})
			err := cmd.Execute()
			if caller.calls != 1 {
				t.Fatalf("calls = %d, want exactly one", caller.calls)
			}
			if test.wantReason != "" {
				if err == nil {
					t.Fatal("expected typed failure")
				}
				var appErr *apperrors.Error
				if !errors.As(err, &appErr) || appErr.Reason != test.wantReason {
					t.Fatalf("error = %T %v, want reason %q", err, err, test.wantReason)
				}
				if stdout.Len() != 0 {
					t.Fatalf("failure leaked success payload: %q", stdout.String())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, emitted, err := output.EmitStoredResult(cmd); err != nil || !emitted {
				t.Fatalf("emit unified result: emitted=%v err=%v", emitted, err)
			}
			var payload map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("decode output: %v\n%s", err, stdout.String())
			}
			data, ok := payload["data"].(map[string]any)
			meta, metaOK := payload["meta"].(map[string]any)
			page, pageOK := meta["pagination"].(map[string]any)
			if !ok || !metaOK || !pageOK || data["paginationKnown"] != true || page["endpoint_exhausted"] != test.wantExhausted {
				t.Fatalf("unified pagination = %#v", payload)
			}
			if test.wantCursor != "" && (data["nextCursor"] != test.wantCursor || page["next_token"] != test.wantCursor) {
				t.Fatalf("pagination payload=%#v meta=%#v, want cursor %q", data, page, test.wantCursor)
			}
		})
	}
}
