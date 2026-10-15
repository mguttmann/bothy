// SPDX-License-Identifier: Apache-2.0

package discord

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
)

// Sleeper waits d or until ctx ends, whichever comes first, and returns the context error in
// the second case. Tests pass a recording sleeper that never waits.
type Sleeper func(ctx context.Context, d time.Duration) error

// sleepContext is the Sleeper of production.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// guard is the rate-limit state of one Client, that is of one run (22 4.3 items 1 and 4):
// which bucket a route belongs to, when an exhausted bucket refills, and which channels a 403
// ended. It is safe for concurrent use.
//
// Keys: a route is the path template plus its major ID (guild or channel); the bucket is the
// X-RateLimit-Bucket that Discord last named for that route, or the path template when it named
// none, again plus the major ID, because Discord counts limits per bucket and major parameter
// ([R7]).
type guard struct {
	clock   clock.Clock
	maxWait time.Duration

	mu        sync.Mutex
	buckets   map[string]string    // route key -> bucket name
	resetAt   map[string]time.Time // bucket key -> time the exhausted bucket refills
	forbidden map[string]bool      // channel IDs a 403 ended
}

// newGuard returns an empty guard.
func newGuard(clk clock.Clock, maxWait time.Duration) *guard {
	return &guard{clock: clk, maxWait: maxWait, buckets: map[string]string{},
		resetAt: map[string]time.Time{}, forbidden: map[string]bool{}}
}

// routeKey is the key of a route.
func routeKey(template, major string) string { return template + "|" + major }

// bucketKey returns the bucket key of a route; the caller holds mu.
func (g *guard) bucketKey(template, major string) string {
	if b, ok := g.buckets[routeKey(template, major)]; ok {
		return b + "|" + major
	}
	return routeKey(template, major)
}

// pending returns how long a call of the route must wait before it is sent: the time until an
// exhausted bucket refills, or 0.
func (g *guard) pending(template, major string) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	at, ok := g.resetAt[g.bucketKey(template, major)]
	if !ok {
		return 0
	}
	return max(0, at.Sub(g.clock.Now()))
}

// observe records the rate-limit headers of an answer of the route: the bucket name, and with
// X-RateLimit-Remaining 0 the refill time from X-RateLimit-Reset-After; any other remaining
// value clears the bucket. A missing or unparsable Reset-After counts as
// import.discord.max_rate_wait (fail towards waiting).
func (g *guard) observe(template, major string, h http.Header) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if b := h.Get("X-RateLimit-Bucket"); b != "" {
		g.buckets[routeKey(template, major)] = b
	}
	key := g.bucketKey(template, major)
	if h.Get("X-RateLimit-Remaining") != "0" {
		delete(g.resetAt, key)
		return
	}
	wait, ok := parseSeconds(h.Get("X-RateLimit-Reset-After"))
	if !ok {
		wait = g.maxWait
	}
	// A wait above the limit is kept as it is, so that the next call ends with
	// ErrRateWaitTooLong instead of waiting.
	g.resetAt[key] = g.clock.Now().Add(wait)
}

// clear forgets the refill time of the route's bucket after the client waited out a 429.
func (g *guard) clear(template, major string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.resetAt, g.bucketKey(template, major))
}

// forbid ends a channel for the rest of the run.
func (g *guard) forbid(channelID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.forbidden[channelID] = true
}

// isForbidden reports whether a 403 ended the channel.
func (g *guard) isForbidden(channelID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.forbidden[channelID]
}

// retryWait returns the wait a 429 asks for: retry_after of the JSON body, else the header
// Retry-After, both in seconds; a missing or unparsable value counts as
// import.discord.max_rate_wait.
func retryWait(body []byte, h http.Header, maxWait time.Duration) time.Duration {
	var b struct {
		RetryAfter *json.Number `json:"retry_after"`
	}
	if json.Unmarshal(body, &b) == nil && b.RetryAfter != nil {
		if d, ok := parseSeconds(b.RetryAfter.String()); ok {
			return d
		}
	}
	if d, ok := parseSeconds(h.Get("Retry-After")); ok {
		return d
	}
	return maxWait
}

// parseSeconds parses a non-negative number of seconds, fractions allowed. A value too large for
// a time.Duration becomes the largest duration, which is above every limit.
func parseSeconds(s string) (time.Duration, bool) {
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return 0, false
	}
	ns := f * float64(time.Second)
	if ns >= math.MaxInt64 {
		return time.Duration(math.MaxInt64), true
	}
	return time.Duration(ns), true
}
