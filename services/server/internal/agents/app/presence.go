// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	gosync "sync"
	"sync/atomic"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// PresenceConfig holds the configuration of the presence tracker, bound in internal/wire
// (wire.PresenceConfig).
type PresenceConfig struct {
	// PresenceTimeout is agent.presence_timeout: without a heartbeat for this long an agent is
	// offline.
	PresenceTimeout time.Duration `config:"agent.presence_timeout"`
	// PersistInterval is agent.heartbeat_persist_interval: heartbeat fields are stored at most
	// once per interval.
	PersistInterval time.Duration `config:"agent.heartbeat_persist_interval"`
	// MismatchAlertAfter is billing.mismatch_alert_after: the delay of the warning
	// billing_mode_mismatch and the threshold of the alarm.
	MismatchAlertAfter time.Duration `config:"billing.mismatch_alert_after"`
	// HeartbeatInterval is sync.heartbeat_interval, the interval_ms of HeartbeatAck.
	HeartbeatInterval time.Duration `config:"sync.heartbeat_interval"`
}

// Timers runs a function after a duration; the real one is time.AfterFunc, the tests drive a fake
// (the pattern of terminal/app).
type Timers interface {
	// AfterFunc calls f in its own goroutine after d; stop cancels it and reports whether it had
	// not run yet.
	AfterFunc(d time.Duration, f func()) (stop func() bool)
}

// RealTimers is Timers over time.AfterFunc.
type RealTimers struct{}

// AfterFunc implements Timers.
func (RealTimers) AfterFunc(d time.Duration, f func()) func() bool { return time.AfterFunc(d, f).Stop }

// Readiness is the part of chat.ProjectBindings the heartbeat applies project readiness with
// (M2-03 note 7).
type Readiness interface {
	ApplyProjectReadiness(ctx context.Context, agentID ids.ID, reports []chat.ProjectReadiness) error
}

// Principals is the part of identity.Directory the tracker reads handle and status with.
type Principals interface {
	Principal(ctx context.Context, id ids.ID) (identity.PrincipalView, error)
	Principals(ctx context.Context, ids []ids.ID) (map[ids.ID]identity.PrincipalView, error)
}

// PresenceDeps are the collaborators of the tracker. Publisher and Readiness may be bound later
// (BindPublisher, BindReadiness), because the gateway and chat are built after agents; they must
// be bound before the first heartbeat.
type PresenceDeps struct {
	Store      PresenceStore
	Principals Principals
	UoW        txn.UnitOfWork
	Authorizer authorize.Authorizer
	// Audit records the change of the reported mode or class inside the unit of work.
	Audit     auditing.Recorder
	Publisher eventlog.EphemeralPublisher
	Readiness Readiness
	// Clock nil means clock.System(); Timers nil means RealTimers; Log nil means slog.Default().
	Clock  clock.Clock
	Timers Timers
	Log    *slog.Logger
}

// Tracker is the process-wide presence of agents (brief M2-20 notes 1 to 8, docs/DECISIONS.md
// D-M2-20-1): one entry per agent with its lock, the last report, the presence state, a deadline
// timer, the last published pair and the start times of a difference between configured and
// reported mode. It implements agents.Presence and is safe for concurrent use. Only the roles api
// and all start it (Start), because only they receive heartbeats.
type Tracker struct {
	cfg PresenceConfig
	d   PresenceDeps

	mu        gosync.Mutex
	entries   map[ids.ID]*entry
	publisher eventlog.EphemeralPublisher
	readiness Readiness
	stopFlush func() bool

	// stopped is set by the stop step: timers stop and nothing is published any more.
	stopped atomic.Bool
}

// Compile-time check: the tracker is the incoming port.
var _ agents.Presence = (*Tracker)(nil)

// NewTracker returns the tracker. Errors: a missing collaborator, a window below 1ns or
// agent.presence_timeout not above sync.heartbeat_interval.
func NewTracker(cfg PresenceConfig, d PresenceDeps) (*Tracker, error) {
	if d.Store == nil || d.Principals == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil {
		return nil, errors.New("agents: NewTracker needs every collaborator")
	}
	if cfg.PresenceTimeout <= 0 || cfg.PersistInterval <= 0 || cfg.MismatchAlertAfter <= 0 || cfg.HeartbeatInterval <= 0 {
		return nil, errors.New("agents: the presence windows must be positive")
	}
	if cfg.PresenceTimeout <= cfg.HeartbeatInterval {
		return nil, errors.New("agents: agent.presence_timeout must be greater than sync.heartbeat_interval")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Timers == nil {
		d.Timers = RealTimers{}
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Tracker{cfg: cfg, d: d, entries: map[ids.ID]*entry{}, publisher: d.Publisher, readiness: d.Readiness}, nil
}

// BindPublisher binds the ephemeral publisher of the process (the hub of the gateway, M1-66).
func (t *Tracker) BindPublisher(p eventlog.EphemeralPublisher) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.publisher = p
}

// BindReadiness binds the readiness port of chat (M2-03).
func (t *Tracker) BindReadiness(r Readiness) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.readiness = r
}

