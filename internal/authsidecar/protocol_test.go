// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package authsidecar

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProtocolSignatureCoversRoutingAndNonce(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	request := CanonicalRequest{
		KeyID: "sandbox-a", Timestamp: "1700000000", Nonce: strings.Repeat("a", 32),
		Method: http.MethodPost, TargetOrigin: "https://mcp.dingtalk.com",
		PathAndQuery: "/server?q=1", BodySHA256: BodySHA256([]byte(`{"ok":true}`)),
	}
	signature := Sign(key, request)
	if err := VerifySignature(key, request, signature); err != nil {
		t.Fatalf("VerifySignature() error = %v", err)
	}
	mutations := []struct {
		name   string
		mutate func(*CanonicalRequest)
	}{
		{name: "key ID", mutate: func(value *CanonicalRequest) { value.KeyID = "sandbox-b" }},
		{name: "timestamp", mutate: func(value *CanonicalRequest) { value.Timestamp = "1700000001" }},
		{name: "nonce", mutate: func(value *CanonicalRequest) { value.Nonce = strings.Repeat("b", 32) }},
		{name: "method", mutate: func(value *CanonicalRequest) { value.Method = http.MethodPut }},
		{name: "target", mutate: func(value *CanonicalRequest) { value.TargetOrigin = "https://api.dingtalk.com" }},
		{name: "path", mutate: func(value *CanonicalRequest) { value.PathAndQuery = "/other?q=1" }},
		{name: "query", mutate: func(value *CanonicalRequest) { value.PathAndQuery = "/server?q=2" }},
		{name: "body digest", mutate: func(value *CanonicalRequest) { value.BodySHA256 = BodySHA256([]byte(`{"ok":false}`)) }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			modified := request
			mutation.mutate(&modified)
			if err := VerifySignature(key, modified, signature); err == nil {
				t.Fatal("VerifySignature() accepted a modified canonical field")
			}
		})
	}
}

// This vector is intentionally made only from literals so implementations in
// other languages can verify byte-for-byte protocol compatibility.
func TestProtocolV1CrossLanguageVector(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	request := CanonicalRequest{
		KeyID:        "sandbox-vector-1",
		Timestamp:    "1700000000",
		Nonce:        "00112233445566778899aabbccddeeff",
		Method:       "post",
		TargetOrigin: "https://[2001:db8::1]:8443",
		PathAndQuery: "/server?key=a%2Fb&x=1",
		BodySHA256:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
	if got := BodySHA256(nil); got != request.BodySHA256 {
		t.Fatalf("empty body SHA-256 = %q, want %q", got, request.BodySHA256)
	}
	wantCanonical := "DWS-AUTHSIDECAR/v1\n" +
		"sandbox-vector-1\n" +
		"1700000000\n" +
		"00112233445566778899aabbccddeeff\n" +
		"POST\n" +
		"https://[2001:db8::1]:8443\n" +
		"/server?key=a%2Fb&x=1\n" +
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got := request.String(); got != wantCanonical {
		t.Fatalf("canonical request mismatch:\n got: %q\nwant: %q", got, wantCanonical)
	}
	const wantSignature = "f7dbb481c80762dfc2bacb662a23d9bb4e736348926ab069c4ccdf0101746c9b"
	if got := Sign(key, request); got != wantSignature {
		t.Fatalf("Sign() = %q, want %q", got, wantSignature)
	}
}

func TestReplayCacheRejectsDuplicateAndFailsClosedAtCapacity(t *testing.T) {
	now := time.Unix(1700000000, 0)
	cache := NewReplayCache(1, 2*MaxTimestampDrift)
	if err := cache.Use("a", strings.Repeat("a", 32), now); err != nil {
		t.Fatalf("Use(first) error = %v", err)
	}
	if err := cache.Use("a", strings.Repeat("a", 32), now); err == nil || !strings.Contains(err.Error(), "replay") {
		t.Fatalf("Use(replay) error = %v", err)
	}
	if err := cache.Use("a", strings.Repeat("b", 32), now); err == nil || !strings.Contains(err.Error(), "capacity") {
		t.Fatalf("Use(capacity) error = %v", err)
	}
	if err := cache.Use("a", strings.Repeat("b", 32), now.Add(2*MaxTimestampDrift)); err != nil {
		t.Fatalf("Use(after expiry) error = %v", err)
	}
}

func TestParseAddressRejectsCrossMachine(t *testing.T) {
	accepted := []string{"unix:///tmp/dws.sock", "http://127.0.0.1:16384", "http://[::1]:16384", "http://host.docker.internal:16384"}
	for _, value := range accepted {
		if _, err := ParseAddress(value); err != nil {
			t.Errorf("ParseAddress(%q) error = %v", value, err)
		}
	}
	rejected := []string{"https://127.0.0.1:16384", "http://example.com:16384", "unix://relative.sock", "http://127.0.0.1"}
	for _, value := range rejected {
		if _, err := ParseAddress(value); err == nil {
			t.Errorf("ParseAddress(%q) unexpectedly succeeded", value)
		}
	}
}

func TestReadKeyFileRequiresProtectedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.key")
	if err := os.WriteFile(path, []byte(strings.Repeat("ab", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	key, err := ReadKeyFile(path)
	if err != nil || len(key) != 32 {
		t.Fatalf("ReadKeyFile() = %d bytes, %v", len(key), err)
	}
	shortHexPath := filepath.Join(t.TempDir(), "short-hex.key")
	if err := os.WriteFile(shortHexPath, []byte(strings.Repeat("ab", 16)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadKeyFile(shortHexPath); err == nil || !strings.Contains(err.Error(), "64 hex") {
		t.Fatalf("ReadKeyFile(short hex) error = %v, want minimum hex length error", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadKeyFile(path); err == nil {
			t.Fatal("ReadKeyFile() accepted group/other-readable key")
		}
	}
}

func TestValidateCommandPathBlocksUnsupportedSidecarCommands(t *testing.T) {
	t.Setenv(EnvAuthMode, AuthModeSidecar)
	for _, path := range []string{"dws auth status", "dws api GET", "dws event listen", "dws plugin list", "dws doctor", "dws upgrade", "dws mcp url get"} {
		if err := ValidateCommandPath(path); err == nil {
			t.Errorf("ValidateCommandPath(%q) unexpectedly succeeded", path)
		}
	}
	if err := ValidateCommandPath("dws doc get"); err != nil {
		t.Fatalf("ValidateCommandPath(MCP command) error = %v", err)
	}
}
