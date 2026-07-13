// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package agentmetadata

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
)

var reviewedManualSelectionFields = map[string]bool{
	"agent_summary": true,
	"use_when":      true,
	"avoid_when":    true,
	"examples":      true,
}

// applyManualAgentHintSource selects the committed manual prose before lower
// precedence evidence is parsed. Later Skill, structured-hint, and MCP prose
// can therefore remain visible to audits without becoming a second delivery
// source. This function only reads the sourceFile already loaded by Generate.
func applyManualAgentHintSource(out *File, files map[string]sourceFile, opts Options, stats *Stats) error {
	if strings.TrimSpace(opts.ManualHintsPath) == "" {
		return nil
	}
	path := resolvePath(opts.Root, opts.ManualHintsPath)
	display := displayPath(opts.Root, path)
	file, ok := files[display]
	if !ok {
		return fmt.Errorf("manual Agent hint source %s was not loaded", display)
	}
	snapshot, err := cli.DecodeManualSchemaHintSource(file.data)
	if err != nil {
		return fmt.Errorf("decode manual Agent hint source %s: %w", display, err)
	}
	expectedTools := expectedCanonicalToolSet(opts)
	if err := cli.ValidateManualAgentHintSet(snapshot.AgentHints, opts.ProductIDs, expectedTools); err != nil {
		return fmt.Errorf("validate manual Agent hint source %s: %w", display, err)
	}
	if err := cli.ValidateManualAgentHintExamples(opts.BoundCommands, snapshot.AgentHints); err != nil {
		return fmt.Errorf("validate manual Agent hint examples from %s: %w", display, err)
	}
	if _, err := cli.ValidateManualAgentSelectionContract(opts.BoundCommands, snapshot.AgentHints); err != nil {
		return fmt.Errorf("validate manual Agent selection contract from %s: %w", display, err)
	}

	productIDs := make([]string, 0, len(snapshot.AgentHints.Products))
	for productID := range snapshot.AgentHints.Products {
		productIDs = append(productIDs, productID)
	}
	sort.Strings(productIDs)
	for _, productID := range productIDs {
		hint := snapshot.AgentHints.Products[productID]
		source := manualAgentHintSource(display, hint.Revision)
		metadata := out.Products[productID]
		metadata.AgentSummary = strings.TrimSpace(hint.AgentSummary)
		metadata.agentSummaryPresent = true
		metadata.agentSummaryRank = selectionRankReviewedManual
		metadata.agentSummaryOrigin = source
		metadata.AgentSummarySource = source
		metadata.UseWhen = normalizeAuthoredStrings(hint.UseWhen)
		metadata.useWhenPresent = true
		metadata.useWhenRank = selectionRankReviewedManual
		metadata.useWhenOrigin = source
		metadata.AvoidWhen = normalizeAuthoredStrings(hint.AvoidWhen)
		metadata.avoidWhenPresent = true
		metadata.avoidWhenRank = selectionRankReviewedManual
		metadata.avoidWhenOrigin = source
		recordProductStringCandidate(&metadata, "agent_summary", metadata.AgentSummary, true, selectionRankReviewedManual, source)
		recordProductListCandidate(&metadata, "use_when", metadata.UseWhen, true, selectionRankReviewedManual, source)
		recordProductListCandidate(&metadata, "avoid_when", metadata.AvoidWhen, true, selectionRankReviewedManual, source)
		metadata.SourceRefs = normalizedStrings(append(append(metadata.SourceRefs, display), hint.Evidence...))
		out.Products[productID] = metadata
		stats.HintProducts++
	}

	canonicalPaths := make([]string, 0, len(snapshot.AgentHints.Tools))
	for canonical := range snapshot.AgentHints.Tools {
		canonicalPaths = append(canonicalPaths, canonical)
	}
	sort.Strings(canonicalPaths)
	for _, canonical := range canonicalPaths {
		hint := snapshot.AgentHints.Tools[canonical]
		if opts.MaxExamples > 0 && len(hint.Examples) > opts.MaxExamples {
			return fmt.Errorf("manual Agent hint %s has %d reviewed examples, exceeding max-examples=%d", canonical, len(hint.Examples), opts.MaxExamples)
		}
		source := manualAgentHintSource(display, hint.Revision)
		metadata := out.Tools[canonical]
		metadata.AgentSummary = strings.TrimSpace(hint.AgentSummary)
		metadata.agentSummaryPresent = true
		metadata.agentSummaryRank = selectionRankReviewedManual
		metadata.agentSummaryOrigin = source
		metadata.AgentSummarySource = source
		metadata.UseWhen = normalizeAuthoredStrings(hint.UseWhen)
		metadata.useWhenPresent = true
		metadata.useWhenRank = selectionRankReviewedManual
		metadata.useWhenOrigin = source
		metadata.AvoidWhen = normalizeAuthoredStrings(hint.AvoidWhen)
		metadata.avoidWhenPresent = true
		metadata.avoidWhenRank = selectionRankReviewedManual
		metadata.avoidWhenOrigin = source
		metadata.Examples = normalizeAuthoredStrings(hint.Examples)
		metadata.examplesPresent = true
		metadata.examplesRank = selectionRankReviewedManual
		metadata.examplesOrigin = source
		for field, value := range map[string]any{
			"agent_summary": metadata.AgentSummary,
			"use_when":      metadata.UseWhen,
			"avoid_when":    metadata.AvoidWhen,
			"examples":      metadata.Examples,
		} {
			recordTypedFieldCandidateValue(&metadata, field, value, true, selectionRankReviewedManual, source, hint.Reason)
		}
		reviewed := true
		metadata.Reviewed = &reviewed
		metadata.reviewedRank = selectionRankReviewedManual
		metadata.reviewedOrigin = source
		recordTypedFieldCandidateValue(&metadata, "reviewed", true, true, selectionRankReviewedManual, source, hint.Reason)
		metadata.SourceRefs = normalizedStrings(append(append(metadata.SourceRefs, display), hint.Evidence...))
		out.Tools[canonical] = metadata
		stats.HintTools++
	}
	stats.HintFiles++
	return nil
}

