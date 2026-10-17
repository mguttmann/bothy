// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/adapters/authzsource"
	agentspg "github.com/OWNER/bothy/services/server/internal/agents/adapters/postgres"
	agentsapp "github.com/OWNER/bothy/services/server/internal/agents/app"
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
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
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

// start is the clock of the agents world.
var start = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

// idCfg is the identity configuration of the world: the limits of M1-26 and the token limits of
// M1-44; no token kind must be DPoP-bound, so an agent token works as a bearer token.
var idCfg = identityapp.Config{
	HandleMaxChars: 12, DisplayNameMaxChars: 20, DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: time.Minute,
	ClockLeeway: 30 * time.Second, PublicURL: httpservertest.PublicURL, PageSizeDefault: 2, PageSizeMax: 3,
	StepUpMaxAge: 5 * time.Minute, AccessTokenTTL: 10 * time.Minute, RefreshTokenTTL: 24 * time.Hour,
	SessionMaxAge: 30 * 24 * time.Hour, RefreshReuseGrace: 30 * time.Second, LastActiveGranularity: 5 * time.Minute,
	StepUpChallengeTTL: 2 * time.Minute, StepUpFailLimit: 3, LoginFailWindow: 15 * time.Minute, CleanupInterval: time.Hour,
	APITokenMaxTTL: 365 * 24 * time.Hour, PersonalTokenMaxTTL: 90 * 24 * time.Hour,
	// The keys of M1-37, M1-38, M1-40, M1-41 and M1-45, which NewIdentity requires (INT-W14/8).
	EnrollmentCodeTTL: 72 * time.Hour, EnrollmentCodeMaxAttempts: 5, LoginFlowTTL: 10 * time.Minute,
	DevicePendingTTL: 24 * time.Hour, DeviceVerificationCodeDigits: 6, PairingCodeTTL: 5 * time.Minute, PairingCodeChars: 8,
	ServerCodeTTL: time.Minute, OIDCJWKSCacheTTL: time.Hour, OIDCClientAssertionTTL: 2 * time.Minute,
	IDPRecheckInterval: 30 * 24 * time.Hour, DeviceInactiveRevokeAfter: 180 * 24 * time.Hour,
}

// testPepper is the token pepper of the world (a fixed test value, never a secret).
var testPepper = []byte("0123456789abcdef0123456789abcdef")

// testKEK is the key-encryption key of the audit recorder in these tests.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// creds is the credential source of the world: the sessions, devices and personal tokens the test
// registers in the fake, and the agent tokens identity issued through the real source, so a
// revocation in api_tokens takes effect.
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

// world is PostgreSQL 18 with the real agents module, identity (principal writer, tokens), authz
// (Authorizer, rights hooks with I1, Admin for grants), sync and audit behind the full HTTP chain
// with response validation against openapi.yaml, the test authenticator of httpservertest, a
// controllable clock and a fake structure source (the channels table comes with M1-53).
type world struct {
	t         *testing.T
	pool      *pg.Pool
	owner     *pg.Pool
	uow       *pg.UnitOfWork
	rec       *auditapp.Recorder
	clk       *clock.Fake
	creds     *creds
	structure *fake.Structure
	actors    *httpservertest.Actors
	h         *httpservertest.Harness
	az        authorize.Authorizer
	id        *wire.Identity
	admin     *authzapp.AdminService
	svc       *agentsapp.Service
	source    *authzsource.Source
	// disabled collects the AgentDisabled events a test subscriber saw inside the transactions.
	disabled *[]agents.AgentDisabled
	mu       *sync.Mutex
}

