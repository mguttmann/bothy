// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	gosync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// consumerLog is the event log of the engine tests: every event is visible unless hidden.
type consumerLog struct {
	mu     gosync.Mutex
	events []sync.Event
	hidden map[int64]bool
	head   int64
	reason sync.ResyncReason
}

// add appends events up to seq n.
func (l *consumerLog) add(n int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for s := l.head + 1; s <= n; s++ {
		l.events = append(l.events, sync.Event{ID: ids.New(), Seq: s, Type: "message.created", OccurredAt: t0, Payload: json.RawMessage(`{}`)})
	}
	l.head = max(l.head, n)
}

func (l *consumerLog) Check(_ context.Context, _ sync.CheckInput) (sync.CheckResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return sync.CheckResult{HeadSeq: l.head, SyncEpoch: epoch, Reason: l.reason}, nil
}

func (l *consumerLog) Page(context.Context, sync.PageInput) (sync.PageResult, error) {
	return sync.PageResult{}, errors.New("not used")
}

func (l *consumerLog) PageEvents(_ context.Context, in sync.PageInput) (sync.EventPage, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := sync.EventPage{NextCursor: in.Head}
	rows := 0
	for _, ev := range l.events {
		if ev.Seq <= in.After || ev.Seq > in.Head {
			continue
		}
		rows++
		if !l.hidden[ev.Seq] {
			out.Events = append(out.Events, ev)
		}
		if rows == in.Limit {
			out.NextCursor = ev.Seq
			break
		}
	}
	return out, nil
}

func (l *consumerLog) Admit(context.Context, actor.Actor, sync.Event) (bool, error) { return true, nil }

// cursorStore is agents.Consumers in memory: the holder fencing of 03 and a call log.
type cursorStore struct {
	mu     gosync.Mutex
	acked  map[agents.ConsumerKey]int64
	holder map[agents.ConsumerKey]ids.ID
	calls  []string
	// onTake runs inside Take, before the holder is written.
	onTake func()
}

func newCursorStore() *cursorStore {
	return &cursorStore{acked: map[agents.ConsumerKey]int64{}, holder: map[agents.ConsumerKey]ids.ID{}}
}

func (s *cursorStore) log(c string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, c)
}

func (s *cursorStore) all() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

func (s *cursorStore) Take(ctx context.Context, consumer string, holder ids.ID, head int64) (agents.ConsumerState, error) {
	a, _ := actor.FromContext(ctx)
	if s.onTake != nil {
		s.onTake()
	}
	s.log("take " + holder.String())
	s.mu.Lock()
	defer s.mu.Unlock()
	key := agents.ConsumerKey{AgentID: a.ID, Consumer: consumer}
	s.holder[key] = holder
	if v, ok := s.acked[key]; ok {
		return agents.ConsumerState{Consumer: consumer, AckedSeq: v}, nil
	}
	s.acked[key] = head
	return agents.ConsumerState{Consumer: consumer, AckedSeq: head, Created: true}, nil
}

func (s *cursorStore) AcceptAck(ctx context.Context, _ string, seq, head int64) error {
	a, _ := actor.FromContext(ctx)
	ok := a.Kind == actor.KindAgent
	authorize.RecordCheck(ctx, authorize.Check{Action: string(authorize.ActionAgentEventsConsume), Kind: authorize.CheckAuthorize, Allowed: ok})
	if !ok {
		return errs.New(errs.CodeForbidden, "").WithReason("subject_kind_not_allowed")
	}
	if seq < 0 || seq > head {
		return errs.New(errs.CodeValidationFailed, "")
	}
	return nil
}

func (s *cursorStore) Save(_ context.Context, key agents.ConsumerKey, holder ids.ID, acked int64) error {
	s.log("save " + holder.String())
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.holder[key] != holder {
		return agents.ErrConsumerReplaced
	}
	s.acked[key] = max(s.acked[key], acked)
	return nil
}

func (s *cursorStore) Restart(_ context.Context, key agents.ConsumerKey, holder ids.ID, seq int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.holder[key] != holder {
		return agents.ErrConsumerReplaced
	}
	s.acked[key] = seq
	return nil
}

func (s *cursorStore) stored(key agents.ConsumerKey) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acked[key]
}

