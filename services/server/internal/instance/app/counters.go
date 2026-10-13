// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// counters implements instance.Counters with the phase guard of 02 "Lock order".
type counters struct {
	repo Repository
}

// Compile-time check: counters is the port.
var _ instance.Counters = (*counters)(nil)

// NewCounters returns the implementation of instance.Counters over repo. internal/wire calls it
// once and passes the result only to the constructors of the phase hooks of authz, sync and
// audit; TestCountersOnlyUsedByPhaseHooks holds that rule.
func NewCounters(repo Repository) instance.Counters {
	return &counters{repo: repo}
}

// Advance checks name, n and the phase of the unit of work in ctx (domain.CheckAdvance) before
// any SQL, then locks and raises the counter (instance.Counters.Advance).
func (c *counters) Advance(ctx context.Context, name instance.CounterName, n int64) (int64, error) {
	if err := domain.CheckAdvance(string(name), txn.CurrentPhase(ctx).String()); err != nil {
		return 0, err
	}
	if err := domain.CheckAdvanceCount(n); err != nil {
		return 0, err
	}
	return c.repo.AdvanceCounter(ctx, string(name), n)
}

// Current checks name and phase (domain.CheckRead), then reads the value without a lock
// (instance.Counters.Current).
func (c *counters) Current(ctx context.Context, name instance.CounterName) (int64, error) {
	if err := domain.CheckRead(string(name), txn.CurrentPhase(ctx).String()); err != nil {
		return 0, err
	}
	return c.repo.CounterValue(ctx, string(name))
}
