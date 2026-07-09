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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/executor"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/transport"
	"github.com/spf13/cobra"
)

type mcpHTTPTestRunner struct {
	invocation executor.Invocation
}

func (r *mcpHTTPTestRunner) Run(_ context.Context, invocation executor.Invocation) (executor.Result, error) {
	r.invocation = invocation
	return executor.Result{
		Invocation: invocation,
		Response: map[string]any{
			"content": map[string]any{"success": true},
		},
	}, nil
}

func TestMCPHTTPDiscoveryURL(t *testing.T) {
	got, err := mcpHTTPDiscoveryURL("https://pre-aihub.dingtalk.com", 1, 10, "")
	if err != nil {
		t.Fatalf("mcpHTTPDiscoveryURL() error = %v", err)
	}
	want := "https://pre-aihub.dingtalk.com/cli/discovery/mcp?keyword=&page=1&pageSize=10"
	if got != want {
		t.Fatalf("mcpHTTPDiscoveryURL() = %q, want %q", got, want)
	}

	source := mcpHTTPCommandCacheSource("https://pre-aihub.dingtalk.com/cli/discovery/mcp")
	if strings.Count(source, "/cli/discovery/mcp") != 1 {
		t.Fatalf("cache source duplicated discovery path: %q", source)
	}
}

func TestMCPHTTPDiscoveryBaseURLDefaultsToPre(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())
	t.Setenv(mcpHTTPCommandDiscoveryEnv, "")

	got, err := mcpHTTPDiscoveryBaseURL()
	if err != nil {
		t.Fatalf("mcpHTTPDiscoveryBaseURL() error = %v", err)
	}
	if got != preAIHubBaseURL {
		t.Fatalf("mcpHTTPDiscoveryBaseURL() = %q, want %q", got, preAIHubBaseURL)
	}
}

