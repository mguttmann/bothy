// SPDX-License-Identifier: Apache-2.0

package terminal_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	"github.com/OWNER/bothy/services/server/internal/terminal/domain"
	"github.com/OWNER/bothy/services/server/internal/wire"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// revCase is one kind of revocation of 24 section 8.3 (a row of TM-T10): run builds the sessions of
// the row, applies the cause and returns the session that must end and the sessions the cause does
// not concern.
type revCase struct {
	name   string
	reason domain.CloseReason
	// action is the audit action of the cause whose correlation terminal.terminate carries; event
	// names an event of the host instead (a cause without its own audit entry); both empty: the
	// cause is the sweep itself, whose only entry is terminal.terminate.
	action, event string
	// job marks a cause committed by a job without a request (no correlation): the cause's entry
	// and terminal.terminate are proven to share their transaction instead.
	job bool
	// control is the close code of the host's control connection for a cause that changes the
	// state of the host (24 section 3.5: the host service ends all PTYs on it); 0 for the others,
	// which require session.close.
	control int
	run     func(w *world) (ended live, keep []live)
}

// revCases are the rows of TestTMT10_RevocationKinds (M2-46 Tests, Integration; Design notes 4 to 6).
func revCases() []revCase {
	return []revCase{
		{name: "grant_revoked", reason: domain.CloseAccessRevoked, action: "grant.revoke", run: func(w *world) (live, []live) {
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			w.must(w.robin, http.MethodPost, "/grants/"+w.alexGrant+"/revoke", revBody(), http.StatusOK)
			return l, []live{keep}
		}},
		{name: "grant_expired", reason: domain.CloseAccessRevoked, action: "grant.expire", job: true, run: func(w *world) (live, []live) {
			w.grant("user", w.alex.PrincipalID, w.r1.String(), stamp(w.clk.Now().Add(revExpiry)))
			w.must(w.robin, http.MethodPost, "/grants/"+w.alexGrant+"/revoke", revBody(), http.StatusOK)
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			w.advance(revExpiry + time.Second)
			w.expirySweep()
			return l, []live{keep}
		}},
		{name: "group_member_removed", reason: domain.CloseAccessRevoked, action: "group.member.remove", run: func(w *world) (live, []live) {
			l, keep := w.start(w.sam, w.r2, w.h2), w.start(w.robin, w.r2, w.h2)
			w.must(w.robin, http.MethodDelete, "/groups/"+w.engineering+"/members/"+w.sam.PrincipalID, nil, http.StatusNoContent)
			return l, []live{keep}
		}},
		{name: "group_membership_expired", reason: domain.CloseAccessRevoked, action: "group.member.expire", job: true,
			run: func(w *world) (live, []live) {
				ops := w.group("Operations", w.sam.PrincipalID, stamp(w.clk.Now().Add(revExpiry)))
				w.grant("group", ops, w.r2.String(), "")
				w.must(w.robin, http.MethodDelete, "/groups/"+w.engineering+"/members/"+w.sam.PrincipalID, nil, http.StatusNoContent)
				l, keep := w.start(w.sam, w.r2, w.h2), w.start(w.robin, w.r2, w.h2)
				w.advance(revExpiry + time.Second)
				w.expirySweep()
				return l, []live{keep}
			}},
		{name: "owner_removed_SR10", reason: domain.CloseAccessRevoked, action: "owner.remove", run: func(w *world) (live, []live) {
			w.revAddOwner(w.sam)
			l, keep := w.start(w.sam, w.r1, w.h1), w.start(w.sam, w.r2, w.h2)
			w.must(w.robin, http.MethodPost, "/owners/"+w.sam.PrincipalID+"/revoke", revBody(), http.StatusNoContent)
			return l, []live{keep}
		}},
		{name: "user_disabled", reason: domain.CloseUserDisabled, action: "user.disable", run: func(w *world) (live, []live) {
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			w.must(w.robin, http.MethodPost, "/users/"+w.alex.PrincipalID+"/disable", revBody(), http.StatusOK)
			return l, []live{keep}
		}},
		{name: "device_revoked", reason: domain.CloseDeviceRevoked, action: "device.revoke", run: func(w *world) (live, []live) {
			l, other, keep := w.start(w.alex, w.r1, w.h1), w.start(w.alex2, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			w.must(w.robin, http.MethodPost, "/devices/"+w.alex.DeviceID+"/revoke", revBody(), http.StatusOK)
			return l, []live{other, keep}
		}},
		{name: "session_revoked", reason: domain.CloseSessionRevoked, action: "session.revoke", run: func(w *world) (live, []live) {
			l, other, keep := w.start(w.alex, w.r1, w.h1), w.start(w.alex2, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			w.must(w.robin, http.MethodPost, "/sessions/"+w.alex.SessionID+"/revoke", nil, http.StatusNoContent)
			return l, []live{other, keep}
		}},
		{name: "session_expired", reason: domain.CloseSessionRevoked, run: func(w *world) (live, []live) {
			l, other, keep := w.start(w.alex, w.r1, w.h1), w.start(w.alex2, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			w.exec(`UPDATE auth_sessions SET expires_at = $2 WHERE id = $1`, w.alex.SessionID, w.clk.Now())
			w.sweep()
			return l, []live{other, keep}
		}},
		{name: "logout", reason: domain.CloseLogout, action: "auth.logout", run: func(w *world) (live, []live) {
			l, other, keep := w.start(w.alex, w.r1, w.h1), w.start(w.alex2, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			w.must(w.alex, http.MethodPost, "/auth/logout", nil, http.StatusNoContent)
			return l, []live{other, keep}
		}},
		{name: "host_disabled", reason: domain.CloseHostDisabled, action: "host.manage", control: 4111, run: func(w *world) (live, []live) {
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r2, w.h2)
			w.revSetHostState(w.r1, "disabled")
			return l, []live{keep}
		}},
		{name: "host_retired", reason: domain.CloseHostRetired, action: "host.manage", control: 4111, run: func(w *world) (live, []live) {
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r2, w.h2)
			w.revSetHostState(w.r1, "retired")
			return l, []live{keep}
		}},
		{name: "host_rekeyed", reason: domain.CloseHostRekeyed, action: "host.manage", control: 4110, run: func(w *world) (live, []live) {
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r2, w.h2)
			w.must(w.robin, http.MethodPost, "/hosts/"+w.r1.String()+"/pairing-code", revBody(), http.StatusCreated)
			return l, []live{keep}
		}},
		{name: "assignment_ended", reason: domain.CloseAccessRevoked, action: "host.manage", run: func(w *world) (live, []live) {
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			w.revIfMatch(http.MethodPatch, w.r1, "/hosts/"+w.r1.String(), revBody("revoke_session_users", []string{w.alex.PrincipalID}))
			return l, []live{keep}
		}},
		{name: "report_without_name", reason: domain.CloseAccessRevoked, event: "host.updated", run: func(w *world) (live, []live) {
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			if err := w.h1.Send("report", report(users("term-sam", "term-robin"), false)); err != nil {
				w.t.Fatal(err)
			}
			return l, []live{keep}
		}},
		{name: "assignee_moved", reason: domain.CloseAccessRevoked, action: "host.manage", run: func(w *world) (live, []live) {
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			w.revIfMatch(http.MethodPatch, w.r1, "/hosts/"+w.r1.String(), revBody("assigned_user_id", w.sam.PrincipalID))
			return l, []live{keep}
		}},
		{name: "report_server_host", reason: domain.CloseAccessRevoked, event: "host.updated", run: func(w *world) (live, []live) {
			l, keep := w.start(w.alex, w.r1, w.h1), w.start(w.robin, w.r1, w.h1)
			if err := w.h1.Send("report", report(r1Users(), true)); err != nil {
				w.t.Fatal(err)
			}
			return l, []live{keep}
		}},
		{name: "i6_violation", reason: domain.CloseI6Violation, action: "host.manage", run: func(w *world) (live, []live) {
			w.revAddOwner(w.sam)
			k3, r3 := w.pairedHost("r3", "", users("term-robin", "term-sam"),
				map[string]string{w.robin.PrincipalID: "term-robin", w.sam.PrincipalID: "term-sam"})
			h3 := w.connect(r3, k3, users("term-robin", "term-sam"))
			l, keep := w.start(w.sam, r3, h3), w.start(w.robin, r3, h3)
			w.revIfMatch(http.MethodPut, r3, "/hosts/"+r3.String()+"/llm-credentials",
				revBody("llm_credentials", "personal", "credential_holder_id", w.robin.PrincipalID))
			return l, []live{keep}
		}},
	}
}

// TestTMT10_RevocationKinds is TM-T10 on the server (24 sections 8.3, 3.1, 6.5 and 14; M2-46 Design
// notes 1, 3 to 6 and 11; threats T-TM-11, T-TM-03 and T-TM-08): every kind of revocation ends the
// session it concerns in the commit of its cause, with the reason of 24 section 10.3 and no detail,
// one terminal_session.changed, one terminal.terminate carrying the correlation of the cause, 4100
// with the reason on the device stream and the host data connection, one session.close at the
// Fake-Host and bothy.terminal.sessions.closed raised; every session the cause does not concern
// stays active and relays. A cause that leaves the host without a control connection (disabled,
// retired, re-paired; 24 sections 3.1, 3.3 and 3.5) instructs the host service through the close
// code of the control connection (4111 or 4110: the host service ends all PTYs), which the server
// sends right after the commit, before the event log tail could send session.close; these rows
// assert that close code instead of session.close (24 section 3.5 "Host service"; reported to the
// integrator as a deviation from the assertion list of the brief). The row owner_removed_SR10 is the integration row of TestSR10Lost
// (07 SR10 on a change of the owner status, Design note 4 b).
func TestTMT10_RevocationKinds(t *testing.T) {
	t.Parallel()
	for _, c := range revCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t, revOptions())
			l, keep := c.run(w)
			w.revAssertEnded(l, c)
			switch {
			case c.job:
				w.revSameTx(l.id, c.action)
			case c.action != "":
				w.revSameCommit(l.id, c.action)
			case c.event != "":
				w.revSameEventCommit(l.id, c.event, l.host.ID)
			}
			if got := w.text(`SELECT coalesce(close_detail, '') FROM terminal_sessions WHERE id = $1`, l.id); got != "" {
				t.Fatalf("close_detail %q, want none", got)
			}
			for _, k := range keep {
				w.revUntouched(k)
			}
		})
	}
}

