// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// Compile-time checks: the app types implement the ports of identity and authz.
var (
	_ identity.Directory    = (*app.Directory)(nil)
	_ authz.PrincipalSource = (*app.PrincipalSource)(nil)
)

// fixture is a test database with the real repository and a unit of work.
type fixture struct {
	t     *testing.T
	repo  *postgres.Repository
	uow   *pg.UnitOfWork
	owner *pg.Pool
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	return &fixture{
		t:    t,
		repo: postgres.NewRepository(pool),
		uow: pg.NewUnitOfWork(pool, pg.UnitConfig{
			StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second,
		}, reg),
		owner: db.Pool(t, pg.RoleOwner),
	}
}

func (f *fixture) do(fn func(ctx context.Context) error) {
	f.t.Helper()
	if err := f.uow.Do(context.Background(), fn); err != nil {
		f.t.Fatal(err)
	}
}

// add inserts a principal of kind with handle; a person gets a users row.
func (f *fixture) add(kind domain.Kind, handle string) domain.Principal {
	f.t.Helper()
	h, err := domain.NewHandle(handle, 32)
	if err != nil {
		f.t.Fatal(err)
	}
	d, err := domain.NewDisplayName("Name of "+handle, 64)
	if err != nil {
		f.t.Fatal(err)
	}
	p, err := domain.NewPrincipal(ids.New(), kind, h, d)
	if err != nil {
		f.t.Fatal(err)
	}
	f.do(func(ctx context.Context) error {
		if err := f.repo.InsertPrincipal(ctx, &p); err != nil {
			return err
		}
		if kind != domain.KindUser {
			return nil
		}
		u, err := domain.NewUser(p, handle+"@example.com", "en", "Europe/Berlin")
		if err != nil {
			return err
		}
		return f.repo.InsertUser(ctx, u)
	})
	return p
}

// grant makes user an owner, granted by by (zero for the first owner).
func (f *fixture) grant(user, by ids.ID) domain.Owner {
	f.t.Helper()
	o, err := domain.NewOwner(ids.New(), user, by, "needs the owner role")
	if err != nil {
		f.t.Fatal(err)
	}
	f.do(func(ctx context.Context) error { return f.repo.InsertOwner(ctx, &o) })
	return o
}

// sql runs a statement as the schema owner, for state no use case of this ticket writes.
func (f *fixture) sql(stmt string, args ...any) {
	f.t.Helper()
	if _, err := f.owner.Exec(context.Background(), stmt, args...); err != nil {
		f.t.Fatalf("%s: %v", stmt, err)
	}
}

func TestPrincipalSource_Facts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	src := app.NewPrincipalSource(f.repo)
	ctx := context.Background()
	user := f.add(domain.KindUser, "alex")
	agent := f.add(domain.KindAgent, "helper")
	appP := f.add(domain.KindApp, "board")
	f.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, agent.ID.String())
	f.sql(`UPDATE principals SET status = 'pending' WHERE id = $1`, appP.ID.String())

	for _, tc := range []struct {
		id     ids.ID
		kind   actor.Kind
		status authz.PrincipalStatus
	}{
		{user.ID, actor.KindUser, authz.PrincipalActive},
		{agent.ID, actor.KindAgent, authz.PrincipalDisabled},
		{appP.ID, actor.KindApp, authz.PrincipalPending},
	} {
		got, ok, err := src.Principal(ctx, tc.id)
		if err != nil || !ok {
			t.Fatalf("Principal(%s): %v, ok %v", tc.kind, err, ok)
		}
		if want := (authz.PrincipalFacts{ID: tc.id, Kind: tc.kind, Status: tc.status}); got != want {
			t.Errorf("facts %+v, want %+v", got, want)
		}
	}

	o := f.grant(user.ID, ids.ID{})
	if got, _, _ := src.Principal(ctx, user.ID); !got.IsOwner {
		t.Error("IsOwner false for an active owners row")
	}
	f.sql(`UPDATE principals SET status = 'disabled' WHERE id = $1`, user.ID.String())
	if got, _, _ := src.Principal(ctx, user.ID); !got.IsOwner || got.Status != authz.PrincipalDisabled {
		t.Errorf("facts of a disabled owner %+v: IsOwner is independent of Status", got)
	}
	f.sql(`UPDATE owners SET revoked_at = now() WHERE id = $1`, o.ID.String())
	if got, _, _ := src.Principal(ctx, user.ID); got.IsOwner {
		t.Error("IsOwner true after revocation")
	}

	got, ok, err := src.Principal(ctx, ids.New())
	if err != nil || ok || got != (authz.PrincipalFacts{}) {
		t.Fatalf("unknown id: %+v, ok %v, %v", got, ok, err)
	}
}

