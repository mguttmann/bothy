// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
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
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// liveInterval is sync.run_patch_coalesce_interval of the live-box tests.
const liveInterval = 250 * time.Millisecond

// blockDeadline bounds every wait of TestCoalescerNeverBlocksDispatch.
const blockDeadline = 5 * time.Second

// fakeLiveBox is runs.LiveBox over documents per run with an active flag; it counts its calls and,
// with block set, waits on block (or the end of ctx) and signals entered before it answers.
type fakeLiveBox struct {
	mu      gosync.Mutex
	docs    map[ids.ID]runs.Run
	active  map[ids.ID]bool
	calls   int
	block   chan struct{}
	entered chan struct{}
}

// newFakeLiveBox returns an empty live box that never blocks.
func newFakeLiveBox() *fakeLiveBox {
	return &fakeLiveBox{docs: map[ids.ID]runs.Run{}, active: map[ids.ID]bool{}}
}

// set stores doc under its run id as an active run.
func (b *fakeLiveBox) set(doc runs.Run) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.docs[doc.ID] = doc
	b.active[doc.ID] = true
}

// wait counts one call and blocks while block is set.
func (b *fakeLiveBox) wait(ctx context.Context) {
	b.mu.Lock()
	b.calls++
	block, entered := b.block, b.entered
	b.mu.Unlock()
	if block == nil {
		return
	}
	select {
	case entered <- struct{}{}:
	default:
	}
	select {
	case <-block:
	case <-ctx.Done():
	}
}

// Documents returns the active documents of threads.
func (b *fakeLiveBox) Documents(ctx context.Context, threads []ids.ID) ([]runs.Run, error) {
	b.wait(ctx)
	b.mu.Lock()
	defer b.mu.Unlock()
	want := map[ids.ID]bool{}
	for _, t := range threads {
		want[t] = true
	}
	var out []runs.Run
	for id, d := range b.docs {
		if b.active[id] && want[d.ThreadID] {
			out = append(out, d)
		}
	}
	return out, nil
}

// Document returns the document of run and whether it is active.
func (b *fakeLiveBox) Document(ctx context.Context, run ids.ID) (runs.Run, bool, error) {
	b.wait(ctx)
	b.mu.Lock()
	defer b.mu.Unlock()
	d, ok := b.docs[run]
	return d, ok && b.active[run], nil
}

// encodeFake is the RunEncoder of the tests: a small object with id, version and model.
func encodeFake(r runs.Run) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"id": r.ID, "version": r.Version, "model": r.Model})
}

// runDoc returns the document of run in thread at version v.
func runDoc(run, thread ids.ID, v int64) runs.Run {
	return runs.Run{RunSummary: runs.RunSummary{ID: run, ThreadID: thread, Version: v, State: "running"}}
}

// replaceOp returns a replace of path with the JSON string value.
func replaceOp(path, value string) runs.PatchOp {
	return runs.PatchOp{Op: "replace", Path: path, Value: json.RawMessage(strconv.Quote(value))}
}

// lenv is the focus world with a bound live box, a fake live box and the hub's metrics.
type lenv struct {
	*fenv
	lb      *app.Coalescer
	box     *fakeLiveBox
	metrics *fakeMetrics
	agent   actor.Actor
}

// newLiveEnv returns the gateway of newFocusEnv over a hub with payloadMax as
// sync.event_payload_max_bytes and fake metrics, with the live box bound.
func newLiveEnv(t *testing.T, payloadMax int) *lenv {
	t.Helper()
	e := &fenv{t: t, w: newFocusWorld(), clk: clock.NewFake(t0), limiter: &fakeOpLimiter{denyReads: map[ids.ID]bool{}, denyTyping: map[ids.ID]bool{}},
		support: ids.New()}
	m := &fakeMetrics{}
	h, err := app.NewHub(app.HubConfig{QueueMax: 100, PageSize: 4, CursorEvery: 1000, RetryBase: time.Millisecond, RetryCap: time.Millisecond,
		PayloadMax: payloadMax}, app.HubDeps{Authenticator: e.w, Authorizer: e.w, CatchUp: e.w, Clock: e.clk, Metrics: m})
	if err != nil {
		t.Fatal(err)
	}
	e.hub = h
	gw, err := app.New(app.Config{
		HTU: "https://chat.example.com/api/v1/gateway", IdentifyTimeout: 10 * time.Second, HeartbeatInterval: 30 * time.Second,
		HeartbeatMissLimit: 2, ResumeWindow: 5 * time.Minute, ReauthLeadTime: time.Minute, ProtocolVersions: []int{1},
		MaxFocusedThreads: maxFocused, TypingMinInterval: typingInterval, TypingTTL: typingTTL,
	}, app.Deps{
		Authenticator: e.w, Authorizer: e.w, Nonces: staticNonce("nonce-1"), Limiter: fakeLimiter{}, Snapshot: fakeSnapshot{head: 41, log: &callLog{}},
		CatchUp: e.w, Epoch: staticEpoch{}, Delivery: h, Decoder: jsonDecoder{}, Alarm: &fakeAlarm{}, Clock: e.clk,
		Threads: e.w, OpLimiter: e.limiter, Publisher: h,
	})
	if err != nil {
		t.Fatal(err)
	}
	for op, action := range map[string]authorize.Action{
		"identify": authorize.ActionGatewayConnect, "resume": authorize.ActionGatewayConnect, "focus": authorize.ActionGatewayFocus,
	} {
		if err := gw.Register(op, action); err != nil {
			t.Fatal(err)
		}
	}
	e.gw = gw
	box := newFakeLiveBox()
	lb, err := h.BindLiveBox(app.LiveBoxConfig{Interval: liveInterval}, app.LiveBoxDeps{Documents: box, Encode: encodeFake})
	if err != nil {
		t.Fatal(err)
	}
	return &lenv{fenv: e, lb: lb, box: box, metrics: m, agent: e.newAgent()}
}

