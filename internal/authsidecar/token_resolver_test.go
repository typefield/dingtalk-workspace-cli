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
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	authpkg "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/keychain"
)

type refreshRoundTripper func(*http.Request) (*http.Response, error)

func (f refreshRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestHardenedOAuthClientDisablesProxyAndRedirects(t *testing.T) {
	client := hardenedOAuthClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("OAuth transport inherits environment proxy configuration")
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLS MinVersion = %v, want TLS 1.2", transport.TLSClientConfig)
	}
	if client.CheckRedirect == nil {
		t.Fatal("OAuth client follows redirects")
	}
	if err := client.CheckRedirect(nil, nil); err == nil {
		t.Fatal("CheckRedirect permits a redirect")
	}
	if client.Timeout <= 0 {
		t.Fatal("OAuth client has no timeout")
	}
}

func TestNewDWSProfileTokenResolverValidatesConfigDirectory(t *testing.T) {
	if _, err := NewDWSProfileTokenResolver(t.TempDir(), nil); err != nil {
		t.Fatalf("NewDWSProfileTokenResolver(valid dir) error = %v", err)
	}
	if _, err := NewDWSProfileTokenResolver(filepath.Join(t.TempDir(), "missing"), nil); err == nil {
		t.Fatal("NewDWSProfileTokenResolver() accepted a missing config directory")
	}
}

func TestTokenForExactIdentityRejectsMismatchAndWhitespace(t *testing.T) {
	valid := &authpkg.TokenData{CorpID: "corp-a", UserID: "user-a", AccessToken: "token"}
	if token, err := tokenForExactIdentity(valid, "corp-a", "user-a"); err != nil || token != "token" {
		t.Fatalf("tokenForExactIdentity(valid) = %q, %v", token, err)
	}
	for _, snapshot := range []*authpkg.TokenData{
		nil,
		{CorpID: " corp-a", UserID: "user-a", AccessToken: "token"},
		{CorpID: "corp-a", UserID: "user-a ", AccessToken: "token"},
		{CorpID: "corp-b", UserID: "user-a", AccessToken: "token"},
		{CorpID: "corp-a", UserID: "user-b", AccessToken: "token"},
		{CorpID: "corp-a", UserID: "user-a", AccessToken: " "},
	} {
		if token, err := tokenForExactIdentity(snapshot, "corp-a", "user-a"); err == nil {
			t.Fatalf("tokenForExactIdentity(%#v) = %q, want rejection", snapshot, token)
		}
	}
}

func TestHandlerRefreshesExpiredExactProfileOnlyOnTrustedSide(t *testing.T) {
	t.Setenv(EnvAuthMode, "")
	isolation := t.TempDir()
	t.Setenv(keychain.StorageDirEnv, filepath.Join(isolation, "keychain"))
	t.Setenv(keychain.TestNamespaceEnv, isolation)
	t.Setenv(keychain.DisableKeychainEnv, "1")
	configDir := filepath.Join(isolation, "dws")
	profile := authpkg.Profile{CorpID: "corp-a", UserID: "user-a", Name: "sidecar-test"}
	if err := authpkg.SaveProfiles(configDir, &authpkg.ProfilesConfig{Version: 2, Profiles: []authpkg.Profile{profile}}); err != nil {
		t.Fatal(err)
	}
	if err := authpkg.SaveTokenDataKeychainForIdentity(profile.CorpID, profile.UserID, &authpkg.TokenData{
		AccessToken: "expired-access", RefreshToken: "refresh-token", ClientID: "client-a", Source: "direct",
		CorpID: profile.CorpID, UserID: profile.UserID,
		ExpiresAt: time.Now().Add(-time.Hour), RefreshExpAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := authpkg.SaveClientSecret("client-a", "client-secret"); err != nil {
		t.Fatal(err)
	}

	resolver, err := NewDWSProfileTokenResolver(configDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	var refreshCalls atomic.Int32
	resolver.oauthClient = &http.Client{Transport: refreshRoundTripper(func(request *http.Request) (*http.Response, error) {
		refreshCalls.Add(1)
		if request.URL.Scheme != "https" {
			t.Errorf("refresh URL scheme = %q, want https", request.URL.Scheme)
		}
		requestBody, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}
		var payload map[string]string
		if err := json.Unmarshal(requestBody, &payload); err != nil {
			t.Fatal(err)
		}
		if payload["refreshToken"] != "refresh-token" || payload["clientSecret"] != "client-secret" {
			t.Errorf("refresh request did not use the trusted-side credentials: keys=%v", sortedMapKeys(payload))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"accessToken":"refreshed-access","refreshToken":"rotated-refresh","expiresIn":7200}`,
			)),
		}, nil
	})}

	upstream := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer refreshed-access" {
			t.Errorf("Authorization = %q, want refreshed trusted-side token", got)
		}
		if got := request.Header.Get("x-user-access-token"); got != "refreshed-access" {
			t.Errorf("x-user-access-token = %q, want refreshed trusted-side token", got)
		}
		_, _ = io.WriteString(response, `{"jsonrpc":"2.0","result":{"ok":true}}`)
	}))
	defer upstream.Close()
	serverConfig, key := testServerConfig(t, upstream.URL, []string{"get_document"})
	handler, err := NewHandler(serverConfig, resolver, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	authpkg.SetRuntimeProfile("different-corp:different-user")
	t.Cleanup(func() { authpkg.SetRuntimeProfile("") })
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_document"}}`)
	request := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("1", 32))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("sidecar status = %d, code = %q, body = %s", recorder.Code, recorder.Header().Get(HeaderError), recorder.Body.String())
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("refreshed-access")) || bytes.Contains(recorder.Body.Bytes(), []byte("rotated-refresh")) {
		t.Fatalf("sandbox response leaked refreshed credential material: %s", recorder.Body.String())
	}
	if refreshCalls.Load() != 1 {
		t.Fatalf("refresh calls = %d, want 1", refreshCalls.Load())
	}
	stored, err := authpkg.LoadTokenDataKeychainForIdentity(profile.CorpID, profile.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AccessToken != "refreshed-access" || stored.RefreshToken != "rotated-refresh" {
		t.Fatalf("persisted refreshed token was not rotated: access=%t refresh=%t", stored.AccessToken == "refreshed-access", stored.RefreshToken == "rotated-refresh")
	}
	if got := authpkg.RuntimeProfile(); got != "different-corp:different-user" {
		t.Fatalf("RuntimeProfile() = %q; sidecar refresh mutated global profile", got)
	}
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
