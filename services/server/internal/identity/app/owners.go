// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// ListOwners implements identity.Owners: user.read on the directory (directoryResource), the
// active owners by granted_at, unpaginated (06 section 6 point 6).
func (s *People) ListOwners(ctx context.Context, a actor.Actor) ([]identity.OwnerView, error) {
	if err := s.check(ctx, a, authorize.ActionUserRead, directoryResource(a)); err != nil {
		return nil, err
	}
	return NewDirectory(s.d.Store).ActiveOwners(ctx)
}

// ownerChange declares the rights change act of the owner role of userID.
func ownerChange(act authorize.Action, userID ids.ID) txn.Option {
	return txn.DeclareRightsChange(txn.RightsChangeSpec{
		Action: string(act), Resource: txn.Ref{Kind: resourceUser, ID: userID}, Principals: []ids.ID{userID},
	})
}

// AddOwner implements identity.Owners. A first unit loads the person, authorizes owner.add once
// for the request (R-RE-02) and checks reason and state, so a refused call never raises the rights
// version; a second unit declared as the rights change locks the active owners rows (ascending
// id) and the person, checks the state again and inserts the owners row. The rights hook then
// verifies I1 and SR8: a personal agent in the instance gives 409 i1_violation, because an owner
// has trigger everywhere (07 SR1, AZ-M23).
func (s *People) AddOwner(ctx context.Context, a actor.Actor, userID ids.ID, reason string) (identity.OwnerView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.OwnerView{}, err
	}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.addOwnerCheck(ctx, a, userID, reason) })
	if err != nil {
		return identity.OwnerView{}, err
	}
	var out identity.OwnerView
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.addOwnerWrite(ctx, a, userID, reason)
		return err
	}, ownerChange(authorize.ActionOwnerAdd, userID))
	return out, err
}

// addOwnerCheck is the first unit of AddOwner.
func (s *People) addOwnerCheck(ctx context.Context, a actor.Actor, userID ids.ID, reason string) error {
	rec, found, err := s.d.Store.User(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.checkThenFound(ctx, a, authorize.ActionOwnerAdd, userID, found); err != nil {
		return err
	}
	if err := domain.CheckReason(reason); err != nil {
		return err
	}
	return ownerCandidate(rec)
}

// addOwnerWrite is the second unit of AddOwner, declared as the rights change.
func (s *People) addOwnerWrite(ctx context.Context, a actor.Actor, userID ids.ID, reason string) (identity.OwnerView, error) {
	if _, err := s.d.Store.LockActiveOwners(ctx); err != nil {
		return identity.OwnerView{}, err
	}
	rec, found, err := s.d.Store.LockUser(ctx, userID)
	if err != nil {
		return identity.OwnerView{}, err
	}
	if !found {
		return identity.OwnerView{}, invisible()
	}
	if err := ownerCandidate(rec); err != nil {
		return identity.OwnerView{}, err
	}
	o, err := domain.NewOwner(ids.New(), userID, a.ID, reason)
	if err != nil {
		return identity.OwnerView{}, err
	}
	if err := s.d.Store.InsertOwner(ctx, &o); err != nil {
		return identity.OwnerView{}, err
	}
	rec.IsOwner = true
	out := identity.OwnerView{User: principalView(rec.User.Principal), GrantedBy: o.GrantedBy, GrantedAt: o.GrantedAt, Reason: o.Reason}
	if err := s.record(ctx, authorize.ActionOwnerAdd, userID, auditing.Attrs{attrReason: auditing.ReasonText(reason)}); err != nil {
		return identity.OwnerView{}, err
	}
	return out, s.emitUpdated(ctx, userView(rec))
}

// ownerCandidate refuses a person that is not active or already an owner: an owner must be able
// to act at once (docs/DECISIONS.md M1-31).
func ownerCandidate(rec UserRecord) error {
	switch {
	case rec.IsOwner:
		return errs.New(errs.CodeConflict, "The person is already an owner.").WithReason("already_owner")
	case rec.User.Principal.Status != domain.StatusActive:
		return errs.New(errs.CodeConflict, "Only an active person can become an owner.").WithReason("principal_not_active")
	}
	return nil
}

// RevokeOwner implements identity.Owners. A person without an active owners row is not_found,
// because the owner role is the resource (docs/DECISIONS.md M1-31). As in AddOwner the first unit
// authorizes and the second, declared as the rights change, revokes under the locks; the rights
// hook answers 409 last_owner when no active owner remains, also when owners remove themselves.
func (s *People) RevokeOwner(ctx context.Context, a actor.Actor, userID ids.ID, reason string) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	if err := s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.revokeOwnerCheck(ctx, a, userID, reason) }); err != nil {
		return err
	}
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		return s.revokeOwnerWrite(ctx, a, userID, reason)
	}, ownerChange(authorize.ActionOwnerRemove, userID))
}

// revokeOwnerCheck is the first unit of RevokeOwner.
func (s *People) revokeOwnerCheck(ctx context.Context, a actor.Actor, userID ids.ID, reason string) error {
	_, found, err := s.d.Store.ActiveOwner(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.checkThenFound(ctx, a, authorize.ActionOwnerRemove, userID, found); err != nil {
		return err
	}
	return domain.CheckReason(reason)
}

// revokeOwnerWrite is the second unit of RevokeOwner, declared as the rights change.
func (s *People) revokeOwnerWrite(ctx context.Context, a actor.Actor, userID ids.ID, reason string) error {
	if _, err := s.d.Store.LockActiveOwners(ctx); err != nil {
		return err
	}
	revoked, err := s.d.Store.RevokeOwner(ctx, userID, a.ID)
	if err != nil {
		return err
	}
	if !revoked {
		return invisible()
	}
	rec, found, err := s.d.Store.User(ctx, userID)
	if err != nil {
		return err
	}
	if !found {
		return errs.New(errs.CodeInternalError, "identity: an owner without a person")
	}
	if err := s.record(ctx, authorize.ActionOwnerRemove, userID, auditing.Attrs{attrReason: auditing.ReasonText(reason)}); err != nil {
		return err
	}
	return s.emitUpdated(ctx, userView(rec))
}
