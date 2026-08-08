// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestLifecycleAgentCommandsReachFinalSchema(t *testing.T) {
	canonicals := []string{
		"auth.status",
		"cli.version",
		"profile.list",
		"skill.get",
		"skill.install",
		"skill.search",
	}
	payload := schemaContractPayloadForBoundCanonicals(t, NewRootCommand(), canonicals...)
	if len(payload.Tools) != len(canonicals) {
		t.Fatalf("lifecycle Schema tools = %d, want %d", len(payload.Tools), len(canonicals))
	}

	wantPaths := map[string]string{
		"auth.status":   "auth status",
		"cli.version":   "version",
		"profile.list":  "profile list",
		"skill.get":     "skill get",
		"skill.install": "skill install",
		"skill.search":  "skill search",
	}
	for canonical, path := range wantPaths {
		tool := payload.Tools[canonical]
		if tool == nil {
			t.Errorf("missing lifecycle tool %s", canonical)
			continue
		}
		if got := schemaContractString(tool["primary_cli_path"]); got != path {
			t.Errorf("%s primary_cli_path = %q, want %q", canonical, got, path)
		}
		if tool["availability"] != "available" {
			t.Errorf("%s availability = %#v, want available", canonical, tool["availability"])
		}
	}

	for _, canonical := range []string{"auth.status", "skill.get", "skill.install", "skill.search"} {
		if got := payload.Tools[canonical]["interface_mode"]; got != "composite" {
			t.Errorf("%s interface_mode = %#v, want composite", canonical, got)
		}
	}
	for _, canonical := range []string{"cli.version", "profile.list"} {
		tool := payload.Tools[canonical]
		if got := tool["interface_mode"]; got != "local" {
			t.Errorf("%s interface_mode = %#v, want local", canonical, got)
		}
		if schemaContractString(tool["interface_reason"]) == "" {
			t.Errorf("%s has no reviewed local interface reason", canonical)
		}
	}

	install := payload.Tools["skill.install"]
	if install["effect"] != "write" || install["risk"] != "high" || install["confirmation"] != "user_required" {
		t.Errorf("skill.install safety = effect:%v risk:%v confirmation:%v", install["effect"], install["risk"], install["confirmation"])
	}
	dryRun, _ := install["dry_run"].(map[string]any)
	_, remoteReadsDeclared := dryRun["remote_reads"]
	if dryRun["preview_kind"] != "request" || remoteReadsDeclared {
		t.Errorf("skill.install dry_run = %#v, want request preview without remote reads", install["dry_run"])
	}
	assertSchemaContractPositional(t, install, "skill_id", true)
	assertSchemaContractPositional(t, install, "target", true)

	searchParams := schemaContractMap(payload.Tools["skill.search"]["parameters"])
	if searchParams["query"]["required"] != true {
		t.Errorf("skill.search --query required = %#v, want true", searchParams["query"]["required"])
	}
	if _, ok := searchParams["scopes"]; ok {
		t.Error("skill.search leaked deprecated hidden --scopes into Agent Schema")
	}
	getParams := schemaContractMap(payload.Tools["skill.get"]["parameters"])
	if getParams["skill-id"]["required"] != true {
		t.Errorf("skill.get --skill-id required = %#v, want true", getParams["skill-id"]["required"])
	}
}

