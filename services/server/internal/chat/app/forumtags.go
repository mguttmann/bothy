// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Audit values of forum_tag.manage: details {op, tag_id} (docs/DECISIONS.md M1-53/3).
const (
	attrOp    = "op"
	attrTagID = "tag_id"

	opCreate = "create"
	opUpdate = "update"
	opMove   = "move"
	opDelete = "delete"
)

// tagDTO converts a tag into the port DTO.
func tagDTO(t domain.ForumTag) chat.ForumTag {
	return chat.ForumTag{ID: t.ID, ChannelID: t.ChannelID, Name: t.Name, Color: t.Color, SortKey: t.SortKey, Restricted: t.Restricted}
}

// notAForum is the answer to a tag change on a channel that is not a forum.
func notAForum() error {
	return codeError("", domain.FieldNotAForum, "the channel is not a forum")
}

// ListForumTags implements chat.Structure: channel.read, then the tags that are not deleted in
// their order; a text channel has none.
func (s *Service) ListForumTags(ctx context.Context, a actor.Actor, channelID ids.ID) ([]chat.ForumTag, error) {
	if _, err := s.loadChannel(ctx, a, authorize.ActionChannelRead, channelID); err != nil {
		return nil, err
	}
	tags, err := s.d.Store.ForumTags(ctx, channelID)
	if err != nil {
		return nil, err
	}
	out := make([]chat.ForumTag, 0, len(tags))
	for _, t := range tags {
		out = append(out, tagDTO(t))
	}
	return out, nil
}

// tagUnit runs fn in one unit of work for a forum tag change: forum_tag.manage on the channel
// (an invisible channel is 404), 400 not_a_forum for another kind, the lock of the channel row
// (it orders every tag change of the forum), 409 for an archived channel. fn changes the tags and
// reports what it did; for a change the unit bumps the channel version once (03 forum_tags),
// records {op, tag_id} and appends channel.updated, otherwise it records no_change.
func (s *Service) tagUnit(ctx context.Context, a actor.Actor, channelID ids.ID, op string,
	fn func(ctx context.Context, c ChannelRow) (tag domain.ForumTag, changed bool, err error)) (domain.ForumTag, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return domain.ForumTag{}, err
	}
	var out domain.ForumTag
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.tagChange(ctx, a, channelID, op, fn)
		return err
	})
	return out, err
}

// tagChange is the body of tagUnit inside its unit of work.
func (s *Service) tagChange(ctx context.Context, a actor.Actor, channelID ids.ID, op string,
	fn func(ctx context.Context, c ChannelRow) (domain.ForumTag, bool, error)) (domain.ForumTag, error) {
	c, err := s.loadChannel(ctx, a, authorize.ActionForumTagManage, channelID)
	if err != nil {
		return domain.ForumTag{}, err
	}
	if c.Channel.Kind != domain.ChannelForum {
		return domain.ForumTag{}, notAForum()
	}
	if c, err = s.lockChannel(ctx, channelID); err != nil {
		return domain.ForumTag{}, err
	}
	if archived(c) {
		return domain.ForumTag{}, channelArchivedConflict()
	}
	tag, changed, err := fn(ctx, c)
	if err != nil {
		return domain.ForumTag{}, err
	}
	attrs := auditing.Attrs{attrOp: auditing.Code(op), attrTagID: auditing.ID(tag.ID)}
	if !changed {
		return tag, s.noChangeChannel(ctx, authorize.ActionForumTagManage, channelID, attrs)
	}
	c.Channel.Touch()
	if err := s.d.Store.SaveChannel(ctx, c.Channel); err != nil {
		return domain.ForumTag{}, err
	}
	if err := s.recordChannel(ctx, authorize.ActionForumTagManage, channelID, attrs); err != nil {
		return domain.ForumTag{}, err
	}
	return tag, s.emitChannel(ctx, authorize.EventChannelUpdated, c)
}

// liveTag returns the tag id of channelID that is not deleted; a tag of another channel, an
// unknown one and (with live) a deleted one are not_found.
func (s *Service) liveTag(ctx context.Context, channelID, tagID ids.ID, live bool) (domain.ForumTag, error) {
	t, found, err := s.d.Store.ForumTag(ctx, tagID)
	if err != nil {
		return domain.ForumTag{}, err
	}
	if !found || t.ChannelID != channelID || (live && t.Deleted()) {
		return domain.ForumTag{}, invisible()
	}
	return t, nil
}

// tagOrder returns the tags of the forum that are not deleted as an order.
func (s *Service) tagOrder(ctx context.Context, channelID ids.ID) ([]domain.Keyed, error) {
	tags, err := s.d.Store.ForumTags(ctx, channelID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Keyed, 0, len(tags))
	for _, t := range tags {
		out = append(out, domain.Keyed{ID: t.ID, SortKey: t.SortKey})
	}
	return out, nil
}

