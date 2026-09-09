// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package clisignal

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestCrossPlatformCoverageInstallAndNilSignalAndEscalationSeam(t *testing.T) {
	ctx, state, stop := Install(context.Background(), func() bool { return false })
	t.Cleanup(stop)
	if ctx == nil || state == nil {
		t.Fatal("Install returned nil")
	}
	stop()

	signals := make(chan os.Signal, 2)
	ctx, _, stop = Manage(context.Background(), nil, signals, func() {}, func(os.Signal) {})
	t.Cleanup(stop)
	signals <- nil
	select {
	case <-ctx.Done():
		t.Fatal("nil signal cancelled execution")
	case <-time.After(50 * time.Millisecond):
	}
	stop()

	testseam.Swap(t, &escalateFindProcess, func(int) (*os.Process, error) {
		return nil, os.ErrNotExist
	})
	exited := 0
	testseam.Swap(t, &escalateExitProcess, func(code int) { exited = code })
	Escalate(syscall.SIGTERM)
	if exited != 143 {
		t.Fatalf("Escalate fallback exit = %d", exited)
	}
}

func TestCrossPlatformCoverageRedeliverSuccessfulSignalDoesNotExit(t *testing.T) {
	exited := false
	// Signal 0 is the existence probe: it exercises process.Signal success
	// without delivering a terminating signal to this test process.
	Redeliver(syscall.Signal(0), os.FindProcess, func(int) { exited = true })
	if exited {
		t.Fatal("successful Signal must not fall back to exit")
	}
}
