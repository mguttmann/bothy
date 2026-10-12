// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"hash/maphash"
	"sync"
	"time"

	otelapi "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// Fixed-point arithmetic: a bucket stores tokens in units of 1/unitsPerToken, chosen so that the
// refill per nanosecond is exactly per_minute units (one token per minute is 60e9 units per 60e9
// ns). Every header value is then an exact integer division, with no float rounding.
const (
	unitsPerToken = int64(60 * time.Second) // 60e9 units per token
	nsPerSecond   = int64(time.Second)
)

// MaxLimit is the largest per_minute and burst a class may have. It keeps burst x unitsPerToken
// (6e16) far below the int64 range; the configuration schema has the same bound.
const MaxLimit = 1_000_000

// Shards: the table is split so that concurrent requests on different keys rarely share a mutex.
// minPerShard keeps small caps in few shards, so the per-shard cap (cap / shards, at least one)
// never rounds the total far below ratelimit.max_tracked_keys.
const (
	maxShards   = 64
	minPerShard = 16
	// evictSample is how many buckets of a full shard an insert inspects: map iteration order
	// in Go is random, so the first entries are a random sample. Eight keeps an insert into a
	// full shard cheap while still finding a full or a long-unused bucket in most floods.
	evictSample = 8
)

// instrumentation is the meter name of bothy.ratelimit.rejected when New gets no meter.
const instrumentation = "github.com/OWNER/bothy/services/server/internal/platform/ratelimit"

// metricRejected is bothy.ratelimit.rejected of signals.md "REST and gateway".
const metricRejected = "bothy.ratelimit.rejected"

// Limits are the token-bucket values of one class: ratelimit.<class>.per_minute and .burst.
type Limits struct{ PerMinute, Burst int }

// Config is the configuration of a Limiter; internal/wire binds it from the keys
// ratelimit.<class>.per_minute, ratelimit.<class>.burst, ratelimit.max_tracked_keys and
// ratelimit.sweep_interval.
type Config struct {
	// Classes holds the limits of every class of Classes(); New rejects a missing or unknown one.
	Classes map[Class]Limits
	// MaxTrackedKeys is ratelimit.max_tracked_keys, the hard cap on tracked buckets.
	MaxTrackedKeys int
	// SweepInterval is ratelimit.sweep_interval, how often Run removes full buckets.
	SweepInterval time.Duration
}

// Decision is the result of one Allow or Peek, with the values of the RateLimit headers.
type Decision struct {
	// Allowed is true when a token was taken (Allow) or is available (Peek).
	Allowed bool
	// Remaining is r: the whole tokens left after the decision, rounded down.
	Remaining int
	// ResetSeconds is t: the seconds until the bucket is full, rounded up; 0 when full.
	ResetSeconds int
	// RetryAfterSeconds is Retry-After when denied: the seconds until one token is there, rounded
	// up and at least 1; 0 when allowed.
	RetryAfterSeconds int
	// Limits are the limits of the class (q is PerMinute).
	Limits Limits
}

// bucket is one token bucket: units of tokens at the instant last (Unix nanoseconds).
type bucket struct {
	units int64
	last  int64
}

// shard is one part of the table with its own mutex.
type shard struct {
	mu sync.Mutex
	m  map[string]bucket
}

// Limiter holds the token buckets of every class and key. It is safe for concurrent use; build
// it with New.
type Limiter struct {
	limits   map[Class]Limits
	clk      clock.Clock
	shards   []shard
	seed     maphash.Seed
	perShard int
	sweep    time.Duration
	rejected metric.Int64Counter
	attrs    map[Class]metric.AddOption
}

