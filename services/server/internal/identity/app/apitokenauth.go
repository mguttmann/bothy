// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/apitoken"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// AuthenticateAPIToken is the API token path of the Authenticator (brief M1-44 "Authentication",
// ADR 0061 item 2); the HTTP chain reaches it through Authenticate and the gateway identify of
// M1-64 calls it with the token and proof of its frame. dpopScheme reports the scheme DPoP
// (false is Bearer); creds carries the proofs, method and htu.
//
// Order: format and checksum, kind whi refused, all without a database call; one query by
// HMAC-SHA-256 of the token with the pepper; the principal kind must fit the token kind. A
// revoked or expired token yields the actor of its principal with the token id and
// CredentialInactive: authorize refuses it in check_active with token_revoked and audits the
// denial (07 section 4.2), the Idempotency-Key stage passes it through without a claim or replay,
// and no proof is checked for it. A bound token (dpop_jkt) needs the scheme DPoP and a proof with exactly
// that key whose ath is the hash of the token; an unbound token under DPoP is refused. Success
// records last_used_at at most once per identity.last_active_granularity. The actor carries no
// session and no device, so it never has owner rights and never provides step-up (SR5).
//
// Errors: 401 unauthenticated (malformed, unknown, whi, kind mismatch, DPoP without binding),
// 401 invalid_dpop_proof or use_dpop_nonce from the verifier (Bearer with a bound token
// included), and the errors of the store.
func (a *Authenticator) AuthenticateAPIToken(ctx context.Context, token string, dpopScheme bool, creds actor.Credentials) (actor.Actor, error) {
	p, err := apitoken.Parse(token)
	if err != nil {
		return actor.Actor{}, unauthenticated()
	}
	wantKind, kind, ok := actorKindOf(p.Kind)
	if !ok {
		return actor.Actor{}, unauthenticated()
	}
	rec, found, err := a.api.APITokenByHash(ctx, apitoken.Hash(a.pepper, token))
	if err != nil {
		return actor.Actor{}, err
	}
	if !found || rec.PrincipalKind != wantKind {
		return actor.Actor{}, unauthenticated()
	}
	out := actor.Actor{ID: rec.PrincipalID, Kind: kind, TokenID: rec.ID, Scopes: rec.Scopes, CredentialExpiresAt: rec.ExpiresAt}
	now := a.clk.Now()
	if !rec.Active(now) {
		out.CredentialInactive = true
		return out, nil
	}
	if err := a.binding(ctx, rec.DPoPJKT, token, dpopScheme, creds); err != nil {
		return actor.Actor{}, err
	}
	if kind == actor.KindApp {
		if err := a.installation(ctx, &out); err != nil {
			return actor.Actor{}, err
		}
	}
	a.touchToken(ctx, rec.APITokenRecord)
	return out, nil
}

// installation reads the installation of the app of out (M2-24 design note 10 b): active gives
// the installation id and its granted scopes; otherwise the installation id stays zero and
// authorize answers unauthenticated app_not_installed and audits the denial (M1-28 liveness rule).
// It reads on every request; nothing is cached.
func (a *Authenticator) installation(ctx context.Context, out *actor.Actor) error {
	inst, found, err := a.apps.ActiveInstallation(ctx, out.ID)
	if err != nil || !found {
		return err
	}
	out.InstallationID, out.GrantedScopes = inst.ID, slices.Clone(inst.GrantedScopes)
	return nil
}

// binding checks the DPoP binding of an active token: a bound token needs the scheme DPoP and a
// proof of the key jkt over token (RFC 9449 section 7.2: a bound token as Bearer is refused); an
// unbound token must not come as DPoP.
func (a *Authenticator) binding(ctx context.Context, jkt, token string, dpopScheme bool, creds actor.Credentials) error {
	if jkt == "" {
		if dpopScheme {
			return unauthenticated()
		}
		return nil
	}
	if !dpopScheme {
		return &identity.DPoPError{Code: identity.DPoPInvalidProof, Nonce: a.dpop.CurrentNonce()}
	}
	proof, err := a.dpop.Verify(ctx, identity.ProofRequest{
		Proofs: proofs(creds), Method: creds.Method, URL: creds.HTU, AccessToken: token, ExpectedJKT: jkt,
	})
	if err != nil {
		return err
	}
	if proof.NonceStale {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, a.dpop.CurrentNonce())
	}
	return nil
}

// touchToken records the use of r outside the request's unit of work when last_used_at is older
// than identity.last_active_granularity; a failure is logged with the token id, never returned.
func (a *Authenticator) touchToken(ctx context.Context, r APITokenRecord) {
	now := a.clk.Now()
	threshold := now.Add(-a.cfg.LastActiveGranularity)
	if !r.LastUsedAt.IsZero() && !r.LastUsedAt.Before(threshold) {
		return
	}
	if err := a.api.TouchAPIToken(ctx, r.ID, now, threshold); err != nil {
		a.log.LogAttrs(ctx, slog.LevelWarn, msgTokenTouchFailed, slog.String(logKeyError, errs.KindOf(err).String()),
			slog.String(logKeyTokenID, r.ID.String()), slog.String(logKeyCorrID, reqmeta.CorrelationID(ctx)))
	}
}

// revalidateToken is Revalidate for the principal of an API token: the token is read again (no
// cache), and it must belong to p, be unrevoked and unexpired.
func (a *Authenticator) revalidateToken(ctx context.Context, p actor.Actor) error {
	r, found, err := a.api.APIToken(ctx, p.TokenID)
	if err != nil {
		return err
	}
	if !found || r.PrincipalID != p.ID || !r.Active(a.clk.Now()) {
		return unauthenticated()
	}
	if p.Kind == actor.KindApp {
		return a.revalidateInstallation(ctx, p)
	}
	return nil
}

// revalidateInstallation is the installation part of Revalidate for an app (M2-24 design note
// 11 a): the installation must still be active, be the one of the actor, and grant every scope
// the actor carries; otherwise the connection ends with unauthenticated app_not_installed.
func (a *Authenticator) revalidateInstallation(ctx context.Context, p actor.Actor) error {
	inst, found, err := a.apps.ActiveInstallation(ctx, p.ID)
	if err != nil {
		return err
	}
	if !found || inst.ID != p.InstallationID || slices.ContainsFunc(p.GrantedScopes, func(s string) bool { return !slices.Contains(inst.GrantedScopes, s) }) {
		return errs.New(errs.CodeUnauthenticated, "").WithHeader("WWW-Authenticate", schemeChallenge).WithReason(reasonAppNotInstalled)
	}
	return nil
}
