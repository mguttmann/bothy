// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/agents"
	"github.com/OWNER/bothy/services/server/internal/agents/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// wantReason checks the public code and the reason of err.
func wantReason(t *testing.T, err error, code errs.Code, reason string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != code || e.Reason != reason {
		t.Fatalf("error %v (reason %q), want %s %s", err, reasonOf(err), code, reason)
	}
}

// reasonOf returns the reason of err, "" for none.
func reasonOf(err error) string {
	if e, ok := errs.As(err); ok {
		return e.Reason
	}
	return ""
}

// wantPointer checks validation_failed with the field pointer.
func wantPointer(t *testing.T, err error, pointer string) {
	t.Helper()
	e, ok := errs.As(err)
	if !ok || e.PublicCode() != errs.CodeValidationFailed || len(e.Fields) == 0 || e.Fields[0].Pointer != pointer {
		t.Fatalf("error %v, want validation_failed on %s", err, pointer)
	}
}

// changed is the decoded payload of agent_session.changed.
type changed struct {
	ThreadID     string `json:"thread_id"`
	AgentSession struct {
		ID         string `json:"id"`
		ThreadID   string `json:"thread_id"`
		IsCurrent  bool   `json:"is_current"`
		EndReason  string `json:"end_reason"`
		Version    int64  `json:"version"`
		CostBase   *int64 `json:"cost_baseline_micros"`
		ExternalID string `json:"external_session_ref"`
	} `json:"agent_session"`
}

// decodeChanged decodes the payload of ev and checks the envelope of design note 15: type, scope
// of the thread, resource thread without version, receive action thread.read.
func (e *sessEnv) decodeChanged(ev eventlog.Event) changed {
	e.t.Helper()
	info := e.threads[ev.ThreadID]
	if ev.Type != "agent_session.changed" || ev.SchemaVersion != 1 || ev.ResourceKind != "thread" || ev.ResourceID != ev.ThreadID ||
		ev.ResourceVersion != 0 || ev.ReceiveAction != "thread.read" || ev.SpaceID != info.SpaceID || ev.ChannelID != info.ChannelID {
		e.t.Fatalf("envelope %+v", ev)
	}
	var c changed
	if err := json.Unmarshal(ev.Payload, &c); err != nil {
		e.t.Fatal(err)
	}
	if c.ThreadID != ev.ThreadID.String() || c.AgentSession.ThreadID != c.ThreadID || c.AgentSession.CostBase != nil {
		e.t.Fatalf("payload %s", ev.Payload)
	}
	if strings.Contains(string(ev.Payload), "null") {
		e.t.Fatalf("payload with null %s", ev.Payload)
	}
	return c
}

// wantEventRows checks that the events carry exactly the rows want, in order, each with its
// version and end reason as stored.
func (e *sessEnv) wantEventRows(want ...agents.Session) []eventlog.Event {
	e.t.Helper()
	evs := e.eventsSnapshot()
	if len(evs) != len(want) {
		e.t.Fatalf("%d events, want %d", len(evs), len(want))
		return nil
	}
	for i, ev := range evs {
		c := e.decodeChanged(ev)
		w := want[i] //nolint:gosec // G602: the lengths were compared above and Fatalf returns
		if c.AgentSession.ID != w.ID.String() || ev.ThreadID != w.ThreadID || c.AgentSession.Version != w.Version ||
			c.AgentSession.EndReason != w.EndReason || c.AgentSession.IsCurrent != w.IsCurrent {
			e.t.Fatalf("event %d: %+v, want row %+v", i, c.AgentSession, w)
		}
	}
	return evs
}

// wantNothingChanged checks the rows against before and that no event, audit entry or
// SessionAttached was written.
func (e *sessEnv) wantNothingChanged(before []agents.Session) {
	e.t.Helper()
	after := e.store.all()
	if len(after) != len(before) {
		e.t.Fatalf("%d rows, want %d", len(after), len(before))
		return
	}
	for i := range after {
		if after[i] != before[i] { //nolint:gosec // G602: the lengths were compared above and Fatalf returns
			e.t.Fatalf("row %d changed: %+v, was %+v", i, after[i], before[i]) //nolint:gosec // G602: as above
		}
	}
	if n, a, p := len(e.eventsSnapshot()), len(e.auditSnapshot()), len(e.attachedSnapshot()); n+a+p != 0 {
		e.t.Fatalf("events %d, audit %d, attached %d, want none", n, a, p)
	}
}

// stored reads the row id from the store.
func (e *sessEnv) stored(id ids.ID) agents.Session {
	e.t.Helper()
	r, ok := e.store.get(id)
	if !ok {
		e.t.Fatalf("row %s missing", id)
	}
	return r
}

// now is the fake clock in the precision of the columns.
func (e *sessEnv) now() time.Time { return e.clk.Now().UTC().Truncate(time.Microsecond) }

// TestNewSessionsRefusesMissingCollaborators: every collaborator and a positive
// agent.session_ref_max_bytes are required; Runs, Clock and Log are optional.
func TestNewSessionsRefusesMissingCollaborators(t *testing.T) {
	t.Parallel()
	e := newSessEnv(t)
	full := app.SessionDeps{Store: e.store, Threads: e.threads, Channels: e.channels, UoW: e.uow, Authorizer: e.az, Audit: e.audit, Events: e.events}
	if _, err := app.NewSessions(app.SessionConfig{SessionRefMaxBytes: 1}, full); err != nil {
		t.Fatalf("NewSessions without the optional collaborators: %v", err)
	}
	if _, err := app.NewSessions(app.SessionConfig{SessionRefMaxBytes: 0}, full); err == nil {
		t.Fatal("NewSessions with agent.session_ref_max_bytes 0")
	}
	missing := full
	missing.Events = nil
	if _, err := app.NewSessions(app.SessionConfig{SessionRefMaxBytes: 1}, missing); err == nil {
		t.Fatal("NewSessions without Events")
	}
}

