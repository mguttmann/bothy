// SPDX-License-Identifier: Apache-2.0

// Package appsworld is the integration world of the module apps (M2-24): one instance on
// PostgreSQL 18 with the real identity, authz, audit, sync and apps behind the full HTTP chain.
// The tests of internal/apps/app, internal/apps/adapters/httpapi and
// internal/apps/adapters/postgres share it, so each package is measured by the tests that run
// through it. Only _test.go files import it.
package appsworld

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
	appshttp "github.com/OWNER/bothy/services/server/internal/apps/adapters/httpapi"
	appspg "github.com/OWNER/bothy/services/server/internal/apps/adapters/postgres"
	appsapp "github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzhttp "github.com/OWNER/bothy/services/server/internal/authz/adapters/httpapi"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	chatapp "github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/egress"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/sync"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// worldStart is the clock of the app world.
var worldStart = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

// scopeMessagesWrite is the scope Tracker requires.
const scopeMessagesWrite = "messages:write"

// AllowedHost is webhook.allowed_hosts of the test configuration (M2-24 "Integration").
const AllowedHost = "hooks.example.com"

// IPHost and LongHost are allowed hosts that are no audit code: an address literal and a name
// longer than an audit code may be.
const (
	IPHost   = "192.0.2.10"
	LongHost = "receiver-with-a-rather-long-name.of-the-load-balancer.hooks.example.com"
)

// idCfg is the identity configuration of the world.
var idCfg = identityapp.Config{
	HandleMaxChars: 32, DisplayNameMaxChars: 64, DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: time.Minute,
	ClockLeeway: 30 * time.Second, PublicURL: httpservertest.PublicURL, PageSizeDefault: 2, PageSizeMax: 3,
	StepUpMaxAge: 5 * time.Minute, AccessTokenTTL: 10 * time.Minute, RefreshTokenTTL: 14 * 24 * time.Hour,
	SessionMaxAge: 30 * 24 * time.Hour, ConsoleSessionMaxAge: 12 * time.Hour,
	ConsoleSessionIdleTimeout: 30 * time.Minute, ViewAsMaxDuration: 15 * time.Minute, RefreshReuseGrace: 30 * time.Second, LastActiveGranularity: 5 * time.Minute,
	StepUpChallengeTTL: 2 * time.Minute, StepUpFailLimit: 3, LoginFailWindow: 15 * time.Minute, CleanupInterval: time.Hour,
	APITokenMaxTTL: 365 * 24 * time.Hour, PersonalTokenMaxTTL: 90 * 24 * time.Hour, DPoPRequiredFor: []string{"agent"},
	// The keys the newer base requires of NewIdentity (INT-W19/8), as in terminalworld.
	EnrollmentCodeTTL: 72 * time.Hour, EnrollmentCodeMaxAttempts: 5, LoginFlowTTL: 10 * time.Minute,
	DevicePendingTTL: 24 * time.Hour, DeviceVerificationCodeDigits: 6, PairingCodeTTL: 5 * time.Minute, PairingCodeChars: 8,
	ServerCodeTTL: time.Minute, OIDCJWKSCacheTTL: time.Hour, OIDCClientAssertionTTL: 2 * time.Minute,
	IDPRecheckInterval: 30 * 24 * time.Hour, DeviceInactiveRevokeAfter: 180 * 24 * time.Hour,
}

// appsCfg is the configuration of the module apps in the world.
var appsCfg = appsapp.Config{
	ManifestMaxBytes: 65536, AllowedHosts: []string{AllowedHost, IPHost, LongHost}, CommandMax: 100, CommandDescriptionMaxChars: 100,
	HandleMaxChars: 32, DisplayNameMaxChars: 64, ReasonMaxChars: 500, PageSizeDefault: 2, PageSizeMax: 3,
}

// chatConfig is the configuration of the chat use cases of the world (the keys NewChat requires).
var chatConfig = chatapp.Config{
	SpaceNameMaxChars: 40, ChannelNameMaxChars: 40, ChannelTopicMaxChars: 200, TagNameMaxChars: 20,
	HistoryBackfillLimitDefault: 20, HistoryBackfillLimitMax: 200, ThreadTitleMaxChars: 200, ThreadMaxTags: 5,
	PlaceholderTitles: []string{"New chat", "Untitled"}, PageSizeDefault: 20, PageSizeMax: 50,
	ProjectBindingTextMaxChars: 64, ReasonMaxChars: 200,
}

