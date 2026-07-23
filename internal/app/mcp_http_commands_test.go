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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
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

func TestMCPHTTPInspectCommandReturnsProtocolAndTools(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode JSON-RPC request: %v", err)
		}
		method, _ := req["method"].(string)
		methods = append(methods, method)
		switch method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]any{
					"protocolVersion": "2025-03-26",
					"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
					"serverInfo":      map[string]any{"name": "test-mcp", "version": "1.0.0"},
				},
			})
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]any{
					"tools": []map[string]any{{
						"name":        "search_city",
						"description": "Search a city",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"cityName": map[string]any{"type": "string"},
							},
						},
					}},
				},
			})
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	cmd := newMCPHTTPInspectCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	t.Setenv(mcpdevEndpointEnv, server.URL+"?key=secret")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect command error = %v", err)
	}

	if want := []string{"initialize", "notifications/initialized", "tools/list"}; !reflect.DeepEqual(methods, want) {
		t.Fatalf("methods = %#v, want %#v", methods, want)
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if got["endpoint"] != server.URL+"?key=REDACTED" {
		t.Fatalf("endpoint = %#v, want redacted URL", got["endpoint"])
	}
	if got["toolCount"] != float64(1) {
		t.Fatalf("toolCount = %#v, want 1", got["toolCount"])
	}
	initialize, _ := got["initialize"].(map[string]any)
	if initialize["protocol_version"] != "2025-03-26" {
		t.Fatalf("protocol_version = %#v", initialize["protocol_version"])
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
						"serverName":  "weather-cli",
						"name":        "Weather MCP",
						"description": "Weather service",
						"icon":        "",
						"mcpUrl":      server.URL + "/server/hash-1001",
					}},
					"currentPage": 1,
					"pageSize":    100,
					"totalCount":  1,
					"totalPages":  1,
				},
			})
		case "/server/hash-1001":
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
	fixedPath := []string{"weather-cli", "get-weather"}
	debugPath := []string{"connector", "mcp", "published", "weather-cli", "get-weather"}
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
	commands := mcpHTTPCommandsFromPublishedTool(publishedMCPService{}, "https://mcp-gw.example/server/hash-1001", transport.ToolDescriptor{
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

func TestMCPHTTPCommandsFromPublishedToolFallsBackToMCPIDBeforeDisplayName(t *testing.T) {
	commands := mcpHTTPCommandsFromPublishedTool(publishedMCPService{
		MCPID: json.Number("10513"),
		Name:  "天气查询测试服务",
	}, "https://mcp-gw.example/server/weather", transport.ToolDescriptor{
		Name: "search_city",
	})

	if findMCPHTTPTestCommandByPath(commands, []string{"mcp-10513", "search-city"}) == nil {
		t.Fatalf("mcpId fallback command missing: %#v", commands)
	}
	for _, command := range commands {
		if strings.Contains(strings.Join(command.Path, " "), "天气查询测试服务") {
			t.Fatalf("display name leaked into command path: %#v", command.Path)
		}
	}
}

func TestMCPHTTPCommandsFromPublishedToolRejectsInvalidServerName(t *testing.T) {
	commands := mcpHTTPCommandsFromPublishedTool(publishedMCPService{
		MCPID:      json.Number("10513"),
		ServerName: "天气服务",
		Name:       "Weather Service",
	}, "https://mcp-gw.example/server/weather", transport.ToolDescriptor{
		Name: "search_city",
	})

	if findMCPHTTPTestCommandByPath(commands, []string{"mcp-10513", "search-city"}) == nil {
		t.Fatalf("invalid serverName should fall back to mcpId: %#v", commands)
	}
}

func TestMCPHTTPCommandCacheMigratesChinesePublishedServicePath(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())
	endpoint := "https://mcp-gw.example/server/weather"
	path, err := mcpHTTPCommandCachePath(endpoint)
	if err != nil {
		t.Fatalf("mcpHTTPCommandCachePath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(cache dir) error = %v", err)
	}
	cache := mcpHTTPCommandCacheFile{
		FetchedAt: time.Now(),
		Commands: []mcpHTTPCommandDescriptor{
			{
				Path:      []string{"天气查询测试服务", "search-city"},
				ProductID: "published-mcp-10513",
				Tool:      "search_city",
			},
			{
				Path:      []string{"connector", "mcp", "published", "天气查询测试服务", "search-city"},
				ProductID: "published-mcp-10513",
				Tool:      "search_city",
			},
		},
	}
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(cache) error = %v", err)
	}

	commands, fresh := readMCPHTTPCommandCache(endpoint)
	if !fresh {
		t.Fatal("migrated cache should remain fresh")
	}
	if findMCPHTTPTestCommandByPath(commands, []string{"mcp-10513", "search-city"}) == nil {
		t.Fatalf("fixed migrated path missing: %#v", commands)
	}
	if findMCPHTTPTestCommandByPath(commands, []string{"connector", "mcp", "published", "mcp-10513", "search-city"}) == nil {
		t.Fatalf("debug migrated path missing: %#v", commands)
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

func TestMCPHTTPCommandsFromPublishedMCPsReportIsolatesTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/slow":
			time.Sleep(200 * time.Millisecond)
		case "/fast":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode JSON-RPC request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result": map[string]any{
					"tools": []map[string]any{{"name": "fast_tool"}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	startedAt := time.Now()
	report := mcpHTTPCommandsFromPublishedMCPsReport(context.Background(), []publishedMCPService{
		{MCPID: json.Number("1001"), ServerName: "slow-one", MCPURL: server.URL + "/slow?key=super-secret"},
		{MCPID: json.Number("1002"), ServerName: "slow-two", MCPURL: server.URL + "/slow?key=super-secret"},
		{MCPID: json.Number("1003"), ServerName: "slow-three", MCPURL: server.URL + "/slow?key=super-secret"},
		{MCPID: json.Number("1004"), ServerName: "slow-four", MCPURL: server.URL + "/slow?key=super-secret"},
		{MCPID: json.Number("1005"), ServerName: "fast-service", MCPURL: server.URL + "/fast"},
	}, 50*time.Millisecond)
	elapsed := time.Since(startedAt)

	if !report.Partial || report.ServiceCount != 5 || report.SuccessfulServiceCount != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if len(report.FailedServices) != 4 || report.FailedServices[0].MCPID != "1001" {
		t.Fatalf("failed services = %#v", report.FailedServices)
	}
	if elapsed >= 150*time.Millisecond {
		t.Fatalf("service probes appear sequential: elapsed=%s", elapsed)
	}
	for _, failure := range report.FailedServices {
		if strings.Contains(failure.Endpoint, "super-secret") || strings.Contains(failure.Error, "super-secret") {
			t.Fatalf("refresh failure leaked endpoint credential: %#v", failure)
		}
	}
	if report.ToolCount != 1 || len(report.Commands) != 2 {
		t.Fatalf("toolCount=%d commands=%d, want 1/2", report.ToolCount, len(report.Commands))
	}
	if findMCPHTTPTestCommandByPath(report.Commands, []string{"fast-service", "fast-tool"}) == nil {
		t.Fatalf("healthy service command missing: %#v", report.Commands)
	}
}

func TestMCPHTTPCommandsFromPublishedMCPsReportBoundsConcurrency(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode JSON-RPC request: %v", err)
			return
		}
		current := active.Add(1)
		for {
			observed := maximum.Load()
			if current <= observed || maximum.CompareAndSwap(observed, current) {
				break
			}
		}
		<-release
		active.Add(-1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  map[string]any{"tools": []map[string]any{{"name": "bounded_tool"}}},
		})
	}))
	defer server.Close()

	services := make([]publishedMCPService, 6)
	for index := range services {
		services[index] = publishedMCPService{
			MCPID:      json.Number(strconv.Itoa(index + 1)),
			ServerName: fmt.Sprintf("service-%d", index+1),
			MCPURL:     server.URL,
		}
	}
	reportCh := make(chan mcpHTTPRefreshReport, 1)
	go func() {
		reportCh <- mcpHTTPCommandsFromPublishedMCPsReport(context.Background(), services, 2*time.Second)
	}()

	deadline := time.Now().Add(time.Second)
	for maximum.Load() < mcpHTTPCommandRefreshWorkers && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := maximum.Load(); got != mcpHTTPCommandRefreshWorkers {
		close(release)
		t.Fatalf("maximum concurrent probes = %d, want %d", got, mcpHTTPCommandRefreshWorkers)
	}
	close(release)
	report := <-reportCh
	if got := maximum.Load(); got > mcpHTTPCommandRefreshWorkers {
		t.Fatalf("maximum concurrent probes = %d, limit %d", got, mcpHTTPCommandRefreshWorkers)
	}
	if report.SuccessfulServiceCount != len(services) || len(report.FailedServices) != 0 {
		t.Fatalf("unexpected bounded-concurrency report: %#v", report)
	}
}

