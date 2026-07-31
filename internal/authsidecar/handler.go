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
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TokenResolver interface {
	ResolveAccessToken(context.Context, string) (string, error)
}

type Handler struct {
	bindings map[string]*Binding
	policies map[string]*Policy
	resolver TokenResolver
	client   *http.Client
	replay   *ReplayCache
	logger   *slog.Logger
	now      func() time.Time
	rates    *rateStore
}

type rpcRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type toolCallParams struct {
	Name string `json:"name"`
}

type problem struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type rateStore struct {
	mu      sync.Mutex
	windows map[string]rateWindow
}

type rateWindow struct {
	minute int64
	count  int
}

func NewHandler(config *ServerConfig, resolver TokenResolver, client *http.Client, logger *slog.Logger) (*Handler, error) {
	if config == nil {
		return nil, fmt.Errorf("sidecar config is nil")
	}
	if resolver == nil {
		return nil, fmt.Errorf("token resolver is nil")
	}
	bindings := make(map[string]*Binding, len(config.Bindings))
	for index := range config.Bindings {
		binding := &config.Bindings[index]
		if len(binding.key) < 32 {
			return nil, fmt.Errorf("binding %q has no prepared key", binding.KeyID)
		}
		bindings[binding.KeyID] = binding
	}
	policies := make(map[string]*Policy, len(config.Policies))
	for index := range config.Policies {
		policy := &config.Policies[index]
		if policy.origins == nil || policy.paths == nil || policy.tools == nil {
			return nil, fmt.Errorf("policy %q is not prepared", policy.Name)
		}
		policies[policy.Name] = policy
	}
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		client = &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return fmt.Errorf("upstream redirects are disabled by the auth sidecar")
			},
		}
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Handler{
		bindings: bindings,
		policies: policies,
		resolver: resolver,
		client:   client,
		replay:   NewReplayCache(10_000, 2*MaxTimestampDrift),
		logger:   logger,
		now:      time.Now,
		rates:    &rateStore{windows: make(map[string]rateWindow)},
	}, nil
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	started := h.now()
	for _, name := range []string{HeaderVersion, HeaderKeyID, HeaderTarget, HeaderTimestamp, HeaderNonce, HeaderBodySHA256, HeaderSignature} {
		if len(request.Header.Values(name)) != 1 {
			h.reject(response, request, http.StatusBadRequest, "protocol_header_invalid", "each sidecar protocol header must appear exactly once", "", "", started)
			return
		}
	}
	keyID := strings.TrimSpace(request.Header.Get(HeaderKeyID))
	binding := h.bindings[keyID]
	if binding == nil {
		h.reject(response, request, http.StatusUnauthorized, "unknown_key", "sidecar key is not recognized", keyID, "", started)
		return
	}
	if !binding.Enabled {
		h.reject(response, request, http.StatusForbidden, "key_disabled", "sidecar key is disabled", keyID, binding.Profile, started)
		return
	}
	now := h.now()
	if !binding.ExpiresAt.IsZero() && !binding.ExpiresAt.After(now) {
		h.reject(response, request, http.StatusForbidden, "key_expired", "sidecar key has expired", keyID, binding.Profile, started)
		return
	}
	policy := h.policies[binding.Policy]
	if policy == nil {
		h.reject(response, request, http.StatusForbidden, "policy_missing", "sidecar policy is unavailable", keyID, binding.Profile, started)
		return
	}
	if request.Header.Get(HeaderVersion) != ProtocolV1 {
		h.reject(response, request, http.StatusBadRequest, "unsupported_version", "unsupported sidecar protocol version", keyID, binding.Profile, started)
		return
	}
	timestamp := strings.TrimSpace(request.Header.Get(HeaderTimestamp))
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || math.Abs(float64(now.Unix()-seconds)) > MaxTimestampDrift.Seconds() {
		h.reject(response, request, http.StatusUnauthorized, "timestamp_invalid", "sidecar timestamp is invalid or outside the allowed drift", keyID, binding.Profile, started)
		return
	}
	nonce := strings.TrimSpace(request.Header.Get(HeaderNonce))
	if err := ValidateNonce(nonce); err != nil {
		h.reject(response, request, http.StatusBadRequest, "nonce_invalid", err.Error(), keyID, binding.Profile, started)
		return
	}
	body, err := readLimitedBody(request.Body, policy.MaxBodyBytes)
	if err != nil {
		h.reject(response, request, http.StatusRequestEntityTooLarge, "body_too_large", err.Error(), keyID, binding.Profile, started)
		return
	}
	claimedBodyHash := strings.TrimSpace(request.Header.Get(HeaderBodySHA256))
	if claimedBodyHash == "" || claimedBodyHash != BodySHA256(body) {
		h.reject(response, request, http.StatusBadRequest, "body_hash_mismatch", "request body digest does not match", keyID, binding.Profile, started)
		return
	}
	target, err := ValidateTargetOrigin(request.Header.Get(HeaderTarget))
	if err != nil {
		h.reject(response, request, http.StatusForbidden, "target_invalid", err.Error(), keyID, binding.Profile, started)
		return
	}
	canonical := CanonicalRequest{
		KeyID:        keyID,
		Timestamp:    timestamp,
		Nonce:        nonce,
		Method:       request.Method,
		TargetOrigin: target,
		PathAndQuery: request.URL.RequestURI(),
		BodySHA256:   claimedBodyHash,
	}
	if err := VerifySignature(binding.key, canonical, request.Header.Get(HeaderSignature)); err != nil {
		h.reject(response, request, http.StatusUnauthorized, "signature_invalid", "sidecar request signature is invalid", keyID, binding.Profile, started)
		return
	}
	if _, allowed := policy.origins[target]; !allowed {
		h.reject(response, request, http.StatusForbidden, "target_denied", "target origin is not allowed by sidecar policy", keyID, binding.Profile, started)
		return
	}
	if _, allowed := policy.paths[request.URL.Path]; !allowed {
		h.reject(response, request, http.StatusForbidden, "path_denied", "target path is not allowed by sidecar policy", keyID, binding.Profile, started)
		return
	}
	if request.Method != http.MethodPost {
		h.reject(response, request, http.StatusForbidden, "policy_denied", "MCP sidecar requests must use POST", keyID, binding.Profile, started)
		return
	}
	operation, tool, err := authorizeRPC(policy, body)
	if err != nil {
		h.reject(response, request, http.StatusForbidden, "policy_denied", err.Error(), keyID, binding.Profile, started)
		return
	}
	if !h.rates.allow(keyID, policy.RequestsPerMinute, now) {
		h.reject(response, request, http.StatusTooManyRequests, "rate_limited", "sidecar request rate exceeded", keyID, binding.Profile, started)
		return
	}
	// Consume the nonce only after every policy and rate gate has passed, so
	// rejected floods cannot occupy replay-protection state.
	if err := h.replay.Use(keyID, nonce, now); err != nil {
		code := "replay_detected"
		status := http.StatusConflict
		if !strings.Contains(err.Error(), "replay detected") {
			code = "replay_cache_unavailable"
			status = http.StatusServiceUnavailable
		}
		h.reject(response, request, status, code, err.Error(), keyID, binding.Profile, started)
		return
	}
	token, err := h.resolver.ResolveAccessToken(request.Context(), binding.Profile)
	if err != nil || strings.TrimSpace(token) == "" {
		h.reject(response, request, http.StatusUnauthorized, "token_resolution_failed", "trusted sidecar could not resolve the bound profile token", keyID, binding.Profile, started)
		return
	}
	upstream, err := http.NewRequestWithContext(request.Context(), request.Method, target+request.URL.RequestURI(), bytes.NewReader(body))
	if err != nil {
		h.reject(response, request, http.StatusBadGateway, "upstream_request_invalid", "failed to construct upstream request", keyID, binding.Profile, started)
		return
	}
	copyForwardHeaders(upstream.Header, request.Header)
	stripCredentialHeaders(upstream.Header)
	upstream.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	upstream.Header.Set("x-user-access-token", strings.TrimSpace(token))
	result, err := h.client.Do(upstream)
	if err != nil {
		h.reject(response, request, http.StatusBadGateway, "upstream_unavailable", "sidecar upstream request failed", keyID, binding.Profile, started)
		return
	}
	defer result.Body.Close()
	copyResponseHeaders(response.Header(), result.Header)
	response.WriteHeader(result.StatusCode)
	_, copyErr := io.Copy(response, result.Body)
	h.logger.Info("sidecar_forward",
		"key", shortHash(keyID),
		"profile", shortHash(binding.Profile),
		"policy", binding.Policy,
		"operation", operation,
		"tool", tool,
		"target", target,
		"status", result.StatusCode,
		"duration_ms", time.Since(started).Milliseconds(),
		"response_copy_error", copyErr != nil,
	)
}

