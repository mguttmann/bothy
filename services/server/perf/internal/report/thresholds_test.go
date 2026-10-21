// SPDX-License-Identifier: Apache-2.0

package report_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/perf/internal/report"
)

// TestThresholdsFromChapterTable: every M1 target is found with a unit and a comparison, in the
// English fixture and in the real chapter 02 of this checkout.
func TestThresholdsFromChapterTable(t *testing.T) {
	t.Parallel()
	for _, path := range []string{filepath.Join("testdata", "chapter-en.md"), report.ArchitectureChapter()} {
		targets, err := report.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range report.Required {
			th, err := targets.Threshold(name)
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			if th.Base <= 0 || th.Name != name {
				t.Errorf("%s: %s = %+v", path, name, th)
			}
		}
		if _, err := targets.Threshold("perf.sustained_messages_per_s"); err == nil {
			t.Errorf("%s: a load parameter was read as a target", path)
		}
		if _, err := targets.Threshold("perf.outside"); err == nil {
			t.Errorf("%s: a row outside the section was read", path)
		}
		if n, err := targets.Count("perf.sustained_messages_per_s"); err != nil || n <= 0 {
			t.Errorf("%s: perf.sustained_messages_per_s = %v, %v", path, n, err)
		}
	}
}

// TestThresholdMissingFails: a missing row, an unparseable value and a meaning without a
// comparison each fail.
func TestThresholdMissingFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	body := "## Configuration values of this chapter\n\n| Name | Place | Default | Meaning |\n|---|---|---|---|\n" +
		"| `perf.a` | Load test | `12parsecs` | target |\n| `perf.b` | Load test | `5s` | environment |\n"
	p := filepath.Join(dir, "c.md")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	targets, err := report.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"perf.a", "perf.b", "perf.missing"} {
		if _, err := targets.Threshold(name); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("Threshold(%s) = %v, want an error naming it", name, err)
		}
	}
}

func TestUnitParsing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		text string
		unit string
		base float64
	}{
		{"100us", "us", 100 * float64(time.Microsecond)},
		{"150ms", "ms", 150 * float64(time.Millisecond)},
		{"5s", "s", 5 * float64(time.Second)},
		{"64KiB", "KiB", 64 << 10},
		{"256MiB", "MiB", 256 << 20},
		{"8GiB", "GiB", 8 << 30},
		{"5000", "", 5000},
		{"1000", "", 1000},
	}
	for _, c := range cases {
		q, err := report.ParseQuantity(c.text)
		if err != nil || q.Unit != c.unit || q.Base != c.base {
			t.Errorf("ParseQuantity(%q) = %+v, %v; want unit %q base %v", c.text, q, err, c.unit, c.base)
		}
	}
	for _, bad := range []string{"", "ms", "1.5.2s", "-3s", "12 parsecs", "64kb"} {
		if q, err := report.ParseQuantity(bad); err == nil {
			t.Errorf("ParseQuantity(%q) = %+v, want an error", bad, q)
		}
	}
	q, _ := report.ParseQuantity("150ms")
	if got := q.FromBase(float64(75 * time.Millisecond)); got != 75 {
		t.Errorf("FromBase = %v, want 75", got)
	}
}

func TestComparisonFromMeaning(t *testing.T) {
	t.Parallel()
	cases := map[string]report.Comparison{
		"target":             report.AtMost,
		"target (maximum)":   report.AtMost,
		"target (minimum)":   report.AtLeast,
		"Target":             report.AtMost,
		" Target (Maximum) ": report.AtMost,
		"Target (Minimum)":   report.AtLeast,
	}
	for meaning, want := range cases {
		got, err := report.ComparisonFromMeaning(meaning)
		if err != nil || got != want {
			t.Errorf("ComparisonFromMeaning(%q) = %q, %v; want %q", meaning, got, err, want)
		}
	}
	if _, err := report.ComparisonFromMeaning("sustained load"); err == nil {
		t.Error("a meaning without target was read as a comparison")
	}
	th := report.Threshold{Name: "perf.x", Quantity: report.Quantity{Base: 10}, Comparison: report.AtLeast}
	if !th.Pass(10) || th.Pass(9.9) {
		t.Error("AtLeast: value >= threshold passes")
	}
	th.Comparison = report.AtMost
	if !th.Pass(10) || th.Pass(10.1) {
		t.Error("AtMost: value <= threshold passes")
	}
}

