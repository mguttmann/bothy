// SPDX-License-Identifier: Apache-2.0

//go:build unix

package app_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/filesink"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/httpapi"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/keystore"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// pgKEK is the KEK source of the audit module in these tests.
type pgKEK struct{}

func (pgKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(strings.Repeat("6b", 32)), nil
}

// decider allows every check, or answers deny with a forbidden decision.
type decider struct{ deny bool }

func (d decider) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	if d.deny {
		return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "owner_only"}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (decider) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// metricsRec records the instrument calls.
type metricsRec struct {
	mu        sync.Mutex
	failures  int
	successes int
}

func (m *metricsRec) VerifyFailed(context.Context) { m.mu.Lock(); m.failures++; m.mu.Unlock() }
func (m *metricsRec) VerifySucceeded(context.Context, time.Time) {
	m.mu.Lock()
	m.successes++
	m.mu.Unlock()
}

// writer collects the batches of an export, failing from batch fail on (0 never).
type writer struct {
	batches [][]audit.Event
	fail    int
}

func (w *writer) WriteEvents(_ context.Context, events []audit.Event) error {
	w.batches = append(w.batches, events)
	if w.fail > 0 && len(w.batches) >= w.fail {
		return errors.New("client gone")
	}
	return nil
}

// pgEnv is the log over PostgreSQL with real keys and a file sink in a temporary directory.
type pgEnv struct {
	t       *testing.T
	super   *pg.Pool
	audit   *wire.Audit
	log     *app.Log
	metrics *metricsRec
	rep     *reporter
	sinkDir string
	inst    ids.ID
	owner   actor.Actor
	// superConn is the superuser of the throwaway container, the attacker of SEC-T16.
	superConn *pgx.Conn
}

// newPGEnv builds the environment; sink false leaves audit.checkpoint_sink empty, deny makes
// every authorization a denial.
func newPGEnv(t *testing.T, sink, deny bool) *pgEnv {
	t.Helper()
	return newPGEnvWrapped(t, sink, deny, nil)
}

// newPGEnvWrapped is newPGEnv whose file sink the job and the verification see through wrap.
func newPGEnvWrapped(t *testing.T, sink, deny bool, wrap func(app.CheckpointSink) app.CheckpointSink) *pgEnv {
	t.Helper()
	db := pgtest.New(t)
	pool := db.OpenPool(t, db.Config(pg.RoleApp))
	unit := pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}
	cfg := app.Config{
		DenyAggregateWindow: time.Millisecond, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
		CheckpointInterval: time.Hour, PageSizeDefault: 2, PageSizeMax: 3,
	}
	a, err := wire.NewAudit(pool, unit, cfg, pgKEK{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC))
	keyDir := filepath.Join(t.TempDir(), "signing-keys")
	if err := os.Mkdir(keyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	provider, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(keyDir), SigningKeyDir: keyDir, CacheTTL: time.Minute, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	ks, err := keystore.New(provider, keyDir)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := instanceapp.NewKeys(instanceapp.KeysConfig{
		CacheTTL: 30 * time.Second, InvocationRotation: 2 * time.Hour, AccessTokenRotation: 30 * 24 * time.Hour,
		JWKSMaxAge: 20 * time.Minute, AccessTokenTTL: 10 * time.Minute, InvocationTokenTTL: time.Minute,
		WebhookDeliveryTimeout: 15 * time.Second, ClockLeeway: 30 * time.Second,
	}, instanceapp.KeysDeps{
		Repo: instancepg.NewRepository(pool), Store: ks, UoW: a.UoW(), Authorizer: decider{}, Audit: a.Recorder(), Clock: clk,
		Log: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.EnsureActive(context.Background()); err != nil {
		t.Fatal(err)
	}
	e := &pgEnv{t: t, super: db.Pool(t, pg.RoleOwner), audit: a, metrics: &metricsRec{}, rep: &reporter{}, sinkDir: t.TempDir(), inst: ids.New(),
		owner: actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New()}, superConn: db.SuperConn(t)}
	var s app.CheckpointSink = app.NoSink{}
	if sink {
		if s, err = filesink.New(e.sinkDir, e.inst); err != nil {
			t.Fatal(err)
		}
		if wrap != nil {
			s = wrap(s)
		}
	}
	if _, err := app.NewLog(cfg, app.LogDeps{}); err == nil {
		t.Fatal("NewLog without collaborators passed")
	}
	deps := app.LogDeps{
		Store: auditpg.NewRepository(pool), UoW: a.UoW(), Authorizer: decider{deny: deny}, Audit: a.Recorder(), Keys: keys, Sink: s,
		Cursors: httpapi.Cursors{}, Metrics: e.metrics, Reporter: e.rep, Clock: clk, InstanceID: e.inst,
	}
	bad := cfg
	bad.PageSizeMax = 1
	if _, err := app.NewLog(bad, deps); err == nil {
		t.Fatal("NewLog with page_size_max below the default passed")
	}
	if e.log, err = app.NewLog(cfg, deps); err != nil {
		t.Fatal(err)
	}
	return e
}

// ctx is the context of the owner.
func (e *pgEnv) ctx() context.Context { return actor.WithActor(context.Background(), e.owner) }

// write records n allow entries of grant.create.
func (e *pgEnv) write(n int) {
	e.t.Helper()
	for range n {
		if err := e.audit.Recorder().RecordDetached(context.Background(), auditing.Entry{Action: "grant.create", Decision: auditing.DecisionAllow}); err != nil {
			e.t.Fatal(err)
		}
	}
}

// tamper runs sql with the protection trigger of table off, as the superuser would.
func (e *pgEnv) tamper(table, sql string, args ...any) {
	e.t.Helper()
	ctx := context.Background()
	conn := e.superConn
	if _, err := conn.Exec(ctx, "ALTER TABLE "+table+" DISABLE TRIGGER "+table+"_no_update_delete"); err != nil {
		e.t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, sql, args...); err != nil {
		e.t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, "ALTER TABLE "+table+" ENABLE TRIGGER "+table+"_no_update_delete"); err != nil {
		e.t.Fatal(err)
	}
}

func TestLog_DeniedCallsDoNothing(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, false, true)
	if _, err := e.log.List(e.ctx(), audit.Query{}); errs.KindOf(err) != errs.KindForbidden {
		t.Errorf("List: %v", err)
	}
	if err := e.log.Export(e.ctx(), time.Now(), time.Now(), &writer{}); errs.KindOf(err) != errs.KindForbidden {
		t.Errorf("Export: %v", err)
	}
	if _, err := e.log.Verify(e.ctx(), audit.VerifyRequest{}); errs.KindOf(err) != errs.KindForbidden {
		t.Errorf("Verify: %v", err)
	}
	if _, err := e.log.List(context.Background(), audit.Query{}); err == nil {
		t.Error("List without a caller passed")
	}
}

