// SPDX-License-Identifier: Apache-2.0

// Package harness is the shared full-server test harness of M1-69 to M1-72 (PLAN M1-71): Start
// runs the real internal/wire graph with role all in the test process against a disposable
// PostgreSQL 18 from pgtest, with secrets as bothy secrets init creates them, listeners on
// 127.0.0.1, and a first owner created through the setup service of bothy setup bootstrap-owner
// and enrolled with fresh ES256 device keys. Clients speak the public API with DPoP (RFC 9449).
//
// A test package that uses it calls Main from TestMain, so the templates of pgtest carry the
// steps of bothy migrate up:
//
//	func TestMain(m *testing.M) { os.Exit(harness.Main(m)) }
//
//	func TestSomething(t *testing.T) {
//		s := harness.Start(t, harness.Options{EgressDenyAll: true})
//		r := s.Owner.Do(t.Context(), harness.Request{Method: http.MethodGet, Path: "/me"})
//		...
//	}
//
// The package belongs to the architecture component testsupport (M1-05) and is never imported by
// production code.
package harness

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/jobstest"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets/stack"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// Main runs the tests of m with the pgtest templates migrated like bothy migrate up (goose and
// the steps of internal/wire); call it from TestMain of every package that uses Start. It builds
// the request validator of the server first, so no start pays it inside its readiness wait.
func Main(m *testing.M) int {
	if err := wire.WarmRequestValidator(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "harness: %v\n", err)
		return 1
	}
	defer cleanupBuild()
	return pgtest.Main(m, pgtest.WithSteps(wire.MigrationSteps))
}

