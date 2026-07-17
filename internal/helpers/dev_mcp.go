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

package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cobracmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/executor"
	"github.com/spf13/cobra"
)

const (
	devMCPProduct = "mcpdev"

	devMCPServerURLGetTool = "mcp_server_url_get"

	devMCPServiceCreateTool = "mcp_service_create"
	devMCPServiceDeleteTool = "mcp_service_delete"
	devMCPServiceGetTool    = "mcp_service_get"
	devMCPServiceListTool   = "mcp_service_list"
	devMCPServiceUpdateTool = "mcp_service_update"

	devMCPToolCreateTool   = "mcp_tool_create"
	devMCPToolDebugTool    = "mcp_tool_debug"
	devMCPToolDeleteTool   = "mcp_tool_delete"
	devMCPToolGetTool      = "mcp_tool_get"
	devMCPToolListTool     = "mcp_tool_list"
	devMCPToolPublishTool  = "mcp_tool_publish"
	devMCPToolUpdateTool   = "mcp_tool_update"
	devMCPToolVersionsTool = "mcp_tool_versions"

	devMCPAuthConfigGetTool  = "mcp_auth_config_get"
	devMCPAuthConfigSaveTool = "mcp_auth_config_save"

	devMCPCredentialBindTool   = "mcp_credential_bind"
	devMCPCredentialDebugTool  = "mcp_credential_debug"
	devMCPCredentialDeleteTool = "mcp_credential_delete"
	devMCPCredentialGetTool    = "mcp_credential_get"
	devMCPCredentialListTool   = "mcp_credential_list"
	devMCPCredentialSaveTool   = "mcp_credential_save"

	devMCPMemberAddTool    = "mcp_member_add"
	devMCPMemberListTool   = "mcp_member_list"
	devMCPMemberRemoveTool = "mcp_member_remove"
)

// newDevMCPCommand builds the `mcp` subtree under `dws connector`.
//
// The command tree is intentionally helper-only, mirroring dev app: Cobra owns
// the ergonomic CLI paths and safety guards, while `dws schema connector.mcp...`
// resolves parameter descriptions from the published MCP's live tools/list via
// each leaf's mcp-tool annotation.
func newDevMCPCommand(runner executor.Runner) *cobra.Command {
	root := &cobra.Command{
		Use:               "mcp",
		Short:             "MCP 服务与工具管理",
		Long:              "管理 MCP 开发平台服务和工具：服务创建/查询/更新/删除，工具创建/查询/调试/发布/版本管理，以及接入地址获取。",
		Args:              cobra.NoArgs,
		TraverseChildren:  true,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	service := &cobra.Command{
		Use:               "service",
		Short:             "MCP 服务管理",
		Args:              cobra.NoArgs,
		TraverseChildren:  true,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	service.AddCommand(
		newDevMCPServiceListCommand(runner),
		newDevMCPServiceGetCommand(runner),
		newDevMCPServiceCreateCommand(runner),
		newDevMCPServiceUpdateCommand(runner),
		newDevMCPServiceDeleteCommand(runner),
	)

	tool := &cobra.Command{
		Use:               "tool",
		Short:             "MCP 工具管理",
		Args:              cobra.NoArgs,
		TraverseChildren:  true,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	tool.AddCommand(
		newDevMCPToolListCommand(runner),
		newDevMCPToolGetCommand(runner),
		newDevMCPToolCreateCommand(runner),
		newDevMCPToolUpdateCommand(runner),
		newDevMCPToolDebugCommand(runner),
		newDevMCPToolPublishCommand(runner),
		newDevMCPToolDeleteCommand(runner),
		newDevMCPToolVersionsCommand(runner),
	)

	url := &cobra.Command{
		Use:               "url",
		Short:             "MCP 接入地址",
		Args:              cobra.NoArgs,
		TraverseChildren:  true,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	url.AddCommand(newDevMCPURLGetCommand(runner))

	auth := newDevMCPCommandGroup("auth", "MCP 下游鉴权配置")
	auth.AddCommand(
		newDevMCPAuthConfigGetCommand(runner),
		newDevMCPAuthConfigSaveCommand(runner),
	)

	credential := newDevMCPCommandGroup("credential", "MCP 凭证账号管理")
	credential.AddCommand(
		newDevMCPCredentialListCommand(runner),
		newDevMCPCredentialGetCommand(runner),
		newDevMCPCredentialSaveCommand(runner),
		newDevMCPCredentialDebugCommand(runner),
		newDevMCPCredentialBindCommand(runner),
		newDevMCPCredentialDeleteCommand(runner),
	)

	member := newDevMCPCommandGroup("member", "MCP 开发协作者管理")
	member.AddCommand(
		newDevMCPMemberListCommand(runner),
		newDevMCPMemberAddCommand(runner),
		newDevMCPMemberRemoveCommand(runner),
	)

	root.AddCommand(service, tool, url, auth, credential, member)
	return root
}

func newDevMCPCommandGroup(name, short string) *cobra.Command {
	return &cobra.Command{
		Use:               name,
		Short:             short,
		Args:              cobra.NoArgs,
		TraverseChildren:  true,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}

func newDevMCPURLGetCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "get",
		Short:             "获取 MCP 实例接入地址（按调用者个人身份生成，含个人 key 勿外发）",
		Example:           "  dws connector mcp url get --mcp-id 10487 --source MARKET --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			source, err := devMCPRequiredString(cmd, "source")
			if err != nil {
				return err
			}
			source = strings.ToUpper(source)
			if source != "MARKET" && source != "PUBLISHED" {
				return apperrors.NewValidation("--source 只支持 MARKET 或 PUBLISHED")
			}
			params := map[string]any{
				"mcpId":  mcpID,
				"source": source,
			}
			return runDevMCPTool(runner, cmd, devMCPServerURLGetTool, params)
		},
	}
	cmd.Flags().Int("mcp-id", 0, "MCP 服务 ID")
	cmd.Flags().String("source", "MARKET", "服务来源：MARKET 或 PUBLISHED")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPServerURLGetTool)
	return cmd
}

func newDevMCPServiceListCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "查询有开发权限的 MCP 服务列表（含 serverName）",
		Example:           "  dws connector mcp service list --keyword 客户 --page-size 20 --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			params := map[string]any{}
			devMCPPutInt(params, "cursor", devMCPIntFlag(cmd, "cursor"))
			devMCPPutInt(params, "pageSize", devMCPIntFlag(cmd, "page-size"))
			devMCPPutString(params, "keyword", devMCPStringFlag(cmd, "keyword"))
			devMCPPutString(params, "creatorUserId", devMCPStringFlag(cmd, "creator-user-id"))
			return runDevMCPTool(runner, cmd, devMCPServiceListTool, params)
		},
	}
	addDevMCPPagingFlags(cmd)
	cmd.Flags().String("keyword", "", "按服务名关键词过滤")
	cmd.Flags().String("creator-user-id", "", "按创建人 staffId 过滤")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPServiceListTool)
	return cmd
}

func newDevMCPServiceGetCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "get",
		Short:             "查询 MCP 服务详情",
		Example:           "  dws connector mcp service get --mcp-id 10487 --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			return runDevMCPTool(runner, cmd, devMCPServiceGetTool, map[string]any{
				"mcpId": mcpID,
			})
		},
	}
	addDevMCPMCPIDFlag(cmd)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPServiceGetTool)
	return cmd
}

func newDevMCPServiceCreateCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "create",
		Short:             "新建 MCP 服务",
		Example:           "  dws connector mcp service create --name 客户信息查询 --description \"查询客户基础资料\" --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp service create"); err != nil {
				return err
			}
			name, err := devMCPRequiredString(cmd, "name")
			if err != nil {
				return err
			}
			description, err := devMCPRequiredString(cmd, "description")
			if err != nil {
				return err
			}
			serverName, err := devMCPServerNameFlag(cmd)
			if err != nil {
				return err
			}
			params := map[string]any{
				"name":        name,
				"description": description,
			}
			devMCPPutString(params, "icon_url", devMCPStringFlag(cmd, "icon-url"))
			devMCPPutString(params, "introduction", devMCPStringFlag(cmd, "introduction"))
			devMCPPutString(params, "serverName", serverName)
			if err := runDevMCPTool(runner, cmd, devMCPServiceCreateTool, params); err != nil {
				return err
			}
			if serverName == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "警告：未设置 --server-name，发布后顶层动态命令将退化为 mcp-<mcpId>（而非语义化服务名）。建议创建时就传 --server-name <kebab-case>；或事后 dws connector mcp service update --mcp-id <mcpId> --server-name <kebab-case>，再执行 dws connector mcp refresh。")
			}
			return nil
		},
	}
	cmd.Flags().String("name", "", "服务名称，组织内唯一")
	cmd.Flags().String("description", "", "服务用途描述")
	cmd.Flags().String("icon-url", "", "服务图标 URL")
	cmd.Flags().String("introduction", "", "服务详情介绍，支持 markdown")
	cmd.Flags().String("server-name", "", "服务英文标识，kebab-case，作为 DWS 一级命令名")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPServiceCreateTool)
	return cmd
}

func newDevMCPServiceUpdateCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "update",
		Short:             "修改 MCP 服务信息",
		Example:           "  dws connector mcp service update --mcp-id 10487 --description \"新描述\" --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp service update"); err != nil {
				return err
			}
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			serverName, err := devMCPServerNameFlag(cmd)
			if err != nil {
				return err
			}
			params := map[string]any{"mcpId": mcpID}
			updates := 0
			updates += devMCPPutString(params, "name", devMCPStringFlag(cmd, "name"))
			updates += devMCPPutString(params, "description", devMCPStringFlag(cmd, "description"))
			updates += devMCPPutString(params, "icon_url", devMCPStringFlag(cmd, "icon-url"))
			updates += devMCPPutString(params, "introduction", devMCPStringFlag(cmd, "introduction"))
			updates += devMCPPutString(params, "serverName", serverName)
			if updates == 0 {
				return apperrors.NewValidation("至少提供一项待更新字段：--name、--description、--icon-url、--introduction 或 --server-name")
			}
			return runDevMCPTool(runner, cmd, devMCPServiceUpdateTool, params)
		},
	}
	addDevMCPMCPIDFlag(cmd)
	cmd.Flags().String("name", "", "新服务名称")
	cmd.Flags().String("description", "", "新服务描述")
	cmd.Flags().String("icon-url", "", "新图标 URL")
	cmd.Flags().String("introduction", "", "新详情介绍")
	cmd.Flags().String("server-name", "", "新服务英文标识，kebab-case")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPServiceUpdateTool)
	return cmd
}

func newDevMCPServiceDeleteCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "delete",
		Short:             "删除 MCP 服务（不可恢复）",
		Example:           "  dws connector mcp service delete --mcp-id 10487 --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp service delete"); err != nil {
				return err
			}
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			return runDevMCPTool(runner, cmd, devMCPServiceDeleteTool, map[string]any{
				"mcpId": mcpID,
			})
		},
	}
	addDevMCPMCPIDFlag(cmd)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPServiceDeleteTool)
	return cmd
}

func newDevMCPToolListCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "查询 MCP 服务下的工具列表",
		Example:           "  dws connector mcp tool list --mcp-id 10487 --page-size 100 --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			params := map[string]any{"mcpId": mcpID}
			devMCPPutInt(params, "cursor", devMCPIntFlag(cmd, "cursor"))
			devMCPPutInt(params, "pageSize", devMCPIntFlag(cmd, "page-size"))
			devMCPPutString(params, "keyword", devMCPStringFlag(cmd, "keyword"))
			return runDevMCPTool(runner, cmd, devMCPToolListTool, params)
		},
	}
	addDevMCPMCPIDFlag(cmd)
	addDevMCPPagingFlags(cmd)
	cmd.Flags().String("keyword", "", "按工具 name 关键词过滤")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPToolListTool)
	return cmd
}

func newDevMCPToolGetCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "get",
		Short:             "读取 MCP 工具定义",
		Example:           "  dws connector mcp tool get --mcp-id 10487 --tool-id G-ACT-xxx --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := devMCPToolLocatorParams(cmd)
			if err != nil {
				return err
			}
			devMCPPutString(params, "versionId", devMCPStringFlag(cmd, "version-id"))
			return runDevMCPTool(runner, cmd, devMCPToolGetTool, params)
		},
	}
	addDevMCPToolLocatorFlags(cmd)
	cmd.Flags().String("version-id", "", "指定读取的历史版本 ID")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPToolGetTool)
	return cmd
}

func newDevMCPToolCreateCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "create",
		Short:             "新建 MCP 工具草稿",
		Example:           "  dws connector mcp tool create --mcp-id 10487 --name get_weather --http-info '{\"method\":\"GET\",\"url\":\"https://example.com\",\"auth\":{\"type\":\"NO_AUTH\"}}' --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp tool create"); err != nil {
				return err
			}
			params, err := devMCPToolUpsertParams(cmd, false)
			if err != nil {
				return err
			}
			return runDevMCPTool(runner, cmd, devMCPToolCreateTool, params)
		},
	}
	addDevMCPToolUpsertFlags(cmd, false)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPToolCreateTool)
	return cmd
}

func newDevMCPToolUpdateCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "update",
		Short:             "编辑 MCP 工具并保存为草稿",
		Example:           "  dws connector mcp tool update --mcp-id 10487 --tool-id G-ACT-xxx --name get_weather --http-info '{\"method\":\"GET\",\"url\":\"https://example.com\",\"auth\":{\"type\":\"NO_AUTH\"}}' --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp tool update"); err != nil {
				return err
			}
			params, err := devMCPToolUpsertParams(cmd, true)
			if err != nil {
				return err
			}
			return runDevMCPTool(runner, cmd, devMCPToolUpdateTool, params)
		},
	}
	addDevMCPToolUpsertFlags(cmd, true)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPToolUpdateTool)
	return cmd
}

func newDevMCPToolDebugCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "debug",
		Short:             "调试 MCP 工具",
		Example:           "  dws connector mcp tool debug --mcp-id 10487 --tool-id G-ACT-xxx --value '{\"city\":\"杭州\"}' --credential-id 10518 --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp tool debug"); err != nil {
				return err
			}
			params, err := devMCPToolLocatorParams(cmd)
			if err != nil {
				return err
			}
			value, err := devMCPRequiredJSONObjectFlag(cmd, "value")
			if err != nil {
				return err
			}
			params["value"] = value
			devMCPPutString(params, "versionId", devMCPStringFlag(cmd, "version-id"))
			credentialID := devMCPIntFlag(cmd, "credential-id")
			if credentialID == 0 && !commandBoolFlag(cmd, "no-credential") {
				return apperrors.NewValidation("请传 --credential-id <id>（credential list 可查）；确认服务无鉴权时显式加 --no-credential。debug 不使用 bind 绑定的凭证，缺省降级会产生 40014 等误导性报错")
			}
			devMCPPutInt(params, "credentialId", credentialID)
			return runDevMCPTool(runner, cmd, devMCPToolDebugTool, params)
		},
	}
	addDevMCPToolLocatorFlags(cmd)
	cmd.Flags().String("value", "", "调试入参 JSON 对象，结构须符合工具 toolInputs 定义；不要传空 {} 走过场")
	cmd.Flags().String("version-id", "", "指定调试的版本 ID")
	cmd.Flags().Int("credential-id", 0, "凭证账号 ID（credential list 可查）；服务已配置鉴权时必须指定，作为本次调试的实际运行时鉴权（debug 不使用 bind 绑定的凭证，缺省降级会产生 40014 等误导性报错）")
	cmd.Flags().Bool("no-credential", false, "显式声明本次调试不使用凭证（确认服务无鉴权配置时用）")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPToolDebugTool)
	return cmd
}

func newDevMCPToolPublishCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "publish",
		Short:             "发布 MCP 工具草稿",
		Example:           "  dws connector mcp tool publish --mcp-id 10487 --tool-id G-ACT-xxx --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp tool publish"); err != nil {
				return err
			}
			params, err := devMCPToolLocatorParams(cmd)
			if err != nil {
				return err
			}
			if !commandDryRun(cmd) {
				if err := devMCPPublishPreflight(runner, cmd, params); err != nil {
					return err
				}
			}
			if err := runDevMCPTool(runner, cmd, devMCPToolPublishTool, params); err != nil {
				return err
			}
			if !commandDryRun(cmd) {
				fmt.Fprintln(cmd.ErrOrStderr(), "提示：发布后本服务暴露为顶层命令 dws <serverName> <工具名>（等价 dws connector mcp published <serverName> <工具名>；serverName 用 service get 查）。动态命令按发布组织隔离，跨组织调用需 --profile；命令缓存最迟 10 分钟自动生效，立即可用请执行 dws connector mcp refresh。")
			}
			return nil
		},
	}
	addDevMCPToolLocatorFlags(cmd)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPToolPublishTool)
	return cmd
}

func newDevMCPToolDeleteCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "delete",
		Short:             "删除 MCP 工具（不可恢复）",
		Example:           "  dws connector mcp tool delete --mcp-id 10487 --tool-id G-ACT-xxx --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp tool delete"); err != nil {
				return err
			}
			params, err := devMCPToolLocatorParams(cmd)
			if err != nil {
				return err
			}
			return runDevMCPTool(runner, cmd, devMCPToolDeleteTool, params)
		},
	}
	addDevMCPToolLocatorFlags(cmd)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPToolDeleteTool)
	return cmd
}

func newDevMCPToolVersionsCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "versions",
		Short:             "查询 MCP 工具版本历史",
		Example:           "  dws connector mcp tool versions --mcp-id 10487 --tool-id G-ACT-xxx --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			params, err := devMCPToolLocatorParams(cmd)
			if err != nil {
				return err
			}
			devMCPPutInt(params, "cursor", devMCPIntFlag(cmd, "cursor"))
			devMCPPutInt(params, "pageSize", devMCPIntFlag(cmd, "page-size"))
			return runDevMCPTool(runner, cmd, devMCPToolVersionsTool, params)
		},
	}
	addDevMCPToolLocatorFlags(cmd)
	addDevMCPPagingFlags(cmd)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPToolVersionsTool)
	return cmd
}

func newDevMCPAuthConfigGetCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "get",
		Short:             "查询 MCP 下游鉴权配置",
		Example:           "  dws connector mcp auth get --mcp-id 10520 --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			return runDevMCPTool(runner, cmd, devMCPAuthConfigGetTool, map[string]any{"mcpId": mcpID})
		},
	}
	addDevMCPMCPIDFlag(cmd)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPAuthConfigGetTool)
	return cmd
}

func newDevMCPAuthConfigSaveCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "save",
		Short:             "保存 MCP 下游鉴权配置",
		Example:           "  dws connector mcp auth save --mcp-id 10520 --auth-type NO_AUTH --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp auth save"); err != nil {
				return err
			}
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			authType, err := devMCPRequiredString(cmd, "auth-type")
			if err != nil {
				return err
			}
			authType = strings.ToUpper(authType)
			if !devMCPValidAuthType(authType) {
				return apperrors.NewValidation("--auth-type 只支持 NO_AUTH、BASIC、TOKEN 或 SIGNATURE（静态 API key 场景用 SIGNATURE 自定义字段+直引）")
			}
			params := map[string]any{"mcpId": mcpID, "authType": authType}
			for _, mapping := range []struct{ flag, key string }{
				{"basic-auth-config", "basicAuthConfig"},
				{"api-secret-auth-config", "apiSecretAuthConfig"},
				{"token-auth-config", "tokenAuthConfig"},
				{"signature-auth-config", "signatureAuthConfig"},
			} {
				if err := devMCPPutJSONObjectFlag(cmd, params, mapping.flag, mapping.key); err != nil {
					return err
				}
			}
			return runDevMCPTool(runner, cmd, devMCPAuthConfigSaveTool, params)
		},
	}
	addDevMCPMCPIDFlag(cmd)
	cmd.Flags().String("auth-type", "", "鉴权类型：NO_AUTH、BASIC、TOKEN 或 SIGNATURE；静态 API key 场景用 SIGNATURE 自定义字段+直引")
	cmd.Flags().String("basic-auth-config", "", "BASIC 鉴权配置 JSON 对象")
	cmd.Flags().String("api-secret-auth-config", "", "")
	_ = cmd.Flags().MarkHidden("api-secret-auth-config")
	cmd.Flags().String("token-auth-config", "", "TOKEN 换取及注入配置 JSON 对象：{authFields, fetchTokenRequest, 注入位, tokenExpireRules, refreshToken, testRequest}；注入位按下游要求三选一：authHeaders（token 放请求头）/ authQuery（token 放 query 参数）/ authBody")
	cmd.Flags().String("signature-auth-config", "", "SIGNATURE 自定义鉴权配置 JSON 对象（静态 API key 直引 / 自定义签名表达式两类场景）")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPAuthConfigSaveTool)
	return cmd
}

func newDevMCPCredentialListCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "查询 MCP 凭证账号列表",
		Example:           "  dws connector mcp credential list --mcp-id 10520 --page-size 20 --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			params := map[string]any{"mcpId": mcpID}
			devMCPPutInt(params, "cursor", devMCPIntFlag(cmd, "cursor"))
			devMCPPutInt(params, "pageSize", devMCPIntFlag(cmd, "page-size"))
			return runDevMCPTool(runner, cmd, devMCPCredentialListTool, params)
		},
	}
	addDevMCPMCPIDFlag(cmd)
	addDevMCPPagingFlags(cmd)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPCredentialListTool)
	return cmd
}

func newDevMCPCredentialGetCommand(runner executor.Runner) *cobra.Command {
	return newDevMCPCredentialLocatorCommand(runner, "get", "查询 MCP 凭证账号详情", devMCPCredentialGetTool, false)
}

func newDevMCPCredentialDebugCommand(runner executor.Runner) *cobra.Command {
	return newDevMCPCredentialLocatorCommand(runner, "debug", "调试 MCP 凭证账号（会真实调用下游接口，TOKEN 型含现场换 token）", devMCPCredentialDebugTool, true)
}

