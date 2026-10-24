// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"slices"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// compareIDs orders ids as PostgreSQL orders uuid values.
func compareIDs(a, b ids.ID) int { return slices.Compare(a[:], b[:]) }

// LockAttachTargets implements app.Repository for the fake: the known files of fileIDs in
// ascending id with the state of their session; it records the call.
func (r *repo) LockAttachTargets(ctx context.Context, fileIDs []ids.ID) ([]app.AttachTarget, error) {
	if err := needUnit(ctx); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "LockAttachTargets")
	sorted := slices.Clone(fileIDs)
	slices.SortFunc(sorted, compareIDs)
	var out []app.AttachTarget
	for _, id := range sorted {
		f, ok := r.files[id]
		if !ok {
			continue
		}
		f.Thumbnails = append([]app.ThumbnailRecord(nil), r.thumbs[id]...)
		out = append(out, app.AttachTarget{File: f, SessionState: r.sessions[id].State})
	}
	return out, nil
}

// AttachedFileIDs implements app.Repository for the fake.
func (r *repo) AttachedFileIDs(_ context.Context, fileIDs []ids.ID) ([]ids.ID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "AttachedFileIDs")
	var out []ids.ID
	for _, id := range fileIDs {
		if _, ok := r.attachments[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}

// InsertAttachments implements app.Repository for the fake; insertErr replaces the insert.
func (r *repo) InsertAttachments(ctx context.Context, rows []app.AttachmentRecord) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "InsertAttachments")
	if r.insertErr != nil {
		return r.insertErr
	}
	for _, row := range rows {
		if _, ok := r.attachments[row.FileID]; ok {
			return app.ErrAlreadyAttached
		}
	}
	for _, row := range rows {
		r.attachments[row.FileID] = row
	}
	return nil
}

// MessageAttachments implements app.Repository for the fake: by message, then position.
func (r *repo) MessageAttachments(_ context.Context, messageIDs []ids.ID) ([]app.AttachedFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "MessageAttachments")
	var out []app.AttachedFile
	for _, row := range r.attachments {
		if slices.Contains(messageIDs, row.MessageID) {
			f := r.files[row.FileID]
			f.Thumbnails = append([]app.ThumbnailRecord(nil), r.thumbs[row.FileID]...)
			out = append(out, app.AttachedFile{Row: row, File: f})
		}
	}
	slices.SortFunc(out, func(a, b app.AttachedFile) int {
		if c := compareIDs(a.Row.MessageID, b.Row.MessageID); c != 0 {
			return c
		}
		return a.Row.Position - b.Row.Position
	})
	return out, nil
}

// AttachmentOf implements app.Repository for the fake.
func (r *repo) AttachmentOf(_ context.Context, fileID ids.ID) (app.AttachmentRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.attachments[fileID]
	return row, ok, nil
}

// LockMessageFiles implements app.Repository for the fake: ascending id.
func (r *repo) LockMessageFiles(ctx context.Context, messageID ids.ID) ([]ids.ID, error) {
	if err := needUnit(ctx); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "LockMessageFiles")
	var out []ids.ID
	for _, row := range r.attachments {
		if row.MessageID == messageID {
			out = append(out, row.FileID)
		}
	}
	slices.SortFunc(out, compareIDs)
	return out, nil
}

// DeleteMessageAttachments implements app.Repository for the fake.
func (r *repo) DeleteMessageAttachments(ctx context.Context, messageID ids.ID) (int, error) {
	if err := needUnit(ctx); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "DeleteMessageAttachments")
	n := 0
	for id, row := range r.attachments {
		if row.MessageID == messageID {
			delete(r.attachments, id)
			n++
		}
	}
	return n, nil
}

// MarkFilesDeleted implements app.Repository for the fake.
func (r *repo) MarkFilesDeleted(ctx context.Context, d app.Deletion, at time.Time) ([]ids.ID, error) {
	if err := needUnit(ctx); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "MarkFilesDeleted")
	var marked []ids.ID
	for _, id := range d.FileIDs {
		f, ok := r.files[id]
		if !ok || f.Deleted || (d.Purpose != "" && f.Purpose != d.Purpose) || (!d.UploadedBy.IsZero() && f.UploadedBy != d.UploadedBy) {
			continue
		}
		f.Deleted = true
		r.files[id] = f
		r.deletedAt[id] = at
		marked = append(marked, id)
	}
	return marked, nil
}

// DeleteTextExtractsOfDeleted implements app.Repository for the fake: it records the call and
// the ids of deleted files.
func (r *repo) DeleteTextExtractsOfDeleted(ctx context.Context, fileIDs []ids.ID) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "DeleteTextExtractsOfDeleted")
	for _, id := range fileIDs {
		if r.files[id].Deleted {
			r.textsDeleted = append(r.textsDeleted, id)
		}
	}
	return nil
}

// AttachmentStates implements app.Repository for the fake.
func (r *repo) AttachmentStates(ctx context.Context, messageID ids.ID) ([]domain.StateFacts, error) {
	if err := needUnit(ctx); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "AttachmentStates")
	var out []domain.StateFacts
	for _, row := range r.attachments {
		if row.MessageID == messageID {
			f := r.files[row.FileID]
			out = append(out, domain.StateFacts{UploadState: f.UploadState, ScanStatus: f.ScanStatus, Deleted: f.Deleted,
				TranscriptStatus: f.TranscriptStatus})
		}
	}
	return out, nil
}

// callList returns the recorded calls of the attachment methods.
func (r *repo) callList() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

// threadRows is the fake of app.ThreadAttachments.
type threadRows struct {
	moves [][2]ids.ID
}

func (t *threadRows) MoveThreadAttachments(ctx context.Context, threadID, channelID ids.ID) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	t.moves = append(t.moves, [2]ids.ID{threadID, channelID})
	return nil
}
