// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
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
		r.runs[i] = x
		return x, true, nil
	}
	return app.RunRow{}, false, nil
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

// msgs is the fake MessageWriter.
type msgs struct{ j *journal }

func (m msgs) SetTriggerState(_ context.Context, cmd messaging.SetTriggerStateCmd) error {
	m.j.add("trigger", cmd)
	return nil
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
}

func (a *authz) Authorize(ctx context.Context, _ authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
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
}

// newWorld builds the world.
func newWorld(t *testing.T) *world {
	t.Helper()
	w := &world{
		t: t, j: &journal{}, repo: &repo{}, errs: &errCounter{}, metrics: &metricsRecorder{}, log: &logBuffer{},
		space: ids.New(), channel: ids.New(), main: ids.New(), thread: ids.New(), lotse: ids.New(), texter: ids.New(), alex: ids.New(),
		dmChannel: ids.New(), dmThread: ids.New(), clk: clock.NewFake(t0),
	}
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
	svc, err := app.New(app.Config{PageSizeDefault: 3, PageSizeMax: 5, StepDetailMaxBytes: 256}, app.Deps{
		Repo: w.repo, UoW: uow{}, Authorizer: w.authz, Events: events{w.j}, Messages: msgs{w.j}, Threads: w.threads, Channels: w.channels,
		Agents: w.agents, Cursors: cursors{}, Errors: w.errs, Log: slog.New(slog.NewJSONHandler(w.log, nil)), Clock: w.clk, Metrics: w.metrics,
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
