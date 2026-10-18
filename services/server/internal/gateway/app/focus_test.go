// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// focused returns the focused thread ids of cl in the hub.
func (e *fenv) focused(cl *client) []ids.ID { return e.hub.FocusOf(cl.c) }

// sameSet reports whether a and b hold the same ids.
func sameSet(a, b []ids.ID) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	byText := func(p, q ids.ID) int { return strings.Compare(p.String(), q.String()) }
	slices.SortFunc(x, byText)
	slices.SortFunc(y, byText)
	return slices.Equal(x, y)
}

// TestFocus_OverLimitRejectedKeepsPrevious: more than sync.max_focused_threads distinct ids
// answer error validation_failed with the key as detail, the connection stays and the previous
// set stays (04 section 11.3 item 2); repeated ids count once.
func TestFocus_OverLimitRejectedKeepsPrevious(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	sam := e.newPerson()
	var threads []ids.ID
	for range maxFocused + 1 {
		th := e.w.thread(e.support)
		e.w.grant(sam.ID, th.ChannelID, levelRead)
		threads = append(threads, th.ID)
	}
	cl := e.join(sam)
	e.focus(cl, false, threads[0])
	e.focus(cl, false, threads...)
	wantOpen(t, cl.fc)
	errsSent := cl.errorFrames()
	if len(errsSent) != 1 || errsSent[0] != (app.ErrorFrame{Op: "focus", Code: "validation_failed", Detail: "sync.max_focused_threads"}) {
		t.Fatalf("error frames %+v", errsSent)
	}
	if got := e.focused(cl); !sameSet(got, threads[:1]) {
		t.Errorf("focus %v, want the previous set", got)
	}
	// Exactly the limit, with repetitions, is accepted.
	e.focus(cl, false, append(threads[:maxFocused:maxFocused], threads[0], threads[1])...)
	if got := e.focused(cl); !sameSet(got, threads[:maxFocused]) {
		t.Errorf("focus %v, want %v", got, threads[:maxFocused])
	}
	if n := len(cl.errorFrames()); n != 1 {
		t.Errorf("%d error frames", n)
	}
}

// TestFocus_UnreadableAndUnknownIgnoredSilently: only the readable ids are kept; unknown and
// unreadable ids give no error frame and are both checked by authorize.
func TestFocus_UnreadableAndUnknownIgnoredSilently(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	sam := e.newPerson()
	readable, foreign := e.w.thread(e.support), e.w.thread(e.support)
	e.w.grant(sam.ID, readable.ChannelID, levelRead)
	cl := e.join(sam)
	before := e.w.checks[authorize.ActionGatewayFocus]
	e.focus(cl, false, ids.New(), foreign.ID, readable.ID)
	wantOpen(t, cl.fc)
	if n := len(cl.errorFrames()); n != 0 {
		t.Errorf("%d error frames", n)
	}
	if got := e.focused(cl); !sameSet(got, []ids.ID{readable.ID}) {
		t.Errorf("focus %v, want only the readable thread", got)
	}
	if n := e.w.checks[authorize.ActionGatewayFocus] - before; n != 3 {
		t.Errorf("%d gateway.focus checks, want one per id", n)
	}
}

// TestFocus_ReplacesPreviousSet: every focus replaces the set; an empty one clears it and is
// accepted without a resource to check.
func TestFocus_ReplacesPreviousSet(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	sam := e.newPerson()
	a, b := e.w.thread(e.support), e.w.thread(e.support)
	e.w.grant(sam.ID, a.ChannelID, levelRead)
	e.w.grant(sam.ID, b.ChannelID, levelRead)
	cl := e.join(sam)
	e.focus(cl, false, a.ID)
	e.focus(cl, false, b.ID)
	if got := e.focused(cl); !sameSet(got, []ids.ID{b.ID}) {
		t.Errorf("focus %v, want only %v", got, b.ID)
	}
	e.focus(cl, false)
	wantOpen(t, cl.fc)
	if got := e.focused(cl); len(got) != 0 {
		t.Errorf("focus %v after an empty set", got)
	}
}

