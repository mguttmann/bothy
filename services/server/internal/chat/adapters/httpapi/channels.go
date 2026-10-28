// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Channels implements the eleven channel and forum tag operations of M1-53 (listSpaceChannels,
// createChannel, getChannel, updateChannel, moveChannel, archiveChannel, listForumTags,
// createForumTag, updateForumTag, moveForumTag, deleteForumTag) as part of
// rest.StrictServerInterface. The use cases authorize; the handlers only convert.
type Channels struct {
	structure chat.Structure
}

// NewChannels returns the channel handlers over the use cases.
func NewChannels(s chat.Structure) *Channels { return &Channels{structure: s} }

// ListSpaceChannels implements rest.StrictServerInterface (GET /spaces/{space_id}/channels).
func (h *Channels) ListSpaceChannels(ctx context.Context, req rest.ListSpaceChannelsRequestObject) (rest.ListSpaceChannelsResponseObject, error) {
	list, err := h.structure.ListSpaceChannels(ctx, principal(ctx), ids.ID(req.SpaceId))
	if err != nil {
		return nil, err
	}
	out := rest.ChannelViewList{Items: make([]rest.ChannelView, 0, len(list))}
	for _, v := range list {
		out.Items = append(out.Items, channelViewREST(v))
	}
	return rest.ListSpaceChannels200JSONResponse{Body: out}, nil
}

