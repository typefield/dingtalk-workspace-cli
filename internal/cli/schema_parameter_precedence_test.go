// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cli

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeSchemaScalarResolverUsesSourceRankNotInputOrder(t *testing.T) {
	tests := []struct {
		name string
		high runtimeSchemaFieldCandidate
		low  runtimeSchemaFieldCandidate
		want any
	}{
		{
			name: "higher source can lower boolean",
			high: runtimeSchemaManualCandidate(false, true, "reviewed lowering"),
			low:  runtimeSchemaCandidate(true, true, "cobra_hard_required"),
			want: false,
		},
		{
			name: "higher source can raise boolean",
			high: runtimeSchemaCandidate(true, true, "native_annotation"),
			low:  runtimeSchemaCandidate(false, true, "tool_schema_hint"),
			want: true,
		},
		{
			name: "typed string beats hint string",
			high: runtimeSchemaCandidate("native", true, "native_annotation"),
			low:  runtimeSchemaCandidate("hint", true, "tool_schema_hint"),
			want: "native",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, candidates := range [][]runtimeSchemaFieldCandidate{{test.low, test.high}, {test.high, test.low}} {
				winner, err := resolveRuntimeSchemaCandidate("field", candidates...)
				if err != nil {
					t.Fatalf("resolveRuntimeSchemaCandidate() error = %v", err)
				}
				if !reflect.DeepEqual(winner.Value, test.want) {
					t.Fatalf("winner = %#v, want %#v", winner.Value, test.want)
				}
				if winner.Source != test.high.Source {
					t.Fatalf("winner source = %q, want %q", winner.Source, test.high.Source)
				}
			}
		})
	}
}

func TestRuntimeSchemaParameterSourcePrecedenceMatrix(t *testing.T) {
	tests := []struct {
		source     string
		rank       int
		precedence string
	}{
		{"reviewed_manual_hint", runtimeSchemaRankReviewedManual, runtimeSchemaPrecedenceReviewedManual},
		{"versioned_parameter_binding", runtimeSchemaRankVersionedBinding, runtimeSchemaPrecedenceVersionedBinding},
		{"require_one_of_constraint", runtimeSchemaRankConstraint, runtimeSchemaPrecedenceConstraint},
		{"typed_parameter_metadata", runtimeSchemaRankTypedMetadata, runtimeSchemaPrecedenceTypedMetadata},
		{"native_annotation", runtimeSchemaRankNativeAnnotation, runtimeSchemaPrecedenceNativeAnnotation},
		{"cobra_hard_required", runtimeSchemaRankCobraContract, runtimeSchemaPrecedenceCobra},
		{"tool_schema_hint", runtimeSchemaRankToolHint, runtimeSchemaPrecedenceToolHint},
		{"mcp_metadata", runtimeSchemaRankMCP, runtimeSchemaPrecedenceMCP},
		{"flag_name_inference", runtimeSchemaRankInference, runtimeSchemaPrecedenceInference},
		{"default", runtimeSchemaRankDefault, runtimeSchemaPrecedenceDefault},
	}
	for index, test := range tests {
		rank, precedence := runtimeSchemaSourcePriority(test.source)
		if rank != test.rank || precedence != test.precedence {
			t.Fatalf("source %q = rank %d/%q, want %d/%q", test.source, rank, precedence, test.rank, test.precedence)
		}
		if index > 0 && tests[index-1].rank <= test.rank {
			t.Fatalf("precedence matrix is not strictly descending at %q > %q", tests[index-1].source, test.source)
		}
	}
}

