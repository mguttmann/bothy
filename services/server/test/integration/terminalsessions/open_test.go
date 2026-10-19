// SPDX-License-Identifier: Apache-2.0

package terminalsessions

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/test/harness"
	"github.com/OWNER/bothy/services/server/test/terminal/fakehost"
)

// TestOpen_HappyPathShell (24 section 6.1, TestToken_ClaimsMatchSpec, TestToken_OwnClaim): Alex
// on D1 with a fresh step-up opens shell; the grant, the row, the consumed step-up, one allow entry
// and one event to Alex, exactly one session.open with expires_at; the token verifies against the
// key set H1 got and carries the claims of 24 section 5.3; with H1 on Accept the session becomes
// active after the start frame.
func TestOpen_HappyPathShell(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	before := time.Now()
	g := w.opened(w.alex, w.r1, openBody("shell"))
	sid := g.Session.ID
	if g.Session.State != "opening" || g.StreamPath != "/api/v1/terminal/sessions/"+sid+"/stream" ||
		g.NoiseProtocol != "Noise_NK_25519_ChaChaPoly_SHA256" || g.PrologueVersion != "bothy-terminal/1" || g.KeyGeneration != 1 ||
		g.NoisePublicKey == "" || g.KeyFingerprint == "" {
		t.Fatalf("grant %+v", g)
	}
	row := w.text(`SELECT host_session_user_id::text || '|' || opened_by_device_id::text || '|' || device_id::text || '|' ||
		auth_session_id::text || '|' || token_jti::text FROM terminal_sessions WHERE id = $1`, sid)
	su := w.text(`SELECT id::text FROM host_session_users WHERE host_id = $1 AND os_user = 'bothy-s-alex'`, w.r1.String())
	_, claims, order := tokenClaims(t, g.SessionToken)
	if want := strings.Join([]string{su, w.alex.DeviceID, w.alex.DeviceID, w.alex.SessionID, claims["jti"].(string)}, "|"); row != want {
		t.Fatalf("row %s, want %s", row, want)
	}
	if w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND step_up_consumed_at IS NOT NULL`, w.alex.SessionID) != 1 {
		t.Fatal("the step-up is not consumed")
	}
	if n := len(w.audits("terminal.open")); n != 1 {
		t.Fatalf("%d allow entries terminal.open", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'terminal_session.changed' AND resource_id = $1 AND audience_principal_id = $2`,
		sid, w.alex.PrincipalID); n != 1 {
		t.Fatalf("%d events to Alex", n)
	}
	ctx := w.ctx()
	if err := w.h1.WaitOpens(ctx, ids.MustParse(sid), 1); err != nil {
		t.Fatal(err)
	}
	opens := w.h1.Opens()
	if len(opens) != 1 || opens[0].DataPath != "/api/v1/hosts/sessions/"+sid+"/stream" || opens[0].Profile != "shell" ||
		opens[0].ExpiresAt.Before(before.Add(19*time.Second)) || opens[0].ExpiresAt.After(time.Now().Add(21*time.Second)) {
		t.Fatalf("session.open %+v", opens)
	}
	// TestToken_ClaimsMatchSpec: the claims in the order of 24 section 5.3, the values of note 5.
	want := []string{"iss", "aud", "sub", "sid", "jti", "iat", "exp", "dev", "prf", "cek", "own", "osu", "att"}
	if !slices.Equal(order, want) {
		t.Fatalf("claims %v", order)
	}
	if claims["aud"] != w.r1.String() || claims["sub"] != w.alex.PrincipalID || claims["sid"] != sid || claims["dev"] != w.alex.DeviceID ||
		claims["prf"] != "shell" || claims["cek"] != cek || claims["osu"] != "bothy-s-alex" || claims["att"] != "open" || claims["own"] != false ||
		claims["exp"].(float64)-claims["iat"].(float64) != 30 {
		t.Fatalf("claims %v", claims)
	}
	header, _, _ := tokenClaims(t, g.SessionToken)
	kid := w.text(`SELECT kid FROM server_signing_keys WHERE purpose = 'terminal' AND state = 'active'`)
	if header["alg"] != "EdDSA" || header["typ"] != "terminal-session+jwt" || header["kid"] != kid {
		t.Fatalf("header %v (active kid %s)", header, kid)
	}
	if err := w.h1.VerifyToken(g.SessionToken, ids.MustParse(sid)); err != nil {
		t.Fatalf("the token does not verify with the key set of hello: %v", err)
	}
	// With H1 on Accept the session becomes active after the start frame.
	w.h1.Default(fakehost.Script{Kind: fakehost.Accept})
	g2 := w.opened(w.alex, w.r1, openBody("shell"))
	d := w.device(w.alex, g2.Session.ID, fakehost.DeviceParams{})
	w.handshake(d, g2.SessionToken)
	w.waitState(g2.Session.ID, "active")
	if w.count(`SELECT count(*) FROM terminal_sessions WHERE id = $1 AND started_at IS NOT NULL`, g2.Session.ID) != 1 {
		t.Fatal("started_at not set")
	}
	// TestToken_OwnClaim: Robin, an owner, gets own true.
	r := w.opened(w.robin, w.r1, openBody("shell"))
	_, rc, _ := tokenClaims(t, r.SessionToken)
	if rc["own"] != true || rc["osu"] != "bothy-s-robin" {
		t.Fatalf("Robin's claims %v", rc)
	}
	_ = http.StatusOK
}

// TestOpen_RevocationRaceRecheck (Design note 12): Robin revokes Alex's grant terminal on R1
// between the decision and the commit of her opening; the recheck after the commit ends the new
// session with access_revoked, writes terminal.terminate, sends no session.open and answers 403
// forbidden.
func TestOpen_RevocationRaceRecheck(t *testing.T) {
	t.Parallel()
	w := newWorld(t, nil)
	revoke := func() {
		harness.MustDo(t, w.robin, harness.Request{Method: http.MethodPost, Path: "/grants/" + w.alexGrant + "/revoke",
			Body: map[string]any{"reason": "terminal test"}}, http.StatusOK)
	}
	w.beforeOpenCommit.Store(&revoke)
	r := w.open(w.alex, w.r1, openBody("shell"))
	// Only status and problem code: a 201 body would carry the session token.
	if code, _ := problem(r); r.Status != http.StatusForbidden || code != "forbidden" {
		t.Fatalf("opening during a revocation: %d %q, want 403 forbidden", r.Status, code)
	}
	if w.beforeOpenCommit.Load() != nil {
		t.Fatal("the seam did not run")
	}
	sid := w.text(`SELECT id::text FROM terminal_sessions WHERE user_id = $1`, w.alex.PrincipalID)
	if got := w.state(sid); got != "closed:access_revoked" {
		t.Fatalf("state %s", got)
	}
	found := false
	for _, e := range w.audits("terminal.terminate") {
		if e["session_id"] == sid && e["close_reason"] == "access_revoked" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no terminal.terminate entry for %s", sid)
	}
	if n := w.h1.OpensOf(ids.MustParse(sid)); n != 0 {
		t.Fatalf("%d session.open", n)
	}
}