func TestLog_ListPagesAndValidates(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, false, false)
	e.write(5)
	var seen []int64
	cursor := ""
	for {
		p, err := e.log.List(e.ctx(), audit.Query{Action: "grant.create", Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range p.Items {
			seen = append(seen, it.Seq)
		}
		if p.NextCursor == "" {
			break
		}
		cursor = p.NextCursor
	}
	if len(seen) != 5 {
		t.Fatalf("rows %v", seen)
	}
	for _, q := range []audit.Query{{Action: "Not A Code"}, {ResourceKind: "@"}, {Decision: "maybe"}} {
		if _, err := e.log.List(e.ctx(), q); !hasCode(err, errs.CodeValidationFailed) {
			t.Errorf("%+v: %v", q, err)
		}
	}
	if _, err := e.log.List(e.ctx(), audit.Query{Cursor: "x"}); !hasCode(err, errs.CodeCursorInvalid) {
		t.Errorf("bad cursor: %v", err)
	}
	if p, err := e.log.List(e.ctx(), audit.Query{Limit: 99, ActorID: ids.New(), ResourceID: ids.New(),
		ResourceKind: "grant", From: time.Now().Add(-time.Hour), To: time.Now()}); err != nil || len(p.Items) != 0 {
		t.Errorf("filters: %+v, %v", p, err)
	}
}

func TestLog_ExportBatchesAndCut(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, false, false)
	e.write(7)
	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	if err := e.log.Export(e.ctx(), time.Time{}, to, &writer{}); !hasCode(err, errs.CodeValidationFailed) {
		t.Errorf("missing from: %v", err)
	}
	w := &writer{}
	if err := e.log.Export(e.ctx(), from, to, w); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, b := range w.batches {
		if len(b) > 3 {
			t.Errorf("batch of %d rows above api.page_size_max", len(b))
		}
		n += len(b)
	}
	if n < 7 || len(w.batches) < 3 {
		t.Errorf("%d rows in %d batches", n, len(w.batches))
	}
	if err := e.log.Export(e.ctx(), from, to, &writer{fail: 2}); !errors.Is(err, app.ErrExportCut) || !e.rep.has("export_cut") {
		t.Errorf("cut export: %v", err)
	}
	if err := e.log.Export(e.ctx(), to, to.Add(time.Hour), &writer{}); err != nil {
		t.Errorf("empty range: %v", err)
	}
}

