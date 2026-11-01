// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/chat/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Values of the channel audit entries and events (05 sections 4.5 and 6).
const (
	resourceChannel    = "channel"
	resourceThread     = "thread"
	receiveChannelRead = "channel.read"
	receiveThreadRead  = "thread.read"

	attrSpaceID   = "space_id"
	attrChannelID = "channel_id"
	attrKind      = "kind"
)

// channelResource describes the channel id; space is a path hint (zero when unknown).
func channelResource(id, space ids.ID) authorize.Resource {
	return authorize.Resource{Kind: authorize.ResourceChannel, ID: id, SpaceID: space}
}

// archived reports a channel that is archived itself or lies in an archived space (07 section
// 4.5): every state change of it or its tags is a 409 conflict (D2 point 3 of the brief).
func archived(c ChannelRow) bool { return c.Channel.Archived() || c.SpaceArchived }

// channelArchivedConflict is the answer to a change of an archived channel.
func channelArchivedConflict() error {
	return errs.New(errs.CodeConflict, "the channel is archived")
}

// loadChannel reads the channel id and authorizes act on it before it answers a missing channel,
// so an unknown and an invisible id give the same answer (02 section 9.2).
func (s *Service) loadChannel(ctx context.Context, a actor.Actor, act authorize.Action, id ids.ID) (ChannelRow, error) {
	c, found, err := s.d.Store.Channel(ctx, id)
	if err != nil {
		return ChannelRow{}, err
	}
	if err := s.check(ctx, a, act, channelResource(id, ids.ID{})); err != nil {
		return ChannelRow{}, err
	}
	if !found {
		return ChannelRow{}, invisible()
	}
	return c, nil
}

// lockChannel locks the channel id inside the unit of work.
func (s *Service) lockChannel(ctx context.Context, id ids.ID) (ChannelRow, error) {
	c, found, err := s.d.Store.LockChannel(ctx, id)
	if err != nil {
		return ChannelRow{}, err
	}
	if !found {
		return ChannelRow{}, invisible()
	}
	return c, nil
}

// channelDTO converts a channel row into the port DTO.
func channelDTO(c ChannelRow) chat.Channel {
	ch := c.Channel
	op := ch.Operator(c.SpaceDefaultOperatorID)
	return chat.Channel{
		ID: ch.ID, SpaceID: ch.SpaceID, Kind: string(ch.Kind), Name: ch.Name, Topic: ch.Topic, SortKey: ch.SortKey,
		Operator:         chat.ChannelOperator{Mode: string(op.Mode), AgentID: op.AgentID, EffectiveAgentID: op.EffectiveAgentID},
		ForumDefaultSort: ch.ForumDefaultSort, DefaultNotificationLevel: ch.DefaultNotificationLevel, ReadOnly: ch.ReadOnly,
		ProjectBound: c.ProjectBound, MainThreadID: ch.MainThreadID, CreatedAt: ch.CreatedAt, ArchivedAt: ch.ArchivedAt, Version: ch.Version,
		SettingsVersion: ch.SettingsVersion, DeletedAt: ch.Deletion.At,
	}
}

// channelView returns the channel with the caller's rights (schema ChannelView, D9 of the brief):
// the RightsQuery of authz answers level and capabilities with the archive cap.
func (s *Service) channelView(ctx context.Context, a actor.Actor, c ChannelRow) (chat.ChannelView, error) {
	r, err := s.d.Rights.EffectiveRights(ctx, a, authorize.ActionChannelRead, channelResource(c.Channel.ID, c.Channel.SpaceID))
	if err != nil {
		return chat.ChannelView{}, err
	}
	v := chat.ChannelView{Channel: channelDTO(c), Level: r.Level, Capabilities: r.Capabilities}
	if v.Level == "" {
		v.Level = "read"
	}
	if v.Capabilities == nil {
		v.Capabilities = []string{}
	}
	return v, nil
}

