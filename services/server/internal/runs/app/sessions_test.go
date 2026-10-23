// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// sessions is the fake agents.Sessions of the runs side: ForInvocation answers row (nil for none)
// and AttachOnStart answers attach or err; both journal their input and whether the run row
// existed when they ran (run row first, then session rows, 03 6.3).
type sessions struct {
	j      *journal
	repo   *repo
	mu     sync.Mutex
	row    *agents.Session
	attach ids.ID
	err    error
}

func (s *sessions) List(context.Context, actor.Actor, ids.ID) ([]agents.Session, error) {
	return nil, nil
}

func (s *sessions) Reset(context.Context, actor.Actor, ids.ID) (agents.Session, error) {
	return agents.Session{}, nil
}

func (s *sessions) Bind(context.Context, actor.Actor, ids.ID, agents.BindCmd) (agents.Session, error) {
	return agents.Session{}, nil
}

func (s *sessions) Report(context.Context, actor.Actor, ids.ID, agents.ReportCmd) (agents.Session, error) {
	return agents.Session{}, nil
}

func (s *sessions) ForInvocation(ctx context.Context, q agents.InvocationSessionQuery) (agents.InvocationSession, error) {
	_, found, _ := s.repo.ActiveRunOfThread(ctx, q.ThreadID)
	s.j.add("for-invocation", sessionCall{q: q, runFirst: found, inUnit: inUnit(ctx)})
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.row == nil {
		return agents.InvocationSession{}, s.err
	}
	r := *s.row
	return agents.InvocationSession{Row: &r}, s.err
}

func (s *sessions) AttachOnStart(ctx context.Context, cmd agents.AttachOnStartCmd) (ids.ID, error) {
	s.j.add("attach-on-start", sessionCall{attach: cmd, inUnit: inUnit(ctx)})
	return s.attach, s.err
}

func (s *sessions) CheckStartModel(string) error { return nil }

func (s *sessions) AddCost(context.Context, agents.AddSessionCost) error { return nil }

// sessionCall is the journaled input of a fake session call.
type sessionCall struct {
	q        agents.InvocationSessionQuery
	attach   agents.AttachOnStartCmd
	runFirst bool
	inUnit   bool
}

// inUnit reports whether ctx carries a unit of work.
func inUnit(ctx context.Context) bool {
	return txn.InUnit(ctx)
}

// withSessions rebuilds w with the fake sessions s.
func (w *world) withSessions(s *sessions) {
	s.j, s.repo = w.j, w.repo
	w.sessions = s
	w.build()
}

// invocationPayload returns the payload members of the last agent.invocation.
func (w *world) invocationPayload() map[string]any {
	w.t.Helper()
	var last *eventlog.Event
	for _, c := range w.j.list() {
		if ev, ok := c.arg.(eventlog.Event); ok && ev.Type == "agent.invocation" {
			last = &ev
		}
	}
	if last == nil {
		w.t.Fatal("no agent.invocation")
	}
	var m map[string]any
	if err := json.Unmarshal(last.Payload, &m); err != nil {
		w.t.Fatal(err)
	}
	return m
}

// sessionRow is a current session row of agent in thread.
func sessionRow(thread, agent ids.ID, started bool) *agents.Session {
	return &agents.Session{
		ID: ids.New(), ThreadID: thread, AgentID: agent, ExternalSessionRef: "sess-1", IsCurrent: true, Started: started,
		Currency: "USD", CreatedAt: t0, Version: 1,
	}
}

// TestInvocationSession (09 9.1, M2-12 design note 9): an operator run asks ForInvocation in the
// unit of the acceptance after Open, with the thread, the agent and the triggering person, and its
// payload carries session_mode and the row; the mode follows the row and whether a run used it. A
// world without Sessions gives new without a row.
func TestInvocationSession(t *testing.T) {
	t.Parallel()
	t.Run("without sessions every operator run gets new", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.mustPost(w.posted(w.thread))
		if p := w.invocationPayload(); p["session_mode"] != "new" || p["agent_session"] != nil {
			t.Fatalf("payload %v", p)
		}
	})
	t.Run("no row gives new", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.withSessions(&sessions{})
		w.mustPost(w.posted(w.thread))
		calls := callsOf(w, "for-invocation")
		if len(calls) != 1 || calls[0].q != (agents.InvocationSessionQuery{ThreadID: w.thread, AgentID: w.lotse, TriggeredBy: w.alex}) ||
			!calls[0].runFirst || !calls[0].inUnit {
			t.Fatalf("ForInvocation %+v", calls)
		}
		if p := w.invocationPayload(); p["session_mode"] != "new" || p["agent_session"] != nil {
			t.Fatalf("payload %v", p)
		}
	})
	for _, c := range []struct {
		name          string
		started, used bool
		mode          string
	}{
		{"unstarted row gives new with the row", false, false, "new"},
		{"started row a run used gives resume", true, true, "resume"},
		{"started row no run used gives bind", true, false, "bind"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			row := sessionRow(w.thread, w.lotse, c.started)
			w.withSessions(&sessions{row: row})
			if c.used {
				w.repo.runs = append(w.repo.runs, app.RunRow{ID: ids.New(), ThreadID: w.thread, AgentID: w.lotse, AgentSessionID: row.ID,
					State: domain.StateCompleted, Version: 3, QueuedAt: t0})
			}
			w.mustPost(w.posted(w.thread))
			p := w.invocationPayload()
			s, _ := p["agent_session"].(map[string]any)
			if p["session_mode"] != c.mode || s["id"] != row.ID.String() || s["external_session_ref"] != "sess-1" || s["version"] != float64(1) {
				t.Fatalf("payload %v", p)
			}
		})
	}
	t.Run("the main thread asks for the channel's effective operator", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.withSessions(&sessions{})
		ch := w.channels[w.channel]
		ch.AutoThread = false
		w.channels[w.channel] = ch
		w.build()
		w.mustPost(w.posted(w.main, w.lotse))
		calls := callsOf(w, "for-invocation")
		if len(calls) != 1 || calls[0].q.ThreadID != w.main || calls[0].q.AgentID != w.lotse {
			t.Fatalf("ForInvocation %+v", calls)
		}
		if p := w.invocationPayload(); p["session_mode"] != "new" {
			t.Fatalf("payload %v", p)
		}
	})
	t.Run("an error of ForInvocation rolls the acceptance back", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		w.withSessions(&sessions{err: errs.New(errs.CodeInternalError, "boom")})
		if err := w.post(w.posted(w.thread)); err == nil {
			t.Fatal("no error")
		}
	})
}