func TestNearestRankPercentile(t *testing.T) {
	t.Parallel()
	samples := []float64{15, 20, 35, 40, 50}
	for p, want := range map[float64]float64{5: 15, 30: 20, 40: 20, 50: 35, 99: 50, 100: 50} {
		got, err := report.NearestRank(samples, p)
		if err != nil || got != want {
			t.Errorf("NearestRank(p%v) = %v, %v; want %v", p, got, err, want)
		}
	}
	hundred := make([]float64, 100)
	for i := range hundred {
		hundred[i] = float64(100 - i)
	}
	if got, _ := report.NearestRank(hundred, 99); got != 99 {
		t.Errorf("p99 of 1..100 = %v, want 99", got)
	}
	if !slices.Equal(samples, []float64{15, 20, 35, 40, 50}) {
		t.Error("NearestRank changed its input")
	}
	if _, err := report.NearestRank(nil, 99); err == nil {
		t.Error("no samples gave a percentile")
	}
	if _, err := report.NearestRank(samples, 0); err == nil {
		t.Error("p0 was accepted")
	}
}

func TestProfileModes(t *testing.T) {
	t.Parallel()
	ref, err := report.ParseReference("4 vCPU, 8 GiB RAM, SSD, PostgreSQL 18 local with fsync and synchronous_commit on, role all")
	if err != nil || ref.CPUs != 4 || ref.MemoryBytes != 8<<30 {
		t.Fatalf("ParseReference = %+v, %v", ref, err)
	}
	cases := []struct {
		cpus int
		mem  int64
		want report.Mode
	}{
		{4, 8 << 30, report.ModeMatch},
		{4, 16 << 30, report.ModeMatch},
		{12, 36 << 30, report.ModeEmulated},
		{8, 8 << 30, report.ModeEmulated},
		{2, 16 << 30, report.ModeBelow},
		{4, 4 << 30, report.ModeBelow},
		{16, 4 << 30, report.ModeBelow},
		// MemTotal of a Linux VM with 8 GiB is below 8 GiB (the kernel keeps its share), and
		// that host has the memory of the profile (M1-72/16).
		{4, 8152604 << 10, report.ModeMatch},
		{8, 8152604 << 10, report.ModeEmulated},
		{4, 6 << 30, report.ModeBelow},
	}
	for _, c := range cases {
		if got := report.ModeFor(ref, c.cpus, c.mem); got != c.want {
			t.Errorf("ModeFor(%d CPUs, %d bytes) = %s, want %s", c.cpus, c.mem, got, c.want)
		}
	}
	// The container runtime of the database counts as well: a large host whose Docker VM is
	// smaller than the profile cannot give the database the memory of the profile.
	if got := report.ModeWith(ref, 12, 36<<30, report.Runtime{CPUs: 4, MemoryBytes: 6 << 30}); got != report.ModeBelow {
		t.Errorf("runtime below the profile: %s, want below", got)
	}
	if got := report.ModeWith(ref, 12, 36<<30, report.Runtime{CPUs: 8, MemoryBytes: 16 << 30}); got != report.ModeEmulated {
		t.Errorf("runtime above the profile: %s, want emulated", got)
	}
	if got := report.ModeWith(ref, 12, 36<<30, report.Runtime{CPUs: 4, MemoryBytes: 8152604 << 10}); got != report.ModeEmulated {
		t.Errorf("runtime with the MemTotal of an 8 GiB VM: %s, want emulated", got)
	}
	if got := report.ModeWith(ref, 4, 8<<30, report.Runtime{}); got != report.ModeMatch {
		t.Errorf("unknown runtime: %s, want the host mode match", got)
	}
	if _, err := report.ParseReference("a fast machine"); err == nil {
		t.Error("a profile without vCPU and RAM was accepted")
	}
}

// green is a results file where every required target passes.
func green(mode report.Mode) report.File {
	f := report.File{
		Profile: report.Profile{Mode: mode, CPUs: 4, MemoryBytes: 8 << 30, OS: "linux", PostgresVersion: "18.0",
			GOMAXPROCS: 4, Fsync: "on", SynchronousCommit: "on"},
		Extra: map[string]any{},
	}
	for _, name := range report.Required {
		f.Results = append(f.Results, report.Result{Name: name, Value: 1, Threshold: 2, Comparison: report.AtMost, Pass: true, Samples: 10, Test: "TestX"})
	}
	return f
}

func TestProfileRejectsNonDurable(t *testing.T) {
	t.Parallel()
	if ok, problems := green(report.ModeEmulated).Verdict(report.Required); !ok {
		t.Fatalf("a green fixture failed: %v", problems)
	}
	f := green(report.ModeMatch)
	f.Profile.SynchronousCommit = "off"
	if ok, _ := f.Verdict(report.Required); ok {
		t.Error("synchronous_commit = off ended green")
	}
	f = green(report.ModeMatch)
	f.Profile.Fsync = "off"
	if ok, _ := f.Verdict(report.Required); ok {
		t.Error("fsync = off ended green")
	}
	f = green(report.ModeMatch)
	f.Profile.Fsync = ""
	if ok, _ := f.Verdict(report.Required); ok {
		t.Error("an unread fsync ended green")
	}
	if ok, _ := green(report.ModeBelow).Verdict(report.Required); ok {
		t.Error("mode below ended green")
	}
}

