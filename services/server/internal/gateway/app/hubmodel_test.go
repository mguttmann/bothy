// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"runtime"
	"slices"
	"strings"
	gosync "sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// TestHub_DeliveryModel is the class test of delivery across the phases of a connection
// (registered, catching up, live, closed; docs/DECISIONS.md M1-65/15). Seeded random runs
// interleave connect, catch-up pages, the log tail, channel creation, space events, personal
// events, grant, space grant and default operator changes and token revocation against the real
// Hub with the fakes of authz and sync. Per connection it checks:
//
//	(a) every event its principal is entitled to from the commit to the end of the run is
//	    delivered exactly once, and all frames carry seqs in log order;
//	(b) no event is queued without an entitlement at some point between its commit and the
//	    queueing, so nothing committed after a revocation is delivered;
//	(c) no frame moves the client's cursor past an undelivered event of (a).
//
// A failure prints the seed and the operations of the run.
func TestHub_DeliveryModel(t *testing.T) {
	t.Parallel()
	runs := 2000
	if testing.Short() {
		runs = 300
	}
	for i := range runs {
		seed := uint64(i)*1_000_003 + 11
		if failed := runModel(t, seed); failed {
			return
		}
	}
}

// modelConn is one connection of the model.
type modelConn struct {
	a        actor.Actor
	c        *app.Connection
	qc       *qconn
	from     int64
	attached bool
	detached bool

	mu      gosync.Mutex
	atGate  bool
	release chan struct{}
}

// model is the state of one run.
type model struct {
	t   *testing.T
	r   *rand.Rand
	w   *hubWorld
	h   *app.Hub
	ops []string

	spaces     []ids.ID
	channels   []ids.ID
	principals []actor.Actor
	conns      []*modelConn
	bySession  map[ids.ID]*modelConn

	// step counts the world changes; every event records its commit step.
	step     atomic.Int64
	tail     int
	commitAt []int64
	// history[i][p] are the entitlements of p to event i from its commit on, one entry per
	// change.
	history []map[ids.ID][]modelEnt
	// sessions guards bySession, which the catch-up runners read.
	sessions gosync.Mutex
}

// modelEnt is the entitlement of a principal to an event from step on.
type modelEnt struct {
	step int64
	ok   bool
}

