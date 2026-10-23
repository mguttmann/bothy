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
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// The values of run.updated and of the durable run.state_changed the coalescer reads.
const (
	eventRunUpdated      = string(authorize.EventRunUpdated)
	eventRunStateChanged = string(authorize.EventRunStateChanged)
	resourceKindRun      = "run"
	runUpdatedSchema     = 1
	// ReasonPatchReplaced is bothy.reason of a run.updated frame dropped at a full queue and
	// replaced by the whole document later (signals.md 4.1).
	ReasonPatchReplaced = "patch_replaced_by_snapshot"
)

// Log messages of the coalescer; never a payload or a document.
const (
	msgLiveBoxRead   = "gateway live box document unavailable, snapshot retried"
	msgLiveBoxEncode = "gateway live box frame not encodable"
	msgLiveBoxBadEv  = "gateway run.updated without its payload type, dropped"
	errorTypeLiveBox = "live_box"
)

// RunEncoder converts a run document into the JSON of the schema Run, the body getRun answers;
// internal/wire passes the converter of runs/adapters/httpapi, so snapshot, conflict current and
// getRun are one projection (M2-10 design note 10).
type RunEncoder func(runs.Run) (json.RawMessage, error)

// LiveBoxConfig holds the configuration of the coalescer.
type LiveBoxConfig struct {
	// Interval is sync.run_patch_coalesce_interval: the window of a run's changes and the period
	// of the snapshot retry.
	Interval time.Duration
}

// LiveBoxDeps are the collaborators of the coalescer.
type LiveBoxDeps struct {
	// Documents reads the whole documents of active runs (runs.LiveBox, delivery reads).
	Documents runs.LiveBox
	// Encode converts a document into the schema Run.
	Encode RunEncoder
}

// sentKey is one connection and one run.
type sentKey struct {
	m   *member
	run ids.ID
}

// sentVersion is the version a connection last received of a run and the run's thread, so a
// focus change can drop it without the run's chain.
type sentVersion struct {
	version int64
	thread  ids.ID
}

// runChain is the coalescer's state of one run: its place, the actor of the last change and
// the chain of the open window (due is zero when no window is open).
type runChain struct {
	thread, space, channel ids.ID
	actor                  *eventlog.ActorRef
	chain                  domain.Chain
	due                    time.Time
	ended                  bool
}

// Coalescer gathers the run.updated changes of every run for sync.run_patch_coalesce_interval and
// sends one frame per run and connection at the end of the window (M2-10 design note 9; 04
// section 11.3 points 3 and 5; 02 "Hub and delivery": the hub collects). Each connection gets the
// merged ops when the version it last received of the run is the chain's base, otherwise the whole
// document; a frame that does not fit the outbound queue is dropped, counted with
// patch_replaced_by_snapshot and replaced by a snapshot later (never 4014). Newly focused threads
// and the focus a resume restored get the document once, after catch-up. Its state lives behind
// its own mutex: the publisher, Hub.Dispatch and the focus ops only record and wake it, and the
// coalescer never holds that mutex while it reads documents, admits or offers frames, so neither
// side waits on the other (Run, Tick). It is safe for concurrent use.
//
// Extension point (M7-17): a "run changed to version V" signal of another api node can call
// Changed with an empty change; the connections then get a snapshot at the window end.
type Coalescer struct {
	h     *Hub
	cfg   LiveBoxConfig
	d     LiveBoxDeps
	clock clock.Clock
	wake  chan struct{}

	mu   gosync.Mutex
	runs map[ids.ID]*runChain
	// sent is the version of a run each connection last received, with the run's thread.
	sent map[sentKey]sentVersion
	// marks are the connections and runs that need the whole document (a dropped frame, a
	// connection that was not live at a window end), with the run's thread.
	marks map[sentKey]ids.ID
	// focused are the newly focused threads per connection whose snapshot is not sent yet.
	focused map[*member]map[ids.ID]struct{}
}

