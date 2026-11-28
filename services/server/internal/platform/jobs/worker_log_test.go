// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"

	"github.com/OWNER/bothy/services/server/internal/platform/jobs"
)

// logLines returns the JSON lines written to buf.
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

// TestOutcomeOf_TimeoutIsFailed: River's job timeout ends the context with DeadlineExceeded and
// uses up an attempt, so it is failed; only a canceled context (stop, cancellation) is cancelled.
func TestOutcomeOf_TimeoutIsFailed(t *testing.T) {
	t.Parallel()
	timedOut, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-timedOut.Done()
	canceled, cancel2 := context.WithCancel(context.Background())
	cancel2()
	for _, c := range []struct {
		name string
		ctx  context.Context
		err  error
		want string
	}{
		{"nil", context.Background(), nil, "completed"},
		{"job timeout", timedOut, context.DeadlineExceeded, "failed"},
		{"stop", canceled, context.Canceled, "cancelled"},
		{"own deadline", context.Background(), context.DeadlineExceeded, "failed"},
		{"other", canceled, errors.New("x"), "failed"},
	} {
		if got := jobs.OutcomeOfForTest(c.ctx, c.err); got != c.want {
			t.Errorf("%s: outcome %q, want %q", c.name, got, c.want)
		}
	}
}

// TestWorker_LogsFailedAttempt: a failed attempt writes "job attempt failed" with
// bothy.job.kind and error.type, Warn while attempts remain and Error for the last one; the
// error text never appears. A panic keeps its own line only.
func TestWorker_LogsFailedAttempt(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name          string
		err           error
		attempt, max  int
		level, errTyp string
	}{
		{"retried", &pgconn.PgError{Code: "40001", Message: "secret content"}, 1, 3, "WARN", "40001"},
		{"last", errors.New("secret content"), 3, 3, "ERROR", "internal"},
		{"timeout", context.DeadlineExceeded, 1, 3, "WARN", "timeout"},
	} {
		var buf bytes.Buffer
		log := slog.New(slog.NewJSONHandler(&buf, nil))
		if err := jobs.WorkFailingForTest(log, c.err, c.attempt, c.max); err == nil {
			t.Fatalf("%s: Work = nil", c.name)
		}
		lines := logLines(t, &buf)
		if len(lines) != 1 || lines[0]["msg"] != "job attempt failed" || lines[0]["level"] != c.level ||
			lines[0]["bothy.job.kind"] != "jobs.timeout" || lines[0]["error.type"] != c.errTyp {
			t.Errorf("%s: log %v, want one %s line with kind and error.type %s", c.name, lines, c.level, c.errTyp)
		}
		if strings.Contains(buf.String(), "secret") {
			t.Errorf("%s: the log carries the error text: %s", c.name, buf.String())
		}
	}
	var buf bytes.Buffer
	_ = jobs.WorkFailingForTest(slog.New(slog.NewJSONHandler(&buf, nil)), nil, 1, 3)
	if lines := logLines(t, &buf); len(lines) != 1 || lines[0]["msg"] != "job panicked" {
		t.Errorf("panic: log %v, want the panic line only", lines)
	}
}

// TestWorker_SnoozeHoldsTheJob: a handler that returns jobs.Snooze, also wrapped, gives River's
// snooze with the pause (which uses up no attempt), the outcome snoozed and no log line, even on
// the last attempt; a negative pause counts as 0.
func TestWorker_SnoozeHoldsTheJob(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		err  error
		want time.Duration
	}{
		{"plain", jobs.Snooze(3 * time.Second), 3 * time.Second},
		{"wrapped", fmt.Errorf("busy: %w", jobs.Snooze(time.Second)), time.Second},
		{"negative", jobs.Snooze(-time.Second), 0},
	} {
		var buf bytes.Buffer
		err := jobs.WorkFailingForTest(slog.New(slog.NewJSONHandler(&buf, nil)), c.err, 3, 3)
		var snooze *river.JobSnoozeError
		if !errors.As(err, &snooze) || snooze.Duration != c.want {
			t.Errorf("%s: Work = %v, want River's snooze of %v", c.name, err, c.want)
		}
		if buf.Len() != 0 {
			t.Errorf("%s: a snooze logged %s", c.name, buf.String())
		}
		if got := jobs.OutcomeOfForTest(context.Background(), c.err); got != "snoozed" {
			t.Errorf("%s: outcome %q, want snoozed", c.name, got)
		}
	}
}

// TestRiverLogger_RoutineLinesAtDebug: River's Info lines reach the server logger at Debug, so an
// Info logger drops them; Warn and Error stay, with River's job_kind as bothy.job.kind and no
// other attribute of River.
func TestRiverLogger_RoutineLinesAtDebug(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := jobs.RiverLoggerForTest(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	log.Info("producer: Producer job counts", slog.String("queue", "default"))
	log.Warn("jobexecutor.JobExecutor: Job appears to be stuck", slog.String("job_kind", "files.scan"), slog.Int64("job_id", 7))
	log.With(slog.String("x", "y")).Error("producer: Error fetching jobs", slog.String("error", "secret content"))
	lines := logLines(t, &buf)
	if len(lines) != 2 {
		t.Fatalf("lines %v, want the Warn and the Error line only", lines)
	}
	if lines[0]["level"] != "WARN" || lines[0]["bothy.job.kind"] != "files.scan" || lines[0]["job_id"] != nil {
		t.Errorf("warn line %v", lines[0])
	}
	if lines[1]["level"] != "ERROR" || lines[1]["error"] != nil || lines[1]["x"] != nil {
		t.Errorf("error line %v", lines[1])
	}

	buf.Reset()
	debug := jobs.RiverLoggerForTest(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	debug.Info("maintenance.JobCleaner: Ran successfully")
	if lines := logLines(t, &buf); len(lines) != 1 || lines[0]["level"] != "DEBUG" {
		t.Errorf("debug logger: %v, want the line at DEBUG", lines)
	}
}
