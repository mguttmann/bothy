// SPDX-License-Identifier: Apache-2.0

package wsrelay

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	gosync "sync"
	"time"

	"github.com/coder/websocket"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
)

// RelayUseCases are the callbacks of the relay into the use cases (app.Service, M2-44 Design note
// 7): every state change and the counters of a pairing.
type RelayUseCases interface {
	RelayActivate(ctx context.Context, id ids.ID) error
	RelayDetach(ctx context.Context, id ids.ID) error
	RelayOpenFailed(ctx context.Context, id ids.ID, detail domain.CloseDetail) error
	RelayCounters(ctx context.Context, id ids.ID, c app.Counts) error
	RelayUnpaired(ctx context.Context, side domain.Side)
	// RelaySessionState and RelayOpeningTimedOut are the reads and the compare-and-swap end of
	// M2-47 (Design notes 3 and 8).
	RelaySessionState(ctx context.Context, id ids.ID) (string, bool, error)
	RelayOpeningTimedOut(ctx context.Context, id ids.ID) error
	RelayExpectation(ctx context.Context, id ids.ID) (app.Expectation, bool)
	// RelayEnd ends a session at a limit of the relay with its unwritten counts (M2-46).
	RelayEnd(ctx context.Context, id ids.ID, reason domain.CloseReason, p domain.Progress) error
}

// Compile-time check: the use cases of the module.
var _ RelayUseCases = (*app.Service)(nil)

// RelayConfig holds the configuration keys of the relay.
type RelayConfig struct {
	// BufferMaxBytes is terminal.relay_buffer_max_bytes: the bytes of queued messages per
	// direction and session.
	BufferMaxBytes int
	// OpenTimeout is terminal.open_timeout: the new deadline after a pairing that ended before
	// session.started on a host with local confirmation, and the lone timer of a side (M2-47
	// Design note 8).
	OpenTimeout time.Duration
	// Multi is cluster.mode multi (M2-47): session.started arrives only on the node of the
	// control connection, so a node without the pairing never detaches on it, and a pairing of a
	// session waiting for local confirmation is announced with terminal.session_paired.
	Multi bool
	// Windows are the limits of a session the relay enforces without the host service (M2-46
	// Design note 7): terminal.idle_timeout plus terminal.idle_grace without device bytes,
	// terminal.max_session_duration from opened_at and terminal.detach_grace in detached; the
	// progress of a held session is written at most once per terminal.idle_grace. A zero Idle
	// switches the limits off (tests of M2-44 that build a relay without them).
	Windows domain.Windows
	// Metrics records bothy.terminal.sessions.active (M2-46); nil records nothing.
	Metrics app.SessionMetrics
}

// The log messages and keys of the relay: length, time, side, close code and session ID only
// (SEC-TERM-1, ruling D-M2-44-5); never frame content, a token, an address or a name.
const (
	msgSideClosed   = "terminal relay connection closed"
	msgSideUnpaired = "terminal relay side unpaired"
	msgRelayFailed  = "terminal relay callback failed"
	logSessionID    = "bothy.terminal.session_id"
	logFrames       = "bothy.terminal.frames"
	logBytes        = "bothy.terminal.bytes"
	logDurationMS   = "bothy.terminal.duration_ms"
	eventSessionChg = "terminal_session.changed"
	errorRelay      = "relay"
)

// Relay is the in-memory relay of one node (Design note 7 of M2-44, docs/spec/24-terminal.md
// sections 5.4, 6.1 to 6.3 and 9): one slot per session with the expectation of its last grant,
// its two sides, one bounded queue per direction and its deadline. domain.Decide says what an
// event does; the relay executes the closes and calls the use cases for the state changes after it
// released its lock. It implements app.RelayControl. It is safe for concurrent use.
type Relay struct {
	cfg    RelayConfig
	uc     RelayUseCases
	links  app.ControlLinks
	timers app.Timers
	clk    clock.Clock
	log    *slog.Logger

	// send delivers terminal.session_paired to the other nodes (app.Service.SendSignal, M2-47);
	// nil sends nothing.
	send func(ctx context.Context, s domain.Signal) error

	mu      gosync.Mutex
	slots   map[ids.ID]*slot
	sides   map[*side]bool
	closing bool
	// idle is signalled when the last side ends (the stop step waits for it).
	idle *gosync.Cond
	// writes counts the progress writes in flight, each until its counts are back after a failure.
	writes gosync.WaitGroup
	// settled is signalled when a progress write of a slot finished (slot.pending, limitDeadline).
	settled *gosync.Cond
}

// Compile-time check: the outgoing port of the use cases.
var _ app.RelayControl = (*Relay)(nil)

// NewRelay returns an empty relay. links receives session.close at the end of a session; timers
// nil means app.RealTimers, clk nil clock.System(), log nil slog.Default(). Errors: a limit below 1
// or missing use cases.
func NewRelay(cfg RelayConfig, uc RelayUseCases, links app.ControlLinks, timers app.Timers, clk clock.Clock, log *slog.Logger) (*Relay, error) {
	if cfg.BufferMaxBytes < 1 || cfg.OpenTimeout <= 0 {
		return nil, errors.New("wsrelay: terminal.relay_buffer_max_bytes and terminal.open_timeout must be positive")
	}
	if uc == nil {
		return nil, errors.New("wsrelay: NewRelay needs the use cases")
	}
	if links == nil {
		links = app.NoLinks{}
	}
	if timers == nil {
		timers = app.RealTimers{}
	}
	if clk == nil {
		clk = clock.System()
	}
	if log == nil {
		log = slog.Default()
	}
	if cfg.Metrics == nil {
		cfg.Metrics = app.NoSessionMetrics{}
	}
	w := cfg.Windows
	if w.Idle > 0 && (w.IdleGrace <= 0 || w.MaxDuration <= 0 || w.DetachGrace <= 0) {
		return nil, errors.New("wsrelay: terminal.idle_grace, terminal.max_session_duration and terminal.detach_grace must be positive")
	}
	r := &Relay{cfg: cfg, uc: uc, links: links, timers: timers, clk: clk, log: log, slots: map[ids.ID]*slot{}, sides: map[*side]bool{}}
	r.idle = gosync.NewCond(&r.mu)
	r.settled = gosync.NewCond(&r.mu)
	return r, nil
}

