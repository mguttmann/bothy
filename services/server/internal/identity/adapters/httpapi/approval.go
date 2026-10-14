// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"net/netip"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// Approval implements the approval and pairing operations of rest.StrictServerInterface (M1-38):
// approveDevice, beginPairing, approvePairing and completePairing over identity.DeviceApproval.
// The public pairing endpoints verify the DPoP proof of the request here (a login proof without
// ath) and pass its thumbprint; the use cases authorize. Errors go to the problem writer of the
// chain unchanged.
type Approval struct {
	approval identity.DeviceApproval
	dpop     identity.DPoP
}

// NewApproval returns the handlers over the approval port and the DPoP verifier.
func NewApproval(a identity.DeviceApproval, dpop identity.DPoP) *Approval {
	return &Approval{approval: a, dpop: dpop}
}

// ApproveDevice implements rest.StrictServerInterface (POST /devices/{device_id}/approve). A wrong
// code answers 200 with the device in state revoked (08 section 5.2).
func (h *Approval) ApproveDevice(ctx context.Context, req rest.ApproveDeviceRequestObject) (rest.ApproveDeviceResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	d, err := h.approval.ApproveDevice(ctx, ids.ID(req.DeviceId), req.Body.VerificationCode)
	if err != nil {
		return nil, err
	}
	return rest.ApproveDevice200JSONResponse{Body: toREST(d)}, nil
}

// BeginPairing implements rest.StrictServerInterface (POST /auth/pairing/begin): the registration
// of the new device under a DPoP proof of its dpop key. The pairing code leaves the server only in
// this answer.
func (h *Approval) BeginPairing(ctx context.Context, req rest.BeginPairingRequestObject) (rest.BeginPairingResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	reg, err := registration(req.Body.Device)
	if err != nil {
		return nil, err
	}
	jkt, err := h.proof(ctx, req.Params.DPoP)
	if err != nil {
		return nil, err
	}
	out, err := h.approval.BeginPairing(ctx, reg, jkt, clientIP(ctx))
	if err != nil {
		return nil, err
	}
	return rest.BeginPairing201JSONResponse{Body: rest.PairingStarted{
		PairingId: rest.Uuid(out.ID), PairingCode: out.Code.Reveal(), ExpiresAt: ms(out.ExpiresAt),
	}}, nil
}

// ApprovePairing implements rest.StrictServerInterface (POST /me/devices/pairings).
func (h *Approval) ApprovePairing(ctx context.Context, req rest.ApprovePairingRequestObject) (rest.ApprovePairingResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	d, err := h.approval.ApprovePairing(ctx, req.Body.PairingCode)
	if err != nil {
		return nil, err
	}
	return rest.ApprovePairing201JSONResponse{Body: toREST(d)}, nil
}

// CompletePairing implements rest.StrictServerInterface (POST /auth/pairing/complete): 202 with
// the status while the pairing waits, 201 with the tokens once it is approved.
func (h *Approval) CompletePairing(ctx context.Context, req rest.CompletePairingRequestObject) (rest.CompletePairingResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	jkt, err := h.proof(ctx, req.Params.DPoP)
	if err != nil {
		return nil, err
	}
	out, err := h.approval.CompletePairing(ctx, ids.ID(req.Body.PairingId), jkt, clientIP(ctx))
	if err != nil {
		return nil, err
	}
	if out.Completed {
		return rest.CompletePairing201JSONResponse{Body: tokenResponse(out.Tokens)}, nil
	}
	return rest.CompletePairing202JSONResponse{Body: rest.PairingStatus{
		PairingId: rest.Uuid(out.PairingID), Status: rest.PairingStatusStatusRequested, ExpiresAt: ms(out.ExpiresAt),
	}}, nil
}

// proof verifies the DPoP proof of a public pairing request as a login proof and returns the
// thumbprint of its key; a proof with the previous nonce gets the current one in DPoP-Nonce.
func (h *Approval) proof(ctx context.Context, header string) (string, error) {
	p, err := h.dpop.Verify(ctx, loginProof(ctx, header))
	if err != nil {
		return "", err
	}
	if p.NonceStale {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, h.dpop.CurrentNonce())
	}
	return p.JKT, nil
}

// clientIP is the client address of the request, the zero Addr for none.
func clientIP(ctx context.Context) netip.Addr {
	ip, _ := reqmeta.ClientIP(ctx)
	return ip
}