// TestInvocationSession_GuestRun (09 3.6, R-AG-04): a run of an agent that does not operate the
// thread gets no session: no ForInvocation, no session_mode, no agent_session.
func TestInvocationSession_GuestRun(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.withSessions(&sessions{row: sessionRow(w.thread, w.texter, true)})
	ch := w.channels[w.channel]
	ch.AllowMentionedAgents = true
	w.channels[w.channel] = ch
	w.build()
	w.mustPost(w.posted(w.thread, w.texter))
	if len(w.repo.runs) != 1 || w.repo.runs[0].AgentID != w.texter {
		t.Fatalf("runs %+v", w.repo.runs)
	}
	if n := len(callsOf(w, "for-invocation")); n != 0 {
		t.Fatalf("%d ForInvocation calls", n)
	}
	p := w.invocationPayload()
	if _, ok := p["session_mode"]; ok {
		t.Fatalf("payload %v", p)
	}
	if _, ok := p["agent_session"]; ok {
		t.Fatalf("payload %v", p)
	}
}

// callsOf returns the journaled session calls named what.
func callsOf(w *world, what string) []sessionCall {
	var out []sessionCall
	for _, c := range w.j.list() {
		if c.what == what {
			out = append(out, c.arg.(sessionCall))
		}
	}
	return out
}

// withSessions rebuilds the delivery world with the fake sessions s and binds the tokens again.
func (w *deliveryWorld) withSessions(s *sessions) {
	w.world.withSessions(s)
	if err := w.svc.BindTokens(newInvEnv(w.t).svc); err != nil {
		w.t.Fatal(err)
	}
}

// startAs runs Start for the run of w as Lotse with the token of a delivery.
func (w *deliveryWorld) startAs(ref *string, model string) (runs.StartResult, error) {
	w.t.Helper()
	out := w.prepare(w.input(domain.EventAgentInvocation))
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: w.lotse, Kind: actor.KindAgent})
	return w.svc.Start(ctx, runs.StartRun{RunID: w.run.ID, Token: out.InvocationToken, BillingModeUsed: "organization", ToolClassUsed: "chat",
		Model: model, ExternalSessionRef: ref})
}

// TestStart_AttachOnStart (09 9.1, 4.7; M2-12 design note 10): queued -> running asks
// AttachOnStart in the unit of the start with the run, its thread and agent, the reference and the
// model; a returned row is stored in runs.agent_session_id before the document is read. A zero id
// stores nothing; a validation error of AttachOnStart is returned unchanged and nothing commits.
func TestStart_AttachOnStart(t *testing.T) {
	t.Parallel()
	ref := "sess-9"
	t.Run("a returned row is attached", func(t *testing.T) {
		t.Parallel()
		w := newDeliveryWorld(t)
		row := ids.New()
		w.withSessions(&sessions{attach: row})
		res, err := w.startAs(&ref, "model-a")
		if err != nil {
			t.Fatal(err)
		}
		calls := callsOf(w.world, "attach-on-start")
		want := agents.AttachOnStartCmd{ThreadID: w.thread, RunID: w.run.ID, AgentID: w.lotse, ExternalSessionRef: ref, Model: "model-a"}
		if len(calls) != 1 || calls[0].attach != want || !calls[0].inUnit {
			t.Fatalf("AttachOnStart %+v", calls)
		}
		if res.Run.AgentSessionID != row || w.current().AgentSessionID != row || res.Run.State != "running" {
			t.Fatalf("run %+v", res.Run)
		}
	})
	t.Run("no reference and zero row attach nothing", func(t *testing.T) {
		t.Parallel()
		w := newDeliveryWorld(t)
		w.withSessions(&sessions{})
		res, err := w.startAs(nil, "")
		if err != nil {
			t.Fatal(err)
		}
		calls := callsOf(w.world, "attach-on-start")
		if len(calls) != 1 || calls[0].attach.ExternalSessionRef != "" || !res.Run.AgentSessionID.IsZero() {
			t.Fatalf("AttachOnStart %+v, run %+v", calls, res.Run)
		}
	})
	t.Run("a validation error is returned unchanged", func(t *testing.T) {
		t.Parallel()
		w := newDeliveryWorld(t)
		bad := errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: "/external_session_ref", Code: "too_long"})
		w.withSessions(&sessions{err: bad})
		_, err := w.startAs(&ref, "")
		if e, ok := errs.As(err); !ok || e.Code != errs.CodeValidationFailed || !errors.Is(err, bad) {
			t.Fatalf("Start: %v", err)
		}
	})
	t.Run("without sessions a reference is ignored", func(t *testing.T) {
		t.Parallel()
		w := newDeliveryWorld(t)
		res, err := w.startAs(&ref, "")
		if err != nil || res.Run.State != "running" || !res.Run.AgentSessionID.IsZero() {
			t.Fatalf("Start: %+v %v", res.Run, err)
		}
	})
}

