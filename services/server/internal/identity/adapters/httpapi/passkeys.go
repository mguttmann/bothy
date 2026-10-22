// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/reqmeta"
)

// Passkeys implements the passkey operations of rest.StrictServerInterface (M1-40):
// beginPasskeyLogin, completePasskeyLogin, beginEnrollment, completeEnrollment,
// beginPasskeyRegistration, completePasskeyRegistration and revokePasskey over identity.Passkeys,
// and completeStepUpWithPasskey over identity.StepUp. The use cases authorize, verify the proofs
// and the WebAuthn responses; the handlers convert. Errors go to the problem writer of the chain
// unchanged.
type Passkeys struct {
	passkeys identity.Passkeys
	stepUp   identity.StepUp
}

// NewPasskeys returns the handlers over the passkey and step-up ports.
func NewPasskeys(p identity.Passkeys, s identity.StepUp) *Passkeys {
	return &Passkeys{passkeys: p, stepUp: s}
}

// BeginPasskeyLogin implements rest.StrictServerInterface (POST /auth/passkey/begin).
func (h *Passkeys) BeginPasskeyLogin(ctx context.Context, req rest.BeginPasskeyLoginRequestObject) (rest.BeginPasskeyLoginResponseObject, error) {
	name := ""
	if req.Body != nil && req.Body.LoginName != nil {
		name = *req.Body.LoginName
	}
	c, err := h.passkeys.BeginPasskeyLogin(ctx, name)
	if err != nil {
		return nil, err
	}
	body, err := webAuthnChallenge(c)
	if err != nil {
		return nil, err
	}
	return rest.BeginPasskeyLogin200JSONResponse{Body: body}, nil
}

// CompletePasskeyLogin implements rest.StrictServerInterface (POST /auth/passkey/complete): the
// assertion with the device registration and the DPoP proof of the request. A proof with the
// previous nonce gets the current one in DPoP-Nonce.
func (h *Passkeys) CompletePasskeyLogin(ctx context.Context, req rest.CompletePasskeyLoginRequestObject) (rest.CompletePasskeyLoginResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	cred, err := jsonObject(req.Body.Credential)
	if err != nil {
		return nil, err
	}
	console := consoleForm(req.Body.Client)
	reg, err := loginDevice(console, req.Body.Device)
	if err != nil {
		return nil, err
	}
	in := identity.PasskeyLogin{ChallengeID: ids.ID(req.Body.ChallengeId), Credential: cred, Device: reg,
		Proof: loginProof(ctx, dpopHeader(req.Params.DPoP)), Console: console}
	if ip, ok := reqmeta.ClientIP(ctx); ok {
		in.ClientIP = ip
	}
	out, err := h.passkeys.CompletePasskeyLogin(ctx, in)
	if err != nil {
		return nil, err
	}
	if c := out.Console; c != nil {
		return rest.CompletePasskeyLogin200JSONResponse{ConsoleLoginJSONResponse: consoleLogin(c)}, nil
	}
	if out.Nonce != "" {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, out.Nonce)
	}
	return rest.CompletePasskeyLogin201JSONResponse{Body: tokenResponse(out.Tokens)}, nil
}

// BeginEnrollment implements rest.StrictServerInterface (POST /auth/enroll/begin).
func (h *Passkeys) BeginEnrollment(ctx context.Context, req rest.BeginEnrollmentRequestObject) (rest.BeginEnrollmentResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	c, err := h.passkeys.BeginEnrollment(ctx, req.Body.EnrollmentCode)
	if err != nil {
		return nil, err
	}
	body, err := webAuthnChallenge(c)
	if err != nil {
		return nil, err
	}
	return rest.BeginEnrollment200JSONResponse{Body: body}, nil
}

// CompleteEnrollment implements rest.StrictServerInterface (POST /auth/enroll/complete): the
// creation response with the passkey name, the device registration and the DPoP proof.
func (h *Passkeys) CompleteEnrollment(ctx context.Context, req rest.CompleteEnrollmentRequestObject) (rest.CompleteEnrollmentResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	cred, err := jsonObject(req.Body.Credential)
	if err != nil {
		return nil, err
	}
	reg, err := registration(req.Body.Device)
	if err != nil {
		return nil, err
	}
	in := identity.PasskeyEnrollment{
		ChallengeID: ids.ID(req.Body.ChallengeId), Credential: cred, PasskeyName: req.Body.PasskeyName, Device: reg,
		Proof: loginProof(ctx, req.Params.DPoP),
	}
	if ip, ok := reqmeta.ClientIP(ctx); ok {
		in.ClientIP = ip
	}
	out, err := h.passkeys.CompleteEnrollment(ctx, in)
	if err != nil {
		return nil, err
	}
	if out.Nonce != "" {
		reqmeta.SetResponseHeader(ctx, headerDPoPNonce, out.Nonce)
	}
	return rest.CompleteEnrollment201JSONResponse{Body: tokenResponse(out.Tokens)}, nil
}

