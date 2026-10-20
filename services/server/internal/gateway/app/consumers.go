// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	gosync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// DeliveryHook is the outgoing port every dispatch to an agent consumer passes, on the WebSocket
// and over REST (brief M2-07 note 7, 02 section 10): Prepare returns the envelope to deliver, or
// false for "do not deliver, treat as checked". adapters/rundelivery binds it to runs.Delivery
// (M2-08: the delivery check and a fresh invocation token). An error closes the connection with 4000 or fails the
// poll; the event comes again from acked_seq + 1.
type DeliveryHook interface {
	Prepare(ctx context.Context, agentID ids.ID, ev sync.Envelope) (sync.Envelope, bool, error)
}

// Position is the hub position of the process: the highest seq the log tail has passed through
// Authorizer.Observe and Hub.Dispatch. Consumers read the log only up to it, so every admission
// uses a rights cache that has seen every access.changed up to that event (brief M2-07 note 6).
type Position interface {
	Position() int64
}

// ConsumerConfig holds the configuration keys of the agent consumers.
type ConsumerConfig struct {
	// MaxInflight is agent.max_inflight_events.
	MaxInflight int
	// AckTimeout is agent.ack_timeout.
	AckTimeout time.Duration
	// PersistInterval is agent.ack_persist_interval, the period of the save ticker.
	PersistInterval time.Duration
	// IdleWindow is sync.resume_window: a REST window idle that long is dropped.
	IdleWindow time.Duration
	// PageSize is sync.catchup_page_size.
	PageSize int
	// CursorEvery is sync.cursor_advance_every.
	CursorEvery int
	// RetryBase is sync.log_poll_interval, the first pause after a failed page read; it doubles
	// up to RetryCap, database.listen_reconnect_backoff_cap.
	RetryBase, RetryCap time.Duration
}

// validate names every invalid value.
func (c ConsumerConfig) validate() error {
	if c.MaxInflight < 1 || c.AckTimeout <= 0 || c.PersistInterval <= 0 || c.IdleWindow <= 0 || c.PageSize < 1 || c.CursorEvery < 1 ||
		c.RetryBase <= 0 || c.RetryCap <= 0 {
		return errors.New("gateway: agent.max_inflight_events, agent.ack_timeout, agent.ack_persist_interval, sync.resume_window, " +
			"sync.catchup_page_size, sync.cursor_advance_every, sync.log_poll_interval and database.listen_reconnect_backoff_cap must be positive")
	}
	return nil
}

// window returns the window configuration.
func (c ConsumerConfig) window() domain.WindowConfig {
	return domain.WindowConfig{MaxInflight: c.MaxInflight, AckTimeout: c.AckTimeout, CursorEvery: int64(c.CursorEvery)}
}

// ConsumerDeps are the collaborators of the consumer engine.
type ConsumerDeps struct {
	// Consumers is the cursor port of agents; the engine reaches agent_consumer_cursors only
	// through it.
	Consumers agents.Consumers
	// CatchUp reads the pages and decides resync_required (sync).
	CatchUp sync.CatchUp
	// Position is the hub position.
	Position Position
	// Hook is the delivery hook (adapters/rundelivery over runs.Delivery, M2-08).
	Hook DeliveryHook
	// Epoch reads the sync epoch for REST pages.
	Epoch EpochSource
	// Clock is the time of the windows; nil means clock.System().
	Clock clock.Clock
	// Log receives takeovers and failures; nil means slog.Default(). It never receives event
	// contents or tokens.
	Log *slog.Logger
}

// consumerConn is the state of one agent consumer connection: its key, the connection and
// session, the window and the last saved acknowledgement. mu guards win, saved and gone.
type consumerConn struct {
	key   agents.ConsumerKey
	c     *Connection
	s     *Session
	actor actor.Actor
	head  int64

	mu    gosync.Mutex
	win   *domain.ConsumerWindow
	saved int64
	gone  bool
	// streaming is true once catchup.complete or resync_required was queued: from then on the
	// hub may offer ephemeral frames to the connection (INT-W41/1).
	streaming bool

	wake   chan struct{}
	cancel context.CancelFunc
}

// held returns the acknowledgement the connection holds and whether it is above the saved one.
func (cc *consumerConn) held() (acked int64, dirty bool) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return cc.win.Acked(), cc.win.Acked() > cc.saved
}

