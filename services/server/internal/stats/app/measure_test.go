// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/stats/app"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// t0 is the start of the fake clock: a minute boundary.
var t0 = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

// fakeFlushMetrics counts drops per reason and flushes per outcome.
type fakeFlushMetrics struct {
	mu      sync.Mutex
	dropped map[string]int
	flushes map[bool]int
}

func newFakeFlushMetrics() *fakeFlushMetrics {
	return &fakeFlushMetrics{dropped: map[string]int{}, flushes: map[bool]int{}}
}

func (f *fakeFlushMetrics) Dropped(_ context.Context, reason string, n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dropped[reason] += n
}

func (f *fakeFlushMetrics) Flushed(_ context.Context, _ time.Duration, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.flushes[ok]++
}

func (f *fakeFlushMetrics) drops(reason string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dropped[reason]
}

func (f *fakeFlushMetrics) flushCount(ok bool) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.flushes[ok]
}

// fakeStore is a MinuteStore in memory: it adds rows per key, can fail, block, or refuse days
// without a partition.
type fakeStore struct {
	mu         sync.Mutex
	rows       map[string]domain.Counters
	calls      int
	err        error
	partitions map[time.Time]bool // nil: every day has one
	ensured    []time.Time
	// block, when set, makes Upsert signal entered and wait for release.
	entered chan struct{}
	release chan struct{}
}

func newFakeStore() *fakeStore { return &fakeStore{rows: map[string]domain.Counters{}} }

func rowKey(r app.MinuteRow) string {
	return r.BucketStart.Format(time.RFC3339) + "|" + r.Metric + "|" + r.ChannelID.String() + "|" + r.AgentID.String() + "|" +
		r.Dimension + "|" + r.Currency
}

func (s *fakeStore) Upsert(_ context.Context, rows []app.MinuteRow) error {
	if s.entered != nil {
		s.entered <- struct{}{}
		<-s.release
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return s.err
	}
	if s.partitions != nil {
		for _, r := range rows {
			if !s.partitions[domain.Day(r.BucketStart)] {
				return app.ErrNoPartition
			}
		}
	}
	for _, r := range rows {
		k := rowKey(r)
		s.rows[k] = s.rows[k].Add(r.Counters)
	}
	return nil
}

func (s *fakeStore) EnsurePartition(_ context.Context, day time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensured = append(s.ensured, day)
	if s.partitions != nil {
		s.partitions[day] = true
	}
	return nil
}

func (s *fakeStore) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func (s *fakeStore) snapshot() map[string]domain.Counters {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]domain.Counters, len(s.rows))
	for k, v := range s.rows {
		out[k] = v
	}
	return out
}

// newMeasurement returns a measurement over a fake clock at t0 with maxKeys and fake metrics.
func newMeasurement(t *testing.T, maxKeys int) (*app.Measurement, *clock.Fake, *fakeFlushMetrics) {
	t.Helper()
	c := clock.NewFake(t0)
	m := newFakeFlushMetrics()
	r, err := app.NewMeasurement(c, maxKeys, m)
	if err != nil {
		t.Fatal(err)
	}
	return r, c, m
}