// recordChannel writes the allow entry of act on the channel id inside the unit of work.
func (s *Service) recordChannel(ctx context.Context, act authorize.Action, id ids.ID, attrs auditing.Attrs) error {
	return s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(act), Decision: auditing.DecisionAllow, ResourceKind: resourceChannel, ResourceID: id, Attrs: attrs,
	})
}

// noChangeChannel records a call on a channel without effect (06 section 2 "Ohne Wirkung").
func (s *Service) noChangeChannel(ctx context.Context, act authorize.Action, id ids.ID, attrs auditing.Attrs) error {
	if attrs == nil {
		attrs = auditing.Attrs{}
	}
	attrs[attrNoChange] = auditing.Bool(true)
	return s.recordChannel(ctx, act, id, attrs)
}

// ListSpaceChannels implements chat.Structure (07 section 7.4, 04 section 7.2): space.read on the
// space (an invisible space is 404), then Scope of channel.read built per request, so a channel
// created after the rights snapshot is listed without a new rights version; the query keeps the
// channels of the space that are not archived and match the scope, and authorize checks each row
// in filter mode. A row authorize refuses is left out.
func (s *Service) ListSpaceChannels(ctx context.Context, a actor.Actor, spaceID ids.ID) ([]chat.ChannelView, error) {
	if _, err := s.load(ctx, a, authorize.ActionSpaceRead, spaceID); err != nil {
		return nil, err
	}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionChannelRead)
	if err != nil {
		return nil, err
	}
	rows, err := s.d.Store.VisibleChannels(ctx, spaceID, sc.InstanceWide, sc.ChannelIDs, sc.FullyVisibleSpaceIDs)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	out := make([]chat.ChannelView, 0, len(rows))
	for _, c := range rows {
		d, err := s.d.Authorizer.Authorize(fctx, a, authorize.ActionChannelRead, channelResource(c.Channel.ID, c.Channel.SpaceID))
		if err != nil {
			return nil, err
		}
		if !d.Allowed() {
			continue
		}
		v, err := s.channelView(ctx, a, c)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

// GetChannel implements chat.Structure: channel.read, then 404 for an unknown id; an archived
// channel stays readable.
func (s *Service) GetChannel(ctx context.Context, a actor.Actor, id ids.ID) (chat.ChannelView, error) {
	c, err := s.loadChannel(ctx, a, authorize.ActionChannelRead, id)
	if err != nil {
		return chat.ChannelView{}, err
	}
	return s.channelView(ctx, a, c)
}

// CreateChannel implements chat.Structure in one unit of work: channel.create on the space, the
// checks of kind, name, topic and forum sort, then the lock of the space row (it orders
// concurrent creates and moves in the space, docs/DECISIONS.md M1-53/1), 409 for an archived
// space, the key after after_channel_id or at the end, the insert, for a text channel the main
// thread (R-DM-11), the audit entry, channel.created and for a text channel thread.created.
func (s *Service) CreateChannel(ctx context.Context, a actor.Actor, spaceID ids.ID, cmd chat.CreateChannelCmd) (chat.Channel, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Channel{}, err
	}
	var out ChannelRow
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.createChannel(ctx, a, spaceID, cmd)
		return err
	})
	return channelDTO(out), err
}

