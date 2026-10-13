// SPDX-License-Identifier: Apache-2.0

// Package pgtest gives integration tests a fresh PostgreSQL 18 database each (ADR 0006, plan
// P-09 and P-10). One disposable container per test binary runs the pinned Image on 127.0.0.1
// with a random host port; its four roles are bootstrapped with pg.Bootstrap and generated
// EXAMPLE passwords, and a template database per migration set is migrated once with
// pg.Migrator as bothy_owner, so tests run against the schema production gets. Every New clones
// that template.
//
// A test package calls Main from TestMain. Without a Docker-compatible runtime Main fails the
// whole test binary with a clear message; integration tests never skip.
//
// Example:
//
//	func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }
//
//	func TestSomething(t *testing.T) {
//		db := pgtest.New(t)
//		pool := db.Pool(t, pg.RoleApp)
//		...
//	}
package pgtest

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/testcontainers/testcontainers-go"
	tclog "github.com/testcontainers/testcontainers-go/log"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/migrations"
)

// The labels of every pgtest container: `docker ps --filter label=org.bothy.test=pgtest` finds
// them (README.md). watchdogLabel marks a container that removes itself when its test binary is
// gone; containers of older pgtest versions lack it.
const (
	label              = "org.bothy.test"
	labelValue         = "pgtest"
	watchdogLabel      = "org.bothy.test.watchdog"
	watchdogLabelValue = "lease"
	// containerPort is the PostgreSQL port inside the container.
	containerPort = "5432/tcp"
)

// The superuser of the container and the database it connects to for CREATE DATABASE.
const (
	superUser     = "postgres"
	superDatabase = "postgres"
)

// secretFiles are the names of the password files in SecretsDir, the names of the secret
// catalog of the Compose stack (docs/operations/configuration.md "Secrets").
var secretFiles = map[pg.Role]string{
	pg.RoleOwner:   "db_owner_password",
	pg.RoleApp:     "db_app_password",
	pg.RoleMaint:   "db_maint_password",
	pg.RoleMonitor: "db_monitor_password",
}

// superSecretFile is the password file of the superuser (migrate.bootstrap_password_ref).
const superSecretFile = "db_superuser_password" // #nosec G101 -- a file name of the secret catalog, not a credential

// The deadlines and pool size of Config: generous, because tests share one container and run
// with the race detector; a test of the deadlines sets its own.
const (
	testMaxConns         = 4
	testAcquireTimeout   = 10 * time.Second
	testStatementTimeout = 30 * time.Second
	testLockTimeout      = 10 * time.Second
	testIdleInTxTimeout  = 60 * time.Second
	connectWait          = 30 * time.Second
	startTimeout         = 3 * time.Minute
	cleanupTimeout       = 30 * time.Second
)

// The lease: the test binary holds one superuser connection with application_name leaseApp for
// as long as it runs, and checks it every leaseInterval, reconnecting when it broke. Right after
// opening it, it marks the lease as taken (the comment leaseApp on the database postgres), so the
// watchdog knows about the lease even when the binary dies within the first second. The watchdog
// in the container (watchdogScript) checks every second; once the lease is taken, leaseMisses
// checks in a row without the connection stop PostgreSQL, the container exits and
// Docker removes it (AutoRemove) with its anonymous data volume. The operating system closes the
// connection of a test binary however it ends (failed or panicking test, the -test.timeout
// alarm, a signal, SIGKILL), so a container usually outlives its run by little more than
// leaseMisses seconds plus the shutdown (at most leaseStopWait). A container whose lease was never
// taken, or whose server gives no readable answer for leaseStartGrace, stops as well.
const (
	leaseApp        = "pgtest-lease"
	leaseInterval   = time.Second
	leaseMisses     = 10
	leaseStartGrace = startTimeout + time.Minute
	leaseStopWait   = 30 * time.Second
)