func TestFetchMCPHTTPCommandListFromPublishedDiscovery(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())
	ResetRuntimeTokenCache()
	t.Cleanup(ResetRuntimeTokenCache)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cli/discovery/mcp":
			if r.Method != http.MethodGet {
				t.Fatalf("discovery method = %s, want GET", r.Method)
			}
			if got := r.URL.Query().Get("page"); got != "1" {
				t.Fatalf("page = %q, want 1", got)
			}
			if got := r.URL.Query().Get("pageSize"); got != "100" {
				t.Fatalf("pageSize = %q, want 100", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": map[string]any{
					"values": []map[string]any{{
						"mcpId":       1001,
						"name":        "Weather MCP",
						"description": "Weather service",
						"icon":        "",
						"mcpUrl":      server.URL + "/server/org-1001",
					}},
					"currentPage": 1,
					"pageSize":    100,
					"totalCount":  1,
					"totalPages":  1,
				},
			})
		case "/server/org-1001":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode JSON-RPC request: %v", err)
			}
			if req["method"] != "tools/list" {
				t.Fatalf("JSON-RPC method = %#v, want tools/list", req["method"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]any{
					"tools": []map[string]any{{
						"name":        "get_weather",
						"description": "Get weather",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"cityName": map[string]any{"type": "string", "description": "city"},
								"limit":    map[string]any{"type": "integer"},
								"options":  map[string]any{"type": "object"},
							},
							"required": []any{"cityName"},
						},
					}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	commands, err := fetchMCPHTTPCommandList(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetchMCPHTTPCommandList() error = %v", err)
	}
	if len(commands) != 2 {
		t.Fatalf("len(commands) = %d, want 2", len(commands))
	}
	fixedPath := []string{"weather-mcp", "get-weather"}
	debugPath := []string{"connector", "mcp", "published", "weather-mcp", "get-weather"}
	cmd := findMCPHTTPTestCommandByPath(commands, fixedPath)
	if cmd == nil {
		t.Fatalf("fixed path command missing: %#v", commands)
	}
	if findMCPHTTPTestCommandByPath(commands, debugPath) == nil {
		t.Fatalf("debug path command missing: %#v", commands)
	}
	if cmd.ProductID != "published-mcp-1001" {
		t.Fatalf("productId = %q, want published-mcp-1001", cmd.ProductID)
	}
	if cmd.Tool != "get_weather" {
		t.Fatalf("tool = %q, want get_weather", cmd.Tool)
	}
	specs := mcpHTTPFlagSpecs(cmd.InputSchema)
	var gotFlags []string
	for _, spec := range specs {
		gotFlags = append(gotFlags, spec.FlagName)
		if spec.FlagName == "city-name" && !spec.Required {
			t.Fatal("city-name should be required")
		}
	}
	wantFlags := []string{"city-name", "limit", "options"}
	if !reflect.DeepEqual(gotFlags, wantFlags) {
		t.Fatalf("flags = %#v, want %#v", gotFlags, wantFlags)
	}
}

func TestMCPHTTPCommandCacheTTL(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())
	endpoint := "https://mcp-gw.example/server/weather?key=secret"
	commands := []mcpHTTPCommandDescriptor{{
		Path:      []string{"connector", "weather"},
		ProductID: "weather",
		Tool:      "get_weather",
	}}
	if err := writeMCPHTTPCommandCache(endpoint, commands); err != nil {
		t.Fatalf("writeMCPHTTPCommandCache() error = %v", err)
	}
	got, fresh := readMCPHTTPCommandCache(endpoint)
	if !fresh {
		t.Fatal("fresh = false, want true")
	}
	if len(got) != 1 || got[0].Tool != "get_weather" {
		t.Fatalf("cached commands = %#v", got)
	}

	path, err := mcpHTTPCommandCachePath(endpoint)
	if err != nil {
		t.Fatalf("mcpHTTPCommandCachePath() error = %v", err)
	}
	old := mcpHTTPCommandCacheFile{
		FetchedAt: time.Now().Add(-11 * time.Minute),
		Commands:  commands,
	}
	data, _ := json.Marshal(old)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(old cache) error = %v", err)
	}
	got, fresh = readMCPHTTPCommandCache(endpoint)
	if fresh {
		t.Fatal("fresh = true for expired cache, want false")
	}
	if len(got) != 1 {
		t.Fatalf("expired cache should still return stale commands, got %#v", got)
	}

	if strings.Contains(filepath.Base(path), "secret") {
		t.Fatalf("cache file path leaked key: %s", path)
	}
}

func TestMCPHTTPCommandsFromPublishedToolFallsBackToToolName(t *testing.T) {
	commands := mcpHTTPCommandsFromPublishedTool(publishedMCPService{
		MCPID: json.Number("1001"),
	}, "https://mcp-gw.example/server/org-1001", transport.ToolDescriptor{
		Name:        "baidu_search_suggest",
		Description: "Baidu suggest",
	})

	if findMCPHTTPTestCommandByPath(commands, []string{"baidu-search-suggest"}) == nil {
		t.Fatalf("tool-name fixed command missing: %#v", commands)
	}
	if findMCPHTTPTestCommandByPath(commands, []string{"connector", "mcp", "published", "baidu-search-suggest", "baidu-search-suggest"}) == nil {
		t.Fatalf("tool-name debug command missing: %#v", commands)
	}
}

func TestMCPHTTPCommandCacheMigratesLegacyConnectMCPPath(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())
	endpoint := "https://mcp-gw.example/server/weather"
	path, err := mcpHTTPCommandCachePath(endpoint)
	if err != nil {
		t.Fatalf("mcpHTTPCommandCachePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(cache dir) error = %v", err)
	}
	legacy := mcpHTTPCommandCacheFile{
		FetchedAt: time.Now(),
		Commands: []mcpHTTPCommandDescriptor{{
			Path:      []string{"connect", "mcp", "published", "weather", "today"},
			ProductID: "weather",
			Tool:      "today",
		}},
	}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(legacy cache) error = %v", err)
	}

	got, fresh := readMCPHTTPCommandCache(endpoint)
	if !fresh {
		t.Fatal("fresh = false, want true")
	}
	wantPath := []string{"connector", "mcp", "published", "weather", "today"}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Path, wantPath) {
		t.Fatalf("cached path = %#v, want %#v", got, wantPath)
	}
}