// createChannel is CreateChannel inside its unit of work.
func (s *Service) createChannel(ctx context.Context, a actor.Actor, spaceID ids.ID, cmd chat.CreateChannelCmd) (ChannelRow, error) {
	if _, err := s.load(ctx, a, authorize.ActionChannelCreate, spaceID); err != nil {
		return ChannelRow{}, err
	}
	ch, err := s.newChannel(spaceID, a.ID, cmd)
	if err != nil {
		return ChannelRow{}, err
	}
	sp, err := s.lock(ctx, spaceID)
	if err != nil {
		return ChannelRow{}, err
	}
	if sp.Archived() {
		return ChannelRow{}, archivedConflict()
	}
	if ch.SortKey, err = s.channelKey(ctx, spaceID, cmd.AfterChannelID); err != nil {
		return ChannelRow{}, err
	}
	if err := s.d.Store.InsertChannel(ctx, ch); err != nil {
		return ChannelRow{}, err
	}
	var main domain.Thread
	if ch.Kind == domain.ChannelText {
		if main, err = s.insertMainThread(ctx, a, ch); err != nil {
			return ChannelRow{}, err
		}
		ch.MainThreadID = main.ID
	}
	row := ChannelRow{Channel: ch, SpaceDefaultOperatorID: sp.DefaultOperatorAgentID}
	if err := s.recordChannel(ctx, authorize.ActionChannelCreate, ch.ID, auditing.Attrs{
		attrSpaceID: auditing.ID(spaceID), attrChannelID: auditing.ID(ch.ID), attrKind: auditing.Code(string(ch.Kind)),
	}); err != nil {
		return ChannelRow{}, err
	}
	if err := s.emitChannel(ctx, authorize.EventChannelCreated, row); err != nil {
		return ChannelRow{}, err
	}
	if ch.Kind == domain.ChannelText {
		if err := s.emitThread(ctx, authorize.EventThreadCreated, spaceID, threadDTO(main, nil)); err != nil {
			return ChannelRow{}, err
		}
	}
	return row, nil
}

// insertMainThread inserts the main thread of the new text channel ch with the search language
// of its creator a (14 3.1, docs/DECISIONS.md D-M3-13-2) and returns the stored thread.
func (s *Service) insertMainThread(ctx context.Context, a actor.Actor, ch domain.Channel) (domain.Thread, error) {
	main := domain.NewMainThread(ids.New(), ch.ID, a.ID, ch.CreatedAt)
	lang, err := s.searchLanguage(ctx, a)
	if err != nil {
		return domain.Thread{}, err
	}
	main.SearchLanguage = lang
	return s.d.Store.InsertMainThread(ctx, main)
}

// newChannel checks kind, forum sort, name and topic of cmd and returns the new channel without
// its sort key.
func (s *Service) newChannel(spaceID, by ids.ID, cmd chat.CreateChannelCmd) (domain.Channel, error) {
	kind := domain.ChannelKind(cmd.Kind)
	if err := domain.CheckCreateKind(kind, cmd.ForumDefaultSort); err != nil {
		return domain.Channel{}, err
	}
	lim := s.cfg.limits()
	name, err := domain.ChannelName(cmd.Name, lim.ChannelName)
	if err != nil {
		return domain.Channel{}, err
	}
	topic, err := domain.ChannelTopic(deref(cmd.Topic), lim.ChannelTopic)
	if err != nil {
		return domain.Channel{}, err
	}
	ch := domain.NewChannel(ids.New(), spaceID, kind, name, topic, "", by, s.cfg.HistoryBackfillLimitDefault)
	ch.ForumDefaultSort = deref(cmd.ForumDefaultSort)
	ch.CreatedAt = s.d.Clock.Now().UTC().Truncate(time.Microsecond)
	return ch, nil
}

// channelKey returns the key of a new channel of spaceID after the channel after, or at the end
// for zero. Errors: validation_failed on /after_channel_id for a channel that is no live sibling.
func (s *Service) channelKey(ctx context.Context, spaceID, after ids.ID) (string, error) {
	order, err := s.d.Store.ChannelOrder(ctx, spaceID)
	if err != nil {
		return "", err
	}
	i := len(order) - 1
	if !after.IsZero() {
		if i = keyIndex(order, after); i < 0 {
			return "", codeError("/after_channel_id", domain.FieldNotASibling, "after_channel_id is no live channel of the space")
		}
	}
	return domain.KeyAt(order, i)
}

// keyIndex returns the position of id in order, -1 when absent.
func keyIndex(order []domain.Keyed, id ids.ID) int {
	return slices.IndexFunc(order, func(k domain.Keyed) bool { return k.ID == id })
}

