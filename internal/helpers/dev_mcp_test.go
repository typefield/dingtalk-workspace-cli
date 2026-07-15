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
		{"mcp", "auth", "get"},
		{"mcp", "auth", "save"},
		{"mcp", "credential", "save"},
		{"mcp", "credential", "list"},
		{"mcp", "member", "list"},
		{"mcp", "member", "add"},
		{"mcp", "member", "remove"},
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
			wantTool: devMCPServerURLGetTool,
			wantParams: map[string]any{
				"mcpId":  10487,
				"source": "MARKET",
			},
		},
		{
			name: "service create with server name",
			args: []string{
				"connector", "mcp", "service", "create",
				"--name", "客户信息服务",
				"--description", "查询客户信息",
				"--server-name", "crm-assets",
				"--dry-run",
			},
			wantTool: devMCPServiceCreateTool,
			wantParams: map[string]any{
				"name":        "客户信息服务",
				"description": "查询客户信息",
				"serverName":  "crm-assets",
			},
		},
		{
			name: "tool create parses json",
			args: []string{
				"connector", "mcp", "tool", "create",
				"--mcp-id", "10487",
				"--name", "get_weather",
				"--http-info", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--tool-inputs", `[{"key":"city","type":"string","description":"City name, for example: Hangzhou."}]`,
				"--output-mappings", `[{"target":"$","type":"reference","source":"$.node_service_activator.Body"}]`,
				"--dry-run",
			},
			wantTool: devMCPToolCreateTool,
			wantParams: map[string]any{
				"mcpId":    10487,
				"name":     "get_weather",
				"toolType": "http",
				"httpInfo": map[string]any{
					"method": "GET",
					"url":    "https://example.com",
					"auth": map[string]any{
						"type": "NO_AUTH",
					},
				},
				"toolInputs": []any{
					map[string]any{"key": "city", "type": "string", "description": "City name, for example: Hangzhou."},
				},
				"outputMappings": []any{
					map[string]any{"target": "$", "type": "reference", "source": "$.node_service_activator.Body"},
				},
			},
		},
		{
			name: "tool update sends toolId",
			args: []string{
				"connector", "mcp", "tool", "update",
				"--mcp-id", "10487",
				"--tool-id", "G-ACT-1",
				"--name", "get_weather",
				"--http-info", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--dry-run",
			},
			wantTool: devMCPToolUpdateTool,
			wantParams: map[string]any{
				"mcpId":    10487,
				"toolId":   "G-ACT-1",
				"name":     "get_weather",
				"toolType": "http",
				"httpInfo": map[string]any{
					"method": "GET",
					"url":    "https://example.com",
					"auth": map[string]any{
						"type": "NO_AUTH",
					},
				},
			},
		},
		{
			name: "tool debug with credential",
			args: []string{
				"connector", "mcp", "tool", "debug",
				"--mcp-id", "10487",
				"--tool-id", "G-ACT-1",
				"--value", `{"city":"杭州"}`,
				"--credential-id", "10518",
				"--dry-run",
			},
			wantTool: devMCPToolDebugTool,
			wantParams: map[string]any{
				"mcpId":        10487,
				"toolId":       "G-ACT-1",
				"value":        map[string]any{"city": "杭州"},
				"credentialId": 10518,
			},
		},
		{
			name: "tool debug no credential explicit",
			args: []string{
				"connector", "mcp", "tool", "debug",
				"--mcp-id", "10487",
				"--tool-id", "G-ACT-1",
				"--value", `{"city":"杭州"}`,
				"--no-credential",
				"--dry-run",
			},
			wantTool: devMCPToolDebugTool,
			wantParams: map[string]any{
				"mcpId":  10487,
				"toolId": "G-ACT-1",
				"value":  map[string]any{"city": "杭州"},
			},
		},
		{
			name:     "tool versions",
			args:     []string{"connector", "mcp", "tool", "versions", "--mcp-id", "10487", "--tool-id", "G-ACT-1", "--page-size", "10"},
			wantTool: devMCPToolVersionsTool,
			wantParams: map[string]any{
				"mcpId":    10487,
				"toolId":   "G-ACT-1",
				"pageSize": 10,
			},
		},
		{
			name: "auth config save",
			args: []string{
				"connector", "mcp", "auth", "save",
				"--mcp-id", "10520",
				"--auth-type", "token",
				"--token-auth-config", `{"refreshToken":true,"fetchTokenRequest":{"method":"GET","url":"https://example.com/token"}}`,
				"--dry-run",
			},
			wantTool: devMCPAuthConfigSaveTool,
			wantParams: map[string]any{
				"mcpId":    10520,
				"authType": "TOKEN",
				"tokenAuthConfig": map[string]any{
					"refreshToken": true,
					"fetchTokenRequest": map[string]any{
						"method": "GET",
						"url":    "https://example.com/token",
					},
				},
			},
		},
		{
			name: "credential save",
			args: []string{
				"connector", "mcp", "credential", "save",
				"--mcp-id", "10520",
				"--name", "生产账号",
				"--content", `{"appKey":"key","appSecret":"secret"}`,
				"--yes",
			},
			wantTool: devMCPCredentialSaveTool,
			wantParams: map[string]any{
				"mcpId": 10520,
				"name":  "生产账号",
				"content": map[string]any{
					"appKey":    "key",
					"appSecret": "secret",
				},
			},
		},
		{
			name:     "credential list",
			args:     []string{"connector", "mcp", "credential", "list", "--mcp-id", "10520", "--cursor", "2", "--page-size", "20"},
			wantTool: devMCPCredentialListTool,
			wantParams: map[string]any{
				"mcpId":    10520,
				"cursor":   2,
				"pageSize": 20,
			},
		},
		{
			name:     "member add",
			args:     []string{"connector", "mcp", "member", "add", "--mcp-id", "10520", "--user-ids", "staff001, staff002", "--dry-run"},
			wantTool: devMCPMemberAddTool,
			wantParams: map[string]any{
				"mcpId":         10520,
				"memberUserIds": []string{"staff001", "staff002"},
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

func TestConnectorMCPToolUpsertMetadataValidation(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "rejects non snake case tool name",
			args: []string{
				"connector", "mcp", "tool", "create",
				"--mcp-id", "10487",
				"--name", "GetWeather",
				"--http-info", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--dry-run",
			},
			wantErr: "--name 必须是 snake_case",
		},
		{
			name: "rejects invalid server name",
			args: []string{
				"connector", "mcp", "service", "create",
				"--name", "客户服务",
				"--description", "查询客户",
				"--server-name", "crm_assets",
				"--dry-run",
			},
			wantErr: "--server-name 必须是 kebab-case",
		},
		{
			name: "rejects exposed input without description",
			args: []string{
				"connector", "mcp", "tool", "create",
				"--mcp-id", "10487",
				"--name", "get_weather",
				"--http-info", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--tool-inputs", `[{"key":"city","type":"string"}]`,
				"--dry-run",
			},
			wantErr: "--tool-inputs[0].description 为必填",
		},
		{
			name: "rejects array without items child",
			args: []string{
				"connector", "mcp", "tool", "create",
				"--mcp-id", "10487",
				"--name", "get_weather",
				"--http-info", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--tool-inputs", `[{"key":"cities","type":"array","description":"City names, for example: Hangzhou."}]`,
				"--dry-run",
			},
			wantErr: "children 必须且只能包含一个 key=items",
		},
		{
			name: "rejects empty output mappings",
			args: []string{
				"connector", "mcp", "tool", "create",
				"--mcp-id", "10487",
				"--name", "get_weather",
				"--http-info", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--tool-inputs", `[{"key":"city","type":"string","description":"City name, for example: Hangzhou."}]`,
				"--output-mappings", `[]`,
				"--dry-run",
			},
			wantErr: "--output-mappings 不能为空",
		},
		{
			name: "rejects mapping without target",
			args: []string{
				"connector", "mcp", "tool", "create",
				"--mcp-id", "10487",
				"--name", "get_weather",
				"--http-info", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--input-mappings", `[{"type":"reference","source":"$.node_start.city"}]`,
				"--dry-run",
			},
			wantErr: "--input-mappings[0].target 为必填",
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

			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Execute() error = %v, want %q\noutput:\n%s", err, tc.wantErr, out.String())
			}
		})
	}
}