// markSaved records acked as saved.
func (cc *consumerConn) markSaved(acked int64) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.saved = max(cc.saved, acked)
}

// signal wakes the runner without blocking; signals coalesce.
func (cc *consumerConn) signal() {
	select {
	case cc.wake <- struct{}{}:
	default:
	}
}

// restWindow is the REST window of one consumer in this process: its own holder id (a UUIDv7
// created with the window), the window and the time of its last poll. mu guards win and used.
type restWindow struct {
	holder ids.ID
	mu     gosync.Mutex
	win    *domain.ConsumerWindow
	used   time.Time
}

// ConsumerEngine is the one delivery engine of agent consumers (brief M2-07 notes 1 and 4 to 10):
// a runner goroutine per consumer connection that pulls the log up to the hub position through a
// ConsumerWindow, the per-process holder index, the REST windows behind agents.ConsumerFeed, the
// save ticker (Run) and the flushes on takeover, detach and shutdown. A per-process lock per
// ConsumerKey serializes takeovers by identify, resume and REST polls. It is safe for concurrent
// use.
type ConsumerEngine struct {
	cfg ConsumerConfig
	d   ConsumerDeps

	mu     gosync.Mutex
	locks  map[agents.ConsumerKey]*gosync.Mutex
	conns  map[agents.ConsumerKey]*consumerConn
	byConn map[*Connection]*consumerConn
	rest   map[agents.ConsumerKey]*restWindow
}

// Compile-time check: the engine is the feed of the REST operations.
var _ agents.ConsumerFeed = (*ConsumerEngine)(nil)

