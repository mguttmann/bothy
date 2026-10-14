// SPDX-License-Identifier: Apache-2.0

package httpapi_test

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
	"sync/atomic"
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
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	identitypg "github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	identityapp "github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/sync/adapters/telemetry"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// start is the clock of the world.
var start = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

// idCfg is the identity configuration: page size 2 and maximum 3, so lists page with few rows.
var idCfg = identityapp.Config{
	HandleMaxChars: 32, DisplayNameMaxChars: 64, DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: time.Minute,
	ClockLeeway: 30 * time.Second, PublicURL: httpservertest.PublicURL, PageSizeDefault: 2, PageSizeMax: 3,
	StepUpMaxAge: 5 * time.Minute, AccessTokenTTL: 30 * time.Minute, RefreshTokenTTL: 14 * 24 * time.Hour,
	SessionMaxAge: 30 * 24 * time.Hour, RefreshReuseGrace: 30 * time.Second, LastActiveGranularity: 5 * time.Minute,
	StepUpChallengeTTL: 2 * time.Minute, StepUpFailLimit: 3, LoginFailWindow: 15 * time.Minute, CleanupInterval: time.Hour,
	APITokenMaxTTL: 365 * 24 * time.Hour, PersonalTokenMaxTTL: 90 * 24 * time.Hour, DPoPRequiredFor: []string{"agent"},
	// The keys of M1-37, M1-38, M1-40, M1-41 and M1-45, which NewIdentity requires (INT-W14/7).
	EnrollmentCodeTTL: 72 * time.Hour, EnrollmentCodeMaxAttempts: 5, LoginFlowTTL: 10 * time.Minute,
	DevicePendingTTL: 24 * time.Hour, DeviceVerificationCodeDigits: 6, PairingCodeTTL: 5 * time.Minute, PairingCodeChars: 8,
	ServerCodeTTL: time.Minute, OIDCJWKSCacheTTL: time.Hour, OIDCClientAssertionTTL: 2 * time.Minute,
	IDPRecheckInterval: 30 * 24 * time.Hour, DeviceInactiveRevokeAfter: 180 * 24 * time.Hour,
}

// viewAsMax is authz.view_as_max_duration of the world.
const viewAsMax = 15 * time.Minute

// pepper is the token pepper of the world (fixed test bytes, never a secret).
var pepper = []byte("0123456789abcdef0123456789abcdef")

// testKEK is the KEK of the audit module.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// world is one app instance on PostgreSQL 18 with the real identity (DPoP, sessions, step-up,
// personal tokens), authz, audit, sync and instance behind the full HTTP chain, and fakes only for
// the structure (07 section 10: Support, Marketing, Research, Engineering) and the agent policies
// (Lotse, Texter organization; Assistent personal for Research/Notizen with the owner as holder).
type world struct {
	t         *testing.T
	db        *pgtest.DB
	pool      *pg.Pool
	owner     *pg.Pool
	uow       *pg.UnitOfWork
	rec       *auditapp.Recorder
	clk       *clock.Fake
	id        *wire.Identity
	h         *httpservertest.Harness
	az        *authzapp.Authorizer
	admin     *authzapp.AdminService
	structure *fake.Structure
	policies  *fake.Policies
	seq       atomic.Int64

	ownerID, alex, sam, robin   ids.ID
	lotse, texter, assistent    ids.ID
	support, marketing          ids.ID
	research, engineering       ids.ID
	allgemein, eskalation       ids.ID
	kampagnen, notizen, projekt ids.ID
	// leser is the group Leser with Sam; g1 (Alex, Support), g2 (Sam, Marketing) and g4 (Leser,
	// Support/Allgemein) are the grants of 07 section 10 and 11.1.
	leser, g1, g2, g4 ids.ID
	// boss is the owner's client with a fresh step-up.
	boss *client
}

