// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// The schemes of the Authorization header (06 section 3).
const (
	schemeDPoP   = "dpop"
	schemeBearer = "bearer"
)

// schemeChallenge is the WWW-Authenticate of a request whose credentials are not usable at all.
const schemeChallenge = `DPoP algs="` + identity.DPoPAlgs + `"`

// headerDPoPNonce carries the current nonce (06 section 3.1 item 2).
const headerDPoPNonce = "DPoP-Nonce"

// Log messages and keys of failed activity writes; the keys are constants for sloglint.
const (
	msgTouchFailed      = "recording session activity failed"
	msgTokenTouchFailed = "recording API token use failed"
	logKeyError         = "error_type"
	logKeyTokenID       = "token_id"
)

// Authenticator implements kernel/actor.Authenticator (brief M1-34 "Authenticator", brief M1-44
// "Authentication"): for people the access token, the DPoP proof, then one query for session,
// device and key; for API tokens the format and checksum, then one query by HMAC, then the DPoP
// binding. It does not reject on the state of a session, a device or a found API token: it
// reports it in the actor, and authorize decides (check_active), so the denial is audited with its
// reason (07 section 4.2; token_revoked for a revoked or expired token). Nothing about a token is
// cached here, so a revocation holds at the next request in every process (AZ-P9). It is safe for
// concurrent use.
type Authenticator struct {
	cfg    Config
	tokens *AccessTokens
	dpop   identity.DPoP
	store  SessionStore
	api    APITokenStore
	pepper []byte
	clk    clock.Clock
	log    *slog.Logger
}

// Compile-time check: the Authenticator is the kernel port.
var _ actor.Authenticator = (*Authenticator)(nil)

// AuthenticatorDeps are the collaborators of the Authenticator.
type AuthenticatorDeps struct {
	// AccessTokens verifies the access tokens of people.
	AccessTokens *AccessTokens
	// DPoP verifies the proofs of access tokens and bound API tokens.
	DPoP identity.DPoP
	// Sessions reads session, device and key of a person's request.
	Sessions SessionStore
	// APITokens reads API tokens by hash and id (M1-44).
	APITokens APITokenStore
	// Pepper is the raw token pepper, the HMAC key of the API token lookup.
	Pepper []byte
	// Clock is the time of the state checks; nil means clock.System().
	Clock clock.Clock
	// Log receives failed activity writes; nil means slog.Default().
	Log *slog.Logger
}

// NewAuthenticator returns the Authenticator with identity.last_active_granularity of cfg (not
// negative) and the collaborators of d. Errors: a missing collaborator, an empty pepper or a
// negative granularity.
func NewAuthenticator(cfg Config, d AuthenticatorDeps) (*Authenticator, error) {
	if d.AccessTokens == nil || d.DPoP == nil || d.Sessions == nil || d.APITokens == nil || len(d.Pepper) == 0 {
		return nil, errors.New("identity: NewAuthenticator needs the access tokens, the DPoP verifier, the session and API token stores and the pepper")
	}
	if cfg.LastActiveGranularity < 0 || cfg.DevicePendingTTL <= 0 {
		return nil, errors.New("identity: identity.last_active_granularity must not be negative and auth.device_pending_ttl must be positive")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}
	return &Authenticator{
		cfg: cfg, tokens: d.AccessTokens, dpop: d.DPoP, store: d.Sessions, api: d.APITokens, pepper: append([]byte(nil), d.Pepper...),
		clk: d.Clock, log: d.Log,
	}, nil
}

// unauthenticated is the 401 of credentials the server cannot use.
func unauthenticated() error {
	return errs.New(errs.CodeUnauthenticated, "").WithHeader("WWW-Authenticate", schemeChallenge)
}

// Authenticate implements actor.Authenticator in the order of the brief: no Authorization header
// is anonymous; more than one header or another scheme is unauthenticated; a token with the
// prefix bothy_ takes the API token path under either scheme (AuthenticateAPIToken); otherwise
// Bearer with this server's access token fails with invalid_dpop_proof (RFC 9449 section 7.2) and
// DPoP checks the access token, then the proof, then reads session, device and key.
func (a *Authenticator) Authenticate(ctx context.Context, creds actor.Credentials) (actor.Actor, error) {
	if !creds.Present() {
		return actor.Anonymous(), nil
	}
	scheme, token, ok := splitAuthorization(creds.Authorization)
	if !ok || scheme != schemeBearer && scheme != schemeDPoP {
		return actor.Actor{}, unauthenticated()
	}
	if apitoken.HasPrefix(token) {
		return a.AuthenticateAPIToken(ctx, token, scheme == schemeDPoP, creds)
	}
	if scheme == schemeBearer {
		return actor.Actor{}, a.bearer(ctx, token)
	}
	return a.person(ctx, creds, token)
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

// bearer is the Bearer branch of a value that is no API token: a DPoP-bound access token of this
// server is invalid_dpop_proof (ID-T14), anything else unauthenticated.
func (a *Authenticator) bearer(ctx context.Context, token string) error {
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
// counts only for the device of the token, and a pending device past auth.device_pending_ttl is
// revoked, so authorize answers 401 (device_inactive) even before the cleanup job revokes it
// (M1-38); the key is active only when cnf.jkt is the device's active dpop key.
func (a *Authenticator) describe(out *actor.Actor, st AuthState) {
	s := st.Session
	now := a.clk.Now()
	own := s.UserID == out.ID && s.DeviceID == out.DeviceID
	out.SessionActive = own && s.Active(now)
	if own {
		out.DeviceState = string(st.DeviceState)
		if st.DeviceState == identity.DeviceStatePending && !now.Before(st.DeviceCreatedAt.Add(a.cfg.DevicePendingTTL)) {
			out.DeviceState = actor.DeviceRevoked
		}
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

// Revalidate implements actor.Authenticator: the anonymous principal passes; a principal of an
// API token passes while the token exists, belongs to it, is not revoked and has not expired
// (else 401 unauthenticated); a person with a session passes while the session is active,
// belongs to it and its device is pending or active with the same key (else 401
// session_revoked); everything else is unauthenticated.
func (a *Authenticator) Revalidate(ctx context.Context, p actor.Actor) error {
	if p.IsAnonymous() {
		return nil
	}
	if !p.TokenID.IsZero() {
		return a.revalidateToken(ctx, p)
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
