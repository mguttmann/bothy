// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzpg "github.com/OWNER/bothy/services/server/internal/authz/adapters/postgres"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
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

// sessStart is the clock of the session world.
var sessStart = time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)

// sessCfg is the identity configuration of the session world (brief M1-34 "Config keys").
var sessCfg = app.Config{
	HandleMaxChars: 32, DisplayNameMaxChars: 64, DPoPNonceTTL: 5 * time.Minute, DPoPProofMaxAge: time.Minute,
	ClockLeeway: 30 * time.Second, PublicURL: httpservertest.PublicURL, PageSizeDefault: 2, PageSizeMax: 3,
	StepUpMaxAge: 5 * time.Minute, AccessTokenTTL: 10 * time.Minute, RefreshTokenTTL: 14 * 24 * time.Hour,
	SessionMaxAge: 30 * 24 * time.Hour, RefreshReuseGrace: 30 * time.Second, LastActiveGranularity: 5 * time.Minute,
	StepUpChallengeTTL: 2 * time.Minute, StepUpFailLimit: 3, LoginFailWindow: 15 * time.Minute, CleanupInterval: time.Hour,
	APITokenMaxTTL: 365 * 24 * time.Hour, PersonalTokenMaxTTL: 90 * 24 * time.Hour, DPoPRequiredFor: []string{"agent"},
	EnrollmentCodeTTL: 72 * time.Hour, EnrollmentCodeMaxAttempts: 3, LoginFlowTTL: 10 * time.Minute,
	DevicePendingTTL: 24 * time.Hour, DeviceVerificationCodeDigits: 6, PairingCodeTTL: 5 * time.Minute, PairingCodeChars: 8,
	ServerCodeTTL: time.Minute, OIDCJWKSCacheTTL: time.Hour, OIDCClientAssertionTTL: 2 * time.Minute,
	IDPRecheckInterval: 7 * 24 * time.Hour,
}

// testPepper is the token pepper of the session world (generated per test run, never a secret).
var testPepper = []byte("0123456789abcdef0123456789abcdef")

// sworld is one app instance on PostgreSQL 18 with the real identity (devices, DPoP, sessions,
// Authenticator), authz with the credential source of identity, sync, audit and the signing keys
// of instance behind the full HTTP chain (brief M1-34 "Integration tests").
type sworld struct {
	t     *testing.T
	db    *pgtest.DB
	pool  *pg.Pool
	owner *pg.Pool
	uow   *pg.UnitOfWork
	rec   *auditapp.Recorder
	clk   *clock.Fake
	id    *wire.Identity
	// settings are the instance settings, branding and discovery (M1-46).
	settings *wire.InstanceSettings
	h        *httpservertest.Harness
	logs     *httpservertest.LogCapture
	ended    *[]identity.SessionEnded
	instID   ids.ID
	// az is the real authorizer; structure the fake structure source behind it (M1-35).
	az        authorize.Authorizer
	structure *fake.Structure
	// dir is the signing key directory of the installation.
	dir string
	// personal is the switch tokens.personal_enabled of the world (M1-44), off at the start.
	personal *personalSwitch
	// seq numbers the handles of the token cases (M1-44).
	seq atomic.Int64
	// routes and idempotency are the routes and the Idempotency-Key stage of h, for a second
	// harness with further stages (the rate limit of M1-38).
	routes      []httpservertest.Route
	idempotency func(http.Handler) http.Handler
}

// personalSwitch is a fake PersonalTokenSwitch whose value a test sets (brief M1-44 "Tests").
type personalSwitch struct{ on atomic.Bool }

// PersonalTokensEnabled implements app.PersonalTokenSwitch.
func (s *personalSwitch) PersonalTokensEnabled(context.Context, ids.ID) (bool, error) {
	return s.on.Load(), nil
}

// newSessionWorld builds a world on a new database.
func newSessionWorld(t *testing.T) *sworld {
	t.Helper()
	return newSessionWorldOn(t, pgtest.New(t), clock.NewFake(sessStart), ids.New(), keyDir(t))
}

