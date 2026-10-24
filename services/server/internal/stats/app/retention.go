// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/stats/domain"
)

// PartitionStore is the outgoing port of the job stats.retention (adapters/postgres).
type PartitionStore interface {
	// EnsurePartition creates the partition of the UTC day of day unless it exists.
	EnsurePartition(ctx context.Context, day time.Time) error
	// DropExpired detaches and drops every partition whose UTC day ends at or before cutoff,
	// waiting at most lockTimeout for the lock of a detach, and returns how many it dropped.
	DropExpired(ctx context.Context, cutoff time.Time, lockTimeout time.Duration) (int, error)
}

// RetentionResult is what one run of stats.retention did.
type RetentionResult struct {
	// Dropped is the number of partitions detached and dropped.
	Dropped int
}

// Retention is the use case of the job stats.retention (25 3.5, 20 retention class stats): it
// creates the partitions of today and tomorrow (UTC) when missing and drops the partitions older
// than retention.stats_minutes. A repeated run is harmless.
type Retention struct {
	clock       clock.Clock
	store       PartitionStore
	period      time.Duration
	lockTimeout time.Duration
}

// NewRetention returns the use case over store with the clock c, the period retention.stats_minutes
// and the lock timeout of a detach (database.lock_timeout). Errors: a missing collaborator or a
// period or timeout that is not positive.
func NewRetention(c clock.Clock, store PartitionStore, period, lockTimeout time.Duration) (*Retention, error) {
	if c == nil || store == nil || period <= 0 || lockTimeout <= 0 {
		return nil, errors.New("stats: the retention needs a clock, the store and a positive retention.stats_minutes and lock timeout")
	}
	return &Retention{clock: c, store: store, period: period, lockTimeout: lockTimeout}, nil
}

// Run creates today's and tomorrow's partition and drops every partition whose day ends at or
// before now - retention.stats_minutes (VZ-T15). Errors: those of the store; River retries the job.
func (r *Retention) Run(ctx context.Context) (RetentionResult, error) {
	now := r.clock.Now()
	today := domain.Day(now)
	for _, day := range []time.Time{today, today.AddDate(0, 0, 1)} {
		if err := r.store.EnsurePartition(ctx, day); err != nil {
			return RetentionResult{}, err
		}
	}
	n, err := r.store.DropExpired(ctx, domain.RetentionCutoff(now, r.period), r.lockTimeout)
	if err != nil {
		return RetentionResult{}, err
	}
	return RetentionResult{Dropped: n}, nil
}
