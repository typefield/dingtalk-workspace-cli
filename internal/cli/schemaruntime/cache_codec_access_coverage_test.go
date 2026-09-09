// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package schemaruntime

import (
	"crypto/sha256"
	"encoding/json"
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

	if _, err := SchemaRegistryFromRuntime("src", []ProductSpec{{}}); err == nil {
		t.Fatal("empty product id")
	}
	if _, err := ToolSpecFromRuntime(RuntimeToolSpecInput{}); err == nil {
		t.Fatal("empty tool spec")
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

func TestCrossPlatformCoverageSchemaCacheShardAccessAndModelValidate(t *testing.T) {
	spec := ToolSpec{
		Identity:       contract.ToolIdentitySpec{CanonicalPath: "sample.run"},
		Title:          "t",
		Description:    "d",
		MetadataSource: "m",
		DryRun:         &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewRequest},
		Safety:         contract.SafetySpec{Effect: "write", EffectSource: "declared", Risk: "high", Confirmation: "user_required", Idempotency: "idempotent"},
		Interface:      contract.InterfaceSpec{Ref: &contract.InterfaceRefSpec{ProductID: "p", RPCName: "n"}, Mode: contract.InterfaceModeMCP, Availability: contract.InterfaceAvailable, Reason: "why"},
		Selection: contract.SelectionSpec{
			AgentSummary: "sum", UseWhen: []string{"u"}, AvoidWhen: []string{"a"},
			Prerequisites: []string{"p"}, Tips: []string{"t"}, WorkflowRefs: []string{"w"}, Examples: []string{"e"},
		},
	}
	for _, field := range []string{
		"canonical_path", "title", "description", "metadata_source", "dry_run",
		"effect", "effect_source", "risk", "confirmation", "idempotency",
		"interface_ref", "interface_mode", "availability", "interface_reason",
		"agent_summary", "use_when", "avoid_when", "prerequisites", "tips",
		"workflow_refs", "examples", "reviewed",
	} {
		if _, ok := spec.provenanceValue(field); !ok {
			t.Fatalf("tool provenance %s", field)
		}
	}
	if _, ok := spec.provenanceValue("unknown"); ok {
		t.Fatal("unknown tool provenance")
	}
	param := ParameterSpec{
		Name: "id", Type: "string", Description: "d", Property: "p", Required: true,
		CLIRequired: true, RequiredWhen: "always", Default: json.RawMessage(`"x"`),
		InterfaceDefault: json.RawMessage(`"y"`), Example: json.RawMessage(`"z"`),
		AnyOf: []contract.FormatAlternative{{Format: "email"}}, Format: "token", Enum: []string{"a"},
		InterfaceDescription: "id", InterfaceType: "string",
	}
	for _, field := range []string{
		"name", "type", "description", "property", "required", "cli_required",
		"required_when", "default", "interface_default", "example", "anyOf",
		"format", "enum", "interface_description", "interface_type",
	} {
		if _, ok := param.provenanceValue(field); !ok {
			t.Fatalf("parameter provenance %s", field)
		}
	}
	product := ProductSpec{Selection: contract.SelectionSpec{AgentSummary: "a", UseWhen: []string{"u"}, AvoidWhen: []string{"v"}}}
	for _, field := range []string{"agent_summary", "use_when", "avoid_when"} {
		if _, ok := product.provenanceValue(field); !ok {
			t.Fatalf("product provenance %s", field)
		}
	}

	base := ToolSpec{Identity: contract.ToolIdentitySpec{ProductID: "sample", Name: "run", CanonicalPath: "sample.run", CLIPath: "sample run"}}
	if err := (ToolSpec{}).Validate(); err == nil {
		t.Fatal("empty product id")
	}
	if err := (ToolSpec{Identity: contract.ToolIdentitySpec{ProductID: "sample"}}).Validate(); err == nil {
		t.Fatal("empty name")
	}
	mismatch := base
	mismatch.Identity.CanonicalPath = "other.run"
	if err := mismatch.Validate(); err == nil {
		t.Fatal("canonical mismatch")
	}
	emptyCLI := base
	emptyCLI.Identity.CLIPath = ""
	if err := emptyCLI.Validate(); err == nil {
		t.Fatal("empty cli path")
	}
	emptyParam := base
	emptyParam.Parameters = []ParameterSpec{{}}
	if err := emptyParam.Validate(); err == nil {
		t.Fatal("empty parameter name")
	}
	dupParam := base
	dupParam.Parameters = []ParameterSpec{{Name: "id"}, {Name: "id"}}
	if err := dupParam.Validate(); err == nil {
		t.Fatal("duplicate parameter")
	}
	badDefault := base
	badDefault.Parameters = []ParameterSpec{{Name: "id", Default: json.RawMessage(`{`)}}
	if err := badDefault.Validate(); err == nil {
		t.Fatal("invalid default json")
	}
	badRef := base
	badRef.Interface.Ref = &contract.InterfaceRefSpec{ProductID: " "}
	if err := badRef.Validate(); err == nil {
		t.Fatal("incomplete interface_ref")
	}
	badDryRun := base
	badDryRun.DryRun = &contract.DryRunSpec{PreviewKind: "nope"}
	if err := badDryRun.Validate(); err == nil {
		t.Fatal("invalid dry-run")
	}
	badResult := base
	badResult.Result = &contract.ResultSpec{Outcomes: []contract.ResultOutcome{"nope"}}
	if err := badResult.Validate(); err == nil {
		t.Fatal("invalid result")
	}
	badPage := base
	badPage.Parameters = []ParameterSpec{{Name: "cursor"}}
	badPage.Pagination = &contract.PaginationSpec{Kind: contract.PaginationKindCursor, CursorParameter: "missing"}
	if err := badPage.Validate(); err == nil {
		t.Fatal("missing pagination cursor")
	}
	badIface := base
	badIface.Interface.Mode = "nope"
	if err := badIface.Validate(); err == nil {
		t.Fatal("invalid interface")
	}
	if _, err := (SchemaRegistry{Products: []ProductSpec{{ID: "sample", Tools: []ToolSpec{base}}}}).ToPayload(); err != nil {
		t.Fatal(err)
	}
	if _, err := (SchemaRegistry{Products: []ProductSpec{{}}}).ToPayload(); err == nil {
		t.Fatal("invalid product payload")
	}
	aliasTool := base
	aliasTool.Identity.IsAlias = true
	if _, err := (SchemaRegistry{Products: []ProductSpec{{ID: "sample", Tools: []ToolSpec{aliasTool}}}}).Index(); err == nil {
		t.Fatal("alias canonical tool")
	}
	splitPath := base
	splitPath.Identity.PrimaryCLIPath = "sample other"
	if _, err := (SchemaRegistry{Products: []ProductSpec{{ID: "sample", Tools: []ToolSpec{splitPath}}}}).Index(); err == nil {
		t.Fatal("cli/primary mismatch")
	}
	badPageKind := base
	badPageKind.Pagination = &contract.PaginationSpec{Kind: "nope", CursorParameter: "cursor"}
	badPageKind.Parameters = []ParameterSpec{{Name: "cursor"}}
	if err := badPageKind.Validate(); err == nil {
		t.Fatal("invalid pagination kind")
	}
	selected := true
	winner := json.RawMessage(`"Run sample"`)
	badProv := base
	badProv.Title = "Run sample"
	badProv.FieldProvenance = map[string]contract.FieldProvenance{"title": {
		Value: json.RawMessage(`"other"`), Source: "contract_final", Precedence: "100", Resolution: "selected",
		Candidates: []contract.FieldCandidateProvenance{{Value: winner, Source: "contract_final", Precedence: "100", Selected: &selected}},
	}}
	if err := badProv.Validate(); err == nil {
		t.Fatal("provenance winner mismatch")
	}
	incompleteProv := base
	incompleteProv.Title = "Run sample"
	incompleteProv.FieldProvenance = map[string]contract.FieldProvenance{"title": {
		Value: winner, Source: "", Precedence: "100", Resolution: "selected",
		Candidates: []contract.FieldCandidateProvenance{{Value: winner, Source: "contract_final", Precedence: "100", Selected: &selected}},
	}}
	if err := incompleteProv.Validate(); err == nil {
		t.Fatal("incomplete provenance")
	}
	noCand := base
	noCand.Title = "Run sample"
	noCand.FieldProvenance = map[string]contract.FieldProvenance{"title": {
		Value: winner, Source: "contract_final", Precedence: "100", Resolution: "selected",
	}}
	if err := noCand.Validate(); err == nil {
		t.Fatal("no provenance candidates")
	}
	badCand := base
	badCand.Title = "Run sample"
	notSel := false
	badCand.FieldProvenance = map[string]contract.FieldProvenance{"title": {
		Value: winner, Source: "contract_final", Precedence: "100", Resolution: "selected",
		Candidates: []contract.FieldCandidateProvenance{
			{Value: winner, Source: "contract_final", Precedence: "100", Selected: &selected},
			{Value: json.RawMessage(`{`), Source: "other", Precedence: "1", Selected: &notSel},
		},
	}}
	if err := badCand.Validate(); err == nil {
		t.Fatal("invalid candidate json")
	}
	selMismatch := base
	selMismatch.Title = "Run sample"
	selMismatch.FieldProvenance = map[string]contract.FieldProvenance{"title": {
		Value: winner, Source: "contract_final", Precedence: "100", Resolution: "selected",
		Candidates: []contract.FieldCandidateProvenance{{Value: json.RawMessage(`"nope"`), Source: "contract_final", Precedence: "100", Selected: &selected}},
	}}
	if err := selMismatch.Validate(); err == nil {
		t.Fatal("selected candidate mismatch")
	}
	badOver := base
	badOver.Title = "Run sample"
	badOver.FieldProvenance = map[string]contract.FieldProvenance{"title": {
		Value: winner, Source: "contract_final", Precedence: "100", Resolution: "selected",
		Candidates:           []contract.FieldCandidateProvenance{{Value: winner, Source: "contract_final", Precedence: "100", Selected: &selected}},
		OverriddenCandidates: []contract.FieldCandidateProvenance{{Value: json.RawMessage(`{`)}},
	}}
	if err := badOver.Validate(); err == nil {
		t.Fatal("invalid overridden candidate")
	}
	selOver := base
	selOver.Title = "Run sample"
	selOver.FieldProvenance = map[string]contract.FieldProvenance{"title": {
		Value: winner, Source: "contract_final", Precedence: "100", Resolution: "selected",
		Candidates:           []contract.FieldCandidateProvenance{{Value: winner, Source: "contract_final", Precedence: "100", Selected: &selected}},
		OverriddenCandidates: []contract.FieldCandidateProvenance{{Value: json.RawMessage(`"old"`), Selected: &selected}},
	}}
	if err := selOver.Validate(); err == nil {
		t.Fatal("selected overridden candidate")
	}
	twoSel := base
	twoSel.Title = "Run sample"
	twoSel.FieldProvenance = map[string]contract.FieldProvenance{"title": {
		Value: winner, Source: "contract_final", Precedence: "100", Resolution: "selected",
		Candidates: []contract.FieldCandidateProvenance{
			{Value: winner, Source: "contract_final", Precedence: "100", Selected: &selected},
			{Value: winner, Source: "contract_final", Precedence: "100", Selected: &selected},
		},
	}}
	if err := twoSel.Validate(); err == nil {
		t.Fatal("two selected candidates")
	}
	productProv := ProductSpec{ID: "sample", Selection: contract.SelectionSpec{UseWhen: []string{"u"}}, FieldProvenance: map[string]contract.FieldProvenance{"use_when": {
		Value: json.RawMessage(`["nope"]`), Source: "contract_final", Precedence: "100", Resolution: "selected",
		Candidates: []contract.FieldCandidateProvenance{{Value: json.RawMessage(`["u"]`), Source: "contract_final", Precedence: "100", Selected: &selected}},
	}}}
	if _, err := (SchemaRegistry{Products: []ProductSpec{productProv}}).Index(); err == nil {
		t.Fatal("product provenance mismatch")
	}
	if _, err := (SchemaRegistry{Kind: "schema", Products: []ProductSpec{{ID: "mail", Description: "mail product"}}}).ToOverviewPayload(); err != nil {
		t.Fatal(err)
	}
	weirdJSON := base
	if _, err := (SchemaRegistry{Products: []ProductSpec{{
		ID: "sample", Tools: []ToolSpec{weirdJSON},
		FieldProvenance: map[string]contract.FieldProvenance{"custom": {Value: json.RawMessage(`{`)}},
	}}}).ToPayload(); err == nil {
		t.Fatal("invalid product provenance json")
	}

	built, meta := buildFixtureCache(t, allFieldsRegistry())
	productID := meta.ProductDescriptors[0].ProductID
	var samplePath string
	for path, id := range meta.LocatorProductByPath {
		if id == productID {
			samplePath = path
			break
		}
	}
	if samplePath == "" {
		t.Fatal("missing locator path")
	}
	miss := meta
	miss.LocatorProductByPath = map[string]string{}
	for path, id := range meta.LocatorProductByPath {
		miss.LocatorProductByPath[path] = id
	}
	miss.LocatorProductByPath["ghost"] = productID
	if _, ok := miss.CommandMeta("ghost"); ok {
		t.Fatal("path missing from shard")
	}
	emptyKey := meta
	emptyShard := proto.Clone(meta.commandEntryShards[0]).(*schemacachepb.CommandMetaEntryShard)
	var emptyList schemacachepb.CommandMetaEntryList
	if err := proto.Unmarshal(emptyShard.Entries, &emptyList); err != nil {
		t.Fatal(err)
	}
	emptyList.Items[0].LookupPath = ""
	emptyBlob, err := MarshalSchemaCacheDeterministic(&emptyList)
	if err != nil {
		t.Fatal(err)
	}
	emptyShard.Entries = emptyBlob
	emptyKey.commandEntryShards = []*schemacachepb.CommandMetaEntryShard{emptyShard}
	if _, ok := emptyKey.CommandMeta(samplePath); ok {
		t.Fatal("empty lookup path")
	}
	wrongProduct := meta
	wrongShard := proto.Clone(meta.commandEntryShards[0]).(*schemacachepb.CommandMetaEntryShard)
	var wrongList schemacachepb.CommandMetaEntryList
	if err := proto.Unmarshal(wrongShard.Entries, &wrongList); err != nil {
		t.Fatal(err)
	}
	wrongList.Items[0].ProductId = "other"
	wrongBlob, err := MarshalSchemaCacheDeterministic(&wrongList)
	if err != nil {
		t.Fatal(err)
	}
	wrongShard.Entries = wrongBlob
	wrongProduct.commandEntryShards = []*schemacachepb.CommandMetaEntryShard{wrongShard}
	if _, ok := wrongProduct.CommandMeta(samplePath); ok {
		t.Fatal("identity product mismatch")
	}
	aliasMiss := meta
	aliasShard := proto.Clone(meta.commandEntryShards[0]).(*schemacachepb.CommandMetaEntryShard)
	var aliasList schemacachepb.CommandMetaEntryList
	if err := proto.Unmarshal(aliasShard.Entries, &aliasList); err != nil {
		t.Fatal(err)
	}
	if len(aliasList.Items[0].Aliases) > 0 {
		aliasMiss.LocatorProductByPath = map[string]string{}
		for path, id := range meta.LocatorProductByPath {
			aliasMiss.LocatorProductByPath[path] = id
		}
		aliasMiss.LocatorProductByPath[aliasList.Items[0].Aliases[0]] = "other"
		aliasMiss.commandEntryShards = []*schemacachepb.CommandMetaEntryShard{aliasShard}
		if _, ok := aliasMiss.CommandMeta(samplePath); ok {
			t.Fatal("alias locator mismatch")
		}
	}
	corrupt := meta
	corruptShard := proto.Clone(meta.commandEntryShards[0]).(*schemacachepb.CommandMetaEntryShard)
	corruptShard.Entries = []byte("not-proto")
	corrupt.commandEntryShards = []*schemacachepb.CommandMetaEntryShard{corruptShard}
	if _, ok := corrupt.CommandMeta(samplePath); ok {
		t.Fatal("corrupt shard entries")
	}
	badMaterialize := meta
	badMaterialize.CommandMetaByPath = map[string]CommandMeta{}
	badMaterialize.commandEntryShards = []*schemacachepb.CommandMetaEntryShard{corruptShard}
	badMaterialize.MaterializeCommandMeta()
	count := meta
	countShard := proto.Clone(meta.commandEntryShards[0]).(*schemacachepb.CommandMetaEntryShard)
	countShard.EntryCount++
	count.commandEntryShards = []*schemacachepb.CommandMetaEntryShard{countShard}
	if _, ok := count.CommandMeta(samplePath); ok {
		t.Fatal("entry count mismatch")
	}

	var message schemacachepb.SchemaMetaCache
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Overview.Registry = nil
	payload, err := MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("overview missing registry")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Overview.Products = nil
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("overview missing products")
	}
	if err := proto.Unmarshal(built.Meta, &message); err != nil {
		t.Fatal(err)
	}
	message.Locators.Items[0].ProductId = "missing-product"
	payload, err = MarshalSchemaCacheDeterministic(&message)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaMetaCache(payload); err == nil {
		t.Fatal("locator unknown product")
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
	payloadRoot.Entries.Items[0].Identity = nil
	badHeader, err := MarshalSchemaCacheDeterministic(&payloadRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCommandPayloadHeader(badHeader, desc); err == nil {
		t.Fatal("missing payload identity")
	}
	if err := proto.Unmarshal(header, &payloadRoot); err != nil {
		t.Fatal(err)
	}
	payloadRoot.Entries.Items[0].Identity.CliPath = ""
	badHeader, err = MarshalSchemaCacheDeterministic(&payloadRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCommandPayloadHeader(badHeader, desc); err == nil {
		t.Fatal("incomplete payload identity")
	}
	if err := proto.Unmarshal(header, &payloadRoot); err != nil {
		t.Fatal(err)
	}
	payloadRoot.RenderedLeafIndex.Items[0].Sha256 = []byte{1}
	badHeader, err = MarshalSchemaCacheDeterministic(&payloadRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCommandPayloadHeader(badHeader, desc); err == nil {
		t.Fatal("short rendered leaf digest")
	}
	if err := proto.Unmarshal(header, &payloadRoot); err != nil {
		t.Fatal(err)
	}
	payloadRoot.RenderedLeafIndex.Items[0].CanonicalPath = ""
	badHeader, err = MarshalSchemaCacheDeterministic(&payloadRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCommandPayloadHeader(badHeader, desc); err == nil {
		t.Fatal("empty rendered leaf path")
	}

	invalidBlob := []byte("x\n")
	if err := proto.Unmarshal(header, &payloadRoot); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(invalidBlob)
	payloadRoot.RenderedLeafIndex.Items[0].Sha256 = digest[:]
	payloadRoot.RenderedLeafIndex.Items[0].Offset = 0
	payloadRoot.RenderedLeafIndex.Items[0].Length = uint64(len(invalidBlob))
	badHeader, err = MarshalSchemaCacheDeterministic(&payloadRoot)
	if err != nil {
		t.Fatal(err)
	}
	assembled, err := assembleCommandPayloadShard(badHeader, invalidBlob)
	if err != nil {
		t.Fatal(err)
	}
	updated := desc
	updated.Length = uint64(len(assembled))
	updated.SHA256 = sha256.Sum256(assembled)
	updated.HeaderLength = uint64(4 + len(badHeader))
	updated.HeaderSHA256 = sha256.Sum256(assembled[:updated.HeaderLength])
	updatedMeta := meta
	updatedMeta.PayloadDescriptors = []CommandPayloadDescriptor{updated}
	if _, err := DecodeSchemaCommandPayloadCache(assembled, updated, updatedMeta); err == nil {
		t.Fatal("invalid rendered json")
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
	index.Locators.Items[0].ProductId = ""
	badIndex, err := MarshalSchemaCacheDeterministic(&index)
	if err != nil {
		t.Fatal(err)
	}
	assembled, err = assembleCommandPayloadShard(badIndex, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaPayloadIndex(assembled); err == nil {
		t.Fatal("incomplete payload index locator")
	}
	if err := proto.Unmarshal(indexHeader, &index); err != nil {
		t.Fatal(err)
	}
	index.Products.Items[0].HeaderLength = 0
	badIndex, err = MarshalSchemaCacheDeterministic(&index)
	if err != nil {
		t.Fatal(err)
	}
	assembled, err = assembleCommandPayloadShard(badIndex, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSchemaPayloadIndex(assembled); err == nil {
		t.Fatal("incomplete payload index descriptor")
	}

	_ = blobs
	if err := rejectUnknownFieldsAndEnums(&schemacachepb.SchemaCommandPayloadCache{}); err != nil {
		t.Fatal(err)
	}
	if err := rejectUnknownFieldsAndEnumsReflect((&schemacachepb.BytesValue{}).ProtoReflect()); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildSchemaCache(SchemaRegistry{Products: []ProductSpec{{}}}, nil, SchemaOverview{}, nil, CacheHashes{}, nil); err == nil {
		t.Fatal("invalid registry build")
	}
	empty := SchemaRegistry{Kind: "schema"}
	if _, err := empty.Index(); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildSchemaCache(empty, map[string]CommandMeta{}, SchemaOverview{}, map[string]string{}, CacheHashes{}, map[string][]byte{}); err == nil {
		t.Fatal("empty product build")
	}
	notSelected := false
	if err := validateFinalFieldProvenance("tool sample.run", "title", contract.FieldProvenance{
		Value: json.RawMessage(`"Run sample"`), Source: "contract_final", Precedence: "100", Resolution: "selected",
		Candidates: []contract.FieldCandidateProvenance{{Value: json.RawMessage(`"Run sample"`), Source: "other", Precedence: "1", Selected: &notSelected}},
	}, "Run sample"); err == nil {
		t.Fatal("zero selected provenance candidates")
	}
}
