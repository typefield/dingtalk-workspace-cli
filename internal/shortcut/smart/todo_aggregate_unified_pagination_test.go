// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package smart

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

var errTodoAggregateSecondPage = errors.New("second todo page unavailable")

type todoAggregatePagingCaller struct {
	pages  []string
	failAt int
	calls  int
}

func (c *todoAggregatePagingCaller) CallTool(_ context.Context, product, tool string, _ map[string]any) (*edition.ToolResult, error) {
	if product != "todo" || tool != "get_user_todos_in_current_org" {
		return nil, fmt.Errorf("unexpected call %s/%s", product, tool)
	}
	c.calls++
	if c.failAt == c.calls {
		return nil, errTodoAggregateSecondPage
	}
	index := c.calls - 1
	if index >= len(c.pages) {
		return nil, fmt.Errorf("unexpected todo page %d", c.calls)
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: c.pages[index]}}}, nil
}

func (c *todoAggregatePagingCaller) Format() string { return "json" }
func (c *todoAggregatePagingCaller) DryRun() bool   { return false }
func (c *todoAggregatePagingCaller) Fields() string { return "" }
func (c *todoAggregatePagingCaller) JQ() string     { return "" }

func TestTodoAggregateRolloutStartsDualValidate(t *testing.T) {
	for name, declaration := range map[string]shortcut.Shortcut{
		"related-tasks": RelatedTasks,
		"due-today":     DueToday,
	} {
		if declaration.OutputRollout != output.RolloutDualValidate {
			t.Fatalf("%s rollout=%q, want dual_validate", name, declaration.OutputRollout)
		}
	}
}

func TestRelatedTasksDualValidatePreservesLegacyBytes(t *testing.T) {
	page := todoAggregatePage(t, 1, 2)
	legacy, legacyErr := runTodoAggregateLegacy(t, RelatedTasks, output.RolloutLegacyOnly, &todoAggregatePagingCaller{pages: []string{page}})
	if legacyErr != nil {
		t.Fatalf("legacy execution: %v", legacyErr)
	}
	dual, dualErr := runTodoAggregateLegacy(t, RelatedTasks, output.RolloutDualValidate, &todoAggregatePagingCaller{pages: []string{page}})
	if dualErr != nil {
		t.Fatalf("dual execution: %v", dualErr)
	}
	if !bytes.Equal(legacy, dual) {
		t.Fatalf("dual validation changed legacy stdout\nlegacy=%s\ndual=%s", legacy, dual)
	}
}

func TestRelatedTasksUnifiedResultPreservesEarlierPagesOnLaterFailure(t *testing.T) {
	envelope, exitCode, calls := runTodoAggregateUnified(t, RelatedTasks, &todoAggregatePagingCaller{
		pages:  []string{todoAggregatePage(t, 1, todoPageSize)},
		failAt: 2,
	})
	if calls != 2 {
		t.Fatalf("calls=%d, want exactly 2: the failed page must not be replayed", calls)
	}
	if exitCode != 7 || envelope["ok"] != false || envelope["outcome"] != "partial_failure" {
		t.Fatalf("partial envelope=%#v exit=%d", envelope, exitCode)
	}
	data := envelope["data"].(map[string]any)
	if data["total"] != float64(2) {
		t.Fatalf("partial data=%#v", data)
	}
	succeeded := data["succeeded"].([]any)
	if len(succeeded) != 1 {
		t.Fatalf("succeeded=%#v", succeeded)
	}
	entry := succeeded[0].(map[string]any)
	if entry["id"] != "todo:related_tasks" || entry["pages_fetched"] != float64(1) || entry["items_fetched"] != float64(todoPageSize) {
		t.Fatalf("succeeded entry=%#v", entry)
	}
	projected := entry["data"].(map[string]any)
	if projected["pagination_known"] != false {
		t.Fatalf("todo aggregation must not invent endpoint exhaustion: %#v", projected)
	}
	tasks := projected["tasks"].([]any)
	if len(tasks) != todoPageSize {
		t.Fatalf("preserved task count=%d, want %d", len(tasks), todoPageSize)
	}
	failed := data["failed"].([]any)
	if len(failed) != 1 {
		t.Fatalf("failed=%#v", failed)
	}
	failure := failed[0].(map[string]any)
	if failure["id"] != "page:2" || failure["error"].(map[string]any)["type"] != "api" {
		t.Fatalf("failed entry=%#v", failure)
	}
}

