// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Install implements apps.Apps (M2-24 design note 6): authorize app.install on the instance
// before anything is loaded; check manifest, reason and consent; then one unit of work declared as
// a rights change (permission_version first) locks the slug, creates the principal or reinstalls
// the existing one, writes apps and app_installations and the allow entry. No token and no grant
// is created (design note 8). A manifest with delivery.mode webhook gets its subscription in the
// same transaction (M2-26 design note 14).
func (s *Service) Install(ctx context.Context, a actor.Actor, cmd apps.InstallCmd) (apps.App, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return apps.App{}, err
	}
	if err := s.authorizeOn(ctx, a, authorize.ActionAppInstall, instanceResource); err != nil {
		return apps.App{}, err
	}
	m, err := s.checkManifest(cmd.Manifest, "/manifest")
	if err != nil {
		return apps.App{}, err
	}
	if err := s.checkReason(cmd.Reason); err != nil {
		return apps.App{}, err
	}
	g, err := domain.ConsentInstall(m, consentRequest(cmd.GrantedScopes, cmd.EventSubscriptions, cmd.EventSubscriptionsSet), s.eventScope)
	if err != nil {
		return apps.App{}, err
	}
	// The principal id comes before the unit, so the rights change names it; a reinstall names
	// the existing principal, read here without a lock and confirmed under the lock.
	id := ids.New()
	if existing, found, err := s.d.Repo.AppBySlug(ctx, m.Slug); err != nil {
		return apps.App{}, err
	} else if found {
		id = existing.PrincipalID
	}
	var out apps.App
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.install(ctx, a, id, m, cmd, g)
		return err
	}, rightsChange(authorize.ActionAppInstall, id))
	return out, err
}

// install is the work of Install inside the rights change for the principal id.
func (s *Service) install(ctx context.Context, a actor.Actor, id ids.ID, m domain.Manifest, cmd apps.InstallCmd, g domain.Grants) (apps.App, error) {
	hash := domain.Hash(cmd.Manifest)
	rec := StoredApp{
		PrincipalID: id, Slug: m.Slug, Manifest: cmd.Manifest, ManifestVersion: m.Version, ManifestSHA256: hash[:],
		Publisher: m.Publisher, HomepageURL: m.HomepageURL,
	}
	existing, reinstall, err := s.d.Repo.LockAppBySlug(ctx, m.Slug)
	if err != nil {
		return apps.App{}, err
	}
	if reinstall {
		rec, err = s.reinstall(ctx, existing, rec, m)
	} else {
		rec, err = s.create(ctx, rec, m)
	}
	if err != nil {
		return apps.App{}, err
	}
	inst, err := s.d.Repo.InsertInstallation(ctx, InstallationRecord{
		ID: ids.New(), AppID: id, GrantedScopes: g.Scopes, EventSubscriptions: g.Events, State: domain.StateActive, InstalledBy: a.ID,
	})
	if err != nil {
		return apps.App{}, err
	}
	attrs := baseAttrs(id, inst.ID)
	attrs[attrSlug] = manifestCode(m.Slug)
	attrs[attrManifestSHA256] = hexHash(hash[:])
	attrs[attrGrantedScopes] = codes(g.Scopes)
	attrs[attrEventSubscriptions] = codes(g.Events)
	attrs[attrDeliveryHosts] = manifestCodes(m.DeliveryHosts())
	attrs[attrReinstall] = auditing.Bool(reinstall)
	attrs[attrReason] = auditing.ReasonText(cmd.Reason)
	if err := s.record(ctx, authorize.ActionAppInstall, id, attrs); err != nil {
		return apps.App{}, err
	}
	if m.Delivery != nil && m.Delivery.Mode == domain.DeliveryWebhook {
		if err := s.d.Webhooks.createManifestSubscription(ctx, a, inst, m.Delivery.EventsURL); err != nil {
			return apps.App{}, err
		}
	}
	return view(rec, inst, m.Name), nil
}

// reinstall reuses the principal of existing, whose installations must all be uninstalled: the
// principal becomes active again with the new display name, and the manifest columns are replaced.
func (s *Service) reinstall(ctx context.Context, existing, rec StoredApp, m domain.Manifest) (StoredApp, error) {
	if existing.PrincipalID != rec.PrincipalID {
		return StoredApp{}, conflict("The app changed during the call; repeat it.")
	}
	if _, live, err := s.d.Repo.LockLiveInstallation(ctx, rec.PrincipalID); err != nil {
		return StoredApp{}, err
	} else if live {
		return StoredApp{}, conflict("The app is installed; use upgradeAppManifest.")
	}
	if err := s.d.Principals.SetActive(ctx, rec.PrincipalID, true); err != nil {
		return StoredApp{}, err
	}
	if err := s.d.Principals.Rename(ctx, rec.PrincipalID, m.Name); err != nil {
		return StoredApp{}, err
	}
	rec.CreatedAt = existing.CreatedAt
	return s.d.Repo.UpdateApp(ctx, rec, s.now())
}

// create creates the principal of a new app (handle = slug, display name = name) and its row; a
// handle of another principal, also one taken concurrently, is 409 conflict.
func (s *Service) create(ctx context.Context, rec StoredApp, m domain.Manifest) (StoredApp, error) {
	taken, err := s.d.Principals.HandleTaken(ctx, m.Slug)
	if err != nil {
		return StoredApp{}, err
	}
	if taken {
		return StoredApp{}, conflict("The slug is the handle of another principal.")
	}
	if err := s.d.Principals.CreateApp(ctx, rec.PrincipalID, m.Slug, m.Name); err != nil {
		return StoredApp{}, asConflict(err)
	}
	return s.d.Repo.InsertApp(ctx, rec)
}

// asConflict keeps a conflict (a handle taken concurrently) and passes every other error.
func asConflict(err error) error {
	if e, ok := errs.As(err); ok && e.Code == errs.CodeConflict {
		return conflict("The slug is the handle of another principal.")
	}
	return err
}