// Options change a harness server.
type Options struct {
	// Config holds dotted keys of docs/operations/configuration.md; each overrides the harness
	// default with the precedence of an environment variable, so an unknown key fails the start.
	Config map[string]string
	// LogWriter receives the JSON log lines of the server besides the harness; nil discards them.
	LogWriter io.Writer
	// Telemetry are in-process telemetry sinks (NewTelemetrySinks): the server exports spans,
	// OTLP log records and metrics into them instead of over OTLP, and telemetry.enabled is true
	// unless Config sets it. otel.Setup installs global providers, so a test that sets it never
	// runs in parallel with another harness start of its binary.
	Telemetry *TelemetrySinks
	// EgressDenyAll makes every outgoing client of the server refuse every target before name
	// resolution (wire.Deps.EgressDenyAll).
	EgressDenyAll bool
	// Clock is the clock of the harness: expiry times of BuildMatrixWorld and the shutdown budget
	// of the server; nil means clock.System().
	Clock clock.Clock
	// DurableDB puts the database on the durable pgtest server (pgtest.Durable: fsync,
	// synchronous_commit and full_page_writes on); every perf and chaos test sets it (ADR 0063).
	DurableDB bool
	// SecretFiles are extra files of the secrets directory of the server by relative path, written
	// with mode 0400 after bothy secrets init (the client secret of a fake OIDC provider as
	// idp/<name>, referenced as file:<secrets.dir>/idp/<name>; M1-69).
	SecretFiles map[string][]byte
	// OIDCRootCAs are the trusted TLS roots of the OIDC provider requests of the server
	// (wire.Deps.OIDCRootCAs): the pool of the fake provider of oidctest; nil means the system
	// roots.
	OIDCRootCAs *x509.CertPool
	// PublicHost is the host of server.public_url; empty means 127.0.0.1. "localhost" gives the
	// server a domain name as WebAuthn relying party (passkeys, M1-40); the listeners stay on
	// 127.0.0.1 either way.
	PublicHost string
	// ProofObserver, when set, sees every DPoP proof a client of the server sends (requests and
	// gateway identify or reauth), so the canary test registers them as secrets.
	ProofObserver func(proof string)
	// TokenObserver, when set, sees every token pair a client of the server takes over (the
	// enrollment of the owner and of NewUser, refreshes, Adopt) and the API token of NewAgent
	// (refresh empty), so the canary test registers them as secrets.
	TokenObserver func(access, refresh string)
	// TerminalTimers drive the deadlines of the terminal relay (wire.Deps.TerminalTimers, M2-44);
	// nil means the real timers.
	TerminalTimers terminalapp.Timers
	// TerminalBeforeOpenCommit runs inside the unit of work of every terminal opening before its
	// commit (wire.Deps.TerminalBeforeOpenCommit, M2-44); nil runs nothing.
	TerminalBeforeOpenCommit func()
	// FilesImageHooks are the test seams of the job files.thumbnail (wire.Deps.FilesImageHooks,
	// M3-06): hold, fail or observe a run; nil in every other test.
	FilesImageHooks *filesapp.ImageHooks
	// PublicURL, when set, is server.public_url instead of the origin of the own public listener:
	// several api nodes behind one edge share it (M2-47). The node still listens on its own port
	// of 127.0.0.1, which Server.ListenURL names.
	PublicURL string
	// Role is the role of the process (wire.RoleAll when empty); a worker has no public listener.
	Role wire.Role
	// TerminalSignals and TerminalListenerGate are the test seams of the terminal across nodes
	// (wire.Deps.TerminalSignals and wire.Deps.TerminalListenerGate, M2-47).
	TerminalSignals      func(terminalapp.Signals) terminalapp.Signals
	TerminalListenerGate func(ctx context.Context) error
	// TerminalMetrics replaces the instruments of the relay (wire.Deps.TerminalMetrics, M2-47).
	TerminalMetrics terminalapp.Metrics
	// TerminalModule receives the use cases of the terminal module (wire.Deps.TerminalModule,
	// M2-47).
	TerminalModule func(*terminalapp.Service)
	// ScopeObserver sees every Scope call of the server's Authorizer with its context and action
	// (wire.Deps.ScopeObserver, M2-14).
	ScopeObserver func(ctx context.Context, act authorize.Action)
	// OnListen, when set, receives the origin of the own public listener before the server starts
	// (an edge in front of several nodes routes to it, M2-47).
	OnListen func(listenURL string)
	// JobModules receives the use cases of the jobs (wire.Deps.JobModules, M2-46): a test runs
	// authz.expiry_sweep or terminal.sweep directly.
	JobModules func(*wire.JobModules)
	// TerminalSessionMetrics replaces the session instruments of the terminal
	// (wire.Deps.TerminalSessionMetrics, M2-46).
	TerminalSessionMetrics terminalapp.SessionMetrics
}

// Server is a running harness server.
type Server struct {
	// PublicURL is server.public_url, the origin of the public listener; APIBase is PublicURL plus
	// /api/v1. GatewayURL is the WebSocket address of the gateway (M1-64 serves it).
	PublicURL, APIBase, GatewayURL string
	// InternalURL is the origin of the internal listener (/healthz, /readyz).
	InternalURL string
	// ListenURL is the origin of the own public listener; it equals PublicURL unless
	// Options.PublicURL put the node behind an edge (M2-47).
	ListenURL string
	// SecretsDir is secrets.dir of the server.
	SecretsDir string
	// SigningKeyDir is secrets.signing_key_dir of the server, the store of its private signing
	// keys.
	SigningKeyDir string
	// FilesRoot is file.fs.root of the server, the root of the file system driver (M3-03).
	FilesRoot string
	// Owner is the first owner, enrolled with fresh device keys.
	Owner *Client

	// proofs and tokens are Options.ProofObserver and Options.TokenObserver, handed to every
	// client of the server.
	proofs func(string)
	tokens func(access, refresh string)
	db     *pgtest.DB
	tb     testing.TB
	clk    clock.Clock
	http   *http.Client
	users  sync.Map // handle -> *Client

	adminOnce sync.Once
	admin     *pgxpool.Pool
	// values and env are the configuration of the server; the setup service reads them.
	values *config.Values
	env    []string
}

