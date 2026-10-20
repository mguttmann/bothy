// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/clock"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// gate holds a fake step until the test releases it, so a run stays busy without sleeping.
type gate struct {
	entered chan struct{}
	release chan struct{}
}

func newGate() *gate { return &gate{entered: make(chan struct{}, 8), release: make(chan struct{})} }

// fakeExec is an app.Executor that records its calls. err fails a method by name; gates hold a
// method until released. PgDump writes a small file at the path, as pg_dump would.
type fakeExec struct {
	mu      sync.Mutex
	calls   []string
	backups []app.PgBackRestBackup
	restic  []app.ResticBackup
	forgets []app.ResticForget
	expires []int
	offsite []string
	dumps   []string
	volumes []string
	err     map[string]error
	gates   map[string]*gate
	usage   map[string]app.VolumeUsage
	// clk, when set, is advanced by step on every call, so a run has a duration.
	clk  *clock.Fake
	step time.Duration
}

func newFakeExec() *fakeExec {
	return &fakeExec{err: map[string]error{}, gates: map[string]*gate{}, usage: map[string]app.VolumeUsage{}}
}

// hit records method, advances the clock, waits at its gate and returns its configured error.
func (f *fakeExec) hit(ctx context.Context, method string) error {
	f.mu.Lock()
	f.calls = append(f.calls, method)
	g := f.gates[method]
	err := f.err[method]
	if f.clk != nil {
		f.clk.Advance(f.step)
	}
	f.mu.Unlock()
	if g != nil {
		g.entered <- struct{}{}
		select {
		case <-g.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func (f *fakeExec) callList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *fakeExec) PgBackRestBackup(ctx context.Context, req app.PgBackRestBackup) (app.PgBackRestResult, error) {
	f.mu.Lock()
	f.backups = append(f.backups, req)
	f.mu.Unlock()
	if err := f.hit(ctx, "PgBackRestBackup"); err != nil {
		return app.PgBackRestResult{}, err
	}
	label := "20261018-020000F"
	if req.Type == "diff" {
		label = "20261018-020000F_20261019-020000D"
	}
	return app.PgBackRestResult{Label: label, SizeBytes: int64(1000 * req.Repo)}, nil
}

func (f *fakeExec) PgBackRestExpireLocal(ctx context.Context, keepFull int) error {
	f.mu.Lock()
	f.expires = append(f.expires, keepFull)
	f.mu.Unlock()
	return f.hit(ctx, "PgBackRestExpireLocal")
}

func (f *fakeExec) PgBackRestExpireOffsite(ctx context.Context, adminDir string, keepFull int) error {
	f.mu.Lock()
	f.offsite = append(f.offsite, adminDir)
	f.expires = append(f.expires, keepFull)
	f.mu.Unlock()
	return f.hit(ctx, "PgBackRestExpireOffsite")
}

func (f *fakeExec) PgDump(ctx context.Context, path string) (int64, error) {
	f.mu.Lock()
	f.dumps = append(f.dumps, path)
	f.mu.Unlock()
	if err := os.WriteFile(path, []byte("PGDMP EXAMPLE"), 0o600); err != nil {
		return 0, err
	}
	if err := f.hit(ctx, "PgDump"); err != nil {
		return 0, err
	}
	return 13, nil
}

func (f *fakeExec) ResticBackup(ctx context.Context, req app.ResticBackup) (app.ResticResult, error) {
	f.mu.Lock()
	f.restic = append(f.restic, req)
	f.mu.Unlock()
	method := "ResticBackup"
	if req.Stdin != nil {
		method = "ResticBackupStdin"
	}
	if err := f.hit(ctx, method); err != nil {
		return app.ResticResult{}, err
	}
	return app.ResticResult{SnapshotID: strings.Repeat("0f", 32), SizeBytes: 2048}, nil
}

func (f *fakeExec) ResticForget(ctx context.Context, req app.ResticForget) error {
	f.mu.Lock()
	f.forgets = append(f.forgets, req)
	f.mu.Unlock()
	return f.hit(ctx, "ResticForget")
}

func (f *fakeExec) S3Mirror(ctx context.Context) (app.MirrorResult, error) {
	if err := f.hit(ctx, "S3Mirror"); err != nil {
		return app.MirrorResult{}, err
	}
	return app.MirrorResult{Copied: 2, SizeBytes: 4096}, nil
}

func (f *fakeExec) VolumeUsage(ctx context.Context, path string) (app.VolumeUsage, error) {
	f.mu.Lock()
	f.volumes = append(f.volumes, path)
	u := f.usage[path]
	f.mu.Unlock()
	if err := f.hit(ctx, "VolumeUsage:"+path); err != nil {
		return app.VolumeUsage{}, err
	}
	return u, nil
}

// fakeSource is an app.ManifestSource with fixed facts.
type fakeSource struct {
	mu          sync.Mutex
	facts       app.ManifestFacts
	err         error
	archived    time.Time
	archivedOK  bool
	archivedErr error
	snapshots   int
}

func newFakeSource() *fakeSource {
	return &fakeSource{facts: app.ManifestFacts{
		SchemaVersion: 49, InstanceID: "0192f4a0-7b1c-7000-8a00-0000000000a1", SyncEpoch: "0192f4a0-7b1c-7000-8a00-0000000000a2",
		HeadSeq: 120, MinRetainedSeq: 1, AuditLastSeq: 42, AuditLastHash: []byte{0xab, 0xcd, 0x01},
		RowCounts: map[string]int64{"messages": 100, "event_log": 120}, FileCount: 3, FileBytes: 4096,
	}}
}

func (s *fakeSource) Snapshot(context.Context) (app.ManifestFacts, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots++
	return s.facts, s.err
}

func (s *fakeSource) LastArchived(context.Context) (time.Time, bool, error) {
	return s.archived, s.archivedOK, s.archivedErr
}

// fakeRow is one row of the in-memory backup_runs.
type fakeRow struct {
	id       ids.ID
	kind     status.BackupKind
	state    string
	started  time.Time
	finished time.Time
	size     int64
	manifest []byte
	checks   []byte
	code     string
}

// fakeRuns is an in-memory status.BackupRuns over the clock clk. states records every state a
// row passed through, so a test sees running before succeeded.
type fakeRuns struct {
	mu         sync.Mutex
	clk        clock.Clock
	rows       []*fakeRow
	startErr   error
	finishErr  error
	lastErr    error
	lastCalls  [][]status.BackupKind
	states     map[ids.ID][]string
	finishCtxs []error
}

func newFakeRuns(clk clock.Clock) *fakeRuns {
	return &fakeRuns{clk: clk, states: map[ids.ID][]string{}}
}

func (r *fakeRuns) Start(_ context.Context, kind status.BackupKind) (ids.ID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.startErr != nil {
		return ids.ID{}, r.startErr
	}
	id := ids.New()
	r.rows = append(r.rows, &fakeRow{id: id, kind: kind, state: "running", started: r.clk.Now(), size: -1})
	r.states[id] = []string{"running"}
	return id, nil
}

func (r *fakeRuns) Finish(ctx context.Context, run status.FinishedRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finishCtxs = append(r.finishCtxs, ctx.Err())
	if r.finishErr != nil {
		return r.finishErr
	}
	for _, row := range r.rows {
		if row.id != run.ID {
			continue
		}
		if row.state != "running" {
			return errors.New("fake: the row is not running")
		}
		row.state = "failed"
		if run.Succeeded {
			row.state = "succeeded"
		}
		row.finished = r.clk.Now()
		row.size = run.SizeBytes
		row.manifest = run.Manifest
		row.checks = run.Checks
		row.code = run.Error
		r.states[run.ID] = append(r.states[run.ID], row.state)
		return nil
	}
	return errors.New("fake: no such row")
}

func (r *fakeRuns) LastSuccess(_ context.Context, kinds ...status.BackupKind) (time.Time, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastCalls = append(r.lastCalls, slices.Clone(kinds))
	if r.lastErr != nil {
		return time.Time{}, false, r.lastErr
	}
	var last time.Time
	found := false
	for _, row := range r.rows {
		if row.state == "succeeded" && slices.Contains(kinds, row.kind) && (!found || row.started.After(last)) {
			last, found = row.started, true
		}
	}
	return last, found, nil
}

func (r *fakeRuns) LatestSucceeded(_ context.Context, kinds ...status.BackupKind) (status.BackupRun, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastErr != nil {
		return status.BackupRun{}, false, r.lastErr
	}
	var out status.BackupRun
	found := false
	for _, row := range r.rows {
		if row.state == "succeeded" && slices.Contains(kinds, row.kind) && (!found || !row.started.Before(out.StartedAt)) {
			out, found = status.BackupRun{ID: row.id, Kind: row.kind, StartedAt: row.started, Manifest: row.manifest}, true
		}
	}
	return out, found, nil
}

// snapshot returns copies of the rows.
func (r *fakeRuns) snapshot() []fakeRow {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]fakeRow, 0, len(r.rows))
	for _, row := range r.rows {
		out = append(out, *row)
	}
	return out
}