// BindLiveBox makes run.updated go through a coalescer over d instead of the immediate path of
// M1-66 and returns it; internal/wire calls it once before serving and runs Coalescer.Run with
// the process lifecycle. Errors: a non-positive interval or a missing collaborator.
func (h *Hub) BindLiveBox(cfg LiveBoxConfig, d LiveBoxDeps) (*Coalescer, error) {
	if cfg.Interval <= 0 {
		return nil, errors.New("gateway: sync.run_patch_coalesce_interval must be positive")
	}
	if d.Documents == nil || d.Encode == nil {
		return nil, errors.New("gateway: BindLiveBox needs the documents and the encoder")
	}
	c := &Coalescer{h: h, cfg: cfg, d: d, clock: h.d.Clock, wake: make(chan struct{}, 1),
		runs: map[ids.ID]*runChain{}, sent: map[sentKey]sentVersion{}, marks: map[sentKey]ids.ID{}, focused: map[*member]map[ids.ID]struct{}{}}
	h.mu.Lock()
	h.livebox = c
	h.mu.Unlock()
	return c, nil
}

// liveBox returns the bound coalescer, nil before BindLiveBox.
func (h *Hub) liveBox() *Coalescer {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.livebox
}

// signal wakes the goroutine of Run without waiting.
func (c *Coalescer) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Run is the goroutine of the coalescer: it calls Tick whenever a window, a retry or a pending
// focus is due and whenever it is woken, until ctx ends. A panic in one round is logged and the
// loop goes on, so a bad document never stops the live box of every run.
func (c *Coalescer) Run(ctx context.Context) {
	timer := time.NewTimer(c.cfg.Interval)
	defer timer.Stop()
	for {
		next := c.safeTick(ctx)
		wait := c.cfg.Interval
		if !next.IsZero() {
			wait = max(next.Sub(c.clock.Now()), time.Millisecond)
		}
		timer.Reset(wait)
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
		case <-timer.C:
		}
	}
}

// safeTick runs Tick and recovers a panic.
func (c *Coalescer) safeTick(ctx context.Context) (next time.Time) {
	defer func() {
		if p := recover(); p != nil {
			c.h.d.Log.LogAttrs(ctx, slog.LevelError, msgPanic, slog.String(logKeyErrorType, errorTypePanic))
			next = c.clock.Now().Add(c.cfg.Interval)
		}
	}()
	return c.Tick(ctx)
}

// Changed records one run.updated as runs published it after a commit (payload runs.RunUpdated)
// and opens the window of its run when none is open. It never waits on a connection.
func (c *Coalescer) Changed(ctx context.Context, ev eventlog.EphemeralEvent) {
	u, ok := ev.Payload.(runs.RunUpdated)
	if !ok {
		c.h.d.Log.LogAttrs(ctx, slog.LevelError, msgLiveBoxBadEv, slog.String(logKeyErrorType, errorTypeLiveBox))
		return
	}
	ops := make([]domain.PatchOp, 0, len(u.Ops))
	for _, o := range u.Ops {
		ops = append(ops, domain.PatchOp{Op: o.Op, Path: o.Path, Value: o.Value})
	}
	c.mu.Lock()
	rc := c.runs[u.RunID]
	if rc == nil {
		rc = &runChain{}
		c.runs[u.RunID] = rc
	}
	rc.thread, rc.space, rc.channel, rc.actor = u.ThreadID, ev.Scope.SpaceID, ev.Scope.ChannelID, ev.Actor
	if rc.chain.Add(u.BaseVersion, u.Version, ops, u.Truncated) && rc.due.IsZero() {
		rc.due = c.clock.Now().Add(c.cfg.Interval)
	}
	c.mu.Unlock()
	c.signal()
}

// stateChanged notes a durable run.state_changed: a run that reached an end state is dropped
// after its last frame (design note 9). Hub.Dispatch calls it.
func (c *Coalescer) stateChanged(ev sync.Event) {
	var p struct {
		Run struct {
			State string `json:"state"`
		} `json:"run"`
	}
	if json.Unmarshal(ev.Payload, &p) != nil {
		return
	}
	switch p.Run.State {
	case "completed", "failed", "cancelled":
	default:
		return
	}
	c.mu.Lock()
	if rc := c.runs[ev.ResourceID]; rc != nil {
		rc.ended = true
	} else {
		c.dropRunLocked(ev.ResourceID)
	}
	c.mu.Unlock()
	c.signal()
}

