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
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/platform/buildinfo"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/health"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/lifecycle"
	"github.com/OWNER/bothy/services/server/internal/platform/otel"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/otelmetrics"
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
	// ConsumerHandlers replaces the production list of worker consumers (consumerHandlers,
	// empty until M2-26 and M3-19) in worker and all; nil keeps it. Tests register probe
	// handlers here.
	ConsumerHandlers map[sync.ConsumerName]sync.ConsumerHandler
	// ConsumerGauges registers the lag gauges of the worker consumers in every role; nil means
	// otelmetrics.RegisterConsumerGauges on the global meter provider of otel.Setup. Tests read
	// the gauges of a role through their own meter provider here.
	ConsumerGauges func(consumers sync.Consumers) error
}

// server is the state of one serve process that the components fill and the stop steps read.
// Each field is written by one component during the start and read by stop steps afterwards;
// the mutex orders the two, because a step may run when its component never started.
type server struct {
	mu       stdsync.Mutex
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
	jobsRT    *jobs.Runtime
	// audit is the audit module (M1-24); later tickets hand its recorder to the modules.
	audit *Audit
	// authorizer is the kernel/authorize.Authorizer of the process (M1-28); the use cases of later
	// tickets receive it.
	authorizer authorize.Authorizer
	// syncCfg and sync are the event log (M1-16): its appender for the modules, the partition
	// maintenance of the job and the log tail of api and all.
	syncCfg SyncConfig
	sync    *Sync
	// identity is the identity module (M1-33): devices, the DPoP verifier and the REST handlers.
	identity *Identity
	// settings are the instance settings, branding and discovery (M1-46); identity reads
	// locale.default through them.
	settings *InstanceSettings
	// signingKeys are the server signing keys (M1-32) and keyCheck the interval of their job.
	signingKeys *SigningKeys
	keyCheck    time.Duration
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
//     6b. sync (M1-16): build the event log over the unit of work of the audit step (appender,
//     partition maintenance; startSync),
//     6c. authz (M1-28): build the Authorizer with the recorder of audit and bind the rights
//     hooks to the appender of sync and the recorder (M1-29; startAuthz),
//     6d. settings (M1-46): build the instance settings, branding and discovery over the unit of
//     work, the recorder, the appender and the Authorizer (startSettings),
//     6e. identity (M1-33): resolve auth.token_pepper_ref once (a missing or malformed pepper ends
//     the start), build the devices, the DPoP verifier with its jti store and the device handlers,
//     and start the sweeper of the jti store; its stop step runs in the HTTP phase (startIdentity),
//  7. extension points, in this order: signing keys (M1-32), routes (the catalogue check of
//     route bindings and registrations for every role, M1-25, then InstallRateLimits with its
//     sweeper, M1-19, InstallIdempotency, M1-20, and MountRoutes, api and all), log
//     tail (M1-16, api and all: local_head is set to the head before the public listener opens,
//     and a tail that fails later ends the process with exit code 1), then the jobs (every role,
//     with sync.partition_maintenance, sync.idempotency_cleanup, authz.expiry_sweep and
//     authz.expiry_reminder): the River migrations in jobs.schema
//     must be complete (else the hint to run "bothy migrate up"), the runtime of the role is
//     built over the pool, and worker and all start River (02 "Start and graceful shutdown" step 8); a River client
//     that stops on its own later ends the process with exit code 1 (02 "Health"). Then
//     worker and all start the worker consumers of the event log (M2-01): their cursor rows at
//     the head for every registered consumer, then the runner with its own LISTEN connection;
//     a runner that ends unexpectedly ends the process with exit code 1. Every role registers
//     the lag gauges of the consumers in step 6b,
//  8. public listener (api and all) through httpserver.ListenPublic: own TLS, or plaintext only
//     from loopback and server.trusted_proxies (SEC-TLS-1, SEC-TLS-2),
//  9. ready: readiness passes startup and the line "server ready" is written.
//
// Stop steps, by phase: not ready (always), drain and HTTP (api, all), event tail (api, all),
// consumers (worker, all: the running rounds finish, no new one starts), jobs (worker, all: River's
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
	if s.syncCfg, err = SyncConfigFrom(v); err != nil {
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

// components returns the start sequence of the role in the order Serve documents.
func (s *server) components() []lifecycle.Component {
	cs := []lifecycle.Component{
		step("telemetry", s.startTelemetry),
		step("secrets", s.startSecrets),
		step("internal_listener", s.startInternal),
		step("database", s.startDatabase),
		step("schema", s.checkSchema),
		step("instance", s.loadInstance),
		step("audit", s.startAudit),
		step("sync", s.startSync),
		step("authz", s.startAuthz),
		step("settings", s.startSettings),
		step("identity", s.startIdentity),
		step("signing_keys", s.startSigningKeys),
		step("audit_log", s.startAuditLog),
		step("sessions", s.startSessions),
		step("routes", s.mountRoutes),
	}
	if s.api() {
		cs = append(cs, step("log_tail", s.startLogTail))
	}
	cs = append(cs, step("jobs", s.startJobs))
	if s.worker() {
		// 02 "Start" step 8: the worker consumers start after River (M2-01).
		cs = append(cs, step("event_consumers", s.startEventConsumers))
	}
	if s.api() {
		cs = append(cs, step("public_listener", s.startPublic))
	}
	return append(cs, step("ready", s.ready))
}

// register adds the components and stop steps in the order Serve documents.
func (s *server) register() {
	r := s.runner
	for _, c := range s.components() {
		r.Add(c)
	}

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
	// The key store of the signing keys, in every role (ADR 0066 rule 1): owner, mode 0700 and
	// writability, so a read-only or foreign volume ends the start before the database wait.
	var store keyStoreKeys
	if err := config.Bind(s.values, &store); err != nil {
		return err
	}
	if err := secrets.CheckSigningKeyDir(store.Dir); err != nil {
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

// startAuthz is start step 6c (M1-28): it builds the Authorizer of authz over the pool with the
// recorder of the audit module (NewAuthorizer) and keeps it for the use cases of later tickets,
// and binds the rights hooks to the appender of sync and the recorder (M1-29).
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
	pool, a, sy := s.pool, s.audit, s.sync
	s.mu.Unlock()
	az, err := NewAuthorizer(pool, cfg, cat, a.Recorder(), s.deps.Clock, log)
	if err != nil {
		return err
	}
	if sy == nil {
		return errors.New("wire: the authz step needs the event log of the sync step")
	}
	// The rights hooks of the unit of work need the appender and the recorder, which need the unit
	// of work themselves; they are bound here, before any route or job runs (M1-29).
	if err := a.RightsHook().Bind(authzapp.RightsBindings{Events: sy.Appender(), Audit: a.Recorder()}); err != nil {
		return err
	}
	s.mu.Lock()
	s.authorizer = az
	s.mu.Unlock()
	return nil
}

// startSettings is start step 6d (M1-46): the instance settings,
// branding and discovery over the pool, the unit of work and recorder of audit, the appender of
// sync and the Authorizer. It comes before identity, because createUser reads locale.default
// through the port instance.Settings.
func (s *server) startSettings(context.Context) error {
	cfg, err := InstanceConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy := s.pool, s.audit, s.authorizer, s.sync
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil {
		return errors.New("wire: the settings step needs the unit of work and recorder of audit, the appender of sync and the Authorizer")
	}
	st, err := NewInstanceSettings(cfg, LocalPasswordEnabled(s.values), pool, a.UoW(), az,
		SettingsWrites{Audit: a.Recorder(), Events: sy.Appender()}, s.deps.Clock)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.settings = st
	s.mu.Unlock()
	return nil
}

// startIdentity is start step 6e (M1-33): the identity module over the pool, the unit of work and
// the recorder of the audit step, the appender of the sync step (M1-31) and the Authorizer of
// step 6c, with the token pepper resolved once.
func (s *server) startIdentity(ctx context.Context) error {
	cfg, err := IdentityConfig(s.values)
	if err != nil {
		return err
	}
	pepper, err := TokenPepper(ctx, s.sp, cfg.TokenPepperRef)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, st := s.pool, s.audit, s.authorizer, s.sync, s.settings
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || st == nil {
		return errors.New("wire: the identity step needs the unit of work and recorder of audit, the appender of sync, the Authorizer and the instance settings")
	}
	id, err := NewIdentity(cfg, pool, a.UoW(), az, IdentityWrites{Audit: a.Recorder(), Events: sy.Appender(), Settings: st.Admin}, pepper, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	id.Start(s.runner)
	s.mu.Lock()
	s.identity = id
	s.mu.Unlock()
	return nil
}

// startSessions is the second extension point of start step 7 (M1-34): the session part of
// identity over the signing keys of the previous step and the instance id, the issuer of access
// tokens. From here on the HTTP chain authenticates people with identity's Authenticator.
func (s *server) startSessions(context.Context) error {
	s.mu.Lock()
	id, keys, rec := s.identity, s.signingKeys, s.instance
	s.mu.Unlock()
	if id == nil || keys == nil {
		return errors.New("wire: the sessions need the identity step and the signing keys")
	}
	if err := id.BindSessions(keys.Keys, rec.InstanceID); err != nil {
		return err
	}
	eg, err := EgressConfig(s.values)
	if err != nil {
		return err
	}
	sec, err := NewProviderSecrets(s.sp, s.values)
	if err != nil {
		return err
	}
	return id.BindOIDC(OIDCDeps{Egress: eg, Secrets: sec})
}

// authenticator returns the Authenticator of the HTTP chain: identity's after startSessions,
// actor.AnonymousOnly before (fail closed).
func (s *server) authenticator() actor.Authenticator {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.identity != nil && s.identity.Authenticator != nil {
		return s.identity.Authenticator
	}
	return actor.AnonymousOnly{}
}

// handlers returns deps.Handlers with every nil module handler replaced by the one the start
// built (tests inject their own).
func (s *server) handlers() Handlers {
	h := s.deps.Handlers
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.identity != nil {
		s.identity.fillHandlers(&h)
	}
	if h.Keys == nil && s.signingKeys != nil {
		h.Keys = s.signingKeys.Handlers
	}
	if h.Log == nil && s.audit != nil {
		h.Log = s.audit.Handlers()
	}
	if h.Admin == nil && s.settings != nil {
		h.Admin = s.settings.Handlers
	}
	return h
}

// fillHandlers sets every nil identity handler of h to the one id built (M1-44/18).
func (id *Identity) fillHandlers(h *Handlers) {
	fill(&h.Devices, id.Handlers)
	fill(&h.People, id.PeopleHandlers)
	fill(&h.Sessions, id.SessionHandlers)
	fill(&h.StepUps, id.StepUpHandlers)
	fill(&h.Providers, id.ProviderHandlers)
	fill(&h.Credentials, id.CredentialHandlers)
	fill(&h.Tokens, id.TokenHandlers)
	fill(&h.Enrollment, id.EnrollmentHandlers)
	fill(&h.Approval, id.ApprovalHandlers)
	fill(&h.Passkeys, id.PasskeyHandlers)
	fill(&h.OIDC, id.OIDCHandlers)
}

// fill sets *dst to built when *dst is nil, so an injected handler stays.
func fill[T any](dst **T, built *T) {
	if *dst == nil {
		*dst = built
	}
}

// startAuditLog is the second extension point of start step 7 (M1-50): the query, export and
// verification of the audit log and the job audit.chain_verify over the signing keys of the
// previous step, the Authorizer and the instance ID. The sink of audit.checkpoint_sink is built
// here; an object store target without an effective retention lock stops the start (M7-22).
// Processes that run jobs write the warning audit_checkpoint_sink_not_configured once here when
// no sink is configured (19 SEC-AUD-2).
func (s *server) startAuditLog(ctx context.Context) error {
	s.mu.Lock()
	a, keys, az, rec := s.audit, s.signingKeys, s.authorizer, s.instance
	s.mu.Unlock()
	if a == nil || keys == nil || az == nil {
		return errors.New("wire: the audit log needs the audit step, the signing keys and the Authorizer")
	}
	sink, err := CheckpointSink(ctx, s.values, rec.InstanceID, CheckpointSinkDeps{Secrets: s.sp, Egress: egress.Deps{Logger: s.logger()}})
	if err != nil {
		return err
	}
	if err := a.BindLog(keys.Keys, az, rec.InstanceID, s.deps.Clock, sink); err != nil {
		return err
	}
	if s.worker() {
		a.WarnWithoutSink(ctx)
	}
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
// rate limiter with its start check and its sweeper (M1-19) and the Idempotency-Key stage over the
// store of sync (M1-20), and mounts the routes through it with the same catalogue. The request validator knows only the enabled operations, so the start
// compiles no schema of a route it does not serve. The chain authenticates with identity's
// Authenticator (M1-34, step sessions); without it, with actor.AnonymousOnly, which refuses every
// credential (fail closed).
func (s *server) mountRoutes(context.Context) error {
	log := s.logger()
	cat, err := NewAuthzCatalog(log)
	if err != nil {
		return err
	}
	var mount func() error
	if s.api() {
		mount = func() error {
			chain, err := NewChain(s.chainCfg, s.authenticator(), log)
			if err != nil {
				return err
			}
			limiter, err := InstallRateLimits(chain, s.values, routes.Enabled(), s.deps.Clock)
			if err != nil {
				return err
			}
			s.runner.OnStop(startSweeper(limiter))
			s.mu.Lock()
			sy := s.sync
			s.mu.Unlock()
			if err := InstallIdempotency(chain, sy, s.syncCfg); err != nil {
				return err
			}
			s.mu.Lock()
			mux, pool, a := s.publicMux(), s.pool, s.audit
			s.chain = chain
			s.mu.Unlock()
			if a == nil {
				return errors.New("wire: the setup gate needs the unit of work of the audit step")
			}
			InstallSetupGate(chain, mux, instanceapp.NewState(instancepg.NewRepository(pool), a.UoW()))
			return MountRoutes(s.role, mux, s.handlers(), cat, chain, log)
		}
	}
	return startRoutes(RouteBindings(routes.Enabled()), BuildRegistry(s.deps.Registrations), cat, log, mount)
}

// NewChain builds the HTTP chain of the public listener for the enabled operations: the validator
// over the embedded contract (api/rest.Spec), the Authenticator auth (identity's from M1-34 on; nil
// means actor.AnonymousOnly, which refuses every credential) and the global meter provider of
// platform/otel (call it after otel.Setup).
func NewChain(cfg httpserver.ChainConfig, auth actor.Authenticator, log *slog.Logger) (*httpserver.Chain, error) {
	v, err := enabledValidator()
	if err != nil {
		return nil, err
	}
	if auth == nil {
		auth = actor.AnonymousOnly{}
	}
	return httpserver.NewChain(cfg, httpserver.Deps{Logger: log, Authenticator: auth, Validator: v})
}

// enabledValidator compiles the request validator of the enabled operations once per process: the
// spec and the operation list are fixed in the binary, the validator is safe for concurrent use,
// and compiling the response schemas of every enabled operation takes seconds under load, which
// each test server of a binary would pay again (docs/DECISIONS.md M1-33/18).
var enabledValidator = stdsync.OnceValues(func() (*httpserver.Validator, error) {
	var ids []string
	for _, op := range routes.Enabled() {
		ids = append(ids, op.ID)
	}
	return httpserver.NewValidator(rest.Spec, ids)
})

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

// startSync is start step 6b: it builds the event log over the pool and the unit of work of the
// audit step, whose registry holds the sync hook (M1-16), and registers the lag gauges of the
// worker consumers in every role, so a stopped or missing worker still shows a growing lag
// (M2-01, D-M2-01-5).
func (s *server) startSync(context.Context) error {
	s.mu.Lock()
	pool, a := s.pool, s.audit
	s.mu.Unlock()
	if a == nil {
		return errors.New("wire: the sync step needs the unit of work of the audit step")
	}
	sy, err := NewSync(pool, a.UoW(), s.syncCfg, nil, s.logger())
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.sync = sy
	s.mu.Unlock()
	gauges := s.deps.ConsumerGauges
	if gauges == nil {
		gauges = func(c sync.Consumers) error {
			_, err := otelmetrics.RegisterConsumerGauges(nil, c, s.deps.Clock)
			return err
		}
	}
	return gauges(sy.Consumers())
}

// startEventConsumers starts the worker consumers of worker and all after River
// (startConsumers) with the production handlers or those of Deps.ConsumerHandlers.
func (s *server) startEventConsumers(ctx context.Context) error {
	s.mu.Lock()
	sy := s.sync
	s.mu.Unlock()
	handlers := consumerHandlers()
	if s.deps.ConsumerHandlers != nil {
		handlers = s.deps.ConsumerHandlers
	}
	return startConsumers(ctx, s.runner, sy, handlers)
}

// startLogTail starts the log tail of api and all (02 "Start and graceful shutdown" step 8, startTail).
func (s *server) startLogTail(ctx context.Context) error {
	s.mu.Lock()
	sy := s.sync
	s.mu.Unlock()
	return startTail(ctx, s.runner, sy.NewTail(tailSinks()...))
}

// startJobs checks River's migrations, builds the job registry with the jobs of the modules and
// the runtime of the role and starts River for worker and all; a River client that later stops
// on its own ends the process.
func (s *server) startJobs(ctx context.Context) error {
	s.mu.Lock()
	pool, sy, a, keys, keyCheck, id := s.pool, s.sync, s.audit, s.signingKeys, s.keyCheck, s.identity
	s.mu.Unlock()
	if err := jobs.ValidateMigrations(ctx, pool, s.jobsCfg.Schema); err != nil {
		return err
	}
	if keys == nil {
		return errors.New("wire: the jobs step needs the signing keys step")
	}
	if id == nil || id.AuthCleanup == nil {
		return errors.New("wire: the jobs step needs the sessions step")
	}
	if a.Log() == nil {
		return errors.New("wire: the jobs step needs the audit log step")
	}
	expiryCfg, err := AuthzExpiryConfig(s.values)
	if err != nil {
		return err
	}
	expiry, err := NewExpiryService(pool, a, sy, expiryCfg, s.logger())
	if err != nil {
		return err
	}
	reg, err := NewJobRegistry(&JobModules{
		Partitions: sy.Maintenance(), PartitionInterval: s.syncCfg.MaintenanceInterval,
		Cleanup: sy.IdempotencyCleanup(), CleanupInterval: s.syncCfg.Cleanup.Interval,
		Expiry: expiry, ExpiryInterval: expiryCfg.Interval,
		KeyRotation: keys.Keys, KeyRotationInterval: keyCheck,
		AuthCleanup: id.AuthCleanup, AuthCleanupInterval: id.cfg.CleanupInterval,
		ChainVerify: a.Log(), ChainVerifyInterval: a.cfg.CheckpointInterval,
	})
	if err != nil {
		return err
	}
	rt, err := jobs.New(pool, s.jobsCfg, JobRole(s.role), reg, nil)
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