// testPepper is the token pepper of the world (never a secret).
var testPepper = []byte("0123456789abcdef0123456789abcdef")

// testKEK is the key-encryption key of the audit IP keys in the world.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// World is one instance on PostgreSQL 18 with the real identity (with the installations of apps),
// authz with a fake structure source (Support with Allgemein and Eskalation), audit, sync, the
// signing keys and the module apps behind the full HTTP chain with the Idempotency-Key stage and
// the response validator.
type World struct {
	t     *testing.T
	db    *pgtest.DB
	pool  *pg.Pool
	owner *pg.Pool
	uow   *pg.UnitOfWork
	rec   *auditapp.Recorder
	// Clk is the fake clock of the world.
	Clk       *clock.Fake
	id        *wire.Identity
	apps      *wire.Apps
	az        authorize.Authorizer
	admin     *authzapp.AdminService
	structure *fake.Structure
	h         *httpservertest.Harness
	appender  *syncapp.Appender
	consumers *syncapp.ConsumerService
	web       webhookParts
	// Support with its channels Allgemein and Eskalation and their main threads (07 section 11.1).
	Support, Allgemein, Eskalation    ids.ID
	AllgemeinThread, EskalationThread ids.ID
	// Robin is an owner; Sam and Alex are members.
	Robin, Sam, Alex ids.ID
	// RobinPhone and SamPhone are logged-in phones of Robin and Sam.
	RobinPhone, SamPhone *Client
	logs                 *httpservertest.LogCapture
}

// NewWorld builds a world on a new database with Robin (owner), Sam and Alex.
func NewWorld(t *testing.T) *World { return NewWorldWith(t, Options{}) }

