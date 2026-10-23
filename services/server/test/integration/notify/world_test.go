// SPDX-License-Identifier: Apache-2.0

// Package notify_test holds the integration tests of the push path of notify (brief M3-19,
// "Tests", Integration): PostgreSQL 18 through pgtest, the real modules built through
// internal/wire as serve builds them, the full HTTP chain with response validation against
// openapi.yaml, the job runtime in role api (insert only) and fakepush providers. No test talks to
// a real push service and no test sleeps for a window: the tests move the fake clock and call the
// job use cases directly.
//
// # The world
//
// newWorld builds the world of 07 sections 10 and 11.1 (as the runs world of M1-54 and M2-05):
// Alex, Sam, Robin and the Owner; the spaces Support (Allgemein, Eskalation), Marketing
// (Kampagnen), Engineering (Projekt-A, Werkzeuge) and Research (Notizen); the agents Lotse,
// Texter, Runner and Assistent; the group Leser with Sam; the grants G-1 to G-5. Every channel of
// Support needs a mention of Lotse in threads too (thread_require_mention), so a human message
// opens no run unless it mentions the agent. On top of it:
//
//   - the push registrations as fixture rows (registerPush is M3-22): Alex phone (ios, apns), Sam
//     phone (android, fcm), the Owner phone (ios, apns), each on an own active device and session
//     (w.alexPhone, w.samPhone, w.ownerPhone);
//   - notify (wire.NewNotify) with the in-transaction subscribers of notify bound as the chat step
//     of serve binds them, and the push path (wire.NewPush) over the real modules: the Authorizer
//     as authorize.Authorizer and authz.Recipients, identity Directory and Devices, the chat and
//     messaging services, the sync history and read positions, the personal settings;
//   - the job runtime in role api over a registry of wire.NewJobRegistry (the jobs of notify with
//     the push path, the other modules without use cases) and River's tables of jobstest, bound to
//     the scheduler of the push path (Push.BindJobs);
//   - a sync ConsumerService with the consumer push registered (wire.Push.Consumer), its cursor
//     placed at the head after the build (Ensure), run deterministically through Round;
//   - fakepush providers registered as apns and fcm (w.apns, w.fcm; withProviders narrows them);
//   - the fake clock of the world in every module (w.clk), the configuration of the push keys with
//     fixture values (pushEnv) loaded through the configuration schema of internal/wire;
//   - a meter provider with an SDK manual reader in PushDeps.Metrics (w.skipped, w.sent) and the log
//     of the push path and of the consumer captured apart (w.pushLog).
//
// # The API of the world
//
// HTTP (helpers_test.go): do, want, wantOK; post and postBody (createMessage), mainThread,
// newThread (createThread from a main message, without first message), createThread (with first
// message), readUpTo (putReadState), setLevel
// (putNotificationPreference), setMySetting (putMySetting), removeMember (removeGroupMember),
// revoke (revokeGrant), grant, behavior, dmChannel (a fixture dm channel with its participants).
//
// Push path (push_helpers_test.go): consume runs the consumer push to the head (consumeOn on a
// ConsumerService of newConsumers, a second worker); cursor and head read the positions; pushJobs,
// pushJobsFor, jobsOf and pendingOf list the jobs of River's table by kind with their decoded
// arguments; hintsFor returns the distinct NotificationIDs of the notify.push_send jobs of a
// recipient (its hints); send runs one job notify.push_send through Push.Send and completes it,
// sendPending every due one; flushDue runs every pending window and flush job due at the clock
// through Push.Flush (the window marker does nothing) and completes them; badgeSyncsDue does the
// same for notify.badge_sync and notify.badge_sync_hold; advance and w.clk move time; skipped,
// sent and counter read the push counters; logMark and logSince return the captured records after
// a mark. Grants and groups change through grant, revoke and removeMember.
//
// Runs (runs_helpers_test.go, the invocation tokens of M2-06 over in-memory keys): deliver stands
// in for pollAgentEvents, start, await, finish and cancel send the run lifecycle operations.
package notify_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

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
	"github.com/OWNER/bothy/services/server/internal/notify/adapters/fakepush"
	notifymetrics "github.com/OWNER/bothy/services/server/internal/notify/adapters/otelmetrics"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
	"github.com/OWNER/bothy/services/server/internal/platform/jobs/jobstest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	syncpkg "github.com/OWNER/bothy/services/server/internal/sync"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// start is the clock of the world. The push path reads only the fake clock; occurred_at of an
