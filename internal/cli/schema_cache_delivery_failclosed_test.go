// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/schemaruntime"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/schemacache"
)

func TestCrossPlatformCoverageSchemaCacheDeliveryFailClosedHelpers(t *testing.T) {
	previous := schemaCacheRegistrationValue.Load()
	uncertain := schemaCacheRuntimeUncertain.Load()
	t.Cleanup(func() {
		schemaCacheRuntimeUncertain.Store(uncertain)
		schemaCacheRegistrationValue.Store(previous)
	})

	if _, err := schemaCacheHashes("nope", "nope"); err == nil {
		t.Fatal("invalid hash prefix")
	}
	badHex := "sha256:" + strings.Repeat("gg", 32)
	if _, err := schemaCacheHashes(badHex, badHex); err == nil {
		t.Fatal("invalid hash hex")
	}
	valid := "sha256:" + hex.EncodeToString(make([]byte, 32))
	if _, err := schemaCacheHashes(valid, "sha256:ff"); err == nil {
		t.Fatal("short surface hash")
	}

	if _, _, err := (SchemaCacheArtifacts{}).PayloadIndexPins(); err == nil {
		t.Fatal("short payload pins")
	}
	oversized := make([]byte, 8)
	oversized[3] = 20
	if _, _, err := (SchemaCacheArtifacts{Payload: oversized}).PayloadIndexPins(); err == nil {
		t.Fatal("payload index overflow")
	}

	if _, err := canonicalSchemaCacheRegistry(SchemaRegistry{AgentMetadata: json.RawMessage(`{`)}); err == nil {
		t.Fatal("invalid agent metadata")
	}
	if _, err := canonicalSchemaCacheRegistry(SchemaRegistry{AgentMetadata: json.RawMessage(`1 2`)}); err == nil {
		t.Fatal("trailing json")
	}
	if _, err := canonicalSchemaCacheRegistry(SchemaRegistry{
		Products: []ProductSpec{{
			FieldProvenance: map[string]contract.FieldProvenance{"title": {Value: json.RawMessage(`{`)}},
		}},
	}); err == nil {
		t.Fatal("invalid provenance json")
	}
	if _, err := canonicalSchemaCacheRegistry(SchemaRegistry{
		Products: []ProductSpec{{
			FieldProvenance: map[string]contract.FieldProvenance{
				"title": {Candidates: []contract.FieldCandidateProvenance{{Value: json.RawMessage(`{`)}}},
			},
		}},
	}); err == nil {
		t.Fatal("invalid candidate json")
	}
	if _, err := canonicalSchemaCacheRegistry(SchemaRegistry{
		Products: []ProductSpec{{
			Tools: []ToolSpec{{
				Parameters: []ParameterSpec{{Name: "id", Default: json.RawMessage(`{`)}},
			}},
		}},
	}); err == nil {
		t.Fatal("invalid parameter default json")
	}
	if _, err := canonicalSchemaCacheRegistry(SchemaRegistry{
		Products: []ProductSpec{{
			Tools: []ToolSpec{{
				Result: &contract.ResultSpec{DataSchema: json.RawMessage(`{`)},
			}},
		}},
	}); err == nil {
		t.Fatal("invalid result schema json")
	}

	if err := RegisterSchemaCacheOptions(SchemaCacheOptions{Enabled: true, GOOS: "windows", GOARCH: "amd64"}); err == nil {
		t.Fatal("unsupported platform")
	}
	PrewarmSchemaCache()
	_, _ = DeliverySchemaCacheArtifactsForTest()
	AwaitSchemaCachePrewarmForTest()
	if SchemaCachePrewarmPayloadsHandleForTest() != nil {
		t.Fatal("missing prewarm handle")
	}
	if normalizeSchemaQueryCLIPath("dws/sample/run") != "sample run" {
		t.Fatal("query path wrapper")
	}
	if got := stripSchemaParamCompact(map[string]any{"type": "string", "property": "drop"}); got["type"] != "string" {
		t.Fatalf("compact wrapper = %#v", got)
	}

	schemaCacheRegistrationValue.Store(nil)
	PrewarmSchemaCache()
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true, RuntimeEligible: func() bool { return false }},
		runtime: &schemaCacheRuntime{},
	})
	PrewarmSchemaCache()
	done := make(chan struct{})
	close(done)
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true},
		runtime: &schemaCacheRuntime{prewarm: &schemaCachePrewarm{done: done}},
	})
	PrewarmSchemaCache()
	schemaCacheRuntimeUncertain.Store(true)
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true},
		runtime: &schemaCacheRuntime{},
	})
	PrewarmSchemaCache()
	schemaCacheRuntimeUncertain.Store(false)

	r := &schemaCacheRuntime{
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	r.seedAll(map[string]any{"kind": "schema"})
	r.seedMeta(schemaruntime.DecodedSchemaMeta{Kind: "schema"})
	r.seedPayloadIndex(schemaruntime.DecodedSchemaPayloadIndex{})
	if got, err := r.loadAllPayload(); err != nil || got["kind"] != "schema" {
		t.Fatalf("seeded all = %#v %v", got, err)
	}
	if got, err := r.loadMeta(); err != nil || got.Kind != "schema" {
		t.Fatalf("seeded meta = %#v %v", got, err)
	}
	if _, err := r.loadPayloadIndex(); err != nil {
		t.Fatalf("seeded payload index: %v", err)
	}
	if _, err := r.cachedProduct("missing"); err == nil {
		t.Fatal("missing product")
	}

	openFailed := &schemaCacheRuntime{
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	openFailed.openOnce.Do(func() { openFailed.openErr = errors.New("open failed") })
	if _, err := openFailed.readMeta(); err == nil {
		t.Fatal("readMeta open")
	}
	if _, err := openFailed.readPayloadIndex(); err == nil {
		t.Fatal("readPayloadIndex open")
	}
	if _, err := openFailed.readProduct(schemaruntime.DecodedSchemaMeta{}, "drive"); err == nil {
		t.Fatal("readProduct open")
	}
	if _, err := openFailed.payloadsHandle(); err == nil {
		t.Fatal("payloadsHandle open")
	}
	if _, _, err := openFailed.resolveCommandMetaFromPayload("drive list"); err == nil {
		t.Fatal("resolveCommandMeta open")
	}
	if _, ok := openFailed.renderedCompactLeaf("drive.list"); ok {
		t.Fatal("renderedCompactLeaf open")
	}
	if _, err := openFailed.loadOverviewPayload(); err == nil {
		t.Fatal("loadOverview open")
	}
	if _, err := openFailed.loadQueryPayload("drive list"); err == nil {
		t.Fatal("loadQuery open")
	}
	if _, err := openFailed.readQueryPayload("drive list"); err == nil {
		t.Fatal("readQuery open")
	}
	if _, err := openFailed.readCommandMetaFromPayloadFresh("drive list"); err == nil {
		t.Fatal("fresh command meta open")
	}
	if _, err := openFailed.readAllPayload(schemaruntime.DecodedSchemaMeta{}, true); err == nil {
		t.Fatal("readAll open")
	}
	if _, err := openFailed.readRenderedLeaf(schemaruntime.DecodedSchemaPayloadIndex{}, "drive", schemaruntime.RenderedLeafRef{}); err == nil {
		t.Fatal("readRenderedLeaf open")
	}
	meta := schemaruntime.DecodedSchemaMeta{LocatorProductByPath: map[string]string{"drive list": "drive"}}
	if _, err := openFailed.queryPayload(meta, "drive list", true); err == nil {
		t.Fatal("queryPayload cached open")
	}
	if _, err := openFailed.queryPayload(meta, "drive list", false); err == nil {
		t.Fatal("queryPayload uncached open")
	}

	if _, err := r.queryPayload(schemaruntime.DecodedSchemaMeta{}, "missing", true); err == nil {
		t.Fatal("unknown locator")
	}
	if _, err := r.overviewPayload(schemaruntime.DecodedSchemaMeta{Overview: schemaruntime.SchemaOverview{Products: []schemaruntime.OverviewProduct{{
		ID: "drive", Summary: "stale",
	}}}}); err == nil {
		t.Fatal("overview without kind")
	}
	_ = r.trustedHashes()
	_, _ = schemaCacheLocator(schemaruntime.DecodedSchemaMeta{}, "x")
	_, _ = r.descriptor(schemaruntime.DecodedSchemaMeta{}, "drive")

	ready := &schemaCachePayloadLoad{payloads: schemaruntime.DecodedCommandPayloads{}}
	ready.ready.Store(true)
	r.payloads["drive"] = ready
	if _, err := r.loadCommandPayload(schemaruntime.DecodedSchemaPayloadIndex{}, "drive"); err != nil {
		t.Fatalf("ready payload: %v", err)
	}
	if _, err := openFailed.loadCommandPayload(schemaruntime.DecodedSchemaPayloadIndex{}, "drive"); err == nil {
		t.Fatal("loadCommandPayload open")
	}

	handle := &schemacache.Registry{}
	cachedHandle := &schemaCacheRuntime{payloadHandle: handle}
	got, err := cachedHandle.payloadsHandle()
	if err != nil || got != handle {
		t.Fatalf("cached payloads handle = %v %v", got, err)
	}
	prewarmed := &schemaCacheRuntime{prewarm: &schemaCachePrewarm{done: done, payloads: handle}}
	got, err = prewarmed.payloadsHandle()
	if err != nil || got != handle {
		t.Fatalf("prewarm payloads handle = %v %v", got, err)
	}
	reset := &schemaCacheRuntime{
		payloadHandle: &schemacache.Registry{},
		prewarm:       &schemaCachePrewarm{done: done, payloads: &schemacache.Registry{}},
	}
	reset.resetPayloadsHandle()

	failedPrewarm := &schemaCacheRuntime{prewarm: &schemaCachePrewarm{done: done, indexErr: errors.New("idx")}}
	failedPrewarm.openOnce.Do(func() { failedPrewarm.openErr = errors.New("open failed") })
	if _, err := failedPrewarm.opened(); err == nil {
		t.Fatal("failed prewarm opened")
	}
	if _, err := failedPrewarm.loadPayloadIndex(); err == nil {
		t.Fatal("failed prewarm index")
	}
	okPrewarm := &schemaCacheRuntime{prewarm: &schemaCachePrewarm{done: done, index: schemaruntime.DecodedSchemaPayloadIndex{}}}
	if _, err := okPrewarm.loadPayloadIndex(); err != nil {
		t.Fatalf("prewarm index: %v", err)
	}

	artifacts := SchemaCacheArtifacts{Meta: []byte("m"), Registry: []byte("r"), Payload: []byte("p")}
	_ = artifacts.MetaArtifact()
	_ = artifacts.RegistryArtifact()
	_ = artifacts.PayloadArtifact()
	if artifacts.match(SchemaCacheIdentity{}) {
		t.Fatal("empty identity must not match")
	}

	if loaded := runtimeDeliveryLiveCatalog.Load(); loaded != nil {
		value, _, err := repairSchemaCache(&schemaCacheRuntime{}, func() (any, error) {
			t.Fatal("recheck must not run when a live catalog is already published")
			return nil, nil
		})
		if err != nil || value != nil {
			t.Fatalf("repair with live catalog = %v %v", value, err)
		}
	}

	previousLive := runtimeDeliveryLiveCatalog.Swap(nil)
	t.Cleanup(func() { runtimeDeliveryLiveCatalog.Store(previousLive) })
	openFailedRuntime := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	openFailedRuntime.openOnce.Do(func() { openFailedRuntime.openErr = errors.New("open failed") })
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true},
		runtime: openFailedRuntime,
	})
	if _, err := deliverySchemaAllPayload(); err != nil {
		t.Fatalf("all repair without live catalog: %v", err)
	}
	runtimeDeliveryLiveCatalog.Store(nil)
	if _, err := deliverySchemaOverviewPayload(); err != nil {
		t.Fatalf("overview repair without live catalog: %v", err)
	}
	runtimeDeliveryLiveCatalog.Store(nil)
	runtimeDeliveryLiveCatalog.Store(nil)
	_, _ = queryDeliverySchemaPayload([]string{"sample group run"})
	runtimeDeliveryLiveCatalog.Store(nil)
	_, _ = queryDeliverySchemaPayload(nil)
	runtimeDeliveryLiveCatalog.Store(nil)
	ResolveMeta("sample group run")

	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		runtime: &schemaCacheRuntime{prewarm: &schemaCachePrewarm{done: done, payloads: handle}},
	})
	if SchemaCachePrewarmPayloadsHandleForTest() != handle {
		t.Fatal("prewarm payloads handle")
	}
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{runtime: &schemaCacheRuntime{}})
	if SchemaCachePrewarmPayloadsHandleForTest() != nil {
		t.Fatal("runtime without prewarm")
	}
}
