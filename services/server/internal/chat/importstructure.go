// SPDX-License-Identifier: Apache-2.0

package chat

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// ReasonNameCollision is the reason of 409 conflict when an imported space or channel would take
// the name of a live space or channel the import did not create (22 5.1 rule 1, M8-03 design note
// 7).
const ReasonNameCollision = "name_collision"

// ReasonImportLeadControls is the reason of 409 conflict when setChannelReadOnly names the target
// channel of an import source that is neither completed nor failed: its ownership list is the only
// switch (22 12.1, M8-03 design note 4).
const ReasonImportLeadControls = "import_lead_controls"

// ImportedSpace is the space an import batch creates or adopts (M8-03 design note 7).
type ImportedSpace struct {
	// Name is the name of a new space; ignored when ExistingID is set.
	Name string
	// SortKey is the fractional index key of a new space (the tool derives it from the Discord
	// position); ignored when ExistingID is set.
	SortKey string
	// ExistingID adopts this space instead of creating one; nil creates.
	ExistingID *ids.ID
}

// ImportedChannel is the channel an import batch creates or adopts (M8-03 design note 7).
type ImportedChannel struct {
	// SpaceID is the space of a new channel; ignored when ExistingID is set.
	SpaceID ids.ID
	// Name and Topic of a new channel.
	Name, Topic string
	// Kind is text or forum.
	Kind string
	// SortKey is the fractional index key of a new channel in its space.
	SortKey string
	// ReadOnly is the read-only flag the channel gets (lead discord of the stored mapping).
	ReadOnly bool
	// ExistingID adopts this channel instead of creating one; nil creates.
	ExistingID *ids.ID
}

// ImportedTag is the forum tag an import batch creates or adopts (22 6.6).
type ImportedTag struct {
	// Name is the tag name; a live tag with the same name without regard to case is adopted.
	Name string
}

// ImportStructure is the incoming port of chat for the structure of an import (M8-03 design note
// 7): importer creates spaces, channels and forum tags only through it, inside the unit of work
// of its batch (txn), so one batch is one transaction across modules. It authorizes nothing: the
// batch was authorized with import.apply. It writes no audit entry (import.apply audits denials
// only) but appends the normal events of chat (space.created, channel.created with thread.created
// of the main thread, channel.updated). Every write method runs only inside the caller's unit of
// work; the actor in ctx is the creator. Errors are kernel/errs errors: validation_failed with
// pointers relative to the item data (/name, /topic, /sort_key, /kind), conflict with reason
// name_collision, not_found for an unknown or archived ExistingID or channel, internal outside a
// unit of work.
type ImportStructure interface {
	// Space creates a space like space.create without the permission check and without audit, or
	// verifies that ExistingID names a live space, and returns its id.
	Space(ctx context.Context, s ImportedSpace) (ids.ID, error)
	// Channel creates a channel in SpaceID: a text channel with its main thread (R-DM-11),
	// read_only as given, no grant and no operator (mode none). A name equal to a live channel of
	// the space is conflict name_collision. With ExistingID it verifies the live channel and sets
	// read_only through SetReadOnly.
	Channel(ctx context.Context, c ImportedChannel) (ids.ID, error)
	// ForumTag creates the tag in the forum channel or adopts the live tag with the same name
	// without regard to case, never a duplicate (22 6.6). A channel that is not a forum is
	// validation_failed with the code not_a_forum.
	ForumTag(ctx context.Context, channelID ids.ID, t ImportedTag) (ids.ID, error)
	// SetReadOnly writes channels.read_only, raises the channel version, appends channel.updated
	// and publishes ChannelReadOnlySet; an equal value changes nothing and publishes nothing.
	// reason is for the caller's audit entry and is not stored by chat. setChannelReadOnly and
	// setImportLead share it.
	SetReadOnly(ctx context.Context, channelID ids.ID, readOnly bool, reason string) error
	// LiveSpace reports whether id names a space that is not archived (mapping validation of
	// createImport). It reads in the ambient transaction of ctx when there is one.
	LiveSpace(ctx context.Context, id ids.ID) (bool, error)
	// LiveChannel reports whether id names a text or forum channel that is not archived and
	// returns its space.
	LiveChannel(ctx context.Context, id ids.ID) (spaceID ids.ID, live bool, err error)
}

// ChannelReadOnlySet is the in-transaction event of a change of channels.read_only (05
// channel.read_only.set, 22 12.1, 13.4): published by ImportStructure.SetReadOnly inside the unit
// of work that writes the flag. runs subscribes and requests the cancellation of every active run
// of the channel with reason channel_read_only when ReadOnly is true (M2-09 design note 7 g).
type ChannelReadOnlySet struct {
	// ChannelID is the channel; ReadOnly its new flag.
	ChannelID ids.ID
	ReadOnly  bool
}

// ReadOnlyGuard is the source port that tells chat whether the read-only flag of a channel is
// controlled by an import lead (22 12.1, M8-03 design note 4): importer implements it and
// internal/wire injects it, so chat never imports importer (02 R-AR-04).
type ReadOnlyGuard interface {
	// ImportControlled reports whether channelID is the target of a posted channel item of an
	// import source that is neither completed nor failed. It reads in the ambient transaction of
	// ctx when there is one.
	ImportControlled(ctx context.Context, channelID ids.ID) (bool, error)
}

// SetChannelReadOnlyCmd is the request of setChannelReadOnly (channel.read_only.set): the channel,
// the new flag, the mandatory reason and the If-Match of the request.
type SetChannelReadOnlyCmd struct {
	ChannelID ids.ID
	ReadOnly  bool
	Reason    string
	IfMatch   Precondition
}

// ReadOnlySwitch is the incoming port of the general owner switch of 05 rule 1.8
// (channel.read_only.set): manage on the channel, step-up, audit with the reason; switching on
// cancels the active runs of the channel through ChannelReadOnlySet. Errors: not_found, forbidden,
// step_up_required, precondition_required, version_conflict with current, validation_failed on
// /reason, conflict for an archived channel, conflict with reason import_lead_controls.
type ReadOnlySwitch interface {
	// SetChannelReadOnly sets or lifts the read-only flag; the same flag answers the channel
	// without a change.
	SetChannelReadOnly(ctx context.Context, a actor.Actor, cmd SetChannelReadOnlyCmd) (Channel, error)
}
