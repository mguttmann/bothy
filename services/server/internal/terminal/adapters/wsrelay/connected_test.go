// SPDX-License-Identifier: Apache-2.0

package wsrelay_test

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/instance"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/terminal/adapters/hosttest"
	"github.com/OWNER/bothy/services/server/internal/terminal/app"
	"github.com/OWNER/bothy/services/server/test/terminalworld"
)

// TestDuplicate_TMT23 (M2-43 Design note 5, 24 section 3.4, TM-T23): (a) the older answers the
// ping: both close with 4112, H1 is disabled with duplicate_connection, Alex's active session
// ends with host_disabled, one deny entry, host.updated, and enabling again is 409; (b) the older
// is paused: it closes with 4009, the new one gets ready, one allow entry
// stale_connection_replaced; (c) H2 confirming with a live duplicate: both 4112, H2 in pairing
// without keys.
func TestDuplicate_TMT23(t *testing.T) {
	t.Parallel()
	t.Run("live duplicate disables", func(t *testing.T) {
		t.Parallel()
		c := newCtl(t)
		ses := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
		old, _ := c.login(c.h1, ses)
		updates := len(c.Events("host.updated"))
		dup := c.dial()
		if err := dup.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: c.key(c.h1)}); err != nil {
			t.Fatal(err)
		}
		c.expectClose(dup, 4112)
		c.expectClose(old, 4112)
		if s := c.state(c.h1); s != "disabled" || c.Text(`SELECT disabled_reason FROM hosts WHERE id = $1`, c.h1.String()) != "duplicate_connection" {
			t.Fatalf("state %s", s)
		}
		if s := c.sessionState(ses); s != "closed:host_disabled" {
			t.Errorf("session %s", s)
		}
		var deny []terminalworld.AuditRow
		for _, r := range c.Audits("host.connect") {
			if r.Decision == "deny" && r.Reason == "host_duplicate_connection" {
				deny = append(deny, r)
			}
		}
		if len(deny) != 1 || !hasDetail(deny[0], "code", "duplicate_connection") || deny[0].Actor != "" || deny[0].ResourceID != c.h1.String() {
			t.Errorf("deny entries %+v", deny)
		}
		if len(c.Events("host.updated")) <= updates {
			t.Error("no host.updated")
		}
		c.Want(c.Do("robin", http.MethodPost, terminalworld.HostPath(c.h1)+"/state", `{"state":"active","reason":"Back"}`, c.Keyed(c.h1)...),
			http.StatusConflict, "invalid_state_transition")
	})
	t.Run("dead older is replaced", func(t *testing.T) {
		t.Parallel()
		c := newCtl(t)
		old, _ := c.login(c.h1)
		old.Pause()
		_, ready := c.login(c.h1)
		if ready.State != "active" {
			t.Fatalf("ready %+v", ready)
		}
		old.Unpause()
		c.expectClose(old, 4009)
		var allow []terminalworld.AuditRow
		for _, r := range c.Allows("host.connect") {
			if hasDetail(r, "code", "stale_connection_replaced") {
				allow = append(allow, r)
			}
		}
		if len(allow) != 1 || c.state(c.h1) != "active" {
			t.Errorf("allow entries %+v, state %s", allow, c.state(c.h1))
		}
	})
	t.Run("unconfirmed host falls back to pairing", func(t *testing.T) {
		t.Parallel()
		c := newCtl(t)
		old, _ := c.login(c.h2)
		dup := c.dial()
		if err := dup.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h2, Key: c.key(c.h2)}); err != nil {
			t.Fatal(err)
		}
		c.expectClose(dup, 4112)
		c.expectClose(old, 4112)
		if s := c.state(c.h2); s != "pairing" {
			t.Fatalf("H2 state %s", s)
		}
		if n := c.Count(`SELECT count(*) FROM hosts WHERE id = $1 AND auth_jkt IS NULL AND noise_public_key IS NULL`, c.h2.String()); n != 1 {
			t.Error("keys kept")
		}
	})
}

