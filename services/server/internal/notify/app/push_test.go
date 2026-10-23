// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	stdsync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/presence"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify/app"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

// kindsOf returns the hint kinds of person, sorted.
func kindsOf(h map[ids.ID][]domain.Kind, person ids.ID) []domain.Kind {
	out := slices.Clone(h[person])
	slices.Sort(out)
	return out
}

func wantKinds(t *testing.T, h map[ids.ID][]domain.Kind, person ids.ID, name string, want ...domain.Kind) {
	t.Helper()
	slices.Sort(want)
	if got := kindsOf(h, person); !slices.Equal(got, want) {
		t.Errorf("%s: hints %v, want %v", name, got, want)
	}
}

// TestClassify_MessageCreated: precedence dm, mention, message; the author is never a candidate;
// agents, apps and inactive people are dropped; silent, import, system, anchors and system lines
// give nothing; another event type gives nothing (13 2, R-PN-11).
func TestClassify_MessageCreated(t *testing.T) {
	t.Parallel()
	t.Run("mention and message", func(t *testing.T) {
		t.Parallel()
		w := newPushWorld(t)
		w.mustHandle(w.messageEvent(nextSeq(), msg{thread: w.thread, channel: w.channel, author: w.sam, seq: 3,
			mentions: []ids.ID{w.alex, w.sam, w.lotse}}))
		h := w.hints()
		wantKinds(t, h, w.alex, "alex", domain.KindMention)
		wantKinds(t, h, w.owner, "owner", domain.KindMessage)
		wantKinds(t, h, w.sam, "author")
		wantKinds(t, h, w.robin, "disabled")
		wantKinds(t, h, w.lotse, "agent")
		wantKinds(t, h, w.textbot, "app")
		for _, j := range w.jobsOf(w.alex) {
			if j.ThreadID != w.thread || j.MessageID.IsZero() || j.Count != 1 || j.Attempt != 1 {
				t.Errorf("alex job %+v", j)
			}
		}
	})
	t.Run("dm over mention", func(t *testing.T) {
		t.Parallel()
		w := newPushWorld(t)
		w.mustHandle(w.messageEvent(nextSeq(), msg{thread: w.dmThread, channel: w.dmChannel, author: w.sam, seq: 2,
			mentions: []ids.ID{w.alex}}))
		h := w.hints()
		wantKinds(t, h, w.alex, "alex", domain.KindDM)
		wantKinds(t, h, w.sam, "author")
		wantKinds(t, h, w.owner, "owner outside the dm")
	})
	for _, c := range []struct {
		name string
		m    func(w *pushWorld) msg
	}{
		{"silent", func(w *pushWorld) msg {
			return msg{thread: w.thread, channel: w.channel, author: w.sam, silent: true, mentions: []ids.ID{w.alex}}
		}},
		{"import", func(w *pushWorld) msg {
			return msg{thread: w.thread, channel: w.channel, author: w.sam, origin: "import", mentions: []ids.ID{w.alex}}
		}},
		{"system origin", func(w *pushWorld) msg {
			return msg{thread: w.thread, channel: w.channel, author: w.sam, origin: "system"}
		}},
		{"anchor", func(w *pushWorld) msg {
			return msg{thread: w.thread, channel: w.channel, author: w.lotse, authorKind: "agent", origin: "agent", kind: "run"}
		}},
		{"system line", func(w *pushWorld) msg {
			return msg{thread: w.thread, channel: w.channel, author: w.sam, kind: "system"}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := newPushWorld(t)
			w.mustHandle(w.messageEvent(nextSeq(), c.m(w)))
			if len(w.sched.sends) != 0 {
				t.Fatalf("%d jobs", len(w.sched.sends))
			}
		})
	}
	t.Run("message.updated", func(t *testing.T) {
		t.Parallel()
		w := newPushWorld(t)
		ev := w.messageEvent(nextSeq(), msg{thread: w.thread, channel: w.channel, author: w.sam, mentions: []ids.ID{w.alex}})
		ev.Type = "message.updated"
		w.mustHandle(ev)
		if len(w.sched.sends) != 0 || w.calls.get("Readers") != 0 {
			t.Fatalf("an edit gave %d jobs", len(w.sched.sends))
		}
	})
}

