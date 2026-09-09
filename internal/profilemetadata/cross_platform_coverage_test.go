// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package profilemetadata

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageProfileSelectorGrammarAndNormalize(t *testing.T) {
	corp, user, ok := ParseIdentitySelector(" corp-a : user-a ")
	if !ok || corp != "corp-a" || user != "user-a" {
		t.Fatalf("ParseIdentitySelector = %q %q %v", corp, user, ok)
	}
	for _, selector := range []string{"", "nocolon", ":user", "corp:", " : ", " :user", "corp: "} {
		if _, _, parsed := ParseIdentitySelector(selector); parsed {
			t.Fatalf("ParseIdentitySelector(%q) succeeded", selector)
		}
	}

	profile := Profile{CorpID: "corp-a", UserID: "user-a", Name: "alpha"}
	if ProfileSelector(profile) != "corp-a:user-a" {
		t.Fatalf("ProfileSelector = %q", ProfileSelector(profile))
	}
	if IdentitySelector(" corp ", "") != "corp" || IdentitySelector("", "user") != "" {
		t.Fatal("IdentitySelector org-only and blank-corp cases")
	}

	cfg := &ProfilesConfig{
		Version: 0,
		Profiles: []Profile{
			{CorpID: " ", UserID: "skip"},
			{CorpID: "corp-a", UserID: "user-a", Name: "alpha"},
			{CorpID: "corp-a", UserID: "user-a", Name: "duplicate"},
			{CorpID: "corp-b", UserID: "", Name: "", CorpName: "Beta Org"},
			{CorpID: "corp-c", UserID: "user-c", Name: "taken", CorpName: "Gamma"},
			{CorpID: "corp-d", UserID: "user-d", Name: "", CorpName: "taken"},
		},
		OrgCurrentProfiles: map[string]string{"corp-a": "corp-a:user-a", "missing": "nope"},
		PrimaryProfile:     "gone",
		CurrentProfile:     "corp-a:user-a",
		PreviousProfile:    "gone",
	}
	Normalize(nil)
	Normalize(cfg)
	if cfg.Version != 1 || len(cfg.Profiles) != 4 || cfg.PrimaryProfile != "" || cfg.PreviousProfile != "" {
		t.Fatalf("Normalize = %#v", cfg)
	}
	if cfg.Profiles[1].Name != "Beta Org" {
		t.Fatalf("corp-name fallback = %#v", cfg.Profiles[1])
	}
	if cfg.Profiles[3].Name != "corp-d" {
		t.Fatalf("taken corp-name must not replace name: %#v", cfg.Profiles[3])
	}
	if cfg.OrgCurrentProfiles["corp-a"] != "corp-a:user-a" || len(cfg.OrgCurrentProfiles) != 1 {
		t.Fatalf("OrgCurrentProfiles = %#v", cfg.OrgCurrentProfiles)
	}

	emptyOrg := &ProfilesConfig{OrgCurrentProfiles: map[string]string{"x": "y"}}
	Normalize(emptyOrg)
	if emptyOrg.OrgCurrentProfiles != nil {
		t.Fatalf("invalid org map not cleared: %#v", emptyOrg.OrgCurrentProfiles)
	}
}

