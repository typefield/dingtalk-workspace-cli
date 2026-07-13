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

package cli

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const manualSchemaHintVersion = 1
const manualSchemaHintSchemaRef = "./schema_manual_hints.schema.json"

const (
	runtimeSchemaManualIdentityAnnotation  = "dws.schema.manual.identity"
	runtimeSchemaManualIdentityReason      = "dws.schema.manual.identity_reason"
	runtimeSchemaManualParameterAnnotation = "dws.schema.manual.parameter"
	runtimeSchemaManualReasonAnnotation    = "dws.schema.manual.reason"
)

//go:embed schema_manual_hints.json
var embeddedManualSchemaHintsJSON []byte

// ManualSchemaHintSnapshot is the human-owned bridge from an existing
// public Cobra leaf to Schema. It cannot create commands, flags, exclusions, or
// interfaces: every referenced command and flag is checked against the live
// tree before any annotation is applied.
type ManualSchemaHintSnapshot struct {
	Schema     string                    `json:"$schema"`
	Version    int                       `json:"version"`
	Commands   []ManualSchemaCommandHint `json:"commands"`
	AgentHints ManualAgentHintSet        `json:"agent_hints"`
}

// ManualAgentHintSet keeps reviewed Agent-selection prose in the same manual
// source as command and parameter overrides without making that prose an
// identity annotation. Generators may read this section; they must never write
// it back or use it to change executable, parameter, safety, or interface
// facts.
type ManualAgentHintSet struct {
	Revisions map[string]ManualAgentHintRevision `json:"revisions,omitempty"`
	Products  map[string]ManualAgentProductHint  `json:"products,omitempty"`
	Tools     map[string]ManualAgentToolHint     `json:"tools,omitempty"`
}

// ManualAgentHintRevision records how one reviewed authoring batch was
// produced. Product and tool entries reference it by the enclosing map key so
// later focused revisions do not relabel unchanged prose.
type ManualAgentHintRevision struct {
	GeneratedBy   string `json:"generated_by"`
	Model         string `json:"model,omitempty"`
	PromptVersion string `json:"prompt_version,omitempty"`
	Reason        string `json:"reason"`
}

// ManualAgentProductHint defines product-level routing prose.
type ManualAgentProductHint struct {
	AgentSummary string   `json:"agent_summary"`
	UseWhen      []string `json:"use_when"`
	AvoidWhen    []string `json:"avoid_when"`
	Reviewed     bool     `json:"reviewed"`
	Revision     string   `json:"revision"`
	Reason       string   `json:"reason"`
	Evidence     []string `json:"evidence"`
}

// ManualAgentToolHint defines the four reviewed fields used to select and
// demonstrate one existing effective command. It deliberately has no fields
// for identity, parameters, safety, or interface disposition.
type ManualAgentToolHint struct {
	AgentSummary string   `json:"agent_summary"`
	UseWhen      []string `json:"use_when"`
	AvoidWhen    []string `json:"avoid_when"`
	Examples     []string `json:"examples"`
	Reviewed     bool     `json:"reviewed"`
	Revision     string   `json:"revision"`
	Reason       string   `json:"reason"`
	Evidence     []string `json:"evidence"`
}

// ManualSchemaCommandHint opts one exact existing Cobra leaf into
// Schema and optionally reviews its CLI-facing parameter projection.
type ManualSchemaCommandHint struct {
	CLIPath       string                               `json:"cli_path"`
	CanonicalPath string                               `json:"canonical_path"`
	Reason        string                               `json:"reason"`
	Reviewed      bool                                 `json:"reviewed"`
	Parameters    map[string]ManualSchemaParameterHint `json:"parameters,omitempty"`
}

// ManualSchemaParameterHint changes only Schema annotations on a real
// flag. Pointer fields distinguish an omitted override from an explicit false.
type ManualSchemaParameterHint struct {
	Description   *string `json:"description,omitempty"`
	Property      *string `json:"property,omitempty"`
	InterfaceType *string `json:"interface_type,omitempty"`
	Required      *bool   `json:"required,omitempty"`
	RequiredWhen  *string `json:"required_when,omitempty"`
}