// NewWorldWith builds a world with the options o.
func NewWorldWith(t *testing.T, o Options) *World {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 16
	pool := db.OpenPool(t, cfg)
	capture := &httpservertest.LogCapture{}
	log := slog.New(capture)
	clk := clock.NewFake(worldStart)
	w := &World{t: t, db: db, pool: pool, owner: db.Pool(t, pg.RoleOwner), Clk: clk, logs: capture}
	installs := wire.NewAppInstallations(pool)
	repo := identitypg.NewRepository(pool)
	principals := identityapp.NewPrincipalSource(repo).WithApps(installs)
	w.structure = fake.NewStructure()
	policies := fake.NewPolicies()
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: w.structure, Policies: policies}, clk, log)
	Must(t, err)
	mods := wire.NewModules(pool, log)
	reg := txn.NewRegistry()
	for _, h := range []struct {
		p txn.Phase
		f txn.Hook
	}{{txn.PhaseRightsBegin, hook.Begin}, {txn.PhaseRightsCheck, hook.BeforeCommit}, {txn.PhaseSync, mods.Sync.Run}, {txn.PhaseAudit, mods.Audit.Hook}} {
		Must(t, reg.SetHook(h.p, h.f))
	}
	reg.Seal()
	w.uow = pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.rec, err = auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: w.uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: clk, Reporter: logs.New(log)})
	Must(t, err)
	appender, err := syncapp.NewAppender(syncapp.AppenderConfig{PayloadMaxBytes: 65536}, telemetry.Correlator{})
	Must(t, err)
	w.appender = appender
	Must(t, hook.Bind(authzapp.RightsBindings{Events: appender, Audit: w.rec}))
	maint, err := syncapp.NewPartitionMaintenance(syncapp.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		w.uow, syncpg.NewRepository(pool, log), clk)
	Must(t, err)
	_, err = maint.Run(context.Background())
	Must(t, err)
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: idCfg.StepUpMaxAge}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: identityapp.NewCredentialSource(repo, repo, repo), Structure: w.structure, Policies: policies,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: w.rec, Clock: clk, Log: log,
	})
	Must(t, err)
	w.az = az
	w.admin, err = authzapp.NewAdminService(authzapp.AdminDeps{UoW: w.uow, Authorizer: az, Store: authzpg.NewRepository(pool), Principals: principals,
		Structure: w.structure, Rights: hook, Audit: w.rec, Clock: clk, Events: appender, Scopes: az, Cursors: authzhttp.Cursors{},
		Config: authzapp.AdminConfig{GroupNameMaxChars: 100, GroupDescriptionMaxChars: 500, PageSizeDefault: 50, PageSizeMax: 200}})
	Must(t, err)
	dir := filepath.Join(t.TempDir(), "signing-keys")
	Must(t, os.Mkdir(dir, 0o700))
	writeMasterKey(t, filepath.Dir(dir))
	sp, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(dir), SigningKeyDir: dir, CacheTTL: time.Minute})
	Must(t, err)
	keys, err := wire.NewSigningKeys(wire.SigningKeyConfig{Keys: instanceapp.KeysConfig{
		CacheTTL: time.Minute, InvocationRotation: 30 * 24 * time.Hour, AccessTokenRotation: 30 * 24 * time.Hour,
		JWKSMaxAge: time.Hour, AccessTokenTTL: idCfg.AccessTokenTTL, ClockLeeway: idCfg.ClockLeeway,
	}, Dir: dir}, pool, sp, w.uow, az, w.rec, clk, log)
	Must(t, err)
	Must(t, keys.Keys.EnsureActive(context.Background()))
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, w.uow, az, wire.SettingsWrites{Audit: w.rec, Events: appender}, clk)
	Must(t, err)
	w.id, err = wire.NewIdentity(idCfg, pool, w.uow, az, wire.IdentityWrites{Audit: w.rec, Events: appender, Apps: installs,
		Settings: settings.Admin, Flags: settings.Admin}, testPepper, clk, log)
	Must(t, err)
	Must(t, w.id.BindSessions(keys.Keys, ids.New()))
	w.buildApps(o, pool, hook, az, keys.Keys, sp, log)
	built, err := routes.Build(wire.Handlers{Sessions: w.id.SessionHandlers, StepUps: w.id.StepUpHandlers, Devices: w.id.Handlers, People: w.id.PeopleHandlers,
		Providers: w.id.ProviderHandlers, Credentials: w.id.CredentialHandlers, Tokens: w.id.TokenHandlers, Apps: w.apps.Handlers,
		Webhooks: w.apps.WebhookHandlers, Keys: keys.Handlers},
		wire.RouteOptions(), routes.SpecCatalog())
	Must(t, err)
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(pool), w.uow), w.uow, clk)
	Must(t, err)
	w.h = httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: w.id.Authenticator, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
	w.Seed()
	return w
}

