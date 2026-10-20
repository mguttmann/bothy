// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SetScopes implements apps.Apps (M2-24 design note 9 a): authorize app.scopes.update on the app
// first (a non-owner gets the 404 of an unknown id), then the reason; equal lists are without
// effect, decided before the rights change; otherwise one rights change replaces the lists.
// Scopes not granted before are the new consent; a removed scope acts on the next request of
// every token (the scope check needs it in the installation and in the token).
func (s *Service) SetScopes(ctx context.Context, a actor.Actor, appID ids.ID, cmd apps.SetScopesCmd) (apps.App, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return apps.App{}, err
	}
	if err := s.authorizeOn(ctx, a, authorize.ActionAppScopesUpdate, appResource(appID)); err != nil {
		return apps.App{}, err
	}
	if err := s.checkReason(cmd.Reason); err != nil {
		return apps.App{}, err
	}
	req := consentRequest(cmd.GrantedScopes, cmd.EventSubscriptions, cmd.EventSubscriptionsSet)
	var out apps.App
	err = s.twoUnits(ctx, func(ctx context.Context) (bool, error) {
		rec, inst, err := s.load(ctx, appID)
		if err != nil {
			return false, err
		}
		_, change, err := s.planScopes(rec, inst, req)
		if err != nil || change {
			return change, err
		}
		attrs := noChange(baseAttrs(appID, inst.ID))
		attrs[attrReason] = reasonText(cmd.Reason)
		if err := s.record(ctx, authorize.ActionAppScopesUpdate, appID, attrs); err != nil {
			return false, err
		}
		out, err = s.viewOf(ctx, rec, inst)
		return false, err
	}, func(ctx context.Context) error {
		var err error
		out, err = s.applyScopes(ctx, appID, req, cmd.Reason)
		return err
	}, rightsChange(authorize.ActionAppScopesUpdate, appID))
	return out, err
}

// planScopes returns the consented lists of req for the app and whether they change the
// installation; an uninstalled app is 409 conflict.
func (s *Service) planScopes(rec StoredApp, inst InstallationRecord, req domain.ConsentRequest) (domain.Grants, bool, error) {
	if inst.State == domain.StateUninstalled {
		return domain.Grants{}, false, conflict("The app is uninstalled.")
	}
	m, err := domain.ParseManifest(rec.Manifest)
	if err != nil {
		return domain.Grants{}, false, err
	}
	current := domain.Grants{Scopes: inst.GrantedScopes, Events: inst.EventSubscriptions}
	next, err := domain.ConsentSetScopes(m, current, req, s.eventScope)
	if err != nil {
		return domain.Grants{}, false, err
	}
	return next, !next.Equal(normalized(current)), nil
}

// applyScopes is the work of SetScopes inside the rights change: lock, check again, write the
// lists and the allow entry with the difference.
func (s *Service) applyScopes(ctx context.Context, appID ids.ID, req domain.ConsentRequest, reason string) (apps.App, error) {
	rec, inst, err := s.lockLive(ctx, appID)
	if err != nil {
		return apps.App{}, err
	}
	next, change, err := s.planScopes(rec, inst, req)
	if err != nil {
		return apps.App{}, err
	}
	if !change {
		return apps.App{}, errRaced
	}
	if err := s.d.Repo.UpdateInstallationGrants(ctx, inst.ID, next); err != nil {
		return apps.App{}, err
	}
	scopesAdded, scopesRemoved := domain.Delta(inst.GrantedScopes, next.Scopes)
	eventsAdded, eventsRemoved := domain.Delta(inst.EventSubscriptions, next.Events)
	attrs := baseAttrs(appID, inst.ID)
	attrs[attrGrantedScopesAdded] = codes(scopesAdded)
	attrs[attrGrantedScopesRemoved] = codes(scopesRemoved)
	attrs[attrEventSubscriptionsAdded] = codes(eventsAdded)
	attrs[attrEventSubscriptionsRemoved] = codes(eventsRemoved)
	attrs[attrReason] = reasonText(reason)
	if err := s.record(ctx, authorize.ActionAppScopesUpdate, appID, attrs); err != nil {
		return apps.App{}, err
	}
	inst.GrantedScopes, inst.EventSubscriptions = next.Scopes, next.Events
	return s.viewOf(ctx, rec, inst)
}

// lockLive locks the app and its live installation FOR UPDATE in the lock order of 03 6.3 (apps,
// then app_installations); an unknown app is 404, an uninstalled one 409 conflict.
func (s *Service) lockLive(ctx context.Context, appID ids.ID) (StoredApp, InstallationRecord, error) {
	rec, found, err := s.d.Repo.LockApp(ctx, appID)
	if err != nil || !found {
		return StoredApp{}, InstallationRecord{}, orNotFound(err)
	}
	inst, live, err := s.d.Repo.LockLiveInstallation(ctx, appID)
	if err != nil {
		return StoredApp{}, InstallationRecord{}, err
	}
	if !live {
		return StoredApp{}, InstallationRecord{}, conflict("The app is uninstalled.")
	}
	return rec, inst, nil
}
