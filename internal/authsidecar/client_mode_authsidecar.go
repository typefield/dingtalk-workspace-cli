//go:build authsidecar

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
	"fmt"
	"strings"
)

// PrepareClient validates sidecar mode before Cobra builds the command tree and
// enables a fail-closed sentinel credential provider in the token manager.
func PrepareClient(args []string) error {
	if err := ValidateAuthMode(); err != nil {
		return err
	}
	if err := ValidateSidecarEnvConsistency(); err != nil {
		return err
	}
	if !SidecarModeRequested() {
		return nil
	}
	if err := ValidateClientArgs(args); err != nil {
		return err
	}
	if _, err := LoadClientConfigFromEnv(); err != nil {
		return err
	}
	return nil
}

// ResolveClientToken is the build-tagged credential provider used before the
// ordinary token manager touches profile metadata, markers, or keychain.
func ResolveClientToken(explicitToken string) (string, bool, error) {
	if !SidecarModeRequested() {
		return "", false, nil
	}
	if strings.TrimSpace(explicitToken) != "" {
		return "", true, fmt.Errorf("explicit token cannot be used with sidecar authentication")
	}
	return SentinelUserToken, true, nil
}
