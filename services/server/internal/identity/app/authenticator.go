// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// apiTokenPrefix starts every API token (08 section 12, ADR 0061).
const apiTokenPrefix = "bothy_"

// The schemes of the Authorization header (06 section 3).
const (
	schemeDPoP   = "dpop"
	schemeBearer = "bearer"
)

// schemeChallenge is the WWW-Authenticate of a request whose credentials are not usable at all.
const schemeChallenge = `DPoP algs="` + identity.DPoPAlgs + `"`

// headerDPoPNonce carries the current nonce (06 section 3.1 item 2).
const headerDPoPNonce = "DPoP-Nonce"

// Log message and key of a failed activity write; the keys are constants for sloglint.
const (
	msgTouchFailed = "recording session activity failed"
	logKeyError    = "error_type"
)

// Authenticator implements kernel/actor.Authenticator for people (brief M1-34 "Authenticator"):
// access token, DPoP proof, then one query for session, device and key. It does not reject on the
// state of session and device: it reports it in the actor, and authorize decides (check_active),
// so the denial is audited with its reason (07 section 4.2). API tokens follow with M1-44. It is
// safe for concurrent use.
type Authenticator struct {
	cfg    Config
	tokens *AccessTokens
	dpop   identity.DPoP
	store  SessionStore
	clk    clock.Clock
	log    *slog.Logger
}

// Compile-time check: the Authenticator is the kernel port.
var _ actor.Authenticator = (*Authenticator)(nil)

// NewAuthenticator returns the Authenticator with identity.last_active_granularity of cfg (not
// negative), the access tokens, the DPoP verifier, the session store, the clock (nil means
// clock.System()) and the log of failed activity writes (nil means slog.Default()). Errors: a
// missing collaborator or a negative granularity.
func NewAuthenticator(cfg Config, tokens *AccessTokens, dpop identity.DPoP, store SessionStore, clk clock.Clock, log *slog.Logger) (*Authenticator, error) {
	if tokens == nil || dpop == nil || store == nil {
		return nil, errors.New("identity: NewAuthenticator needs the access tokens, the DPoP verifier and the session store")
	}
	if cfg.LastActiveGranularity < 0 {
		return nil, errors.New("identity: identity.last_active_granularity must not be negative")
	}
	if clk == nil {
		clk = clock.System()
	}
	if log == nil {
		log = slog.Default()
	}
	return &Authenticator{cfg: cfg, tokens: tokens, dpop: dpop, store: store, clk: clk, log: log}, nil
}

// unauthenticated is the 401 of credentials the server cannot use.
func unauthenticated() error {
	return errs.New(errs.CodeUnauthenticated, "").WithHeader("WWW-Authenticate", schemeChallenge)
}

// Authenticate implements actor.Authenticator in the order of the brief: no Authorization header
// is anonymous; more than one header or another scheme is unauthenticated; Bearer goes to the API
// token branch (M1-44) or, for this server's access token, fails with invalid_dpop_proof (RFC 9449
// section 7.2); DPoP checks the access token, then the proof, then reads session, device and key.
func (a *Authenticator) Authenticate(ctx context.Context, creds actor.Credentials) (actor.Actor, error) {
	if !creds.Present() {
		return actor.Anonymous(), nil
	}
	scheme, token, ok := splitAuthorization(creds.Authorization)
	if !ok {
		return actor.Actor{}, unauthenticated()
	}
	switch scheme {
	case schemeBearer:
		return actor.Actor{}, a.bearer(ctx, token)
	case schemeDPoP:
		return a.person(ctx, creds, token)
	}
	return actor.Actor{}, unauthenticated()
}

// splitAuthorization splits "<scheme> <token>" (scheme without regard to case, RFC 9110 section
// 11.1); a value with a comma is several headers joined by the chain, which is refused.
func splitAuthorization(v string) (scheme, token string, ok bool) {
	if strings.Contains(v, ",") {
		return "", "", false
	}
	scheme, token, ok = strings.Cut(v, " ")
	if !ok || token == "" || strings.ContainsAny(token, " \t") {
		return "", "", false
	}
	return strings.ToLower(scheme), token, true
}

// bearer is the Bearer branch: API tokens arrive with M1-44; a DPoP-bound access token of this
// server is invalid_dpop_proof (ID-T14), anything else unauthenticated.
func (a *Authenticator) bearer(ctx context.Context, token string) error {
	if strings.HasPrefix(token, apiTokenPrefix) {
		return unauthenticated()
	}
	if a.tokens.IsOwnAccessToken(ctx, token) {
		return &identity.DPoPError{Code: identity.DPoPInvalidProof, Nonce: a.dpop.CurrentNonce()}
	}
	return unauthenticated()
}

