// SPDX-License-Identifier: Apache-2.0

package agentsessions_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
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
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// transitions records the committed transitions of runs (app.Metrics).
type transitions struct {
	mu   sync.Mutex
	list []runsapp.Transitioned
}

// RunTransitioned implements runsapp.Metrics.
func (r *transitions) RunTransitioned(_ context.Context, t runsapp.Transitioned) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.list = append(r.list, t)
}

// CostBooked implements runsapp.Metrics; these tests book no costs.
func (r *transitions) CostBooked(context.Context, runsapp.BookedCost) {}

// sessionRefMaxBytes is agent.session_ref_max_bytes of the world.
const sessionRefMaxBytes = 64

// start is the clock of the world.
var start = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// idConfig is the identity configuration of the world: the keys NewIdentity requires; no token
// kind is DPoP-bound, so an agent token works as a bearer token.
var idConfig = identityapp.Config{
	HandleMaxChars: 32, DisplayNameMaxChars: 64, DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: time.Minute,
	ClockLeeway: 30 * time.Second, PublicURL: httpservertest.PublicURL, PageSizeDefault: 20, PageSizeMax: 50,
	StepUpMaxAge: 5 * time.Minute, AccessTokenTTL: 10 * time.Minute, RefreshTokenTTL: 24 * time.Hour,
	SessionMaxAge: 30 * 24 * time.Hour, RefreshReuseGrace: 30 * time.Second, LastActiveGranularity: 5 * time.Minute,
	StepUpChallengeTTL: 2 * time.Minute, StepUpFailLimit: 3, LoginFailWindow: 15 * time.Minute, CleanupInterval: time.Hour,
	APITokenMaxTTL: 365 * 24 * time.Hour, PersonalTokenMaxTTL: 90 * 24 * time.Hour,
	EnrollmentCodeTTL: 72 * time.Hour, EnrollmentCodeMaxAttempts: 5, LoginFlowTTL: 10 * time.Minute,
	DevicePendingTTL: 24 * time.Hour, DeviceVerificationCodeDigits: 6, PairingCodeTTL: 5 * time.Minute, PairingCodeChars: 8,
	ServerCodeTTL: time.Minute, OIDCJWKSCacheTTL: time.Hour, OIDCClientAssertionTTL: 2 * time.Minute,
	IDPRecheckInterval: 30 * 24 * time.Hour, DeviceInactiveRevokeAfter: 180 * 24 * time.Hour,
}

// chatConfig is the configuration of the chat use cases of the world.
var chatConfig = chatapp.Config{
	SpaceNameMaxChars: 40, ChannelNameMaxChars: 40, ChannelTopicMaxChars: 200, TagNameMaxChars: 20,
	HistoryBackfillLimitDefault: 20, HistoryBackfillLimitMax: 200,
	ThreadTitleMaxChars: 200, ThreadMaxTags: 5,
	PlaceholderTitles: []string{"Neuer Chat", "New chat", "Chat", "Ohne Titel", "Untitled", "Test"},
	PageSizeDefault:   20, PageSizeMax: 50,
	ProjectBindingTextMaxChars: 64, ReasonMaxChars: 200,
}

// pepper is the token pepper of the world (a fixed test value, never a secret).
var pepper = []byte("0123456789abcdef0123456789abcdef")

// testKEK is the key-encryption key of the audit recorder.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// creds is the credential source of the world: sessions, devices and personal tokens from the
// fake (the test authenticator stands in for the login), agent tokens from identity.
type creds struct {
	fake *fake.Credentials
	real authz.CredentialSource
	mu   sync.Mutex
	own  map[ids.ID]bool
}

