//go:build authsidecar

// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package app

import (
	"context"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/authsidecar"
)

func TestSidecarTokenManagerReturnsSentinelWithoutCredentialStorage(t *testing.T) {
	t.Setenv(authsidecar.EnvAuthMode, authsidecar.AuthModeSidecar)
	snapshot, err := NewTokenManager().Get(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.AccessToken != authsidecar.SentinelUserToken || snapshot.Source != "sidecar" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if _, err := NewTokenManager().Get(context.Background(), t.TempDir(), "real-token"); err == nil {
		t.Fatal("TokenManager accepted an explicit token in sidecar mode")
	}
}
