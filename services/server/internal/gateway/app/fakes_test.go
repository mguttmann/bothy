// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// t0 is the start of every fake clock.
var t0 = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

// fakeConn records the frames and the close of one connection.
type fakeConn struct {
	mu     gosync.Mutex
	frames []any
	code   domain.CloseCode
	reason string
	closes int
}

func (c *fakeConn) Send(f any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = append(c.frames, f)
	return nil
}

func (c *fakeConn) Close(code domain.CloseCode, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	if c.closes == 1 {
		c.code, c.reason = code, reason
	}
}

// Offer records the frame of an entry of the hub; the fake queue never fills.
func (c *fakeConn) Offer(e domain.Entry) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = append(c.frames, e)
	return true
}

// WaitSpace returns at once.
func (c *fakeConn) WaitSpace(context.Context) error { return nil }

// Purge drops nothing.
func (c *fakeConn) Purge(domain.Targets) {}

func (c *fakeConn) closed() (domain.CloseCode, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.code, c.reason
}

// sent returns the frames of type T.
func sent[T any](c *fakeConn) []T {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []T
	for _, f := range c.frames {
		if v, ok := f.(T); ok {
			out = append(out, v)
		}
	}
	return out
}

// fakeAuth authenticates the tokens of its map; an unknown token is 401 unauthenticated.
type fakeAuth struct {
	mu      gosync.Mutex
	actors  map[string]actor.Actor
	errs    map[string]error
	creds   []actor.Credentials
	revoked map[ids.ID]bool
	// revalidate, when set, answers Revalidate instead of revoked.
	revalidate func() error
	// authenticating, when set, runs inside Authenticate before it answers.
	authenticating func()
}

func (a *fakeAuth) Authenticate(_ context.Context, c actor.Credentials) (actor.Actor, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.creds = append(a.creds, c)
	if a.authenticating != nil {
		a.authenticating()
	}
	_, tok, _ := cutScheme(c.Authorization)
	if err, ok := a.errs[tok]; ok {
		return actor.Actor{}, err
	}
	if p, ok := a.actors[tok]; ok {
		return p, nil
	}
	return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "")
}

// Revalidate refuses an actor whose principal is in revoked with 401 session_revoked.
func (a *fakeAuth) Revalidate(_ context.Context, p actor.Actor) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.revalidate != nil {
		return a.revalidate()
	}
	if a.revoked[p.ID] {
		return errs.New(errs.CodeSessionRevoked, "")
	}
	return nil
}

// cutScheme splits "<scheme> <token>".
func cutScheme(v string) (string, string, bool) {
	for i := range v {
		if v[i] == ' ' {
			return v[:i], v[i+1:], true
		}
	}
	return "", v, false
}

// fakeAuthz decides gateway.connect per principal and records every check in the check log.
type fakeAuthz struct {
	mu        gosync.Mutex
	decisions map[ids.ID]authorize.Decision
	err       error
	calls     int
}

func (z *fakeAuthz) Authorize(ctx context.Context, p authorize.Principal, a authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.calls++
	if z.err != nil {
		return authorize.Decision{}, z.err
	}
	d, ok := z.decisions[p.PrincipalID()]
	if !ok {
		d = authorize.Decision{Outcome: authorize.OutcomeAllow}
	}
	authorize.RecordCheck(ctx, authorize.Check{Action: string(a), Kind: authorize.CheckAuthorize, Allowed: d.Allowed()})
	return d, nil
}

func (z *fakeAuthz) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

// Observe implements authorize.Authorizer; the fake keeps no cache.
func (*fakeAuthz) Observe(context.Context, authorize.LogEvent) {}

func (z *fakeAuthz) set(id ids.ID, d authorize.Decision) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.decisions[id] = d
}

// fakeLimiter refuses the actors in deny.
type fakeLimiter struct{ deny map[ids.ID]bool }

func (l fakeLimiter) AllowConnect(a actor.Actor) (bool, int, error) {
	if l.deny[a.ID] {
		return false, 7, nil
	}
	return true, 0, nil
}