// Credentials implements authz.CredentialSource.
func (c *creds) Credentials(ctx context.Context, ref authz.CredentialRef) (authz.CredentialFacts, error) {
	c.mu.Lock()
	faked := c.own[ref.TokenID]
	c.mu.Unlock()
	if ref.TokenID.IsZero() || faked {
		return c.fake.Credentials(ctx, ref)
	}
	f, err := c.fake.Credentials(ctx, authz.CredentialRef{PrincipalID: ref.PrincipalID, SessionID: ref.SessionID, DeviceID: ref.DeviceID})
	if err != nil {
		return f, err
	}
	r, err := c.real.Credentials(ctx, authz.CredentialRef{PrincipalID: ref.PrincipalID, TokenID: ref.TokenID})
	f.TokenActive, f.TokenExpiresAt = r.TokenActive, r.TokenExpiresAt
	return f, err
}

// world is the cross-module world of M1-54: PostgreSQL 18 with the real identity, authz
// (Authorizer, rights hook with I1, Admin), agents, chat (structure source with served targets),
// audit and sync modules behind the full HTTP chain with response validation against
// openapi.yaml. No source is a fake; only the login is replaced by the test authenticator. The
// structure is the world of 07 section 10 and 11.1 ("Beispiele", "Testmatrix"), built through
// the API.
type world struct {
	t      *testing.T
	pool   *pg.Pool
	db     *pg.Pool
	rec    *auditapp.Recorder
	clk    *clock.Fake
	creds  *creds
	actors *httpservertest.Actors
	acts   map[string]actor.Actor
	h      *httpservertest.Harness
	az     *authzapp.Authorizer
	id     *wire.Identity
	admin  *authzapp.AdminService
	hook   *authzapp.RightsHook
	// structure is the production structure source of chat the hook and authorize read.
	structure authz.StructureSource
	// chat is the chat module; the tests of M2-03 call ApplyProjectReadiness and Threads.Create
	// on it the way M2-20 and messaging will.
	chat *chatapp.Service
	// uow is the unit of work of the modules.
	uow txn.UnitOfWork
	// azConfig and azDeps built az; newAuthorizer builds a second instance from them, the
	// Authorizer of another process (M1-62).
	azConfig authzapp.AuthorizerConfig
	azDeps   authzapp.AuthorizerDeps
	// runs is the runs module (M2-05): its acceptance is subscribed to MessagePosted, and the
	// lifecycle tests call Open and Transition on it.
	runs *runsapp.Service
	// sessions are the agent sessions of M2-12.
	sessions *agentsapp.SessionService
	// metrics records the committed transitions of runs.
	metrics *transitions
	// log captures the log of the modules.
	log *httpservertest.LogCapture
	// cancels records the jobs runs.cancel_timeout the runs module schedules (M2-09); keys and
	// issuer sign and name the invocation tokens.
	cancels *cancelJobs
	keys    *testKeys
	issuer  ids.ID

	owner, alex, sam, robin                    ids.ID
	support, marketing, engineering, research  ids.ID
	allgemein, eskalation, kampagnen, projektA ids.ID
	werkzeuge, notizen                         ids.ID
	lotse, texter, runner, assistent           ids.ID
	leser                                      ids.ID
	g1, g2, g3, g4, g5                         ids.ID
	versions                                   map[ids.ID]int64
	actorsMu                                   sync.Mutex
}

// newWorld builds the modules, the harness and the world of 07 section 10 and 11.1.
func newWorld(t *testing.T) *world {
	t.Helper()
	w := newEmptyWorld(t)
	w.build()
	return w
}

// newEmptyWorld builds the modules and the harness with only the owner and the language
// configuration of the chapter defaults (docs/spec/14-search.md section 15).
func newEmptyWorld(t *testing.T) *world {
	t.Helper()
	return newEmptyWorldWith(t, chapterSearch(t))
}

// chapterSearch is the language configuration of the chapter defaults: search.languages german and
// english, search.locale_map de and en.
func chapterSearch(t *testing.T) searchlang.Map {
	t.Helper()
	m, err := searchlang.New([]string{"german", "english"}, map[string]string{"de": "german", "en": "english"})
	must(t, err)
	return m
}