// NewConsumerEngine returns the engine. Errors: an invalid configuration value or a missing
// collaborator.
func NewConsumerEngine(cfg ConsumerConfig, d ConsumerDeps) (*ConsumerEngine, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if d.Consumers == nil || d.CatchUp == nil || d.Position == nil || d.Hook == nil || d.Epoch == nil {
		return nil, errors.New("gateway: NewConsumerEngine needs the consumers, catch-up, the position, the hook and the epoch")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &ConsumerEngine{cfg: cfg, d: d, locks: map[agents.ConsumerKey]*gosync.Mutex{}, conns: map[agents.ConsumerKey]*consumerConn{},
		byConn: map[*Connection]*consumerConn{}, rest: map[agents.ConsumerKey]*restWindow{}}, nil
}

// Log messages and keys of the consumer engine (docs/observability/signals.md "Logs").
const (
	msgConsumerReplaced = "gateway agent consumer taken over"
	msgConsumerSave     = "gateway agent consumer acknowledgement not saved, retrying"
	msgConsumerPage     = "gateway agent consumer page failed, retrying"
	msgConsumerHook     = "gateway agent consumer delivery hook failed"
	errorTypeSave       = "consumer_save"
	errorTypeHook       = "delivery_hook"
	logKeyAgentID       = "bothy.agent.id"
	logKeyConsumer      = "bothy.consumer.name"
	logKeySession       = "bothy.gateway.session_id"
	logKeyPrevSession   = "bothy.gateway.previous_session_id"
	reasonReplaced      = "consumer_replaced"
)

// cursors returns the cursor port of agents (the ack handler checks acknowledgements through it).
func (e *ConsumerEngine) cursors() agents.Consumers { return e.d.Consumers }

// keyLock returns the lock of key.
func (e *ConsumerEngine) keyLock(key agents.ConsumerKey) *gosync.Mutex {
	e.mu.Lock()
	defer e.mu.Unlock()
	l := e.locks[key]
	if l == nil {
		l = &gosync.Mutex{}
		e.locks[key] = l
	}
	return l
}

// Attach starts the consumer of the agent session s on c (brief M2-07 note 4). A fresh session
// takes the consumer over: the acknowledgement of another connection of the key in this process is
// saved, Take writes s as the holder (creating the row at head on the first connection), that
// connection closes with 4015 and a REST window of the key is dropped. A resumed session keeps its
// holder and continues from its acknowledgement in memory. Then the runner starts. Errors: the
// close of a failed Take (4018 forbidden, 4012 revoked, 4004 unauthenticated, 4000 otherwise).
func (e *ConsumerEngine) Attach(ctx context.Context, c *Connection, s *Session, head int64) error {
	_, a := c.current()
	key := agents.ConsumerKey{AgentID: a.ID, Consumer: s.Consumer}
	l := e.keyLock(key)
	l.Lock()
	defer l.Unlock()
	e.mu.Lock()
	old := e.conns[key]
	taken, acked := s.consumerTaken, s.consumerAcked
	e.mu.Unlock()
	if taken {
		// A resume keeps the holder only while the session still holds the row: a fenced save
		// proves it before anything else is touched. Another connection, a REST poll or another
		// process may have taken the consumer over while the session was detached; then the
		// resume closes with 4015 and the newer holder stays (R-SY-09).
		if old != nil && old.s == s {
			acked = max(acked, old.heldNow())
		}
		if err := e.d.Consumers.Save(ctx, key, s.ID, acked); errors.Is(err, agents.ErrConsumerReplaced) {
			return &CloseError{Code: domain.CloseConsumerReplaced, Reason: reasonReplaced}
		} else if err != nil {
			return closeWith(domain.CloseUnknownError)
		}
	}
	if old != nil && old.c != c {
		e.flush(ctx, old)
	}
	if !taken {
		st, err := e.d.Consumers.Take(actor.WithActor(ctx, a), s.Consumer, s.ID, head)
		if err != nil {
			return takeClose(err)
		}
		acked = st.AckedSeq
	}
	if old != nil && old.c != c {
		e.retire(old)
		old.c.Close(domain.CloseConsumerReplaced, reasonReplaced)
		e.d.Log.LogAttrs(ctx, slog.LevelInfo, msgConsumerReplaced, slog.String(logKeyAgentID, key.AgentID.String()),
			slog.String(logKeyConsumer, key.Consumer), slog.String(logKeySession, s.ID.String()), slog.String(logKeyPrevSession, old.s.ID.String()))
	}
	rctx, cancel := context.WithCancel(authorize.WithoutRequestScope(context.WithoutCancel(ctx)))
	cc := &consumerConn{key: key, c: c, s: s, actor: a, head: head, win: domain.NewConsumerWindow(e.cfg.window(), acked, head), saved: acked,
		wake: make(chan struct{}, 1), cancel: cancel}
	e.mu.Lock()
	delete(e.rest, key)
	s.consumerTaken, s.consumerAcked = true, acked
	e.conns[key] = cc
	e.byConn[c] = cc
	e.mu.Unlock()
	go e.run(rctx, cc)
	return nil
}

// heldNow returns the acknowledgement the connection holds.
func (cc *consumerConn) heldNow() int64 {
	acked, _ := cc.held()
	return acked
}

// takeClose maps an error of Take to its close code (04 section 6.4).
func takeClose(err error) error {
	e, ok := errs.As(err)
	switch {
	case !ok:
		return closeWith(domain.CloseUnknownError)
	case e.Kind() == errs.KindForbidden:
		return closeWith(domain.CloseForbidden)
	case e.Code == errs.CodeSessionRevoked:
		return closeWith(domain.CloseRevoked)
	case e.Kind() == errs.KindUnauthenticated:
		return closeWith(domain.CloseAuthenticationFailed)
	}
	return closeWith(domain.CloseUnknownError)
}

// retire removes cc from the indices and stops its runner; a later Detach of its connection does
// nothing. The caller saved its acknowledgement first.
func (e *ConsumerEngine) retire(cc *consumerConn) {
	e.mu.Lock()
	if e.conns[cc.key] == cc {
		delete(e.conns, cc.key)
	}
	if e.byConn[cc.c] == cc {
		delete(e.byConn, cc.c)
	}
	e.mu.Unlock()
	cc.mu.Lock()
	cc.gone = true
	cc.mu.Unlock()
	cc.cancel()
}

// flush saves the held acknowledgement of cc when it is above the saved one. ErrConsumerReplaced
// closes cc with 4015 (the takeover happened in another process, 04 section 15.1 item 3); other
// errors are logged and keep it dirty.
func (e *ConsumerEngine) flush(ctx context.Context, cc *consumerConn) {
	acked, dirty := cc.held()
	if !dirty {
		return
	}
	err := e.d.Consumers.Save(context.WithoutCancel(ctx), cc.key, cc.s.ID, acked)
	switch {
	case err == nil:
		cc.markSaved(acked)
	case errors.Is(err, agents.ErrConsumerReplaced):
		e.retire(cc)
		cc.c.Close(domain.CloseConsumerReplaced, reasonReplaced)
	default:
		e.d.Log.LogAttrs(ctx, slog.LevelWarn, msgConsumerSave, slog.String(logKeyErrorType, errorTypeSave))
	}
}

// Detach ends the consumer of c (any close, 4012 and 4014 included): the runner stops, the held
// acknowledgement is saved and kept in the session for a resume (brief M2-07 note 9).
func (e *ConsumerEngine) Detach(c *Connection) {
	e.mu.Lock()
	cc := e.byConn[c]
	e.mu.Unlock()
	if cc == nil {
		return
	}
	l := e.keyLock(cc.key)
	l.Lock()
	defer l.Unlock()
	e.mu.Lock()
	current := e.byConn[c] == cc
	e.mu.Unlock()
	if !current {
		return
	}
	cc.cancel()
	e.flush(context.Background(), cc)
	e.retire(cc)
	e.mu.Lock()
	cc.s.consumerAcked = max(cc.s.consumerAcked, cc.heldNow())
	e.mu.Unlock()
}

// Ack applies the acknowledgement seq of an ack op to the window of c and wakes its runner;
// nothing is written until the next save (brief M2-07 note 8).
func (e *ConsumerEngine) Ack(c *Connection, seq int64) {
	e.mu.Lock()
	cc := e.byConn[c]
	e.mu.Unlock()
	if cc == nil {
		return
	}
	cc.mu.Lock()
	cc.win.Ack(seq)
	cc.mu.Unlock()
	cc.signal()
}

// AckHead returns the head an ack op of c is checked against: the hub position, or the window's
// own position when it starts above it (the cursor stored at the head of ready).
func (e *ConsumerEngine) AckHead(c *Connection) int64 {
	head := e.d.Position.Position()
	e.mu.Lock()
	cc := e.byConn[c]
	e.mu.Unlock()
	if cc == nil {
		return head
	}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return max(head, cc.win.Read())
}

// Purge drops the inflight entries of the removed targets t from the window of c and wakes its
// runner, so they hold no budget (brief M2-07 note 5).
func (e *ConsumerEngine) Purge(c *Connection, t domain.Targets) {
	e.mu.Lock()
	cc := e.byConn[c]
	e.mu.Unlock()
	if cc == nil {
		return
	}
	cc.mu.Lock()
	cc.win.Purge(t)
	cc.mu.Unlock()
	cc.signal()
}

// WakeAll wakes every runner of this process; the hub calls it after each event it dispatched.
func (e *ConsumerEngine) WakeAll() {
	e.mu.Lock()
	all := make([]*consumerConn, 0, len(e.conns))
	for _, cc := range e.conns {
		all = append(all, cc)
	}
	e.mu.Unlock()
	for _, cc := range all {
		cc.signal()
	}
}

// Len returns the number of consumer connections and REST windows of this process.
func (e *ConsumerEngine) Len() (conns, windows int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.conns), len(e.rest)
}

