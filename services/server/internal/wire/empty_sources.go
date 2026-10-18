// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The empty sources of authz (ruling D2 of docs/briefs/M1-28-authz-core.md, 02 section 4.4):
// until the owning modules have tables they report the truth of an instance without those
// tables, so authorize decides fail closed. The credentials come from identity since M1-33, the
// agent policy from agents since M1-51 and the spaces from chat since M1-52 (wire.NewAuthorizer).

// emptyChannels is the channel and thread part of the structure source: it knows no channel,
// thread or dm participation. Spaces come from chat since M1-52 (chat.go, chatStructure); M1-53
// (channels) and M1-55 (threads, dm participation) replace this part with chat.
type emptyChannels struct{}

func (emptyChannels) Channel(context.Context, ids.ID) (authz.ChannelFacts, bool, error) {
	return authz.ChannelFacts{}, false, nil
}
func (emptyChannels) Thread(context.Context, ids.ID) (authz.ThreadFacts, bool, error) {
	return authz.ThreadFacts{}, false, nil
}
func (emptyChannels) Channels(context.Context) ([]authz.ChannelFacts, error)       { return nil, nil }
func (emptyChannels) ChannelsOperatedBy(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }
func (emptyChannels) DMChannelsOf(context.Context, ids.ID) ([]ids.ID, error)       { return nil, nil }

// ServedTargets knows no served target until M1-54 implements it in chat.
func (emptyChannels) ServedTargets(context.Context, ids.ID) ([]authz.ChannelRef, error) {
	return nil, nil
}

// ChannelDetails knows no channel until M1-53; the space details come from chat since M1-52
// (INT-W15/2).
func (emptyChannels) ChannelDetails(context.Context, []ids.ID) (map[ids.ID]authz.ChannelDetail, error) {
	return map[ids.ID]authz.ChannelDetail{}, nil
}