// codeError returns validation_failed with one field error of code.
func codeError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// UpdateChannel implements chat.Structure in one unit of work (channel.update is no rights
// change): it authorizes before the row lock, refuses an archived channel with 409, checks If-Match
// against the locked version and then the body, and bumps the version once. A call without change
// records no_change and appends nothing. The audit entry names the changed fields, never a text.
func (s *Service) UpdateChannel(ctx context.Context, a actor.Actor, id ids.ID, cmd chat.UpdateChannelCmd) (chat.Channel, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Channel{}, err
	}
	var out ChannelRow
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.updateChannel(ctx, a, id, cmd)
		return err
	})
	return channelDTO(out), err
}

// updateChannel is UpdateChannel inside its unit of work.
func (s *Service) updateChannel(ctx context.Context, a actor.Actor, id ids.ID, cmd chat.UpdateChannelCmd) (ChannelRow, error) {
	if _, err := s.loadChannel(ctx, a, authorize.ActionChannelUpdate, id); err != nil {
		return ChannelRow{}, err
	}
	c, err := s.lockChannel(ctx, id)
	if err != nil {
		return ChannelRow{}, err
	}
	if archived(c) {
		return ChannelRow{}, channelArchivedConflict()
	}
	if err := channelPrecondition(cmd.IfMatch, c); err != nil {
		return ChannelRow{}, err
	}
	change := domain.ChannelChange{
		Name: cmd.Name, Topic: cmd.Topic, ForumDefaultSort: cmd.ForumDefaultSort,
		DefaultNotificationLevel: cmd.DefaultNotificationLevel, ClearDefaultNotificationLevel: cmd.ClearDefaultNotificationLevel,
	}
	if err := change.Check(c.Channel.Kind); err != nil {
		return ChannelRow{}, err
	}
	lim := s.cfg.limits()
	if cmd.Name != nil {
		n, err := domain.ChannelName(*cmd.Name, lim.ChannelName)
		if err != nil {
			return ChannelRow{}, err
		}
		change.Name = &n
	}
	if cmd.Topic != nil {
		if _, err := domain.ChannelTopic(*cmd.Topic, lim.ChannelTopic); err != nil {
			return ChannelRow{}, err
		}
	}
	changed := c.Channel.Update(change)
	if len(changed) == 0 {
		return c, s.noChangeChannel(ctx, authorize.ActionChannelUpdate, id, nil)
	}
	if err := s.d.Store.SaveChannel(ctx, c.Channel); err != nil {
		return ChannelRow{}, err
	}
	if err := s.recordChannel(ctx, authorize.ActionChannelUpdate, id, auditing.Attrs{attrChangedFields: codes(changed)}); err != nil {
		return ChannelRow{}, err
	}
	return c, s.emitChannel(ctx, authorize.EventChannelUpdated, c)
}

// channelPrecondition checks If-Match against the current channel (06 section 8): no header is 428
// precondition_required, a stale or malformed one 412 version_conflict with the current channel.
func channelPrecondition(p chat.Precondition, c ChannelRow) error {
	if !p.Present {
		return errs.PreconditionRequired()
	}
	if p.Valid && p.Version == c.Channel.Version {
		return nil
	}
	return errs.VersionConflict(channelBody(c))
}

// MoveChannel implements chat.Structure in one unit of work: the visibility of the channel
// (channel.read in filter mode, so an invisible channel is 404 like an unknown one), then
// channel.reorder on its space, the position check, the lock of the space row and of the channel,
// 409 for an archived channel, a key between the neighbours and one written row (sort key and
// version), the audit entry and channel.updated.
func (s *Service) MoveChannel(ctx context.Context, a actor.Actor, id ids.ID, pos chat.MovePosition) (chat.Channel, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Channel{}, err
	}
	var out ChannelRow
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.moveChannel(ctx, a, id, domain.MovePosition{AfterID: pos.AfterID, BeforeID: pos.BeforeID})
		return err
	})
	return channelDTO(out), err
}

