// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// EnrollmentPurpose is enrollment_codes.purpose (08 section 3.2); the rules live in domain.
type EnrollmentPurpose = domain.EnrollmentPurpose

// The three purposes of an enrollment code.
const (
	// PurposeFirstLogin is the first device of a person with a local identity and no device.
	PurposeFirstLogin = domain.PurposeFirstLogin
	// PurposeRecovery is a new device of a person with a local identity and a device, or of an
	// owner through bothy setup issue-enrollment-code.
	PurposeRecovery = domain.PurposeRecovery
	// PurposeIdentityLink links an external identity of a person without a local identity.
	PurposeIdentityLink = domain.PurposeIdentityLink
)

// IssuedEnrollmentCode is a new code: Code is the display form, shown exactly once.
type IssuedEnrollmentCode struct {
	ID        ids.ID
	Code      string
	Purpose   EnrollmentPurpose
	ExpiresAt time.Time
}

// MatchedEnrollmentCode is a usable code found by its hash and locked in the unit of work.
// HostIssued reports a code without creator: issued by bothy setup on the host.
type MatchedEnrollmentCode struct {
	ID, UserID ids.ID
	Purpose    EnrollmentPurpose
	HostIssued bool
}

// EnrollmentCodeRecord is a row of enrollment_codes. Zero values stand for NULL. It never holds
// the code, only its HMAC.
type EnrollmentCodeRecord struct {
	ID, UserID     ids.ID
	Purpose        EnrollmentPurpose
	Hash           []byte
	ExpiresAt      time.Time
	Attempts       int
	CreatedBy      ids.ID
	CreatedAt      time.Time
	UsedAt         time.Time
	UsedByDeviceID ids.ID
}

// EnrollmentStore is the outgoing port over enrollment_codes (implemented by adapters/postgres).
// Writes and the lock run only inside a unit of work. Errors are kernel/errs errors without SQL,
// codes or hashes.
type EnrollmentStore interface {
	// InsertEnrollmentCode inserts rec.
	InsertEnrollmentCode(ctx context.Context, rec EnrollmentCodeRecord) error
	// ExpireOtherEnrollmentCodes sets expires_at = now on every unused code of userID other than
	// keep that has not expired at now.
	ExpireOtherEnrollmentCodes(ctx context.Context, userID, keep ids.ID, now time.Time) error
	// LockEnrollmentCodeByHash returns the code with hash, locked FOR UPDATE until the end of the
	// unit of work; found is false when no code has it.
	LockEnrollmentCodeByHash(ctx context.Context, hash []byte) (rec EnrollmentCodeRecord, found bool, err error)
	// LockEnrollmentCode returns the code id, locked FOR UPDATE until the end of the unit of work;
	// found is false for an unknown id (the passkey enrollment, M1-40).
	LockEnrollmentCode(ctx context.Context, id ids.ID) (rec EnrollmentCodeRecord, found bool, err error)
	// IncrementEnrollmentAttempts adds one failed attempt to the unused code id.
	IncrementEnrollmentAttempts(ctx context.Context, id ids.ID) error
	// MarkEnrollmentCodeUsed sets used_at = at and used_by_device_id = deviceID on the unused code
	// id; marked is false when it was used already.
	MarkEnrollmentCodeUsed(ctx context.Context, id, deviceID ids.ID, at time.Time) (marked bool, err error)
}

// EnrollmentDeps are the collaborators of EnrollmentCodes.
type EnrollmentDeps struct {
	// Codes reads and writes enrollment_codes.
	Codes EnrollmentStore
	// People locks the person a code is issued for and reads its status.
	People Store
	// UoW runs RecordFailedAttempt in an own transaction.
	UoW txn.UnitOfWork
	// Random is the source of new codes; nil means domain.EnrollmentRandom (crypto/rand).
	Random io.Reader
	// Clock is the time of issue, expiry and use; nil means clock.System().
	Clock clock.Clock
}

