// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// t0 is the clock of the unit tests.
var t0 = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

// repo is an in-memory app.Repository with the partial unique index of the active run of a thread.
type repo struct {
	mu    sync.Mutex
	runs  []app.RunRow
	steps []app.StepRow
	// answerLocks counts LockRunForAnswer (M2-19).
	answerLocks int
	// patches counts the stored patches (WritePatch).
	patches int
	// locks counts the row locks (LockRun).
	locks int
}

// LockRunForAnswer implements app.Repository: the four columns of the answer check (M2-19).
func (r *repo) LockRunForAnswer(ctx context.Context, id ids.ID) (domain.AnswerRun, bool, error) {
	r.mu.Lock()
	r.answerLocks++
	r.mu.Unlock()
	row, found, err := r.Run(ctx, id)
	if err != nil || !found {
		return domain.AnswerRun{}, found, err
	}
	return domain.AnswerRun{ID: row.ID, ThreadID: row.ThreadID, AgentID: row.AgentID, State: row.State}, true, nil
}

func (r *repo) InsertRun(_ context.Context, n app.NewRun) (app.RunRow, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.runs {
		if x.ThreadID == n.ThreadID && x.State.Active() {
			return app.RunRow{}, false, nil
		}
	}
	row := app.RunRow{
		ID: n.ID, ThreadID: n.ThreadID, AgentID: n.AgentID, TriggerKind: n.TriggerKind, TriggeringMessageID: n.TriggeringMessageID,
		TriggeredBy: n.TriggeredBy, State: domain.StateQueued, Version: 1, BillingModeExpected: n.BillingModeExpected,
		InvocationJTI: n.InvocationJTI, Plan: []byte("{}"), Metrics: []byte("{}"), Currency: "USD", CostIsEstimate: true, QueuedAt: n.QueuedAt,
	}
	r.runs = append(r.runs, row)
	return row, true, nil
}

func (r *repo) Run(_ context.Context, id ids.ID) (app.RunRow, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.runs {
		if x.ID == id {
			return x, true, nil
		}
	}
	return app.RunRow{}, false, nil
}

func (r *repo) LockRun(ctx context.Context, id ids.ID) (app.RunRow, bool, error) {
	r.mu.Lock()
	r.locks++
	r.mu.Unlock()
	return r.Run(ctx, id)
}

func (r *repo) RecordDelivery(_ context.Context, w app.DeliveryWrite) (app.RunRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, x := range r.runs {
		if x.ID != w.ID {
			continue
		}
		if !w.JTI.IsZero() {
			x.InvocationJTI, x.RenewedFromJTI = w.JTI, w.RenewedFromJTI
		}
		x.BillingModeExpected, x.Version = w.BillingModeExpected, x.Version+1
		r.runs[i] = x
		return x, nil
	}
	return app.RunRow{}, errs.New(errs.CodeNotFound, "")
}

func (r *repo) ActiveRunOfThread(_ context.Context, thread ids.ID) (app.RunRow, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.runs {
		if x.ThreadID == thread && x.State.Active() {
			return x, true, nil
		}
	}
	return app.RunRow{}, false, nil
}

func (r *repo) TransitionRun(_ context.Context, w app.TransitionWrite) (app.RunRow, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, x := range r.runs {
		if x.ID != w.ID || x.State != w.From || x.Version != w.Version {
			continue
		}
		x.State, x.StateReason, x.Version = w.To, w.Reason, x.Version+1
		if w.To == domain.StateRunning && x.StartedAt.IsZero() {
			x.StartedAt = w.At
		}
		if w.To.Ended() {
			x.FinishedAt = w.At
			if !x.StartedAt.IsZero() {
				x.DurationMS, x.HasDuration = w.At.Sub(x.StartedAt).Milliseconds(), true
			}
		}
		if !w.StartedJTI.IsZero() {
			x.StartedJTI, x.RenewedFromJTI = w.StartedJTI, ids.ID{}
		}
		x.BillingModeUsed = cmp.Or(w.BillingModeUsed, x.BillingModeUsed)
		x.Model, x.StatusLine, x.Summary = cmp.Or(w.Model, x.Model), cmp.Or(w.StatusLine, x.StatusLine), cmp.Or(w.Summary, x.Summary)
		if w.BillingEvidence != nil {
			x.BillingEvidence = w.BillingEvidence
		}
		if w.Metrics != nil {
			x.Metrics = w.Metrics
		}
		r.runs[i] = x
		return x, true, nil
	}
	return app.RunRow{}, false, nil
}

