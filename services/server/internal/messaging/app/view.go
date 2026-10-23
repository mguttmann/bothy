// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
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
// request: its author, mentions, attachments, reactions, own reactions and pin now, read with the
// visible channels of the caller.
func (s *Service) stored(ctx context.Context, m domain.Message, visible []ids.ID) (messaging.Message, error) {
	return s.storedWith(ctx, m, visible, nil)
}

// storedWith is stored with the attachments a caller already holds (the result of Attach in the
// same request, brief M3-07 design note 6), so decorate does not read them again; nil reads them.
func (s *Service) storedWith(ctx context.Context, m domain.Message, visible []ids.ID, attached []messaging.Attachment) (messaging.Message, error) {
	v, err := s.base(ctx, m)
	if err != nil {
		return messaging.Message{}, err
	}
	if v.HasAttachments && len(attached) > 0 {
		v.Attachments = attached
	}
	list := []messaging.Message{v}
	if err := s.decorate(ctx, visible, list); err != nil {
		return messaging.Message{}, err
	}
	return list[0], nil
}

// decorate fills the attachments (attachments, M3-07), reactions, my_reactions (for the
// principal of the request) and pinned of the live messages of list with one statement each for
// the whole list (brief M1-59 design note 7), and the components of the live cards with one
// ComponentReader call (M2-15 design note 7); a tombstone keeps empty attachments and reactions,
// no components and pinned false. It is the one decoration of every representation (stored,
// History, Get, the mention list, the export, SetTriggerState).
func (s *Service) decorate(ctx context.Context, visible []ids.ID, list []messaging.Message) error {
	if err := s.attachments(ctx, list); err != nil {
		return err
	}
	if err := s.attachComponents(ctx, list); err != nil {
		return err
	}
	return s.decorateReactions(ctx, visible, list)
}

// decorateReactions is the reaction and pin part of decorate (brief M1-59 design note 7).
func (s *Service) decorateReactions(ctx context.Context, visible []ids.ID, list []messaging.Message) error {
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
	applyReactions(list, groups, own, pinned)
	return nil
}

// applyReactions sets reactions, my_reactions and pinned of the live messages of list from the
// reads of decorate; a tombstone keeps empty reactions and pinned false.
func applyReactions(list []messaging.Message, groups map[ids.ID][]domain.ReactionGroup, own map[ids.ID]map[string]bool, pinned map[ids.ID]bool) {
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
}

// attachments fills the attachments of the live messages of list whose has_attachments is set and
// that have none yet, with one ForMessages call of files for the whole list (brief M3-07 design
// note 6); a list without such a message makes no call (perf.message_create_p99), and a tombstone
// keeps []. The caller read every message of list before (message.read or its own write).
func (s *Service) attachments(ctx context.Context, list []messaging.Message) error {
	var want []ids.ID
	for _, m := range list {
		if m.HasAttachments && m.DeletedAt.IsZero() && len(m.Attachments) == 0 {
			want = append(want, m.ID)
		}
	}
	if len(want) == 0 || s.d.Files == nil {
		return nil
	}
	got, err := s.d.Files.ForMessages(ctx, want)
	if err != nil {
		return err
	}
	for i := range list {
		if found, ok := got[list[i].ID]; ok && len(list[i].Attachments) == 0 {
			list[i].Attachments = attachmentsOf(found)
		}
	}
	return nil
}

// attachmentsOf maps the attachments of files into the port of messaging (02 section 7.2: the
// port of messaging names no other module): position order as files returns it, never nil, and
// the attached form of the file without scope_channel_id (docs/decisions.d/M3-07.md D-M3-07-5).
func attachmentsOf(list []files.Attachment) []messaging.Attachment {
	out := make([]messaging.Attachment, 0, len(list))
	for _, a := range list {
		f := a.File
		thumbs := make([]messaging.AttachedThumbnail, 0, len(f.Thumbnails))
		for _, t := range f.Thumbnails {
			thumbs = append(thumbs, messaging.AttachedThumbnail{Variant: t.Variant, MIMEType: t.MIMEType, Width: t.Width, Height: t.Height})
		}
		out = append(out, messaging.Attachment{Position: a.Position, File: messaging.AttachedFile{
			ID: f.ID, SHA256: f.SHA256, SizeBytes: f.SizeBytes, MIMEType: f.MIMEType, OriginalName: f.OriginalName, UploadedBy: f.UploadedBy,
			Purpose: f.Purpose, UploadState: f.UploadState, ScanStatus: f.ScanStatus, Width: f.Width, Height: f.Height, DurationMS: f.DurationMS,
			TranscriptStatus: f.TranscriptStatus, TranscriptLanguage: f.TranscriptLanguage, TranscriptSource: f.TranscriptSource,
			Thumbnails: thumbs, CreatedAt: f.CreatedAt,
		}})
	}
	return out
}

