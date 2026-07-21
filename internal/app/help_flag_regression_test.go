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
	"context"
	"os"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/pipeline"
)

func TestLongHelpFlagIsNotFuzzyRewritten(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", t.TempDir())

	root := NewRootCommandWithEngine(context.Background(), nil)
	engine := newPipelineEngine()
	argv := []string{"connector", "mcp", "tool", "create", "--help"}
	previousArgs := os.Args
	os.Args = append([]string{"dws"}, argv...)
	t.Cleanup(func() { os.Args = previousArgs })

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	pipeline.RunPreParse(root, engine)
	_, err := root.ExecuteC()
	if err != nil {
		t.Fatalf("ExecuteC() error = %v; --help must reach the leaf command", err)
	}
	body := out.String()
	if strings.Contains(body, "flag needs an argument") {
		t.Fatalf("--help was rewritten to a value-taking flag:\n%s", body)
	}
	for _, want := range []string{"新建 MCP 工具草稿", "Usage:", "--http-info"} {
		if !strings.Contains(body, want) {
			t.Fatalf("leaf help missing %q:\n%s", want, body)
		}
	}
}