func newDevMCPCredentialBindCommand(runner executor.Runner) *cobra.Command {
	return newDevMCPCredentialLocatorCommand(runner, "bind", "绑定 MCP 凭证账号", devMCPCredentialBindTool, true)
}

func newDevMCPCredentialDeleteCommand(runner executor.Runner) *cobra.Command {
	return newDevMCPCredentialLocatorCommand(runner, "delete", "删除 MCP 凭证账号（不可恢复）", devMCPCredentialDeleteTool, true)
}

func newDevMCPCredentialLocatorCommand(runner executor.Runner, use, short, tool string, write bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:               use,
		Short:             short,
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if write {
				if err := devAppRequireWriteGuard(cmd, "mcp credential "+use); err != nil {
					return err
				}
			}
			params, err := devMCPCredentialLocatorParams(cmd)
			if err != nil {
				return err
			}
			return runDevMCPTool(runner, cmd, tool, params)
		},
	}
	addDevMCPCredentialLocatorFlags(cmd)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, tool)
	return cmd
}

func newDevMCPCredentialSaveCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "save",
		Short:             "新增或修改 MCP 凭证账号（TOKEN 型会现场调换 token 接口验密钥，密钥无效则保存失败）",
		Example:           "  dws connector mcp credential save --mcp-id 10520 --name 生产账号 --content-file credentials.json --dry-run --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp credential save"); err != nil {
				return err
			}
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			name, err := devMCPRequiredString(cmd, "name")
			if err != nil {
				return err
			}
			content, err := devMCPCredentialContent(cmd)
			if err != nil {
				return err
			}
			params := map[string]any{"mcpId": mcpID, "name": name, "content": content}
			devMCPPutInt(params, "credentialId", devMCPIntFlag(cmd, "credential-id"))
			if commandDryRun(cmd) {
				params["content"] = map[string]any{"redacted": true}
			}
			return runDevMCPTool(runner, cmd, devMCPCredentialSaveTool, params)
		},
	}
	addDevMCPMCPIDFlag(cmd)
	cmd.Flags().Int("credential-id", 0, "已有凭证账号 ID；不传表示新增")
	cmd.Flags().String("name", "", "凭证账号名称")
	cmd.Flags().String("content", "", "密钥键值 JSON 对象；推荐改用 --content-file")
	cmd.Flags().String("content-file", "", "密钥键值 JSON 文件路径，传 - 从 stdin 读取")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPCredentialSaveTool)
	return cmd
}

func newDevMCPMemberListCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "查询 MCP 开发协作者列表",
		Example:           "  dws connector mcp member list --mcp-id 10520 --format json",
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			return runDevMCPTool(runner, cmd, devMCPMemberListTool, map[string]any{"mcpId": mcpID})
		},
	}
	addDevMCPMCPIDFlag(cmd)
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPMemberListTool)
	return cmd
}

func newDevMCPMemberAddCommand(runner executor.Runner) *cobra.Command {
	return newDevMCPMemberMutationCommand(runner, "add", "新增 MCP 开发协作者", devMCPMemberAddTool)
}

func newDevMCPMemberRemoveCommand(runner executor.Runner) *cobra.Command {
	return newDevMCPMemberMutationCommand(runner, "remove", "移除 MCP 开发协作者", devMCPMemberRemoveTool)
}

func newDevMCPMemberMutationCommand(runner executor.Runner, use, short, tool string) *cobra.Command {
	cmd := &cobra.Command{
		Use:               use,
		Short:             short,
		Args:              cobra.NoArgs,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devAppRequireWriteGuard(cmd, "mcp member "+use); err != nil {
				return err
			}
			mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
			if err != nil {
				return err
			}
			userIDs := splitDevAppList(devMCPStringFlag(cmd, "user-ids"))
			if len(userIDs) == 0 {
				return apperrors.NewValidation("--user-ids 至少包含一个成员 staffId")
			}
			return runDevMCPTool(runner, cmd, tool, map[string]any{
				"mcpId":         mcpID,
				"memberUserIds": userIDs,
			})
		},
	}
	addDevMCPMCPIDFlag(cmd)
	cmd.Flags().String("user-ids", "", "成员 staffId 列表，多个用逗号或分号分隔")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, tool)
	return cmd
}

func devMCPValidAuthType(authType string) bool {
	switch authType {
	case "NO_AUTH", "BASIC", "API_SECRET", "TOKEN", "SIGNATURE":
		return true
	default:
		return false
	}
}

func devMCPServerNameFlag(cmd *cobra.Command) (string, error) {
	serverName := strings.ToLower(devMCPStringFlag(cmd, "server-name"))
	if serverName == "" {
		return "", nil
	}
	if len(serverName) > 255 {
		return "", apperrors.NewValidation("--server-name 必须不超过 255 个字符")
	}
	for i, r := range serverName {
		isAlphaNum := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		validDash := r == '-' && i > 0 && i < len(serverName)-1 && serverName[i-1] != '-'
		if !isAlphaNum && !validDash {
			return "", apperrors.NewValidation("--server-name 必须是 kebab-case，仅含字母、数字和单个中划线")
		}
	}
	return serverName, nil
}

func addDevMCPCredentialLocatorFlags(cmd *cobra.Command) {
	addDevMCPMCPIDFlag(cmd)
	cmd.Flags().Int("credential-id", 0, "凭证账号 ID")
}

func devMCPCredentialLocatorParams(cmd *cobra.Command) (map[string]any, error) {
	mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
	if err != nil {
		return nil, err
	}
	credentialID, err := devMCPRequiredInt(cmd, "credential-id")
	if err != nil {
		return nil, err
	}
	return map[string]any{"mcpId": mcpID, "credentialId": credentialID}, nil
}