// hubPos is the hub position of the tests.
type hubPos struct{ v atomic.Int64 }

func (p *hubPos) Position() int64 { return p.v.Load() }

// dropHook drops the seqs of drop.
type dropHook struct{ drop map[int64]bool }

func (h dropHook) Prepare(_ context.Context, _ ids.ID, ev sync.Envelope) (sync.Envelope, bool, error) {
	if ev.Seq != nil && h.drop[*ev.Seq] {
		return ev, false, nil
	}
	return ev, true, nil
}

// engineEnv is a consumer engine over the fakes with one agent.
type engineEnv struct {
	t       *testing.T
	e       *app.ConsumerEngine
	log     *consumerLog
	cursors *cursorStore
	pos     *hubPos
	clk     *clock.Fake
	agent   actor.Actor
	cfg     app.ConsumerConfig
}

// newEngineEnv returns the engine with mod applied to its configuration and hook.
func newEngineEnv(t *testing.T, mod func(*app.ConsumerConfig, *app.ConsumerDeps)) *engineEnv {
	t.Helper()
	env := &engineEnv{t: t, log: &consumerLog{hidden: map[int64]bool{}}, cursors: newCursorStore(), pos: &hubPos{}, clk: clock.NewFake(t0),
		agent: actor.Actor{ID: ids.New(), Kind: actor.KindAgent, TokenID: ids.New()}}
	env.cfg = app.ConsumerConfig{MaxInflight: 3, AckTimeout: time.Hour, PersistInterval: time.Hour, IdleWindow: time.Minute, PageSize: 4,
		CursorEvery: 100, RetryBase: time.Millisecond, RetryCap: 10 * time.Millisecond}
	d := app.ConsumerDeps{Consumers: env.cursors, CatchUp: env.log, Position: env.pos, Hook: dropHook{}, Epoch: staticEpoch{}, Clock: env.clk}
	if mod != nil {
		mod(&env.cfg, &d)
	}
	e, err := app.NewConsumerEngine(env.cfg, d)
	if err != nil {
		t.Fatal(err)
	}
	env.e = e
	return env
}

// publish appends events up to n and moves the hub position there.
func (env *engineEnv) publish(n int64) {
	env.log.add(n)
	env.pos.v.Store(n)
	env.e.WakeAll()
}

// attach opens a consumer connection of the agent for consumer at head.
func (env *engineEnv) attach(consumer string, head int64) (*app.Connection, *app.Session, *qconn) {
	env.t.Helper()
	q := newQConn(64)
	c, s := app.NewTestConnection(q, env.agent)
	s.Consumer = consumer
	if err := env.e.Attach(context.Background(), c, s, head); err != nil {
		env.t.Fatal(err)
	}
	env.t.Cleanup(func() { env.e.Detach(c) })
	return c, s, q
}

// dispatch is a dispatch frame as an agent reads it.
type dispatch struct {
	seq         int64
	redelivered bool
}

// written drains q and returns its dispatch frames.
func written(t *testing.T, q *qconn) []dispatch {
	t.Helper()
	var ds []dispatch
	for _, e := range q.drain() {
		var f struct {
			Op string `json:"op"`
			D  struct {
				Seq         int64 `json:"seq"`
				Redelivered bool  `json:"redelivered"`
			} `json:"d"`
		}
		if err := json.Unmarshal(e.Frame, &f); err != nil {
			t.Fatal(err)
		}
		if f.Op == "dispatch" {
			ds = append(ds, dispatch{f.D.Seq, f.D.Redelivered})
		}
	}
	return ds
}

// seqsOf returns the seqs of ds.
func seqsOf(ds []dispatch) []int64 {
	out := make([]int64, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.seq)
	}
	return out
}

