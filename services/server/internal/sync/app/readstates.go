// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/sync"
	"github.com/OWNER/bothy/services/server/internal/sync/domain"
)

// Advance implements sync.ReadStates (brief M1-61 notes 1 and 2): read_state.update on the
// thread, the range 0..last_message_seq under FOR SHARE on the thread, then GREATEST in one
// upsert. No change answers the current state without event; a change appends
// read_state.updated with the mention count of the new position.
func (s *Personal) Advance(ctx context.Context, cmd sync.AdvanceReadState) (sync.ReadState, error) {
	a := caller(ctx)
	var out sync.ReadState
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.advance(ctx, a, cmd)
		return err
	})
	return out, err
}

// advance is Advance inside its unit of work.
func (s *Personal) advance(ctx context.Context, a actor.Actor, cmd sync.AdvanceReadState) (sync.ReadState, error) {
	info, err := s.threadFor(ctx, a, authorize.ActionReadStateUpdate, cmd.ThreadID)
	if err != nil {
		return sync.ReadState{}, err
	}
	if err := domain.CheckReadSeq(cmd.LastReadSeq, info.LastMessageSeq); err != nil {
		return sync.ReadState{}, err
	}
	row, changed := sync.ReadState{}, false
	if cmd.LastReadSeq > 0 {
		row, changed, err = s.d.Store.AdvanceReadState(ctx, a.ID, sync.ReadState{
			ThreadID: cmd.ThreadID, ChannelID: info.ChannelID, LastReadSeq: cmd.LastReadSeq, UpdatedByDeviceID: a.DeviceID, UpdatedAt: s.now(),
		})
		if err != nil {
			return sync.ReadState{}, err
		}
	}
	if !changed {
		if row, err = s.current(ctx, a, info, false); err != nil {
			return sync.ReadState{}, err
		}
	}
	return s.answer(ctx, a, info, row, changed)
}

// answer returns row with its mention count and, when changed, appends read_state.updated.
func (s *Personal) answer(ctx context.Context, a actor.Actor, info chat.ThreadInfo, row sync.ReadState, changed bool) (sync.ReadState, error) {
	out, err := s.withMentions(ctx, a, info, row)
	if err != nil || !changed {
		return out, err
	}
	return out, s.appendReadState(ctx, a, info, out)
}

// MarkUnread implements sync.ReadStates (brief M1-61 note 3): If-Match is required; the version
// of the locked row (0 for none) must match, otherwise 412 with the current state; the value must
// be at most the current position. An equal value answers without event.
func (s *Personal) MarkUnread(ctx context.Context, cmd sync.MarkUnread) (sync.ReadState, error) {
	if !cmd.IfMatch.Present {
		return sync.ReadState{}, errs.PreconditionRequired()
	}
	a := caller(ctx)
	var out sync.ReadState
	err := s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.markUnread(ctx, a, cmd)
		return err
	})
	return out, err
}

// markUnread is MarkUnread inside its unit of work.
func (s *Personal) markUnread(ctx context.Context, a actor.Actor, cmd sync.MarkUnread) (sync.ReadState, error) {
	info, err := s.threadFor(ctx, a, authorize.ActionReadStateMarkUnread, cmd.ThreadID)
	if err != nil {
		return sync.ReadState{}, err
	}
	cur, err := s.current(ctx, a, info, true)
	if err != nil {
		return sync.ReadState{}, err
	}
	if !cmd.IfMatch.Valid || cmd.IfMatch.Version != cur.Version {
		view, err := s.withMentions(ctx, a, info, cur)
		if err != nil {
			return sync.ReadState{}, err
		}
		return sync.ReadState{}, errs.VersionConflict(readStateBody(view))
	}
	changed, err := domain.MarkUnread(cur.LastReadSeq, cmd.LastReadSeq)
	if err != nil {
		return sync.ReadState{}, err
	}
	row := cur
	if changed {
		if row, err = s.d.Store.SetReadState(ctx, a.ID, cmd.ThreadID, cmd.LastReadSeq, a.DeviceID, s.now()); err != nil {
			return sync.ReadState{}, err
		}
	}
	return s.answer(ctx, a, info, row, changed)
}

