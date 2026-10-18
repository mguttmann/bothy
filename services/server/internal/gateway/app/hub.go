// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	gosync "sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// The event types the hub acts on beyond delivering them (04 sections 8.3 and 8.5).
const (
	eventAccessChanged  = "access.changed"
	eventSessionRevoked = "session.revoked"
	// eventChannelCreated goes to every connection: a principal can read a new channel without a
	// rights change and without a visible target in its space (the default operator agent of the
	// space, M1-65/13).
	eventChannelCreated = "channel.created"
	// The structure events after which the focused threads they affect are re-checked (M1-66).
	eventThreadMoved     = "thread.moved"
	eventChannelArchived = "channel.archived"
	eventSpaceArchived   = "space.archived"
)

// HubConfig holds the configuration keys of delivery.
type HubConfig struct {
	// QueueMax is sync.outbound_queue_max: the bound of the live buffer of a connection in
	// catch-up (the outbound queue itself is bounded by the adapter with the same key).
	QueueMax int
	// PageSize is sync.catchup_page_size.
	PageSize int
	// CursorEvery is sync.cursor_advance_every.
	CursorEvery int
	// RetryBase is sync.log_poll_interval, the first pause before a failed catch-up page is read
	// again; the pause doubles up to RetryCap.
	RetryBase time.Duration
	// RetryCap is database.listen_reconnect_backoff_cap.
	RetryCap time.Duration
	// PayloadMax is sync.event_payload_max_bytes: the payload of an ephemeral event must be
	// smaller (M1-66).
	PayloadMax int
}

// validate names every invalid value.
func (c HubConfig) validate() error {
	if c.QueueMax < 1 || c.PageSize < 1 || c.CursorEvery < 1 || c.RetryBase <= 0 || c.RetryCap <= 0 || c.PayloadMax < 1 {
		return errors.New("gateway: sync.outbound_queue_max, sync.catchup_page_size, sync.cursor_advance_every, " +
			"sync.log_poll_interval, database.listen_reconnect_backoff_cap and sync.event_payload_max_bytes must be positive")
	}
	return nil
}

// HubDeps are the collaborators of the hub.
type HubDeps struct {
	// Authenticator revalidates the credential of a connection on access.changed (identity).
	Authenticator actor.Authenticator
	// Authorizer gives the visible sets of the indices (Scope) and decides gateway.connect.
	Authorizer authorize.Authorizer
	// CatchUp reads catch-up pages and admits each event (sync, M1-63).
	CatchUp sync.CatchUp
	// Log receives failures of revalidation, of the index and of catch-up; nil means
	// slog.Default(). It never receives event contents.
	Log *slog.Logger
	// Clock stamps ephemeral events without a time; nil means clock.System().
	Clock clock.Clock
}

// member is the hub's state of one registered connection. The hub's mutex guards the index
// fields (pending, scopeGen, spaces, channels, instanceWide); mu guards the delivery state.
type member struct {
	c *Connection
	s *Session

	// pending is true while the visible sets are unknown: the member is a candidate of every
	// event, so the candidate set stays a superset of the allowed one.
	pending      bool
	scopeGen     uint64
	spaces       []ids.ID
	channels     []ids.ID
	instanceWide bool

	mu    gosync.Mutex
	phase domain.Phase
	// head is H, the head read for ready or resumed; 0 until Attach.
	head int64
	// buffer holds the live events while the connection catches up.
	buffer []sync.Event
	// lastSeq is the seq of the last frame on the connection that carried one (dispatch or cursor).
	lastSeq int64
	// removed are the removals of access.changed while the connection was not live, each bound
	// to the events before it; the catch-up runner drops their events (04 section 8.3 item 2).
	removed []domain.Targets
	cancel  context.CancelFunc

	// focus are the focused threads of the connection (M1-66) and focusGen counts their
	// replacements, so a re-check never overwrites a newer focus; both are guarded by the hub's
	// mutex like the indices.
	focus    []domain.FocusEntry
	focusGen uint64
}