// watchdogScript is the entry point of the container: it starts the image's own entry point
// with the command ("$@") in the background and stops PostgreSQL (fast shutdown) when the lease
// of the test binary is gone, or when docker stop sends its stop signal (SIGINT or SIGTERM). A
// count it cannot read (server starting, restarting or busy) is no verdict, but once the lease
// is taken, leaseStartGrace without a readable count stops the server as well, so a hung server
// cannot keep its container. stop waits at most leaseStopWait for PostgreSQL to end; then the
// script exits, and with it the container's main process.
const watchdogScript = `set -u
docker-entrypoint.sh "$@" &
pg=$!
stop() {
	kill -INT "$pg" 2>/dev/null
	waited=0
	while kill -0 "$pg" 2>/dev/null && [ "$waited" -lt "$PGTEST_STOP_WAIT" ]; do
		sleep 1
		waited=$((waited + 1))
	done
	exit 0
}
trap stop TERM INT
query="SELECT coalesce(shobj_description(d.oid, 'pg_database'), '') = '$PGTEST_LEASE_APP', (SELECT count(*) FROM pg_stat_activity WHERE application_name = '$PGTEST_LEASE_APP') FROM pg_database d WHERE d.datname = 'postgres'"
taken=f
misses=0
seen=$SECONDS
while kill -0 "$pg" 2>/dev/null; do
	sleep 1 & wait $!
	out=$(timeout 10 psql -h /var/run/postgresql -U postgres -d postgres -XAtq -F ' ' -c "$query" 2>/dev/null) || out=
	read -r marked n <<<"$out" || true
	case "${n:-x}" in
	*[!0-9]*) ;;
	*)
		seen=$SECONDS
		if [ "$marked" = t ]; then
			taken=t
			if [ "$n" -eq 0 ]; then misses=$((misses + 1)); else misses=0; fi
		fi
		;;
	esac
	if [ "$taken" = t ] && [ "$misses" -ge "$PGTEST_LEASE_MISSES" ]; then stop; fi
	if [ "$taken" = t ] && [ $((SECONDS - seen)) -ge "$PGTEST_LEASE_START_GRACE" ]; then stop; fi
	if [ "$taken" = f ] && [ "$SECONDS" -ge "$PGTEST_LEASE_START_GRACE" ]; then stop; fi
done
wait "$pg"
`

// watchdogEnv is the environment of watchdogScript for the given limits.
func watchdogEnv(misses int, grace, stopWait time.Duration) map[string]string {
	return map[string]string{
		"PGTEST_LEASE_APP":         leaseApp,
		"PGTEST_LEASE_MISSES":      strconv.Itoa(misses),
		"PGTEST_LEASE_START_GRACE": strconv.Itoa(int(grace / time.Second)),
		"PGTEST_STOP_WAIT":         strconv.Itoa(int(stopWait / time.Second)),
	}
}

// legacyReapAge is the age after which Main removes a container labelled org.bothy.test without
// the watchdog label: one that an older pgtest or a test helper without a watchdog left behind.
// go test stops a binary after 10 minutes by default; such a run with -timeout 0 or a timeout
// above legacyReapAge is not supported and may lose its container.
const legacyReapAge = 2 * time.Hour

// noRuntimeMessage is what Main prints when no container runtime answers (plan P-09).
const noRuntimeMessage = "pgtest: no Docker-compatible runtime reachable (%v); integration tests need one (plan P-09)\n"

// state is the container of the running test binary.
var state struct {
	mu      sync.Mutex
	active  bool // Main is running the tests
	cluster *cluster
	err     error // a failed start, reported to every later New
	steps   Steps // the steps of bothy migrate up for every template, from Main
}

// Steps builds the steps bothy migrate up runs around the goose migrations, for a pool of
// bothy_owner on the database being migrated and the migration set fsys. A test binary that
// needs what those steps write (the instance row of M1-15) passes wire.MigrationSteps through
// WithSteps; pgtest itself cannot import internal/wire.
type Steps func(pool *pg.Pool, fsys fs.FS) (pg.MigratorOptions, error)

// MainOption changes how Main prepares the databases of a test binary.
type MainOption func(*mainOptions)

type mainOptions struct {
	steps Steps
}

// WithSteps makes every template of the binary migrate with the steps of steps, so New returns
// the database bothy migrate up produces, including the rows those steps write. Without it the
// templates get the goose migrations only.
func WithSteps(steps Steps) MainOption { return func(o *mainOptions) { o.steps = steps } }

