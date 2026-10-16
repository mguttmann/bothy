// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

// PasskeysDeps are the collaborators of the passkey use cases (M1-40).
type PasskeysDeps struct {
	// Passkeys reads and writes passkey_credentials.
	Passkeys PasskeyStore
	// Challenges stores the ceremony rows in auth_challenges.
	Challenges ChallengeStore
	// Verifier checks the WebAuthn responses; nil when the relying party is disabled.
	Verifier PasskeyVerifier
	// RelyingParty is the resolved relying party (domain.ResolveRelyingParty).
	RelyingParty domain.RelyingParty
	// Logins completes the logins (CompleteLogin), matches the enrollment codes and verifies the
	// DPoP proofs; its stores read people, identities and providers.
	Logins *Service
	// UoW runs every write; its registry holds the rights, sync and audit hooks.
	UoW txn.UnitOfWork
	// Authorizer decides auth.login.begin, auth.login.complete, passkey.register and
	// passkey.revoke.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries and the denials.
	Audit auditing.Recorder
	// Clock is the time of ceremonies and uses; nil means clock.System().
	Clock clock.Clock
}

// Passkeys implements identity.Passkeys (brief M1-40): login of local accounts, the first setup
// with an enrollment code, further passkeys and their revocation. The state of every ceremony is a
// row of auth_challenges (03 R-DM-31), so it is safe for concurrent use on any number of nodes.
type Passkeys struct {
	cfg Config
	c   *ceremonies
	d   PasskeysDeps
}

// Compile-time check: the incoming port of the passkey operations.
var _ identity.Passkeys = (*Passkeys)(nil)