// newEmptyWorldWith builds the modules and the harness with only the owner and the language
// configuration search, which identity answers the language of new messages and threads with
// (M3-13).
func newEmptyWorldWith(t *testing.T, search searchlang.Map) *world {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 16
	pool := db.OpenPool(t, cfg)
	capture := &httpservertest.LogCapture{}
	log := slog.New(capture)
	w := &world{t: t, pool: pool, db: db.Pool(t, pg.RoleOwner), clk: clock.NewFake(start), actors: httpservertest.NewActors(),
		acts: map[string]actor.Actor{}, versions: map[ids.ID]int64{}, log: capture, metrics: &transitions{},
		cancels: &cancelJobs{}, issuer: ids.New()}
	repo := identitypg.NewRepository(pool)
	principals := identityapp.NewPrincipalSource(repo)
	w.creds = &creds{fake: fake.NewCredentials(), real: identityapp.NewCredentialSource(repo, repo, repo), own: map[ids.ID]bool{}}
	// The production sources: the structure of chat, the agent policies of agents, the hosts of
	// terminal.
	structure, policies, hosts := wire.NewStructureSource(pool), wire.NewAgentPolicySource(pool), wire.NewHostSource(pool)
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: structure, Policies: policies, Hosts: hosts}, w.clk, log)
	must(t, err)
	w.hook, w.structure = hook, structure
	mods := wire.NewModules(pool, log)
	reg := txn.NewRegistry()
	for _, h := range []struct {
		p txn.Phase
		f txn.Hook
	}{{txn.PhaseRightsBegin, hook.Begin}, {txn.PhaseRightsCheck, hook.BeforeCommit}, {txn.PhaseSync, mods.Sync.Run}, {txn.PhaseAudit, mods.Audit.Hook}} {
		must(t, reg.SetHook(h.p, h.f))
	}
	// The subscriptions of internal/wire in its order (M1-61, M1-62): drafts with clear_draft,
	// then the copies of channel_id on a thread move.
	must(t, txn.Subscribe(reg, mods.SyncSubscribers.OnMessagePosted))
	must(t, txn.Subscribe(reg, mods.MessagingSubscribers.OnThreadMoved))
	must(t, txn.Subscribe(reg, mods.SyncSubscribers.OnThreadMoved))
	// The acceptance of runs after the existing subscribers (M2-05, registerTxn), then the
	// revocation and the disabled agent (M2-09).
	must(t, txn.Subscribe(reg, mods.RunsAcceptor.OnMessagePosted))
	must(t, txn.Subscribe(reg, mods.RunsAcceptor.OnAccessChanged))
	must(t, txn.Subscribe(reg, mods.RunsAcceptor.OnAgentDisabled))
	// The sessions of M2-12 after AgentDisabled, as registerTxn orders them.
	must(t, txn.Subscribe(reg, mods.RunsAcceptor.OnSessionAttached))
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.uow = uow
	w.rec, err = auditapp.NewRecorder(withDefaults(t, wire.AuditConfig, auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}), auditapp.Deps{UoW: uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: w.clk, Reporter: logs.New(log)})
	must(t, err)
	appender, err := syncapp.NewAppender(syncapp.AppenderConfig{PayloadMaxBytes: 65536}, telemetry.Correlator{})
	must(t, err)
	must(t, hook.Bind(authzapp.RightsBindings{Events: appender, Audit: w.rec}))
	must(t, mods.SyncSubscribers.Bind(appender))
	maint, err := syncapp.NewPartitionMaintenance(syncapp.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		uow, syncpg.NewRepository(pool, log), w.clk)
	must(t, err)
	_, err = maint.Run(context.Background())
	must(t, err)
	w.azConfig = withDefaults(t, wire.AuthzConfig, authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: idConfig.StepUpMaxAge, ViewAsMaxDuration: 15 * time.Minute})
	w.azDeps = authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: w.creds, Structure: structure, Policies: policies, Hosts: hosts,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: w.rec, Clock: w.clk, Log: log,
	}
	w.az, err = authzapp.NewAuthorizer(w.azConfig, w.azDeps)
	must(t, err)
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, uow, w.az, wire.SettingsWrites{Audit: w.rec, Events: appender}, w.clk)
	must(t, err)
	writes := wire.IdentityWrites{Audit: w.rec, Events: appender, Settings: settings.Admin, Flags: settings.Admin, Search: search}
	w.id, err = wire.NewIdentity(identityConfig(t), pool, uow, w.az, writes, pepper, w.clk, log)
	must(t, err)
	ag, err := wire.NewAgents(withDefaults(t, wire.AgentsConfig, agentsapp.Config{PageSizeDefault: 20, PageSizeMax: 50, ReasonMaxChars: 200}), pool, uow, w.az, hook, writes, w.id, w.clk, log)
	must(t, err)
	w.admin, err = authzapp.NewAdminService(authzapp.AdminDeps{
		UoW: uow, Authorizer: w.az, Store: authzpg.NewRepository(pool), Principals: principals, Structure: structure, Hosts: hosts,
		Rights: hook, Audit: w.rec, Clock: w.clk, Events: appender, Scopes: w.az, Cursors: authzhttp.Cursors{},
		Config: withDefaults(t, wire.AuthzAdminConfig, authzapp.AdminConfig{GroupNameMaxChars: 100, GroupDescriptionMaxChars: 500, PageSizeDefault: 20, PageSizeMax: 50}),
	})
	must(t, err)
	c, err := wire.NewChat(withDefaults(t, wire.ChatConfig, chatConfig), pool, uow, w.az, w.az, wire.ChatWrites{Audit: w.rec, Events: appender, Rights: w.hook,
		Directory: w.id.Directory}, nil, w.clk)
	must(t, err)
	w.chat = c.Service
	// Messaging, threads and the instance settings serve the search tests of M3-13.
	// The template, list and page limits (M1-57, M1-60) and the write ports (M1-58) are required (INT-W22/3).
	m, err := wire.NewMessaging(withDefaults(t, wire.MessagingConfig, messagingapp.Config{MaxContentBytes: 4096, MaxMentions: 4, EventPayloadMaxBytes: 65536,
		TemplateNameMaxChars: 64, TemplateMaxPerScope: 100, ListContentMaxBytes: 4096, PageSizeDefault: 20, PageSizeMax: 50,
		EmojiMaxBytes: 64, MaxDistinctReactions: 20, MaxPinsPerThread: 50}),
		pool, uow, w.az, wire.MessagingWrites{Events: appender, Redactor: syncpg.NewRedactor(pool), Audit: w.rec}, c.Service, w.id.Directory, w.clk, nil)
	must(t, err)
	// The personal data of sync (read states and drafts, M1-61) serve the move tests of M1-62.
	personal, err := wire.NewPersonal(withDefaults(t, wire.PersonalConfig, syncapp.PersonalConfig{MaxContentBytes: 4096, MaxAttachments: 3, EventPayloadMaxBytes: 65536}),
		pool, uow, w.az, appender, c.Service, m.Service, w.clk)
	must(t, err)
	// The agent sessions of M2-12 with the runs implementation of agents.RunSource.
	ss, err := wire.NewAgentSessions(agentsapp.SessionConfig{SessionRefMaxBytes: sessionRefMaxBytes}, pool, uow, w.az,
		wire.SessionWrites{Audit: w.rec, Events: appender}, c.Service, c.Service, w.clk, log)
	must(t, err)
	w.sessions = ss.Service
	rn, err := wire.NewRuns(withDefaults(t, wire.RunsConfig, runsapp.Config{PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 256}), pool, uow, w.az, wire.RunsDeps{
		Events: appender, Messages: m.Service, Threads: c.Service, Channels: c.Service, Agents: ag.Service, Metrics: w.metrics,
		Expiry: noExpiry{}, Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Audit: w.rec, CancelTimeouts: w.cancels,
		Sessions: ss.Service,
	}, w.clk, log)
	must(t, err)
	must(t, mods.RunsAcceptor.Bind(rn.Service))
	w.runs = rn.Service
	// The invocation tokens of M2-06 over in-memory keys and a fixed issuer (M2-09).
	w.keys = newTestKeys(t)
	must(t, rn.BindInvocationTokens(tokenConfig, w.keys, fixedInstance{id: w.issuer}, w.clk))
	reads, err := wire.NewSyncReads(syncapp.CatchUpConfig{PageSize: 500, MaxEvents: 100000},
		wire.SyncReadDeps{Pool: pool, UoW: uow, Authorizer: w.az, Chat: c.Service, Messaging: m.Service, Personal: personal.Service, Agents: ag.Service,
			Directory: w.id.Directory, Flags: settings.Admin, Runs: rn.Service})
	must(t, err)
	built, err := routes.Build(wire.Handlers{Access: authzhttp.New(w.admin), Agents: ag.Handlers, Spaces: c.Handlers, Channels: c.ChannelHandlers,
		People: w.id.PeopleHandlers, Threads: c.ThreadHandlers, Behavior: c.BehaviorHandlers, Messages: m.Handlers, Admin: settings.Handlers, Personal: personal.Handlers,
		Runs: rn.Handlers, Sync: reads.Handlers, AgentSessions: ss.Handlers},
		wire.RouteOptions(), routes.SpecCatalog())
	must(t, err)
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(pool), uow), uow, w.clk)
	must(t, err)
	w.h = httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: w.actors, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
	w.owner = w.person("owner", true)
	return w
}