// run is the runner of one consumer connection (brief M2-07 notes 5 and 6): the resync check of
// the start, then pages up to the hub position while the window has budget, waiting for the hub,
// an acknowledgement or the ack deadline in between. A panic closes the connection with 4000.
func (e *ConsumerEngine) run(ctx context.Context, cc *consumerConn) {
	defer func() {
		if p := recover(); p != nil {
			e.d.Log.LogAttrs(ctx, slog.LevelError, msgPanic, slog.String(logKeyErrorType, errorTypePanic))
			cc.c.Close(domain.CloseUnknownError, "")
		}
	}()
	if !e.startCheck(ctx, cc) {
		return
	}
	for {
		if !e.step(ctx, cc) {
			return
		}
		cc.mu.Lock()
		deadline, ok := cc.win.Deadline()
		cc.mu.Unlock()
		var (
			t     *time.Timer
			timer <-chan time.Time
		)
		if ok {
			t = time.NewTimer(max(deadline.Sub(e.d.Clock.Now()), 0))
			timer = t.C
		}
		select {
		case <-ctx.Done():
		case <-cc.wake:
		case <-timer:
		}
		if t != nil {
			t.Stop()
		}
	}
}

// actorCtx returns ctx with the agent of cc as the caller.
func actorCtx(ctx context.Context, a actor.Actor) context.Context { return actor.WithActor(ctx, a) }