// cluster is the started container with its roles and template databases.
type cluster struct {
	ctr        *testcontainers.DockerContainer
	host       string
	port       int
	secretsDir string
	provider   *secrets.Provider

	mu        sync.Mutex        // serialises template creation and cloning
	templates map[string]string // migration set hash -> template database
	steps     Steps             // the steps of Main's WithSteps, nil for none

	stopLease func() // closes the lease connection (watchdogScript)
}

// Main runs the tests of m and returns the exit code for os.Exit; call it from TestMain. It
// disables Ryuk (testcontainers' reaper would publish its port beyond 127.0.0.1) and checks that
// a Docker-compatible runtime answers: without one it prints
// "pgtest: no Docker-compatible runtime reachable (<error>); integration tests need one (plan P-09)"
// and returns 1 without running a test. The container starts with the first New; after the tests
// Main terminates it and removes SecretsDir, and turns a failed cleanup into exit code 1. When
// the binary ends without that cleanup (a panic, the -test.timeout alarm, a signal), the
// container removes itself once the lease connection is gone. Main also removes test containers
// labelled org.bothy.test that lack that watchdog (older pgtest versions, other test helpers)
// once they are older than legacyReapAge.
//
// Options: WithSteps adds the steps of bothy migrate up to the templates.
func Main(m *testing.M, opts ...MainOption) int {
	var o mainOptions
	for _, opt := range opts {
		opt(&o)
	}
	state.mu.Lock()
	state.steps = o.steps
	state.mu.Unlock()
	// testcontainers reads its configuration once, at the first call; only the exact value
	// "true" disables Ryuk.
	if err := os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true"); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "pgtest: %v\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectWait)
	err := runtimeReachable(ctx)
	cancel()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, noRuntimeMessage, err)
		return 1
	}
	ctx, cancel = context.WithTimeout(context.Background(), cleanupTimeout)
	err = reapLegacy(ctx, label, time.Now())
	cancel()
	if err != nil {
		// Only housekeeping for containers of other, finished runs; the tests can still run.
		_, _ = fmt.Fprintf(os.Stderr, "pgtest: remove leftover containers: %v\n", err)
	}
	setActive(true)
	code := m.Run()
	setActive(false)
	if err := stopCluster(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "pgtest: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// runtimeReachable asks the container runtime for its info. testcontainers panics on some
// broken setups (a socket path it cannot parse); that counts as unreachable.
func runtimeReachable(ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	p, err := testcontainers.NewDockerProvider(testcontainers.WithLogger(tclog.NewNoopLogger()))
	if err != nil {
		return err
	}
	return p.Health(ctx)
}

func setActive(active bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.active = active
}

// sharedCluster returns the container of this test binary, starting it on first use.
func sharedCluster() (*cluster, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.active {
		return nil, errors.New("pgtest: New needs pgtest.Main: add func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) } to the test package")
	}
	if state.cluster == nil && state.err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
		state.cluster, state.err = startCluster(ctx)
		cancel()
		if state.cluster != nil {
			state.cluster.steps = state.steps
		}
	}
	return state.cluster, state.err
}