func manualAgentHintSource(display, revision string) string {
	return strings.TrimSpace(display) + "#agent_hints@" + strings.TrimSpace(revision)
}

func expectedCanonicalToolSet(opts Options) map[string]bool {
	if len(opts.CanonicalToolPaths) > 0 {
		expected := make(map[string]bool, len(opts.CanonicalToolPaths))
		for canonical := range opts.CanonicalToolPaths {
			if canonical = strings.TrimSpace(canonical); canonical != "" {
				expected[canonical] = true
			}
		}
		return expected
	}
	if len(opts.ToolPaths) == 0 {
		return nil
	}
	expected := map[string]bool{}
	for path := range opts.ToolPaths {
		path = strings.TrimSpace(path)
		if !strings.ContainsAny(path, " \t") && strings.Contains(path, ".") {
			expected[path] = true
		}
	}
	return expected
}

// retainReviewedManualSelectionCandidates removes legacy prose from the
// precedence candidate set. The source references remain for evidence/audit,
// while only the versioned manual values can be delivered for these fields.
func retainReviewedManualSelectionCandidates(file *File) {
	if file == nil {
		return
	}
	for productID, metadata := range file.Products {
		for _, field := range []string{"agent_summary", "use_when", "avoid_when"} {
			metadata.fieldCandidates[field] = onlyReviewedManualCandidates(metadata.fieldCandidates[field])
		}
		file.Products[productID] = metadata
	}
	for path, metadata := range file.Tools {
		for field := range reviewedManualSelectionFields {
			metadata.fieldCandidates[field] = onlyReviewedManualCandidates(metadata.fieldCandidates[field])
		}
		file.Tools[path] = metadata
	}
}

func onlyReviewedManualCandidates(candidates []FieldCandidateProvenance) []FieldCandidateProvenance {
	selected := make([]FieldCandidateProvenance, 0, len(candidates))
	for _, candidate := range candidates {
		if precedenceRank(candidate.Precedence) == selectionRankReviewedManual {
			selected = append(selected, candidate)
		}
	}
	return selected
}

