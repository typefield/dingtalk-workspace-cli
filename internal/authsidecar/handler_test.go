// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package authsidecar

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewHandlerDefaultClientDisablesProxyAndRequiresTLS12(t *testing.T) {
	config, _ := testServerConfig(t, "https://mcp.dingtalk.com", []string{"read_doc"})
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := handler.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("default transport = %T, want *http.Transport", handler.client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("default upstream transport inherits proxy configuration")
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLS MinVersion = %v, want TLS 1.2", transport.TLSClientConfig)
	}
	if handler.client.CheckRedirect == nil {
		t.Fatal("default upstream client permits redirects")
	}
}

type staticTokenResolver struct{ token string }

func (r staticTokenResolver) ResolveAccessToken(context.Context, string) (string, error) {
	return r.token, nil
}

func TestHandlerForwardsAllowedToolAndRejectsReplay(t *testing.T) {
	var upstreamHeaders http.Header
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		upstreamHeaders = request.Header.Clone()
		response.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(response, `{"jsonrpc":"2.0","result":{}}`)
	}))
	defer upstream.Close()

	config, key := testServerConfig(t, upstream.URL, []string{"get_document"})
	handler, err := NewHandler(config, staticTokenResolver{token: "trusted-token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_document","arguments":{}}}`)
	request := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("a", 32))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := upstreamHeaders.Get("Authorization"); got != "Bearer trusted-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := upstreamHeaders.Get("x-user-access-token"); got != "trusted-token" {
		t.Fatalf("x-user-access-token = %q", got)
	}
	if upstreamHeaders.Get(HeaderSignature) != "" {
		t.Fatal("sidecar signature leaked upstream")
	}

	replay := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("a", 32))
	replayRecorder := httptest.NewRecorder()
	handler.ServeHTTP(replayRecorder, replay)
	if replayRecorder.Code != http.StatusConflict || replayRecorder.Header().Get(HeaderError) != "replay_detected" {
		t.Fatalf("replay status = %d, code = %q", replayRecorder.Code, replayRecorder.Header().Get(HeaderError))
	}
}

func TestHandlerPolicyDeniesUnknownTool(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("denied request reached upstream")
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, []string{"allowed_tool"})
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"denied_tool"}}`)
	request := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("b", 32))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || recorder.Header().Get(HeaderError) != "policy_denied" {
		t.Fatalf("status = %d, code = %q, body = %s", recorder.Code, recorder.Header().Get(HeaderError), recorder.Body.String())
	}
}

func TestHandlerPolicyDeniesUnknownPath(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("denied path reached upstream")
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, []string{"allowed_tool"})
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"allowed_tool"}}`)
	request := signedHandlerRequestForPath(t, key, upstream.URL, "/another", body, now, strings.Repeat("c", 32))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || recorder.Header().Get(HeaderError) != "path_denied" {
		t.Fatalf("status = %d, code = %q", recorder.Code, recorder.Header().Get(HeaderError))
	}
}

func testServerConfig(t *testing.T, origin string, tools []string) (*ServerConfig, []byte) {
	t.Helper()
	directory := t.TempDir()
	keyHex := strings.Repeat("ab", 32)
	keyPath := filepath.Join(directory, "sandbox.key")
	if err := os.WriteFile(keyPath, []byte(keyHex), 0o600); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"version": 1,
		"bindings": []map[string]any{{
			"key_id": "sandbox-a", "key_file": keyPath, "profile": "corp-a:user-a",
			"policy": "mcp", "enabled": true,
		}},
		"policies": []map[string]any{{
			"name": "mcp", "allowed_origins": []string{origin}, "allowed_paths": []string{"/mcp"}, "allowed_tools": tools,
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
	config, err := LoadServerConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ReadKeyFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	return config, key
}

func signedHandlerRequest(t *testing.T, key []byte, target string, body []byte, now time.Time, nonce string) *http.Request {
	return signedHandlerRequestForPath(t, key, target, "/mcp", body, now, nonce)
}

func signedHandlerRequestForPath(t *testing.T, key []byte, target, requestPath string, body []byte, now time.Time, nonce string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "http://sidecar"+requestPath+"?q=1", bytes.NewReader(body))
	canonical := CanonicalRequest{
		KeyID: "sandbox-a", Timestamp: fmt.Sprint(now.Unix()), Nonce: nonce,
		Method: http.MethodPost, TargetOrigin: target, PathAndQuery: requestPath + "?q=1", BodySHA256: BodySHA256(body),
	}
	request.Header.Set(HeaderVersion, ProtocolV1)
	request.Header.Set(HeaderKeyID, canonical.KeyID)
	request.Header.Set(HeaderTarget, target)
	request.Header.Set(HeaderTimestamp, canonical.Timestamp)
	request.Header.Set(HeaderNonce, canonical.Nonce)
	request.Header.Set(HeaderBodySHA256, canonical.BodySHA256)
	request.Header.Set(HeaderSignature, Sign(key, canonical))
	return request
}
