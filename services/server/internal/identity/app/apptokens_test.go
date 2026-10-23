// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// fakeInstallations is an identity.AppInstallations over a map; locks counts the calls of
// LockActiveInstallation and the units they ran in.
type fakeInstallations struct {
	mu     sync.Mutex
	active map[ids.ID]identity.ActiveAppInstallation
	locks  int
	inUnit bool
}

func (f *fakeInstallations) set(app ids.ID, inst *identity.ActiveAppInstallation) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active == nil {
		f.active = map[ids.ID]identity.ActiveAppInstallation{}
	}
	if inst == nil {
		delete(f.active, app)
		return
	}
	f.active[app] = *inst
}

func (f *fakeInstallations) ActiveInstallation(_ context.Context, app ids.ID) (identity.ActiveAppInstallation, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i, ok := f.active[app]
	return i, ok, nil
}

func (f *fakeInstallations) LockActiveInstallation(ctx context.Context, app ids.ID) (identity.ActiveAppInstallation, bool, error) {
	f.mu.Lock()
	f.locks++
	f.inUnit = txn.InUnit(ctx)
	f.mu.Unlock()
	return f.ActiveInstallation(ctx, app)
}

// TestCreateToken_AppScopes (M2-24 design note 10 a): without an active installation createToken
// keeps 409 app_not_installed; with one the token takes the granted scopes, an explicit subset is
// stored as sent, and a scope that is not granted is 400 at /scopes/<i>. The lock runs inside the
// unit of work, and the audit entry names the stored scopes.
func TestCreateToken_AppScopes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	now := time.Now().UTC()
	apps := &fakeInstallations{}
	rec := &entries{}
	cfg := app.Config{APITokenMaxTTL: 90 * 24 * time.Hour, PersonalTokenMaxTTL: 30 * 24 * time.Hour, PageSizeDefault: 50, PageSizeMax: 200}
	s, err := app.NewTokens(cfg, tokenPepper, app.TokensDeps{
		Store: f.repo, Principals: f.repo, UoW: f.uow, Authorizer: allowAll{}, Audit: rec, Cursors: nopCursors{},
		Switch: personalOff{}, Clock: clock.NewFake(now), Apps: apps,
	})
	if err != nil {
		t.Fatal(err)
	}
	tracker := f.add(domain.KindApp, "tracker")
	robin := f.add(domain.KindUser, "robin")
	owner := actor.Actor{ID: robin.ID, Kind: actor.KindUser}
	ctx := context.Background()
	create := func(scopes ...string) (identity.IssuedToken, error) {
		return s.CreateToken(ctx, owner, tracker.ID, identity.CreateTokenCmd{Name: "Sync", Scopes: scopes, ExpiresAt: now.Add(time.Hour)})
	}
	wantReason := func(err error, code errs.Code, reason string) {
		t.Helper()
		e, ok := errs.As(err)
		if !ok || e.Code != code || e.Reason != reason {
			t.Fatalf("error %v, want %s %s", err, code, reason)
		}
	}
	_, err = create()
	wantReason(err, errs.CodeConflict, "app_not_installed")
	if apps.locks != 1 || !apps.inUnit {
		t.Fatalf("lock calls %d, in unit %v", apps.locks, apps.inUnit)
	}
	apps.set(tracker.ID, &identity.ActiveAppInstallation{ID: ids.New(), GrantedScopes: []string{"messages:write", "messages:read"}})
	all, err := create()
	if err != nil || !slices.Equal(all.Token.Scopes, []string{"messages:read", "messages:write"}) || all.Token.Kind != identity.TokenKindApp {
		t.Fatalf("default scopes %+v %v", all.Token, err)
	}
	if p, err := apitoken.Parse(all.Secret.Reveal()); err != nil || p.Kind != apitoken.KindApp {
		t.Fatalf("secret %v", err)
	}
	e := rec.last(t)
	if e.Attrs["scopes"].Kind() != auditing.KindList || e.Attrs["kind"].Data() != "app" {
		t.Errorf("audit %+v", e.Attrs)
	}
	sub, err := create("messages:write")
	if err != nil || !slices.Equal(sub.Token.Scopes, []string{"messages:write"}) {
		t.Fatalf("subset %+v %v", sub.Token, err)
	}
	stored, _, _ := f.repo.APIToken(ctx, sub.Token.ID)
	if !slices.Equal(stored.Scopes, []string{"messages:write"}) {
		t.Errorf("stored scopes %v", stored.Scopes)
	}
	_, err = create("files:read")
	wantField(t, err, "/scopes/0")
	_, err = create("messages:write", "messages:write")
	wantField(t, err, "/scopes/1")
	apps.set(tracker.ID, nil)
	_, err = create()
	wantReason(err, errs.CodeConflict, "app_not_installed")
}