// NewPasskeys returns the passkey use cases with auth.login_flow_ttl above 0 and the raw token
// pepper (the HMAC key of challenges and decoys). Errors: a missing collaborator (the verifier
// may be nil only for a disabled relying party), an empty pepper or a lifetime of 0.
func NewPasskeys(cfg Config, pepper []byte, d PasskeysDeps) (*Passkeys, error) {
	if d.Passkeys == nil || d.Challenges == nil || d.Logins == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil ||
		(d.Verifier == nil && d.RelyingParty.Enabled) {
		return nil, errors.New("identity: NewPasskeys needs every collaborator")
	}
	if len(pepper) == 0 || cfg.LoginFlowTTL <= 0 || cfg.DisplayNameMaxChars < 1 {
		return nil, errors.New("identity: passkeys need the token pepper, auth.login_flow_ttl above 0 and principal.display_name_max_chars at least 1")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	c := &ceremonies{
		passkeys: d.Passkeys, people: d.Logins.d.People, verifier: d.Verifier, rp: d.RelyingParty,
		pepper: append([]byte(nil), pepper...), audit: d.Audit,
	}
	return &Passkeys{cfg: cfg, c: c, d: d}, nil
}

// The field pointers of the passkey names.
const (
	fieldPasskeyName  = "/passkey_name"
	fieldName         = "/name"
	attrNewPasskeyID  = "passkey_id"
	attrChallenge     = "challenge_id"
	reasonUnreadable  = "unreadable"
	fieldCredential   = "/credential" //nolint:gosec // G101: a field pointer, no secret
	causeLoginRefused = "login_refused"
)

// now is the time of the clock at the precision of PostgreSQL.
func (p *Passkeys) now() time.Time { return p.d.Clock.Now().Truncate(time.Microsecond) }

// authorizeAnonymous decides act for the anonymous principal on the instance.
func (p *Passkeys) authorizeAnonymous(ctx context.Context, act authorize.Action) error {
	d, err := p.d.Authorizer.Authorize(ctx, actor.Anonymous(), act, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return err
	}
	return d.Err()
}

// begin stores the ceremony row rec in one unit of work with options, which returns the options
// and the session and may complete rec (the code of an enrollment), and returns the challenge.
func (p *Passkeys) begin(ctx context.Context, rec ChallengeRecord, raw []byte,
	options func(ctx context.Context, raw []byte, rec *ChallengeRecord) ([]byte, []byte, error)) (identity.WebAuthnChallenge, error) {
	var opts []byte
	err := p.d.UoW.Do(ctx, func(ctx context.Context) error {
		o, session, err := options(ctx, raw, &rec)
		if err != nil {
			return err
		}
		opts, rec.WebAuthnSession = o, session
		return p.d.Challenges.InsertChallenge(ctx, rec)
	})
	if err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	return identity.WebAuthnChallenge{ID: rec.ID, PublicKey: opts, ExpiresAt: rec.ExpiresAt}, nil
}

// record is a new ceremony row of purpose with the HMAC hash, expiring after auth.login_flow_ttl.
func (p *Passkeys) record(purpose ChallengePurpose, hash []byte) ChallengeRecord {
	now := p.now()
	return ChallengeRecord{ID: ids.New(), Purpose: purpose, Hash: hash, Status: ChallengeIssued, CreatedAt: now, ExpiresAt: now.Add(p.cfg.LoginFlowTTL)}
}

// BeginPasskeyLogin implements identity.Passkeys (brief M1-40 "Login"): with a login name the
// active passkeys of the person with that local identity, else one decoy credential ID, so the
// answer has the same shape; without a name no allowCredentials (discoverable). Residual risk
// (docs/DECISIONS.md M1-40): the number of real passkeys is visible for a known name.
func (p *Passkeys) BeginPasskeyLogin(ctx context.Context, loginName string) (identity.WebAuthnChallenge, error) {
	if err := p.c.enabled(); err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	if err := p.authorizeAnonymous(ctx, authorize.ActionAuthLoginBegin); err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	raw, hash, err := p.c.newChallenge()
	if err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	return p.begin(ctx, p.record(PurposePasskeyLogin, hash), raw, func(ctx context.Context, raw []byte, _ *ChallengeRecord) ([]byte, []byte, error) {
		var allow []StoredPasskey
		if loginName != "" {
			var err error
			if allow, err = p.loginAllow(ctx, loginName); err != nil {
				return nil, nil, err
			}
		}
		opts, err := p.c.verifier.RequestOptions(allow, raw)
		if err != nil {
			return nil, nil, errs.Wrap(errs.CodeInternalError, err, "identity: request options")
		}
		session, err := p.c.session(allow, nil)
		return opts, session, err
	})
}

// loginAllow returns the active passkeys of the person whose active identity at the enabled local
// provider has the subject loginName (lower case), or one decoy for any other name.
func (p *Passkeys) loginAllow(ctx context.Context, loginName string) ([]StoredPasskey, error) {
	decoy := []StoredPasskey{{CredentialID: domain.DecoyCredentialID(p.c.pepper, loginName)}}
	providers, err := p.d.Logins.d.Providers.Providers(ctx, true)
	if err != nil {
		return nil, err
	}
	for _, prov := range providers {
		if prov.Kind != identity.ProviderLocal {
			continue
		}
		rec, found, err := p.d.Logins.d.Identities.ActiveIdentityBySubject(ctx, prov.ID, strings.ToLower(loginName))
		if err != nil || !found {
			return decoy, err
		}
		list, err := p.d.Passkeys.ActivePasskeysOfUser(ctx, rec.UserID)
		if err != nil || len(list) == 0 {
			return decoy, err
		}
		return list, nil
	}
	return decoy, nil
}

// CompletePasskeyLogin implements identity.Passkeys (brief M1-40 "Login"): authorize
// auth.login.complete, verify the DPoP proof as a login proof without ath, then in one unit of
// work open the challenge, verify the assertion, require an active identity at the enabled local
// provider (a passkey login of an OIDC-only person would bypass auth.idp_recheck_interval), apply
// the sign counter and complete the login with an unconfirmed device. Every refusal burns the
// challenge, is audited and answers the same 401.
func (p *Passkeys) CompletePasskeyLogin(ctx context.Context, in identity.PasskeyLogin) (identity.RedeemResult, error) {
	if err := p.c.enabled(); err != nil {
		return identity.RedeemResult{}, err
	}
	if err := p.authorizeAnonymous(ctx, authorize.ActionAuthLoginComplete); err != nil {
		return identity.RedeemResult{}, err
	}
	proof, err := p.d.Logins.verifyLoginProof(ctx, in.Console, in.Proof)
	if err != nil {
		return identity.RedeemResult{}, err
	}
	var (
		out    LoginResult
		opened ids.ID
	)
	err = p.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = p.login(ctx, in, proof.JKT, &opened)
		return err
	})
	if err != nil {
		return identity.RedeemResult{}, p.loginFailed(ctx, err, opened)
	}
	res := out.Result()
	if proof.NonceStale {
		res.Nonce = p.d.Logins.d.DPoP.CurrentNonce()
	}
	return res, nil
}