// revCaseNamed returns the row name of revCases.
func revCaseNamed(t *testing.T, name string) revCase {
	t.Helper()
	for _, c := range revCases() {
		if c.name == name {
			return c
		}
	}
	t.Fatalf("no row %s", name)
	return revCase{}
}

// TestTMT10_NoReattachAfterEnd proves that an ended session cannot be continued without a new
// opening (M2-46 requirement 22, Tests "TestTMT10_NoReattachAfterEnd", 24 sections 6.2 and 8.3):
// after the rows grant_revoked, device_revoked, logout, host_disabled and an idle end,
// attachTerminalSession for the ended session answers a problem with a code and no session_token,
// both as a quick reattach from the device of the session without a step-up within
// terminal.quick_reattach_window and from Alex's second device with a fresh step-up; the row stays
// closed with its reason.
func TestTMT10_NoReattachAfterEnd(t *testing.T) {
	t.Parallel()
	rows := []revCase{revCaseNamed(t, "grant_revoked"), revCaseNamed(t, "device_revoked"), revCaseNamed(t, "logout"),
		revCaseNamed(t, "host_disabled"),
		{name: "idle_timeout", reason: domain.CloseIdleTimeout, run: func(w *world) (live, []live) {
			l := w.start(w.alex, w.r1, w.h1)
			w.advance(idleTimeout + idleGrace)
			return l, nil
		}}}
	for _, c := range rows {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			w := newWorld(t, revOptions())
			l, _ := c.run(w)
			w.revAssertEnded(l, c)
			w.revNoReattach(l, w.alex, false, c.reason)
			w.revNoReattach(l, w.alex2, true, c.reason)
		})
	}
}

