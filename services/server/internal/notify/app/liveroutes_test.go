// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// liveWorld returns a push world with the live routes of M3-23 over its fake scheduler and a
// recording publisher; Alex has a desktop (macos, no registration) beside the phone.
func liveWorld(t *testing.T) (*pushWorld, ids.ID) {
	t.Helper()
	w := newPushWorld(t)
	w.liveRoute = app.NewLiveRoutes(w.sched)
	w.publisher = &fakePublisher{}
	desktop := ids.New()
	w.devices[w.alex] = append(w.devices[w.alex], identity.DeviceRef{ID: desktop, Platform: "macos"})
	return w, desktop
}

// TestLiveRoutes_GatewayOneJobPerHint: two gateway routes of one hint (desktop and laptop) insert
// one GatewaySend; another hint inserts a second one; the job holds the hint's pointers.
func TestLiveRoutes_GatewayOneJobPerHint(t *testing.T) {
	t.Parallel()
	w, desktop := liveWorld(t)
	laptop := ids.New()
	w.devices[w.alex] = append(w.devices[w.alex], identity.DeviceRef{ID: laptop, Platform: "windows"})
	w.conns[w.alex] = []presence.Connection{{DeviceID: desktop, Focus: []ids.ID{}}, {DeviceID: laptop, Focus: []ids.ID{}}}
	first := ids.New()
	w.mustHandle(w.messageEvent(nextSeq(), msg{id: first, thread: w.thread, channel: w.channel, author: w.sam, seq: 4, mentions: []ids.ID{w.alex}}))
	if len(w.sched.gateways) != 1 {
		t.Fatalf("%d gateway jobs for one hint with two connections", len(w.sched.gateways))
	}
	g := w.sched.gateways[0]
	if g.RecipientID != w.alex || g.ThreadID != w.thread || g.MessageID != first || g.Kind != domain.KindMention || g.Count != 1 || g.NotificationID.IsZero() {
		t.Fatalf("gateway job %+v", g)
	}
	w.mustHandle(w.messageEvent(nextSeq(), msg{thread: w.thread, channel: w.channel, author: w.sam, seq: 5, mentions: []ids.ID{w.alex}}))
	if len(w.sched.gateways) != 2 || w.sched.gateways[1].NotificationID == g.NotificationID {
		t.Fatalf("gateway jobs %+v, want a second hint", w.sched.gateways)
	}
}

