package smart

import "testing"

func TestPendingApprovalsProjection(t *testing.T) {
	items := shortcutApproveItems(map[string]any{
		"result": map[string]any{
			"instances": []any{
				map[string]any{
					"processInstanceId": "pi-1",
					"title":             "采购审批",
					"originatorName":    "张三",
					"createTime":        "2026-08-08T10:00:00+08:00",
				},
			},
		},
	})
	if len(items) != 1 || shortcutApproveInstanceID(items[0]) != "pi-1" {
		t.Fatalf("pending approval projection = %#v", items)
	}
	if got := pendingApprovalsOriginator(items[0]); got != "张三" {
		t.Fatalf("originator = %q", got)
	}
}
