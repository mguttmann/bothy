// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/device"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
)

// TestEnrollmentStoreRoundTrip covers every method of app.EnrollmentStore against PostgreSQL 18:
// insert, the lock by hash inside a unit (and its refusal outside one), the expiry of the other
// codes of a person, the attempt counter and the single use.
func TestEnrollmentStoreRoundTrip(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	alex := e.mustAddUser(t, "alex", "alex@example.com")
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	first := app.EnrollmentCodeRecord{
		ID: ids.New(), UserID: alex.Principal.ID, Purpose: app.PurposeFirstLogin, Hash: []byte("hash-one-EXAMPLE"),
		ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}
	second := app.EnrollmentCodeRecord{
		ID: ids.New(), UserID: alex.Principal.ID, Purpose: app.PurposeRecovery, Hash: []byte("hash-two-EXAMPLE"),
		ExpiresAt: now.Add(time.Hour), CreatedAt: now, CreatedBy: alex.Principal.ID,
	}
	if err := e.write(t, func(ctx context.Context) error {
		if err := e.repo.InsertEnrollmentCode(ctx, first); err != nil {
			return err
		}
		if err := e.repo.ExpireOtherEnrollmentCodes(ctx, alex.Principal.ID, second.ID, now); err != nil {
			return err
		}
		return e.repo.InsertEnrollmentCode(ctx, second)
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.repo.LockEnrollmentCodeByHash(ctx, first.Hash); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("lock outside a unit: %v, want kind internal", err)
	}
	var got, missing app.EnrollmentCodeRecord
	var found, foundMissing bool
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		if got, found, err = e.repo.LockEnrollmentCodeByHash(ctx, first.Hash); err != nil {
			return err
		}
		missing, foundMissing, err = e.repo.LockEnrollmentCodeByHash(ctx, []byte("unknown"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !found || foundMissing || missing.ID != (ids.ID{}) || got.ID != first.ID || got.Purpose != app.PurposeFirstLogin || !got.ExpiresAt.Equal(now) ||
		!got.CreatedBy.IsZero() || !got.UsedAt.IsZero() {
		t.Fatalf("first code %+v (found %v), unknown found %v", got, found, foundMissing)
	}
	dev := ids.New()
	if err := e.write(t, func(ctx context.Context) error {
		if err := e.repo.IncrementEnrollmentAttempts(ctx, second.ID); err != nil {
			return err
		}
		_, err := e.repo.InsertDevice(ctx, deviceOf(t, dev, alex.Principal.ID))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var marked, again bool
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		if marked, err = e.repo.MarkEnrollmentCodeUsed(ctx, second.ID, dev, now); err != nil {
			return err
		}
		again, err = e.repo.MarkEnrollmentCodeUsed(ctx, second.ID, dev, now)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !marked || again {
		t.Fatalf("mark used = %v, again %v", marked, again)
	}
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		got, found, err = e.repo.LockEnrollmentCodeByHash(ctx, second.Hash)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !found || got.Attempts != 1 || got.UsedAt.IsZero() || got.UsedByDeviceID != dev || got.CreatedBy != alex.Principal.ID {
		t.Fatalf("second code %+v", got)
	}
	// Writes outside a unit of work are refused.
	if err := e.repo.InsertEnrollmentCode(ctx, first); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("insert outside a unit: %v", err)
	}
	if err := e.repo.ExpireOtherEnrollmentCodes(ctx, alex.Principal.ID, first.ID, now); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("expire outside a unit: %v", err)
	}
	if err := e.repo.IncrementEnrollmentAttempts(ctx, first.ID); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("increment outside a unit: %v", err)
	}
	if _, err := e.repo.MarkEnrollmentCodeUsed(ctx, first.ID, dev, now); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("mark outside a unit: %v", err)
	}
	// A taken hash (160 random bits, so only a bug) is internal, never SQL text.
	if err := e.write(t, func(ctx context.Context) error {
		dup := first
		dup.ID = ids.New()
		return e.repo.InsertEnrollmentCode(ctx, dup)
	}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("taken hash: %v", err)
	}
}

// deviceOf returns a valid active device id of user with a new ES256 key.
func deviceOf(t *testing.T, id, user ids.ID) device.Device {
	t.Helper()
	_, pub := identitytest.NewDeviceKey(jose.AlgES256)
	d, err := device.New(device.Registration{
		ID: id, UserID: user, Name: "Alex Phone", Platform: device.PlatformIOS, State: device.StateActive, ApprovedBy: user,
		DPoPKey: device.KeyInput{JWK: pub.JWK, Alg: pub.Alg},
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}
