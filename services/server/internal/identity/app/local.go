// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net/url"
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

// PasswordHasher hashes and verifies passwords with Argon2id in PHC format (implemented by
// adapters/passwordhash with the parameters auth.argon2id_*). The passwords it gets are already
// normalised to NFC.
type PasswordHasher interface {
	// Hash returns the PHC string of password with a new random salt.
	Hash(password string) (phc string, err error)
	// Verify reports a match and whether the stored parameters differ from the configured ones.
	Verify(phc, password string) (ok bool, needsRehash bool, err error)
	// VerifyDummy spends the same work as Verify for an unknown login name.
	VerifyDummy(password string)
}

// SecretSealer encrypts the TOTP secret of an identity with a key from the secret provider
// (implemented by adapters/secretbox, AES-256-GCM bound to the identity).
type SecretSealer interface {
	// Seal encrypts plaintext for identityID with the current key and returns the ciphertext and
	// the reference of that key, stored in totp_key_ref.
	Seal(identityID ids.ID, plaintext []byte) (ciphertext []byte, keyRef string, err error)
	// Open decrypts ciphertext of identityID with the key behind keyRef. Errors: a wrong
	// identity, a tampered ciphertext or an unknown key.
	Open(identityID ids.ID, ciphertext []byte, keyRef string) ([]byte, error)
}

// CommonPasswords is the local list of common passwords (implemented by adapters/passwordpolicy).
type CommonPasswords interface{ Contains(password string) bool }

// PasswordNormalizer normalises a password to Unicode NFC before every rule, hash and
// verification, so the same password typed on two keyboards is the same password (08 section
// 2.5). adapters/passwordpolicy implements it with golang.org/x/text, which app may not import.
type PasswordNormalizer interface{ NFC(password string) string }

// LocalCredentialRecord is a row of local_credentials. Zero values stand for NULL, except
// TOTPLastStep, which is nil for NULL.
type LocalCredentialRecord struct {
	IdentityID        ids.ID
	PasswordHash      string
	PasswordChangedAt time.Time
	TOTPCiphertext    []byte
	TOTPKeyRef        string
	TOTPLastStep      *int64
	TOTPEnabledAt     time.Time
	// Lockout holds failed_count, fail_window_started_at and locked_until.
	Lockout domain.Lockout
}

// LocalCredentialStore is the outgoing port over local_credentials (implemented by
// adapters/postgres). Writes and the lock run only inside a unit of work.
type LocalCredentialStore interface {
	// LocalCredential returns the row of identityID without a lock; found is false when there is
	// none.
	LocalCredential(ctx context.Context, identityID ids.ID) (rec LocalCredentialRecord, found bool, err error)
	// LockLocalCredential returns the row of identityID locked FOR UPDATE until the end of the unit
	// of work; found is false when there is none.
	LockLocalCredential(ctx context.Context, identityID ids.ID) (rec LocalCredentialRecord, found bool, err error)
	// SaveLocalCredential inserts rec or replaces every column of the existing row.
	SaveLocalCredential(ctx context.Context, rec LocalCredentialRecord) error
	// LocalCredentialOfUser returns the row of the active local identity of userID; found is false
	// when there is none.
	LocalCredentialOfUser(ctx context.Context, userID ids.ID) (rec LocalCredentialRecord, found bool, err error)
}

// LocalDeps are the collaborators of LocalCredentials.
type LocalDeps struct {
	// Credentials reads and writes local_credentials.
	Credentials LocalCredentialStore
	// Providers finds the enabled local provider.
	Providers ProviderStore
	// Identities finds the local identity by login name and of a person.
	Identities IdentityStore
	// People reads the status of the person.
	People Store
	// Logins completes a login (CompleteLogin of M1-37).
	Logins *Service
	// DPoP verifies the proof of a login.
	DPoP identity.DPoP
	// Hasher, Sealer, Common and Normalizer are the outbound ports of the password path.
	Hasher     PasswordHasher
	Sealer     SecretSealer
	Common     CommonPasswords
	Normalizer PasswordNormalizer
	// UoW runs every write.
	UoW txn.UnitOfWork
	// Authorizer decides local_credential.set and auth.login.complete.
	Authorizer authorize.Authorizer
	// Audit writes the allow entries, the lock entries and the denials.
	Audit auditing.Recorder
	// Random is the source of TOTP secrets; nil means crypto/rand.
	Random io.Reader
	// Clock is the time of TOTP steps, windows and locks; nil means clock.System().
	Clock clock.Clock
}