// startCheck is the resync check of the start (brief M2-07 note 5): CatchUp.Check after the
// acknowledgement; retention and cursor_ahead send resync_required, move the cursor to the head
// with Restart and continue live from there. A failing check is retried with backoff. It reports
// false when the connection ended.
func (e *ConsumerEngine) startCheck(ctx context.Context, cc *consumerConn) bool {
	pause := e.cfg.RetryBase
	for {
		cc.mu.Lock()
		after := cc.win.Acked()
		cc.mu.Unlock()
		res, err := e.d.CatchUp.Check(actorCtx(authorize.WithRequestScope(ctx), cc.actor), sync.CheckInput{After: after})
		if errs.KindOf(err) == errs.KindUnauthenticated {
			cc.c.Close(domain.CloseRevoked, "")
			return false
		}
		if err != nil {
			e.d.Log.LogAttrs(ctx, slog.LevelWarn, msgConsumerPage, slog.String(logKeyErrorType, errorTypeCatchUp))
			if !sleepCtx(ctx, pause) {
				return false
			}
			pause = min(2*pause, e.cfg.RetryCap)
			continue
		}
		if !domain.ConsumerResync(string(res.Reason)) {
			return true
		}
		if err := e.d.Consumers.Restart(ctx, cc.key, cc.s.ID, res.HeadSeq); errors.Is(err, agents.ErrConsumerReplaced) {
			e.retire(cc)
			cc.c.Close(domain.CloseConsumerReplaced, reasonReplaced)
			return false
		} else if err != nil {
			e.d.Log.LogAttrs(ctx, slog.LevelWarn, msgConsumerSave, slog.String(logKeyErrorType, errorTypeSave))
		}
		cc.mu.Lock()
		cc.win.Restart(res.HeadSeq)
		cc.saved = max(cc.saved, res.HeadSeq)
		cc.mu.Unlock()
		if cc.c.conn.Send(ResyncRequired{Reason: res.Reason, HeadSeq: res.HeadSeq, SyncEpoch: res.SyncEpoch}) != nil {
			return false
		}
		cc.mu.Lock()
		cc.streaming = true
		cc.mu.Unlock()
		return true
	}
}

// step applies the ack timeout and reads pages up to the hub position while the window has
// budget, queueing the dispatches and the control frames. It reports false when the connection
// ended.
func (e *ConsumerEngine) step(ctx context.Context, cc *consumerConn) bool {
	pause := e.cfg.RetryBase
	for {
		if ctx.Err() != nil {
			return false
		}
		pos := e.d.Position.Position()
		cc.mu.Lock()
		cc.win.Expire(e.d.Clock.Now())
		read, budget := cc.win.Read(), cc.win.Budget()
		frames := cc.win.Frames()
		cc.mu.Unlock()
		if !e.sendFrames(ctx, cc, frames) {
			return false
		}
		if budget == 0 || read >= pos {
			return true
		}
		ok, err := e.readPage(ctx, cc.actor, &cc.mu, cc.win, pos, e.cfg.MaxInflight, func(ev sync.Event, env sync.Envelope) bool {
			return e.dispatch(ctx, cc, ev, env)
		})
		switch {
		case errs.KindOf(err) == errs.KindUnauthenticated:
			cc.c.Close(domain.CloseRevoked, "")
			return false
		case errors.Is(err, errHook):
			cc.c.Close(domain.CloseUnknownError, "")
			return false
		case err != nil:
			e.d.Log.LogAttrs(ctx, slog.LevelWarn, msgConsumerPage, slog.String(logKeyErrorType, errorTypeCatchUp))
			if !sleepCtx(ctx, pause) {
				return false
			}
			pause = min(2*pause, e.cfg.RetryCap)
			continue
		case !ok:
			return false
		}
		pause = e.cfg.RetryBase
	}
}

// dispatch queues the dispatch frame of env for ev on cc; false when the connection ended. An
// envelope that cannot be encoded is logged and skipped.
func (e *ConsumerEngine) dispatch(ctx context.Context, cc *consumerConn, ev sync.Event, env sync.Envelope) bool {
	b, err := json.Marshal(outFrame{Op: opDispatch, D: env})
	if err != nil {
		e.d.Log.LogAttrs(ctx, slog.LevelError, msgEncodeEvent, slog.String(logKeyErrorType, errorTypeEncodeEv))
		return true
	}
	return e.offer(ctx, cc, domain.Entry{Frame: b, Class: domain.ClassDispatch, Seq: ev.Seq, SpaceID: ev.SpaceID,
		ChannelID: ev.ChannelID, OccurredAt: ev.OccurredAt})
}

