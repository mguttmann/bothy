// SPDX-License-Identifier: Apache-2.0

// Package terminalworld is the HTTP test world of the host records of M2-39: PostgreSQL 18 through
// pgtest with the real identity, authz, audit, sync, instance and terminal modules behind the HTTP
// chain with response validation, a fake clock and the people, group, hosts and grants of the
// brief. Only tests import it; it lives under services/server/test (test code, 02 "Code-Layout").
package terminalworld

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzhttp "github.com/OWNER/bothy/services/server/internal/authz/adapters/httpapi"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	terminalapp "github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/terminaltest"
)

// StateActive is the state of the hosts of the world.
const StateActive = "active"

// Start is the clock of the host world.
var Start = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// IDConfig is the identity configuration of the world: the keys NewIdentity requires; no token kind
// must be DPoP-bound, so an agent token works as a bearer token.
var IDConfig = identityapp.Config{
	HandleMaxChars: 32, DisplayNameMaxChars: 64, DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: time.Minute,
	ClockLeeway: 30 * time.Second, PublicURL: httpservertest.PublicURL, PageSizeDefault: 2, PageSizeMax: 3,
	StepUpMaxAge: 5 * time.Minute, AccessTokenTTL: 10 * time.Minute, RefreshTokenTTL: 24 * time.Hour,
	SessionMaxAge: 30 * 24 * time.Hour, RefreshReuseGrace: 30 * time.Second, LastActiveGranularity: 5 * time.Minute,
	StepUpChallengeTTL: 2 * time.Minute, StepUpFailLimit: 3, LoginFailWindow: 15 * time.Minute, CleanupInterval: time.Hour,
	APITokenMaxTTL: 365 * 24 * time.Hour, PersonalTokenMaxTTL: 90 * 24 * time.Hour,
	EnrollmentCodeTTL: 72 * time.Hour, EnrollmentCodeMaxAttempts: 5, LoginFlowTTL: 10 * time.Minute,
	DevicePendingTTL: 24 * time.Hour, DeviceVerificationCodeDigits: 6, PairingCodeTTL: 5 * time.Minute, PairingCodeChars: 8,
	ServerCodeTTL: time.Minute, OIDCJWKSCacheTTL: time.Hour, OIDCClientAssertionTTL: 2 * time.Minute,
	IDPRecheckInterval: 30 * 24 * time.Hour, DeviceInactiveRevokeAfter: 180 * 24 * time.Hour,
}

// HostConfig is the configuration of the host use cases: small limits so the edges are cheap to test.
var HostConfig = terminalapp.Config{NameMaxChars: 40, OSUserMaxChars: 32, ReasonMaxChars: 60, PairingCodeTTL: 15 * time.Minute,
	PageSizeDefault: 2, PageSizeMax: 3, FingerprintConfirmChars: 8, FingerprintMaxAttempts: 3, ReportLabelMaxChars: 40, ReportMaxEntries: 6,
	ClockLeeway: time.Minute, LastActiveGranularity: 5 * time.Minute}

// Pepper is the token pepper of the world (a fixed test value, never a secret).
var Pepper = []byte("0123456789abcdef0123456789abcdef")

// testKEK is the key-encryption key of the audit recorder.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// FailingAudit wraps the recorder and fails Record for the entries fail selects: the injected
// failure of the tests of one transaction (a failure after the session update rolls back).
type FailingAudit struct {
	auditing.Recorder
	mu   sync.Mutex
	fail func(auditing.Entry) bool
}

// ErrInjected is the injected failure.
var ErrInjected = errors.New("injected failure")

// Record implements auditing.Recorder: the injected failure, or the wrapped recorder.
func (f *FailingAudit) Record(ctx context.Context, e auditing.Entry) error {
	f.mu.Lock()
	fail := f.fail
	f.mu.Unlock()
	if fail != nil && fail(e) {
		return ErrInjected
	}
	return f.Recorder.Record(ctx, e)
}

// Set installs fn as the failure selector; nil removes it.
func (f *FailingAudit) Set(fn func(auditing.Entry) bool) {
	f.mu.Lock()
	f.fail = fn
	f.mu.Unlock()
}

// creds is the credential source of the world: sessions, devices and personal tokens from the
// fake, agent tokens from identity.
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

