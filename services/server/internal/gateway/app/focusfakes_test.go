// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// The levels of the focus world.
const (
	levelRead  = 1
	levelWrite = 2
)

// focusWorld is the world of the focus and typing tests: threads in channels of spaces, the level
// of each principal per channel and the read-only channels. It is the Authorizer, the
// Authenticator, catch-up and the chat port of a gateway with a real hub.
type focusWorld struct {
	mu       gosync.Mutex
	threads  map[ids.ID]chat.ThreadInfo
	levels   map[ids.ID]map[ids.ID]int
	readOnly map[ids.ID]bool
	actors   map[string]actor.Actor
	// checks counts the Authorize calls per action.
	checks map[authorize.Action]int
}

func newFocusWorld() *focusWorld {
	return &focusWorld{threads: map[ids.ID]chat.ThreadInfo{}, levels: map[ids.ID]map[ids.ID]int{}, readOnly: map[ids.ID]bool{},
		actors: map[string]actor.Actor{}, checks: map[authorize.Action]int{}}
}

// thread adds a thread in a new channel of space and returns it.
func (w *focusWorld) thread(space ids.ID) chat.ThreadInfo {
	w.mu.Lock()
	defer w.mu.Unlock()
	th := chat.ThreadInfo{ID: ids.New(), ChannelID: ids.New(), SpaceID: space, ChannelKind: "text", Kind: "thread", State: "open"}
	w.threads[th.ID] = th
	return th
}

// move moves thread to a new channel of the same space and returns the new channel.
func (w *focusWorld) move(thread ids.ID) chat.ThreadInfo {
	w.mu.Lock()
	defer w.mu.Unlock()
	th := w.threads[thread]
	th.ChannelID = ids.New()
	w.threads[thread] = th
	return th
}

// grant gives p level on channel; 0 removes it.
func (w *focusWorld) grant(p, channel ids.ID, level int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.levels[p] == nil {
		w.levels[p] = map[ids.ID]int{}
	}
	if level == 0 {
		delete(w.levels[p], channel)
		return
	}
	w.levels[p][channel] = level
}

// level returns the level of p on the channel of thread and whether the thread exists; w.mu held.
func (w *focusWorld) level(p, thread ids.ID) (chat.ThreadInfo, int, bool) {
	th, ok := w.threads[thread]
	if !ok {
		return chat.ThreadInfo{}, 0, false
	}
	return th, w.levels[p][th.ChannelID], true
}

// threadOf returns the thread a resource names.
func threadOf(r authorize.Resource) ids.ID {
	if r.Kind == authorize.ResourceThread && !r.ID.IsZero() {
		return r.ID
	}
	return r.ThreadID
}

// Authorize decides gateway.connect (always), gateway.focus and thread.read (read on the channel)
// and typing.start (people only, write, not in a read-only channel); unknown and unreadable
// threads are not_found.
func (w *focusWorld) Authorize(ctx context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	w.mu.Lock()
	w.checks[a]++
	d := w.decide(p, a, r)
	w.mu.Unlock()
	authorize.RecordCheck(ctx, authorize.Check{Action: string(a), Kind: authorize.CheckAuthorize, Allowed: d.Allowed()})
	return d, nil
}

// decide is Authorize without the check log; w.mu is held.
func (w *focusWorld) decide(p authorize.Principal, a authorize.Action, r authorize.Resource) authorize.Decision {
	allow := authorize.Decision{Outcome: authorize.OutcomeAllow}
	if a == authorize.ActionGatewayConnect {
		return allow
	}
	th, level, ok := w.level(p.PrincipalID(), threadOf(r))
	if !ok || level < levelRead {
		return authorize.Decision{Outcome: authorize.OutcomeNotFound, Reason: "no_grant"}
	}
	if a != authorize.ActionTypingStart {
		return allow
	}
	switch {
	case p.PrincipalKind() != string(actor.KindUser):
		return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "subject_kind_not_allowed"}
	case level < levelWrite:
		return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "level_insufficient"}
	case w.readOnly[th.ChannelID]:
		return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "channel_read_only"}
	}
	return allow
}

// Scope returns the channels p reads.
func (w *focusWorld) Scope(_ context.Context, p authorize.Principal, _ authorize.Action) (authorize.Scope, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var sc authorize.Scope
	for c := range w.levels[p.PrincipalID()] {
		sc.ChannelIDs = append(sc.ChannelIDs, c)
	}
	return sc, nil
}

func (*focusWorld) Observe(context.Context, authorize.LogEvent) {}

// Authenticate returns the actor of the token.
func (w *focusWorld) Authenticate(_ context.Context, c actor.Credentials) (actor.Actor, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, tok, _ := strings.Cut(c.Authorization, " ")
	if a, ok := w.actors[tok]; ok {
		return a, nil
	}
	return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "")
}

func (*focusWorld) Revalidate(context.Context, actor.Actor) error { return nil }