// build creates the world of 07 section 10 and 11.1 through the API: Alex, Sam and Robin; the
// spaces Support, Marketing, Engineering and Research with their channels; the agents Lotse
// (default operator of Support), Texter (default operator of Marketing), Runner (operator of
// Engineering/Projekt-A and Engineering/Werkzeuge) and Assistent (personal, holder the owner,
// default operator of Research); the group Leser with Sam; the grants G-1 to G-5. G-6 (the app
// Tracker) is left out: apps come with M2 (docs/DECISIONS.md M1-54/4).
func (w *world) build() {
	w.alex, w.sam, w.robin = w.person("alex", false), w.person("sam", false), w.person("robin", false)
	w.support, w.marketing, w.engineering, w.research = w.space("Support"), w.space("Marketing"), w.space("Engineering"), w.space("Research")
	w.allgemein, w.eskalation = w.channel(w.support, "text", "Allgemein"), w.channel(w.support, "text", "Eskalation")
	w.kampagnen = w.channel(w.marketing, "text", "Kampagnen")
	w.projektA, w.werkzeuge = w.channel(w.engineering, "forum", "Projekt-A"), w.channel(w.engineering, "text", "Werkzeuge")
	w.notizen = w.channel(w.research, "text", "Notizen")
	w.lotse, w.texter, w.runner, w.assistent = w.agent("lotse"), w.agent("texter"), w.agent("runner"), w.agent("assistent")
	w.wantOK(w.setSpaceOperator(w.support, `{"mode":"agent","agent_id":"`+w.lotse.String()+`","reason":"Support agent"}`))
	w.wantOK(w.setSpaceOperator(w.marketing, `{"mode":"agent","agent_id":"`+w.texter.String()+`","reason":"Marketing agent"}`))
	w.wantOK(w.setChannelOperator(w.projektA, `{"mode":"agent","agent_id":"`+w.runner.String()+`","reason":"Project runner"}`))
	w.wantOK(w.setChannelOperator(w.werkzeuge, `{"mode":"agent","agent_id":"`+w.runner.String()+`","reason":"Tools runner"}`))
	w.setPersonal(w.assistent, w.owner)
	w.wantOK(w.setSpaceOperator(w.research, `{"mode":"agent","agent_id":"`+w.assistent.String()+`","reason":"Own assistant"}`))
	w.g1 = w.grant(`"subject_kind":"user","subject_id":"` + w.alex.String() + `","target_kind":"space","target_id":"` + w.support.String() +
		`","level":"trigger","capabilities":["upload","approve_actions"],"reason":"Support lead"`)
	w.g2 = w.grant(`"subject_kind":"user","subject_id":"` + w.sam.String() + `","target_kind":"space","target_id":"` + w.marketing.String() +
		`","level":"trigger","capabilities":["upload"],"reason":"Marketing"`)
	w.g3 = w.grant(`"subject_kind":"user","subject_id":"` + w.robin.String() + `","target_kind":"channel","target_id":"` + w.projektA.String() +
		`","level":"trigger","capabilities":["upload"],"expires_at":"2026-12-31T00:00:00Z","reason":"Project work"`)
	g := w.want(w.do("owner", http.MethodPost, "/api/v1/groups", `{"name":"Leser"}`, key()...), http.StatusCreated, "")
	w.leser = ids.MustParse(g["id"].(string))
	w.want(w.do("owner", http.MethodPost, "/api/v1/groups/"+w.leser.String()+"/members", `{"user_id":"`+w.sam.String()+
		`","expires_at":"2026-11-30T00:00:00Z"}`, key()...), http.StatusCreated, "")
	w.g4 = w.grant(`"subject_kind":"group","subject_id":"` + w.leser.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"read","reason":"Readers"`)
	w.g5 = w.grant(`"subject_kind":"agent","subject_id":"` + w.lotse.String() + `","target_kind":"space","target_id":"` + w.support.String() +
		`","level":"write","capabilities":["upload"],"reason":"Support agent writes"`)
}

