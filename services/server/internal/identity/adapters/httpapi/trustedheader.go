// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// TrustedHeader implements loginWithTrustedHeader of rest.StrictServerInterface (M1-43) over
// identity.TrustedHeaderLogins. The use case authorizes, evaluates the headers and verifies the
// proof; the handler converts and passes the raw TCP peer, never a forwarded address.
type TrustedHeader struct {
	logins identity.TrustedHeaderLogins
}

// NewTrustedHeader returns the handler over the port.
func NewTrustedHeader(l identity.TrustedHeaderLogins) *TrustedHeader {
	return &TrustedHeader{logins: l}
}

// LoginWithTrustedHeader implements rest.StrictServerInterface (POST
// /auth/trusted-header/login): the registration, the optional enrollment code, the DPoP proof of
// the request and the transport facts of the chain (peer and header). A proof with the previous
// nonce gets the current one in DPoP-Nonce.
func (h *TrustedHeader) LoginWithTrustedHeader(ctx context.Context, req rest.LoginWithTrustedHeaderRequestObject) (rest.LoginWithTrustedHeaderResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	console := consoleForm(req.Body.Client)
	reg, err := loginDevice(console, req.Body.Device)
	if err != nil {
		return nil, err
	}
	in := identity.TrustedHeaderLogin{Device: reg, Proof: loginProof(ctx, dpopHeader(req.Params.DPoP)), Console: console}
	if req.Body.EnrollmentCode != nil {
		in.EnrollmentCode = *req.Body.EnrollmentCode
	}
	if t, ok := reqmeta.TransportOf(ctx); ok {
		in.Peer, in.Header = t.Peer, t.Header
	}
	if ip, ok := reqmeta.ClientIP(ctx); ok {
		in.ClientIP = ip
	}
	out, err := h.logins.LoginWithTrustedHeader(ctx, in)
	if err != nil {
		return nil, err
	}
	if c := out.Console; c != nil {
		return rest.LoginWithTrustedHeader200JSONResponse{ConsoleLoginJSONResponse: consoleLogin(c)}, nil
	}
	if out.Nonce != "" {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, out.Nonce)
	}
	return rest.LoginWithTrustedHeader201JSONResponse{Body: tokenResponse(out.Tokens)}, nil
}