func (*focusWorld) Check(context.Context, sync.CheckInput) (sync.CheckResult, error) {
	return sync.CheckResult{HeadSeq: 41, SyncEpoch: epoch}, nil
}

func (*focusWorld) Page(context.Context, sync.PageInput) (sync.PageResult, error) {
	return sync.PageResult{}, nil
}

func (*focusWorld) PageEvents(context.Context, sync.PageInput) (sync.EventPage, error) {
	return sync.EventPage{}, nil
}

// Admit admits a personal event to its audience and every other event when its receive action is
// allowed on its thread.
func (w *focusWorld) Admit(_ context.Context, a actor.Actor, ev sync.Event) (bool, error) {
	if !ev.AudiencePrincipalID.IsZero() {
		return ev.AudiencePrincipalID == a.ID, nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	r := authorize.ResourceFromEvent(ev.ResourceKind, ev.ResourceID, ev.SpaceID, ev.ChannelID, ev.ThreadID, ev.AudiencePrincipalID)
	return w.decide(a, authorize.Action(ev.ReceiveAction), r).Allowed(), nil
}

// Info is the chat port: the thread or not_found.
func (w *focusWorld) Info(_ context.Context, id ids.ID) (chat.ThreadInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	th, ok := w.threads[id]
	if !ok {
		return chat.ThreadInfo{}, errs.New(errs.CodeNotFound, "")
	}
	return th, nil
}

// fakeOpLimiter refuses the principals and connections in its sets and records the keys.
type fakeOpLimiter struct {
	mu         gosync.Mutex
	denyReads  map[ids.ID]bool
	denyTyping map[ids.ID]bool
	readKeys   []ids.ID
	typingKeys []ids.ID
}

func (l *fakeOpLimiter) AllowRead(principal ids.ID) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.readKeys = append(l.readKeys, principal)
	return !l.denyReads[principal], nil
}

func (l *fakeOpLimiter) AllowTyping(conn ids.ID) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.typingKeys = append(l.typingKeys, conn)
	return !l.denyTyping[conn], nil
}

// The configuration of the focus world.
const (
	maxFocused     = 3
	typingInterval = 5 * time.Second
	typingTTL      = 8 * time.Second
)

// fenv is a gateway with a real hub over a focus world.
type fenv struct {
	t       *testing.T
	w       *focusWorld
	hub     *app.Hub
	gw      *app.Gateway
	clk     *clock.Fake
	limiter *fakeOpLimiter
	support ids.ID
}

// newFocusEnv returns the gateway with every op of M1-64 and M1-66 registered.
func newFocusEnv(t *testing.T) *fenv {
	t.Helper()
	return newFocusEnvOn(t, newFocusWorld(), nil)
}

