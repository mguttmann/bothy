// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/authz"
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
//
// With resolve (the guided flow, M2-21) the first unit also validates it with the reason, and the
// second unit lets authz switch the listed agents after the checks under the locks and before the
// owners row is inserted, so a second owner becomes possible while a personal agent exists
// (OQ-01, L22: I1 keeps no owner exception).
func (s *People) AddOwner(ctx context.Context, a actor.Actor, userID ids.ID, reason string, resolve *identity.ResolveI1) (identity.OwnerView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.OwnerView{}, err
	}
	req := resolveRequest(resolve)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error { return s.addOwnerCheck(ctx, a, userID, reason, req) })
	if err != nil {
		return identity.OwnerView{}, err
	}
	var out identity.OwnerView
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.addOwnerWrite(ctx, a, userID, reason, req)
		return err
	}, ownerChange(authorize.ActionOwnerAdd, userID))
	return out, err
}

// resolveRequest converts the guided flow of the port into the request of authz; nil stays nil.
func resolveRequest(r *identity.ResolveI1) *authz.ResolveI1 {
	if r == nil {
		return nil
	}
	return &authz.ResolveI1{AgentIDs: slices.Clone(r.AgentIDs), Reason: r.Reason}
}

// errNoResolver is the refusal of a guided flow without a bound resolver (fail closed).
func errNoResolver() error {
	return errs.New(errs.CodeInternalError, "identity: the guided flow needs the resolver of authz")
}

// addOwnerCheck is the first unit of AddOwner. The field errors of the reason and of resolve_i1
// come in one answer (06 section 5 point 3).
func (s *People) addOwnerCheck(ctx context.Context, a actor.Actor, userID ids.ID, reason string, req *authz.ResolveI1) error {
	rec, found, err := s.d.Store.User(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.checkThenFound(ctx, a, authorize.ActionOwnerAdd, userID, found); err != nil {
		return err
	}
	var resolveErr error
	if req != nil {
		if s.d.Resolver == nil {
			return errNoResolver()
		}
		resolveErr = s.d.Resolver.CheckResolveI1(*req)
	}
	if err := joinFieldErrors(domain.CheckReason(reason), resolveErr); err != nil {
		return err
	}
	return ownerCandidate(rec)
}

// joinFieldErrors returns the validation errors of list as one validation_failed with all their
// field errors, or the first error of another code; nil entries are skipped.
func joinFieldErrors(list ...error) error {
	var out *errs.Error
	for _, err := range list {
		if err == nil {
			continue
		}
		e, ok := errs.As(err)
		if !ok || e.Code != errs.CodeValidationFailed {
			return err
		}
		if out == nil {
			out = errs.New(errs.CodeValidationFailed, e.Detail)
		}
		out = out.WithFields(e.Fields...)
	}
	if out == nil {
		return nil
	}
	return out
}

// addOwnerWrite is the second unit of AddOwner, declared as the rights change.
func (s *People) addOwnerWrite(ctx context.Context, a actor.Actor, userID ids.ID, reason string, req *authz.ResolveI1) (identity.OwnerView, error) {
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
	if req != nil {
		// After the checks under the locks, before the first write (M2-21 design note 3 e).
		if s.d.Resolver == nil {
			return identity.OwnerView{}, errNoResolver()
		}
		if err := s.d.Resolver.ResolveI1(ctx, a, *req); err != nil {
			return identity.OwnerView{}, err
		}
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