// finished is one RunFinished call.
type finished struct {
	kind       domain.Kind
	succeeded  bool
	duration   time.Duration
	size       int64
	finishedAt time.Time
}

// fakeMetrics records the calls of app.Metrics.
type fakeMetrics struct {
	mu       sync.Mutex
	finished []finished
	last     map[domain.Kind]time.Time
	wal      []time.Time
	volumes  map[string]app.VolumeUsage
	failed   [][]domain.CheckName
}

func newFakeMetrics() *fakeMetrics {
	return &fakeMetrics{last: map[domain.Kind]time.Time{}, volumes: map[string]app.VolumeUsage{}}
}

func (m *fakeMetrics) RunFinished(_ context.Context, kind domain.Kind, succeeded bool, d time.Duration, size int64, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finished = append(m.finished, finished{kind, succeeded, d, size, at})
}

func (m *fakeMetrics) LastSuccess(_ context.Context, kind domain.Kind, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.last[kind] = at
}

func (m *fakeMetrics) WALArchived(_ context.Context, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wal = append(m.wal, at)
}

func (m *fakeMetrics) Volume(_ context.Context, volume string, usage app.VolumeUsage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.volumes[volume] = usage
}

func (m *fakeMetrics) RestoreChecksFailed(_ context.Context, checks []domain.CheckName) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failed = append(m.failed, slices.Clone(checks))
}