func TestLog_VerifyAndJob(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, true, false)
	if res, err := e.log.Verify(e.ctx(), audit.VerifyRequest{}); err != nil || !res.OK {
		t.Fatalf("log of the key transitions: %+v, %v", res, err)
	}
	e.write(5)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.write(2)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.metrics.successes != 2 || e.metrics.failures != 0 {
		t.Errorf("metrics %+v", e.metrics)
	}
	res, err := e.log.Verify(e.ctx(), audit.VerifyRequest{})
	if err != nil || !res.OK || res.CheckpointsCompared != 2 || res.LastCheckpointSeq == 0 {
		t.Fatalf("verify: %+v, %v", res, err)
	}
	one, three := int64(1), int64(3)
	if _, err := e.log.Verify(e.ctx(), audit.VerifyRequest{FromSeq: &three, ToSeq: &one}); !hasCode(err, errs.CodeValidationFailed) {
		t.Errorf("from above to: %v", err)
	}
	if res, err := e.log.Verify(e.ctx(), audit.VerifyRequest{FromSeq: &three}); err != nil || !res.OK || res.CheckedFromSeq != 3 {
		t.Errorf("from 3: %+v, %v", res, err)
	}
	e.tamper("audit_events", `UPDATE audit_events SET details = '{"x":true}' WHERE seq = 2`)
	if res, _ := e.log.Verify(e.ctx(), audit.VerifyRequest{}); res.OK || res.FirstBrokenSeq != 2 || res.FailureReason != string(domain.HashMismatch) {
		t.Errorf("changed row: %+v", res)
	}
	e.write(1)
	e.tamper("audit_events", `UPDATE audit_events SET details = '{"x":true}' WHERE seq = (SELECT max(seq) FROM audit_events)`)
	if err := e.log.RunCheckpoint(context.Background()); err != nil || e.metrics.failures != 1 || !e.rep.has("verify_failed hash_mismatch") {
		t.Errorf("job over a break: %v, %+v", err, e.metrics)
	}
	// An unreadable sink is 503 for the verification and an error for the job.
	if err := os.Rename(e.sinkDir, e.sinkDir+".away"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.log.Verify(e.ctx(), audit.VerifyRequest{}); !hasCode(err, errs.CodeUnavailable) {
		t.Errorf("unreadable sink: %v", err)
	}
	if err := e.log.RunCheckpoint(context.Background()); err == nil {
		t.Error("the job passed with an unreadable sink")
	}
}

func TestLog_EmptySinkWarnsAndJobSignsIntoTheDatabase(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, false, false)
	e.write(2)
	if err := e.log.RunCheckpoint(context.Background()); err != nil || !e.rep.has("sink_not_configured") {
		t.Fatalf("job: %v", err)
	}
	res, err := e.log.Verify(e.ctx(), audit.VerifyRequest{})
	if err != nil || !res.OK || len(res.Warnings) != 1 || res.Warnings[0] != audit.WarningSinkNotConfigured {
		t.Fatalf("verify: %+v, %v", res, err)
	}
	if err := (app.NoSink{}).Put(context.Background(), app.SignedCheckpoint{}); err == nil {
		t.Error("NoSink.Put passed")
	}
	if _, err := (app.NoSink{}).List(context.Background()); err == nil {
		t.Error("NoSink.List passed")
	}
}

// pausingSink lets the first List after armed was set wait until release is closed, after it
// closed entered; every other List, also of another run, passes at once.
type pausingSink struct {
	app.CheckpointSink
	armed   atomic.Bool
	entered chan struct{}
	release chan struct{}
}

func (s *pausingSink) List(ctx context.Context) ([]app.SignedCheckpoint, error) {
	if s.armed.CompareAndSwap(true, false) {
		close(s.entered)
		<-s.release
	}
	return s.CheckpointSink.List(ctx)
}