func TestNewMCPHTTPRefreshClientUsesRequestedTimeout(t *testing.T) {
	client := newMCPHTTPRefreshClient(45 * time.Second)
	if client.HTTPClient.Timeout != 45*time.Second {
		t.Fatalf("http client timeout = %s, want 45s", client.HTTPClient.Timeout)
	}
	transportConfig, ok := client.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("HTTP transport = %T, want *http.Transport", client.HTTPClient.Transport)
	}
	if transportConfig.ResponseHeaderTimeout != 45*time.Second {
		t.Fatalf("response header timeout = %s, want 45s", transportConfig.ResponseHeaderTimeout)
	}
}

func TestMergeMCPHTTPRefreshCommandsPreservesFailedProductCache(t *testing.T) {
	existing := []mcpHTTPCommandDescriptor{
		{Path: []string{"service-one", "old-one"}, ProductID: "published-mcp-1001", Tool: "old_one"},
		{Path: []string{"service-two", "old-two"}, ProductID: "published-mcp-1002", Tool: "old_two"},
		{Path: []string{"removed-service", "old-three"}, ProductID: "published-mcp-1003", Tool: "old_three"},
	}
	report := mcpHTTPRefreshReport{
		Commands: []mcpHTTPCommandDescriptor{
			{Path: []string{"service-two", "new-two"}, ProductID: "published-mcp-1002", Tool: "new_two"},
		},
		discoveredProducts: map[string]bool{
			"published-mcp-1001": true,
			"published-mcp-1002": true,
		},
		successfulProducts: map[string]bool{
			"published-mcp-1002": true,
		},
	}

	got := mergeMCPHTTPRefreshCommands(existing, report)
	if findMCPHTTPTestCommand(got, "old_one") == nil {
		t.Fatalf("failed product cache should be preserved: %#v", got)
	}
	if findMCPHTTPTestCommand(got, "new_two") == nil || findMCPHTTPTestCommand(got, "old_two") != nil {
		t.Fatalf("successful product cache should be replaced: %#v", got)
	}
	if findMCPHTTPTestCommand(got, "old_three") != nil {
		t.Fatalf("undiscovered product cache should be removed: %#v", got)
	}
}

