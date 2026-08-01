// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package authsidecar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPrepareClientFailsClosedOnHalfConfiguration(t *testing.T) {
	t.Setenv(EnvAuthMode, "")
	t.Setenv(EnvSidecarAddress, "unix:///run/dws-sidecar/dws.sock")

	err := PrepareClient(nil)
	if err == nil || !strings.Contains(err.Error(), "sidecar_config_incomplete") {
		t.Fatalf("PrepareClient() error = %v, want sidecar_config_incomplete", err)
	}
}

func TestValidateSidecarEnvConsistency(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		keyID   string
		wantErr bool
	}{
		{name: "nothing set", mode: "", keyID: "", wantErr: false},
		{name: "full sidecar mode", mode: AuthModeSidecar, keyID: "sandbox-a", wantErr: false},
		{name: "key id without mode", mode: "", keyID: "sandbox-a", wantErr: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(EnvAuthMode, testCase.mode)
			t.Setenv(EnvSidecarAddress, "")
			t.Setenv(EnvSidecarKeyID, testCase.keyID)
			t.Setenv(EnvSidecarKeyFile, "")
			err := ValidateSidecarEnvConsistency()
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateSidecarEnvConsistency() error = %v, wantErr = %v", err, testCase.wantErr)
			}
		})
	}
}

func TestLoadClientConfigRequiresAbsoluteKeyPath(t *testing.T) {
	t.Setenv(EnvAuthMode, AuthModeSidecar)
	t.Setenv(EnvSidecarAddress, "unix:///run/dws-sidecar/dws.sock")
	t.Setenv(EnvSidecarKeyID, "sandbox-a")
	t.Setenv(EnvSidecarKeyFile, "relative.key")
	if _, err := LoadClientConfigFromEnv(); err == nil || !strings.Contains(err.Error(), "absolute path") {
		t.Fatalf("LoadClientConfigFromEnv() error = %v, want absolute path rejection", err)
	}
}

func TestParseExactIdentitySelector(t *testing.T) {
	corpID, userID, err := ParseExactIdentitySelector("corp-a:user-a")
	if err != nil || corpID != "corp-a" || userID != "user-a" {
		t.Fatalf("ParseExactIdentitySelector() = %q, %q, %v", corpID, userID, err)
	}
	for _, selector := range []string{
		"", "corp-a", ":user-a", "corp-a:", " corp-a:user-a", "corp-a : user-a", "corp-a:user-a ",
	} {
		if _, _, err := ParseExactIdentitySelector(selector); err == nil {
			t.Fatalf("ParseExactIdentitySelector(%q) accepted a non-literal selector", selector)
		}
	}
}

func TestPolicyRequiresCanonicalExactRequestURI(t *testing.T) {
	valid := Policy{
		AllowedOrigins:     []string{"https://mcp.dingtalk.com"},
		AllowedPaths:       []string{"/server"},
		AllowedRequestURIs: []string{"/server?key=reviewed"},
	}
	if err := valid.prepare(); err != nil {
		t.Fatalf("valid policy prepare() error = %v", err)
	}
	if _, ok := valid.requestURIs["/server?key=reviewed"]; !ok {
		t.Fatal("valid exact request URI was not indexed")
	}

	for _, requestURI := range []string{
		"/server", "/other?key=reviewed", "/%73erver?key=reviewed", "https://mcp.dingtalk.com/server?key=reviewed",
	} {
		policy := Policy{
			AllowedOrigins:     []string{"https://mcp.dingtalk.com"},
			AllowedPaths:       []string{"/server"},
			AllowedRequestURIs: []string{requestURI},
		}
		if err := policy.prepare(); err == nil {
			t.Errorf("prepare() accepted non-canonical request URI %q", requestURI)
		}
	}
	for _, allowedPath := range []string{"/%73erver", "/server?key=reviewed", "server", "/server "} {
		policy := Policy{
			AllowedOrigins: []string{"https://mcp.dingtalk.com"},
			AllowedPaths:   []string{allowedPath},
		}
		if err := policy.prepare(); err == nil {
			t.Errorf("prepare() accepted non-canonical path %q", allowedPath)
		}
	}
}

func TestPolicyRejectsAmbiguousOrDuplicateAuthorizationEntries(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(*Policy)
	}{
		{name: "non-canonical origin", mutate: func(policy *Policy) {
			policy.AllowedOrigins = []string{" HTTPS://MCP.DINGTALK.COM"}
		}},
		{name: "duplicate origin", mutate: func(policy *Policy) {
			policy.AllowedOrigins = append(policy.AllowedOrigins, policy.AllowedOrigins[0])
		}},
		{name: "request URI whitespace", mutate: func(policy *Policy) {
			policy.AllowedRequestURIs = []string{"/server?key=reviewed "}
		}},
		{name: "duplicate request URI", mutate: func(policy *Policy) {
			policy.AllowedRequestURIs = append(policy.AllowedRequestURIs, policy.AllowedRequestURIs[0])
		}},
		{name: "tool whitespace", mutate: func(policy *Policy) {
			policy.AllowedTools = []string{"doc.get_document "}
		}},
		{name: "tool control character", mutate: func(policy *Policy) {
			policy.AllowedTools = []string{"doc.get_document\n"}
		}},
		{name: "duplicate tool", mutate: func(policy *Policy) {
			policy.AllowedTools = []string{"doc.get_document", "doc.get_document"}
		}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			policy := Policy{
				AllowedOrigins:     []string{"https://mcp.dingtalk.com"},
				AllowedPaths:       []string{"/server"},
				AllowedRequestURIs: []string{"/server?key=reviewed"},
				AllowedTools:       []string{"doc.get_document"},
			}
			testCase.mutate(&policy)
			if err := policy.prepare(); err == nil {
				t.Fatal("prepare() accepted an ambiguous or duplicate authorization entry")
			}
		})
	}
}