// LocalCredentials implements identity.LocalCredentials (brief M1-39): password with TOTP for
// local accounts. It is safe for concurrent use.
type LocalCredentials struct {
	enabled   bool
	minLength int
	policy    domain.LockoutPolicy
	issuer    string
	d         LocalDeps
}

// Compile-time check: the incoming port.
var _ identity.LocalCredentials = (*LocalCredentials)(nil)

// NewLocalCredentials returns the use cases with auth.local_password_enabled,
// auth.password_min_length, auth.login_fail_limit, auth.login_fail_window,
// auth.login_lock_duration and the host of server.public_url (the otpauth issuer) of cfg. Errors:
// a missing collaborator, a limit or duration below 1, a public URL without host.
func NewLocalCredentials(cfg Config, d LocalDeps) (*LocalCredentials, error) {
	if d.Credentials == nil || d.Providers == nil || d.Identities == nil || d.People == nil || d.Logins == nil || d.DPoP == nil ||
		d.Hasher == nil || d.Sealer == nil || d.Common == nil || d.Normalizer == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil {
		return nil, errors.New("identity: NewLocalCredentials needs every collaborator")
	}
	if cfg.PasswordMinLength < 1 || cfg.LoginFailLimit < 1 || cfg.LoginFailWindow <= 0 || cfg.LoginLockDuration <= 0 {
		return nil, errors.New("identity: auth.password_min_length and auth.login_fail_limit must be at least 1, auth.login_fail_window and auth.login_lock_duration above 0")
	}
	u, err := url.Parse(cfg.PublicURL)
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("identity: server.public_url has no host for the TOTP issuer")
	}
	if d.Random == nil {
		d.Random = rand.Reader
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	return &LocalCredentials{
		enabled: cfg.LocalPasswordEnabled, minLength: cfg.PasswordMinLength, issuer: u.Hostname(), d: d,
		policy: domain.LockoutPolicy{Limit: cfg.LoginFailLimit, Window: cfg.LoginFailWindow, LockDuration: cfg.LoginLockDuration},
	}, nil
}

// Field pointers, codes and reasons of setMyLocalCredential.
const (
	fieldPassword         = "/password"
	fieldTOTPCode         = "/totp_code"
	codeRequired          = "required"
	codeTOTPInvalid       = "totp_invalid"
	reasonNoLocalIdentity = "no_local_identity"
)

// Audit attribute names of the local password path (codes, IDs and flags only, 19 SEC-AUD-5);
// attrIdentityID is shared with the OIDC login (oidc.go).
const (
	attrPasswordChanged = "password_changed"
	attrTOTPEnrolled    = "totp_enroll_started"
	attrTOTPConfirmed   = "totp_confirmed"
	attrLock            = "lock"
)

// SetLocalCredential implements identity.LocalCredentials (brief M1-39 "SetLocalCredential"). The
// switch auth.local_password_enabled is checked first; then, in one unit of work,
// local_credential.set is authorized on the caller (step-up only once the password path is
// established, decided in authz), the active local identity at the enabled local provider is
// found, the row is locked and the members are applied in the order password, totp_enroll,
// totp_code. Any rejected member rolls back the whole request.
func (l *LocalCredentials) SetLocalCredential(ctx context.Context, a actor.Actor, in identity.SetLocalCredential) (identity.LocalCredentialResult, error) {
	if !l.enabled {
		return identity.LocalCredentialResult{}, errs.New(errs.CodeFeatureDisabled, "")
	}
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.LocalCredentialResult{}, err
	}
	var out identity.LocalCredentialResult
	err = l.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = l.set(ctx, a, in)
		return err
	})
	if err != nil {
		return identity.LocalCredentialResult{}, err
	}
	return out, nil
}

