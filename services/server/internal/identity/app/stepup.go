// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
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
	// Passkeys, People, Verifier and RelyingParty serve the step-up with a passkey (M1-40):
	// CreateChallenge adds passkey_options for a person with an active passkey, and
	// CompleteWithPasskey checks the assertion. Without Passkeys, or with a disabled relying
	// party, there are no passkey options and CompleteWithPasskey answers feature_disabled.
	Passkeys     PasskeyStore
	People       Store
	Verifier     PasskeyVerifier
	RelyingParty domain.RelyingParty
	// Fresh judges the fresh step-up of a terminal opening with the rule of authorize
	// (terminal.step_up_max_age, M2-40); nil fails ConsumeFresh closed.
	Fresh authz.FreshStepUpCheck
	// Console rotates the cookie of a console session after its step-up (M6-01); a world without
	// console sessions leaves it nil.
	Console *Console
}

// StepUps implements identity.StepUp with the device key (08 section 6.2, brief M1-35 "Design
// notes"). It is safe for concurrent use.
type StepUps struct {
	cfg    Config
	pepper []byte
	d      StepUpDeps
	c      *ceremonies
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
	s := &StepUps{cfg: cfg, pepper: append([]byte(nil), pepper...), d: d}
	if d.Passkeys != nil {
		if d.People == nil || (d.Verifier == nil && d.RelyingParty.Enabled) {
			return nil, errors.New("identity: the step-up with a passkey needs the people store and the verifier")
		}
		s.c = &ceremonies{passkeys: d.Passkeys, people: d.People, verifier: d.Verifier, rp: d.RelyingParty, pepper: s.pepper, audit: d.Audit}
	}
	return s, nil
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
	var options []byte
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		if options, c.WebAuthnSession, err = s.passkeyOptions(ctx, a.ID, value); err != nil {
			return err
		}
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
	return identity.Challenge{ID: c.ID, Value: value, ExpiresAt: c.ExpiresAt, PasskeyOptions: options}, nil
}

// passkeyOptions returns the WebAuthn request options over value and the webauthn_session.v1 of
// the step-up challenge when the person userID has an active passkey and the relying party is
// enabled (08 section 6.2 step 1, M1-40); nil and nil otherwise.
func (s *StepUps) passkeyOptions(ctx context.Context, userID ids.ID, value string) ([]byte, []byte, error) {
	if s.c == nil || s.c.enabled() != nil {
		return nil, nil, nil
	}
	active, err := s.c.passkeys.ActivePasskeysOfUser(ctx, userID)
	if err != nil || len(active) == 0 {
		return nil, nil, err
	}
	raw, err := domain.DecodeChallenge(value)
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternalError, err, "identity: challenge value")
	}
	opts, err := s.c.verifier.RequestOptions(active, raw)
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternalError, err, "identity: request options")
	}
	session, err := s.c.session(active, nil)
	return opts, session, err
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

// lockSession locks the session rows of a completion first (03 section 6.3), checks that the
// session sid is still active at now and returns it.
func (s *StepUps) lockSession(ctx context.Context, sid ids.ID, now time.Time) (SessionRecord, error) {
	tree, err := s.d.Sessions.LockSessionTree(ctx, sid)
	if err != nil {
		return SessionRecord{}, err
	}
	// authorize saw the session active; a revocation that committed since then ends here.
	sess, ok := find(tree, sid)
	if !ok || !sess.Active(now) {
		return SessionRecord{}, denyStepUp(errs.New(errs.CodeSessionRevoked, ""), auditing.ReasonSessionRevoked, causeSessionEnded, sid)
	}
	return sess, nil
}

// opened locks the challenge challengeID after the session rows and checks that it is an issued
// step-up challenge of the session sid, not expired at now. Errors: the denials that do not count
// (08 section 6.2).
func (s *StepUps) opened(ctx context.Context, sid, challengeID ids.ID, now time.Time) (ChallengeRecord, error) {
	ch, found, err := s.d.Challenges.LockChallenge(ctx, challengeID)
	if err != nil {
		return ChallengeRecord{}, err
	}
	// Without the challenge the value cannot be signed, so these do not count (08 section 6.2).
	switch {
	case !found || ch.Purpose != PurposeStepUp || ch.SessionID != sid:
		return ChallengeRecord{}, required(causeUnknownChallenge, sid)
	case ch.Status != ChallengeIssued:
		return ChallengeRecord{}, required(causeChallengeFinished, sid)
	case !now.Before(ch.ExpiresAt):
		return ChallengeRecord{}, required(causeChallengeExpired, sid)
	}
	return ch, nil
}

