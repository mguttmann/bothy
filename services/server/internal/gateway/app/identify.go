// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// defaultConsumer is the consumer of an agent or app that names none (04 section 6.3 step 3).
const defaultConsumer = "default"

// revokedReasons are the reasons of an unauthenticated decision that close with 4012.
var revokedReasons = map[authorize.Reason]bool{
	"principal_inactive": true, "session_revoked": true, "device_inactive": true, "token_revoked": true, "app_not_installed": true,
}

// identify is the handler of identify (notes 6 to 13 of the brief).
func (g *Gateway) identify(ctx context.Context, c *Connection, d any) error {
	if g.isDraining() {
		return closeWith(domain.CloseServerRestart)
	}
	in, err := g.checkIdentify(d)
	if err != nil {
		return err
	}
	a, err := g.connect(ctx, in.AccessToken, in.APIToken, in.DPoP, func(a actor.Actor) bool {
		return a.Kind != actor.KindUser || !in.DeviceID.IsZero() && in.DeviceID == a.DeviceID
	})
	if err != nil {
		return err
	}
	ctx = actor.WithActor(ctx, a)
	s := newSession(a, in)
	replace(g.sessions.bind(s, c))
	if !c.become(s, a, g.d.Clock.Now()) {
		// The connection closed while identify ran; Closed found no session, so the new one
		// leaves the store here.
		g.sessions.release(s, c, false)
		return nil
	}
	g.d.Delivery.Register(ctx, c, s)
	view, err := g.d.Snapshot.Build(ctx)
	if err != nil {
		return setupClose(err)
	}
	if err := c.conn.Send(Ready{SessionID: s.ID, DeviceID: a.DeviceID, View: view}); err != nil {
		return nil
	}
	g.connected(ctx, c, string(a.Kind))
	// The heartbeat deadline runs from ready (note 14), not from the start of identify.
	c.beat(g.d.Clock.Now())
	if err := g.attach(ctx, c, in.Cursor, in.SyncEpoch, view.HeadSeq); err != nil {
		return setupClose(err)
	}
	return nil
}

// checkIdentify checks the body of identify before any credential is looked at: exactly one
// token, a cursor of at least 0 and a consumer that is not empty (4002), a supported protocol
// version (4007).
func (g *Gateway) checkIdentify(d any) (Identify, error) {
	in, ok := d.(Identify)
	if !ok || (in.AccessToken == "") == (in.APIToken == "") || in.Cursor != nil && *in.Cursor < 0 || in.Consumer != nil && *in.Consumer == "" {
		return Identify{}, closeWith(domain.CloseDecodeError)
	}
	if !slices.Contains(g.cfg.ProtocolVersions, in.ProtocolVersion) {
		return Identify{}, closeWith(domain.CloseProtocolVersionUnsupported)
	}
	return in, nil
}

// newSession returns the session of a fresh identify of a: a new UUIDv7, the principal, device,
// auth session and token, the consumer of an agent or app (default "default") and the client.
func newSession(a actor.Actor, in Identify) *Session {
	s := &Session{ID: ids.New(), PrincipalID: a.ID, PrincipalKind: a.Kind, DeviceID: a.DeviceID, AuthSessionID: a.SessionID, TokenID: a.TokenID}
	if a.Kind != actor.KindUser {
		s.Consumer = defaultConsumer
		if in.Consumer != nil {
			s.Consumer = *in.Consumer
		}
	}
	if in.Client != nil {
		s.Client = *in.Client
	}
	return s
}

// connect authenticates token (an access token or an API token) with its proof, requires
// matches of the verified actor (the device of identify; 4004 otherwise), takes a token of
// rl.gateway_connect for the verified device or token and authorizes gateway.connect on the
// instance; the errors are the close codes of the table of 04 section 6.4 (note 10).
func (g *Gateway) connect(ctx context.Context, accessToken, apiToken, proof string, matches func(actor.Actor) bool) (actor.Actor, error) {
	a, err := g.authenticate(ctx, accessToken+apiToken, proof)
	if err != nil {
		return actor.Actor{}, authClose(err)
	}
	if !matches(a) {
		return actor.Actor{}, closeWith(domain.CloseAuthenticationFailed)
	}
	allowed, retry, err := g.d.Limiter.AllowConnect(a)
	if err != nil {
		return actor.Actor{}, closeWith(domain.CloseAuthenticationFailed)
	}
	if !allowed {
		return actor.Actor{}, &CloseError{Code: domain.CloseRateLimited, Reason: "retry_after=" + strconv.Itoa(retry)}
	}
	if err := g.authorizeConnect(actor.WithActor(ctx, a), a); err != nil {
		return actor.Actor{}, err
	}
	return a, nil
}