// TestSessionsList: an unknown thread is not_found; a denial of agent_session.read passes
// through; an empty history is a non-nil empty list; the rows come in the order of the store
// (created_at DESC, id DESC) and only those of the thread; the descriptor is the thread.
func TestSessionsList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("unknown thread", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		_, err := e.svc.List(ctx, e.alex, ids.New())
		wantCode(t, err, errs.CodeNotFound)
		_, err = e.svc.List(ctx, e.alex, ids.ID{})
		wantCode(t, err, errs.CodeNotFound)
	})
	t.Run("denials pass through", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.row(e.thread, e.lotse.ID, "sess-a", true, true, 0)
		e.deny(authorize.ActionAgentSessionRead, authorize.OutcomeNotFound, "not_visible")
		_, err := e.svc.List(ctx, e.sam, e.thread)
		wantCode(t, err, errs.CodeNotFound)
		e.deny(authorize.ActionAgentSessionRead, authorize.OutcomeForbidden, "level_insufficient")
		_, err = e.svc.List(ctx, e.sam, e.thread)
		wantReason(t, err, errs.CodeForbidden, "level_insufficient")
	})
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		got, err := e.svc.List(ctx, e.alex, e.thread)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("List = %#v, %v; want a non-nil empty list", got, err)
		}
	})
	t.Run("order and descriptor", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		old := e.row(e.thread, e.lotse.ID, "sess-a", false, true, 0)
		cur := e.row(e.thread, e.lotse.ID, "sess-c", true, true, 2*time.Hour)
		mid := e.row(e.thread, e.runner.ID, "sess-b", false, true, time.Hour)
		e.row(e.other, e.lotse.ID, "sess-x", true, true, 3*time.Hour)
		got, err := e.svc.List(ctx, e.alex, e.thread)
		if err != nil || len(got) != 3 || got[0].ID != cur.ID || got[1].ID != mid.ID || got[2].ID != old.ID || got[0] != cur {
			t.Fatalf("List = %+v, %v", got, err)
		}
		checks := e.checksSnapshot()
		if len(checks) != 1 || checks[0].Action != authorize.ActionAgentSessionRead || checks[0].Principal != e.alex.ID ||
			checks[0].Resource.Kind != authorize.ResourceThread || checks[0].Resource.ID != e.thread || checks[0].Resource.Invoke != nil {
			t.Fatalf("checks %+v", checks)
		}
		if len(e.eventsSnapshot())+len(e.auditSnapshot()) != 0 {
			t.Fatal("List wrote an event or an audit entry")
		}
	})
}

// TestSessionsReset: without a current row 409 no_current_session; the current row ends with
// reset (one version step, ended_at now), one agent_session.changed with the actor, no audit; the
// descriptor carries the agent.invoke facts of a session switch with the operator; a guard reason
// is 403 agent_invocation_denied with that reason, a level denial stays forbidden, and a denial
// changes nothing.
func TestSessionsReset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("no current session", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.row(e.thread, e.lotse.ID, "sess-a", false, true, 0)
		before := e.store.all()
		_, err := e.svc.Reset(ctx, e.alex, e.thread)
		wantReason(t, err, errs.CodeConflict, agents.ReasonNoCurrentSession)
		e.wantNothingChanged(before)
	})
	t.Run("unknown thread", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		_, err := e.svc.Reset(ctx, e.alex, ids.New())
		wantCode(t, err, errs.CodeNotFound)
	})
	t.Run("ends with reset", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		cur := e.row(e.thread, e.lotse.ID, "sess-a", true, true, 0)
		e.clk.Advance(time.Hour)
		got, err := e.svc.Reset(ctx, e.alex, e.thread)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != cur.ID || got.IsCurrent || got.EndReason != agents.EndReset || got.Version != cur.Version+1 || !got.EndedAt.Equal(e.now()) ||
			got != e.stored(cur.ID) {
			t.Fatalf("Reset = %+v", got)
		}
		evs := e.wantEventRows(got)
		if evs[0].ActorKind != eventlog.ActorUser || evs[0].ActorPrincipalID != e.alex.ID {
			t.Fatalf("actor %s %s", evs[0].ActorKind, evs[0].ActorPrincipalID)
		}
		if n := len(e.auditSnapshot()); n != 0 {
			t.Fatalf("%d audit entries, want none", n)
		}
		checks := e.checksSnapshot()
		inv := checks[0].Resource.Invoke
		if len(checks) != 1 || checks[0].Action != authorize.ActionAgentSessionReset || checks[0].Resource.Kind != authorize.ResourceThread ||
			checks[0].Resource.ID != e.thread || inv == nil || inv.AgentID != e.lotse.ID || inv.TriggerOrigin != "human" || !inv.MentionsAgent ||
			!inv.ActiveRunAgentID.IsZero() || !checks[0].Resource.TargetThreadID.IsZero() {
			t.Fatalf("checks %+v", checks)
		}
		_, err = e.svc.Reset(ctx, e.alex, e.thread)
		wantReason(t, err, errs.CodeConflict, agents.ReasonNoCurrentSession)
	})
	t.Run("operator of a main thread and of a thread without operator", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.channels[e.support] = e.runner.ID
		e.row(e.main, e.runner.ID, "sess-m", true, true, 0)
		if _, err := e.svc.Reset(ctx, e.alex, e.main); err != nil {
			t.Fatal(err)
		}
		_, err := e.svc.Reset(ctx, e.alex, e.noOp)
		wantReason(t, err, errs.CodeConflict, agents.ReasonNoCurrentSession)
		checks := e.checksSnapshot()
		if len(checks) != 2 || checks[0].Resource.Invoke.AgentID != e.runner.ID || !checks[1].Resource.Invoke.AgentID.IsZero() {
			t.Fatalf("checks %+v", checks)
		}
	})
	t.Run("denials", func(t *testing.T) {
		t.Parallel()
		for _, reason := range []authorize.Reason{
			"agent_not_reachable", "billing_mode_personal_not_holder", "tool_class_owner_only", "project_not_ready",
			"dev_environment_not_ready", "run_active_other_agent",
		} {
			e := newSessEnv(t)
			e.row(e.thread, e.lotse.ID, "sess-a", true, true, 0)
			before := e.store.all()
			e.deny(authorize.ActionAgentSessionReset, authorize.OutcomeForbidden, reason)
			_, err := e.svc.Reset(ctx, e.alex, e.thread)
			wantReason(t, err, errs.CodeAgentInvocationDenied, string(reason))
			e.wantNothingChanged(before)
		}
		for _, d := range []authorize.Decision{
			{Outcome: authorize.OutcomeForbidden, Reason: "level_insufficient"},
			{Outcome: authorize.OutcomeForbidden, Reason: "channel_read_only"},
			{Outcome: authorize.OutcomeNotFound, Reason: "not_visible"},
		} {
			e := newSessEnv(t)
			e.row(e.thread, e.lotse.ID, "sess-a", true, true, 0)
			before := e.store.all()
			e.deny(authorize.ActionAgentSessionReset, d.Outcome, d.Reason)
			_, err := e.svc.Reset(ctx, e.sam, e.thread)
			want, _ := errs.As(d.Err())
			wantReason(t, err, want.Code, want.Reason)
			if want.Code == errs.CodeAgentInvocationDenied {
				t.Fatalf("%s maps to agent_invocation_denied", d.Reason)
			}
			e.wantNothingChanged(before)
		}
	})
}

