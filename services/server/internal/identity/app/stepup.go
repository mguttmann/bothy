// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/stepup"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/jose"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// StepUpDeps are the collaborators of the step-up use cases (M1-35).
type StepUpDeps struct {
	// Challenges stores the challenges and the step-up of a session.
	Challenges ChallengeStore
	// Sessions reads and locks the session of the request.
	Sessions SessionStore
	// Devices reads the device of the session and its active step_up key.
	Devices DeviceStore
	// Auth revokes a session after too many failed confirmations (RevokeSession, M1-34).
	Auth identity.Auth
	// UoW runs every write; its registry holds the sync and audit hooks.
	UoW txn.UnitOfWork
	// Authorizer decides auth.step_up.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries and the denials of this module.
	Audit auditing.Recorder
	// Clock is the time of challenges and confirmations; nil means clock.System().
	Clock clock.Clock
}

// StepUps implements identity.StepUp with the device key (08 section 6.2, brief M1-35 "Design
// notes"). It is safe for concurrent use.
type StepUps struct {
	cfg    Config
	pepper []byte
	d      StepUpDeps
}

// Compile-time check: StepUps is the step-up port of identity.
var _ identity.StepUp = (*StepUps)(nil)

// NewStepUps returns the step-up use cases with auth.step_up_challenge_ttl, auth.step_up_max_age
// and auth.login_fail_window above 0, auth.step_up_fail_limit at least 1, auth.clock_leeway not
// negative and the raw token pepper (the HMAC key of challenges). Errors: a missing collaborator,
// an empty pepper or a value outside its range.
func NewStepUps(cfg Config, pepper []byte, d StepUpDeps) (*StepUps, error) {
	if d.Challenges == nil || d.Sessions == nil || d.Devices == nil || d.Auth == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil {
		return nil, errors.New("identity: NewStepUps needs every collaborator")
	}
	if len(pepper) == 0 || cfg.StepUpChallengeTTL <= 0 || cfg.StepUpMaxAge <= 0 || cfg.LoginFailWindow <= 0 ||
		cfg.StepUpFailLimit < 1 || cfg.ClockLeeway < 0 {
		return nil, errors.New("identity: step-up needs the token pepper, auth.step_up_challenge_ttl, auth.step_up_max_age and auth.login_fail_window above 0, auth.step_up_fail_limit at least 1 and auth.clock_leeway not negative")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &StepUps{cfg: cfg, pepper: append([]byte(nil), pepper...), d: d}, nil
}

// Audit values of auth.step_up (05 section 6, SEC-AUD-5): codes and ids only, never the value or
// the signature.
const (
	attrStep           = "step"
	attrMethod         = "method"
	attrChallengeID    = "challenge_id"
	attrSessionRevoked = "session_revoked"
	stepChallenge      = "challenge"
	stepConfirm        = "confirm"
)

// The causes of the denials that are no failed confirmation (audit details).
const (
	causeNoSession         = "no_session"
	causeNotUser           = "not_a_person"
	causeCLIDevice         = "cli_device"
	causeNoStepUpKey       = "no_step_up_key"
	causeUnknownChallenge  = "unknown_challenge"
	causeChallengeFinished = "challenge_finished"
	causeChallengeExpired  = "challenge_expired"
	causeSessionEnded      = "session_ended"
)

// reasonStepUpNotPossible is the reason of a step-up this caller or device cannot give (07
// section 4.1).
const reasonStepUpNotPossible = "step_up_not_possible"

// stepUpDeniedError is a refused step-up: the answer and the audit entry written after the rollback.
// It never carries the value or the signature.
type stepUpDeniedError struct {
	answer error
	entry  auditing.Entry
}

func (d *stepUpDeniedError) Error() string { return "identity: step-up denied" }

// denyStepUp builds a denial of auth.step_up on the session sid with the answer, reason and cause.
func denyStepUp(answer error, reason auditing.Reason, cause string, sid ids.ID) *stepUpDeniedError {
	return &stepUpDeniedError{answer: answer, entry: auditing.Entry{
		Action: string(authorize.ActionAuthStepUp), Decision: auditing.DecisionDeny, Reason: reason,
		ResourceKind: resourceAuthSession, ResourceID: sid, Attrs: auditing.Attrs{attrCause: auditing.Code(cause)},
	}}
}

// notPossible is the 403 forbidden of a caller or device that cannot give a step-up.
func notPossible(cause string, sid ids.ID) *stepUpDeniedError {
	answer := errs.New(errs.CodeForbidden, "").WithReason(reasonStepUpNotPossible)
	return denyStepUp(answer, auditing.ReasonStepUpNotPossible, cause, sid)
}

// required is the 401 step_up_required of a challenge that cannot be completed (not counted).
func required(cause string, sid ids.ID) *stepUpDeniedError {
	return denyStepUp(errs.New(errs.CodeStepUpRequired, ""), auditing.ReasonStepUpRequired, cause, sid)
}

// finish writes the audit entry of a denial and returns its answer; any other error passes.
func (s *StepUps) finish(ctx context.Context, err error) error {
	var denial *stepUpDeniedError
	if errors.As(err, &denial) {
		_ = s.d.Audit.RecordDenial(ctx, denial.entry)
		return denial.answer
	}
	return err
}

// admit runs the actor checks and authorize of both operations (brief M1-35 "Design notes" items
// 1 and 2): a person with a session on a device that is not cli, then auth.step_up with right self
// on the caller's session. The anonymous principal goes straight to authorize, which answers 401.
func (s *StepUps) admit(ctx context.Context, a actor.Actor) error {
	if !a.IsAnonymous() {
		switch {
		case a.Kind != actor.KindUser:
			return notPossible(causeNotUser, ids.ID{})
		case a.SessionID.IsZero():
			return notPossible(causeNoSession, ids.ID{})
		}
		dev, found, err := s.d.Devices.Device(ctx, a.DeviceID)
		if err != nil {
			return err
		}
		// A cli device has no step_up key and no user verification (08 section 6.4).
		if found && dev.Platform == identity.PlatformCLI {
			return notPossible(causeCLIDevice, a.SessionID)
		}
	}
	r := authorize.Resource{Kind: authorize.ResourceAuthSession, ID: a.SessionID}
	if !a.SessionID.IsZero() {
		sess, found, err := s.d.Sessions.Session(ctx, a.SessionID)
		if err != nil {
			return err
		}
		if found {
			r.OwnerID = sess.UserID
		}
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionAuthStepUp, r)
	if err != nil {
		return err
	}
	return d.Err()
}

// now is the time of the clock at the precision of PostgreSQL, so the answer equals the stored
// step_up_at.
func (s *StepUps) now() time.Time { return s.d.Clock.Now().Truncate(time.Microsecond) }

// CreateChallenge implements identity.StepUp (brief M1-35 "Create challenge").
func (s *StepUps) CreateChallenge(ctx context.Context, a actor.Actor) (identity.Challenge, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.Challenge{}, err
	}
	if err := s.admit(ctx, a); err != nil {
		return identity.Challenge{}, s.finish(ctx, err)
	}
	value, err := stepup.NewChallengeValue()
	if err != nil {
		return identity.Challenge{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	now := s.now()
	c := ChallengeRecord{
		ID: ids.New(), SessionID: a.SessionID, Purpose: PurposeStepUp, Hash: stepup.ChallengeHash(s.pepper, value),
		Status: ChallengeIssued, CreatedAt: now, ExpiresAt: now.Add(s.cfg.StepUpChallengeTTL),
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.d.Challenges.InsertChallenge(ctx, c); err != nil {
			return err
		}
		return s.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionAuthStepUp), Decision: auditing.DecisionAllow, ResourceKind: resourceAuthSession,
			ResourceID: a.SessionID, Attrs: auditing.Attrs{attrStep: auditing.Code(stepChallenge), attrChallengeID: auditing.ID(c.ID)},
		})
	})
	if err != nil {
		return identity.Challenge{}, err
	}
	return identity.Challenge{ID: c.ID, Value: value, ExpiresAt: c.ExpiresAt}, nil
}

