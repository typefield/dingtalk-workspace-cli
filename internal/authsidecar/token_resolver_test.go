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
	"crypto/tls"
	"net/http"
	"testing"
)

func TestHardenedOAuthClientDisablesProxyAndRedirects(t *testing.T) {
	client := hardenedOAuthClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("OAuth transport inherits environment proxy configuration")
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLS MinVersion = %v, want TLS 1.2", transport.TLSClientConfig)
	}
	if client.CheckRedirect == nil {
		t.Fatal("OAuth client follows redirects")
	}
	if err := client.CheckRedirect(nil, nil); err == nil {
		t.Fatal("CheckRedirect permits a redirect")
	}
	if client.Timeout <= 0 {
		t.Fatal("OAuth client has no timeout")
	}
}