// buildApps builds the module apps with the outgoing webhooks of the world: two TLS receivers,
// the fake resolver, the egress client with the fake metrics, the master key in the secrets
// directory, River's tables with an insert-only runtime, and the worker consumer webhooks with
// its cursor at the head.
func (w *World) buildApps(o Options, pool *pg.Pool, hook *authzapp.RightsHook, az authorize.Authorizer, keys appsapp.Signers,
	sp *secrets.Provider, log *slog.Logger) {
	t := w.t
	cert, roots := testCertificate(t)
	w.web.Hooks, w.web.Alt = newReceiver(t, HooksHost, cert), newReceiver(t, AltHost, cert)
	w.web.Resolver = &Resolver{m: map[string][]netip.Addr{}}
	w.web.Resolver.Set(HooksHost, "127.0.0.1")
	w.web.Resolver.Set(AltHost, "127.0.0.1")
	w.web.Metrics = &Metrics{n: map[egress.Reason]int{}}
	wcfg := WebhookConfig()
	wcfg.AllowedHosts = append(append([]string{}, appsCfg.AllowedHosts...),
		HooksHost+":"+w.web.Hooks.Port(), AltHost+":"+w.web.Alt.Port())
	if o.Webhook != nil {
		o.Webhook(&wcfg)
	}
	w.web.Config = wcfg
	cfg := appsCfg
	cfg.AllowedHosts = wcfg.AllowedHosts
	sender, err := wire.NewWebhookSender(egressConfig(t, o, w.web.Hooks, w.web.Alt, wcfg.DeliveryTimeout), wcfg,
		egress.Deps{Resolver: w.web.Resolver, RootCAs: roots, Logger: log, Metrics: w.web.Metrics})
	Must(t, err)
	master := wire.NewHMACMasterKey(sp, "file:<secrets.dir>/webhook_hmac_master_key")
	Must(t, master.Check(context.Background()))
	syncRepo := syncpg.NewRepository(pool, log)
	// The instance row with its sync_epoch, as bothy migrate up writes it (M1-15).
	w.SQL(`INSERT INTO instance (id, instance_id, schema_version, sync_epoch) VALUES (1, $1, 1, $2) ON CONFLICT (id) DO NOTHING`,
		ids.New().String(), ids.New().String())
	repo := appspg.NewRepository(pool)
	w.web.deps = appsapp.WebhookDeps{Store: repo, Repo: repo, UoW: w.uow, Authorizer: az, Audit: w.rec, Sender: sender, Master: master,
		Keys: keys, State: instanceapp.NewState(instancepg.NewRepository(pool), w.uow), Log: syncRepo, Cursors: appshttp.Cursors{},
		Clock: w.Clk, Logger: log}
	// Chat and messaging serve the incoming webhooks the module builds (M2-27); the tests of this
	// world reach them only through uninstall, which revokes the webhooks of the app.
	rights, ok := az.(authz.RightsQuery)
	if !ok {
		t.Fatal("appsworld: the Authorizer is no rights query")
	}
	ch, err := wire.NewChat(chatConfig, pool, w.uow, az, rights, wire.ChatWrites{Audit: w.rec, Events: w.appender, Rights: hook,
		Directory: w.id.Directory}, nil, w.Clk)
	Must(t, err)
	msgs, err := wire.NewMessaging(messagingapp.Config{MaxContentBytes: 4096, MaxMentions: 4, EventPayloadMaxBytes: 65536,
		TemplateNameMaxChars: 64, TemplateMaxPerScope: 100, ListContentMaxBytes: 4096, PageSizeDefault: 20, PageSizeMax: 50,
		EmojiMaxBytes: 64, MaxDistinctReactions: 20, HandleMaxChars: 32, DisplayNameMaxChars: 64, MaxPinsPerThread: 50},
		pool, w.uow, az, wire.MessagingWrites{Events: w.appender, Redactor: syncpg.NewRedactor(pool), Audit: w.rec}, ch.Service, w.id.Directory, w.Clk, log)
	Must(t, err)
	// The commands of a manifest go through the command register of agents (M2-25); the world has
	// no runs, so the components of an app's cards are disabled by noComponents, which finds none
	// (no test of this world posts a card).
	cmds, err := agentsapp.NewCommands(agentsapp.CommandConfig{MaxPerPrincipal: appsCfg.CommandMax,
		DescriptionMaxChars: appsCfg.CommandDescriptionMaxChars, MaxChoices: 25, EventPayloadMaxBytes: 65536},
		agentsapp.CommandDeps{Store: agentspg.NewRepository(pool), UoW: w.uow, Authorizer: az, Audit: w.rec, Events: w.appender,
			Principals: w.id.Principals, Channels: ch.Service})
	Must(t, err)
	w.apps, err = wire.NewApps(cfg, pool, wire.AppsDeps{UoW: w.uow, Rights: hook, Authorizer: az, Audit: w.rec, Identity: w.id, Grants: w.admin,
		Clock: w.Clk, Webhooks: wire.WebhookDeps{Config: wcfg, Keys: keys, State: instanceapp.NewState(instancepg.NewRepository(pool), w.uow),
			Log: syncRepo, Master: master, Sender: sender, Logger: log},
		Incoming: appsapp.IncomingConfig{MaxContentBytes: 4096, FormatsEnabled: []string{"native", "alertmanager"},
			LastUsedGranularity: idCfg.LastActiveGranularity, PublicURL: httpservertest.PublicURL},
		Chat: ch, Messages: msgs, Commands: cmds, Components: noComponents{}})
	Must(t, err)
	w.migrateJobs()
	w.apps.BindJobs(w.newRuntime(jobs.RoleAPI).Inserter())
	w.consumers, err = syncapp.NewConsumerService(syncapp.ConsumerConfig{BatchSize: 100, PollInterval: time.Second,
		ReconnectBackoffCap: time.Second, StatesTimeout: 5 * time.Second}, w.uow, syncRepo, syncRepo, log)
	Must(t, err)
	Must(t, w.consumers.Register(sync.ConsumerWebhooks, w.apps.Consumer()))
	Must(t, w.consumers.Ensure(context.Background()))
}

