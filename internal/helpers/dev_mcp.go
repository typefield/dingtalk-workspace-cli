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
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cobracmd"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/executor"
	"github.com/spf13/cobra"
)

const (
	devMCPProduct = "mcpdev"

	devMCPGetServerURLTool = "mcp_get_server_url"

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

	root.AddCommand(service, tool, url)
	return root
}

func newDevMCPURLGetCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "get",
		Short:             "获取 MCP Streamable HTTP 接入地址",
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
			return runDevMCPTool(runner, cmd, devMCPGetServerURLTool, params)
		},
	}
	cmd.Flags().Int("mcp-id", 0, "MCP 服务 ID")
	cmd.Flags().String("source", "MARKET", "服务来源：MARKET 或 PUBLISHED")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPGetServerURLTool)
	return cmd
}

func newDevMCPServiceListCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list",
		Short:             "查询有开发权限的 MCP 服务列表",
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
			params := map[string]any{
				"name":        name,
				"description": description,
			}
			devMCPPutString(params, "icon_url", devMCPStringFlag(cmd, "icon-url"))
			devMCPPutString(params, "introduction", devMCPStringFlag(cmd, "introduction"))
			return runDevMCPTool(runner, cmd, devMCPServiceCreateTool, params)
		},
	}
	cmd.Flags().String("name", "", "服务名称，组织内唯一")
	cmd.Flags().String("description", "", "服务用途描述")
	cmd.Flags().String("icon-url", "", "服务图标 URL")
	cmd.Flags().String("introduction", "", "服务详情介绍，支持 markdown")
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
			params := map[string]any{"mcpId": mcpID}
			updates := 0
			updates += devMCPPutString(params, "name", devMCPStringFlag(cmd, "name"))
			updates += devMCPPutString(params, "description", devMCPStringFlag(cmd, "description"))
			updates += devMCPPutString(params, "icon_url", devMCPStringFlag(cmd, "icon-url"))
			updates += devMCPPutString(params, "introduction", devMCPStringFlag(cmd, "introduction"))
			if updates == 0 {
				return apperrors.NewValidation("至少提供一项待更新字段：--name、--description、--icon-url 或 --introduction")
			}
			return runDevMCPTool(runner, cmd, devMCPServiceUpdateTool, params)
		},
	}
	addDevMCPMCPIDFlag(cmd)
	cmd.Flags().String("name", "", "新服务名称")
	cmd.Flags().String("description", "", "新服务描述")
	cmd.Flags().String("icon-url", "", "新图标 URL")
	cmd.Flags().String("introduction", "", "新详情介绍")
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
		Example:           "  dws connector mcp tool get --mcp-id 10487 --action-id G-ACT-xxx --format json",
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
		Example:           "  dws connector mcp tool create --mcp-id 10487 --name get_weather --http '{\"method\":\"GET\",\"url\":\"https://example.com\",\"auth\":{\"type\":\"NO_AUTH\"}}' --dry-run --format json",
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
		Example:           "  dws connector mcp tool update --mcp-id 10487 --action-id G-ACT-xxx --name get_weather --http '{\"method\":\"GET\",\"url\":\"https://example.com\",\"auth\":{\"type\":\"NO_AUTH\"}}' --dry-run --format json",
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
		Example:           "  dws connector mcp tool debug --mcp-id 10487 --action-id G-ACT-xxx --value '{\"city\":\"杭州\"}' --dry-run --format json",
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
			return runDevMCPTool(runner, cmd, devMCPToolDebugTool, params)
		},
	}
	addDevMCPToolLocatorFlags(cmd)
	cmd.Flags().String("value", "", "调试入参 JSON 对象")
	cmd.Flags().String("version-id", "", "指定调试的版本 ID")
	preferLegacyLeaf(cmd)
	annotateDevMCPTool(cmd, devMCPToolDebugTool)
	return cmd
}

func newDevMCPToolPublishCommand(runner executor.Runner) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "publish",
		Short:             "发布 MCP 工具草稿",
		Example:           "  dws connector mcp tool publish --mcp-id 10487 --action-id G-ACT-xxx --dry-run --format json",
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
			return runDevMCPTool(runner, cmd, devMCPToolPublishTool, params)
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
		Example:           "  dws connector mcp tool delete --mcp-id 10487 --action-id G-ACT-xxx --dry-run --format json",
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
		Example:           "  dws connector mcp tool versions --mcp-id 10487 --action-id G-ACT-xxx --format json",
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

func addDevMCPMCPIDFlag(cmd *cobra.Command) {
	cmd.Flags().Int("mcp-id", 0, "MCP 服务 ID")
}

func addDevMCPToolLocatorFlags(cmd *cobra.Command) {
	addDevMCPMCPIDFlag(cmd)
	cmd.Flags().String("action-id", "", "MCP 工具 ID，G-ACT- 开头")
}

func addDevMCPPagingFlags(cmd *cobra.Command) {
	cmd.Flags().Int("cursor", 0, "分页游标，从 1 开始")
	cmd.Flags().Int("page-size", 0, "每页条数，最大 100")
}

func addDevMCPToolUpsertFlags(cmd *cobra.Command, includeActionID bool) {
	addDevMCPMCPIDFlag(cmd)
	if includeActionID {
		cmd.Flags().String("action-id", "", "MCP 工具 ID，G-ACT- 开头")
	}
	cmd.Flags().String("name", "", "工具唯一标识，snake_case")
	cmd.Flags().String("title", "", "工具中文标题")
	cmd.Flags().String("description", "", "工具功能描述")
	cmd.Flags().String("http", "", "HTTP 配置 JSON 对象")
	cmd.Flags().String("api-inputs", "", "接口真实入参 JSON 对象")
	cmd.Flags().String("api-outputs", "", "接口真实出参 JSON 对象")
	cmd.Flags().String("tool-inputs", "", "暴露给 LLM 的入参 JSON 数组")
	cmd.Flags().String("tool-outputs", "", "暴露给 LLM 的出参 JSON 数组")
	cmd.Flags().String("input-mappings", "", "入参映射 JSON 数组")
	cmd.Flags().String("output-mappings", "", "出参映射 JSON 数组")
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
	actionID, err := devMCPRequiredString(cmd, "action-id")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"mcpId":    mcpID,
		"actionId": actionID,
	}, nil
}

func devMCPToolUpsertParams(cmd *cobra.Command, includeActionID bool) (map[string]any, error) {
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
	if includeActionID {
		actionID, err := devMCPRequiredString(cmd, "action-id")
		if err != nil {
			return nil, err
		}
		params["actionId"] = actionID
	}

	http, err := devMCPRequiredJSONObjectFlag(cmd, "http")
	if err != nil {
		return nil, err
	}
	params["http"] = http

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
	return params, nil
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
