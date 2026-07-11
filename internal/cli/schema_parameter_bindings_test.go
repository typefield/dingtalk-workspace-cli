// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import "testing"

func TestSchemaParameterBindingsMatchEmbeddedCatalog(t *testing.T) {
	count := 0
	for canonical, bindings := range runtimeSchemaParameterBindings {
		detail, ok := runtimeEmbeddedSchemaCatalog.Snapshot.Tools[canonical]
		if !ok {
			t.Errorf("binding references unknown canonical path %q", canonical)
			continue
		}
		parameters, _ := detail["parameters"].(map[string]any)
		for flagName, propertyName := range bindings {
			count++
			parameter, _ := parameters[flagName].(map[string]any)
			if parameter == nil {
				t.Errorf("binding %s --%s references an unknown flag", canonical, flagName)
				continue
			}
			if got := schemaString(parameter["property"]); got != propertyName {
				t.Errorf("binding %s --%s property = %q, want %q", canonical, flagName, got, propertyName)
			}
		}
	}
	if count != 308 {
		t.Fatalf("active parameter binding count = %d, want 308", count)
	}
	snapshot := runtimeSchemaParameterBindingSnapshot
	if snapshot.HistoricalBindingCount != 311 || len(snapshot.Migrations) != 5 || len(snapshot.Excluded) != 3 {
		t.Fatalf("binding seed audit = historical:%d migrations:%d excluded:%d",
			snapshot.HistoricalBindingCount, len(snapshot.Migrations), len(snapshot.Excluded))
	}
	if snapshot.SourceCatalogHash != "sha256:6dc63141d35119ff6095d189ef9d8994cd35162ecf9fff9ad23b2ab68e4e2b7f" {
		t.Fatalf("binding source catalog hash = %q", snapshot.SourceCatalogHash)
	}
	if got := runtimeSchemaParameterBindings["calendar.get_calendar"]["id"]; got != "calendarId" {
		t.Fatalf("calendar.get_calendar --id property = %q, want calendarId", got)
	}
	for canonical, flagName := range map[string]string{
		"contact.get_dept_info_by_dept_id":   "dept",
		"contact.get_dept_members_by_deptId": "depts",
		"contact.get_sub_depts_by_dept_id":   "dept",
		"oa.list_pending_approvals":          "limit",
		"oa.list_user_visible_process":       "limit",
	} {
		if runtimeSchemaParameterBindings[canonical][flagName] == "" {
			t.Errorf("migrated binding %s --%s is missing", canonical, flagName)
		}
	}
}
