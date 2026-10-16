// SPDX-License-Identifier: Apache-2.0

// Package httpapi holds the REST handlers of the messaging module as part of
// rest.StrictServerInterface: createMessage and createThread (M1-56), listThreadMessages and
// getMessage (M1-57), updateMessage, deleteMessage and listMessageRevisions (M1-58),
// listMyMentions and the six template operations (M1-60), the three reaction and the three pin
// operations (M1-59) of docs/api/openapi.yaml. The use cases authorize; the handlers only convert, set Idempotent-Replay
// on a repetition and ETag on a template change, and pass errors to the problem writer of the
// chain unchanged.
//
// Example:
//
//	h := httpapi.New(svc, svc.Mentions(), svc.Templates(), svc.Reactions(), svc.Pins(), threads)
//	built, err := routes.Build(wire.Handlers{Messages: h}, wire.RouteOptions(), cat)
package httpapi

import (
	"context"
	"time"

	"github.com/OWNER/bothy/services/server/api/rest"
	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
)

// Messages implements the REST operations of messaging.
type Messages struct {
	messages  messaging.Messages
	mentions  messaging.Mentions
	templates messaging.Templates
	reactions messaging.Reactions
	pins      messaging.Pins
	threads   chat.Threads
}

// New returns the handlers over the use cases of messaging (sending m, mentions mn, templates
// tp, reactions rc, pins pn) and the thread port of chat, which gives the thread of a
// createThread answer.
func New(m messaging.Messages, mn messaging.Mentions, tp messaging.Templates, rc messaging.Reactions, pn messaging.Pins, t chat.Threads) *Messages {
	return &Messages{messages: m, mentions: mn, templates: tp, reactions: rc, pins: pn, threads: t}
}

// principal returns the actor of the request; anonymous when the chain set none.
func principal(ctx context.Context) actor.Actor {
	if a, ok := actor.FromContext(ctx); ok {
		return a
	}
	return actor.Anonymous()
}