// slot is the state of one session on this node.
type slot struct {
	id, host, device ids.ID
	started, confirm bool
	// hostEnding is set by HostEnding: the control connection of the host closes with a state
	// change, so the loss of the host side detaches nothing.
	hostEnding bool
	wdi        *int
	// gen counts the grants; a deadline of an older grant does nothing.
	gen int
	// grant is the grant of the expectation (M2-47): the same grant again changes nothing.
	grant    ids.ID
	stop     func() bool
	dev, hst *side
	paired   bool
	toDev    queue
	toHost   queue
	// pairedOnce marks a pairing since the grant on this node; pairedElsewhere a
	// terminal.session_paired of another node since the grant; overdue a deadline that passed
	// while an unpaired side was present (its lone timer decides, M2-47 Design note 8).
	pairedOnce, pairedElsewhere, overdue bool
	// in and out are the transport bytes of the pairings since the last take; lastInput the time
	// of the last device frame; seenDev and seenHost mark the handshake message of each direction
	// since the grant.
	in, out           int64
	lastInput         time.Time
	seenDev, seenHost bool
	// times are the times of the row of the last grant, kept current from the log tail; lastDev
	// is the last binary message from the device seen by this node, never cleared (M2-46 Design
	// note 7). limitStop and limitGen are the timer of the limits; persisted the last write of the
	// progress; storedInput the last input time written; counted the state this slot counts in
	// bothy.terminal.sessions.active ("" none).
	times       domain.SessionTimes
	lastDev     time.Time
	limitStop   func() bool
	limitGen    int
	persisted   time.Time
	storedInput time.Time
	counted     string
	// flushStop and flushGen are the flush timer of the progress (persist).
	flushStop func() bool
	flushGen  int
	// held marks a slot that held a side on this node since its grant: only such a node runs the
	// limits (Design note 7: "one set per session the process holds a side of"). With several
	// nodes every node expects the session, but only the node of the pairing sees the device's
	// bytes; a new grant clears the mark there, because the next pairing may live elsewhere.
	held bool
	// limitEnded marks a slot whose limit timer ended the session on this node: a later row of
	// the log tail (the active row of a reattach before the closed row) must not start it again,
	// because a second end would carry no counts. A new grant clears it.
	limitEnded bool
	// pending counts the progress writes of this slot in flight (taken from the slot, not yet
	// stored or given back); ending stops new progress writes while the limit end waits for them
	// (D-M2-46-24).
	pending int
	ending  bool
}

// queue is one direction of a slot, bounded by terminal.relay_buffer_max_bytes.
type queue struct {
	msgs  [][]byte
	bytes int
}

// state returns the facts of domain.Decide.
func (s *slot) state() domain.SlotState {
	return domain.SlotState{Started: s.started, LocalConfirm: s.confirm, DevicePresent: s.dev != nil, HostPresent: s.hst != nil, Paired: s.paired,
		HostEnding: s.hostEnding}
}

// side is one connection of a slot: the device stream or the data connection.
type side struct {
	kind      domain.Side
	sessionID ids.ID
	ws        *websocket.Conn
	wake      chan struct{}
	done      chan struct{}
	once      gosync.Once
	opened    time.Time
	frames    int64
	bytes     int64
	// lone stops the lone timer of the side (M2-47 Design note 8); paired marks a side that was
	// paired once and never counts as unpaired.
	lone   func() bool
	paired bool
}

// close closes the connection once with code and reason, in its own goroutine, so no caller waits
// on the network.
func (sd *side) close(code int, reason string) bool {
	closed := false
	sd.once.Do(func() {
		closed = true
		close(sd.done)
		go func() { _ = sd.ws.Close(websocket.StatusCode(code), reason) }()
	})
	return closed
}

// effects are the calls the relay makes after it released its lock.
type effects []func(ctx context.Context)

// run calls every effect in order.
func (e effects) run(ctx context.Context) {
	for _, f := range e {
		f(ctx)
	}
}

// Expect implements app.RelayControl: a new grant (Design note 7). An existing slot gets the
// event new grant (older device side 4103, older host side 1000, a started session whose pairing
// ends detaches) and its counts are written; then the slot carries the new expectation with a
// fresh deadline and empty queues.
func (r *Relay) Expect(e app.Expectation) {
	ctx := context.Background()
	r.mu.Lock()
	s := r.slots[e.SessionID]
	if s != nil && !e.Grant.IsZero() && s.grant == e.Grant {
		// The same grant again (a side built it from the row first, or a signal twice).
		r.mu.Unlock()
		return
	}
	var fx effects
	if s == nil {
		s = &slot{id: e.SessionID}
		r.slots[e.SessionID] = s
	} else {
		fx = r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvNewGrant}))
	}
	if s.stop != nil {
		s.stop()
	}
	s.host, s.device, s.started, s.confirm, s.wdi, s.grant = e.HostID, e.DeviceID, e.Started, e.LocalConfirm, e.WorkdirIndex, e.Grant
	s.toDev, s.toHost, s.seenDev, s.seenHost, s.paired = queue{}, queue{}, false, false, false
	s.pairedOnce, s.pairedElsewhere, s.overdue, s.limitEnded = false, false, false, false
	if r.cfg.Multi {
		s.held = false
		s.stopLimit()
	}
	s.gen++
	r.arm(s, max(e.Deadline.Sub(r.clk.Now()), 0))
	if !e.Times.OpenedAt.IsZero() {
		s.times = e.Times
	}
	r.armLimit(s)
	r.mu.Unlock()
	fx.run(ctx)
}

// armLimit (re)starts the timer of the limits of s at the earliest of its deadlines
// (domain.RelayDeadline); r.mu is held. Without configured windows or without the times of a row
// nothing runs.
func (r *Relay) armLimit(s *slot) {
	if r.cfg.Windows.Idle <= 0 || s.times.OpenedAt.IsZero() || !s.held || s.limitEnded {
		return
	}
	if s.limitStop != nil {
		s.limitStop()
	}
	s.limitGen++
	id, gen := s.id, s.limitGen
	at, _ := domain.RelayDeadline(s.times, s.lastDev, r.cfg.Windows)
	s.limitStop = r.timers.AfterFunc(max(at.Sub(r.clk.Now()), 0), func() { r.limitDeadline(id, gen) })
}

