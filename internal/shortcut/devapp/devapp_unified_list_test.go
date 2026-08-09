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

package devapp

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

// devAppListCaller returns one fully formed, non-final ServiceResult for each
// list leaf.  It proves the shortcut's active renderer preserves the projected
// list and the continuation evidence without a second business call.
type devAppListCaller struct {
	product string
	tool    string
	result  string
}

func (c *devAppListCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.product, c.tool = product, tool
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: c.result}}}, nil
}

func (*devAppListCaller) Format() string { return "json" }
func (*devAppListCaller) DryRun() bool   { return false }
func (*devAppListCaller) Fields() string { return "" }
func (*devAppListCaller) JQ() string     { return "" }

func TestDevAppPaginatedShortcutsEmitUnifiedResumableResults(t *testing.T) {
	tests := []struct {
		name    string
		decl    shortcut.Shortcut
		tool    string
		args    []string
		itemKey string
		item    map[string]any
	}{
		{
			name:    "apps",
			decl:    ListApp,
			tool:    "list_dev_app",
			itemKey: "apps",
			item:    map[string]any{"unifiedAppId": "app-1", "name": "Example app"},
		},
		{
			name:    "permissions",
			decl:    PermissionList,
			tool:    "list_dev_app_permissions",
			args:    []string{"--unified-app-id", "app-1"},
			itemKey: "permissions",
			item:    map[string]any{"scopeValue": "contact:user.base:read", "scopeName": "Read users"},
		},
		{
			name:    "events",
			decl:    EventList,
			tool:    "list_dev_app_events",
			args:    []string{"--unified-app-id", "app-1"},
			itemKey: "events",
			item:    map[string]any{"eventCode": "chat_add_member", "eventName": "Member added"},
		},
		{
			name:    "versions",
			decl:    VersionList,
			tool:    "list_dev_app_versions",
			args:    []string{"--unified-app-id", "app-1"},
			itemKey: "versions",
			item:    map[string]any{"versionId": "version-1", "version": "1.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serviceResult, err := json.Marshal(map[string]any{
				"success": true,
				"result": map[string]any{
					"items":      []any{tt.item},
					"hasMore":    true,
					"nextCursor": "cursor-next",
				},
			})
			if err != nil {
				t.Fatalf("marshal service result: %v", err)
			}
			caller := &devAppListCaller{result: string(serviceResult)}
			helpers.InitDepsForTest(t, caller)

			// The package declarations stay reusable as plain values; init registers
			// a rollout-stamped copy. Stamp the same copy here to exercise the
			// actual active-command execution path rather than the legacy default.
			cmd := corecmd.New(shortcut.FromShortcut(frameworkUnified(tt.decl)))
			cmd.PersistentFlags().String("format", "json", "")
			ctx, _ := output.WithResultStore(context.Background())
			cmd.SetContext(ctx)
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(append([]string{"--format", "json"}, tt.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			exitCode, emitted, err := output.EmitStoredResult(cmd)
			if err != nil || !emitted || exitCode != 0 {
				t.Fatalf("emit: code=%d emitted=%v err=%v", exitCode, emitted, err)
			}
			if caller.product != productDevApp || caller.tool != tt.tool {
				t.Fatalf("route = %s/%s, want %s/%s", caller.product, caller.tool, productDevApp, tt.tool)
			}

			var envelope map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("decode output: %v\n%s", err, stdout.String())
			}
			if envelope["ok"] != true || envelope["outcome"] != "success" {
				t.Fatalf("envelope = %#v", envelope)
			}
			if _, found := envelope["contract_version"]; found {
				t.Fatalf("removed version marker leaked into result: %#v", envelope)
			}
			data, ok := envelope["data"].(map[string]any)
			if !ok || data["count"] != float64(1) {
				t.Fatalf("data = %#v", envelope["data"])
			}
			items, ok := data[tt.itemKey].([]any)
			if !ok || len(items) != 1 {
				t.Fatalf("projected %s = %#v", tt.itemKey, data[tt.itemKey])
			}
			meta, ok := envelope["meta"].(map[string]any)
			if !ok {
				t.Fatalf("meta = %#v", envelope["meta"])
			}
			pagination, ok := meta["pagination"].(map[string]any)
			if !ok || pagination["endpoint_exhausted"] != false || pagination["next_token"] != "cursor-next" {
				t.Fatalf("pagination = %#v", meta["pagination"])
			}
		})
	}
}