// newWorld builds a world on a new database and seeds 07 section 10.
func newWorld(t *testing.T) *world {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 12
	pool := db.OpenPool(t, cfg)
	log := slog.New(slog.DiscardHandler)
	w := &world{t: t, db: db, pool: pool, owner: db.Pool(t, pg.RoleOwner), clk: clock.NewFake(start), structure: fake.NewStructure(), policies: fake.NewPolicies()}
	principals := identityapp.NewPrincipalSource(identitypg.NewRepository(pool))
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: w.structure, Policies: w.policies}, w.clk, log)
	must(t, err)
	mods := wire.NewModules(pool, log)
	reg := txn.NewRegistry()
	for _, h := range []struct {
		p txn.Phase
		f txn.Hook
	}{{txn.PhaseRightsBegin, hook.Begin}, {txn.PhaseRightsCheck, hook.BeforeCommit}, {txn.PhaseSync, mods.Sync.Run}, {txn.PhaseAudit, mods.Audit.Hook}} {
		must(t, reg.SetHook(h.p, h.f))
	}
	reg.Seal()
	w.uow = pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.rec, err = auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: w.uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: w.clk, Reporter: logs.New(log)})
	must(t, err)
	appender, err := syncapp.NewAppender(syncapp.AppenderConfig{PayloadMaxBytes: 65536}, telemetry.Correlator{})
	must(t, err)
	must(t, hook.Bind(authzapp.RightsBindings{Events: appender, Audit: w.rec}))
	maint, err := syncapp.NewPartitionMaintenance(syncapp.PartitionConfig{Size: 1000, Ahead: 2, Retention: 30 * 24 * time.Hour, LockTimeout: 2 * time.Second},
		w.uow, syncpg.NewRepository(pool, log), w.clk)
	must(t, err)
	_, err = maint.Run(context.Background())
	must(t, err)
	repo := identitypg.NewRepository(pool)
	w.az, err = authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: idCfg.StepUpMaxAge, ViewAsMaxDuration: viewAsMax}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: identityapp.NewCredentialSource(repo, repo, repo), Structure: w.structure, Policies: w.policies,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: w.rec, Clock: w.clk, Log: log,
	})
	must(t, err)
	w.admin, err = authzapp.NewAdminService(authzapp.AdminDeps{
		UoW: w.uow, Authorizer: w.az, Store: authzpg.NewRepository(pool), Principals: principals, Structure: w.structure, Rights: hook,
		Audit: w.rec, Clock: w.clk, Events: appender, Scopes: w.az, Cursors: authzhttp.Cursors{},
		Config: authzapp.AdminConfig{GroupNameMaxChars: 20, GroupDescriptionMaxChars: 40, PageSizeDefault: 2, PageSizeMax: 3},
	})
	must(t, err)
	dir := filepath.Join(t.TempDir(), "signing-keys")
	must(t, os.Mkdir(dir, 0o700))
	sp, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(dir), SigningKeyDir: dir, CacheTTL: time.Minute})
	must(t, err)
	keys, err := wire.NewSigningKeys(wire.SigningKeyConfig{Keys: instanceapp.KeysConfig{
		CacheTTL: time.Minute, InvocationRotation: 30 * 24 * time.Hour, AccessTokenRotation: 30 * 24 * time.Hour,
		JWKSMaxAge: time.Hour, AccessTokenTTL: idCfg.AccessTokenTTL, ClockLeeway: idCfg.ClockLeeway,
	}, Dir: dir}, pool, sp, w.uow, w.az, w.rec, w.clk, log)
	must(t, err)
	must(t, keys.Keys.EnsureActive(context.Background()))
	// Identity takes the instance settings (M1-46) and the feature flags (M1-47); the tests mint
	// personal tokens through the use case, so tokens.personal_enabled is on (brief M1-48 "Tests",
	// INT-W14/7).
	settings, err := wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, w.uow, w.az, wire.SettingsWrites{Audit: w.rec, Events: appender}, w.clk)
	must(t, err)
	w.id, err = wire.NewIdentity(idCfg, pool, w.uow, w.az, wire.IdentityWrites{Audit: w.rec, Events: appender, Settings: settings.Admin, Flags: settings.Admin}, pepper, w.clk, log)
	must(t, err)
	w.sql(`INSERT INTO feature_flags (key, enabled) VALUES ('tokens.personal_enabled', true)`)
	must(t, w.id.BindSessions(keys.Keys, ids.New()))
	built, err := routes.Build(wire.Handlers{Sessions: w.id.SessionHandlers, StepUps: w.id.StepUpHandlers, Access: authzhttp.New(w.admin)},
		wire.RouteOptions(), routes.SpecCatalog())
	must(t, err)
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(pool), w.uow), w.uow, w.clk)
	must(t, err)
	w.h = httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: w.id.Authenticator, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
	w.seed()
	return w
}

