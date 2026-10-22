// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/status/app"
)

// fakeScratch is an app.Scratch over a fakeScratchDB. err fails a method by name, panics makes
// a method of the copy panic, step advances the clock on every call so a check has a duration.
type fakeScratch struct {
	mu      sync.Mutex
	calls   []string
	labels  []string
	fetched [][]string
	err     map[string]error
	db      *fakeScratchDB
	objects map[string][sha256.Size]byte
	clk     *clock.Fake
	step    time.Duration
	// restoreTook is added to the clock by Restore, the slow step of a large backup.
	restoreTook time.Duration
	// onStart runs when the copy starts, for a stop of the process during a test.
	onStart func()
}

func newFakeScratch(clk *clock.Fake) *fakeScratch {
	return &fakeScratch{err: map[string]error{}, db: newFakeScratchDB(), objects: map[string][sha256.Size]byte{}, clk: clk, step: time.Second}
}

// hit records method, advances the clock and returns its configured error.
func (s *fakeScratch) hit(method string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, method)
	s.clk.Advance(s.step)
	return s.err[method]
}

func (s *fakeScratch) callList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

func (s *fakeScratch) Restore(_ context.Context, label string) error {
	s.mu.Lock()
	s.labels = append(s.labels, label)
	s.clk.Advance(s.restoreTook)
	s.mu.Unlock()
	return s.hit("Restore")
}

func (s *fakeScratch) Verify(context.Context) error { return s.hit("Verify") }

func (s *fakeScratch) Start(context.Context) (app.ScratchDB, error) {
	if s.onStart != nil {
		s.onStart()
	}
	if err := s.hit("Start"); err != nil {
		return nil, err
	}
	return s.db, nil
}

func (s *fakeScratch) FetchObjects(_ context.Context, keys []string) (map[string][sha256.Size]byte, error) {
	if err := s.hit("FetchObjects"); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fetched = append(s.fetched, slices.Clone(keys))
	out := map[string][sha256.Size]byte{}
	for _, k := range keys {
		if h, ok := s.objects[k]; ok {
			out[k] = h
		}
	}
	return out, nil
}

func (s *fakeScratch) Clear(context.Context) error { return s.hit("Clear") }

// fakeScratchDB is an app.ScratchDB with fixed facts; panics names a method that panics.
type fakeScratchDB struct {
	mu      sync.Mutex
	facts   app.ManifestFacts
	events  app.EventLogFacts
	chain   audit.RowsInput
	threads []app.ThreadSeqs
	files   []app.SampleFile
	err     map[string]error
	panics  string
	samples map[string][]int
}

func newFakeScratchDB() *fakeScratchDB {
	return &fakeScratchDB{
		facts: newFakeSource().facts,
		events: app.EventLogFacts{
			Partitions: []app.EventPartition{{Name: "event_log_p0", Count: 99, MinSeq: 1, MaxSeq: 99}, {Name: "event_log_p1", Count: 21, MinSeq: 100, MaxSeq: 120}},
			Counters:   map[string]int64{"event_seq": 120, "audit_seq": 42, "permission_version": 7},
			AuditHead:  42,
		},
		err:     map[string]error{},
		samples: map[string][]int{},
	}
}

// call records a sample size, panics when asked and returns the configured error.
func (d *fakeScratchDB) call(method string, n int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n >= 0 {
		d.samples[method] = append(d.samples[method], n)
	}
	if d.panics == method {
		panic("fake: " + method)
	}
	return d.err[method]
}

func (d *fakeScratchDB) Facts(context.Context) (app.ManifestFacts, error) {
	return d.facts, d.call("Facts", -1)
}

func (d *fakeScratchDB) EventLog(context.Context) (app.EventLogFacts, error) {
	return d.events, d.call("EventLog", -1)
}

func (d *fakeScratchDB) AuditChain(context.Context) (audit.RowsInput, error) {
	return d.chain, d.call("AuditChain", -1)
}

func (d *fakeScratchDB) ThreadSample(_ context.Context, n int) ([]app.ThreadSeqs, error) {
	if err := d.call("ThreadSample", n); err != nil {
		return nil, err
	}
	return d.threads[:min(n, len(d.threads))], nil
}

func (d *fakeScratchDB) FileSample(_ context.Context, n int) ([]app.SampleFile, error) {
	if err := d.call("FileSample", n); err != nil {
		return nil, err
	}
	return d.files[:min(n, len(d.files))], nil
}

// setThreads fills the copy with n threads of 3 messages without gaps.
func (d *fakeScratchDB) setThreads(n int) {
	d.threads = nil
	for i := range n {
		d.threads = append(d.threads, app.ThreadSeqs{ThreadID: fmt.Sprintf("thread-%d", i), Messages: 3, MinSeq: 1, MaxSeq: 3, LastMessageSeq: 3})
	}
}

// setFiles fills the copy with n files and the object backup s with their objects.
func (s *fakeScratch) setFiles(n int) {
	s.db.files = nil
	for i := range n {
		key := fmt.Sprintf("f/%02x/orig-%d", i, i)
		sum := sha256.Sum256([]byte(key))
		s.db.files = append(s.db.files, app.SampleFile{ID: fmt.Sprintf("file-%d", i), ObjectKey: key, SHA256: sum[:]})
		s.objects[key] = sum
	}
}

// fakeVerifier is an audit.RowsVerifier with a fixed result.
type fakeVerifier struct {
	mu    sync.Mutex
	res   audit.VerifyResult
	err   error
	calls int
}

func (v *fakeVerifier) VerifyRows(context.Context, audit.RowsInput) (audit.VerifyResult, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls++
	return v.res, v.err
}