// TestLiveSessions_HostRestarted_TMT16 (M2-43 Design note 6, TM-T16): Alex on H1 with opening A,
// active B, detached C, active D and closed E (access_revoked); identify lists B, E and an unknown
// X: C and D end with host_restarted, each with terminal_session.changed to Alex and one
// terminal.terminate entry; the client gets session.close for E and X after ready; A and B stay.
func TestLiveSessions_HostRestarted_TMT16(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	a := c.TerminalSession(c.h1, c.Alex, c.alexSU, "opening")
	b := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
	cs := c.TerminalSession(c.h1, c.Alex, c.alexSU, "detached")
	d := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
	e := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
	c.SQL(`UPDATE terminal_sessions SET state = 'closed', closed_at = now(), close_reason = 'access_revoked' WHERE id = $1`, e.String())
	x := ids.New()
	cl, _ := c.login(c.h1, b, e, x)
	got := map[string]string{}
	for range 2 {
		var sc rest.HostSessionClose
		cl.NextOp(c.ctx(), "session.close", &sc)
		got[ids.ID(sc.SessionId).String()] = string(sc.CloseReason)
	}
	if got[e.String()] != "access_revoked" || got[x.String()] != "access_revoked" {
		t.Errorf("session.close %v", got)
	}
	for id, want := range map[ids.ID]string{a: "opening", b: "active", cs: "closed:host_restarted", d: "closed:host_restarted"} {
		if s := c.sessionState(id); s != want {
			t.Errorf("session %s, want %s", s, want)
		}
	}
	changed := 0
	for _, ev := range c.Events("terminal_session.changed") {
		if ev.Audience == c.Alex.String() && (ev.ResourceID == cs.String() || ev.ResourceID == d.String()) {
			changed++
		}
	}
	terminated := 0
	for _, r := range c.Allows("terminal.terminate") {
		if hasDetail(r, "close_reason", "host_restarted") {
			terminated++
		}
	}
	if changed != 2 || terminated != 2 {
		t.Errorf("terminal_session.changed %d, terminal.terminate %d, want 2 each", changed, terminated)
	}
}

// TestReportBeforeSessionOpen (M2-43 Design note 7 a and d, 24 section 2.2 point 5): after ready
// without report OpenSession gives ErrHostNotReady and the host is not connected; after the report
// host.presence_changed {connected: true} is published, Robin and Alex receive it and Sam does not
// (no host.read), and OpenSession delivers session.open.
func TestReportBeforeSessionOpen(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	if err := c.hc.Links.OpenSession(context.Background(), c.h1, app.SessionOpen{}); !errors.Is(err, app.ErrHostNotConnected) {
		t.Fatalf("no link: %v", err)
	}
	cl, _ := c.login(c.h1)
	open := app.SessionOpen{SessionID: ids.New(), DataPath: "/api/v1/hosts/sessions/x/stream", Profile: "shell", ExpiresAt: c.Clk.Now().Add(time.Minute)}
	if err := c.hc.Links.OpenSession(context.Background(), c.h1, open); !errors.Is(err, app.ErrHostNotReady) {
		t.Fatalf("before report: %v", err)
	}
	if c.hc.Links.Connected(c.h1) {
		t.Fatal("connected before the report")
	}
	c.send(cl, "report", report("bothy-s-alex", "bothy-s-robin", "bothy-s-sam"))
	c.sync(cl)
	if !c.hc.Links.Connected(c.h1) || !slices.Equal(c.pub.presence(c.h1), []bool{true}) {
		t.Fatalf("presence %v", c.pub.presence(c.h1))
	}
	if !c.receives("robin", c.h1) || !c.receives("alex", c.h1) || c.receives("sam", c.h1) {
		t.Error("host.presence_changed must reach exactly Robin and Alex")
	}
	if err := c.hc.Links.OpenSession(context.Background(), c.h1, open); err != nil {
		t.Fatal(err)
	}
	var so rest.HostSessionOpen
	cl.NextOp(c.ctx(), "session.open", &so)
	if ids.ID(so.SessionId) != open.SessionID || so.Profile != "shell" {
		t.Errorf("session.open %+v", so)
	}
	m := c.Want(c.Do("robin", http.MethodGet, terminalworld.HostPath(c.h1), ""), http.StatusOK, "")
	if m["connected"] != true {
		t.Errorf("getHost connected %v", m["connected"])
	}
}