// TestLiveRoutes_DelayedOneJobPerRegistration: a push_delayed route with two registrations
// inserts two PushSend at NotBefore, with DueAt = NotBefore, ReleaseSeq = MessageSeq, the hint's
// nid and quiet flag.
func TestLiveRoutes_DelayedOneJobPerRegistration(t *testing.T) {
	t.Parallel()
	w := newPushWorld(t)
	w.liveRoute = app.NewLiveRoutes(w.sched)
	regs := []ids.ID{ids.New(), ids.New()}
	notBefore := w.clock.Now().Add(w.cfg.MobileDelay)
	h := app.Hint{NotificationID: ids.New(), RecipientID: w.alex, ThreadID: w.thread, MessageID: ids.New(), Kind: domain.KindMessage,
		Quiet: true, Count: 1, DueAt: w.clock.Now(), MessageSeq: 9}
	err := pushUoW{}.Do(context.Background(), func(ctx context.Context) error {
		return w.liveRoute.Delayed(ctx, h, domain.Route{DeviceID: ids.New(), Via: domain.ViaPushDelayed, Registrations: regs, NotBefore: notBefore})
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(w.sched.sends) != 2 {
		t.Fatalf("%d push jobs, want 2", len(w.sched.sends))
	}
	for i, s := range w.sched.sends {
		j := s.job
		if !s.at.Equal(notBefore) || !j.DueAt.Equal(notBefore) || j.ReleaseSeq != 9 || j.NotificationID != h.NotificationID || !j.Quiet ||
			j.RegistrationID != regs[i] || j.MessageID != h.MessageID || j.Count != 1 || j.Attempt != 1 {
			t.Errorf("job %d %+v at %v", i, j, s.at)
		}
	}
}

// gatewayJob returns a gateway job of Alex for the thread, due now.
func gatewayJob(w *pushWorld) app.GatewayJob {
	return app.GatewayJob{NotificationID: ids.New(), RecipientID: w.alex, ThreadID: w.thread, MessageID: ids.New(), Kind: domain.KindMessage,
		Count: 1, DueAt: w.clock.Now()}
}

// TestGatewaySend_Steps: too old gives expired; an inactive recipient and a denied push.deliver
// give no_access; admin and security use the recipient rule and never call push.deliver; no
// publisher gives no_registration with one Warn line; nothing is published in any skip; the
// publish runs after the unit of work; each skip counts its reason.
func TestGatewaySend_Steps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		prepare func(w *pushWorld, job *app.GatewayJob)
		skip    string
	}{
		{"too old", func(w *pushWorld, _ *app.GatewayJob) { w.clock.Advance(w.cfg.MaxAge + time.Second) }, "expired"},
		{"disabled person", func(w *pushWorld, _ *app.GatewayJob) {
			v := w.principals[w.alex]
			v.Status = identity.StatusDisabled
			w.principals[w.alex] = v
		}, "no_access"},
		{"denied", func(w *pushWorld, job *app.GatewayJob) { w.denied[[2]ids.ID{w.alex, job.ThreadID}] = true }, "no_access"},
		{"thread gone", func(_ *pushWorld, job *app.GatewayJob) { job.ThreadID = ids.New() }, "no_access"},
		{"no publisher", func(w *pushWorld, _ *app.GatewayJob) { w.publisher = nil }, "no_registration"},
	}
	for _, c := range cases {
		w, _ := liveWorld(t)
		job := gatewayJob(w)
		c.prepare(w, &job)
		pub := w.publisher
		p := w.push()
		if err := p.GatewaySend(context.Background(), job); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !slices.Equal(w.skips, []string{c.skip}) {
			t.Errorf("%s: skips %v, want %s", c.name, w.skips, c.skip)
		}
		if pub != nil && len(pub.events) != 0 {
			t.Errorf("%s: published %d events", c.name, len(pub.events))
		}
		if c.skip == "no_registration" {
			if err := p.GatewaySend(context.Background(), job); err != nil {
				t.Fatal(err)
			}
			logged := w.log.String()
			if strings.Count(logged, "gateway hint without a gateway in this process") != 1 || strings.Contains(logged, w.alex.String()) {
				t.Errorf("log %q, want one Warn line without person data", logged)
			}
		}
	}
	for _, kind := range []domain.Kind{domain.KindAdmin, domain.KindSecurity} {
		w, _ := liveWorld(t)
		job := app.GatewayJob{NotificationID: ids.New(), RecipientID: w.alex, TargetID: ids.New(), Kind: kind, Count: 1, DueAt: w.clock.Now()}
		if err := w.push().GatewaySend(context.Background(), job); err != nil {
			t.Fatal(err)
		}
		if n := w.calls.get("Authorize"); n != 0 || len(w.publisher.events) != 1 {
			t.Errorf("%s: %d authorize calls, %d events", kind, n, len(w.publisher.events))
		}
		v := w.principals[w.alex]
		v.Status = identity.StatusDisabled
		w.principals[w.alex] = v
		if err := w.push().GatewaySend(context.Background(), job); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(w.skips, []string{"no_access"}) || len(w.publisher.events) != 1 {
			t.Errorf("%s of a disabled person: skips %v", kind, w.skips)
		}
	}
	w, _ := liveWorld(t)
	if err := w.push().GatewaySend(context.Background(), gatewayJob(w)); err != nil {
		t.Fatal(err)
	}
	if len(w.publisher.events) != 1 || w.publisher.inTx || len(w.skips) != 0 {
		t.Fatalf("events %d, inside a unit of work %v, skips %v", len(w.publisher.events), w.publisher.inTx, w.skips)
	}
	w.publisher.failing = errors.New("unregistered type")
	if err := w.push().GatewaySend(context.Background(), gatewayJob(w)); err == nil {
		t.Fatal("a publish error was swallowed")
	}
}