// TestExitFailsOnMissedTarget: the exit logic of make perf returns 1 for a failing fixture (one
// target missed, one missing) and prints the table with the mode first.
func TestExitFailsOnMissedTarget(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if code := report.Exit(green(report.ModeMatch), report.Required, &out); code != 0 {
		t.Fatalf("green fixture: exit %d\n%s", code, out.String())
	}
	if !strings.HasPrefix(out.String(), "profile mode: match") {
		t.Errorf("the table does not start with the mode:\n%s", out.String())
	}
	f := green(report.ModeMatch)
	f.Results[0].Pass = false
	out.Reset()
	if code := report.Exit(f, report.Required, &out); code != 1 || !strings.Contains(out.String(), f.Results[0].Name) {
		t.Errorf("missed target: exit %d\n%s", code, out.String())
	}
	f = green(report.ModeMatch)
	missing := f.Results[len(f.Results)-1].Name
	f.Results = f.Results[:len(f.Results)-1]
	out.Reset()
	if code := report.Exit(f, report.Required, &out); code != 1 || !strings.Contains(out.String(), missing) {
		t.Errorf("missing target: exit %d\n%s", code, out.String())
	}
}

func TestResultsJSONSchema(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "perf", "results.json")
	f := green(report.ModeEmulated)
	if err := report.Merge(path, f.Profile, f.Results[:2], map[string]any{"pool_acquire_wait_p99_ms": 1.5}); err != nil {
		t.Fatal(err)
	}
	second := f.Profile
	second.Fsync = "off"
	if err := report.Merge(path, second, f.Results[1:], map[string]any{"db_clock_skew_ms": 0.2}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path) // #nosec G304 -- a temp file of the test
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatal(err)
	}
	if keys := sortedKeys(top); !slices.Equal(keys, []string{"extra", "profile", "results"}) {
		t.Fatalf("top-level keys %v", keys)
	}
	var prof map[string]any
	if err := json.Unmarshal(top["profile"], &prof); err != nil {
		t.Fatal(err)
	}
	want := []string{"cpus", "fsync", "gomaxprocs", "memory_bytes", "mode", "os", "postgres_version", "synchronous_commit"}
	if keys := sortedKeys(prof); !slices.Equal(keys, want) {
		t.Errorf("profile keys %v, want %v", keys, want)
	}
	if prof["fsync"] != "off" {
		t.Errorf("a database with fsync off was not kept: %v", prof["fsync"])
	}
	var results []map[string]any
	if err := json.Unmarshal(top["results"], &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != len(f.Results) {
		t.Fatalf("%d results after the merge, want %d (one per name)", len(results), len(f.Results))
	}
	want = []string{"comparison", "name", "pass", "samples", "test", "threshold", "unit", "value"}
	if keys := sortedKeys(results[0]); !slices.Equal(keys, want) {
		t.Errorf("result keys %v, want %v", keys, want)
	}
	var extra map[string]any
	_ = json.Unmarshal(top["extra"], &extra)
	if len(extra) != 2 {
		t.Errorf("extra %v, want both entries", extra)
	}
	read, err := report.Read(path)
	if err != nil || read.Profile.Fsync != "off" {
		t.Fatalf("Read = %+v, %v", read.Profile, err)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// TestClockSkewCorrection: the offset comes from the sample with the shortest round trip, and a
// latency is corrected by it.
func TestClockSkewCorrection(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	samples := []report.ClockSample{
		{Before: base, DB: base.Add(60 * time.Millisecond), After: base.Add(40 * time.Millisecond)},                                         // rtt 40ms, offset 40ms
		{Before: base.Add(time.Second), DB: base.Add(time.Second + 52*time.Millisecond), After: base.Add(time.Second + 4*time.Millisecond)}, // rtt 4ms, offset 50ms
	}
	off, err := report.ClockOffset(samples)
	if err != nil || off != 50*time.Millisecond {
		t.Fatalf("ClockOffset = %v, %v; want 50ms", off, err)
	}
	// The database clock runs 50ms ahead: an event it stamped at 12:00:00.150 happened at host time
	// 12:00:00.100, and a receive at host time 12:00:00.180 is 80ms later.
	occurred := base.Add(150 * time.Millisecond)
	received := base.Add(180 * time.Millisecond)
	if got := report.CorrectedLatency(received, occurred, off); got != 80*time.Millisecond {
		t.Errorf("CorrectedLatency = %v, want 80ms", got)
	}
	if _, err := report.ClockOffset(nil); err == nil {
		t.Error("no samples gave an offset")
	}
}
