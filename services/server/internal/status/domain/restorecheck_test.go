// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// validChecks returns restore_test_checks.v1 with every check passed.
func validChecks() domain.RestoreChecks {
	c := domain.RestoreChecks{
		Schema:   domain.RestoreChecksSchema,
		BackupID: "0192f4a0-7b1c-7000-8a00-000000000001",
		Label:    "20261018-020000F",
	}
	for i, name := range domain.RestoreCheckNames() {
		c.Checks = append(c.Checks, domain.CheckResult{Name: name, Result: domain.ResultPassed, DurationMS: int64(10 * i)})
	}
	return c
}

// TestRestoreChecksSchema holds restore_test_checks.v1 (docs/spec/03-datenmodell.md 4.12
// backup_runs.checks, 16 8.3): the seven checks in the order of 8.3, each with its own result
// (passed, failed, skipped) and duration; failed and skipped carry a reason code, passed none;
// unknown keys, other names, another order and text in place of a code fail.
func TestRestoreChecksSchema(t *testing.T) {
	t.Parallel()
	want := []domain.CheckName{"pgbackrest", "manifest", "event_log", "audit_chain", "thread_sample", "file_sample", "duration"}
	if got := domain.RestoreCheckNames(); !slices.Equal(got, want) {
		t.Fatalf("RestoreCheckNames = %v, want %v", got, want)
	}

	ok := validChecks()
	b, err := ok.Marshal()
	if err != nil {
		t.Fatalf("Marshal of a valid result: %v", err)
	}
	back, err := domain.ParseRestoreChecks(b)
	if err != nil {
		t.Fatalf("ParseRestoreChecks(Marshal): %v", err)
	}
	if !back.Passed() || len(back.Failed()) != 0 {
		t.Errorf("all passed: Passed %v Failed %v", back.Passed(), back.Failed())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"schema", "backup_id", "pgbackrest_label", "checks"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("key %s missing in %s", k, b)
		}
	}

	failed := validChecks()
	failed.Checks[2] = domain.CheckResult{Name: domain.CheckEventLog, Result: domain.ResultFailed, DurationMS: 3, Reason: "event_log_gap", Items: 2}
	failed.Checks[4] = domain.CheckResult{Name: domain.CheckThreadSample, Result: domain.ResultSkipped, Reason: domain.ReasonRestoreFailed}
	if err := failed.Validate(); err != nil {
		t.Fatalf("failed and skipped with reasons: %v", err)
	}
	if failed.Passed() || !slices.Equal(failed.Failed(), []domain.CheckName{domain.CheckEventLog}) {
		t.Errorf("Passed %v Failed %v, want false [event_log]", failed.Passed(), failed.Failed())
	}

	noBackup := domain.RestoreChecks{Schema: domain.RestoreChecksSchema}
	for _, name := range domain.RestoreCheckNames() {
		noBackup.Checks = append(noBackup.Checks, domain.CheckResult{Name: name, Result: domain.ResultSkipped, Reason: domain.ReasonRestoreFailed})
	}
	noBackup.Checks[0] = domain.CheckResult{Name: domain.CheckPgBackRest, Result: domain.ResultFailed, Reason: "no_backup"}
	if err := noBackup.Validate(); err != nil {
		t.Errorf("no backup to test (no id, no label): %v", err)
	}

	for _, c := range []struct {
		name   string
		mutate func(*domain.RestoreChecks)
	}{
		{"schema", func(c *domain.RestoreChecks) { c.Schema = "restore_test_checks.v2" }},
		{"six checks", func(c *domain.RestoreChecks) { c.Checks = c.Checks[:6] }},
		{"order", func(c *domain.RestoreChecks) { c.Checks[0], c.Checks[1] = c.Checks[1], c.Checks[0] }},
		{"unknown name", func(c *domain.RestoreChecks) { c.Checks[6].Name = "speed" }},
		{"unknown result", func(c *domain.RestoreChecks) { c.Checks[1].Result = "ok" }},
		{"negative duration", func(c *domain.RestoreChecks) { c.Checks[1].DurationMS = -1 }},
		{"negative items", func(c *domain.RestoreChecks) { c.Checks[1].Items = -1 }},
		{"failed without reason", func(c *domain.RestoreChecks) { c.Checks[1].Result = domain.ResultFailed }},
		{"skipped without reason", func(c *domain.RestoreChecks) { c.Checks[1].Result = domain.ResultSkipped }},
		{"passed with reason", func(c *domain.RestoreChecks) { c.Checks[1].Reason = "row_counts_differ" }},
		{"text as reason", func(c *domain.RestoreChecks) {
			c.Checks[1] = domain.CheckResult{Name: domain.CheckManifest, Result: domain.ResultFailed, Reason: "table messages has 3 rows"}
		}},
		{"backup id", func(c *domain.RestoreChecks) { c.BackupID = "run-1" }},
		{"label", func(c *domain.RestoreChecks) { c.Label = "latest" }},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			v := validChecks()
			v.Checks = slices.Clone(v.Checks)
			c.mutate(&v)
			if err := v.Validate(); !errors.Is(err, domain.ErrInvalidRestoreChecks) {
				t.Errorf("Validate = %v, want ErrInvalidRestoreChecks", err)
			}
			if _, err := v.Marshal(); err == nil {
				t.Error("Marshal accepted an invalid result")
			}
		})
	}

	for _, in := range []string{
		strings.Replace(string(b), `"schema"`, `"extra":1,"schema"`, 1),
		strings.Replace(string(b), `"duration_ms"`, `"note":"x","duration_ms"`, 1),
		string(b) + `{}`,
		`[]`,
	} {
		if _, err := domain.ParseRestoreChecks([]byte(in)); !errors.Is(err, domain.ErrInvalidRestoreChecks) {
			t.Errorf("ParseRestoreChecks(%.60s...) = %v, want ErrInvalidRestoreChecks", in, err)
		}
	}
}
