// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
	"github.com/OWNER/bothy/services/server/internal/sync"
)

// BadgeCounter computes the badge number of 13 section 8 (brief M3-19 design note 16 a,
// D-M3-19-15): the job notify.push_send sets it on every Notification, and M3-22 calls it for the
// badge of the preview endpoint.
type BadgeCounter struct{ p *Push }

// Badge returns the badge counter of the push path.
func (p *Push) Badge() BadgeCounter { return BadgeCounter{p: p} }

// For returns the badge number of user now: the number of threads user can read that hold an
// unread mention of user, or are unread with effective level all, or are unread in a DM channel.
// Unread at read position pos means last_message_seq > max(pos, unread_floor_seq) (03 read_states
// "Unread"). Every thread that counts is confirmed with push.deliver in filter mode, so a thread
// the person cannot read never counts. A person who is not an active human has the number 0.
//
// Errors: those of the reads and of authorize.
func (b BadgeCounter) For(ctx context.Context, user ids.ID) (int64, error) {
	counting, err := b.p.counting(ctx, user)
	return int64(len(counting)), err
}

// counting returns the threads that count for the badge of user now.
func (p *Push) counting(ctx context.Context, user ids.ID) (map[ids.ID]bool, error) {
	scope, err := p.d.Authorizer.Scope(authorize.Filter(ctx), actor.Recheck(user), authorize.ActionPushDeliver)
	if err != nil {
		if denied(err) {
			return map[ids.ID]bool{}, nil
		}
		return nil, err
	}
	if len(scope.ChannelIDs) == 0 {
		return map[ids.ID]bool{}, nil
	}
	cands, err := p.d.Threads.UnreadCandidates(ctx, scope.ChannelIDs)
	if err != nil || len(cands) == 0 {
		return map[ids.ID]bool{}, err
	}
	threadIDs := make([]ids.ID, 0, len(cands))
	for _, c := range cands {
		threadIDs = append(threadIDs, c.ThreadID)
	}
	pos, err := p.d.Positions.PositionsOfUser(ctx, user, threadIDs)
	if err != nil {
		return nil, err
	}
	return p.countThreads(ctx, user, cands, scope.ChannelIDs, pos)
}

// countThreads applies the rule of 13 8 to the candidate threads cands of user at the read
// positions pos (absent: 0); their channels are among channels.
func (p *Push) countThreads(ctx context.Context, user ids.ID, cands []chat.ThreadPosition, channels []ids.ID, pos map[ids.ID]int64) (map[ids.ID]bool, error) {
	out := map[ids.ID]bool{}
	var unread []chat.ThreadPosition
	floors := map[ids.ID]int64{}
	for _, c := range cands {
		from := max(pos[c.ThreadID], c.UnreadFloorSeq)
		if c.LastMessageSeq > from {
			unread = append(unread, c)
			floors[c.ThreadID] = from
		}
	}
	if len(unread) == 0 {
		return out, nil
	}
	mentions, err := p.d.Mentions.CountUnread(ctx, messaging.UnreadMentionQuery{PrincipalID: user, VisibleChannelIDs: channels, LastReadSeq: floors})
	if err != nil {
		return nil, err
	}
	mentioned := map[ids.ID]bool{}
	for _, m := range mentions {
		mentioned[m.ThreadID] = m.Count > 0
	}
	var levelThreads []ids.ID
	for _, c := range unread {
		if !mentioned[c.ThreadID] && c.ChannelKind != channelKindDM {
			levelThreads = append(levelThreads, c.ThreadID)
		}
	}
	levels, err := p.d.Facts.LevelFactsOfUser(ctx, user, levelThreads)
	if err != nil {
		return nil, err
	}
	now := p.d.Clock.Now()
	for _, c := range unread {
		counts := mentioned[c.ThreadID] || c.ChannelKind == channelKindDM
		if !counts && slices.Contains(levelThreads, c.ThreadID) {
			lf, ok := levels[c.ThreadID]
			if !ok {
				continue
			}
			eff, err := domain.ResolveLevel(lf, now)
			if err != nil {
				return nil, err
			}
			counts = eff.Level == domain.LevelAll && !eff.Muted
		}
		if !counts {
			continue
		}
		ok, err := p.canRead(ctx, user, c.ThreadID, c.ChannelID, ids.ID{})
		if err != nil {
			return nil, err
		}
		if ok {
			out[c.ThreadID] = true
		}
	}
	return out, nil
}

// threadCounts reports whether thread counts for the badge of user at the read position pos
// (design note 16 b), with the counters and rights of now.
func (p *Push) threadCounts(ctx context.Context, user, thread ids.ID, pos int64) (bool, error) {
	info, err := p.d.Threads.Info(ctx, thread)
	if err != nil {
		if errs.KindOf(err) == errs.KindNotFound {
			return false, nil
		}
		return false, err
	}
	if info.LastMessageSeq <= max(pos, info.UnreadFloorSeq) {
		return false, nil
	}
	c := chat.ThreadPosition{ThreadID: thread, ChannelID: info.ChannelID, ChannelKind: info.ChannelKind,
		LastMessageSeq: info.LastMessageSeq, UnreadFloorSeq: info.UnreadFloorSeq}
	got, err := p.countThreads(ctx, user, []chat.ThreadPosition{c}, []ids.ID{info.ChannelID}, map[ids.ID]int64{thread: pos})
	return got[thread], err
}

