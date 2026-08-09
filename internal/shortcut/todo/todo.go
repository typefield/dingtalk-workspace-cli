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

// Package todo declares high-fidelity shortcuts for the DingTalk "todo"
// (personal todo) MCP service. Tool names and parameter keys are copied verbatim
// from internal/helpers/todo.go; do not invent tools or params here.
package todo

import (
	"strconv"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// Create maps helper `create_personal_todo`.
// CreateSub maps helper `create_personal_sub_todo`.
// GetMyTasks maps helper `get_user_todos_in_current_org`.
var GetMyTasks = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "todo",
	Command:       "+get-my-tasks",
	Product:       "todo",
	Description:   "查询当前组织下我的待办列表",
	Intent:        "当你想查看自己在当前组织下的待办清单、盘点未完成事项或按条件筛选任务时使用；可按完成状态、优先级、角色（创建者/执行者/参与者）和截止时间范围过滤并分页，返回匹配的待办列表。",
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "todo",
			Name:           "shortcut_get_my_tasks",
			CanonicalPath:  "todo.shortcut_get_my_tasks",
			CLIPath:        "todo +get-my-tasks",
			PrimaryCLIPath: "todo +get-my-tasks",
		},
		Description: "查询当前组织下我的待办列表",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "查询当前组织下我的待办列表",
			UseWhen:      []string{"当你想查看自己在当前组织下的待办清单、盘点未完成事项或按条件筛选任务时使用；可按完成状态、优先级、角色（创建者/执行者/参与者）和截止时间范围过滤并分页，返回匹配的待办列表。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws todo +get-my-tasks --status false --priority 40,30"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "page", Type: shortcut.FlagString, Default: "1", Desc: "页码"},
		{Name: "size", Type: shortcut.FlagString, Default: "20", Desc: "每页数量"},
		{Name: "status", Type: shortcut.FlagString, Enum: []string{"true", "false"}, Desc: "true=已完成, false=未完成"},
		{Name: "priority", Type: shortcut.FlagStringSlice, Desc: "优先级过滤: 10/20/30/40"},
		{Name: "role-types", Type: shortcut.FlagStringSlice, Default: "executor", Desc: "角色类型: creator/executor/participant"},
		{Name: "plan-finish-start", Type: shortcut.FlagInt, Desc: "截止时间范围开始（Unix 毫秒时间戳）"},
		{Name: "plan-finish-end", Type: shortcut.FlagInt, Desc: "截止时间范围结束（Unix 毫秒时间戳）"},
	},
	Tips: []string{`dws todo +get-my-tasks --status false --priority 40,30`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"pageNum":  rt.Str("page"),
			"pageSize": rt.Str("size"),
		}
		if rt.Changed("status") {
			params["todoStatus"] = rt.Str("status")
		}
		if rt.Changed("priority") {
			var ps []int
			for _, p := range rt.StrSlice("priority") {
				if n, err := strconv.Atoi(p); err == nil {
					ps = append(ps, n)
				}
			}
			if ps != nil {
				params["priorityList"] = ps
			}
		}
		roles := rt.StrSlice("role-types")
		if len(roles) == 0 {
			roles = []string{"executor"}
		}
		params["roleTypes"] = roles
		if rt.Changed("plan-finish-start") {
			params["planFinishDateStart"] = rt.Int("plan-finish-start")
		}
		if rt.Changed("plan-finish-end") {
			params["planFinishDateEnd"] = rt.Int("plan-finish-end")
		}
		data, err := rt.CallMCPData("todo", "get_user_todos_in_current_org", params)
		if err != nil {
			return err
		}
		cards, err := getMyTasksProject(data)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"count":            len(cards),
			"todos":            cards,
			"pagination_known": false,
		}
		return todoListOutput(rt, payload, len(cards))
	},
}