// New checks cfg and returns a Limiter on clk (nil means clock.System()) that counts rejections
// in bothy.ratelimit.rejected of m (nil means a meter of the global provider of platform/otel, so
// call New after otel.Setup). The error names every problem: a class
// of Classes() without limits, a class 05 does not list, PerMinute or Burst outside 1 to
// MaxLimit, MaxTrackedKeys below 1, SweepInterval not positive, or a counter m cannot create.
//
// Example:
//
//	l, err := ratelimit.New(ratelimit.Config{
//		Classes:        map[ratelimit.Class]ratelimit.Limits{"rl.read": {PerMinute: 600, Burst: 100}, ...},
//		MaxTrackedKeys: 100000,
//		SweepInterval:  time.Minute,
//	}, clock.System(), meter)
func New(cfg Config, clk clock.Clock, m metric.Meter) (*Limiter, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if clk == nil {
		clk = clock.System()
	}
	if m == nil {
		m = otelapi.GetMeterProvider().Meter(instrumentation)
	}
	counter, err := m.Int64Counter(metricRejected,
		metric.WithUnit("{request}"),
		metric.WithDescription("Requests refused because their rate-limit class was exhausted."))
	if err != nil {
		return nil, fmt.Errorf("ratelimit: %s: %w", metricRejected, err)
	}
	n := min(max(cfg.MaxTrackedKeys/minPerShard, 1), maxShards)
	l := &Limiter{
		limits: make(map[Class]Limits, len(cfg.Classes)), clk: clk,
		shards: make([]shard, n), seed: maphash.MakeSeed(), perShard: cfg.MaxTrackedKeys / n, sweep: cfg.SweepInterval,
		rejected: counter, attrs: make(map[Class]metric.AddOption, len(cfg.Classes)),
	}
	for c, lim := range cfg.Classes {
		l.limits[c] = lim
		// The attribute values come from the closed class list, so the metric's cardinality is
		// bounded (signals.md "Cardinality").
		l.attrs[c] = metric.WithAttributeSet(attribute.NewSet(attribute.String(attr.RatelimitClass, string(c))))
	}
	for i := range l.shards {
		l.shards[i].m = make(map[string]bucket)
	}
	return l, nil
}

// validate returns every problem of cfg joined, or nil (see New).
func (cfg Config) validate() error {
	var problems []error
	for _, c := range Classes() {
		if _, ok := cfg.Classes[c]; !ok {
			problems = append(problems, fmt.Errorf("ratelimit: class %s has no limits", c))
		}
	}
	for c, lim := range cfg.Classes {
		if _, ok := kinds[c]; !ok {
			problems = append(problems, fmt.Errorf("ratelimit: class %s is not a class of 05", c))
		}
		if lim.PerMinute < 1 || lim.PerMinute > MaxLimit || lim.Burst < 1 || lim.Burst > MaxLimit {
			problems = append(problems, fmt.Errorf("ratelimit: class %s: per_minute and burst must be between 1 and %d", c, MaxLimit))
		}
	}
	if cfg.MaxTrackedKeys < 1 {
		problems = append(problems, errors.New("ratelimit: max_tracked_keys must be at least 1"))
	}
	if cfg.SweepInterval <= 0 {
		problems = append(problems, errors.New("ratelimit: sweep_interval must be positive"))
	}
	return errors.Join(problems...)
}

// Allow takes one token from the bucket of class and key and returns the decision. key is the
// value of a KeyFor* function or of RequestKey; buckets of different classes never mix. A denied
// request takes nothing and counts in bothy.ratelimit.rejected. The error reports a class the
// limiter does not know or an empty key; nothing is counted then.
func (l *Limiter) Allow(class Class, key string) (Decision, error) {
	lim, err := l.check(class, key)
	if err != nil {
		return Decision{}, err
	}
	now := l.clk.Now().UnixNano()
	ik := internalKey(class, key)
	s := l.shard(ik)
	s.mu.Lock()
	b, ok := s.m[ik]
	if !ok {
		b = bucket{units: capacity(lim), last: now}
	} else {
		b = refill(b, now, lim)
	}
	allowed := b.units >= unitsPerToken
	if allowed {
		b.units -= unitsPerToken
	}
	if allowed || ok {
		// A denied request on a missing bucket cannot happen (a new bucket is full); a denied one
		// on an existing bucket stores the refill.
		if !ok && len(s.m) >= l.perShard {
			s.evict(now, l.limitsOf)
		}
		s.m[ik] = b
	}
	s.mu.Unlock()
	if !allowed {
		l.Reject(class)
	}
	return decide(b.units, lim, allowed), nil
}

