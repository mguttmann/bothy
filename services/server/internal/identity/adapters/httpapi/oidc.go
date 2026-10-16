// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// OIDC implements the OIDC operations of rest.StrictServerInterface (M1-41): beginOidcLogin,
// handleOidcCallback, completeOidcLogin, beginIdentityLink and completeIdentityLink over
// identity.OIDCLogin. The use cases authorize, verify and audit; the handlers convert. None of
// them carries an Idempotency-Key (06 "Idempotenz" item 3), and the chain sets Cache-Control:
// no-store and Referrer-Policy: no-referrer on every answer, the redirect included.
type OIDC struct {
	oidc identity.OIDCLogin
}

// NewOIDC returns the handlers over the OIDC port.
func NewOIDC(o identity.OIDCLogin) *OIDC { return &OIDC{oidc: o} }

// BeginOidcLogin implements rest.StrictServerInterface (POST /auth/oidc/begin).
func (h *OIDC) BeginOidcLogin(ctx context.Context, req rest.BeginOidcLoginRequestObject) (rest.BeginOidcLoginResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	in := identity.OIDCBegin{
		ProviderID: ids.ID(req.Body.ProviderId), AppRedirectURI: req.Body.AppRedirectUri, ClientChallenge: req.Body.ClientChallenge,
		Console: consoleForm(req.Body.Client),
	}
	if req.Body.EnrollmentCode != nil {
		in.EnrollmentCode = *req.Body.EnrollmentCode
	}
	out, err := h.oidc.BeginOIDCLogin(ctx, in)
	if err != nil {
		return nil, err
	}
	return rest.BeginOidcLogin200JSONResponse{Body: beginREST(out)}, nil
}

// HandleOidcCallback implements rest.StrictServerInterface (GET /auth/oidc/callback): the
// redirect of the provider. It answers 302 to the app, or a problem when the state names no
// usable flow. The query values never reach a log: the chain logs the route pattern only.
func (h *OIDC) HandleOidcCallback(ctx context.Context, req rest.HandleOidcCallbackRequestObject) (rest.HandleOidcCallbackResponseObject, error) {
	p := req.Params
	in := identity.OIDCCallback{State: p.State, HasIss: p.Iss != nil}
	for dst, v := range map[*string]*string{&in.Code: p.Code, &in.Iss: p.Iss, &in.Error: p.Error} {
		if v != nil {
			*dst = *v
		}
	}
	location, err := h.oidc.HandleOIDCCallback(ctx, in)
	if err != nil {
		return nil, err
	}
	return rest.HandleOidcCallback302Response{Headers: rest.HandleOidcCallback302ResponseHeaders{Location: &location}}, nil
}

// CompleteOidcLogin implements rest.StrictServerInterface (POST /auth/oidc/complete): the
// login_code with the client_verifier, the device registration and the DPoP proof of the
// request. A proof with the previous nonce gets the current one in DPoP-Nonce.
func (h *OIDC) CompleteOidcLogin(ctx context.Context, req rest.CompleteOidcLoginRequestObject) (rest.CompleteOidcLoginResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	console := consoleForm(req.Body.Client)
	reg, err := loginDevice(console, req.Body.Device)
	if err != nil {
		return nil, err
	}
	in := identity.OIDCComplete{
		LoginCode: req.Body.LoginCode, ClientVerifier: req.Body.ClientVerifier, Device: reg, Proof: loginProof(ctx, dpopHeader(req.Params.DPoP)),
		Console: console,
	}
	if ip, ok := reqmeta.ClientIP(ctx); ok {
		in.ClientIP = ip
	}
	out, err := h.oidc.CompleteOIDCLogin(ctx, in)
	if err != nil {
		return nil, err
	}
	if c := out.Console; c != nil {
		return rest.CompleteOidcLogin200JSONResponse{ConsoleLoginJSONResponse: consoleLogin(c)}, nil
	}
	if out.Nonce != "" {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, out.Nonce)
	}
	return rest.CompleteOidcLogin201JSONResponse{Body: tokenResponse(out.Tokens)}, nil
}

// BeginIdentityLink implements rest.StrictServerInterface (POST /me/identities/begin).
func (h *OIDC) BeginIdentityLink(ctx context.Context, req rest.BeginIdentityLinkRequestObject) (rest.BeginIdentityLinkResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	out, err := h.oidc.BeginIdentityLink(ctx, principal(ctx), identity.OIDCBegin{
		ProviderID: ids.ID(req.Body.ProviderId), AppRedirectURI: req.Body.AppRedirectUri, ClientChallenge: req.Body.ClientChallenge,
	})
	if err != nil {
		return nil, err
	}
	return rest.BeginIdentityLink200JSONResponse{Body: beginREST(out)}, nil
}

// CompleteIdentityLink implements rest.StrictServerInterface (POST /me/identities/complete).
func (h *OIDC) CompleteIdentityLink(ctx context.Context, req rest.CompleteIdentityLinkRequestObject) (rest.CompleteIdentityLinkResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	out, err := h.oidc.CompleteIdentityLink(ctx, principal(ctx), req.Body.LoginCode, req.Body.ClientVerifier)
	if err != nil {
		return nil, err
	}
	return rest.CompleteIdentityLink201JSONResponse{Body: identityREST(out)}, nil
}

// beginREST converts the answer of a begin (schema OidcBeginResponse).
func beginREST(o identity.OIDCBeginResult) rest.OidcBeginResponse {
	return rest.OidcBeginResponse{LoginAttemptId: rest.Uuid(o.LoginAttemptID), AuthorizationUrl: o.AuthorizationURL, ExpiresAt: ms(o.ExpiresAt)}
}
