// SPDX-License-Identifier: Apache-2.0

package app_test

// Tests of the failed-credentials rule of 02 9.1 on the gateway (05 3; audit of 2026-11-11,
// srv-gateway-sync-2): identify, resume and reauth reserve one token of rl.public_auth of the
// client address before the credentials are checked, keep it when the check fails and give it
// back otherwise; with the bucket empty no check runs.

import (
	"context"
	"encoding/json"
	"net/netip"
	gosync "sync"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/gateway/app"
	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// fakeCredentials is the bucket rl.public_auth of the client addresses: tokens left, the wait it
// names when empty, and what happened to every reserved token.
type fakeCredentials struct {
	mu             gosync.Mutex
	tokens, retry  int
	reserved       []netip.Addr
	kept, refunded int
}

// ReserveCredentials implements app.CredentialLimiter.
func (l *fakeCredentials) ReserveCredentials(ip netip.Addr) (bool, int, func(bool)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reserved = append(l.reserved, ip)
	if l.tokens == 0 {
		return false, l.retry, func(bool) {}
	}
	l.tokens--
	return true, 0, func(failed bool) {
		l.mu.Lock()
		defer l.mu.Unlock()
		if failed {
			l.kept++
			return
		}
		l.tokens++
		l.refunded++
	}
}

// outcome returns the kept and the refunded tokens.
func (l *fakeCredentials) outcome() (kept, refunded int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.kept, l.refunded
}

// clientAddr is the client address of the upgrade in these tests.
var clientAddr = netip.MustParseAddr("203.0.113.7")

// credEnv returns an env whose gateway counts failed credentials in l.
func credEnv(t *testing.T, l *fakeCredentials) *env {
	t.Helper()
	return newEnv(t, func(_ *app.Config, d *app.Deps) { d.Credentials = l })
}

// sendFrom dispatches op with the JSON of d on c as a frame of a connection upgraded from
// clientAddr.
func (e *env) sendFrom(c *app.Connection, op string, d any) {
	e.t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		e.t.Fatal(err)
	}
	e.gw.Handle(reqmeta.WithClientIP(context.Background(), clientAddr), c, op, b)
}

