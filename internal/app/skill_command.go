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
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	authpkg "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/auth"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/configmeta"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/cobra"
)

var (
	skillLoadAccessToken    = loadSkillAccessToken
	skillDownloadToTmp      = downloadSkillToTmpDir
	skillHTTPDo             = func(client *http.Client, req *http.Request) (*http.Response, error) { return client.Do(req) }
	skillNewRequest         = http.NewRequestWithContext
	skillResolveAccessToken = ResolveAuxiliaryAccessToken
	skillResolveTargetPath  = resolveSkillTargetPath
	skillFetchDownloadInfo  = fetchSkillDownloadInfo
	skillDownloadFile       = downloadSkillFile
	skillExtractZip         = extractSkillZip
	skillUserHomeDir        = os.UserHomeDir
	skillMkdirTemp          = os.MkdirTemp
	skillCreate             = os.Create
	skillCreateTemp         = os.CreateTemp
	skillRemoveAll          = os.RemoveAll
	skillRemove             = os.Remove
	skillMkdirAll           = os.MkdirAll
	skillOpenFile           = os.OpenFile
	skillCopy               = io.Copy
	skillOpenZipFile        = func(file *zip.File) (io.ReadCloser, error) { return file.Open() }
)

func init() {
	configmeta.Register(configmeta.ConfigItem{
		Name:         "DWS_SKILL_API_HOST",
		Category:     configmeta.CategoryNetwork,
		Description:  "覆盖 Skill API 地址",
		DefaultValue: "https://mcp.dingtalk.com",
		Example:      "https://custom-mcp.example.com",
	})
}

const (
	// legacySkillAPIHost is the legacy skill market host used by the old cli.
	legacySkillAPIHost = "https://mcp.dingtalk.com"
	// skillDownloadTimeout is the timeout for skill download operations.
	skillDownloadTimeout = 5 * time.Minute
)

// skillDownloadEndpoint is variable so tests and private distributions can
// exercise the download flow without contacting the public service.
var skillDownloadEndpoint = "https://aihub.dingtalk.com/cli/download"

// downloadSkillResponse represents the API response for skill download.
type downloadSkillResponse struct {
	Success   bool                 `json:"success"`
	ErrorCode string               `json:"errorCode,omitempty"`
	ErrorMsg  string               `json:"errorMsg,omitempty"`
	Result    *downloadSkillResult `json:"result,omitempty"`
}

// downloadSkillResult contains the download URL and file name.
type downloadSkillResult struct {
	DownloadURL string `json:"downloadUrl"`
	FileName    string `json:"fileName"`
}

// findSkillsResponse represents the legacy skill search API response.
type findSkillsResponse struct {
	Success   bool          `json:"success"`
	ErrorCode string        `json:"errorCode,omitempty"`
	ErrorMsg  string        `json:"errorMsg,omitempty"`
	Result    []CliSkillDTO `json:"result,omitempty"`
}

// CliSkillDTO mirrors the old cli response payload for `skill search`.
type CliSkillDTO struct {
	SkillID        string `json:"skillId"`
	Name           string `json:"name"`
	Desc           string `json:"desc"`
	Icon           string `json:"icon"`
	Version        any    `json:"version,omitempty"`
	Source         any    `json:"source,omitempty"`
	SecurityStatus any    `json:"securityStatus,omitempty"`
}

type skillSearchOutput struct {
	Success bool          `json:"success"`
	Count   int           `json:"count"`
	Skills  []CliSkillDTO `json:"skills"`
}

type skillGetOutput struct {
	Success bool   `json:"success"`
	SkillID string `json:"skillId"`
	TempDir string `json:"tempDir"`
}

type skillInstallOutput struct {
	Success bool   `json:"success"`
	DryRun  bool   `json:"dry_run,omitempty"`
	SkillID string `json:"skillId"`
	Target  string `json:"target"`
	Path    string `json:"path"`
}

