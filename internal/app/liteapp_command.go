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
	"strconv"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/cobra"
)

const (
	// liteappDefaultMCPID 是轻应用六个工具所在的市场服务「钉钉开放平台应用管理」。
	// 解析规则：显式 --mcp-id 最高优先级；未指定时固定回落到该服务。
	liteappDefaultMCPID = "10357"
	liteappCreateTool   = "create_lite_app"
	liteappUpdateTool     = "update_lite_app"
	liteappDeleteTool     = "delete_lite_app"
	liteappListTool       = "list_lite_apps"
	liteappDetailTool     = "get_lite_app_detail"
	liteappCredentialTool = "get_lite_app_credentials"
)

func newLiteappGroup(caller edition.ToolCaller, factory mcpPublishedTransportFactory) *cobra.Command {
	group := &cobra.Command{
		Use:   "liteapp",
		Short: "轻应用开发（快捷应用全生命周期）",
		Long: "轻应用全生命周期指令：创建、更新、删除、列表、详情、凭证查询。\n\n" +
			"调用身份由系统上下文注入（corpId/userId），仅能操作当前调用人创建的轻应用；" +
			"AppSecret 明文随创建响应与凭证查询返回，注意防泄露，不得写入日志或仓库。",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
	}
	corecmd.ApplyGroupPolicy(group, corecmd.GroupPolicy{
		Mode:        corecmd.GroupNavigationOnly,
		Positionals: corecmd.PositionalsReject,
		Recovery:    corecmd.RecoverySibling,
	})
	if factory == nil {
		factory = newAuthenticatedMCPPublishedTransportFactory(nil, nil)
	}
	// dev 子树叶子身份约定：ProductID 固定 "dev"，Name 用远端工具名，
	// CanonicalPath 为 "dev." + 工具名，CLIPath 为完整命令路径（与 dev app/dev mcp 一致）。

	group.AddCommand(
		newLiteappCreateCommand(caller, factory),
		newLiteappUpdateCommand(caller, factory),
		newLiteappDeleteCommand(caller, factory),
		newLiteappListCommand(caller, factory),
		newLiteappDetailCommand(caller, factory),
		newLiteappCredentialCommand(caller, factory),
	)
	return group
}

// effectiveLiteappMCPID：显式 --mcp-id 最高优先级；未指定时默认使用开放平台应用管理（10357）。
func effectiveLiteappMCPID(explicit string) string {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return trimmed
	}
	return liteappDefaultMCPID
}

// runLiteappTool 是六个子命令共享的执行路径：dry-run 预演 / 确认门禁 / endpoint 解析 / 已发布工具调用。
func runLiteappTool(
	cmd *cobra.Command,
	caller edition.ToolCaller,
	factory mcpPublishedTransportFactory,
	mcpID string,
	tool string,
	params map[string]any,
	mutating bool,
) error {
	// 写操作的 --yes 门禁由 DeclareLeafMetadata 的 ConfirmSafety 统一执行（见各叶 Validate）
	resolved := effectiveLiteappMCPID(mcpID)
	dryRun := corecmd.BoolFlag(cmd, "dry-run")
	if dryRun {
		payload := map[string]any{
			"kind":      "helper_invocation",
			"dry_run":   true,
			"executed":  false,
			"product":   "liteapp",
			"mcpId":     resolved,
			"tool":      tool,
			"arguments": params,
		}
		if err := output.ValidateResult(output.Success(payload, output.WithDryRun())); err != nil {
			return err
		}
		return output.WriteCommandPayload(cmd, payload, output.FormatJSON)
	}
	ctx, cancel, err := mcpPublishedOperationContext(cmd)
	if err != nil {
		return err
	}
	defer cancel()
	endpoint, err := resolvePublishedMCPEndpoint(ctx, mcpPublishedCallerWithDeadline(caller, ctx), resolved)
	if err != nil {
		return err
	}
	client, err := factory(ctx)
	if err != nil {
		return err
	}
	invocation, err := client.InvokeValidated(ctx, endpoint, tool, params)
	if err != nil {
		return err
	}
	if invocation.Result.IsError {
		return apperrors.NewAPI(
			"已发布 MCP 工具返回错误: "+extractMCPErrorMessage(invocation.Result),
			apperrors.WithReason("published_mcp_tool_error"),
		)
	}
	payload := mcpPublishedInvokeResult{
		MCPID:                 resolved,
		Tool:                  tool,
		InputSchemaValidation: invocation.InputSchemaValidation,
		InputSchemaDigest:     invocation.InputSchemaDigest,
		Result:                invocation.Result,
	}
	if err := output.ValidateResult(output.Success(payload)); err != nil {
		return err
	}
	return output.WriteCommandPayload(cmd, payload, output.FormatJSON)
}