func TestPublishedMCPURLForIDUsesDiscoveryEnvironment(t *testing.T) {
	t.Setenv(mcpHTTPCommandDiscoveryEnv, "https://pre-aihub.dingtalk.com")
	got, err := publishedMCPURLForID("10487")
	if err != nil {
		t.Fatalf("publishedMCPURLForID() error = %v", err)
	}
	want := "https://pre-mcp-gw.dingtalk.com/server/org-10487"
	if got != want {
		t.Fatalf("publishedMCPURLForID() = %q, want %q", got, want)
	}
}

func TestMergeMCPHTTPCommandCacheForProductPreservesExistingServicePath(t *testing.T) {
	existing := []mcpHTTPCommandDescriptor{
		{
			Path:      []string{"connector", "mcp", "published", "weather-service", "old-tool"},
			ProductID: "published-mcp-1001",
			Tool:      "old_tool",
		},
		{
			Path:      []string{"connector", "mcp", "published", "todo", "get-todo"},
			ProductID: "published-mcp-2002",
			Tool:      "get_todo",
		},
	}
	replacement := []mcpHTTPCommandDescriptor{
		{
			Path:      []string{"mcp-1001", "new-tool"},
			ProductID: "published-mcp-1001",
			Tool:      "new_tool",
		},
		{
			Path:      []string{"connector", "mcp", "published", "mcp-1001", "new-tool"},
			ProductID: "published-mcp-1001",
			Tool:      "new_tool",
		},
	}

	got := mergeMCPHTTPCommandCacheForProduct(existing, replacement, "published-mcp-1001")
	if len(got) != 3 {
		t.Fatalf("len(merged) = %d, want 3: %#v", len(got), got)
	}
	if findMCPHTTPTestCommand(got, "old_tool") != nil {
		t.Fatalf("old product command should be replaced: %#v", got)
	}
	wantFixedPath := []string{"weather-service", "new-tool"}
	if findMCPHTTPTestCommandByPath(got, wantFixedPath) == nil {
		t.Fatalf("fixed new command missing: %#v", got)
	}
	wantDebugPath := []string{"connector", "mcp", "published", "weather-service", "new-tool"}
	if findMCPHTTPTestCommandByPath(got, wantDebugPath) == nil {
		t.Fatalf("debug new command missing: %#v", got)
	}
	if findMCPHTTPTestCommand(got, "get_todo") == nil {
		t.Fatalf("unrelated product command should be preserved: %#v", got)
	}
}