// TestIdentifyAndResumeCountFailedCredentials: one case per outcome of the credential check of
// identify and resume. A rejected token or proof, a session the authenticator reports revoked, a
// credential it reports inactive without an error (actor.Actor.CredentialInactive: a revoked or
// expired API token, a revoked session, device or device key) and a device that is not the
// token's keep the token of rl.public_auth; valid credentials, the normal flows token_expired and
// use_dpop_nonce and an error that is not the client's give it back, also when authorize refuses
// the verified principal of an active credential afterwards.
func TestIdentifyAndResumeCountFailedCredentials(t *testing.T) {
	t.Parallel()
	type tc struct {
		authErr  error
		inactive bool
		deny     bool
		frame    func(e *env, m map[string]any)
		wantKept bool
		want     domain.CloseCode
	}
	cases := map[string]tc{
		"unknown token":   {frame: func(_ *env, m map[string]any) { m["access_token"] = "nope" }, wantKept: true, want: domain.CloseAuthenticationFailed},
		"bad proof":       {authErr: &identity.DPoPError{Code: identity.DPoPInvalidProof}, wantKept: true, want: domain.CloseAuthenticationFailed},
		"session revoked": {authErr: errs.New(errs.CodeSessionRevoked, ""), wantKept: true, want: domain.CloseRevoked},
		"device mismatch": {frame: func(_ *env, m map[string]any) { m["device_id"] = ids.New() }, wantKept: true, want: domain.CloseAuthenticationFailed},
		"inactive":        {inactive: true, wantKept: true, want: domain.CloseRevoked},
		"wrong nonce":     {authErr: &identity.DPoPError{Code: identity.DPoPUseNonce}, want: domain.CloseAuthenticationFailed},
		"expired":         {authErr: errs.New(errs.CodeTokenExpired, ""), want: domain.CloseTokenExpired},
		"unavailable":     {authErr: errs.New(errs.CodeUnavailable, ""), want: domain.CloseUnknownError},
		"denied later":    {deny: true, want: domain.CloseRevoked},
		"valid":           {},
	}
	for name, c := range cases {
		for _, op := range []string{"identify", "resume"} {
			if op == "resume" && (name == "device mismatch" || name == "valid") {
				// resume names no device, and its success needs a session (TestResumeWindow).
				continue
			}
			t.Run(name+"/"+op, func(t *testing.T) {
				t.Parallel()
				l := &fakeCredentials{tokens: 1}
				e := credEnv(t, l)
				if c.authErr != nil {
					e.auth.errs[alexToken] = c.authErr
				}
				if c.inactive {
					// The authenticator rejects no revoked or expired credential itself: it marks
					// the actor, and authorize refuses it.
					a := e.alex
					a.CredentialInactive = true
					e.auth.actors[alexToken] = a
					e.authz.set(e.alex.ID, authorize.Decision{Outcome: authorize.OutcomeUnauthenticated, Reason: "session_revoked"})
				}
				if c.deny {
					e.authz.set(e.alex.ID, authorize.Decision{Outcome: authorize.OutcomeUnauthenticated, Reason: "principal_inactive"})
				}
				frame := map[string]any{"access_token": alexToken, "dpop": "proof"}
				if op == "identify" {
					frame["protocol_version"], frame["device_id"] = 1, e.alex.DeviceID
				} else {
					frame["gateway_session_id"], frame["cursor"] = ids.New(), 0
				}
				if c.frame != nil {
					c.frame(e, frame)
				}
				conn, fc := e.open()
				e.sendFrom(conn, op, frame)
				if c.want == 0 {
					wantOpen(t, fc)
				} else {
					wantClosed(t, fc, c.want)
				}
				kept, refunded := l.outcome()
				if wantKept, wantRefunded := btoi(c.wantKept), btoi(!c.wantKept); kept != wantKept || refunded != wantRefunded {
					t.Fatalf("kept %d and refunded %d tokens, want %d and %d", kept, refunded, wantKept, wantRefunded)
				}
				if len(l.reserved) != 1 || l.reserved[0] != clientAddr {
					t.Fatalf("reserved for %v, want once for %v", l.reserved, clientAddr)
				}
			})
		}
	}
}

// btoi is 1 for true.
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TestIdentifyAndResumeWithEmptyCredentialBucket: while rl.public_auth of the client address is
// empty, identify and resume close with 4008 and retry_after=<seconds> before any credential is
// checked.
func TestIdentifyAndResumeWithEmptyCredentialBucket(t *testing.T) {
	t.Parallel()
	for op, frame := range map[string]map[string]any{
		"identify": {"protocol_version": 1, "access_token": alexToken, "dpop": "proof", "device_id": ids.New()},
		"resume":   {"gateway_session_id": ids.New(), "access_token": alexToken, "dpop": "proof", "cursor": 0},
	} {
		t.Run(op, func(t *testing.T) {
			t.Parallel()
			l := &fakeCredentials{retry: 17}
			e := credEnv(t, l)
			conn, fc := e.open()
			e.sendFrom(conn, op, frame)
			wantClosed(t, fc, domain.CloseRateLimited)
			if _, reason := fc.closed(); reason != "retry_after=17" {
				t.Fatalf("close reason %q, want retry_after=17", reason)
			}
			if n := len(e.auth.creds); n != 0 {
				t.Fatalf("%d credential checks with an empty bucket", n)
			}
		})
	}
}