// noComponents is the component port of apps in a world without runs: there are no cards.
type noComponents struct{}

// DisableAuthoredBy implements appsapp.Components: nothing to disable.
func (noComponents) DisableAuthoredBy(context.Context, ids.ID) (int, error) { return 0, nil }

// Must fails t on err.
func Must(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Seed creates Robin (owner), Sam and Alex with phones and the structure of 07 section 11.1.
func (w *World) Seed() {
	w.Robin, w.Sam, w.Alex = w.Person("robin", true), w.Person("sam", false), w.Person("alex", false)
	w.RobinPhone, w.SamPhone = w.Phone(w.Robin), w.Phone(w.Sam)
	w.Support, w.Allgemein, w.Eskalation = ids.New(), ids.New(), ids.New()
	w.structure.PutSpace(w.Support)
	// The grants of the world need the rows of the space and its channels since the foreign keys
	// of M1-52 and M1-53 (INT-W19/8); authz still reads the structure from the fake.
	w.SQL(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1::uuid, 'Support', 'a0', $2)`, w.Support.String(), w.Robin.String())
	for i, c := range []ids.ID{w.Allgemein, w.Eskalation} {
		w.structure.PutChannel(authz.ChannelFacts{ID: c, SpaceID: w.Support, Kind: authz.ChannelText})
		w.SQL(`INSERT INTO channels (id, space_id, kind, name, sort_key, operator_inherit, history_backfill_limit, created_by)
			VALUES ($1::uuid, $2::uuid, 'text', $1::text, $3, true, 20, $4)`, c.String(), w.Support.String(), fmt.Sprintf("a%d", i), w.Robin.String())
	}
	w.AllgemeinThread, w.EskalationThread = ids.New(), ids.New()
	w.structure.PutThread(authz.ThreadFacts{ID: w.AllgemeinThread, ChannelID: w.Allgemein, Main: true})
	w.structure.PutThread(authz.ThreadFacts{ID: w.EskalationThread, ChannelID: w.Eskalation, Main: true})
}

// SQL runs stmt as the database owner.
func (w *World) SQL(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.owner.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// Scalar scans the one value of query into dst.
func (w *World) Scalar(dst any, query string, args ...any) {
	w.t.Helper()
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(dst); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
}

// Count returns the integer result of query.
func (w *World) Count(query string, args ...any) int {
	w.t.Helper()
	var n int
	w.Scalar(&n, query, args...)
	return n
}

// Version returns the rights version.
func (w *World) Version() int64 {
	w.t.Helper()
	var v int64
	w.Scalar(&v, `SELECT value FROM sequence_counters WHERE name = 'permission_version'`)
	return v
}

// Person inserts an active person, an owner when owner is true.
func (w *World) Person(handle string, owner bool) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.SQL(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, id.String(), handle)
	w.SQL(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		w.SQL(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	return id
}

// Client is a person on an active phone with a step_up key.
type Client struct {
	w                     *World
	key, step             *identitytest.Key
	user, device, session ids.ID
	access                string
}

// Phone logs user in on an active phone with a step_up key.
func (w *World) Phone(user ids.ID) *Client {
	w.t.Helper()
	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
	stepKey, stepPub := identitytest.NewDeviceKey(jose.AlgES256)
	reg := identity.RegisterDevice{DeviceID: ids.New(), UserID: user, Name: "Phone", Platform: identity.PlatformIOS, DPoPKey: pub,
		State: identity.DeviceStateActive, StepUpKey: &stepPub, ApprovedBy: &user}
	dev, err := w.id.Devices.Register(context.Background(), reg)
	Must(w.t, err)
	set, err := w.id.Sessions.StartSession(context.Background(), identity.StartSession{UserID: user, DeviceID: dev.ID, AuthMethod: identity.AuthPasskey})
	Must(w.t, err)
	return &Client{w: w, key: key, step: stepKey, user: user, device: dev.ID, session: set.Session.ID, access: set.AccessToken.Reveal()}
}

// Call sends a request with the access token, a fresh proof and the headers (name, value pairs).
func (c *Client) Call(method, path, body string, headers ...string) *httpservertest.Response {
	c.w.t.Helper()
	proof := identitytest.SignDPoPProof(c.key, identitytest.ProofClaims{HTM: method, HTU: httpservertest.PublicURL + path, IAT: c.w.Clk.Now(),
		Nonce: c.w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(c.access)})
	h := append([]string{"Authorization", "DPoP " + c.access, "DPoP", proof}, headers...)
	return c.w.h.Do(c.w.h.NewRequest(method, path, body, h...))
}

// StepUp runs a full step-up through the API.
func (c *Client) StepUp() *Client {
	c.w.t.Helper()
	m := c.w.Want(c.Call(http.MethodPost, "/api/v1/auth/step-up/challenge", ""), http.StatusOK, "")
	body, err := json.Marshal(map[string]any{"challenge": m["challenge"], "sid": c.session.String(),
		"iat": json.Number(strconv.FormatInt(c.w.Clk.Now().Unix(), 10))})
	Must(c.w.t, err)
	sig := identitytest.SignJWS(c.step, jose.Header{Alg: jose.AlgES256}, body)
	confirm, _ := json.Marshal(map[string]string{"challenge_id": m["challenge_id"].(string), "signature": sig})
	c.w.Want(c.Call(http.MethodPost, "/api/v1/auth/step-up/device-key", string(confirm)), http.StatusOK, "")
	return c
}

// Actor authenticates the current access token of c the way the chain does.
func (c *Client) Actor() actor.Actor {
	c.w.t.Helper()
	a, err := c.w.id.Authenticator.Authenticate(context.Background(), actor.Credentials{
		Method: http.MethodGet, HTU: httpservertest.PublicURL + "/api/v1/me", Authorization: "DPoP " + c.access,
		DPoP: identitytest.SignDPoPProof(c.key, identitytest.ProofClaims{HTM: http.MethodGet, HTU: httpservertest.PublicURL + "/api/v1/me",
			IAT: c.w.Clk.Now(), Nonce: c.w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(c.access)}),
	})
	Must(c.w.t, err)
	return a
}

// Idem returns the header pair of a fresh Idempotency-Key.
func Idem() []string { return []string{"Idempotency-Key", ids.New().String()} }

// Bearer sends a request with an API token as Bearer.
func (w *World) Bearer(token, method, path, body string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	return w.h.Do(w.h.NewRequest(method, path, body, append([]string{"Authorization", "Bearer " + token}, headers...)...))
}

// TokenActor authenticates an API token the way the chain does.
func (w *World) TokenActor(token string) actor.Actor {
	w.t.Helper()
	a, err := w.id.Authenticator.Authenticate(context.Background(), actor.Credentials{Authorization: "Bearer " + token})
	Must(w.t, err)
	return a
}

// Want checks status and, for a problem, its code; it returns the decoded body. A success body
// never holds JSON null.
func (w *World) Want(r *httpservertest.Response, status int, code string) map[string]any {
	w.t.Helper()
	if r.StatusCode != status {
		w.t.Fatalf("status %d, want %d: %s", r.StatusCode, status, r.Body)
	}
	var m map[string]any
	if code != "" {
		m = r.Problem(w.t)
		if m["code"] != code {
			w.t.Fatalf("code %v, want %s: %s", m["code"], code, r.Body)
		}
		return m
	}
	if len(r.Body) > 0 {
		if strings.Contains(string(r.Body), "null") {
			w.t.Errorf("JSON null in %s", r.Body)
		}
		Must(w.t, json.Unmarshal(r.Body, &m))
	}
	return m
}

// WantReason checks status, code and reason of a problem.
func (w *World) WantReason(r *httpservertest.Response, status int, code, reason string) {
	w.t.Helper()
	m := w.Want(r, status, code)
	if m["reason"] != reason {
		w.t.Fatalf("reason %v, want %s: %s", m["reason"], reason, r.Body)
	}
}

// WantField checks a 400 problem with code and a field error at pointer with fieldCode.
func (w *World) WantField(r *httpservertest.Response, code, pointer, fieldCode string) {
	w.t.Helper()
	m := w.Want(r, http.StatusBadRequest, code)
	errsList, _ := m["errors"].([]any)
	for _, e := range errsList {
		f := e.(map[string]any)
		if f["pointer"] == pointer && f["code"] == fieldCode {
			return
		}
	}
	w.t.Fatalf("no %s at %s: %s", fieldCode, pointer, r.Body)
}

// AuditRow is one row of audit_events.
type AuditRow struct {
	Decision, Reason, ResourceID, Raw string
	Details                           map[string]any
}

// Audits returns the audit rows of action, oldest first, after flushing the aggregated denials.
func (w *World) Audits(action string) []AuditRow {
	w.t.Helper()
	Must(w.t, w.rec.Aggregator().Flush(context.Background()))
	rows, err := w.owner.Query(context.Background(), `SELECT decision, coalesce(deny_reason, ''), coalesce(resource_id::text, ''), details::text
		FROM audit_events WHERE action = $1 ORDER BY seq`, action)
	Must(w.t, err)
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var r AuditRow
		Must(w.t, rows.Scan(&r.Decision, &r.Reason, &r.ResourceID, &r.Raw))
		Must(w.t, json.Unmarshal([]byte(r.Raw), &r.Details))
		out = append(out, r)
	}
	return out
}

// Allows returns the allow rows of action.
func (w *World) Allows(action string) []AuditRow {
	var out []AuditRow
	for _, r := range w.Audits(action) {
		if r.Decision == "allow" {
			out = append(out, r)
		}
	}
	return out
}

// AccessChanged returns the payloads of the access.changed events of principal after seq.
func (w *World) AccessChanged(principal ids.ID, after int64) []map[string]any {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT payload::text FROM event_log WHERE type = 'access.changed'
		AND audience_principal_id = $1 AND seq > $2 ORDER BY seq`, principal.String(), after)
	Must(w.t, err)
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var p string
		Must(w.t, rows.Scan(&p))
		var m map[string]any
		Must(w.t, json.Unmarshal([]byte(p), &m))
		out = append(out, m)
	}
	return out
}