func TestRefreshCurrentPublishedMCPCommandCacheMergesDirectToolsList(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())
	ResetRuntimeTokenCache()
	t.Cleanup(ResetRuntimeTokenCache)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/org-1001" {
			http.NotFound(w, r)
			return
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode JSON-RPC request: %v", err)
		}
		if req["method"] != "tools/list" {
			t.Fatalf("JSON-RPC method = %#v, want tools/list", req["method"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]any{
				"tools": []map[string]any{{
					"name":        "new_weather",
					"description": "New weather",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"city": map[string]any{"type": "string"},
						},
					},
				}},
			},
		})
	}))
	defer server.Close()
	t.Setenv(mcpHTTPCommandDiscoveryEnv, server.URL)

	source, err := currentMCPHTTPCommandCacheSource()
	if err != nil {
		t.Fatalf("currentMCPHTTPCommandCacheSource() error = %v", err)
	}
	existing := []mcpHTTPCommandDescriptor{{
		Path:      []string{"connector", "mcp", "published", "weather-service", "old-weather"},
		ProductID: "published-mcp-1001",
		Tool:      "old_weather",
	}}
	if err := writeMCPHTTPCommandCache(source, existing); err != nil {
		t.Fatalf("writeMCPHTTPCommandCache() error = %v", err)
	}

	ok, err := refreshCurrentPublishedMCPCommandCache(context.Background(), executor.Invocation{
		Tool:   "mcp_tool_publish",
		Params: map[string]any{"mcpId": 1001},
	}, executor.Result{})
	if err != nil {
		t.Fatalf("refreshCurrentPublishedMCPCommandCache() error = %v", err)
	}
	if !ok {
		t.Fatal("refreshCurrentPublishedMCPCommandCache() ok = false, want true")
	}
	got, fresh := readMCPHTTPCommandCache(source)
	if !fresh {
		t.Fatal("cache should be fresh after direct refresh")
	}
	if findMCPHTTPTestCommand(got, "old_weather") != nil {
		t.Fatalf("old tool should be replaced: %#v", got)
	}
	newCommand := findMCPHTTPTestCommand(got, "new_weather")
	if newCommand == nil {
		t.Fatalf("new tool missing from cache: %#v", got)
	}
	wantPath := []string{"connector", "mcp", "published", "weather-service", "new-weather"}
	if !reflect.DeepEqual(newCommand.Path, wantPath) {
		t.Fatalf("new command path = %#v, want %#v", newCommand.Path, wantPath)
	}
	wantFixedPath := []string{"weather-service", "new-weather"}
	if findMCPHTTPTestCommandByPath(got, wantFixedPath) == nil {
		t.Fatalf("fixed new command missing from cache: %#v", got)
	}
}

func findMCPHTTPTestCommand(commands []mcpHTTPCommandDescriptor, tool string) *mcpHTTPCommandDescriptor {
	for i := range commands {
		if commands[i].Tool == tool {
			return &commands[i]
		}
	}
	return nil
}

func findMCPHTTPTestCommandByPath(commands []mcpHTTPCommandDescriptor, path []string) *mcpHTTPCommandDescriptor {
	for i := range commands {
		if reflect.DeepEqual(commands[i].Path, path) {
			return &commands[i]
		}
	}
	return nil
}

func TestShouldRefreshMCPHTTPCommandsAfterPublishedToolPublish(t *testing.T) {
	invocation := executor.Invocation{
		CanonicalProduct: "published-mcp-10487",
		Tool:             "mcp_tool_publish",
	}
	if !shouldRefreshMCPHTTPCommandsAfterInvocation(invocation, nil) {
		t.Fatal("published MCP tool publish should refresh dynamic command cache")
	}

	invocation.DryRun = true
	if shouldRefreshMCPHTTPCommandsAfterInvocation(invocation, nil) {
		t.Fatal("dry-run publish should not refresh dynamic command cache")
	}
	invocation.DryRun = false
	if shouldRefreshMCPHTTPCommandsAfterInvocation(invocation, &GlobalFlags{DryRun: true}) {
		t.Fatal("global dry-run should not refresh dynamic command cache")
	}

	invocation.Tool = "create_document"
	if shouldRefreshMCPHTTPCommandsAfterInvocation(invocation, nil) {
		t.Fatal("unrelated tool should not refresh dynamic command cache")
	}
}

func TestAddMCPHTTPCommandsRegistersFixedRunnableCommand(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")

	runner := &mcpHTTPTestRunner{}
	addMCPHTTPCommands(root, runner, nil, []mcpHTTPCommandDescriptor{{
		Path:      []string{"weather-service", "get-weather"},
		ProductID: "published-mcp-1001",
		Tool:      "get_weather",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cityName": map[string]any{"type": "string"},
			},
			"required": []any{"cityName"},
		},
	}})

	if findCommandByPath(root, []string{"weather-service", "get-weather"}) == nil {
		t.Fatal("fixed dynamic command was not registered")
	}
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{
		"weather-service", "get-weather",
		"--city-name", "杭州",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.invocation.CanonicalProduct != "published-mcp-1001" {
		t.Fatalf("product = %q, want published-mcp-1001", runner.invocation.CanonicalProduct)
	}
	if runner.invocation.Tool != "get_weather" {
		t.Fatalf("tool = %q, want get_weather", runner.invocation.Tool)
	}
	if runner.invocation.Params["cityName"] != "杭州" {
		t.Fatalf("cityName param = %#v", runner.invocation.Params["cityName"])
	}
}

