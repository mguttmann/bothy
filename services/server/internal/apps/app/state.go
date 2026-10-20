// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Suspend implements apps.Apps (M2-24 design note 9 b): active to suspended; every grant and
// token stays, the tokens stop working because the installation is not active (so do the incoming
// webhooks, whose rows stay, M2-27 design note 11 b), and the empty
// snapshot of the app writes access.changed. A suspended app is without effect; an uninstalled
// one is 409 invalid_state_transition. Deliveries stop and keep their state: the delivery job sends
// nothing for a suspended installation (M2-26). Later tickets add their parts of 11 section 4.3
// here (M2-25 commands and components).
func (s *Service) Suspend(ctx context.Context, a actor.Actor, appID ids.ID) error {
	return s.transition(ctx, a, appID, domain.TransitionSuspend, authorize.ActionAppSuspend, "")
}

// Resume implements apps.Apps (M2-24 design note 9 c): suspended to active; the heads of the
// active subscriptions become due now (ResumeInstallation, M2-26 design note 14).
func (s *Service) Resume(ctx context.Context, a actor.Actor, appID ids.ID) error {
	return s.transition(ctx, a, appID, domain.TransitionResume, authorize.ActionAppResume, "")
}

// Uninstall implements apps.Apps (M2-24 design note 9 d): active or suspended to uninstalled with
// uninstalled_at and uninstalled_by; in the same unit every active token of the app is revoked,
// every active grant with the app as subject is revoked with the reason, and the principal is
// disabled; its messages keep their author; every subscription of the installation becomes disabled
// (M2-26) and every active incoming webhook of the app is revoked (M2-27). An uninstalled app is
// without effect. Later tickets add their parts of 11 section 4.3 here (M2-25).
func (s *Service) Uninstall(ctx context.Context, a actor.Actor, appID ids.ID, reason string) error {
	return s.transition(ctx, a, appID, domain.TransitionUninstall, authorize.ActionAppUninstall, reason)
}

// transition runs t on the installation of appID: authorize act on the app first, decide the
// call without effect in a unit without rights change (audited with no_change), then the rights
// change of design note 9 e.
func (s *Service) transition(ctx context.Context, a actor.Actor, appID ids.ID, t domain.Transition, act authorize.Action, reason string) error {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return err
	}
	if err := s.authorizeOn(ctx, a, act, appResource(appID)); err != nil {
		return err
	}
	if t == domain.TransitionUninstall {
		if err := s.checkReason(reason); err != nil {
			return err
		}
	}
	tr := stateChange{appID: appID, t: t, act: act, reason: reason}
	return s.twoUnits(ctx, func(ctx context.Context) (bool, error) {
		return s.planTransition(ctx, tr)
	}, func(ctx context.Context) error {
		return s.applyTransition(ctx, a, tr)
	}, rightsChange(act, appID))
}

// stateChange is one call of suspend, resume or uninstall.
type stateChange struct {
	appID  ids.ID
	t      domain.Transition
	act    authorize.Action
	reason string
}

// attrs returns the base attributes of the call, with the reason when it has one.
func (c stateChange) attrs(inst ids.ID) auditing.Attrs {
	attrs := baseAttrs(c.appID, inst)
	if c.reason != "" {
		attrs[attrReason] = reasonText(c.reason)
	}
	return attrs
}

// planTransition reports whether the transition changes the installation; without a change it
// writes the allow entry with no_change.
func (s *Service) planTransition(ctx context.Context, c stateChange) (bool, error) {
	_, inst, err := s.load(ctx, c.appID)
	if err != nil {
		return false, err
	}
	_, change, err := domain.Next(inst.State, c.t)
	if err != nil || change {
		return change, err
	}
	return false, s.record(ctx, c.act, c.appID, noChange(c.attrs(inst.ID)))
}

// applyTransition is the work inside the rights change: lock, check again, write the state and,
// for an uninstall, revoke tokens and grants and disable the principal.
func (s *Service) applyTransition(ctx context.Context, a actor.Actor, c stateChange) error {
	if _, found, err := s.d.Repo.LockApp(ctx, c.appID); err != nil || !found {
		return orNotFound(err)
	}
	inst, live, err := s.d.Repo.LockLiveInstallation(ctx, c.appID)
	if err != nil {
		return err
	}
	if !live {
		inst.State = domain.StateUninstalled
	}
	to, change, err := domain.Next(inst.State, c.t)
	if err != nil {
		return err
	}
	if !change {
		return errRaced
	}
	if err := s.d.Repo.SetInstallationState(ctx, inst.ID, to, s.now(), a.ID); err != nil {
		return err
	}
	attrs := c.attrs(inst.ID)
	switch c.t {
	case domain.TransitionUninstall:
		if err := s.uninstallParts(ctx, a, c, inst.ID, attrs); err != nil {
			return err
		}
	case domain.TransitionResume:
		if err := s.d.Webhooks.ResumeInstallation(ctx, inst.ID); err != nil {
			return err
		}
	case domain.TransitionSuspend:
		// The delivery job sends nothing for a suspended installation; rows keep their state.
	}
	return s.record(ctx, c.act, c.appID, attrs)
}

// revokeAll revokes tokens and grants of the app and disables its principal (11 section 4.3),
// in this order after the installation row, and adds the counts to attrs.
func (s *Service) revokeAll(ctx context.Context, a actor.Actor, appID ids.ID, reason string, attrs auditing.Attrs) error {
	tokens, err := s.d.Tokens.RevokeAllForPrincipal(ctx, appID, a.ID)
	if err != nil {
		return err
	}
	grants, err := s.d.Grants.RevokeSubjectGrants(ctx, a, appID, reason)
	if err != nil {
		return err
	}
	if err := s.d.Principals.SetActive(ctx, appID, false); err != nil {
		return err
	}
	attrs[attrTokensRevoked] = auditing.Int(int64(tokens))
	attrs[attrGrantsRevoked] = auditing.Int(int64(grants))
	return nil
}

// uninstallParts is the uninstall work of 11 section 4.3 inside the rights change: revoke the
// tokens and grants, disable the subscriptions of the installation (M2-26) and revoke the active
// incoming webhooks of the app (M2-27 design note 11 a).
func (s *Service) uninstallParts(ctx context.Context, a actor.Actor, c stateChange, installation ids.ID, attrs auditing.Attrs) error {
	if err := s.revokeAll(ctx, a, c.appID, c.reason, attrs); err != nil {
		return err
	}
	if err := s.d.Webhooks.disableInstallation(ctx, installation); err != nil {
		return err
	}
	return s.d.Incoming.revokeIncomingForApp(ctx, c.appID)
}
