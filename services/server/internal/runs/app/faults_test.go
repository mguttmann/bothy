// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// errInjected is the infrastructure error the fault gate injects.
var errInjected = errors.New("injected infrastructure error")

// faultGate counts the port calls of the use cases once armed and fails exactly the call number
// at with errInjected. Every port the follow-up paths use goes through it.
type faultGate struct {
	mu    sync.Mutex
	armed bool
	n, at int
	fired bool
}

func (g *faultGate) hit() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.armed {
		return nil
	}
	g.n++
	if g.n == g.at {
		g.fired = true
		return errInjected
	}
	return nil
}

// wrap puts the ports of d behind the gate.
func (g *faultGate) wrap(d app.Deps) app.Deps {
	d.Repo = gatedRepo{d.Repo, g}
	d.FollowupRows = gatedFollowups{d.FollowupRows, g}
	d.Messages = gatedMessages{d.Messages, g}
	d.Events = gatedEvents{d.Events, g}
	d.Threads = gatedThreads{d.Threads, g}
	d.Channels = gatedChannels{d.Channels, g}
	d.Agents = gatedAgents{d.Agents, g}
	d.Authorizer = gatedAuthorizer{d.Authorizer, g}
	d.Audit = gatedAudit{d.Audit, g}
	return d
}

type gatedRepo struct {
	app.Repository
	g *faultGate
}

func (r gatedRepo) InsertRun(ctx context.Context, n app.NewRun) (app.RunRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.RunRow{}, false, err
	}
	return r.Repository.InsertRun(ctx, n)
}

func (r gatedRepo) Run(ctx context.Context, id ids.ID) (app.RunRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.RunRow{}, false, err
	}
	return r.Repository.Run(ctx, id)
}

func (r gatedRepo) LockRun(ctx context.Context, id ids.ID) (app.RunRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.RunRow{}, false, err
	}
	return r.Repository.LockRun(ctx, id)
}

func (r gatedRepo) RecordDelivery(ctx context.Context, w app.DeliveryWrite) (app.RunRow, error) {
	if err := r.g.hit(); err != nil {
		return app.RunRow{}, err
	}
	return r.Repository.RecordDelivery(ctx, w)
}

func (r gatedRepo) TransitionRun(ctx context.Context, w app.TransitionWrite) (app.RunRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.RunRow{}, false, err
	}
	return r.Repository.TransitionRun(ctx, w)
}

func (r gatedRepo) LockActiveRunOfThread(ctx context.Context, thread ids.ID) (app.RunRow, bool, error) {
	if err := r.g.hit(); err != nil {
		return app.RunRow{}, false, err
	}
	return r.Repository.LockActiveRunOfThread(ctx, thread)
}

func (r gatedRepo) RaiseRunVersion(ctx context.Context, id ids.ID) (int64, error) {
	if err := r.g.hit(); err != nil {
		return 0, err
	}
	return r.Repository.RaiseRunVersion(ctx, id)
}

type gatedFollowups struct {
	app.FollowupRepository
	g *faultGate
}

func (f gatedFollowups) InsertFollowup(ctx context.Context, run, message ids.ID) error {
	if err := f.g.hit(); err != nil {
		return err
	}
	return f.FollowupRepository.InsertFollowup(ctx, run, message)
}

func (f gatedFollowups) LockOpenFollowups(ctx context.Context, run ids.ID) ([]app.FollowupRow, error) {
	if err := f.g.hit(); err != nil {
		return nil, err
	}
	return f.FollowupRepository.LockOpenFollowups(ctx, run)
}

func (f gatedFollowups) RunFollowups(ctx context.Context, run ids.ID) ([]app.FollowupRow, error) {
	if err := f.g.hit(); err != nil {
		return nil, err
	}
	return f.FollowupRepository.RunFollowups(ctx, run)
}

