// SPDX-License-Identifier: Apache-2.0

// Package invocationtoken_test holds the cross-module tests of the invocation token (M2-06): the
// real signing keys of instance (M1-32) over PostgreSQL 18, the real getJwks handler behind the
// HTTP chain on 127.0.0.1 port 0, the invocation tokens of runs in two server processes and the
// reference verifier of runs/adapters/runstest standing in for the adapters.
package invocationtoken_test

import (
	"context"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/instance"
	instancehttp "github.com/OWNER/bothy/services/server/internal/instance/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/instance/adapters/keystore"
	instancepg "github.com/OWNER/bothy/services/server/internal/instance/adapters/postgres"
	instanceapp "github.com/OWNER/bothy/services/server/internal/instance/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/runs/adapters/runstest"
	runsapp "github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
	"github.com/OWNER/bothy/services/server/internal/wire"
	mw "github.com/OWNER/bothy/services/server/test/messagingworld"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Main(m)) }

// testKEK is the key-encryption key of the audit module of the key processes.
type testKEK struct{}

func (testKEK) KEK(context.Context, string) ([]byte, error) {
	return []byte(hex.EncodeToString(make([]byte, 32))), nil
}

// The configuration of these tests, set through the configuration schema of the server:
// signing_key.cache_ttl and agent.invocation_token_ttl plus auth.clock_leeway lie below
// agent.jwks_max_age, which lies below signing_key.invocation_rotation_interval;
// signing_key.rotation_check_interval is shorter than a token's validity, so promotions fall
// inside the validity of tokens. The reference verifiers use auth.clock_leeway as
// adapter.invocation_token_leeway (the edge of 09 4.5 "Keys and rotation" point 2).
var testEnv = []string{
	"BOTHY_SERVER__PUBLIC_URL=https://chat.example.com",
	"BOTHY_AGENT__INVOCATION_TOKEN_TTL=1m",
	"BOTHY_AUTH__CLOCK_LEEWAY=30s",
	"BOTHY_AGENT__JWKS_MAX_AGE=5m",
	"BOTHY_SIGNING_KEY__INVOCATION_ROTATION_INTERVAL=15m",
	"BOTHY_SIGNING_KEY__CACHE_TTL=30s",
	"BOTHY_SIGNING_KEY__ROTATION_CHECK_INTERVAL=20s",
}

// refreshMin is adapter.jwks_refresh_min_interval of the reference verifiers.
const refreshMin = 2 * time.Minute

// proc is one server process: its signing keys (own key store provider and verification cache)
// and its invocation tokens.
type proc struct {
	keys   *instanceapp.Keys
	tokens *runsapp.InvocationTokens
}

// world is the messaging world of M1-56 with Alex, Sam, the agent Lotse, the space Support with a
// text channel, a message of Alex in its main thread and a queued run of Lotse on it, plus two
// server processes A and B over the same database and the getJwks handler of A.
type world struct {
	t   *testing.T
	w   *mw.World
	cfg wire.SigningKeyConfig
	tok runsapp.InvocationTokenConfig
	dir string
	// logs captures the log of the key processes; the chain logs into jwks.Logs.
	logs *httpservertest.LogCapture
	a, b *proc
	jwks *httpservertest.Harness

	instanceID, alex, sam, lotse, space, channel, thread, message, run ids.ID
}