func TestRuntimeSchemaScalarResolverFailsEqualRankConflict(t *testing.T) {
	winner, err := resolveRuntimeSchemaCandidate("required",
		runtimeSchemaCandidate(false, true, "typed_parameter_metadata"),
		runtimeSchemaCandidate(false, true, "typed_parameter_metadata"),
	)
	if err != nil {
		t.Fatalf("equal values should coalesce: %v", err)
	}
	provenance := runtimeSchemaFieldProvenance(winner)
	selected := 0
	for _, candidate := range provenance.Candidates {
		if candidate.Selected != nil && *candidate.Selected {
			selected++
		}
	}
	if selected != 1 || len(provenance.Candidates) != 2 {
		t.Fatalf("coalesced provenance = %#v", provenance)
	}

	_, err = resolveRuntimeSchemaCandidate("required",
		runtimeSchemaCandidate(false, true, "typed_parameter_metadata"),
		runtimeSchemaCandidate(true, true, "typed_parameter_metadata"),
	)
	if err == nil || !strings.Contains(err.Error(), "conflicting equal-precedence") {
		t.Fatalf("equal-rank conflict error = %v", err)
	}
}

func TestRuntimeSchemaScalarResolverRejectsLowerRankConflictBehindWinner(t *testing.T) {
	high := runtimeSchemaManualCandidate(false, true, "reviewed winner")
	lowerA := runtimeSchemaStringCandidateAtRank(true, "lower-a", runtimeSchemaRankInference, runtimeSchemaPrecedenceInference)
	lowerB := runtimeSchemaStringCandidateAtRank(false, "lower-b", runtimeSchemaRankInference, runtimeSchemaPrecedenceInference)
	permutations := [][]runtimeSchemaFieldCandidate{
		{high, lowerA, lowerB},
		{high, lowerB, lowerA},
		{lowerA, high, lowerB},
		{lowerA, lowerB, high},
		{lowerB, high, lowerA},
		{lowerB, lowerA, high},
	}

	var stableError string
	for index, candidates := range permutations {
		_, err := resolveRuntimeSchemaCandidate("required", candidates...)
		if err == nil || !strings.Contains(err.Error(), "conflicting equal-precedence") ||
			!strings.Contains(err.Error(), "lower-a") || !strings.Contains(err.Error(), "lower-b") {
			t.Fatalf("permutation %d error = %v, want lower-rank conflict", index, err)
		}
		if stableError == "" {
			stableError = err.Error()
		} else if err.Error() != stableError {
			t.Fatalf("permutation %d error = %q, want stable %q", index, err, stableError)
		}
	}
}