// strictPayload decodes b into the generated NotificationCreatedPayload and refuses unknown
// members.
func strictPayload(t *testing.T, b []byte) rest.NotificationCreatedPayload {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var p rest.NotificationCreatedPayload
	if err := dec.Decode(&p); err != nil {
		t.Fatalf("payload %s: %v", b, err)
	}
	return p
}

// TestGatewaySend_Envelope: a thread hint has resource thread and the scope of the current
// thread place (a moved thread gives the new channel), audience Alex and no actor; an admin hint
// has resource user with Alex's ID, an empty scope and target_id; the payload decodes strictly
// into the generated schema, has only the members of design note 7 and no null.
func TestGatewaySend_Envelope(t *testing.T) {
	t.Parallel()
	w, _ := liveWorld(t)
	moved := ids.New()
	info := w.threads[w.thread]
	info.ChannelID = moved
	w.threads[w.thread] = info
	job := gatewayJob(w)
	job.Quiet = true
	if err := w.push().GatewaySend(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	ev := w.publisher.events[0]
	if ev.Type != "notification.created" || ev.SchemaVersion != 1 || ev.Actor != nil || ev.Audience != w.alex ||
		ev.Resource.Kind != "thread" || ev.Resource.ID != w.thread || ev.Scope.ChannelID != moved || ev.Scope.SpaceID != w.space ||
		ev.Scope.ThreadID != w.thread {
		t.Fatalf("thread hint envelope %+v", ev)
	}
	b, err := json.Marshal(ev.Payload)
	if err != nil {
		t.Fatal(err)
	}
	p := strictPayload(t, b)
	if p.Nid.String() != job.NotificationID.String() || p.Kind != "message" || p.ThreadId == nil || p.ThreadId.String() != w.thread.String() ||
		p.MessageId == nil || p.MessageId.String() != job.MessageID.String() || !p.Quiet || p.Count != 1 || p.TargetId != nil {
		t.Fatalf("thread hint payload %s", b)
	}
	if testkit.HasJSONNull(b) {
		t.Fatalf("payload with null: %s", b)
	}

	grant := ids.New()
	admin := app.GatewayJob{NotificationID: ids.New(), RecipientID: w.alex, TargetID: grant, Kind: domain.KindAdmin, Count: 1, DueAt: w.clock.Now()}
	if err := w.push().GatewaySend(context.Background(), admin); err != nil {
		t.Fatal(err)
	}
	ev = w.publisher.events[1]
	if ev.Resource.Kind != "user" || ev.Resource.ID != w.alex || ev.Scope != (struct{ SpaceID, ChannelID, ThreadID ids.ID }{}) || ev.Audience != w.alex {
		t.Fatalf("admin hint envelope %+v", ev)
	}
	b, err = json.Marshal(ev.Payload)
	if err != nil {
		t.Fatal(err)
	}
	p = strictPayload(t, b)
	if p.Kind != "admin" || p.TargetId == nil || p.TargetId.String() != grant.String() || p.ThreadId != nil || p.MessageId != nil {
		t.Fatalf("admin hint payload %s", b)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(b, &members); err != nil {
		t.Fatal(err)
	}
	for k := range members {
		if !slices.Contains([]string{"nid", "kind", "thread_id", "message_id", "target_id", "quiet", "count"}, k) {
			t.Errorf("payload member %q outside design note 7", k)
		}
	}
}

// TestPushSend_ReleaseCheck: ReleaseSeq 5 with read position 4 sends; last_read_seq 5 skips
// read; last_read_seq 2 with unread_floor_seq 6 skips read; ReleaseSeq 0 sends without reading the
// read position; a delayed push whose recipient lost access gives no_access, not read.
func TestPushSend_ReleaseCheck(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		release, read int64
		floor         int64
		denied        bool
		want          string // "" sends
	}{
		{"unread", 5, 4, 0, false, ""},
		{"read", 5, 5, 0, false, "read"},
		{"floor", 5, 2, 6, false, "read"},
		{"immediate", 0, 9, 0, false, ""},
		{"lost access", 5, 5, 0, true, "no_access"},
	}
	for _, c := range cases {
		w := newPushWorld(t)
		prov := &testProvider{name: "apns", result: domain.SendResult{Class: domain.ResultAccepted}}
		w.providers = []app.PushProvider{prov}
		w.positions[[2]ids.ID{w.alex, w.thread}] = c.read
		info := w.threads[w.thread]
		info.UnreadFloorSeq = c.floor
		w.threads[w.thread] = info
		job := sendJob(w)
		job.ReleaseSeq = c.release
		if c.denied {
			w.denied[[2]ids.ID{w.alex, w.thread}] = true
		}
		positions := &countingPositions{pushPositions: pushPositions{w}}
		p := w.pushWith(func(d *app.PushDeps) { d.Positions = positions })
		if err := p.Send(context.Background(), job); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		sent := len(prov.calls) == 1
		switch {
		case c.want == "" && (!sent || len(w.skips) != 0):
			t.Errorf("%s: calls %d, skips %v, want a send", c.name, len(prov.calls), w.skips)
		case c.want != "" && (sent || !slices.Equal(w.skips, []string{c.want})):
			t.Errorf("%s: calls %d, skips %v, want %s", c.name, len(prov.calls), w.skips, c.want)
		}
		if c.release == 0 && positions.n != 0 {
			t.Errorf("%s: an immediate push read the read position", c.name)
		}
	}
}

// countingPositions counts the reads of PositionsFor.
type countingPositions struct {
	pushPositions
	n int
}

func (c *countingPositions) PositionsFor(ctx context.Context, thread ids.ID, users []ids.ID) (map[ids.ID]int64, error) {
	c.n++
	return c.pushPositions.PositionsFor(ctx, thread, users)
}

// TestHintMessageSeq: a message.created hint carries the payload's thread_seq, a run_done hint
// the thread_seq of the last agent message, an admin hint zero; a delayed push carries it as
// ReleaseSeq.
func TestHintMessageSeq(t *testing.T) {
	t.Parallel()
	w, desktop := liveWorld(t)
	w.conns[w.alex] = []presence.Connection{{DeviceID: desktop, Focus: []ids.ID{}}}
	w.mustHandle(w.messageEvent(nextSeq(), msg{thread: w.thread, channel: w.channel, author: w.sam, seq: 7, mentions: []ids.ID{w.alex}}))
	phone := w.jobsOf(w.alex)
	if len(phone) != 1 || phone[0].ReleaseSeq != 7 || len(w.sched.gateways) != 1 {
		t.Fatalf("message hint: phone jobs %+v, gateway jobs %d", phone, len(w.sched.gateways))
	}

	r, run, _ := runWorld(t, 2, 3)
	r.liveRoute = app.NewLiveRoutes(r.sched)
	desk := ids.New()
	r.devices[r.alex] = append(r.devices[r.alex], identity.DeviceRef{ID: desk, Platform: "windows"})
	r.conns[r.alex] = []presence.Connection{{DeviceID: desk, Focus: []ids.ID{}}}
	r.mustHandle(r.runEvent(50, run, r.thread, r.lotse, r.alex, "completed"))
	var seq int64 = -1
	for _, j := range r.jobsOf(r.alex) {
		if j.Kind == domain.KindRunDone {
			seq = j.ReleaseSeq
		}
	}
	if seq != 3 {
		t.Fatalf("run_done release seq %d, want 3", seq)
	}

	a, adesk := liveWorld(t)
	a.conns[a.alex] = []presence.Connection{{DeviceID: adesk, Focus: []ids.ID{}}}
	a.mustHandle(a.event(nextSeq(), "access.expiring", "principal", a.alex, a.alex,
		map[string]any{"kind": "grant", "grant_id": ids.New(), "subject_id": a.alex, "expires_at": "2026-10-22T12:00:00.000Z"}))
	for _, j := range a.jobsOf(a.alex) {
		if j.ReleaseSeq != 0 {
			t.Fatalf("admin hint release seq %d", j.ReleaseSeq)
		}
	}
	if len(a.sched.gateways) != 1 || a.sched.gateways[0].Kind != domain.KindAdmin {
		t.Fatalf("admin gateway jobs %+v", a.sched.gateways)
	}
}