// wantField checks a validation_failed error with a field at pointer.
func wantField(t *testing.T, err error, pointer string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed || len(e.Fields) != 1 || e.Fields[0].Pointer != pointer {
		t.Fatalf("error %v, want validation_failed at %s", err, pointer)
	}
}

// TestAuthenticateAPIToken_AppInstallation (M2-24 design note 10 b, 11 a): an app token of an app
// without an active installation yields an actor with a zero InstallationID (authorize answers
// app_not_installed); an active installation fills InstallationID and GrantedScopes next to the
// token scopes. Revalidate fails with app_not_installed once the installation is no longer active
// or no longer grants a scope of the actor.
func TestAuthenticateAPIToken_AppInstallation(t *testing.T) {
	t.Parallel()
	store := &countingTokens{}
	e := newTokenEnv(t)
	d := newDPoPEnv(t, "pepper")
	apps := &fakeInstallations{}
	a, err := app.NewAuthenticator(app.Config{LastActiveGranularity: 5 * time.Minute, DevicePendingTTL: 24 * time.Hour}, app.AuthenticatorDeps{
		AccessTokens: e.at, DPoP: d.v, Sessions: noSessions{}, APITokens: store, Pepper: tokenPepper, Clock: e.clk, Apps: apps,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	secret, rec := store.store(t, apitoken.KindApp, domain.KindApp, func(r *app.APITokenRecord) { r.Scopes = []string{"messages:write"} })
	got, err := a.Authenticate(ctx, actor.Credentials{Authorization: "Bearer " + secret})
	if err != nil || got.Kind != actor.KindApp || !got.InstallationID.IsZero() || len(got.GrantedScopes) != 0 || got.TokenID != rec.ID {
		t.Fatalf("without installation %+v %v", got, err)
	}
	inst := identity.ActiveAppInstallation{ID: ids.New(), GrantedScopes: []string{"messages:read", "messages:write"}}
	apps.set(rec.PrincipalID, &inst)
	got, err = a.Authenticate(ctx, actor.Credentials{Authorization: "Bearer " + secret})
	if err != nil || got.InstallationID != inst.ID || !slices.Equal(got.GrantedScopes, inst.GrantedScopes) || !slices.Equal(got.Scopes, []string{"messages:write"}) {
		t.Fatalf("with installation %+v %v", got, err)
	}
	if err := a.Revalidate(ctx, got); err != nil {
		t.Fatalf("Revalidate: %v", err)
	}
	apps.set(rec.PrincipalID, &identity.ActiveAppInstallation{ID: inst.ID, GrantedScopes: []string{"messages:write"}})
	if err := a.Revalidate(ctx, got); reasonOfErr(err) != "app_not_installed" {
		t.Errorf("Revalidate after a scope reduction: %v", err)
	}
	apps.set(rec.PrincipalID, nil)
	if err := a.Revalidate(ctx, got); reasonOfErr(err) != "app_not_installed" || codeOfErr(err) != "unauthenticated" {
		t.Errorf("Revalidate after suspend: %v", err)
	}
}

// reasonOfErr returns the reason of a kernel/errs error.
func reasonOfErr(err error) string {
	if e, ok := errs.As(err); ok {
		return e.Reason
	}
	return ""
}

// TestPrincipalWrites (M2-24): an app principal is created with the handle and display name rules,
// its status set and its display name updated inside a unit of work; outside one every call is an
// internal error; a person is not found.
func TestPrincipalWrites(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	now := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	w, err := app.NewPrincipalWrites(app.Config{HandleMaxChars: 32, DisplayNameMaxChars: 64}, f.repo, clock.NewFake(now))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id := ids.New()
	if _, err := w.CreatePrincipal(ctx, identity.NewPrincipalCmd{ID: id, Kind: actor.KindApp, Handle: "tracker", DisplayName: "Tracker"}); codeOfErr(err) != "internal_error" {
		t.Fatalf("outside a unit: %v", err)
	}
	f.do(func(ctx context.Context) error {
		v, err := w.CreatePrincipal(ctx, identity.NewPrincipalCmd{ID: id, Kind: actor.KindApp, Handle: "tracker", DisplayName: "Tracker"})
		if err != nil || v.Kind != actor.KindApp || v.Status != identity.StatusActive || v.Handle != "tracker" {
			t.Fatalf("create %+v %v", v, err)
		}
		return nil
	})
	f.add(domain.KindUser, "alex")
	_ = f.uow.Do(ctx, func(ctx context.Context) error {
		_, err := w.CreatePrincipal(ctx, identity.NewPrincipalCmd{ID: ids.New(), Kind: actor.KindApp, Handle: "Alex", DisplayName: "Alex"})
		wantField(t, err, "/handle")
		_, err = w.CreatePrincipal(ctx, identity.NewPrincipalCmd{ID: ids.New(), Kind: actor.KindApp, Handle: "alex", DisplayName: "Alex"})
		if codeOfErr(err) != "conflict" {
			t.Errorf("taken handle: %v", err)
		}
		return nil
	})
	f.do(func(ctx context.Context) error {
		if err := w.SetPrincipalStatus(ctx, id, identity.StatusDisabled); err != nil {
			return err
		}
		return w.UpdateDisplayName(ctx, id, "Tracker Two")
	})
	rec, _, _ := f.repo.Principal(ctx, id)
	if rec.Principal.Status != domain.StatusDisabled || !rec.Principal.DisabledAt.Equal(now) || rec.Principal.DisplayName.String() != "Tracker Two" {
		t.Fatalf("after disable %+v", rec.Principal)
	}
	f.do(func(ctx context.Context) error { return w.SetPrincipalStatus(ctx, id, identity.StatusActive) })
	rec, _, _ = f.repo.Principal(ctx, id)
	if rec.Principal.Status != domain.StatusActive || !rec.Principal.DisabledAt.IsZero() {
		t.Fatalf("after enable %+v", rec.Principal)
	}
	_ = f.uow.Do(ctx, func(ctx context.Context) error {
		if err := w.SetPrincipalStatus(ctx, id, identity.StatusDeleted); codeOfErr(err) != "validation_failed" {
			t.Errorf("status deleted: %v", err)
		}
		if err := w.SetPrincipalStatus(ctx, ids.New(), identity.StatusActive); codeOfErr(err) != "not_found" {
			t.Errorf("unknown principal: %v", err)
		}
		if err := w.UpdateDisplayName(ctx, id, " "); codeOfErr(err) != "validation_failed" {
			t.Errorf("blank name: %v", err)
		}
		return nil
	})
}

// TestPrincipalSource_AppGrantedScopes (M2-25 design note 3 a): the principal source reports the
// granted scopes of an active installation with the facts of an app, so authz decides the
// app-owner guard from the facts of the owner; without an active installation the list is empty.
func TestPrincipalSource_AppGrantedScopes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	apps := &fakeInstallations{}
	src := app.NewPrincipalSource(f.repo).WithApps(apps)
	tracker := f.add(domain.KindApp, "tracker")
	ctx := context.Background()
	got, found, err := src.Principal(ctx, tracker.ID)
	if err != nil || !found || got.AppInstallationActive || len(got.AppGrantedScopes) != 0 {
		t.Fatalf("without installation: %+v %v %v", got, found, err)
	}
	apps.set(tracker.ID, &identity.ActiveAppInstallation{ID: ids.New(), GrantedScopes: []string{"commands:write", "messages:write"}})
	got, found, err = src.Principal(ctx, tracker.ID)
	if err != nil || !found || !got.AppInstallationActive || !slices.Equal(got.AppGrantedScopes, []string{"commands:write", "messages:write"}) {
		t.Fatalf("with installation: %+v %v %v", got, found, err)
	}
}