// getMyTasksProject reshapes get_user_todos_in_current_org into a clean todo
// list (subject/taskId/dueTime/priority/done) — clean output projection.
// The card list lives under result.todoCards; fields are probed defensively.
// A known empty card list succeeds, but an unrecognized response cannot claim
// that the user has no tasks.
func getMyTasksProject(data map[string]any) ([]map[string]any, error) {
	container := data
	if r, ok := data["result"].(map[string]any); ok {
		container = r
	}
	raw, ok := container["todoCards"].([]any)
	if !ok {
		return nil, todoProjectionUnknown("待办列表响应缺少 result.todoCards")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, todoProjectionUnknown("待办列表包含无法识别的条目")
		}
		row := map[string]any{}
		if taskID, ok := todoStableTaskID(m, "taskId", "task_id", "id"); ok {
			row["taskId"] = taskID
		} else {
			return nil, todoProjectionUnknown("待办列表条目缺少可用于后续操作的稳定 taskId")
		}
		for _, k := range []string{"subject", "dueTime", "priority", "finalStatusStage", "creatorId"} {
			if v, ok := m[k]; ok {
				row[k] = v
			}
		}
		if len(row) == 0 {
			return nil, todoProjectionUnknown("待办列表条目缺少可识别字段")
		}
		out = append(out, row)
	}
	return out, nil
}

func todoStableTaskID(m map[string]any, keys ...string) (string, bool) {
	return todoStableID(m, keys...)
}

func todoStableID(m map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := m[key]
		if !ok {
			continue
		}
		id, ok := value.(string)
		if ok && strings.TrimSpace(id) != "" {
			return id, true
		}
	}
	return "", false
}

func todoProjectionUnknown(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

// ListSub maps helper `list_sub_tasks`.
var ListSub = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "todo",
	Command:       "+list-sub",
	Product:       "todo",
	Description:   "查询子待办列表",
	Intent:        "当你已知某个待办任务 ID、想了解它被拆解出的所有子任务时使用；输入父任务 ID，返回其下的子待办列表。",
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "todo",
			Name:           "shortcut_list_sub",
			CanonicalPath:  "todo.shortcut_list_sub",
			CLIPath:        "todo +list-sub",
			PrimaryCLIPath: "todo +list-sub",
		},
		Description: "查询子待办列表",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "查询子待办列表",
			UseWhen:      []string{"当你已知某个待办任务 ID、想了解它被拆解出的所有子任务时使用；输入父任务 ID，返回其下的子待办列表。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws todo +list-sub --task-id <taskId>"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "task-id", Type: shortcut.FlagString, Desc: "待办任务 ID", Required: true},
	},
	Tips: []string{`dws todo +list-sub --task-id <taskId>`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		data, err := rt.CallMCPData("todo", "list_sub_tasks", map[string]any{
			"todoSubTaskListRequest": map[string]any{
				"taskId": rt.Str("task-id"),
			},
		})
		if err != nil {
			return err
		}
		subs, err := listSubProject(data)
		if err != nil {
			return err
		}
		payload := map[string]any{"count": len(subs), "subTasks": subs}
		return todoListOutput(rt, payload, len(subs))
	},
}

// listSubProject reshapes list_sub_tasks into a clean sub-todo list
// (subject/taskId/dueTime) — clean output projection. The list
// container and field names are probed defensively across candidate keys.
// A missing list, malformed entry, or non-targetable entry is a projection
// failure, never a fabricated empty sub-task list.
func listSubProject(data map[string]any) ([]map[string]any, error) {
	raw, known := todoListExtract(data, "list", "items", "subTasks", "subTaskList")
	if !known {
		return nil, todoProjectionUnknown("子待办列表响应缺少可识别的列表容器")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, todoProjectionUnknown("子待办列表包含无法识别的条目")
		}
		row := map[string]any{}
		if taskID, ok := todoStableTaskID(m, "taskId", "task_id", "id", "subTaskId"); ok {
			row["taskId"] = taskID
		} else {
			return nil, todoProjectionUnknown("子待办列表条目缺少可用于后续操作的稳定 taskId")
		}
		if v := listSubFirst(m, "subject", "title", "name", "content"); v != nil {
			row["subject"] = v
		}
		if v := listSubFirst(m, "dueTime", "dueDate", "deadline"); v != nil {
			row["dueTime"] = v
		}
		out = append(out, row)
	}
	return out, nil
}

// todoListExtract unwraps a bare slice or one nested under result/data,
// returning whether a recognized list container was present. This distinction
// is essential: a known empty list is a successful empty result, while an
// unknown response shape cannot safely be called empty.
func todoListExtract(data map[string]any, keys ...string) ([]any, bool) {
	if data == nil {
		return nil, false
	}
	containers := []map[string]any{data}
	for _, k := range []string{"result", "data"} {
		if m, ok := data[k].(map[string]any); ok {
			containers = append(containers, m)
		}
	}
	for _, c := range containers {
		for _, k := range keys {
			if arr, ok := c[k].([]any); ok {
				return arr, true
			}
		}
	}
	// data itself may carry a bare list under result/data.
	for _, k := range []string{"result", "data"} {
		if arr, ok := data[k].([]any); ok {
			return arr, true
		}
	}
	return nil, false
}