// tick runs one round of the coalescer at the fake clock's now.
func (e *lenv) tick() { e.lb.Tick(context.Background()) }

// windowEnd advances the clock by one interval and runs one round.
func (e *lenv) windowEnd() {
	e.clk.Advance(liveInterval)
	e.tick()
}

// publish publishes run.updated of run in th from base to v as runs does after a commit.
func (e *lenv) publish(th chat.ThreadInfo, run ids.ID, base, v int64, truncated bool, ops ...runs.PatchOp) {
	e.t.Helper()
	err := e.hub.PublishEphemeral(context.Background(), runUpdatedEvent(e.agent, th, run, base, v, truncated, ops...))
	if err != nil {
		e.t.Fatal(err)
	}
}

// runUpdatedEvent is the ephemeral run.updated of run in th from base to v by agent.
func runUpdatedEvent(agent actor.Actor, th chat.ThreadInfo, run ids.ID, base, v int64, truncated bool, ops ...runs.PatchOp) eventlog.EphemeralEvent {
	return eventlog.EphemeralEvent{Type: "run.updated", SchemaVersion: 1,
		Actor:    &eventlog.ActorRef{Kind: eventlog.ActorAgent, PrincipalID: agent.ID},
		Scope:    eventlog.Scope{SpaceID: th.SpaceID, ChannelID: th.ChannelID, ThreadID: th.ID},
		Resource: eventlog.ResourceRef{Kind: "run", ID: run, Version: v},
		Payload:  runs.RunUpdated{RunID: run, ThreadID: th.ID, BaseVersion: base, Version: v, Ops: ops, Truncated: truncated},
	}
}

// runStateChanged is the durable run.state_changed of run in th to state at seq.
func runStateChanged(th chat.ThreadInfo, run ids.ID, seq, v int64, state string) sync.Event {
	return sync.Event{Seq: seq, ID: ids.New(), Type: "run.state_changed", SchemaVersion: 1, OccurredAt: t0,
		SpaceID: th.SpaceID, ChannelID: th.ChannelID, ThreadID: th.ID, ResourceKind: "run", ResourceID: run, ResourceVersion: v,
		ReceiveAction: "run.read", Payload: json.RawMessage(`{"run":{"state":"` + state + `"}}`), PermissionVersion: 1}
}

// joinOn opens a connection of a on conn (whose frames fc records) and identifies it; it is live
// afterwards.
func (e *lenv) joinOn(a actor.Actor, conn app.Conn, fc *fakeConn) *client {
	e.t.Helper()
	c, err := e.gw.Open(context.Background(), conn)
	if err != nil {
		e.t.Fatal(err)
	}
	e.send(c, "identify", map[string]any{"protocol_version": 1, "access_token": e.token(a), "dpop": "proof", "device_id": a.DeviceID})
	wantOpen(e.t, fc)
	if got := e.hub.PhaseOf(c); got != domain.PhaseLive {
		e.t.Fatalf("phase %s after identify", got)
	}
	return &client{a: a, c: c, fc: fc}
}

// registered registers a connection of a on the hub without Attach (it stays in catch-up) and
// focuses th through the hub.
func (e *lenv) registered(a actor.Actor, th chat.ThreadInfo) *client {
	fc := &fakeConn{}
	c, sess := app.NewTestConnection(fc, a)
	e.hub.Register(context.Background(), c, sess)
	e.hub.Focus(c, domain.Focus{Entries: []domain.FocusEntry{{ThreadID: th.ID, ChannelID: th.ChannelID, SpaceID: th.SpaceID}}})
	return &client{a: a, c: c, fc: fc}
}