func TestLoadServerConfigRequiresProtectedRegularFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix file permission semantics are not available")
	}
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "key")
	if err := os.WriteFile(keyPath, []byte(strings.Repeat("ab", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"version": 1,
		"bindings": []map[string]any{{
			"key_id": "sandbox-a", "key_file": keyPath, "profile": "corp:user", "policy": "p",
			"enabled": true, "expires_at": time.Now().Add(24 * time.Hour),
		}},
		"policies": []map[string]any{{
			"name": "p", "allowed_origins": []string{"https://mcp.dingtalk.com"},
			"allowed_paths": []string{"/mcp"}, "allowed_tools": []string{"doc.get_document"},
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "config.json")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadServerConfig(configPath); err != nil {
		t.Fatalf("LoadServerConfig() rejected a protected regular file: %v", err)
	}
	if err := os.Chmod(configPath, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadServerConfig(configPath); err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("LoadServerConfig() error = %v, want broad-permissions rejection", err)
	}
	if err := os.Chmod(configPath, 0o600); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(directory, "config-link.json")
	if err := os.Symlink(configPath, symlinkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadServerConfig(symlinkPath); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("LoadServerConfig() error = %v, want symlink rejection", err)
	}
}

func TestServerConfigRequiresFutureExpiryForEnabledBinding(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyPath, []byte(strings.Repeat("ab", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	newConfig := func(enabled bool, expiresAt time.Time) *ServerConfig {
		return &ServerConfig{
			Version: 1,
			Bindings: []Binding{{
				KeyID: "sandbox-a", KeyFile: keyPath, Profile: "corp:user", Policy: "p",
				Enabled: enabled, ExpiresAt: expiresAt,
			}},
			Policies: []Policy{{
				Name: "p", AllowedOrigins: []string{"https://mcp.dingtalk.com"},
				AllowedPaths: []string{"/mcp"}, AllowedTools: []string{"doc.get_document"},
			}},
		}
	}
	if err := newConfig(true, time.Time{}).prepare(filepath.Dir(keyPath)); err == nil || !strings.Contains(err.Error(), "expires_at") {
		t.Fatalf("enabled binding without expiry error = %v", err)
	}
	if err := newConfig(true, time.Now().Add(-time.Hour)).prepare(filepath.Dir(keyPath)); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("enabled binding with past expiry error = %v", err)
	}
	if err := newConfig(true, time.Now().Add(time.Hour)).prepare(filepath.Dir(keyPath)); err != nil {
		t.Fatalf("enabled binding with future expiry error = %v", err)
	}
	if err := newConfig(false, time.Time{}).prepare(filepath.Dir(keyPath)); err != nil {
		t.Fatalf("disabled historical binding without expiry error = %v", err)
	}
}

func TestServerConfigRejectsHMACKeyReuseAcrossBindings(t *testing.T) {
	directory := t.TempDir()
	keyAPath := filepath.Join(directory, "key-a")
	keyBPath := filepath.Join(directory, "key-b")
	sharedKey := []byte(strings.Repeat("ab", 32))
	for _, path := range []string{keyAPath, keyBPath} {
		if err := os.WriteFile(path, sharedKey, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	newConfig := func() *ServerConfig {
		return &ServerConfig{
			Version: 1,
			Bindings: []Binding{
				{KeyID: "sandbox-a", KeyFile: keyAPath, Profile: "corp:user-a", Policy: "p", Enabled: true, ExpiresAt: time.Now().Add(time.Hour)},
				{KeyID: "sandbox-b", KeyFile: keyBPath, Profile: "corp:user-b", Policy: "p", Enabled: true, ExpiresAt: time.Now().Add(time.Hour)},
			},
			Policies: []Policy{{
				Name: "p", AllowedOrigins: []string{"https://mcp.dingtalk.com"},
				AllowedPaths: []string{"/mcp"}, AllowedTools: []string{"doc.get_document"},
			}},
		}
	}
	if err := newConfig().prepare(directory); err == nil || !strings.Contains(err.Error(), "reuse") {
		t.Fatalf("duplicate HMAC key material error = %v", err)
	}
	if err := os.WriteFile(keyBPath, []byte(strings.Repeat("cd", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := newConfig().prepare(directory); err != nil {
		t.Fatalf("distinct HMAC keys error = %v", err)
	}
}

func TestLoadServerConfigRejectsUnknownPolicyFields(t *testing.T) {
	directory := t.TempDir()
	keyPath := filepath.Join(directory, "key")
	if err := os.WriteFile(keyPath, []byte(strings.Repeat("ab", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"version": 1,
		"bindings": []map[string]any{{
			"key_id": "sandbox-a", "key_file": keyPath, "profile": "corp:user", "policy": "p",
			"enabled": true, "expires_at": time.Now().Add(24 * time.Hour),
		}},
		"policies": []map[string]any{{
			"name": "p", "allowed_origins": []string{"https://mcp.dingtalk.com"},
			"allowed_paths": []string{"/mcp"}, "allowed_tools": []string{}, "allowed_query_prefixes": []string{"key="},
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "config.json")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadServerConfig(configPath); err == nil {
		t.Fatal("LoadServerConfig() accepted an unreviewed query policy field")
	}
}