func TestRefreshMCPHTTPCommandCachePartialSuccessPreservesFailedProduct(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())
	ResetRuntimeTokenCache()
	t.Cleanup(ResetRuntimeTokenCache)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cli/discovery/mcp":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": map[string]any{
					"values": []map[string]any{
						{"mcpId": 1001, "serverName": "slow-service", "mcpUrl": server.URL + "/slow"},
						{"mcpId": 1002, "serverName": "fast-service", "mcpUrl": server.URL + "/fast"},
					},
					"totalPages": 1,
				},
			})
		case "/slow":
			time.Sleep(100 * time.Millisecond)
		case "/fast":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode JSON-RPC request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req["id"],
				"result":  map[string]any{"tools": []map[string]any{{"name": "new_fast"}}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv(mcpHTTPCommandDiscoveryEnv, server.URL)

	source, err := currentMCPHTTPCommandCacheSource()
	if err != nil {
		t.Fatalf("currentMCPHTTPCommandCacheSource() error = %v", err)
	}
	existing := []mcpHTTPCommandDescriptor{
		{Path: []string{"slow-service", "old-slow"}, ProductID: "published-mcp-1001", Tool: "old_slow"},
		{Path: []string{"fast-service", "old-fast"}, ProductID: "published-mcp-1002", Tool: "old_fast"},
		{Path: []string{"removed-service", "old-removed"}, ProductID: "published-mcp-1003", Tool: "old_removed"},
	}
	if err := writeMCPHTTPCommandCache(source, existing); err != nil {
		t.Fatalf("writeMCPHTTPCommandCache() error = %v", err)
	}

	report, err := refreshMCPHTTPCommandCache(context.Background(), 30*time.Millisecond)
	if err != nil {
		t.Fatalf("refreshMCPHTTPCommandCache() error = %v", err)
	}
	if !report.Partial || !report.CacheUpdated || len(report.FailedServices) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if findMCPHTTPTestCommand(report.Commands, "old_slow") == nil {
		t.Fatalf("failed service stale command missing: %#v", report.Commands)
	}
	if findMCPHTTPTestCommand(report.Commands, "new_fast") == nil || findMCPHTTPTestCommand(report.Commands, "old_fast") != nil {
		t.Fatalf("healthy service cache not replaced: %#v", report.Commands)
	}
	if findMCPHTTPTestCommand(report.Commands, "old_removed") != nil {
		t.Fatalf("removed service cache should be dropped: %#v", report.Commands)
	}
	cached, fresh := readMCPHTTPCommandCache(source)
	if !fresh || !reflect.DeepEqual(cached, report.Commands) {
		t.Fatalf("cache mismatch: fresh=%v cached=%#v report=%#v", fresh, cached, report.Commands)
	}
}

