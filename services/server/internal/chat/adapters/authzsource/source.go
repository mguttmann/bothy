// SPDX-License-Identifier: Apache-2.0

// Package authzsource is the adapter through which the chat module answers authz (M1-52, M1-53,
// M1-54): Source implements authz.StructureSource (Space, SpaceArchived, Spaces, SpaceDetails,
// Channel, Channels, ChannelsOperatedBy, ChannelDetails; since M1-54 Thread with the fixed
// operator and ServedTargets for I1) over the tables spaces, channels and threads, so authorize
// and the rights hook resolve the structure and the operator rule from real data
// (docs/spec/02-architektur.md section 4.3, "Quellen-Port"). The dm participation follows with
// V3; until then internal/wire joins this source with the empty dm part. authz never reads these
// tables.
//
// Example:
//
//	src := authzsource.New(chatpg.NewRepository(pool))
//	ok, err := src.Space(ctx, id)
package authzsource

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/authz"
	"github.com/OWNER/bothy/services/server/internal/chat/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Source answers authz.StructureSource except the dm participation. It reads inside the ambient
// transaction of ctx when there is one, so the rights hook sees a space a unit of work just
// archived. It is safe for concurrent use when its store is.
type Source struct {
	store app.Store
}

// New returns the source over the spaces of store.
func New(store app.Store) *Source { return &Source{store: store} }

// Space implements authz.StructureSource: whether the space exists, archived or not.
func (s *Source) Space(ctx context.Context, id ids.ID) (bool, error) {
	_, found, err := s.store.Space(ctx, id)
	return found, err
}

// SpaceArchived implements authz.StructureSource: false for an unknown space.
func (s *Source) SpaceArchived(ctx context.Context, id ids.ID) (bool, error) {
	sp, found, err := s.store.Space(ctx, id)
	return found && sp.Archived(), err
}

// Spaces implements authz.StructureSource: every space, archived ones included, ascending by id.
func (s *Source) Spaces(ctx context.Context) ([]ids.ID, error) {
	return s.store.SpaceIDs(ctx)
}

// SpaceDetails implements authz.StructureSource: the details of the known spaces among spaceIDs,
// archived or not; an unknown id is absent (M1-49 on the spaces of M1-52, INT-W15/2).
func (s *Source) SpaceDetails(ctx context.Context, spaceIDs []ids.ID) (map[ids.ID]authz.SpaceDetail, error) {
	out := make(map[ids.ID]authz.SpaceDetail, len(spaceIDs))
	for _, id := range spaceIDs {
		sp, found, err := s.store.Space(ctx, id)
		if err != nil {
			return nil, err
		}
		if found {
			out[id] = authz.SpaceDetail{ID: sp.ID, Name: sp.Name, Icon: sp.Icon, Color: sp.Color, SortKey: sp.SortKey,
				DefaultOperatorID: sp.DefaultOperatorAgentID, ArchivedAt: sp.ArchivedAt, Version: sp.Version, CreatedAt: sp.CreatedAt}
		}
	}
	return out, nil
}

// Channel implements authz.StructureSource: the facts of the channel id; Archived is set for an
// archived channel and for every channel of an archived space (07 section 4.5).
func (s *Source) Channel(ctx context.Context, id ids.ID) (authz.ChannelFacts, bool, error) {
	c, found, err := s.store.Channel(ctx, id)
	if err != nil || !found {
		return authz.ChannelFacts{}, false, err
	}
	return facts(c), true, nil
}

// Channels implements authz.StructureSource: every channel that is not a dm channel, archived
// ones included, ascending by id.
func (s *Source) Channels(ctx context.Context) ([]authz.ChannelFacts, error) {
	rows, err := s.store.AllChannels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]authz.ChannelFacts, 0, len(rows))
	for _, c := range rows {
		out = append(out, facts(c))
	}
	return out, nil
}