func TestMCPHTTPDynamicCommandHelpIsAgentFriendly(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	addMCPHTTPCommands(root, &mcpHTTPTestRunner{}, nil, []mcpHTTPCommandDescriptor{{
		Path:        []string{"weather-service", "get-weather"},
		ProductID:   "published-mcp-1001",
		Tool:        "get_weather",
		Description: "Get weather by city.\nUse for read-only weather lookup.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cityName": map[string]any{"type": "string", "description": "City name, for example: Hangzhou."},
				"options":  map[string]any{"type": "object", "description": "Optional lookup options as JSON."},
			},
			"required": []any{"cityName", "options"},
		},
	}})

	cmd := findCommandByPath(root, []string{"weather-service", "get-weather"})
	if cmd == nil {
		t.Fatal("dynamic command was not registered")
	}
	if strings.Contains(cmd.Short, "\n") {
		t.Fatalf("short should be single line, got %q", cmd.Short)
	}
	for _, want := range []string{
		"Dynamic DWS command generated from a published MCP tool.",
		"Use --format json for agent-readable stdout.",
		"Pass inputs either as individual flags or as --params '<JSON object>'.",
		"Command path: dws weather-service get-weather",
		"MCP tool: get_weather",
		"--city-name (string, required): City name",
		"--options (JSON string, required): Optional lookup options as JSON. Pass object/array values as a JSON string.",
	} {
		if !strings.Contains(cmd.Long, want) {
			t.Fatalf("Long missing %q:\n%s", want, cmd.Long)
		}
	}
	if want := `dws weather-service get-weather --city-name Hangzhou --options '{"key":"value"}' --format json`; !strings.Contains(cmd.Example, want) {
		t.Fatalf("Example = %q, want to contain %q", cmd.Example, want)
	}
	if want := `dws weather-service get-weather --params '{"cityName":"Hangzhou","options":{"key":"value"}}' --format json`; !strings.Contains(cmd.Example, want) {
		t.Fatalf("Example = %q, want to contain %q", cmd.Example, want)
	}
	paramsFlag := cmd.Flags().Lookup("params")
	if paramsFlag == nil || !strings.Contains(paramsFlag.Usage, "Full MCP input JSON object") {
		t.Fatalf("params flag usage = %#v", paramsFlag)
	}
	cityFlag := cmd.Flags().Lookup("city-name")
	if cityFlag == nil || !strings.Contains(cityFlag.Usage, "type: string; required") {
		t.Fatalf("city flag usage = %#v", cityFlag)
	}
	optionsFlag := cmd.Flags().Lookup("options")
	if optionsFlag == nil || !strings.Contains(optionsFlag.Usage, "type: JSON string; required; pass as JSON string") {
		t.Fatalf("options flag usage = %#v", optionsFlag)
	}
}

