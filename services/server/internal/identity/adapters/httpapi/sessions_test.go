// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/postgres"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/dpop"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/session"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/testkit"
)

const mySessions = "/api/v1/me/sessions"

// TestAuthenticate_Valid: the Authenticator fills the actor from token, session, device and key.
func TestAuthenticate_Valid(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	c := w.login(alex, identity.DeviceStateActive)
	htu := httpservertest.PublicURL + mySessions
	a, err := w.id.Authenticator.Authenticate(context.Background(), actor.Credentials{
		Method: http.MethodGet, HTU: htu, Authorization: "DPoP " + c.access,
		DPoPProofs: []string{c.proof(c.key, http.MethodGet, mySessions, c.access)},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := actor.Actor{ID: alex, Kind: actor.KindUser, SessionID: c.session, SessionActive: true, DeviceID: c.device,
		DeviceState: actor.DeviceActive, DeviceKeyActive: true, KeyThumbprint: c.key.Thumbprint, AuthMethod: "passkey", ClientComponent: "app"}
	if fmt.Sprint(a) != fmt.Sprint(want) {
		t.Fatalf("actor\n got %+v\nwant %+v", a, want)
	}
	if err := w.id.Authenticator.Revalidate(context.Background(), a); err != nil {
		t.Fatalf("Revalidate: %v", err)
	}
	r := w.want(c.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	if items := r["items"].([]any); len(items) != 1 || items[0].(map[string]any)["current"] != true {
		t.Fatalf("own sessions %v", r)
	}
	// No credentials: anonymous; an unknown scheme or two Authorization headers: unauthenticated.
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "")), http.StatusUnauthorized, "unauthenticated")
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "Basic YWxleDpw")), http.StatusUnauthorized, "unauthenticated")
	req := w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "DPoP "+c.access, "DPoP", c.proof(c.key, http.MethodGet, mySessions, c.access))
	req.Header.Add("Authorization", "DPoP "+c.access)
	res := w.h.Do(req)
	w.want(res, http.StatusUnauthorized, "unauthenticated")
	if !strings.HasPrefix(res.Header.Get("WWW-Authenticate"), "DPoP") {
		t.Errorf("challenge %q", res.Header.Get("WWW-Authenticate"))
	}
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "Bearer bothy_agt_x")), http.StatusUnauthorized, "unauthenticated")
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "DPoP not-a-token", "DPoP", "x")), http.StatusUnauthorized, "unauthenticated")
}

// TestAuthenticate_IDT14_BearerRejected: an access token sent as Bearer is 401
// invalid_dpop_proof (RFC 9449 section 7.2).
func TestAuthenticate_IDT14_BearerRejected(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	r := w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "Bearer "+c.access))
	w.want(r, http.StatusUnauthorized, "invalid_dpop_proof")
	if !strings.Contains(r.Header.Get("WWW-Authenticate"), `error="invalid_dpop_proof"`) || r.Header.Get("DPoP-Nonce") == "" {
		t.Fatalf("headers %v", r.Header)
	}
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "Bearer opaque")), http.StatusUnauthorized, "unauthenticated")
}

// TestAuthenticate_ProofOtherKey and the ath mismatch: 401 invalid_dpop_proof.
func TestAuthenticate_ProofOtherKey(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	other, _ := identitytest.NewDeviceKey(jose.AlgEdDSA)
	r := w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "DPoP "+c.access, "DPoP", c.proof(other, http.MethodGet, mySessions, c.access)))
	w.want(r, http.StatusUnauthorized, "invalid_dpop_proof")
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "DPoP "+c.access)), http.StatusUnauthorized, "invalid_dpop_proof")
}

func TestAuthenticate_AthMismatch(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	d := w.login(w.person("sam", false), identity.DeviceStateActive)
	r := w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "DPoP "+c.access, "DPoP", c.proof(c.key, http.MethodGet, mySessions, d.access)))
	w.want(r, http.StatusUnauthorized, "invalid_dpop_proof")
}

