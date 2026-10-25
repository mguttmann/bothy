// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// CompleteContainerFile implements app.Repository for the fake (M3-09 design note 5).
func (r *repo) CompleteContainerFile(ctx context.Context, fileID ids.ID, m app.MediaInfo) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f := r.files[fileID]
	if f.UploadState != "pending" || f.Deleted {
		return nil
	}
	if m.Width != 0 {
		f.Width, f.Height = m.Width, m.Height
	}
	if m.DurationMS != 0 {
		f.DurationMS = m.DurationMS
	}
	f.UploadState = "complete"
	r.files[fileID] = f
	return nil
}

// TranscriptFile implements app.Repository for the fake: deleted or not.
func (r *repo) TranscriptFile(_ context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	return f, ok, nil
}

// LockTranscriptFile implements app.Repository for the fake.
func (r *repo) LockTranscriptFile(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	if err := needUnit(ctx); err != nil {
		return app.FileRecord{}, false, err
	}
	return r.TranscriptFile(ctx, fileID)
}

// LockVoiceFile implements app.Repository for the fake.
func (r *repo) LockVoiceFile(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	return r.LockTranscriptFile(ctx, fileID)
}

// SetTranscript implements app.Repository for the fake.
func (r *repo) SetTranscript(ctx context.Context, fileID ids.ID, t app.TranscriptWrite) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok || f.Deleted {
		return nil
	}
	f.TranscriptStatus, f.TranscriptText, f.TranscriptLanguage, f.TranscriptSource = t.Status, t.Text, t.Language, t.Source
	r.files[fileID] = f
	return nil
}
