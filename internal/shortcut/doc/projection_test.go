package doc

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestDocProjectionPreservesKnownEmptyContainers(t *testing.T) {
	docs, err := searchDocsProject(map[string]any{"documents": []any{}})
	if err != nil {
		t.Fatalf("known empty search response returned error: %v", err)
	}
	if docs == nil || len(docs) != 0 {
		t.Fatalf("known empty search response = %#v, want non-nil empty list", docs)
	}

	nodes, err := listNodesProject(map[string]any{"nodes": []any{}})
	if err != nil {
		t.Fatalf("known empty node response returned error: %v", err)
	}
	if nodes == nil || len(nodes) != 0 {
		t.Fatalf("known empty node response = %#v, want non-nil empty list", nodes)
	}
}

func TestDocProjectionRejectsUnknownShapeAndRow(t *testing.T) {
	_, err := searchDocsProject(map[string]any{"result": map[string]any{"unexpected": []any{}}})
	assertDocProjectionUnknown(t, err)

	_, err = listNodesProject(map[string]any{"nodes": []any{map[string]any{"opaque": true}}})
	assertDocProjectionUnknown(t, err)
}

func TestDocProjectionAcceptsNestedKnownContainers(t *testing.T) {
	docs, err := searchDocsProject(map[string]any{
		"result": map[string]any{"items": []any{map[string]any{"docId": "d1", "title": "Design"}}},
	})
	if err != nil {
		t.Fatalf("nested search response returned error: %v", err)
	}
	if len(docs) != 1 || docs[0]["nodeId"] != "d1" || docs[0]["name"] != "Design" {
		t.Fatalf("nested document projection = %#v", docs)
	}

	nodes, err := listNodesProject(map[string]any{
		"data": map[string]any{"nodes": []any{map[string]any{"nodeId": "n1", "nodeName": "Folder"}}},
	})
	if err != nil {
		t.Fatalf("nested list response returned error: %v", err)
	}
	if len(nodes) != 1 || nodes[0]["nodeId"] != "n1" || nodes[0]["name"] != "Folder" {
		t.Fatalf("nested node projection = %#v", nodes)
	}
}

func assertDocProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