func TestConnectorMCPLegacyFlagRenameHints(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "tool debug requires credential choice",
			args: []string{
				"connector", "mcp", "tool", "debug",
				"--mcp-id", "10487",
				"--tool-id", "G-ACT-1",
				"--value", `{"city":"杭州"}`,
				"--dry-run",
			},
			wantErr: "请传 --credential-id",
		},
		{
			name:    "tool get rejects action-id",
			args:    []string{"connector", "mcp", "tool", "get", "--mcp-id", "10487", "--action-id", "G-ACT-1"},
			wantErr: "--action-id 已更名为 --tool-id",
		},
		{
			name: "tool update rejects action-id",
			args: []string{
				"connector", "mcp", "tool", "update",
				"--mcp-id", "10487",
				"--action-id", "G-ACT-1",
				"--name", "get_weather",
				"--http-info", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--dry-run",
			},
			wantErr: "--action-id 已更名为 --tool-id",
		},
		{
			name: "tool create rejects legacy http flag",
			args: []string{
				"connector", "mcp", "tool", "create",
				"--mcp-id", "10487",
				"--name", "get_weather",
				"--http", `{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}`,
				"--dry-run",
			},
			wantErr: "--http 已更名为 --http-info",
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

			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Execute() error = %v, want %q\noutput:\n%s", err, tc.wantErr, out.String())
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
		"--http-info", "[]",
		"--dry-run",
	})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--http-info 必须是 JSON 对象") {
		t.Fatalf("Execute() error = %v, want JSON object validation\noutput:\n%s", err, out.String())
	}
}

func TestConnectorMCPCredentialDryRunRedactsContent(t *testing.T) {
	runner := &captureRunner{}
	root := newDevAppTestRoot(runner)
	root.SetArgs([]string{
		"connector", "mcp", "credential", "save",
		"--mcp-id", "10520",
		"--name", "生产账号",
		"--content", `{"appKey":"key","appSecret":"secret"}`,
		"--dry-run",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"redacted": true}
	if got := runner.last.Params["content"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("content = %#v, want %#v", got, want)
	}
}

func TestConnectorMCPCredentialContentFromStdin(t *testing.T) {
	runner := &captureRunner{}
	root := newDevAppTestRoot(runner)
	root.SetIn(strings.NewReader(`{"username":"api-user","password":"secret"}`))
	root.SetArgs([]string{
		"connector", "mcp", "credential", "save",
		"--mcp-id", "10520",
		"--name", "生产账号",
		"--content-file", "-",
		"--yes",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := map[string]any{"username": "api-user", "password": "secret"}
	if got := runner.last.Params["content"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("content = %#v, want %#v", got, want)
	}
}