// login is the body of CompletePasskeyLogin inside its unit of work; opened receives the row it
// opened.
func (p *Passkeys) login(ctx context.Context, in identity.PasskeyLogin, jkt string, opened *ids.ID) (LoginResult, error) {
	now := p.now()
	ch, err := p.c.open(ctx, p.d.Challenges, in.ChallengeID, PurposePasskeyLogin, now)
	*opened = ch.ID
	if err != nil {
		return LoginResult{}, err
	}
	res, err := p.c.assertion(ctx, ch, in.Credential, ids.ID{})
	if err != nil {
		return LoginResult{}, err
	}
	local, enabled, err := p.d.Logins.localIdentity(ctx, res.User.ID)
	if err != nil {
		return LoginResult{}, err
	}
	if local == nil || !enabled {
		return LoginResult{}, refused(causeNoLocalIdentity)
	}
	if err := p.c.recordUse(ctx, res, authorize.ActionAuthLoginComplete, now); err != nil {
		return LoginResult{}, err
	}
	if err := p.d.Challenges.FinishChallenge(ctx, ch.ID, ChallengeUsed, now); err != nil {
		return LoginResult{}, err
	}
	return p.d.Logins.completeLogin(ctx, LoginCompletion{
		UserID: res.User.ID, IdentityID: local, AuthMethod: identity.AuthPasskey, Device: in.Device, ProofJKT: jkt,
		Trust: TrustUnconfirmed, ClientIP: in.ClientIP, Console: in.Console,
	})
}

// loginFailed burns the opened challenge after a failed login or enrollment and maps a refusal
// to the one 401 of every rejected login, audited with its cause; other errors pass unchanged.
func (p *Passkeys) loginFailed(ctx context.Context, err error, opened ids.ID) error {
	if berr := burn(ctx, p.d.UoW, p.d.Challenges, opened, p.now()); berr != nil {
		return berr
	}
	cause, ok := refusalCause(err)
	if !ok {
		return err
	}
	_ = p.d.Audit.RecordDenial(ctx, loginDenial(cause, opened))
	return errs.New(errs.CodeUnauthenticated, "")
}

// loginDenial is the deny entry of a refused passkey login or enrollment: cause and challenge id.
func loginDenial(cause string, challengeID ids.ID) auditing.Entry {
	attrs := auditing.Attrs{attrCause: auditing.Code(cause)}
	if !challengeID.IsZero() {
		attrs[attrChallenge] = auditing.ID(challengeID)
	}
	return auditing.Entry{
		Action: string(authorize.ActionAuthLoginComplete), Decision: auditing.DecisionDeny, Reason: auditing.ReasonUnauthenticated,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	}
}

