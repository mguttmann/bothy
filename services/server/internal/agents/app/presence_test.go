// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/agents/domain"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The short windows of the tracker tests; no test depends on a default.
const (
	tTimeout  = 10 * time.Second
	tPersist  = 3 * time.Second
	tAlert    = 20 * time.Second
	tInterval = 2 * time.Second
)

// pStart is the clock of the tracker tests.
var pStart = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

// fakeTimers runs the functions of AfterFunc when the test advances the fake clock past them.
type fakeTimers struct {
	mu  gosync.Mutex
	clk *clock.Fake
	all []*fakeTimer
}

type fakeTimer struct {
	at      time.Time
	f       func()
	stopped bool
}

func (ft *fakeTimers) AfterFunc(d time.Duration, f func()) func() bool {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	t := &fakeTimer{at: ft.clk.Now().Add(d), f: f}
	ft.all = append(ft.all, t)
	return func() bool {
		ft.mu.Lock()
		defer ft.mu.Unlock()
		was := !t.stopped
		t.stopped = true
		return was
	}
}

// advance moves the clock by d and runs every due timer once, in order of their time.
func (ft *fakeTimers) advance(d time.Duration) {
	ft.clk.Advance(d)
	for {
		ft.mu.Lock()
		var due *fakeTimer
		for _, t := range ft.all {
			if !t.stopped && !t.at.After(ft.clk.Now()) && (due == nil || t.at.Before(due.at)) {
				due = t
			}
		}
		if due != nil {
			due.stopped = true
		}
		ft.mu.Unlock()
		if due == nil {
			return
		}
		due.f()
	}
}

// fakeStore is the presence store over rows in memory with counters of the writes.
type fakeStore struct {
	mu      gosync.Mutex
	rows    map[ids.ID]domain.PresenceRow
	lockRow map[ids.ID]domain.PresenceRow // returned by LockPresenceRow instead of rows when set
	// lockWait, when set, makes LockPresenceRow close lockEntered and wait for lockWait, as a row
	// lock held by another transaction would.
	lockWait    chan struct{}
	lockEntered chan struct{}
	changes     []app.PresenceWrite
	persists    []app.PresenceWrite
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[ids.ID]domain.PresenceRow{}, lockRow: map[ids.ID]domain.PresenceRow{}}
}

func (s *fakeStore) GetPresenceRow(_ context.Context, id ids.ID) (domain.PresenceRow, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[id]
	return r, ok, nil
}