func (f gatedFollowups) FollowupStatesOf(ctx context.Context, run ids.ID, list []ids.ID) ([]app.FollowupRow, error) {
	if err := f.g.hit(); err != nil {
		return nil, err
	}
	return f.FollowupRepository.FollowupStatesOf(ctx, run, list)
}

func (f gatedFollowups) LockFollowup(ctx context.Context, run, message ids.ID) (app.FollowupRow, bool, error) {
	if err := f.g.hit(); err != nil {
		return app.FollowupRow{}, false, err
	}
	return f.FollowupRepository.LockFollowup(ctx, run, message)
}

func (f gatedFollowups) SetFollowupStates(ctx context.Context, run ids.ID, list []ids.ID, st domain.FollowupState) error {
	if err := f.g.hit(); err != nil {
		return err
	}
	return f.FollowupRepository.SetFollowupStates(ctx, run, list, st)
}

func (f gatedFollowups) MoveFollowups(ctx context.Context, from, to ids.ID, list []ids.ID) error {
	if err := f.g.hit(); err != nil {
		return err
	}
	return f.FollowupRepository.MoveFollowups(ctx, from, to, list)
}

func (f gatedFollowups) FollowupOfMessage(ctx context.Context, message ids.ID) (app.FollowupRow, bool, error) {
	if err := f.g.hit(); err != nil {
		return app.FollowupRow{}, false, err
	}
	return f.FollowupRepository.FollowupOfMessage(ctx, message)
}

type gatedMessages struct {
	app.MessageWriter
	g *faultGate
}

func (m gatedMessages) SetTriggerState(ctx context.Context, cmd messaging.SetTriggerStateCmd) error {
	if err := m.g.hit(); err != nil {
		return err
	}
	return m.MessageWriter.SetTriggerState(ctx, cmd)
}

func (m gatedMessages) CreateRunAnchor(ctx context.Context, cmd messaging.RunAnchorCmd) (messaging.AnchorRef, error) {
	if err := m.g.hit(); err != nil {
		return messaging.AnchorRef{}, err
	}
	return m.MessageWriter.CreateRunAnchor(ctx, cmd)
}

func (m gatedMessages) TriggerMessageOf(ctx context.Context, id ids.ID) (messaging.TriggerMessage, error) {
	if err := m.g.hit(); err != nil {
		return messaging.TriggerMessage{}, err
	}
	return m.MessageWriter.TriggerMessageOf(ctx, id)
}

func (m gatedMessages) PrincipalRefs(ctx context.Context, list []ids.ID) (map[ids.ID]messaging.PrincipalRef, error) {
	if err := m.g.hit(); err != nil {
		return nil, err
	}
	return m.MessageWriter.PrincipalRefs(ctx, list)
}

type gatedEvents struct {
	eventlog.Appender
	g *faultGate
}

func (e gatedEvents) Append(ctx context.Context, evs ...eventlog.Event) error {
	if err := e.g.hit(); err != nil {
		return err
	}
	return e.Appender.Append(ctx, evs...)
}

type gatedThreads struct {
	app.ThreadPort
	g *faultGate
}

func (t gatedThreads) Info(ctx context.Context, id ids.ID) (chat.ThreadInfo, error) {
	if err := t.g.hit(); err != nil {
		return chat.ThreadInfo{}, err
	}
	return t.ThreadPort.Info(ctx, id)
}

type gatedChannels struct {
	chat.TriggerFacts
	g *faultGate
}

func (c gatedChannels) TriggerChannel(ctx context.Context, id ids.ID) (chat.TriggerChannel, error) {
	if err := c.g.hit(); err != nil {
		return chat.TriggerChannel{}, err
	}
	return c.TriggerFacts.TriggerChannel(ctx, id)
}

type gatedAgents struct {
	agents.Facts
	g *faultGate
}

func (a gatedAgents) RunFacts(ctx context.Context, list []ids.ID) (map[ids.ID]agents.RunFacts, error) {
	if err := a.g.hit(); err != nil {
		return nil, err
	}
	return a.Facts.RunFacts(ctx, list)
}

