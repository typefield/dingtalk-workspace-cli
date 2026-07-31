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

// ReplayCache tracks used nonces per key so one key flooding unique nonces
// can only exhaust its own bucket, never another sandbox's replay protection.
type ReplayCache struct {
	mu             sync.Mutex
	buckets        map[string]map[string]time.Time
	ttl            time.Duration
	perKeyCapacity int
}

func NewReplayCache(perKeyCapacity int, ttl time.Duration) *ReplayCache {
	if perKeyCapacity <= 0 {
		perKeyCapacity = 10_000
	}
	if ttl < 2*MaxTimestampDrift {
		ttl = 2 * MaxTimestampDrift
	}
	return &ReplayCache{
		buckets:        make(map[string]map[string]time.Time),
		ttl:            ttl,
		perKeyCapacity: perKeyCapacity,
	}
}

func (c *ReplayCache) Use(keyID, nonce string, now time.Time) error {
	if c == nil {
		return fmt.Errorf("replay cache is not configured")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for owner, bucket := range c.buckets {
		for value, expiresAt := range bucket {
			if !expiresAt.After(now) {
				delete(bucket, value)
			}
		}
		if len(bucket) == 0 {
			delete(c.buckets, owner)
		}
	}
	bucket := c.buckets[keyID]
	if bucket == nil {
		bucket = make(map[string]time.Time)
		c.buckets[keyID] = bucket
	}
	if _, exists := bucket[nonce]; exists {
		return fmt.Errorf("replay detected")
	}
	if len(bucket) >= c.perKeyCapacity {
		return fmt.Errorf("replay cache capacity exhausted for this key")
	}
	bucket[nonce] = now.Add(c.ttl)
	return nil
}
