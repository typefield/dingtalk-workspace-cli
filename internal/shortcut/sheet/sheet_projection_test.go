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

func TestListSheetsUsesUnifiedOutput(t *testing.T) {
	if ListSheets.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("list-sheets rollout = %q, want unified active", ListSheets.OutputRollout)
	}
	caller := &sheetListCaller{}
	helpers.InitDeps(caller)
	cmd := corecmd.New(shortcut.FromShortcut(ListSheets))
	cmd.PersistentFlags().String("format", "json", "")
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--node", "node-1", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	exitCode, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted || exitCode != 0 {
		t.Fatalf("emit: code=%d emitted=%v err=%v", exitCode, emitted, err)
	}
	if caller.product != "sheet" || caller.tool != "get_all_sheets" {
		t.Fatalf("route = %s/%s", caller.product, caller.tool)
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
	if data["count"] != float64(1) || len(data["sheets"].([]any)) != 1 {
		t.Fatalf("data = %#v", data)
	}
	if envelope["meta"].(map[string]any)["count"] != float64(1) {
		t.Fatalf("meta = %#v", envelope["meta"])
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