// stopCluster terminates the container and removes the secrets directory.
func stopCluster() error {
	state.mu.Lock()
	defer state.mu.Unlock()
	c := state.cluster
	state.cluster, state.err = nil, nil
	if c == nil {
		return nil
	}
	var problems []error
	c.stopLease()
	// TerminateContainer ignores the errors of a container that AutoRemove already removes.
	if err := testcontainers.TerminateContainer(c.ctr); err != nil {
		problems = append(problems, fmt.Errorf("terminate container %s: %w", c.ctr.GetContainerID(), err))
	}
	if err := os.RemoveAll(c.secretsDir); err != nil {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}

// startCluster starts the container and bootstraps the roles.
func startCluster(ctx context.Context) (c *cluster, err error) {
	dir, err := os.MkdirTemp("", "pgtest-secrets-")
	if err != nil {
		return nil, fmt.Errorf("pgtest: secrets directory: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()
	superPassword, passwords, err := writePasswords(dir)
	if err != nil {
		return nil, err
	}
	ctr, err := runContainer(ctx, superPassword)
	if ctr != nil {
		defer func() {
			if err != nil {
				_ = testcontainers.TerminateContainer(ctr)
			}
		}()
	}
	if err != nil {
		return nil, fmt.Errorf("pgtest: start %s: %w", Image, err)
	}
	mapped, err := ctr.MappedPort(ctx, containerPort)
	if err != nil {
		return nil, fmt.Errorf("pgtest: mapped port: %w", err)
	}
	provider, err := secrets.NewProvider(secrets.Options{Dir: dir})
	if err != nil {
		return nil, err
	}
	c = &cluster{
		ctr: ctr, host: "127.0.0.1", port: int(mapped.Num()), secretsDir: dir, provider: provider,
		templates: map[string]string{},
	}
	conn, err := c.superConn(ctx, superDatabase)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(context.Background()) }()
	if err := pg.BootstrapRoles(ctx, conn, passwords); err != nil {
		return nil, fmt.Errorf("pgtest: bootstrap roles: %w", err)
	}
	if c.stopLease, err = c.holdLease(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// holdLease opens the lease connection, marks the lease as taken and keeps the connection open
// until the returned stop is called:
// every leaseInterval it pings the connection and reopens it when it broke, so the watchdog in
// the container sees the lease for as long as this process lives.
func (c *cluster) holdLease(ctx context.Context) (stop func(), err error) {
	cfg := c.superConfig(superDatabase).WithAppName(leaseApp)
	quiet := slog.New(slog.DiscardHandler)
	conn, err := pg.Connect(ctx, cfg, c.provider, connectWait, quiet)
	if err != nil {
		return nil, fmt.Errorf("pgtest: lease connection: %w", err)
	}
	mark := "COMMENT ON DATABASE " + pgx.Identifier{superDatabase}.Sanitize() + " IS '" + leaseApp + "'"
	if _, err := conn.Exec(ctx, mark); err != nil {
		_ = conn.Close(context.WithoutCancel(ctx))
		return nil, fmt.Errorf("pgtest: mark the lease: %w", err)
	}
	// The lease outlives ctx, which only bounds the start; stop ends it.
	lctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	go func() {
		defer close(done)
		tick := time.NewTicker(leaseInterval)
		defer tick.Stop()
		for {
			select {
			case <-lctx.Done():
				if conn != nil {
					cctx, ccancel := context.WithTimeout(context.WithoutCancel(lctx), cleanupTimeout)
					_ = conn.Close(cctx)
					ccancel()
				}
				return
			case <-tick.C:
			}
			conn = keepLease(lctx, conn, func(ctx context.Context) (*pgx.Conn, error) {
				return pg.Connect(ctx, cfg, c.provider, 5*leaseInterval, quiet)
			})
		}
	}()
	return func() { cancel(); <-done }, nil
}

// keepLease pings conn and returns it when it answers; otherwise it closes it and returns a new
// connection from open, or nil when that fails too (the next tick tries again).
func keepLease(ctx context.Context, conn *pgx.Conn, open func(context.Context) (*pgx.Conn, error)) *pgx.Conn {
	if conn != nil {
		pctx, cancel := context.WithTimeout(ctx, 5*leaseInterval)
		err := conn.Ping(pctx)
		cancel()
		if err == nil {
			return conn
		}
		_ = conn.Close(context.WithoutCancel(ctx))
	}
	octx, cancel := context.WithTimeout(ctx, 5*leaseInterval)
	defer cancel()
	next, err := open(octx)
	if err != nil {
		return nil
	}
	return next
}

// reapLegacy removes the containers that carry the label key (any value) but lack the watchdog
// label and were created more than legacyReapAge before now, with their anonymous volumes. Main
// passes the key org.bothy.test, so this also covers other test helpers that label their
// containers with it (org.bothy.test=seaweedfs) but have no watchdog.
func reapLegacy(ctx context.Context, key string, now time.Time) error {
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()
	res, err := cli.ContainerList(ctx, client.ContainerListOptions{
		All: true, Filters: client.Filters{}.Add("label", key),
	})
	if err != nil {
		return err
	}
	var problems []error
	for _, ctr := range res.Items {
		if _, ok := ctr.Labels[watchdogLabel]; ok || now.Sub(time.Unix(ctr.Created, 0)) < legacyReapAge {
			continue
		}
		if _, err := cli.ContainerRemove(ctx, ctr.ID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
			problems = append(problems, fmt.Errorf("remove %s: %w", ctr.ID, err))
		}
	}
	return errors.Join(problems...)
}

// runContainer starts the pinned image on 127.0.0.1 with a random host port, under
// watchdogScript and with AutoRemove, so the container and its anonymous data volume disappear
// when the lease of this process ends. The server flags trade durability for speed, which a
// disposable test database does not need.
func runContainer(ctx context.Context, superPassword string) (*testcontainers.DockerContainer, error) {
	port := network.MustParsePort(containerPort)
	return testcontainers.Run(ctx, Image,
		testcontainers.WithLogger(tclog.NewNoopLogger()),
		testcontainers.WithName("bothy-pgtest-"+randomHex(6)),
		testcontainers.WithLabels(map[string]string{label: labelValue, watchdogLabel: watchdogLabelValue}),
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER": superUser, "POSTGRES_PASSWORD": superPassword, "POSTGRES_DB": superDatabase,
		}),
		testcontainers.WithEnv(watchdogEnv(leaseMisses, leaseStartGrace, leaseStopWait)),
		testcontainers.WithEntrypoint("bash", "-c", watchdogScript, "pgtest-watchdog"),
		testcontainers.WithCmd("postgres", "-c", "fsync=off", "-c", "synchronous_commit=off", "-c", "full_page_writes=off"),
		testcontainers.WithExposedPorts(containerPort),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{port: {{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "0"}}}
			hc.AutoRemove = true
		}),
		// The image's entry point starts the server once to initialise the data directory and
		// then for good; only the second line means ready.
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(startTimeout)),
	)
}

