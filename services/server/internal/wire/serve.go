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
	"github.com/OWNER/bothy/services/server/internal/authz"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	gatewayapp "github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/secretbox"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/trustedheader"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
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
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/runs"
	runsjobs "github.com/OWNER/bothy/services/server/internal/runs/adapters/jobs"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/otelmetrics"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/wsrelay"
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
	// PresenceGauges registers the presence gauges of the agents in the roles api and all (M2-20)
	// over the samples of the tracker; nil means agents/adapters/otelmetrics.RegisterPresence on the
	// global meter provider of otel.Setup. Tests read the gauges through their own meter provider.
	PresenceGauges func(samples func() []agentsapp.GaugeSample) error
	// EgressDenyAll makes every outgoing client of the process (OIDC providers, webhook delivery,
	// audit checkpoint sink) refuse every target before resolution (egress.Deps.DenyAll). Only
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
	// FilesImageHooks are the test seams of the job files.thumbnail (app.ImageHooks, ADR 0071),
	// for the integration tests of M3-06; nil in production.
	FilesImageHooks *filesapp.ImageHooks
	// TerminalSignals wraps the signals of the terminal module (M2-47); a test wraps the NOTIFY
	// signals to fail a unit after its signal was staged. nil in production.
	TerminalSignals func(terminalapp.Signals) terminalapp.Signals
	// TerminalListenerGate runs before every dial of the LISTEN ephemeral connection (M2-47); a
	// test blocks it to keep a node deaf after it ended the listener's backend. nil in production.
	TerminalListenerGate func(ctx context.Context) error
	// TerminalMetrics replaces the instruments of the relay (M2-47): two nodes in one test process
	// count bothy.terminal.relay.unpaired apart. nil creates the OpenTelemetry adapter.
	TerminalMetrics terminalapp.Metrics
	// TerminalModule receives the use cases of the terminal module once they are built (M2-47): a
	// test commits an end in a worker process through them. nil in production.
	TerminalModule func(*terminalapp.Service)
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
	// syncCfg and sync are the event log (M1-16): its appender for the modules, the partition
	// maintenance of the job and the log tail of api and all.
	syncCfg SyncConfig
	sync    *Sync
	// authorizer is the kernel/authorize.Authorizer of the process (M1-28); the use cases of later
	// tickets receive it.
	authorizer authorize.Authorizer
	// authzAdmin is the group and grant management of authz with its REST handlers (M1-48).
	authzAdmin *AuthzAdmin
	// identity is the identity module (M1-33): devices, the DPoP verifier and the REST handlers.
	identity *Identity
	// totpBox seals the TOTP secrets of local accounts with auth.totp_key_ref (M1-39), resolved in
	// the identity step and bound with the sessions.
	totpBox *secretbox.Box
	// settings are the instance settings, branding and discovery (M1-46); identity reads
	// locale.default through them.
	settings *InstanceSettings
	// agents is the agents module (M1-51): the agent records and their REST handlers.
	agents *Agents
	// chat is the chat module (M1-52): spaces, the sidebar and their REST handlers.
	chat *Chat
	// messaging is the messaging module (M1-56), built in the chat step over the threads of chat.
	messaging *Messaging
	// runs is the runs module (M2-05), built in the chat step over messaging, chat and agents; the
	// chat step binds the acceptance of the unit of work to it.
	runs *Runs
	// personal is the personal data of sync (M1-61), built in the chat step over chat and messaging.
	personal *Personal
	// stats is the client metrics part of the stats module (M4-31), built in the chat step.
	stats *Stats
	// notify is the notify module (M3-18), built in the chat step over chat, sync and identity.
	notify *Notify
	// syncReads is catch-up and snapshot of sync (M1-63), built in the chat step after personal.
	syncReads *SyncReads
	// files is the files module (M3-03): the upload sessions over the object store, built in the
	// files step after chat.
	files *Files
	// gateway is the WebSocket gateway of api and all (M1-64) with the hub the log tail feeds
	// (M1-65).
	gateway *Gateway
	// terminal is the terminal module (M2-39): the host records and their REST handlers.
	terminal *Terminal
	// terminalStreams are the relay and the stream routes of api and all (M2-44); the log tail
	// feeds the relay.
	terminalStreams *TerminalStreams
	// plan is what cluster.mode means for the role (M2-47); terminalLocal are the local signals of
	// one node, whose handler the hub becomes; terminalHub is the handler of the signals of an api
	// process with its configuration hubCfg.
	plan          ClusterPlan
	terminalLocal *terminalapp.LocalSignals
	terminalHub   *wsrelay.Hub
	hubCfg        ClusterHubConfig
	// apps is the module apps (M2-24): the app use cases and their REST handlers.
	apps *Apps
	// signingKeys are the server signing keys (M1-32) and keyCheck the interval of their job.
	signingKeys *SigningKeys
	keyCheck    time.Duration
	// blocked is set when a readiness check fails for good (schema_requires_newer_server).
	blocked bool
	// restorePending is set by the start lock of R-DM-50 (restore_not_finalized, M7-04): the
	// process then serves and processes nothing, see whileFinalized.
	restorePending bool
	// search is the language configuration of full-text search (search.languages,
	// search.locale_map; M3-13): identity answers the language of new messages and threads with
	// it, and the step schema checks its configurations against the database.
	search searchlang.Map
}

