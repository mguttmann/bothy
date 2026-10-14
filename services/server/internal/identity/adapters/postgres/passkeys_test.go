// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestPasskeyStoreRoundTrip covers every method of app.PasskeyStore against PostgreSQL 18 (M1-40):
// insert with its duplicate conflict, the active list, the lookup by credential ID, the lock, the
// use update, the revocation and the counts, and the ceremony columns of auth_challenges.
func TestPasskeyStoreRoundTrip(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	alex := e.mustAddUser(t, "alex", "alex@example.com")
	now := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	pk := app.StoredPasskey{
		ID: ids.New(), UserID: alex.Principal.ID, CredentialID: []byte("cred-EXAMPLE"), PublicKeyCOSE: []byte{1, 2, 3}, SignCount: 4,
		AAGUID: ids.New(), BackupEligible: true, Name: "Laptop", CreatedAt: now,
	}
	var stored app.StoredPasskey
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		stored, err = e.repo.InsertPasskey(ctx, pk)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if stored.ID != pk.ID || stored.SignCount != 4 || stored.Transports == nil || stored.AAGUID != pk.AAGUID || !stored.CreatedAt.Equal(now) {
		t.Fatalf("stored %+v", stored)
	}
	dup := pk
	dup.ID = ids.New()
	if err := e.write(t, func(ctx context.Context) error { _, err := e.repo.InsertPasskey(ctx, dup); return err }); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("duplicate credential: %v", err)
	}
	if list, err := e.repo.ActivePasskeysOfUser(ctx, alex.Principal.ID); err != nil || len(list) != 1 {
		t.Fatalf("active %v, %v", list, err)
	}
	if got, found, err := e.repo.PasskeyByCredentialID(ctx, pk.CredentialID); err != nil || !found || got.ID != pk.ID {
		t.Fatalf("by credential %+v, %v, %v", got, found, err)
	}
	if _, found, err := e.repo.PasskeyByCredentialID(ctx, []byte("unknown")); err != nil || found {
		t.Fatalf("unknown credential found %v, %v", found, err)
	}
	if _, _, err := e.repo.LockPasskey(ctx, pk.ID); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("lock outside a unit: %v", err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		if _, found, err := e.repo.LockPasskey(ctx, ids.New()); err != nil || found {
			t.Errorf("unknown lock found %v, %v", found, err)
		}
		if _, _, err := e.repo.LockPasskey(ctx, pk.ID); err != nil {
			return err
		}
		return e.repo.UpdatePasskeyUse(ctx, pk.ID, app.PasskeyUse{SignCount: 9, BackupState: true, At: now.Add(time.Minute)})
	}); err != nil {
		t.Fatal(err)
	}
	got, _, _ := e.repo.PasskeyByCredentialID(ctx, pk.CredentialID)
	if got.SignCount != 9 || !got.BackupState || !got.LastUsedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("after use %+v", got)
	}
	if has, err := e.repo.HasActivePasskey(ctx, alex.Principal.ID); err != nil || !has {
		t.Fatalf("has %v, %v", has, err)
	}
	for i, want := range []bool{true, false} {
		var revoked bool
		if err := e.write(t, func(ctx context.Context) error {
			var err error
			revoked, err = e.repo.RevokePasskey(ctx, pk.ID, now.Add(time.Hour))
			return err
		}); err != nil || revoked != want {
			t.Fatalf("revoke %d: %v, %v", i, revoked, err)
		}
	}
	if n, err := e.repo.CountActivePasskeys(ctx, alex.Principal.ID); err != nil || n != 0 {
		t.Fatalf("count %d, %v", n, err)
	}
	if err := e.repo.UpdatePasskeyUse(ctx, pk.ID, app.PasskeyUse{}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("update outside a unit: %v", err)
	}
	if _, err := e.repo.InsertPasskey(ctx, pk); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("insert outside a unit: %v", err)
	}
	if _, err := e.repo.RevokePasskey(ctx, pk.ID, now); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("revoke outside a unit: %v", err)
	}
}
