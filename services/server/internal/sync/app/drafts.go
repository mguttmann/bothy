// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// channelKindForum is the kind of a channel whose new posts have drafts (chat.Channel.Kind).
const channelKindForum = "forum"

// PutThreadDraft implements sync.Drafts (brief M1-61 notes 7 to 10): draft.write on the thread
// (read suffices, so read-only and archived channels save drafts), the form of the draft, the
// reply target in the same thread, then the upsert or, for an empty draft, the delete.
func (s Drafts) PutThreadDraft(ctx context.Context, cmd sync.PutDraft) (sync.DraftPutResult, error) {
	a := caller(ctx)
	in := draftInput(cmd, false)
	var out sync.DraftPutResult
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		info, err := s.threadFor(ctx, a, authorize.ActionDraftWrite, cmd.ThreadID)
		if err != nil {
			return err
		}
		if err := domain.ValidateDraft(in, s.limits()); err != nil {
			return err
		}
		if cmd.ReplyToID != nil {
			ok, err := s.d.Replies.InThread(ctx, *cmd.ReplyToID, cmd.ThreadID)
			if err != nil {
				return err
			}
			if !ok {
				return domain.NotInThread()
			}
		}
		out, err = s.putDraft(ctx, a, info.SpaceID, draftOf(in, cmd.ThreadID, info.ChannelID), in.Empty(), cmd.BaseVersion)
		return err
	})
	return out, err
}

// PutChannelDraft implements sync.Drafts: draft.write on the channel, which must be a forum
// (a text channel drafts on its main thread), no reply target, then as PutThreadDraft.
func (s Drafts) PutChannelDraft(ctx context.Context, cmd sync.PutDraft) (sync.DraftPutResult, error) {
	a := caller(ctx)
	in := draftInput(cmd, true)
	var out sync.DraftPutResult
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		space, kind, err := s.channelFor(ctx, a, cmd.ChannelID)
		if err != nil {
			return err
		}
		if kind != channelKindForum {
			return domain.NotAForum()
		}
		if err := domain.ValidateDraft(in, s.limits()); err != nil {
			return err
		}
		out, err = s.putDraft(ctx, a, space, draftOf(in, ids.ID{}, cmd.ChannelID), in.Empty(), cmd.BaseVersion)
		return err
	})
	return out, err
}

// DeleteThreadDraft implements sync.Drafts (brief M1-61 note 11): draft.write on the thread,
// then the delete with draft.deleted; no draft is no error.
func (s Drafts) DeleteThreadDraft(ctx context.Context, threadID ids.ID) error {
	a := caller(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		info, err := s.threadFor(ctx, a, authorize.ActionDraftWrite, threadID)
		if err != nil {
			return err
		}
		return s.deleteDraft(ctx, a, info.SpaceID, threadID, ids.ID{})
	})
}

// DeleteChannelDraft implements sync.Drafts: draft.write on the channel, then the delete of the
// channel draft with draft.deleted; no draft is no error.
func (s Drafts) DeleteChannelDraft(ctx context.Context, channelID ids.ID) error {
	a := caller(ctx)
	return s.d.UoW.Do(ctx, func(ctx context.Context) error {
		space, _, err := s.channelFor(ctx, a, channelID)
		if err != nil {
			return err
		}
		return s.deleteDraft(ctx, a, space, ids.ID{}, channelID)
	})
}

// List implements sync.Drafts (brief M1-61 note 13): draft.read on the caller, the drafts in the
// visible channels of thread.read, each checked in filter mode with thread.read on its thread or
// channel.read on its channel. Drafts of lost channels stay stored and are not listed.
func (s Drafts) List(ctx context.Context) ([]sync.Draft, error) {
	a := caller(ctx)
	if err := s.check(ctx, a, authorize.ActionDraftRead, self(a.ID)); err != nil {
		return nil, err
	}
	out := []sync.Draft{}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionThreadRead)
	if err != nil || len(sc.ChannelIDs) == 0 {
		return out, err
	}
	rows, err := s.d.Store.DraftsInChannels(ctx, a.ID, sc.ChannelIDs)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	for _, d := range rows {
		act, r := authorize.ActionThreadRead, authorize.Resource{Kind: authorize.ResourceThread, ID: d.ThreadID}
		if d.ThreadID.IsZero() {
			act, r = authorize.ActionChannelRead, authorize.Resource{Kind: authorize.ResourceChannel, ID: d.ChannelID}
		}
		ok, err := s.allowed(fctx, a, act, r)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, d)
		}
	}
	return out, nil
}

