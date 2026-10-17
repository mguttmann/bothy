// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/jobstest"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// TestMain runs the tests of the package with a PostgreSQL container for the serve tests (plan
// P-09: without a Docker-compatible runtime the binary fails, it never skips). The templates run
// the steps of bothy migrate up, so every database has its instance row.
// TestMain builds the request validator first, so no readiness wait of a test pays for it
// (docs/DECISIONS.md M1-31/18).
func TestMain(m *testing.M) {
	if err := wire.WarmEnabledValidator(); err != nil {
		panic(err)
	}
	os.Exit(pgtest.Main(m, pgtest.WithSteps(wire.MigrationSteps)))
}

// newServeDB returns a database as bothy migrate up leaves it: the goose schema of pgtest and
// River's tables in the default jobs.schema.
func newServeDB(t *testing.T) *pgtest.DB {
	t.Helper()
	db := pgtest.New(t)
	jobstest.Migrate(t, db, "jobs")
	// Every process against one database shares one key store, as every role of a deployment
	// mounts the same volume (M1-32): a process with another store would find the keys of the
	// others without their files.
	keyStores.Store(db.Name(), keyStoreDir(t))
	return db
}

// completeSetup sets instance.setup_completed_at as the schema owner, the state after bothy setup
// bootstrap-owner, for tests of operations behind the setup gate (M1-37).
func completeSetup(t *testing.T, db *pgtest.DB) {
	t.Helper()
	if _, err := db.Pool(t, pg.RoleOwner).Exec(context.Background(), `UPDATE instance SET setup_completed_at = now() WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
}

// keyStores maps a database name of newServeDB to the key store its processes share.
var keyStores sync.Map

// logBuffer collects the JSON log lines of a serve process.
type logBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// port returns the server.port of the first line with msg, or 0.
func (l *logBuffer) port(msg string) int {
	sc := bufio.NewScanner(strings.NewReader(l.String()))
	for sc.Scan() {
		var line map[string]any
		if json.Unmarshal(sc.Bytes(), &line) != nil || line["msg"] != msg {
			continue
		}
		if p, ok := line["server.port"].(float64); ok {
			return int(p)
		}
	}
	return 0
}

// secretsDir writes the app password of db, a telemetry key and the audit KEK into a fresh directory with
// mode 0600 files, as the Compose stack mounts them.
func secretsDir(t *testing.T, db *pgtest.DB) string {
	t.Helper()
	dir := t.TempDir()
	pw, err := os.ReadFile(filepath.Join(db.SecretsDir(), "db_app_password"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"db_app_password": pw, "telemetry_pseudonym_key": bytes.Repeat([]byte("k"), 48),
		"audit_ip_kek": []byte(strings.Repeat("6b", 32) + "\n"), "token_pepper": []byte(strings.Repeat("7c", 32) + "\n"),
	}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// sharedKeyStore returns the key store of db from newServeDB, or a fresh one for a database of
// another origin.
func sharedKeyStore(t *testing.T, db *pgtest.DB) string {
	t.Helper()
	if dir, ok := keyStores.Load(db.Name()); ok {
		return dir.(string)
	}
	return keyStoreDir(t)
}

// keyStoreDir returns a fresh key store (secrets.signing_key_dir) with mode 0700.
func keyStoreDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "signing-keys")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// setMode sets the permission bits of path; tests build key stores with a wrong mode with it.
func setMode(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// serveEnv is the environment of a serve process against db; extra entries replace defaults.
func serveEnv(t *testing.T, db *pgtest.DB, extra ...string) []string {
	t.Helper()
	cfg := db.Config(pg.RoleApp)
	env := []string{
		"BOTHY_SECRETS__DIR=" + secretsDir(t, db),
		"BOTHY_SECRETS__SIGNING_KEY_DIR=" + sharedKeyStore(t, db),
		"BOTHY_DATABASE__HOST=" + cfg.Host,
		"BOTHY_DATABASE__PORT=" + strconv.Itoa(cfg.Port),
		"BOTHY_DATABASE__NAME=" + db.Name(),
		"BOTHY_DATABASE__USER=bothy_app",
		"BOTHY_DATABASE__SSLMODE=disable",
		"BOTHY_SERVER__LISTEN_ADDR=127.0.0.1:0",
		"BOTHY_SERVER__INTERNAL_LISTEN_ADDR=127.0.0.1:0",
		"BOTHY_STARTUP__DEPENDENCY_WAIT=10s",
		"BOTHY_TELEMETRY__ENABLED=false",
		"BOTHY_TELEMETRY__LOGS__EXPORTER=stdout",
		"BOTHY_SHUTDOWN__DRAIN_DELAY=0s",
	}
	for _, e := range extra {
		name, _, _ := strings.Cut(e, "=")
		env = slices.DeleteFunc(env, func(have string) bool { return strings.HasPrefix(have, name+"=") })
		env = append(env, e)
	}
	return env
}

func loadEnv(t *testing.T, env []string) *config.Values {
	t.Helper()
	if !slices.ContainsFunc(env, func(e string) bool { return strings.HasPrefix(e, "BOTHY_SERVER__PUBLIC_URL=") }) {
		env = append(testEnv(), env...)
	}
	v, err := config.Load(schema(t), config.Sources{Env: env})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return v
}

// process is one in-process serve run.
type process struct {
	runner *lifecycle.Runner
	logs   *logBuffer
	sigs   chan os.Signal
	done   chan int
}

func start(t *testing.T, role wire.Role, env []string, deps wire.Deps) *process {
	t.Helper()
	p := &process{logs: &logBuffer{}, sigs: make(chan os.Signal, 2), done: make(chan int, 1)}
	deps.Env, deps.Stdout, deps.Signals = env, p.logs, p.sigs
	r, err := wire.Serve(context.Background(), loadEnv(t, env), role, deps)
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}
	p.runner = r
	go func() { p.done <- r.Run(context.Background()) }()
	t.Cleanup(func() {
		select {
		case <-p.done:
		default:
			p.sigs <- syscall.SIGTERM
			p.sigs <- syscall.SIGTERM
			<-p.done
		}
	})
	return p
}

func (p *process) exit(t *testing.T) int {
	t.Helper()
	select {
	case code := <-p.done:
		p.done <- code
		return code
	case <-time.After(60 * time.Second):
		t.Fatalf("serve did not exit; logs:\n%s", p.logs)
		return -1
	}
}

// waitInternal waits for the line of the internal listener and returns its port.
func (p *process) waitInternal(t *testing.T) int {
	t.Helper()
	const msg = "internal listener started"
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if port := p.logs.port(msg); port != 0 {
			return port
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no %q line; logs:\n%s", msg, p.logs)
	return 0
}

func get(t *testing.T, port int, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// waitReadyz polls /readyz until it answers code.
func waitReadyz(t *testing.T, port int, code int) string {
	t.Helper()
	const path = "/readyz"
	deadline := time.Now().Add(30 * time.Second)
	var got int
	var body string
	for time.Now().Before(deadline) {
		if got, body = get(t, port, path); got == code {
			return body
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s = %d %q, want %d", path, got, body, code)
	return ""
}

func TestServeRolesStartReadyAndStop(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	// Past the setup gate, so an unmatched path reaches the mux (before setup it is 503
	// setup_required, TestServe_IDT06_SetupGate).
	completeSetup(t, db)
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker, wire.RoleAll} {
		t.Run(string(role), func(t *testing.T) {
			t.Parallel()
			p := start(t, role, serveEnv(t, db), wire.Deps{})
			internal := p.waitInternal(t)
			waitReadyz(t, internal, http.StatusOK)
			if !strings.Contains(p.logs.String(), `"msg":"server ready"`) {
				t.Fatalf("no ready line:\n%s", p.logs)
			}
			// EnsureActive of the start (M1-32): exactly one active key per purpose, also with the
			// three roles starting against one database at once.
			var purposes, actives int
			if err := db.Pool(t, pg.RoleOwner).QueryRow(context.Background(),
				`SELECT count(DISTINCT purpose), count(*) FROM server_signing_keys WHERE state = 'active'`).Scan(&purposes, &actives); err != nil {
				t.Fatal(err)
			}
			if purposes != 4 || actives != 4 {
				t.Fatalf("%d active keys over %d purposes after the start, want 4 and 4", actives, purposes)
			}
			public := p.logs.port("public listener started")
			if role == wire.RoleWorker {
				if public != 0 {
					t.Fatal("worker opened a public listener")
				}
			} else {
				for _, path := range []string{"/healthz", "/readyz"} {
					// Since M1-18 the public listener answers an unmatched path with a 404 problem
					// that never echoes the path (it replaces the empty 404 of M1-11).
					if code, body := get(t, public, path); code != http.StatusNotFound || !strings.Contains(body, `"code":"not_found"`) || strings.Contains(body, path) {
						t.Fatalf("public %s = %d %q, want a 404 problem", path, code, body)
					}
				}
			}
			if code, _ := get(t, internal, "/healthz"); code != http.StatusOK {
				t.Fatalf("/healthz = %d", code)
			}
			p.sigs <- syscall.SIGTERM
			if code := p.exit(t); code != lifecycle.ExitOK {
				t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
			}
			if code, _ := get(t, internal, "/healthz"); code != 0 {
				t.Fatal("internal listener still open after the shutdown")
			}
		})
	}
}

func TestServeReadyzDuringDrain(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	p := start(t, wire.RoleAPI, serveEnv(t, db, "BOTHY_SHUTDOWN__DRAIN_DELAY=2s"), wire.Deps{})
	internal := p.waitInternal(t)
	waitReadyz(t, internal, http.StatusOK)
	p.sigs <- syscall.SIGTERM
	body := waitReadyz(t, internal, http.StatusServiceUnavailable)
	if body != `{"status":"unavailable","failed":["shutting_down"]}`+"\n" {
		t.Fatalf("body = %q", body)
	}
	if code, _ := get(t, internal, "/healthz"); code != http.StatusOK {
		t.Fatalf("/healthz during drain = %d", code)
	}
	if code := p.exit(t); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d", code)
	}
}

func TestServeAbortsOnOldSchema(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t, pgtest.WithoutMigrations())
	// The binary of this test needs version 2; the database has none.
	fsys := fstest.MapFS{
		"00001_a.sql": {Data: []byte("-- +goose Up\nSELECT 1;\n")},
		"00002_b.sql": {Data: []byte("-- +goose Up\nSELECT 1;\n")},
	}
	p := start(t, wire.RoleAll, serveEnv(t, db), wire.Deps{Migrations: fsys})
	if code := p.exit(t); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d", code)
	}
	if err := p.runner.Err(); err == nil || !strings.Contains(err.Error(), "bothy migrate up") {
		t.Fatalf("Err() = %v", err)
	}
	internal := p.logs.port("internal listener started")
	if internal == 0 {
		t.Fatal("internal listener never opened")
	}
	if code, _ := get(t, internal, "/healthz"); code != 0 {
		t.Fatal("internal listener left open")
	}
	if p.logs.port("public listener started") != 0 {
		t.Fatal("public listener opened")
	}
}

// TestServeRefusesCheckpointSinkWithoutLock: the start step audit_log runs the lock check of an
// s3:// audit.checkpoint_sink, so a bucket without Object Lock stops the start with an error
// naming audit.checkpoint_sink and never the endpoint (M7-22, runbook backup-restore 8).
func TestServeRefusesCheckpointSinkWithoutLock(t *testing.T) {
	t.Parallel()
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bothy-audit" && r.URL.Query().Has("object-lock") {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>ObjectLockConfigurationNotFoundError</Code><Message>none</Message></Error>`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(fake.Close)
	db := newServeDB(t)
	dir := secretsDir(t, db)
	for _, name := range []string{"audit_checkpoint_s3_access_key", "audit_checkpoint_s3_secret_key"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("EXAMPLEKEY"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	host := strings.TrimPrefix(fake.URL, "http://")
	p := start(t, wire.RoleAll, serveEnv(t, db, "BOTHY_SECRETS__DIR="+dir, "BOTHY_AUDIT__CHECKPOINT_SINK=s3://bothy-audit",
		"BOTHY_AUDIT__CHECKPOINT_S3_ENDPOINT="+fake.URL, "BOTHY_EGRESS__ALLOWED_HOSTS="+host+";allow_private"), wire.Deps{})
	if code := p.exit(t); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d", code)
	}
	err := p.runner.Err()
	if err == nil || !strings.Contains(err.Error(), "audit.checkpoint_sink") || strings.Contains(err.Error(), host) {
		t.Fatalf("Err() = %v, want an error naming audit.checkpoint_sink without the endpoint", err)
	}
	if p.logs.port("public listener started") != 0 {
		t.Fatal("public listener opened")
	}
}

func TestServeDatabaseUnreachableExits1AfterWait(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	start := time.Now()
	p := startProc(t, serveEnv(t, db, "BOTHY_DATABASE__PORT=1", "BOTHY_STARTUP__DEPENDENCY_WAIT=500ms"))
	if code := p.exit(t); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d", code)
	}
	if time.Since(start) > 20*time.Second {
		t.Fatal("the start waited far beyond startup.dependency_wait")
	}
	if err := p.runner.Err(); err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("Err() = %v", err)
	}
}