// TestLog_OverlappingRunsAreNoBreak: a run that pauses while another run signs and exports a
// checkpoint is no break (red team round 2): the sink is read before the last checkpoint of the
// database, so every checkpoint of the sink is already in the database the run reads.
func TestLog_OverlappingRunsAreNoBreak(t *testing.T) {
	t.Parallel()
	ps := &pausingSink{entered: make(chan struct{}), release: make(chan struct{})}
	e := newPGEnvWrapped(t, true, false, func(s app.CheckpointSink) app.CheckpointSink {
		ps.CheckpointSink = s
		return ps
	})
	e.write(3)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.write(2)
	ps.armed.Store(true)
	done := make(chan error, 1)
	go func() { done <- e.log.RunCheckpoint(context.Background()) }()
	<-ps.entered
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	close(ps.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if e.metrics.failures != 0 || e.rep.has("verify_failed") {
		t.Errorf("overlapping runs reported a break: %+v", e.metrics)
	}
	if res, err := e.log.Verify(e.ctx(), audit.VerifyRequest{}); err != nil || !res.OK {
		t.Errorf("verify: %+v, %v", res, err)
	}
}

// gate holds the first call that passes it while armed until release is closed, after it closed
// entered.
type gate struct {
	armed   atomic.Bool
	entered chan struct{}
	release chan struct{}
}

func newGate() *gate { return &gate{entered: make(chan struct{}), release: make(chan struct{})} }

func (g *gate) pass(match bool) {
	if match && g.armed.CompareAndSwap(true, false) {
		close(g.entered)
		<-g.release
	}
}

// putSink fails every Put while down is set and holds Puts at the gates: a at the first Put, b
// at the first Put of a seq above bAbove.
type putSink struct {
	app.CheckpointSink
	down   atomic.Bool
	a, b   *gate
	bAbove atomic.Int64
}

func (s *putSink) Put(ctx context.Context, c app.SignedCheckpoint) error {
	if s.down.Load() {
		return errors.New("sink down")
	}
	s.a.pass(true)
	s.b.pass(c.Seq > s.bAbove.Load())
	return s.CheckpointSink.Put(ctx, c)
}

// TestLog_OverlappingExportIsNoBreak: run A exports a backlog of more than one batch while run B
// signs a newer checkpoint; A meets it in a later batch and must not take it for a checkpoint
// above the head (red team round 3): the head is read after every batch of unexported
// checkpoints, and a checkpoint is inserted only after its row committed.
func TestLog_OverlappingExportIsNoBreak(t *testing.T) {
	t.Parallel()
	ps := &putSink{a: newGate(), b: newGate()}
	e := newPGEnvWrapped(t, true, false, func(s app.CheckpointSink) app.CheckpointSink {
		ps.CheckpointSink = s
		return ps
	})
	// A backlog of four unexported checkpoints, more than one batch of api.page_size_max.
	ps.down.Store(true)
	for range 4 {
		e.write(1)
		if err := e.log.RunCheckpoint(context.Background()); err == nil {
			t.Fatal("the job passed with a sink that is down")
		}
	}
	ps.down.Store(false)
	var backlog int64
	if err := e.superConn.QueryRow(context.Background(), "SELECT max(seq) FROM audit_checkpoints").Scan(&backlog); err != nil {
		t.Fatal(err)
	}
	ps.bAbove.Store(backlog)
	ps.a.armed.Store(true)
	doneA := make(chan error, 1)
	go func() { doneA <- e.log.RunCheckpoint(context.Background()) }()
	<-ps.a.entered
	e.write(2)
	ps.b.armed.Store(true)
	doneB := make(chan error, 1)
	go func() { doneB <- e.log.RunCheckpoint(context.Background()) }()
	<-ps.b.entered
	close(ps.a.release)
	if err := <-doneA; err != nil {
		t.Fatal(err)
	}
	close(ps.b.release)
	if err := <-doneB; err != nil {
		t.Fatal(err)
	}
	if e.metrics.failures != 0 || e.rep.has("verify_failed") {
		t.Errorf("overlapping exports reported a break: %+v", e.metrics)
	}
	if res, err := e.log.Verify(e.ctx(), audit.VerifyRequest{}); err != nil || !res.OK || res.CheckpointsCompared != 5 {
		t.Errorf("verify: %+v, %v", res, err)
	}
}

// TestLog_RowBelowOneIsABreak: a row with a seq below 1 lies outside the chain that starts at
// row 1; the verification and the job report it as seq_gap at that seq instead of claiming to
// have checked it (red team round 2).
func TestLog_RowBelowOneIsABreak(t *testing.T) {
	t.Parallel()
	e := newPGEnv(t, true, false)
	e.write(3)
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.superConn.Exec(context.Background(), `INSERT INTO audit_events (seq, id, action, decision, prev_hash, hash)
		VALUES (0, gen_random_uuid(), 'grant.create', 'allow', sha256('a'::bytea), sha256(random()::text::bytea))`); err != nil {
		t.Fatal(err)
	}
	res, err := e.log.Verify(e.ctx(), audit.VerifyRequest{})
	if err != nil || res.OK || res.FirstBrokenSeq != 0 || res.FailureReason != string(domain.SeqGap) || res.CheckedFromSeq != 0 {
		t.Errorf("verify: %+v, %v", res, err)
	}
	e.write(1)
	if err := e.log.RunCheckpoint(context.Background()); err != nil || e.metrics.failures != 1 || !e.rep.has("verify_failed seq_gap") {
		t.Errorf("job: %v, %+v", err, e.metrics)
	}
}

// hasCode reports whether err is a kernel/errs error with code c.
func hasCode(err error, c errs.Code) bool {
	e, ok := errs.As(err)
	return ok && e.Code == c
}
