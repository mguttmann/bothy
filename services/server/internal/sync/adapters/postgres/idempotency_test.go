// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// keyEnv is a database with the idempotency store of sync over the unit of work of platform/pg
// as bothy_app, the registry as internal/wire builds it.
type keyEnv struct {
	db    *pgtest.DB
	pool  *pg.Pool
	rows  *postgres.IdempotencyRepository
	uow   *pg.UnitOfWork
	store *app.IdempotencyStore
	clock *clock.Fake
}

// newKeyEnv builds a keyEnv whose units wait at most lockTimeout for a lock and retry retries
// times.
func newKeyEnv(t *testing.T, lockTimeout time.Duration, retries int) *keyEnv {
	t.Helper()
	db := pgtest.New(t)
	pool := db.Pool(t, pg.RoleApp)
	reg, err := wire.NewTxnRegistry(wire.NewModules(pool, nil))
	if err != nil {
		t.Fatal(err)
	}
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: retries, LockTimeout: lockTimeout, StatementTimeout: 30 * time.Second}, reg)
	rows := postgres.NewIdempotencyRepository(pool)
	return &keyEnv{db: db, pool: pool, rows: rows, uow: uow, store: app.NewIdempotencyStore(rows, uow),
		clock: clock.NewFake(time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC))}
}

// principal inserts an active user principal and returns its ID.
func (e *keyEnv) principal(t *testing.T) ids.ID {
	t.Helper()
	id := ids.New()
	if _, err := e.db.SuperConn(t).Exec(context.Background(),
		`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, 'Test')`,
		id.String(), "u"+fmt.Sprint(time.Now().UnixNano())); err != nil {
		t.Fatal(err)
	}
	return id
}

// request returns a request of principal p with a fresh key and the fingerprint of body.
func (e *keyEnv) request(p ids.ID, body string) idempotency.Request {
	return idempotency.Request{PrincipalID: p, Key: ids.New(), Action: "thing.create",
		SHA256: sha256.Sum256([]byte(body)), Now: e.clock.Now(), TTL: time.Hour}
}

// count returns the number of rows of idempotency_keys.
func (e *keyEnv) count(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.db.SuperConn(t).QueryRow(context.Background(), "SELECT count(*) FROM idempotency_keys").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// begin runs Claim in its own unit and commits; resp, when not nil, is completed in the unit.
func (e *keyEnv) begin(t *testing.T, req idempotency.Request, resp *idempotency.Response) (idempotency.Entry, bool) {
	t.Helper()
	var entry idempotency.Entry
	var found bool
	err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		var err error
		entry, found, err = e.store.Claim(ctx, req)
		if err != nil || found || resp == nil {
			return err
		}
		return e.store.Complete(ctx, req, *resp)
	})
	if err != nil {
		t.Fatal(err)
	}
	return entry, found
}

func TestStore_CompleteThenBeginFindsTheResponse(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 2*time.Second, 0)
	req := e.request(e.principal(t), "a")
	resID := ids.New()
	if _, found := e.begin(t, req, &idempotency.Response{Status: 201, Body: []byte(`{"id":"x","b":[1, 2]}`), ResourceID: resID}); found {
		t.Fatal("a new key was found")
	}
	got, found := e.begin(t, req, nil)
	if !found || !got.Completed || got.Status != 201 || got.ResourceID != resID || got.SHA256 != req.SHA256 {
		t.Fatalf("Claim after Complete = %+v, %v", got, found)
	}
	// jsonb keeps the value, not the bytes; the store hands out the compact form.
	if string(got.Body) != `{"b":[1,2],"id":"x"}` {
		t.Errorf("stored body %s, want the compact jsonb form", got.Body)
	}
}

func TestStore_RollbackLeavesNoKey(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 2*time.Second, 0)
	req := e.request(e.principal(t), "a")
	boom := errors.New("boom")
	err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		if _, _, err := e.store.Claim(ctx, req); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if n := e.count(t); n != 0 {
		t.Errorf("%d rows after a rollback, want 0", n)
	}
}

func TestStore_ClaimOutsideUnitIsInternal(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 2*time.Second, 0)
	if _, _, err := e.store.Claim(context.Background(), e.request(e.principal(t), "a")); err == nil {
		t.Fatal("Claim outside a unit of work succeeded")
	}
}

func TestStore_SaveFinalKeepsAnExistingRow(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 2*time.Second, 0)
	req := e.request(e.principal(t), "a")
	ctx := context.Background()
	if err := e.store.SaveFinal(ctx, req, idempotency.Response{Status: 404, Body: []byte(`{"code":"not_found"}`)}); err != nil {
		t.Fatal(err)
	}
	if err := e.store.SaveFinal(ctx, req, idempotency.Response{Status: 422, Body: []byte(`{"code":"other"}`)}); err != nil {
		t.Fatal(err)
	}
	got, found := e.begin(t, req, nil)
	if !found || got.Status != 404 || !got.Completed {
		t.Errorf("Claim = %+v, %v; want the first saved 404", got, found)
	}
}