type gatedAuthorizer struct {
	authorize.Authorizer
	g *faultGate
}

func (a gatedAuthorizer) Authorize(ctx context.Context, p authorize.Principal, act authorize.Action, r authorize.Resource) (authorize.Decision, error) {
	if err := a.g.hit(); err != nil {
		return authorize.Decision{}, err
	}
	return a.Authorizer.Authorize(ctx, p, act, r)
}

type gatedAudit struct {
	app.AuditRecorder
	g *faultGate
}

func (a gatedAudit) Record(ctx context.Context, e auditing.Entry) error {
	if err := a.g.hit(); err != nil {
		return err
	}
	return a.AuditRecorder.Record(ctx, e)
}

// faultCase prepares a world without faults and returns the step that runs under the gate.
type faultCase struct {
	name  string
	setup func(t *testing.T, w *world) func() error
	// tokens binds the invocation tokens again after the rebuild behind the gate (deliveries).
	tokens bool
}

// addFollowup stores an open follow-up of run whose message is by author at seq.
func addFollowup(t *testing.T, w *world, run ids.ID, author ids.ID, seq int64) ids.ID {
	t.Helper()
	m := ids.New()
	w.msgFacts.m[m] = messaging.TriggerMessage{ID: m, ThreadID: w.thread, ChannelID: w.channel, Origin: "human", Kind: "text",
		Author: messaging.PrincipalRef{ID: author, Kind: "user", Handle: "p", DisplayName: "P"}, ThreadSeq: seq, TriggerState: "received"}
	if err := w.fus.InsertFollowup(context.Background(), run, m); err != nil {
		t.Fatal(err)
	}
	return m
}

// steer makes Lotse report followups = steer.
func steer(w *world) {
	f := w.agents[w.lotse]
	f.FollowupsReported = "steer"
	w.agents[w.lotse] = f
}