// event is the transaction time of PostgreSQL (04 3.2), so the t0 of a hint (DueAt) is a wall
// clock reading, which the tests of push.max_age read from the job and move the fake clock past.
var start = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// jobsSchema is the schema of River's tables in the test database (jobs.schema).
const jobsSchema = "jobs"

// pushEnv are the fixture values of every configuration key of the push path (13 section 17, 03
// section 9) and of the consumers, as BOTHY_ variables of the configuration schema.
var pushEnv = []string{
	"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com",
	"BOTHY_NOTIFY__USER_DEFAULT_LEVEL=mentions",
	"BOTHY_NOTIFY__COALESCE_WINDOW=60s",
	"BOTHY_NOTIFY__MOBILE_DELAY_WHEN_DESKTOP_ACTIVE=60s",
	"BOTHY_NOTIFY__DEFAULT_ADMIN_ENABLED=true",
	"BOTHY_NOTIFY__DEFAULT_INPUT_NEEDED_BREAKS_QUIET_HOURS=false",
	"BOTHY_NOTIFY__BADGE_SYNC_MIN_INTERVAL=30s",
	"BOTHY_PUSH__MAX_AGE=1h",
	"BOTHY_PUSH__RETRY_BASE=2s",
	"BOTHY_PUSH__RETRY_CAP=5m",
	"BOTHY_PUSH__MAX_CONCURRENT_REQUESTS=8",
	"BOTHY_PUSH__MAX_CONSECUTIVE_FAILURES=3",
	"BOTHY_CONSUMER__BATCH_SIZE=4",
	"BOTHY_CONSUMER__POLL_INTERVAL=1s",
}

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

// dmStructure is the structure source of chat with the dm participation of the fixture dm
// channels. chat answers no dm participation before V3 (internal/wire joins its source with an
// empty dm part), so the world stands in for that part with the participants it wrote as fixture
// rows (dmChannel); every other answer is the production source.
type dmStructure struct {
	authz.StructureSource
	mu  sync.Mutex
	dms map[ids.ID][]ids.ID
}

// Channel implements authz.StructureSource with the participants of a fixture dm channel.
func (s *dmStructure) Channel(ctx context.Context, id ids.ID) (authz.ChannelFacts, bool, error) {
	f, ok, err := s.StructureSource.Channel(ctx, id)
	if err != nil || !ok || f.Kind != authz.ChannelDM {
		return f, ok, err
	}
	s.mu.Lock()
	f.DMParticipants = slices.Clone(s.dms[id])
	s.mu.Unlock()
	return f, ok, nil
}

// DMChannelsOf implements authz.StructureSource over the fixture dm channels.
func (s *dmStructure) DMChannelsOf(_ context.Context, user ids.ID) ([]ids.ID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ids.ID
	for ch, ps := range s.dms {
		if slices.Contains(ps, user) {
			out = append(out, ch)
		}
	}
	slices.SortFunc(out, func(a, b ids.ID) int { return strings.Compare(a.String(), b.String()) })
	return out, nil
}

// put records the participants of the dm channel ch.
func (s *dmStructure) put(ch ids.ID, participants ...ids.ID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := slices.Clone(participants)
	slices.SortFunc(ps, func(a, b ids.ID) int { return strings.Compare(a.String(), b.String()) })
	s.dms[ch] = ps
}

// phone is one fixture push registration on an own active device and session.
type phone struct {
	// Device, Session and Registration are the rows; Provider apns or fcm.
	Device, Session, Registration ids.ID
	Provider                      string
	// Token is the fixture device token; it must never reach a log.
	Token string
}