// TestFocus_IdleStored: idle is stored with the set of the session.
func TestFocus_IdleStored(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	sam := e.newPerson()
	a := e.w.thread(e.support)
	e.w.grant(sam.ID, a.ChannelID, levelRead)
	cl := e.join(sam)
	e.focus(cl, true, a.ID)
	if !e.gw.SessionOf(cl.c).FocusState().Idle {
		t.Error("idle not stored")
	}
	e.focus(cl, false, a.ID)
	if e.gw.SessionOf(cl.c).FocusState().Idle {
		t.Error("idle not replaced")
	}
}

// TestFocus_RateLimitedErrorFrame: focus counts rl.read keyed by the principal; over the limit
// it answers error rate_limited, keeps the connection and changes nothing.
func TestFocus_RateLimitedErrorFrame(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	sam := e.newPerson()
	a := e.w.thread(e.support)
	e.w.grant(sam.ID, a.ChannelID, levelRead)
	cl := e.join(sam)
	e.limiter.denyReads[sam.ID] = true
	e.focus(cl, false, a.ID)
	wantOpen(t, cl.fc)
	if got := cl.errorFrames(); len(got) != 1 || got[0] != (app.ErrorFrame{Op: "focus", Code: "rate_limited"}) {
		t.Fatalf("error frames %+v", got)
	}
	if got := e.focused(cl); len(got) != 0 {
		t.Errorf("focus %v despite the limit", got)
	}
	if !slices.Equal(e.limiter.readKeys, []ids.ID{sam.ID}) {
		t.Errorf("rl.read keys %v, want the principal", e.limiter.readKeys)
	}
}

// typingScene is Alex typing in a Support thread with two devices, Sam reading it.
type typingScene struct {
	e          *fenv
	th         ids.ID
	alex, sam  *client
	alexSecond *client
}

// newTypingScene builds the scene: Alex and Sam have write on the thread's channel; Sam and
// Alex's second device focus the thread.
func newTypingScene(t *testing.T) *typingScene {
	t.Helper()
	e := newFocusEnv(t)
	th := e.w.thread(e.support)
	alex, sam := e.newPerson(), e.newPerson()
	e.w.grant(alex.ID, th.ChannelID, levelWrite)
	e.w.grant(sam.ID, th.ChannelID, levelWrite)
	s := &typingScene{e: e, th: th.ID, alex: e.join(alex), sam: e.join(sam)}
	s.alexSecond = e.join(e.device(alex))
	e.focus(s.sam, false, th.ID)
	e.focus(s.alexSecond, false, th.ID)
	return s
}

// TestTyping_ThrottlePerConnectionAndThread: a second start inside sync.typing_min_interval does
// not fan out, one after the interval does, and after typing.stop the next start fans out at once.
func TestTyping_ThrottlePerConnectionAndThread(t *testing.T) {
	t.Parallel()
	s := newTypingScene(t)
	e := s.e
	count := func() int { return len(s.sam.events(t)) }
	e.typing(s.alex, "typing.start", s.th)
	e.clk.Advance(typingInterval - time.Millisecond)
	e.typing(s.alex, "typing.start", s.th)
	if n := count(); n != 1 {
		t.Fatalf("%d events inside the interval, want 1", n)
	}
	e.clk.Advance(time.Millisecond)
	e.typing(s.alex, "typing.start", s.th)
	if n := count(); n != 2 {
		t.Fatalf("%d events after the interval, want 2", n)
	}
	e.typing(s.alex, "typing.stop", s.th)
	if n := count(); n != 2 {
		t.Fatalf("typing.stop sent an event")
	}
	e.typing(s.alex, "typing.start", s.th)
	if n := count(); n != 3 {
		t.Fatalf("%d events after typing.stop, want 3", n)
	}
	wantOpen(t, s.alex.fc)
	if n := len(s.alex.errorFrames()); n != 0 {
		t.Errorf("%d error frames", n)
	}
}

