// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package smart

import (
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestFindFileProjectDistinguishesKnownEmptyFromUnknown(t *testing.T) {
	files, err := shortcutFindFileProject(map[string]any{"result": map[string]any{"items": []any{}}})
	if err != nil {
		t.Fatalf("known empty search: %v", err)
	}
	if files == nil || len(files) != 0 {
		t.Fatalf("known empty files=%#v, want non-nil empty list", files)
	}

	_, err = shortcutFindFileProject(map[string]any{"result": map[string]any{"diagnostics": []any{}}})
	assertFindFileProjectionUnknown(t, err)
}

func TestFindFileProjectRejectsMalformedAndUntargetableRows(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"non-object row": {"items": []any{"opaque"}},
		"missing id":     {"items": []any{map[string]any{"name": "display-only"}}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := shortcutFindFileProject(data)
			assertFindFileProjectionUnknown(t, err)
		})
	}
}

func TestFindFileProjectProjectsStableID(t *testing.T) {
	files, err := shortcutFindFileProject(map[string]any{
		"result": map[string]any{"data": map[string]any{"files": []any{map[string]any{
			"fileName": "report", "dentryUuid": "file-1", "size": float64(42),
		}}}},
	})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if len(files) != 1 || files[0]["dentryId"] != "file-1" || files[0]["fileSize"] != int64(42) {
		t.Fatalf("projected files=%#v", files)
	}
}

func TestFindFileRolloutIsUnifiedActive(t *testing.T) {
	if FindFile.OutputRollout != output.RolloutUnifiedActive {
		t.Fatalf("find-file rollout=%q, want unified_active", FindFile.OutputRollout)
	}
}

func assertFindFileProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want projection error")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error=%T %#v, want non-retryable projection_unknown", err, err)
	}
}
