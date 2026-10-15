// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// CleanupStore is what the job identity.auth_cleanup deletes: the challenges (M1-35) and the login
// flows (M1-41) past expires_at. adapters/postgres implements it.
type CleanupStore interface {
	ChallengeStore
	// DeleteExpiredLoginFlows deletes the login flows whose expires_at lies before now.
	DeleteExpiredLoginFlows(ctx context.Context, now time.Time) (int64, error)
}

// AuthCleanup is the use case of the job identity.auth_cleanup (docs/spec/02-architecture.md
// section 12.3): it deletes the challenges and the OIDC login flows past expires_at (M1-41) and,
// since M1-38, revokes the expired pending devices and expires the pairings past expires_at
// (Expiry). Requests never depend on it, because every use case checks expiry itself. It is safe
// for concurrent use; a repeated run is harmless.
type AuthCleanup struct {
	cfg    Config
	uow    txn.UnitOfWork
	store  CleanupStore
	expiry Expiry
	clock  clock.Clock
}

// Expiry is the device and pairing expiry the cleanup runs (Service implements it, M1-38).
type Expiry interface {
	// ExpirePendingDevices revokes the pending devices past auth.device_pending_ttl.
	ExpirePendingDevices(ctx context.Context) (int64, error)
	// ExpirePairings expires the pairings past expires_at.
	ExpirePairings(ctx context.Context) (int64, error)
}

// NewAuthCleanup returns the use case with auth.cleanup_interval and auth.login_fail_window of cfg
// above 0, the expiry of devices and pairings and the clock c (nil means clock.System()). Errors: a
// missing collaborator or a duration that is not positive.
func NewAuthCleanup(cfg Config, uow txn.UnitOfWork, store CleanupStore, expiry Expiry, c clock.Clock) (*AuthCleanup, error) {
	if uow == nil || store == nil || expiry == nil {
		return nil, errors.New("identity: NewAuthCleanup needs the unit of work, the cleanup store and the expiry")
	}
	if cfg.CleanupInterval <= 0 || cfg.LoginFailWindow <= 0 {
		return nil, errors.New("identity: auth.cleanup_interval and auth.login_fail_window must be positive")
	}
	if c == nil {
		c = clock.System()
	}
	return &AuthCleanup{cfg: cfg, uow: uow, store: store, expiry: expiry, clock: c}, nil
}

// Run deletes in one unit of work every challenge whose expires_at has passed at the time of the
// clock, except failed step-up challenges that still count inside auth.login_fail_window (08
// section 13) and every login flow past its expires_at, then revokes the expired pending devices
// and expires the due pairings, each in its own unit of work. It returns how many rows it ended.
// Errors: the first failing step; the steps before it stay committed and a retry continues.
func (c *AuthCleanup) Run(ctx context.Context) (int64, error) {
	now := c.clock.Now()
	var n int64
	err := c.uow.Do(ctx, func(ctx context.Context) error {
		challenges, err := c.store.DeleteExpiredChallenges(ctx, now, now.Add(-c.cfg.LoginFailWindow))
		if err != nil {
			return err
		}
		flows, err := c.store.DeleteExpiredLoginFlows(ctx, now)
		n = challenges + flows
		return err
	})
	if err != nil {
		return n, err
	}
	devices, err := c.expiry.ExpirePendingDevices(ctx)
	n += devices
	if err != nil {
		return n, err
	}
	pairings, err := c.expiry.ExpirePairings(ctx)
	return n + pairings, err
}
