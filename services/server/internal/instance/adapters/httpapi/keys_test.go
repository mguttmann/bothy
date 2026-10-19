// SPDX-License-Identifier: Apache-2.0

//go:build unix

package httpapi_test

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/keystore"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	syncpg "github.com/OWNER/bothy/services/server/internal/sync/adapters/postgres"
	syncapp "github.com/OWNER/bothy/services/server/internal/sync/app"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// keyCfg are the signing key values of these tests; agent.jwks_max_age is 20 minutes.
var keyCfg = app.KeysConfig{
	CacheTTL: 30 * time.Second, InvocationRotation: 2 * time.Hour, AccessTokenRotation: 30 * 24 * time.Hour,
	JWKSMaxAge: 20 * time.Minute, AccessTokenTTL: 10 * time.Minute, InvocationTokenTTL: time.Minute,
	WebhookDeliveryTimeout: 15 * time.Second, ClockLeeway: 30 * time.Second,
}

// testKEK is the KEK source of the audit module.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(strings.Repeat("6b", 32)), nil
}

// liveCredentials reports every session, device and token as alive.
type liveCredentials struct{}

func (liveCredentials) Credentials(_ context.Context, ref authz.CredentialRef) (authz.CredentialFacts, error) {
	return authz.CredentialFacts{
		SessionActive: !ref.SessionID.IsZero(), SessionExpiresAt: time.Now().Add(time.Hour),
		DeviceState: authz.DeviceActive, TokenActive: !ref.TokenID.IsZero(),
	}, nil
}

// tokenAuth authenticates "Bearer <name>" as the actor registered under name.
type tokenAuth struct {
	mu     sync.Mutex
	actors map[string]actor.Actor
}

func (a *tokenAuth) Authenticate(_ context.Context, c actor.Credentials) (actor.Actor, error) {
	if c.Authorization == "" {
		return actor.Anonymous(), nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if p, ok := a.actors[strings.TrimPrefix(c.Authorization, "Bearer ")]; ok {
		return p, nil
	}
	return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "")
}

func (*tokenAuth) Revalidate(context.Context, actor.Actor) error { return nil }

// logLines captures the lines of the key use cases.
type logLines struct {
	mu    sync.Mutex
	lines []string
}

func (l *logLines) Enabled(context.Context, slog.Level) bool { return true }
func (l *logLines) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteString(" " + a.Key + "=" + a.Value.String())
		return true
	})
	l.mu.Lock()
	l.lines = append(l.lines, b.String())
	l.mu.Unlock()
	return nil
}
func (l *logLines) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *logLines) WithGroup(string) slog.Handler      { return l }