// ChannelsOperatedBy implements authz.StructureSource: the channels that are not dm channels
// whose effective operator is agentID, archived ones included, ascending by id; none for the zero
// id.
func (s *Source) ChannelsOperatedBy(ctx context.Context, agentID ids.ID) ([]ids.ID, error) {
	if agentID.IsZero() {
		return nil, nil
	}
	rows, err := s.store.AllChannels(ctx)
	if err != nil {
		return nil, err
	}
	var out []ids.ID
	for _, c := range rows {
		if c.Channel.Operator(c.SpaceDefaultOperatorID).EffectiveAgentID == agentID {
			out = append(out, c.Channel.ID)
		}
	}
	return out, nil
}

// ChannelDetails implements authz.StructureSource: the details of the known channels among
// channelIDs, archived or not; an unknown id is absent (M1-49).
func (s *Source) ChannelDetails(ctx context.Context, channelIDs []ids.ID) (map[ids.ID]authz.ChannelDetail, error) {
	out := make(map[ids.ID]authz.ChannelDetail, len(channelIDs))
	for _, id := range channelIDs {
		c, found, err := s.store.Channel(ctx, id)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		ch := c.Channel
		op := ch.Operator(c.SpaceDefaultOperatorID)
		out[id] = authz.ChannelDetail{
			ID: ch.ID, SpaceID: ch.SpaceID, Kind: authz.ChannelKind(ch.Kind), Name: ch.Name, Topic: ch.Topic, SortKey: ch.SortKey,
			OperatorMode: string(op.Mode), OperatorAgentID: op.AgentID, EffectiveOperatorID: op.EffectiveAgentID,
			ForumDefaultSort: ch.ForumDefaultSort, DefaultNotificationLevel: ch.DefaultNotificationLevel, ReadOnly: ch.ReadOnly,
			MainThreadID: ch.MainThreadID, ArchivedAt: ch.ArchivedAt, Version: ch.Version, SettingsVersion: ch.SettingsVersion,
			CreatedAt: ch.CreatedAt,
		}
	}
	return out, nil
}

// Thread implements authz.StructureSource (brief M1-55 D8): the channel, the kind, the creator and
// the fixed operator of the thread id; the space and the archived or read-only state come with the
// facts of its channel, which authz resolves next. It reads inside the ambient transaction of ctx
// when there is one.
func (s *Source) Thread(ctx context.Context, id ids.ID) (authz.ThreadFacts, bool, error) {
	th, found, err := s.store.Thread(ctx, id)
	if err != nil || !found {
		return authz.ThreadFacts{}, false, err
	}
	return authz.ThreadFacts{ID: th.ID, ChannelID: th.ChannelID, Main: th.Main(), OperatorAgentID: th.OperatorAgentID, CreatedBy: th.CreatedBy}, true, nil
}

// facts converts a channel row into the facts of authorize. Project binding comes with M2-03 and
// dm participation with V3; both are absent here.
func facts(c app.ChannelRow) authz.ChannelFacts {
	ch := c.Channel
	return authz.ChannelFacts{
		ID: ch.ID, SpaceID: ch.SpaceID, Kind: authz.ChannelKind(ch.Kind), ReadOnly: ch.ReadOnly,
		EffectiveOperatorID: ch.Operator(c.SpaceDefaultOperatorID).EffectiveAgentID,
		RequireMention:      ch.RequireMention, ThreadRequireMention: ch.ThreadRequireMention, FreeResponse: ch.FreeResponse,
		AllowMentionedAgents: ch.AllowMentionedAgents, Archived: ch.Archived() || c.SpaceArchived,
	}
}

// ServedTargets implements authz.StructureSource: the targets agentID serves for I1 (10 sections
// 5.2 and 5.3, D5 of M1-54), read inside the ambient transaction under the lock of
// permission_version; none for the zero id.
func (s *Source) ServedTargets(ctx context.Context, agentID ids.ID) ([]authz.ChannelRef, error) {
	if agentID.IsZero() {
		return nil, nil
	}
	list, err := s.store.ServedTargets(ctx, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]authz.ChannelRef, 0, len(list))
	for _, t := range list {
		out = append(out, authz.ChannelRef{ChannelID: t.ChannelID, SpaceID: t.SpaceID})
	}
	return out, nil
}
