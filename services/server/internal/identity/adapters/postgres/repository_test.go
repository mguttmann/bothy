// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// env is one test database with the repository, a unit of work and a pool of bothy_app.
type env struct {
	repo *postgres.Repository
	uow  *pg.UnitOfWork
	pool *pg.Pool
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool := pgtest.New(t).Pool(t, pg.RoleApp)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{
		StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second,
	}, reg)
	return env{repo: postgres.NewRepository(pool), uow: uow, pool: pool}
}

// write runs fn in a unit of work.
func (e env) write(t *testing.T, fn func(ctx context.Context) error) error {
	t.Helper()
	return e.uow.Do(context.Background(), fn)
}

func principal(t *testing.T, kind domain.Kind, handle, name string) domain.Principal {
	t.Helper()
	h, err := domain.NewHandle(handle, 32)
	if err != nil {
		t.Fatal(err)
	}
	d, err := domain.NewDisplayName(name, 64)
	if err != nil {
		t.Fatal(err)
	}
	p, err := domain.NewPrincipal(ids.New(), kind, h, d)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// addUser inserts a person with email in one unit of work.
func (e env) addUser(t *testing.T, handle, email string) (domain.User, error) {
	t.Helper()
	u, err := domain.NewUser(principal(t, domain.KindUser, handle, strings.ToUpper(handle[:1])+handle[1:]), email, "", "")
	if err != nil {
		t.Fatal(err)
	}
	err = e.write(t, func(ctx context.Context) error {
		if err := e.repo.InsertPrincipal(ctx, &u.Principal); err != nil {
			return err
		}
		return e.repo.InsertUser(ctx, u)
	})
	return u, err
}

func (e env) mustAddUser(t *testing.T, handle, email string) domain.User {
	t.Helper()
	u, err := e.addUser(t, handle, email)
	if err != nil {
		t.Fatalf("add user %s: %v", handle, err)
	}
	return u
}

func (e env) addOwner(t *testing.T, userID ids.ID) (domain.Owner, error) {
	t.Helper()
	o, err := domain.NewOwner(ids.New(), userID, ids.ID{}, "first owner")
	if err != nil {
		t.Fatal(err)
	}
	err = e.write(t, func(ctx context.Context) error { return e.repo.InsertOwner(ctx, &o) })
	return o, err
}

// wantError checks kind, code and field pointer of err and that its text names no value.
func wantError(t *testing.T, err error, kind errs.Kind, code errs.Code, pointer string, values ...string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok {
		t.Fatalf("error %v (%T) is no *errs.Error", err, err)
	}
	if e.Kind() != kind || e.Code != code {
		t.Fatalf("error %v: kind %v code %q, want %v %q", err, e.Kind(), e.Code, kind, code)
	}
	if pointer != "" && (len(e.Fields) != 1 || e.Fields[0].Pointer != pointer) {
		t.Fatalf("fields %+v, want one on %s", e.Fields, pointer)
	}
	texts := []string{e.Error(), e.Detail}
	for _, f := range e.Fields {
		texts = append(texts, f.Detail, f.Code)
	}
	for _, s := range texts {
		for _, v := range append(values, "INSERT", "SELECT", "UPDATE", "principals_", "users_", "owners_") {
			if strings.Contains(s, v) {
				t.Errorf("error text %q contains %q", s, v)
			}
		}
	}
}

func TestInsertAndReadBack(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	u := e.mustAddUser(t, "alex", "alex@example.com")
	if u.Principal.CreatedAt.IsZero() {
		t.Error("InsertPrincipal did not set CreatedAt")
	}
	ctx := context.Background()
	got, found, err := e.repo.User(ctx, u.Principal.ID)
	if err != nil || !found {
		t.Fatalf("User: %v, found %v", err, found)
	}
	if got.User.Principal.Handle.String() != "alex" || got.User.Email != "alex@example.com" ||
		got.User.Locale != "de" || got.User.TimeZone != "UTC" || got.IsOwner {
		t.Fatalf("read back %+v", got)
	}
	if !got.User.Principal.CreatedAt.Equal(u.Principal.CreatedAt) {
		t.Errorf("CreatedAt %v, want %v", got.User.Principal.CreatedAt, u.Principal.CreatedAt)
	}
}

func TestWritesNeedAUnitOfWork(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	p := principal(t, domain.KindAgent, "helper", "Helper")
	err := e.repo.InsertPrincipal(context.Background(), &p)
	wantError(t, err, errs.KindInternal, errs.CodeInternalError, "")
}

func TestHandleUniqueCaseInsensitive(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.mustAddUser(t, "alex", "")
	_, err := e.addUser(t, "alex", "")
	wantError(t, err, errs.KindConflict, errs.CodeConflict, "/handle", "alex")

	// The domain refuses upper case; the database check refuses it as well, and the unique index
	// on lower(handle) would catch it if the check ever widened.
	_, err = e.pool.Exec(context.Background(),
		`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'agent', 'Alex', 'Alex')`, ids.New().String())
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "principals_handle_check" {
		t.Fatalf("raw insert of Alex: %v, want check violation of principals_handle_check", err)
	}
}

func TestEmailUniqueCaseInsensitive(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.mustAddUser(t, "alex", "alex@example.com")
	_, err := e.addUser(t, "alex2", "Alex@example.com")
	wantError(t, err, errs.KindConflict, errs.CodeConflict, "/email", "Alex@example.com", "alex@example.com")
	e.mustAddUser(t, "sam", "")
	e.mustAddUser(t, "robin", "")
}

func TestUsersRowRequiresUserKind(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	agent := principal(t, domain.KindAgent, "helper", "Helper")
	err := e.write(t, func(ctx context.Context) error {
		if err := e.repo.InsertPrincipal(ctx, &agent); err != nil {
			return err
		}
		// Built by hand: domain.NewUser refuses an agent, the trigger is the second line.
		return e.repo.InsertUser(ctx, domain.User{Principal: agent, Locale: "de", TimeZone: "UTC"})
	})
	wantError(t, err, errs.KindBadRequest, errs.CodeValidationFailed, "/principal_id")
}

func TestPrincipalKindImmutable(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	u := e.mustAddUser(t, "alex", "")
	_, err := e.pool.Exec(context.Background(), `UPDATE principals SET kind = 'agent' WHERE id = $1`, u.Principal.ID.String())
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("kind change: %v, want SQLSTATE 23514", err)
	}
}

