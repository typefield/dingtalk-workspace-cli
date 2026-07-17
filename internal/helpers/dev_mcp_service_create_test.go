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

package helpers

import (
	"bytes"
	"strings"
	"testing"
)

// 漏传 --server-name 时顶层动态命令会静默退化为 mcp-<mcpId>（0717 实测），
// service create 必须给出显式警告，让 Agent 有机会自纠。
func TestConnectorMCPServiceCreateServerNameWarning(t *testing.T) {
	baseArgs := []string{
		"connector", "mcp", "service", "create",
		"--name", "天气查询",
		"--description", "Open-Meteo 天气数据查询",
		"--dry-run",
	}
	const warning = "未设置 --server-name"

	t.Run("warns when server name omitted", func(t *testing.T) {
		runner := &captureRunner{}
		root := newDevAppTestRoot(runner)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(baseArgs)

		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v\noutput:\n%s", err, out.String())
		}
		if !strings.Contains(out.String(), warning) {
			t.Fatalf("output missing server-name warning:\n%s", out.String())
		}
	})

	t.Run("no warning when server name provided", func(t *testing.T) {
		runner := &captureRunner{}
		root := newDevAppTestRoot(runner)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(append(append([]string{}, baseArgs...), "--server-name", "open-meteo-weather"))

		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v\noutput:\n%s", err, out.String())
		}
		if strings.Contains(out.String(), warning) {
			t.Fatalf("unexpected server-name warning:\n%s", out.String())
		}
	})
}
