// SPDX-License-Identifier: Apache-2.0

package appdispatch

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/apps/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Installation is what the dispatcher needs of an active installation: its id and granted scopes
// for the actor of a reply, and the manifest with the interaction address.
type Installation struct {
	// ID is app_installations.id.
	ID ids.ID
	// GrantedScopes are app_installations.granted_scopes.
	GrantedScopes []string
	// Manifest is the current manifest of the app.
	Manifest domain.Manifest
}

// Installations reads the active installation of an app with its manifest. The dispatcher reads
// it for every request, so a suspend, an uninstall, a scope change or a manifest upgrade takes
// effect at the next request.
type Installations interface {
	// ActiveInstallation returns the installation of appID in state active; found is false when the
	// app has none (not installed, suspended or uninstalled). Errors: faults of the database, and
	// a stored manifest that does not decode (kind internal).
	ActiveInstallation(ctx context.Context, appID ids.ID) (Installation, bool, error)
}

// Repository is the part of the apps repository (adapters/postgres.Repository) that
// NewInstallations reads.
type Repository interface {
	// ActiveInstallation returns the active installation of appID.
	ActiveInstallation(ctx context.Context, appID ids.ID) (app.InstallationRecord, bool, error)
	// App returns the app with its stored manifest.
	App(ctx context.Context, id ids.ID) (app.StoredApp, bool, error)
}

// NewInstallations returns the Installations of the dispatcher over the apps repository r.
//
// Example:
//
//	inst := appdispatch.NewInstallations(appspg.NewRepository(pool))
func NewInstallations(r Repository) Installations { return repoInstallations{r: r} }

// repoInstallations implements Installations over a Repository.
type repoInstallations struct{ r Repository }

// ActiveInstallation implements Installations: the active installation, then the app row with the
// manifest. An active installation without its app row is a broken invariant (internal).
func (i repoInstallations) ActiveInstallation(ctx context.Context, appID ids.ID) (Installation, bool, error) {
	rec, found, err := i.r.ActiveInstallation(ctx, appID)
	if err != nil || !found {
		return Installation{}, false, err
	}
	a, found, err := i.r.App(ctx, appID)
	if err != nil {
		return Installation{}, false, err
	}
	if !found {
		return Installation{}, false, errs.New(errs.CodeInternalError, "apps: an active installation has no app row")
	}
	m, err := domain.ParseManifest(a.Manifest)
	if err != nil {
		return Installation{}, false, errs.Wrap(errs.CodeInternalError, err, "apps: the stored manifest does not decode")
	}
	return Installation{ID: rec.ID, GrantedScopes: slices.Clone(rec.GrantedScopes), Manifest: m}, true, nil
}
