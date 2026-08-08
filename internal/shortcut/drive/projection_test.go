package drive

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestDriveListProjectionSeparatesKnownEmptyFromUnknown(t *testing.T) {
	files, err := listFilesProject(map[string]any{"files": []any{}})
	if err != nil {
		t.Fatalf("known empty drive list returned error: %v", err)
	}
	if files == nil || len(files) != 0 {
		t.Fatalf("known empty drive list = %#v, want non-nil empty list", files)
	}

	_, err = listFilesProject(map[string]any{"unexpected": []any{}})
	assertDriveProjectionUnknown(t, err)
}

func TestDriveSearchProjectionRejectsUnknownRows(t *testing.T) {
	_, err := searchFilesProject(map[string]any{"data": map[string]any{"items": []any{"opaque"}}})
	assertDriveProjectionUnknown(t, err)

	_, err = searchDocsProject(map[string]any{"documents": []any{map[string]any{"opaque": true}}})
	assertDriveProjectionUnknown(t, err)
}

func TestDriveSearchProjectionAcceptsNestedKnownContainers(t *testing.T) {
	files, err := searchFilesProject(map[string]any{
		"result": map[string]any{"items": []any{map[string]any{"dentryUuid": "f1", "fileName": "report"}}},
	})
	if err != nil {
		t.Fatalf("nested file search returned error: %v", err)
	}
	if len(files) != 1 || files[0]["dentryId"] != "f1" || files[0]["name"] != "report" {
		t.Fatalf("nested file projection = %#v", files)
	}

	docs, err := searchDocsProject(map[string]any{
		"data": map[string]any{"docs": []any{map[string]any{"docId": "d1", "title": "Design"}}},
	})
	if err != nil {
		t.Fatalf("nested document search returned error: %v", err)
	}
	if len(docs) != 1 || docs[0]["nodeId"] != "d1" || docs[0]["name"] != "Design" {
		t.Fatalf("nested document projection = %#v", docs)
	}
}

func assertDriveProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