func (s *fakeStore) LockPresenceRow(_ context.Context, id ids.ID) (domain.PresenceRow, bool, error) {
	s.mu.Lock()
	wait, entered := s.lockWait, s.lockEntered
	s.mu.Unlock()
	if wait != nil {
		close(entered)
		<-wait
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.lockRow[id]; ok {
		return r, true, nil
	}
	r, ok := s.rows[id]
	return r, ok, nil
}

func (s *fakeStore) GetPresenceRows(_ context.Context, list []ids.ID) ([]domain.PresenceRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.PresenceRow
	for _, id := range list {
		if r, ok := s.rows[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *fakeStore) ListLivePresence(context.Context) ([]domain.PresenceRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.PresenceRow
	for _, r := range s.rows {
		if r.Stored.State.Present() {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b domain.PresenceRow) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	return out, nil
}

func (s *fakeStore) WriteReportedChange(_ context.Context, w app.PresenceWrite) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.changes = append(s.changes, w)
	r := s.rows[w.ID]
	r.Stored, r.LastHeartbeatAt = w.Report, w.LastHeartbeatAt
	r.Stored.State = w.State
	s.rows[w.ID] = r
	return nil
}

func (s *fakeStore) PersistHeartbeat(_ context.Context, w app.PresenceWrite) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persists = append(s.persists, w)
	r := s.rows[w.ID]
	if !r.LastHeartbeatAt.IsZero() && r.LastHeartbeatAt.After(w.LastHeartbeatAt) {
		return false, nil
	}
	mode, class := r.Stored.BillingMode, r.Stored.ToolClass
	r.Stored, r.LastHeartbeatAt = w.Report, w.LastHeartbeatAt
	r.Stored.State, r.Stored.BillingMode, r.Stored.ToolClass = w.State, mode, class
	s.rows[w.ID] = r
	return true, nil
}

// counts returns the number of change and persist writes.
func (s *fakeStore) counts() (changes, persists int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.changes), len(s.persists)
}

// setTarget changes the configured values of id.
func (s *fakeStore) setTarget(id ids.ID, t domain.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rows[id]
	r.Target = t
	s.rows[id] = r
}

// fakePrincipals answers handle and status.
type fakePrincipals struct {
	mu     gosync.Mutex
	status map[ids.ID]identity.Status
}

func (p *fakePrincipals) Principal(_ context.Context, id ids.ID) (identity.PrincipalView, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return identity.PrincipalView{ID: id, Handle: "support-agent", Status: p.status[id]}, nil
}

func (p *fakePrincipals) Principals(ctx context.Context, list []ids.ID) (map[ids.ID]identity.PrincipalView, error) {
	out := map[ids.ID]identity.PrincipalView{}
	for _, id := range list {
		v, _ := p.Principal(ctx, id)
		out[id] = v
	}
	return out, nil
}

// allowAgents allows agent.heartbeat exactly for agents on themselves.
type allowAgents struct{}

func (allowAgents) Authorize(_ context.Context, p authorize.Principal, a authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if pa, ok := p.(actor.Actor); ok && pa.Kind == actor.KindAgent && r.ID == pa.ID {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: "subject_kind_not_allowed"}, nil
}
func (allowAgents) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}
func (allowAgents) Observe(context.Context, authorize.LogEvent) {}

// runUoW runs the function directly.
type runUoW struct{}

func (runUoW) Do(ctx context.Context, fn func(ctx context.Context) error, _ ...txn.Option) error {
	return fn(ctx)
}

// fakeRecorder records the entries.
type fakeRecorder struct {
	mu      gosync.Mutex
	entries []auditing.Entry
}

func (r *fakeRecorder) Record(_ context.Context, e auditing.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	return nil
}
func (r *fakeRecorder) RecordDenial(context.Context, auditing.Entry) error   { return nil }
func (r *fakeRecorder) RecordDetached(context.Context, auditing.Entry) error { return nil }
func (r *fakeRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

// fakePublisher records the ephemeral events.
type fakePublisher struct {
	mu  gosync.Mutex
	evs []eventlog.EphemeralEvent
}

func (p *fakePublisher) PublishEphemeral(_ context.Context, ev eventlog.EphemeralEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evs = append(p.evs, ev)
	return nil
}
func (p *fakePublisher) events() []eventlog.EphemeralEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.evs)
}

// fakeReadiness counts the calls of the readiness port.
type fakeReadiness struct {
	mu    gosync.Mutex
	calls [][]chat.ProjectReadiness
}

func (r *fakeReadiness) ApplyProjectReadiness(_ context.Context, _ ids.ID, reports []chat.ProjectReadiness) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, reports)
	return nil
}
func (r *fakeReadiness) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// tracker is the tracker over the fakes with one agent of the configured values organization and
// chat.
type tracker struct {
	t     *testing.T
	tr    *app.Tracker
	clk   *clock.Fake
	tm    *fakeTimers
	store *fakeStore
	pr    *fakePrincipals
	rec   *fakeRecorder
	pub   *fakePublisher
	rd    *fakeReadiness
	agent actor.Actor
}