// newWorld builds a world on a new database.
func newWorld(t *testing.T) *world {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 12
	pool := db.OpenPool(t, cfg)
	log := slog.New(&httpservertest.LogCapture{})
	w := &world{t: t, pool: pool, owner: db.Pool(t, pg.RoleOwner), clk: clock.NewFake(start), structure: fake.NewStructure(),
		actors: httpservertest.NewActors(), disabled: &[]agents.AgentDisabled{}, mu: &sync.Mutex{}}
	repo := identitypg.NewRepository(pool)
	principals := identityapp.NewPrincipalSource(repo)
	w.creds = &creds{fake: fake.NewCredentials(), real: identityapp.NewCredentialSource(repo, repo, repo), own: map[ids.ID]bool{}}
	w.source = authzsource.New(agentspg.NewRepository(pool), identityapp.NewDirectory(repo))
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: w.structure, Policies: w.source}, w.clk, log)
	if err != nil {
		t.Fatal(err)
	}
	mods := wire.NewModules(pool, log)
	reg := txn.NewRegistry()
	for _, h := range []struct {
		p txn.Phase
		f txn.Hook
	}{{txn.PhaseRightsBegin, hook.Begin}, {txn.PhaseRightsCheck, hook.BeforeCommit}, {txn.PhaseSync, mods.Sync.Run}, {txn.PhaseAudit, mods.Audit.Hook}} {
		if err := reg.SetHook(h.p, h.f); err != nil {
			t.Fatal(err)
		}
	}
	// A subscriber of AgentDisabled the way runs subscribes from M2 on.
	if err := txn.Subscribe(reg, w.onDisabled); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	w.uow = pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.rec, err = auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: w.uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: w.clk, Reporter: logs.New(log)})
	if err != nil {
		t.Fatal(err)
	}
	appender, err := syncapp.NewAppender(syncapp.AppenderConfig{PayloadMaxBytes: 65536}, telemetry.Correlator{})
	if err != nil {
		t.Fatal(err)
	}
	if err := hook.Bind(authzapp.RightsBindings{Events: appender, Audit: w.rec}); err != nil {
		t.Fatal(err)
	}
	maint, err := syncapp.NewPartitionMaintenance(syncapp.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		w.uow, syncpg.NewRepository(pool, log), w.clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maint.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: idCfg.StepUpMaxAge}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: w.creds, Structure: w.structure, Policies: w.source,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: w.rec, Clock: w.clk, Log: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	w.az = az
	// Identity takes the instance settings (M1-46) and the feature flags (M1-47) (INT-W14/8).
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, w.uow, az, wire.SettingsWrites{Audit: w.rec, Events: appender}, w.clk)
	if err != nil {
		t.Fatal(err)
	}
	writes := wire.IdentityWrites{Audit: w.rec, Events: appender, Settings: settings.Admin, Flags: settings.Admin}
	if w.id, err = wire.NewIdentity(idCfg, pool, w.uow, az, writes, testPepper, w.clk, log); err != nil {
		t.Fatal(err)
	}
	ag, err := wire.NewAgents(agentsapp.Config{PageSizeDefault: 2, PageSizeMax: 3}, pool, w.uow, az, hook, writes, w.id, w.clk, log)
	if err != nil {
		t.Fatal(err)
	}
	w.svc = ag.Service
	if w.admin, err = authzapp.NewAdminService(authzapp.AdminDeps{
		UoW: w.uow, Authorizer: az, Store: authzpg.NewRepository(pool), Principals: principals, Structure: w.structure,
		Rights: hook, Audit: w.rec, Clock: w.clk,
		// The dependencies NewAdminService requires since M1-48 (INT-W14/8).
		Events: appender, Scopes: az, Cursors: authzhttp.Cursors{}, Config: authzapp.AdminConfig{
			GroupNameMaxChars: 100, GroupDescriptionMaxChars: 500, PageSizeDefault: 2, PageSizeMax: 3,
		},
	}); err != nil {
		t.Fatal(err)
	}
	built, err := routes.Build(wire.Handlers{Agents: ag.Handlers}, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(pool), w.uow), w.uow, w.clk)
	if err != nil {
		t.Fatal(err)
	}
	w.h = httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: w.actors, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
	return w
}

