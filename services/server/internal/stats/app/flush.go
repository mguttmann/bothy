// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// ErrNoPartition is returned by MinuteStore.Upsert when a row falls into a UTC day without a
// partition of stats_minutes (SQLSTATE 23514 of a partitioned insert).
var ErrNoPartition = errors.New("stats: stats_minutes has no partition for a day of the rows")

// MinuteStore is the outgoing port of the flush (adapters/postgres).
type MinuteStore interface {
	// Upsert adds rows to stats_minutes in one statement, its own short transaction, never the
	// transaction of a domain unit of work: an existing row of the same key adds count, sum and
	// good (25 3.4). ErrNoPartition when a day has no partition.
	Upsert(ctx context.Context, rows []MinuteRow) error
	// EnsurePartition creates the partition of the UTC day of day unless it exists.
	EnsurePartition(ctx context.Context, day time.Time) error
}

// flushChunkRows bounds the rows of one upsert statement, so one statement and its transaction
// stay short however many keys stats.flush_queue_max admits. It is no operating limit: every
// chunk of a flush is written in turn.
const flushChunkRows = 1000

// Log message of a failed periodic flush and its one attribute; it carries the error kind only,
// never a key.
const (
	msgFlushFailed  = "stats flush failed, the counters stay for the next flush"
	logKeyErrorType = "error.type"
)

// Flush writes the counters of a Measurement to stats_minutes every stats.flush_interval and once
// at an orderly shutdown (25 3.4). It is safe for concurrent use; flushes run one at a time.
type Flush struct {
	rec      *Measurement
	store    MinuteStore
	metrics  FlushMetrics
	clock    clock.Clock
	interval time.Duration
	log      *slog.Logger

	run sync.Mutex
}

// NewFlush returns the flush of rec into store every interval (stats.flush_interval), with the
// operating metrics m (nil records none), the clock c for the flush duration and log for a failed
// periodic flush (nil means slog.Default()). Errors: a missing collaborator or an interval that is
// not positive.
func NewFlush(rec *Measurement, store MinuteStore, m FlushMetrics, c clock.Clock, interval time.Duration, log *slog.Logger) (*Flush, error) {
	if rec == nil || store == nil || c == nil || interval <= 0 {
		return nil, errors.New("stats: the flush needs the measurement, the store, a clock and a positive stats.flush_interval")
	}
	if m == nil {
		m = NopFlushMetrics{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Flush{rec: rec, store: store, metrics: m, clock: c, interval: interval, log: log}, nil
}

// Flush swaps the counters out and writes them in chunks of ascending minute, each chunk one
// upsert. A chunk that meets a day without a partition creates the partitions of its days and is
// written once more (docs/DECISIONS.md D-M6-13-6). On the first error the chunk and every later
// one go back into the measurement (bounded by stats.flush_queue_max, the oldest dropped first and
// counted as db_error), and the error is returned; the chunks written before stay written, so
// nothing is added twice. bothy.stats.flush.duration records every flush with keys.
func (f *Flush) Flush(ctx context.Context) error {
	f.run.Lock()
	defer f.run.Unlock()
	rows := f.rec.take()
	if len(rows) == 0 {
		return nil
	}
	start := f.clock.Now()
	for i := 0; i < len(rows); i += flushChunkRows {
		chunk := rows[i:min(i+flushChunkRows, len(rows))]
		if err := f.write(ctx, chunk); err != nil {
			f.rec.restore(rows[i:])
			f.metrics.Flushed(context.WithoutCancel(ctx), f.clock.Now().Sub(start), false)
			return err
		}
	}
	f.metrics.Flushed(context.WithoutCancel(ctx), f.clock.Now().Sub(start), true)
	return nil
}

// write upserts one chunk; ErrNoPartition creates the partitions of the chunk's days and retries
// once.
func (f *Flush) write(ctx context.Context, chunk []MinuteRow) error {
	err := f.store.Upsert(ctx, chunk)
	if !errors.Is(err, ErrNoPartition) {
		return err
	}
	var last time.Time
	for _, r := range chunk {
		day := domain.Day(r.BucketStart)
		if day.Equal(last) {
			continue
		}
		if err := f.store.EnsurePartition(ctx, day); err != nil {
			return err
		}
		last = day
	}
	return f.store.Upsert(ctx, chunk)
}

// Run flushes every stats.flush_interval until ctx ends. A failed flush is logged with its error
// kind; its counters stay for the next one.
func (f *Flush) Run(ctx context.Context) {
	t := time.NewTicker(f.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := f.Flush(ctx); err != nil && ctx.Err() == nil {
				f.log.LogAttrs(ctx, slog.LevelWarn, msgFlushFailed, slog.String(logKeyErrorType, errs.KindOf(err).String()))
			}
		}
	}
}

// Shutdown is the flush of an orderly shutdown (25 3.4 item 4): one flush within the deadline of
// ctx; when it fails, every key still held is dropped and counted as db_error, and the error is
// returned.
func (f *Flush) Shutdown(ctx context.Context) error {
	err := f.Flush(ctx)
	if err != nil {
		f.rec.dropAll(ReasonDBError)
	}
	return err
}