func newTracker(t *testing.T) *tracker {
	t.Helper()
	clk := clock.NewFake(pStart)
	w := &tracker{t: t, clk: clk, tm: &fakeTimers{clk: clk}, store: newFakeStore(), pr: &fakePrincipals{status: map[ids.ID]identity.Status{}},
		rec: &fakeRecorder{}, pub: &fakePublisher{}, rd: &fakeReadiness{}}
	w.agent = actor.Actor{ID: ids.New(), Kind: actor.KindAgent, TokenID: ids.New()}
	w.addAgent(w.agent.ID)
	tr, err := app.NewTracker(app.PresenceConfig{PresenceTimeout: tTimeout, PersistInterval: tPersist, MismatchAlertAfter: tAlert,
		HeartbeatInterval: tInterval}, app.PresenceDeps{Store: w.store, Principals: w.pr, UoW: runUoW{}, Authorizer: allowAgents{},
		Audit: w.rec, Publisher: w.pub, Readiness: w.rd, Clock: clk, Timers: w.tm})
	if err != nil {
		t.Fatal(err)
	}
	w.tr = tr
	return w
}

// addAgent stores an active agent with the configured values organization and chat.
func (w *tracker) addAgent(id ids.ID) {
	w.store.mu.Lock()
	w.store.rows[id] = domain.PresenceRow{ID: id, Target: domain.Target{BillingMode: domain.Organization, ToolClass: domain.ToolChat},
		Stored: domain.Report{State: domain.RuntimeUnknown}}
	w.store.mu.Unlock()
	w.pr.mu.Lock()
	w.pr.status[id] = identity.StatusActive
	w.pr.mu.Unlock()
}

// beat sends body as the agent and requires success.
func (w *tracker) beat(body string) agents.PresenceStatus {
	w.t.Helper()
	st, err := w.tr.Heartbeat(actor.WithActor(context.Background(), w.agent), json.RawMessage(body))
	if err != nil {
		w.t.Fatalf("heartbeat %s: %v", body, err)
	}
	return st
}

// status returns the presence of the agent.
func (w *tracker) status() agents.PresenceStatus {
	w.t.Helper()
	st, err := w.tr.Statuses(context.Background(), []ids.ID{w.agent.ID})
	if err != nil || len(st) != 1 {
		w.t.Fatalf("statuses %v %v", st, err)
	}
	return st[0]
}

// TestHeartbeat_PublishesOnlyOnStateOrModeChange: the pair (state, reported mode) triggers
// agent.presence_changed, nothing else does (D-M2-20-4).
func TestHeartbeat_PublishesOnlyOnStateOrModeChange(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	w.beat(`{"state":"online","reported_billing_mode":"organization"}`)
	w.beat(`{"state":"online","reported_billing_mode":"organization"}`)
	if n := len(w.pub.events()); n != 1 {
		t.Fatalf("%d events after the same heartbeat twice", n)
	}
	w.beat(`{"state":"degraded","reported_billing_mode":"organization","details":{"degraded_reasons":["slow"]}}`)
	w.beat(`{"state":"degraded","reported_billing_mode":"organization","details":{"degraded_reasons":["slower"]}}`)
	w.beat(`{"state":"degraded","reported_billing_mode":"personal","details":{"degraded_reasons":["slower"]}}`)
	evs := w.pub.events()
	if len(evs) != 3 {
		t.Fatalf("%d events, want 3", len(evs))
	}
	last := evs[2]
	payload, _ := json.Marshal(last.Payload)
	if last.Type != "agent.presence_changed" || last.Actor == nil || last.Actor.PrincipalID != w.agent.ID || last.Resource.Kind != "agent" ||
		last.Resource.ID != w.agent.ID || last.Resource.Version != 0 || !last.Audience.IsZero() || last.Scope != (eventlog.Scope{}) ||
		string(payload) != `{"agent_id":"`+w.agent.ID.String()+`","state":"degraded","reported_billing_mode":"personal"}` {
		t.Fatalf("event %+v payload %s", last, payload)
	}
}

