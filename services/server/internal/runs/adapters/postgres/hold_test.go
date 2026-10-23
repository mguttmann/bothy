// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// heldRun is a run of agent in thread held until held.
func heldRun(w world, thread, agent ids.ID, queued, held time.Time) app.NewRun {
	n := newRun(w, thread, agent, queued)
	n.InvocationHeldUntil = held
	return n
}

// TestRepository_HoldQueries: the release time reads back; the held-run lookup finds only the
// queued held runs of the message in ascending id; the release clears the column and raises the
// version once, a second release and the release of an ended held run change nothing.
func TestRepository_HoldQueries(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	ctx := context.Background()
	queued := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	held := queued.Add(3 * time.Minute)
	a := e.insert(t, heldRun(w, w.thread, w.robin, queued, held), true)
	b := e.insert(t, heldRun(w, w.other, w.sam, queued, held), true)
	if !a.InvocationHeldUntil.Equal(held) {
		t.Fatalf("held until %v, want %v", a.InvocationHeldUntil, held)
	}
	if read, _, _ := e.repo.Run(ctx, a.ID); !read.InvocationHeldUntil.Equal(held) {
		t.Fatalf("read back %v", read.InvocationHeldUntil)
	}
	third := ids.New()
	e.exec(t, `INSERT INTO threads (id, channel_id, kind, created_by) SELECT $1, channel_id, 'thread', created_by FROM threads WHERE id = $2`,
		third.String(), w.thread.String())
	if plain := e.insert(t, newRun(w, third, w.robin, queued), true); !plain.InvocationHeldUntil.IsZero() {
		t.Fatalf("a run never held has %v", plain.InvocationHeldUntil)
	}
	if _, err := e.repo.LockHeldRunsOfMessage(ctx, w.msg); err == nil {
		t.Fatal("LockHeldRunsOfMessage outside a unit of work")
	}
	var got []ids.ID
	e.do(t, func(ctx context.Context) error {
		var err error
		got, err = e.repo.LockHeldRunsOfMessage(ctx, w.msg)
		return err
	})
	lo, hi := a.ID, b.ID
	if hi.String() < lo.String() {
		lo, hi = hi, lo
	}
	if len(got) != 2 || got[0] != lo || got[1] != hi {
		t.Fatalf("held runs %v, want %v %v", got, lo, hi)
	}
	release := func(id ids.ID, want bool) {
		t.Helper()
		e.do(t, func(ctx context.Context) error {
			ok, err := e.repo.ReleaseHeldInvocation(ctx, id)
			if err == nil && ok != want {
				t.Errorf("ReleaseHeldInvocation %v, want %v", ok, want)
			}
			return err
		})
	}
	release(a.ID, true)
	release(a.ID, false)
	if r, _, _ := e.repo.Run(ctx, a.ID); !r.InvocationHeldUntil.IsZero() || r.Version != a.Version+1 || r.State != domain.StateQueued {
		t.Fatalf("released run %+v", r)
	}
	ended := e.transition(t, app.TransitionWrite{ID: b.ID, From: domain.StateQueued, Version: b.Version, To: domain.StateFailed,
		Reason: domain.RunReasonAgentUnavailable, At: queued.Add(time.Hour)}, true)
	release(b.ID, false)
	if r, _, _ := e.repo.Run(ctx, b.ID); !r.InvocationHeldUntil.Equal(held) || r.Version != ended.Version {
		t.Fatalf("ended run %+v", r)
	}
	e.do(t, func(ctx context.Context) error {
		var err error
		got, err = e.repo.LockHeldRunsOfMessage(ctx, w.msg)
		return err
	})
	if len(got) != 0 {
		t.Fatalf("held runs after release and end: %v", got)
	}
}

// TestInvocationHoldTimeout_Postgres: the use case of runs.invocation_hold_timeout over the real
// repository and unit of work: early it schedules itself again at the stored release time and
// changes nothing; at the release time it clears the column, raises the version and appends one
// agent.invocation without actor; a second run has no effect.
func TestInvocationHoldTimeout_Postgres(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	w := e.world(t)
	queued := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	held := queued.Add(3 * time.Minute)
	run := e.insert(t, heldRun(w, w.thread, w.robin, queued, held), true)
	clk := clock.NewFake(held.Add(-time.Second))
	evs, sched := &eventRecorder{}, &holdRecorder{}
	channel, space := ids.New(), ids.New()
	svc, err := app.New(app.Config{PageSizeDefault: 1, PageSizeMax: 1, StepDetailMaxBytes: 1, QueuedMaxAge: time.Hour, CancelGrace: time.Second,
		PresenceTimeout: time.Second, OrphanTimeout: time.Second, ReasonMaxChars: 1, ExcerptMaxChars: 1, MaxStepsPersisted: 1,
		EventPayloadMaxBytes: 1, ComponentDefaultTTL: time.Hour, ComponentMaxPerMessage: 1, ComponentCustomIDMaxChars: 1, ComponentFormMaxFields: 1,
		MaxContentBytes: 1, InvocationHoldMax: 3 * time.Minute}, app.Deps{
		Repo: e.repo, UoW: e.uow, Authorizer: denyAll{}, Events: evs, Messages: trigger{author: w.alex},
		Threads: threadInfo{channel: channel, space: space}, Channels: triggerChannel{}, Agents: agentFacts{w.robin},
		Cursors: noCursors{}, Clock: clk, Holds: sched,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := svc.InvocationHoldTimeout(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	if r, _, _ := e.repo.Run(ctx, run.ID); !r.InvocationHeldUntil.Equal(held) || r.Version != run.Version || len(evs.list()) != 0 {
		t.Fatalf("early: %+v events %d", r, len(evs.list()))
	}
	if at := sched.list(); len(at) != 1 || !at[0].Equal(held) {
		t.Fatalf("rescheduled %v", at)
	}
	clk.Advance(time.Second)
	for range 2 {
		if err := svc.InvocationHoldTimeout(ctx, run.ID); err != nil {
			t.Fatal(err)
		}
	}
	r, _, _ := e.repo.Run(ctx, run.ID)
	if !r.InvocationHeldUntil.IsZero() || r.Version != run.Version+1 || r.State != domain.StateQueued {
		t.Fatalf("released: %+v", r)
	}
	got := evs.list()
	if len(got) != 1 || got[0].Type != "agent.invocation" || got[0].ActorKind != "" || got[0].ResourceID != run.ID ||
		got[0].ChannelID != channel || got[0].AudiencePrincipalID != w.robin || len(sched.list()) != 1 {
		t.Fatalf("events %+v schedules %v", got, sched.list())
	}
}

// eventRecorder records the appended events.
type eventRecorder struct {
	mu  sync.Mutex
	evs []eventlog.Event
}

func (r *eventRecorder) Append(_ context.Context, evs ...eventlog.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evs = append(r.evs, evs...)
	return nil
}

func (r *eventRecorder) list() []eventlog.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]eventlog.Event(nil), r.evs...)
}