// World is the test world of M2-39: PostgreSQL 18 with the real identity, authz, audit, sync,
// instance and terminal modules behind the HTTP chain with response validation against
// openapi.yaml, the test authenticator of httpservertest with sessions and step-up, a fake clock,
// an empty structure and no agent policies. Robin is the owner; Alex and Sam are people; the
// group Leser has the member Sam. R1 "Build server" (active, holder Alex), R2 "Lab machine"
// (active) and R3 "Spare machine" (active) exist as fixture rows; Robin grants terminal on R2 to
// Sam until samR2Expiry and on R3 to Leser through createGrant.
type World struct {
	t           *testing.T
	pool        *pg.Pool
	Owner       *pg.Pool
	uow         *pg.UnitOfWork
	Rec         *auditapp.Recorder
	Audit       *FailingAudit
	Clk         *clock.Fake
	creds       *creds
	Actors      *httpservertest.Actors
	H           *httpservertest.Harness
	Az          authorize.Authorizer
	Identity    *wire.Identity
	Logs        *httpservertest.LogCapture
	Robin       ids.ID
	Alex        ids.ID
	Sam         ids.ID
	Leser       ids.ID
	R1, R2, R3  ids.ID
	SamR2Expiry time.Time
	SamR2Grant  ids.ID
	// Terminal is the terminal module of the world (M2-40: the report and the declaration use
	// cases beside the handlers).
	Terminal *wire.Terminal
}

// FreshAge and ReattachWindow are terminal.step_up_max_age and terminal.quick_reattach_window of
// the world (M2-40).
const (
	FreshAge       = time.Minute
	ReattachWindow = time.Minute
)

// New builds the world on a new database with the people, the group, the hosts and the grants.
func New(t *testing.T) *World {
	t.Helper()
	w := NewEmpty(t)
	w.Robin = w.Person("robin", true)
	w.Alex = w.Person("alex", false)
	w.Sam = w.Person("sam", false)
	g := w.Want(w.Do("robin", http.MethodPost, "/api/v1/groups", `{"name":"Leser"}`, Key()...), http.StatusCreated, "")
	w.Leser = ids.MustParse(g["id"].(string))
	w.Want(w.Do("robin", http.MethodPost, "/api/v1/groups/"+w.Leser.String()+"/members", `{"user_id":"`+w.Sam.String()+`"}`, Key()...),
		http.StatusCreated, "")
	w.R1 = w.Host(terminaltest.Host{Name: "Build server", State: StateActive, AssignedUserID: w.Alex,
		ReportedSessionUsers: []string{"bothy-s-alex", "bothy-s-robin", "bothy-s-sam", "bothy-s-shared", "BOTHY-S-SHARED"}})
	w.R2 = w.Host(terminaltest.Host{Name: "Lab machine", State: StateActive, ReportedSessionUsers: []string{"bothy-s-robin", "bothy-s-sam"}})
	w.R3 = w.Host(terminaltest.Host{Name: "Spare machine", State: StateActive})
	w.SamR2Expiry = Start.Add(24 * time.Hour)
	gr := w.Want(w.Do("robin", http.MethodPost, "/api/v1/grants", `{"subject_kind":"user","subject_id":"`+w.Sam.String()+
		`","target_kind":"host","target_id":"`+w.R2.String()+`","capabilities":["terminal"],"expires_at":"`+
		w.SamR2Expiry.Format(time.RFC3339)+`","reason":"Lab work"}`, Key()...), http.StatusCreated, "")
	w.SamR2Grant = ids.MustParse(gr["id"].(string))
	w.Want(w.Do("robin", http.MethodPost, "/api/v1/grants", `{"subject_kind":"group","subject_id":"`+w.Leser.String()+
		`","target_kind":"host","target_id":"`+w.R3.String()+`","capabilities":["terminal"],"reason":"Spare for readers"}`, Key()...),
		http.StatusCreated, "")
	return w
}

