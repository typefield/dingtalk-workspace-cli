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
	"strings"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// FindRoom: list meeting rooms that are AVAILABLE within a given time window.
//
// Steps:
//
//  1. parse the ISO8601 --start/--end into epoch millis, exactly as the
//     calendar room-search tool expects (startTime/endTime are int64 millis,
//     mirroring helpers.roomSearch Mode 2);
//
//  2. call query_available_meeting_room with {startTime, endTime} — the same
//     MCP tool + parameter names used by `calendar room search` availability
//     mode (see helpers/calendar.go callMeetingRoomSearchResult);
//
//  3. defensively project each returned room to {roomId, name, capacity}; an
//     unknown list shape or untargetable room fails closed instead of becoming
//     a successful empty availability result.
//
// Read-only: it only queries availability, it never books or mutates anything.
//
//	dws calendar +find-room --start "2026-03-10T14:00:00+08:00" --end "2026-03-10T15:00:00+08:00"
var FindRoom = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "calendar",
	Command:       "+find-room",
	Product:       "calendar",
	Description:   "查询指定时间段内所有可用的会议室",
	Intent: "当你想在某个明确的时间段内找出所有当前可预定的空闲会议室（比如临时要约线下会、先看看哪些会议室有空）时使用；" +
		"内部把你给的 ISO8601 起止时间解析成毫秒时间戳，调用会议室可用性查询，只返回该时间范围内可预定的会议室，" +
		"并投影出每个会议室的 roomId、名称与容量，方便你随后用来预订。" +
		"这是纯只读操作，只做可用性查询，不会预订或改动任何会议室或日程；" +
		"注意大部分会议室仅在工作时间可用，非工作时间可能查不到结果，且 start 需为未来时间。",
	Risk: shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "calendar",
			Name:           "shortcut_find_room",
			CanonicalPath:  "calendar.shortcut_find_room",
			CLIPath:        "calendar +find-room",
			PrimaryCLIPath: "calendar +find-room",
		},
		Description: "查询指定时间段内所有可用的会议室",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns time validation, availability lookup and stable room projection; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "查询指定时间段内所有可用的会议室",
			UseWhen:      []string{"当你想在某个明确的时间段内找出所有当前可预定的空闲会议室（比如临时要约线下会、先看看哪些会议室有空）时使用；内部把你给的 ISO8601 起止时间解析成毫秒时间戳，调用会议室可用性查询，只返回该时间范围内可预定的会议室，并投影出每个会议室的 roomId、名称与容量，方便你随后用来预订。这是纯只读操作，只做可用性查询，不会预订或改动任何会议室或日程；注意大部分会议室仅在工作时间可用，非工作时间可能查不到结果，且 start 需为未来时间。"},
			AvoidWhen:    []string{"需要直接预订、修改会议室或读取原始响应时，改用对应原子命令"},
			Examples:     []string{"dws calendar +find-room --start 2026-03-10T14:00:00+08:00 --end 2026-03-10T15:00:00+08:00"},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "start", Type: shortcut.FlagString, Desc: "开始时间（ISO8601，如 2026-03-10T14:00:00+08:00，需为未来时间）", Required: true},
		{Name: "end", Type: shortcut.FlagString, Desc: "结束时间（ISO8601，如 2026-03-10T15:00:00+08:00）", Required: true},
	},
	Tips: []string{
		`dws calendar +find-room --start "2026-03-10T14:00:00+08:00" --end "2026-03-10T15:00:00+08:00"`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		// Step 1 — parse the ISO8601 range to epoch millis. The room-search
		// availability tool expects startTime/endTime as int64 millis, exactly
		// like helpers.roomSearch (parseISOTimeToMillis).
		startMillis, err := findRoomParseMillis("start", rt.Str("start"))
		if err != nil {
			return err
		}
		endMillis, err := findRoomParseMillis("end", rt.Str("end"))
		if err != nil {
			return err
		}
		if endMillis <= startMillis {
			return apperrors.NewValidation("--end 必须晚于 --start")
		}

		// Step 2 — query available meeting rooms over the range. tool name +
		// param names copied verbatim from helpers callMeetingRoomSearchResult
		// (query_available_meeting_room) / roomSearch Mode 2 toolArgs.
		data, err := rt.CallMCPData("calendar", "query_available_meeting_room", map[string]any{
			"startTime": startMillis,
			"endTime":   endMillis,
		})
		if err != nil {
			return err
		}

		// Step 3 — project each returned room to {roomId, name, capacity}.
		rooms, err := findRoomProject(data)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"count":            len(rooms),
			"rooms":            rooms,
			"pagination_known": false,
		}
		return rt.OutputResult(payload, output.Success(payload,
			output.WithMeta(&output.Meta{Count: output.NewCount(len(rooms))}),
		))
	},
}