// completion is the outcome of the unit of work of a completion: the result of a success, or a
// counted failure that committed (revoked reports the revocation of the session).
type completion struct {
	result  identity.StepUpResult
	failed  bool
	revoked bool
}

// CompleteWithDeviceKey implements identity.StepUp (brief M1-35 "Complete with the device key"):
// actor checks and authorize, then one unit of work that locks the session rows and the challenge
// (03 section 6.3), so concurrent completions of one challenge serialize. A wrong confirmation
// commits the failed challenge, the revocation when it reached the limit and its audit entry; every
// other denial rolls back and is audited afterwards.
func (s *StepUps) CompleteWithDeviceKey(ctx context.Context, a actor.Actor, challengeID ids.ID, signature string) (identity.StepUpResult, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.StepUpResult{}, err
	}
	if err := s.admit(ctx, a); err != nil {
		return identity.StepUpResult{}, s.finish(ctx, err)
	}
	var out completion
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.complete(ctx, a, challengeID, signature)
		return err
	})
	if err != nil {
		return identity.StepUpResult{}, s.finish(ctx, err)
	}
	switch {
	case out.revoked:
		return identity.StepUpResult{}, errs.New(errs.CodeSessionRevoked, "")
	case out.failed:
		return identity.StepUpResult{}, errs.New(errs.CodeStepUpRequired, "")
	}
	return out.result, nil
}

