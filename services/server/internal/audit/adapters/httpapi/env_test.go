// SPDX-License-Identifier: Apache-2.0

//go:build unix

package httpapi_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/filesink"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/logs"
	"github.com/OWNER/bothy/services/server/internal/audit/adapters/otelmetrics"
	auditpg "github.com/OWNER/bothy/services/server/internal/audit/adapters/postgres"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/audit/domain"
	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/authz/adapters/fake"
	authzapp "github.com/OWNER/bothy/services/server/internal/authz/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/keystore"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// pageMax is api.page_size_max of these tests: small, so a walk and an export run in several
// batches.
const pageMax = 4

// keyCfg are the signing key values of these tests.
var keyCfg = instanceapp.KeysConfig{
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

// logLines captures the log lines of the module.
type logLines struct {
	mu    sync.Mutex
	lines []string
}

func (l *logLines) Enabled(context.Context, slog.Level) bool { return true }
func (l *logLines) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Level.String() + " " + r.Message)
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

// count returns how many lines contain s.
func (l *logLines) count(s string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, line := range l.lines {
		if strings.Contains(line, s) {
			n++
		}
	}
	return n
}

// env is a database, the audit module with its log, the real Authorizer and signing keys, a sink
// in a temporary directory (t.TempDir, from mktemp), a fake clock and the HTTP chain.
type env struct {
	t        *testing.T
	pool     *pg.Pool
	super    *pg.Pool
	audit    *wire.Audit
	log      *auditapp.Log
	keys     *instanceapp.Keys
	sink     auditapp.CheckpointSink
	sinkDir  string
	instance ids.ID
	clk      *clock.Fake
	auth     *tokenAuth
	people   *fake.Principals
	reader   *sdkmetric.ManualReader
	logs     *logLines
	h        *httpservertest.Harness
	// superConn is the superuser of the throwaway container, the attacker of SEC-T16.
	superConn *pgx.Conn
}