// runWorld prepares a run of Lotse triggered by Alex in the thread: the anchor at position 1,
// agent messages at the given positions with their message.created events before endSeq.
func runWorld(t *testing.T, positions ...int64) (*pushWorld, ids.ID, []ids.ID) {
	t.Helper()
	w := newPushWorld(t)
	run := ids.New()
	w.anchors = append(w.anchors, messaging.AnchorRef{MessageID: ids.New(), RunID: run, ThreadSeq: 1})
	var msgs []ids.ID
	for i, p := range positions {
		id := ids.New()
		w.agentMsg = append(w.agentMsg, agentMessage{id: id, thread: w.thread, agent: w.lotse, seq: p})
		w.remember(w.messageEvent(int64(10+i), msg{id: id, thread: w.thread, channel: w.channel, author: w.lotse,
			authorKind: "agent", origin: "agent", seq: p}))
		msgs = append(msgs, id)
	}
	return w, run, msgs
}

// TestClassify_RunStateChanged: each state of design note 4 b.
func TestClassify_RunStateChanged(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"queued", "running"} {
		w, run, _ := runWorld(t, 2)
		w.mustHandle(w.runEvent(50, run, w.thread, w.lotse, w.alex, state))
		if len(w.sched.sends) != 0 {
			t.Errorf("%s: %d jobs", state, len(w.sched.sends))
		}
	}
	cases := []struct {
		state            string
		alex, sam, owner []domain.Kind
	}{
		{"waiting_input", []domain.Kind{domain.KindInputNeeded}, nil, nil},
		{"completed", []domain.Kind{domain.KindRunDone}, []domain.Kind{domain.KindMessage}, []domain.Kind{domain.KindMessage}},
		{"failed", []domain.Kind{domain.KindRunFailed}, []domain.Kind{domain.KindMessage}, []domain.Kind{domain.KindMessage}},
		{"cancelled", []domain.Kind{domain.KindMessage}, []domain.Kind{domain.KindMessage}, []domain.Kind{domain.KindMessage}},
	}
	for _, c := range cases {
		w, run, msgs := runWorld(t, 2, 3, 4)
		w.mustHandle(w.runEvent(50, run, w.thread, w.lotse, w.alex, c.state))
		h := w.hints()
		wantKinds(t, h, w.alex, c.state+" alex", c.alex...)
		wantKinds(t, h, w.sam, c.state+" sam", c.sam...)
		wantKinds(t, h, w.owner, c.state+" owner", c.owner...)
		wantKinds(t, h, w.lotse, c.state+" agent")
		for _, s := range w.sched.sends {
			if s.job.MessageID != msgs[2] {
				t.Errorf("%s: job points at %v, want the last agent message", c.state, s.job.MessageID)
			}
		}
	}
	t.Run("no agent message", func(t *testing.T) {
		w, run, _ := runWorld(t)
		w.mustHandle(w.runEvent(50, run, w.thread, w.lotse, w.alex, "completed"))
		h := w.hints()
		wantKinds(t, h, w.alex, "alex", domain.KindRunDone)
		wantKinds(t, h, w.sam, "sam")
		for _, j := range w.jobsOf(w.alex) {
			if !j.MessageID.IsZero() || j.ThreadID != w.thread {
				t.Errorf("run_done without agent message: %+v", j)
			}
		}
	})
}

// TestClassify_AccessExpiring: an active human audience gets admin with the grant or group as
// target; an agent or an inactive human gets nothing.
func TestClassify_AccessExpiring(t *testing.T) {
	t.Parallel()
	w := newPushWorld(t)
	grant, group := ids.New(), ids.New()
	w.mustHandle(w.event(nextSeq(), "access.expiring", "principal", w.alex, w.alex,
		map[string]any{"kind": "grant", "grant_id": grant, "subject_id": w.alex, "expires_at": "2026-10-22T12:00:00.000Z"}))
	w.mustHandle(w.event(nextSeq(), "access.expiring", "principal", w.sam, w.sam,
		map[string]any{"kind": "group_membership", "group_id": group, "subject_id": w.sam, "expires_at": "2026-10-22T12:00:00.000Z"}))
	w.mustHandle(w.event(nextSeq(), "access.expiring", "principal", w.lotse, w.lotse,
		map[string]any{"kind": "grant", "grant_id": ids.New(), "subject_id": w.lotse, "expires_at": "2026-10-22T12:00:00.000Z"}))
	w.mustHandle(w.event(nextSeq(), "access.expiring", "principal", w.robin, w.robin,
		map[string]any{"kind": "grant", "grant_id": ids.New(), "subject_id": w.robin, "expires_at": "2026-10-22T12:00:00.000Z"}))
	h := w.hints()
	wantKinds(t, h, w.alex, "alex", domain.KindAdmin)
	wantKinds(t, h, w.sam, "sam", domain.KindAdmin)
	wantKinds(t, h, w.lotse, "agent")
	wantKinds(t, h, w.robin, "disabled")
	for _, j := range w.jobsOf(w.alex) {
		if j.TargetID != grant || !j.ThreadID.IsZero() || !j.MessageID.IsZero() {
			t.Errorf("alex admin %+v", j)
		}
	}
	for _, j := range w.jobsOf(w.sam) {
		if j.TargetID != group {
			t.Errorf("sam admin %+v", j)
		}
	}
}

