// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"

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

// The audit details of thread.move (brief M1-62 note 1h): from, to and the reason, nothing else.
const (
	attrFromChannelID = "from_channel_id"
	attrToChannelID   = "to_channel_id"
)

// moveResource is the descriptor of thread.move (07 4.2 "Actions with two targets"): the thread
// with its creator as owner and the destination in TargetChannelID, without a channel hint, so
// authz resolves the thread's place itself at decision time and a concurrent move cannot make the
// hint contradict the structure; for an unknown thread only the id, so authorize answers it like
// an invisible one.
func moveResource(th domain.Thread, found bool, id, target ids.ID) authorize.Resource {
	r := authorize.Resource{Kind: authorize.ResourceThread, ID: id, TargetChannelID: target}
	if found {
		r.OwnerID = th.CreatedBy
	}
	return r
}

// Move implements chat.Threads (brief M1-62 note 1) in one unit of work; see the port for the
// contract. Concurrency: the thread row is locked FOR UPDATE, so a concurrent move, message,
// deletion or read-state write of the same thread is serialized with it (M1-58, M1-61); the target
// channel and, when it inherits, its space are held FOR SHARE before the effective operator is
// computed, so a concurrent channel.operator.set or space.operator.set either committed before
// (the move sees the new operator) or waits for the commit, and the move never breaks I1 (03 "I1
// check", target kind 2). A deadlock with an operator change is retried by the unit of work
// (database.tx_retry_max).
func (s *Service) Move(ctx context.Context, a actor.Actor, cmd chat.MoveThread) (chat.Thread, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return chat.Thread{}, err
	}
	var out chat.Thread
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.moveThread(ctx, a, cmd)
		return err
	})
	return out, err
}

// moveThread is the body of Move inside its unit of work.
func (s *Service) moveThread(ctx context.Context, a actor.Actor, cmd chat.MoveThread) (chat.Thread, error) {
	th, found, err := s.d.Store.Thread(ctx, cmd.ThreadID)
	if err != nil {
		return chat.Thread{}, err
	}
	if err := s.check(ctx, a, authorize.ActionThreadMove, moveResource(th, found, cmd.ThreadID, cmd.TargetChannelID)); err != nil {
		return chat.Thread{}, err
	}
	if !found {
		return chat.Thread{}, invisible()
	}
	locked, err := s.lockThread(ctx, cmd.ThreadID)
	if err != nil {
		return chat.Thread{}, err
	}
	if locked.ChannelID != th.ChannelID {
		// A concurrent move committed between the read and the lock: decide again on the place
		// the thread has now, so no decision uses the old location (07 4.2).
		if err := s.check(ctx, a, authorize.ActionThreadMove, moveResource(locked, true, cmd.ThreadID, cmd.TargetChannelID)); err != nil {
			return chat.Thread{}, err
		}
	}
	source, err := s.threadChannel(ctx, locked)
	if err != nil {
		return chat.Thread{}, err
	}
	target, operator, err := s.shareTarget(ctx, cmd.TargetChannelID)
	if err != nil {
		return chat.Thread{}, err
	}
	if err := domain.CanMove(locked, movePlace(source), movePlace(target), operator); err != nil {
		return chat.Thread{}, err
	}
	if err := domain.CheckReason(cmd.Reason); err != nil {
		return chat.Thread{}, err
	}
	return s.moveWrite(ctx, locked, source, target, cmd.Reason)
}

// shareTarget holds the target channel FOR SHARE and, when it inherits its operator, its space,
// and returns the channel with the effective operator computed after these locks (brief M1-62
// note 1d). An unknown target cannot pass authorize; a target deleted since answers 404 anyway.
func (s *Service) shareTarget(ctx context.Context, id ids.ID) (ChannelRow, ids.ID, error) {
	c, found, err := s.d.Store.ShareChannel(ctx, id)
	if err != nil {
		return ChannelRow{}, ids.ID{}, err
	}
	if !found {
		return ChannelRow{}, ids.ID{}, invisible()
	}
	if !c.Channel.OperatorInherit {
		return c, c.Channel.OperatorAgentID, nil
	}
	sp, found, err := s.d.Store.ShareSpace(ctx, c.Channel.SpaceID)
	if err != nil {
		return ChannelRow{}, ids.ID{}, err
	}
	if !found {
		return ChannelRow{}, ids.ID{}, errs.New(errs.CodeInternalError, "chat: a channel without its space")
	}
	c.SpaceDefaultOperatorID, c.SpaceArchived = sp.DefaultOperatorAgentID, sp.Archived()
	return c, sp.DefaultOperatorAgentID, nil
}

