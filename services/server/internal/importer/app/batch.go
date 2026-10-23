// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/identity"
	"github.com/OWNER/bothy/services/server/internal/importer"
	"github.com/OWNER/bothy/services/server/internal/importer/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// Target kinds of import_items.target_kind for the structure items.
const (
	targetSpace    = "space"
	targetChannel  = "channel"
	targetForumTag = "forum_tag"
	targetUser     = "user"
	targetAgent    = "agent"
)

// parsed is one item of a batch after the checks of design note 5 a (M8-03) and design note 2
// (M8-11).
type parsed struct {
	obj    importer.BatchObject
	key    domain.OriginKey
	sha    []byte
	space  domain.SpaceData
	chann  domain.ChannelData
	tag    domain.ForumTagData
	user   domain.UserData
	thread domain.ThreadData
	msg    domain.MessageData
	pin    domain.PinData
}

// parseBatch checks the batch before anything is written (M8-03 design note 5 a, M8-11 design
// note 1): a batch without thread_origin_key holds structure items only (space, channel,
// forum_tag, user); a thread batch has the shape of domain.CheckThreadBatch. The origin key must
// match its kind; content_sha256 is 64 hex digits; data follows the schema of its kind. Every
// violation is reported at once as 400 validation_failed. The thread key is nil for a structure
// batch.
func parseBatch(b importer.Batch) ([]parsed, *domain.ThreadKey, error) {
	var p domain.Problems
	var tk *domain.ThreadKey
	if b.ThreadOriginKeySet {
		refs := make([]domain.ItemRef, 0, len(b.Items))
		for _, it := range b.Items {
			refs = append(refs, domain.ItemRef{Kind: it.ItemKind, OriginKey: it.OriginKey})
		}
		k := domain.CheckThreadBatch(b.ThreadOriginKey, refs, &p)
		tk = &k
	}
	out := make([]parsed, 0, len(b.Items))
	for i, it := range b.Items {
		base := domain.Pointer("/items", i)
		x := parsed{obj: it}
		switch {
		case tk == nil && !domain.Structure(it.ItemKind):
			p.Add(base+"/item_kind", domain.FieldInvalid, "a thread, message or pin item belongs into a batch with thread_origin_key")
			continue
		case tk != nil && !domain.Content(it.ItemKind):
			// CheckThreadBatch recorded it.
			continue
		}
		key, code := domain.ParseOriginKey(it.ItemKind, it.OriginKey)
		if code != "" {
			p.Add(base+"/origin_key", code, "the origin key does not fit the item kind")
		}
		x.key = key
		if it.ContentSHA256 != "" {
			sum, err := hex.DecodeString(it.ContentSHA256)
			if err != nil || len(sum) != 32 {
				p.Add(base+"/content_sha256", domain.FieldInvalid, "a hex SHA-256 is required")
			}
			x.sha = sum
		}
		parseData(&x, base+"/data", &p)
		out = append(out, x)
	}
	if err := p.Err(); err != nil {
		return nil, nil, err
	}
	return out, tk, nil
}

// parseData checks the data of x by its kind.
func parseData(x *parsed, data string, p *domain.Problems) {
	switch x.obj.ItemKind {
	case domain.KindSpace:
		x.space = domain.ParseSpaceData(x.obj.Data, data, p)
	case domain.KindChannel:
		x.chann = domain.ParseChannelData(x.obj.Data, data, p)
	case domain.KindForumTag:
		x.tag = domain.ParseForumTagData(x.obj.Data, data, p)
	case domain.KindUser:
		x.user = domain.ParseUserData(x.obj.Data, data, p)
	case domain.KindThread:
		x.thread = domain.ParseThreadData(x.obj.Data, data, p)
	case domain.KindMessage:
		x.msg = domain.ParseMessageData(x.obj.Data, data, p)
	case domain.KindPin:
		x.pin = domain.ParsePinData(x.obj.Data, data, p)
	}
}

// batch is the state of one batch inside its unit of work.
type batch struct {
	src    importer.Source
	m      domain.Mapping
	counts domain.Counts
	// changed forces the counters and import.progress of a thread batch whose only change is a
	// mirrored thread (M8-11 design note 5).
	changed bool
}

