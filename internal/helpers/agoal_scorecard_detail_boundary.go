// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/spf13/cobra"
)

const agoalScorecardDetailTool = "get_score_card_detail"

// runAgoalScorecardDetail preserves the legacy non-null payload, but refuses
// to turn a service JSON null into a successful command with stdout "null".
// The command remains outside the reviewed Agent Schema until a non-empty
// entity response is available for a complete ResultSpec.
func runAgoalScorecardDetail(cmd *cobra.Command, toolArgs map[string]any) error {
	if deps.Caller.DryRun() {
		return callMCPTool(agoalScorecardDetailTool, toolArgs)
	}
	payload, err := CallMCPToolPayloadOnServer(cmd.Context(), "agoal", agoalScorecardDetailTool, toolArgs)
	if err != nil {
		return err
	}
	if payload.Data == nil {
		return errors.NewAPI(agoalScorecardDetailTool+" returned JSON null; scorecard state cannot be determined",
			errors.WithOperation(agoalScorecardDetailTool),
			errors.WithSubtype(errors.SubtypeProjectionUnknown),
			errors.WithOrigin("mcp_gateway"),
			errors.WithFailureStage("response_projection"),
			errors.WithRetryable(false),
		)
	}
	return WriteMCPToolPayloadLegacy("agoal", agoalScorecardDetailTool, payload)
}
