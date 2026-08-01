// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/authsidecar"
)

func TestRunCheckConfigValidatesWithoutCreatingSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets are not available")
	}
	t.Setenv(authsidecar.EnvAuthMode, "")
	directory := shortTempDir(t)
	keyPath := filepath.Join(directory, "sandbox.key")
	if err := os.WriteFile(keyPath, []byte(strings.Repeat("ab", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(directory, "config.json")
	payload := map[string]any{
		"version": 1,
		"bindings": []map[string]any{{
			"key_id": "sandbox-a", "key_file": keyPath, "profile": "corp:user", "policy": "p",
			"enabled": true, "expires_at": time.Now().Add(24 * time.Hour),
		}},
		"policies": []map[string]any{{
			"name": "p", "allowed_origins": []string{"https://mcp.dingtalk.com"},
			"allowed_paths": []string{"/mcp"}, "allowed_tools": []string{"get_document"},
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(directory, "must-not-exist.sock")
	err = run([]string{
		"--check-config", "--config", configPath, "--dws-config-dir", directory,
		"--listen", "unix://" + socketPath,
	})
	if err != nil {
		t.Fatalf("run(--check-config) error = %v", err)
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("--check-config created a socket: %v", err)
	}
}

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
