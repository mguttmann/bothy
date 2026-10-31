// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// ListSidebar implements sync.Settings (brief M1-61 note 18): settings.read on the caller
// (R-AZ-21), then the rows of user_space_states from chat, which keeps only spaces that are not
// archived and that the caller may read (space.read in filter mode).
func (s Settings) ListSidebar(ctx context.Context) ([]sync.SidebarState, error) {
	a := caller(ctx)
	if err := s.check(ctx, a, authorize.ActionSettingsRead, self(a.ID)); err != nil {
		return nil, err
	}
	rows, err := s.d.Channels.ListSidebarStates(ctx, a)
	if err != nil {
		return nil, err
	}
	out := make([]sync.SidebarState, 0, len(rows))
	for _, r := range rows {
		out = append(out, sync.SidebarState{SpaceID: r.SpaceID, Collapsed: r.Collapsed, SortKey: r.SortKey, ChannelOrder: r.ChannelOrder,
			Version: r.Version, UpdatedByDeviceID: r.UpdatedByDeviceID})
	}
	return out, nil
}

// ListSidebarChannels implements sync.Settings (M4-51): settings.read on the caller, then the rows
// of user_channel_states from chat, which keeps only channels that are not archived, lie in a
// space that is not archived and that the caller may read (channel.read in filter mode).
func (s Settings) ListSidebarChannels(ctx context.Context) ([]sync.SidebarChannelState, error) {
	a := caller(ctx)
	if err := s.check(ctx, a, authorize.ActionSettingsRead, self(a.ID)); err != nil {
		return nil, err
	}
	rows, err := s.d.Channels.ListSidebarChannelStates(ctx, a)
	if err != nil {
		return nil, err
	}
	out := make([]sync.SidebarChannelState, 0, len(rows))
	for _, r := range rows {
		out = append(out, sync.SidebarChannelState{ChannelID: r.ChannelID, ThreadOrder: r.ThreadOrder, Version: r.Version,
			UpdatedByDeviceID: r.UpdatedByDeviceID})
	}
	return out, nil
}
