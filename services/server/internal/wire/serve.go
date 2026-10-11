// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/migrations"
)

// Log messages of the start; tests read the bound ports from the listener lines.
const (
	msgInternalListener = "internal listener started"
	msgPublicListener   = "public listener started"
	msgReady            = "server ready"
)

// ServeConfig holds the keys the start and the shutdown read (02 "Start" and "Shutdown",
// 16 "Konfigurationswerte dieses Kapitels").
type ServeConfig struct {
	// ListenAddr is server.listen_addr, the public listener (api and all).
	ListenAddr string `config:"server.listen_addr"`
	// InternalListenAddr is server.internal_listen_addr, the listener of /healthz and /readyz.
	InternalListenAddr string `config:"server.internal_listen_addr"`
	// DependencyWait is startup.dependency_wait, how long the start waits for the database.
	DependencyWait time.Duration `config:"startup.dependency_wait"`
	// DrainDelay is shutdown.drain_delay.
	DrainDelay time.Duration `config:"shutdown.drain_delay"`
	// HTTPGrace is shutdown.http_grace.
	HTTPGrace time.Duration `config:"shutdown.http_grace"`
	// TelemetryFlushTimeout is shutdown.telemetry_flush_timeout.
	TelemetryFlushTimeout time.Duration `config:"shutdown.telemetry_flush_timeout"`
	// TotalTimeout is shutdown.total_timeout.
	TotalTimeout time.Duration `config:"shutdown.total_timeout"`
}

// Deps are the process dependencies of Serve that do not come from the configuration.
type Deps struct {
	// Env is the process environment as cmd/bothy read it ("NAME=value" entries); the secret
	// provider and the telemetry read it.
	Env []string
	// Stdout receives the JSON log lines; nil means os.Stdout.
	Stdout io.Writer
	// Clock measures the shutdown budget; nil means clock.System().
	Clock clock.Clock
	// Signals replaces os/signal (tests); nil means SIGTERM and SIGINT.
	Signals <-chan os.Signal
	// Version is service.version, buildinfo.Version() in the binary.
	Version string
	// Migrations is the migration set whose highest version the database must have; nil means
	// the embedded migrations.FS.
	Migrations fs.FS
	// Handlers is the composite of the module handlers that MountRoutes mounts (M1-17).
	Handlers Handlers
}

// server is the state of one serve process that the components fill and the stop steps read.
// Each field is written by one component during the start and read by stop steps afterwards;
// the mutex orders the two, because a step may run when its component never started.
type server struct {
	mu       sync.Mutex
	runner   *lifecycle.Runner
	log      *switchLogger
	boot     *slog.Logger
	cfg      ServeConfig
	httpCfg  httpserver.Config
	role     Role
	values   *config.Values
	deps     Deps
	sp       *secrets.Provider
	reg      *health.Registry
	tel      *otel.Telemetry
	pool     *pg.Pool
	internal *http.Server
	public   *http.Server
	mux      *http.ServeMux
	required int64
}

// Serve assembles `bothy serve` for role from the loaded values v and returns the Runner;
// cmd/bothy exits with Runner.Run. It starts nothing itself. The components start in the order
// of docs/spec/02-architektur.md "Start":
//
//  1. telemetry (otel.Setup); the log lines switch from stdout JSON to the server logger,
//  2. secrets: the provider resolves database.password_ref once, so a missing secret fails at
//     once instead of after startup.dependency_wait,
//  3. internal listener with /healthz and /readyz (readiness fails with startup until ready),
//  4. database: pg.Open and WaitReady within startup.dependency_wait; readiness check database,
//  5. schema: pg.CheckSchema against the highest version of deps.Migrations; an older database
//     ends the start with the hint to run "bothy migrate up" (Serve never migrates, R-AR-16),
//  6. extension points, in this order: instance (M1-15), signing keys (M1-32), routes
//     (MountRoutes, api and all), log tail (M1-16, api and all), jobs and consumers (M1-14,
//     worker and all),
//  7. public listener (api and all),
//  8. ready: readiness passes startup and the line "server ready" is written.
//
// Stop steps, by phase: not ready (always), drain and HTTP (api, all), telemetry, database,
// internal listener. Their budgets come from shutdown.*.
//
// The error reports values that do not bind (a schema without a key) or an unknown role; it is
// a usage or configuration error (exit code 2). Budgets above shutdown.total_timeout make Run
// return lifecycle.ExitUsage.
func Serve(_ context.Context, v *config.Values, role Role, deps Deps) (*lifecycle.Runner, error) {
	switch role {
	case RoleAPI, RoleWorker, RoleAll:
	default:
		return nil, fmt.Errorf("wire: unknown role %q (want %s, %s or %s)", role, RoleAPI, RoleWorker, RoleAll)
	}
	s := &server{role: role, values: v, deps: deps, reg: health.NewRegistry()}
	if err := s.bind(); err != nil {
		return nil, err
	}
	if deps.Stdout == nil {
		s.deps.Stdout = os.Stdout
	}
	level, err := otel.ParseLevel(v.String("telemetry.logs.level"))
	if err != nil {
		return nil, err
	}
	s.boot = otel.NewCommandLogger(s.deps.Stdout, level)
	s.log = newSwitchLogger(s.boot)
	logger := slog.New(s.log)
	s.sp, err = NewSecretsProvider(v, deps.Env, logger)
	if err != nil {
		return nil, err
	}
	fsys := deps.Migrations
	if fsys == nil {
		fsys = migrations.FS
	}
	if s.required, err = pg.RequiredSchemaVersion(fsys); err != nil {
		return nil, err
	}
	s.runner = lifecycle.New(lifecycle.Options{
		Clock: deps.Clock, Logger: logger, TotalTimeout: s.cfg.TotalTimeout, Signals: deps.Signals,
	})
	s.register()
	return s.runner, nil
}

