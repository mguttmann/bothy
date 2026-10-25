// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/chat"
	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/actor"
	"github.com/OWNER/bothy/services/server/internal/kernel/authorize"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/personaldata"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// personalBatch is the number of rows one read of the export or one round of the content deletion
// takes: small enough for short statements and transactions, large enough for few round trips.
const personalBatch = 500

// PersonalMessage is one message of the person for the export (PersonalStore.OwnMessages).
type PersonalMessage struct {
	ID, ThreadID, ChannelID ids.ID
	// Kind is text, voice, run or card; system lines are never read.
	Kind string
	// ContentMD is the current text; empty for a tombstone.
	ContentMD string
	// HasAttachments is messages.has_attachments: only these messages are asked for files.
	HasAttachments bool
	// CreatedAt is the send time; EditedAt and DeletedAt are zero when not edited or deleted.
	CreatedAt, EditedAt, DeletedAt time.Time
}

// PersonalReaction is one reaction of the person (PersonalStore.PersonalReactions).
type PersonalReaction struct {
	MessageID ids.ID
	Emoji     string
	CreatedAt time.Time
}

// OwnReaction is one reaction of the person on a message that is not deleted, with the channel of
// the message (PersonalStore.LiveOwnReactions, the content step of the account deletion).
type OwnReaction struct {
	MessageID, ChannelID ids.ID
	Emoji                string
}

// PersonalMention is one mention of the person (PersonalStore.MentionsOf): the message and the
// time only, because the message belongs to another person.
type PersonalMention struct {
	MessageID ids.ID
	CreatedAt time.Time
}

// PersonalStore is the outgoing port of the personal data of messaging (M7-06); adapters/postgres
// implements it on the Repository. Every read selects by the person's own ID and runs on the pool
// (each statement its own short read-only transaction); DeletePersonalTemplates is one statement.
// Errors are kernel/errs errors.
type PersonalStore interface {
	// OwnMessages returns up to limit messages authored by author with an id above after, by id,
	// tombstones included, system lines excluded.
	OwnMessages(ctx context.Context, author, after ids.ID, limit int) ([]PersonalMessage, error)
	// OwnRevisions returns the revisions of messages that author made, by message and number.
	OwnRevisions(ctx context.Context, author ids.ID, messages []ids.ID) ([]domain.Revision, error)
	// PersonalReactions returns up to limit reactions of principal after the keyset position after
	// (message, symbol), in that order; the zero value starts at the beginning.
	PersonalReactions(ctx context.Context, principal ids.ID, after PersonalReaction, limit int) ([]PersonalReaction, error)
	// MentionsOf returns up to limit mentions of principal with a message id above after, by
	// message id.
	MentionsOf(ctx context.Context, principal, after ids.ID, limit int) ([]PersonalMention, error)
	// AllPersonalTemplates returns every personal template of owner, deleted ones included, by id.
	AllPersonalTemplates(ctx context.Context, owner ids.ID) ([]domain.Template, error)
	// DeletePersonalTemplates deletes every personal template of owner and returns their number.
	DeletePersonalTemplates(ctx context.Context, owner ids.ID) (int64, error)
	// LiveOwnMessages returns up to limit messages of author that are not deleted, with their
	// channel, by id (the content step of the account deletion), system lines excluded.
	LiveOwnMessages(ctx context.Context, author ids.ID, limit int) ([]ExpiredMessage, error)
	// LiveOwnReactions returns up to limit reactions of principal on messages that are not
	// deleted, with the channel of the message, by message and symbol (the content step).
	LiveOwnReactions(ctx context.Context, principal ids.ID, limit int) ([]OwnReaction, error)
}

// PlaceNames is the part of chat.Structure the export needs: the channel name and the thread title
// of a thread (chat's PreviewNames, which never authorizes; the export asks
// personaldata.Export.CanRead first).
type PlaceNames interface {
	// PreviewNames returns the space, channel and thread names of threadID ("" for none).
	PreviewNames(ctx context.Context, threadID ids.ID) (chat.PreviewNames, error)
}

