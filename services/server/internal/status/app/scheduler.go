// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// The retry of a failed backup run (AUD srv-stats-status-8): a run of a kind that fails, also
// one that could not write its row, starts again retryDelay after it ended, at most retryLimit
// times for one due time. The next due time of the kind replaces a pending retry. Named values,
// no configuration key: the delay only has to outlast a short outage of the database, the
// off-site target or a container update.
const (
	retryDelay = 15 * time.Minute
	retryLimit = 2
)

// stateFailed is backup_runs.state of a failed run, also of a canceled or interrupted one.
const stateFailed = "failed"

// Scheduler starts the kinds on their schedules (docs/spec/16-operations.md 8.1, brief M7-02 design
// note 1). It does not own a timer: the caller ticks it, so the loop of cmd/bothy-backupd decides
// the cadence and a test moves a fake clock. Each Tick covers the window from the previous tick,
// so a minute is started at most once and a time missed before the start is never caught up,
// except through the retry of a failed run (retryDelay, retryLimit, RetryFailed).
// Tick, RetryFailed and Wait are safe for concurrent use.
type Scheduler struct {
	svc *Service

	mu   sync.Mutex
	prev time.Time
	// retries are the pending retries per backup kind.
	retries map[domain.Kind]pendingRetry
	wg      sync.WaitGroup
}

// pendingRetry is a retry of a failed run: it starts at the first tick at or after at, as the
// retry number try of its due time (1 to retryLimit).
type pendingRetry struct {
	at  time.Time
	try int
}

// NewScheduler returns a scheduler of svc whose first window begins at start, the start of the
// process.
//
// Example:
//
//	s := app.NewScheduler(svc, clk.Now())
//	for now := range ticks { s.Tick(ctx, now) }
//	s.Wait()
func NewScheduler(svc *Service, start time.Time) *Scheduler {
	return &Scheduler{svc: svc, prev: start, retries: map[domain.Kind]pendingRetry{}}
}

// RetryFailed plans a retry, retryDelay after now, of each kind with a schedule whose newest run
// failed (also canceled by the stop of the previous process, or closed as interrupted by
// CloseInterrupted, which must have run before) when no due time of its schedule has passed since
// that run started: such a run belongs to the latest due time, and the stop or the failure kept
// it from its retries. A run older than the latest due time stays as it is, like any time missed
// while the process was down. cmd/bothy-backupd calls it once at start, after CloseInterrupted.
//
// It returns the error of the first kind whose newest run could not be read; the kinds before it
// keep their retries, and a second call plans each kind at most once.
//
// Example:
//
//	if err := s.RetryFailed(ctx, clk.Now()); err != nil { /* call again on the next tick */ }
func (s *Scheduler) RetryFailed(ctx context.Context, now time.Time) error {
	for _, kind := range domain.Kinds() {
		sched, ok := s.svc.cfg.Schedules[kind]
		if !ok {
			continue
		}
		s.mu.Lock()
		_, planned := s.retries[kind]
		s.mu.Unlock()
		if planned {
			continue
		}
		run, ok, err := s.svc.deps.Runs.LatestRun(ctx, status.BackupKind(kind))
		if err != nil {
			s.svc.log.LogAttrs(ctx, slog.LevelError, "backup runs before the start not read", slog.String(attrKind, string(kind)))
			return fmt.Errorf("status: reading the latest %s run: %w", kind, err)
		}
		if !ok || run.State != stateFailed {
			continue
		}
		if next := sched.Next(run.StartedAt); !next.IsZero() && !next.After(now) {
			continue
		}
		s.mu.Lock()
		if _, planned := s.retries[kind]; !planned {
			s.retries[kind] = pendingRetry{at: now.Add(retryDelay), try: 1}
		}
		s.mu.Unlock()
		s.svc.log.LogAttrs(ctx, slog.LevelInfo, "backup run of the previous process is retried", slog.String(attrKind, string(kind)),
			slog.String(attrCode, run.Error))
	}
	return nil
}