// state fails unless the coalescer holds exactly the given numbers of runs, sent versions and
// marks, and no pending focus set.
func (e *lenv) state(runsN, sentN, marksN int) {
	e.t.Helper()
	r, s, m, f := e.lb.LiveBoxState()
	if r != runsN || s != sentN || m != marksN || f != 0 {
		e.t.Fatalf("coalescer state runs %d sent %d marks %d focused %d, want %d %d %d 0", r, s, m, f, runsN, sentN, marksN)
	}
}

// capConn is a fake connection whose outbound queue holds capacity dispatch entries until drain;
// Offer refuses when it is full and never closes. The frames go to the embedded fakeConn.
type capConn struct {
	*fakeConn
	qmu      gosync.Mutex
	capacity int
	queued   int
}

// newCapConn returns a queue of capacity entries.
func newCapConn(capacity int) *capConn { return &capConn{fakeConn: &fakeConn{}, capacity: capacity} }

// Offer queues e when the queue has room.
func (c *capConn) Offer(e domain.Entry) bool {
	c.qmu.Lock()
	if c.queued >= c.capacity {
		c.qmu.Unlock()
		return false
	}
	c.queued++
	c.qmu.Unlock()
	return c.fakeConn.Offer(e)
}

// drain empties the queue as a writer would.
func (c *capConn) drain() {
	c.qmu.Lock()
	defer c.qmu.Unlock()
	c.queued = 0
}

// wireRunOp is one op of a run.updated frame as a client reads it.
type wireRunOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// runFrame is a run.updated frame as a client reads it.
type runFrame struct {
	RunID       string          `json:"run_id"`
	BaseVersion *int64          `json:"base_version"`
	Version     int64           `json:"version"`
	Ops         *[]wireRunOp    `json:"ops"`
	Snapshot    json.RawMessage `json:"snapshot"`
	truncated   bool
}

// frameReader returns the run.updated frames of one client since its last call.
type frameReader struct {
	cl   *client
	seen int
}

// next checks every new run.updated frame of the client as a client reads it (op dispatch, no
// seq, no permission_version, resource {kind run, id run_id, version}, no JSON null) and returns
// them.
func (r *frameReader) next(t *testing.T) []runFrame {
	t.Helper()
	var all []runFrame
	for _, e := range sent[domain.Entry](r.cl.fc) {
		var env struct {
			Op string         `json:"op"`
			D  map[string]any `json:"d"`
		}
		if err := json.Unmarshal(e.Frame, &env); err != nil {
			t.Fatal(err)
		}
		if env.Op != "dispatch" || env.D["type"] != "run.updated" {
			continue
		}
		all = append(all, checkRunFrame(t, e.Frame, env.D))
	}
	out := all[r.seen:]
	r.seen = len(all)
	return out
}

// checkRunFrame checks the envelope d of the frame raw and returns its payload.
func checkRunFrame(t *testing.T, raw []byte, d map[string]any) runFrame {
	t.Helper()
	if hasJSONNull(t, raw) {
		t.Fatalf("frame with a JSON null: %s", raw)
	}
	if _, ok := d["seq"]; ok {
		t.Fatalf("ephemeral frame with seq: %s", raw)
	}
	if _, ok := d["permission_version"]; ok {
		t.Fatalf("ephemeral frame with permission_version: %s", raw)
	}
	payload, err := json.Marshal(d["payload"])
	if err != nil {
		t.Fatal(err)
	}
	var f runFrame
	if err := json.Unmarshal(payload, &f); err != nil {
		t.Fatal(err)
	}
	f.truncated, _ = d["truncated"].(bool)
	res, _ := d["resource"].(map[string]any)
	if res["kind"] != "run" || res["id"] != f.RunID || res["version"] != float64(f.Version) {
		t.Fatalf("resource %v for run %s version %d", res, f.RunID, f.Version)
	}
	return f
}

// hasJSONNull reports whether the JSON raw holds a null literal.
func hasJSONNull(t *testing.T, raw []byte) bool {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if tok == nil {
			return true
		}
	}
}

