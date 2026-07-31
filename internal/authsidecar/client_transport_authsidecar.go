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
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type clientRoundTripper struct {
	config ClientConfig
	route  http.RoundTripper
	now    func() time.Time
	nonce  func() (string, error)
}

type errorRoundTripper struct{ err error }

func (r errorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, r.err
}

func WrapRoundTripper(base http.RoundTripper) http.RoundTripper {
	if !SidecarModeRequested() {
		return base
	}
	config, err := LoadClientConfigFromEnv()
	if err != nil {
		return errorRoundTripper{err: fmt.Errorf("invalid sidecar client configuration: %w", err)}
	}
	return &clientRoundTripper{
		config: config,
		route:  sidecarRouteTransport(config.Address),
		now:    time.Now,
		nonce:  NewNonce,
	}
}

func sidecarRouteTransport(address Address) http.RoundTripper {
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = 30 * time.Second
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, address.Network, address.Value)
	}
	return transport
}

func (t *clientRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	managed, err := sidecarManagedRequest(request)
	if err != nil {
		return nil, err
	}
	if !managed {
		return nil, fmt.Errorf("sidecar_unmanaged_request: authenticated sidecar mode refuses requests without both sentinel headers")
	}
	if request.URL.User != nil {
		return nil, fmt.Errorf("sidecar_target_userinfo_forbidden: target URL must not contain user information")
	}
	body, err := readAndRestoreRequestBody(request)
	if err != nil {
		return nil, fmt.Errorf("read request body for sidecar signing: %w", err)
	}
	targetOrigin, err := ValidateTargetOrigin(request.URL.Scheme + "://" + request.URL.Host)
	if err != nil {
		return nil, err
	}
	nonce, err := t.nonce()
	if err != nil {
		return nil, err
	}
	timestamp := strconv.FormatInt(t.now().Unix(), 10)
	canonical := CanonicalRequest{
		KeyID:        t.config.KeyID,
		Timestamp:    timestamp,
		Nonce:        nonce,
		Method:       request.Method,
		TargetOrigin: targetOrigin,
		PathAndQuery: request.URL.RequestURI(),
		BodySHA256:   BodySHA256(body),
	}

	proxied := request.Clone(request.Context())
	proxied.Header = request.Header.Clone()
	proxied.Body = io.NopCloser(bytes.NewReader(body))
	proxied.ContentLength = int64(len(body))
	proxied.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	stripCredentialHeaders(proxied.Header)
	proxied.Header.Set(HeaderVersion, ProtocolV1)
	proxied.Header.Set(HeaderKeyID, canonical.KeyID)
	proxied.Header.Set(HeaderTarget, canonical.TargetOrigin)
	proxied.Header.Set(HeaderTimestamp, canonical.Timestamp)
	proxied.Header.Set(HeaderNonce, canonical.Nonce)
	proxied.Header.Set(HeaderBodySHA256, canonical.BodySHA256)
	proxied.Header.Set(HeaderSignature, Sign(t.config.Key, canonical))
	proxied.URL.Scheme = "http"
	proxied.URL.Host = t.config.Address.URLHost
	proxied.Host = t.config.Address.URLHost
	return t.route.RoundTrip(proxied)
}

func sidecarManagedRequest(request *http.Request) (bool, error) {
	if request == nil || request.URL == nil {
		return false, fmt.Errorf("sidecar request is nil")
	}
	authorization := strings.TrimSpace(request.Header.Get("Authorization"))
	userToken := strings.TrimSpace(request.Header.Get("x-user-access-token"))
	wantAuthorization := "Bearer " + SentinelUserToken
	if authorization == "" && userToken == "" {
		return false, nil
	}
	if authorization != wantAuthorization || userToken != SentinelUserToken {
		return false, fmt.Errorf("sidecar_auth_header_mismatch: both DWS user authentication headers must contain the sidecar sentinel")
	}
	return true, nil
}

func readAndRestoreRequestBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(request.Body)
	closeErr := request.Body.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}