// moveChannel is MoveChannel inside its unit of work.
func (s *Service) moveChannel(ctx context.Context, a actor.Actor, id ids.ID, pos domain.MovePosition) (ChannelRow, error) {
	c, err := s.loadChannel(authorize.Filter(ctx), a, authorize.ActionChannelRead, id)
	if err != nil {
		return ChannelRow{}, err
	}
	if err := s.check(ctx, a, authorize.ActionChannelReorder, spaceResource(c.Channel.SpaceID)); err != nil {
		return ChannelRow{}, err
	}
	if err := pos.Check(); err != nil {
		return ChannelRow{}, err
	}
	if _, err := s.lock(ctx, c.Channel.SpaceID); err != nil {
		return ChannelRow{}, err
	}
	if c, err = s.lockChannel(ctx, id); err != nil {
		return ChannelRow{}, err
	}
	if archived(c) {
		return ChannelRow{}, channelArchivedConflict()
	}
	order, err := s.d.Store.ChannelOrder(ctx, c.Channel.SpaceID)
	if err != nil {
		return ChannelRow{}, err
	}
	order = slices.DeleteFunc(order, func(k domain.Keyed) bool { return k.ID == id })
	i, err := pos.Index(order, id)
	if err != nil {
		return ChannelRow{}, err
	}
	key, err := domain.KeyAt(order, i)
	if err != nil {
		return ChannelRow{}, err
	}
	c.Channel.Move(key)
	if err := s.d.Store.SaveChannel(ctx, c.Channel); err != nil {
		return ChannelRow{}, err
	}
	if err := s.recordChannel(ctx, authorize.ActionChannelReorder, id, nil); err != nil {
		return ChannelRow{}, err
	}
	return c, s.emitChannel(ctx, authorize.EventChannelUpdated, c)
}

// ArchiveChannel implements chat.Structure (07 section 4.5). A first unit loads, authorizes
// channel.archive (manage, step-up) and checks the reason; an archived channel is a call without
// effect that answers the channel. Otherwise a second unit, declared as the rights change
// channel.archive with the channel as target (02 section 8.1), locks the row, sets archived_at,
// bumps the version, publishes ChannelArchived, records the audit entry with the reason and
// appends channel.archived; the rights hook raises permission_version and writes access.changed
// for everyone whose level on the channel was above read.
func (s *Service) ArchiveChannel(ctx context.Context, a actor.Actor, id ids.ID, reason string) (chat.Channel, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Channel{}, err
	}
	var (
		out    ChannelRow
		change bool
	)
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		c, err := s.loadChannel(ctx, a, authorize.ActionChannelArchive, id)
		if err != nil {
			return err
		}
		if err := domain.CheckReason(reason); err != nil {
			return err
		}
		out = c
		if c.Channel.Archived() {
			return s.noChangeChannel(ctx, authorize.ActionChannelArchive, id, auditing.Attrs{attrReason: auditing.ReasonText(reason)})
		}
		change = true
		return nil
	})
	if err != nil || !change {
		return channelDTO(out), err
	}
	ref := txn.Ref{Kind: resourceChannel, ID: id}
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.archiveChannelWrite(ctx, id, reason)
		return err
	}, txn.DeclareRightsChange(txn.RightsChangeSpec{Action: string(authorize.ActionChannelArchive), Resource: ref, Targets: []txn.Ref{ref}}))
	return channelDTO(out), err
}

