// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// qconn is a connection with the bounded queue of the domain and no writer: the test drains it
// (a released writer) or leaves it full (a writer blocked on the network).
type qconn struct {
	mu     gosync.Mutex
	q      *domain.Queue
	out    []domain.Entry
	code   domain.CloseCode
	closes int
	space  chan struct{}
	// waiting is true while a WaitSpace call waits for the writer; drain clears it.
	waiting bool
	// stamp, when set, tags every first offer of a dispatch seq in offeredAt (the model test
	// records the world step of the queueing).
	stamp     func() int64
	offeredAt map[int64]int64
}

func newQConn(maxFrames int) *qconn {
	return &qconn{q: domain.NewQueue(maxFrames), space: make(chan struct{})}
}

func (c *qconn) Send(any) error { return nil }

func (c *qconn) Close(code domain.CloseCode, _ string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closes++
	if c.closes == 1 {
		c.code = code
		c.q.Clear()
	}
}

func (c *qconn) Offer(e domain.Entry) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closes > 0 {
		return false
	}
	ok := c.q.Push(e)
	if ok && c.stamp != nil && e.Class == domain.ClassDispatch {
		if c.offeredAt == nil {
			c.offeredAt = map[int64]int64{}
		}
		if _, seen := c.offeredAt[e.Seq]; !seen {
			c.offeredAt[e.Seq] = c.stamp()
		}
	}
	return ok
}

func (c *qconn) WaitSpace(ctx context.Context) error {
	for {
		c.mu.Lock()
		roomy, closed, ch := c.q.Roomy(), c.closes > 0, c.space
		c.mu.Unlock()
		switch {
		case closed:
			return errors.New("closed")
		case roomy:
			return nil
		}
		c.mu.Lock()
		if c.space == ch {
			c.waiting = true
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

func (c *qconn) Purge(t domain.Targets) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.q.Purge(t)
}

// drain writes every queued frame (the writer runs once) and returns all frames written so far.
func (c *qconn) drain() []domain.Entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		e, ok := c.q.Pop()
		if !ok {
			break
		}
		c.out = append(c.out, e)
	}
	close(c.space)
	c.space, c.waiting = make(chan struct{}), false
	return append([]domain.Entry(nil), c.out...)
}

// dispatches drains c and returns how many dispatch frames were written so far.
func (c *qconn) dispatches() int {
	n := 0
	for _, e := range c.drain() {
		if e.Class == domain.ClassDispatch {
			n++
		}
	}
	return n
}

// len returns the number of queued frames.
func (c *qconn) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.q.Len()
}

func (c *qconn) closedWith() domain.CloseCode {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.code
}

// frame is a written frame as the client reads it.
type frame struct {
	op  string
	seq int64
}

// framesOf decodes written entries: the op and the seq of dispatch and cursor, the head of
// catchup.complete.
func framesOf(t testing.TB, es []domain.Entry) []frame {
	t.Helper()
	out := make([]frame, 0, len(es))
	for _, e := range es {
		var f struct {
			Op string `json:"op"`
			D  struct {
				Seq     int64 `json:"seq"`
				HeadSeq int64 `json:"head_seq"`
			} `json:"d"`
		}
		if err := json.Unmarshal(e.Frame, &f); err != nil {
			t.Fatalf("frame %s: %v", e.Frame, err)
		}
		out = append(out, frame{op: f.Op, seq: f.D.Seq + f.D.HeadSeq})
	}
	return out
}

// dispatched returns the seqs of the dispatch frames.
func dispatched(fs []frame) []int64 {
	var out []int64
	for _, f := range fs {
		if f.op == "dispatch" {
			out = append(out, f.seq)
		}
	}
	return out
}

