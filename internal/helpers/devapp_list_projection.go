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

package helpers

import (
	"fmt"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

// DevAppListPage is the single projection shared by the native `dev app ...`
// tree and the existing `devapp +...` shortcuts. LegacyPage retains the old
// clean shortcut payload for an internal rollout rollback; Data and Meta are
// the only active unified result.
type DevAppListPage struct {
	LegacyPage map[string]any
	Data       map[string]any
	Meta       *output.Meta
}

type devAppListSpec struct {
	dataKey       string
	containerKeys []string
	project       func(map[string]any, int) (map[string]any, *output.ErrorInfo)
}

var devAppListSpecs = map[string]devAppListSpec{
	devAppListTool: {
		dataKey:       "apps",
		containerKeys: []string{"list", "items", "apps", "appList", "result", "data"},
		project: func(item map[string]any, index int) (map[string]any, *output.ErrorInfo) {
			id, ok := devAppProjectionStableString(item, "unifiedAppId", "unified_app_id")
			if !ok {
				return nil, devAppProjectionUnknownInfo(devAppListTool, fmt.Sprintf("devapp app list item %d has no stable unifiedAppId", index+1))
			}
			row := map[string]any{"unifiedAppId": id}
			devAppProjectionCopy(row, item, "name", "name", "appName", "app_name")
			devAppProjectionCopy(row, item, "appKey", "appKey", "clientId", "app_key", "client_id")
			devAppProjectionCopy(row, item, "agentId", "agentId", "agent_id")
			devAppProjectionCopy(row, item, "status", "status", "appStatus", "app_status")
			devAppProjectionCopy(row, item, "gmtModified", "gmtModified", "gmt_modified", "modifyTime", "modified_time")
			return row, nil
		},
	},
	devAppPermissionListTool: {
		dataKey:       "permissions",
		containerKeys: []string{"list", "items", "permissions", "permissionList", "scopes", "result", "data"},
		project: func(item map[string]any, index int) (map[string]any, *output.ErrorInfo) {
			scope, ok := devAppProjectionStableString(item, "scopeValue", "scope_value", "permissionCode", "code")
			if !ok {
				return nil, devAppProjectionUnknownInfo(devAppPermissionListTool, fmt.Sprintf("devapp permission list item %d has no stable scopeValue", index+1))
			}
			row := map[string]any{"scopeValue": scope}
			devAppProjectionCopy(row, item, "scopeName", "scopeName", "scope_name", "permissionName", "name")
			devAppProjectionCopy(row, item, "apiName", "apiName", "api_name", "interfaceName")
			devAppProjectionCopy(row, item, "authStatus", "authStatus", "auth_status", "status")
			devAppProjectionCopy(row, item, "scopeType", "scopeType", "scope_type")
			return row, nil
		},
	},
	devAppEventListTool: {
		dataKey:       "events",
		containerKeys: []string{"list", "items", "events", "eventList", "result", "data"},
		project: func(item map[string]any, index int) (map[string]any, *output.ErrorInfo) {
			code, ok := devAppProjectionStableString(item, "eventCode", "event_code", "code")
			if !ok {
				return nil, devAppProjectionUnknownInfo(devAppEventListTool, fmt.Sprintf("devapp event list item %d has no stable eventCode", index+1))
			}
			row := map[string]any{"eventCode": code}
			devAppProjectionCopy(row, item, "eventName", "eventName", "event_name", "name")
			devAppProjectionCopy(row, item, "status", "status", "subscribeStatus", "subscribe_status")
			devAppProjectionCopy(row, item, "gmtModified", "gmtModified", "gmt_modified", "modifyTime", "modified_time")
			return row, nil
		},
	},
	devAppVersionListTool: {
		dataKey:       "versions",
		containerKeys: []string{"list", "items", "versions", "versionList", "result", "data"},
		project: func(item map[string]any, index int) (map[string]any, *output.ErrorInfo) {
			id, ok := devAppProjectionStableString(item, "versionId", "version_id", "id")
			if !ok {
				return nil, devAppProjectionUnknownInfo(devAppVersionListTool, fmt.Sprintf("devapp version list item %d has no stable versionId", index+1))
			}
			row := map[string]any{"versionId": id}
			devAppProjectionCopy(row, item, "version", "version", "versionName", "version_name")
			devAppProjectionCopy(row, item, "status", "status", "publishStatus", "publish_status", "versionStatus")
			devAppProjectionCopy(row, item, "desc", "desc", "description", "remark")
			devAppProjectionCopy(row, item, "gmtCreate", "gmtCreate", "gmt_create", "createTime", "create_time")
			return row, nil
		},
	},
}

// ProjectDevAppListPage projects one supported DevApp list response. handled
// is false for non-list tools. A recognized empty array is success; an unknown
// container, malformed item, missing stable ID, or inconsistent pagination is
// a typed failure and must not be fabricated as an empty/complete result.
func ProjectDevAppListPage(tool string, source map[string]any) (page *DevAppListPage, problem *output.ErrorInfo, handled bool) {
	spec, handled := devAppListSpecs[strings.TrimSpace(tool)]
	if !handled {
		return nil, nil, false
	}
	raw, problem := devAppProjectionFindList(source, spec.containerKeys, tool)
	if problem != nil {
		return nil, problem, true
	}
	items := make([]map[string]any, 0, len(raw))
	for index, value := range raw {
		item, ok := value.(map[string]any)
		if !ok {
			return nil, devAppProjectionUnknownInfo(tool, fmt.Sprintf("devapp %s list contains a non-object item", spec.dataKey)), true
		}
		projected, issue := spec.project(item, index)
		if issue != nil {
			return nil, issue, true
		}
		items = append(items, projected)
	}

	legacy := map[string]any{"count": len(items), spec.dataKey: items}
	meta, issue := devAppProjectionPagination(source, legacy, len(items))
	if issue != nil {
		return nil, issue, true
	}
	return &DevAppListPage{
		LegacyPage: legacy,
		Data:       map[string]any{spec.dataKey: items},
		Meta:       meta,
	}, nil, true
}

// DevAppListProjectionError recreates the repository typed error used by the
// Cobra error path when a shortcut cannot store a CommandResult directly.
func DevAppListProjectionError(problem *output.ErrorInfo) error {
	if problem == nil {
		return nil
	}
	opts := []apperrors.Option{
		apperrors.WithSubtype(apperrors.Subtype(problem.Subtype)),
		apperrors.WithOperation(problem.Operation),
		apperrors.WithFailureStage(problem.Stage),
		apperrors.WithHint(problem.Hint),
	}
	if problem.Type == string(apperrors.CategoryAPI) {
		opts = append(opts, apperrors.WithRetryable(false))
		return apperrors.NewAPI(problem.Message, opts...)
	}
	return apperrors.NewValidation(problem.Message, opts...)
}

func devAppProjectionFindList(data map[string]any, keys []string, tool string) ([]any, *output.ErrorInfo) {
	if data == nil {
		return nil, devAppProjectionUnknownInfo(tool, "devapp list response is empty or not an object")
	}
	for _, key := range keys {
		value, present := data[key]
		if !present {
			continue
		}
		if list, ok := value.([]any); ok {
			return list, nil
		}
		inner, ok := value.(map[string]any)
		if !ok {
			continue
		}
		for _, nestedKey := range keys {
			if list, ok := inner[nestedKey].([]any); ok {
				return list, nil
			}
		}
	}
	return nil, devAppProjectionUnknownInfo(tool, "devapp list response does not contain a recognized list container")
}

func devAppProjectionFirst(data map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := data[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func devAppProjectionStableString(data map[string]any, keys ...string) (string, bool) {
	value, ok := devAppProjectionFirst(data, keys...)
	if !ok {
		return "", false
	}
	id, ok := value.(string)
	id = strings.TrimSpace(id)
	return id, ok && id != ""
}

func devAppProjectionCopy(target, source map[string]any, targetKey string, sourceKeys ...string) {
	if value, ok := devAppProjectionFirst(source, sourceKeys...); ok {
		target[targetKey] = value
	}
}

func devAppProjectionUnknownInfo(tool, message string) *output.ErrorInfo {
	return &output.ErrorInfo{
		Type:      string(apperrors.CategoryAPI),
		Subtype:   string(apperrors.SubtypeProjectionUnknown),
		Message:   message,
		Hint:      "不要将结果当作空列表；保留脱敏响应证据后修复 DevApp 投影。",
		Operation: devAppProduct + "/" + tool,
		Stage:     "response_projection",
	}
}

type devAppPaginationEvidence struct {
	hasMoreSet bool
	hasMore    bool
	cursorSet  bool
	cursor     string
}

func devAppProjectionPagination(source, legacy map[string]any, count int) (*output.Meta, *output.ErrorInfo) {
	var evidence devAppPaginationEvidence
	var visit func(map[string]any, int) *output.ErrorInfo
	visit = func(data map[string]any, depth int) *output.ErrorInfo {
		if data == nil || depth > 3 {
			return nil
		}
		if raw, present := data["hasMore"]; present {
			value, ok := raw.(bool)
			if !ok {
				return devAppPaginationProblem(apperrors.SubtypePaginationInvalid, "devapp pagination hasMore must be a boolean")
			}
			if evidence.hasMoreSet && evidence.hasMore != value {
				return devAppPaginationProblem(apperrors.SubtypePaginationConflict, "devapp pagination hasMore conflicts across nested response envelopes")
			}
			evidence.hasMoreSet, evidence.hasMore = true, value
		}
		if raw, present := data["nextCursor"]; present {
			value, ok := raw.(string)
			if !ok {
				return devAppPaginationProblem(apperrors.SubtypePaginationIncomplete, "devapp pagination nextCursor must be a string")
			}
			value = strings.TrimSpace(value)
			if evidence.cursorSet && evidence.cursor != "" && value != "" && evidence.cursor != value {
				return devAppPaginationProblem(apperrors.SubtypePaginationConflict, "devapp pagination nextCursor conflicts across nested response envelopes")
			}
			if value != "" {
				evidence.cursor = value
			}
			evidence.cursorSet = true
		}
		for _, key := range []string{"result", "data", "content", "pageInfo", "pagination"} {
			if nested, ok := data[key].(map[string]any); ok {
				if problem := visit(nested, depth+1); problem != nil {
					return problem
				}
			}
		}
		return nil
	}
	if problem := visit(source, 0); problem != nil {
		return nil, problem
	}
	if evidence.hasMoreSet && evidence.hasMore && evidence.cursor == "" {
		return nil, devAppPaginationProblem(apperrors.SubtypePaginationIncomplete, "devapp pagination hasMore=true requires nextCursor")
	}
	if evidence.hasMoreSet {
		legacy["hasMore"] = evidence.hasMore
	}
	if evidence.cursor != "" && (!evidence.hasMoreSet || evidence.hasMore) {
		legacy["nextCursor"] = evidence.cursor
	}

	meta := &output.Meta{Count: output.NewCount(count)}
	switch {
	case evidence.hasMoreSet && evidence.hasMore:
		meta.Pagination = &output.Pagination{EndpointExhausted: false, NextToken: evidence.cursor}
	case evidence.hasMoreSet && !evidence.hasMore:
		meta.Pagination = &output.Pagination{EndpointExhausted: true}
	case evidence.cursor != "":
		meta.Pagination = &output.Pagination{EndpointExhausted: false, NextToken: evidence.cursor}
	}
	return meta, nil
}

func devAppPaginationProblem(subtype apperrors.Subtype, message string) *output.ErrorInfo {
	return &output.ErrorInfo{
		Type:      string(apperrors.CategoryValidation),
		Subtype:   string(subtype),
		Message:   message,
		Hint:      "不要将结果当作完整列表；保留脱敏响应证据后排查上游分页字段。",
		Operation: "devapp.pagination_projection",
		Stage:     "response_projection",
	}
}
