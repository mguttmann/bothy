// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/identity/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Directory implements identity.Directory over a Store. It is read only and does not call
// authorize: callers authorize their own actions. It is safe for concurrent use when the
// Store is.
type Directory struct {
	store Store
}

// Compile-time check: Directory is the port identity.Directory.
var _ identity.Directory = (*Directory)(nil)

// NewDirectory returns the directory over store.
func NewDirectory(store Store) *Directory {
	return &Directory{store: store}
}

// Principal implements identity.Directory. Errors: kind not_found for an unknown id; the
// errors of the Store.
func (d *Directory) Principal(ctx context.Context, id ids.ID) (identity.PrincipalView, error) {
	rec, found, err := d.store.Principal(ctx, id)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	if !found {
		return identity.PrincipalView{}, notFound("principal")
	}
	return principalView(rec.Principal), nil
}

// Principals implements identity.Directory: unknown ids are absent from the map, and an empty
// list reads nothing.
func (d *Directory) Principals(ctx context.Context, list []ids.ID) (map[ids.ID]identity.PrincipalView, error) {
	out := map[ids.ID]identity.PrincipalView{}
	if len(list) == 0 {
		return out, nil
	}
	ps, err := d.store.Principals(ctx, list)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		out[p.ID] = principalView(p)
	}
	return out, nil
}

// PrincipalByHandle implements identity.Directory: the match ignores case. Errors: kind
// not_found when no principal has the handle; the errors of the Store.
func (d *Directory) PrincipalByHandle(ctx context.Context, handle string) (identity.PrincipalView, error) {
	p, found, err := d.store.PrincipalByHandle(ctx, handle)
	if err != nil {
		return identity.PrincipalView{}, err
	}
	if !found {
		return identity.PrincipalView{}, notFound("principal")
	}
	return principalView(p), nil
}

// User implements identity.Directory. Errors: kind not_found for an unknown id and for a
// principal that is not a person; the errors of the Store.
func (d *Directory) User(ctx context.Context, id ids.ID) (identity.UserView, error) {
	rec, found, err := d.store.User(ctx, id)
	if err != nil {
		return identity.UserView{}, err
	}
	if !found {
		return identity.UserView{}, notFound("person")
	}
	u := rec.User
	return identity.UserView{
		PrincipalView: principalView(u.Principal),
		Email:         u.Email,
		Locale:        u.Locale,
		TimeZone:      u.TimeZone,
		IsOwner:       rec.IsOwner,
	}, nil
}

// ActiveOwners implements identity.Directory: owners rows with revoked_at IS NULL, ordered by
// granted_at, then id. Errors: those of the Store.
func (d *Directory) ActiveOwners(ctx context.Context) ([]identity.OwnerView, error) {
	recs, err := d.store.ActiveOwners(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]identity.OwnerView, 0, len(recs))
	for _, r := range recs {
		out = append(out, identity.OwnerView{
			User:      principalView(r.User),
			GrantedBy: r.Owner.GrantedBy,
			GrantedAt: r.Owner.GrantedAt,
			Reason:    r.Owner.Reason,
		})
	}
	return out, nil
}

// principalView converts a principal into the port DTO.
func principalView(p domain.Principal) identity.PrincipalView {
	return identity.PrincipalView{
		ID:           p.ID,
		Kind:         actor.Kind(p.Kind),
		Status:       identity.Status(p.Status),
		Handle:       p.Handle.String(),
		DisplayName:  p.DisplayName.String(),
		AvatarFileID: p.AvatarFileID,
		CreatedAt:    p.CreatedAt,
	}
}

// notFound is the not_found error of the directory; it names the kind of object, never the id
// or handle the caller asked for.
func notFound(what string) *errs.Error {
	return errs.New(errs.CodeNotFound, what+" not found")
}
