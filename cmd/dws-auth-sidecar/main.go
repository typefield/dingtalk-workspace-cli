// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/authsidecar"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "dws-auth-sidecar: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if err := authsidecar.ValidateAuthMode(); err != nil {
		return err
	}
	if authsidecar.SidecarModeRequested() {
		return fmt.Errorf("refusing to start with %s=sidecar; the trusted server must not proxy itself", authsidecar.EnvAuthMode)
	}
	defaultDir := filepath.Join(config.DefaultConfigDir(), "sidecar")
	flags := flag.NewFlagSet("dws-auth-sidecar", flag.ContinueOnError)
	configPath := flags.String("config", filepath.Join(defaultDir, "config.json"), "sidecar bindings and policies JSON")
	listenAddress := flags.String("listen", "unix://"+filepath.Join(defaultDir, "dws.sock"), "unix:///path.sock or loopback http://host:port")
	dwsConfigDir := flags.String("dws-config-dir", config.DefaultConfigDir(), "trusted DWS config directory containing profiles")
	checkConfig := flags.Bool("check-config", false, "validate configuration and exit without opening a listener")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(flags.Args(), " "))
	}
	serverConfig, err := authsidecar.LoadServerConfig(*configPath)
	if err != nil {
		return err
	}
	address, err := authsidecar.ParseAddress(*listenAddress)
	if err != nil {
		return fmt.Errorf("listen address: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	resolver, err := authsidecar.NewDWSProfileTokenResolver(*dwsConfigDir, logger)
	if err != nil {
		return err
	}
	handler, err := authsidecar.NewHandler(serverConfig, resolver, nil, logger)
	if err != nil {
		return err
	}
	if *checkConfig {
		fmt.Fprintf(os.Stdout, "sidecar configuration is valid: bindings=%d policies=%d\n", len(serverConfig.Bindings), len(serverConfig.Policies))
		return nil
	}
	listener, cleanup, err := listen(address)
	if err != nil {
		return err
	}
	defer cleanup()
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       65 * time.Second,
		WriteTimeout:      65 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(listener)
	}()
	logger.Info("sidecar_started", "network", address.Network, "address", address.Value)
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown sidecar: %w", err)
		}
		err := <-done
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func listen(address authsidecar.Address) (net.Listener, func(), error) {
	if address.Network == "unix" {
		parent := filepath.Dir(address.Value)
		if err := os.MkdirAll(parent, config.DirPerm); err != nil {
			return nil, nil, fmt.Errorf("create socket directory: %w", err)
		}
		parentInfo, err := os.Stat(parent)
		if err != nil {
			return nil, nil, fmt.Errorf("inspect socket directory: %w", err)
		}
		if !parentInfo.IsDir() || parentInfo.Mode().Perm()&0o077 != 0 {
			return nil, nil, fmt.Errorf("socket directory %q must be an owner-only directory (0700 or stricter)", parent)
		}
		if info, err := os.Lstat(address.Value); err == nil {
			if info.Mode()&os.ModeSocket == 0 {
				return nil, nil, fmt.Errorf("refusing to replace non-socket path %q", address.Value)
			}
			connection, dialErr := net.DialTimeout("unix", address.Value, 250*time.Millisecond)
			if dialErr == nil {
				_ = connection.Close()
				return nil, nil, fmt.Errorf("refusing to replace active unix socket %q", address.Value)
			}
			if err := os.Remove(address.Value); err != nil {
				return nil, nil, fmt.Errorf("remove stale socket: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("inspect socket path: %w", err)
		}
		listener, err := net.Listen("unix", address.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("listen on unix socket: %w", err)
		}
		if err := os.Chmod(address.Value, 0o600); err != nil {
			_ = listener.Close()
			_ = os.Remove(address.Value)
			return nil, nil, fmt.Errorf("protect unix socket: %w", err)
		}
		return listener, func() {
			_ = listener.Close()
			_ = os.Remove(address.Value)
		}, nil
	}
	host, _, err := net.SplitHostPort(address.Value)
	if err != nil {
		return nil, nil, err
	}
	if host != "localhost" {
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if ip == nil || !ip.IsLoopback() {
			return nil, nil, fmt.Errorf("server TCP listen address must be a loopback literal or localhost")
		}
	}
	listener, err := net.Listen("tcp", address.Value)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on loopback TCP: %w", err)
	}
	return listener, func() { _ = listener.Close() }, nil
}
