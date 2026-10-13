// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// env is a migrated database with a pool of bothy_app, the repository and a unit of work
// without hooks (the repository tests need no audit_seq).
type env struct {
	db   *pgtest.DB
	pool *pg.Pool
	repo *postgres.Repository
	uow  *pg.UnitOfWork
}

func newEnv(t *testing.T, maxConns int) *env {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = maxConns
	pool := db.OpenPool(t, cfg)
	reg := txn.NewRegistry()
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: cfg.StatementTimeout, LockTimeout: cfg.LockTimeout, IdleInTxTimeout: cfg.IdleInTxTimeout}, reg)
	return &env{db: db, pool: pool, repo: postgres.NewRepository(pool), uow: uow}
}

// fullRow returns a chained row with every optional column set.
func fullRow(seq int64, prev [domain.HashSize]byte) domain.Row {
	r := domain.Row{
		Seq: seq, ID: ids.New(), OccurredAt: time.Date(2026, 10, 12, 8, 0, 0, 120_000_000, time.UTC),
		ActorPrincipalID: ids.New(), ActorDeviceID: ids.New(), ActorSessionID: ids.New(),
		Action: "grant.create", Decision: "deny", DenyReason: "no_grant", ResourceKind: "grant", ResourceID: ids.New(),
		Details: map[string]any{
			"ip_hmac": strings.Repeat("ab", 32), "ip_key_id": ids.New().String(), "n": int64(1) << 52,
			"aggregate": map[string]any{"count": int64(2), "first_at": "2026-10-12T08:00:00.120Z", "last_at": "2026-10-12T08:00:01.000Z"},
			"reason":    "Révision   <a&b>",
		},
		CorrelationID: ids.MustParse("01928f3a-7b1c-7000-8a00-0000000000c1"), PrevHash: prev,
	}
	c, err := domain.CanonicalRow(r)
	if err != nil {
		panic(err)
	}
	r.Hash = domain.ChainHash(prev, c)
	return r
}

func TestRepository_InsertRowsRoundTrip(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 4)
	ctx := context.Background()
	r1 := fullRow(1, domain.ZeroHash)
	r2 := domain.Row{Seq: 2, ID: ids.New(), OccurredAt: r1.OccurredAt, Action: "a.b", Decision: "allow", Details: map[string]any{}, PrevHash: r1.Hash}
	c, _ := domain.CanonicalRow(r2)
	r2.Hash = domain.ChainHash(r1.Hash, c)
	if err := e.uow.Do(ctx, func(ctx context.Context) error { return e.repo.InsertRows(ctx, []domain.Row{r1, r2}) }); err != nil {
		t.Fatal(err)
	}
	got, err := e.repo.Rows(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d rows", len(got))
	}
	for i, want := range []domain.Row{r1, r2} {
		c, err := domain.CanonicalRow(got[i])
		if err != nil {
			t.Fatal(err)
		}
		if h := domain.ChainHash(got[i].PrevHash, c); h != want.Hash || got[i].Hash != want.Hash {
			t.Fatalf("row %d does not recompute after the round trip:\n%s", want.Seq, c)
		}
	}
	if got[1].DenyReason != "" || !got[1].ActorPrincipalID.IsZero() || got[1].CorrelationID != (ids.ID{}) {
		t.Fatalf("NULL columns came back set: %+v", got[1])
	}
	h, ok, err := e.repo.HashAt(ctx, 2)
	if err != nil || !ok || h != r2.Hash {
		t.Fatalf("HashAt(2) = %x, %v, %v", h, ok, err)
	}
	if _, ok, err := e.repo.HashAt(ctx, 3); err != nil || ok {
		t.Fatalf("HashAt(3) = %v, %v; want missing", ok, err)
	}
}

func TestRepository_InsertRowsOutsideUnitFails(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 2)
	if err := e.repo.InsertRows(context.Background(), []domain.Row{fullRow(1, domain.ZeroHash)}); err == nil {
		t.Fatal("InsertRows outside a unit of work succeeded")
	}
}

