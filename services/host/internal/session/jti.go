// SPDX-License-Identifier: Apache-2.0

package session

import (
	"sync"
	"time"
)

// JTIMemory remembers every jti of a session token until its exp plus auth.clock_leeway (24 5.3,
// Ruling T9). It lives only in memory: after a restart every earlier session has ended
// (host_restarted, D-W-90), so only a server that opens an ended session again could replay, which
// 24 5.1 leaves outside the protection goal. It is safe for concurrent use.
type JTIMemory struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// NewJTIMemory returns an empty memory.
func NewJTIMemory() *JTIMemory { return &JTIMemory{seen: map[string]time.Time{}} }

// Remember records jti until until and reports whether it was new. Entries whose time passed
// before now are pruned first.
func (j *JTIMemory) Remember(jti string, until, now time.Time) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	for k, t := range j.seen {
		if now.After(t) {
			delete(j.seen, k)
		}
	}
	if _, ok := j.seen[jti]; ok {
		return false
	}
	j.seen[jti] = until
	return true
}

// Len returns the number of remembered entries.
func (j *JTIMemory) Len() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.seen)
}
