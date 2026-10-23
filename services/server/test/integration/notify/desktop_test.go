// SPDX-License-Identifier: Apache-2.0

package notify_test

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
	notifyjobs "github.com/OWNER/bothy/services/server/internal/notify/adapters/jobs"
	notifyapp "github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// The tests of M3-23 (brief "Tests", Integration) over the world of M3-19 wired as role all
// (withLive): the presence port answers what the gateway's PresenceView would report for the live
// connections the test sets (the view itself is tested against a real hub in internal/gateway/app
// and end to end in internal/wire), the live routes insert their jobs through the real job
// runtime, and the publisher records notification.created as the hub would receive it.

// livePresence is the presence of the gateway as a test sets it.
type livePresence struct {
	mu    sync.Mutex
	conns map[ids.ID][]presence.Connection
}

// Connections implements presence.View.
func (p *livePresence) Connections(_ context.Context, users []ids.ID) (map[ids.ID][]presence.Connection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[ids.ID][]presence.Connection{}
	for _, u := range users {
		if c, ok := p.conns[u]; ok {
			out[u] = slices.Clone(c)
		}
	}
	return out, nil
}

// set makes device of user live with focus and idle; a nil focus is an empty focus.
func (p *livePresence) set(user, device ids.ID, idle bool, focus ...ids.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conns[user] = slices.DeleteFunc(p.conns[user], func(c presence.Connection) bool { return c.DeviceID == device })
	if focus == nil {
		focus = []ids.ID{}
	}
	p.conns[user] = append(p.conns[user], presence.Connection{DeviceID: device, Focus: focus, Idle: idle})
}

// recordPublisher records notification.created as the hub receives it.
type recordPublisher struct {
	mu     sync.Mutex
	events []eventlog.EphemeralEvent
}

// PublishEphemeral implements eventlog.EphemeralPublisher.
func (p *recordPublisher) PublishEphemeral(_ context.Context, ev eventlog.EphemeralEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, ev)
	return nil
}

// to returns the recorded events whose audience is user.
func (p *recordPublisher) to(user ids.ID) []eventlog.EphemeralEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []eventlog.EphemeralEvent
	for _, ev := range p.events {
		if ev.Audience == user {
			out = append(out, ev)
		}
	}
	return out
}

// desktop writes an active device of platform without a push registration for user.
func (w *world) desktop(user ids.ID, platform string) ids.ID {
	w.t.Helper()
	d := ids.New()
	w.sql(`INSERT INTO devices (id, user_id, name, platform, state) VALUES ($1, $2, 'Desktop', $3, 'active')`, d.String(), user.String(), platform)
	return d
}

// gatewayRow is one job notify.gateway_send with its decoded arguments.
type gatewayRow struct {
	jobRow
	Args notifyjobs.GatewaySend
}

// gatewayJobsFor returns the jobs notify.gateway_send of recipient.
func (w *world) gatewayJobsFor(recipient ids.ID) []gatewayRow {
	w.t.Helper()
	var out []gatewayRow
	for _, j := range w.jobsOf(notifyjobs.KindGatewaySend) {
		g := gatewayRow{jobRow: j}
		must(w.t, json.Unmarshal(j.Args, &g.Args))
		if g.Args.RecipientID == recipient {
			out = append(out, g)
		}
	}
	return out
}

// runGateway runs every pending notify.gateway_send through Push.GatewaySend and completes it.
func (w *world) runGateway() {
	w.t.Helper()
	for _, j := range w.jobsOf(notifyjobs.KindGatewaySend) {
		if !j.pending() {
			continue
		}
		var a notifyjobs.GatewaySend
		must(w.t, json.Unmarshal(j.Args, &a))
		kind, err := domain.KindFromCode(a.KindCode)
		must(w.t, err)
		must(w.t, w.push.Push.GatewaySend(context.Background(), notifyapp.GatewayJob{
			NotificationID: a.NotificationID, RecipientID: a.RecipientID, ThreadID: a.ThreadID, MessageID: a.MessageID, TargetID: a.TargetID,
			Kind: kind, Quiet: a.Quiet == 1, Count: a.Count, DueAt: time.UnixMicro(a.DueAtUnixMicro).UTC(),
		}))
		w.complete(j.ID)
	}
}