// Interval returns sync.heartbeat_interval, the interval_ms of HeartbeatAck (D-M2-20-11).
func (t *Tracker) Interval() time.Duration { return t.cfg.HeartbeatInterval }

// bound returns the late-bound collaborators.
func (t *Tracker) bound() (eventlog.EphemeralPublisher, Readiness) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.publisher, t.readiness
}

// entry is the presence of one agent in this process. mu guards every field except readiness
// and readinessSet and is never held across a database call; beat runs the heartbeats of the
// agent one after the other and guards readiness and readinessSet (D-M2-21-11). loaded is false
// until the stored row has filled it.
type entry struct {
	beat   gosync.Mutex
	mu     gosync.Mutex
	id     ids.ID
	handle string
	active bool
	loaded bool

	target   domain.Target
	report   domain.Report
	state    domain.RuntimeState
	last     time.Time
	deadline time.Time
	dirty    bool

	// pubState and pubMode are the last published pair (initially the stored values).
	pubState domain.RuntimeState
	pubMode  domain.BillingMode
	// anySince is the start of a difference of any kind, modeSince of kind (a) (note 8).
	anySince, modeSince time.Time

	// readiness is the last project readiness list this process applied; readinessSet tells an
	// applied empty list from none.
	readiness    []domain.Readiness
	readinessSet bool

	stop func() bool
	gen  uint64
	// changes counts the calls of Changed, so a heartbeat keeps configured values newer than its
	// read.
	changes uint64
}

// lookup returns the entry of id, creating it with handle and status from the directory.
func (t *Tracker) lookup(ctx context.Context, id ids.ID) (*entry, error) {
	t.mu.Lock()
	e, ok := t.entries[id]
	t.mu.Unlock()
	if ok {
		return e, nil
	}
	p, err := t.d.Principals.Principal(ctx, id)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.entries[id]; ok {
		return e, nil
	}
	e = &entry{id: id, handle: p.Handle, active: p.Status == identity.StatusActive}
	t.entries[id] = e
	return e, nil
}

// tracked returns the entry of id when it is loaded.
func (t *Tracker) tracked(id ids.ID) *entry {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.entries[id]
}

// fill sets the entry from its stored row (the caller holds e.mu): the configured values, the
// stored report and state, and the stored values as the last published pair. A row stored as
// present by another process gets the deadline now plus agent.presence_timeout, as at start.
func (t *Tracker) fill(e *entry, row domain.PresenceRow, now time.Time) {
	e.target, e.report, e.last, e.loaded = row.Target, row.Stored, row.LastHeartbeatAt, true
	e.state = row.Stored.State
	if e.state == "" {
		e.state = domain.RuntimeUnknown
	}
	e.pubState, e.pubMode = e.state, row.Stored.BillingMode
	if e.state.Present() {
		e.deadline = now.Add(t.cfg.PresenceTimeout)
		t.arm(e, now)
	}
	t.reassess(e, now)
}

// reassess updates the start times of the differences of note 8 at now (the caller holds e.mu):
// they begin with the first heartbeat that shows the difference and end with its cause, with
// offline or with unknown.
func (t *Tracker) reassess(e *entry, now time.Time) {
	as := domain.Assess(e.target, e.report, e.state)
	e.anySince = since(e.anySince, as.Differs(), now)
	e.modeSince = since(e.modeSince, as.ModeDiffers, now)
}

// since keeps start while cond holds, starts it at now when it begins and clears it otherwise.
func since(start time.Time, cond bool, now time.Time) time.Time {
	switch {
	case !cond:
		return time.Time{}
	case start.IsZero():
		return now
	}
	return start
}

// arm (re)starts the deadline timer of e (the caller holds e.mu). An older timer that fires
// anyway sees a newer generation and does nothing.
func (t *Tracker) arm(e *entry, now time.Time) {
	if e.stop != nil {
		e.stop()
	}
	e.gen++
	gen := e.gen
	e.stop = t.d.Timers.AfterFunc(max(e.deadline.Sub(now), 0), func() { t.expire(e, gen) })
}

