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

package shortcut

import (
	"encoding/json"
	stderrors "errors"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

// PreserveTypedErrorInfo enriches a composite command's contextual error with
// the lower layer's reviewed error facts.  Composite reads commonly add a
// command-specific operation/stage/hint, but must not thereby turn an
// auth/validation/projection failure into a generic retryable API error.
//
// fallback owns the composite context and remains the result for an untyped
// error.  A repository *errors.Error is authoritative for category, subtype,
// recovery guidance and retry safety.  The helper makes a fresh ErrorInfo and
// copies collections, so a caller may safely reuse its fallback template.
func PreserveTypedErrorInfo(fallback *output.ErrorInfo, err error) *output.ErrorInfo {
	info := cloneErrorInfoFallback(fallback)
	if err == nil {
		return info
	}
	if info.Message == "" {
		info.Message = err.Error()
	}

	var typed *apperrors.Error
	if !stderrors.As(err, &typed) || typed == nil {
		return info
	}
	if typed.Category == apperrors.CategoryPartial {
		// A plain error has no three-channel result data, so it cannot be
		// represented as an error type named partial_failure.  Preserve the
		// framework's fail-closed behavior instead.
		info.Type = string(apperrors.CategoryInternal)
	} else if typed.Category != "" {
		info.Type = string(typed.Category)
	}
	if typed.StableSubtype != "" {
		info.Subtype = typed.StableSubtype
	} else if typed.Reason != "" {
		info.Subtype = typed.Reason
	}
	if typed.Hint != "" {
		info.Hint = typed.Hint
	}
	if len(typed.Actions) > 0 {
		info.Actions = append([]string(nil), typed.Actions...)
	}
	if typed.RetryableSet {
		info.Retryable = typed.Retryable
	}
	if typed.RetryAfterSeconds != nil {
		info.RetryAfterSeconds = typed.RetryAfterSeconds
	}
	if typed.ExecutionStarted != nil {
		started := *typed.ExecutionStarted
		info.ExecutionStarted = &started
	}
	if typed.Operation != "" {
		info.Operation = typed.Operation
	}
	if typed.ServerKey != "" {
		info.ServerKey = typed.ServerKey
	}
	if typed.Origin != "" {
		info.Origin = typed.Origin
	}
	if typed.FailureStage != "" {
		info.Stage = typed.FailureStage
	}
	if typed.RPCCode != 0 {
		info.RPCCode = typed.RPCCode
	}
	if len(typed.RPCData) > 0 {
		var data any
		if json.Unmarshal(typed.RPCData, &data) == nil {
			info.RPCData = data
		}
	}
	if typed.ServerDiag.TraceID != "" {
		info.TraceID = typed.ServerDiag.TraceID
	}
	if typed.ServerDiag.ServerErrorCode != "" {
		info.UpstreamCode = typed.ServerDiag.ServerErrorCode
	}
	if typed.ServerDiag.TechnicalDetail != "" {
		info.TechnicalDetail = typed.ServerDiag.TechnicalDetail
	}
	if hint, actionURL := apperrors.ServerGuidance(typed.ServerDiag); hint != "" || actionURL != "" {
		info.FriendlyHint = hint
		info.ActionURL = actionURL
	}
	if typed.Cause != nil {
		info.Cause = typed.Cause.Error()
	}
	if len(typed.Details) > 0 {
		info.Details = mergeErrorDetails(info.Details, typed.Details)
	}
	return info
}

func cloneErrorInfoFallback(source *output.ErrorInfo) *output.ErrorInfo {
	if source == nil {
		return &output.ErrorInfo{Type: string(apperrors.CategoryInternal)}
	}
	clone := *source
	clone.Actions = append([]string(nil), source.Actions...)
	clone.AvailableFlags = append([]string(nil), source.AvailableFlags...)
	clone.Params = append([]string(nil), source.Params...)
	if source.ExecutionStarted != nil {
		started := *source.ExecutionStarted
		clone.ExecutionStarted = &started
	}
	clone.Details = mergeErrorDetails(nil, source.Details)
	return &clone
}

// mergeErrorDetails preserves the composite context. If a lower error uses
// the same key, retain both pieces of evidence instead of silently replacing
// one with the other.
func mergeErrorDetails(base, upstream map[string]any) map[string]any {
	if len(base) == 0 && len(upstream) == 0 {
		return nil
	}
	merged := make(map[string]any, len(base)+len(upstream))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range upstream {
		if _, exists := merged[key]; exists {
			merged["upstream_"+key] = value
			continue
		}
		merged[key] = value
	}
	return merged
}
