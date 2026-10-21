// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// channelForum is chat.ThreadInfo.ChannelKind of a forum.
const channelForum = "forum"

// CreateThread implements messaging.Messages (brief D8, 02 section 4.6): thread.create on the
// channel (an invisible channel is 404, a read-only or archived one 403 channel_read_only), the
// body rules of the first message, then one unit of work: a repetition of the first message
// answers the original thread; otherwise the parent message is loaded, chat creates the thread,
// and the first message is sent into it with message.create on its channel path. A forum post
// needs its first message. Without a first message the Idempotency-Key stage of the HTTP chain
// makes the request idempotent. The caller reads the resulting thread through chat.Threads.View;
// a replay authorizes message.create on that thread first.
func (s *Service) CreateThread(ctx context.Context, a actor.Actor, cmd messaging.CreateThreadCmd) (messaging.ThreadCreated, error) {
	ctx, err := withCaller(ctx, a)
	if err != nil {
		return messaging.ThreadCreated{}, err
	}
	if err := s.check(ctx, a, authorize.ActionThreadCreate, authorize.Resource{Kind: authorize.ResourceChannel, ID: cmd.ChannelID}); err != nil {
		return messaging.ThreadCreated{}, err
	}
	var p *prepared
	if cmd.FirstMessage != nil {
		// A thread created by this request has no run (M2-19 design note 5).
		if err := domain.CheckFirstMessageRunID(cmd.FirstMessage.RunID); err != nil {
			return messaging.ThreadCreated{}, err
		}
		first, err := s.prepare(ctx, a, *cmd.FirstMessage, ids.ID{}, newThreadContext(cmd))
		if err != nil {
			return messaging.ThreadCreated{}, err
		}
		p = &first
	}
	var out messaging.ThreadCreated
	err = s.d.UoW.Do(ctx, func(ctx context.Context) error {
		var err error
		out, err = s.createThread(ctx, a, cmd, p)
		return err
	})
	if err == nil {
		out, err = s.committedFirst(ctx, a, out)
	}
	if errors.Is(err, ErrClientMsgIDTaken) && p != nil {
		var c messaging.Created
		if c, err = s.afterRace(ctx, a, *p); err == nil {
			out = messaging.ThreadCreated{ThreadID: c.Message.ThreadID, FirstMessage: &c.Message, Replay: true}
		}
	}
	if err == nil && out.Replay {
		// The handler answers with the thread without thread.read, so a replay proves
		// message.create on the thread as it is now, like a replay of createMessage (brief D1).
		if _, err := s.loadThread(ctx, a, out.ThreadID); err != nil {
			return messaging.ThreadCreated{}, err
		}
	}
	return out, err
}

// committedFirst replaces the first message of a new thread with the row as committed, like the
// 201 of createMessage (M2-05 brief note 7): a subscriber may have changed it in the unit.
func (s *Service) committedFirst(ctx context.Context, a actor.Actor, out messaging.ThreadCreated) (messaging.ThreadCreated, error) {
	if out.Replay || out.FirstMessage == nil {
		return out, nil
	}
	m, err := s.reread(ctx, a, out.FirstMessage.ID)
	if err != nil {
		return messaging.ThreadCreated{}, err
	}
	out.FirstMessage = &m
	return out, nil
}

// createThread is the unit of work of CreateThread.
func (s *Service) createThread(ctx context.Context, a actor.Actor, cmd messaging.CreateThreadCmd, p *prepared) (messaging.ThreadCreated, error) {
	if p != nil {
		m, found, err := s.repetition(ctx, a, *p)
		if err != nil || found {
			return messaging.ThreadCreated{ThreadID: m.ThreadID, FirstMessage: &m, Replay: found}, err
		}
	}
	create := chat.CreateThreadCmd{ChannelID: cmd.ChannelID, Title: cmd.Title, TagIDs: cmd.TagIDs}
	if cmd.ParentMessageID != nil {
		parent, err := s.parent(ctx, *cmd.ParentMessageID)
		if err != nil {
			return messaging.ThreadCreated{}, err
		}
		create.Parent = &parent
	}
	th, err := s.d.Threads.Create(ctx, a, create)
	if err != nil {
		return messaging.ThreadCreated{}, err
	}
	info, err := s.d.Threads.Info(ctx, th.ID)
	if err != nil {
		return messaging.ThreadCreated{}, err
	}
	out := messaging.ThreadCreated{ThreadID: th.ID}
	if p == nil {
		if info.ChannelKind == channelForum {
			return messaging.ThreadCreated{}, domain.RequiredForForumPost()
		}
		return out, nil
	}
	if err := s.check(ctx, a, authorize.ActionMessageCreate, newThreadResource(info)); err != nil {
		return messaging.ThreadCreated{}, err
	}
	c, err := s.send(ctx, a, *p, info, true)
	if err != nil {
		return messaging.ThreadCreated{}, err
	}
	out.FirstMessage = &c.Message
	return out, nil
}

// parent returns the parent message for chat: a missing or deleted message keeps a zero thread,
// which chat answers with 400 not_in_main_thread like a message outside the main thread of the
// channel (brief D8), so nothing about other channels leaks.
func (s *Service) parent(ctx context.Context, id ids.ID) (chat.ParentMessage, error) {
	m, found, err := s.d.Store.Message(ctx, id)
	if err != nil {
		return chat.ParentMessage{}, err
	}
	pm := chat.ParentMessage{MessageID: id}
	if found && m.DeletedAt.IsZero() {
		pm.ThreadID = m.ThreadID
	}
	return pm, nil
}

// newThreadContext is the hash context of a first message (brief D8): channel, parent, title and
// the sorted tag ids.
func newThreadContext(cmd messaging.CreateThreadCmd) string {
	tags := make([]string, 0, len(cmd.TagIDs))
	for _, t := range cmd.TagIDs {
		tags = append(tags, t.String())
	}
	slices.Sort(tags)
	parent, title := "", ""
	if cmd.ParentMessageID != nil {
		parent = cmd.ParentMessageID.String()
	}
	if cmd.Title != nil {
		title = *cmd.Title
	}
	return "new-thread:" + cmd.ChannelID.String() + "|" + parent + "|" + title + "|" + strings.Join(tags, ",")
}
