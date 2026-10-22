// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/audit"
	"github.com/OWNER/bothy/services/server/internal/status"
	"github.com/OWNER/bothy/services/server/internal/status/app"
	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

var errFake = errors.New("fake: unavailable")

// TestRestoreCheckFailureReasons: every check names the cause of its failure as a reason code of
// restore_test_checks.v1: a query of the copy that fails, a table the manifest does not know, an
// audit head whose hash differs, a missing sequence counter, a verifier that fails or reports a
// reason outside the code form, and an object fetch that fails. The failed check fails the test
// with checks_failed; the other checks keep their result.
func TestRestoreCheckFailureReasons(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		damage func(r *restoreEnv)
		check  domain.CheckName
		reason string
	}{
		{"facts unreadable", func(r *restoreEnv) { r.scratch.db.err["Facts"] = errFake }, domain.CheckManifest, "scratch_query_failed"},
		{"unexpected table", func(r *restoreEnv) {
			counts := maps.Clone(r.scratch.db.facts.RowCounts)
			counts["intruder"] = 1
			r.scratch.db.facts.RowCounts = counts
		}, domain.CheckManifest, "table_unexpected"},
		{"audit head hash", func(r *restoreEnv) { r.scratch.db.facts.AuditLastHash = []byte{0x01} }, domain.CheckManifest, "audit_head_mismatch"},
		{"event log unreadable", func(r *restoreEnv) { r.scratch.db.err["EventLog"] = errFake }, domain.CheckEventLog, "scratch_query_failed"},
		{"counter missing", func(r *restoreEnv) {
			r.scratch.db.events.Counters = map[string]int64{"event_seq": 120}
		}, domain.CheckEventLog, "sequence_counter_missing"},
		{"audit rows unreadable", func(r *restoreEnv) { r.scratch.db.err["AuditChain"] = errFake }, domain.CheckAuditChain, "scratch_query_failed"},
		{"verifier fails", func(r *restoreEnv) { r.chain.err = errFake }, domain.CheckAuditChain, "scratch_query_failed"},
		{"verifier reason not a code", func(r *restoreEnv) {
			r.chain.res = audit.VerifyResult{OK: false, FailureReason: "Not A Code"}
		}, domain.CheckAuditChain, "audit_chain_broken"},
		{"threads unreadable", func(r *restoreEnv) { r.scratch.db.err["ThreadSample"] = errFake }, domain.CheckThreadSample, "scratch_query_failed"},
		{"files unreadable", func(r *restoreEnv) { r.scratch.db.err["FileSample"] = errFake }, domain.CheckFileSample, "scratch_query_failed"},
		{"objects not fetched", func(r *restoreEnv) {
			r.scratch.err["FetchObjects"] = &app.ExecError{Code: "restic_exit_1"}
		}, domain.CheckFileSample, "restic_exit_1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newRestoreEnv(t, nil)
			c.damage(r)
			row, checks, err := r.test(t)
			if err == nil || !strings.Contains(err.Error(), string(c.check)) {
				t.Errorf("RestoreTest = %v, want an error naming %s", err, c.check)
			}
			if res := result(checks, c.check); res.Result != domain.ResultFailed || res.Reason != c.reason {
				t.Errorf("%s = %+v, want failed with %s", c.check, res, c.reason)
			}
			if row.state != "failed" || row.code != "checks_failed" {
				t.Errorf("row %s %q, want failed with checks_failed", row.state, row.code)
			}
			if res := result(checks, domain.CheckPgBackRest); res.Result != domain.ResultPassed {
				t.Errorf("pgbackrest = %+v, want passed", res)
			}
		})
	}
}

// TestRestoreNoFilesPasses: a copy without a complete file passes check 6 with no item and
// fetches nothing.
func TestRestoreNoFilesPasses(t *testing.T) {
	t.Parallel()
	r := newRestoreEnv(t, nil)
	r.scratch.setFiles(0)
	_, checks, err := r.test(t)
	if err != nil {
		t.Fatalf("RestoreTest: %v", err)
	}
	if res := result(checks, domain.CheckFileSample); res.Result != domain.ResultPassed || res.Items != 0 {
		t.Errorf("file_sample = %+v, want passed with no item", res)
	}
	if len(r.scratch.fetched) != 0 {
		t.Errorf("fetched %v, want nothing", r.scratch.fetched)
	}
}

// TestRestoreRecordingFailures: a test whose start cannot be recorded runs nothing; an
// unreadable backup_runs fails check 1 with backup_runs_unreadable and skips the copy; a
// scratch area that cannot be cleared fails the row with scratch_cleanup_failed even when every
// check passed; a row whose end cannot be recorded returns that error.
func TestRestoreRecordingFailures(t *testing.T) {
	t.Parallel()
	t.Run("start not recorded", func(t *testing.T) {
		t.Parallel()
		r := newRestoreEnv(t, nil)
		r.runs.mu.Lock()
		r.runs.startErr = errFake
		r.runs.mu.Unlock()
		if err := r.svc.RestoreTest(context.Background()); !errors.Is(err, errFake) {
			t.Errorf("RestoreTest = %v, want the recording error", err)
		}
		if calls := r.scratch.callList(); len(calls) != 0 {
			t.Errorf("scratch calls %v, want none", calls)
		}
	})
	t.Run("backup_runs unreadable", func(t *testing.T) {
		t.Parallel()
		r := newRestoreEnv(t, nil)
		r.runs.mu.Lock()
		r.runs.lastErr = errFake
		r.runs.mu.Unlock()
		_, checks, err := r.test(t)
		if err == nil {
			t.Error("RestoreTest succeeded without backup_runs")
		}
		if res := result(checks, domain.CheckPgBackRest); res.Result != domain.ResultFailed || res.Reason != "backup_runs_unreadable" {
			t.Errorf("pgbackrest = %+v, want failed with backup_runs_unreadable", res)
		}
		if res := result(checks, domain.CheckManifest); res.Result != domain.ResultSkipped {
			t.Errorf("manifest = %+v, want skipped", res)
		}
	})
	t.Run("scratch not cleared", func(t *testing.T) {
		t.Parallel()
		r := newRestoreEnv(t, nil)
		r.scratch.err["Clear"] = &app.ExecError{Code: "scratch_cleanup_failed"}
		row, checks, err := r.test(t)
		if err == nil || !strings.Contains(err.Error(), "scratch_cleanup_failed") {
			t.Errorf("RestoreTest = %v, want an error naming scratch_cleanup_failed", err)
		}
		if row.state != "failed" || row.code != "scratch_cleanup_failed" {
			t.Errorf("row %s %q, want failed with scratch_cleanup_failed", row.state, row.code)
		}
		if !checks.Passed() {
			t.Errorf("checks %v, want every check passed", outcomes(checks))
		}
	})
	t.Run("end not recorded", func(t *testing.T) {
		t.Parallel()
		r := newRestoreEnv(t, nil)
		r.runs.mu.Lock()
		r.runs.finishErr = errFake
		r.runs.mu.Unlock()
		if err := r.svc.RestoreTest(context.Background()); !errors.Is(err, errFake) {
			t.Errorf("RestoreTest = %v, want the recording error", err)
		}
		rows := r.runs.snapshot()
		if last := rows[len(rows)-1]; last.kind != status.BackupRestoreTest {
			t.Errorf("last row %s, want restore_test", last.kind)
		}
	})
}
