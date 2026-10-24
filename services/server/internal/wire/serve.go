// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	stdsync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/trustedheader"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
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
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
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
	// PushProviders are additional push providers of the push path (M3-19); production adds
	// none, the APNs and FCM adapters come from the configuration (M3-20, M3-21). Tests register
	// fakepush providers here.
	PushProviders []notifyapp.PushProvider
	// ConsumerGauges registers the lag gauges of the worker consumers in every role; nil means
	// otelmetrics.RegisterConsumerGauges on the global meter provider of otel.Setup. Tests read
	// the gauges of a role through their own meter provider here.
	ConsumerGauges func(consumers sync.Consumers) error
	// PresenceGauges registers the presence gauges of the agents in the roles api and all (M2-20)
	// over the samples of the tracker; nil means agents/adapters/otelmetrics.RegisterPresence on the
	// global meter provider of otel.Setup. Tests read the gauges through their own meter provider.
	PresenceGauges func(samples func() []agentsapp.GaugeSample) error
	// EgressDenyAll makes every outgoing client of the process (OIDC providers and the audit
	// checkpoint sink) refuse every target before resolution (egress.Deps.DenyAll). Only
	// test servers set it (the contract server of G7, M1-71); production leaves it false.
	EgressDenyAll bool
	// TelemetryExporters replace the OTLP exporters of the telemetry (otel.Deps.Exporters). They
	// exist for in-process tests (the canary test of M1-69); the zero value means production.
	TelemetryExporters otel.Exporters
	// OIDCRootCAs are the trusted TLS roots of the OIDC provider requests (OIDCDeps.RootCAs).
	// They exist for in-process tests against the fake provider of M1-41 (the canary test of
	// M1-69); nil means the system roots, as in production.
	OIDCRootCAs *x509.CertPool
	// TerminalTimers drive the deadlines of the terminal relay (M2-44); nil means the real timers.
	// Tests pass a fake that they fire, so no test sleeps for terminal.open_timeout.
	TerminalTimers terminalapp.Timers
	// TerminalBeforeOpenCommit is a test seam of the terminal openings (M2-44,
	// terminalapp.Deps.BeforeOpenCommit): it runs inside the unit of work of an opening before
	// the commit, so a test commits a revocation between the decision and the commit. nil in
	// production.
	TerminalBeforeOpenCommit func()
	// TerminalSignals wraps the signals of the terminal module (M2-47); a test wraps the NOTIFY
	// signals to fail a unit after its signal was staged. nil in production.
	TerminalSignals func(terminalapp.Signals) terminalapp.Signals
	// TerminalListenerGate runs before every dial of the LISTEN ephemeral connection (M2-47; the
	// one connection of the process, shared by the terminal and the gateway since M7-17); a test
	// blocks it to keep a node deaf after it ended the listener's backend. nil in production.
	TerminalListenerGate func(ctx context.Context) error
	// TerminalMetrics replaces the instruments of the relay (M2-47): two nodes in one test process
	// count bothy.terminal.relay.unpaired apart. nil creates the OpenTelemetry adapter.
	TerminalMetrics terminalapp.Metrics
	// TerminalModule receives the use cases of the terminal module once they are built (M2-47): a
	// test commits an end in a worker process through them. nil in production.
	TerminalModule func(*terminalapp.Service)
	// FilesImageHooks are the test seams of the job files.thumbnail (app.ImageHooks, ADR 0071),
	// for the integration tests of M3-06; nil in production.
	FilesImageHooks *filesapp.ImageHooks
	// JobModules receives the use cases of the jobs before the registry is built (M2-46): a test
	// runs a job handler directly (authz.expiry_sweep, terminal.sweep) instead of waiting for its
	// period. nil in production.
	JobModules func(*JobModules)
	// TxnTestSubscribers registers in-transaction subscribers of a test after every production
	// subscriber of the txn registry (Modules.TestSubscribers, M3-07): a veto after
	// files.OnThreadMoved or runs.OnFileFinalized rolls the whole unit back. nil in production.
	TxnTestSubscribers func(reg *txn.Registry) error
	// TerminalSessionMetrics replaces the session instruments of M2-46 (sessions.closed,
	// sessions.active, hosts.connected), so a test counts them per process. nil creates the
	// OpenTelemetry adapter.
	TerminalSessionMetrics terminalapp.SessionMetrics
	// ScopeObserver sees every Scope call of the process's Authorizer with its context and action
	// (authzapp.Authorizer.SetScopeObserver, M2-14): the test of SR9 records which operation lists
	// through Scope. nil in production.
	ScopeObserver func(ctx context.Context, act authorize.Action)
	// APNsEndpoints maps an APNs environment (production, sandbox) to the base URL the APNs
	// adapter sends to (apns.Deps.Endpoints), and APNsRootCAs are the trusted TLS roots of its
	// egress client (egress.Deps.RootCAs). They exist for in-process tests against Fake-APNs (the
	// push handoff measurement of M3-24); no configuration key, flag or environment variable
	// reaches them, and nil means the two Apple hosts and the system roots, as in production.
	APNsEndpoints map[string]string
	APNsRootCAs   *x509.CertPool
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
	// internalMux is the mux of the internal listener; the routes step adds the webhook routes.
	internalMux *http.ServeMux
	public      *http.Server
	mux         *http.ServeMux
	required    int64
	jobsCfg     jobs.Config
	jobsRT      *jobs.Runtime
	// The state of each module lives in its module file (module_<name>.go); each module's
	// start parts fill it.
	agentsState
	appsState
	auditState
	authzState
	billingState
	chatState
	filesState
	gatewayState
	identityState
	importerState
	instanceState
	messagingState
	notifyState
	runsState
	statsState
	syncState
	terminalState
	// search is the language configuration of full-text search (search.languages,
	// search.locale_map; M3-13): identity answers the language of new messages and threads with
	// it, and the step schema checks its configurations against the database.
	search searchlang.Map
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
//     ends the start with the hint to run "bothy migrate up" (Serve never migrates, R-AR-16);
//     then pg.CheckTextSearch with the configurations of search.languages and search.locale_map
//     (M3-13): a missing configuration or a database without case folding of non-ASCII letters
//     ends the start,
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
//     6e. agents (M1-51): build the agent records over the unit of work, the Authorizer, the rights
//     hook and the identity module (startAgents),
//     6f. chat (M1-52): build the spaces and the sidebar over the unit of work, the Authorizer,
//     the recorder and the appender (startChat), then messaging over the threads of chat (M1-56),
//     runs, the personal data, catch-up and snapshot, notify and stats (their module files),
//     6f'. files (M3-03): open the object store of file.storage.driver and build the upload
//     sessions over the unit of work, the Authorizer, the appender and the structure of chat
//     (startFiles); a root the file system driver refuses ends the start; worker and all also
//     build the scan pass with the clamd client and its health monitor (M3-04),
//     6g. terminal (M2-39): build the host records over the unit of work, the Authorizer, the
//     recorder, the appender and the identity module (startTerminal),
//     6h. apps (M2-24): build the app use cases and their handlers over identity, the rights hook
//     and the Authorizer (startApps),
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
//     a runner that ends unexpectedly ends the process with exit code 1. Then worker and all
//     start the health monitor of the virus scanner (M3-04, scan_health; it never changes
//     readiness). Every role registers the lag gauges of the consumers in step 6b,
//  8. public listener (api and all) through httpserver.ListenPublic: own TLS, or plaintext only
//     from loopback and server.trusted_proxies (SEC-TLS-1, SEC-TLS-2),
//  9. ready: readiness passes startup and the line "server ready" is written.
//
// Stop steps, by phase: not ready (always), drain and HTTP (api, all), event tail (api, all),
// consumers (worker, all: the running rounds finish, no new one starts), jobs (worker, all: the
// health monitor of the virus scanner ends, River's
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
	s.plan = TerminalClusterPlan(clusterMode(v), role)
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
	if s.search, err = SearchLanguages(v); err != nil {
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

// egressDeps are the dependencies of the outgoing clients of the process.
func (s *server) egressDeps() egress.Deps {
	return egress.Deps{Logger: s.logger(), DenyAll: s.deps.EgressDenyAll}
}

// logger returns the current process logger.
func (s *server) logger() *slog.Logger { return slog.New(s.log) }

// stepJobs is the name of the start step and of the stop step of River.
const stepJobs = "jobs"

// processWiring returns the start steps of the process itself, around those of the modules: the
// platform before the modules, the routes and the jobs after them, the public listener and
// readiness last. The order keys of the modules lie between them (module_<name>.go).
func processWiring() moduleWiring {
	return moduleWiring{parts: []part{
		{order: 100, step: "telemetry", start: (*server).startTelemetry},
		{order: 200, step: "secrets", start: (*server).startSecrets},
		{order: 300, step: "internal_listener", start: (*server).startInternal},
		{order: 400, step: "database", start: (*server).startDatabase},
		{order: 500, step: "schema", start: (*server).checkSchema},
		{order: 2000, step: "routes", start: (*server).mountRoutes},
		{order: 2300, step: stepJobs, start: whileFinalized((*server).startJobs)},
		{order: 2600, step: "public_listener", when: (*server).api, start: whileFinalized((*server).startPublic)},
		{order: 2700, step: "ready", start: (*server).ready},
	}}
}

// whileFinalized returns start, or a part that starts nothing while the start lock of R-DM-50
// holds (restore_not_finalized, M7-04): a restored database whose restore was not finalized must
// neither take writes on the public listener nor deliver from the log tail, River or the worker
// consumers, because they would mix the old and the new timeline. Only the internal listener
// stays, so /readyz names the reason; the stop steps handle the parts that never started.
func whileFinalized(start func(s *server, ctx context.Context) error) func(s *server, ctx context.Context) error {
	return func(s *server, ctx context.Context) error {
		if !s.finalized() {
			return nil
		}
		return start(s, ctx)
	}
}

// finalized reports whether the start lock of R-DM-50 is free. Work inside a start step that
// writes (the terminal reconcile of mountTerminalStreams) asks it, like whileFinalized does for a
// whole step.
func (s *server) finalized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.restorePending
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
		Exporters: s.deps.TelemetryExporters,
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
	mux := httpserver.Internal(s.reg)
	srv, host, port := s.serve(ln, httpserver.InternalHandler(mux, s.sec), "internal listener")
	s.logger().InfoContext(ctx, msgInternalListener, slog.String(attr.ServerAddress, host), slog.Int(attr.ServerPort, port))
	s.mu.Lock()
	s.internal, s.internalMux = srv, mux
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
	s.mu.Lock()
	id := s.identity
	s.mu.Unlock()
	if id != nil {
		// The bound socket is the fact, not the configured text (M1-43, 08 section 2.4 rule 1).
		id.SetPublicListenerLoopback(trustedheader.IsLoopbackListener(ln.Addr()))
		RegisterTrustedHeaderReadiness(s.reg, id)
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
	// bothy.db.pool.connections of the process pool (M1-68); the callback ends with the process.
	_, err = pool.RegisterMetrics(nil)
	return err
}

// handlers returns deps.Handlers with every nil module handler replaced by the one the start
// built (tests inject their own).
func (s *server) handlers() Handlers {
	h := s.deps.Handlers
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fillModuleHandlers(&h)
	return h
}

// checkSchema is start step 5: the schema version (pg.CheckSchema), then the text search of the
// database (M3-13, docs/DECISIONS.md D-M3-13-6): every configuration of search.languages and
// search.locale_map exists, and the database folds the case of non-ASCII letters. A failure ends
// the start with exit code 1 before any public listener opens.
func (s *server) checkSchema(ctx context.Context) error {
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	if err := pg.CheckSchema(ctx, pool, s.required); err != nil {
		return err
	}
	return textSearchError(pg.CheckTextSearch(ctx, pool, s.search.Configs()), s.search)
}

// textSearchError names the configuration key and entry of a missing text search configuration
// instead of its position in langs.Configs(), never the name itself: the first entry of
// search.languages, else of search.locale_map in key order, that holds the missing name. Other
// errors pass unchanged.
func textSearchError(err error, langs searchlang.Map) error {
	var missing *pg.TextSearchConfigMissingError
	if !errors.As(err, &missing) {
		return err
	}
	configs := langs.Configs()
	if missing.Position < 1 || missing.Position > len(configs) {
		return err
	}
	name := configs[missing.Position-1]
	if i := slices.Index(langs.Languages(), name); i >= 0 {
		return fmt.Errorf("wire: search.languages entry %d names a text search configuration the database does not have", i+1)
	}
	if i := langs.LocaleIndex(name); i > 0 {
		return fmt.Errorf("wire: the value of search.locale_map entry %d (in key order) names a text search configuration the database does not have", i)
	}
	return fmt.Errorf("wire: the database has no text search configuration %s", searchlang.Simple)
}

// mountRoutes checks the route bindings and the registrations of the modules against the closed
// catalogue of authz (every role, M1-25), then builds the HTTP chain (api and all), installs the
// rate limiter with its start check and its sweeper (M1-19) and the Idempotency-Key stage over the
// store of sync (M1-20), and mounts the routes through it with the same catalogue. The request validator knows only the enabled operations, so the start
// compiles no schema of a route it does not serve. The chain authenticates with identity's
// Authenticator (M1-34, step sessions); without it, with actor.AnonymousOnly, which refuses every
// credential (fail closed).
func (s *server) mountRoutes(ctx context.Context) error {
	log := s.logger()
	cat, err := NewAuthzCatalog(log)
	if err != nil {
		return err
	}
	var mount func() error
	if s.api() {
		mount = func() error { return s.mountAPI(ctx, cat, log) }
	}
	bindings := append(RouteBindings(routes.Enabled()), moduleRouteBindings()...)
	if err := startRoutes(bindings, BuildRegistry(s.deps.Registrations), cat, log, mount); err != nil {
		return err
	}
	// Every role serves the incoming webhooks on the internal listener (M2-27 design note 12).
	return s.mountInternalHooks(cat, log)
}

// mountAPI is the mount part of mountRoutes for api and all: the chain, the rate limiter with its
// sweeper, the Idempotency-Key stage, the setup gate, the REST routes, the web console of
// console.assets_dir (M6-02) and the mounts of the modules (gateway, terminal; their module files).
func (s *server) mountAPI(ctx context.Context, cat *authzapp.Catalog, log *slog.Logger) error {
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
	ap := s.apps
	s.mu.Unlock()
	if err := InstallWebhookToken(chain, ap, limiter); err != nil {
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
	if err := MountRoutes(s.role, mux, s.handlers(), cat, chain, log, WebhookBodyLimit(s.values.Int(keyWebhookInMaxBodyBytes))); err != nil {
		return err
	}
	if err := MountConsole(mux, s.values); err != nil {
		return err
	}
	return s.mountModules(ctx, mux, chain, limiter, log)
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

// WarmRequestValidator builds the request validator of the enabled operations of this process
// ahead of the first start (the result is cached): test binaries that start whole servers call it
// before their tests, so no readiness wait pays the build (tens of seconds under -race). Errors:
// those of httpserver.NewValidator.
func WarmRequestValidator() error {
	_, err := enabledValidator()
	return err
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
	return append(steps, lifecycle.StopStep{Name: stepJobs, Phase: lifecycle.PhaseJobs, Budget: s.jobsCfg.SoftStopTimeout, Run: s.stopJobs})
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

// startJobs checks River's migrations, builds the job registry with the jobs of the modules and
// the runtime of the role and starts River for worker and all; a River client that later stops
// on its own ends the process.
func (s *server) startJobs(ctx context.Context) error {
	s.mu.Lock()
	pool := s.pool
	s.mu.Unlock()
	if err := jobs.ValidateMigrations(ctx, pool, s.jobsCfg.Schema); err != nil {
		return err
	}
	var modules JobModules
	if err := s.moduleJobs(&modules); err != nil {
		return err
	}
	if s.deps.JobModules != nil {
		s.deps.JobModules(&modules)
	}
	reg, err := NewJobRegistry(&modules)
	if err != nil {
		return err
	}
	rt, err := jobs.New(pool, s.jobsCfg, JobRole(s.role), reg, nil)
	if err != nil {
		return err
	}
	s.bindModuleJobs(rt.Inserter())
	s.mu.Lock()
	s.jobsRT = rt
	s.mu.Unlock()
	if err := rt.Start(ctx); err != nil {
		return err
	}
	s.runner.FailOn(stepJobs, rt.Failed())
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
	pool, files := s.pool, s.files
	s.mu.Unlock()
	if pool != nil {
		pool.Close()
	}
	// The object store of the files module goes with the database: no request uses it any more.
	return files.Close()
}
