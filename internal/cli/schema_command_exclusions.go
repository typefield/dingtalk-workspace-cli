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

// reviewedRuntimeSchemaExclusionGroups is the reviewed authority for
// public Cobra leaves intentionally kept outside the stable Agent
// Schema surface. Exact path match only; no prefix or wildcard.
// Each group must be reviewed with a non-empty reason.
var reviewedRuntimeSchemaExclusionGroups = []runtimeSchemaExclusionGroup{
	{
		ID:       "cli-management",
		Reason:   "Unreviewed local CLI lifecycle, credential mutation, configuration, and plugin-management commands remain user-operated controls rather than stable Agent tools.",
		Reviewed: true,
		Commands: []string{
			"api",
			"auth export",
			"auth import",
			"auth login",
			"auth logout",
			"auth migrate-keychain",
			"auth reset",
			"completion",
			"config list",
			"doctor",
			"plugin build",
			"plugin config get",
			"plugin config list",
			"plugin config set",
			"plugin config unset",
			"plugin create",
			"plugin dev",
			"plugin disable",
			"plugin enable",
			"plugin info",
			"plugin install",
			"plugin list",
			"plugin remove",
			"plugin validate",
			"profile switch",
			"profile use",
			"schema",
			"upgrade",
		},
	},
	{
		ID:       "agoal-out-of-surface",
		Reason:   "The Agoal product remains executable for compatibility but is outside the currently reviewed open-source Agent command surface.",
		Reviewed: true,
		Commands: []string{
			"agoal contract detail",
			"agoal contract list",
			"agoal contract update",
			"agoal obj-template create-or-update",
			"agoal report submit-detail",
			"agoal scorecard detail",
			"agoal scorecard entity-detail",
			"agoal scorecard update",
			"agoal strategy detail",
			"agoal strategy list",
			"agoal strategy update",
			"agoal user objectives",
		},
	},
}