func devMCPCredentialContent(cmd *cobra.Command) (map[string]any, error) {
	raw := devMCPStringFlag(cmd, "content")
	path := devMCPStringFlag(cmd, "content-file")
	if raw != "" && path != "" {
		return nil, apperrors.NewValidation("--content 与 --content-file 只能使用一个")
	}
	if path != "" {
		var data []byte
		var err error
		if path == "-" {
			data, err = io.ReadAll(cmd.InOrStdin())
		} else {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			return nil, apperrors.NewValidation(fmt.Sprintf("读取 --content-file 失败: %v", err))
		}
		raw = strings.TrimSpace(string(data))
	}
	if raw == "" {
		return nil, apperrors.NewValidation("--content 或 --content-file 为必填")
	}
	var content map[string]any
	if err := json.Unmarshal([]byte(raw), &content); err != nil || content == nil {
		return nil, apperrors.NewValidation("凭证 content 必须是 JSON 对象")
	}
	return content, nil
}

func addDevMCPMCPIDFlag(cmd *cobra.Command) {
	cmd.Flags().Int("mcp-id", 0, "MCP 服务 ID")
}

func addDevMCPToolLocatorFlags(cmd *cobra.Command) {
	addDevMCPMCPIDFlag(cmd)
	addDevMCPToolIDFlag(cmd)
}

func addDevMCPToolIDFlag(cmd *cobra.Command) {
	cmd.Flags().String("tool-id", "", "MCP 工具 ID，G-ACT- 开头")
	cmd.Flags().String("action-id", "", "已更名为 --tool-id")
	_ = cmd.Flags().MarkHidden("action-id")
}

// devMCPRequiredToolID reads --tool-id and rejects the pre-0714 --action-id
// spelling with a rename hint instead of silently ignoring it.
func devMCPRequiredToolID(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Changed("action-id") {
		return "", apperrors.NewValidation("--action-id 已更名为 --tool-id（脚手架 0714 契约变更），请改用 --tool-id")
	}
	return devMCPRequiredString(cmd, "tool-id")
}

func addDevMCPPagingFlags(cmd *cobra.Command) {
	cmd.Flags().Int("cursor", 0, "分页游标，从 1 开始")
	cmd.Flags().Int("page-size", 0, "每页条数，最大 100")
}

func addDevMCPToolUpsertFlags(cmd *cobra.Command, includeToolID bool) {
	addDevMCPMCPIDFlag(cmd)
	if includeToolID {
		addDevMCPToolIDFlag(cmd)
	}
	cmd.Flags().String("name", "", "工具唯一标识，snake_case")
	cmd.Flags().String("title", "", "必填。工具中文标题：中文自然语言、≤30 字、与功能一致")
	cmd.Flags().String("description", "", "必填。工具功能完整描述（LLM 选择工具的核心依据）：动词开头，说明功能/何时用/入参来源/破坏性行为")
	cmd.Flags().String("http-info", "", "必填。HTTP 接口配置 JSON 对象：{method,url,auth:{type,...}}")
	cmd.Flags().String("http", "", "已更名为 --http-info")
	_ = cmd.Flags().MarkHidden("http")
	cmd.Flags().String("api-inputs", "", "必填。接口真实入参 JSON 对象：{headers,body,query,path} 四组，每组=字段数组，字段项={key,title,type,required,description,children}；⚠️平台不支持 enum/default/example 属性——枚举/默认值/示例写进字段 description 文本")
	cmd.Flags().String("api-outputs", "", "接口真实出参 JSON 对象：{headers,body} 两组，字段结构同 --api-inputs；⚠️出参按此 schema 精确裁剪——声明什么字段就返回什么，未声明的被过滤；⚠️与 --output-mappings 必须同批提交（整体透传也必须声明，否则 UI 标「变量已失效」且 publish 被拒），真实结构未知时先建裸草稿→debug 取样→update 补齐")
	cmd.Flags().String("tool-inputs", "", "必填。暴露给 LLM 的入参字段树 JSON 数组（array 型 children 固定一项 key=items）；每字段必须写自包含 description：含义+取值格式+示例，可对 api-inputs 裁剪/改名/加防呆")
	cmd.Flags().String("tool-outputs", "", "可选。暴露给 LLM 的出参字段树 JSON 数组；与 --output-mappings 配套做出参精修（裁字段/改名/补语义）；留空且整体透传时=返回按 --api-outputs 裁剪后的完整响应体")
	cmd.Flags().String("input-mappings", "", "必填。入参映射 JSON 数组，每项 {target,type,source}，type=reference/fixed/express；⚠️target 位置名必须 Pascal（$.Body./$.Query./$.Head./$.Path.），全小写/全大写会静默失效不报错")
	cmd.Flags().String("output-mappings", "", "出参映射 JSON 数组：整体透传 [{\"target\":\"$\",\"type\":\"reference\",\"source\":\"$.node_service_activator.Body\"}] 或字段级精修（配合 --tool-outputs 裁剪/改名，详见 skill mapping-rules）；⚠️必须与 --api-outputs 同批提交并静态互验（source 引用的字段必须已声明，红线#13）；⚠️省略或传 []＝草稿仍建成但运行时多包一层 Body 且 publish 被拦，需先 update 补齐")
}

func annotateDevMCPTool(cmd *cobra.Command, tool string) *cobra.Command {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["mcp-tool"] = tool
	cmd.Annotations["mcp-source"] = devMCPProduct
	return cmd
}

func runDevMCPTool(runner executor.Runner, cmd *cobra.Command, tool string, params map[string]any) error {
	invocation := executor.NewHelperInvocation(
		cobracmd.LegacyCommandPath(cmd),
		devMCPProduct,
		tool,
		params,
	)
	invocation.DryRun = commandDryRun(cmd)
	result, err := runner.Run(cmd.Context(), invocation)
	if err != nil {
		return err
	}
	return writeCommandPayload(cmd, result)
}

