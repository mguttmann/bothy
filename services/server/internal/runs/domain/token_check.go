// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// TokenVerdict is the verdict on an invocation token that runs hands to authorize as a fact
// (brief M2-09 note 2). It mirrors authorize.TokenVerdict value for value; runs/app converts it,
// because this package must not import kernel/authorize.
type TokenVerdict uint8

// The verdicts of 09 4.5. The zero value is no verdict, so a forgotten field never reads as valid.
const (
	// VerdictZero: no verdict; never handed to authorize.
	VerdictZero TokenVerdict = iota
	// VerdictValid: the token passed every check of the operation.
	VerdictValid
	// VerdictExpired: the token is current but exp plus auth.clock_leeway has passed.
	VerdictExpired
	// VerdictSuperseded: a newer delivery replaced the token, or a start consumed it.
	VerdictSuperseded
	// VerdictInvalid: parse, signature or claim checks failed (09 4.5 table).
	VerdictInvalid
)

// TokenClaims are the claims of an invocation token that the currency rules read. The caller
// fills them only after the token passed parsing and the signature check (brief M2-09 note 3a);
// a token that failed there is VerdictInvalid and never reaches StartVerdict or RenewVerdict.
type TokenClaims struct {
	// JTI is the claim jti.
	JTI ids.ID
	// ExpiresAt is the claim exp.
	ExpiresAt time.Time
}

// RunTokenState is the part of a run row the currency rules read, loaded FOR UPDATE. A zero ID
// stands for NULL.
type RunTokenState struct {
	// State is runs.state.
	State State
	// InvocationJTI is runs.invocation_jti, the jti of the current delivery or renewal.
	InvocationJTI ids.ID
	// StartedJTI is runs.started_jti, the jti that started or resumed the run last.
	StartedJTI ids.ID
	// RenewedFromJTI is runs.renewed_from_jti, the jti a renewal replaced (09 4.5.1 step 5).
	RenewedFromJTI ids.ID
}

// StartOp names the endpoint of a start: each endpoint performs only its own transition.
type StartOp uint8

// The two start endpoints (06 16).
const (
	// StartOpStart is startRun: queued -> running.
	StartOpStart StartOp = iota + 1
	// StartOpResume is resumeRun: waiting_input -> running.
	StartOpResume
)

// StartNext names what follows an allow of authorize for a start or a renewal.
type StartNext uint8

// The follow-ups of brief M2-09 notes 3b and 4b. The zero value is invalid.
const (
	// NextDeny: the verdict is not valid, so authorize denies; answer by the verdict.
	NextDeny StartNext = iota + 1
	// NextReplay: answer the current run with Idempotent-Replay: true and change nothing.
	NextReplay
	// NextAlreadyStarted: answer 409 already_started.
	NextAlreadyStarted
	// NextInvalidTransition: answer 409 invalid_state_transition (wrong endpoint for the state).
	NextInvalidTransition
	// NextTransition: perform the transition (start, resume) or issue the new token (renewal).
	NextTransition
)

// StartCheck is the result of StartVerdict.
type StartCheck struct {
	// Verdict goes to authorize as the token fact.
	Verdict TokenVerdict
	// Next is what follows an allow.
	Next StartNext
	// ModeFacts reports whether authorize gets the billing mode and tool class facts (note 3c):
	// only for a transition, so a replay or a refused endpoint never fails on a mode mismatch.
	ModeFacts bool
}