func startProc(t *testing.T, env []string) *process {
	t.Helper()
	return start(t, wire.RoleWorker, env, wire.Deps{})
}

func TestServeMissingSecretFailsBeforeTheWait(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	p := start(t, wire.RoleWorker, serveEnv(t, db, "BOTHY_DATABASE__PASSWORD_REF=file:<secrets.dir>/missing"), wire.Deps{})
	if code := p.exit(t); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d", code)
	}
	if err := p.runner.Err(); err == nil || !strings.Contains(err.Error(), "start secrets") {
		t.Fatalf("Err() = %v", err)
	}
	if p.logs.port("internal listener started") != 0 {
		t.Fatal("the internal listener opened before the secrets were resolved")
	}
}

func TestServeBudgetsAboveTotalExit2(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	p := start(t, wire.RoleAPI, serveEnv(t, db, "BOTHY_SHUTDOWN__DRAIN_DELAY=30s", "BOTHY_SHUTDOWN__HTTP_GRACE=30s", "BOTHY_SHUTDOWN__TOTAL_TIMEOUT=60s"), wire.Deps{})
	if code := p.exit(t); code != lifecycle.ExitUsage {
		t.Fatalf("exit = %d", code)
	}
	if err := p.runner.Err(); err == nil || !strings.Contains(err.Error(), "1m5s") || !strings.Contains(err.Error(), "1m0s") {
		t.Fatalf("Err() = %v", err)
	}
}