// TestTMT10_HostOfflineGetsCloseOnReconnect proves the close path to a host service that was
// offline at the end (24 sections 6.2 and 8.3; M2-46 Design note 3, last paragraph; M2-43 Design
// note 6 b; threat T-TM-11): the Fake-Host of r1 loses its control connection while Alex works,
// Robin revokes Alex's grant, and the host that logs in again with the session in live_sessions
// receives session.close with access_revoked, with one terminal.terminate and one closed event.
func TestTMT10_HostOfflineGetsCloseOnReconnect(t *testing.T) {
	t.Parallel()
	w := newWorld(t, revOptions())
	l := w.start(w.alex, w.r1, w.h1)
	w.h1.Drop()
	w.waitState(l.id.String(), "detached")
	w.must(w.robin, http.MethodPost, "/grants/"+w.alexGrant+"/revoke", revBody(), http.StatusOK)
	w.waitState(l.id.String(), "closed:"+string(domain.CloseAccessRevoked))
	w.revSameCommit(l.id, "grant.revoke")
	// The login of the host by hand: the host sends nothing after ready, so session.close is the
	// server's own reaction to live_sessions (fakehost.Connect waits for heartbeat_ack after its
	// report and drops the frames before it).
	w.syncClock()
	ctx := w.ctx()
	c := hosttest.Dial(t, ctx, w.s.PublicURL)
	t.Cleanup(func() { _ = c.Close() })
	c.Login(ctx, hosttest.IdentifyParams{HostID: w.r1, Key: w.k1, LiveSessions: []ids.ID{l.id}})
	for {
		op, d, err := c.Next(ctx)
		if err != nil {
			t.Fatalf("waiting for session.close after ready: %v", err)
		}
		if op != "session.close" {
			continue
		}
		var cl rest.HostSessionClose
		if err := json.Unmarshal(d, &cl); err != nil {
			t.Fatal(err)
		}
		if ids.ID(cl.SessionId) != l.id || cl.CloseReason != rest.TerminalCloseReason(domain.CloseAccessRevoked) {
			t.Fatalf("session.close %s %s, want %s access_revoked", ids.ID(cl.SessionId), cl.CloseReason, l.id)
		}
		break
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1`, l.id); n != 1 {
		t.Fatalf("%d entries terminal.terminate, want 1", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'terminal_session.changed' AND resource_id = $1
		AND payload->>'state' = 'closed'`, l.id); n != 1 {
		t.Fatalf("%d events closed, want 1", n)
	}
}

