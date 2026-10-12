// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// PartitionConfig is the configuration of the partition maintenance (03 event_log, 02
// "Konfigurationswerte dieses Kapitels").
type PartitionConfig struct {
	// Size is sync.event_log_partition_size: sequence numbers per partition.
	Size int64 `config:"sync.event_log_partition_size"`
	// Ahead is sync.event_log_partitions_ahead: partitions that stand ready above the head.
	Ahead int `config:"sync.event_log_partitions_ahead"`
	// Retention is sync.event_retention: older partitions are detached.
	Retention time.Duration `config:"sync.event_retention"`
	// LockTimeout is database.lock_timeout: the longest DETACH waits for its lock.
	LockTimeout time.Duration `config:"database.lock_timeout"`
}

// MaintenanceResult is the outcome of one run of the partition maintenance.
type MaintenanceResult struct {
	// Head is max(seq) when the run started.
	Head int64
	// Created is the number of partitions created.
	Created int
	// Dropped is the number of partitions detached and dropped.
	Dropped int
}

// PartitionMaintenance keeps partitions of event_log ahead of the head and detaches expired
// ones (job sync.partition_maintenance, and once in bothy migrate up, upgrade.md rule M10). It is
// safe for concurrent use; two runs at once serialise in the database.
type PartitionMaintenance struct {
	cfg   PartitionConfig
	uow   txn.UnitOfWork
	parts Partitions
	clock clock.Clock
}

// NewPartitionMaintenance returns the use case. The error reports a size below 1, a negative
// ahead, or a retention or lock timeout that is not positive.
func NewPartitionMaintenance(cfg PartitionConfig, uow txn.UnitOfWork, parts Partitions, c clock.Clock) (*PartitionMaintenance, error) {
	switch {
	case cfg.Size < 1:
		return nil, errors.New("sync: sync.event_log_partition_size must be at least 1")
	case cfg.Ahead < 0:
		return nil, errors.New("sync: sync.event_log_partitions_ahead must not be negative")
	case cfg.Retention <= 0:
		return nil, errors.New("sync: sync.event_retention must be positive")
	case cfg.LockTimeout <= 0:
		return nil, errors.New("sync: database.lock_timeout must be positive")
	}
	return &PartitionMaintenance{cfg: cfg, uow: uow, parts: parts, clock: c}, nil
}

// ErrDropDeferred wraps the error of the drop step: the expired partitions stay until a later
// run, while the partitions created ahead are committed already.
var ErrDropDeferred = errors.New("sync: dropping expired partitions is deferred to a later run")

// Run reads the head and creates partitions up to Ahead partitions above it in one unit of work,
// then drops the partitions whose events are all older than now minus Retention, never the one
// holding the head (03 event_log "Aufbewahrung"), in a second one; neither appends an event. The
// two units keep a DETACH that cannot get its lock (a long reader holds event_log) from rolling
// back the partitions ahead. The time comes from the clock, so tests move the cutoff instead of
// rewriting rows.
//
// Errors: those of the unit of work and of Partitions for the first unit; for the drop unit a
// lock timeout of DETACH after the retries of the unit of work (kind unavailable) wrapped in
// ErrDropDeferred, and any other error unchanged, both with the result of the first unit.
func (m *PartitionMaintenance) Run(ctx context.Context) (MaintenanceResult, error) {
	var res MaintenanceResult
	err := m.uow.Do(ctx, func(ctx context.Context) error {
		res = MaintenanceResult{}
		head, err := m.parts.Head(ctx)
		if err != nil {
			return err
		}
		res.Head = head
		res.Created, err = m.parts.Ensure(ctx, m.cfg.Size, m.cfg.Ahead, head)
		return err
	})
	if err != nil {
		return MaintenanceResult{}, err
	}
	err = m.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		res.Dropped, err = m.parts.DropExpired(ctx, m.clock.Now().Add(-m.cfg.Retention), res.Head, m.cfg.LockTimeout)
		return err
	})
	switch {
	case err == nil:
		return res, nil
	case errs.KindOf(err) == errs.KindUnavailable:
		// A lock timeout after the retries of the unit of work: a long reader holds event_log.
		res.Dropped = 0
		return res, fmt.Errorf("%w: %w", ErrDropDeferred, err)
	default:
		res.Dropped = 0
		return res, err
	}
}
