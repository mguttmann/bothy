// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// Scheduler starts the kinds on their schedules (docs/spec/16-betrieb.md 8.1, brief M7-02 design
// note 1). It does not own a timer: the caller ticks it, so the loop of cmd/bothy-backupd decides
// the cadence and a test moves a fake clock. Each Tick covers the window from the previous tick,
// so a minute is started at most once and a time missed before the start is never caught up.
// Tick and Wait are safe for concurrent use.
type Scheduler struct {
	svc *Service

	mu   sync.Mutex
	prev time.Time
	wg   sync.WaitGroup
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
	return &Scheduler{svc: svc, prev: start}
}

// Tick starts the kinds due in the window (previous tick, now] (domain.DueKinds), each in its own
// goroutine with ctx, unless its slot is busy: a busy kind is skipped, not queued, because its
// next due time comes anyway. A now that is not after the previous tick starts nothing and keeps
// the window.
//
// It returns the kinds it started, in the order of domain.Kinds(). The runs report their outcome
// through backup_runs, the log and the metrics; Wait waits for them.
func (s *Scheduler) Tick(ctx context.Context, now time.Time) []domain.Kind {
	s.mu.Lock()
	from := s.prev
	if now.After(s.prev) {
		s.prev = now
	}
	s.mu.Unlock()
	var started []domain.Kind
	for _, kind := range domain.DueKinds(s.svc.cfg.Schedules, from, now) {
		// The slot is taken here, not in the goroutine, so the result is exact and a second tick
		// cannot start the same slot before the first run has begun.
		if !s.svc.acquire(kind) {
			s.svc.log.LogAttrs(ctx, slog.LevelWarn, "backup run skipped: the previous run of its kind is still running",
				slog.String(attrKind, string(kind)))
			continue
		}
		started = append(started, kind)
		s.wg.Go(func() {
			defer s.svc.release(kind)
			_ = s.svc.run(ctx, kind) // the run logs and records its outcome itself
		})
	}
	return started
}

// Wait blocks until every run started by Tick has finished.
func (s *Scheduler) Wait() { s.wg.Wait() }