// TestTMT10_IdentityAndRightsBothFire proves the idempotence of the three subscribers within one
// unit (M2-46 Design note 5 c; 08 section 11): disableUser of Alex publishes DeviceRevoked,
// SessionEnded and AccessChanged in one transaction (all of Alex's devices and login sessions are
// revoked and the access.changed of Alex carries the correlation of the end); each of Alex's two
// sessions (one per device) ends once with user_disabled, with one terminal.terminate, one closed
// event and one count of bothy.terminal.sessions.closed.
func TestTMT10_IdentityAndRightsBothFire(t *testing.T) {
	t.Parallel()
	w := newWorld(t, revOptions())
	l1, l2 := w.start(w.alex, w.r1, w.h1), w.start(w.alex2, w.r1, w.h1)
	w.must(w.robin, http.MethodPost, "/users/"+w.alex.PrincipalID+"/disable", revBody(), http.StatusOK)
	for _, l := range []live{l1, l2} {
		w.assertEnded(l, domain.CloseUserDisabled)
		w.revSameCommit(l.id, "user.disable")
		n := w.count(`SELECT count(*) FROM event_log e JOIN audit_events t ON t.correlation_id = e.correlation_id
			WHERE t.action = 'terminal.terminate' AND t.resource_id = $1 AND e.type = 'access.changed'`, l.id)
		if n == 0 {
			t.Fatalf("no access.changed in the commit of the end of %s", l.id)
		}
	}
	if n := w.count(`SELECT count(*) FROM devices WHERE user_id = $1 AND revoked_at IS NULL`, w.alex.PrincipalID); n != 0 {
		t.Fatalf("%d devices of Alex not revoked (DeviceRevoked)", n)
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE user_id = $1 AND revoked_at IS NULL`, w.alex.PrincipalID); n != 0 {
		t.Fatalf("%d login sessions of Alex not revoked (SessionEnded)", n)
	}
	if got := w.metrics.closedOf(domain.CloseUserDisabled); got != 2 {
		t.Fatalf("bothy.terminal.sessions.closed user_disabled %d, want 2", got)
	}
}

// revNode is a second server process on the database of the world, with its own deadline
// scheduler, session metrics and log.
type revNode struct {
	s       *harness.Server
	timers  *fakehost.ClockTimers
	metrics *sessionMetrics
	logs    *syncBuffer
	jobsMu  gosync.Mutex
	jobs    *wire.JobModules
}

// revStartNode starts a process of role on the database and the fake clock of w (cluster.mode
// single, the default of the test configuration).
func (w *world) revStartNode(role wire.Role) *revNode {
	w.t.Helper()
	n := &revNode{timers: fakehost.NewClockTimers(w.clk), metrics: newSessionMetrics(), logs: &syncBuffer{}}
	w.nodeTimers = append(w.nodeTimers, n.timers)
	n.s = harness.StartNode(w.t, w.s, harness.Options{Config: testConfig(revOptions().cfg), Clock: w.clk, TerminalTimers: n.timers,
		EgressDenyAll: true, LogWriter: n.logs, Role: role, TerminalSessionMetrics: n.metrics,
		JobModules: func(m *wire.JobModules) { n.jobsMu.Lock(); n.jobs = m; n.jobsMu.Unlock() }})
	return n
}

// jobModules returns the job use cases of the node.
func (n *revNode) jobModules(t *testing.T) *wire.JobModules {
	t.Helper()
	n.jobsMu.Lock()
	defer n.jobsMu.Unlock()
	if n.jobs == nil {
		t.Fatal("the jobs of the node are not built")
	}
	return n.jobs
}

// revStartOn opens a shell of c on host at node b (the open request, the device stream and the
// Fake-Host h all at b) and runs the handshake; the session is active when it returns.
func (w *world) revStartOn(b *revNode, c *harness.Client, host ids.ID, h *fakehost.Host) live {
	w.t.Helper()
	w.syncClock()
	if err := c.StepUp(w.t.Context()); err != nil {
		w.t.Fatal(err)
	}
	r := c.Do(w.t.Context(), harness.Request{Method: http.MethodPost, Path: "/hosts/" + host.String() + "/terminal/sessions",
		Body: openBody()})
	if r.Status != http.StatusCreated {
		w.t.Fatalf("openTerminalSession at B: %d %s", r.Status, r.Body)
	}
	var g grantBody
	r.JSON(w.t, &g)
	id := ids.MustParse(g.Session.ID)
	d, status, err := fakehost.DialDevice(w.t, w.ctx(), b.s.PublicURL, b.s.PublicURL, id, c, fakehost.DeviceParams{DeviceID: c.DeviceID})
	if err != nil {
		w.t.Fatalf("device stream at B: %d %v", status, err)
	}
	w.handshake(d, g.SessionToken)
	w.waitState(g.Session.ID, "active")
	data, err := h.WaitData(w.ctx(), id)
	if err != nil {
		w.t.Fatalf("data connection at B: %v", err)
	}
	return live{g: g, id: id, dev: d, data: data, host: h}
}

// revEndedAt checks the end of l held by another process: the row closed with reason, 4100 with
// the reason on the device stream and the data connection, session.close at the Fake-Host, one
// terminal.terminate, one closed event, and the metric raised in the committing process m.
func (w *world) revEndedAt(l live, reason domain.CloseReason, m *sessionMetrics) {
	w.t.Helper()
	w.waitState(l.id.String(), "closed:"+string(reason))
	w.closeCode("device stream", l.dev.CloseCode, 4100, string(reason))
	w.closeCode("data connection", l.data.CloseCode, 4100, string(reason))
	if got, err := l.host.WaitClose(w.ctx(), l.id); err != nil || got != string(reason) {
		w.t.Fatalf("session.close at the host: %q %v, want %s", got, err, reason)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1`, l.id); n != 1 {
		w.t.Fatalf("%d entries terminal.terminate, want 1", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'terminal_session.changed' AND resource_id = $1
		AND payload->>'state' = 'closed'`, l.id); n != 1 {
		w.t.Fatalf("%d events closed, want 1", n)
	}
	w.eventually("bothy.terminal.sessions.closed "+string(reason)+" in the committing process", func() bool { return m.closedOf(reason) > 0 })
}

// TestTMT10_CloseReachesOtherProcess proves the close path across processes with cluster.mode
// single (M2-46 Design note 3; 02 sections 11.2 and 16 level 1; threat T-TM-11): instances A (the
// world's server) and B (role all) and a worker share one database; the host r3 keeps its control
// connection at B and Alex's sessions run through B. A revokeGrant committed at A, and in a second
// session the handler of terminal.sweep committed in the worker (max_duration after the fake clock
// passed the window without firing a relay deadline), both close B's device stream and data
// connection with 4100 and the reason, and B (the holder of the control connection) sends
// session.close. With cluster.mode single no listener of NOTIFY ephemeral runs, so the event log
// tail alone carried the end (B's log has no line of the ephemeral listener).
func TestTMT10_CloseReachesOtherProcess(t *testing.T) {
	t.Parallel()
	w := newWorld(t, revOptions())
	b := w.revStartNode(wire.RoleAll)
	worker := w.revStartNode(wire.RoleWorker)
	k3, r3 := w.pairedHost("r3", w.alex.PrincipalID, users("term-alex"), map[string]string{w.alex.PrincipalID: "term-alex"})
	grant := w.grant("user", w.alex.PrincipalID, r3.String(), "")
	w.syncClock()
	h3 := fakehost.Connect(t, b.s.PublicURL, r3, k3, report(users("term-alex"), false))
	h3.Default(fakehost.Script{Kind: fakehost.Accept})
	alexB, err := b.s.UserSession(t.Context(), w.alex.PrincipalID)
	if err != nil {
		t.Fatal(err)
	}

	l := w.revStartOn(b, alexB, r3, h3)
	w.must(w.robin, http.MethodPost, "/grants/"+grant+"/revoke", revBody(), http.StatusOK)
	w.revEndedAt(l, domain.CloseAccessRevoked, w.metrics)
	w.revSameCommit(l.id, "grant.revoke")

	w.grant("user", w.alex.PrincipalID, r3.String(), "")
	l2 := w.revStartOn(b, alexB, r3, h3)
	w.jump(maxDuration)
	res, err := worker.jobModules(t).TerminalSweep.Sweep(w.ctx())
	if err != nil {
		t.Fatalf("terminal.sweep in the worker: %v", err)
	}
	if got := w.state(l2.id.String()); got != "closed:"+string(domain.CloseMaxDuration) {
		t.Fatalf("after the sweep of the worker (%d ended) the session is %s; opened_at %s, now %s", res.Ended, got,
			w.text(`SELECT opened_at::text FROM terminal_sessions WHERE id = $1`, l2.id), w.clk.Now())
	}
	w.revEndedAt(l2, domain.CloseMaxDuration, worker.metrics)
	for _, logs := range []string{b.logs.String(), worker.logs.String(), w.logs.String()} {
		// Since M7-17 a process in role api holds the shared LISTEN ephemeral connection in every
		// mode (hints of a separate worker); the terminal's route and its drops stay off.
		if strings.Contains(logs, "terminal signal dropped") || strings.Contains(logs, "ephemeral listener lost") {
			t.Fatal("the terminal received NOTIFY ephemeral with cluster.mode single")
		}
	}
}

// revAuditDetails returns the details of the terminal.terminate entry of the session.
func (w *world) revAuditDetails(id ids.ID) map[string]any {
	w.t.Helper()
	var d map[string]any
	raw := w.text(`SELECT details::text FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1`, id)
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		w.t.Fatal(err)
	}
	return d
}

// revCode matches a code of a closed list.
var revCode = regexp.MustCompile(`^[a-z0-9_]+$`)

// TestAuditTerminate_Details proves the audit entry of an end by the server (24 section 10.1; M2-46
// requirement 16, Design note 11; SEC-AUD-5, R-TM-10): the details of terminal.terminate are
// exactly session_id, close_reason, bytes_in, bytes_out and duration_ms, plus close_detail exactly
// when the end has a detail (an opening that times out gives open_failed with open_timeout); the
// values are the session ID, codes of the closed lists and integers, and duration_ms is closed_at
// minus opened_at in milliseconds; the entry is a system action without actor.
func TestAuditTerminate_Details(t *testing.T) {
	t.Parallel()
	w := newWorld(t, revOptions())
	l := w.start(w.alex, w.r1, w.h1)
	w.must(w.robin, http.MethodPost, "/grants/"+w.alexGrant+"/revoke", revBody(), http.StatusOK)
	w.assertEnded(l, domain.CloseAccessRevoked)
	g := w.openTimedOut(w.sam, w.r2, w.h2)
	opening := ids.MustParse(g.Session.ID)
	for _, c := range []struct {
		id     ids.ID
		reason string
		detail string
	}{{l.id, "access_revoked", ""}, {opening, "open_failed", "open_timeout"}} {
		d := w.revAuditDetails(c.id)
		want := []string{"bytes_in", "bytes_out", "close_reason", "duration_ms", "session_id"}
		if c.detail != "" {
			want = append(want, "close_detail")
		}
		slices.Sort(want)
		keys := make([]string, 0, len(d))
		for k := range d {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if !slices.Equal(keys, want) {
			t.Fatalf("details of %s have the keys %v, want %v", c.reason, keys, want)
		}
		if d["session_id"] != c.id.String() || d["close_reason"] != c.reason || c.detail != "" && d["close_detail"] != c.detail {
			t.Fatalf("details of %s: %v", c.reason, d)
		}
		for _, k := range []string{"close_reason", "close_detail"} {
			if v, ok := d[k]; ok && (!revIsString(v) || !revCode.MatchString(v.(string))) {
				t.Fatalf("details %s of %s is no code: %v", k, c.reason, v)
			}
		}
		for _, k := range []string{"bytes_in", "bytes_out", "duration_ms"} {
			f, ok := d[k].(float64)
			if !ok || f != float64(int64(f)) || f < 0 {
				t.Fatalf("details %s of %s is no integer: %v", k, c.reason, d[k])
			}
		}
		ms := w.count(`SELECT floor(extract(epoch FROM closed_at - opened_at) * 1000)::bigint FROM terminal_sessions WHERE id = $1`, c.id)
		if int64(d["duration_ms"].(float64)) != int64(ms) {
			t.Fatalf("duration_ms of %s is %v, closed_at minus opened_at is %d ms", c.reason, d["duration_ms"], ms)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1
			AND actor_principal_id IS NULL AND decision = 'allow'`, c.id); n != 1 {
			t.Fatalf("terminal.terminate of %s is no allowed system action", c.reason)
		}
	}
}