// AdminDB returns a pool of the superuser of the disposable container; only for assertions.
func (s *Server) AdminDB() *pgxpool.Pool {
	s.adminOnce.Do(func() { s.admin = s.db.OpenPool(s.tb, s.db.SuperConfig()).PGX() })
	return s.admin
}

// Clock returns the clock of the harness.
func (s *Server) Clock() clock.Clock { return s.clk }

// The fixed values of a harness server.
const (
	ownerHandle      = "owner"
	ownerDisplayName = "Owner"
	readyWait        = 3 * time.Minute
	stopWait         = 60 * time.Second
	portAttempts     = 3
)

// Start runs a harness server for tb and stops it in tb.Cleanup.
func Start(tb testing.TB, opts Options) *Server {
	tb.Helper()
	clk := opts.Clock
	if clk == nil {
		clk = clock.System()
	}
	var dbOpts []pgtest.Option
	if opts.DurableDB {
		dbOpts = append(dbOpts, pgtest.Durable())
	}
	db := pgtest.New(tb, dbOpts...)
	jobstest.Migrate(tb, db, "jobs")
	secretsDir := initSecrets(tb, db)
	writeSecretFiles(tb, secretsDir, opts.SecretFiles)
	keys := filepath.Join(tb.TempDir(), "signing-keys")
	if err := os.Mkdir(keys, 0o700); err != nil {
		tb.Fatal(err)
	}
	// file.fs.root of the file system driver (M3-03): a directory of the test with mode 0700.
	files := filepath.Join(tb.TempDir(), "files")
	if err := os.Mkdir(files, 0o700); err != nil {
		tb.Fatal(err)
	}
	code := ""
	return startRetrying(tb, db, opts, clk, secretsDir, keys, files, &code)
}

// startRetrying starts one server process, again when its reserved public port was taken.
func startRetrying(tb testing.TB, db *pgtest.DB, opts Options, clk clock.Clock, secretsDir, keys, files string, code *string) *Server {
	tb.Helper()
	var lastErr error
	for range portAttempts {
		s, err := start(tb, db, opts, clk, secretsDir, keys, files, code)
		if err == nil {
			return s
		}
		lastErr = err
		if !errors.Is(err, errPortTaken) {
			break
		}
	}
	tb.Fatalf("harness: %v", lastErr)
	return nil
}

// StartNode starts another process of the installation of first (M2-47): the same database,
// secrets, signing key store and file root, with opts for its own configuration, role and test
// seams. It enrolls no owner: Owner is the owner of first, whose clients reach the node through
// the shared Options.PublicURL. It stops in tb.Cleanup.
func StartNode(tb testing.TB, first *Server, opts Options) *Server {
	tb.Helper()
	code := "-"
	s := startRetrying(tb, first.db, opts, first.clk, first.SecretsDir, first.SigningKeyDir, first.FilesRoot, &code)
	s.Owner = first.Owner
	return s
}

// errPortTaken reports that the reserved public port was taken before the server bound it.
var errPortTaken = errors.New("harness: the reserved public port was taken")