// TestSessionsBindCompareAndSwap: a stale or missing expected_current_session_id is 409
// session_changed and changes nothing (AG-17); a reference current in another thread without
// take_over is 409 session_bound_elsewhere after the authorization with the second target and
// changes nothing.
func TestSessionsBindCompareAndSwap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("stale expected id", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		cur := e.row(e.thread, e.lotse.ID, "sess-c", true, true, 0)
		ended := e.row(e.thread, e.lotse.ID, "sess-old", false, true, -time.Hour)
		before := e.store.all()
		for _, expected := range []ids.ID{ids.New(), {}, ended.ID} {
			_, err := e.svc.Bind(ctx, e.alex, e.thread, agents.BindCmd{ExternalSessionRef: "sess-new", ExpectedCurrentSessionID: expected})
			wantReason(t, err, errs.CodeConflict, agents.ReasonSessionChanged)
		}
		e.wantNothingChanged(before)
		// No current row: an expected id that names one is stale as well.
		e2 := newSessEnv(t)
		_, err := e2.svc.Bind(ctx, e2.alex, e2.thread, agents.BindCmd{ExternalSessionRef: "sess-new", ExpectedCurrentSessionID: cur.ID})
		wantReason(t, err, errs.CodeConflict, agents.ReasonSessionChanged)
		e2.wantNothingChanged(nil)
	})
	t.Run("bound elsewhere without take_over", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		cur := e.row(e.thread, e.lotse.ID, "sess-c", true, true, 0)
		e.row(e.other, e.lotse.ID, "sess-x", true, true, 0)
		before := e.store.all()
		_, err := e.svc.Bind(ctx, e.alex, e.thread, agents.BindCmd{ExternalSessionRef: "sess-x", ExpectedCurrentSessionID: cur.ID})
		wantReason(t, err, errs.CodeConflict, agents.ReasonSessionBoundElsewhere)
		e.wantNothingChanged(before)
		checks := e.checksSnapshot()
		if len(checks) != 1 || checks[0].Action != authorize.ActionAgentSessionBind || checks[0].Resource.TargetThreadID != e.other ||
			checks[0].Resource.Invoke == nil || checks[0].Resource.Invoke.AgentID != e.lotse.ID {
			t.Fatalf("checks %+v", checks)
		}
	})
	t.Run("the reference of another agent is no second target", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.row(e.other, e.runner.ID, "sess-x", true, true, 0)
		got, err := e.svc.Bind(ctx, e.alex, e.thread, agents.BindCmd{ExternalSessionRef: "sess-x"})
		if err != nil || !got.IsCurrent || got.AgentID != e.lotse.ID {
			t.Fatalf("Bind = %+v, %v", got, err)
		}
		if c := e.checksSnapshot(); !c[0].Resource.TargetThreadID.IsZero() {
			t.Fatalf("target %s", c[0].Resource.TargetThreadID)
		}
	})
}

// TestSessionsBindTakeOver: with take_over both rows end with rebind, the new row is current,
// started, at version 1, with the cost baseline of the most recent row of agent and reference;
// the events come ended rows by ascending id, then the new row, each in the scope of its thread;
// one allow audit entry with the IDs of design note 15 and never the reference.
func TestSessionsBindTakeOver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newSessEnv(t)
	// The row of the other thread is created first, so its id is lower than the current row of
	// the thread: the events must still come in ascending id.
	x := e.row(e.other, e.lotse.ID, "sess-x", true, true, 0)
	x.CumulativeCostMicros, x.CostBaselineMicros, x.Currency, x.Version = 700, 300, "EUR", 4
	x = e.store.put(x)
	cur := e.row(e.thread, e.lotse.ID, "sess-c", true, true, time.Minute)
	e.clk.Advance(time.Hour)
	got, err := e.svc.Bind(ctx, e.alex, e.thread, agents.BindCmd{ExternalSessionRef: "sess-x", ExpectedCurrentSessionID: cur.ID, TakeOver: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.ThreadID != e.thread || got.AgentID != e.lotse.ID || got.ExternalSessionRef != "sess-x" || !got.IsCurrent || !got.Started ||
		got.Version != 1 || got.CostBaselineMicros != 1000 || got.Currency != "EUR" || got.CumulativeCostMicros != 0 ||
		!got.CreatedAt.Equal(e.now()) || got != e.stored(got.ID) {
		t.Fatalf("Bind = %+v", got)
	}
	endedX, endedCur := e.stored(x.ID), e.stored(cur.ID)
	for _, r := range []agents.Session{endedX, endedCur} {
		if r.IsCurrent || r.EndReason != agents.EndRebind || !r.EndedAt.Equal(e.now()) {
			t.Fatalf("old row %+v", r)
		}
	}
	if endedX.Version != 5 || endedCur.Version != 2 {
		t.Fatalf("versions %d %d", endedX.Version, endedCur.Version)
	}
	evs := e.wantEventRows(endedX, endedCur, got)
	if evs[0].ThreadID != e.other || evs[1].ThreadID != e.thread || evs[2].ThreadID != e.thread {
		t.Fatalf("scopes %s %s %s", evs[0].ThreadID, evs[1].ThreadID, evs[2].ThreadID)
	}
	audit := e.auditSnapshot()
	if len(audit) != 1 {
		t.Fatalf("%d audit entries", len(audit))
	}
	a := audit[0].Entry
	if a.Action != "agent_session.bind" || a.Decision != auditing.DecisionAllow || a.ResourceKind != "thread" || a.ResourceID != e.thread ||
		audit[0].Actor.ID != e.alex.ID {
		t.Fatalf("audit %+v", audit[0])
	}
	wantAttrs(t, a.Attrs, map[string]any{
		"session_id": got.ID.String(), "ended_session_id": cur.ID.String(), "taken_over_session_id": x.ID.String(),
		"taken_over_thread_id": e.other.String(), "take_over": true, "cause": "bind",
	})
	checks := e.checksSnapshot()
	if len(checks) != 1 || checks[0].Resource.TargetThreadID != e.other {
		t.Fatalf("checks %+v", checks)
	}
	if n := len(e.attachedSnapshot()); n != 0 {
		t.Fatalf("Bind published %d SessionAttached", n)
	}
}

// wantAttrs checks that attrs hold exactly want and never a reference.
func wantAttrs(t *testing.T, attrs auditing.Attrs, want map[string]any) {
	t.Helper()
	if len(attrs) != len(want) {
		t.Fatalf("attrs %v, want %v", attrs, want)
	}
	for k, v := range want {
		got, ok := attrs[k]
		if !ok || got.Data() != v {
			t.Fatalf("attr %s = %v, want %v", k, got.Data(), v)
		}
	}
	for k, v := range attrs {
		if s, ok := v.Data().(string); ok && strings.HasPrefix(s, "sess-") {
			t.Fatalf("attr %s carries the reference", k)
		}
	}
}

// TestSessionsBindPaths: a bind without current row and without earlier rows starts at baseline
// 0 and the default currency, with an audit entry without ended and taken-over IDs; the baseline
// comes from the most recent row of agent and reference in any thread, ended included; an
// invalid reference is 400 only after the authorization; a guard denial is
// agent_invocation_denied; a target row that ended before the lock is no take-over; an insert
// that loses against a concurrent row maps to 409 and rolls back.
func TestSessionsBindPaths(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("first session", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		got, err := e.svc.Bind(ctx, e.alex, e.thread, agents.BindCmd{ExternalSessionRef: "sess-a"})
		if err != nil || got.CostBaselineMicros != 0 || got.Currency != sessDefaultCurrency || !got.Started || got.Version != 1 {
			t.Fatalf("Bind = %+v, %v", got, err)
		}
		e.wantEventRows(got)
		audit := e.auditSnapshot()
		if len(audit) != 1 {
			t.Fatalf("%d audit entries", len(audit))
		}
		wantAttrs(t, audit[0].Entry.Attrs, map[string]any{"session_id": got.ID.String(), "take_over": false, "cause": "bind"})
	})
	t.Run("baseline of the most recent row", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		older := e.ended(e.other, e.lotse.ID, "sess-a", agents.EndReset, 0)
		older.CumulativeCostMicros, older.Currency = 999, "GBP"
		e.store.put(older)
		latest := e.ended(e.other, e.lotse.ID, "sess-a", agents.EndRebind, time.Hour)
		latest.CumulativeCostMicros, latest.CostBaselineMicros, latest.Currency = 50, 20, "EUR"
		e.store.put(latest)
		got, err := e.svc.Bind(ctx, e.alex, e.thread, agents.BindCmd{ExternalSessionRef: "sess-a"})
		if err != nil || got.CostBaselineMicros != 70 || got.Currency != "EUR" {
			t.Fatalf("Bind = %+v, %v", got, err)
		}
	})
	t.Run("validation after authorization", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.deny(authorize.ActionAgentSessionBind, authorize.OutcomeForbidden, "level_insufficient")
		_, err := e.svc.Bind(ctx, e.sam, e.thread, agents.BindCmd{ExternalSessionRef: " "})
		wantReason(t, err, errs.CodeForbidden, "level_insufficient")
		e2 := newSessEnv(t)
		for _, ref := range []string{"", " \t", "sess\u0007", strings.Repeat("a", sessRefMax+1)} {
			_, err = e2.svc.Bind(ctx, e2.alex, e2.thread, agents.BindCmd{ExternalSessionRef: ref})
			wantPointer(t, err, "/external_session_ref")
		}
		e2.wantNothingChanged(nil)
	})
	t.Run("guard denial", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.deny(authorize.ActionAgentSessionBind, authorize.OutcomeForbidden, "billing_mode_personal_not_holder")
		_, err := e.svc.Bind(ctx, e.sam, e.thread, agents.BindCmd{ExternalSessionRef: "sess-a"})
		wantReason(t, err, errs.CodeAgentInvocationDenied, "billing_mode_personal_not_holder")
		_, err = e.svc.Bind(ctx, e.sam, ids.New(), agents.BindCmd{ExternalSessionRef: "sess-a"})
		wantCode(t, err, errs.CodeNotFound)
		e.wantNothingChanged(nil)
	})
	t.Run("target ended before the lock", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		x := e.row(e.other, e.lotse.ID, "sess-x", true, true, 0)
		e.store.beforeLock = func() {
			if _, ok, _ := e.store.EndSession(ctx, app.SessionEnd{ID: x.ID, Version: x.Version, Reason: agents.EndReset, At: start}); !ok {
				t.Error("could not end the target")
			}
		}
		got, err := e.svc.Bind(ctx, e.alex, e.thread, agents.BindCmd{ExternalSessionRef: "sess-x", TakeOver: true})
		if err != nil || !got.IsCurrent {
			t.Fatalf("Bind = %+v, %v", got, err)
		}
		if r := e.stored(x.ID); r.EndReason != agents.EndReset {
			t.Fatalf("target %+v ended again", r)
		}
		e.wantEventRows(got)
		wantAttrs(t, e.auditSnapshot()[0].Entry.Attrs, map[string]any{"session_id": got.ID.String(), "take_over": true, "cause": "bind"})
	})
	t.Run("insert loses", func(t *testing.T) {
		t.Parallel()
		for _, c := range []struct {
			name   string
			thread func(e *sessEnv) ids.ID
			reason string
		}{
			{"the thread got a current row", func(e *sessEnv) ids.ID { return e.thread }, agents.ReasonSessionChanged},
			{"the reference became current elsewhere", func(e *sessEnv) ids.ID { return e.other }, agents.ReasonSessionBoundElsewhere},
		} {
			e := newSessEnv(t)
			cur := e.row(e.thread, e.lotse.ID, "sess-c", true, true, 0)
			before := e.store.all()
			e.store.beforeInsert = func(app.NewSession) {
				// A concurrent transaction commits its row first; the use case ended cur in its
				// own transaction, which the conflict rolls back.
				e.store.mu.Lock()
				defer e.store.mu.Unlock()
				r := agents.Session{ID: ids.New(), ThreadID: c.thread(e), AgentID: e.lotse.ID, ExternalSessionRef: "sess-x", IsCurrent: true, Version: 1}
				e.store.rows[r.ID] = r
			}
			_, err := e.svc.Bind(ctx, e.alex, e.thread, agents.BindCmd{ExternalSessionRef: "sess-x", ExpectedCurrentSessionID: cur.ID})
			wantReason(t, err, errs.CodeConflict, c.reason)
			e.wantNothingChanged(before)
		}
	})
}