// TestFollowups_InfrastructureErrorsReachTheCaller: every follow-up path (acceptance, run end
// with a follow-on run, deletion, the adapter's acknowledgement, the run document and the
// delivery with and without a denial) returns an infrastructure error of any port it calls to its
// caller, so the unit of work rolls back (02 section 8.3) and nothing is half written: the gate
// fails the first, second, ... call until a run passes without reaching the gate's number.
func TestFollowups_InfrastructureErrorsReachTheCaller(t *testing.T) {
	t.Parallel()
	agentCtx := func(w *world) context.Context {
		return actor.WithActor(context.Background(), actor.Actor{ID: w.lotse, Kind: actor.KindAgent})
	}
	cases := []faultCase{
		{name: "acceptance with steer", setup: func(t *testing.T, w *world) func() error {
			steer(w)
			openThreadRun(t, w)
			w.repo.runs[0].State = domain.StateRunning
			ev := w.posted(w.thread)
			return func() error { return w.post(ev) }
		}},
		{name: "acceptance while another agent runs", setup: func(t *testing.T, w *world) func() error {
			openThreadRun(t, w)
			ev := w.posted(w.thread, w.texter)
			return func() error { return w.post(ev) }
		}},
		{name: "run end with a follow-on run", setup: func(t *testing.T, w *world) func() error {
			steer(w)
			run := openThreadRun(t, w)
			addFollowup(t, w, run.ID, ids.New(), 4)
			addFollowup(t, w, run.ID, ids.New(), 9)
			gone := addFollowup(t, w, run.ID, ids.New(), 12)
			f := w.msgFacts.m[gone]
			f.Deleted = true
			w.msgFacts.m[gone] = f
			var calls int
			w.authz.decide = func(act authorize.Action, filter bool, _ authorize.Resource) decision {
				if act == authorize.ActionAgentInvoke && filter {
					calls++
					if calls == 1 {
						return decision{reason: "no_grant"}
					}
				}
				return decision{allowed: true}
			}
			w.clk.Advance(queuedMaxAge + 1)
			return func() error { calls = 0; return w.svc.ExpireQueued(context.Background(), run.ID) }
		}},
		{name: "deletion of a follow-up message", setup: func(t *testing.T, w *world) func() error {
			run := openThreadRun(t, w)
			m := addFollowup(t, w, run.ID, w.alex, 4)
			ev := messaging.MessageDeleted{MessageID: m, ThreadID: w.thread, ChannelID: w.channel, AuthorID: w.alex, DeletedBy: w.alex}
			ctx := actor.WithActor(context.Background(), actor.Actor{ID: w.alex, Kind: actor.KindUser})
			return func() error {
				return uow{}.Do(ctx, func(ctx context.Context) error { return w.acc.OnMessageDeleted(ctx, ev) })
			}
		}},
		{name: "acknowledgement and states", setup: func(t *testing.T, w *world) func() error {
			run := openThreadRun(t, w)
			m := addFollowup(t, w, run.ID, w.alex, 4)
			return func() error {
				fu := w.svc.Followups()
				set := func(ctx context.Context) error { return fu.SetState(ctx, run.ID, m, "delivered") }
				if err := (uow{}).Do(agentCtx(w), set); err != nil {
					return err
				}
				_, err := fu.States(context.Background(), run.ID, []ids.ID{m})
				return err
			}
		}},
		{name: "run document", setup: func(t *testing.T, w *world) func() error {
			run := openThreadRun(t, w)
			addFollowup(t, w, run.ID, w.alex, 4)
			return func() error {
				_, err := w.svc.Followups().Document(context.Background(), run.ID)
				return err
			}
		}},
		{name: "delivery of agent.followup", tokens: true, setup: func(t *testing.T, w *world) func() error {
			return followupDelivery(t, w, false)
		}},
		{name: "refused delivery of agent.followup", tokens: true, setup: func(t *testing.T, w *world) func() error {
			return followupDelivery(t, w, true)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			for at := 1; ; at++ {
				if at > 200 {
					t.Fatal("the path did not end within 200 port calls")
				}
				w := newWorld(t)
				step := c.setup(t, w)
				g := &faultGate{at: at}
				w.gate = g
				w.build()
				if c.tokens {
					if err := w.svc.BindTokens(newInvEnv(t).svc); err != nil {
						t.Fatal(err)
					}
				}
				g.mu.Lock()
				g.armed = true
				g.mu.Unlock()
				err := step()
				if !g.fired {
					if err != nil {
						t.Fatalf("without a fault: %v", err)
					}
					if at == 1 {
						t.Fatal("the path called no port")
					}
					return
				}
				if !errors.Is(err, errInjected) {
					t.Fatalf("fault at port call %d: the caller got %v", at, err)
				}
			}
		})
	}
}

// followupDelivery prepares a running run of Lotse with one open follow-up and returns the
// delivery of its agent.followup; refused makes agent.invoke refuse the follow-up's author.
func followupDelivery(t *testing.T, w *world, refused bool) func() error {
	t.Helper()
	if err := w.svc.BindTokens(newInvEnv(t).svc); err != nil {
		t.Fatal(err)
	}
	run := openThreadRun(t, w)
	w.repo.runs[0].State = domain.StateRunning
	m := addFollowup(t, w, run.ID, w.alex, 4)
	p, err := domain.FollowupPayload(run.ID, w.thread, m)
	if err != nil {
		t.Fatal(err)
	}
	if refused {
		w.authz.decide = func(act authorize.Action, _ bool, _ authorize.Resource) decision {
			if act == authorize.ActionAgentInvoke {
				return decision{reason: "level_insufficient"}
			}
			return decision{allowed: true}
		}
	}
	in := runs.DeliveryInput{AgentID: w.lotse, EventID: ids.New(), Seq: 7, Type: domain.EventAgentFollowup, ResourceKind: "run",
		ResourceID: run.ID, Payload: p}
	return func() error {
		_, err := w.svc.Prepare(context.Background(), in)
		return err
	}
}