func validateReviewedManualSelectionDelivery(file File, opts Options) error {
	problems := []string{}
	productIDs := sortedBoolKeys(opts.ProductIDs)
	for _, productID := range productIDs {
		metadata, ok := file.Products[productID]
		if !ok {
			problems = append(problems, "missing product "+productID)
			continue
		}
		if strings.TrimSpace(metadata.AgentSummary) == "" || metadata.agentSummaryRank != selectionRankReviewedManual {
			problems = append(problems, productID+" agent_summary is not reviewed_manual")
		}
		if len(metadata.UseWhen) == 0 || metadata.useWhenRank != selectionRankReviewedManual {
			problems = append(problems, productID+" use_when is not reviewed_manual")
		}
		if len(metadata.AvoidWhen) == 0 || metadata.avoidWhenRank != selectionRankReviewedManual {
			problems = append(problems, productID+" avoid_when is not reviewed_manual")
		}
		for _, field := range []string{"agent_summary", "use_when", "avoid_when"} {
			provenance, ok := metadata.FieldProvenance[field]
			if !ok || provenance.Precedence != selectionPrecedenceReviewedManual || selectedCandidateCount(provenance.Candidates) != 1 {
				problems = append(problems, productID+" "+field+" provenance is not one reviewed_manual winner")
			}
		}
	}

	expected := expectedCanonicalToolPaths(opts)
	canonicalPaths := make([]string, 0, len(expected))
	for canonical := range expected {
		canonicalPaths = append(canonicalPaths, canonical)
	}
	sort.Strings(canonicalPaths)
	for _, canonical := range canonicalPaths {
		primary := expected[canonical]
		metadata, ok := file.Tools[primary]
		if !ok {
			problems = append(problems, "missing tool "+canonical)
			continue
		}
		checks := []struct {
			name  string
			valid bool
		}{
			{"agent_summary", strings.TrimSpace(metadata.AgentSummary) != "" && metadata.agentSummaryRank == selectionRankReviewedManual},
			{"use_when", len(metadata.UseWhen) > 0 && metadata.useWhenRank == selectionRankReviewedManual},
			{"avoid_when", len(metadata.AvoidWhen) > 0 && metadata.avoidWhenRank == selectionRankReviewedManual},
			{"examples", len(metadata.Examples) > 0 && metadata.examplesRank == selectionRankReviewedManual},
			{"reviewed", metadata.Reviewed != nil && *metadata.Reviewed && metadata.reviewedRank == selectionRankReviewedManual},
		}
		for _, check := range checks {
			if !check.valid {
				problems = append(problems, canonical+" "+check.name+" is not reviewed_manual")
			}
		}
		for _, field := range []string{"agent_summary", "use_when", "avoid_when", "examples"} {
			provenance, ok := metadata.FieldProvenance[field]
			if !ok || provenance.Precedence != selectionPrecedenceReviewedManual || selectedCandidateCount(provenance.Candidates) != 1 {
				problems = append(problems, canonical+" "+field+" provenance is not one reviewed_manual winner")
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("manual Agent selection delivery invariant failed: %s", strings.Join(problems, "; "))
}

func expectedCanonicalToolPaths(opts Options) map[string]string {
	if len(opts.CanonicalToolPaths) > 0 {
		result := make(map[string]string, len(opts.CanonicalToolPaths))
		for canonical, primary := range opts.CanonicalToolPaths {
			canonical = strings.TrimSpace(canonical)
			primary = normalizeCommandPath(primary)
			if canonical != "" && primary != "" {
				result[canonical] = primary
			}
		}
		return result
	}
	result := map[string]string{}
	for canonical, primary := range opts.ToolPaths {
		canonical = strings.TrimSpace(canonical)
		if !strings.ContainsAny(canonical, " \t") && strings.Contains(canonical, ".") {
			result[canonical] = normalizeCommandPath(primary)
		}
	}
	return result
}

func selectedCandidateCount(candidates []FieldCandidateProvenance) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.Selected {
			count++
		}
	}
	return count
}
