package mail

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

// mailListMeta carries only pagination facts observed in a mailbox response.
// A response with no pagination fields remains a successful single response;
// it is not silently promoted to an exhausted endpoint.
func mailListMeta(data map[string]any, count int) (*output.Meta, error) {
	meta := &output.Meta{Count: output.NewCount(count)}
	page, known, err := mailPagination(data)
	if err != nil {
		return nil, err
	}
	if known {
		meta.Pagination = page
	}
	return meta, nil
}

// mailPagination projects the documented hasMore/nextCursor pair. Both the
// outer response and its conventional result/data wrappers are inspected;
// contradictory duplicates are an upstream inconsistency, never an excuse to
// advertise a completed list.
func mailPagination(data map[string]any) (*output.Pagination, bool, error) {
	containers := []map[string]any{data}
	for _, key := range []string{"result", "data"} {
		if nested, ok := data[key].(map[string]any); ok {
			containers = append(containers, nested)
		}
	}

	var (
		hasMoreSet bool
		hasMore    bool
		cursorSet  bool
		cursor     string
	)
	for _, container := range containers {
		for _, key := range []string{"hasMore", "has_more"} {
			raw, present := container[key]
			if !present {
				continue
			}
			value, ok := raw.(bool)
			if !ok {
				return nil, false, mailPaginationError("邮件列表的 hasMore 必须是布尔值")
			}
			if hasMoreSet && hasMore != value {
				return nil, false, mailPaginationError("邮件列表的分页 hasMore 证据互相矛盾")
			}
			hasMoreSet, hasMore = true, value
		}
		for _, key := range []string{"nextCursor", "next_cursor"} {
			raw, present := container[key]
			if !present {
				continue
			}
			value, err := mailCursor(raw)
			if err != nil {
				return nil, false, err
			}
			if cursorSet && cursor != value {
				return nil, false, mailPaginationError("邮件列表的分页 nextCursor 证据互相矛盾")
			}
			cursorSet, cursor = true, value
		}
	}

	if !hasMoreSet && !cursorSet {
		return nil, false, nil
	}
	if !hasMoreSet {
		return nil, false, mailPaginationError("邮件列表返回 nextCursor 但缺少 hasMore，无法判断是否可续页")
	}
	if hasMore {
		if !cursorSet || mailTerminalCursor(cursor) {
			return nil, false, mailPaginationError("邮件列表 hasMore=true 但缺少可用 nextCursor")
		}
		return &output.Pagination{EndpointExhausted: false, NextToken: cursor}, true, nil
	}
	if cursorSet && !mailTerminalCursor(cursor) {
		return nil, false, mailPaginationError("邮件列表 hasMore=false 却携带续页 nextCursor")
	}
	return &output.Pagination{EndpointExhausted: true}, true, nil
}

func mailCursor(raw any) (string, error) {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value), nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) || value != math.Trunc(value) {
			return "", mailPaginationError("邮件列表的 nextCursor 必须是字符串或整数")
		}
		return strconv.FormatInt(int64(value), 10), nil
	case int:
		return strconv.Itoa(value), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	default:
		return "", mailPaginationError(fmt.Sprintf("邮件列表的 nextCursor 类型无效: %T", raw))
	}
}

func mailTerminalCursor(cursor string) bool {
	switch strings.TrimSpace(cursor) {
	case "", "$", "0":
		return true
	default:
		return false
	}
}

func mailPaginationError(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithReason("pagination_inconsistent"),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}