// set is SetLocalCredential inside its unit of work.
func (l *LocalCredentials) set(ctx context.Context, a actor.Actor, in identity.SetLocalCredential) (identity.LocalCredentialResult, error) {
	d, err := l.d.Authorizer.Authorize(ctx, a, authorize.ActionLocalCredentialSet, userResource(a.ID))
	if err != nil {
		return identity.LocalCredentialResult{}, err
	}
	if err := d.Err(); err != nil {
		return identity.LocalCredentialResult{}, err
	}
	local, err := l.ownLocalIdentity(ctx, a.ID)
	if err != nil {
		return identity.LocalCredentialResult{}, err
	}
	enroll := in.TOTPEnroll != nil && *in.TOTPEnroll
	if in.Password == nil && !enroll && in.TOTPCode == nil {
		return identity.LocalCredentialResult{}, fieldError("", codeRequired, "Set password, totp_enroll or totp_code.")
	}
	rec, found, err := l.d.Credentials.LockLocalCredential(ctx, local.ID)
	if err != nil {
		return identity.LocalCredentialResult{}, err
	}
	if !found && in.Password == nil {
		return identity.LocalCredentialResult{}, fieldError(fieldPassword, codeRequired, "The first call sets a password.")
	}
	rec.IdentityID = local.ID
	uri, err := l.apply(&rec, in, enroll, local.Subject)
	if err != nil {
		return identity.LocalCredentialResult{}, err
	}
	attrs := auditing.Attrs{attrIdentityID: auditing.ID(local.ID), attrPasswordChanged: auditing.Bool(in.Password != nil),
		attrTOTPEnrolled: auditing.Bool(enroll), attrTOTPConfirmed: auditing.Bool(in.TOTPCode != nil)}
	if err := l.d.Credentials.SaveLocalCredential(ctx, rec); err != nil {
		return identity.LocalCredentialResult{}, err
	}
	err = l.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionLocalCredentialSet), Decision: auditing.DecisionAllow,
		ResourceKind: resourceUser, ResourceID: a.ID, Attrs: attrs,
	})
	if err != nil {
		return identity.LocalCredentialResult{}, err
	}
	return identity.LocalCredentialResult{PasswordSet: rec.PasswordHash != "", TOTPEnabled: !rec.TOTPEnabledAt.IsZero(), TOTPSecretURI: uri}, nil
}

// apply applies the members of in to rec in the order password, totp_enroll, totp_code and
// returns the otpauth URI of a new enrollment (the zero Token without one).
func (l *LocalCredentials) apply(rec *LocalCredentialRecord, in identity.SetLocalCredential, enroll bool, loginName string) (identity.Token, error) {
	now := l.d.Clock.Now()
	if in.Password != nil {
		if err := l.setPassword(rec, *in.Password, now); err != nil {
			return identity.Token{}, err
		}
	}
	var uri identity.Token
	if enroll {
		var err error
		if uri, err = l.enroll(rec, loginName); err != nil {
			return identity.Token{}, err
		}
	}
	if in.TOTPCode != nil {
		if err := l.confirm(rec, *in.TOTPCode, now); err != nil {
			return identity.Token{}, err
		}
	}
	return uri, nil
}

