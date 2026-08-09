// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"fmt"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
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
