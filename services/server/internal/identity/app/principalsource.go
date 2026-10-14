// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// PrincipalSource implements authz.PrincipalSource over a Store, so authz learns kind, status
// and owner role of a principal without reading identity tables (03 Grundsatz 12). internal/wire
// hands it to authz (M1-28).
type PrincipalSource struct {
	store Store
}

// Compile-time check: PrincipalSource is the source port of authz.
var _ authz.PrincipalSource = (*PrincipalSource)(nil)

// NewPrincipalSource returns the source over store.
func NewPrincipalSource(store Store) *PrincipalSource {
	return &PrincipalSource{store: store}
}

// Principal implements authz.PrincipalSource: ok is false for an unknown id. IsOwner reports an
// active owners row whatever the status of the principal. Errors: those of the Store.
func (s *PrincipalSource) Principal(ctx context.Context, id ids.ID) (authz.PrincipalFacts, bool, error) {
	rec, found, err := s.store.Principal(ctx, id)
	if err != nil || !found {
		return authz.PrincipalFacts{}, false, err
	}
	p := rec.Principal
	return authz.PrincipalFacts{
		ID:      p.ID,
		Kind:    actor.Kind(p.Kind),
		Status:  authz.PrincipalStatus(p.Status),
		IsOwner: rec.IsOwner,
	}, true, nil
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