// ownLocalIdentity returns the active identity of userID at the enabled local provider (share
// locked, like a login); conflict no_local_identity when there is none.
func (l *LocalCredentials) ownLocalIdentity(ctx context.Context, userID ids.ID) (identity.Identity, error) {
	recs, err := l.d.Identities.ActiveIdentitiesOfUser(ctx, userID)
	if err != nil {
		return identity.Identity{}, err
	}
	for _, r := range recs {
		if r.ProviderKind != identity.ProviderLocal {
			continue
		}
		_, enabled, err := l.d.Providers.EnabledProvider(ctx, r.ProviderID)
		if err != nil {
			return identity.Identity{}, err
		}
		if enabled {
			return r.Identity, nil
		}
	}
	return identity.Identity{}, errs.New(errs.CodeConflict, "").WithReason(reasonNoLocalIdentity)
}

// setPassword checks the password rules on the NFC form and stores its new hash.
func (l *LocalCredentials) setPassword(rec *LocalCredentialRecord, password string, now time.Time) error {
	nfc := l.d.Normalizer.NFC(password)
	if code := domain.CheckPassword(nfc, l.minLength, l.d.Common.Contains(nfc)); code != "" {
		detail := "The password is too short."
		if code == domain.PasswordCommon {
			detail = "The password is too common."
		}
		return fieldError(fieldPassword, code, detail)
	}
	phc, err := l.d.Hasher.Hash(nfc)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "identity: hashing a password")
	}
	rec.PasswordHash, rec.PasswordChangedAt = phc, now
	return nil
}

// enroll seals a new secret and makes the enrollment pending: totp_enabled_at and totp_last_step
// become NULL until a code confirms it. It returns the otpauth URI, shown once.
func (l *LocalCredentials) enroll(rec *LocalCredentialRecord, loginName string) (identity.Token, error) {
	secret, err := domain.NewTOTPSecret(l.d.Random)
	if err != nil {
		return identity.Token{}, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	ct, ref, err := l.d.Sealer.Seal(rec.IdentityID, secret)
	if err != nil {
		return identity.Token{}, errs.Wrap(errs.CodeInternalError, err, "identity: sealing a TOTP secret")
	}
	rec.TOTPCiphertext, rec.TOTPKeyRef, rec.TOTPEnabledAt, rec.TOTPLastStep = ct, ref, time.Time{}, nil
	return identity.NewToken(domain.TOTPURI(l.issuer, loginName, secret)), nil
}

// confirm verifies code against the stored secret; success enables TOTP and stores the step.
func (l *LocalCredentials) confirm(rec *LocalCredentialRecord, code string, now time.Time) error {
	if len(rec.TOTPCiphertext) == 0 {
		return fieldError(fieldTOTPCode, codeTOTPInvalid, "No TOTP secret is enrolled.")
	}
	secret, err := l.d.Sealer.Open(rec.IdentityID, rec.TOTPCiphertext, rec.TOTPKeyRef)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "identity: opening a TOTP secret")
	}
	step, ok := domain.VerifyTOTP(secret, code, now, rec.TOTPLastStep)
	if !ok {
		return fieldError(fieldTOTPCode, codeTOTPInvalid, "The TOTP code is not valid.")
	}
	rec.TOTPLastStep = &step
	if rec.TOTPEnabledAt.IsZero() {
		rec.TOTPEnabledAt = now
	}
	return nil
}

// fieldError is validation_failed with one field error.
func fieldError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, "").WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// The causes of a rejected password login in audit details (codes only); causePersonNotActive
// is shared with the passkey ceremonies (ceremony.go).
const (
	causeUnknownLogin     = "unknown_login"
	causeLocked           = "locked"
	causeTOTPNotEnabled   = "totp_not_enabled"
	causeWrongPassword    = "wrong_password"
	causeTOTPRejected     = "totp_rejected"
	causeLocalNotPossible = "local_login_not_possible"
	causeChanged          = "credential_changed"
)

// passwordAttempt is the outcome of the unit of work of a password login: the completed login,
// or the cause of a rejection with the facts of its audit entry.
type passwordAttempt struct {
	login    LoginResult
	cause    string
	userID   ids.ID
	identity ids.ID
	locked   bool
}

