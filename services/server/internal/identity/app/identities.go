// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

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

// LoginMethodSource is one kind of sign-in method of a person (M1-36 extension point; the brief
// calls it CredentialSource, a name app already uses for authz.CredentialSource). Each source
// contributes its part of the credential overview and counts the methods that remain when an
// identity is unlinked. This ticket registers the sources of identities and of active devices;
// M1-39 (local password, TOTP) and M1-40 (passkeys) add theirs to IdentitiesDeps.Sources.
type LoginMethodSource interface {
	// Contribute fills the part of out this source owns for userID.
	Contribute(ctx context.Context, userID ids.ID, out *identity.CredentialOverview) error
	// Remaining counts the sign-in methods of userID this source provides once the identity
	// unlinking is unlinked. It runs after the active identities of userID are locked.
	Remaining(ctx context.Context, userID, unlinking ids.ID) (int, error)
}

// identitySource is the source of the active identities: it lists them and counts those at
// enabled providers other than the one being unlinked (08 section 3.3).
type identitySource struct{ store IdentityStore }

// Contribute implements LoginMethodSource.
func (s identitySource) Contribute(ctx context.Context, userID ids.ID, out *identity.CredentialOverview) error {
	recs, err := s.store.ActiveIdentitiesOfUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, r := range recs {
		out.Identities = append(out.Identities, r.Identity)
	}
	return nil
}

// Remaining implements LoginMethodSource.
func (s identitySource) Remaining(ctx context.Context, userID, unlinking ids.ID) (int, error) {
	return s.store.CountOtherIdentitiesAtEnabledProviders(ctx, userID, unlinking)
}

// deviceSource is the source of device pairing: an active device of the person that can approve
// a new device with step-up (08 section 5.3) keeps a way in. It contributes nothing to the
// overview, which lists no devices.
type deviceSource struct{ store IdentityStore }

// Contribute implements LoginMethodSource.
func (deviceSource) Contribute(context.Context, ids.ID, *identity.CredentialOverview) error {
	return nil
}

// Remaining implements LoginMethodSource.
func (s deviceSource) Remaining(ctx context.Context, userID, _ ids.ID) (int, error) {
	return s.store.CountPairingDevices(ctx, userID)
}

// IdentitiesDeps are the collaborators of the identity use cases (M1-36).
type IdentitiesDeps struct {
	// Providers reads the providers.
	Providers ProviderStore
	// Identities reads and writes user_identities and counts the remaining methods.
	Identities IdentityStore
	// People reads the person of a credential overview or an unlink.
	People Store
	// UoW runs every write; Resolve, Link and MarkLogin join the caller's unit.
	UoW txn.UnitOfWork
	// Authorizer decides identity.read and identity.unlink.
	Authorizer authorize.Authorizer
	// Audit writes the allow entry of identity.unlink.
	Audit auditing.Recorder
	// Auth revokes the sessions started with an unlinked identity.
	Auth identity.Auth
	// Sources are the extra sign-in method sources of later tickets; the sources of identities
	// and devices are always present.
	Sources []LoginMethodSource
	// Clock is the time of links, unlinks and session checks; nil means clock.System().
	Clock clock.Clock
}

// Identities implements identity.Identities. It is safe for concurrent use.
type Identities struct {
	d       IdentitiesDeps
	sources []LoginMethodSource
}

// Compile-time check: the incoming port.
var _ identity.Identities = (*Identities)(nil)

// NewIdentities returns the identity use cases. Errors: a missing collaborator.
func NewIdentities(d IdentitiesDeps) (*Identities, error) {
	if d.Providers == nil || d.Identities == nil || d.People == nil || d.UoW == nil || d.Authorizer == nil || d.Audit == nil || d.Auth == nil {
		return nil, errors.New("identity: NewIdentities needs every collaborator")
	}
	if d.Clock == nil {
		d.Clock = clock.System()
	}
	sources := append([]LoginMethodSource{identitySource{d.Identities}, deviceSource{d.Identities}}, d.Sources...)
	return &Identities{d: d, sources: sources}, nil
}

// Audit attribute names of identity.unlink and the provider actions.
const (
	attrProviderID      = "provider_id"
	attrProviderKind    = "provider_kind"
	attrSessionsRevoked = "sessions_revoked"
)

// unmatched is the outcome without a person.
var unmatched = identity.Resolution{Outcome: identity.ResolutionUnmatched}