// ApplyBatch implements importer.Imports (design note 5, 22 7.2 and 8). The checks of parseBatch
// run first; then one unit of work: import.apply on the source (an owner; authorize records its
// denials), the row lock (batches, state changes and the cursor of a source are serialized), 404
// for an unknown source and 409 import_not_running unless it is running, then every item in
// request order. Afterwards the counters of the new transitions are added to stats.server and one
// import.progress is appended; a batch that made no new transition writes neither (a replayed
// batch has no event). A refusal of import.apply by the module is recorded as a denial.
func (s *Service) ApplyBatch(ctx context.Context, a actor.Actor, id ids.ID, b importer.Batch) (importer.BatchResult, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return importer.BatchResult{}, err
	}
	items, tk, err := parseBatch(b)
	if err != nil {
		return importer.BatchResult{}, err
	}
	var out importer.BatchResult
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.applyBatch(ctx, a, id, items, tk)
		return err
	})
	if err != nil {
		return importer.BatchResult{}, err
	}
	return out, nil
}

// applyBatch is ApplyBatch inside its unit of work; tk is the thread key of a thread batch.
func (s *Service) applyBatch(ctx context.Context, a actor.Actor, id ids.ID, items []parsed, tk *domain.ThreadKey) (importer.BatchResult, error) {
	src, err := s.load(ctx, a, authorize.ActionImportApply, id, true)
	if err != nil {
		return importer.BatchResult{}, err
	}
	if src.State != importer.StateRunning {
		s.deny(ctx, id, importer.ReasonImportNotRunning)
		return importer.BatchResult{}, conflict(importer.ReasonImportNotRunning, "the source is not running")
	}
	m, ok := domain.StoredMapping(src.Config)
	if !ok {
		return importer.BatchResult{}, errs.New(errs.CodeInternalError, "importer: the stored mapping does not parse")
	}
	bt := &batch{src: src, m: m}
	if tk != nil {
		return s.applyThreadBatch(ctx, a, bt, *tk, items)
	}
	out := importer.BatchResult{Items: make([]importer.Item, 0, len(items))}
	for i, x := range items {
		it, err := s.applyItem(ctx, bt, x)
		if err != nil {
			if e, ok := errs.As(err); ok && e.Reason == importer.ReasonTargetOutsideSource {
				s.deny(ctx, id, importer.ReasonTargetOutsideSource)
			}
			return importer.BatchResult{}, itemError(err, i)
		}
		out.Items = append(out.Items, it)
	}
	return out, s.count(ctx, bt)
}

// count adds the counters of the batch to stats.server and appends import.progress; a batch
// without a new transition and without a mirrored change writes neither.
func (s *Service) count(ctx context.Context, bt *batch) error {
	if bt.counts.Zero() && !bt.changed {
		return nil
	}
	var err error
	if bt.src.Stats, err = domain.AddCounts(bt.src.Stats, bt.counts); err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "importer: stats")
	}
	if err := s.d.Store.SaveSource(ctx, bt.src); err != nil {
		return err
	}
	return s.changed(ctx, bt.src)
}

// itemError puts the field pointers of a validation error of item i under /items/<i>/data; other
// errors pass unchanged.
func itemError(err error, i int) error {
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeValidationFailed {
		return err
	}
	base := domain.Pointer("/items", i) + "/data"
	fields := make([]errs.FieldError, 0, len(e.Fields))
	for _, f := range e.Fields {
		f.Pointer = base + f.Pointer
		fields = append(fields, f)
	}
	if len(fields) == 0 {
		fields = append(fields, errs.FieldError{Pointer: base, Code: domain.FieldInvalid, Detail: e.Detail})
	}
	return errs.New(errs.CodeValidationFailed, e.Detail).WithFields(fields...)
}

// outside is 409 target_outside_source (05 4.12 import.apply).
func outside(detail string) error {
	return conflict(importer.ReasonTargetOutsideSource, detail)
}

// applyItem writes one item (design note 5 b and c): a new key gets its pending row, its target
// through the port of the owning module and posted (or skipped) in the same unit; an existing key
// is locked and answered with its stored item, with source_changed when the hash differs.
func (s *Service) applyItem(ctx context.Context, bt *batch, x parsed) (importer.Item, error) {
	row, created, err := s.d.Store.InsertPendingItem(ctx, NewItem{
		ID: ids.New(), SourceID: bt.src.ID, OriginKey: x.obj.OriginKey, ItemKind: x.obj.ItemKind, ContentSHA256: x.sha, At: s.now(),
	})
	if err != nil {
		return importer.Item{}, err
	}
	if !created {
		return s.replay(ctx, bt, x)
	}
	kind, target, err := s.target(ctx, bt, x)
	if err != nil {
		return importer.Item{}, err
	}
	state := importer.ItemPosted
	if target.IsZero() {
		state, kind = importer.ItemSkipped, ""
	}
	if row, err = s.d.Store.FinishItem(ctx, row.ID, state, kind, target, s.now()); err != nil {
		return importer.Item{}, err
	}
	bt.counts.Add(x.obj.ItemKind, state)
	return itemDTO(row, false), nil
}

