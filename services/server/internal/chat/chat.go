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
// space.created, space.updated, space.archived and sidebar.updated (M1-52); channel.created,
// channel.updated, channel.archived and thread.created for the main thread (M1-53); thread.updated,
// thread.closed and thread.reopened (M1-55). internal/wire lists it in moduleRegistrations.
func Register(r *authorize.Registry) {
	for _, t := range []authorize.EventType{
		authorize.EventSpaceCreated, authorize.EventSpaceUpdated, authorize.EventSpaceArchived, authorize.EventSidebarUpdated,
		authorize.EventChannelCreated, authorize.EventChannelUpdated, authorize.EventChannelArchived, authorize.EventThreadCreated,
		authorize.EventThreadUpdated, authorize.EventThreadClosed, authorize.EventThreadReopened,
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

// ChannelArchived is the in-transaction event of channel.archive (02 section 4.3): published
// inside the unit of work that archives the channel. No module subscribes in M1; notify drops the
// notification preferences of the channel in M3.
type ChannelArchived struct {
	// ChannelID is the archived channel; SpaceID its space.
	ChannelID, SpaceID ids.ID
	// ArchivedAt is the archive time.
	ArchivedAt time.Time
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
	// DefaultOperatorAgentID is the default operator; zero for none (set by SetSpaceOperator).
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

// ChannelOperator is the operator of a channel (schema ChannelOperator): Mode is inherit, none or
// agent; AgentID the own agent of mode agent and EffectiveAgentID the operator after inheritance,
// zero for absent.
type ChannelOperator struct {
	Mode                      string
	AgentID, EffectiveAgentID ids.ID
}

// Channel is a channel as the API shows it (schema Channel), never with prompt or behaviour.
// Zero values are absent members.
type Channel struct {
	ID, SpaceID ids.ID
	// Kind is text or forum (dm in V3).
	Kind string
	// Name is trimmed; Topic may be empty.
	Name, Topic string
	// SortKey is the fractional index of the order in the space.
	SortKey string
	// Operator is the operator representation.
	Operator ChannelOperator
	// ForumDefaultSort and DefaultNotificationLevel are "" for none.
	ForumDefaultSort, DefaultNotificationLevel string
	// ReadOnly is channels.read_only.
	ReadOnly bool
	// MainThreadID is the main thread of a text channel.
	MainThreadID ids.ID
	// CreatedAt is the creation time; ArchivedAt the archive time, zero while not archived.
	CreatedAt, ArchivedAt time.Time
	// Version is the ETag; SettingsVersion the counter of behaviour and prompt.
	Version, SettingsVersion int64
}

// ChannelView is a channel with the caller's effective rights on it (schema ChannelView).
type ChannelView struct {
	Channel Channel
	// Level is read, write or trigger; read on an archived channel (07 section 4.5).
	Level string
	// Capabilities are the capability names, ascending.
	Capabilities []string
}

// CreateChannelCmd is the body of createChannel (schema ChannelCreate); nil members are absent.
type CreateChannelCmd struct {
	// Kind is text or forum.
	Kind string
	Name string
	// Topic and ForumDefaultSort are optional.
	Topic, ForumDefaultSort *string
	// AfterChannelID inserts after this channel of the space; zero inserts at the end.
	AfterChannelID ids.ID
}

// UpdateChannelCmd is the body of updateChannel (schema ChannelUpdate); nil members stay
// unchanged.
type UpdateChannelCmd struct {
	Name, Topic, ForumDefaultSort, DefaultNotificationLevel *string
	// ClearDefaultNotificationLevel removes the notification default.
	ClearDefaultNotificationLevel bool
	// IfMatch is the required precondition.
	IfMatch Precondition
}

// MovePosition is the body of moveChannel and moveForumTag (schema MovePosition): at most one of
// AfterID and BeforeID; none moves to the start.
type MovePosition struct {
	AfterID, BeforeID ids.ID
}

// ForumTag is a forum tag as the API shows it (schema ForumTag); Color "" is absent.
type ForumTag struct {
	ID, ChannelID ids.ID
	Name, Color   string
	SortKey       string
	Restricted    bool
}

// CreateForumTagCmd is the body of createForumTag (schema ForumTagCreate).
type CreateForumTagCmd struct {
	Name string
	// Color is a category colour key; nil for none.
	Color *string
	// Restricted defaults to false.
	Restricted *bool
	// AfterTagID inserts after this tag; zero inserts at the end.
	AfterTagID ids.ID
}

// UpdateForumTagCmd is the body of updateForumTag (schema ForumTagUpdate); nil members stay
// unchanged.
type UpdateForumTagCmd struct {
	Name, Color *string
	// ClearColor removes the colour; not together with Color.
	ClearColor bool
	Restricted *bool
}

// OperatorAssignment is the operator a request sets (schema OperatorSet without the reason):
// Mode is inherit, none or agent; AgentID the agent of mode agent and zero when the member is
// absent.
type OperatorAssignment struct {
	Mode    string
	AgentID ids.ID
}

// SetSpaceOperatorCmd is the request of setSpaceOperator (M1-54): the space, the new default
// operator (none or agent), the reason and the If-Match of the request.
type SetSpaceOperatorCmd struct {
	SpaceID  ids.ID
	Operator OperatorAssignment
	Reason   string
	IfMatch  Precondition
}

// SetChannelOperatorCmd is the request of setChannelOperator (M1-54): the channel, the new
// operator (inherit, none or agent), the reason and the If-Match of the request.
type SetChannelOperatorCmd struct {
	ChannelID ids.ID
	Operator  OperatorAssignment
	Reason    string
	IfMatch   Precondition
}

// Structure is the incoming port of the structure, here its space, sidebar, channel, forum tag
// and operator part (M1-52, M1-53, M1-54; threads M1-55). Every method authorizes the caller a with the
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

	// ListSpaceChannels returns the channels of the space the caller may read that are not
	// archived, in their order, without pagination (space.read on the space, then channel.read
	// per item, 07 section 7.4).
	ListSpaceChannels(ctx context.Context, a actor.Actor, spaceID ids.ID) ([]ChannelView, error)
	// CreateChannel creates a text or forum channel in the space (channel.create, manage), a text
	// channel with its main thread in the same unit of work (R-DM-11). Errors: validation_failed,
	// conflict for a taken name or an archived space.
	CreateChannel(ctx context.Context, a actor.Actor, spaceID ids.ID, cmd CreateChannelCmd) (Channel, error)
	// GetChannel returns the channel with the caller's rights (channel.read); an archived channel
	// stays readable.
	GetChannel(ctx context.Context, a actor.Actor, id ids.ID) (ChannelView, error)
	// UpdateChannel changes name, topic, forum sort or notification default (channel.update,
	// manage). Errors: precondition_required, version_conflict with current, validation_failed,
	// conflict for a taken name or an archived channel.
	UpdateChannel(ctx context.Context, a actor.Actor, id ids.ID, cmd UpdateChannelCmd) (Channel, error)
	// MoveChannel changes the position in the space (channel.reorder on the space, manage) and
	// writes exactly one row. Errors: validation_failed on /after_id or /before_id, conflict for
	// an archived channel.
	MoveChannel(ctx context.Context, a actor.Actor, id ids.ID, pos MovePosition) (Channel, error)
	// ArchiveChannel archives the channel for good as a rights change (channel.archive, manage,
	// step-up) and publishes ChannelArchived; archiving again answers the channel and writes
	// nothing. Errors: validation_failed on /reason.
	ArchiveChannel(ctx context.Context, a actor.Actor, id ids.ID, reason string) (Channel, error)
	// ListForumTags returns the tags of the channel that are not deleted, in their order
	// (channel.read).
	ListForumTags(ctx context.Context, a actor.Actor, channelID ids.ID) ([]ForumTag, error)
	// CreateForumTag creates a tag in the forum (forum_tag.manage). Errors: validation_failed
	// (not_a_forum among them), conflict for a taken name or an archived channel.
	CreateForumTag(ctx context.Context, a actor.Actor, channelID ids.ID, cmd CreateForumTagCmd) (ForumTag, error)
	// UpdateForumTag changes a tag (forum_tag.manage); a tag outside the channel is not_found.
	UpdateForumTag(ctx context.Context, a actor.Actor, channelID, tagID ids.ID, cmd UpdateForumTagCmd) (ForumTag, error)
	// MoveForumTag changes the position of a tag among the tags of the forum (forum_tag.manage).
	MoveForumTag(ctx context.Context, a actor.Actor, channelID, tagID ids.ID, pos MovePosition) (ForumTag, error)
	// DeleteForumTag deletes a tag (forum_tag.manage); deleting a deleted tag succeeds without a
	// change.
	DeleteForumTag(ctx context.Context, a actor.Actor, channelID, tagID ids.ID) error

	// SetSpaceOperator sets the default operator of a space (space.operator.set, owner, step-up)
	// as a rights change checked against I1 (M1-54): the space and every inheriting channel that
	// is not archived get a new version and their event. The same assignment answers the space
	// without a change. Errors: not_found, forbidden, step_up_required, precondition_required,
	// version_conflict with current, validation_failed, conflict for an archived space,
	// i1_violation with the conflicts.
	SetSpaceOperator(ctx context.Context, a actor.Actor, cmd SetSpaceOperatorCmd) (Space, error)
	// SetChannelOperator sets the operator of a channel or lets it inherit (channel.operator.set,
	// owner, step-up) as a rights change checked against I1 (M1-54). The same assignment answers
	// the channel without a change. Errors as SetSpaceOperator, conflict for an archived channel.
	SetChannelOperator(ctx context.Context, a actor.Actor, cmd SetChannelOperatorCmd) (Channel, error)
}
