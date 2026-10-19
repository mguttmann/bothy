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
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TestLocalCredentialStoreRoundTrip covers every method of app.LocalCredentialStore against
// PostgreSQL 18 (M1-39, INT-W18/7): the reads without a row, the lock outside a unit of work, the
// insert, the replacement of every column, the lock inside a unit, the read by person and the
// established check, which needs a confirmed TOTP enrollment.
func TestLocalCredentialStoreRoundTrip(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	alex := e.mustAddUser(t, "alex", "alex@example.com")
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	local := e.mustInsertProvider(t, identity.Provider{
		ID: ids.New(), Kind: identity.ProviderLocal, Name: "Local", Config: json.RawMessage(`{}`), Enabled: true, CreatedAt: at, UpdatedAt: at,
	})
	var ident identity.Identity
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		ident, err = e.repo.InsertIdentity(ctx, identity.Identity{
			ID: ids.New(), UserID: alex.Principal.ID, ProviderID: local.ID, ProviderKind: identity.ProviderLocal, Subject: "alex", LinkedAt: at,
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := e.repo.LocalCredential(ctx, ident.ID); err != nil || found {
		t.Fatalf("no row: found %v, %v", found, err)
	}
	if _, found, err := e.repo.LocalCredentialOfUser(ctx, alex.Principal.ID); err != nil || found {
		t.Fatalf("no row of the person: found %v, %v", found, err)
	}
	if _, _, err := e.repo.LockLocalCredential(ctx, ident.ID); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("lock outside a unit: %v", err)
	}
	rec := app.LocalCredentialRecord{IdentityID: ident.ID, PasswordHash: "$argon2id$v=19$m=19456,t=2,p=1$x$y", PasswordChangedAt: at} //nolint:gosec // G101: a fake hash of the test
	if err := e.write(t, func(ctx context.Context) error { return e.repo.SaveLocalCredential(ctx, rec) }); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.repo.HasEstablishedLocalCredential(ctx, alex.Principal.ID); err != nil || ok {
		t.Fatalf("established without TOTP: %v, %v", ok, err)
	}
	step := int64(59)
	rec.TOTPCiphertext, rec.TOTPKeyRef, rec.TOTPLastStep, rec.TOTPEnabledAt = []byte{1, 2, 3}, "file:/k", &step, at.Add(time.Minute)
	rec.Lockout = domain.Lockout{FailedCount: 2, WindowStartedAt: at.Add(2 * time.Minute), LockedUntil: at.Add(time.Hour)}
	if err := e.write(t, func(ctx context.Context) error {
		if _, found, err := e.repo.LockLocalCredential(ctx, ids.New()); err != nil || found {
			t.Errorf("unknown lock found %v, %v", found, err)
		}
		if _, found, err := e.repo.LockLocalCredential(ctx, ident.ID); err != nil || !found {
			t.Errorf("lock found %v, %v", found, err)
		}
		return e.repo.SaveLocalCredential(ctx, rec)
	}); err != nil {
		t.Fatal(err)
	}
	got, found, err := e.repo.LocalCredential(ctx, ident.ID)
	if err != nil || !found || got.TOTPKeyRef != "file:/k" || got.TOTPLastStep == nil || *got.TOTPLastStep != 59 ||
		!got.TOTPEnabledAt.Equal(rec.TOTPEnabledAt) || got.Lockout.FailedCount != 2 || !got.Lockout.LockedUntil.Equal(rec.Lockout.LockedUntil) {
		t.Fatalf("stored %+v, %v, %v", got, found, err)
	}
	if got, found, err := e.repo.LocalCredentialOfUser(ctx, alex.Principal.ID); err != nil || !found || got.IdentityID != ident.ID {
		t.Fatalf("of the person %+v, %v, %v", got, found, err)
	}
	if ok, err := e.repo.HasEstablishedLocalCredential(ctx, alex.Principal.ID); err != nil || !ok {
		t.Fatalf("established: %v, %v", ok, err)
	}
}
