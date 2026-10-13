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
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
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
	msgStartedNotReady  = "server started, not ready"
)

// ServeConfig holds the keys the start and the shutdown read (02 "Start" and "Shutdown",
// 16 "Configuration values of this chapter").
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
	// Version is service.version and the version the start compares with
	// instance.min_server_version; empty means buildinfo.Version().
	Version string
	// Migrations is the migration set whose highest version the database must have; nil means
	// the embedded migrations.FS.
	Migrations fs.FS
	// Handlers is the composite of the module handlers that MountRoutes mounts (M1-17).
	Handlers Handlers
	// Registrations are the Register functions whose event types, gateway ops and terminal
	// channel ops the start checks against the catalogue (M1-25); nil means the functions of the
	// module ports. Tests inject a bad registration here.
	Registrations []func(*authorize.Registry)
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
	chainCfg httpserver.ChainConfig
	chain    *httpserver.Chain
	// transport and sec are the transport protection of the public listener and the SEC-HDR-1
	// headers of both listeners (M1-21), derived from the configuration in bind.
	transport httpserver.TransportConfig
	sec       httpserver.SecurityHeaders
	role      Role
	values    *config.Values
	deps      Deps
	sp        *secrets.Provider
	reg       *health.Registry
	tel       *otel.Telemetry
	pool      *pg.Pool
	internal  *http.Server
	public    *http.Server
	mux       *http.ServeMux
	required  int64
	instance  instance.Record
	jobsCfg   jobs.Config
	jobsReg   *jobs.Registry
	jobsRT    *jobs.Runtime
	// audit is the audit module (M1-24); later tickets hand its recorder to the modules.
	audit *Audit
	// authorizer is the kernel/authorize.Authorizer of the process (M1-28); the use cases of later
	// tickets receive it.
	authorizer authorize.Authorizer
	// blocked is set when a readiness check fails for good (schema_requires_newer_server).
	blocked bool
}

