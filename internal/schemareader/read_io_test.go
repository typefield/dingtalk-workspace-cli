// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package schemareader

import (
	"errors"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/schemaruntime"
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
}