// LoginWithPassword implements identity.LocalCredentials (brief M1-39 "LoginWithPassword"): 404
// when the switch is off or no local provider is enabled; auth.login.complete for the anonymous
// principal; the DPoP proof as a login proof without ath. Then exactly one Argon2id computation
// (VerifyDummy without a row, also while locked) on a read of the identity, the person and the
// row without locks, so no lock is held while hashing and parallel attempts on one name do not
// queue behind each other (time reveals no account). A known row continues in one unit of work
// that finds them again with the row locked FOR UPDATE, counts a failure of an unlocked row
// (commits although the login is rejected) and on success consumes the TOTP step, resets the
// counter, rehashes when the parameters changed and completes the login with a pending new
// device. Every rejection is the same 401 and a denial in the audit log.
func (l *LocalCredentials) LoginWithPassword(ctx context.Context, in identity.PasswordLogin) (identity.RedeemResult, error) {
	if !l.enabled {
		return identity.RedeemResult{}, invisible()
	}
	provider, found, err := l.localProvider(ctx)
	if err != nil {
		return identity.RedeemResult{}, err
	}
	if !found {
		return identity.RedeemResult{}, invisible()
	}
	d, err := l.d.Authorizer.Authorize(ctx, actor.Anonymous(), authorize.ActionAuthLoginComplete, authorize.Resource{Kind: authorize.ResourceInstance})
	if err != nil {
		return identity.RedeemResult{}, err
	}
	if err := d.Err(); err != nil {
		return identity.RedeemResult{}, err
	}
	proof, err := l.d.Logins.verifyLoginProof(ctx, in.Console, in.Proof)
	if err != nil {
		return identity.RedeemResult{}, err
	}
	name, nfc := strings.ToLower(strings.TrimSpace(in.LoginName)), l.d.Normalizer.NFC(in.Password)
	v, err := l.verify(ctx, provider, name, nfc)
	if err != nil {
		return identity.RedeemResult{}, err
	}
	var at passwordAttempt
	err = l.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		at, err = l.attempt(ctx, provider, name, v, in, proof.JKT)
		return err
	})
	if err != nil {
		return identity.RedeemResult{}, err
	}
	if at.cause != "" {
		_ = l.d.Audit.RecordDenial(ctx, passwordDenial(at))
		return identity.RedeemResult{}, errs.New(errs.CodeUnauthenticated, "")
	}
	res := at.login.Result()
	if proof.NonceStale {
		res.Nonce = l.d.DPoP.CurrentNonce()
	}
	return res, nil
}

// verified is the outcome of the one Argon2id computation of a password login: the attempt so
// far, whether a row was found, the hash that was verified and the result.
type verified struct {
	at         passwordAttempt
	found      bool
	hash       string
	ok, rehash bool
	nfc        string
}

// verify reads identity, person and row of name without locks and computes exactly one Argon2id
// hash: Verify against the row, or VerifyDummy without one, so time reveals no account (ID-T21).
func (l *LocalCredentials) verify(ctx context.Context, p identity.Provider, name, nfc string) (verified, error) {
	at, rec, found, err := l.find(ctx, p, name, false)
	if err != nil {
		return verified{}, err
	}
	if !found {
		l.d.Hasher.VerifyDummy(nfc)
		if at.cause == "" {
			at.cause = causeUnknownLogin
		}
		return verified{at: at}, nil
	}
	ok, rehash, err := l.d.Hasher.Verify(rec.PasswordHash, nfc)
	if err != nil {
		return verified{}, errs.Wrap(errs.CodeInternalError, err, "identity: verifying a password")
	}
	return verified{at: at, found: true, hash: rec.PasswordHash, ok: ok, rehash: rehash, nfc: nfc}, nil
}

// localProvider returns the enabled local provider.
func (l *LocalCredentials) localProvider(ctx context.Context) (identity.Provider, bool, error) {
	ps, err := l.d.Providers.Providers(ctx, true)
	if err != nil {
		return identity.Provider{}, false, err
	}
	for _, p := range ps {
		if p.Kind == identity.ProviderLocal {
			return p, true, nil
		}
	}
	return identity.Provider{}, false, nil
}