// focusChanged records a replaced focus of m (Hub.setFocusLocked, the hub's mutex held): the
// threads it no longer focuses lose their state (07 "Immediate effect on revocation" point 3), the
// newly focused ones wait for their snapshot.
func (c *Coalescer) focusChanged(m *member, added, removed []ids.ID) {
	c.mu.Lock()
	c.unfocusLocked(m, removed)
	if len(added) > 0 {
		set := c.focused[m]
		if set == nil {
			set = map[ids.ID]struct{}{}
			c.focused[m] = set
		}
		for _, t := range added {
			set[t] = struct{}{}
		}
	}
	if len(c.focused[m]) == 0 {
		delete(c.focused, m)
	}
	c.mu.Unlock()
	if len(added) > 0 {
		c.signal()
	}
}

// unfocusLocked drops the sent versions, marks and pending snapshots of m for the threads of
// removed; c.mu is held.
func (c *Coalescer) unfocusLocked(m *member, removed []ids.ID) {
	if len(removed) == 0 {
		return
	}
	gone := make(map[ids.ID]bool, len(removed))
	for _, t := range removed {
		gone[t] = true
		delete(c.focused[m], t)
	}
	for k, v := range c.sent {
		if k.m == m && gone[v.thread] {
			delete(c.sent, k)
		}
	}
	for k, t := range c.marks {
		if k.m == m && gone[t] {
			delete(c.marks, k)
		}
	}
}

// memberGone drops every state of m (Detach).
func (c *Coalescer) memberGone(m *member) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.focused, m)
	for k := range c.sent {
		if k.m == m {
			delete(c.sent, k)
		}
	}
	for k := range c.marks {
		if k.m == m {
			delete(c.marks, k)
		}
	}
}

// dropRunLocked removes every state of run; c.mu is held.
func (c *Coalescer) dropRunLocked(run ids.ID) {
	delete(c.runs, run)
	for k := range c.sent {
		if k.run == run {
			delete(c.sent, k)
		}
	}
	for k := range c.marks {
		if k.run == run {
			delete(c.marks, k)
		}
	}
}

// window is one due run, copied out of the state so the round works without the mutex.
type window struct {
	run   ids.ID
	rc    runChain
	ended bool
}

// Tick runs one round at the clock's now: the windows that are due, the snapshot marks and the
// newly focused threads of live connections. It returns the instant of the next due window, or
// the retry instant while marks or focused threads wait, or zero when nothing waits. Tests call it
// with a fake clock instead of Run.
func (c *Coalescer) Tick(ctx context.Context) time.Time {
	now := c.clock.Now()
	dctx := authorize.Delivery(ctx)
	docs := map[ids.ID]*docRead{}
	// Only the marks from before this round are retried now; a frame dropped at this window end
	// waits one interval, so one drop counts once.
	earlier := c.markKeys()
	for _, w := range c.dueWindows(now) {
		c.flushWindow(dctx, w, docs)
	}
	c.retryMarks(dctx, docs, earlier)
	c.sendFocused(dctx)
	return c.next(now)
}

// dueWindows copies and resets the windows due at now; an ended run without a window is dropped.
func (c *Coalescer) dueWindows(now time.Time) []window {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []window
	for run, rc := range c.runs {
		switch {
		case !rc.due.IsZero() && !rc.due.After(now):
			w := window{run: run, rc: *rc, ended: rc.ended}
			w.rc.chain.Ops = append([]domain.PatchOp(nil), rc.chain.Ops...)
			out = append(out, w)
			rc.chain.Reset()
			rc.due = time.Time{}
			if rc.ended {
				delete(c.runs, run)
			}
		case rc.due.IsZero() && rc.ended:
			c.dropRunLocked(run)
		}
	}
	return out
}

// next returns the next instant Tick wants.
func (c *Coalescer) next(now time.Time) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	var next time.Time
	for _, rc := range c.runs {
		if !rc.due.IsZero() && (next.IsZero() || rc.due.Before(next)) {
			next = rc.due
		}
	}
	if len(c.marks) > 0 || len(c.focused) > 0 {
		retry := now.Add(c.cfg.Interval)
		if next.IsZero() || retry.Before(next) {
			next = retry
		}
	}
	return next
}

// docRead is the result of one document read in a round: one read per run, shared by every
// connection that needs it (design note 9).
type docRead struct {
	frame   []byte
	version int64
	active  bool
	err     error
}

