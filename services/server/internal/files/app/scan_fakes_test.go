// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/files/app"
	"github.com/OWNER/bothy/services/server/internal/files/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/auditing"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
)

// scanTarget reads a session with its file, deleted or not.
func (r *repo) scanTarget(fileID ids.ID) (app.SessionRecord, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[fileID]
	if !ok {
		return app.SessionRecord{}, false
	}
	s.File = r.files[fileID]
	return s, true
}

func (r *repo) ScanTarget(_ context.Context, fileID ids.ID) (app.SessionRecord, bool, error) {
	s, ok := r.scanTarget(fileID)
	return s, ok, nil
}

func (r *repo) LockScanTarget(ctx context.Context, fileID ids.ID) (app.SessionRecord, bool, error) {
	if err := needUnit(ctx); err != nil {
		return app.SessionRecord{}, false, err
	}
	s, ok := r.scanTarget(fileID)
	return s, ok, nil
}

func (r *repo) FinishScan(ctx context.Context, fileID ids.ID, res app.ScanResult) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f := r.files[fileID]
	f.UploadState, f.ScanStatus, f.MIMEType, f.SHA256, f.SizeBytes = res.UploadState, res.ScanStatus, res.MIMEType, res.SHA256, res.SizeBytes
	r.files[fileID] = f
	r.finished = append(r.finished, res)
	s := r.sessions[fileID]
	s.File = f
	r.sessions[fileID] = s
	r.failure[fileID] = res.FailureReason
	return nil
}

func (r *repo) CountAwaitingScan(context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for id, s := range r.sessions {
		f := r.files[id]
		if s.State == domain.SessionCompleted && !f.Deleted && f.UploadState == "pending" && f.ScanStatus == "pending" {
			n++
		}
	}
	return n, nil
}

// setDeleted marks a file deleted (the deletion of M3-05/M3-08).
func (r *repo) setDeleted(fileID ids.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f := r.files[fileID]
	f.Deleted = true
	r.files[fileID] = f
}

// scanQueue records the inserted files.scan jobs; it needs a unit of work.
type scanQueue struct {
	mu   sync.Mutex
	jobs []scanJob
}

type scanJob struct {
	fileID ids.ID
	round  int64
	at     time.Time
}

func (q *scanQueue) EnqueueScan(ctx context.Context, fileID ids.ID, round int64, at time.Time) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, scanJob{fileID: fileID, round: round, at: at})
	return nil
}

func (q *scanQueue) list() []scanJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]scanJob(nil), q.jobs...)
}

// scanner is a scripted Scanner: it reads up to readLimit bytes (0: all), then returns verdict and
// err; pingErr fails Ping. during runs while the scan reads.
type scanner struct {
	mu        sync.Mutex
	verdict   domain.Verdict
	err       error
	pingErr   error
	readLimit int
	scans     int
	pings     int
	seen      []byte
	during    func()
}

func (s *scanner) Scan(_ context.Context, r io.Reader) (domain.Verdict, error) {
	s.mu.Lock()
	s.scans++
	limit, v, err, during := s.readLimit, s.verdict, s.err, s.during
	s.mu.Unlock()
	var b []byte
	if limit > 0 {
		b, _ = io.ReadAll(io.LimitReader(r, int64(limit)))
	} else {
		b, _ = io.ReadAll(r)
	}
	if during != nil {
		during()
	}
	s.mu.Lock()
	s.seen = b
	s.mu.Unlock()
	return v, err
}

func (s *scanner) Ping(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pings++
	return s.pingErr
}

func (s *scanner) counts() (scans, pings int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scans, s.pings
}

// detector reports image/png for the PNG signature, else application/octet-stream.
type detector struct{}

var pngMagic = []byte("\x89PNG\r\n\x1a\n")

func (detector) Detect(head []byte) string {
	if bytes.HasPrefix(head, pngMagic) {
		return "image/png"
	}
	return "application/octet-stream"
}

// audit keeps the recorded entries; Record needs a unit of work.
type audit struct {
	mu      sync.Mutex
	entries []auditing.Entry
}

func (a *audit) Record(ctx context.Context, e auditing.Entry) error {
	if err := needUnit(ctx); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.entries = append(a.entries, e)
	return nil
}

func (a *audit) RecordDenial(context.Context, auditing.Entry) error   { return nil }
func (a *audit) RecordDetached(context.Context, auditing.Entry) error { return nil }

func (a *audit) list() []auditing.Entry {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]auditing.Entry(nil), a.entries...)
}

// scanMetrics records the outcomes and queue values.
type scanMetrics struct {
	mu       sync.Mutex
	outcomes []string
	queue    []int64
}

func (m *scanMetrics) ScanCompleted(_ context.Context, outcome string, _ time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outcomes = append(m.outcomes, outcome)
}

func (m *scanMetrics) ScanQueue(_ context.Context, n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = append(m.queue, n)
}

// countingStore counts the bytes read from every key of the fake store.
type countingStore struct {
	*store
	mu    sync.Mutex
	read  map[string]int64
	gets  map[string]int
	puts  map[string][]byte
	after func(op, key string)
}

func newCountingStore(s *store) *countingStore {
	return &countingStore{store: s, read: map[string]int64{}, gets: map[string]int{}, puts: map[string][]byte{}}
}

type countingReader struct {
	io.ReadCloser
	add func(int)
}

func (c countingReader) Read(b []byte) (int, error) {
	n, err := c.ReadCloser.Read(b)
	c.add(n)
	return n, err
}

func (c *countingStore) Get(ctx context.Context, key string, rng *app.ByteRange) (io.ReadCloser, error) {
	r, err := c.store.Get(ctx, key, rng)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.gets[key]++
	c.mu.Unlock()
	return countingReader{ReadCloser: r, add: func(n int) {
		c.mu.Lock()
		c.read[key] += int64(n)
		c.mu.Unlock()
	}}, nil
}

func (c *countingStore) Put(ctx context.Context, key string, body io.Reader, size int64) error {
	var buf bytes.Buffer
	err := c.store.Put(ctx, key, io.TeeReader(body, &buf), size)
	c.mu.Lock()
	if err == nil {
		c.puts[key] = buf.Bytes()
	}
	c.mu.Unlock()
	return err
}

func (c *countingStore) has(key string) bool {
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	_, ok := c.store.objects[key]
	return ok
}
