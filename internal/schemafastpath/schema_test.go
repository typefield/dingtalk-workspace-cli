// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package schemafastpath

import "testing"

func TestCrossPlatformCoverageSharedSchemaRequestRouting(t *testing.T) {
	for _, args := range [][]string{
		{"dws", "schema"}, {"dws", "schema", "list"}, {"dws", "schema", "calendar", "--compact"},
		{"dws", "schema", "--cli-path=calendar event create", "--compact=false", "--format=json"},
		{"dws", "schema", "-f", "json", "calendar/create_calendar_event"},
	} {
		if _, ok := parseSchemaRequest(args); !ok {
			t.Errorf("supported args declined: %q", args)
		}
	}
	for _, args := range [][]string{
		{"dws", "--format=json", "schema"}, {"dws", "schem", "calendar"}, {"dws", "schema", "--all"},
		{"dws", "schema", "--fields", "title"}, {"dws", "schema", "--jq", "."}, {"dws", "schema", "--output", "file"},
		{"dws", "schema", "--help"}, {"dws", "schema", "--compact", "false"}, {"dws", "schema", "--compact=1"},
		{"dws", "schema", "--compact", "--compact=false"}, {"dws", "schema", "--format", "table"},
		{"dws", "schema", "--format"}, {"dws", "schema", "--cli-path="}, {"dws", "schema", "--cli-path", "list", "list"},
		{"dws", "schema", "calendar", "event"}, {"dws", "schema", "--", "calendar"},
	} {
		if _, ok := parseSchemaRequest(args); ok {
			t.Errorf("ambiguous args handled: %q", args)
		}
	}
}