// authenticate runs the Authenticator on the token and proof of a frame like on a request GET
// of the gateway URL: the scheme DPoP when a proof is sent, Bearer otherwise (an API token
// without dpop_jkt), with the htu of the configuration and the client IP of the upgrade.
func (g *Gateway) authenticate(ctx context.Context, token, proof string) (actor.Actor, error) {
	ip, _ := reqmeta.ClientIP(ctx)
	creds := actor.Credentials{Method: "GET", HTU: g.cfg.HTU, Authorization: "Bearer " + token, ClientIP: ip}
	if proof != "" {
		creds.Authorization, creds.DPoP, creds.DPoPProofs = "DPoP "+token, proof, []string{proof}
	}
	return g.d.Authenticator.Authenticate(ctx, creds)
}

// authorizeConnect authorizes gateway.connect for a on the instance and maps a denial to its
// close code: unauthenticated with a revocation reason 4012, other unauthenticated 4004, a
// pending device or person 4016, any other denial 4018; an error of the check 4000.
func (g *Gateway) authorizeConnect(ctx context.Context, a actor.Actor) error {
	dec, err := g.d.Authorizer.Authorize(ctx, a, authorize.ActionGatewayConnect, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return closeWith(domain.CloseUnknownError)
	}
	switch {
	case dec.Allowed():
		return nil
	case dec.Outcome == authorize.OutcomeUnauthenticated && revokedReasons[dec.Reason]:
		return closeWith(domain.CloseRevoked)
	case dec.Outcome == authorize.OutcomeUnauthenticated:
		return closeWith(domain.CloseAuthenticationFailed)
	}
	return closeWith(forbiddenClose(dec))
}

// authClose maps an error of the Authenticator to its close code: an expired access token 4011, a
// session the authenticator revoked itself 4012, an unavailable dependency or an internal error
// 4000, every other rejection (token, signature, iss, aud, proof, nonce, jti) 4004.
func authClose(err error) error {
	e, ok := errs.As(err)
	if !ok {
		return closeWith(domain.CloseUnknownError)
	}
	switch {
	case e.Code == errs.CodeTokenExpired:
		return closeWith(domain.CloseTokenExpired)
	case e.Code == errs.CodeSessionRevoked:
		return closeWith(domain.CloseRevoked)
	case e.Kind() == errs.KindUnauthenticated:
		return closeWith(domain.CloseAuthenticationFailed)
	}
	return closeWith(domain.CloseUnknownError)
}

// setupClose maps an error while building ready or resumed (note 10): a pending device 4016,
// any other forbidden 4018 (an app, whose token passes gateway.connect, may not read the
// snapshot: sync.snapshot.read has no app scope in the catalogue; docs/DECISIONS.md M1-64/11), a
// revoked session 4012, another unauthenticated 4004, everything else (the database unreachable
// above all) 4000.
func setupClose(err error) error {
	var ce *CloseError
	if errors.As(err, &ce) {
		return ce
	}
	e, ok := errs.As(err)
	switch {
	case !ok:
		return closeWith(domain.CloseUnknownError)
	case e.Code == errs.CodeDevicePending:
		return closeWith(domain.CloseDevicePending)
	case e.Kind() == errs.KindForbidden:
		return closeWith(domain.CloseForbidden)
	case e.Code == errs.CodeSessionRevoked:
		return closeWith(domain.CloseRevoked)
	case e.Kind() == errs.KindUnauthenticated:
		return closeWith(domain.CloseAuthenticationFailed)
	}
	return closeWith(domain.CloseUnknownError)
}
