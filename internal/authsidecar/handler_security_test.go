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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type failingTokenResolver struct{}

func (failingTokenResolver) ResolveAccessToken(context.Context, string) (string, error) {
	return "", errors.New("credential store unavailable")
}

func TestHandlerRequiresExactAllowedRequestURIForQuery(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("a request with an unreviewed query reached upstream")
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, []string{"get_document"})
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_document"}}`)
	request := signedHandlerRequestForURI(t, key, upstream.URL, "/mcp?q=2", body, now, strings.Repeat("4", 32))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || recorder.Header().Get(HeaderError) != "request_uri_denied" {
		t.Fatalf("status = %d, code = %q", recorder.Code, recorder.Header().Get(HeaderError))
	}
}

func TestHandlerRejectsNonCanonicalTargetAndContentType(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("a non-canonical request reached upstream")
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, nil)
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"jsonrpc":"2.0","method":"tools/list"}`)

	nonCanonicalTarget := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("d", 32))
	nonCanonicalTarget.Header.Set(HeaderTarget, " "+upstream.URL)
	assertSidecarRejection(t, handler, nonCanonicalTarget, http.StatusForbidden, "target_invalid")

	wrongContentType := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("e", 32))
	wrongContentType.Header.Set("Content-Type", "text/plain")
	assertSidecarRejection(t, handler, wrongContentType, http.StatusForbidden, "policy_denied")
}

func TestHandlerRejectsDuplicateProtocolHeader(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("a request with duplicate protocol headers reached upstream")
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, nil)
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"jsonrpc":"2.0","method":"tools/list"}`)
	for index, name := range []string{HeaderKeyID, "Content-Type", "Mcp-Session-Id"} {
		request := signedHandlerRequest(t, key, upstream.URL, body, now, fmt.Sprintf("%032x", index+30))
		if name == "Mcp-Session-Id" {
			request.Header.Set(name, "session-a")
		}
		request.Header.Add(name, request.Header.Get(name))
		assertSidecarRejection(t, handler, request, http.StatusBadRequest, "protocol_header_invalid")
	}
}

func TestHandlerRejectsAmbiguousJSONRPCFields(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("ambiguous JSON-RPC reached upstream")
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, []string{"allowed_tool"})
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	cases := []string{
		`{"jsonrpc":"2.0","method":"tools/list","method":"tools/call","params":{"name":"allowed_tool"}}`,
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"denied_tool","name":"allowed_tool"}}`,
		`{"jsonrpc":"2.0","method":" tools/call","params":{"name":"allowed_tool"}}`,
		`{"jsonrpc":"2.0","method":"tools/call","params":{"name":" allowed_tool"}}`,
	}
	for index, raw := range cases {
		body := []byte(raw)
		request := signedHandlerRequest(t, key, upstream.URL, body, now, fmt.Sprintf("%032x", index+10))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusForbidden || recorder.Header().Get(HeaderError) != "policy_denied" {
			t.Errorf("case %d: status = %d, code = %q", index, recorder.Code, recorder.Header().Get(HeaderError))
		}
	}
}

func TestHandlerReplaysDoNotConsumeRateBudget(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		_, _ = io.WriteString(response, `{"jsonrpc":"2.0","result":{}}`)
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, []string{"get_document"})
	config.Policies[0].RequestsPerMinute = 2
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_document"}}`)

	firstNonce := strings.Repeat("5", 32)
	for attempt, nonce := range []string{firstNonce, firstNonce, strings.Repeat("6", 32)} {
		request := signedHandlerRequest(t, key, upstream.URL, body, now, nonce)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		want := http.StatusOK
		if attempt == 1 {
			want = http.StatusConflict
		}
		if recorder.Code != want {
			t.Fatalf("attempt %d status = %d, want %d, code = %q", attempt, recorder.Code, want, recorder.Header().Get(HeaderError))
		}
	}
	if upstreamCalls.Load() != 2 {
		t.Fatalf("upstream calls = %d, want 2", upstreamCalls.Load())
	}
}

