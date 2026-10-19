// SPDX-License-Identifier: Apache-2.0

package terminalsessions

import (
	"net/http"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// TestAttach_TMT12 (TM-T12 without the console part) with TestToken_NewJtiPerGrant and
// TestAttach_KeepsOpenedAtAndOsu: after D1's stream drops, D2 without step-up gets 401; D1 in S1
// within terminal.quick_reattach_window attaches without consuming a step-up and gets att attach;
// D1 after the window gets 401; D1 while the session is active gets 401; D2 with a fresh step-up
// attaches. Every grant has a new jti, the last one is token_jti; opened_at,
// host_session_user_id and key_generation never change.
func TestAttach_TMT12(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	g, d := w.started(w.alex)
	sid := g.Session.ID
	fixed := w.text(`SELECT opened_at::text || host_session_user_id::text || key_generation::text FROM terminal_sessions WHERE id = $1`, sid)
	_ = d.Close()
	w.waitState(sid, "detached")
	expect(t, "D2 without step-up", w.attach(w.alex2, sid, false), http.StatusUnauthorized, "step_up_required", "")
	consumed := w.text(`SELECT coalesce(step_up_consumed_at::text, '') FROM auth_sessions WHERE id = $1`, w.alex.SessionID)
	r := w.attach(w.alex, sid, false)
	if r.Status != http.StatusOK {
		t.Fatalf("quick reattach: %d %s", r.Status, r.Body)
	}
	if now := w.text(`SELECT coalesce(step_up_consumed_at::text, '') FROM auth_sessions WHERE id = $1`, w.alex.SessionID); now != consumed {
		t.Fatal("the quick reattach consumed a step-up")
	}
	var g2 grantBody
	r.JSON(t, &g2)
	_, c2, _ := tokenClaims(t, g2.SessionToken)
	if c2["att"] != "attach" {
		t.Fatalf("claims %v", c2)
	}
	d2 := w.device(w.alex, sid, fakehost.DeviceParams{})
	w.handshake(d2, g2.SessionToken)
	w.waitState(sid, "active")
	expect(t, "D1 while active", w.attach(w.alex, sid, false), http.StatusUnauthorized, "step_up_required", "")
	_ = d2.Close()
	w.waitState(sid, "detached")
	w.exec(`UPDATE terminal_sessions SET detached_at = now() - interval '5 minutes' WHERE id = $1`, sid)
	expect(t, "D1 after the window", w.attach(w.alex, sid, false), http.StatusUnauthorized, "step_up_required", "")
	r = w.attach(w.alex2, sid, true)
	if r.Status != http.StatusOK {
		t.Fatalf("D2 with step-up: %d %s", r.Status, r.Body)
	}
	var g3 grantBody
	r.JSON(t, &g3)
	_, c1, _ := tokenClaims(t, g.SessionToken)
	_, c3, _ := tokenClaims(t, g3.SessionToken)
	if c1["jti"] == c2["jti"] || c2["jti"] == c3["jti"] || c1["jti"] == c3["jti"] {
		t.Fatal("a jti repeats")
	}
	if got := w.text(`SELECT token_jti::text FROM terminal_sessions WHERE id = $1`, sid); got != c3["jti"] {
		t.Fatalf("token_jti %s, want %s", got, c3["jti"])
	}
	if got := w.text(`SELECT opened_at::text || host_session_user_id::text || key_generation::text FROM terminal_sessions WHERE id = $1`, sid); got != fixed {
		t.Fatal("opened_at, host_session_user_id or key_generation changed")
	}
	if c3["osu"] != "bothy-s-alex" || c3["dev"] != w.alex2.DeviceID {
		t.Fatalf("claims %v", c3)
	}
	_ = ctx
}

// TestAttach_SecondDeviceCloses4103: D1 streams, Alex attaches from D2 with a fresh step-up: D1's
// stream gets 4103, H1's old data connection 1000, H1 gets a new session.open, D2 pairs and the
// session is active with device_id D2; a later identify from D1 gets 4103.
func TestAttach_SecondDeviceCloses4103(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	g, d1 := w.started(w.alex)
	sid := ids.MustParse(g.Session.ID)
	old := w.h1.Data(sid)
	r := w.attach(w.alex2, g.Session.ID, true)
	if r.Status != http.StatusOK {
		t.Fatalf("attach from D2: %d %s", r.Status, r.Body)
	}
	var g2 grantBody
	r.JSON(t, &g2)
	w.closeCode("D1", func() (int, string) { return d1.CloseCode(ctx) }, 4103, "")
	w.closeCode("old data connection", func() (int, string) { return old.CloseCode(ctx) }, 1000, "")
	if err := w.h1.WaitOpens(ctx, sid, 2); err != nil {
		t.Fatal(err)
	}
	w.eventually("the new data connection", func() bool { return w.h1.Data(sid) != old })
	d2 := w.device(w.alex2, g.Session.ID, fakehost.DeviceParams{})
	if err := d2.Send(ctx, []byte(g2.SessionToken)); err != nil {
		t.Fatal(err)
	}
	if m, err := d2.Recv(ctx); err != nil || string(m) != "message-2" {
		t.Fatalf("message 2: %q %v", m, err)
	}
	w.waitState(g.Session.ID, "active")
	if got := w.text(`SELECT device_id::text FROM terminal_sessions WHERE id = $1`, g.Session.ID); got != w.alex2.DeviceID {
		t.Fatalf("device_id %s", got)
	}
	again := w.device(w.alex, g.Session.ID, fakehost.DeviceParams{})
	w.closeCode("D1 again", func() (int, string) { return again.CloseCode(ctx) }, 4103, "")
}

// TestAttach_OpeningSession (Design note 6 d): D1 drops before message 1 of a claude opening with
// index 1; the attach with a fresh step-up gives att open and wdi 1.
func TestAttach_OpeningSession(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	ctx := w.ctx()
	w.h1.Default(fakehost.Script{Kind: fakehost.Ignore})
	g := w.opened(w.alex, w.r1, openBody("claude", 1))
	d := w.device(w.alex, g.Session.ID, fakehost.DeviceParams{})
	if err := d.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()
	r := w.attach(w.alex, g.Session.ID, true)
	if r.Status != http.StatusOK {
		t.Fatalf("attach: %d %s", r.Status, r.Body)
	}
	var g2 grantBody
	r.JSON(t, &g2)
	_, c, _ := tokenClaims(t, g2.SessionToken)
	if c["att"] != "open" || c["wdi"] != float64(1) {
		t.Fatalf("claims %v", c)
	}
	if w.state(g.Session.ID) != "opening" {
		t.Fatalf("state %s", w.state(g.Session.ID))
	}
}
