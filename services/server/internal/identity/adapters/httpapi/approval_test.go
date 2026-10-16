// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/identitytest"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver"
	"github.com/OWNER/bothy/services/server/internal/platform/httpserver/httpservertest"
	"github.com/OWNER/bothy/services/server/internal/platform/ratelimit"
)

// The paths of M1-38.
const (
	pairingBeginPath    = "/api/v1/auth/pairing/begin"
	pairingCompletePath = "/api/v1/auth/pairing/complete"
	pairingApprovePath  = "/api/v1/me/devices/pairings"
)

// pairingForm is the display form of a pairing code of eight characters.
var pairingForm = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}$`)

// newApprovalWorld is a session world with an owner, as every installation has one: an approval
// is a rights change, and the rights hook keeps at least one active owner.
func newApprovalWorld(t *testing.T) *sworld {
	t.Helper()
	w := newSessionWorld(t)
	w.person("robin", true)
	return w
}

// runApprovalCases runs the cases on one new approval world, each as a subtest named after the
// test of the brief; w.t is the subtest while it runs. One world per group keeps the package
// inside the default test timeout on a loaded machine (as runTokenCases).
func runApprovalCases(t *testing.T, cases []tokenCase) {
	t.Parallel()
	w := newApprovalWorld(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prev := w.t
			w.t = t
			defer func() { w.t = prev }()
			c.fn(t, w)
		})
	}
}

// TestDeviceApproval: pending devices, the verification code and approveDevice (ID-T12, ID-T13,
// the rights of the approval).
func TestDeviceApproval(t *testing.T) {
	runApprovalCases(t, []tokenCase{
		{"TestIDT12_PendingDeviceHasTokensButNoAccess", caseIDT12PendingDeviceHasTokensButNoAccess},
		{"TestVerificationCodeOnlyForPendingDeviceItself", caseVerificationCodeOnlyForPendingDeviceItself},
		{"TestApproveDeviceBySelfFromActiveDevice", caseApproveDeviceBySelfFromActiveDevice},
		{"TestApproveDeviceByOwner", caseApproveDeviceByOwner},
		{"TestIDT13_WrongCodeRevokesDevice", caseIDT13WrongCodeRevokesDevice},
		{"TestApproveDeviceNeedsStepUp", caseApproveDeviceNeedsStepUp},
		{"TestApproveDeviceFromPendingDevice", caseApproveDeviceFromPendingDevice},
		{"TestApproveForeignDeviceByNonOwner", caseApproveForeignDeviceByNonOwner},
		{"TestApproveActiveDevice", caseApproveActiveDevice},
		{"TestApproveRevokedDevice", caseApproveRevokedDevice},
	})
}

// TestDevicePairing: the pairing end to end, a wrong code, another key, the rate limit and the
// logs.
func TestDevicePairing(t *testing.T) {
	runApprovalCases(t, []tokenCase{
		{"TestPairingHappyPath", casePairingHappyPath},
		{"TestPairingWrongCodeEndsStepUp", casePairingWrongCodeEndsStepUp},
		{"TestPairingCompleteWithOtherKeyRejects", casePairingCompleteWithOtherKeyRejects},
		{"TestPairingBeginRateLimitedPerIP", casePairingBeginRateLimitedPerIP},
		{"TestPairingCodeNeverLogged", casePairingCodeNeverLogged},
	})
}

// TestPairingExpiry: expired pairings on access and in the cleanup job; the cases move the clock.
func TestPairingExpiry(t *testing.T) {
	runApprovalCases(t, []tokenCase{
		{"TestPairingExpiryOnAccess", casePairingExpiryOnAccess},
		{"TestPairingExpiry", casePairingExpiry},
	})
}

// auditsOf returns the audit rows of action on the resource id, oldest first.
func (w *sworld) auditsOf(action, resourceID string) []sauditRow {
	w.t.Helper()
	var out []sauditRow
	for _, r := range w.audits(action) {
		if r.ResourceID == resourceID {
			out = append(out, r)
		}
	}
	return out
}

// devicePath is the path of getDevice; approvePath that of approveDevice.
func devicePath(id ids.ID) string  { return "/api/v1/devices/" + id.String() }
func approvePath(id ids.ID) string { return devicePath(id) + "/approve" }

// waiting is a device that logged in without confirmation: its client and the code it was shown.
type waiting struct {
	*client
	code string
	reg  identity.RegisterDevice
	jkt  string
}

// pendingLogin logs user in on a new device through CompleteLogin with TrustUnconfirmed (the way
// the password, passkey and OIDC logins will) and returns the pending device with its code.
func (w *sworld) pendingLogin(user ids.ID) *waiting {
	w.t.Helper()
	p := w.phoneKeys()
	reg := identity.RegisterDevice{DeviceID: p.id, Name: "Alex Laptop", Platform: identity.PlatformMacOS, DPoPKey: p.pub}
	res, err := w.id.Logins.CompleteLogin(context.Background(), app.LoginCompletion{
		UserID: user, AuthMethod: identity.AuthPasskey, Device: reg, ProofJKT: p.jkt, Trust: app.TrustUnconfirmed,
	})
	if err != nil {
		w.t.Fatal(err)
	}
	if res.Device.State != identity.DeviceStatePending || len(res.Tokens.Device.VerificationCode) != 6 {
		w.t.Fatalf("login left device %s with code %q", res.Device.State, res.Tokens.Device.VerificationCode)
	}
	c := &client{w: w, key: p.key, user: user, device: p.id}
	c.take(res.Tokens)
	return &waiting{client: c, code: res.Tokens.Device.VerificationCode, reg: reg, jkt: p.jkt}
}

// relogin starts a second session on the known pending device of d.
func (w *sworld) relogin(d *waiting) *client {
	w.t.Helper()
	res, err := w.id.Logins.CompleteLogin(context.Background(), app.LoginCompletion{
		UserID: d.user, AuthMethod: identity.AuthPasskey, Device: d.reg, ProofJKT: d.jkt, Trust: app.TrustUnconfirmed,
	})
	if err != nil || res.NewDevice {
		w.t.Fatalf("second login: %v, new device %v", err, res.NewDevice)
	}
	c := &client{w: w, key: d.key, user: d.user, device: d.device}
	c.take(res.Tokens)
	return c
}

// approve sends approveDevice for dev with code from c.
func (c *client) approve(dev ids.ID, code string) *httpservertest.Response {
	c.w.t.Helper()
	body, _ := json.Marshal(map[string]string{"verification_code": code})
	// Clients send an Idempotency-Key (the operation is keyed), so the stage runs the use case
	// inside its own unit of work (red team M1-38 round 1).
	return c.callH(http.MethodPost, approvePath(dev), string(body), "Idempotency-Key", ids.New().String())
}

// wrong returns a code of the same length that differs from code.
func wrong(code string) string {
	last := code[len(code)-1]
	if last == '9' {
		return code[:len(code)-1] + "0"
	}
	return code[:len(code)-1] + string(last+1)
}

// deviceRow returns state and revoke_reason ("" for none) of the device id.
func (w *sworld) deviceRow(id ids.ID) (string, string) {
	w.t.Helper()
	var state, reason string
	if err := w.owner.QueryRow(context.Background(), `SELECT state, coalesce(revoke_reason, '') FROM devices WHERE id = $1`, id.String()).Scan(&state, &reason); err != nil {
		w.t.Fatal(err)
	}
	return state, reason
}

func caseIDT12PendingDeviceHasTokensButNoAccess(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	p := w.pendingLogin(alex)
	w.want(p.call(http.MethodGet, "/api/v1/me", ""), http.StatusOK, "")
	own := w.want(p.call(http.MethodGet, devicePath(p.device), ""), http.StatusOK, "")
	if own["state"] != "pending" || own["verification_code"] != p.code {
		t.Fatalf("own device %v, want pending with code %s", own, p.code)
	}
	w.want(p.call(http.MethodGet, "/api/v1/users", ""), http.StatusForbidden, "device_pending")
	w.want(p.call(http.MethodGet, devicePath(phone.device), ""), http.StatusForbidden, "device_pending")
	p.rotate()
	r := p.call(http.MethodPost, "/api/v1/auth/logout", "")
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("logout %d: %s", r.StatusCode, r.Body)
	}
}

func caseVerificationCodeOnlyForPendingDeviceItself(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	owner := w.phone(w.person(w.handle("sam"), true))
	phone := w.phone(alex)
	p := w.pendingLogin(alex)
	for name, c := range map[string]*client{"owner": owner.client, "other device": phone.client} {
		m := w.want(c.call(http.MethodGet, devicePath(p.device), ""), http.StatusOK, "")
		if _, ok := m["verification_code"]; ok || m["state"] != "pending" {
			t.Fatalf("%s reads %v", name, m)
		}
	}
	list := w.want(owner.call(http.MethodGet, "/api/v1/users/"+alex.String()+"/devices", ""), http.StatusOK, "")
	mine := w.want(phone.call(http.MethodGet, "/api/v1/me/devices", ""), http.StatusOK, "")
	for _, page := range []map[string]any{list, mine} {
		for _, it := range page["items"].([]any) {
			if _, ok := it.(map[string]any)["verification_code"]; ok {
				t.Fatalf("code in a list of another device: %v", it)
			}
		}
	}
	// The listMyDevices of the pending device itself is device_pending (only its own device is
	// readable, 05 section 2.5); getDevice of its own entry carries the code.
	w.want(p.call(http.MethodGet, "/api/v1/me/devices", ""), http.StatusForbidden, "device_pending")
	if m := w.want(p.call(http.MethodGet, devicePath(p.device), ""), http.StatusOK, ""); m["verification_code"] != p.code {
		t.Fatalf("own device %v", m)
	}
}

// wantApproved checks that dev is active after an approval by approver and that its former
// pending client has full access.
func (w *sworld) wantApproved(t *testing.T, r *httpservertest.Response, p *waiting, approver ids.ID) {
	t.Helper()
	m := w.want(r, http.StatusOK, "")
	if m["state"] != "active" || m["approved_at"] == nil {
		t.Fatalf("answer %v", m)
	}
	if _, ok := m["verification_code"]; ok {
		t.Fatal("verification code in the answer to the approver")
	}
	var by string
	w.scalar(&by, `SELECT approved_by::text FROM devices WHERE id = $1`, p.device.String())
	if by != approver.String() {
		t.Fatalf("approved_by %s, want %s", by, approver)
	}
	e := w.auditsOf("device.approve", p.device.String())
	if len(e) != 1 || e[0].Decision != "allow" || e[0].Details["result"] != "approved" || e[0].Details["device_id"] != p.device.String() {
		t.Fatalf("audit %+v", e)
	}
	w.want(p.call(http.MethodGet, "/api/v1/users", ""), http.StatusOK, "")
}

func caseApproveDeviceBySelfFromActiveDevice(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	p := w.pendingLogin(alex)
	phone.stepUp()
	w.wantApproved(t, phone.approve(p.device, p.code), p, alex)
}

func caseApproveDeviceByOwner(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	p := w.pendingLogin(alex)
	owner := w.ownerWithStepUp()
	w.wantApproved(t, owner.approve(p.device, p.code), p, owner.user)
}

func caseIDT13WrongCodeRevokesDevice(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	p := w.pendingLogin(alex)
	second := w.relogin(p)
	phone.stepUp()
	m := w.want(phone.approve(p.device, wrong(p.code)), http.StatusOK, "")
	if m["state"] != "revoked" || m["revoke_reason"] != "verification_failed" {
		t.Fatalf("answer %v", m)
	}
	state, reason := w.deviceRow(p.device)
	if state != "revoked" || reason != "verification_failed" {
		t.Fatalf("device %s %s", state, reason)
	}
	if n := w.count(`SELECT count(*) FROM devices WHERE id = $1 AND revoked_by = $2 AND revoked_at IS NOT NULL`, p.device.String(), alex.String()); n != 1 {
		t.Fatal("revoked_by or revoked_at not set")
	}
	if n := w.count(`SELECT count(*) FROM device_keys WHERE device_id = $1 AND revoked_at IS NULL`, p.device.String()); n != 0 {
		t.Fatalf("%d keys still active", n)
	}
	for _, sid := range []ids.ID{p.session, second.session} {
		if got := w.sessionState(sid); got != "device_revoked" {
			t.Fatalf("session %s: %q", sid, got)
		}
		if reason, _ := w.familyState(sid); reason != "session_revoked" {
			t.Fatalf("family of %s: %q", sid, reason)
		}
	}
	var events []map[string]string
	for _, ev := range w.revokedEvents() {
		if ev["resource"] == p.session.String() || ev["resource"] == second.session.String() {
			events = append(events, ev)
		}
	}
	if len(events) != 2 || events[0]["actor"] != alex.String() || events[0]["audience"] != alex.String() {
		t.Fatalf("session.revoked %v, want one per session", events)
	}
	e := w.auditsOf("device.approve", p.device.String())
	if len(e) != 1 || e[0].Decision != "allow" || e[0].Details["result"] != "rejected_wrong_code" {
		t.Fatalf("audit %+v", e)
	}
	w.want(p.call(http.MethodGet, "/api/v1/me", ""), http.StatusUnauthorized, "session_revoked")
	w.want(phone.approve(p.device, p.code), http.StatusConflict, "invalid_state_transition")
}

func caseApproveDeviceNeedsStepUp(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	p := w.pendingLogin(alex)
	w.want(phone.approve(p.device, p.code), http.StatusUnauthorized, "step_up_required")
	if state, _ := w.deviceRow(p.device); state != "pending" {
		t.Fatalf("device %s", state)
	}
}

func caseApproveDeviceFromPendingDevice(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	p := w.pendingLogin(alex)
	other := w.pendingLogin(alex)
	w.want(other.approve(p.device, p.code), http.StatusForbidden, "device_pending")
	w.want(p.approve(p.device, p.code), http.StatusForbidden, "device_pending")
}

func caseApproveForeignDeviceByNonOwner(t *testing.T, w *sworld) {
	p := w.pendingLogin(w.person(w.handle("alex"), false))
	sam := w.phone(w.person(w.handle("sam"), false))
	sam.stepUp()
	w.want(sam.approve(p.device, p.code), http.StatusNotFound, "not_found")
	w.want(sam.approve(ids.New(), p.code), http.StatusNotFound, "not_found")
	if state, _ := w.deviceRow(p.device); state != "pending" {
		t.Fatalf("device %s", state)
	}
}

func caseApproveActiveDevice(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	other := w.phone(alex)
	phone.stepUp()
	var before time.Time
	w.scalar(&before, `SELECT approved_at FROM devices WHERE id = $1`, other.device.String())
	m := w.want(phone.approve(other.device, "000000"), http.StatusOK, "")
	var after time.Time
	w.scalar(&after, `SELECT approved_at FROM devices WHERE id = $1`, other.device.String())
	if m["state"] != "active" || !after.Equal(before) {
		t.Fatalf("answer %v, approved_at %v -> %v", m, before, after)
	}
}

func caseApproveRevokedDevice(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	p := w.pendingLogin(alex)
	phone.stepUp()
	w.want(phone.approve(p.device, wrong(p.code)), http.StatusOK, "")
	w.want(phone.approve(p.device, p.code), http.StatusConflict, "invalid_state_transition")
}

func TestPendingDeviceExpires(t *testing.T) {
	t.Parallel()
	w := newApprovalWorld(t)
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	p := w.pendingLogin(alex)
	idle := w.pendingLogin(alex)
	// Both devices waited auth.device_pending_ttl (moved in the row, so the access tokens of the
	// world stay valid and every 401 below is the device's).
	w.sql(`UPDATE devices SET created_at = created_at - make_interval(secs => $2) WHERE id = ANY($1::uuid[])`,
		[]string{p.device.String(), idle.device.String()}, sessCfg.DevicePendingTTL.Seconds())
	// Authentication treats the device as revoked before any job ran.
	w.want(p.call(http.MethodGet, "/api/v1/me", ""), http.StatusUnauthorized, "session_revoked")
	w.want(p.refreshWith(p.refresh, p.key), http.StatusUnauthorized, "session_revoked")
	if state, _ := w.deviceRow(idle.device); state != "pending" {
		t.Fatalf("idle device %s before the cleanup", state)
	}
	phone.stepUp()
	w.want(phone.approve(p.device, p.code), http.StatusConflict, "invalid_state_transition")
	if state, reason := w.deviceRow(p.device); state != "revoked" || reason != "pending_expired" {
		t.Fatalf("device after the approval %s %s", state, reason)
	}
	if _, err := w.id.AuthCleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if state, reason := w.deviceRow(idle.device); state != "revoked" || reason != "pending_expired" {
		t.Fatalf("idle device after the cleanup %s %s", state, reason)
	}
	if got := w.sessionState(idle.session); got != "device_revoked" {
		t.Fatalf("session of the idle device %q", got)
	}
	e := w.audits("device.expire")
	if len(e) != 2 || e[1].ResourceID != idle.device.String() || e[1].Details["revoke_reason"] != "pending_expired" {
		t.Fatalf("audit %+v", e)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'device.expire' AND actor_principal_id IS NULL`); n != 2 {
		t.Fatalf("%d device.expire entries without actor, want 2", n)
	}
	// A repeated run is harmless.
	if _, err := w.id.AuthCleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(w.audits("device.expire")) != 2 {
		t.Fatal("a second run audited again")
	}
}