func TestHandlerRateRejectedNonceCanRetryInNextWindow(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(response, `{"jsonrpc":"2.0","result":{}}`)
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, []string{"get_document"})
	config.Policies[0].RequestsPerMinute = 1
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1700000000, 0).Truncate(time.Minute).Add(59 * time.Second)
	current := base
	handler.now = func() time.Time { return current }
	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_document"}}`)

	first := signedHandlerRequest(t, key, upstream.URL, body, base, strings.Repeat("7", 32))
	firstRecorder := httptest.NewRecorder()
	handler.ServeHTTP(firstRecorder, first)
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first status = %d", firstRecorder.Code)
	}

	retryNonce := strings.Repeat("8", 32)
	rateLimited := signedHandlerRequest(t, key, upstream.URL, body, base, retryNonce)
	rateRecorder := httptest.NewRecorder()
	handler.ServeHTTP(rateRecorder, rateLimited)
	if rateRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited status = %d", rateRecorder.Code)
	}

	current = base.Add(2 * time.Second)
	retry := signedHandlerRequest(t, key, upstream.URL, body, base, retryNonce)
	retryRecorder := httptest.NewRecorder()
	handler.ServeHTTP(retryRecorder, retry)
	if retryRecorder.Code != http.StatusOK {
		t.Fatalf("retry status = %d, code = %q", retryRecorder.Code, retryRecorder.Header().Get(HeaderError))
	}
}

func TestHandlerRejectsUnknownDisabledExpiredKeysAndTokenFailure(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("a rejected binding reached upstream")
	}))
	defer upstream.Close()
	now := time.Unix(1700000000, 0)
	body := []byte(`{"jsonrpc":"2.0","method":"tools/list"}`)

	t.Run("unknown", func(t *testing.T) {
		config, key := testServerConfig(t, upstream.URL, nil)
		handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.now = func() time.Time { return now }
		request := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("9", 32))
		request.Header.Set(HeaderKeyID, "unknown-key")
		assertSidecarRejection(t, handler, request, http.StatusUnauthorized, "unknown_key")
	})

	for _, testCase := range []struct {
		name   string
		code   string
		mutate func(*Binding)
	}{
		{name: "disabled", code: "key_disabled", mutate: func(binding *Binding) { binding.Enabled = false }},
		{name: "expired", code: "key_expired", mutate: func(binding *Binding) { binding.ExpiresAt = now }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config, key := testServerConfig(t, upstream.URL, nil)
			testCase.mutate(&config.Bindings[0])
			handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
			if err != nil {
				t.Fatal(err)
			}
			handler.now = func() time.Time { return now }
			request := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("a", 32))
			assertSidecarRejection(t, handler, request, http.StatusForbidden, testCase.code)
		})
	}

	t.Run("token failure", func(t *testing.T) {
		config, key := testServerConfig(t, upstream.URL, nil)
		handler, err := NewHandler(config, failingTokenResolver{}, upstream.Client(), nil)
		if err != nil {
			t.Fatal(err)
		}
		handler.now = func() time.Time { return now }
		request := signedHandlerRequest(t, key, upstream.URL, body, now, strings.Repeat("b", 32))
		assertSidecarRejection(t, handler, request, http.StatusUnauthorized, "token_resolution_failed")
	})
}

func TestHandlerAllowsInitializeAndToolsList(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(response, `{"jsonrpc":"2.0","result":{}}`)
	}))
	defer upstream.Close()
	config, key := testServerConfig(t, upstream.URL, nil)
	handler, err := NewHandler(config, staticTokenResolver{token: "token"}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	for index, method := range []string{"initialize", "tools/list"} {
		body := []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":%q}`, method))
		request := signedHandlerRequest(t, key, upstream.URL, body, now, fmt.Sprintf("%032x", index+20))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status = %d, code = %q", method, recorder.Code, recorder.Header().Get(HeaderError))
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("upstream calls = %d, want 2", calls.Load())
	}
}

func TestHandlerKeyCannotUseAnotherBindingPolicy(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("key-a reached key-b's endpoint")
	}))
	defer upstream.Close()
	config, keys := twoProfileServerConfig(t, upstream.URL)
	handler, err := NewHandler(config, profileTokenResolver{}, upstream.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	handler.now = func() time.Time { return now }
	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"get_document"}}`)
	request := signedRequestForKey(t, keys["sandbox-a"], "sandbox-a", upstream.URL, "/mcp-b", body, now, strings.Repeat("c", 32))
	assertSidecarRejection(t, handler, request, http.StatusForbidden, "path_denied")
}

func assertSidecarRejection(t *testing.T, handler *Handler, request *http.Request, status int, code string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != status || recorder.Header().Get(HeaderError) != code {
		t.Fatalf("status = %d, code = %q; want %d/%s", recorder.Code, recorder.Header().Get(HeaderError), status, code)
	}
}
