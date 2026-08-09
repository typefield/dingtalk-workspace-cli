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
	"math"
	"strconv"
	"strings"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// LatestMinutes: fetch the details of MY most recent minutes (听记) in one step.
//
// Steps:
//  1. list my minutes via list_by_keyword_and_time_range (belongingConditionId
//     = "created"), optionally filtered by --keyword;
//  2. pick the newest entry (largest create time, falling back to the first
//     item) and read its taskUuid;
//  3. print that minute's basic info via get_minutes_basic_info.
//
// If the list is empty it reports "暂无妙记" instead of failing obscurely.
//
//	dws minutes +latest-minutes
//	dws minutes +latest-minutes --keyword 周会
var LatestMinutes = shortcut.Shortcut{
	Service:     "minutes",
	Command:     "+latest-minutes",
	Product:     "minutes",
	Description: "取我最新的一条妙记（听记）详情",
	Intent: "当你只想快速看回自己最近的一条会议听记，却不想先翻列表、复制 taskUuid 再查详情时使用；" +
		"内部先列出你创建的听记（可用 --keyword 缩小范围），自动挑出最新的一条，再拉取它的基础信息（标题、创建人、时间、访问链接等）。" +
		"这是只读操作，不会修改任何听记；若你名下没有任何听记则提示「暂无妙记」。",
	Risk: shortcut.RiskRead,
	Flags: []shortcut.Flag{
		{Name: "keyword", Type: shortcut.FlagString, Desc: "按关键字过滤听记（可选）", Required: false},
	},
	Tips: []string{
		`dws minutes +latest-minutes`,
		`dws minutes +latest-minutes --keyword 周会`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		// Step 1 — list my minutes (newest first). belongingConditionId /
		// maxResults / keyword mirror helpers.callListByKeywordRange.
		listArgs := map[string]any{
			"belongingConditionId": "created",
			"maxResults":           float64(20),
		}
		if kw := rt.Str("keyword"); kw != "" {
			listArgs["keyword"] = kw
		}
		data, err := rt.CallMCPData("minutes", "list_by_keyword_and_time_range", listArgs)
		if err != nil {
			return err
		}

		// Step 2 — locate the newest minute's taskUuid. Unknown or
		// untargetable list responses must not become "暂无妙记".
		taskUUID, err := latestMinutesTaskUUID(data)
		if err != nil {
			return err
		}
		if taskUUID == "" {
			return apperrors.NewValidation("暂无妙记")
		}

		// Step 3 — print its basic info (taskUuid param mirrors helper).
		return rt.CallMCP("get_minutes_basic_info", map[string]any{
			"taskUuid": taskUUID,
		})
	},
}

// latestMinutesTaskUUID picks the newest minute's taskUuid: the item with the
// largest numeric create time, falling back to the first item that carries a
// taskUuid (lists come back newest-first). It distinguishes an explicit empty
// list from an unrecognized container or an item that cannot be targeted.
func latestMinutesTaskUUID(data map[string]any) (string, error) {
	raw, known := minutesListItems(data)
	if !known {
		return "", minutesProjectionUnknown("无法识别 list_by_keyword_and_time_range 返回的妙记列表容器")
	}
	if len(raw) == 0 {
		return "", nil
	}

	best := ""
	var bestTime int64
	haveTime := false
	timedCount := 0
	firstUUID := ""

	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			return "", minutesProjectionUnknown("妙记列表包含无法识别的条目")
		}
		uuid := latestMinutesUUID(m)
		if uuid == "" {
			return "", minutesProjectionUnknown("妙记条目缺少可用于读取详情的稳定 taskUuid")
		}
		if firstUUID == "" {
			firstUUID = uuid
		}
		t, hasTime, timeErr := latestMinutesCreateTime(m)
		if timeErr != nil {
			return "", minutesProjectionUnknown(timeErr.Error())
		}
		if hasTime {
			timedCount++
			if !haveTime || t > bestTime {
				haveTime = true
				bestTime = t
				best = uuid
			}
		}
	}
	if timedCount > 0 && timedCount != len(raw) {
		return "", minutesProjectionUnknown("妙记列表只有部分条目带可比较时间，无法可靠选择最新听记")
	}

	if haveTime && best != "" {
		return best, nil
	}
	return firstUUID, nil
}

func latestMinutesUUID(m map[string]any) string {
	// A generic id can be the Minutes document identifier rather than the
	// asynchronous task identifier accepted by the detail endpoints. Treating
	// it as taskUuid would select a different resource and turn an ambiguous
	// list response into a misleading successful composite read.
	for _, key := range []string{"taskUuid", "taskUUID", "uuid"} {
		if s, ok := m[key].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func latestMinutesCreateTime(m map[string]any) (int64, bool, error) {
	for _, key := range []string{"createTime", "gmtCreate", "startTime", "createTimeStart"} {
		value, present := m[key]
		if !present {
			continue
		}
		switch typed := value.(type) {
		case int:
			if typed > 0 {
				return int64(typed), true, nil
			}
		case int64:
			if typed > 0 {
				return typed, true, nil
			}
		case float64:
			if typed > 0 && !math.IsInf(typed, 0) && !math.IsNaN(typed) && math.Trunc(typed) == typed && typed < float64(math.MaxInt64) {
				return int64(typed), true, nil
			}
		case string:
			parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
			if err == nil && parsed > 0 {
				return parsed, true, nil
			}
		}
		return 0, false, fmt.Errorf("妙记条目的 %s 不是有效正整数时间戳", key)
	}
	return 0, false, nil
}

func init() {
	shortcut.Register(LatestMinutes)
}