func liteappMCPIDFlag(cmd *cobra.Command) {
	cmd.Flags().String("mcp-id", "", "轻应用 MCP 市场服务 ID；默认 10357（钉钉开放平台应用管理），一般无需指定")
}

func newLiteappCreateCommand(caller edition.ToolCaller, factory mcpPublishedTransportFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "创建轻应用（创建即发布挂工作台，并注册统一应用与 OAuth 凭证）",
		Long: "创建钉钉企业内部轻应用：创建即发布并挂载工作台\"我的\"分组，同步注册统一应用（企业内部应用），" +
			"并生成 OAuth 凭证。AppSecret 明文随创建响应与 credential 子命令返回，注意防泄露。\n\n" +
			"传 --request-id（幂等键）时，同键同参数重试返回首次结果，参数变化将被拒绝。" +
			"调用身份由系统上下文注入，无需传 corpId/userId。",
		Example:           "  dws dev liteapp create --name 周报助手 --homepage-url https://example.com --request-id 6f1c2b3a-uuid --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			name, _ := cmd.Flags().GetString("name")
			homepageURL, _ := cmd.Flags().GetString("homepage-url")
			pcURL, _ := cmd.Flags().GetString("pc-url")
			desc, _ := cmd.Flags().GetString("desc")
			iconMediaID, _ := cmd.Flags().GetString("icon-media-id")
			requestID, _ := cmd.Flags().GetString("request-id")
			if strings.TrimSpace(name) == "" {
				return apperrors.NewValidation("--name 不能为空")
			}
			if strings.TrimSpace(homepageURL) == "" {
				return apperrors.NewValidation("--homepage-url 不能为空")
			}
			params := map[string]any{"appName": name, "homepageUrl": homepageURL}
			if strings.TrimSpace(pcURL) != "" {
				params["pcUrl"] = pcURL
			}
			if strings.TrimSpace(desc) != "" {
				params["desc"] = desc
			}
			if strings.TrimSpace(iconMediaID) != "" {
				params["iconUrl"] = iconMediaID
			}
			if strings.TrimSpace(requestID) != "" {
				params["requestId"] = requestID
			}
			mcpID, _ := cmd.Flags().GetString("mcp-id")
			return runLiteappTool(cmd, caller, factory, mcpID, liteappCreateTool, params, true)
		},
	}
	cmd.Flags().String("name", "", "应用名称，必填，trim 后非空")
	cmd.Flags().String("homepage-url", "", "移动端首页地址，必填，http(s) URL")
	cmd.Flags().String("pc-url", "", "PC 端首页地址，可选，缺省取移动端首页地址")
	cmd.Flags().String("desc", "", "应用描述，可选")
	cmd.Flags().String("icon-media-id", "", "图标文件 media_id（logoImg 格式）；不接受 http 地址；不传自动生成默认图标")
	cmd.Flags().String("request-id", "", "幂等键，建议传 UUID；同键同参数重试返回首次结果，参数变化拒绝")
	liteappMCPIDFlag(cmd)
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "medium",
			Confirmation: "user_required", Idempotency: "idempotent",
		},
		Validate: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			homepageURL, _ := cmd.Flags().GetString("homepage-url")
			if strings.TrimSpace(name) == "" {
				return apperrors.NewValidation("--name 不能为空")
			}
			if strings.TrimSpace(homepageURL) == "" {
				return apperrors.NewValidation("--homepage-url 不能为空")
			}
			return nil
		},
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID: "dev", Name: "create_lite_app", CanonicalPath: "dev.create_lite_app",
				CLIPath: "dev liteapp create", PrimaryCLIPath: "dev liteapp create",
			},
			Description: "经确认后创建钉钉企业内部轻应用，创建即发布挂工作台并注册统一应用与 OAuth 凭证",
			DryRun:      &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewInvocation, RemoteReads: false},
			Interface: &contract.InterfaceSpec{
				Mode: contract.InterfaceModeComposite, Availability: contract.InterfaceAvailable,
				Reason: "Delegates to the published lite-app tools on the OpenPlatform app-management MCP service (default mcpId 10357); the remote tool's effect cannot be statically bound.",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "经用户确认后创建轻应用，返回 appId/appKey/secret/unifiedAppId",
				UseWhen:      []string{"需要在当前组织创建一个新的钉钉轻应用并直接发布到工作台"},
				AvoidWhen: []string{
					"需要管理已有应用时使用 dev liteapp update/delete 等子命令",
					"用户未确认创建目标名称与首页地址时不要真实执行",
				},
				Examples: []string{"dws dev liteapp create --name 周报助手 --homepage-url https://example.com --request-id 6f1c2b3a-uuid --dry-run --format json"},
			},
		},
	})
	return cmd
}