// view converts a row into the port DTO; the mentions and attachments are never nil. It is the
// one representation of a message (brief M1-57 note 6, also for M1-58, M1-59 and M3-07): a
// deleted message is a tombstone with empty content, no mentions, no attachments, no reactions, no
// components and not pinned, every other field as stored; run_id and the trigger state are not
// content, so a tombstone keeps them (03 messages, M2-05). Attachments, reactions, the pin and the
// components start empty; decorate fills them for a stored message.
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
		HasAttachments: m.HasAttachments && m.DeletedAt.IsZero(), Attachments: []messaging.Attachment{},
		Reactions: []messaging.ReactionCount{}, MyReactions: []string{}, Components: []messaging.Component{}, Version: m.Version,
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

// attachmentJSON is the schema Attachment: the position from 1 and the file.
type attachmentJSON struct {
	Position int      `json:"position"`
	File     fileJSON `json:"file"`
}

// fileJSON is the schema File of an attached file (brief M3-07 design note 6): scope_channel_id is
// never shown once a file is attached, sha256 only when files set it (upload_state complete), the
// thumbnails always an array; other optional members are left out, never null (ADR 0053).
type fileJSON struct {
	ID                 string          `json:"id"`
	SHA256             string          `json:"sha256,omitempty"`
	SizeBytes          int64           `json:"size_bytes"`
	MIMEType           string          `json:"mime_type"`
	OriginalName       string          `json:"original_name"`
	UploadedBy         string          `json:"uploaded_by"`
	Purpose            string          `json:"purpose"`
	UploadState        string          `json:"upload_state"`
	ScanStatus         string          `json:"scan_status"`
	Width              int             `json:"width,omitempty"`
	Height             int             `json:"height,omitempty"`
	DurationMS         int64           `json:"duration_ms,omitempty"`
	TranscriptStatus   string          `json:"transcript_status"`
	TranscriptLanguage string          `json:"transcript_language,omitempty"`
	TranscriptSource   string          `json:"transcript_source,omitempty"`
	Thumbnails         []thumbnailJSON `json:"thumbnails"`
	CreatedAt          string          `json:"created_at"`
}

// thumbnailJSON is the schema Thumbnail.
type thumbnailJSON struct {
	Variant  string `json:"variant"`
	MIMEType string `json:"mime_type"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// attachmentsBody converts the attachments of a message into their event form, in position
// order; never nil.
func attachmentsBody(list []messaging.Attachment) []attachmentJSON {
	out := make([]attachmentJSON, 0, len(list))
	for _, a := range list {
		f := a.File
		thumbs := make([]thumbnailJSON, 0, len(f.Thumbnails))
		for _, t := range f.Thumbnails {
			thumbs = append(thumbs, thumbnailJSON{Variant: t.Variant, MIMEType: t.MIMEType, Width: t.Width, Height: t.Height})
		}
		out = append(out, attachmentJSON{Position: a.Position, File: fileJSON{
			ID: f.ID.String(), SHA256: f.SHA256, SizeBytes: f.SizeBytes, MIMEType: f.MIMEType, OriginalName: f.OriginalName,
			UploadedBy: f.UploadedBy.String(), Purpose: f.Purpose, UploadState: f.UploadState, ScanStatus: f.ScanStatus, Width: f.Width,
			Height: f.Height, DurationMS: f.DurationMS, TranscriptStatus: f.TranscriptStatus, TranscriptLanguage: f.TranscriptLanguage,
			TranscriptSource: f.TranscriptSource, Thumbnails: thumbs, CreatedAt: stamp(f.CreatedAt),
		}})
	}
	return out
}

// messageJSON is the schema Message in an event (brief D7): the attachments in position order
// (M3-07), the reaction summary, pinned (M1-59) and the components (M2-15) as decorated, never
// my_reactions, which only REST responses carry. Optional members are left out, never null (ADR
// 0053).
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
	Attachments      []attachmentJSON    `json:"attachments"`
	MentionIDs       []string            `json:"mention_ids"`
	Reactions        []reactionCountJSON `json:"reactions"`
	Pinned           bool                `json:"pinned"`
	Components       []componentJSON     `json:"components"`
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
		TriggerStateAt: stamp(m.TriggerStateAt), TriggerReason: m.TriggerStateReason, Silent: m.Silent, Attachments: attachmentsBody(m.Attachments),
		MentionIDs: mentions, Reactions: reactions, Pinned: m.Pinned, Components: componentsBody(m.Components), Version: m.Version, EditedAt: stamp(m.EditedAt),
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