// errHook marks a failure of the delivery hook.
var errHook = errors.New("gateway: delivery hook failed")

// readPage reads one page after the window's read up to pos in filter mode for the agent a and
// hands at most limit deliverable envelopes to emit, each admitted and passed through the hook;
// mu guards win. An event at or below the acknowledgement or refused by the hook counts as
// checked. When the page is not cut by the budget or limit, read moves to the highest seq the
// page checked. It reports false when emit stopped (the connection ended); errors are those of the
// page read and errHook.
func (e *ConsumerEngine) readPage(ctx context.Context, a actor.Actor, mu *gosync.Mutex, win *domain.ConsumerWindow, pos int64, limit int,
	emit func(sync.Event, sync.Envelope) bool) (bool, error) {
	mu.Lock()
	after := win.Read()
	mu.Unlock()
	// Filter mode with a request scope of its own per page: the rights version is read once per
	// page, so a revocation without a log event (a revoked API token) refuses the next page with
	// unauthenticated; the delivery mode of the hub would only learn it from the next event
	// (docs/DECISIONS.md D-M2-07-7). A REST poll keeps the memo of its request.
	page, err := e.d.CatchUp.PageEvents(actorCtx(authorize.WithRequestScope(authorize.Filter(ctx)), a),
		sync.PageInput{After: after, Head: pos, Limit: e.cfg.PageSize})
	if err != nil {
		return true, err
	}
	sent := 0
	for _, ev := range page.Events {
		mu.Lock()
		if win.Budget() == 0 || sent >= limit {
			mu.Unlock()
			return true, nil
		}
		skip, again := ev.Seq <= win.Acked(), win.Again(ev.Seq)
		mu.Unlock()
		if skip {
			continue
		}
		env, deliver, err := e.d.Hook.Prepare(ctx, a.ID, ev.Envelope(sync.DeliveryFields{IncludeTraceContext: true, Redelivered: again}))
		if err != nil {
			e.d.Log.LogAttrs(ctx, slog.LevelError, msgConsumerHook, slog.String(logKeyErrorType, errorTypeHook))
			return true, fmt.Errorf("%w: %w", errHook, err)
		}
		mu.Lock()
		if !deliver {
			win.Checked(ev.Seq)
			mu.Unlock()
			continue
		}
		env.Redelivered = again
		win.Dispatch(ev.Seq, ev.SpaceID, ev.ChannelID, e.d.Clock.Now())
		mu.Unlock()
		sent++
		if !emit(ev, env) {
			return false, nil
		}
	}
	mu.Lock()
	win.Checked(page.NextCursor)
	mu.Unlock()
	return true, nil
}

// offer queues e on the connection of cc once its queue has room (WaitSpace, leaving the control
// reserve free); false when the connection ended.
func (e *ConsumerEngine) offer(ctx context.Context, cc *consumerConn, entry domain.Entry) bool {
	for {
		if err := cc.c.conn.WaitSpace(ctx); err != nil {
			return false
		}
		cc.mu.Lock()
		gone := cc.gone
		cc.mu.Unlock()
		if gone {
			return false
		}
		if cc.c.conn.Offer(entry) {
			return true
		}
	}
}

// sendFrames queues the control frames of the window; false when the connection ended.
func (e *ConsumerEngine) sendFrames(ctx context.Context, cc *consumerConn, frames []domain.WindowFrame) bool {
	for _, f := range frames {
		var entry domain.Entry
		switch f.Kind {
		case domain.FrameCursor:
			entry = controlEntry("cursor", cursorFrame{Seq: f.Seq}, f.Seq)
		case domain.FrameComplete:
			entry = controlEntry("catchup.complete", completeFrame{HeadSeq: f.Seq}, 0)
		}
		if !e.offer(ctx, cc, entry) {
			return false
		}
		if f.Kind == domain.FrameComplete {
			cc.mu.Lock()
			cc.streaming = true
			cc.mu.Unlock()
		}
	}
	return true
}