// waitDispatches drains q until cond holds for its dispatches or testkit.Patience passed.
func waitDispatches(t *testing.T, q *qconn, cond func([]dispatch) bool) []dispatch {
	t.Helper()
	deadline := testkit.Deadline()
	for {
		ds := written(t, q)
		if cond(ds) {
			return ds
		}
		if time.Now().After(deadline) {
			t.Fatalf("dispatches %v", ds)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// count returns a condition "exactly n dispatches".
func count(n int) func([]dispatch) bool { return func(ds []dispatch) bool { return len(ds) >= n } }

// settle waits a little and returns the dispatches, to show that nothing more arrives.
func settle(t *testing.T, q *qconn) []dispatch {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	ds := written(t, q)
	return ds
}

// TestConsumerRunner_WaitsAtMaxInflight: with nothing acknowledged exactly agent.max_inflight_events
// events are dispatched; acknowledging k of them releases exactly k more (04 section 15.2 item 2).
func TestConsumerRunner_WaitsAtMaxInflight(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, nil)
	c, _, q := env.attach("default", 0)
	env.publish(10)
	waitDispatches(t, q, count(3))
	if ds := settle(t, q); !slices.Equal(seqsOf(ds), []int64{1, 2, 3}) {
		t.Fatalf("dispatched %v", seqsOf(ds))
	}
	env.e.Ack(c, 2)
	waitDispatches(t, q, count(5))
	if ds := settle(t, q); !slices.Equal(seqsOf(ds), []int64{1, 2, 3, 4, 5}) {
		t.Fatalf("after ack 2 dispatched %v", seqsOf(ds))
	}
}

// TestConsumerRunner_ReadsOnlyUpToHubPosition: the runner never reads beyond the hub position,
// even when the log holds later events.
func TestConsumerRunner_ReadsOnlyUpToHubPosition(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, func(c *app.ConsumerConfig, _ *app.ConsumerDeps) { c.MaxInflight = 50 })
	_, _, q := env.attach("default", 0)
	env.log.add(10)
	env.pos.v.Store(4)
	env.e.WakeAll()
	waitDispatches(t, q, count(4))
	if ds := settle(t, q); !slices.Equal(seqsOf(ds), []int64{1, 2, 3, 4}) {
		t.Fatalf("dispatched %v beyond the hub position 4", seqsOf(ds))
	}
	env.pos.v.Store(10)
	env.e.WakeAll()
	waitDispatches(t, q, count(10))
}

// TestConsumerRunner_RedeliversAfterAckTimeout: without an acknowledgement the events come again
// from acked_seq + 1 with redelivered; the first delivery has no flag (04 section 15.2 item 3).
func TestConsumerRunner_RedeliversAfterAckTimeout(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, func(c *app.ConsumerConfig, _ *app.ConsumerDeps) { c.AckTimeout = 10 * time.Second })
	c, _, q := env.attach("default", 0)
	env.publish(2)
	first := waitDispatches(t, q, count(2))
	if first[0].redelivered || first[1].redelivered {
		t.Fatalf("first delivery flagged %v", first)
	}
	env.e.Ack(c, 1)
	// The fake clock passes the timeout only after the ack, so only 2 is still unacknowledged.
	env.clk.Advance(10 * time.Second)
	env.e.WakeAll()
	all := waitDispatches(t, q, count(3))
	if again := all[2]; again.seq != 2 || !again.redelivered {
		t.Fatalf("redelivery %v", all)
	}
	env.e.Ack(c, 2)
	if ds := settle(t, q); len(ds) != 3 {
		t.Fatalf("after the ack %v", ds)
	}
}

// TestConsumerRunner_DeliveryHookCanDrop: an envelope the hook rejects is not sent, counts as
// checked and holds no budget.
func TestConsumerRunner_DeliveryHookCanDrop(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, func(c *app.ConsumerConfig, d *app.ConsumerDeps) {
		c.MaxInflight = 2
		d.Hook = dropHook{drop: map[int64]bool{2: true}}
	})
	_, _, q := env.attach("default", 0)
	env.publish(3)
	waitDispatches(t, q, count(2))
	if ds := settle(t, q); !slices.Equal(seqsOf(ds), []int64{1, 3}) {
		t.Fatalf("dispatched %v", seqsOf(ds))
	}
}