// TestClassify_DevicePending: security to the active human with device_id as target and no
// thread; the pending device is never a route; an inactive human gets nothing.
func TestClassify_DevicePending(t *testing.T) {
	t.Parallel()
	w := newPushWorld(t)
	pending := ids.New()
	// Even if a source reported the pending device as a device of Alex, it gets no route.
	w.devices[w.alex] = append(w.devices[w.alex], identity.DeviceRef{ID: pending, Platform: "android"})
	pendingReg := ids.New()
	w.regs[w.alex] = append(w.regs[w.alex], w.regs[w.alex][0])
	w.regs[w.alex][1].ID, w.regs[w.alex][1].DeviceID = pendingReg, pending
	w.store.put(app.Registration{ID: pendingReg, UserID: w.alex, DeviceID: pending, Provider: "fcm"})
	w.mustHandle(w.event(nextSeq(), "device.pending", "device", pending, w.alex, map[string]any{"device_id": pending, "platform": "android"}))
	w.mustHandle(w.event(nextSeq(), "device.pending", "device", ids.New(), w.robin, map[string]any{"device_id": ids.New(), "platform": "ios"}))
	h := w.hints()
	wantKinds(t, h, w.alex, "alex", domain.KindSecurity)
	wantKinds(t, h, w.robin, "disabled")
	jobs := w.jobsOf(w.alex)
	if len(jobs) != 1 || jobs[0].RegistrationID != w.regs[w.alex][0].ID || jobs[0].TargetID != pending || !jobs[0].ThreadID.IsZero() {
		t.Fatalf("security jobs %+v", jobs)
	}
}

// TestAgentMessageDuringRun: the commit order decides, never the state at processing time.
func TestAgentMessageDuringRun(t *testing.T) {
	t.Parallel()
	agentMsg := func(w *pushWorld, seq int64, author ids.ID) {
		w.mustHandle(w.messageEvent(seq, msg{thread: w.thread, channel: w.channel, author: author, authorKind: "agent",
			origin: "agent", seq: 5}))
	}
	cases := []struct {
		name    string
		prepare func(w *pushWorld, run ids.ID)
		author  func(w *pushWorld) ids.ID
		silent  bool
	}{
		{"active before", func(w *pushWorld, run ids.ID) {
			w.remember(w.runEvent(10, run, w.thread, w.lotse, w.alex, "queued"))
			w.remember(w.runEvent(11, run, w.thread, w.lotse, w.alex, "running"))
		}, func(w *pushWorld) ids.ID { return w.lotse }, true},
		{"terminal before", func(w *pushWorld, run ids.ID) {
			w.remember(w.runEvent(10, run, w.thread, w.lotse, w.alex, "running"))
			w.remember(w.runEvent(11, run, w.thread, w.lotse, w.alex, "completed"))
		}, func(w *pushWorld) ids.ID { return w.lotse }, false},
		{"another agent", func(w *pushWorld, run ids.ID) {
			w.remember(w.runEvent(10, run, w.thread, w.lotse, w.alex, "running"))
		}, func(w *pushWorld) ids.ID { return w.textbot }, false},
		{"no anchor", nil, func(w *pushWorld) ids.ID { return w.lotse }, false},
		{"ended after the message", func(w *pushWorld, run ids.ID) {
			w.remember(w.runEvent(10, run, w.thread, w.lotse, w.alex, "running"))
			w.remember(w.runEvent(30, run, w.thread, w.lotse, w.alex, "completed"))
		}, func(w *pushWorld) ids.ID { return w.lotse }, true},
	}
	for _, c := range cases {
		w := newPushWorld(t)
		run := ids.New()
		if c.prepare != nil {
			w.anchors = append(w.anchors, messaging.AnchorRef{MessageID: ids.New(), RunID: run, ThreadSeq: 1})
			c.prepare(w, run)
		}
		agentMsg(w, 20, c.author(w))
		got := len(w.hints()[w.sam]) == 0
		if got != c.silent {
			t.Errorf("%s: suppressed %v, want %v", c.name, got, c.silent)
		}
	}
}

