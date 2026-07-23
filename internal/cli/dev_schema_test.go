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

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

// buildHelperTestTree mirrors the shape of the real `dws dev` subtree closely
// enough to exercise the live-schema renderer: a group and leaves carrying the
// `mcp-tool` annotation that names the op-app tool to fetch.
func buildHelperTestTree() *cobra.Command {
	root := &cobra.Command{Use: "dws"}

	create := &cobra.Command{
		Use:         "create",
		Short:       "创建应用",
		Annotations: map[string]string{"mcp-tool": "create_dev_app"},
		Run:         func(*cobra.Command, []string) {},
	}

	config := &cobra.Command{
		Use:         "config",
		Short:       "配置机器人",
		Annotations: map[string]string{"mcp-tool": "set_extension_robot_config"},
		Run:         func(*cobra.Command, []string) {},
	}

	// A leaf without an mcp-tool annotation (e.g. dev connect / dev doc search).
	noTool := &cobra.Command{Use: "connect", Short: "无 MCP 工具", Run: func(*cobra.Command, []string) {}}

	robot := &cobra.Command{Use: "robot", Short: "机器人能力"}
	robot.AddCommand(config)

	app := &cobra.Command{Use: "app", Short: "应用"}
	app.AddCommand(create, robot)

	dev := &cobra.Command{Use: "dev", Short: "开放平台开发者命令"}
	dev.AddCommand(app, noTool)

	root.AddCommand(dev)
	return root
}

// fakeFetcher returns a canned op-app tools/list so the renderer is exercised
// without network. It mirrors the MCP shape: properties keyed by camelCase param
// name, required[] listing the camelCase names.
func fakeFetcher(tools map[string]HelperToolSchema) HelperToolFetcher {
	return func(context.Context, string) (map[string]HelperToolSchema, error) {
		return tools, nil
	}
}

func robotConfigToolSchema() HelperToolSchema {
	return HelperToolSchema{
		Name:        "set_extension_robot_config",
		Description: "创建或更新现有应用的机器人配置",
		Properties: map[string]any{
			"unifiedAppId":     map[string]any{"type": "string", "description": "统一应用 ID"},
			"eventCallbackUrl": map[string]any{"type": "string", "description": "事件回调地址"},
			"skills":           map[string]any{"type": "array", "description": "技能列表"},
			"mode":             map[string]any{"type": "string", "description": "机器人模式", "enum": []any{"HTTPS", "STREAM", "AISKILL"}},
		},
		Required: []string{"unifiedAppId"},
	}
}

func buildAnnotatedMCPTestTree() *cobra.Command {
	root := &cobra.Command{Use: "dws"}
	lookup := &cobra.Command{
		Use:   "lookup-english-word",
		Short: "Lookup English Word",
		Annotations: map[string]string{
			"mcp-tool":         "lookup_english_word",
			"mcp-source":       "published-mcp-1001",
			"mcp-description":  "Look up one English word",
			"mcp-input-schema": `{"type":"object","properties":{"word":{"type":"string","description":"English word, for example: hello."}},"required":["word"]}`,
		},
		Run: func(*cobra.Command, []string) {},
	}
	service := &cobra.Command{Use: "english-dictionary", Short: "English Dictionary"}
	service.AddCommand(lookup)
	root.AddCommand(service)
	return root
}

func buildConnectorAnnotatedMCPTestTree() *cobra.Command {
	root := &cobra.Command{Use: "dws"}
	lookup := &cobra.Command{
		Use:   "lookup-english-word",
		Short: "Lookup English Word",
		Annotations: map[string]string{
			"mcp-tool":         "lookup_english_word",
			"mcp-source":       "published-mcp-1001",
			"mcp-description":  "Look up one English word",
			"mcp-input-schema": `{"type":"object","properties":{"word":{"type":"string","description":"English word, for example: hello."}},"required":["word"]}`,
		},
		Run: func(*cobra.Command, []string) {},
	}
	service := &cobra.Command{Use: "english-dictionary", Short: "English Dictionary"}
	service.AddCommand(lookup)
	published := &cobra.Command{Use: "published", Short: "Published MCP"}
	published.AddCommand(service)
	mcp := &cobra.Command{Use: "mcp", Short: "MCP"}
	mcp.AddCommand(published)
	connector := &cobra.Command{Use: "connector", Short: "Connector"}
	connector.AddCommand(mcp)
	root.AddCommand(connector)
	return root
}

func englishWordToolSchema() HelperToolSchema {
	return HelperToolSchema{
		Name:        "lookup_english_word",
		Description: "Look up one English word",
		Properties: map[string]any{
			"word": map[string]any{"type": "string", "description": "English word, for example: hello."},
		},
		Required: []string{"word"},
	}
}