func TestRelatedTasksDualValidatePreservesLegacyFailureWithoutPayload(t *testing.T) {
	stdout, err := runTodoAggregateLegacy(t, RelatedTasks, output.RolloutDualValidate, &todoAggregatePagingCaller{
		pages:  []string{todoAggregatePage(t, 1, todoPageSize)},
		failAt: 2,
	})
	if !errors.Is(err, errTodoAggregateSecondPage) {
		t.Fatalf("dual error=%v, want the historical second-page error", err)
	}
	if len(stdout) != 0 {
		t.Fatalf("dual validation must not invent legacy partial stdout: %s", stdout)
	}
}

func TestTodoAggregateLegacyFacadeRemainsFailClosedForWritePreflight(t *testing.T) {
	// +todo-done continues to call shortcutListAllTodoCards. The facade is the
	// explicit boundary that discards an incomplete aggregate so a later read
	// failure cannot select a target for a write. Its behavior is covered here
	// through the same core command and fake MCP caller.
	caller := &todoAggregatePagingCaller{pages: []string{todoAggregatePage(t, 1, todoPageSize)}, failAt: 2}
	_, err := runTodoAggregateLegacy(t, TodoDone, output.RolloutLegacyOnly, caller, "--task", "todo-1", "--yes")
	if !errors.Is(err, errTodoAggregateSecondPage) {
		t.Fatalf("todo-done error=%v, want later read failure", err)
	}
	if caller.calls != 2 {
		t.Fatalf("todo-done calls=%d, want only the two read pages and no write", caller.calls)
	}
}

func runTodoAggregateLegacy(t *testing.T, declaration shortcut.Shortcut, rollout output.RolloutState, caller *todoAggregatePagingCaller, args ...string) ([]byte, error) {
	t.Helper()
	helpers.InitDeps(caller)
	declaration.OutputRollout = rollout
	cmd := corecmd.New(shortcut.FromShortcut(declaration))
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.PersistentFlags().String("format", "json", "")
	cmd.PersistentFlags().Bool("yes", false, "")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return append([]byte(nil), stdout.Bytes()...), err
}

func runTodoAggregateUnified(t *testing.T, declaration shortcut.Shortcut, caller *todoAggregatePagingCaller, args ...string) (map[string]any, int, int) {
	t.Helper()
	helpers.InitDeps(caller)
	declaration.OutputRollout = output.RolloutUnifiedActive
	cmd := corecmd.New(shortcut.FromShortcut(declaration))
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.PersistentFlags().String("format", "json", "")
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unified execution: %v", err)
	}
	exitCode, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted {
		t.Fatalf("unified emission exit=%d emitted=%v err=%v", exitCode, emitted, err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode unified envelope: %v\n%s", err, stdout.String())
	}
	if _, present := envelope["contract_version"]; present {
		t.Fatalf("unified envelope must not expose a protocol version: %#v", envelope)
	}
	return envelope, exitCode, caller.calls
}

func todoAggregatePage(t *testing.T, start, count int) string {
	t.Helper()
	cards := make([]map[string]any, 0, count)
	for offset := 0; offset < count; offset++ {
		id := start + offset
		cards = append(cards, map[string]any{
			"taskId":           fmt.Sprintf("todo-%d", id),
			"subject":          fmt.Sprintf("todo-%d", id),
			"finalStatusStage": "TODO",
		})
	}
	raw, err := json.Marshal(map[string]any{"result": map[string]any{"todoCards": cards}})
	if err != nil {
		t.Fatalf("marshal todo page: %v", err)
	}
	return string(raw)
}
