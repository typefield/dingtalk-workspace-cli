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

package helpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeRoleFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestLoadRoleConfig_Full(t *testing.T) {
	dir := t.TempDir()
	body := `
name: 人事助理
client_id: dinghr0001
owner_user_id: "012345678901"
confirm_policy: remember
persona: |
  You are the HR assistant.
knowledge_sources:
  - ./knowledge/hr
  - wiki:1234567890
  - doc:abcDEF
allowed_scopes:
  - attendance
  - approval
extra:
  team: people-ops
`
	p := writeRoleFile(t, dir, "hr.yaml", body)
	cfg, err := LoadRoleConfig(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "人事助理" {
		t.Errorf("name = %q", cfg.Name)
	}
	if cfg.ClientID != "dinghr0001" {
		t.Errorf("client_id = %q", cfg.ClientID)
	}
	if cfg.OwnerUserID != "012345678901" {
		t.Errorf("owner_user_id = %q", cfg.OwnerUserID)
	}
	if cfg.ConfirmPolicy != ConfirmRemember {
		t.Errorf("confirm_policy = %q", cfg.ConfirmPolicy)
	}
	if !strings.Contains(cfg.Persona, "HR assistant") {
		t.Errorf("persona = %q", cfg.Persona)
	}
	if len(cfg.KnowledgeSources) != 3 {
		t.Fatalf("knowledge_sources = %v", cfg.KnowledgeSources)
	}
	if len(cfg.AllowedScopes) != 2 || cfg.AllowedScopes[0] != "attendance" {
		t.Errorf("allowed_scopes = %v", cfg.AllowedScopes)
	}
	if cfg.Extra["team"] != "people-ops" {
		t.Errorf("extra = %v", cfg.Extra)
	}
}

func TestLoadRoleConfig_DefaultConfirmPolicy(t *testing.T) {
	dir := t.TempDir()
	body := `
name: 前端分身
client_id: dingfe0001
owner_user_id: "999"
`
	p := writeRoleFile(t, dir, "fe.yaml", body)
	cfg, err := LoadRoleConfig(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ConfirmPolicy != ConfirmManual {
		t.Errorf("default confirm_policy = %q, want manual", cfg.ConfirmPolicy)
	}
}

func TestLoadRoleConfig_Errors(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantSub string
	}{
		{
			name:    "missing name",
			body:    "client_id: a\nowner_user_id: b\n",
			wantSub: `"name" is required`,
		},
		{
			name:    "missing client_id",
			body:    "name: x\nowner_user_id: b\n",
			wantSub: `"client_id" is required`,
		},
		{
			name:    "missing owner_user_id",
			body:    "name: x\nclient_id: a\n",
			wantSub: `"owner_user_id" is required`,
		},
		{
			name:    "bad confirm_policy",
			body:    "name: x\nclient_id: a\nowner_user_id: b\nconfirm_policy: sometimes\n",
			wantSub: `"confirm_policy" must be one of`,
		},
		{
			name:    "empty knowledge source",
			body:    "name: x\nclient_id: a\nowner_user_id: b\nknowledge_sources:\n  - \"\"\n",
			wantSub: "knowledge_sources[0] is empty",
		},
		{
			name:    "bad wiki source",
			body:    "name: x\nclient_id: a\nowner_user_id: b\nknowledge_sources:\n  - \"wiki:\"\n",
			wantSub: "missing a spaceId",
		},
		{
			name:    "malformed yaml",
			body:    "name: [unterminated\n",
			wantSub: "parse role config",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			p := writeRoleFile(t, dir, "r.yaml", tc.body)
			_, err := LoadRoleConfig(p)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestLoadRoleConfig_MissingFile(t *testing.T) {
	_, err := LoadRoleConfig(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil || !strings.Contains(err.Error(), "read role config") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestRoleConfigExample_Parses(t *testing.T) {
	dir := t.TempDir()
	p := writeRoleFile(t, dir, "example.yaml", RoleConfigExample)
	cfg, err := LoadRoleConfig(p)
	if err != nil {
		t.Fatalf("example YAML must be valid: %v", err)
	}
	if cfg.ConfirmPolicy != ConfirmManual {
		t.Errorf("example confirm_policy = %q", cfg.ConfirmPolicy)
	}
}

func TestLoadRoleConfigs_IndexByClientID(t *testing.T) {
	dir := t.TempDir()
	writeRoleFile(t, dir, "hr.yaml", "name: HR\nclient_id: dinghr\nowner_user_id: o1\n")
	writeRoleFile(t, dir, "fin.yml", "name: Finance\nclient_id: dingfin\nowner_user_id: o2\n")
	// Non-yaml and subdirectory entries must be ignored.
	writeRoleFile(t, dir, "notes.txt", "ignore me")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	roles, err := LoadRoleConfigs(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("got %d roles, want 2", len(roles))
	}
	if roles["dinghr"].Name != "HR" {
		t.Errorf("dinghr -> %q", roles["dinghr"].Name)
	}
	if roles["dingfin"].Name != "Finance" {
		t.Errorf("dingfin -> %q", roles["dingfin"].Name)
	}
}

func TestLoadRoleConfigs_DuplicateClientID(t *testing.T) {
	dir := t.TempDir()
	writeRoleFile(t, dir, "a.yaml", "name: A\nclient_id: dingdup\nowner_user_id: o1\n")
	writeRoleFile(t, dir, "b.yaml", "name: B\nclient_id: dingdup\nowner_user_id: o2\n")
	_, err := LoadRoleConfigs(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate client_id") {
		t.Fatalf("expected duplicate client_id error, got %v", err)
	}
}

func TestLoadRoleConfigs_Empty(t *testing.T) {
	_, err := LoadRoleConfigs(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no role config") {
		t.Fatalf("expected empty-dir error, got %v", err)
	}
}

func TestLoadRoleConfigs_MissingDir(t *testing.T) {
	_, err := LoadRoleConfigs(filepath.Join(t.TempDir(), "nope"))
	if err == nil || !strings.Contains(err.Error(), "read role config dir") {
		t.Fatalf("expected dir read error, got %v", err)
	}
}
