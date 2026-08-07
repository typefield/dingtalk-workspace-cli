// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package attendance

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestAttendanceRegisteredShortcutsHaveNoShadowSurface(t *testing.T) {
	const wantRegistered = 33
	seen := 0
	for _, item := range shortcut.All() {
		if item.Service != "attendance" {
			continue
		}
		seen++
		if item.Hidden {
			t.Errorf("%s remains executable but hidden", item.Command)
		}
		if !shortcut.InPublicCatalog(item.Service, item.Command) {
			t.Errorf("%s is missing from the public catalog", item.Command)
		}
		if item.Contract.Empty() {
			t.Errorf("%s has no final Agent contract", item.Command)
		}
		safety := shortcut.EffectiveSafety(item)
		if item.Risk == shortcut.RiskRead {
			if safety.Effect != "read" || safety.Confirmation != "not_required" {
				t.Errorf("%s read safety = %#v", item.Command, safety)
			}
			continue
		}
		if safety.Effect == "read" || safety.Confirmation != "user_required" {
			t.Errorf("%s write safety = %#v", item.Command, safety)
		}
	}
	if seen != wantRegistered {
		t.Fatalf("registered attendance shortcuts = %d, want %d", seen, wantRegistered)
	}
}