// keyDir returns a new signing key directory with mode 0700.
func keyDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "signing-keys")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// newSessionWorldOn builds a world on db with its own pools, caches and chain; worlds on the same
// db, clock, instance id and key directory are two server instances of one installation. Each
// bind runs after BindSessions and before the routes are built (the OIDC world of M1-41 binds the
// provider client there).
func newSessionWorldOn(t *testing.T, db *pgtest.DB, clk *clock.Fake, instID ids.ID, dir string, binds ...func(*sworld)) *sworld {
	t.Helper()
	return newSessionWorldWith(t, db, clk, instID, dir, sessCfg, binds...)
}

// newSessionWorldWith is newSessionWorldOn with the identity configuration cfg (M1-43). The public
// listener of the world is the loopback test server, so the flag of the trusted header driver is
// set; a test of the off state clears it. The binds run as in newSessionWorldOn.
func newSessionWorldWith(t *testing.T, db *pgtest.DB, clk *clock.Fake, instID ids.ID, dir string, cfg app.Config, binds ...func(*sworld)) *sworld {
	t.Helper()
	pcfg := db.Config(pg.RoleApp)
	pcfg.MaxConns = 12
	pool := db.OpenPool(t, pcfg)
	capture := &httpservertest.LogCapture{}
	log := slog.New(capture)
	ended := &[]identity.SessionEnded{}
	w := &sworld{t: t, db: db, pool: pool, owner: db.Pool(t, pg.RoleOwner), clk: clk, logs: capture, ended: ended, instID: instID, dir: dir,
		personal: &personalSwitch{}}
	repo := postgres.NewRepository(pool)
	principals := app.NewPrincipalSource(repo)
	structure, policies := fake.NewStructure(), fake.NewPolicies()
	hook, err := wire.NewRightsHook(pool, wire.RightsSources{Principals: principals, Structure: structure, Policies: policies}, clk, log)
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
	// A subscriber of SessionEnded the way notify subscribes from M3 on.
	if err := txn.Subscribe(reg, func(_ context.Context, ev identity.SessionEnded) error {
		*ended = append(*ended, ev)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	w.uow = pg.NewUnitOfWork(pool, pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 20 * time.Second, IdleInTxTimeout: 60 * time.Second}, reg)
	w.rec, err = auditapp.NewRecorder(auditapp.Config{
		DenyAggregateWindow: time.Hour, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, auditapp.Deps{UoW: w.uow, Store: auditpg.NewRepository(pool), KEK: testKEK{}, Clock: clk, Reporter: logs.New(log)})
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
		w.uow, syncpg.NewRepository(pool, log), clk)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := maint.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 1000, StepUpMaxAge: cfg.StepUpMaxAge}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(log), Store: authzpg.NewRepository(pool), Principals: principals,
		Credentials: app.NewCredentialSource(repo, repo, repo), Structure: structure, Policies: policies,
		Versions: instanceapp.NewVersions(instancepg.NewRepository(pool)), Recorder: w.rec, Clock: clk, Log: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	w.az, w.structure = az, structure
	sp, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(dir), SigningKeyDir: dir, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	keys, err := wire.NewSigningKeys(wire.SigningKeyConfig{Keys: instanceapp.KeysConfig{
		CacheTTL: time.Minute, InvocationRotation: 30 * 24 * time.Hour, AccessTokenRotation: 30 * 24 * time.Hour,
		JWKSMaxAge: time.Hour, AccessTokenTTL: cfg.AccessTokenTTL, ClockLeeway: cfg.ClockLeeway,
	}, Dir: dir}, pool, sp, w.uow, az, w.rec, clk, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.Keys.EnsureActive(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.settings, err = wire.NewInstanceSettings(wire.DefaultInstanceConfig(), true, pool, w.uow, az, wire.SettingsWrites{Audit: w.rec, Events: appender}, clk)
	if err != nil {
		t.Fatal(err)
	}
	w.id, err = wire.NewIdentity(cfg, pool, w.uow, az, wire.IdentityWrites{Audit: w.rec, Events: appender, PersonalTokens: w.personal, Settings: w.settings.Admin}, testPepper, clk, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.id.BindSessions(keys.Keys, instID); err != nil {
		t.Fatal(err)
	}
	for _, bind := range binds {
		bind(w)
	}
	w.id.SetPublicListenerLoopback(true)
	built, err := routes.Build(wire.Handlers{Sessions: w.id.SessionHandlers, StepUps: w.id.StepUpHandlers, Devices: w.id.Handlers, People: w.id.PeopleHandlers,
		Providers: w.id.ProviderHandlers, Credentials: w.id.CredentialHandlers,
		Tokens: w.id.TokenHandlers, Enrollment: w.id.EnrollmentHandlers, Approval: w.id.ApprovalHandlers, Admin: w.settings.Handlers,
		Passkeys: w.id.PasskeyHandlers, OIDC: w.id.OIDCHandlers,
		TrustedHeader: w.id.TrustedHeaderHandlers},
		wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(pool), w.uow), w.uow, clk)
	if err != nil {
		t.Fatal(err)
	}
	w.routes, w.idempotency = rs, stage
	w.h = httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: w.id.Authenticator, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
	return w
}

// sql runs stmt as the database owner.
func (w *sworld) sql(stmt string, args ...any) {
	w.t.Helper()
	if _, err := w.owner.Exec(context.Background(), stmt, args...); err != nil {
		w.t.Fatalf("%s: %v", stmt, err)
	}
}

// scalar scans the one value of query into dst.
func (w *sworld) scalar(dst any, query string, args ...any) {
	w.t.Helper()
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(dst); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
}

// count returns the integer result of query.
func (w *sworld) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	w.scalar(&n, query, args...)
	return n
}

// client is an app on a device: its DPoP key and its current token pair.
type client struct {
	w                     *sworld
	key                   *identitytest.Key
	user, device, session ids.ID
	access, refresh       string
	refreshExpiresAt      time.Time
	parent                *client
}

// person inserts an active person with owner flag.
func (w *sworld) person(handle string, owner bool) ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'user', 'active', $2, $2)`, id.String(), handle)
	w.sql(`INSERT INTO users (principal_id) VALUES ($1)`, id.String())
	if owner {
		w.sql(`INSERT INTO owners (id, user_id, reason) VALUES ($1, $2, 'first owner')`, ids.New().String(), id.String())
	}
	return id
}

// login registers a device of user in state (active or pending) and starts an app session on it
// through identity.Auth, the way the login tickets will.
func (w *sworld) login(user ids.ID, state identity.DeviceState) *client {
	w.t.Helper()
	key, pub := identitytest.NewDeviceKey(jose.AlgES256)
	reg := identity.RegisterDevice{DeviceID: ids.New(), UserID: user, Name: "Alex Phone", Platform: identity.PlatformIOS, DPoPKey: pub, State: state}
	if state == identity.DeviceStateActive {
		reg.ApprovedBy = &user
	}
	dev, err := w.id.Devices.Register(context.Background(), reg)
	if err != nil {
		w.t.Fatal(err)
	}
	set, err := w.id.Sessions.StartSession(context.Background(), identity.StartSession{UserID: user, DeviceID: dev.ID, AuthMethod: identity.AuthPasskey})
	if err != nil {
		w.t.Fatal(err)
	}
	c := &client{w: w, key: key, user: user, device: dev.ID}
	c.take(set)
	return c
}

// take stores a token set.
func (c *client) take(s identity.TokenSet) {
	c.access, c.refresh, c.session, c.refreshExpiresAt = s.AccessToken.Reveal(), s.RefreshToken.Reveal(), s.Session.ID, s.RefreshExpiresAt
}

// proof signs a DPoP proof of c's key for method and path, with ath of token when it is not "".
func (c *client) proof(key *identitytest.Key, method, path, token string) string {
	claims := identitytest.ProofClaims{HTM: method, HTU: httpservertest.PublicURL + path, IAT: c.w.clk.Now(), Nonce: c.w.id.DPoP.CurrentNonce()}
	if token != "" {
		claims.ATH = dpop.ATH(token)
	}
	return identitytest.SignDPoPProof(key, claims)
}

// call sends an authenticated request with the access token and a fresh proof.
func (c *client) call(method, path, body string) *httpservertest.Response {
	c.w.t.Helper()
	return c.w.h.Do(c.w.h.NewRequest(method, path, body, "Authorization", "DPoP "+c.access, "DPoP", c.proof(c.key, method, path, c.access)))
}

const refreshPath = "/api/v1/auth/token/refresh"

// refreshWith sends refreshToken with a proof of key and returns the response.
func (c *client) refreshWith(token string, key *identitytest.Key) *httpservertest.Response {
	c.w.t.Helper()
	body, _ := json.Marshal(map[string]string{"refresh_token": token})
	return c.w.h.Do(c.w.h.NewRequest(http.MethodPost, refreshPath, string(body), "DPoP", c.proof(key, http.MethodPost, refreshPath, "")))
}

// rotate refreshes with the current token and key, expects 200 and stores the new pair.
func (c *client) rotate() map[string]any {
	c.w.t.Helper()
	r := c.refreshWith(c.refresh, c.key)
	m := c.w.want(r, http.StatusOK, "")
	c.access, c.refresh = m["access_token"].(string), m["refresh_token"].(string)
	return m
}

// child creates the share extension session of c through the API.
func (c *client) child() *client {
	c.w.t.Helper()
	m := c.w.want(c.call(http.MethodPost, "/api/v1/me/sessions/extensions", `{"client_component":"share_ext"}`), http.StatusCreated, "")
	sess := m["session"].(map[string]any)
	return &client{w: c.w, key: c.key, user: c.user, device: c.device, session: ids.MustParse(sess["id"].(string)),
		access: m["access_token"].(string), refresh: m["refresh_token"].(string), parent: c}
}

// want checks status and, for a problem, its code; it returns the decoded body.
func (w *sworld) want(r *httpservertest.Response, status int, code string) map[string]any {
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
		if err := json.Unmarshal(r.Body, &m); err != nil {
			w.t.Fatalf("body: %v: %s", err, r.Body)
		}
	}
	return m
}

// sauditRow is one row of audit_events.
type sauditRow struct {
	Action, Decision, Reason, ResourceID, Raw string
	Details                                   map[string]any
}

// audits returns the audit rows of action, oldest first, after flushing the aggregated denials.
func (w *sworld) audits(action string) []sauditRow {
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
	var out []sauditRow
	for rows.Next() {
		var r sauditRow
		if err := rows.Scan(&r.Action, &r.Decision, &r.Reason, &r.ResourceID, &r.Raw); err != nil {
			w.t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(r.Raw), &r.Details); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// revokedEvents returns resource id, audience, actor and payload of every session.revoked.
func (w *sworld) revokedEvents() []map[string]string {
	w.t.Helper()
	rows, err := w.owner.Query(context.Background(), `SELECT resource_id::text, coalesce(audience_principal_id::text, ''),
		coalesce(actor_principal_id::text, ''), receive_action, payload::text FROM event_log WHERE type = 'session.revoked' ORDER BY seq`)
	if err != nil {
		w.t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		var res, aud, act, recv, payload string
		if err := rows.Scan(&res, &aud, &act, &recv, &payload); err != nil {
			w.t.Fatal(err)
		}
		out = append(out, map[string]string{"resource": res, "audience": aud, "actor": act, "receive": recv, "payload": payload})
	}
	return out
}

// sessionState returns revoke_reason of the session ("" while active).
func (w *sworld) sessionState(id ids.ID) string {
	w.t.Helper()
	var s string
	w.scalar(&s, `SELECT coalesce(revoke_reason, '') FROM auth_sessions WHERE id = $1`, id.String())
	return s
}

// familyState returns revoke_reason and current_generation of the family of session id that
// was created last.
func (w *sworld) familyState(id ids.ID) (string, int) {
	w.t.Helper()
	var reason string
	var gen int
	if err := w.owner.QueryRow(context.Background(), `SELECT coalesce(revoke_reason, ''), current_generation FROM refresh_token_families
		WHERE session_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1`, id.String()).Scan(&reason, &gen); err != nil {
		w.t.Fatal(err)
	}
	return reason, gen
}
