// SPDX-License-Identifier: Apache-2.0

package app

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Cache is the in-memory result cache of the statistics of one api process (M6-05 design note 6,
// docs/observability/admin-statistics.md section 5): an entry expires stats.cache_ttl after it was
// stored, by the clock, and the cache holds at most stats.cache_max_entries entries, evicting the
// least recently used one when full, because arbitrary ranges give unbounded distinct keys. It is
// safe for concurrent use.
type Cache struct {
	ttl time.Duration
	max int
	clk clock.Clock

	mu    sync.Mutex
	order *list.List // front is the most recently used entry
	items map[string]*list.Element
}

// cacheEntry is one cached result.
type cacheEntry struct {
	key     string
	value   any
	expires time.Time
}

// NewCache returns an empty cache with the lifetime ttl and at most maxEntries entries over clk
// (nil means clock.System()). A ttl or maxEntries of zero or less disables caching.
func NewCache(ttl time.Duration, maxEntries int, clk clock.Clock) *Cache {
	if clk == nil {
		clk = clock.System()
	}
	return &Cache{ttl: ttl, max: maxEntries, clk: clk, order: list.New(), items: map[string]*list.Element{}}
}

// Get returns the value of key when it is stored and not expired; an expired entry is removed.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*cacheEntry)
	if !c.clk.Now().Before(e.expires) {
		c.order.Remove(el)
		delete(c.items, key)
		return nil, false
	}
	c.order.MoveToFront(el)
	return e.value, true
}

// Put stores value under key for the lifetime of the cache and evicts the least recently used
// entries above the bound. The caller never changes value afterwards.
func (c *Cache) Put(key string, value any) {
	if c.ttl <= 0 || c.max <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	expires := c.clk.Now().Add(c.ttl)
	if el, ok := c.items[key]; ok {
		e := el.Value.(*cacheEntry)
		e.value, e.expires = value, expires
		c.order.MoveToFront(el)
		return
	}
	c.items[key] = c.order.PushFront(&cacheEntry{key: key, value: value, expires: expires})
	for c.order.Len() > c.max {
		last := c.order.Back()
		c.order.Remove(last)
		delete(c.items, last.Value.(*cacheEntry).key)
	}
}

// Len returns the number of stored entries, expired ones included until they are read or evicted.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}

// scopeKey is the part of a cache key that names the caller's scope: the rights version, whether
// the scope is instance-wide by an owner, and a hash of the channel set. A permission change gives
// a new rights version and so a miss; a manager whose scope shrank never sees the old numbers
// (design note 6).
func scopeKey(permissionVersion int64, instanceWide bool, channels []ids.ID) string {
	h := sha256.New()
	for _, c := range channels {
		h.Write(c[:])
	}
	return strconv.FormatInt(permissionVersion, 10) + "|" + strconv.FormatBool(instanceWide) + "|" + hex.EncodeToString(h.Sum(nil))
}

// cacheKey joins the operation, its normalized parameters and the scope key.
func cacheKey(op string, params []string, scope string) string {
	return op + "|" + strings.Join(params, "|") + "|" + scope
}