// onReadState handles read_state.updated of person r for thread T (design note 16 b): when the
// read lowers the badge number (T counts at the previous position and not at the new one), it
// schedules one badge sync of r, unless one is pending, at the end of a pending hold or now. A
// mark as unread never lowers it.
func (p *Push) onReadState(ctx context.Context, ev sync.Event) error {
	user := ev.AudiencePrincipalID
	var pl readStatePayload
	if err := decode(ev, &pl); err != nil || user.IsZero() {
		return err
	}
	thread, now := pl.ReadState.ThreadID, pl.ReadState.LastReadSeq
	prev, err := p.previousPosition(ctx, user, thread, ev.Seq)
	if err != nil || now <= prev {
		return err
	}
	before, err := p.threadCounts(ctx, user, thread, prev)
	if err != nil || !before {
		return err
	}
	after, err := p.threadCounts(ctx, user, thread, now)
	if err != nil || after {
		return err
	}
	pending, err := p.d.Scheduler.BadgeSyncPending(ctx, user)
	if err != nil || pending {
		return err
	}
	at := p.d.Clock.Now()
	if end, ok, err := p.d.Scheduler.HoldEnd(ctx, user); err != nil {
		return err
	} else if ok && end.After(at) {
		at = end
	}
	return p.d.Scheduler.ScheduleBadgeSync(ctx, user, ev.Seq-1, at)
}

// previousPosition returns last_read_seq of the newest read_state.updated of user for thread
// before seq, else 0.
func (p *Push) previousPosition(ctx context.Context, user, thread ids.ID, seq int64) (int64, error) {
	events, err := p.d.History.ResourceEvents(ctx, sync.ResourceQuery{
		ResourceKind: resourceThread, ResourceID: thread, Types: []string{string(authorize.EventReadStateUpdated)}, Before: seq,
	})
	if err != nil {
		return 0, err
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].AudiencePrincipalID != user {
			continue
		}
		var pl readStatePayload
		if err := decode(events[i], &pl); err != nil {
			return 0, err
		}
		return pl.ReadState.LastReadSeq, nil
	}
	return 0, nil
}

// BadgeSync is the use case of the job notify.badge_sync of user (design note 16 c), in one unit
// of work: the badge number now, Cleared = the threads of the read_state.updated events of user
// with seq above afterSeq that no longer count, distinct and in event order; a hold until now +
// notify.badge_sync_min_interval; then one notify.push_send of the kind KindSync with a new
// NotificationID per active apns or fcm registration of an active ios or android device of user
// (13 8: mobile devices only). A person who is not an active human gets nothing.
//
// Errors: those of the reads and inserts; River retries the job.
func (p *Push) BadgeSync(ctx context.Context, user ids.ID, afterSeq int64) error {
	return p.d.UoW.Do(ctx, func(ctx context.Context) error {
		users, err := p.activeHumans(ctx, []ids.ID{user})
		if err != nil || len(users) == 0 {
			return err
		}
		now := p.d.Clock.Now()
		counting, err := p.counting(ctx, user)
		if err != nil {
			return err
		}
		events, err := p.d.History.PersonalEvents(ctx, sync.PersonalQuery{
			AudienceID: user, Types: []string{string(authorize.EventReadStateUpdated)}, After: afterSeq,
		})
		if err != nil {
			return err
		}
		var cleared []ids.ID
		for _, ev := range events {
			var pl readStatePayload
			if err := decode(ev, &pl); err != nil {
				return err
			}
			t := pl.ReadState.ThreadID
			if !counting[t] && !slices.Contains(cleared, t) {
				cleared = append(cleared, t)
			}
		}
		if err := p.d.Scheduler.Hold(ctx, user, now.Add(p.cfg.BadgeSyncMinInterval)); err != nil {
			return err
		}
		return p.enqueueSync(ctx, user, cleared, now)
	})
}

// enqueueSync inserts the sync jobs of user for its mobile registrations.
func (p *Push) enqueueSync(ctx context.Context, user ids.ID, cleared []ids.ID, now time.Time) error {
	refs, err := p.d.Devices.ActiveForUsers(ctx, []ids.ID{user})
	if err != nil {
		return err
	}
	mobile := map[ids.ID]bool{}
	for _, d := range refs[user] {
		if d.Platform == platformIOS || d.Platform == platformAndroid {
			mobile[d.ID] = true
		}
	}
	regs, err := p.d.Registrations.Active(ctx, []ids.ID{user})
	if err != nil {
		return err
	}
	for _, r := range regs[user] {
		if !mobile[r.DeviceID] || (r.Provider != providerAPNs && r.Provider != providerFCM) {
			continue
		}
		job := PushJob{NotificationID: ids.New(), RegistrationID: r.ID, RecipientID: user, Cleared: slices.Clone(cleared),
			Kind: domain.KindSync, DueAt: now, Attempt: 1}
		if err := p.d.Scheduler.Send(ctx, job, time.Time{}); err != nil {
			return err
		}
	}
	return nil
}

// denied reports an error that a decision gave (the person cannot be decided for: disabled,
// unknown, no right), as opposed to an internal or unavailable source.
func denied(err error) bool {
	if _, ok := errs.As(err); !ok {
		return false
	}
	k := errs.KindOf(err)
	return k != errs.KindInternal && k != errs.KindUnavailable
}
