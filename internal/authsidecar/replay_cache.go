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
	"sync"
	"time"
)

type ReplayCache struct {
	mu       sync.Mutex
	entries  map[string]time.Time
	ttl      time.Duration
	capacity int
}

func NewReplayCache(capacity int, ttl time.Duration) *ReplayCache {
	if capacity <= 0 {
		capacity = 10_000
	}
	if ttl < 2*MaxTimestampDrift {
		ttl = 2 * MaxTimestampDrift
	}
	return &ReplayCache{entries: make(map[string]time.Time), ttl: ttl, capacity: capacity}
}

func (c *ReplayCache) Use(keyID, nonce string, now time.Time) error {
	if c == nil {
		return fmt.Errorf("replay cache is not configured")
	}
	cacheKey := keyID + "\x00" + nonce
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, expiresAt := range c.entries {
		if !expiresAt.After(now) {
			delete(c.entries, key)
		}
	}
	if _, exists := c.entries[cacheKey]; exists {
		return fmt.Errorf("replay detected")
	}
	if len(c.entries) >= c.capacity {
		return fmt.Errorf("replay cache capacity exhausted")
	}
	c.entries[cacheKey] = now.Add(c.ttl)
	return nil
}
