// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package schemareader

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/schemaruntime"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/schemacache"
)

func TestCrossPlatformCoverageReadHelpersFailClosedWithoutCache(t *testing.T) {
	meta := schemaruntime.DecodedSchemaMeta{
		ProductDescriptors:   []schemaruntime.ProductDescriptor{{ProductID: "drive"}},
		LocatorProductByPath: map[string]string{"drive": "drive"},
	}
	index := schemaruntime.DecodedSchemaPayloadIndex{
		PayloadDescriptors:   []schemaruntime.CommandPayloadDescriptor{{ProductID: "drive"}},
		LocatorProductByPath: map[string]string{"drive": "drive"},
	}
	identity := Identity{Edition: "open"}

	if _, err := ReadMeta(nil, identity); !errors.Is(err, schemacache.ErrClosed) {
		t.Fatalf("ReadMeta nil cache = %v", err)
	}
	if _, err := ReadProduct(nil, identity, meta, "mail"); err == nil || err.Error() != `unknown Schema product "mail"` {
		t.Fatalf("unknown product = %v", err)
	}
	if _, err := ReadProduct(nil, identity, meta, "drive"); !errors.Is(err, schemacache.ErrClosed) {
		t.Fatalf("ReadProduct closed = %v", err)
	}
	if _, err := ReadPayloadIndex(nil, identity); !errors.Is(err, schemacache.ErrClosed) {
		t.Fatalf("ReadPayloadIndex = %v", err)
	}
	if _, err := ReadCommandPayload(nil, identity, index, "mail"); err == nil {
		t.Fatal("unknown payload product")
	}
	if _, err := ReadCommandPayload(nil, identity, index, "drive"); !errors.Is(err, schemacache.ErrClosed) {
		t.Fatalf("ReadCommandPayload closed = %v", err)
	}
	if _, err := ReadCommandPayloadRange(nil, identity, index, "mail"); err == nil {
		t.Fatal("unknown payload range product")
	}
	if _, err := ReadRenderedLeaf(nil, identity, index, "mail", schemaruntime.RenderedLeafRef{}); err == nil {
		t.Fatal("unknown rendered leaf product")
	}
	if _, err := ReadRenderedLeaf(nil, identity, index, "drive", schemaruntime.RenderedLeafRef{}); !errors.Is(err, schemacache.ErrClosed) {
		t.Fatalf("ReadRenderedLeaf closed = %v", err)
	}
	if _, err := ReadPayloadIndexRange(nil, identity); err == nil {
		t.Fatal("nil payloads handle")
	}
	if product, ok := Locator(meta, "drive"); !ok || product != "drive" {
		t.Fatalf("Locator = %q %v", product, ok)
	}
	if _, ok := Locator(meta, "mail"); ok {
		t.Fatal("Locator matched an unknown path")
	}
	if product, ok := IndexLocator(index, "drive"); !ok || product != "drive" {
		t.Fatalf("IndexLocator = %q %v", product, ok)
	}
}

