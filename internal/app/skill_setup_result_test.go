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
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestSkillSetupAllFailedResultHasStableRecoveryEnvelope(t *testing.T) {
	result := skillSetupResult(skillSetupModeMono, "embedded", []string{"codex"}, nil, skillSetupApplyReport{
		Succeeded: []any{},
		Failed: []output.PartialFailedEntry{{
			ID: "codex/dingtalk-workspace", Error: &output.ErrorInfo{Type: "internal", Message: "copy failed"},
		}},
		Unknown: []output.PartialUnknownEntry{},
	})
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatal(err)
	}
	if env.Outcome != output.OutcomeFailure || env.Error == nil ||
		env.Error.Subtype != "skill_setup_failed" || env.Error.Hint == "" ||
		env.Error.Operation != "skill.setup" || result.ExitCode() == 0 {
		t.Fatalf("skill setup failure = outcome:%q error:%#v rc:%d", env.Outcome, env.Error, result.ExitCode())
	}
}

func TestSkillSetupInvalidPartialHasStableRecoveryEnvelope(t *testing.T) {
	result := skillSetupResult(skillSetupModeMono, "embedded", []string{"codex"}, nil, skillSetupApplyReport{
		Succeeded: []any{map[string]any{"id": "codex/dingtalk-workspace"}},
		Failed:    []output.PartialFailedEntry{},
		Unknown:   []output.PartialUnknownEntry{{ID: "codex/dingtalk-workspace", Reason: "duplicate ID"}},
	})
	env, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatal(err)
	}
	if env.Outcome != output.OutcomeFailure || env.Error == nil ||
		env.Error.Subtype != "skill_setup_result_invalid" || env.Error.Hint == "" || result.ExitCode() == 0 {
		t.Fatalf("invalid partial result = outcome:%q error:%#v rc:%d", env.Outcome, env.Error, result.ExitCode())
	}
}