// hintPayload is the payload of a recorded notification.created.
type hintPayload struct {
	NID       ids.ID `json:"nid"`
	Kind      string `json:"kind"`
	ThreadID  ids.ID `json:"thread_id"`
	MessageID ids.ID `json:"message_id"`
	TargetID  ids.ID `json:"target_id"`
	Quiet     bool   `json:"quiet"`
	Count     int64  `json:"count"`
}

// payloadOf encodes the payload of ev, validates it against NotificationCreatedPayload and
// decodes it.
func (w *world) payloadOf(ev eventlog.EphemeralEvent) hintPayload {
	w.t.Helper()
	b, err := json.Marshal(ev.Payload)
	must(w.t, err)
	validateSchema(w.t, "NotificationCreatedPayload", b)
	var p hintPayload
	must(w.t, json.Unmarshal(b, &p))
	return p
}

// TestDesktopFocus_NoPushToPhone (PLAN row M3-23, 13 14 "Thread in focus on the desktop"): Alex's
// desktop is live and focuses T, not idle; the Owner writes in T: no notify.push_send, no
// notify.gateway_send and no window job for Alex, bothy.push.skipped active_in_thread rose by
// one, the provider saw no call. An idle desktop focusing T delays nothing: one gateway job and
// one immediate push for the phone, and the gateway job publishes to Alex. A desktop with an
// empty focus gets the hint over the gateway and the phone's push is delayed.
func TestDesktopFocus_NoPushToPhone(t *testing.T) {
	t.Parallel()
	t.Run("focus", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, withLive())
		th := w.newThread(w.allgemein)
		w.setLevel("alex", th, "all")
		desk := w.desktop(w.alex, "macos")
		w.live.set(w.alex, desk, false, th)
		before := w.skipped("active_in_thread")
		w.post("owner", th, "The export is back")
		w.consume()
		if jobs := w.pushJobsFor(w.alex); len(jobs) != 0 {
			t.Fatalf("push jobs for Alex %+v", jobs)
		}
		if jobs := w.gatewayJobsFor(w.alex); len(jobs) != 0 {
			t.Fatalf("gateway jobs for Alex %+v", jobs)
		}
		for _, kind := range []string{notifyjobs.KindCoalesceWindow, notifyjobs.KindCoalesceFlush} {
			if jobs := w.windowsOf(kind, w.alex, th); len(jobs) != 0 {
				t.Fatalf("%s for Alex %+v", kind, jobs)
			}
		}
		if got := w.skipped("active_in_thread") - before; got != 1 {
			t.Fatalf("active_in_thread rose by %d, want 1", got)
		}
		w.sendPending()
		if n := len(w.apns.Calls()); n != 0 {
			t.Fatalf("%d provider calls", n)
		}
		// The phone live and focusing T gives no hint either.
		w.live.set(w.alex, desk, true)
		w.live.set(w.alex, w.alexPhone.Device, false, th)
		w.post("owner", th, "And the import too")
		w.consume()
		if jobs := w.pushJobsFor(w.alex); len(jobs) != 0 {
			t.Fatalf("push jobs for Alex with the phone in focus %+v", jobs)
		}
	})
	t.Run("idle", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, withLive())
		th := w.newThread(w.allgemein)
		w.setLevel("alex", th, "all")
		desk := w.desktop(w.alex, "macos")
		w.live.set(w.alex, desk, true, th)
		w.post("owner", th, "The export is back")
		w.consume()
		phone := w.pushJobsFor(w.alex)
		if len(phone) != 1 || phone[0].ScheduledAt.Equal(w.clk.Now().Add(time.Minute)) || phone[0].Args.ReleaseSeq != 0 {
			t.Fatalf("phone jobs %+v, want one immediate push", phone)
		}
		if n := len(w.gatewayJobsFor(w.alex)); n != 1 {
			t.Fatalf("%d gateway jobs, want 1", n)
		}
		w.runGateway()
		if evs := w.published.to(w.alex); len(evs) != 1 || w.payloadOf(evs[0]).NID != phone[0].Args.NotificationID {
			t.Fatalf("published %+v", evs)
		}
	})
	t.Run("empty focus", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, withLive())
		th := w.newThread(w.allgemein)
		w.setLevel("alex", th, "all")
		desk := w.desktop(w.alex, "macos")
		w.live.set(w.alex, desk, false)
		w.post("owner", th, "The export is back")
		w.consume()
		phone := w.pushJobsFor(w.alex)
		if len(phone) != 1 || phone[0].Args.ReleaseSeq == 0 || phone[0].ScheduledAt.Before(w.clk.Now().Add(time.Minute-time.Second)) {
			t.Fatalf("phone jobs %+v, want one delayed push", phone)
		}
		if n := len(w.gatewayJobsFor(w.alex)); n != 1 {
			t.Fatalf("%d gateway jobs, want 1", n)
		}
	})
}