// findRoomParseMillis parses an ISO8601 timestamp into epoch milliseconds,
// returning a clear validation error naming the offending flag.
func findRoomParseMillis(flag, value string) (int64, error) {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return 0, apperrors.NewValidation(fmt.Sprintf(
			"--%s 时间格式无效：%q，请使用 ISO8601（如 2026-03-10T14:00:00+08:00）", flag, value))
	}
	return t.UnixMilli(), nil
}

// findRoomProject distinguishes a known empty availability response from an
// unknown gateway shape. Every non-empty row must have a usable room ID so an
// Agent can safely select it for a later booking command.
func findRoomProject(data map[string]any) ([]map[string]any, error) {
	items, known := findRoomItems(data)
	if !known {
		return nil, findRoomProjectionUnknown("无法识别 query_available_meeting_room 返回的会议室列表容器")
	}
	rooms := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		room, ok := raw.(map[string]any)
		if !ok {
			return nil, findRoomProjectionUnknown("会议室列表包含无法识别的条目")
		}
		roomID := findRoomFirstString(room, "roomId", "roomID", "id", "room_id")
		if roomID == "" {
			return nil, findRoomProjectionUnknown("会议室条目缺少可用于后续预订的稳定 roomId")
		}
		rooms = append(rooms, map[string]any{
			"roomId":   roomID,
			"name":     findRoomFirstString(room, "roomName", "name", "title", "displayName"),
			"capacity": findRoomCapacity(room),
		})
	}
	return rooms, nil
}

func findRoomItems(data map[string]any) ([]any, bool) {
	for _, container := range findRoomScopes(data) {
		if arr, ok := container.([]any); ok {
			return arr, true
		}
		object, ok := container.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"rooms", "roomList", "meetingRooms", "list", "items", "records"} {
			if arr, ok := object[key].([]any); ok {
				return arr, true
			}
		}
	}
	return nil, false
}

func findRoomScopes(data map[string]any) []any {
	if data == nil {
		return nil
	}
	scopes := make([]any, 0, 5)
	for _, outerKey := range []string{"result", "data"} {
		outer, ok := data[outerKey]
		if !ok {
			continue
		}
		if object, ok := outer.(map[string]any); ok {
			for _, innerKey := range []string{"result", "data"} {
				if inner, ok := object[innerKey]; ok {
					scopes = append(scopes, inner)
				}
			}
		}
		scopes = append(scopes, outer)
	}
	return append(scopes, data)
}

func findRoomProjectionUnknown(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

// findRoomFirstString returns the first non-empty string value among the given
// candidate keys.
func findRoomFirstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				return s
			}
		}
	}
	return ""
}

// findRoomCapacity reads a room's seating capacity, tolerating numeric
// (float64/int) and string JSON encodings across a few common key names.
func findRoomCapacity(m map[string]any) any {
	for _, k := range []string{"capacity", "maxCapacity", "seatCount", "seats"} {
		switch v := m[k].(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case string:
			if v != "" {
				return v
			}
		}
	}
	return nil
}

func init() {
	shortcut.Register(FindRoom)
}