// Streaming reports whether the consumer connection c queued catchup.complete or resync_required,
// so ephemeral frames may follow (M1-66, INT-W41/1); false for a connection the engine does not
// serve.
func (e *ConsumerEngine) Streaming(c *Connection) bool {
	e.mu.Lock()
	cc := e.byConn[c]
	e.mu.Unlock()
	if cc == nil {
		return false
	}
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return cc.streaming
}

// sleepCtx waits d; false when ctx ends first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// Run is the save ticker and the REST window sweeper of the process: every agent.ack_persist_interval
// it saves every consumer connection whose held acknowledgement is above its saved one (04
// section 15.2 item 4) and drops the REST windows idle for sync.resume_window, until ctx ends.
// The lifecycle of internal/wire starts and stops it.
func (e *ConsumerEngine) Run(ctx context.Context) {
	t := time.NewTicker(e.cfg.PersistInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.FlushAll(ctx)
			e.sweep(e.d.Clock.Now())
		}
	}
}

// FlushAll saves the held acknowledgement of every consumer connection of this process; step 3 of
// the shutdown calls it before every connection closes with 4013 (02 section 15.3).
func (e *ConsumerEngine) FlushAll(ctx context.Context) {
	e.mu.Lock()
	all := make([]*consumerConn, 0, len(e.conns))
	for _, cc := range e.conns {
		all = append(all, cc)
	}
	e.mu.Unlock()
	for _, cc := range all {
		e.flush(ctx, cc)
	}
}

// sweep drops the REST windows idle for sync.resume_window at now; the next poll starts a fresh
// window from acked_seq + 1.
func (e *ConsumerEngine) sweep(now time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for key, rw := range e.rest {
		rw.mu.Lock()
		idle := !now.Before(rw.used.Add(e.cfg.IdleWindow))
		rw.mu.Unlock()
		if idle {
			delete(e.rest, key)
		}
	}
}

// Poll implements agents.ConsumerFeed (brief M2-07 note 10): under the lock of the key, a gateway
// connection of the key in this process is flushed and closed with 4015; Take writes the holder of
// the REST window (the first poll creates the cursor at the hub position and answers an empty
// page); the window is reconciled with the stored acknowledgement, the ack timeout applied, the
// resync check run, and events read up to the budget and limit.
func (e *ConsumerEngine) Poll(ctx context.Context, consumer string, limit int) (agents.FeedPage, error) {
	a, _ := actor.FromContext(ctx)
	key := agents.ConsumerKey{AgentID: a.ID, Consumer: consumer}
	if limit < 1 || limit > e.cfg.MaxInflight {
		limit = e.cfg.MaxInflight
	}
	l := e.keyLock(key)
	l.Lock()
	defer l.Unlock()
	e.mu.Lock()
	old := e.conns[key]
	rw := e.rest[key]
	if rw == nil {
		rw = &restWindow{holder: ids.New()}
		e.rest[key] = rw
	}
	e.mu.Unlock()
	if old != nil {
		e.replaceForREST(ctx, old, rw.holder)
	}
	pos := e.d.Position.Position()
	st, err := e.d.Consumers.Take(ctx, consumer, rw.holder, pos)
	if err != nil {
		return agents.FeedPage{}, err
	}
	epoch, err := e.d.Epoch.SyncEpoch(ctx)
	if err != nil {
		return agents.FeedPage{}, err
	}
	acked := rw.reconcile(e.cfg.window(), st, e.d.Clock.Now())
	out := agents.FeedPage{Events: []json.RawMessage{}, HeadSeq: pos, SyncEpoch: epoch}
	if st.Created {
		out.NextCursor = st.AckedSeq
		return out, nil
	}
	res, err := e.d.CatchUp.Check(ctx, sync.CheckInput{After: acked})
	if err != nil {
		return agents.FeedPage{}, err
	}
	if domain.ConsumerResync(string(res.Reason)) {
		if err := e.d.Consumers.Restart(ctx, key, rw.holder, res.HeadSeq); err != nil {
			return agents.FeedPage{}, err
		}
		rw.mu.Lock()
		rw.win.Restart(res.HeadSeq)
		rw.mu.Unlock()
		return agents.FeedPage{Events: []json.RawMessage{}, NextCursor: res.HeadSeq, HeadSeq: res.HeadSeq, SyncEpoch: res.SyncEpoch,
			Resync: string(res.Reason)}, nil
	}
	// next_cursor never covers an event an earlier poll handed out that is still unacknowledged:
	// a lost response or a crashed adapter must not acknowledge it away (at least once, 04 section
	// 9); it comes again after agent.ack_timeout.
	rw.mu.Lock()
	earlier, pending := rw.win.OldestInflight()
	rw.mu.Unlock()
	err = e.fillREST(ctx, a, rw, pos, limit, &out)
	rw.mu.Lock()
	out.NextCursor = rw.win.Read()
	if pending {
		out.NextCursor = min(out.NextCursor, earlier-1)
	}
	rw.mu.Unlock()
	if err != nil {
		return agents.FeedPage{}, err
	}
	return out, nil
}

