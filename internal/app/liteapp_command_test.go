// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/publishedmcp"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/transport"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/cobra"
)

// liteappTestCaller 记录 mcp-meta 端点解析调用，并返回预置接入地址。
type liteappTestCaller struct {
	mu       sync.Mutex
	metaArgs map[string]any
	result   *edition.ToolResult
	err      error
}

func (c *liteappTestCaller) CallTool(_ context.Context, productID, toolName string, args map[string]any) (*edition.ToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if productID == mcpMetaServerID && toolName == mcpMetaURLTool {
		c.metaArgs = args
		return &edition.ToolResult{Content: []edition.ContentBlock{
			{Type: "text", Text: `{"mcpURL":"https://pre-mcp-gw.example.com/mcp/10357"}`},
		}}, nil
	}
	if c.err != nil {
		return nil, c.err
	}
	return c.result, nil
}

func (*liteappTestCaller) Format() string { return "json" }
func (*liteappTestCaller) DryRun() bool   { return false }
func (*liteappTestCaller) Fields() string { return "" }
func (*liteappTestCaller) JQ() string     { return "" }

type liteappTestTransport struct {
	mu       sync.Mutex
	endpoint string
	tool     string
	args     map[string]any
	result   transport.ToolCallResult
}

func (t *liteappTestTransport) Tools(_ context.Context, endpoint string) (transport.ToolsListResult, error) {
	return transport.ToolsListResult{}, nil
}

func (t *liteappTestTransport) InvokeValidated(_ context.Context, endpoint, tool string, arguments map[string]any) (publishedmcp.ValidatedInvocationResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.endpoint = endpoint
	t.tool = tool
	t.args = arguments
	return publishedmcp.ValidatedInvocationResult{
		InputSchemaValidation: "fresh_core_subset_snapshot",
		InputSchemaDigest:     "digest-001",
		Result:                t.result,
	}, nil
}

func executeLiteappCommand(t *testing.T, caller edition.ToolCaller, factory mcpPublishedTransportFactory, args ...string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "dws", SilenceErrors: true, SilenceUsage: true}
	bindPersistentFlags(root, &GlobalFlags{})
	root.AddCommand(newLiteappGroup(caller, factory))
	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestRootRegistersLiteappUnderDev(t *testing.T) {
	root := NewRootCommand(t.Context())
	dev, _, err := root.Find([]string{"dev"})
	if err != nil || dev == nil || dev.Name() != "dev" {
		t.Fatalf("find dev: %v", err)
	}
	cmd, _, err := root.Find([]string{"dev", "liteapp", "create"})
	if err != nil {
		t.Fatalf("find dev liteapp create: %v", err)
	}
	if got := cmd.CommandPath(); got != "dws dev liteapp create" {
		t.Fatalf("command path = %q, want %q", got, "dws dev liteapp create")
	}
	if _, _, err := root.Find([]string{"liteapp"}); err == nil {
		top, _, _ := root.Find([]string{"liteapp"})
		if top != nil && top.Parent() == root {
			t.Fatal("liteapp must not be a top-level command")
		}
	}
}

