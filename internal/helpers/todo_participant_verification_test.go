package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func installTodoParticipantVerificationCaller(t *testing.T, caller *scriptedToolCaller) *bytes.Buffer {
	t.Helper()
	previousDeps, previousArgs := deps, os.Args
	t.Cleanup(func() {
		deps = previousDeps
		os.Args = previousArgs
	})
	os.Args = []string{"dws", "todo"}
	caller.format = "json"
	InitDeps(caller)
	out := &bytes.Buffer{}
	deps.Out.w = out
	deps.Out.errW = &bytes.Buffer{}
	return out
}

func TestAddTodoParticipantsTreatsVerifiedWriteAfterErrorAsSuccess(t *testing.T) {
	caller := &scriptedToolCaller{steps: []scriptedToolStep{
		{err: errors.New("upstream response lost")},
		{text: `{"result":{"todoDetailModel":{"participantIds":["u1","u2"]}}}`},
	}}
	out := installTodoParticipantVerificationCaller(t, caller)

	if err := addTodoParticipantsWithVerification(context.Background(), "task-1", []string{"u1", "u2"}); err != nil {
		t.Fatalf("verified applied write returned error: %v", err)
	}
	if caller.calls != 2 {
		t.Fatalf("calls = %d, want one write plus one read", caller.calls)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("recovery output is not JSON: %v\n%s", err, out.String())
	}
	if payload["success"] != true || payload["applied"] != true || payload["verified"] != true ||
		payload["verification"] != "read_after_error" {
		t.Fatalf("recovery payload = %#v", payload)
	}
}

func TestAddTodoParticipantsDoesNotReplayWhenVerificationIsUnknown(t *testing.T) {
	caller := &scriptedToolCaller{steps: []scriptedToolStep{
		{err: errors.New("timeout")},
		{text: `{"result":{"todoDetailModel":{"taskId":"task-1"}}}`},
	}}
	installTodoParticipantVerificationCaller(t, caller)

	err := addTodoParticipantsWithVerification(context.Background(), "task-1", []string{"u1"})
	if err == nil || !strings.Contains(err.Error(), "远端是否已落库未知") || !strings.Contains(err.Error(), "不要直接重试") {
		t.Fatalf("unknown outcome error = %v", err)
	}
	if caller.calls != 2 {
		t.Fatalf("calls = %d, want one write plus one read and no replay", caller.calls)
	}
}

func TestTodoParticipantIDsFromDetailRequiresParticipantEvidence(t *testing.T) {
	ids, known, err := todoParticipantIDsFromDetail(`{"result":{"todoDetailModel":{"participants":[{"userId":"u1"}],"executorIds":["e1"]}}}`)
	if err != nil || !known || !ids["u1"] || ids["e1"] {
		t.Fatalf("participant evidence = ids=%v known=%v err=%v", ids, known, err)
	}
	_, known, err = todoParticipantIDsFromDetail(`{"result":{"todoDetailModel":{"taskId":"t"}}}`)
	if err != nil || known {
		t.Fatalf("missing participant evidence = known=%v err=%v", known, err)
	}
}
