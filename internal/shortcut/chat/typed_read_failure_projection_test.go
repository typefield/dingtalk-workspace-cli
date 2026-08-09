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

package chat

import (
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestChatCompositeReadFailuresPreserveTypedRecoveryFacts(t *testing.T) {
	typed := apperrors.NewAuth("登录态失效",
		apperrors.WithSubtype(apperrors.SubtypeUpstreamAuthenticationRequired),
		apperrors.WithHint("重新登录后从失败边界继续。"),
		apperrors.WithActions("dws login"),
		apperrors.WithRetryable(false),
	)
	cases := map[string]func(error) *output.ErrorInfo{
		"group search": func(err error) *output.ErrorInfo {
			return chatSearchReadFailureInfo(err, "pagination_read")
		},
		"joined groups": chatListAllReadFailureInfo,
		"conversations": conversationListReadFailureInfo,
		"favorites":     flagListReadFailureInfo,
	}
	for name, project := range cases {
		t.Run(name, func(t *testing.T) {
			info := project(typed)
			if info == nil {
				t.Fatal("typed read failure projected to nil")
			}
			if info.Type != "auth" || info.Subtype != string(apperrors.SubtypeUpstreamAuthenticationRequired) {
				t.Fatalf("typed category/subtype lost: %#v", info)
			}
			if info.Retryable {
				t.Fatalf("explicit retryable=false became retryable=true: %#v", info)
			}
			if info.Hint != "重新登录后从失败边界继续。" || len(info.Actions) != 1 || info.Actions[0] != "dws login" {
				t.Fatalf("typed recovery guidance lost: %#v", info)
			}
			if info.Operation == "" || info.Stage != "pagination_read" {
				t.Fatalf("composite read context lost: %#v", info)
			}
		})
	}
}
