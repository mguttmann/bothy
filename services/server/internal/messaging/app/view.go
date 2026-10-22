// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// Values of the events (05 section 6).
const (
	resourceMessage    = "message"
	receiveMessageRead = "message.read"
)

// author returns the PrincipalRef of the principal id.
func (s *Service) author(ctx context.Context, id ids.ID) (messaging.PrincipalRef, error) {
	p, err := s.d.Directory.Principal(ctx, id)
	if err != nil {
		if errs.KindOf(err) == errs.KindNotFound {
			return messaging.PrincipalRef{}, errs.New(errs.CodeInternalError, "messaging: a message without its author")
		}
		return messaging.PrincipalRef{}, err
	}
	return messaging.PrincipalRef{ID: p.ID, Kind: string(p.Kind), Handle: p.Handle, DisplayName: p.DisplayName, AvatarFileID: p.AvatarFileID}, nil
}

// base returns the representation of a stored message with its author and mentions now, without
// reactions and pin (decorate adds them).
func (s *Service) base(ctx context.Context, m domain.Message) (messaging.Message, error) {
	author, err := s.author(ctx, m.AuthorID)
	if err != nil {
		return messaging.Message{}, err
	}
	mentions, err := s.d.Store.Mentions(ctx, m.ID)
	if err != nil {
		return messaging.Message{}, err
	}
	return view(m, author, mentions), nil
}

// stored returns the current representation of a stored message for the principal of the
// request: its author, mentions, reactions, own reactions and pin now, read with the visible
// channels of the caller.
func (s *Service) stored(ctx context.Context, m domain.Message, visible []ids.ID) (messaging.Message, error) {
	v, err := s.base(ctx, m)
	if err != nil {
		return messaging.Message{}, err
	}
	list := []messaging.Message{v}
	if err := s.decorate(ctx, visible, list); err != nil {
		return messaging.Message{}, err
	}
	return list[0], nil
}

// decorate fills reactions, my_reactions (for the principal of the request) and pinned of the
// live messages of list with one statement each for the whole list (brief M1-59 design note 7);
// a tombstone keeps empty reactions and pinned false.
func (s *Service) decorate(ctx context.Context, visible []ids.ID, list []messaging.Message) error {
	live := make([]ids.ID, 0, len(list))
	for _, m := range list {
		if m.DeletedAt.IsZero() {
			live = append(live, m.ID)
		}
	}
	if len(live) == 0 || len(visible) == 0 {
		return nil
	}
	groups, err := s.d.Store.ReactionGroups(ctx, live, visible)
	if err != nil {
		return err
	}
	var own map[ids.ID]map[string]bool
	if viewer := caller(ctx); !viewer.ID.IsZero() {
		if own, err = s.d.Store.OwnReactions(ctx, viewer.ID, live, visible); err != nil {
			return err
		}
	}
	pinned, err := s.d.Store.Pinned(ctx, live, visible)
	if err != nil {
		return err
	}
	for i := range list {
		m := &list[i]
		if !m.DeletedAt.IsZero() {
			continue
		}
		ordered := domain.OrderSummary(groups[m.ID])
		m.Reactions = make([]messaging.ReactionCount, 0, len(ordered))
		for _, g := range ordered {
			m.Reactions = append(m.Reactions, messaging.ReactionCount{Emoji: g.Emoji, Count: g.Count})
		}
		m.MyReactions = domain.OwnInOrder(ordered, own[m.ID])
		m.Pinned = pinned[m.ID]
	}
	return nil
}