// replay answers a key that exists: the row is locked (a concurrent batch with the same key has
// committed by now), and a different content hash is counted as changed at the source without any
// other change (22 7.2 item 4).
func (s *Service) replay(ctx context.Context, bt *batch, x parsed) (importer.Item, error) {
	row, found, err := s.d.Store.LockItem(ctx, bt.src.ID, x.obj.OriginKey)
	if err != nil {
		return importer.Item{}, err
	}
	if !found || row.ItemKind != x.obj.ItemKind {
		return importer.Item{}, errs.New(errs.CodeInternalError, "importer: an existing origin key vanished or changed its kind")
	}
	changed := x.sha != nil && row.ContentSHA256 != nil && !bytes.Equal(x.sha, row.ContentSHA256)
	if changed {
		bt.counts.SourceChanged()
	}
	return itemDTO(row, changed), nil
}

// target creates or adopts the target of a new item; a zero id means skipped.
func (s *Service) target(ctx context.Context, bt *batch, x parsed) (string, ids.ID, error) {
	switch x.obj.ItemKind {
	case domain.KindSpace:
		id, err := s.space(ctx, bt, x)
		return targetSpace, id, err
	case domain.KindChannel:
		id, err := s.channel(ctx, bt, x)
		return targetChannel, id, err
	case domain.KindForumTag:
		id, err := s.forumTag(ctx, bt, x)
		return targetForumTag, id, err
	}
	return s.person(ctx, bt, x)
}

// existingOrOutside turns not_found of an adopted object into target_outside_source: the mapping
// named it, but it was archived since.
func existingOrOutside(err error) error {
	if e, ok := errs.As(err); ok && e.Code == errs.CodeNotFound {
		return outside("the mapped object is no longer live")
	}
	return err
}

// space creates or adopts the space of a category (22 5.1 rule 1): only a category of the
// mapping; skip is skipped; an existing space must be the one the mapping names; a new space takes
// the name of the mapping.
func (s *Service) space(ctx context.Context, bt *batch, x parsed) (ids.ID, error) {
	e := bt.m.Space(x.key.IDs[0])
	if e == nil {
		return ids.ID{}, outside("the category is not in the mapping")
	}
	if e.Target.Skip {
		return ids.ID{}, nil
	}
	if !e.Target.Existing.IsZero() {
		if x.space.ExistingSpaceID != e.Target.Existing {
			return ids.ID{}, outside("existing_space_id is not the space of the mapping")
		}
		existing := e.Target.Existing
		id, err := s.d.Structure.Space(ctx, chat.ImportedSpace{ExistingID: &existing})
		return id, existingOrOutside(err)
	}
	if !x.space.ExistingSpaceID.IsZero() {
		return ids.ID{}, outside("the mapping creates this space")
	}
	return s.d.Structure.Space(ctx, chat.ImportedSpace{Name: e.Target.CreateName, SortKey: x.space.SortKey})
}

