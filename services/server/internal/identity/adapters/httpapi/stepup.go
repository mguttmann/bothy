// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StepUps implements the step-up operations of rest.StrictServerInterface (M1-35):
// createStepUpChallenge and completeStepUpWithDeviceKey over identity.StepUp. The use cases check
// the caller and authorize; the handlers convert. Neither operation takes an Idempotency-Key (06
// section 7 item 3, x-idempotency-exempt auth_flow). Errors go to the problem writer of the chain
// unchanged.
type StepUps struct {
	stepUp identity.StepUp
}

// NewStepUps returns the handlers over the step-up port.
func NewStepUps(s identity.StepUp) *StepUps { return &StepUps{stepUp: s} }

// CreateStepUpChallenge implements rest.StrictServerInterface (POST /auth/step-up/challenge). The
// challenge value leaves the server only here; passkey_options stays absent until M1-40.
func (h *StepUps) CreateStepUpChallenge(ctx context.Context, _ rest.CreateStepUpChallengeRequestObject) (rest.CreateStepUpChallengeResponseObject, error) {
	c, err := h.stepUp.CreateChallenge(ctx, principal(ctx))
	if err != nil {
		return nil, err
	}
	return rest.CreateStepUpChallenge200JSONResponse{Body: rest.StepUpChallenge{
		ChallengeId: rest.Uuid(c.ID), Challenge: c.Value, ExpiresAt: ms(c.ExpiresAt),
	}}, nil
}

// CompleteStepUpWithDeviceKey implements rest.StrictServerInterface (POST
// /auth/step-up/device-key).
func (h *StepUps) CompleteStepUpWithDeviceKey(ctx context.Context, req rest.CompleteStepUpWithDeviceKeyRequestObject) (rest.CompleteStepUpWithDeviceKeyResponseObject, error) {
	var id ids.ID
	var sig string
	if req.Body != nil {
		id, sig = ids.ID(req.Body.ChallengeId), req.Body.Signature
	}
	r, err := h.stepUp.CompleteWithDeviceKey(ctx, principal(ctx), id, sig)
	if err != nil {
		return nil, err
	}
	return rest.CompleteStepUpWithDeviceKey200JSONResponse{Body: rest.StepUpResult{
		StepUpAt: ms(r.StepUpAt), ValidUntil: ms(r.ValidUntil),
	}}, nil
}