// TestReport_StoresListsAfterHD7_TMT29c (M2-43, TM-T29 (c)): Sam's list without the folder HD7
// dropped is stored exactly in order; getHost shows the lists; host.updated validates.
func TestReport_StoresListsAfterHD7_TMT29c(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl, _ := c.login(c.h1)
	r := report("bothy-s-alex", "bothy-s-robin", "bothy-s-sam")
	r["session_users"].([]any)[2] = map[string]any{"os_user": "bothy-s-sam", "workdirs": []any{"Zeta", "Alpha"}}
	c.send(cl, "report", r)
	c.sync(cl)
	m := c.Want(c.Do("robin", http.MethodGet, terminalworld.HostPath(c.h1), ""), http.StatusOK, "")
	list := m["reported_session_users"].([]any)
	sam := list[2].(map[string]any)
	if sam["os_user"] != "bothy-s-sam" || len(sam["workdirs"].([]any)) != 2 || sam["workdirs"].([]any)[0] != "Zeta" {
		t.Fatalf("reported %v", list)
	}
	evs := c.Events("host.updated")
	if p := evs[len(evs)-1].Payload["host"].(map[string]any); p["os_version"] != "Debian 13" {
		t.Errorf("host.updated %v", p)
	}
}

// TestReport_AuditOnRelevantChange and TestReport_StoresDoctorEntriesPerAccount (M2-43 Design note
// 8 b, T-TM-12, T-TM-14): HD6 failing for bothy-s-alex and bothy-s-sam, HD10 with bothy-s-sam, HD9
// with a former name and one entry without a name: stored as sent, one allow entry with the
// accounts; the same report again writes nothing; a profile-only change writes host.updated
// without audit.
func TestReport_AuditOnRelevantChange(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl, _ := c.login(c.h1)
	base := report("bothy-s-alex", "bothy-s-robin", "bothy-s-sam")
	c.send(cl, "report", base)
	c.sync(cl)
	allows := len(c.Allows("host.connect"))
	r := report("bothy-s-alex", "bothy-s-robin", "bothy-s-sam")
	// HD9 fails for a former session user and for another user (the entry without a name), so its
	// passed entry goes.
	var doc []any
	for _, e := range checks(
		map[string]any{"check": "hd6_llm_login", "ok": false, "code": "subscription_login_detected", "session_user": "bothy-s-alex"},
		map[string]any{"check": "hd6_llm_login", "ok": false, "code": "subscription_login_detected", "session_user": "bothy-s-sam"},
		map[string]any{"check": "hd10_helper_integrity", "ok": false, "code": "helper_untrusted", "session_user": "bothy-s-sam"},
		map[string]any{"check": "hd9_foreign_homes", "ok": false, "code": "foreign_home", "session_user": "bothy-s-former"},
		map[string]any{"check": "hd9_foreign_homes", "ok": false, "code": "foreign_home"},
	) {
		if m := e.(map[string]any); m["check"] == "hd9_foreign_homes" && m["ok"] == true {
			continue
		}
		doc = append(doc, e)
	}
	r["doctor"] = doc
	c.send(cl, "report", r)
	c.sync(cl)
	rows := c.Allows("host.connect")
	if len(rows) != allows+1 {
		t.Fatalf("%d allow entries, want one more", len(rows)-allows)
	}
	last := rows[len(rows)-1]
	for key, want := range map[string]string{"hd6_users": `["bothy-s-alex","bothy-s-sam"]`, "hd10_users": `["bothy-s-sam"]`,
		"hd9_users": `["bothy-s-former"]`, "changed": `["doctor"]`} {
		if !hasDetail(last, key, want) {
			t.Errorf("%s: %v", key, last.Details[key])
		}
	}
	if !hasDetail(last, "failed_checks", "hd6_llm_login:subscription_login_detected") || last.Actor != "" {
		t.Errorf("details %v actor %q", last.Details, last.Actor)
	}
	stored := c.Text(`SELECT reported_doctor::text FROM hosts WHERE id = $1`, c.h1.String())
	if strings.Count(stored, `"session_user"`) != 4 {
		t.Errorf("reported_doctor %s", stored)
	}
	version := c.HostVersion(c.h1)
	c.send(cl, "report", r)
	c.sync(cl)
	if len(c.Allows("host.connect")) != allows+1 || c.HostVersion(c.h1) != version {
		t.Error("the same report wrote something")
	}
	r["profiles"] = []any{map[string]any{"name": "shell", "display_name": "Bash", "kind": "shell"}}
	updates := len(c.Events("host.updated"))
	c.send(cl, "report", r)
	c.sync(cl)
	if len(c.Allows("host.connect")) != allows+1 || len(c.Events("host.updated")) != updates+1 {
		t.Error("a profile change must write host.updated without audit")
	}
}

