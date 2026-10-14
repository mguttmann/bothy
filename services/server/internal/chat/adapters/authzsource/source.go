// SPDX-License-Identifier: Apache-2.0

// Package authzsource is the adapter through which the chat module answers authz (M1-52): Source
// implements the space part of authz.StructureSource (Space, SpaceArchived, Spaces, SpaceDetails) over the table
// spaces, so authorize resolves spaces from real data (docs/spec/02-architektur.md section 4.3,
// "Quellen-Port"). The channel and thread part follows with M1-53 and M1-55; until then
// internal/wire joins this source with the empty channel part. authz never reads the table spaces.
//
// Example:
//
//	src := authzsource.New(chatpg.NewRepository(pool))
//	ok, err := src.Space(ctx, id)
package authzsource

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Source answers the space part of authz.StructureSource. It reads inside the ambient
// transaction of ctx when there is one, so the rights hook sees a space a unit of work just
// archived. It is safe for concurrent use when its store is.
type Source struct {
	store app.Store
}

// New returns the source over the spaces of store.
func New(store app.Store) *Source { return &Source{store: store} }

// Space implements authz.StructureSource: whether the space exists, archived or not.
func (s *Source) Space(ctx context.Context, id ids.ID) (bool, error) {
	_, found, err := s.store.Space(ctx, id)
	return found, err
}

// SpaceArchived implements authz.StructureSource: false for an unknown space.
func (s *Source) SpaceArchived(ctx context.Context, id ids.ID) (bool, error) {
	sp, found, err := s.store.Space(ctx, id)
	return found && sp.Archived(), err
}

// Spaces implements authz.StructureSource: every space, archived ones included, ascending by id.
func (s *Source) Spaces(ctx context.Context) ([]ids.ID, error) {
	return s.store.SpaceIDs(ctx)
}

// SpaceDetails implements authz.StructureSource: the details of the known spaces among spaceIDs,
// archived or not; an unknown id is absent (M1-49 on the spaces of M1-52, INT-W15/2).
func (s *Source) SpaceDetails(ctx context.Context, spaceIDs []ids.ID) (map[ids.ID]authz.SpaceDetail, error) {
	out := make(map[ids.ID]authz.SpaceDetail, len(spaceIDs))
	for _, id := range spaceIDs {
		sp, found, err := s.store.Space(ctx, id)
		if err != nil {
			return nil, err
		}
		if found {
			out[id] = authz.SpaceDetail{ID: sp.ID, Name: sp.Name, Icon: sp.Icon, Color: sp.Color, SortKey: sp.SortKey,
				DefaultOperatorID: sp.DefaultOperatorAgentID, ArchivedAt: sp.ArchivedAt, Version: sp.Version, CreatedAt: sp.CreatedAt}
		}
	}
	return out, nil
}