func TestRuntimeCommandParameterSpecsBuildTypedContractDirectly(t *testing.T) {
	root, leaf := manualSchemaHintTestTree()
	leaf.Flags().Int("limit", 20, "Optional page size")
	if err := leaf.MarkFlagRequired("query"); err != nil {
		t.Fatalf("MarkFlagRequired() error = %v", err)
	}
	setFlagAnnotation(leaf.Flags().Lookup("query"), runtimeSchemaFlagExampleAnnotation, "hello")
	required := false
	_, err := applyManualSchemaHints(root, ManualSchemaHintSnapshot{
		Schema:  manualSchemaHintSchemaRef,
		Version: manualSchemaHintVersion,
		Commands: []ManualSchemaCommandHint{{
			CLIPath:       "sample item search",
			CanonicalPath: "sample.search_items",
			Reason:        "Review optional Agent projection",
			Reviewed:      true,
			Parameters: map[string]ManualSchemaParameterHint{
				"query": {Required: &required},
			},
		}},
	})
	if err != nil {
		t.Fatalf("applyManualSchemaHints() error = %v", err)
	}

	specs, err := runtimeCommandParameterSpecs(leaf, "sample.search_items", nil, map[string]embeddedMCPParamMeta{
		"limit": {Default: "50"},
	}, RuntimeSchemaConstraints{})
	if err != nil {
		t.Fatalf("runtimeCommandParameterSpecs() error = %v", err)
	}
	if got := []string{specs[0].Name, specs[1].Name}; !reflect.DeepEqual(got, []string{"limit", "query"}) {
		t.Fatalf("typed parameter order = %#v", got)
	}
	limit, query := specs[0], specs[1]
	if got := rawSchemaString(t, limit.Default); got != "20" {
		t.Fatalf("typed CLI default = %q", got)
	}
	if got := rawSchemaString(t, limit.InterfaceDefault); got != "50" {
		t.Fatalf("typed interface default = %q", got)
	}
	if query.Required || !query.CLIRequired {
		t.Fatalf("typed required projection = %v, CLIRequired = %v", query.Required, query.CLIRequired)
	}
	if got := rawSchemaString(t, query.Example); got != "hello" {
		t.Fatalf("typed example = %q", got)
	}
	provenance := query.FieldProvenance["required"]
	if provenance.Source != "reviewed_manual_hint" || provenance.ReviewReason != "Review optional Agent projection" {
		t.Fatalf("typed required provenance = %#v", provenance)
	}
	if !typedProvenanceHasCandidate(t, provenance, "cobra_hard_required", true) {
		t.Fatalf("typed provenance omitted Cobra observation: %#v", provenance)
	}
	for _, parameter := range []ParameterSpec{limit, query} {
		emptyRequiredWhen, ok := parameter.FieldProvenance["required_when"]
		if !ok || string(emptyRequiredWhen.Value) != `""` || emptyRequiredWhen.Source != "default" {
			t.Fatalf("%s empty required_when provenance = %#v", parameter.Name, emptyRequiredWhen)
		}
	}

	// The compatibility wrapper serializes the typed values and preserves the
	// historical JSON string representation of defaults/examples.
	payload, err := runtimeCommandParameters(leaf, "sample.search_items", nil, map[string]embeddedMCPParamMeta{
		"limit": {Default: "50"},
	}, RuntimeSchemaConstraints{})
	if err != nil {
		t.Fatalf("runtimeCommandParameters() error = %v", err)
	}
	limitPayload := schemaParameterEntry(t, payload, "limit")
	queryPayload := schemaParameterEntry(t, payload, "query")
	if limitPayload["default"] != "20" || limitPayload["interface_default"] != "50" {
		t.Fatalf("wire defaults = %#v", limitPayload)
	}
	if queryPayload["example"] != "hello" || queryPayload["required"] != false || queryPayload["cli_required"] != true {
		t.Fatalf("wire query = %#v", queryPayload)
	}
}

func rawSchemaString(t *testing.T, value json.RawMessage) string {
	t.Helper()
	var decoded string
	if err := json.Unmarshal(value, &decoded); err != nil {
		t.Fatalf("decode raw Schema string %q: %v", value, err)
	}
	return decoded
}

func typedProvenanceHasCandidate(t *testing.T, provenance FieldProvenance, source string, value bool) bool {
	t.Helper()
	for _, candidate := range provenance.Candidates {
		if candidate.Source != source {
			continue
		}
		var decoded bool
		if err := json.Unmarshal(candidate.Value, &decoded); err != nil {
			t.Fatalf("decode provenance candidate %q: %v", candidate.Value, err)
		}
		if decoded == value {
			return true
		}
	}
	return false
}