// TestReport_DroppedNameEndsSessions_SR12 (M2-43 Design note 4 c, 07 SR12): Alex has an active and
// a detached session, Robin one active; a report without bothy-s-alex ends Alex's two with
// access_revoked, sends session.close for both, leaves Robin's and raises permission_version by
// one; the same report again raises nothing.
func TestReport_DroppedNameEndsSessions_SR12(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	a1 := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
	a2 := c.TerminalSession(c.h1, c.Alex, c.alexSU, "detached")
	r1 := c.TerminalSession(c.h1, c.Robin, c.robinSU, "active")
	cl, _ := c.login(c.h1, a1, a2, r1)
	c.send(cl, "report", report("bothy-s-alex", "bothy-s-robin", "bothy-s-sam"))
	c.sync(cl)
	pv := c.PermissionVersion()
	c.send(cl, "report", report("bothy-s-robin", "bothy-s-sam"))
	got := map[string]string{}
	for range 2 {
		var sc rest.HostSessionClose
		cl.NextOp(c.ctx(), "session.close", &sc)
		got[ids.ID(sc.SessionId).String()] = string(sc.CloseReason)
	}
	if got[a1.String()] != "access_revoked" || got[a2.String()] != "access_revoked" {
		t.Errorf("session.close %v", got)
	}
	if c.sessionState(a1) != "closed:access_revoked" || c.sessionState(a2) != "closed:access_revoked" || c.sessionState(r1) != "active" {
		t.Errorf("states %s %s %s", c.sessionState(a1), c.sessionState(a2), c.sessionState(r1))
	}
	if c.PermissionVersion() != pv+1 {
		t.Errorf("permission_version %d, want %d", c.PermissionVersion(), pv+1)
	}
	c.send(cl, "report", report("bothy-s-robin", "bothy-s-sam"))
	c.sync(cl)
	if c.PermissionVersion() != pv+1 {
		t.Error("the same report raised the rights version")
	}
}

// TestReport_ServerHostEndsNonOwnerSessions_SR10 (M2-43 Design note 8 c, 07 SR10): the flag turns
// on; Alex's sessions end with access_revoked, Robin's stays; one audit entry with changed
// is_server_host; permission_version unchanged.
func TestReport_ServerHostEndsNonOwnerSessions_SR10(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	a1 := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
	r1 := c.TerminalSession(c.h1, c.Robin, c.robinSU, "active")
	cl, _ := c.login(c.h1, a1, r1)
	c.send(cl, "report", report("bothy-s-alex", "bothy-s-robin", "bothy-s-sam"))
	c.sync(cl)
	pv, allows := c.PermissionVersion(), len(c.Allows("host.connect"))
	r := report("bothy-s-alex", "bothy-s-robin", "bothy-s-sam")
	r["is_server_host"] = true
	c.send(cl, "report", r)
	var sc rest.HostSessionClose
	cl.NextOp(c.ctx(), "session.close", &sc)
	if ids.ID(sc.SessionId) != a1 || c.sessionState(a1) != "closed:access_revoked" || c.sessionState(r1) != "active" {
		t.Errorf("close %+v, states %s %s", sc, c.sessionState(a1), c.sessionState(r1))
	}
	rows := c.Allows("host.connect")
	if len(rows) != allows+1 || !hasDetail(rows[len(rows)-1], "changed", "is_server_host") {
		t.Errorf("audit %+v", rows)
	}
	if c.PermissionVersion() != pv {
		t.Error("the server-host flag raised the rights version")
	}
}

