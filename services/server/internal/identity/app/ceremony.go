// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"time"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/identity/domain/stepup"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// The causes of a refused ceremony in audit details (codes only, 19 SEC-AUD-5).
const (
	causeCeremonyUnknown    = "challenge_unknown"
	causeCeremonyFinished   = "challenge_finished"
	causeCeremonyExpired    = "challenge_expired"
	causeChallengeMismatch  = "challenge_mismatch"
	causeSessionInvalid     = "webauthn_session_invalid"
	causeVerification       = "verification_failed"
	causeCredentialUnknown  = "passkey_unknown"
	causeCredentialRevoked  = "passkey_revoked"
	causeCredentialRefused  = "passkey_not_allowed"
	causeForeignPasskey     = "foreign_passkey"
	causeUserHandle         = "user_handle_mismatch"
	causePersonNotActive    = "person_not_active"
	causeNoLocalIdentity    = "no_local_identity"
	causeForeignCeremony    = "foreign_session"
	causePasskeysDisabled   = "passkeys_disabled"
	attrPasskeyID           = "passkey_id"
	attrSignCountRegression = "sign_count_regression"
	attrStored              = "stored"
	attrReceived            = "received"
	resourcePasskey         = "passkey"
)

// ceremonyRefusedError is a refused WebAuthn ceremony: the cause for the audit details only. The
// answer depends on the operation (401 for login and enrollment, 400 for a registration, 401
// step_up_required for a step-up).
type ceremonyRefusedError struct{ cause string }

// Error implements error without any value of the ceremony.
func (e *ceremonyRefusedError) Error() string {
	return "identity: passkey ceremony refused (" + e.cause + ")"
}

// refused returns a ceremonyRefusedError with cause.
func refused(cause string) error { return &ceremonyRefusedError{cause: cause} }

// refusalCause returns the cause of a refused ceremony; ok is false for any other error.
func refusalCause(err error) (string, bool) {
	var r *ceremonyRefusedError
	if errors.As(err, &r) {
		return r.cause, true
	}
	return "", false
}

// ceremonies holds what every WebAuthn ceremony needs (M1-40): the passkey store, the people,
// the verifier, the relying party and the pepper of the challenge HMAC. The passkey use cases and
// the step-up share it.
type ceremonies struct {
	passkeys PasskeyStore
	people   Store
	verifier PasskeyVerifier
	rp       domain.RelyingParty
	pepper   []byte
	audit    auditing.Recorder
}

// enabled answers feature_disabled on an instance without a relying party (an IP address as host
// of server.public_url without auth.webauthn_rp_id, docs/DECISIONS.md M1-40).
func (c *ceremonies) enabled() error {
	if c.verifier == nil || !c.rp.Enabled {
		return errs.New(errs.CodeFeatureDisabled, "Passkeys need a domain name as relying party.").WithReason(causePasskeysDisabled)
	}
	return nil
}

// newChallenge returns a new challenge value (32 bytes from crypto/rand, base64url) with its raw
// bytes and its HMAC.
func (c *ceremonies) newChallenge() (raw, hash []byte, err error) {
	value, err := stepup.NewChallengeValue()
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternalError, err, "identity: random source")
	}
	raw, err = domain.DecodeChallenge(value)
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternalError, err, "identity: challenge value")
	}
	return raw, stepup.ChallengeHash(c.pepper, value), nil
}

// session encodes the webauthn_session.v1 of a ceremony over the allowed passkeys and the user
// handle (nil for none).
func (c *ceremonies) session(allow []StoredPasskey, userHandle []byte) ([]byte, error) {
	idsOf := make([][]byte, 0, len(allow))
	for _, p := range allow {
		idsOf = append(idsOf, p.CredentialID)
	}
	b, err := domain.NewWebAuthnSession(c.rp.ID, idsOf, userHandle).Marshal()
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "identity: webauthn_session")
	}
	return b, nil
}

// open locks the challenge id in the ambient unit of work and checks that it is of purpose,
// issued and not expired at now. found reports a row of that purpose, which a failed ceremony
// must burn. Errors: a refusal for an unknown, foreign, finished or expired challenge; the store
// error.
func (c *ceremonies) open(ctx context.Context, store ChallengeStore, id ids.ID, purpose ChallengePurpose, now time.Time) (ChallengeRecord, error) {
	ch, found, err := store.LockChallenge(ctx, id)
	if err != nil {
		return ChallengeRecord{}, err
	}
	if !found || ch.Purpose != purpose {
		return ChallengeRecord{}, refused(causeCeremonyUnknown)
	}
	switch domain.CheckCeremony(string(ch.Status), ch.ExpiresAt, now) {
	case domain.CeremonyOpen:
		return ch, nil
	case domain.CeremonyFinished:
		return ch, refused(causeCeremonyFinished)
	case domain.CeremonyExpired:
		return ch, refused(causeCeremonyExpired)
	}
	return ch, refused(causeCeremonyUnknown)
}

// checked returns the raw challenge of credentialJSON when its HMAC is the one of ch, and the
// stored session of ch, whose RP ID must be the current one. Errors: a refusal.
func (c *ceremonies) checked(ch ChallengeRecord, credentialJSON []byte) ([]byte, domain.WebAuthnSession, error) {
	value, ok := domain.ClientDataChallenge(credentialJSON)
	if !ok || !domain.ChallengeMatches(c.pepper, ch.Hash, value) {
		return nil, domain.WebAuthnSession{}, refused(causeChallengeMismatch)
	}
	raw, err := domain.DecodeChallenge(value)
	if err != nil {
		return nil, domain.WebAuthnSession{}, refused(causeChallengeMismatch)
	}
	s, err := domain.ParseWebAuthnSession(ch.WebAuthnSession)
	if err != nil || s.RPID != c.rp.ID {
		return nil, domain.WebAuthnSession{}, refused(causeSessionInvalid)
	}
	return raw, s, nil
}

