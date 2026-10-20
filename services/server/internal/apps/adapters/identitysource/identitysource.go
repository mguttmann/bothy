// SPDX-License-Identifier: Apache-2.0

// Package identitysource implements the port identity.AppInstallations over the repository of the
// module apps (M2-24 design note 12), so identity reads the installation of an app principal
// without importing apps; internal/wire binds it.
package identitysource

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/apps/app"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Installations is the subset of app.Repository the source reads.
type Installations interface {
	ActiveInstallation(ctx context.Context, appID ids.ID) (app.InstallationRecord, bool, error)
	ShareActiveInstallation(ctx context.Context, appID ids.ID) (app.InstallationRecord, bool, error)
}

// Source implements identity.AppInstallations. It reads in the ambient transaction of ctx and
// caches nothing (AZ-P9). It is safe for concurrent use when the repository is.
type Source struct {
	repo Installations
}

// Compile-time check: Source is the port of identity.
var _ identity.AppInstallations = Source{}

// New returns the source over repo (apps/adapters/postgres.Repository).
func New(repo Installations) Source { return Source{repo: repo} }

// ActiveInstallation implements identity.AppInstallations.
func (s Source) ActiveInstallation(ctx context.Context, appID ids.ID) (identity.ActiveAppInstallation, bool, error) {
	r, found, err := s.repo.ActiveInstallation(ctx, appID)
	return active(r, found, err)
}

// LockActiveInstallation implements identity.AppInstallations: FOR SHARE in the ambient unit of
// work; outside one the repository fails with internal.
func (s Source) LockActiveInstallation(ctx context.Context, appID ids.ID) (identity.ActiveAppInstallation, bool, error) {
	r, found, err := s.repo.ShareActiveInstallation(ctx, appID)
	return active(r, found, err)
}

// active converts a record of an active installation.
func active(r app.InstallationRecord, found bool, err error) (identity.ActiveAppInstallation, bool, error) {
	if err != nil || !found {
		return identity.ActiveAppInstallation{}, false, err
	}
	return identity.ActiveAppInstallation{ID: r.ID, GrantedScopes: slices.Clone(r.GrantedScopes)}, true, nil
}