// writePasswords generates the superuser and role passwords and writes them as files of mode
// 0600 into dir, under the names of the secret catalog.
func writePasswords(dir string) (string, map[pg.Role]secrets.Value, error) {
	write := func(name, value string) error {
		return os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o600)
	}
	super := examplePassword()
	if err := write(superSecretFile, super); err != nil {
		return "", nil, fmt.Errorf("pgtest: write %s: %w", superSecretFile, err)
	}
	passwords := map[pg.Role]secrets.Value{}
	for _, r := range pg.Roles() {
		pw := examplePassword()
		if err := write(secretFiles[r], pw); err != nil {
			return "", nil, fmt.Errorf("pgtest: write %s: %w", secretFiles[r], err)
		}
		passwords[r] = secrets.NewValue([]byte(pw))
	}
	return super, passwords, nil
}

// examplePassword returns a generated password marked EXAMPLE (white-label rule for example
// credentials).
func examplePassword() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b) // never fails (crypto/rand)
	return "pgtest-EXAMPLE-" + base64.RawURLEncoding.EncodeToString(b)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // never fails (crypto/rand)
	return hex.EncodeToString(b)
}

// superConfig is the configuration of a superuser connection to database.
func (c *cluster) superConfig(database string) pg.Config {
	cfg := c.config(database, superUser, "file:"+filepath.Join(c.secretsDir, superSecretFile))
	return cfg
}

// config is the configuration of a role on database with the test deadlines.
func (c *cluster) config(database, user, passwordRef string) pg.Config {
	return pg.Config{
		Host: c.host, Port: c.port, Name: database, User: user, PasswordRef: passwordRef,
		SSLMode: pg.SSLModeDisable, MaxConns: testMaxConns, AcquireTimeout: testAcquireTimeout,
		StatementTimeout: testStatementTimeout, LockTimeout: testLockTimeout, IdleInTxTimeout: testIdleInTxTimeout,
	}.WithAppName("bothy-test")
}

// superConn opens a superuser connection to database.
func (c *cluster) superConn(ctx context.Context, database string) (*pgx.Conn, error) {
	conn, err := pg.Connect(ctx, c.superConfig(database), c.provider, connectWait, nil)
	if err != nil {
		return nil, fmt.Errorf("pgtest: superuser connection to %s: %w", database, err)
	}
	return conn, nil
}

// Option selects the schema of the database New creates.
type Option func(*options)

type options struct {
	fsys fs.FS // nil: no migrations
}

// WithMigrations makes New clone a template migrated with fsys instead of the server's
// migrations (schema tests: an older or a newer schema, a broken migration).
func WithMigrations(fsys fs.FS) Option { return func(o *options) { o.fsys = fsys } }