// channelFor authorizes draft.write on the channel as the caller's own draft and returns its
// space and kind. An unknown and an invisible channel give the same 404.
func (s *Personal) channelFor(ctx context.Context, a actor.Actor, channel ids.ID) (ids.ID, string, error) {
	if err := s.check(ctx, a, authorize.ActionDraftWrite, own(authorize.ResourceChannel, channel, a.ID)); err != nil {
		return ids.ID{}, "", err
	}
	v, err := s.d.Channels.GetChannel(ctx, a, channel)
	if err != nil {
		return ids.ID{}, "", err
	}
	return v.Channel.SpaceID, v.Channel.Kind, nil
}

// limits returns the draft limits of the configuration.
func (s *Personal) limits() domain.DraftLimits {
	return domain.DraftLimits{MaxContentBytes: s.cfg.MaxContentBytes, MaxAttachments: s.cfg.MaxAttachments}
}

// draftInput converts the request for the domain rules.
func draftInput(cmd sync.PutDraft, channel bool) domain.DraftInput {
	in := domain.DraftInput{ContentMD: cmd.ContentMD, FileIDs: cmd.FileIDs, Channel: channel}
	if cmd.ReplyToID != nil {
		in.ReplyToID = *cmd.ReplyToID
	}
	return in
}

// draftOf returns the row to write for in: a thread draft, or with a zero thread a channel draft.
func draftOf(in domain.DraftInput, thread, channel ids.ID) sync.Draft {
	files := in.FileIDs
	if files == nil {
		files = []ids.ID{}
	}
	return sync.Draft{ThreadID: thread, ChannelID: channel, ContentMD: in.ContentMD, ReplyToID: in.ReplyToID, FileIDs: files}
}

// putDraft writes d for the caller (brief M1-61 notes 9 and 10). A non-empty draft is upserted:
// a returned version 1 is a new row, a larger one replaced the version before it under the row
// lock, from which overwrote_newer follows. An empty draft deletes the row like DELETE and
// answers the empty final state with the last version plus one (1 without a row).
func (s *Personal) putDraft(ctx context.Context, a actor.Actor, space ids.ID, d sync.Draft, empty bool, base *int64) (sync.DraftPutResult, error) {
	now := s.now()
	if empty {
		old, found, err := s.d.Store.DeleteDraft(ctx, a.ID, d.ThreadID, channelKey(d))
		if err != nil {
			return sync.DraftPutResult{}, err
		}
		final := sync.Draft{ID: ids.New(), ThreadID: d.ThreadID, ChannelID: d.ChannelID, FileIDs: []ids.ID{}, Version: 1,
			UpdatedByDeviceID: a.DeviceID, UpdatedAt: now}
		if found {
			final.ID, final.ChannelID, final.Version = old.ID, old.ChannelID, old.Version+1
			if err := s.appendDraftDeleted(ctx, a, space, old); err != nil {
				return sync.DraftPutResult{}, err
			}
		}
		return sync.DraftPutResult{Draft: final, OverwroteNewer: domain.OverwroteNewer(found, base, old.Version)}, nil
	}
	d.ID, d.UpdatedByDeviceID, d.UpdatedAt = ids.New(), a.DeviceID, now
	row, err := s.d.Store.UpsertDraft(ctx, a.ID, d)
	if err != nil {
		return sync.DraftPutResult{}, err
	}
	existed := row.Version > 1
	res := sync.DraftPutResult{Draft: row, OverwroteNewer: domain.OverwroteNewer(existed, base, row.Version-1)}
	return res, s.appendDraftUpdated(ctx, a, space, row)
}