// holdRecorder records the scheduled release times.
type holdRecorder struct {
	mu sync.Mutex
	at []time.Time
}

func (h *holdRecorder) ScheduleInvocationHoldTimeout(_ context.Context, _ ids.ID, at time.Time) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.at = append(h.at, at)
	return nil
}

func (h *holdRecorder) list() []time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]time.Time(nil), h.at...)
}

// denyAll is an Authorizer the release never calls.
type denyAll struct{}

func (denyAll) Authorize(context.Context, authorize.Principal, authorize.Action, authorize.Resource) (authorize.Decision, error) {
	return authorize.Decision{Outcome: authorize.OutcomeForbidden}, nil
}

func (denyAll) Scope(context.Context, authorize.Principal, authorize.Action) (authorize.Scope, error) {
	return authorize.Scope{}, nil
}

func (denyAll) Observe(context.Context, authorize.LogEvent) {}

// trigger is the MessageWriter: the triggering message of Alex.
type trigger struct{ author ids.ID }

func (trigger) SetTriggerState(context.Context, messaging.SetTriggerStateCmd) error { return nil }

func (trigger) CreateRunAnchor(context.Context, messaging.RunAnchorCmd) (messaging.AnchorRef, error) {
	return messaging.AnchorRef{}, nil
}

func (t trigger) TriggerMessageOf(_ context.Context, id ids.ID) (messaging.TriggerMessage, error) {
	return messaging.TriggerMessage{ID: id, Origin: "human", Kind: "text",
		Author: messaging.PrincipalRef{ID: t.author, Kind: "user", Handle: "alex", DisplayName: "Alex"}}, nil
}

// AppendSystemLine is unused by the release; it writes no system line.
func (trigger) AppendSystemLine(context.Context, messaging.SystemLine) (messaging.Message, error) {
	return messaging.Message{}, nil
}

// PrincipalRefs is unused by the release; it renders no principal.
func (trigger) PrincipalRefs(context.Context, []ids.ID) (map[ids.ID]messaging.PrincipalRef, error) {
	return map[ids.ID]messaging.PrincipalRef{}, nil
}

// threadInfo places every thread in one channel of one space.
type threadInfo struct{ channel, space ids.ID }

func (t threadInfo) Info(_ context.Context, id ids.ID) (chat.ThreadInfo, error) {
	return chat.ThreadInfo{ID: id, ChannelID: t.channel, SpaceID: t.space, ChannelKind: "text", Kind: "main", State: "open"}, nil
}

func (t threadInfo) Locate(context.Context, []ids.ID) ([]chat.ThreadInfo, error) { return nil, nil }

func (threadInfo) LockForMessageChange(context.Context, ids.ID) error { return nil }

func (threadInfo) Create(context.Context, actor.Actor, chat.CreateThreadCmd) (chat.Thread, error) {
	return chat.Thread{}, nil
}

// triggerChannel answers every channel with settings_version 1.
type triggerChannel struct{}

func (triggerChannel) TriggerChannel(_ context.Context, id ids.ID) (chat.TriggerChannel, error) {
	return chat.TriggerChannel{ChannelID: id, Kind: "text", SettingsVersion: 1}, nil
}

// agentFacts knows the one agent as active in organization mode.
type agentFacts struct{ agent ids.ID }

func (a agentFacts) RunFacts(_ context.Context, list []ids.ID) (map[ids.ID]agents.RunFacts, error) {
	out := map[ids.ID]agents.RunFacts{}
	for _, id := range list {
		if id == a.agent {
			out[id] = agents.RunFacts{ID: id, Handle: "robin", Status: "active", BillingMode: "organization"}
		}
	}
	return out, nil
}

// noCursors is a Cursors the use case never calls here.
type noCursors struct{}

func (noCursors) Encode(string, []byte, any) (string, error) { return "", nil }

func (noCursors) Decode(string, []byte, string, any) error { return nil }