// TestSessionsReportRun: a report needs the active run of the reporting agent in running or
// waiting_input, and that agent must operate the thread; everything else is the not_found of an
// unknown thread. The descriptor is the session of the run with the run's agent as owner.
func TestSessionsReportRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	report := agents.ReportCmd{ExternalSessionRef: "sess-a"}
	e := newSessEnv(t)
	_, err := e.svc.Report(ctx, e.lotse, e.thread, report)
	wantCode(t, err, errs.CodeNotFound)
	_, err = e.svc.Report(ctx, e.lotse, ids.New(), report)
	wantCode(t, err, errs.CodeNotFound)
	e.run(e.thread, agents.ActiveRun{AgentID: e.runner.ID, State: "running"})
	_, err = e.svc.Report(ctx, e.lotse, e.thread, report)
	wantCode(t, err, errs.CodeNotFound)
	// The guest run of Runner in a thread Lotse operates has no session.
	_, err = e.svc.Report(ctx, e.runner, e.thread, report)
	wantCode(t, err, errs.CodeNotFound)
	e.run(e.thread, agents.ActiveRun{AgentID: e.lotse.ID, State: "queued"})
	_, err = e.svc.Report(ctx, e.lotse, e.thread, report)
	wantCode(t, err, errs.CodeNotFound)
	e.wantNothingChanged(nil)
	if n := len(e.checksSnapshot()); n != 0 {
		t.Fatalf("%d authorizations before the run check", n)
	}
	for _, state := range []string{"running", "waiting_input"} {
		e := newSessEnv(t)
		run := e.run(e.thread, agents.ActiveRun{AgentID: e.lotse.ID, State: state})
		if _, err := e.svc.Report(ctx, e.lotse, e.thread, report); err != nil {
			t.Fatalf("%s: %v", state, err)
		}
		c := e.checksSnapshot()
		if len(c) != 1 || c[0].Action != authorize.ActionAgentSessionReport || c[0].Resource.Kind != authorize.ResourceAgentSession ||
			!c[0].Resource.ID.IsZero() || c[0].Resource.ThreadID != e.thread || c[0].Resource.OwnerID != run.AgentID {
			t.Fatalf("checks %+v", c)
		}
	}
	e = newSessEnv(t)
	e.run(e.main, agents.ActiveRun{AgentID: e.lotse.ID, State: "running"})
	if _, err := e.svc.Report(ctx, e.lotse, e.main, report); err != nil {
		t.Fatalf("report in the main thread of the effective operator: %v", err)
	}
	e = newSessEnv(t)
	e.run(e.thread, agents.ActiveRun{AgentID: e.lotse.ID, State: "running"})
	e.deny(authorize.ActionAgentSessionReport, authorize.OutcomeForbidden, "not_self")
	_, err = e.svc.Report(ctx, e.lotse, e.thread, report)
	wantReason(t, err, errs.CodeForbidden, "not_self")
	e.wantNothingChanged(nil)
}