// Hub implements Delivery and delivers the durable events of the log to the registered
// connections (02 "Hub and delivery", 04 sections 7 to 9): Dispatch is called by the log tail of
// the process, in seq order and from one goroutine, after Authorizer.Observe. It keeps the indices
// principal, space and channel to connections and the instance-wide set, checks every candidate
// with sync.Admit in delivery mode, and never waits on a connection: a full queue closes that
// connection with 4014. It is safe for concurrent use.
type Hub struct {
	cfg HubConfig
	d   HubDeps

	mu           gosync.Mutex
	members      map[*Connection]*member
	byPrincipal  map[ids.ID]map[*member]struct{}
	bySpace      map[ids.ID]map[*member]struct{}
	byChannel    map[ids.ID]map[*member]struct{}
	instanceWide map[*member]struct{}
	pending      map[*member]struct{}
	// byThread is the focus index thread -> connections that focus it (M1-66).
	byThread map[ids.ID]map[*member]struct{}
	// notLive are the connections from Register until they go live: each is a candidate of every
	// event of a space or channel, so its live buffer holds everything it may be entitled to,
	// and the flush decides with the rights of that moment (M1-65/15).
	notLive   map[*member]struct{}
	processed int64
}

// Compile-time check: the hub is the delivery of the gateway.
var _ Delivery = (*Hub)(nil)