func TestCrossPlatformCoverageProfileSelectionAndUnresolvedSlots(t *testing.T) {
	unresolved := UnresolvedProfileSelector(" corp-b ")
	if unresolved == "" || !strings.HasPrefix(unresolved, UnresolvedSelectorPrefix) {
		t.Fatalf("UnresolvedProfileSelector = %q", unresolved)
	}
	if UnresolvedProfileSelector(" ") != "" {
		t.Fatal("blank corp selector")
	}
	corpID, ok := ParseUnresolvedProfileSelector(unresolved)
	if !ok || corpID != "corp-b" {
		t.Fatalf("ParseUnresolvedProfileSelector = %q %v", corpID, ok)
	}
	if _, parsed := ParseUnresolvedProfileSelector("not-legacy"); parsed {
		t.Fatal("non-legacy selector parsed")
	}
	if _, parsed := ParseUnresolvedProfileSelector(UnresolvedSelectorPrefix + "%%%"); parsed {
		t.Fatal("invalid base64 parsed")
	}
	if _, parsed := ParseUnresolvedProfileSelector(UnresolvedSelectorPrefix + base64.RawURLEncoding.EncodeToString([]byte(" "))); parsed {
		t.Fatal("blank decoded corp parsed")
	}

	cfg := &ProfilesConfig{Profiles: []Profile{
		{Name: "alpha", CorpID: "corp-a", UserID: "user-a", UserName: "Alice", CorpName: "Alpha Org"},
		{Name: "beta-slot", CorpID: "corp-b", UserID: "", CorpName: "Beta Org"},
		{Name: "beta-exact", CorpID: "corp-b", UserID: "user-b", UserName: "Bob"},
		{Name: "shared", CorpID: "corp-c", UserID: "user-c1", CorpName: "Shared"},
		{Name: "shared", CorpID: "corp-d", UserID: "user-d1", CorpName: "Shared"},
		{Name: "dup-name", CorpID: "corp-e", UserID: "user-e1"},
		{Name: "dup-name", CorpID: "corp-e", UserID: "user-e2"},
		{Name: "local-only", CorpID: "corp-f", UserID: "", CorpName: "Local"},
		{Name: "local-only", CorpID: "corp-g", UserID: ""},
	}}
	if ExactProfileSelectorForCorp(cfg, "corp-a", "corp-a:user-a") != "corp-a:user-a" {
		t.Fatal("exact corp selector")
	}
	if ExactProfileSelectorForCorp(cfg, "corp-a", "corp-b:user-a") != "" || FindExactProfile(cfg, "missing", "user") != nil {
		t.Fatal("mismatched exact selector")
	}
	if ExactProfileSelectorForCorp(cfg, "corp-a", "corp-a:missing") != "" {
		t.Fatal("missing exact account")
	}
	if ProfilesForCorpID(nil, "corp-a") != nil {
		t.Fatal("nil cfg corp list")
	}
	if ProfileIndexByIdentity(nil, "corp-a", "user-a") != -1 {
		t.Fatal("nil cfg index")
	}

	if !LocalProfileSelectorIsSafe(cfg, &cfg.Profiles[0], "alpha") {
		t.Fatal("unique local name should be safe")
	}
	if LocalProfileSelectorIsSafe(nil, &cfg.Profiles[0], "alpha") || LocalProfileSelectorIsSafe(cfg, nil, "alpha") {
		t.Fatal("nil inputs must be unsafe")
	}
	if LocalProfileSelectorIsSafe(cfg, &cfg.Profiles[0], "corp-a") || LocalProfileSelectorIsSafe(cfg, &cfg.Profiles[0], "Alpha Org") {
		t.Fatal("org grammar capture must be unsafe")
	}
	if LocalProfileSelectorIsSafe(cfg, &cfg.Profiles[0], "dup-name") || LocalProfileSelectorIsSafe(cfg, &cfg.Profiles[0], "corp-a:user-a") {
		t.Fatal("duplicate or identity local names must be unsafe")
	}

	if StoredProfileSelector(nil, nil) != "" {
		t.Fatal("nil stored selector")
	}
	if StoredProfileSelector(nil, &Profile{CorpID: "corp-z"}) != "corp-z" {
		t.Fatal("nil cfg org selector")
	}
	if StoredProfileSelector(cfg, &cfg.Profiles[1]) == "" {
		t.Fatal("unresolved historical selector")
	}
	if candidates := ProfileSelectorCandidates([]*Profile{nil, &cfg.Profiles[0]}); len(candidates) != 1 {
		t.Fatalf("candidates = %#v", candidates)
	}

	if UnresolvedProfileForCorp(nil, "corp-b") != nil || UnresolvedProfileForLocalName(nil, "x") != nil {
		t.Fatal("nil cfg unresolved lookups")
	}
	if UnresolvedProfileForCorp(cfg, "corp-b") == nil || UnresolvedProfileForLocalName(cfg, "") != nil {
		t.Fatal("unresolved lookups")
	}
	if UnresolvedProfileForLocalName(cfg, "local-only") != nil {
		t.Fatal("ambiguous unresolved local name")
	}

	if !ProfileSelectorReferenceExists(cfg, "corp-a:user-a") || !ProfileSelectorReferenceExists(cfg, "alpha") {
		t.Fatal("existing references")
	}
	if ProfileSelectorReferenceExists(nil, "alpha") || ProfileSelectorReferenceExists(cfg, "") {
		t.Fatal("empty reference")
	}
	if ProfileSelectorReferenceExists(cfg, "corp-a:missing") {
		t.Fatal("missing exact reference")
	}
	if !ProfileSelectorReferenceExists(cfg, unresolved) {
		t.Fatal("unresolved reference")
	}
	if !ProfileSelectorReferenceExists(cfg, "Alice") && !ProfileSelectorReferenceExists(cfg, "corp-a:Alice") {
		t.Fatal("username/org compound reference")
	}

	if _, err := ResolveOrganizationCorpID(nil, "corp-a"); err != nil {
		t.Fatal(err)
	}
	if corp, err := ResolveOrganizationCorpID(cfg, "Alpha Org"); err != nil || corp != "corp-a" {
		t.Fatalf("org name = %q %v", corp, err)
	}
	if _, err := ResolveOrganizationCorpID(cfg, "Shared"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous org name: %v", err)
	}

	selected, _, err := ResolveOrganizationDefault(cfg, "corp-a", "corp-a", ProfilesForCorpID(cfg, "corp-a"))
	if err != nil || selected == nil || selected.UserID != "user-a" {
		t.Fatalf("org default = %#v %v", selected, err)
	}
	if _, _, err := ResolveOrganizationDefault(cfg, "missing", "missing", nil); err == nil {
		t.Fatal("missing org default")
	}
	cfg.OrgCurrentProfiles = map[string]string{"corp-b": "corp-b:user-b"}
	selected, _, err = ResolveOrganizationDefault(cfg, "corp-b", "corp-b", ProfilesForCorpID(cfg, "corp-b"))
	if err != nil || selected == nil || selected.UserID != "user-b" {
		t.Fatalf("org current exact = %#v %v", selected, err)
	}
	cfg.OrgCurrentProfiles = nil
	selected, _, err = ResolveOrganizationDefault(cfg, "corp-b", "corp-b", ProfilesForCorpID(cfg, "corp-b"))
	if err != nil || selected == nil || selected.UserID != "" {
		t.Fatalf("unresolved org default = %#v %v", selected, err)
	}
	if _, _, err := ResolveOrganizationDefault(cfg, "corp-e", "corp-e", ProfilesForCorpID(cfg, "corp-e")); err == nil {
		t.Fatal("multi-account org without current")
	}

	if _, _, err := ResolveSelection("", nil, "alpha"); err == nil {
		t.Fatal("nil cfg selection")
	}
	if _, _, err := ResolveSelection("", cfg, " "); err == nil {
		t.Fatal("empty selector")
	}
	got, historical, err := ResolveSelection("", cfg, unresolved)
	if err != nil || !historical || got == nil || got.CorpID != "corp-b" {
		t.Fatalf("unresolved selection = %#v %v %v", got, historical, err)
	}
	if _, _, err := ResolveSelection("", cfg, UnresolvedProfileSelector("missing")); err == nil {
		t.Fatal("missing unresolved selection")
	}
	got, historical, err = ResolveSelection("", cfg, "corp-a:user-a")
	if err != nil || !historical || got.UserID != "user-a" {
		t.Fatalf("exact selection = %#v %v %v", got, historical, err)
	}
	got, _, err = ResolveSelection("", cfg, "corp-a:Alice")
	if err != nil || got == nil || got.UserID != "user-a" {
		t.Fatalf("username selection = %#v %v", got, err)
	}
	if _, _, err := ResolveSelection("", cfg, "missing:user"); err == nil {
		t.Fatal("missing org compound")
	}
	if _, _, err := ResolveSelection("", cfg, "Shared:user"); err == nil {
		t.Fatal("ambiguous org compound")
	}
	if _, _, err := ResolveSelection("", cfg, "corp-a:missing"); err == nil {
		t.Fatal("missing account")
	}
	got, _, err = ResolveSelection("", cfg, "corp-a")
	if err != nil || got == nil {
		t.Fatalf("org selector = %#v %v", got, err)
	}
	got, _, err = ResolveSelection("", cfg, "Alpha Org")
	if err != nil || got == nil {
		t.Fatalf("org name selector = %#v %v", got, err)
	}
	if _, _, err := ResolveSelection("", cfg, "Shared"); err == nil {
		t.Fatal("ambiguous org name selector")
	}
	got, historical, err = ResolveSelection("", cfg, "alpha")
	if err != nil || !historical || got.Name != "alpha" {
		t.Fatalf("local name = %#v %v %v", got, historical, err)
	}
	if _, _, err := ResolveSelection("", cfg, "dup-name"); err == nil {
		t.Fatal("ambiguous local name")
	}
	if _, _, err := ResolveSelection("", cfg, "missing-profile"); err == nil {
		t.Fatal("missing local name")
	}
}

