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

package output

// Phase I：golden fixture 与字节稳定性（B156/B157/B188；AC-07）。
// 落盘策略：轮8裁决⑩新文件——不编辑 envelope_test.go 既有断言。
//
// golden 常量：四类 outcome 各一个标准信封。测试数据直接随 Go 测试编译，
// 不在仓库中维护第二份 JSON 文件。
// 比对基准 = jsonutil.MarshalIndent（与 WriteJSON 生产路径同一序列化函数），
// 逐字节比对——字段声明序 + snake_case 键 + 两空格缩进共同决定稳定字节。

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/jsonutil"
)

const goldenEnvelopeFixture = `{
  "failure": {
    "ok": false,
    "outcome": "failure",
    "error": {
      "type": "api",
      "subtype": "rate_limit",
      "code": 90018,
      "message": "rate limited",
      "retryable": true,
      "retry_after_seconds": 30
    }
  },
  "partial_failure": {
    "ok": false,
    "outcome": "partial_failure",
    "data": {
      "total": 3,
      "succeeded": [
        {
          "id": "a",
          "messageId": "m_1"
        }
      ],
      "failed": [
        {
          "id": "b",
          "error": {
            "type": "api",
            "code": 40001,
            "message": "invalid recipient"
          }
        }
      ],
      "unknown": [
        {
          "id": "c",
          "reason": "timeout after submit"
        }
      ]
    }
  },
  "pending": {
    "ok": true,
    "outcome": "pending",
    "data": {
      "accepted": true,
      "taskId": "t_9001"
    },
    "meta": {
      "operation": {
        "id": "t_9001",
        "state": "processing",
        "timed_out": true,
        "next_command": "dws op get t_9001"
      }
    }
  },
  "success": {
    "ok": true,
    "outcome": "success",
    "data": [
      {
        "id": "a",
        "name": "alpha"
      },
      {
        "id": "b",
        "name": "beta"
      }
    ],
    "meta": {
      "count": 2,
      "pagination": {
        "endpoint_exhausted": false,
        "pages": 2,
        "items": 50,
        "next_token": "cursor_abc"
      }
    }
  }
}`

// 四类 outcome 的标准信封构造器（golden 的唯一事实源——B156）。每个都过
// Validate()，保证 golden 本身是合法契约形态而非随手拼的样例。
func goldenEnvelopes(t *testing.T) map[string]*Envelope {
	t.Helper()
	success := NewSuccessEnvelope([]any{
		map[string]any{"id": "a", "name": "alpha"},
		map[string]any{"id": "b", "name": "beta"},
	})
	success.Meta = &Meta{
		Count: NewCount(2),
		Pagination: &Pagination{
			EndpointExhausted: false,
			Pages:             2,
			Items:             50,
			NextToken:         "cursor_abc",
		},
	}

	pending := NewPendingEnvelope(&OperationInfo{
		ID:          "t_9001",
		State:       OperationStateProcessing,
		TimedOut:    true,
		NextCommand: "dws op get t_9001",
	})
	pending.Data = map[string]any{"accepted": true, "taskId": "t_9001"}

	partialData, err := NewPartialData(3,
		[]any{map[string]any{"id": "a", "messageId": "m_1"}},
		[]PartialFailedEntry{{ID: "b", Error: &ErrorInfo{Type: "api", Code: 40001, Message: "invalid recipient"}}},
		[]PartialUnknownEntry{{ID: "c", Reason: "timeout after submit"}},
	)
	if err != nil {
		t.Fatalf("golden partial data must be legal: %v", err)
	}
	partial := NewPartialEnvelope(partialData)

	failure := NewFailureEnvelope(&ErrorInfo{
		Type:              "api",
		Subtype:           "rate_limit",
		Code:              90018,
		Message:           "rate limited",
		Retryable:         true,
		RetryAfterSeconds: int64Ptr(30),
	})

	envs := map[string]*Envelope{
		"success":         success,
		"pending":         pending,
		"partial_failure": partial,
		"failure":         failure,
	}
	for name, env := range envs {
		if verr := env.Validate(); verr != nil {
			t.Fatalf("golden %s envelope must pass Validate: %v", name, verr)
		}
	}
	return envs
}

func int64Ptr(v int64) *int64 { return &v }

// marshalGolden 用生产路径同一序列化函数渲染信封（两空格缩进，与 WriteJSON 一致）。
func marshalGolden(t *testing.T, env *Envelope) []byte {
	t.Helper()
	data, err := jsonutil.MarshalIndent(env, "", "  ")
	if err != nil {
		t.Fatalf("jsonutil.MarshalIndent: %v", err)
	}
	return data
}

// TestEnvelopeGoldenByteForByte 是 B157 的 golden 比对测试：四类 outcome 的
// 标准信封序列化后与评审过的内联常量逐字节一致。
func TestEnvelopeGoldenByteForByte(t *testing.T) {
	envs := goldenEnvelopes(t)
	var golden map[string]json.RawMessage
	if err := json.Unmarshal([]byte(goldenEnvelopeFixture), &golden); err != nil {
		t.Fatalf("golden fixture is not a valid JSON object: %v", err)
	}
	if len(golden) != 4 {
		t.Fatalf("golden must carry exactly the four outcomes, got %d keys", len(golden))
	}
	for name, env := range envs {
		wantRaw, ok := golden[name]
		if !ok {
			t.Fatalf("golden missing outcome %q", name)
		}
		got := marshalGolden(t, env)
		// RawMessage 保留 golden 原始字节；缩进与 MarshalIndent 对齐后逐字节比对。
		var wantBuf bytes.Buffer
		if err := json.Indent(&wantBuf, wantRaw, "", "  "); err != nil {
			t.Fatalf("golden %q is not valid JSON: %v", name, err)
		}
		if !bytes.Equal(got, wantBuf.Bytes()) {
			t.Fatalf("golden %q drifted:\n--- got ---\n%s\n--- want ---\n%s", name, got, wantBuf.Bytes())
		}
	}
}

// TestEnvelopeGoldenFourOutcomesPresent 锁定 golden 覆盖且仅覆盖四类规范
// outcome——不得多（引入第五值）也不得少（漏掉某类形态）。
func TestEnvelopeGoldenFourOutcomesPresent(t *testing.T) {
	var golden map[string]json.RawMessage
	if err := json.Unmarshal([]byte(goldenEnvelopeFixture), &golden); err != nil {
		t.Fatalf("golden invalid: %v", err)
	}
	want := []string{"success", "pending", "partial_failure", "failure"}
	for _, outcome := range want {
		if _, ok := golden[outcome]; !ok {
			t.Fatalf("golden must include outcome %q", outcome)
		}
	}
	if len(golden) != len(want) {
		t.Fatalf("golden must carry exactly the four outcomes, got %d", len(golden))
	}
}
