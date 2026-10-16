// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// provider returns an oidc provider row with a fixed configuration and timestamps.
func provider(name string, enabled bool) identity.Provider {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return identity.Provider{
		ID: ids.New(), Kind: identity.ProviderOIDC, Name: name, Config: json.RawMessage(`{"issuer":"https://idp.example.com"}`),
		Enabled: enabled, AllowProvisioning: true, CreatedAt: at, UpdatedAt: at,
	}
}

func (e env) mustInsertProvider(t *testing.T, p identity.Provider) identity.Provider {
	t.Helper()
	var out identity.Provider
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		out, err = e.repo.InsertProvider(ctx, p)
		return err
	}); err != nil {
		t.Fatalf("insert provider %s: %v", p.Name, err)
	}
	return out
}

func (e env) mustLink(t *testing.T, userID, providerID ids.ID, subject string) identity.Identity {
	t.Helper()
	var out identity.Identity
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		out, err = e.repo.InsertIdentity(ctx, identity.Identity{
			ID: ids.New(), UserID: userID, ProviderID: providerID, ProviderKind: identity.ProviderOIDC,
			Subject: subject, Issuer: "https://idp.example.com", EmailAtLink: "a@example.com",
			LinkedAt: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
		})
		return err
	}); err != nil {
		t.Fatalf("link %s: %v", subject, err)
	}
	return out
}

