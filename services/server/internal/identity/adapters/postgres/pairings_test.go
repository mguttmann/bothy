// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestPairingStoreRoundTrip covers app.PairingStore against PostgreSQL 18: the insert with the
// collision of a requested code hash, the locks inside a unit (and their refusal outside one), the
// update of every column and the due list of the cleanup (M1-38).
func TestPairingStoreRoundTrip(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	alex := e.mustAddUser(t, "alex", "alex@example.com")
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	reg := domain.DeviceRegistrationV1{V: 1, DeviceID: ids.New().String(), Name: "Alex Laptop", Platform: "macos",
		DPoP: domain.RegisteredKey{JWK: json.RawMessage(`{"kty":"EC"}`), Alg: "ES256"}}
	rec := app.PairingRecord{ID: ids.New(), Hash: []byte("pairing-hash-EXAMPLE"), Registration: reg, CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute)}
	var inserted, collided bool
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		if inserted, err = e.repo.InsertPairing(ctx, rec); err != nil {
			return err
		}
		other := rec
		other.ID = ids.New()
		collided, err = e.repo.InsertPairing(ctx, other)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !inserted || collided {
		t.Fatalf("insert %v, collision inserted %v", inserted, collided)
	}
	if _, _, err := e.repo.LockPairing(ctx, rec.ID); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("lock outside a unit: %v", err)
	}
	if _, _, err := e.repo.LockRequestedPairingByHash(ctx, rec.Hash); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("lock by hash outside a unit: %v", err)
	}
	dev := ids.New()
	if err := e.write(t, func(ctx context.Context) error {
		got, found, err := e.repo.LockRequestedPairingByHash(ctx, rec.Hash)
		if err != nil || !found || got.ID != rec.ID || got.Status != domain.PairingRequested || got.Registration.Name != "Alex Laptop" {
			t.Errorf("by hash %+v, %v, %v", got, found, err)
		}
		if _, found, err := e.repo.LockRequestedPairingByHash(ctx, []byte("unknown")); err != nil || found {
			t.Errorf("unknown hash found %v, %v", found, err)
		}
		if _, err := e.repo.InsertDevice(ctx, deviceOf(t, dev, alex.Principal.ID)); err != nil {
			return err
		}
		got.Status, got.UserID, got.ApprovedByDeviceID, got.DeviceID = domain.PairingApproved, alex.Principal.ID, dev, dev
		return e.repo.UpdatePairing(ctx, got)
	}); err != nil {
		t.Fatal(err)
	}
	due, err := e.repo.DuePairings(ctx, now.Add(5*time.Minute), 10)
	if err != nil || len(due) != 1 || due[0].Status != domain.PairingApproved || due[0].DeviceID != dev || due[0].UserID != alex.Principal.ID {
		t.Fatalf("due %+v, %v", due, err)
	}
	if early, err := e.repo.DuePairings(ctx, now, 10); err != nil || len(early) != 0 {
		t.Fatalf("due before the end %+v, %v", early, err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		got, found, err := e.repo.LockPairing(ctx, rec.ID)
		if err != nil || !found {
			t.Errorf("lock %v, %v", found, err)
		}
		got.Status, got.CompletedAt = domain.PairingCompleted, now
		if err := e.repo.UpdatePairing(ctx, got); err != nil {
			return err
		}
		if _, found, err := e.repo.LockPairing(ctx, ids.New()); err != nil || found {
			t.Errorf("unknown pairing found %v, %v", found, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if left, err := e.repo.DuePairings(ctx, now.Add(time.Hour), 10); err != nil || len(left) != 0 {
		t.Fatalf("a completed pairing is due: %+v, %v", left, err)
	}
	// A completed pairing frees its code hash for a new request.
	if err := e.write(t, func(ctx context.Context) error {
		again := rec
		again.ID = ids.New()
		ok, err := e.repo.InsertPairing(ctx, again)
		if !ok {
			t.Error("the hash of a completed pairing is still taken")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `UPDATE device_pairings SET device_registration = '"not an object"' WHERE id = $1`, rec.ID.String()); err != nil {
		t.Fatal(err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		_, _, err := e.repo.LockPairing(ctx, rec.ID)
		if errs.KindOf(err) != errs.KindInternal {
			t.Errorf("undecodable registration: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestDeviceApprovalStore covers the device and session methods of M1-38: lock, activation,
// revocation with the keys, the expiry list and the open sessions of a device.
func TestDeviceApprovalStore(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	alex := e.mustAddUser(t, "alex", "alex@example.com")
	created := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	pending := deviceOf(t, ids.New(), alex.Principal.ID)
	pending.State, pending.ApprovedBy, pending.CreatedAt = device.StatePending, ids.ID{}, created
	sid := ids.New()
	if err := e.write(t, func(ctx context.Context) error {
		d, err := e.repo.InsertDevice(ctx, pending)
		if err != nil {
			return err
		}
		if !d.CreatedAt.Equal(created) || !d.ApprovedAt.IsZero() {
			t.Errorf("created_at %v approved_at %v", d.CreatedAt, d.ApprovedAt)
		}
		return e.repo.InsertSession(ctx, app.SessionRecord{
			ID: sid, UserID: alex.Principal.ID, DeviceID: pending.ID, Component: identity.ComponentApp, AuthMethod: identity.AuthPasskey,
			CreatedAt: created, ExpiresAt: created.Add(time.Hour),
		})
	}); err != nil {
		t.Fatal(err)
	}
	due, err := e.repo.PendingDevicesCreatedBefore(ctx, created, 10)
	if err != nil || len(due) != 1 || due[0].ID != pending.ID || due[0].KeyThumbprint == "" {
		t.Fatalf("due %+v, %v", due, err)
	}
	if early, err := e.repo.PendingDevicesCreatedBefore(ctx, created.Add(-time.Second), 10); err != nil || len(early) != 0 {
		t.Fatalf("due before its time %+v, %v", early, err)
	}
	open, err := e.repo.OpenSessionsOfDevice(ctx, pending.ID)
	if err != nil || len(open) != 1 || open[0] != sid {
		t.Fatalf("open sessions %v, %v", open, err)
	}
	if _, _, err := e.repo.LockDevice(ctx, pending.ID); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("lock outside a unit: %v", err)
	}
	if err := e.repo.ClearSessionStepUp(ctx, sid); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("clear outside a unit: %v", err)
	}
	var user ids.ID
	var revoked, again, activated bool
	if err := e.write(t, func(ctx context.Context) error {
		d, found, err := e.repo.LockDevice(ctx, pending.ID)
		if err != nil || !found || d.State != identity.DeviceStatePending {
			t.Errorf("lock %+v, %v, %v", d, found, err)
		}
		if _, found, err := e.repo.LockDevice(ctx, ids.New()); err != nil || found {
			t.Errorf("unknown device found %v, %v", found, err)
		}
		if err := e.repo.ClearSessionStepUp(ctx, sid); err != nil {
			return err
		}
		if user, revoked, err = e.repo.RevokeDevice(ctx, pending.ID, identity.DeviceRevokeVerificationFailed, alex.Principal.ID, created); err != nil {
			return err
		}
		if _, again, err = e.repo.RevokeDevice(ctx, pending.ID, identity.DeviceRevokePendingExpired, ids.ID{}, created); err != nil {
			return err
		}
		activated, err = e.repo.ActivateDevice(ctx, pending.ID, alex.Principal.ID, created)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if user != alex.Principal.ID || !revoked || again || activated {
		t.Fatalf("revoke %v %v, again %v, activate a revoked device %v", user, revoked, again, activated)
	}
	d, _, err := e.repo.Device(ctx, pending.ID)
	if err != nil || d.State != identity.DeviceStateRevoked || d.RevokeReason != "verification_failed" || d.KeyThumbprint != "" {
		t.Fatalf("revoked device %+v, %v", d, err)
	}
	// An active pending device: ActivateDevice sets the approver.
	next := deviceOf(t, ids.New(), alex.Principal.ID)
	next.State, next.ApprovedBy = device.StatePending, ids.ID{}
	if err := e.write(t, func(ctx context.Context) error {
		if _, err := e.repo.InsertDevice(ctx, next); err != nil {
			return err
		}
		ok, err := e.repo.ActivateDevice(ctx, next.ID, alex.Principal.ID, created)
		if !ok {
			t.Error("pending device not activated")
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if d, _, err := e.repo.Device(ctx, next.ID); err != nil || d.State != identity.DeviceStateActive || !d.ApprovedAt.Equal(created) {
		t.Fatalf("activated device %+v, %v", d, err)
	}
}