func TestCrossPlatformCoverageExpectedIdentityAndMalformedOptional(t *testing.T) {
	digest := [32]byte{1}
	identity := Identity{
		Edition: "open", CatalogSnapshotVersion: CatalogSnapshotVersion,
		SourceSHA256: digest, SurfaceSHA256: digest, BuildID: digest,
	}
	got := identity.ExpectedIdentity()
	if got.CatalogSnapshotVersion != CatalogSnapshotVersion || got.SourceSHA256 != digest {
		t.Fatalf("ExpectedIdentity = %#v", got)
	}
	if _, ok := parseSchemaCacheLowerHex("ZZ"); ok {
		t.Fatal("hex parser accepted invalid input")
	}
	if _, ok := parseSchemaCacheLowerHex("aa"); ok {
		t.Fatal("short hex")
	}
	if _, ok := parseSchemaCacheLowerHex(strings.Repeat("g", 64)); ok {
		t.Fatal("non-hex alphabet")
	}
	if _, ok := parseSchemaCachePositiveDecimal("0"); ok {
		t.Fatal("zero decimal")
	}
	if _, ok := parseSchemaCachePositiveDecimal("1a"); ok {
		t.Fatal("non-decimal")
	}
	if _, err := ParseOptionalIdentity(RawIdentity{
		Edition: "open", SourceSHA256: "not-hex", SurfaceSHA256: "aa", BuildID: "aa",
		MetaLength: "1", MetaSHA256: "aa", RegistryLength: "1", RegistrySHA256: "aa",
		PayloadLength: "1", PayloadSHA256: "aa", PayloadIndexLength: "1", PayloadIndexSHA256: "aa",
	}); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("malformed optional identity = %v", err)
	}

	if _, err := ReadProduct(nil, Identity{}, schemaruntime.DecodedSchemaMeta{}, "drive"); err == nil {
		t.Fatal("unknown product")
	}
	if _, ok := Descriptor(schemaruntime.DecodedSchemaMeta{}, "drive"); ok {
		t.Fatal("missing descriptor")
	}
	if _, err := ReadCommandPayloadRange(nil, Identity{}, schemaruntime.DecodedSchemaPayloadIndex{}, "drive"); err == nil {
		t.Fatal("unknown payload product")
	}
	if _, err := ReadRenderedLeafRange(nil, Identity{}, schemaruntime.DecodedSchemaPayloadIndex{}, "drive", schemaruntime.RenderedLeafRef{}); err == nil {
		t.Fatal("unknown rendered product")
	}
	if _, err := ReadCommandPayload(nil, Identity{}, schemaruntime.DecodedSchemaPayloadIndex{}, "drive"); err == nil {
		t.Fatal("nil cache command payload")
	}
	if _, err := ReadRenderedLeaf(nil, Identity{}, schemaruntime.DecodedSchemaPayloadIndex{}, "drive", schemaruntime.RenderedLeafRef{}); err == nil {
		t.Fatal("nil cache rendered leaf")
	}
	if _, err := ReadPayloadIndex(nil, Identity{}); err == nil {
		t.Fatal("nil cache payload index")
	}
}

