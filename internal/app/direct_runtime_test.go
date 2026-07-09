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

package app

import "testing"

func TestMCPDevMCPEndpointDefaultsToPreGateway(t *testing.T) {
	t.Setenv(mcpdevEndpointEnv, "")

	got := mcpdevMCPEndpoint()
	want := preMCPGatewayBaseURL + mcpdevServerPath
	if got != want {
		t.Fatalf("mcpdevMCPEndpoint() = %q, want %q", got, want)
	}
}

func TestMCPDevMCPEndpointUsesExplicitURL(t *testing.T) {
	t.Setenv(mcpdevEndpointEnv, "https://mcp-gw.example.com/server/custom/")

	got := mcpdevMCPEndpoint()
	want := "https://mcp-gw.example.com/server/custom"
	if got != want {
		t.Fatalf("mcpdevMCPEndpoint() = %q, want %q", got, want)
	}
}
