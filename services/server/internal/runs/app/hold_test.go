// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/runs/app"
	"github.com/OWNER/bothy/services/server/internal/runs/domain"
)

// invocations returns the agent.invocation events of the journal.
func invocations(w *world) []eventlog.Event {
	var out []eventlog.Event
	for _, c := range w.j.list() {
		if c.what == "event" && c.arg.(eventlog.Event).Type == "agent.invocation" {
			out = append(out, c.arg.(eventlog.Event))
		}
	}
	return out
}

// holds returns the scheduled runs.invocation_hold_timeout jobs.
func holds(w *world) []expiryAt {
	var out []expiryAt
	for _, c := range w.j.list() {
		if c.what == "hold" {
			out = append(out, c.arg.(expiryAt))
		}
	}
	return out
}

// postWithAttachment accepts a message of Alex with an attachment in the thread of the world that
// triggers Lotse; pending is the answer of files.Attachments.Pending for it.
func postWithAttachment(t *testing.T, w *world, pending bool) (messaging.MessagePosted, app.RunRow) {
	t.Helper()
	ev := w.posted(w.thread)
	ev.HasAttachments = true
	w.files.setPending(ev.MessageID, pending)
	w.mustPost(ev)
	if len(w.repo.runs) != 1 {
		t.Fatalf("runs %+v", w.repo.runs)
	}
	return ev, w.repo.runs[0]
}

// release runs ReleaseInvocation in a unit of work whose context carries the actor Alex and a
// request correlation id, which the released event must not take over.
func release(w *world, run ids.ID, cause domain.ReleaseCause) error {
	ctx := actor.WithActor(context.Background(), actor.Actor{ID: w.alex, Kind: actor.KindUser})
	ctx = reqmeta.WithCorrelationID(ctx, ids.New().String())
	return uow{}.Do(ctx, func(ctx context.Context) error { return w.svc.ReleaseInvocation(ctx, run, cause) })
}

// TestHold_NotPendingInvokesAtAcceptance: a message without attachments never asks files; one
// whose attachments are final gets its agent.invocation at acceptance, no hold column and no hold
// job.
func TestHold_NotPendingInvokesAtAcceptance(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.mustPost(w.posted(w.thread))
	if w.j.count("files:pending") != 0 || len(invocations(w)) != 1 {
		t.Fatalf("without attachments: %v", w.j.whats())
	}
	w = newWorld(t)
	ev, run := postWithAttachment(t, w, false)
	if w.j.count("files:pending") != 1 || len(invocations(w)) != 1 || !run.InvocationHeldUntil.IsZero() || len(holds(w)) != 0 {
		t.Fatalf("final attachments: run %+v journal %v", run, w.j.whats())
	}
	if got := w.j.list(); got[0].what == "files:pending" && got[0].arg != ev.MessageID {
		t.Fatalf("Pending asked for %v", got[0].arg)
	}
}

// TestHold_PendingHoldsAtAcceptance: a pending attachment holds the invocation: the run is queued
// with invocation_held_until = the clock plus file.invocation_hold_max, one hold job is scheduled
// at that time, trigger_state is received and runs.queued_expiry is scheduled as before.
func TestHold_PendingHoldsAtAcceptance(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ev, run := postWithAttachment(t, w, true)
	want := t0.Add(holdMax)
	if len(invocations(w)) != 0 || run.State != domain.StateQueued || !run.InvocationHeldUntil.Equal(want) {
		t.Fatalf("run %+v invocations %d", run, len(invocations(w)))
	}
	if h := holds(w); len(h) != 1 || h[0].run != run.ID || !h[0].at.Equal(want) {
		t.Fatalf("hold jobs %+v", h)
	}
	if w.j.count("expiry") != 1 {
		t.Fatalf("expiry jobs %d", w.j.count("expiry"))
	}
	tr := w.triggers()
	if last := tr[len(tr)-1]; last.MessageID != ev.MessageID || last.State != "received" {
		t.Fatalf("trigger %+v", tr)
	}
}

