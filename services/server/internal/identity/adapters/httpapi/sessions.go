// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"net/http"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// headerDPoPNonce carries the current nonce (06 section 3.1 item 2).
const headerDPoPNonce = "DPoP-Nonce"

// tokenTypeDPoP is TokenResponse.token_type, always DPoP (RFC 9449).
const tokenTypeDPoP = "DPoP"

// Sessions implements the session operations of rest.StrictServerInterface (M1-34): refreshToken,
// logout, listMySessions, listUserSessions and createExtensionSession over identity.Auth. The use
// cases authorize; the handlers convert, and refreshToken verifies the DPoP proof that binds the
// new tokens first. Errors go to the problem writer of the chain unchanged.
type Sessions struct {
	auth identity.Auth
	dpop identity.DPoP
}

// NewSessions returns the handlers over the session port and the DPoP verifier.
func NewSessions(auth identity.Auth, dpop identity.DPoP) *Sessions {
	return &Sessions{auth: auth, dpop: dpop}
}

// RefreshToken implements rest.StrictServerInterface (POST /auth/token/refresh): the proof of the
// DPoP header (every value the request carries, method and htu from the chain, no access token)
// is verified, then the refresh token rotates under its key. A proof with the previous nonce gets
// the current one in DPoP-Nonce.
func (h *Sessions) RefreshToken(ctx context.Context, req rest.RefreshTokenRequestObject) (rest.RefreshTokenResponseObject, error) {
	p, _ := reqmeta.ProofOf(ctx)
	values := p.Values
	if values == nil && req.Params.DPoP != "" {
		values = []string{req.Params.DPoP}
	}
	method := p.Method
	if method == "" {
		method = http.MethodPost
	}
	proof, err := h.dpop.Verify(ctx, identity.ProofRequest{Proofs: values, Method: method, URL: p.HTU})
	if err != nil {
		return nil, err
	}
	if proof.NonceStale {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, h.dpop.CurrentNonce())
	}
	var token string
	if req.Body != nil {
		token = req.Body.RefreshToken
	}
	set, err := h.auth.Refresh(ctx, token, proof)
	if err != nil {
		return nil, err
	}
	return rest.RefreshToken200JSONResponse{Body: tokenResponse(set)}, nil
}

// Logout implements rest.StrictServerInterface (POST /auth/logout).
func (h *Sessions) Logout(ctx context.Context, _ rest.LogoutRequestObject) (rest.LogoutResponseObject, error) {
	if err := h.auth.Logout(ctx, principal(ctx)); err != nil {
		return nil, err
	}
	return rest.Logout204Response{}, nil
}

// ListMySessions implements rest.StrictServerInterface (GET /me/sessions).
func (h *Sessions) ListMySessions(ctx context.Context, req rest.ListMySessionsRequestObject) (rest.ListMySessionsResponseObject, error) {
	a := principal(ctx)
	p, err := h.auth.ListSessions(ctx, a, a.ID, pageOf(req.Params.Cursor, req.Params.Limit))
	if err != nil {
		return nil, err
	}
	return rest.ListMySessions200JSONResponse{Body: sessionPage(p)}, nil
}

// ListUserSessions implements rest.StrictServerInterface (GET /users/{user_id}/sessions).
func (h *Sessions) ListUserSessions(ctx context.Context, req rest.ListUserSessionsRequestObject) (rest.ListUserSessionsResponseObject, error) {
	p, err := h.auth.ListSessions(ctx, principal(ctx), ids.ID(req.UserId), pageOf(req.Params.Cursor, req.Params.Limit))
	if err != nil {
		return nil, err
	}
	return rest.ListUserSessions200JSONResponse{Body: sessionPage(p)}, nil
}

// CreateExtensionSession implements rest.StrictServerInterface (POST /me/sessions/extensions).
func (h *Sessions) CreateExtensionSession(ctx context.Context, req rest.CreateExtensionSessionRequestObject) (rest.CreateExtensionSessionResponseObject, error) {
	var component identity.ClientComponent
	if req.Body != nil {
		component = identity.ClientComponent(req.Body.ClientComponent)
	}
	set, err := h.auth.CreateExtensionSession(ctx, principal(ctx), component)
	if err != nil {
		return nil, err
	}
	return rest.CreateExtensionSession201JSONResponse{Body: tokenResponse(set)}, nil
}

// tokenResponse converts a token set; the token values leave the server only here.
func tokenResponse(s identity.TokenSet) rest.TokenResponse {
	return rest.TokenResponse{
		AccessToken: s.AccessToken.Reveal(), RefreshToken: s.RefreshToken.Reveal(), TokenType: tokenTypeDPoP,
		ExpiresIn: s.ExpiresIn, RefreshExpiresAt: ms(s.RefreshExpiresAt), Session: sessionREST(s.Session), Device: toREST(s.Device),
	}
}

// sessionPage converts a page of sessions.
func sessionPage(p identity.SessionPage) rest.SessionPage {
	out := rest.SessionPage{Items: make([]rest.Session, 0, len(p.Items))}
	for _, s := range p.Items {
		out.Items = append(out.Items, sessionREST(s))
	}
	if p.NextCursor != "" {
		out.NextCursor = &p.NextCursor
	}
	return out
}

// sessionREST converts a session; empty values become absent members, never null (ADR 0053), and
// current is present only for the session of the request.
func sessionREST(s identity.Session) rest.Session {
	out := rest.Session{
		Id: rest.Uuid(s.ID), UserId: rest.Uuid(s.UserID), DeviceId: rest.Uuid(s.DeviceID), AuthMethod: rest.SessionAuthMethod(s.AuthMethod),
		ClientComponent: (*rest.SessionClientComponent)(optional(string(s.ClientComponent))), CreatedAt: ms(s.CreatedAt),
		ExpiresAt: ms(s.ExpiresAt), StepUpAt: optionalTime(s.StepUpAt), StepUpMethod: (*rest.SessionStepUpMethod)(optional(s.StepUpMethod)),
		LastSeenAt: optionalTime(s.LastSeenAt),
	}
	if s.Current {
		t := true
		out.Current = &t
	}
	return out
}