// AttachmentReader is the part of files.Attachments the export needs: the attachments of the
// person's messages, for their file IDs.
type AttachmentReader interface {
	// ForMessages returns the attachments of messageIDs in position order by message.
	ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]files.Attachment, error)
}

// PersonalData is the participant of messaging in the privacy requests (M7-06,
// docs/spec/20-privacy.md sections 4.2, 4.4 and 5.2). It lives in the application layer because
// step 5 deletes messages through the deletion use case of 20 4.2 (Retention.remove, the same path
// as retention.content: tombstone, revisions, mentions, reactions, pin and attachments removed,
// payloads redacted, message.deleted without deleted_by, no audit entry). It is safe for
// concurrent use.
type PersonalData struct {
	s     *Service
	store PersonalStore
	names PlaceNames
	att   AttachmentReader
}

// Compile-time check: PersonalData is a participant of kernel/personaldata.
var _ personaldata.Participant = (*PersonalData)(nil)

// NewPersonalData returns the participant over the use cases s, the store of the module, the
// names of chat (chat's Service) and the attachments of files (files.Attachments; nil exports no
// attachment IDs, for a role or a test without the files module).
//
// Errors: s, store or names is nil.
//
// Example:
//
//	p, err := app.NewPersonalData(m.Service, postgres.NewRepository(pool), chatService, filesService)
func NewPersonalData(s *Service, store PersonalStore, names PlaceNames, att AttachmentReader) (*PersonalData, error) {
	if s == nil || store == nil || names == nil {
		return nil, errors.New("messaging: NewPersonalData needs the service, the store and the names of chat")
	}
	return &PersonalData{s: s, store: store, names: names, att: att}, nil
}

// Module returns messaging, the module of architecture/modules.yaml.
func (*PersonalData) Module() string { return "messaging" }

// Tables returns the tables of messaging that 20 section 2 marks with a personal reference. pins
// is listed but neither exported nor erased: who pinned a message stays as evidence with the
// pseudonymous ID, and the pin goes with its message (20 section 2).
func (*PersonalData) Tables() []string {
	return []string{"messages", "message_revisions", "mentions", "reactions", "pins", "message_templates"}
}

// exportRevisionJSON is one revision of a messages.jsonl line.
type exportRevisionJSON struct {
	Revision  int       `json:"revision"`
	ContentMD string    `json:"content_md"`
	CreatedAt time.Time `json:"created_at"`
}

// exportMessageJSON is one line of messages.jsonl (docs/api/gdpr-export/v1/lines/messages.schema.json).
type exportMessageJSON struct {
	ID                string               `json:"id"`
	ThreadID          string               `json:"thread_id"`
	ChannelID         string               `json:"channel_id"`
	ChannelName       string               `json:"channel_name,omitempty"`
	ThreadTitle       string               `json:"thread_title,omitempty"`
	Kind              string               `json:"kind"`
	ContentMD         string               `json:"content_md"`
	CreatedAt         time.Time            `json:"created_at"`
	EditedAt          time.Time            `json:"edited_at,omitzero"`
	Deleted           bool                 `json:"deleted"`
	DeletedAt         time.Time            `json:"deleted_at,omitzero"`
	Revisions         []exportRevisionJSON `json:"revisions"`
	AttachmentFileIDs []string             `json:"attachment_file_ids,omitempty"`
}

