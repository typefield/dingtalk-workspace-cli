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

package smart

import (
	"fmt"
	"strconv"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// todoPageSize is the per-page fetch size for get_user_todos_in_current_org.
// The real backend currently returns an empty page for pageSize=50 while
// pageSize=20 works, so keep this aligned with the public +get-my-tasks default
// instead of pushing the maximum. todoMaxPages caps total pages so a runaway
// list can never loop unbounded (40 * 20 = 800 todos, well past any realistic
// backlog a single user filters/matches against).
const (
	todoPageSize = 20
	todoMaxPages = 40
)

// todoCardsRead keeps the evidence obtained before a paged todo read stopped.
// The upstream API does not provide an authoritative has_more / next_cursor
// pair, so a short page is only a local stopping heuristic. It must never be
// promoted to endpoint_exhausted=true in the unified result.
//
// Err is deliberately retained alongside Cards: read-only callers can express
// a truthful partial_failure after a later-page error, whereas write preflight
// callers must continue to fail closed through shortcutListAllTodoCards.
type todoCardsRead struct {
	Cards        []map[string]any
	PagesFetched int
	Err          error
	Failure      *output.ErrorInfo
}

// shortcutReadAllTodoCards pages through get_user_todos_in_current_org while
// preserving pages already read if a later page fails. It performs exactly one
// MCP call per page; callers must not retry it to construct a unified result.
func shortcutReadAllTodoCards(rt *shortcut.RuntimeContext, base map[string]any) todoCardsRead {
	all := make([]map[string]any, 0)
	for page := 1; page <= todoMaxPages; page++ {
		params := make(map[string]any, len(base)+2)
		for k, v := range base {
			params[k] = v
		}
		params["pageNum"] = strconv.Itoa(page)
		params["pageSize"] = strconv.Itoa(todoPageSize)

		data, err := rt.CallMCPData("todo", "get_user_todos_in_current_org", params)
		if err != nil {
			return todoCardsRead{
				Cards:        all,
				PagesFetched: page - 1,
				Err:          err,
				Failure:      todoCardsReadFailureInfo(err),
			}
		}
		cards, err := shortcutTodoCards(data)
		if err != nil {
			return todoCardsRead{
				Cards:        all,
				PagesFetched: page - 1,
				Err:          err,
				Failure:      todoCardsProjectionFailureInfo(err),
			}
		}
		all = append(all, cards...)
		pagesFetched := page
		if len(cards) < todoPageSize {
			return todoCardsRead{Cards: all, PagesFetched: pagesFetched}
		}
		if page == todoMaxPages {
			// A full final page proves that another page may exist. Never return
			// this capped result as a complete list: callers would otherwise
			// project a silently truncated success and Agents could act on an
			// incomplete todo set.
			err := apperrors.NewValidation("待办列表超过单次聚合上限，请缩小筛选范围后重试")
			return todoCardsRead{
				Cards:        all,
				PagesFetched: pagesFetched,
				Err:          err,
				Failure:      todoCardsCapFailureInfo(err),
			}
		}
	}
	// todoMaxPages is an explicit cap, so this return is only defensive. The
	// branch above handles the reachable full-page case.
	return todoCardsRead{Cards: all, PagesFetched: todoMaxPages}
}

// shortcutListAllTodoCards is the compatibility/fail-closed facade used by
// existing callers, notably +todo-done's write preflight. It deliberately
// discards any earlier read pages when the aggregate is incomplete: a write
// must never select a task from a partial list.
func shortcutListAllTodoCards(rt *shortcut.RuntimeContext, base map[string]any) ([]map[string]any, error) {
	read := shortcutReadAllTodoCards(rt, base)
	if read.Err != nil {
		return nil, read.Err
	}
	return read.Cards, nil
}

// todoAggregateResult builds the future unified result for one read-only todo
// aggregate. A partial read preserves the already-projected payload in the
// succeeded channel, while the failed page remains typed and addressable.
// During dual_validate the caller still returns the historical error and no
// payload; activation is the only point at which this result reaches stdout.
func todoAggregateResult(read todoCardsRead, resultID string, payload map[string]any, dryRun bool) (output.CommandResult, error) {
	payload = todoAggregateUnifiedPayload(payload)
	options := []output.ResultOption{output.WithMeta(&output.Meta{Count: output.NewCount(todoAggregateCount(payload))})}
	if dryRun {
		options = append(options, output.WithDryRun())
	}
	if read.Err == nil {
		return output.Success(payload, options...), nil
	}
	if read.Failure == nil {
		return nil, fmt.Errorf("todo aggregate failure is missing typed error information")
	}
	if len(read.Cards) == 0 {
		return output.Failure(read.Failure, options...), nil
	}
	entry := map[string]any{
		"id":            resultID,
		"pages_fetched": read.PagesFetched,
		"items_fetched": len(read.Cards),
		"data":          payload,
	}
	failed := output.PartialFailedEntry{
		ID:    fmt.Sprintf("page:%d", read.PagesFetched+1),
		Error: read.Failure,
	}
	partial, err := output.NewPartialData(2, []any{entry}, []output.PartialFailedEntry{failed}, []output.PartialUnknownEntry{})
	if err != nil {
		return nil, err
	}
	return output.Partial(partial, options...), nil
}

// todoAggregateOutput preserves the current public behavior until activation:
// legacy/dual callers receive the legacy payload on success and the original
// error with no payload on a partial read. Unified callers emit the prepared
// result once, including partial_failure (rc 7).
func todoAggregateOutput(rt *shortcut.RuntimeContext, payload map[string]any, result output.CommandResult, legacyErr error) error {
	if result == nil {
		return fmt.Errorf("todo aggregate result must not be nil")
	}
	if result.Outcome() == output.OutcomePartialFailure {
		return rt.OutputPartial(result, legacyErr)
	}
	if output.UsesUnifiedResult(rt.Command()) {
		return rt.OutputResult(payload, result)
	}
	if output.CommandRollout(rt.Command()) == output.RolloutDualValidate {
		if err := output.ValidateResult(result); err != nil {
			return err
		}
	}
	if legacyErr != nil {
		return legacyErr
	}
	return rt.OutputResult(payload, result)
}

func todoAggregateUnifiedPayload(payload map[string]any) map[string]any {
	result := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		result[key] = value
	}
	// A short todoCards page is a local heuristic, not an upstream completion
	// declaration. This explicit fact prevents an Agent from treating an empty
	// or short response as proof that the whole todo corpus was searched.
	result["pagination_known"] = false
	return result
}

