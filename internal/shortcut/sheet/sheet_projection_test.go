package sheet

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

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
