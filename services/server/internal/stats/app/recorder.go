// SPDX-License-Identifier: Apache-2.0

package app

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
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// The values of bothy.reason on bothy.stats.flush.dropped (signals.md 4.3b).
const (
	// ReasonQueueFull drops the oldest keys when more than stats.flush_queue_max are held.
	ReasonQueueFull = "queue_full"
	// ReasonDBError drops keys of a failed write that could not be kept: over the bound after the
	// merge back, or left over when the shutdown flush failed.
	ReasonDBError = "db_error"
	// ReasonInvalidKey drops a Record call outside the closed lists of 25 3.2 (production only;
	// a test binary panics instead).
	ReasonInvalidKey = "invalid_key"
)

// FlushMetrics is the outgoing port of the operating values of the own measurement (signals.md
// 4.3b); adapters/otelmetrics implements it. Calls never block.
type FlushMetrics interface {
	// Dropped counts n discarded keys with reason (bothy.stats.flush.dropped).
	Dropped(ctx context.Context, reason string, n int)
	// Flushed records one flush that took d, ok or failed (bothy.stats.flush.duration).
	Flushed(ctx context.Context, d time.Duration, ok bool)
}

// NopFlushMetrics records nothing.
type NopFlushMetrics struct{}

// Dropped implements FlushMetrics.
func (NopFlushMetrics) Dropped(context.Context, string, int) {}

// Flushed implements FlushMetrics.
func (NopFlushMetrics) Flushed(context.Context, time.Duration, bool) {}

// MinuteRow is one row of stats_minutes as a flush adds it (03 4.15): the key and the counters to
// add. A zero ChannelID or AgentID and an empty Currency are NULL.
type MinuteRow struct {
	BucketStart time.Time
	Metric      string
	ChannelID   ids.ID
	AgentID     ids.ID
	Dimension   string
	Currency    string
	domain.Counters
}

// cell is the key of one counter within a minute.
type cell struct {
	metric measure.Metric
	key    measure.Key
}

// entry is the counters of one cell; retried marks counters a failed write merged back, so they
// count as db_error when they are dropped later.
type entry struct {
	domain.Counters
	retried bool
}

// Measurement implements measure.Recorder (25 3.1, 3.4): the counters of one process per key
// (minute, metric, channel, agent, dimension, currency) in memory. Record takes the mutex for one
// map update; Flush (flush.go) swaps the counters out under it and writes outside it, so a flush
// that waits on the database never delays a Record. At most stats.flush_queue_max keys are held;
// beyond that the oldest minute loses keys first, counted as dropped (R-VZ-04).
type Measurement struct {
	clock   clock.Clock
	max     int
	metrics FlushMetrics
	// strict panics on an invalid key: true in a test binary, so a measuring point outside the
	// closed lists fails its test instead of becoming a counted drop.
	strict bool

	mu      sync.Mutex
	minutes map[int64]map[cell]entry
	order   []int64
	size    int
}

// Compile-time check: Measurement is the kernel port.
var _ measure.Recorder = (*Measurement)(nil)

// NewMeasurement returns the recorder of a process with the clock c (the minute of a Record),
// at most maxKeys keys (stats.flush_queue_max) and the operating metrics m (nil records none).
// Errors: a nil clock or maxKeys below 1.
func NewMeasurement(c clock.Clock, maxKeys int, m FlushMetrics) (*Measurement, error) {
	if c == nil || maxKeys < 1 {
		return nil, errors.New("stats: the measurement needs a clock and stats.flush_queue_max of at least 1")
	}
	if m == nil {
		m = NopFlushMetrics{}
	}
	return &Measurement{clock: c, max: maxKeys, metrics: m, strict: testing.Testing(), minutes: map[int64]map[cell]entry{}}, nil
}

// Record implements measure.Recorder: it adds value to the counters of metric and key in the
// current UTC minute of the clock. An invalid call (measure.Validate) panics in a test binary and
// is dropped with reason invalid_key otherwise. A new key beyond stats.flush_queue_max first drops
// a key of the oldest minute (queue_full, or db_error for counters of a failed write).
func (r *Measurement) Record(metric measure.Metric, key measure.Key, value measure.Value) {
	if err := measure.Validate(metric, key, value); err != nil {
		if r.strict {
			panic(err)
		}
		r.metrics.Dropped(context.Background(), ReasonInvalidKey, 1)
		return
	}
	c := domain.Event(string(metric), value.Count, value.Sum, value.Elapsed, value.OK)
	minute := domain.Minute(r.clock.Now()).Unix()
	r.mu.Lock()
	var d drops
	r.addLocked(minute, cell{metric: metric, key: key}, entry{Counters: c}, true, &d)
	r.mu.Unlock()
	d.report(r.metrics)
}