// Serve assembles `bothy serve` for role from the loaded values v and returns the Runner;
// cmd/bothy exits with Runner.Run. It starts nothing itself. The components start in the order
// of docs/spec/02-architecture.md "Start":
//
//  1. telemetry (otel.Setup); the log lines switch from stdout JSON to the server logger,
//  2. secrets: the provider resolves database.password_ref once, so a missing secret fails at
//     once instead of after startup.dependency_wait; api and all also load and check the TLS
//     certificate and key when server.tls_cert_ref and server.tls_key_ref are set (M1-21),
//  3. internal listener with /healthz and /readyz (readiness fails with startup until ready),
//     plaintext without a peer filter (R-SE-09),
//  4. database: pg.Open and WaitReady within startup.dependency_wait; readiness check database,
//  5. schema: pg.CheckSchema against the highest version of deps.Migrations; an older database
//     ends the start with the hint to run "bothy migrate up" (Serve never migrates, R-AR-16),
//  6. instance (M1-15): load the instance row (missing: the start ends with the hint to run
//     "bothy migrate up") and compare the own version with instance.min_server_version; below
//     it the readiness check schema_requires_newer_server fails for good (loadInstance),
//     6a. audit (M1-24): resolve privacy.audit_ip_kek_ref once (a missing or malformed key ends the
//     start), build the audit module with the unit of work of the role and start the aggregator
//     of denials; its stop step audit_denials flushes the open aggregations before the pool
//     closes (startAudit),
//     6b. authz (M1-28): build the Authorizer with the recorder of audit (startAuthz),
//  7. extension points, in this order: signing keys (M1-32), routes (the catalogue check of
//     route bindings and registrations for every role, M1-25, then InstallRateLimits with its
//     sweeper, M1-19, and MountRoutes, api and all), log
//     tail (M1-16, api and all), then the jobs (every role): the River migrations in jobs.schema
//     must be complete (else the hint to run "bothy migrate up"), the runtime of the role is
//     built over the pool, and worker and all start River (02 "Start and graceful shutdown" step 8); a River client
//     that stops on its own later ends the process with exit code 1 (02 "Health"). The
//     consumers (M2) follow,
//  8. public listener (api and all) through httpserver.ListenPublic: own TLS, or plaintext only
//     from loopback and server.trusted_proxies (SEC-TLS-1, SEC-TLS-2),
//  9. ready: readiness passes startup and the line "server ready" is written.
//
// Stop steps, by phase: not ready (always), drain and HTTP (api, all), jobs (worker, all: River's
// soft stop within shutdown.jobs_soft_timeout, then what the fixed budgets leave of the total
// for the cancelled jobs to end), telemetry, database (the flush of the audit denials, then the
// pool), internal listener. Their budgets come from shutdown.*.
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
	if deps.Version == "" {
		s.deps.Version = buildinfo.Version()
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
	if s.jobsCfg, err = JobsConfig(v); err != nil {
		return nil, err
	}
	s.jobsCfg.Logger = logger
	if s.jobsReg, err = NewJobRegistry(&Modules{}); err != nil {
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
	if err := config.Bind(s.values, &s.chainCfg); err != nil {
		return err
	}
	if err := config.Bind(s.values, &s.httpCfg); err != nil {
		return err
	}
	t, err := TransportConfig(s.values, s.chainCfg)
	if err != nil {
		return err
	}
	s.transport = t
	s.sec = httpserver.SecurityHeadersFor(t.PublicURL, s.chainCfg.HSTSMaxAge)
	return nil
}

// tlsKeys are the keys of the TLS references and the directory their placeholder names.
type tlsKeys struct {
	CertRef    string `config:"server.tls_cert_ref"`
	KeyRef     string `config:"server.tls_key_ref"`
	SecretsDir string `config:"secrets.dir"`
}

// TransportConfig returns the transport protection of the public listener (M1-21) from v and the
// bound chain configuration: server.listen_addr, server.public_url, server.trusted_proxies (the
// slice of chain, so the peer filter and the client IP read one list, design note 8) and the TLS
// references, the empty text being the zero secrets.Ref (plaintext). The error names the key of
// a reference that does not parse, never its text; the loader has already checked the grammar,
// so it means a schema without these keys or a hand-built Values.
func TransportConfig(v *config.Values, chain httpserver.ChainConfig) (httpserver.TransportConfig, error) {
	var keys tlsKeys
	if err := config.Bind(v, &keys); err != nil {
		return httpserver.TransportConfig{}, err
	}
	public, err := url.Parse(chain.PublicURL)
	if err != nil {
		return httpserver.TransportConfig{}, errors.New("wire: server.public_url is not a URL")
	}
	t := httpserver.TransportConfig{
		ListenAddr: v.String("server.listen_addr"), PublicURL: public, TrustedProxies: chain.TrustedProxies,
	}
	for _, r := range []struct {
		key, raw string
		ref      *secrets.Ref
	}{{keyTLSCertRef, keys.CertRef, &t.TLSCertRef}, {keyTLSKeyRef, keys.KeyRef, &t.TLSKeyRef}} {
		if r.raw == "" {
			continue
		}
		ref, err := secrets.ParseRef(r.raw, keys.SecretsDir)
		if err != nil {
			return httpserver.TransportConfig{}, fmt.Errorf("wire: %s: %w", r.key, err)
		}
		*r.ref = ref
	}
	return t, nil
}

// api reports whether the role serves the public API.
func (s *server) api() bool { return s.role == RoleAPI || s.role == RoleAll }

// worker reports whether the role runs jobs.
func (s *server) worker() bool { return s.role == RoleWorker || s.role == RoleAll }

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
	r.Add(step("instance", s.loadInstance))
	r.Add(step("audit", s.startAudit))
	r.Add(step("authz", s.startAuthz))
	// Extension point (README.md): M1-32 adds the signing keys here.
	r.Add(step("routes", s.mountRoutes))
	// Extension point: M1-16 adds the log tail (api, all) here, M2 the consumers after the jobs.
	r.Add(step("jobs", s.startJobs))
	if s.api() {
		r.Add(step("public_listener", s.startPublic))
	}
	r.Add(step("ready", s.ready))

	r.OnStop(lifecycle.StopStep{Name: "readiness", Phase: lifecycle.PhaseNotReady, Run: func(context.Context) error {
		s.reg.SetShuttingDown()
		return nil
	}})
	if s.api() {
		r.OnStop(lifecycle.StopStep{Name: "drain", Phase: lifecycle.PhaseDrain, Budget: s.cfg.DrainDelay, Wait: true, Run: s.drain})
		r.OnStop(lifecycle.StopStep{Name: "public_listener", Phase: lifecycle.PhaseHTTP, Budget: s.cfg.HTTPGrace, Run: s.stopPublic})
	}
	for _, st := range s.jobsStopSteps() {
		r.OnStop(st)
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
// "Failure of a dependency"), and a missing file is reported before the database wait. For the
// roles with a public listener it also loads and checks the TLS material when the server
// terminates TLS itself (M1-21), so a broken pair ends the start here too.
func (s *server) startSecrets(ctx context.Context) error {
	db, err := DatabaseConfig(s.values, "bothy-"+string(s.role))
	if err != nil {
		return err
	}
	if _, _, err = s.sp.Resolve(ctx, db.PasswordRef); err != nil {
		return err
	}
	if !s.api() {
		// The worker opens no public listener and does not read the certificate.
		return nil
	}
	return httpserver.CheckTransport(ctx, s.transport, s.sp)
}

func (s *server) startInternal(ctx context.Context) error {
	// The internal listener stays plaintext without a peer filter (R-SE-09).
	ln, err := httpserver.Listen(ctx, s.cfg.InternalListenAddr)
	if err != nil {
		return err
	}
	srv, host, port := s.serve(ln, httpserver.InternalHandler(httpserver.Internal(s.reg), s.sec), "internal listener")
	s.logger().InfoContext(ctx, msgInternalListener, slog.String(attr.ServerAddress, host), slog.Int(attr.ServerPort, port))
	s.mu.Lock()
	s.internal = srv
	s.mu.Unlock()
	return nil
}

func (s *server) startPublic(ctx context.Context) error {
	s.mu.Lock()
	mux, chain := s.publicMux(), s.chain
	s.mu.Unlock()
	if chain == nil {
		return errors.New("wire: the public listener needs the HTTP chain of the routes step")
	}
	ln, err := httpserver.ListenPublic(ctx, s.transport, s.sp, s.logger())
	if err != nil {
		return err
	}
	srv, host, port := s.serve(ln, chain.Public(mux), "public listener")
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

// serve serves h on ln and returns the server with the bound host and port (the caller logs
// them; port 0 in the configuration becomes the real port here). A serve error other than the
// orderly close is a fatal runtime error (Runner.Fail).
func (s *server) serve(ln net.Listener, h http.Handler, what string) (*http.Server, string, int) {
	srv := httpserver.NewServer(s.httpCfg, h, s.logger())
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.runner.Fail(fmt.Errorf("%s: %w", what, err))
		}
	}()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(port)
	return srv, host, p
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

// startAudit is start step 6a: it checks the key-encryption key, builds the audit module over the
// pool of the role and starts its aggregator with the stop step that flushes it (Audit.Start;
// the pool step is registered earlier, so the flush runs before the pool closes).
func (s *server) startAudit(ctx context.Context) error {
	cfg, err := AuditConfig(s.values)
	if err != nil {
		return err
	}
	unitCfg, err := UnitConfig(s.values)
	if err != nil {
		return err
	}
	kek := SecretsKEK{Provider: s.sp}
	if err := checkAuditKEK(ctx, kek, cfg.IPKEKRef); err != nil {
		return err
	}
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	a, err := NewAudit(pool, unitCfg, cfg, kek, s.logger())
	if err != nil {
		return err
	}
	a.Start(s.runner)
	s.mu.Lock()
	s.audit = a
	s.mu.Unlock()
	return nil
}

// startAuthz is start step 6b (M1-28): it builds the Authorizer of authz over the pool with the
// recorder of the audit module (NewAuthorizer) and keeps it for the use cases of later tickets.
func (s *server) startAuthz(context.Context) error {
	cfg, err := AuthzConfig(s.values)
	if err != nil {
		return err
	}
	log := s.logger()
	cat, err := NewAuthzCatalog(log)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a := s.pool, s.audit
	s.mu.Unlock()
	az, err := NewAuthorizer(pool, cfg, cat, a.Recorder(), s.deps.Clock, log)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.authorizer = az
	s.mu.Unlock()
	return nil
}

// authorizerOf returns the Authorizer of the process, nil before the step authz ran.
func (s *server) authorizerOf() authorize.Authorizer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authorizer
}

func (s *server) checkSchema(ctx context.Context) error {
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	return pg.CheckSchema(ctx, pool, s.required)
}

// mountRoutes checks the route bindings and the registrations of the modules against the closed
// catalogue of authz (every role, M1-25), then builds the HTTP chain (api and all), installs the
// rate limiter with its start check and its sweeper (M1-19) and mounts the routes through it with
// the same catalogue. The request validator knows only the enabled operations, so the start
// compiles no schema of a route it does not serve. Until identity is wired (M1-34) the chain
// authenticates with actor.AnonymousOnly: every credential is refused
// (fail closed).
func (s *server) mountRoutes(context.Context) error {
	log := s.logger()
	cat, err := NewAuthzCatalog(log)
	if err != nil {
		return err
	}
	var mount func() error
	if s.api() {
		mount = func() error {
			chain, err := NewChain(s.chainCfg, log)
			if err != nil {
				return err
			}
			limiter, err := InstallRateLimits(chain, s.values, routes.Enabled(), s.deps.Clock)
			if err != nil {
				return err
			}
			s.runner.OnStop(startSweeper(limiter))
			s.mu.Lock()
			mux := s.publicMux()
			s.chain = chain
			s.mu.Unlock()
			return MountRoutes(s.role, mux, s.deps.Handlers, cat, chain, log)
		}
	}
	return startRoutes(RouteBindings(routes.Enabled()), BuildRegistry(s.deps.Registrations), cat, log, mount)
}

// NewChain builds the HTTP chain of the public listener for the enabled operations: the validator
// over the embedded contract (api/rest.Spec), actor.AnonymousOnly until identity is wired, and the
// global meter provider of platform/otel (call it after otel.Setup).
func NewChain(cfg httpserver.ChainConfig, log *slog.Logger) (*httpserver.Chain, error) {
	var ids []string
	for _, op := range routes.Enabled() {
		ids = append(ids, op.ID)
	}
	v, err := httpserver.NewValidator(rest.Spec, ids)
	if err != nil {
		return nil, err
	}
	return httpserver.NewChain(cfg, httpserver.Deps{Logger: log, Authenticator: actor.AnonymousOnly{}, Validator: v})
}

// jobsStopSteps returns the stop steps of phase jobs in registration order; none for api. Steps
// of one phase run in reverse registration order: the soft stop within
// shutdown.jobs_soft_timeout first, then jobs_wait for the cancelled jobs within what the fixed
// budgets leave of the total (jobsWaitBudget), so the telemetry flush keeps its own budget; no
// jobs_wait when nothing is left (docs/DECISIONS.md M1-14/4).
func (s *server) jobsStopSteps() []lifecycle.StopStep {
	if !s.worker() {
		return nil
	}
	var steps []lifecycle.StopStep
	if wait := jobsWaitBudget(s.cfg, s.role, s.jobsCfg.SoftStopTimeout); wait > 0 {
		steps = append(steps, lifecycle.StopStep{Name: "jobs_wait", Phase: lifecycle.PhaseJobs, Budget: wait, Run: s.stopJobs})
	}
	return append(steps, lifecycle.StopStep{Name: "jobs", Phase: lifecycle.PhaseJobs, Budget: s.jobsCfg.SoftStopTimeout, Run: s.stopJobs})
}

// jobsWaitBudget returns the budget of the stop step jobs_wait for role: shutdown.total_timeout
// minus the fixed budgets of the role's stop steps (drain and HTTP for all, the soft stop soft,
// the telemetry flush). 0 means there is no slack and the step is left out; a step budget of 0
// would take the whole rest of the total instead.
func jobsWaitBudget(cfg ServeConfig, role Role, soft time.Duration) time.Duration {
	fixed := soft + cfg.TelemetryFlushTimeout
	if role == RoleAPI || role == RoleAll {
		fixed += cfg.DrainDelay + cfg.HTTPGrace
	}
	return max(cfg.TotalTimeout-fixed, 0)
}

// startJobs checks River's migrations, builds the job runtime of the role and starts River for
// worker and all; a River client that later stops on its own ends the process.
func (s *server) startJobs(ctx context.Context) error {
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	if err := jobs.ValidateMigrations(ctx, pool, s.jobsCfg.Schema); err != nil {
		return err
	}
	rt, err := jobs.New(pool, s.jobsCfg, JobRole(s.role), s.jobsReg, nil)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.jobsRT = rt
	s.mu.Unlock()
	if err := rt.Start(ctx); err != nil {
		return err
	}
	s.runner.FailOn("jobs", rt.Failed())
	return nil
}

// stopJobs stops River (worker, all); it does nothing when the runtime never started.
func (s *server) stopJobs(ctx context.Context) error {
	s.mu.Lock()
	rt := s.jobsRT
	s.mu.Unlock()
	if rt == nil {
		return nil
	}
	return rt.Stop(ctx)
}

func (s *server) ready(ctx context.Context) error {
	s.reg.SetStarted()
	s.mu.Lock()
	blocked := s.blocked
	s.mu.Unlock()
	// bothy.server.role is a resource attribute: OTLP carries it on the resource, the stdout
	// handler drops it like every key outside the span and log scope (docs/DECISIONS.md M1-11).
	if blocked {
		// A check that fails for good keeps /readyz at 503; "server ready" would mislead.
		s.logger().WarnContext(ctx, msgStartedNotReady, slog.String(attr.ServerRole, string(s.role)))
		return nil
	}
	s.logger().InfoContext(ctx, msgReady, slog.String(attr.ServerRole, string(s.role)))
	return nil
}

// drain waits shutdown.drain_delay after readiness turned false, so the proxy stops sending
// new requests; its budget is the delay (a Wait step), so the end of its context is its normal
// end, also when the rest of the total cuts the delay short. It ends at once when the public
// listener never opened.
func (s *server) drain(ctx context.Context) error {
	s.mu.Lock()
	public := s.public
	s.mu.Unlock()
	if public == nil || s.cfg.DrainDelay <= 0 {
		// The public listener never opened (the start ended before it), or there is no delay:
		// nothing to drain. A zero budget would let the step wait for the rest of the total.
		return nil
	}
	<-ctx.Done()
	return nil
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
