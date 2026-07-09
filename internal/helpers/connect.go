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
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/executor"
	"github.com/spf13/cobra"
)

func init() {
	RegisterPublic(func() Handler {
		return connectorHandler{}
	})
}

// connectorHandler mounts the top-level `dws connector` umbrella. It holds
// connection and access-address oriented capabilities. MCP development tools
// live under `dws connector mcp` so service/tool lifecycle operations sit next to
// the connection surface that ultimately consumes published MCP endpoints.
type connectorHandler struct{}

func (connectorHandler) Name() string {
	return "connector"
}

func (connectorHandler) Command(runner executor.Runner) *cobra.Command {
	root := &cobra.Command{
		Use:               "connector",
		Short:             "连接器与接入能力",
		Long:              "管理钉钉连接器与接入能力：MCP 服务/工具管理、接入地址获取，以及后续连接器类扩展。",
		Args:              cobra.NoArgs,
		TraverseChildren:  true,
		DisableAutoGenTag: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.AddCommand(newDevMCPCommand(runner))
	return root
}
