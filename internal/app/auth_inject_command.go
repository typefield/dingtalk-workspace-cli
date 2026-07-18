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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/config"
	"github.com/spf13/cobra"
)

const (
	// defaultInjectEnvVar is the environment variable name injected into child
	// processes. Other CLIs read this to consume DWS-managed tokens.
	defaultInjectEnvVar = "DWS_ACCESS_TOKEN"

	// authErrorKeywords are substrings in child stderr that indicate auth failure.
	authErrorKeywords = "token,auth,登录,认证,授权,unauthorized,expired,invalid"
)

// newAuthInjectCommand creates the `dws auth inject` subcommand.
//
// dws auth inject wraps another CLI: it fetches a valid token, injects it as
// an environment variable into the child process, then execs the target command.
// The token never appears in command-line args or shell history.
//
// Usage:
//
//	dws auth inject -- my-cli do-something
//	dws auth inject --env MY_TOKEN -- my-cli do-something
//	dws auth inject --auto-login -- my-cli do-something
func newAuthInjectCommand() *cobra.Command {
	var envVar string
	var autoLogin bool

	cmd := &cobra.Command{
		Use:   "inject -- <command> [args...]",
		Short: "获取 token 并注入到子进程环境变量，然后执行目标命令",
		Long: `获取当前有效的 DingTalk access token，注入到子进程的环境变量中，
然后执行目标命令。token 不出现在命令行参数或 shell 历史中。

环境变量:
  DWS_ACCESS_TOKEN (默认)    注入的 token 变量名
  --env <NAME>               自定义变量名

其他 CLI 通过读取 DWS_ACCESS_TOKEN 环境变量即可使用 DWS 管理的 token，
无需自己实现 OAuth 登录。

示例:
  dws auth inject -- echo $DWS_ACCESS_TOKEN
  dws auth inject --env DINGTALK_TOKEN -- my-tool api-call
  dws auth inject --auto-login -- my-tool api-call  # token 失效自动重新登录

退出码:
  0   目标命令成功
  非0  目标命令的退出码（透传）
`,
		DisableAutoGenTag: true,
		// Don't let cobra parse flags after `--` as inject's own flags.
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Extract the command after `--`.
			targetArgs := extractTargetCommand(cmd)
			if len(targetArgs) == 0 {
				return fmt.Errorf("用法: dws auth inject -- <command> [args...]")
			}

			ctx := context.Background()
			configDir := config.DefaultConfigDir()

			token, err := resolveAccessTokenFromDir(ctx, configDir)
			if err != nil {
				if autoLogin {
					return runInjectWithAutoLogin(ctx, configDir, envVar, targetArgs)
				}
				return fmt.Errorf("获取 token 失败: %w\n提示: 加 --auto-login 自动登录，或先运行 dws auth login", err)
			}

			exitCode, err := runWithToken(token, envVar, targetArgs)
			if err != nil && autoLogin && isChildAuthError(err, exitCode) {
				// Auth error detected — retry with fresh login.
				return runInjectWithAutoLogin(ctx, configDir, envVar, targetArgs)
			}
			// Forward the child's exit code.
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return err
		},
	}

	cmd.Flags().StringVar(&envVar, "env", defaultInjectEnvVar, "注入的 token 环境变量名")
	cmd.Flags().BoolVar(&autoLogin, "auto-login", false, "token 失效时自动重新登录并重试一次")

	return cmd
}

// extractTargetCommand gets the command after `--` from cobra args.
// cobra puts everything after `--` into Args, but with UnknownFlags whitelist
// the flags before `--` may also leak into Args. We find `--` boundary.
func extractTargetCommand(cmd *cobra.Command) []string {
	// With FParseErrWhitelist.UnknownFlags=true, cobra passes unknown flags
	// and positional args to Args. The `--` separator itself is consumed
	// by cobra, so all remaining args ARE the target command.
	args := cmd.Flags().Args()
	if len(args) == 0 {
		// Fallback: use the command's raw args.
		args = append(args, cmd.Flags().Args()...)
	}
	// Filter out our own --env/--auto-login values that leaked.
	return filterInjectedFlags(args)
}

// filterInjectedFlags removes our own flag values that cobra may have
// left in the args when UnknownFlags=true.
func filterInjectedFlags(args []string) []string {
	var out []string
	skipNext := false
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}
		switch {
		case arg == "--env", arg == "--auto-login":
			continue
		case strings.HasPrefix(arg, "--env="), strings.HasPrefix(arg, "--auto-login="):
			continue
		case strings.HasPrefix(arg, "--env ") || strings.HasPrefix(arg, "--auto-login "):
			continue
		case arg == "--":
			// Everything after -- is the target command.
			out = append(out, args[i+1:]...)
			return out
		default:
			// Check if previous arg was --env (needs a value).
			if len(out) > 0 && out[len(out)-1] == "--env" {
				out = out[:len(out)-1] // remove the --env we accidentally kept
				continue
			}
			out = append(out, arg)
		}
	}
	return out
}

// runWithToken sets the env var and execs the target command.
func runWithToken(token, envVar string, targetArgs []string) (int, error) {
	cmd := exec.Command(targetArgs[0], targetArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), envVar+"="+token)

	var stderrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return 1, fmt.Errorf("exec %s: %w", targetArgs[0], err)
		}
	}

	// Check if stderr indicates auth failure.
	if exitCode != 0 && isAuthErrorStderr(stderrBuf.String()) {
		return exitCode, fmt.Errorf("auth error detected in child stderr")
	}

	return exitCode, err
}

// isChildAuthError checks if the error/exit code indicates auth failure.
func isChildAuthError(err error, exitCode int) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return containsAny(msg, authErrorKeywords)
}

// isAuthErrorStderr checks stderr content for auth-related keywords.
func isAuthErrorStderr(stderr string) bool {
	lower := strings.ToLower(stderr)
	return containsAny(lower, authErrorKeywords)
}

func containsAny(s, keywords string) bool {
	for _, kw := range strings.Split(keywords, ",") {
		kw = strings.TrimSpace(kw)
		if kw != "" && strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// runInjectWithAutoLogin performs login then retries the inject.
func runInjectWithAutoLogin(ctx context.Context, configDir, envVar string, targetArgs []string) error {
	// Signal to Agent that login is needed.
	fmt.Fprintf(os.Stderr, `{"action":"login_required","message":"token 失效，正在触发 Device Flow 登录"}`+"\n")

	// Attempt login via the same binary.
	loginCmd := exec.Command(os.Args[0], "auth", "login", "--device", "--yes")
	loginCmd.Stdin = os.Stdin
	loginCmd.Stdout = os.Stderr // login progress to stderr, not stdout
	loginCmd.Stderr = os.Stderr
	if err := loginCmd.Run(); err != nil {
		return fmt.Errorf("自动登录失败: %w", err)
	}

	// Retry: get fresh token and run.
	token, err := resolveAccessTokenFromDir(ctx, configDir)
	if err != nil {
		return fmt.Errorf("登录后获取 token 仍失败: %w", err)
	}

	exitCode, err := runWithToken(token, envVar, targetArgs)
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return err
}