func newLiteappUpdateCommand(caller edition.ToolCaller, factory mcpPublishedTransportFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <appId>",
		Short: "更新轻应用（仅创建者本人；不传=不修改，空串拒绝）",
		Long: "更新指定轻应用的基础信息与 OAuth 回调地址。字段语义：不传=null=不修改；" +
			"传空串会被拒绝（防误清空）。redirectUris 传入即整体覆盖登记。仅创建者本人可更新。",
		Example: "  dws dev liteapp update 5005426001 --desc 新描述\n" +
			"  dws dev liteapp update 5005426001 --redirect-uris https://a.example.com/cb,https://b.example.com/cb --yes",
		Args:              cobra.ExactArgs(1),
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			appID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil || appID <= 0 {
				return apperrors.NewValidation("appId 必须是正整数")
			}
			params := map[string]any{"appId": appID}
			for _, pair := range []struct {
				flag, key string
			}{{"app-name", "appName"}, {"homepage-url", "homepageUrl"}, {"pc-url", "pcUrl"}, {"desc", "desc"}, {"icon-media-id", "iconUrl"}} {
				value, _ := cmd.Flags().GetString(pair.flag)
				if strings.TrimSpace(value) != "" {
					params[pair.key] = value
				}
			}
			if redirectURIs, _ := cmd.Flags().GetString("redirect-uris"); strings.TrimSpace(redirectURIs) != "" {
				uris := []string{}
				for _, uri := range strings.Split(redirectURIs, ",") {
					if trimmed := strings.TrimSpace(uri); trimmed != "" {
						uris = append(uris, trimmed)
					}
				}
				if len(uris) > 0 {
					params["redirectUris"] = uris
				}
			}
			if len(params) == 1 {
				return apperrors.NewValidation("至少提供一个要修改的字段；不修改请勿调用")
			}
			mcpID, _ := cmd.Flags().GetString("mcp-id")
			return runLiteappTool(cmd, caller, factory, mcpID, liteappUpdateTool, params, true)
		},
	}
	cmd.Flags().String("app-name", "", "应用名称；不传=不修改，空串拒绝")
	cmd.Flags().String("homepage-url", "", "移动端首页地址；不传=不修改，空串拒绝")
	cmd.Flags().String("pc-url", "", "PC 端首页地址；不传=不修改，空串拒绝")
	cmd.Flags().String("desc", "", "应用描述；不传=不修改，空串拒绝")
	cmd.Flags().String("icon-media-id", "", "图标文件 media_id；不接受 http 地址；不传=不修改")
	cmd.Flags().String("redirect-uris", "", "OAuth 回调地址列表，逗号分隔；不传=不修改，传入=整体覆盖")
	liteappMCPIDFlag(cmd)
	cli.AnnotateRuntimePositionals(cmd, contract.RuntimeSchemaPositional{
		Name: "app_id", Type: "integer", Description: "轻应用 ID（microAppId）", Required: true, Index: 0,
	})
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "medium",
			Confirmation: "user_required", Idempotency: "idempotent",
		},
		Validate: func(cmd *cobra.Command, args []string) error {
			appID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil || appID <= 0 {
				return apperrors.NewValidation("appId 必须是正整数")
			}
			fields := 0
			for _, flag := range []string{"app-name", "homepage-url", "pc-url", "desc", "icon-media-id", "redirect-uris"} {
				if v, _ := cmd.Flags().GetString(flag); strings.TrimSpace(v) != "" {
					fields++
				}
			}
			if fields == 0 {
				return apperrors.NewValidation("至少提供一个要修改的字段；不修改请勿调用")
			}
			return nil
		},
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID: "dev", Name: "update_lite_app", CanonicalPath: "dev.update_lite_app",
				CLIPath: "dev liteapp update", PrimaryCLIPath: "dev liteapp update",
			},
			Description: "经确认后更新创建者本人的轻应用基础信息与 OAuth 回调地址",
			Interface: &contract.InterfaceSpec{
				Mode: contract.InterfaceModeComposite, Availability: contract.InterfaceAvailable,
				Reason: "Delegates to the published lite-app tools on the OpenPlatform app-management MCP service (default mcpId 10357); the remote tool's effect cannot be statically bound.",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "经用户确认后更新创建者本人的轻应用",
				UseWhen:      []string{"需要修改创建者本人的轻应用名称、首页、描述、回调地址等"},
				AvoidWhen:    []string{"目标应用不是当前用户创建时不要调用（服务端仅创建者可更新）"},
				Examples:     []string{"dws dev liteapp update 5005426001 --desc 新描述"},
			},
		},
	})
	return cmd
}