// TestHeartbeat_OfflineTimer: without a heartbeat the deadline sets offline and publishes without
// actor, keeping the last reported mode; a heartbeat just before it re-arms it.
func TestHeartbeat_OfflineTimer(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	w.beat(`{"state":"online","reported_billing_mode":"personal"}`)
	w.tm.advance(tTimeout - time.Millisecond)
	w.beat(`{"state":"online","reported_billing_mode":"personal"}`)
	w.tm.advance(tTimeout - time.Millisecond)
	if st := w.status(); st.State != agents.StateOnline {
		t.Fatalf("state %s before the re-armed deadline", st.State)
	}
	w.tm.advance(time.Millisecond)
	st := w.status()
	evs := w.pub.events()
	if st.State != agents.StateOffline || st.ReportedBillingMode != "personal" || len(evs) != 2 || evs[1].Actor != nil {
		t.Fatalf("status %+v events %+v", st, evs)
	}
	payload, _ := json.Marshal(evs[1].Payload)
	if !bytes.Contains(payload, []byte(`"state":"offline","reported_billing_mode":"personal"`)) {
		t.Fatalf("payload %s", payload)
	}
}

// TestHeartbeat_PersistsAtMostOncePerInterval: many heartbeats in one interval are one
// PersistHeartbeat; a mode change is one WriteReportedChange at once and one audit entry.
func TestHeartbeat_PersistsAtMostOncePerInterval(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	if err := w.tr.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		w.beat(`{"state":"online"}`)
		w.tm.advance(100 * time.Millisecond)
	}
	if c, p := w.store.counts(); c != 0 || p != 0 {
		t.Fatalf("writes before the interval: %d changes, %d persists", c, p)
	}
	w.tm.advance(tPersist)
	if c, p := w.store.counts(); c != 0 || p != 1 {
		t.Fatalf("after one interval: %d changes, %d persists", c, p)
	}
	w.beat(`{"state":"online","reported_billing_mode":"personal","billing_evidence":{"key_type":"oauth","method":"resolved_credentials"}}`)
	if c, _ := w.store.counts(); c != 1 || w.rec.count() != 1 {
		t.Fatalf("mode change: %d changes, %d audit entries", c, w.rec.count())
	}
	e := w.rec.entries[0]
	if e.Action != "agent.heartbeat" || e.Decision != auditing.DecisionAllow || e.ResourceKind != "agent" || e.ResourceID != w.agent.ID ||
		len(e.Attrs) != 3 || e.Attrs["billing_mode_new"].Data() != "personal" || e.Attrs["key_type"].Data() != "oauth" ||
		e.Attrs["method"].Data() != "resolved_credentials" {
		t.Fatalf("audit entry %+v", e)
	}
	w.beat(`{"state":"online","reported_billing_mode":"personal"}`)
	if c, _ := w.store.counts(); c != 1 || w.rec.count() != 1 {
		t.Fatal("a repeated mode wrote again")
	}
	w.tm.advance(tPersist)
	if _, p := w.store.counts(); p != 2 {
		t.Fatalf("%d persists after the second interval, want 2 (the change wrote the state itself, the repeat is dirty)", p)
	}
}

// TestHeartbeat_ChangeEqualAfterLockWritesNothing: another process stored the new values first.
func TestHeartbeat_ChangeEqualAfterLockWritesNothing(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	w.store.mu.Lock()
	w.store.lockRow[w.agent.ID] = domain.PresenceRow{ID: w.agent.ID, Target: domain.Target{BillingMode: domain.Organization, ToolClass: domain.ToolChat},
		Stored: domain.Report{State: domain.RuntimeOnline, BillingMode: domain.Personal}}
	w.store.mu.Unlock()
	w.beat(`{"state":"online","reported_billing_mode":"personal"}`)
	if c, _ := w.store.counts(); c != 0 || w.rec.count() != 0 {
		t.Fatalf("%d changes, %d audit entries", c, w.rec.count())
	}
	if err := w.tr.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, p := w.store.counts(); p != 1 {
		t.Fatal("the heartbeat was not flushed")
	}
}