// view converts a row into the port DTO; the mentions are never nil. It is the one representation
// of a message (brief M1-57 note 6, also for M1-58 and M1-59): a deleted message is a tombstone
// with empty content, no mentions, no reactions and not pinned, every other field as stored
// (attachments and components stay empty in the REST and event forms); run_id and the trigger
// state are not content, so a tombstone keeps them (03 messages, M2-05). Reactions and the pin
// start empty; decorate fills them for a stored message.
func view(m domain.Message, author messaging.PrincipalRef, mentions []ids.ID) messaging.Message {
	if mentions == nil || !m.DeletedAt.IsZero() {
		mentions = []ids.ID{}
	}
	if !m.DeletedAt.IsZero() {
		m.ContentMD = ""
	}
	return messaging.Message{
		ID: m.ID, ThreadID: m.ThreadID, ChannelID: m.ChannelID, ThreadSeq: m.ThreadSeq, Author: author, ClientMsgID: m.ClientMsgID,
		Origin: string(m.Origin), Kind: string(m.Kind), SystemCode: m.SystemCode, ContentMD: m.ContentMD,
		ContentFormat: int(m.ContentFormat), ReplyToID: m.ReplyToID, RunID: m.RunID, TriggerState: m.TriggerState,
		TriggerStateReason: m.TriggerStateReason, TriggerStateAt: m.TriggerStateAt, Silent: m.Silent, MentionIDs: mentions,
		Reactions: []messaging.ReactionCount{}, MyReactions: []string{}, Version: m.Version,
		EditedAt: m.EditedAt, DeletedAt: m.DeletedAt, CreatedAt: m.CreatedAt,
	}
}

// The payload helpers are the shared ones of the kernel: stamp formats an API time (the zero time
// is absent) and optionalID an id (the zero ID is absent).
var (
	stamp      = clock.FormatAPI
	optionalID = ids.ID.StringOrEmpty
)

// eventActor returns the event actor of the request in ctx; none outside a request.
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	a, _ := actor.FromContext(ctx)
	return eventlog.ActorFor(string(a.Kind), a.ID)
}

// principalJSON is the schema PrincipalRef.
type principalJSON struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Handle       string `json:"handle"`
	DisplayName  string `json:"display_name"`
	AvatarFileID string `json:"avatar_file_id,omitempty"`
}

// reactionCountJSON is the schema ReactionSummary.
type reactionCountJSON struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
}

// messageJSON is the schema Message in an event (brief D7): attachments and components empty, the
// reaction summary and pinned as decorated (M1-59), never my_reactions, which only REST responses
// carry. Optional members are left out, never null (ADR 0053).
type messageJSON struct {
	ID               string              `json:"id"`
	ThreadID         string              `json:"thread_id"`
	ChannelID        string              `json:"channel_id"`
	ThreadSeq        int64               `json:"thread_seq"`
	Author           principalJSON       `json:"author"`
	ClientMsgID      string              `json:"client_msg_id"`
	Origin           string              `json:"origin"`
	Kind             string              `json:"kind"`
	SystemCode       string              `json:"system_code,omitempty"`
	ContentMD        string              `json:"content_md"`
	ContentFormat    int                 `json:"content_format"`
	ContentTruncated bool                `json:"content_truncated,omitempty"`
	ReplyToMessageID string              `json:"reply_to_message_id,omitempty"`
	RunID            string              `json:"run_id,omitempty"`
	TriggerState     string              `json:"trigger_state,omitempty"`
	TriggerStateAt   string              `json:"trigger_state_at,omitempty"`
	TriggerReason    string              `json:"trigger_state_reason,omitempty"`
	Silent           bool                `json:"silent"`
	Attachments      []struct{}          `json:"attachments"`
	MentionIDs       []string            `json:"mention_ids"`
	Reactions        []reactionCountJSON `json:"reactions"`
	Pinned           bool                `json:"pinned"`
	Components       []struct{}          `json:"components"`
	Version          int64               `json:"version"`
	EditedAt         string              `json:"edited_at,omitempty"`
	DeletedAt        string              `json:"deleted_at,omitempty"`
	CreatedAt        string              `json:"created_at"`
}