// exportReactionJSON is one line of reactions.jsonl (lines/reactions.schema.json).
type exportReactionJSON struct {
	MessageID string    `json:"message_id"`
	Emoji     string    `json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

// exportMentionJSON is one line of mentions.jsonl (lines/mentions.schema.json): never the text or the
// author of the mentioning message.
type exportMentionJSON struct {
	MessageID string    `json:"message_id"`
	CreatedAt time.Time `json:"created_at"`
}

// exportTemplateJSON is one template of the section templates of personal.json
// (sections/personal.templates.schema.json).
type exportTemplateJSON struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ContentMD string    `json:"content_md"`
	CreatedAt time.Time `json:"created_at"`
	DeletedAt time.Time `json:"deleted_at,omitzero"`
}

// Export implements personaldata.Participant: messages.jsonl, reactions.jsonl, mentions.jsonl and
// the section templates of personal.json for e.Subject (20 section 5.2). Only the person's own
// messages, reactions and personal templates, and of the mentions only message ID and time; the
// channel name and thread title only while e.CanRead allows the channel. Errors: those of the
// storage, of chat, of files, of e.Readable and of w.
func (p *PersonalData) Export(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	if err := p.exportMessages(ctx, e, w); err != nil {
		return err
	}
	if err := p.exportReactions(ctx, e.Subject, w); err != nil {
		return err
	}
	if err := p.exportMentions(ctx, e.Subject, w); err != nil {
		return err
	}
	return p.exportTemplates(ctx, e.Subject, w)
}

// places caches the readability of channels and the names of threads during one export, so a
// person with many messages in one thread costs one check and one read.
type places struct {
	p        *PersonalData
	e        personaldata.Export
	readable map[ids.ID]bool
	names    map[ids.ID]chat.PreviewNames
}

// of returns the channel name and thread title of m, both "" when the person may no longer read
// its channel (20 section 5.2: then only the IDs).
func (c *places) of(ctx context.Context, m PersonalMessage) (channel, thread string, err error) {
	ok, seen := c.readable[m.ChannelID]
	if !seen {
		if ok, err = c.e.CanRead(ctx, m.ChannelID); err != nil {
			return "", "", err
		}
		c.readable[m.ChannelID] = ok
	}
	if !ok {
		return "", "", nil
	}
	n, seen := c.names[m.ThreadID]
	if !seen {
		if n, err = c.p.names.PreviewNames(ctx, m.ThreadID); err != nil {
			return "", "", err
		}
		c.names[m.ThreadID] = n
	}
	return n.ChannelName, n.ThreadTitle, nil
}

// exportMessages writes one line per own message, batch by batch.
func (p *PersonalData) exportMessages(ctx context.Context, e personaldata.Export, w personaldata.Writer) error {
	c := &places{p: p, e: e, readable: map[ids.ID]bool{}, names: map[ids.ID]chat.PreviewNames{}}
	after := ids.ID{}
	for {
		batch, err := p.store.OwnMessages(ctx, e.Subject, after, personalBatch)
		if err != nil || len(batch) == 0 {
			return err
		}
		lines, err := p.messageLines(ctx, e.Subject, batch, c)
		if err != nil {
			return err
		}
		for _, l := range lines {
			if err := w.Line(personaldata.FileMessages, l); err != nil {
				return err
			}
		}
		if len(batch) < personalBatch {
			return nil
		}
		after = batch[len(batch)-1].ID
	}
}

// messageLines converts one batch with its revisions and attachments, read once per batch.
func (p *PersonalData) messageLines(ctx context.Context, subject ids.ID, batch []PersonalMessage, c *places) ([]exportMessageJSON, error) {
	revs, attached, err := p.details(ctx, subject, batch)
	if err != nil {
		return nil, err
	}
	out := make([]exportMessageJSON, 0, len(batch))
	for _, m := range batch {
		channel, thread, err := c.of(ctx, m)
		if err != nil {
			return nil, err
		}
		l := exportMessageJSON{
			ID: m.ID.String(), ThreadID: m.ThreadID.String(), ChannelID: m.ChannelID.String(), ChannelName: channel, ThreadTitle: thread,
			Kind: m.Kind, ContentMD: m.ContentMD, CreatedAt: m.CreatedAt.UTC(), EditedAt: utc(m.EditedAt), Deleted: !m.DeletedAt.IsZero(),
			DeletedAt: utc(m.DeletedAt), Revisions: revs[m.ID],
		}
		if l.Revisions == nil {
			l.Revisions = []exportRevisionJSON{}
		}
		for _, a := range attached[m.ID] {
			l.AttachmentFileIDs = append(l.AttachmentFileIDs, a.File.ID.String())
		}
		out = append(out, l)
	}
	return out, nil
}

// details reads the revisions the person made of the messages of batch and, through files, the
// attachments of those with has_attachments, each in one call per batch.
func (p *PersonalData) details(ctx context.Context, subject ids.ID, batch []PersonalMessage) (
	map[ids.ID][]exportRevisionJSON, map[ids.ID][]files.Attachment, error) {
	all := make([]ids.ID, 0, len(batch))
	var withFiles []ids.ID
	for _, m := range batch {
		all = append(all, m.ID)
		if m.HasAttachments {
			withFiles = append(withFiles, m.ID)
		}
	}
	list, err := p.store.OwnRevisions(ctx, subject, all)
	if err != nil {
		return nil, nil, err
	}
	revs := map[ids.ID][]exportRevisionJSON{}
	for _, r := range list {
		revs[r.MessageID] = append(revs[r.MessageID], exportRevisionJSON{Revision: r.Number, ContentMD: r.ContentMD, CreatedAt: r.CreatedAt.UTC()})
	}
	attached := map[ids.ID][]files.Attachment{}
	if p.att != nil && len(withFiles) > 0 {
		if attached, err = p.att.ForMessages(ctx, withFiles); err != nil {
			return nil, nil, err
		}
	}
	return revs, attached, nil
}

// exportReactions writes one line per own reaction, batch by batch.
func (p *PersonalData) exportReactions(ctx context.Context, subject ids.ID, w personaldata.Writer) error {
	after := PersonalReaction{}
	for {
		batch, err := p.store.PersonalReactions(ctx, subject, after, personalBatch)
		if err != nil || len(batch) == 0 {
			return err
		}
		for _, r := range batch {
			if err := w.Line(personaldata.FileReactions, exportReactionJSON{MessageID: r.MessageID.String(), Emoji: r.Emoji, CreatedAt: r.CreatedAt.UTC()}); err != nil {
				return err
			}
		}
		if len(batch) < personalBatch {
			return nil
		}
		after = batch[len(batch)-1]
	}
}

// exportMentions writes one line per mention of the person, batch by batch.
func (p *PersonalData) exportMentions(ctx context.Context, subject ids.ID, w personaldata.Writer) error {
	after := ids.ID{}
	for {
		batch, err := p.store.MentionsOf(ctx, subject, after, personalBatch)
		if err != nil || len(batch) == 0 {
			return err
		}
		for _, m := range batch {
			if err := w.Line(personaldata.FileMentions, exportMentionJSON{MessageID: m.MessageID.String(), CreatedAt: m.CreatedAt.UTC()}); err != nil {
				return err
			}
		}
		if len(batch) < personalBatch {
			return nil
		}
		after = batch[len(batch)-1].MessageID
	}
}

// exportTemplates writes the section templates: every personal template, deleted ones that are
// still stored included, because the stored text is data of the person until step 3 deletes it.
func (p *PersonalData) exportTemplates(ctx context.Context, subject ids.ID, w personaldata.Writer) error {
	list, err := p.store.AllPersonalTemplates(ctx, subject)
	if err != nil {
		return err
	}
	out := make([]exportTemplateJSON, 0, len(list))
	for _, t := range list {
		out = append(out, exportTemplateJSON{ID: t.ID.String(), Name: t.Name, ContentMD: t.ContentMD, CreatedAt: t.CreatedAt.UTC(), DeletedAt: utc(t.DeletedAt)})
	}
	return w.Section(personaldata.FilePersonal, "templates", out)
}

// utc returns t in UTC and keeps the zero time zero, so omitzero leaves it out.
func utc(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.UTC()
}

// Erase implements personaldata.Participant. StepPersonal deletes the personal templates of the
// person in one statement (the channel templates the person wrote stay, like every created_by).
// StepContent with personaldata.DeleteContent deletes every own message that is not deleted yet
// through the deletion path of 20 4.2 without an actor, one unit of work per message, so a
// resumed call finds only the rest, and then removes the reactions of the person on the messages
// that stay (those of others) like reaction.remove, so clients get reaction.removed; with
// personaldata.KeepPseudonymized messages and reactions stay (20 section 4.4 item 5 names them as
// what stays only in that mode). Every other step has nothing to do in messaging. Errors: an unknown content mode, those
// of the storage and of the deletion path.
func (p *PersonalData) Erase(ctx context.Context, step personaldata.Step, e personaldata.Erasure) error {
	switch step {
	case personaldata.StepPersonal:
		_, err := p.store.DeletePersonalTemplates(ctx, e.Subject)
		return err
	case personaldata.StepContent:
		if !e.Content.Valid() {
			return errs.New(errs.CodeInternalError, fmt.Sprintf("messaging: unknown privacy.account_deletion_content %q", e.Content))
		}
		if e.Content != personaldata.DeleteContent {
			return nil
		}
		if err := p.deleteOwnMessages(ctx, e.Subject); err != nil {
			return err
		}
		return p.removeOwnReactions(ctx, e.Subject)
	case personaldata.StepDisable, personaldata.StepSecurity, personaldata.StepAccess, personaldata.StepPseudonymize, personaldata.StepAudit:
		return nil
	default:
		return nil
	}
}

// deleteOwnMessages deletes the messages of subject batch by batch until none is left. A message
// moved to another channel between the read and its unit of work is skipped and read again with
// its new channel in the next round; a round that deletes nothing although messages are left
// ends with an error, so a persistent refusal cannot loop and the job retries the step later.
func (p *PersonalData) deleteOwnMessages(ctx context.Context, subject ids.ID) error {
	r := p.s.Retention()
	for {
		batch, err := p.store.LiveOwnMessages(ctx, subject, personalBatch)
		if err != nil || len(batch) == 0 {
			return err
		}
		deleted := 0
		for _, m := range batch {
			done, err := r.remove(ctx, m)
			if err != nil {
				return err
			}
			if done {
				deleted++
			}
		}
		if deleted == 0 {
			return errs.New(errs.CodeInternalError, "messaging: the content step deleted none of the remaining messages")
		}
	}
}

// removeOwnReactions removes the reactions of subject on live messages batch by batch, each in its
// own unit of work through the statements of reaction.remove, with reaction.removed naming the
// person and no actor (the system acts). A reaction gone or a message deleted or moved since the
// read is skipped and read again in the next round; a round that removes nothing although
// reactions are left ends with an error, so the job retries the step instead of looping.
func (p *PersonalData) removeOwnReactions(ctx context.Context, subject ids.ID) error {
	person := actor.Actor{ID: subject, Kind: actor.KindUser}
	for {
		batch, err := p.store.LiveOwnReactions(ctx, subject, personalBatch)
		if err != nil || len(batch) == 0 {
			return err
		}
		removed := 0
		for _, r := range batch {
			done, err := p.removeReaction(ctx, person, r)
			if err != nil {
				return err
			}
			if done {
				removed++
			}
		}
		if removed == 0 {
			return errs.New(errs.CodeInternalError, "messaging: the content step removed none of the remaining reactions")
		}
	}
}

// removeReaction removes the reaction r of person in one unit of work and reports whether it did.
func (p *PersonalData) removeReaction(ctx context.Context, person actor.Actor, r OwnReaction) (bool, error) {
	done := false
	visible := []ids.ID{r.ChannelID}
	err := p.s.d.UoW.Do(ctx, func(ctx context.Context) error {
		done = false
		row, found, err := p.s.d.Store.LockMessage(ctx, r.MessageID, visible)
		if err != nil || !found || !row.DeletedAt.IsZero() {
			return err
		}
		_, ofEmoji, err := p.s.d.Store.ReactionStats(ctx, row.ID, r.Emoji, visible)
		if err != nil {
			return err
		}
		deleted, err := p.s.d.Store.DeleteReaction(ctx, row.ID, person.ID, r.Emoji, visible)
		if err != nil || !deleted {
			return err
		}
		done = true
		return p.s.emitReaction(ctx, authorize.EventReactionRemoved, row, person, r.Emoji, ofEmoji-1)
	})
	return done, err
}