// Peek returns the decision Allow would make for class and key without taking a token and
// without creating a bucket: Allowed is true when a whole token is there. A caller that only
// wants to look (for example to show the RateLimit headers of a class it does not count) uses it;
// the failed-credentials precheck reserves with Allow instead, so concurrent requests cannot pass
// on the same token. The error is that of Allow.
func (l *Limiter) Peek(class Class, key string) (Decision, error) {
	lim, err := l.check(class, key)
	if err != nil {
		return Decision{}, err
	}
	now := l.clk.Now().UnixNano()
	ik := internalKey(class, key)
	s := l.shard(ik)
	s.mu.Lock()
	b, ok := s.m[ik]
	s.mu.Unlock()
	units := capacity(lim)
	if ok {
		units = refill(b, now, lim).units
	}
	return decide(units, lim, units >= unitsPerToken), nil
}

// Refund returns one token to the bucket of class and key, never above the burst. A caller that
// took a token with Allow as a reservation gives it back with Refund when the reason to count
// fell away (the failed-credentials rule reserves before it verifies). A missing bucket is full
// already, so nothing is created. The error is that of Allow.
func (l *Limiter) Refund(class Class, key string) error {
	lim, err := l.check(class, key)
	if err != nil {
		return err
	}
	now := l.clk.Now().UnixNano()
	ik := internalKey(class, key)
	s := l.shard(ik)
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.m[ik]
	if !ok {
		return nil
	}
	b = refill(b, now, lim)
	b.units = min(b.units+unitsPerToken, capacity(lim))
	s.m[ik] = b
	return nil
}

// Reject counts one refused request of class in bothy.ratelimit.rejected. Allow calls it for
// every denial; a caller that refuses on a Peek calls it itself. An unknown class is ignored.
func (l *Limiter) Reject(class Class) {
	if opt, ok := l.attrs[class]; ok {
		l.rejected.Add(context.Background(), 1, opt)
	}
}

// Limits returns the limits of class and true, or false for a class the limiter does not know.
func (l *Limiter) Limits(class Class) (Limits, bool) {
	lim, ok := l.limits[class]
	return lim, ok
}

// Len returns the number of tracked buckets; it never exceeds ratelimit.max_tracked_keys.
func (l *Limiter) Len() int {
	n := 0
	for i := range l.shards {
		s := &l.shards[i]
		s.mu.Lock()
		n += len(s.m)
		s.mu.Unlock()
	}
	return n
}

// Sweep removes every bucket that is full now. A full bucket decides exactly like a missing one
// (a new bucket starts full), so no decision changes.
func (l *Limiter) Sweep() {
	now := l.clk.Now().UnixNano()
	for i := range l.shards {
		s := &l.shards[i]
		s.mu.Lock()
		for k, b := range s.m {
			if l.full(k, b, now) {
				delete(s.m, k)
			}
		}
		s.mu.Unlock()
	}
}

// Run calls Sweep every ratelimit.sweep_interval until ctx ends; start it once in its own
// goroutine (internal/wire does, under the lifecycle of the server).
func (l *Limiter) Run(ctx context.Context) {
	t := time.NewTicker(l.sweep)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.Sweep()
		}
	}
}

// check returns the limits of class, or the error of Allow.
func (l *Limiter) check(class Class, key string) (Limits, error) {
	lim, ok := l.limits[class]
	if !ok {
		return Limits{}, fmt.Errorf("ratelimit: unknown class %q", class)
	}
	if key == "" {
		return Limits{}, fmt.Errorf("ratelimit: empty key for class %s", class)
	}
	return lim, nil
}