// TestServeWorkerBudgetIncludesJobsSoftTimeout: the worker roles register the River stop step
// with shutdown.jobs_soft_timeout, so it counts against shutdown.total_timeout (02 "Shutdown").
func TestServeWorkerBudgetIncludesJobsSoftTimeout(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	env := serveEnv(t, db, "BOTHY_SHUTDOWN__JOBS_SOFT_TIMEOUT=30s", "BOTHY_SHUTDOWN__TOTAL_TIMEOUT=30s")
	p := start(t, wire.RoleWorker, env, wire.Deps{})
	if code := p.exit(t); code != lifecycle.ExitUsage {
		t.Fatalf("exit = %d", code)
	}
	if err := p.runner.Err(); err == nil || !strings.Contains(err.Error(), "35s") {
		t.Fatalf("Err() = %v, want the sum 35s of jobs and telemetry", err)
	}
	// The api role has no River step: the same budgets pass.
	api := start(t, wire.RoleAPI, env, wire.Deps{})
	internal := api.waitInternal(t)
	waitReadyz(t, internal, http.StatusOK)
	api.sigs <- syscall.SIGTERM
	if code := api.exit(t); code != lifecycle.ExitOK {
		t.Fatalf("api exit = %d", code)
	}
}

// TestServeAbortsWithoutRiverTables: a database without River's migrations ends the start of
// every role with the hint to run bothy migrate up; serve never migrates (R-AR-16).
func TestServeAbortsWithoutRiverTables(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	for _, role := range []wire.Role{wire.RoleAPI, wire.RoleWorker} {
		p := start(t, role, serveEnv(t, db), wire.Deps{})
		if code := p.exit(t); code != lifecycle.ExitFailure {
			t.Fatalf("%s: exit = %d", role, code)
		}
		if err := p.runner.Err(); err == nil || !strings.Contains(err.Error(), "bothy migrate up") || !strings.Contains(err.Error(), "River") {
			t.Fatalf("%s: Err() = %v", role, err)
		}
	}
}