// env is the full chain with the Idempotency-Key stage, the real Authorizer and the keys.
type env struct {
	t      *testing.T
	super  *pg.Pool
	keys   *app.Keys
	clk    *clock.Fake
	dir    string
	auth   *tokenAuth
	people *fake.Principals
	h      *httpservertest.Harness
	logs   *logLines
	// seen collects every response and error text of the test for the canary.
	seen []string
	// pems collects every private key file the test saw in the key store.
	pems map[string][]byte
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 8
	pool := db.OpenPool(t, cfg)
	unit := pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}
	logs := &logLines{}
	a, err := wire.NewAudit(pool, unit, auditapp.Config{
		DenyAggregateWindow: time.Millisecond, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, testKEK{}, slog.New(logs))
	if err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(time.Now().UTC().Truncate(time.Second))
	people := fake.NewPrincipals()
	versions := &fake.Versions{}
	versions.Set(1)
	az, err := authzapp.NewAuthorizer(authzapp.AuthorizerConfig{SnapshotCacheMaxEntries: 100, StepUpMaxAge: 5 * time.Minute}, authzapp.AuthorizerDeps{
		Catalog: authzapp.NewCatalog(slog.New(slog.DiscardHandler)), Store: fake.NewStore(), Principals: people,
		Credentials: liveCredentials{}, Structure: fake.NewStructure(), Policies: fake.NewPolicies(), Versions: versions,
		Recorder: a.Recorder(), Clock: clk, Log: slog.New(logs),
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "signing-keys")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	provider, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(dir), SigningKeyDir: dir, CacheTTL: time.Minute, Clock: clk, Logger: slog.New(logs)})
	if err != nil {
		t.Fatal(err)
	}
	ks, err := keystore.New(provider, dir)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := app.NewKeys(keyCfg, app.KeysDeps{
		Repo: postgres.NewRepository(pool), Store: ks, UoW: a.UoW(), Authorizer: az, Audit: a.Recorder(), Clock: clk, Log: slog.New(logs),
	})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, super: db.Pool(t, pg.RoleOwner), keys: keys, clk: clk, dir: dir, people: people, logs: logs,
		auth: &tokenAuth{actors: map[string]actor.Actor{}}, pems: map[string][]byte{}}
	if err := keys.EnsureActive(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.collectPEMs()
	built, err := routes.Build(wire.Handlers{Keys: httpapi.NewKeys(keys, keyCfg.JWKSMaxAge)}, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		if r.Operation.ID == "getJwks" || r.Operation.ID == "rotateSigningKey" {
			rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
		}
	}
	stage, err := httpserver.NewIdempotencyStage(httpserver.IdempotencyConfig{TTL: time.Hour},
		syncapp.NewIdempotencyStore(syncpg.NewIdempotencyRepository(pool), a.UoW()), a.UoW(), clk)
	if err != nil {
		t.Fatal(err)
	}
	e.h = httpservertest.New(t, httpservertest.Options{
		Routes: rs, Contract: rest.Spec, Authenticator: e.auth,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: stage},
	})
	return e
}

