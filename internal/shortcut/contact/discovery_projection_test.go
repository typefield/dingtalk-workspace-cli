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

package contact

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type contactDiscoveryCaller struct {
	product string
	tool    string
	texts   map[string]string
}

func (c *contactDiscoveryCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.product, c.tool = product, tool
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: c.texts[tool]}}}, nil
}

func (*contactDiscoveryCaller) Format() string { return "json" }
func (*contactDiscoveryCaller) DryRun() bool   { return false }
func (*contactDiscoveryCaller) Fields() string { return "" }
func (*contactDiscoveryCaller) JQ() string     { return "" }

func TestContactDiscoveryProjectionsSeparateKnownEmptyFromUnknown(t *testing.T) {
	tests := []struct {
		name      string
		project   func(map[string]any) ([]map[string]any, error)
		known     map[string]any
		malformed map[string]any
		display   map[string]any
	}{
		{
			name:      "followings",
			project:   listFollowingsProject,
			known:     map[string]any{"result": map[string]any{"models": []any{}}},
			malformed: map[string]any{"result": map[string]any{"models": []any{"opaque"}}},
			display:   map[string]any{"result": map[string]any{"models": []any{map[string]any{"name": "Only name"}}}},
		},
		{
			name:      "users",
			project:   searchUserProject,
			known:     map[string]any{"result": []any{}},
			malformed: map[string]any{"result": []any{"opaque"}},
			display:   map[string]any{"result": []any{map[string]any{"name": "Only name"}}},
		},
		{
			name:      "sub departments",
			project:   listSubDeptsProject,
			known:     map[string]any{"result": map[string]any{"depts": []any{}}},
			malformed: map[string]any{"result": map[string]any{"depts": []any{"opaque"}}},
			display:   map[string]any{"result": map[string]any{"depts": []any{map[string]any{"deptName": "Only name"}}}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items, err := tc.project(tc.known)
			if err != nil || items == nil || len(items) != 0 {
				t.Fatalf("known empty = %#v, %v; want non-nil empty list", items, err)
			}
			for name, data := range map[string]map[string]any{
				"unknown container": {"result": map[string]any{"unexpected": []any{}}},
				"malformed row":     tc.malformed,
				"display only row":  tc.display,
			} {
				t.Run(name, func(t *testing.T) {
					_, err := tc.project(data)
					assertContactProjectionUnknown(t, err)
				})
			}
		})
	}
}

func TestContactDiscoveryUsesUnifiedOutput(t *testing.T) {
	for name, declaration := range map[string]shortcut.Shortcut{
		"list-followings": ListFollowings,
		"search-user":     SearchUser,
		"search-mobile":   SearchMobile,
		"list-sub-depts":  ListSubDepts,
	} {
		if declaration.OutputRollout != output.RolloutUnifiedActive {
			t.Fatalf("%s rollout = %q, want unified active", name, declaration.OutputRollout)
		}
	}
}

func TestContactDiscoveryUnifiedOutputHasOneMachineEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		decl    shortcut.Shortcut
		tool    string
		text    string
		itemKey string
		args    []string
	}{
		{
			name:    "followings preserve open id",
			decl:    ListFollowings,
			tool:    "list_my_followings",
			text:    `{"result":{"models":[{"openDingTalkId":"open-user-1","name":"Alice"}]}}`,
			itemKey: "followings",
		},
		{
			name:    "search user",
			decl:    SearchUser,
			tool:    "search_contact_by_key_word",
			text:    `{"result":[{"userId":"user-1","name":"Alice"}]}`,
			itemKey: "users",
			args:    []string{"--query", "Alice"},
		},
		{
			name:    "search mobile",
			decl:    SearchMobile,
			tool:    "search_user_by_mobile",
			text:    `{"result":[{"userId":"user-1","name":"Alice"}]}`,
			itemKey: "users",
			args:    []string{"--mobile", "13800138000"},
		},
		{
			name:    "sub departments preserve numeric id",
			decl:    ListSubDepts,
			tool:    "get_sub_depts_by_dept_id",
			text:    `{"result":{"depts":[{"deptId":2,"deptName":"Engineering"}]}}`,
			itemKey: "depts",
			args:    []string{"--dept", "1"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &contactDiscoveryCaller{texts: map[string]string{tc.tool: tc.text}}
			helpers.InitDeps(caller)
			cmd := corecmd.New(shortcut.FromShortcut(tc.decl))
			cmd.PersistentFlags().String("format", "json", "")
			ctx, _ := output.WithResultStore(context.Background())
			cmd.SetContext(ctx)
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(append(tc.args, "--format", "json"))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			exitCode, emitted, err := output.EmitStoredResult(cmd)
			if err != nil || !emitted || exitCode != 0 {
				t.Fatalf("emit: code=%d emitted=%v err=%v", exitCode, emitted, err)
			}
			if caller.product != "contact" || caller.tool != tc.tool {
				t.Fatalf("route = %s/%s, want contact/%s", caller.product, caller.tool, tc.tool)
			}
			var envelope map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("decode output: %v\n%s", err, stdout.String())
			}
			if envelope["ok"] != true || envelope["outcome"] != "success" {
				t.Fatalf("envelope = %#v", envelope)
			}
			if _, leaked := envelope["contract_version"]; leaked {
				t.Fatalf("result leaked removed version marker: %#v", envelope)
			}
			data := envelope["data"].(map[string]any)
			if data["count"] != float64(1) || len(data[tc.itemKey].([]any)) != 1 {
				t.Fatalf("data = %#v", data)
			}
			if envelope["meta"].(map[string]any)["count"] != float64(1) {
				t.Fatalf("meta = %#v", envelope["meta"])
			}
		})
	}
}

func assertContactProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