// CreateMessage implements rest.StrictServerInterface (POST /threads/{thread_id}/messages): 201
// with the new message, 200 with Idempotent-Replay for a repetition (04 section 5.1).
func (h *Messages) CreateMessage(ctx context.Context, req rest.CreateMessageRequestObject) (rest.CreateMessageResponseObject, error) {
	cmd := messageCmd(*req.Body)
	cmd.ThreadID = ids.ID(req.ThreadId)
	c, err := h.messages.Create(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	if c.Replay {
		replay := true
		return rest.CreateMessage200JSONResponse{Body: MessageREST(c.Message), Headers: rest.CreateMessage200ResponseHeaders{IdempotentReplay: &replay}}, nil
	}
	return rest.CreateMessage201JSONResponse{Body: MessageREST(c.Message)}, nil
}

// CreateThread implements rest.StrictServerInterface (POST /channels/{channel_id}/threads): 201
// with the thread and its first message, 200 with Idempotent-Replay for a repetition of the same
// first_message.client_msg_id.
func (h *Messages) CreateThread(ctx context.Context, req rest.CreateThreadRequestObject) (rest.CreateThreadResponseObject, error) {
	b := req.Body
	cmd := messaging.CreateThreadCmd{ChannelID: ids.ID(req.ChannelId), Title: b.Title}
	if b.ParentMessageId != nil {
		p := ids.ID(*b.ParentMessageId)
		cmd.ParentMessageID = &p
	}
	if b.TagIds != nil {
		cmd.TagIDs = idList(*b.TagIds)
	}
	if b.FirstMessage != nil {
		first := messageCmd(*b.FirstMessage)
		cmd.FirstMessage = &first
	}
	c, err := h.messages.CreateThread(ctx, principal(ctx), cmd)
	if err != nil {
		return nil, err
	}
	// The thread as it is after the first message (last_message_seq, message_count, version).
	// CreateThread authorized thread.create (and message.create for a first message), so the
	// answer does not need thread.read: an app with messages:write only gets the thread it
	// created instead of a 403 after the commit (05 section 2.6).
	th, err := h.threads.View(ctx, c.ThreadID)
	if err != nil {
		return nil, err
	}
	out := rest.ThreadCreated{Thread: threadREST(th)}
	if c.FirstMessage != nil {
		m := MessageREST(*c.FirstMessage)
		out.FirstMessage = &m
	}
	if c.Replay {
		replay := true
		return rest.CreateThread200JSONResponse{Body: out, Headers: rest.CreateThread200ResponseHeaders{IdempotentReplay: &replay}}, nil
	}
	return rest.CreateThread201JSONResponse{Body: out}, nil
}

// messageCmd converts the body MessageCreate.
func messageCmd(b rest.MessageCreate) messaging.CreateMessageCmd {
	cmd := messaging.CreateMessageCmd{ClientMsgID: ids.ID(b.ClientMsgId), ContentMD: b.ContentMd}
	if b.Kind != nil {
		k := string(*b.Kind)
		cmd.Kind = &k
	}
	if b.ReplyToMessageId != nil {
		r := ids.ID(*b.ReplyToMessageId)
		cmd.ReplyToID = &r
	}
	if b.AttachmentFileIds != nil {
		cmd.AttachmentIDs = idList(*b.AttachmentFileIds)
	}
	if b.Components != nil {
		cmd.Components = len(*b.Components)
	}
	if b.RunId != nil {
		r := ids.ID(*b.RunId)
		cmd.RunID = &r
	}
	if b.MentionIds != nil {
		cmd.MentionIDs = idList(*b.MentionIds)
	}
	cmd.Silent = b.Silent != nil && *b.Silent
	cmd.ClearDraft = b.ClearDraft != nil && *b.ClearDraft
	return cmd
}

// idList converts a list of ids; never nil.
func idList(list []rest.Uuid) []ids.ID {
	out := make([]ids.ID, 0, len(list))
	for _, u := range list {
		out = append(out, ids.ID(u))
	}
	return out
}

// MessageREST converts a message (schema Message): empty values become absent members, never
// null (ADR 0053); attachments and components are empty until their tickets (M3-07, M2-15). The
// reaction summary, my_reactions (always present in REST, possibly empty) and pinned come from
// the message (M1-59).
func MessageREST(m messaging.Message) rest.Message {
	mentions := make([]rest.Uuid, 0, len(m.MentionIDs))
	for _, id := range m.MentionIDs {
		mentions = append(mentions, rest.Uuid(id))
	}
	out := rest.Message{
		Id: rest.Uuid(m.ID), ThreadId: rest.Uuid(m.ThreadID), ChannelId: rest.Uuid(m.ChannelID), ThreadSeq: m.ThreadSeq,
		Author: rest.PrincipalRef{
			Id: rest.Uuid(m.Author.ID), Kind: rest.PrincipalKind(m.Author.Kind), Handle: m.Author.Handle,
			DisplayName: m.Author.DisplayName, AvatarFileId: optionalID(m.Author.AvatarFileID),
		},
		ClientMsgId: rest.Uuid(m.ClientMsgID), Origin: rest.MessageOrigin(m.Origin), Kind: rest.MessageKind(m.Kind),
		ContentMd: m.ContentMD, ContentFormat: m.ContentFormat, ReplyToMessageId: optionalID(m.ReplyToID), Silent: m.Silent,
		Attachments: []rest.Attachment{}, MentionIds: mentions, Reactions: reactionsREST(m.Reactions), Components: []rest.Component{},
		MyReactions: myReactions(m.MyReactions), Pinned: m.Pinned,
		Version: m.Version, EditedAt: optionalTime(m.EditedAt), DeletedAt: optionalTime(m.DeletedAt),
		CreatedAt: m.CreatedAt.UTC().Truncate(time.Millisecond),
	}
	if m.SystemCode != "" {
		c := rest.MessageSystemCode(m.SystemCode)
		out.SystemCode = &c
	}
	if m.ContentTruncated {
		cut := true
		out.ContentTruncated = &cut
	}
	return out
}

// myReactions returns the own symbols for the member my_reactions; never nil, so REST always
// carries it.
func myReactions(list []string) *[]string {
	out := append([]string{}, list...)
	return &out
}

// threadREST converts a thread (schema Thread) like the thread handlers of chat: empty values
// become absent members, unread_floor_seq only when not 0, tag_ids always present.
func threadREST(th chat.Thread) rest.Thread {
	out := rest.Thread{
		Id: rest.Uuid(th.ID), ChannelId: rest.Uuid(th.ChannelID), Kind: rest.ThreadKind(th.Kind),
		ParentMessageId: optionalID(th.ParentMessageID), Title: th.Title, TitleSource: rest.ThreadTitleSource(th.TitleSource),
		State: rest.ThreadState(th.State), ClosedAt: optionalTime(th.ClosedAt), ClosedBy: optionalID(th.ClosedBy),
		OperatorAgentId: optionalID(th.OperatorAgentID), LastMessageSeq: th.LastMessageSeq,
		LastActivityAt: th.LastActivityAt.UTC().Truncate(time.Millisecond), MessageCount: th.MessageCount,
		CreatedBy: rest.Uuid(th.CreatedBy), CreatedAt: th.CreatedAt.UTC().Truncate(time.Millisecond), MovedAt: optionalTime(th.MovedAt),
		Version: th.Version, TagIds: idsREST(th.TagIDs),
	}
	if th.UnreadFloorSeq != 0 {
		v := th.UnreadFloorSeq
		out.UnreadFloorSeq = &v
	}
	if th.BillingMode != "" && !th.OperatorAgentID.IsZero() {
		m := rest.BillingMode(th.BillingMode)
		out.BillingMode = &m
	}
	return out
}

// The response conversions are the shared ones of api/rest: idsREST never returns nil,
// optionalID and optionalTime leave the zero value absent.
var (
	idsREST      = rest.IDs
	optionalID   = rest.OptionalID
	optionalTime = rest.OptionalTime
)