// limitsOf returns the limits of the class an internal key belongs to.
func (l *Limiter) limitsOf(ik string) Limits {
	for i := 0; i < len(ik); i++ {
		if ik[i] == '|' {
			return l.limits[Class(ik[:i])]
		}
	}
	return Limits{}
}

// full reports whether the bucket of internal key ik is full at now.
func (l *Limiter) full(ik string, b bucket, now int64) bool {
	lim := l.limitsOf(ik)
	return refill(b, now, lim).units >= capacity(lim)
}

// tracked reports whether a bucket exists for class and key (tests).
func (l *Limiter) tracked(class Class, key string) bool {
	ik := internalKey(class, key)
	s := l.shard(ik)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[ik]
	return ok
}

// shard returns the shard of internal key ik. The hash has a random seed per limiter, so a client
// cannot compute which of its keys land in the shard of another bucket and aim a flood of new
// keys at that one shard to evict it (docs/DECISIONS.md M1-19/4).
func (l *Limiter) shard(ik string) *shard {
	return &l.shards[maphash.String(l.seed, ik)%uint64(len(l.shards))]
}

// evict makes room in a full shard; the caller holds s.mu. It inspects a random sample of
// evictSample buckets, removes every full one among them, and only when none is full removes the
// sampled bucket with the oldest last use. An evicted key starts with a fresh (full) bucket on its
// next request; that happens only when more distinct keys are active within one refill window
// than the cap allows.
func (s *shard) evict(now int64, limitsOf func(string) Limits) {
	oldestKey, oldest, seen, removed := "", int64(0), 0, false
	for k, b := range s.m {
		lim := limitsOf(k)
		if refill(b, now, lim).units >= capacity(lim) {
			delete(s.m, k)
			removed = true
		} else if oldestKey == "" || b.last < oldest {
			oldestKey, oldest = k, b.last
		}
		if seen++; seen >= evictSample {
			break
		}
	}
	if !removed && oldestKey != "" {
		delete(s.m, oldestKey)
	}
}

// internalKey is <class>|<key>, so classes never share a bucket.
func internalKey(class Class, key string) string {
	return string(class) + "|" + key
}

// capacity is the burst in units.
func capacity(lim Limits) int64 { return int64(lim.Burst) * unitsPerToken }

// refill returns b refilled up to now. A clock that moved back refills nothing and keeps last,
// so the time it jumped back is not credited twice.
func refill(b bucket, now int64, lim Limits) bucket {
	if now <= b.last {
		return b
	}
	elapsed, rate, full := now-b.last, int64(lim.PerMinute), capacity(lim)
	b.last = now
	need := full - b.units
	// Comparing with the time to full first keeps elapsed x rate below need + rate, so the
	// product cannot overflow however long the bucket was idle.
	if need <= 0 || elapsed >= ceilDiv(need, rate) {
		b.units = full
		return b
	}
	b.units += elapsed * rate
	return b
}

// decide builds the decision from the units left after it (06 "Rate limits"; docs/DECISIONS.md
// M1-19 on rounding): r rounds down, t and Retry-After round up, so a client that waits as told
// always finds what the headers promised.
func decide(units int64, lim Limits, allowed bool) Decision {
	perSecond := int64(lim.PerMinute) * nsPerSecond // units refilled per second
	d := Decision{
		Allowed:      allowed,
		Remaining:    int(units / unitsPerToken),
		ResetSeconds: int(ceilDiv(capacity(lim)-units, perSecond)),
		Limits:       lim,
	}
	if !allowed {
		d.RetryAfterSeconds = max(1, int(ceilDiv(unitsPerToken-units, perSecond)))
	}
	return d
}

// ceilDiv is a / b rounded up for a >= 0 and b > 0; 0 for a <= 0.
func ceilDiv(a, b int64) int64 {
	if a <= 0 {
		return 0
	}
	return (a + b - 1) / b
}