// TestConsumerTakeover_FlushTakeCloseOrder: a second identify of the same consumer in this process
// saves the acknowledgement of the old session, then takes the consumer, then closes the old
// connection with 4015; the old one is gone from the engine (brief M2-07 note 4).
func TestConsumerTakeover_FlushTakeCloseOrder(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, nil)
	c1, s1, q1 := env.attach("default", 0)
	env.publish(3)
	waitDispatches(t, q1, count(3))
	env.e.Ack(c1, 2)
	var openAtTake bool
	env.cursors.onTake = func() { openAtTake = q1.closedWith() == 0 }
	_, s2, q2 := env.attach("default", 3)
	calls := env.cursors.all()
	want := []string{"take " + s1.ID.String(), "save " + s1.ID.String(), "take " + s2.ID.String()}
	if !slices.Equal(calls, want) || !openAtTake {
		t.Fatalf("calls %v (open at take %v), want %v", calls, openAtTake, want)
	}
	if q1.closedWith() != domain.CloseConsumerReplaced {
		t.Fatalf("old connection closed with %d", q1.closedWith())
	}
	if conns, _ := env.e.Len(); conns != 1 {
		t.Fatalf("%d consumer connections", conns)
	}
	env.e.Detach(c1)
	if got := env.cursors.all(); len(got) != 3 {
		t.Fatalf("a detach of the replaced connection saved: %v", got)
	}
	env.publish(5)
	waitDispatches(t, q2, count(2))
	if got := env.cursors.stored(agents.ConsumerKey{AgentID: env.agent.ID, Consumer: "default"}); got != 2 {
		t.Fatalf("stored %d", got)
	}
}

// TestConsumerSave_ReplacedCloses4015: a save that finds another holder (a takeover in another
// process) closes the connection with 4015.
func TestConsumerSave_ReplacedCloses4015(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, nil)
	c, _, q := env.attach("default", 0)
	env.publish(1)
	waitDispatches(t, q, count(1))
	env.e.Ack(c, 1)
	if _, err := env.cursors.Take(actor.WithActor(context.Background(), env.agent), "default", ids.New(), 1); err != nil {
		t.Fatal(err)
	}
	env.e.FlushAll(context.Background())
	if q.closedWith() != domain.CloseConsumerReplaced {
		t.Fatalf("closed with %d", q.closedWith())
	}
}

// TestConsumerFlush_OnCloseAndShutdown: the held acknowledgement is saved when the connection
// detaches and when the shutdown flushes every consumer.
func TestConsumerFlush_OnCloseAndShutdown(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, nil)
	c1, _, q1 := env.attach("default", 0)
	c2, _, q2 := env.attach("worker-1", 0)
	env.publish(3)
	waitDispatches(t, q1, count(3))
	waitDispatches(t, q2, count(3))
	env.e.Ack(c1, 3)
	env.e.Ack(c2, 2)
	env.e.Detach(c1)
	if got := env.cursors.stored(agents.ConsumerKey{AgentID: env.agent.ID, Consumer: "default"}); got != 3 {
		t.Fatalf("after the detach stored %d", got)
	}
	env.e.FlushAll(context.Background())
	if got := env.cursors.stored(agents.ConsumerKey{AgentID: env.agent.ID, Consumer: "worker-1"}); got != 2 {
		t.Fatalf("after the shutdown flush stored %d", got)
	}
}

// pollCtx is the request context of the agent.
func (env *engineEnv) pollCtx() context.Context {
	return actor.WithActor(context.Background(), env.agent)
}

// envelopes decodes the seqs and flags of a REST page.
func envelopes(t *testing.T, p agents.FeedPage) []dispatch {
	t.Helper()
	out := make([]dispatch, 0, len(p.Events))
	for _, raw := range p.Events {
		var v struct {
			Seq         int64 `json:"seq"`
			Redelivered bool  `json:"redelivered"`
		}
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		if testkit.HasJSONNull(raw) {
			t.Fatalf("JSON null in %s", raw)
		}
		out = append(out, dispatch{v.Seq, v.Redelivered})
	}
	return out
}