// EnrollmentCodes issues, matches and consumes enrollment codes (08 section 3.2, brief M1-37). The
// login flows of M1-37, M1-40 and M1-41 share it. It is safe for concurrent use.
type EnrollmentCodes struct {
	ttl         time.Duration
	maxAttempts int
	pepper      []byte
	d           EnrollmentDeps
}

// NewEnrollmentCodes returns the service with auth.enrollment_code_ttl and
// auth.enrollment_code_max_attempts of cfg and the raw token pepper, the HMAC key of code_hash.
// Errors: a missing collaborator, an empty pepper, a lifetime or limit below 1.
func NewEnrollmentCodes(cfg Config, pepper []byte, d EnrollmentDeps) (*EnrollmentCodes, error) {
	if d.Codes == nil || d.People == nil || d.UoW == nil {
		return nil, errors.New("identity: NewEnrollmentCodes needs the code store, the people store and the unit of work")
	}
	if len(pepper) == 0 || cfg.EnrollmentCodeTTL <= 0 || cfg.EnrollmentCodeMaxAttempts < 1 {
		return nil, errors.New("identity: enrollment codes need the token pepper, auth.enrollment_code_ttl above 0 and auth.enrollment_code_max_attempts at least 1")
	}
	if d.Random == nil {
		d.Random = domain.EnrollmentRandom
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &EnrollmentCodes{ttl: cfg.EnrollmentCodeTTL, maxAttempts: cfg.EnrollmentCodeMaxAttempts, pepper: append([]byte(nil), pepper...), d: d}, nil
}

// Issue creates a code of purpose for the person userID inside the ambient unit of work: it locks
// the person, ends every other valid code of the person (at most one code per person, 08 section
// 3.2), stores only the HMAC and returns the display form, which never reaches a log, an audit
// detail or an event. createdBy is the issuing owner, nil for a host command. Errors: kind
// internal outside a unit of work, for an unknown purpose or a failing random source; not_found
// for an unknown person; the store error.
func (e *EnrollmentCodes) Issue(ctx context.Context, userID ids.ID, purpose EnrollmentPurpose, createdBy *ids.ID) (IssuedEnrollmentCode, error) {
	if !txn.InUnit(ctx) {
		return IssuedEnrollmentCode{}, errs.New(errs.CodeInternalError, "identity: Issue needs a unit of work")
	}
	if !purpose.Valid() {
		return IssuedEnrollmentCode{}, errs.New(errs.CodeInternalError, "identity: unknown enrollment purpose")
	}
	if _, found, err := e.d.People.LockUser(ctx, userID); err != nil || !found {
		return IssuedEnrollmentCode{}, orInvisible(err)
	}
	canonical, err := domain.NewEnrollmentCode(e.d.Random)
	if err != nil {
		return IssuedEnrollmentCode{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	now := e.d.Clock.Now()
	rec := EnrollmentCodeRecord{
		ID: ids.New(), UserID: userID, Purpose: purpose, Hash: domain.EnrollmentCodeHash(e.pepper, canonical),
		ExpiresAt: now.Add(e.ttl), CreatedAt: now,
	}
	if createdBy != nil {
		rec.CreatedBy = *createdBy
	}
	if err := e.d.Codes.ExpireOtherEnrollmentCodes(ctx, userID, rec.ID, now); err != nil {
		return IssuedEnrollmentCode{}, err
	}
	if err := e.d.Codes.InsertEnrollmentCode(ctx, rec); err != nil {
		return IssuedEnrollmentCode{}, err
	}
	return IssuedEnrollmentCode{ID: rec.ID, Code: domain.DisplayEnrollmentCode(canonical), Purpose: purpose, ExpiresAt: rec.ExpiresAt}, nil
}

// The causes of a rejected code in audit details (codes, 19 SEC-AUD-5).
const (
	causeCodeMalformed      = "code_malformed"
	causeCodeUnknown        = "code_unknown"
	causeCodeUsed           = "code_used"
	causeCodeExpired        = "code_expired"
	causeCodeExhausted      = "code_exhausted"
	causeCodeWrongPurpose   = "code_wrong_purpose"
	causeCodePersonInactive = "person_not_active"
	causeLocalProviderOff   = "local_provider_disabled"
)

// EnrollmentRejectedError is a code that cannot be redeemed. Every rejection unwraps to the same
// 401 unauthenticated without detail (08 "Schutz der Anmeldung"); Cause and CodeID (zero when no
// row matched) are for the audit entry only.
type EnrollmentRejectedError struct {
	// Cause is one of the code_* causes or person_not_active.
	Cause string
	// CodeID is the matched row; zero for a malformed or unknown code.
	CodeID ids.ID
}

// Error implements error without any value of the code.
func (r *EnrollmentRejectedError) Error() string {
	return "identity: enrollment code rejected (" + r.Cause + ")"
}

// Unwrap returns the one answer of every rejected code.
func (r *EnrollmentRejectedError) Unwrap() error { return errs.New(errs.CodeUnauthenticated, "") }

// Match normalises code, finds it by hash and locks its row FOR UPDATE in the ambient unit of
// work. It returns the code when it is unused, not expired, below
// auth.enrollment_code_max_attempts, of one of the allowed purposes and its person is active.
// Errors: *EnrollmentRejectedError (401 unauthenticated) for every rejected code; kind internal
// outside a unit of work; the store error.
func (e *EnrollmentCodes) Match(ctx context.Context, code string, allowed ...EnrollmentPurpose) (MatchedEnrollmentCode, error) {
	if !txn.InUnit(ctx) {
		return MatchedEnrollmentCode{}, errs.New(errs.CodeInternalError, "identity: Match needs a unit of work")
	}
	canonical, ok := domain.NormalizeEnrollmentCode(code)
	if !ok {
		return MatchedEnrollmentCode{}, &EnrollmentRejectedError{Cause: causeCodeMalformed}
	}
	rec, found, err := e.d.Codes.LockEnrollmentCodeByHash(ctx, domain.EnrollmentCodeHash(e.pepper, canonical))
	if err != nil {
		return MatchedEnrollmentCode{}, err
	}
	return e.check(ctx, rec, found, allowed)
}

// MatchID locks the code id FOR UPDATE in the ambient unit of work and checks it like Match: the
// complete step of a passkey enrollment checks again the code its begin step matched (M1-40).
// Errors: as Match.
func (e *EnrollmentCodes) MatchID(ctx context.Context, id ids.ID, allowed ...EnrollmentPurpose) (MatchedEnrollmentCode, error) {
	if !txn.InUnit(ctx) {
		return MatchedEnrollmentCode{}, errs.New(errs.CodeInternalError, "identity: MatchID needs a unit of work")
	}
	rec, found, err := e.d.Codes.LockEnrollmentCode(ctx, id)
	if err != nil {
		return MatchedEnrollmentCode{}, err
	}
	return e.check(ctx, rec, found, allowed)
}

// check is the decision of Match and MatchID over the locked row rec.
func (e *EnrollmentCodes) check(ctx context.Context, rec EnrollmentCodeRecord, found bool, allowed []EnrollmentPurpose) (MatchedEnrollmentCode, error) {
	if !found {
		return MatchedEnrollmentCode{}, &EnrollmentRejectedError{Cause: causeCodeUnknown}
	}
	if cause := e.unusable(rec); cause != "" {
		return MatchedEnrollmentCode{}, &EnrollmentRejectedError{Cause: cause, CodeID: rec.ID}
	}
	if !purposeAllowed(rec.Purpose, allowed) {
		return MatchedEnrollmentCode{}, &EnrollmentRejectedError{Cause: causeCodeWrongPurpose, CodeID: rec.ID}
	}
	p, found, err := e.d.People.Principal(ctx, rec.UserID)
	if err != nil {
		return MatchedEnrollmentCode{}, err
	}
	if !found || p.Principal.Status != domain.StatusActive {
		return MatchedEnrollmentCode{}, &EnrollmentRejectedError{Cause: causeCodePersonInactive, CodeID: rec.ID}
	}
	return MatchedEnrollmentCode{ID: rec.ID, UserID: rec.UserID, Purpose: rec.Purpose, HostIssued: rec.CreatedBy.IsZero()}, nil
}

// unusable returns the cause that makes rec unusable now, "" for a usable code.
func (e *EnrollmentCodes) unusable(rec EnrollmentCodeRecord) string {
	now := e.d.Clock.Now()
	switch {
	case !rec.UsedAt.IsZero():
		return causeCodeUsed
	case !now.Before(rec.ExpiresAt):
		return causeCodeExpired
	case rec.Attempts >= e.maxAttempts:
		return causeCodeExhausted
	}
	return ""
}

// purposeAllowed reports whether p is among allowed.
func purposeAllowed(p EnrollmentPurpose, allowed []EnrollmentPurpose) bool {
	for _, a := range allowed {
		if a == p {
			return true
		}
	}
	return false
}

// RecordFailedAttempt adds one failed attempt to the code codeID in an own short transaction, so
// the count survives the rollback of the failed redemption (08 section 3.2). Call it after that
// unit of work has ended: inside a unit it fails with kind internal, because the row is still
// locked by the caller. Errors: the store error.
func (e *EnrollmentCodes) RecordFailedAttempt(ctx context.Context, codeID ids.ID) error {
	if txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "identity: RecordFailedAttempt runs after the failed unit of work, not inside one")
	}
	return e.d.UoW.Do(ctx, func(ctx context.Context) error {
		return e.d.Codes.IncrementEnrollmentAttempts(ctx, codeID)
	})
}

// MarkUsed sets used_at and used_by_device_id of the code codeID in the ambient unit of work.
// Errors: kind internal outside a unit of work; *EnrollmentRejectedError when the code was used
// meanwhile (the row lock of Match prevents that within one unit); the store error.
func (e *EnrollmentCodes) MarkUsed(ctx context.Context, codeID, deviceID ids.ID) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "identity: MarkUsed needs a unit of work")
	}
	marked, err := e.d.Codes.MarkEnrollmentCodeUsed(ctx, codeID, deviceID, e.d.Clock.Now())
	if err != nil {
		return err
	}
	if !marked {
		return &EnrollmentRejectedError{Cause: causeCodeUsed, CodeID: codeID}
	}
	return nil
}