// complete is the body of CompleteWithDeviceKey inside its unit of work.
func (s *StepUps) complete(ctx context.Context, a actor.Actor, challengeID ids.ID, signature string) (completion, error) {
	sid := a.SessionID
	now := s.now()
	tree, err := s.d.Sessions.LockSessionTree(ctx, sid)
	if err != nil {
		return completion{}, err
	}
	// authorize saw the session active; a revocation that committed since then ends here.
	if sess, ok := find(tree, sid); !ok || !sess.Active(now) {
		return completion{}, denyStepUp(errs.New(errs.CodeSessionRevoked, ""), auditing.ReasonSessionRevoked, causeSessionEnded, sid)
	}
	key, found, err := s.d.Devices.ActiveKey(ctx, a.DeviceID, identity.KeyPurposeStepUp)
	if err != nil {
		return completion{}, err
	}
	if !found {
		return completion{}, notPossible(causeNoStepUpKey, sid)
	}
	jwk, err := jose.ParseJWK(key.PublicJWK)
	if err != nil {
		return completion{}, errs.Wrap(errs.CodeInternalError, err, "identity: stored step_up key")
	}
	ch, found, err := s.d.Challenges.LockChallenge(ctx, challengeID)
	if err != nil {
		return completion{}, err
	}
	// Without the challenge the value cannot be signed, so these do not count (08 section 6.2).
	switch {
	case !found || ch.Purpose != PurposeStepUp || ch.SessionID != sid:
		return completion{}, required(causeUnknownChallenge, sid)
	case ch.Status != ChallengeIssued:
		return completion{}, required(causeChallengeFinished, sid)
	case !now.Before(ch.ExpiresAt):
		return completion{}, required(causeChallengeExpired, sid)
	}
	cause := stepup.Check(signature, stepup.Expect{
		Key: jwk, Alg: key.Alg, ChallengeHash: ch.Hash, Pepper: s.pepper, SessionID: sid, IssuedAt: ch.CreatedAt, Now: now,
		Leeway: s.cfg.ClockLeeway,
	})
	if cause != "" {
		return s.fail(ctx, a, ch.ID, cause, now)
	}
	if err := s.d.Challenges.FinishChallenge(ctx, ch.ID, ChallengeUsed, now); err != nil {
		return completion{}, err
	}
	if err := s.d.Challenges.SetSessionStepUp(ctx, sid, now, identity.StepUpMethodDeviceKey); err != nil {
		return completion{}, err
	}
	err = s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAuthStepUp), Decision: auditing.DecisionAllow, ResourceKind: resourceAuthSession, ResourceID: sid,
		Attrs: auditing.Attrs{
			attrStep: auditing.Code(stepConfirm), attrMethod: auditing.Code(identity.StepUpMethodDeviceKey), attrChallengeID: auditing.ID(ch.ID),
		},
	})
	if err != nil {
		return completion{}, err
	}
	return completion{result: identity.StepUpResult{StepUpAt: now, ValidUntil: now.Add(s.cfg.StepUpMaxAge)}}, nil
}

// fail records a wrong confirmation in the unit of work: the challenge becomes failed, the failed
// challenges of the session inside auth.login_fail_window are counted, the session is revoked with
// step_up_failed when they reach auth.step_up_fail_limit, and the deny entry is written.
func (s *StepUps) fail(ctx context.Context, a actor.Actor, challengeID ids.ID, cause stepup.Cause, now time.Time) (completion, error) {
	sid := a.SessionID
	if err := s.d.Challenges.FinishChallenge(ctx, challengeID, ChallengeFailed, now); err != nil {
		return completion{}, err
	}
	times, err := s.d.Challenges.FailedStepUpTimes(ctx, sid, now.Add(-s.cfg.LoginFailWindow))
	if err != nil {
		return completion{}, err
	}
	revoke := stepup.LimitReached(times, now, s.cfg.LoginFailWindow, s.cfg.StepUpFailLimit)
	if revoke {
		by := a.ID
		if err := s.d.Auth.RevokeSession(ctx, sid, identity.RevokeStepUpFailed, &by); err != nil {
			return completion{}, err
		}
	}
	err = s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAuthStepUp), Decision: auditing.DecisionDeny, Reason: auditing.ReasonStepUpRequired,
		ResourceKind: resourceAuthSession, ResourceID: sid,
		Attrs: auditing.Attrs{
			attrCause: auditing.Code(string(cause)), attrChallengeID: auditing.ID(challengeID), attrSessionRevoked: auditing.Bool(revoke),
		},
	})
	if err != nil {
		return completion{}, err
	}
	return completion{failed: true, revoked: revoke}, nil
}
