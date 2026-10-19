// SPDX-License-Identifier: Apache-2.0

// Package channels is the adapter of the files module over the port of chat: the space of a
// channel for the scope of file.updated (brief M3-03 design note 16), read without a permission
// check through chat.Structure.NotificationPath, which reads in the ambient transaction. It
// implements app.ChannelLocator.
//
// Example:
//
//	loc := channels.Locator{Structure: chatService}
//	space, err := loc.SpaceOf(ctx, channelID)
package channels

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Locator reads the space of a channel through the structure port of chat.
type Locator struct {
	// Structure is the chat port; only NotificationPath is used.
	Structure chat.Structure
}

// Compile-time check: Locator is the outgoing port of app.
var _ app.ChannelLocator = Locator{}

// SpaceOf implements app.ChannelLocator: the space of the channel, zero for a dm channel.
// Errors: not_found for an unknown channel, which cannot happen for the scope channel of a file
// (files.scope_channel_id references channels ON DELETE RESTRICT).
func (l Locator) SpaceOf(ctx context.Context, channelID ids.ID) (ids.ID, error) {
	p, err := l.Structure.NotificationPath(ctx, chat.TargetChannel, channelID)
	if err != nil {
		return ids.ID{}, err
	}
	return p.SpaceID, nil
}