// ManualSchemaHintReport records the exact reviewed inputs applied to
// a command tree. It is useful to generators and tests; no runtime discovery is
// performed.
type ManualSchemaHintReport struct {
	Commands   []string
	Parameters []string
}

var (
	manualSchemaHintsOnce     sync.Once
	manualSchemaHintsSnapshot ManualSchemaHintSnapshot
	manualSchemaHintsErr      error
)

// ApplyEmbeddedManualSchemaHints applies the committed human review
// file to an already-built Cobra tree. The operation is deterministic and
// idempotent.
func ApplyEmbeddedManualSchemaHints(root *cobra.Command) (ManualSchemaHintReport, error) {
	snapshot, err := embeddedManualSchemaHints()
	if err != nil {
		return ManualSchemaHintReport{}, err
	}
	return applyManualSchemaHints(root, snapshot)
}

func embeddedManualSchemaHints() (ManualSchemaHintSnapshot, error) {
	manualSchemaHintsOnce.Do(func() {
		manualSchemaHintsSnapshot, manualSchemaHintsErr = decodeManualSchemaHints(embeddedManualSchemaHintsJSON)
	})
	return manualSchemaHintsSnapshot, manualSchemaHintsErr
}

func decodeManualSchemaHints(data []byte) (ManualSchemaHintSnapshot, error) {
	var snapshot ManualSchemaHintSnapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return snapshot, fmt.Errorf("decode manual Schema hints: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return snapshot, fmt.Errorf("decode manual Schema hints: %w", err)
	}
	if snapshot.Version != manualSchemaHintVersion {
		return snapshot, fmt.Errorf("unsupported manual Schema hint version %d", snapshot.Version)
	}
	if strings.TrimSpace(snapshot.Schema) != manualSchemaHintSchemaRef {
		return snapshot, fmt.Errorf("manual Schema hints must declare $schema=%q", manualSchemaHintSchemaRef)
	}
	if err := ValidateManualAgentHintSet(snapshot.AgentHints, nil, nil); err != nil {
		return snapshot, fmt.Errorf("validate manual Agent hints: %w", err)
	}
	return snapshot, nil
}

// DecodeManualSchemaHintSource strictly decodes the reviewed manual input for
// build-time generators and policy checks. Callers receive data only; this
// function never writes or normalizes the source file in place.
func DecodeManualSchemaHintSource(data []byte) (ManualSchemaHintSnapshot, error) {
	return decodeManualSchemaHints(data)
}

