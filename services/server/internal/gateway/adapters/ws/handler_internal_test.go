// SPDX-License-Identifier: Apache-2.0

package ws

import (
	"context"
	"net/http/httptest"
	"slices"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/api/routes"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/gateway/adapters/wstest"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/sync"
	synchttp "github.com/OWNER/bothy/services/server/internal/sync/adapters/httpapi"
)

// publicURL is the instance origin of the tests.
const publicURL = "https://chat.example.com"

// agentToken is the one token the fake authenticator knows.
const agentToken = "agent-token"

// order records calls of the fakes in order.
type order struct {
	mu    gosync.Mutex
	calls []string
}

func (o *order) add(s string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.calls = append(o.calls, s)
}

func (o *order) all() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.calls)
}

// agentAuth authenticates agentToken as one agent and records the transport of each call.
type agentAuth struct {
	agent      actor.Actor
	transports chan reqmeta.Transport
}

func (a agentAuth) Authenticate(ctx context.Context, c actor.Credentials) (actor.Actor, error) {
	if t, ok := reqmeta.TransportOf(ctx); ok && a.transports != nil {
		a.transports <- t
	}
	if c.Authorization == "Bearer "+agentToken {
		return a.agent, nil
	}
	return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "")
}

func (agentAuth) Revalidate(context.Context, actor.Actor) error { return nil }

// allowAll allows every action and records the check.
type allowAll struct{}

