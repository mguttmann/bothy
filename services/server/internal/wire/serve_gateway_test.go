// SPDX-License-Identifier: Apache-2.0

package wire_test

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	auditapp "github.com/OWNER/bothy/services/server/internal/audit/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/config"
	"github.com/OWNER/bothy/services/server/internal/platform/pg"
	"github.com/OWNER/bothy/services/server/internal/platform/pg/pgtest"
	"github.com/OWNER/bothy/services/server/internal/platform/secrets"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/wire"
)

// The gateway world: a started api process with short gateway windows, and identity built beside
// it over the same installation, so tests issue tokens and sign proofs the process accepts.
const (
	gwPublicURL = "https://chat.example.com"
	gwHTU       = gwPublicURL + "/api/v1/gateway"
)

// gwWorld is one api process and its side identity.
type gwWorld struct {
	t  *testing.T
	db *pgtest.DB
	// owner is the one pool of the schema owner for sql; a pool per statement would keep one
	// connection each until the suite ends and exhaust max_connections next to the other tests.
	owner  *pg.Pool
	p      *process
	base   string
	id     *wire.Identity
	pepper []byte
	cfg    wire.GatewayConfig
	// clk is the clock of the process and of the side identity (deps.Clock or the system clock).
	clk clock.Clock
	// env is the environment of the process, for a second process on the same database.
	env []string
	// secrets are every token, proof and nonce of the world; reasons every close reason seen.
	mu      gosync.Mutex
	secrets []string
	reasons []string
	// app is the bothy_app pool of the side identity; the restore tests close it, because
	// bothy restore finalize refuses while a bothy_app session is connected (M7-04).
	app *pg.Pool
}

// gwHeartbeatInterval is the sync.heartbeat_interval of a gateway world: the default of the key.
const gwHeartbeatInterval = 30 * time.Second

// gwAccessTokenTTL is the auth.access_token_ttl of a gateway world, longer than any run of the
// package: its people get their access token once, when the world starts, and a suite on a loaded
// runner can take longer than the default ten minutes (TestCommandAutocomplete: its Owner got 401
// token_expired in the middle of a subtest). A test of token expiry sets the key
// itself.
const gwAccessTokenTTL = 24 * time.Hour

// patientWindows are the environment entries of testkit.PatientGateway for the heartbeat interval
// interval: no heartbeat or identify deadline of the gateway closes a connection of a world
// before the test has waited testkit.Patience for what it checks, however slow the host.
func patientWindows(interval time.Duration) []string {
	var env []string
	for k, v := range testkit.PatientGateway(interval) {
		env = append(env, config.EnvName(k)+"="+v)
	}
	slices.Sort(env)
	return env
}

// newGatewayWorld starts an api process with the gateway windows of extra (environment entries)
// over patientWindows(gwHeartbeatInterval); only a test of a deadline sets that window short.
func newGatewayWorld(t *testing.T, extra ...string) *gwWorld {
	t.Helper()
	return newGatewayWorldWith(t, wire.Deps{}, extra...)
}

// newGatewayWorldWith is newGatewayWorld with the dependencies deps of the process; a clock of
// deps is the clock of the side identity and of the proofs too.
func newGatewayWorldWith(t *testing.T, deps wire.Deps, extra ...string) *gwWorld {
	t.Helper()
	return newGatewayWorldRole(t, wire.RoleAPI, deps, extra...)
}

// newGatewayWorldAs starts a process in role with the gateway windows of extra (role all for the
// desktop hints of M3-23, whose consumer and jobs share the process with the gateway).
func newGatewayWorldAs(t *testing.T, role wire.Role, extra ...string) *gwWorld {
	t.Helper()
	return newGatewayWorldRole(t, role, wire.Deps{}, extra...)
}

