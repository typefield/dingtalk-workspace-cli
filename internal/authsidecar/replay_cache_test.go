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
	"testing"
	"time"
)

func TestReplayCacheIsolatesCapacityPerKey(t *testing.T) {
	cache := NewReplayCache(3, 0)
	now := time.Unix(1700000000, 0)
	for index := 0; index < 3; index++ {
		nonce := strings.Repeat(fmt.Sprint(index), 32)
		if err := cache.Use("key-a", nonce, now); err != nil {
			t.Fatalf("Use(key-a, #%d) = %v", index, err)
		}
	}
	if err := cache.Use("key-a", strings.Repeat("9", 32), now); err == nil {
		t.Fatal("key-a exceeded its capacity without an error")
	}
	if err := cache.Use("key-b", strings.Repeat("8", 32), now); err != nil {
		t.Fatalf("key-b was starved by key-a's flood: %v", err)
	}
}

func TestReplayCacheDetectsReplayAndExpiresEntries(t *testing.T) {
	cache := NewReplayCache(10, 0)
	now := time.Unix(1700000000, 0)
	nonce := strings.Repeat("a", 32)
	if err := cache.Use("key-a", nonce, now); err != nil {
		t.Fatal(err)
	}
	if err := cache.Use("key-a", nonce, now); err == nil || !strings.Contains(err.Error(), "replay detected") {
		t.Fatalf("second Use() = %v, want replay detected", err)
	}
	later := now.Add(3 * MaxTimestampDrift)
	if err := cache.Use("key-a", nonce, later); err != nil {
		t.Fatalf("expired nonce was not reusable: %v", err)
	}
}
