// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"log/slog"
	"maps"
	"slices"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// sessDefaultCurrency is the column default of agent_sessions.currency (03).
const sessDefaultCurrency = "USD"

// sessRefMax is agent.session_ref_max_bytes in these tests.
const sessRefMax = 32

// sessStore is agent_sessions in memory with the semantics of the queries: the partial unique
// indexes (thread_id) WHERE is_current and (agent_id, external_session_ref) WHERE is_current, the
// version compare-and-swap of end and update, and ON CONFLICT DO NOTHING on insert.
type sessStore struct {
	mu   gosync.Mutex
	rows map[ids.ID]agents.Session
	// locked records the ID lists of LockSessions in call order.
	locked [][]ids.ID
	// beforeInsert runs before an insert is decided, outside the mutex; a test uses it to play a
	// concurrent transaction that commits a row first.
	beforeInsert func(app.NewSession)
	// beforeLock runs once before the next LockSessions, outside the mutex; a test uses it to end
	// a row between the unlocked read and the lock.
	beforeLock func()
}

func newSessStore() *sessStore { return &sessStore{rows: map[ids.ID]agents.Session{}} }

// put stores row as it is (fixtures).
func (f *sessStore) put(row agents.Session) agents.Session {
	f.mu.Lock()
	defer f.mu.Unlock()
	if row.Version == 0 {
		row.Version = 1
	}
	if row.Currency == "" {
		row.Currency = sessDefaultCurrency
	}
	f.rows[row.ID] = row
	return row
}

// get returns the stored row id.
func (f *sessStore) get(id ids.ID) (agents.Session, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	return r, ok
}

// all returns every row ordered by id.
func (f *sessStore) all() []agents.Session {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := slices.Collect(maps.Values(f.rows))
	slices.SortFunc(out, func(a, b agents.Session) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	return out
}

// snapshot and restore give the unit of work its rollback.
func (f *sessStore) snapshot() map[ids.ID]agents.Session {
	f.mu.Lock()
	defer f.mu.Unlock()
	return maps.Clone(f.rows)
}

func (f *sessStore) restore(rows map[ids.ID]agents.Session) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = rows
}

// newestFirst orders by created_at DESC, id DESC.
func newestFirst(a, b agents.Session) int {
	if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
		return c
	}
	return bytes.Compare(b.ID[:], a.ID[:])
}

