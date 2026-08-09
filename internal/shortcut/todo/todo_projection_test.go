package todo

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

type todoListCaller struct {
	product string
	tool    string
}

func (c *todoListCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.product, c.tool = product, tool
	return &edition.ToolResult{Content: []edition.ContentBlock{{
		Type: "text",
		Text: `{"result":{"todoCards":[{"taskId":"todo-1","subject":"Review report"}]}}`,
	}}}, nil
}

func (*todoListCaller) Format() string { return "json" }
func (*todoListCaller) DryRun() bool   { return false }
func (*todoListCaller) Fields() string { return "" }
func (*todoListCaller) JQ() string     { return "" }

func TestGetMyTasksProjectSeparatesKnownEmptyFromUnknown(t *testing.T) {
	cards, err := getMyTasksProject(map[string]any{
		"result": map[string]any{"todoCards": []any{}},
	})
	if err != nil || cards == nil || len(cards) != 0 {
		t.Fatalf("known empty = %#v, %v; want non-nil empty list", cards, err)
	}

	for name, data := range map[string]map[string]any{
		"unknown container": {"result": map[string]any{"items": []any{}}},
		"malformed row":     {"result": map[string]any{"todoCards": []any{"opaque"}}},
		"unknown row":       {"result": map[string]any{"todoCards": []any{map[string]any{"opaque": true}}}},
		"display only row":  {"result": map[string]any{"todoCards": []any{map[string]any{"subject": "Review"}}}},
		"blank task id":     {"result": map[string]any{"todoCards": []any{map[string]any{"taskId": " "}}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := getMyTasksProject(data)
			assertTodoProjectionUnknown(t, err)
		})
	}
}

func TestGetMyTasksUsesUnifiedOutput(t *testing.T) {
	if GetMyTasks.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("get-my-tasks rollout = %q, want unified active", GetMyTasks.OutputRollout)
	}
}

func TestGetMyTasksUnifiedOutputHasOneMachineEnvelope(t *testing.T) {
	caller := &todoListCaller{}
	helpers.InitDeps(caller)
	cmd := corecmd.New(shortcut.FromShortcut(GetMyTasks))
	cmd.PersistentFlags().String("format", "json", "")
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	exitCode, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted || exitCode != 0 {
		t.Fatalf("emit: code=%d emitted=%v err=%v", exitCode, emitted, err)
	}
	if caller.product != "todo" || caller.tool != "get_user_todos_in_current_org" {
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
	if data["count"] != float64(1) || data["pagination_known"] != false || len(data["todos"].([]any)) != 1 {
		t.Fatalf("data = %#v", data)
	}
	if envelope["meta"].(map[string]any)["count"] != float64(1) {
		t.Fatalf("meta = %#v", envelope["meta"])
	}
}

func TestGetMyTasksProjectAcceptsKnownCards(t *testing.T) {
	cards, err := getMyTasksProject(map[string]any{
		"result": map[string]any{"todoCards": []any{map[string]any{
			"taskId": "task-1", "subject": "Review report",
		}}},
	})
	if err != nil {
		t.Fatalf("known card projection returned error: %v", err)
	}
	if len(cards) != 1 || cards[0]["taskId"] != "task-1" {
		t.Fatalf("projected cards = %#v", cards)
	}
}

func assertTodoProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
