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
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
)

func TestContactLabelLeavesPublishReviewedAgentContracts(t *testing.T) {
	root := newContactCommand()
	tests := []struct {
		path          []string
		canonicalPath string
		rpc           string
		parameter     string
		property      string
	}{
		{[]string{"label", "list"}, "contact.get_org_labels", "get_org_labels", "", ""},
		{[]string{"label", "get"}, "contact.search_label_by_name", "search_label_by_name", "names", "labelNames"},
		{[]string{"label", "list-members"}, "contact.get_label_members_by_label_id", "get_label_members_by_labelId", "id", "labelId"},
	}

	for _, test := range tests {
		cmd, _, err := root.Find(test.path)
		if err != nil || cmd == nil || !cmd.Runnable() {
			t.Fatalf("%v is not runnable: cmd=%v err=%v", test.path, cmd, err)
		}
		final, ok := contractfinal.RuntimeContractFinal(cmd)
		if !ok {
			t.Fatalf("%s has no ContractFinal", cmd.CommandPath())
		}
		if final.Identity == nil || final.Identity.CanonicalPath != test.canonicalPath || final.Identity.CLIPath != "contact "+test.path[0]+" "+test.path[1] {
			t.Fatalf("%s identity=%#v", cmd.CommandPath(), final.Identity)
		}
		if final.Interface == nil || final.Interface.Ref == nil || final.Interface.Mode != "mcp" || final.Interface.Availability != "available" || final.Interface.Ref.RPCName != test.rpc {
			t.Fatalf("%s interface=%#v", cmd.CommandPath(), final.Interface)
		}
		if final.Safety == nil || final.Safety.Effect != "read" || final.Safety.Risk != "low" || final.Safety.Confirmation != "not_required" || final.Safety.Idempotency != "idempotent" {
			t.Fatalf("%s safety=%#v", cmd.CommandPath(), final.Safety)
		}
		if final.Selection == nil || len(final.Selection.UseWhen) == 0 || len(final.Selection.AvoidWhen) == 0 || len(final.Selection.Examples) == 0 {
			t.Fatalf("%s selection=%#v", cmd.CommandPath(), final.Selection)
		}
		if test.parameter == "" {
			if len(final.Parameters) != 0 {
				t.Fatalf("%s parameters=%#v, want none", cmd.CommandPath(), final.Parameters)
			}
			continue
		}
		if len(final.Parameters) != 1 || final.Parameters[0].Name != test.parameter || final.Parameters[0].Property != test.property || final.Parameters[0].Required == nil || !*final.Parameters[0].Required {
			t.Fatalf("%s parameters=%#v", cmd.CommandPath(), final.Parameters)
		}
	}
}