func newLiteappDeleteCommand(caller edition.ToolCaller, factory mcpPublishedTransportFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <appId>",
		Short: "删除轻应用（24 小时软删，仅创建者或组织管理员）",
		Long: "删除指定轻应用。24 小时软删，软删期内仍占用配额、不可恢复，返回软删截止时间 timeToDel。" +
			"仅创建者或组织管理员可删除。",
		Example:           "  dws dev liteapp delete 5005426001",
		Args:              cobra.ExactArgs(1),
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			appID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil || appID <= 0 {
				return apperrors.NewValidation("appId 必须是正整数")
			}
			mcpID, _ := cmd.Flags().GetString("mcp-id")
			return runLiteappTool(cmd, caller, factory, mcpID, liteappDeleteTool,
				map[string]any{"appId": appID}, true)
		},
	}
	liteappMCPIDFlag(cmd)
	cli.AnnotateRuntimePositionals(cmd, contract.RuntimeSchemaPositional{
		Name: "app_id", Type: "integer", Description: "轻应用 ID（microAppId）", Required: true, Index: 0,
	})
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "write", Risk: "high",
			Confirmation: "user_required", Idempotency: "idempotent",
		},
		Validate: func(cmd *cobra.Command, args []string) error {
			appID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil || appID <= 0 {
				return apperrors.NewValidation("appId 必须是正整数")
			}
			return nil
		},
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID: "dev", Name: "delete_lite_app", CanonicalPath: "dev.delete_lite_app",
				CLIPath: "dev liteapp delete", PrimaryCLIPath: "dev liteapp delete",
			},
			Description: "经确认后软删创建者本人或组织管理员的轻应用，返回软删截止时间",
			Interface: &contract.InterfaceSpec{
				Mode: contract.InterfaceModeComposite, Availability: contract.InterfaceAvailable,
				Reason: "Delegates to the published lite-app tools on the OpenPlatform app-management MCP service (default mcpId 10357); the remote tool's effect cannot be statically bound.",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "经用户确认后软删轻应用（24 小时后物理删除）",
				UseWhen:      []string{"需要删除不再使用的轻应用以释放配额"},
				AvoidWhen:    []string{"应用正在被其他系统使用时不要删除；软删期内不可恢复"},
				Examples:     []string{"dws dev liteapp delete 5005426001"},
			},
		},
	})
	return cmd
}

func newLiteappListCommand(caller edition.ToolCaller, factory mcpPublishedTransportFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "查询当前用户在本组织创建的轻应用列表",
		Long: "查询当前用户在本组织创建的轻应用列表，按创建时间倒序，不含任何 secret 字段。" +
			"软删期应用仍计入列表与配额，条目带 timeToDel。",
		Example:           "  dws dev liteapp list --size 20 --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			params := map[string]any{}
			if size, _ := cmd.Flags().GetInt("size"); size > 0 {
				params["size"] = size
			}
			if offset, _ := cmd.Flags().GetInt("offset"); offset > 0 {
				params["offset"] = offset
			}
			mcpID, _ := cmd.Flags().GetString("mcp-id")
			return runLiteappTool(cmd, caller, factory, mcpID, liteappListTool, params, false)
		},
	}
	cmd.Flags().Int("size", 0, "每页数量，默认 20，上限 50")
	cmd.Flags().Int("offset", 0, "偏移量，从 0 开始")
	liteappMCPIDFlag(cmd)
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID: "dev", Name: "list_lite_apps", CanonicalPath: "dev.list_lite_apps",
				CLIPath: "dev liteapp list", PrimaryCLIPath: "dev liteapp list",
			},
			Description: "查询当前用户在本组织创建的轻应用列表（不含 secret 字段）",
			Interface: &contract.InterfaceSpec{
				Mode: contract.InterfaceModeComposite, Availability: contract.InterfaceAvailable,
				Reason: "Delegates to the published lite-app tools on the OpenPlatform app-management MCP service (default mcpId 10357).",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "查询当前用户在本组织创建的轻应用列表",
				UseWhen:      []string{"需要查看当前用户创建了哪些轻应用、占用多少配额"},
				AvoidWhen:    []string{"查询单个应用详情时用 dev liteapp detail"},
				Examples:     []string{"dws dev liteapp list --size 20 --format json"},
			},
		},
	})
	return cmd
}

