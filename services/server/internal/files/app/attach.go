// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// Compile-time check: the Service is the attachment port of messaging and runs (M3-07).
var _ files.Attachments = (*Service)(nil)

// reasonFileNotAttachable is the reason of the one 409 conflict of Attach (12 10 point 3,
// docs/decisions.d/M3-07.md D-M3-07).
const reasonFileNotAttachable = "file_not_attachable"

// notAttachable is the answer for every refused file: the same status, code, reason and headers
// for an unknown, foreign, deleted, other-channel, open, aborted, expired, failed or attached file,
// so the answer tells nothing about files of other people (SEC-UP-9); only detail names the entry.
func notAttachable(pointer string) error {
	return conflict(reasonFileNotAttachable, pointer)
}

// needUnit is the internal error of a port method that only runs in the caller's unit of work.
func needUnit(ctx context.Context, method string) error {
	if txn.InUnit(ctx) {
		return nil
	}
	return errs.New(errs.CodeInternalError, "files: Attachments."+method+" needs a unit of work")
}

// Attach implements files.Attachments (brief M3-07 design note 4): the files rows of the distinct
// ids FOR UPDATE in ascending id with the state of their sessions, then in a later statement the
// rows of message_attachments of these files, then the walk in request order, where the first
// file domain.Attachable refuses ends the call with notAttachable("<Pointer>/<index>") and an id
// listed a second time counts as attached; then one insert of the rows. A concurrent attach that
// committed first surfaces as a unique violation and gives notAttachable(Pointer). An empty list
// costs no statement. It never calls authorize: message.create with +upload was decided by the
// caller.
func (s *Service) Attach(ctx context.Context, cmd files.AttachCmd) ([]files.Attachment, error) {
	if err := needUnit(ctx, "Attach"); err != nil {
		return nil, err
	}
	if len(cmd.FileIDs) == 0 {
		return []files.Attachment{}, nil
	}
	distinct := slices.Clone(cmd.FileIDs)
	slices.SortFunc(distinct, func(a, b ids.ID) int { return slices.Compare(a[:], b[:]) })
	distinct = slices.Compact(distinct)
	targets, err := s.d.Repo.LockAttachTargets(ctx, distinct)
	if err != nil {
		return nil, err
	}
	attachedIDs, err := s.d.Repo.AttachedFileIDs(ctx, distinct)
	if err != nil {
		return nil, err
	}
	byID := make(map[ids.ID]AttachTarget, len(targets))
	for _, t := range targets {
		byID[t.File.ID] = t
	}
	attached := make(map[ids.ID]bool, len(attachedIDs)+len(cmd.FileIDs))
	for _, id := range attachedIDs {
		attached[id] = true
	}
	rows := make([]AttachmentRecord, 0, len(cmd.FileIDs))
	out := make([]files.Attachment, 0, len(cmd.FileIDs))
	for i, id := range cmd.FileIDs {
		t, found := byID[id]
		facts := domain.AttachFacts{Found: found, Deleted: t.File.Deleted, UploadedBy: t.File.UploadedBy, AuthorID: cmd.AuthorID,
			Purpose: t.File.Purpose, ScopeChannelID: t.File.ScopeChannelID, ChannelID: cmd.ChannelID, SessionState: t.SessionState,
			UploadState: t.File.UploadState, ScanStatus: t.File.ScanStatus, Attached: attached[id]}
		if !domain.Attachable(facts) {
			return nil, notAttachable(cmd.Pointer + "/" + strconv.Itoa(i))
		}
		attached[id] = true
		rows = append(rows, AttachmentRecord{MessageID: cmd.MessageID, FileID: id, ThreadID: cmd.ThreadID, ChannelID: cmd.ChannelID, Position: i + 1})
		out = append(out, files.Attachment{Position: i + 1, File: attachedView(t.File)})
	}
	return s.insertAttachments(ctx, cmd, rows, out, byID)
}