// start starts one server process; the public port is reserved first, because server.public_url
// must name it before the listener exists.
func start(tb testing.TB, db *pgtest.DB, opts Options, clk clock.Clock, secretsDir, keys, files string, code *string) (*Server, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	host := opts.PublicHost
	if host == "" {
		host = "127.0.0.1"
	}
	listen := "http://" + host + ":" + strconv.Itoa(port)
	public := listen
	if opts.PublicURL != "" {
		public = opts.PublicURL
	}
	env := environment(db, opts, public, secretsDir, keys, files, port)
	if opts.OnListen != nil {
		opts.OnListen(listen)
	}
	schema, err := wire.ConfigSchema()
	if err != nil {
		return nil, err
	}
	values, err := config.Load(schema, config.Sources{Env: env})
	if err != nil {
		return nil, err
	}
	if *code == "" {
		if *code, err = bootstrapOwner(tb, values, env); err != nil {
			return nil, err
		}
	}
	logs := &logBuffer{}
	var out io.Writer = logs
	if opts.LogWriter != nil {
		out = io.MultiWriter(logs, opts.LogWriter)
	}
	sigs := make(chan os.Signal, 2)
	deps := wire.Deps{
		Env: env, Stdout: out, Signals: sigs, Clock: clk, EgressDenyAll: opts.EgressDenyAll, OIDCRootCAs: opts.OIDCRootCAs,
		TerminalTimers: opts.TerminalTimers, TerminalBeforeOpenCommit: opts.TerminalBeforeOpenCommit,

		FilesImageHooks: opts.FilesImageHooks,
		TerminalSignals: opts.TerminalSignals, TerminalListenerGate: opts.TerminalListenerGate, TerminalMetrics: opts.TerminalMetrics,
		TerminalModule: opts.TerminalModule, JobModules: opts.JobModules, TerminalSessionMetrics: opts.TerminalSessionMetrics,
		ScopeObserver: opts.ScopeObserver,
	}
	if opts.Telemetry != nil {
		deps.TelemetryExporters = opts.Telemetry.exporters()
	}
	role := opts.Role
	if role == "" {
		role = wire.RoleAll
	}
	runner, err := wire.Serve(context.Background(), values, role, deps)
	if err != nil {
		return nil, err
	}
	done := make(chan int, 1)
	go func() { done <- runner.Run(context.Background()) }()
	tb.Cleanup(func() { stop(tb, sigs, done, logs) })
	internal, err := waitReady(done, logs)
	if err != nil {
		if strings.Contains(logs.String(), "address already in use") {
			return nil, errPortTaken
		}
		return nil, err
	}
	gateway := "ws://" + host + ":" + strconv.Itoa(port) + apiPrefix + "/gateway"
	if opts.PublicURL != "" {
		gateway = "ws" + strings.TrimPrefix(public, "http") + apiPrefix + "/gateway"
	}
	s := &Server{
		PublicURL: public, APIBase: public + apiPrefix, GatewayURL: gateway, ListenURL: listen,
		InternalURL: internal, SecretsDir: secretsDir, SigningKeyDir: keys, FilesRoot: files, proofs: opts.ProofObserver, tokens: opts.TokenObserver, db: db, tb: tb, clk: clk, values: values, env: env,
		http: &http.Client{Timeout: 60 * time.Second},
	}
	if *code == "-" {
		// Another node of an installation (StartNode): the owner exists.
		return s, nil
	}
	owner, err := s.redeem(tb.Context(), *code, "Owner device")
	if err != nil {
		return nil, err
	}
	s.Owner = owner
	return s, nil
}

// freePort returns a port of 127.0.0.1 that was free a moment ago.
func freePort() (int, error) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return port, ln.Close()
}

// environment builds the process environment of the server: the database of db as
// bothy_app, the secrets, the key store and the root of the file system driver, both listeners
// on 127.0.0.1, then Options.Config.
func environment(db *pgtest.DB, opts Options, public, secretsDir, keys, files string, port int) []string {
	cfg := db.Config(pg.RoleApp)
	set := map[string]string{
		"secrets.dir":                 secretsDir,
		"secrets.signing_key_dir":     keys,
		"file.fs.root":                files,
		"database.host":               cfg.Host,
		"database.port":               strconv.Itoa(cfg.Port),
		"database.name":               db.Name(),
		"database.user":               "bothy_app",
		"database.sslmode":            "disable",
		"server.public_url":           public,
		"server.listen_addr":          "127.0.0.1:" + strconv.Itoa(port),
		"server.internal_listen_addr": "127.0.0.1:0",
		"startup.dependency_wait":     "30s",
		"telemetry.enabled":           "false",
		"telemetry.logs.exporter":     "stdout",
		"shutdown.drain_delay":        "0s",
	}
	if opts.Telemetry != nil {
		set["telemetry.enabled"] = "true"
	}
	for k, v := range opts.Config {
		set[k] = v
	}
	env := make([]string, 0, len(set))
	for k, v := range set {
		env = append(env, config.EnvName(k)+"="+v)
	}
	return env
}

