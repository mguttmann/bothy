// SPDX-License-Identifier: Apache-2.0

package pg_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// show returns the value of a run-time parameter on q.
func show(t *testing.T, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, name string) string {
	t.Helper()
	var v string
	if err := q.QueryRow(context.Background(), "SELECT current_setting($1)", name).Scan(&v); err != nil {
		t.Fatalf("current_setting(%s): %v", name, err)
	}
	return v
}

func TestPoolTimeouts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.StatementTimeout, cfg.LockTimeout, cfg.IdleInTxTimeout = 400*time.Millisecond, 700*time.Millisecond, 3*time.Second
	cfg.MaxConns = 1
	pool := db.OpenPool(t, cfg)

	session := map[string]string{
		"statement_timeout": "400ms", "lock_timeout": "700ms", "idle_in_transaction_session_timeout": "3s",
		"application_name": "bothy-test", "default_transaction_isolation": "read committed", "TimeZone": "UTC",
	}
	c, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range session {
		if got := show(t, c, name); got != want {
			t.Errorf("session %s = %q, want %q", name, got, want)
		}
	}
	// A statement longer than statement_timeout is canceled with query_canceled.
	_, err = c.Exec(ctx, "SELECT pg_sleep(2)")
	c.Release()
	assertSQLState(t, err, "57014")

	tx, err := pool.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := show(t, tx, "transaction_isolation"); got != "read committed" {
		t.Errorf("transaction_isolation = %q, want read committed", got)
	}
	if err := pg.SetLocalTimeouts(ctx, tx, pg.Timeouts{Statement: 250 * time.Millisecond, Lock: 100 * time.Millisecond, IdleInTx: 2 * time.Second}); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"statement_timeout": "250ms", "lock_timeout": "100ms", "idle_in_transaction_session_timeout": "2s"} {
		if got := show(t, tx, name); got != want {
			t.Errorf("in transaction %s = %q, want %q", name, got, want)
		}
	}
	_, err = tx.Exec(ctx, "SELECT pg_sleep(1)")
	assertSQLState(t, err, "57014")
	_ = tx.Rollback(ctx)

	// A zero field keeps the session value; after commit the session values are back.
	tx, err = pool.Begin(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.SetLocalTimeouts(ctx, tx, pg.Timeouts{Lock: 300 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if got, got2 := show(t, tx, "lock_timeout"), show(t, tx, "statement_timeout"); got != "300ms" || got2 != "400ms" {
		t.Errorf("lock_timeout %q, statement_timeout %q; want 300ms and the session value 400ms", got, got2)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	c, err = pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Release()
	for _, name := range []string{"statement_timeout", "lock_timeout", "idle_in_transaction_session_timeout"} {
		if got := show(t, c, name); got != session[name] {
			t.Errorf("after commit %s = %q, want the session value %q", name, got, session[name])
		}
	}
}

func assertSQLState(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Errorf("error %v, want SQLSTATE %s", err, code)
	}
}

func TestSetLocalTimeoutsRejectsInvalid(t *testing.T) {
	t.Parallel()
	for _, tm := range []pg.Timeouts{{Statement: -time.Millisecond}, {Lock: -1}, {IdleInTx: 25 * 24 * time.Hour}} {
		if err := pg.SetLocalTimeouts(context.Background(), nil, tm); err == nil {
			t.Errorf("SetLocalTimeouts(%+v) accepted", tm)
		}
	}
}

func TestBeginKeepsAnExplicitIsolation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := pgtest.New(t).Pool(t, pg.RoleApp)
	tx, err := pool.Begin(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if got := show(t, tx, "transaction_isolation"); got != "repeatable read" {
		t.Errorf("transaction_isolation = %q, want repeatable read", got)
	}
}

func TestPoolAcquireTimeout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns, cfg.AcquireTimeout = 1, 300*time.Millisecond
	pool := db.OpenPool(t, cfg)
	// The first Acquire builds the connection within the short deadline; on a busy machine that
	// can take longer, so it may try again before the test holds the only connection.
	var held *pgxpool.Conn
	var err error
	for range 20 {
		if held, err = pool.Acquire(ctx); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	for name, call := range map[string]func() error{
		"Acquire": func() error {
			c, err := pool.Acquire(ctx)
			if err == nil {
				c.Release()
			}
			return err
		},
		"Begin": func() error {
			tx, err := pool.Begin(ctx, pgx.TxOptions{})
			if err == nil {
				_ = tx.Rollback(ctx)
			}
			return err
		},
		"Ping": func() error { return pool.Ping(ctx) },
	} {
		start := time.Now()
		err := call()
		elapsed := time.Since(start)
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeUnavailable || e.Cause != errs.CauseDBPoolExhausted {
			t.Errorf("%s: error %v, want unavailable with cause %s", name, err, errs.CauseDBPoolExhausted)
			continue
		}
		if errs.KindOf(err).Status() != 503 {
			t.Errorf("%s: status %d, want 503", name, errs.KindOf(err).Status())
		}
		if strings.Contains(err.Error(), errs.CauseDBPoolExhausted) {
			t.Errorf("%s: the error text %q carries the internal cause", name, err)
		}
		if elapsed < 250*time.Millisecond || elapsed > 3*time.Second {
			t.Errorf("%s returned after %s, want about database.acquire_timeout (300ms)", name, elapsed)
		}
	}

	// A caller whose own deadline ends first gets deadline_exceeded, a canceled caller the
	// context error.
	short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if e, ok := errs.As(pool.Ping(short)); !ok || e.Cause != errs.CauseDeadlineExceeded {
		t.Errorf("ping with an expired request context: %v, want cause %s", e, errs.CauseDeadlineExceeded)
	}
	canceled, cancelNow := context.WithCancel(ctx)
	cancelNow()
	if err := pool.Ping(canceled); !errors.Is(err, context.Canceled) {
		t.Errorf("ping with a canceled context: %v, want context.Canceled", err)
	}
}

func TestPoolPasswordRotation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t)
	super := db.SuperConn(t)
	role := "rot_" + db.Name()
	ident := pgx.Identifier{role}.Sanitize()
	// A role of this test only: the four server roles are shared by all tests of the binary.
	for _, stmt := range []string{
		"CREATE ROLE " + ident + " LOGIN PASSWORD 'first-EXAMPLE-rotation'",
		"GRANT CONNECT ON DATABASE " + pgx.Identifier{db.Name()}.Sanitize() + " TO " + ident,
	} {
		if _, err := super.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = super.Exec(context.Background(), "DROP OWNED BY "+ident)
		_, _ = super.Exec(context.Background(), "DROP ROLE "+ident)
	})
	dir := t.TempDir()
	file := filepath.Join(dir, "password")
	writeSecret(t, file, "first-EXAMPLE-rotation")
	fake := clock.NewFake(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir, CacheTTL: time.Minute, Clock: fake})
	if err != nil {
		t.Fatal(err)
	}
	cfg := db.Config(pg.RoleApp)
	cfg.User, cfg.PasswordRef, cfg.MaxConns = role, "file:"+file, 1
	pool, err := pg.Open(ctx, cfg, sp, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("first password: %v", err)
	}

	if _, err := super.Exec(ctx, "ALTER ROLE "+ident+" PASSWORD 'second-EXAMPLE-rotation'"); err != nil {
		t.Fatal(err)
	}
	writeSecret(t, file, "second-EXAMPLE-rotation")
	dropConnection(t, pool)
	// Within the cache TTL a new connection still uses the cached, now wrong password.
	if err := pool.Ping(ctx); err == nil {
		t.Fatal("a new connection logged in with the old password after the rotation")
	}
	fake.Advance(time.Minute)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("after secrets.cache_ttl the rotated password does not work: %v", err)
	}
}