// pairingBody is the PairingBeginRequest of p with a step-up key.
func (p *newPhone) pairingBody(step identity.PublicKey) string {
	b, _ := json.Marshal(map[string]any{"device": map[string]any{
		"device_id": p.id.String(), "name": "Alex Laptop", "platform": "macos", "public_jwk": json.RawMessage(p.pub.JWK), "alg": "ES256",
		"step_up_key": map[string]any{"public_jwk": json.RawMessage(step.JWK), "alg": "ES256"},
	}})
	return string(b)
}

// publicCall sends a request without access token with a DPoP proof of key to path.
func (w *sworld) publicCall(key *identitytest.Key, path, body string, headers ...string) *httpservertest.Response {
	w.t.Helper()
	proof := identitytest.SignDPoPProof(key, identitytest.ProofClaims{
		HTM: http.MethodPost, HTU: httpservertest.PublicURL + path, IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(),
	})
	return w.h.Do(w.h.NewRequest(http.MethodPost, path, body, append([]string{"DPoP", proof}, headers...)...))
}

// pairing is a new device with its open pairing.
type pairing struct {
	*newPhone
	id, code string
}

// beginPairing starts a pairing of a new device and expects 201.
func (w *sworld) beginPairing() *pairing {
	w.t.Helper()
	p := w.phoneKeys()
	_, step := identitytest.NewDeviceKey(jose.AlgES256)
	m := w.want(w.publicCall(p.key, pairingBeginPath, p.pairingBody(step)), http.StatusCreated, "")
	code, _ := m["pairing_code"].(string)
	if !pairingForm.MatchString(code) {
		w.t.Fatalf("pairing code %q", code)
	}
	return &pairing{newPhone: p, id: m["pairing_id"].(string), code: code}
}

