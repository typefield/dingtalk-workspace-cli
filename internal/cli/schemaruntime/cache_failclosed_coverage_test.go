// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package schemaruntime

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/schemacachepb"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

func TestCrossPlatformCoverageSchemaCacheCodecFailClosedEdges(t *testing.T) {
	if _, err := (SchemaOverview{Products: []OverviewProduct{{
		ID: "drive", SummaryKind: OverviewSummaryNone, Summary: "stale",
	}}}).ToPayload(); err == nil {
		t.Fatal("summary without kind")
	}
	if _, err := (SchemaOverview{Products: []OverviewProduct{{
		ID: "drive", SummaryKind: OverviewSummaryKind("unknown"),
	}}}).ToPayload(); err == nil {
		t.Fatal("unknown summary kind")
	}
	if _, err := (SchemaOverview{Products: []OverviewProduct{{
		ID: "mail", SummaryKind: OverviewSummaryUseWhen, Summary: "when",
	}}, AgentMetadata: json.RawMessage(`{`)}).ToPayload(); err == nil {
		t.Fatal("invalid agent metadata")
	}
	if _, ok := (DecodedSchemaMeta{}).CommandMeta("missing"); ok {
		t.Fatal("empty meta lookup")
	}
	meta := DecodedSchemaMeta{LocatorProductByPath: map[string]string{}}
	meta.MaterializeCommandMeta()
	if _, ok := metaPayloadDescriptor(DecodedSchemaMeta{}, "drive"); ok {
		t.Fatal("missing payload descriptor")
	}
	if _, _, err := splitCommandPayloadShard(nil); err == nil {
		t.Fatal("short shard")
	}
	if _, err := assembleCommandPayloadShard(nil, nil); err == nil {
		t.Fatal("empty header shard")
	}
	if _, err := MarshalSchemaCacheDeterministic(nil); err == nil {
		t.Fatal("nil proto")
	}
	if _, err := DecodeSchemaMetaCache([]byte("not-proto")); err == nil {
		t.Fatal("invalid meta cache")
	}
	if _, err := DecodeSchemaPayloadIndex(nil); err == nil {
		t.Fatal("empty payload index")
	}
	if _, _, err := ProductShardBounds(ProductDescriptor{}, 1); err == nil {
		t.Fatal("zero-length shard")
	}
	if err := AuthenticateCommandPayloadDescriptor(DecodedSchemaMeta{}, CommandPayloadDescriptor{ProductID: "drive"}); err == nil {
		t.Fatal("unauthenticated descriptor")
	}
	if _, err := DecodeSchemaCommandPayloadHeader(nil, CommandPayloadDescriptor{}); err == nil {
		t.Fatal("empty command payload header")
	}
	if _, err := DecodeSchemaCommandPayloadCache(nil, CommandPayloadDescriptor{}, DecodedSchemaMeta{}); err == nil {
		t.Fatal("empty command payload cache")
	}
	if _, err := DecodeSchemaProductFromShards(nil, DecodedSchemaMeta{}, "drive"); err == nil {
		t.Fatal("empty product shards")
	}
	if _, _, err := DecodeAllSchemaProducts(nil, DecodedSchemaMeta{}); err == nil {
		t.Fatal("empty all products")
	}
	if _, err := BuildSchemaOverview(SchemaRegistry{Products: []ProductSpec{{}}}); err == nil {
		t.Fatal("empty overview registry")
	}
	if _, err := BuildSchemaProductLocators(SchemaRegistry{Products: []ProductSpec{{}}}); err == nil {
		t.Fatal("empty locator registry")
	}
	if _, err := BuildSchemaCache(SchemaRegistry{}, nil, SchemaOverview{}, nil, CacheHashes{}, nil); err == nil {
		t.Fatal("empty schema cache build")
	}
	if err := validateRenderedSchemaLeaves(nil, map[string][]byte{"x": {}}); err == nil {
		t.Fatal("rendered leaf without lookup")
	}
	if err := validateRenderedSchemaLeaves(map[string]CommandMeta{"x": {}}, map[string][]byte{"x": []byte("{}\n"), "y": []byte("{}\n")}); err == nil {
		t.Fatal("extra rendered leaf")
	}

	if _, err := RenderAll(SchemaRegistry{Products: []ProductSpec{{}}}, TrustedHashes{}); err == nil {
		t.Fatal("invalid render all")
	}
	if _, err := RenderOverview(SchemaRegistry{Products: []ProductSpec{{}}}, TrustedHashes{}); err == nil {
		t.Fatal("invalid render overview")
	}
	if _, err := RenderCatalog(SchemaRegistry{Products: []ProductSpec{{}}}, TrustedHashes{}); err == nil {
		t.Fatal("invalid render catalog")
	}

	if _, _, err := splitCommandPayloadShard([]byte{0, 0, 0, 0}); err == nil {
		t.Fatal("zero header length")
	}
	if _, _, err := splitCommandPayloadShard([]byte{0, 0, 0, 10, 1, 2, 3}); err == nil {
		t.Fatal("header length overflow")
	}
	if _, err := DecodeSchemaPayloadIndex([]byte{0, 0, 0, 1, 0x00, 0xff}); err == nil {
		t.Fatal("payload index trailing bytes")
	}
	if _, err := DecodeSchemaPayloadIndex([]byte{0, 0, 0, 3, 0xff, 0xff, 0xff}); err == nil {
		t.Fatal("invalid payload index proto")
	}
	if _, err := DecodeSchemaMetaCache(make([]byte, MaxSchemaMetaBytes+1)); err == nil {
		t.Fatal("oversized meta")
	}
	if _, _, err := ProductShardBounds(ProductDescriptor{ProductID: "drive", Length: 1}, 0); err == nil {
		t.Fatal("range exceeds total")
	}
	if _, _, err := ProductShardBounds(ProductDescriptor{ProductID: "drive", Offset: uint64(math.MaxInt64) + 1, Length: 1}, math.MaxUint64); err == nil {
		t.Fatal("unrepresentable range")
	}
	if _, err := DecodeSchemaProductCache([]byte("x"), ProductDescriptor{ProductID: "drive", Length: 1, SHA256: sha256.Sum256([]byte("x"))}, DecodedSchemaMeta{}); err == nil {
		t.Fatal("unauthenticated product shard")
	}
	if _, err := DecodeSchemaCommandPayloadHeader([]byte("xx"), CommandPayloadDescriptor{HeaderLength: 1}); err == nil {
		t.Fatal("header length mismatch")
	}
	if _, err := DecodeSchemaCommandPayloadHeader([]byte("abcd"), CommandPayloadDescriptor{HeaderLength: 4, HeaderSHA256: [32]byte{1}}); err == nil {
		t.Fatal("header digest mismatch")
	}
	desc := CommandPayloadDescriptor{ProductID: "drive", Length: 4, SHA256: sha256.Sum256([]byte("abcd")), HeaderLength: 4, HeaderSHA256: sha256.Sum256([]byte("abcd"))}
	payloadMeta := DecodedSchemaMeta{PayloadDescriptors: []CommandPayloadDescriptor{desc}}
	if _, err := DecodeSchemaCommandPayloadCache([]byte("ab"), desc, payloadMeta); err == nil {
		t.Fatal("payload length mismatch")
	}
	desc.SHA256 = [32]byte{1}
	payloadMeta.PayloadDescriptors[0] = desc
	if _, err := DecodeSchemaCommandPayloadCache([]byte("abcd"), desc, payloadMeta); err == nil {
		t.Fatal("payload digest mismatch")
	}
}

