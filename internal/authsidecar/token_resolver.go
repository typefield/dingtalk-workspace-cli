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
	"fmt"
	"io"
	"log/slog"
	"strings"

	authpkg "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
)

type DWSProfileTokenResolver struct {
	configDir string
	logger    *slog.Logger
}

func NewDWSProfileTokenResolver(configDir string, logger *slog.Logger) (*DWSProfileTokenResolver, error) {
	if strings.TrimSpace(configDir) == "" {
		return nil, fmt.Errorf("DWS config directory is empty")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &DWSProfileTokenResolver{configDir: configDir, logger: logger}, nil
}

func (r *DWSProfileTokenResolver) ResolveAccessToken(ctx context.Context, profile string) (string, error) {
	if _, _, ok := authpkg.ParseIdentitySelector(profile); !ok {
		return "", fmt.Errorf("profile must be an exact corpId:userId selector")
	}
	provider := authpkg.NewOAuthProvider(r.configDir, r.logger)
	snapshot, err := provider.GetTokenSnapshotForProfile(ctx, profile)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(snapshot.AccessToken)
	if token == "" {
		return "", fmt.Errorf("resolved access token is empty")
	}
	return token, nil
}