func TestMCPHTTPRefreshCommandReportsTotalFailureAndKeepsCache(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())
	ResetRuntimeTokenCache()
	t.Cleanup(ResetRuntimeTokenCache)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cli/discovery/mcp" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"result": map[string]any{
				"values": []map[string]any{{
					"mcpId":      1001,
					"serverName": "offline-service",
					"mcpUrl":     "",
				}},
				"totalPages": 1,
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
		Path:      []string{"offline-service", "cached-tool"},
		ProductID: "published-mcp-1001",
		Tool:      "cached_tool",
	}}
	if err := writeMCPHTTPCommandCache(source, existing); err != nil {
		t.Fatalf("writeMCPHTTPCommandCache() error = %v", err)
	}

	root := &cobra.Command{Use: "dws", SilenceErrors: true, SilenceUsage: true}
	flags := &GlobalFlags{}
	bindPersistentFlags(root, flags)
	ensureMCPHTTPRefreshCommand(root, flags)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"connector", "mcp", "refresh", "--format", "json"})
	_, err = root.ExecuteC()
	if err == nil {
		t.Fatal("refresh error = nil, want total failure")
	}
	if got := apperrors.ExitCode(err); got != 6 {
		t.Fatalf("refresh exit code = %d, want discovery code 6: %v", got, err)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode refresh failure payload: %v; output=%q", err, stdout.String())
	}
	if payload["success"] != false || payload["partial"] != true || payload["cacheUpdated"] != false {
		t.Fatalf("unexpected refresh failure payload: %#v", payload)
	}
	if payload["failedServiceCount"] != float64(1) || payload["commandCount"] != float64(1) {
		t.Fatalf("unexpected refresh failure counts: %#v", payload)
	}
	cached, fresh := readMCPHTTPCommandCache(source)
	if !fresh || findMCPHTTPTestCommand(cached, "cached_tool") == nil {
		t.Fatalf("total failure should preserve cache: fresh=%v cached=%#v", fresh, cached)
	}
}