// options are the variants of a world.
type options struct {
	// providers are the names of the fakepush providers registered in the push path.
	providers []string
	// veto registers, after the subscriber of notify, a second MessagePosted subscriber that
	// refuses every human message with clear_draft (withVeto).
	veto bool
}

// option changes the variant of a world.
type option func(*options)

// withProviders registers only the fakepush providers of names (default apns and fcm).
func withProviders(names ...string) option {
	return func(o *options) { o.providers = names }
}

// withVeto adds the vetoing MessagePosted subscriber after the subscriber of notify. The txn
// registry derives the module of a handler from its package below internal/, so a type of this
// test package cannot subscribe; the vetoing subscriber is a second, unbound instance of the
// MessagePosted subscriber of sync, which refuses exactly the human messages with clear_draft
// (its appender is missing) and lets every other message pass.
func withVeto() option {
	return func(o *options) { o.veto = true }
}

// world is the cross-module world of the push path (see the package comment).
type world struct {
	t      *testing.T
	pool   *pg.Pool
	db     *pg.Pool
	pgdb   *pgtest.DB
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
	// structure is the structure source of chat with the fixture dm participation.
	structure *dmStructure
	chat      *chatapp.Service
	messaging *messagingapp.Service
	personal  *wire.Personal
	uow       txn.UnitOfWork
	appender  *syncapp.Appender
	runs      *runsapp.Service
	// log captures the log of the modules; pushLog the log of the push path and the consumer.
	log, pushLog *httpservertest.LogCapture
	// cancels records the jobs runs.cancel_timeout (M2-09); keys and issuer sign and name the
	// invocation tokens.
	cancels *cancelJobs
	keys    *testKeys
	issuer  ids.ID
	// values is the loaded configuration with pushEnv.
	values *config.Values
	// notify is the notify module; push the push path; jobsRT the job runtime in role api.
	notify *wire.Notify
	push   *wire.Push
	jobsRT *jobs.Runtime
	// consumers runs the consumer push; consumerCfg is its configuration.
	consumers   *syncapp.ConsumerService
	consumerCfg syncapp.ConsumerConfig
	// apns and fcm are the fakepush providers (nil when not registered).
	apns, fcm *fakepush.FakeProvider
	// reader reads the push metrics of the meter provider of the push path.
	reader *sdkmetric.ManualReader
	// instanceID is the instance record's ID of every Notification.
	instanceID ids.ID

	owner, alex, sam, robin                    ids.ID
	support, marketing, engineering, research  ids.ID
	allgemein, eskalation, kampagnen, projektA ids.ID
	werkzeuge, notizen                         ids.ID
	lotse, texter, runner, assistent           ids.ID
	leser                                      ids.ID
	g1, g2, g3, g4, g5                         ids.ID
	alexPhone, samPhone, ownerPhone            phone
	actorsMu                                   sync.Mutex
}

// newWorld builds the modules, the harness, the world of 07 sections 10 and 11.1, the fixture
// registrations and the push path, and places the cursor of the consumer push at the head.
func newWorld(t *testing.T, opts ...option) *world {
	t.Helper()
	o := options{providers: []string{"apns", "fcm"}}
	for _, f := range opts {
		f(&o)
	}
	w := newEmptyWorld(t, o)
	w.build()
	w.alexPhone = w.phone(w.alex, "ios", "apns")
	w.samPhone = w.phone(w.sam, "android", "fcm")
	w.ownerPhone = w.phone(w.owner, "ios", "apns")
	must(t, w.consumers.Ensure(context.Background()))
	return w
}