// channel creates or adopts the channel (22 5.1 rules 1 to 3): only a channel of the mapping;
// skip is skipped; read_only follows the stored lead; an existing channel must be the one the
// mapping names; a new channel lies in the target of its posted space item (the category of the
// data, or defaults.uncategorized_space without one) and takes the name of the mapping.
func (s *Service) channel(ctx context.Context, bt *batch, x parsed) (ids.ID, error) {
	e := bt.m.Channel(x.key.IDs[0])
	if e == nil {
		return ids.ID{}, outside("the channel is not in the mapping")
	}
	if e.Target.Skip {
		return ids.ID{}, nil
	}
	readOnly := e.Lead == domain.LeadDiscord
	if !e.Target.Existing.IsZero() {
		if x.chann.ExistingChannelID != e.Target.Existing {
			return ids.ID{}, outside("existing_channel_id is not the channel of the mapping")
		}
		existing := e.Target.Existing
		id, err := s.d.Structure.Channel(ctx, chat.ImportedChannel{ExistingID: &existing, ReadOnly: readOnly})
		return id, existingOrOutside(err)
	}
	if !x.chann.ExistingChannelID.IsZero() {
		return ids.ID{}, outside("the mapping creates this channel")
	}
	spaceKey := x.chann.SpaceOriginKey
	if spaceKey == "" {
		if bt.m.Defaults.UncategorizedSpace == "skip" || bt.m.Defaults.UncategorizedSpace == "" {
			return ids.ID{}, nil
		}
		spaceKey = domain.Key(domain.KindSpace, bt.m.Defaults.UncategorizedSpace)
	}
	sp, found, err := s.d.Store.Item(ctx, bt.src.ID, spaceKey)
	switch {
	case err != nil:
		return ids.ID{}, err
	case !found || sp.State == importer.ItemPending || sp.State == importer.ItemFailed:
		return ids.ID{}, outside("the space of the channel is not imported")
	case sp.State == importer.ItemSkipped:
		return ids.ID{}, nil
	case !x.chann.ExistingSpaceID.IsZero() && x.chann.ExistingSpaceID != sp.TargetID:
		return ids.ID{}, outside("existing_space_id is not the space of the category")
	}
	return s.d.Structure.Channel(ctx, chat.ImportedChannel{
		SpaceID: sp.TargetID, Name: e.Target.CreateName, Kind: x.chann.Kind, Topic: x.chann.Topic, SortKey: x.chann.SortKey, ReadOnly: readOnly,
	})
}

// forumTag creates or adopts a tag in the forum of the posted channel item of its key (22 6.6).
func (s *Service) forumTag(ctx context.Context, bt *batch, x parsed) (ids.ID, error) {
	channelKey := domain.Key(domain.KindChannel, x.key.IDs[0])
	if x.tag.ChannelOriginKey != channelKey {
		return ids.ID{}, outside("channel_origin_key is not the channel of the origin key")
	}
	ch, found, err := s.d.Store.Item(ctx, bt.src.ID, channelKey)
	if err != nil {
		return ids.ID{}, err
	}
	if !found || ch.State != importer.ItemPosted || ch.TargetID.IsZero() {
		return ids.ID{}, outside("the channel of the tag is not imported")
	}
	return s.d.Structure.ForumTag(ctx, ch.TargetID, chat.ImportedTag{Name: x.tag.Name})
}

// person maps a Discord author (22 5.3): a mapped person or agent by handle, a placeholder, or
// skipped.
func (s *Service) person(ctx context.Context, bt *batch, x parsed) (string, ids.ID, error) {
	t := bt.m.ResolveUser(x.key.IDs[0], x.user)
	switch {
	case t.Skip:
		return "", ids.ID{}, nil
	case t.Kind == domain.UserPerson || t.Kind == domain.UserAgent:
		kind, target := actor.KindUser, targetUser
		if t.Kind == domain.UserAgent {
			kind, target = actor.KindAgent, targetAgent
		}
		id, err := s.d.People.Resolve(ctx, identity.ImportRef{Kind: kind, Handle: t.Handle})
		if e, ok := errs.As(err); ok && e.Code == errs.CodeNotFound {
			return "", ids.ID{}, outside("the mapped principal does not exist")
		}
		return target, id, err
	}
	id, err := s.placeholder(ctx, x.user)
	return targetUser, id, err
}

// placeholder creates the placeholder person of u with the first free handle of the counter
// rule (design note 8): a taken handle, also one a concurrent batch of another source took between
// HandleFree and the insert (a conflict on /handle that leaves the unit usable), moves on to the
// next counter. The loop ends: every skipped round names a handle a principal holds.
func (s *Service) placeholder(ctx context.Context, u domain.UserData) (ids.ID, error) {
	name := u.Username
	if strings.TrimSpace(name) == "" {
		name = u.GlobalName
	}
	for n := 1; ; n++ {
		h := domain.PlaceholderHandle(name, n, s.cfg.HandleMaxChars)
		free, err := s.d.People.HandleFree(ctx, h)
		if err != nil {
			return ids.ID{}, err
		}
		if !free {
			continue
		}
		display := domain.PlaceholderDisplayName(u.GlobalName, u.Username, h, s.cfg.DisplayNameMaxChars)
		id, err := s.d.People.Placeholder(ctx, identity.PlaceholderSpec{Handle: h, DisplayName: display})
		if handleTaken(err) {
			continue
		}
		return id, err
	}
}

// handleTaken reports the conflict on /handle of identity.ImportPeople.Placeholder.
func handleTaken(err error) bool {
	e, ok := errs.As(err)
	return ok && e.Code == errs.CodeConflict && len(e.Fields) == 1 && e.Fields[0].Pointer == "/handle"
}