func TestMCPHTTPRefreshCommandErrorPreservesStructuredCategory(t *testing.T) {
	internalErr := apperrors.NewInternal("cache write failed")
	if got := apperrors.ExitCode(mcpHTTPRefreshCommandError(internalErr)); got != 5 {
		t.Fatalf("structured internal error exit code = %d, want 5", got)
	}
	if got := apperrors.ExitCode(mcpHTTPRefreshCommandError(fmt.Errorf("all probes failed"))); got != 6 {
		t.Fatalf("plain refresh error exit code = %d, want discovery code 6", got)
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

func TestShouldRefreshMCPHTTPCommandsSkipsExplicitRefreshStartupFetch(t *testing.T) {
	if shouldRefreshMCPHTTPCommands([]string{"connector", "mcp", "refresh"}) {
		t.Fatal("explicit refresh should not trigger a duplicate startup refresh")
	}
	if !shouldRefreshMCPHTTPCommands([]string{"connector", "mcp", "service", "list"}) {
		t.Fatal("other connector commands should refresh stale dynamic commands")
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
	if cmd.Annotations["mcp-source"] != "published-mcp-1001" {
		t.Fatalf("mcp-source annotation = %q", cmd.Annotations["mcp-source"])
	}
	if cmd.Annotations["mcp-description"] != "Get weather by city.\nUse for read-only weather lookup." {
		t.Fatalf("mcp-description annotation = %q", cmd.Annotations["mcp-description"])
	}
	var cachedSchema map[string]any
	if err := json.Unmarshal([]byte(cmd.Annotations["mcp-input-schema"]), &cachedSchema); err != nil {
		t.Fatalf("mcp-input-schema annotation is not JSON: %v", err)
	}
	if props, _ := cachedSchema["properties"].(map[string]any); props["cityName"] == nil {
		t.Fatalf("cached schema properties = %#v", props)
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
	if runner.invocation.Params["limit"] != int64(3) {
		t.Fatalf("limit param = %#v", runner.invocation.Params["limit"])
	}
	options, ok := runner.invocation.Params["options"].(map[string]any)
	if !ok || options["unit"] != "c" {
		t.Fatalf("options param = %#v", runner.invocation.Params["options"])
	}
}

func TestMCPHTTPDynamicLeafHelpIsReachable(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())
	t.Setenv(mcpHTTPCommandDiscoveryEnv, "https://mcp-discovery.example")
	source, err := currentMCPHTTPCommandCacheSource()
	if err != nil {
		t.Fatalf("currentMCPHTTPCommandCacheSource() error = %v", err)
	}
	if err := writeMCPHTTPCommandCache(source, []mcpHTTPCommandDescriptor{{
		Path:        []string{"connector", "mcp", "published", "weather-service", "get-forecast"},
		ProductID:   "weather-product",
		Tool:        "get_forecast",
		Description: "Get the weather forecast for one department.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deptId": map[string]any{"type": "integer", "description": "Department ID"},
			},
		},
	}}); err != nil {
		t.Fatalf("writeMCPHTTPCommandCache() error = %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "long help flag",
			args: []string{"connector", "mcp", "published", "weather-service", "get-forecast", "--help"},
		},
		{
			name: "short help flag",
			args: []string{"connector", "mcp", "published", "weather-service", "get-forecast", "-h"},
		},
		{
			name: "help command",
			args: []string{"help", "connector", "mcp", "published", "weather-service", "get-forecast"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := NewRootCommandWithEngine(context.Background(), nil)

			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(tt.args)
			if _, err := root.ExecuteC(); err != nil {
				t.Fatalf("ExecuteC() error = %v", err)
			}
			body := out.String()
			for _, want := range []string{
				"Get the weather forecast for one department.",
				"dws connector mcp published weather-service get-forecast",
				"--dept-id",
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("leaf help missing %q:\n%s", want, body)
				}
			}
		})
	}
}

func TestMCPHTTPIntegerFlagUsesInt64(t *testing.T) {
	descriptor := mcpHTTPCommandDescriptor{
		Path:      []string{"connector", "mcp", "published", "weather-service", "get-forecast"},
		ProductID: "weather-product",
		Tool:      "get_forecast",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deptId": map[string]any{"type": "integer", "description": "Department ID"},
			},
		},
	}
	runner := &mcpHTTPTestRunner{}
	cmd := newMCPHTTPDynamicCommand(runner, nil, descriptor)
	var out bytes.Buffer
	cmd.SetOut(&out)
	deptID := cmd.Flags().Lookup("dept-id")
	if deptID == nil {
		t.Fatal("dept-id flag was not registered")
	}
	if got := deptID.Value.Type(); got != "int64" {
		t.Fatalf("dept-id flag type = %q, want int64", got)
	}

	cmd.SetArgs([]string{"--dept-id", "9223372036854775807"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := runner.invocation.Params["deptId"]; got != int64(9223372036854775807) {
		t.Fatalf("deptId param = %#v, want int64 max", got)
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
	if runner.invocation.Params["limit"] != int64(3) {
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