func (f *sessStore) ThreadSessions(_ context.Context, threadID ids.ID) ([]agents.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []agents.Session
	for _, r := range f.rows {
		if r.ThreadID == threadID {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, newestFirst)
	return out, nil
}

func (f *sessStore) current(threadID ids.ID) (agents.Session, bool) {
	for _, r := range f.rows {
		if r.ThreadID == threadID && r.IsCurrent {
			return r, true
		}
	}
	return agents.Session{}, false
}

func (f *sessStore) CurrentSession(_ context.Context, threadID ids.ID) (agents.Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.current(threadID)
	return r, ok, nil
}

func (f *sessStore) LockCurrentSession(ctx context.Context, threadID ids.ID) (agents.Session, bool, error) {
	return f.CurrentSession(ctx, threadID)
}

func (f *sessStore) LockSessions(_ context.Context, want []ids.ID) ([]agents.Session, error) {
	if f.beforeLock != nil {
		hook := f.beforeLock
		f.beforeLock = nil
		hook()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.locked = append(f.locked, slices.Clone(want))
	var out []agents.Session
	for _, id := range want {
		if r, ok := f.rows[id]; ok {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b agents.Session) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	return out, nil
}

func (f *sessStore) currentByRef(agentID ids.ID, ref string) (agents.Session, bool) {
	for _, r := range f.rows {
		if r.IsCurrent && r.AgentID == agentID && r.ExternalSessionRef == ref {
			return r, true
		}
	}
	return agents.Session{}, false
}

func (f *sessStore) CurrentByRef(_ context.Context, agentID ids.ID, ref string) (agents.Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.currentByRef(agentID, ref)
	return r, ok, nil
}

// latest returns the newest row that keep accepts.
func (f *sessStore) latest(keep func(agents.Session) bool) (agents.Session, bool) {
	var out []agents.Session
	for _, r := range f.rows {
		if keep(r) {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return agents.Session{}, false
	}
	slices.SortFunc(out, newestFirst)
	return out[0], true
}

func (f *sessStore) LatestByRef(_ context.Context, agentID ids.ID, ref string) (agents.Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.latest(func(r agents.Session) bool { return r.AgentID == agentID && r.ExternalSessionRef == ref })
	return r, ok, nil
}

func (f *sessStore) LatestInThreadByRef(_ context.Context, threadID, agentID ids.ID, ref string) (agents.Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.latest(func(r agents.Session) bool {
		return r.ThreadID == threadID && r.AgentID == agentID && r.ExternalSessionRef == ref
	})
	return r, ok, nil
}

func (f *sessStore) InsertSession(_ context.Context, s app.NewSession) (agents.Session, bool, error) {
	if f.beforeInsert != nil {
		hook := f.beforeInsert
		f.beforeInsert = nil
		hook(s)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if s.IsCurrent {
		if _, taken := f.current(s.ThreadID); taken {
			return agents.Session{}, false, nil
		}
		if _, taken := f.currentByRef(s.AgentID, s.ExternalSessionRef); taken {
			return agents.Session{}, false, nil
		}
	}
	row := agents.Session{
		ID: s.ID, ThreadID: s.ThreadID, AgentID: s.AgentID, ExternalSessionRef: s.ExternalSessionRef, IsCurrent: s.IsCurrent,
		Started: s.Started, Model: s.Model, CostBaselineMicros: s.CostBaselineMicros, Currency: s.Currency,
		CreatedAt: s.CreatedAt, EndedAt: s.EndedAt, EndReason: s.EndReason, Version: s.Version,
	}
	if row.Version == 0 {
		row.Version = 1
	}
	if row.Currency == "" {
		row.Currency = sessDefaultCurrency
	}
	f.rows[row.ID] = row
	return row, true, nil
}

func (f *sessStore) EndSession(_ context.Context, e app.SessionEnd) (agents.Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[e.ID]
	if !ok || !r.IsCurrent || r.Version != e.Version {
		return agents.Session{}, false, nil
	}
	r.IsCurrent, r.EndedAt, r.EndReason, r.Version = false, e.At, e.Reason, r.Version+1
	f.rows[r.ID] = r
	return r, true, nil
}

func (f *sessStore) UpdateSession(_ context.Context, c app.SessionChange) (agents.Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[c.ID]
	if !ok || r.Version != c.Version {
		return agents.Session{}, false, nil
	}
	if r.IsCurrent {
		if other, taken := f.currentByRef(r.AgentID, c.ExternalSessionRef); taken && other.ID != r.ID {
			return agents.Session{}, false, app.ErrSessionRefTaken
		}
	}
	r.ExternalSessionRef, r.Model, r.Started, r.Version = c.ExternalSessionRef, c.Model, c.Started, r.Version+1
	f.rows[r.ID] = r
	return r, true, nil
}

func (f *sessStore) SetSessionCost(_ context.Context, c app.SessionCost) (agents.Session, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[c.ID]
	if !ok || r.Version != c.Version {
		return agents.Session{}, false, nil
	}
	r.CumulativeCostMicros, r.Currency, r.Version = c.CumulativeCostMicros, c.Currency, r.Version+1
	f.rows[r.ID] = r
	return r, true, nil
}

// sessThreads is chat.Threads.Info over a map; an unknown thread is not_found.
type sessThreads map[ids.ID]chat.ThreadInfo

func (t sessThreads) Info(_ context.Context, id ids.ID) (chat.ThreadInfo, error) {
	info, ok := t[id]
	if !ok {
		return chat.ThreadInfo{}, errs.New(errs.CodeNotFound, "")
	}
	return info, nil
}

// sessChannels is chat.TriggerFacts with the effective operator per channel.
type sessChannels map[ids.ID]ids.ID

func (c sessChannels) TriggerChannel(_ context.Context, id ids.ID) (chat.TriggerChannel, error) {
	op, ok := c[id]
	if !ok {
		return chat.TriggerChannel{}, errs.New(errs.CodeNotFound, "")
	}
	return chat.TriggerChannel{ChannelID: id, Kind: "text", EffectiveOperatorID: op}, nil
}

// sessCheck is one call of the authorizer.
type sessCheck struct {
	Principal ids.ID
	Action    authorize.Action
	Resource  authorize.Resource
}

// sessAuthorizer allows every action unless deny names a decision for it; it records the calls.
type sessAuthorizer struct {
	mu     gosync.Mutex
	deny   map[authorize.Action]authorize.Decision
	checks []sessCheck
}

func (z *sessAuthorizer) Authorize(_ context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	z.mu.Lock()
	defer z.mu.Unlock()
	var who ids.ID
	if act, ok := p.(actor.Actor); ok {
		who = act.ID
	}
	z.checks = append(z.checks, sessCheck{Principal: who, Action: a, Resource: r})
	if d, ok := z.deny[a]; ok {
		return d, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
}

func (z *sessAuthorizer) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

func (z *sessAuthorizer) Observe(context.Context, authorize.LogEvent) {}

// sessAudit is one entry of the recorder with the actor of its context.
type sessAudit struct {
	Entry auditing.Entry
	Actor actor.Actor
}

// sessRecorder records the allow entries the use cases write; denials belong to authz.
type sessRecorder struct {
	mu      gosync.Mutex
	entries []sessAudit
}

func (r *sessRecorder) Record(ctx context.Context, e auditing.Entry) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "Record outside a unit of work")
	}
	a, _ := actor.FromContext(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, sessAudit{Entry: e, Actor: a})
	return nil
}

func (r *sessRecorder) RecordDenial(context.Context, auditing.Entry) error {
	return errs.New(errs.CodeInternalError, "the session use cases record no denial")
}

func (r *sessRecorder) RecordDetached(context.Context, auditing.Entry) error {
	return errs.New(errs.CodeInternalError, "the session use cases record nothing detached")
}

// sessEvents records the appended durable events in order.
type sessEvents struct {
	mu  gosync.Mutex
	evs []eventlog.Event
}

func (e *sessEvents) Append(ctx context.Context, evs ...eventlog.Event) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "Append outside a unit of work")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.evs = append(e.evs, evs...)
	return nil
}

// sessRuns is the declared RunSource with at most one active run per thread.
type sessRuns struct {
	mu   gosync.Mutex
	runs map[ids.ID]agents.ActiveRun
}

func (r *sessRuns) ActiveRun(_ context.Context, threadID ids.ID) (agents.ActiveRun, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[threadID]
	return run, ok, nil
}

// sessAttached records the SessionAttached events that reach the subscriber of runs.
type sessAttached struct {
	mu  gosync.Mutex
	evs []agents.SessionAttached
}

// OnSessionAttached is the subscriber the test registers in place of runs.OnSessionAttached.
func (s *sessAttached) OnSessionAttached(_ context.Context, ev agents.SessionAttached) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, ev)
	return nil
}

