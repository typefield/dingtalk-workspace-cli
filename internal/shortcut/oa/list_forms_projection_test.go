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

package oa

import (
	"encoding/json"
	"errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

// TestListFormsProjectProcessCodeListShape guards against the projection-data-loss
// class: list_user_visible_process nests the forms under result.processCodeList.
// The resolver MUST probe that exact key — otherwise the whole list silently
// projects to empty (exit 0, no error envelope) while the backend has data.
func TestListFormsProjectProcessCodeListShape(t *testing.T) {
	// Faithful list_user_visible_process shape (as returned by the backend).
	const raw = `{"result":{"processCodeList":[
		{"processCode":"PROC-1","processName":"leave","dirName":"attendance"},
		{"processCode":"PROC-2","processName":"check-in fix","dirName":"attendance"},
		{"processCode":"PROC-3","processName":"overtime","dirName":"attendance"}
	],"totalCount":-1},"success":true}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	forms, err := listFormsProject(data)
	if err != nil {
		t.Fatalf("projection returned error: %v", err)
	}
	if len(forms) != 3 {
		t.Fatalf("lower/upper mismatch: 3 forms in backend, projection returned %d (forms=%v)", len(forms), forms)
	}
	for _, f := range forms {
		if f["processCode"] == nil || f["name"] == nil {
			t.Fatalf("projected form missing processCode/name: %v", f)
		}
	}
}

// TestListFormsProjectBareArrayShape ensures a bare top-level array still works.
func TestListFormsProjectBareArrayShape(t *testing.T) {
	const raw = `{"result":[
		{"processCode":"PROC-9","processName":"generic approval"}
	]}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	forms, err := listFormsProject(data)
	if err != nil {
		t.Fatalf("bare array projection returned error: %v", err)
	}
	if len(forms) != 1 {
		t.Fatalf("bare array shape: want 1 form, got %d (%v)", len(forms), forms)
	}
}

// TestOAInstanceResolveListValuesShape guards the shared resolver behind
// +list-pending / +list-executed / +list-submitted / +list-cc. Those approval
// instance tools nest the list under result.values; the resolver must probe
// "values" or all four commands silently return empty despite backend records.
func TestOAInstanceResolveListValuesShape(t *testing.T) {
	const raw = `{"result":{"hasMore":false,"values":[
		{"processInstanceId":"i-1","title":"user leave"},
		{"processInstanceId":"i-2","title":"user reimbursement"}
	]}}`
	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	got, known := oaInstanceResolveList(data)
	if !known || len(got) != 2 {
		t.Fatalf("lower/upper mismatch: result.values has 2 entries, resolver returned %d", len(got))
	}
	// End-to-end through a real projection that uses the shared resolver.
	instances, err := listSubmittedProject(data)
	if err != nil {
		t.Fatalf("listSubmittedProject returned error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("listSubmittedProject: want 2, got %d (%v)", len(instances), instances)
	}
}

func TestOAListProjectionSeparatesKnownEmptyFromUnknown(t *testing.T) {
	for name, project := range map[string]func(map[string]any) ([]map[string]any, error){
		"forms":     listFormsProject,
		"search":    searchFormsProject,
		"pending":   listPendingProject,
		"executed":  listExecutedProject,
		"submitted": listSubmittedProject,
		"cc":        listCcProject,
	} {
		t.Run(name+" known empty", func(t *testing.T) {
			rows, err := project(map[string]any{"items": []any{}})
			if err != nil || rows == nil || len(rows) != 0 {
				t.Fatalf("known empty = %#v, %v; want non-nil empty list", rows, err)
			}
		})
		t.Run(name+" unknown", func(t *testing.T) {
			_, err := project(map[string]any{"unexpected": []any{}})
			assertOAProjectionUnknown(t, err)
		})
		t.Run(name+" malformed row", func(t *testing.T) {
			_, err := project(map[string]any{"items": []any{"opaque"}})
			assertOAProjectionUnknown(t, err)
		})
	}
}

func assertOAProjectionUnknown(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("projection unexpectedly succeeded")
	}
	var typed *apperrors.Error
	if !errors.As(err, &typed) || typed.Reason != "projection_unknown" || typed.Retryable {
		t.Fatalf("projection error = %T %#v, want non-retryable projection_unknown", err, err)
	}
}
