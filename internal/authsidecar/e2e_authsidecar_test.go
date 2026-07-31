//go:build authsidecar

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
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEndToEndUnixSocketProxy drives the full sandbox path: env-configured
// client RoundTripper -> unix socket -> trusted handler -> mock MCP upstream.
func TestEndToEndUnixSocketProxy(t *testing.T) {
	var upstreamHeaders http.Header
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		upstreamHeaders = request.Header.Clone()
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("Authorization", "Bearer upstream-echo")
		_, _ = io.WriteString(response, `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	}))
	defer upstream.Close()

	config, _ := testServerConfig(t, upstream.URL, []string{"get_document"})
	handler, err := NewHandler(config, staticTokenResolver{token: "real-user-token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}

	socketDir, err := os.MkdirTemp("", "dws-sc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(socketDir)
	socketPath := filepath.Join(socketDir, "s.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: handler}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	keyPath := filepath.Join(socketDir, "sandbox.key")
	if err := os.WriteFile(keyPath, []byte(strings.Repeat("ab", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvAuthMode, AuthModeSidecar)
	t.Setenv(EnvSidecarAddress, "unix://"+socketPath)
	t.Setenv(EnvSidecarKeyID, "sandbox-a")
	t.Setenv(EnvSidecarKeyFile, keyPath)

	client := &http.Client{Transport: WrapRoundTripper(http.DefaultTransport)}
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_document"}}`
	sendManaged := func() *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, upstream.URL+"/mcp?q=1", bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+SentinelUserToken)
		request.Header.Set("x-user-access-token", SentinelUserToken)
		request.Header.Set("Cookie", "sandbox=evil")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}

	response := sendManaged()
	defer response.Body.Close()
	payload, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(payload), `"ok":true`) {
		t.Fatalf("E2E status = %d, body = %s", response.StatusCode, payload)
	}
	if got := upstreamHeaders.Get("Authorization"); got != "Bearer real-user-token" {
		t.Fatalf("upstream Authorization = %q", got)
	}
	if got := upstreamHeaders.Get("x-user-access-token"); got != "real-user-token" {
		t.Fatalf("upstream x-user-access-token = %q", got)
	}
	if got := upstreamHeaders.Get("Cookie"); got != "" {
		t.Fatalf("sandbox Cookie reached upstream: %q", got)
	}
	if got := upstreamHeaders.Get(HeaderSignature); got != "" {
		t.Fatal("sidecar protocol headers reached upstream")
	}
	if got := response.Header.Get("Authorization"); got != "" {
		t.Fatalf("upstream credential echo reached sandbox: %q", got)
	}

	// A second request must mint a fresh nonce and succeed end to end.
	second := sendManaged()
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second E2E request status = %d", second.StatusCode)
	}

	// Requests without both sentinel headers must fail closed client-side.
	plain, err := http.NewRequest(http.MethodPost, upstream.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(plain); err == nil || !strings.Contains(err.Error(), "sidecar_unmanaged_request") {
		t.Fatalf("unmanaged request error = %v, want sidecar_unmanaged_request", err)
	}
}