// agentSkillPaths maps target names to their relative skill installation paths.
// These paths are relative to the user's home directory.
//
// Source of truth for both `dws skill install <skillId> <target>` and
// `dws skill setup --target <name>`. Every entry in skillSetupAgentHomes
// (skill_setup.go) MUST have a matching path value here — enforced by
// TestAgentSkillPathsCoversSetupHomes.
var agentSkillPaths = map[string]string{
	// `agents` is the generic-agent sentinel: install scripts and `setup`
	// special-case ~/.agents/skills as a no-checks-required fallback so a
	// fresh machine without any IDE/agent registry still gets skills.
	"agents":    ".agents/skills",
	"qoder":     ".qoder/skills",
	"qoderwork": ".qoderwork/skills",
	"claude":    ".claude/skills",
	"cursor":    ".cursor/skills",
	"codex":     ".codex/skills",
	"opencode":  filepath.Join(".config", "opencode", "skills"),
	// IDE / agent registries also probed by `dws skill setup --target all`.
	"gemini":   ".gemini/skills",
	"github":   ".github/skills",
	"windsurf": ".windsurf/skills",
	"augment":  ".augment/skills",
	"cline":    ".cline/skills",
	"amp":      ".amp/skills",
	"kiro":     ".kiro/skills",
	"trae":     ".trae/skills",
	"openclaw": ".openclaw/skills",
	"hermes":   ".hermes/skills",
}

// supportedTargets returns a sorted, comma-separated list of supported
// targets. Sorted so help text and error messages stay stable across runs
// (Go map iteration order is intentionally randomized).
func supportedTargets() string {
	targets := make([]string, 0, len(agentSkillPaths)+1)
	for target := range agentSkillPaths {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	targets = append(targets, ".")
	return strings.Join(targets, ", ")
}

// longestAgentTargetName returns the character count of the longest target
// name in agentSkillPaths. Used by --help formatting to keep the "." entry
// vertically aligned with named targets.
func longestAgentTargetName() int {
	n := 0
	for name := range agentSkillPaths {
		if len(name) > n {
			n = len(name)
		}
	}
	return n
}

// formatAgentSkillPathsForHelp renders agentSkillPaths as an aligned
// "  <name> -> ~/<path>" block, sorted by name, for use in --help output.
// Keeps `dws skill install --help` in sync with the map without hand-edits.
func formatAgentSkillPathsForHelp() string {
	names := make([]string, 0, len(agentSkillPaths))
	maxWidth := 0
	for n := range agentSkillPaths {
		names = append(names, n)
		if len(n) > maxWidth {
			maxWidth = len(n)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "  %-*s -> ~/%s/\n", maxWidth, n, agentSkillPaths[n])
	}
	return b.String()
}

func buildSkillCommand() *cobra.Command {
	contract.RegisterProductDecl(contract.ProductDecl{
		ID: "skill",
		Selection: contract.ProductSelectionDecl{
			AgentSummary: "搜索、下载或安装钉钉技能市场中的 Agent Skill",
			UseWhen:      []string{"用户明确要求查找、检查或安装钉钉技能市场中的 Skill"},
			AvoidWhen:    []string{"执行具体钉钉业务能力时直接使用相应产品 Skill，不要先搜索或安装新的 Skill"},
		},
	})
	cmd := &cobra.Command{
		Use:               "skill",
		Short:             "技能管理",
		Long:              "管理钉钉技能市场的技能。支持搜索、下载与安装到指定 Agent 目录。",
		Args:              cobra.NoArgs,
		TraverseChildren:  true,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newSkillInstallCommand(),
		newSkillGetCommand(),
		newSkillSearchCommand(),
		newSkillFindHintCommand(),
		newSkillAddHintCommand(),
		newSkillSetupCommand(),
	)
	return cmd
}

func newSkillGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "get",
		Short:             "获取技能压缩文件",
		Long:              "从服务端下载技能包到本地临时目录。命令执行成功后会输出临时目录路径，供调用方使用。",
		Example:           "  dws skill get --skill-id <skillId>",
		DisableAutoGenTag: true,
		RunE:              runSkillGet,
	}
	cmd.Flags().String("skill-id", "", "技能 ID（必填）")
	_ = cmd.MarkFlagRequired("skill-id")
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "skill",
				Name:           "get",
				CanonicalPath:  "skill.get",
				CLIPath:        "skill get",
				PrimaryCLIPath: "skill get",
			},
			Description: "从钉钉技能市场下载指定技能包到本地临时目录",
			Interface: &contract.InterfaceSpec{
				Mode:         contract.InterfaceModeComposite,
				Availability: contract.InterfaceAvailable,
				Reason:       "命令通过受控技能市场 HTTP 端点下载文件并保存到本地临时目录，不对应单个 pinned MCP RPC",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "从钉钉技能市场下载指定技能包到本地临时目录",
				UseWhen:      []string{"用户明确要求下载已知 skillId 的技能包，以便本地检查或后续处理"},
				AvoidWhen:    []string{"只是查找技能时使用 skill search", "需要直接安装到 Agent 目录时使用 skill install"},
				Examples:     []string{"dws skill get --skill-id <skillId> --format json"},
			},
			Parameters: []contract.ParamDecl{{Name: "skill-id", Description: "技能市场返回的稳定 skillId"}},
		},
	})
	return cmd
}

func newSkillSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "search",
		Short:             "从钉钉技能市场搜索技能",
		Long:              "从钉钉技能市场搜索技能，根据关键词返回匹配的技能列表。",
		Example:           "  dws skill search --query 关键词",
		DisableAutoGenTag: true,
		RunE:              runSkillFind,
	}
	cmd.Flags().String("query", "", "搜索关键词（必填）")
	_ = cmd.MarkFlagRequired("query")
	cmd.Flags().String("source", "", "查询范围，空格分隔。备选值：DingtalkMarket（钉钉市场）、OrgInternal（企业内部）")
	cmd.Flags().String("scopes", "", "查询范围（已废弃，请使用 --source）")
	_ = cmd.Flags().MarkDeprecated("scopes", "请使用 --source 替代")
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "skill",
				Name:           "search",
				CanonicalPath:  "skill.search",
				CLIPath:        "skill search",
				PrimaryCLIPath: "skill search",
			},
			Description: "按关键词搜索钉钉技能市场中的技能",
			Interface: &contract.InterfaceSpec{
				Mode:         contract.InterfaceModeComposite,
				Availability: contract.InterfaceAvailable,
				Reason:       "命令通过受控技能市场 HTTP 搜索端点查询，不对应单个 pinned MCP RPC",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "按关键词搜索钉钉技能市场中的技能",
				UseWhen:      []string{"用户要查找可安装的技能，或需要取得后续下载/安装使用的真实 skillId"},
				AvoidWhen:    []string{"已经有稳定 skillId 且要下载时使用 skill get", "已经有稳定 skillId 且要安装时使用 skill install"},
				Examples: []string{
					"dws skill search --query 周报 --format json",
					"dws skill search --query 日报 --source OrgInternal --format json",
				},
			},
			Parameters: []contract.ParamDecl{
				{Name: "query", Description: "技能搜索关键词"},
				{Name: "source", Description: "可选查询范围：DingtalkMarket 或 OrgInternal"},
			},
		},
	})
	return cmd
}

func newSkillFindHintCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "find",
		Short:             "兼容旧用法，提示使用 skill search",
		Hidden:            true,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use: dws skill search --query <关键词>")
			return nil
		},
	}
}

func newSkillInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <skillId> <target>",
		Short: "下载并安装技能到指定目录",
		Long: fmt.Sprintf(`从钉钉技能市场下载技能并安装到指定 Agent 目录。

参数:
  skillId   技能 ID（必填），可从钉钉技能市场获取
  target    安装目标（必填），支持: %s

安装路径:
%s  .%s -> 当前目录

示例:
  dws skill install skill-123 claude    # 安装到 ~/.claude/skills/
  dws skill install skill-123 qoder     # 安装到 ~/.qoder/skills/
  dws skill install skill-123 .         # 安装到当前目录`,
			supportedTargets(),
			formatAgentSkillPathsForHelp(),
			strings.Repeat(" ", longestAgentTargetName()-1)),
		Args:              cobra.ExactArgs(2),
		DisableAutoGenTag: true,
		RunE:              runSkillAdd,
	}
	cli.AnnotateRuntimePositionals(cmd,
		contract.RuntimeSchemaPositional{Name: "skill_id", Type: "string", Description: "技能市场返回的稳定 skillId", Required: true, Index: 0},
		contract.RuntimeSchemaPositional{Name: "target", Type: "string", Description: "安装目标 Agent 名称或 .（当前目录）", Required: true, Index: 1},
	)
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "high",
			Confirmation: "user_required", Idempotency: "unknown",
		},
		Validate: validateSkillInstall,
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID:      "skill",
				Name:           "install",
				CanonicalPath:  "skill.install",
				CLIPath:        "skill install",
				PrimaryCLIPath: "skill install",
			},
			Description: "从钉钉技能市场下载技能，并写入指定 Agent 的技能目录",
			Positionals: []contract.RuntimeSchemaPositional{
				{Name: "skill_id", Type: "string", Description: "技能市场返回的稳定 skillId", Required: true, Index: 0},
				{Name: "target", Type: "string", Description: "安装目标 Agent 名称或 .（当前目录）", Required: true, Index: 1},
			},
			DryRun: &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewRequest, RemoteReads: false},
			Interface: &contract.InterfaceSpec{
				Mode:         contract.InterfaceModeComposite,
				Availability: contract.InterfaceAvailable,
				Reason:       "命令从受控技能市场下载 ZIP，校验归档路径后写入本地 Agent 技能目录，不对应单个 pinned MCP RPC",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "从钉钉技能市场下载技能，并写入指定 Agent 的技能目录",
				UseWhen:      []string{"用户已确认真实 skillId、安装目标和安全状态，并明确要求安装该市场技能"},
				AvoidWhen: []string{
					"还没有稳定 skillId 时先用 skill search",
					"只想下载归档检查、不想修改 Agent 目录时使用 skill get",
					"用户尚未确认第三方技能代码和目标目录时不要执行",
				},
				Examples: []string{
					"dws skill install <skillId> codex --dry-run --format json",
				},
			},
		},
	})

	return cmd
}

func newSkillAddHintCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "add",
		Short:             "兼容旧用法，提示使用 skill install",
		Hidden:            true,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "use: dws skill install <skillId> <target>")
			return nil
		},
	}
}

func runSkillGet(cmd *cobra.Command, args []string) error {
	skillID, _ := cmd.Flags().GetString("skill-id")
	accessToken, err := skillLoadAccessToken(cmd.Context())
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("%s/cli/install?skillId=%s", skillAPIHost(), url.QueryEscape(strings.TrimSpace(skillID)))
	jsonOutput := commandRequestsJSONErrors(cmd)
	if !jsonOutput {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "⬇️  下载技能包...")
	}

	tmpDir, err := skillDownloadToTmp(cmd.Context(), apiURL, accessToken)
	if err != nil {
		return err
	}

	if jsonOutput {
		return output.WriteJSON(cmd.OutOrStdout(), skillGetOutput{Success: true, SkillID: strings.TrimSpace(skillID), TempDir: tmpDir})
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), tmpDir)
	return nil
}

func runSkillFind(cmd *cobra.Command, args []string) error {
	keyword, _ := cmd.Flags().GetString("query")
	source, _ := cmd.Flags().GetString("source")
	if source == "" {
		source, _ = cmd.Flags().GetString("scopes")
	}
	accessToken, err := skillLoadAccessToken(cmd.Context())
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("%s/cli/find-skills?keyword=%s", skillAPIHost(), url.QueryEscape(strings.TrimSpace(keyword)))
	if source != "" {
		apiURL += "&source=" + url.QueryEscape(source)
	}
	req, err := skillNewRequest(cmd.Context(), http.MethodGet, apiURL, nil)
	if err != nil {
		return apperrors.NewInternal(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("x-user-access-token", accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := skillHTTPDo(client, req)
	if err != nil {
		return apperrors.NewAPI(fmt.Sprintf("failed to search skills: %v", err), apperrors.WithRetryable(true))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return parseLegacySkillAPIError(resp)
	}

	var result findSkillsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return apperrors.NewAPI(fmt.Sprintf("failed to parse search response: %v", err))
	}
	if !result.Success {
		errMsg := strings.TrimSpace(result.ErrorMsg)
		if errMsg == "" {
			errMsg = strings.TrimSpace(result.ErrorCode)
		}
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return apperrors.NewAPI(fmt.Sprintf("failed to search skills: %s", errMsg))
	}

	if result.Result == nil {
		result.Result = []CliSkillDTO{}
	}
	if commandRequestsJSONErrors(cmd) {
		return output.WriteJSON(cmd.OutOrStdout(), skillSearchOutput{Success: true, Count: len(result.Result), Skills: result.Result})
	}
	if len(result.Result) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "未找到匹配的技能")
		return nil
	}

	for _, skill := range result.Result {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "SkillID: %s\n", skill.SkillID)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\n", skill.Name)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Desc: %s\n", skill.Desc)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "---")
	}
	return nil
}

