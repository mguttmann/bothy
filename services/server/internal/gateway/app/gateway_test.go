// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// TestOpenSendsHello: the first frame carries the configured windows, the protocol versions, the
// current nonce, the server time and the sync epoch.
func TestOpenSendsHello(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	_, fc := e.open()
	h := sent[app.Hello](fc)
	if len(h) != 1 || len(fc.frames) != 1 {
		t.Fatalf("frames %#v", fc.frames)
	}
	want := app.Hello{HeartbeatInterval: 30 * time.Second, ResumeWindow: 5 * time.Minute, ProtocolVersions: []int{1},
		DPoPNonce: "nonce-1", ServerTime: t0, SyncEpoch: epoch}
	if h[0].HeartbeatInterval != want.HeartbeatInterval || h[0].ResumeWindow != want.ResumeWindow || !slices.Equal(h[0].ProtocolVersions, want.ProtocolVersions) ||
		h[0].DPoPNonce != want.DPoPNonce || !h[0].ServerTime.Equal(t0) || h[0].SyncEpoch != epoch {
		t.Errorf("hello %+v", h[0])
	}
}

// TestIdentifyOutcomeMapping is note 10 of the brief: one case per row of the table of causes
// at connection setup.
func TestIdentifyOutcomeMapping(t *testing.T) {
	t.Parallel()
	type tc struct {
		prepare func(e *env)
		frame   func(e *env) map[string]any
		want    domain.CloseCode
	}
	alexFrame := func(e *env) map[string]any {
		return map[string]any{"protocol_version": 1, "access_token": alexToken, "dpop": "p", "device_id": e.alex.DeviceID}
	}
	authErr := func(err error) func(e *env) {
		return func(e *env) { e.auth.errs[alexToken] = err }
	}
	deny := func(o authorize.Outcome, r authorize.Reason) func(e *env) {
		return func(e *env) { e.authz.set(e.alex.ID, authorize.Decision{Outcome: o, Reason: r}) }
	}
	cases := map[string]tc{
		"unknown token":         {nil, func(e *env) map[string]any { m := alexFrame(e); m["access_token"] = "nope"; return m }, domain.CloseAuthenticationFailed},
		"bad proof":             {authErr(&identity.DPoPError{Code: identity.DPoPInvalidProof}), alexFrame, domain.CloseAuthenticationFailed},
		"wrong nonce":           {authErr(&identity.DPoPError{Code: identity.DPoPUseNonce}), alexFrame, domain.CloseAuthenticationFailed},
		"device mismatch":       {nil, func(e *env) map[string]any { m := alexFrame(e); m["device_id"] = ids.New(); return m }, domain.CloseAuthenticationFailed},
		"device missing":        {nil, func(e *env) map[string]any { m := alexFrame(e); delete(m, "device_id"); return m }, domain.CloseAuthenticationFailed},
		"expired":               {authErr(errs.New(errs.CodeTokenExpired, "")), alexFrame, domain.CloseTokenExpired},
		"principal inactive":    {deny(authorize.OutcomeUnauthenticated, "principal_inactive"), alexFrame, domain.CloseRevoked},
		"session revoked":       {deny(authorize.OutcomeUnauthenticated, "session_revoked"), alexFrame, domain.CloseRevoked},
		"device inactive":       {deny(authorize.OutcomeUnauthenticated, "device_inactive"), alexFrame, domain.CloseRevoked},
		"token revoked":         {deny(authorize.OutcomeUnauthenticated, "token_revoked"), alexFrame, domain.CloseRevoked},
		"app not installed":     {deny(authorize.OutcomeUnauthenticated, "app_not_installed"), alexFrame, domain.CloseRevoked},
		"device pending":        {deny(authorize.OutcomeForbidden, "device_pending"), alexFrame, domain.CloseDevicePending},
		"scope missing":         {deny(authorize.OutcomeForbidden, "scope_missing"), alexFrame, domain.CloseForbidden},
		"rate limited":          {func(e *env) { e.limiter.deny[e.alex.ID] = true }, alexFrame, domain.CloseRateLimited},
		"database unavailable":  {nil, alexFrame, domain.CloseUnknownError},
		"protocol version":      {nil, func(e *env) map[string]any { m := alexFrame(e); m["protocol_version"] = 2; return m }, domain.CloseProtocolVersionUnsupported},
		"no token":              {nil, func(e *env) map[string]any { m := alexFrame(e); delete(m, "access_token"); return m }, domain.CloseDecodeError},
		"both tokens":           {nil, func(e *env) map[string]any { m := alexFrame(e); m["api_token"] = agentToken; return m }, domain.CloseDecodeError},
		"authenticator failing": {authErr(errs.New(errs.CodeUnavailable, "")), alexFrame, domain.CloseUnknownError},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, func(_ *app.Config, d *app.Deps) {
				if name == "database unavailable" {
					d.Snapshot = fakeSnapshot{err: errs.New(errs.CodeUnavailable, ""), log: &callLog{}}
				}
			})
			if c.prepare != nil {
				c.prepare(e)
			}
			conn, fc := e.open()
			e.send(conn, "identify", c.frame(e))
			wantClosed(t, fc, c.want)
			if len(sent[app.Ready](fc)) != 0 {
				t.Error("ready sent")
			}
		})
	}
}

