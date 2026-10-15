// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
)

// The paths of the three revocation operations (M1-45).
func disablePath(id ids.ID) string       { return "/api/v1/users/" + id.String() + "/disable" }
func deviceRevokePath(id ids.ID) string  { return "/api/v1/devices/" + id.String() + "/revoke" }
func sessionRevokePath(id ids.ID) string { return "/api/v1/sessions/" + id.String() + "/revoke" }

// reasonJSON is the body ReasonBody.
func reasonJSON(reason string) string {
	b, _ := json.Marshal(map[string]string{"reason": reason})
	return string(b)
}

// eventsOf counts the event_log rows of typ whose audience is audience ("" for every audience).
func (w *sworld) eventsOf(typ string, audience ids.ID) int {
	w.t.Helper()
	if audience.IsZero() {
		return w.count(`SELECT count(*) FROM event_log WHERE type = $1`, typ)
	}
	return w.count(`SELECT count(*) FROM event_log WHERE type = $1 AND audience_principal_id = $2`, typ, audience.String())
}

// endedSince returns the SessionEnded events after the first n.
func (w *sworld) endedSince(n int) []identity.SessionEnded {
	w.evMu.Lock()
	defer w.evMu.Unlock()
	return slices.Clone((*w.ended)[n:])
}

// devicesRevokedSince returns the DeviceRevoked events after the first n.
func (w *sworld) devicesRevokedSince(n int) []identity.DeviceRevoked {
	w.evMu.Lock()
	defer w.evMu.Unlock()
	return slices.Clone((*w.devicesRevoked)[n:])
}

// eventCounts are the lengths of the two in-transaction event lists.
func (w *sworld) eventCounts() (ended, devices int) {
	w.evMu.Lock()
	defer w.evMu.Unlock()
	return len(*w.ended), len(*w.devicesRevoked)
}

// allowOf returns the allow entries of action on resource id, oldest first.
func (w *sworld) allowOf(action, id string) []sauditRow {
	w.t.Helper()
	var out []sauditRow
	for _, r := range w.audits(action) {
		if r.Decision == "allow" && r.ResourceID == id {
			out = append(out, r)
		}
	}
	return out
}

// hasDeny reports a deny entry of action with reason.
func (w *sworld) hasDeny(action, reason string) bool {
	w.t.Helper()
	for _, r := range w.audits(action) {
		if r.Decision == "deny" && r.Reason == reason {
			return true
		}
	}
	return false
}

// spaceGrant puts a space into the structure and grants principal read on it, so the principal
// sees something and a disable changes its view (access.changed).
func (w *sworld) spaceGrant(principal, by ids.ID) ids.ID {
	w.t.Helper()
	space, grant := ids.New(), ids.New()
	w.structure.PutSpace(space)
	// The spaces row behind the foreign key of grants.target_space_id (M1-52).
	w.sql(`INSERT INTO spaces (id, name, sort_key, created_by) VALUES ($1, $2, 'a0', $3)`, space.String(), space.String(), by.String())
	w.sql(`INSERT INTO grants (id, subject_kind, subject_principal_id, target_kind, target_space_id, level, reason, granted_by)
		VALUES ($1, 'user', $2, 'space', $3, 'read', 'Team', $4)`, grant.String(), principal.String(), space.String(), by.String())
	return grant
}

// payloadReason returns the member reason of an event payload.
func payloadReason(e map[string]string) string {
	var m map[string]any
	_ = json.Unmarshal([]byte(e["payload"]), &m)
	r, _ := m["reason"].(string)
	return r
}

// sameJSON reports two JSON documents with equal values.
func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return len(a) == 0 && len(b) == 0
	}
	ax, _ := json.Marshal(x)
	by, _ := json.Marshal(y)
	return string(ax) == string(by)
}

// ensureOwner adds an active owner: every rights change verifies SR8, and an installation always
// has one (bootstrap-owner).
func (w *sworld) ensureOwner() {
	w.t.Helper()
	w.person(w.handle("first"), true)
}

// num reads a JSON number of an audit detail as int.
func num(v any) int {
	f, _ := v.(float64)
	return int(f)
}

