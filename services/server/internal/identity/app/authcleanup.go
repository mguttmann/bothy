// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// AuthCleanup is the use case of the job identity.auth_cleanup (docs/spec/02-architektur.md
// section 12.3): it deletes the challenges past expires_at. M1-38 adds pairings and device expiry,
// M1-41 adds auth_login_flows. Requests never depend on it, because every use case checks expiry
// itself. It is safe for concurrent use; a repeated run is harmless.
type AuthCleanup struct {
	cfg   Config
	uow   txn.UnitOfWork
	store ChallengeStore
	clock clock.Clock
}

// NewAuthCleanup returns the use case with auth.cleanup_interval and auth.login_fail_window of cfg
// above 0 and the clock c (nil means clock.System()). Errors: a missing collaborator or a duration
// that is not positive.
func NewAuthCleanup(cfg Config, uow txn.UnitOfWork, store ChallengeStore, c clock.Clock) (*AuthCleanup, error) {
	if uow == nil || store == nil {
		return nil, errors.New("identity: NewAuthCleanup needs the unit of work and the challenge store")
	}
	if cfg.CleanupInterval <= 0 || cfg.LoginFailWindow <= 0 {
		return nil, errors.New("identity: auth.cleanup_interval and auth.login_fail_window must be positive")
	}
	if c == nil {
		c = clock.System()
	}
	return &AuthCleanup{cfg: cfg, uow: uow, store: store, clock: c}, nil
}

// Run deletes in one unit of work every challenge whose expires_at has passed at the time of the
// clock, except failed step-up challenges that still count inside auth.login_fail_window (08
// section 13), and returns how many it deleted. Errors: those of the unit of work.
func (c *AuthCleanup) Run(ctx context.Context) (int64, error) {
	now := c.clock.Now()
	var n int64
	err := c.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		n, err = c.store.DeleteExpiredChallenges(ctx, now, now.Add(-c.cfg.LoginFailWindow))
		return err
	})
	return n, err
}