func (allowAll) Authorize(ctx context.Context, _ authorize.Principal, a authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	authorize.RecordCheck(ctx, authorize.Check{Action: string(a), Kind: authorize.CheckAuthorize, Allowed: true})
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (allowAll) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// Observe implements authorize.Authorizer; the fake keeps no cache.
func (allowAll) Observe(context.Context, authorize.LogEvent) {}

type nonce struct{}

func (nonce) CurrentNonce() string { return "n-1" }

type unlimited struct{}

func (unlimited) AllowConnect(actor.Actor) (bool, int, error) { return true, 0, nil }

// snapshot reads the head (recorded in o) and returns an empty view.
type snapshot struct{ o *order }

func (s snapshot) Build(ctx context.Context) (sync.SnapshotView, error) {
	s.o.add("head")
	a, _ := actor.FromContext(ctx)
	return sync.SnapshotView{HeadSeq: 9, SyncEpoch: testEpoch, Principal: sync.SnapshotPrincipal{ID: a.ID, Kind: string(a.Kind), Handle: "robin", DisplayName: "Robin"}}, nil
}

type catchUp struct{}

func (catchUp) Check(context.Context, sync.CheckInput) (sync.CheckResult, error) {
	return sync.CheckResult{HeadSeq: 9, SyncEpoch: testEpoch}, nil
}

func (catchUp) Page(context.Context, sync.PageInput) (sync.PageResult, error) {
	return sync.PageResult{}, nil
}

func (catchUp) PageEvents(context.Context, sync.PageInput) (sync.EventPage, error) {
	return sync.EventPage{}, nil
}

func (catchUp) Admit(context.Context, actor.Actor, sync.Event) (bool, error) { return true, nil }

var testEpoch = ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f8001")

type epochSource struct{}

func (epochSource) SyncEpoch(context.Context) (ids.ID, error) { return testEpoch, nil }

// delivery records Register and Attach in o.
type delivery struct{ o *order }

func (d delivery) Register(context.Context, *app.Connection, *app.Session) { d.o.add("register") }
func (d delivery) Attach(context.Context, *app.Connection, int64, int64) error {
	d.o.add("attach")
	return nil
}
func (d delivery) Detach(*app.Connection) { d.o.add("detach") }

func (d delivery) Focus(*app.Connection, domain.Focus) { d.o.add("focus") }

// noThreads knows no thread.
type noThreads struct{}

func (noThreads) Info(context.Context, ids.ID) (chat.ThreadInfo, error) {
	return chat.ThreadInfo{}, errs.New(errs.CodeNotFound, "")
}

// noLimits never limits.
type noLimits struct{}

func (noLimits) AllowRead(ids.ID) (bool, error)   { return true, nil }
func (noLimits) AllowTyping(ids.ID) (bool, error) { return true, nil }

// noPublisher drops every ephemeral event.
type noPublisher struct{}

func (noPublisher) PublishEphemeral(context.Context, eventlog.EphemeralEvent) error { return nil }

type noAlarm struct{}

func (noAlarm) NotChecked(context.Context, string, authorize.Action) {}

// testServer is a real server on 127.0.0.1:0 with the handler over fakes.
type testServer struct {
	srv        *httptest.Server
	o          *order
	h          *Handler
	agent      actor.Actor
	transports chan reqmeta.Transport
}

// newTestServer starts the server; mod changes the handler before it serves.
func newTestServer(t *testing.T, queueMax int, mod func(*Handler)) *testServer {
	t.Helper()
	return newTestServerWith(t, queueMax, mod, nil)
}

// newTestServerWith is newTestServer with the metrics gm of the gateway use cases.
func newTestServerWith(t *testing.T, queueMax int, mod func(*Handler), gm app.Metrics) *testServer {
	t.Helper()
	return newTestServerOpts(t, queueMax, mod, serverOpts{gm: gm})
}

// testBeat is the heartbeat interval of the test server. Its heartbeat and identify deadlines are
// those of testkit.PatientGateway, so no test that does not check a deadline loses its connection
// on a loaded host; the deadline tests set their windows and inject a clock (serverOpts).
const testBeat = 2 * time.Second

// serverOpts are the optional parts of a test server: the metrics gm of the gateway use cases,
// the clock of the gateway use cases and the clock of the handler (nil: the system clock), and
// windows, which changes the deadlines of the gateway configuration.
type serverOpts struct {
	gm           app.Metrics
	clock        clock.Clock
	handlerClock clock.Clock
	windows      func(*app.Config)
}

// newTestServerOpts is newTestServer with the options so.
func newTestServerOpts(t *testing.T, queueMax int, mod func(*Handler), so serverOpts) *testServer {
	t.Helper()
	gm := so.gm
	o := &order{}
	transports := make(chan reqmeta.Transport, 16)
	agent := actor.Actor{ID: ids.New(), Kind: actor.KindAgent, TokenID: ids.New(), CredentialExpiresAt: time.Now().Add(time.Hour)}
	var names []string
	schemas := map[string]string{}
	for _, op := range routes.GatewayOps() {
		if !slices.Contains(names, op.Schema) {
			names = append(names, op.Schema)
		}
		schemas[op.Op] = op.Schema
	}
	check, err := httpserver.NewSchemaValidator(rest.Spec, names)
	if err != nil {
		t.Fatal(err)
	}
	cfg := app.Config{HTU: publicURL + "/api/v1/gateway", IdentifyTimeout: testkit.Patience, HeartbeatInterval: testBeat,
		HeartbeatMissLimit: testkit.HeartbeatMissLimit(testBeat), ResumeWindow: time.Minute, ReauthLeadTime: time.Second, ProtocolVersions: []int{1},
		MaxFocusedThreads: 20, TypingMinInterval: 5 * time.Second, TypingTTL: 8 * time.Second}
	if so.windows != nil {
		so.windows(&cfg)
	}
	gw, err := app.New(cfg,
		app.Deps{Authenticator: agentAuth{agent: agent, transports: transports}, Authorizer: allowAll{}, Nonces: nonce{}, Limiter: unlimited{}, Snapshot: snapshot{o},
			CatchUp: catchUp{}, Epoch: epochSource{}, Delivery: delivery{o}, Decoder: NewDecoder(schemas, check), Alarm: noAlarm{},
			Threads: noThreads{}, OpLimiter: noLimits{}, Publisher: noPublisher{}, Metrics: gm, Clock: so.clock})
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range routes.GatewayOps() {
		if op.Op == "ack" || op.Op == "command.autocomplete_result" {
			// ack needs the consumer engine of M2-07 and command.autocomplete_result the command
			// execution of runs (M2-17), which these transport tests do not build.
			continue
		}
		if err := gw.Register(op.Op, authorize.ActionGatewayConnect); err != nil {
			t.Fatal(err)
		}
	}
	h, err := NewHandler(Config{PublicURL: publicURL, MaxFrameBytes: 1 << 16, MaxJSONDepth: 32, OutboundQueueMax: queueMax, WriteTimeout: 5 * time.Second},
		Deps{Gateway: gw, Snapshot: synchttp.SnapshotREST, Clock: so.handlerClock})
	if err != nil {
		t.Fatal(err)
	}
	if mod != nil {
		mod(h)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &testServer{srv: srv, o: o, h: h, agent: agent, transports: transports}
}

// dial connects a client with the instance origin.
func (s *testServer) dial(t *testing.T, opts ...wstest.Option) *wstest.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	c, err := wstest.Dial(t, ctx, s.srv.URL, append([]wstest.Option{wstest.WithOrigin(publicURL), wstest.WithMaxFrameBytes(1 << 16)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestRegisterBeforeHead (04 section 6.3 step 4): with a fake Delivery, Register is called before
// the snapshot reads the head; delivery is attached after ready.
func TestRegisterBeforeHead(t *testing.T) {
	t.Parallel()
	s := newTestServer(t, 16, nil)
	c := s.dial(t)
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	if _, err := c.Identify(ctx, wstest.IdentifyParams{APIToken: agentToken}); err != nil {
		t.Fatal(err)
	}
	// Attach runs after ready is sent, so it may still be under way when Identify returns.
	testkit.Until(t, "attach after ready", func() bool { return len(s.o.all()) >= 3 })
	if got := s.o.all(); got[0] != "register" || got[1] != "head" || got[2] != "attach" {
		t.Errorf("calls %v, want register, head, attach", got)
	}
}

// TestOutboundQueueOverflowCloses4014: with a writer that blocks, filling the outbound queue of
// sync.outbound_queue_max frames closes the connection with 4014.
func TestOutboundQueueOverflowCloses4014(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	var block bool
	var mu gosync.Mutex
	s := newTestServer(t, 2, func(h *Handler) {
		h.beforeWrite = func() {
			mu.Lock()
			b := block
			mu.Unlock()
			if b {
				<-release
			}
		}
	})
	t.Cleanup(func() { close(release) })
	c := s.dial(t, wstest.WithoutHeartbeat())
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	if _, err := c.Identify(ctx, wstest.IdentifyParams{APIToken: agentToken}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	block = true
	mu.Unlock()
	// Every heartbeat queues one heartbeat_ack; the blocked writer takes the first, the queue holds
	// two, the fourth overflows.
	for i := 0; i < 4; i++ {
		if err := c.Send("heartbeat", map[string]any{"last_seq": 0}); err != nil {
			break
		}
	}
	code, reason, err := c.ExpectClose(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if code != int(domain.CloseSlowConsumer) || reason != "slow_consumer" {
		t.Errorf("close %d %q, want 4014 slow_consumer", code, reason)
	}
}

// TestTestClientJittersFirstHeartbeat: the first heartbeat of the wstest client waits a share of
// heartbeat_interval_ms from zero up to the interval (04 section 6.3 step 5), taken from its
// random source.
func TestTestClientJittersFirstHeartbeat(t *testing.T) {
	t.Parallel()
	interval := testBeat
	for _, r := range []float64{0, 0.25, 0.999999} {
		got := wstest.FirstHeartbeatDelay(interval, r)
		if got < 0 || got >= interval {
			t.Errorf("r %v: delay %v outside [0, %v)", r, got, interval)
		}
	}
	s := newTestServer(t, 16, nil)
	delays := make(chan time.Duration, 1)
	c := s.dial(t, wstest.WithRandom(func() float64 { return 0.5 }), wstest.OnFirstHeartbeatDelay(func(d time.Duration) { delays <- d }))
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	if _, err := c.Identify(ctx, wstest.IdentifyParams{APIToken: agentToken}); err != nil {
		t.Fatal(err)
	}
	if d := <-delays; d != interval/2 {
		t.Errorf("first delay %v, want half of heartbeat_interval_ms %v", d, interval/2)
	}
}

// TestHTU: the htu is the HTTPS form of the origin of server.public_url, also for a server that
// speaks HTTP behind a TLS terminator.
func TestHTU(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"https://chat.example.com":      "https://chat.example.com/api/v1/gateway",
		"http://chat.example.com":       "https://chat.example.com/api/v1/gateway",
		"https://chat.example.com:8443": "https://chat.example.com:8443/api/v1/gateway",
	} {
		got, err := HTU(in)
		if err != nil || got != want {
			t.Errorf("HTU(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// TestTransportReachesAuthenticator (red team M1-64): identify authenticates with the transport
// of the upgrade request in the context, the TCP peer, its header fields and the action
// gateway.connect, as the authenticate stage of REST does; the session binding of trusted header
// logins (M1-43) reads it and would revoke the session without it.
func TestTransportReachesAuthenticator(t *testing.T) {
	t.Parallel()
	s := newTestServer(t, 16, nil)
	c := s.dial(t, wstest.WithHeader("X-Login", "robin"))
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Patience)
	defer cancel()
	if _, err := c.Identify(ctx, wstest.IdentifyParams{APIToken: agentToken}); err != nil {
		t.Fatal(err)
	}
	select {
	case tr := <-s.transports:
		if !tr.Peer.IsLoopback() || tr.Action != "gateway.connect" || len(tr.Values("X-Login")) != 1 || tr.Values("X-Login")[0] != "robin" {
			t.Errorf("transport %+v", tr)
		}
	default:
		t.Fatal("the authenticator got no transport")
	}
}
