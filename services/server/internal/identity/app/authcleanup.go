// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// CleanupStore is what the job identity.auth_cleanup deletes: the challenges (M1-35), the login
// flows (M1-41) and the closed pairings (M7-05) past expires_at. adapters/postgres implements it.
type CleanupStore interface {
	ChallengeStore
	// DeleteExpiredLoginFlows deletes the login flows whose expires_at lies before now.
	DeleteExpiredLoginFlows(ctx context.Context, now time.Time) (int64, error)
	// DeleteClosedPairings deletes the completed, rejected and expired pairings whose expires_at
	// lies before now (03 device_pairings, retention class transient).
	DeleteClosedPairings(ctx context.Context, now time.Time) (int64, error)
}

// AuthCleanup is the use case of the job identity.auth_cleanup (docs/spec/02-architecture.md
// section 12.3): it deletes the challenges, the OIDC login flows (M1-41) and the closed pairings
// (M7-05) past expires_at,
// revokes the expired pending devices and expires the pairings past expires_at (Expiry, M1-38)
// and revokes the devices without contact for auth.device_inactive_revoke_after (DeviceExpirer,
// M1-45). Requests never depend on it, because every use case checks expiry itself. It is safe
// for concurrent use; a repeated run is harmless.
type AuthCleanup struct {
	cfg     Config
	uow     txn.UnitOfWork
	store   CleanupStore
	expiry  Expiry
	devices DeviceExpirer
	console ConsoleExpirer
	clock   clock.Clock
}

// ConsoleExpirer revokes the console sessions past their end or idle (Console, M6-01).
type ConsoleExpirer interface {
	// ExpireConsoleSessions revokes them and returns their number.
	ExpireConsoleSessions(ctx context.Context) (int, error)
}

// WithConsole makes the cleanup also end the idle and expired console sessions through c
// (D-M6-01-3); internal/wire calls it once while it builds identity. It returns the cleanup.
func (c *AuthCleanup) WithConsole(e ConsoleExpirer) *AuthCleanup {
	c.console = e
	return c
}

// Expiry is the device and pairing expiry the cleanup runs (Service implements it, M1-38).
type Expiry interface {
	// ExpirePendingDevices revokes the pending devices past auth.device_pending_ttl.
	ExpirePendingDevices(ctx context.Context) (int64, error)
	// ExpirePairings expires the pairings past expires_at.
	ExpirePairings(ctx context.Context) (int64, error)
}

// DeviceExpirer revokes the inactive devices (Lifecycle.ExpireInactiveDevices, M1-45).
type DeviceExpirer interface {
	// ExpireInactiveDevices revokes every inactive device and returns their number.
	ExpireInactiveDevices(ctx context.Context) (int, error)
}

// NewAuthCleanup returns the use case with auth.cleanup_interval and auth.login_fail_window of cfg
// above 0, the expiry of pending devices and pairings, the inactive device sweep devices and the
// clock c (nil means clock.System()). Errors: a missing collaborator or a duration that is not
// positive.
func NewAuthCleanup(cfg Config, uow txn.UnitOfWork, store CleanupStore, expiry Expiry, devices DeviceExpirer, c clock.Clock) (*AuthCleanup, error) {
	if uow == nil || store == nil || expiry == nil || devices == nil {
		return nil, errors.New("identity: NewAuthCleanup needs the unit of work, the cleanup store, the expiry and the device sweep")
	}
	if cfg.CleanupInterval <= 0 || cfg.LoginFailWindow <= 0 {
		return nil, errors.New("identity: auth.cleanup_interval and auth.login_fail_window must be positive")
	}
	if c == nil {
		c = clock.System()
	}
	return &AuthCleanup{cfg: cfg, uow: uow, store: store, expiry: expiry, devices: devices, clock: c}, nil
}

// Run deletes in one unit of work every challenge whose expires_at has passed at the time of the
// clock, except failed step-up challenges that still count inside auth.login_fail_window (08
// section 13), every login flow past its expires_at and every closed pairing past its expires_at
// (M7-05; an open one is expired below first and goes with the next run), then revokes the expired pending devices,
// expires the due pairings (M1-38), revokes the inactive devices (M1-45) and ends the idle and
// expired console sessions (M6-01), each in its own unit of work. It returns how many rows it ended. Errors: the first failing step; the steps before it
// stay committed and a retry continues.
func (c *AuthCleanup) Run(ctx context.Context) (int64, error) {
	now := c.clock.Now()
	var n int64
	err := c.uow.Do(ctx, func(ctx context.Context) error {
		challenges, err := c.store.DeleteExpiredChallenges(ctx, now, now.Add(-c.cfg.LoginFailWindow))
		if err != nil {
			return err
		}
		flows, err := c.store.DeleteExpiredLoginFlows(ctx, now)
		if err != nil {
			return err
		}
		pairings, err := c.store.DeleteClosedPairings(ctx, now)
		n = challenges + flows + pairings
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
	n += pairings
	if err != nil {
		return n, err
	}
	revoked, err := c.devices.ExpireInactiveDevices(ctx)
	n += int64(revoked)
	if err != nil || c.console == nil {
		return n, err
	}
	ended, err := c.console.ExpireConsoleSessions(ctx)
	return n + int64(ended), err
}
