// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package schemaruntime

import (
	"errors"
	"reflect"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

func TestCrossPlatformCoverageCompactWrappersAndCollectionEdges(t *testing.T) {
	if Compact(nil) != nil {
		t.Fatal("Compact(nil)")
	}
	payload := Compact(map[string]any{
		"description": "keep",
		"provenance":  "drop",
		"product":     "raw",
		"products":    "not-a-collection",
		"tools": []any{
			map[string]any{"description": "leaf", "property": "drop"},
			"skip",
		},
		"parameters": map[string]any{
			"raw":   "value",
			"typed": map[string]any{"type": "string", "property": "remote"},
		},
	})
	if _, exists := payload["provenance"]; exists || payload["product"] != "raw" || payload["products"] != "not-a-collection" {
		t.Fatalf("compact payload = %#v", payload)
	}
	nested := Compact(map[string]any{
		"product": map[string]any{"description": "child", "property": "drop"},
		"products": []map[string]any{
			{"description": "one", "property": "drop"},
		},
	})
	product, _ := nested["product"].(map[string]any)
	if _, exists := product["property"]; exists || product["description"] != "child" {
		t.Fatalf("nested product = %#v", nested["product"])
	}

	if got := CompactCollection("raw"); got != "raw" {
		t.Fatalf("CompactCollection raw = %#v", got)
	}
	if got := CompactParameters("raw"); got != "raw" {
		t.Fatalf("CompactParameters raw = %#v", got)
	}
	if got := CompactValue(true); got != true {
		t.Fatalf("CompactValue scalar = %#v", got)
	}
	if got := CompactValue(map[string]any{"type": "string", "property": "drop"}).(map[string]any); got["type"] != "string" {
		t.Fatalf("CompactValue param = %#v", got)
	}
	if _, exists := CompactValue(map[string]any{"description": "keep", "property": "drop"}).(map[string]any)["property"]; exists {
		t.Fatal("CompactValue payload leaked a dropped key")
	}
	if got := CompactValue([]map[string]any{{"description": "keep", "property": "drop"}}).([]map[string]any); got[0]["description"] != "keep" {
		t.Fatalf("CompactValue typed collection = %#v", got)
	}
	if got := CompactValue([]any{map[string]any{"required": true, "property": "drop"}}).([]any); got[0].(map[string]any)["required"] != true {
		t.Fatalf("CompactValue any collection = %#v", got)
	}
	parameter := CompactParameter(map[string]any{"type": "string", "property": "remote"})
	if _, exists := parameter["property"]; exists || parameter["type"] != "string" {
		t.Fatalf("CompactParameter = %#v", parameter)
	}
}

func TestCrossPlatformCoverageQueryProjectorsErrorsAndHelpers(t *testing.T) {
	tool := ToolSpec{Identity: contract.ToolIdentitySpec{
		ProductID:      "sample",
		Name:           "run",
		CanonicalPath:  "sample.run",
		CLIPath:        "sample group run",
		PrimaryCLIPath: "sample group run",
		Aliases:        []string{"sample legacy run"},
	}}
	registry, err := SchemaRegistryFromRuntime(" ", []ProductSpec{{ID: "sample", Tools: []ToolSpec{tool}}})
	if err != nil {
		t.Fatal(err)
	}
	index, err := registry.Index()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := RenderQueryWithProjectors(registry, index, "sample", QueryProjectors{
		ProductSummary: func(ProductSpec) (map[string]any, error) { return nil, errors.New("product boom") },
	}); err == nil || err.Error() != "product boom" {
		t.Fatalf("product projector = %v", err)
	}
	if _, err := RenderQueryWithProjectors(registry, index, "sample group", QueryProjectors{
		ToolSummary: func(ToolSpec) (map[string]any, error) { return nil, errors.New("tool boom") },
	}); err == nil || err.Error() != "tool boom" {
		t.Fatalf("tool projector = %v", err)
	}

	aliased := AliasView(tool, "sample legacy run")
	if !aliased.Identity.IsAlias || aliased.Identity.CLIPath != "sample legacy run" {
		t.Fatalf("AliasView = %#v", aliased.Identity)
	}
	if AliasView(tool, "sample group run").Identity.IsAlias {
		t.Fatal("canonical query must not mark alias")
	}
	if AliasView(tool, "unrelated").Identity.IsAlias {
		t.Fatal("unrelated query must not mark alias")
	}
	if ToolUnderGroup(tool, "mail") {
		t.Fatal("unrelated group matched")
	}
	if !ToolUnderGroup(tool, "sample group") {
		t.Fatal("primary group missed")
	}

	if got := SplitPathTokens(" sample.. / run\t "); !reflect.DeepEqual(got, []string{"sample", "run"}) {
		t.Fatalf("SplitPathTokens = %#v", got)
	}
	if normalizeSchemaCLIPath("dws") != "" || normalizeSchemaQueryCLIPath("dws/sample/run") != "sample run" {
		t.Fatal("schema path wrappers")
	}

	payload := map[string]any{"kind": "schema"}
	StampTrustedHashes(payload, TrustedHashes{CatalogHash: "catalog"})
	if payload["catalog_hash"] != "catalog" {
		t.Fatalf("catalog hash = %#v", payload)
	}
	if _, exists := payload["surface_hash"]; exists {
		t.Fatal("empty surface hash must omit")
	}
	StampTrustedHashes(payload, TrustedHashes{CatalogHash: "catalog", SurfaceHash: "surface"})
	if payload["surface_hash"] != "surface" {
		t.Fatal("surface hash")
	}
	if sourceOrDefault(" ") != runtimeAssembledSource || sourceOrDefault("live") != "live" {
		t.Fatal("sourceOrDefault")
	}
	if quote(`a"b`) != `"a\"b"` {
		t.Fatalf("quote = %s", quote(`a"b`))
	}
}
