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

package app

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
)

var manualAgentExamplePlaceholderPattern = regexp.MustCompile(`<([^>]+)>`)

// TestManualAgentExamplesDryRun executes every reviewed example through the
// real Cobra command and argument-validation path with global --dry-run forced
// on. The test is opt-in because it is intentionally exhaustive; normal CI
// already validates every path and flag in-process. No shell is involved and
// HOME is isolated, so examples cannot expand shell syntax or consume a
// developer's local DWS configuration.
func TestManualAgentExamplesDryRun(t *testing.T) {
	if os.Getenv("DWS_AGENT_EXAMPLES_DRY_RUN") != "1" {
		t.Skip("set DWS_AGENT_EXAMPLES_DRY_RUN=1 to execute every reviewed Agent example through Cobra --dry-run")
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("NO_PROXY", "")
	files := newManualAgentExampleFiles(t)

	data, err := os.ReadFile("../cli/schema_manual_hints.json")
	if err != nil {
		t.Fatalf("read reviewed Manual Agent hints: %v", err)
	}
	snapshot, err := cli.DecodeManualSchemaHintSource(data)
	if err != nil {
		t.Fatalf("DecodeManualSchemaHintSource() error = %v", err)
	}

	canonicals := make([]string, 0, len(snapshot.AgentHints.Tools))
	for canonical := range snapshot.AgentHints.Tools {
		canonicals = append(canonicals, canonical)
	}
	sort.Strings(canonicals)

	validated := 0
	for _, canonical := range canonicals {
		for index, example := range snapshot.AgentHints.Tools[canonical].Examples {
			canonical, index, example := canonical, index, example
			t.Run(fmt.Sprintf("%s/%d", strings.ReplaceAll(canonical, ".", "/"), index), func(t *testing.T) {
				argv, err := cli.ParseManualAgentExampleArgv(example)
				if err != nil {
					t.Fatalf("parse example %q: %v", example, err)
				}
				args := materializeManualAgentExampleArgv(argv[1:], files)
				if !manualAgentExampleHasFlag(args, "dry-run") {
					args = append([]string{"--dry-run"}, args...)
				}

				output, err := executeManualAgentExampleCapture(t, args)
				if err != nil {
					t.Fatalf("dry-run example failed: %v\nsource: %s\nargv: %q\noutput:\n%s", err, example, args, output)
				}
				if !manualAgentExampleDryRunObserved(output) {
					t.Fatalf("example returned without dry-run evidence\nsource: %s\nargv: %q\noutput:\n%s", example, args, output)
				}
				validated++
			})
		}
	}
	if validated == 0 {
		t.Fatal("no reviewed Agent examples were executed")
	}
	t.Logf("executed %d reviewed Agent examples through real Cobra --dry-run", validated)
}

func executeManualAgentExampleCapture(t testing.TB, args []string) (string, error) {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("open output capture pipe: %v", err)
	}
	os.Stdout, os.Stderr = writePipe, writePipe

	root := NewRootCommand()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs(args)
	execErr := root.Execute()

	_ = writePipe.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	captured, readErr := io.ReadAll(readPipe)
	_ = readPipe.Close()
	if readErr != nil {
		t.Fatalf("read output capture pipe: %v", readErr)
	}
	return output.String() + string(captured), execErr
}

type manualAgentExampleFiles struct {
	root     string
	markdown string
	binary   string
	image    string
}

func newManualAgentExampleFiles(t testing.TB) manualAgentExampleFiles {
	t.Helper()
	root := t.TempDir()
	markdown := filepath.Join(root, "content.md")
	binary := filepath.Join(root, "report.pdf")
	image := filepath.Join(root, "chart.png")
	for path, content := range map[string][]byte{
		markdown: []byte("# Agent dry-run fixture\n\nNo business call is allowed.\n"),
		binary:   []byte("%PDF-1.4\n%%EOF\n"),
		image:    {0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
	} {
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write dry-run fixture %s: %v", path, err)
		}
	}
	return manualAgentExampleFiles{root: root, markdown: markdown, binary: binary, image: image}
}

func materializeManualAgentExampleArgv(argv []string, files manualAgentExampleFiles) []string {
	result := append([]string(nil), argv...)
	for index := range result {
		result[index] = manualAgentExamplePlaceholderPattern.ReplaceAllStringFunc(result[index], func(match string) string {
			name := strings.TrimSuffix(strings.TrimPrefix(match, "<"), ">")
			switch strings.ToLower(name) {
			case "basetime", "remindertimestamp", "reminder-time-stamp":
				return "1780000000000"
			case "duedateoffset", "due-date-offset":
				return "0"
			case "reminderrules", "reminder-rules":
				return `[{"remindType":"minute","remindTime":10}]`
			case "filepath", "file-path":
				return files.binary
			case "uuid1,uuid2":
				return "uuid1,uuid2"
			default:
				clean := strings.NewReplacer(",", "_", "-", "_", ".", "_").Replace(name)
				return "test_" + clean
			}
		})
	}

	for index := 0; index < len(result); index++ {
		name, inline, ok := manualAgentExampleLongFlag(result[index])
		if !ok {
			continue
		}
		valueIndex := index + 1
		value := inline
		if inline == "" && valueIndex < len(result) {
			value = result[valueIndex]
		}
		replacement := ""
		switch name {
		case "file", "file-path":
			if strings.Contains(strings.ToLower(value), "png") {
				replacement = files.image
			} else {
				replacement = files.binary
			}
		case "content-file", "contents-file":
			replacement = files.markdown
		case "output":
			if value == "." || value == "" {
				replacement = files.root
			} else {
				replacement = filepath.Join(files.root, filepath.Base(value))
			}
		}
		if replacement == "" {
			continue
		}
		if inline != "" {
			result[index] = "--" + name + "=" + replacement
		} else if valueIndex < len(result) {
			result[valueIndex] = replacement
			index++
		}
	}
	return result
}

func manualAgentExampleLongFlag(argument string) (name, inline string, ok bool) {
	if !strings.HasPrefix(argument, "--") {
		return "", "", false
	}
	name, inline, _ = strings.Cut(strings.TrimPrefix(argument, "--"), "=")
	return name, inline, name != ""
}

func manualAgentExampleHasFlag(argv []string, target string) bool {
	for _, argument := range argv {
		if argument == "--"+target || strings.HasPrefix(argument, "--"+target+"=") {
			return true
		}
	}
	return false
}

func manualAgentExampleDryRunObserved(output string) bool {
	normalized := strings.ToLower(output)
	return strings.Contains(normalized, "dry-run") ||
		strings.Contains(normalized, `"dry_run":true`) ||
		strings.Contains(normalized, `"dry_run": true`) ||
		(strings.Contains(output, "Tool:") && strings.Contains(output, "Arguments:"))
}