// must fails the test on err.
func must(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// seed builds the world of 07 section 10 and 11.1: the people, the agents with their policies,
// the spaces and channels, the group Leser with Sam, and the grants G-1, G-2 and G-4, written
// through the Admin use cases by the owner.
func (w *world) seed() {
	w.t.Helper()
	w.ownerID = w.person("owner", true)
	w.alex, w.sam, w.robin = w.person("alex", false), w.person("sam", false), w.person("robin", false)
	w.lotse, w.texter, w.assistent = w.principal("lotse", "agent"), w.principal("texter", "agent"), w.principal("assistent", "agent")
	w.support, w.marketing, w.research, w.engineering = ids.New(), ids.New(), ids.New(), ids.New()
	for i, s := range []ids.ID{w.support, w.marketing, w.research, w.engineering} {
		w.structure.PutSpaceDetail(authz.SpaceDetail{ID: s, Name: []string{"Support", "Marketing", "Research", "Engineering"}[i],
			SortKey: "a" + strconv.Itoa(i), Version: 1, CreatedAt: start})
		// The spaces row behind the foreign key of grants.target_space_id (M1-52).
		w.sql(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, $2, 'a0', $3)`, s.String(), s.String(), w.ownerID.String())
	}
	w.allgemein = w.channel(w.support, w.lotse)
	w.eskalation = w.channel(w.support, ids.ID{})
	w.kampagnen = w.channel(w.marketing, w.texter)
	w.notizen = w.channel(w.research, w.assistent)
	w.projekt = w.channel(w.engineering, ids.ID{})
	w.policies.Put(w.lotse, authz.AgentPolicy{BillingMode: "organization", ToolClass: "chat"})
	w.policies.Put(w.texter, authz.AgentPolicy{BillingMode: "organization", ToolClass: "scoped"})
	w.policies.Put(w.assistent, authz.AgentPolicy{BillingMode: "personal", CredentialHolderID: w.ownerID, ToolClass: "scoped"})
	w.boss = w.login(w.ownerID)
	w.boss.stepUp()
	ctx, a := context.Background(), w.boss.actor()
	g, err := w.admin.CreateGroup(ctx, a, authz.CreateGroupCmd{Name: "Leser"})
	must(w.t, err)
	w.leser = g.ID
	_, err = w.admin.AddGroupMember(ctx, a, w.leser, authz.AddMemberCmd{UserID: w.sam})
	must(w.t, err)
	w.g1 = w.grantAs(a, "user", w.alex, "space", w.support, "trigger", "upload", "approve_actions")
	w.g2 = w.grantAs(a, "user", w.sam, "space", w.marketing, "trigger", "upload")
	w.g4 = w.grantAs(a, "group", w.leser, "channel", w.allgemein, "read")
}

// grantAs creates a grant through the use case as a and returns its id.
func (w *world) grantAs(a actor.Actor, subjectKind string, subject ids.ID, targetKind string, target ids.ID, level string, caps ...string) ids.ID {
	w.t.Helper()
	g, err := w.admin.CreateGrant(context.Background(), a, authz.CreateGrantCmd{SubjectKind: subjectKind, SubjectID: subject,
		TargetKind: targetKind, TargetID: target, Level: level, Capabilities: caps, Reason: "seed"})
	must(w.t, err)
	return g.ID
}

// channel adds a text channel of space with the effective operator op (zero for none).
func (w *world) channel(space, op ids.ID) ids.ID {
	id := ids.New()
	w.structure.PutChannel(authz.ChannelFacts{ID: id, SpaceID: space, Kind: authz.ChannelText, EffectiveOperatorID: op})
	return id
}

// sql runs stmt as the database owner.
func (w *world) sql(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.owner.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// scalar scans the one value of query into dst.
func (w *world) scalar(dst any, query string, args ...any) {
	w.t.Helper()
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(dst); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
}

// count returns the integer result of query.
func (w *world) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	w.scalar(&n, query, args...)
	return n
}

// principal inserts an active principal of kind with a unique handle.
func (w *world) principal(handle, kind string) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, $2, 'active', $3, $3)`, id.String(), kind,
		handle+strconv.FormatInt(w.seq.Add(1), 10))
	return id
}

// person inserts an active person, an owner when owner is set.
func (w *world) person(handle string, owner bool) ids.ID {
	w.t.Helper()
	id := w.principal(handle, "user")
	w.sql(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		w.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	return id
}

// client is a person's app on an active device with a step_up key and a session.
type client struct {
	w             *world
	key, step     *identitytest.Key
	user, session ids.ID
	access        string
}

// login registers an active device of user with a step_up key and starts a session on it.
func (w *world) login(user ids.ID) *client {
	w.t.Helper()
	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
	step, stepPub := identitytest.NewDeviceKey(jose.AlgES256)
	dev, err := w.id.Devices.Register(context.Background(), identity.RegisterDevice{DeviceID: ids.New(), UserID: user, Name: "Phone",
		Platform: identity.PlatformIOS, DPoPKey: pub, StepUpKey: &stepPub, State: identity.DeviceStateActive, ApprovedBy: &user})
	must(w.t, err)
	set, err := w.id.Sessions.StartSession(context.Background(), identity.StartSession{UserID: user, DeviceID: dev.ID, AuthMethod: identity.AuthPasskey})
	must(w.t, err)
	return &client{w: w, key: key, step: step, user: user, session: set.Session.ID, access: set.AccessToken.Reveal()}
}

// proof signs a DPoP proof for method and path bound to the access token.
func (c *client) proof(method, path string) string {
	return identitytest.SignDPoPProof(c.key, identitytest.ProofClaims{
		HTM: method, HTU: httpservertest.PublicURL + strings.SplitN(path, "?", 2)[0], IAT: c.w.clk.Now(), Nonce: c.w.id.DPoP.CurrentNonce(), ATH: dpop.ATH(c.access),
	})
}

// call sends an authenticated request; a POST gets a fresh Idempotency-Key.
func (c *client) call(method, path, body string) *httpservertest.Response {
	c.w.t.Helper()
	h := []string{"Authorization", "DPoP " + c.access, "DPoP", c.proof(method, path)}
	if method == http.MethodPost || method == http.MethodPatch {
		h = append(h, "Idempotency-Key", ids.New().String())
	}
	return c.w.h.Do(c.w.h.NewRequest(method, path, body, h...))
}

// stepUp confirms a fresh step-up with the device key (M1-35 flow).
func (c *client) stepUp() {
	c.w.t.Helper()
	m := c.w.want(c.call(http.MethodPost, "/api/v1/auth/step-up/challenge", ""), http.StatusOK, "")
	payload, err := json.Marshal(map[string]any{"challenge": m["challenge"], "sid": c.session.String(), "iat": json.Number(strconv.FormatInt(c.w.clk.Now().Unix(), 10))})
	must(c.w.t, err)
	body, _ := json.Marshal(map[string]any{"challenge_id": m["challenge_id"], "signature": identitytest.SignJWS(c.step, jose.Header{Alg: jose.AlgES256}, payload)})
	c.w.want(c.call(http.MethodPost, "/api/v1/auth/step-up/device-key", string(body)), http.StatusOK, "")
}

// actor authenticates the access token of c the way the chain does.
func (c *client) actor() actor.Actor {
	c.w.t.Helper()
	a, err := c.w.id.Authenticator.Authenticate(context.Background(), actor.Credentials{
		Method: http.MethodGet, HTU: httpservertest.PublicURL + "/api/v1/me", Authorization: "DPoP " + c.access, DPoP: c.proof(http.MethodGet, "/api/v1/me"),
	})
	must(c.w.t, err)
	return a
}

// personalToken mints a personal token of c's person through the token use case (c needs a
// fresh step-up) and returns its secret.
func (c *client) personalToken() string {
	c.w.t.Helper()
	issued, err := c.w.id.Tokens.CreateToken(context.Background(), c.actor(), c.user, identity.CreateTokenCmd{Name: "Laptop", ExpiresAt: c.w.clk.Now().Add(24 * time.Hour)})
	must(c.w.t, err)
	return issued.Secret.Reveal()
}

// withToken sends a request with a personal token (Bearer).
func (w *world) withToken(secret, method, path, body string) *httpservertest.Response {
	w.t.Helper()
	h := []string{"Authorization", "Bearer " + secret}
	if method == http.MethodPost {
		h = append(h, "Idempotency-Key", ids.New().String())
	}
	return w.h.Do(w.h.NewRequest(method, path, body, h...))
}

// want checks status and, for a problem, its code, and returns the decoded body. A success body
// never holds JSON null.
func (w *world) want(r *httpservertest.Response, status int, code string) map[string]any {
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
		must(w.t, json.Unmarshal(r.Body, &m))
	}
	return m
}

// wantReason checks status, code and reason of a problem.
func (w *world) wantReason(r *httpservertest.Response, status int, code, reason string) {
	w.t.Helper()
	m := w.want(r, status, code)
	if m["reason"] != reason {
		w.t.Fatalf("reason %v, want %s: %s", m["reason"], reason, r.Body)
	}
}

// wantField checks a 400 validation_failed with a field error of pointer and code.
func (w *world) wantField(r *httpservertest.Response, pointer, code string) {
	w.t.Helper()
	w.wantProblemField(r, http.StatusBadRequest, "validation_failed", pointer, code)
}

// wantProblemField checks status, problem code and a field error of pointer and code.
func (w *world) wantProblemField(r *httpservertest.Response, status int, problem, pointer, code string) {
	w.t.Helper()
	m := w.want(r, status, problem)
	errsList, _ := m["errors"].([]any)
	for _, e := range errsList {
		f := e.(map[string]any)
		if f["pointer"] == pointer && f["code"] == code {
			return
		}
	}
	w.t.Fatalf("no field error %s %s: %s", pointer, code, r.Body)
}

// auditRow is one row of audit_events.
type auditRow struct {
	Action, Decision, Reason, ResourceID string
	Details                              map[string]any
}

// audits returns the audit rows of action with seq above after, oldest first, after flushing the
// aggregated denials.
func (w *world) audits(action string, after int64) []auditRow {
	w.t.Helper()
	must(w.t, w.rec.Aggregator().Flush(context.Background()))
	rows, err := w.owner.Query(context.Background(), `SELECT action, decision, coalesce(deny_reason, ''), coalesce(resource_id::text, ''), details::text
		FROM audit_events WHERE action = $1 AND seq > $2 ORDER BY seq`, action, after)
	must(w.t, err)
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var r auditRow
		var raw string
		must(w.t, rows.Scan(&r.Action, &r.Decision, &r.Reason, &r.ResourceID, &raw))
		must(w.t, json.Unmarshal([]byte(raw), &r.Details))
		out = append(out, r)
	}
	return out
}

