// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs/jobstest"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
)

// bothyBinary is the binary TestMain builds once for the tests that run bothy as a child
// process and signal it by PID.
var bothyBinary string

// lockedBuffer is a bytes.Buffer safe for a child's output and the test reading it.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// logPort returns the server.port of the first JSON line with msg in logs, or 0.
func logPort(logs, msg string) int {
	sc := bufio.NewScanner(strings.NewReader(logs))
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

// newServeDB returns a database as bothy migrate up leaves it: the goose schema of pgtest and
// River's tables in the default jobs.schema.
func newServeDB(t *testing.T) *pgtest.DB {
	t.Helper()
	db := pgtest.New(t)
	jobstest.Migrate(t, db, "jobs")
	return db
}

// serveTestEnv is the environment of bothy serve against db as bothy_app, with a secrets
// directory of its own (mode 0600 files), a key store of its own and both listeners on
// 127.0.0.1:0.
func serveTestEnv(t *testing.T, db *pgtest.DB, extra ...string) []string {
	t.Helper()
	dir := t.TempDir()
	pw, err := os.ReadFile(filepath.Join(db.SecretsDir(), "db_app_password"))
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]byte{
		"db_app_password": pw, "telemetry_pseudonym_key": bytes.Repeat([]byte("k"), 48),
		"audit_ip_kek": []byte(strings.Repeat("6b", 32)), "token_pepper": []byte(strings.Repeat("7c", 32)),
		"totp_key": []byte(strings.Repeat("5d", 32)), "webhook_hmac_master_key": []byte(strings.Repeat("4e", 32)),
		"file_ticket_key": []byte(strings.Repeat("3f", 32)),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The key store of the signing keys (M1-32): a directory of the test with mode 0700.
	keys := filepath.Join(t.TempDir(), "signing-keys")
	if err := os.Mkdir(keys, 0o700); err != nil {
		t.Fatal(err)
	}
	// The root of the file system driver (M3-03): a directory of the test with mode 0700.
	files := filepath.Join(t.TempDir(), "files")
	if err := os.Mkdir(files, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := db.Config(pg.RoleApp)
	env := []string{
		"BOTHY_SECRETS__DIR=" + dir,
		"BOTHY_SECRETS__SIGNING_KEY_DIR=" + keys,
		"BOTHY_FILE__FS__ROOT=" + files,
		"BOTHY_DATABASE__HOST=" + cfg.Host,
		"BOTHY_DATABASE__PORT=" + strconv.Itoa(cfg.Port),
		"BOTHY_DATABASE__NAME=" + db.Name(),
		"BOTHY_DATABASE__SSLMODE=disable",
		"BOTHY_SERVER__LISTEN_ADDR=127.0.0.1:0",
		"BOTHY_SERVER__INTERNAL_LISTEN_ADDR=127.0.0.1:0",
		"BOTHY_STARTUP__DEPENDENCY_WAIT=10s",
		"BOTHY_TELEMETRY__ENABLED=false",
		"BOTHY_TELEMETRY__LOGS__EXPORTER=stdout",
		"BOTHY_SHUTDOWN__DRAIN_DELAY=0s",
		testPublicURL,
	}
	for _, e := range extra {
		name, _, _ := strings.Cut(e, "=")
		env = slices.DeleteFunc(env, func(have string) bool { return strings.HasPrefix(have, name+"=") })
		env = append(env, e)
	}
	return env
}

// child is a bothy process started by a test; only its own PID is ever signaled.
type child struct {
	cmd            *exec.Cmd
	stdout, stderr *lockedBuffer
	done           chan struct{}
	code           int
	exited         time.Time
}

func startChild(t *testing.T, env []string, args ...string) *child {
	t.Helper()
	c := &child{stdout: &lockedBuffer{}, stderr: &lockedBuffer{}, done: make(chan struct{})}
	c.cmd = exec.CommandContext(context.Background(), bothyBinary, args...) // #nosec G204 -- the test's own binary
	c.cmd.Env = env
	c.cmd.Stdout, c.cmd.Stderr = c.stdout, c.stderr
	if err := c.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = c.cmd.Wait()
		c.code = c.cmd.ProcessState.ExitCode()
		c.exited = time.Now()
		close(c.done)
	}()
	t.Cleanup(func() {
		select {
		case <-c.done:
		default:
			_ = c.cmd.Process.Kill()
			<-c.done
		}
	})
	return c
}

func (c *child) signal(t *testing.T) {
	t.Helper()
	if err := c.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
}

func (c *child) wait(t *testing.T, limit time.Duration) int {
	t.Helper()
	select {
	case <-c.done:
		return c.code
	case <-time.After(limit):
		t.Fatalf("bothy did not exit within %v; stdout:\n%s\nstderr:\n%s", limit, c.stdout, c.stderr)
		return -1
	}
}

func (c *child) port(t *testing.T, msg string) int {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if p := logPort(c.stdout.String(), msg); p != 0 {
			return p
		}
		select {
		case <-c.done:
			t.Fatalf("bothy exited %d before %q; stderr:\n%s\nstdout:\n%s", c.code, msg, c.stderr, c.stdout)
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatalf("no %q line; stdout:\n%s", msg, c.stdout)
	return 0
}

// waitLine waits up to 30 seconds for stdout to contain text.
func (c *child) waitLine(t *testing.T, text string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(c.stdout.String(), text) {
		if time.Now().After(deadline) {
			t.Fatalf("no line with %s; stdout:\n%s", text, c.stdout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func httpGet(port int, path string) (int, string) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+path, nil)
	if err != nil {
		return 0, err.Error()
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func waitHTTP(t *testing.T, port int, path string, want int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var code int
	var body string
	for time.Now().Before(deadline) {
		if code, body = httpGet(port, path); code == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s = %d %q, want %d", path, code, body, want)
}

func TestServeHealthOnlyOnInternalListener(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	c := startChild(t, serveTestEnv(t, db), "serve", "--role=api")
	internal := c.port(t, "internal listener started")
	public := c.port(t, "public listener started")
	waitHTTP(t, internal, "/readyz", http.StatusOK)
	waitHTTP(t, internal, "/healthz", http.StatusOK)
	for _, path := range []string{"/healthz", "/readyz"} {
		// Before setup every path but discovery is 503 setup_required (M1-37), after the setup an
		// unmatched path is a 404 problem (M1-18); neither echoes the path.
		if code, body := httpGet(public, path); code != http.StatusServiceUnavailable || !strings.Contains(body, `"code":"setup_required"`) || strings.Contains(body, path) {
			t.Fatalf("public %s before setup = %d %q", path, code, body)
		}
	}
	if _, err := db.Pool(t, pg.RoleOwner).Exec(context.Background(), `UPDATE instance SET setup_completed_at = now() WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/healthz", "/readyz"} {
		if code, body := httpGet(public, path); code != http.StatusNotFound || !strings.Contains(body, `"code":"not_found"`) || strings.Contains(body, path) {
			t.Fatalf("public %s = %d %q", path, code, body)
		}
	}
	c.signal(t)
	if code := c.wait(t, 30*time.Second); code != 0 {
		t.Fatalf("exit %d; stderr %s", code, c.stderr)
	}
}

func TestServeReadyAfterStart(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	c := startChild(t, serveTestEnv(t, db), "serve")
	internal := c.port(t, "internal listener started")
	waitHTTP(t, internal, "/readyz", http.StatusOK)
	// The child's stdout reaches the buffer through a pipe, so a line written before /readyz
	// passed can still be in flight; wait for it instead of reading the buffer once.
	c.waitLine(t, `"msg":"server ready"`)
	if c.port(t, "public listener started") == 0 {
		t.Fatal("role all (the default) has no public listener")
	}
	c.signal(t)
	c.wait(t, 30*time.Second)
}

func TestServeSigtermExitsZeroWithinTotal(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	c := startChild(t, serveTestEnv(t, db, "BOTHY_SHUTDOWN__DRAIN_DELAY=2s", "BOTHY_SHUTDOWN__HTTP_GRACE=3s", "BOTHY_SHUTDOWN__TELEMETRY_FLUSH_TIMEOUT=1s", "BOTHY_SHUTDOWN__TOTAL_TIMEOUT=10s"), "serve", "--role=api")
	internal := c.port(t, "internal listener started")
	waitHTTP(t, internal, "/readyz", http.StatusOK)
	sent := time.Now()
	c.signal(t)
	waitHTTP(t, internal, "/readyz", http.StatusServiceUnavailable)
	if code, _ := httpGet(internal, "/healthz"); code != http.StatusOK {
		t.Fatalf("/healthz during the drain = %d", code)
	}
	if code := c.wait(t, 30*time.Second); code != 0 {
		t.Fatalf("exit %d; stderr %s", code, c.stderr)
	}
	if took := c.exited.Sub(sent); took > 10*time.Second || took < 2*time.Second {
		t.Fatalf("shutdown took %v, want between the drain delay and the total", took)
	}
	if !strings.Contains(c.stdout.String(), `"msg":"shutdown complete"`) {
		t.Fatalf("no shutdown line:\n%s", c.stdout)
	}
}

func TestServeSecondSignalExitsImmediately(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	c := startChild(t, serveTestEnv(t, db, "BOTHY_SHUTDOWN__DRAIN_DELAY=30s"), "serve", "--role=api")
	internal := c.port(t, "internal listener started")
	waitHTTP(t, internal, "/readyz", http.StatusOK)
	sent := time.Now()
	c.signal(t)
	waitHTTP(t, internal, "/readyz", http.StatusServiceUnavailable)
	c.signal(t)
	if code := c.wait(t, 20*time.Second); code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
	if took := c.exited.Sub(sent); took > 10*time.Second {
		t.Fatalf("forced end took %v", took)
	}
}

func TestServeWorkerHasNoPublicListener(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	c := startChild(t, serveTestEnv(t, db), "serve", "--role", "worker")
	internal := c.port(t, "internal listener started")
	waitHTTP(t, internal, "/readyz", http.StatusOK)
	if strings.Contains(c.stdout.String(), "public listener started") {
		t.Fatal("worker opened the public listener")
	}
	c.signal(t)
	if code := c.wait(t, 30*time.Second); code != 0 {
		t.Fatalf("exit %d", code)
	}
}

func TestServeAbortsOnOldSchema(t *testing.T) {
	t.Parallel()
	// The binary embeds migration 00001; a database without migrations has version 0.
	db := pgtest.New(t, pgtest.WithoutMigrations())
	c := startChild(t, serveTestEnv(t, db), "serve")
	if code := c.wait(t, 30*time.Second); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(c.stderr.String(), "bothy migrate up") {
		t.Fatalf("stderr = %q", c.stderr)
	}
	internal := logPort(c.stdout.String(), "internal listener started")
	if internal == 0 {
		t.Fatal("the internal listener never opened")
	}
	if code, _ := httpGet(internal, "/healthz"); code != 0 {
		t.Fatal("a listener stayed open")
	}
	// R-AR-16: serve never migrated.
	conn := db.SuperConn(t)
	var exists bool
	if err := conn.QueryRow(context.Background(), "SELECT to_regclass('goose_db_version') IS NOT NULL").Scan(&exists); err != nil || exists {
		t.Fatalf("goose_db_version exists = %v (%v)", exists, err)
	}
}

func TestServeDatabaseUnreachableExits1AfterWait(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	began := time.Now()
	c := startChild(t, serveTestEnv(t, db, "BOTHY_DATABASE__PORT=1", "BOTHY_STARTUP__DEPENDENCY_WAIT=1s"), "serve")
	if code := c.wait(t, 30*time.Second); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if took := time.Since(began); took < time.Second {
		t.Fatalf("exited after %v, before startup.dependency_wait", took)
	}
	if !strings.Contains(c.stderr.String(), "not reachable within 1s") {
		t.Fatalf("stderr = %q", c.stderr)
	}
}

func TestServeCommandUsage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args   []string
		env    []string
		stderr string
	}{
		{[]string{"serve", "--role=both"}, nil, "server.role"},
		{[]string{"serve", "--verbose"}, nil, "verbose"},
		{[]string{"serve", "extra"}, nil, "extra"},
		{[]string{"serve", "--config="}, nil, "--config needs a file path"},
		{[]string{"serve"}, []string{"BOTHY_SHUTDOWN__TOTAL_TIMEOUT=forever"}, "shutdown.total_timeout"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), tc.args, tc.env, &stdout, &stderr)
		if code != 2 || !strings.Contains(stderr.String(), tc.stderr) {
			t.Errorf("%q: exit %d, stderr %q", tc.args, code, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"serve", "--help"}, nil, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "Usage: bothy serve") {
		t.Errorf("help: exit %d, stdout %q", code, stdout.String())
	}
}

// TestServeCommandInProcess runs serve in this process with an injected signal channel, so the
// coverage of the command counts and the exit code path of a budget error is seen.
func TestServeCommandInProcess(t *testing.T) {
	t.Parallel()
	db := newServeDB(t)
	sigs := make(chan os.Signal, 2)
	stdout, stderr := &lockedBuffer{}, &lockedBuffer{}
	done := make(chan int, 1)
	go func() {
		done <- serveCommand(context.Background(), []string{"--role=worker"}, serveTestEnv(t, db), stdout, stderr, sigs)
	}()
	deadline := time.Now().Add(30 * time.Second)
	for !strings.Contains(stdout.String(), "server ready") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	sigs <- syscall.SIGTERM
	if code := <-done; code != 0 {
		t.Fatalf("exit %d; stderr %s", code, stderr)
	}

	env := serveTestEnv(t, db, "BOTHY_SHUTDOWN__DRAIN_DELAY=50s", "BOTHY_SHUTDOWN__HTTP_GRACE=20s")
	var out, errOut bytes.Buffer
	if code := serveCommand(context.Background(), nil, env, &out, &errOut, sigs); code != 2 || !strings.Contains(errOut.String(), "shutdown.total_timeout") {
		t.Fatalf("budget: exit %d, stderr %q", code, errOut.String())
	}
}

// TestExitCodesMatchLifecycle holds the exit codes of cmd/bothy and of the runner equal
// (02 "Befehle des Binarys").
func TestExitCodesMatchLifecycle(t *testing.T) {
	t.Parallel()
	if exitOK != lifecycle.ExitOK || exitFailure != lifecycle.ExitFailure || exitUsage != lifecycle.ExitUsage || exitForced != 3 {
		t.Fatal("exit codes differ")
	}
}