// TestHeartbeat_ReadinessOnlyOnChange: the same list twice calls the port once, a changed list
// again, an absent member never (D-M2-20-10).
func TestHeartbeat_ReadinessOnlyOnChange(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	ch := ids.New().String()
	w.beat(`{"state":"online"}`)
	if w.rd.count() != 0 {
		t.Fatal("absent member called the port")
	}
	w.beat(`{"state":"online","project_readiness":[{"channel_id":"` + ch + `","ready":true}]}`)
	w.beat(`{"state":"online","project_readiness":[{"channel_id":"` + ch + `","ready":true}]}`)
	w.beat(`{"state":"online"}`)
	if w.rd.count() != 1 {
		t.Fatalf("%d calls, want 1", w.rd.count())
	}
	w.beat(`{"state":"online","project_readiness":[{"channel_id":"` + ch + `","ready":false}]}`)
	if w.rd.count() != 2 {
		t.Fatalf("%d calls after a change, want 2", w.rd.count())
	}
}

// TestHeartbeat_ShutdownFlushPublishesNothing: the stop step writes every dirty entry and
// publishes no event, also when a deadline passes afterwards.
func TestHeartbeat_ShutdownFlushPublishesNothing(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	if err := w.tr.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.beat(`{"state":"online"}`)
	before := len(w.pub.events())
	if err := w.tr.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, p := w.store.counts(); p != 1 {
		t.Fatalf("%d persists at shutdown, want 1", p)
	}
	w.tm.advance(2 * tTimeout)
	if len(w.pub.events()) != before {
		t.Fatal("published during shutdown")
	}
}

// TestTargetsChanged_Reassesses: after SetBillingMode to personal the tracked entry reports
// difference (a) from that time on (note 13).
func TestTargetsChanged_Reassesses(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	w.beat(`{"state":"online","reported_billing_mode":"organization"}`)
	w.store.setTarget(w.agent.ID, domain.Target{BillingMode: domain.Personal, ToolClass: domain.ToolChat})
	if err := w.tr.Changed(context.Background(), []ids.ID{w.agent.ID}); err != nil {
		t.Fatal(err)
	}
	if st := w.status(); st.BillingModeMatches || len(st.Warnings) != 0 {
		t.Fatalf("status at once %+v", st)
	}
	// Heartbeats within agent.presence_timeout keep the agent online past billing.mismatch_alert_after.
	for range 5 {
		w.tm.advance(tAlert / 4)
		w.beat(`{"state":"online","reported_billing_mode":"organization"}`)
	}
	if st := w.status(); !slices.Equal(st.Warnings, []string{agents.WarningBillingModeMismatch}) {
		t.Fatalf("warnings %v after billing.mismatch_alert_after", st.Warnings)
	}
	if s := w.tr.Samples(); len(s) != 1 || s[0].MismatchAge != 5*tAlert/4 {
		t.Fatalf("samples %+v", s)
	}
}

// TestPresenceGauges: heartbeat age and mismatch age per active agent; a disabled agent has no
// series.
func TestPresenceGauges(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	w.beat(`{"state":"online","billing_evidence":{"key_type":"unknown"}}`)
	w.tm.advance(4 * time.Second)
	s := w.tr.Samples()
	if len(s) != 1 || s[0].Handle != "support-agent" || !s[0].HasHeartbeat || s[0].HeartbeatAge != 4*time.Second || s[0].MismatchAge != 4*time.Second {
		t.Fatalf("samples %+v", s)
	}
	w.pr.mu.Lock()
	w.pr.status[w.agent.ID] = identity.StatusDisabled
	w.pr.mu.Unlock()
	if err := w.tr.Changed(context.Background(), []ids.ID{w.agent.ID}); err != nil {
		t.Fatal(err)
	}
	if s := w.tr.Samples(); len(s) != 0 {
		t.Fatalf("disabled agent has samples %+v", s)
	}
}

// TestHeartbeat_RefusesPeopleAndBadBodies: a person gets forbidden, a body outside the schema
// one validation_failed with every field error, and neither changes the presence.
func TestHeartbeat_RefusesPeopleAndBadBodies(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	person := actor.Actor{ID: ids.New(), Kind: actor.KindUser}
	_, err := w.tr.Heartbeat(actor.WithActor(context.Background(), person), json.RawMessage(`{"state":"online"}`))
	if e, ok := errs.As(err); !ok || e.PublicCode() != errs.CodeForbidden {
		t.Fatalf("person: %v", err)
	}
	_, err = w.tr.Heartbeat(actor.WithActor(context.Background(), w.agent), json.RawMessage(`{"state":"online","rate_limit":{},"x":1}`))
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != errs.CodeValidationFailed || len(e.Fields) != 2 {
		t.Fatalf("bad body: %v", err)
	}
	if st := w.status(); st.State != agents.StateUnknown || len(w.pub.events()) != 0 {
		t.Fatalf("presence changed: %+v", st)
	}
}

