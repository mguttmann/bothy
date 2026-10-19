// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PrincipalSource implements authz.PrincipalSource over a Store, so authz learns kind, status
// and owner role of a principal without reading identity tables (03 "Principles" item 12). internal/wire
// hands it to authz (M1-28).
type PrincipalSource struct {
	store Store
	apps  identity.AppInstallations
}

// Compile-time check: PrincipalSource is the source port of authz.
var _ authz.PrincipalSource = (*PrincipalSource)(nil)

// NewPrincipalSource returns the source over store. It knows no installation of an app
// (identity.NoAppInstallations), so every app has an empty snapshot until WithApps binds the
// installations of the module apps (M2-24).
func NewPrincipalSource(store Store) *PrincipalSource {
	return &PrincipalSource{store: store, apps: identity.NoAppInstallations{}}
}

// WithApps returns a copy of s that reads the installation of app principals from apps
// (internal/wire binds apps/adapters/identitysource); nil keeps none.
func (s *PrincipalSource) WithApps(apps identity.AppInstallations) *PrincipalSource {
	c := *s
	if apps != nil {
		c.apps = apps
	}
	return &c
}

// Principal implements authz.PrincipalSource: ok is false for an unknown id. IsOwner reports an
// active owners row whatever the status of the principal; AppInstallationActive an active
// installation of an app, read on every call in the ambient transaction (M2-24 design note 11 b).
// Errors: those of the Store and of the installations.
func (s *PrincipalSource) Principal(ctx context.Context, id ids.ID) (authz.PrincipalFacts, bool, error) {
	rec, found, err := s.store.Principal(ctx, id)
	if err != nil || !found {
		return authz.PrincipalFacts{}, false, err
	}
	p := rec.Principal
	f := authz.PrincipalFacts{
		ID:      p.ID,
		Kind:    actor.Kind(p.Kind),
		Status:  authz.PrincipalStatus(p.Status),
		IsOwner: rec.IsOwner,
	}
	if f.Kind == actor.KindApp {
		_, active, err := s.apps.ActiveInstallation(ctx, id)
		if err != nil {
			return authz.PrincipalFacts{}, false, err
		}
		f.AppInstallationActive = active
	}
	return f, true, nil
}

// ActiveOwnerIDs implements authz.PrincipalSource: persons with an active owners row and
// principal status active, ascending by id. Errors: those of the Store.
func (s *PrincipalSource) ActiveOwnerIDs(ctx context.Context) ([]ids.ID, error) {
	return s.store.ActiveOwnerIDs(ctx)
}

// HasActivePasskey implements authz.PrincipalSource: a passkey of userID with revoked_at IS NULL
// (M1-40). Errors: those of the Store.
func (s *PrincipalSource) HasActivePasskey(ctx context.Context, userID ids.ID) (bool, error) {
	return s.store.HasActivePasskey(ctx, userID)
}

// HasEstablishedLocalCredential implements authz.PrincipalSource over the Store. Errors: those
// of the Store.
func (s *PrincipalSource) HasEstablishedLocalCredential(ctx context.Context, userID ids.ID) (bool, error) {
	return s.store.HasEstablishedLocalCredential(ctx, userID)
}

// Refs implements authz.PrincipalSource (M1-48): the public views (handle, display name, avatar)
// of the known principals among principals, whatever their status; unknown ids are absent.
// Errors: those of the Store.
func (s *PrincipalSource) Refs(ctx context.Context, principals []ids.ID) (map[ids.ID]authz.PrincipalRef, error) {
	out := map[ids.ID]authz.PrincipalRef{}
	if len(principals) == 0 {
		return out, nil
	}
	rows, err := s.store.Principals(ctx, principals)
	if err != nil {
		return nil, err
	}
	for _, p := range rows {
		out[p.ID] = authz.PrincipalRef{
			ID: p.ID, Kind: actor.Kind(p.Kind), Handle: p.Handle.String(), DisplayName: p.DisplayName.String(), AvatarFileID: p.AvatarFileID,
		}
	}
	return out, nil
}

// Profiles implements authz.PrincipalSource (M1-49): the public views and statuses of the known
// principals among principals, whatever their status, with users.locale and users.time_zone for
// people; unknown ids are absent. Errors: those of the Store.
func (s *PrincipalSource) Profiles(ctx context.Context, principals []ids.ID) (map[ids.ID]authz.PrincipalProfile, error) {
	out := map[ids.ID]authz.PrincipalProfile{}
	if len(principals) == 0 {
		return out, nil
	}
	rows, err := s.store.Principals(ctx, principals)
	if err != nil {
		return nil, err
	}
	for _, p := range rows {
		prof := authz.PrincipalProfile{
			PrincipalRef: authz.PrincipalRef{
				ID: p.ID, Kind: actor.Kind(p.Kind), Handle: p.Handle.String(), DisplayName: p.DisplayName.String(), AvatarFileID: p.AvatarFileID,
			},
			Status: authz.PrincipalStatus(p.Status),
		}
		if p.Kind == domain.KindUser {
			u, found, err := s.store.User(ctx, p.ID)
			if err != nil {
				return nil, err
			}
			if found {
				prof.Locale, prof.TimeZone = u.User.Locale, u.User.TimeZone
			}
		}
		out[p.ID] = prof
	}
	return out, nil
}

// OwnerIDs implements authz.PrincipalSource (M1-49): the persons with an owners row that is not
// revoked, whatever their principal status, ascending by id. Errors: those of the Store.
func (s *PrincipalSource) OwnerIDs(ctx context.Context) ([]ids.ID, error) {
	rows, err := s.store.ActiveOwners(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ids.ID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Owner.UserID)
	}
	slices.SortFunc(out, func(a, b ids.ID) int { return slices.Compare(a[:], b[:]) })
	return slices.Compact(out), nil
}