// TestReport_Invalid (M2-43 Design note 8): each rule of ValidateReport answers error with
// validation_failed, stores nothing and keeps the connection.
func TestReport_Invalid(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl, _ := c.login(c.h1)
	long := strings.Repeat("a", terminalworld.HostConfig.OSUserMaxChars+1)
	label := strings.Repeat("x", terminalworld.HostConfig.ReportLabelMaxChars+1)
	cases := map[string]func(map[string]any){
		"name too long in session_users": func(r map[string]any) {
			r["session_users"] = []any{map[string]any{"os_user": long, "workdirs": []any{}}}
		},
		"name too long in doctor": func(r map[string]any) {
			r["doctor"] = checks(map[string]any{"check": "hd6_llm_login", "ok": false, "session_user": long})
		},
		"label too long": func(r map[string]any) { r["os_version"] = label },
		"too many profiles": func(r map[string]any) {
			var list []any
			for i := range terminalworld.HostConfig.ReportMaxEntries + 1 {
				list = append(list, map[string]any{"name": "p" + string(rune('a'+i)), "display_name": "P", "kind": "command"})
			}
			r["profiles"] = list
		},
		"check missing": func(r map[string]any) { r["doctor"] = checks()[1:] },
		"name on a passed check": func(r map[string]any) {
			r["doctor"] = checks(map[string]any{"check": "hd1_user", "ok": true, "session_user": "bothy-s-alex"})
		},
		"names equal without case": func(r map[string]any) {
			r["session_users"] = []any{map[string]any{"os_user": "bothy-s-alex", "workdirs": []any{}}, map[string]any{"os_user": "BOTHY-S-ALEX", "workdirs": []any{}}}
		},
	}
	version := c.HostVersion(c.h1)
	for name, mod := range cases {
		r := report("bothy-s-alex")
		mod(r)
		c.send(cl, "report", r)
		var e rest.GatewayError
		cl.NextOp(c.ctx(), "error", &e)
		if e.Op != "report" || e.Code != "validation_failed" {
			t.Errorf("%s: %+v", name, e)
		}
	}
	c.sync(cl)
	if c.HostVersion(c.h1) != version || c.hc.Links.Connected(c.h1) {
		t.Error("an invalid report was stored or connected the host")
	}
}

// TestRefusedStartReport_DWW91 (M2-43, D-W-91, T-TM-14): identify, a report with HD6 failing for
// two names, close 1000: reported_doctor holds both names, one audit entry, presence true then
// false, exactly one login of H1.
func TestRefusedStartReport_DWW91(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl, _ := c.login(c.h1)
	r := report("bothy-s-alex", "bothy-s-robin", "bothy-s-sam")
	r["doctor"] = checks(
		map[string]any{"check": "hd6_llm_login", "ok": false, "code": "subscription_login_detected", "session_user": "bothy-s-alex"},
		map[string]any{"check": "hd6_llm_login", "ok": false, "code": "subscription_login_detected", "session_user": "bothy-s-sam"},
	)
	c.send(cl, "report", r)
	c.sync(cl)
	if err := cl.Close(); err != nil {
		t.Fatal(err)
	}
	c.eventually("presence false", func() bool { return slices.Equal(c.pub.presence(c.h1), []bool{true, false}) })
	stored := c.Text(`SELECT reported_doctor::text FROM hosts WHERE id = $1`, c.h1.String())
	if !strings.Contains(stored, "bothy-s-alex") || !strings.Contains(stored, "bothy-s-sam") {
		t.Errorf("reported_doctor %s", stored)
	}
	rows := c.Allows("host.connect")
	if len(rows) != 1 || !hasDetail(rows[0], "hd6_users", `["bothy-s-alex","bothy-s-sam"]`) {
		t.Errorf("audit %+v", rows)
	}
}