// attempt is the unit of work of a password login: it finds identity, person and row again with
// the row locked and decides on the verification v. Without a row in v it runs the lookups of a
// known name in the unit of work and rejects; what remains different is the counter update of a
// known name, well below authz.not_found_timing_tolerance (ID-T21). A row whose hash changed
// since v (a concurrent password change) is rejected without counting. A rejection is a result,
// not an error, so the counter update commits.
func (l *LocalCredentials) attempt(ctx context.Context, p identity.Provider, name string, v verified, in identity.PasswordLogin, jkt string) (passwordAttempt, error) {
	at, rec, found, err := l.find(ctx, p, name, true)
	if err != nil {
		return passwordAttempt{}, err
	}
	if !v.found {
		return v.at, nil
	}
	if !found || rec.PasswordHash != v.hash {
		if at.cause == "" {
			at.cause = causeChanged
		}
		return at, nil
	}
	now := l.d.Clock.Now()
	if rec.Lockout.Locked(now) {
		// A locked row has computed the hash too and counts nothing (brief step 6).
		at.cause = causeLocked
		return at, nil
	}
	if at.cause == "" {
		at.cause, err = l.check(&rec, v.ok, in.TOTPCode, now)
		if err != nil {
			return passwordAttempt{}, err
		}
	}
	if at.cause != "" {
		return l.fail(ctx, at, rec, now)
	}
	return l.succeed(ctx, at, rec, v.nfc, v.rehash, in, jkt)
}

// find returns the identity of loginName at p (re-checked enabled; share locked inside a unit of
// work), its person and its row, locked FOR UPDATE when lock is set. A missing identity, provider
// or row gives found false; a person that is not active gives the cause person_not_active with the
// row still returned, so its failure counts.
func (l *LocalCredentials) find(ctx context.Context, p identity.Provider, loginName string, lock bool) (passwordAttempt, LocalCredentialRecord, bool, error) {
	if _, enabled, err := l.d.Providers.EnabledProvider(ctx, p.ID); err != nil || !enabled {
		return passwordAttempt{cause: causeLocalNotPossible}, LocalCredentialRecord{}, false, err
	}
	ident, found, err := l.d.Identities.ActiveIdentityBySubject(ctx, p.ID, loginName)
	if err != nil || !found {
		return passwordAttempt{}, LocalCredentialRecord{}, false, err
	}
	at := passwordAttempt{userID: ident.UserID, identity: ident.ID}
	user, found, err := l.d.People.User(ctx, ident.UserID)
	if err != nil {
		return passwordAttempt{}, LocalCredentialRecord{}, false, err
	}
	if !found || user.User.Principal.Status != domain.StatusActive {
		at.cause = causePersonNotActive
	}
	read := l.d.Credentials.LocalCredential
	if lock {
		read = l.d.Credentials.LockLocalCredential
	}
	rec, found, err := read(ctx, ident.ID)
	if err != nil {
		return passwordAttempt{}, LocalCredentialRecord{}, false, err
	}
	return at, rec, found, nil
}

// check returns the cause of a rejection of an unlocked row, "" for a valid password with an
// accepted TOTP code; the accepted step is stored in rec.
func (l *LocalCredentials) check(rec *LocalCredentialRecord, passwordOK bool, code string, now time.Time) (string, error) {
	switch {
	case rec.TOTPEnabledAt.IsZero():
		return causeTOTPNotEnabled, nil
	case !passwordOK:
		return causeWrongPassword, nil
	}
	secret, err := l.d.Sealer.Open(rec.IdentityID, rec.TOTPCiphertext, rec.TOTPKeyRef)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, err, "identity: opening a TOTP secret")
	}
	step, ok := domain.VerifyTOTP(secret, code, now, rec.TOTPLastStep)
	if !ok {
		return causeTOTPRejected, nil
	}
	rec.TOTPLastStep = &step
	return "", nil
}