// replaceForREST saves and closes the gateway connection old of a key a REST poll takes over for
// holder (4015), and logs the takeover.
func (e *ConsumerEngine) replaceForREST(ctx context.Context, old *consumerConn, holder ids.ID) {
	e.flush(ctx, old)
	e.retire(old)
	old.c.Close(domain.CloseConsumerReplaced, reasonReplaced)
	e.d.Log.LogAttrs(ctx, slog.LevelInfo, msgConsumerReplaced, slog.String(logKeyAgentID, old.key.AgentID.String()),
		slog.String(logKeyConsumer, old.key.Consumer), slog.String(logKeySession, holder.String()), slog.String(logKeyPrevSession, old.s.ID.String()))
}

// reconcile brings the REST window up to the stored state st of a poll at now: a fresh window at
// the stored acknowledgement when there is none or the row was created, otherwise the stored
// acknowledgement is applied; then the ack timeout. It returns the acknowledgement.
func (rw *restWindow) reconcile(cfg domain.WindowConfig, st agents.ConsumerState, now time.Time) int64 {
	rw.mu.Lock()
	defer rw.mu.Unlock()
	rw.used = now
	if rw.win == nil || st.Created {
		rw.win = domain.NewConsumerWindow(cfg, st.AckedSeq, st.AckedSeq)
		rw.win.Restart(st.AckedSeq)
	} else {
		rw.win.Ack(st.AckedSeq)
	}
	rw.win.Expire(now)
	return rw.win.Acked()
}

// fillREST reads pages into out until the budget or limit is used or the window reaches pos.
func (e *ConsumerEngine) fillREST(ctx context.Context, a actor.Actor, rw *restWindow, pos int64, limit int, out *agents.FeedPage) error {
	for {
		rw.mu.Lock()
		read, budget := rw.win.Read(), rw.win.Budget()
		rw.mu.Unlock()
		left := limit - len(out.Events)
		if budget == 0 || left <= 0 || read >= pos {
			return nil
		}
		var encErr error
		if _, err := e.readPage(ctx, a, &rw.mu, rw.win, pos, left, func(_ sync.Event, env sync.Envelope) bool {
			b, err := json.Marshal(env)
			if err != nil {
				encErr = errs.Wrap(errs.CodeInternalError, err, "gateway: encoding an envelope")
				return false
			}
			out.Events = append(out.Events, b)
			return true
		}); err != nil {
			return err
		}
		if encErr != nil {
			return encErr
		}
		rw.mu.Lock()
		progressed := rw.win.Read() > read
		rw.mu.Unlock()
		if !progressed {
			return nil
		}
	}
}

// Acked implements agents.ConsumerFeed: the windows of key in this process take over the stored
// acknowledgement acked (ackAgentEvents), and a gateway connection counts it as saved.
func (e *ConsumerEngine) Acked(_ context.Context, key agents.ConsumerKey, acked int64) {
	e.mu.Lock()
	rw, cc := e.rest[key], e.conns[key]
	e.mu.Unlock()
	if rw != nil {
		rw.mu.Lock()
		if rw.win != nil {
			rw.win.Ack(acked)
		}
		rw.mu.Unlock()
	}
	if cc != nil {
		cc.mu.Lock()
		cc.win.Ack(acked)
		cc.saved = max(cc.saved, acked)
		cc.mu.Unlock()
		cc.signal()
	}
}

// Head implements agents.ConsumerFeed: the hub position and the sync epoch.
func (e *ConsumerEngine) Head(ctx context.Context) (int64, ids.ID, error) {
	epoch, err := e.d.Epoch.SyncEpoch(ctx)
	if err != nil {
		return 0, ids.ID{}, err
	}
	return e.d.Position.Position(), epoch, nil
}