func devMCPToolLocatorParams(cmd *cobra.Command) (map[string]any, error) {
	mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
	if err != nil {
		return nil, err
	}
	toolID, err := devMCPRequiredToolID(cmd)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"mcpId":  mcpID,
		"toolId": toolID,
	}, nil
}

func devMCPToolUpsertParams(cmd *cobra.Command, includeToolID bool) (map[string]any, error) {
	mcpID, err := devMCPRequiredInt(cmd, "mcp-id")
	if err != nil {
		return nil, err
	}
	name, err := devMCPRequiredString(cmd, "name")
	if err != nil {
		return nil, err
	}
	params := map[string]any{
		"mcpId": mcpID,
		"name":  name,
	}
	if includeToolID {
		toolID, err := devMCPRequiredToolID(cmd)
		if err != nil {
			return nil, err
		}
		params["toolId"] = toolID
	}

	if cmd.Flags().Changed("http") {
		return nil, apperrors.NewValidation("--http 已更名为 --http-info（脚手架契约变更：请求体键名 http→httpInfo），请改用 --http-info")
	}
	httpInfo, err := devMCPRequiredJSONObjectFlag(cmd, "http-info")
	if err != nil {
		return nil, err
	}
	params["httpInfo"] = httpInfo

	devMCPPutString(params, "title", devMCPStringFlag(cmd, "title"))
	devMCPPutString(params, "description", devMCPStringFlag(cmd, "description"))

	if err := devMCPPutJSONObjectFlag(cmd, params, "api-inputs", "apiInputs"); err != nil {
		return nil, err
	}
	if err := devMCPPutJSONObjectFlag(cmd, params, "api-outputs", "apiOutputs"); err != nil {
		return nil, err
	}
	if err := devMCPPutJSONArrayFlag(cmd, params, "tool-inputs", "toolInputs"); err != nil {
		return nil, err
	}
	if err := devMCPPutJSONArrayFlag(cmd, params, "tool-outputs", "toolOutputs"); err != nil {
		return nil, err
	}
	if err := devMCPPutJSONArrayFlag(cmd, params, "input-mappings", "inputMappings"); err != nil {
		return nil, err
	}
	if err := devMCPPutJSONArrayFlag(cmd, params, "output-mappings", "outputMappings"); err != nil {
		return nil, err
	}
	if err := devMCPValidateToolUpsertParams(params); err != nil {
		return nil, err
	}
	return params, nil
}

func devMCPValidateToolUpsertParams(params map[string]any) error {
	if err := devMCPValidateToolName(stringValue(params["name"])); err != nil {
		return err
	}
	if fields, ok := params["toolInputs"].([]any); ok {
		if err := devMCPValidateFieldsFlag("tool-inputs", fields, true); err != nil {
			return err
		}
	}
	if fields, ok := params["toolOutputs"].([]any); ok {
		if err := devMCPValidateFieldsFlag("tool-outputs", fields, true); err != nil {
			return err
		}
	}
	if inputs, ok := params["apiInputs"].(map[string]any); ok {
		if err := devMCPValidateAPIFieldsFlag("api-inputs", inputs); err != nil {
			return err
		}
	}
	if outputs, ok := params["apiOutputs"].(map[string]any); ok {
		if err := devMCPValidateAPIFieldsFlag("api-outputs", outputs); err != nil {
			return err
		}
	}
	if mappings, ok := params["inputMappings"].([]any); ok {
		if err := devMCPValidateMappingsFlag("input-mappings", mappings); err != nil {
			return err
		}
	}
	if mappings, ok := params["outputMappings"].([]any); ok {
		if err := devMCPValidateMappingsFlag("output-mappings", mappings); err != nil {
			return err
		}
	}
	if err := devMCPLintMappingReferences(params); err != nil {
		return err
	}
	return nil
}

func devMCPValidateToolName(name string) error {
	if name == "" {
		return nil
	}
	if len([]rune(name)) > 32 {
		return apperrors.NewValidation("--name 必须不超过 32 个字符，便于生成稳定 DWS 命令")
	}
	for i, r := range name {
		valid := r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if !valid || i == 0 && !(r >= 'a' && r <= 'z') {
			return apperrors.NewValidation("--name 必须是 snake_case，且以小写英文动词开头，例如 lookup_english_word")
		}
	}
	if !devMCPContainsKnownVerb(name) {
		return apperrors.NewValidation("--name 必须包含清晰动作词，例如 get/list/search/query/lookup/create/update/delete，便于 Agent 选择工具")
	}
	return nil
}

func devMCPContainsKnownVerb(name string) bool {
	for _, token := range strings.Split(name, "_") {
		if devMCPKnownVerb(token) {
			return true
		}
	}
	return false
}

func devMCPKnownVerb(verb string) bool {
	switch verb {
	case "get", "list", "search", "query", "lookup", "find", "fetch", "read", "check", "validate",
		"create", "add", "new", "update", "set", "edit", "delete", "remove",
		"send", "submit", "publish", "debug", "run", "call", "sync", "refresh",
		"convert", "generate", "calculate", "parse", "build", "upload", "download", "export", "import":
		return true
	default:
		return false
	}
}

func devMCPValidateAPIFieldsFlag(flag string, groups map[string]any) error {
	for _, group := range []string{"headers", "head", "body", "query", "path"} {
		raw, ok := groups[group]
		if !ok {
			continue
		}
		fields, ok := raw.([]any)
		if !ok {
			return apperrors.NewValidation(fmt.Sprintf("--%s.%s 必须是字段数组", flag, group))
		}
		if err := devMCPValidateFieldsFlag(flag+"."+group, fields, false); err != nil {
			return err
		}
	}
	return nil
}

