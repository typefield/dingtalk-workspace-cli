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

// Package authsidecar implements the same-host authentication sidecar protocol.
// The sandbox owns only a scoped HMAC key; real DingTalk credentials remain in
// the trusted sidecar process.
package authsidecar

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	ProtocolV1 = "v1"

	HeaderVersion    = "X-DWS-Sidecar-Version"
	HeaderKeyID      = "X-DWS-Sidecar-Key-Id"
	HeaderTarget     = "X-DWS-Sidecar-Target"
	HeaderTimestamp  = "X-DWS-Sidecar-Timestamp"
	HeaderNonce      = "X-DWS-Sidecar-Nonce"
	HeaderBodySHA256 = "X-DWS-Sidecar-Body-SHA256"
	HeaderSignature  = "X-DWS-Sidecar-Signature"
	HeaderError      = "X-DWS-Sidecar-Error"

	SentinelUserToken = "sidecar-managed-user-token"
	MaxTimestampDrift = 60 * time.Second
)

// CanonicalRequest is the complete set of caller-controlled routing fields
// authenticated by the v1 HMAC. Profile and auth-header are deliberately not
// present: both are fixed by the trusted server.
type CanonicalRequest struct {
	KeyID        string
	Timestamp    string
	Nonce        string
	Method       string
	TargetOrigin string
	PathAndQuery string
	BodySHA256   string
}

func (r CanonicalRequest) String() string {
	return strings.Join([]string{
		"DWS-AUTHSIDECAR/" + ProtocolV1,
		r.KeyID,
		r.Timestamp,
		r.Nonce,
		strings.ToUpper(r.Method),
		r.TargetOrigin,
		r.PathAndQuery,
		r.BodySHA256,
	}, "\n")
}

func BodySHA256(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func Sign(key []byte, request CanonicalRequest) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(request.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifySignature(key []byte, request CanonicalRequest, signature string) error {
	provided, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(request.String()))
	if !hmac.Equal(mac.Sum(nil), provided) {
		return fmt.Errorf("HMAC signature mismatch")
	}
	return nil
}

func NewNonce() (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return hex.EncodeToString(nonce[:]), nil
}

func ValidateNonce(value string) error {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 16 {
		return fmt.Errorf("nonce must be 128-bit lowercase or uppercase hex")
	}
	return nil
}