// Resolve implements identity.Identities (08 section 3.1 steps 1 and 4). The provider must exist
// and be enabled; the identity is looked up by (provider_id, subject) among the active rows, and
// for oidc the issuer must equal the stored one (a multi-tenant provider has one issuer per
// tenant). EmailHint and DisplayName are never read (R-ID-02, ID-T04). The status of the person is
// not decided here: authorize and the login tickets do.
func (s *Identities) Resolve(ctx context.Context, ext identity.ExternalIdentity) (identity.Resolution, error) {
	if ext.Subject == "" {
		return unmatched, nil
	}
	p, found, err := s.d.Providers.EnabledProvider(ctx, ext.ProviderID)
	if err != nil || !found {
		return unmatched, err
	}
	rec, found, err := s.d.Identities.ActiveIdentityBySubject(ctx, p.ID, ext.Subject)
	if err != nil || !found {
		return unmatched, err
	}
	if p.Kind == identity.ProviderOIDC && rec.Issuer != ext.Issuer {
		return unmatched, nil
	}
	return identity.Resolution{Outcome: identity.ResolutionMatched, UserID: rec.UserID, IdentityID: rec.ID}, nil
}

// Link implements identity.Identities. The provider must be enabled (otherwise not_found); the
// row gets the issuer only for oidc (required there), email_at_link only when EmailHint is one
// plain address (it is a display snapshot, schema Identity format email), linked_at = now. An
// active row with the same subject is conflict; a subject unlinked before may be linked again.
func (s *Identities) Link(ctx context.Context, userID ids.ID, ext identity.ExternalIdentity) (identity.Identity, error) {
	if err := checkSubject(ext.Subject); err != nil {
		return identity.Identity{}, err
	}
	var out identity.Identity
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		p, found, err := s.d.Providers.EnabledProvider(ctx, ext.ProviderID)
		if err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		rec := identity.Identity{
			ID: ids.New(), UserID: userID, ProviderID: p.ID, ProviderKind: p.Kind, Subject: ext.Subject, LinkedAt: s.d.Clock.Now(),
		}
		if p.Kind == identity.ProviderOIDC {
			if ext.Issuer == "" {
				return errs.New(errs.CodeValidationFailed, "").
					WithFields(errs.FieldError{Pointer: "/issuer", Code: domain.FieldRequired, Detail: "An OIDC identity needs its issuer."})
			}
			rec.Issuer = ext.Issuer
		}
		if ext.EmailHint != "" && domain.CheckEmail(ext.EmailHint) == nil {
			rec.EmailAtLink = ext.EmailHint
		}
		out, err = s.d.Identities.InsertIdentity(ctx, rec)
		return err
	})
	return out, err
}

// checkSubject refuses an empty subject and one that is not valid UTF-8 or holds NUL, which
// PostgreSQL text cannot store.
func checkSubject(subject string) error {
	if subject == "" || !utf8.ValidString(subject) || strings.IndexByte(subject, 0) >= 0 {
		return errs.New(errs.CodeValidationFailed, "").
			WithFields(errs.FieldError{Pointer: "/subject", Code: domain.FieldInvalid, Detail: "The subject must be a non-empty text."})
	}
	return nil
}

// MarkLogin implements identity.Identities; an unknown id is not_found.
func (s *Identities) MarkLogin(ctx context.Context, identityID ids.ID, at time.Time) error {
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		found, err := s.d.Identities.MarkIdentityLogin(ctx, identityID, at)
		if err == nil && !found {
			return invisible()
		}
		return err
	})
}

// person authorizes act on the person userID and then answers a missing person with not_found,
// so a refused caller sees the same answer for existing and unknown ids (checkThenFound); the
// refusal not_self (another person, no owner) is not_found as well.
func (s *Identities) person(ctx context.Context, a actor.Actor, act authorize.Action, userID ids.ID) error {
	_, found, err := s.d.People.User(ctx, userID)
	if err != nil {
		return err
	}
	d, err := s.d.Authorizer.Authorize(ctx, a, act, userResource(userID))
	if err != nil {
		return err
	}
	if d.Outcome == authorize.OutcomeForbidden && d.Reason == reasonNotSelf {
		// Another person's credentials are not visible: 404 like an unknown id (05 "self oder
		// owner", brief M1-36), as for the session lists.
		return invisible()
	}
	if err := d.Err(); err != nil {
		return err
	}
	if !found {
		return invisible()
	}
	return nil
}