// RedeemEnrollmentCode implements identity.Enrollment (brief M1-37 "Redemption"): authorize
// auth.login.complete for the anonymous principal; verify the DPoP proof as a login proof without
// ath (a rejected proof never looks the code up); then, in one unit of work, match the code
// (first_login or recovery, row locked), complete the login with a confirmed device and the
// person's active local identity, and mark the code used. A rejected code rolls back and is
// audited as a denial with its cause; a rejected login with a matching code rolls back, counts a
// failed attempt in an own transaction and is audited too.
func (s *Service) RedeemEnrollmentCode(ctx context.Context, in identity.RedeemEnrollment) (identity.RedeemResult, error) {
	d, err := s.d.Authorizer.Authorize(ctx, actor.Anonymous(), authorize.ActionAuthLoginComplete, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return identity.RedeemResult{}, err
	}
	if err := d.Err(); err != nil {
		return identity.RedeemResult{}, err
	}
	proof, err := s.d.DPoP.Verify(ctx, in.Proof)
	if err != nil {
		return identity.RedeemResult{}, err
	}
	var (
		out     LoginResult
		matched MatchedEnrollmentCode
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		matched = MatchedEnrollmentCode{}
		m, err := s.d.Enrollment.Match(ctx, in.Code, domain.ConsumablePurposes(domain.ConsumerRedeem)...)
		if err != nil {
			return err
		}
		matched = m
		local, err := s.redeemIdentity(ctx, m)
		if err != nil {
			return err
		}
		out, err = s.completeLogin(ctx, LoginCompletion{
			UserID: m.UserID, IdentityID: local, AuthMethod: identity.AuthEnrollmentCode, Device: in.Device,
			ProofJKT: proof.JKT, Trust: TrustConfirmed, ClientIP: in.ClientIP,
		})
		if err != nil {
			return err
		}
		return s.d.Enrollment.MarkUsed(ctx, m.ID, out.Device.ID)
	})
	if err != nil {
		return identity.RedeemResult{}, s.redeemFailed(ctx, err, matched.ID)
	}
	res := identity.RedeemResult{Tokens: out.Tokens}
	if proof.NonceStale {
		res.Nonce = s.d.DPoP.CurrentNonce()
	}
	return res, nil
}