// newEmptyWorld builds the modules, the push path and the harness with only the owner.
func newEmptyWorld(t *testing.T, o options) *world {
	t.Helper()
	db := pgtest.New(t)
	jobstest.Migrate(t, db, jobsSchema)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 24
	pool := db.OpenPool(t, cfg)
	capture, pushCapture := &httpservertest.LogCapture{}, &httpservertest.LogCapture{}
	log, pushLog := slog.New(capture), slog.New(pushCapture)
	w := &world{t: t, pool: pool, db: db.Pool(t, pg.RoleOwner), pgdb: db, clk: clock.NewFake(start), actors: httpservertest.NewActors(),
		acts: map[string]actor.Actor{}, log: capture, pushLog: pushCapture, cancels: &cancelJobs{}, issuer: ids.New(), instanceID: ids.New()}
	schema, err := wire.ConfigSchema()
	must(t, err)
	w.values, err = config.Load(schema, config.Sources{Env: pushEnv})
	must(t, err)
	search, err := searchlang.New([]string{"german", "english"}, map[string]string{"de": "german", "en": "english"})
	must(t, err)
	repo := identitypg.NewRepository(pool)
	principals := identityapp.NewPrincipalSource(repo)
	w.creds = &creds{fake: fake.NewCredentials(), real: identityapp.NewCredentialSource(repo, repo, repo), own: map[ids.ID]bool{}}
	w.structure = &dmStructure{StructureSource: wire.NewStructureSource(pool), dms: map[ids.ID][]ids.ID{}}
	policies, hosts := wire.NewAgentPolicySource(pool), wire.NewHostSource(pool)
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: w.structure, Policies: policies, Hosts: hosts}, w.clk, log)
	must(t, err)
	w.hook = hook
	mods := wire.NewModules(pool, log)
	mods.Authz = hook
	reg := w.registry(mods, o)
	uow := pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.uow = uow
	w.rec, err = auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: w.clk, Reporter: logs.New(log)})
	must(t, err)
	w.appender, err = syncapp.NewAppender(syncapp.AppenderConfig{PayloadMaxBytes: 65536}, telemetry.Correlator{})
	must(t, err)
	must(t, hook.Bind(authzapp.RightsBindings{Events: w.appender, Audit: w.rec}))
	must(t, mods.SyncSubscribers.Bind(w.appender))
	syncRepo := syncpg.NewRepository(pool, log)
	maint, err := syncapp.NewPartitionMaintenance(syncapp.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		uow, syncRepo, w.clk)
	must(t, err)
	_, err = maint.Run(context.Background())
	must(t, err)
	w.az, err = authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: idConfig.StepUpMaxAge, ViewAsMaxDuration: 15 * time.Minute},
		authzapp.AuthorizerDeps{
			Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
			Credentials: w.creds, Structure: w.structure, Policies: policies, Hosts: hosts,
			Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: w.rec, Clock: w.clk, Log: log,
		})
	must(t, err)
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, uow, w.az, wire.SettingsWrites{Audit: w.rec, Events: w.appender}, w.clk)
	must(t, err)
	writes := wire.IdentityWrites{Audit: w.rec, Events: w.appender, Settings: settings.Admin, Flags: settings.Admin, Search: search}
	w.id, err = wire.NewIdentity(idConfig, pool, uow, w.az, writes, pepper, w.clk, log)
	must(t, err)
	ag, err := wire.NewAgents(agentsapp.Config{PageSizeDefault: 20, PageSizeMax: 50, ReasonMaxChars: 200}, pool, uow, w.az, hook, writes, w.id, w.clk, log)
	must(t, err)
	w.admin, err = authzapp.NewAdminService(authzapp.AdminDeps{
		UoW: uow, Authorizer: w.az, Store: authzpg.NewRepository(pool), Principals: principals, Structure: w.structure, Hosts: hosts,
		Rights: hook, Audit: w.rec, Clock: w.clk, Events: w.appender, Scopes: w.az, Cursors: authzhttp.Cursors{},
		Config: authzapp.AdminConfig{GroupNameMaxChars: 100, GroupDescriptionMaxChars: 500, PageSizeDefault: 20, PageSizeMax: 50},
	})
	must(t, err)
	c, err := wire.NewChat(chatConfig, pool, uow, w.az, w.az, wire.ChatWrites{Audit: w.rec, Events: w.appender, Rights: w.hook,
		Directory: w.id.Directory}, nil, w.clk)
	must(t, err)
	w.chat = c.Service
	m, err := wire.NewMessaging(messagingapp.Config{MaxContentBytes: 4096, MaxMentions: 4, EventPayloadMaxBytes: 65536,
		TemplateNameMaxChars: 64, TemplateMaxPerScope: 100, ListContentMaxBytes: 4096, PageSizeDefault: 20, PageSizeMax: 50,
		EmojiMaxBytes: 64, MaxDistinctReactions: 20, MaxPinsPerThread: 50},
		pool, uow, w.az, wire.MessagingWrites{Events: w.appender, Redactor: syncpg.NewRedactor(pool), Audit: w.rec}, c.Service, w.id.Directory, w.clk, nil)
	must(t, err)
	w.messaging = m.Service
	w.personal, err = wire.NewPersonal(syncapp.PersonalConfig{MaxContentBytes: 4096, MaxAttachments: 3, EventPayloadMaxBytes: 65536},
		pool, uow, w.az, w.appender, c.Service, m.Service, w.clk)
	must(t, err)
	// The chat step of serve (startNotify): notify over chat, the personal settings and the
	// directory, then the in-transaction subscribers of notify bound to the appender and the
	// chat paths with the clock of the world.
	notifyCfg, err := wire.NotifyConfig(w.values)
	must(t, err)
	w.notify, err = wire.NewNotify(notifyCfg, pool, uow, w.az, w.appender, wire.NotifyDeps{Chat: c.Service, Settings: w.personal.Service.Settings(),
		Users: w.id.Directory}, w.clk, log)
	must(t, err)
	must(t, mods.NotifySubscribers.WithClock(w.clk).Bind(w.appender, c.Service))
	rn, err := wire.NewRuns(runsapp.Config{PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 256, QueuedMaxAge: 48 * time.Hour,
		ClockLeeway: 30 * time.Second, CancelGrace: 30 * time.Second, PresenceTimeout: 90 * time.Second,
		OrphanTimeout: 15 * time.Minute, ReasonMaxChars: 500, EventPayloadMaxBytes: 65536}, pool, uow, w.az, wire.RunsDeps{
		Events: w.appender, Messages: m.Service, Threads: c.Service, Channels: c.Service, Agents: ag.Service, Metrics: noTransitions{},
		Expiry: noExpiry{}, Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Audit: w.rec, CancelTimeouts: w.cancels,
	}, w.clk, log)
	must(t, err)
	must(t, mods.RunsAcceptor.Bind(rn.Service))
	w.runs = rn.Service
	w.keys = newTestKeys(t)
	must(t, rn.BindInvocationTokens(tokenConfig, w.keys, fixedInstance{id: w.issuer}, w.clk))
	w.buildPush(pool, uow, syncRepo, o, pushLog)
	built, err := routes.Build(wire.Handlers{Access: authzhttp.New(w.admin), Agents: ag.Handlers, Spaces: c.Handlers, Channels: c.ChannelHandlers,
		People: w.id.PeopleHandlers, Threads: c.ThreadHandlers, Behavior: c.BehaviorHandlers, Messages: m.Handlers, Admin: settings.Handlers,
		Personal: w.personal.Handlers, Runs: rn.Handlers, Preferences: w.notify.Handlers},
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

// registry builds the sealed txn registry of the world in the order of internal/wire
// (registerTxn): the four phase hooks, then messaging.MessagePosted -> sync, chat.ThreadMoved ->
// messaging and -> sync, chat.ChannelArchived and chat.SpaceArchived -> notify,
// messaging.MessagePosted -> runs and -> notify (the automatic following), the access change and
// the disabled agent -> runs, chat.ChannelArchived -> apps; with o.veto the vetoing subscriber
// last. internal/wire.NewTxnRegistry seals the registry, which would leave no place for it.
func (w *world) registry(mods *wire.Modules, o options) *txn.Registry {
	reg := txn.NewRegistry()
	for _, h := range []struct {
		p txn.Phase
		f txn.Hook
	}{{txn.PhaseRightsBegin, mods.Authz.Begin}, {txn.PhaseRightsCheck, mods.Authz.BeforeCommit}, {txn.PhaseSync, mods.Sync.Run}, {txn.PhaseAudit, mods.Audit.Hook}} {
		must(w.t, reg.SetHook(h.p, h.f))
	}
	must(w.t, txn.Subscribe(reg, mods.SyncSubscribers.OnMessagePosted))
	must(w.t, txn.Subscribe(reg, mods.MessagingSubscribers.OnThreadMoved))
	must(w.t, txn.Subscribe(reg, mods.SyncSubscribers.OnThreadMoved))
	must(w.t, txn.Subscribe(reg, mods.NotifySubscribers.OnChannelArchived))
	must(w.t, txn.Subscribe(reg, mods.NotifySubscribers.OnSpaceArchived))
	must(w.t, txn.Subscribe(reg, mods.RunsAcceptor.OnMessagePosted))
	must(w.t, txn.Subscribe(reg, mods.NotifySubscribers.OnMessagePosted))
	must(w.t, txn.Subscribe(reg, mods.RunsAcceptor.OnAccessChanged))
	must(w.t, txn.Subscribe(reg, mods.RunsAcceptor.OnAgentDisabled))
	must(w.t, txn.Subscribe(reg, mods.AppsArchive.OnChannelArchived))
	if o.veto {
		rows := syncpg.NewPersonalRepository(w.pool)
		must(w.t, txn.Subscribe(reg, syncapp.NewSubscribers(rows, rows).OnMessagePosted))
	}
	reg.Seal()
	return reg
}

// buildPush builds the push path as the jobs step of serve does (buildPush, startJobs): the
// configuration of the push keys, wire.NewPush over the real modules, the job registry of
// wire.NewJobRegistry with the push path, the job runtime in role api and the scheduler bound to
// its inserter; then the consumer push on a ConsumerService of its own.
func (w *world) buildPush(pool *pg.Pool, uow txn.UnitOfWork, syncRepo *syncpg.Repository, o options, log *slog.Logger) {
	t := w.t
	cfg, err := wire.PushConfig(w.values)
	must(t, err)
	var providers []notifyapp.PushProvider
	for _, name := range o.providers {
		p := fakepush.New(name, w.clk)
		switch name {
		case "apns":
			w.apns = p
		case "fcm":
			w.fcm = p
		}
		providers = append(providers, p)
	}
	w.reader = sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(w.reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	w.push, err = wire.NewPush(cfg, pool, uow, wire.PushDeps{
		Notify: w.notify, Authorizer: w.az, Recipients: w.az, Directory: w.id.Directory, Devices: w.id.Devices,
		Settings: w.personal.Service.Settings(), Chat: w.chat, Messaging: w.messaging, History: syncRepo,
		Positions: syncpg.NewPersonalRepository(pool), InstanceID: w.instanceID, Providers: providers, Metrics: mustMetrics(t, mp),
	}, w.clk, log)
	must(t, err)
	reg, err := wire.NewJobRegistry(&wire.JobModules{PartitionInterval: time.Minute, CleanupInterval: time.Hour, ExpiryInterval: time.Minute,
		KeyRotationInterval: time.Minute, AuthCleanupInterval: time.Hour, ChainVerifyInterval: time.Hour, StallInterval: time.Minute,
		Push: w.push.Push})
	must(t, err)
	mw := map[string]int{}
	for _, q := range jobs.Queues() {
		mw[q] = 1
	}
	w.jobsRT, err = jobs.New(pool, jobs.Config{Schema: jobsSchema, MaxWorkers: mw, SoftStopTimeout: time.Second, Logger: log}, jobs.RoleAPI, reg, nil)
	must(t, err)
	w.push.BindJobs(w.jobsRT.Inserter())
	must(t, config.Bind(w.values, &w.consumerCfg))
	w.consumers = w.newConsumers(w.push.Consumer())
}

// newConsumers returns a ConsumerService of its own (a worker process) with h registered as the
// consumer push, over the unit of work and the log of the world; the log goes to pushLog.
func (w *world) newConsumers(h syncpkg.ConsumerHandler) *syncapp.ConsumerService {
	w.t.Helper()
	repo := syncpg.NewRepository(w.pool, slog.New(w.pushLog))
	svc, err := syncapp.NewConsumerService(w.consumerCfg, w.uow, repo, repo, slog.New(w.pushLog))
	must(w.t, err)
	must(w.t, svc.Register(syncpkg.ConsumerPush, h))
	return svc
}

// build creates the world of 07 sections 10 and 11.1 through the API: Alex, Sam and Robin; the
// spaces Support, Marketing, Engineering and Research with their channels; the agents Lotse,
// Texter, Runner and Assistent; the group Leser with Sam; the grants G-1 to G-5. Allgemein and
// Eskalation need a mention of Lotse in threads too, so a human message opens no run by itself.
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
	g := w.want(w.do("owner", http.MethodPost, "/api/v1/groups", `{"name":"Leser"}`, key()...), http.StatusCreated)
	w.leser = ids.MustParse(g["id"].(string))
	w.want(w.do("owner", http.MethodPost, "/api/v1/groups/"+w.leser.String()+"/members", `{"user_id":"`+w.sam.String()+
		`","expires_at":"2026-11-30T00:00:00Z"}`, key()...), http.StatusCreated)
	w.g4 = w.grant(`"subject_kind":"group","subject_id":"` + w.leser.String() + `","target_kind":"channel","target_id":"` + w.allgemein.String() +
		`","level":"read","reason":"Readers"`)
	w.g5 = w.grant(`"subject_kind":"agent","subject_id":"` + w.lotse.String() + `","target_kind":"space","target_id":"` + w.support.String() +
		`","level":"write","capabilities":["upload"],"reason":"Support agent writes"`)
	w.behavior(w.allgemein, `"thread_require_mention":true`)
	w.behavior(w.eskalation, `"thread_require_mention":true`)
}

// phone writes an active device of platform, a session on it and a push registration of
// provider for user as fixture rows (registerPush is M3-22); created_at is the clock of the world.
func (w *world) phone(user ids.ID, platform, provider string) phone {
	w.t.Helper()
	p := phone{Device: ids.New(), Session: ids.New(), Registration: ids.New(), Provider: provider, Token: "device-token-" + ids.New().String()}
	w.sql(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Phone', $3, 'active')`, p.Device.String(), user.String(), platform)
	w.sql(`INSERT INTO auth_sessions (id, user_id, device_id, auth_method, expires_at) VALUES ($1, $2, $3, 'passkey', $4)`,
		p.Session.String(), user.String(), p.Device.String(), start.Add(365*24*time.Hour))
	w.sql(`INSERT INTO push_registrations (id, user_id, device_id, auth_session_id, provider, token, app_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'com.example.app', $7)`,
		p.Registration.String(), user.String(), p.Device.String(), p.Session.String(), provider, p.Token, w.clk.Now())
	return p
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

// session registers token name as id in a new session on an active web device with step-up at
// stepUp. The device row exists in devices too, because read states name the device of the change
// (M1-61); it has no push registration, so it is never a push route.
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

// want checks status and returns the decoded body.
func (w *world) want(r *httpservertest.Response, status int) map[string]any {
	w.t.Helper()
	if r.StatusCode != status {
		w.t.Fatalf("status %d, want %d: %s", r.StatusCode, status, r.Body)
	}
	var m map[string]any
	if len(r.Body) > 0 {
		if err := json.Unmarshal(r.Body, &m); err != nil {
			w.t.Fatalf("body: %v: %s", err, r.Body)
		}
	}
	return m
}

// wantOK checks a 200 answer and returns its body.
func (w *world) wantOK(r *httpservertest.Response) map[string]any {
	w.t.Helper()
	return w.want(r, http.StatusOK)
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
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/spaces", `{"name":"`+name+`"}`, key()...), http.StatusCreated)
	return ids.MustParse(m["id"].(string))
}

// channel creates a channel of kind in space through the API as the owner.
func (w *world) channel(space ids.ID, kind, name string) ids.ID {
	w.t.Helper()
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/spaces/"+space.String()+"/channels", `{"kind":"`+kind+`","name":"`+name+`"}`, key()...),
		http.StatusCreated)
	return ids.MustParse(m["id"].(string))
}

// agent creates an agent through the API as the owner (organization) and registers a real API
// token of it under handle.
func (w *world) agent(handle string) ids.ID {
	w.t.Helper()
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/agents", `{"handle":"`+handle+`","display_name":"`+handle+`","adapter_kind":"hermes"}`, key()...),
		http.StatusCreated)
	id := ids.MustParse(m["id"].(string))
	issued, err := w.id.Tokens.Issue(context.Background(), identity.IssueTokenCmd{
		PrincipalID: id, Name: "Adapter", ExpiresAt: w.clk.Now().Add(365 * 24 * time.Hour), CreatedBy: w.owner,
	})
	must(w.t, err)
	w.setActor(handle, httpservertest.WithToken(id, actor.KindAgent, issued.Token.ID))
	return id
}

// setPersonal switches agent to personal with holder through the API as the owner.
func (w *world) setPersonal(agent, holder ids.ID) {
	w.t.Helper()
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/agents/"+agent.String()+"/billing-mode",
		`{"billing_mode":"personal","credential_holder_id":"`+holder.String()+`","reason":"Own use"}`,
		ifMatch(w.count(`SELECT version FROM agents WHERE principal_id = $1`, agent.String()))...))
}