// newFlush returns a flush of r into s.
func newFlush(t *testing.T, r *app.Measurement, s app.MinuteStore, m app.FlushMetrics, c clock.Clock) *app.Flush {
	t.Helper()
	f, err := app.NewFlush(r, s, m, c, time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// TestRecorderAggregatesPerMinuteAndKey shows the counters per (minute, metric, channel, agent,
// dimension, currency): calls of one key in one minute add up, another minute or another key is
// another row, durations go in whole milliseconds into sum and good counts only good SLI events.
func TestRecorderAggregatesPerMinuteAndKey(t *testing.T) {
	t.Parallel()
	r, c, _ := newMeasurement(t, 100)
	ch, ag := ids.New(), ids.New()
	human := measure.Key{ChannelID: ch, Dimension: "human"}
	r.Record(measure.MessagesCreated, human, measure.One())
	c.Advance(59 * time.Second)
	r.Record(measure.MessagesCreated, human, measure.One())
	r.Record(measure.MessagesCreated, measure.Key{ChannelID: ch, Dimension: "agent"}, measure.One())
	run := measure.Key{ChannelID: ch, AgentID: ag}
	thr := domain.Threshold(string(measure.SLIRunStart))
	r.Record(measure.SLIRunStart, run, measure.SLI(thr, true))
	r.Record(measure.SLIRunStart, run, measure.SLI(thr+time.Millisecond, true))
	r.Record(measure.SLIRunStart, run, measure.SLI(time.Second, false))
	r.Record(measure.Cost, measure.Key{ChannelID: ch, AgentID: ag, Dimension: "organization", Currency: "EUR"}, measure.Amount(1500))
	c.Advance(time.Second) // the next minute
	r.Record(measure.MessagesCreated, human, measure.One())
	if got := r.Len(); got != 5 {
		t.Fatalf("Len = %d, want 5 keys", got)
	}
	rows := app.Take(r)
	if r.Len() != 0 {
		t.Fatal("take left keys behind")
	}
	got := map[string]domain.Counters{}
	for _, row := range rows {
		got[rowKey(row)] = row.Counters
	}
	m1 := app.MinuteRow{BucketStart: t0, Metric: "messages_created", ChannelID: ch, Dimension: "human"}
	m2 := m1
	m2.BucketStart = t0.Add(time.Minute)
	want := map[string]domain.Counters{
		rowKey(m1): {Count: 2},
		rowKey(app.MinuteRow{BucketStart: t0, Metric: "messages_created", ChannelID: ch, Dimension: "agent"}): {Count: 1},
		rowKey(app.MinuteRow{BucketStart: t0, Metric: "sli.run_start", ChannelID: ch, AgentID: ag}): {
			Count: 3, Sum: 2*thr.Milliseconds() + 1 + 1000, Good: 1,
		},
		rowKey(app.MinuteRow{BucketStart: t0, Metric: "cost", ChannelID: ch, AgentID: ag, Dimension: "organization", Currency: "EUR"}): {
			Count: 1, Sum: 1500,
		},
		rowKey(m2): {Count: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %v\nwant %v", got, want)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %+v, want %+v", k, got[k], w)
		}
	}
	if !slices.IsSortedFunc(rows, func(a, b app.MinuteRow) int { return a.BucketStart.Compare(b.BucketStart) }) {
		t.Error("rows are not in ascending minute")
	}
}

// TestRecordInvalidKeyPanicsInTests shows both behaviours of an invalid Record call: a panic in a
// test binary, a drop counted as invalid_key otherwise.
func TestRecordInvalidKeyPanicsInTests(t *testing.T) {
	t.Parallel()
	r, _, m := newMeasurement(t, 10)
	func() {
		defer func() {
			if p := recover(); p == nil {
				t.Error("an unknown dimension did not panic in a test binary")
			}
		}()
		r.Record(measure.ToolCalls, measure.Key{Dimension: "grep"}, measure.One())
	}()
	app.SetStrict(r, false)
	r.Record(measure.ToolCalls, measure.Key{Dimension: "grep"}, measure.One())
	if got := m.drops(app.ReasonInvalidKey); got != 1 || r.Len() != 0 {
		t.Fatalf("invalid_key drops = %d, keys %d, want 1 and 0", got, r.Len())
	}
}

// TestRecordDoesNotBlock shows that a flush blocked inside its database write does not delay
// Record: while the fake store holds the write, Record calls of old and new keys return and are
// kept for the next flush. BenchmarkRecord measures the cost of one call.
func TestRecordDoesNotBlock(t *testing.T) {
	t.Parallel()
	r, c, m := newMeasurement(t, 100)
	s := newFakeStore()
	s.entered, s.release = make(chan struct{}), make(chan struct{})
	f := newFlush(t, r, s, m, c)
	r.Record(measure.Catchups, measure.Key{}, measure.One())
	flushed := make(chan error, 1)
	go func() { flushed <- f.Flush(context.Background()) }()
	testkit.Receive(t, "the flush entered its write", s.entered)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 1000 {
			r.Record(measure.Catchups, measure.Key{}, measure.One())
			r.Record(measure.Resyncs, measure.Key{Dimension: "retention"}, measure.One())
		}
	}()
	testkit.Receive(t, "Record returned while the flush waits on the database", done)
	close(s.release)
	if err := testkit.Receive(t, "the flush", flushed); err != nil {
		t.Fatal(err)
	}
	if got := r.Len(); got != 2 {
		t.Fatalf("keys after the blocked flush = %d, want 2", got)
	}
	s.entered = nil
	if err := f.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := s.snapshot()
	if c := got[rowKey(app.MinuteRow{BucketStart: t0, Metric: "catchups"})]; c.Count != 1001 {
		t.Fatalf("catchups = %+v, want 1001", c)
	}
}