// fakeSnapshot returns a view at head and records the order of calls in log.
type fakeSnapshot struct {
	head int64
	err  error
	log  *callLog
}

func (s fakeSnapshot) Build(ctx context.Context) (sync.SnapshotView, error) {
	s.log.add("snapshot")
	if s.err != nil {
		return sync.SnapshotView{}, s.err
	}
	a, _ := actor.FromContext(ctx)
	return sync.SnapshotView{HeadSeq: s.head, SyncEpoch: epoch, PermissionVersion: 3,
		Principal: sync.SnapshotPrincipal{ID: a.ID, Kind: string(a.Kind), Handle: "alex", DisplayName: "Alex"}}, nil
}

// fakeCatchUp answers Check with reason for every cursor.
type fakeCatchUp struct {
	head   int64
	reason sync.ResyncReason
}

func (c fakeCatchUp) Check(_ context.Context, in sync.CheckInput) (sync.CheckResult, error) {
	r := c.reason
	if r == "" && in.Epoch != nil && *in.Epoch != epoch {
		r = sync.ResyncEpochChanged
	}
	return sync.CheckResult{HeadSeq: c.head, SyncEpoch: epoch, Reason: r}, nil
}

func (fakeCatchUp) Page(context.Context, sync.PageInput) (sync.PageResult, error) {
	return sync.PageResult{}, nil
}

func (fakeCatchUp) PageEvents(context.Context, sync.PageInput) (sync.EventPage, error) {
	return sync.EventPage{}, nil
}

func (fakeCatchUp) Admit(context.Context, actor.Actor, sync.Event) (bool, error) { return true, nil }

// epoch is the sync epoch of the fakes.
var epoch = ids.MustParse("0192f4a0-7b1c-7577-8bf0-c4f4ee4f8000")

// callLog records calls in order.
type callLog struct {
	mu    gosync.Mutex
	calls []string
}

func (l *callLog) add(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, s)
}

func (l *callLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.calls)
}

// fakeDelivery records Register, Attach and Detach.
type fakeDelivery struct{ log *callLog }

func (d fakeDelivery) Register(_ context.Context, _ *app.Connection, s *app.Session) {
	d.log.add("register " + s.ID.String())
}
func (d fakeDelivery) Attach(_ context.Context, _ *app.Connection, from, _ int64) error {
	d.log.add(fmt.Sprintf("attach %d", from))
	return nil
}
func (d fakeDelivery) Detach(*app.Connection) { d.log.add("detach") }

// fakeAlarm counts unproven ops.
type fakeAlarm struct {
	mu  gosync.Mutex
	ops []string
}

func (a *fakeAlarm) NotChecked(_ context.Context, op string, _ authorize.Action) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ops = append(a.ops, op)
}

// jsonDecoder decodes the body of the four enabled ops into the frames of app with
// encoding/json; the adapter's codec does the schema check in production.
type jsonDecoder struct{}

func (jsonDecoder) Decode(op string, d json.RawMessage) (any, error) {
	var v any
	switch op {
	case "identify":
		var f app.Identify
		err := json.Unmarshal(d, &f)
		v = f
		if err != nil {
			return nil, err
		}
	case "resume":
		var f app.Resume
		if err := json.Unmarshal(d, &f); err != nil {
			return nil, err
		}
		v = f
	case "heartbeat":
		var f app.Heartbeat
		if err := json.Unmarshal(d, &f); err != nil {
			return nil, err
		}
		v = f
	case "reauth":
		var f app.Reauth
		if err := json.Unmarshal(d, &f); err != nil {
			return nil, err
		}
		v = f
	default:
		return d, nil
	}
	return v, nil
}

// env is a gateway over fakes with a fake clock.
type env struct {
	t        *testing.T
	gw       *app.Gateway
	clk      *clock.Fake
	auth     *fakeAuth
	authz    *fakeAuthz
	limiter  fakeLimiter
	log      *callLog
	alarm    *fakeAlarm
	cfg      app.Config
	alex     actor.Actor
	agent    actor.Actor
	snapshot fakeSnapshot
}