// hubWorld is the state the fakes of authz and sync read: which channels each principal sees,
// the space of each channel, the space grants, the instance-wide principals and the revoked ones.
type hubWorld struct {
	mu          gosync.Mutex
	spaceOf     map[ids.ID]ids.ID
	channels    map[ids.ID]map[ids.ID]bool
	spaceGrants map[ids.ID]map[ids.ID]bool
	// operator lists, per principal, the spaces whose default operator it is: it reads every
	// channel of the space, also later ones, without the space itself in its visible sets.
	operator     map[ids.ID]map[ids.ID]bool
	instanceWide map[ids.ID]bool
	revoked      map[ids.ID]bool
	// connect is the decision of gateway.connect per principal; absent allows.
	connect map[ids.ID]authorize.Decision
	log     []sync.Event
	// onPage runs before each catch-up page, outside the lock.
	onPage func(after int64)
	// gate runs before each catch-up page after onPage, outside the lock, with the page's context.
	gate  func(ctx context.Context)
	pages int
	fail  int
	// admitFail makes Admit answer unavailable for these seqs (a database that is briefly gone).
	admitFail map[int64]bool
	// pageModes records, per catch-up page, whether its context was in delivery mode and
	// the request memo it carried.
	pageModes []pageMode
}

// pageMode is the authorization mode of one catch-up page.
type pageMode struct {
	delivery bool
	memo     *authorize.RequestMemo
}

func newHubWorld() *hubWorld {
	return &hubWorld{spaceOf: map[ids.ID]ids.ID{}, channels: map[ids.ID]map[ids.ID]bool{}, spaceGrants: map[ids.ID]map[ids.ID]bool{},
		operator: map[ids.ID]map[ids.ID]bool{}, instanceWide: map[ids.ID]bool{}, revoked: map[ids.ID]bool{}}
}

// channel adds a channel in space.
func (w *hubWorld) channel(space ids.ID) ids.ID {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := ids.New()
	w.spaceOf[c] = space
	return c
}

// grantChannel lets p see channel c.
func (w *hubWorld) grantChannel(p, c ids.ID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.channels[p] == nil {
		w.channels[p] = map[ids.ID]bool{}
	}
	w.channels[p][c] = true
}

// revokeChannel removes channel c from p.
func (w *hubWorld) revokeChannel(p, c ids.ID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.channels[p], c)
}

// grantSpace lets p see every channel of space s, also later ones.
func (w *hubWorld) grantSpace(p, s ids.ID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.spaceGrants[p] == nil {
		w.spaceGrants[p] = map[ids.ID]bool{}
	}
	w.spaceGrants[p][s] = true
}

// sees reports whether p may read an event of space and channel now; w.mu is held.
func (w *hubWorld) sees(p, space, channel ids.ID) bool {
	if w.instanceWide[p] {
		return true
	}
	if !channel.IsZero() {
		space = w.spaceOf[channel]
		if w.channels[p][channel] || w.operator[p][space] {
			return true
		}
	}
	return !space.IsZero() && w.spaceGrants[p][space]
}