// Serve assembles `bothy serve` for role from the loaded values v and returns the Runner;
// cmd/bothy exits with Runner.Run. It starts nothing itself. The components start in the order
// of docs/spec/02-architektur.md "Start":
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
//     the recorder and the appender, then messaging over the threads of chat (M1-56; startChat),
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
//     built over the pool, and worker and all start River (02 "Start" step 8); a River client
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
		step("agents", s.startAgents),
		step("chat", s.startChat),
		step("files", s.startFiles),
		step("terminal", s.startTerminal),
		step("apps", s.startApps),
		step("signing_keys", s.startSigningKeys),
		step("audit_log", s.startAuditLog),
		step("sessions", s.startSessions),
		step("routes", s.mountRoutes),
	}
	if s.api() {
		cs = append(cs, step("log_tail", s.whileFinalized(s.startLogTail)), step("agent_presence", s.whileFinalized(s.startPresence)))
	}
	cs = append(cs, step("jobs", s.whileFinalized(s.startJobs)))
	if s.worker() {
		// 02 "Start" step 8: the worker consumers start after River (M2-01).
		cs = append(cs, step("event_consumers", s.whileFinalized(s.startEventConsumers)), step("scan_health", s.startScanHealth))
	}
	if s.api() {
		cs = append(cs, step("public_listener", s.whileFinalized(s.startPublic)))
	}
	if s.plan.Listener {
		// M2-47: the LISTEN ephemeral connection after the HTTP listeners (soft dependency).
		cs = append(cs, step("terminal_ephemeral", s.startTerminalEphemeral))
	}
	return append(cs, step("ready", s.ready))
}