// flushWindow sends the window w to every live connection that focuses the run's thread and is
// admitted with run.read now (design note 9). After an ended run, the state of the run goes.
func (c *Coalescer) flushWindow(ctx context.Context, w window, docs map[ids.ID]*docRead) {
	defer func() {
		if w.ended {
			c.mu.Lock()
			c.dropRunLocked(w.run)
			c.mu.Unlock()
		}
	}()
	if !w.rc.chain.Pending() {
		return
	}
	ops := &opsOnce{}
	for _, m := range c.h.focusing(w.rc.thread) {
		key := sentKey{m: m, run: w.run}
		if !c.h.live(m) {
			c.mark(key, w.rc.thread)
			continue
		}
		if !c.admitted(ctx, m, w.run, w.rc) {
			c.unmark(key)
			continue
		}
		if frame, version, ok := c.windowFrame(ctx, w, key, ops, docs); ok {
			c.offer(ctx, m, key, w.rc, frame, version)
		}
	}
}

// opsOnce holds the ops frame of a window, encoded at most once.
type opsOnce struct {
	tried bool
	frame []byte
}

// windowFrame returns the frame of w for key: the ops frame when the connection's last version is
// the chain's base and the frame fits, otherwise the snapshot; false when the run is no longer
// active or the connection already holds the window's newest version.
func (c *Coalescer) windowFrame(ctx context.Context, w window, key sentKey, ops *opsOnce, docs map[ids.ID]*docRead) ([]byte, int64, bool) {
	c.mu.Lock()
	last := c.sent[key].version
	c.mu.Unlock()
	// A retried snapshot can already carry the window's newest version; sending it again would
	// only repeat the document.
	if last >= w.rc.chain.Version {
		return nil, 0, false
	}
	if w.rc.chain.Sendable(last) {
		if !ops.tried {
			ops.tried, ops.frame = true, c.opsFrame(ctx, w)
		}
		if ops.frame != nil {
			return ops.frame, w.rc.chain.Version, true
		}
	}
	d := c.document(ctx, w.run, w.rc, docs)
	if !d.active {
		return nil, 0, false
	}
	return d.frame, d.version, true
}

// retryMarks sends the whole document to every marked connection that is live, still focuses the
// run's thread and passes run.read; a mark goes when it is sent, when the connection no longer
// focuses the thread or when the run is no longer active.
func (c *Coalescer) retryMarks(ctx context.Context, docs map[ids.ID]*docRead, earlier map[sentKey]bool) {
	c.mu.Lock()
	marks := make(map[sentKey]ids.ID, len(c.marks))
	for k, t := range c.marks {
		if earlier[k] {
			marks[k] = t
		}
	}
	c.mu.Unlock()
	for k, thread := range marks {
		e, ok := c.h.focusEntry(k.m, thread)
		if !ok {
			c.unmark(k)
			continue
		}
		if !c.h.live(k.m) {
			continue
		}
		rc := c.place(k.run, e)
		d := c.document(ctx, k.run, rc, docs)
		switch {
		case d.err != nil:
			continue
		case !d.active:
			c.unmark(k)
			continue
		}
		if !c.admitted(ctx, k.m, k.run, rc) {
			c.unmark(k)
			continue
		}
		c.offer(ctx, k.m, k, rc, d.frame, d.version)
	}
}

// markKeys returns the keys of the current marks.
func (c *Coalescer) markKeys() map[sentKey]bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[sentKey]bool, len(c.marks))
	for k := range c.marks {
		out[k] = true
	}
	return out
}

// sendFocused sends one snapshot per active run of the threads each live connection focused
// newly (design note 10); a connection in catch-up keeps its threads until it is live, because
// M1-66 buffers no ephemeral frame.
func (c *Coalescer) sendFocused(ctx context.Context) {
	for m, threads := range c.pendingFocus() {
		if !c.h.live(m) {
			continue
		}
		docs, err := c.d.Documents.Documents(ctx, threads)
		if err != nil {
			c.h.d.Log.LogAttrs(ctx, slog.LevelWarn, msgLiveBoxRead, slog.String(logKeyErrorType, errorTypeLiveBox))
			continue
		}
		c.focusDone(m, threads)
		for _, doc := range docs {
			c.sendFocusSnapshot(ctx, m, doc)
		}
	}
}

// pendingFocus returns the newly focused threads per connection.
func (c *Coalescer) pendingFocus() map[*member][]ids.ID {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[*member][]ids.ID{}
	for m, set := range c.focused {
		for t := range set {
			out[m] = append(out[m], t)
		}
	}
	return out
}