// TestDesktopActive_ReadBeforeDelay_NoPush (PLAN row M3-23, 13 14 "Desktop active, message read
// there before the delay expires"): the desktop is live with focus on T2; the Owner writes m in T:
// one notify.gateway_send and one notify.push_send at clock + notify.mobile_delay_when_desktop_active
// with ReleaseSeq = thread_seq of m; the gateway job publishes one notification.created with the
// push's nid, kind message, T, m, quiet false, count 1. Alex reads T up to m; after the delay the
// push ends without a provider call and read rose by one. Without the read, or with a read only
// up to an earlier message, exactly one provider call.
func TestDesktopActive_ReadBeforeDelay_NoPush(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		read  func(w *world, th ids.ID, earlier, m msg)
		calls int
	}{
		{"read on the desktop", func(w *world, th ids.ID, _, m msg) { w.readUpTo("alex", th, m.ThreadSeq) }, 0},
		{"no read", func(*world, ids.ID, msg, msg) {}, 1},
		{"read up to an earlier message", func(w *world, th ids.ID, earlier, _ msg) { w.readUpTo("alex", th, earlier.ThreadSeq) }, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t, withLive())
			th, t2 := w.newThread(w.allgemein), w.newThread(w.allgemein)
			earlier := w.post("owner", th, "Before the follow")
			w.consume()
			w.setLevel("alex", th, "all")
			desk := w.desktop(w.alex, "macos")
			w.live.set(w.alex, desk, false, t2)
			m := w.post("owner", th, "The export is back")
			w.consume()
			phone := w.pushJobsFor(w.alex)
			if len(phone) != 1 {
				t.Fatalf("phone jobs %+v, want 1", phone)
			}
			p := phone[0]
			if p.Args.ReleaseSeq != m.ThreadSeq || !p.ScheduledAt.Equal(w.clk.Now().Add(time.Minute)) ||
				p.Args.DueAtUnixMicro != w.clk.Now().Add(time.Minute).UnixMicro() || p.Args.RegistrationID != w.alexPhone.Registration {
				t.Fatalf("phone job %+v at %v, want ReleaseSeq %d at the end of the delay", p.Args, p.ScheduledAt, m.ThreadSeq)
			}
			gw := w.gatewayJobsFor(w.alex)
			if len(gw) != 1 {
				t.Fatalf("gateway jobs %+v, want 1", gw)
			}
			w.runGateway()
			evs := w.published.to(w.alex)
			if len(evs) != 1 {
				t.Fatalf("published %d events to Alex, want 1", len(evs))
			}
			ev, pl := evs[0], w.payloadOf(evs[0])
			if ev.Resource.Kind != "thread" || ev.Resource.ID != th || ev.Scope.ChannelID != w.allgemein || ev.Actor != nil ||
				pl.NID != p.Args.NotificationID || pl.Kind != "message" || pl.ThreadID != th || pl.MessageID != m.ID || pl.Quiet || pl.Count != 1 {
				t.Fatalf("notification.created %+v payload %+v", ev, pl)
			}
			c.read(w, th, earlier, m)
			before := w.skipped("read")
			w.advance(time.Minute)
			w.sendPending()
			if n := len(w.apns.Calls()); n != c.calls {
				t.Fatalf("%d provider calls, want %d", n, c.calls)
			}
			if c.calls == 0 {
				if got := w.skipped("read") - before; got != 1 {
					t.Fatalf("read rose by %d, want 1", got)
				}
			} else if n := w.apns.Calls()[0].Notification.ID; n != p.Args.NotificationID {
				t.Fatalf("pushed nid %v, want %v", n, p.Args.NotificationID)
			}
		})
	}
}

