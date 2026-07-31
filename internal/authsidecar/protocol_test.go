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
	request.Nonce = strings.Repeat("b", 32)
	if err := VerifySignature(key, request, signature); err == nil {
		t.Fatal("VerifySignature() accepted a modified nonce")
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
	accepted := []string{"unix:///tmp/dws.sock", "http://127.0.0.1:16384", "http://host.docker.internal:16384"}
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
	for _, path := range []string{"dws auth status", "dws api GET", "dws event listen", "dws plugin list", "dws doctor", "dws upgrade"} {
		if err := ValidateCommandPath(path); err == nil {
			t.Errorf("ValidateCommandPath(%q) unexpectedly succeeded", path)
		}
	}
	if err := ValidateCommandPath("dws doc get"); err != nil {
		t.Fatalf("ValidateCommandPath(MCP command) error = %v", err)
	}
}
