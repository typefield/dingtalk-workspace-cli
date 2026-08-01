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

package auth

import (
	"errors"
	"testing"
)

func TestSidecarModeDeniesEveryLocalCredentialReadBoundary(t *testing.T) {
	t.Setenv(sidecarAuthModeEnv, sidecarAuthModeValue)
	SetClientSecret("runtime-secret")
	t.Cleanup(func() { SetClientSecret("") })

	readers := []struct {
		name string
		read func() error
	}{
		{name: "profile token", read: func() error { _, err := LoadTokenDataForProfile(t.TempDir(), "corp:user"); return err }},
		{name: "global keychain", read: func() error { _, err := LoadTokenDataKeychain(); return err }},
		{name: "corp keychain", read: func() error { _, err := LoadTokenDataKeychainForCorpID("corp"); return err }},
		{name: "identity keychain", read: func() error { _, err := LoadTokenDataKeychainForIdentity("corp", "user"); return err }},
		{name: "encrypted file", read: func() error { _, err := LoadSecureTokenData(t.TempDir()); return err }},
		{name: "app token", read: func() error { _, err := LoadAppTokenData("client"); return err }},
		{name: "app config", read: func() error { _, err := LoadAppConfig(t.TempDir()); return err }},
		{name: "app config reload", read: func() error { _, err := ReloadAppConfig(t.TempDir()); return err }},
		{name: "strict app credentials", read: func() error {
			_, _, _, _, err := ResolveAppCredentialsStrict(t.TempDir())
			return err
		}},
		{name: "plain client secret", read: func() error { _, err := ResolveSecret(SecretInput{Plain: "secret"}); return err }},
	}
	for _, reader := range readers {
		t.Run(reader.name, func(t *testing.T) {
			if err := reader.read(); !errors.Is(err, ErrSidecarCredentialAccessDenied) {
				t.Fatalf("credential read error = %v, want ErrSidecarCredentialAccessDenied", err)
			}
		})
	}
	if got := LoadClientSecret("client"); got != "" {
		t.Fatalf("LoadClientSecret() = %q in sidecar mode", got)
	}
	if got := ClientSecret(); got != "" {
		t.Fatalf("ClientSecret() = %q in sidecar mode", got)
	}
	if clientID, secret := ResolveAppCredentials(t.TempDir()); clientID != "" || secret != "" {
		t.Fatalf("ResolveAppCredentials() = %q, %q in sidecar mode", clientID, secret)
	}
	if got := GetCachedAppConfig(t.TempDir()); got != nil {
		t.Fatalf("GetCachedAppConfig() = %#v in sidecar mode", got)
	}
}

func TestLocalCredentialReadsRemainAvailableOutsideSidecarMode(t *testing.T) {
	t.Setenv(sidecarAuthModeEnv, "")
	if err := rejectSidecarCredentialAccess(); err != nil {
		t.Fatalf("rejectSidecarCredentialAccess() = %v outside sidecar mode", err)
	}
}
