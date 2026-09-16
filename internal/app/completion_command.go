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
	"github.com/spf13/cobra"
)

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "生成 Shell 自动补全脚本",
		Long: `生成指定 Shell 的自动补全脚本。

Zsh (推荐):
  # 将补全脚本写入 fpath 目录
  dws completion zsh > "${fpath[1]}/_dws"
  # 或者加入 .zshrc
  source <(dws completion zsh)

Bash:
  # Linux
  dws completion bash > /etc/bash_completion.d/dws
  # macOS (需安装 bash-completion)
  dws completion bash > $(brew --prefix)/etc/bash_completion.d/dws

Fish:
  dws completion fish > ~/.config/fish/completions/dws.fish`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(out)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			default:
				return nil
			}
		},
	}
}