// proofs returns the DPoP values of creds: DPoPProofs, or DPoP alone for callers that fill only it.
func proofs(creds actor.Credentials) []string {
	if creds.DPoPProofs != nil {
		return creds.DPoPProofs
	}
	if creds.DPoP != "" {
		return []string{creds.DPoP}
	}
	return nil
}

// person authenticates a person with a DPoP-bound access token.
func (a *Authenticator) person(ctx context.Context, creds actor.Credentials, token string) (actor.Actor, error) {
	c, err := a.tokens.Verify(ctx, token)
	if err != nil {
		return actor.Actor{}, err
	}
	proof, err := a.dpop.Verify(ctx, identity.ProofRequest{
		Proofs: proofs(creds), Method: creds.Method, URL: creds.HTU, AccessToken: token, ExpectedJKT: c.Cnf.JKT,
	})
	if err != nil {
		return actor.Actor{}, err
	}
	if proof.NonceStale {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, a.dpop.CurrentNonce())
	}
	sub, sid, did := ids.MustParse(c.Subject), ids.MustParse(c.SessionID), ids.MustParse(c.DeviceID)
	st, found, err := a.store.AuthState(ctx, sid)
	if err != nil {
		return actor.Actor{}, err
	}
	out := actor.Actor{ID: sub, Kind: actor.KindUser, SessionID: sid, DeviceID: did, KeyThumbprint: c.Cnf.JKT, DeviceState: actor.DeviceUnknown}
	if len(c.AMR) == 1 {
		out.AuthMethod = c.AMR[0]
	}
	if !found {
		return out, nil
	}
	a.describe(&out, st)
	a.touch(ctx, st, creds)
	return out, nil
}

// describe fills the actor from the session, device and key read for this request. The session is
// active only when it is not revoked, has not ended and belongs to sub and did; the device state
// counts only for the device of the token; the key is active only when cnf.jkt is the device's
// active dpop key.
func (a *Authenticator) describe(out *actor.Actor, st AuthState) {
	s := st.Session
	own := s.UserID == out.ID && s.DeviceID == out.DeviceID
	out.SessionActive = own && s.Active(a.clk.Now())
	if own {
		out.DeviceState = string(st.DeviceState)
	}
	out.DeviceKeyActive = own && st.DeviceKeyJKT != "" && st.DeviceKeyJKT == out.KeyThumbprint
	out.StepUpAt, out.StepUpMethod = s.StepUpAt, s.StepUpMethod
	out.AuthMethod, out.ClientComponent = string(s.AuthMethod), string(s.Component)
}

// touch records activity outside the request's unit of work when the session or the device was
// last seen longer than identity.last_active_granularity ago; a failure is logged, never returned.
func (a *Authenticator) touch(ctx context.Context, st AuthState, creds actor.Credentials) {
	now := a.clk.Now()
	threshold := now.Add(-a.cfg.LastActiveGranularity)
	if !st.Session.LastSeenAt.IsZero() && !st.Session.LastSeenAt.Before(threshold) &&
		!st.DeviceLastSeenAt.IsZero() && !st.DeviceLastSeenAt.Before(threshold) {
		return
	}
	if err := a.store.Touch(ctx, st.Session.ID, st.Session.DeviceID, creds.ClientIP, now, threshold); err != nil {
		a.log.LogAttrs(ctx, slog.LevelWarn, msgTouchFailed, slog.String(logKeyError, errs.KindOf(err).String()),
			slog.String(logKeyCorrID, reqmeta.CorrelationID(ctx)))
	}
}

// Revalidate implements actor.Authenticator: the anonymous principal passes; a person passes while
// the session is active, belongs to it and its device is pending or active with the same key;
// everything else is 401 session_revoked (people) or unauthenticated (API tokens until M1-44).
func (a *Authenticator) Revalidate(ctx context.Context, p actor.Actor) error {
	if p.IsAnonymous() {
		return nil
	}
	if p.Kind != actor.KindUser || p.SessionID.IsZero() {
		return unauthenticated()
	}
	st, found, err := a.store.AuthState(ctx, p.SessionID)
	if err != nil {
		return err
	}
	if !found {
		return errs.New(errs.CodeSessionRevoked, "")
	}
	check := p
	a.describe(&check, st)
	if !check.SessionActive || !check.DeviceKeyActive ||
		check.DeviceState != actor.DeviceActive && check.DeviceState != actor.DevicePending {
		return errs.New(errs.CodeSessionRevoked, "")
	}
	return nil
}
