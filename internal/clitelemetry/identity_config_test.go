// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package clitelemetry

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/profilemetadata"
)

func TestCrossPlatformCoverageIdentityProjectionConfigurationAndRenderedError(t *testing.T) {
	if IdentityFromProfile(nil) != (Identity{}) {
		t.Fatal("nil profile")
	}
	got := IdentityFromProfile(&profilemetadata.ProfileMetadata{UserID: " u ", UserName: " n ", CorpID: " c "})
	if got != (Identity{UserID: "u", UserName: "n", CorpID: "c"}) {
		t.Fatalf("IdentityFromProfile = %#v", got)
	}
	if (RenderedError{}).Error() != "" {
		t.Fatal("RenderedError must stay empty")
	}

	command := "schema list"
	message := "boom"
	cfg := Configuration("dev", Identity{UserID: "u", CorpID: "c"}, &command, &message)
	fields := cfg.ExtraFields()
	if fields["c9"] != "schema list" || fields["c10"] != "c" || fields["c5"] != "boom" {
		t.Fatalf("extra fields = %#v", fields)
	}
	empty := ""
	if fields := Configuration("dev", Identity{}, &command, &empty).ExtraFields(); len(fields) != 1 || fields["c9"] != "schema list" {
		t.Fatalf("empty corp/error fields = %#v", fields)
	}

	called := false
	Run(cfg, func() error {
		called = true
		return nil
	}, func(error) int { return 0 })
	if !called {
		t.Fatal("Run did not execute")
	}
}