// writeSecret writes a secret file with the mode the file driver requires.
func writeSecret(t *testing.T, file, value string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

// dropConnection closes the one connection of a pool of size 1, so that the next use connects
// anew.
func dropConnection(t *testing.T, pool *pg.Pool) {
	t.Helper()
	c, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Conn().Close(context.Background())
	c.Release()
}

func TestWaitReadyGivesUp(t *testing.T) {
	t.Parallel()
	l, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	dir := t.TempDir()
	const canary = "canary-EXAMPLE-waitready"
	writeSecret(t, filepath.Join(dir, "password"), canary)
	sp, err := secrets.NewProvider(secrets.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	pool, err := pg.Open(context.Background(), pg.Config{
		Host: "127.0.0.1", Port: port, Name: "bothy", User: "bothy_app", PasswordRef: "file:" + filepath.Join(dir, "password"),
		SSLMode: pg.SSLModeDisable, MaxConns: 2, AcquireTimeout: 200 * time.Millisecond,
	}, sp, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	start := time.Now()
	err = pool.WaitReady(context.Background(), 700*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("WaitReady succeeded against a closed port")
	}
	if elapsed < 650*time.Millisecond || elapsed > 3*time.Second {
		t.Errorf("WaitReady gave up after %s, want about 700ms", elapsed)
	}
	if want := "127.0.0.1:" + strconv.Itoa(port); !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not name %s", err, want)
	}
	if strings.Contains(err.Error(), canary) || strings.Contains(logs.String(), canary) {
		t.Error("the password appears in the error or the log")
	}
	if !strings.Contains(logs.String(), "database not reachable yet, retrying") ||
		!strings.Contains(logs.String(), "error.type=connect") || strings.Contains(logs.String(), "refused") {
		t.Errorf("want retry lines with the error class only, got %q", logs.String())
	}
}

func TestOpenRejectsInvalidConfig(t *testing.T) {
	t.Parallel()
	sp, err := secrets.NewProvider(secrets.Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Open(context.Background(), pg.Config{SSLMode: "require"}, sp, nil); err == nil ||
		!strings.Contains(err.Error(), "database.sslmode") {
		t.Errorf("Open with sslmode require: %v, want an error naming database.sslmode", err)
	}
	if _, err := pg.Open(context.Background(), pgtestConfig(), nil, nil); err == nil {
		t.Error("Open without a secret provider succeeded")
	}
}

// pgtestConfig is a valid configuration that is never connected.
func pgtestConfig() pg.Config {
	return pg.Config{Host: "127.0.0.1", Port: 5432, Name: "bothy", User: "bothy_app", PasswordRef: "file:/run/secrets/x",
		SSLMode: pg.SSLModeDisable, MaxConns: 1, AcquireTimeout: time.Second}
}

// TestDBSpansHaveNoParameters is not parallel: it swaps the tracer provider of new pools.
func TestDBSpansHaveNoParameters(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	restore := pg.SetTracerProviderForTest(tp)
	defer restore()

	pool := pgtest.New(t).Pool(t, pg.RoleApp)
	const canary = "canary-EXAMPLE-span"
	ctx, parent := tp.Tracer("test").Start(context.Background(), "request")
	c, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var echoed string
	if err := c.QueryRow(ctx, "SELECT $1::text", canary).Scan(&echoed); err != nil || echoed != canary {
		t.Fatalf("query: %q, %v", echoed, err)
	}
	// PostgreSQL quotes the value in this error; the span must not.
	_, err = c.Exec(ctx, "SELECT $1::int", canary)
	if err == nil || !strings.Contains(err.Error(), canary) {
		t.Fatalf("the failing query returned %v, want an error quoting the value", err)
	}
	c.Release()
	parent.End()

	keys := map[string]bool{}
	var query, failed bool
	for _, s := range rec.Ended() {
		texts := []string{s.Name(), s.Status().Description}
		for _, a := range s.Attributes() {
			keys[string(a.Key)] = true
			texts = append(texts, a.Value.String())
			if a.Key == "db.query.text" && a.Value.AsString() == "SELECT $1::text" {
				query = true
			}
			if a.Key == "pgx.sql_state" && a.Value.AsString() == "22P02" {
				failed = true
			}
		}
		for _, e := range s.Events() {
			texts = append(texts, e.Name)
			for _, a := range e.Attributes {
				texts = append(texts, a.Value.String())
			}
		}
		for _, text := range texts {
			if strings.Contains(text, canary) {
				t.Errorf("span %q carries the parameter value: %q", s.Name(), text)
			}
		}
	}
	if !query {
		t.Error("no span with db.query.text = \"SELECT $1::text\"")
	}
	if !failed {
		t.Error("the failed query lost its SQLSTATE (pgx.sql_state 22P02)")
	}
	for _, k := range []string{"db.system.name", "db.namespace", "db.operation.name", "db.query.text"} {
		if !keys[k] {
			t.Errorf("no span carries %s", k)
		}
	}
	t.Logf("span attribute keys: %v", keys)
}

// TestWaitReadyKeepsTheCause: when WaitReady and Connect give up, the error carries the last
// real cause (here a wrong password and a missing database), not the deadline of the wait.
func TestWaitReadyKeepsTheCause(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	dir := t.TempDir()
	writeSecret(t, filepath.Join(dir, "pw"), "wrong-EXAMPLE-password")
	cfg := db.Config(pg.RoleApp)
	cfg.PasswordRef = "file:" + filepath.Join(dir, "pw")
	pool, err := pg.Open(ctx, cfg, providerFor(t, dir), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	err = pool.WaitReady(ctx, 1500*time.Millisecond)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "28P01" {
		t.Errorf("WaitReady with a wrong password: %v, want the cause 28P01", err)
	}

	missing := db.SuperConfig()
	missing.Name = "no_such_database"
	_, err = pg.Connect(ctx, missing, providerFor(t, db.SecretsDir()), 1500*time.Millisecond, slog.New(slog.DiscardHandler))
	if !errors.As(err, &pgErr) || pgErr.Code != "3D000" || !strings.Contains(err.Error(), "no_such_database") {
		t.Errorf("Connect to a missing database: %v, want the cause 3D000 naming the database", err)
	}
	if !strings.Contains(err.Error(), "the password of postgres") && strings.Contains(err.Error(), "database.password_ref") {
		t.Errorf("Connect names database.password_ref for another login: %v", err)
	}
}

// TestBootstrapSpansCarryNoVerifier is not parallel: it swaps the tracer provider. The role
// statements carry the SCRAM verifier as a literal; spans must not.
func TestBootstrapSpansCarryNoVerifier(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	restore := pg.SetTracerProviderForTest(tp)
	defer restore()

	db := pgtest.New(t, pgtest.WithoutMigrations())
	ctx, parent := tp.Tracer("test").Start(context.Background(), "migrate")
	conn, err := pg.Connect(ctx, db.SuperConfig(), providerFor(t, db.SecretsDir()), testkit.Patience, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	if err := pg.BootstrapRoles(ctx, conn, pgtestPasswords(t, db)); err != nil {
		t.Fatal(err)
	}
	parent.End()
	roleSpans := 0
	for _, s := range rec.Ended() {
		for _, a := range s.Attributes() {
			if strings.Contains(a.Value.String(), "SCRAM-SHA-256$") {
				t.Errorf("span %q attribute %s carries a verifier", s.Name(), a.Key)
			}
			if a.Key == "db.query.text" && strings.Contains(a.Value.String(), "ROLE") {
				roleSpans++
			}
		}
	}
	if roleSpans == 0 {
		t.Error("no span of a role statement was recorded; the test would prove nothing")
	}
}