// listSubFirst returns the first present value among the candidate keys.
func listSubFirst(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

// Update maps helper `update_todo_task`.
// Complete maps helper `update_todo_done_status`.
// Get maps helper `get_todo_detail`.
var Get = shortcut.Shortcut{
	Service:     "todo",
	Command:     "+get",
	Product:     "todo",
	Description: "查询待办详情",
	Intent:      "当你已知某条待办的任务 ID、想查看它的完整信息（标题、执行者、截止时间、优先级、状态等）时使用；输入任务 ID，返回该待办的详细内容。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "todo",
			Name:           "shortcut_get",
			CanonicalPath:  "todo.shortcut_get",
			CLIPath:        "todo +get",
			PrimaryCLIPath: "todo +get",
		},
		Description: "查询待办详情",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "查询待办详情",
			UseWhen:      []string{"当你已知某条待办的任务 ID、想查看它的完整信息（标题、执行者、截止时间、优先级、状态等）时使用；输入任务 ID，返回该待办的详细内容。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws todo +get --task-id <taskId>"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "task-id", Type: shortcut.FlagString, Desc: "待办任务 ID", Required: true},
	},
	Tips: []string{`dws todo +get --task-id <taskId>`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		return rt.CallMCP("get_todo_detail", map[string]any{
			"taskId": rt.Str("task-id"),
		})
	},
}

// Delete maps helper `delete_todo`.
// AddExecutor maps helper `add_task_executors`.
// RemoveExecutor maps helper `remove_task_executors`.
// AddParticipant maps helper `add_task_participants`.
// RemoveParticipant maps helper `remove_task_participants`.
// AddReminder maps helper `add_todo_reminder`.
// ListAttachment maps helper `list_todo_attachment`.
var ListAttachment = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "todo",
	Command:       "+list-attachment",
	Product:       "todo",
	Description:   "查询待办任务的附件列表",
	Intent:        "当你想查看某条待办上挂了哪些附件、或需要拿到附件 ID 以便后续删除时使用；输入任务 ID，返回该待办的附件列表。",
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "todo",
			Name:           "shortcut_list_attachment",
			CanonicalPath:  "todo.shortcut_list_attachment",
			CLIPath:        "todo +list-attachment",
			PrimaryCLIPath: "todo +list-attachment",
		},
		Description: "查询待办任务的附件列表",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "查询待办任务的附件列表",
			UseWhen:      []string{"当你想查看某条待办上挂了哪些附件、或需要拿到附件 ID 以便后续删除时使用；输入任务 ID，返回该待办的附件列表。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws todo +list-attachment --task-id <taskId>"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "task-id", Type: shortcut.FlagString, Desc: "待办任务 ID", Required: true},
	},
	Tips: []string{`dws todo +list-attachment --task-id <taskId>`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		data, err := rt.CallMCPData("todo", "list_todo_attachment", map[string]any{
			"todoAttachmentListRequest": map[string]any{
				"taskId": rt.Str("task-id"),
			},
		})
		if err != nil {
			return err
		}
		atts, err := listAttachmentProject(data)
		if err != nil {
			return err
		}
		payload := map[string]any{"count": len(atts), "attachments": atts}
		return todoListOutput(rt, payload, len(atts))
	},
}

// listAttachmentProject reshapes list_todo_attachment into a clean attachment
// list (attachmentId/fileName/fileSize/fileType) — output-projection fidelity
// for clean output. The list container and field names are probed defensively across
// candidate keys. An unknown container or untargetable attachment fails
// closed instead of being silently omitted from a successful list.
func listAttachmentProject(data map[string]any) ([]map[string]any, error) {
	raw, known := todoListExtract(data, "list", "items", "attachments", "attachmentList")
	if !known {
		return nil, todoProjectionUnknown("待办附件列表响应缺少可识别的列表容器")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, todoProjectionUnknown("待办附件列表包含无法识别的条目")
		}
		row := map[string]any{}
		if attachmentID, ok := todoStableID(m, "attachmentId", "attachment_id", "id", "fileId", "file_id"); ok {
			row["attachmentId"] = attachmentID
		} else {
			return nil, todoProjectionUnknown("待办附件列表条目缺少可用于后续操作的稳定 attachmentId")
		}
		if v := listAttachmentFirst(m, "fileName", "name", "spaceFileName"); v != nil {
			row["fileName"] = v
		}
		if v := listAttachmentFirst(m, "fileSize", "size"); v != nil {
			row["fileSize"] = v
		}
		if v := listAttachmentFirst(m, "fileType", "type"); v != nil {
			row["fileType"] = v
		}
		out = append(out, row)
	}
	return out, nil
}