// revIsString reports whether v is a string.
func revIsString(v any) bool { _, ok := v.(string); return ok }

// TestAuditTerminate_ExactlyOnce proves one audit entry per end (M2-46 Design notes 1, 11 and 15;
// threat T-AZ-04 of 5.16 "Host service R"): a revokeGrant and the handler of terminal.sweep (the
// fake clock past terminal.max_session_duration without firing a relay deadline) race on one
// session, and exactly one of them ends it: one terminal.terminate, one closed event, one count of
// bothy.terminal.sessions.closed; a later sweep, revocation or host report changes nothing. An end
// through closeTerminalSession keeps its terminal.close entry and writes no terminal.terminate.
func TestAuditTerminate_ExactlyOnce(t *testing.T) {
	t.Parallel()
	t.Run("race", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, revOptions())
		l := w.start(w.alex, w.r1, w.h1)
		w.jump(maxDuration)
		// The step-up of revokeGrant is fresh on the advanced clock, so the request races the sweep
		// directly.
		if err := w.robin.StepUp(t.Context()); err != nil {
			t.Fatal(err)
		}
		var wg gosync.WaitGroup
		var revoke harness.Response
		var sweepErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			revoke = w.s.Owner.Do(context.Background(), harness.Request{Method: http.MethodPost, Path: "/grants/" + w.alexGrant + "/revoke",
				Body: revBody()})
		}()
		go func() {
			defer wg.Done()
			_, sweepErr = w.jobModules().TerminalSweep.Sweep(w.ctx())
		}()
		wg.Wait()
		if revoke.Status != http.StatusOK || sweepErr != nil {
			t.Fatalf("revokeGrant %d %s, sweep %v", revoke.Status, revoke.Body, sweepErr)
		}
		state := w.state(l.id.String())
		if state != "closed:access_revoked" && state != "closed:max_duration" {
			t.Fatalf("state %s", state)
		}
		w.sweep()
		if err := w.h1.Send("session.ended", map[string]any{"session_id": l.id.String(), "close_reason": "process_exited"}); err != nil {
			t.Fatal(err)
		}
		if err := l.data.Sync(w.ctx()); err != nil && !strings.Contains(err.Error(), "close") {
			t.Logf("data connection after the end: %v", err)
		}
		if got := w.state(l.id.String()); got != state {
			t.Fatalf("a later end changed the session: %s, want %s", got, state)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1`, l.id); n != 1 {
			t.Fatalf("%d entries terminal.terminate, want 1", n)
		}
		if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'terminal_session.changed' AND resource_id = $1
			AND payload->>'state' = 'closed'`, l.id); n != 1 {
			t.Fatalf("%d events closed, want 1", n)
		}
		total := w.metrics.closedOf(domain.CloseAccessRevoked) + w.metrics.closedOf(domain.CloseMaxDuration) +
			w.metrics.closedOf(domain.CloseProcessExited)
		if total != 1 {
			t.Fatalf("bothy.terminal.sessions.closed counted %d ends, want 1", total)
		}
	})
	t.Run("close_request", func(t *testing.T) {
		t.Parallel()
		w := newWorld(t, revOptions())
		l := w.start(w.alex, w.r1, w.h1)
		w.must(w.alex, http.MethodPost, "/terminal/sessions/"+l.id.String()+"/close", revBody(), http.StatusOK)
		w.waitState(l.id.String(), "closed:client_closed")
		w.must(w.robin, http.MethodPost, "/grants/"+w.alexGrant+"/revoke", revBody(), http.StatusOK)
		w.sweep()
		if got := w.state(l.id.String()); got != "closed:client_closed" {
			t.Fatalf("state %s", got)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.terminate' AND resource_id = $1`, l.id); n != 0 {
			t.Fatalf("%d entries terminal.terminate for a close by the person, want 0", n)
		}
		if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'terminal.close' AND resource_id = $1
			AND decision = 'allow'`, l.id); n != 1 {
			t.Fatalf("%d entries terminal.close, want 1", n)
		}
	})
}

// TestEventsValidate proves the personal event of the session (24 section 10.2; 05 section 6;
// M2-46 requirement 20; ADR 0052 and 0053): in a world with openings, a start, a detach, an attach
// and ends by a revocation, an open timeout and a close by the person, every terminal_session.changed
// row, mapped to its envelope as the gateway delivers it, validates against
// TerminalSessionChangedEvent, is personal to the person of the session (audience_principal_id and
// payload user_id equal terminal_sessions.user_id, receive action self) and has no JSON null.
func TestEventsValidate(t *testing.T) {
	t.Parallel()
	w := newWorld(t, revOptions())
	l := w.start(w.alex, w.r1, w.h1)
	_ = l.dev.Close()
	w.waitState(l.id.String(), "detached")
	r := w.attach(w.alex, l.id.String(), false)
	if r.Status != http.StatusOK {
		t.Fatalf("quick reattach: %d %s", r.Status, r.Body)
	}
	var g grantBody
	r.JSON(t, &g)
	d := w.device(w.alex, l.id.String())
	w.handshake(d, g.SessionToken)
	w.waitState(l.id.String(), "active")
	w.must(w.robin, http.MethodPost, "/grants/"+w.alexGrant+"/revoke", revBody(), http.StatusOK)
	w.waitState(l.id.String(), "closed:access_revoked")
	w.openTimedOut(w.sam, w.r2, w.h2)
	c := w.start(w.robin, w.r2, w.h2)
	w.must(w.robin, http.MethodPost, "/terminal/sessions/"+c.id.String()+"/close", revBody(), http.StatusOK)
	w.waitState(c.id.String(), "closed:client_closed")

	rows, err := w.s.AdminDB().Query(context.Background(), `SELECT e.seq, e.id, e.type, e.schema_version, e.occurred_at,
		coalesce(e.actor_kind, ''), coalesce(e.actor_principal_id::text, ''), e.resource_kind, e.resource_id, coalesce(e.resource_version, 0),
		coalesce(e.audience_principal_id::text, ''), e.receive_action, e.payload::text, coalesce(e.correlation_id::text, ''),
		e.permission_version, s.user_id::text
		FROM event_log e JOIN terminal_sessions s ON s.id = e.resource_id WHERE e.type = 'terminal_session.changed' ORDER BY e.seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	states := map[string]bool{}
	n := 0
	for rows.Next() {
		var ev sync.Event
		var actor, audience, payload, corr, owner, receive string
		if err := rows.Scan(&ev.Seq, &ev.ID, &ev.Type, &ev.SchemaVersion, &ev.OccurredAt, &ev.ActorKind, &actor, &ev.ResourceKind, &ev.ResourceID,
			&ev.ResourceVersion, &audience, &receive, &payload, &corr, &ev.PermissionVersion, &owner); err != nil {
			t.Fatal(err)
		}
		n++
		ev.ActorPrincipalID, ev.AudiencePrincipalID, ev.CorrelationID = revID(t, actor), revID(t, audience), revID(t, corr)
		ev.Payload = json.RawMessage(payload)
		raw, err := json.Marshal(ev.Envelope(sync.DeliveryFields{}))
		if err != nil {
			t.Fatal(err)
		}
		problems, err := revValidateSchema("TerminalSessionChangedEvent", raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(problems) > 0 {
			t.Fatalf("event %d does not validate against TerminalSessionChangedEvent: %v", ev.Seq, problems)
		}
		var p map[string]any
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			t.Fatal(err)
		}
		if revHasNull(p) {
			t.Fatalf("event %d has a JSON null: %s", ev.Seq, payload)
		}
		if audience != owner || p["user_id"] != owner || receive != "self" {
			t.Fatalf("event %d is not personal to %s: audience %s, payload user_id %v, receive %s", ev.Seq, owner, audience, p["user_id"], receive)
		}
		states[p["state"].(string)] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"opening", "active", "detached", "closed"} {
		if !states[s] {
			t.Fatalf("no event with state %s among %d events", s, n)
		}
	}
}

// revID decodes the UUID text s without the UUIDv7 check of ids.Parse (a correlation_id is a W3C
// trace ID written as a UUID), the zero ID for "".
func revID(t *testing.T, s string) ids.ID {
	t.Helper()
	var id ids.ID
	if s == "" {
		return id
	}
	b, err := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	if err != nil || len(b) != len(id) {
		t.Fatalf("no UUID: %q", s)
	}
	copy(id[:], b)
	return id
}

// TestCloseValuesOnlyFromLists proves the closed lists and pairs of 24 section 10.3 in the stored
// rows (M2-46 requirement 12, Design note 2): in one world ends by a revocation, a host report with
// a detail (open_failed with pty_failed through the Fake-Host script End), an open timeout, an idle
// end and a close by the person leave in terminal_sessions only pairs of close_reason and
// close_detail that domain.ValidateEnd allows, every reason from domain.CloseReasons and every
// detail from domain.CloseDetails.
func TestCloseValuesOnlyFromLists(t *testing.T) {
	t.Parallel()
	w := newWorld(t, revOptions())
	l := w.start(w.alex, w.r1, w.h1)
	w.must(w.robin, http.MethodPost, "/grants/"+w.alexGrant+"/revoke", revBody(), http.StatusOK)
	w.waitState(l.id.String(), "closed:access_revoked")
	w.h2.Default(fakehost.Script{Kind: fakehost.End, Reason: "open_failed", Detail: "pty_failed"})
	refused := w.open(w.sam, w.r2)
	w.waitState(refused.Session.ID, "closed:open_failed:pty_failed")
	w.h2.Default(fakehost.Script{Kind: fakehost.Accept})
	w.openTimedOut(w.sam, w.r2, w.h2)
	idle := w.start(w.sam, w.r2, w.h2)
	w.advance(idleTimeout + idleGrace)
	w.waitState(idle.id.String(), "closed:idle_timeout")
	c := w.start(w.robin, w.r2, w.h2)
	w.must(w.robin, http.MethodPost, "/terminal/sessions/"+c.id.String()+"/close", revBody(), http.StatusOK)
	w.waitState(c.id.String(), "closed:client_closed")

	rows, err := w.s.AdminDB().Query(context.Background(),
		`SELECT DISTINCT close_reason, coalesce(close_detail, '') FROM terminal_sessions WHERE state = 'closed'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var r, d string
		if err := rows.Scan(&r, &d); err != nil {
			t.Fatal(err)
		}
		seen[r+":"+d] = true
		if !slices.Contains(domain.CloseReasons(), domain.CloseReason(r)) {
			t.Fatalf("close_reason %q is not in the closed list", r)
		}
		if d != "" && !slices.Contains(domain.CloseDetails(), domain.CloseDetail(d)) {
			t.Fatalf("close_detail %q is not in the closed list", d)
		}
		if err := domain.ValidateEnd(domain.CloseReason(r), domain.CloseDetail(d)); err != nil {
			t.Fatalf("the stored pair %s %s is not allowed: %v", r, d, err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"access_revoked:", "open_failed:pty_failed", "open_failed:open_timeout", "idle_timeout:", "client_closed:"} {
		if !seen[want] {
			t.Fatalf("the pair %s was not produced (%v)", want, seen)
		}
	}
}
