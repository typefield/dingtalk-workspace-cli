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

// liteappTestCaller 记录 mcpdev 侧调用，并按 toolName 返回预置结果。
type liteappTestCaller struct {
	mu        sync.Mutex
	listCalls int
	result    *edition.ToolResult
	err       error
}

func (c *liteappTestCaller) CallTool(_ context.Context, productID, toolName string, _ map[string]any) (*edition.ToolResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if productID == liteappMCPDevServerID && toolName == liteappMCPDevListTool {
		c.listCalls++
	}
	if c.err != nil {
		return nil, c.err
	}
	if productID == mcpMetaServerID {
		return &edition.ToolResult{Content: []edition.ContentBlock{
			{Type: "text", Text: `{"mcpURL":"https://pre-mcp-gw.example.com/mcp/10718"}`},
		}}, nil
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

const liteappTestServiceListResponse = `{"ok":true,"data":{"services":[
	{"mcpId":9999,"serverName":null},
	{"mcpId":10718,"serverName":"dingtalk-lite-app"}
]}}`

func liteappTestCallerWithServiceList() *liteappTestCaller {
	return &liteappTestCaller{
		result: &edition.ToolResult{Content: []edition.ContentBlock{
			{Type: "text", Text: liteappTestServiceListResponse},
		}},
	}
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

func TestLiteappCreateDryRunDoesNotResolveOrCall(t *testing.T) {
	caller := liteappTestCallerWithServiceList()
	factory := func(context.Context) (mcpPublishedTransport, error) {
		t.Fatal("factory must not be called during dry-run")
		return nil, nil
	}
	out, err := executeLiteappCommand(t, caller, factory, "liteapp", "create", "--name", "周报助手", "--homepage-url", "https://example.com", "--request-id", "req-1", "--dry-run", "--format", "json")
	if err != nil {
		t.Fatalf("execute create dry-run: %v", err)
	}
	if caller.listCalls != 0 {
		t.Fatalf("mcpdev service list must not be called during dry-run, got %d calls", caller.listCalls)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if payload["dry_run"] != true || payload["executed"] != false {
		t.Fatalf("payload = %#v, want dry-run invocation preview", payload)
	}
	arguments, _ := payload["arguments"].(map[string]any)
	if arguments["appName"] != "周报助手" || arguments["homepageUrl"] != "https://example.com" || arguments["requestId"] != "req-1" {
		t.Fatalf("arguments = %#v", arguments)
	}
}

func TestLiteappCreateRequiresYesForRealRun(t *testing.T) {
	caller := liteappTestCallerWithServiceList()
	_, err := executeLiteappCommand(t, caller, nil, "liteapp", "create",
		"--name", "周报助手", "--homepage-url", "https://example.com", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %v, want confirmation_required for mutating run", err)
	}
}

func TestLiteappCreateResolvesServerNameAndInvokes(t *testing.T) {
	caller := liteappTestCallerWithServiceList()
	transport := &liteappTestTransport{result: transport.ToolCallResult{}}
	out, err := executeLiteappCommand(t, caller, func(context.Context) (mcpPublishedTransport, error) {
		return transport, nil
	}, "liteapp", "create",
		"--name", "周报助手", "--homepage-url", "https://example.com", "--desc", "desc-1",
		"--request-id", "req-1", "--yes", "--format", "json")
	if err != nil {
		t.Fatalf("execute create: %v", err)
	}
	if caller.listCalls != 1 {
		t.Fatalf("mcpdev service list calls = %d, want 1", caller.listCalls)
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
	if payload["mcpId"] != "10718" {
		t.Fatalf("mcpId = %#v, want resolved 10718", payload["mcpId"])
	}
	if payload["inputSchemaValidation"] != "fresh_core_subset_snapshot" {
		t.Fatalf("inputSchemaValidation = %#v", payload["inputSchemaValidation"])
	}
	if transport.args["desc"] != "desc-1" {
		t.Fatalf("arguments = %#v, want desc passed through", transport.args)
	}
}

func TestLiteappCreateRejectsBlankRequiredFlags(t *testing.T) {
	_, err := executeLiteappCommand(t, liteappTestCallerWithServiceList(), nil,
		"liteapp", "create", "--name", "周报助手", "--yes")
	if err == nil || !strings.Contains(err.Error(), "--homepage-url 不能为空") {
		t.Fatalf("error = %v, want blank homepage-url error", err)
	}
}

func TestLiteappUpdateRequiresAtLeastOneField(t *testing.T) {
	_, err := executeLiteappCommand(t, liteappTestCallerWithServiceList(), nil,
		"liteapp", "update", "5005426001", "--yes")
	if err == nil || !strings.Contains(err.Error(), "至少提供一个要修改的字段") {
		t.Fatalf("error = %v, want at-least-one-field error", err)
	}
}

func TestLiteappUpdateParsesRedirectUrisAndInvokes(t *testing.T) {
	caller := liteappTestCallerWithServiceList()
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
	caller := liteappTestCallerWithServiceList()
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
	caller := liteappTestCallerWithServiceList()
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

func TestLiteappUnresolvableServerNameHintsExplicitMcpID(t *testing.T) {
	empty := &liteappTestCaller{
		result: &edition.ToolResult{Content: []edition.ContentBlock{
			{Type: "text", Text: `{"ok":true,"data":{"services":[]}}`},
		}},
	}
	_, err := executeLiteappCommand(t, empty, nil, "liteapp", "list", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "--mcp-id") {
		t.Fatalf("error = %v, want unresolved serverName hint", err)
	}
}