// TestAuthenticate_ExpiryLeeway (request part): beyond exp plus auth.clock_leeway the token is 401
// token_expired.
func TestAuthenticate_ExpiryLeeway(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	w.clk.Advance(10*time.Minute + 20*time.Second)
	w.want(c.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	w.clk.Advance(20 * time.Second)
	r := c.call(http.MethodGet, mySessions, "")
	w.want(r, http.StatusUnauthorized, "token_expired")
	if r.Header.Get("WWW-Authenticate") != `DPoP error="invalid_token", algs="ES256 EdDSA"` {
		t.Fatalf("challenge %q", r.Header.Get("WWW-Authenticate"))
	}
}

// TestAuthenticate_RevokedSessionBeforeExpiry: after RevokeSession the still valid token gets 401
// session_revoked from authorize, with an audit entry of reason session_revoked (AZ-M41).
func TestAuthenticate_RevokedSessionBeforeExpiry(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	c := w.login(alex, identity.DeviceStateActive)
	w.want(c.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	if err := w.id.Sessions.RevokeSession(context.Background(), c.session, identity.RevokeAdmin, nil); err != nil {
		t.Fatal(err)
	}
	w.want(c.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	rows := w.audits("session.list")
	if len(rows) != 1 || rows[0].Decision != "deny" || rows[0].Reason != "session_revoked" {
		t.Fatalf("audit %+v", rows)
	}
	// A revoked device key or device ends the access the same way.
	d := w.login(alex, identity.DeviceStateActive)
	w.sql(`UPDATE device_keys SET revoked_at = now() WHERE device_id = $1`, d.device.String())
	w.want(d.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	e := w.login(alex, identity.DeviceStateActive)
	w.sql(`UPDATE devices SET state = 'revoked', revoked_at = now(), revoke_reason = 'revoked_by_self' WHERE id = $1`, e.device.String())
	w.want(e.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	if err := w.id.Authenticator.Revalidate(context.Background(), actor.Actor{ID: alex, Kind: actor.KindUser, SessionID: c.session,
		DeviceID: c.device, KeyThumbprint: c.key.Thumbprint}); err == nil {
		t.Fatal("Revalidate accepts a revoked session")
	}
}

// TestAuthenticate_RevocationAcrossInstances: two server instances on one database; a logout
// through instance A takes effect on instance B at the next request (AZ-P9 for sessions).
func TestAuthenticate_RevocationAcrossInstances(t *testing.T) {
	t.Parallel()
	a := newSessionWorld(t)
	b := newSessionWorldOn(t, a.db, a.clk, a.instID, a.dir)
	alex := a.person("alex", false)
	c := a.login(alex, identity.DeviceStateActive)
	onB := *c
	onB.w = b
	b.want(onB.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	a.want(c.call(http.MethodPost, "/api/v1/auth/logout", ""), http.StatusNoContent, "")
	b.want(onB.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
}

// TestAuthenticate_StaleNonceGetsFreshNonce: a proof with the previous nonce passes and the
// response carries the current one (06 section 3.1 item 2).
func TestAuthenticate_StaleNonceGetsFreshNonce(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	old := w.id.DPoP.CurrentNonce()
	w.clk.Advance(sessCfg.DPoPNonceTTL)
	proof := identitytest.SignDPoPProof(c.key, identitytest.ProofClaims{HTM: http.MethodGet, HTU: httpservertest.PublicURL + mySessions,
		IAT: w.clk.Now(), Nonce: old, ATH: dpop.ATH(c.access)})
	r := w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "DPoP "+c.access, "DPoP", proof))
	w.want(r, http.StatusOK, "")
	if got := r.Header.Get("DPoP-Nonce"); got == "" || got == old || got != w.id.DPoP.CurrentNonce() {
		t.Fatalf("DPoP-Nonce %q, old %q", got, old)
	}
	if r := c.call(http.MethodGet, mySessions, ""); r.Header.Get("DPoP-Nonce") != "" {
		t.Fatalf("fresh proof got DPoP-Nonce %q", r.Header.Get("DPoP-Nonce"))
	}
}

// TestAuthenticate_LastSeenGranularity: at most one activity write per
// identity.last_active_granularity, on session and device.
func TestAuthenticate_LastSeenGranularity(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	seen := func() (time.Time, time.Time, string) {
		var s, d time.Time
		var ip string
		if err := w.owner.QueryRow(context.Background(), `SELECT s.last_seen_at, d.last_seen_at, host(s.last_ip) FROM auth_sessions s
			JOIN devices d ON d.id = s.device_id WHERE s.id = $1`, c.session.String()).Scan(&s, &d, &ip); err != nil {
			t.Fatal(err)
		}
		return s.UTC(), d.UTC(), ip
	}
	w.want(c.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	s1, d1, ip := seen()
	if !s1.Equal(sessStart) || !d1.Equal(sessStart) || ip == "" {
		t.Fatalf("first write %v %v %q", s1, d1, ip)
	}
	w.clk.Advance(4 * time.Minute)
	w.want(c.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	if s2, d2, _ := seen(); !s2.Equal(s1) || !d2.Equal(d1) {
		t.Fatalf("written again within the granularity: %v %v", s2, d2)
	}
	w.clk.Advance(2 * time.Minute)
	w.want(c.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	if s3, d3, _ := seen(); !s3.Equal(w.clk.Now()) || !d3.Equal(w.clk.Now()) {
		t.Fatalf("not written after the granularity: %v %v", s3, d3)
	}
}

// TestRefresh_Rotates: a new pair, the old token used, current_generation + 1.
func TestRefresh_Rotates(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	old := c.refresh
	m := c.rotate()
	if m["token_type"] != "DPoP" || m["refresh_token"] == old || len(m["refresh_token"].(string)) != 43 {
		t.Fatalf("response %v", m)
	}
	if s := m["session"].(map[string]any); s["id"] != c.session.String() || s["current"] != nil {
		t.Fatalf("session %v", s)
	}
	if _, gen := w.familyState(c.session); gen != 1 {
		t.Fatalf("generation %d", gen)
	}
	if n := w.count(`SELECT count(*) FROM refresh_tokens t JOIN refresh_token_families f ON f.id = t.family_id
		WHERE f.session_id = $1 AND t.generation = 0 AND t.used_at IS NOT NULL`, c.session.String()); n != 1 {
		t.Fatalf("old token used_at not set (%d)", n)
	}
	w.want(c.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
}

// TestRefresh_IDT15_ReuseRevokes: reuse of a rotated token after the grace revokes family,
// session and children, writes one session.revoked per session, the audit deny and answers 401
// session_revoked; the old access token is dead too.
func TestRefresh_IDT15_ReuseRevokes(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	c := w.login(alex, identity.DeviceStateActive)
	kid := c.child()
	old := c.refresh
	c.rotate()
	w.clk.Advance(sessCfg.RefreshReuseGrace)
	w.want(c.refreshWith(old, c.key), http.StatusUnauthorized, "session_revoked")
	if r, _ := w.familyState(c.session); r != "reuse_detected" {
		t.Errorf("family %q", r)
	}
	if r, _ := w.familyState(kid.session); r != "session_revoked" {
		t.Errorf("child family %q", r)
	}
	if w.sessionState(c.session) != "refresh_reuse" || w.sessionState(kid.session) != "refresh_reuse" {
		t.Errorf("sessions %q %q", w.sessionState(c.session), w.sessionState(kid.session))
	}
	evs := w.revokedEvents()
	if len(evs) != 2 {
		t.Fatalf("events %v", evs)
	}
	for _, e := range evs {
		if e["audience"] != alex.String() || e["actor"] != "" || e["receive"] != "self" || !strings.Contains(e["payload"], `"reason": "refresh_reuse"`) {
			t.Errorf("event %v", e)
		}
	}
	rows := w.audits("auth.token.refresh")
	if len(rows) != 1 || rows[0].Decision != "deny" || rows[0].Reason != "session_revoked" ||
		rows[0].Details["cause"] != "refresh_reuse" || rows[0].Details["family_id"] == nil {
		t.Fatalf("audit %+v", rows)
	}
	if len(*w.ended) != 1 || (*w.ended)[0].Reason != identity.RevokeRefreshReuse || len((*w.ended)[0].SessionIDs) != 2 {
		t.Fatalf("SessionEnded %+v", *w.ended)
	}
	w.want(c.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	w.want(c.refreshWith(c.refresh, c.key), http.StatusUnauthorized, "session_revoked")
}

// TestRefresh_GraceSameKey: the previous generation within the grace with the family key gives a
// new pair; after the grace it is reuse.
func TestRefresh_GraceSameKey(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	old := c.refresh
	c.rotate()
	w.clk.Advance(sessCfg.RefreshReuseGrace - time.Second)
	m := w.want(c.refreshWith(old, c.key), http.StatusOK, "")
	if _, gen := w.familyState(c.session); gen != 2 {
		t.Fatalf("generation %d", gen)
	}
	// The token of generation 1 never arrived; it is used now and the new one is current.
	c.refresh = m["refresh_token"].(string)
	c.rotate()
	w.clk.Advance(sessCfg.RefreshReuseGrace)
	w.want(c.refreshWith(old, c.key), http.StatusUnauthorized, "session_revoked")
	if w.sessionState(c.session) != "refresh_reuse" {
		t.Fatal("not revoked after the grace")
	}
}

// TestRefresh_ConcurrentSameTokenWithinGrace: two parallel refreshes with the same token and key
// both succeed without revocation; the pair returned first is no longer current.
func TestRefresh_ConcurrentSameTokenWithinGrace(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	var wg sync.WaitGroup
	got := make([]*httpservertest.Response, 2)
	for i := range got {
		wg.Go(func() { got[i] = c.refreshWith(c.refresh, c.key) })
	}
	wg.Wait()
	var tokens []string
	for _, r := range got {
		tokens = append(tokens, w.want(r, http.StatusOK, "")["refresh_token"].(string))
	}
	if w.sessionState(c.session) != "" {
		t.Fatal("revoked")
	}
	if _, gen := w.familyState(c.session); gen != 2 {
		t.Fatalf("generation %d", gen)
	}
	if n := w.count(`SELECT count(*) FROM refresh_tokens t JOIN refresh_token_families f ON f.id = t.family_id
		WHERE f.session_id = $1 AND t.used_at IS NULL`, c.session.String()); n != 1 {
		t.Fatalf("%d unused tokens, want 1", n)
	}
	// Exactly one of the two returned tokens is the current generation; the other one, returned
	// first, is marked used and no longer valid.
	var used []bool
	for _, tok := range tokens {
		var u bool
		w.scalar(&u, `SELECT used_at IS NOT NULL FROM refresh_tokens WHERE token_hash = $1`, session.TokenHash(testPepper, tok))
		used = append(used, u)
	}
	if used[0] == used[1] {
		t.Fatalf("used flags of the two returned tokens %v", used)
	}
}

// TestRefresh_IDT16_ParentAndChildConcurrent: app and share extension refresh in parallel many
// times; no reuse detection, separate families.
func TestRefresh_IDT16_ParentAndChildConcurrent(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	kid := c.child()
	for range 8 {
		var wg sync.WaitGroup
		for _, x := range []*client{c, kid} {
			wg.Go(func() {
				r := x.refreshWith(x.refresh, x.key)
				if r.StatusCode != http.StatusOK {
					t.Errorf("status %d: %s", r.StatusCode, r.Body)
					return
				}
				var m map[string]any
				_ = json.Unmarshal(r.Body, &m)
				x.refresh, x.access = m["refresh_token"].(string), m["access_token"].(string)
			})
		}
		wg.Wait()
	}
	if w.sessionState(c.session) != "" || w.sessionState(kid.session) != "" {
		t.Fatal("revoked")
	}
	if n := w.count(`SELECT count(DISTINCT id) FROM refresh_token_families WHERE session_id IN ($1, $2) AND revoked_at IS NULL`,
		c.session.String(), kid.session.String()); n != 2 {
		t.Fatalf("%d active families", n)
	}
}

// TestRefresh_IDT17_OtherKey: a proof of another key is 401 invalid_dpop_proof, the family stays
// and the denial is audited.
func TestRefresh_IDT17_OtherKey(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	other, _ := identitytest.NewDeviceKey(jose.AlgES256)
	r := c.refreshWith(c.refresh, other)
	w.want(r, http.StatusUnauthorized, "invalid_dpop_proof")
	if r.Header.Get("DPoP-Nonce") == "" {
		t.Error("no DPoP-Nonce")
	}
	if reason, gen := w.familyState(c.session); reason != "" || gen != 0 || w.sessionState(c.session) != "" {
		t.Fatalf("family %q %d", reason, gen)
	}
	rows := w.audits("auth.token.refresh")
	if len(rows) != 1 || rows[0].Reason != "unauthenticated" || rows[0].Details["cause"] != "dpop_key_mismatch" {
		t.Fatalf("audit %+v", rows)
	}
	c.rotate()
}

// TestRefresh_MissingProof: 401 invalid_dpop_proof, not 400.
func TestRefresh_MissingProof(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	w.want(w.h.Do(w.h.NewRequest(http.MethodPost, refreshPath, `{"refresh_token":"`+c.refresh+`"}`)), http.StatusUnauthorized, "invalid_dpop_proof")
	w.want(c.refreshWith("unknown-token", c.key), http.StatusUnauthorized, "unauthenticated")
}

// TestRefresh_Expired: an expired refresh token is 401 unauthenticated and changes nothing.
func TestRefresh_Expired(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	if !c.refreshExpiresAt.Equal(sessStart.Add(sessCfg.RefreshTokenTTL)) {
		t.Fatalf("refresh_expires_at %v", c.refreshExpiresAt)
	}
	w.clk.Advance(sessCfg.RefreshTokenTTL)
	w.want(c.refreshWith(c.refresh, c.key), http.StatusUnauthorized, "unauthenticated")
	if reason, gen := w.familyState(c.session); reason != "" || gen != 0 {
		t.Fatalf("family %q %d", reason, gen)
	}
	if rows := w.audits("auth.token.refresh"); len(rows) != 1 || rows[0].Details["cause"] != "refresh_expired" {
		t.Fatalf("audit %+v", rows)
	}
}

// TestRefresh_SessionEnd: refresh expires_at and access exp never lie after the session end; past
// the end the refresh is 401 session_revoked.
func TestRefresh_SessionEnd(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	end := w.clk.Now().Add(3 * time.Minute)
	w.sql(`UPDATE auth_sessions SET expires_at = $2 WHERE id = $1`, c.session.String(), end)
	m := c.rotate()
	exp, err := time.Parse(time.RFC3339, m["refresh_expires_at"].(string))
	if err != nil || !exp.Equal(end) {
		t.Fatalf("refresh_expires_at %v, %v", m["refresh_expires_at"], err)
	}
	if n := m["expires_in"].(float64); n != 180 {
		t.Fatalf("expires_in %v", n)
	}
	w.clk.Advance(3 * time.Minute)
	w.want(c.refreshWith(c.refresh, c.key), http.StatusUnauthorized, "session_revoked")
}

// TestRefresh_PendingDevice: a pending device refreshes like an active one; a disabled person
// cannot.
func TestRefresh_PendingDevice(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	c := w.login(alex, identity.DeviceStatePending)
	c.rotate()
	w.sql(`UPDATE principals SET status = 'disabled', disabled_at = now() WHERE id = $1`, alex.String())
	w.want(c.refreshWith(c.refresh, c.key), http.StatusUnauthorized, "session_revoked")
}

// TestRefresh_TokenNeverLogged: refresh and access tokens and their hashes appear in no log, audit
// detail or event.
func TestRefresh_TokenNeverLogged(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	canaries := []string{c.refresh, c.access}
	old := c.refresh
	c.rotate()
	canaries = append(canaries, c.refresh, c.access)
	other, _ := identitytest.NewDeviceKey(jose.AlgEdDSA)
	c.refreshWith(c.refresh, other)
	c.refreshWith("guess", c.key)
	w.h.Do(w.h.NewRequest(http.MethodGet, mySessions, "", "Authorization", "Bearer "+c.access))
	w.clk.Advance(time.Minute)
	c.refreshWith(old, c.key)
	var all []string
	for _, r := range append(w.logs.Records(), w.h.Logs.Records()...) {
		all = append(all, fmt.Sprint(r))
	}
	for _, r := range w.audits("auth.token.refresh") {
		all = append(all, r.Raw)
	}
	for _, e := range w.revokedEvents() {
		all = append(all, e["payload"])
	}
	var hashes []string
	rows, err := w.owner.Query(context.Background(), `SELECT encode(token_hash, 'hex'), encode(token_hash, 'base64') FROM refresh_tokens`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var hx, b64 string
		if err := rows.Scan(&hx, &b64); err != nil {
			t.Fatal(err)
		}
		hashes = append(hashes, hx, b64)
	}
	rows.Close()
	text := strings.Join(all, "\n")
	for _, s := range append(canaries, hashes...) {
		if strings.Contains(text, s) {
			t.Fatalf("token or hash leaked into logs, audit or events")
		}
	}
	if len(all) == 0 {
		t.Fatal("nothing captured")
	}
}

// TestChildSession_Rules: the child has its own family on the parent key, the parent's end, the
// same device and login path; a second child replaces the first; a child cannot have a child;
// only share_ext is a component.
func TestChildSession_Rules(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	c := w.login(alex, identity.DeviceStateActive)
	kid := c.child()
	var parentEnd, childEnd time.Time
	var parent, comp, method, device string
	if err := w.owner.QueryRow(context.Background(), `SELECT c.expires_at, p.expires_at, c.parent_session_id::text, c.client_component,
		c.auth_method, c.device_id::text FROM auth_sessions c JOIN auth_sessions p ON p.id = c.parent_session_id WHERE c.id = $1`,
		kid.session.String()).Scan(&childEnd, &parentEnd, &parent, &comp, &method, &device); err != nil {
		t.Fatal(err)
	}
	if !childEnd.Equal(parentEnd) || parent != c.session.String() || comp != "share_ext" || method != "passkey" || device != c.device.String() {
		t.Fatalf("child %v %v %s %s %s %s", childEnd, parentEnd, parent, comp, method, device)
	}
	if n := w.count(`SELECT count(DISTINCT dpop_jkt) FROM refresh_token_families WHERE session_id IN ($1, $2)`, c.session.String(), kid.session.String()); n != 1 {
		t.Fatalf("%d keys", n)
	}
	kid.rotate()
	second := c.child()
	if w.sessionState(kid.session) != "logout" || w.sessionState(second.session) != "" {
		t.Fatalf("first child %q, second %q", w.sessionState(kid.session), w.sessionState(second.session))
	}
	if r, _ := w.familyState(kid.session); r != "logout" {
		t.Fatalf("first child family %q", r)
	}
	w.want(kid.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	w.want(second.call(http.MethodPost, "/api/v1/me/sessions/extensions", `{"client_component":"share_ext"}`), http.StatusNotFound, "not_found")
	w.want(c.call(http.MethodPost, "/api/v1/me/sessions/extensions", `{"client_component":"notification_ext"}`), http.StatusBadRequest, "validation_failed")
	if _, err := w.id.Sessions.StartChildSession(context.Background(), c.session, identity.ComponentConsole); err == nil {
		t.Fatal("console child created")
	}
	if rows := w.audits("session.extension.create"); len(rows) != 2 || rows[0].Decision != "allow" {
		t.Fatalf("audit %+v", rows)
	}
	w.want(w.h.Do(w.h.NewRequest(http.MethodPost, "/api/v1/me/sessions/extensions", `{"client_component":"share_ext"}`)), http.StatusUnauthorized, "unauthenticated")
}

// TestRevokeParent_RevokesChildren: revoking the parent revokes its children with the same reason.
func TestRevokeParent_RevokesChildren(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	kid := c.child()
	if err := w.id.Sessions.RevokeSession(context.Background(), c.session, identity.RevokeAdmin, nil); err != nil {
		t.Fatal(err)
	}
	if w.sessionState(c.session) != "admin" || w.sessionState(kid.session) != "admin" {
		t.Fatalf("%q %q", w.sessionState(c.session), w.sessionState(kid.session))
	}
	if r, _ := w.familyState(kid.session); r != "session_revoked" {
		t.Fatalf("child family %q", r)
	}
	// Idempotent: nothing changes, no second event.
	if err := w.id.Sessions.RevokeSession(context.Background(), c.session, identity.RevokeLogout, nil); err != nil {
		t.Fatal(err)
	}
	if w.sessionState(c.session) != "admin" || len(w.revokedEvents()) != 2 {
		t.Fatalf("second revocation changed something: %q, %d events", w.sessionState(c.session), len(w.revokedEvents()))
	}
	if err := w.id.Sessions.RevokeSession(context.Background(), ids.New(), identity.RevokeAdmin, nil); err == nil {
		t.Fatal("unknown session revoked")
	}
	if err := w.id.Sessions.RevokeSession(context.Background(), c.session, "stolen", nil); err == nil {
		t.Fatal("unknown reason accepted")
	}
	w.want(kid.refreshWith(kid.refresh, kid.key), http.StatusUnauthorized, "session_revoked")
}

// TestLogout: session, children and families end with logout, one event per session,
// SessionEnded, audit allow, 204; the same token afterwards is 401 session_revoked; a pending
// device may log out.
func TestLogout(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	c := w.login(alex, identity.DeviceStateActive)
	kid := c.child()
	w.want(c.call(http.MethodPost, "/api/v1/auth/logout", ""), http.StatusNoContent, "")
	for _, s := range []ids.ID{c.session, kid.session} {
		if w.sessionState(s) != "logout" {
			t.Errorf("session %q", w.sessionState(s))
		}
		if r, _ := w.familyState(s); r != "logout" {
			t.Errorf("family %q", r)
		}
	}
	evs := w.revokedEvents()
	if len(evs) != 2 || evs[0]["actor"] != alex.String() || evs[0]["resource"] != c.session.String() {
		t.Fatalf("events %v", evs)
	}
	if len(*w.ended) != 1 || (*w.ended)[0].UserID != alex || (*w.ended)[0].Reason != identity.RevokeLogout {
		t.Fatalf("SessionEnded %+v", *w.ended)
	}
	if rows := w.audits("auth.logout"); len(rows) != 1 || rows[0].Decision != "allow" || rows[0].ResourceID != c.session.String() {
		t.Fatalf("audit %+v", rows)
	}
	w.want(c.call(http.MethodPost, "/api/v1/auth/logout", ""), http.StatusUnauthorized, "session_revoked")
	w.want(c.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	p := w.login(alex, identity.DeviceStatePending)
	w.want(p.call(http.MethodPost, "/api/v1/auth/logout", ""), http.StatusNoContent, "")
	w.want(w.h.Do(w.h.NewRequest(http.MethodPost, "/api/v1/auth/logout", "")), http.StatusUnauthorized, "unauthenticated")
}

// TestListSessions: self, owner, another person 404, revoked and expired left out, current,
// pages, pending device 403 device_pending, no null members.
func TestListSessions(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	sam := w.person("sam", false)
	robin := w.person("robin", true)
	a1 := w.login(alex, identity.DeviceStateActive)
	a2 := w.login(alex, identity.DeviceStateActive)
	a3 := w.login(alex, identity.DeviceStateActive)
	gone := w.login(alex, identity.DeviceStateActive)
	old := w.login(alex, identity.DeviceStateActive)
	if err := w.id.Sessions.RevokeSession(context.Background(), gone.session, identity.RevokeAdmin, nil); err != nil {
		t.Fatal(err)
	}
	w.sql(`UPDATE auth_sessions SET expires_at = $2 WHERE id = $1`, old.session.String(), w.clk.Now().Add(-time.Second))
	kid := a1.child()
	s := w.login(sam, identity.DeviceStateActive)
	r := w.login(robin, identity.DeviceStateActive)

	collect := func(c *client, path string) ([]string, []any) {
		var got []string
		var currents []any
		next := ""
		for range 5 {
			p := path
			if next != "" {
				p += "?cursor=" + next
			}
			m := w.want(c.call(http.MethodGet, p, ""), http.StatusOK, "")
			items := m["items"].([]any)
			if len(items) > sessCfg.PageSizeDefault {
				t.Fatalf("page of %d", len(items))
			}
			for _, it := range items {
				got = append(got, it.(map[string]any)["id"].(string))
				currents = append(currents, it.(map[string]any)["current"])
			}
			n, ok := m["next_cursor"].(string)
			if !ok {
				break
			}
			next = n
		}
		return got, currents
	}
	want := []string{kid.session.String(), a3.session.String(), a2.session.String(), a1.session.String()}
	got, currents := collect(a2, mySessions)
	if !slices.Equal(got, want) {
		t.Fatalf("own sessions\n got %v\nwant %v", got, want)
	}
	if !slices.Equal(currents, []any{nil, nil, true, nil}) {
		t.Fatalf("current %v", currents)
	}
	if got, _ := collect(r, "/api/v1/users/"+alex.String()+"/sessions"); !slices.Equal(got, want) {
		t.Fatalf("owner view %v", got)
	}
	if got, _ := collect(a1, "/api/v1/users/"+alex.String()+"/sessions"); !slices.Equal(got, want) {
		t.Fatalf("self by id %v", got)
	}
	w.want(s.call(http.MethodGet, "/api/v1/users/"+alex.String()+"/sessions", ""), http.StatusNotFound, "not_found")
	w.want(r.call(http.MethodGet, "/api/v1/users/"+ids.New().String()+"/sessions", ""), http.StatusNotFound, "not_found")
	w.want(s.call(http.MethodGet, "/api/v1/users/"+ids.New().String()+"/sessions", ""), http.StatusNotFound, "not_found")
	p := w.login(alex, identity.DeviceStatePending)
	w.want(p.call(http.MethodGet, mySessions, ""), http.StatusForbidden, "device_pending")
	w.want(w.h.Do(w.h.NewRequest(http.MethodGet, "/api/v1/users/"+alex.String()+"/sessions", "")), http.StatusUnauthorized, "unauthenticated")
	w.want(a1.call(http.MethodGet, mySessions+"?cursor=bogus", ""), http.StatusBadRequest, "cursor_invalid")
}

// TestStartSession_Rules: a device of another person, a revoked device and an unknown method are
// refused; a pending device gets a session.
func TestStartSession_Rules(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	alex := w.person("alex", false)
	sam := w.person("sam", false)
	c := w.login(alex, identity.DeviceStateActive)
	ctx := context.Background()
	if _, err := w.id.Sessions.StartSession(ctx, identity.StartSession{UserID: sam, DeviceID: c.device, AuthMethod: identity.AuthOIDC}); err == nil {
		t.Fatal("session on another person's device")
	}
	if _, err := w.id.Sessions.StartSession(ctx, identity.StartSession{UserID: alex, DeviceID: c.device, AuthMethod: "magic"}); err == nil {
		t.Fatal("unknown auth method")
	}
	w.sql(`UPDATE devices SET state = 'revoked', revoked_at = now(), revoke_reason = 'revoked_by_self' WHERE id = $1`, c.device.String())
	if _, err := w.id.Sessions.StartSession(ctx, identity.StartSession{UserID: alex, DeviceID: c.device, AuthMethod: identity.AuthOIDC}); err == nil {
		t.Fatal("session on a revoked device")
	}
	w.login(alex, identity.DeviceStatePending)
}

// TestRevokeParent_ChildCreatedConcurrently (red team M1-34, minor 1): a child inserted by a
// transaction that holds the parent lock while a logout of the parent waits on it is revoked with
// the parent, because the children are read after the parent lock is granted.
func TestRevokeParent_ChildCreatedConcurrently(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	repo := postgres.NewRepository(w.pool)
	child := ids.New()
	locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- w.uow.Do(context.Background(), func(ctx context.Context) error {
			if _, err := repo.LockSessionTree(ctx, c.session); err != nil {
				return err
			}
			now := w.clk.Now()
			err := repo.InsertSession(ctx, app.SessionRecord{ID: child, UserID: c.user, DeviceID: c.device, Component: identity.ComponentShareExt,
				ParentID: c.session, AuthMethod: identity.AuthPasskey, CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
			close(locked)
			<-release
			return err
		})
	}()
	<-locked
	revoked := make(chan error, 1)
	go func() {
		revoked <- w.id.Sessions.RevokeSession(context.Background(), c.session, identity.RevokeLogout, nil)
	}()
	// The proof needs the logout to wait on the parent lock before the child commits.
	deadline := testkit.Deadline()
	for w.count(`SELECT count(*) FROM pg_locks WHERE NOT granted
		AND pid IN (SELECT pid FROM pg_stat_activity WHERE datname = current_database())`) == 0 {
		if time.Now().After(deadline) {
			close(release)
			t.Fatal("the logout never waited on the parent lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	release <- struct{}{}
	if err := <-revoked; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if w.sessionState(child) != "logout" {
		t.Fatalf("child created under the parent lock survived the logout: %q", w.sessionState(child))
	}
}

// TestRefresh_DeviceKeyRevoked (red team M1-34, minor 4): a family whose key is no longer the
// device's active dpop key refreshes no more: 401 session_revoked, nothing rotates.
func TestRefresh_DeviceKeyRevoked(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	w.sql(`UPDATE device_keys SET revoked_at = now() WHERE device_id = $1`, c.device.String())
	w.want(c.refreshWith(c.refresh, c.key), http.StatusUnauthorized, "session_revoked")
	if _, gen := w.familyState(c.session); gen != 0 {
		t.Fatalf("generation %d", gen)
	}
}

// TestTouch_KeepsLastIP (red team M1-34 round 2): an activity write without a valid client IP keeps
// the stored last_ip.
func TestTouch_KeepsLastIP(t *testing.T) {
	t.Parallel()
	w := newSessionWorld(t)
	c := w.login(w.person("alex", false), identity.DeviceStateActive)
	repo := postgres.NewRepository(w.pool)
	ctx := context.Background()
	now := w.clk.Now()
	if err := repo.Touch(ctx, c.session, c.device, netip.MustParseAddr("192.0.2.1"), now, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.Touch(ctx, c.session, c.device, netip.Addr{}, now.Add(time.Hour), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	var ip string
	var seen time.Time
	w.scalar(&ip, `SELECT host(last_ip) FROM auth_sessions WHERE id = $1`, c.session.String())
	w.scalar(&seen, `SELECT last_seen_at FROM auth_sessions WHERE id = $1`, c.session.String())
	if ip != "192.0.2.1" || !seen.Equal(now.Add(time.Hour)) {
		t.Fatalf("last_ip %q, last_seen_at %v", ip, seen)
	}
}