// limitDeadline is the timer of the limits of the session id (M2-46 Design note 7): when the
// earliest deadline is reached (the instant itself ends), the session ends through
// RelayUseCases.RelayEnd with the reason and the counts of this node; when device input moved the
// idle deadline meanwhile, the timer starts again for the new deadline. The connections close
// after the commit through the log tail (4100 with the reason).
func (r *Relay) limitDeadline(id ids.ID, gen int) {
	r.mu.Lock()
	var marked *slot
	for {
		s := r.slots[id]
		if s == nil || s.limitGen != gen || !s.held {
			unmark(marked)
			r.mu.Unlock()
			return
		}
		at, reason := domain.RelayDeadline(s.times, s.lastDev, r.cfg.Windows)
		if !domain.Reached(at, r.clk.Now()) {
			unmark(marked)
			r.armLimit(s)
			r.mu.Unlock()
			return
		}
		if s.pending > 0 {
			// A progress write of this session is in flight: it must reach the row before the end
			// commits, or it would find the session closed and its counts would be lost; a failed
			// one gives its counts back for the end (D-M2-46-24).
			s.ending, marked = true, s
			r.settled.Wait()
			continue
		}
		c := s.take()
		s.limitGen++
		s.limitEnded = true
		r.mu.Unlock()
		ctx := context.Background()
		if err := r.uc.RelayEnd(ctx, id, reason, domain.Progress{BytesIn: c.In, BytesOut: c.Out, LastInputAt: c.LastInput}); err != nil {
			r.endFailed(ctx, s, c, err)
		}
		return
	}
}

// unmark clears the wait mark of D-M2-46-24 on s, which a limit end set while it waited; s is nil
// when the end did not wait. r.mu is held.
func unmark(s *slot) {
	if s != nil {
		s.ending = false
	}
}

// endFailed handles a failed RelayEnd of a limit end of s with the counts c it took: the session
// stays open, so the counts go back to the slot and the progress writes go on, and last_input_at
// keeps moving; terminal.sweep repeats the end (D-M2-46-25).
func (r *Relay) endFailed(ctx context.Context, s *slot, c app.Counts, err error) {
	r.call(ctx, err)
	r.mu.Lock()
	s.ending = false
	fx := r.giveBackLocked(s.id, c)
	r.mu.Unlock()
	fx.run(ctx)
}

// startWrite starts a progress write of the counts c of s, which persist or apply took from the
// slot: the write is pending on s until wrote. r.mu is held.
func (r *Relay) startWrite(s *slot, c app.Counts) func(context.Context) {
	s.pending++
	id := s.id
	return func(ctx context.Context) {
		err := r.uc.RelayCounters(ctx, id, c)
		if err != nil {
			r.call(ctx, err)
		}
		r.wrote(s, c, err)
	}
}

// wrote ends a progress write of s started by startWrite: a failed write gives its counts back to
// the slot of the session (and lets persist schedule the retry), then the write is no longer
// pending and a waiting limit end goes on.
func (r *Relay) wrote(s *slot, c app.Counts, err error) {
	var fx effects
	r.mu.Lock()
	if err != nil {
		fx = r.giveBackLocked(s.id, c)
	}
	s.pending--
	r.settled.Broadcast()
	r.mu.Unlock()
	fx.run(context.Background())
}

// hold marks s as held by this node and starts its limits, once per grant; r.mu is held.
func (r *Relay) hold(s *slot) {
	if s.held {
		return
	}
	s.held = true
	r.armLimit(s)
}

// countState moves the state s counts in bothy.terminal.sessions.active to state ("" stops
// counting it); r.mu is held.
func (r *Relay) countState(s *slot, state string) {
	if s.counted == state {
		return
	}
	ctx := context.Background()
	if s.counted != "" {
		r.cfg.Metrics.SessionsActive(ctx, s.counted, -1)
	}
	if state != "" {
		r.cfg.Metrics.SessionsActive(ctx, state, 1)
	}
	s.counted = state
}

// heldState is the state a slot counts once it holds a side: the state of its row, opening when
// none is known yet.
func (s *slot) heldState() string {
	switch st := s.times.State; st {
	case domain.SessionOpening, domain.SessionActive, domain.SessionDetached:
		return st
	}
	if s.started {
		return domain.SessionActive
	}
	return domain.SessionOpening
}

// arm starts the deadline of s after d; r.mu is held.
func (r *Relay) arm(s *slot, d time.Duration) {
	id, gen := s.id, s.gen
	s.stop = r.timers.AfterFunc(d, func() { r.deadline(id, gen) })
}

// deadline is the deadline of the grant gen of the session id (M2-44 Design note 7, refined by
// M2-47 Design notes 3 and 8): while an unpaired side is present its lone timer decides; a slot
// that never saw session.started asks the database, because session.started arrives only on the
// node of the control connection; then a session that started elsewhere is met, an expectation
// without sides of a session waiting for local confirmation that paired elsewhere removes itself,
// and everything else follows the table of M2-44.
func (r *Relay) deadline(id ids.ID, gen int) {
	ctx := context.Background()
	r.mu.Lock()
	s := r.slots[id]
	if s == nil || s.gen != gen {
		r.mu.Unlock()
		return
	}
	if s.lonePresent() {
		s.overdue = true
		r.mu.Unlock()
		return
	}
	started := s.started
	r.mu.Unlock()
	state := domain.SessionOpening
	if !started {
		st, found, err := r.uc.RelaySessionState(ctx, id)
		switch {
		case err != nil:
			r.call(ctx, err)
		case found:
			state = st
		}
	}
	r.mu.Lock()
	s = r.slots[id]
	if s == nil || s.gen != gen {
		r.mu.Unlock()
		return
	}
	fx := r.expire(s, started || state != domain.SessionOpening, state)
	r.mu.Unlock()
	fx.run(ctx)
}

