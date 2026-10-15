// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
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

// stored returns the current representation of a stored message: its author and mentions now.
func (s *Service) stored(ctx context.Context, m domain.Message) (messaging.Message, error) {
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

// view converts a row into the port DTO; the mentions are never nil. It is the one representation
// of a message (brief M1-57 note 6, also for M1-58): a deleted message is a tombstone with empty
// content and no mentions, every other field as stored (attachments, reactions and components stay
// empty and pinned false in the REST and event forms).
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
		ContentFormat: int(m.ContentFormat), ReplyToID: m.ReplyToID, Silent: m.Silent, MentionIDs: mentions, Version: m.Version,
		EditedAt: m.EditedAt, DeletedAt: m.DeletedAt, CreatedAt: m.CreatedAt,
	}
}

// timestampLayout is the form of every API time (06 section 2): UTC, milliseconds, suffix Z.
const timestampLayout = "2006-01-02T15:04:05.000Z"

// stamp formats t; the zero time is absent.
func stamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Truncate(time.Millisecond).Format(timestampLayout)
}

// optionalID formats id; the zero ID is absent.
func optionalID(id ids.ID) string {
	if id.IsZero() {
		return ""
	}
	return id.String()
}

// principalJSON is the schema PrincipalRef.
type principalJSON struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Handle       string `json:"handle"`
	DisplayName  string `json:"display_name"`
	AvatarFileID string `json:"avatar_file_id,omitempty"`
}

// messageJSON is the schema Message in an event (brief D7): attachments, reactions and components
// empty, pinned false, without my_reactions. Optional members are left out, never null (ADR 0053).
type messageJSON struct {
	ID               string        `json:"id"`
	ThreadID         string        `json:"thread_id"`
	ChannelID        string        `json:"channel_id"`
	ThreadSeq        int64         `json:"thread_seq"`
	Author           principalJSON `json:"author"`
	ClientMsgID      string        `json:"client_msg_id"`
	Origin           string        `json:"origin"`
	Kind             string        `json:"kind"`
	SystemCode       string        `json:"system_code,omitempty"`
	ContentMD        string        `json:"content_md"`
	ContentFormat    int           `json:"content_format"`
	ContentTruncated bool          `json:"content_truncated,omitempty"`
	ReplyToMessageID string        `json:"reply_to_message_id,omitempty"`
	Silent           bool          `json:"silent"`
	Attachments      []struct{}    `json:"attachments"`
	MentionIDs       []string      `json:"mention_ids"`
	Reactions        []struct{}    `json:"reactions"`
	Pinned           bool          `json:"pinned"`
	Components       []struct{}    `json:"components"`
	Version          int64         `json:"version"`
	EditedAt         string        `json:"edited_at,omitempty"`
	DeletedAt        string        `json:"deleted_at,omitempty"`
	CreatedAt        string        `json:"created_at"`
}

// messageBody converts a message into the JSON form of the schema Message.
func messageBody(m messaging.Message) messageJSON {
	mentions := make([]string, 0, len(m.MentionIDs))
	for _, id := range m.MentionIDs {
		mentions = append(mentions, id.String())
	}
	return messageJSON{
		ID: m.ID.String(), ThreadID: m.ThreadID.String(), ChannelID: m.ChannelID.String(), ThreadSeq: m.ThreadSeq,
		Author: principalJSON{
			ID: m.Author.ID.String(), Kind: m.Author.Kind, Handle: m.Author.Handle, DisplayName: m.Author.DisplayName,
			AvatarFileID: optionalID(m.Author.AvatarFileID),
		},
		ClientMsgID: m.ClientMsgID.String(), Origin: m.Origin, Kind: m.Kind, SystemCode: m.SystemCode, ContentMD: m.ContentMD,
		ContentFormat: m.ContentFormat, ReplyToMessageID: optionalID(m.ReplyToID), Silent: m.Silent, Attachments: []struct{}{},
		MentionIDs: mentions, Reactions: []struct{}{}, Components: []struct{}{}, Version: m.Version, EditedAt: stamp(m.EditedAt),
		DeletedAt: stamp(m.DeletedAt), CreatedAt: stamp(m.CreatedAt),
	}
}

// payload encodes the payload MessagePayload of body without HTML escaping, so the size is the
// size of the content.
func payload(body messageJSON) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(struct {
		Message messageJSON `json:"message"`
	}{body}); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, err, "messaging: message.created payload")
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
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
		return nil, false, errs.New(errs.CodeInternalError, "messaging: message.created does not fit sync.event_payload_max_bytes")
	}
	return out, true, nil
}

// emitCreated appends message.created for m in the thread of info (brief D7): resource the
// message with version 1, scope space, channel and thread, receive action message.read and, for
// a message of a client, the echo of client_msg_id.
func (s *Service) emitCreated(ctx context.Context, info chat.ThreadInfo, m messaging.Message, echo bool) error {
	p, truncated, err := s.eventPayload(m)
	if err != nil {
		return err
	}
	kind, actorID := eventActor(ctx)
	ev := eventlog.Event{
		Type: string(authorize.EventMessageCreated), SchemaVersion: 1, ActorKind: kind, ActorPrincipalID: actorID,
		SpaceID: info.SpaceID, ChannelID: m.ChannelID, ThreadID: m.ThreadID, ResourceKind: resourceMessage, ResourceID: m.ID,
		ResourceVersion: m.Version, ReceiveAction: receiveMessageRead, Payload: p, Truncated: truncated,
	}
	if echo {
		ev.ClientMsgID = m.ClientMsgID
	}
	return s.d.Events.Append(ctx, ev)
}

// eventActor returns the actor of the request in ctx; none outside a request.
func eventActor(ctx context.Context) (eventlog.ActorKind, ids.ID) {
	a, ok := actor.FromContext(ctx)
	if !ok || a.ID.IsZero() {
		return "", ids.ID{}
	}
	switch a.Kind {
	case actor.KindUser:
		return eventlog.ActorUser, a.ID
	case actor.KindAgent:
		return eventlog.ActorAgent, a.ID
	case actor.KindApp:
		return eventlog.ActorApp, a.ID
	case actor.KindHost, actor.KindAnonymous:
	}
	return "", ids.ID{}
}
