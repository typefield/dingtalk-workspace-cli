// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

// Command skill-command-check verifies that executable `dws ...` references
// in published skill Markdown resolve against the current Cobra command tree.
package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/app"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	inlineCommand = regexp.MustCompile("`(dws\\s+[^`]+)`")
	lineCommand   = regexp.MustCompile(`^\s*(?:[>$]\s*)?(dws\s+.+?)\s*$`)
	antiMarkers   = []string{
		"禁止", "不存在", "不要使用", "不要用", "错误写法", "错误命令",
		"反模式", "反例", "错例", "臆造", "虚构", "不支持", "unknown ",
		"❌", "×", "已下线", "废弃",
	}
	antiCommands = map[string]bool{
		"dws calendar list":         true,
		"dws minutes detail":        true,
		"dws minutes info":          true,
		"dws minutes summary":       true,
		"dws minutes transcribe":    true,
		"dws minutes transcription": true,
		"dws report inbox":          true,
	}
	// These are intentionally retained as badcase examples in the published
	// Skill. They must not be treated as executable recipes, even though their
	// parent command exists and the checker can otherwise resolve that path.
	antiReferences = map[string]bool{
		"dws minutes get --uuid <uuid>":      true,
		"dws minutes get --task-uuid <uuid>": true,
	}
)

type commandRef struct {
	File string
	Line int
	Text string
}

type commandResolution uint8

const (
	resolutionValid commandResolution = iota
	resolutionInvalid
	resolutionSkip
)

func main() {
	rootPath, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	semantic := false
	for _, arg := range os.Args[1:] {
		if arg == "--agent-semantic" {
			semantic = true
		}
	}
	os.Exit(runWithOptions(rootPath, app.NewRootCommand(), os.Stdout, os.Stderr, semantic))
}

func run(rootPath string, root *cobra.Command, stdout, stderr io.Writer) int {
	return runWithOptions(rootPath, root, stdout, stderr, false)
}

// runWithOptions keeps the normal path checker backward compatible while
// allowing the Agent-facing audit to report a semantic distinction that path
// integrity alone cannot see: a referenced flag may still execute only because
// it is a hidden compatibility alias. This mode is evidence-only; it never
// changes the normal checker result or CI gate.
func runWithOptions(rootPath string, root *cobra.Command, stdout, stderr io.Writer, agentSemantic bool) int {
	refs, err := extractReferences(filepath.Join(rootPath, "skills"))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	root.InitDefaultHelpCmd()
	var failures []string
	var hiddenFlagReferences []string
	checked := map[string]bool{}
	for _, ref := range refs {
		path, flags, skip := parseReference(ref.Text)
		if skip || path == "" || antiCommands[path] || antiReferences[strings.TrimSpace(ref.Text)] {
			continue
		}
		if issue := schemaProjectionIssue(ref.Text); issue != "" {
			failures = append(failures, formatFailure(rootPath, ref, issue))
			continue
		}
		if checked[path] {
			continue
		}

		switch resolveCommandReference(root, path) {
		case resolutionSkip:
			continue
		case resolutionInvalid:
			failures = append(failures, formatFailure(rootPath, ref, "command path does not exist"))
			continue
		case resolutionValid:
			if agentSemantic {
				if hidden := hiddenReferencedFlags(root, path, flags); len(hidden) > 0 {
					hiddenFlagReferences = append(hiddenFlagReferences,
						fmt.Sprintf("%s:%d: `%s`: hidden compatibility flag(s): %s",
							relativePath(rootPath, ref.File), ref.Line, ref.Text, strings.Join(hidden, ", ")))
				}
			}
			if issue := validateReferenceFlags(root, path, flags); issue != "" {
				failures = append(failures, formatFailure(rootPath, ref, issue))
				continue
			}
			checked[path] = true
		}
	}

	sort.Strings(failures)
	if len(failures) > 0 {
		fmt.Fprintf(stderr, "skill command integrity check failed (%d references):\n", len(failures))
		for _, failure := range failures {
			fmt.Fprintf(stderr, "  - %s\n", failure)
		}
		return 1
	}
	if agentSemantic {
		sort.Strings(hiddenFlagReferences)
		fmt.Fprintf(stdout, "Agent semantic flag review: %d hidden compatibility references\n", len(hiddenFlagReferences))
		for _, finding := range hiddenFlagReferences {
			fmt.Fprintf(stdout, "  - REVIEW: %s\n", finding)
		}
	}
	fmt.Fprintf(stdout, "skill command integrity check: ok (%d executable command paths)\n", len(checked))
	return 0
}

func relativePath(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return relative
}

func hiddenReferencedFlags(root *cobra.Command, path string, flags []string) []string {
	if len(flags) == 0 {
		return nil
	}
	cmd, remaining, err := root.Find(strings.Fields(strings.TrimPrefix(path, "dws ")))
	if err != nil || cmd == nil || len(remaining) > 0 {
		return nil
	}
	var hidden []string
	for _, name := range flags {
		if name == "help" {
			continue
		}
		var flag *pflag.Flag
		if flag = cmd.Flags().Lookup(name); flag == nil {
			flag = cmd.InheritedFlags().Lookup(name)
		}
		if flag == nil {
			flag = cmd.PersistentFlags().Lookup(name)
		}
		if flag != nil && flag.Hidden {
			hidden = append(hidden, "--"+name)
		}
	}
	return uniqueSorted(hidden)
}

// schemaProjectionIssue keeps published Agent instructions on the bounded
// Schema projection. A targeted full leaf is intentionally reserved for
// mapping/provenance audits; those callers must select the exact fields they
// need with --jq/--fields instead of loading the full payload into context.
func schemaProjectionIssue(raw string) string {
	tokens := shellFields(raw)
	if len(tokens) < 2 || tokens[0] != "dws" || tokens[1] != "schema" {
		return ""
	}
	var targeted, compact, selected, all bool
	for i := 2; i < len(tokens); i++ {
		token := tokens[i]
		switch {
		case token == "--compact":
			compact = true
		case token == "--all":
			all = true
		case token == "--cli-path":
			targeted = true
			if i+1 < len(tokens) {
				i++
			}
		case strings.HasPrefix(token, "--cli-path="):
			targeted = true
		case token == "--jq" || token == "--fields":
			selected = true
			if i+1 < len(tokens) {
				i++
			}
		case strings.HasPrefix(token, "--jq="):
			selected = true
		case strings.HasPrefix(token, "--fields="):
			selected = true
		case token == "--format" || token == "-f":
			if i+1 < len(tokens) {
				i++
			}
		case strings.HasPrefix(token, "--format="):
			// Output encoding does not bound the Schema fields.
		case strings.HasPrefix(token, "-"):
			// Other global flags do not affect projection size.
		default:
			targeted = true
		}
	}
	if targeted && !all && !compact && !selected {
		return "targeted Schema queries in Agent instructions must use --compact (or an explicit --jq/--fields projection)"
	}
	return ""
}

func extractReferences(root string) ([]commandRef, error) {
	var refs []commandRef
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		lineNumber := 0
		inFence := false
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inFence = !inFence
				continue
			}
			if isAntiPatternLine(line) {
				continue
			}
			seen := map[string]bool{}
			for _, match := range inlineCommand.FindAllStringSubmatch(line, -1) {
				command := strings.TrimSpace(match[1])
				seen[command] = true
				refs = append(refs, commandRef{File: path, Line: lineNumber, Text: command})
			}
			if inFence {
				match := lineCommand.FindStringSubmatch(line)
				if len(match) != 2 {
					continue
				}
				command := strings.TrimSpace(match[1])
				if !seen[command] {
					refs = append(refs, commandRef{File: path, Line: lineNumber, Text: command})
				}
			}
		}
		return scanner.Err()
	})
	return refs, err
}

func isAntiPatternLine(line string) bool {
	for _, marker := range antiMarkers {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

func parseReference(raw string) (string, []string, bool) {
	if strings.Contains(raw, "|") || strings.Contains(raw, "$(") || strings.Contains(raw, "&&") || strings.Contains(raw, " & ") {
		return "", nil, true
	}
	if comment := strings.Index(raw, " #"); comment >= 0 {
		raw = raw[:comment]
	}
	raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "\\"))
	if strings.Contains(raw, "[flags]") || strings.Contains(raw, "[command]") || strings.Contains(raw, "...") {
		return "", nil, true
	}
	tokens := shellFields(raw)
	if len(tokens) < 2 || tokens[0] != "dws" {
		return "", nil, true
	}
	var pathTokens []string
	var flags []string
	for i := 1; i < len(tokens); i++ {
		token := tokens[i]
		if strings.HasPrefix(token, "--") {
			name := strings.TrimPrefix(strings.SplitN(token, "=", 2)[0], "--")
			if name != "" {
				flags = append(flags, name)
			}
			if !strings.Contains(token, "=") && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				i++
			}
			continue
		}
		if strings.HasPrefix(token, "-") && len(token) == 2 {
			// Shorthand validity is already covered by the help compatibility
			// baseline; command references are keyed by long flags where present.
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				i++
			}
			continue
		}
		pathTokens = append(pathTokens, token)
	}
	for _, token := range pathTokens {
		if strings.Contains(token, "/") || strings.Contains(token, "*") {
			return "", nil, true
		}
	}
	return "dws " + strings.Join(pathTokens, " "), uniqueSorted(flags), false
}

func resolveCommandReference(root *cobra.Command, path string) commandResolution {
	cmd, remaining, err := root.Find(strings.Fields(strings.TrimPrefix(path, "dws ")))
	if err != nil || cmd == nil {
		return resolutionInvalid
	}
	if len(remaining) == 0 {
		return resolutionValid
	}
	if !cmd.HasSubCommands() {
		// Once a leaf command has been found, remaining tokens are positional
		// arguments. This check intentionally validates command paths only.
		return resolutionValid
	}
	if isPlaceholder(remaining[0]) {
		// Documentation such as `dws <cmd> --help` or
		// `dws sheet <command> --help` describes a command shape rather than an
		// executable command reference.
		return resolutionSkip
	}
	return resolutionInvalid
}

// validateReferenceFlags checks the flags that a published recipe explicitly
// passes against the same Cobra leaf used for path resolution. Path-only
// checks are insufficient: a hidden/deprecated alias may keep executing while
// a recipe's canonical flag has already drifted or been removed. We accept
// hidden flags here because they remain valid compatibility inputs; the
// semantic Agent review can separately enforce which spelling is documented.
func validateReferenceFlags(root *cobra.Command, path string, flags []string) string {
	if len(flags) == 0 {
		return ""
	}
	cmd, remaining, err := root.Find(strings.Fields(strings.TrimPrefix(path, "dws ")))
	if err != nil || cmd == nil || len(remaining) > 0 {
		return ""
	}
	for _, name := range flags {
		// Cobra installs --help lazily on command execution; it is valid for
		// every command even when Lookup has not materialized the flag yet.
		if name == "help" {
			continue
		}
		if cmd.Flags().Lookup(name) == nil && cmd.InheritedFlags().Lookup(name) == nil && cmd.PersistentFlags().Lookup(name) == nil {
			return fmt.Sprintf("flag --%s is not accepted by the command (run leaf --help)", name)
		}
	}
	return ""
}

func isPlaceholder(token string) bool {
	if len(token) < 2 {
		return false
	}
	first, last := token[0], token[len(token)-1]
	return (first == '<' && last == '>') || (first == '[' && last == ']')
}

func shellFields(input string) []string {
	var fields []string
	var current strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			fields = append(fields, current.String())
			current.Reset()
		}
	}
	for _, char := range input {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				current.WriteRune(char)
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == ' ' || char == '\t' {
			flush()
			continue
		}
		current.WriteRune(char)
	}
	flush()
	return fields
}

func uniqueSorted(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func formatFailure(root string, ref commandRef, reason string) string {
	relative, _ := filepath.Rel(root, ref.File)
	return fmt.Sprintf("%s:%d: `%s`: %s", relative, ref.Line, ref.Text, reason)
}