// expire applies the deadline of s; started says whether the session started (here or on
// another node), state is the state read; r.mu is held.
func (r *Relay) expire(s *slot, started bool, state string) effects {
	if started && !s.started {
		// Started on another node (Design note 3 of M2-47).
		return r.markStarted(s, state)
	}
	if s.lonePresent() {
		s.overdue = true
		return nil
	}
	if !started && s.dev == nil && s.hst == nil && s.confirm && s.pairedElsewhere {
		// The pairing on another node met the opening deadline there; the host service bounds
		// start and consent itself (ruling D-M2-47-3).
		s.stopDeadline()
		r.forget(s)
		return nil
	}
	return r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvDeadline}))
}

// lonePresent reports a present side without its counterpart.
func (s *slot) lonePresent() bool { return !s.paired && (s.dev != nil) != (s.hst != nil) }

// Counters implements app.RelayControl: it takes the live counts of the session's pairings, so the
// end path writes them once and the relay never writes them again.
func (r *Relay) Counters(sessionID ids.ID) app.Counts {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.slots[sessionID]
	if s == nil {
		return app.Counts{}
	}
	return s.take()
}

// take returns and clears the counts of s.
func (s *slot) take() app.Counts {
	c := app.Counts{In: s.in, Out: s.out, LastInput: s.lastInput}
	s.in, s.out, s.lastInput = 0, 0, time.Time{}
	return c
}

// OpeningWorkdir implements app.RelayControl: the working directory index of an opening of a
// claude profile on this node.
func (r *Relay) OpeningWorkdir(sessionID ids.ID) (*int, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.slots[sessionID]
	if s == nil || s.started || s.wdi == nil {
		return nil, false
	}
	w := *s.wdi
	return &w, true
}

// apply executes the actions of Decide on s; r.mu is held. Closes happen now (in their own
// goroutines); the calls into the use cases are returned as effects. When a pairing ends for
// another reason than the end of the session, its counts are written.
func (r *Relay) apply(s *slot, acts []domain.RelayAction) effects {
	var fx effects
	wasPaired, dropped := s.paired, false
	for _, a := range acts {
		if f := r.applyOne(s, a); f != nil {
			fx = append(fx, f)
		}
		dropped = dropped || a.Kind == domain.ActDrop
	}
	if s.dev == nil || s.hst == nil {
		s.paired = false
	}
	if wasPaired && !s.paired && !dropped {
		fx = append([]func(context.Context){r.startWrite(s, s.take())}, fx...)
	}
	if s.dev == nil && s.hst == nil && !dropped {
		// A session without a connection on this node is not held here (M2-46 Design note 12 b).
		r.countState(s, "")
	}
	return fx
}

// applyOne executes one action on s; r.mu is held. It returns the call into the use cases, or nil.
func (r *Relay) applyOne(s *slot, a domain.RelayAction) func(context.Context) {
	id := s.id
	switch a.Kind {
	case domain.ActCloseDevice:
		r.closeSide(s, s.dev, a.Code, a.Reason)
	case domain.ActCloseHost:
		r.closeSide(s, s.hst, a.Code, a.Reason)
	case domain.ActActivate:
		// The detach limit stops now on this node, the mirror of ActDetach; the committed row of
		// the log tail replaces the local state (red team M2-46, B1).
		if s.times.State == domain.SessionDetached {
			s.times.State, s.times.DetachedAt = domain.SessionActive, time.Time{}
			r.armLimit(s)
		}
		return func(ctx context.Context) { r.call(ctx, r.uc.RelayActivate(ctx, id)) }
	case domain.ActDetach:
		// The detach limit starts now on this node; the committed detached_at of the log tail
		// replaces the instant (M2-46 Design note 7 b).
		if s.times.State == domain.SessionActive {
			s.times.State, s.times.DetachedAt = domain.SessionDetached, r.clk.Now()
			r.armLimit(s)
		}
		return func(ctx context.Context) { r.call(ctx, r.uc.RelayDetach(ctx, id)) }
	case domain.ActEndOpenFailed:
		detail := a.Detail
		if detail == domain.DetailOpenTimeout {
			// The end of an opening at its deadline is a compare-and-swap on opening: a second node
			// that times out the same session writes nothing (M2-47 Design note 8 c).
			return func(ctx context.Context) { r.call(ctx, r.uc.RelayOpeningTimedOut(ctx, id)) }
		}
		return func(ctx context.Context) { r.call(ctx, r.uc.RelayOpenFailed(ctx, id, detail)) }
	case domain.ActMeetDeadline:
		// A new generation: a deadline already waiting for the lock does nothing.
		s.gen++
		s.stopDeadline()
	case domain.ActRearmDeadline:
		s.gen++
		s.stopDeadline()
		r.arm(s, r.cfg.OpenTimeout)
	case domain.ActCountUnpaired:
		side := a.Side
		return func(ctx context.Context) { r.uc.RelayUnpaired(ctx, side) }
	case domain.ActDrop:
		s.stopDeadline()
		r.forget(s)
	}
	return nil
}

// stopLimit stops the timer of the limits of s, if any; r.mu is held.
func (s *slot) stopLimit() {
	if s.limitStop != nil {
		s.limitStop()
	}
	s.limitGen++
}

// forget removes the slot s: its timers stop and it no longer counts as held; r.mu is held.
func (r *Relay) forget(s *slot) {
	s.stopLimit()
	s.stopFlush()
	r.countState(s, "")
	delete(r.slots, s.id)
}

// stopDeadline stops the running deadline of s, if any.
func (s *slot) stopDeadline() {
	if s.stop != nil {
		s.stop()
	}
}

// closeSide removes sd from s and closes it; nil does nothing.
func (r *Relay) closeSide(s *slot, sd *side, code int, reason string) {
	if sd == nil {
		return
	}
	if s.dev == sd {
		s.dev, s.toDev = nil, queue{}
	}
	if s.hst == sd {
		s.hst, s.toHost = nil, queue{}
	}
	if sd.close(code, reason) {
		r.logClose(sd, code)
	}
}

// logClose writes the one record of a side at its end: session ID, side, frames, bytes, duration
// and close code; never content.
func (r *Relay) logClose(sd *side, code int) {
	r.log.LogAttrs(context.Background(), slog.LevelInfo, msgSideClosed, slog.String(logSessionID, sd.sessionID.String()),
		slog.String(attr.TerminalSide, string(sd.kind)), slog.Int64(logFrames, sd.frames), slog.Int64(logBytes, sd.bytes),
		slog.Int64(logDurationMS, r.clk.Now().Sub(sd.opened).Milliseconds()), slog.Int(attr.CloseCode, code))
}