// lockActive returns the active runs that keep holds, ascending by id.
func (r *repo) lockActive(keep func(app.RunRow) bool) []app.RunRow {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.RunRow
	for _, x := range r.runs {
		if x.State.Active() && keep(x) {
			out = append(out, x)
		}
	}
	slices.SortFunc(out, func(a, b app.RunRow) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	return out
}

func (r *repo) LockActiveRunsByStarter(_ context.Context, principals []ids.ID) ([]app.RunRow, error) {
	return r.lockActive(func(x app.RunRow) bool { return slices.Contains(principals, x.TriggeredBy) }), nil
}

func (r *repo) LockActiveRunsByAgent(_ context.Context, agent ids.ID) ([]app.RunRow, error) {
	return r.lockActive(func(x app.RunRow) bool { return x.AgentID == agent }), nil
}

func (r *repo) OrphanCandidates(context.Context) ([]app.RunRow, error) {
	return r.lockActive(func(x app.RunRow) bool { return x.State != domain.StateQueued }), nil
}

func (r *repo) AppendStep(_ context.Context, s app.StepRow) (app.StepRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, x := range r.steps {
		if x.RunID == s.RunID {
			n = max(n, x.Ordinal)
		}
	}
	s.Ordinal = n + 1
	r.steps = append(r.steps, s)
	return s, nil
}

func (r *repo) StepCounts(_ context.Context, runIDs []ids.ID) (map[ids.ID]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[ids.ID]int{}
	for _, s := range r.steps {
		if slices.Contains(runIDs, s.RunID) {
			out[s.RunID]++
		}
	}
	return out, nil
}

func (r *repo) Steps(_ context.Context, q app.StepsRead) ([]app.StepRow, error) {
	all, _ := r.AllSteps(context.Background(), q.RunID)
	var out []app.StepRow
	for _, s := range all {
		if s.Ordinal > q.AfterOrdinal && len(out) < q.Limit {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *repo) AllSteps(_ context.Context, run ids.ID) ([]app.StepRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.StepRow
	for _, s := range r.steps {
		if s.RunID == run {
			out = append(out, s)
		}
	}
	slices.SortFunc(out, func(a, b app.StepRow) int { return a.Ordinal - b.Ordinal })
	return out, nil
}

// keyset returns the rows of keep in (queued_at DESC, id DESC) order after after, at most limit.
func (r *repo) keyset(keep func(app.RunRow) bool, after *app.RunKey, limit int) []app.RunRow {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []app.RunRow
	for _, x := range r.runs {
		if keep(x) {
			all = append(all, x)
		}
	}
	slices.SortFunc(all, func(a, b app.RunRow) int {
		if c := b.QueuedAt.Compare(a.QueuedAt); c != 0 {
			return c
		}
		return bytes.Compare(b.ID[:], a.ID[:])
	})
	var out []app.RunRow
	for _, x := range all {
		if after != nil {
			if x.QueuedAt.After(after.QueuedAt) || (x.QueuedAt.Equal(after.QueuedAt) && bytes.Compare(x.ID[:], after.ID[:]) >= 0) {
				continue
			}
		}
		if len(out) == limit {
			break
		}
		out = append(out, x)
	}
	return out
}

func (r *repo) ThreadRuns(_ context.Context, q app.ThreadRunsRead) ([]app.RunRow, error) {
	return r.keyset(func(x app.RunRow) bool { return x.ThreadID == q.ThreadID }, q.After, q.Limit), nil
}

func (r *repo) ActiveRuns(_ context.Context, q app.ActiveRunsRead) ([]app.RunRow, error) {
	return r.keyset(func(x app.RunRow) bool { return x.State.Active() && (q.AgentID.IsZero() || x.AgentID == q.AgentID) }, q.After, q.Limit), nil
}

func (r *repo) PatchSteps(ctx context.Context, run ids.ID) ([]app.StepRow, error) {
	all, err := r.AllSteps(ctx, run)
	for i := range all {
		if all[i].Name != domain.SummaryStepName {
			all[i].Detail = nil
		}
	}
	return all, err
}

// WritePatch applies the change like adapters/postgres: one version step, stall_warned_at
// cleared (the fake has no column for it), then deletes, updates and inserts of steps.
func (r *repo) WritePatch(_ context.Context, w app.PatchWrite) (app.RunRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.patches++
	ch := w.Change
	for i, x := range r.runs {
		if x.ID != w.ID {
			continue
		}
		if ch.StatusLine.Set {
			x.StatusLine = ch.StatusLine.Value
		}
		if ch.LastTextExcerpt.Set {
			x.LastTextExcerpt = ch.LastTextExcerpt.Value
		}
		if ch.Model.Set {
			x.Model = ch.Model.Value
		}
		if ch.Metrics != nil {
			x.Metrics = ch.Metrics
		}
		if ch.Plan != nil {
			x.Plan = ch.Plan
		}
		if ch.Cost != nil {
			x.CostMicros, x.Currency, x.CostIsEstimate = ch.Cost.AmountMicros, ch.Cost.Currency, ch.Cost.IsEstimate
		}
		x.Version++
		r.runs[i] = x
		kept := r.steps[:0]
		for _, s := range r.steps {
			if s.RunID != w.ID || !slices.Contains(ch.DeleteSteps, s.ID) {
				kept = append(kept, s)
			}
		}
		r.steps = kept
		for _, u := range ch.UpdateSteps {
			for j, s := range r.steps {
				if s.ID != u.ID {
					continue
				}
				if u.Status != nil {
					s.Status = *u.Status
				}
				if u.Title != nil {
					s.Title = *u.Title
				}
				if u.Detail != nil {
					s.Detail = u.Detail
				}
				if u.FinishedAt != nil {
					s.FinishedAt = *u.FinishedAt
				}
				if u.Ordinal != nil {
					s.Ordinal = *u.Ordinal
				}
				r.steps[j] = s
			}
		}
		for _, s := range ch.InsertSteps {
			r.steps = append(r.steps, app.StepRow{ID: s.ID, RunID: w.ID, Ordinal: s.Ordinal, ParentStepID: s.ParentStepID, Kind: s.Kind,
				Name: s.Name, Title: s.Title, Status: s.Status, Detail: s.Detail, StartedAt: s.StartedAt, FinishedAt: s.FinishedAt})
		}
		return x, nil
	}
	return app.RunRow{}, errs.New(errs.CodeNotFound, "")
}

func (r *repo) ActiveRunsOfThreads(_ context.Context, list []ids.ID) ([]app.RunRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.RunRow
	for _, x := range r.runs {
		if x.State.Active() && slices.Contains(list, x.ThreadID) {
			out = append(out, x)
		}
	}
	return out, nil
}

func (r *repo) ActiveCounts(context.Context) ([]app.ActiveCount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.ActiveCount
	for _, x := range r.runs {
		if x.State.Active() {
			out = append(out, app.ActiveCount{AgentID: x.AgentID, State: x.State, Count: 1})
		}
	}
	return out, nil
}

// call is one recorded call of a fake collaborator, in order.
type call struct {
	what string
	arg  any
}

// journal records the calls of the fakes of one test in order.
type journal struct {
	mu    sync.Mutex
	calls []call
}

func (j *journal) add(what string, arg any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.calls = append(j.calls, call{what, arg})
}

func (j *journal) list() []call {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.calls)
}

// whats returns the names of the recorded calls.
func (j *journal) whats() []string {
	var out []string
	for _, c := range j.list() {
		out = append(out, c.what)
	}
	return out
}

// count returns how often what was called.
func (j *journal) count(what string) int {
	n := 0
	for _, c := range j.list() {
		if c.what == what {
			n++
		}
	}
	return n
}

// msgs is the fake MessageWriter; facts holds the trigger facts of TriggerMessageOf (a missing
// message reads as a human text message of Alex without mentions in the main thread).
type msgs struct {
	j     *journal
	facts *msgFacts
}

// msgFacts are the stored trigger facts of the fake messages.
type msgFacts struct {
	mu   sync.Mutex
	m    map[ids.ID]messaging.TriggerMessage
	dflt messaging.TriggerMessage
}

func (m msgs) SetTriggerState(_ context.Context, cmd messaging.SetTriggerStateCmd) error {
	m.j.add("trigger", cmd)
	if m.facts != nil {
		m.facts.mu.Lock()
		f, ok := m.facts.m[cmd.MessageID]
		if !ok {
			f = m.facts.dflt
			f.ID = cmd.MessageID
		}
		f.TriggerState, f.TriggerStateReason = cmd.State, cmd.Reason
		m.facts.m[cmd.MessageID] = f
		m.facts.mu.Unlock()
	}
	return nil
}

func (m msgs) TriggerMessageOf(_ context.Context, id ids.ID) (messaging.TriggerMessage, error) {
	m.j.add("read-message", id)
	if m.facts == nil {
		return messaging.TriggerMessage{ID: id, Origin: "human", Kind: "text", Author: messaging.PrincipalRef{ID: ids.New(), Kind: "user", Handle: "alex", DisplayName: "Alex"}}, nil
	}
	m.facts.mu.Lock()
	defer m.facts.mu.Unlock()
	f, ok := m.facts.m[id]
	if !ok {
		f = m.facts.dflt
		f.ID = id
	}
	return f, nil
}

// PrincipalRefs renders every id as a person named Alex (M2-16).
func (m msgs) PrincipalRefs(_ context.Context, list []ids.ID) (map[ids.ID]messaging.PrincipalRef, error) {
	out := map[ids.ID]messaging.PrincipalRef{}
	for _, id := range list {
		out[id] = messaging.PrincipalRef{ID: id, Kind: "user", Handle: "alex", DisplayName: "Alex"}
	}
	return out, nil
}

func (m msgs) CreateRunAnchor(_ context.Context, cmd messaging.RunAnchorCmd) (messaging.AnchorRef, error) {
	m.j.add("anchor", cmd)
	return messaging.AnchorRef{MessageID: ids.New(), ThreadSeq: 1}, nil
}

// events is the fake appender.
type events struct{ j *journal }

func (e events) Append(_ context.Context, evs ...eventlog.Event) error {
	for _, ev := range evs {
		e.j.add("event", ev)
	}
	return nil
}

// threads is the fake ThreadPort over a map of threads.
type threads struct {
	j     *journal
	mu    sync.Mutex
	infos map[ids.ID]chat.ThreadInfo
}

func (t *threads) Info(_ context.Context, id ids.ID) (chat.ThreadInfo, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if in, ok := t.infos[id]; ok {
		return in, nil
	}
	return chat.ThreadInfo{}, errs.New(errs.CodeNotFound, "")
}

// LockForMessageChange records the thread lock of a model command (M2-16).
func (t *threads) LockForMessageChange(_ context.Context, id ids.ID) error {
	t.j.add("lock-thread", id)
	return nil
}

func (t *threads) Locate(_ context.Context, list []ids.ID) ([]chat.ThreadInfo, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []chat.ThreadInfo
	for _, id := range list {
		if in, ok := t.infos[id]; ok {
			out = append(out, in)
		}
	}
	return out, nil
}

func (t *threads) Create(_ context.Context, _ actor.Actor, cmd chat.CreateThreadCmd) (chat.Thread, error) {
	t.j.add("thread", cmd)
	t.mu.Lock()
	defer t.mu.Unlock()
	id := ids.New()
	main := t.infos[cmd.Parent.ThreadID]
	t.infos[id] = chat.ThreadInfo{ID: id, ChannelID: cmd.ChannelID, SpaceID: main.SpaceID, ChannelKind: main.ChannelKind, Kind: "thread", State: "open"}
	return chat.Thread{ID: id, ChannelID: cmd.ChannelID, Kind: "thread"}, nil
}

// channels is the fake chat.TriggerFacts.
type channels map[ids.ID]chat.TriggerChannel

func (c channels) TriggerChannel(_ context.Context, id ids.ID) (chat.TriggerChannel, error) {
	if ch, ok := c[id]; ok {
		return ch, nil
	}
	return chat.TriggerChannel{}, errs.New(errs.CodeNotFound, "")
}

// agentFacts is the fake agents.Facts.
type agentFacts map[ids.ID]agents.RunFacts

func (a agentFacts) RunFacts(_ context.Context, list []ids.ID) (map[ids.ID]agents.RunFacts, error) {
	out := map[ids.ID]agents.RunFacts{}
	for _, id := range list {
		if f, ok := a[id]; ok {
			out[id] = f
		}
	}
	return out, nil
}

// cursors encodes the key as JSON with the list and binding in front.
type cursors struct{}

type cursorBody struct {
	List    string          `json:"l"`
	Binding string          `json:"b"`
	Key     json.RawMessage `json:"k"`
}

func (cursors) Encode(list string, binding []byte, key any) (string, error) {
	k, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(cursorBody{List: list, Binding: string(binding), Key: k})
	return string(b), err
}

func (cursors) Decode(list string, binding []byte, s string, key any) error {
	var c cursorBody
	if err := json.Unmarshal([]byte(s), &c); err != nil || c.List != list || c.Binding != string(binding) {
		return errs.New(errs.CodeCursorInvalid, "")
	}
	return json.Unmarshal(c.Key, key)
}

// decision is a scripted verdict of the fake authorizer.
type decision struct {
	allowed bool
	reason  string
}

// authz is the fake Authorizer: per action and filter mode a function of the resource; the calls
// are journaled as "authz:<action>:<mode>".
type authz struct {
	j      *journal
	decide func(act authorize.Action, filter bool, r authorize.Resource) decision
	scope  []ids.ID
	// principalCheck, when set, refuses run.update with forbidden for a principal it rejects (the
	// AgentSelf rule of M1-28 as the tests script it).
	principalCheck func(p authorize.Principal, r authorize.Resource) bool
	// mu guards principals and pins: the principal and the pinned version of every call.
	mu         sync.Mutex
	principals []authorize.Principal
	pins       []int64
}

func (a *authz) Authorize(ctx context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	a.mu.Lock()
	v, _ := authorize.PinnedPermissionVersion(ctx)
	a.principals, a.pins = append(a.principals, p), append(a.pins, v)
	a.mu.Unlock()
	filter := authorize.IsFilter(ctx)
	mode := "request"
	if filter {
		mode = "filter"
	}
	a.j.add("authz:"+string(act)+":"+mode, r)
	d := decision{allowed: true}
	if a.decide != nil {
		d = a.decide(act, filter, r)
	}
	if a.principalCheck != nil && act == authorize.ActionRunUpdate && r.OwnerID != (ids.ID{}) && !a.principalCheck(p, r) {
		d = decision{reason: "not_self"}
	}
	if d.allowed {
		return authorize.Decision{Outcome: authorize.OutcomeAllow}, nil
	}
	return authorize.Decision{Outcome: authorize.OutcomeForbidden, Reason: authorize.Reason(d.reason)}, nil
}

func (a *authz) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	sc := slices.Clone(a.scope)
	slices.SortFunc(sc, func(x, y ids.ID) int { return bytes.Compare(x[:], y[:]) })
	return authorize.Scope{ChannelIDs: sc}, nil
}

func (a *authz) Observe(context.Context, authorize.LogEvent) {}

// uow is the fake unit of work: it runs fn in a txn unit and calls the AfterCommit functions
// when fn succeeds.
type uow struct{}

func (uow) Do(ctx context.Context, fn func(ctx context.Context) error, _ ...txn.Option) error {
	u, c := txn.NewControlledUnit(txn.NewRegistry(), txn.Options{})
	c.SetPhase(txn.PhaseWork)
	if err := fn(txn.WithUnit(ctx, u)); err != nil {
		return err
	}
	c.Committed()
	return nil
}

// errCounter counts the reasons of bothy.authz.errors.
type errCounter struct {
	mu sync.Mutex
	n  map[string]int
}

func (e *errCounter) CountError(_ context.Context, reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.n == nil {
		e.n = map[string]int{}
	}
	e.n[reason]++
}

// metricsRecorder records the committed transitions.
type metricsRecorder struct {
	mu   sync.Mutex
	list []app.Transitioned
}

func (m *metricsRecorder) RunTransitioned(_ context.Context, t app.Transitioned) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.list = append(m.list, t)
}