// messageBody converts a message into the JSON form of the schema Message.
func messageBody(m messaging.Message) messageJSON {
	mentions := make([]string, 0, len(m.MentionIDs))
	for _, id := range m.MentionIDs {
		mentions = append(mentions, id.String())
	}
	reactions := make([]reactionCountJSON, 0, len(m.Reactions))
	for _, r := range m.Reactions {
		reactions = append(reactions, reactionCountJSON{Emoji: r.Emoji, Count: r.Count})
	}
	return messageJSON{
		ID: m.ID.String(), ThreadID: m.ThreadID.String(), ChannelID: m.ChannelID.String(), ThreadSeq: m.ThreadSeq,
		Author: principalJSON{
			ID: m.Author.ID.String(), Kind: m.Author.Kind, Handle: m.Author.Handle, DisplayName: m.Author.DisplayName,
			AvatarFileID: optionalID(m.Author.AvatarFileID),
		},
		ClientMsgID: m.ClientMsgID.String(), Origin: m.Origin, Kind: m.Kind, SystemCode: m.SystemCode, ContentMD: m.ContentMD,
		ContentFormat: m.ContentFormat, ReplyToMessageID: optionalID(m.ReplyToID), RunID: optionalID(m.RunID), TriggerState: m.TriggerState,
		TriggerStateAt: stamp(m.TriggerStateAt), TriggerReason: m.TriggerStateReason, Silent: m.Silent, Attachments: []struct{}{},
		MentionIDs: mentions, Reactions: reactions, Pinned: m.Pinned, Components: []struct{}{}, Version: m.Version, EditedAt: stamp(m.EditedAt),
		DeletedAt: stamp(m.DeletedAt), CreatedAt: stamp(m.CreatedAt),
	}
}

// encode marshals v without HTML escaping, so the size of a payload is the size of its content.
func encode(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "messaging: encoding an event payload")
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// payload encodes the payload MessagePayload of body.
func payload(body messageJSON) ([]byte, error) {
	return encode(struct {
		Message messageJSON `json:"message"`
	}{body})
}

// eventPayload returns the payload of m below sync.event_payload_max_bytes: whole when it fits,
// otherwise with content_md cut to the longest valid UTF-8 prefix that fits, content_truncated
// set and truncated reported for the envelope (04 payload rule 2, brief D7).
func (s *Service) eventPayload(m messaging.Message) ([]byte, bool, error) {
	body := messageBody(m)
	full, err := payload(body)
	if err != nil || len(full) < s.cfg.EventPayloadMaxBytes {
		return full, false, err
	}
	body.ContentTruncated = true
	content := body.ContentMD
	cut, _ := domain.TruncateForEvent(content, func(prefix string) bool {
		body.ContentMD = prefix
		p, err := payload(body)
		return err == nil && len(p) < s.cfg.EventPayloadMaxBytes
	})
	body.ContentMD = cut
	out, err := payload(body)
	if err != nil {
		return nil, false, err
	}
	if len(out) >= s.cfg.EventPayloadMaxBytes {
		return nil, false, errs.New(errs.CodeInternalError, "messaging: a message event does not fit sync.event_payload_max_bytes")
	}
	return out, true, nil
}

// emitCreated appends message.created for m in the thread of info (brief D7): resource the
// message with version 1, scope space, channel and thread, receive action message.read and, for
// a message of a client, the echo of client_msg_id.
func (s *Service) emitCreated(ctx context.Context, info chat.ThreadInfo, m messaging.Message, echo bool) error {
	return s.emitMessage(ctx, authorize.EventMessageCreated, info, m, echo)
}

// emitMessage appends the event typ with the payload {message} of m (message.created and, from
// M1-58, message.updated): resource the message with its version after the change, scope space,
// channel and thread, receive action message.read, the content cut by eventPayload, and with echo
// the client_msg_id.
func (s *Service) emitMessage(ctx context.Context, typ authorize.EventType, info chat.ThreadInfo, m messaging.Message, echo bool) error {
	p, truncated, err := s.eventPayload(m)
	if err != nil {
		return err
	}
	kind, actorID := eventActor(ctx)
	ev := eventlog.Event{
		Type: string(typ), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: m.ChannelID, ThreadID: m.ThreadID, ResourceKind: resourceMessage, ResourceID: m.ID,
		ResourceVersion: m.Version, ReceiveAction: receiveMessageRead, Payload: p, Truncated: truncated,
	}
	if echo {
		ev.ClientMsgID = m.ClientMsgID
	}
	return s.d.Events.Append(ctx, ev)
}
