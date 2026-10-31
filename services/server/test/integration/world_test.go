// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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
	"github.com/OWNER/bothy/services/server/internal/chat/domain/title"
	filesfs "github.com/OWNER/bothy/services/server/internal/files/adapters/fs"
	filesapp "github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/measure"
	"github.com/OWNER/bothy/services/server/internal/kernel/searchlang"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	messagingapp "github.com/OWNER/bothy/services/server/internal/messaging/app"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

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
	SidebarOrderMaxItems: 1000, SpaceNameMaxChars: 40, ChannelNameMaxChars: 40, ChannelTopicMaxChars: 200, TagNameMaxChars: 20,
	HistoryBackfillLimitDefault: 20, HistoryBackfillLimitMax: 200,
	ThreadTitleMaxChars: 200, ThreadMaxTags: 5,
	PlaceholderTitles: title.DefaultEntries(),
	PageSizeDefault:   20, PageSizeMax: 50,
	ProjectBindingTextMaxChars: 64, ReasonMaxChars: 200,
}

// reasonMaxChars is audit.reason_max_chars of the world (the guided flow and the policy bodies,
// M2-21); short so the tests reach it.
const reasonMaxChars = 200

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
	// measure keeps the own measurement of agents (billing_mode_switches, M6-13).
	measure *measure.Recording
	t       *testing.T
	pool    *pg.Pool
	db      *pg.Pool
	rec     *auditapp.Recorder
	clk     *clock.Fake
	creds   *creds
	actors  *httpservertest.Actors
	acts    map[string]actor.Actor
	h       *httpservertest.Harness
	az      *authzapp.Authorizer
	id      *wire.Identity
	admin   *authzapp.AdminService
	hook    *authzapp.RightsHook
	// structure is the production structure source of chat the hook and authorize read.
	structure authz.StructureSource
	// chat is the chat module; the tests of M2-03 call ApplyProjectReadiness and Threads.Create
	// on it the way M2-20 and messaging will.
	chat *chatapp.Service
	// uow is the unit of work of the modules.
	uow txn.UnitOfWork
	// presence is the presence tracker of M2-20 over the fake clock with fake timers and a
	// recording publisher; readiness goes to chat.
	presence *agentsapp.Tracker
	timers   *fakeTimers
	pub      *recPublisher
	// logs captures every log record of the modules (M2-20 leak checks).
	logs *httpservertest.LogCapture
	// resolver is the guided flow resolve_i1 of M2-21 with the switch of the agents module.
	resolver *authzapp.Resolver
	// search is the search module of M3-14 behind the routes search and searchThreadTitles, over
	// searchReads, the read operations of messaging, chat and files; searchLanguages are its
	// search.languages, and newSearch builds another coordinator over them.
	search          *wire.Search
	searchReads     wire.SearchReads
	searchLanguages []string
	// azConfig and azDeps built az; newAuthorizer builds a second instance from them, the
	// Authorizer of another process (M1-62).
	azConfig authzapp.AuthorizerConfig
	azDeps   authzapp.AuthorizerDeps

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
	w := &world{measure: &measure.Recording{}, t: t, logs: capture, pool: pool, db: db.Pool(t, pg.RoleOwner), clk: clock.NewFake(start), actors: httpservertest.NewActors(),
		acts: map[string]actor.Actor{}, versions: map[ids.ID]int64{}}
	repo := identitypg.NewRepository(pool)
	principals := identityapp.NewPrincipalSource(repo)
	w.creds = &creds{fake: fake.NewCredentials(), real: identityapp.NewCredentialSource(repo, repo, repo), own: map[ids.ID]bool{}}
	// The production sources: the structure of chat and the agent policies of agents.
	structure, policies := wire.NewStructureSource(pool), wire.NewAgentPolicySource(pool)
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: structure, Policies: policies}, w.clk, log)
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
	reg.Seal()
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.uow = uow
	w.rec, err = auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: w.clk, Reporter: logs.New(log)})
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
	w.azConfig = authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: idConfig.StepUpMaxAge, ViewAsMaxDuration: 15 * time.Minute}
	w.azDeps = authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: w.creds, Structure: structure, Policies: policies,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: w.rec, Clock: w.clk, Log: log,
	}
	w.az, err = authzapp.NewAuthorizer(w.azConfig, w.azDeps)
	must(t, err)
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, uow, w.az, wire.SettingsWrites{Audit: w.rec, Events: appender}, w.clk)
	must(t, err)
	// The guided flow resolve_i1 of M2-21: authz authorizes and names the agents, the agents
	// module switches them (bound below, as the start steps of internal/wire do).
	w.resolver, err = authzapp.NewResolver(authzapp.ResolverDeps{Authorizer: w.az, Policies: policies, Rights: hook, ReasonMaxChars: reasonMaxChars})
	must(t, err)
	writes := wire.IdentityWrites{Audit: w.rec, Events: appender, Settings: settings.Admin, Flags: settings.Admin, Search: search, Resolver: w.resolver,
		Measure: w.measure}
	w.id, err = wire.NewIdentity(idConfig, pool, uow, w.az, writes, pepper, w.clk, log)
	must(t, err)
	ag, err := wire.NewAgents(agentsapp.Config{PageSizeDefault: 20, PageSizeMax: 50, ReasonMaxChars: reasonMaxChars}, pool, uow, w.az, hook, writes, w.id, w.clk, log)
	must(t, err)
	w.resolver.BindSwitch(ag.Switch)
	w.timers, w.pub = &fakeTimers{clk: w.clk}, &recPublisher{}
	must(t, ag.BindPresence(presenceConfig, pool, wire.PresenceDeps{UoW: uow, Authorizer: w.az, Audit: w.rec, Clock: w.clk, Timers: w.timers, Log: log}))
	w.presence = ag.Presence
	w.presence.BindPublisher(w.pub)
	w.admin, err = authzapp.NewAdminService(authzapp.AdminDeps{
		UoW: uow, Authorizer: w.az, Store: authzpg.NewRepository(pool), Principals: principals, Structure: structure,
		Rights: hook, Audit: w.rec, Clock: w.clk, Events: appender, Scopes: w.az, Cursors: authzhttp.Cursors{},
		Config: authzapp.AdminConfig{GroupNameMaxChars: 100, GroupDescriptionMaxChars: 500, PageSizeDefault: 20, PageSizeMax: 50,
			ReasonMaxChars: reasonMaxChars},
		Resolver: w.resolver,
	})
	must(t, err)
	c, err := wire.NewChat(chatConfig, pool, uow, w.az, w.az, wire.ChatWrites{Audit: w.rec, Events: appender, Rights: w.hook,
		Directory: w.id.Directory, Resolver: w.resolver}, nil, w.clk)
	must(t, err)
	w.chat = c.Service
	w.presence.BindReadiness(c.Service)
	// Messaging, threads and the instance settings serve the search tests of M3-13; messaging
	// also writes the messages a moved thread carries (M1-62).
	m, err := wire.NewMessaging(messagingapp.Config{MaxContentBytes: 4096, MaxMentions: 4, EventPayloadMaxBytes: 65536,
		TemplateNameMaxChars: 64, TemplateMaxPerScope: 100, ListContentMaxBytes: 4096, PageSizeDefault: 20, PageSizeMax: 50,
		EmojiMaxBytes: 64, MaxDistinctReactions: 20, HandleMaxChars: 32, DisplayNameMaxChars: 64, MaxPinsPerThread: 50},
		pool, uow, w.az, wire.MessagingWrites{Events: appender, Redactor: syncpg.NewRedactor(pool), Audit: w.rec}, c.Service, w.id.Directory, w.clk, nil)
	must(t, err)
	// The personal data of sync (read states and drafts, M1-61) serve the move tests of M1-62.
	personal, err := wire.NewPersonal(syncapp.PersonalConfig{MaxContentBytes: 4096, MaxAttachments: 3, EventPayloadMaxBytes: 65536},
		pool, uow, w.az, appender, c.Service, m.Service, w.clk)
	must(t, err)
	// The files module over a file system store in the test's directory serves the read
	// operations of the search (M3-14); this world creates no uploads.
	root := filepath.Join(t.TempDir(), "files")
	must(t, os.Mkdir(root, 0o700))
	store, err := filesfs.Open(filesfs.Config{Root: root})
	must(t, err)
	t.Cleanup(func() { _ = store.Close() })
	f, err := wire.NewFiles(filesapp.Config{MaxUploadBytes: 1 << 20, PartBytes: 1 << 16, SessionTTL: time.Hour, MaxOpenSessions: 10,
		NameMaxBytes: 255}, "fs", store, pool, uow, w.az, wire.FilesWrites{Events: appender, Redactor: syncpg.NewRedactor(pool)}, c.Service, w.clk, log)
	must(t, err)
	w.searchLanguages = search.Languages()
	w.searchReads = wire.SearchReads{Messaging: m.Service, Chat: c.Service, Files: f.Service}
	w.search, err = wire.NewSearch(searchConfig(), w.searchLanguages, pool, uow, w.az, w.searchReads, w.clk, log)
	must(t, err)
	built, err := routes.Build(wire.Handlers{Access: authzhttp.New(w.admin), Agents: ag.Handlers, Spaces: c.Handlers, Channels: c.ChannelHandlers,
		People: w.id.PeopleHandlers, Threads: c.ThreadHandlers, Behavior: c.BehaviorHandlers, Messages: m.Handlers, Personal: personal.Handlers, Admin: settings.Handlers,
		Finder: w.search.Handlers},
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

// personalToken registers token name as a personal API token of the person id (SR5).
func (w *world) personalToken(name string, id ids.ID) {
	tok := ids.New()
	w.creds.fake.PutToken(tok, fake.Token{PrincipalID: id})
	w.creds.mu.Lock()
	w.creds.own[tok] = true
	w.creds.mu.Unlock()
	w.setActor(name, httpservertest.WithToken(id, actor.KindUser, tok))
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

// agentBody is the OperatorSet body of mode agent with agent.
func agentBody(agent ids.ID) string {
	return `{"mode":"agent","agent_id":"` + agent.String() + `","reason":"Operator change"}`
}

// permissionVersion returns the committed rights version.
func (w *world) permissionVersion() int64 {
	w.t.Helper()
	return w.count(`SELECT value FROM sequence_counters WHERE name = 'permission_version'`)
}

// event is one row of event_log.
type event struct {
	Type, ResourceID, Audience string
	Version                    int64
	Payload                    map[string]any
}

// events returns the durable events of typ, oldest first.
func (w *world) events(typ string) []event {
	w.t.Helper()
	rs, err := w.db.Query(context.Background(), `SELECT type, coalesce(resource_id::text, ''), coalesce(audience_principal_id::text, ''),
		coalesce(resource_version, 0), payload::text FROM event_log WHERE type = $1 ORDER BY seq`, typ)
	must(w.t, err)
	defer rs.Close()
	var out []event
	for rs.Next() {
		var e event
		var raw string
		must(w.t, rs.Scan(&e.Type, &e.ResourceID, &e.Audience, &e.Version, &raw))
		if strings.Contains(raw, "null") {
			w.t.Errorf("event payload with null: %s", raw)
		}
		must(w.t, json.Unmarshal([]byte(raw), &e.Payload))
		out = append(out, e)
	}
	return out
}

// eventCount returns the number of durable events.
func (w *world) eventCount() int64 {
	return w.count(`SELECT count(*) FROM event_log`)
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

// denies returns the deny rows of action with reason.
func (w *world) denies(action, reason string) []auditRow {
	var out []auditRow
	for _, r := range w.audits(action) {
		if r.Decision == "deny" && r.Reason == reason {
			out = append(out, r)
		}
	}
	return out
}

// allows returns the allow rows of action.
func (w *world) allows(action string) []auditRow {
	var out []auditRow
	for _, r := range w.audits(action) {
		if r.Decision == "allow" {
			out = append(out, r)
		}
	}
	return out
}

// wantI1 checks a 409 i1_violation with the hint and exactly one conflict of agent with holder
// and the extra principals, and that the body names nobody (IDs only).
func (w *world) wantI1(r *httpservertest.Response, agent, holder ids.ID, extra ...ids.ID) {
	w.t.Helper()
	p := w.want(r, http.StatusConflict, "i1_violation")
	if p["reason"] != "i1_violation" || p["hint"] != authz.HintI1 {
		w.t.Fatalf("reason or hint: %v", p)
	}
	for _, name := range []string{"alex", "sam", "robin", "assistent", "lotse"} {
		if strings.Contains(string(r.Body), `"`+name) {
			w.t.Fatalf("body names %s: %s", name, r.Body)
		}
	}
	list, _ := p["conflicts"].([]any)
	if len(list) != 1 {
		w.t.Fatalf("conflicts %v", p["conflicts"])
	}
	c, _ := list[0].(map[string]any)
	if c["code"] != "i1_violation" || c["agent_id"] != agent.String() || c["credential_holder_id"] != holder.String() {
		w.t.Fatalf("conflict %v", c)
	}
	var want []string
	for _, e := range extra {
		want = append(want, e.String())
	}
	slices.Sort(want)
	got := strs(c["principal_ids"])
	if !slices.Equal(got, want) {
		w.t.Fatalf("principal_ids %v, want %v", got, want)
	}
}

// strs converts a decoded JSON array of strings.
func strs(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, e := range list {
		s, _ := e.(string)
		out = append(out, s)
	}
	return out
}

// mainThread reads the main thread of a text channel.
func (w *world) mainThread(channel ids.ID) ids.ID {
	return ids.MustParse(w.text(`SELECT id::text FROM threads WHERE channel_id = $1 AND kind = 'main'`, channel.String()))
}

// state is what a rejected operator change must leave unchanged: the operator columns and
// versions of spaces and channels, permission_version and the event log.
type state struct {
	spaces, channels string
	version, events  int64
	allows           int
}

// snapshot reads the state.
func (w *world) snapshot(action string) state {
	w.t.Helper()
	return state{
		spaces: w.text(`SELECT coalesce(string_agg(id::text || ':' || coalesce(default_operator_agent_id::text, '-') || ':' || version, ',' ORDER BY id), '')
			FROM spaces`),
		channels: w.text(`SELECT coalesce(string_agg(id::text || ':' || operator_inherit || ':' || coalesce(operator_agent_id::text, '-') || ':' || version,
			',' ORDER BY id), '') FROM channels`),
		version: w.permissionVersion(), events: w.eventCount(), allows: len(w.allows(action)),
	}
}

// unchanged fails when the state differs from before.
func (w *world) unchanged(before state, action string) {
	w.t.Helper()
	if after := w.snapshot(action); after != before {
		w.t.Fatalf("state changed:\nbefore %+v\nafter  %+v", before, after)
	}
}
