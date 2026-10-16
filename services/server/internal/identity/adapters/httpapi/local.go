// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// Local implements the operations of local accounts of rest.StrictServerInterface (M1-39):
// loginWithPassword and setMyLocalCredential over identity.LocalCredentials. The use cases check
// the switch, authorize and verify the proof; the handlers convert. Errors go to the problem
// writer of the chain unchanged.
type Local struct {
	local identity.LocalCredentials
}

// NewLocal returns the handlers over the local credential port.
func NewLocal(l identity.LocalCredentials) *Local { return &Local{local: l} }

// LoginWithPassword implements rest.StrictServerInterface (POST /auth/password/login): login
// name, password and TOTP code with the device registration and the DPoP proof of the request. A
// proof with the previous nonce gets the current one in DPoP-Nonce.
func (h *Local) LoginWithPassword(ctx context.Context, req rest.LoginWithPasswordRequestObject) (rest.LoginWithPasswordResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	console := consoleForm(req.Body.Client)
	reg, err := loginDevice(console, req.Body.Device)
	if err != nil {
		return nil, err
	}
	in := identity.PasswordLogin{
		LoginName: req.Body.LoginName, Password: req.Body.Password, TOTPCode: req.Body.TotpCode, Device: reg,
		Proof: loginProof(ctx, dpopHeader(req.Params.DPoP)), Console: console,
	}
	if ip, ok := reqmeta.ClientIP(ctx); ok {
		in.ClientIP = ip
	}
	out, err := h.local.LoginWithPassword(ctx, in)
	if err != nil {
		return nil, err
	}
	if c := out.Console; c != nil {
		return rest.LoginWithPassword200JSONResponse{ConsoleLoginJSONResponse: consoleLogin(c)}, nil
	}
	if out.Nonce != "" {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, out.Nonce)
	}
	return rest.LoginWithPassword201JSONResponse{Body: tokenResponse(out.Tokens)}, nil
}

// SetMyLocalCredential implements rest.StrictServerInterface (PUT /me/local-credential). A
// response with totp_secret_uri is a one-time secret: an Idempotency-Key stage keeps no response
// that holds it (06 "Idempotenz", R-API-26).
func (h *Local) SetMyLocalCredential(ctx context.Context, req rest.SetMyLocalCredentialRequestObject) (rest.SetMyLocalCredentialResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	out, err := h.local.SetLocalCredential(ctx, principal(ctx), identity.SetLocalCredential{
		Password: req.Body.Password, TOTPEnroll: req.Body.TotpEnroll, TOTPCode: req.Body.TotpCode,
	})
	if err != nil {
		return nil, err
	}
	body := rest.LocalCredentialResult{PasswordSet: out.PasswordSet, TotpEnabled: out.TOTPEnabled}
	if uri := out.TOTPSecretURI.Reveal(); uri != "" {
		idempotency.MarkOneTimeSecret(ctx)
		body.TotpSecretUri = &uri
	}
	return rest.SetMyLocalCredential200JSONResponse{Body: body}, nil
}