// TestRestWindow_NoRepeatWithinTimeout: the first poll creates the cursor at the hub position and
// answers an empty page; within agent.ack_timeout a poll never repeats inflight events; after it
// they come again with redelivered (brief M2-07 note 10).
func TestRestWindow_NoRepeatWithinTimeout(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, func(c *app.ConsumerConfig, _ *app.ConsumerDeps) { c.AckTimeout = 10 * time.Second })
	env.publish(5)
	first, err := env.e.Poll(env.pollCtx(), "default", 0)
	if err != nil || len(first.Events) != 0 || first.NextCursor != 5 || first.HeadSeq != 5 {
		t.Fatalf("first poll %+v, %v", first, err)
	}
	env.publish(10)
	p, err := env.e.Poll(env.pollCtx(), "default", 2)
	if got := envelopes(t, p); err != nil || !slices.Equal(seqsOf(got), []int64{6, 7}) || p.NextCursor != 7 {
		t.Fatalf("poll %v next %d, %v", got, p.NextCursor, err)
	}
	p, _ = env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); !slices.Equal(seqsOf(got), []int64{8}) {
		t.Fatalf("second poll %v, want only 8 (budget 3)", got)
	}
	p, _ = env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); len(got) != 0 {
		t.Fatalf("a poll at zero budget %v", got)
	}
	env.clk.Advance(10 * time.Second)
	p, _ = env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); !slices.Equal(got, []dispatch{{6, true}, {7, true}, {8, true}}) {
		t.Fatalf("after the timeout %v", got)
	}
}

// TestRestWindow_SweepDuringTakeKeepsTheNewWindow: a sweep that runs while a poll takes the
// consumer over (here inside Take) must not drop its REST window as idle, neither a new one nor an
// existing one at the edge of sync.resume_window; otherwise the poll delivers from an orphaned window and the next poll starts
// a fresh one at the stored acknowledgement and repeats the event within agent.ack_timeout,
// without redelivered (CI flake of TestConsumerRedeliveryAfterAckTimeout).
func TestRestWindow_SweepDuringTakeKeepsTheNewWindow(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, nil) // agent.ack_timeout an hour, sync.resume_window a minute
	env.cursors.onTake = func() { env.e.Sweep(env.clk.Now()) }
	env.publish(5)
	if _, err := env.e.Poll(env.pollCtx(), "default", 0); err != nil {
		t.Fatal(err)
	}
	env.publish(6)
	p, err := env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); err != nil || !slices.Equal(got, []dispatch{{6, false}}) {
		t.Fatalf("delivering poll %v, %v", got, err)
	}
	p, err = env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); err != nil || len(got) != 0 {
		t.Fatalf("a poll inside the timeout repeated %v, %v", got, err)
	}
	if _, w := env.e.Len(); w != 1 {
		t.Fatalf("%d REST windows, want the one of the poll", w)
	}
	// An existing window at the edge of sync.resume_window: the poll that comes just before the
	// edge keeps it although a sweep at the edge runs inside its Take.
	env.clk.Advance(time.Minute - time.Millisecond)
	env.cursors.onTake = func() { env.e.Sweep(env.clk.Now().Add(time.Millisecond)) }
	env.publish(7)
	p, err = env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); err != nil || !slices.Equal(got, []dispatch{{7, false}}) {
		t.Fatalf("poll at the edge %v, %v", got, err)
	}
	env.cursors.onTake = nil
	p, err = env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); err != nil || len(got) != 0 {
		t.Fatalf("a poll after the edge repeated %v, %v", got, err)
	}
}

// TestRestWindow_IdleSweptAfterResumeWindow: a REST window idle for sync.resume_window is dropped.
func TestRestWindow_IdleSweptAfterResumeWindow(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, nil)
	env.publish(1)
	if _, err := env.e.Poll(env.pollCtx(), "default", 0); err != nil {
		t.Fatal(err)
	}
	env.e.Sweep(env.clk.Now().Add(time.Minute - time.Second))
	if _, w := env.e.Len(); w != 1 {
		t.Fatal("swept before sync.resume_window")
	}
	env.e.Sweep(env.clk.Now().Add(time.Minute))
	if _, w := env.e.Len(); w != 0 {
		t.Fatal("not swept after sync.resume_window")
	}
}

// TestPollTakesOverConnection: a poll of the consumer of an open connection in this process
// flushes it and closes it with 4015.
func TestPollTakesOverConnection(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, nil)
	c, s, q := env.attach("default", 0)
	env.publish(2)
	waitDispatches(t, q, count(2))
	env.e.Ack(c, 2)
	if _, err := env.e.Poll(env.pollCtx(), "default", 0); err != nil {
		t.Fatal(err)
	}
	if q.closedWith() != domain.CloseConsumerReplaced || !slices.Contains(env.cursors.all(), "save "+s.ID.String()) {
		t.Fatalf("closed %d calls %v", q.closedWith(), env.cursors.all())
	}
}