// must fails the test on err.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// sql runs stmt as the database owner (fixtures outside the use cases).
func (w *world) sql(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.db.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// count returns the integer result of query.
func (w *world) count(query string, args ...any) int64 {
	w.t.Helper()
	var n int64
	if err := w.db.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// text returns the text result of query.
func (w *world) text(query string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.db.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return s
}

// setActor registers a under the token name.
func (w *world) setActor(name string, a actor.Actor) {
	w.actorsMu.Lock()
	w.acts[name] = a
	w.actorsMu.Unlock()
	w.actors.Set(name, a)
}

// actor returns the principal registered under name.
func (w *world) actor(name string) actor.Actor {
	w.actorsMu.Lock()
	defer w.actorsMu.Unlock()
	return w.acts[name]
}

// person inserts an active person, with owner an active owners row, and registers the token name
// for it in a session with a fresh step-up.
func (w *world) person(name string, owner bool) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, id.String(), name)
	w.sql(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		w.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	w.session(name, id, w.clk.Now())
	return id
}

// session registers token name as id in a new session on an active device with step-up at stepUp
// (zero for none). The device row exists in devices too, because read states and drafts name the
// device of the change (M1-61, 03 read_states.updated_by_device_id).
func (w *world) session(name string, id ids.ID, stepUp time.Time) {
	s, d := ids.New(), ids.New()
	w.sql(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Laptop', 'web', 'active')`, d.String(), id.String())
	w.creds.fake.PutDevice(d, fake.Device{PrincipalID: id, State: authz.DeviceActive})
	w.creds.fake.PutSession(s, fake.Session{PrincipalID: id, DeviceID: d, ExpiresAt: start.Add(365 * 24 * time.Hour)})
	w.setActor(name, httpservertest.InSession(id, s, d, stepUp))
}

// do sends a request with token name ("" for anonymous) and the extra headers.
func (w *world) do(name, method, path, body string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	if name != "" {
		headers = append(headers, "Authorization", "Bearer "+name)
	}
	return w.h.Do(w.h.NewRequest(method, path, body, headers...))
}

// want checks status and, for a problem, its code; it returns the decoded body. A success body
// never holds JSON null (ADR 0053); the harness validated it against openapi.yaml.
func (w *world) want(r *httpservertest.Response, status int, code string) map[string]any {
	w.t.Helper()
	if r.StatusCode != status {
		w.t.Fatalf("status %d, want %d: %s", r.StatusCode, status, r.Body)
	}
	if code != "" {
		p := r.Problem(w.t)
		if p["code"] != code {
			w.t.Fatalf("code %v, want %s: %s", p["code"], code, r.Body)
		}
		return p
	}
	var m map[string]any
	if len(r.Body) > 0 {
		if strings.Contains(string(r.Body), "null") {
			w.t.Errorf("JSON null in %s", r.Body)
		}
		if err := json.Unmarshal(r.Body, &m); err != nil {
			w.t.Fatalf("body: %v: %s", err, r.Body)
		}
	}
	return m
}

// wantOK checks a 200 answer and returns its body.
func (w *world) wantOK(r *httpservertest.Response) map[string]any {
	w.t.Helper()
	return w.want(r, http.StatusOK, "")
}

// key returns a fresh Idempotency-Key header.
func key() []string { return []string{"Idempotency-Key", ids.New().String()} }

// ifMatch returns the If-Match header of version and a fresh Idempotency-Key.
func ifMatch(version int64) []string {
	return append([]string{"If-Match", `"` + strconv.FormatInt(version, 10) + `"`}, key()...)
}

// space creates a space through the API as the owner.
func (w *world) space(name string) ids.ID {
	w.t.Helper()
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/spaces", `{"name":"`+name+`"}`, key()...), http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// channel creates a channel of kind in space through the API as the owner.
func (w *world) channel(space ids.ID, kind, name string) ids.ID {
	w.t.Helper()
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/spaces/"+space.String()+"/channels", `{"kind":"`+kind+`","name":"`+name+`"}`, key()...),
		http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// agent creates an agent through the API as the owner (organization) and registers a real API
// token of it under handle.
func (w *world) agent(handle string) ids.ID {
	w.t.Helper()
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/agents", `{"handle":"`+handle+`","display_name":"`+handle+`","adapter_kind":"hermes"}`, key()...),
		http.StatusCreated, "")
	id := ids.MustParse(m["id"].(string))
	issued, err := w.id.Tokens.Issue(context.Background(), identity.IssueTokenCmd{
		PrincipalID: id, Name: "Adapter", ExpiresAt: w.clk.Now().Add(time.Hour), CreatedBy: w.owner,
	})
	must(w.t, err)
	w.setActor(handle, httpservertest.WithToken(id, actor.KindAgent, issued.Token.ID))
	return id
}

// agentVersion reads agents.version of id.
func (w *world) agentVersion(id ids.ID) int64 {
	return w.count(`SELECT version FROM agents WHERE principal_id = $1`, id.String())
}

// setPersonal switches agent to personal with holder through the API as the owner.
func (w *world) setPersonal(agent, holder ids.ID) *httpservertest.Response {
	w.t.Helper()
	r := w.do("owner", http.MethodPut, "/api/v1/agents/"+agent.String()+"/billing-mode",
		`{"billing_mode":"personal","credential_holder_id":"`+holder.String()+`","reason":"Own use"}`, ifMatch(w.agentVersion(agent))...)
	return r
}

// grant creates a grant with the JSON members body through the API as the owner, expects 201
// and returns its id.
func (w *world) grant(body string) ids.ID {
	w.t.Helper()
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/grants", "{"+body+"}", key()...), http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// spaceVersion and channelVersion read the stored versions.
func (w *world) spaceVersion(id ids.ID) int64 {
	return w.count(`SELECT version FROM spaces WHERE id = $1`, id.String())
}

func (w *world) channelVersion(id ids.ID) int64 {
	return w.count(`SELECT version FROM channels WHERE id = $1`, id.String())
}

// setSpaceOperator sends setSpaceOperator as the owner with the current version.
func (w *world) setSpaceOperator(space ids.ID, body string) *httpservertest.Response {
	return w.setSpaceOperatorAs("owner", space, body, ifMatch(w.spaceVersion(space))...)
}

// setSpaceOperatorAs sends setSpaceOperator as name with the headers.
func (w *world) setSpaceOperatorAs(name string, space ids.ID, body string, headers ...string) *httpservertest.Response {
	return w.do(name, http.MethodPut, "/api/v1/spaces/"+space.String()+"/operator", body, headers...)
}

// setChannelOperator sends setChannelOperator as the owner with the current version.
func (w *world) setChannelOperator(channel ids.ID, body string) *httpservertest.Response {
	return w.setChannelOperatorAs("owner", channel, body, ifMatch(w.channelVersion(channel))...)
}

// setChannelOperatorAs sends setChannelOperator as name with the headers.
func (w *world) setChannelOperatorAs(name string, channel ids.ID, body string, headers ...string) *httpservertest.Response {
	return w.do(name, http.MethodPut, "/api/v1/channels/"+channel.String()+"/operator", body, headers...)
}

// auditRow is one row of audit_events.
type auditRow struct {
	Decision, Reason, ResourceID string
	Details                      map[string]any
}

// audits returns the audit rows of action, oldest first, after flushing the aggregated denials.
func (w *world) audits(action string) []auditRow {
	w.t.Helper()
	must(w.t, w.rec.Aggregator().Flush(context.Background()))
	rs, err := w.db.Query(context.Background(), `SELECT decision, coalesce(deny_reason, ''), coalesce(resource_id::text, ''), details::text
		FROM audit_events WHERE action = $1 ORDER BY seq`, action)
	must(w.t, err)
	defer rs.Close()
	var out []auditRow
	for rs.Next() {
		var r auditRow
		var raw string
		must(w.t, rs.Scan(&r.Decision, &r.Reason, &r.ResourceID, &raw))
		must(w.t, json.Unmarshal([]byte(raw), &r.Details))
		out = append(out, r)
	}
	return out
}

// mainThread reads the main thread of a text channel.
func (w *world) mainThread(channel ids.ID) ids.ID {
	return ids.MustParse(w.text(`SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, channel.String()))
}

// noExpiry schedules nothing: the world has no job runtime (M2-08 tests the job in internal/wire).
type noExpiry struct{}

func (noExpiry) ScheduleQueuedExpiry(context.Context, ids.ID, time.Time) error { return nil }

// configValues are the defaults of the server's configuration schema (wire.ConfigSchema, no
// environment besides the public URL).
func configValues(t *testing.T) *config.Values {
	t.Helper()
	schema, err := wire.ConfigSchema()
	must(t, err)
	v, err := config.Load(schema, config.Sources{Env: []string{"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com"}})
	must(t, err)
	return v
}

// withDefaults is a module configuration of the world: every key at the default of the server's
// configuration schema (bound by the module's wire binder), then every non-zero field of world, the
// values the tests read. A key that a module adds later is therefore set here without touching
// the world; before, every new required key broke the setup of this package (runs
// sync.event_payload_max_bytes, run.excerpt_max_chars, messaging principal.handle_max_chars).
func withDefaults[T any](t *testing.T, bind func(*config.Values) (T, error), world T) T {
	t.Helper()
	c, err := bind(configValues(t))
	must(t, err)
	dst, src := reflect.ValueOf(&c).Elem(), reflect.ValueOf(world)
	for i := range src.NumField() {
		if f := src.Field(i); dst.Field(i).CanSet() && !f.IsZero() {
			dst.Field(i).Set(f)
		}
	}
	return c
}

// identityConfig is idConfig over the schema defaults; the world keeps every token kind a bearer
// token and no login method beyond the test logins, so the fields that stay empty in idConfig
// stay empty here too.
func identityConfig(t *testing.T) identityapp.Config {
	t.Helper()
	c := withDefaults(t, wire.IdentityConfig, idConfig)
	c.DPoPRequiredFor, c.WebAuthnOrigins, c.TrustedProxies = nil, nil, nil
	c.TrustedHeaderBindRequests, c.LocalPasswordEnabled = false, false
	return c
}
