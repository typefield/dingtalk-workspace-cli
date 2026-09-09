// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/schemaruntime"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/schemacache"
)

func TestCrossPlatformCoverageSchemaCacheDeliveryHitRepairAndPlatformFill(t *testing.T) {
	previous := schemaCacheRegistrationValue.Load()
	uncertain := schemaCacheRuntimeUncertain.Load()
	previousLive := runtimeDeliveryLiveCatalog.Load()
	t.Cleanup(func() {
		schemaCacheRuntimeUncertain.Store(uncertain)
		schemaCacheRegistrationValue.Store(previous)
		runtimeDeliveryLiveCatalog.Store(previousLive)
	})

	if err := RegisterSchemaCacheOptions(SchemaCacheOptions{Enabled: true}); err == nil {
		t.Fatal("empty identity must fail after filling GOOS/GOARCH")
	}

	dummy := &loadedSchemaCatalog{}
	runtimeDeliveryLiveCatalog.Store(dummy)
	value, loaded, err := repairSchemaCache(&schemaCacheRuntime{}, func() (any, error) {
		t.Fatal("recheck must not run when a live catalog is already published")
		return nil, nil
	})
	if err != nil || value != nil || loaded.Registry.Kind != dummy.Registry.Kind {
		t.Fatalf("repair with dummy live catalog = %#v %#v %v", value, loaded, err)
	}

	valid := "sha256:" + hex.EncodeToString(make([]byte, 32))
	if _, err := buildSchemaCacheArtifacts(SchemaRegistry{Products: []ProductSpec{{}}}, valid, valid); err == nil {
		t.Fatal("empty product id artifacts")
	}

	if !((runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") || (runtime.GOOS == "linux" && runtime.GOARCH == "amd64")) {
		return
	}

	registry := SchemaRegistry{
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
	source := sha256.Sum256([]byte("delivery-hit-source"))
	surface := sha256.Sum256([]byte("delivery-hit-surface"))
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
		SourceSHA256: source, SurfaceSHA256: surface, BuildID: sha256.Sum256([]byte("delivery-hit-build")),
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
	base, err := os.MkdirTemp(home, ".dws-schemacache-delivery-")
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

	r := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	r.openOnce.Do(func() { r.cache = cache })

	meta, err := schemaruntime.DecodeSchemaMetaCache(artifacts.Meta)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := schemaruntime.DecodeSchemaProductFromShards(artifacts.Registry, meta, "sample")
	if err != nil {
		t.Fatal(err)
	}
	r.storeProduct("sample", decoded)
	if _, err := r.queryPayload(meta, "sample run", true); err != nil {
		t.Fatalf("queryPayload hit: %v", err)
	}
	if _, err := r.queryPayload(schemaruntime.DecodedSchemaMeta{LocatorProductByPath: map[string]string{"ghost": "sample"}}, "ghost", true); err == nil {
		t.Fatal("locator hit with unknown path")
	}

	r.seedMeta(meta)
	if _, err := r.loadOverviewPayload(); err != nil {
		t.Fatalf("overview hit: %v", err)
	}
	if _, err := r.readCommandMetaFromPayloadFresh("sample run"); err != nil {
		t.Fatalf("fresh command meta: %v", err)
	}
	if _, err := r.readCommandMetaFromPayloadFresh("missing"); err != nil {
		t.Fatalf("fresh missing path: %v", err)
	}
	if _, ok := r.renderedCompactLeaf("sample.run"); !ok {
		t.Fatal("rendered compact canonical")
	}
	if _, ok := r.renderedCompactLeaf("sample run"); !ok {
		t.Fatal("rendered compact cli path")
	}

	runtimeDeliveryLiveCatalog.Store(nil)
	rechecked, _, err := repairSchemaCache(r, func() (any, error) {
		return map[string]any{"rechecked": true}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := rechecked.(map[string]any)
	if got["rechecked"] != true {
		t.Fatalf("repair recheck = %#v", rechecked)
	}

	stale := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	stale.openOnce.Do(func() { stale.cache = cache })
	stale.indexOnce.Do(func() { stale.indexErr = errors.New("stale index") })
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true, Identity: identity},
		runtime: stale,
	})
	runtimeDeliveryLiveCatalog.Store(nil)
	resolvedMeta, ok := ResolveMeta("sample run")
	if !ok || resolvedMeta.Identity.Canonical != "sample.run" {
		t.Fatalf("ResolveMeta repair recheck = %#v %v", resolvedMeta, ok)
	}

	if _, err := r.queryPayload(meta, "sample run", false); err != nil {
		t.Fatalf("uncached queryPayload: %v", err)
	}
	if _, err := r.payloadsHandle(); err != nil {
		t.Fatalf("payloads handle: %v", err)
	}

	emptyIdentity := identity
	emptyIdentity.Edition = "emptyed"
	if err := emptyIdentity.Validate(); err != nil {
		t.Fatal(err)
	}
	emptyCache, err := schemacache.Open(emptyIdentity.Edition)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = emptyCache.Close() })
	emptyRuntime := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: emptyIdentity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	emptyRuntime.openOnce.Do(func() { emptyRuntime.cache = emptyCache })
	if _, err := emptyRuntime.payloadsHandle(); err == nil {
		t.Fatal("empty payloads handle")
	}

	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true, Identity: emptyIdentity, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		runtime: &schemaCacheRuntime{options: SchemaCacheOptions{Enabled: true, Identity: emptyIdentity}},
	})
	PrewarmSchemaCache()
	AwaitSchemaCachePrewarmForTest()

	allOnce := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	allOnce.openOnce.Do(func() { allOnce.cache = cache })
	allOnce.allOnce.Do(func() { allOnce.allErr = errors.New("stale all") })
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true, Identity: identity},
		runtime: allOnce,
	})
	runtimeDeliveryLiveCatalog.Store(nil)
	if _, err := deliverySchemaAllPayload(); err != nil {
		t.Fatalf("all repair recheck: %v", err)
	}
	runtimeDeliveryLiveCatalog.Store(nil)
	overviewOnce := &schemaCacheRuntime{
		options:  SchemaCacheOptions{Enabled: true, Identity: identity, LockTimeout: time.Second},
		products: make(map[string]*schemaCacheProductLoad),
		payloads: make(map[string]*schemaCachePayloadLoad),
	}
	overviewOnce.openOnce.Do(func() { overviewOnce.cache = cache })
	overviewOnce.metaOnce.Do(func() { overviewOnce.metaErr = errors.New("stale meta") })
	schemaCacheRegistrationValue.Store(&schemaCacheRegistration{
		options: SchemaCacheOptions{Enabled: true, Identity: identity},
		runtime: overviewOnce,
	})
	if _, err := deliverySchemaOverviewPayload(); err != nil {
		t.Fatalf("overview repair recheck: %v", err)
	}
}
