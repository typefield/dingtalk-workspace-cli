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
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	authpkg "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
)

type DWSProfileTokenResolver struct {
	configDir   string
	logger      *slog.Logger
	oauthClient *http.Client
}

func NewDWSProfileTokenResolver(configDir string, logger *slog.Logger) (*DWSProfileTokenResolver, error) {
	if strings.TrimSpace(configDir) == "" {
		return nil, fmt.Errorf("DWS config directory is empty")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &DWSProfileTokenResolver{
		configDir:   configDir,
		logger:      logger,
		oauthClient: hardenedOAuthClient(),
	}, nil
}

// hardenedOAuthClient never inherits environment proxies and never follows
// redirects: a 307/308 must not re-send a refresh token or client secret to a
// different origin.
func hardenedOAuthClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("OAuth redirects are disabled by the auth sidecar")
		},
	}
}

func (r *DWSProfileTokenResolver) ResolveAccessToken(ctx context.Context, profile string) (string, error) {
	corpID, userID, err := ParseExactIdentitySelector(profile)
	if err != nil {
		return "", err
	}
	provider := authpkg.NewOAuthProvider(r.configDir, r.logger)
	provider.SetHTTPClient(r.oauthClient)
	snapshot, err := provider.GetTokenSnapshotForProfile(ctx, profile)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(snapshot.CorpID) != corpID || strings.TrimSpace(snapshot.UserID) != userID {
		return "", fmt.Errorf("resolved credential identity does not literally match the bound corpId:userId selector")
	}
	token := strings.TrimSpace(snapshot.AccessToken)
	if token == "" {
		return "", fmt.Errorf("resolved access token is empty")
	}
	return token, nil
}
