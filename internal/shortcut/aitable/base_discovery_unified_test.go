// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package aitable

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

type baseDiscoveryUnifiedCaller struct {
	tool string
	text string
}

func (c *baseDiscoveryUnifiedCaller) CallTool(_ context.Context, _, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.tool = tool
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: c.text}}}, nil
}

func (*baseDiscoveryUnifiedCaller) Format() string { return "json" }
func (*baseDiscoveryUnifiedCaller) DryRun() bool   { return false }
func (*baseDiscoveryUnifiedCaller) Fields() string { return "" }
func (*baseDiscoveryUnifiedCaller) JQ() string     { return "" }

func TestBaseDiscoveryUsesUnifiedOutput(t *testing.T) {
	tests := []struct {
		name              string
		decl              shortcut.Shortcut
		args              []string
		tool              string
		text              string
		paginationKnown   bool
		paginationPresent bool
		endpointExhausted bool
		nextToken         string
	}{
		{
			name:              "recent bases preserves resumable page",
			decl:              BaseList,
			tool:              "list_bases",
			text:              `{"data":{"bases":[{"baseId":"base-1","baseName":"Recent"}],"hasMore":true,"nextCursor":"cursor-2"}}`,
			paginationKnown:   true,
			paginationPresent: true,
			endpointExhausted: false,
			nextToken:         "cursor-2",
		},
		{
			name:              "name search preserves terminal endpoint only",
			decl:              BaseSearch,
			args:              []string{"--query", "项目"},
			tool:              "search_bases",
			text:              `{"result":{"bases":[{"baseId":"base-2","baseName":"项目表"}],"hasMore":false,"nextCursor":"0"}}`,
			paginationKnown:   true,
			paginationPresent: true,
			endpointExhausted: true,
		},
		{
			name:              "recent bases preserve unknown pagination",
			decl:              BaseList,
			tool:              "list_bases",
			text:              `{"bases":[{"baseId":"base-3","baseName":"Recent without paging evidence"}]}`,
			paginationKnown:   false,
			paginationPresent: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.decl.OutputRollout != output.RolloutUnifiedActive {
				t.Fatalf("rollout = %q, want unified active", tc.decl.OutputRollout)
			}
			caller := &baseDiscoveryUnifiedCaller{text: tc.text}
			helpers.InitDeps(caller)
			cmd := corecmd.New(shortcut.FromShortcut(tc.decl))
			cmd.PersistentFlags().String("format", "json", "")
			cmd.PersistentFlags().Bool("dry-run", false, "")
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
			if caller.tool != tc.tool {
				t.Fatalf("tool = %q, want %q", caller.tool, tc.tool)
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
			data, ok := envelope["data"].(map[string]any)
			if !ok || data["count"] != float64(1) || data["authoritativeInventory"] != false || data["paginationKnown"] != tc.paginationKnown {
				t.Fatalf("data = %#v", envelope["data"])
			}
			for _, key := range []string{"complete", "hasMore", "endpointExhausted", "nextCursor"} {
				if _, leaked := data[key]; leaked {
					t.Fatalf("data leaked duplicate pagination key %q: %#v", key, data)
				}
			}
			meta, ok := envelope["meta"].(map[string]any)
			if !ok || meta["count"] != float64(1) {
				t.Fatalf("meta = %#v", envelope["meta"])
			}
			page, present := meta["pagination"].(map[string]any)
			if present != tc.paginationPresent {
				t.Fatalf("pagination presence = %v, want %v: %#v", present, tc.paginationPresent, meta)
			}
			if present {
				if page["endpoint_exhausted"] != tc.endpointExhausted {
					t.Fatalf("pagination = %#v", page)
				}
				if got, _ := page["next_token"].(string); got != tc.nextToken {
					t.Fatalf("next token = %q, want %q", got, tc.nextToken)
				}
			}
		})
	}
}

func mustUnmarshalMap(t *testing.T, text string) map[string]any {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return data
}

func TestBaseDiscoveryRejectsGenericOrUnusableIdentifiers(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"generic id only": {"bases": []any{map[string]any{"id": "wrapper-id"}}},
		"empty base id":   {"bases": []any{map[string]any{"baseId": "  "}}},
		"numeric base id": {"bases": []any{map[string]any{"baseId": 1}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := baseListProject("list_bases", data)
			var typed *apperrors.Error
			if !errors.As(err, &typed) || typed.StableSubtype != string(apperrors.SubtypeProjectionUnknown) ||
				typed.FailureStage != "response_projection" || !typed.RetryableSet || typed.Retryable {
				t.Fatalf("projection error = %#v", err)
			}
		})
	}
}