func todoAggregateCount(payload map[string]any) int {
	for _, key := range []string{"count", "tasks", "created", "overdue"} {
		value, ok := payload[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case []map[string]any:
			return len(typed)
		case []any:
			return len(typed)
		}
	}
	return 0
}

func todoCardsReadFailureInfo(err error) *output.ErrorInfo {
	message := "待办聚合分页读取失败"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	info := &output.ErrorInfo{
		Type:             "api",
		Message:          message,
		Hint:             "保留已读取待办；从失败页重新读取，不要把当前结果解释为完整待办集合。",
		Operation:        "todo/get_user_todos_in_current_org",
		Origin:           "mcp_gateway",
		Stage:            "pagination_read",
		ExecutionStarted: &started,
		Retryable:        true,
	}
	return shortcut.PreserveTypedErrorInfo(info, err)
}

func todoCardsProjectionFailureInfo(err error) *output.ErrorInfo {
	message := "待办聚合响应无法可靠投影"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	return &output.ErrorInfo{
		Type:             "api",
		Subtype:          string(apperrors.SubtypeProjectionUnknown),
		Message:          message,
		Hint:             "检查服务端返回结构；不要把未知响应形状解释为空待办结果。",
		Operation:        "todo/get_user_todos_in_current_org",
		Origin:           "mcp_gateway",
		Stage:            "response_projection",
		ExecutionStarted: &started,
	}
}

func todoCardsCapFailureInfo(err error) *output.ErrorInfo {
	message := "待办聚合达到安全页数上限"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	started := true
	return &output.ErrorInfo{
		Type:             "validation",
		Subtype:          string(apperrors.SubtypePaginationIncomplete),
		Message:          message,
		Hint:             "缩小筛选范围后重新读取；不要把当前结果解释为完整待办集合。",
		Operation:        "todo/get_user_todos_in_current_org",
		Origin:           "mcp_gateway",
		Stage:            "pagination_limit",
		ExecutionStarted: &started,
	}
}