// auditSeq returns the highest seq of audit_events.
func (w *world) auditSeq() int64 {
	w.t.Helper()
	var n int64
	w.scalar(&n, `SELECT coalesce(max(seq), 0) FROM audit_events`)
	return n
}

// event is one row of event_log.
type event struct {
	Type, Resource, Audience, Actor, Space, Channel, Receive string
	Payload                                                  map[string]any
}

// eventSeq returns the highest seq of event_log.
func (w *world) eventSeq() int64 {
	w.t.Helper()
	var n int64
	w.scalar(&n, `SELECT coalesce(max(seq), 0) FROM event_log`)
	return n
}

// events returns the events with seq above after, in log order.
func (w *world) events(after int64) []event {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT type, coalesce(resource_id::text, ''), coalesce(audience_principal_id::text, ''),
		coalesce(actor_principal_id::text, ''), coalesce(space_id::text, ''), coalesce(channel_id::text, ''), receive_action, payload::text
		FROM event_log WHERE seq > $1 ORDER BY seq`, after)
	must(w.t, err)
	defer rows.Close()
	var out []event
	for rows.Next() {
		var e event
		var raw string
		must(w.t, rows.Scan(&e.Type, &e.Resource, &e.Audience, &e.Actor, &e.Space, &e.Channel, &e.Receive, &raw))
		must(w.t, json.Unmarshal([]byte(raw), &e.Payload))
		out = append(out, e)
	}
	return out
}

// types returns the event types of evs; access.changed carries its audience.
func types(evs []event) []string {
	out := make([]string, 0, len(evs))
	for _, e := range evs {
		if e.Type == "access.changed" {
			out = append(out, e.Type+"@"+e.Audience)
			continue
		}
		out = append(out, e.Type+":"+change(e))
	}
	slices.Sort(out)
	return out
}

// change returns the change member of a grant.changed or group.changed payload.
func change(e event) string {
	s, _ := e.Payload["change"].(string)
	return s
}

// path helpers of the twelve operations.
func groupPath(id ids.ID) string           { return "/api/v1/groups/" + id.String() }
func membersPath(id ids.ID) string         { return groupPath(id) + "/members" }
func memberPath(group, user ids.ID) string { return membersPath(group) + "/" + user.String() }
func grantPath(id ids.ID) string           { return "/api/v1/grants/" + id.String() }
func revokePath(id ids.ID) string          { return grantPath(id) + "/revoke" }
func idOf(t *testing.T, m map[string]any) ids.ID {
	t.Helper()
	s, _ := m["id"].(string)
	id, err := ids.Parse(s)
	must(t, err)
	return id
}

// grantBody is the JSON of GrantCreate; an empty target id is left out.
func grantBody(subjectKind string, subject ids.ID, targetKind string, target ids.ID, level string, caps ...string) string {
	m := map[string]any{"subject_kind": subjectKind, "subject_id": subject.String(), "target_kind": targetKind, "reason": "project work"}
	if !target.IsZero() {
		m["target_id"] = target.String()
	}
	if level != "" {
		m["level"] = level
	}
	if caps != nil {
		m["capabilities"] = caps
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// manager gives a person level and caps on Support (with manage) through the owner and returns
// the person's client with a fresh step-up.
func (w *world) manager(user ids.ID, level string, caps ...string) *client {
	w.t.Helper()
	w.grantAs(w.boss.actor(), "user", user, "space", w.support, level, append([]string{"manage"}, caps...)...)
	c := w.login(user)
	c.stepUp()
	return c
}
