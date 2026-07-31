//go:build authsidecar

// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package authsidecar

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestClientRoundTripperSignsAndStripsSentinel(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	var captured *http.Request
	transport := &clientRoundTripper{
		config: ClientConfig{Address: Address{Network: "tcp", Value: "127.0.0.1:16384", URLHost: "127.0.0.1:16384"}, KeyID: "sandbox-a", Key: key},
		route: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			captured = request
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header), Request: request}, nil
		}),
		now:   func() time.Time { return time.Unix(1700000000, 0) },
		nonce: func() (string, error) { return strings.Repeat("a", 32), nil },
	}
	body := []byte(`{"jsonrpc":"2.0","method":"tools/list"}`)
	request, err := http.NewRequest(http.MethodPost, "https://mcp.dingtalk.com/server?q=1", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+SentinelUserToken)
	request.Header.Set("x-user-access-token", SentinelUserToken)
	if _, err := transport.RoundTrip(request); err != nil {
		t.Fatal(err)
	}
	if captured.URL.String() != "http://127.0.0.1:16384/server?q=1" {
		t.Fatalf("proxied URL = %q", captured.URL.String())
	}
	if captured.Header.Get("Authorization") != "" || captured.Header.Get("x-user-access-token") != "" {
		t.Fatal("sentinel credentials were not stripped")
	}
	canonical := CanonicalRequest{
		KeyID: "sandbox-a", Timestamp: "1700000000", Nonce: strings.Repeat("a", 32), Method: http.MethodPost,
		TargetOrigin: "https://mcp.dingtalk.com", PathAndQuery: "/server?q=1", BodySHA256: BodySHA256(body),
	}
	if err := VerifySignature(key, canonical, captured.Header.Get(HeaderSignature)); err != nil {
		t.Fatalf("captured signature: %v", err)
	}
}

func TestClientRoundTripperRejectsTargetUserinfo(t *testing.T) {
	transport := &clientRoundTripper{
		config: ClientConfig{
			Address: Address{Network: "tcp", Value: "127.0.0.1:16384", URLHost: "127.0.0.1:16384"},
			KeyID:   "sandbox-a",
			Key:     []byte("01234567890123456789012345678901"),
		},
		route: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("route must not be called for target URL userinfo")
			return nil, nil
		}),
		now:   time.Now,
		nonce: NewNonce,
	}
	request, err := http.NewRequest(http.MethodPost, "https://user:password@mcp.dingtalk.com/server", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+SentinelUserToken)
	request.Header.Set("x-user-access-token", SentinelUserToken)

	_, err = transport.RoundTrip(request)
	if err == nil || !strings.Contains(err.Error(), "sidecar_target_userinfo_forbidden") {
		t.Fatalf("RoundTrip() error = %v, want sidecar_target_userinfo_forbidden", err)
	}
}