func authorizeRPC(policy *Policy, body []byte) (operation, tool string, err error) {
	var rpc rpcRequest
	if err := json.Unmarshal(body, &rpc); err != nil {
		return "", "", fmt.Errorf("request is not valid JSON-RPC JSON")
	}
	operation = strings.TrimSpace(rpc.Method)
	switch operation {
	case "initialize", "notifications/initialized", "tools/list":
		return operation, "", nil
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(rpc.Params, &params); err != nil || strings.TrimSpace(params.Name) == "" {
			return operation, "", fmt.Errorf("tools/call has no valid tool name")
		}
		tool = strings.TrimSpace(params.Name)
		if _, allowed := policy.tools[tool]; !allowed {
			return operation, tool, fmt.Errorf("MCP tool %q is not allowed by sidecar policy", tool)
		}
		return operation, tool, nil
	default:
		return operation, "", fmt.Errorf("JSON-RPC method %q is not allowed by sidecar policy", operation)
	}
}

func readLimitedBody(body io.ReadCloser, maximum int64) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, maximum+1))
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("request body exceeds %d bytes", maximum)
	}
	return data, nil
}

func (h *Handler) reject(response http.ResponseWriter, request *http.Request, status int, code, message, keyID, profile string, started time.Time) {
	response.Header().Set("Content-Type", "application/problem+json")
	response.Header().Set(HeaderError, code)
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(problem{Code: code, Message: message})
	h.logger.Warn("sidecar_reject",
		"key", shortHash(keyID),
		"profile", shortHash(profile),
		"code", code,
		"method", request.Method,
		"path", sanitizeAuditPath(request.URL.Path),
		"duration_ms", time.Since(started).Milliseconds(),
	)
}