// ValidateManualAgentHintSet validates the reviewed Agent prose and, when
// expected sets are supplied, requires exact product and canonical-tool
// coverage. It is intentionally independent from generated Catalog data.
func ValidateManualAgentHintSet(hints ManualAgentHintSet, expectedProducts, expectedTools map[string]bool) error {
	if len(hints.Revisions) == 0 {
		return fmt.Errorf("agent_hints.revisions must not be empty")
	}
	for rawRevision, provenance := range hints.Revisions {
		revision := strings.TrimSpace(rawRevision)
		if revision == "" || revision != rawRevision {
			return fmt.Errorf("agent_hints has invalid revision key %q", rawRevision)
		}
		generatedBy := strings.TrimSpace(provenance.GeneratedBy)
		if generatedBy == "" || strings.TrimSpace(provenance.Reason) == "" {
			return fmt.Errorf("agent_hints revision %q requires generated_by and reason", revision)
		}
		if strings.EqualFold(generatedBy, "ai") && (strings.TrimSpace(provenance.Model) == "" || strings.TrimSpace(provenance.PromptVersion) == "") {
			return fmt.Errorf("AI agent_hints revision %q requires model and prompt_version", revision)
		}
	}

	for rawProductID, hint := range hints.Products {
		productID := strings.TrimSpace(rawProductID)
		if productID == "" || productID != rawProductID || strings.ContainsAny(productID, ". \t\r\n") {
			return fmt.Errorf("agent_hints has invalid product key %q", rawProductID)
		}
		if err := validateManualAgentHintFields("product "+productID, hint.AgentSummary, hint.UseWhen, hint.AvoidWhen, nil, hint.Reviewed, hint.Revision, hint.Reason, hint.Evidence, hints.Revisions); err != nil {
			return err
		}
	}
	for rawCanonical, hint := range hints.Tools {
		canonical := strings.TrimSpace(rawCanonical)
		if canonical != rawCanonical {
			return fmt.Errorf("agent_hints has invalid canonical tool key %q", rawCanonical)
		}
		if _, _, ok := splitManualSchemaCanonicalPath(canonical); !ok {
			return fmt.Errorf("agent_hints has invalid canonical tool key %q", rawCanonical)
		}
		if err := validateManualAgentHintFields("tool "+canonical, hint.AgentSummary, hint.UseWhen, hint.AvoidWhen, hint.Examples, hint.Reviewed, hint.Revision, hint.Reason, hint.Evidence, hints.Revisions); err != nil {
			return err
		}
		if len(hint.Examples) == 0 {
			return fmt.Errorf("agent_hints tool %s requires non-empty examples", canonical)
		}
		if len(hint.Examples) > 2 {
			return fmt.Errorf("agent_hints tool %s has %d examples; maximum is 2", canonical, len(hint.Examples))
		}
		for _, example := range hint.Examples {
			argv, err := tokenizeManualAgentExample(example)
			if err != nil {
				return fmt.Errorf("agent_hints tool %s example has invalid argv syntax: %w", canonical, err)
			}
			if len(argv) < 2 || argv[0] != "dws" {
				return fmt.Errorf("agent_hints tool %s example must start with dws: %q", canonical, example)
			}
			for _, argument := range argv[1:] {
				if argument == "--yes" || strings.HasPrefix(argument, "--yes=") {
					return fmt.Errorf("agent_hints tool %s example must not bypass confirmation with --yes", canonical)
				}
				if argument == "--help" || strings.HasPrefix(argument, "--help=") || argument == "-h" || strings.HasPrefix(argument, "-h=") {
					return fmt.Errorf("agent_hints tool %s example must demonstrate execution, not only --help", canonical)
				}
			}
		}
	}

	if err := validateManualAgentHintExactSet("products", expectedProducts, mapKeysManualAgentProducts(hints.Products)); err != nil {
		return err
	}
	if err := validateManualAgentHintExactSet("tools", expectedTools, mapKeysManualAgentTools(hints.Tools)); err != nil {
		return err
	}
	return nil
}

func validateManualAgentHintFields(scope, summary string, useWhen, avoidWhen, examples []string, reviewed bool, revision, reason string, evidence []string, revisions map[string]ManualAgentHintRevision) error {
	if !reviewed {
		return fmt.Errorf("agent_hints %s must be reviewed", scope)
	}
	revision = strings.TrimSpace(revision)
	if _, ok := revisions[revision]; revision == "" || !ok {
		return fmt.Errorf("agent_hints %s references unknown revision %q", scope, revision)
	}
	if strings.TrimSpace(summary) == "" || strings.TrimSpace(reason) == "" {
		return fmt.Errorf("agent_hints %s requires agent_summary and reason", scope)
	}
	for name, values := range map[string][]string{
		"use_when":   useWhen,
		"avoid_when": avoidWhen,
		"evidence":   evidence,
	} {
		if err := validateNonEmptyManualAgentStrings(scope, name, values); err != nil {
			return err
		}
	}
	if examples != nil {
		if err := validateNonEmptyManualAgentStrings(scope, "examples", examples); err != nil {
			return err
		}
	}
	return nil
}

func validateNonEmptyManualAgentStrings(scope, field string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("agent_hints %s requires non-empty %s", scope, field)
	}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("agent_hints %s has an empty %s entry", scope, field)
		}
		if seen[value] {
			return fmt.Errorf("agent_hints %s has duplicate %s entry %q", scope, field, value)
		}
		seen[value] = true
	}
	return nil
}