func TestCrossPlatformCoverageSchemaCacheConversionNilAndUnknownEnums(t *testing.T) {
	if overviewSummaryKindToProto(OverviewSummaryUseWhen) == schemacachepb.OverviewSummaryKind_OVERVIEW_SUMMARY_KIND_UNSPECIFIED {
		t.Fatal("use_when proto")
	}
	if overviewSummaryKindToProto(OverviewSummaryDescription) == schemacachepb.OverviewSummaryKind_OVERVIEW_SUMMARY_KIND_UNSPECIFIED {
		t.Fatal("description proto")
	}
	if overviewSummaryKindToProto(OverviewSummaryKind("nope")) != schemacachepb.OverviewSummaryKind_OVERVIEW_SUMMARY_KIND_UNSPECIFIED {
		t.Fatal("unknown overview kind")
	}
	if overviewSummaryKindFromProto(schemacachepb.OverviewSummaryKind_OVERVIEW_SUMMARY_KIND_USE_WHEN) != OverviewSummaryUseWhen {
		t.Fatal("use_when from proto")
	}
	if overviewSummaryKindFromProto(schemacachepb.OverviewSummaryKind_OVERVIEW_SUMMARY_KIND_DESCRIPTION) != OverviewSummaryDescription {
		t.Fatal("description from proto")
	}
	if overviewSummaryKindFromProto(schemacachepb.OverviewSummaryKind(99)) != OverviewSummaryNone {
		t.Fatal("unknown overview from proto")
	}
	if descriptorsFromProto(nil) != nil || payloadDescriptorsFromProto(nil) != nil || toolsFromProto(nil) != nil {
		t.Fatal("nil list from proto")
	}
	if constraintsFromProto(nil).RequireOneOf != nil || interfaceFromProto(nil).Ref != nil || selectionFromProto(nil).UseWhen != nil {
		t.Fatal("nil nested from proto")
	}
	if provenanceFromProto(nil) != nil || intToProto(nil) != nil || intFromProto(nil) != nil {
		t.Fatal("nil scalar from proto")
	}
	if got, err := toolsToProto(nil); got != nil || err != nil {
		t.Fatal("nil tools to proto")
	}
	if _, err := productToProto(ProductSpec{Tools: []ToolSpec{{Result: &contract.ResultSpec{Outcomes: []contract.ResultOutcome{"nope"}}}}}); err == nil {
		t.Fatal("invalid result outcome")
	}
	if _, err := productToProto(ProductSpec{Selection: contract.SelectionSpec{ExampleDispositions: []contract.ExampleDisposition{{Mode: "nope"}}}}); err == nil {
		t.Fatal("invalid product selection")
	}
	if _, ok := resultOutcomeToProto(contract.ResultOutcome("nope")); ok {
		t.Fatal("unknown result outcome")
	}
	if resultOutcomeFromProto(schemacachepb.ResultOutcome(99)) != "" {
		t.Fatal("unknown result from proto")
	}
	if _, ok := dispositionModeToProto(contract.ExampleDispositionModeContract); !ok {
		t.Fatal("contract mode")
	}
	if _, ok := dispositionModeToProto(contract.ExampleDispositionModeDryRun); !ok {
		t.Fatal("dry-run mode")
	}
	if _, ok := dispositionModeToProto("nope"); ok {
		t.Fatal("unknown mode")
	}
	if dispositionModeFromProto(schemacachepb.ExampleDispositionMode_EXAMPLE_DISPOSITION_MODE_CONTRACT) == "" {
		t.Fatal("contract from proto")
	}
	if dispositionModeFromProto(schemacachepb.ExampleDispositionMode_EXAMPLE_DISPOSITION_MODE_DRY_RUN) == "" {
		t.Fatal("dry-run from proto")
	}
	if dispositionModeFromProto(schemacachepb.ExampleDispositionMode(99)) != "" {
		t.Fatal("unknown mode from proto")
	}
	if _, ok := dispositionReasonToProto(contract.ExampleDispositionReasonStatefulPreflight); !ok {
		t.Fatal("stateful reason")
	}
	if _, ok := dispositionReasonToProto("nope"); ok {
		t.Fatal("unknown reason")
	}
	if dispositionReasonFromProto(schemacachepb.ExampleDispositionReasonCode_EXAMPLE_DISPOSITION_REASON_CODE_STATEFUL_PREFLIGHT) == "" {
		t.Fatal("stateful from proto")
	}
	if dispositionReasonFromProto(schemacachepb.ExampleDispositionReasonCode(99)) != "" {
		t.Fatal("unknown reason from proto")
	}
	if _, err := dispositionsToProto([]contract.ExampleDisposition{{Mode: "nope"}}); err == nil {
		t.Fatal("invalid disposition mode")
	}
	if _, err := dispositionsToProto([]contract.ExampleDisposition{{Mode: contract.ExampleDispositionModeContractOnly, ReasonCode: "nope"}}); err == nil {
		t.Fatal("invalid disposition reason")
	}
	if _, err := selectionToProtoExact(contract.SelectionSpec{ExampleDispositions: []contract.ExampleDisposition{{Mode: "nope"}}}); err == nil {
		t.Fatal("invalid selection disposition")
	}
	cloned := cloneProductExact(ProductSpec{Tools: []ToolSpec{
		{Identity: contract.ToolIdentitySpec{CanonicalPath: "b", CLIPath: "b"}},
		{Identity: contract.ToolIdentitySpec{CanonicalPath: "a", CLIPath: "z"}},
		{Identity: contract.ToolIdentitySpec{CanonicalPath: "a", CLIPath: "a"}},
	}})
	if cloned.Tools[0].Identity.CLIPath != "a" || cloned.Tools[1].Identity.CLIPath != "z" {
		t.Fatalf("sorted clone = %#v", cloned.Tools)
	}
}