// TestReleaseInvocation: ReleaseFinal with nothing pending releases once (version + 1, column
// cleared, one agent.invocation without actor and correlation, the scope of the thread now, the
// current billing mode); with something pending it keeps holding; a second release is a no-op;
// ReleaseTimeout releases regardless and logs one Info line with the run id only; an ended run is
// never released.
func TestReleaseInvocation(t *testing.T) {
	t.Parallel()
	t.Run("final", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		ev, run := postWithAttachment(t, w, true)
		triggers := w.j.count("trigger")
		if err := release(w, run.ID, domain.ReleaseFinal); err != nil || len(invocations(w)) != 0 || w.repo.runs[0].Version != run.Version {
			t.Fatalf("pending: %v %+v", err, w.repo.runs[0])
		}
		moved := ids.New()
		w.threads.mu.Lock()
		in := w.threads.infos[w.thread]
		in.ChannelID = moved
		w.threads.infos[w.thread] = in
		w.threads.mu.Unlock()
		w.channels[moved] = chat.TriggerChannel{ChannelID: moved, SpaceID: w.space, Kind: "text", SettingsVersion: 7}
		f := w.agents[w.lotse]
		f.BillingMode = "personal"
		w.agents[w.lotse] = f
		w.files.setPending(ev.MessageID, false)
		if err := release(w, run.ID, domain.ReleaseFinal); err != nil {
			t.Fatal(err)
		}
		got := w.repo.runs[0]
		if got.Version != run.Version+1 || !got.InvocationHeldUntil.IsZero() || got.State != domain.StateQueued {
			t.Fatalf("run %+v", got)
		}
		evs := invocations(w)
		if len(evs) != 1 {
			t.Fatalf("invocations %+v", evs)
		}
		e := evs[0]
		if e.ActorKind != "" || !e.ActorPrincipalID.IsZero() || w.j.count("correlated") != 0 {
			t.Fatalf("actor %q %v, correlated %d", e.ActorKind, e.ActorPrincipalID, w.j.count("correlated"))
		}
		if e.ChannelID != moved || e.ThreadID != w.thread || e.AudiencePrincipalID != w.lotse || e.ResourceID != run.ID ||
			e.ReceiveAction != "agent.events.consume" || e.ResourceVersion != 0 {
			t.Fatalf("event %+v", e)
		}
		if !strings.Contains(string(e.Payload), `"billing_mode":"personal"`) || !strings.Contains(string(e.Payload), `"settings_version":7`) {
			t.Fatalf("payload %s", e.Payload)
		}
		if err := release(w, run.ID, domain.ReleaseFinal); err != nil || len(invocations(w)) != 1 || w.repo.runs[0].Version != run.Version+1 {
			t.Fatalf("second release: %v %d", err, len(invocations(w)))
		}
		if err := release(w, run.ID, domain.ReleaseTimeout); err != nil || len(invocations(w)) != 1 {
			t.Fatalf("timeout after release: %v %d", err, len(invocations(w)))
		}
		if w.j.count("trigger") != triggers {
			t.Fatalf("the release touched trigger_state: %+v", w.triggers())
		}
	})
	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		_, run := postWithAttachment(t, w, true)
		if err := release(w, run.ID, domain.ReleaseTimeout); err != nil || len(invocations(w)) != 1 || !w.repo.runs[0].InvocationHeldUntil.IsZero() {
			t.Fatalf("timeout: %v %d %+v", err, len(invocations(w)), w.repo.runs[0])
		}
		if w.j.count("files:pending") != 1 {
			t.Fatalf("the timeout read the attachments: %d", w.j.count("files:pending"))
		}
		lines := strings.Split(strings.TrimSpace(w.log.String()), "\n")
		if len(lines) != 1 || !strings.Contains(lines[0], `"level":"INFO"`) || !strings.Contains(lines[0], `"bothy.run.id":"`+run.ID.String()+`"`) {
			t.Fatalf("log %q", w.log.String())
		}
	})
	t.Run("ended run", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		ev, run := postWithAttachment(t, w, true)
		w.repo.runs[0].State = domain.StateFailed
		w.files.setPending(ev.MessageID, false)
		for _, c := range []domain.ReleaseCause{domain.ReleaseFinal, domain.ReleaseTimeout} {
			if err := release(w, run.ID, c); err != nil || len(invocations(w)) != 0 || w.repo.runs[0].InvocationHeldUntil.IsZero() {
				t.Fatalf("%v: %v %+v", c, err, w.repo.runs[0])
			}
		}
	})
	t.Run("unknown run and outside a unit", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t)
		if err := release(w, ids.New(), domain.ReleaseTimeout); err != nil {
			t.Fatal(err)
		}
		if err := w.svc.ReleaseInvocation(context.Background(), ids.New(), domain.ReleaseTimeout); errs.KindOf(err) != errs.KindInternal {
			t.Fatalf("outside a unit: %v", err)
		}
	})
}

// TestOnFileFinalized: an unattached file does nothing; an attached one releases the held runs of
// its message when nothing else is pending; an unbound acceptor does nothing.
func TestOnFileFinalized(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ev, run := postWithAttachment(t, w, true)
	fin := func(file ids.ID) error {
		return uow{}.Do(context.Background(), func(ctx context.Context) error {
			return w.acc.OnFileFinalized(ctx, files.FileFinalized{FileID: file})
		})
	}
	if err := fin(ids.New()); err != nil || w.j.count("files:message-of") != 1 || len(invocations(w)) != 0 {
		t.Fatalf("unattached: %v %v", err, w.j.whats())
	}
	file := ids.New()
	w.files.attach(file, ev.MessageID, w.thread, w.channel)
	if err := fin(file); err != nil || len(invocations(w)) != 0 {
		t.Fatalf("still pending: %v %d", err, len(invocations(w)))
	}
	w.files.setPending(ev.MessageID, false)
	if err := fin(file); err != nil || len(invocations(w)) != 1 || w.repo.runs[0].ID != run.ID || !w.repo.runs[0].InvocationHeldUntil.IsZero() {
		t.Fatalf("final: %v %d %+v", err, len(invocations(w)), w.repo.runs[0])
	}
	if err := fin(file); err != nil || len(invocations(w)) != 1 {
		t.Fatalf("again: %v %d", err, len(invocations(w)))
	}
	if err := app.NewAcceptor().OnFileFinalized(context.Background(), files.FileFinalized{FileID: file}); err != nil {
		t.Fatalf("unbound: %v", err)
	}
}

