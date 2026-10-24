// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"io"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/kernel/txn"
)

// extractRow is a row of file_text_extracts in the fake.
type extractRow struct {
	app.ExtractWrite
	Lang string
}

// extractRepo is an in-memory app.ExtractRepository over files it shares with nobody; writes need
// a unit of work.
type extractRepo struct {
	mu    sync.Mutex
	files map[ids.ID]app.FileRecord
	rows  map[ids.ID]*extractRow
	// tooLarge answers ErrVectorTooLarge to that many done writes; writes counts every write.
	tooLarge int
	writes   []app.ExtractWrite
	// beforeReset runs in ResetTextExtract before the compare, to change a row in between.
	beforeReset func(id ids.ID)
}

func newExtractRepo() *extractRepo {
	return &extractRepo{files: map[ids.ID]app.FileRecord{}, rows: map[ids.ID]*extractRow{}}
}

func (r *extractRepo) InsertTextExtract(ctx context.Context, fileID ids.ID, lang string) (bool, error) {
	if err := needUnit(ctx); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.rows[fileID]; ok {
		return false, nil
	}
	r.rows[fileID] = &extractRow{ExtractWrite: app.ExtractWrite{State: domain.ExtractPending}, Lang: lang}
	return true, nil
}

func (r *extractRepo) ExtractTarget(_ context.Context, fileID ids.ID) (app.ExtractTarget, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[fileID]
	if !ok {
		return app.ExtractTarget{}, false, nil
	}
	return app.ExtractTarget{State: row.State, File: r.files[fileID]}, true, nil
}

func (r *extractRepo) WriteExtract(ctx context.Context, fileID ids.ID, w app.ExtractWrite) (bool, error) {
	if err := needUnit(ctx); err != nil {
		return false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writes = append(r.writes, w)
	if w.State == domain.ExtractDone && r.tooLarge > 0 {
		r.tooLarge--
		return false, app.ErrVectorTooLarge
	}
	row, ok := r.rows[fileID]
	if !ok || row.State != domain.ExtractPending || r.files[fileID].Deleted {
		return false, nil
	}
	row.ExtractWrite = w
	return true, nil
}

func (r *extractRepo) DeleteTextExtract(ctx context.Context, fileID ids.ID) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rows, fileID)
	return nil
}

func (r *extractRepo) RefreshCandidates(ctx context.Context, after ids.ID, limit int) ([]app.RefreshCandidate, error) {
	if err := needUnit(ctx); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var keys []ids.ID
	for id, f := range r.files {
		if bytes.Compare(id[:], after[:]) > 0 && f.Purpose == "attachment" && domain.IsDeliverable(f.UploadState, f.ScanStatus, f.Deleted) {
			keys = append(keys, id)
		}
	}
	slices.SortFunc(keys, func(a, b ids.ID) int { return bytes.Compare(a[:], b[:]) })
	if len(keys) > limit {
		keys = keys[:limit]
	}
	out := []app.RefreshCandidate{}
	for _, id := range keys {
		f := r.files[id]
		c := app.RefreshCandidate{FileID: id, MIMEType: f.MIMEType, SizeBytes: f.SizeBytes}
		if row, ok := r.rows[id]; ok {
			c.HasRow, c.State, c.Extractor, c.Error = true, row.State, row.Version, row.Code
		}
		out = append(out, c)
	}
	return out, nil
}

func (r *extractRepo) ResetTextExtract(ctx context.Context, fileID ids.ID, seenState, seenExtractor string) (bool, error) {
	if err := needUnit(ctx); err != nil {
		return false, err
	}
	if r.beforeReset != nil {
		r.beforeReset(fileID)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[fileID]
	if !ok || row.State != seenState || row.Version != seenExtractor {
		return false, nil
	}
	row.State = domain.ExtractPending
	return true, nil
}

func (r *extractRepo) DeleteOrphanTextExtracts(ctx context.Context, limit int) (int, error) {
	if err := needUnit(ctx); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for id := range r.rows {
		if r.files[id].Deleted && n < limit {
			delete(r.rows, id)
			n++
		}
	}
	return n, nil
}

func (r *extractRepo) row(id ids.ID) (extractRow, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return extractRow{}, false
	}
	return *row, true
}