// LastSeq is the highest seq of event_log.
func (w *World) LastSeq() int64 {
	w.t.Helper()
	var n int64
	w.Scalar(&n, `SELECT coalesce(max(seq), 0) FROM event_log`)
	return n
}

// Manifest returns a manifest of slug at version with the required and optional scopes; mods
// change it before encoding.
func Manifest(slug, version string, required, optional []string, mods ...func(m map[string]any)) map[string]any {
	m := map[string]any{
		"schema_version": 1, "slug": slug, "name": "App " + slug, "description": "Mirrors tickets of a help desk.", "publisher": "Example Org",
		"version": version, "min_api_version": "0.1",
		"scopes": map[string]any{"required": OrEmpty(required), "optional": OrEmpty(optional)},
	}
	for _, f := range mods {
		f(m)
	}
	return m
}

// OrEmpty is list or an empty list.
func OrEmpty(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// InstallBody returns the JSON of AppInstall; subs nil leaves event_subscriptions out.
func InstallBody(m map[string]any, granted []string, subs []string) string {
	b := map[string]any{"manifest": m, "granted_scopes": OrEmpty(granted), "reason": "Sync tickets with the help desk"}
	if subs != nil {
		b["event_subscriptions"] = subs
	}
	raw, _ := json.Marshal(b)
	return string(raw)
}

// Install installs m with granted through the API as owner o and returns the App.
func (w *World) Install(o *Client, m map[string]any, granted []string) map[string]any {
	w.t.Helper()
	return w.Want(o.Call(http.MethodPost, "/api/v1/apps", InstallBody(m, granted, nil), Idem()...), http.StatusCreated, "")
}

// Token creates an app token with the granted scopes as owner o and returns its secret.
func (w *World) Token(o *Client, app string) string {
	w.t.Helper()
	raw, _ := json.Marshal(map[string]any{"name": "Sync", "expires_at": w.Clk.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339Nano)})
	m := w.Want(o.Call(http.MethodPost, "/api/v1/principals/"+app+"/tokens", string(raw), Idem()...), http.StatusCreated, "")
	return m["secret"].(string)
}