// StartVerdict decides a start (StartOpStart) or a resume (StartOpResume) on a token that passed
// parsing and the signature check, against the run row loaded FOR UPDATE (brief M2-09 note 3b).
// The first matching rule wins:
//
//  1. jti equals a set started_jti and the run is running: valid, replay. The expiry is not
//     checked, because the start already happened (06 "Idempotency" point 5, R-SY-19).
//  2. jti equals started_jti and the run is waiting_input: superseded, the start consumed it.
//  3. the run is neither queued nor waiting_input: valid, then 409 already_started.
//  4. jti differs from invocation_jti: superseded by a newer delivery.
//  5. now is after exp plus leeway (auth.clock_leeway; exactly at that instant is still valid):
//     expired.
//  6. startRun on waiting_input or resumeRun on queued: valid, then 409 invalid_state_transition.
//  7. otherwise valid with the mode facts, then the transition.
//
// Example:
//
//	c := domain.StartVerdict(claims, row, domain.StartOpStart, now, leeway)
//	// c.Verdict goes to authorize; after an allow, c.Next says what to do.
func StartVerdict(c TokenClaims, r RunTokenState, op StartOp, now time.Time, leeway time.Duration) StartCheck {
	if !r.StartedJTI.IsZero() && c.JTI == r.StartedJTI {
		//exhaustive:ignore // A replay is decided only for running and waiting_input; the checks below cover the rest.
		switch r.State {
		case StateRunning:
			return StartCheck{Verdict: VerdictValid, Next: NextReplay}
		case StateWaitingInput:
			return StartCheck{Verdict: VerdictSuperseded, Next: NextDeny}
		}
	}
	if r.State != StateQueued && r.State != StateWaitingInput {
		return StartCheck{Verdict: VerdictValid, Next: NextAlreadyStarted}
	}
	if c.JTI != r.InvocationJTI {
		return StartCheck{Verdict: VerdictSuperseded, Next: NextDeny}
	}
	if now.After(c.ExpiresAt.Add(leeway)) {
		return StartCheck{Verdict: VerdictExpired, Next: NextDeny}
	}
	if (op == StartOpStart && r.State == StateWaitingInput) || (op == StartOpResume && r.State == StateQueued) {
		return StartCheck{Verdict: VerdictValid, Next: NextInvalidTransition}
	}
	return StartCheck{Verdict: VerdictValid, Next: NextTransition, ModeFacts: true}
}

// RenewCheck is the result of RenewVerdict.
type RenewCheck struct {
	// Verdict goes to authorize as the token fact; never VerdictExpired.
	Verdict TokenVerdict
	// Next is NextDeny, NextAlreadyStarted or NextTransition.
	Next StartNext
	// WasCurrent reports that the presented jti is invocation_jti: only then does the renewal
	// set renewed_from_jti to it, so a retry with the renewed-from token keeps the stored value
	// (09 4.5.1 step 5, AG-32).
	WasCurrent bool
}

// RenewVerdict decides steps 2 and 3 of 09 4.5.1 for renewInvocationToken on a token that passed
// step 1 (parsing and signature, without the expiry), against the run row loaded FOR UPDATE
// (brief M2-09 note 4b):
//
//   - step 2: the run is neither queued nor waiting_input: valid, then 409 already_started. It
//     comes before step 3, so a running run answers already_started even for a replaced jti
//     (AG-32).
//   - step 3: jti equals invocation_jti, or a set renewed_from_jti, and started_jti is distinct
//     from jti (a NULL started_jti is distinct): valid, then the new token. Else superseded.
//
// The expiry is never checked: renewing an expired token is the purpose of the operation.
//
// Example:
//
//	c := domain.RenewVerdict(claims, row)
//	if c.Next == domain.NextTransition && c.WasCurrent { /* renewed_from_jti = claims.JTI */ }
func RenewVerdict(c TokenClaims, r RunTokenState) RenewCheck {
	if r.State != StateQueued && r.State != StateWaitingInput {
		return RenewCheck{Verdict: VerdictValid, Next: NextAlreadyStarted}
	}
	known := c.JTI == r.InvocationJTI || (!r.RenewedFromJTI.IsZero() && c.JTI == r.RenewedFromJTI)
	if known && r.StartedJTI != c.JTI {
		return RenewCheck{Verdict: VerdictValid, Next: NextTransition, WasCurrent: c.JTI == r.InvocationJTI}
	}
	return RenewCheck{Verdict: VerdictSuperseded, Next: NextDeny}
}
