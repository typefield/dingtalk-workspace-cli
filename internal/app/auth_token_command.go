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

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	authpkg "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/config"
	"github.com/spf13/cobra"
)

// newAuthTokenCommand creates the `dws auth token` subcommand.
//
// dws auth token outputs the current valid access token (auto-refreshed).
// Other CLIs and Agent scripts can consume it via stdout piping without
// performing their own OAuth login.
//
// Usage:
//
//	dws auth token              # pure token string
//	dws auth token --format json # token + metadata (corp/user/expires)
//	dws auth token --profile cid:uid
func newAuthTokenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "输出当前有效 access token（自动刷新，不暴露 refresh token）",
		Long: `输出当前用户的 DingTalk access token。

自动刷新过期 token（access 2h / refresh 30d）。只输出 access token，
不输出 refresh token——确保最小暴露面。

其他 CLI / Agent 脚本可通过 stdout 消费：
  TOKEN=$(dws auth token)
  curl -H "Authorization: Bearer $TOKEN" https://api.dingtalk.com/...

或用 dws auth inject 直接注入到子进程环境变量（更安全）。

退出码:
  0  成功
  1  未登录或 token 失效（需 dws auth login）
`,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			configDir := config.DefaultConfigDir()

			token, err := resolveAccessTokenFromDir(ctx, configDir)
			if err != nil {
				return fmt.Errorf("获取 token 失败: %w\n请运行 dws auth login", err)
			}
			if strings.TrimSpace(token) == "" {
				return fmt.Errorf("token 为空，请运行 dws auth login")
			}

			format, _ := cmd.Root().PersistentFlags().GetString("format")
			if format == "json" {
				return outputTokenJSON(token, configDir)
			}

			// Default: pure token string.
			fmt.Println(token)
			return nil
		},
	}
	return cmd
}

// tokenJSONResponse is the JSON output of `dws auth token --format json`.
type tokenJSONResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	CorpID      string `json:"corp_id,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	UserName    string `json:"user_name,omitempty"`
}

func outputTokenJSON(token, configDir string) error {
	resp := tokenJSONResponse{AccessToken: token}

	if data, err := authpkg.LoadTokenData(configDir); err == nil && data != nil {
		if !data.ExpiresAt.IsZero() {
			resp.ExpiresAt = data.ExpiresAt.Format(time.RFC3339)
		}
		resp.CorpID = data.CorpID
		resp.UserID = data.UserID
		resp.UserName = data.UserName
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}