// BenchmarkRecord measures one Record call of an existing key and of new keys, with other
// goroutines recording at the same time.
func BenchmarkRecord(b *testing.B) {
	c := clock.NewFake(t0)
	r, err := app.NewMeasurement(c, 1<<20, nil)
	if err != nil {
		b.Fatal(err)
	}
	ch := ids.New()
	key := measure.Key{ChannelID: ch, Dimension: "human"}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.Record(measure.MessagesCreated, key, measure.One())
		}
	})
}

// TestFlushDropsOldestOverQueueMax is VZ-T04 for the bound: beyond stats.flush_queue_max keys the
// oldest minute loses keys first and bothy.stats.flush.dropped counts them as queue_full; the
// newest keys are flushed.
func TestFlushDropsOldestOverQueueMax(t *testing.T) {
	t.Parallel()
	r, c, m := newMeasurement(t, 3)
	r.Record(measure.Resyncs, measure.Key{Dimension: "retention"}, measure.One())
	r.Record(measure.Resyncs, measure.Key{Dimension: "cursor_ahead"}, measure.One())
	c.Advance(time.Minute)
	r.Record(measure.Resyncs, measure.Key{Dimension: "retention"}, measure.One())
	r.Record(measure.Resyncs, measure.Key{Dimension: "retention"}, measure.One()) // same key: no new key
	c.Advance(time.Minute)
	r.Record(measure.Catchups, measure.Key{}, measure.One())
	r.Record(measure.SlowConsumerCloses, measure.Key{}, measure.One())
	if got := m.drops(app.ReasonQueueFull); got != 2 || r.Len() != 3 {
		t.Fatalf("queue_full = %d, keys %d, want 2 and 3", got, r.Len())
	}
	s := newFakeStore()
	if err := newFlush(t, r, s, m, c).Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := s.snapshot()
	for _, k := range []app.MinuteRow{
		{BucketStart: t0.Add(time.Minute), Metric: "resyncs", Dimension: "retention"},
		{BucketStart: t0.Add(2 * time.Minute), Metric: "catchups"},
		{BucketStart: t0.Add(2 * time.Minute), Metric: "slow_consumer_closes"},
	} {
		if _, ok := got[rowKey(k)]; !ok {
			t.Errorf("newer key %s was not flushed: %v", rowKey(k), got)
		}
	}
	if len(got) != 3 {
		t.Fatalf("flushed %d rows, want the 3 newest", len(got))
	}
}

// TestFlushDatabaseDown is VZ-T04 for an unreachable database: Record goes on, the failed flush
// keeps the counters for the next flush, and when they no longer fit beside new keys the oldest
// are dropped and counted as db_error; the next flush after the database is back writes what was
// kept, exactly once.
func TestFlushDatabaseDown(t *testing.T) {
	t.Parallel()
	r, c, m := newMeasurement(t, 3)
	s := newFakeStore()
	f := newFlush(t, r, s, m, c)
	r.Record(measure.Catchups, measure.Key{}, measure.Timed(time.Second))
	r.Record(measure.Resyncs, measure.Key{Dimension: "retention"}, measure.One())
	down := errors.New("connection refused")
	s.setErr(down)
	if err := f.Flush(context.Background()); !errors.Is(err, down) {
		t.Fatalf("Flush = %v, want the database error", err)
	}
	if r.Len() != 2 || m.flushCount(false) != 1 {
		t.Fatalf("after the failed flush: %d keys, %d failed flushes", r.Len(), m.flushCount(false))
	}
	c.Advance(time.Minute)
	r.Record(measure.Catchups, measure.Key{}, measure.Timed(time.Second))
	r.Record(measure.SlowConsumerCloses, measure.Key{}, measure.One())
	if got := m.drops(app.ReasonDBError); got != 1 || r.Len() != 3 {
		t.Fatalf("db_error drops = %d, keys %d, want 1 and 3", got, r.Len())
	}
	if m.drops(app.ReasonQueueFull) != 0 {
		t.Fatal("a kept counter of a failed write was counted as queue_full")
	}
	s.setErr(nil)
	if err := f.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := s.snapshot()
	if len(got) != 3 || m.flushCount(true) != 1 {
		t.Fatalf("after recovery: rows %v, ok flushes %d", got, m.flushCount(true))
	}
	if c := got[rowKey(app.MinuteRow{BucketStart: t0.Add(time.Minute), Metric: "catchups"})]; c != (domain.Counters{Count: 1, Sum: 1000}) {
		t.Fatalf("catchups of the second minute = %+v", c)
	}
}

