// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
)

// env is a database with the repository and a plain unit of work (no hooks).
type env struct {
	repo  *postgres.Repository
	uow   *pg.UnitOfWork
	owner *pg.Pool
}

func newEnv(t *testing.T) env {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	return env{repo: postgres.NewRepository(pool), owner: db.Pool(t, pg.RoleOwner),
		uow: pg.NewUnitOfWork(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)}
}

// person inserts a person and returns its id.
func (e env) person(t *testing.T, handle string) ids.ID {
	t.Helper()
	id := ids.New()
	if _, err := e.owner.Exec(context.Background(), `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`,
		id.String(), handle); err != nil {
		t.Fatal(err)
	}
	if _, err := e.owner.Exec(context.Background(), `INSERT INTO users (principal_id) VALUES ($1)`, id.String()); err != nil {
		t.Fatal(err)
	}
	return id
}

// write runs fn in a unit of work.
func (e env) write(fn func(ctx context.Context) error) error {
	return e.uow.Do(context.Background(), fn)
}

// TestRepository_RoundTrip (M2-39): a host is inserted in pairing and read back; a live name in
// another case is 409 conflict; the update writes name, holder, state and version; the lists of
// ids, the holder index and the keyset page; session users are inserted, listed, ended, and a
// taken name or a second valid assignment is 409 session_user_taken; open sessions are locked and
// closed by the server; writes outside a unit of work fail.
func TestRepository_RoundTrip(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	robin, alex := e.person(t, "robin"), e.person(t, "alex")
	at := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	h := domain.Host{ID: ids.New(), Name: "Build server", State: domain.StatePairing, PairingCodeHash: []byte{1, 2, 3},
		PairingExpiresAt: at.Add(15 * time.Minute), CreatedBy: robin, CreatedAt: at, Version: 1}
	if err := e.repo.InsertHost(ctx, h); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("insert outside a unit of work: %v", err)
	}
	if err := e.write(func(ctx context.Context) error { return e.repo.InsertHost(ctx, h) }); err != nil {
		t.Fatal(err)
	}
	twin := h
	twin.ID, twin.Name, twin.PairingCodeHash = ids.New(), "BUILD SERVER", []byte{4}
	if err := e.write(func(ctx context.Context) error { return e.repo.InsertHost(ctx, twin) }); errs.KindOf(err) != errs.KindConflict {
		t.Fatalf("a live name in another case: %v", err)
	}
	got, found, err := e.repo.Host(ctx, h.ID)
	if err != nil || !found || got.Name != h.Name || got.State != domain.StatePairing || !got.PairingExpiresAt.Equal(h.PairingExpiresAt) ||
		got.LLMCredentials != "none" || len(got.Profiles) != 0 || got.Paired() {
		t.Fatalf("Host = %+v, %v, %v", got, found, err)
	}
	if _, found, err := e.repo.Host(ctx, ids.New()); found || err != nil {
		t.Fatalf("unknown host: %v %v", found, err)
	}
	r1, err := terminaltest.InsertHost(ctx, e.owner, terminaltest.Host{Name: "Lab", State: "active", AssignedUserID: alex, CreatedBy: robin,
		ReportedSessionUsers: []string{"bothy-s-alex"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.write(func(ctx context.Context) error {
		cur, found, err := e.repo.LockHost(ctx, r1)
		if err != nil || !found || !cur.Paired() || cur.ReportedUsers[0].OSUser != "bothy-s-alex" || cur.KeyFingerprint != terminaltest.KeyFingerprint {
			t.Errorf("LockHost = %+v, %v, %v", cur, found, err)
		}
		cur.Name, cur.AssignedUserID = "Lab two", ids.ID{}
		cur = cur.Apply(domain.Result{To: domain.StateDisabled, DisabledReason: domain.DisabledOwner}, at)
		return e.repo.UpdateHost(ctx, cur)
	}); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := e.repo.Host(ctx, r1); got.Name != "Lab two" || got.State != domain.StateDisabled || got.DisabledReason != "owner" ||
		!got.AssignedUserID.IsZero() || got.Version != 2 || got.DisabledAt.IsZero() {
		t.Fatalf("after the update %+v", got)
	}
	all, err := e.repo.HostIDs(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("HostIDs %v %v", all, err)
	}
	if mine, err := e.repo.AssignedTo(ctx, alex); err != nil || len(mine) != 0 {
		t.Fatalf("AssignedTo after clearing the holder %v %v", mine, err)
	}
	page, err := e.repo.ListHosts(ctx, true, nil, "", ids.ID{}, 1)
	if err != nil || len(page) != 1 || page[0].ID != h.ID {
		t.Fatalf("first page %v %v", page, err)
	}
	page, err = e.repo.ListHosts(ctx, false, []ids.ID{r1}, page[0].Name, page[0].ID, 5)
	if err != nil || len(page) != 1 || page[0].ID != r1 {
		t.Fatalf("second page %v %v", page, err)
	}

	su := ids.New()
	if err := e.write(func(ctx context.Context) error {
		if err := e.repo.InsertSessionUser(ctx, su, r1, domain.SessionUserAssign{UserID: alex, OSUser: "bothy-s-alex"}, robin, at); err != nil {
			return err
		}
		rows, err := e.repo.LockSessionUsers(ctx, r1)
		if err != nil || len(rows) != 1 || rows[0].Revoked {
			t.Errorf("LockSessionUsers %v %v", rows, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, a := range []domain.SessionUserAssign{{UserID: robin, OSUser: "BOTHY-S-ALEX"}, {UserID: alex, OSUser: "bothy-s-other"}} {
		err := e.write(func(ctx context.Context) error { return e.repo.InsertSessionUser(ctx, ids.New(), r1, a, robin, at) })
		if x, ok := errs.As(err); !ok || x.Code != errs.CodeConflict || x.Reason != domain.ReasonSessionUserTaken {
			t.Errorf("%+v: %v", a, err)
		}
	}
	valid, err := e.repo.ValidSessionUsers(ctx, []ids.ID{r1, h.ID})
	if err != nil || len(valid[r1]) != 1 || valid[r1][0].OSUser != "bothy-s-alex" || len(valid[h.ID]) != 0 {
		t.Fatalf("ValidSessionUsers %v %v", valid, err)
	}
	s1, err := terminaltest.InsertSession(ctx, e.owner, terminaltest.Session{HostID: r1, UserID: alex, SessionUserID: su, State: "detached", BytesIn: 7})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.write(func(ctx context.Context) error {
		open, err := e.repo.LockOpenSessions(ctx, r1, alex)
		if err != nil || len(open) != 1 || open[0].ID != s1 || open[0].BytesIn != 7 {
			t.Errorf("LockOpenSessions %v %v", open, err)
		}
		if err := e.repo.EndSession(ctx, s1, domain.ServerEnd(domain.CloseAccessRevoked), at); err != nil {
			return err
		}
		// A second close changes no row: an internal error that rolls the unit back.
		return e.repo.EndSession(ctx, s1, domain.ServerEnd(domain.CloseAccessRevoked), at)
	}); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("closing a closed session: %v", err)
	}
	if err := e.write(func(ctx context.Context) error {
		if err := e.repo.EndSession(ctx, s1, domain.ServerEnd(domain.CloseAccessRevoked), at); err != nil {
			return err
		}
		return e.repo.RevokeSessionUser(ctx, su, robin, at)
	}); err != nil {
		t.Fatal(err)
	}
	if valid, _ := e.repo.ValidSessionUsers(ctx, []ids.ID{r1}); len(valid[r1]) != 0 {
		t.Fatal("an ended assignment is still valid")
	}
	if err := e.write(func(ctx context.Context) error { return e.repo.RevokeSessionUser(ctx, su, robin, at) }); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("ending an ended assignment: %v", err)
	}
	if err := e.write(func(ctx context.Context) error {
		open, err := e.repo.LockOpenSessions(ctx, r1, ids.ID{})
		if err != nil || len(open) != 0 {
			t.Errorf("open sessions after the end: %v %v", open, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if rows, err := e.repo.SessionUsers(ctx, r1); err != nil || len(rows) != 1 || !rows[0].Revoked || rows[0].OSUser != "bothy-s-alex" {
		t.Fatalf("SessionUsers %v %v", rows, err)
	}
	if _, err := e.owner.Exec(ctx, `UPDATE hosts SET reported_is_server_host = true, reported_require_local_confirm = false,
		reported_os_version = 'Debian 13', reported_host_version = '1.0.0', last_seen_at = now() WHERE id = $1`, r1.String()); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := e.repo.Host(ctx, r1); got.ReportedIsServerHost == nil || !*got.ReportedIsServerHost || got.RequireLocalConfirm == nil ||
		*got.RequireLocalConfirm || got.OSVersion != "Debian 13" || got.HostVersion != "1.0.0" || got.LastSeenAt.IsZero() {
		t.Fatalf("reported columns %+v", got)
	}
	if none, err := e.repo.ListHosts(ctx, true, nil, "", ids.ID{}, -1); err != nil || len(none) != 0 {
		t.Fatalf("a negative limit lists %v, %v", none, err)
	}
	if err := e.repo.UpdateHost(ctx, got); errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("an update outside a unit of work: %v", err)
	}
	ids1, _ := e.repo.HostIDs(ctx)
	if !slices.Contains(ids1, r1) {
		t.Fatal("HostIDs lost a host")
	}
}
