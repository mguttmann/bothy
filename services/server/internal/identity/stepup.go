// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// StepUpMethodDeviceKey is auth_sessions.step_up_method of a step-up confirmed with the step_up
// key of a native device (docs/spec/08-identity.md section 6.2).
const StepUpMethodDeviceKey = "device_key"

// Challenge is a single-use step-up challenge (schema StepUpChallenge). Value is the random value
// the step_up key signs; it appears only in this response, the server keeps its HMAC.
type Challenge struct {
	// ID is auth_challenges.id, challenge_id in the API.
	ID ids.ID
	// Value is the base64url random value (256 bits).
	Value string
	// ExpiresAt is the end of the challenge, auth.step_up_challenge_ttl after its issue.
	ExpiresAt time.Time
	// PasskeyOptions are the WebAuthn request options over the same value when the person has an
	// active passkey (M1-40); nil otherwise.
	PasskeyOptions json.RawMessage
}

// StepUpResult is a confirmed step-up (schema StepUpResult).
type StepUpResult struct {
	// StepUpAt is the new auth_sessions.step_up_at; ValidUntil is StepUpAt plus
	// auth.step_up_max_age.
	StepUpAt, ValidUntil time.Time
	// Cookie is the new value of the console cookie after the step-up of a console session
	// (session fixation, 08 section 7.5, M6-01); the zero Token for every other session.
	// CookieMaxAge is its Max-Age.
	Cookie       Token
	CookieMaxAge time.Duration
}

// StepUp is the step-up port of identity (PLAN M1-35; 08 section 6.2, 07 SR5): a person in a
// session confirms administrative actions with the step_up key of the session's device. Both
// operations are the action auth.step_up (right self on the caller's session, audited); M1-40
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
	// CompleteWithPasskey checks credential, a PublicKeyCredential with an assertion response in
	// JSON form, against the challenge challengeID and an active passkey of the session's person
	// and sets the step-up of the session with method passkey (M1-40). Errors: as
	// CompleteWithDeviceKey, except that a device without a step_up key may use it; a passkey of
	// another person is a wrong confirmation and counts.
	CompleteWithPasskey(ctx context.Context, a actor.Actor, challengeID ids.ID, credential json.RawMessage) (StepUpResult, error)
	// ConsumeFresh consumes the fresh step-up of the session of a for one terminal opening (08
	// section 6.3 point 4, M2-40). It runs only inside the caller's unit of work (none is kind
	// internal) and is the first domain row lock of that transaction: it locks the session row,
	// answers 401 session_revoked for a revoked or ended session, checks the freshness again on the
	// locked row with the rule of authorize (terminal.step_up_max_age, passkey or device key,
	// unconsumed, not the console) and then sets step_up_consumed_at, so one confirmation opens
	// exactly one session. A stale step-up is 401 step_up_required without a reason in the problem
	// body; the audit reason of the caller is ReasonStepUpNotFresh. It writes no event and no
	// audit entry: the use case of the action records the opening and the denial.
	ConsumeFresh(ctx context.Context, a actor.Actor) error
}

// ReasonStepUpNotFresh is the audit reason of a terminal opening whose step-up is not fresh or
// already consumed (07 section 4.1 terminal_step_up_not_fresh), for the callers of ConsumeFresh.
const ReasonStepUpNotFresh = "terminal_step_up_not_fresh"