// TestTyping_RateLimitedErrorFrame: typing.start and typing.stop count rl.typing keyed by the
// connection; over the limit they answer error rate_limited and publish nothing.
func TestTyping_RateLimitedErrorFrame(t *testing.T) {
	t.Parallel()
	s := newTypingScene(t)
	e := s.e
	e.limiter.denyTyping[s.alex.c.ID()] = true
	e.typing(s.alex, "typing.start", s.th)
	e.typing(s.alex, "typing.stop", s.th)
	wantOpen(t, s.alex.fc)
	want := []app.ErrorFrame{{Op: "typing.start", Code: "rate_limited"}, {Op: "typing.stop", Code: "rate_limited"}}
	if got := s.alex.errorFrames(); !slices.Equal(got, want) {
		t.Fatalf("error frames %+v", got)
	}
	if n := len(s.sam.events(t)); n != 0 {
		t.Errorf("%d events despite the limit", n)
	}
	if !slices.Equal(e.limiter.typingKeys, []ids.ID{s.alex.c.ID(), s.alex.c.ID()}) {
		t.Errorf("rl.typing keys %v, want the connection", e.limiter.typingKeys)
	}
}

// TestTyping_OnlyPeople: an agent with write gets error forbidden and nothing is published
// (requirement 13 of the brief).
func TestTyping_OnlyPeople(t *testing.T) {
	t.Parallel()
	s := newTypingScene(t)
	e := s.e
	agent := e.newAgent()
	e.w.grant(agent.ID, e.w.threads[s.th].ChannelID, levelWrite)
	cl := e.join(agent)
	e.typing(cl, "typing.start", s.th)
	wantOpen(t, cl.fc)
	if got := cl.errorFrames(); len(got) != 1 || got[0].Code != "forbidden" || got[0].Op != "typing.start" {
		t.Fatalf("error frames %+v", got)
	}
	if n := len(s.sam.events(t)); n != 0 {
		t.Errorf("%d events from an agent", n)
	}
}

// TestTyping_DeniedCodes: a denial answers the code of the decision.
func TestTyping_DeniedCodes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(e *fenv, p ids.ID) ids.ID
		want  string
	}{
		{"unreadable thread", func(e *fenv, _ ids.ID) ids.ID { return e.w.thread(e.support).ID }, "not_found"},
		{"unknown thread", func(*fenv, ids.ID) ids.ID { return ids.New() }, "not_found"},
		{"read grant", func(e *fenv, p ids.ID) ids.ID {
			th := e.w.thread(e.support)
			e.w.grant(p, th.ChannelID, levelRead)
			return th.ID
		}, "forbidden"},
		{"read-only channel", func(e *fenv, p ids.ID) ids.ID {
			th := e.w.thread(e.support)
			e.w.grant(p, th.ChannelID, levelWrite)
			e.w.readOnly[th.ChannelID] = true
			return th.ID
		}, "channel_read_only"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := newFocusEnv(t)
			alex := e.newPerson()
			th := tc.setup(e, alex.ID)
			cl := e.join(alex)
			e.typing(cl, "typing.start", th)
			wantOpen(t, cl.fc)
			if got := cl.errorFrames(); len(got) != 1 || got[0] != (app.ErrorFrame{Op: "typing.start", Code: tc.want}) {
				t.Fatalf("error frames %+v, want %s", got, tc.want)
			}
		})
	}
}

// TestTypingStarted_OnlyFocusedReaders: the focused reader receives typing.started; a reader
// without focus, a focused connection without read permission and the typist's own focused
// second connection do not.
func TestTypingStarted_OnlyFocusedReaders(t *testing.T) {
	t.Parallel()
	s := newTypingScene(t)
	e := s.e
	robin := e.newPerson()
	readerNoFocus := e.newPerson()
	e.w.grant(readerNoFocus.ID, e.w.threads[s.th].ChannelID, levelRead)
	robinCl, quiet := e.join(robin), e.join(readerNoFocus)
	// Robin's focus is dropped by gateway.focus; force it into the index to prove admission too.
	e.hub.ForceFocus(robinCl.c, s.th)
	e.typing(s.alex, "typing.start", s.th)
	if n := len(s.sam.events(t)); n != 1 {
		t.Fatalf("Sam received %d events, want 1", n)
	}
	for name, cl := range map[string]*client{"robin": robinCl, "reader without focus": quiet, "alex second device": s.alexSecond, "alex": s.alex} {
		if n := len(cl.events(t)); n != 0 {
			t.Errorf("%s received %d events", name, n)
		}
	}
}

