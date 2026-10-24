// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

// TestThresholdsFromSearchChapter (brief M3-16 "Tests"): the real chapter 14 gives
// search.p95_target and search.prefix_p95_target in ms with <=, and search.benchmark_messages as
// a count; read together with 02 the targets of 02 are unchanged.
func TestThresholdsFromSearchChapter(t *testing.T) {
	t.Parallel()
	targets, err := report.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"search.p95_target", "search.prefix_p95_target"} {
		th, err := targets.Threshold(name)
		if err != nil {
			t.Fatal(err)
		}
		if th.Unit != "ms" || th.Comparison != report.AtMost || th.Base <= 0 {
			t.Errorf("%s = %+v, want unit ms, <= and a positive value", name, th)
		}
	}
	if n, err := targets.Count("search.benchmark_messages"); err != nil || n <= 0 {
		t.Errorf("search.benchmark_messages = %v, %v", n, err)
	}
	if _, err := targets.Threshold("search.benchmark_messages"); err == nil {
		t.Error("the corpus size was read as a target")
	}
	alone, err := report.Load(report.ArchitectureChapter())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range report.Required {
		if strings.HasPrefix(name, "search.") {
			continue
		}
		a, errA := alone.Threshold(name)
		b, errB := targets.Threshold(name)
		if errA != nil || errB != nil || a != b {
			t.Errorf("%s: 02 alone %+v (%v), with 14 %+v (%v)", name, a, errA, b, errB)
		}
	}
	for _, name := range []string{"search.p95_target", "search.prefix_p95_target"} {
		if !slices.Contains(report.Required, name) {
			t.Errorf("%s is not required: a run without the search test would end green", name)
		}
	}
}

// writeChapter writes a fixture chapter with one table section and returns its path.
func writeChapter(t *testing.T, name, rows string) string {
	t.Helper()
	body := "# " + name + "\n\n## 15. Configuration values of this chapter\n\n| Name | Place | Default | Meaning |\n|---|---|---|---|\n" + rows + "\n## 16. Sources\n"
	p := filepath.Join(t.TempDir(), name+".md")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestLoadTestRowsAcrossChapters (brief M3-16 "Tests"): rows of both chapters are read; a name in
// both fails; a row with place Server is never loaded; a missing row fails; the meaning decides
// target and comparison.
func TestLoadTestRowsAcrossChapters(t *testing.T) {
	t.Parallel()
	arch := writeChapter(t, "02", "| `perf.min` | Load test | `1000` | target (minimum) |\n| `sync.x` | Server | `1s` | poll |\n")
	search := writeChapter(t, "14", "| `search.languages` | Server | `[]` | languages |\n"+
		"| `search.p95_target` | Load test | `500ms` | p95 target in the load test |\n"+
		"| `search.benchmark_messages` | Load test | `2000000` | corpus size in the load test |\n")
	targets, err := report.LoadChapters(arch, search)
	if err != nil {
		t.Fatal(err)
	}
	if th, err := targets.Threshold("perf.min"); err != nil || th.Comparison != report.AtLeast {
		t.Errorf("target (minimum) = %+v, %v; want >=", th, err)
	}
	if th, err := targets.Threshold("search.p95_target"); err != nil || th.Comparison != report.AtMost || th.Unit != "ms" {
		t.Errorf("p95 target in the load test = %+v, %v; want <= in ms", th, err)
	}
	if _, err := targets.Threshold("search.benchmark_messages"); err == nil {
		t.Error("corpus size in the load test was read as a target")
	}
	if n, err := targets.Count("search.benchmark_messages"); err != nil || n != 2000000 {
		t.Errorf("corpus size = %d, %v", n, err)
	}
	for _, name := range []string{"sync.x", "search.languages"} {
		if _, err := targets.Text(name); err == nil {
			t.Errorf("%s with place Server was loaded", name)
		}
	}
	missing := writeChapter(t, "14b", "| `search.benchmark_messages` | Load test | `2000000` | corpus size in the load test |\n")
	targets, err = report.LoadChapters(arch, missing)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := targets.Threshold("search.p95_target"); err == nil || !strings.Contains(err.Error(), "search.p95_target") {
		t.Errorf("missing search.p95_target: %v", err)
	}
	twice := writeChapter(t, "14c", "| `perf.min` | Load test | `5` | target (minimum) |\n")
	if _, err := report.LoadChapters(arch, twice); err == nil || !strings.Contains(err.Error(), "perf.min") {
		t.Errorf("a name in both chapters: %v, want an error naming it", err)
	}
	for meaning, want := range map[string]report.Comparison{
		"p95 target in the load test": report.AtMost, "p95 target of the prefix search": report.AtMost, "target (minimum)": report.AtLeast,
	} {
		if got, err := report.ComparisonFromMeaning(meaning); err != nil || got != want {
			t.Errorf("ComparisonFromMeaning(%q) = %q, %v; want %q", meaning, got, err, want)
		}
	}
	for _, meaning := range []string{"corpus size in the load test", "environment of the load tests; measurement modes in ADR 0063", "targeting"} {
		if _, err := report.ComparisonFromMeaning(meaning); err == nil {
			t.Errorf("ComparisonFromMeaning(%q) read a parameter as a target", meaning)
		}
	}
}

// TestReport_SearchTargetsFailAboveThreshold (brief M3-16 "Tests"): one millisecond above the
// threshold fails the exit logic and marks the row failed, exactly at it passes, for both search
// targets; a passing value with fsync off does not end green.
func TestReport_SearchTargetsFailAboveThreshold(t *testing.T) {
	t.Parallel()
	targets, err := report.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"search.p95_target", "search.prefix_p95_target"} {
		th, err := targets.Threshold(name)
		if err != nil {
			t.Fatal(err)
		}
		with := func(base float64) report.File {
			f := green(report.ModeEmulated)
			for i, r := range f.Results {
				if r.Name == name {
					f.Results[i] = th.Result("perf/search", base, 2000)
				}
			}
			return f
		}
		var out bytes.Buffer
		above := with(th.Base + float64(time.Millisecond))
		if code := report.Exit(above, report.Required, &out); code != 1 || !strings.Contains(out.String(), "perf: not green: "+name) {
			t.Errorf("%s one millisecond above the threshold: exit %d\n%s", name, code, out.String())
		}
		if r := above.Results[slices.IndexFunc(above.Results, func(r report.Result) bool { return r.Name == name })]; r.Pass {
			t.Errorf("%s: row above the threshold marked passed: %+v", name, r)
		}
		out.Reset()
		if code := report.Exit(with(th.Base), report.Required, &out); code != 0 {
			t.Errorf("%s exactly at the threshold: exit %d\n%s", name, code, out.String())
		}
		f := with(th.Base)
		f.Profile.Fsync = "off"
		out.Reset()
		if code := report.Exit(f, report.Required, &out); code != 1 {
			t.Errorf("%s passing with fsync off: exit %d\n%s", name, code, out.String())
		}
	}
}
