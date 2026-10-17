// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/adapters/httpapi"
	"github.com/OWNER/bothy/services/server/internal/identity/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// noDPoP is a DPoP verifier the revocation use cases never call.
type noDPoP struct{ identity.DPoP }

// lifeWorld drives the revocation use cases directly over PostgreSQL 18 with the real
// authorizer, rights hooks, sync and audit of peopleWorld (the HTTP tests in adapters/httpapi run
// the same through the chain with real tokens).
type lifeWorld struct {
	*peopleWorld
	l    *app.Lifecycle
	cfg  app.Config
	deps app.LifecycleDeps
}

func newLifeWorld(t *testing.T) *lifeWorld {
	t.Helper()
	w := newPeopleWorld(t)
	cfg := peopleCfg
	cfg.RefreshTokenTTL, cfg.SessionMaxAge = time.Hour, 24*time.Hour
	cfg.APITokenMaxTTL, cfg.PersonalTokenMaxTTL = 90*24*time.Hour, 30*24*time.Hour
	cfg.DPoPRequiredFor, cfg.DeviceInactiveRevokeAfter = []string{app.DPoPForAgent}, 180*24*time.Hour
	cfg.DevicePendingTTL = 24 * time.Hour // required by the sessions since M1-38 (INT-W14/9)
	sessions, err := app.NewSessions(cfg, tokenPepper, app.SessionsDeps{
		Store: w.repo, Devices: w.repo, People: w.repo, Tokens: &app.AccessTokens{}, DPoP: noDPoP{}, UoW: w.uow,
		Authorizer: w.az, Audit: w.rec, Events: w.appender, Cursors: httpapi.Cursors{}, Clock: w.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := app.NewTokens(cfg, tokenPepper, app.TokensDeps{
		Store: w.repo, Principals: w.repo, UoW: w.uow, Authorizer: w.az, Audit: w.rec, Cursors: httpapi.Cursors{},
		Switch: personalOff{}, Clock: w.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	deps := app.LifecycleDeps{Store: w.repo, People: w.p, Sessions: sessions, Tokens: tokens, Clock: w.clk}
	l, err := app.NewLifecycle(cfg, deps)
	if err != nil {
		t.Fatal(err)
	}
	return &lifeWorld{peopleWorld: w, l: l, cfg: cfg, deps: deps}
}

// device inserts a device of user in state with a dpop key, created at seen and, when active,
// approved and last seen at seen.
func (w *lifeWorld) device(user ids.ID, state string, seen time.Time) ids.ID {
	w.t.Helper()
	id := ids.New()
	var approvedBy, approvedAt any
	if state == "active" {
		approvedBy, approvedAt = user.String(), seen
	}
	w.sql(`INSERT INTO devices (id, user_id, name, platform, state, approved_by, approved_at, last_seen_at, created_at)
		VALUES ($1, $2, 'Phone', 'ios', $3, $4, $5, $5, $6)`, id.String(), user.String(), state, approvedBy, approvedAt, seen)
	w.sql(`INSERT INTO device_keys (id, device_id, purpose, jkt, public_jwk, alg) VALUES ($1, $2, 'dpop', $3, '{}', 'ES256')`,
		ids.New().String(), id.String(), ids.New().String())
	return id
}

// session inserts a live session of user on device (a share_ext child of parent when parent is
// not zero) with one live refresh family.
func (w *lifeWorld) session(user, device, parent ids.ID) ids.ID {
	w.t.Helper()
	id := ids.New()
	component, p := "app", any(nil)
	if !parent.IsZero() {
		component, p = "share_ext", parent.String()
	}
	w.sql(`INSERT INTO auth_sessions (id, user_id, device_id, client_component, parent_session_id, auth_method, expires_at)
		VALUES ($1, $2, $3, $4, $5, 'passkey', $6)`, id.String(), user.String(), device.String(), component, p, peopleStart.Add(24*time.Hour))
	w.sql(`INSERT INTO refresh_token_families (id, session_id, dpop_jkt) VALUES ($1, $2, 'jkt')`, ids.New().String(), id.String())
	return id
}

// personalToken inserts a live personal API token of user.
func (w *lifeWorld) personalToken(user ids.ID) {
	w.t.Helper()
	id := ids.New()
	w.sql(`INSERT INTO api_tokens (id, principal_id, name, token_prefix, token_hash, expires_at, created_by)
		VALUES ($1, $2, 'cli', 'bothy_pat_x', $3, $4, $2)`, id.String(), user.String(), id[:], peopleStart.Add(24*time.Hour))
}

// count returns the result of a count query.
func (w *lifeWorld) count(query string, args ...any) int {
	w.t.Helper()
	var n int
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	return n
}

// text returns the text result of query.
func (w *lifeWorld) text(query string, args ...any) string {
	w.t.Helper()
	var s *string
	if err := w.owner.QueryRow(context.Background(), query, args...).Scan(&s); err != nil {
		w.t.Fatalf("%s: %v", query, err)
	}
	if s == nil {
		return ""
	}
	return *s
}

// TestLifecycle_DisableUserUseCase (M1-45, 08 section 11): the owner disables a person: status,
// sessions with the child, families, devices including the pending one, keys and personal tokens
// revoked in one rights change with one session.revoked per session; a repeat is without effect,
// an unknown person is not_found, a person without owner role is refused, a deleted person is an
// invalid transition and the last owner cannot disable themselves.
func TestLifecycle_DisableUserUseCase(t *testing.T) {
	t.Parallel()
	w := newLifeWorld(t)
	o := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	d1, d2 := w.device(alex.ID, "active", peopleStart), w.device(alex.ID, "pending", peopleStart)
	s1 := w.session(alex.ID, d1, ids.ID{})
	w.session(alex.ID, d1, s1)
	w.session(alex.ID, d2, ids.ID{})
	w.personalToken(alex.ID)
	ctx := ctxOf(o)

	got, err := w.l.DisableUser(ctx, identity.DisableUserCmd{Actor: o, UserID: alex.ID, Reason: "Left"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "disabled" {
		t.Errorf("answer %+v", got)
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE user_id = $1 AND revoke_reason = 'user_disabled'`, alex.ID.String()); n != 3 {
		t.Errorf("revoked sessions %d", n)
	}
	if n := w.count(`SELECT count(*) FROM refresh_token_families f JOIN auth_sessions s ON s.id = f.session_id
		WHERE s.user_id = $1 AND f.revoke_reason = 'session_revoked'`, alex.ID.String()); n != 3 {
		t.Errorf("revoked families %d", n)
	}
	if n := w.count(`SELECT count(*) FROM devices WHERE user_id = $1 AND state = 'revoked' AND revoke_reason = 'user_disabled'`, alex.ID.String()); n != 2 {
		t.Errorf("revoked devices %d", n)
	}
	if n := w.count(`SELECT count(*) FROM device_keys k JOIN devices d ON d.id = k.device_id WHERE d.user_id = $1 AND k.revoked_at IS NULL`, alex.ID.String()); n != 0 {
		t.Errorf("live keys %d", n)
	}
	if n := w.count(`SELECT count(*) FROM api_tokens WHERE principal_id = $1 AND revoked_at IS NULL`, alex.ID.String()); n != 0 {
		t.Errorf("live personal tokens %d", n)
	}
	if n := w.count(`SELECT count(*) FROM event_log WHERE type = 'session.revoked'`); n != 3 {
		t.Errorf("session.revoked events %d", n)
	}

	// A repeat changes nothing.
	again, err := w.l.DisableUser(ctx, identity.DisableUserCmd{Actor: o, UserID: alex.ID, Reason: "Left"})
	if err != nil || again.Status != "disabled" {
		t.Errorf("repeat %+v, %v", again, err)
	}
	_, err = w.l.DisableUser(ctx, identity.DisableUserCmd{Actor: o, UserID: ids.New(), Reason: "Left"})
	wantCode(t, err, errs.CodeNotFound)

	sam := w.person("sam", "active", false)
	_, err = w.l.DisableUser(ctxOf(sam), identity.DisableUserCmd{Actor: sam, UserID: o.ID, Reason: "x"})
	if err == nil {
		t.Error("a person without owner role disabled the owner")
	}
	gone := w.person("gone", "deleted", false)
	_, err = w.l.DisableUser(ctx, identity.DisableUserCmd{Actor: o, UserID: gone.ID, Reason: "x"})
	wantCode(t, err, errs.CodeInvalidStateTransition)
	_, err = w.l.DisableUser(ctx, identity.DisableUserCmd{Actor: o, UserID: o.ID, Reason: "x"})
	wantCode(t, err, errs.CodeLastOwner)
	if s := w.text(`SELECT status FROM principals WHERE id = $1`, o.ID.String()); s != "active" {
		t.Errorf("the last owner is %s", s)
	}
	// The caller must be the principal of the request.
	if _, err := w.l.DisableUser(ctxOf(sam), identity.DisableUserCmd{Actor: o, UserID: alex.ID}); err == nil {
		t.Error("a caller other than the request's principal passed")
	}
}

// TestLifecycle_RevokeDeviceUseCase (M1-45, 08 section 10): a person revokes the own device
// (revoked_by_self, sessions device_revoked), the owner a device of another person
// (revoked_by_owner); a repeat is without effect and an unknown device is not_found.
func TestLifecycle_RevokeDeviceUseCase(t *testing.T) {
	t.Parallel()
	w := newLifeWorld(t)
	o := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	d1, d2 := w.device(alex.ID, "active", peopleStart), w.device(alex.ID, "active", peopleStart)
	s1 := w.session(alex.ID, d1, ids.ID{})
	w.session(alex.ID, d1, s1)

	got, err := w.l.RevokeDevice(ctxOf(alex), identity.RevokeDeviceCmd{Actor: alex, DeviceID: d1, Reason: "Lost"})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != identity.DeviceStateRevoked || got.RevokeReason != "revoked_by_self" {
		t.Errorf("answer %+v", got)
	}
	if n := w.count(`SELECT count(*) FROM auth_sessions WHERE device_id = $1 AND revoke_reason = 'device_revoked'`, d1.String()); n != 2 {
		t.Errorf("revoked sessions %d", n)
	}
	if again, err := w.l.RevokeDevice(ctxOf(alex), identity.RevokeDeviceCmd{Actor: alex, DeviceID: d1, Reason: "Lost"}); err != nil || again.State != identity.DeviceStateRevoked {
		t.Errorf("repeat %+v, %v", again, err)
	}
	got, err = w.l.RevokeDevice(ctxOf(o), identity.RevokeDeviceCmd{Actor: o, DeviceID: d2, Reason: "Policy"})
	if err != nil || got.RevokeReason != "revoked_by_owner" {
		t.Errorf("owner revoke %+v, %v", got, err)
	}
	_, err = w.l.RevokeDevice(ctxOf(o), identity.RevokeDeviceCmd{Actor: o, DeviceID: ids.New(), Reason: "x"})
	wantCode(t, err, errs.CodeNotFound)
	if _, err := w.l.RevokeDevice(ctxOf(o), identity.RevokeDeviceCmd{Actor: alex, DeviceID: d2}); err == nil {
		t.Error("a caller other than the request's principal passed")
	}
}

// TestLifecycle_RevokeSessionUseCase (M1-45, 05 session.revoke): the own session and its child
// end with logout, a session ended by the owner with admin; a repeat is without effect and an
// unknown session is not_found.
func TestLifecycle_RevokeSessionUseCase(t *testing.T) {
	t.Parallel()
	w := newLifeWorld(t)
	o := w.person("owner", "active", true)
	alex := w.person("alex", "active", false)
	d := w.device(alex.ID, "active", peopleStart)
	s1, s2 := w.session(alex.ID, d, ids.ID{}), w.session(alex.ID, d, ids.ID{})
	child := w.session(alex.ID, d, s1)

	if err := w.l.RevokeSession(ctxOf(alex), identity.RevokeSessionCmd{Actor: alex, SessionID: s1}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []ids.ID{s1, child} {
		if r := w.text(`SELECT revoke_reason FROM auth_sessions WHERE id = $1`, s.String()); r != "logout" {
			t.Errorf("session %s reason %q", s, r)
		}
	}
	if err := w.l.RevokeSession(ctxOf(alex), identity.RevokeSessionCmd{Actor: alex, SessionID: s1}); err != nil {
		t.Errorf("repeat %v", err)
	}
	if err := w.l.RevokeSession(ctxOf(o), identity.RevokeSessionCmd{Actor: o, SessionID: s2}); err != nil {
		t.Fatal(err)
	}
	if r := w.text(`SELECT revoke_reason FROM auth_sessions WHERE id = $1`, s2.String()); r != "admin" {
		t.Errorf("owner revoke reason %q", r)
	}
	wantCode(t, w.l.RevokeSession(ctxOf(o), identity.RevokeSessionCmd{Actor: o, SessionID: ids.New()}), errs.CodeNotFound)
	if err := w.l.RevokeSession(ctxOf(o), identity.RevokeSessionCmd{Actor: alex, SessionID: s2}); err == nil {
		t.Error("a caller other than the request's principal passed")
	}
}

// TestLifecycle_ExpireInactiveDevicesUseCase (M1-45, 08 section 5.4): the sweep revokes the
// active devices without contact for auth.device_inactive_revoke_after with reason inactive and
// no actor, ends their sessions with reason inactive and leaves recent devices alone.
func TestLifecycle_ExpireInactiveDevicesUseCase(t *testing.T) {
	t.Parallel()
	w := newLifeWorld(t)
	w.person("owner", "active", true) // every rights change keeps an active owner (SR8)
	alex := w.person("alex", "active", false)
	old := w.device(alex.ID, "active", peopleStart.Add(-200*24*time.Hour))
	recent := w.device(alex.ID, "active", peopleStart.Add(-time.Hour))
	s := w.session(alex.ID, old, ids.ID{})

	n, err := w.l.ExpireInactiveDevices(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("ExpireInactiveDevices = %d, %v", n, err)
	}
	if r := w.text(`SELECT revoke_reason FROM devices WHERE id = $1`, old.String()); r != "inactive" {
		t.Errorf("old device reason %q", r)
	}
	if by := w.text(`SELECT revoked_by::text FROM devices WHERE id = $1`, old.String()); by != "" {
		t.Errorf("revoked_by %q", by)
	}
	if r := w.text(`SELECT revoke_reason FROM auth_sessions WHERE id = $1`, s.String()); r != "inactive" {
		t.Errorf("session reason %q", r)
	}
	if st := w.text(`SELECT state FROM devices WHERE id = $1`, recent.String()); st != "active" {
		t.Errorf("recent device %s", st)
	}
	if n, err := w.l.ExpireInactiveDevices(context.Background()); err != nil || n != 0 {
		t.Errorf("second run = %d, %v", n, err)
	}
}
