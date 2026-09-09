// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/executor"
	"github.com/spf13/cobra"
)

type namedStubHandler struct {
	name string
	cmd  *cobra.Command
}

func (h namedStubHandler) Name() string { return h.name }
func (h namedStubHandler) Command(executor.Runner) *cobra.Command {
	return h.cmd
}

func TestCrossPlatformCoverageRegisterPublicNamedAndBuildNameGuard(t *testing.T) {
	if (Manifest{Vendor: " dingtalk ", Name: " oa "}).FullName() != "dingtalk/oa" {
		t.Fatal("Manifest.FullName trims vendor and name")
	}

	first := buildCommands([]registeredFactory{{
		name: "beta",
		factory: func() Handler {
			return namedStubHandler{name: "beta", cmd: &cobra.Command{Use: "beta"}}
		},
	}, {
		name: "alpha",
		factory: func() Handler {
			return namedStubHandler{name: "alpha", cmd: &cobra.Command{Use: "alpha"}}
		},
	}}, nil)
	if len(first) != 2 || first[0].Name() != "alpha" || first[1].Name() != "beta" {
		t.Fatalf("sorted commands = %#v", first)
	}

	defer func() {
		got := recover()
		if got == nil {
			t.Fatal("mismatched factory name must panic")
		}
		if msg, ok := got.(string); !ok || msg == "" {
			t.Fatalf("panic = %#v", got)
		}
	}()
	buildCommands([]registeredFactory{{
		name: "want",
		factory: func() Handler {
			return namedStubHandler{name: "want", cmd: &cobra.Command{Use: "got"}}
		},
	}}, nil)
}

func TestCrossPlatformCoverageBuildCommandsRejectsNilCommand(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil command must panic")
		}
	}()
	buildCommands([]registeredFactory{{
		name: "leaf",
		factory: func() Handler {
			return namedStubHandler{name: "leaf"}
		},
	}}, nil)
}
