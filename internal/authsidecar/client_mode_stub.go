//go:build !authsidecar

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

import "fmt"

func PrepareClient([]string) error {
	if err := ValidateAuthMode(); err != nil {
		return err
	}
	if err := ValidateSidecarEnvConsistency(); err != nil {
		return err
	}
	if !SidecarModeRequested() {
		return nil
	}
	return fmt.Errorf("sidecar_build_required: this dws binary was built without -tags authsidecar")
}

func ResolveClientToken(string) (string, bool, error) { return "", false, nil }
