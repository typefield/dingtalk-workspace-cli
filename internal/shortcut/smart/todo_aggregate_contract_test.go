// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package smart

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestTodoAggregateShortcutsHaveReadOnlyContracts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		command  string
		shortcut shortcut.Shortcut
	}{
		{name: "related", path: "todo.shortcut_related_tasks", command: "+related-tasks", shortcut: RelatedTasks},
		{name: "due-today", path: "todo.shortcut_due_today", command: "+due-today", shortcut: DueToday},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotPath, gotCommand := tc.shortcut.Contract.Identity.CanonicalPath, tc.shortcut.Command
			effect, risk := tc.shortcut.Safety.Effect, tc.shortcut.Safety.Risk
			confirmation, idempotency := tc.shortcut.Safety.Confirmation, tc.shortcut.Safety.Idempotency
			if gotPath != tc.path || gotCommand != tc.command {
				t.Fatalf("identity=(%q,%q), want (%q,%q)", gotPath, gotCommand, tc.path, tc.command)
			}
			if effect != "read" || risk != "low" || confirmation != "not_required" || idempotency != "idempotent" {
				t.Fatalf("safety=(%q,%q,%q,%q), want read/low/not_required/idempotent", effect, risk, confirmation, idempotency)
			}
		})
	}
}

func TestRelatedRoleTypesRejectsUnknownAndEmptyEntries(t *testing.T) {
	got, err := parseRelatedRoleTypes(" creator, executor ,,participant ")
	if err != nil || len(got) != 3 {
		t.Fatalf("parse valid roles=%v err=%v", got, err)
	}
	if _, err := parseRelatedRoleTypes("creator,owner"); err == nil {
		t.Fatal("unknown role must be rejected")
	}
}

func TestRelatedProjectPreservesStableIdentityAndDueTime(t *testing.T) {
	item := shortcutRelatedProject(map[string]any{
		"taskId":           "task-1",
		"subject":          "Prepare report",
		"finalStatusStage": "TODO",
		"priority":         float64(2),
		"creatorName":      "Alice",
		"planFinishDate":   float64(1770000000000),
	}, "task-1")
	if item["taskId"] != "task-1" || item["title"] != "Prepare report" || item["status"] != "TODO" || item["creator"] != "Alice" {
		t.Fatalf("projected item=%#v", item)
	}
	if item["planFinishDate"] != int64(1770000000000) {
		t.Fatalf("due time=%#v", item["planFinishDate"])
	}
}
