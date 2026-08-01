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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// profileTokenResolver returns a distinct token per bound profile so a
// cross-profile mix-up is observable upstream.
type profileTokenResolver struct{}

func (profileTokenResolver) ResolveAccessToken(_ context.Context, profile string) (string, error) {
	return "token-for-" + profile, nil
}

func TestHandlerRejectsPercentEncodedPath(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("a non-canonical path reached upstream")
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, []string{"get_document"})
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	// "/%6dcp" decodes to "/mcp": the ACL would allow it while the forwarded
	// URL kept the encoded form.
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_document"}}`)
	request := signedHandlerRequestForPath(t, key, upstream.URL, "/%6dcp", body, now, strings.Repeat("1", 32))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || recorder.Header().Get(HeaderError) != "path_not_canonical" {
		t.Fatalf("status = %d, code = %q", recorder.Code, recorder.Header().Get(HeaderError))
	}
}

func TestHandlerConcurrentProfilesDoNotCrossTalk(t *testing.T) {
	type observation struct {
		path  string
		token string
	}
	seen := make(chan observation, 64)
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		seen <- observation{path: request.URL.Path, token: request.Header.Get("x-user-access-token")}
		_, _ = io.WriteString(response, `{"jsonrpc":"2.0","result":{}}`)
	}))
	defer upstream.Close()

	config, keys := twoProfileServerConfig(t, upstream.URL)
	handler, err := NewHandler(config, profileTokenResolver{}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_document"}}`)
	const perKey = 16
	var waitGroup sync.WaitGroup
	for index := 0; index < perKey; index++ {
		for _, sandbox := range []struct{ keyID, path, profile string }{
			{"sandbox-a", "/mcp-a", "corp-a:user-a"},
			{"sandbox-b", "/mcp-b", "corp-b:user-b"},
		} {
			waitGroup.Add(1)
			go func(keyID, path, profile string, attempt int) {
				defer waitGroup.Done()
				nonce := fmt.Sprintf("%s%02d", strings.Repeat("c", 30), attempt)
				request := signedRequestForKey(t, keys[keyID], keyID, upstream.URL, path, body, now, nonce)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				if recorder.Code != http.StatusOK {
					t.Errorf("%s status = %d, code = %q", keyID, recorder.Code, recorder.Header().Get(HeaderError))
				}
			}(sandbox.keyID, sandbox.path, sandbox.profile, index)
		}
	}
	waitGroup.Wait()
	close(seen)

	for observed := range seen {
		want := "token-for-corp-a:user-a"
		if observed.path == "/mcp-b" {
			want = "token-for-corp-b:user-b"
		}
		if observed.token != want {
			t.Fatalf("path %s received %q, want %q: profiles crossed", observed.path, observed.token, want)
		}
	}
}

func TestHandlerAuditLogRedactsKeyProfileAndToken(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(response, `{"jsonrpc":"2.0","result":{}}`)
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, []string{"get_document"})
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler, err := NewHandler(config, staticTokenResolver{token: "super-secret-token"}, upstream.Client(), logger)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_document"}}`)

	accepted := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("2", 32))
	handler.ServeHTTP(httptest.NewRecorder(), accepted)

	denied := signedHandlerRequest(t, key, upstream.URL,
		[]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"denied_tool"}}`), now, strings.Repeat("3", 32))
	handler.ServeHTTP(httptest.NewRecorder(), denied)

	output := logs.String()
	if output == "" {
		t.Fatal("handler produced no audit log")
	}
	for _, secret := range []string{"super-secret-token", "sandbox-a", "corp-a:user-a", "user-a"} {
		if strings.Contains(output, secret) {
			t.Fatalf("audit log leaked %q: %s", secret, output)
		}
	}
	if !strings.Contains(output, "sidecar_forward") || !strings.Contains(output, "sidecar_reject") {
		t.Fatalf("audit log missing forward/reject records: %s", output)
	}
	if !strings.Contains(output, `"request":`) {
		t.Fatalf("audit log omitted the request correlation hash: %s", output)
	}
	if strings.Contains(output, "/mcp") || !strings.Contains(output, "path_hash") {
		t.Fatalf("audit log exposed a raw endpoint path or omitted its hash: %s", output)
	}
}

func TestReplayCacheFullBucketRejectsUnrecordedNonce(t *testing.T) {
	cache := NewReplayCache(1, 0)
	now := time.Unix(1700000000, 0)
	if err := cache.Use("key-a", strings.Repeat("a", 32), now); err != nil {
		t.Fatal(err)
	}
	err := cache.Use("key-a", strings.Repeat("b", 32), now)
	if err == nil {
		t.Fatal("a full bucket admitted an unrecorded nonce")
	}
	if strings.Contains(err.Error(), "replay detected") {
		t.Fatalf("capacity exhaustion must be distinguishable from a replay: %v", err)
	}
}

func twoProfileServerConfig(t *testing.T, origin string) (*ServerConfig, map[string][]byte) {
	t.Helper()
	directory := t.TempDir()
	keys := map[string][]byte{}
	bindings := make([]map[string]any, 0, 2)
	policies := make([]map[string]any, 0, 2)
	for _, sandbox := range []struct{ keyID, keyHex, profile, path string }{
		{"sandbox-a", strings.Repeat("a1", 32), "corp-a:user-a", "/mcp-a"},
		{"sandbox-b", strings.Repeat("b2", 32), "corp-b:user-b", "/mcp-b"},
	} {
		keyPath := filepath.Join(directory, sandbox.keyID+".key")
		if err := os.WriteFile(keyPath, []byte(sandbox.keyHex), 0o600); err != nil {
			t.Fatal(err)
		}
		bindings = append(bindings, map[string]any{
			"key_id": sandbox.keyID, "key_file": keyPath, "profile": sandbox.profile,
			"policy": "policy-" + sandbox.keyID, "enabled": true, "expires_at": time.Now().Add(24 * time.Hour),
		})
		policies = append(policies, map[string]any{
			"name": "policy-" + sandbox.keyID, "allowed_origins": []string{origin},
			"allowed_paths": []string{sandbox.path}, "allowed_tools": []string{"get_document"},
		})
		key, err := ReadKeyFile(keyPath)
		if err != nil {
			t.Fatal(err)
		}
		keys[sandbox.keyID] = key
	}
	data, err := json.Marshal(map[string]any{"version": 1, "bindings": bindings, "policies": policies})
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "config.json")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadServerConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	return config, keys
}

func signedRequestForKey(t *testing.T, key []byte, keyID, target, requestPath string, body []byte, now time.Time, nonce string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "http://sidecar"+requestPath, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	canonical := CanonicalRequest{
		KeyID: keyID, Timestamp: fmt.Sprint(now.Unix()), Nonce: nonce, Method: http.MethodPost,
		TargetOrigin: target, PathAndQuery: requestPath, BodySHA256: BodySHA256(body),
	}
	request.Header.Set(HeaderVersion, ProtocolV1)
	request.Header.Set(HeaderKeyID, keyID)
	request.Header.Set(HeaderTarget, target)
	request.Header.Set(HeaderTimestamp, canonical.Timestamp)
	request.Header.Set(HeaderNonce, nonce)
	request.Header.Set(HeaderBodySHA256, canonical.BodySHA256)
	request.Header.Set(HeaderSignature, Sign(key, canonical))
	return request
}