func TestCrossPlatformCoverageReadMetaHashMismatchAndProductRangeFailure(t *testing.T) {
	registry := schemaruntime.SchemaRegistry{
		Kind: "schema", Level: "catalog", Source: "test",
		Products: []schemaruntime.ProductSpec{{
			ID: "sample",
			Tools: []schemaruntime.ToolSpec{{
				Identity: contract.ToolIdentitySpec{
					ProductID: "sample", Name: "run", CanonicalPath: "sample.run", CLIPath: "sample run",
				},
				Title: "Run", Description: "Runs sample",
			}},
		}},
	}
	overview, err := schemaruntime.BuildSchemaOverview(registry)
	if err != nil {
		t.Fatal(err)
	}
	locators, err := schemaruntime.BuildSchemaProductLocators(registry)
	if err != nil {
		t.Fatal(err)
	}
	lookup := schemaruntime.BuildCommandMetaLookup(registry)
	rendered := map[string][]byte{}
	for path, meta := range lookup {
		if path == meta.Identity.CLIPath && meta.Identity.Canonical != "" {
			rendered[meta.Identity.Canonical] = []byte("{}\n")
		}
	}
	built, err := schemaruntime.BuildSchemaCache(registry, lookup, overview, locators, schemaruntime.CacheHashes{
		SourceSHA256: sha256.Sum256([]byte("reader-source")), SurfaceSHA256: sha256.Sum256([]byte("reader-surface")),
	}, rendered)
	if err != nil {
		t.Fatal(err)
	}
	indexLength := built.PayloadIndexLength
	indexDigest := built.PayloadIndexSHA256
	identity := Identity{
		Edition: "official", CatalogSnapshotVersion: CatalogSnapshotVersion,
		SourceSHA256: sha256.Sum256([]byte("envelope-source")), SurfaceSHA256: sha256.Sum256([]byte("envelope-surface")),
		BuildID:            sha256.Sum256([]byte("reader-build")),
		Meta:               schemaCacheArtifactExpectation(schemacache.KindMeta, uint64(len(built.Meta)), sha256.Sum256(built.Meta)),
		Registry:           schemaCacheArtifactExpectation(schemacache.KindRegistry, uint64(len(built.ProductShards)), built.RegistrySHA256),
		Payload:            schemaCacheArtifactExpectation(schemacache.KindPayloads, uint64(len(built.PayloadShards)), built.PayloadSHA256),
		PayloadIndexLength: indexLength, PayloadIndexSHA256: indexDigest,
	}
	if err := identity.Validate(); err != nil {
		t.Fatal(err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp(home, ".dws-schemareader-")
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
		if errors.Is(err, schemacache.ErrDisabled) {
			t.Skip("schema cache disabled on this platform")
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cache.Close() })
	if err := cache.Publish(identity.ExpectedIdentity(), schemacache.Artifact{Expectation: identity.Registry, Payload: built.ProductShards}, schemacache.Artifact{Expectation: identity.Meta, Payload: built.Meta}, schemacache.Artifact{Expectation: identity.Payload, Payload: built.PayloadShards}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMeta(cache, identity); !errors.Is(err, schemacache.ErrIdentityMismatch) {
		t.Fatalf("inner hash mismatch = %v", err)
	}

	matching := identity
	matching.SourceSHA256 = sha256.Sum256([]byte("reader-source"))
	matching.SurfaceSHA256 = sha256.Sum256([]byte("reader-surface"))
	if err := cache.Publish(matching.ExpectedIdentity(), schemacache.Artifact{Expectation: matching.Registry, Payload: built.ProductShards}, schemacache.Artifact{Expectation: matching.Meta, Payload: built.Meta}, schemacache.Artifact{Expectation: matching.Payload, Payload: built.PayloadShards}); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(cache, matching)
	if err != nil {
		t.Fatal(err)
	}
	meta.ProductDescriptors[0].SHA256 = sha256.Sum256([]byte("wrong"))
	if _, err := ReadProduct(cache, matching, meta, "sample"); err == nil {
		t.Fatal("product range digest mismatch")
	}

	garbage := []byte("not-a-valid-schema-meta-protobuf!!")
	garbageIdentity := matching
	garbageIdentity.Meta = schemaCacheArtifactExpectation(schemacache.KindMeta, uint64(len(garbage)), sha256.Sum256(garbage))
	if err := cache.Publish(garbageIdentity.ExpectedIdentity(), schemacache.Artifact{Expectation: garbageIdentity.Registry, Payload: built.ProductShards}, schemacache.Artifact{Expectation: garbageIdentity.Meta, Payload: garbage}, schemacache.Artifact{Expectation: garbageIdentity.Payload, Payload: built.PayloadShards}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMeta(cache, garbageIdentity); err == nil {
		t.Fatal("decode meta")
	}

	if err := cache.Publish(matching.ExpectedIdentity(), schemacache.Artifact{Expectation: matching.Registry, Payload: built.ProductShards}, schemacache.Artifact{Expectation: matching.Meta, Payload: built.Meta}, schemacache.Artifact{Expectation: matching.Payload, Payload: built.PayloadShards}); err != nil {
		t.Fatal(err)
	}
	index, err := ReadPayloadIndex(cache, matching)
	if err != nil {
		t.Fatal(err)
	}
	index.PayloadDescriptors[0].HeaderSHA256[0] ^= 1
	if _, err := ReadCommandPayload(cache, matching, index, "sample"); err == nil {
		t.Fatal("payload range digest")
	}
	if _, err := ReadRenderedLeaf(cache, matching, index, "sample", schemaruntime.RenderedLeafRef{Length: 4, SHA256: sha256.Sum256([]byte("nope"))}); err == nil {
		t.Fatal("rendered leaf range")
	}
}
