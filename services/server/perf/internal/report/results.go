// SPDX-License-Identifier: Apache-2.0

package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"text/tabwriter"
)

// Required are the targets make perf measures: those of M1 (brief M1-72, requirement 9) and
// perf.push_handoff_p95 (M3-24); a results file without one of them does not end green.
var Required = []string{
	"perf.message_create_p99",
	"perf.event_commit_min_per_s",
	"perf.fanout_local_p99",
	"perf.catchup_page_p99",
	"perf.authorize_cached_p99",
	"perf.ws_connections_per_node",
	"perf.ws_idle_conn_mem_max",
	"perf.idle_rss_max",
	"perf.startup_ready_max",
	"perf.push_handoff_p95",
}

// Result is one measured target. Value and Threshold are in Unit, the unit of the threshold.
// A target measured by two tests (the catch-up variants) has one result per test.
type Result struct {
	Name       string     `json:"name"`
	Value      float64    `json:"value"`
	Unit       string     `json:"unit"`
	Threshold  float64    `json:"threshold"`
	Comparison Comparison `json:"comparison"`
	Pass       bool       `json:"pass"`
	Samples    int        `json:"samples"`
	Test       string     `json:"test"`
}

// File is the content of results.json: the profile, one result per target and test, and the
// supporting measurements (pool acquire wait, counter lock wait, database size, clock skew, and
// the named parameters of each test).
type File struct {
	Profile Profile        `json:"profile"`
	Results []Result       `json:"results"`
	Extra   map[string]any `json:"extra"`
}

// ResultsPath returns .cache/perf/results.json at the repository root.
func ResultsPath() string { return filepath.Join(RepoRoot(), ".cache", "perf", "results.json") }

// Read reads a results file. The error reports a missing or malformed file.
func Read(path string) (File, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- the results file of this repository or of a test
	if err != nil {
		return File{}, fmt.Errorf("report: %w", err)
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("report: %s: %w", path, err)
	}
	return f, nil
}

// Merge adds the results and extra entries of one test package to the file at path, creating it
// when it does not exist: a result replaces the one with the same name and test, an extra entry
// the one with the same key. The profile takes the host fields of profile; fsync and
// synchronous_commit keep any value other than "on" that a database reported, so one
// non-durable database ends the whole run not green. Packages run one after another (go test
// -p 1 in make perf), so there is no concurrent writer.
func Merge(path string, profile Profile, results []Result, extra map[string]any) error {
	f, err := Read(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err == nil {
		profile.Fsync = worst(f.Profile.Fsync, profile.Fsync)
		profile.SynchronousCommit = worst(f.Profile.SynchronousCommit, profile.SynchronousCommit)
		if profile.PostgresVersion == "" {
			profile.PostgresVersion = f.Profile.PostgresVersion
		}
	}
	f.Profile = profile
	for _, r := range results {
		f.Results = slices.DeleteFunc(f.Results, func(x Result) bool { return x.Name == r.Name && x.Test == r.Test })
		f.Results = append(f.Results, r)
	}
	if f.Extra == nil {
		f.Extra = map[string]any{}
	}
	maps.Copy(f.Extra, extra)
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	return os.Rename(tmp, path)
}

// worst returns the durability value that counts after two databases reported a and b: a value
// other than "on" wins, and an empty value means not read yet.
func worst(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "" || a != "on":
		return a
	default:
		return b
	}
}

// Verdict reports whether the file ends green and, if not, why: the mode must be match or
// emulated, fsync and synchronous_commit must be on, every required target must have a result,
// and every result must pass.
func (f File) Verdict(required []string) (bool, []string) {
	var problems []string
	if f.Profile.Mode != ModeMatch && f.Profile.Mode != ModeEmulated {
		problems = append(problems, fmt.Sprintf("profile mode %q: the values are not proven on perf.reference_profile", f.Profile.Mode))
	}
	if !f.Profile.Durable() {
		problems = append(problems, fmt.Sprintf("fsync = %q, synchronous_commit = %q: measured without durability (ADR 0063 point 2)",
			f.Profile.Fsync, f.Profile.SynchronousCommit))
	}
	for _, name := range required {
		if !slices.ContainsFunc(f.Results, func(r Result) bool { return r.Name == name }) {
			problems = append(problems, name+": no result")
		}
	}
	for _, r := range f.Results {
		if !r.Pass {
			problems = append(problems, fmt.Sprintf("%s (%s): %g %s, threshold %s %g %s", r.Name, r.Test, r.Value, r.Unit, r.Comparison, r.Threshold, r.Unit))
		}
	}
	return len(problems) == 0, problems
}

// WriteTable writes the profile, then one line per result with value, comparison, threshold and
// pass, then the extra entries.
func (f File) WriteTable(w io.Writer) {
	p := f.Profile
	_, _ = fmt.Fprintf(w, "profile mode: %s (cpus %d, memory %d bytes, %s, gomaxprocs %d, postgres %s, fsync %s, synchronous_commit %s)\n",
		p.Mode, p.CPUs, p.MemoryBytes, p.OS, p.GOMAXPROCS, p.PostgresVersion, p.Fsync, p.SynchronousCommit)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "target\tvalue\tthreshold\tpass\tsamples\ttest")
	rows := slices.Clone(f.Results)
	slices.SortStableFunc(rows, func(a, b Result) int {
		if a.Name != b.Name {
			if a.Name < b.Name {
				return -1
			}
			return 1
		}
		return 0
	})
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%.3g %s\t%s %g %s\t%t\t%d\t%s\n", r.Name, r.Value, r.Unit, r.Comparison, r.Threshold, r.Unit, r.Pass, r.Samples, r.Test)
	}
	_ = tw.Flush()
	for _, k := range slices.Sorted(maps.Keys(f.Extra)) {
		_, _ = fmt.Fprintf(w, "extra %s = %v\n", k, f.Extra[k])
	}
}

// Exit writes the table and the verdict of f to w and returns the exit code of make perf: 0 when
// f ends green against required, 1 otherwise.
func Exit(f File, required []string, w io.Writer) int {
	f.WriteTable(w)
	ok, problems := f.Verdict(required)
	if ok {
		_, _ = fmt.Fprintln(w, "perf: every target passed")
		return 0
	}
	for _, p := range problems {
		_, _ = fmt.Fprintln(w, "perf: not green:", p)
	}
	return 1
}