func TestCrossPlatformCoverageResolveReadOnlyMetadata(t *testing.T) {
	dir := t.TempDir()
	if got, err := ResolveReadOnly(dir, ""); err != nil || got != nil {
		t.Fatalf("missing file = %#v %v", got, err)
	}
	if _, err := ResolveReadOnlyWithReader(dir, "", func(string) ([]byte, error) { return nil, errors.New("boom") }); err == nil {
		t.Fatal("read error")
	}
	if _, err := ResolveReadOnlyWithReader(dir, "", func(string) ([]byte, error) { return []byte("{"), nil }); err == nil {
		t.Fatal("corrupt json")
	}
	if _, err := ResolveReadOnlyWithReader(dir, "", func(string) ([]byte, error) { return []byte(`{"version":999}`), nil }); err == nil {
		t.Fatal("future version")
	}
	if got, err := ResolveReadOnlyWithReader(dir, "", func(string) ([]byte, error) {
		return []byte(`{"version":3,"profiles":[{"name":"alpha","corpId":"corp-a","userId":"user-a"}]}`), nil
	}); err != nil || got != nil {
		t.Fatalf("no current = %#v %v", got, err)
	}

	path := filepath.Join(dir, "profiles.json")
	if err := os.WriteFile(path, []byte(`{"version":3,"currentProfile":"corp-a:user-a","profiles":[{"name":"alpha","corpId":"corp-a","userId":"user-a","userName":"Alice"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveReadOnly(dir, "")
	if err != nil || got == nil || got.UserID != "user-a" || got.UserName != "Alice" || got.CorpID != "corp-a" {
		t.Fatalf("current = %#v %v", got, err)
	}
	got, err = ResolveReadOnly(dir, "alpha")
	if err != nil || got == nil || got.UserID != "user-a" {
		t.Fatalf("explicit = %#v %v", got, err)
	}
	if _, err := ResolveReadOnly(dir, "missing"); err == nil {
		t.Fatal("unknown selector")
	}
}