// BeginEnrollment implements identity.Passkeys (brief M1-40 "Enrollment with passkey"): the code
// is matched read-only (no attempt is counted on begin) for first_login or recovery, the creation
// options are for the code's person with its active passkeys excluded, and the row keeps the code.
func (p *Passkeys) BeginEnrollment(ctx context.Context, code string) (identity.WebAuthnChallenge, error) {
	if err := p.c.enabled(); err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	if err := p.authorizeAnonymous(ctx, authorize.ActionAuthLoginBegin); err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	raw, hash, err := p.c.newChallenge()
	if err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	out, err := p.begin(ctx, p.record(PurposePasskeyEnroll, hash), raw, func(ctx context.Context, raw []byte, rec *ChallengeRecord) ([]byte, []byte, error) {
		m, err := p.d.Logins.d.Enrollment.Match(ctx, code, domain.ConsumablePurposes(domain.ConsumerPasskey)...)
		if err != nil {
			return nil, nil, err
		}
		if _, err := p.d.Logins.redeemIdentity(ctx, m); err != nil {
			return nil, nil, err
		}
		rec.EnrollmentCodeID = m.ID
		return p.creation(ctx, m.UserID, raw)
	})
	var rej *EnrollmentRejectedError
	if errors.As(err, &rej) {
		_ = p.d.Audit.RecordDenial(ctx, redeemDenial(rej.Cause, rej.CodeID))
		return identity.WebAuthnChallenge{}, rej.Unwrap()
	}
	if _, ok := refusalCause(err); ok {
		return identity.WebAuthnChallenge{}, errs.New(errs.CodeUnauthenticated, "")
	}
	return out, err
}

// creation returns the creation options and the session of a creation for the person userID.
func (p *Passkeys) creation(ctx context.Context, userID ids.ID, raw []byte) ([]byte, []byte, error) {
	u, err := p.c.person(ctx, p.d.Logins.d.Identities, userID)
	if err != nil {
		return nil, nil, err
	}
	opts, err := p.c.verifier.CreationOptions(u, raw)
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternalError, err, "identity: creation options")
	}
	session, err := p.c.session(nil, domain.UserHandle(userID))
	return opts, session, err
}

// created verifies the creation response credential against the open challenge ch for the person
// userID and returns the new passkey with name. Errors: a refusal; validation_failed on pointer
// for a bad name.
func (p *Passkeys) created(ctx context.Context, ch ChallengeRecord, userID ids.ID, credential json.RawMessage, name, pointer string, now time.Time) (StoredPasskey, error) {
	raw, s, err := p.c.checked(ch, credential)
	if err != nil {
		return StoredPasskey{}, err
	}
	if h, ok := domain.UserOfHandle(mustHandle(s.UserHandle)); !ok || h != userID {
		return StoredPasskey{}, refused(causeUserHandle)
	}
	if err := domain.CheckPasskeyName(pointer, name, p.cfg.DisplayNameMaxChars); err != nil {
		return StoredPasskey{}, err
	}
	u, err := p.c.person(ctx, p.d.Logins.d.Identities, userID)
	if err != nil {
		return StoredPasskey{}, err
	}
	np, err := p.c.verifier.VerifyCreation(u, raw, credential)
	if errors.Is(err, ErrPasskeyRejected) {
		return StoredPasskey{}, refused(causeVerification)
	}
	if err != nil {
		return StoredPasskey{}, err
	}
	return p.d.Passkeys.InsertPasskey(ctx, StoredPasskey{
		ID: ids.New(), UserID: userID, CredentialID: np.CredentialID, PublicKeyCOSE: np.PublicKeyCOSE, SignCount: np.SignCount,
		AAGUID: np.AAGUID, Transports: np.Transports, BackupEligible: np.BackupEligible, BackupState: np.BackupState, Name: name,
		CreatedAt: now,
	})
}

// mustHandle decodes the base64url user handle of a stored session; nil when it is not one.
func mustHandle(s string) []byte {
	b, err := domain.DecodeChallenge(s)
	if err != nil {
		return nil
	}
	return b
}

// CompleteEnrollment implements identity.Passkeys (brief M1-40 "Enrollment with passkey"): in one
// unit of work open the challenge, lock and check its code again, verify the creation response,
// store the passkey, complete the login with a confirmed device and the person's local identity
// and mark the code used. A rejected code answers the one 401 of every rejected code; a rejected
// response or login with the matching code rolls back, counts a failed attempt in an own
// transaction and answers 401 (a rejected device keeps its own answer, like redeemEnrollmentCode).
func (p *Passkeys) CompleteEnrollment(ctx context.Context, in identity.PasskeyEnrollment) (identity.RedeemResult, error) {
	if err := p.c.enabled(); err != nil {
		return identity.RedeemResult{}, err
	}
	if err := p.authorizeAnonymous(ctx, authorize.ActionAuthLoginComplete); err != nil {
		return identity.RedeemResult{}, err
	}
	proof, err := p.d.Logins.d.DPoP.Verify(ctx, in.Proof)
	if err != nil {
		return identity.RedeemResult{}, err
	}
	var (
		out             LoginResult
		opened, matched ids.ID
	)
	err = p.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = p.enroll(ctx, in, proof.JKT, &opened, &matched)
		return err
	})
	if err != nil {
		return identity.RedeemResult{}, p.enrollFailed(ctx, err, opened, matched)
	}
	res := identity.RedeemResult{Tokens: out.Tokens}
	if proof.NonceStale {
		res.Nonce = p.d.Logins.d.DPoP.CurrentNonce()
	}
	return res, nil
}

