// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/authsidecar"
)

func TestUnixListenerRequiresPrivateDirectoryAndCleansSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not available")
	}
	parent := filepath.Join(shortTempDir(t), "sidecar")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	address := authsidecar.Address{Network: "unix", Value: filepath.Join(parent, "dws.sock")}
	if _, _, err := listen(address); err == nil {
		t.Fatal("listen() accepted a group/other-accessible socket directory")
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	listener, cleanup, err := listen(address)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(address.Value); err != nil {
		t.Fatalf("socket was not created: %v", err)
	}
	cleanup()
	if _, err := os.Stat(address.Value); !os.IsNotExist(err) {
		t.Fatalf("socket still exists after cleanup: %v", err)
	}
	_ = listener.Close()
}

func TestUnixListenerRefusesRegularFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not available")
	}
	parent := filepath.Join(shortTempDir(t), "sidecar")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "dws.sock")
	if err := os.WriteFile(path, []byte("do not replace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := listen(authsidecar.Address{Network: "unix", Value: path}); err == nil {
		t.Fatal("listen() replaced a regular file")
	}
}

func TestUnixListenerRefusesActiveSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not available")
	}
	parent := filepath.Join(shortTempDir(t), "sidecar")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	address := authsidecar.Address{Network: "unix", Value: filepath.Join(parent, "dws.sock")}
	listener, cleanup, err := listen(address)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer listener.Close()

	if _, _, err := listen(address); err == nil {
		t.Fatal("listen() replaced an active unix socket")
	}
	connection, err := net.DialTimeout("unix", address.Value, 250*time.Millisecond)
	if err != nil {
		t.Fatalf("original unix socket is no longer reachable: %v", err)
	}
	_ = connection.Close()
}

func shortTempDir(t *testing.T) string {
	t.Helper()
	directory, err := os.MkdirTemp("/tmp", "dws-sc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	return directory
}