func runModel(t *testing.T, seed uint64) (failed bool) {
	t.Helper()
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) //nolint:gosec // G404: a seeded generator makes a failing run reproducible.
	m := &model{t: t, r: r, w: newHubWorld(), bySession: map[ids.ID]*modelConn{}}
	cfg := app.HubConfig{QueueMax: 100000, PageSize: 1 + r.IntN(4), CursorEvery: 1 + r.IntN(4),
		RetryBase: time.Millisecond, RetryCap: time.Millisecond}
	h, err := app.NewHub(cfg, app.HubDeps{Authenticator: m.w, Authorizer: m.w, CatchUp: m.w, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	m.h = h
	m.w.gate = m.gate
	defer func() {
		for _, c := range m.conns {
			m.h.Detach(c.c)
		}
		if failed {
			t.Logf("seed %d, config %+v, operations:\n%s", seed, cfg, strings.Join(m.ops, "\n"))
		}
	}()
	m.setup()
	for range 40 + r.IntN(80) {
		m.randomOp()
		m.reap()
		if m.check(false) {
			return true
		}
	}
	m.finish()
	return m.check(true)
}

// setup creates the spaces, the first channels and the principals with random rights.
func (m *model) setup() {
	for range 2 + m.r.IntN(2) {
		m.spaces = append(m.spaces, ids.New())
	}
	for range 1 + m.r.IntN(3) {
		m.channels = append(m.channels, m.w.channel(m.spaces[m.r.IntN(len(m.spaces))]))
	}
	for i := range 3 {
		a := person()
		if i == 2 {
			a.Kind = actor.KindAgent
		}
		m.principals = append(m.principals, a)
		for _, c := range m.channels {
			if m.r.IntN(2) == 0 {
				m.w.grantChannel(a.ID, c)
			}
		}
	}
	if m.r.IntN(3) == 0 {
		a := person()
		m.w.instanceWide[a.ID] = true
		m.principals = append(m.principals, a)
	}
}

// randomOp runs one random operation.
func (m *model) randomOp() {
	switch n := m.r.IntN(100); {
	case n < 22:
		c := m.channels[m.r.IntN(len(m.channels))]
		m.commit(msg(0, m.spaceOf(c), c), "message in %s", short(c))
	case n < 26:
		s := m.spaces[m.r.IntN(len(m.spaces))]
		ev := msg(0, s, ids.ID{})
		ev.Type, ev.ResourceKind = "space.updated", "space"
		m.commit(ev, "space event in %s", short(s))
	case n < 30:
		p := m.principals[m.r.IntN(len(m.principals))].ID
		m.commit(personal(0, p), "personal event of %s", short(p))
	case n < 36:
		s := m.spaces[m.r.IntN(len(m.spaces))]
		c := ids.New()
		m.channels = append(m.channels, c)
		m.rights(func() { m.w.spaceOf[c] = s })
		ev := msg(0, s, c)
		ev.Type, ev.ResourceKind, ev.ResourceID = "channel.created", "channel", c
		m.commit(ev, "channel %s created in %s", short(c), short(s))
	case n < 43:
		p, c := m.principal(), m.channels[m.r.IntN(len(m.channels))]
		grant := m.r.IntN(2) == 0
		m.rightsChange(p, fmt.Sprintf("channel %s of %s grant=%v", short(c), short(p), grant), func() {
			if grant {
				m.w.channels[p] = setOr(m.w.channels[p], c, true)
			} else {
				delete(m.w.channels[p], c)
			}
		})
	case n < 46:
		p, s := m.principal(), m.spaces[m.r.IntN(len(m.spaces))]
		grant := m.r.IntN(2) == 0
		m.rightsChange(p, fmt.Sprintf("space %s of %s grant=%v", short(s), short(p), grant), func() {
			if grant {
				m.w.spaceGrants[p] = setOr(m.w.spaceGrants[p], s, true)
			} else {
				delete(m.w.spaceGrants[p], s)
			}
		})
	case n < 50:
		p, s := m.principal(), m.spaces[m.r.IntN(len(m.spaces))]
		grant := m.r.IntN(2) == 0
		m.rightsChange(p, fmt.Sprintf("operator of %s for %s=%v", short(s), short(p), grant), func() {
			if grant {
				m.w.operator[p] = setOr(m.w.operator[p], s, true)
			} else {
				delete(m.w.operator[p], s)
			}
		})
	case n < 51:
		p := m.principal()
		m.rights(func() { m.w.revoked[p] = true })
		m.logf("token of %s revoked (no access.changed)", short(p))
	case n < 72:
		for range 1 + m.r.IntN(3) {
			m.tailStep()
		}
	case n < 77:
		m.register()
	case n < 82:
		m.attach()
	case n < 91:
		if c := m.pick(); c != nil {
			m.logf("page step of conn %d", m.index(c))
			m.stepRunner(c)
		}
	default:
		if c := m.pick(); c != nil {
			m.logf("drain conn %d", m.index(c))
			c.qc.drain()
			m.settle(c)
		}
	}
}

// principal returns a random principal that is not instance-wide.
func (m *model) principal() ids.ID {
	for {
		a := m.principals[m.r.IntN(len(m.principals))]
		if !m.w.instanceWide[a.ID] {
			return a.ID
		}
	}
}

func (m *model) logf(format string, args ...any) {
	m.ops = append(m.ops, fmt.Sprintf("%3d tail=%d ", len(m.ops), m.tail)+fmt.Sprintf(format, args...))
}

func (m *model) spaceOf(c ids.ID) ids.ID {
	m.w.mu.Lock()
	defer m.w.mu.Unlock()
	return m.w.spaceOf[c]
}

// commit appends ev to the log at the next seq and records the entitlement of every principal.
func (m *model) commit(ev sync.Event, format string, args ...any) {
	m.w.mu.Lock()
	ev.Seq = int64(len(m.w.log) + 1)
	m.w.log = append(m.w.log, ev)
	m.w.mu.Unlock()
	step := m.step.Load()
	m.commitAt = append(m.commitAt, step)
	hist := map[ids.ID][]modelEnt{}
	for _, a := range m.principals {
		hist[a.ID] = []modelEnt{{step: step, ok: m.entitled(a.ID, ev)}}
	}
	m.history = append(m.history, hist)
	m.logf("commit %d: "+format, append([]any{ev.Seq}, args...)...)
}

// entitled is the admission rule of the world now, false for a revoked credential.
func (m *model) entitled(p ids.ID, ev sync.Event) bool {
	ok, err := m.w.Admit(context.Background(), actor.Actor{ID: p}, ev)
	return ok && err == nil
}

// rights changes the world under its lock and records the new entitlements.
func (m *model) rights(change func()) {
	m.w.mu.Lock()
	change()
	m.w.mu.Unlock()
	step := m.step.Add(1)
	m.w.mu.Lock()
	log := slices.Clone(m.w.log)
	m.w.mu.Unlock()
	for i, ev := range log {
		for _, a := range m.principals {
			h := m.history[i][a.ID]
			if ok := m.entitled(a.ID, ev); ok != h[len(h)-1].ok {
				m.history[i][a.ID] = append(h, modelEnt{step: step, ok: ok})
			}
		}
	}
}

// rightsChange changes the rights of p and commits access.changed with the difference of its
// visible sets, if there is one (04 section 8.2).
func (m *model) rightsChange(p ids.ID, what string, change func()) {
	before := m.visible(p)
	m.rights(change)
	after := m.visible(p)
	type target struct {
		Kind  string `json:"kind"`
		ID    ids.ID `json:"id"`
		Level string `json:"level,omitempty"`
	}
	payload := struct {
		PermissionVersion int64    `json:"permission_version"`
		Added             []target `json:"added"`
		Removed           []target `json:"removed"`
		Changed           []target `json:"changed"`
	}{PermissionVersion: m.step.Load(), Added: []target{}, Removed: []target{}, Changed: []target{}}
	for k, kind := range after {
		if _, ok := before[k]; !ok {
			payload.Added = append(payload.Added, target{Kind: kind.kind, ID: k, Level: "read"})
		}
	}
	for k, kind := range before {
		if _, ok := after[k]; !ok {
			payload.Removed = append(payload.Removed, target{Kind: kind.kind, ID: k})
		}
	}
	if len(payload.Added)+len(payload.Removed) == 0 {
		m.logf("%s (no visible difference, no access.changed)", what)
		return
	}
	b, _ := json.Marshal(payload)
	ev := sync.Event{ID: ids.New(), Type: "access.changed", SchemaVersion: 1, OccurredAt: t0, ResourceKind: "principal",
		ResourceID: p, ReceiveAction: "self", AudiencePrincipalID: p, Payload: b, PermissionVersion: m.step.Load()}
	m.commit(ev, "access.changed of %s: %s, added %d removed %d", short(p), what, len(payload.Added), len(payload.Removed))
}

// visibleKind is a visible target and its kind.
type visibleKind struct {
	kind string
}

// visible returns the visible spaces and channels of p as authz lists them.
func (m *model) visible(p ids.ID) map[ids.ID]visibleKind {
	out := map[ids.ID]visibleKind{}
	sc, err := m.w.Scope(context.Background(), actor.Actor{ID: p}, "")
	if err != nil {
		return out
	}
	for _, s := range sc.SpaceIDs {
		out[s] = visibleKind{"space"}
	}
	for _, c := range sc.ChannelIDs {
		out[c] = visibleKind{"channel"}
	}
	return out
}

// tailStep lets the log tail dispatch the next committed event.
func (m *model) tailStep() {
	m.w.mu.Lock()
	if m.tail >= len(m.w.log) {
		m.w.mu.Unlock()
		return
	}
	ev := m.w.log[m.tail]
	m.w.mu.Unlock()
	m.tail++
	m.logf("tail dispatches %d (%s)", ev.Seq, ev.Type)
	m.h.Dispatch(context.Background(), ev)
	m.reap()
}

// register opens a connection of a random principal (identify or resume up to Register).
func (m *model) register() {
	if len(m.conns) >= 6 {
		return
	}
	a := m.principals[m.r.IntN(len(m.principals))]
	a.SessionID, a.DeviceID = ids.New(), ids.New()
	qc := newQConn(4 + m.r.IntN(9))
	qc.stamp = m.step.Load
	c, s := app.NewTestConnection(qc, a)
	mc := &modelConn{a: a, c: c, qc: qc, release: make(chan struct{})}
	m.conns = append(m.conns, mc)
	m.sessions.Lock()
	m.bySession[a.SessionID] = mc
	m.sessions.Unlock()
	m.logf("register conn %d of %s", len(m.conns)-1, short(a.ID))
	m.h.Register(context.Background(), c, s)
}

// attach reads the head and attaches a registered connection with a cursor or a snapshot.
func (m *model) attach() {
	for _, c := range m.conns {
		if c.attached || c.detached {
			continue
		}
		m.w.mu.Lock()
		head := int64(len(m.w.log))
		m.w.mu.Unlock()
		c.from = head
		if m.r.IntN(3) > 0 {
			c.from = m.r.Int64N(head + 1)
		}
		c.attached = true
		m.logf("attach conn %d from %d head %d", m.index(c), c.from, head)
		if err := m.h.Attach(context.Background(), c.c, c.from, head); err != nil {
			m.t.Fatal(err)
		}
		m.settle(c)
		return
	}
}

// pick returns a random attached connection that is still open, or nil.
func (m *model) pick() *modelConn {
	var open []*modelConn
	for _, c := range m.conns {
		if c.attached && !c.detached {
			open = append(open, c)
		}
	}
	if len(open) == 0 {
		return nil
	}
	return open[m.r.IntN(len(open))]
}

func (m *model) index(c *modelConn) int { return slices.Index(m.conns, c) }

// gate holds the catch-up runner of a connection before each page until the model releases it.
func (m *model) gate(ctx context.Context) {
	a, _ := actor.FromContext(ctx)
	m.sessions.Lock()
	c := m.bySession[a.SessionID]
	m.sessions.Unlock()
	c.mu.Lock()
	c.atGate = true
	ch := c.release
	c.mu.Unlock()
	select {
	case <-ch:
	case <-ctx.Done():
	}
}

// stepRunner lets the runner of c read one page if it waits at the gate.
func (m *model) stepRunner(c *modelConn) {
	c.mu.Lock()
	if !c.atGate {
		c.mu.Unlock()
		return
	}
	c.atGate = false
	close(c.release)
	c.release = make(chan struct{})
	c.mu.Unlock()
	m.settle(c)
}

// settle waits until the runner of c stopped: at the gate, waiting for queue space, done (live)
// or closed.
func (m *model) settle(c *modelConn) {
	deadline := testkit.Deadline()
	for {
		c.mu.Lock()
		gated := c.atGate
		c.mu.Unlock()
		c.qc.mu.Lock()
		waiting, closed := c.qc.waiting, c.qc.closes > 0
		c.qc.mu.Unlock()
		if gated || waiting || closed || !c.attached || m.h.PhaseOf(c.c) != domain.PhaseCatchingUp {
			return
		}
		if time.Now().After(deadline) {
			m.t.Fatalf("runner of conn %d did not settle", m.index(c))
		}
		runtime.Gosched()
	}
}

// reap detaches the connections the hub closed, as the gateway does.
func (m *model) reap() {
	for i, c := range m.conns {
		if !c.detached && c.qc.closedWith() != 0 {
			c.detached = true
			m.logf("conn %d closed with %d", i, c.qc.closedWith())
			m.h.Detach(c.c)
		}
	}
}

// finish lets the tail reach the head, attaches every connection and runs every catch-up to its
// end with the writers draining.
func (m *model) finish() {
	m.logf("finish")
	m.w.mu.Lock()
	n := len(m.w.log)
	m.w.mu.Unlock()
	for m.tail < n {
		m.tailStep()
	}
	for range m.conns {
		m.attach()
	}
	for range 100000 {
		busy := false
		for _, c := range m.conns {
			if c.detached {
				continue
			}
			c.qc.drain()
			m.settle(c)
			m.stepRunner(c)
			m.reap()
			if !c.detached && (m.h.PhaseOf(c.c) == domain.PhaseCatchingUp || c.qc.len() > 0) {
				busy = true
			}
		}
		if !busy {
			return
		}
	}
	m.t.Fatal("catch-up did not end")
}

// entitledBetween reports whether p was entitled to event i at some step from its commit up to
// step to.
func (m *model) entitledBetween(i int, p ids.ID, to int64) bool {
	for _, e := range m.history[i][p] {
		if e.step > to {
			return false
		}
		if e.ok {
			return true
		}
	}
	return false
}

// must reports whether p is entitled to event i from its commit to the end of the run.
func (m *model) must(i int, p ids.ID) bool {
	for _, e := range m.history[i][p] {
		if !e.ok {
			return false
		}
	}
	return true
}

// check verifies (a), (b) and (c) on the frames written so far; final also requires every event
// of (a) on the connections that ended live.
func (m *model) check(final bool) bool {
	m.t.Helper()
	for i, c := range m.conns {
		if !c.attached {
			continue
		}
		c.qc.mu.Lock()
		out := slices.Clone(c.qc.out)
		offered := make(map[int64]int64, len(c.qc.offeredAt))
		for k, v := range c.qc.offeredAt {
			offered[k] = v
		}
		c.qc.mu.Unlock()
		fs := framesOf(m.t, out)
		last := c.from
		got := map[int64]bool{}
		for _, f := range fs {
			switch f.op {
			case "dispatch":
				if f.seq <= last {
					m.t.Errorf("conn %d: dispatch %d after a frame with seq %d (duplicate or out of log order); frames %v", i, f.seq, last, fs)
					return true
				}
				got[f.seq] = true
				if !m.entitledBetween(int(f.seq-1), c.a.ID, offered[f.seq]) {
					m.t.Errorf("conn %d: dispatch %d queued at step %d without an entitlement since its commit (step %d); frames %v",
						i, f.seq, offered[f.seq], m.commitAt[f.seq-1], fs)
					return true
				}
			default:
				if f.seq < last {
					m.t.Errorf("conn %d: %s %d after a frame with seq %d; frames %v", i, f.op, f.seq, last, fs)
					return true
				}
			}
			last = max(last, f.seq)
		}
		upTo := last
		if final && c.qc.closedWith() == 0 {
			upTo = int64(len(m.history))
		}
		for s := c.from + 1; s <= upTo; s++ {
			if m.must(int(s-1), c.a.ID) && !got[s] {
				m.t.Errorf("conn %d of %s: event %d, entitled since its commit, never delivered, written up to seq %d; frames %v",
					i, short(c.a.ID), s, last, fs)
				return true
			}
		}
	}
	return false
}

func short(id ids.ID) string {
	s := id.String()
	return s[len(s)-4:]
}

func setOr(set map[ids.ID]bool, k ids.ID, v bool) map[ids.ID]bool {
	if set == nil {
		set = map[ids.ID]bool{}
	}
	set[k] = v
	return set
}