// Authorize allows gateway.connect; Revalidate refuses a revoked principal.
func (w *hubWorld) Authorize(_ context.Context, p authorize.Principal, a authorize.Action, _ authorize.Resource) (authorize.Decision, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if d, ok := w.connect[p.PrincipalID()]; ok && a == authorize.ActionGatewayConnect {
		return d, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

// Scope returns the visible sets of p now.
func (w *hubWorld) Scope(_ context.Context, p authorize.Principal, _ authorize.Action) (authorize.Scope, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	id := p.PrincipalID()
	if w.revoked[id] {
		return authorize.Scope{}, errs.New(errs.CodeSessionRevoked, "")
	}
	sc := authorize.Scope{InstanceWide: w.instanceWide[id]}
	spaces := map[ids.ID]bool{}
	for c, sp := range w.spaceOf {
		if w.sees(id, ids.ID{}, c) {
			sc.ChannelIDs = append(sc.ChannelIDs, c)
			spaces[sp] = true
		}
	}
	for s := range w.spaceGrants[id] {
		spaces[s] = true
	}
	for s := range spaces {
		sc.SpaceIDs = append(sc.SpaceIDs, s)
	}
	return sc, nil
}

func (w *hubWorld) Observe(context.Context, authorize.LogEvent) {}

// Authenticate is not used by the hub.
func (w *hubWorld) Authenticate(context.Context, actor.Credentials) (actor.Actor, error) {
	return actor.Actor{}, errs.New(errs.CodeUnauthenticated, "")
}

// Revalidate refuses a revoked principal with 401 session_revoked.
func (w *hubWorld) Revalidate(_ context.Context, a actor.Actor) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.revoked[a.ID] {
		return errs.New(errs.CodeSessionRevoked, "")
	}
	return nil
}

func (w *hubWorld) Check(context.Context, sync.CheckInput) (sync.CheckResult, error) {
	return sync.CheckResult{}, nil
}

func (w *hubWorld) Page(context.Context, sync.PageInput) (sync.PageResult, error) {
	return sync.PageResult{}, nil
}

// PageEvents serves the stored log after the cursor with the admission rule of Admit.
func (w *hubWorld) PageEvents(ctx context.Context, in sync.PageInput) (sync.EventPage, error) {
	memo, _ := authorize.RequestMemoFrom(ctx)
	w.mu.Lock()
	w.pageModes = append(w.pageModes, pageMode{delivery: authorize.IsDelivery(ctx), memo: memo})
	hook, gate := w.onPage, w.gate
	w.pages++
	failing := w.fail > 0
	if failing {
		w.fail--
	}
	w.mu.Unlock()
	if failing {
		return sync.EventPage{}, errs.New(errs.CodeUnavailable, "")
	}
	if hook != nil {
		hook(in.After)
	}
	if gate != nil {
		gate(ctx)
	}
	a, _ := actor.FromContext(ctx)
	w.mu.Lock()
	if w.revoked[a.ID] {
		w.mu.Unlock()
		return sync.EventPage{}, errs.New(errs.CodeSessionRevoked, "")
	}
	var rows []sync.Event
	for _, e := range w.log {
		if e.Seq > in.After && e.Seq <= in.Head && len(rows) < in.Limit {
			rows = append(rows, e)
		}
	}
	w.mu.Unlock()
	page := sync.EventPage{NextCursor: in.Head}
	if len(rows) == in.Limit {
		page.NextCursor = rows[len(rows)-1].Seq
	}
	for _, e := range rows {
		if ok, _ := w.Admit(ctx, a, e); ok {
			page.Events = append(page.Events, e)
		}
	}
	return page, nil
}

// Admit is the admission rule of the world.
func (w *hubWorld) Admit(_ context.Context, a actor.Actor, ev sync.Event) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.revoked[a.ID] {
		return false, errs.New(errs.CodeSessionRevoked, "")
	}
	if w.admitFail[ev.Seq] {
		return false, errs.New(errs.CodeUnavailable, "")
	}
	if !ev.AudiencePrincipalID.IsZero() {
		return ev.AudiencePrincipalID == a.ID, nil
	}
	if ev.SpaceID.IsZero() && ev.ChannelID.IsZero() {
		return true, nil
	}
	return w.sees(a.ID, ev.SpaceID, ev.ChannelID), nil
}

// store appends events to the stored log.
func (w *hubWorld) store(evs ...sync.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.log = append(w.log, evs...)
}

// hubCfg is the configuration of the hub tests.
var hubCfg = app.HubConfig{QueueMax: 100, PageSize: 4, CursorEvery: 3, RetryBase: time.Millisecond, RetryCap: 4 * time.Millisecond,
	PayloadMax: 65536}