func TestAddMCPHTTPCommandsRegistersRunnableCommand(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	ensureMCPHTTPGroup(root, []string{"connector", "mcp", "published"})

	runner := &mcpHTTPTestRunner{}
	addMCPHTTPCommands(root, runner, nil, []mcpHTTPCommandDescriptor{{
		Path:      []string{"connector", "mcp", "published", "weather"},
		ProductID: "weather-product",
		Tool:      "get_weather",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cityName": map[string]any{"type": "string"},
				"limit":    map[string]any{"type": "integer"},
				"options":  map[string]any{"type": "object"},
			},
			"required": []any{"cityName"},
		},
	}})

	if findCommandByPath(root, []string{"connector", "mcp", "published", "weather"}) == nil {
		t.Fatal("dynamic command was not registered")
	}
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{
		"connector", "mcp", "published", "weather",
		"--city-name", "杭州",
		"--limit", "3",
		"--options", `{"unit":"c"}`,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.invocation.CanonicalProduct != "weather-product" {
		t.Fatalf("product = %q, want weather-product", runner.invocation.CanonicalProduct)
	}
	if runner.invocation.Tool != "get_weather" {
		t.Fatalf("tool = %q, want get_weather", runner.invocation.Tool)
	}
	if runner.invocation.Params["cityName"] != "杭州" {
		t.Fatalf("cityName param = %#v", runner.invocation.Params["cityName"])
	}
	if runner.invocation.Params["limit"] != 3 {
		t.Fatalf("limit param = %#v", runner.invocation.Params["limit"])
	}
	options, ok := runner.invocation.Params["options"].(map[string]any)
	if !ok || options["unit"] != "c" {
		t.Fatalf("options param = %#v", runner.invocation.Params["options"])
	}
}

func TestAddMCPHTTPCommandsAcceptsParamsJSON(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	ensureMCPHTTPGroup(root, []string{"connector", "mcp", "published"})

	runner := &mcpHTTPTestRunner{}
	addMCPHTTPCommands(root, runner, nil, []mcpHTTPCommandDescriptor{{
		Path:      []string{"connector", "mcp", "published", "weather"},
		ProductID: "weather-product",
		Tool:      "get_weather",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cityName": map[string]any{"type": "string"},
				"limit":    map[string]any{"type": "integer"},
				"options":  map[string]any{"type": "object"},
			},
			"required": []any{"cityName", "options"},
		},
	}})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{
		"connector", "mcp", "published", "weather",
		"--params", `{"cityName":"杭州","limit":2,"options":{"unit":"c"}}`,
		"--limit", "3",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if runner.invocation.Params["cityName"] != "杭州" {
		t.Fatalf("cityName param = %#v", runner.invocation.Params["cityName"])
	}
	if runner.invocation.Params["limit"] != 3 {
		t.Fatalf("limit param = %#v", runner.invocation.Params["limit"])
	}
	options, ok := runner.invocation.Params["options"].(map[string]any)
	if !ok || options["unit"] != "c" {
		t.Fatalf("options param = %#v", runner.invocation.Params["options"])
	}
}

func TestAddMCPHTTPCommandsValidatesRequiredParamsAfterMerge(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	ensureMCPHTTPGroup(root, []string{"connector", "mcp", "published"})

	addMCPHTTPCommands(root, &mcpHTTPTestRunner{}, nil, []mcpHTTPCommandDescriptor{{
		Path:      []string{"connector", "mcp", "published", "weather"},
		ProductID: "weather-product",
		Tool:      "get_weather",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cityName": map[string]any{"type": "string"},
			},
			"required": []any{"cityName"},
		},
	}})

	root.SetArgs([]string{"connector", "mcp", "published", "weather"})
	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() succeeded, want validation error")
	}
	if !strings.Contains(err.Error(), `missing required MCP input "cityName"; pass --city-name or include "cityName" in --params`) {
		t.Fatalf("error = %v", err)
	}
}

func TestAddMCPHTTPCommandsSkipsExistingStaticPath(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	parent := ensureMCPHTTPGroup(root, []string{"connector", "mcp", "service"})
	parent.AddCommand(&cobra.Command{
		Use:         "list",
		Annotations: map[string]string{"static": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	})

	addMCPHTTPCommands(root, &mcpHTTPTestRunner{}, nil, []mcpHTTPCommandDescriptor{{
		Path:      []string{"connector", "mcp", "service", "list"},
		ProductID: "remote",
		Tool:      "remote_list",
	}})
	cmd := findCommandByPath(root, []string{"connector", "mcp", "service", "list"})
	if cmd == nil {
		t.Fatal("static command missing")
	}
	if cmd.Annotations["static"] != "true" {
		t.Fatalf("static command was overwritten: %#v", cmd.Annotations)
	}
}