// newGatewayWorldRole starts a process in role with the dependencies deps and the gateway windows
// of extra over patientWindows(gwHeartbeatInterval).
func newGatewayWorldRole(t *testing.T, role wire.Role, deps wire.Deps, extra ...string) *gwWorld {
	t.Helper()
	ctx := context.Background()
	db := newServeDB(t)
	completeSetup(t, db)
	base := append([]string{"BOTHY_SERVER__PUBLIC_URL=" + gwPublicURL, "BOTHY_AUTH__ACCESS_TOKEN_TTL=" + strconv.Itoa(int(gwAccessTokenTTL.Hours())) + "h"},
		patientWindows(gwHeartbeatInterval)...)
	env := serveEnv(t, db, append(base, extra...)...)
	p := start(t, role, env, deps)
	waitReadyz(t, p.waitInternal(t), http.StatusOK)
	public := p.logs.port("public listener started")
	values := loadEnv(t, env)
	cfg, err := wire.IdentityConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	keyCfg, err := wire.SigningKeysConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	keyCfg.Dir = envValue(env, "BOTHY_SECRETS__SIGNING_KEY_DIR")
	sp, err := secrets.NewProvider(secrets.Options{Dir: envValue(env, "BOTHY_SECRETS__DIR"), SigningKeyDir: keyCfg.Dir, CacheTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	pepper, err := wire.TokenPepper(ctx, sp, cfg.TokenPepperRef)
	if err != nil {
		t.Fatal(err)
	}
	pool := db.Pool(t, pg.RoleApp)
	log := slog.New(slog.DiscardHandler)
	a, err := wire.NewAudit(pool, pg.UnitConfig{StatementTimeout: 30 * time.Second, LockTimeout: 10 * time.Second, IdleInTxTimeout: time.Minute},
		auditapp.Config{DenyAggregateWindow: time.Minute, DenyAggregateMaxKeys: 10, IPKeyPeriod: 24 * time.Hour, IPKeyRetention: 48 * time.Hour,
			IPKEKRef: "file:<secrets.dir>/audit_ip_kek"}, testKEK{}, log)
	if err != nil {
		t.Fatal(err)
	}
	syncCfg, err := wire.SyncConfigFrom(values)
	if err != nil {
		t.Fatal(err)
	}
	sy, err := wire.NewSync(pool, a.UoW(), syncCfg, nil, log)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := wire.NewSigningKeys(keyCfg, pool, sp, a.UoW(), allowAll{}, a.Recorder(), nil, log)
	if err != nil {
		t.Fatal(err)
	}
	instCfg, err := wire.InstanceConfig(values)
	if err != nil {
		t.Fatal(err)
	}
	st, err := wire.NewInstanceSettings(instCfg, true, pool, a.UoW(), allowAll{}, wire.SettingsWrites{Audit: a.Recorder(), Events: sy.Appender()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := wire.NewIdentity(cfg, pool, a.UoW(), allowAll{}, wire.IdentityWrites{Audit: a.Recorder(), Events: sy.Appender(), Settings: st.Admin, Flags: st.Admin}, pepper, deps.Clock, log)
	if err != nil {
		t.Fatal(err)
	}
	var instanceID string
	owner := db.Pool(t, pg.RoleOwner)
	if err := owner.QueryRow(ctx, `SELECT instance_id::text FROM instance`).Scan(&instanceID); err != nil {
		t.Fatal(err)
	}
	if err := id.BindSessions(keys.Keys, ids.MustParse(instanceID)); err != nil {
		t.Fatal(err)
	}
	gcfg, err := wire.GatewayConfigFrom(values)
	if err != nil {
		t.Fatal(err)
	}
	clk := deps.Clock
	if clk == nil {
		clk = clock.System()
	}
	return &gwWorld{t: t, db: db, owner: owner, p: p, base: "http://127.0.0.1:" + strconv.Itoa(public), id: id, pepper: pepper, cfg: gcfg, env: env, clk: clk, app: pool}
}

// sql runs a statement as the schema owner.
func (w *gwWorld) sql(query string, args ...any) {
	w.t.Helper()
	if _, err := w.owner.Exec(context.Background(), query, args...); err != nil {
		w.t.Fatal(err)
	}
}

// remember records secrets that must never show up in a close reason or a log line.
func (w *gwWorld) remember(s ...string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.secrets = append(w.secrets, s...)
}

// gwPerson is a person with a device, its key and a session.
type gwPerson struct {
	user, device ids.ID
	key          *identitytest.Key
	set          identity.TokenSet
	access       string
}

// newPerson creates a person with one device in state and a session of it.
func (w *gwWorld) newPerson(handle string, state identity.DeviceState) gwPerson {
	w.t.Helper()
	ctx := context.Background()
	user := ids.New()
	w.sql(`INSERT INTO principals (id, kind, handle, display_name) VALUES ($1, 'user', $2, $2)`, user.String(), handle+strings.ReplaceAll(user.String()[24:], "-", ""))
	w.sql(`INSERT INTO users (principal_id) VALUES ($1)`, user.String())
	key, pub := identitytest.NewDeviceKey(jose.AlgEdDSA)
	in := identity.RegisterDevice{DeviceID: ids.New(), UserID: user, Name: "Laptop", Platform: identity.PlatformMacOS, DPoPKey: pub, State: state}
	if state == identity.DeviceStateActive {
		in.ApprovedBy = &user
	}
	dev, err := w.id.Devices.Register(ctx, in)
	if err != nil {
		w.t.Fatal(err)
	}
	set, err := w.id.Sessions.StartSession(ctx, identity.StartSession{UserID: user, DeviceID: dev.ID, AuthMethod: identity.AuthPasskey})
	if err != nil {
		w.t.Fatal(err)
	}
	p := gwPerson{user: user, device: dev.ID, key: key, set: set, access: set.AccessToken.Reveal()}
	w.remember(p.access, set.RefreshToken.Reveal())
	return p
}

// proofFor returns the proof function of key over token with htu.
func (w *gwWorld) proofFor(key *identitytest.Key, token, htu string) func(string) string {
	return func(nonce string) string {
		pr := identitytest.SignDPoPProof(key, identitytest.ProofClaims{HTM: http.MethodGet, HTU: htu, IAT: w.clk.Now(), Nonce: nonce, ATH: dpop.ATH(token)})
		w.remember(pr, nonce)
		return pr
	}
}

// params returns the identify parameters of p with a valid proof.
func (w *gwWorld) params(p gwPerson) wstest.IdentifyParams {
	return wstest.IdentifyParams{AccessToken: p.access, DeviceID: p.device.String(), Proof: w.proofFor(p.key, p.access, gwHTU)}
}

// agentToken stores an agent API token for principal without scopes, bound to key when not nil.
func (w *gwWorld) agentToken(principal ids.ID, key *identitytest.Key, expires time.Time) string {
	w.t.Helper()
	tok, err := apitoken.Generate(nil, apitoken.KindAgent)
	if err != nil {
		w.t.Fatal(err)
	}
	pp, _ := apitoken.Parse(tok)
	var jkt any
	if key != nil {
		jkt = key.Thumbprint
	}
	w.sql(`INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, scopes, dpop_jkt, expires_at, created_by)
		VALUES ($1, $2, 'x', $3, $4, $5, $6, $7, $2)`, ids.New().String(), principal.String(), pp.Prefix(), apitoken.Hash(w.pepper, tok), []string{}, jkt, expires)
	w.remember(tok)
	return tok
}

// newAgent creates an agent principal with its agents record.
func (w *gwWorld) newAgent() ids.ID {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO principals (id, kind, status, handle, display_name) VALUES ($1, 'agent', 'active', $2, $2)`, id.String(), "agent"+strings.ReplaceAll(id.String()[24:], "-", ""))
	// Every agent principal has its record (M1-51); the consumer cursor of M2-07 refers to it.
	w.sql(`INSERT INTO agents (principal_id, adapter_kind) VALUES ($1, 'custom')`, id.String())
	return id
}

// dial connects with the instance origin through the harness, which validates every frame of
// the connection for t (G9) and drops it in t's cleanup.
func (w *gwWorld) dial(t testing.TB, opts ...wstest.Option) *wstest.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	c, err := wstest.Dial(t, ctx, w.base, append([]wstest.Option{wstest.WithOrigin(gwPublicURL), wstest.WithMaxFrameBytes(w.cfg.MaxFrameBytes)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	w.remember(c.Hello().DpopNonce)
	return c
}

// expectClose waits for the close of c and requires code.
func (w *gwWorld) expectClose(t *testing.T, c *wstest.Client, code int) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	got, reason, err := c.ExpectClose(ctx)
	if err != nil {
		t.Fatalf("no close: %v", err)
	}
	w.mu.Lock()
	w.reasons = append(w.reasons, reason)
	w.mu.Unlock()
	if got != code {
		t.Fatalf("closed with %d %q, want %d", got, reason, code)
	}
	if len(reason) > 123 {
		t.Errorf("close reason of %d bytes", len(reason))
	}
	return reason
}

// identifyErr identifies on a fresh connection dialed with opts and requires the close code.
func (w *gwWorld) identifyErr(t *testing.T, p wstest.IdentifyParams, code int, opts ...wstest.Option) {
	t.Helper()
	c := w.dial(t, opts...)
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	_, err := c.Identify(ctx, p)
	var ce *wstest.CloseError
	if !errors.As(err, &ce) {
		t.Fatalf("identify: %v, want close %d", err, code)
	}
	w.mu.Lock()
	w.reasons = append(w.reasons, ce.Reason)
	w.mu.Unlock()
	if ce.Code != code {
		t.Fatalf("identify closed with %d %q, want %d", ce.Code, ce.Reason, code)
	}
}

// ready identifies on a fresh connection and returns the client and ready.
func (w *gwWorld) ready(t *testing.T, p wstest.IdentifyParams, opts ...wstest.Option) (*wstest.Client, rest.GatewayReady) {
	t.Helper()
	c := w.dial(t, opts...)
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	r, err := c.Identify(ctx, p)
	if err != nil {
		t.Fatalf("identify: %v", err)
	}
	return c, r
}

// sessions maps a client to the gateway session id of its ready.
var sessions gosync.Map

// liveSession returns the gateway session id of c.
func liveSession(t *testing.T, c *wstest.Client) string {
	t.Helper()
	v, ok := sessions.Load(c)
	if !ok {
		t.Fatal("no session recorded")
	}
	return v.(string)
}

// gwFastBeat is the heartbeat interval of TestGateway: short, so a revocation is seen at the next
// heartbeat within seconds; patientWindows keeps its deadline beyond testkit.Patience.
const gwFastBeat = 2 * time.Second

// TestGateway is the integration suite of M1-64 against a started api process (PostgreSQL 18 via
// pgtest, people, devices, tokens and proofs from identity, the wstest client), with
// patientWindows(gwFastBeat), the default sync.resume_window and sync.ws_max_frame_bytes 4096.
// The deadlines themselves are tested in TestGatewayDeadlines.
func TestGateway(t *testing.T) {
	t.Parallel()
	w := newGatewayWorld(t, append(patientWindows(gwFastBeat), "BOTHY_SYNC__WS_MAX_FRAME_BYTES=4096")...)
	ctx := context.Background()

	t.Run("TestSECCORS3ForeignOriginRejected", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.base+"/api/v1/gateway", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Origin", "https://evil.example.com")
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Version", "13")
		// The handshake key is 16 bytes in base64 (RFC 6455 section 4.1); any value works here.
		req.Header.Set("Sec-WebSocket-Key", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 16))))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(body), `"code":"forbidden"`) {
			t.Fatalf("foreign origin: %d %s", resp.StatusCode, body)
		}
		for _, o := range []string{"http://chat.example.com", "https://chat.example.com:8443", "null"} {
			if status, err := wstest.DialStatus(ctx, w.base, wstest.WithOrigin(o)); err != nil || status != http.StatusForbidden {
				t.Errorf("origin %s: status %d, %v", o, status, err)
			}
		}
		w.dial(t)
		c, err := wstest.Dial(t, ctx, w.base, wstest.WithMaxFrameBytes(w.cfg.MaxFrameBytes))
		if err != nil {
			t.Fatalf("without Origin: %v", err)
		}
		c.Abort()
	})

	t.Run("TestHelloFields", func(t *testing.T) {
		c := w.dial(t)
		h := c.Hello()
		var epoch string
		if err := w.owner.QueryRow(ctx, `SELECT sync_epoch::text FROM instance`).Scan(&epoch); err != nil {
			t.Fatal(err)
		}
		if h.HeartbeatIntervalMs != gwFastBeat.Milliseconds() || h.ResumeWindowMs != (5*time.Minute).Milliseconds() || len(h.ProtocolVersions) != 1 || h.ProtocolVersions[0] != 1 ||
			h.SyncEpoch.String() != epoch || h.DpopNonce == "" || time.Since(h.ServerTime) > time.Minute { // nosemgrep: bothy-go-test-wall-clock-wait -- the clock of the hello, not a wait
			t.Fatalf("hello %+v", h)
		}
		p := w.newPerson("alex", identity.DeviceStateActive)
		ctx, cancel := context.WithTimeout(ctx, testkit.Patience)
		defer cancel()
		if _, err := c.Identify(ctx, w.params(p)); err != nil {
			t.Fatalf("identify with the nonce of hello: %v", err)
		}
	})

	t.Run("TestIdentifyPersonReady", func(t *testing.T) {
		p := w.newPerson("sam", identity.DeviceStateActive)
		_, r := w.ready(t, w.params(p))
		if r.Snapshot.HeadSeq != r.HeadSeq || r.Principal.Id.String() != p.user.String() || r.DeviceId == nil || r.DeviceId.String() != p.device.String() ||
			r.SyncEpoch != r.Snapshot.SyncEpoch || r.PermissionVersion != r.Snapshot.PermissionVersion {
			t.Fatalf("ready %+v", r)
		}
	})

	t.Run("TestIdentifyAgentToken", func(t *testing.T) {
		agent := w.newAgent()
		key, _ := identitytest.NewDeviceKey(jose.AlgES256)
		tok := w.agentToken(agent, key, time.Now().Add(time.Hour))
		_, r := w.ready(t, wstest.IdentifyParams{APIToken: tok, Proof: w.proofFor(key, tok, gwHTU)})
		if r.Principal.Id.String() != agent.String() || r.DeviceId != nil {
			t.Fatalf("ready %+v", r)
		}
		w.identifyErr(t, wstest.IdentifyParams{APIToken: tok}, 4004)
	})

	t.Run("TestClose4001UnknownOp", func(t *testing.T) {
		// nope is no op of the contract: a negative test, so only server frames are validated.
		c := w.dial(t, wstest.WithRawClientFrames())
		if err := c.Send("nope", map[string]any{}); err != nil {
			t.Fatal(err)
		}
		w.expectClose(t, c, 4001)
	})

	t.Run("TestGatewayOpsEnableList", func(t *testing.T) {
		p := w.newPerson("robin", identity.DeviceStateActive)
		c, _ := w.ready(t, w.params(p))
		// focus is enabled since M1-66 and ack since M2-07; stats.subscribe still waits for M6-15.
		if err := c.Send("stats.subscribe", map[string]any{}); err != nil {
			t.Fatal(err)
		}
		w.expectClose(t, c, 4001)
	})

	t.Run("TestClose4002DecodeError", func(t *testing.T) {
		cases := map[string]func(c *wstest.Client) error{
			"bad JSON":      func(c *wstest.Client) error { return c.SendRaw([]byte(`{"op":"identify","d":`)) },
			"duplicate key": func(c *wstest.Client) error { return c.SendRaw([]byte(`{"op":"identify","op":"identify","d":{}}`)) },
			"missing op":    func(c *wstest.Client) error { return c.SendRaw([]byte(`{"d":{}}`)) },
			"unknown member": func(c *wstest.Client) error {
				return c.Send("identify", map[string]any{"protocol_version": 1, "api_token": "x", "zzz": 1})
			},
			"unknown envelope member": func(c *wstest.Client) error { return c.SendRaw([]byte(`{"op":"identify","d":{},"x":1}`)) },
			"binary frame":            func(c *wstest.Client) error { return c.SendBinary([]byte(`{"op":"identify","d":{}}`)) },
			"too large": func(c *wstest.Client) error {
				return c.SendRaw([]byte(`{"op":"identify","d":{"protocol_version":1,"api_token":"` + strings.Repeat("a", 4100) + `"}}`))
			},
		}
		for name, send := range cases {
			t.Run(name, func(t *testing.T) {
				// Invalid on purpose: only the server frames are validated.
				c := w.dial(t, wstest.WithRawClientFrames())
				if err := send(c); err != nil {
					t.Fatal(err)
				}
				reason := w.expectClose(t, c, 4002)
				if name == "too large" && reason != "frame_too_large" {
					t.Errorf("reason %q", reason)
				}
			})
		}
	})

	t.Run("TestClose4003OpBeforeIdentify", func(t *testing.T) {
		c := w.dial(t)
		if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
			t.Fatal(err)
		}
		w.expectClose(t, c, 4003)
	})

	t.Run("TestClose4004BadProof", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		wrongHTU := w.params(p)
		wrongHTU.Proof = w.proofFor(p.key, p.access, "https://chat.example.com/api/v1/other")
		w.identifyErr(t, wrongHTU, 4004)
		wrongNonce := w.params(p)
		wrongNonce.Proof = func(string) string { return w.proofFor(p.key, p.access, gwHTU)("not-the-nonce") }
		w.identifyErr(t, wrongNonce, 4004)
		missing := w.params(p)
		missing.Proof = nil
		w.identifyErr(t, missing, 4004)
		var used string
		replay := w.params(p)
		replay.Proof = func(n string) string { used = w.proofFor(p.key, p.access, gwHTU)(n); return used }
		w.ready(t, replay)
		again := w.params(p)
		again.Proof = func(string) string { return used }
		w.identifyErr(t, again, 4004)
		other := w.newPerson("sam", identity.DeviceStateActive)
		mismatch := w.params(p)
		mismatch.DeviceID = other.device.String()
		w.identifyErr(t, mismatch, 4004)
	})

	t.Run("TestClose4005SecondIdentify", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		c, _ := w.ready(t, w.params(p))
		pr := w.params(p)
		if err := c.Send("identify", map[string]any{"protocol_version": 1, "access_token": p.access, "dpop": pr.Proof(c.Hello().DpopNonce), "device_id": pr.DeviceID}); err != nil {
			t.Fatal(err)
		}
		w.expectClose(t, c, 4005)
	})

	t.Run("TestClose4006ResumeUnknown", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		c := w.dial(t)
		cctx, cancel := context.WithTimeout(ctx, testkit.Patience)
		defer cancel()
		_, err := c.Resume(cctx, wstest.ResumeParams{SessionID: ids.New().String(), AccessToken: p.access, Proof: w.proofFor(p.key, p.access, gwHTU)})
		if ce := (*wstest.CloseError)(nil); !errors.As(err, &ce) || ce.Code != 4006 {
			t.Fatalf("resume of an unknown session: %v", err)
		}
	})

	t.Run("TestResumeWithinWindow", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		c, r := w.ready(t, w.params(p))
		c.Abort()
		time.Sleep(100 * time.Millisecond)
		n := w.dial(t)
		cctx, cancel := context.WithTimeout(ctx, testkit.Patience)
		defer cancel()
		res, err := n.Resume(cctx, wstest.ResumeParams{SessionID: r.GatewaySessionId.String(), AccessToken: p.access, Proof: w.proofFor(p.key, p.access, gwHTU), Cursor: r.HeadSeq})
		if err != nil {
			t.Fatal(err)
		}
		if res.GatewaySessionId != r.GatewaySessionId || res.SyncEpoch != r.SyncEpoch {
			t.Fatalf("resumed %+v", res)
		}
	})

	t.Run("TestClose4007ProtocolVersion", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		params := w.params(p)
		params.ProtocolVersion = 2
		w.identifyErr(t, params, 4007)
	})

	t.Run("TestClose4008RateLimited", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		var last *wstest.Client
		for i := 0; ; i++ {
			c := w.dial(t)
			cctx, cancel := context.WithTimeout(ctx, testkit.Patience)
			_, err := c.Identify(cctx, w.params(p))
			cancel()
			if err == nil {
				last = c
				if i > 20 {
					t.Fatal("no rate limit after 20 identifies")
				}
				continue
			}
			var ce *wstest.CloseError
			if !errors.As(err, &ce) || ce.Code != 4008 || !strings.HasPrefix(ce.Reason, "retry_after=") {
				t.Fatalf("identify %d: %v", i, err)
			}
			break
		}
		if last == nil {
			t.Fatal("not a single identify passed")
		}
	})

	t.Run("TestClose4012RevokedAtIdentify", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		// Revoked in the table: the api process creates no event_log partition for the event of
		// RevokeSession, and the gateway reads the state, not the event.
		w.sql(`UPDATE auth_sessions SET revoked_at = now(), revoke_reason = 'admin' WHERE id = $1`, p.set.Session.ID.String())
		w.identifyErr(t, w.params(p), 4012)
	})

	t.Run("TestClose4012RevokedAtHeartbeat", func(t *testing.T) {
		agent := w.newAgent()
		tok := w.agentToken(agent, nil, time.Now().Add(time.Hour))
		c, _ := w.ready(t, wstest.IdentifyParams{APIToken: tok})
		w.sql(`UPDATE api_tokens SET revoked_at = now(), revoked_by = principal_id WHERE principal_id = $1`, agent.String())
		w.expectClose(t, c, 4012)

		p := w.newPerson("sam", identity.DeviceStateActive)
		pc, _ := w.ready(t, w.params(p))
		w.sql(`UPDATE auth_sessions SET revoked_at = now(), revoke_reason = 'admin' WHERE id = $1`, p.set.Session.ID.String())
		w.expectClose(t, pc, 4012)
	})

	t.Run("TestClose4016PendingDevice", func(t *testing.T) {
		// Registering a pending device appends device.pending (R-AZ-31), which needs a partition.
		w.partitions(t)
		p := w.newPerson("alex", identity.DeviceStatePending)
		w.identifyErr(t, w.params(p), 4016)
	})

	t.Run("TestClose4017SecondConnectionOfDevice", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		first, r := w.ready(t, w.params(p))
		second, r2 := w.ready(t, w.params(p))
		sessions.Store(second, r2.GatewaySessionId.String())
		w.expectClose(t, first, 4017)
		third := w.dial(t)
		cctx, cancel := context.WithTimeout(ctx, testkit.Patience)
		defer cancel()
		_, err := third.Resume(cctx, wstest.ResumeParams{SessionID: r.GatewaySessionId.String(), AccessToken: p.access, Proof: w.proofFor(p.key, p.access, gwHTU)})
		if ce := (*wstest.CloseError)(nil); !errors.As(err, &ce) || ce.Code != 4006 {
			t.Fatalf("resume of the replaced session: %v", err)
		}
		// Resume of the live session replaces its connection with 4017 before resumed.
		fourth := w.dial(t)
		cctx2, cancel2 := context.WithTimeout(ctx, testkit.Patience)
		defer cancel2()
		if _, err := fourth.Resume(cctx2, wstest.ResumeParams{SessionID: liveSession(t, second), AccessToken: p.access,
			Proof: w.proofFor(p.key, p.access, gwHTU)}); err != nil {
			t.Fatalf("resume of the live session: %v", err)
		}
		w.expectClose(t, second, 4017)
	})

	t.Run("TestResyncRequiredAfterReady", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		params := w.params(p)
		cursor := int64(0)
		params.Cursor = &cursor
		params.SyncEpoch = ids.New().String()
		c, r := w.ready(t, params)
		cctx, cancel := context.WithTimeout(ctx, testkit.Patience)
		defer cancel()
		op, d, err := c.Next(cctx)
		if err != nil || op != "resync_required" {
			t.Fatalf("after ready: %s %v", op, err)
		}
		var rr rest.GatewayResyncRequired
		if err := wstest.Strict(d, &rr); err != nil || rr.Reason != "epoch_changed" || rr.HeadSeq != r.HeadSeq || rr.SyncEpoch != r.SyncEpoch {
			t.Fatalf("resync_required %+v, %v", rr, err)
		}
		// The connection stays open after resync_required.
		time.Sleep(2500 * time.Millisecond)
		if err := c.Send("heartbeat", map[string]any{"last_seq": r.HeadSeq}); err != nil {
			t.Fatalf("connection closed after resync_required: %v", err)
		}
	})

	t.Run("TestNoSecretsInCloseReasonsAndLogs", func(t *testing.T) {
		w.mu.Lock()
		defer w.mu.Unlock()
		logs := w.p.logs.String()
		for _, s := range w.secrets {
			if s == "" {
				continue
			}
			for _, r := range w.reasons {
				if strings.Contains(r, s) {
					t.Errorf("a close reason carries a secret")
				}
			}
			if strings.Contains(logs, s) {
				t.Errorf("the log carries a secret of %d bytes", len(s))
			}
		}
		if len(w.reasons) == 0 || len(w.secrets) == 0 {
			t.Fatal("nothing collected")
		}
	})
}

// shiftClock is the system clock moved forward by Advance: a world's process reads it, so a test
// reaches an expiry without waiting for it and without a window a loaded host could miss.
type shiftClock struct {
	mu    gosync.Mutex
	shift time.Duration
}

// Now returns the system time plus the shift.
func (c *shiftClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().UTC().Add(c.shift)
}

// Advance moves the clock forward by d.
func (c *shiftClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shift += d
}

// TestGatewayDeadlines: each deadline of the gateway closes a connection of a started api process
// with its code, each in a world of its own where only that window is short; nothing in the setup
// of a subtest races the short window, so a slow host can only make the close come later. The
// deadlines at the exact instant, and a first heartbeat after more than one interval, are tested
// with an injected clock in gateway/app (TestIdentifyTimeout, TestHeartbeatDeadline,
// TestResumeWindow) and over a real WebSocket in gateway/adapters/ws
// (TestDeadlinesFollowTheInjectedClock).
func TestGatewayDeadlines(t *testing.T) {
	t.Parallel()

	t.Run("TestClose4003IdentifyTimeout", func(t *testing.T) {
		t.Parallel()
		w := newGatewayWorld(t, "BOTHY_SYNC__IDENTIFY_TIMEOUT=2s") // nosemgrep: bothy-go-test-gateway-window -- the deadline under test; the client only dials
		// The stopwatch starts before the dial, so it measures at least the server's window.
		start := time.Now()
		c := w.dial(t)
		w.expectClose(t, c, 4003)
		if d := time.Since(start); d < 2*time.Second {
			t.Errorf("closed after %v, before sync.identify_timeout", d)
		}
	})

	t.Run("TestClose4009HeartbeatTimeout", func(t *testing.T) {
		t.Parallel()
		w := newGatewayWorld(t, "BOTHY_SYNC__HEARTBEAT_INTERVAL=1s", "BOTHY_SYNC__HEARTBEAT_MISS_LIMIT=1") // nosemgrep: bothy-go-test-gateway-window -- the deadline under test; the client sends no heartbeat
		p := w.newPerson("alex", identity.DeviceStateActive)
		// The stopwatch starts before identify, so it measures at least the server's deadline.
		start := time.Now()
		c, _ := w.ready(t, w.params(p), wstest.WithoutHeartbeat())
		w.expectClose(t, c, 4009)
		if d := time.Since(start); d < 2*time.Second {
			t.Errorf("closed after %v, before the heartbeat deadline", d)
		}
	})

	t.Run("TestClose4006ResumeAfterWindow", func(t *testing.T) {
		t.Parallel()
		w := newGatewayWorld(t, "BOTHY_SYNC__RESUME_WINDOW=1s") // nosemgrep: bothy-go-test-gateway-window -- the window under test; the resume comes after it on purpose
		ctx := context.Background()
		p := w.newPerson("alex", identity.DeviceStateActive)
		c, r := w.ready(t, w.params(p))
		c.Abort()
		// The window starts when the server sees the connection end; the wait leaves it ten
		// seconds for that beyond the window of one second.
		time.Sleep(11 * time.Second)
		n := w.dial(t)
		cctx, cancel := context.WithTimeout(ctx, testkit.Patience)
		defer cancel()
		_, err := n.Resume(cctx, wstest.ResumeParams{SessionID: r.GatewaySessionId.String(), AccessToken: p.access, Proof: w.proofFor(p.key, p.access, gwHTU)})
		if ce := (*wstest.CloseError)(nil); !errors.As(err, &ce) || ce.Code != 4006 {
			t.Fatalf("resume after the window: %v", err)
		}
	})

	// The token outlives the setup by an hour; the clock of the process then jumps past its
	// expiry, and the next heartbeat lets the gateway close with 4011. ready comes before the
	// consumer of the agent is taken (4004 on a moved clock) and before its runner checks the
	// catch-up (4012), so the clock moves only after both have read it: after the heartbeat_ack
	// of a first heartbeat (the gateway handles the frames of a connection in order, so identify,
	// the take included, has returned) and after the consumer's catchup.complete (sent once that
	// check passed), in either order.
	t.Run("TestClose4011AgentTokenExpired", func(t *testing.T) {
		t.Parallel()
		clk := &shiftClock{}
		w := newGatewayWorldWith(t, wire.Deps{Clock: clk}, patientWindows(gwFastBeat)...)
		agent := w.newAgent()
		tok := w.agentToken(agent, nil, time.Now().Add(time.Hour))
		c, _ := w.ready(t, wstest.IdentifyParams{APIToken: tok}, wstest.WithoutHeartbeat())
		if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
			t.Fatalf("heartbeat: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
		defer cancel()
		for acked, complete := false, false; !acked || !complete; {
			op, _, err := c.Next(ctx)
			switch {
			case err != nil:
				t.Fatalf("waiting for heartbeat_ack and catchup.complete: %v", err)
			case op == "heartbeat_ack":
				acked = true
			case op == "catchup.complete":
				complete = true
			}
		}
		clk.Advance(time.Hour + time.Minute)
		// A tick of the gateway may close before this heartbeat is written; the close decides.
		_ = c.Send("heartbeat", map[string]any{"last_seq": 0})
		w.expectClose(t, c, 4011)
	})
}

// tokenTTL is the auth.access_token_ttl of TestGatewayTokenExpiry and tokenLead its
// sync.reauth_lead_time.
const (
	tokenTTL  = 10 * time.Minute
	tokenLead = time.Minute
)

// TestGatewayTokenExpiry: reauth and the expiry of a person's access token, with
// auth.access_token_ttl tokenTTL, auth.clock_leeway 0s and sync.reauth_lead_time tokenLead. The
// process and the side identity read a shiftClock: a subtest identifies, then moves the clock to
// the instant of reauth_required and past the expiry, so no setup on a loaded host can reach the
// lifetime of a token. The heartbeat deadline is beyond every move (an interval of an hour), and
// the clients beat by hand, which also makes the gateway tick at once after a move. The subtests
// share the clock and run one after the other. agent.presence_timeout lies above that interval,
// as the cross-check presence_timeout_above_heartbeat_interval requires (M2-20).
func TestGatewayTokenExpiry(t *testing.T) {
	t.Parallel()
	clk := &shiftClock{}
	w := newGatewayWorldWith(t, wire.Deps{Clock: clk}, append(patientWindows(time.Hour), "BOTHY_AGENT__PRESENCE_TIMEOUT=2h",
		"BOTHY_AUTH__ACCESS_TOKEN_TTL=10m", "BOTHY_AUTH__CLOCK_LEEWAY=0s", "BOTHY_SYNC__REAUTH_LEAD_TIME=60s")...)
	ctx := context.Background()
	next := func(t *testing.T, c *wstest.Client) (string, []byte) {
		t.Helper()
		cctx, cancel := context.WithTimeout(ctx, testkit.Patience)
		defer cancel()
		op, d, err := c.Next(cctx)
		if err != nil {
			t.Fatalf("next frame: %v", err)
		}
		return op, d
	}
	// beat sends a heartbeat and requires its heartbeat_ack: the connection is open, and the
	// gateway ticked after it.
	beat := func(t *testing.T, c *wstest.Client) {
		t.Helper()
		if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
			t.Fatalf("heartbeat: %v", err)
		}
		if op, d := next(t, c); op != "heartbeat_ack" {
			t.Fatalf("after a heartbeat: %s %s, want heartbeat_ack", op, d)
		}
	}
	// reauthDue moves the clock one second past the instant of reauth_required of a token issued
	// at or before issued, beats, and returns the reauth_required that follows. It reads both the
	// reauth_required and the heartbeat_ack of its heartbeat, in either order (a gateway tick may
	// send the reauth_required before the heartbeat is answered), so no frame of it is left for
	// the next read.
	reauthDue := func(t *testing.T, c *wstest.Client, issued time.Time) rest.GatewayReauthRequired {
		t.Helper()
		if d := issued.Add(tokenTTL - tokenLead + time.Second).Sub(clk.Now()); d > 0 {
			clk.Advance(d)
		}
		if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
			t.Fatalf("heartbeat: %v", err)
		}
		var rr rest.GatewayReauthRequired
		acked, due := false, false
		for !acked || !due {
			op, d := next(t, c)
			switch {
			case op == "heartbeat_ack" && !acked:
				acked = true
			case op == "reauth_required" && !due:
				if wstest.Strict(d, &rr) != nil || rr.DpopNonce == "" {
					t.Fatalf("after the reauth instant: %s %s, want reauth_required", op, d)
				}
				due = true
			default:
				t.Fatalf("after the reauth instant: %s %s, want reauth_required and heartbeat_ack", op, d)
			}
		}
		return rr
	}

	t.Run("TestClose4011ExpiredWithoutReauth", func(t *testing.T) {
		p := w.newPerson("alex", identity.DeviceStateActive)
		issued := clk.Now()
		c, _ := w.ready(t, w.params(p), wstest.WithoutHeartbeat())
		beat(t, c)
		rr := reauthDue(t, c, issued)
		w.remember(rr.DpopNonce)
		if rr.ExpiresAt.After(issued.Add(tokenTTL)) || rr.ExpiresAt.After(clk.Now().Add(tokenLead)) {
			t.Fatalf("reauth_required for the expiry %v, token issued at %v", rr.ExpiresAt, issued)
		}
		clk.Advance(rr.ExpiresAt.Sub(clk.Now()))
		_ = c.Send("heartbeat", map[string]any{"last_seq": 0})
		w.expectClose(t, c, 4011)
	})

	t.Run("TestReauthKeepsConnection", func(t *testing.T) {
		p := w.newPerson("sam", identity.DeviceStateActive)
		issued := clk.Now()
		c, _ := w.ready(t, w.params(p), wstest.WithoutHeartbeat())
		rr := reauthDue(t, c, issued)
		// An invalid proof answers error and keeps the connection.
		bad := w.proofFor(p.key, p.access, "https://chat.example.com/api/v1/other")(rr.DpopNonce)
		if err := c.Send("reauth", map[string]any{"access_token": p.access, "dpop": bad}); err != nil {
			t.Fatal(err)
		}
		op, d := next(t, c)
		var ef rest.GatewayError
		if op != "error" || wstest.Strict(d, &ef) != nil || ef.Op != "reauth" || ef.Code != "invalid_dpop_proof" {
			t.Fatalf("after a bad proof: %s %s", op, d)
		}
		// A refreshed token of the same session moves the expiry.
		set, err := w.id.Sessions.Refresh(ctx, p.set.RefreshToken.Reveal(), identity.VerifiedProof{JKT: p.key.Thumbprint, PublicJWK: p.key.JWK, IssuedAt: clk.Now()})
		if err != nil {
			t.Fatal(err)
		}
		fresh := set.AccessToken.Reveal()
		w.remember(fresh, set.RefreshToken.Reveal())
		refreshed := clk.Now()
		if err := c.Send("reauth", map[string]any{"access_token": fresh, "dpop": w.proofFor(p.key, fresh, gwHTU)(rr.DpopNonce)}); err != nil {
			t.Fatal(err)
		}
		beat(t, c)
		clk.Advance(rr.ExpiresAt.Add(time.Second).Sub(clk.Now()))
		beat(t, c)
		next := reauthDue(t, c, refreshed)
		// A token of another auth session of the same device closes with 4004.
		other, err := w.id.Sessions.StartSession(ctx, identity.StartSession{UserID: p.user, DeviceID: p.device, AuthMethod: identity.AuthPasskey})
		if err != nil {
			t.Fatal(err)
		}
		tok := other.AccessToken.Reveal()
		w.remember(tok, other.RefreshToken.Reveal())
		if err := c.Send("reauth", map[string]any{"access_token": tok, "dpop": w.proofFor(p.key, tok, gwHTU)(next.DpopNonce)}); err != nil {
			t.Fatal(err)
		}
		w.expectClose(t, c, 4004)
	})

	t.Run("TestNoSecretsInCloseReasonsAndLogs", func(t *testing.T) {
		w.mu.Lock()
		defer w.mu.Unlock()
		logs := w.p.logs.String()
		for _, s := range w.secrets {
			for _, r := range w.reasons {
				if s != "" && strings.Contains(r, s) {
					t.Error("a close reason carries a secret")
				}
			}
			if s != "" && strings.Contains(logs, s) {
				t.Errorf("the log carries a secret of %d bytes", len(s))
			}
		}
	})
}

// TestGatewayWorldTokenOutlivesTheRun: a person of a gateway world gets an access token for
// gwAccessTokenTTL, longer than any run of the package, so a suite that takes longer than the
// default auth.access_token_ttl on a loaded runner keeps its people signed in. The token of the side identity carries the lifetime of the
// process's configuration, which serves the same key.
func TestGatewayWorldTokenOutlivesTheRun(t *testing.T) {
	t.Parallel()
	w := newGatewayWorld(t)
	p := w.newPerson("alex", identity.DeviceStateActive)
	if got := time.Duration(p.set.ExpiresIn) * time.Second; got != gwAccessTokenTTL {
		t.Fatalf("access token for %s, want %s", got, gwAccessTokenTTL)
	}
	if v := envValue(w.env, "BOTHY_AUTH__ACCESS_TOKEN_TTL"); v != "24h" {
		t.Fatalf("auth.access_token_ttl of the process %q, want 24h", v)
	}
}