// logBuffer is a JSON log for the assertions on the log lines.
type logBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// world is the test world of the app tests: one space with a text channel C (operator Lotse), its
// main thread and a thread T of C; agents Lotse and Texter; the person Alex.
type world struct {
	t                   *testing.T
	j                   *journal
	repo                *repo
	threads             *threads
	channels            channels
	agents              agentFacts
	authz               *authz
	errs                *errCounter
	metrics             *metricsRecorder
	log                 *logBuffer
	svc                 *app.Service
	acc                 *app.Acceptor
	space, channel      ids.ID
	main, thread        ids.ID
	lotse, texter, alex ids.ID
	dmChannel, dmThread ids.ID
	clk                 *clock.Fake
	defaultSwitches     chat.TriggerChannel
	msgFacts            *msgFacts
	audit               *auditJournal
	telemetry           *telemetryJournal
	pub                 *publisher
	steps               *stepCounter
	payloadMax          int
	uow                 txn.UnitOfWork
}

// newWorld builds the world.
func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{
		t: t, j: &journal{}, repo: &repo{}, errs: &errCounter{}, metrics: &metricsRecorder{}, log: &logBuffer{},
		space: ids.New(), channel: ids.New(), main: ids.New(), thread: ids.New(), lotse: ids.New(), texter: ids.New(), alex: ids.New(),
		dmChannel: ids.New(), dmThread: ids.New(), clk: clock.NewFake(t0), audit: &auditJournal{}, telemetry: &telemetryJournal{},
		pub: &publisher{}, steps: &stepCounter{}, payloadMax: 65536,
	}
	w.msgFacts = &msgFacts{m: map[ids.ID]messaging.TriggerMessage{}, dflt: messaging.TriggerMessage{
		Origin: "human", Kind: "text", ThreadID: w.main, ChannelID: w.channel,
		Author: messaging.PrincipalRef{ID: w.alex, Kind: "user", Handle: "alex", DisplayName: "Alex"},
	}}
	w.threads = &threads{j: w.j, infos: map[ids.ID]chat.ThreadInfo{
		w.main:     {ID: w.main, ChannelID: w.channel, SpaceID: w.space, ChannelKind: "text", Kind: "main", State: "open"},
		w.thread:   {ID: w.thread, ChannelID: w.channel, SpaceID: w.space, ChannelKind: "text", Kind: "thread", State: "open", OperatorAgentID: w.lotse},
		w.dmThread: {ID: w.dmThread, ChannelID: w.dmChannel, ChannelKind: "dm", Kind: "main", State: "open"},
	}}
	w.defaultSwitches = chat.TriggerChannel{
		ChannelID: w.channel, SpaceID: w.space, Kind: "text", EffectiveOperatorID: w.lotse, RequireMention: true, AutoThread: true,
	}
	w.channels = channels{w.channel: w.defaultSwitches, w.dmChannel: {ChannelID: w.dmChannel, Kind: "dm"}}
	w.agents = agentFacts{
		w.lotse:  {ID: w.lotse, Handle: "lotse", Status: "active", BillingMode: "organization", ToolClass: "chat"},
		w.texter: {ID: w.texter, Handle: "texter", Status: "active", BillingMode: "organization", ToolClass: "chat"},
	}
	w.authz = &authz{j: w.j, scope: []ids.ID{w.channel}}
	w.build()
	return w
}