// TestSessionStartedEnded and TestSessionEnded_HostReasons (M2-43 Design note 10): started turns
// an opening session active; every accepted end reason and detail rule; access_revoked from the
// host is validation_failed without a change; an unknown session gets not_found and
// session.close.
func TestSessionStartedEnded(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	o := c.TerminalSession(c.h1, c.Alex, c.alexSU, "opening")
	cl, _ := c.login(c.h1, o)
	c.send(cl, "session.started", map[string]any{"session_id": o.String()})
	c.sync(cl)
	if s := c.sessionState(o); s != "active" {
		t.Fatalf("started: %s", s)
	}
	c.send(cl, "session.ended", map[string]any{"session_id": o.String(), "close_reason": "process_exited", "exit_code": 3})
	c.sync(cl)
	if s := c.sessionState(o); s != "closed:process_exited" || c.Count(`SELECT exit_code FROM terminal_sessions WHERE id = $1`, o.String()) != 3 {
		t.Errorf("ended: %s", s)
	}
	unknown := ids.New()
	c.send(cl, "session.started", map[string]any{"session_id": unknown.String()})
	var e rest.GatewayError
	cl.NextOp(c.ctx(), "error", &e)
	var sc rest.HostSessionClose
	cl.NextOp(c.ctx(), "session.close", &sc)
	if e.Code != "not_found" || ids.ID(sc.SessionId) != unknown || sc.CloseReason != "access_revoked" {
		t.Errorf("unknown session: %+v %+v", e, sc)
	}
}

func TestSessionEnded_HostReasons(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl, _ := c.login(c.h1)
	good := []map[string]any{
		{"close_reason": "client_closed"}, {"close_reason": "idle_timeout"}, {"close_reason": "detach_expired"},
		{"close_reason": "open_failed", "close_detail": "pty_failed"},
		{"close_reason": "host_policy_violation", "close_detail": "helper_untrusted"},
	}
	for _, g := range good {
		id := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
		g["session_id"] = id.String()
		c.send(cl, "session.ended", g)
		c.sync(cl)
		if s := c.sessionState(id); s != "closed:"+g["close_reason"].(string) {
			t.Errorf("%v: %s", g, s)
		}
	}
	open := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
	for _, b := range []map[string]any{
		{"close_reason": "access_revoked"}, {"close_reason": "open_failed"},
		{"close_reason": "host_policy_violation", "close_detail": "pty_failed"}, {"close_reason": "client_closed", "exit_code": 1},
	} {
		b["session_id"] = open.String()
		c.send(cl, "session.ended", b)
		var e rest.GatewayError
		cl.NextOp(c.ctx(), "error", &e)
		if e.Code != "validation_failed" || e.Op != "session.ended" {
			t.Errorf("%v: %+v", b, e)
		}
	}
	if s := c.sessionState(open); s != "active" {
		t.Errorf("refused ends changed the session: %s", s)
	}
}

// TestHeartbeatMissCloses4009 (M2-43 Design note 9): without heartbeats the server closes with
// 4009 after interval times (miss limit + 1) and the active session becomes detached.
func TestHeartbeatMissCloses4009(t *testing.T) {
	t.Parallel()
	cfg := hostCfg
	cfg.HeartbeatInterval, cfg.HeartbeatMissLimit = 150*time.Millisecond, 1
	c := newCtlWith(t, cfg)
	ses := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
	cl, _ := c.login(c.h1, ses)
	start := time.Now()
	c.expectClose(cl, 4009)
	if d := time.Since(start); d < 2*cfg.HeartbeatInterval-50*time.Millisecond {
		t.Errorf("closed after %v, before the deadline", d)
	}
	c.eventually("detached", func() bool { return c.sessionState(ses) == "detached" })
}

// TestHeartbeatAckCarriesNewJWKS (M2-43 Design note 9, 24 section 5.3): after a new key of purpose
// terminal is published, the next heartbeat_ack carries the set with it, the one after no jwks.
func TestHeartbeatAckCarriesNewJWKS(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl, _ := c.login(c.h1)
	ack := func() rest.HostHeartbeatAck {
		c.send(cl, "heartbeat", map[string]any{"active_sessions": 0})
		var a rest.HostHeartbeatAck
		cl.NextOp(c.ctx(), "heartbeat_ack", &a)
		return a
	}
	if a := ack(); a.Jwks != nil {
		t.Fatalf("unchanged set sent: %+v", a.Jwks)
	}
	c.keys.add("terminal-key-2", instance.KeyNext)
	if a := ack(); a.Jwks == nil || len(a.Jwks.Keys) != 2 {
		t.Fatalf("new set: %+v", a.Jwks)
	}
	if a := ack(); a.Jwks != nil {
		t.Fatal("the set was sent twice")
	}
}

