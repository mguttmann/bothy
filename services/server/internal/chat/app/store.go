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

// ChannelRow is a channel with what its representation and authorize need from its space and its
// main thread: the default operator of the space and whether the space is archived.
type ChannelRow struct {
	Channel domain.Channel
	// SpaceDefaultOperatorID is spaces.default_operator_agent_id; zero for none.
	SpaceDefaultOperatorID ids.ID
	// SpaceArchived reports an archived space.
	SpaceArchived bool
}

// Store is the outgoing port of the chat application layer over spaces, user_space_states,
// channels, forum_tags and the main threads (implemented by adapters/postgres). Reads run in the ambient transaction of ctx when there is
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

	// Channel returns the channel id with its space facts; found is false for an unknown id.
	Channel(ctx context.Context, id ids.ID) (c ChannelRow, found bool, err error)
	// LockChannel returns the channel id like Channel and locks its row until the end of the unit.
	LockChannel(ctx context.Context, id ids.ID) (c ChannelRow, found bool, err error)
	// ChannelOrder returns the channels of space that are not archived in their order (sort_key,
	// id).
	ChannelOrder(ctx context.Context, space ids.ID) ([]domain.Keyed, error)
	// VisibleChannels returns the channels of space that are not archived and match the scope:
	// all when instanceWide or space is among fullyVisible, else those in visible; in their order
	// (07 section 7.4 rule 1).
	VisibleChannels(ctx context.Context, space ids.ID, instanceWide bool, visible, fullyVisible []ids.ID) ([]ChannelRow, error)
	// AllChannels returns every channel that is not a dm channel, archived ones included,
	// ascending by id.
	AllChannels(ctx context.Context) ([]ChannelRow, error)
	// InsertChannel inserts c. Errors: conflict on /name for a name taken in the space.
	InsertChannel(ctx context.Context, c domain.Channel) error
	// SaveChannel writes name, topic, forum sort, notification default, sort key, archive time
	// and version of c, exactly one row. Errors: conflict on /name.
	SaveChannel(ctx context.Context, c domain.Channel) error
	// InsertMainThread inserts the main thread th and returns the stored row.
	InsertMainThread(ctx context.Context, th domain.Thread) (domain.Thread, error)
	// ForumTags returns the tags of channel that are not deleted, in their order.
	ForumTags(ctx context.Context, channel ids.ID) ([]domain.ForumTag, error)
	// ForumTag returns the tag id, deleted or not; found is false for an unknown id.
	ForumTag(ctx context.Context, id ids.ID) (t domain.ForumTag, found bool, err error)
	// InsertForumTag inserts t. Errors: conflict on /name for a name taken in the forum.
	InsertForumTag(ctx context.Context, t domain.ForumTag) error
	// SaveForumTag writes name, color, sort key, restricted and deletion time of t. Errors:
	// conflict on /name.
	SaveForumTag(ctx context.Context, t domain.ForumTag) error
}