// call logs a failed callback with its type only.
func (r *Relay) call(ctx context.Context, err error) {
	if err != nil {
		r.log.LogAttrs(ctx, slog.LevelError, msgRelayFailed, slog.String(attr.ErrorType, errorRelay))
	}
}

// newSide registers a connection of kind for the session; r.mu is held.
func (r *Relay) newSide(kind domain.Side, sessionID ids.ID, ws *websocket.Conn) *side {
	sd := &side{kind: kind, sessionID: sessionID, ws: ws, wake: make(chan struct{}, 1), done: make(chan struct{}), opened: r.clk.Now()}
	r.sides[sd] = true
	return sd
}

// attachDevice makes ws the device side of the session after an allowed identify of deviceID. It
// returns the side, or the close code for the connection: 4013 while the process stops, 4102
// without an expectation on this node (no counterpart here), 4103 for a device that is not the
// one of the current grant. A device side already present is replaced (4103 for the older).
func (r *Relay) attachDevice(sessionID, deviceID ids.ID, ws *websocket.Conn) (*side, int) {
	r.ensureSlot(sessionID)
	r.mu.Lock()
	s := r.slots[sessionID]
	switch {
	case r.closing:
		r.mu.Unlock()
		return nil, int(domain.StreamServerRestart)
	case s == nil:
		r.mu.Unlock()
		return nil, int(domain.StreamHostUnavailable)
	case s.device != deviceID:
		r.mu.Unlock()
		return nil, int(domain.StreamReplaced)
	}
	var fx effects
	if s.dev != nil {
		fx = r.apply(s, []domain.RelayAction{{Kind: domain.ActCloseDevice, Code: int(domain.StreamReplaced)}})
	}
	sd := r.newSide(domain.SideClient, sessionID, ws)
	s.dev = sd
	if s.counted == "" {
		r.countState(s, s.heldState())
	}
	r.hold(s)
	fx = append(fx, r.pairIfBoth(s)...)
	r.armLone(s, sd)
	r.mu.Unlock()
	r.wakeSide(sd)
	fx.run(context.Background())
	return sd, 0
}

// attachHost makes ws the host side of the session after an allowed identify of the host service
// of hostID (Design note 9). It returns the side, or the close code: 4013 while the process stops,
// 1000 without an expectation on this node or while a paired host side exists (the host service
// waits for session.open), 4100 for a session of another host. An unpaired older host side is
// replaced (1000 for the older).
func (r *Relay) attachHost(sessionID, hostID ids.ID, ws *websocket.Conn) (*side, int) {
	r.ensureSlot(sessionID)
	r.mu.Lock()
	s := r.slots[sessionID]
	switch {
	case r.closing:
		r.mu.Unlock()
		return nil, int(domain.StreamServerRestart)
	case s == nil:
		r.mu.Unlock()
		return nil, int(domain.StreamNormal)
	case s.host != hostID:
		r.mu.Unlock()
		return nil, int(domain.StreamTerminalClosed)
	case s.hst != nil && s.paired:
		r.mu.Unlock()
		return nil, int(domain.StreamNormal)
	}
	var fx effects
	if s.hst != nil {
		fx = r.apply(s, []domain.RelayAction{{Kind: domain.ActCloseHost, Code: int(domain.StreamNormal)}})
	}
	sd := r.newSide(domain.SideHost, sessionID, ws)
	s.hst = sd
	if s.counted == "" {
		r.countState(s, s.heldState())
	}
	r.hold(s)
	fx = append(fx, r.pairIfBoth(s)...)
	r.armLone(s, sd)
	r.mu.Unlock()
	r.wakeSide(sd)
	fx.run(context.Background())
	return sd, 0
}

// ensureSlot applies the expectation of the current grant of a session, read from its row, when a
// side arrives (M2-47): with several nodes terminal.session_open and the side race, and the
// database is the source of the grant; Expect ignores the grant the slot already holds. With one
// node the expectation always exists before the side.
func (r *Relay) ensureSlot(sessionID ids.ID) {
	if !r.cfg.Multi {
		return
	}
	r.mu.Lock()
	closing := r.closing
	r.mu.Unlock()
	if closing {
		return
	}
	if e, ok := r.uc.RelayExpectation(context.Background(), sessionID); ok {
		r.Expect(e)
	}
}

// pairIfBoth pairs the two sides of s when both are present; r.mu is held. The lone timers of
// both sides stop; with several nodes the pairing of a session waiting for local confirmation is
// announced with terminal.session_paired (M2-47 Design note 3).
func (r *Relay) pairIfBoth(s *slot) effects {
	if s.dev == nil || s.hst == nil || s.paired {
		return nil
	}
	s.paired, s.pairedOnce, s.overdue = true, true, false
	for _, sd := range []*side{s.dev, s.hst} {
		sd.paired = true
		if sd.lone != nil {
			sd.lone()
		}
	}
	r.wakeLocked(s.dev)
	r.wakeLocked(s.hst)
	fx := r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvPaired}))
	if r.cfg.Multi && r.send != nil && !s.started && s.confirm {
		sig := domain.Signal{Kind: domain.SignalSessionPaired, HostID: s.host, SessionID: s.id}
		fx = append(fx, func(ctx context.Context) { r.call(ctx, r.send(ctx, sig)) })
	}
	return fx
}

// armLone starts the lone timer of sd when it is not paired (M2-47 Design note 8 e): from the
// acceptance of its identify, terminal.open_timeout; a host data connection of a started session
// waits without one (Design note 8 d). r.mu is held.
func (r *Relay) armLone(s *slot, sd *side) {
	if sd.paired || !domain.LoneSideOutcome(sd.kind, s.phase()).Timer {
		return
	}
	sd.lone = r.timers.AfterFunc(r.cfg.OpenTimeout, func() { r.loneDeadline(sd) })
}

// phase is the state of the session as this node knows it for the rules of the lone side.
func (s *slot) phase() string {
	if s.started {
		return domain.SessionDetached
	}
	return domain.SessionOpening
}