func validateManualAgentHintExactSet(scope string, expected, actual map[string]bool) error {
	if expected == nil {
		return nil
	}
	missing := make([]string, 0)
	unexpected := make([]string, 0)
	for key, include := range expected {
		if include && !actual[key] {
			missing = append(missing, key)
		}
	}
	for key := range actual {
		if !expected[key] {
			unexpected = append(unexpected, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	if len(missing) != 0 || len(unexpected) != 0 {
		return fmt.Errorf("agent_hints %s do not exactly match EffectiveCommandRegistry: missing=%v unexpected=%v", scope, missing, unexpected)
	}
	return nil
}

func mapKeysManualAgentProducts(values map[string]ManualAgentProductHint) map[string]bool {
	keys := make(map[string]bool, len(values))
	for key := range values {
		keys[key] = true
	}
	return keys
}

func mapKeysManualAgentTools(values map[string]ManualAgentToolHint) map[string]bool {
	keys := make(map[string]bool, len(values))
	for key := range values {
		keys[key] = true
	}
	return keys
}

// ValidateManualAgentHintExamples binds every authored example to the exact
// reviewed primary/alias path and rejects flags not accepted by the live Cobra
// command. It validates syntax only and never executes an example.
func ValidateManualAgentHintExamples(bound BoundCommandRegistry, hints ManualAgentHintSet) error {
	canonicalPaths := make([]string, 0, len(hints.Tools))
	for canonical := range hints.Tools {
		canonicalPaths = append(canonicalPaths, canonical)
	}
	sort.Strings(canonicalPaths)
	for _, canonical := range canonicalPaths {
		spec, ok := bound.ByCanonical[canonical]
		if !ok {
			return fmt.Errorf("agent_hints example references unknown canonical tool %q", canonical)
		}
		paths := []manualAgentExamplePath{{Path: spec.PrimaryCLIPath, Argv: strings.Fields(spec.PrimaryCLIPath), Command: spec.PrimaryCommand}}
		for _, alias := range spec.AliasCommands {
			paths = append(paths, manualAgentExamplePath{Path: alias.Path, Argv: strings.Fields(alias.Path), Command: alias.Command})
		}
		sort.Slice(paths, func(i, j int) bool {
			if len(paths[i].Argv) != len(paths[j].Argv) {
				return len(paths[i].Argv) > len(paths[j].Argv)
			}
			return paths[i].Path < paths[j].Path
		})
		for _, example := range hints.Tools[canonical].Examples {
			argv, err := tokenizeManualAgentExample(example)
			if err != nil {
				return fmt.Errorf("agent_hints tool %s example has invalid argv syntax: %w", canonical, err)
			}
			remainder, matched, ok := matchManualAgentExamplePath(argv, paths)
			if !ok {
				return fmt.Errorf("agent_hints tool %s example does not use its reviewed primary/alias path: %q", canonical, example)
			}
			if matched.Command == nil {
				return fmt.Errorf("agent_hints tool %s reviewed path %q has no bound Cobra command", canonical, matched.Path)
			}
			if err := validateManualAgentExampleFlags(matched.Command, remainder); err != nil {
				return fmt.Errorf("agent_hints tool %s example for %q: %w", canonical, matched.Path, err)
			}
		}
	}
	return nil
}

type manualAgentExamplePath struct {
	Path    string
	Argv    []string
	Command *cobra.Command
}

func matchManualAgentExamplePath(argv []string, paths []manualAgentExamplePath) ([]string, manualAgentExamplePath, bool) {
	if len(argv) == 0 || argv[0] != "dws" {
		return nil, manualAgentExamplePath{}, false
	}
	for index := range paths {
		if paths[index].Argv == nil {
			paths[index].Argv = strings.Fields(strings.TrimSpace(paths[index].Path))
		}
		pathArgv := paths[index].Argv
		if len(pathArgv) == 0 || len(argv) < len(pathArgv)+1 {
			continue
		}
		matches := true
		for offset := range pathArgv {
			if argv[offset+1] != pathArgv[offset] {
				matches = false
				break
			}
		}
		if matches {
			return argv[len(pathArgv)+1:], paths[index], true
		}
	}
	return nil, manualAgentExamplePath{}, false
}

// tokenizeManualAgentExample parses one deliberately small, shell-free argv
// grammar. Quotes and backslash escaping are supported for readable values,
// while shell control operators, expansion, redirection, and the "--"
// terminator fail closed. Angle-bracket placeholders are data tokens, not
// redirection operators.
func tokenizeManualAgentExample(input string) ([]string, error) {
	var (
		argv         []string
		current      strings.Builder
		quote        byte
		tokenStarted bool
	)
	flush := func() {
		if tokenStarted {
			argv = append(argv, current.String())
			current.Reset()
			tokenStarted = false
		}
	}
	for index := 0; index < len(input); index++ {
		character := input[index]
		if quote != 0 {
			if character == quote {
				quote = 0
				continue
			}
			if quote == '"' && character == '\\' {
				if index+1 >= len(input) {
					return nil, fmt.Errorf("trailing escape in double-quoted value")
				}
				index++
				current.WriteByte(input[index])
				continue
			}
			if quote == '"' && (character == '`' || character == '$') {
				return nil, fmt.Errorf("shell expansion is not allowed")
			}
			current.WriteByte(character)
			continue
		}

		switch character {
		case ' ', '\t':
			flush()
		case '\r', '\n':
			return nil, fmt.Errorf("unquoted newline shell operator is not allowed")
		case '\'', '"':
			quote = character
			tokenStarted = true
		case '\\':
			if index+1 >= len(input) {
				return nil, fmt.Errorf("trailing escape")
			}
			index++
			current.WriteByte(input[index])
			tokenStarted = true
		case '<':
			placeholder, next, ok := manualAgentPlaceholderAt(input, index)
			if !ok {
				return nil, fmt.Errorf("shell redirection operator %q is not allowed", string(character))
			}
			current.WriteString(placeholder)
			tokenStarted = true
			index = next
		case '>', ';', '|', '&', '(', ')':
			return nil, fmt.Errorf("shell operator %q is not allowed", string(character))
		case '`', '$':
			return nil, fmt.Errorf("shell expansion is not allowed")
		case '#':
			return nil, fmt.Errorf("shell comments are not allowed")
		default:
			current.WriteByte(character)
			tokenStarted = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quoted value")
	}
	flush()
	for _, argument := range argv {
		if argument == "--" {
			return nil, fmt.Errorf("the -- argument terminator is not allowed")
		}
	}
	return argv, nil
}

// ParseManualAgentExampleArgv exposes the same deliberately shell-free argv
// parser used by Manual Agent hint validation. Contract tests use it to run
// reviewed examples through Cobra without invoking a shell or maintaining a
// second parser with different escaping rules.
func ParseManualAgentExampleArgv(input string) ([]string, error) {
	return tokenizeManualAgentExample(input)
}

func manualAgentPlaceholderAt(input string, start int) (string, int, bool) {
	endOffset := strings.IndexByte(input[start+1:], '>')
	if endOffset < 1 {
		return "", start, false
	}
	end := start + 1 + endOffset
	body := input[start+1 : end]
	for index := 0; index < len(body); index++ {
		character := body[index]
		if strings.ContainsRune(" \t\r\n<>;&|`$()#'\"\\", rune(character)) {
			return "", start, false
		}
	}
	return input[start : end+1], end, true
}

func validateManualAgentExampleFlags(command *cobra.Command, arguments []string) error {
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			return fmt.Errorf("the -- argument terminator is not allowed")
		}
		if strings.HasPrefix(argument, "--") {
			nameAndValue := strings.TrimPrefix(argument, "--")
			name, _, hasValue := strings.Cut(nameAndValue, "=")
			if name == "" {
				return fmt.Errorf("invalid empty long flag")
			}
			if name == "help" {
				return fmt.Errorf("must demonstrate execution, not only --help")
			}
			flag := runtimeCommandFlag(command, name)
			if flag == nil {
				return fmt.Errorf("uses unknown flag --%s", name)
			}
			if !hasValue && flag.NoOptDefVal == "" {
				if index+1 >= len(arguments) {
					return fmt.Errorf("flag --%s requires a value", name)
				}
				index++
			}
			continue
		}
		if !strings.HasPrefix(argument, "-") || argument == "-" {
			continue
		}

		shorthandsAndValue := strings.TrimPrefix(argument, "-")
		shorthands, _, hasExplicitValue := strings.Cut(shorthandsAndValue, "=")
		if shorthands == "" {
			return fmt.Errorf("invalid empty shorthand flag")
		}
		for offset := 0; offset < len(shorthands); offset++ {
			shorthand := shorthands[offset : offset+1]
			if shorthand[0] >= 0x80 {
				return fmt.Errorf("uses invalid non-ASCII shorthand flag")
			}
			if shorthand == "h" {
				return fmt.Errorf("must demonstrate execution, not only -h")
			}
			flag := runtimeCommandFlagByShorthand(command, shorthand)
			if flag == nil {
				return fmt.Errorf("uses unknown shorthand flag -%s", shorthand)
			}
			if flag.NoOptDefVal == "" {
				if offset+1 < len(shorthands) || hasExplicitValue {
					break
				}
				if index+1 >= len(arguments) {
					return fmt.Errorf("shorthand flag -%s requires a value", shorthand)
				}
				index++
				break
			}
		}
	}
	return nil
}

func runtimeCommandFlagByShorthand(command *cobra.Command, shorthand string) *pflag.Flag {
	if command == nil || len(shorthand) != 1 {
		return nil
	}
	if flag := command.Flags().ShorthandLookup(shorthand); flag != nil {
		return flag
	}
	for current := command; current != nil; current = current.Parent() {
		if flag := current.PersistentFlags().ShorthandLookup(shorthand); flag != nil {
			return flag
		}
	}
	return nil
}

type validatedManualSchemaHint struct {
	hint    ManualSchemaCommandHint
	command *cobra.Command
}

func applyManualSchemaHints(root *cobra.Command, snapshot ManualSchemaHintSnapshot) (ManualSchemaHintReport, error) {
	if root == nil {
		return ManualSchemaHintReport{}, fmt.Errorf("apply manual Schema hints: root is nil")
	}
	if snapshot.Version != manualSchemaHintVersion {
		return ManualSchemaHintReport{}, fmt.Errorf("unsupported manual Schema hint version %d", snapshot.Version)
	}
	reviewedRegistry, err := loadEmbeddedCommandRegistry()
	if err != nil {
		return ManualSchemaHintReport{}, fmt.Errorf("load reviewed Schema command registry: %w", err)
	}

	validated := make([]validatedManualSchemaHint, 0, len(snapshot.Commands))
	seenPaths := map[string]bool{}
	for _, raw := range snapshot.Commands {
		hint := raw
		hint.CLIPath = normalizeSchemaCLIPath(hint.CLIPath)
		hint.CanonicalPath = strings.TrimSpace(hint.CanonicalPath)
		hint.Reason = strings.TrimSpace(hint.Reason)
		if hint.CLIPath == "" || strings.ContainsAny(hint.CLIPath, "*?[]") {
			return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint has invalid exact cli_path %q", raw.CLIPath)
		}
		if seenPaths[hint.CLIPath] {
			return ManualSchemaHintReport{}, fmt.Errorf("duplicate manual Schema hint for %q", hint.CLIPath)
		}
		seenPaths[hint.CLIPath] = true
		if !hint.Reviewed || hint.Reason == "" {
			return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q is not reviewed or has no reason", hint.CLIPath)
		}
		_, _, ok := splitManualSchemaCanonicalPath(hint.CanonicalPath)
		if !ok {
			return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q has invalid canonical_path %q", hint.CLIPath, hint.CanonicalPath)
		}
		match, resolveErr := resolveExactCobraPath(root, hint.CLIPath)
		if resolveErr != nil {
			return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q cannot be resolved exactly: %w", hint.CLIPath, resolveErr)
		}
		command := match.Command
		if command == nil {
			return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q does not resolve to an existing Cobra command", hint.CLIPath)
		}
		if !publicRunnableSchemaLeaf(command) {
			return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q must target a public runnable Cobra leaf", hint.CLIPath)
		}
		if productID, toolName, _ := runtimeSchemaAnnotations(command); productID != "" || toolName != "" {
			existing := strings.Trim(strings.TrimSpace(productID)+"."+strings.TrimSpace(toolName), ".")
			if existing != hint.CanonicalPath {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q canonical path %q conflicts with existing canonical path %q", hint.CLIPath, hint.CanonicalPath, existing)
			}
		}
		if identity, ok := reviewedRegistry.ByCLIPath[hint.CLIPath]; ok && identity.CanonicalPath != hint.CanonicalPath {
			return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q canonical path %q conflicts with reviewed CommandRegistry canonical path %q", hint.CLIPath, hint.CanonicalPath, identity.CanonicalPath)
		}
		if match.UsedAlias {
			namePath := normalizeSchemaCLIPath(strings.Join(commandPathParts(command), " "))
			identity, ok := reviewedRegistry.ByCLIPath[namePath]
			if !ok {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q uses a Cobra alias, but real command path %q is not present in reviewed CommandRegistry", hint.CLIPath, namePath)
			}
			if identity.CanonicalPath != hint.CanonicalPath {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q canonical path %q conflicts with real command path %q canonical path %q", hint.CLIPath, hint.CanonicalPath, namePath, identity.CanonicalPath)
			}
		}
		for flagName, parameter := range hint.Parameters {
			flagName = strings.TrimSpace(flagName)
			if flagName == "" {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q contains an empty flag name", hint.CLIPath)
			}
			if runtimeCommandFlag(command, flagName) == nil {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q references missing flag --%s", hint.CLIPath, flagName)
			}
			if parameter.Description == nil && parameter.Property == nil && parameter.InterfaceType == nil && parameter.Required == nil && parameter.RequiredWhen == nil {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q flag --%s has no Schema overrides", hint.CLIPath, flagName)
			}
			if parameter.Description != nil && strings.TrimSpace(*parameter.Description) == "" {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q flag --%s has an empty description override", hint.CLIPath, flagName)
			}
			if parameter.Property != nil && strings.TrimSpace(*parameter.Property) == "" {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q flag --%s has an empty property override", hint.CLIPath, flagName)
			}
			if parameter.InterfaceType != nil && !supportedManualSchemaInterfaceType(*parameter.InterfaceType) {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q flag --%s has unsupported interface_type %q", hint.CLIPath, flagName, *parameter.InterfaceType)
			}
			if parameter.RequiredWhen != nil && strings.TrimSpace(*parameter.RequiredWhen) == "" {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q flag --%s has an empty required_when override", hint.CLIPath, flagName)
			}
		}
		validated = append(validated, validatedManualSchemaHint{hint: hint, command: command})
	}

	report := ManualSchemaHintReport{}
	for _, item := range validated {
		annotateManualSchemaIdentity(item.command, item.hint.CanonicalPath, item.hint.Reason)
		report.Commands = append(report.Commands, item.hint.CLIPath)
		flagNames := make([]string, 0, len(item.hint.Parameters))
		for flagName := range item.hint.Parameters {
			flagNames = append(flagNames, flagName)
		}
		sort.Strings(flagNames)
		for _, flagName := range flagNames {
			parameter := item.hint.Parameters[flagName]
			flag := runtimeCommandFlag(item.command, flagName)
			if err := annotateManualSchemaParameter(flag, parameter, item.hint.Reason); err != nil {
				return ManualSchemaHintReport{}, fmt.Errorf("manual Schema hint %q flag --%s: %w", item.hint.CLIPath, flagName, err)
			}
			report.Parameters = append(report.Parameters, item.hint.CLIPath+" --"+flagName)
		}
	}
	sort.Strings(report.Commands)
	return report, nil
}

// annotateManualSchemaIdentity keeps reviewed implementation evidence in a
// source-owned annotation. It deliberately does not rewrite code-owned runtime
// annotations: EffectiveCommandRegistry owns the selected identity, and the
// Cobra binder requires every manual/native annotation to agree with it before
// retaining the evidence in provenance.
func annotateManualSchemaIdentity(cmd *cobra.Command, canonicalPath, reason string) {
	if cmd == nil {
		return
	}
	setRuntimeCommandAnnotation(cmd, runtimeSchemaManualIdentityAnnotation, strings.TrimSpace(canonicalPath))
	setRuntimeCommandAnnotation(cmd, runtimeSchemaManualIdentityReason, strings.TrimSpace(reason))
}

func runtimeManualSchemaIdentity(cmd *cobra.Command) (productID, toolName, reason string, ok bool) {
	if cmd == nil || cmd.Annotations == nil {
		return "", "", "", false
	}
	canonicalPath := strings.TrimSpace(cmd.Annotations[runtimeSchemaManualIdentityAnnotation])
	productID, toolName, ok = splitManualSchemaCanonicalPath(canonicalPath)
	if !ok {
		return "", "", "", false
	}
	reason = strings.TrimSpace(cmd.Annotations[runtimeSchemaManualIdentityReason])
	return productID, toolName, reason, true
}

// annotateManualSchemaParameter stores the reviewed value in a source-owned
// annotation instead of overwriting the annotations used by bindings, Cobra
// adapters, and code-owned runtime metadata. The renderer resolves these
// independent candidates field by field and can therefore retain provenance.
func annotateManualSchemaParameter(flag *pflag.Flag, hint ManualSchemaParameterHint, reason string) error {
	if flag == nil {
		return fmt.Errorf("flag is nil")
	}
	hint = normalizeManualSchemaParameterHint(hint)
	encoded, err := json.Marshal(hint)
	if err != nil {
		return fmt.Errorf("encode reviewed parameter: %w", err)
	}
	setFlagAnnotation(flag, runtimeSchemaManualParameterAnnotation, string(encoded))
	setFlagAnnotation(flag, runtimeSchemaManualReasonAnnotation, strings.TrimSpace(reason))
	return nil
}

func runtimeManualSchemaParameter(flag *pflag.Flag) (ManualSchemaParameterHint, string, bool, error) {
	raw := firstFlagAnnotation(flag, runtimeSchemaManualParameterAnnotation)
	if raw == "" {
		return ManualSchemaParameterHint{}, "", false, nil
	}
	var hint ManualSchemaParameterHint
	if err := json.Unmarshal([]byte(raw), &hint); err != nil {
		return ManualSchemaParameterHint{}, "", false, fmt.Errorf("decode reviewed manual parameter hint: %w", err)
	}
	return normalizeManualSchemaParameterHint(hint), firstFlagAnnotation(flag, runtimeSchemaManualReasonAnnotation), true, nil
}

func normalizeManualSchemaParameterHint(hint ManualSchemaParameterHint) ManualSchemaParameterHint {
	hint.Description = trimmedManualSchemaString(hint.Description)
	hint.Property = trimmedManualSchemaString(hint.Property)
	hint.InterfaceType = trimmedManualSchemaString(hint.InterfaceType)
	hint.RequiredWhen = trimmedManualSchemaString(hint.RequiredWhen)
	return hint
}

func trimmedManualSchemaString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func supportedManualSchemaInterfaceType(value string) bool {
	switch strings.TrimSpace(value) {
	case "string", "array", "object", "integer", "number", "boolean":
		return true
	default:
		return false
	}
}

func splitManualSchemaCanonicalPath(path string) (string, string, bool) {
	path = strings.TrimSpace(path)
	productID, toolName, ok := strings.Cut(path, ".")
	productID = strings.TrimSpace(productID)
	toolName = strings.TrimSpace(toolName)
	if !ok || productID == "" || toolName == "" || strings.ContainsAny(productID+toolName, " \t\r\n") {
		return "", "", false
	}
	return productID, toolName, true
}

func publicRunnableSchemaLeaf(command *cobra.Command) bool {
	if command == nil || !command.Runnable() || command.HasSubCommands() {
		return false
	}
	for current := command; current != nil; current = current.Parent() {
		if current.Hidden {
			return false
		}
	}
	return true
}
