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
	texts   map[string]string
}

func (c *todoListCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	c.product, c.tool = product, tool
	text := c.texts[tool]
	if text == "" {
		text = `{"result":{"todoCards":[{"taskId":"todo-1","subject":"Review report"}]}}`
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{
		Type: "text",
		Text: text,
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
	for name, declaration := range map[string]shortcut.Shortcut{
		"get-my-tasks":    GetMyTasks,
		"list-sub":        ListSub,
		"list-attachment": ListAttachment,
		"list-comment":    ListComment,
	} {
		if declaration.OutputRollout != output.RolloutUnifiedActive {
			t.Fatalf("%s rollout = %q, want unified active", name, declaration.OutputRollout)
		}
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

func TestTodoDetailListProjectionSeparatesKnownEmptyFromUnknown(t *testing.T) {
	tests := []struct {
		name      string
		project   func(map[string]any) ([]map[string]any, error)
		known     map[string]any
		malformed map[string]any
		display   map[string]any
	}{
		{
			name:      "sub tasks",
			project:   listSubProject,
			known:     map[string]any{"result": map[string]any{"subTasks": []any{}}},
			malformed: map[string]any{"result": map[string]any{"subTasks": []any{"opaque"}}},
			display:   map[string]any{"result": map[string]any{"subTasks": []any{map[string]any{"subject": "Only title"}}}},
		},
		{
			name:      "attachments",
			project:   listAttachmentProject,
			known:     map[string]any{"data": map[string]any{"attachments": []any{}}},
			malformed: map[string]any{"data": map[string]any{"attachments": []any{"opaque"}}},
			display:   map[string]any{"data": map[string]any{"attachments": []any{map[string]any{"fileName": "only-name.txt"}}}},
		},
		{
			name:      "comments",
			project:   listCommentProject,
			known:     map[string]any{"result": map[string]any{"comments": []any{}}},
			malformed: map[string]any{"result": map[string]any{"comments": []any{"opaque"}}},
			display:   map[string]any{"result": map[string]any{"comments": []any{map[string]any{"content": "only content"}}}},
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
					assertTodoProjectionUnknown(t, err)
				})
			}
		})
	}
}

func TestTodoDetailListsUnifiedOutputHasOneMachineEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		decl    shortcut.Shortcut
		tool    string
		text    string
		itemKey string
		args    []string
		paged   bool
	}{
		{
			name:    "sub tasks",
			decl:    ListSub,
			tool:    "list_sub_tasks",
			text:    `{"result":{"subTasks":[{"taskId":"child-1","subject":"Follow up"}]}}`,
			itemKey: "subTasks",
			args:    []string{"--task-id", "parent-1"},
		},
		{
			name:    "attachments",
			decl:    ListAttachment,
			tool:    "list_todo_attachment",
			text:    `{"result":{"attachments":[{"attachmentId":"attachment-1","fileName":"design.pdf"}]}}`,
			itemKey: "attachments",
			args:    []string{"--task-id", "task-1"},
		},
		{
			name:    "comments",
			decl:    ListComment,
			tool:    "list_todo_comment",
			text:    `{"result":{"comments":[{"commentId":"comment-1","content":"Looks good"}]}}`,
			itemKey: "comments",
			args:    []string{"--task-id", "task-1"},
			paged:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &todoListCaller{texts: map[string]string{tc.tool: tc.text}}
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
			if caller.product != "todo" || caller.tool != tc.tool {
				t.Fatalf("route = %s/%s, want todo/%s", caller.product, caller.tool, tc.tool)
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
			if tc.paged && data["pagination_known"] != false {
				t.Fatalf("paged data = %#v", data)
			}
			if envelope["meta"].(map[string]any)["count"] != float64(1) {
				t.Fatalf("meta = %#v", envelope["meta"])
			}
		})
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