// TestTypingStarted_EnvelopeAndExpiry: the envelope has no seq and no permission_version, the
// actor is the typist, scope and resource name the thread, and expires_at = occurred_at +
// sync.typing_ttl in RFC 3339 UTC with milliseconds.
func TestTypingStarted_EnvelopeAndExpiry(t *testing.T) {
	t.Parallel()
	s := newTypingScene(t)
	e := s.e
	e.clk.Advance(1234567 * time.Microsecond)
	e.typing(s.alex, "typing.start", s.th)
	evs := s.sam.events(t)
	if len(evs) != 1 {
		t.Fatalf("%d events", len(evs))
	}
	ev := evs[0].D
	th := e.w.threads[s.th]
	if ev.Seq != nil || ev.PermissionVersion != nil {
		t.Errorf("seq %v, permission_version %v: an ephemeral event carries neither", ev.Seq, ev.PermissionVersion)
	}
	if ev.Type != "typing.started" || ev.SchemaVersion != 1 || ev.ID == "" {
		t.Errorf("type %q schema %d id %q", ev.Type, ev.SchemaVersion, ev.ID)
	}
	if ev.Actor == nil || *ev.Actor != (wireActor{ID: s.alex.a.ID.String(), Kind: "user"}) {
		t.Errorf("actor %+v", ev.Actor)
	}
	wantScope := map[string]any{"space_id": th.SpaceID.String(), "channel_id": th.ChannelID.String(), "thread_id": th.ID.String()}
	if !mapsEqual(ev.Scope, wantScope) || !mapsEqual(ev.Resource, map[string]any{"kind": "thread", "id": th.ID.String()}) {
		t.Errorf("scope %v resource %v", ev.Scope, ev.Resource)
	}
	var p struct {
		ThreadID    string `json:"thread_id"`
		PrincipalID string `json:"principal_id"`
		ExpiresAt   string `json:"expires_at"`
	}
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		t.Fatal(err)
	}
	if p.ThreadID != th.ID.String() || p.PrincipalID != s.alex.a.ID.String() {
		t.Errorf("payload %+v", p)
	}
	occurred, err1 := time.Parse(time.RFC3339Nano, ev.OccurredAt)
	expires, err2 := time.Parse(time.RFC3339Nano, p.ExpiresAt)
	if err1 != nil || err2 != nil {
		t.Fatalf("times %q %q: %v %v", ev.OccurredAt, p.ExpiresAt, err1, err2)
	}
	if expires.Sub(occurred) != typingTTL {
		t.Errorf("expires_at %s - occurred_at %s = %s, want %s", p.ExpiresAt, ev.OccurredAt, expires.Sub(occurred), typingTTL)
	}
	if want := "2026-10-16T09:00:01.234Z"; ev.OccurredAt != want || p.ExpiresAt != "2026-10-16T09:00:09.234Z" {
		t.Errorf("occurred_at %s expires_at %s, want UTC with milliseconds (%s)", ev.OccurredAt, p.ExpiresAt, want)
	}
}

// mapsEqual compares two decoded JSON objects of strings.
func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range b {
		if a[k] != v {
			return false
		}
	}
	return true
}

// TestFocus_PrunedOnAccessChanged: when access.changed removes Sam's channel, the focus on its
// thread ends before the event is dispatched; a still readable focused thread stays.
func TestFocus_PrunedOnAccessChanged(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	sam := e.newPerson()
	gone, kept := e.w.thread(e.support), e.w.thread(e.support)
	e.w.grant(sam.ID, gone.ChannelID, levelRead)
	e.w.grant(sam.ID, kept.ChannelID, levelRead)
	cl := e.join(sam)
	e.focus(cl, false, gone.ID, kept.ID)
	e.w.grant(sam.ID, gone.ChannelID, 0)
	ev := accessChanged(42, sam.ID, gone.ChannelID)
	e.hub.Dispatch(context.Background(), ev)
	if got := e.focused(cl); !sameSet(got, []ids.ID{kept.ID}) {
		t.Errorf("focus %v, want only %v", got, kept.ID)
	}
	// An ended focus stays ended: once every thread is pruned, a later access.changed that makes
	// the thread readable again does not bring it back from the session.
	e.w.grant(sam.ID, kept.ChannelID, 0)
	e.hub.Dispatch(context.Background(), accessChanged(43, sam.ID, kept.ChannelID))
	e.w.grant(sam.ID, gone.ChannelID, levelRead)
	e.hub.Dispatch(context.Background(), accessGranted(44, sam.ID, gone.ChannelID))
	if got := e.focused(cl); len(got) != 0 {
		t.Errorf("focus %v came back without a focus op", got)
	}
}