// complete sends completePairing for pr signed by key (pr's own key when nil).
func (w *sworld) complete(pr *pairing, key *identitytest.Key) *httpservertest.Response {
	w.t.Helper()
	if key == nil {
		key = pr.key
	}
	return w.publicCall(key, pairingCompletePath, `{"pairing_id":"`+pr.id+`"}`)
}

// approvePairing sends approvePairing with code from c.
func (c *client) approvePairing(code string) *httpservertest.Response {
	c.w.t.Helper()
	return c.callH(http.MethodPost, pairingApprovePath, `{"pairing_code":"`+code+`"}`, "Idempotency-Key", ids.New().String())
}

// approvePairingWithoutKey sends approvePairing without Idempotency-Key.
func (c *client) approvePairingWithoutKey(code string) *httpservertest.Response {
	c.w.t.Helper()
	return c.call(http.MethodPost, pairingApprovePath, `{"pairing_code":"`+code+`"}`)
}

// pairingStatus returns the status of the pairing id.
func (w *sworld) pairingStatus(id string) string {
	w.t.Helper()
	var s string
	w.scalar(&s, `SELECT status FROM device_pairings WHERE id = $1`, id)
	return s
}

func casePairingHappyPath(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	pr := w.beginPairing()
	m := w.want(w.complete(pr, nil), http.StatusAccepted, "")
	if m["status"] != "requested" || m["pairing_id"] != pr.id {
		t.Fatalf("status %v", m)
	}
	phone.stepUp()
	// The code is accepted in lower case and without the hyphen.
	dev := w.want(phone.approvePairing(strings.ToLower(strings.ReplaceAll(pr.code, "-", ""))), http.StatusCreated, "")
	if dev["state"] != "active" || dev["user_id"] != alex.String() || dev["id"] != pr.id2() || dev["step_up_key_thumbprint"] == nil {
		t.Fatalf("device %v", dev)
	}
	tok := w.want(w.complete(pr, nil), http.StatusCreated, "")
	sess := tok["session"].(map[string]any)
	if sess["auth_method"] != "device_pairing" || tok["device"].(map[string]any)["state"] != "active" {
		t.Fatalf("tokens %v", tok)
	}
	if st := w.pairingStatus(pr.id); st != "completed" {
		t.Fatalf("pairing %s", st)
	}
	if n := w.count(`SELECT count(*) FROM device_pairings WHERE id = $1 AND user_id = $2 AND approved_by_device_id = $3 AND device_id = $4
		AND completed_at IS NOT NULL`, pr.id, alex.String(), phone.device.String(), pr.newPhone.id.String()); n != 1 {
		t.Fatal("pairing row incomplete")
	}
	e := w.auditsOf("device.approve", pr.id2())
	if len(e) != 1 || e[0].Details["pairing_id"] != pr.id || e[0].Details["device_id"] != pr.id2() {
		t.Fatalf("audit %+v", e)
	}
	c := &client{w: w, key: pr.key, user: alex, device: pr.newPhone.id, access: tok["access_token"].(string)}
	w.want(c.call(http.MethodGet, "/api/v1/users", ""), http.StatusOK, "")
	// Collected once: a further completion is gone.
	w.want(w.complete(pr, nil), http.StatusGone, "gone")
}