// TestDesktopHint_LaptopWithoutRegistration: only Alex's laptop (windows) is live with an empty
// focus and the phone is offline: the laptop's hint goes over the gateway and the phone's push is
// delayed; with no desktop live the phone's push is immediate and nothing is published.
func TestDesktopHint_LaptopWithoutRegistration(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withLive())
	th := w.newThread(w.allgemein)
	w.setLevel("alex", th, "all")
	laptop := w.desktop(w.alex, "windows")
	w.live.set(w.alex, laptop, false)
	w.post("owner", th, "The export is back")
	w.consume()
	if n := len(w.gatewayJobsFor(w.alex)); n != 1 {
		t.Fatalf("%d gateway jobs, want 1", n)
	}
	if phone := w.pushJobsFor(w.alex); len(phone) != 1 || phone[0].Args.ReleaseSeq == 0 {
		t.Fatalf("phone jobs %+v, want one delayed push", phone)
	}
	w.runGateway()
	w.live.mu.Lock()
	delete(w.live.conns, w.alex)
	w.live.mu.Unlock()
	w.advance(2 * time.Minute)
	th2 := w.newThread(w.allgemein)
	w.setLevel("alex", th2, "all")
	w.post("owner", th2, "Nobody at the desk")
	w.consume()
	var immediate int
	for _, j := range w.pushJobsFor(w.alex) {
		if j.Args.ThreadID == th2 && j.Args.ReleaseSeq == 0 {
			immediate++
		}
	}
	if immediate != 1 || len(w.gatewayJobsFor(w.alex)) != 1 || len(w.published.to(w.alex)) != 1 {
		t.Fatalf("immediate %d, gateway jobs %d, published %d", immediate, len(w.gatewayJobsFor(w.alex)), len(w.published.to(w.alex)))
	}
}

// TestGatewayHint_RevocationBeforePublish (13 14 "Revocation between event and sending" for the
// gateway route): Sam follows T with all and has a live desktop; the round inserts her
// notify.gateway_send; the Owner removes Sam from the group Leser; the job publishes nothing and
// counts no_access.
func TestGatewayHint_RevocationBeforePublish(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withLive())
	th := w.newThread(w.allgemein)
	w.setLevel("sam", th, "all")
	desk := w.desktop(w.sam, "macos")
	w.live.set(w.sam, desk, false)
	w.post("owner", th, "The export is back")
	w.consume()
	if n := len(w.gatewayJobsFor(w.sam)); n != 1 {
		t.Fatalf("%d gateway jobs for Sam, want 1", n)
	}
	w.removeMember(w.leser, w.sam)
	before := w.skipped("no_access")
	w.runGateway()
	if evs := w.published.to(w.sam); len(evs) != 0 {
		t.Fatalf("published to Sam after the revocation: %+v", evs)
	}
	if got := w.skipped("no_access") - before; got != 1 {
		t.Fatalf("no_access rose by %d, want 1", got)
	}
}