// movePlace converts a channel row into the form of domain.CanMove.
func movePlace(c ChannelRow) domain.MovePlace {
	return domain.MovePlace{ID: c.Channel.ID, Kind: c.Channel.Kind, Archived: archived(c)}
}

// moveWrite performs the checked move of the locked thread th from source to target (brief M1-62
// notes 1e to 1h): the thread row, the tags, ThreadMoved for the copies of messaging and sync,
// thread.moved_out and thread.moved in this order, and the audit entry.
func (s *Service) moveWrite(ctx context.Context, th domain.Thread, source, target ChannelRow, reason string) (chat.Thread, error) {
	moved, err := s.d.Store.MoveThread(ctx, th.ID, target.Channel.ID, s.now())
	if err != nil {
		return chat.Thread{}, err
	}
	if err := s.d.Store.DeleteThreadTags(ctx, th.ID); err != nil {
		return chat.Thread{}, err
	}
	if err := txn.Publish(ctx, chat.ThreadMoved{
		ThreadID: th.ID, FromChannelID: source.Channel.ID, FromSpaceID: source.Channel.SpaceID,
		ToChannelID: target.Channel.ID, ToSpaceID: target.Channel.SpaceID, ThreadVersion: moved.Version,
	}); err != nil {
		return chat.Thread{}, err
	}
	v, err := s.threadView(ctx, moved)
	if err != nil {
		return chat.Thread{}, err
	}
	if err := s.emitMovedOut(ctx, th.ID, source); err != nil {
		return chat.Thread{}, err
	}
	if err := s.emitMoved(ctx, v, source.Channel.ID, target.Channel.SpaceID); err != nil {
		return chat.Thread{}, err
	}
	return v, s.d.Audit.Record(ctx, auditing.Entry{
		Action: string(authorize.ActionThreadMove), Decision: auditing.DecisionAllow, ResourceKind: resourceThread, ResourceID: th.ID,
		Attrs: auditing.Attrs{
			attrFromChannelID: auditing.ID(source.Channel.ID), attrToChannelID: auditing.ID(target.Channel.ID),
			attrReason: auditing.ReasonText(reason),
		},
	})
}

// emitMovedOut appends thread.moved_out in the source channel (brief M1-62 note 1g): scope the
// source space and channel, resource the source channel without a version (the channel did not
// change, and a version would make clients drop the event, 04 "Delivery guarantee" point 3),
// payload ThreadMovedOutPayload, receive action channel.read. Nothing in it names the target.
func (s *Service) emitMovedOut(ctx context.Context, thread ids.ID, source ChannelRow) error {
	payload, err := json.Marshal(struct {
		ThreadID  string `json:"thread_id"`
		ChannelID string `json:"channel_id"`
	}{thread.String(), source.Channel.ID.String()})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "chat: thread.moved_out payload")
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventThreadMovedOut), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: source.Channel.SpaceID, ChannelID: source.Channel.ID, ResourceKind: resourceChannel, ResourceID: source.Channel.ID,
		ReceiveAction: receiveChannelRead, Payload: payload,
	})
}

// emitMoved appends thread.moved in the target channel (brief M1-62 note 1g): scope the target
// space, channel and thread, resource the thread with its new version, payload ThreadMovedPayload
// (the full thread after the move and the source channel), receive action thread.read.
func (s *Service) emitMoved(ctx context.Context, v chat.Thread, from, space ids.ID) error {
	payload, err := json.Marshal(struct {
		Thread        threadJSON `json:"thread"`
		FromChannelID string     `json:"from_channel_id"`
	}{threadBody(v), from.String()})
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, err, "chat: thread.moved payload")
	}
	kind, actorID := eventActor(ctx)
	return s.d.Events.Append(ctx, eventlog.Event{
		Type: string(authorize.EventThreadMoved), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID, SpaceID: space,
		ChannelID: v.ChannelID, ThreadID: v.ID, ResourceKind: resourceThread, ResourceID: v.ID, ResourceVersion: v.Version,
		ReceiveAction: receiveThreadRead, Payload: payload,
	})
}