// whileFinalized returns start, or a step that starts nothing while the start lock of R-DM-50
// holds (restore_not_finalized, M7-04): a restored database whose restore was not finalized must
// neither take writes on the public listener nor deliver from the log tail, River or the worker
// consumers, because they would mix the old and the new timeline. Only the internal listener
// stays, so /readyz names the reason; the stop steps handle the parts that never started.
func (s *server) whileFinalized(start func(ctx context.Context) error) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		s.mu.Lock()
		pending := s.restorePending
		s.mu.Unlock()
		if pending {
			return nil
		}
		return start(ctx)
	}
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
		r.OnStop(lifecycle.StopStep{Name: "drain", Phase: lifecycle.PhaseDrain, Budget: s.cfg.DrainDelay, Run: s.drain})
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
// "Ausfall einer Abhängigkeit"), and a missing file is reported before the database wait. For the
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
// binds the rights hooks to the appender of sync and the recorder (M1-29), and builds the group
// and grant management with its REST handlers (M1-48).
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
	adminCfg, err := AuthzAdminConfig(s.values)
	if err != nil {
		return err
	}
	admin, err := NewAuthzAdmin(pool, adminCfg, a, sy, az, s.deps.Clock)
	if err != nil {
		return err
	}
	// The rights hooks of the unit of work need the appender and the recorder, which need the unit
	// of work themselves; they are bound here, before any route or job runs (M1-29).
	if err := a.RightsHook().Bind(authzapp.RightsBindings{Events: sy.Appender(), Audit: a.Recorder()}); err != nil {
		return err
	}
	if err := a.SyncSubscribers().Bind(sy.Appender()); err != nil {
		return err
	}
	s.mu.Lock()
	s.authorizer, s.authzAdmin = az, admin
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
	if st.CSPReports, err = NewCSPReports(az, s.logger(), nil, int(s.values.Int(KeyCSPReportMaxRecords))); err != nil {
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
	// The TOTP key is resolved once here: a missing key or one that is not 32 bytes stops the
	// start (M1-39), not the first password login.
	box, err := TOTPBox(ctx, s.sp, cfg.TOTPKeyRef)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, st := s.pool, s.audit, s.authorizer, s.sync, s.settings
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || st == nil {
		return errors.New("wire: the identity step needs the unit of work and recorder of audit, the appender of sync, the Authorizer and the instance settings")
	}
	fresh, err := FreshStepUpCheck(s.values)
	if err != nil {
		return err
	}
	id, err := NewIdentity(cfg, pool, a.UoW(), az, IdentityWrites{Audit: a.Recorder(), Events: sy.Appender(), Settings: st.Admin, Flags: st.Admin,
		Fresh: fresh, Apps: NewAppInstallations(pool), Search: s.search}, pepper, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	id.Start(s.runner)
	s.mu.Lock()
	s.identity, s.totpBox = id, box
	s.mu.Unlock()
	return nil
}

// startAgents is start step 6e (M1-51): the agents module over the pool, the unit of work, the
// recorder and the rights hook of the audit step, the appender of the sync step, the Authorizer
// and the identity module.
func (s *server) startAgents(context.Context) error {
	cfg, err := AgentsConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, id := s.pool, s.audit, s.authorizer, s.sync, s.identity
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || id == nil {
		return errors.New("wire: the agents step needs audit, sync, the Authorizer and identity")
	}
	ag, err := NewAgents(cfg, pool, a.UoW(), az, a.RightsHook(), IdentityWrites{Audit: a.Recorder(), Events: sy.Appender()}, id, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	pcfg, err := PresenceConfigFrom(s.values)
	if err != nil {
		return err
	}
	if err := ag.BindPresence(pcfg, pool, PresenceDeps{UoW: a.UoW(), Authorizer: az, Audit: a.Recorder(), Clock: s.deps.Clock,
		Log: s.logger()}); err != nil {
		return err
	}
	if s.api() {
		// Registered before the gateway's stop steps (mountGateway), so it runs after them: no
		// heartbeat arrives once it flushes (D-M2-20-5).
		s.runner.OnStop(ag.PresenceStopStep())
	}
	s.mu.Lock()
	s.agents = ag
	s.mu.Unlock()
	return nil
}

// startPresence starts the presence tracker of the agents in the roles api and all (M2-20): the
// start rule, the flush loop and the presence gauges.
func (s *server) startPresence(ctx context.Context) error {
	s.mu.Lock()
	ag := s.agents
	s.mu.Unlock()
	return ag.StartPresence(ctx, s.deps.PresenceGauges)
}

// startChat is start step 6f (M1-52): the chat module over the pool, the unit of work and the
// recorder of the audit step, the appender of the sync step and the Authorizer, and the messaging
// module over the threads of chat (M1-56).
func (s *server) startChat(context.Context) error {
	cfg, err := ChatConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, id := s.pool, s.audit, s.authorizer, s.sync, s.identity
	s.mu.Unlock()
	rights, ok := az.(authz.RightsQuery)
	if a == nil || sy == nil || !ok || id == nil {
		return errors.New("wire: the chat step needs audit, sync, the Authorizer of authz and identity")
	}
	c, err := NewChat(cfg, pool, a.UoW(), az, rights, ChatWrites{Audit: a.Recorder(), Events: sy.Appender(), Rights: a.RightsHook(),
		Directory: id.Directory}, nil, s.deps.Clock)
	if err != nil {
		return err
	}
	mcfg, err := MessagingConfig(s.values)
	if err != nil {
		return err
	}
	m, err := NewMessaging(mcfg, pool, a.UoW(), az, MessagingWrites{Events: sy.Appender(), Redactor: sy.Redactor(), Audit: a.Recorder(),
		Credentials: lazyCredentials(s.authenticator)},
		c.Service, id.Directory, s.deps.Clock, s.logger())
	if err != nil {
		return err
	}
	rn, err := s.buildRuns(pool, a, az, sy, c, m)
	if err != nil {
		return err
	}
	pcfg, err := PersonalConfig(s.values)
	if err != nil {
		return err
	}
	p, err := NewPersonal(pcfg, pool, a.UoW(), az, sy.Appender(), c.Service, m.Service, s.deps.Clock)
	if err != nil {
		return err
	}
	s.mu.Lock()
	ag := s.agents
	s.mu.Unlock()
	if ag != nil && ag.Presence != nil {
		// The heartbeat applies project readiness through chat (M2-03, D-M2-20-10).
		ag.Presence.BindReadiness(c.Service)
	}
	n, err := s.startNotify(pool, a, az, sy, c, p, id)
	if err != nil {
		return err
	}
	r, err := s.buildSyncReads(pool, a.UoW(), az, c, m, p, rn)
	if err != nil {
		return err
	}
	st, err := NewStats(az, p.Service.Settings())
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.chat, s.messaging, s.runs, s.personal, s.notify, s.syncReads, s.stats = c, m, rn, p, n, r, st
	s.mu.Unlock()
	return nil
}

// buildRuns builds the runs module (M2-05) in the chat step over messaging (anchor and trigger
// state), the threads and channel facts of chat and the agent facts of the agents step, binds the
// acceptance of the unit of work to it and, in the worker role, registers the gauge
// bothy.run.active on the global meter provider.
func (s *server) buildRuns(pool *pg.Pool, a *Audit, az authorize.Authorizer, sy *Sync, c *Chat, m *Messaging) (*Runs, error) {
	cfg, err := RunsConfig(s.values)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	ag := s.agents
	s.mu.Unlock()
	if ag == nil {
		return nil, errors.New("wire: the runs module needs the agents step")
	}
	rn, err := NewRuns(cfg, pool, a.UoW(), az, RunsDeps{
		Events: sy.Appender(), Messages: m.Service, Threads: c.Service, Channels: c.Service, Agents: ag.Service,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Audit: a.Recorder(),
	}, s.deps.Clock, s.logger())
	if err != nil {
		return nil, err
	}
	if err := a.RunsAcceptor().Bind(rn.Service); err != nil {
		return nil, err
	}
	if s.worker() {
		if err := rn.RegisterActiveGauge(); err != nil {
			return nil, err
		}
	}
	return rn, nil
}

// startNotify builds the notify module (M3-18) in the chat step over the chat service, the
// settings of the personal data of sync and the directory of identity, and binds the
// in-transaction subscribers of notify of the unit of work to the appender and the chat paths.
func (s *server) startNotify(pool *pg.Pool, a *Audit, az authorize.Authorizer, sy *Sync, c *Chat, p *Personal, id *Identity) (*Notify, error) {
	cfg, err := NotifyConfig(s.values)
	if err != nil {
		return nil, err
	}
	n, err := NewNotify(cfg, pool, a.UoW(), az, sy.Appender(), NotifyDeps{Chat: c.Service, Settings: p.Service.Settings(), Users: id.Directory},
		s.deps.Clock, s.logger())
	if err != nil {
		return nil, err
	}
	if err := a.NotifySubscribers().Bind(sy.Appender(), c.Service); err != nil {
		return nil, err
	}
	return n, nil
}

// buildSyncReads builds catch-up and snapshot of sync (M1-63) over the chat and messaging modules
// and the personal data of the chat step, the agents and identity steps and the instance
// settings (feature flags).
func (s *server) buildSyncReads(pool *pg.Pool, uow txn.UnitOfWork, az authorize.Authorizer, c *Chat, m *Messaging, p *Personal,
	rn *Runs) (*SyncReads, error) {
	cfg, err := CatchUpConfig(s.values)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	ag, id, st := s.agents, s.identity, s.settings
	s.mu.Unlock()
	if ag == nil || id == nil || st == nil {
		return nil, errors.New("wire: catch-up and snapshot need the agents, identity and settings steps")
	}
	return NewSyncReads(cfg, SyncReadDeps{Pool: pool, UoW: uow, Authorizer: az, Chat: c.Service, Messaging: m.Service, Personal: p.Service,
		Runs:   rn.Service,
		Agents: ag.Service, Directory: id.Directory, Flags: st.Admin})
}

// startTerminal is start step 6g (M2-39): the terminal module over the pool, the unit of work and
// the recorder of the audit step, the appender of the sync step, the Authorizer and the identity
// module.
func (s *server) startTerminal(context.Context) error {
	cfg, err := TerminalConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, id := s.pool, s.audit, s.authorizer, s.sync, s.identity
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || id == nil {
		return errors.New("wire: the terminal step needs audit, sync, the Authorizer and identity")
	}
	tm, err := NewTerminal(cfg, pool, a.UoW(), az, IdentityWrites{Audit: a.Recorder(), Events: sy.Appender()}, id, s.deps.Clock,
		s.deps.TerminalBeforeOpenCommit, s.logger())
	if err != nil {
		return err
	}
	sig, local := terminalSignals(s.plan, pool, s.deps.TerminalSignals)
	tm.Service.BindSignals(sig)
	if s.deps.TerminalModule != nil {
		s.deps.TerminalModule(tm.Service)
	}
	s.mu.Lock()
	s.terminal, s.terminalLocal = tm, local
	s.mu.Unlock()
	return nil
}

// startApps is start step 6h (M2-24, M2-26): the module apps over the pool, the unit of work,
// rights hook and recorder of the audit step, the Authorizer and identity, with the outgoing
// webhooks over the egress configuration, the master key of the v1 secrets (resolved once, so a
// missing or malformed key ends the start), the instance row and the log reader of sync. The
// signing keys and the job runtime start later and are bound then (BindKeys, BindJobs).
func (s *server) startApps(ctx context.Context) error {
	cfg, err := AppsConfig(s.values)
	if err != nil {
		return err
	}
	wcfg, masterRef, err := WebhooksConfig(s.values)
	if err != nil {
		return err
	}
	eg, err := EgressConfig(s.values)
	if err != nil {
		return err
	}
	sender, err := NewWebhookSender(eg, wcfg, s.egressDeps())
	if err != nil {
		return err
	}
	master := NewHMACMasterKey(s.sp, masterRef)
	if err := master.Check(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, admin, id, sy := s.pool, s.audit, s.authorizer, s.authzAdmin, s.identity, s.sync
	s.mu.Unlock()
	if a == nil || az == nil || admin == nil || id == nil || sy == nil {
		return errors.New("wire: the apps step needs the audit step, sync, the Authorizer, the authz Admin and identity")
	}
	state := instanceapp.NewState(instancepg.NewRepository(pool), a.UoW())
	ap, err := NewApps(cfg, pool, AppsDeps{UoW: a.UoW(), Rights: a.RightsHook(), Authorizer: az, Audit: a.Recorder(), Identity: id,
		Grants: admin.Admin, Clock: s.deps.Clock, Webhooks: WebhookDeps{
			Config: wcfg, State: state, Log: sy.LogReader(), Master: master, Sender: sender, Logger: s.logger(),
		}})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.apps = ap
	s.mu.Unlock()
	return nil
}

// startSessions is the second extension point of start step 7 (M1-34): the session part of
// identity over the signing keys of the previous step and the instance id, the issuer of access
// tokens. From here on the HTTP chain authenticates people with identity's Authenticator.
func (s *server) startSessions(context.Context) error {
	s.mu.Lock()
	id, keys, rec, admin, box := s.identity, s.signingKeys, s.instance, s.authzAdmin, s.totpBox
	s.mu.Unlock()
	if id == nil || keys == nil || admin == nil {
		return errors.New("wire: the sessions need the identity step, the signing keys and the authz admin")
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
	if err := id.BindOIDC(OIDCDeps{Egress: eg, Secrets: sec, ClaimGroups: admin.Admin, EgressDenyAll: s.deps.EgressDenyAll,
		RootCAs: s.deps.OIDCRootCAs}); err != nil {
		return err
	}
	return id.BindLocal(box, nil)
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
	if h.Agents == nil && s.agents != nil {
		h.Agents = s.agents.Handlers
	}
	if s.chat != nil {
		s.chat.fillHandlers(&h)
	}
	s.messaging.fillHandlers(&h)
	s.runs.fillHandlers(&h)
	s.personal.fillHandlers(&h)
	s.notify.fillHandlers(&h)
	s.syncReads.fillHandlers(&h)
	s.files.fillHandlers(&h)
	s.stats.fillHandlers(&h)
	s.terminal.fillHandlers(&h)
	if h.Keys == nil && s.signingKeys != nil {
		h.Keys = s.signingKeys.Handlers
	}
	if h.Log == nil && s.audit != nil {
		h.Log = s.audit.Handlers()
	}
	s.settings.fillHandlers(&h)
	if h.Access == nil && s.authzAdmin != nil {
		h.Access = s.authzAdmin.Handlers
	}
	s.apps.fillHandlers(&h)
	return h
}

// fillHandlers sets every nil identity handler of h to the one id built (INT-W12/3, INT-W13/3).
func (id *Identity) fillHandlers(h *Handlers) {
	fill(&h.Devices, id.Handlers)
	fill(&h.People, id.PeopleHandlers)
	fill(&h.Avatars, id.AvatarHandlers)
	fill(&h.Sessions, id.SessionHandlers)
	fill(&h.StepUps, id.StepUpHandlers)
	fill(&h.Providers, id.ProviderHandlers)
	fill(&h.Credentials, id.CredentialHandlers)
	fill(&h.Tokens, id.TokenHandlers)
	fill(&h.Enrollment, id.EnrollmentHandlers)
	fill(&h.Approval, id.ApprovalHandlers)
	fill(&h.Passkeys, id.PasskeyHandlers)
	fill(&h.OIDC, id.OIDCHandlers)
	fill(&h.TrustedHeader, id.TrustedHeaderHandlers)
	fill(&h.Lifecycle, id.LifecycleHandlers)
	fill(&h.Local, id.LocalHandlers)
	fill(&h.Console, id.ConsoleHandlers)
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
	sink, err := CheckpointSink(ctx, s.values, rec.InstanceID, CheckpointSinkDeps{Secrets: s.sp, Egress: s.egressDeps()})
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
func (s *server) mountRoutes(context.Context) error {
	log := s.logger()
	cat, err := NewAuthzCatalog(log)
	if err != nil {
		return err
	}
	var mount func() error
	if s.api() {
		mount = func() error { return s.mountAPI(cat, log) }
	}
	bindings := append(RouteBindings(routes.Enabled()), GatewayRouteBinding(), HostConnectRouteBinding())
	bindings = append(bindings, TerminalStreamRouteBindings()...)
	return startRoutes(bindings, BuildRegistry(s.deps.Registrations),
		cat, log, mount)
}

// mountAPI is the mount part of mountRoutes for api and all: the chain, the rate limiter with its
// sweeper, the Idempotency-Key stage, the setup gate, the REST routes, the web console of
// console.assets_dir (M6-02) and the gateway.
func (s *server) mountAPI(cat *authzapp.Catalog, log *slog.Logger) error {
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
	if err := MountRoutes(s.role, mux, s.handlers(), cat, chain, log); err != nil {
		return err
	}
	if err := MountConsole(mux, s.values); err != nil {
		return err
	}
	if err := s.mountGateway(mux, chain, limiter, log); err != nil {
		return err
	}
	return s.mountHostControl(mux, chain, log)
}

// mountHostControl builds the control connection of the host services (M2-43) over the terminal
// module, the ephemeral publisher of the gateway and the signing keys, mounts GET
// /api/v1/hosts/connect on mux and registers its stop step.
func (s *server) mountHostControl(mux Mux, chain *httpserver.Chain, log *slog.Logger) error {
	cfg, err := HostControlConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	tm, gw, keys := s.terminal, s.gateway, s.signingKeys
	s.mu.Unlock()
	if tm == nil || gw == nil || keys == nil {
		return errors.New("wire: the control connection needs the terminal module, the gateway and the signing keys")
	}
	hc, err := NewHostControl(cfg, tm, gw.Publisher, keys.Keys, s.deps.Clock, log)
	if err != nil {
		return err
	}
	hc.Mount(mux, chain)
	s.runner.OnStop(hc.StopConnections())
	return s.mountTerminalStreams(mux, chain, tm, hc, log)
}

// mountTerminalStreams builds the relay and the two stream routes of the terminal sessions
// (M2-44) over the module, the control connection, identity's authenticator and nonces and the
// signing keys, builds the hub of the signals across nodes (M2-47), reconciles the sessions of the
// previous process (Design note 14, only with cluster.mode single), mounts both routes on mux and
// registers the stop step of the relay.
func (s *server) mountTerminalStreams(mux Mux, chain *httpserver.Chain, tm *Terminal, hc *HostControl, log *slog.Logger) error {
	cfg, err := TerminalStreamsConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	id, keys := s.identity, s.signingKeys
	s.mu.Unlock()
	if id == nil || id.Authenticator == nil || id.DPoP == nil || keys == nil {
		return errors.New("wire: the terminal streams need identity's authenticator and nonces and the signing keys")
	}
	ts, err := NewTerminalStreams(cfg, tm, hc, TerminalStreamsDeps{Authenticator: id.Authenticator, Nonces: id.DPoP, Signer: keys.Keys,
		Timers: s.deps.TerminalTimers, Metrics: s.deps.TerminalMetrics, Clock: s.deps.Clock, Log: log, Multi: s.plan.Multi})
	if err != nil {
		return err
	}
	s.mu.Lock()
	local := s.terminalLocal
	s.mu.Unlock()
	hub, hubCfg, err := newTerminalHub(s.values, s.plan, tm, hc, ts, local, s.deps.Clock, log)
	if err != nil {
		return err
	}
	if !s.plan.Multi {
		// One node: the slots and connections of the previous process are gone (M2-44 Design
		// note 14); with several nodes the other nodes still hold theirs.
		if err := tm.Service.Reconcile(context.Background()); err != nil {
			return err
		}
	}
	ts.Mount(mux, chain)
	s.runner.OnStop(ts.StopConnections())
	s.mu.Lock()
	s.terminalStreams, s.terminalHub, s.hubCfg = ts, hub, hubCfg
	s.mu.Unlock()
	return nil
}

// mountGateway builds the WebSocket gateway (M1-64) over identity's Authenticator and DPoP
// nonces, the Authorizer, the limiter of the chain (classes rl.gateway_connect, rl.read and
// rl.typing), catch-up and snapshot of sync, the instance row and the threads of chat (M1-66),
// mounts GET /api/v1/gateway on mux and starts the sweeper of the gateway sessions with its stop
// step.
func (s *server) mountGateway(mux Mux, chain *httpserver.Chain, limiter *ratelimit.Limiter, log *slog.Logger) error {
	cfg, err := GatewayConfigFrom(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	id, az, reads, pool, a, ch, ag, rn := s.identity, s.authorizer, s.syncReads, s.pool, s.audit, s.chat, s.agents, s.runs
	s.mu.Unlock()
	var invocations runs.Delivery
	if rn != nil {
		invocations = rn.Service
	}
	if id == nil || id.Authenticator == nil || az == nil || reads == nil || a == nil || ch == nil || ag == nil {
		return errors.New("wire: the gateway needs identity, the Authorizer, catch-up and snapshot, the unit of work, chat and agents")
	}
	gw, err := NewGateway(cfg, GatewayDeps{
		Authenticator: id.Authenticator, Authorizer: az, Nonces: id.DPoP, Limiter: limiter, Reads: reads,
		Instance: instanceapp.NewState(instancepg.NewRepository(pool), a.UoW()), Threads: ch.Service, Consumers: ag.Consumers,
		Presence: ag.Presence, Invocations: invocations, Clock: s.deps.Clock, Log: log,
	})
	if err != nil {
		return err
	}
	if ag.Presence != nil {
		// agent.presence_changed goes through the hub of this process (M1-66, D-M2-20-1).
		ag.Presence.BindPublisher(gw.Publisher)
	}
	if gw.Engine != nil {
		// The REST consumer operations read through the engine (agents.ConsumerFeed, M2-07).
		ag.Consumers.SetFeed(gw.Engine)
	}
	gw.Mount(mux, chain)
	s.runner.OnStop(gw.StartSweeper())
	s.runner.OnStop(gw.StartConsumerSaver())
	s.runner.OnStop(gw.StopConnections())
	s.mu.Lock()
	s.gateway = gw
	s.mu.Unlock()
	return nil
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
	sy, ap := s.sync, s.apps
	s.mu.Unlock()
	handlers := consumerHandlers(ap)
	if s.deps.ConsumerHandlers != nil {
		handlers = s.deps.ConsumerHandlers
	}
	return startConsumers(ctx, s.runner, sy, handlers)
}

// startLogTail starts the log tail of api and all (02 "Start" step 8, startTail).
func (s *server) startLogTail(ctx context.Context) error {
	s.mu.Lock()
	pool, sy, az, gw, ts := s.pool, s.sync, s.authorizer, s.gateway, s.terminalStreams
	s.mu.Unlock()
	var hub *gatewayapp.Hub
	if gw != nil {
		hub = gw.Hub
	}
	var observers []LogObserver
	if ts != nil {
		observers = append(observers, ts.Observer())
	}
	versions := instanceapp.NewVersions(instancepg.NewRepository(pool))
	tail := sy.NewTail(DeliverySink(az, hub, versions, observers...))
	if err := startTail(ctx, s.runner, s.reg, tail); err != nil {
		return err
	}
	if hub != nil {
		// The hub position starts at the head the tail started at (agent consumers, M2-07).
		hub.Seed(tail.Head())
	}
	return nil
}

// startJobs checks River's migrations, builds the job registry with the jobs of the modules and
// the runtime of the role and starts River for worker and all; a River client that later stops
// on its own ends the process.
func (s *server) startJobs(ctx context.Context) error {
	s.mu.Lock()
	pool, sy, a, keys, keyCheck, id, ap, files := s.pool, s.sync, s.audit, s.signingKeys, s.keyCheck, s.identity, s.apps, s.files
	s.mu.Unlock()
	if err := jobs.ValidateMigrations(ctx, pool, s.jobsCfg.Schema); err != nil {
		return err
	}
	if ap == nil {
		return errors.New("wire: the jobs step needs the apps step")
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
	var queuedExpiry runsjobs.Expirer
	if s.runs != nil {
		queuedExpiry = s.runs.Service
	}
	reg, err := NewJobRegistry(&JobModules{
		Partitions: sy.Maintenance(), PartitionInterval: s.syncCfg.MaintenanceInterval,
		Cleanup: sy.IdempotencyCleanup(), CleanupInterval: s.syncCfg.Cleanup.Interval,
		Expiry: expiry, ExpiryInterval: expiryCfg.Interval,
		KeyRotation: keys.Keys, KeyRotationInterval: keyCheck,
		AuthCleanup: id.AuthCleanup, AuthCleanupInterval: id.cfg.CleanupInterval,
		ChainVerify: a.Log(), ChainVerifyInterval: a.cfg.CheckpointInterval,
		Deliveries:  ap.Webhooks,
		FilesScan:   files.ScanRunner(),
		FilesImages: files.ImageRunner(), FilesImageTimeout: s.imageTimeout(),
		QueuedExpiry: queuedExpiry,
	})
	if err != nil {
		return err
	}
	rt, err := jobs.New(pool, s.jobsCfg, JobRole(s.role), reg, nil)
	if err != nil {
		return err
	}
	ap.BindJobs(rt.Inserter())
	files.BindJobs(rt.Inserter())
	s.mu.Lock()
	rn := s.runs
	s.mu.Unlock()
	rn.BindJobs(rt.Inserter())
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
	pool, files := s.pool, s.files
	s.mu.Unlock()
	if pool != nil {
		pool.Close()
	}
	// The object store of the files module goes with the database: no request uses it any more.
	return files.Close()
}

// startFiles is start step 6f' (M3-03): it opens the object store of file.storage.driver
// (FilesObjectStore: the driver fs checks file.fs.root, the driver s3 its keys, secrets and egress)
// and builds the files module over the pool, the unit of work of the audit step, the Authorizer,
// the appender of sync and the structure of chat. A root that fs.Open refuses ends the start.
func (s *server) startFiles(ctx context.Context) error {
	cfg, err := FilesConfig(s.values)
	if err != nil {
		return err
	}
	s.mu.Lock()
	pool, a, az, sy, c := s.pool, s.audit, s.authorizer, s.sync, s.chat
	s.mu.Unlock()
	if a == nil || az == nil || sy == nil || c == nil {
		return errors.New("wire: the files step needs audit, the Authorizer, sync and chat")
	}
	store, err := FilesObjectStore(ctx, s.values, FilesDeps{Secrets: s.sp, Egress: s.egressDeps()})
	if err != nil {
		return err
	}
	f, err := NewFiles(cfg, s.values.String(keyStorageDriver), store, pool, a.UoW(), az, FilesWrites{Events: sy.Appender()},
		c.Service, s.deps.Clock, s.logger())
	if err != nil {
		if cl, ok := store.(io.Closer); ok {
			_ = cl.Close()
		}
		return err
	}
	if s.api() {
		// Downloads, tickets and transcripts are served by the roles with routes; the ticket key
		// must exist and be long enough before such a role starts (D-M3-05-15).
		if err := s.enableDownloads(ctx, f); err != nil {
			_ = f.Close()
			return err
		}
	}
	if s.worker() {
		if err := s.enableFilesJobs(f, a.Recorder()); err != nil {
			_ = f.Close()
			return err
		}
	}
	s.mu.Lock()
	s.files = f
	id, st := s.identity, s.settings
	s.mu.Unlock()
	f.BindProfileImages(id, st)
	return nil
}

// enableFilesJobs builds the use cases of files.scan (M3-04) and files.thumbnail (M3-06) in a
// role that runs jobs.
func (s *server) enableFilesJobs(f *Files, audit auditing.Recorder) error {
	scanCfg, clamdCfg, err := ScanConfigs(s.values)
	if err != nil {
		return err
	}
	if err := f.EnableScan(scanCfg, clamdCfg, audit, s.deps.Clock, s.logger()); err != nil {
		return err
	}
	imgCfg, err := ImageConfig(s.values)
	if err != nil {
		return err
	}
	return f.EnableImages(imgCfg, s.logger(), s.deps.FilesImageHooks)
}

// imageTimeout is file.image.processing_timeout, the River timeout of files.thumbnail in every
// role (M3-06).
func (s *server) imageTimeout() time.Duration {
	return s.values.Duration("file.image.processing_timeout")
}

// enableDownloads binds the download keys and the ticket key source and adds the downloads to f.
func (s *server) enableDownloads(ctx context.Context, f *Files) error {
	cfg, err := DownloadConfig(s.values)
	if err != nil {
		return err
	}
	keys, err := NewTicketKeys(ctx, s.sp, s.values.String(keyTicketKeyRef))
	if err != nil {
		return err
	}
	s.mu.Lock()
	id := s.identity
	s.mu.Unlock()
	return f.EnableDownloads(cfg, id, keys)
}

// startScanHealth starts the health monitor of the virus scanner in the roles worker and all
// (M3-04, 12 5.3): a loop that pings clamd every file.scan.health_interval until the stop step
// scan_health ends it. It never changes readiness.
func (s *server) startScanHealth(context.Context) error {
	s.mu.Lock()
	f := s.files
	s.mu.Unlock()
	if f == nil || f.Health == nil {
		return errors.New("wire: the scan health step needs the scan pass of the files step")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.Health.Run(ctx)
	}()
	s.runner.OnStop(lifecycle.StopStep{Name: "scan_health", Phase: lifecycle.PhaseJobs, Run: func(stop context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stop.Done():
			return stop.Err()
		}
	}})
	return nil
}
