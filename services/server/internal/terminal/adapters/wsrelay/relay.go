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
}

// Compile-time check: the use cases of the module.
var _ RelayUseCases = (*app.Service)(nil)

// RelayConfig holds the configuration keys of the relay.
type RelayConfig struct {
	// BufferMaxBytes is terminal.relay_buffer_max_bytes: the bytes of queued messages per
	// direction and session.
	BufferMaxBytes int
	// OpenTimeout is terminal.open_timeout: the new deadline after a pairing that ended before
	// session.started on a host with local confirmation.
	OpenTimeout time.Duration
}

// The log messages and keys of the relay: length, time, side, close code and session ID only
// (SEC-TERM-1, ruling D-M2-44-5); never frame content, a token, an address or a name.
const (
	msgSideClosed   = "terminal relay connection closed"
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

	mu      gosync.Mutex
	slots   map[ids.ID]*slot
	sides   map[*side]bool
	closing bool
	// idle is signalled when the last side ends (the stop step waits for it).
	idle *gosync.Cond
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
	r := &Relay{cfg: cfg, uc: uc, links: links, timers: timers, clk: clk, log: log, slots: map[ids.ID]*slot{}, sides: map[*side]bool{}}
	r.idle = gosync.NewCond(&r.mu)
	return r, nil
}

// slot is the state of one session on this node.
type slot struct {
	id, host, device ids.ID
	started, confirm bool
	wdi              *int
	// gen counts the grants; a deadline of an older grant does nothing.
	gen      int
	stop     func() bool
	dev, hst *side
	paired   bool
	toDev    queue
	toHost   queue
	// in and out are the transport bytes of the pairings since the last take; lastInput the time
	// of the last device frame; seenDev and seenHost mark the handshake message of each direction
	// since the grant.
	in, out           int64
	lastInput         time.Time
	seenDev, seenHost bool
}

// queue is one direction of a slot, bounded by terminal.relay_buffer_max_bytes.
type queue struct {
	msgs  [][]byte
	bytes int
}

// state returns the facts of domain.Decide.
func (s *slot) state() domain.SlotState {
	return domain.SlotState{Started: s.started, LocalConfirm: s.confirm, DevicePresent: s.dev != nil, HostPresent: s.hst != nil, Paired: s.paired}
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
	s.host, s.device, s.started, s.confirm, s.wdi = e.HostID, e.DeviceID, e.Started, e.LocalConfirm, e.WorkdirIndex
	s.toDev, s.toHost, s.seenDev, s.seenHost, s.paired = queue{}, queue{}, false, false, false
	s.gen++
	r.arm(s, max(e.Deadline.Sub(r.clk.Now()), 0))
	r.mu.Unlock()
	fx.run(ctx)
}

// arm starts the deadline of s after d; r.mu is held.
func (r *Relay) arm(s *slot, d time.Duration) {
	id, gen := s.id, s.gen
	s.stop = r.timers.AfterFunc(d, func() { r.deadline(id, gen) })
}

// deadline is the deadline of the grant gen of the session id.
func (r *Relay) deadline(id ids.ID, gen int) {
	r.mu.Lock()
	s := r.slots[id]
	if s == nil || s.gen != gen {
		r.mu.Unlock()
		return
	}
	fx := r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvDeadline}))
	r.mu.Unlock()
	fx.run(context.Background())
}

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
		id, c := s.id, s.take()
		fx = append([]func(context.Context){func(ctx context.Context) { r.call(ctx, r.uc.RelayCounters(ctx, id, c)) }}, fx...)
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
		return func(ctx context.Context) { r.call(ctx, r.uc.RelayActivate(ctx, id)) }
	case domain.ActDetach:
		return func(ctx context.Context) { r.call(ctx, r.uc.RelayDetach(ctx, id)) }
	case domain.ActEndOpenFailed:
		detail := a.Detail
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
		delete(r.slots, id)
	}
	return nil
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
	fx = append(fx, r.pairIfBoth(s)...)
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
	fx = append(fx, r.pairIfBoth(s)...)
	r.mu.Unlock()
	r.wakeSide(sd)
	fx.run(context.Background())
	return sd, 0
}

// pairIfBoth pairs the two sides of s when both are present; r.mu is held.
func (r *Relay) pairIfBoth(s *slot) effects {
	if s.dev == nil || s.hst == nil || s.paired {
		return nil
	}
	s.paired = true
	r.wakeLocked(s.dev)
	r.wakeLocked(s.hst)
	return r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvPaired}))
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
	r.mu.Unlock()
}

// count adds a message of n bytes from kind to the counters of s; r.mu is held.
func (r *Relay) count(s *slot, kind domain.Side, n int) {
	if kind == domain.SideClient {
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
	}
	r.release(sd)
	r.mu.Unlock()
	fx.run(context.Background())
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
	fx := r.apply(s, domain.Decide(s.state(), domain.RelayEvent{Kind: domain.EvStarted}))
	s.started = true
	r.mu.Unlock()
	fx.run(context.Background())
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
	if err := json.Unmarshal(payload, &p); err != nil || p.State != domain.SessionClosed {
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
