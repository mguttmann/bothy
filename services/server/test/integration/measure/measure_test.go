// SPDX-License-Identifier: Apache-2.0

// Package measure_test holds the integration tests of the own measurement (M6-13) against
// PostgreSQL 18 (pgtest): the measurement of internal/wire (NewMeasure) with its flush, the
// additive upsert and the job stats.retention.
package measure_test

import (
	"context"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// cfg is the configuration of the measurements of the tests: the defaults of 25 15 and 20.
var cfg = wire.MeasureConfig{FlushInterval: 10 * time.Second, QueueMax: 20000, Retention: 90 * 24 * time.Hour, LockTimeout: 5 * time.Second}

// newMeasure builds the own measurement of one process over the app pool of db with clock c.
func newMeasure(t *testing.T, pool *pg.Pool, c clock.Clock) *wire.Measure {
	t.Helper()
	m, err := wire.NewMeasure(cfg, pool, c, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// row is one row of stats_minutes as the tests read it.
type row struct {
	bucket            time.Time
	metric, dimension string
	channel, agent    *string
	currency          *string
	count, sum, good  int64
}

// rows reads stats_minutes ordered by its key.
func rows(t *testing.T, pool *pg.Pool) []row {
	t.Helper()
	r, err := pool.Query(context.Background(), `SELECT bucket_start, metric, dimension, channel_id::text, agent_id::text, currency, count, sum, good
		FROM stats_minutes ORDER BY bucket_start, metric, dimension, channel_id, agent_id, currency`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var out []row
	for r.Next() {
		var x row
		if err := r.Scan(&x.bucket, &x.metric, &x.dimension, &x.channel, &x.agent, &x.currency, &x.count, &x.sum, &x.good); err != nil {
			t.Fatal(err)
		}
		out = append(out, x)
	}
	if err := r.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// everyMetric records one event of every metric of 25 3.2 into rec, with valid keys: the channel
// and agent where the metric has them, the first dimension of its list and EUR for cost. An SLI
// takes elapsed with a good outcome.
func everyMetric(rec measure.Recorder, ch, ag ids.ID, elapsed time.Duration) {
	for _, m := range measure.Metrics() {
		k := measure.Key{Dimension: measure.Dimensions(m)[0]}
		scope, _ := measure.ScopeOf(m)
		if scope == measure.ScopeChannel || scope == measure.ScopeChannelAgent {
			k.ChannelID = ch
		}
		if scope == measure.ScopeAgent || scope == measure.ScopeChannelAgent {
			k.AgentID = ag
		}
		v := measure.Timed(elapsed)
		switch {
		case m == measure.Cost:
			k.Currency, v = "EUR", measure.Amount(2500)
		case m.IsSLI():
			v = measure.SLI(elapsed, true)
		}
		rec.Record(m, k, v)
	}
}

// TestMeasure_TwoProcessesAddUp is VZ-T01 for the store: two measurements (two processes) with
// the same fake clock record the same keys of all 18 metrics and flush into one database; every
// key is one row holding the sum of both, the minute is the clock's, the partition of the day is
// created on demand, and good never exceeds count. A second round in the next minute is a second
// row per key.
func TestMeasure_TwoProcessesAddUp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	start := time.Date(2026, 1, 10, 12, 34, 56, 0, time.UTC)
	c := clock.NewFake(start)
	a, b := newMeasure(t, pool, c), newMeasure(t, pool, c)
	ch, ag := ids.New(), ids.New()
	everyMetric(a.Recorder, ch, ag, 300*time.Millisecond)
	everyMetric(b.Recorder, ch, ag, 900*time.Second)
	if err := a.Flush.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	got := rows(t, pool)
	if len(got) != len(measure.Metrics()) {
		t.Fatalf("%d rows, want one per metric (%d)", len(got), len(measure.Metrics()))
	}
	minute := time.Date(2026, 1, 10, 12, 34, 0, 0, time.UTC)
	for _, r := range got {
		m := measure.Metric(r.metric)
		if !r.bucket.Equal(minute) || r.count != 2 || r.good > r.count {
			t.Errorf("%s: bucket %v count %d good %d", r.metric, r.bucket, r.count, r.good)
		}
		wantSum := int64(300 + 900000)
		if m == measure.Cost {
			wantSum = 5000
		}
		if r.sum != wantSum {
			t.Errorf("%s: sum %d, want %d", r.metric, r.sum, wantSum)
		}
		scope, _ := measure.ScopeOf(m)
		if (r.channel != nil) != (scope == measure.ScopeChannel || scope == measure.ScopeChannelAgent) ||
			(r.agent != nil) != (scope == measure.ScopeAgent || scope == measure.ScopeChannelAgent) ||
			(r.currency != nil) != (m == measure.Cost) {
			t.Errorf("%s: channel %v agent %v currency %v", r.metric, r.channel, r.agent, r.currency)
		}
		if m.IsSLI() {
			// 300 ms is within every threshold; 900 s is above the largest (turn duration).
			if r.good != 1 && r.good != 2 {
				t.Errorf("%s: good %d", r.metric, r.good)
			}
		} else if r.good != 0 {
			t.Errorf("%s: good %d outside an sli", r.metric, r.good)
		}
	}
	c.Advance(time.Minute)
	everyMetric(a.Recorder, ch, ag, 0)
	if err := a.Flush.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if got := len(rows(t, pool)); got != 2*len(measure.Metrics()) {
		t.Fatalf("after the next minute %d rows, want %d", got, 2*len(measure.Metrics()))
	}
}

// partitions returns the partitions of stats_minutes by name.
func partitions(t *testing.T, pool *pg.Pool) []string {
	t.Helper()
	r, err := pool.Query(context.Background(), `SELECT c.relname FROM pg_inherits i JOIN pg_class c ON c.oid = i.inhrelid
		WHERE i.inhparent = 'stats_minutes'::regclass ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var out []string
	for r.Next() {
		var n string
		if err := r.Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

// TestRetention is VZ-T15: with a fake clock at 2026-01-10 12:00 UTC, partitions for the 4th to the
// 9th and retention.stats_minutes of 72 hours, one run of stats.retention creates today's and
// tomorrow's partition and detaches and drops exactly the days that ended before the cutoff
// (2026-01-07 12:00): the 4th, 5th and 6th, with their rows; the 7th keeps its rows. A second run
// changes nothing.
func TestRetention(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	c := clock.NewFake(time.Date(2026, 1, 4, 23, 59, 0, 0, time.UTC))
	short := cfg
	short.Retention = 72 * time.Hour
	m, err := wire.NewMeasure(short, pool, c, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	for c.Now().Before(now) {
		m.Recorder.Record(measure.Catchups, measure.Key{}, measure.One())
		c.Advance(24 * time.Hour)
	}
	if err := m.Flush.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if got := partitions(t, pool); len(got) != 6 {
		t.Fatalf("partitions before the job %q, want the 4th to the 9th", got)
	}
	c.Set(now)
	for range 2 {
		res, err := m.Retention.Run(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got := partitions(t, pool); !slices.Equal(got, []string{
			"stats_minutes_20260107", "stats_minutes_20260108", "stats_minutes_20260109", "stats_minutes_20260110", "stats_minutes_20260111",
		}) {
			t.Fatalf("partitions after the job %q (dropped %d)", got, res.Dropped)
		}
	}
	var n int
	var oldest time.Time
	if err := pool.QueryRow(ctx, `SELECT count(*), min(bucket_start) FROM stats_minutes`).Scan(&n, &oldest); err != nil {
		t.Fatal(err)
	}
	if n != 3 || !oldest.Equal(time.Date(2026, 1, 7, 23, 59, 0, 0, time.UTC)) {
		t.Fatalf("%d rows, oldest %v; want the rows of the 7th to the 9th", n, oldest)
	}
}
