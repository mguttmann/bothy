// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// logicalOnly keeps only the schedule of logical (03:00 every day).
func logicalOnly(cfg *app.Config) {
	for _, k := range []domain.Kind{domain.KindPGFull, domain.KindPGDiff, domain.KindObjects} {
		delete(cfg.Schedules, k)
	}
}

// ticker ticks s at now with the fake clock set and waits for the started runs.
func ticker(t *testing.T, e *env, s *app.Scheduler) func(now time.Time, want ...domain.Kind) {
	return func(now time.Time, want ...domain.Kind) {
		t.Helper()
		e.clk.Set(now)
		got := s.Tick(context.Background(), now)
		s.Wait()
		if !slices.Equal(got, want) {
			t.Errorf("Tick(%v) = %v, want %v", now.Format(time.DateTime), got, want)
		}
	}
}

// rowsOf returns state:code of every row of kind in order.
func rowsOf(e *env, kind status.BackupKind) []string {
	var out []string
	for _, r := range e.runs.snapshot() {
		if r.kind == kind {
			out = append(out, r.state+":"+r.code)
		}
	}
	return out
}

// TestFailedRunRetried holds the retry of AUD srv-stats-status-8: a failed run starts again
// retryDelay (15 minutes) after it ended, at most twice for one due time; the next due time runs
// as usual and a run that succeeds plans nothing.
func TestFailedRunRetried(t *testing.T) {
	t.Parallel()
	e := newEnv(t, logicalOnly)
	e.exec.err["PgDump"] = &app.ExecError{Code: "pg_dump_exit_1"}
	s := app.NewScheduler(e.svc, e.start)
	tick := ticker(t, e, s)
	three := sunday.Add(2 * time.Hour) // Sunday 03:00
	tick(three, domain.KindLogical)
	tick(three.Add(14 * time.Minute))
	tick(three.Add(16*time.Minute), domain.KindLogical) // retry 1
	tick(three.Add(32*time.Minute), domain.KindLogical) // retry 2
	tick(three.Add(48 * time.Minute))                   // the limit is reached
	tick(three.Add(2 * time.Hour))
	if got, want := rowsOf(e, status.BackupLogical), []string{"failed:pg_dump_exit_1", "failed:pg_dump_exit_1", "failed:pg_dump_exit_1"}; !slices.Equal(got, want) {
		t.Errorf("rows %v, want %v", got, want)
	}
	// The next due time runs; it fails once and its retry succeeds, which plans nothing more.
	tick(three.Add(day), domain.KindLogical)
	delete(e.exec.err, "PgDump")
	tick(three.Add(day+16*time.Minute), domain.KindLogical)
	tick(three.Add(day + 32*time.Minute))
	got := rowsOf(e, status.BackupLogical)
	if len(got) != 5 || got[3] != "failed:pg_dump_exit_1" || got[4] != "succeeded:" {
		t.Errorf("rows %v, want a failed run and a succeeded retry on Monday", got)
	}
}

// TestRunNotRecordedRetried: a run that could not even write its row (a short outage of the
// database) is retried as well.
func TestRunNotRecordedRetried(t *testing.T) {
	t.Parallel()
	e := newEnv(t, logicalOnly)
	e.runs.startErr = errors.New("connection refused")
	s := app.NewScheduler(e.svc, e.start)
	tick := ticker(t, e, s)
	three := sunday.Add(2 * time.Hour)
	tick(three, domain.KindLogical)
	e.runs.mu.Lock()
	e.runs.startErr = nil
	e.runs.mu.Unlock()
	tick(three.Add(16*time.Minute), domain.KindLogical)
	if got := rowsOf(e, status.BackupLogical); !slices.Equal(got, []string{"succeeded:"}) {
		t.Errorf("rows %v, want the retry succeeded", got)
	}
}

// TestRetryFailedAfterRestart holds the restart case of AUD srv-stats-status-8: the newest run of
// a kind that the stop of the previous process canceled (or that failed or was interrupted) and
// that belongs to the latest due time is retried retryDelay after the start; a failed run older
// than the latest due time, a succeeded newest run and a kind without a schedule are not, so a
// time missed while the process was down is still not caught up.
func TestRetryFailedAfterRestart(t *testing.T) {
	t.Parallel()
	three := sunday.Add(2 * time.Hour) // Sunday 03:00
	e := newEnv(t, nil)
	e.runs.add(status.BackupLogical, "failed", "canceled", three)                               // the latest due time
	e.runs.add(status.BackupObjects, "failed", "interrupted", three.Add(-2*day+30*time.Minute)) // Friday 03:30, before Saturday 03:30
	e.runs.add(status.BackupPGFull, "succeeded", "", sunday.Add(time.Hour))
	start := three.Add(10 * time.Minute)
	s := app.NewScheduler(e.svc, start)
	if err := s.RetryFailed(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	// A second call plans nothing twice.
	if err := s.RetryFailed(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	tick := ticker(t, e, s)
	tick(start.Add(14 * time.Minute))
	tick(start.Add(15*time.Minute), domain.KindLogical)
	tick(start.Add(31*time.Minute), domain.KindObjects) // its due time 03:30, no retry
	if got := rowsOf(e, status.BackupLogical); !slices.Equal(got, []string{"failed:canceled", "succeeded:"}) {
		t.Errorf("logical rows %v, want the canceled run and its retry", got)
	}
	if got := rowsOf(e, status.BackupObjects); !slices.Equal(got, []string{"failed:interrupted", "succeeded:"}) {
		t.Errorf("objects rows %v, want no retry of a run older than the latest due time", got)
	}

	// The rows cannot be read: an error, and the next call plans the retry.
	f := newEnv(t, logicalOnly)
	f.runs.add(status.BackupLogical, "failed", "canceled", three)
	f.runs.lastErr = errors.New("connection refused")
	s = app.NewScheduler(f.svc, start)
	if err := s.RetryFailed(context.Background(), start); err == nil {
		t.Error("RetryFailed with unreadable rows = nil")
	}
	f.runs.mu.Lock()
	f.runs.lastErr = nil
	f.runs.mu.Unlock()
	if err := s.RetryFailed(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	ticker(t, f, s)(start.Add(15*time.Minute), domain.KindLogical)
}
