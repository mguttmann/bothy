// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// GetMe implements identity.People: self.read on the caller's principal, for people, agents and
// apps, also from a pending device (05 section 2.5). The rights version is read first and the
// check is pinned to it, so permission_version is exactly the version the request was authorized
// with. Email, locale and time zone are reported for people only; step_up_valid_until only while
// the step-up of the session still holds.
func (s *People) GetMe(ctx context.Context, a actor.Actor) (identity.Me, error) {
	v, err := s.d.Versions.PermissionVersion(ctx)
	if err != nil {
		return identity.Me{}, errs.Wrap(errs.CodeUnavailable, err, "identity: the rights version cannot be read")
	}
	ctx = authorize.WithPermissionVersion(ctx, v)
	r := authorize.Resource{Kind: authorize.ResourcePrincipal, ID: a.ID, OwnerID: a.ID}
	if err := s.check(ctx, a, authorize.ActionSelfRead, r); err != nil {
		return identity.Me{}, err
	}
	rec, found, err := s.d.Store.Principal(ctx, a.ID)
	if err != nil {
		return identity.Me{}, err
	}
	if !found {
		return identity.Me{}, invisible()
	}
	me := identity.Me{
		Principal: principalView(rec.Principal), IsOwner: rec.IsOwner, PermissionVersion: v,
		SessionID: a.SessionID, DeviceID: a.DeviceID,
	}
	if rec.Principal.Kind == domain.KindUser {
		u, found, err := s.d.Store.User(ctx, a.ID)
		if err != nil {
			return identity.Me{}, err
		}
		if found {
			me.Email, me.Locale, me.TimeZone = u.User.Email, u.User.Locale, u.User.TimeZone
		}
	}
	if !a.StepUpAt.IsZero() && !a.SessionID.IsZero() {
		if until := a.StepUpAt.Add(s.cfg.StepUpMaxAge); until.After(s.d.Clock.Now()) {
			me.StepUpValidUntil = until
		}
	}
	return me, nil
}

// UpdateMyProfile implements identity.People: self.profile.update on the caller's person, without
// step-up and without audit (05 table 4.3). The profile is a row of users, which only people have
// (03 users), so an agent or app gets 403 subject_kind_not_allowed. A call without change emits
// no event.
func (s *People) UpdateMyProfile(ctx context.Context, a actor.Actor, cmd identity.ProfileCmd) (identity.UserView, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return identity.UserView{}, err
	}
	var out identity.UserView
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.updateMyProfile(ctx, a, cmd)
		return err
	})
	return out, err
}

// updateMyProfile is UpdateMyProfile inside its unit of work.
func (s *People) updateMyProfile(ctx context.Context, a actor.Actor, cmd identity.ProfileCmd) (identity.UserView, error) {
	if err := s.check(ctx, a, authorize.ActionSelfProfileUpdate, userResource(a.ID)); err != nil {
		return identity.UserView{}, err
	}
	if a.Kind != actor.KindUser {
		return identity.UserView{}, errs.New(errs.CodeForbidden, "").WithReason(reasonKindNotUser)
	}
	rec, found, err := s.d.Store.LockUser(ctx, a.ID)
	if err != nil {
		return identity.UserView{}, err
	}
	if !found {
		return identity.UserView{}, invisible()
	}
	p, err := s.applyProfile(rec.User, cmd.DisplayName, nil, cmd.Locale, cmd.TimeZone)
	if err != nil || len(p.changed) == 0 {
		return userView(rec), err
	}
	rec.User = p.apply(rec.User)
	if err := s.d.Store.UpdateUser(ctx, rec.User); err != nil {
		return identity.UserView{}, err
	}
	out := userView(rec)
	return out, s.emitUpdated(ctx, out)
}