// person returns the ceremony user of the active person userID: name is the subject of the local
// identity when there is one, else the handle (brief M1-40 "Enrollment with passkey"); the
// credentials are the active passkeys. Errors: a refusal for a person that is not active; the
// store error.
func (c *ceremonies) person(ctx context.Context, idents IdentityStore, userID ids.ID) (CeremonyUser, error) {
	rec, found, err := c.people.User(ctx, userID)
	if err != nil {
		return CeremonyUser{}, err
	}
	if !found || rec.User.Principal.Status != domain.StatusActive {
		return CeremonyUser{}, refused(causePersonNotActive)
	}
	p := rec.User.Principal
	u := CeremonyUser{ID: userID, Name: p.Handle.String(), DisplayName: p.DisplayName.String()}
	if idents != nil {
		list, err := idents.ActiveIdentitiesOfUser(ctx, userID)
		if err != nil {
			return CeremonyUser{}, err
		}
		for _, r := range list {
			if r.ProviderKind == identity.ProviderLocal {
				u.Name = r.Subject
				break
			}
		}
	}
	if u.Credentials, err = c.passkeys.ActivePasskeysOfUser(ctx, userID); err != nil {
		return CeremonyUser{}, err
	}
	return u, nil
}

// lookup is the credential lookup of an assertion: the credential must be allowed by the stored
// session, known, not revoked, of the person only (when only is not zero) and of the person the
// user handle names; the person must be active.
func (c *ceremonies) lookup(ctx context.Context, s domain.WebAuthnSession, only ids.ID) func(credentialID, userHandle []byte) (CeremonyUser, error) {
	return func(credentialID, userHandle []byte) (CeremonyUser, error) {
		if !s.Allows(credentialID) {
			return CeremonyUser{}, refused(causeCredentialRefused)
		}
		pk, found, err := c.passkeys.PasskeyByCredentialID(ctx, credentialID)
		switch {
		case err != nil:
			return CeremonyUser{}, err
		case !found:
			return CeremonyUser{}, refused(causeCredentialUnknown)
		case !pk.RevokedAt.IsZero():
			return CeremonyUser{}, refused(causeCredentialRevoked)
		case !only.IsZero() && pk.UserID != only:
			return CeremonyUser{}, refused(causeForeignPasskey)
		}
		if userHandle != nil {
			if id, ok := domain.UserOfHandle(userHandle); !ok || id != pk.UserID {
				return CeremonyUser{}, refused(causeUserHandle)
			}
		}
		return c.person(ctx, nil, pk.UserID)
	}
}

// assertion checks credentialJSON against the open challenge ch: the challenge HMAC, the stored
// session and the assertion through the verifier with lookup (restricted to the person only when
// it is not zero). Errors: a refusal for every rejected response; the store error.
func (c *ceremonies) assertion(ctx context.Context, ch ChallengeRecord, credentialJSON []byte, only ids.ID) (AssertionResult, error) {
	raw, s, err := c.checked(ch, credentialJSON)
	if err != nil {
		return AssertionResult{}, err
	}
	res, err := c.verifier.VerifyAssertion(raw, credentialJSON, c.lookup(ctx, s, only))
	if errors.Is(err, ErrPasskeyRejected) {
		return AssertionResult{}, refused(causeVerification)
	}
	return res, err
}

// recordUse locks the passkey of res, applies the sign counter rule (an audit entry of act with
// decision allow on a regression, the higher value kept), and writes the backup state and the
// time of use, all in the ambient unit of work. The entry names the person of the passkey.
func (c *ceremonies) recordUse(ctx context.Context, res AssertionResult, act authorize.Action, now time.Time) error {
	pk, found, err := c.passkeys.LockPasskey(ctx, res.Passkey.ID)
	if err != nil {
		return err
	}
	if !found {
		return refused(causeCredentialUnknown)
	}
	keep, regression := domain.SignCount(pk.SignCount, res.SignCount)
	if regression {
		as := ctx
		if _, ok := actor.FromContext(ctx); !ok {
			as = actor.WithActor(ctx, actor.Actor{Kind: actor.KindUser, ID: pk.UserID})
		}
		err := c.audit.Record(as, auditing.Entry{
			Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourcePasskey, ResourceID: pk.ID,
			Attrs: auditing.Attrs{
				attrPasskeyID: auditing.ID(pk.ID), attrSignCountRegression: auditing.Bool(true),
				attrStored: auditing.Int(int64(pk.SignCount)), attrReceived: auditing.Int(int64(res.SignCount)),
			},
		})
		if err != nil {
			return err
		}
	}
	return c.passkeys.UpdatePasskeyUse(ctx, pk.ID, PasskeyUse{SignCount: keep, BackupState: res.BackupState, At: now})
}

// burn sets the issued challenge id to failed in an own short unit of work after a ceremony that
// rolled back, so a challenge works once even when the completion failed after its check (03
// auth_challenges "Redemption"). A finished challenge stays unchanged; a dropped connection does not
// stop it.
func burn(ctx context.Context, uow txn.UnitOfWork, store ChallengeStore, id ids.ID, at time.Time) error {
	if id.IsZero() {
		return nil
	}
	return uow.Do(context.WithoutCancel(ctx), func(ctx context.Context) error {
		return store.FinishChallenge(ctx, id, ChallengeFailed, at)
	})
}