// TestIdentifyReadyAndRateLimitReason: a person and an agent get ready with the snapshot; the
// credentials reach the authenticator with method GET, the HTTPS htu and the proof as DPoP; the
// rate limit closes with retry_after=<seconds> as reason.
func TestIdentifyReadyAndRateLimitReason(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	c, fc := e.open()
	e.identifyAlex(c)
	wantOpen(t, fc)
	r := sent[app.Ready](fc)
	if len(r) != 1 || r[0].View.HeadSeq != 41 || r[0].DeviceID != e.alex.DeviceID || r[0].SessionID.IsZero() {
		t.Fatalf("ready %+v", r)
	}
	cr := e.auth.creds[0]
	if cr.Method != "GET" || cr.HTU != e.cfg.HTU || cr.Authorization != "DPoP "+alexToken || cr.DPoP != "proof" {
		t.Errorf("credentials %+v", cr)
	}
	ac, afc := e.open()
	e.send(ac, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
	wantOpen(t, afc)
	if r := sent[app.Ready](afc); len(r) != 1 {
		t.Fatalf("agent ready %+v", r)
	}
	if got := e.auth.creds[1].Authorization; got != "Bearer "+agentToken {
		t.Errorf("an API token without proof goes as %q", got)
	}
	if s := e.gw.SessionOf(ac); s == nil || s.Consumer != "default" {
		t.Errorf("agent session %+v, want consumer default", s)
	}
	e.limiter.deny[e.agent.ID] = true
	lc, lfc := e.open()
	e.send(lc, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
	if code, reason := lfc.closed(); code != domain.CloseRateLimited || reason != "retry_after=7" {
		t.Errorf("rate limit close %d %q", code, reason)
	}
}

// TestDispatcherRequiresAuthorize: a handler that returns without an authorize check of its
// declared action closes the connection with 4000 and raises the alarm; an op error answers error
// and needs no proof.
func TestDispatcherRequiresAuthorize(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	if err := e.gw.Dispatcher().Register("focus", authorize.ActionGatewayFocus, func(context.Context, *app.Connection, any) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := e.gw.Dispatcher().Register("typing.start", authorize.ActionTypingStart, func(context.Context, *app.Connection, any) error {
		return &app.OpError{Code: string(errs.CodeValidationFailed)}
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.gw.Dispatcher().Register("focus", authorize.ActionGatewayFocus, nil); err == nil {
		t.Error("a second registration of an op was accepted")
	}
	c, fc := e.open()
	e.identifyAlex(c)
	e.send(c, "typing.start", map[string]any{})
	wantOpen(t, fc)
	if ef := sent[app.ErrorFrame](fc); len(ef) != 1 || ef[0].Op != "typing.start" || ef[0].Code != "validation_failed" {
		t.Fatalf("error frames %+v", ef)
	}
	e.send(c, "focus", map[string]any{})
	wantClosed(t, fc, domain.CloseUnknownError)
	if !slices.Equal(e.alarm.ops, []string{"focus"}) {
		t.Errorf("alarm %v", e.alarm.ops)
	}
}

// TestUnregisteredKnownOpIsUnknown: an op of gateway_ops.later is unknown (4001) before and after
// identify; a registered op before identify is 4003; a second identify or a resume after it 4005.
func TestUnregisteredKnownOpIsUnknown(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	c, fc := e.open()
	e.send(c, "focus", map[string]any{"thread_ids": []string{}})
	wantClosed(t, fc, domain.CloseUnknownOp)

	c, fc = e.open()
	e.send(c, "heartbeat", map[string]any{"last_seq": 0})
	wantClosed(t, fc, domain.CloseNotIdentified)

	c, fc = e.open()
	e.identifyAlex(c)
	e.send(c, "typing.start", map[string]any{})
	wantClosed(t, fc, domain.CloseUnknownOp)

	c, fc = e.open()
	e.identifyAlex(c)
	e.identifyAlex(c)
	wantClosed(t, fc, domain.CloseAlreadyIdentified)

	c, fc = e.open()
	e.identifyAlex(c)
	e.send(c, "resume", map[string]any{"gateway_session_id": ids.New(), "access_token": alexToken, "dpop": "p", "cursor": 0})
	wantClosed(t, fc, domain.CloseAlreadyIdentified)

	c, fc = e.open()
	e.send(c, "identify", []int{1})
	wantClosed(t, fc, domain.CloseDecodeError)
}

// TestIdentifyTimeout: without identify within sync.identify_timeout the server closes with 4003.
func TestIdentifyTimeout(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	c, fc := e.open()
	next := e.gw.Tick(context.Background(), c, e.clk.Now())
	if !next.Equal(t0.Add(10 * time.Second)) {
		t.Errorf("next %v", next)
	}
	e.gw.Tick(context.Background(), c, t0.Add(10*time.Second-time.Millisecond))
	wantOpen(t, fc)
	e.gw.Tick(context.Background(), c, t0.Add(10*time.Second))
	wantClosed(t, fc, domain.CloseNotIdentified)
}

// TestHeartbeatDeadline: the deadline is sync.heartbeat_interval times (miss limit + 1) after
// ready and after every heartbeat; a heartbeat is answered with heartbeat_ack and authorized; a
// revoked principal is closed with 4012 at the next heartbeat.
func TestHeartbeatDeadline(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	c, fc := e.open()
	e.identifyAlex(c)
	ctx := context.Background()
	if next := e.gw.Tick(ctx, c, t0); !next.Equal(t0.Add(90 * time.Second)) {
		t.Fatalf("first deadline %v", next)
	}
	e.clk.Set(t0.Add(89 * time.Second))
	before := e.authz.calls
	e.send(c, "heartbeat", map[string]any{"last_seq": 4})
	if acks := sent[app.HeartbeatAck](fc); len(acks) != 1 || !acks[0].ServerTime.Equal(e.clk.Now()) || e.authz.calls != before+1 {
		t.Fatalf("acks %+v, authorize calls %d", acks, e.authz.calls-before)
	}
	e.gw.Tick(ctx, c, t0.Add(90*time.Second))
	wantOpen(t, fc)
	e.gw.Tick(ctx, c, t0.Add(179*time.Second-time.Millisecond))
	wantOpen(t, fc)
	e.gw.Tick(ctx, c, t0.Add(179*time.Second))
	wantClosed(t, fc, domain.CloseHeartbeatTimeout)

	c, fc = e.open()
	e.identifyAlex(c)
	e.send(c, "heartbeat", map[string]any{"last_seq": -1})
	wantClosed(t, fc, domain.CloseDecodeError)

	c, fc = e.open()
	e.identifyAlex(c)
	e.authz.set(e.alex.ID, authorize.Decision{Outcome: authorize.OutcomeUnauthenticated, Reason: "session_revoked"})
	e.send(c, "heartbeat", map[string]any{"last_seq": 0})
	wantClosed(t, fc, domain.CloseRevoked)
}

// TestReauthSchedule: reauth_required goes out sync.reauth_lead_time before the access token
// expires, once, with the current nonce; a valid reauth of the same session moves the expiry; a
// token of another auth session closes with 4004; an invalid token answers error and keeps the
// connection; without reauth the connection closes with 4011 at the expiry; an agent gets no
// reauth_required and is closed with 4011 at the expiry of its token.
func TestReauthSchedule(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	ctx := context.Background()
	c, fc := e.open()
	e.identifyAlex(c)
	exp := e.alex.CredentialExpiresAt
	// Heartbeats keep the connection alive while the clock runs to the lead time.
	beat := func(at time.Time) {
		e.clk.Set(at)
		e.send(c, "heartbeat", map[string]any{"last_seq": 0})
	}
	beat(exp.Add(-2 * time.Minute))
	if next := e.gw.Tick(ctx, c, e.clk.Now()); !next.Equal(exp.Add(-time.Minute)) {
		t.Fatalf("next %v, want the reauth instant", next)
	}
	beat(exp.Add(-time.Minute))
	e.gw.Tick(ctx, c, e.clk.Now())
	e.gw.Tick(ctx, c, e.clk.Now())
	rr := sent[app.ReauthRequired](fc)
	if len(rr) != 1 || !rr[0].ExpiresAt.Equal(exp) || rr[0].DPoPNonce != "nonce-1" {
		t.Fatalf("reauth_required %+v", rr)
	}
	// An invalid proof answers error and keeps the connection.
	e.auth.errs["bad"] = &identity.DPoPError{Code: identity.DPoPInvalidProof}
	e.send(c, "reauth", map[string]any{"access_token": "bad", "dpop": "p"})
	wantOpen(t, fc)
	if ef := sent[app.ErrorFrame](fc); len(ef) != 1 || ef[0].Op != "reauth" || ef[0].Code != "invalid_dpop_proof" {
		t.Fatalf("error frames %+v", ef)
	}
	// A refreshed token of the same session moves the expiry.
	fresh := e.alex
	fresh.CredentialExpiresAt = exp.Add(10 * time.Minute)
	e.auth.actors["fresh"] = fresh
	e.send(c, "reauth", map[string]any{"access_token": "fresh", "dpop": "p"})
	wantOpen(t, fc)
	beat(exp.Add(time.Second))
	e.gw.Tick(ctx, c, e.clk.Now())
	wantOpen(t, fc)
	// Without a further reauth the connection closes at the new expiry.
	beat(fresh.CredentialExpiresAt.Add(-time.Second))
	e.gw.Tick(ctx, c, fresh.CredentialExpiresAt)
	wantClosed(t, fc, domain.CloseTokenExpired)

	// A token of another auth session of the same person closes with 4004.
	c, fc = e.open()
	e.identifyAlex(c)
	other := e.alex
	other.SessionID = ids.New()
	e.auth.actors["other"] = other
	e.send(c, "reauth", map[string]any{"access_token": "other", "dpop": "p"})
	wantClosed(t, fc, domain.CloseAuthenticationFailed)

	// Agents: no reauth_required, 4011 at expires_at.
	c, fc = e.open()
	e.clk.Set(e.agent.CredentialExpiresAt.Add(-30 * time.Second))
	e.send(c, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
	e.gw.Tick(ctx, c, e.clk.Now())
	if rr := sent[app.ReauthRequired](fc); len(rr) != 0 {
		t.Fatalf("agent got reauth_required %+v", rr)
	}
	e.gw.Tick(ctx, c, e.agent.CredentialExpiresAt)
	wantClosed(t, fc, domain.CloseTokenExpired)
}

// TestResumeWindow: a detached session resumes within sync.resume_window with resumed and the
// same id; after the window, for another principal or device, or for an unknown id the server
// closes with 4006; the sweeper removes expired sessions.
func TestResumeWindow(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	c, fc := e.open()
	e.identifyAlex(c)
	id := sent[app.Ready](fc)[0].SessionID
	c.Close(domain.CloseGoingAway, "")
	e.gw.Closed(c)
	resume := func(token string, sid ids.ID) *fakeConn {
		rc, rfc := e.open()
		e.send(rc, "resume", map[string]any{"gateway_session_id": sid, "access_token": token, "dpop": "p", "cursor": 3})
		return rfc
	}
	e.clk.Set(t0.Add(5*time.Minute - time.Second))
	rfc := resume(alexToken, id)
	wantOpen(t, rfc)
	if r := sent[app.Resumed](rfc); len(r) != 1 || r[0].SessionID != id || r[0].HeadSeq != 41 || r[0].SyncEpoch != epoch {
		t.Fatalf("resumed %+v", r)
	}
	if !slices.Contains(e.log.all(), "attach 3") {
		t.Errorf("delivery %v, want attach from the cursor", e.log.all())
	}

	// Another principal and an unknown id are 4006 like an expired session.
	bob := actor.Actor{ID: ids.New(), Kind: actor.KindUser, SessionID: ids.New(), DeviceID: ids.New(), CredentialExpiresAt: t0.Add(time.Hour)}
	e.auth.actors["bob"] = bob
	wantClosed(t, resume("bob", id), domain.CloseSessionInvalid)
	wantClosed(t, resume(alexToken, ids.New()), domain.CloseSessionInvalid)

	// Detached longer than the window: 4006, and the sweeper removes it.
	c2, fc2 := e.open()
	e.identifyAlex(c2)
	id2 := sent[app.Ready](fc2)[0].SessionID
	c2.Close(domain.CloseGoingAway, "")
	e.gw.Closed(c2)
	e.clk.Set(e.clk.Now().Add(5 * time.Minute))
	wantClosed(t, resume(alexToken, id2), domain.CloseSessionInvalid)
	if n := e.gw.Sessions().Sweep(e.clk.Now()); n < 1 {
		t.Errorf("sweep removed %d sessions", n)
	}
}

// TestOneConnectionPerDevice: identify and resume of the same device close the older open
// connection with 4017 before the new one gets ready or resumed; agents are not affected.
func TestOneConnectionPerDevice(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	old, oldFC := e.open()
	e.identifyAlex(old)
	id := sent[app.Ready](oldFC)[0].SessionID
	c, fc := e.open()
	e.identifyAlex(c)
	wantClosed(t, oldFC, domain.CloseDeviceReplaced)
	wantOpen(t, fc)
	e.gw.Closed(old)

	r, rfc := e.open()
	e.send(r, "resume", map[string]any{"gateway_session_id": sent[app.Ready](fc)[0].SessionID, "access_token": alexToken, "dpop": "p", "cursor": 0})
	wantClosed(t, fc, domain.CloseDeviceReplaced)
	wantOpen(t, rfc)
	if id == sent[app.Resumed](rfc)[0].SessionID {
		t.Error("the replaced session was resumed")
	}

	a1, a1fc := e.open()
	e.send(a1, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
	a2, a2fc := e.open()
	e.send(a2, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
	wantOpen(t, a1fc)
	wantOpen(t, a2fc)
}

// TestResyncRequiredAfterReady: an identify with a cursor of another epoch gets ready, then
// resync_required with epoch_changed, and delivery is attached at the head of ready; without a
// cursor delivery starts at the head.
func TestResyncRequiredAfterReady(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	c, fc := e.open()
	e.send(c, "identify", map[string]any{"protocol_version": 1, "access_token": alexToken, "dpop": "p", "device_id": e.alex.DeviceID,
		"cursor": 7, "sync_epoch": ids.New()})
	wantOpen(t, fc)
	rr := sent[app.ResyncRequired](fc)
	if len(rr) != 1 || rr[0].Reason != sync.ResyncEpochChanged || rr[0].HeadSeq != 41 || rr[0].SyncEpoch != epoch {
		t.Fatalf("resync_required %+v", rr)
	}
	if calls := e.log.all(); calls[len(calls)-1] != "attach 41" {
		t.Errorf("delivery %v", calls)
	}
}

// TestRegisterBeforeSnapshot: the session is registered with delivery before the snapshot reads
// the head (04 section 6.3 step 4).
func TestRegisterBeforeSnapshot(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	c, _ := e.open()
	e.identifyAlex(c)
	calls := e.log.all()
	if len(calls) < 3 || !strings.HasPrefix(calls[0], "register ") || calls[1] != "snapshot" || calls[2] != "attach 41" {
		t.Errorf("calls %v, want register, snapshot, attach", calls)
	}
}

// TestNewRejectsBadConfig: a missing collaborator or a non-positive window is an error.
func TestNewRejectsBadConfig(t *testing.T) {
	t.Parallel()
	if _, err := app.New(app.Config{}, app.Deps{}); err == nil {
		t.Fatal("empty config accepted")
	}
	e := newEnv(t, nil)
	if err := e.gw.Register("ack", authorize.ActionGatewayConnect); err == nil {
		t.Error("an op without a handler of the gateway was registered")
	}
	if err := e.gw.Register("identify", authorize.ActionGatewayConnect); err == nil {
		t.Error("identify was registered twice")
	}
}

// closingAuth closes the connection of the frame while it authenticates, like the identify
// deadline or a newer connection of the device does under load.
type closingAuth struct {
	*fakeAuth
	conn func() *app.Connection
}

func (a closingAuth) Authenticate(ctx context.Context, c actor.Credentials) (actor.Actor, error) {
	a.conn().Close(domain.CloseNotIdentified, "")
	return a.fakeAuth.Authenticate(ctx, c)
}

// TestCloseDuringIdentifyLeavesNoSession (review M1-64): a connection that closes while identify
// or resume runs leaves no session behind: a new session is removed, a resumed one is detached
// and expires with sync.resume_window.
func TestCloseDuringIdentifyLeavesNoSession(t *testing.T) {
	t.Parallel()
	var current *app.Connection
	e := newEnv(t, nil)
	c, _ := e.open()
	e.identifyAlex(c)
	id := e.gw.SessionOf(c).ID
	c.Close(domain.CloseGoingAway, "")
	e.gw.Closed(c)

	e2 := newEnv(t, func(_ *app.Config, d *app.Deps) {
		d.Authenticator = closingAuth{fakeAuth: e.auth, conn: func() *app.Connection { return current }}
	})
	current, _ = e2.open()
	e2.send(current, "identify", map[string]any{"protocol_version": 1, "access_token": alexToken, "dpop": "p", "device_id": e.alex.DeviceID})
	if e2.authz.calls == 0 {
		t.Fatal("identify did not get past authentication in the setup")
	}
	e2.gw.Closed(current)
	if n := e2.gw.Sessions().Len(); n != 0 {
		t.Fatalf("%d sessions after a close during identify, want 0", n)
	}
	if calls := e2.log.all(); len(calls) != 0 {
		t.Errorf("delivery calls %v for a connection closed during identify", calls)
	}

	// The same for resume in the first env: the resumed session is detached again.
	e.auth = &fakeAuth{actors: e.auth.actors, errs: map[string]error{}}
	r, _ := e.open()
	e.gw.Handle(context.Background(), r, "resume", mustJSON(t, map[string]any{"gateway_session_id": id, "access_token": alexToken, "dpop": "p", "cursor": 0}))
	if e.gw.SessionOf(r) == nil {
		t.Fatal("resume failed in the setup")
	}
	r.Close(domain.CloseGoingAway, "")
	e.gw.Closed(r)
	e.clk.Set(e.clk.Now().Add(5 * time.Minute))
	if n := e.gw.Sessions().Sweep(e.clk.Now()); n != 1 {
		t.Errorf("sweep removed %d sessions, want the resumed one", n)
	}
}

// mustJSON encodes v.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestHeartbeatRevalidates (red team M1-64): a heartbeat checks the credential again, so a
// session, device, token or principal revoked after identify closes with 4012 at the next
// heartbeat, even when the rights version did not move.
func TestHeartbeatRevalidates(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	c, fc := e.open()
	e.identifyAlex(c)
	e.auth.mu.Lock()
	e.auth.revoked = map[ids.ID]bool{e.alex.ID: true}
	e.auth.mu.Unlock()
	e.send(c, "heartbeat", map[string]any{"last_seq": 0})
	wantClosed(t, fc, domain.CloseRevoked)
}

// TestDeadlineWaitsForAFrameInFlight: a deadline that passes while a frame that arrived before
// it is still being handled closes nothing; the frame decides. An identify whose authentication
// outlasts sync.identify_timeout ends ready, a heartbeat whose revalidation outlasts the heartbeat
// deadline is acknowledged, and the deadlines apply again once the frame is handled.
func TestDeadlineWaitsForAFrameInFlight(t *testing.T) {
	t.Parallel()
	e := newEnv(t, nil)
	ctx := context.Background()
	c, fc := e.open()
	e.auth.authenticating = func() {
		e.clk.Set(t0.Add(e.cfg.IdentifyTimeout + time.Second))
		e.gw.Tick(ctx, c, e.clk.Now())
	}
	e.identifyAlex(c)
	wantOpen(t, fc)
	if r := sent[app.Ready](fc); len(r) != 1 {
		t.Fatalf("ready frames %d, want 1", len(r))
	}
	deadline := e.clk.Now().Add(e.cfg.HeartbeatInterval * time.Duration(e.cfg.HeartbeatMissLimit+1))
	e.auth.mu.Lock()
	e.auth.revalidate = func() error {
		e.clk.Set(deadline.Add(time.Second))
		e.gw.Tick(ctx, c, e.clk.Now())
		return nil
	}
	e.auth.mu.Unlock()
	e.send(c, "heartbeat", map[string]any{"last_seq": 0})
	wantOpen(t, fc)
	if acks := sent[app.HeartbeatAck](fc); len(acks) != 1 {
		t.Fatalf("heartbeat_ack frames %d, want 1", len(acks))
	}
	e.gw.Tick(ctx, c, e.clk.Now().Add(e.cfg.HeartbeatInterval*time.Duration(e.cfg.HeartbeatMissLimit+1)))
	wantClosed(t, fc, domain.CloseHeartbeatTimeout)
}

// TestExpiryBeforeHeartbeatDeadline: when the heartbeat deadline and the expiry of the credential
// have both passed, the tick closes with 4011, also while a frame is in flight, so the client
// renews its token instead of reconnecting with the expired one.
func TestExpiryBeforeHeartbeatDeadline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	e := newEnv(t, nil)
	c, fc := e.open()
	e.identifyAlex(c)
	e.gw.Tick(ctx, c, e.alex.CredentialExpiresAt.Add(time.Second))
	wantClosed(t, fc, domain.CloseTokenExpired)

	e = newEnv(t, nil)
	c, fc = e.open()
	e.identifyAlex(c)
	e.auth.mu.Lock()
	e.auth.revalidate = func() error {
		e.gw.Tick(ctx, c, e.alex.CredentialExpiresAt.Add(time.Second))
		return nil
	}
	e.auth.mu.Unlock()
	e.send(c, "heartbeat", map[string]any{"last_seq": 0})
	wantClosed(t, fc, domain.CloseTokenExpired)
}

// TestHeartbeatAtExpiryCloses4011: a heartbeat whose revalidation refuses the credential as
// unauthenticated once it has reached its expiry (also when the expiry passes during the
// revalidation) closes with 4011 like the tick of the expiry, not with 4012; before the expiry,
// and for a revoked session at any time, it closes with 4012.
func TestHeartbeatAtExpiryCloses4011(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		at   time.Duration
		err  error
		want domain.CloseCode
	}{
		"expired during the revalidation": {0, errs.New(errs.CodeUnauthenticated, ""), domain.CloseTokenExpired},
		"revoked before the expiry":       {-time.Second, errs.New(errs.CodeUnauthenticated, ""), domain.CloseRevoked},
		"session revoked at the expiry":   {0, errs.New(errs.CodeSessionRevoked, ""), domain.CloseRevoked},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, nil)
			conn, fc := e.open()
			e.identifyAlex(conn)
			exp := e.alex.CredentialExpiresAt
			e.clk.Set(exp.Add(-time.Second - time.Millisecond))
			e.auth.mu.Lock()
			e.auth.revalidate = func() error {
				e.clk.Set(exp.Add(c.at))
				return c.err
			}
			e.auth.mu.Unlock()
			e.send(conn, "heartbeat", map[string]any{"last_seq": 0})
			wantClosed(t, fc, c.want)
		})
	}
}

// TestSnapshotDenialCloses4018 (red team M1-64): when the snapshot of ready is refused after
// gateway.connect allowed (an app: sync.snapshot.read has no app scope in the catalogue), the
// connection closes with 4018 like any other forbidden at setup, not with 4000, so the client
// does not reconnect in a loop; a revoked session at that point closes with 4012.
func TestSnapshotDenialCloses4018(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		err  error
		want domain.CloseCode
	}{
		"forbidden":       {errs.New(errs.CodeForbidden, "").WithReason("subject_kind_not_allowed"), domain.CloseForbidden},
		"scope missing":   {errs.New(errs.CodeScopeMissing, ""), domain.CloseForbidden},
		"session revoked": {errs.New(errs.CodeSessionRevoked, ""), domain.CloseRevoked},
		"device pending":  {errs.New(errs.CodeDevicePending, ""), domain.CloseDevicePending},
		"unavailable":     {errs.New(errs.CodeUnavailable, ""), domain.CloseUnknownError},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newEnv(t, func(_ *app.Config, d *app.Deps) { d.Snapshot = fakeSnapshot{err: c.err, log: &callLog{}} })
			conn, fc := e.open()
			e.send(conn, "identify", map[string]any{"protocol_version": 1, "api_token": agentToken})
			wantClosed(t, fc, c.want)
		})
	}
}
