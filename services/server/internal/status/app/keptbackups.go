// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// KeptRunsReader reads the succeeded backup runs with the journal position of their manifest
// (adapters/postgres).
type KeptRunsReader interface {
	// SucceededRuns returns every succeeded run of the four backup kinds.
	SucceededRuns(ctx context.Context) ([]domain.KeptRun, error)
}

// KeptBackups implements status.KeptBackups over the runs of backup_runs and the retention rules
// of the backups (M7-07). It is safe for concurrent use.
type KeptBackups struct {
	keep  domain.Keep
	runs  KeptRunsReader
	clock clock.Clock
}

// Compile-time check.
var _ status.KeptBackups = (*KeptBackups)(nil)

// NewKeptBackups returns the port over runs with the rules keep; c nil means clock.System().
// Errors: a missing reader or a rule that is not positive.
func NewKeptBackups(keep domain.Keep, runs KeptRunsReader, c clock.Clock) (*KeptBackups, error) {
	if runs == nil || keep.Logical <= 0 || keep.Objects <= 0 || keep.FullCount < 1 {
		return nil, errors.New("status: NewKeptBackups needs the reader and positive retention rules")
	}
	if c == nil {
		c = clock.System()
	}
	return &KeptBackups{keep: keep, runs: runs, clock: c}, nil
}

// JournalFloor implements status.KeptBackups with domain.JournalFloor at the time of the clock.
func (k *KeptBackups) JournalFloor(ctx context.Context) (int64, bool, error) {
	runs, err := k.runs.SucceededRuns(ctx)
	if err != nil {
		return 0, false, err
	}
	floor, limited := domain.JournalFloor(runs, k.keep, k.clock.Now())
	return floor, limited, nil
}