// WithoutMigrations makes New create a bootstrapped database without any migration, as a fresh
// installation has it before the first bothy migrate up.
func WithoutMigrations() Option { return func(o *options) { o.fsys = nil } }

// DB is one database of a test, dropped when the test ends.
type DB struct {
	name string
	c    *cluster
}

// New returns a fresh database for t: a clone of the template migrated with the server's
// migrations (package migrations), or with the set of WithMigrations, or a bootstrapped database
// without migrations (WithoutMigrations). Its owner is bothy_owner, and it carries the database
// and schema grants of pg.BootstrapDatabase. t.Cleanup drops it, also when the test dropped it
// already. New fails t when Main is not in use or the container cannot start.
func New(t testing.TB, opts ...Option) *DB {
	t.Helper()
	o := options{fsys: migrations.FS}
	for _, opt := range opts {
		opt(&o)
	}
	c, err := sharedCluster()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
	defer cancel()
	name := "t_" + randomHex(8)
	if err := c.createDatabase(ctx, name, o.fsys); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		defer cancel()
		if err := c.dropDatabase(ctx, name); err != nil {
			t.Errorf("pgtest: drop database %s: %v", name, err)
		}
	})
	return &DB{name: name, c: c}
}

// createDatabase creates name from the template of fsys (none for nil) and bootstraps it.
func (c *cluster) createDatabase(ctx context.Context, name string, fsys fs.FS) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	template := ""
	if fsys != nil {
		var err error
		if template, err = c.template(ctx, fsys); err != nil {
			return err
		}
	}
	return c.create(ctx, name, template, nil)
}

// template returns the template database migrated with fsys, creating it on first use. The
// caller holds c.mu.
func (c *cluster) template(ctx context.Context, fsys fs.FS) (string, error) {
	key, err := hashFS(fsys)
	if err != nil {
		return "", err
	}
	if name, ok := c.templates[key]; ok {
		return name, nil
	}
	name := "tpl_" + key[:16]
	if err := c.create(ctx, name, "", fsys); err != nil {
		return "", err
	}
	c.templates[key] = name
	return name, nil
}

// create runs CREATE DATABASE name (from template when set) OWNER bothy_owner and
// pg.BootstrapDatabase in it; with fsys it then migrates the database as bothy_owner and closes
// it for connections, so it can serve as a template. When a step after CREATE DATABASE fails, it
// drops the database again, so a retry with the same name reports the real error.
func (c *cluster) create(ctx context.Context, name, template string, fsys fs.FS) (err error) {
	super, err := c.superConn(ctx, superDatabase)
	if err != nil {
		return err
	}
	defer func() { _ = super.Close(context.Background()) }()
	stmt := "CREATE DATABASE " + pgx.Identifier{name}.Sanitize() + " OWNER " + pgx.Identifier{string(pg.RoleOwner)}.Sanitize()
	if template != "" {
		stmt += " TEMPLATE " + pgx.Identifier{template}.Sanitize()
	}
	if _, err := super.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("pgtest: create database %s: %w", name, err)
	}
	defer func() {
		if err != nil {
			dctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
			defer cancel()
			_ = c.dropDatabase(dctx, name)
		}
	}()
	conn, err := c.superConn(ctx, name)
	if err != nil {
		return err
	}
	err = pg.BootstrapDatabase(ctx, conn, name)
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	_ = conn.Close(cctx)
	cancel()
	if err != nil {
		return fmt.Errorf("pgtest: %w", err)
	}
	if fsys == nil {
		return nil
	}
	if err := c.migrate(ctx, name, fsys); err != nil {
		return err
	}
	closeTemplate := "ALTER DATABASE " + pgx.Identifier{name}.Sanitize() + " WITH ALLOW_CONNECTIONS false"
	if _, err := super.Exec(ctx, closeTemplate); err != nil {
		return fmt.Errorf("pgtest: close template %s: %w", name, err)
	}
	return nil
}

