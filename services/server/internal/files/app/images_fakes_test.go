// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// imageFile reads a file with its thumbnails, deleted or not.
func (r *repo) imageFile(fileID ids.ID) (app.FileRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok {
		return app.FileRecord{}, false
	}
	f.Thumbnails = append([]app.ThumbnailRecord(nil), r.thumbs[fileID]...)
	return f, true
}

func (r *repo) ImageFile(_ context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	f, ok := r.imageFile(fileID)
	return f, ok, nil
}

func (r *repo) LockImageFile(ctx context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	if err := needUnit(ctx); err != nil {
		return app.FileRecord{}, false, err
	}
	f, ok := r.imageFile(fileID)
	return f, ok, nil
}

func (r *repo) StoreImageOriginal(ctx context.Context, fileID ids.ID, o app.StoredOriginal) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f := r.files[fileID]
	f.ObjectKey, f.SHA256, f.SizeBytes, f.MIMEType, f.Width, f.Height = o.ObjectKey, o.SHA256, o.SizeBytes, o.MIMEType, o.Width, o.Height
	f.UploadState = "complete"
	r.files[fileID] = f
	r.writes = append(r.writes, "T1")
	return nil
}

func (r *repo) FailImage(ctx context.Context, fileID ids.ID, reason string) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f := r.files[fileID]
	f.UploadState = "failed"
	r.files[fileID] = f
	r.failure[fileID] = reason
	return nil
}

func (r *repo) InsertThumbnails(ctx context.Context, fileID ids.ID, rows []app.ThumbnailRecord) (int, error) {
	if err := needUnit(ctx); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, row := range rows {
		dup := false
		for _, h := range r.thumbs[fileID] {
			dup = dup || h.Variant == row.Variant
		}
		if !dup {
			r.thumbs[fileID] = append(r.thumbs[fileID], row)
			n++
		}
	}
	r.writes = append(r.writes, "T2")
	return n, nil
}

func (r *repo) SetMediaInfo(ctx context.Context, fileID ids.ID, m app.MediaInfo) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f := r.files[fileID]
	if m.Width != 0 {
		f.Width, f.Height = m.Width, m.Height
	}
	if m.DurationMS != 0 {
		f.DurationMS = m.DurationMS
	}
	r.files[fileID] = f
	return nil
}