// TestLastAgentMessage: the newest agent message before the end event wins; a message created
// after the end is skipped; none gives no message ID.
func TestLastAgentMessage(t *testing.T) {
	t.Parallel()
	w, run, msgs := runWorld(t, 2, 3)
	late := ids.New()
	w.agentMsg = append(w.agentMsg, agentMessage{id: late, thread: w.thread, agent: w.lotse, seq: 4})
	w.remember(w.messageEvent(60, msg{id: late, thread: w.thread, channel: w.channel, author: w.lotse, authorKind: "agent", origin: "agent", seq: 4}))
	w.mustHandle(w.runEvent(50, run, w.thread, w.lotse, w.alex, "completed"))
	for _, j := range w.jobsOf(w.alex) {
		if j.MessageID != msgs[1] {
			t.Fatalf("run_done points at %v, want the second message", j.MessageID)
		}
	}
}

// TestGather_OneCallPerPortPerEvent: 50 readers give one call each to the batch ports; a stored
// switch "yes" counts as unset and logs one Warn line without person data.
func TestGather_OneCallPerPortPerEvent(t *testing.T) {
	t.Parallel()
	w := newPushWorld(t)
	var readers []ids.ID
	for range 50 {
		p := ids.New()
		w.principals[p] = identity.PrincipalView{ID: p, Kind: "user", Status: identity.StatusActive}
		w.addPhone(p, "apns")
		readers = append(readers, p)
	}
	w.readers[w.channel] = append(readers, w.sam)
	w.mustHandle(w.messageEvent(nextSeq(), msg{thread: w.thread, channel: w.channel, author: w.sam}))
	for _, port := range []string{"LevelFacts", "QuietRules", "ValuesFor", "ActiveForUsers", "Active", "Connections"} {
		if n := w.calls.get(port); n != 1 {
			t.Errorf("%s called %d times, want 1", port, n)
		}
	}
	if n := len(w.hints()); n != 50 {
		t.Errorf("%d people got a hint, want 50", n)
	}

	w2 := newPushWorld(t)
	w2.cfg.DefaultAdminEnabled = true
	w2.settings[w2.alex] = map[string]json.RawMessage{"notify.admin_enabled": json.RawMessage(`"yes"`)}
	w2.mustHandle(w2.event(nextSeq(), "access.expiring", "principal", w2.alex, w2.alex,
		map[string]any{"kind": "grant", "grant_id": ids.New(), "subject_id": w2.alex, "expires_at": "2026-10-22T12:00:00.000Z"}))
	wantKinds(t, w2.hints(), w2.alex, "invalid switch, default on", domain.KindAdmin)
	logged := w2.log.String()
	if strings.Count(logged, "not a boolean") != 1 || strings.Contains(logged, w2.alex.String()) {
		t.Errorf("log %q", logged)
	}
	w3 := newPushWorld(t)
	w3.cfg.DefaultAdminEnabled = false
	w3.settings[w3.alex] = map[string]json.RawMessage{"notify.admin_enabled": json.RawMessage(`"yes"`)}
	w3.mustHandle(w3.event(nextSeq(), "access.expiring", "principal", w3.alex, w3.alex,
		map[string]any{"kind": "grant", "grant_id": ids.New(), "subject_id": w3.alex, "expires_at": "2026-10-22T12:00:00.000Z"}))
	wantKinds(t, w3.hints(), w3.alex, "invalid switch, default off")
	if !slices.Equal(w3.skips, []string{"preference"}) {
		t.Errorf("skips %v", w3.skips)
	}
}

