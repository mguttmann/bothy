// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// testKEK is the KEK source of the audit tests: one key under every reference.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(strings.Repeat("6b", 32)), nil
}

// auditEnv is the audit module over a fresh database.
type auditEnv struct {
	db    *pgtest.DB
	pool  *pg.Pool
	audit *wire.Audit
	repo  *auditpg.Repository
}

func newAuditEnv(t testing.TB, maxConns int, window time.Duration) *auditEnv {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = maxConns
	pool := db.OpenPool(t, cfg)
	unit := pg.UnitConfig{RetryMax: 5, StatementTimeout: cfg.StatementTimeout, LockTimeout: cfg.LockTimeout, IdleInTxTimeout: cfg.IdleInTxTimeout}
	a, err := wire.NewAudit(pool, unit, auditapp.Config{
		DenyAggregateWindow: window, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, testKEK{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return &auditEnv{db: db, pool: pool, audit: a, repo: auditpg.NewRepository(pool)}
}

// auditSeq reads the committed value of audit_seq.
func (e *auditEnv) auditSeq(t testing.TB) int64 {
	t.Helper()
	var v int64
	if err := e.pool.QueryRow(context.Background(), "SELECT value FROM sequence_counters WHERE name = 'audit_seq'").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// verifyChain reads rows 1 .. n back and checks gaplessness, the links and every hash.
func (e *auditEnv) verifyChain(t *testing.T, n int64) []domain.Row {
	t.Helper()
	rows, err := e.repo.Rows(context.Background(), 1, n+1)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(rows)) != n {
		t.Fatalf("read %d rows, want %d", len(rows), n)
	}
	prev := domain.ZeroHash
	for i, r := range rows {
		if r.Seq != int64(i+1) {
			t.Fatalf("row %d has seq %d: a gap", i+1, r.Seq)
		}
		if r.PrevHash != prev {
			t.Fatalf("row %d: prev_hash is not the hash of row %d", r.Seq, r.Seq-1)
		}
		c, err := domain.CanonicalRow(r)
		if err != nil {
			t.Fatalf("row %d: %v", r.Seq, err)
		}
		if domain.ChainHash(prev, c) != r.Hash {
			t.Fatalf("row %d: the hash does not recompute from the row read back", r.Seq)
		}
		prev = r.Hash
	}
	return rows
}

var errRollback = errors.New("business rollback")

func person() context.Context {
	return actor.WithActor(context.Background(), actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New()})
}

func TestAuditChain_ConcurrentWritersGapless(t *testing.T) {
	t.Parallel()
	e := newAuditEnv(t, 12, time.Minute)
	const writers, unitsEach = 12, 15
	var committed atomic.Int64
	var wg sync.WaitGroup
	errc := make(chan error, writers)
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := reqmeta.WithCorrelationID(person(), fmt.Sprintf("4bf92f35-77b3-4da6-a3ce-%012d", w))
			for range unitsEach {
				k := 1 + rand.IntN(3)         //nolint:gosec // test spread
				rollback := rand.IntN(3) == 0 //nolint:gosec // test spread
				err := e.audit.UoW().Do(ctx, func(ctx context.Context) error {
					for i := range k {
						if err := e.audit.Recorder().Record(ctx, auditing.Entry{
							Action: "grant.create", Decision: auditing.DecisionAllow,
							Attrs: auditing.Attrs{"writer": auditing.Int(int64(w)), "i": auditing.Int(int64(i))},
						}); err != nil {
							return err
						}
					}
					if rollback {
						return errRollback
					}
					return nil
				})
				switch {
				case rollback && errors.Is(err, errRollback):
				case !rollback && err == nil:
					committed.Add(int64(k))
				default:
					errc <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errc)
	for err := range errc {
		t.Fatal(err)
	}
	if got := e.auditSeq(t); got != committed.Load() {
		t.Fatalf("audit_seq = %d, committed entries %d", got, committed.Load())
	}
	e.verifyChain(t, committed.Load())
}

func TestAudit_InTxEntryRolledBackWithBusinessTx(t *testing.T) {
	t.Parallel()
	e := newAuditEnv(t, 4, time.Minute)
	err := e.audit.UoW().Do(person(), func(ctx context.Context) error {
		if err := e.audit.Recorder().Record(ctx, auditing.Entry{Action: "grant.create", Decision: auditing.DecisionAllow}); err != nil {
			return err
		}
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
	if e.auditSeq(t) != 0 {
		t.Fatal("the rollback kept the audit number")
	}
	e.verifyChain(t, 0)
}

func TestAudit_DenialSurvivesBusinessRollback(t *testing.T) {
	t.Parallel()
	e := newAuditEnv(t, 4, time.Hour)
	ctx := person()
	rec := e.audit.Recorder()
	err := e.audit.UoW().Do(ctx, func(ctx context.Context) error {
		// An error entry (unknown_action) is written at once, outside the failing unit.
		if err := rec.RecordDetached(ctx, auditing.Entry{Action: "a.unknown", Decision: auditing.DecisionError, Reason: auditing.ReasonUnknownAction}); err != nil {
			return err
		}
		return errRollback // for example an I1 violation in phase 3.1
	})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
	for range 3 {
		if err := rec.RecordDenial(ctx, auditing.Entry{Action: "grant.create", Decision: auditing.DecisionDeny, Reason: auditing.ReasonI1Violation}); err != nil {
			t.Fatal(err)
		}
	}
	if err := rec.Aggregator().Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	rows := e.verifyChain(t, 2)
	if rows[0].Decision != "error" || rows[1].Decision != "deny" || rows[1].DenyReason != "i1_violation" {
		t.Fatalf("rows = %+v", rows)
	}
	agg, _ := rows[1].Details[domain.DetailAggregate].(map[string]any)
	if fmt.Sprint(agg["count"]) != "3" {
		t.Fatalf("aggregate = %v", agg)
	}
}

func TestAudit_RecordOutsideTransactionFails(t *testing.T) {
	t.Parallel()
	e := newAuditEnv(t, 2, time.Minute)
	err := e.audit.Recorder().Record(person(), auditing.Entry{Action: "grant.create", Decision: auditing.DecisionAllow})
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("Record outside a unit = %v, want kind internal", err)
	}
	e.verifyChain(t, 0)
}

func TestAudit_NoEntriesNoCounterLock(t *testing.T) {
	t.Parallel()
	e := newAuditEnv(t, 4, time.Minute)
	ctx := context.Background()
	// Another transaction holds the row lock of audit_seq; a unit without entries must not wait.
	holder, err := e.pool.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Rollback(ctx) }()
	// The holder stays idle in its transaction; without this the idle timeout of the test
	// database would end it and release the lock before testkit.Patience.
	if _, err := holder.Exec(ctx, "SET LOCAL idle_in_transaction_session_timeout = 0"); err != nil {
		t.Fatal(err)
	}
	if _, err := holder.Exec(ctx, "SELECT value FROM sequence_counters WHERE name = 'audit_seq' FOR UPDATE"); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	if err := e.audit.UoW().Do(person(), func(context.Context) error { return nil }); err != nil {
		t.Fatalf("a unit without entries failed while audit_seq was locked: %v", err)
	}
	// The holder keeps the lock until the test ends: a unit that waited for it would fail with
	// the lock timeout of the unit or wait past testkit.Patience.
	if d := time.Since(began); d > testkit.Patience {
		t.Fatalf("a unit without entries waited %v", d)
	}
	if e.auditSeq(t) != 0 {
		t.Fatal("audit_seq moved without entries")
	}
}

func TestAudit_IPStoredOnlyAsHMAC(t *testing.T) {
	t.Parallel()
	e := newAuditEnv(t, 4, time.Minute)
	ctx := reqmeta.WithClientIP(actor.WithActor(context.Background(), actor.Anonymous()), netip.MustParseAddr("192.0.2.44"))
	err := e.audit.UoW().Do(ctx, func(ctx context.Context) error {
		return e.audit.Recorder().Record(ctx, auditing.Entry{Action: "auth.login.begin", Decision: auditing.DecisionAllow})
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := e.verifyChain(t, 1)
	keyID, _ := rows[0].Details[domain.DetailIPKeyID].(string)
	hmacHex, _ := rows[0].Details[domain.DetailIPHMAC].(string)
	if len(hmacHex) != 64 || keyID == "" {
		t.Fatalf("details = %v", rows[0].Details)
	}
	var text string
	if err := e.pool.QueryRow(context.Background(), "SELECT a::text FROM audit_events a").Scan(&text); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "192.0.2.44") {
		t.Fatalf("the row contains the raw IP: %s", text)
	}
	var n int
	if err := e.pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_ip_keys WHERE id = $1::uuid", keyID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("ip_key_id names %d keys: %v", n, err)
	}
}

func TestAudit_StartFlushesBeforeThePoolCloses(t *testing.T) {
	t.Parallel()
	e := newAuditEnv(t, 4, time.Hour)
	var order []string
	var mu sync.Mutex
	note := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, s)
	}
	sigs := make(chan os.Signal, 1)
	r := lifecycle.New(lifecycle.Options{TotalTimeout: 30 * time.Second, Signals: sigs, Logger: slog.New(slog.DiscardHandler)})
	r.OnStop(lifecycle.StopStep{Name: "pool", Phase: lifecycle.PhaseDatabase, Run: func(context.Context) error {
		var n int
		if err := e.pool.QueryRow(context.Background(), "SELECT count(*) FROM audit_events").Scan(&n); err != nil {
			return err
		}
		note(fmt.Sprintf("pool closes with %d rows", n))
		return nil
	}})
	started := make(chan struct{})
	r.Add(componentFunc{name: "audit", start: func(context.Context) error {
		e.audit.Start(r)
		close(started)
		return nil
	}})
	done := make(chan int, 1)
	go func() { done <- r.Run(context.Background()) }()
	<-started
	if err := e.audit.Recorder().RecordDenial(person(), auditing.Entry{Action: "grant.create", Decision: auditing.DecisionDeny, Reason: auditing.ReasonNoGrant}); err != nil {
		t.Fatal(err)
	}
	sigs <- os.Interrupt
	if code := <-done; code != 0 {
		t.Fatalf("Run = %d", code)
	}
	if len(order) != 1 || order[0] != "pool closes with 1 rows" {
		t.Fatalf("order = %v; the denial must be flushed before the pool step", order)
	}
}

// componentFunc adapts a function to lifecycle.Component.
type componentFunc struct {
	name  string
	start func(context.Context) error
}

func (c componentFunc) Name() string                    { return c.name }
func (c componentFunc) Start(ctx context.Context) error { return c.start(ctx) }

func TestNewTxnRegistry_RequiresTheAuditHook(t *testing.T) {
	t.Parallel()
	m := wire.NewModules(nil, nil)
	m.Audit = nil
	if _, err := wire.NewTxnRegistry(m); err == nil {
		t.Fatal("a registry without the audit hook was built")
	}
}

// TestNewTxnRegistry_RequiresTheSyncHook: every unit of work must number its durable events, so
// a registry without the sync hook is refused (M1-16).
func TestNewTxnRegistry_RequiresTheSyncHook(t *testing.T) {
	t.Parallel()
	m := wire.NewModules(nil, nil)
	m.Sync = nil
	if _, err := wire.NewTxnRegistry(m); err == nil {
		t.Fatal("a registry without the sync hook was built")
	}
}

func BenchmarkAuditAppend(b *testing.B) {
	e := newAuditEnv(b, 4, time.Minute)
	ctx := person()
	for b.Loop() {
		err := e.audit.UoW().Do(ctx, func(ctx context.Context) error {
			return e.audit.Recorder().Record(ctx, auditing.Entry{Action: "grant.create", Decision: auditing.DecisionAllow})
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