func runSkillAdd(cmd *cobra.Command, args []string) error {
	skillID := strings.TrimSpace(args[0])
	target := strings.TrimSpace(args[1])

	if skillID == "" {
		return apperrors.NewValidation("skillId is required")
	}

	// Resolve target path
	destPath, err := skillResolveTargetPath(target)
	if err != nil {
		return apperrors.NewValidation(fmt.Sprintf("invalid target '%s': %v. Supported targets: %s", target, err, supportedTargets()))
	}

	dryRun, _ := cmd.Root().PersistentFlags().GetBool("dry-run")
	if dryRun {
		result := skillInstallOutput{Success: true, DryRun: true, SkillID: skillID, Target: target, Path: destPath}
		if commandRequestsJSONErrors(cmd) {
			return output.WriteJSON(cmd.OutOrStdout(), result)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "[DRY-RUN] 将从技能市场下载 skillId=%s，并安装到 %s（不发请求、不写文件）\n", skillID, destPath)
		return nil
	}

	accessToken, err := skillLoadAccessToken(cmd.Context())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), skillDownloadTimeout)
	defer cancel()

	w := cmd.OutOrStdout()
	jsonOutput := commandRequestsJSONErrors(cmd)

	// Step 1: Get download URL from API
	if !jsonOutput {
		fmt.Fprintf(w, "正在获取技能信息...\n")
	}
	downloadResp, err := skillFetchDownloadInfo(ctx, accessToken, skillID)
	if err != nil {
		return err
	}

	if !downloadResp.Success {
		errMsg := downloadResp.ErrorMsg
		if errMsg == "" {
			errMsg = downloadResp.ErrorCode
		}
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return apperrors.NewAPI(fmt.Sprintf("failed to get skill download info: %s", errMsg),
			apperrors.WithOperation("skill.install.download_info"),
			apperrors.WithSubtype(apperrors.SubtypeSkillDownloadInfoUnavailable),
			apperrors.WithServerDiag(apperrors.ServerDiagnostics{
				ServerErrorCode: strings.TrimSpace(downloadResp.ErrorCode),
			}),
		)
	}

	if downloadResp.Result == nil || downloadResp.Result.DownloadURL == "" {
		return apperrors.NewAPI("skill download URL not found in response")
	}

	// Step 2: Download the skill zip file
	if !jsonOutput {
		fmt.Fprintf(w, "正在下载技能...\n")
	}
	tempZipPath, err := skillDownloadFile(ctx, downloadResp.Result.DownloadURL, downloadResp.Result.FileName)
	if err != nil {
		return err
	}
	defer cleanupTempFile(tempZipPath)

	// Step 3: Extract zip to destination
	if !jsonOutput {
		fmt.Fprintf(w, "正在解压到 %s...\n", destPath)
	}
	if err := skillExtractZip(tempZipPath, destPath); err != nil {
		return err
	}

	if jsonOutput {
		return output.WriteJSON(w, skillInstallOutput{Success: true, SkillID: skillID, Target: target, Path: destPath})
	}
	fmt.Fprintf(w, "\n[OK] 技能安装成功！\n")
	fmt.Fprintf(w, "安装路径: %s\n", destPath)

	return nil
}

func validateSkillInstall(_ *cobra.Command, args []string) error {
	if len(args) != 2 {
		return apperrors.NewValidation("skill install requires <skillId> and <target>")
	}
	if strings.TrimSpace(args[0]) == "" {
		return apperrors.NewValidation("skillId is required")
	}
	target := strings.TrimSpace(args[1])
	if _, err := skillResolveTargetPath(target); err != nil {
		return apperrors.NewValidation(fmt.Sprintf("invalid target '%s': %v. Supported targets: %s", target, err, supportedTargets()))
	}
	return nil
}

