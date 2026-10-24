// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
	"github.com/OWNER/bothy/services/server/internal/messaging"
	"github.com/OWNER/bothy/services/server/internal/messaging/domain"
)

// ImportStore is the outgoing port of the imported messages (M8-11); adapters/postgres
// implements it. It needs a unit of work. Errors are kernel/errs errors without SQL text or
// values.
type ImportStore interface {
	// InsertImportedMessage inserts m with origin import, kind text, silent, its import_item_id
	// and edited_at, and returns the stored row.
	InsertImportedMessage(ctx context.Context, m domain.Message) (domain.Message, error)
}

// The pointers of the import data that the errors of the port name (schema ImportMessageData and
// ImportPinData).
const (
	importContentPointer     = "/content_md"
	importReplyPointer       = "/reply_to_origin_key"
	importAttachmentsPointer = "/attachment_file_ids"
	importPinPointer         = "/message_origin_key"
	// importHashContext is the context of the request hash of an imported message, followed by the
	// id of its item: the hash is stored like every hash and never compared, because the origin key
	// carries the idempotency (22 6.2).
	importHashContext = "import:"
)

// ImportMessagesPort implements messaging.ImportMessages over the service (M8-11).
type ImportMessagesPort struct{ s *Service }

// Compile-time check: the import port of the messages.
var _ messaging.ImportMessages = ImportMessagesPort{}

// ImportMessages returns the port messaging.ImportMessages over s.
func (s *Service) ImportMessages() ImportMessagesPort { return ImportMessagesPort{s: s} }

// importDeps checks the unit of work and the collaborators of the import port.
func (s *Service) importDeps(ctx context.Context) error {
	if !txn.InUnit(ctx) {
		return errs.New(errs.CodeInternalError, "messaging: ImportMessages runs inside the caller's unit of work")
	}
	if s.d.ImportThreads == nil || s.d.Imported == nil {
		return errs.New(errs.CodeInternalError, "messaging: the service has no import collaborators")
	}
	return nil
}

// importFieldError is a 400 validation_failed with one entry.
func importFieldError(pointer, code, detail string) error {
	return errs.New(errs.CodeValidationFailed, detail).WithFields(errs.FieldError{Pointer: pointer, Code: code, Detail: detail})
}

// checkImported applies the content rules of a new message to an imported one (22 6.3, M8-11
// design note 2): at most message.max_content_bytes (too_long, never split), no U+0000, a visible
// character unless the message has attachments, and at most message.max_attachments files.
func (s *Service) checkImported(m messaging.ImportedMessage) error {
	if len(m.AttachmentIDs) > 0 && s.d.Files == nil {
		return domain.NotEnabled(importAttachmentsPointer)
	}
	if err := domain.CheckAttachments(len(m.AttachmentIDs), s.cfg.MaxAttachments, importAttachmentsPointer); err != nil {
		return err
	}
	if len(m.ContentMD) > s.cfg.MaxContentBytes {
		return importFieldError(importContentPointer, "too_long", "content_md is larger than message.max_content_bytes")
	}
	return domain.CheckNewContent(m.ContentMD, s.cfg.MaxContentBytes, len(m.AttachmentIDs) > 0)
}

// checkImportedReply allows a reply only to a message of the thread (22 6.2); every other target
// is not_in_thread on the origin key of the reply.
func (s *Service) checkImportedReply(ctx context.Context, target, thread ids.ID) error {
	if target.IsZero() {
		return nil
	}
	m, found, err := s.d.Store.Message(ctx, target)
	if err != nil {
		return err
	}
	if !found || m.ThreadID != thread {
		return importFieldError(importReplyPointer, domain.FieldNotInThread, "the reply target is not in this thread")
	}
	return nil
}

