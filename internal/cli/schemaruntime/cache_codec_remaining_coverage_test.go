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
	"google.golang.org/protobuf/proto"
)

func TestCrossPlatformCoverageSchemaCacheRemainingValidateAndReject(t *testing.T) {
	if !validMetaAliasExpansion(map[string]CommandMeta{
		"a": {Identity: CommandIdentity{CLIPath: "a", Aliases: []string{"x"}}},
		"b": {Identity: CommandIdentity{CLIPath: "b", Aliases: []string{"x"}}},
		"x": {Identity: CommandIdentity{CLIPath: "b", Aliases: []string{"x"}}},
	}) {
		// lexical owner is "a" but lookup["x"] belongs to "b"
	} else {
		t.Fatal("alias owner mismatch accepted")
	}
	if equalJSONValues([]byte(`{`), []byte(`{`)) {
		t.Fatal("invalid equal json")
	}

	if _, err := buildSchemaProductLocatorsUnchecked(SchemaRegistry{Products: []ProductSpec{
		{ID: "beta", Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{
			ProductID: "beta", Name: "run", CanonicalPath: "beta.run", CLIPath: "beta run", Aliases: []string{"alpha"},
		}}}},
		{ID: "alpha"},
	}}); err == nil {
		t.Fatal("product id locator conflict")
	}
	if _, err := buildSchemaProductLocatorsUnchecked(SchemaRegistry{Products: []ProductSpec{
		{ID: "drive", Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{
			ProductID: "drive", Name: "list", CanonicalPath: "drive.files.list", CLIPath: "drive files list",
		}}}},
		{ID: "mail", Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{
			ProductID: "mail", Name: "send", CanonicalPath: "mail.send", CLIPath: "drive other",
		}}}},
	}}); err == nil {
		t.Fatal("prefix locator conflict")
	}

	registry := SchemaRegistry{Kind: "schema", Products: []ProductSpec{{
		ID: "sample",
		Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{
			ProductID: "sample", Name: "run", CanonicalPath: "sample.run", CLIPath: "sample run",
		}, Title: "Run", Description: "Runs"}},
	}}}
	lookup := BuildCommandMetaLookup(registry)
	overview, err := BuildSchemaOverview(registry)
	if err != nil {
		t.Fatal(err)
	}
	locators, err := BuildSchemaProductLocators(registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildSchemaCache(registry, map[string]CommandMeta{"x": {}}, overview, locators, fixtureHashes(), fixtureRenderedLeaves(lookup)); err == nil {
		t.Fatal("lookup mismatch")
	}
	if _, err := BuildSchemaCache(registry, lookup, SchemaOverview{}, locators, fixtureHashes(), fixtureRenderedLeaves(lookup)); err == nil {
		t.Fatal("overview mismatch")
	}
	if _, err := BuildSchemaCache(registry, lookup, overview, map[string]string{"x": "sample"}, fixtureHashes(), fixtureRenderedLeaves(lookup)); err == nil {
		t.Fatal("locator mismatch")
	}
	if _, err := BuildSchemaCache(registry, lookup, overview, locators, fixtureHashes(), nil); err == nil {
		t.Fatal("rendered leaves")
	}
	conflicting := SchemaRegistry{Kind: "schema", Products: []ProductSpec{
		{ID: "alpha", Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{
			ProductID: "alpha", Name: "run", CanonicalPath: "alpha.run", CLIPath: "alpha run",
		}}}},
		{ID: "beta", Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{
			ProductID: "beta", Name: "run", CanonicalPath: "beta.run", CLIPath: "beta run", Aliases: []string{"alpha"},
		}}}},
	}}
	conflictLookup := BuildCommandMetaLookup(conflicting)
	conflictOverview, err := BuildSchemaOverview(conflicting)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildSchemaCache(conflicting, conflictLookup, conflictOverview, map[string]string{}, fixtureHashes(), fixtureRenderedLeaves(conflictLookup)); err == nil {
		t.Fatal("unchecked locator conflict")
	}

	built, meta := buildFixtureCache(t, allFieldsRegistry())
	aliasMiss := meta
	aliasMiss.CommandMetaByPath = map[string]CommandMeta{}
	aliasMiss.LocatorProductByPath = map[string]string{}
	for path, id := range meta.LocatorProductByPath {
		aliasMiss.LocatorProductByPath[path] = id
	}
	var samplePath, canonical string
	samplePath = "sample group run"
	if m, ok := meta.CommandMeta(samplePath); ok {
		canonical = m.Identity.Canonical
	}
	if canonical != "" {
		aliasMiss.LocatorProductByPath[canonical] = "other"
		if _, ok := aliasMiss.CommandMeta(samplePath); ok {
			t.Fatal("canonical locator mismatch")
		}
	}

	var message schemacachepb.SchemaMetaCache
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.CommandEntryShards.Items[0].EntryCount = 0
	payload, err := MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("zero entry count")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.CommandEntryShards.Items[0].ProductId = "ghost"
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("shard without descriptor")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Registry.Kind = "other"
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("overview registry disagree")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Overview.Products.Items[0].Id = "zzz"
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("overview unsorted")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Overview.Products.Items[0].SummaryKind = 0
	message.Overview.Products.Items[0].Summary = "stale"
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("overview summary presence")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Overview.ToolCount = 0
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("overview tool count")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	second := proto.Clone(message.Overview.Products.Items[0]).(*schemacachepb.OverviewProduct)
	second.Id = "zzzz"
	second.SchemaPath = "zzzz"
	second.ToolCount = math.MaxUint64
	message.Overview.Products.Items[0].ToolCount = math.MaxUint64
	message.Overview.Products.Items = append(message.Overview.Products.Items, second)
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("overview overflow")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Overview.Products.Items = nil
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("overview product count")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Overview.Products.Items[0].Id = "other"
	message.Overview.Products.Items[0].SchemaPath = "other"
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("overview descriptor disagree")
	}

	if err := validateDescriptors(nil, 1); err == nil {
		t.Fatal("empty descriptors")
	}
	if err := validateDescriptors([]ProductDescriptor{{ProductID: "b"}, {ProductID: "a", Offset: 1, Length: 1}}, 2); err == nil {
		t.Fatal("unsorted descriptors")
	}
	if err := validateDescriptors([]ProductDescriptor{{ProductID: "a", Offset: 1, Length: 1}}, 2); err == nil {
		t.Fatal("descriptor gap")
	}
	if err := validateDescriptors([]ProductDescriptor{{ProductID: "a", Length: 1}}, 2); err == nil {
		t.Fatal("descriptor cover")
	}

	if err := rejectUnknownFieldsAndEnums(&schemacachepb.SchemaProductCache{DtoVersion: 99}); err == nil {
		t.Fatal("unknown product dto")
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.OverviewProduct{SummaryKind: 99}); err == nil {
		t.Fatal("unknown summary kind")
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.LocatorEntryList{Items: []*schemacachepb.LocatorEntry{{LookupPath: "p"}}}); err != nil {
		t.Fatal(err)
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.ProductDescriptorList{Items: []*schemacachepb.ProductDescriptor{{ProductId: "p"}}}); err != nil {
		t.Fatal(err)
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.ToolList{Items: []*schemacachepb.ToolSpec{}}); err != nil {
		t.Fatal(err)
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.ParameterList{Items: []*schemacachepb.ParameterSpec{}}); err != nil {
		t.Fatal(err)
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.StringListList{Items: []*schemacachepb.StringList{{Items: []string{"a"}}}}); err != nil {
		t.Fatal(err)
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.PositionalList{Items: []*schemacachepb.Positional{{Name: "p"}}}); err != nil {
		t.Fatal(err)
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.ResultOutcomeList{Items: []schemacachepb.ResultOutcome{99}}); err == nil {
		t.Fatal("unknown outcome")
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.ExampleDispositionList{Items: []*schemacachepb.ExampleDisposition{{Mode: 99}}}); err == nil {
		t.Fatal("unknown disposition mode")
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.ExampleDisposition{Mode: schemacachepb.ExampleDispositionMode_EXAMPLE_DISPOSITION_MODE_CONTRACT_ONLY, ReasonCode: 99}); err == nil {
		t.Fatal("unknown reason code")
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.ProvenanceList{Items: []*schemacachepb.ProvenanceEntry{}}); err != nil {
		t.Fatal(err)
	}
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.CandidateList{Items: []*schemacachepb.FieldCandidate{}}); err != nil {
		t.Fatal(err)
	}
	unknown := &schemacachepb.SchemaCommandPayloadCache{DtoVersion: 99}
	if err := rejectUnknownFieldsAndEnums(unknown); err == nil {
		t.Fatal("unknown payload dto via reflect")
	}
	withUnknown := &schemacachepb.BytesValue{}
	withUnknown.ProtoReflect().SetUnknown([]byte{0x62, 0x01, 0x00})
	if err := rejectUnknownFieldsAndEnumsReflect(withUnknown.ProtoReflect()); err == nil {
		t.Fatal("unknown reflect fields")
	}

	var product schemacachepb.SchemaProductCache
	if err := proto.Unmarshal(built.ProductShards, &product); err != nil {
		t.Fatal(err)
	}
	product.Product.Id = ""
	if err := validateProductProto(product.Product); err == nil {
		t.Fatal("missing product identity")
	}
	if err := proto.Unmarshal(built.ProductShards, &product); err != nil {
		t.Fatal(err)
	}
	product.Registry.Kind = "other"
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
		t.Fatal("registry fields disagree")
	}
	if err := proto.Unmarshal(built.ProductShards, &product); err != nil {
		t.Fatal(err)
	}
	product.Product.Id = "other"
	badProduct, err = MarshalSchemaCacheDeterministic(&product)
	if err != nil {
		t.Fatal(err)
	}
	updated.Length = uint64(len(badProduct))
	updated.SHA256 = sha256.Sum256(badProduct)
	updatedMeta.ProductDescriptors[0] = updated
	if _, err := DecodeSchemaProductCache(badProduct, updated, updatedMeta); err == nil {
		t.Fatal("product identity mismatch")
	}

	if err := validateProductProto(&schemacachepb.ProductSpec{
		Id: "p", Selection: &schemacachepb.Selection{},
		Tools: &schemacachepb.ToolList{Items: []*schemacachepb.ToolSpec{nil}},
	}); err == nil {
		t.Fatal("nil tool")
	}
	if err := validateProductProto(&schemacachepb.ProductSpec{
		Id: "p", Selection: &schemacachepb.Selection{},
		Tools: &schemacachepb.ToolList{Items: []*schemacachepb.ToolSpec{
			{
				Identity:    &schemacachepb.ToolIdentity{CanonicalPath: "b.run"},
				Constraints: &schemacachepb.Constraints{},
				Safety:      &schemacachepb.Safety{},
				Interface:   &schemacachepb.Interface{},
				Selection:   &schemacachepb.Selection{},
			},
			{
				Identity:    &schemacachepb.ToolIdentity{CanonicalPath: "a.run"},
				Constraints: &schemacachepb.Constraints{},
				Safety:      &schemacachepb.Safety{},
				Interface:   &schemacachepb.Interface{},
				Selection:   &schemacachepb.Selection{},
			},
		}},
	}); err == nil {
		t.Fatal("unsorted tools")
	}
	if err := validateProductProto(&schemacachepb.ProductSpec{
		Id: "p", Selection: &schemacachepb.Selection{},
		Tools: &schemacachepb.ToolList{Items: []*schemacachepb.ToolSpec{{
			Identity:    &schemacachepb.ToolIdentity{CanonicalPath: "p.run"},
			Constraints: &schemacachepb.Constraints{},
			Safety:      &schemacachepb.Safety{},
			Interface:   &schemacachepb.Interface{},
			Selection:   &schemacachepb.Selection{},
			Parameters:  &schemacachepb.ParameterList{Items: []*schemacachepb.ParameterSpec{nil}},
		}}},
	}); err == nil {
		t.Fatal("nil parameter")
	}
	if err := validateProductProto(&schemacachepb.ProductSpec{
		Id: "p", Selection: &schemacachepb.Selection{},
		Tools: &schemacachepb.ToolList{Items: []*schemacachepb.ToolSpec{{
			Identity:    &schemacachepb.ToolIdentity{CanonicalPath: "p.run"},
			Constraints: &schemacachepb.Constraints{},
			Safety:      &schemacachepb.Safety{},
			Interface:   &schemacachepb.Interface{},
			Selection:   &schemacachepb.Selection{},
			Result:      &schemacachepb.Result{},
		}}},
	}); err == nil {
		t.Fatal("missing result schema")
	}
	if err := validateSelectionEnums(&schemacachepb.Selection{
		ExampleDispositions: &schemacachepb.ExampleDispositionList{Items: []*schemacachepb.ExampleDisposition{{}}},
	}, "tool"); err == nil {
		t.Fatal("unspecified disposition")
	}
	if err := validateProvenanceProto(&schemacachepb.ProvenanceList{Items: []*schemacachepb.ProvenanceEntry{nil}}, "p"); err == nil {
		t.Fatal("nil provenance")
	}
	if err := validateProvenanceProto(&schemacachepb.ProvenanceList{Items: []*schemacachepb.ProvenanceEntry{{
		Key: "a", Value: &schemacachepb.FieldProvenance{Candidates: &schemacachepb.CandidateList{Items: []*schemacachepb.FieldCandidate{nil}}},
	}}}, "p"); err == nil {
		t.Fatal("nil candidate")
	}

	desc := meta.PayloadDescriptors[0]
	start := int(built.PayloadIndexLength + desc.Offset)
	shard := built.PayloadShards[start : start+int(desc.Length)]
	header, blobs, err := splitCommandPayloadShard(shard)
	if err != nil {
		t.Fatal(err)
	}
	var payloadRoot schemacachepb.SchemaCommandPayloadCache
	if err := proto.Unmarshal(header, &payloadRoot); err != nil {
		t.Fatal(err)
	}
	payloadRoot.DtoVersion = 99
	badHeader, err := MarshalSchemaCacheDeterministic(&payloadRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCommandPayloadHeader(badHeader, desc); err == nil {
		t.Fatal("payload reject unknown")
	}

	indexRegion := built.PayloadShards[:built.PayloadIndexLength]
	indexHeader, _, err := splitCommandPayloadShard(indexRegion)
	if err != nil {
		t.Fatal(err)
	}
	var index schemacachepb.SchemaPayloadIndex
	if err := proto.Unmarshal(indexHeader, &index); err != nil {
		t.Fatal(err)
	}
	index.Locators.Items = append(index.Locators.Items, index.Locators.Items[0])
	badIndex, err := MarshalSchemaCacheDeterministic(&index)
	if err != nil {
		t.Fatal(err)
	}
	assembled, err := assembleCommandPayloadShard(badIndex, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaPayloadIndex(assembled); err == nil {
		t.Fatal("unsorted payload locators")
	}

	wrongHeader := desc
	wrongHeader.HeaderSHA256[0] ^= 1
	if _, err := DecodeSchemaCommandPayloadHeader(shard[:desc.HeaderLength], wrongHeader); err == nil {
		t.Fatal("header digest")
	}
	wrongPayload := desc
	wrongPayload.SHA256[0] ^= 1
	if _, err := DecodeSchemaCommandPayloadCache(shard, wrongPayload, meta); err == nil {
		t.Fatal("payload digest")
	}
	splitFail := desc
	splitFail.Length = 3
	splitFail.SHA256 = sha256.Sum256(shard[:3])
	splitMeta := meta
	splitMeta.PayloadDescriptors = []CommandPayloadDescriptor{splitFail}
	if _, err := DecodeSchemaCommandPayloadCache(shard[:3], splitFail, splitMeta); err == nil {
		t.Fatal("payload split")
	}
	headerDisagree := desc
	headerDisagree.HeaderLength++
	headerDisagree.SHA256 = desc.SHA256
	headerMeta := meta
	headerMeta.PayloadDescriptors = []CommandPayloadDescriptor{headerDisagree}
	if _, err := DecodeSchemaCommandPayloadCache(shard, headerDisagree, headerMeta); err == nil {
		t.Fatal("header prefix disagree")
	}

	rangeFail := desc
	rangeFail.HeaderLength = desc.HeaderLength
	var payloadRoot2 schemacachepb.SchemaCommandPayloadCache
	if err := proto.Unmarshal(header, &payloadRoot2); err != nil {
		t.Fatal(err)
	}
	if len(payloadRoot2.RenderedLeafIndex.Items) > 0 {
		payloadRoot2.RenderedLeafIndex.Items[0].Offset = uint64(len(blobs) + 1)
		payloadRoot2.RenderedLeafIndex.Items[0].Length = 1
		sum := sha256.Sum256([]byte("x"))
		payloadRoot2.RenderedLeafIndex.Items[0].Sha256 = sum[:]
		badHeader, err = MarshalSchemaCacheDeterministic(&payloadRoot2)
		if err != nil {
			t.Fatal(err)
		}
		assembled, err = assembleCommandPayloadShard(badHeader, blobs)
		if err != nil {
			t.Fatal(err)
		}
		rangeDesc := desc
		rangeDesc.Length = uint64(len(assembled))
		rangeDesc.SHA256 = sha256.Sum256(assembled)
		rangeDesc.HeaderLength = uint64(4 + len(badHeader))
		rangeDesc.HeaderSHA256 = sha256.Sum256(assembled[:rangeDesc.HeaderLength])
		rangeMeta := meta
		rangeMeta.PayloadDescriptors = []CommandPayloadDescriptor{rangeDesc}
		if _, err := DecodeSchemaCommandPayloadCache(assembled, rangeDesc, rangeMeta); err == nil {
			t.Fatal("leaf range")
		}
	}

	digestFail := meta
	digestFail.RegistryDataSHA256[0] ^= 1
	if _, err := DecodeSchemaProductFromShards(built.ProductShards, digestFail, "sample"); err == nil {
		t.Fatal("shard digest")
	}
	boundsMeta := meta
	if len(boundsMeta.ProductDescriptors) > 0 {
		boundsMeta.ProductDescriptors[0].Offset = uint64(len(built.ProductShards) + 1)
		if _, err := DecodeSchemaProductFromShards(built.ProductShards, boundsMeta, boundsMeta.ProductDescriptors[0].ProductID); err == nil {
			t.Fatal("shard bounds")
		}
		if _, _, err := DecodeAllSchemaProducts(built.ProductShards, boundsMeta); err == nil {
			t.Fatal("all products bounds")
		}
	}
	_ = json.RawMessage(nil)
}

func TestCrossPlatformCoverageDecodeSchemaProductMissingWrappers(t *testing.T) {
	built, meta := buildFixtureCache(t, allFieldsRegistry())
	var product schemacachepb.SchemaProductCache
	if err := proto.Unmarshal(built.ProductShards, &product); err != nil {
		t.Fatal(err)
	}
	product.Registry = nil
	payload, err := MarshalSchemaCacheDeterministic(&product)
	if err != nil {
		t.Fatal(err)
	}
	updated := meta.ProductDescriptors[0]
	updated.Length = uint64(len(payload))
	updated.SHA256 = sha256.Sum256(payload)
	updatedMeta := meta
	updatedMeta.ProductDescriptors = []ProductDescriptor{updated}
	if _, err := DecodeSchemaProductCache(payload, updated, updatedMeta); err == nil {
		t.Fatal("missing registry wrapper")
	}
	if err := proto.Unmarshal(built.ProductShards, &product); err != nil {
		t.Fatal(err)
	}
	product.Registry.AgentMetadata = &schemacachepb.BytesValue{Value: []byte(`{`)}
	payload, err = MarshalSchemaCacheDeterministic(&product)
	if err != nil {
		t.Fatal(err)
	}
	updated.Length = uint64(len(payload))
	updated.SHA256 = sha256.Sum256(payload)
	updatedMeta.ProductDescriptors[0] = updated
	if _, err := DecodeSchemaProductCache(payload, updated, updatedMeta); err == nil {
		t.Fatal("invalid product agent metadata")
	}
}
