// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/schemaruntime"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/schemacache"
)

func tinyDeliveryRegistry() SchemaRegistry {
	return SchemaRegistry{
		Kind: "schema", Level: "catalog", Source: "test",
		Products: []ProductSpec{{
			ID: "sample",
			Tools: []ToolSpec{{
				Identity: contract.ToolIdentitySpec{
					ProductID: "sample", Name: "run", CanonicalPath: "sample.run", CLIPath: "sample run",
				},
				Title: "Run", Description: "Runs sample",
			}},
		}},
	}
}

func publishTinyDeliveryCache(t *testing.T) (*schemacache.Cache, SchemaCacheIdentity, SchemaCacheArtifacts) {
	t.Helper()
	if !((runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") || (runtime.GOOS == "linux" && runtime.GOARCH == "amd64")) {
		t.Skip("unix schema cache")
	}
	registry := tinyDeliveryRegistry()
	source := sha256.Sum256([]byte("remaining-delivery-source"))
	surface := sha256.Sum256([]byte("remaining-delivery-surface"))
	artifacts, err := buildSchemaCacheArtifacts(registry, "sha256:"+hex.EncodeToString(source[:]), "sha256:"+hex.EncodeToString(surface[:]))
	if err != nil {
		t.Fatal(err)
	}
	indexLength, indexDigest, err := artifacts.PayloadIndexPins()
	if err != nil {
		t.Fatal(err)
	}
	identity := SchemaCacheIdentity{
		Edition: "official", CatalogSnapshotVersion: uint32(artifacts.Version),
		SourceSHA256: source, SurfaceSHA256: surface, BuildID: sha256.Sum256([]byte("remaining-delivery-build")),
		Meta: artifacts.MetaArtifact().Expectation, Registry: artifacts.RegistryArtifact().Expectation,
		Payload: artifacts.PayloadArtifact().Expectation, PayloadIndexLength: indexLength, PayloadIndexSHA256: indexDigest,
	}
	if err := identity.Validate(); err != nil {
		t.Fatal(err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp(home, ".dws-schemacache-remaining-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	if err := os.Chmod(base, 0o700); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DWS_SCHEMA_CACHE_DIR", resolved)
	cache, err := schemacache.Open(identity.Edition)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	if err := cache.Publish(identity.ExpectedIdentity(), artifacts.RegistryArtifact(), artifacts.MetaArtifact(), artifacts.PayloadArtifact()); err != nil {
		t.Fatal(err)
	}
	return cache, identity, artifacts
}

func TestCrossPlatformCoverageSchemaCacheDeliveryRepairCatalogErrorsAndRoundTrip(t *testing.T) {
	previous := schemaCacheRegistrationValue.Load()
	uncertain := schemaCacheRuntimeUncertain.Load()
	previousLive := runtimeDeliveryLiveCatalog.Load()
	previousErr := runtimeDeliverySchemaCatalogErr
	t.Cleanup(func() {
		schemaCacheRuntimeUncertain.Store(uncertain)
		schemaCacheRegistrationValue.Store(previous)
		runtimeDeliveryLiveCatalog.Store(previousLive)
		runtimeDeliverySchemaCatalogErr = previousErr
		renderSchemaProductSummary = func(product ProductSpec) (map[string]any, error) { return product.ToSummaryPayload() }
	})

	valid := "sha256:" + hex.EncodeToString(make([]byte, 32))
	if _, err := buildSchemaCacheArtifacts(SchemaRegistry{AgentMetadata: json.RawMessage(`{`)}, valid, valid); err == nil {
		t.Fatal("canonical invalid json")
	}
	if _, err := buildSchemaCacheArtifacts(SchemaRegistry{Products: []ProductSpec{{ID: ""}}}, valid, valid); err == nil {
		t.Fatal("index empty product id")
	}
	if _, err := canonicalSchemaCacheRegistry(SchemaRegistry{
		Products: []ProductSpec{{
			FieldProvenance: map[string]contract.FieldProvenance{
				"title": {OverriddenCandidates: []contract.FieldCandidateProvenance{{Value: json.RawMessage(`"old"`)}}},
			},
		}},
	}); err != nil {
		t.Fatalf("overridden candidates: %v", err)
	}
	if _, err := renderCompactSchemaLeaves(SchemaRegistry{Products: []ProductSpec{{
		Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{}}},
	}}}, SchemaIndex{}); err != nil {
		t.Fatalf("empty leaf paths: %v", err)
	}
	if _, err := renderCompactSchemaLeaves(tinyDeliveryRegistry(), SchemaIndex{}); err == nil {
		t.Fatal("render compact without index")
	}

	cache, identity, artifacts := publishTinyDeliveryCache(t)
	r := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	r.openOnce.Do(func() { r.cache = cache })

	index, err := r.loadPayloadIndex()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.loadCommandPayload(index, "sample")
		}()
	}
	wg.Wait()

	if _, _, err := r.resolveCommandMetaFromPayload("ghost"); err != nil {
		t.Fatalf("unknown path: %v", err)
	}
	badIndexRuntime := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	badIndexRuntime.openOnce.Do(func() { badIndexRuntime.cache = cache })
	badIndexRuntime.indexOnce.Do(func() { badIndexRuntime.indexErr = errors.New("index") })
	if _, _, err := badIndexRuntime.resolveCommandMetaFromPayload("sample run"); err == nil {
		t.Fatal("resolve with index error")
	}
	if _, ok := badIndexRuntime.renderedCompactLeaf("sample.run"); ok {
		t.Fatal("rendered with index error")
	}
	if _, ok := r.renderedCompactLeaf("ghost"); ok {
		t.Fatal("rendered unknown path")
	}

	closed := identity
	closed.Edition = "closeded"
	if err := closed.Validate(); err != nil {
		t.Fatal(err)
	}
	closedCache, err := schemacache.Open(closed.Edition)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closedCache.Close() })
	freshMiss := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: closed, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	freshMiss.openOnce.Do(func() { freshMiss.cache = closedCache })
	if _, err := freshMiss.readCommandMetaFromPayloadFresh("sample run"); err == nil {
		t.Fatal("fresh meta without payloads")
	}

	payloadMiss := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Millisecond},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	payloadMiss.openOnce.Do(func() { payloadMiss.cache = cache })
	payloadMiss.payloadMu.Lock()
	payloadMiss.payloads["sample"] = &schemaCachePayloadLoad{}
	payloadMiss.payloads["sample"].err = errors.New("payload")
	payloadMiss.payloads["sample"].ready.Store(true)
	payloadMiss.payloadMu.Unlock()
	if _, _, err := payloadMiss.resolveCommandMetaFromPayload("sample run"); err == nil {
		t.Fatal("payload load error")
	}
	if _, ok := payloadMiss.renderedCompactLeaf("sample.run"); ok {
		t.Fatal("rendered payload error")
	}

	meta, err := schemaruntime.DecodeSchemaMetaCache(artifacts.Meta)
	if err != nil {
		t.Fatal(err)
	}
	prevSummary := renderSchemaProductSummary
	renderSchemaProductSummary = func(ProductSpec) (map[string]any, error) { return nil, errors.New("summary") }
	if _, err := r.queryPayload(meta, "sample", true); err == nil {
		t.Fatal("query product summary")
	}
	idx, err := tinyDeliveryRegistry().Index()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := schemaPayloadFromLoadedCatalog(loadedSchemaCatalog{Registry: tinyDeliveryRegistry(), Index: idx}, []string{"sample"}); err == nil {
		t.Fatal("loaded catalog summary")
	}
	renderSchemaProductSummary = prevSummary

	corruptMeta := meta
	if len(corruptMeta.ProductDescriptors) > 0 {
		corruptMeta.ProductDescriptors[0].SHA256[0] ^= 1
		if _, err := r.readAllPayload(corruptMeta, false); err == nil {
			t.Fatal("all payload digest")
		}
	}

	_ = deliverySchemaCatalog()
	runtimeDeliveryLiveCatalog.Store(nil)
	runtimeDeliverySchemaCatalogErr = errors.New("catalog down")
	openFailed := &schemaCacheRuntime{options: SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Millisecond}}
	openFailed.openOnce.Do(func() { openFailed.openErr = errors.New("open failed") })
	if _, _, err := repairSchemaCache(openFailed, func() (any, error) { return nil, errors.New("recheck") }); err == nil {
		t.Fatal("repair without lock")
	}
	if _, err := deliverySchemaAllPayload(); err == nil {
		t.Fatal("all repair catalog error")
	}

	locked := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	locked.openOnce.Do(func() { locked.cache = cache })
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true, Identity: identity},
		runtime: locked,
	})
	runtimeDeliveryLiveCatalog.Store(nil)
	if _, _, err := repairSchemaCache(locked, func() (any, error) { return nil, errors.New("stale") }); err == nil {
		t.Fatal("repair lock catalog error")
	}

	empty := identity
	empty.Edition = "emptyed"
	if err := empty.Validate(); err != nil {
		t.Fatal(err)
	}
	emptyCache, err := schemacache.Open(empty.Edition)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = emptyCache.Close() })
	overviewFail := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: empty, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	overviewFail.openOnce.Do(func() { overviewFail.cache = emptyCache })
	overviewFail.metaOnce.Do(func() { overviewFail.metaErr = errors.New("stale meta") })
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true, Identity: empty},
		runtime: overviewFail,
	})
	runtimeDeliveryLiveCatalog.Store(nil)
	if _, err := deliverySchemaOverviewPayload(); err == nil {
		t.Fatal("overview repair catalog error")
	}

	queryFail := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: empty, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	queryFail.openOnce.Do(func() { queryFail.cache = emptyCache })
	queryFail.metaOnce.Do(func() { queryFail.metaErr = errors.New("stale meta") })
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true, Identity: empty},
		runtime: queryFail,
	})
	runtimeDeliveryLiveCatalog.Store(nil)
	if _, err := queryDeliverySchemaPayload([]string{"sample run"}); err == nil {
		t.Fatal("query repair catalog error")
	}

	runtimeDeliverySchemaCatalogErr = previousErr
	queryHit := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	queryHit.openOnce.Do(func() { queryHit.cache = cache })
	queryHit.metaOnce.Do(func() { queryHit.metaErr = errors.New("stale meta") })
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true, Identity: identity},
		runtime: queryHit,
	})
	runtimeDeliveryLiveCatalog.Store(nil)
	if _, err := queryDeliverySchemaPayload([]string{"sample run"}); err != nil {
		t.Fatalf("query repair recheck: %v", err)
	}

	drift := artifacts
	drift.registry = SchemaRegistry{Kind: "schema"}
	if err := drift.ValidateRoundTrip(); err == nil {
		t.Fatal("round trip count")
	}
	identityDrift := artifacts
	identityDrift.registry = tinyDeliveryRegistry()
	identityDrift.registry.Products[0].Tools[0].Title = "other"
	if err := identityDrift.ValidateRoundTrip(); err == nil {
		t.Fatal("round trip identity")
	}
	locatorDrift := artifacts
	locatorDrift.locators = map[string]string{"ghost": "sample"}
	if err := locatorDrift.ValidateRoundTrip(); err == nil {
		t.Fatal("round trip locators")
	}
	overviewDrift := artifacts
	overviewDrift.registry = SchemaRegistry{Kind: "other", Products: tinyDeliveryRegistry().Products}
	if err := overviewDrift.ValidateRoundTrip(); err == nil {
		t.Fatal("round trip overview")
	}
	registryDrift := artifacts
	registryDrift.Registry = []byte("not-registry")
	if err := registryDrift.ValidateRoundTrip(); err == nil {
		t.Fatal("round trip decode registry")
	}
	decodedDrift := artifacts
	decodedDrift.registry.Kind = "mutated"
	if err := decodedDrift.ValidateRoundTrip(); err == nil {
		t.Fatal("round trip registry value")
	}
	queryDrift := artifacts
	queryDrift.index = SchemaIndex{}
	if err := queryDrift.ValidateRoundTrip(); err == nil {
		t.Fatal("round trip query")
	}
	overviewJSON := artifacts
	overviewJSON.registry.AgentMetadata = json.RawMessage(`{`)
	if err := overviewJSON.ValidateRoundTrip(); err == nil {
		t.Fatal("round trip overview encode")
	}

	leafRuntime := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	leafRuntime.openOnce.Do(func() { leafRuntime.cache = cache })
	index, err = leafRuntime.readPayloadIndex()
	if err != nil {
		t.Fatal(err)
	}
	leafRuntime.seedPayloadIndex(index)
	leafRuntime.payloadMu.Lock()
	missingLeaves := &schemaCachePayloadLoad{payloads: schemaruntime.DecodedCommandPayloads{
		ProductID: "sample",
		Commands: map[string]schemaruntime.CommandMeta{
			"sample run": {Identity: schemaruntime.CommandIdentity{CLIPath: "sample run", Canonical: "sample.run", ProductID: "sample"}},
		},
	}}
	missingLeaves.ready.Store(true)
	leafRuntime.payloads["sample"] = missingLeaves
	leafRuntime.payloadMu.Unlock()
	if _, ok := leafRuntime.renderedCompactLeaf("sample run"); ok {
		t.Fatal("rendered leaf miss")
	}
	leafRuntime.payloadMu.Lock()
	badRef := &schemaCachePayloadLoad{payloads: schemaruntime.DecodedCommandPayloads{
		ProductID: "sample",
		Commands: map[string]schemaruntime.CommandMeta{
			"sample.run": {Identity: schemaruntime.CommandIdentity{CLIPath: "sample run", Canonical: "sample.run", ProductID: "sample"}},
		},
		LeafIndex: []schemaruntime.RenderedLeafRef{{CanonicalPath: "sample.run", Offset: 1 << 40, Length: 8}},
	}}
	badRef.ready.Store(true)
	leafRuntime.payloads["sample"] = badRef
	leafRuntime.payloadMu.Unlock()
	if _, ok := leafRuntime.renderedCompactLeaf("sample.run"); ok {
		t.Fatal("rendered leaf range")
	}
}