// initSecrets creates the secret volume as bothy secrets init does, with the database passwords
// of the pgtest container as supplied source, and returns the directory of the server service.
func initSecrets(tb testing.TB, db *pgtest.DB) string {
	tb.Helper()
	src := tb.TempDir()
	for _, name := range []string{"db_superuser_password", "db_owner_password", "db_app_password", "db_maint_password", "db_monitor_password", "db_backup_password"} {
		b, err := os.ReadFile(filepath.Join(db.SecretsDir(), name)) // #nosec G304 -- a fixed name in the pgtest directory
		if err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, name), b, 0o600); err != nil { //nolint:gosec // G703: a fixed name in a temp dir of the test
			tb.Fatal(err)
		}
	}
	target := filepath.Join(tb.TempDir(), "secrets")
	me := stack.Owner{UID: os.Getuid(), GID: os.Getgid()}
	if _, err := stack.Init(tb.Context(), stack.InitOptions{
		Target: target, Source: src, Owners: map[string]stack.Owner{"server": me, "migrate": me},
	}); err != nil {
		tb.Fatalf("harness: secrets init: %v", err)
	}
	dir := filepath.Join(target, "server")
	tb.Cleanup(func() { makeRemovable(target) })
	return dir
}

// writeSecretFiles writes files below dir with mode 0400 in directories of mode 0700, through an
// os.Root, so a relative path cannot leave dir; dir itself is made writable for the owner only
// while it writes.
func writeSecretFiles(tb testing.TB, dir string, files map[string][]byte) {
	tb.Helper()
	if len(files) == 0 {
		return
	}
	fi, err := os.Stat(dir)
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // G302: a directory needs the search bit; owner only
		tb.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	for rel, b := range files {
		if err := root.MkdirAll(filepath.Dir(rel), 0o700); err != nil {
			tb.Fatal(err)
		}
		if err := root.WriteFile(rel, b, 0o400); err != nil {
			tb.Fatal(err)
		}
	}
	if err := os.Chmod(dir, fi.Mode().Perm()); err != nil {
		tb.Fatal(err)
	}
}

// makeRemovable gives the owner write access to the secret volume again, so t.TempDir can remove
// the read-only service directories.
func makeRemovable(root string) {
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(path, 0o700) //nolint:gosec // G122, G302: a temp directory of the own test
		}
		return nil
	})
}

// bootstrapOwner runs the service of bothy setup bootstrap-owner and returns the enrollment code.
func bootstrapOwner(tb testing.TB, values *config.Values, env []string) (string, error) {
	ctx := tb.Context()
	setup, err := wire.NewSetup(ctx, values, env, slog.New(slog.DiscardHandler))
	if err != nil {
		return "", err
	}
	defer setup.Close(context.WithoutCancel(ctx))
	res, err := setup.BootstrapOwner(ctx, wire.BootstrapOwnerCmd{Handle: ownerHandle, DisplayName: ownerDisplayName, Locale: "de", TimeZone: "UTC"})
	if err != nil {
		return "", fmt.Errorf("bootstrap-owner: %w", err)
	}
	return res.Code.Code, nil
}

// waitReady waits for the internal listener and /readyz 200 and returns the internal origin.
// A process that exits first is an error with its log.
func waitReady(done chan int, logs *logBuffer) (string, error) {
	deadline := time.Now().Add(readyWait)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		select {
		case code := <-done:
			done <- code
			return "", fmt.Errorf("server exited with %d:\n%s", code, logs.String())
		default:
		}
		if port := logs.port("internal listener started"); port != 0 {
			origin := "http://127.0.0.1:" + strconv.Itoa(port)
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, origin+"/readyz", nil)
			if resp, err := client.Do(req); err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return origin, nil
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", fmt.Errorf("server not ready after %s:\n%s", readyWait, logs.String())
}