// ListForChannel implements sync.ReadStates (brief M1-61 note 6): settings.read on the caller,
// then the rows of the channel only when it is in the visible set of thread.read, each row
// checked with thread.read in filter mode. An invisible and an unknown channel answer the same
// empty list.
func (s *Personal) ListForChannel(ctx context.Context, channelID ids.ID) ([]sync.ReadState, error) {
	a := caller(ctx)
	if err := s.check(ctx, a, authorize.ActionSettingsRead, self(a.ID)); err != nil {
		return nil, err
	}
	out := []sync.ReadState{}
	sc, err := s.d.Authorizer.Scope(ctx, a, authorize.ActionThreadRead)
	if err != nil {
		return nil, err
	}
	if !containsID(sc.ChannelIDs, channelID) {
		return out, nil
	}
	rows, err := s.d.Store.ReadStatesOfChannel(ctx, a.ID, channelID)
	if err != nil {
		return nil, err
	}
	fctx := authorize.Filter(ctx)
	seqs := map[ids.ID]int64{}
	for _, r := range rows {
		ok, err := s.allowed(fctx, a, authorize.ActionThreadRead, authorize.Resource{Kind: authorize.ResourceThread, ID: r.ThreadID})
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, r)
			seqs[r.ThreadID] = r.LastReadSeq
		}
	}
	if len(out) == 0 {
		return out, nil
	}
	counts, err := s.d.Mentions.CountUnread(ctx, messaging.UnreadMentionQuery{PrincipalID: a.ID, VisibleChannelIDs: []ids.ID{channelID}, LastReadSeq: seqs})
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].MentionCount = countOf(counts, out[i].ThreadID)
	}
	return out, nil
}

// threadFor authorizes act on the thread as the caller's own read state and returns its routing
// data under FOR SHARE (brief M1-61 note 23). An unknown and an invisible thread give the same
// 404.
func (s *Personal) threadFor(ctx context.Context, a actor.Actor, act authorize.Action, thread ids.ID) (chat.ThreadInfo, error) {
	if err := s.check(ctx, a, act, own(authorize.ResourceThread, thread, a.ID)); err != nil {
		return chat.ThreadInfo{}, err
	}
	return s.d.Threads.InfoForShare(ctx, thread)
}

// current returns the stored read state of the caller in the thread of info, or the implicit
// state (position 0, version 0) without a row; lock locks the row.
func (s *Personal) current(ctx context.Context, a actor.Actor, info chat.ThreadInfo, lock bool) (sync.ReadState, error) {
	get := s.d.Store.ReadState
	if lock {
		get = s.d.Store.LockReadState
	}
	row, found, err := get(ctx, a.ID, info.ID)
	if err != nil {
		return sync.ReadState{}, err
	}
	if !found {
		return sync.ReadState{ThreadID: info.ID, ChannelID: info.ChannelID}, nil
	}
	return row, nil
}

// withMentions returns rs with the unread mentions of the caller above its position, computed
// now and never stored (R-DM-32). It counts above last_read_seq like CountUnread does for every
// reader: imports write no mentions, so threads.unread_floor_seq hides none (M1-60/2).
func (s *Personal) withMentions(ctx context.Context, a actor.Actor, info chat.ThreadInfo, rs sync.ReadState) (sync.ReadState, error) {
	from := rs.LastReadSeq
	counts, err := s.d.Mentions.CountUnread(ctx, messaging.UnreadMentionQuery{
		PrincipalID: a.ID, VisibleChannelIDs: []ids.ID{info.ChannelID}, LastReadSeq: map[ids.ID]int64{info.ID: from},
	})
	if err != nil {
		return sync.ReadState{}, err
	}
	rs.MentionCount = countOf(counts, info.ID)
	return rs, nil
}

// countOf returns the count of thread in counts; 0 when absent.
func countOf(counts []messaging.ThreadMentionCount, thread ids.ID) int {
	for _, c := range counts {
		if c.ThreadID == thread {
			return c.Count
		}
	}
	return 0
}

// containsID reports whether id is in list.
func containsID(list []ids.ID, id ids.ID) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

// readStateJSON is the schema ReadState; absent members are omitted, never null.
type readStateJSON struct {
	ThreadID          string `json:"thread_id"`
	ChannelID         string `json:"channel_id"`
	LastReadSeq       int64  `json:"last_read_seq"`
	MentionCount      int    `json:"mention_count"`
	Version           int64  `json:"version"`
	UpdatedByDeviceID string `json:"updated_by_device_id,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

// readStateBody converts rs into the schema ReadState (also the current of a 412).
func readStateBody(rs sync.ReadState) readStateJSON {
	return readStateJSON{
		ThreadID: rs.ThreadID.String(), ChannelID: rs.ChannelID.String(), LastReadSeq: rs.LastReadSeq, MentionCount: rs.MentionCount,
		Version: rs.Version, UpdatedByDeviceID: optionalID(rs.UpdatedByDeviceID), UpdatedAt: stamp(rs.UpdatedAt),
	}
}

// appendReadState appends read_state.updated for the caller (brief M1-61 note 19): scope space,
// channel and thread, resource the thread without a version (04 section 9 point 3), payload
// ReadStatePayload.
func (s *Personal) appendReadState(ctx context.Context, a actor.Actor, info chat.ThreadInfo, rs sync.ReadState) error {
	p, err := marshal(struct {
		ReadState readStateJSON `json:"read_state"`
	}{readStateBody(rs)})
	if err != nil {
		return err
	}
	e := personalEvent(a, authorize.EventReadStateUpdated, p)
	e.SpaceID, e.ChannelID, e.ThreadID = info.SpaceID, info.ChannelID, info.ID
	e.ResourceKind, e.ResourceID = resourceThread, info.ID
	return s.d.Events.Append(ctx, e)
}