// wantSnapshot fails unless f is the whole document of run at version v: snapshot and version,
// neither base_version nor ops, not truncated.
func wantSnapshot(t *testing.T, f runFrame, run ids.ID, v int64) {
	t.Helper()
	if f.RunID != run.String() || f.Version != v || f.BaseVersion != nil || f.Ops != nil || len(f.Snapshot) == 0 || f.truncated {
		t.Fatalf("frame %+v, want the snapshot of %s at version %d", f, run, v)
	}
	var snap struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	if err := json.Unmarshal(f.Snapshot, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.ID != run.String() || snap.Version != v {
		t.Fatalf("snapshot %s, want run %s version %d", f.Snapshot, run, v)
	}
}

// wantOps fails unless f is the ops frame of run from base to v with exactly ops.
func wantOps(t *testing.T, f runFrame, run ids.ID, base, v int64, ops ...runs.PatchOp) {
	t.Helper()
	if f.RunID != run.String() || f.BaseVersion == nil || *f.BaseVersion != base || f.Version != v || f.Ops == nil || len(f.Snapshot) != 0 || f.truncated {
		t.Fatalf("frame %+v, want ops of %s from %d to %d", f, run, base, v)
	}
	got := *f.Ops
	if len(got) != len(ops) {
		t.Fatalf("ops %+v, want %+v", got, ops)
	}
	for i, o := range ops {
		if got[i].Op != o.Op || got[i].Path != o.Path || string(got[i].Value) != string(o.Value) {
			t.Fatalf("op %d %+v, want %+v", i, got[i], o)
		}
	}
}

// wantNone fails when frames holds a frame.
func wantNone(t *testing.T, frames []runFrame, when string) {
	t.Helper()
	if len(frames) != 0 {
		t.Fatalf("%d run.updated frames %s: %+v", len(frames), when, frames)
	}
}

// liveScene is a live env with Sam reading thread th and the active run at version 1.
type liveScene struct {
	*lenv
	sam actor.Actor
	th  chat.ThreadInfo
	run ids.ID
}

// newLiveScene returns the scene over a hub with payloadMax.
func newLiveScene(t *testing.T, payloadMax int) *liveScene {
	t.Helper()
	e := newLiveEnv(t, payloadMax)
	s := &liveScene{lenv: e, sam: e.newPerson(), th: e.w.thread(e.support), run: ids.New()}
	e.w.grant(s.sam.ID, s.th.ChannelID, levelRead)
	e.box.set(runDoc(s.run, s.th.ID, 1))
	return s
}

// focused joins a with conn, focuses th and ticks once, so the connection has the snapshot at the
// box's version as its last frame; it returns the client and its reader after that snapshot.
func (s *liveScene) focused(a actor.Actor, conn app.Conn, fc *fakeConn, v int64) (*client, *frameReader) {
	s.t.Helper()
	cl := s.joinOn(a, conn, fc)
	s.focus(cl, false, s.th.ID)
	s.tick()
	r := &frameReader{cl: cl}
	fs := r.next(s.t)
	if len(fs) != 1 {
		s.t.Fatalf("%d frames after focus, want the snapshot", len(fs))
	}
	wantSnapshot(s.t, fs[0], s.run, v)
	return cl, r
}

// plain returns a fake connection whose queue never fills.
func plain() (app.Conn, *fakeConn) {
	fc := &fakeConn{}
	return fc, fc
}

// TestCoalescerOneFramePerWindow: three patches inside sync.run_patch_coalesce_interval give
// exactly one frame from the first base to the last version at the window end, with the merged
// ops (a replace of the same path once, with the last value), and nothing before the window end.
func TestCoalescerOneFramePerWindow(t *testing.T) {
	t.Parallel()
	s := newLiveScene(t, 65536)
	conn, fc := plain()
	_, r := s.focused(s.sam, conn, fc, 1)
	s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", "Reading export.log"))
	s.publish(s.th, s.run, 2, 3, false, replaceOp("/model", "model-a"))
	s.publish(s.th, s.run, 3, 4, false, replaceOp("/status_line", "Writing report.md"))
	s.box.set(runDoc(s.run, s.th.ID, 4))
	s.tick()
	wantNone(t, r.next(t), "at the window start")
	s.clk.Advance(liveInterval - time.Millisecond)
	s.tick()
	wantNone(t, r.next(t), "before the window end")
	s.clk.Advance(time.Millisecond)
	s.tick()
	fs := r.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames at the window end, want 1", len(fs))
	}
	wantOps(t, fs[0], s.run, 1, 4, replaceOp("/model", "model-a"), replaceOp("/status_line", "Writing report.md"))
	s.windowEnd()
	wantNone(t, r.next(t), "after the window")
}