// BeginPasskeyRegistration implements rest.StrictServerInterface (POST /me/passkeys/begin).
func (h *Passkeys) BeginPasskeyRegistration(ctx context.Context, _ rest.BeginPasskeyRegistrationRequestObject) (rest.BeginPasskeyRegistrationResponseObject, error) {
	c, err := h.passkeys.BeginPasskeyRegistration(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	body, err := webAuthnChallenge(c)
	if err != nil {
		return nil, err
	}
	return rest.BeginPasskeyRegistration200JSONResponse{Body: body}, nil
}

// CompletePasskeyRegistration implements rest.StrictServerInterface (POST /me/passkeys/complete).
func (h *Passkeys) CompletePasskeyRegistration(ctx context.Context, req rest.CompletePasskeyRegistrationRequestObject) (rest.CompletePasskeyRegistrationResponseObject, error) {
	if req.Body == nil {
		return nil, errs.New(errs.CodeValidationFailed, "")
	}
	cred, err := jsonObject(req.Body.Credential)
	if err != nil {
		return nil, err
	}
	p, err := h.passkeys.CompletePasskeyRegistration(ctx, principal(ctx), ids.ID(req.Body.ChallengeId), cred, req.Body.Name)
	if err != nil {
		return nil, err
	}
	return rest.CompletePasskeyRegistration201JSONResponse{Body: passkeyREST(p)}, nil
}

// RevokePasskey implements rest.StrictServerInterface (DELETE /users/{user_id}/passkeys/{passkey_id});
// a passkey revoked before answers 204 again.
func (h *Passkeys) RevokePasskey(ctx context.Context, req rest.RevokePasskeyRequestObject) (rest.RevokePasskeyResponseObject, error) {
	if err := h.passkeys.RevokePasskey(ctx, principal(ctx), ids.ID(req.UserId), ids.ID(req.PasskeyId)); err != nil {
		return nil, err
	}
	return rest.RevokePasskey204Response{}, nil
}

// CompleteStepUpWithPasskey implements rest.StrictServerInterface (POST /auth/step-up/passkey).
func (h *Passkeys) CompleteStepUpWithPasskey(ctx context.Context, req rest.CompleteStepUpWithPasskeyRequestObject) (rest.CompleteStepUpWithPasskeyResponseObject, error) {
	var (
		id   ids.ID
		cred json.RawMessage
	)
	if req.Body != nil {
		var err error
		if cred, err = jsonObject(req.Body.Credential); err != nil {
			return nil, err
		}
		id = ids.ID(req.Body.ChallengeId)
	}
	r, err := h.stepUp.CompleteWithPasskey(ctx, principal(ctx), id, cred)
	if err != nil {
		return nil, err
	}
	return rest.CompleteStepUpWithPasskey200JSONResponse{StepUpDoneJSONResponse: stepUpDone(r)}, nil
}

// webAuthnChallenge converts a ceremony challenge (schema WebAuthnChallenge).
func webAuthnChallenge(c identity.WebAuthnChallenge) (rest.WebAuthnChallenge, error) {
	pk, err := object(c.PublicKey)
	if err != nil {
		return rest.WebAuthnChallenge{}, err
	}
	return rest.WebAuthnChallenge{ChallengeId: rest.Uuid(c.ID), PublicKey: pk, ExpiresAt: ms(c.ExpiresAt)}, nil
}

// object decodes the JSON object raw for a JsonObject member.
func object(raw json.RawMessage) (rest.JsonObject, error) {
	var o rest.JsonObject
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "identity: WebAuthn options")
	}
	return o, nil
}

// passkeyREST converts a passkey (schema Passkey); empty values become absent members.
func passkeyREST(p identity.Passkey) rest.Passkey {
	return rest.Passkey{
		Id: rest.Uuid(p.ID), Name: p.Name, Aaguid: optionalID(p.AAGUID), BackupEligible: p.BackupEligible,
		CreatedAt: ms(p.CreatedAt), LastUsedAt: optionalTime(p.LastUsedAt),
	}
}
