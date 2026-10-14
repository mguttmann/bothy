// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ChallengePurpose is auth_challenges.purpose.
type ChallengePurpose string

// The purposes of a challenge (03 auth_challenges): step_up (M1-35) and the passkey ceremonies
// (M1-40).
const (
	// PurposeStepUp is a step-up challenge of a session.
	PurposeStepUp ChallengePurpose = "step_up"
	// PurposePasskeyLogin is a passkey login; it is bound to nothing, the credential names the
	// person.
	PurposePasskeyLogin ChallengePurpose = "passkey_login"
	// PurposePasskeyEnroll is the first passkey with an enrollment code; it carries the code.
	PurposePasskeyEnroll ChallengePurpose = "passkey_enroll"
	// PurposePasskeyRegister is a further passkey of a person in a session; it carries the session.
	PurposePasskeyRegister ChallengePurpose = "passkey_register"
)

// ChallengeStatus is auth_challenges.status.
type ChallengeStatus string

// The states of a challenge.
const (
	// ChallengeIssued is open: it may be completed once before it expires.
	ChallengeIssued ChallengeStatus = "issued"
	// ChallengeUsed was completed.
	ChallengeUsed ChallengeStatus = "used"
	// ChallengeFailed received a wrong confirmation; it counts toward auth.step_up_fail_limit.
	ChallengeFailed ChallengeStatus = "failed"
)

// ChallengeRecord is a row of auth_challenges. Zero values stand for NULL.
type ChallengeRecord struct {
	ID, SessionID ids.ID
	// EnrollmentCodeID is the checked code of a passkey_enroll challenge (M1-40).
	EnrollmentCodeID ids.ID
	Purpose          ChallengePurpose
	// WebAuthnSession is webauthn_session (schema webauthn_session.v1, domain.WebAuthnSession) of a
	// WebAuthn ceremony; nil for a step-up without passkey options.
	WebAuthnSession []byte
	// Hash is challenge_hash, the HMAC of the value with the token pepper.
	Hash                 []byte
	Status               ChallengeStatus
	CreatedAt, ExpiresAt time.Time
	FinishedAt           time.Time
}

// ChallengeStore is the outgoing port of the step-up use cases over auth_challenges and the
// step-up columns of auth_sessions (implemented by adapters/postgres). Writes and locks run only
// inside a unit of work. Errors are kernel/errs errors without SQL, values or hashes.
type ChallengeStore interface {
	// InsertChallenge inserts c with status issued.
	InsertChallenge(ctx context.Context, c ChallengeRecord) error
	// LockChallenge locks the challenge id and returns it; found is false for an unknown id.
	LockChallenge(ctx context.Context, id ids.ID) (c ChallengeRecord, found bool, err error)
	// FinishChallenge sets status (used or failed) and finished_at = at on the challenge id while
	// it is issued.
	FinishChallenge(ctx context.Context, id ids.ID, status ChallengeStatus, at time.Time) error
	// FailedStepUpTimes returns finished_at of the failed step-up challenges of sessionID after
	// since, ascending.
	FailedStepUpTimes(ctx context.Context, sessionID ids.ID, since time.Time) ([]time.Time, error)
	// SetSessionStepUp sets step_up_at = at and step_up_method = method on the session id.
	SetSessionStepUp(ctx context.Context, id ids.ID, at time.Time, method string) error
	// LockSessionForStepUpConsumption locks the session id of userID FOR UPDATE and returns it;
	// found is false for an unknown id or another person (M2-40).
	LockSessionForStepUpConsumption(ctx context.Context, id, userID ids.ID) (s SessionRecord, found bool, err error)
	// ConsumeStepUp sets step_up_consumed_at = at on the session id (M2-40).
	ConsumeStepUp(ctx context.Context, id ids.ID, at time.Time) error
	// DeleteExpiredChallenges deletes the challenges with expires_at <= now, except failed step-up
	// challenges with finished_at after keepFailedSince, and returns how many it deleted.
	DeleteExpiredChallenges(ctx context.Context, now, keepFailedSince time.Time) (int64, error)
}
