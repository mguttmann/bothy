// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/chat/domain/fracindex"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Compile-time check: the import port.
var _ chat.ImportStructure = (*Service)(nil)

// errImportOutsideUnit is the misuse of an import write outside the caller's unit of work.
func errImportOutsideUnit() error {
	return errs.New(errs.CodeInternalError, "chat: import writes run inside the caller's unit of work")
}

// nameCollision is the answer to a new imported space or channel whose name is taken (22 5.1
// rule 1).
func nameCollision() error {
	return errs.New(errs.CodeConflict, "the name is taken by an object the import did not create").WithReason(chat.ReasonNameCollision)
}

// asNameCollision turns the conflict on /name of an insert into name_collision.
func asNameCollision(err error) error {
	if e, ok := errs.As(err); ok && e.Code == errs.CodeConflict {
		return nameCollision()
	}
	return err
}

// importKey checks the sort key of a new imported object: a fractional index key of
// chat/domain/fracindex, so the import keeps the Discord order (22 5.1 rule 4).
func importKey(key string) error {
	if !fracindex.Valid(key) {
		return codeError("/sort_key", domain.FieldInvalid, "sort_key is no fractional index key")
	}
	return nil
}

// importActor returns the actor of ctx, the creator of imported objects.
func importActor(ctx context.Context) (actor.Actor, error) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.ID.IsZero() {
		return actor.Actor{}, errs.New(errs.CodeInternalError, "chat: an import write without the importing principal")
	}
	return a, nil
}

// Space implements chat.ImportStructure (M8-03 design note 7): an existing space must be live; a
// new one is checked like space.create (name, then the fractional sort key of the tool), inserted
// without audit and announced with space.created. A taken name is conflict name_collision.
func (s *Service) Space(ctx context.Context, in chat.ImportedSpace) (ids.ID, error) {
	if !txn.InUnit(ctx) {
		return ids.ID{}, errImportOutsideUnit()
	}
	if in.ExistingID != nil {
		sp, found, err := s.d.Store.LockSpace(ctx, *in.ExistingID)
		if err != nil {
			return ids.ID{}, err
		}
		if !found || sp.Archived() {
			return ids.ID{}, invisible()
		}
		return sp.ID, nil
	}
	a, err := importActor(ctx)
	if err != nil {
		return ids.ID{}, err
	}
	name, err := domain.SpaceName(in.Name, s.cfg.SpaceNameMaxChars)
	if err != nil {
		return ids.ID{}, err
	}
	if err := importKey(in.SortKey); err != nil {
		return ids.ID{}, err
	}
	sp := domain.NewSpace(ids.New(), name, "", "", in.SortKey, a.ID)
	sp.CreatedAt = s.now()
	if err := s.d.Store.InsertSpace(ctx, sp); err != nil {
		return ids.ID{}, asNameCollision(err)
	}
	return sp.ID, s.emitSpace(ctx, authorize.EventSpaceCreated, sp)
}

// Channel implements chat.ImportStructure (M8-03 design note 7, 22 5.1 rules 2 and 3): an
// existing channel must be a live text or forum channel and gets read_only through SetReadOnly; a
// new one is checked like channel.create, gets read_only as given, operator mode none and no
// grant, a text channel its main thread in the instance search language (14 3.1: imports), and
// channel.created (with thread.created of the main thread). The space row is locked like
// channel.create does; an archived space is not_found.
func (s *Service) Channel(ctx context.Context, in chat.ImportedChannel) (ids.ID, error) {
	if !txn.InUnit(ctx) {
		return ids.ID{}, errImportOutsideUnit()
	}
	if in.ExistingID != nil {
		c, found, err := s.d.Store.Channel(ctx, *in.ExistingID)
		if err != nil {
			return ids.ID{}, err
		}
		if !found || archived(c) || c.Channel.Kind == domain.ChannelDM {
			return ids.ID{}, invisible()
		}
		return c.Channel.ID, s.SetReadOnly(ctx, c.Channel.ID, in.ReadOnly, "")
	}
	a, err := importActor(ctx)
	if err != nil {
		return ids.ID{}, err
	}
	ch, err := s.newChannel(in.SpaceID, a.ID, chat.CreateChannelCmd{Kind: in.Kind, Name: in.Name, Topic: &in.Topic})
	if err != nil {
		return ids.ID{}, err
	}
	if err := importKey(in.SortKey); err != nil {
		return ids.ID{}, err
	}
	sp, found, err := s.d.Store.LockSpace(ctx, in.SpaceID)
	if err != nil {
		return ids.ID{}, err
	}
	if !found || sp.Archived() {
		return ids.ID{}, invisible()
	}
	ch.SortKey, ch.ReadOnly, ch.OperatorInherit = in.SortKey, in.ReadOnly, false
	if err := s.d.Store.InsertChannel(ctx, ch); err != nil {
		return ids.ID{}, asNameCollision(err)
	}
	return ch.ID, s.announceImportedChannel(ctx, ch, sp)
}

