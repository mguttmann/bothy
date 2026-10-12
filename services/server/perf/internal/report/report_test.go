// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

func TestReport_FailsBelowThreshold(t *testing.T) {
	t.Parallel()
	if err := report.CheckMinimum("perf.x", 999.9, 1000); err == nil || !strings.Contains(err.Error(), "perf.x") {
		t.Fatalf("CheckMinimum below = %v, want an error naming the target", err)
	}
	for _, measured := range []float64{1000, 1000.1, 5000} {
		if err := report.CheckMinimum("perf.x", measured, 1000); err != nil {
			t.Errorf("CheckMinimum(%v) = %v, want nil at or above the threshold", measured, err)
		}
	}
}

// TestReport_ReadsTheRealTable: the loader finds the section of 02 by its title and reads the
// two values the event commit test needs; a threshold literal never appears in the test.
func TestReport_ReadsTheRealTable(t *testing.T) {
	t.Parallel()
	targets, err := report.Load(report.ArchitectureChapter())
	if err != nil {
		t.Fatal(err)
	}
	min, err := targets.Minimum("perf.event_commit_min_per_s")
	if err != nil || min <= 0 {
		t.Fatalf("perf.event_commit_min_per_s = %v, %v", min, err)
	}
	profile, err := targets.Text("perf.reference_profile")
	if err != nil || !strings.Contains(profile, "fsync") {
		t.Fatalf("perf.reference_profile = %q, %v", profile, err)
	}
	if _, err := targets.Text("sync.log_poll_interval"); err == nil {
		t.Fatal("a server key was read as a load test value; only place Load test counts")
	}
}

func TestReport_MissingOrBadValuesFail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := "# 02\n\n## 22. Configuration values of this chapter\n\n| Name | Place | Default | Meaning |\n|---|---|---|---|\n" +
		"| `perf.a` | Load test | `12` | x |\n| `perf.b` | Load test | `many` | x |\n\n## Sources\n\n| `perf.c` | Load test | `5` | x |\n"
	targets, err := report.Load(write("good.md", good))
	if err != nil {
		t.Fatal(err)
	}
	if v, err := targets.Minimum("perf.a"); err != nil || v != 12 {
		t.Fatalf("perf.a = %v, %v", v, err)
	}
	if _, err := targets.Minimum("perf.b"); err == nil {
		t.Error("an unparseable value was accepted")
	}
	if _, err := targets.Minimum("perf.c"); err == nil {
		t.Error("a row outside the section was read")
	}
	if _, err := targets.Minimum("perf.missing"); err == nil {
		t.Error("a missing row was accepted")
	}
	if _, err := report.Load(write("nosection.md", "# 02\n\n| `perf.a` | Load test | `1` | x |\n")); err == nil {
		t.Error("a chapter without the section was accepted")
	}
	if _, err := report.Load(filepath.Join(dir, "absent.md")); err == nil {
		t.Error("a missing file was accepted")
	}
}