// TestFlushChunksAndCreatesMissingPartitions shows a flush of more keys than one chunk across two
// days without partitions: each day's partition is created once on demand and every row is
// written once.
func TestFlushChunksAndCreatesMissingPartitions(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 3, 10, 23, 59, 0, 0, time.UTC)
	c := clock.NewFake(start)
	m := newFakeFlushMetrics()
	r, err := app.NewMeasurement(c, 10*app.FlushChunkRows, m)
	if err != nil {
		t.Fatal(err)
	}
	n := app.FlushChunkRows + 5
	for i := range n {
		r.Record(measure.MessagesCreated, measure.Key{ChannelID: ids.New(), Dimension: "app"}, measure.One())
		if i == n/2 {
			c.Advance(time.Minute)
		}
	}
	s := newFakeStore()
	s.partitions = map[time.Time]bool{}
	if err := newFlush(t, r, s, m, c).Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := len(s.snapshot()); got != n {
		t.Fatalf("rows = %d, want %d", got, n)
	}
	wantDays := []time.Time{domain.Day(start), domain.Day(start).AddDate(0, 0, 1)}
	if !slices.EqualFunc(s.ensured, wantDays, time.Time.Equal) {
		t.Fatalf("partitions created %v, want %v", s.ensured, wantDays)
	}
}

// TestShutdownFlushesOnce shows the shutdown flush: one write of every key, nothing left behind;
// and when the database is gone, every key held is dropped and counted as db_error.
func TestShutdownFlushesOnce(t *testing.T) {
	t.Parallel()
	r, c, m := newMeasurement(t, 10)
	s := newFakeStore()
	f := newFlush(t, r, s, m, c)
	r.Record(measure.Catchups, measure.Key{}, measure.One())
	r.Record(measure.Resyncs, measure.Key{Dimension: "epoch_changed"}, measure.One())
	if err := f.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.calls != 1 || len(s.snapshot()) != 2 || r.Len() != 0 {
		t.Fatalf("shutdown: %d writes, %d rows, %d keys left", s.calls, len(s.snapshot()), r.Len())
	}
	r.Record(measure.Catchups, measure.Key{}, measure.One())
	s.setErr(errors.New("database gone"))
	if err := f.Shutdown(context.Background()); err == nil {
		t.Fatal("a failed shutdown flush returned no error")
	}
	if got := m.drops(app.ReasonDBError); got != 1 || r.Len() != 0 {
		t.Fatalf("after the failed shutdown flush: db_error %d, keys %d", got, r.Len())
	}
}

// fakePartitions is a PartitionStore in memory.
type fakePartitions struct {
	days        map[time.Time]bool
	lockTimeout time.Duration
}

func (p *fakePartitions) EnsurePartition(_ context.Context, day time.Time) error {
	p.days[day] = true
	return nil
}

func (p *fakePartitions) DropExpired(_ context.Context, cutoff time.Time, lockTimeout time.Duration) (int, error) {
	p.lockTimeout = lockTimeout
	n := 0
	for d := range p.days {
		if !d.AddDate(0, 0, 1).After(cutoff) {
			delete(p.days, d)
			n++
		}
	}
	return n, nil
}

// TestRetentionUseCase shows one run of stats.retention at a fake clock: today's and tomorrow's
// partition exist afterwards, and the partitions whose day ended at or before now minus the
// period are dropped (the database side is TestRetention in the integration tests).
func TestRetentionUseCase(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	p := &fakePartitions{days: map[time.Time]bool{}}
	for i := range 6 {
		p.days[time.Date(2026, 3, 4+i, 0, 0, 0, 0, time.UTC)] = true
	}
	ret, err := app.NewRetention(clock.NewFake(now), p, 72*time.Hour, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ret.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Dropped != 3 || p.lockTimeout != 5*time.Second {
		t.Fatalf("dropped %d with lock timeout %v, want 3 (days 4 to 6) and 5s", res.Dropped, p.lockTimeout)
	}
	for d := 7; d <= 11; d++ {
		if !p.days[time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC)] {
			t.Errorf("day %d is missing", d)
		}
	}
	if len(p.days) != 5 {
		t.Fatalf("days = %v", p.days)
	}
}