// kek returns a KEK source with one key under ref.
type kek map[string][]byte

func (k kek) KEK(_ context.Context, ref string) ([]byte, error) { return k[ref], nil }

const ref = "file:<secrets.dir>/audit_ip_kek"

var kekHex = []byte(strings.Repeat("5a", 32))

func TestAuditIPKeys_OneKeyPerPeriodUnderConcurrency(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 8)
	now := time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)
	const processes = 8
	got := make([]ids.ID, processes)
	keys := make([][]byte, processes)
	errs := make([]error, processes)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range processes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			kr := app.NewKeyring(e.repo, e.uow, kek{ref: kekHex}, ref, 30*24*time.Hour) // one per process
			<-start
			got[i], keys[i], errs[i] = kr.Current(context.Background(), now)
		}()
	}
	close(start)
	wg.Wait()
	for i := range processes {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if got[i] != got[0] || !bytes.Equal(keys[i], keys[0]) {
			t.Fatal("two processes use different keys for the same period")
		}
	}
	var n int
	if err := e.pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_ip_keys").Scan(&n); err != nil || n != 1 {
		t.Fatalf("audit_ip_keys has %d rows: %v", n, err)
	}
}

func TestAuditIPKeys_NewPeriodNewKey(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 4)
	ctx := context.Background()
	period := time.Hour
	kr := app.NewKeyring(e.repo, e.uow, kek{ref: kekHex}, ref, period)
	now := time.Date(2026, 10, 12, 8, 30, 0, 0, time.UTC)
	id1, k1, err := kr.Current(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	id2, k2, err := kr.Current(ctx, now.Add(period))
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 || bytes.Equal(k1, k2) {
		t.Fatal("a new period reused the key")
	}
	rec, ok, err := e.repo.IPKeyByID(ctx, id2)
	if err != nil || !ok {
		t.Fatalf("IPKeyByID = %v, %v", ok, err)
	}
	if !rec.ValidFrom.Equal(time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)) || !rec.ValidUntil.Equal(rec.ValidFrom.Add(period)) || rec.KeyRef != ref {
		t.Fatalf("row = %+v", rec)
	}
	if _, ok, err := e.repo.IPKeyByID(ctx, ids.New()); err != nil || ok {
		t.Fatalf("IPKeyByID of an unknown id = %v, %v", ok, err)
	}
}

func TestIPKey_EncryptedAtRestInTheDatabase(t *testing.T) {
	t.Parallel()
	e := newEnv(t, 4)
	ctx := context.Background()
	kr := app.NewKeyring(e.repo, e.uow, kek{ref: kekHex}, ref, 30*24*time.Hour)
	id, key, err := kr.Current(ctx, time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var stored []byte
	var storedRef string
	if err := e.pool.QueryRow(ctx, "SELECT key_ciphertext, key_ref FROM audit_ip_keys WHERE id = $1", id.String()).Scan(&stored, &storedRef); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, key) || storedRef != ref {
		t.Fatal("the stored key is not encrypted or names another reference")
	}
	kekBytes, _ := domain.DecodeKEK(kekHex)
	if opened, err := domain.OpenIPKey(kekBytes, stored, id); err != nil || !bytes.Equal(opened, key) {
		t.Fatalf("the configured KEK does not open the stored key: %v", err)
	}
	if _, err := domain.OpenIPKey(bytes.Repeat([]byte{1}, 32), stored, id); err == nil {
		t.Fatal("another KEK opened the stored key")
	}
	// A destroyed key (column cleared, M7-05) is never used again, also by a fresh process.
	if _, err := e.pool.Exec(ctx, "UPDATE audit_ip_keys SET key_ciphertext = NULL, destroyed_at = now()"); err != nil {
		t.Fatal(err)
	}
	fresh := app.NewKeyring(e.repo, e.uow, kek{ref: kekHex}, ref, 30*24*time.Hour)
	if _, _, err := fresh.Current(ctx, time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("a destroyed key was used")
	}
}