// listAttachmentFirst returns the first present value among the candidate keys.
func listAttachmentFirst(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

// RemoveAttachment maps helper `remove_todo_attachment`.
// AddComment maps helper `add_todo_comment`.
// ListComment maps helper `list_todo_comment`.
var ListComment = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "todo",
	Command:       "+list-comment",
	Product:       "todo",
	Description:   "查询待办评论列表",
	Intent:        "当你想查看某条待办下的讨论记录、了解协作沟通历史或获取评论 ID 以便删除时使用；输入任务 ID 并可分页，返回该待办的评论列表。",
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "todo",
			Name:           "shortcut_list_comment",
			CanonicalPath:  "todo.shortcut_list_comment",
			CLIPath:        "todo +list-comment",
			PrimaryCLIPath: "todo +list-comment",
		},
		Description: "查询待办评论列表",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "查询待办评论列表",
			UseWhen:      []string{"当你想查看某条待办下的讨论记录、了解协作沟通历史或获取评论 ID 以便删除时使用；输入任务 ID 并可分页，返回该待办的评论列表。"},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples:     []string{"dws todo +list-comment --task-id <taskId>"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "task-id", Type: shortcut.FlagString, Desc: "待办任务 ID", Required: true},
		{Name: "page", Type: shortcut.FlagString, Default: "1", Desc: "页码"},
		{Name: "size", Type: shortcut.FlagString, Default: "20", Desc: "每页数量"},
	},
	Tips: []string{`dws todo +list-comment --task-id <taskId>`},
	Execute: func(rt *shortcut.RuntimeContext) error {
		data, err := rt.CallMCPData("todo", "list_todo_comment", map[string]any{
			"taskId":   rt.Str("task-id"),
			"page":     rt.Str("page"),
			"pageSize": rt.Str("size"),
		})
		if err != nil {
			return err
		}
		comments, err := listCommentProject(data)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"count":            len(comments),
			"comments":         comments,
			"pagination_known": false,
		}
		return todoListOutput(rt, payload, len(comments))
	},
}

// listCommentProject reshapes list_todo_comment into a clean comment list
// (commentId/content/creatorId/createTime) — clean output projection.
// The list container and field names are probed defensively across
// candidate keys. An unknown container or untargetable comment fails closed
// rather than becoming a successful empty or incomplete comment list.
func listCommentProject(data map[string]any) ([]map[string]any, error) {
	raw, known := todoListExtract(data, "list", "items", "comments", "commentList")
	if !known {
		return nil, todoProjectionUnknown("待办评论列表响应缺少可识别的列表容器")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, todoProjectionUnknown("待办评论列表包含无法识别的条目")
		}
		row := map[string]any{}
		if commentID, ok := todoStableID(m, "commentId", "comment_id", "id"); ok {
			row["commentId"] = commentID
		} else {
			return nil, todoProjectionUnknown("待办评论列表条目缺少可用于后续操作的稳定 commentId")
		}
		if v := listCommentFirst(m, "content", "text", "comment"); v != nil {
			row["content"] = v
		}
		if v := listCommentFirst(m, "creatorId", "creator", "userId"); v != nil {
			row["creatorId"] = v
		}
		if v := listCommentFirst(m, "createTime", "createdTime", "gmtCreate"); v != nil {
			row["createTime"] = v
		}
		out = append(out, row)
	}
	return out, nil
}

// listCommentFirst returns the first present value among the candidate keys.
func listCommentFirst(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func todoListOutput(rt *shortcut.RuntimeContext, payload map[string]any, count int) error {
	return rt.OutputResult(payload, output.Success(payload,
		output.WithMeta(&output.Meta{Count: output.NewCount(count)}),
	))
}

// DeleteComment maps helper `delete_todo_comment`.
func init() {
	shortcut.Register(
		GetMyTasks,
		ListSub,
		Get,
		ListAttachment,
		ListComment,
	)
}
