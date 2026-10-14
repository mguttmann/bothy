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

// upgradePlan is what the planning unit of an upgrade decided.
type upgradePlan struct {
	m    domain.Manifest
	next domain.Grants
	out  apps.App
}

// Upgrade implements apps.Apps (M2-24 design note 7): authorize app.install on the instance, then
// load the app (unknown: 404) and check the manifest (same slug, new content needs another
// version), the reason and the consent (no new scope or event: they need SetScopes). An identical
// manifest with identical lists is without effect; otherwise one rights change replaces the
// manifest, renames the principal and narrows the lists, so what the new manifest dropped acts at
// once. New delivery hosts are consented by this call and named in the audit entry.
func (s *Service) Upgrade(ctx context.Context, a actor.Actor, appID ids.ID, cmd apps.UpgradeCmd) (apps.App, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return apps.App{}, err
	}
	if err := s.authorizeOn(ctx, a, authorize.ActionAppInstall, instanceResource); err != nil {
		return apps.App{}, err
	}
	if _, found, err := s.d.Repo.App(ctx, appID); err != nil {
		return apps.App{}, err
	} else if !found {
		return apps.App{}, notFound()
	}
	m, err := s.checkManifest(cmd.Manifest, "/manifest")
	if err != nil {
		return apps.App{}, err
	}
	if err := s.checkReason(cmd.Reason); err != nil {
		return apps.App{}, err
	}
	var p upgradePlan
	err = s.twoUnits(ctx, func(ctx context.Context) (bool, error) {
		rec, inst, err := s.load(ctx, appID)
		if err != nil {
			return false, err
		}
		change, err := s.planUpgrade(ctx, rec, inst, m, cmd, &p)
		if err != nil || change {
			return change, err
		}
		attrs := noChange(baseAttrs(appID, inst.ID))
		attrs[attrReason] = reasonText(cmd.Reason)
		return false, s.record(ctx, authorize.ActionAppInstall, appID, attrs)
	}, func(ctx context.Context) error {
		return s.upgradeLocked(ctx, appID, m, cmd, &p)
	}, rightsChange(authorize.ActionAppInstall, appID))
	return p.out, err
}

// upgradeLocked is the work of Upgrade inside the rights change: lock, check again, write.
func (s *Service) upgradeLocked(ctx context.Context, appID ids.ID, m domain.Manifest, cmd apps.UpgradeCmd, p *upgradePlan) error {
	rec, inst, err := s.lockLive(ctx, appID)
	if err != nil {
		return err
	}
	change, err := s.planUpgrade(ctx, rec, inst, m, cmd, p)
	if err != nil {
		return err
	}
	if !change {
		return errRaced
	}
	return s.applyUpgrade(ctx, rec, inst, cmd, p)
}

// planUpgrade checks the upgrade against the stored app and installation and reports whether it
// changes anything; without a change p.out is the current app.
func (s *Service) planUpgrade(ctx context.Context, rec StoredApp, inst InstallationRecord, m domain.Manifest, cmd apps.UpgradeCmd, p *upgradePlan) (bool, error) {
	if inst.State == domain.StateUninstalled {
		return false, conflict("The app is uninstalled.")
	}
	if m.Slug != rec.Slug {
		return false, fieldErr("/manifest/slug", domain.CodeInvalid, "A new manifest keeps the slug of the app (11 section 4.2).")
	}
	same := domain.SameContent(rec.Manifest, cmd.Manifest)
	if !same {
		oldVersion, _ := domain.ParseSemVer(rec.ManifestVersion)
		newVersion, _ := domain.ParseSemVer(m.Version)
		if oldVersion.Compare(newVersion) == 0 {
			return false, fieldErr("/manifest/version", domain.CodeInvalid, "Changed content needs another version (11 section 4.2).")
		}
	}
	current := domain.Grants{Scopes: inst.GrantedScopes, Events: inst.EventSubscriptions}
	next, err := domain.ConsentUpgrade(m, current, consentRequest(cmd.GrantedScopes, cmd.EventSubscriptions, cmd.EventSubscriptionsSet), s.eventScope)
	if err != nil {
		return false, err
	}
	p.m, p.next = m, next
	if same && next.Equal(normalized(current)) {
		out, err := s.viewOf(ctx, rec, inst)
		p.out = out
		return false, err
	}
	return true, nil
}

// applyUpgrade writes the upgrade inside the rights change and records it.
func (s *Service) applyUpgrade(ctx context.Context, rec StoredApp, inst InstallationRecord, cmd apps.UpgradeCmd, p *upgradePlan) error {
	old, err := domain.ParseManifest(rec.Manifest)
	if err != nil {
		return err
	}
	hash := domain.Hash(cmd.Manifest)
	next := StoredApp{
		PrincipalID: rec.PrincipalID, Slug: rec.Slug, Manifest: cmd.Manifest, ManifestVersion: p.m.Version, ManifestSHA256: hash[:],
		Publisher: p.m.Publisher, HomepageURL: p.m.HomepageURL, CreatedAt: rec.CreatedAt,
	}
	updated, err := s.d.Repo.UpdateApp(ctx, next, s.now())
	if err != nil {
		return err
	}
	if err := s.d.Principals.Rename(ctx, rec.PrincipalID, p.m.Name); err != nil {
		return err
	}
	if err := s.d.Repo.UpdateInstallationGrants(ctx, inst.ID, p.next); err != nil {
		return err
	}
	attrs := upgradeAttrs(rec, inst, hash[:], domain.DiffManifests(old, p.m), p.next, cmd.Reason)
	if err := s.record(ctx, authorize.ActionAppInstall, rec.PrincipalID, attrs); err != nil {
		return err
	}
	after := inst
	after.GrantedScopes, after.EventSubscriptions = p.next.Scopes, p.next.Events
	p.out = view(updated, after, p.m.Name)
	return nil
}

// normalized returns g with sorted lists without duplicates, the form the consent rules return.
func normalized(g domain.Grants) domain.Grants {
	return domain.Grants{Scopes: sortedSet(g.Scopes), Events: sortedSet(g.Events)}
}

// orNotFound is err, or not_found when err is nil.
func orNotFound(err error) error {
	if err != nil {
		return err
	}
	return notFound()
}