// planRetry plans the retry after a failed run that was retry number try of its due time (0 for
// the run of the due time itself), unless retryLimit is reached or the next due time of the kind
// came first.
func (s *Scheduler) planRetry(ctx context.Context, kind domain.Kind, try int) {
	if try >= retryLimit {
		return
	}
	at := s.svc.deps.Clock.Now().Add(retryDelay)
	s.mu.Lock()
	if _, planned := s.retries[kind]; !planned {
		s.retries[kind] = pendingRetry{at: at, try: try + 1}
	}
	s.mu.Unlock()
	s.svc.log.LogAttrs(ctx, slog.LevelInfo, "backup run will be retried", slog.String(attrKind, string(kind)))
}

// startable returns the kinds to start at now and the retry number of each: the due kinds of the
// window (retry 0, replacing a pending retry of the same kind) and the pending retries whose time
// has come, in the order of domain.Kinds(); a pg_full drops a pg_diff, as in domain.DueKinds.
func (s *Scheduler) startable(from, now time.Time) ([]domain.Kind, map[domain.Kind]int) {
	due := domain.DueKinds(s.svc.cfg.Schedules, from, now)
	tries := make(map[domain.Kind]int, len(due))
	s.mu.Lock()
	for _, k := range due {
		tries[k] = 0
		delete(s.retries, k)
	}
	for k, r := range s.retries {
		if !r.at.After(now) {
			tries[k] = r.try
			delete(s.retries, k)
		}
	}
	s.mu.Unlock()
	if _, full := tries[domain.KindPGFull]; full {
		delete(tries, domain.KindPGDiff)
	}
	var kinds []domain.Kind
	for _, k := range domain.Kinds() {
		if _, ok := tries[k]; ok {
			kinds = append(kinds, k)
		}
	}
	return kinds, tries
}

// Tick starts the kinds due in the window (previous tick, now] (domain.DueKinds) and the retries
// whose time has come (startable), each in its own goroutine with ctx, unless its slot is busy: a
// busy kind is skipped, not queued, because its next due time comes anyway. A run that fails while
// ctx lives plans its retry (planRetry). A now that is not after the previous tick starts no due
// kind and keeps the window.
//
// After the backups it starts the restore test (domain.KindRestoreTest) when
// backup.restore_test.schedule is due in the window, in its own slot (M7-03).
//
// It returns the kinds it started, in the order of domain.Kinds(), the restore test last. The runs
// report their outcome through backup_runs, the log and the metrics; Wait waits for them.
func (s *Scheduler) Tick(ctx context.Context, now time.Time) []domain.Kind {
	s.mu.Lock()
	from := s.prev
	if now.After(s.prev) {
		s.prev = now
	}
	s.mu.Unlock()
	var started []domain.Kind
	kinds, tries := s.startable(from, now)
	for _, kind := range kinds {
		// The slot is taken here, not in the goroutine, so the result is exact and a second tick
		// cannot start the same slot before the first run has begun.
		if !s.svc.acquire(kind) {
			s.svc.log.LogAttrs(ctx, slog.LevelWarn, "backup run skipped: the previous run of its kind is still running",
				slog.String(attrKind, string(kind)))
			continue
		}
		started = append(started, kind)
		try := tries[kind]
		s.wg.Go(func() {
			defer s.svc.release(kind)
			// The run logs and records its outcome itself. A run the stop canceled is retried by
			// the next process (RetryFailed).
			if err := s.svc.run(ctx, kind); err != nil && ctx.Err() == nil {
				s.planRetry(ctx, kind, try)
			}
		})
	}
	if s.restoreDue(from, now) {
		if !s.svc.acquire(domain.KindRestoreTest) {
			s.svc.log.LogAttrs(ctx, slog.LevelWarn, "restore test skipped: the previous restore test is still running")
			return started
		}
		started = append(started, domain.KindRestoreTest)
		s.wg.Go(func() {
			defer s.svc.release(domain.KindRestoreTest)
			_ = s.svc.restoreTest(ctx) // the test logs and records its outcome itself
		})
	}
	return started
}

// restoreDue reports whether backup.restore_test.schedule has a matching minute in the window
// (from, to] and the service can run the restore test.
func (s *Scheduler) restoreDue(from, to time.Time) bool {
	r := s.svc.cfg.Restore
	if !r.Scheduled || s.svc.deps.Scratch == nil || s.svc.deps.Chain == nil || !to.After(from) {
		return false
	}
	next := r.Schedule.Next(from)
	return !next.IsZero() && !next.After(to)
}

// Wait blocks until every run started by Tick has finished.
func (s *Scheduler) Wait() { s.wg.Wait() }
