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
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// FindFile: search 钉盘 files by name keyword and project the essentials.
//
// This is a one-step convenience wrapper over the drive MCP tool search_files.
// It mirrors helpers driveSearchCmd exactly for the "仅搜钉盘文件" path:
//   - keyword      ← --query (the free-text file-name search term; MCP arg "keyword")
//   - searchTarget ← "file"  (restrict to 钉盘 files/folders, no doc-space aggregation)
//
// The raw search response is then reduced locally to a compact list of
// {name, type, dentryId, fileSize} per hit. It only accepts recognizable
// containers and targetable rows, so a gateway-shape drift cannot turn into a
// fabricated empty list or a display-only result that an Agent cannot follow.
//
// Read-only: it never mutates anything, it only searches and projects locally.
//
//	dws drive +find-file --query 季度汇报
const findFileIntent = "当你只记得钉盘文件的名字（或其中一部分），想快速按文件名关键词找到它、拿到它的 dentryId 以便后续下载/查看，却不想手动翻目录或写复杂过滤条件时使用；内部调用钉盘的 search_files 工具，把 --query 作为文件名关键词(keyword) 并限定搜索范围为钉盘文件(searchTarget=file)，再在本地把每条命中结果精简为「文件名、类型、dentryId、大小」四个字段后打印。这是纯只读操作，只做搜索与本地投影，不会创建、移动或删除任何文件；只有服务端明确返回文件数组时，空数组才表示本页无命中。"

var FindFile = shortcut.Shortcut{
	OutputRollout: output.RolloutUnifiedActive,
	Service:       "drive",
	Command:       "+find-file",
	Product:       "drive",
	Description:   "按名称关键词搜索钉盘文件并投影关键字段（只读）",
	Intent:        findFileIntent,
	Risk:          shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "drive",
			Name:           "shortcut_find_file",
			CanonicalPath:  "drive.shortcut_find_file",
			CLIPath:        "drive +find-file",
			PrimaryCLIPath: "drive +find-file",
		},
		Description: "按名称关键词搜索钉盘文件并投影关键字段（只读）",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "按名称关键词搜索钉盘文件并投影关键字段（只读）",
			UseWhen:      []string{findFileIntent},
			AvoidWhen:    []string{"需要该 Shortcut 未公开的底层参数、原始响应或不同执行语义时，改用对应原子命令"},
			Examples: []string{
				"dws drive +find-file --query 季度汇报",
				"dws drive +find-file --query 合同",
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "query", Type: shortcut.FlagString, Desc: "文件名关键词（必填）", Required: true},
	},
	Tips: []string{
		`dws drive +find-file --query 季度汇报`,
		`dws drive +find-file --query 合同`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		keyword := strings.TrimSpace(rt.Str("query"))

		// keyword / searchTarget mirror helpers driveSearchCmd (search_files),
		// restricted to 钉盘 files only.
		data, err := rt.CallMCPData("drive", "search_files", map[string]any{
			"keyword":      keyword,
			"searchTarget": "file",
		})
		if err != nil {
			return err
		}

		files, err := shortcutFindFileProject(data)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"count":                len(files),
			"files":                files,
			"index_coverage_known": false,
			"pagination_known":     false,
		}
		return rt.OutputResult(payload, output.Success(payload,
			output.WithMeta(&output.Meta{Count: output.NewCount(len(files))}),
		))
	},
}

// shortcutFindFileProject only accepts a recognized list container and rows
// with stable file IDs. An unfamiliar gateway shape is not evidence of an
// empty search result, and a display-only row cannot safely feed a follow-up
// read or write command.
func shortcutFindFileProject(data map[string]any) ([]map[string]any, error) {
	items, known := shortcutFindFileItems(data)
	if !known {
		return nil, shortcutFindFileProjectionUnknown("无法识别 search_files 返回的文件列表容器")
	}
	files := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			return nil, shortcutFindFileProjectionUnknown("文件搜索结果包含无法识别的条目")
		}
		fileID := shortcutFindFileStr(m, "dentryId", "dentryUuid", "fileId", "nodeId", "id")
		if fileID == "" {
			return nil, shortcutFindFileProjectionUnknown("文件搜索结果缺少可用于后续操作的稳定 dentryId")
		}
		files = append(files, map[string]any{
			"name":     shortcutFindFileStr(m, "name", "fileName", "title", "dentryName"),
			"type":     shortcutFindFileStr(m, "type", "dentryType", "extension", "fileType"),
			"dentryId": fileID,
			"fileSize": shortcutFindFileSize(m),
		})
	}
	return files, nil
}

// shortcutFindFileItems locates only common list containers. It deliberately
// does not scan an arbitrary map for the first array, because a diagnostic or
// unrelated array would otherwise become a fabricated search result.
func shortcutFindFileItems(data map[string]any) ([]any, bool) {
	for _, container := range shortcutFindFileScopes(data) {
		for _, key := range []string{"items", "files", "nodes", "list", "dentries", "results", "records"} {
			if arr, ok := container[key].([]any); ok {
				return arr, true
			}
		}
	}
	return nil, false
}

func shortcutFindFileScopes(data map[string]any) []map[string]any {
	if data == nil {
		return nil
	}
	// Prefer explicit response envelopes over top-level incidental fields. The
	// older adapter unwrapped result/data in sequence, so retain that shape
	// while also accepting either single wrapper.
	scopes := make([]map[string]any, 0, 5)
	for _, outerKey := range []string{"result", "data"} {
		outer, ok := data[outerKey].(map[string]any)
		if !ok {
			continue
		}
		for _, innerKey := range []string{"result", "data"} {
			if inner, ok := outer[innerKey].(map[string]any); ok {
				scopes = append(scopes, inner)
			}
		}
		scopes = append(scopes, outer)
	}
	return append(scopes, data)
}

func shortcutFindFileProjectionUnknown(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}

// shortcutFindFileStr returns the first non-empty string value among the given
// candidate keys.
func shortcutFindFileStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				return s
			}
		}
	}
	return ""
}

// shortcutFindFileSize reads a hit's file size, tolerating numeric (float64) and
// string JSON encodings across the common size key names. Returns nil when
// absent (e.g. folders), so the field is simply omitted-as-null in output.
func shortcutFindFileSize(m map[string]any) any {
	for _, k := range []string{"fileSize", "size"} {
		switch v := m[k].(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case string:
			if strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	return nil
}

func init() {
	shortcut.Register(FindFile)
}