// newFocusEnvOn returns the gateway of newFocusEnv over the world w, shared with other nodes; with
// cluster the gateway publishes through the cluster and announces its device bindings, and the
// cluster receives for its hub and gateway (one api node of M7-17).
func newFocusEnvOn(t *testing.T, w *focusWorld, cluster *app.Cluster) *fenv {
	t.Helper()
	e := &fenv{t: t, w: w, clk: clock.NewFake(t0), limiter: &fakeOpLimiter{denyReads: map[ids.ID]bool{}, denyTyping: map[ids.ID]bool{}},
		support: ids.New()}
	h, err := app.NewHub(app.HubConfig{QueueMax: 100, PageSize: 4, CursorEvery: 1000, RetryBase: time.Millisecond, RetryCap: time.Millisecond,
		PayloadMax: 65536}, app.HubDeps{Authenticator: e.w, Authorizer: e.w, CatchUp: e.w, Clock: e.clk})
	if err != nil {
		t.Fatal(err)
	}
	e.hub = h
	var publisher eventlog.EphemeralPublisher = h
	var signals app.ClusterSignals
	if cluster != nil {
		publisher, signals = cluster.Publisher(h), cluster
	}
	gw, err := app.New(app.Config{
		HTU: "https://chat.example.com/api/v1/gateway", IdentifyTimeout: 10 * time.Second, HeartbeatInterval: 30 * time.Second,
		HeartbeatMissLimit: 2, ResumeWindow: 5 * time.Minute, ReauthLeadTime: time.Minute, ProtocolVersions: []int{1},
		MaxFocusedThreads: maxFocused, TypingMinInterval: typingInterval, TypingTTL: typingTTL,
	}, app.Deps{
		Authenticator: e.w, Authorizer: e.w, Nonces: staticNonce("nonce-1"), Limiter: fakeLimiter{}, Snapshot: fakeSnapshot{head: 41, log: &callLog{}},
		CatchUp: e.w, Epoch: staticEpoch{}, Delivery: h, Decoder: jsonDecoder{}, Alarm: &fakeAlarm{}, Clock: e.clk,
		Threads: e.w, OpLimiter: e.limiter, Publisher: publisher, Cluster: signals,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cluster != nil {
		if err := cluster.Bind(h, gw, nil); err != nil {
			t.Fatal(err)
		}
	}
	for op, action := range map[string]authorize.Action{
		"heartbeat": authorize.ActionGatewayConnect, "identify": authorize.ActionGatewayConnect, "reauth": authorize.ActionGatewayConnect,
		"resume": authorize.ActionGatewayConnect, "focus": authorize.ActionGatewayFocus, "typing.start": authorize.ActionTypingStart,
		"typing.stop": authorize.ActionTypingStart,
	} {
		if err := gw.Register(op, action); err != nil {
			t.Fatal(err)
		}
	}
	e.gw = gw
	return e
}

// client is one identified connection of the focus world.
type client struct {
	a  actor.Actor
	c  *app.Connection
	fc *fakeConn
}

// newPerson returns a person with a token in the world.
func (e *fenv) newPerson() actor.Actor {
	a := actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New(), CredentialExpiresAt: t0.Add(time.Hour)}
	e.w.mu.Lock()
	e.w.actors[a.ID.String()] = a
	e.w.mu.Unlock()
	return a
}

// newAgent returns an agent with a token in the world.
func (e *fenv) newAgent() actor.Actor {
	a := actor.Actor{ID: ids.New(), Kind: actor.KindAgent, TokenID: ids.New(), CredentialExpiresAt: t0.Add(time.Hour)}
	e.w.mu.Lock()
	e.w.actors[a.ID.String()] = a
	e.w.mu.Unlock()
	return a
}

// device returns a second device of the person a, with the same token.
func (e *fenv) device(a actor.Actor) actor.Actor {
	b := a
	b.DeviceID, b.SessionID = ids.New(), ids.New()
	tok := a.ID.String() + "-" + b.DeviceID.String()
	e.w.mu.Lock()
	e.w.actors[tok] = b
	e.w.mu.Unlock()
	return b
}

// token returns the token of a in the world.
func (e *fenv) token(a actor.Actor) string {
	e.w.mu.Lock()
	defer e.w.mu.Unlock()
	for tok, x := range e.w.actors {
		if x.ID == a.ID && x.DeviceID == a.DeviceID && x.TokenID == a.TokenID {
			return tok
		}
	}
	e.t.Fatalf("no token for %v", a.ID)
	return ""
}

// join opens a connection of a and identifies it; it is live afterwards.
func (e *fenv) join(a actor.Actor) *client {
	e.t.Helper()
	fc := &fakeConn{}
	c, err := e.gw.Open(context.Background(), fc)
	if err != nil {
		e.t.Fatal(err)
	}
	body := map[string]any{"protocol_version": 1}
	if a.Kind == actor.KindUser {
		body["access_token"], body["dpop"], body["device_id"] = e.token(a), "proof", a.DeviceID
	} else {
		body["api_token"] = e.token(a)
	}
	e.send(c, "identify", body)
	wantOpen(e.t, fc)
	if len(sent[app.Ready](fc)) != 1 {
		e.t.Fatal("no ready")
	}
	if got := e.hub.PhaseOf(c); got != domain.PhaseLive {
		e.t.Fatalf("phase %s after identify", got)
	}
	return &client{a: a, c: c, fc: fc}
}

// send dispatches op with the JSON of d on c.
func (e *fenv) send(c *app.Connection, op string, d any) {
	e.t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		e.t.Fatal(err)
	}
	e.gw.Handle(context.Background(), c, op, b)
}

// focus sends focus with threads.
func (e *fenv) focus(cl *client, idle bool, threads ...ids.ID) {
	e.t.Helper()
	if threads == nil {
		threads = []ids.ID{}
	}
	e.send(cl.c, "focus", map[string]any{"thread_ids": threads, "idle": idle})
}

// typing sends typing.start or typing.stop on thread.
func (e *fenv) typing(cl *client, op string, thread ids.ID) {
	e.t.Helper()
	e.send(cl.c, op, map[string]any{"thread_id": thread})
}

// wireEvent is a dispatch frame as a client reads it.
type wireEvent struct {
	Op string `json:"op"`
	D  struct {
		ID                string          `json:"id"`
		Seq               *int64          `json:"seq"`
		PermissionVersion *int64          `json:"permission_version"`
		Type              string          `json:"type"`
		SchemaVersion     int             `json:"schema_version"`
		OccurredAt        string          `json:"occurred_at"`
		Actor             *wireActor      `json:"actor"`
		Scope             map[string]any  `json:"scope"`
		Resource          map[string]any  `json:"resource"`
		Payload           json.RawMessage `json:"payload"`
	} `json:"d"`
}

type wireActor struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// events returns the dispatch frames the hub queued on cl.
func (cl *client) events(t *testing.T) []wireEvent {
	t.Helper()
	var out []wireEvent
	for _, e := range sent[domain.Entry](cl.fc) {
		var ev wireEvent
		if err := json.Unmarshal(e.Frame, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Op == "dispatch" {
			out = append(out, ev)
		}
	}
	return out
}

// errorFrames returns the error frames sent on cl.
func (cl *client) errorFrames() []app.ErrorFrame { return sent[app.ErrorFrame](cl.fc) }