// bind reads the typed configuration of the start and of the listeners.
func (s *server) bind() error {
	if err := config.Bind(s.values, &s.cfg); err != nil {
		return err
	}
	return config.Bind(s.values, &s.httpCfg)
}

// api reports whether the role serves the public API.
func (s *server) api() bool { return s.role == RoleAPI || s.role == RoleAll }

// logger returns the current process logger.
func (s *server) logger() *slog.Logger { return slog.New(s.log) }

// register adds the components and stop steps in the order Serve documents.
func (s *server) register() {
	r := s.runner
	r.Add(step("telemetry", s.startTelemetry))
	r.Add(step("secrets", s.startSecrets))
	r.Add(step("internal_listener", s.startInternal))
	r.Add(step("database", s.startDatabase))
	r.Add(step("schema", s.checkSchema))
	// Extension points (README.md): M1-15 adds the instance here, M1-32 the signing keys.
	r.Add(step("routes", s.mountRoutes))
	// Extension points: M1-16 adds the log tail (api, all), M1-14 the jobs and M2 the consumers
	// (worker, all), each with its stop step.
	if s.api() {
		r.Add(step("public_listener", s.startPublic))
	}
	r.Add(step("ready", s.ready))

	r.OnStop(lifecycle.StopStep{Name: "readiness", Phase: lifecycle.PhaseNotReady, Run: func(context.Context) error {
		s.reg.SetShuttingDown()
		return nil
	}})
	if s.api() {
		r.OnStop(lifecycle.StopStep{Name: "drain", Phase: lifecycle.PhaseDrain, Budget: s.cfg.DrainDelay, Run: s.drain})
		r.OnStop(lifecycle.StopStep{Name: "public_listener", Phase: lifecycle.PhaseHTTP, Budget: s.cfg.HTTPGrace, Run: s.stopPublic})
	}
	r.OnStop(lifecycle.StopStep{Name: "telemetry", Phase: lifecycle.PhaseTelemetry, Budget: s.cfg.TelemetryFlushTimeout, Run: s.stopTelemetry})
	r.OnStop(lifecycle.StopStep{Name: "pool", Phase: lifecycle.PhaseDatabase, Run: s.stopDatabase})
	r.OnStop(lifecycle.StopStep{Name: "internal_listener", Phase: lifecycle.PhaseInternalListener, Run: s.stopInternal})
}

// component adapts a start function to lifecycle.Component.
type component struct {
	name  string
	start func(ctx context.Context) error
}

func (c component) Name() string                    { return c.name }
func (c component) Start(ctx context.Context) error { return c.start(ctx) }

func step(name string, start func(ctx context.Context) error) lifecycle.Component {
	return component{name: name, start: start}
}