// causeLoginRejected is the audit cause of a rejected login with a matching code.
const causeLoginRejected = "login_rejected"

// redeemFailed audits a failed redemption and returns its answer: a rejected code gives the one
// 401 of every rejected code; a rejected login with the matching code codeID counts a failed
// attempt after the rollback; a server failure is returned unchanged and counts nothing.
func (s *Service) redeemFailed(ctx context.Context, err error, codeID ids.ID) error {
	var rej *EnrollmentRejectedError
	if errors.As(err, &rej) {
		_ = s.d.Audit.RecordDenial(ctx, redeemDenial(rej.Cause, rej.CodeID))
		return rej.Unwrap()
	}
	if codeID.IsZero() || !countsAsAttempt(err) {
		return err
	}
	// The count must not depend on the client: a dropped connection cancels ctx, not the count.
	if aerr := s.d.Enrollment.RecordFailedAttempt(context.WithoutCancel(ctx), codeID); aerr != nil {
		return aerr
	}
	_ = s.d.Audit.RecordDenial(ctx, redeemDenial(causeLoginRejected, codeID))
	return err
}

// redeemDenial is the deny entry of a failed redemption: cause and, when a row matched, the id of
// the code, never the code.
func redeemDenial(cause string, codeID ids.ID) auditing.Entry {
	attrs := auditing.Attrs{attrCause: auditing.Code(cause)}
	if !codeID.IsZero() {
		attrs[attrEnrollmentCodeID] = auditing.ID(codeID)
	}
	return auditing.Entry{
		Action: string(authorize.ActionAuthLoginComplete), Decision: auditing.DecisionDeny, Reason: auditing.ReasonUnauthenticated,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	}
}

