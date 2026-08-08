// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

type serverFailureClass struct {
	message string
	subtype apperrors.Subtype
	origin  string
	stage   string
	hint    string
	actions []string
}

func classifyServerFailure(message string, diag apperrors.ServerDiagnostics) (serverFailureClass, bool) {
	code := strings.ToUpper(strings.TrimSpace(diag.ServerErrorCode))
	detail := strings.ToLower(strings.TrimSpace(diag.TechnicalDetail))
	text := strings.ToLower(strings.TrimSpace(message))

	if code == "NETWORK_ERROR" ||
		strings.Contains(detail, "statuscode.unavailable") ||
		strings.Contains(detail, "connection refused") {
		classified := serverFailureClass{
			message: "MCP 后端依赖暂时不可用",
			subtype: apperrors.SubtypeBackendDependencyUnavailable,
			origin:  "mcp_gateway",
			stage:   "backend_dependency",
			hint:    "请求参数无需修改；请使用相同参数稍后重试。持续失败时请提供 Trace ID 排查 MCP 服务。",
			actions: []string{
				"使用相同参数重试一次",
				"持续失败时保留 Trace ID 并排查 MCP 后端依赖",
			},
		}
		if strings.Contains(detail, "querytoolmeta") {
			classified.message = "MCP 后端元数据服务暂时不可用"
			classified.stage = "tool_metadata_lookup"
		}
		return classified, true
	}

	if code == "PARAM_ERROR" ||
		strings.Contains(text, "opencid or cid is required") ||
		strings.Contains(text, "openconversationid") && strings.Contains(text, "required") {
		return serverFailureClass{
			message: message,
			subtype: apperrors.SubtypeUpstreamRequestRejected,
			origin:  "dingtalk_api",
			stage:   "tool_validation",
			hint:    "请求未通过后端参数校验；请核对当前 leaf Help/Schema 和稳定 ID 类型后重试。",
		}, true
	}

	return serverFailureClass{}, false
}

func newServerFailureAPIError(
	message string,
	fallbackReason string,
	fallbackHint string,
	serverKey string,
	diag apperrors.ServerDiagnostics,
) error {
	classified, classifiedOK := classifyServerFailure(message, diag)
	// A server-provided retryable bit is a diagnostic observation, not proof
	// that replaying an arbitrary tools/call is safe.  Preserve it as details
	// unless the reviewed backend-dependency class explicitly accepts service
	// retry guidance.
	recoveryDiag := diag
	if diag.ServerRetryable != nil && (!classifiedOK || classified.subtype != apperrors.SubtypeBackendDependencyUnavailable) {
		recoveryDiag.ServerRetryable = nil
	}
	opts := []apperrors.Option{
		apperrors.WithOperation("tools/call"),
		// The free-form fallback is retained only as a diagnostic label.  It is
		// not an Agent branch key: upstream business failures must use a closed
		// subtype even when their server message changes.
		apperrors.WithSubtype(apperrors.SubtypeUpstreamUnclassified),
		apperrors.WithServerKey(serverKey),
		apperrors.WithHint(fallbackHint),
		apperrors.WithServerDiag(recoveryDiag),
		apperrors.WithDetails(map[string]any{"server_failure_kind": fallbackReason}),
	}
	if diag.ServerRetryable != nil && recoveryDiag.ServerRetryable == nil {
		opts = append(opts, apperrors.WithDetails(map[string]any{"server_retryable": *diag.ServerRetryable}))
	}
	if classifiedOK {
		message = classified.message
		opts = append(opts,
			apperrors.WithSubtype(classified.subtype),
			apperrors.WithOrigin(classified.origin),
			apperrors.WithFailureStage(classified.stage),
			apperrors.WithHint(classified.hint),
			apperrors.WithActions(classified.actions...),
		)
	}
	return apperrors.NewAPI(message, opts...)
}

func serverFailureReason(err error, fallback string) string {
	typed, ok := err.(*apperrors.Error)
	if ok && strings.TrimSpace(typed.Reason) != "" {
		return typed.Reason
	}
	return fallback
}
