package smart

import "testing"

func TestDoneApprovalsProjection(t *testing.T) {
	items := shortcutApproveItems(map[string]any{
		"result": map[string]any{
			"instances": []any{
				map[string]any{
					"processInstanceId": "done-1",
					"title":             "报销审批",
					"originatorName":    "李四",
				},
			},
		},
	})
	if len(items) != 1 || shortcutApproveInstanceID(items[0]) != "done-1" {
		t.Fatalf("done approval projection = %#v", items)
	}
	if got := pendingApprovalsOriginator(items[0]); got != "李四" {
		t.Fatalf("originator = %q", got)
	}
}