// focusDone removes threads from the pending focus of m.
func (c *Coalescer) focusDone(m *member, threads []ids.ID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range threads {
		delete(c.focused[m], t)
	}
	if len(c.focused[m]) == 0 {
		delete(c.focused, m)
	}
}

// sendFocusSnapshot sends the snapshot of doc to m when m still focuses its thread, passes
// run.read and did not receive this version yet (a retried mark may have sent it).
func (c *Coalescer) sendFocusSnapshot(ctx context.Context, m *member, doc runs.Run) {
	e, ok := c.h.focusEntry(m, doc.ThreadID)
	if !ok {
		return
	}
	key := sentKey{m: m, run: doc.ID}
	c.mu.Lock()
	have := c.sent[key].version
	c.mu.Unlock()
	if have >= doc.Version {
		return
	}
	rc := c.place(doc.ID, e)
	if !c.admitted(ctx, m, doc.ID, rc) {
		return
	}
	d := c.encodeDocument(ctx, doc, rc)
	if d.err != nil {
		return
	}
	c.offer(ctx, m, key, rc, d.frame, d.version)
}

// place returns the place of run for a frame from the focus entry e of the connection (the
// thread's channel and space now) and the actor of the run's last change when the coalescer knows
// it.
func (c *Coalescer) place(run ids.ID, e domain.FocusEntry) runChain {
	rc := runChain{thread: e.ThreadID, space: e.SpaceID, channel: e.ChannelID}
	c.mu.Lock()
	defer c.mu.Unlock()
	if known := c.runs[run]; known != nil {
		rc.actor = known.actor
	}
	return rc
}

// mark records that key needs the whole document.
func (c *Coalescer) mark(key sentKey, thread ids.ID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.marks[key] = thread
}

// unmark removes the mark of key.
func (c *Coalescer) unmark(key sentKey) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.marks, key)
}

// admitted reports whether m may receive run.updated of run now: sync.Admit with run.read in
// delivery mode (04 section 11.3 point 2, 07 "Delivery filter"). A failed check refuses; the
// durable path closes or revokes the connection on its next event.
func (c *Coalescer) admitted(ctx context.Context, m *member, run ids.ID, rc runChain) bool {
	_, a := m.c.current()
	ok, err := c.h.d.CatchUp.Admit(ctx, a, c.admitEvent(run, rc))
	return err == nil && ok
}

// admitEvent is the event the admission decides on: run.updated of run in its thread.
func (c *Coalescer) admitEvent(run ids.ID, rc runChain) sync.Event {
	return sync.Event{
		Type: eventRunUpdated, SchemaVersion: runUpdatedSchema, OccurredAt: c.clock.Now(),
		SpaceID: rc.space, ChannelID: rc.channel, ThreadID: rc.thread,
		ResourceKind: resourceKindRun, ResourceID: run, ReceiveAction: string(authorize.ActionRunRead),
	}
}

// offer queues frame for m and records version as sent; a full queue drops it, counts
// patch_replaced_by_snapshot and marks m and the run for a snapshot. It never closes m.
func (c *Coalescer) offer(ctx context.Context, m *member, key sentKey, rc runChain, frame []byte, version int64) {
	entry := domain.Entry{Frame: frame, Class: domain.ClassDispatch, SpaceID: rc.space, ChannelID: rc.channel,
		OccurredAt: c.clock.Now(), Live: true, Ephemeral: true}
	m.mu.Lock()
	ok := m.phase == domain.PhaseLive && m.c.conn.Offer(entry)
	m.mu.Unlock()
	if !ok {
		c.h.d.Metrics.Dropped(ctx, ReasonPatchReplaced, 1)
		c.mark(key, rc.thread)
		return
	}
	c.mu.Lock()
	c.sent[key] = sentVersion{version: version, thread: rc.thread}
	delete(c.marks, key)
	c.mu.Unlock()
}

// runUpdatedBody is the payload RunUpdatedPayload of a frame.
type runUpdatedBody struct {
	RunID       ids.ID          `json:"run_id"`
	BaseVersion int64           `json:"base_version,omitzero"`
	Version     int64           `json:"version"`
	Ops         *[]wireOp       `json:"ops,omitempty"`
	Snapshot    json.RawMessage `json:"snapshot,omitempty"`
}

// wireOp is one op of a frame (schema JsonPatchOperation).
type wireOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value,omitempty"`
}