// drops counts the keys dropped under the mutex, reported after it is released.
type drops struct{ queueFull, dbError int }

// report counts the drops in bothy.stats.flush.dropped.
func (d drops) report(m FlushMetrics) {
	if d.queueFull > 0 {
		m.Dropped(context.Background(), ReasonQueueFull, d.queueFull)
	}
	if d.dbError > 0 {
		m.Dropped(context.Background(), ReasonDBError, d.dbError)
	}
}

// addLocked adds e to the cell of minute. A new cell with bound set first makes room by dropping
// the oldest keys into d. The caller holds r.mu.
func (r *Measurement) addLocked(minute int64, c cell, e entry, bound bool, d *drops) {
	if b := r.minutes[minute]; b != nil {
		if cur, ok := b[c]; ok {
			cur.Counters = cur.Add(e.Counters)
			cur.retried = cur.retried || e.retried
			b[c] = cur
			return
		}
	}
	for bound && r.size >= r.max {
		r.evictOldestLocked(d)
	}
	b := r.minutes[minute]
	if b == nil {
		b = map[cell]entry{}
		r.minutes[minute] = b
		i, _ := slices.BinarySearch(r.order, minute)
		r.order = slices.Insert(r.order, i, minute)
	}
	b[c] = e
	r.size++
}

// evictOldestLocked drops one key of the oldest minute into d. The caller holds r.mu and r.size
// is positive.
func (r *Measurement) evictOldestLocked(d *drops) {
	minute := r.order[0]
	b := r.minutes[minute]
	for c, e := range b {
		delete(b, c)
		r.size--
		if e.retried {
			d.dbError++
		} else {
			d.queueFull++
		}
		break
	}
	if len(b) == 0 {
		delete(r.minutes, minute)
		r.order = r.order[1:]
	}
}

// take swaps the counters out and returns them as rows in ascending minute.
func (r *Measurement) take() []MinuteRow {
	r.mu.Lock()
	minutes, order := r.minutes, r.order
	r.minutes, r.order, r.size = map[int64]map[cell]entry{}, nil, 0
	r.mu.Unlock()
	var rows []MinuteRow
	for _, m := range order {
		start := time.Unix(m, 0).UTC()
		for c, e := range minutes[m] {
			rows = append(rows, MinuteRow{
				BucketStart: start, Metric: string(c.metric), ChannelID: c.key.ChannelID, AgentID: c.key.AgentID,
				Dimension: c.key.Dimension, Currency: c.key.Currency, Counters: e.Counters,
			})
		}
	}
	return rows
}

// restore merges rows of a failed write back, marked as retried, then drops the oldest keys while
// more than stats.flush_queue_max are held (25 3.4 item 3).
func (r *Measurement) restore(rows []MinuteRow) {
	var d drops
	r.mu.Lock()
	for _, row := range rows {
		c := cell{metric: measure.Metric(row.Metric), key: measure.Key{
			ChannelID: row.ChannelID, AgentID: row.AgentID, Dimension: row.Dimension, Currency: row.Currency,
		}}
		r.addLocked(row.BucketStart.Unix(), c, entry{Counters: row.Counters, retried: true}, false, &d)
	}
	for r.size > r.max {
		r.evictOldestLocked(&d)
	}
	r.mu.Unlock()
	d.report(r.metrics)
}

// dropAll discards every key held and counts them with reason.
func (r *Measurement) dropAll(reason string) {
	r.mu.Lock()
	n := r.size
	r.minutes, r.order, r.size = map[int64]map[cell]entry{}, nil, 0
	r.mu.Unlock()
	if n > 0 {
		r.metrics.Dropped(context.Background(), reason, n)
	}
}

// Len returns the number of keys held, for tests and the flush.
func (r *Measurement) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.size
}