// TestGatewayHint_WithoutThread: an access.expiring to Alex gives Alex's live desktop one
// notification.created with kind admin, target_id the grant, no thread_id and resource user; no
// event goes to anyone else.
func TestGatewayHint_WithoutThread(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withLive())
	desk := w.desktop(w.alex, "macos")
	w.live.set(w.alex, desk, false)
	w.live.set(w.sam, w.desktop(w.sam, "macos"), false)
	w.expiring(w.alex, w.alex, w.g1, w.support, "trigger")
	w.consume()
	w.runGateway()
	evs := w.published.to(w.alex)
	if len(evs) != 1 || len(w.published.events) != 1 {
		t.Fatalf("published %+v", w.published.events)
	}
	ev, pl := evs[0], w.payloadOf(evs[0])
	if ev.Resource.Kind != "user" || ev.Resource.ID != w.alex || !ev.Scope.ThreadID.IsZero() || !ev.Scope.ChannelID.IsZero() ||
		pl.Kind != "admin" || pl.TargetID != w.g1 || !pl.ThreadID.IsZero() || !pl.MessageID.IsZero() {
		t.Fatalf("admin hint %+v payload %+v", ev, pl)
	}
}

// TestFlushHintOverGateway: ten messages of the Owner in T within one notify.coalesce_window
// while Alex's desktop is live with focus elsewhere: the first gives one gateway hint and one
// delayed phone push; the flush at the window end gives a second gateway hint with count equal to
// the unread count (all ten) and a second delayed phone push.
func TestFlushHintOverGateway(t *testing.T) {
	t.Parallel()
	w := newWorld(t, withLive())
	th, other := w.newThread(w.allgemein), w.newThread(w.allgemein)
	w.setLevel("alex", th, "all")
	desk := w.desktop(w.alex, "macos")
	w.live.set(w.alex, desk, false, other)
	for i := range 10 {
		w.post("owner", th, "Update "+string(rune('a'+i)))
		w.advance(time.Second)
	}
	w.consume()
	if g, p := len(w.gatewayJobsFor(w.alex)), len(w.pushJobsFor(w.alex)); g != 1 || p != 1 {
		t.Fatalf("after the burst %d gateway and %d push jobs, want 1 and 1", g, p)
	}
	w.advance(time.Minute)
	w.flushDue()
	gw := w.gatewayJobsFor(w.alex)
	if len(gw) != 2 || gw[1].Args.Count != 10 {
		t.Fatalf("gateway jobs %+v, want a flush hint with the unread count 10", gw)
	}
	phone := w.pushJobsFor(w.alex)
	if len(phone) != 2 || phone[1].Args.ReleaseSeq == 0 || phone[1].Args.NotificationID != gw[1].Args.NotificationID {
		t.Fatalf("phone jobs %+v, want a second delayed push of the flush hint", phone)
	}
}

// TestRoleWorker_PushOnly (design note 1): wired as role worker (presence.None and the fail-closed
// routes of M3-19), a desktop that is live on a separate api process is invisible: the phone gets
// an immediate push and no notify.gateway_send exists. The wiring and its Info line are tested in
// internal/wire (TestWirePresence_ByRole).
func TestRoleWorker_PushOnly(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	th := w.newThread(w.allgemein)
	w.setLevel("alex", th, "all")
	w.desktop(w.alex, "macos")
	w.post("owner", th, "The export is back")
	w.consume()
	if phone := w.pushJobsFor(w.alex); len(phone) != 1 || phone[0].Args.ReleaseSeq != 0 {
		t.Fatalf("phone jobs %+v, want one immediate push", phone)
	}
	if n := len(w.jobsOf(notifyjobs.KindGatewaySend)); n != 0 {
		t.Fatalf("%d gateway jobs in role worker", n)
	}
}