func TestLiteappCreateDryRunDoesNotTouchNetwork(t *testing.T) {
	caller := &liteappTestCaller{}
	factory := func(context.Context) (mcpPublishedTransport, error) {
		t.Fatal("factory must not be called during dry-run")
		return nil, nil
	}
	out, err := executeLiteappCommand(t, caller, factory, "liteapp", "create", "--name", "周报助手", "--homepage-url", "https://example.com", "--request-id", "req-1", "--dry-run", "--format", "json")
	if err != nil {
		t.Fatalf("execute create dry-run: %v", err)
	}
	if caller.metaArgs != nil {
		t.Fatalf("mcp-meta must not be called during dry-run, got %#v", caller.metaArgs)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if payload["dry_run"] != true || payload["executed"] != false {
		t.Fatalf("payload = %#v, want dry-run invocation preview", payload)
	}
	if payload["mcpId"] != liteappDefaultMCPID {
		t.Fatalf("mcpId = %#v, want default %s", payload["mcpId"], liteappDefaultMCPID)
	}
	arguments, _ := payload["arguments"].(map[string]any)
	if arguments["appName"] != "周报助手" || arguments["homepageUrl"] != "https://example.com" || arguments["requestId"] != "req-1" {
		t.Fatalf("arguments = %#v", arguments)
	}
}

func TestLiteappCreateRequiresYesForRealRun(t *testing.T) {
	_, err := executeLiteappCommand(t, &liteappTestCaller{}, nil, "liteapp", "create",
		"--name", "周报助手", "--homepage-url", "https://example.com", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %v, want confirmation_required for mutating run", err)
	}
}

func TestLiteappCreateDefaultsToAppManagementMCP(t *testing.T) {
	caller := &liteappTestCaller{}
	transport := &liteappTestTransport{result: transport.ToolCallResult{}}
	out, err := executeLiteappCommand(t, caller, func(context.Context) (mcpPublishedTransport, error) {
		return transport, nil
	}, "liteapp", "create",
		"--name", "周报助手", "--homepage-url", "https://example.com", "--desc", "desc-1",
		"--request-id", "req-1", "--yes", "--format", "json")
	if err != nil {
		t.Fatalf("execute create: %v", err)
	}
	if caller.metaArgs["mcpId"] != liteappDefaultMCPID {
		t.Fatalf("mcp-meta mcpId = %#v, want default %s", caller.metaArgs["mcpId"], liteappDefaultMCPID)
	}
	if transport.tool != liteappCreateTool {
		t.Fatalf("tool = %q, want %q", transport.tool, liteappCreateTool)
	}
	if transport.endpoint == "" {
		t.Fatalf("endpoint = %q, want resolved endpoint", transport.endpoint)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if payload["mcpId"] != liteappDefaultMCPID {
		t.Fatalf("mcpId = %#v, want default %s", payload["mcpId"], liteappDefaultMCPID)
	}
	if payload["inputSchemaValidation"] != "fresh_core_subset_snapshot" {
		t.Fatalf("inputSchemaValidation = %#v", payload["inputSchemaValidation"])
	}
	if transport.args["desc"] != "desc-1" {
		t.Fatalf("arguments = %#v, want desc passed through", transport.args)
	}
}

func TestLiteappExplicitMCPIDOverridesDefault(t *testing.T) {
	caller := &liteappTestCaller{}
	transport := &liteappTestTransport{result: transport.ToolCallResult{}}
	out, err := executeLiteappCommand(t, caller, func(context.Context) (mcpPublishedTransport, error) {
		return transport, nil
	}, "liteapp", "list", "--mcp-id", "9999", "--format", "json")
	if err != nil {
		t.Fatalf("execute list: %v", err)
	}
	if caller.metaArgs["mcpId"] != "9999" {
		t.Fatalf("mcp-meta mcpId = %#v, want explicit 9999", caller.metaArgs["mcpId"])
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if payload["mcpId"] != "9999" {
		t.Fatalf("mcpId = %#v, want explicit 9999", payload["mcpId"])
	}
	if transport.tool != liteappListTool {
		t.Fatalf("tool = %q, want %q", transport.tool, liteappListTool)
	}
}

func TestLiteappBlankMCPIDFallsBackToDefault(t *testing.T) {
	caller := &liteappTestCaller{}
	transport := &liteappTestTransport{result: transport.ToolCallResult{}}
	out, err := executeLiteappCommand(t, caller, func(context.Context) (mcpPublishedTransport, error) {
		return transport, nil
	}, "liteapp", "list", "--mcp-id", "  ", "--format", "json")
	if err != nil {
		t.Fatalf("execute list: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if payload["mcpId"] != liteappDefaultMCPID {
		t.Fatalf("mcpId = %#v, want default %s for blank flag", payload["mcpId"], liteappDefaultMCPID)
	}
}

func TestLiteappCreateRejectsBlankRequiredFlags(t *testing.T) {
	_, err := executeLiteappCommand(t, &liteappTestCaller{}, nil,
		"liteapp", "create", "--name", "周报助手", "--yes")
	if err == nil || !strings.Contains(err.Error(), "--homepage-url 不能为空") {
		t.Fatalf("error = %v, want blank homepage-url error", err)
	}
}

func TestLiteappUpdateRequiresAtLeastOneField(t *testing.T) {
	_, err := executeLiteappCommand(t, &liteappTestCaller{}, nil,
		"liteapp", "update", "5005426001", "--yes")
	if err == nil || !strings.Contains(err.Error(), "至少提供一个要修改的字段") {
		t.Fatalf("error = %v, want at-least-one-field error", err)
	}
}

func TestLiteappUpdateParsesRedirectUrisAndInvokes(t *testing.T) {
	caller := &liteappTestCaller{}
	transport := &liteappTestTransport{result: transport.ToolCallResult{}}
	_, err := executeLiteappCommand(t, caller, func(context.Context) (mcpPublishedTransport, error) {
		return transport, nil
	}, "liteapp", "update", "5005426001",
		"--redirect-uris", "https://a.example.com/cb, https://b.example.com/cb", "--yes", "--format", "json")
	if err != nil {
		t.Fatalf("execute update: %v", err)
	}
	if transport.tool != liteappUpdateTool {
		t.Fatalf("tool = %q, want %q", transport.tool, liteappUpdateTool)
	}
	uris, _ := transport.args["redirectUris"].([]string)
	if len(uris) != 2 {
		t.Fatalf("redirectUris = %#v, want 2 entries", transport.args["redirectUris"])
	}
	if uris[0] != "https://a.example.com/cb" || uris[1] != "https://b.example.com/cb" {
		t.Fatalf("redirectUris = %#v, want trimmed entries", transport.args["redirectUris"])
	}
}

func TestLiteappDeleteInvokesWithParsedAppID(t *testing.T) {
	caller := &liteappTestCaller{}
	transport := &liteappTestTransport{result: transport.ToolCallResult{}}
	if _, err := executeLiteappCommand(t, caller, func(context.Context) (mcpPublishedTransport, error) {
		return transport, nil
	}, "liteapp", "delete", "5005426001", "--yes"); err != nil {
		t.Fatalf("execute delete: %v", err)
	}
	if transport.tool != liteappDeleteTool {
		t.Fatalf("tool = %q, want %q", transport.tool, liteappDeleteTool)
	}
	if transport.args["appId"] != int64(5005426001) {
		t.Fatalf("appId = %#v, want int64 5005426001", transport.args["appId"])
	}
}

func TestLiteappListAndDetailAreReadOpsWithoutYes(t *testing.T) {
	caller := &liteappTestCaller{}
	transport := &liteappTestTransport{result: transport.ToolCallResult{}}
	if _, err := executeLiteappCommand(t, caller, func(context.Context) (mcpPublishedTransport, error) {
		return transport, nil
	}, "liteapp", "list", "--size", "5"); err != nil {
		t.Fatalf("execute list: %v", err)
	}
	if transport.args["size"] != 5 {
		t.Fatalf("size = %#v, want 5", transport.args["size"])
	}
	if _, err := executeLiteappCommand(t, caller, func(context.Context) (mcpPublishedTransport, error) {
		return transport, nil
	}, "liteapp", "detail", "5005426001"); err != nil {
		t.Fatalf("execute detail: %v", err)
	}
	if transport.tool != liteappDetailTool {
		t.Fatalf("tool = %q, want %q", transport.tool, liteappDetailTool)
	}
}
