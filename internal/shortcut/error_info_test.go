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
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestPreserveTypedErrorInfoKeepsTypedRecoveryAndCompositeContext(t *testing.T) {
	started := true
	fallback := &output.ErrorInfo{
		Type:             "api",
		Message:          "复合读取失败",
		Hint:             "从失败页继续读取。",
		Operation:        "chat/list_messages",
		Origin:           "mcp_gateway",
		Stage:            "pagination_read",
		ExecutionStarted: &started,
		Retryable:        true,
		Details:          map[string]any{"page": 2, "context": "composite"},
	}
	typed := apperrors.NewAuth("登录已过期",
		apperrors.WithSubtype(apperrors.SubtypeUpstreamAuthenticationRequired),
		apperrors.WithHint("先重新登录。"),
		apperrors.WithActions("dws login"),
		apperrors.WithRetryable(false),
		apperrors.WithOperation("chat/list_messages"),
		apperrors.WithDetails(map[string]any{"page": "upstream", "request": "r-1"}),
	)

	info := PreserveTypedErrorInfo(fallback, typed)
	if info.Type != "auth" || info.Subtype != string(apperrors.SubtypeUpstreamAuthenticationRequired) {
		t.Fatalf("type/subtype=%#v", info)
	}
	if info.Retryable || info.Hint != "先重新登录。" || len(info.Actions) != 1 || info.Actions[0] != "dws login" {
		t.Fatalf("recovery=%#v", info)
	}
	if info.Details["page"] != 2 || info.Details["upstream_page"] != "upstream" || info.Details["request"] != "r-1" {
		t.Fatalf("merged details=%#v", info.Details)
	}
	if fallback.Retryable != true || fallback.Details["page"] != 2 {
		t.Fatalf("fallback was mutated: %#v", fallback)
	}
}

func TestPreserveTypedErrorInfoRejectsPlainPartialErrorType(t *testing.T) {
	info := PreserveTypedErrorInfo(&output.ErrorInfo{Type: "api"}, &apperrors.Error{
		Category: apperrors.CategoryPartial,
		Message:  "invalid partial error",
	})
	if info.Type != "internal" {
		t.Fatalf("partial error type=%q, want internal", info.Type)
	}
}