// NewHub returns the hub. Errors: an invalid configuration value or a missing collaborator.
func NewHub(cfg HubConfig, d HubDeps) (*Hub, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if d.Authenticator == nil || d.Authorizer == nil || d.CatchUp == nil {
		return nil, errors.New("gateway: NewHub needs the authenticator, the authorizer and catch-up")
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &Hub{
		cfg: cfg, d: d, members: map[*Connection]*member{}, byPrincipal: map[ids.ID]map[*member]struct{}{},
		bySpace: map[ids.ID]map[*member]struct{}{}, byChannel: map[ids.ID]map[*member]struct{}{},
		instanceWide: map[*member]struct{}{}, pending: map[*member]struct{}{}, notLive: map[*member]struct{}{},
		byThread: map[ids.ID]map[*member]struct{}{},
	}, nil
}

// Log messages and values of the hub (docs/observability/signals.md "Logs").
const (
	msgRevalidateFailed = "gateway revalidation failed, connection kept"
	msgScopeFailed      = "gateway visible sets unavailable, connection gets every candidate"
	msgCatchUpRetry     = "gateway catch-up page failed, retrying"
	msgEncodeEvent      = "gateway event not encodable"
	msgAdmitFailed      = "gateway admission check failed, connection closed for resume"
	msgPanic            = "panic recovered"
	errorTypePanic      = "panic"
	errorTypeRevalidate = "revalidate"
	errorTypeScope      = "scope"
	errorTypeCatchUp    = "catch_up"
	errorTypeEncodeEv   = "encode"
	errorTypeAdmit      = "admit"
)

// Register implements Delivery: c enters the hub in phase catching_up as a candidate of every
// event, then its visible sets are read (Authorizer.Scope in delivery mode) and the indices set,
// unless an access.changed of the principal was processed meanwhile (that one set fresher ones).
// The focus of a resumed session is restored, each thread re-checked with gateway.focus in
// delivery mode (04 section 6.3 step 7).
func (h *Hub) Register(ctx context.Context, c *Connection, s *Session) {
	defer h.restoreFocus(ctx, c, s)
	_, a := c.current()
	m := &member{c: c, s: s, pending: true, phase: domain.PhaseCatchingUp}
	h.mu.Lock()
	if old := h.members[c]; old != nil {
		h.removeLocked(old)
	}
	h.members[c] = m
	addTo(h.byPrincipal, a.ID, m)
	h.pending[m] = struct{}{}
	h.notLive[m] = struct{}{}
	gen := m.scopeGen
	h.mu.Unlock()
	sc, err := h.d.Authorizer.Scope(authorize.Delivery(ctx), a, authorize.ActionChannelRead)
	if err != nil {
		h.d.Log.LogAttrs(ctx, slog.LevelWarn, msgScopeFailed, slog.String(logKeyErrorType, errorTypeScope))
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.members[c] == m && m.scopeGen == gen {
		h.setIndexLocked(m, sc)
	}
}

// Attach implements Delivery: without a gap (from >= head) c goes live at once; otherwise a
// catch-up runner reads the pages after from up to head in its own goroutine and then goes live
// (04 section 7.2). Events the tail delivered up to head are covered by the snapshot or the
// catch-up and are dropped from the live buffer.
func (h *Hub) Attach(ctx context.Context, c *Connection, from, head int64) error {
	h.mu.Lock()
	m := h.members[c]
	h.mu.Unlock()
	if m == nil {
		return nil // the connection closed meanwhile
	}
	// The runner outlives the identify or resume op: it drops the op's request memo, so every
	// catch-up page reads the current rights version with its own memo (AZ-P9).
	rctx, cancel := context.WithCancel(authorize.WithoutRequestScope(context.WithoutCancel(ctx)))
	m.mu.Lock()
	m.head, m.lastSeq, m.cancel = head, from, cancel
	m.mu.Unlock()
	if from >= head {
		m.mu.Lock()
		m.lastSeq = head
		m.mu.Unlock()
		h.flush(rctx, m)
		return nil
	}
	go h.runCatchUp(rctx, m, from, head)
	return nil
}

// Detach implements Delivery: c leaves the indices, its catch-up runner stops.
func (h *Hub) Detach(c *Connection) {
	h.mu.Lock()
	m := h.members[c]
	if m != nil {
		h.removeLocked(m)
	}
	h.mu.Unlock()
	if m == nil {
		return
	}
	m.mu.Lock()
	m.phase, m.buffer = domain.PhaseClosing, nil
	cancel := m.cancel
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Len returns the number of registered connections.
func (h *Hub) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.members)
}

// Dispatch delivers ev to every connection that may receive it now (02 "Hub and delivery"). It is
// called by the log tail only, in seq order, after Authorizer.Observe, and never waits on a
// connection. access.changed first revalidates, re-indexes and purges the connections of its
// principal; session.revoked first closes the connections of that session with 4012 (04 sections
// 8.3 and 8.5); then the event takes the normal path, so it never overtakes earlier frames.
func (h *Hub) Dispatch(ctx context.Context, ev sync.Event) {
	switch {
	case ev.Type == eventAccessChanged && !ev.AudiencePrincipalID.IsZero():
		h.accessChanged(ctx, ev)
	case ev.Type == eventSessionRevoked && !ev.AudiencePrincipalID.IsZero():
		h.sessionRevoked(ev)
	case ev.Type == eventThreadMoved || ev.Type == eventChannelArchived || ev.Type == eventSpaceArchived:
		h.structureChanged(ctx, ev)
	}
	enc := &encoded{ev: ev}
	for _, m := range h.candidates(ev) {
		h.deliver(ctx, m, ev, enc)
	}
	h.advance(ctx, ev.Seq)
}

// encoded encodes the dispatch frame of one event once per form: with trace_context for agents,
// without for everyone else (04 "Envelope").
type encoded struct {
	ev           sync.Event
	plain, agent []byte
}

// frame returns the dispatch frame for a receiver of kind.
func (e *encoded) frame(kind actor.Kind) ([]byte, error) {
	isAgent := kind == actor.KindAgent
	cached := &e.plain
	if isAgent {
		cached = &e.agent
	}
	if *cached != nil {
		return *cached, nil
	}
	b, err := json.Marshal(outFrame{Op: "dispatch", D: e.ev.Envelope(sync.DeliveryFields{IncludeTraceContext: isAgent})})
	if err != nil {
		return nil, err
	}
	*cached = b
	return b, nil
}

// outFrame is the outer object {op, d} of a server frame the hub encodes itself.
type outFrame struct {
	Op string `json:"op"`
	D  any    `json:"d"`
}

// cursorFrame and completeFrame are the bodies of cursor and catchup.complete (schemas
// GatewayCursor and GatewayCatchupComplete).
type (
	cursorFrame struct {
		Seq int64 `json:"seq"`
	}
	completeFrame struct {
		HeadSeq int64 `json:"head_seq"`
	}
)

// controlEntry returns the control entry of the frame op with body d.
func controlEntry(op string, d any, seq int64) domain.Entry {
	// Both bodies are plain integers, which always encode.
	b, _ := json.Marshal(outFrame{Op: op, D: d})
	return domain.Entry{Frame: b, Class: domain.ClassControl, Seq: seq}
}

// dispatchEntry returns the dispatch entry of ev for a receiver of kind.
func dispatchEntry(enc *encoded, kind actor.Kind) (domain.Entry, error) {
	b, err := enc.frame(kind)
	if err != nil {
		return domain.Entry{}, err
	}
	ev := enc.ev
	return domain.Entry{Frame: b, Class: domain.ClassDispatch, Seq: ev.Seq, SpaceID: ev.SpaceID, ChannelID: ev.ChannelID,
		OccurredAt: ev.OccurredAt}, nil
}

// candidates returns the connections that may receive ev (02 "Hub and delivery"): the
// connections of the audience of a personal event; for an event of a space or channel the space
// and channel indices, the instance-wide set, every connection whose sets are not known yet and
// every connection that is not live yet (M1-65/15);
// for channel.created and an event of the instance every connection. The set is a superset of the
// allowed one; Admit decides, and an admitted event of a channel adds the channel to the indices
// of the connection (learn).
func (h *Hub) candidates(ev sync.Event) []*member {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := map[*member]struct{}{}
	switch {
	case !ev.AudiencePrincipalID.IsZero():
		addAll(set, h.byPrincipal[ev.AudiencePrincipalID])
	case ev.Type != eventChannelCreated && (!ev.SpaceID.IsZero() || !ev.ChannelID.IsZero()):
		addAll(set, h.bySpace[ev.SpaceID])
		addAll(set, h.byChannel[ev.ChannelID])
		addAll(set, h.instanceWide)
		addAll(set, h.pending)
		addAll(set, h.notLive)
	default:
		for _, m := range h.members {
			set[m] = struct{}{}
		}
	}
	out := make([]*member, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	return out
}

// deliver hands ev to m: dropped while it closes or when its seq is at most the head of m (the
// snapshot or the catch-up covers it, a lagging tail must not send it twice), buffered while m
// catches up, otherwise admitted and queued without waiting. A full buffer or queue closes m
// with 4014; an unauthenticated decision with 4012; a failed check with 4000.
func (h *Hub) deliver(ctx context.Context, m *member, ev sync.Event, enc *encoded) {
	m.mu.Lock()
	switch {
	case m.phase == domain.PhaseClosing:
		m.mu.Unlock()
		return
	case m.head > 0 && ev.Seq <= m.head:
		m.mu.Unlock()
		h.covered(m, ev)
		return
	case m.phase == domain.PhaseCatchingUp:
		m.buffer = append(m.buffer, ev)
		over := len(m.buffer) > h.cfg.QueueMax
		m.mu.Unlock()
		if over {
			h.closeMember(m, domain.CloseSlowConsumer)
		}
		return
	}
	m.mu.Unlock()
	e, ok := h.admit(ctx, m, ev, enc)
	if !ok {
		return
	}
	m.mu.Lock()
	switch {
	case m.phase == domain.PhaseClosing:
		m.mu.Unlock()
		return
	case m.c.conn.Offer(e):
		m.lastSeq = ev.Seq
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	h.closeMember(m, domain.CloseSlowConsumer)
}

// admit checks ev for m with sync.Admit in delivery mode and returns the dispatch entry to queue;
// false means nothing is queued. An unauthenticated answer closes m with 4012 (its credential was
// revoked). Any other error closes m with 4000: dropping the event would let the next frame with
// a seq move the client's cursor past it, so the client resumes from its own cursor instead and
// catches up (04 section 9 item 1).
func (h *Hub) admit(ctx context.Context, m *member, ev sync.Event, enc *encoded) (domain.Entry, bool) {
	_, a := m.c.current()
	ok, err := h.d.CatchUp.Admit(authorize.Delivery(ctx), a, ev)
	switch {
	case errs.KindOf(err) == errs.KindUnauthenticated:
		h.closeMember(m, domain.CloseRevoked)
		return domain.Entry{}, false
	case err != nil:
		h.d.Log.LogAttrs(ctx, slog.LevelWarn, msgAdmitFailed, slog.String(logKeyErrorType, errorTypeAdmit))
		h.closeMember(m, domain.CloseUnknownError)
		return domain.Entry{}, false
	case !ok:
		return domain.Entry{}, false
	}
	h.learn(m, ev)
	e, err := dispatchEntry(enc, a.Kind)
	if err != nil {
		h.d.Log.LogAttrs(ctx, slog.LevelError, msgEncodeEvent, slog.String(logKeyErrorType, errorTypeEncodeEv))
		return domain.Entry{}, false
	}
	return e, true
}

// learn adds the channel and space of ev, admitted for m, to the indices of m when they are
// missing, so the later events of a channel m reads without a rights change (a new channel of a
// space whose default operator m's agent is) stay candidates of m (M1-65/13). The next
// access.changed of m replaces the indices with a fresh Scope again.
func (h *Hub) learn(m *member, ev sync.Event) {
	if !ev.AudiencePrincipalID.IsZero() || ev.ChannelID.IsZero() {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.members[m.c] != m || m.pending {
		return
	}
	if _, ok := h.byChannel[ev.ChannelID][m]; !ok {
		m.channels = append(m.channels, ev.ChannelID)
		addTo(h.byChannel, ev.ChannelID, m)
	}
	if _, ok := h.bySpace[ev.SpaceID][m]; !ok && !ev.SpaceID.IsZero() {
		m.spaces = append(m.spaces, ev.SpaceID)
		addTo(h.bySpace, ev.SpaceID, m)
	}
}

// covered handles an event that m does not receive because its seq is at most the head of m: a
// channel.created still adds its channel to the indices of m, since the visible sets of m may
// have been read before the channel existed and no access.changed follows (M1-65/13). This only
// widens the candidate set; Admit decides every later event.
func (h *Hub) covered(m *member, ev sync.Event) {
	if ev.Type == eventChannelCreated {
		h.learn(m, ev)
	}
}

// advance counts the event seq the hub processed and, every sync.cursor_advance_every processed
// events, sends cursor {seq} to each live connection whose last frame with a seq is older (04 section
// 7.1, R-SY-03). Not being a candidate implies not visible, because the candidate set is a
// superset.
func (h *Hub) advance(_ context.Context, seq int64) {
	h.mu.Lock()
	h.processed++
	due := h.processed%int64(h.cfg.CursorEvery) == 0
	var live []*member
	if due {
		for _, m := range h.members {
			live = append(live, m)
		}
	}
	h.mu.Unlock()
	for _, m := range live {
		m.mu.Lock()
		if m.phase != domain.PhaseLive || m.lastSeq >= seq {
			m.mu.Unlock()
			continue
		}
		ok := m.c.conn.Offer(controlEntry("cursor", cursorFrame{Seq: seq}, seq))
		if ok {
			m.lastSeq = seq
		}
		m.mu.Unlock()
		if !ok {
			h.closeMember(m, domain.CloseSlowConsumer)
		}
	}
}

// flush ends the catch-up of m (04 section 7.2 step 5): the buffered live events with seq > H are
// admitted with the newest rights and queued in seq order, batch by batch, until the buffer is
// empty; then m is live. Like the pages it waits for queue space (only the live buffer is bounded
// by sync.outbound_queue_max); Dispatch keeps appending to the buffer meanwhile, so nothing is
// mixed or lost.
func (h *Hub) flush(ctx context.Context, m *member) {
	for {
		m.mu.Lock()
		if m.phase != domain.PhaseCatchingUp {
			m.mu.Unlock()
			return
		}
		if len(m.buffer) == 0 {
			m.phase = domain.PhaseLive
			m.mu.Unlock()
			h.mu.Lock()
			delete(h.notLive, m)
			h.mu.Unlock()
			return
		}
		batch, head := m.buffer, m.head
		m.buffer = nil
		m.mu.Unlock()
		if !h.flushBatch(ctx, m, batch, head) {
			return
		}
	}
}

// flushBatch admits and queues the events of batch with seq > head; false when m closed.
func (h *Hub) flushBatch(ctx context.Context, m *member, batch []sync.Event, head int64) bool {
	for _, ev := range batch {
		if ev.Seq <= head {
			h.covered(m, ev)
			continue
		}
		if e, ok := h.admit(ctx, m, ev, &encoded{ev: ev}); ok && !h.offer(ctx, m, e) {
			return false
		}
	}
	return true
}

// closeMember marks m closing and closes its connection with code.
func (h *Hub) closeMember(m *member, code domain.CloseCode) {
	m.mu.Lock()
	m.phase, m.buffer = domain.PhaseClosing, nil
	m.mu.Unlock()
	m.c.Close(code, "")
}

// setIndexLocked replaces the visible sets of m with sc; h.mu is held.
func (h *Hub) setIndexLocked(m *member, sc authorize.Scope) {
	h.unindexLocked(m)
	m.spaces, m.channels, m.instanceWide, m.pending = sc.SpaceIDs, sc.ChannelIDs, sc.InstanceWide, false
	delete(h.pending, m)
	for _, s := range m.spaces {
		addTo(h.bySpace, s, m)
	}
	for _, c := range m.channels {
		addTo(h.byChannel, c, m)
	}
	if m.instanceWide {
		h.instanceWide[m] = struct{}{}
	}
}

// unindexLocked removes m from the space, channel and instance-wide indices; h.mu is held.
func (h *Hub) unindexLocked(m *member) {
	for _, s := range m.spaces {
		removeFrom(h.bySpace, s, m)
	}
	for _, c := range m.channels {
		removeFrom(h.byChannel, c, m)
	}
	delete(h.instanceWide, m)
	m.spaces, m.channels, m.instanceWide = nil, nil, false
}

// removeLocked removes m from every index; h.mu is held.
func (h *Hub) removeLocked(m *member) {
	h.unindexLocked(m)
	h.setFocusLocked(m, nil)
	delete(h.pending, m)
	delete(h.notLive, m)
	_, a := m.c.current()
	removeFrom(h.byPrincipal, a.ID, m)
	if h.members[m.c] == m {
		delete(h.members, m.c)
	}
}

// addTo adds m to the set of key in idx.
func addTo(idx map[ids.ID]map[*member]struct{}, key ids.ID, m *member) {
	set := idx[key]
	if set == nil {
		set = map[*member]struct{}{}
		idx[key] = set
	}
	set[m] = struct{}{}
}

// removeFrom removes m from the set of key in idx.
func removeFrom(idx map[ids.ID]map[*member]struct{}, key ids.ID, m *member) {
	set := idx[key]
	delete(set, m)
	if len(set) == 0 {
		delete(idx, key)
	}
}

// addAll adds every member of src to dst.
func addAll(dst, src map[*member]struct{}) {
	for m := range src {
		dst[m] = struct{}{}
	}
}

// membersOf returns the connections of principal p.
func (h *Hub) membersOf(p ids.ID) []*member {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*member, 0, len(h.byPrincipal[p]))
	for m := range h.byPrincipal[p] {
		out = append(out, m)
	}
	return out
}