// enroll is the body of CompleteEnrollment inside its unit of work; opened and matched receive
// the row and the code it opened.
func (p *Passkeys) enroll(ctx context.Context, in identity.PasskeyEnrollment, jkt string, opened, matched *ids.ID) (LoginResult, error) {
	now := p.now()
	*opened, *matched = ids.ID{}, ids.ID{}
	ch, err := p.c.open(ctx, p.d.Challenges, in.ChallengeID, PurposePasskeyEnroll, now)
	*opened = ch.ID
	if err != nil {
		return LoginResult{}, err
	}
	m, err := p.d.Logins.d.Enrollment.MatchID(ctx, ch.EnrollmentCodeID, domain.ConsumablePurposes(domain.ConsumerPasskey)...)
	if err != nil {
		return LoginResult{}, err
	}
	*matched = m.ID
	local, err := p.d.Logins.redeemIdentity(ctx, m)
	if err != nil {
		return LoginResult{}, err
	}
	if _, err := p.created(ctx, ch, m.UserID, in.Credential, in.PasskeyName, fieldPasskeyName, now); err != nil {
		return LoginResult{}, err
	}
	if err := p.d.Challenges.FinishChallenge(ctx, ch.ID, ChallengeUsed, now); err != nil {
		return LoginResult{}, err
	}
	out, err := p.d.Logins.completeLogin(ctx, LoginCompletion{
		UserID: m.UserID, IdentityID: local, AuthMethod: identity.AuthEnrollmentCode, Device: in.Device, ProofJKT: jkt,
		Trust: TrustConfirmed, ClientIP: in.ClientIP,
	})
	if err != nil {
		return LoginResult{}, err
	}
	return out, p.d.Logins.d.Enrollment.MarkUsed(ctx, m.ID, out.Device.ID)
}

// enrollFailed burns the challenge, counts a failed attempt of the matched code for a refused
// response and for a rejected login (08 section 3.2), audits and answers: 401 for a rejected code
// or response, the error itself for a rejected device or a server failure.
func (p *Passkeys) enrollFailed(ctx context.Context, err error, opened, matched ids.ID) error {
	if berr := burn(ctx, p.d.UoW, p.d.Challenges, opened, p.now()); berr != nil {
		return berr
	}
	var rej *EnrollmentRejectedError
	if errors.As(err, &rej) {
		_ = p.d.Audit.RecordDenial(ctx, redeemDenial(rej.Cause, rej.CodeID))
		return rej.Unwrap()
	}
	cause, refusedResponse := refusalCause(err)
	if matched.IsZero() || !refusedResponse && !countsAsAttempt(err) {
		if refusedResponse {
			_ = p.d.Audit.RecordDenial(ctx, loginDenial(cause, opened))
			return errs.New(errs.CodeUnauthenticated, "")
		}
		return err
	}
	// The count must not depend on the client: a dropped connection cancels ctx, not the count.
	if aerr := p.d.Logins.d.Enrollment.RecordFailedAttempt(context.WithoutCancel(ctx), matched); aerr != nil {
		return aerr
	}
	if refusedResponse {
		_ = p.d.Audit.RecordDenial(ctx, redeemDenial(cause, matched))
		return errs.New(errs.CodeUnauthenticated, "")
	}
	_ = p.d.Audit.RecordDenial(ctx, redeemDenial(causeLoginRejected, matched))
	return err
}