// build (re)creates the use cases over the fakes of w.
func (w *world) build() {
	svc, err := app.New(app.Config{PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 256, QueuedMaxAge: queuedMaxAge,
		ClockLeeway: 30 * time.Second, CancelGrace: 30 * time.Second, PresenceTimeout: 90 * time.Second, OrphanTimeout: 15 * time.Minute, ReasonMaxChars: 500,
		ExcerptMaxChars: 20, MaxStepsPersisted: 4, EventPayloadMaxBytes: w.payloadMax}, app.Deps{
		Repo: w.repo, UoW: cmp.Or[txn.UnitOfWork](w.uow, uow{}), Authorizer: w.authz, Events: events{w.j}, Messages: msgs{w.j, w.msgFacts}, Threads: w.threads, Channels: w.channels,
		Agents: w.agents, Cursors: cursors{}, Errors: w.errs, Log: slog.New(slog.NewJSONHandler(w.log, nil)), Clock: w.clk, Metrics: w.metrics,
		Versions: versions{}, Audit: w.audit, Telemetry: w.telemetry, Expiry: expiryJournal{w.j},
		Publisher: w.pub, Steps: w.steps,
	})
	if err != nil {
		w.t.Fatal(err)
	}
	w.svc = svc
	w.acc = app.NewAcceptor()
	if err := w.acc.Bind(svc); err != nil {
		w.t.Fatal(err)
	}
}