func newWorld(t *testing.T) *world {
	t.Helper()
	v, err := wireSchemaValues(t)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := wire.SigningKeysConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := wire.InvocationTokenConfigFrom(v)
	if err != nil {
		t.Fatal(err)
	}
	m := mw.New(t, func(*txn.Registry, *mw.Recorder) error { return nil })
	x := &world{t: t, w: m, cfg: cfg, tok: tok, logs: &httpservertest.LogCapture{}}
	x.instanceID = ids.New()
	m.SQL(`INSERT INTO instance (instance_id, schema_version, sync_epoch) VALUES ($1, 1, $2)`, x.instanceID.String(), ids.New().String())
	x.alex, x.sam = m.Person("alex", false), m.Person("sam", false)
	x.lotse = m.Agent("lotse")
	x.space = m.Space("Support")
	m.Grant("user", x.alex, x.space, "trigger")
	m.Grant("user", x.sam, x.space, "write")
	x.channel = m.Channel(x.space, "text", "general")
	x.thread = m.MainThread(x.channel)
	x.message = ids.MustParse(m.Send("alex", x.thread, "The export fails since this morning", http.StatusCreated)["id"].(string))
	x.run = ids.New()
	m.SQL(`INSERT INTO runs (id, thread_id, agent_id, trigger_kind, triggering_message_id, triggered_by, billing_mode_expected, invocation_jti)
		VALUES ($1, $2, $3, 'message', $4, $5, 'organization', $6)`, x.run.String(), x.thread.String(), x.lotse.String(), x.message.String(),
		x.alex.String(), ids.New().String())
	// The key store directory, mode 0700, in the temporary directory of the test.
	x.dir = filepath.Join(t.TempDir(), "signing-keys")
	if err := os.Mkdir(x.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	x.a, x.b = x.process(), x.process()
	if err := x.a.keys.EnsureActive(context.Background()); err != nil {
		t.Fatal(err)
	}
	built, err := routes.Build(wire.Handlers{Keys: instancehttp.NewKeys(x.a.keys, cfg.Keys.JWKSMaxAge)}, wire.RouteOptions(), routes.SpecCatalog())
	if err != nil {
		t.Fatal(err)
	}
	var rs []httpservertest.Route
	for _, r := range built {
		if r.Operation.ID == "getJwks" {
			rs = append(rs, httpservertest.Route{Op: wire.HTTPOperation(r.Operation), Handler: r.Handler.ServeHTTP})
		}
	}
	x.jwks = httpservertest.New(t, httpservertest.Options{Routes: rs, Contract: rest.Spec, ValidateResponses: true})
	return x
}

// wireSchemaValues loads the configuration of testEnv through the schema of the server.
func wireSchemaValues(t *testing.T) (*config.Values, error) {
	t.Helper()
	s, err := wire.ConfigSchema()
	if err != nil {
		return nil, err
	}
	return config.Load(s, config.Sources{Env: testEnv})
}

// process builds one server process over the world's database: an audit module of its own, a
// secret provider and key store over the shared key directory, the real Authorizer of the world,
// and the invocation tokens with the instance row as issuer.
func (x *world) process() *proc {
	t := x.t
	t.Helper()
	log := slog.New(x.logs)
	unit := pg.UnitConfig{RetryMax: 3, StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: 60 * time.Second}
	a, err := wire.NewAudit(x.w.Pool, unit, auditapp.Config{
		DenyAggregateWindow: time.Minute, DenyAggregateMaxKeys: 100, IPKeyPeriod: 30 * 24 * time.Hour,
		IPKeyRetention: 90 * 24 * time.Hour, IPKEKRef: "file:<secrets.dir>/audit_ip_kek",
	}, testKEK{}, log)
	if err != nil {
		t.Fatal(err)
	}
	p, err := secrets.NewProvider(secrets.Options{Dir: filepath.Dir(x.dir), SigningKeyDir: x.dir, CacheTTL: time.Minute, Clock: x.w.Clock, Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	ks, err := keystore.New(p, x.dir)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := instanceapp.NewKeys(x.cfg.Keys, instanceapp.KeysDeps{
		Repo: instancepg.NewRepository(x.w.Pool), Store: ks, UoW: a.UoW(), Authorizer: x.w.Authorizer, Audit: a.Recorder(),
		Clock: x.w.Clock, Log: log,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := &wire.Runs{}
	if err := r.BindInvocationTokens(x.tok, keys, instanceapp.NewState(instancepg.NewRepository(x.w.Pool), a.UoW()), x.w.Clock); err != nil {
		t.Fatal(err)
	}
	return &proc{keys: keys, tokens: r.Invocations}
}

// grant is the grant of a delivery of the run to Lotse.
func (x *world) grant() domain.InvocationGrant {
	return domain.InvocationGrant{
		AgentID: x.lotse, TriggeredBy: x.alex, TriggerKind: domain.TriggerMessage, RunID: x.run, MessageID: x.message,
		ThreadID: x.thread, ChannelID: x.channel, SpaceID: x.space, Level: "trigger", BillingMode: "organization",
		ToolClass: "chat", PermissionVersion: 1,
	}
}

// expect is the expectation of the adapter for the event of the run.
func (x *world) expect() runstest.ReferenceExpectation {
	return runstest.ReferenceExpectation{RunID: x.run, ThreadID: x.thread, MessageID: x.message, ActualBillingMode: "organization",
		ActualToolClass: "chat"}
}

// verify is the expectation of the server verifier for the run of Lotse.
func (x *world) verify() runsapp.VerifyInvocation {
	return runsapp.VerifyInvocation{Audience: x.lotse, RunID: x.run}
}

// verifier returns a reference verifier of Lotse against the JWKS of the world.
func (x *world) verifier() *runstest.ReferenceVerifier {
	x.t.Helper()
	v, err := runstest.NewReferenceVerifier(runstest.ReferenceOptions{
		JWKSURL: x.jwks.Server.URL + "/api/v1/instance/jwks", Client: x.jwks.Server.Client(), Clock: x.w.Clock,
		Issuer: x.instanceID, AgentID: x.lotse, Leeway: x.tok.ClockLeeway, RefreshMinInterval: refreshMin,
	})
	if err != nil {
		x.t.Fatal(err)
	}
	return v
}

// issue issues a token in process p.
func (x *world) issue(p *proc) runsapp.IssuedInvocationToken {
	x.t.Helper()
	got, err := p.tokens.Issue(context.Background(), x.grant())
	if err != nil {
		x.t.Fatalf("Issue: %v", err)
	}
	return got
}

// keyRow is one row of server_signing_keys of purpose invocation.
type keyRow struct {
	kid, state string
	created    time.Time
}

// invocationKeys reads the keys of purpose invocation as the database owner.
func (x *world) invocationKeys() []keyRow {
	x.t.Helper()
	rows, err := x.w.Owner.Query(context.Background(), `SELECT kid, state, created_at FROM server_signing_keys WHERE purpose = 'invocation' ORDER BY created_at, kid`)
	if err != nil {
		x.t.Fatal(err)
	}
	defer rows.Close()
	var out []keyRow
	for rows.Next() {
		var r keyRow
		if err := rows.Scan(&r.kid, &r.state, &r.created); err != nil {
			x.t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

// activeKID returns the kid of the active invocation key.
func (x *world) activeKID() string {
	x.t.Helper()
	for _, r := range x.invocationKeys() {
		if r.state == string(instance.KeyActive) {
			return r.kid
		}
	}
	x.t.Fatal("no active invocation key")
	return ""
}

// leaks reports whether s contains the token or one of its segments.
func leaks(s, token string) bool {
	if strings.Contains(s, token) {
		return true
	}
	for _, p := range strings.Split(token, ".") {
		if len(p) >= 8 && strings.Contains(s, p) {
			return true
		}
	}
	return false
}
