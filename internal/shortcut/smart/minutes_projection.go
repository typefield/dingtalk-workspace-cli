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
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

// minutesListItems finds the list returned by list_by_keyword_and_time_range.
// In particular, the gateway's documented shape is result.itemList.  Keeping
// this lookup shared prevents the search and "latest" composites from
// disagreeing about whether the same response is empty, usable, or unknown.
func minutesListItems(data map[string]any) ([]any, bool) {
	if data == nil {
		return nil, false
	}
	for _, scope := range minutesListScopes(data) {
		if items, ok := scope.([]any); ok {
			return items, true
		}
		object, ok := scope.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"list", "minutesList", "items", "itemList", "records", "result", "data", "dataList"} {
			if items, ok := object[key].([]any); ok {
				return items, true
			}
		}
	}
	return nil, false
}

func minutesListScopes(data map[string]any) []any {
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

func minutesProjectionUnknown(message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithSubtype(apperrors.SubtypeProjectionUnknown),
		apperrors.WithFailureStage("response_projection"),
		apperrors.WithRetryable(false),
	)
}