func devMCPValidateFieldsFlag(flag string, fields []any, requireDescription bool) error {
	for i, raw := range fields {
		field, ok := raw.(map[string]any)
		if !ok {
			return apperrors.NewValidation(fmt.Sprintf("--%s[%d] 必须是 JSON 对象", flag, i))
		}
		path := fmt.Sprintf("--%s[%d]", flag, i)
		if err := devMCPValidateField(path, field, requireDescription); err != nil {
			return err
		}
	}
	return nil
}

func devMCPValidateField(path string, field map[string]any, requireDescription bool) error {
	key := stringValue(field["key"])
	if key == "" {
		return apperrors.NewValidation(path + ".key 为必填，DWS flag 需要稳定字段名")
	}
	typ := stringValue(field["type"])
	if typ == "" {
		return apperrors.NewValidation(path + ".type 为必填，DWS 需要据此生成参数类型")
	}
	if !devMCPValidFieldType(typ) {
		return apperrors.NewValidation(path + ".type 只支持 string/number/integer/boolean/object/array")
	}
	if requireDescription && stringValue(field["description"]) == "" {
		return apperrors.NewValidation(path + ".description 为必填，DWS 命令和 Agent 选参依赖字段描述")
	}
	children, hasChildren := field["children"].([]any)
	if typ == "array" {
		if !hasChildren || len(children) != 1 {
			return apperrors.NewValidation(path + ".children 必须且只能包含一个 key=items 的数组元素描述")
		}
		child, ok := children[0].(map[string]any)
		if !ok || stringValue(child["key"]) != "items" {
			return apperrors.NewValidation(path + ".children[0].key 必须是 items")
		}
		return devMCPValidateField(path+".children[0]", child, requireDescription)
	}
	if typ == "object" && hasChildren {
		for i, raw := range children {
			child, ok := raw.(map[string]any)
			if !ok {
				return apperrors.NewValidation(fmt.Sprintf("%s.children[%d] 必须是 JSON 对象", path, i))
			}
			if err := devMCPValidateField(fmt.Sprintf("%s.children[%d]", path, i), child, requireDescription); err != nil {
				return err
			}
		}
	}
	return nil
}

func devMCPValidFieldType(typ string) bool {
	switch typ {
	case "string", "number", "integer", "boolean", "object", "array":
		return true
	default:
		return false
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func devMCPValidateMappingsFlag(flag string, mappings []any) error {
	for i, raw := range mappings {
		mapping, ok := raw.(map[string]any)
		if !ok {
			return apperrors.NewValidation(fmt.Sprintf("--%s[%d] 必须是 JSON 对象", flag, i))
		}
		path := fmt.Sprintf("--%s[%d]", flag, i)
		if stringValue(mapping["target"]) == "" {
			return apperrors.NewValidation(path + ".target 为必填")
		}
		typ := stringValue(mapping["type"])
		if typ == "" {
			return apperrors.NewValidation(path + ".type 为必填")
		}
		if typ != "reference" && typ != "fixed" && typ != "express" {
			return apperrors.NewValidation(path + ".type 只支持 reference/fixed/express")
		}
		if _, ok := mapping["source"]; !ok {
			return apperrors.NewValidation(path + ".source 为必填")
		}
	}
	return nil
}

func devMCPRequiredString(cmd *cobra.Command, flag string) (string, error) {
	value := devMCPStringFlag(cmd, flag)
	if value == "" {
		return "", apperrors.NewValidation(fmt.Sprintf("--%s 为必填", flag))
	}
	return value, nil
}

func devMCPRequiredInt(cmd *cobra.Command, flag string) (int, error) {
	value := devMCPIntFlag(cmd, flag)
	if value <= 0 {
		return 0, apperrors.NewValidation(fmt.Sprintf("--%s 为必填", flag))
	}
	return value, nil
}

func devMCPStringFlag(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return strings.TrimSpace(value)
}

func devMCPIntFlag(cmd *cobra.Command, name string) int {
	value, _ := cmd.Flags().GetInt(name)
	return value
}

func devMCPPutString(params map[string]any, key, value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	params[key] = value
	return 1
}

func devMCPPutInt(params map[string]any, key string, value int) {
	if value != 0 {
		params[key] = value
	}
}

func devMCPRequiredJSONObjectFlag(cmd *cobra.Command, flag string) (map[string]any, error) {
	value := devMCPStringFlag(cmd, flag)
	if value == "" {
		return nil, apperrors.NewValidation(fmt.Sprintf("--%s 为必填", flag))
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(value), &out); err != nil || out == nil {
		return nil, apperrors.NewValidation(fmt.Sprintf("--%s 必须是 JSON 对象", flag))
	}
	return out, nil
}

func devMCPPutJSONObjectFlag(cmd *cobra.Command, params map[string]any, flag, key string) error {
	value := devMCPStringFlag(cmd, flag)
	if value == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(value), &out); err != nil || out == nil {
		return apperrors.NewValidation(fmt.Sprintf("--%s 必须是 JSON 对象", flag))
	}
	params[key] = out
	return nil
}

func devMCPPutJSONArrayFlag(cmd *cobra.Command, params map[string]any, flag, key string) error {
	value := devMCPStringFlag(cmd, flag)
	if value == "" {
		return nil
	}
	var out []any
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return apperrors.NewValidation(fmt.Sprintf("--%s 必须是 JSON 数组", flag))
	}
	params[key] = out
	return nil
}