// TestPresenceStartRule: a stored online agent with a fresh heartbeat stays online until start
// plus agent.presence_timeout; an old one is offline at once and flushed (D-M2-20-6).
func TestPresenceStartRule(t *testing.T) {
	t.Parallel()
	w := newTracker(t)
	old := ids.New()
	w.addAgent(old)
	w.store.mu.Lock()
	fresh := w.store.rows[w.agent.ID]
	fresh.Stored.State, fresh.LastHeartbeatAt = domain.RuntimeOnline, pStart.Add(-time.Second)
	w.store.rows[w.agent.ID] = fresh
	stale := w.store.rows[old]
	stale.Stored.State, stale.LastHeartbeatAt = domain.RuntimeOnline, pStart.Add(-tTimeout-tPersist-time.Second)
	w.store.rows[old] = stale
	w.store.mu.Unlock()
	if err := w.tr.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, _ := w.tr.Statuses(context.Background(), []ids.ID{w.agent.ID, old})
	if len(st) != 2 || st[0].State != agents.StateOnline || st[1].State != agents.StateOffline {
		t.Fatalf("after start %+v", st)
	}
	w.tm.advance(tTimeout - time.Millisecond)
	if w.status().State != agents.StateOnline {
		t.Fatal("offline before start plus agent.presence_timeout")
	}
	w.tm.advance(time.Millisecond)
	if w.status().State != agents.StateOffline {
		t.Fatal("still online after start plus agent.presence_timeout")
	}
	w.store.mu.Lock()
	stored := w.store.rows[old].Stored.State
	w.store.mu.Unlock()
	if stored != domain.RuntimeOffline {
		t.Fatalf("the old agent is stored %s after the first flush", stored)
	}
}

// TestPresenceReadInUnitDoesNotWaitForHeartbeat: a heartbeat that stores a changed reported mode
// holds the entry lock while it waits for the agents row lock; a unit of work that holds that row
// (a switch of M1-51 or the guided flow of M2-21) and reads the presence for agent.updated or the
// I1 check must not wait for the entry lock, or the two wait on each other until a lock timeout
// (red team of M2-21). Inside a unit the read falls back to the stored row.
func TestPresenceReadInUnitDoesNotWaitForHeartbeat(t *testing.T) {
	w := newTracker(t)
	w.beat(`{"state":"online","reported_billing_mode":"organization"}`)
	wait, entered := make(chan struct{}), make(chan struct{})
	w.store.mu.Lock()
	w.store.lockWait, w.store.lockEntered = wait, entered
	w.store.mu.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.beat(`{"state":"online","reported_billing_mode":"personal"}`)
	}()
	<-entered
	u, setPhase := txn.NewUnit(txn.NewRegistry(), txn.Options{})
	setPhase(txn.PhaseWork)
	read := make(chan []agents.PresenceStatus, 1)
	go func() {
		st, err := w.tr.Statuses(txn.WithUnit(context.Background(), u), []ids.ID{w.agent.ID})
		if err != nil {
			t.Errorf("statuses: %v", err)
		}
		read <- st
	}()
	select {
	case st := <-read:
		if len(st) != 1 || st[0].AgentID != w.agent.ID {
			t.Errorf("statuses in a unit = %+v", st)
		}
	case <-time.After(5 * time.Second):
		t.Error("a presence read inside a unit of work waits for the entry lock of a running heartbeat")
	}
	close(wait)
	<-done
	select {
	case <-read:
	case <-time.After(5 * time.Second):
	}
}