// TestFocus_RecheckedOnThreadMoved: thread.moved re-checks the connections that focus the
// thread: moved into a channel Sam cannot read, the focus ends; Robin, who reads the target
// channel, keeps it; a channel.archived of a still readable channel keeps the focus.
func TestFocus_RecheckedOnThreadMoved(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	sam, robin := e.newPerson(), e.newPerson()
	th := e.w.thread(e.support)
	e.w.grant(sam.ID, th.ChannelID, levelRead)
	e.w.grant(robin.ID, th.ChannelID, levelRead)
	samCl, robinCl := e.join(sam), e.join(robin)
	e.focus(samCl, false, th.ID)
	e.focus(robinCl, false, th.ID)
	moved := e.w.move(th.ID)
	e.w.grant(robin.ID, moved.ChannelID, levelRead)
	e.hub.Dispatch(context.Background(), sync.Event{Seq: 42, ID: ids.New(), Type: "thread.moved", SchemaVersion: 1, OccurredAt: t0,
		SpaceID: moved.SpaceID, ChannelID: moved.ChannelID, ThreadID: th.ID, ResourceKind: "thread", ResourceID: th.ID, ResourceVersion: 2,
		ReceiveAction: "thread.read", Payload: json.RawMessage(`{}`), PermissionVersion: 1})
	if got := e.focused(samCl); len(got) != 0 {
		t.Errorf("Sam focus %v after the move into an unreadable channel", got)
	}
	if got := e.focused(robinCl); !sameSet(got, []ids.ID{th.ID}) {
		t.Errorf("Robin focus %v, want the moved thread", got)
	}
	// The kept entry carries the new channel: an archive of it re-checks Robin again.
	e.w.grant(robin.ID, moved.ChannelID, 0)
	e.hub.Dispatch(context.Background(), sync.Event{Seq: 43, ID: ids.New(), Type: "channel.archived", SchemaVersion: 1, OccurredAt: t0,
		SpaceID: moved.SpaceID, ChannelID: moved.ChannelID, ResourceKind: "channel", ResourceID: moved.ChannelID, ResourceVersion: 3,
		ReceiveAction: "channel.read", Payload: json.RawMessage(`{}`), PermissionVersion: 1})
	if got := e.focused(robinCl); len(got) != 0 {
		t.Errorf("Robin focus %v after the archive re-check", got)
	}
}

// TestFocus_RestoredOnResume: resume restores the focus of the session, re-checked: a thread Sam
// can no longer read is dropped, the readable one stays, idle is kept.
func TestFocus_RestoredOnResume(t *testing.T) {
	t.Parallel()
	e := newFocusEnv(t)
	sam := e.newPerson()
	a, b := e.w.thread(e.support), e.w.thread(e.support)
	e.w.grant(sam.ID, a.ChannelID, levelRead)
	e.w.grant(sam.ID, b.ChannelID, levelRead)
	cl := e.join(sam)
	e.focus(cl, true, a.ID, b.ID)
	sid := sent[app.Ready](cl.fc)[0].SessionID
	cl.c.Close(domain.CloseGoingAway, "")
	e.gw.Closed(cl.c)
	e.w.grant(sam.ID, b.ChannelID, 0)
	fc := &fakeConn{}
	c, err := e.gw.Open(context.Background(), fc)
	if err != nil {
		t.Fatal(err)
	}
	e.send(c, "resume", map[string]any{"gateway_session_id": sid, "access_token": e.token(sam), "dpop": "proof", "cursor": 41})
	wantOpen(t, fc)
	if len(sent[app.Resumed](fc)) != 1 {
		t.Fatal("no resumed")
	}
	if got := e.hub.FocusOf(c); !sameSet(got, []ids.ID{a.ID}) {
		t.Errorf("focus after resume %v, want %v", got, a.ID)
	}
	if !e.gw.SessionOf(c).FocusState().Idle {
		t.Error("idle lost on resume")
	}
}