// TestSessionsReportOnRow: with a row R If-Match must name R's version (missing or different is
// 412 with current R); an unchanged report is 200 without version or event; a change raises the
// version once with one event; started false keeps started; a new reference current elsewhere
// is 409 session_bound_elsewhere; bind_failed on an unstarted R is 409 session_not_bound; the
// body is validated after the authorization.
func TestSessionsReportOnRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	setup := func(t *testing.T, started bool) (*sessEnv, agents.Session) {
		e := newSessEnv(t)
		r := e.row(e.thread, e.lotse.ID, "sess-a", true, started, 0)
		r.Model, r.Version = "model-large", 2
		r = e.store.put(r)
		e.run(e.thread, agents.ActiveRun{AgentID: e.lotse.ID, SessionID: r.ID, State: "running"})
		return e, r
	}
	t.Run("If-Match", func(t *testing.T) {
		t.Parallel()
		e, r := setup(t, true)
		before := e.store.all()
		for _, im := range []agents.Precondition{{}, match(1), match(3), {Present: true}} {
			_, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-a", Model: "model-small", IfMatch: im})
			wantCode(t, err, errs.CodeVersionConflict)
			ee, _ := errs.As(err)
			raw, ok := ee.Current.(json.RawMessage)
			want, _ := agents.SessionJSON(r)
			if !ok || string(raw) != string(want) {
				t.Fatalf("current %v, want %s", ee.Current, want)
			}
		}
		e.wantNothingChanged(before)
	})
	t.Run("unchanged", func(t *testing.T) {
		t.Parallel()
		e, r := setup(t, true)
		for _, cmd := range []agents.ReportCmd{
			{ExternalSessionRef: "sess-a", IfMatch: match(2)},
			{ExternalSessionRef: "sess-a", Model: "model-large", Started: true, IfMatch: match(2)},
		} {
			got, err := e.svc.Report(ctx, e.lotse, e.thread, cmd)
			if err != nil || got != r {
				t.Fatalf("Report = %+v, %v", got, err)
			}
		}
		e.wantNothingChanged([]agents.Session{r})
	})
	t.Run("changes", func(t *testing.T) {
		t.Parallel()
		e, r := setup(t, false)
		got, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-b", Model: "model-small", Started: true, IfMatch: match(2)})
		if err != nil || got.ID != r.ID || got.ExternalSessionRef != "sess-b" || got.Model != "model-small" || !got.Started || got.Version != 3 ||
			got != e.stored(r.ID) {
			t.Fatalf("Report = %+v, %v", got, err)
		}
		got2, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-b", IfMatch: match(3)})
		if err != nil || !got2.Started || got2.Model != "model-small" || got2.Version != 3 {
			t.Fatalf("started false or absent model changed the row: %+v, %v", got2, err)
		}
		evs := e.wantEventRows(got)
		if evs[0].ActorKind != eventlog.ActorAgent || evs[0].ActorPrincipalID != e.lotse.ID {
			t.Fatalf("actor %s %s", evs[0].ActorKind, evs[0].ActorPrincipalID)
		}
		if n := len(e.attachedSnapshot()) + len(e.auditSnapshot()); n != 0 {
			t.Fatalf("%d SessionAttached or audit entries", n)
		}
	})
	t.Run("reference current elsewhere", func(t *testing.T) {
		t.Parallel()
		e, _ := setup(t, true)
		e.row(e.other, e.lotse.ID, "sess-x", true, true, 0)
		before := e.store.all()
		_, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-x", IfMatch: match(2)})
		wantReason(t, err, errs.CodeConflict, agents.ReasonSessionBoundElsewhere)
		e.wantNothingChanged(before)
	})
	t.Run("reference change of an ended row", func(t *testing.T) {
		t.Parallel()
		// A reset during the run ended R; its reports still reach it (design note 7 d).
		e, r := setup(t, true)
		ended, _, _ := e.store.EndSession(ctx, app.SessionEnd{ID: r.ID, Version: 2, Reason: agents.EndReset, At: start})
		got, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-b", IfMatch: match(ended.Version)})
		if err != nil || got.ExternalSessionRef != "sess-b" || got.IsCurrent || got.Version != ended.Version+1 {
			t.Fatalf("Report = %+v, %v", got, err)
		}
	})
	t.Run("bind_failed on an unstarted row", func(t *testing.T) {
		t.Parallel()
		e, r := setup(t, false)
		_, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-b", BindFailed: agents.BindFailedSessionMissing, IfMatch: match(2)})
		wantReason(t, err, errs.CodeConflict, agents.ReasonSessionNotBound)
		e.wantNothingChanged([]agents.Session{r})
	})
	t.Run("validation", func(t *testing.T) {
		t.Parallel()
		e, r := setup(t, true)
		for _, c := range []struct {
			cmd     agents.ReportCmd
			pointer string
		}{
			{agents.ReportCmd{ExternalSessionRef: "", IfMatch: match(2)}, "/external_session_ref"},
			{agents.ReportCmd{ExternalSessionRef: "sess-\u202e", IfMatch: match(2)}, "/external_session_ref"},
			{agents.ReportCmd{ExternalSessionRef: "sess-a", Model: "model large", IfMatch: match(2)}, "/model"},
			{agents.ReportCmd{ExternalSessionRef: "sess-a", BindFailed: "gone", IfMatch: match(2)}, "/bind_failed"},
		} {
			_, err := e.svc.Report(ctx, e.lotse, e.thread, c.cmd)
			wantPointer(t, err, c.pointer)
		}
		e.wantNothingChanged([]agents.Session{r})
		e.deny(authorize.ActionAgentSessionReport, authorize.OutcomeForbidden, "not_self")
		_, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: ""})
		wantReason(t, err, errs.CodeForbidden, "not_self")
	})
}

