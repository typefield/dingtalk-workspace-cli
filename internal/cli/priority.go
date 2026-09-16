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

package cli

import (
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cobracmd"
	"github.com/spf13/cobra"
)

// SetOverridePriority delegates to cobracmd.SetOverridePriority.
func SetOverridePriority(cmd *cobra.Command, priority int) {
	cobracmd.SetOverridePriority(cmd, priority)
}

// OverridePriority delegates to cobracmd.OverridePriority.
func OverridePriority(cmd *cobra.Command) int {
	return cobracmd.OverridePriority(cmd)
}
