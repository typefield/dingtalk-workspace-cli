// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package schemacache

import (
	"crypto/sha256"
	"errors"
	"testing"
)

func TestCrossPlatformCoverageCacheNilGuardsAndInvalidArtifacts(t *testing.T) {
	var cache *Cache
	if cache.Directory() != "" {
		t.Fatal("nil Directory")
	}
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cache.WriteArtifact(ExpectedIdentity{}, Artifact{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("nil WriteArtifact = %v", err)
	}
	if err := cache.Publish(ExpectedIdentity{}, Artifact{}, Artifact{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("nil Publish = %v", err)
	}
	if _, err := cache.AcquireLock(nil, 0); !errors.Is(err, ErrClosed) {
		t.Fatalf("nil AcquireLock = %v", err)
	}
	var registry *Registry
	if err := registry.Close(); err != nil {
		t.Fatal(err)
	}
	if err := registry.ValidateAggregate(); !errors.Is(err, ErrClosed) {
		t.Fatalf("nil ValidateAggregate = %v", err)
	}
	var lock *Lock
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if (*Counters)(nil).Snapshot() != (IOSnapshot{}) {
		t.Fatal("nil Snapshot")
	}

	zero := ExpectedIdentity{}
	if err := zero.validate(); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("zero identity = %v", err)
	}
	if err := (ArtifactExpectation{Kind: KindMeta}).validate(KindRegistry); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("kind mismatch = %v", err)
	}
	if _, err := envelopeFrom(zero, ArtifactExpectation{Kind: KindMeta}); err == nil {
		t.Fatal("envelopeFrom zero identity")
	}
	identity := ExpectedIdentity{
		CatalogSnapshotVersion: 1,
		EditionSHA256:          sha256.Sum256([]byte("edition")),
		SourceSHA256:           sha256.Sum256([]byte("source")),
		SurfaceSHA256:          sha256.Sum256([]byte("surface")),
		BuildID:                sha256.Sum256([]byte("build")),
	}
	if _, err := envelopeFrom(identity, ArtifactExpectation{Kind: KindMeta}); err == nil {
		t.Fatal("envelopeFrom invalid artifact")
	}
	if err := validateArtifactPayload(ExpectedIdentity{}, Artifact{}); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("zero identity payload = %v", err)
	}
	payload := []byte("meta")
	lengthMismatch := ArtifactExpectation{
		Kind: KindMeta, Serializer: SerializerProtobuf, Codec: CodecRaw,
		FormatVersion: DTOFormatVersion, EncodedLength: 3, DecodedLength: 3,
		EncodedSHA256: sha256.Sum256(payload),
	}
	if err := validateArtifactPayload(identity, Artifact{Expectation: lengthMismatch, Payload: payload}); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("length mismatch = %v", err)
	}
	digestMismatch := lengthMismatch
	digestMismatch.EncodedLength = 4
	digestMismatch.DecodedLength = 4
	digestMismatch.EncodedSHA256 = sha256.Sum256([]byte("other"))
	if err := validateArtifactPayload(identity, Artifact{Expectation: digestMismatch, Payload: payload}); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("digest mismatch = %v", err)
	}
	if _, err := Open(""); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("invalid edition Open = %v", err)
	}
	if _, err := EditionSHA256(""); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("invalid edition digest = %v", err)
	}
}