// TestAckOp_ErrorKeepsConnection: an ack above the head and an ack of a person answer error and
// keep the connection; a valid ack answers nothing.
func TestAckOp_ErrorKeepsConnection(t *testing.T) {
	t.Parallel()
	cursors := newCursorStore()
	pos := &hubPos{}
	pos.v.Store(10)
	e := newEnv(t, func(_ *app.Config, d *app.Deps) {
		eng, err := app.NewConsumerEngine(app.ConsumerConfig{MaxInflight: 3, AckTimeout: time.Hour, PersistInterval: time.Hour, IdleWindow: time.Minute,
			PageSize: 4, CursorEvery: 100, RetryBase: time.Millisecond, RetryCap: time.Millisecond},
			app.ConsumerDeps{Consumers: cursors, CatchUp: &consumerLog{}, Position: pos, Hook: dropHook{}, Epoch: staticEpoch{}})
		if err != nil {
			t.Fatal(err)
		}
		d.Consumers = eng
	})
	if err := e.gw.Register("ack", authorize.ActionAgentEventsConsume); err != nil {
		t.Fatal(err)
	}
	c, fc := e.open()
	e.send(c, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
	wantOpen(t, fc)
	e.send(c, "ack", map[string]any{"seq": 11})
	e.send(c, "ack", map[string]any{"seq": 10})
	wantOpen(t, fc)
	errsSent := sent[app.ErrorFrame](fc)
	if len(errsSent) != 1 || errsSent[0].Code != "validation_failed" || errsSent[0].Op != "ack" {
		t.Fatalf("error frames %+v", errsSent)
	}
	p, pc := e.open()
	e.identifyAlex(p)
	e.send(p, "ack", map[string]any{"seq": 1})
	wantOpen(t, pc)
	if got := sent[app.ErrorFrame](pc); len(got) != 1 || got[0].Code != "forbidden" {
		t.Fatalf("error frames of a person %+v", got)
	}
}

// TestConsumerResumeAfterTakeoverCloses4015: a session that was detached when another connection
// took its consumer over cannot take it back by resume: the resume closes with 4015 and the newer
// connection stays (R-SY-09, review M2-07 major 1).
func TestConsumerResumeAfterTakeoverCloses4015(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, nil)
	c1, s1, _ := env.attach("default", 0)
	env.e.Detach(c1)
	_, _, q2 := env.attach("default", 0)
	q3 := newQConn(64)
	c3, _ := app.NewTestConnection(q3, env.agent)
	err := env.e.Attach(context.Background(), c3, s1, 0)
	var ce *app.CloseError
	if !errors.As(err, &ce) || ce.Code != domain.CloseConsumerReplaced {
		t.Fatalf("resume of the replaced session = %v", err)
	}
	if q2.closedWith() != 0 {
		t.Fatalf("the newer connection closed with %d", q2.closedWith())
	}
}

// TestRestWindow_NextCursorNeverCoversEarlierInflight: next_cursor stays below an event an earlier
// poll handed out that is not acknowledged, so a lost response cannot acknowledge it away (review
// M2-07 major 2).
func TestRestWindow_NextCursorNeverCoversEarlierInflight(t *testing.T) {
	t.Parallel()
	env := newEngineEnv(t, nil)
	env.publish(5)
	if _, err := env.e.Poll(env.pollCtx(), "default", 0); err != nil {
		t.Fatal(err)
	}
	env.publish(10)
	if p, err := env.e.Poll(env.pollCtx(), "default", 2); err != nil || p.NextCursor != 7 {
		t.Fatalf("poll %+v, %v", p, err)
	}
	p, err := env.e.Poll(env.pollCtx(), "default", 0)
	if got := envelopes(t, p); err != nil || !slices.Equal(seqsOf(got), []int64{8}) || p.NextCursor != 5 {
		t.Fatalf("second poll %v next %d, %v; want [8] and next 5", got, p.NextCursor, err)
	}
	p, _ = env.e.Poll(env.pollCtx(), "default", 0)
	if len(p.Events) != 0 || p.NextCursor != 5 {
		t.Fatalf("a poll at zero budget %+v", p)
	}
}