// expire is the deadline timer of e: under the entry lock and only when no newer heartbeat
// re-armed it, the agent becomes offline, the entry dirty and the mismatch times end; then the
// change is published without an actor (D-M2-20-4). A timer that fires early re-arms itself.
func (t *Tracker) expire(e *entry, gen uint64) {
	if t.stopped.Load() {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if gen != e.gen || !e.state.Present() {
		return
	}
	now := t.d.Clock.Now()
	if now.Before(e.deadline) {
		t.arm(e, now)
		return
	}
	e.state, e.dirty, e.stop = domain.RuntimeOffline, true, nil
	t.reassess(e, now)
	t.publish(context.Background(), e, nil)
}

// publish sends agent.presence_changed when state or reported mode differ from the last
// published pair (the caller holds e.mu; D-M2-20-4). The envelope has an empty scope, the
// resource agent without version, no audience and the payload AgentPresencePayload; the hub
// admits receivers with agent.read in delivery mode, never before catchup.complete. A publish
// error is logged and never fails the heartbeat.
func (t *Tracker) publish(ctx context.Context, e *entry, by *ids.ID) {
	if t.stopped.Load() || (e.state == e.pubState && e.report.BillingMode == e.pubMode) {
		return
	}
	pub, _ := t.bound()
	e.pubState, e.pubMode = e.state, e.report.BillingMode
	if pub == nil {
		return
	}
	ev := eventlog.EphemeralEvent{
		Type: string(authorize.EventAgentPresenceChanged), SchemaVersion: 1,
		Resource: eventlog.ResourceRef{Kind: resourceAgent, ID: e.id},
		Payload:  presencePayload{AgentID: e.id.String(), State: string(e.state), ReportedBillingMode: string(e.report.BillingMode)},
	}
	if by != nil {
		ev.Actor = &eventlog.ActorRef{Kind: eventlog.ActorAgent, PrincipalID: *by}
	}
	if err := pub.PublishEphemeral(ctx, ev); err != nil {
		t.d.Log.LogAttrs(ctx, slog.LevelError, "agent.presence_changed not published", slog.String(logKeyAgentID, e.id.String()))
	}
}

// logKeyAgentID is the log key of the agent of a failed publish (signals.md 7: IDs only in logs
// and spans).
const logKeyAgentID = "bothy.agent.id"

// presencePayload is the schema AgentPresencePayload; the last reported mode stays while offline
// and is left out when not reported (ADR 0053: no null).
type presencePayload struct {
	AgentID             string `json:"agent_id"`
	State               string `json:"state"`
	ReportedBillingMode string `json:"reported_billing_mode,omitempty"`
}

// status converts e at now (the caller holds e.mu).
func (t *Tracker) status(e *entry, now time.Time) agents.PresenceStatus {
	st := statusOf(e.id, e.target, e.report, e.state, e.last)
	if e.state.Present() {
		st.Warnings = domain.Assess(e.target, e.report, e.state).Warnings(e.modeSince, now, t.cfg.MismatchAlertAfter)
	}
	return st
}

// statusOf converts a report without the delayed warning.
func statusOf(id ids.ID, target domain.Target, r domain.Report, state domain.RuntimeState, last time.Time) agents.PresenceStatus {
	if state == "" {
		state = domain.RuntimeUnknown
	}
	st := agents.PresenceStatus{
		AgentID: id, State: string(state), ReportedBillingMode: string(r.BillingMode), RuntimeVersion: r.RuntimeVersion,
		LastHeartbeatAt: last, ReportedToolClass: string(r.ToolClass), BillingModeMatches: r.BillingMode == "" || r.BillingMode == target.BillingMode,
	}
	for _, m := range r.BillingModesSupported {
		st.BillingModesSupported = append(st.BillingModesSupported, string(m))
	}
	if r.Evidence != nil {
		st.BillingEvidence = &agents.BillingEvidence{Provider: r.Evidence.Provider, KeyType: r.Evidence.KeyType, Method: r.Evidence.Method}
	}
	return st
}

// Statuses implements agents.Presence: loaded entries from memory, the other agents from their
// stored row as stored (unknown or offline in one process), in the order of agentIDs.
func (t *Tracker) Statuses(ctx context.Context, agentIDs []ids.ID) ([]agents.PresenceStatus, error) {
	now := t.d.Clock.Now()
	found := make(map[ids.ID]agents.PresenceStatus, len(agentIDs))
	var rest []ids.ID
	for _, id := range agentIDs {
		if st, ok := t.memory(id, now); ok {
			found[id] = st
			continue
		}
		rest = append(rest, id)
	}
	rows, err := t.d.Store.GetPresenceRows(ctx, rest)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		found[row.ID] = storedStatus(row, now, t.cfg.MismatchAlertAfter)
	}
	out := make([]agents.PresenceStatus, 0, len(agentIDs))
	for _, id := range agentIDs {
		if st, ok := found[id]; ok {
			out = append(out, st)
		}
	}
	return out, nil
}

