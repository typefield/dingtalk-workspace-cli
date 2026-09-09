// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package schemaruntime

import (
	"crypto/sha256"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/schemacachepb"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"google.golang.org/protobuf/proto"
)

func TestCrossPlatformCoverageSchemaCacheAccessAndDecodeErrorBranches(t *testing.T) {
	built, meta := buildFixtureCache(t, allFieldsRegistry())
	if _, ok := meta.CommandMeta("missing"); ok {
		t.Fatal("missing command")
	}
	if _, ok := meta.CommandMeta("sample group run"); !ok {
		t.Fatal("lazy shard lookup")
	}
	hit := DecodedSchemaMeta{CommandMetaByPath: map[string]CommandMeta{"p": {Identity: CommandIdentity{Canonical: "p"}}}}
	if _, ok := hit.CommandMeta("p"); !ok {
		t.Fatal("map hit")
	}
	missProduct := meta
	missProduct.LocatorProductByPath = map[string]string{"ghost": "missing-product"}
	if _, ok := missProduct.CommandMeta("ghost"); ok {
		t.Fatal("missing shard product")
	}
	meta.MaterializeCommandMeta()
	if _, ok := meta.CommandMeta("sample group run"); !ok {
		t.Fatal("materialized lookup")
	}
	emptyMaterialize := DecodedSchemaMeta{CommandMetaByPath: map[string]CommandMeta{}, commandEntryShards: []*schemacachepb.CommandMetaEntryShard{{ProductId: "ghost"}}}
	emptyMaterialize.MaterializeCommandMeta()
	if _, ok := meta.commandMetaMapForProduct("missing-product"); ok {
		t.Fatal("missing product map")
	}

	if _, err := BuildSchemaOverview(SchemaRegistry{Kind: "schema", Products: []ProductSpec{{
		ID: "drive", Selection: contract.SelectionSpec{UseWhen: []string{"when"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildSchemaOverview(SchemaRegistry{Kind: "schema", Products: []ProductSpec{{
		ID: "mail", Description: "mail product",
	}}}); err != nil {
		t.Fatal(err)
	}
	if err := validateRenderedSchemaLeaves(map[string]CommandMeta{
		"cli": {Identity: CommandIdentity{CLIPath: "cli", Canonical: "sample.run"}},
	}, nil); err == nil {
		t.Fatal("missing rendered leaf")
	}
	if err := validateRenderedSchemaLeaves(map[string]CommandMeta{
		"cli": {Identity: CommandIdentity{CLIPath: "cli", Canonical: "sample.run"}},
	}, map[string][]byte{"sample.run": []byte("nope")}); err == nil {
		t.Fatal("invalid rendered json")
	}
	if _, err := toolsToProto([]ToolSpec{{
		Identity:  contract.ToolIdentitySpec{CanonicalPath: "sample.run"},
		Selection: contract.SelectionSpec{ExampleDispositions: []contract.ExampleDisposition{{Mode: "nope"}}},
	}}); err == nil {
		t.Fatal("invalid tool selection")
	}

	conflicting := SchemaRegistry{Kind: "schema", Products: []ProductSpec{
		{ID: "alpha", Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{
			ProductID: "alpha", Name: "run", CanonicalPath: "alpha.run", CLIPath: "alpha run",
		}}}},
		{ID: "beta", Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{
			ProductID: "beta", Name: "run", CanonicalPath: "beta.run", CLIPath: "beta run", Aliases: []string{"alpha"},
		}}}},
	}}
	if _, err := BuildSchemaProductLocators(conflicting); err == nil {
		t.Fatal("locator conflict")
	}

	if !commandMetaSubsetEqual(map[string]CommandMeta{"a": {}}, map[string]CommandMeta{}) {
		t.Fatal("empty subset")
	}
	if commandMetaSubsetEqual(map[string]CommandMeta{}, map[string]CommandMeta{"a": {}}) {
		t.Fatal("missing subset path")
	}
	if commandIdentitySubsetEqual(map[string]CommandMeta{}, map[string]CommandMeta{"a": {}}) {
		t.Fatal("missing identity subset")
	}
	if locatorSubsetEqual(map[string]string{"a": "p"}, map[string]string{"a": "q"}) {
		t.Fatal("locator mismatch")
	}

	if _, err := DecodeSchemaProductFromShards(nil, meta, "sample"); err == nil {
		t.Fatal("shard length mismatch")
	}
	wrongHash := append([]byte(nil), built.ProductShards...)
	wrongHash[0] ^= 1
	if uint64(len(wrongHash)) == meta.RegistryDataLength {
		if _, err := DecodeSchemaProductFromShards(wrongHash, meta, "sample"); err == nil {
			t.Fatal("shard digest mismatch")
		}
	}
	if _, err := DecodeSchemaProductFromShards(built.ProductShards, meta, "missing"); err == nil {
		t.Fatal("unknown product from shards")
	}
	if _, _, err := DecodeAllSchemaProducts(nil, meta); err == nil {
		t.Fatal("all-products identity mismatch")
	}

	desc := meta.PayloadDescriptors[0]
	start := int(built.PayloadIndexLength + desc.Offset)
	shard := built.PayloadShards[start : start+int(desc.Length)]
	if _, err := DecodeSchemaCommandPayloadCache(shard, desc, meta); err != nil {
		t.Fatalf("valid payload cache: %v", err)
	}
	headerOnly := shard[:int(desc.HeaderLength)]
	if _, err := DecodeSchemaCommandPayloadHeader(headerOnly, desc); err != nil {
		t.Fatalf("valid payload header: %v", err)
	}
	header, blobs, err := splitCommandPayloadShard(shard)
	if err != nil {
		t.Fatal(err)
	}
	var payloadRoot schemacachepb.SchemaCommandPayloadCache
	if err := proto.Unmarshal(header, &payloadRoot); err != nil {
		t.Fatal(err)
	}
	payloadRoot.DtoVersion = schemacachepb.DTOVersion_DTO_VERSION_V1
	badHeader, err := MarshalSchemaCacheDeterministic(&payloadRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCommandPayloadHeader(badHeader, desc); err == nil {
		t.Fatal("wrong payload dto version")
	}
	payloadRoot.DtoVersion = schemacachepb.DTOVersion_DTO_VERSION_V5
	payloadRoot.ProductId = "other"
	badHeader, err = MarshalSchemaCacheDeterministic(&payloadRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCommandPayloadHeader(badHeader, desc); err == nil {
		t.Fatal("payload identity mismatch")
	}
	if _, err := assembleCommandPayloadShard(header, blobs); err != nil {
		t.Fatal(err)
	}

	var message schemacachepb.SchemaMetaCache
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.DtoVersion = schemacachepb.DTOVersion_DTO_VERSION_V1
	payload, err := MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("retired meta version")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.CommandEntryShards = nil
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("missing meta wrapper")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Registry.AgentMetadata = &schemacachepb.BytesValue{Value: []byte(`{`)}
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("invalid meta agent metadata")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.SourceSha256 = []byte{1}
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("short meta digest")
	}

	indexRegion := built.PayloadShards[:built.PayloadIndexLength]
	if _, err := DecodeSchemaPayloadIndex(indexRegion); err != nil {
		t.Fatalf("valid payload index: %v", err)
	}
	var index schemacachepb.SchemaPayloadIndex
	indexHeader, _, err := splitCommandPayloadShard(indexRegion)
	if err != nil {
		t.Fatal(err)
	}
	if err := proto.Unmarshal(indexHeader, &index); err != nil {
		t.Fatal(err)
	}
	index.DtoVersion = schemacachepb.DTOVersion_DTO_VERSION_V1
	badIndex, err := MarshalSchemaCacheDeterministic(&index)
	if err != nil {
		t.Fatal(err)
	}
	assembled, err := assembleCommandPayloadShard(badIndex, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaPayloadIndex(assembled); err == nil {
		t.Fatal("wrong payload index version")
	}
	index.DtoVersion = schemacachepb.DTOVersion_DTO_VERSION_V5
	index.Locators = nil
	missingWrapper, err := MarshalSchemaCacheDeterministic(&index)
	if err != nil {
		t.Fatal(err)
	}
	assembled, err = assembleCommandPayloadShard(missingWrapper, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaPayloadIndex(assembled); err == nil {
		t.Fatal("missing payload index wrapper")
	}

	var product schemacachepb.SchemaProductCache
	if err := proto.Unmarshal(built.ProductShards, &product); err != nil {
		t.Fatal(err)
	}
	product.DtoVersion = schemacachepb.DTOVersion_DTO_VERSION_V1
	badProduct, err := MarshalSchemaCacheDeterministic(&product)
	if err != nil {
		t.Fatal(err)
	}
	updated := meta.ProductDescriptors[0]
	updated.Length = uint64(len(badProduct))
	updated.SHA256 = sha256.Sum256(badProduct)
	updatedMeta := meta
	updatedMeta.ProductDescriptors = append([]ProductDescriptor(nil), meta.ProductDescriptors...)
	updatedMeta.ProductDescriptors[0] = updated
	if _, err := DecodeSchemaProductCache(badProduct, updated, updatedMeta); err == nil {
		t.Fatal("wrong product dto version")
	}
}