// localIdentity returns the active identity of userID at the local provider and whether that
// provider is enabled; inside a unit of work the enabled provider is share-locked, so a concurrent
// disable waits until the login has committed its session (M1-36/7). The identity is nil when
// there is none (an owner without a local identity recovers without one, 08 section 4).
func (s *Service) localIdentity(ctx context.Context, userID ids.ID) (*ids.ID, bool, error) {
	recs, err := s.d.Identities.ActiveIdentitiesOfUser(ctx, userID)
	if err != nil {
		return nil, false, err
	}
	for _, r := range recs {
		if r.ProviderKind != identity.ProviderLocal {
			continue
		}
		_, enabled, err := s.d.Providers.EnabledProvider(ctx, r.ProviderID)
		if err != nil {
			return nil, false, err
		}
		id := r.ID
		return &id, enabled, nil
	}
	return nil, false, nil
}

// redeemIdentity returns the identity a redemption of m logs in with: the person's local identity
// while the local provider is enabled. A disabled local provider closes the code path of a person
// with a local identity (08 section 3.3, R-ID-12): the code is rejected like any other, except a
// recovery code issued on the host, which logs in without the closed identity, because host access
// is the root of trust and an owner must not be locked out (08 section 4, 16 section 4.3).
func (s *Service) redeemIdentity(ctx context.Context, m MatchedEnrollmentCode) (*ids.ID, error) {
	local, enabled, err := s.localIdentity(ctx, m.UserID)
	if err != nil || local == nil || enabled {
		return local, err
	}
	if m.Purpose == PurposeRecovery && m.HostIssued {
		return nil, nil
	}
	return nil, &EnrollmentRejectedError{Cause: causeLocalProviderOff, CodeID: m.ID}
}