// Grant creates a grant for the app through authz.Admin as Robin (tier 1 of M2-24 design note 17).
func (w *World) Grant(app ids.ID, target ids.ID, level string) ids.ID {
	w.t.Helper()
	robin := w.RobinPhone.Actor()
	g, err := w.admin.CreateGrant(actor.WithActor(context.Background(), robin), robin, authz.CreateGrantCmd{
		SubjectKind: "app", SubjectID: app, TargetKind: "channel", TargetID: target, Level: level, Reason: "Posts per ticket",
	})
	Must(w.t, err)
	return g.ID
}

// Decide authorizes act on r for a with the real authorizer.
func (w *World) Decide(a actor.Actor, act authorize.Action, r authorize.Resource) authorize.Decision {
	w.t.Helper()
	d, err := w.az.Authorize(context.Background(), a, act, r)
	Must(w.t, err)
	return d
}

// Logged reports whether s appears in a message or attribute of a captured log line of the
// modules or of the chain.
func (w *World) Logged(s string) bool {
	for _, l := range []*httpservertest.LogCapture{w.logs, w.h.Logs} {
		for _, r := range l.Records() {
			if strings.Contains(r.Message, s) {
				return true
			}
			for _, v := range r.Attrs {
				if strings.Contains(v, s) {
					return true
				}
			}
		}
	}
	return false
}