func TestSkillSearchJSONPreservesAgentRoutingFacts(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", filepath.Join(t.TempDir(), "config"))
	t.Cleanup(CloseFileLogger)
	testseam.Swap(t, &skillResolveAccessToken, func(context.Context, string, string) (string, error) {
		return "token", nil
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("keyword"); got != "周报" {
			t.Errorf("keyword = %q, want 周报", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"result":[{"skillId":"skill-1","name":"周报","desc":"demo","icon":"i","version":"2.1.0","source":"DingtalkMarket","securityStatus":"passed"}]}`)
	}))
	defer server.Close()
	t.Setenv("DWS_SKILL_API_HOST", server.URL)

	root := NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"skill", "search", "--query", "周报", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("skill search JSON error = %v\nstderr=%s", err, stderr.String())
	}
	var result skillSearchOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode skill search JSON: %v\n%s", err, stdout.String())
	}
	if !result.Success || result.Count != 1 || len(result.Skills) != 1 {
		t.Fatalf("skill search result = %#v", result)
	}
	skill := result.Skills[0]
	if skill.SkillID != "skill-1" || skill.Version != "2.1.0" || skill.Source != "DingtalkMarket" || skill.SecurityStatus != "passed" {
		t.Fatalf("skill routing facts = %#v", skill)
	}
	var raw map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, exists := raw["contract_version"]; exists {
		t.Fatalf("skill search must not emit removed contract_version: %s", stdout.String())
	}
}

func TestSkillGetJSONDoesNotMixProgressText(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", filepath.Join(t.TempDir(), "config"))
	t.Cleanup(CloseFileLogger)
	testseam.Swap(t, &skillLoadAccessToken, func(context.Context) (string, error) { return "token", nil })
	testseam.Swap(t, &skillDownloadToTmp, func(context.Context, string, string) (string, error) {
		return "/tmp/dws-skill-agent", nil
	})

	root := NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"skill", "get", "--skill-id", "skill-1", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("skill get JSON error = %v\nstderr=%s", err, stderr.String())
	}
	var result skillGetOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("skill get stdout is not one JSON document: %v\n%s", err, stdout.String())
	}
	if !result.Success || result.SkillID != "skill-1" || result.TempDir != "/tmp/dws-skill-agent" {
		t.Fatalf("skill get result = %#v", result)
	}
	if strings.Contains(stdout.String(), "下载技能包") {
		t.Fatalf("skill get JSON stdout contains progress text: %q", stdout.String())
	}
}

func TestSkillInstallDryRunHasNoNetworkOrWrite(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", filepath.Join(t.TempDir(), "config"))
	t.Cleanup(CloseFileLogger)
	destination := filepath.Join(t.TempDir(), "skills")
	var tokenCalls, infoCalls, downloadCalls, extractCalls int
	testseam.Swap(t, &skillResolveTargetPath, func(string) (string, error) { return destination, nil })
	testseam.Swap(t, &skillLoadAccessToken, func(context.Context) (string, error) {
		tokenCalls++
		return "", errors.New("must not load credentials")
	})
	testseam.Swap(t, &skillFetchDownloadInfo, func(context.Context, string, string) (*downloadSkillResponse, error) {
		infoCalls++
		return nil, errors.New("must not fetch metadata")
	})
	testseam.Swap(t, &skillDownloadFile, func(context.Context, string, string) (string, error) {
		downloadCalls++
		return "", errors.New("must not download")
	})
	testseam.Swap(t, &skillExtractZip, func(string, string) error {
		extractCalls++
		return errors.New("must not extract")
	})

	root := NewRootCommand()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(""))
	root.SetArgs([]string{"skill", "install", "skill-1", "codex", "--dry-run", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("skill install dry-run error = %v\nstderr=%s", err, stderr.String())
	}
	var result skillInstallOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode skill install dry-run: %v\n%s", err, stdout.String())
	}
	if !result.Success || !result.DryRun || result.Path != destination || result.SkillID != "skill-1" || result.Target != "codex" {
		t.Fatalf("skill install dry-run result = %#v", result)
	}
	if tokenCalls != 0 || infoCalls != 0 || downloadCalls != 0 || extractCalls != 0 {
		t.Fatalf("dry-run side effects: token=%d info=%d download=%d extract=%d", tokenCalls, infoCalls, downloadCalls, extractCalls)
	}
}

func TestSkillInstallRequiresConfirmationBeforeNetwork(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", filepath.Join(t.TempDir(), "config"))
	t.Cleanup(CloseFileLogger)
	destination := filepath.Join(t.TempDir(), "skills")
	var tokenCalls int
	testseam.Swap(t, &skillResolveTargetPath, func(string) (string, error) { return destination, nil })
	testseam.Swap(t, &skillLoadAccessToken, func(context.Context) (string, error) {
		tokenCalls++
		return "token", nil
	})

	root := NewRootCommand()
	root.SetIn(strings.NewReader(""))
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"skill", "install", "skill-1", "codex", "--format", "json"})
	err := root.Execute()
	if err == nil {
		t.Fatal("skill install without --yes or --dry-run unexpectedly succeeded")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Category != apperrors.CategoryValidation || appErr.Reason != "confirmation_required" {
		t.Fatalf("skill install confirmation error = %T %v", err, err)
	}
	if tokenCalls != 0 {
		t.Fatalf("confirmation gate ran after credential/network work: token calls=%d", tokenCalls)
	}
}