// onDisabled is the test subscriber of AgentDisabled; it runs inside the unit of work.
func (w *world) onDisabled(ctx context.Context, ev agents.AgentDisabled) error {
	if _, ok := txn.UnitFrom(ctx); !ok {
		w.t.Error("AgentDisabled outside a unit of work")
	}
	w.mu.Lock()
	*w.disabled = append(*w.disabled, ev)
	w.mu.Unlock()
	return nil
}

// sql runs stmt as the database owner (fixtures outside the use cases).
func (w *world) sql(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.owner.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// count returns the integer result of query.
func (w *world) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// text returns the text result of query.
func (w *world) text(query string, args ...any) string {
	w.t.Helper()
	var s string
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return s
}

// person inserts a person with status, registers the token name for it in a session on an active
// device with a fresh step-up, and with owner an active owners row.
func (w *world) person(name, status string, owner bool) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', $2, $3, $3)`, id.String(), status, name)
	w.sql(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		w.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	w.session(name, id, w.clk.Now())
	return id
}

// session registers token name as id in a new session on an active device, with step-up at
// stepUp (zero for none).
func (w *world) session(name string, id ids.ID, stepUp time.Time) {
	s, d := ids.New(), ids.New()
	w.creds.fake.PutDevice(d, fake.Device{PrincipalID: id, State: authz.DeviceActive})
	w.creds.fake.PutSession(s, fake.Session{PrincipalID: id, DeviceID: d, ExpiresAt: start.Add(365 * 24 * time.Hour)})
	w.actors.Set(name, httpservertest.InSession(id, s, d, stepUp))
}

// personalToken registers token name as a personal API token of the person id (SR5).
func (w *world) personalToken(name string, id ids.ID) {
	tok := ids.New()
	w.creds.fake.PutToken(tok, fake.Token{PrincipalID: id})
	w.creds.mu.Lock()
	w.creds.own[tok] = true
	w.creds.mu.Unlock()
	w.actors.Set(name, httpservertest.WithToken(id, actor.KindUser, tok))
}

// agentToken issues a real API token of the agent through identity and registers it under name.
func (w *world) agentToken(name string, agent, by ids.ID) {
	w.t.Helper()
	issued, err := w.id.Tokens.Issue(context.Background(), identity.IssueTokenCmd{
		PrincipalID: agent, Name: "Adapter", ExpiresAt: w.clk.Now().Add(time.Hour), CreatedBy: by,
	})
	if err != nil {
		w.t.Fatal(err)
	}
	w.actors.Set(name, httpservertest.WithToken(agent, actor.KindAgent, issued.Token.ID))
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

// auditRow is one row of audit_events.
type auditRow struct {
	Action, Decision, Reason, ResourceID string
	Details                              map[string]any
}

// audits returns the audit rows of action, oldest first, after flushing the aggregated denials.
func (w *world) audits(action string) []auditRow {
	w.t.Helper()
	if err := w.rec.Aggregator().Flush(context.Background()); err != nil {
		w.t.Fatal(err)
	}
	rows, err := w.owner.Query(context.Background(), `SELECT action, decision, coalesce(deny_reason, ''), coalesce(resource_id::text, ''), details::text
		FROM audit_events WHERE action = $1 ORDER BY seq`, action)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		var raw string
		if err := rows.Scan(&r.Action, &r.Decision, &r.Reason, &r.ResourceID, &raw); err != nil {
			w.t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(raw), &r.Details); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, r)
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

// i1Denies returns the deny rows of action with reason i1_violation.
func (w *world) i1Denies(action string) []auditRow {
	var out []auditRow
	for _, r := range w.audits(action) {
		if r.Decision == "deny" && r.Reason == "i1_violation" {
			out = append(out, r)
		}
	}
	return out
}

// event is one row of event_log.
type event struct {
	ResourceID, ReceiveAction, Actor string
	Version                          int64
	HasScope                         bool
	Payload                          map[string]any
}

// updates returns the durable events agent.updated, oldest first.
func (w *world) updates() []event {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT coalesce(resource_id::text, ''), receive_action, coalesce(actor_principal_id::text, ''),
		coalesce(resource_version, 0), space_id IS NOT NULL OR channel_id IS NOT NULL OR thread_id IS NOT NULL, payload::text
		FROM event_log WHERE type = 'agent.updated' ORDER BY seq`)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []event
	for rows.Next() {
		var e event
		var raw string
		if err := rows.Scan(&e.ResourceID, &e.ReceiveAction, &e.Actor, &e.Version, &e.HasScope, &raw); err != nil {
			w.t.Fatal(err)
		}
		if strings.Contains(raw, "null") {
			w.t.Errorf("event payload with null: %s", raw)
		}
		if err := json.Unmarshal([]byte(raw), &e.Payload); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, e)
	}
	return out
}

// permissionVersion returns the committed rights version.
func (w *world) permissionVersion() int {
	w.t.Helper()
	return w.count(`SELECT value::int FROM sequence_counters WHERE name = 'permission_version'`)
}

// channel adds a space with a text channel operated by operator (zero for none) to the fake
// structure.
func (w *world) channel(operator ids.ID) ids.ID {
	space, ch := ids.New(), ids.New()
	w.structure.PutSpace(space)
	w.structure.PutChannel(authz.ChannelFacts{ID: ch, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: operator})
	return ch
}

// rawGrant inserts a grant of level to the person subject on channel (zero: the instance)
// directly, bypassing the rights hooks: the fixture of a grant that existed before.
func (w *world) rawGrant(subject, channel ids.ID, level string) {
	w.t.Helper()
	target, col := "instance", ""
	args := []any{ids.New().String(), subject.String(), level}
	if !channel.IsZero() {
		target, col = "channel", ", target_channel_id"
		args = append(args, channel.String())
	}
	vals := "$1, 'user', $2, '" + target + "', $3, '{}', 'fixture', $2"
	if col != "" {
		vals += ", $4"
	}
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, level, capabilities, reason, granted_by`+col+`)
		VALUES (`+vals+`)`, args...)
}

// agentsPath, agentPath and the sub paths of the operations.
const agentsPath = "/api/v1/agents"

func agentPath(id ids.ID) string { return agentsPath + "/" + id.String() }

// ifMatch returns the If-Match header of version.
func ifMatch(version int64) []string {
	return []string{"If-Match", `"` + strconv.FormatInt(version, 10) + `"`}
}

// key returns a fresh Idempotency-Key header.
func key() []string { return []string{"Idempotency-Key", ids.New().String()} }

// create creates an agent through the API as the owner and returns its id.
func (w *world) create(handle string) ids.ID {
	w.t.Helper()
	m := w.want(w.do("owner", http.MethodPost, agentsPath, `{"handle":"`+handle+`","display_name":"`+handle+`","adapter_kind":"hermes"}`, key()...),
		http.StatusCreated, "")
	return ids.MustParse(m["id"].(string))
}

// version reads agents.version of id.
func (w *world) version(id ids.ID) int64 {
	return int64(w.count(`SELECT version::int FROM agents WHERE principal_id = $1`, id.String()))
}

// policy reads billing_mode and the holder ("" for none) of id.
func (w *world) policy(id ids.ID) (string, string) {
	w.t.Helper()
	var mode, holder string
	if err := w.owner.QueryRow(context.Background(), `SELECT billing_mode, coalesce(credential_holder_id::text, '') FROM agents WHERE principal_id = $1`,
		id.String()).Scan(&mode, &holder); err != nil {
		w.t.Fatal(err)
	}
	return mode, holder
}

// setPersonal switches id to personal for holder through the API as the owner and expects 200.
func (w *world) setPersonal(id, holder ids.ID) map[string]any {
	w.t.Helper()
	return w.want(w.do("owner", http.MethodPut, agentPath(id)+"/billing-mode",
		`{"billing_mode":"personal","credential_holder_id":"`+holder.String()+`","reason":"Own use"}`, ifMatch(w.version(id))...), http.StatusOK, "")
}