// TestReauthCountsFailedCredentials: a rejected token of reauth answers error, keeps the
// connection and keeps the token of rl.public_auth; a valid one gives it back; with the bucket
// empty reauth answers error rate_limited with retry_after=<seconds> without a check, and the
// connection stays.
func TestReauthCountsFailedCredentials(t *testing.T) {
	t.Parallel()
	l := &fakeCredentials{tokens: 3, retry: 9}
	e := credEnv(t, l)
	conn, fc := e.open()
	e.sendFrom(conn, "identify", map[string]any{"protocol_version": 1, "access_token": alexToken, "dpop": "proof", "device_id": e.alex.DeviceID})
	wantOpen(t, fc)

	e.sendFrom(conn, "reauth", map[string]any{"access_token": "nope", "dpop": "proof"})
	e.sendFrom(conn, "reauth", map[string]any{"access_token": alexToken, "dpop": "proof"})
	wantOpen(t, fc)
	if got := sent[app.ErrorFrame](fc); len(got) != 1 || got[0] != (app.ErrorFrame{Op: "reauth", Code: "unauthenticated"}) {
		t.Fatalf("error frames %+v", got)
	}
	if kept, refunded := l.outcome(); kept != 1 || refunded != 2 {
		t.Fatalf("kept %d and refunded %d tokens, want 1 (the rejected reauth) and 2 (identify and the valid reauth)", kept, refunded)
	}

	l.mu.Lock()
	l.tokens = 0
	l.mu.Unlock()
	checks := len(e.auth.creds)
	e.sendFrom(conn, "reauth", map[string]any{"access_token": alexToken, "dpop": "proof"})
	wantOpen(t, fc)
	if got := sent[app.ErrorFrame](fc); len(got) != 2 || got[1] != (app.ErrorFrame{Op: "reauth", Code: "rate_limited", Detail: "retry_after=9"}) {
		t.Fatalf("error frames %+v", got)
	}
	if n := len(e.auth.creds); n != checks {
		t.Fatalf("%d credential checks with an empty bucket", n-checks)
	}
}

// TestReauthCountsAnInactiveCredential: a reauth whose credential the authenticator reports
// inactive without an error (the session was revoked since identify) keeps the token of
// rl.public_auth, and authorize then closes the connection with 4012.
func TestReauthCountsAnInactiveCredential(t *testing.T) {
	t.Parallel()
	l := &fakeCredentials{tokens: 2}
	e := credEnv(t, l)
	conn, fc := e.open()
	e.sendFrom(conn, "identify", map[string]any{"protocol_version": 1, "access_token": alexToken, "dpop": "proof", "device_id": e.alex.DeviceID})
	wantOpen(t, fc)

	a := e.alex
	a.CredentialInactive = true
	e.auth.mu.Lock()
	e.auth.actors[alexToken] = a
	e.auth.mu.Unlock()
	e.authz.set(e.alex.ID, authorize.Decision{Outcome: authorize.OutcomeUnauthenticated, Reason: "session_revoked"})
	e.sendFrom(conn, "reauth", map[string]any{"access_token": alexToken, "dpop": "proof"})
	wantClosed(t, fc, domain.CloseRevoked)
	if kept, refunded := l.outcome(); kept != 1 || refunded != 1 {
		t.Fatalf("kept %d and refunded %d tokens, want 1 (the reauth with the inactive credential) and 1 (identify)", kept, refunded)
	}
}

// countingEpoch is the epoch source of a gateway that counts its reads.
type countingEpoch struct {
	mu    gosync.Mutex
	reads int
	err   error
}

// SyncEpoch implements app.EpochSource.
func (c *countingEpoch) SyncEpoch(context.Context) (ids.ID, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reads++
	return epoch, c.err
}

// TestOpenReadsTheEpochOnce: the upgrade reads the instance row for the sync epoch of hello only
// until one read succeeded (threat model 5.5, "the upgrade reads no database"): every later hello
// carries the kept epoch, and a failed read is tried again by the next upgrade.
func TestOpenReadsTheEpochOnce(t *testing.T) {
	t.Parallel()
	src := &countingEpoch{err: errs.New(errs.CodeUnavailable, "")}
	e := newEnv(t, func(_ *app.Config, d *app.Deps) { d.Epoch = src })
	if _, err := e.gw.Open(context.Background(), &fakeConn{}); err == nil {
		t.Fatal("Open succeeded without the epoch")
	}
	src.mu.Lock()
	src.err = nil
	src.mu.Unlock()
	for range 3 {
		_, fc := e.open()
		if h := sent[app.Hello](fc); len(h) != 1 || h[0].SyncEpoch != epoch {
			t.Fatalf("hello %+v", h)
		}
	}
	if src.reads != 2 {
		t.Fatalf("%d reads of the epoch for four upgrades, want the failed one and one more", src.reads)
	}
}
