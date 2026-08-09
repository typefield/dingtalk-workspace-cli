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

func TestBaseDiscoveryDualValidationKeepsLegacyWire(t *testing.T) {
	tests := []struct {
		name              string
		decl              shortcut.Shortcut
		args              []string
		tool              string
		text              string
		endpointExhausted bool
		nextToken         string
	}{
		{
			name:              "recent bases preserves resumable page",
			decl:              BaseList,
			tool:              "list_bases",
			text:              `{"data":{"bases":[{"baseId":"base-1","baseName":"Recent"}],"hasMore":true,"nextCursor":"cursor-2"}}`,
			endpointExhausted: false,
			nextToken:         "cursor-2",
		},
		{
			name:              "name search preserves terminal endpoint only",
			decl:              BaseSearch,
			args:              []string{"--query", "项目"},
			tool:              "search_bases",
			text:              `{"result":{"bases":[{"baseId":"base-2","baseName":"项目表"}],"hasMore":false,"nextCursor":"0"}}`,
			endpointExhausted: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.decl.OutputRollout != output.RolloutDualValidate {
				t.Fatalf("rollout = %q, want dual validation", tc.decl.OutputRollout)
			}
			caller := &baseDiscoveryUnifiedCaller{text: tc.text}
			helpers.InitDeps(caller)
			cmd := corecmd.New(shortcut.FromShortcut(tc.decl))
			cmd.PersistentFlags().String("format", "json", "")
			cmd.PersistentFlags().Bool("dry-run", false, "")
			cmd.SetContext(context.Background())
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(append(tc.args, "--format", "json"))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if caller.tool != tc.tool {
				t.Fatalf("tool = %q, want %q", caller.tool, tc.tool)
			}

			var payload map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
				t.Fatalf("decode output: %v\n%s", err, stdout.String())
			}
			if payload["count"] != float64(1) || payload["authoritativeInventory"] != false {
				t.Fatalf("legacy payload = %#v", payload)
			}

			projected, err := baseListProject(tc.tool, mustUnmarshalMap(t, tc.text))
			if err != nil {
				t.Fatalf("project candidate: %v", err)
			}
			candidate, err := baseDiscoveryPayload(tc.tool, mustUnmarshalMap(t, tc.text), projected, map[bool]string{true: "name_search_index", false: "recently_accessed"}[tc.tool == "search_bases"])
			if err != nil {
				t.Fatalf("candidate payload: %v", err)
			}
			meta, err := baseDiscoveryMeta(tc.tool, mustUnmarshalMap(t, tc.text), len(projected))
			if err != nil {
				t.Fatalf("candidate meta: %v", err)
			}
			envelope, err := output.EnvelopeFromResult(output.Success(candidate, output.WithMeta(meta)))
			if err != nil {
				t.Fatalf("shadow unified envelope: %v", err)
			}
			if envelope.OK != true || envelope.Outcome != output.OutcomeSuccess || envelope.Meta == nil || envelope.Meta.Count == nil || *envelope.Meta.Count != 1 {
				t.Fatalf("shadow envelope = %#v", envelope)
			}
			page := envelope.Meta.Pagination
			if page == nil || page.EndpointExhausted != tc.endpointExhausted {
				t.Fatalf("shadow pagination = %#v", page)
			}
			if page.NextToken != tc.nextToken {
				t.Fatalf("shadow next token = %q, want %q", page.NextToken, tc.nextToken)
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