// TestInvocationHoldTimeout: the job use case does nothing for a released or ended run, schedules
// itself again at the stored release time when it runs early, and releases once at or after it.
func TestInvocationHoldTimeout(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	_, run := postWithAttachment(t, w, true)
	w.clk.Advance(holdMax - 1)
	if err := w.svc.InvocationHoldTimeout(context.Background(), run.ID); err != nil || len(invocations(w)) != 0 {
		t.Fatalf("early: %v %d", err, len(invocations(w)))
	}
	if h := holds(w); len(h) != 2 || !h[1].at.Equal(run.InvocationHeldUntil) {
		t.Fatalf("hold jobs %+v", h)
	}
	w.clk.Advance(1)
	if err := w.svc.InvocationHoldTimeout(context.Background(), run.ID); err != nil || len(invocations(w)) != 1 {
		t.Fatalf("due: %v %d", err, len(invocations(w)))
	}
	if err := w.svc.InvocationHoldTimeout(context.Background(), run.ID); err != nil || len(invocations(w)) != 1 || len(holds(w)) != 2 {
		t.Fatalf("again: %v %d %d", err, len(invocations(w)), len(holds(w)))
	}
	if err := w.svc.InvocationHoldTimeout(context.Background(), ids.New()); err != nil {
		t.Fatalf("unknown run: %v", err)
	}
}

// TestFollowOnRun_Held: the follow-on run of a deferred follow-up asks Pending for its triggering
// message unconditionally and is held while an attachment is pending.
func TestFollowOnRun_Held(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	run := openThreadRun(t, w)
	m := ids.New()
	w.msgFacts.m[m] = messaging.TriggerMessage{ID: m, ThreadID: w.thread, ChannelID: w.channel, Origin: "human", Kind: "text",
		Author: messaging.PrincipalRef{ID: w.alex, Kind: "user", Handle: "alex", DisplayName: "Alex"}, ThreadSeq: 4, TriggerState: "received"}
	if err := w.fus.InsertFollowup(context.Background(), run.ID, m); err != nil {
		t.Fatal(err)
	}
	w.files.setPending(m, true)
	before := len(invocations(w))
	w.clk.Advance(queuedMaxAge + 1)
	if err := w.svc.ExpireQueued(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	if len(w.repo.runs) != 2 || w.repo.runs[1].TriggeringMessageID != m {
		t.Fatalf("runs %+v", w.repo.runs)
	}
	next := w.repo.runs[1]
	if want := next.QueuedAt.Add(holdMax); !next.InvocationHeldUntil.Equal(want) || len(invocations(w)) != before {
		t.Fatalf("follow-on run %+v invocations %d", next, len(invocations(w)))
	}
	if h := holds(w); len(h) != 1 || h[0].run != next.ID {
		t.Fatalf("hold jobs %+v", h)
	}
}

// TestAccept_FilesOutSwitch: a message of an agent with attachments is refused with
// feature_disabled when the owner switched supports_files_out off; an absent key or true allows,
// and a message without attachments is never checked.
func TestAccept_FilesOutSwitch(t *testing.T) {
	t.Parallel()
	off, on := false, true
	for _, c := range []struct {
		name   string
		sw     *bool
		attach bool
		denied bool
	}{
		{"off with attachments", &off, true, true},
		{"absent", nil, true, false},
		{"on", &on, true, false},
		{"off without attachments", &off, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)
			f := w.agents[w.lotse]
			f.FilesOutPermitted = c.sw
			w.agents[w.lotse] = f
			ev := w.posted(w.thread)
			ev.AuthorID, ev.AuthorKind, ev.Origin, ev.HasAttachments = w.lotse, "agent", "agent", c.attach
			err := w.post(ev)
			if c.denied {
				fe, ok := errs.As(err)
				if !ok || fe.Code != errs.CodeFeatureDisabled || errs.KindOf(err) != errs.KindForbidden {
					t.Fatalf("err %v", err)
				}
				return
			}
			if err != nil || len(w.repo.runs) != 0 {
				t.Fatalf("err %v runs %+v", err, w.repo.runs)
			}
		})
	}
}
