// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/apps"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// appsList is the list name of the cursors of listApps.
const appsList = "apps"

// appCursor is the keyset position of an app page: lower(slug) and principal id of its last row.
type appCursor struct {
	Slug string `json:"slug"`
	ID   ids.ID `json:"id"`
}

// view assembles the App of the port.
func view(a StoredApp, inst InstallationRecord, displayName string) apps.App {
	return apps.App{
		ID: a.PrincipalID, Slug: a.Slug, DisplayName: displayName, Manifest: slices.Clone(a.Manifest), ManifestVersion: a.ManifestVersion,
		Installation: apps.Installation{
			ID: inst.ID, GrantedScopes: nonNil(inst.GrantedScopes), EventSubscriptions: nonNil(inst.EventSubscriptions),
			State: string(inst.State), InstalledBy: inst.InstalledBy, InstalledAt: inst.InstalledAt,
		},
	}
}

// nonNil returns a copy of list that is never nil (06 section 2: no JSON null).
func nonNil(list []string) []string { return append([]string{}, list...) }

// load reads the app id with its latest installation; not_found for an unknown id.
func (s *Service) load(ctx context.Context, id ids.ID) (StoredApp, InstallationRecord, error) {
	a, found, err := s.d.Repo.App(ctx, id)
	if err != nil {
		return StoredApp{}, InstallationRecord{}, err
	}
	if !found {
		return StoredApp{}, InstallationRecord{}, notFound()
	}
	inst, found, err := s.d.Repo.LatestInstallation(ctx, id)
	if err != nil {
		return StoredApp{}, InstallationRecord{}, err
	}
	if !found {
		return StoredApp{}, InstallationRecord{}, errs.New(errs.CodeInternalError, "apps: an app without installation")
	}
	return a, inst, nil
}

// viewOf reads the display name of the app and assembles its view.
func (s *Service) viewOf(ctx context.Context, a StoredApp, inst InstallationRecord) (apps.App, error) {
	p, err := s.d.Principals.Principal(ctx, a.PrincipalID)
	if err != nil {
		return apps.App{}, err
	}
	return view(a, inst, p.DisplayName), nil
}

// List implements apps.Apps (M2-24 design note 13): app.read on the collection, then the apps
// with a live installation ordered by lower(slug) and id, keyset pages bound to the caller.
func (s *Service) List(ctx context.Context, a actor.Actor, cursor string, limit int) (apps.Page, error) {
	if err := s.authorizeOn(ctx, a, authorize.ActionAppRead, collection); err != nil {
		return apps.Page{}, err
	}
	binding := []byte(a.ID.String() + "\x00sort=slug")
	var after appCursor
	if cursor != "" {
		if err := s.d.Cursors.Decode(appsList, binding, cursor, &after); err != nil {
			return apps.Page{}, err
		}
		if after.ID.IsZero() || after.Slug == "" {
			return apps.Page{}, errs.New(errs.CodeCursorInvalid, "")
		}
	}
	if limit < 1 {
		limit = s.cfg.PageSizeDefault
	}
	limit = min(limit, s.cfg.PageSizeMax)
	rows, err := s.d.Repo.ListLive(ctx, after.Slug, after.ID, limit+1)
	if err != nil {
		return apps.Page{}, err
	}
	var out apps.Page
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[limit-1].App
		next, err := s.d.Cursors.Encode(appsList, binding, appCursor{Slug: strings.ToLower(last.Slug), ID: last.PrincipalID})
		if err != nil {
			return apps.Page{}, errs.Wrap(errs.CodeInternalError, err, "apps: cursor")
		}
		out.NextCursor = next
	}
	idList := make([]ids.ID, 0, len(rows))
	for _, r := range rows {
		idList = append(idList, r.App.PrincipalID)
	}
	names, err := s.d.Principals.DisplayNames(ctx, idList)
	if err != nil {
		return apps.Page{}, err
	}
	out.Items = make([]apps.App, 0, len(rows))
	for _, r := range rows {
		out.Items = append(out.Items, view(r.App, r.Installation, names[r.App.PrincipalID]))
	}
	return out, nil
}

// Get implements apps.Apps: app.read on the app first, so a non-owner gets the 404 of an unknown
// id (AZ-P4); then any app with its latest installation.
func (s *Service) Get(ctx context.Context, a actor.Actor, appID ids.ID) (apps.App, error) {
	if err := s.authorizeOn(ctx, a, authorize.ActionAppRead, appResource(appID)); err != nil {
		return apps.App{}, err
	}
	rec, inst, err := s.load(ctx, appID)
	if err != nil {
		return apps.App{}, err
	}
	return s.viewOf(ctx, rec, inst)
}

// Validate implements apps.Apps (M2-24 design note 3 c): app.read on the collection, then the
// rules; nothing is stored and no audit entry is written.
func (s *Service) Validate(ctx context.Context, a actor.Actor, manifest []byte) (apps.ManifestValidation, error) {
	if err := s.authorizeOn(ctx, a, authorize.ActionAppRead, collection); err != nil {
		return apps.ManifestValidation{}, err
	}
	m, err := domain.ParseManifest(manifest)
	if err != nil {
		return apps.ManifestValidation{}, err
	}
	fe := domain.ValidateManifest(m, manifest, s.env())
	out := apps.ManifestValidation{Valid: len(fe) == 0, Errors: make([]apps.FieldError, 0, len(fe))}
	for _, f := range fe {
		out.Errors = append(out.Errors, apps.FieldError{Pointer: f.Pointer, Code: f.Code, Detail: f.Detail})
	}
	return out, nil
}