// TestProviderStoreRoundTrip covers insert, the reads with and without a unit of work, the list
// filters, update and delete of identity_providers.
func TestProviderStoreRoundTrip(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	on := e.mustInsertProvider(t, provider("Beta", true))
	off := e.mustInsertProvider(t, provider("alpha", false))
	if on.Kind != identity.ProviderOIDC || on.Name != "Beta" || !on.Enabled || !on.AllowProvisioning || on.CreatedAt.IsZero() {
		t.Fatalf("inserted provider %+v", on)
	}

	got, ok, err := e.repo.Provider(ctx, off.ID)
	if err != nil || !ok || got.Name != "alpha" || string(got.Config) == "" {
		t.Fatalf("Provider = %+v %v %v", got, ok, err)
	}
	if _, ok, err := e.repo.Provider(ctx, ids.New()); err != nil || ok {
		t.Fatalf("unknown Provider = %v %v", ok, err)
	}
	if _, ok, err := e.repo.EnabledProvider(ctx, on.ID); err != nil || !ok {
		t.Fatalf("EnabledProvider outside a unit = %v %v", ok, err)
	}
	if _, ok, err := e.repo.EnabledProvider(ctx, off.ID); err != nil || ok {
		t.Fatalf("EnabledProvider of a disabled provider = %v %v", ok, err)
	}
	if err := e.write(t, func(ctx context.Context) error {
		if _, ok, err := e.repo.EnabledProvider(ctx, on.ID); err != nil || !ok {
			t.Errorf("EnabledProvider in a unit = %v %v", ok, err)
		}
		if _, ok, err := e.repo.LockProvider(ctx, off.ID); err != nil || !ok {
			t.Errorf("LockProvider = %v %v", ok, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	all, err := e.repo.Providers(ctx, false)
	if err != nil || len(all) != 2 || all[0].Name != "alpha" || all[1].Name != "Beta" {
		t.Fatalf("Providers(all) = %+v %v", all, err)
	}
	enabled, err := e.repo.Providers(ctx, true)
	if err != nil || len(enabled) != 1 || enabled[0].ID != on.ID {
		t.Fatalf("Providers(enabled) = %+v %v", enabled, err)
	}
	sessions, err := e.repo.LiveSessionsOfProvider(ctx, on.ID, time.Now())
	if err != nil || len(sessions) != 0 {
		t.Fatalf("LiveSessionsOfProvider = %v %v", sessions, err)
	}

	off.Name, off.Enabled, off.UpdatedAt = "Gamma", true, off.UpdatedAt.Add(time.Hour)
	var updated identity.Provider
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		updated, err = e.repo.UpdateProvider(ctx, off)
		return err
	}); err != nil || updated.Name != "Gamma" || !updated.Enabled || !updated.UpdatedAt.Equal(off.UpdatedAt) {
		t.Fatalf("UpdateProvider = %+v %v", updated, err)
	}

	var deleted, again bool
	if err := e.write(t, func(ctx context.Context) error {
		var err error
		if deleted, err = e.repo.DeleteProvider(ctx, off.ID); err != nil {
			return err
		}
		again, err = e.repo.DeleteProvider(ctx, off.ID)
		return err
	}); err != nil || !deleted || again {
		t.Fatalf("DeleteProvider = %v then %v, %v", deleted, again, err)
	}
}

// TestProviderNameTakenIsConflict checks the case-insensitive name index through mapError.
func TestProviderNameTakenIsConflict(t *testing.T) {
	e := newEnv(t)
	e.mustInsertProvider(t, provider("Corp", true))
	err := e.write(t, func(ctx context.Context) error {
		_, err := e.repo.InsertProvider(ctx, provider("corp", true))
		return err
	})
	if e, ok := errs.As(err); !ok || e.Code != errs.CodeConflict {
		t.Fatalf("second provider with the same name: %v", err)
	}
}

// TestIdentityStoreRoundTrip covers link, the subject lookup with and without a unit of work, the
// reads of a person's identities, the counts, the login mark, unlink and the restrict on delete.
func TestIdentityStoreRoundTrip(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	u := e.mustAddUser(t, "ada", "ada@example.com")
	p1 := e.mustInsertProvider(t, provider("One", true))
	p2 := e.mustInsertProvider(t, provider("Two", true))
	a := e.mustLink(t, u.Principal.ID, p1.ID, "sub-a")
	b := e.mustLink(t, u.Principal.ID, p2.ID, "sub-b")
	if a.ProviderKind != identity.ProviderOIDC || a.Subject != "sub-a" || a.Issuer == "" || a.EmailAtLink == "" {
		t.Fatalf("linked identity %+v", a)
	}

	got, ok, err := e.repo.ActiveIdentityBySubject(ctx, p1.ID, "sub-a")
	if err != nil || !ok || got.ID != a.ID || got.UserID != u.Principal.ID {
		t.Fatalf("ActiveIdentityBySubject outside a unit = %+v %v %v", got, ok, err)
	}
	if _, ok, err := e.repo.ActiveIdentityBySubject(ctx, p1.ID, "nobody"); err != nil || ok {
		t.Fatalf("unknown subject = %v %v", ok, err)
	}
	rec, ok, err := e.repo.IdentityOfUser(ctx, u.Principal.ID, b.ID)
	if err != nil || !ok || rec.ProviderKind != identity.ProviderOIDC || !rec.ProviderEnabled || !rec.UnlinkedAt.IsZero() {
		t.Fatalf("IdentityOfUser = %+v %v %v", rec, ok, err)
	}
	if _, ok, err := e.repo.IdentityOfUser(ctx, u.Principal.ID, ids.New()); err != nil || ok {
		t.Fatalf("unknown IdentityOfUser = %v %v", ok, err)
	}
	list, err := e.repo.ActiveIdentitiesOfUser(ctx, u.Principal.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("ActiveIdentitiesOfUser = %+v %v", list, err)
	}
	if n, err := e.repo.CountOtherIdentitiesAtEnabledProviders(ctx, u.Principal.ID, a.ID); err != nil || n != 1 {
		t.Fatalf("CountOtherIdentitiesAtEnabledProviders = %d %v", n, err)
	}
	if n, err := e.repo.CountPairingDevices(ctx, u.Principal.ID); err != nil || n != 0 {
		t.Fatalf("CountPairingDevices = %d %v", n, err)
	}
	if s, err := e.repo.LiveSessionsOfIdentity(ctx, a.ID, time.Now()); err != nil || len(s) != 0 {
		t.Fatalf("LiveSessionsOfIdentity = %v %v", s, err)
	}

	at := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if err := e.write(t, func(ctx context.Context) error {
		if _, ok, err := e.repo.ActiveIdentityBySubject(ctx, p2.ID, "sub-b"); err != nil || !ok {
			t.Errorf("ActiveIdentityBySubject in a unit = %v %v", ok, err)
		}
		locked, err := e.repo.LockActiveIdentitiesOfUser(ctx, u.Principal.ID)
		if err != nil || len(locked) != 2 {
			t.Errorf("LockActiveIdentitiesOfUser = %v %v", locked, err)
		}
		if ok, err := e.repo.MarkIdentityLogin(ctx, a.ID, at); err != nil || !ok {
			t.Errorf("MarkIdentityLogin = %v %v", ok, err)
		}
		if ok, err := e.repo.UnlinkIdentity(ctx, a.ID, at); err != nil || !ok {
			t.Errorf("UnlinkIdentity = %v %v", ok, err)
		}
		if ok, err := e.repo.UnlinkIdentity(ctx, a.ID, at); err != nil || ok {
			t.Errorf("second UnlinkIdentity = %v %v", ok, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rec, ok, err = e.repo.IdentityOfUser(ctx, u.Principal.ID, a.ID)
	if err != nil || !ok || !rec.UnlinkedAt.Equal(at) || !rec.LastLoginAt.Equal(at) {
		t.Fatalf("unlinked identity = %+v %v %v", rec, ok, err)
	}
	if _, ok, err := e.repo.ActiveIdentityBySubject(ctx, p1.ID, "sub-a"); err != nil || ok {
		t.Fatalf("unlinked subject still active: %v %v", ok, err)
	}

	// An unlinked identity still keeps its provider (08 section 3.3): delete is a conflict.
	err = e.write(t, func(ctx context.Context) error {
		_, err := e.repo.DeleteProvider(ctx, p1.ID)
		return err
	})
	ee, isErr := errs.As(err)
	if !isErr || ee.Code != errs.CodeConflict || ee.Reason != "provider_in_use" {
		t.Fatalf("DeleteProvider of a referenced provider: %v", err)
	}
}

// TestProviderAndIdentityWritesNeedAUnitOfWork checks that every write refuses to run outside a
// unit of work.
func TestProviderAndIdentityWritesNeedAUnitOfWork(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id, now := ids.New(), time.Now()
	writes := map[string]func() error{
		"InsertProvider": func() error {
			_, err := e.repo.InsertProvider(ctx, provider("x", true))
			return err
		},
		"LockProvider": func() error { _, _, err := e.repo.LockProvider(ctx, id); return err },
		"UpdateProvider": func() error {
			_, err := e.repo.UpdateProvider(ctx, provider("x", true))
			return err
		},
		"DeleteProvider": func() error { _, err := e.repo.DeleteProvider(ctx, id); return err },
		"InsertIdentity": func() error { _, err := e.repo.InsertIdentity(ctx, identity.Identity{ID: id}); return err },
		"LockActiveIdentitiesOfUser": func() error {
			_, err := e.repo.LockActiveIdentitiesOfUser(ctx, id)
			return err
		},
		"UnlinkIdentity":    func() error { _, err := e.repo.UnlinkIdentity(ctx, id, now); return err },
		"MarkIdentityLogin": func() error { _, err := e.repo.MarkIdentityLogin(ctx, id, now); return err },
	}
	for name, fn := range writes {
		if err := fn(); err == nil {
			t.Errorf("%s outside a unit of work: no error", name)
		}
	}
}