// posted returns a MessagePosted of a text message of Alex in thread.
func (w *world) posted(thread ids.ID, mentions ...ids.ID) messaging.MessagePosted {
	in := w.threads.infos[thread]
	return messaging.MessagePosted{
		MessageID: ids.New(), ThreadID: thread, ChannelID: in.ChannelID, SpaceID: in.SpaceID, ThreadKind: in.Kind, ThreadSeq: 1,
		AuthorID: w.alex, AuthorKind: "user", Origin: "human", Kind: "text", ClientMsgID: ids.New(), MentionedIDs: mentions, CreatedAt: t0,
	}
}

// post runs the acceptance of ev in a unit of work as Alex.
func (w *world) post(ev messaging.MessagePosted) error {
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: ev.AuthorID, Kind: actor.Kind(ev.AuthorKind)})
	return uow{}.Do(ctx, func(ctx context.Context) error { return w.acc.OnMessagePosted(ctx, ev) })
}

// mustPost is post that fails the test on an error.
func (w *world) mustPost(ev messaging.MessagePosted) {
	w.t.Helper()
	if err := w.post(ev); err != nil {
		w.t.Fatal(err)
	}
}

// triggers returns the trigger state commands of the journal.
func (w *world) triggers() []messaging.SetTriggerStateCmd {
	var out []messaging.SetTriggerStateCmd
	for _, c := range w.j.list() {
		if c.what == "trigger" {
			out = append(out, c.arg.(messaging.SetTriggerStateCmd))
		}
	}
	return out
}

