// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files"
	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/eventlog"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// purgeQueue records files.purge_objects inserts; Enqueue needs a unit of work.
type purgeQueue struct {
	mu  sync.Mutex
	ids []ids.ID
}

func (q *purgeQueue) Enqueue(ctx context.Context, fileID ids.ID) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.ids = append(q.ids, fileID)
	return nil
}

// list returns the queued file ids.
func (q *purgeQueue) list() []ids.ID {
	q.mu.Lock()
	defer q.mu.Unlock()
	return slices.Clone(q.ids)
}

// redactor records the redacted resources; Redact needs a unit of work.
type redactor struct {
	mu   sync.Mutex
	done []ids.ID
}

func (r *redactor) Redact(ctx context.Context, kind string, id ids.ID, _ eventlog.RedactFunc) (int, error) {
	if err := needUnit(ctx); err != nil {
		return 0, err
	}
	if kind != "file" {
		return 0, errors.New("fake redactor: kind is not file")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done = append(r.done, id)
	return 1, nil
}

func (r *redactor) list() []ids.ID {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.done)
}

// drafts is the fake of files.DraftFileSource: held maps a file to the principal whose draft
// holds it.
type drafts struct {
	held map[ids.ID]ids.ID
	// fail makes the question for this file fail.
	fail ids.ID
}

func (d *drafts) HeldByUploader(ctx context.Context, fs []files.UploaderFile) (map[ids.ID]bool, error) {
	if err := needUnit(ctx); err != nil {
		return nil, err
	}
	out := map[ids.ID]bool{}
	for _, f := range fs {
		if f.FileID == d.fail {
			return nil, errors.New("fake drafts: failing file")
		}
		if p, ok := d.held[f.FileID]; ok && p == f.UploaderID {
			out[f.FileID] = true
		}
	}
	return out, nil
}

// sweeper is the fake of app.TempSweeper.
type sweeper struct {
	mu    sync.Mutex
	calls []time.Time
	n     int
}

func (s *sweeper) SweepTemp(_ context.Context, olderThan time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, olderThan)
	return s.n, nil
}

// backlogMetric records bothy.file.purge.backlog.
type backlogMetric struct {
	mu   sync.Mutex
	vals []int64
}

func (m *backlogMetric) PurgeBacklog(_ context.Context, n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vals = append(m.vals, n)
}

// AbortOpenSessions implements app.Repository for the fake.
func (r *repo) AbortOpenSessions(ctx context.Context, fileIDs []ids.ID) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range fileIDs {
		if s, ok := r.sessions[id]; ok && s.State == domain.SessionOpen {
			s.State = domain.SessionAborted
			r.sessions[id] = s
		}
	}
	return nil
}

// candidates filters the files by keep, orders them by (at, id) and pages after the key.
func (r *repo) candidates(keep func(app.FileRecord) (time.Time, bool), after app.Candidate, limit int) []app.Candidate {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.Candidate
	for id, f := range r.files {
		if at, ok := keep(f); ok {
			out = append(out, app.Candidate{FileID: id, At: at})
		}
	}
	slices.SortFunc(out, func(a, b app.Candidate) int {
		if c := a.At.Compare(b.At); c != 0 {
			return c
		}
		return slices.Compare(a.FileID[:], b.FileID[:])
	})
	var page []app.Candidate
	for _, c := range out {
		cmp := c.At.Compare(after.At)
		if cmp < 0 || (cmp == 0 && slices.Compare(c.FileID[:], after.FileID[:]) <= 0) {
			continue
		}
		page = append(page, c)
		if len(page) == limit {
			break
		}
	}
	return page
}

func (r *repo) attachedLocked(id ids.ID) bool {
	_, ok := r.attachments[id]
	return ok
}

// ExpiredOpenSessions implements app.CleanupStore for the fake.
func (r *repo) ExpiredOpenSessions(_ context.Context, now time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	return r.candidates(func(f app.FileRecord) (time.Time, bool) {
		s, ok := r.sessions[f.ID]
		return s.ExpiresAt, ok && !f.Deleted && s.State == domain.SessionOpen && !s.ExpiresAt.After(now)
	}, after, limit), nil
}

// UnattachedCandidates implements app.CleanupStore for the fake.
func (r *repo) UnattachedCandidates(_ context.Context, cutoff time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	return r.candidates(func(f app.FileRecord) (time.Time, bool) {
		return f.CreatedAt, f.Purpose == "attachment" && !f.Deleted && domain.IsDeliverable(f.UploadState, f.ScanStatus, false) &&
			!r.attachedLocked(f.ID) && !f.CreatedAt.After(cutoff)
	}, after, limit), nil
}