// TestCoalescerGapRestartsChain: a change whose base is above the chain's version restarts the
// chain at that base; a connection whose last version is 3 gets the snapshot, one whose last
// version is the new base gets the ops.
func TestCoalescerGapRestartsChain(t *testing.T) {
	t.Parallel()
	s := newLiveScene(t, 65536)
	s.box.set(runDoc(s.run, s.th.ID, 2))
	conn, fc := plain()
	_, samR := s.focused(s.sam, conn, fc, 2)
	s.publish(s.th, s.run, 2, 3, false, replaceOp("/status_line", "Reading export.log"))
	s.box.set(runDoc(s.run, s.th.ID, 3))
	s.windowEnd()
	fs := samR.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames, want the ops 2 -> 3", len(fs))
	}
	wantOps(t, fs[0], s.run, 2, 3, replaceOp("/status_line", "Reading export.log"))

	// Version 4 comes without a patch (a state step); Robin focuses at 4.
	s.box.set(runDoc(s.run, s.th.ID, 4))
	robin := s.newPerson()
	s.w.grant(robin.ID, s.th.ChannelID, levelRead)
	rconn, rfc := plain()
	_, robinR := s.focused(robin, rconn, rfc, 4)
	wantNone(t, samR.next(t), "on Sam's connection after Robin's focus")

	s.publish(s.th, s.run, 4, 5, false, replaceOp("/model", "model-b"))
	s.box.set(runDoc(s.run, s.th.ID, 5))
	s.windowEnd()
	fs = samR.next(t)
	if len(fs) != 1 {
		t.Fatalf("Sam got %d frames, want the snapshot", len(fs))
	}
	wantSnapshot(t, fs[0], s.run, 5)
	fs = robinR.next(t)
	if len(fs) != 1 {
		t.Fatalf("Robin got %d frames, want the ops 4 -> 5", len(fs))
	}
	wantOps(t, fs[0], s.run, 4, 5, replaceOp("/model", "model-b"))
}

// TestCoalescerJoinedConnectionGetsSnapshot: a connection that focuses during an open window gets
// its focus snapshot and at the window end the snapshot (its version is not the chain's base),
// never a stale ops frame; the connection that was there gets the ops.
func TestCoalescerJoinedConnectionGetsSnapshot(t *testing.T) {
	t.Parallel()
	s := newLiveScene(t, 65536)
	conn, fc := plain()
	_, samR := s.focused(s.sam, conn, fc, 1)
	s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", "Reading export.log"))
	s.box.set(runDoc(s.run, s.th.ID, 2))

	robin := s.newPerson()
	s.w.grant(robin.ID, s.th.ChannelID, levelRead)
	rconn, rfc := plain()
	_, robinR := s.focused(robin, rconn, rfc, 2)
	wantNone(t, samR.next(t), "on Sam's connection before the window end")

	s.publish(s.th, s.run, 2, 3, false, replaceOp("/status_line", "Writing report.md"))
	s.box.set(runDoc(s.run, s.th.ID, 3))
	s.windowEnd()
	fs := samR.next(t)
	if len(fs) != 1 {
		t.Fatalf("Sam got %d frames, want the ops", len(fs))
	}
	wantOps(t, fs[0], s.run, 1, 3, replaceOp("/status_line", "Writing report.md"))
	fs = robinR.next(t)
	if len(fs) != 1 {
		t.Fatalf("Robin got %d frames, want one snapshot", len(fs))
	}
	wantSnapshot(t, fs[0], s.run, 3)
}

// TestCoalescerQueueFullGivesSnapshot: at a full queue the frame is dropped, the connection is not
// closed, bothy.gateway.outbound.dropped counts patch_replaced_by_snapshot, the mark is retried
// every interval until the queue has room, and then exactly one snapshot of the newest version is
// sent.
func TestCoalescerQueueFullGivesSnapshot(t *testing.T) {
	t.Parallel()
	s := newLiveScene(t, 65536)
	cc := newCapConn(1)
	_, r := s.focused(s.sam, cc, cc.fakeConn, 1)
	// The focus snapshot fills the queue.
	s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", "Reading export.log"))
	s.publish(s.th, s.run, 2, 3, false, replaceOp("/status_line", "Writing report.md"))
	s.box.set(runDoc(s.run, s.th.ID, 3))
	s.windowEnd()
	wantNone(t, r.next(t), "at a full queue")
	wantOpen(t, cc.fakeConn)
	s.metrics.has(t, "dropped patch_replaced_by_snapshot 1")
	s.state(1, 1, 1)

	s.windowEnd()
	wantNone(t, r.next(t), "while the queue is still full")
	wantOpen(t, cc.fakeConn)
	s.state(1, 1, 1)

	cc.drain()
	s.windowEnd()
	fs := r.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames once the queue has room, want one snapshot", len(fs))
	}
	wantSnapshot(t, fs[0], s.run, 3)
	s.state(1, 1, 0)
	cc.drain()
	s.windowEnd()
	wantNone(t, r.next(t), "after the snapshot")
	wantOpen(t, cc.fakeConn)
	drops := 0
	for _, l := range s.metrics.all() {
		if strings.HasPrefix(l, "dropped ") {
			if l != "dropped patch_replaced_by_snapshot 1" {
				t.Errorf("metric %q", l)
			}
			drops++
		}
	}
	// The window frame and at least the retry one interval later were dropped; a retry in the
	// round of the drop counts as well.
	if drops < 2 {
		t.Errorf("%d drops, want the window frame and the retry", drops)
	}
}

