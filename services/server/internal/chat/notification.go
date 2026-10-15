// SPDX-License-Identifier: Apache-2.0

package chat

import "github.com/OWNER/bothy/services/server/internal/kernel/ids"

// NotificationPath is where the target of a notification preference sits (13 section 3.1,
// M3-18): the facts notify needs to authorize a preference and to gather the level of a thread.
// Structure.NotificationPath and Structure.LockNotificationPath return it without authorizing.
type NotificationPath struct {
	// Kind is space, channel or thread.
	Kind string
	// SpaceID is zero for a DM channel and its threads.
	SpaceID ids.ID
	// ChannelID is zero for a space.
	ChannelID ids.ID
	// ThreadID is zero unless Kind is thread.
	ThreadID ids.ID
	// ChannelKind is text, forum or dm; "" for a space.
	ChannelKind string
	// DefaultNotificationLevel is channels.default_notification_level; "" when NULL or for a
	// space.
	DefaultNotificationLevel string
	// Archived reports that the space, the channel or the channel's space is archived.
	Archived bool
}

// The kinds of a notification target (03 notification_preferences.target_kind).
const (
	// TargetSpace names a space.
	TargetSpace = "space"
	// TargetChannel names a channel.
	TargetChannel = "channel"
	// TargetThread names a thread.
	TargetThread = "thread"
)