// BeginPasskeyRegistration implements identity.Passkeys (brief M1-40 "Registration"):
// passkey.register on the caller (self; step-up once an active passkey exists, decided in authz),
// creation options with the caller's active passkeys excluded, the row bound to the caller's
// session.
func (p *Passkeys) BeginPasskeyRegistration(ctx context.Context, a actor.Actor) (identity.WebAuthnChallenge, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	if err := p.c.enabled(); err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	raw, hash, err := p.c.newChallenge()
	if err != nil {
		return identity.WebAuthnChallenge{}, err
	}
	rec := p.record(PurposePasskeyRegister, hash)
	rec.SessionID = a.SessionID
	out, err := p.begin(ctx, rec, raw, func(ctx context.Context, raw []byte, _ *ChallengeRecord) ([]byte, []byte, error) {
		if err := p.authorizeRegister(ctx, a); err != nil {
			return nil, nil, err
		}
		return p.creation(ctx, a.ID, raw)
	})
	if _, ok := refusalCause(err); ok {
		return identity.WebAuthnChallenge{}, errs.New(errs.CodeValidationFailed, "").WithReason(causePersonNotActive)
	}
	return out, err
}

// authorizeRegister decides passkey.register on the caller itself.
func (p *Passkeys) authorizeRegister(ctx context.Context, a actor.Actor) error {
	d, err := p.d.Authorizer.Authorize(ctx, a, authorize.ActionPasskeyRegister, userResource(a.ID))
	if err != nil {
		return err
	}
	return d.Err()
}

// CompletePasskeyRegistration implements identity.Passkeys (brief M1-40 "Registration"): in one
// unit of work authorize passkey.register again, open the challenge of the caller's session,
// verify the creation response, store the passkey (409 conflict for a credential that exists) and
// write the allow entry. A refused challenge or response burns the challenge and answers 400
// validation_failed.
func (p *Passkeys) CompletePasskeyRegistration(ctx context.Context, a actor.Actor, challengeID ids.ID, credential json.RawMessage,
	name string) (identity.Passkey, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.Passkey{}, err
	}
	if err := p.c.enabled(); err != nil {
		return identity.Passkey{}, err
	}
	var (
		pk     StoredPasskey
		opened ids.ID
	)
	err = p.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		pk, err = p.registered(ctx, a, challengeID, credential, name, &opened)
		return err
	})
	if err == nil {
		return passkeyView(pk), nil
	}
	if berr := burn(ctx, p.d.UoW, p.d.Challenges, opened, p.now()); berr != nil {
		return identity.Passkey{}, berr
	}
	// A refused response is a bad request, not a denial of 07 section 4.1: no deny entry.
	if _, ok := refusalCause(err); ok {
		return identity.Passkey{}, errs.New(errs.CodeValidationFailed, "The passkey response was not accepted; start a new registration.").
			WithFields(errs.FieldError{Pointer: fieldCredential, Code: reasonUnreadable, Detail: "not accepted"})
	}
	return identity.Passkey{}, err
}

// registered is the body of CompletePasskeyRegistration inside its unit of work; opened receives
// the row it opened for the caller's session.
func (p *Passkeys) registered(ctx context.Context, a actor.Actor, challengeID ids.ID, credential json.RawMessage, name string,
	opened *ids.ID) (StoredPasskey, error) {
	*opened = ids.ID{}
	if err := p.authorizeRegister(ctx, a); err != nil {
		return StoredPasskey{}, err
	}
	now := p.now()
	ch, err := p.c.open(ctx, p.d.Challenges, challengeID, PurposePasskeyRegister, now)
	if err != nil {
		return StoredPasskey{}, err
	}
	if ch.SessionID != a.SessionID || a.SessionID.IsZero() {
		// Another session's challenge stays usable for its own session.
		return StoredPasskey{}, refused(causeForeignCeremony)
	}
	*opened = ch.ID
	pk, err := p.created(ctx, ch, a.ID, credential, name, fieldName, now)
	if err != nil {
		return StoredPasskey{}, err
	}
	if err := p.d.Challenges.FinishChallenge(ctx, ch.ID, ChallengeUsed, now); err != nil {
		return StoredPasskey{}, err
	}
	return pk, p.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionPasskeyRegister), Decision: auditing.DecisionAllow, ResourceKind: resourceUser, ResourceID: a.ID,
		Attrs: auditing.Attrs{attrNewPasskeyID: auditing.ID(pk.ID), attrChallenge: auditing.ID(ch.ID)},
	})
}