// TestCoalescerAdmitsAtFlush: run.read removed between the patch and the window end gives no
// frame, and no retry later.
func TestCoalescerAdmitsAtFlush(t *testing.T) {
	t.Parallel()
	s := newLiveScene(t, 65536)
	conn, fc := plain()
	_, r := s.focused(s.sam, conn, fc, 1)
	s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", "Reading export.log"))
	s.box.set(runDoc(s.run, s.th.ID, 2))
	s.w.grant(s.sam.ID, s.th.ChannelID, 0)
	s.windowEnd()
	wantNone(t, r.next(t), "after run.read was removed")
	s.windowEnd()
	wantNone(t, r.next(t), "one interval later")
	s.state(1, 1, 0)
}

// TestCoalescerDropsStateOnPruneAndRunEnd: a focus without the thread and Detach drop the
// connection's state; a run.state_changed to an end state drops the run's state after its last
// window, and no snapshot is retried for it afterwards.
func TestCoalescerDropsStateOnPruneAndRunEnd(t *testing.T) {
	t.Parallel()
	t.Run("unfocus after a change", func(t *testing.T) {
		t.Parallel()
		s := newLiveScene(t, 65536)
		conn, fc := plain()
		cl, r := s.focused(s.sam, conn, fc, 1)
		s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", "Reading export.log"))
		s.windowEnd()
		if fs := r.next(t); len(fs) != 1 {
			t.Fatalf("%d frames, want the ops", len(fs))
		}
		s.state(1, 1, 0)
		other := s.w.thread(s.support)
		s.w.grant(s.sam.ID, other.ChannelID, levelRead)
		s.focus(cl, false, other.ID)
		s.tick()
		s.state(1, 0, 0)
	})
	t.Run("unfocus after the focus snapshot only", func(t *testing.T) {
		t.Parallel()
		s := newLiveScene(t, 65536)
		conn, fc := plain()
		cl, _ := s.focused(s.sam, conn, fc, 1)
		s.state(0, 1, 0)
		other := s.w.thread(s.support)
		s.w.grant(s.sam.ID, other.ChannelID, levelRead)
		s.focus(cl, false, other.ID)
		s.tick()
		s.state(0, 0, 0)
	})
	t.Run("detach", func(t *testing.T) {
		t.Parallel()
		s := newLiveScene(t, 65536)
		conn, fc := plain()
		cl, _ := s.focused(s.sam, conn, fc, 1)
		s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", "Reading export.log"))
		s.windowEnd()
		// A second connection whose snapshot cannot be queued holds a mark and a pending focus.
		robin := s.newPerson()
		s.w.grant(robin.ID, s.th.ChannelID, levelRead)
		cc := newCapConn(0)
		rcl := s.joinOn(robin, cc, cc.fakeConn)
		s.focus(rcl, false, s.th.ID)
		s.tick()
		s.state(1, 1, 1)
		s.hub.Detach(cl.c)
		s.hub.Detach(rcl.c)
		s.state(1, 0, 0)
	})
	t.Run("run end", func(t *testing.T) {
		t.Parallel()
		s := newLiveScene(t, 65536)
		conn, fc := plain()
		_, samR := s.focused(s.sam, conn, fc, 1)
		robin := s.newPerson()
		s.w.grant(robin.ID, s.th.ChannelID, levelRead)
		cc := newCapConn(1)
		_, robinR := s.focused(robin, cc, cc.fakeConn, 1)
		cc.drain()
		s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", "Done"))
		s.box.set(runDoc(s.run, s.th.ID, 2))
		// The durable frame fills Robin's queue, so the last window is dropped there.
		s.hub.Dispatch(context.Background(), runStateChanged(s.th, s.run, 42, 2, "completed"))
		wantOpen(t, cc.fakeConn)
		s.windowEnd()
		fs := samR.next(t)
		if len(fs) != 1 {
			t.Fatalf("Sam got %d frames, want the last window", len(fs))
		}
		wantOps(t, fs[0], s.run, 1, 2, replaceOp("/status_line", "Done"))
		wantNone(t, robinR.next(t), "on Robin's full queue")
		s.state(0, 0, 0)
		cc.drain()
		s.windowEnd()
		wantNone(t, robinR.next(t), "after the run ended")
		wantNone(t, samR.next(t), "after the run ended")
	})
}

// TestCoalescerNoFrameBeforeCatchupComplete: a connection in catch-up gets no run.updated, neither
// its focus snapshot nor a window; once it is live, the snapshot of the newest version arrives and
// no ops frame.
func TestCoalescerNoFrameBeforeCatchupComplete(t *testing.T) {
	t.Parallel()
	s := newLiveScene(t, 65536)
	cl := s.registered(s.sam, s.th)
	r := &frameReader{cl: cl}
	s.tick()
	wantNone(t, r.next(t), "in catch-up")
	s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", "Reading export.log"))
	s.box.set(runDoc(s.run, s.th.ID, 2))
	s.windowEnd()
	wantNone(t, r.next(t), "at a window end in catch-up")
	s.windowEnd()
	wantNone(t, r.next(t), "one interval later in catch-up")
	if err := s.hub.Attach(context.Background(), cl.c, 41, 41); err != nil {
		t.Fatal(err)
	}
	s.tick()
	fs := r.next(t)
	if len(fs) == 0 {
		t.Fatal("no snapshot once live")
	}
	for _, f := range fs {
		wantSnapshot(t, f, s.run, 2)
	}
	s.windowEnd()
	wantNone(t, r.next(t), "after the snapshot")
}

