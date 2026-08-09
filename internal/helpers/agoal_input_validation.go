// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"fmt"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/spf13/cobra"
)

func normalizeAgoalScopeType(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case "DEPT", "PERSONAL":
		return normalized, nil
	default:
		return "", apperrors.NewValidation(
			fmt.Sprintf("--scope-type must be DEPT or PERSONAL, got %q", value),
			apperrors.WithSubtype(apperrors.SubtypeInvalidFlagValue),
			apperrors.WithHint("Use --scope-type DEPT with a department ID, or --scope-type PERSONAL with a user ID."),
		)
	}
}

func normalizeAgoalSubmitState(value string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch normalized {
	case "ON_TIME", "LATE", "NOT_SUBMITTED":
		return normalized, nil
	default:
		return "", apperrors.NewValidation(
			fmt.Sprintf("--submit-state must be ON_TIME, LATE, or NOT_SUBMITTED, got %q", value),
			apperrors.WithSubtype(apperrors.SubtypeInvalidFlagValue),
			apperrors.WithHint("Use ON_TIME, LATE, or NOT_SUBMITTED exactly as shown by `dws agoal report submit-detail --help`."),
		)
	}
}

func parseRequiredAgoalPeriodIDs(value string) ([]string, error) {
	periodIDs := parseCSVValues(value)
	if len(periodIDs) > 0 {
		return periodIDs, nil
	}
	return nil, apperrors.NewValidation(
		"--period-ids must contain at least one non-empty period ID",
		apperrors.WithSubtype(apperrors.SubtypeInvalidFlagValue),
		apperrors.WithHint("Obtain periodId values from `dws agoal user rules --format json` and pass them as a comma-separated list."),
	)
}

func positiveAgoalIntFlag(cmd *cobra.Command, name string) (int, bool, error) {
	if !cmd.Flags().Changed(name) {
		return 0, false, nil
	}
	value, err := cmd.Flags().GetInt(name)
	if err != nil {
		return 0, false, apperrors.NewValidation(
			fmt.Sprintf("--%s must be an integer: %v", name, err),
			apperrors.WithSubtype(apperrors.SubtypeInvalidFlagValue),
		)
	}
	if value < 1 {
		return 0, false, apperrors.NewValidation(
			fmt.Sprintf("--%s must be at least 1, got %d", name, value),
			apperrors.WithSubtype(apperrors.SubtypeInvalidFlagValue),
		)
	}
	return value, true, nil
}