// TrackerManifest is app Tracker (slug tracker) at version with messages:write required.
func TrackerManifest(version string, mods ...func(m map[string]any)) map[string]any {
	return Manifest("tracker", version, []string{scopeMessagesWrite}, nil, mods...)
}

// EqualList reports a JSON array of exactly want.
func EqualList(v any, want ...string) bool {
	list, ok := v.([]any)
	if !ok {
		return false
	}
	got := make([]string, 0, len(list))
	for _, x := range list {
		s, ok := x.(string)
		if !ok {
			return false
		}
		got = append(got, s)
	}
	return slices.Equal(got, want)
}

// ForeignHostManifest names a delivery host outside webhook.allowed_hosts.
func ForeignHostManifest() map[string]any {
	return Manifest("tracker", "1.0.0", []string{scopeMessagesWrite}, []string{scopeMessagesRead, scopeEventsSubscribe}, func(m map[string]any) {
		m["events"] = []any{typeMessageCreated}
		m["delivery"] = map[string]any{"mode": "webhook", "events_url": "https://hooks.other.example.com/events"}
		m["data_egress"] = "Messages of the granted channels go to the help desk."
	})
}

// ScopesBody is the JSON of AppScopesPut; subs nil leaves event_subscriptions out.
func ScopesBody(granted []string, subs []string) string {
	b := map[string]any{"granted_scopes": OrEmpty(granted), "reason": "Consent of the owner"}
	if subs != nil {
		b["event_subscriptions"] = subs
	}
	raw, _ := json.Marshal(b)
	return string(raw)
}

// ThreadOf is the main thread of a channel as a resource.
func ThreadOf(thread ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceThread, ID: thread}
}

// MessageIn is a message in thread as a resource.
func MessageIn(thread ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceMessage, ID: ids.New(), ThreadID: thread}
}

// TargetIn reports a target with id in a list of access targets.
func TargetIn(v any, id string) bool {
	list, _ := v.([]any)
	for _, x := range list {
		if m, ok := x.(map[string]any); ok && m["id"] == id {
			return true
		}
	}
	return false
}

// TrackerWithG6 installs Tracker with messages:write, grants G-6 (write on Support/Eskalation)
// and returns the world and a token of Tracker.
func TrackerWithG6(t *testing.T) (*World, string) {
	t.Helper()
	w := NewWorld(t)
	o := w.RobinPhone.StepUp()
	app := w.Install(o, TrackerManifest("1.0.0"), []string{scopeMessagesWrite})
	w.Grant(ids.MustParse(app["id"].(string)), w.Eskalation, "write")
	return w, w.Token(o, app["id"].(string))
}
