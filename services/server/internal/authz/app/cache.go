// SPDX-License-Identifier: Apache-2.0

package app

import (
	"container/list"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// cacheKey identifies one rights snapshot (07 section 7.3): the principal with its credentials and
// the rights version. Snapshots of "view as" and acting-for persons use zero credential ids.
type cacheKey struct {
	principal, session, device, token ids.ID
	version                           int64
}

// entry is one cached snapshot with the liveness facts read with it.
type entry struct {
	key   cacheKey
	found bool
	facts authz.PrincipalFacts
	creds authz.CredentialFacts
	snap  domain.Snapshot
}

// snapshotCache is the LRU of snapshots, bounded by authz.snapshot_cache_max_entries. A hit needs
// the current rights version (part of the key) and t < ValidUntil, so an expiry takes effect
// without a job (07 section 2.5, AZ-P10). It is safe for concurrent use.
type snapshotCache struct {
	mu    sync.Mutex
	max   int
	order *list.List
	items map[cacheKey]*list.Element
}

func newSnapshotCache(maxEntries int) *snapshotCache {
	return &snapshotCache{max: maxEntries, order: list.New(), items: map[cacheKey]*list.Element{}}
}

// get returns the entry of k when it is valid at t; an expired entry is dropped.
func (c *snapshotCache) get(k cacheKey, t time.Time) (entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		return entry{}, false
	}
	e, _ := el.Value.(entry)
	if !e.snap.ValidAt(t) {
		c.order.Remove(el)
		delete(c.items, k)
		return entry{}, false
	}
	c.order.MoveToFront(el)
	return e, true
}

// put stores e as the most recent entry and evicts the least recent ones above the bound.
func (c *snapshotCache) put(e entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[e.key]; ok {
		el.Value = e
		c.order.MoveToFront(el)
		return
	}
	c.items[e.key] = c.order.PushFront(e)
	for c.order.Len() > c.max {
		last := c.order.Back()
		old, _ := last.Value.(entry)
		c.order.Remove(last)
		delete(c.items, old.key)
	}
}

// len returns the number of entries.
func (c *snapshotCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
