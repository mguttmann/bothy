// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// NotificationPath implements chat.Structure (M3-18): the path of a preference target without
// authorizing; not_found for an unknown id or an id of another kind.
func (s *Service) NotificationPath(ctx context.Context, kind string, id ids.ID) (chat.NotificationPath, error) {
	return s.notificationPath(ctx, kind, id, pathReads{space: s.d.Store.Space, channel: s.d.Store.Channel, thread: s.d.Store.Thread})
}

// LockNotificationPath implements chat.Structure (M3-18): NotificationPath with FOR SHARE on the
// thread, then the channel, then the space row, in that order (the order of moveThread).
func (s *Service) LockNotificationPath(ctx context.Context, kind string, id ids.ID) (chat.NotificationPath, error) {
	return s.notificationPath(ctx, kind, id, pathReads{space: s.d.Store.ShareSpace, channel: s.d.Store.ShareChannel, thread: s.d.Store.ShareThread})
}

// ChannelIDsOfSpace implements chat.Structure (M3-18) without authorizing.
func (s *Service) ChannelIDsOfSpace(ctx context.Context, spaceID ids.ID) ([]ids.ID, error) {
	return s.d.Store.ChannelIDsOfSpace(ctx, spaceID)
}

// ThreadIDsOfChannels implements chat.Structure (M3-18) without authorizing.
func (s *Service) ThreadIDsOfChannels(ctx context.Context, channelIDs []ids.ID) ([]ids.ID, error) {
	return s.d.Store.ThreadIDsOfChannels(ctx, channelIDs)
}

// pathReads are the three row reads of a path: plain or under FOR SHARE.
type pathReads struct {
	space   func(context.Context, ids.ID) (domain.Space, bool, error)
	channel func(context.Context, ids.ID) (ChannelRow, bool, error)
	thread  func(context.Context, ids.ID) (domain.Thread, bool, error)
}

// notificationPath reads the target and the rows above it, top-down from the target: thread,
// channel, space.
func (s *Service) notificationPath(ctx context.Context, kind string, id ids.ID, r pathReads) (chat.NotificationPath, error) {
	p := chat.NotificationPath{Kind: kind}
	channelID := id
	switch kind {
	case chat.TargetSpace:
		return s.pathSpace(ctx, p, id, r)
	case chat.TargetThread:
		th, found, err := r.thread(ctx, id)
		if err != nil {
			return chat.NotificationPath{}, err
		}
		if !found {
			return chat.NotificationPath{}, invisible()
		}
		p.ThreadID, channelID = th.ID, th.ChannelID
	case chat.TargetChannel:
	default:
		return chat.NotificationPath{}, invisible()
	}
	c, found, err := r.channel(ctx, channelID)
	if err != nil {
		return chat.NotificationPath{}, err
	}
	if !found {
		return chat.NotificationPath{}, invisible()
	}
	p.ChannelID, p.ChannelKind = c.Channel.ID, string(c.Channel.Kind)
	p.DefaultNotificationLevel = c.Channel.DefaultNotificationLevel
	p.Archived = c.Channel.Archived()
	if c.Channel.SpaceID.IsZero() {
		return p, nil
	}
	return s.pathSpace(ctx, p, c.Channel.SpaceID, r)
}

// pathSpace adds the space id to p, archived or not.
func (s *Service) pathSpace(ctx context.Context, p chat.NotificationPath, id ids.ID, r pathReads) (chat.NotificationPath, error) {
	sp, found, err := r.space(ctx, id)
	if err != nil {
		return chat.NotificationPath{}, err
	}
	if !found {
		return chat.NotificationPath{}, invisible()
	}
	p.SpaceID = sp.ID
	p.Archived = p.Archived || sp.Archived()
	return p, nil
}
