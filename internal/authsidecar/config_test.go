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

package authsidecar

import (
	"strings"
	"testing"
)

func TestPrepareClientFailsClosedOnHalfConfiguration(t *testing.T) {
	t.Setenv(EnvAuthMode, "")
	t.Setenv(EnvSidecarAddress, "unix:///run/dws-sidecar/dws.sock")

	err := PrepareClient(nil)
	if err == nil || !strings.Contains(err.Error(), "sidecar_config_incomplete") {
		t.Fatalf("PrepareClient() error = %v, want sidecar_config_incomplete", err)
	}
}

func TestValidateSidecarEnvConsistency(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		keyID   string
		wantErr bool
	}{
		{name: "nothing set", mode: "", keyID: "", wantErr: false},
		{name: "full sidecar mode", mode: AuthModeSidecar, keyID: "sandbox-a", wantErr: false},
		{name: "key id without mode", mode: "", keyID: "sandbox-a", wantErr: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(EnvAuthMode, testCase.mode)
			t.Setenv(EnvSidecarAddress, "")
			t.Setenv(EnvSidecarKeyID, testCase.keyID)
			t.Setenv(EnvSidecarKeyFile, "")
			err := ValidateSidecarEnvConsistency()
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateSidecarEnvConsistency() error = %v, wantErr = %v", err, testCase.wantErr)
			}
		})
	}
}

func TestParseExactIdentitySelector(t *testing.T) {
	corpID, userID, err := ParseExactIdentitySelector("corp-a:user-a")
	if err != nil || corpID != "corp-a" || userID != "user-a" {
		t.Fatalf("ParseExactIdentitySelector() = %q, %q, %v", corpID, userID, err)
	}
	for _, selector := range []string{
		"", "corp-a", ":user-a", "corp-a:", " corp-a:user-a", "corp-a : user-a", "corp-a:user-a ",
	} {
		if _, _, err := ParseExactIdentitySelector(selector); err == nil {
			t.Fatalf("ParseExactIdentitySelector(%q) accepted a non-literal selector", selector)
		}
	}
}
