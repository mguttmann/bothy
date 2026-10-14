// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/idempotency"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// Enrollment implements the enrollment code operations of rest.StrictServerInterface (M1-37):
// redeemEnrollmentCode and createEnrollmentCode over identity.Enrollment. The use cases authorize
// and verify the proof; the handlers convert. Errors go to the problem writer of the chain
// unchanged.
type Enrollment struct {
	enrollment identity.Enrollment
}

// NewEnrollment returns the handlers over the enrollment port.
func NewEnrollment(e identity.Enrollment) *Enrollment {
	return &Enrollment{enrollment: e}
}

// RedeemEnrollmentCode implements rest.StrictServerInterface (POST /auth/enroll/redeem): the code
// with the device registration and the DPoP proof of the request (every value it carries, method
// and htu from the chain, no access token). A proof with the previous nonce gets the current one in
// DPoP-Nonce.
func (h *Enrollment) RedeemEnrollmentCode(ctx context.Context, req rest.RedeemEnrollmentCodeRequestObject) (rest.RedeemEnrollmentCodeResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	reg, err := registration(req.Body.Device)
	if err != nil {
		return nil, err
	}
	in := identity.RedeemEnrollment{Code: req.Body.EnrollmentCode, Device: reg, Proof: loginProof(ctx, req.Params.DPoP)}
	if ip, ok := reqmeta.ClientIP(ctx); ok {
		in.ClientIP = ip
	}
	out, err := h.enrollment.RedeemEnrollmentCode(ctx, in)
	if err != nil {
		return nil, err
	}
	if out.Nonce != "" {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, out.Nonce)
	}
	return rest.RedeemEnrollmentCode201JSONResponse{Body: tokenResponse(out.Tokens)}, nil
}

// CreateEnrollmentCode implements rest.StrictServerInterface (POST /users/{user_id}/enrollment-codes).
// The code is a one-time secret: the Idempotency-Key stage keeps no response that holds it and a
// replay answers 409 one_time_secret (06 "Idempotency", R-API-26).
func (h *Enrollment) CreateEnrollmentCode(ctx context.Context, req rest.CreateEnrollmentCodeRequestObject) (rest.CreateEnrollmentCodeResponseObject, error) {
	c, err := h.enrollment.CreateEnrollmentCode(ctx, principal(ctx), ids.ID(req.UserId))
	if err != nil {
		return nil, err
	}
	idempotency.MarkOneTimeSecret(ctx)
	return rest.CreateEnrollmentCode201JSONResponse{Body: enrollmentREST(c)}, nil
}

// enrollmentREST converts an issued code; the code value leaves the server only here.
func enrollmentREST(c identity.EnrollmentCode) rest.EnrollmentCode {
	return rest.EnrollmentCode{Code: c.Code.Reveal(), Purpose: rest.EnrollmentCodePurpose(c.Purpose), ExpiresAt: ms(c.ExpiresAt)}
}

// loginProof is the proof request of a login endpoint: every DPoP value of the request, method and
// htu from the chain, no access token and no expected key.
func loginProof(ctx context.Context, header string) identity.ProofRequest {
	p, _ := reqmeta.ProofOf(ctx)
	values := p.Values
	if values == nil && header != "" {
		values = []string{header}
	}
	method := p.Method
	if method == "" {
		method = http.MethodPost
	}
	return identity.ProofRequest{Proofs: values, Method: method, URL: p.HTU}
}

// registration converts the schema DeviceRegistration; the JWK objects are passed on as JSON and
// validated by the device rules (kernel/jose).
func registration(d rest.DeviceRegistration) (identity.RegisterDevice, error) {
	jwk, err := jsonObject(d.PublicJwk)
	if err != nil {
		return identity.RegisterDevice{}, err
	}
	out := identity.RegisterDevice{
		DeviceID: ids.ID(d.DeviceId), Name: d.Name, Platform: identity.Platform(d.Platform),
		DPoPKey: identity.PublicKey{JWK: jwk, Alg: string(d.Alg)},
	}
	if d.AppVersion != nil {
		out.AppVersion = *d.AppVersion
	}
	if d.Attestation != nil {
		if out.DPoPKey.Attestation, err = jsonObject(*d.Attestation); err != nil {
			return identity.RegisterDevice{}, err
		}
	}
	if k := d.StepUpKey; k != nil {
		sk, err := jsonObject(k.PublicJwk)
		if err != nil {
			return identity.RegisterDevice{}, err
		}
		out.StepUpKey = &identity.PublicKey{JWK: sk, Alg: string(k.Alg)}
		if k.Attestation != nil {
			if out.StepUpKey.Attestation, err = jsonObject(*k.Attestation); err != nil {
				return identity.RegisterDevice{}, err
			}
		}
	}
	return out, nil
}

// jsonObject encodes a decoded JSON object back to JSON.
func jsonObject(o rest.JsonObject) (json.RawMessage, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return nil, errs.Wrap(errs.CodeValidationFailed, err, "")
	}
	return b, nil
}