// The tokens of the env.
const (
	alexToken  = "alex-token"
	agentToken = "agent-token"
)

// newEnv returns a gateway with the four ops registered; mod changes the deps before New.
func newEnv(t *testing.T, mod func(*app.Config, *app.Deps)) *env {
	t.Helper()
	e := &env{t: t, clk: clock.NewFake(t0), log: &callLog{}, alarm: &fakeAlarm{}}
	e.alex = actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New(),
		CredentialExpiresAt: t0.Add(10 * time.Minute)}
	e.agent = actor.Actor{ID: ids.New(), Kind: actor.KindAgent, TokenID: ids.New(), CredentialExpiresAt: t0.Add(time.Hour)}
	e.auth = &fakeAuth{actors: map[string]actor.Actor{alexToken: e.alex, agentToken: e.agent}, errs: map[string]error{}}
	e.authz = &fakeAuthz{decisions: map[ids.ID]authorize.Decision{}}
	e.limiter = fakeLimiter{deny: map[ids.ID]bool{}}
	e.snapshot = fakeSnapshot{head: 41, log: e.log}
	e.cfg = app.Config{
		HTU: "https://chat.example.com/api/v1/gateway", IdentifyTimeout: 10 * time.Second, HeartbeatInterval: 30 * time.Second,
		HeartbeatMissLimit: 2, ResumeWindow: 5 * time.Minute, ReauthLeadTime: time.Minute, ProtocolVersions: []int{1},
	}
	d := app.Deps{
		Authenticator: e.auth, Authorizer: e.authz, Nonces: staticNonce("nonce-1"), Limiter: e.limiter,
		Snapshot: e.snapshot, CatchUp: fakeCatchUp{head: 41}, Epoch: staticEpoch{}, Delivery: fakeDelivery{log: e.log},
		Decoder: jsonDecoder{}, Alarm: e.alarm, Clock: e.clk,
	}
	if mod != nil {
		mod(&e.cfg, &d)
	}
	gw, err := app.New(e.cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"heartbeat", "identify", "reauth", "resume"} {
		if err := gw.Register(op, authorize.ActionGatewayConnect); err != nil {
			t.Fatal(err)
		}
	}
	e.gw = gw
	return e
}

type staticNonce string

func (n staticNonce) CurrentNonce() string { return string(n) }

type staticEpoch struct{}

func (staticEpoch) SyncEpoch(context.Context) (ids.ID, error) { return epoch, nil }

// open opens a connection and returns it with its fake.
func (e *env) open() (*app.Connection, *fakeConn) {
	e.t.Helper()
	fc := &fakeConn{}
	c, err := e.gw.Open(context.Background(), fc)
	if err != nil {
		e.t.Fatal(err)
	}
	return c, fc
}

// send dispatches op with the JSON of d on c.
func (e *env) send(c *app.Connection, op string, d any) {
	e.t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		e.t.Fatal(err)
	}
	e.gw.Handle(context.Background(), c, op, b)
}

// identifyAlex identifies Alex with his device on c.
func (e *env) identifyAlex(c *app.Connection) {
	e.t.Helper()
	e.send(c, "identify", map[string]any{"protocol_version": 1, "access_token": alexToken, "dpop": "proof", "device_id": e.alex.DeviceID})
}

// wantOpen fails when fc was closed.
func wantOpen(t *testing.T, fc *fakeConn) {
	t.Helper()
	if code, reason := fc.closed(); code != 0 {
		t.Fatalf("closed with %d %q", code, reason)
	}
}

// wantClosed fails unless fc was closed with code.
func wantClosed(t *testing.T, fc *fakeConn, code domain.CloseCode) {
	t.Helper()
	if got, reason := fc.closed(); got != code {
		t.Fatalf("closed with %d %q, want %d", got, reason, code)
	}
}