// announceImportedChannel inserts the main thread of a new text channel and appends
// channel.created and thread.created.
func (s *Service) announceImportedChannel(ctx context.Context, ch domain.Channel, sp domain.Space) error {
	var main domain.Thread
	if ch.Kind == domain.ChannelText {
		main = domain.NewMainThread(ids.New(), ch.ID, ch.CreatedBy, ch.CreatedAt)
		lang, err := s.d.Directory.InstanceSearchLanguage(ctx)
		if err != nil {
			return err
		}
		main.SearchLanguage = lang
		if main, err = s.d.Store.InsertMainThread(ctx, main); err != nil {
			return err
		}
		ch.MainThreadID = main.ID
	}
	if err := s.emitChannel(ctx, authorize.EventChannelCreated, ChannelRow{Channel: ch, SpaceDefaultOperatorID: sp.DefaultOperatorAgentID}); err != nil {
		return err
	}
	if ch.Kind != domain.ChannelText {
		return nil
	}
	return s.emitThread(ctx, authorize.EventThreadCreated, ch.SpaceID, threadDTO(main, nil))
}

// ForumTag implements chat.ImportStructure (22 6.6): the channel is locked like a tag change of
// forum_tag.manage; a live tag with the same name without regard to case is adopted, otherwise
// the tag is inserted at the end, the channel version raised once and channel.updated appended.
func (s *Service) ForumTag(ctx context.Context, channelID ids.ID, t chat.ImportedTag) (ids.ID, error) {
	if !txn.InUnit(ctx) {
		return ids.ID{}, errImportOutsideUnit()
	}
	c, err := s.lockChannel(ctx, channelID)
	if err != nil {
		return ids.ID{}, err
	}
	if archived(c) {
		return ids.ID{}, invisible()
	}
	if c.Channel.Kind != domain.ChannelForum {
		return ids.ID{}, notAForum()
	}
	name, err := domain.TagName(t.Name, s.cfg.TagNameMaxChars)
	if err != nil {
		return ids.ID{}, err
	}
	tags, err := s.d.Store.ForumTags(ctx, channelID)
	if err != nil {
		return ids.ID{}, err
	}
	order := make([]domain.Keyed, 0, len(tags))
	for _, have := range tags {
		if strings.EqualFold(have.Name, name) {
			return have.ID, nil
		}
		order = append(order, domain.Keyed{ID: have.ID, SortKey: have.SortKey})
	}
	key, err := domain.KeyAt(order, len(order)-1)
	if err != nil {
		return ids.ID{}, err
	}
	tag := domain.ForumTag{ID: ids.New(), ChannelID: channelID, Name: name, SortKey: key}
	if err := s.d.Store.InsertForumTag(ctx, tag); err != nil {
		return ids.ID{}, err
	}
	c.Channel.Touch()
	if err := s.d.Store.SaveChannel(ctx, c.Channel); err != nil {
		return ids.ID{}, err
	}
	return tag.ID, s.emitChannel(ctx, authorize.EventChannelUpdated, c)
}

// SetReadOnly implements chat.ImportStructure: it locks the channel, refuses an unknown, dm or
// archived one (not_found), changes nothing for an equal flag, and otherwise writes read_only with
// a new version, appends channel.updated and publishes ChannelReadOnlySet, whose subscriber in
// runs cancels the active runs of a channel that becomes read-only. reason belongs to the
// caller's audit entry.
func (s *Service) SetReadOnly(ctx context.Context, channelID ids.ID, readOnly bool, _ string) error {
	if !txn.InUnit(ctx) {
		return errImportOutsideUnit()
	}
	c, err := s.lockChannel(ctx, channelID)
	if err != nil {
		return err
	}
	if archived(c) || c.Channel.Kind == domain.ChannelDM {
		return invisible()
	}
	_, err = s.writeReadOnly(ctx, c, readOnly)
	return err
}

// writeReadOnly writes the flag of the locked channel c when it differs and reports whether it
// did (channel.updated and ChannelReadOnlySet in the same unit).
func (s *Service) writeReadOnly(ctx context.Context, c ChannelRow, readOnly bool) (bool, error) {
	if c.Channel.ReadOnly == readOnly {
		return false, nil
	}
	c.Channel.ReadOnly = readOnly
	c.Channel.Touch()
	if err := s.d.Store.SaveChannelReadOnly(ctx, c.Channel); err != nil {
		return false, err
	}
	if err := s.emitChannel(ctx, authorize.EventChannelUpdated, c); err != nil {
		return false, err
	}
	return true, txn.Publish(ctx, chat.ChannelReadOnlySet{ChannelID: c.Channel.ID, ReadOnly: readOnly})
}

// LiveSpace implements chat.ImportStructure.
func (s *Service) LiveSpace(ctx context.Context, id ids.ID) (bool, error) {
	sp, found, err := s.d.Store.Space(ctx, id)
	return found && !sp.Archived(), err
}

// LiveChannel implements chat.ImportStructure.
func (s *Service) LiveChannel(ctx context.Context, id ids.ID) (ids.ID, bool, error) {
	c, found, err := s.d.Store.Channel(ctx, id)
	if err != nil || !found || archived(c) || c.Channel.Kind == domain.ChannelDM {
		return ids.ID{}, false, err
	}
	return c.Channel.SpaceID, true, nil
}