// stop ends the server like SIGTERM and waits for the runner; a hanging stop fails tb.
func stop(tb testing.TB, sigs chan os.Signal, done chan int, logs *logBuffer) {
	select {
	case <-done:
		return
	default:
	}
	sigs <- syscall.SIGTERM
	select {
	case code := <-done:
		if code != lifecycle.ExitOK {
			tb.Errorf("harness: server exited with %d:\n%s", code, logs.String())
		}
	case <-time.After(stopWait):
		tb.Errorf("harness: server did not stop within %s", stopWait)
	}
}

// redeem enrolls a new device with code (redeemEnrollmentCode) and returns its client.
func (s *Server) redeem(ctx context.Context, code, name string) (*Client, error) {
	c := s.NewDevice()
	resp := c.send(ctx, Request{Method: http.MethodPost, Path: "/auth/enroll/redeem", Body: map[string]any{
		"enrollment_code": code,
		"device":          c.registration(name),
	}}, false)
	if resp.Status != http.StatusCreated {
		return nil, fmt.Errorf("redeemEnrollmentCode: %d %s", resp.Status, resp.Body)
	}
	var pair tokenPair
	if err := json.Unmarshal(resp.Body, &pair); err != nil {
		return nil, err
	}
	c.adopt(pair)
	c.PrincipalID = pair.Device.PrincipalID
	if c.PrincipalID == "" {
		me := c.Do(ctx, Request{Method: http.MethodGet, Path: "/me"})
		var body struct {
			Principal struct {
				ID string `json:"id"`
			} `json:"principal"`
		}
		if err := json.Unmarshal(me.Body, &body); err != nil || body.Principal.ID == "" {
			return nil, fmt.Errorf("getMe: %d %s", me.Status, me.Body)
		}
		c.PrincipalID = body.Principal.ID
	}
	return c, nil
}

// NewDevice returns a client of the server with fresh DPoP and step-up device keys and no
// tokens yet: the device of a login, pairing or enrollment flow the test drives itself. Adopt
// gives it the token pair of the flow.
func (s *Server) NewDevice() *Client {
	return &Client{publicURL: s.PublicURL, http: s.http, tb: s.tb, key: newDeviceKey(), stepKey: newDeviceKey(), DeviceID: ids.New().String(), proofs: s.proofs, tokens: s.tokens}
}

// registration is the DeviceRegistration of c with its DPoP and step-up keys.
func (c *Client) registration(name string) map[string]any {
	return map[string]any{
		"device_id": c.DeviceID, "name": name, "platform": "macos", "public_jwk": json.RawMessage(c.key.jwk), "alg": "ES256",
		"step_up_key": map[string]any{"public_jwk": json.RawMessage(c.stepKey.jwk), "alg": "ES256"},
	}
}

// logBuffer collects the JSON log lines of the server.
type logBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

// Write appends p.
func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

// String returns the collected lines.
func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// port returns server.port of the first line with msg, or 0.
func (l *logBuffer) port(msg string) int {
	sc := bufio.NewScanner(strings.NewReader(l.String()))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
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

// RaisedRateLimits returns Options.Config entries that raise per_minute and burst of every
// rate-limit class to the largest allowed value. Test servers that build large worlds through the
// public API use it (23 R-QA-11); measured load tests and tests of the limits keep the defaults.
func RaisedRateLimits() map[string]string {
	limit := strconv.Itoa(ratelimit.MaxLimit)
	out := map[string]string{}
	for _, c := range ratelimit.Classes() {
		out["ratelimit."+string(c)+".per_minute"] = limit
		out["ratelimit."+string(c)+".burst"] = limit
	}
	return out
}

// ConfigValue returns the effective value of a configuration key of the server as text.
func (s *Server) ConfigValue(key string) string { return s.values.Text(key) }
