package sheet

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

type sheetListCaller struct {
	product string
	tool    string
}

func (c *sheetListCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.product, c.tool = product, tool
	return &edition.ToolResult{Content: []edition.ContentBlock{{
		Type: "text",
		Text: `{"result":{"sheets":[{"sheetId":"sheet-1","title":"Overview"}]}}`,
	}}}, nil
}

func (*sheetListCaller) Format() string { return "json" }
func (*sheetListCaller) DryRun() bool   { return false }
func (*sheetListCaller) Fields() string { return "" }
func (*sheetListCaller) JQ() string     { return "" }

func TestListSheetsProjectPreservesKnownEmptyList(t *testing.T) {
	got, err := listSheetsProject(map[string]any{"sheets": []any{}})
	if err != nil {
		t.Fatalf("known empty response returned error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("known empty response = %#v, want non-nil empty list", got)
	}
}

func TestListSheetsProjectRejectsUnknownContainer(t *testing.T) {
	_, err := listSheetsProject(map[string]any{"result": map[string]any{"unexpected": []any{}}})
	assertSheetProjectionUnknown(t, err)
}

func TestListSheetsProjectRejectsUnknownRow(t *testing.T) {
	_, err := listSheetsProject(map[string]any{"sheets": []any{"not-a-row"}})
	assertSheetProjectionUnknown(t, err)
}

func TestListSheetsProjectRejectsDisplayOnlyRow(t *testing.T) {
	_, err := listSheetsProject(map[string]any{"sheets": []any{map[string]any{"title": "Overview"}}})
	assertSheetProjectionUnknown(t, err)
}

func TestListSheetsProjectSupportsNestedKnownContainer(t *testing.T) {
	got, err := listSheetsProject(map[string]any{
		"result": map[string]any{
			"data": []any{map[string]any{"sheetId": "s1", "title": "Overview"}},
		},
	})
	if err != nil {
		t.Fatalf("nested known response returned error: %v", err)
	}
	if len(got) != 1 || got[0]["sheetId"] != "s1" || got[0]["title"] != "Overview" {
		t.Fatalf("nested projection = %#v", got)
	}
}

func TestListSheetsDualValidationPreservesLegacyOutput(t *testing.T) {
	if ListSheets.OutputRollout != output.RolloutDualValidate {
		t.Fatalf("list-sheets rollout = %q, want dual validation", ListSheets.OutputRollout)
	}
	caller := &sheetListCaller{}
	helpers.InitDeps(caller)
	cmd := corecmd.New(shortcut.FromShortcut(ListSheets))
	cmd.PersistentFlags().String("format", "json", "")
	cmd.SetContext(context.Background())
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--node", "node-1", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if caller.product != "sheet" || caller.tool != "get_all_sheets" {
		t.Fatalf("route = %s/%s", caller.product, caller.tool)
	}
	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode output: %v\n%s", err, stdout.String())
	}
	if _, changed := payload["ok"]; changed {
		t.Fatalf("dual validation changed the external legacy wire: %#v", payload)
	}
	if _, changed := payload["outcome"]; changed {
		t.Fatalf("dual validation changed the external legacy wire: %#v", payload)
	}
	if payload["count"] != float64(1) || len(payload["sheets"].([]any)) != 1 {
		t.Fatalf("legacy payload = %#v", payload)
	}
}

func assertSheetProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" {
		t.Fatalf("projection error = %T %#v, want typed projection_unknown", err, err)
	}
}