func loadSkillAccessToken(ctx context.Context) (string, error) {
	configDir := defaultConfigDir()
	token, err := skillResolveAccessToken(ctx, configDir, "")
	if errors.Is(err, authpkg.ErrTokenDataNotFound) {
		return "", skillAuthError()
	}
	if err != nil {
		return "", fmt.Errorf("resolve skill access token: %w", err)
	}
	return token, nil
}

func skillAuthError() error {
	if edition.Get().IsEmbedded {
		return apperrors.NewAuth("认证信息已失效",
			apperrors.WithSubtype(apperrors.SubtypeNotAuthenticated),
			apperrors.WithHint("请先完成钉钉账号登录后重试"),
			apperrors.WithActions("dws auth login"))
	}
	return apperrors.NewAuth("not logged in or token expired. Please run 'dws auth login' first",
		apperrors.WithSubtype(apperrors.SubtypeNotAuthenticated),
		apperrors.WithHint("请先执行 'dws auth login' 登录"),
		apperrors.WithActions("dws auth login"))
}

func skillAPIHost() string {
	if override := strings.TrimSpace(os.Getenv("DWS_SKILL_API_HOST")); override != "" {
		return strings.TrimRight(override, "/")
	}
	return legacySkillAPIHost
}

// resolveSkillTargetPath resolves the target argument to an absolute path.
func resolveSkillTargetPath(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("target is required")
	}

	// Special case: current directory
	if target == "." {
		return os.Getwd()
	}

	// Look up predefined agent paths
	relPath, ok := agentSkillPaths[strings.ToLower(target)]
	if !ok {
		return "", fmt.Errorf("unsupported target")
	}

	homeDir, err := skillUserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, relPath), nil
}

// fetchSkillDownloadInfo calls the download API to get the skill download URL.
func fetchSkillDownloadInfo(ctx context.Context, accessToken, skillID string) (*downloadSkillResponse, error) {
	url := fmt.Sprintf("%s?skillId=%s", skillDownloadEndpoint, skillID)

	req, err := skillNewRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, apperrors.NewInternal(fmt.Sprintf("failed to create request: %v", err))
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-user-access-token", accessToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := skillHTTPDo(client, req)
	if err != nil {
		return nil, apperrors.NewAPI(fmt.Sprintf("failed to call download API: %v", err),
			apperrors.WithRetryable(true))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, skillAuthError()
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.NewAPI(fmt.Sprintf("download API returned HTTP %d", resp.StatusCode),
			apperrors.WithRetryable(resp.StatusCode >= 500))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return nil, apperrors.NewAPI(fmt.Sprintf("failed to read response: %v", err))
	}

	var result downloadSkillResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, apperrors.NewAPI(fmt.Sprintf("failed to parse response: %v", err))
	}

	return &result, nil
}

func downloadSkillToTmpDir(ctx context.Context, apiURL, accessToken string) (string, error) {
	req, err := skillNewRequest(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", apperrors.NewInternal(fmt.Sprintf("failed to create request: %v", err))
	}
	req.Header.Set("x-user-access-token", accessToken)

	client := &http.Client{Timeout: skillDownloadTimeout}
	resp, err := skillHTTPDo(client, req)
	if err != nil {
		return "", apperrors.NewAPI(fmt.Sprintf("failed to download skill package: %v", err), apperrors.WithRetryable(true))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", parseLegacySkillAPIError(resp)
	}

	tmpDir, err := skillMkdirTemp("", "dws-skill-*")
	if err != nil {
		return "", apperrors.NewInternal(fmt.Sprintf("failed to create temp dir: %v", err))
	}

	filename := filenameFromDisposition(resp.Header.Get("Content-Disposition"))
	destPath := filepath.Join(tmpDir, filename)
	file, err := skillCreate(destPath)
	if err != nil {
		_ = skillRemoveAll(tmpDir)
		return "", apperrors.NewInternal(fmt.Sprintf("failed to create temp file: %v", err))
	}
	defer file.Close()

	if _, err := skillCopy(file, resp.Body); err != nil {
		_ = skillRemoveAll(tmpDir)
		return "", apperrors.NewAPI(fmt.Sprintf("failed to save downloaded file: %v", err))
	}
	return tmpDir, nil
}

