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

// scan_rollout_ledger is an Agent release-review helper. It builds the current
// Cobra command tree in an isolated configuration directory and writes a
// Markdown-only inventory of runnable command-node rollout declarations. It never
// changes the CLI public surface and is deliberately not a CI/policy gate.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/app"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/spf13/cobra"
)

type row struct {
	Path     string
	Rollout  output.RolloutState
	Contract output.ContractMode
	Hidden   bool
}

type previousRow struct {
	Rollout  output.RolloutState
	Contract output.ContractMode
}

var ledgerRow = regexp.MustCompile("^\\| `([^`]+)` \\| `([^`]+)` \\| `([^`]+)` \\| (yes|no) \\|$")

func main() {
	os.Exit(run())
}

func run() int {
	outputPath := flag.String("output", "", "Markdown evidence output path (required)")
	baselinePath := flag.String("baseline", "", "optional prior Markdown ledger for transition review")
	flag.Parse()
	if *outputPath == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: go run scripts/agent/scan_rollout_ledger.go --output <markdown> [--baseline <markdown>]")
		return 2
	}

	home, err := os.MkdirTemp("", "dws-rollout-ledger-agent-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create isolated configuration: %v\n", err)
		return 2
	}
	defer os.RemoveAll(home)
	restore := setIsolatedEnvironment(home)
	defer restore()

	rows := collect(app.NewRootCommand(context.Background()))
	previous, baselineNote, err := parseBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	report := render(rows, previous, baselineNote)
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create report directory: %v\n", err)
		return 2
	}
	if err := os.WriteFile(*outputPath, []byte(report), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write Markdown evidence: %v\n", err)
		return 2
	}
	return 0
}