func TestExpired_TreatedAsAbsentByTheStore(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 2*time.Second, 0)
	req := e.request(e.principal(t), "a")
	e.begin(t, req, &idempotency.Response{Status: 200, Body: []byte(`{}`)})
	later := req
	later.Now = req.Now.Add(req.TTL)
	later.SHA256 = sha256.Sum256([]byte("other"))
	if got, found := e.begin(t, later, &idempotency.Response{Status: 201, Body: []byte(`{"n":2}`)}); found {
		t.Fatalf("an expired key was found: %+v", got)
	}
	got, found := e.begin(t, later, nil)
	if !found || got.Status != 201 || got.SHA256 != later.SHA256 {
		t.Errorf("Claim = %+v, %v; want the new row", got, found)
	}
}

func TestPerPrincipal_SameKeyIndependentInTheStore(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 2*time.Second, 0)
	a := e.request(e.principal(t), "a")
	b := a
	b.PrincipalID = e.principal(t)
	e.begin(t, a, &idempotency.Response{Status: 200, Body: []byte(`{}`)})
	if _, found := e.begin(t, b, &idempotency.Response{Status: 200, Body: []byte(`{}`)}); found {
		t.Error("the key of another principal was found")
	}
}

// TestStore_WaitEndsInErrInProgress: a second unit with the same key waits at the primary key of
// the uncommitted first one; after the lock timeout Claim returns ErrInProgress, and the unit
// of work does not retry the wait (02 "Lock order").
func TestStore_WaitEndsInErrInProgress(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 200*time.Millisecond, 3)
	req := e.request(e.principal(t), "a")
	claimed, release := make(chan struct{}), make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- e.uow.Do(context.Background(), func(ctx context.Context) error {
			if _, _, err := e.store.Claim(ctx, req); err != nil {
				return err
			}
			close(claimed)
			<-release
			return e.store.Complete(ctx, req, idempotency.Response{Status: 200, Body: []byte(`{}`)})
		})
	}()
	<-claimed
	attempts := 0
	err := e.uow.Do(context.Background(), func(ctx context.Context) error {
		attempts++
		_, _, err := e.store.Claim(ctx, req)
		return err
	})
	close(release)
	if !errors.Is(err, idempotency.ErrInProgress) {
		t.Fatalf("second Claim = %v, want ErrInProgress", err)
	}
	if attempts != 1 {
		t.Errorf("the unit of work ran %d attempts, want 1 (no retry of the wait)", attempts)
	}
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if _, found := e.begin(t, req, nil); !found {
		t.Error("the first request's row is missing")
	}
}

// insertExpired inserts n rows of principal p that expire at at.
func (e *keyEnv) insertExpired(t *testing.T, p ids.ID, n int, at time.Time) {
	t.Helper()
	for range n {
		req := e.request(p, "x")
		req.Now = at.Add(-time.Hour)
		if err := e.store.SaveFinal(context.Background(), req, idempotency.Response{Status: 404, Body: []byte(`{}`)}); err != nil {
			t.Fatal(err)
		}
	}
}

func (e *keyEnv) cleanup(t *testing.T, batch int) *app.IdempotencyCleanup {
	t.Helper()
	c, err := app.NewIdempotencyCleanup(app.IdempotencyCleanupConfig{Interval: time.Hour, BatchSize: batch}, e.uow, e.rows, e.clock)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCleanupJob_DeletesExpiredInBatches(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 2*time.Second, 0)
	p := e.principal(t)
	now := e.clock.Now()
	e.insertExpired(t, p, 25, now)                 // expired exactly now
	e.insertExpired(t, p, 3, now.Add(time.Minute)) // still valid
	res, err := e.cleanup(t, 10).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 25 || res.Batches != 3 {
		t.Errorf("Run = %+v, want 25 deleted in 3 batches", res)
	}
	if n := e.count(t); n != 3 {
		t.Errorf("%d rows left, want the 3 valid ones", n)
	}
}

func TestCleanupJob_Idempotent(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 2*time.Second, 0)
	e.insertExpired(t, e.principal(t), 4, e.clock.Now())
	c := e.cleanup(t, 10)
	if _, err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := c.Run(context.Background())
	if err != nil || res.Deleted != 0 || res.Batches != 1 {
		t.Errorf("second Run = %+v, %v; want nothing deleted in one batch", res, err)
	}
}

func TestCleanupJob_StopsOnCancel(t *testing.T) {
	t.Parallel()
	e := newKeyEnv(t, 2*time.Second, 0)
	e.insertExpired(t, e.principal(t), 4, e.clock.Now())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.cleanup(t, 1).Run(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("Run with a cancelled context = %v, want context.Canceled", err)
	}
	if n := e.count(t); n != 4 {
		t.Errorf("%d rows left, want 4 (nothing deleted after the cancel)", n)
	}
}

// Compile-time check: the store is the kernel port.
var _ idempotency.Store = (*app.IdempotencyStore)(nil)