// sessUoW runs fn in a unit of the sealed registry in PhaseWork, so txn.InUnit holds and
// txn.Publish reaches the subscribers; an inner Do joins the ambient unit. An error of fn rolls
// back the store, the events, the audit entries and the published events, as a transaction would.
type sessUoW struct {
	reg      *txn.Registry
	store    *sessStore
	events   *sessEvents
	audit    *sessRecorder
	attached *sessAttached
}

func (u *sessUoW) Do(ctx context.Context, fn func(ctx context.Context) error, opts ...txn.Option) error {
	if txn.InUnit(ctx) {
		return fn(ctx)
	}
	unit, setPhase := txn.NewUnit(u.reg, txn.ApplyOptions(opts...))
	setPhase(txn.PhaseWork)
	rows := u.store.snapshot()
	u.events.mu.Lock()
	nEvents := len(u.events.evs)
	u.events.mu.Unlock()
	u.audit.mu.Lock()
	nAudit := len(u.audit.entries)
	u.audit.mu.Unlock()
	u.attached.mu.Lock()
	nAttached := len(u.attached.evs)
	u.attached.mu.Unlock()
	err := fn(txn.WithUnit(ctx, unit))
	if err != nil {
		u.store.restore(rows)
		u.events.mu.Lock()
		u.events.evs = u.events.evs[:nEvents]
		u.events.mu.Unlock()
		u.audit.mu.Lock()
		u.audit.entries = u.audit.entries[:nAudit]
		u.audit.mu.Unlock()
		u.attached.mu.Lock()
		u.attached.evs = u.attached.evs[:nAttached]
		u.attached.mu.Unlock()
	}
	return err
}

// sessEnv is SessionService over the fakes: a space with a text channel Support whose effective
// operator is Lotse, the main thread of Support and a thread of kind thread operated by Lotse,
// a second thread of kind thread operated by Lotse for take-overs, a thread without operator,
// the people Alex and Sam and the agents Lotse and Runner.
type sessEnv struct {
	t        *testing.T
	svc      *app.SessionService
	store    *sessStore
	threads  sessThreads
	channels sessChannels
	runs     *sessRuns
	az       *sessAuthorizer
	audit    *sessRecorder
	events   *sessEvents
	attached *sessAttached
	uow      *sessUoW
	clk      *clock.Fake
	log      *bytes.Buffer

	space, support            ids.ID
	main, thread, other, noOp ids.ID
	alex, sam                 actor.Actor
	lotse, runner             actor.Actor
}