// NewEmpty builds the modules and the harness without people or hosts.
func NewEmpty(t *testing.T) *World {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 16
	pool := db.OpenPool(t, cfg)
	logCapture := &httpservertest.LogCapture{}
	log := slog.New(logCapture)
	w := &World{t: t, pool: pool, Owner: db.Pool(t, pg.RoleOwner), Clk: clock.NewFake(Start), Actors: httpservertest.NewActors(), Logs: logCapture}
	repo := identitypg.NewRepository(pool)
	principals := identityapp.NewPrincipalSource(repo)
	w.creds = &creds{fake: fake.NewCredentials(), real: identityapp.NewCredentialSource(repo, repo, repo), own: map[ids.ID]bool{}}
	structure, policies, hosts := fake.NewStructure(), fake.NewPolicies(), wire.NewHostSource(pool)
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: structure, Policies: policies, Hosts: hosts}, w.Clk, log)
	must(t, err)
	appender := w.units(hook, log)
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: IDConfig.StepUpMaxAge,
		TerminalStepUpMaxAge: FreshAge, QuickReattachWindow: ReattachWindow}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: w.creds, Structure: structure, Policies: policies, Hosts: hosts,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: w.Rec, Clock: w.Clk, Log: log,
	})
	must(t, err)
	w.Az = az
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, w.uow, az, wire.SettingsWrites{Audit: w.Rec, Events: appender}, w.Clk)
	must(t, err)
	writes := wire.IdentityWrites{Audit: w.Rec, Events: appender, Settings: settings.Admin, Flags: settings.Admin}
	w.Identity, err = wire.NewIdentity(IDConfig, pool, w.uow, az, writes, Pepper, w.Clk, log)
	must(t, err)
	admin, err := authzapp.NewAdminService(authzapp.AdminDeps{
		UoW: w.uow, Authorizer: az, Store: authzpg.NewRepository(pool), Principals: principals, Structure: structure, Hosts: hosts,
		Rights: hook, Audit: w.Rec, Clock: w.Clk, Events: appender, Scopes: az, Cursors: authzhttp.Cursors{},
		Config: authzapp.AdminConfig{GroupNameMaxChars: 100, GroupDescriptionMaxChars: 500, PageSizeDefault: 2, PageSizeMax: 3},
	})
	must(t, err)
	tm, err := wire.NewTerminal(HostConfig, pool, w.uow, az, wire.IdentityWrites{Audit: w.Audit, Events: appender}, w.Identity, w.Clk, nil, log)
	must(t, err)
	w.Terminal = tm
	w.harness(wire.Handlers{Hosts: tm.Handlers, Access: authzhttp.New(admin), People: w.Identity.PeopleHandlers})
	return w
}

// must fails the test on err.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// units builds the unit of work with the rights, sync and audit hooks, the recorder, the
// failing wrapper and the appender, binds the hook and prepares the event log partitions. It
// returns the appender.
func (w *World) units(hook *authzapp.RightsHook, log *slog.Logger) eventlog.Appender {
	t := w.t
	mods := wire.NewModules(w.pool, log)
	reg := txn.NewRegistry()
	for _, h := range []struct {
		p txn.Phase
		f txn.Hook
	}{{txn.PhaseRightsBegin, hook.Begin}, {txn.PhaseRightsCheck, hook.BeforeCommit}, {txn.PhaseSync, mods.Sync.Run}, {txn.PhaseAudit, mods.Audit.Hook}} {
		must(t, reg.SetHook(h.p, h.f))
	}
	reg.Seal()
	w.uow = pg.NewUnitOfWork(w.pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	var err error
	w.Rec, err = auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: w.uow, Store: auditpg.NewRepository(w.pool), KEK: testKEK{}, Clock: w.Clk, Reporter: logs.New(log)})
	must(t, err)
	w.Audit = &FailingAudit{Recorder: w.Rec}
	appender, err := syncapp.NewAppender(syncapp.AppenderConfig{PayloadMaxBytes: 65536}, telemetry.Correlator{})
	must(t, err)
	must(t, hook.Bind(authzapp.RightsBindings{Events: appender, Audit: w.Rec}))
	maint, err := syncapp.NewPartitionMaintenance(syncapp.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		w.uow, syncpg.NewRepository(w.pool, log), w.Clk)
	must(t, err)
	_, err = maint.Run(context.Background())
	must(t, err)
	return appender
}

// harness builds the routes of handlers and the HTTP harness with the Idempotency-Key stage.
func (w *World) harness(handlers wire.Handlers) {
	t := w.t
	built, err := routes.Build(handlers, wire.RouteOptions(), routes.SpecCatalog())
	must(t, err)
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(w.pool), w.uow), w.uow, w.Clk)
	must(t, err)
	w.H = httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: w.Actors, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
}