// TestResume_NeverAttaches (M2-12 design note 10): waiting_input -> running keeps the run's row
// and never asks AttachOnStart.
func TestResume_NeverAttaches(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	w.withSessions(&sessions{attach: ids.New()})
	if _, err := w.startAs(nil, ""); err != nil {
		t.Fatal(err)
	}
	w.setState(domain.StateWaitingInput)
	// The token of a follow-up delivery resumes a waiting run (AG-31).
	follow := ids.New()
	w.msgFacts.m[follow] = messaging.TriggerMessage{ID: follow, ThreadID: w.thread, Origin: "human", Kind: "text",
		Author: messaging.PrincipalRef{ID: w.alex, Kind: "user", Handle: "alex", DisplayName: "Alex"}}
	// An agent.followup is delivered only for an open follow-up row of the run (M2-11).
	if err := w.fus.InsertFollowup(context.Background(), w.run.ID, follow); err != nil {
		t.Fatal(err)
	}
	p, err := domain.FollowupPayload(w.run.ID, w.thread, follow)
	if err != nil {
		t.Fatal(err)
	}
	in := w.input(domain.EventAgentFollowup)
	in.Payload = p
	out := w.prepare(in)
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: w.lotse, Kind: actor.KindAgent})
	res, err := w.svc.Resume(ctx, runs.StartRun{RunID: w.run.ID, Token: out.InvocationToken, BillingModeUsed: "organization", ToolClassUsed: "chat"})
	if err != nil || res.Run.State != "running" {
		t.Fatalf("Resume: %+v %v", res.Run, err)
	}
	if n := len(callsOf(w.world, "attach-on-start")); n != 1 {
		t.Fatalf("%d AttachOnStart calls", n)
	}
}

// TestOnSessionAttached (M2-12 design notes 11 and 12): the subscriber stores the row in
// runs.agent_session_id and, for bind_failed, writes exactly one system line
// agent_session.resume_failed with the run's agent as author; without bind_failed no line. An
// unbound acceptor does nothing.
func TestOnSessionAttached(t *testing.T) {
	t.Parallel()
	w := newDeliveryWorld(t)
	first, second := ids.New(), ids.New()
	attached := func(ev agents.SessionAttached) {
		t.Helper()
		if err := (uow{}).Do(context.Background(), func(ctx context.Context) error { return w.acc.OnSessionAttached(ctx, ev) }); err != nil {
			t.Fatal(err)
		}
	}
	attached(agents.SessionAttached{ThreadID: w.thread, RunID: w.run.ID, AgentID: w.lotse, SessionID: first})
	if w.current().AgentSessionID != first || w.j.count("system-line") != 0 {
		t.Fatalf("after the first report: %+v, %v", w.current(), w.j.whats())
	}
	attached(agents.SessionAttached{ThreadID: w.thread, RunID: w.run.ID, AgentID: w.lotse, SessionID: second, ReplacedSessionID: first, BindFailed: true})
	if w.current().AgentSessionID != second {
		t.Fatalf("run %+v", w.current())
	}
	var lines []messaging.SystemLine
	for _, c := range w.j.list() {
		if c.what == "system-line" {
			lines = append(lines, c.arg.(messaging.SystemLine))
		}
	}
	if !slices.Equal(lines, []messaging.SystemLine{{ThreadID: w.thread, AuthorID: w.lotse, Code: "agent_session.resume_failed"}}) {
		t.Fatalf("system lines %+v", lines)
	}
	idle := app.NewAcceptor()
	if err := idle.OnSessionAttached(context.Background(), agents.SessionAttached{RunID: w.run.ID, SessionID: ids.New(), BindFailed: true}); err != nil {
		t.Fatal(err)
	}
	if w.current().AgentSessionID != second || w.j.count("system-line") != 1 {
		t.Fatal("an unbound acceptor wrote")
	}
}
