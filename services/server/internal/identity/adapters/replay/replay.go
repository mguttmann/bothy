// SPDX-License-Identifier: Apache-2.0

// Package replay is the in-memory jti store of the DPoP verifier (identity/app.ReplayStore;
// docs/spec/08-identitaet.md "7.3 DPoP"): per process, keyed by SHA-256(htu || 0x00 || jti), each
// entry kept until its expiry. Across processes the server nonce protects (08 "7.3"). There is
// no size cap, because the spec names none: the window auth.dpop_proof_max_age plus
// auth.clock_leeway and the rate limits bound the entries (docs/DECISIONS.md M1-33).
//
// Example:
//
//	store := replay.New(clock.System())
//	go store.Run(ctx, interval) // removes expired entries until ctx ends
//	first := store.CheckAndRemember(key, until)
package replay

import (
	"context"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// shards spreads the keys over independent locks; the first key byte picks the shard, and the
// keys are SHA-256 values, so the spread is even.
const shards = 16

// insertProbe is how many entries an insert inspects for expiry: a small constant keeps the
// insert constant time, and with more than one the removals outpace the inserts once the window
// has passed.
const insertProbe = 4

type shard struct {
	mu      sync.Mutex
	entries map[[32]byte]time.Time
}

// Store is the concurrency-safe store. The zero value is not usable; call New.
type Store struct {
	clk    clock.Clock
	shards [shards]shard
}

// New returns an empty store that reads the time from clk.
func New(clk clock.Clock) *Store {
	s := &Store{clk: clk}
	for i := range s.shards {
		s.shards[i].entries = map[[32]byte]time.Time{}
	}
	return s
}

// CheckAndRemember reports true and remembers key until the given time when key is unknown or
// expired, and false when key is remembered and not yet expired. Check and insert are one atomic
// step under the shard lock. On the way it looks at a few entries of the shard and removes the
// expired ones, so inserts stay constant time and the ticker of Run does the full sweep.
func (s *Store) CheckAndRemember(key [32]byte, until time.Time) bool {
	now := s.clk.Now()
	sh := &s.shards[key[0]%shards]
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if exp, ok := sh.entries[key]; ok && now.Before(exp) {
		return false
	}
	visited := 0
	for k, exp := range sh.entries { // map order is random, so every entry gets its turn
		if visited == insertProbe {
			break
		}
		visited++
		if !now.Before(exp) {
			delete(sh.entries, k)
		}
	}
	sh.entries[key] = until
	return true
}

// sweepShard removes the expired entries of sh; the caller holds its lock.
func sweepShard(sh *shard, now time.Time) {
	for k, exp := range sh.entries {
		if !now.Before(exp) {
			delete(sh.entries, k)
		}
	}
}

// Sweep removes every expired entry.
func (s *Store) Sweep() {
	now := s.clk.Now()
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		sweepShard(sh, now)
		sh.mu.Unlock()
	}
}

// Len returns the number of entries, expired ones included until the next sweep.
func (s *Store) Len() int {
	n := 0
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		n += len(sh.entries)
		sh.mu.Unlock()
	}
	return n
}

// Run sweeps every interval until ctx ends; the lifecycle of the server owns the goroutine
// (internal/wire). An interval below one millisecond counts as one millisecond.
func (s *Store) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(max(interval, time.Millisecond))
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Sweep()
		}
	}
}