func setIsolatedEnvironment(home string) func() {
	values := map[string]string{
		"DWS_CONFIG_DIR": home,
		"HOME":           home,
		"USERPROFILE":    home,
		"NO_COLOR":       "1",
	}
	type previous struct {
		value string
		set   bool
	}
	old := make(map[string]previous, len(values))
	for key, value := range values {
		prior, set := os.LookupEnv(key)
		old[key] = previous{value: prior, set: set}
		_ = os.Setenv(key, value)
	}
	return func() {
		for key, prior := range old {
			if prior.set {
				_ = os.Setenv(key, prior.value)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	}
}

func collect(root *cobra.Command) []row {
	var rows []row
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		if command != root && (command.Run != nil || command.RunE != nil) {
			rows = append(rows, row{
				Path:     command.CommandPath(),
				Rollout:  output.CommandRollout(command),
				Contract: output.ActiveContract(command),
				Hidden:   command.Hidden,
			})
		}
		for _, child := range command.Commands() {
			walk(child)
		}
	}
	walk(root)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	return rows
}

func parseBaseline(path string) (map[string]previousRow, string, error) {
	if path == "" {
		return nil, "未提供基线；这是初始 inventory，后续发布应以本报告作为 --baseline。", nil
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read baseline ledger %q: %w", path, err)
	}
	previous := make(map[string]previousRow)
	for _, line := range strings.Split(string(contents), "\n") {
		matches := ledgerRow.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		rollout, err := output.ParseRolloutState(matches[2])
		if err != nil {
			return nil, "", fmt.Errorf("parse rollout in baseline %q: %w", path, err)
		}
		contract := output.ContractMode(matches[3])
		if contract != output.ContractLegacy && contract != output.ContractUnified {
			return nil, "", fmt.Errorf("parse active contract %q in baseline %q", contract, path)
		}
		previous[matches[1]] = previousRow{Rollout: rollout, Contract: contract}
	}
	if len(previous) == 0 {
		return nil, "", fmt.Errorf("baseline ledger %q has no command rows", path)
	}
	return previous, fmt.Sprintf("基线：`%s`（%d 条 runnable command node）。", filepath.Base(path), len(previous)), nil
}

func render(rows []row, previous map[string]previousRow, baselineNote string) string {
	byState := map[output.RolloutState]int{}
	for _, row := range rows {
		byState[row.Rollout]++
	}

	var transitions []string
	var additions []string
	seen := make(map[string]bool, len(rows))
	for _, current := range rows {
		seen[current.Path] = true
		prior, exists := previous[current.Path]
		if !exists {
			if previous != nil {
				additions = append(additions, current.Path)
			}
			continue
		}
		if prior.Rollout == current.Rollout && prior.Contract == current.Contract {
			continue
		}
		if err := output.ValidateRolloutTransition(prior.Rollout, current.Rollout, false); err != nil {
			transitions = append(transitions, fmt.Sprintf("REVIEW: `%s`: `%s` → `%s` (%v)", current.Path, prior.Rollout, current.Rollout, err))
			continue
		}
		transitions = append(transitions, fmt.Sprintf("PASS: `%s`: `%s` → `%s`", current.Path, prior.Rollout, current.Rollout))
	}
	var removals []string
	for path := range previous {
		if !seen[path] {
			removals = append(removals, path)
		}
	}
	sort.Strings(transitions)
	sort.Strings(additions)
	sort.Strings(removals)

	var builder strings.Builder
	builder.WriteString("# Unified output rollout Agent ledger\n\n")
	builder.WriteString("扫描时间：")
	builder.WriteString(time.Now().Format(time.RFC3339))
	builder.WriteString("\n\n")
	builder.WriteString("> 本报告由 Agent 在隔离配置目录中装配真实 Cobra tree 后生成。它只记录 Markdown，不暴露内部 rollout 到 Help/Schema/CLI，不保存 JSON catalog，也不是 CI / policy gate。\n\n")
	builder.WriteString("## 当前 inventory\n\n")
	builder.WriteString("| rollout state | runnable command nodes |\n|---|---:|\n")
	for _, state := range []output.RolloutState{output.RolloutLegacyOnly, output.RolloutDualValidate, output.RolloutUnifiedActive, output.RolloutUnifiedStable, output.RolloutUnifiedOnly} {
		fmt.Fprintf(&builder, "| `%s` | %d |\n", state, byState[state])
	}
	fmt.Fprintf(&builder, "\n可执行命令节点总数：**%d**。其中可能包含兼具子命令与直接执行语义的 Cobra 节点；它不等同于公开 Schema 工具计数。\n\n", len(rows))

	builder.WriteString("## Transition review\n\n")
	builder.WriteString(baselineNote)
	builder.WriteString("\n\n")
	if previous == nil {
		builder.WriteString("初始 inventory 不对状态迁移下结论；发布审阅必须补充兼容样本、观测窗口与回滚责任人。\n\n")
	} else {
		writeList(&builder, "状态迁移", transitions, "无状态迁移。")
		writeList(&builder, "新增可执行命令节点", additions, "无新增可执行命令节点。")
		writeList(&builder, "移除可执行命令节点", removals, "无移除可执行命令节点。")
	}

	builder.WriteString("## Live command declaration\n\n")
	builder.WriteString("| cli path | rollout state | active contract | hidden |\n|---|---|---|---|\n")
	for _, row := range rows {
		fmt.Fprintf(&builder, "| `%s` | `%s` | `%s` | %s |\n", row.Path, row.Rollout, row.Contract, yesNo(row.Hidden))
	}
	builder.WriteString("\n## Review boundary\n\n")
	builder.WriteString("- 本 inventory 只证明当前 command declaration 与 rollout state；不证明业务请求、服务端终态、分页覆盖或真实账号安全性。\n")
	builder.WriteString("- `dual_validate` 仍应保持 legacy stdout；所有晋级必须由 Agent 另行核对 legacy golden、统一结果样本、非零退出码与回滚计划。\n")
	builder.WriteString("- `REVIEW` 表示跳级或未记录回退；它不是自动阻断。人类发布负责人必须决定是否附上批准与理由。\n")
	return builder.String()
}

func writeList(builder *strings.Builder, heading string, values []string, empty string) {
	builder.WriteString("### ")
	builder.WriteString(heading)
	builder.WriteString("\n\n")
	if len(values) == 0 {
		builder.WriteString(empty)
		builder.WriteString("\n\n")
		return
	}
	for _, value := range values {
		builder.WriteString("- ")
		builder.WriteString(value)
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