func TestRuntimeSchemaParameterPrecedenceManualWinsAllProjectedSources(t *testing.T) {
	root, leaf := manualSchemaHintTestTree()
	flag := leaf.Flags().Lookup("query")
	setFlagAnnotation(flag, runtimeSchemaFlagBindingPropertyAnnotation, "boundQuery")
	setFlagAnnotation(flag, runtimeSchemaFlagTypeAnnotation, "boolean")
	setFlagAnnotation(flag, runtimeSchemaFlagDescriptionAnnotation, "Native description")
	setFlagAnnotation(flag, runtimeSchemaFlagRequiredAnnotation, "false")
	setFlagAnnotation(flag, runtimeSchemaFlagRequiredWhenAnnotation, "native condition")
	setFlagAnnotation(flag, runtimeSchemaFlagMetadataRequiredAnnotation, "false")
	setFlagAnnotation(flag, runtimeSchemaFlagMetadataRequiredWhenAnnotation, "typed condition")

	description := "Reviewed description"
	property := "manualQuery"
	interfaceType := "object"
	required := true
	requiredWhen := "manual condition"
	_, err := applyManualSchemaHints(root, ManualSchemaHintSnapshot{
		Schema:  manualSchemaHintSchemaRef,
		Version: manualSchemaHintVersion,
		Commands: []ManualSchemaCommandHint{{
			CLIPath:       "sample item search",
			CanonicalPath: "sample.search_items",
			Reason:        "Reviewed conflict resolution",
			Reviewed:      true,
			Parameters: map[string]ManualSchemaParameterHint{
				"query": {
					Description:   &description,
					Property:      &property,
					InterfaceType: &interfaceType,
					Required:      &required,
					RequiredWhen:  &requiredWhen,
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("applyManualSchemaHints() error = %v", err)
	}

	parameters, err := runtimeCommandParameters(leaf, "sample.search_items", map[string]ParameterSchemaHint{
		"manualQuery": {
			Type:         "string",
			Description:  "Tool hint description",
			Required:     boolPointer(false),
			RequiredWhen: "tool hint condition",
		},
	}, map[string]embeddedMCPParamMeta{
		"manualQuery": {
			Type:         "array",
			Description:  "MCP description",
			Required:     boolPointer(false),
			RequiredWhen: "MCP condition",
		},
	}, RuntimeSchemaConstraints{})
	if err != nil {
		t.Fatalf("runtimeCommandParameters() error = %v", err)
	}
	query := schemaParameterEntry(t, parameters, "query")
	if query["description"] != description || query["property"] != property || query["interface_type"] != interfaceType || query["required"] != true || query["required_when"] != requiredWhen {
		t.Fatalf("resolved query = %#v", query)
	}
	for _, field := range []string{"description", "property", "interface_type", "required", "required_when"} {
		if source := schemaParameterFieldSource(t, query, field); source != "reviewed_manual_hint" {
			t.Fatalf("%s source = %q, want reviewed_manual_hint", field, source)
		}
	}
	if reason := schemaParameterFieldReviewReason(t, query, "property"); reason != "Reviewed conflict resolution" {
		t.Fatalf("property review_reason = %q", reason)
	}

	// Manual review is a separate resolver input; it must not erase the
	// independently owned annotations that it outranks.
	if got := firstFlagAnnotation(flag, runtimeSchemaFlagTypeAnnotation); got != "boolean" {
		t.Fatalf("runtime type annotation = %q", got)
	}
	if got := firstFlagAnnotation(flag, runtimeSchemaFlagRequiredWhenAnnotation); got != "native condition" {
		t.Fatalf("runtime required_when annotation = %q", got)
	}
}

func TestRuntimeSchemaParameterPrecedenceManualCanLowerCobraRequired(t *testing.T) {
	root, leaf := manualSchemaHintTestTree()
	if err := leaf.MarkFlagRequired("query"); err != nil {
		t.Fatalf("MarkFlagRequired() error = %v", err)
	}
	required := false
	_, err := applyManualSchemaHints(root, ManualSchemaHintSnapshot{
		Schema:  manualSchemaHintSchemaRef,
		Version: manualSchemaHintVersion,
		Commands: []ManualSchemaCommandHint{{
			CLIPath:       "sample item search",
			CanonicalPath: "sample.search_items",
			Reason:        "Review optional projection",
			Reviewed:      true,
			Parameters: map[string]ManualSchemaParameterHint{
				"query": {Required: &required},
			},
		}},
	})
	if err != nil {
		t.Fatalf("applyManualSchemaHints() error = %v", err)
	}
	parameters, err := runtimeCommandParameters(leaf, "sample.search_items", nil, nil, RuntimeSchemaConstraints{})
	if err != nil {
		t.Fatalf("runtimeCommandParameters() error = %v", err)
	}
	query := schemaParameterEntry(t, parameters, "query")
	if query["required"] != false {
		t.Fatalf("required = %#v, want false", query["required"])
	}
	if source := schemaParameterFieldSource(t, query, "required"); source != "reviewed_manual_hint" {
		t.Fatalf("required source = %q", source)
	}
	if query["cli_required"] != true {
		t.Fatalf("cli_required = %#v, want true", query["cli_required"])
	}
	if !schemaParameterFieldHasCandidate(t, query, "required", "cobra_hard_required", true) {
		t.Fatalf("required provenance does not retain Cobra observation: %#v", schemaParameterFieldProvenance(t, query, "required"))
	}
	if required, annotated := runtimeFlagRequiredState(leaf.Flags().Lookup("query")); !required || !annotated {
		t.Fatalf("runtimeFlagRequiredState() = %v, %v", required, annotated)
	}
}

func TestRuntimeSchemaParameterPrecedenceConstraintCanLowerCobraRequired(t *testing.T) {
	_, leaf := manualSchemaHintTestTree()
	leaf.Flags().String("scope", "", "Optional scope")
	if err := leaf.MarkFlagRequired("query"); err != nil {
		t.Fatalf("MarkFlagRequired() error = %v", err)
	}
	parameters, err := runtimeCommandParameters(leaf, "sample.search_items", nil, nil, RuntimeSchemaConstraints{
		RequireOneOf: [][]string{{"query", "scope"}},
	})
	if err != nil {
		t.Fatalf("runtimeCommandParameters() error = %v", err)
	}
	query := schemaParameterEntry(t, parameters, "query")
	if query["required"] != false || query["cli_required"] != true {
		t.Fatalf("resolved query = %#v", query)
	}
	if source := schemaParameterFieldSource(t, query, "required"); source != "require_one_of_constraint" {
		t.Fatalf("required source = %q", source)
	}
}

func TestRuntimeSchemaParameterPrecedenceBindingSelectsMCPMetadata(t *testing.T) {
	_, leaf := manualSchemaHintTestTree()
	canonical := "sample.search_items"
	applyRuntimeSchemaParameterBindingsFrom(leaf, canonical, map[string]map[string]string{
		canonical: {"query": "queryText"},
	})

	parameters, err := runtimeCommandParameters(leaf, canonical, nil, map[string]embeddedMCPParamMeta{
		"queryText": {
			Type:         "object",
			Description:  "MCP query object",
			Required:     boolPointer(true),
			RequiredWhen: "MCP condition",
		},
	}, RuntimeSchemaConstraints{})
	if err != nil {
		t.Fatalf("runtimeCommandParameters() error = %v", err)
	}
	query := schemaParameterEntry(t, parameters, "query")
	if query["property"] != "queryText" || query["interface_type"] != "object" || query["required"] != true || query["required_when"] != "MCP condition" {
		t.Fatalf("resolved query = %#v", query)
	}
	if source := schemaParameterFieldSource(t, query, "property"); source != "versioned_parameter_binding" {
		t.Fatalf("property source = %q", source)
	}
	if source := schemaParameterFieldSource(t, query, "interface_type"); source != "mcp_metadata" {
		t.Fatalf("interface_type source = %q", source)
	}
	if source := schemaParameterFieldSource(t, query, "required"); source != "mcp_metadata" {
		t.Fatalf("required source = %q", source)
	}
	if source := schemaParameterFieldSource(t, query, "required_when"); source != "mcp_metadata" {
		t.Fatalf("required_when source = %q", source)
	}
}

func schemaParameterFieldHasCandidate(t *testing.T, parameter map[string]any, field, source string, value any) bool {
	t.Helper()
	items, ok := schemaParameterFieldProvenance(t, parameter, field)["candidates"].([]map[string]any)
	if !ok {
		if raw, rawOK := schemaParameterFieldProvenance(t, parameter, field)["candidates"].([]any); rawOK {
			for _, candidate := range raw {
				item, _ := candidate.(map[string]any)
				if schemaString(item["source"]) == source && item["value"] == value {
					return true
				}
			}
		}
		return false
	}
	for _, item := range items {
		if schemaString(item["source"]) == source && item["value"] == value {
			return true
		}
	}
	return false
}

func TestRuntimeSchemaParameterPrecedenceTypedMetadataBeatsToolHint(t *testing.T) {
	_, leaf := manualSchemaHintTestTree()
	canonical := "sample.search_items"
	previous, existed := runtimeSchemaParameterMetadataByCanonical[canonical]
	runtimeSchemaParameterMetadataByCanonical[canonical] = RuntimeSchemaParameterMetadata{
		Required:     []string{"query"},
		RequiredWhen: map[string]string{"query": "typed condition"},
	}
	t.Cleanup(func() {
		if existed {
			runtimeSchemaParameterMetadataByCanonical[canonical] = previous
		} else {
			delete(runtimeSchemaParameterMetadataByCanonical, canonical)
		}
	})
	applyRuntimeSchemaParameterMetadata(leaf, canonical)

	parameters, err := runtimeCommandParameters(leaf, canonical, map[string]ParameterSchemaHint{
		"query": {Required: boolPointer(false), RequiredWhen: "tool hint condition"},
	}, nil, RuntimeSchemaConstraints{})
	if err != nil {
		t.Fatalf("runtimeCommandParameters() error = %v", err)
	}
	query := schemaParameterEntry(t, parameters, "query")
	if query["required"] != true || query["required_when"] != "typed condition" {
		t.Fatalf("resolved query = %#v", query)
	}
	if source := schemaParameterFieldSource(t, query, "required"); source != "typed_parameter_metadata" {
		t.Fatalf("required source = %q", source)
	}
	if source := schemaParameterFieldSource(t, query, "required_when"); source != "typed_parameter_metadata" {
		t.Fatalf("required_when source = %q", source)
	}
}

func TestRuntimeSchemaParameterPrecedenceNativeAndCobraBeatHints(t *testing.T) {
	_, leaf := manualSchemaHintTestTree()
	flag := leaf.Flags().Lookup("query")
	setFlagAnnotation(flag, runtimeSchemaFlagTypeAnnotation, "boolean")
	setFlagAnnotation(flag, runtimeSchemaFlagDescriptionAnnotation, "Native description")
	setFlagAnnotation(flag, runtimeSchemaFlagRequiredAnnotation, "false")

	parameters, err := runtimeCommandParameters(leaf, "sample.search_items", map[string]ParameterSchemaHint{
		"query": {
			Type:        "object",
			Description: "Tool hint description",
			Required:    boolPointer(true),
		},
	}, map[string]embeddedMCPParamMeta{
		"query": {
			Type:        "array",
			Description: "MCP description",
			Required:    boolPointer(true),
		},
	}, RuntimeSchemaConstraints{})
	if err != nil {
		t.Fatalf("runtimeCommandParameters() error = %v", err)
	}
	query := schemaParameterEntry(t, parameters, "query")
	if query["interface_type"] != "boolean" || query["description"] != "Native description" || query["required"] != false {
		t.Fatalf("resolved query = %#v", query)
	}
	for _, field := range []string{"interface_type", "description", "required"} {
		if source := schemaParameterFieldSource(t, query, field); source != "native_annotation" {
			t.Fatalf("%s source = %q, want native_annotation", field, source)
		}
	}

	// Without a native required override, the executable Cobra contract wins
	// over descriptive Tool/MCP hints, while CLIRequired remains independently
	// observable in the final projection.
	flag.Annotations[runtimeSchemaFlagRequiredAnnotation] = nil
	if err := leaf.MarkFlagRequired("query"); err != nil {
		t.Fatalf("MarkFlagRequired() error = %v", err)
	}
	parameters, err = runtimeCommandParameters(leaf, "sample.search_items", map[string]ParameterSchemaHint{
		"query": {Required: boolPointer(false)},
	}, map[string]embeddedMCPParamMeta{
		"query": {Required: boolPointer(false)},
	}, RuntimeSchemaConstraints{})
	if err != nil {
		t.Fatalf("runtimeCommandParameters() error = %v", err)
	}
	query = schemaParameterEntry(t, parameters, "query")
	if query["required"] != true || query["cli_required"] != true {
		t.Fatalf("Cobra-required query = %#v", query)
	}
	if source := schemaParameterFieldSource(t, query, "required"); source != "cobra_hard_required" {
		t.Fatalf("required source = %q, want cobra_hard_required", source)
	}
}

func TestRuntimeSchemaParameterPrecedenceToolHintBeatsMCPWithoutChangingFlagIdentity(t *testing.T) {
	_, leaf := manualSchemaHintTestTree()
	parameters, err := runtimeCommandParameters(leaf, "sample.search_items", map[string]ParameterSchemaHint{
		"query": {
			FlagName:     "query",
			Type:         "object",
			Description:  "Tool hint description",
			Required:     boolPointer(true),
			RequiredWhen: "tool condition",
		},
	}, map[string]embeddedMCPParamMeta{
		"query": {
			Type:         "array",
			Description:  "MCP description",
			Required:     boolPointer(false),
			RequiredWhen: "MCP condition",
		},
	}, RuntimeSchemaConstraints{})
	if err != nil {
		t.Fatalf("runtimeCommandParameters() error = %v", err)
	}
	query := schemaParameterEntry(t, parameters, "query")
	if query["interface_type"] != "object" || query["required"] != true || query["required_when"] != "tool condition" {
		t.Fatalf("resolved query = %#v", query)
	}
	// Cobra Help owns the CLI description and outranks descriptive hints.
	if query["description"] != "Original query text" {
		t.Fatalf("description = %#v, want Cobra usage", query["description"])
	}
	for _, field := range []string{"interface_type", "required", "required_when"} {
		if source := schemaParameterFieldSource(t, query, field); source != "tool_schema_hint" {
			t.Fatalf("%s source = %q, want tool_schema_hint", field, source)
		}
	}

	_, err = runtimeCommandParameters(leaf, "sample.search_items", map[string]ParameterSchemaHint{
		"query": {FlagName: "renamed-query"},
	}, nil, RuntimeSchemaConstraints{})
	if err == nil || !strings.Contains(err.Error(), "does not identify the existing Cobra flag") {
		t.Fatalf("phantom flag rename error = %v", err)
	}
}

func schemaParameterEntry(t *testing.T, parameters map[string]any, name string) map[string]any {
	t.Helper()
	entry, ok := parameters[name].(map[string]any)
	if !ok {
		t.Fatalf("parameter %q = %#v", name, parameters[name])
	}
	return entry
}

func schemaParameterFieldSource(t *testing.T, parameter map[string]any, field string) string {
	t.Helper()
	return schemaString(schemaParameterFieldProvenance(t, parameter, field)["source"])
}

func schemaParameterFieldReviewReason(t *testing.T, parameter map[string]any, field string) string {
	t.Helper()
	return schemaString(schemaParameterFieldProvenance(t, parameter, field)["review_reason"])
}

func schemaParameterFieldProvenance(t *testing.T, parameter map[string]any, field string) map[string]any {
	t.Helper()
	provenance, ok := parameter["field_provenance"].(map[string]any)
	if !ok {
		t.Fatalf("field_provenance = %#v", parameter["field_provenance"])
	}
	fieldProvenance, ok := provenance[field].(map[string]any)
	if !ok {
		t.Fatalf("field provenance for %q = %#v", field, provenance[field])
	}
	return fieldProvenance
}