// caseIDT22DisableUserAllSteps (ID-T22): every step of 08 section 11 that belongs to M1 is in the
// database state after one disableUser by the owner with a fresh step-up.
func caseIDT22DisableUserAllSteps(t *testing.T, w *sworld) {
	ctx := context.Background()
	o := w.ownerWithStepUp()
	alex := w.person(w.handle("alex"), false)
	grant := w.spaceGrant(alex, o.user)
	phone := w.phone(alex)
	child := phone.child()
	pending := w.login(alex, identity.DeviceStatePending)
	pat, err := w.id.Tokens.Issue(ctx, identity.IssueTokenCmd{PrincipalID: alex, Name: "Laptop", ExpiresAt: w.clk.Now().Add(24 * time.Hour), CreatedBy: alex})
	if err != nil {
		t.Fatal(err)
	}
	version := w.permissionVersion()
	changed, revoked, updated := w.eventsOf("access.changed", alex), w.eventsOf("session.revoked", alex), w.eventsOf("user.updated", ids.ID{})
	endedN, devN := w.eventCounts()

	m := w.want(o.callH(http.MethodPost, disablePath(alex), reasonJSON("Left the team")), http.StatusOK, "")
	if m["status"] != "disabled" || m["id"] != alex.String() {
		t.Fatalf("answer %v", m)
	}
	// Step 2.
	var status string
	w.scalar(&status, `SELECT status FROM principals WHERE id = $1 AND disabled_at IS NOT NULL`, alex.String())
	if status != "disabled" {
		t.Fatalf("status %s", status)
	}
	// Step 3: sessions with user_disabled, their families session_revoked.
	sessions := []ids.ID{phone.session, child.session, pending.session}
	for _, s := range sessions {
		if r := w.sessionState(s); r != "user_disabled" {
			t.Errorf("session %v reason %q", s, r)
		}
		if r, _ := w.familyState(s); r != "session_revoked" {
			t.Errorf("family of %v reason %q", s, r)
		}
	}
	// Step 4: every device (pending included) and every key.
	if n := w.count(`SELECT count(*) FROM devices WHERE user_id = $1 AND (state <> 'revoked' OR revoke_reason <> 'user_disabled'
		OR revoked_by <> $2 OR revoked_at IS NULL)`, alex.String(), o.user.String()); n != 0 {
		t.Errorf("%d devices not revoked as user_disabled by the owner", n)
	}
	if n := w.count(`SELECT count(*) FROM device_keys k JOIN devices d ON d.id = k.device_id WHERE d.user_id = $1 AND k.revoked_at IS NULL`, alex.String()); n != 0 {
		t.Errorf("%d device keys active", n)
	}
	if n := w.count(`SELECT count(*) FROM device_keys k JOIN devices d ON d.id = k.device_id WHERE d.user_id = $1`, alex.String()); n != 3 {
		t.Errorf("%d device keys, want 3 (dpop and step_up of the phone, dpop of the pending device)", n)
	}
	// Step 5: the personal token.
	if n := w.count(`SELECT count(*) FROM api_tokens WHERE id = $1 AND revoked_at IS NOT NULL AND revoked_by = $2`, pat.Token.ID.String(), o.user.String()); n != 1 {
		t.Error("the personal token is not revoked by the owner")
	}
	// Step 6: rights version, access.changed once, session.revoked per session, user.updated.
	if v := w.permissionVersion(); v <= version {
		t.Errorf("permission_version %d, before %d", v, version)
	}
	if n := w.eventsOf("access.changed", alex) - changed; n != 1 {
		t.Errorf("%d access.changed for alex, want 1", n)
	}
	if n := w.eventsOf("session.revoked", alex) - revoked; n != len(sessions) {
		t.Errorf("%d session.revoked, want %d", n, len(sessions))
	}
	for _, e := range w.revokedEvents() {
		if slices.Contains(sessions, ids.MustParse(e["resource"])) && (e["audience"] != alex.String() || e["actor"] != o.user.String() ||
			payloadReason(e) != "user_disabled") {
			t.Errorf("session.revoked %v", e)
		}
	}
	if n := w.eventsOf("user.updated", ids.ID{}) - updated; n != 1 {
		t.Errorf("%d user.updated, want 1", n)
	}
	var payload string
	w.scalar(&payload, `SELECT payload::text FROM event_log WHERE type = 'user.updated' AND resource_id = $1 ORDER BY seq DESC LIMIT 1`, alex.String())
	var up struct {
		User map[string]any `json:"user"`
	}
	if err := json.Unmarshal([]byte(payload), &up); err != nil || up.User["status"] != "disabled" {
		t.Errorf("user.updated payload %s", payload)
	}
	// In-transaction events: every session in exactly one SessionEnded of its device, one
	// DeviceRevoked per device.
	seen := map[ids.ID]int{}
	for _, ev := range w.endedSince(endedN) {
		if ev.UserID != alex || ev.Reason != identity.RevokeUserDisabled || ev.At.IsZero() {
			t.Errorf("SessionEnded %+v", ev)
		}
		for _, s := range ev.SessionIDs {
			seen[s]++
			if want := map[ids.ID]ids.ID{phone.session: phone.device, child.session: phone.device, pending.session: pending.device}[s]; want != ev.DeviceID {
				t.Errorf("session %v reported with device %v", s, ev.DeviceID)
			}
		}
	}
	for _, s := range sessions {
		if seen[s] != 1 {
			t.Errorf("session %v in %d SessionEnded events", s, seen[s])
		}
	}
	devs := w.devicesRevokedSince(devN)
	if len(devs) != 2 {
		t.Fatalf("DeviceRevoked %+v", devs)
	}
	for _, d := range devs {
		if d.UserID != alex || d.Reason != "user_disabled" || (d.DeviceID != phone.device && d.DeviceID != pending.device) {
			t.Errorf("DeviceRevoked %+v", d)
		}
	}
	// Step 8.
	a := w.allowOf("user.disable", alex.String())
	if len(a) != 1 || a[0].Details["reason"] != "Left the team" || num(a[0].Details["sessions_revoked"]) != 3 ||
		num(a[0].Details["devices_revoked"]) != 2 || num(a[0].Details["tokens_revoked"]) != 1 {
		t.Errorf("audit %+v", a)
	}
	if strings.Contains(a[0].Raw, pat.Secret.Reveal()) {
		t.Error("the audit entry holds the token")
	}
	// Grants stay.
	if n := w.count(`SELECT count(*) FROM grants WHERE id = $1 AND revoked_at IS NULL`, grant.String()); n != 1 {
		t.Error("the grant of the disabled person was revoked")
	}
}