// grant creates a grant with the JSON members body through the API as the owner, expects 201
// and returns its id.
func (w *world) grant(body string) ids.ID {
	w.t.Helper()
	m := w.want(w.do("owner", http.MethodPost, "/api/v1/grants", "{"+body+"}", key()...), http.StatusCreated)
	return ids.MustParse(m["id"].(string))
}

// setSpaceOperator sends setSpaceOperator as the owner with the current version.
func (w *world) setSpaceOperator(space ids.ID, body string) *httpservertest.Response {
	return w.do("owner", http.MethodPut, "/api/v1/spaces/"+space.String()+"/operator", body,
		ifMatch(w.count(`SELECT version FROM spaces WHERE id = $1`, space.String()))...)
}

// setChannelOperator sends setChannelOperator as the owner with the current version.
func (w *world) setChannelOperator(channel ids.ID, body string) *httpservertest.Response {
	return w.do("owner", http.MethodPut, "/api/v1/channels/"+channel.String()+"/operator", body,
		ifMatch(w.count(`SELECT version FROM channels WHERE id = $1`, channel.String()))...)
}

// behavior replaces the behaviour of channel with the defaults of 09 3.3 and the extra members
// (putChannelBehavior as the owner, with step-up and If-Match).
func (w *world) behavior(channel ids.ID, extra string) {
	w.t.Helper()
	cur := w.wantOK(w.do("owner", http.MethodGet, "/api/v1/channels/"+channel.String()+"/behavior", ""))
	version := int64(cur["settings_version"].(float64))
	body := map[string]string{
		"require_mention": "true", "thread_require_mention": "false", "free_response": "false", "auto_thread": "true",
		"allow_mentioned_agents": "false", "history_backfill_limit": "20",
	}
	for _, kv := range strings.Split(extra, ",") {
		if kv == "" {
			continue
		}
		k, v, _ := strings.Cut(kv, ":")
		body[strings.Trim(k, `"`)] = v
	}
	parts := make([]string, 0, len(body))
	for k, v := range body {
		parts = append(parts, `"`+k+`":`+v)
	}
	w.wantOK(w.do("owner", http.MethodPut, "/api/v1/channels/"+channel.String()+"/behavior", "{"+strings.Join(parts, ",")+`,"prompt":"","skills":[]}`,
		ifMatch(version)...))
}

// noExpiry schedules nothing: the queued expiry of runs is not part of the push path.
type noExpiry struct{}

func (noExpiry) ScheduleQueuedExpiry(context.Context, ids.ID, time.Time) error { return nil }

// noTransitions records no run transition (runsapp.Metrics).
type noTransitions struct{}

func (noTransitions) RunTransitioned(context.Context, runsapp.Transitioned) {}

// mustMetrics returns the push metrics over mp.
func mustMetrics(t *testing.T, mp *sdkmetric.MeterProvider) *notifymetrics.Metrics {
	t.Helper()
	m, err := notifymetrics.New(mp)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