func TestOneActiveOwnerRowPerUser(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	u := e.mustAddUser(t, "alex", "")
	first, err := e.addOwner(t, u.Principal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.GrantedAt.IsZero() {
		t.Error("InsertOwner did not set GrantedAt")
	}
	_, err = e.addOwner(t, u.Principal.ID)
	wantError(t, err, errs.KindConflict, errs.CodeConflict, "/user_id")

	if _, err := e.pool.Exec(context.Background(), `UPDATE owners SET revoked_at = now() WHERE id = $1`, first.ID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.addOwner(t, u.Principal.ID); err != nil {
		t.Fatalf("owner row after revocation: %v", err)
	}

	agent := principal(t, domain.KindAgent, "helper", "Helper")
	if err := e.write(t, func(ctx context.Context) error { return e.repo.InsertPrincipal(ctx, &agent) }); err != nil {
		t.Fatal(err)
	}
	_, err = e.addOwner(t, agent.ID)
	wantError(t, err, errs.KindBadRequest, errs.CodeValidationFailed, "/user_id")
}

func TestOwnerGrantedByMustExist(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	u := e.mustAddUser(t, "alex", "")
	o, err := domain.NewOwner(ids.New(), u.Principal.ID, ids.New(), "granted by a ghost")
	if err != nil {
		t.Fatal(err)
	}
	err = e.write(t, func(ctx context.Context) error { return e.repo.InsertOwner(ctx, &o) })
	wantError(t, err, errs.KindBadRequest, errs.CodeValidationFailed, "/granted_by")
}

func TestReadsJoinTheAmbientTransaction(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	agent := principal(t, domain.KindAgent, "helper", "Helper")
	err := e.write(t, func(ctx context.Context) error {
		if err := e.repo.InsertPrincipal(ctx, &agent); err != nil {
			return err
		}
		// Not committed yet: only the ambient transaction sees the row.
		got, found, err := e.repo.Principal(ctx, agent.ID)
		if err != nil || !found || got.Principal.Handle.String() != "helper" {
			t.Errorf("read inside the unit: %+v, %v, %v", got, found, err)
		}
		_, found, err = e.repo.Principal(context.Background(), agent.ID)
		if err != nil || found {
			t.Errorf("read outside the unit saw the uncommitted row: %v, %v", found, err)
		}
		return errors.New("roll back")
	})
	if err == nil || err.Error() != "roll back" {
		t.Fatalf("unit: %v", err)
	}
	if _, found, _ := e.repo.Principal(context.Background(), agent.ID); found {
		t.Fatal("rolled back row is visible")
	}
}

func TestPrincipalsByIDsAndHandle(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	alex := e.mustAddUser(t, "alex", "")
	sam := e.mustAddUser(t, "sam", "")
	got, err := e.repo.Principals(ctx, []ids.ID{sam.Principal.ID, ids.New(), alex.Principal.ID, alex.Principal.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("Principals returned %d, want 2", len(got))
	}
	if got, err := e.repo.Principals(ctx, nil); err != nil || len(got) != 0 {
		t.Fatalf("Principals(nil) = %v, %v", got, err)
	}
	p, found, err := e.repo.PrincipalByHandle(ctx, "ALEX")
	if err != nil || !found || p.ID != alex.Principal.ID {
		t.Fatalf("PrincipalByHandle(ALEX) = %+v, %v, %v", p, found, err)
	}
	if _, found, err := e.repo.PrincipalByHandle(ctx, "nobody"); err != nil || found {
		t.Fatalf("PrincipalByHandle(nobody) found %v, %v", found, err)
	}
}