// insertAttachments ends Attach: the voice check of a voice message on its one locked file
// before the insert (M3-09 design note 6 b), the rows, then the start of the server transcript
// in mode service (design note 6 c), which needs the attachment row for the scope of
// file.updated.
func (s *Service) insertAttachments(ctx context.Context, cmd files.AttachCmd, rows []AttachmentRecord, out []files.Attachment,
	byID map[ids.ID]AttachTarget) ([]files.Attachment, error) {
	if cmd.Voice {
		// messaging allows exactly one attachment for a voice message (M3-09 design note 7).
		if len(cmd.FileIDs) != 1 {
			return nil, errs.New(errs.CodeInternalError, "files: a voice message needs exactly one attachment")
		}
		if err := s.checkVoice(byID[cmd.FileIDs[0]].File, cmd.Pointer, cmd.AuthorKind); err != nil {
			return nil, err
		}
	}
	if err := s.d.Repo.InsertAttachments(ctx, rows); err != nil {
		if errors.Is(err, ErrAlreadyAttached) {
			return nil, notAttachable(cmd.Pointer)
		}
		return nil, err
	}
	if !cmd.Voice {
		return out, nil
	}
	started, err := s.startServiceTranscript(ctx, cmd.AuthorID, cmd.AuthorKind, byID[cmd.FileIDs[0]].File)
	if err != nil {
		return nil, err
	}
	if started {
		out[0].File.TranscriptStatus, out[0].File.TranscriptLanguage, out[0].File.TranscriptSource = transcriptPending, "", ""
	}
	return out, nil
}

// attachedView is the attached form of a file (brief M3-07 design note 6, docs/decisions.d/M3-07.md
// D-M3-07-5): FileView without scope_channel_id, because after a move it names a channel the reader
// may not see; the channel of the message counts (05 2.2).
func attachedView(f FileRecord) files.File {
	v := FileView(f)
	v.ScopeChannelID = ids.ID{}
	return v
}

// ForMessages implements files.Attachments (design note 6): one read of the rows of every message
// with their files and thumbnails, grouped by message in position order. No message costs no
// statement. It reads in the ambient transaction when there is one and on the pool otherwise,
// because the history reads of messaging run outside a unit of work.
func (s *Service) ForMessages(ctx context.Context, messageIDs []ids.ID) (map[ids.ID][]files.Attachment, error) {
	out := map[ids.ID][]files.Attachment{}
	if len(messageIDs) == 0 {
		return out, nil
	}
	rows, err := s.d.Repo.MessageAttachments(ctx, messageIDs)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		a := files.Attachment{Position: r.Row.Position, File: attachedView(r.File)}
		if r.File.TranscriptStatus == transcriptDone {
			a.Transcript = r.File.TranscriptText
		}
		out[r.Row.MessageID] = append(out[r.Row.MessageID], a)
	}
	return out, nil
}

// DeleteForMessage implements files.Attachments (design note 7 b and c): the files of the
// attachments FOR UPDATE in ascending id, the rows of message_attachments, then markDeleted at the
// clock time. It writes no file.updated: message.deleted removes the message with its attachments
// from every client, and the objects go with files.purge_objects (M3-08). A message without
// attachments writes nothing.
func (s *Service) DeleteForMessage(ctx context.Context, messageID ids.ID) error {
	if err := needUnit(ctx, "DeleteForMessage"); err != nil {
		return err
	}
	fileIDs, err := s.d.Repo.LockMessageFiles(ctx, messageID)
	if err != nil || len(fileIDs) == 0 {
		return err
	}
	if _, err := s.d.Repo.DeleteMessageAttachments(ctx, messageID); err != nil {
		return err
	}
	return s.markDeleted(ctx, Deletion{FileIDs: fileIDs})
}

// MessageOf implements files.Attachments: the attachment row of fileID by its unique key.
func (s *Service) MessageOf(ctx context.Context, fileID ids.ID) (files.AttachmentRef, bool, error) {
	row, found, err := s.d.Repo.AttachmentOf(ctx, fileID)
	if err != nil || !found {
		return files.AttachmentRef{}, false, err
	}
	return files.AttachmentRef{MessageID: row.MessageID, ThreadID: row.ThreadID, ChannelID: row.ChannelID, Position: row.Position}, true, nil
}

// Pending implements files.Attachments (design note 11 a; M3-09 design note 3): PendingFiles while
// one attached file of the message is not final (domain.Final, 12 2), else PendingTranscript while
// one has transcript_status pending, else PendingNone. It reads in the caller's unit of work, which
// holds the file rows when it attached them, so the answer is current.
func (s *Service) Pending(ctx context.Context, messageID ids.ID) (files.PendingReason, error) {
	if err := needUnit(ctx, "Pending"); err != nil {
		return files.PendingNone, err
	}
	states, err := s.d.Repo.AttachmentStates(ctx, messageID)
	if err != nil {
		return files.PendingNone, err
	}
	reason := files.PendingNone
	for _, st := range states {
		if !domain.Final(st.UploadState, st.ScanStatus, st.Deleted) {
			return files.PendingFiles, nil
		}
		if st.TranscriptStatus == domain.TranscriptPending {
			reason = files.PendingTranscript
		}
	}
	return reason, nil
}