// errVeto is the error a vetoing step of a test returns.
var errVeto = errors.New("veto")

// queuedMaxAge is run.queued_max_age of the unit tests.
const queuedMaxAge = 48 * time.Hour

// versions is the fake permission version.
type versions struct{}

func (versions) PermissionVersion(context.Context) (int64, error) { return 9, nil }

// auditJournal records the audit entries with the actor of their context.
type auditJournal struct {
	mu      sync.Mutex
	entries []auditing.Entry
	actors  []actor.Actor
}

func (a *auditJournal) Record(ctx context.Context, e auditing.Entry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, _ := actor.FromContext(ctx)
	a.entries, a.actors = append(a.entries, e), append(a.actors, p)
	return nil
}

func (a *auditJournal) list() ([]auditing.Entry, []actor.Actor) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.entries), slices.Clone(a.actors)
}

// telemetryJournal records the denial counter and the decisions of the spans.
type telemetryJournal struct {
	mu        sync.Mutex
	denied    []string
	decisions []string
}

func (t *telemetryJournal) Denied(_ context.Context, handle, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.denied = append(t.denied, handle+":"+reason)
}

func (t *telemetryJournal) StartDispatch(ctx context.Context, _ ids.ID, _ string) (context.Context, func(string)) {
	return ctx, func(d string) {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.decisions = append(t.decisions, d)
	}
}

// expiryJournal records the scheduled jobs.
type expiryJournal struct{ j *journal }

func (e expiryJournal) ScheduleQueuedExpiry(_ context.Context, run ids.ID, at time.Time) error {
	e.j.add("expiry", expiryAt{run, at})
	return nil
}

// expiryAt is one scheduled job.
type expiryAt struct {
	run ids.ID
	at  time.Time
}

// publisher records the published ephemeral events.
type publisher struct {
	mu  sync.Mutex
	evs []eventlog.EphemeralEvent
	err error
}

func (p *publisher) PublishEphemeral(_ context.Context, ev eventlog.EphemeralEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evs = append(p.evs, ev)
	return p.err
}

func (p *publisher) list() []eventlog.EphemeralEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.evs)
}

// stepCounter records bothy.run.steps as handle:kind.
type stepCounter struct {
	mu   sync.Mutex
	list []string
}

func (s *stepCounter) StepAdded(_ context.Context, handle, kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, handle+":"+kind)
}