func TestPrincipalSource_ActiveOwnerIDs(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	src := app.NewPrincipalSource(f.repo)
	ctx := context.Background()
	if got, err := src.ActiveOwnerIDs(ctx); err != nil || len(got) != 0 {
		t.Fatalf("no owners: %v, %v", got, err)
	}
	a := f.add(domain.KindUser, "alex")
	s := f.add(domain.KindUser, "sam")
	r := f.add(domain.KindUser, "robin")
	k := f.add(domain.KindUser, "kim")
	f.add(domain.KindUser, "noowner")
	// Grant in an order that differs from the id order.
	f.grant(r.ID, ids.ID{})
	f.grant(a.ID, r.ID)
	revoked := f.grant(s.ID, r.ID)
	f.grant(k.ID, r.ID)
	f.sql(`UPDATE owners SET revoked_at = now(), revoked_by = $2 WHERE id = $1`, revoked.ID.String(), r.ID.String())
	f.sql(`UPDATE principals SET status = 'disabled' WHERE id = $1`, k.ID.String())

	got, err := src.ActiveOwnerIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []ids.ID{a.ID, r.ID}
	slices.SortFunc(want, compare)
	if !slices.Equal(got, want) {
		t.Fatalf("ActiveOwnerIDs = %v, want %v", got, want)
	}
}

func compare(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) }

func TestPrincipalSource_JoinsTheAmbientTransaction(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	src := app.NewPrincipalSource(f.repo)
	user := f.add(domain.KindUser, "alex")
	f.do(func(ctx context.Context) error {
		o, err := domain.NewOwner(ids.New(), user.ID, ids.ID{}, "first owner")
		if err != nil {
			return err
		}
		if err := f.repo.InsertOwner(ctx, &o); err != nil {
			return err
		}
		if got, _ := src.ActiveOwnerIDs(ctx); !slices.Equal(got, []ids.ID{user.ID}) {
			t.Errorf("inside the unit: %v, want the new owner", got)
		}
		if got, _ := src.ActiveOwnerIDs(context.Background()); len(got) != 0 {
			t.Errorf("outside the unit: %v, want none before the commit", got)
		}
		return nil
	})
}

