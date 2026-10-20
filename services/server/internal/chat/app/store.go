// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
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

// ServedTarget is one target an agent serves for I1 (10 section 5.2): a channel with its space,
// or a space alone (ChannelID zero) whose default operator is the agent.
type ServedTarget struct {
	ChannelID, SpaceID ids.ID
}

// ThreadQuery is the keyset query of listChannelThreads (brief M1-55 D6): the threads of kind
// thread of a channel, newest first.
type ThreadQuery struct {
	ChannelID ids.ID
	// State is open, closed or "" for both; TagID zero for any tag.
	State string
	TagID ids.ID
	// ByCreated sorts by (created_at DESC, id DESC), else by (last_activity_at DESC, id DESC).
	ByCreated bool
	// HasAfter starts after the position (AfterAt, AfterID) of the sort.
	HasAfter bool
	AfterAt  time.Time
	AfterID  ids.ID
	// Limit is the most rows to return.
	Limit int
}

// Store is the outgoing port of the chat application layer over spaces, user_space_states,
// channels, forum_tags, threads and thread_tags (implemented by adapters/postgres). Reads run in the ambient transaction of ctx when there is
// one and on the pool otherwise; writes and locks run only inside a unit of work of kernel/txn and
// fail with kind internal outside one. Errors are kernel/errs errors without SQL text or values.
type Store interface {
	// Space returns the space id; found is false for an unknown id.
	Space(ctx context.Context, id ids.ID) (s domain.Space, found bool, err error)
	// LockSpace returns the space id like Space and locks its row until the end of the unit.
	LockSpace(ctx context.Context, id ids.ID) (s domain.Space, found bool, err error)
	// ShareSpace returns the space id like Space and holds FOR SHARE on its row until the end of
	// the unit (M3-18; thread.move into an inheriting channel against space.operator.set, M1-62).
	ShareSpace(ctx context.Context, id ids.ID) (s domain.Space, found bool, err error)
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
	// ShareChannel returns the channel id like Channel and holds FOR SHARE on its row until the
	// end of the unit (M3-18; thread.move, M1-62: channel.operator.set and channel.archive update
	// the row, so they wait for the move or the move reads their committed state).
	ShareChannel(ctx context.Context, id ids.ID) (c ChannelRow, found bool, err error)
	// ChannelIDsOfSpace returns every channel of space, archived ones included, ascending.
	ChannelIDsOfSpace(ctx context.Context, space ids.ID) ([]ids.ID, error)
	// ThreadIDsOfChannels returns every thread of the channels, ascending.
	ThreadIDsOfChannels(ctx context.Context, channels []ids.ID) ([]ids.ID, error)
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

	// SaveSpaceOperator writes the default operator and the version of s (M1-54).
	SaveSpaceOperator(ctx context.Context, s domain.Space) error
	// SaveChannelOperator writes the operator columns and the version of c (M1-54).
	SaveChannelOperator(ctx context.Context, c domain.Channel) error
	// TouchChannel writes only the version of c: an inheriting channel whose space got a new
	// default operator (M1-54).
	TouchChannel(ctx context.Context, c domain.Channel) error
	// LockInheritingChannels locks and returns the channels of space that inherit the default
	// operator and are not archived, ascending by id.
	LockInheritingChannels(ctx context.Context, space ids.ID) ([]ChannelRow, error)
	// ServedTargets returns the targets agent serves for I1 (10 sections 5.2 and 5.3): the
	// channels that are not archived in spaces that are not archived whose effective operator is
	// the agent or that hold a thread with the agent as fixed operator, and every space that is
	// not archived with the agent as default operator (ChannelID zero); ascending by (SpaceID,
	// ChannelID) without duplicates.
	ServedTargets(ctx context.Context, agent ids.ID) ([]ServedTarget, error)

	// DeleteTagAssignments deletes every thread_tags row of tag (deleteForumTag, M1-55 D5).
	DeleteTagAssignments(ctx context.Context, tag ids.ID) error

	// Thread returns the thread id; found is false for an unknown id.
	Thread(ctx context.Context, id ids.ID) (th domain.Thread, found bool, err error)
	// LockThread returns the thread id like Thread and locks its row until the end of the unit.
	LockThread(ctx context.Context, id ids.ID) (th domain.Thread, found bool, err error)
	// ShareThread returns the thread id like Thread and holds FOR SHARE on its row until the end
	// of the unit (M1-61).
	ShareThread(ctx context.Context, id ids.ID) (th domain.Thread, found bool, err error)
	// InsertThread inserts the thread th of kind thread and returns the stored row. Errors:
	// conflict on /parent_message_id when the parent message already started a thread.
	InsertThread(ctx context.Context, th domain.Thread) (domain.Thread, error)
	// SaveThread writes title, title source, state, closed_at, closed_by and version of th.
	SaveThread(ctx context.Context, th domain.Thread) error
	// AppendToThread raises last_message_seq, message_count and version of the locked thread id,
	// sets last_activity_at to at and with reopen opens it; it returns the stored row (03 section
	// 6.2).
	AppendToThread(ctx context.Context, id ids.ID, reopen bool, at time.Time) (domain.Thread, error)
	// DecrementMessageCount lowers message_count of the locked thread id by one and raises its
	// version (M1-58). Errors: internal when no row changed (unknown thread or a count of zero).
	DecrementMessageCount(ctx context.Context, id ids.ID) error
	// ThreadCounters returns the counters of the threads in channels with last_message_seq above
	// unread_floor_seq, ascending by thread (M1-63).
	ThreadCounters(ctx context.Context, channels []ids.ID) ([]chat.ThreadCounter, error)
	// ListThreads returns the threads of q in the order of its sort.
	ListThreads(ctx context.Context, q ThreadQuery) ([]domain.Thread, error)
	// ThreadTags returns the live tags of each of threads in the order of the tags; a thread
	// without tags is absent.
	ThreadTags(ctx context.Context, threads []ids.ID) (map[ids.ID][]ids.ID, error)
	// ReplaceThreadTags replaces the tags of thread with tags, added by by at at.
	ReplaceThreadTags(ctx context.Context, thread ids.ID, tags []ids.ID, by ids.ID, at time.Time) error

	// MoveThread writes channel_id = channel, moved_at = at and version + 1 of the locked thread
	// id and returns the stored row (M1-62). Errors: internal for an unknown thread.
	MoveThread(ctx context.Context, id, channel ids.ID, at time.Time) (domain.Thread, error)
	// DeleteThreadTags deletes every thread_tags row of thread: a tag belongs to the channel of
	// its thread, so no tag survives a move (03 thread_tags, M1-62).
	DeleteThreadTags(ctx context.Context, thread ids.ID) error
}