// newEnv builds the environment; withSink false leaves audit.checkpoint_sink empty.
func newEnv(t *testing.T, withSink bool) *env {
	t.Helper()
	db := pgtest.New(t)
	cfg := db.Config(pg.RoleApp)
	cfg.MaxConns = 8
	pool := db.OpenPool(t, cfg)
	unit := pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}
	ll := &logLines{}
	auditCfg := auditapp.Config{
		DenyAggregateWindow: time.Millisecond, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
		CheckpointInterval: time.Hour, PageSizeDefault: 2, PageSizeMax: pageMax,
	}
	a, err := wire.NewAudit(pool, unit, auditCfg, testKEK{}, slog.New(ll))
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
		Recorder: a.Recorder(), Clock: clk, Log: slog.New(ll),
	})
	if err != nil {
		t.Fatal(err)
	}
	keyDir := filepath.Join(t.TempDir(), "signing-keys")
	if err := os.Mkdir(keyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	provider, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(keyDir), SigningKeyDir: keyDir, CacheTTL: time.Minute, Clock: clk, Logger: slog.New(ll)})
	if err != nil {
		t.Fatal(err)
	}
	ks, err := keystore.New(provider, keyDir)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := instanceapp.NewKeys(keyCfg, instanceapp.KeysDeps{
		Repo: instancepg.NewRepository(pool), Store: ks, UoW: a.UoW(), Authorizer: az, Audit: a.Recorder(), Clock: clk, Log: slog.New(ll),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.EnsureActive(context.Background()); err != nil {
		t.Fatal(err)
	}
	inst := ids.New()
	var sink auditapp.CheckpointSink = auditapp.NoSink{}
	sinkDir := t.TempDir()
	if withSink {
		if sink, err = filesink.New(sinkDir, inst); err != nil {
			t.Fatal(err)
		}
	}
	reader := sdkmetric.NewManualReader()
	metrics, err := otelmetrics.New(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	if err != nil {
		t.Fatal(err)
	}
	l, err := auditapp.NewLog(auditCfg, auditapp.LogDeps{
		Store: auditpg.NewRepository(pool), UoW: a.UoW(), Authorizer: az, Audit: a.Recorder(), Keys: keys, Sink: sink,
		Cursors: httpapi.Cursors{}, Metrics: metrics, Reporter: logs.New(slog.New(ll)), Clock: clk, InstanceID: inst,
	})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{
		t: t, pool: pool, super: db.Pool(t, pg.RoleOwner), audit: a, log: l, keys: keys, sink: sink, sinkDir: sinkDir,
		instance: inst, clk: clk, auth: &tokenAuth{actors: map[string]actor.Actor{}}, people: people, reader: reader, logs: ll,
	}
	built, err := routes.Build(wire.Handlers{Log: httpapi.New(l)}, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		switch r.Operation.ID {
		case "listAuditEvents", "exportAuditEvents", "verifyAuditChain":
			rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
		}
	}
	e.h = httpservertest.New(t, httpservertest.Options{Routes: rs, Contract: rest.Spec, Authenticator: e.auth, ValidateResponses: true})
	e.superConn = db.SuperConn(t)
	return e
}

// person registers a principal under token: a user with a session and device (owner adds the
// owner role, stepUp the time of the last step-up, zero for none), or an agent with a token.
func (e *env) person(token string, kind actor.Kind, owner bool, stepUp time.Time) actor.Actor {
	e.t.Helper()
	id := ids.New()
	if _, err := e.super.Exec(context.Background(), `INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, $2, $3, $3)`,
		id.String(), string(kind), token); err != nil {
		e.t.Fatal(err)
	}
	e.people.Put(authz.PrincipalFacts{ID: id, Kind: kind, Status: authz.PrincipalActive, IsOwner: owner})
	a := actor.Actor{ID: id, Kind: kind, StepUpAt: stepUp}
	if kind == actor.KindUser {
		a.SessionID, a.DeviceID = ids.New(), ids.New()
	} else {
		a.TokenID = ids.New()
	}
	e.auth.mu.Lock()
	e.auth.actors[token] = a
	e.auth.mu.Unlock()
	return a
}

// owner registers an owner with a fresh step-up under token.
func (e *env) owner(token string) actor.Actor {
	e.t.Helper()
	return e.person(token, actor.KindUser, true, e.clk.Now())
}

// write records n allow entries of action by a (zero for no actor) through the recorder, each in
// its own unit of work, as the modules do.
func (e *env) write(n int, a actor.Actor, action, kind string, resource ids.ID) {
	e.t.Helper()
	ctx := context.Background()
	if !a.ID.IsZero() {
		ctx = actor.WithActor(ctx, a)
	}
	for range n {
		if err := e.audit.Recorder().RecordDetached(ctx, auditing.Entry{
			Action: action, Decision: auditing.DecisionAllow, ResourceKind: kind, ResourceID: resource,
		}); err != nil {
			e.t.Fatal(err)
		}
	}
}

// head returns the largest seq.
func (e *env) head() int64 {
	e.t.Helper()
	var n int64
	if err := e.super.QueryRow(context.Background(), "SELECT coalesce(max(seq), 0) FROM audit_events").Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// attack runs sql as the superuser of the throwaway container with the protection trigger of
// table switched off, the way an attacker with write access to the database would.
func (e *env) attack(table, trigger, sql string, args ...any) {
	e.t.Helper()
	ctx := context.Background()
	if _, err := e.superConn.Exec(ctx, "ALTER TABLE "+table+" DISABLE TRIGGER "+trigger); err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.superConn.Exec(ctx, sql, args...); err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.superConn.Exec(ctx, "ALTER TABLE "+table+" ENABLE TRIGGER "+trigger); err != nil {
		e.t.Fatal(err)
	}
}

// rewriteFrom recomputes prev_hash and hash of every row from seq k on, as an attacker who
// rewrites the chain consistently after changing row k.
func (e *env) rewriteFrom(k int64) {
	e.t.Helper()
	ctx := context.Background()
	repo := auditpg.NewRepository(e.pool)
	rows, err := repo.Rows(ctx, k, e.head())
	if err != nil {
		e.t.Fatal(err)
	}
	prev := rows[0].PrevHash
	for _, r := range rows {
		r.PrevHash = prev
		c, err := domain.CanonicalRow(r)
		if err != nil {
			e.t.Fatal(err)
		}
		r.Hash = domain.ChainHash(r.PrevHash, c)
		e.attack("audit_events", "audit_events_no_update_delete",
			"UPDATE audit_events SET prev_hash = $1, hash = $2 WHERE seq = $3", r.PrevHash[:], r.Hash[:], r.Seq)
		prev = r.Hash
	}
}

// verify posts verifyAuditChain with body as token and decodes the result.
func (e *env) verify(token, body string) (*httpservertest.Response, rest.AuditVerifyResult) {
	e.t.Helper()
	resp := e.h.Do(e.h.NewRequest(http.MethodPost, "/api/v1/audit/verify", body, "Authorization", "Bearer "+token))
	var res rest.AuditVerifyResult
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(resp.Body, &res); err != nil {
			e.t.Fatal(err)
		}
	}
	return resp, res
}

// get sends a GET to path as token.
func (e *env) get(token, path string) *httpservertest.Response {
	e.t.Helper()
	headers := []string{}
	if token != "" {
		headers = append(headers, "Authorization", "Bearer "+token)
	}
	return e.h.Do(e.h.NewRequest(http.MethodGet, path, "", headers...))
}

// run runs the job audit.chain_verify once.
func (e *env) run() {
	e.t.Helper()
	if err := e.log.RunCheckpoint(context.Background()); err != nil {
		e.t.Fatalf("RunCheckpoint: %v", err)
	}
}

// count returns the rows of audit_events matching where.
func (e *env) count(where string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.super.QueryRow(context.Background(), "SELECT count(*) FROM audit_events WHERE "+where, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// metrics returns the failure count and the last success gauge (0 when unset).
func (e *env) metrics() (int64, float64) {
	e.t.Helper()
	var rm metricdata.ResourceMetrics
	if err := e.reader.Collect(context.Background(), &rm); err != nil {
		e.t.Fatal(err)
	}
	var failures int64
	var last float64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Sum[int64]:
				for _, p := range d.DataPoints {
					failures += p.Value
				}
			case metricdata.Gauge[float64]:
				for _, p := range d.DataPoints {
					last = p.Value
				}
			}
		}
	}
	return failures, last
}

// sinkFiles returns the names of the checkpoint files of the instance.
func (e *env) sinkFiles() []string {
	e.t.Helper()
	des, err := os.ReadDir(filepath.Join(e.sinkDir, e.instance.String()))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		e.t.Fatal(err)
	}
	var out []string
	for _, de := range des {
		if strings.HasSuffix(de.Name(), ".jws") {
			out = append(out, de.Name())
		}
	}
	return out
}