// TestLiveRoutes_DefaultFailsClosed: a connection reported by presence leads to a gateway route,
// which the default LiveRoutes refuses with an internal error; with presence.None no decision
// yields gateway or push_delayed.
func TestLiveRoutes_DefaultFailsClosed(t *testing.T) {
	t.Parallel()
	w := newPushWorld(t)
	w.liveRoute = app.FailClosedRoutes{}
	w.conns[w.alex] = []presence.Connection{{DeviceID: w.devices[w.alex][0].ID}}
	err := w.handle(w.messageEvent(nextSeq(), msg{thread: w.thread, channel: w.channel, author: w.sam, mentions: []ids.ID{w.alex}}))
	if errs.KindOf(err) != errs.KindInternal {
		t.Fatalf("gateway route with the default: %v", err)
	}

	n := newPushWorld(t)
	n.presence = presence.None{}
	for _, p := range []ids.ID{n.alex, n.sam, n.owner} {
		n.addDevice(p, "macos", "apns")
		n.addDevice(p, "android", "fcm")
	}
	run := ids.New()
	n.anchors = append(n.anchors, messaging.AnchorRef{MessageID: ids.New(), RunID: run, ThreadSeq: 1})
	events := []func() error{
		func() error {
			return n.handle(n.messageEvent(nextSeq(), msg{thread: n.thread, channel: n.channel, author: n.sam, mentions: []ids.ID{n.alex}}))
		},
		func() error {
			return n.handle(n.messageEvent(nextSeq(), msg{thread: n.dmThread, channel: n.dmChannel, author: n.sam}))
		},
		func() error { return n.handle(n.runEvent(nextSeq(), run, n.thread, n.lotse, n.alex, "waiting_input")) },
		func() error { return n.handle(n.runEvent(nextSeq(), run, n.thread, n.lotse, n.alex, "failed")) },
		func() error {
			return n.handle(n.event(nextSeq(), "device.pending", "device", ids.New(), n.alex, map[string]any{"device_id": ids.New(), "platform": "ios"}))
		},
		func() error {
			return n.handle(n.event(nextSeq(), "access.expiring", "principal", n.owner, n.owner,
				map[string]any{"kind": "grant", "grant_id": ids.New(), "subject_id": n.owner, "expires_at": "2026-10-22T12:00:00.000Z"}))
		},
	}
	for i, ev := range events {
		if err := ev(); err != nil {
			t.Fatalf("event %d with presence.None: %v", i, err)
		}
	}
	if len(n.routes) != 0 {
		t.Fatalf("presence.None gave live routes %+v", n.routes)
	}
	if len(n.sched.sends) == 0 {
		t.Fatal("no push at all")
	}
}

// sendJob returns a push job of Alex's phone for the thread, due now.
func sendJob(w *pushWorld) app.PushJob {
	return app.PushJob{NotificationID: ids.New(), RegistrationID: w.regs[w.alex][0].ID, RecipientID: w.alex, ThreadID: w.thread,
		MessageID: ids.New(), Kind: domain.KindMessage, Count: 1, DueAt: w.clock.Now(), Attempt: 1}
}