func TestServeRejectsUnknownRole(t *testing.T) {
	t.Parallel()
	v, err := config.Load(schema(t), config.Sources{Env: testEnv()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wire.Serve(context.Background(), v, "both", wire.Deps{}); err == nil {
		t.Fatal("unknown role accepted")
	}
}

func TestReadyzReportsDatabaseLoss(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	cfg := db.Config(pg.RoleApp)
	cfg.AcquireTimeout = 2 * time.Second
	pool := db.OpenPool(t, cfg)
	reg := health.NewRegistry()
	reg.AddReadiness("database", pool.Ping)
	reg.SetStarted()
	h := reg.Handler()
	ask := func() (int, string) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", nil)
		rec := newRecorder()
		h.ServeHTTP(rec, req)
		return rec.code, rec.body.String()
	}
	if code, body := ask(); code != http.StatusOK {
		t.Fatalf("before = %d %q", code, body)
	}
	super := db.SuperConfig()
	super.Name = "postgres"
	sp, err := secrets.NewProvider(secrets.Options{Dir: db.SecretsDir()})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pg.Connect(context.Background(), super, sp, 10*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	if _, err := conn.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{db.Name()}.Sanitize()+" WITH (FORCE)"); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	code, body := ask()
	if code != http.StatusServiceUnavailable || body != `{"status":"unavailable","failed":["database"]}`+"\n" {
		t.Fatalf("after = %d %q", code, body)
	}
	if took := time.Since(began); took > cfg.AcquireTimeout+5*time.Second {
		t.Fatalf("the check took %v", took)
	}
}

// recorder is a minimal http.ResponseWriter.
type recorder struct {
	code int
	hdr  http.Header
	body bytes.Buffer
}

func newRecorder() *recorder { return &recorder{hdr: http.Header{}} }

func (r *recorder) Header() http.Header         { return r.hdr }
func (r *recorder) Write(b []byte) (int, error) { return r.body.Write(b) }
func (r *recorder) WriteHeader(code int)        { r.code = code }

func TestServeSignalDuringStartSkipsTheDrain(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	// The database never answers; the drain delay would hold the end for 30s although the public
	// listener never opened.
	p := start(t, wire.RoleAPI, serveEnv(t, db, "BOTHY_DATABASE__PORT=1", "BOTHY_STARTUP__DEPENDENCY_WAIT=20s", "BOTHY_SHUTDOWN__DRAIN_DELAY=30s"), wire.Deps{})
	p.waitInternal(t)
	sent := time.Now()
	p.sigs <- syscall.SIGTERM
	if code := p.exit(t); code != lifecycle.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if took := time.Since(sent); took > 10*time.Second {
		t.Fatalf("the end took %v; the drain ran without a public listener", took)
	}
}

// TestServeAbortsOnBadRegistration (02 15.1 step 7, 07 AZ-P2): a module that registers a
// server-to-client op ends `bothy serve` with exit code 1 and a line naming the violation; the
// public listener never opens.
func TestServeAbortsOnBadRegistration(t *testing.T) {
	t.Parallel()
	db := pgtest.New(t)
	bad := []func(*authorize.Registry){func(r *authorize.Registry) { r.Op("gateway", authorize.OpHello) }}
	p := start(t, wire.RoleAll, serveEnv(t, db), wire.Deps{Registrations: bad})
	if code := p.exit(t); code != lifecycle.ExitFailure {
		t.Fatalf("exit = %d; logs:\n%s", code, p.logs)
	}
	if !strings.Contains(p.logs.String(), `"msg":"registration check failed","error.type":"outbound_op"`) {
		t.Fatalf("no line naming the violation; logs:\n%s", p.logs)
	}
	if err := p.runner.Err(); err == nil || !strings.Contains(err.Error(), `op "hello" (module "gateway")`) {
		t.Fatalf("Err() = %v", err)
	}
	if p.logs.port("public listener started") != 0 {
		t.Fatal("public listener opened")
	}
}