func newLiteappDetailCommand(caller edition.ToolCaller, factory mcpPublishedTransportFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detail <appId>",
		Short: "查询轻应用详情（无 secret 明文）",
		Long: "按 appId 查询轻应用详情。逐项鉴权，无权限或不存在统一返回 E_NOT_FOUND（防探测）。" +
			"返回基础信息、appKey、secret 掩码、redirectUris、unifiedAppId 等；无 secret 明文。",
		Example:           "  dws dev liteapp detail 5005426001 --format json",
		Args:              cobra.ExactArgs(1),
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			appID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil || appID <= 0 {
				return apperrors.NewValidation("appId 必须是正整数")
			}
			mcpID, _ := cmd.Flags().GetString("mcp-id")
			return runLiteappTool(cmd, caller, factory, mcpID, liteappDetailTool,
				map[string]any{"appId": appID}, false)
		},
	}
	liteappMCPIDFlag(cmd)
	cli.AnnotateRuntimePositionals(cmd, contract.RuntimeSchemaPositional{
		Name: "app_id", Type: "integer", Description: "轻应用 ID（microAppId）", Required: true, Index: 0,
	})
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID: "dev", Name: "get_lite_app_detail", CanonicalPath: "dev.get_lite_app_detail",
				CLIPath: "dev liteapp detail", PrimaryCLIPath: "dev liteapp detail",
			},
			Description: "按 appId 查询轻应用详情（无 secret 明文）",
			Interface: &contract.InterfaceSpec{
				Mode: contract.InterfaceModeComposite, Availability: contract.InterfaceAvailable,
				Reason: "Delegates to the published lite-app tools on the OpenPlatform app-management MCP service (default mcpId 10357).",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "查询单个轻应用的详情与凭证掩码",
				UseWhen:      []string{"需要查看某个轻应用的基础信息、回调地址与凭证掩码"},
				AvoidWhen:    []string{"需要 secret 明文时用 dev liteapp credential"},
				Examples:     []string{"dws dev liteapp detail 5005426001 --format json"},
			},
		},
	})
	return cmd
}

func newLiteappCredentialCommand(caller edition.ToolCaller, factory mcpPublishedTransportFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credential <appId>",
		Short: "查询轻应用凭证（appKey + secret 明文，注意防泄露）",
		Long: "查询轻应用 OAuth 凭证：appKey 明文 + secret 明文与掩码，可持续获取。" +
			"secret 明文注意防泄露，不得写入日志、文档、邮件或代码仓库；" +
			"重置能力不在本命令范围（需在开发者后台人工完成）。",
		Example:           "  dws dev liteapp credential 5005426001 --format json",
		Args:              cobra.ExactArgs(1),
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			appID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
			if err != nil || appID <= 0 {
				return apperrors.NewValidation("appId 必须是正整数")
			}
			mcpID, _ := cmd.Flags().GetString("mcp-id")
			return runLiteappTool(cmd, caller, factory, mcpID, liteappCredentialTool,
				map[string]any{"appId": appID}, false)
		},
	}
	liteappMCPIDFlag(cmd)
	cli.AnnotateRuntimePositionals(cmd, contract.RuntimeSchemaPositional{
		Name: "app_id", Type: "integer", Description: "轻应用 ID（microAppId）", Required: true, Index: 0,
	})
	helpers.DeclareLeafMetadata(cmd, helpers.LeafSpec{
		Safety: contract.SafetySpec{
			Effect: "read", Risk: "high",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Contract: helpers.LeafContract{
			Identity: contract.ToolIdentitySpec{
				ProductID: "dev", Name: "get_lite_app_credentials", CanonicalPath: "dev.get_lite_app_credentials",
				CLIPath: "dev liteapp credential", PrimaryCLIPath: "dev liteapp credential",
			},
			Description: "查询轻应用 OAuth 凭证（appKey 明文 + secret 明文与掩码，可持续获取）",
			Interface: &contract.InterfaceSpec{
				Mode: contract.InterfaceModeComposite, Availability: contract.InterfaceAvailable,
				Reason: "Delegates to the published lite-app tools on the OpenPlatform app-management MCP service (default mcpId 10357).",
			},
			Selection: contract.SelectionSpec{
				AgentSummary: "查询轻应用的 OAuth 凭证（含 secret 明文，注意防泄露）",
				UseWhen:      []string{"服务端接入需要 appKey/secret 换取 accessToken 时"},
				AvoidWhen: []string{
					"不要把返回的 secret 写入日志、文档、邮件、群聊或代码仓库",
					"仅需要应用基础信息时用 dev liteapp detail",
				},
				Examples: []string{"dws dev liteapp credential 5005426001 --format json"},
			},
		},
	})
	return cmd
}