// extractQueue records the inserted jobs; inserts need a unit of work.
type extractQueue struct {
	mu       sync.Mutex
	extracts []ids.ID
	refresh  []ids.ID
}

func (q *extractQueue) EnqueueExtract(ctx context.Context, fileID ids.ID) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.extracts = append(q.extracts, fileID)
	return nil
}

func (q *extractQueue) EnqueueExtractRefresh(ctx context.Context, after ids.ID) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.refresh = append(q.refresh, after)
	return nil
}

func (q *extractQueue) jobs() ([]ids.ID, []ids.ID) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return slices.Clone(q.extracts), slices.Clone(q.refresh)
}

// fakeExtractor supports the types of versions; Extract reads the whole reader (counting) and
// answers result, err or, with stall, waits for the context.
type fakeExtractor struct {
	versions map[string]string
	result   app.ExtractResult
	err      error
	stall    bool
	// started is closed when a stalling Extract begins; nil closes nothing.
	started chan struct{}
	mu      sync.Mutex
	read    int64
	calls   int
}

func (x *fakeExtractor) Version(mime string) (string, bool) {
	v, ok := x.versions[mime]
	return v, ok
}

func (x *fakeExtractor) Extract(ctx context.Context, r io.Reader, mime string) (app.ExtractResult, error) {
	x.mu.Lock()
	x.calls++
	x.mu.Unlock()
	if x.stall {
		if x.started != nil {
			close(x.started)
		}
		<-ctx.Done()
		return app.ExtractResult{}, ctx.Err()
	}
	n, err := io.Copy(io.Discard, r)
	x.mu.Lock()
	x.read += n
	x.mu.Unlock()
	if err != nil {
		return app.ExtractResult{}, err
	}
	if x.err != nil {
		return app.ExtractResult{}, x.err
	}
	res := x.result
	res.Version = x.versions[mime]
	return res, nil
}

// langs answers the instance language.
type langs string

func (l langs) InstanceSearchLanguage(context.Context) (string, error) { return string(l), nil }

// extractEnv is the extraction over the fakes.
type extractEnv struct {
	ext   *app.Extraction
	repo  *extractRepo
	queue *extractQueue
	x     *fakeExtractor
	store *store
	uow   *uow
}

// extractCfg are the limits of the app tests.
var extractCfg = app.ExtractConfig{MaxInputBytes: 1000, Timeout: 1 << 62, RefreshBatchSize: 2}

func newExtractEnv(t *testing.T, cfg app.ExtractConfig) *extractEnv {
	t.Helper()
	e := &extractEnv{repo: newExtractRepo(), queue: &extractQueue{}, store: newStore(t), uow: newUoWWith(t, noSubscribers),
		x: &fakeExtractor{versions: map[string]string{"text/plain": "builtin-text/1", extractDOCX: "builtin-ooxml/1"}}}
	ext, err := app.NewExtraction(cfg, app.ExtractDeps{Repo: e.repo, Store: e.store, UoW: e.uow, Extractor: e.x, Queue: e.queue,
		Languages: langs("german"), Clock: fakeClock()})
	if err != nil {
		t.Fatal(err)
	}
	e.ext = ext
	return e
}

// noSubscribers registers nothing.
func noSubscribers(*txn.Registry) error { return nil }

// fakeClock is a fake clock at a fixed time.
func fakeClock() *clock.Fake { return clock.NewFake(time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC)) }

// extractDOCX is the media type of a DOCX.
const extractDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// newTestTrigger is a trigger over fresh fakes for the scan and image tests.
func newTestTrigger(t *testing.T) (*app.ExtractTrigger, *extractRepo, *extractQueue) {
	t.Helper()
	e := newExtractEnv(t, extractCfg)
	return e.ext.Trigger(), e.repo, e.queue
}