// loneDeadline is the lone timer of sd (M2-47 Design note 8): a side that is still current and
// was never paired closes with 4102, counts bothy.terminal.relay.unpaired with its side and logs
// one record; a session still in opening then ends open_failed with open_timeout through the
// compare-and-swap. Its frames never left the node.
func (r *Relay) loneDeadline(sd *side) {
	r.mu.Lock()
	s, ok := r.current(sd)
	if !ok || sd.paired {
		r.mu.Unlock()
		return
	}
	out := domain.LoneSideOutcome(sd.kind, s.phase())
	if !out.Timer {
		// The session started meanwhile: a host data connection waits for an attaching device.
		r.mu.Unlock()
		return
	}
	r.closeSide(s, sd, out.Code, "")
	id := s.id
	fx := effects{func(ctx context.Context) { r.uc.RelayUnpaired(ctx, out.Side) }}
	if out.End {
		fx = append(fx, func(ctx context.Context) { r.call(ctx, r.uc.RelayOpeningTimedOut(ctx, id)) })
	}
	r.mu.Unlock()
	r.log.LogAttrs(context.Background(), slog.LevelWarn, msgSideUnpaired, slog.String(attr.TerminalSide, string(out.Side)),
		slog.Int(attr.CloseCode, out.Code))
	fx.run(context.Background())
}

// wakeSide and wakeLocked signal the writer of sd.
func (r *Relay) wakeSide(sd *side) { r.wakeLocked(sd) }

// wakeLocked signals the writer of sd without blocking.
func (r *Relay) wakeLocked(sd *side) {
	if sd == nil {
		return
	}
	select {
	case sd.wake <- struct{}{}:
	default:
	}
}

// current reports whether sd is still a side of its slot; r.mu is held.
func (r *Relay) current(sd *side) (*slot, bool) {
	s := r.slots[sd.sessionID]
	if s == nil || (s.dev != sd && s.hst != sd) {
		return nil, false
	}
	return s, true
}

// forward queues one Noise message from sd toward the other side, unchanged (Design note 7): the
// first message of each direction since the grant is the handshake and is not counted; every later
// message adds its length to bytes_in (from the device) or bytes_out (to the device), and a device
// message sets the time of the last device frame. A message that does not fit into the queue of
// its direction closes the receiver with 4014.
func (r *Relay) forward(sd *side, msg []byte) {
	r.mu.Lock()
	s, ok := r.current(sd)
	if !ok {
		r.mu.Unlock()
		return
	}
	sd.frames++
	sd.bytes += int64(len(msg))
	q, receiver, to := &s.toHost, domain.SideHost, s.hst
	if sd.kind == domain.SideHost {
		q, receiver, to = &s.toDev, domain.SideClient, s.dev
	}
	if q.bytes+len(msg) > r.cfg.BufferMaxBytes {
		fx := r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvQueueFull, Side: receiver}))
		r.mu.Unlock()
		fx.run(context.Background())
		return
	}
	r.count(s, sd.kind, len(msg))
	q.msgs = append(q.msgs, msg)
	q.bytes += len(msg)
	r.wakeLocked(to)
	fx := r.persist(s)
	r.mu.Unlock()
	fx.run(context.Background())
}

// persist is the one place that decides the progress write of s (M2-46 Design note 7 d,
// D-M2-46-7 and D-M2-46-22): the counts since the last write are added once and last_input_at moves
// forward, without an event and without a new version. It writes when terminal.idle_grace passed
// since the last attempt (domain.PersistDue) and also at once when unwritten input is a full grace
// newer than the last input sent, so an output-only write never holds input back. Otherwise, while
// unwritten input waits, a flush timer writes it at the last attempt plus the grace, so input reaches
// the store within one grace even when no further frame arrives. A failed write gives its counts
// back and comes through here again, so it is retried at most once per grace. Together: the stored
// last_input_at lags the true one by less than the grace while the store works, and the relay
// writes at most twice per grace. r.mu is held.
func (r *Relay) persist(s *slot) effects {
	if r.cfg.Windows.Idle <= 0 || r.closing || s.ending || (s.in == 0 && s.out == 0 && s.lastInput.IsZero()) {
		return nil
	}
	now, grace := r.clk.Now(), r.cfg.Windows.IdleGrace
	inputDue := !s.lastInput.IsZero() && s.lastInput.Sub(s.storedInput) >= grace
	if !inputDue && !domain.PersistDue(s.persisted, now, grace) {
		if !s.lastInput.IsZero() && s.flushStop == nil {
			s.flushGen++
			id, gen := s.id, s.flushGen
			s.flushStop = r.timers.AfterFunc(s.persisted.Add(grace).Sub(now), func() { r.flush(id, gen) })
		}
		return nil
	}
	s.stopFlush()
	s.persisted = now
	c := s.take()
	if !c.LastInput.IsZero() {
		s.storedInput = c.LastInput
	}
	write := r.startWrite(s, c)
	r.writes.Add(1)
	return effects{func(ctx context.Context) {
		go func() {
			defer r.writes.Done()
			write(ctx)
		}()
	}}
}

// flush is the flush timer of the progress of the session id (persist).
func (r *Relay) flush(id ids.ID, gen int) {
	r.mu.Lock()
	s := r.slots[id]
	if s == nil || s.flushGen != gen {
		r.mu.Unlock()
		return
	}
	s.flushStop = nil
	fx := r.persist(s)
	r.mu.Unlock()
	fx.run(context.Background())
}

// stopFlush stops the flush timer of s, if any; r.mu is held.
func (s *slot) stopFlush() {
	if s.flushStop != nil {
		s.flushStop()
		s.flushStop = nil
	}
	s.flushGen++
}

// giveBackLocked adds counts of a failed write back to the slot of the session, if it still exists, and
// lets persist schedule the retry; r.mu is held.
func (r *Relay) giveBackLocked(id ids.ID, c app.Counts) effects {
	s := r.slots[id]
	if s == nil {
		return nil
	}
	s.in += c.In
	s.out += c.Out
	if c.LastInput.After(s.lastInput) {
		s.lastInput = c.LastInput
	}
	return r.persist(s)
}