// memory returns the status of the loaded entry of id. The entry lock is never held across a
// database call (D-M2-21-11), so this read inside a unit that holds the agents row does not wait
// for a heartbeat.
func (t *Tracker) memory(id ids.ID, now time.Time) (agents.PresenceStatus, bool) {
	e := t.tracked(id)
	if e == nil {
		return agents.PresenceStatus{}, false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.loaded {
		return agents.PresenceStatus{}, false
	}
	return t.status(e, now), true
}

// storedStatus is the status of an untracked row: the immediate warnings only, because this
// process does not know when a difference began.
func storedStatus(row domain.PresenceRow, now time.Time, alertAfter time.Duration) agents.PresenceStatus {
	st := statusOf(row.ID, row.Target, row.Stored, row.Stored.State, row.LastHeartbeatAt)
	if row.Stored.State.Present() {
		st.Warnings = domain.Assess(row.Target, row.Stored, row.Stored.State).Warnings(time.Time{}, now, alertAfter)
	}
	return st
}

// recordStatus is the presence of the record rec for getAgent and listAgents: the entry from
// memory when this process tracks the agent, otherwise the stored columns of rec.
func (t *Tracker) recordStatus(rec domain.Agent) agents.PresenceStatus {
	now := t.d.Clock.Now()
	if st, ok := t.memory(rec.ID, now); ok {
		return st
	}
	return recordPresence(rec, now, t.cfg.MismatchAlertAfter)
}

// recordPresence converts the stored reported state of rec.
func recordPresence(rec domain.Agent, now time.Time, alertAfter time.Duration) agents.PresenceStatus {
	r := rec.Reported
	stored := domain.Report{State: r.State, RuntimeVersion: r.RuntimeVersion, BillingMode: r.BillingMode,
		BillingModesSupported: r.BillingModesSupported, Evidence: r.Evidence, ToolClass: r.ToolClass}
	return storedStatus(domain.PresenceRow{ID: rec.ID, Target: domain.Target{BillingMode: rec.Policy.Mode, ToolClass: rec.ToolClass},
		Stored: stored, LastHeartbeatAt: r.LastHeartbeatAt}, now, alertAfter)
}

// Changed takes the configured values and the status of the tracked agents among list again after
// a committed SetBillingMode, SetBillingModeAll, SetToolClass, Disable or Enable (note 13) and
// reassesses them. A disabled agent becomes inactive (no metric series) while its deadline keeps
// running. Nothing is published: M1-51 already appends agent.updated. Errors: the reads'.
func (t *Tracker) Changed(ctx context.Context, list []ids.ID) error {
	var mine []ids.ID
	for _, id := range list {
		if t.tracked(id) != nil {
			mine = append(mine, id)
		}
	}
	if len(mine) == 0 {
		return nil
	}
	rows, err := t.d.Store.GetPresenceRows(ctx, mine)
	if err != nil {
		return err
	}
	principals, err := t.d.Principals.Principals(ctx, mine)
	if err != nil {
		return err
	}
	now := t.d.Clock.Now()
	for _, row := range rows {
		e := t.tracked(row.ID)
		e.mu.Lock()
		e.changes++
		if e.loaded {
			e.target = row.Target
			if p, ok := principals[row.ID]; ok {
				e.active = p.Status == identity.StatusActive
			}
			t.reassess(e, now)
		}
		e.mu.Unlock()
	}
	return nil
}

// GaugeSample is one agent of the presence gauges (signals.md 4.3): the handle, the age of the
// last heartbeat (HasHeartbeat false without one) and the age of a difference of any kind of note
// 8 (0 without one).
type GaugeSample struct {
	Handle       string
	HasHeartbeat bool
	HeartbeatAge time.Duration
	MismatchAge  time.Duration
}

// Samples returns the gauge values of every active tracked agent at the current time, in
// ascending agent id. It reads only memory and never blocks on the database (17 rule 4); a
// disabled agent has no series.
func (t *Tracker) Samples() []GaugeSample {
	now := t.d.Clock.Now()
	var out []GaugeSample
	for _, e := range t.sorted() {
		e.mu.Lock()
		if e.loaded && e.active {
			s := GaugeSample{Handle: e.handle, HasHeartbeat: !e.last.IsZero()}
			if s.HasHeartbeat {
				s.HeartbeatAge = max(now.Sub(e.last), 0)
			}
			if !e.anySince.IsZero() {
				s.MismatchAge = max(now.Sub(e.anySince), 0)
			}
			out = append(out, s)
		}
		e.mu.Unlock()
	}
	return out
}

// sorted returns the entries in ascending agent id.
func (t *Tracker) sorted() []*entry {
	t.mu.Lock()
	out := make([]*entry, 0, len(t.entries))
	for _, e := range t.entries {
		out = append(out, e)
	}
	t.mu.Unlock()
	slices.SortFunc(out, func(a, b *entry) int { return bytes.Compare(a.id[:], b.id[:]) })
	return out
}