// channelKey is the channel argument of DeleteDraft: the channel of a channel draft, zero for a
// thread draft.
func channelKey(d sync.Draft) ids.ID {
	if d.ThreadID.IsZero() {
		return d.ChannelID
	}
	return ids.ID{}
}

// deleteDraft deletes the draft of the caller and appends draft.deleted when one existed.
func (s *Personal) deleteDraft(ctx context.Context, a actor.Actor, space, thread, channel ids.ID) error {
	old, found, err := s.d.Store.DeleteDraft(ctx, a.ID, thread, channel)
	if err != nil || !found {
		return err
	}
	return s.appendDraftDeleted(ctx, a, space, old)
}

// appendDraftDeleted appends draft.deleted of the deleted row d for the caller.
func (s *Personal) appendDraftDeleted(ctx context.Context, a actor.Actor, space ids.ID, d sync.Draft) error {
	e, err := draftDeletedEvent(a.ID, space, d)
	if err != nil {
		return err
	}
	return s.d.Events.Append(ctx, e)
}

// draftJSON is the schema Draft; absent members are omitted, never null.
type draftJSON struct {
	ID                string   `json:"id"`
	ThreadID          string   `json:"thread_id,omitempty"`
	ChannelID         string   `json:"channel_id"`
	ContentMD         string   `json:"content_md"`
	ReplyToMessageID  string   `json:"reply_to_message_id,omitempty"`
	FileIDs           []string `json:"file_ids"`
	Version           int64    `json:"version"`
	UpdatedByDeviceID string   `json:"updated_by_device_id,omitempty"`
	UpdatedAt         string   `json:"updated_at,omitempty"`
}

// draftBody converts d into the schema Draft.
func draftBody(d sync.Draft) draftJSON {
	files := make([]string, 0, len(d.FileIDs))
	for _, f := range d.FileIDs {
		files = append(files, f.String())
	}
	return draftJSON{
		ID: d.ID.String(), ThreadID: optionalID(d.ThreadID), ChannelID: d.ChannelID.String(), ContentMD: d.ContentMD,
		ReplyToMessageID: optionalID(d.ReplyToID), FileIDs: files, Version: d.Version,
		UpdatedByDeviceID: optionalID(d.UpdatedByDeviceID), UpdatedAt: stamp(d.UpdatedAt),
	}
}

// draftPayload encodes DraftPayload of body.
func draftPayload(body draftJSON) ([]byte, error) {
	return marshal(struct {
		Draft draftJSON `json:"draft"`
	}{body})
}

// appendDraftUpdated appends draft.updated of the stored row d (brief M1-61 notes 19 and 21):
// scope space, channel and, for a thread draft, thread; resource the draft with its version. A
// payload that is not smaller than sync.event_payload_max_bytes gets the longest prefix of
// content_md that fits and the flag truncated; clients reload through listMyDrafts.
func (s *Personal) appendDraftUpdated(ctx context.Context, a actor.Actor, space ids.ID, d sync.Draft) error {
	body := draftBody(d)
	p, err := draftPayload(body)
	if err != nil {
		return err
	}
	truncated := false
	if len(p) >= s.cfg.EventPayloadMaxBytes {
		var cut string
		cut, truncated = domain.TruncateContent(body.ContentMD, func(prefix string) bool {
			b := body
			b.ContentMD = prefix
			enc, err := draftPayload(b)
			return err == nil && len(enc) < s.cfg.EventPayloadMaxBytes
		})
		body.ContentMD = cut
		if p, err = draftPayload(body); err != nil {
			return err
		}
	}
	e := personalEvent(a, authorize.EventDraftUpdated, p)
	e.SpaceID, e.ChannelID, e.ThreadID = space, d.ChannelID, d.ThreadID
	e.ResourceKind, e.ResourceID, e.ResourceVersion = resourceDraft, d.ID, d.Version
	e.Truncated = truncated
	return s.d.Events.Append(ctx, e)
}