// newHub returns a hub over w with cfg.
func newHub(t testing.TB, w *hubWorld, cfg app.HubConfig) *app.Hub {
	t.Helper()
	h, err := app.NewHub(cfg, app.HubDeps{Authenticator: w, Authorizer: w, CatchUp: w})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// person returns a person actor with its own session.
func person() actor.Actor {
	return actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New()}
}

// connect registers a connection of a with a queue of maxFrames and attaches it at from with
// head; it returns the connection's fake.
func connect(t testing.TB, h *app.Hub, a actor.Actor, maxFrames int, from, head int64) *qconn {
	t.Helper()
	qc := newQConn(maxFrames)
	c, s := app.NewTestConnection(qc, a)
	h.Register(context.Background(), c, s)
	if err := h.Attach(context.Background(), c, from, head); err != nil {
		t.Fatal(err)
	}
	return qc
}

// msg returns a message.created event at seq in channel of space.
func msg(seq int64, space, channel ids.ID) sync.Event {
	return sync.Event{Seq: seq, ID: ids.New(), Type: "message.created", SchemaVersion: 1, OccurredAt: t0, SpaceID: space, ChannelID: channel,
		ResourceKind: "message", ResourceID: ids.New(), ReceiveAction: "message.read", Payload: json.RawMessage(`{}`), PermissionVersion: 1}
}

// personal returns a personal event at seq for audience.
func personal(seq int64, audience ids.ID) sync.Event {
	return sync.Event{Seq: seq, ID: ids.New(), Type: "read_state.updated", SchemaVersion: 1, OccurredAt: t0, ResourceKind: "thread",
		ResourceID: ids.New(), ReceiveAction: "self", AudiencePrincipalID: audience, Payload: json.RawMessage(`{}`), PermissionVersion: 1}
}

// accessChanged returns access.changed at seq for audience with the removed channels.
func accessChanged(seq int64, audience ids.ID, removed ...ids.ID) sync.Event {
	type target struct {
		Kind string `json:"kind"`
		ID   ids.ID `json:"id"`
	}
	p := struct {
		PermissionVersion int64    `json:"permission_version"`
		Added             []target `json:"added"`
		Removed           []target `json:"removed"`
		Changed           []target `json:"changed"`
	}{PermissionVersion: 2, Added: []target{}, Removed: []target{}, Changed: []target{}}
	for _, c := range removed {
		p.Removed = append(p.Removed, target{Kind: "channel", ID: c})
	}
	b, _ := json.Marshal(p)
	return sync.Event{Seq: seq, ID: ids.New(), Type: "access.changed", SchemaVersion: 1, OccurredAt: t0, ResourceKind: "principal",
		ResourceID: audience, ReceiveAction: "self", AudiencePrincipalID: audience, Payload: b, PermissionVersion: 2}
}

// accessGranted returns access.changed at seq for audience with the added channels.
func accessGranted(seq int64, audience ids.ID, added ...ids.ID) sync.Event {
	type target struct {
		Kind  string `json:"kind"`
		ID    ids.ID `json:"id"`
		Level string `json:"level"`
	}
	p := struct {
		PermissionVersion int64    `json:"permission_version"`
		Added             []target `json:"added"`
		Removed           []target `json:"removed"`
		Changed           []target `json:"changed"`
	}{PermissionVersion: 3, Added: []target{}, Removed: []target{}, Changed: []target{}}
	for _, c := range added {
		p.Added = append(p.Added, target{Kind: "channel", ID: c, Level: "read"})
	}
	b, _ := json.Marshal(p)
	return sync.Event{Seq: seq, ID: ids.New(), Type: "access.changed", SchemaVersion: 1, OccurredAt: t0, ResourceKind: "principal",
		ResourceID: audience, ReceiveAction: "self", AudiencePrincipalID: audience, Payload: b, PermissionVersion: 3}
}

// waitFor polls cond until it holds or testkit.Patience passes.
func waitFor(t testing.TB, what string, cond func() bool) {
	t.Helper()
	deadline := testkit.Deadline()
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}