// TestLastSeenGranularity (M2-43 Design note 9): a heartbeat writes last_seen_at only when it is
// older than identity.last_active_granularity, without a new version.
func TestLastSeenGranularity(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl, _ := c.login(c.h1)
	seen := c.Text(`SELECT last_seen_at::text FROM hosts WHERE id = $1`, c.h1.String())
	version := c.HostVersion(c.h1)
	c.Clk.Advance(time.Minute)
	c.sync(cl)
	if got := c.Text(`SELECT last_seen_at::text FROM hosts WHERE id = $1`, c.h1.String()); got != seen {
		t.Errorf("written within the granularity: %s", got)
	}
	c.Clk.Advance(terminalworld.HostConfig.LastActiveGranularity)
	c.sync(cl)
	if got := c.Text(`SELECT last_seen_at::text FROM hosts WHERE id = $1`, c.h1.String()); got == seen || c.HostVersion(c.h1) != version {
		t.Errorf("last_seen_at %s, version %d", got, c.HostVersion(c.h1))
	}
}

// TestControlLossDetachesSessions (M2-43 Design note 11): Alex's active session becomes detached
// and the presence turns false when the connection drops.
func TestControlLossDetachesSessions(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	ses := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
	cl, _ := c.login(c.h1, ses)
	c.send(cl, "report", report("bothy-s-alex", "bothy-s-robin", "bothy-s-sam"))
	c.sync(cl)
	if err := cl.Close(); err != nil {
		t.Fatal(err)
	}
	c.eventually("detached", func() bool { return c.sessionState(ses) == "detached" })
	c.eventually("presence false", func() bool { return slices.Equal(c.pub.presence(c.h1), []bool{true, false}) })
}

// TestShutdownCloses4013 (M2-43 Design note 11, 02 section 15.3 step 3): the stop step detaches
// the sessions and closes every connection with 4013.
func TestShutdownCloses4013(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	ses := c.TerminalSession(c.h1, c.Alex, c.alexSU, "active")
	cl, _ := c.login(c.h1, ses)
	ctx, cancel := context.WithTimeout(context.Background(), waitLong)
	defer cancel()
	go func() { _ = c.hc.StopConnections().Run(ctx) }()
	c.expectClose(cl, 4013)
	c.eventually("detached", func() bool { return c.sessionState(ses) == "detached" })
}

// TestNoSecretsInLogsAuditOrEvents (M2-43, 24 section 10.4, SEC-LOG-2): the proof, the challenge
// and a canary in os_version never appear in captured logs or audit details; the canary only in
// hosts and host.updated.
func TestNoSecretsInLogsAuditOrEvents(t *testing.T) {
	t.Parallel()
	c := newCtl(t)
	cl := c.dial()
	h := cl.Hello()
	proof := hosttest.Proof(c.key(c.h1), c.h1, h.Challenge, h.ServerTime)
	cl.Login(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: c.key(c.h1), Proof: proof})
	const canary = "Canary OS 7"
	r := report("bothy-s-alex")
	r["os_version"] = canary
	c.send(cl, "report", r)
	c.sync(cl)
	bad := c.dial()
	if err := bad.Identify(c.ctx(), hosttest.IdentifyParams{HostID: c.h1, Key: keyFrom("thief")}); err != nil {
		t.Fatal(err)
	}
	c.expectClose(bad, 4110)
	for _, s := range []string{proof, h.Challenge, strings.Split(proof, ".")[2]} {
		c.AssertNowhere(s, "proof or challenge")
	}
	if n := c.Count(`SELECT count(*) FROM audit_events WHERE details::text LIKE '%' || $1 || '%'`, canary); n != 0 {
		t.Error("the canary reached an audit entry")
	}
	if n := c.Count(`SELECT count(*) FROM hosts WHERE reported_os_version = $1`, canary); n != 1 {
		t.Error("the canary was not stored")
	}
	found := false
	for _, ev := range c.Events("host.updated") {
		found = found || strings.Contains(ev.Raw, canary)
	}
	if !found {
		t.Error("the canary is not in host.updated")
	}
	for _, rec := range c.Logs.Records() {
		if strings.Contains(rec.Message, canary) {
			t.Error("the canary reached a log line")
		}
		for _, v := range rec.Attrs {
			if strings.Contains(v, canary) {
				t.Error("the canary reached a log attribute")
			}
		}
	}
}