// opsFrame encodes the ops frame of w once; nil when the payload reaches
// sync.event_payload_max_bytes (then every connection gets the snapshot, ruling D-M2-10-12).
func (c *Coalescer) opsFrame(ctx context.Context, w window) []byte {
	ops := make([]wireOp, 0, len(w.rc.chain.Ops))
	for _, o := range w.rc.chain.Ops {
		ops = append(ops, wireOp{Op: o.Op, Path: o.Path, Value: o.Value})
	}
	body := runUpdatedBody{RunID: w.run, BaseVersion: w.rc.chain.Base, Version: w.rc.chain.Version, Ops: &ops}
	frame, fits := c.frame(ctx, w.run, w.rc, body, false)
	if !fits {
		return nil
	}
	return frame
}

// document reads and encodes the snapshot frame of run at the place rc once per round.
func (c *Coalescer) document(ctx context.Context, run ids.ID, rc runChain, docs map[ids.ID]*docRead) *docRead {
	if d, ok := docs[run]; ok {
		return d
	}
	doc, active, err := c.d.Documents.Document(ctx, run)
	d := &docRead{active: active, err: err}
	if err != nil {
		c.h.d.Log.LogAttrs(ctx, slog.LevelWarn, msgLiveBoxRead, slog.String(logKeyErrorType, errorTypeLiveBox))
		d.active = false
	} else if active {
		d = c.encodeDocument(ctx, doc, rc)
	}
	docs[run] = d
	return d
}

// encodeDocument encodes the snapshot frame of doc: run.updated with snapshot and version and
// neither base_version nor ops; when that payload reaches sync.event_payload_max_bytes, the frame
// carries only run_id and version with the envelope member truncated, and the client loads getRun
// (04 section 2.2 rule 2).
func (c *Coalescer) encodeDocument(ctx context.Context, doc runs.Run, rc runChain) *docRead {
	snap, err := c.d.Encode(doc)
	if err != nil {
		c.h.d.Log.LogAttrs(ctx, slog.LevelError, msgLiveBoxEncode, slog.String(logKeyErrorType, errorTypeLiveBox))
		return &docRead{err: err}
	}
	frame, fits := c.frame(ctx, doc.ID, rc, runUpdatedBody{RunID: doc.ID, Version: doc.Version, Snapshot: snap}, false)
	if !fits {
		frame, _ = c.frame(ctx, doc.ID, rc, runUpdatedBody{RunID: doc.ID, Version: doc.Version}, true)
	}
	if frame == nil {
		return &docRead{err: errors.New("gateway: the live box frame cannot be encoded")}
	}
	return &docRead{frame: frame, version: doc.Version, active: true}
}

// frame encodes the dispatch frame of body for run: the envelope of an ephemeral event without
// seq and permission_version, resource {kind run, id, version} (ADR 0052, ADR 0053). fits is false
// when the payload reaches sync.event_payload_max_bytes and truncated is false.
func (c *Coalescer) frame(ctx context.Context, run ids.ID, rc runChain, body runUpdatedBody, truncated bool) ([]byte, bool) {
	payload, err := json.Marshal(body)
	if err != nil {
		c.h.d.Log.LogAttrs(ctx, slog.LevelError, msgLiveBoxEncode, slog.String(logKeyErrorType, errorTypeLiveBox))
		return nil, false
	}
	if !truncated && len(payload) >= c.h.cfg.PayloadMax {
		return nil, false
	}
	ev := sync.Event{
		Type: eventRunUpdated, SchemaVersion: runUpdatedSchema, OccurredAt: c.clock.Now(),
		SpaceID: rc.space, ChannelID: rc.channel, ThreadID: rc.thread,
		ResourceKind: resourceKindRun, ResourceID: run, ResourceVersion: body.Version, Payload: payload,
	}
	if rc.actor != nil {
		ev.ActorKind, ev.ActorPrincipalID = string(rc.actor.Kind), rc.actor.PrincipalID
	}
	env := ephemeralEnvelope(ev)
	env.Truncated = truncated
	frame, err := json.Marshal(outFrame{Op: opDispatch, D: env})
	if err != nil {
		c.h.d.Log.LogAttrs(ctx, slog.LevelError, msgLiveBoxEncode, slog.String(logKeyErrorType, errorTypeLiveBox))
		return nil, false
	}
	return frame, true
}