// migrate applies fsys to database as bothy_owner through pg.Migrator, the path of bothy
// migrate up.
func (c *cluster) migrate(ctx context.Context, database string, fsys fs.FS) error {
	cfg := c.config(database, string(pg.RoleOwner), c.ref(pg.RoleOwner)).ForMigrate(testLockTimeout)
	pool, err := pg.Open(ctx, cfg, c.provider, nil)
	if err != nil {
		return fmt.Errorf("pgtest: %w", err)
	}
	defer pool.Close()
	db := pool.OpenSQL()
	defer func() { _ = db.Close() }()
	var steps pg.MigratorOptions
	if c.steps != nil {
		if steps, err = c.steps(pool, fsys); err != nil {
			return fmt.Errorf("pgtest: migration steps: %w", err)
		}
	}
	m, err := pg.NewMigrator(db, fsys, steps)
	if err != nil {
		return fmt.Errorf("pgtest: %w", err)
	}
	if _, err := m.Up(ctx); err != nil {
		return fmt.Errorf("pgtest: migrate template %s: %w", database, err)
	}
	return nil
}

// dropDatabase drops name, ending its sessions; a database the test dropped already is fine.
func (c *cluster) dropDatabase(ctx context.Context, name string) error {
	super, err := c.superConn(ctx, superDatabase)
	if err != nil {
		return err
	}
	defer func() { _ = super.Close(context.Background()) }()
	drop := "DROP DATABASE IF EXISTS " + pgx.Identifier{name}.Sanitize() + " WITH (FORCE)"
	_, err = super.Exec(ctx, drop)
	return err
}

// ref is the secret reference of the password file of role.
func (c *cluster) ref(role pg.Role) string {
	return "file:" + filepath.Join(c.secretsDir, secretFiles[role])
}

// hashFS identifies a migration set by the names and contents of its *.sql files.
func hashFS(fsys fs.FS) (string, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return "", fmt.Errorf("pgtest: list migrations: %w", err)
	}
	slices.Sort(names)
	h := sha256.New()
	for _, n := range names {
		b, err := fs.ReadFile(fsys, n)
		if err != nil {
			return "", fmt.Errorf("pgtest: read %s: %w", n, err)
		}
		_, _ = io.WriteString(h, n+"\x00"+strconv.Itoa(len(b))+"\x00")
		_, _ = h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Name returns the name of the database.
func (d *DB) Name() string { return d.name }

// Config returns the connection configuration of role for this database: host 127.0.0.1, the
// mapped port, sslmode disable, the password as a file: reference into SecretsDir, and generous
// test deadlines (pool of 4, acquire 10s, statement 30s, lock 10s, idle in transaction 60s).
// Tests of a deadline change the fields they test.
func (d *DB) Config(role pg.Role) pg.Config {
	return d.c.config(d.name, string(role), d.c.ref(role))
}

// SecretsDir returns the directory with the password files db_owner_password,
// db_app_password, db_maint_password, db_monitor_password and db_superuser_password (mode
// 0600), so tests resolve passwords through a real secrets.Provider. The roles are cluster-wide:
// a test that changes a password must not run in parallel and must restore it.
func (d *DB) SecretsDir() string { return d.c.secretsDir }

// Pool opens a pool of role on this database (Config) through a secrets.Provider over
// SecretsDir and closes it when the test ends.
func (d *DB) Pool(t testing.TB, role pg.Role) *pg.Pool {
	t.Helper()
	return d.OpenPool(t, d.Config(role))
}

// OpenPool opens a pool for cfg, usually a Config of this database with changed fields, through
// a secrets.Provider over SecretsDir, and closes it when the test ends.
func (d *DB) OpenPool(t testing.TB, cfg pg.Config) *pg.Pool {
	t.Helper()
	p, err := pg.Open(context.Background(), cfg, d.c.provider, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// SuperConn opens a superuser connection to this database and closes it when the test ends.
func (d *DB) SuperConn(t testing.TB) *pgx.Conn {
	t.Helper()
	conn, err := d.c.superConn(context.Background(), d.name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

// SuperConfig returns the connection configuration of the superuser for this database, the
// login bothy migrate up --bootstrap uses (migrate.bootstrap_user).
func (d *DB) SuperConfig() pg.Config { return d.c.superConfig(d.name) }

// ContainerLogs returns the server log of the container so far (stdout and stderr).
func (d *DB) ContainerLogs(t testing.TB) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	r, err := d.c.ctr.Logs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
