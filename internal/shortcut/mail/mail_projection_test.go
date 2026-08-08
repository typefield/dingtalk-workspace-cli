package mail

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

type mailListProject func(map[string]any) ([]map[string]any, error)

func TestMailListProjectionPreservesKnownEmpty(t *testing.T) {
	for name, project := range map[string]mailListProject{
		"threads":   threadListProject,
		"folders":   folderListProject,
		"tags":      tagListProject,
		"users":     userSearchProject,
		"templates": templateListProject,
		"contacts":  contactListProject,
	} {
		t.Run(name, func(t *testing.T) {
			rows, err := project(map[string]any{"items": []any{}})
			if err != nil {
				t.Fatalf("known empty list returned error: %v", err)
			}
			if rows == nil || len(rows) != 0 {
				t.Fatalf("known empty list = %#v, want non-nil empty list", rows)
			}
		})
	}
}

func TestMailListProjectionRejectsUnknownResponsesAndRows(t *testing.T) {
	for name, project := range map[string]mailListProject{
		"threads":   threadListProject,
		"folders":   folderListProject,
		"tags":      tagListProject,
		"users":     userSearchProject,
		"templates": templateListProject,
		"contacts":  contactListProject,
	} {
		t.Run(name+" unknown container", func(t *testing.T) {
			_, err := project(map[string]any{"unexpected": []any{}})
			assertMailProjectionUnknown(t, err)
		})
		t.Run(name+" malformed row", func(t *testing.T) {
			_, err := project(map[string]any{"items": []any{"opaque"}})
			assertMailProjectionUnknown(t, err)
		})
		t.Run(name+" unprojectable row", func(t *testing.T) {
			_, err := project(map[string]any{"items": []any{map[string]any{"opaque": true}}})
			assertMailProjectionUnknown(t, err)
		})
	}
}

func TestMailListProjectionAcceptsKnownNestedContainers(t *testing.T) {
	tests := []struct {
		name    string
		project mailListProject
		data    map[string]any
	}{
		{"threads", threadListProject, map[string]any{"result": map[string]any{"threads": []any{map[string]any{"conversationId": "t1"}}}}},
		{"folders", folderListProject, map[string]any{"data": map[string]any{"folders": []any{map[string]any{"id": "f1"}}}}},
		{"tags", tagListProject, map[string]any{"result": map[string]any{"tags": []any{map[string]any{"id": "tag1"}}}}},
		{"users", userSearchProject, map[string]any{"data": map[string]any{"users": []any{map[string]any{"email": "a@example.com"}}}}},
		{"templates", templateListProject, map[string]any{"result": map[string]any{"templates": []any{map[string]any{"id": "template1"}}}}},
		{"contacts", contactListProject, map[string]any{"data": map[string]any{"contacts": []any{map[string]any{"id": "contact1"}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows, err := test.project(test.data)
			if err != nil {
				t.Fatalf("nested known container returned error: %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("nested known container = %#v, want one row", rows)
			}
		})
	}
}

func assertMailProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
