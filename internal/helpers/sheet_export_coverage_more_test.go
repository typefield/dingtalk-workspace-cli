package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func executeSheetExportCoverage(t *testing.T, caller *scriptedToolCaller, args ...string) error {
	t.Helper()
	installScriptedCaller(t, caller)
	oldArgs := os.Args
	os.Args = []string{"dws", "sheet"}
	t.Cleanup(func() { os.Args = oldArgs })
	cmd := newExportCmd()
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	output.SetCommandRollout(cmd, output.RolloutUnifiedActive)
	for index := 0; index < len(args); index += 2 {
		if err := cmd.Flags().Set(args[index], args[index+1]); err != nil {
			t.Fatalf("set %s: %v", args[index], err)
		}
	}
	return runSheetExport(cmd, nil)
}

func executeSheetExportCapture(t *testing.T, caller *scriptedToolCaller, args ...string) (string, string, error) {
	t.Helper()
	oldDeps := deps
	InitDeps(caller)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	deps.Out.SetWriters(stdout, stderr)
	t.Cleanup(func() { deps = oldDeps })

	cmd := newExportCmd()
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	output.SetCommandRollout(cmd, output.RolloutUnifiedActive)
	for index := 0; index < len(args); index += 2 {
		if err := cmd.Flags().Set(args[index], args[index+1]); err != nil {
			t.Fatalf("set %s: %v", args[index], err)
		}
	}
	err := runSheetExport(cmd, nil)
	if err == nil {
		if _, _, emitErr := output.EmitStoredResult(cmd); emitErr != nil {
			err = emitErr
		}
	}
	return stdout.String(), stderr.String(), err
}

func TestCrossPlatformCoverageSheetExportCommandRemainingCoverage(t *testing.T) {
	installImmediateTiming(t)
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{}); err == nil {
		t.Fatal("missing node returned nil")
	} else {
		var typed *apperrors.Error
		if !errors.As(err, &typed) || typed.Category != apperrors.CategoryValidation {
			t.Fatalf("missing node error = %#v, want typed validation", err)
		}
	}
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{dry: true}, "node", "node", "output", "file.xlsx"); err != nil {
		t.Fatalf("dry run with output: %v", err)
	}
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{dry: true}, "node", "node"); err != nil {
		t.Fatalf("dry run without output: %v", err)
	}

	successSteps := func(url string) []scriptedToolStep {
		return []scriptedToolStep{{text: `{"jobId":"job"}`}, {text: `{"status":"SUCCESS","downloadUrl":"` + url + `"}`}}
	}
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{steps: []scriptedToolStep{{err: errors.New("submit")}}}, "node", "node"); err == nil {
		t.Fatal("submit error returned nil")
	}
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{steps: []scriptedToolStep{{text: `{`}}}, "node", "node"); err == nil {
		t.Fatal("invalid submit response returned nil")
	}
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"jobId":"job"}`}, {text: `{`}}}, "node", "node"); err == nil {
		t.Fatal("poll parse error returned nil")
	}
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{format: "table", steps: successSteps("https://example.test/file.xlsx")}, "node", "node"); err != nil {
		t.Fatalf("table URL result: %v", err)
	}
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{format: "json", steps: successSteps("https://example.test/file.xlsx")}, "node", "node"); err != nil {
		t.Fatalf("JSON URL result: %v", err)
	}

	oldGet := httpGetFile
	t.Cleanup(func() { httpGetFile = oldGet })
	var destination string
	httpGetFile = func(_ context.Context, _ string, _ map[string]string, output string) error {
		destination = output
		return nil
	}
	directory := t.TempDir()
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{format: "table", steps: successSteps("https://example.test/")}, "node", "node", "output", directory); err != nil {
		t.Fatalf("directory output: %v", err)
	}
	if filepath.Base(destination) != "sheet-export-job.xlsx" {
		t.Fatalf("fallback destination = %q", destination)
	}
	file := filepath.Join(directory, "explicit.xlsx")
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{format: "json", steps: successSteps("https://example.test/file.xlsx")}, "node", "node", "output", file); err != nil {
		t.Fatalf("JSON file output: %v", err)
	}

	boom := errors.New("download failed")
	httpGetFile = func(context.Context, string, map[string]string, string) error { return boom }
	if err := executeSheetExportCoverage(t, &scriptedToolCaller{format: "table", steps: successSteps("https://example.test/file.xlsx")}, "node", "node", "output", file); err == nil {
		t.Fatal("download error returned nil")
	}
}

func TestSheetExportDryRunUsesUnifiedResult(t *testing.T) {
	stdout, stderr, err := executeSheetExportCapture(t, &scriptedToolCaller{dry: true},
		"node", "node-1", "output", "report.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal([]byte(stdout), &wire); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if wire["ok"] != true || wire["outcome"] != "success" || wire["dry_run"] != true {
		t.Fatalf("wire = %#v", wire)
	}
	if _, present := wire["contract_version"]; present {
		t.Fatalf("unified result must not carry a protocol version: %#v", wire)
	}
	data, ok := wire["data"].(map[string]any)
	if !ok || data["executed"] != false || data["operation"] != "export_sheet_xlsx" {
		t.Fatalf("data = %#v", wire["data"])
	}
	if stderr != "" {
		t.Fatalf("dry-run stderr = %q, want empty", stderr)
	}
}

func TestCrossPlatformCoverageSheetExportFilenameDotCoverage(t *testing.T) {
	if got := inferSheetExportFilename("https://example.test/."); got != "" {
		t.Fatalf("dot filename = %q", got)
	}
}
