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

// allowedBrowserSchemes restricts the URL schemes that openBrowser will
// pass to the OS open command. Without this check, a malicious MCP server
// could return a file:// or custom-scheme URL via VerificationURIComplete
// and trick the CLI into opening arbitrary local resources.
var allowedBrowserSchemes = map[string]bool{
	"http":  true,
	"https": true,
}