// count adds a message of n bytes from kind to the counters of s; r.mu is held.
func (r *Relay) count(s *slot, kind domain.Side, n int) {
	if kind == domain.SideClient {
		// Every binary message of the device moves the idle reference of this node, the handshake
		// included (24 section 6.3: only device bytes count).
		s.lastDev = r.clk.Now()
		if !s.seenDev {
			s.seenDev = true
			return
		}
		s.in += int64(n)
		s.lastInput = r.clk.Now()
		return
	}
	if !s.seenHost {
		s.seenHost = true
		return
	}
	s.out += int64(n)
}

// pop returns the next message toward sd; nil when there is none or sd is no longer current.
func (r *Relay) pop(sd *side) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.current(sd)
	if !ok || !s.paired {
		return nil
	}
	q := &s.toHost
	if sd.kind == domain.SideClient {
		q = &s.toDev
	}
	if len(q.msgs) == 0 {
		return nil
	}
	m := q.msgs[0]
	q.msgs[0] = nil
	q.msgs = q.msgs[1:]
	q.bytes -= len(m)
	return m
}

// write is the writer of sd: the queued messages toward it, each as one binary message, until sd
// ends. A failed write ends sd like a close by the peer.
func (r *Relay) write(ctx context.Context, sd *side) {
	for {
		select {
		case <-sd.done:
			return
		case <-sd.wake:
		}
		for {
			m := r.pop(sd)
			if m == nil {
				break
			}
			if err := sd.ws.Write(ctx, websocket.MessageBinary, m); err != nil {
				return
			}
		}
	}
}

// closed reports that the reader of sd ended: by the peer with code and reason, or for any other
// reason (code 0). A side that is no longer current changes nothing.
func (r *Relay) closed(sd *side, code int, reason string) {
	r.mu.Lock()
	var fx effects
	if s, ok := r.current(sd); ok {
		ev := domain.RelayEvent{Kind: domain.EvDeviceClosed}
		if sd.kind == domain.SideHost {
			ev = domain.RelayEvent{Kind: domain.EvHostClosed, Code: code, Reason: reason}
		}
		if sd.kind == domain.SideClient {
			s.dev, s.toDev = nil, queue{}
		} else {
			s.hst, s.toHost = nil, queue{}
		}
		was := s.state()
		was.Paired = s.paired
		if sd.kind == domain.SideClient {
			was.DevicePresent = false
		} else {
			was.HostPresent = false
		}
		fx = r.apply(s, domain.Decide(was, ev))
		if sd.close(code, reason) {
			r.logClose(sd, code)
		}
		if sd.lone != nil {
			sd.lone()
		}
		fx = append(fx, r.afterOverdue(s)...)
	}
	r.release(sd)
	r.mu.Unlock()
	fx.run(context.Background())
}

// afterOverdue applies a deadline that passed while a lone side was present, once the slot has
// no side left (M2-47 Design note 8); r.mu is held.
func (r *Relay) afterOverdue(s *slot) effects {
	if !s.overdue || s.dev != nil || s.hst != nil || r.slots[s.id] != s {
		return nil
	}
	s.overdue = false
	if s.started {
		return nil
	}
	id := s.id
	return effects{func(ctx context.Context) { r.call(ctx, r.uc.RelayOpeningTimedOut(ctx, id)) }}
}

// protocolError closes sd for a text frame after identify or a message above the Noise limit
// (4002) and the other side as the table says.
func (r *Relay) protocolError(sd *side) {
	r.mu.Lock()
	var fx effects
	if s, ok := r.current(sd); ok {
		fx = r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvProtocolError, Side: sd.kind}))
	} else if sd.close(int(domain.StreamDecodeError), "") {
		r.logClose(sd, int(domain.StreamDecodeError))
	}
	r.mu.Unlock()
	fx.run(context.Background())
}

// release forgets sd once its reader ended; r.mu is held.
func (r *Relay) release(sd *side) {
	delete(r.sides, sd)
	if len(r.sides) == 0 {
		r.idle.Broadcast()
	}
}

// Started is the hook after M2-43's SessionStarted applied session.started of the session
// (Design note 10): the slot is started, its deadline is met, and without a paired device the
// session detaches at once.
func (r *Relay) Started(sessionID ids.ID) {
	r.mu.Lock()
	s := r.slots[sessionID]
	if s == nil {
		r.mu.Unlock()
		return
	}
	if r.cfg.Multi && !s.paired && !s.pairedOnce {
		// The pairing may live on another node (M2-47): the deadline is met, nothing detaches here.
		s.started = true
		s.gen++
		s.stopDeadline()
		r.mu.Unlock()
		return
	}
	fx := r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvStarted}))
	s.started = true
	r.mu.Unlock()
	fx.run(context.Background())
}

// HostEnding marks every slot of hostID on this node before the control connection closes with
// 4110, 4111 or 4112: the host service ends its data connections then, and the loss of such a
// host side neither closes the device with 4102 nor detaches; the terminal_session.changed of
// the change closes the device with 4100 and the reason (24 13, found by M2-48).
func (r *Relay) HostEnding(hostID ids.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.slots {
		if s.host == hostID {
			s.hostEnding = true
		}
	}
}

// ControlLost is the hook after the control connection of hostID ended with code (Design note
// 10): without 4110, 4111 or 4112 every slot of the host closes its device side with 4102 and its
// host side with 1000; M2-43's ConnectionLost detaches the sessions.
func (r *Relay) ControlLost(hostID ids.ID, code domain.CloseCode) {
	r.mu.Lock()
	var fx effects
	for _, s := range r.slots {
		if s.host == hostID {
			fx = append(fx, r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvControlLost, Code: int(code)}))...)
		}
	}
	r.mu.Unlock()
	fx.run(context.Background())
}

// sessionEnd is the part of the payload TerminalSession of terminal_session.changed the observer
// reads.
type sessionEnd struct {
	HostID      string `json:"host_id"`
	State       string `json:"state"`
	CloseReason string `json:"close_reason"`
	CloseDetail string `json:"close_detail"`
	// OpenedAt, StartedAt and DetachedAt keep the times of a held slot current (M2-46).
	OpenedAt   string `json:"opened_at"`
	StartedAt  string `json:"started_at"`
	DetachedAt string `json:"detached_at"`
}