func TestDirectory(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	dir := app.NewDirectory(f.repo)
	ctx := context.Background()
	alex := f.add(domain.KindUser, "alex")
	sam := f.add(domain.KindUser, "sam")
	agent := f.add(domain.KindAgent, "helper")

	_, err := dir.Principal(ctx, ids.New())
	if errs.KindOf(err) != errs.KindNotFound {
		t.Fatalf("Principal(unknown) = %v, want not_found", err)
	}
	pv, err := dir.Principal(ctx, agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pv.ID != agent.ID || pv.Kind != actor.KindAgent || pv.Status != identity.StatusActive ||
		pv.Handle != "helper" || pv.DisplayName != "Name of helper" || !pv.AvatarFileID.IsZero() || pv.CreatedAt.IsZero() {
		t.Errorf("PrincipalView %+v", pv)
	}

	unknown := ids.New()
	m, err := dir.Principals(ctx, []ids.ID{alex.ID, unknown, agent.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m[alex.ID].Handle != "alex" || m[agent.ID].Kind != actor.KindAgent {
		t.Errorf("Principals = %+v", m)
	}
	if _, present := m[unknown]; present {
		t.Error("Principals contains an unknown id")
	}
	if m, err := dir.Principals(ctx, nil); err != nil || len(m) != 0 {
		t.Errorf("Principals(nil) = %v, %v", m, err)
	}

	byHandle, err := dir.PrincipalByHandle(ctx, "alex")
	if err != nil || byHandle.ID != alex.ID {
		t.Errorf("PrincipalByHandle(alex) = %+v, %v", byHandle, err)
	}
	if byHandle, err := dir.PrincipalByHandle(ctx, "Alex"); err != nil || byHandle.ID != alex.ID {
		t.Errorf("PrincipalByHandle(Alex) = %+v, %v", byHandle, err)
	}
	if _, err := dir.PrincipalByHandle(ctx, "nobody"); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("PrincipalByHandle(nobody) = %v, want not_found", err)
	}

	if _, err := dir.User(ctx, agent.ID); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("User(agent) = %v, want not_found", err)
	}
	if _, err := dir.User(ctx, ids.New()); errs.KindOf(err) != errs.KindNotFound {
		t.Errorf("User(unknown) = %v, want not_found", err)
	}
	uv, err := dir.User(ctx, alex.ID)
	if err != nil {
		t.Fatal(err)
	}
	if uv.Email != "alex@example.com" || uv.Locale != "en" || uv.TimeZone != "Europe/Berlin" || uv.IsOwner || uv.Kind != actor.KindUser {
		t.Errorf("UserView %+v", uv)
	}

	// ActiveOwners: ordered by granted_at, then id; revoked rows are left out.
	first := f.grant(sam.ID, ids.ID{})
	second := f.grant(alex.ID, sam.ID)
	owners, err := dir.ActiveOwners(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(owners) != 2 || owners[0].User.ID != sam.ID || owners[1].User.ID != alex.ID {
		t.Fatalf("ActiveOwners = %+v", owners)
	}
	if !owners[0].GrantedBy.IsZero() || owners[1].GrantedBy != sam.ID || owners[1].Reason != "needs the owner role" ||
		!owners[0].GrantedAt.Equal(first.GrantedAt) || !owners[1].GrantedAt.Equal(second.GrantedAt) {
		t.Errorf("ActiveOwners fields %+v", owners)
	}
	if uv, _ := dir.User(ctx, alex.ID); !uv.IsOwner {
		t.Error("UserView.IsOwner false for an owner")
	}
	// Same granted_at: the id decides.
	f.sql(`UPDATE owners SET granted_at = $1`, time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC))
	owners, _ = dir.ActiveOwners(ctx)
	wantOrder := []ids.ID{sam.ID, alex.ID}
	if compare(second.ID, first.ID) < 0 {
		wantOrder = []ids.ID{alex.ID, sam.ID}
	}
	if len(owners) != 2 || owners[0].User.ID != wantOrder[0] || owners[1].User.ID != wantOrder[1] {
		t.Errorf("ActiveOwners with equal granted_at not ordered by id: %+v", owners)
	}
	f.sql(`UPDATE owners SET revoked_at = now() WHERE id = $1`, first.ID.String())
	owners, _ = dir.ActiveOwners(ctx)
	if len(owners) != 1 || owners[0].User.ID != alex.ID {
		t.Errorf("ActiveOwners after revocation = %+v", owners)
	}
}

func TestDirectoryErrorsCarryNoSQL(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	dir := app.NewDirectory(f.repo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := dir.Principal(ctx, ids.New())
	if err == nil {
		t.Fatal("canceled context gave no error")
	}
	for _, s := range []string{"SELECT", "principals", "owners"} {
		if strings.Contains(err.Error(), s) {
			t.Errorf("error %q contains %q", err.Error(), s)
		}
	}
}