// caseAZM40DisabledPrincipalIsUnauthenticated (AZ-M40): after the disable the unexpired access
// token with a valid proof gets 401 session_revoked, audited as principal_inactive; the personal
// token gets 401 too.
func caseAZM40DisabledPrincipalIsUnauthenticated(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	alex := w.phone(w.person(w.handle("alex"), false))
	alex.stepUp()
	pat := alex.personalToken()
	w.want(alex.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	w.want(w.withToken(pat, http.MethodGet, tokensPath(alex.user), ""), http.StatusOK, "")
	w.want(o.callH(http.MethodPost, disablePath(alex.user), reasonJSON("Left")), http.StatusOK, "")
	w.want(alex.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	if !w.hasDeny("session.list", "principal_inactive") {
		t.Error("no deny entry with principal_inactive")
	}
	if r := w.withToken(pat, http.MethodGet, tokensPath(alex.user), ""); r.StatusCode != http.StatusUnauthorized {
		t.Errorf("personal token after the disable: %d %s", r.StatusCode, r.Body)
	}
}

// caseDisableUserRequiresOwnerAndStepUp: a person without owner role gets 403 owner_only, an owner
// with an old step-up 401 step_up_required, the owner's personal token 403.
func caseDisableUserRequiresOwnerAndStepUp(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	sam := w.phone(w.person(w.handle("sam"), false))
	sam.stepUp()
	w.wantReason(sam.callH(http.MethodPost, disablePath(alex), reasonJSON("x")), http.StatusForbidden, "forbidden", "owner_only")
	o := w.ownerWithStepUp()
	pat := o.personalToken()
	w.want(w.withToken(pat, http.MethodPost, disablePath(alex), reasonJSON("x")), http.StatusForbidden, "forbidden")
	w.clk.Advance(sessCfg.StepUpMaxAge + time.Second)
	w.want(o.callH(http.MethodPost, disablePath(alex), reasonJSON("x")), http.StatusUnauthorized, "step_up_required")
	var status string
	w.scalar(&status, `SELECT status FROM principals WHERE id = $1`, alex.String())
	if status != "active" {
		t.Fatalf("status %s after refused calls", status)
	}
}

// caseDisableUserAlreadyDisabledNoChange: a second disable answers 200 with the disabled person,
// raises no rights version, appends no event and is audited with no_change.
func caseDisableUserAlreadyDisabledNoChange(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	alex := w.person(w.handle("alex"), false)
	w.want(o.callH(http.MethodPost, disablePath(alex), reasonJSON("x")), http.StatusOK, "")
	v, events := w.permissionVersion(), w.count(`SELECT count(*) FROM event_log`)
	m := w.want(o.callH(http.MethodPost, disablePath(alex), reasonJSON("again")), http.StatusOK, "")
	if m["status"] != "disabled" || w.permissionVersion() != v || w.count(`SELECT count(*) FROM event_log`) != events {
		t.Fatalf("second disable changed something: %v", m)
	}
	a := w.allowOf("user.disable", alex.String())
	if len(a) != 2 || a[1].Details["no_change"] != true || a[1].Details["reason"] != "again" {
		t.Fatalf("audit %+v", a)
	}
}

// caseDisableUserDeletedIsInvalidTransition: a deleted person cannot be disabled (409).
func caseDisableUserDeletedIsInvalidTransition(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	robin := w.person(w.handle("robin"), false)
	w.sql(`UPDATE principals SET status = 'deleted', deleted_at = now() WHERE id = $1`, robin.String())
	v := w.permissionVersion()
	w.want(o.callH(http.MethodPost, disablePath(robin), reasonJSON("x")), http.StatusConflict, "invalid_state_transition")
	if w.permissionVersion() != v {
		t.Error("a refused disable raised the rights version")
	}
	// An agent is no person: not_found, like an unknown id.
	w.want(o.callH(http.MethodPost, disablePath(w.agentPrincipal("lotse")), reasonJSON("x")), http.StatusNotFound, "not_found")
	w.want(o.callH(http.MethodPost, disablePath(ids.New()), reasonJSON("x")), http.StatusNotFound, "not_found")
	// The reason is required.
	w.want(o.callH(http.MethodPost, disablePath(robin), `{"reason":""}`), http.StatusBadRequest, "validation_failed")
}

// caseIDT26RevokeDeviceAllSteps (ID-T26): device, keys, sessions with the child, families revoked
// with the codes of 08 section 10, session.revoked, DeviceRevoked and the audit entry; the old
// refresh token gets 401 session_revoked without a reuse entry, the old access token 401.
func caseIDT26RevokeDeviceAllSteps(t *testing.T, w *sworld) {
	w.ensureOwner()
	alex := w.person(w.handle("alex"), false)
	lost := w.phone(alex)
	child := lost.child()
	laptop := w.phone(alex)
	laptop.stepUp()
	version := w.permissionVersion()
	endedN, devN := w.eventCounts()
	refreshDenials := len(w.audits("auth.token.refresh"))

	m := w.want(laptop.callH(http.MethodPost, deviceRevokePath(lost.device), reasonJSON("Lost on the train")), http.StatusOK, "")
	if m["state"] != "revoked" || m["id"] != lost.device.String() {
		t.Fatalf("answer %v", m)
	}
	if n := w.count(`SELECT count(*) FROM devices WHERE id = $1 AND state = 'revoked' AND revoke_reason = 'revoked_by_self'
		AND revoked_by = $2 AND revoked_at IS NOT NULL`, lost.device.String(), alex.String()); n != 1 {
		t.Error("device not revoked as revoked_by_self")
	}
	if n := w.count(`SELECT count(*) FROM device_keys WHERE device_id = $1 AND revoked_at IS NULL`, lost.device.String()); n != 0 {
		t.Errorf("%d keys active", n)
	}
	for _, s := range []ids.ID{lost.session, child.session} {
		if r := w.sessionState(s); r != "device_revoked" {
			t.Errorf("session %v reason %q", s, r)
		}
		if r, _ := w.familyState(s); r != "session_revoked" {
			t.Errorf("family of %v reason %q", s, r)
		}
	}
	if r := w.sessionState(laptop.session); r != "" {
		t.Errorf("the session of the other device ended: %q", r)
	}
	if v := w.permissionVersion(); v <= version {
		t.Errorf("permission_version %d, before %d", v, version)
	}
	ended := w.endedSince(endedN)
	if len(ended) != 1 || ended[0].DeviceID != lost.device || ended[0].Reason != identity.RevokeDeviceRevoked || len(ended[0].SessionIDs) != 2 {
		t.Errorf("SessionEnded %+v", ended)
	}
	if d := w.devicesRevokedSince(devN); len(d) != 1 || d[0].DeviceID != lost.device || d[0].Reason != "revoked_by_self" || d[0].UserID != alex {
		t.Errorf("DeviceRevoked %+v", d)
	}
	var revokedFor []string
	for _, e := range w.revokedEvents() {
		if payloadReason(e) == "device_revoked" && e["audience"] == alex.String() {
			revokedFor = append(revokedFor, e["resource"])
		}
	}
	if !slices.Contains(revokedFor, lost.session.String()) || !slices.Contains(revokedFor, child.session.String()) {
		t.Errorf("session.revoked for %v", revokedFor)
	}
	a := w.allowOf("device.revoke", lost.device.String())
	if len(a) != 1 || a[0].Details["reason"] != "Lost on the train" || a[0].Details["revoke_reason"] != "revoked_by_self" ||
		num(a[0].Details["sessions_revoked"]) != 2 || a[0].Details["user_id"] != alex.String() {
		t.Errorf("audit %+v", a)
	}
	// The old tokens.
	w.want(lost.refreshWith(lost.refresh, lost.key), http.StatusUnauthorized, "session_revoked")
	rows := w.audits("auth.token.refresh")[refreshDenials:]
	if len(rows) != 1 || rows[0].Details["cause"] == "refresh_reuse" {
		t.Errorf("refresh audit %+v", rows)
	}
	if r, _ := w.familyState(lost.session); r != "session_revoked" {
		t.Errorf("the refresh turned the family into %q", r)
	}
	w.want(lost.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	w.want(child.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	w.want(laptop.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	// A second revoke is without effect.
	v := w.permissionVersion()
	w.want(laptop.callH(http.MethodPost, deviceRevokePath(lost.device), reasonJSON("again")), http.StatusOK, "")
	if w.permissionVersion() != v {
		t.Error("revoking a revoked device raised the rights version")
	}
	if a := w.allowOf("device.revoke", lost.device.String()); len(a) != 2 || a[1].Details["no_change"] != true {
		t.Errorf("audit of the repeat %+v", a)
	}
}

// caseInactiveDeviceRefusedThenExpired (08 section 5.4): an active device without contact for
// auth.device_inactive_revoke_after is refused at once (device_inactive, no activity recorded, no
// refresh), and identity.auth_cleanup revokes it with reason inactive as the system action
// device.expire, like device.revoke; a device never seen counts from approved_at; recent and
// pending devices stay; a run without such a device raises no rights version.
func caseInactiveDeviceRefusedThenExpired(t *testing.T, w *sworld) {
	w.ensureOwner()
	ctx := context.Background()
	alex := w.person(w.handle("alex"), false)
	old, never, recent := w.phone(alex), w.phone(alex), w.phone(alex)
	pending := w.login(alex, identity.DeviceStatePending)
	stale := w.clk.Now().Add(-sessCfg.DeviceInactiveRevokeAfter - time.Hour)
	w.sql(`UPDATE devices SET last_seen_at = $2 WHERE id = $1`, old.device.String(), stale)
	w.sql(`UPDATE devices SET last_seen_at = NULL, approved_at = $2 WHERE id = $1`, never.device.String(), stale)
	// The pending device keeps its creation time: older than auth.device_pending_ttl it would be
	// revoked as pending_expired by M1-38 (INT-W14/9); the inactivity rule ignores it either way.
	w.sql(`UPDATE devices SET last_seen_at = $2 WHERE id = $1`, pending.device.String(), stale)
	w.want(old.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	w.want(never.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	if !w.hasDeny("session.list", "device_inactive") {
		t.Error("no deny entry with device_inactive")
	}
	var seen time.Time
	w.scalar(&seen, `SELECT last_seen_at FROM devices WHERE id = $1`, old.device.String())
	if !seen.Equal(stale) {
		t.Errorf("the refused request recorded activity: %v", seen)
	}
	w.want(old.refreshWith(old.refresh, old.key), http.StatusUnauthorized, "session_revoked")
	w.want(recent.call(http.MethodGet, mySessions, ""), http.StatusOK, "")

	version := w.permissionVersion()
	_, devN := w.eventCounts()
	n, err := w.id.AuthCleanup.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 || w.permissionVersion() <= version {
		t.Fatalf("cleanup removed %d, version %d after %d", n, w.permissionVersion(), version)
	}
	for _, c := range []*stepClient{old, never} {
		if k := w.count(`SELECT count(*) FROM devices WHERE id = $1 AND state = 'revoked' AND revoke_reason = 'inactive' AND revoked_by IS NULL`, c.device.String()); k != 1 {
			t.Errorf("device %v not revoked as inactive", c.device)
		}
		if r := w.sessionState(c.session); r != "inactive" {
			t.Errorf("session reason %q", r)
		}
		if r, _ := w.familyState(c.session); r != "session_revoked" {
			t.Errorf("family reason %q", r)
		}
		if k := w.count(`SELECT count(*) FROM device_keys WHERE device_id = $1 AND revoked_at IS NULL`, c.device.String()); k != 0 {
			t.Errorf("%d keys active", k)
		}
		a := w.allowOf("device.expire", c.device.String())
		if len(a) != 1 || a[0].Details["revoke_reason"] != "inactive" || num(a[0].Details["sessions_revoked"]) != 1 {
			t.Errorf("audit %+v", a)
		}
	}
	var actor string
	w.scalar(&actor, `SELECT coalesce(actor_principal_id::text, '') FROM audit_events WHERE action = 'device.expire' AND resource_id = $1`, old.device.String())
	if actor != "" {
		t.Errorf("device.expire has the actor %s", actor)
	}
	if d := w.devicesRevokedSince(devN); len(d) != 2 || d[0].Reason != "inactive" {
		t.Errorf("DeviceRevoked %+v", d)
	}
	for _, id := range []ids.ID{recent.device, pending.device} {
		if k := w.count(`SELECT count(*) FROM devices WHERE id = $1 AND state <> 'revoked'`, id.String()); k != 1 {
			t.Errorf("device %v was revoked", id)
		}
	}
	w.want(recent.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	v := w.permissionVersion()
	if _, err := w.id.AuthCleanup.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if w.permissionVersion() != v {
		t.Error("a cleanup run without inactive devices raised the rights version")
	}
}

// sameNotFound checks that a and b are both 404 with equal problems and equal headers apart from
// the per-request ones.
func (w *sworld) sameNotFound(a, b *httpservertest.Response) {
	w.t.Helper()
	w.want(a, http.StatusNotFound, "not_found")
	w.want(b, http.StatusNotFound, "not_found")
	if strip(a.Problem(w.t)) != strip(b.Problem(w.t)) {
		w.t.Errorf("bodies differ\n%s\n%s", a.Body, b.Body)
	}
	for _, h := range []string{"Content-Type", "Cache-Control", "WWW-Authenticate", "Idempotent-Replay", "Content-Language"} {
		if a.Header.Get(h) != b.Header.Get(h) {
			w.t.Errorf("header %s: %q vs %q", h, a.Header.Get(h), b.Header.Get(h))
		}
	}
	if len(a.Header) != len(b.Header) {
		w.t.Errorf("header sets differ: %v vs %v", a.Header, b.Header)
	}
}

// caseRevokeDeviceOtherPersonsDeviceIs404ForNonOwner: the device of another person is invisible
// to a non-owner: the same 404 as an unknown device id.
func caseRevokeDeviceOtherPersonsDeviceIs404ForNonOwner(t *testing.T, w *sworld) {
	w.ensureOwner()
	alex := w.phone(w.person(w.handle("alex"), false))
	sam := w.phone(w.person(w.handle("sam"), false))
	sam.stepUp()
	w.sameNotFound(sam.callH(http.MethodPost, deviceRevokePath(alex.device), reasonJSON("x")),
		sam.callH(http.MethodPost, deviceRevokePath(ids.New()), reasonJSON("x")))
	w.want(alex.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
}

// caseRevokeDeviceOwnerRevokesForeignDevice: an owner revokes the device of another person with
// revoked_by_owner.
func caseRevokeDeviceOwnerRevokesForeignDevice(t *testing.T, w *sworld) {
	alex := w.phone(w.person(w.handle("alex"), false))
	o := w.ownerWithStepUp()
	m := w.want(o.callH(http.MethodPost, deviceRevokePath(alex.device), reasonJSON("Left")), http.StatusOK, "")
	if m["state"] != "revoked" {
		t.Fatalf("answer %v", m)
	}
	if n := w.count(`SELECT count(*) FROM devices WHERE id = $1 AND revoke_reason = 'revoked_by_owner' AND revoked_by = $2`,
		alex.device.String(), o.user.String()); n != 1 {
		t.Error("not revoked_by_owner")
	}
	w.want(alex.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
}

// caseRevokeDeviceStepUpRequired: without a fresh step-up the person gets 401 step_up_required and
// the device stays.
func caseRevokeDeviceStepUpRequired(t *testing.T, w *sworld) {
	w.ensureOwner()
	alex := w.person(w.handle("alex"), false)
	phone, other := w.phone(alex), w.phone(alex)
	v := w.permissionVersion()
	w.want(phone.callH(http.MethodPost, deviceRevokePath(other.device), reasonJSON("x")), http.StatusUnauthorized, "step_up_required")
	w.want(other.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	if w.permissionVersion() != v {
		t.Error("a refused revoke raised the rights version")
	}
}

// caseRevokeDeviceSelfCurrentDevice: a person revokes the device it uses; the answer is sent,
// the next request fails.
func caseRevokeDeviceSelfCurrentDevice(t *testing.T, w *sworld) {
	w.ensureOwner()
	phone := w.phone(w.person(w.handle("alex"), false))
	phone.stepUp()
	m := w.want(phone.callH(http.MethodPost, deviceRevokePath(phone.device), reasonJSON("Sold")), http.StatusOK, "")
	if m["state"] != "revoked" {
		t.Fatalf("answer %v", m)
	}
	w.want(phone.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
}

// caseAZM41RevokedSessionIsUnauthenticated (AZ-M41): after revokeSession the session's unexpired
// access token gets 401 session_revoked, audited as session_revoked.
func caseAZM41RevokedSessionIsUnauthenticated(t *testing.T, w *sworld) {
	w.ensureOwner()
	alex := w.person(w.handle("alex"), false)
	phone, laptop, extra := w.phone(alex), w.phone(alex), w.phone(alex)
	w.want(phone.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	// A keyed answer stored before the revocation (06 section 7).
	key := ids.New().String()
	w.want(phone.callH(http.MethodPost, sessionRevokePath(extra.session), "", "Idempotency-Key", key), http.StatusNoContent, "")
	version := w.permissionVersion()
	w.want(laptop.callH(http.MethodPost, sessionRevokePath(phone.session), ""), http.StatusNoContent, "")
	if w.permissionVersion() <= version {
		t.Error("session.revoke did not raise the rights version")
	}
	// The same key again: a revoked session never gets the stored answer back.
	replay := phone.callH(http.MethodPost, sessionRevokePath(extra.session), "", "Idempotency-Key", key)
	w.want(replay, http.StatusUnauthorized, "session_revoked")
	if replay.Header.Get("Idempotent-Replay") != "" {
		t.Errorf("replay header %q", replay.Header.Get("Idempotent-Replay"))
	}
	w.want(phone.callH(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	if !w.hasDeny("session.list", "session_revoked") {
		t.Error("no deny entry with session_revoked")
	}
	w.want(laptop.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
}

// caseRevokeSessionChildSessionsEnd: the child session ends with its parent, same reason.
func caseRevokeSessionChildSessionsEnd(t *testing.T, w *sworld) {
	w.ensureOwner()
	alex := w.person(w.handle("alex"), false)
	phone, laptop := w.phone(alex), w.phone(alex)
	child := phone.child()
	endedN, _ := w.eventCounts()
	w.want(laptop.callH(http.MethodPost, sessionRevokePath(phone.session), ""), http.StatusNoContent, "")
	if w.sessionState(phone.session) != "logout" || w.sessionState(child.session) != "logout" {
		t.Errorf("reasons %q %q", w.sessionState(phone.session), w.sessionState(child.session))
	}
	if r, _ := w.familyState(child.session); r != "logout" {
		t.Errorf("child family %q", r)
	}
	w.want(child.call(http.MethodGet, mySessions, ""), http.StatusUnauthorized, "session_revoked")
	if e := w.endedSince(endedN); len(e) != 1 || len(e[0].SessionIDs) != 2 {
		t.Errorf("SessionEnded %+v", e)
	}
	a := w.allowOf("session.revoke", phone.session.String())
	if len(a) != 1 || num(a[0].Details["sessions_revoked"]) != 2 || a[0].Details["revoke_reason"] != "logout" {
		t.Errorf("audit %+v", a)
	}
	// Revoking the child alone afterwards is without effect.
	v := w.permissionVersion()
	w.want(laptop.callH(http.MethodPost, sessionRevokePath(child.session), ""), http.StatusNoContent, "")
	if w.permissionVersion() != v {
		t.Error("revoking a revoked session raised the rights version")
	}
	if a := w.allowOf("session.revoke", child.session.String()); len(a) != 1 || a[0].Details["no_change"] != true {
		t.Errorf("audit of the repeat %+v", a)
	}
}

// caseRevokeSessionReasonLogoutForSelfAdminForOwner: the person's own revoke is logout, the
// owner's admin; session.revoked carries the reason.
func caseRevokeSessionReasonLogoutForSelfAdminForOwner(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	a, b, c := w.phone(alex), w.phone(alex), w.phone(alex)
	o := w.ownerWithStepUp()
	w.want(b.callH(http.MethodPost, sessionRevokePath(a.session), ""), http.StatusNoContent, "")
	w.want(o.callH(http.MethodPost, sessionRevokePath(c.session), ""), http.StatusNoContent, "")
	if w.sessionState(a.session) != "logout" || w.sessionState(c.session) != "admin" {
		t.Fatalf("reasons %q %q", w.sessionState(a.session), w.sessionState(c.session))
	}
	for _, e := range w.revokedEvents() {
		switch e["resource"] {
		case a.session.String():
			if payloadReason(e) != "logout" || e["actor"] != alex.String() {
				t.Errorf("event %v", e)
			}
		case c.session.String():
			if payloadReason(e) != "admin" || e["actor"] != o.user.String() {
				t.Errorf("event %v", e)
			}
		}
	}
}

// caseRevokeSessionOtherPersonsSessionIs404: the session of another person is invisible to a
// non-owner: the same 404 as an unknown session.
func caseRevokeSessionOtherPersonsSessionIs404(t *testing.T, w *sworld) {
	w.ensureOwner()
	alex := w.phone(w.person(w.handle("alex"), false))
	sam := w.phone(w.person(w.handle("sam"), false))
	w.sameNotFound(sam.callH(http.MethodPost, sessionRevokePath(alex.session), ""), sam.callH(http.MethodPost, sessionRevokePath(ids.New()), ""))
	w.want(alex.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
}

// caseRevokeSessionNoStepUpNeeded: session.revoke needs no step-up.
func caseRevokeSessionNoStepUpNeeded(t *testing.T, w *sworld) {
	w.ensureOwner()
	alex := w.person(w.handle("alex"), false)
	a, b := w.phone(alex), w.phone(alex)
	w.clk.Advance(sessCfg.StepUpMaxAge + time.Second)
	w.want(a.callH(http.MethodPost, sessionRevokePath(b.session), ""), http.StatusNoContent, "")
	if w.sessionState(b.session) != "logout" {
		t.Error("session not ended")
	}
}

// caseRevocationIdempotencyReplay: each of the three operations with an Idempotency-Key replays
// its stored answer with Idempotent-Replay: true and changes nothing again.
func caseRevocationIdempotencyReplay(t *testing.T, w *sworld) {
	o := w.ownerWithStepUp()
	alex := w.person(w.handle("alex"), false)
	sam := w.person(w.handle("sam"), false)
	phone, laptop := w.phone(sam), w.phone(sam)
	laptop.stepUp()
	extra := w.phone(sam)
	for _, c := range []struct {
		name   string
		client *stepClient
		path   string
		body   string
		status int
	}{
		{"disableUser", o, disablePath(alex), reasonJSON("Left"), http.StatusOK},
		{"revokeDevice", laptop, deviceRevokePath(phone.device), reasonJSON("Lost"), http.StatusOK},
		{"revokeSession", laptop, sessionRevokePath(extra.session), "", http.StatusNoContent},
	} {
		key := ids.New().String()
		first := c.client.callH(http.MethodPost, c.path, c.body, "Idempotency-Key", key)
		w.want(first, c.status, "")
		v, events := w.permissionVersion(), w.count(`SELECT count(*) FROM event_log`)
		replay := c.client.callH(http.MethodPost, c.path, c.body, "Idempotency-Key", key)
		w.want(replay, c.status, "")
		if replay.Header.Get("Idempotent-Replay") != "true" || !sameJSON(replay.Body, first.Body) {
			t.Errorf("%s: replay %v %s, first %s", c.name, replay.Header, replay.Body, first.Body)
		}
		if w.permissionVersion() != v || w.count(`SELECT count(*) FROM event_log`) != events {
			t.Errorf("%s: the replay changed something", c.name)
		}
	}
}

// TestLifecycle_Revocation: user.disable (ID-T22, AZ-M40, the requirements and the
// transitions), device.revoke (ID-T26, the visibility and step-up rules, the inactive device
// sweep), session.revoke (AZ-M41 and the reason, child and visibility rules) and the idempotency
// replay of all three operations, as subtests on one world (a world costs seconds, a case tenths).
func TestLifecycle_Revocation(t *testing.T) {
	runTokenCases(t, []tokenCase{
		{"TestIDT22_DisableUserAllSteps", caseIDT22DisableUserAllSteps},
		{"TestAZM40_DisabledPrincipalIsUnauthenticated", caseAZM40DisabledPrincipalIsUnauthenticated},
		{"TestDisableUser_AlreadyDisabledNoChange", caseDisableUserAlreadyDisabledNoChange},
		{"TestDisableUser_DeletedIsInvalidTransition", caseDisableUserDeletedIsInvalidTransition},
		{"TestIDT26_RevokeDeviceAllSteps", caseIDT26RevokeDeviceAllSteps},
		{"TestRevokeDevice_OtherPersonsDeviceIs404ForNonOwner", caseRevokeDeviceOtherPersonsDeviceIs404ForNonOwner},
		{"TestRevokeDevice_OwnerRevokesForeignDevice", caseRevokeDeviceOwnerRevokesForeignDevice},
		{"TestRevokeDevice_StepUpRequired", caseRevokeDeviceStepUpRequired},
		{"TestRevokeDevice_SelfCurrentDevice", caseRevokeDeviceSelfCurrentDevice},
		{"TestDeviceInactive_RefusedThenExpired", caseInactiveDeviceRefusedThenExpired},
		{"TestAZM41_RevokedSessionIsUnauthenticated", caseAZM41RevokedSessionIsUnauthenticated},
		{"TestRevokeSession_ChildSessionsEnd", caseRevokeSessionChildSessionsEnd},
		{"TestRevokeSession_ReasonLogoutForSelfAdminForOwner", caseRevokeSessionReasonLogoutForSelfAdminForOwner},
		{"TestRevokeSession_OtherPersonsSessionIs404", caseRevokeSessionOtherPersonsSessionIs404},
		{"TestRevocation_IdempotencyReplay", caseRevocationIdempotencyReplay},
		// Last: both advance the clock past the step-up window.
		{"TestDisableUser_RequiresOwnerAndStepUp", caseDisableUserRequiresOwnerAndStepUp},
		{"TestRevokeSession_NoStepUpNeeded", caseRevokeSessionNoStepUpNeeded},
	})
}

// snapshot returns every row of the tables a disable touches, as text, plus the rights version.
func (w *sworld) snapshot() map[string]string {
	w.t.Helper()
	out := map[string]string{}
	for _, table := range []string{"principals", "users", "owners", "devices", "device_keys", "auth_sessions", "refresh_token_families",
		"refresh_tokens", "api_tokens", "grants", "group_memberships", "event_log"} {
		var s string
		w.scalar(&s, `SELECT coalesce(string_agg(t::text, '|' ORDER BY t::text), '') FROM `+table+` t`)
		out[table] = s
	}
	var v string
	w.scalar(&v, `SELECT value::text FROM sequence_counters WHERE name = 'permission_version'`)
	out["permission_version"] = v
	return out
}

// TestLifecycle_Owners: the cases that need an exact number of owners, in order on one world:
// first the only owner (AZ-M26), then two owners.
func TestLifecycle_Owners(t *testing.T) {
	runTokenCases(t, []tokenCase{
		{"TestAZM26_DisableLastOwnerFails", caseAZM26DisableLastOwnerFails},
		{"TestDisableUser_ConcurrentOwners", caseDisableUserConcurrentOwners},
	})
}

// caseAZM26DisableLastOwnerFails (AZ-M26): the only owner disables themselves: 409 last_owner,
// every touched table is unchanged row by row, and a deny entry exists after the rollback.
func caseAZM26DisableLastOwnerFails(t *testing.T, w *sworld) {
	if n := w.count(`SELECT count(*) FROM owners`); n != 0 {
		t.Fatalf("%d owners before the case", n)
	}
	o := w.ownerWithStepUp()
	child := o.child()
	before := w.snapshot()
	w.wantReason(o.callH(http.MethodPost, disablePath(o.user), reasonJSON("Leaving")), http.StatusConflict, "last_owner", "last_owner")
	after := w.snapshot()
	for table, rows := range before {
		if after[table] != rows {
			t.Errorf("%s changed:\nbefore %s\nafter  %s", table, rows, after[table])
		}
	}
	if !w.hasDeny("user.disable", "last_owner") {
		t.Errorf("no deny entry: %+v", w.audits("user.disable"))
	}
	w.want(o.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
	w.want(child.call(http.MethodGet, mySessions, ""), http.StatusOK, "")
}

// caseDisableUserConcurrentOwners: the only two owners disable each other at the same time:
// exactly one succeeds, the other gets 401 session_revoked (authorize in the serialized unit sees
// its caller disabled), and exactly one active owner remains. The owner of the case before steps
// down first.
func caseDisableUserConcurrentOwners(t *testing.T, w *sworld) {
	w.sql(`UPDATE owners SET revoked_at = now() WHERE revoked_at IS NULL`)
	w.sql(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
	a, b := w.ownerWithStepUp(), w.ownerWithStepUp()
	for round := range 3 {
		var wg sync.WaitGroup
		res := make([]*httpservertest.Response, 2)
		for i, pair := range [][2]*stepClient{{a, b}, {b, a}} {
			wg.Go(func() { res[i] = pair[0].callH(http.MethodPost, disablePath(pair[1].user), reasonJSON("x")) })
		}
		wg.Wait()
		ok, refused := 0, 0
		for _, r := range res {
			switch {
			case r.StatusCode == http.StatusOK:
				ok++
			case r.StatusCode == http.StatusUnauthorized && strings.Contains(string(r.Body), `"session_revoked"`):
				refused++
			default:
				t.Fatalf("round %d: %d %s", round, r.StatusCode, r.Body)
			}
		}
		if ok != 1 || refused != 1 {
			t.Fatalf("round %d: %d succeeded, %d refused", round, ok, refused)
		}
		if n := w.count(`SELECT count(*) FROM owners o JOIN principals p ON p.id = o.user_id WHERE o.revoked_at IS NULL AND p.status = 'active'`); n != 1 {
			t.Fatalf("round %d: %d active owners", round, n)
		}
		// Back to two active owners on fresh devices for the next round.
		// The raised version drops the cached snapshots that saw a person disabled.
		w.sql(`UPDATE principals SET status = 'active', disabled_at = NULL WHERE kind = 'user'`)
		w.sql(`UPDATE sequence_counters SET value = value + 1 WHERE name = 'permission_version'`)
		a, b = w.freshPhone(a.user), w.freshPhone(b.user)
	}
}

// freshPhone starts a fresh phone of user with a fresh step-up (named relogin in M1-45; M1-38
// has a relogin of its own, INT-W14/9).
func (w *sworld) freshPhone(user ids.ID) *stepClient {
	w.t.Helper()
	c := w.phone(user)
	c.stepUp()
	return c
}