// id2 is the device id of the pairing as text.
func (pr *pairing) id2() string { return pr.newPhone.id.String() }

func casePairingWrongCodeEndsStepUp(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	pr := w.beginPairing()
	phone.stepUp()
	// A malformed code is no guess: the step-up stays.
	w.want(phone.approvePairing("UUUU-UUUU"), http.StatusBadRequest, "validation_failed")
	unknown := "0000-0000"
	if pr.code == unknown {
		unknown = "1111-1111"
	}
	denials := w.count(`SELECT count(*) FROM audit_events WHERE action = 'device.approve' AND decision = 'deny'`)
	w.want(phone.approvePairing(unknown), http.StatusNotFound, "not_found")
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE id = $1 AND step_up_at IS NULL AND step_up_method IS NULL`, phone.session.String()); n != 1 {
		t.Fatal("step-up of the session not ended")
	}
	e := w.audits("device.approve")
	if len(e) == 0 || e[len(e)-1].Decision != "deny" || e[len(e)-1].Reason != "no_grant" || e[len(e)-1].Details["result"] != "pairing_code_unknown" ||
		w.count(`SELECT count(*) FROM audit_events WHERE action = 'device.approve' AND decision = 'deny'`) != denials+1 {
		t.Fatalf("audit %+v", e)
	}
	w.want(phone.approvePairing(pr.code), http.StatusUnauthorized, "step_up_required")
	if st := w.pairingStatus(pr.id); st != "requested" {
		t.Fatalf("pairing %s", st)
	}
	// Without an Idempotency-Key the use case runs its own units: the same answer.
	phone.stepUp()
	w.want(phone.approvePairingWithoutKey(unknown), http.StatusNotFound, "not_found")
	w.want(phone.approvePairingWithoutKey(pr.code), http.StatusUnauthorized, "step_up_required")
	phone.stepUp()
	w.want(phone.approvePairing(pr.code), http.StatusCreated, "")
}

func casePairingCompleteWithOtherKeyRejects(t *testing.T, w *sworld) {
	pr := w.beginPairing()
	other, _ := identitytest.NewDeviceKey(jose.AlgES256)
	w.want(w.complete(pr, other), http.StatusUnauthorized, "invalid_dpop_proof")
	if st := w.pairingStatus(pr.id); st != "rejected" {
		t.Fatalf("pairing %s", st)
	}
	w.want(w.complete(pr, nil), http.StatusGone, "gone")
	// The code of a rejected pairing matches nothing.
	phone := w.phone(w.person(w.handle("alex"), false))
	phone.stepUp()
	w.want(phone.approvePairing(pr.code), http.StatusNotFound, "not_found")
}

func casePairingExpiry(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	// Requested past its end: gone on access.
	requested := w.beginPairing()
	w.clk.Advance(sessCfg.PairingCodeTTL)
	w.want(w.complete(requested, nil), http.StatusGone, "gone")
	if st := w.pairingStatus(requested.id); st != "expired" {
		t.Fatalf("requested pairing %s", st)
	}
	phone.stepUp()
	w.want(phone.approvePairing(requested.code), http.StatusNotFound, "not_found")
	// Approved but never collected: the cleanup expires it and revokes the device.
	phone.stepUp()
	approved := w.beginPairing()
	w.want(phone.approvePairing(approved.code), http.StatusCreated, "")
	lazy := w.beginPairing()
	phone.stepUp()
	w.want(phone.approvePairing(lazy.code), http.StatusCreated, "")
	w.clk.Advance(sessCfg.PairingCodeTTL)
	expires := w.count(`SELECT count(*) FROM audit_events WHERE action = 'device.expire' AND actor_principal_id IS NULL`)
	if _, err := w.id.AuthCleanup.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := w.pairingStatus(approved.id); st != "expired" {
		t.Fatalf("approved pairing %s", st)
	}
	if state, reason := w.deviceRow(approved.newPhone.id); state != "revoked" || reason != "pairing_expired" {
		t.Fatalf("device %s %s", state, reason)
	}
	w.want(w.complete(approved, nil), http.StatusGone, "gone")
	if n := w.count(`SELECT count(*) FROM devices WHERE user_id = $1 AND state = 'active'`, alex.String()); n != 1 {
		t.Fatalf("%d active devices, want only the phone", n)
	}
	if n := w.count(`SELECT count(*) FROM audit_events WHERE action = 'device.expire' AND actor_principal_id IS NULL`); n != expires+2 {
		t.Fatalf("%d device.expire entries, want %d", n, expires+2)
	}
}

func casePairingExpiryOnAccess(t *testing.T, w *sworld) {
	phone := w.phone(w.person(w.handle("alex"), false))
	pr := w.beginPairing()
	phone.stepUp()
	w.want(phone.approvePairing(pr.code), http.StatusCreated, "")
	w.clk.Advance(sessCfg.PairingCodeTTL)
	// The job did not run: the completion itself expires the pairing and revokes the device.
	w.want(w.complete(pr, nil), http.StatusGone, "gone")
	if state, reason := w.deviceRow(pr.newPhone.id); state != "revoked" || reason != "pairing_expired" {
		t.Fatalf("device %s %s", state, reason)
	}
}

// rateLimited returns a harness on the routes of w with the rate limit of the chain, whose class
// rl.public_auth allows burst requests per client address.
func (w *sworld) rateLimited(burst int) *httpservertest.Harness {
	w.t.Helper()
	classes := map[ratelimit.Class]ratelimit.Limits{}
	for _, c := range ratelimit.Classes() {
		classes[c] = ratelimit.Limits{PerMinute: 1000, Burst: 1000}
	}
	classes["rl.public_auth"] = ratelimit.Limits{PerMinute: 1, Burst: burst}
	l, err := ratelimit.New(ratelimit.Config{Classes: classes, MaxTrackedKeys: 1000, SweepInterval: time.Minute}, w.clk, nil)
	if err != nil {
		w.t.Fatal(err)
	}
	return httpservertest.New(w.t, httpservertest.Options{
		Routes: w.routes, Contract: rest.Spec, Authenticator: w.id.Authenticator, ValidateResponses: true,
		Stages: map[httpserver.Stage]func(http.Handler) http.Handler{httpserver.StageIdempotency: w.idempotency},
		Setup:  func(c *httpserver.Chain) { ratelimit.Install(c, l) },
	})
}

func casePairingBeginRateLimitedPerIP(t *testing.T, w *sworld) {
	h := w.rateLimited(3)
	begin := func(ip string) *httpservertest.Response {
		p := w.phoneKeys()
		_, step := identitytest.NewDeviceKey(jose.AlgES256)
		proof := identitytest.SignDPoPProof(p.key, identitytest.ProofClaims{
			HTM: http.MethodPost, HTU: httpservertest.PublicURL + pairingBeginPath, IAT: w.clk.Now(), Nonce: w.id.DPoP.CurrentNonce(),
		})
		return h.Do(h.NewRequest(http.MethodPost, pairingBeginPath, p.pairingBody(step), "DPoP", proof, "X-Forwarded-For", ip))
	}
	for i := range 3 {
		if r := begin("192.0.2.1"); r.StatusCode != http.StatusCreated {
			t.Fatalf("request %d: %d %s", i, r.StatusCode, r.Body)
		}
	}
	if r := begin("192.0.2.1"); r.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("request above the burst: %d %s", r.StatusCode, r.Body)
	}
	if r := begin("192.0.2.2"); r.StatusCode != http.StatusCreated {
		t.Fatalf("another address: %d %s", r.StatusCode, r.Body)
	}
}

func casePairingCodeNeverLogged(t *testing.T, w *sworld) {
	alex := w.person(w.handle("alex"), false)
	phone := w.phone(alex)
	p := w.pendingLogin(alex)
	pr := w.beginPairing()
	phone.stepUp()
	w.want(phone.approvePairing(pr.code), http.StatusCreated, "")
	w.want(w.complete(pr, nil), http.StatusCreated, "")
	phone.stepUp()
	w.want(phone.approve(p.device, p.code), http.StatusOK, "")
	canonical := strings.ReplaceAll(pr.code, "-", "")
	secrets := []string{pr.code, canonical, p.code}
	for _, logs := range []*httpservertest.LogCapture{w.logs, w.h.Logs} {
		for _, r := range logs.Records() {
			text := r.Message
			for k, v := range r.Attrs {
				text += " " + k + "=" + v
			}
			for _, s := range secrets {
				if strings.Contains(text, s) {
					t.Fatalf("log record %q contains a code", text)
				}
			}
		}
	}
	var details string
	w.scalar(&details, `SELECT coalesce(string_agg(details::text, ' '), '') FROM audit_events`)
	var stored string
	w.scalar(&stored, `SELECT coalesce(string_agg(device_registration::text || encode(pairing_code_hash, 'escape'), ' '), '') FROM device_pairings`)
	for _, s := range secrets {
		if strings.Contains(details, s) || strings.Contains(stored, s) {
			t.Fatalf("code %q stored in an audit entry or a pairing row", s)
		}
	}
}
