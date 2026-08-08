// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package shortcut

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/spf13/cobra"
)

func testPartialResult(t *testing.T) output.CommandResult {
	t.Helper()
	partial, err := output.NewPartialData(
		2,
		[]any{map[string]any{"id": "step:created"}},
		[]output.PartialFailedEntry{{
			ID: "step:write",
			Error: &output.ErrorInfo{
				Type: "api", Subtype: "document_write_failed", Message: "write failed",
			},
		}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewPartialData: %v", err)
	}
	return output.Partial(partial)
}

func TestOutputPartialPreservesLegacyErrorDuringDualValidation(t *testing.T) {
	ctx, store := output.WithResultStore(context.Background())
	cmd := &cobra.Command{Use: "sample"}
	cmd.SetContext(ctx)
	output.SetCommandRollout(cmd, output.RolloutDualValidate)
	legacy := errors.New("legacy partial error")
	err := (&RuntimeContext{cmd: cmd}).OutputPartial(testPartialResult(t), legacy)
	if !errors.Is(err, legacy) {
		t.Fatalf("OutputPartial error = %v, want legacy error", err)
	}
	if _, emitted := output.StoredExitCode(store); emitted {
		t.Fatal("dual validation stored an externally emitted result")
	}
}

func TestOutputPartialEmitsOneRC7ResultWhenUnifiedIsActive(t *testing.T) {
	ctx, _ := output.WithResultStore(context.Background())
	cmd := &cobra.Command{Use: "sample"}
	cmd.SetContext(ctx)
	cmd.Flags().String("format", "json", "")
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	output.SetCommandRollout(cmd, output.RolloutUnifiedActive)
	if err := (&RuntimeContext{cmd: cmd}).OutputPartial(testPartialResult(t), errors.New("legacy partial error")); err != nil {
		t.Fatalf("OutputPartial: %v", err)
	}
	code, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted || code != 7 {
		t.Fatalf("EmitStoredResult code=%d emitted=%v err=%v", code, emitted, err)
	}
	if !strings.Contains(stdout.String(), `"outcome": "partial_failure"`) || strings.Contains(stderr.String(), "legacy partial error") {
		t.Fatalf("unified partial output stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