// headOfFile returns the seq of a checkpoint file name.
func (e *env) headOfFile(name string) int64 {
	e.t.Helper()
	n, err := strconv.ParseInt(strings.TrimSuffix(name, ".jws"), 10, 64)
	if err != nil {
		e.t.Fatal(err)
	}
	return n
}

// signForeign returns the checkpoint of jws with another instance_id, signed by the active
// checkpoint key of this server: a valid signature over a payload of the wrong instance.
func (e *env) signForeign(t *testing.T, jws []byte) []byte {
	t.Helper()
	parts := strings.Split(string(jws), ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	c, err := domain.ParseCheckpoint(payload)
	if err != nil {
		t.Fatal(err)
	}
	c.InstanceID = ids.New()
	p, err := c.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s, err := e.keys.Signer(ctx, instance.PurposeAuditCheckpoint)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := jose.SignCompact(jose.SignerFunc{Algorithm: s.Alg(), Fn: func(in []byte) ([]byte, error) { return s.Sign(ctx, in) }},
		jose.Header{Typ: domain.CheckpointType, Kid: s.KeyID()}, p)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(tok)
}

// appendRow appends a valid row at occurred_at at to the chain, written directly with the
// database owner the way the audit hook writes it.
func (e *env) appendRow(at time.Time) {
	e.t.Helper()
	ctx := context.Background()
	head := e.head()
	var prev [domain.HashSize]byte
	if head > 0 {
		rows, err := auditpg.NewRepository(e.pool).Rows(ctx, head, head)
		if err != nil || len(rows) != 1 {
			e.t.Fatalf("head row: %v", err)
		}
		prev = rows[0].Hash
	}
	r := domain.Row{Seq: head + 1, ID: ids.New(), OccurredAt: at, Action: "grant.create", Decision: domain.DecisionAllow,
		Details: map[string]any{}, PrevHash: prev}
	c, err := domain.CanonicalRow(r)
	if err != nil {
		e.t.Fatal(err)
	}
	r.Hash = domain.ChainHash(prev, c)
	if _, err := e.super.Exec(ctx, `UPDATE sequence_counters SET value = $1 WHERE name = 'audit_seq'`, r.Seq); err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.super.Exec(ctx, `INSERT INTO audit_events (seq, id, occurred_at, action, decision, details, prev_hash, hash)
		VALUES ($1, $2, $3, $4, $5, '{}', $6, $7)`, r.Seq, r.ID.String(), at, r.Action, r.Decision, r.PrevHash[:], r.Hash[:]); err != nil {
		e.t.Fatal(err)
	}
}