// observeTimes takes the state and the times of a committed terminal_session.changed into the slot
// of the session on this node, moves its count in bothy.terminal.sessions.active and restarts the
// timer of its limits (M2-46 Design note 7 b: detached_at starts terminal.detach_grace).
func (r *Relay) observeTimes(id ids.ID, p sessionEnd) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.slots[id]
	if s == nil {
		return
	}
	s.times.State = p.State
	if t, err := time.Parse(time.RFC3339Nano, p.OpenedAt); err == nil && s.times.OpenedAt.IsZero() {
		// opened_at never changes; the row's value of the expectation keeps its full precision.
		s.times.OpenedAt = t
	}
	s.times.StartedAt, s.times.DetachedAt = parseStamp(p.StartedAt), parseStamp(p.DetachedAt)
	if s.counted != "" {
		r.countState(s, s.heldState())
	}
	r.armLimit(s)
}

// parseStamp parses an RFC 3339 time of a payload; "" or an invalid value is the zero time.
func parseStamp(v string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}
	}
	return t
}

// ObserveLogEvent is the third consumer of the log tail of an api process (Design note 11 of
// M2-44, 02 "Log tail per api process"): for a terminal_session.changed whose payload has state
// closed it closes the sides of the session (device with StreamCodeForEnd and the close_reason as
// reason, host 4100), drops the slot without writing its counters and sends session.close with the
// close_reason to the host service (a no-op without its link on this node). It never blocks on the
// network; every other event is ignored.
func (r *Relay) ObserveLogEvent(ctx context.Context, typ string, resourceID ids.ID, payload []byte) {
	if typ != eventSessionChg {
		return
	}
	var p sessionEnd
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}
	if p.State != domain.SessionClosed {
		r.observeTimes(resourceID, p)
		r.observeStarted(ctx, resourceID, p.State)
		return
	}
	host, err := ids.Parse(p.HostID)
	if err != nil {
		return
	}
	r.mu.Lock()
	var fx effects
	if s := r.slots[resourceID]; s != nil {
		fx = r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvSessionEnded,
			End: domain.CloseReason(p.CloseReason), Detail: domain.CloseDetail(p.CloseDetail)}))
	}
	r.mu.Unlock()
	fx.run(ctx)
	_ = r.links.CloseSession(ctx, host, resourceID, domain.CloseReason(p.CloseReason))
}

// observeStarted learns from the log tail that a session started on any node (M2-47): with several
// nodes session.started arrives only on the node of the control connection, so the node of the
// pairing marks its slot started here, meets its deadline and, when its pairing already ended,
// detaches the session as the hook of session.started does on one node. With one node the hook
// has done it already.
func (r *Relay) observeStarted(ctx context.Context, id ids.ID, state string) {
	if !r.cfg.Multi || (state != domain.SessionActive && state != domain.SessionDetached) {
		return
	}
	r.mu.Lock()
	var fx effects
	s := r.slots[id]
	switch {
	case s == nil:
	case !s.started:
		fx = r.markStarted(s, state)
	case state == domain.SessionDetached && s.paired:
		// Another node detached a session this node still relays (its control connection of the
		// host ended, M2-43 ConnectionLost): both sides here are alive, so the session is active
		// again (code review M2-47; RelayActivate changes only a detached row).
		fx = effects{func(ctx context.Context) { r.call(ctx, r.uc.RelayActivate(ctx, id)) }}
	}
	r.mu.Unlock()
	fx.run(ctx)
}

// markStarted marks s started on another node's word: the deadline is met; a pairing of this node
// that ended before session.started leaves the session detached, as session.started without a
// pairing does on one node. r.mu is held.
func (r *Relay) markStarted(s *slot, state string) effects {
	s.started = true
	s.gen++
	s.stopDeadline()
	if s.pairedOnce && !s.paired && state == domain.SessionActive {
		id := s.id
		return effects{func(ctx context.Context) { r.call(ctx, r.uc.RelayDetach(ctx, id)) }}
	}
	return nil
}

// Shutdown is the stop step of the relay (Design note 14 of M2-44, 02 section 15.3 step 3): no
// new side attaches; every side closes with 4013 and the counts of every pairing that ends here are
// written (the pool closes after this phase); it waits for the readers within ctx.
func (r *Relay) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	r.closing = true
	var fx effects
	for _, s := range r.slots {
		fx = append(fx, r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvShutdown}))...)
		if s.stop != nil {
			s.stop()
		}
		if s.limitStop != nil {
			s.limitStop()
		}
		s.stopFlush()
	}
	for sd := range r.sides {
		sd.close(int(domain.StreamServerRestart), "")
	}
	r.mu.Unlock()
	fx.run(ctx)
	r.mu.Lock()
	done := make(chan struct{})
	go func() {
		r.mu.Lock()
		for len(r.sides) > 0 {
			r.idle.Wait()
		}
		r.mu.Unlock()
		close(done)
	}()
	r.mu.Unlock()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BindSender sets how the relay announces terminal.session_paired to the other nodes
// (app.Service.SendSignal, M2-47); internal/wire binds it with cluster.mode multi.
func (r *Relay) BindSender(send func(ctx context.Context, s domain.Signal) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.send = send
}

// MarkPairedElsewhere records terminal.session_paired of another node for the slot of the session
// (M2-47 Design note 3): an expectation without sides then only removes itself at its deadline.
// A slot that holds a side or paired on this node keeps M2-44's rule: the node's own signal comes
// back through the listener and must not spare its opening.
func (r *Relay) MarkPairedElsewhere(sessionID ids.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s := r.slots[sessionID]; s != nil && !s.pairedOnce && s.dev == nil && s.hst == nil {
		s.pairedElsewhere = true
	}
}

// hostsWithSides returns the hosts of the slots of this node that hold a side (the reconciliation
// of M2-47 Design note 9).
func (r *Relay) hostsWithSides() []ids.ID {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[ids.ID]bool{}
	var out []ids.ID
	for _, s := range r.slots {
		if (s.dev != nil || s.hst != nil) && !seen[s.host] {
			seen[s.host] = true
			out = append(out, s.host)
		}
	}
	return out
}