// person registers a user under token; owner adds an active owners role. The actor gets a session
// and device, and a step-up at stepUp (zero for none).
func (e *env) person(token string, owner bool, stepUp time.Time) actor.Actor {
	e.t.Helper()
	id := ids.New()
	if _, err := e.super.Exec(context.Background(), `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, id.String(), token); err != nil {
		e.t.Fatal(err)
	}
	e.people.Put(authz.PrincipalFacts{ID: id, Kind: actor.KindUser, Status: authz.PrincipalActive, IsOwner: owner})
	a := actor.Actor{ID: id, Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New(), StepUpAt: stepUp}
	e.auth.mu.Lock()
	e.auth.actors[token] = a
	e.auth.mu.Unlock()
	return a
}

// rotate posts a rotation with token and key ("" for none) and records the response.
func (e *env) rotate(token, key, body string) *httpservertest.Response {
	e.t.Helper()
	headers := []string{}
	if token != "" {
		headers = append(headers, "Authorization", "Bearer "+token)
	}
	if key != "" {
		headers = append(headers, "Idempotency-Key", key)
	}
	resp := e.h.Do(e.h.NewRequest(http.MethodPost, "/api/v1/signing-keys/rotate", body, headers...))
	e.record(resp)
	e.collectPEMs()
	return resp
}

func (e *env) jwks(headers ...string) *httpservertest.Response {
	e.t.Helper()
	resp := e.h.Do(e.h.NewRequest(http.MethodGet, "/api/v1/instance/jwks", "", headers...))
	e.record(resp)
	return resp
}

func (e *env) record(resp *httpservertest.Response) {
	var b strings.Builder
	b.Write(resp.Body)
	for name, vs := range resp.Header {
		b.WriteString("\n" + name + ": " + strings.Join(vs, ","))
	}
	e.seen = append(e.seen, b.String())
}

// collectPEMs reads every key file in the key store, so the canary knows keys that are deleted
// later.
func (e *env) collectPEMs() {
	e.t.Helper()
	des, err := os.ReadDir(e.dir)
	if err != nil {
		e.t.Fatal(err)
	}
	for _, de := range des {
		b, err := os.ReadFile(filepath.Join(e.dir, de.Name()))
		if err != nil {
			e.t.Fatal(err)
		}
		e.pems[de.Name()] = b
	}
}

func body(purpose, mode, reason string) string {
	m := map[string]string{"purpose": purpose, "reason": reason}
	if mode != "" {
		m["mode"] = mode
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// auditRotations returns the details of every signing_key.rotate allow entry.
func (e *env) auditRotations() []map[string]string {
	e.t.Helper()
	rs, err := e.super.Query(context.Background(), `SELECT details FROM audit_events WHERE action = 'signing_key.rotate' AND decision = 'allow' ORDER BY seq`)
	if err != nil {
		e.t.Fatal(err)
	}
	defer rs.Close()
	var out []map[string]string
	for rs.Next() {
		var raw []byte
		if err := rs.Scan(&raw); err != nil {
			e.t.Fatal(err)
		}
		d := map[string]string{}
		if err := json.Unmarshal(raw, &d); err != nil {
			e.t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

func hexKID(t *testing.T, kid string) string {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(kid)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func TestRotateSigningKey_HTTP(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	now := e.clk.Now()
	e.person("owner", true, now)
	e.person("stale", true, now.Add(-10*time.Minute))
	e.person("member", false, now)
	pat := e.person("pat", true, time.Time{})
	pat.SessionID, pat.DeviceID, pat.TokenID = ids.ID{}, ids.ID{}, ids.New()
	e.auth.actors["pat"] = pat

	key := ids.New().String()
	resp := e.rotate("owner", key, body("access_token", "scheduled", "Scheduled rotation"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("owner: %d %s", resp.StatusCode, resp.Body)
	}
	var got rest.SigningKeyRotation
	if err := json.Unmarshal(resp.Body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Purpose != "access_token" || got.NewKid == "" || got.PreviousKid == "" || got.NewKid == got.PreviousKid ||
		!got.PreviousNotAfter.Equal(now.Add(keyCfg.JWKSMaxAge)) {
		t.Errorf("rotation %+v", got)
	}
	replay := e.rotate("owner", key, body("access_token", "scheduled", "Scheduled rotation"))
	var replayed rest.SigningKeyRotation
	if err := json.Unmarshal(replay.Body, &replayed); err != nil {
		t.Fatal(err)
	}
	if replay.StatusCode != http.StatusOK || replayed != got || replay.Header.Get("Idempotent-Replay") != "true" {
		t.Errorf("repeat with the same Idempotency-Key: %d %s %q, first %s", replay.StatusCode, replay.Body, replay.Header.Get("Idempotent-Replay"), resp.Body)
	}
	audits := e.auditRotations()
	if len(audits) != 1 {
		t.Fatalf("%d signing_key.rotate entries, want 1 (the replay writes none)", len(audits))
	}
	want := map[string]string{"purpose": "access_token", "mode": "scheduled", "new_kid": hexKID(t, got.NewKid),
		"previous_kid": hexKID(t, got.PreviousKid), "reason": "Scheduled rotation"}
	for k, v := range want {
		if audits[0][k] != v {
			t.Errorf("audit details[%s] = %q, want %q", k, audits[0][k], v)
		}
	}

	// audit_checkpoint is in SigningKeyPurpose and rotates on owner request (19 section 10).
	if resp := e.rotate("owner", ids.New().String(), body("audit_checkpoint", "emergency", "Key store leaked")); resp.StatusCode != http.StatusOK {
		t.Errorf("audit_checkpoint emergency: %d %s", resp.StatusCode, resp.Body)
	}

	for _, c := range []struct {
		name, token, body string
		status            int
		code, reason      string
	}{
		{"old step-up", "stale", body("invocation", "emergency", "x"), http.StatusUnauthorized, "step_up_required", ""},
		{"no owner", "member", body("invocation", "emergency", "x"), http.StatusForbidden, "forbidden", ""},
		// A personal token never carries the owner role (07 SR5), so step 6 of 07 section 4.2
		// refuses it before the step-up rule of step 8 (docs/DECISIONS.md M1-32).
		{"personal token", "pat", body("invocation", "emergency", "x"), http.StatusForbidden, "forbidden", "owner_only"},
		{"anonymous", "", body("invocation", "emergency", "x"), http.StatusUnauthorized, "unauthenticated", ""},
		{"unknown purpose", "owner", body("session", "scheduled", "x"), http.StatusBadRequest, "validation_failed", ""},
		{"missing mode", "owner", body("invocation", "", "x"), http.StatusBadRequest, "validation_failed", ""},
		{"empty reason", "owner", body("invocation", "scheduled", ""), http.StatusBadRequest, "validation_failed", ""},
	} {
		resp := e.rotate(c.token, ids.New().String(), c.body)
		if resp.StatusCode != c.status {
			t.Errorf("%s: %d %s, want %d", c.name, resp.StatusCode, resp.Body, c.status)
			continue
		}
		p := resp.Problem(t)
		if p["code"] != c.code || (c.reason != "" && p["reason"] != c.reason) {
			t.Errorf("%s: problem %v, want code %s reason %q", c.name, p, c.code, c.reason)
		}
	}
	if n := len(e.auditRotations()); n != 2 {
		t.Errorf("%d allow entries after the refused calls, want 2", n)
	}
}

func TestGetJwks_HTTP(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	owner := e.person("owner", true, e.clk.Now())
	rot := func(p instance.Purpose, mode instance.RotateMode) {
		t.Helper()
		if _, err := e.keys.Rotate(ctx, owner, instance.RotateCommand{Purpose: p, Mode: mode, Reason: "Test"}); err != nil {
			t.Fatal(err)
		}
	}
	// access_token: retiring, active and next; invocation: one retired, one active; the other
	// purposes are never published.
	rot(instance.PurposeAccessToken, instance.ModeScheduled)
	e.clk.Advance(keyCfg.JWKSMaxAge)
	if _, err := e.keys.RunRotation(ctx); err != nil {
		t.Fatal(err)
	}
	owner = e.person("owner2", true, e.clk.Now())
	rot(instance.PurposeAccessToken, instance.ModeScheduled)
	rot(instance.PurposeInvocation, instance.ModeEmergency)
	rot(instance.PurposeWebhookOut, instance.ModeScheduled)

	resp := e.jwks("Origin", "https://app.example.com")
	if resp.StatusCode != http.StatusOK {
		for _, r := range e.h.Logs.Records() {
			t.Logf("log %s %v", r.Message, r.Attrs)
		}
		t.Fatalf("anonymous GET: %d %s", resp.StatusCode, resp.Body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/jwk-set+json" {
		t.Errorf("Content-Type %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=1200" {
		t.Errorf("Cache-Control %q, want public, max-age=1200", cc)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" || resp.Header.Get("Access-Control-Allow-Credentials") != "" {
		t.Errorf("CORS headers %v", resp.Header)
	}
	var set struct {
		Keys []map[string]any `json:"keys"`
	}
	dec := json.NewDecoder(strings.NewReader(string(resp.Body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&set); err != nil {
		t.Fatalf("body does not match Jwks: %v", err)
	}
	rows, err := e.super.Query(ctx, `SELECT kid FROM server_signing_keys
		WHERE purpose IN ('access_token', 'invocation') AND state IN ('next', 'active', 'retiring')`)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for rows.Next() {
		var kid string
		if err := rows.Scan(&kid); err != nil {
			t.Fatal(err)
		}
		want = append(want, kid)
	}
	rows.Close()
	var gotKIDs []string
	for _, k := range set.Keys {
		members := make([]string, 0, len(k))
		for m := range k {
			members = append(members, m)
		}
		slices.Sort(members)
		if !slices.Equal(members, []string{"alg", "crv", "kid", "kty", "use", "x"}) {
			t.Errorf("JWK members %v", members)
		}
		if k["kty"] != "OKP" || k["crv"] != "Ed25519" || k["alg"] != "EdDSA" || k["use"] != "sig" {
			t.Errorf("JWK %v", k)
		}
		x, err := base64.RawURLEncoding.DecodeString(k["x"].(string))
		if err != nil || len(x) != ed25519.PublicKeySize || keystore.Thumbprint(x) != k["kid"] {
			t.Errorf("JWK %v: x is not the key of the kid", k)
		}
		gotKIDs = append(gotKIDs, k["kid"].(string))
	}
	slices.Sort(gotKIDs)
	slices.Sort(want)
	if !slices.Equal(gotKIDs, want) || len(want) != 4 {
		t.Errorf("JWKS kids %v, want the 4 published keys of access_token and invocation %v", gotKIDs, want)
	}
}

// TestSigningKeyCanary: after every transition and both HTTP operations no private key appears
// in a row of server_signing_keys or audit_events, a log line, a response or an error string,
// as PEM, base64url seed or hex seed.
func TestSigningKeyCanary(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	e.person("owner", true, e.clk.Now())
	for _, p := range []string{"access_token", "invocation", "webhook_out", "audit_checkpoint"} {
		for _, mode := range []string{"scheduled", "emergency"} {
			if resp := e.rotate("owner", ids.New().String(), body(p, mode, "Canary")); resp.StatusCode != http.StatusOK {
				t.Fatalf("%s %s: %d %s", p, mode, resp.StatusCode, resp.Body)
			}
		}
	}
	e.rotate("owner", ids.New().String(), body("terminal", "scheduled", "Canary"))
	e.rotate("", ids.New().String(), body("invocation", "emergency", "Canary"))
	e.rotate("owner", ids.New().String(), body("access_token", "scheduled", "Canary"))
	e.clk.Advance(keyCfg.JWKSMaxAge + keyCfg.AccessTokenTTL + time.Hour)
	for range 3 {
		if _, err := e.keys.RunRotation(ctx); err != nil {
			e.seen = append(e.seen, err.Error())
		}
		e.collectPEMs()
		e.clk.Advance(time.Hour)
	}
	e.jwks()
	for _, call := range []func() error{
		func() error { _, err := e.keys.VerificationKey(ctx, instance.PurposeInvocation, "unknown"); return err },
		func() error { _, err := e.keys.PublicKey(ctx, "unknown"); return err },
		func() error {
			_, err := e.keys.Rotate(ctx, actor.Anonymous(), instance.RotateCommand{Purpose: "terminal", Mode: "x"})
			return err
		},
	} {
		if err := call(); err != nil {
			e.seen = append(e.seen, err.Error(), errs.KindOf(err).String())
		}
	}
	// A signature is no key material either.
	if s, err := e.keys.Signer(ctx, instance.PurposeAccessToken); err == nil {
		sig, _ := s.Sign(ctx, []byte("x"))
		e.seen = append(e.seen, string(sig))
	}

	var haystack []string
	for _, q := range []string{"SELECT row_to_json(k)::text FROM server_signing_keys k", "SELECT row_to_json(a)::text FROM audit_events a"} {
		rows, err := e.super.Query(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				t.Fatal(err)
			}
			haystack = append(haystack, s)
		}
		rows.Close()
	}
	e.logs.mu.Lock()
	haystack = append(haystack, e.logs.lines...)
	e.logs.mu.Unlock()
	for _, r := range e.h.Logs.Records() {
		var b strings.Builder
		b.WriteString(r.Message)
		for k, v := range r.Attrs {
			b.WriteString(" " + k + "=" + v)
		}
		haystack = append(haystack, b.String())
	}
	haystack = append(haystack, e.seen...)
	if len(e.pems) < 12 {
		t.Fatalf("the test saw %d key files, want every generated key", len(e.pems))
	}
	all := strings.Join(haystack, "\n")
	for name, raw := range e.pems {
		block, _ := pem.Decode(raw)
		if block == nil {
			t.Fatalf("%s is not PEM", name)
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		seed := parsed.(ed25519.PrivateKey).Seed()
		body := strings.Join(strings.Split(strings.TrimSpace(string(raw)), "\n")[1:], "")
		body = strings.TrimSuffix(body, "-----END PRIVATE KEY-----")
		for form, needle := range map[string]string{
			"PEM": string(raw), "PEM body": body, "base64url seed": base64.RawURLEncoding.EncodeToString(seed),
			"base64 seed": base64.StdEncoding.EncodeToString(seed), "hex seed": hex.EncodeToString(seed),
		} {
			if strings.Contains(all, needle) {
				t.Errorf("the private key of %s appears as %s", name, form)
			}
		}
	}
}
