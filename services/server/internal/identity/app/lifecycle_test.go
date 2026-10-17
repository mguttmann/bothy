// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
)

// noExpirer is a device sweep that revokes nothing.
type noExpirer struct{}

func (noExpirer) ExpireInactiveDevices(context.Context) (int, error) { return 0, nil }

// TestNewLifecycle_Refuses (M1-45): the revocation use cases need their store and the people,
// session and token use cases, and a positive auth.device_inactive_revoke_after; the cleanup job
// needs the device sweep.
func TestNewLifecycle_Refuses(t *testing.T) {
	t.Parallel()
	cfg := app.Config{DeviceInactiveRevokeAfter: time.Hour}
	if _, err := app.NewLifecycle(cfg, app.LifecycleDeps{}); err == nil {
		t.Error("NewLifecycle without collaborators passed")
	}
	f := newFixture(t)
	if _, err := app.NewLifecycle(app.Config{}, app.LifecycleDeps{Store: f.repo, People: &app.People{}, Sessions: &app.Sessions{}, Tokens: &app.Tokens{}}); err == nil {
		t.Error("NewLifecycle with auth.device_inactive_revoke_after 0 passed")
	}
	if _, err := app.NewLifecycle(cfg, app.LifecycleDeps{Store: f.repo, People: &app.People{}, Sessions: &app.Sessions{}, Tokens: &app.Tokens{}}); err != nil {
		t.Errorf("NewLifecycle = %v", err)
	}
	clean := app.Config{CleanupInterval: time.Hour, LoginFailWindow: time.Hour}
	if _, err := app.NewAuthCleanup(clean, f.uow, f.repo, noPendingExpiry{}, nil, nil); err == nil {
		t.Error("NewAuthCleanup without the device sweep passed")
	}
	c, err := app.NewAuthCleanup(clean, f.uow, f.repo, noPendingExpiry{}, noExpirer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := c.Run(context.Background()); err != nil || n != 0 {
		t.Errorf("Run on an empty database = %d, %v", n, err)
	}
}

// noPendingExpiry is the pending device and pairing expiry of M1-38 doing nothing; the cleanup
// takes it beside the inactive device sweep (INT-W14/9).
type noPendingExpiry struct{}

func (noPendingExpiry) ExpirePendingDevices(context.Context) (int64, error) { return 0, nil }

func (noPendingExpiry) ExpirePairings(context.Context) (int64, error) { return 0, nil }
