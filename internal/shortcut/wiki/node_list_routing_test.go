// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package wiki

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type nodeListRoutingCaller struct {
	product string
	tool    string
	args    map[string]any
}

func (c *nodeListRoutingCaller) CallTool(_ context.Context, product, tool string, args map[string]any) (*edition.ToolResult, error) {
	c.product, c.tool, c.args = product, tool, args
	return &edition.ToolResult{Content: []edition.ContentBlock{{
		Type: "text",
		Text: `{"result":{"nodes":[{"name":"Guide","nodeId":"node-1","nodeType":"doc"}]}}`,
	}}}, nil
}

func (*nodeListRoutingCaller) Format() string { return "json" }
func (*nodeListRoutingCaller) DryRun() bool   { return false }
func (*nodeListRoutingCaller) Fields() string { return "" }
func (*nodeListRoutingCaller) JQ() string     { return "" }

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
}
