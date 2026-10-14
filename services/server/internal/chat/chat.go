// SPDX-License-Identifier: Apache-2.0

package chat

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Register declares the durable events chat appends (M1-25, docs/spec/05-aktionen.md section 6):
// space.created, space.updated, space.archived and sidebar.updated (M1-52). internal/wire lists it
// in moduleRegistrations.
func Register(r *authorize.Registry) {
	for _, t := range []authorize.EventType{
		authorize.EventSpaceCreated, authorize.EventSpaceUpdated, authorize.EventSpaceArchived, authorize.EventSidebarUpdated,
	} {
		r.Event("chat", t)
	}
}

// SpaceArchived is the in-transaction event of space.archive (02 section 4.3): published inside
// the unit of work that archives the space. No module subscribes in M1; notify drops the
// notification preferences of the space in M3.
type SpaceArchived struct {
	// SpaceID is the archived space.
	SpaceID ids.ID
}

// Space is a space as the API shows it (schema Space). Zero values are absent members.
type Space struct {
	ID ids.ID
	// Name is the trimmed display name.
	Name string
	// Icon is a key of designkeys.SpaceIcons; Color a key of designkeys.CategoryColors.
	Icon, Color string
	// SortKey is the fractional index of the global order.
	SortKey string
	// DefaultOperatorAgentID is the default operator; zero for none (M1-54 sets it).
	DefaultOperatorAgentID ids.ID
	// CreatedAt is the creation time; ArchivedAt the archive time, zero while not archived.
	CreatedAt, ArchivedAt time.Time
	// Version is the ETag.
	Version int64
}

// SpaceView is a space with the caller's effective rights on it (schema SpaceView).
type SpaceView struct {
	Space Space
	// Level is read, write or trigger; a space visible only through a channel reports read.
	Level string
	// Capabilities are the capability names, ascending; none for a space visible only through a
	// channel.
	Capabilities []string
}

// Precondition is the If-Match header of a request (06 section 8): Present when the header was
// sent, Valid when it names one strong ETag, Version that ETag.
type Precondition struct {
	Present, Valid bool
	Version        int64
}

// CreateSpaceCmd is the body of createSpace (schema SpaceCreate); nil Icon and Color are absent.
type CreateSpaceCmd struct {
	Name        string
	Icon, Color *string
	// AfterSpaceID inserts after this space; zero inserts at the end.
	AfterSpaceID ids.ID
}

// UpdateSpaceCmd is the body of updateSpace (schema SpaceUpdate); nil fields stay unchanged, and
// icon and color cannot be cleared.
type UpdateSpaceCmd struct {
	Name, Icon, Color *string
	// IfMatch is the required precondition.
	IfMatch Precondition
}

// MoveSpaceCmd is the body of moveSpace (schema MovePosition): at most one of AfterID and
// BeforeID; none moves to the start.
type MoveSpaceCmd struct {
	AfterID, BeforeID ids.ID
}

// SidebarState is the personal sidebar state of one space (schema SidebarState).
type SidebarState struct {
	SpaceID   ids.ID
	Collapsed bool
	// SortKey is the personal order; "" uses the global order.
	SortKey string
	// Version counts the changes of the row.
	Version int64
	// UpdatedByDeviceID is the device of the last change; zero for none.
	UpdatedByDeviceID ids.ID
}

// SidebarPutCmd is the body of putMySidebarSpace (schema SidebarStatePut).
type SidebarPutCmd struct {
	Collapsed bool
	// SortKey is the new personal key; nil keeps the stored one.
	SortKey *string
	// UseGlobalOrder removes the personal key.
	UseGlobalOrder bool
}

// Structure is the incoming port of the structure, here its space and sidebar part (M1-52;
// channels M1-53, operators M1-54, threads M1-55). Every method authorizes the caller a with the
// action of 05 section 4.5 and runs its writes in one unit of work with audit and the durable
// event; a missing and an invisible space give the same 404 not_found. Errors are kernel/errs
// errors.
type Structure interface {
	// ListSpaces returns the spaces the caller may read that are not archived, in the global
	// order (sort_key, then id), without pagination (06 section 6 rule 6).
	ListSpaces(ctx context.Context, a actor.Actor) ([]SpaceView, error)
	// GetSpace returns the space id with the caller's rights; an archived space stays readable.
	GetSpace(ctx context.Context, a actor.Actor, id ids.ID) (SpaceView, error)
	// CreateSpace creates a space (space.create, owner). Errors: validation_failed, conflict on
	// /name.
	CreateSpace(ctx context.Context, a actor.Actor, cmd CreateSpaceCmd) (Space, error)
	// UpdateSpace changes name, icon or color (space.update, manage). Errors:
	// precondition_required, version_conflict with current, validation_failed, conflict for a
	// taken name or an archived space.
	UpdateSpace(ctx context.Context, a actor.Actor, id ids.ID, cmd UpdateSpaceCmd) (Space, error)
	// MoveSpace changes the global position (space.reorder, owner) and writes exactly one row.
	// Errors: validation_failed on /after_id or /before_id, conflict for an archived space.
	MoveSpace(ctx context.Context, a actor.Actor, id ids.ID, cmd MoveSpaceCmd) (Space, error)
	// ArchiveSpace archives the space for good as a rights change (space.archive, owner,
	// step-up) and publishes SpaceArchived; archiving again answers the space and writes
	// nothing. Errors: validation_failed on /reason.
	ArchiveSpace(ctx context.Context, a actor.Actor, id ids.ID, reason string) (Space, error)
	// PutSidebarSpace sets the caller's collapsed state and personal order of the space
	// (sidebar.update, people only), last write wins. Errors: validation_failed,
	// forbidden subject_kind_not_allowed.
	PutSidebarSpace(ctx context.Context, a actor.Actor, spaceID ids.ID, cmd SidebarPutCmd) (SidebarState, error)
	// ListSidebarStates returns the sidebar rows of the person a for the spaces a may read that
	// are not archived, ascending by space id (getMySidebar, M1-61).
	ListSidebarStates(ctx context.Context, a actor.Actor) ([]SidebarState, error)
}