// complete is the body of CompleteWithDeviceKey inside its unit of work.
func (s *StepUps) complete(ctx context.Context, a actor.Actor, challengeID ids.ID, signature string) (completion, error) {
	sid := a.SessionID
	now := s.now()
	sess, err := s.lockSession(ctx, sid, now)
	if err != nil {
		return completion{}, err
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
	ch, err := s.opened(ctx, sid, challengeID, now)
	if err != nil {
		return completion{}, err
	}
	cause := stepup.Check(signature, stepup.Expect{
		Key: jwk, Alg: key.Alg, ChallengeHash: ch.Hash, Pepper: s.pepper, SessionID: sid, IssuedAt: ch.CreatedAt, Now: now,
		Leeway: s.cfg.ClockLeeway,
	})
	if cause != "" {
		return s.fail(ctx, a, ch.ID, cause, now)
	}
	return s.confirmed(ctx, sess, ch.ID, identity.StepUpMethodDeviceKey, now)
}

// confirmed ends the challenge as used, sets the step-up of the session sess with method, gives a
// console session a new cookie value in the same transaction (session fixation, 08 section 7.5,
// D-M6-01-2) and writes the allow entry, in the ambient unit of work.
func (s *StepUps) confirmed(ctx context.Context, sess SessionRecord, challengeID ids.ID, method string, now time.Time) (completion, error) {
	sid := sess.ID
	var (
		cookie identity.Token
		maxAge time.Duration
	)
	if sess.Component == identity.ComponentConsole {
		if s.d.Console == nil {
			return completion{}, errs.New(errs.CodeInternalError, "identity: a console step-up without the console")
		}
		var err error
		if cookie, maxAge, err = s.d.Console.rotate(ctx, sess); err != nil {
			return completion{}, err
		}
	}
	if err := s.d.Challenges.FinishChallenge(ctx, challengeID, ChallengeUsed, now); err != nil {
		return completion{}, err
	}
	if err := s.d.Challenges.SetSessionStepUp(ctx, sid, now, method); err != nil {
		return completion{}, err
	}
	err := s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAuthStepUp), Decision: auditing.DecisionAllow, ResourceKind: resourceAuthSession, ResourceID: sid,
		Attrs: auditing.Attrs{
			attrStep: auditing.Code(stepConfirm), attrMethod: auditing.Code(method), attrChallengeID: auditing.ID(challengeID),
		},
	})
	if err != nil {
		return completion{}, err
	}
	return completion{result: identity.StepUpResult{StepUpAt: now, ValidUntil: now.Add(s.cfg.StepUpMaxAge), Cookie: cookie, CookieMaxAge: maxAge}}, nil
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

// CompleteWithPasskey implements identity.StepUp (brief M1-40 "Step-up with passkey"): the actor
// checks and authorize of CompleteWithDeviceKey (a cli session gets 403 before any check of the
// assertion), then one unit of work that locks the session rows and the challenge, verifies the
// assertion against an active passkey of the session's person and sets the step-up with method
// passkey. A wrong confirmation (another challenge, another person's passkey, a refused
// assertion) fails the challenge and counts toward auth.step_up_fail_limit like a wrong
// signature. It works for local and OIDC people.
func (s *StepUps) CompleteWithPasskey(ctx context.Context, a actor.Actor, challengeID ids.ID, credential json.RawMessage) (identity.StepUpResult, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.StepUpResult{}, err
	}
	if err := s.admit(ctx, a); err != nil {
		return identity.StepUpResult{}, s.finish(ctx, err)
	}
	if s.c == nil {
		return identity.StepUpResult{}, (&ceremonies{}).enabled()
	}
	if err := s.c.enabled(); err != nil {
		return identity.StepUpResult{}, err
	}
	var out completion
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.completePasskey(ctx, a, challengeID, credential)
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

// completePasskey is the body of CompleteWithPasskey inside its unit of work.
func (s *StepUps) completePasskey(ctx context.Context, a actor.Actor, challengeID ids.ID, credential json.RawMessage) (completion, error) {
	sid := a.SessionID
	now := s.now()
	sess, err := s.lockSession(ctx, sid, now)
	if err != nil {
		return completion{}, err
	}
	ch, err := s.opened(ctx, sid, challengeID, now)
	if err != nil {
		return completion{}, err
	}
	res, err := s.c.assertion(ctx, ch, credential, a.ID)
	if cause, ok := refusalCause(err); ok {
		return s.fail(ctx, a, ch.ID, stepup.Cause(cause), now)
	}
	if err != nil {
		return completion{}, err
	}
	err = s.c.recordUse(ctx, res, authorize.ActionAuthStepUp, now)
	if cause, ok := refusalCause(err); ok {
		// A passkey revoked since the lookup counts like a refused assertion.
		return s.fail(ctx, a, ch.ID, stepup.Cause(cause), now)
	}
	if err != nil {
		return completion{}, err
	}
	return s.confirmed(ctx, sess, ch.ID, identity.StepUpMethodPasskey, now)
}