// TestSend_Steps: the checks of step 1 end the job with the right skip reason and no provider
// call; the provider never runs inside a transaction; the semaphore bounds the parallel calls.
func TestSend_Steps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		prepare func(w *pushWorld, job *app.PushJob)
		skip    string
	}{
		{"disabled", func(w *pushWorld, job *app.PushJob) {
			_ = w.store.Update(context.Background(), job.RegistrationID, domain.RegistrationState{DisabledAt: pushStart, DisabledReason: "logout"})
		}, "no_registration"},
		{"device revoked", func(w *pushWorld, _ *app.PushJob) { w.devices[w.alex] = nil }, "no_registration"},
		{"unknown registration", func(_ *pushWorld, job *app.PushJob) { job.RegistrationID = ids.New() }, "no_registration"},
		{"too old", func(w *pushWorld, _ *app.PushJob) { w.clock.Advance(w.cfg.MaxAge + time.Second) }, "expired"},
		{"denied", func(w *pushWorld, job *app.PushJob) { w.denied[[2]ids.ID{w.alex, job.ThreadID}] = true }, "no_access"},
		{"disabled person", func(w *pushWorld, _ *app.PushJob) {
			v := w.principals[w.alex]
			v.Status = identity.StatusDisabled
			w.principals[w.alex] = v
		}, "no_access"},
		{"no provider", func(w *pushWorld, _ *app.PushJob) { w.providers = nil }, "no_provider"},
	}
	for _, c := range cases {
		w := newPushWorld(t)
		prov := &testProvider{name: "apns", result: domain.SendResult{Class: domain.ResultAccepted}}
		w.providers = []app.PushProvider{prov}
		job := sendJob(w)
		c.prepare(w, &job)
		if err := w.push().Send(context.Background(), job); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(prov.calls) != 0 || !slices.Equal(w.skips, []string{c.skip}) {
			t.Errorf("%s: calls %d, skips %v, want %s", c.name, len(prov.calls), w.skips, c.skip)
		}
	}

	w := newPushWorld(t)
	prov := &testProvider{name: "apns", result: domain.SendResult{Class: domain.ResultAccepted}}
	w.providers = []app.PushProvider{prov}
	job := sendJob(w)
	if err := w.push().Send(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if len(prov.calls) != 1 || prov.inTx {
		t.Fatalf("calls %d, inside a transaction %v", len(prov.calls), prov.inTx)
	}
	n := prov.calls[0]
	if n.ID != job.NotificationID || n.ThreadID != job.ThreadID || n.MessageID != job.MessageID || n.Kind != "message" || !n.HasBadge {
		t.Fatalf("notification %+v", n)
	}
	if _, st, _ := w.store.get(job.RegistrationID); st.LastSuccessAt.IsZero() || !slices.Equal(w.sent, []string{"apns/accepted"}) {
		t.Fatalf("state %+v, sent %v", st, w.sent)
	}

	t.Run("semaphore", func(t *testing.T) {
		w := newPushWorld(t)
		w.cfg.MaxConcurrentRequests = 3
		block := make(chan struct{})
		prov := &testProvider{name: "apns", result: domain.SendResult{Class: domain.ResultAccepted}, block: block}
		w.providers = []app.PushProvider{prov}
		p := w.push()
		job := sendJob(w)
		var wg stdsync.WaitGroup
		for range 100 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = p.Send(context.Background(), job)
			}()
		}
		testkit.Until(t, "three sends in flight", func() bool {
			prov.mu.Lock()
			defer prov.mu.Unlock()
			return prov.running == 3
		})
		close(block)
		wg.Wait()
		if prov.maxRun != 3 || len(prov.calls) != 100 {
			t.Fatalf("max parallel %d, calls %d", prov.maxRun, len(prov.calls))
		}
	})
}

// TestSend_Retry: a transient failure raises the count and inserts the next attempt at the
// retry delay; at the limit the registration is deactivated without retry; a retry past
// push.max_age is dropped as expired.
func TestSend_Retry(t *testing.T) {
	t.Parallel()
	w := newPushWorld(t)
	prov := &testProvider{name: "apns", result: domain.SendResult{Class: domain.ResultRetry, RetryAfter: time.Minute}}
	w.providers = []app.PushProvider{prov}
	p := w.push()
	job := sendJob(w)
	if err := p.Send(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if len(w.sched.sends) != 1 || w.sched.sends[0].job.Attempt != 2 || !w.sched.sends[0].at.Equal(pushStart.Add(time.Minute)) {
		t.Fatalf("retry %+v", w.sched.sends)
	}
	if _, st, _ := w.store.get(job.RegistrationID); st.FailureCount != 1 {
		t.Fatalf("failure count %d", st.FailureCount)
	}
	prov.err = context.DeadlineExceeded
	for range 2 {
		if err := p.Send(context.Background(), job); err != nil {
			t.Fatal(err)
		}
	}
	if _, st, _ := w.store.get(job.RegistrationID); st.DisabledReason != domain.DisableRepeatedFailures {
		t.Fatalf("state %+v", st)
	}
	if len(w.sched.sends) != 2 {
		t.Fatalf("deactivated registration retried: %d jobs", len(w.sched.sends))
	}

	e := newPushWorld(t)
	eprov := &testProvider{name: "apns", result: domain.SendResult{Class: domain.ResultThrottled, RetryAfter: 2 * time.Hour}}
	e.providers = []app.PushProvider{eprov}
	if err := e.push().Send(context.Background(), sendJob(e)); err != nil {
		t.Fatal(err)
	}
	if len(e.sched.sends) != 0 || !slices.Equal(e.skips, []string{"expired"}) {
		t.Fatalf("retry past max age: jobs %d, skips %v", len(e.sched.sends), e.skips)
	}
}