func filenameFromDisposition(cd string) string {
	if cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if name := strings.TrimSpace(params["filename"]); name != "" {
				return name
			}
		}
	}
	return "skill.zip"
}

func parseLegacySkillAPIError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return skillAuthError()
	case http.StatusBadRequest:
		return apperrors.NewValidation("request parameters are invalid")
	case http.StatusNotFound:
		return apperrors.NewValidation("skill does not exist or corresponding file was not found")
	default:
		return apperrors.NewAPI(fmt.Sprintf("skill API returned HTTP %d", resp.StatusCode),
			apperrors.WithRetryable(resp.StatusCode >= 500))
	}
}

// downloadSkillFile downloads the skill zip file to a temporary location.
func downloadSkillFile(ctx context.Context, downloadURL, fileName string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", apperrors.NewInternal(fmt.Sprintf("failed to create download request: %v", err))
	}

	client := &http.Client{Timeout: skillDownloadTimeout}
	resp, err := skillHTTPDo(client, req)
	if err != nil {
		return "", apperrors.NewAPI(fmt.Sprintf("failed to download skill: %v", err),
			apperrors.WithRetryable(true))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", apperrors.NewAPI(fmt.Sprintf("download returned HTTP %d", resp.StatusCode),
			apperrors.WithRetryable(resp.StatusCode >= 500))
	}

	// Create temp file
	if fileName == "" {
		fileName = "skill.zip"
	}
	tempFile, err := skillCreateTemp("", "dws-skill-*.zip")
	if err != nil {
		return "", apperrors.NewInternal(fmt.Sprintf("failed to create temp file: %v", err))
	}
	tempPath := tempFile.Name()

	// Copy response body to temp file
	_, err = skillCopy(tempFile, resp.Body)
	closeErr := tempFile.Close()
	if err != nil {
		_ = skillRemove(tempPath)
		return "", apperrors.NewAPI(fmt.Sprintf("failed to save downloaded file: %v", err))
	}
	if closeErr != nil {
		_ = skillRemove(tempPath)
		return "", apperrors.NewInternal(fmt.Sprintf("failed to close temp file: %v", closeErr))
	}

	return tempPath, nil
}

// extractSkillZip extracts a zip file to the destination directory.
func extractSkillZip(zipPath, destDir string) error {
	// Ensure destination directory exists
	if err := skillMkdirAll(destDir, 0755); err != nil {
		return apperrors.NewInternal(fmt.Sprintf("failed to create destination directory: %v", err))
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return apperrors.NewInternal(fmt.Sprintf("failed to open zip file: %v", err))
	}
	defer reader.Close()

	for _, file := range reader.File {
		if err := extractZipFile(file, destDir); err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile extracts a single file from the zip archive.
func extractZipFile(file *zip.File, destDir string) error {
	// Sanitize file path to prevent zip slip attacks
	filePath := filepath.Join(destDir, file.Name)
	if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(destDir)+string(os.PathSeparator)) {
		return apperrors.NewValidation(fmt.Sprintf("invalid file path in zip: %s", file.Name))
	}

	if file.FileInfo().IsDir() {
		// Use 0755 to ensure we have write permission for creating files inside
		return skillMkdirAll(filePath, 0755)
	}

	// Ensure parent directory exists with write permission
	if err := skillMkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return apperrors.NewInternal(fmt.Sprintf("failed to create directory: %v", err))
	}

	// Extract file
	srcFile, err := skillOpenZipFile(file)
	if err != nil {
		return apperrors.NewInternal(fmt.Sprintf("failed to open file in zip: %v", err))
	}
	defer srcFile.Close()

	// Use file mode from zip but ensure at least 0644 for files
	fileMode := file.Mode()
	if fileMode&0600 == 0 {
		fileMode = 0644
	}
	destFile, err := skillOpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileMode)
	if err != nil {
		return apperrors.NewInternal(fmt.Sprintf("failed to create file: %v", err))
	}
	defer destFile.Close()

	if _, err := skillCopy(destFile, srcFile); err != nil {
		return apperrors.NewInternal(fmt.Sprintf("failed to extract file: %v", err))
	}

	return nil
}

// cleanupTempFile removes a temporary file, ignoring errors.
func cleanupTempFile(path string) {
	if path != "" {
		_ = skillRemove(path)
	}
}
