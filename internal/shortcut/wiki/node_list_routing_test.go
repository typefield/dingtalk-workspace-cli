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
	cmd.SetArgs([]string{"--workspace", "workspace-1", "--folder", "folder-1", "--limit", "20"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
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
	if payload["count"] != float64(1) {
		t.Fatalf("payload = %#v", payload)
	}
	if payload["paginationKnown"] != false {
		t.Fatalf("paginationKnown = %#v, want false without server evidence", payload["paginationKnown"])
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
			name:       "unknown projection",
			text:       `{"result":{"unexpected":[]}}`,
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
			var payload map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("decode output: %v\n%s", err, stdout.String())
			}
			if payload["endpointExhausted"] != test.wantExhausted || payload["paginationKnown"] != true {
				t.Fatalf("pagination = %#v", payload)
			}
			if test.wantCursor != "" && payload["nextCursor"] != test.wantCursor {
				t.Fatalf("nextCursor = %#v, want %q", payload["nextCursor"], test.wantCursor)
			}
		})
	}
}