// reasonUserNotActive is the reason of 409 conflict for a code for a person that is not active.
const reasonUserNotActive = "user_not_active"

// CreateEnrollmentCode implements identity.Enrollment (operation createEnrollmentCode): user.update
// on the person (owner with step-up through the catalogue) before the person is answered, a human
// with status active (else 409 conflict, reason user_not_active), the purpose from the local
// identity and the devices of the person (domain.IssuePurpose), issued with the caller as creator
// and an allow entry {purpose, enrollment_code_id}.
func (s *Service) CreateEnrollmentCode(ctx context.Context, a actor.Actor, userID ids.ID) (identity.EnrollmentCode, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.EnrollmentCode{}, err
	}
	var out IssuedEnrollmentCode
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.createCode(ctx, a, userID)
		return err
	})
	if err != nil {
		return identity.EnrollmentCode{}, err
	}
	return enrollmentView(out), nil
}

// createCode is CreateEnrollmentCode inside its unit of work.
func (s *Service) createCode(ctx context.Context, a actor.Actor, userID ids.ID) (IssuedEnrollmentCode, error) {
	rec, found, err := s.d.People.User(ctx, userID)
	if err != nil {
		return IssuedEnrollmentCode{}, err
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, authorize.ActionUserUpdate, userResource(userID))
	if err != nil {
		return IssuedEnrollmentCode{}, err
	}
	if err := d.Err(); err != nil {
		return IssuedEnrollmentCode{}, err
	}
	if !found {
		return IssuedEnrollmentCode{}, invisible()
	}
	if rec.User.Principal.Status != domain.StatusActive {
		return IssuedEnrollmentCode{}, errs.New(errs.CodeConflict, "").WithReason(reasonUserNotActive)
	}
	purpose, err := s.issuePurpose(ctx, userID)
	if err != nil {
		return IssuedEnrollmentCode{}, err
	}
	creator := a.ID
	out, err := s.d.Enrollment.Issue(ctx, userID, purpose, &creator)
	if err != nil {
		return IssuedEnrollmentCode{}, err
	}
	return out, s.d.Audit.Record(ctx, codeEntry(authorize.ActionUserUpdate, userID, out))
}

// issuePurpose returns the purpose of a code createEnrollmentCode issues for userID (08 section
// 3.2 table).
func (s *Service) issuePurpose(ctx context.Context, userID ids.ID) (EnrollmentPurpose, error) {
	local, enabled, err := s.localIdentity(ctx, userID)
	if err != nil {
		return "", err
	}
	if local != nil && !enabled {
		// The code could not be redeemed while the local provider is disabled.
		return "", errs.New(errs.CodeConflict, "").WithReason(reasonLocalProviderDisabled)
	}
	devs, err := s.d.Devices.DevicesOfUser(ctx, userID, ids.ID{}, 1)
	if err != nil {
		return "", err
	}
	return domain.IssuePurpose(local != nil, len(devs) > 0), nil
}

// attrPurpose is the audit attribute of the purpose of an issued code.
const attrPurpose = "purpose"

// codeEntry is the allow entry of an issued code on the person userID: purpose and id, never the
// code (19 SEC-AUD-5).
func codeEntry(act authorize.Action, userID ids.ID, c IssuedEnrollmentCode) auditing.Entry {
	return auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceUser, ResourceID: userID,
		Attrs: auditing.Attrs{attrPurpose: auditing.Code(string(c.Purpose)), attrEnrollmentCodeID: auditing.ID(c.ID)},
	}
}

// enrollmentView converts an issued code into the port DTO with the redacting Token.
func enrollmentView(c IssuedEnrollmentCode) identity.EnrollmentCode {
	return identity.EnrollmentCode{ID: c.ID, Code: identity.NewToken(c.Code), Purpose: string(c.Purpose), ExpiresAt: c.ExpiresAt}
}