// archiveChannelWrite is the second unit of ArchiveChannel, declared as the rights change
// channel.archive.
func (s *Service) archiveChannelWrite(ctx context.Context, id ids.ID, reason string) (ChannelRow, error) {
	c, err := s.lockChannel(ctx, id)
	if err != nil {
		return ChannelRow{}, err
	}
	if c.Channel.Archived() {
		// A concurrent call won the race: answered as without effect (06 section 2).
		return c, s.noChangeChannel(ctx, authorize.ActionChannelArchive, id, auditing.Attrs{attrReason: auditing.ReasonText(reason)})
	}
	c.Channel.Archive(s.d.Clock.Now().UTC().Truncate(time.Microsecond))
	if err := s.d.Store.SaveChannel(ctx, c.Channel); err != nil {
		return ChannelRow{}, err
	}
	if err := txn.Publish(ctx, chat.ChannelArchived{ChannelID: id, SpaceID: c.Channel.SpaceID, ArchivedAt: c.Channel.ArchivedAt}); err != nil {
		return ChannelRow{}, err
	}
	if err := s.recordChannel(ctx, authorize.ActionChannelArchive, id, auditing.Attrs{attrReason: auditing.ReasonText(reason)}); err != nil {
		return ChannelRow{}, err
	}
	return c, s.emitChannel(ctx, authorize.EventChannelArchived, c)
}

// operatorJSON is the schema ChannelOperator.
type operatorJSON struct {
	Mode             string `json:"mode"`
	AgentID          string `json:"agent_id,omitempty"`
	EffectiveAgentID string `json:"effective_agent_id,omitempty"`
}

// channelJSON is the schema Channel: the payload member channel of the channel events and the
// member current of 412 version_conflict. Optional members are left out, never null (ADR 0053).
type channelJSON struct {
	ID                       string       `json:"id"`
	SpaceID                  string       `json:"space_id,omitempty"`
	Kind                     string       `json:"kind"`
	Name                     string       `json:"name"`
	Topic                    string       `json:"topic"`
	SortKey                  string       `json:"sort_key"`
	Operator                 operatorJSON `json:"operator"`
	ForumDefaultSort         string       `json:"forum_default_sort,omitempty"`
	DefaultNotificationLevel string       `json:"default_notification_level,omitempty"`
	ReadOnly                 bool         `json:"read_only"`
	ProjectBound             bool         `json:"project_bound"`
	MainThreadID             string       `json:"main_thread_id,omitempty"`
	ArchivedAt               string       `json:"archived_at,omitempty"`
	Version                  int64        `json:"version"`
	SettingsVersion          int64        `json:"settings_version"`
	CreatedAt                string       `json:"created_at"`
	DeletedAt                string       `json:"deleted_at,omitempty"`
}

// channelBody converts a channel row into the JSON form of the schema Channel.
func channelBody(c ChannelRow) channelJSON {
	d := channelDTO(c)
	return channelJSON{
		ID: d.ID.String(), SpaceID: optionalID(d.SpaceID), Kind: d.Kind, Name: d.Name, Topic: d.Topic, SortKey: d.SortKey,
		Operator:         operatorJSON{Mode: d.Operator.Mode, AgentID: optionalID(d.Operator.AgentID), EffectiveAgentID: optionalID(d.Operator.EffectiveAgentID)},
		ForumDefaultSort: d.ForumDefaultSort, DefaultNotificationLevel: d.DefaultNotificationLevel, ReadOnly: d.ReadOnly,
		ProjectBound: d.ProjectBound, MainThreadID: optionalID(d.MainThreadID), ArchivedAt: stamp(d.ArchivedAt), Version: d.Version,
		SettingsVersion: d.SettingsVersion, CreatedAt: stamp(d.CreatedAt), DeletedAt: stamp(d.DeletedAt),
	}
}

// emitChannel appends the channel event typ for c (05 section 6, 04 section 2.2): resource the
// channel with its version, scope space and channel, payload ChannelPayload, receive action
// channel.read.
func (s *Service) emitChannel(ctx context.Context, typ authorize.EventType, c ChannelRow) error {
	payload, err := json.Marshal(struct {
		Channel channelJSON `json:"channel"`
	}{channelBody(c)})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "chat: channel event payload")
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(typ), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID, SpaceID: c.Channel.SpaceID,
		ChannelID: c.Channel.ID, ResourceKind: resourceChannel, ResourceID: c.Channel.ID, ResourceVersion: c.Channel.Version,
		ReceiveAction: receiveChannelRead, Payload: payload,
	})
}