// SQL runs stmt as the database owner (fixtures outside the use cases).
func (w *World) SQL(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.Owner.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// Count returns the integer result of query.
func (w *World) Count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.Owner.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// Text returns the text result of query.
func (w *World) Text(query string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.Owner.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return s
}

// Person inserts an active person, with owner an active owners row, and registers the token name
// for it in a session on an active device with a fresh step-up.
func (w *World) Person(name string, owner bool) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.SQL(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, id.String(), name)
	w.SQL(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		w.SQL(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	w.Session(name, id, w.Clk.Now())
	return id
}

// Session registers token name as id in a new session on an active device, with step-up at
// stepUp (zero for none).
func (w *World) Session(name string, id ids.ID, stepUp time.Time) {
	s, d := ids.New(), ids.New()
	w.creds.fake.PutDevice(d, fake.Device{PrincipalID: id, State: authz.DeviceActive})
	w.creds.fake.PutSession(s, fake.Session{PrincipalID: id, DeviceID: d, ExpiresAt: Start.Add(365 * 24 * time.Hour)})
	a := httpservertest.InSession(id, s, d, stepUp)
	// A session of the app with a step-up by the device key (M2-40: the fresh step-up of the
	// terminal reads both).
	a.ClientComponent = "app"
	if !stepUp.IsZero() {
		a.StepUpMethod = "device_key"
	}
	w.Actors.Set(name, a)
}

// Actor returns the principal registered under name (M2-40: the terminal tests authorize with it).
func (w *World) Actor(name string) actor.Actor {
	w.t.Helper()
	a := w.Actors.Get(name)
	if a.ID.IsZero() {
		w.t.Fatalf("no actor %s", name)
	}
	return a
}

// PersonalToken registers token name as a personal API token of the person id (SR5).
func (w *World) PersonalToken(name string, id ids.ID) {
	tok := ids.New()
	w.creds.fake.PutToken(tok, fake.Token{PrincipalID: id})
	w.creds.mu.Lock()
	w.creds.own[tok] = true
	w.creds.mu.Unlock()
	w.Actors.Set(name, httpservertest.WithToken(id, actor.KindUser, tok))
}

// AgentToken creates an agent principal and registers a real API token of it under name.
func (w *World) AgentToken(name string) ids.ID {
	w.t.Helper()
	agent := ids.New()
	w.SQL(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', $2, $2)`, agent.String(), name)
	issued, err := w.Identity.Tokens.Issue(context.Background(), identity.IssueTokenCmd{
		PrincipalID: agent, Name: "Adapter", ExpiresAt: w.Clk.Now().Add(time.Hour), CreatedBy: w.Robin,
	})
	if err != nil {
		w.t.Fatal(err)
	}
	w.Actors.Set(name, httpservertest.WithToken(agent, actor.KindAgent, issued.Token.ID))
	return agent
}

// AppToken creates an app principal with an installation and registers a token of it under name.
func (w *World) AppToken(name string) ids.ID {
	app, tok := ids.New(), ids.New()
	w.SQL(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'app', 'active', $2, $2)`, app.String(), name)
	w.creds.fake.PutToken(tok, fake.Token{PrincipalID: app})
	w.creds.mu.Lock()
	w.creds.own[tok] = true
	w.creds.mu.Unlock()
	a := httpservertest.WithToken(app, actor.KindApp, tok)
	a.InstallationID = ids.New()
	w.Actors.Set(name, a)
	return app
}

// Host inserts a host fixture created by Robin.
func (w *World) Host(h terminaltest.Host) ids.ID {
	w.t.Helper()
	if h.CreatedBy.IsZero() {
		h.CreatedBy = w.Robin
	}
	id, err := terminaltest.InsertHost(context.Background(), w.Owner, h)
	if err != nil {
		w.t.Fatal(err)
	}
	return id
}

// SessionUser inserts a session user row of user on host.
func (w *World) SessionUser(host, user ids.ID, osUser string) ids.ID {
	w.t.Helper()
	id, err := terminaltest.InsertSessionUser(context.Background(), w.Owner, terminaltest.SessionUser{HostID: host, UserID: user, OSUser: osUser, CreatedBy: w.Robin})
	if err != nil {
		w.t.Fatal(err)
	}
	return id
}

// TerminalSession inserts a session of user on host with the session user su in state.
func (w *World) TerminalSession(host, user, su ids.ID, state string) ids.ID {
	w.t.Helper()
	id, err := terminaltest.InsertSession(context.Background(), w.Owner, terminaltest.Session{HostID: host, UserID: user, SessionUserID: su,
		State: state, BytesIn: 120, BytesOut: 4096, OpenedAt: w.Clk.Now().Add(-time.Minute)})
	if err != nil {
		w.t.Fatal(err)
	}
	return id
}

// Do sends a request with token name ("" for anonymous) and the extra headers.
func (w *World) Do(name, method, path, body string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	if name != "" {
		headers = append(headers, "Authorization", "Bearer "+name)
	}
	return w.H.Do(w.H.NewRequest(method, path, body, headers...))
}

// Want checks status and, for a problem, its code; it returns the decoded body. A success body
// never holds JSON null (ADR 0053); the harness validated it against openapi.yaml.
func (w *World) Want(r *httpservertest.Response, status int, code string) map[string]any {
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

// Field returns code and pointer of the first field error of a problem.
func Field(p map[string]any) (code, pointer string) {
	list, _ := p["errors"].([]any)
	if len(list) == 0 {
		return "", ""
	}
	e, _ := list[0].(map[string]any)
	c, _ := e["code"].(string)
	ptr, _ := e["pointer"].(string)
	return c, ptr
}

// AuditRow is one row of audit_events.
type AuditRow struct {
	Action, Decision, Reason, ResourceID, Actor string
	Details                                     map[string]any
}

// Audits returns the audit rows of action, oldest first, after flushing the aggregated denials.
func (w *World) Audits(action string) []AuditRow {
	w.t.Helper()
	if err := w.Rec.Aggregator().Flush(context.Background()); err != nil {
		w.t.Fatal(err)
	}
	rows, err := w.Owner.Query(context.Background(), `SELECT action, decision, coalesce(deny_reason, ''), coalesce(resource_id::text, ''),
		coalesce(actor_principal_id::text, ''), details::text FROM audit_events WHERE action = $1 ORDER BY seq`, action)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var r AuditRow
		var raw string
		if err := rows.Scan(&r.Action, &r.Decision, &r.Reason, &r.ResourceID, &r.Actor, &raw); err != nil {
			w.t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(raw), &r.Details); err != nil {
			w.t.Fatal(err)
		}
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

// Event is one row of event_log.
type Event struct {
	Type, ResourceID, ReceiveAction, Actor, Audience string
	Seq, Version                                     int64
	HasScope                                         bool
	Raw                                              string
	Payload                                          map[string]any
}

// Events returns the durable events of type typ, oldest first.
func (w *World) Events(typ string) []Event {
	w.t.Helper()
	rows, err := w.Owner.Query(context.Background(), `SELECT seq, type, coalesce(resource_id::text, ''), receive_action, coalesce(actor_principal_id::text, ''),
		coalesce(audience_principal_id::text, ''), coalesce(resource_version, 0),
		space_id IS NOT NULL OR channel_id IS NOT NULL OR thread_id IS NOT NULL, payload::text
		FROM event_log WHERE type = $1 ORDER BY seq`, typ)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Seq, &e.Type, &e.ResourceID, &e.ReceiveAction, &e.Actor, &e.Audience, &e.Version, &e.HasScope, &e.Raw); err != nil {
			w.t.Fatal(err)
		}
		if strings.Contains(e.Raw, "null") {
			w.t.Errorf("event payload with null: %s", e.Raw)
		}
		if err := json.Unmarshal([]byte(e.Raw), &e.Payload); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// PermissionVersion returns the committed rights version.
func (w *World) PermissionVersion() int {
	w.t.Helper()
	return w.Count(`SELECT value::int FROM sequence_counters WHERE name = 'permission_version'`)
}

// HostVersion reads hosts.version of id.
func (w *World) HostVersion(id ids.ID) int64 {
	return int64(w.Count(`SELECT version::int FROM hosts WHERE id = $1`, id.String()))
}

// HostsPath is the path of the host collection.
const HostsPath = "/api/v1/hosts"

// HostPath is the path of the host id.
func HostPath(id ids.ID) string { return HostsPath + "/" + id.String() }

// IfMatch returns the If-Match header of version.
func IfMatch(version int64) []string {
	return []string{"If-Match", `"` + strconv.FormatInt(version, 10) + `"`}
}

// Key returns a fresh Idempotency-Key header.
func Key() []string { return []string{"Idempotency-Key", ids.New().String()} }

// Keyed returns the If-Match header of the current version of id with a fresh Idempotency-Key.
func (w *World) Keyed(id ids.ID) []string { return append(IfMatch(w.HostVersion(id)), Key()...) }

// AssertNowhere checks that s appears in no table row, no log line, no event payload and no
// audit detail of the world (the canary checks of SEC-T10).
func (w *World) AssertNowhere(s, what string) {
	w.t.Helper()
	for _, q := range []string{
		`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%' OR coalesce(deny_reason, '') LIKE '%' || $1 || '%'`,
		`SELECT count(*) FROM event_log WHERE payload::text LIKE '%' || $1 || '%'`,
		`SELECT count(*) FROM idempotency_keys WHERE coalesce(response_body::text, '') LIKE '%' || $1 || '%'`,
		`SELECT count(*) FROM hosts WHERE name LIKE '%' || $1 || '%' OR encode(pairing_code_hash, 'escape') LIKE '%' || $1 || '%'`,
	} {
		if n := w.Count(q, s); n != 0 {
			w.t.Errorf("%s found %d times: %s", what, n, q)
		}
	}
	for _, r := range append(w.H.Logs.Records(), w.Logs.Records()...) {
		if strings.Contains(r.Message, s) {
			w.t.Errorf("%s in log message %q", what, r.Message)
		}
		for k, v := range r.Attrs {
			if strings.Contains(v, s) {
				w.t.Errorf("%s in log attribute %s", what, k)
			}
		}
	}
}

// AccessChanges returns the payloads of access.changed after the event seq, per audience.
func (w *World) AccessChanges(seq int64) map[ids.ID][]string {
	out := map[ids.ID][]string{}
	for _, e := range w.Events("access.changed") {
		if e.Seq <= seq {
			continue
		}
		p := ids.MustParse(e.Audience)
		out[p] = append(out[p], e.Raw)
	}
	return out
}

// LastSeq returns the highest seq of the event log.
func (w *World) LastSeq() int64 {
	return int64(w.Count(`SELECT coalesce(max(seq), 0)::int FROM event_log`))
}

// HasHostEntry reports an entry {kind: host, id, capabilities: [terminal]} without level in the list
// of payload (added or removed; a removed entry carries kind and id only, terminal false).
func HasHostEntry(t *testing.T, payload, list string, host ids.ID, terminal bool) bool {
	t.Helper()
	var p map[string][]map[string]any
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatal(err)
	}
	p = map[string][]map[string]any{}
	if err := json.Unmarshal(raw[list], &p); err != nil {
		var entries []map[string]any
		if err := json.Unmarshal(raw[list], &entries); err != nil {
			t.Fatal(err)
		}
		p[list] = entries
	}
	for _, e := range p[list] {
		if e["kind"] != "host" || e["id"] != host.String() {
			continue
		}
		if _, hasLevel := e["level"]; hasLevel {
			return false
		}
		caps, _ := e["capabilities"].([]any)
		if terminal {
			return len(caps) == 1 && caps[0] == "terminal"
		}
		return e["capabilities"] == nil
	}
	return false
}

// ListIDs lists the hosts as name, following every next_cursor with limit, and returns the ids
// in order.
func (w *World) ListIDs(name string, limit int) []ids.ID {
	w.t.Helper()
	var out []ids.ID
	path := HostsPath + "?limit=" + strconv.Itoa(limit)
	for {
		m := w.Want(w.Do(name, http.MethodGet, path, ""), http.StatusOK, "")
		for _, it := range m["items"].([]any) {
			out = append(out, ids.MustParse(it.(map[string]any)["id"].(string)))
		}
		next, ok := m["next_cursor"].(string)
		if !ok {
			return out
		}
		path = HostsPath + "?limit=" + strconv.Itoa(limit) + "&cursor=" + next
	}
}

// SameAnswer checks that two answers have the same status, body and headers apart from the ones
// that differ per request.
func SameAnswer(t *testing.T, a, b *httpservertest.Response) {
	t.Helper()
	if a.StatusCode != b.StatusCode || string(StripCorrelation(a.Body)) != string(StripCorrelation(b.Body)) {
		t.Fatalf("answers differ: %d %s / %d %s", a.StatusCode, a.Body, b.StatusCode, b.Body)
	}
	for k := range a.Header {
		switch k {
		case "Date", "Correlation-Id", "Traceparent", "Content-Length":
			continue
		}
		if !slices.Equal(a.Header.Values(k), b.Header.Values(k)) {
			t.Errorf("header %s differs: %v / %v", k, a.Header.Values(k), b.Header.Values(k))
		}
	}
}

// StripCorrelation removes the members of a problem body that differ per request.
func StripCorrelation(body []byte) []byte {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return body
	}
	delete(m, "correlation_id")
	delete(m, "instance")
	out, _ := json.Marshal(m)
	return out
}