func (s *rateStore) allow(keyID string, limit int, now time.Time) bool {
	if limit == 0 {
		return true
	}
	minute := now.Unix() / 60
	s.mu.Lock()
	defer s.mu.Unlock()
	window := s.windows[keyID]
	if window.minute != minute {
		window = rateWindow{minute: minute}
	}
	if window.count >= limit {
		s.windows[keyID] = window
		return false
	}
	window.count++
	s.windows[keyID] = window
	return true
}

func stripCredentialHeaders(header http.Header) {
	for _, name := range []string{
		"Authorization", "x-user-access-token", "Cookie", "Proxy-Authorization",
		HeaderVersion, HeaderKeyID, HeaderTarget, HeaderTimestamp, HeaderNonce,
		HeaderBodySHA256, HeaderSignature,
	} {
		header.Del(name)
	}
}

func copyForwardHeaders(destination, source http.Header) {
	for _, name := range []string{"Content-Type", "Accept", "X-Cli-Source", "X-Cli-Version", "X-Cli-Execution-Id"} {
		for _, value := range source.Values(name) {
			destination.Add(name, value)
		}
	}
}

// forwardableResponseHeaders is an allowlist: upstream credential echoes such
// as Authorization or x-user-access-token and hop-by-hop headers must never
// reach the sandbox.
var forwardableResponseHeaders = []string{
	"Content-Type",
	"Content-Length",
	"Content-Encoding",
	"Content-Language",
	"Cache-Control",
	"Date",
	"Retry-After",
	"Mcp-Session-Id",
}

func copyResponseHeaders(destination, source http.Header) {
	for _, name := range forwardableResponseHeaders {
		for _, value := range source.Values(name) {
			destination.Add(name, value)
		}
	}
}

func shortHash(value string) string {
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:6])
}

func sanitizeAuditPath(path string) string {
	if len(path) > 256 {
		return path[:256]
	}
	return path
}