// Append implements messaging.ImportMessages (M8-11 design note 4, 22 6.2 and 6.4): the checks of
// every message first, then the numbers of the thread in one Reserve under its row lock, then per
// message the row (origin import, silent, the instance search language, import_item_id,
// created_at and edited_at of the source) and its attachments through files.Attachments.Attach
// without a check of upload by the author. Nothing is published and no event is appended.
func (p ImportMessagesPort) Append(ctx context.Context, threadID ids.ID, msgs []messaging.ImportedMessage) ([]ids.ID, error) {
	s := p.s
	if err := s.importDeps(ctx); err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return []ids.ID{}, nil
	}
	last := time.Time{}
	for _, m := range msgs {
		if err := s.checkImported(m); err != nil {
			return nil, err
		}
		if err := s.checkImportedReply(ctx, m.ReplyToID, threadID); err != nil {
			return nil, err
		}
		if m.CreatedAt.After(last) {
			last = m.CreatedAt
		}
	}
	res, err := s.d.ImportThreads.Reserve(ctx, threadID, len(msgs), last)
	if err != nil {
		return nil, err
	}
	lang, err := s.d.Directory.InstanceSearchLanguage(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ids.ID, 0, len(msgs))
	for i, m := range msgs {
		id, err := s.insertImported(ctx, res.ThreadID, res.ChannelID, res.FirstSeq+int64(i), lang, m)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// insertImported writes one imported message at seq and binds its files.
func (s *Service) insertImported(ctx context.Context, thread, channel ids.ID, seq int64, lang string, m messaging.ImportedMessage) (ids.ID, error) {
	reply := ""
	if !m.ReplyToID.IsZero() {
		reply = m.ReplyToID.String()
	}
	row, err := s.d.Imported.InsertImportedMessage(ctx, domain.Message{
		ID: ids.New(), ThreadID: thread, ChannelID: channel, ThreadSeq: seq, AuthorID: m.AuthorID, ClientMsgID: ids.New(),
		PayloadSHA256: domain.RequestHash(domain.HashInput{Context: importHashContext + m.ImportItemID.String(), Kind: string(domain.KindText),
			Content: m.ContentMD, ReplyTo: reply, AttachmentIDs: idStrings(m.AttachmentIDs)}),
		Origin: domain.OriginImport, Kind: domain.KindText, ContentMD: m.ContentMD, ContentFormat: domain.ContentFormat, ReplyToID: m.ReplyToID,
		Silent: true, HasAttachments: len(m.AttachmentIDs) > 0, ImportItemID: m.ImportItemID, Version: 1, EditedAt: m.EditedAt,
		CreatedAt: m.CreatedAt, SearchLanguage: lang,
	})
	if err != nil {
		return ids.ID{}, err
	}
	s.countCreated(ctx, row)
	if len(m.AttachmentIDs) == 0 {
		return row.ID, nil
	}
	_, err = s.d.Files.Attach(ctx, files.AttachCmd{
		MessageID: row.ID, ThreadID: thread, ChannelID: channel, AuthorID: m.AuthorID, FileIDs: slices.Clone(m.AttachmentIDs),
		Pointer: importAttachmentsPointer,
	})
	return row.ID, err
}

// ThreadOf implements messaging.ImportMessages.
func (p ImportMessagesPort) ThreadOf(ctx context.Context, messageID ids.ID) (ids.ID, bool, error) {
	m, found, err := p.s.d.Store.Message(ctx, messageID)
	if err != nil || !found {
		return ids.ID{}, false, err
	}
	return m.ThreadID, true, nil
}

// Pin implements messaging.ImportMessages (22 6.6): the thread lock and the message row lock in
// the lock order of pinMessage, then the rules of insertPin without pin.added: an existing pin is
// no change, pin.max_per_thread holds.
func (p ImportMessagesPort) Pin(ctx context.Context, threadID, messageID, pinnedBy ids.ID) error {
	s := p.s
	if err := s.importDeps(ctx); err != nil {
		return err
	}
	m, found, err := s.d.Store.Message(ctx, messageID)
	if err != nil {
		return err
	}
	if !found || m.ThreadID != threadID {
		return importFieldError(importPinPointer, domain.FieldNotInThread, "the pinned message is not in this thread")
	}
	if err := s.d.Threads.LockForMessageChange(ctx, threadID); err != nil {
		return err
	}
	visible := []ids.ID{m.ChannelID}
	row, err := s.lockVisible(ctx, messageID, visible)
	if err != nil {
		return err
	}
	if !row.DeletedAt.IsZero() {
		return importFieldError(importPinPointer, domain.FieldNotInThread, "the pinned message is deleted")
	}
	if _, pinned, err := s.d.Store.Pin(ctx, row.ID, visible); err != nil || pinned {
		return err
	}
	n, err := s.d.Store.CountPins(ctx, row.ThreadID, visible)
	if err != nil {
		return err
	}
	if n >= s.cfg.MaxPinsPerThread {
		return importFieldError(importPinPointer, domain.FieldLimitReached, "the thread already holds pin.max_per_thread pins")
	}
	_, err = s.d.Store.InsertPin(ctx, PinRow{MessageID: row.ID, ThreadID: row.ThreadID, PinnedBy: pinnedBy, PinnedAt: s.now()}, visible)
	return err
}