// TestCoalescerOversizeFrame: an ops frame that reaches sync.event_payload_max_bytes is replaced
// by the snapshot; a snapshot that reaches it goes out with only run_id and version and the
// envelope's truncated; a change published truncated gives the snapshot too.
func TestCoalescerOversizeFrame(t *testing.T) {
	t.Parallel()
	s := newLiveScene(t, 300)
	conn, fc := plain()
	_, r := s.focused(s.sam, conn, fc, 1)
	long := strings.Repeat("x", 400)

	s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", long))
	s.box.set(runDoc(s.run, s.th.ID, 2))
	s.windowEnd()
	fs := r.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames for oversize ops, want the snapshot", len(fs))
	}
	wantSnapshot(t, fs[0], s.run, 2)

	s.publish(s.th, s.run, 2, 3, false, replaceOp("/model", long))
	big := runDoc(s.run, s.th.ID, 3)
	big.Model = long
	s.box.set(big)
	s.windowEnd()
	fs = r.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames for an oversize snapshot, want one truncated frame", len(fs))
	}
	f := fs[0]
	if !f.truncated || f.RunID != s.run.String() || f.Version != 3 || f.BaseVersion != nil || f.Ops != nil || len(f.Snapshot) != 0 {
		t.Fatalf("frame %+v, want only run_id and version 3 with truncated", f)
	}

	s.publish(s.th, s.run, 3, 4, true)
	s.box.set(runDoc(s.run, s.th.ID, 4))
	s.windowEnd()
	fs = r.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames for a truncated change, want the snapshot", len(fs))
	}
	wantSnapshot(t, fs[0], s.run, 4)
}

// TestCoalescerNeverBlocksDispatch: while Run waits on a live box whose reads block,
// PublishEphemeral of run.updated, Hub.Dispatch of a durable event and a focus return.
func TestCoalescerNeverBlocksDispatch(t *testing.T) {
	t.Parallel()
	s := newLiveScene(t, 65536)
	block := make(chan struct{})
	s.box.mu.Lock()
	s.box.block, s.box.entered = block, make(chan struct{}, 1)
	s.box.mu.Unlock()
	conn, fc := plain()
	cl := s.joinOn(s.sam, conn, fc)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.lb.Run(ctx)
	}()
	defer func() {
		close(block)
		cancel()
		select {
		case <-done:
		case <-time.After(blockDeadline):
			t.Error("Run did not end")
		}
	}()
	s.focus(cl, false, s.th.ID)
	select {
	case <-s.box.entered:
	case <-time.After(blockDeadline):
		t.Fatal("the coalescer never read the documents")
	}
	returned := make(chan error, 1)
	go func() {
		bg := context.Background()
		err := s.hub.PublishEphemeral(bg, runUpdatedEvent(s.agent, s.th, s.run, 1, 2, false, replaceOp("/status_line", "Reading export.log")))
		s.hub.Dispatch(bg, runStateChanged(s.th, s.run, 42, 2, "running"))
		s.hub.Focus(cl.c, domain.Focus{})
		returned <- err
	}()
	select {
	case err := <-returned:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(blockDeadline):
		t.Fatal("publish, dispatch or focus waited on the coalescer")
	}
}

// TestFocusSendsDocumentOnce: a newly focused thread with an active run gets exactly one snapshot
// after Tick; the same set again gets none; a thread without an active run and an unreadable
// thread get none; during catch-up the snapshot waits until the connection is live.
func TestFocusSendsDocumentOnce(t *testing.T) {
	t.Parallel()
	e := newLiveEnv(t, 65536)
	sam := e.newPerson()
	withRun, withoutRun, unreadable := e.w.thread(e.support), e.w.thread(e.support), e.w.thread(e.support)
	e.w.grant(sam.ID, withRun.ChannelID, levelRead)
	e.w.grant(sam.ID, withoutRun.ChannelID, levelRead)
	run, hidden := ids.New(), ids.New()
	e.box.set(runDoc(run, withRun.ID, 3))
	e.box.set(runDoc(hidden, unreadable.ID, 1))
	conn, fc := plain()
	cl := e.joinOn(sam, conn, fc)
	r := &frameReader{cl: cl}

	e.focus(cl, false, withRun.ID, withoutRun.ID)
	wantNone(t, r.next(t), "before Tick")
	e.tick()
	fs := r.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames after the focus, want one snapshot", len(fs))
	}
	wantSnapshot(t, fs[0], run, 3)

	e.focus(cl, false, withRun.ID, withoutRun.ID)
	e.tick()
	wantNone(t, r.next(t), "for the same set")

	e.hub.ForceFocus(cl.c, unreadable.ID)
	e.tick()
	wantNone(t, r.next(t), "for an unreadable thread")

	robin := e.newPerson()
	e.w.grant(robin.ID, withRun.ChannelID, levelRead)
	rcl := e.registered(robin, withRun)
	rr := &frameReader{cl: rcl}
	e.tick()
	wantNone(t, rr.next(t), "in catch-up")
	if err := e.hub.Attach(context.Background(), rcl.c, 41, 41); err != nil {
		t.Fatal(err)
	}
	e.tick()
	fs = rr.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames once live, want one snapshot", len(fs))
	}
	wantSnapshot(t, fs[0], run, 3)
	e.tick()
	wantNone(t, rr.next(t), "after the snapshot")
}

