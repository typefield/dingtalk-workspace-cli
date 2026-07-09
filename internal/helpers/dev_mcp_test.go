package helpers

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestConnectorMCPCommandTree(t *testing.T) {
	connector := connectorHandler{}.Command(&captureRunner{})
	mcp, _, err := connector.Find([]string{"mcp"})
	if err != nil {
		t.Fatalf("Find(mcp) error = %v", err)
	}
	if mcp == nil || mcp.Name() != "mcp" {
		t.Fatalf("mcp command not registered: %#v", mcp)
	}
	for _, path := range [][]string{
		{"mcp", "service", "list"},
		{"mcp", "service", "create"},
		{"mcp", "tool", "create"},
		{"mcp", "tool", "debug"},
		{"mcp", "tool", "publish"},
		{"mcp", "url", "get"},
	} {
		cmd, _, err := connector.Find(path)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", path, err)
		}
		if cmd == nil || cmd.Annotations["mcp-source"] != devMCPProduct || cmd.Annotations["mcp-tool"] == "" {
			t.Fatalf("Find(%v) missing MCP annotations: %#v", path, cmd)
		}
	}
}

func TestConnectorMCPCommandsBuildToolParams(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		wantTool   string
		wantParams map[string]any
	}{
		{
			name:     "service list",
			args:     []string{"connector", "mcp", "service", "list", "--keyword", "脚手架", "--cursor", "2", "--page-size", "20"},
			wantTool: devMCPServiceListTool,
			wantParams: map[string]any{
				"keyword":  "脚手架",
				"cursor":   2,
				"pageSize": 20,
			},
		},
		{
			name:     "url get",
			args:     []string{"connector", "mcp", "url", "get", "--mcp-id", "10487", "--source", "MARKET"},
			wantTool: devMCPGetServerURLTool,
			wantParams: map[string]any{
				"mcpId":  10487,
				"source": "MARKET",
			},
		},
		{
			name: "tool create parses json",
			args: []string{
				"connector", "mcp", "tool", "create",
				"--mcp-id", "10487",
				"--name", "get_weather",
				"--http", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--tool-inputs", `[{"key":"city","type":"string"}]`,
				"--output-mappings", `[{"target":"$","type":"reference","source":"$.node_service_activator.Body"}]`,
				"--dry-run",
			},
			wantTool: devMCPToolCreateTool,
			wantParams: map[string]any{
				"mcpId": 10487,
				"name":  "get_weather",
				"http": map[string]any{
					"method": "GET",
					"url":    "https://example.com",
					"auth": map[string]any{
						"type": "NO_AUTH",
					},
				},
				"toolInputs": []any{
					map[string]any{"key": "city", "type": "string"},
				},
				"outputMappings": []any{
					map[string]any{"target": "$", "type": "reference", "source": "$.node_service_activator.Body"},
				},
			},
		},
		{
			name:     "tool versions",
			args:     []string{"connector", "mcp", "tool", "versions", "--mcp-id", "10487", "--action-id", "G-ACT-1", "--page-size", "10"},
			wantTool: devMCPToolVersionsTool,
			wantParams: map[string]any{
				"mcpId":    10487,
				"actionId": "G-ACT-1",
				"pageSize": 10,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &captureRunner{}
			root := newDevAppTestRoot(runner)
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(tc.args)

			if err := root.Execute(); err != nil {
				t.Fatalf("Execute() error = %v\noutput:\n%s", err, out.String())
			}

			if got := runner.last.CanonicalProduct; got != devMCPProduct {
				t.Fatalf("CanonicalProduct = %q, want %q", got, devMCPProduct)
			}
			if got := runner.last.Tool; got != tc.wantTool {
				t.Fatalf("Tool = %q, want %q", got, tc.wantTool)
			}
			if !reflect.DeepEqual(runner.last.Params, tc.wantParams) {
				t.Fatalf("Params = %#v, want %#v", runner.last.Params, tc.wantParams)
			}
		})
	}
}

func TestConnectorMCPWriteGuard(t *testing.T) {
	runner := &captureRunner{}
	root := newDevAppTestRoot(runner)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"connector", "mcp", "service", "create", "--name", "客户服务", "--description", "查询客户"})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("Execute() error = %v, want --yes guard\noutput:\n%s", err, out.String())
	}
}

func TestConnectorMCPInvalidJSON(t *testing.T) {
	runner := &captureRunner{}
	root := newDevAppTestRoot(runner)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"connector", "mcp", "tool", "create",
		"--mcp-id", "10487",
		"--name", "get_weather",
		"--http", "[]",
		"--dry-run",
	})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--http 必须是 JSON 对象") {
		t.Fatalf("Execute() error = %v, want JSON object validation\noutput:\n%s", err, out.String())
	}
}