// FailedRowCandidates implements app.CleanupStore for the fake.
func (r *repo) FailedRowCandidates(_ context.Context, cutoff time.Time, after app.Candidate, limit int) ([]app.Candidate, error) {
	return r.candidates(func(f app.FileRecord) (time.Time, bool) {
		blocked := f.UploadState == "failed" || f.ScanStatus == "infected" || f.ScanStatus == "error"
		return f.CreatedAt, f.Purpose == "attachment" && !f.Deleted && blocked && !r.attachedLocked(f.ID) && !f.CreatedAt.After(cutoff)
	}, after, limit), nil
}

// DeletedRows implements app.CleanupStore for the fake.
func (r *repo) DeletedRows(_ context.Context, after app.Candidate, limit int) ([]app.Candidate, error) {
	return r.candidates(func(f app.FileRecord) (time.Time, bool) { return r.deletedAt[f.ID], f.Deleted }, after, limit), nil
}

// PurgeBacklog implements app.CleanupStore for the fake.
func (r *repo) PurgeBacklog(context.Context) (int64, time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	var oldest time.Time
	for id, f := range r.files {
		if f.Deleted {
			n++
			if at := r.deletedAt[id]; oldest.IsZero() || at.Before(oldest) {
				oldest = at
			}
		}
	}
	return n, oldest, nil
}

// PurgeRecord implements app.CleanupStore for the fake.
func (r *repo) PurgeRecord(_ context.Context, fileID ids.ID) (app.PurgeRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok {
		return app.PurgeRecord{}, false, nil
	}
	rec := app.PurgeRecord{Facts: domain.PurgeFacts{Exists: true, Deleted: f.Deleted, UploadState: f.UploadState, ScanStatus: f.ScanStatus},
		ObjectKey: f.ObjectKey, UploadedBy: f.UploadedBy}
	if s, ok := r.sessions[fileID]; ok {
		rec.HasSession, rec.PartCount, rec.StorageUploadID = true, s.PartCount, s.StorageUploadID
	}
	return rec, true, nil
}

// LockFile implements app.CleanupStore for the fake.
func (r *repo) LockFile(ctx context.Context, fileID ids.ID) (bool, error) {
	if err := needUnit(ctx); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.files[fileID]
	return ok, nil
}

// LockFileForDelete implements app.CleanupStore for the fake.
func (r *repo) LockFileForDelete(ctx context.Context, fileID ids.ID) (bool, error) {
	return r.LockFile(ctx, fileID)
}

// CleanupRecord implements app.CleanupStore for the fake.
func (r *repo) CleanupRecord(_ context.Context, fileID ids.ID) (app.CleanupRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok {
		return app.CleanupRecord{}, false, nil
	}
	rec := app.CleanupRecord{UploadedBy: f.UploadedBy, Purpose: f.Purpose, UploadState: f.UploadState, ScanStatus: f.ScanStatus,
		Deleted: f.Deleted, Attached: r.attachedLocked(fileID), Ages: domain.CleanupFacts{CreatedAt: f.CreatedAt}}
	if s, ok := r.sessions[fileID]; ok {
		rec.Ages.SessionState, rec.Ages.CompletedAt, rec.Ages.ExpiresAt = s.State, s.CompletedAt, s.ExpiresAt
	}
	return rec, true, nil
}

// FileForDelete implements app.CleanupStore for the fake.
func (r *repo) FileForDelete(_ context.Context, fileID ids.ID) (app.FileRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	return f, ok, nil
}

// ExpireSession implements app.CleanupStore for the fake.
func (r *repo) ExpireSession(ctx context.Context, fileID ids.ID) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[fileID]
	if !ok || s.State != domain.SessionOpen {
		return errors.New("fake repo: expiring a session that is not open")
	}
	s.State = domain.SessionExpired
	r.sessions[fileID] = s
	f := r.files[fileID]
	f.UploadState = "failed"
	r.files[fileID] = f
	delete(r.parts, fileID)
	return nil
}

// DeleteFileRow implements app.CleanupStore for the fake.
func (r *repo) DeleteFileRow(ctx context.Context, fileID ids.ID) (bool, error) {
	if err := needUnit(ctx); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.files[fileID]
	if !ok || !f.Deleted {
		return false, nil
	}
	delete(r.files, fileID)
	delete(r.sessions, fileID)
	delete(r.parts, fileID)
	delete(r.deletedAt, fileID)
	return true, nil
}

// setPurged removes the row of fileID with its session and parts, as files.purge_objects does.
func (r *repo) setPurged(fileID ids.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.files, fileID)
	delete(r.sessions, fileID)
	delete(r.parts, fileID)
	delete(r.thumbs, fileID)
}