func newSessEnv(t *testing.T) *sessEnv {
	t.Helper()
	e := &sessEnv{
		t: t, store: newSessStore(), threads: sessThreads{}, channels: sessChannels{}, runs: &sessRuns{runs: map[ids.ID]agents.ActiveRun{}},
		az: &sessAuthorizer{deny: map[authorize.Action]authorize.Decision{}}, audit: &sessRecorder{}, events: &sessEvents{},
		attached: &sessAttached{}, clk: clock.NewFake(start), log: &bytes.Buffer{},
		space: ids.New(), support: ids.New(), main: ids.New(), thread: ids.New(), other: ids.New(), noOp: ids.New(),
		alex: actor.Actor{ID: ids.New(), Kind: actor.KindUser}, sam: actor.Actor{ID: ids.New(), Kind: actor.KindUser},
		lotse: actor.Actor{ID: ids.New(), Kind: actor.KindAgent}, runner: actor.Actor{ID: ids.New(), Kind: actor.KindAgent},
	}
	e.channels[e.support] = e.lotse.ID
	e.threads[e.main] = chat.ThreadInfo{ID: e.main, ChannelID: e.support, SpaceID: e.space, ChannelKind: "text", Kind: "main", State: "open", CreatedBy: e.alex.ID}
	for _, id := range []ids.ID{e.thread, e.other} {
		e.threads[id] = chat.ThreadInfo{ID: id, ChannelID: e.support, SpaceID: e.space, ChannelKind: "text", Kind: "thread", State: "open",
			OperatorAgentID: e.lotse.ID, CreatedBy: e.alex.ID}
	}
	e.threads[e.noOp] = chat.ThreadInfo{ID: e.noOp, ChannelID: e.support, SpaceID: e.space, ChannelKind: "text", Kind: "thread", State: "open", CreatedBy: e.alex.ID}
	reg := txn.NewRegistry()
	if err := txn.Subscribe(reg, e.attached.OnSessionAttached); err != nil {
		t.Fatal(err)
	}
	reg.Seal()
	e.uow = &sessUoW{reg: reg, store: e.store, events: e.events, audit: e.audit, attached: e.attached}
	svc, err := app.NewSessions(app.SessionConfig{SessionRefMaxBytes: sessRefMax}, app.SessionDeps{
		Store: e.store, Threads: e.threads, Channels: e.channels, Runs: e.runs, UoW: e.uow, Authorizer: e.az, Audit: e.audit,
		Events: e.events, Clock: e.clk, Log: slog.New(slog.NewTextHandler(e.log, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	return e
}

// row stores a session fixture of thread and agent with ref; created orders the history.
func (e *sessEnv) row(thread, agent ids.ID, ref string, current, started bool, created time.Duration) agents.Session {
	e.t.Helper()
	r := agents.Session{ID: ids.New(), ThreadID: thread, AgentID: agent, ExternalSessionRef: ref, IsCurrent: current, Started: started,
		CreatedAt: start.Add(created)}
	if !current {
		r.EndedAt, r.EndReason = start.Add(created+time.Minute), agents.EndReset
	}
	return e.store.put(r)
}

// ended stores an ended fixture with reason.
func (e *sessEnv) ended(thread, agent ids.ID, ref, reason string, created time.Duration) agents.Session {
	e.t.Helper()
	r := e.row(thread, agent, ref, false, true, created)
	r.EndReason = reason
	return e.store.put(r)
}

// run makes run the active run of thread.
func (e *sessEnv) run(thread ids.ID, run agents.ActiveRun) agents.ActiveRun {
	e.runs.mu.Lock()
	defer e.runs.mu.Unlock()
	if run.RunID.IsZero() {
		run.RunID = ids.New()
	}
	e.runs.runs[thread] = run
	return run
}

// deny makes the authorizer answer action with outcome and reason.
func (e *sessEnv) deny(action authorize.Action, outcome authorize.Outcome, reason authorize.Reason) {
	e.az.mu.Lock()
	defer e.az.mu.Unlock()
	e.az.deny[action] = authorize.Decision{Outcome: outcome, Reason: reason}
}

// inUnit runs fn inside a unit of work of the env, as runs calls ForInvocation and AttachOnStart.
func (e *sessEnv) inUnit(fn func(ctx context.Context) error) error {
	return e.uow.Do(context.Background(), fn)
}

// eventsSnapshot returns the appended events.
func (e *sessEnv) eventsSnapshot() []eventlog.Event {
	e.events.mu.Lock()
	defer e.events.mu.Unlock()
	return slices.Clone(e.events.evs)
}

// auditSnapshot returns the audit entries.
func (e *sessEnv) auditSnapshot() []sessAudit {
	e.audit.mu.Lock()
	defer e.audit.mu.Unlock()
	return slices.Clone(e.audit.entries)
}

// attachedSnapshot returns the published SessionAttached events.
func (e *sessEnv) attachedSnapshot() []agents.SessionAttached {
	e.attached.mu.Lock()
	defer e.attached.mu.Unlock()
	return slices.Clone(e.attached.evs)
}

// checksSnapshot returns the authorizer calls.
func (e *sessEnv) checksSnapshot() []sessCheck {
	e.az.mu.Lock()
	defer e.az.mu.Unlock()
	return slices.Clone(e.az.checks)
}