// TestResumeSendsSnapshotForRestoredFocus: after resume the restored focus gets one snapshot per
// thread with an active run, and none for a thread without one.
func TestResumeSendsSnapshotForRestoredFocus(t *testing.T) {
	t.Parallel()
	e := newLiveEnv(t, 65536)
	sam := e.newPerson()
	a, b, c := e.w.thread(e.support), e.w.thread(e.support), e.w.thread(e.support)
	for _, th := range []chat.ThreadInfo{a, b, c} {
		e.w.grant(sam.ID, th.ChannelID, levelRead)
	}
	runA, runC := ids.New(), ids.New()
	e.box.set(runDoc(runA, a.ID, 2))
	e.box.set(runDoc(runC, c.ID, 5))
	conn, fc := plain()
	cl := e.joinOn(sam, conn, fc)
	e.focus(cl, false, a.ID, b.ID, c.ID)
	e.tick()
	if fs := (&frameReader{cl: cl}).next(t); len(fs) != 2 {
		t.Fatalf("%d frames before resume, want 2", len(fs))
	}
	sid := sent[app.Ready](fc)[0].SessionID
	cl.c.Close(domain.CloseGoingAway, "")
	e.gw.Closed(cl.c)

	nfc := &fakeConn{}
	nc, err := e.gw.Open(context.Background(), nfc)
	if err != nil {
		t.Fatal(err)
	}
	e.send(nc, "resume", map[string]any{"gateway_session_id": sid, "access_token": e.token(sam), "dpop": "proof", "cursor": 41})
	wantOpen(t, nfc)
	if len(sent[app.Resumed](nfc)) != 1 {
		t.Fatal("no resumed")
	}
	r := &frameReader{cl: &client{a: sam, c: nc, fc: nfc}}
	wantNone(t, r.next(t), "before Tick")
	e.tick()
	fs := r.next(t)
	if len(fs) != 2 {
		t.Fatalf("%d frames after resume, want one snapshot per active run", len(fs))
	}
	for _, f := range fs {
		switch f.RunID {
		case runA.String():
			wantSnapshot(t, f, runA, 2)
		case runC.String():
			wantSnapshot(t, f, runC, 5)
		default:
			t.Fatalf("snapshot of %s", f.RunID)
		}
	}
	if fs[0].RunID == fs[1].RunID {
		t.Fatalf("two snapshots of %s", fs[0].RunID)
	}
	e.tick()
	wantNone(t, r.next(t), "after the snapshots")
}

// TestCoalescerNoSecondSnapshotOfSentVersion: a retried snapshot that already carries the
// newest version of a pending window ends the stall; the window end sends nothing more to that
// connection instead of the same snapshot again.
func TestCoalescerNoSecondSnapshotOfSentVersion(t *testing.T) {
	t.Parallel()
	s := newLiveScene(t, 65536)
	cc := newCapConn(1)
	_, r := s.focused(s.sam, cc, cc.fakeConn, 1)
	s.publish(s.th, s.run, 1, 2, false, replaceOp("/status_line", "Reading export.log"))
	s.box.set(runDoc(s.run, s.th.ID, 2))
	s.windowEnd()
	wantNone(t, r.next(t), "at a full queue")
	// The next patch opens a window; the retry of the mark reads the document at its version.
	s.publish(s.th, s.run, 2, 3, false, replaceOp("/status_line", "Writing report.md"))
	s.box.set(runDoc(s.run, s.th.ID, 3))
	cc.drain()
	s.tick()
	fs := r.next(t)
	if len(fs) != 1 {
		t.Fatalf("%d frames at the retry, want one snapshot", len(fs))
	}
	wantSnapshot(t, fs[0], s.run, 3)
	cc.drain()
	s.windowEnd()
	wantNone(t, r.next(t), "at the window end of the version the snapshot carried")
}