// Credentials implements identity.Identities: identity.read on the person (self or owner, others
// and unknown ids not_found, a pending device device_pending). Every source contributes;
// passkeys stay empty and the local flags false until M1-40 and M1-39 register their sources.
func (s *Identities) Credentials(ctx context.Context, a actor.Actor, userID ids.ID) (identity.CredentialOverview, error) {
	if err := s.person(ctx, a, authorize.ActionIdentityRead, userID); err != nil {
		return identity.CredentialOverview{}, err
	}
	out := identity.CredentialOverview{Identities: []identity.Identity{}, Passkeys: []identity.Passkey{}}
	for _, src := range s.sources {
		if err := src.Contribute(ctx, userID, &out); err != nil {
			return identity.CredentialOverview{}, err
		}
	}
	return out, nil
}

// Unlink implements identity.Identities (08 section 3.3, brief M1-36 "Unlink"): identity.unlink on
// the person (self or owner, step-up); a missing identity is not_found; an unlinked one answers
// without change. The active identities of the person are locked in ascending id before the
// remaining methods are counted, so of two parallel unlinks at most one removes the last other
// method. Zero remaining is conflict no_login_method_left. Otherwise unlinked_at = now and every
// live session started with the identity is revoked (logout for the person's own hand, admin
// otherwise), all in one unit of work with the audit entry.
func (s *Identities) Unlink(ctx context.Context, a actor.Actor, userID, identityID ids.ID) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		if err := s.person(ctx, a, authorize.ActionIdentityUnlink, userID); err != nil {
			return err
		}
		rec, found, err := s.d.Identities.IdentityOfUser(ctx, userID, identityID)
		if err != nil {
			return err
		}
		if !found {
			return invisible()
		}
		if !rec.UnlinkedAt.IsZero() {
			return s.recordUnlink(ctx, rec, auditing.Attrs{attrNoChange: auditing.Bool(true)})
		}
		if _, err := s.d.Identities.LockActiveIdentitiesOfUser(ctx, userID); err != nil {
			return err
		}
		return s.unlinkLocked(ctx, a, rec)
	})
}

// unlinkLocked is Unlink after the lock: count, unlink, end the sessions, audit.
func (s *Identities) unlinkLocked(ctx context.Context, a actor.Actor, rec IdentityRecord) error {
	remaining := 0
	for _, src := range s.sources {
		n, err := src.Remaining(ctx, rec.UserID, rec.ID)
		if err != nil {
			return err
		}
		remaining += n
	}
	if remaining == 0 {
		return errs.New(errs.CodeConflict, "No sign-in method would remain.").WithReason("no_login_method_left").
			WithHint("Link another identity or pair a device before unlinking this one.")
	}
	now := s.d.Clock.Now()
	unlinked, err := s.d.Identities.UnlinkIdentity(ctx, rec.ID, now)
	if err != nil {
		return err
	}
	if !unlinked {
		// A concurrent unlink of the same identity won the lock: answered without change.
		return s.recordUnlink(ctx, rec, auditing.Attrs{attrNoChange: auditing.Bool(true)})
	}
	sessions, err := s.d.Identities.LiveSessionsOfIdentity(ctx, rec.ID, now)
	if err != nil {
		return err
	}
	reason := identity.RevokeAdmin
	if a.ID == rec.UserID {
		reason = identity.RevokeLogout
	}
	if err := revokeAll(ctx, s.d.Auth, sessions, reason, a.ID); err != nil {
		return err
	}
	return s.recordUnlink(ctx, rec, auditing.Attrs{attrSessionsRevoked: auditing.Int(int64(len(sessions)))})
}

// recordUnlink writes the allow entry of identity.unlink with the provider and attrs.
func (s *Identities) recordUnlink(ctx context.Context, rec IdentityRecord, attrs auditing.Attrs) error {
	attrs["identity_id"] = auditing.ID(rec.ID)
	attrs[attrProviderID] = auditing.ID(rec.ProviderID)
	attrs[attrProviderKind] = auditing.Code(string(rec.ProviderKind))
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionIdentityUnlink), Decision: auditing.DecisionAllow, ResourceKind: resourceUser,
		ResourceID: rec.UserID, Attrs: attrs,
	})
}

// revokeAll revokes every session of list through Auth.RevokeSession by the principal by, in the
// ambient unit of work. A session that a concurrent revocation removed meanwhile is skipped.
func revokeAll(ctx context.Context, auth identity.Auth, list []ids.ID, reason identity.SessionRevokeReason, by ids.ID) error {
	for _, id := range list {
		err := auth.RevokeSession(ctx, id, reason, &by)
		if err != nil && errs.KindOf(err) != errs.KindNotFound {
			return err
		}
	}
	return nil
}