func TestRenderHelperSchema_LeafGwsFlat(t *testing.T) {
	root := buildHelperTestTree()
	fetch := fakeFetcher(map[string]HelperToolSchema{
		"set_extension_robot_config": robotConfigToolSchema(),
	})

	payload, ok, err := renderHelperSchema(context.Background(), root, "dev app robot config", fetch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected helper renderer to claim the path")
	}

	// Flat top-level: description / path / source / parameters; no wrapper.
	if payload["description"] != "创建或更新现有应用的机器人配置" {
		t.Fatalf("description = %v", payload["description"])
	}
	if payload["path"] != "dev app robot config" {
		t.Fatalf("path = %v", payload["path"])
	}
	if payload["source"] != "mcp:op-app" {
		t.Fatalf("source = %v", payload["source"])
	}
	for _, leaked := range []string{"kind", "tool", "product", "helper"} {
		if _, present := payload[leaked]; present {
			t.Fatalf("gws-flat output must not carry %q wrapper key", leaked)
		}
	}

	params, _ := payload["parameters"].(map[string]any)
	if params == nil {
		t.Fatalf("no parameters: %#v", payload)
	}

	// Keys are kebab-case of the MCP param name == the CLI flag.
	uid, _ := params["unified-app-id"].(map[string]any)
	if uid == nil {
		t.Fatalf("missing unified-app-id param: %#v", params)
	}
	if uid["type"] != "string" || uid["required"] != true {
		t.Fatalf("unified-app-id = %#v, want string+required", uid)
	}
	if _, hasDefault := uid["default"]; hasDefault {
		t.Fatal("unified-app-id must not carry a default (MCP provides none)")
	}

	cb, _ := params["event-callback-url"].(map[string]any)
	if cb == nil || cb["required"] != false {
		t.Fatalf("event-callback-url = %#v, want required=false", cb)
	}

	skills, _ := params["skills"].(map[string]any)
	if skills == nil || skills["type"] != "array" {
		t.Fatalf("skills = %#v, want array", skills)
	}

	mode, _ := params["mode"].(map[string]any)
	if mode == nil || mode["type"] != "string" {
		t.Fatalf("mode = %#v, want string", mode)
	}
	if _, hasDefault := mode["default"]; hasDefault {
		t.Fatalf("mode default = %v, want none", mode["default"])
	}
	if mode["required"] != false {
		t.Fatalf("mode required = %v, want false", mode["required"])
	}
}

func TestRenderAnnotatedMCPSchema_FixedDynamicLeaf(t *testing.T) {
	root := buildAnnotatedMCPTestTree()
	calledFetch := false
	fetch := func(_ context.Context, source string) (map[string]HelperToolSchema, error) {
		calledFetch = true
		return nil, errors.New("fetcher must not be called for cached dynamic MCP schema")
	}

	payload, ok, err := renderAnnotatedMCPSchema(context.Background(), root, "english-dictionary.lookup-english-word", fetch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected annotated MCP renderer to claim fixed dynamic path")
	}
	if calledFetch {
		t.Fatal("renderAnnotatedMCPSchema must use cached schema annotations, not live fetcher")
	}
	if payload["path"] != "english-dictionary lookup-english-word" {
		t.Fatalf("path = %v", payload["path"])
	}
	if payload["source"] != "mcp:published-mcp-1001" {
		t.Fatalf("source = %v", payload["source"])
	}
	params, _ := payload["parameters"].(map[string]any)
	word, _ := params["word"].(map[string]any)
	if word == nil || word["type"] != "string" || word["required"] != true {
		t.Fatalf("word param = %#v, want required string", word)
	}
}

func TestRenderHelperSchema_CachedDynamicLeafDoesNotFetch(t *testing.T) {
	root := buildConnectorAnnotatedMCPTestTree()
	calledFetch := false
	fetch := func(_ context.Context, source string) (map[string]HelperToolSchema, error) {
		calledFetch = true
		return nil, errors.New("fetcher must not be called for cached dynamic MCP schema")
	}

	payload, ok, err := renderHelperSchema(context.Background(), root, "connector mcp published english-dictionary lookup-english-word", fetch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected helper renderer to claim connector path")
	}
	if calledFetch {
		t.Fatal("renderHelperSchema must use cached schema annotations for dynamic MCP leaves")
	}
	if payload["source"] != "mcp:published-mcp-1001" {
		t.Fatalf("source = %v", payload["source"])
	}
	params, _ := payload["parameters"].(map[string]any)
	if _, ok := params["word"].(map[string]any); !ok {
		t.Fatalf("missing cached word param: %#v", params)
	}
}

