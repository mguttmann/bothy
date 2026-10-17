// SPDX-License-Identifier: Apache-2.0

package wire

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// The empty sources of authz (ruling D2 of docs/briefs/M1-28-authz-core.md, 02 section 4.4):
// until the owning modules have tables they report the truth of an instance without those
// tables, so authorize decides fail closed. The credentials come from identity since M1-33 and
// the agent policy from agents since M1-51 (wire.NewAuthorizer).

// emptyStructure knows no space, channel or thread. M1-52 (spaces), M1-53 (channels) and M1-55
// (threads, dm participation) replace it with the StructureSource of chat.
type emptyStructure struct{}

var _ authz.StructureSource = emptyStructure{}

func (emptyStructure) Space(context.Context, ids.ID) (bool, error) { return false, nil }
func (emptyStructure) Spaces(context.Context) ([]ids.ID, error)    { return nil, nil }

// SpaceArchived knows no space until M1-52.
func (emptyStructure) SpaceArchived(context.Context, ids.ID) (bool, error) { return false, nil }
func (emptyStructure) Channel(context.Context, ids.ID) (authz.ChannelFacts, bool, error) {
	return authz.ChannelFacts{}, false, nil
}
func (emptyStructure) Thread(context.Context, ids.ID) (authz.ThreadFacts, bool, error) {
	return authz.ThreadFacts{}, false, nil
}
func (emptyStructure) Channels(context.Context) ([]authz.ChannelFacts, error)       { return nil, nil }
func (emptyStructure) ChannelsOperatedBy(context.Context, ids.ID) ([]ids.ID, error) { return nil, nil }
func (emptyStructure) DMChannelsOf(context.Context, ids.ID) ([]ids.ID, error)       { return nil, nil }

// ServedTargets knows no served target until M1-54 implements it in chat.
func (emptyStructure) ServedTargets(context.Context, ids.ID) ([]authz.ChannelRef, error) {
	return nil, nil
}