// CreateChannel implements rest.StrictServerInterface (POST /spaces/{space_id}/channels).
func (h *Channels) CreateChannel(ctx context.Context, req rest.CreateChannelRequestObject) (rest.CreateChannelResponseObject, error) {
	b := req.Body
	cmd := chat.CreateChannelCmd{Kind: string(b.Kind), Name: b.Name, Topic: b.Topic}
	if b.ForumDefaultSort != nil {
		s := string(*b.ForumDefaultSort)
		cmd.ForumDefaultSort = &s
	}
	if b.AfterChannelId != nil {
		cmd.AfterChannelID = ids.ID(*b.AfterChannelId)
	}
	c, err := h.structure.CreateChannel(ctx, principal(ctx), ids.ID(req.SpaceId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateChannel201JSONResponse{Body: channelREST(c)}, nil
}

// GetChannel implements rest.StrictServerInterface (GET /channels/{channel_id}).
func (h *Channels) GetChannel(ctx context.Context, req rest.GetChannelRequestObject) (rest.GetChannelResponseObject, error) {
	v, err := h.structure.GetChannel(ctx, principal(ctx), ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	return rest.GetChannel200JSONResponse{Body: channelViewREST(v), Headers: rest.GetChannel200ResponseHeaders{ETag: etag(v.Channel.Version)}}, nil
}

// UpdateChannel implements rest.StrictServerInterface (PATCH /channels/{channel_id}).
func (h *Channels) UpdateChannel(ctx context.Context, req rest.UpdateChannelRequestObject) (rest.UpdateChannelResponseObject, error) {
	b := req.Body
	cmd := chat.UpdateChannelCmd{Name: b.Name, Topic: b.Topic, IfMatch: precondition(req.Params.IfMatch),
		ClearDefaultNotificationLevel: b.ClearDefaultNotificationLevel != nil && *b.ClearDefaultNotificationLevel}
	if b.ForumDefaultSort != nil {
		s := string(*b.ForumDefaultSort)
		cmd.ForumDefaultSort = &s
	}
	if b.DefaultNotificationLevel != nil {
		s := string(*b.DefaultNotificationLevel)
		cmd.DefaultNotificationLevel = &s
	}
	c, err := h.structure.UpdateChannel(ctx, principal(ctx), ids.ID(req.ChannelId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.UpdateChannel200JSONResponse{Body: channelREST(c), Headers: rest.UpdateChannel200ResponseHeaders{ETag: etag(c.Version)}}, nil
}

// MoveChannel implements rest.StrictServerInterface (POST /channels/{channel_id}/move).
func (h *Channels) MoveChannel(ctx context.Context, req rest.MoveChannelRequestObject) (rest.MoveChannelResponseObject, error) {
	c, err := h.structure.MoveChannel(ctx, principal(ctx), ids.ID(req.ChannelId), movePosition(req.Body))
	if err != nil {
		return nil, err
	}
	return rest.MoveChannel200JSONResponse{Body: channelREST(c)}, nil
}

// ArchiveChannel implements rest.StrictServerInterface (POST /channels/{channel_id}/archive).
func (h *Channels) ArchiveChannel(ctx context.Context, req rest.ArchiveChannelRequestObject) (rest.ArchiveChannelResponseObject, error) {
	c, err := h.structure.ArchiveChannel(ctx, principal(ctx), ids.ID(req.ChannelId), req.Body.Reason)
	if err != nil {
		return nil, err
	}
	return rest.ArchiveChannel200JSONResponse{Body: channelREST(c)}, nil
}

// ListForumTags implements rest.StrictServerInterface (GET /channels/{channel_id}/tags).
func (h *Channels) ListForumTags(ctx context.Context, req rest.ListForumTagsRequestObject) (rest.ListForumTagsResponseObject, error) {
	list, err := h.structure.ListForumTags(ctx, principal(ctx), ids.ID(req.ChannelId))
	if err != nil {
		return nil, err
	}
	out := rest.ForumTagList{Items: make([]rest.ForumTag, 0, len(list))}
	for _, t := range list {
		out.Items = append(out.Items, tagREST(t))
	}
	return rest.ListForumTags200JSONResponse{Body: out}, nil
}

// CreateForumTag implements rest.StrictServerInterface (POST /channels/{channel_id}/tags).
func (h *Channels) CreateForumTag(ctx context.Context, req rest.CreateForumTagRequestObject) (rest.CreateForumTagResponseObject, error) {
	b := req.Body
	cmd := chat.CreateForumTagCmd{Name: b.Name, Color: b.Color, Restricted: b.Restricted}
	if b.AfterTagId != nil {
		cmd.AfterTagID = ids.ID(*b.AfterTagId)
	}
	t, err := h.structure.CreateForumTag(ctx, principal(ctx), ids.ID(req.ChannelId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.CreateForumTag201JSONResponse{Body: tagREST(t)}, nil
}

// UpdateForumTag implements rest.StrictServerInterface (PATCH /channels/{channel_id}/tags/{tag_id}).
func (h *Channels) UpdateForumTag(ctx context.Context, req rest.UpdateForumTagRequestObject) (rest.UpdateForumTagResponseObject, error) {
	b := req.Body
	cmd := chat.UpdateForumTagCmd{Name: b.Name, Color: b.Color, ClearColor: b.ClearColor != nil && *b.ClearColor, Restricted: b.Restricted}
	t, err := h.structure.UpdateForumTag(ctx, principal(ctx), ids.ID(req.ChannelId), ids.ID(req.TagId), cmd)
	if err != nil {
		return nil, err
	}
	return rest.UpdateForumTag200JSONResponse{Body: tagREST(t)}, nil
}

// MoveForumTag implements rest.StrictServerInterface (POST /channels/{channel_id}/tags/{tag_id}/move).
func (h *Channels) MoveForumTag(ctx context.Context, req rest.MoveForumTagRequestObject) (rest.MoveForumTagResponseObject, error) {
	t, err := h.structure.MoveForumTag(ctx, principal(ctx), ids.ID(req.ChannelId), ids.ID(req.TagId), movePosition(req.Body))
	if err != nil {
		return nil, err
	}
	return rest.MoveForumTag200JSONResponse{Body: tagREST(t)}, nil
}

// DeleteForumTag implements rest.StrictServerInterface (DELETE /channels/{channel_id}/tags/{tag_id}).
func (h *Channels) DeleteForumTag(ctx context.Context, req rest.DeleteForumTagRequestObject) (rest.DeleteForumTagResponseObject, error) {
	if err := h.structure.DeleteForumTag(ctx, principal(ctx), ids.ID(req.ChannelId), ids.ID(req.TagId)); err != nil {
		return nil, err
	}
	return rest.DeleteForumTag204Response{}, nil
}

// movePosition converts a MovePosition body; nil is the start.
func movePosition(b *rest.MovePosition) chat.MovePosition {
	var p chat.MovePosition
	if b == nil {
		return p
	}
	if b.AfterId != nil {
		p.AfterID = ids.ID(*b.AfterId)
	}
	if b.BeforeId != nil {
		p.BeforeID = ids.ID(*b.BeforeId)
	}
	return p
}

// channelREST converts a channel (schema Channel); empty values become absent members, never
// null (ADR 0053).
func channelREST(c chat.Channel) rest.Channel {
	out := rest.Channel{
		Id: rest.Uuid(c.ID), SpaceId: optionalID(c.SpaceID), Kind: rest.ChannelKind(c.Kind), Name: c.Name, Topic: c.Topic, SortKey: c.SortKey,
		Operator: rest.ChannelOperator{Mode: rest.OperatorMode(c.Operator.Mode), AgentId: optionalID(c.Operator.AgentID),
			EffectiveAgentId: optionalID(c.Operator.EffectiveAgentID)},
		ReadOnly: c.ReadOnly, ProjectBound: c.ProjectBound, MainThreadId: optionalID(c.MainThreadID), ArchivedAt: optionalTime(c.ArchivedAt),
		CreatedAt: c.CreatedAt.UTC().Truncate(time.Millisecond), Version: c.Version, SettingsVersion: c.SettingsVersion,
	}
	if c.ForumDefaultSort != "" {
		s := rest.ForumSort(c.ForumDefaultSort)
		out.ForumDefaultSort = &s
	}
	if c.DefaultNotificationLevel != "" {
		l := rest.NotificationLevel(c.DefaultNotificationLevel)
		out.DefaultNotificationLevel = &l
	}
	return out
}

// channelViewREST converts a channel view (schema ChannelView).
func channelViewREST(v chat.ChannelView) rest.ChannelView {
	caps := make([]rest.Capability, 0, len(v.Capabilities))
	for _, c := range v.Capabilities {
		caps = append(caps, rest.Capability(c))
	}
	return rest.ChannelView{Channel: channelREST(v.Channel), Level: rest.Level(v.Level), Capabilities: caps}
}

// tagREST converts a forum tag (schema ForumTag).
func tagREST(t chat.ForumTag) rest.ForumTag {
	return rest.ForumTag{Id: rest.Uuid(t.ID), ChannelId: rest.Uuid(t.ChannelID), Name: t.Name, Color: optional(t.Color), SortKey: t.SortKey,
		Restricted: t.Restricted}
}