// TestSessionsFirstReport: without a row If-Match is 412 without current and bind_failed is 409
// session_not_bound; otherwise the report inserts a current row at version 1 with the reported
// values (started absent is false), appends one event and publishes SessionAttached; an insert
// that loses is 409 session_changed or session_bound_elsewhere.
func TestSessionsFirstReport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	setup := func(t *testing.T) (*sessEnv, agents.ActiveRun) {
		e := newSessEnv(t)
		return e, e.run(e.thread, agents.ActiveRun{AgentID: e.lotse.ID, State: "running"})
	}
	t.Run("If-Match and bind_failed", func(t *testing.T) {
		t.Parallel()
		e, _ := setup(t)
		_, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-a", IfMatch: match(1)})
		wantCode(t, err, errs.CodeVersionConflict)
		if ee, _ := errs.As(err); ee.Current != nil {
			t.Fatalf("current %v, want none", ee.Current)
		}
		_, err = e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-a", BindFailed: agents.BindFailedSessionMissing})
		wantReason(t, err, errs.CodeConflict, agents.ReasonSessionNotBound)
		e.wantNothingChanged(nil)
	})
	t.Run("creates the row", func(t *testing.T) {
		t.Parallel()
		e, run := setup(t)
		e.clk.Advance(time.Minute)
		got, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-a", Model: "model-large"})
		if err != nil || got.ThreadID != e.thread || got.AgentID != e.lotse.ID || got.ExternalSessionRef != "sess-a" || got.Model != "model-large" ||
			got.Started || !got.IsCurrent || got.Version != 1 || got.CostBaselineMicros != 0 || !got.CreatedAt.Equal(e.now()) || got != e.stored(got.ID) {
			t.Fatalf("Report = %+v, %v", got, err)
		}
		e.wantEventRows(got)
		att := e.attachedSnapshot()
		if len(att) != 1 || att[0] != (agents.SessionAttached{ThreadID: e.thread, RunID: run.RunID, AgentID: e.lotse.ID, SessionID: got.ID}) {
			t.Fatalf("SessionAttached %+v", att)
		}
		if n := len(e.auditSnapshot()); n != 0 {
			t.Fatalf("%d audit entries", n)
		}
	})
	t.Run("a bind during the run made a row", func(t *testing.T) {
		t.Parallel()
		e, _ := setup(t)
		e.row(e.thread, e.lotse.ID, "sess-b", true, true, 0)
		before := e.store.all()
		_, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-a"})
		wantReason(t, err, errs.CodeConflict, agents.ReasonSessionChanged)
		e.wantNothingChanged(before)
	})
	t.Run("the reference is current elsewhere", func(t *testing.T) {
		t.Parallel()
		e, _ := setup(t)
		e.row(e.other, e.lotse.ID, "sess-a", true, true, 0)
		before := e.store.all()
		_, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-a"})
		wantReason(t, err, errs.CodeConflict, agents.ReasonSessionBoundElsewhere)
		e.wantNothingChanged(before)
	})
	t.Run("the session of the run no longer exists", func(t *testing.T) {
		t.Parallel()
		// runs.agent_session_id names a row that is gone (ON DELETE SET NULL raced): the report
		// is a first report again.
		e := newSessEnv(t)
		e.run(e.thread, agents.ActiveRun{AgentID: e.lotse.ID, SessionID: ids.New(), State: "running"})
		got, err := e.svc.Report(ctx, e.lotse, e.thread, agents.ReportCmd{ExternalSessionRef: "sess-a", Started: true})
		if err != nil || !got.Started || !got.IsCurrent {
			t.Fatalf("Report = %+v, %v", got, err)
		}
	})
}

// TestSessionsReportBindFailed: bind_failed on a current started R ends R with rebind and
// inserts a current N above R's version with cost baseline 0 (events R then N); when a reset
// ended R during the run, N is inserted already ended with R's end_reason; both publish
// SessionAttached with BindFailed and the replaced row; a duplicate with R's If-Match fails.
func TestSessionsReportBindFailed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	setup := func(t *testing.T) (*sessEnv, agents.Session, agents.ActiveRun) {
		e := newSessEnv(t)
		r := e.row(e.thread, e.lotse.ID, "sess-import", true, true, 0)
		r.Version, r.CostBaselineMicros, r.Currency = 3, 400, "EUR"
		r = e.store.put(r)
		return e, r, e.run(e.thread, agents.ActiveRun{AgentID: e.lotse.ID, SessionID: r.ID, State: "running"})
	}
	cmd := agents.ReportCmd{ExternalSessionRef: "sess-new", Model: "model-large", Started: true, BindFailed: agents.BindFailedSessionMissing, IfMatch: match(3)}
	t.Run("current row", func(t *testing.T) {
		t.Parallel()
		e, r, run := setup(t)
		e.clk.Advance(time.Minute)
		n, err := e.svc.Report(ctx, e.lotse, e.thread, cmd)
		if err != nil {
			t.Fatal(err)
		}
		if n.ID == r.ID || !n.IsCurrent || n.ExternalSessionRef != "sess-new" || n.Model != "model-large" || !n.Started ||
			n.Version <= r.Version || n.CostBaselineMicros != 0 || n.Currency != sessDefaultCurrency || n != e.stored(n.ID) {
			t.Fatalf("N = %+v", n)
		}
		ended := e.stored(r.ID)
		if ended.IsCurrent || ended.EndReason != agents.EndRebind || !ended.EndedAt.Equal(e.now()) || ended.Version != r.Version+1 {
			t.Fatalf("R = %+v", ended)
		}
		e.wantEventRows(ended, n)
		att := e.attachedSnapshot()
		want := agents.SessionAttached{ThreadID: e.thread, RunID: run.RunID, AgentID: e.lotse.ID, SessionID: n.ID, ReplacedSessionID: r.ID, BindFailed: true}
		if len(att) != 1 || att[0] != want {
			t.Fatalf("SessionAttached %+v, want %+v", att, want)
		}
		// runs attaches N; a concurrent duplicate with R's If-Match fails on N and on R.
		for _, sid := range []ids.ID{n.ID, r.ID} {
			e.run(e.thread, agents.ActiveRun{RunID: run.RunID, AgentID: e.lotse.ID, SessionID: sid, State: "running"})
			_, err = e.svc.Report(ctx, e.lotse, e.thread, cmd)
			wantCode(t, err, errs.CodeVersionConflict)
		}
		if len(e.attachedSnapshot()) != 1 || len(e.eventsSnapshot()) != 2 {
			t.Fatal("the duplicate wrote")
		}
	})
	t.Run("row ended during the run", func(t *testing.T) {
		t.Parallel()
		e, r, run := setup(t)
		ended, _, _ := e.store.EndSession(ctx, app.SessionEnd{ID: r.ID, Version: r.Version, Reason: agents.EndReset, At: start})
		// After the reset another session became current; N must not disturb it.
		other := e.row(e.thread, e.lotse.ID, "sess-other", true, true, time.Hour)
		e.clk.Advance(2 * time.Hour)
		c := cmd
		c.IfMatch = match(ended.Version)
		n, err := e.svc.Report(ctx, e.lotse, e.thread, c)
		if err != nil {
			t.Fatal(err)
		}
		if n.IsCurrent || n.EndReason != agents.EndReset || !n.EndedAt.Equal(e.now()) || n.Version <= ended.Version || n.ExternalSessionRef != "sess-new" {
			t.Fatalf("N = %+v", n)
		}
		if e.stored(r.ID) != ended || e.stored(other.ID) != other {
			t.Fatal("R or the current row changed")
		}
		e.wantEventRows(n)
		att := e.attachedSnapshot()
		if len(att) != 1 || att[0].SessionID != n.ID || att[0].ReplacedSessionID != r.ID || !att[0].BindFailed || att[0].RunID != run.RunID {
			t.Fatalf("SessionAttached %+v", att)
		}
	})
	t.Run("new reference current elsewhere", func(t *testing.T) {
		t.Parallel()
		e, r, _ := setup(t)
		e.row(e.other, e.lotse.ID, "sess-new", true, true, 0)
		before := e.store.all()
		_, err := e.svc.Report(ctx, e.lotse, e.thread, cmd)
		wantReason(t, err, errs.CodeConflict, agents.ReasonSessionBoundElsewhere)
		e.wantNothingChanged(before)
		if !e.stored(r.ID).IsCurrent {
			t.Fatal("R ended")
		}
	})
}

