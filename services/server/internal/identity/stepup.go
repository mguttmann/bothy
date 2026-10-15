// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StepUpMethodDeviceKey is auth_sessions.step_up_method of a step-up confirmed with the step_up
// key of a native device (docs/spec/08-identity.md section 6.2).
const StepUpMethodDeviceKey = "device_key"

// Challenge is a single-use step-up challenge (schema StepUpChallenge without passkey_options,
// which M1-40 adds). Value is the random value the step_up key signs; it appears only in this
// response, the server keeps its HMAC.
type Challenge struct {
	// ID is auth_challenges.id, challenge_id in the API.
	ID ids.ID
	// Value is the base64url random value (256 bits).
	Value string
	// ExpiresAt is the end of the challenge, auth.step_up_challenge_ttl after its issue.
	ExpiresAt time.Time
}

// StepUpResult is a confirmed step-up (schema StepUpResult).
type StepUpResult struct {
	// StepUpAt is the new auth_sessions.step_up_at; ValidUntil is StepUpAt plus
	// auth.step_up_max_age.
	StepUpAt, ValidUntil time.Time
}

// StepUp is the step-up port of identity (PLAN M1-35; 08 section 6.2, 07 SR5): a person in a
// session confirms administrative actions with the step_up key of the session's device. Both
// operations are the action auth.step_up (right self on the caller's session, audited). M1-40
// adds CompleteWithPasskey. Every error is a kernel/errs error without the challenge value or the
// signature.
type StepUp interface {
	// CreateChallenge issues a challenge for the session of a. Errors: 403 forbidden with reason
	// step_up_not_possible for a caller without a session or on a cli device; the decision of
	// authorize otherwise (403 device_pending, 401 session_revoked).
	CreateChallenge(ctx context.Context, a actor.Actor) (Challenge, error)
	// CompleteWithDeviceKey checks signature, a compact JWS by the active step_up key of the
	// session's device over {challenge, sid, iat}, against the challenge challengeID and sets the
	// step-up of the session. Errors: as CreateChallenge, plus 403 forbidden
	// (step_up_not_possible) for a device without a step_up key, 401 step_up_required for an
	// unknown, foreign, finished or expired challenge and for a wrong confirmation, and 401
	// session_revoked when that failure reached auth.step_up_fail_limit and revoked the session.
	CompleteWithDeviceKey(ctx context.Context, a actor.Actor, challengeID ids.ID, signature string) (StepUpResult, error)
}
