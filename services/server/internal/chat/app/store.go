// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// SpaceKey is one entry of the global order: a space and its sort key.
type SpaceKey struct {
	ID      ids.ID
	SortKey string
}

// Store is the outgoing port of the chat application layer over spaces and user_space_states
// (implemented by adapters/postgres). Reads run in the ambient transaction of ctx when there is
// one and on the pool otherwise; writes and locks run only inside a unit of work of kernel/txn and
// fail with kind internal outside one. Errors are kernel/errs errors without SQL text or values.
type Store interface {
	// Space returns the space id; found is false for an unknown id.
	Space(ctx context.Context, id ids.ID) (s domain.Space, found bool, err error)
	// LockSpace returns the space id like Space and locks its row until the end of the unit.
	LockSpace(ctx context.Context, id ids.ID) (s domain.Space, found bool, err error)
	// LockActiveSpaces locks every space that is not archived in ascending id (create and move,
	// docs/DECISIONS.md M1-52).
	LockActiveSpaces(ctx context.Context) error
	// ActiveOrder returns the spaces that are not archived in the global order (sort_key, id).
	ActiveOrder(ctx context.Context) ([]SpaceKey, error)
	// VisibleSpaces returns the spaces that are not archived, all of them or those in visible, in
	// the global order (07 section 7.4 rule 1).
	VisibleSpaces(ctx context.Context, all bool, visible []ids.ID) ([]domain.Space, error)
	// SpaceIDs returns every space, archived ones included, ascending.
	SpaceIDs(ctx context.Context) ([]ids.ID, error)
	// InsertSpace inserts s. Errors: conflict on /name for a name taken among the spaces that are
	// not archived.
	InsertSpace(ctx context.Context, s domain.Space) error
	// UpdateSpace writes name, icon, color and version of s. Errors: conflict on /name.
	UpdateSpace(ctx context.Context, s domain.Space) error
	// MoveSpace writes sort key and version of s, exactly one row.
	MoveSpace(ctx context.Context, s domain.Space) error
	// ArchiveSpace writes archived_at and version of s.
	ArchiveSpace(ctx context.Context, s domain.Space) error
	// PutSidebarState inserts or replaces the row of st.UserID and st.SpaceID with version + 1;
	// keepKey keeps the stored personal key. It returns the stored row.
	PutSidebarState(ctx context.Context, st domain.SidebarState, keepKey bool) (domain.SidebarState, error)
	// SidebarStates returns the rows of user for spaces that are not archived, ascending by space.
	SidebarStates(ctx context.Context, user ids.ID) ([]domain.SidebarState, error)
}