func (s *server) startTelemetry(ctx context.Context) error {
	cfg, err := TelemetryConfig(s.values)
	if err != nil {
		return err
	}
	tel, err := otel.Setup(ctx, cfg, otel.Deps{
		Secrets: s.sp, Clock: s.deps.Clock, Stdout: s.deps.Stdout, ServiceVersion: s.deps.Version, Env: s.deps.Env,
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.tel = tel
	s.mu.Unlock()
	s.log.set(tel.Logger())
	return nil
}

func (s *server) stopTelemetry(ctx context.Context) error {
	s.mu.Lock()
	tel := s.tel
	s.mu.Unlock()
	if tel == nil {
		return nil
	}
	// The last lines (pool, internal listener, budget) go to stdout from now on, also when the
	// flush outlives its budget and the plan moves on without waiting for it.
	s.log.set(s.boot)
	return tel.Shutdown(ctx)
}

// startSecrets resolves the database password once: the secret provider fails fast (02
// "Ausfall einer Abhängigkeit"), and a missing file is reported before the database wait.
func (s *server) startSecrets(ctx context.Context) error {
	db, err := DatabaseConfig(s.values, "bothy-"+string(s.role))
	if err != nil {
		return err
	}
	_, _, err = s.sp.Resolve(ctx, db.PasswordRef)
	return err
}

func (s *server) startInternal(ctx context.Context) error {
	srv, host, port, err := s.listen(ctx, s.cfg.InternalListenAddr, httpserver.Internal(s.reg), "internal listener")
	if err != nil {
		return err
	}
	s.logger().InfoContext(ctx, msgInternalListener, slog.String(attr.ServerAddress, host), slog.Int(attr.ServerPort, port))
	s.mu.Lock()
	s.internal = srv
	s.mu.Unlock()
	return nil
}

func (s *server) startPublic(ctx context.Context) error {
	s.mu.Lock()
	mux := s.publicMux()
	s.mu.Unlock()
	srv, host, port, err := s.listen(ctx, s.cfg.ListenAddr, httpserver.Public(mux), "public listener")
	if err != nil {
		return err
	}
	s.logger().InfoContext(ctx, msgPublicListener, slog.String(attr.ServerAddress, host), slog.Int(attr.ServerPort, port))
	s.mu.Lock()
	s.public = srv
	s.mu.Unlock()
	return nil
}

// publicMux returns the mux of the public listener; the caller holds s.mu.
func (s *server) publicMux() *http.ServeMux {
	if s.mux == nil {
		s.mux = http.NewServeMux()
	}
	return s.mux
}

// listen opens addr, serves h on it and returns the server with the bound host and port (the
// caller logs them; port 0 in the configuration becomes the real port here). A serve error
// other than the orderly close is a fatal runtime error (Runner.Fail).
func (s *server) listen(ctx context.Context, addr string, h http.Handler, what string) (*http.Server, string, int, error) {
	ln, err := httpserver.Listen(ctx, addr)
	if err != nil {
		return nil, "", 0, err
	}
	srv := httpserver.NewServer(s.httpCfg, h, s.logger())
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.runner.Fail(fmt.Errorf("%s: %w", what, err))
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(port)
	return srv, host, p, nil
}

func (s *server) startDatabase(ctx context.Context) error {
	db, err := DatabaseConfig(s.values, "bothy-"+string(s.role))
	if err != nil {
		return err
	}
	pool, err := pg.Open(ctx, db, s.sp, s.logger())
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.pool = pool
	s.mu.Unlock()
	if err := pool.WaitReady(ctx, s.cfg.DependencyWait); err != nil {
		return err
	}
	s.reg.AddReadiness("database", pool.Ping)
	return nil
}

func (s *server) checkSchema(ctx context.Context) error {
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	return pg.CheckSchema(ctx, pool, s.required)
}

func (s *server) mountRoutes(context.Context) error {
	s.mu.Lock()
	mux := s.publicMux()
	s.mu.Unlock()
	return MountRoutes(s.role, mux, s.deps.Handlers, routes.SpecCatalog(), s.logger())
}

func (s *server) ready(ctx context.Context) error {
	s.reg.SetStarted()
	// bothy.server.role is a resource attribute: OTLP carries it on the resource, the stdout
	// handler drops it like every key outside the span and log scope (docs/DECISIONS.md M1-11).
	s.logger().InfoContext(ctx, msgReady, slog.String(attr.ServerRole, string(s.role)))
	return nil
}

// drain waits shutdown.drain_delay after readiness turned false, so the proxy stops sending
// new requests; it ends early when its budget does, and at once when the public listener never
// opened.
func (s *server) drain(ctx context.Context) error {
	s.mu.Lock()
	public := s.public
	s.mu.Unlock()
	if public == nil {
		// The public listener never opened (the start ended before it): nothing to drain.
		return nil
	}
	t := time.NewTimer(s.cfg.DrainDelay)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *server) stopPublic(ctx context.Context) error {
	s.mu.Lock()
	srv := s.public
	s.mu.Unlock()
	return shutdownServer(ctx, srv)
}

func (s *server) stopInternal(ctx context.Context) error {
	s.mu.Lock()
	srv := s.internal
	s.mu.Unlock()
	return shutdownServer(ctx, srv)
}

// shutdownServer closes the listener of srv and waits for running requests within ctx; when
// ctx ends first, the remaining connections are closed. Hijacked connections are left to
// PhaseConnections (M1-65).
func shutdownServer(ctx context.Context, srv *http.Server) error {
	if srv == nil {
		return nil
	}
	if err := srv.Shutdown(ctx); err != nil {
		return errors.Join(err, srv.Close())
	}
	return nil
}

func (s *server) stopDatabase(context.Context) error {
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	if pool != nil {
		pool.Close()
	}
	return nil
}