// passkeyView converts a stored passkey into the port DTO (schema Passkey).
func passkeyView(p StoredPasskey) identity.Passkey {
	return identity.Passkey{
		ID: p.ID, Name: p.Name, AAGUID: p.AAGUID, BackupEligible: p.BackupEligible, CreatedAt: p.CreatedAt, LastUsedAt: p.LastUsedAt,
	}
}

// RevokePasskey implements identity.Passkeys (brief M1-40 "Revoke"): passkey.revoke on the
// passkey owned by its person (self or owner, step-up) before its absence is answered; a passkey
// of another person than userID is not_found; a revoked passkey answers without change.
func (p *Passkeys) RevokePasskey(ctx context.Context, a actor.Actor, userID, passkeyID ids.ID) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	return p.d.UoW.Do(ctx, func(ctx context.Context) error {
		pk, found, err := p.d.Passkeys.LockPasskey(ctx, passkeyID)
		if err != nil {
			return err
		}
		owner := userID
		if found {
			owner = pk.UserID
		}
		d, err := p.d.Authorizer.Authorize(ctx, a, authorize.ActionPasskeyRevoke,
			authorize.Resource{Kind: authorize.ResourcePasskey, ID: passkeyID, OwnerID: owner})
		if err != nil {
			return err
		}
		if err := d.Err(); err != nil {
			return err
		}
		if !found || pk.UserID != userID {
			return invisible()
		}
		revoked, err := p.d.Passkeys.RevokePasskey(ctx, passkeyID, p.now())
		if err != nil {
			return err
		}
		return p.d.Audit.Record(ctx, auditing.Entry{
			Action: string(authorize.ActionPasskeyRevoke), Decision: auditing.DecisionAllow, ResourceKind: resourcePasskey, ResourceID: passkeyID,
			Attrs: auditing.Attrs{attrNewPasskeyID: auditing.ID(passkeyID), attrNoChange: auditing.Bool(!revoked)},
		})
	})
}

// PasskeySource is the sign-in method source of passkeys (the M1-36 extension point
// LoginMethodSource): the credential overview lists the active passkeys of the person, and an
// active passkey keeps a way in when an identity is unlinked (08 section 3.3), but only while the
// person keeps an active identity at the enabled local provider, because a passkey logs in only
// with one (08 section 2.5, docs/DECISIONS.md M1-40/10).
type PasskeySource struct {
	store      PasskeyStore
	identities IdentityStore
}

// Compile-time check: a source of IdentitiesDeps.Sources.
var _ LoginMethodSource = PasskeySource{}

// NewPasskeySource returns the source over the passkey and identity stores.
func NewPasskeySource(store PasskeyStore, identities IdentityStore) PasskeySource {
	return PasskeySource{store: store, identities: identities}
}

// Contribute implements LoginMethodSource: the active passkeys, oldest first.
func (s PasskeySource) Contribute(ctx context.Context, userID ids.ID, out *identity.CredentialOverview) error {
	list, err := s.store.ActivePasskeysOfUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, p := range list {
		out.Passkeys = append(out.Passkeys, passkeyView(p))
	}
	return nil
}

// Remaining implements LoginMethodSource: the number of active passkeys while an active identity
// at an enabled local provider other than unlinking remains, else 0.
func (s PasskeySource) Remaining(ctx context.Context, userID, unlinking ids.ID) (int, error) {
	list, err := s.identities.ActiveIdentitiesOfUser(ctx, userID)
	if err != nil {
		return 0, err
	}
	for _, r := range list {
		if r.ProviderKind == identity.ProviderLocal && r.ProviderEnabled && r.ID != unlinking {
			return s.store.CountActivePasskeys(ctx, userID)
		}
	}
	return 0, nil
}