// checkTagText checks a tag name (when given) and colour (when given) and returns the trimmed
// name.
func (s *Service) checkTagText(name, color *string) (*string, error) {
	if color != nil {
		if err := domain.CheckColor(*color); err != nil {
			return nil, err
		}
	}
	if name == nil {
		return nil, nil
	}
	n, err := domain.TagName(*name, s.cfg.TagNameMaxChars)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// CreateForumTag implements chat.Structure (forum_tag.manage, see tagUnit): name and colour
// checks, the key after after_tag_id or at the end, the insert.
func (s *Service) CreateForumTag(ctx context.Context, a actor.Actor, channelID ids.ID, cmd chat.CreateForumTagCmd) (chat.ForumTag, error) {
	t, err := s.tagUnit(ctx, a, channelID, opCreate, func(ctx context.Context, _ ChannelRow) (domain.ForumTag, bool, error) {
		name, err := s.checkTagText(&cmd.Name, cmd.Color)
		if err != nil {
			return domain.ForumTag{}, false, err
		}
		order, err := s.tagOrder(ctx, channelID)
		if err != nil {
			return domain.ForumTag{}, false, err
		}
		after := len(order) - 1
		if !cmd.AfterTagID.IsZero() {
			if after = keyIndex(order, cmd.AfterTagID); after < 0 {
				return domain.ForumTag{}, false, codeError("/after_tag_id", domain.FieldNotASibling, "after_tag_id is no live tag of the forum")
			}
		}
		key, err := domain.KeyAt(order, after)
		if err != nil {
			return domain.ForumTag{}, false, err
		}
		t := domain.ForumTag{ID: ids.New(), ChannelID: channelID, Name: *name, Color: deref(cmd.Color), SortKey: key,
			Restricted: cmd.Restricted != nil && *cmd.Restricted}
		return t, true, s.d.Store.InsertForumTag(ctx, t)
	})
	return tagDTO(t), err
}

// UpdateForumTag implements chat.Structure (forum_tag.manage, see tagUnit): the body rules of
// domain.TagChange, name and colour checks; no change answers the tag without an event.
func (s *Service) UpdateForumTag(ctx context.Context, a actor.Actor, channelID, tagID ids.ID, cmd chat.UpdateForumTagCmd) (chat.ForumTag, error) {
	t, err := s.tagUnit(ctx, a, channelID, opUpdate, func(ctx context.Context, _ ChannelRow) (domain.ForumTag, bool, error) {
		t, err := s.liveTag(ctx, channelID, tagID, true)
		if err != nil {
			return domain.ForumTag{}, false, err
		}
		change := domain.TagChange{Name: cmd.Name, Color: cmd.Color, ClearColor: cmd.ClearColor, Restricted: cmd.Restricted}
		if err := change.Check(); err != nil {
			return domain.ForumTag{}, false, err
		}
		if change.Name, err = s.checkTagText(cmd.Name, cmd.Color); err != nil {
			return domain.ForumTag{}, false, err
		}
		if len(t.Update(change)) == 0 {
			return t, false, nil
		}
		return t, true, s.d.Store.SaveForumTag(ctx, t)
	})
	return tagDTO(t), err
}

// MoveForumTag implements chat.Structure (forum_tag.manage, see tagUnit): the position rules of
// moveChannel among the tags of the forum that are not deleted; one tag row changes.
func (s *Service) MoveForumTag(ctx context.Context, a actor.Actor, channelID, tagID ids.ID, pos chat.MovePosition) (chat.ForumTag, error) {
	p := domain.MovePosition{AfterID: pos.AfterID, BeforeID: pos.BeforeID}
	t, err := s.tagUnit(ctx, a, channelID, opMove, func(ctx context.Context, _ ChannelRow) (domain.ForumTag, bool, error) {
		t, err := s.liveTag(ctx, channelID, tagID, true)
		if err != nil {
			return domain.ForumTag{}, false, err
		}
		order, err := s.tagOrder(ctx, channelID)
		if err != nil {
			return domain.ForumTag{}, false, err
		}
		order = slices.DeleteFunc(order, func(k domain.Keyed) bool { return k.ID == tagID })
		i, err := p.Index(order, tagID)
		if err != nil {
			return domain.ForumTag{}, false, err
		}
		if t.SortKey, err = domain.KeyAt(order, i); err != nil {
			return domain.ForumTag{}, false, err
		}
		return t, true, s.d.Store.SaveForumTag(ctx, t)
	})
	return tagDTO(t), err
}

// DeleteForumTag implements chat.Structure (forum_tag.manage, see tagUnit): sets deleted_at; a
// deleted tag of the channel is a call without change, a tag that never belonged to the channel
// is not_found.
func (s *Service) DeleteForumTag(ctx context.Context, a actor.Actor, channelID, tagID ids.ID) error {
	_, err := s.tagUnit(ctx, a, channelID, opDelete, func(ctx context.Context, _ ChannelRow) (domain.ForumTag, bool, error) {
		t, err := s.liveTag(ctx, channelID, tagID, false)
		if err != nil {
			return domain.ForumTag{}, false, err
		}
		if t.Deleted() {
			return t, false, nil
		}
		t.DeletedAt = s.d.Clock.Now().UTC().Truncate(time.Microsecond)
		return t, true, s.d.Store.SaveForumTag(ctx, t)
	})
	return err
}