// TestSessionsForInvocation: outside a unit of work it refuses; the current row of another agent
// ends with agent_changed (event with the triggering human as actor); the own current row comes
// back unchanged; otherwise an import row of the operator binds once (newest, started, model,
// baseline and currency of the import row, audit cause import with the triggering human), never
// after another row, never against a reference current elsewhere, and a concurrent first trigger
// takes the row the other one bound.
func TestSessionsForInvocation(t *testing.T) {
	t.Parallel()
	q := func(e *sessEnv) agents.InvocationSessionQuery {
		return agents.InvocationSessionQuery{ThreadID: e.thread, AgentID: e.lotse.ID, TriggeredBy: e.alex.ID}
	}
	invoke := func(e *sessEnv, q agents.InvocationSessionQuery) (agents.InvocationSession, error) {
		var out agents.InvocationSession
		err := e.inUnit(func(ctx context.Context) error {
			var err error
			out, err = e.svc.ForInvocation(ctx, q)
			return err
		})
		return out, err
	}
	t.Run("outside a unit of work", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		_, err := e.svc.ForInvocation(context.Background(), q(e))
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("another agent", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		cur := e.row(e.thread, e.runner.ID, "sess-r", true, true, 0)
		e.clk.Advance(time.Hour)
		got, err := invoke(e, q(e))
		if err != nil || got.Row != nil {
			t.Fatalf("ForInvocation = %+v, %v", got, err)
		}
		ended := e.stored(cur.ID)
		if ended.IsCurrent || ended.EndReason != agents.EndAgentChanged || !ended.EndedAt.Equal(e.now()) || ended.Version != cur.Version+1 {
			t.Fatalf("row %+v", ended)
		}
		evs := e.wantEventRows(ended)
		if evs[0].ActorKind != eventlog.ActorUser || evs[0].ActorPrincipalID != e.alex.ID {
			t.Fatalf("actor %s %s", evs[0].ActorKind, evs[0].ActorPrincipalID)
		}
		if len(e.checksSnapshot())+len(e.auditSnapshot()) != 0 {
			t.Fatal("ForInvocation authorized or audited")
		}
	})
	t.Run("own row", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		cur := e.row(e.thread, e.lotse.ID, "sess-a", true, false, 0)
		got, err := invoke(e, q(e))
		if err != nil || got.Row == nil || *got.Row != cur {
			t.Fatalf("ForInvocation = %+v, %v", got, err)
		}
		e.wantNothingChanged([]agents.Session{cur})
	})
	t.Run("nothing", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		got, err := invoke(e, q(e))
		if err != nil || got.Row != nil {
			t.Fatalf("ForInvocation = %+v, %v", got, err)
		}
		_, err = invoke(e, agents.InvocationSessionQuery{ThreadID: ids.New(), AgentID: e.lotse.ID})
		wantCode(t, err, errs.CodeNotFound)
	})
	t.Run("import binding once", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.ended(e.thread, e.lotse.ID, "sess-old", agents.EndImport, -2*time.Hour)
		newest := e.ended(e.thread, e.lotse.ID, "sess-imp", agents.EndImport, -time.Hour)
		newest.Model, newest.CostBaselineMicros, newest.Currency = "model-large", 1234, "EUR"
		newest = e.store.put(newest)
		e.ended(e.thread, e.runner.ID, "sess-runner", agents.EndImport, 0)
		got, err := invoke(e, q(e))
		if err != nil || got.Row == nil {
			t.Fatalf("ForInvocation = %+v, %v", got, err)
		}
		r := *got.Row
		if r.ExternalSessionRef != "sess-imp" || !r.Started || !r.IsCurrent || r.Model != "model-large" || r.CostBaselineMicros != 1234 ||
			r.Currency != "EUR" || r.AgentID != e.lotse.ID || r.Version != 1 || r != e.stored(r.ID) {
			t.Fatalf("bound row %+v", r)
		}
		audit := e.auditSnapshot()
		if len(audit) != 1 || audit[0].Entry.Action != "agent_session.bind" || audit[0].Entry.Decision != auditing.DecisionAllow ||
			audit[0].Entry.ResourceID != e.thread || audit[0].Actor.ID != e.alex.ID {
			t.Fatalf("audit %+v", audit)
		}
		wantAttrs(t, audit[0].Entry.Attrs, map[string]any{"session_id": r.ID.String(), "take_over": false, "cause": "import"})
		evs := e.wantEventRows(r)
		if evs[0].ActorPrincipalID != e.alex.ID {
			t.Fatalf("actor %s", evs[0].ActorPrincipalID)
		}
		again, err := invoke(e, q(e))
		if err != nil || again.Row == nil || again.Row.ID != r.ID || len(e.auditSnapshot()) != 1 || len(e.eventsSnapshot()) != 1 {
			t.Fatalf("second ForInvocation = %+v, %v", again, err)
		}
	})
	t.Run("a reset prevents the import binding", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.ended(e.thread, e.lotse.ID, "sess-imp", agents.EndImport, -time.Hour)
		e.ended(e.thread, e.lotse.ID, "sess-a", agents.EndReset, 0)
		before := e.store.all()
		got, err := invoke(e, q(e))
		if err != nil || got.Row != nil {
			t.Fatalf("ForInvocation = %+v, %v", got, err)
		}
		e.wantNothingChanged(before)
	})
	t.Run("an import row of another agent", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.ended(e.thread, e.runner.ID, "sess-imp", agents.EndImport, 0)
		before := e.store.all()
		got, err := invoke(e, q(e))
		if err != nil || got.Row != nil {
			t.Fatalf("ForInvocation = %+v, %v", got, err)
		}
		e.wantNothingChanged(before)
	})
	t.Run("an agent change prevents the import binding", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.ended(e.thread, e.lotse.ID, "sess-imp", agents.EndImport, -time.Hour)
		cur := e.row(e.thread, e.runner.ID, "sess-r", true, true, 0)
		got, err := invoke(e, q(e))
		if err != nil || got.Row != nil {
			t.Fatalf("ForInvocation = %+v, %v", got, err)
		}
		e.wantEventRows(e.stored(cur.ID))
		if n := len(e.auditSnapshot()); n != 0 {
			t.Fatalf("%d audit entries", n)
		}
	})
	t.Run("import reference current elsewhere", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.ended(e.thread, e.lotse.ID, "sess-imp", agents.EndImport, -time.Hour)
		e.row(e.other, e.lotse.ID, "sess-imp", true, true, 0)
		before := e.store.all()
		got, err := invoke(e, q(e))
		if err != nil || got.Row != nil {
			t.Fatalf("ForInvocation = %+v, %v", got, err)
		}
		e.wantNothingChanged(before)
	})
	t.Run("a concurrent first trigger bound first", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.ended(e.thread, e.lotse.ID, "sess-imp", agents.EndImport, -time.Hour)
		var winner agents.Session
		e.store.beforeInsert = func(s app.NewSession) {
			s.ID = ids.New()
			winner, _, _ = e.store.InsertSession(context.Background(), s)
		}
		got, err := invoke(e, q(e))
		if err != nil || got.Row == nil || got.Row.ID != winner.ID {
			t.Fatalf("ForInvocation = %+v, %v; winner %s", got, err, winner.ID)
		}
		if n := len(e.auditSnapshot()) + len(e.eventsSnapshot()); n != 0 {
			t.Fatalf("the loser wrote %d entries", n)
		}
	})
	t.Run("the actor of the context stays", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.row(e.thread, e.runner.ID, "sess-r", true, true, 0)
		err := e.inUnit(func(ctx context.Context) error {
			_, err := e.svc.ForInvocation(actor.WithActor(ctx, e.sam), q(e))
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if evs := e.eventsSnapshot(); len(evs) != 1 || evs[0].ActorPrincipalID != e.sam.ID {
			t.Fatalf("events %+v", evs)
		}
	})
}