// fail counts the failure of an unlocked row in the failure window and locks the password path at
// auth.login_fail_limit with a deny entry {lock: true}, committed with the counter.
func (l *LocalCredentials) fail(ctx context.Context, at passwordAttempt, rec LocalCredentialRecord, now time.Time) (passwordAttempt, error) {
	rec.Lockout, at.locked = domain.RecordFailure(rec.Lockout, l.policy, now)
	if err := l.d.Credentials.SaveLocalCredential(ctx, rec); err != nil {
		return passwordAttempt{}, err
	}
	if !at.locked {
		return at, nil
	}
	err := l.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionAuthLoginComplete), Decision: auditing.DecisionDeny, Reason: auditing.ReasonUnauthenticated,
		ResourceKind: resourceUser, ResourceID: at.userID,
		Attrs: auditing.Attrs{attrLock: auditing.Bool(true), attrIdentityID: auditing.ID(at.identity)},
	})
	return at, err
}

// succeed resets the counter, rehashes when needed and completes the login: a new device is
// pending (TrustUnconfirmed), a known active device with the same key stays active.
func (l *LocalCredentials) succeed(ctx context.Context, at passwordAttempt, rec LocalCredentialRecord, nfc string, rehash bool,
	in identity.PasswordLogin, jkt string) (passwordAttempt, error) {
	rec.Lockout = domain.Lockout{}
	if rehash {
		phc, err := l.d.Hasher.Hash(nfc)
		if err != nil {
			return passwordAttempt{}, errs.Wrap(errs.CodeInternalError, err, "identity: rehashing a password")
		}
		rec.PasswordHash = phc
	}
	if err := l.d.Credentials.SaveLocalCredential(ctx, rec); err != nil {
		return passwordAttempt{}, err
	}
	identityID := at.identity
	out, err := l.d.Logins.completeLogin(ctx, LoginCompletion{
		UserID: at.userID, IdentityID: &identityID, AuthMethod: identity.AuthPasswordTOTP, Device: in.Device,
		ProofJKT: jkt, Trust: TrustUnconfirmed, ClientIP: in.ClientIP, Console: in.Console,
	})
	if err != nil {
		return passwordAttempt{}, err
	}
	at.login = out
	return at, nil
}

// passwordDenial is the deny entry of a rejected password login: the cause and, when the login
// name matched, the identity; never the login name, the password or the code.
func passwordDenial(at passwordAttempt) auditing.Entry {
	attrs := auditing.Attrs{attrCause: auditing.Code(at.cause)}
	if !at.identity.IsZero() {
		attrs[attrIdentityID] = auditing.ID(at.identity)
	}
	return auditing.Entry{
		Action: string(authorize.ActionAuthLoginComplete), Decision: auditing.DecisionDeny, Reason: auditing.ReasonUnauthenticated,
		ResourceKind: string(authorize.ResourceInstance), Attrs: attrs,
	}
}

// localSource is the LoginMethodSource of local credentials (M1-36 extension point): it fills
// local_password_set and totp_enrolled of the overview. It adds no remaining method, because the
// password needs the local identity, which identitySource counts.
type localSource struct{ store LocalCredentialStore }

// NewLocalSource returns the overview source over store, for IdentitiesDeps.Sources.
func NewLocalSource(store LocalCredentialStore) LoginMethodSource { return localSource{store: store} }

// Contribute implements LoginMethodSource.
func (s localSource) Contribute(ctx context.Context, userID ids.ID, out *identity.CredentialOverview) error {
	rec, found, err := s.store.LocalCredentialOfUser(ctx, userID)
	if err != nil || !found {
		return err
	}
	out.LocalPasswordSet, out.TOTPEnrolled = rec.PasswordHash != "", !rec.TOTPEnabledAt.IsZero()
	return nil
}

// Remaining implements LoginMethodSource.
func (localSource) Remaining(context.Context, ids.ID, ids.ID) (int, error) { return 0, nil }