func TestRenderAnnotatedMCPSchema_FixedDynamicGroup(t *testing.T) {
	root := buildAnnotatedMCPTestTree()
	payload, ok, err := renderAnnotatedMCPSchema(context.Background(), root, "english-dictionary", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected annotated MCP renderer to claim group path")
	}
	if payload["path"] != "english-dictionary" {
		t.Fatalf("path = %v", payload["path"])
	}
	if cmds, _ := payload["commands"].([]map[string]any); len(cmds) != 1 {
		t.Fatalf("commands = %#v, want one child", payload["commands"])
	}
}

func TestNewSchemaCommand_AnnotatedMCPPath(t *testing.T) {
	root := buildAnnotatedMCPTestTree()
	root.AddCommand(NewSchemaCommand(nil, func(context.Context, string) (map[string]HelperToolSchema, error) {
		return nil, errors.New("fetcher must not be called for cached dynamic MCP schema")
	}))

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"schema", "english-dictionary.lookup-english-word"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("schema output is not JSON: %v\n%s", err, out.String())
	}
	if payload["path"] != "english-dictionary lookup-english-word" {
		t.Fatalf("path = %v", payload["path"])
	}
	if payload["source"] != "mcp:published-mcp-1001" {
		t.Fatalf("source = %v", payload["source"])
	}
}

func TestRenderHelperSchema_Group(t *testing.T) {
	root := buildHelperTestTree()
	payload, ok, err := renderHelperSchema(context.Background(), root, "dev app", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected claim")
	}
	if payload["path"] != "dev app" {
		t.Fatalf("path = %v", payload["path"])
	}
	cmds, _ := payload["commands"].([]map[string]any)
	if len(cmds) != 2 { // create + robot
		t.Fatalf("commands count = %d, want 2", len(cmds))
	}
}

func TestRenderHelperSchema_NoAnnotation(t *testing.T) {
	root := buildHelperTestTree()
	payload, ok, err := renderHelperSchema(context.Background(), root, "dev connect", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected claim")
	}
	if payload["error"] == nil {
		t.Fatalf("expected a clear no-MCP-tool error, got %#v", payload)
	}
	if _, present := payload["parameters"]; present {
		t.Fatal("no-tool command must not emit parameters")
	}
}

func TestRenderHelperSchema_UnknownSubcommand(t *testing.T) {
	root := buildHelperTestTree()
	payload, ok, err := renderHelperSchema(context.Background(), root, "dev app nope", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected claim")
	}
	if payload["error"] == nil {
		t.Fatalf("expected error for unknown subcommand, got %#v", payload)
	}
	if avail, _ := payload["available"].([]map[string]any); len(avail) == 0 {
		t.Fatal("expected available subcommands listed")
	}
}

func TestRenderHelperSchema_ToolMissingInList(t *testing.T) {
	root := buildHelperTestTree()
	// Fetcher returns an empty list — the annotated tool isn't present.
	payload, ok, err := renderHelperSchema(context.Background(), root, "dev app create", fakeFetcher(map[string]HelperToolSchema{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected claim")
	}
	if payload["error"] == nil {
		t.Fatalf("expected not-found error, got %#v", payload)
	}
}

func TestRenderHelperSchema_FetchError(t *testing.T) {
	root := buildHelperTestTree()
	failing := func(context.Context, string) (map[string]HelperToolSchema, error) {
		return nil, errors.New("network down")
	}
	_, ok, err := renderHelperSchema(context.Background(), root, "dev app create", failing)
	if !ok {
		t.Fatal("expected claim even on fetch error")
	}
	if err == nil {
		t.Fatal("expected the fetch error to surface")
	}
}

func TestRenderHelperSchema_NonHelperPathDeclined(t *testing.T) {
	root := buildHelperTestTree()
	if _, ok, _ := renderHelperSchema(context.Background(), root, "ding.message.send", nil); ok {
		t.Fatal("non-helper path must not be claimed by the helper renderer")
	}
}

func TestKebabCase(t *testing.T) {
	cases := map[string]string{
		"eventCallbackUrl": "event-callback-url",
		"unifiedAppId":     "unified-app-id",
		"disableSSLVerify": "disable-ssl-verify",
		"mode":             "mode",
		"skills":           "skills",
		"i18nName":         "i18n-name",
	}
	for in, want := range cases {
		if got := kebabCase(in); got != want {
			t.Errorf("kebabCase(%q) = %q, want %q", in, got, want)
		}
	}
}