// TestSessionsAttachOnStart: outside a unit of work it refuses; a guest run gets zero and its
// reference, even an invalid one, is ignored; the current row of the agent is attached unchanged;
// no reference and no row gives zero; an earlier row of the thread with the reference is
// attached; otherwise a new unstarted current row with one event; an invalid reference or model
// is 400; an insert that loses attaches nothing and logs the IDs without the reference.
func TestSessionsAttachOnStart(t *testing.T) {
	t.Parallel()
	attach := func(e *sessEnv, cmd agents.AttachOnStartCmd) (ids.ID, error) {
		var out ids.ID
		err := e.inUnit(func(ctx context.Context) error {
			var err error
			out, err = e.svc.AttachOnStart(actor.WithActor(ctx, e.lotse), cmd)
			return err
		})
		return out, err
	}
	cmdOf := func(e *sessEnv, ref, model string) agents.AttachOnStartCmd {
		return agents.AttachOnStartCmd{ThreadID: e.thread, RunID: ids.New(), AgentID: e.lotse.ID, ExternalSessionRef: ref, Model: model}
	}
	t.Run("outside a unit of work", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		_, err := e.svc.AttachOnStart(context.Background(), cmdOf(e, "sess-a", ""))
		wantCode(t, err, errs.CodeInternalError)
	})
	t.Run("guest run", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		for _, cmd := range []agents.AttachOnStartCmd{
			{ThreadID: e.thread, RunID: ids.New(), AgentID: e.runner.ID, ExternalSessionRef: "sess\u0007"},
			{ThreadID: e.thread, RunID: ids.New(), AgentID: e.runner.ID, ExternalSessionRef: "sess-a"},
			{ThreadID: e.noOp, RunID: ids.New(), AgentID: e.lotse.ID, ExternalSessionRef: "sess-a"},
		} {
			got, err := attach(e, cmd)
			if err != nil || !got.IsZero() {
				t.Fatalf("AttachOnStart = %s, %v", got, err)
			}
		}
		e.wantNothingChanged(nil)
		_, err := attach(e, agents.AttachOnStartCmd{ThreadID: ids.New(), AgentID: e.lotse.ID})
		wantCode(t, err, errs.CodeNotFound)
	})
	t.Run("current row of the agent", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		cur := e.row(e.thread, e.lotse.ID, "sess-a", true, false, 0)
		for _, cmd := range []agents.AttachOnStartCmd{cmdOf(e, "sess-other", "model-small"), cmdOf(e, "", "")} {
			got, err := attach(e, cmd)
			if err != nil || got != cur.ID {
				t.Fatalf("AttachOnStart = %s, %v", got, err)
			}
		}
		e.wantNothingChanged([]agents.Session{cur})
	})
	t.Run("no reference and no row", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		got, err := attach(e, cmdOf(e, "", ""))
		if err != nil || !got.IsZero() {
			t.Fatalf("AttachOnStart = %s, %v", got, err)
		}
		e.wantNothingChanged(nil)
	})
	t.Run("earlier row with the reference", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.ended(e.thread, e.lotse.ID, "sess-a", agents.EndReset, 0)
		latest := e.ended(e.thread, e.lotse.ID, "sess-a", agents.EndRebind, time.Hour)
		before := e.store.all()
		got, err := attach(e, cmdOf(e, "sess-a", ""))
		if err != nil || got != latest.ID {
			t.Fatalf("AttachOnStart = %s, %v; want %s", got, err, latest.ID)
		}
		e.wantNothingChanged(before)
	})
	t.Run("new row", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.ended(e.other, e.lotse.ID, "sess-a", agents.EndReset, 0)
		e.clk.Advance(time.Minute)
		got, err := attach(e, cmdOf(e, "sess-a", "model-large"))
		if err != nil || got.IsZero() {
			t.Fatalf("AttachOnStart = %s, %v", got, err)
		}
		r := e.stored(got)
		if r.ThreadID != e.thread || r.AgentID != e.lotse.ID || r.ExternalSessionRef != "sess-a" || r.Model != "model-large" || r.Started ||
			!r.IsCurrent || r.Version != 1 || r.CostBaselineMicros != 0 || !r.CreatedAt.Equal(e.now()) {
			t.Fatalf("row %+v", r)
		}
		evs := e.wantEventRows(r)
		if evs[0].ActorKind != eventlog.ActorAgent || evs[0].ActorPrincipalID != e.lotse.ID {
			t.Fatalf("actor %s %s", evs[0].ActorKind, evs[0].ActorPrincipalID)
		}
		if n := len(e.auditSnapshot()) + len(e.attachedSnapshot()); n != 0 {
			t.Fatalf("%d audit entries or SessionAttached", n)
		}
	})
	t.Run("validation", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		_, err := attach(e, cmdOf(e, " ", ""))
		wantPointer(t, err, "/external_session_ref")
		_, err = attach(e, cmdOf(e, "sess\u0007", ""))
		wantPointer(t, err, "/external_session_ref")
		_, err = attach(e, cmdOf(e, strings.Repeat("a", sessRefMax+1), ""))
		wantPointer(t, err, "/external_session_ref")
		_, err = attach(e, cmdOf(e, "sess-a", "model large"))
		wantPointer(t, err, "/model")
		e.wantNothingChanged(nil)
	})
	t.Run("insert loses", func(t *testing.T) {
		t.Parallel()
		e := newSessEnv(t)
		e.row(e.other, e.lotse.ID, "sess-secret", true, true, 0)
		before := e.store.all()
		cmd := cmdOf(e, "sess-secret", "")
		got, err := attach(e, cmd)
		if err != nil || !got.IsZero() {
			t.Fatalf("AttachOnStart = %s, %v", got, err)
		}
		e.wantNothingChanged(before)
		line := e.log.String()
		if !strings.Contains(line, cmd.RunID.String()) || !strings.Contains(line, e.thread.String()) || strings.Contains(line, "sess-secret") {
			t.Fatalf("log %q", line)
		}
	})
}
