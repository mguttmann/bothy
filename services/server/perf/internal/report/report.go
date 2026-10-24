// SPDX-License-Identifier: Apache-2.0

// Package report reads the load test thresholds of make perf from the specification, compares
// measurements with them and writes the results file of ADR 0063 (chapter 02 "Performance
// targets"). The values perf.* of chapter 02 and the search targets of chapter 14
// (search.p95_target, search.prefix_p95_target, search.benchmark_messages; brief M3-16) have the
// place "Load test" in the tables "Configuration values of this chapter" and are never part of the
// server configuration schema, so the tests read them from those tables instead of a literal
// (LoadAll).
//
// The package also holds units and nearest-rank percentiles, the profile mode of the host
// (match, emulated, below), the clock offset between database and host, and the results file
// .cache/perf/results.json with its table and exit logic (Exit), which make perf runs last.
//
// Example:
//
//	targets, err := report.LoadAll()
//	th, err := targets.Threshold("perf.message_create_p99")
//	res := th.Result("TestPerf_MessageCreateP99", float64(p99), len(samples))
//	err = report.Merge(report.ResultsPath(), profile, []report.Result{res}, nil)
package report

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// sectionTitle is the title of the table section of a chapter.
const sectionTitle = "Configuration values of this chapter"

// placeLoadTest is the place of the load test values in that table.
const placeLoadTest = "Load test"

// Targets are the load test values of one chapter, by name, as written in the table, with the
// meaning column of each row.
type Targets struct {
	values   map[string]string
	meanings map[string]string
}

// ArchitectureChapter returns the path of docs/spec/02-architecture.md in this repository.
func ArchitectureChapter() string {
	return filepath.Join(RepoRoot(), "docs", "spec", "02-architecture.md")
}

// SearchChapter returns the path of docs/spec/14-search.md in this repository, whose table holds
// the targets of the search load test (brief M3-16).
func SearchChapter() string {
	return filepath.Join(RepoRoot(), "docs", "spec", "14-search.md")
}

// LoadAll reads the load test values of every chapter that has them: 02 and 14
// (LoadChapters(ArchitectureChapter(), SearchChapter())).
func LoadAll() (Targets, error) {
	return LoadChapters(ArchitectureChapter(), SearchChapter())
}

// LoadChapters reads the rows with place "Load test" of the section "Configuration values of this
// chapter" of every chapter in paths (Load) into one set. The error reports a chapter that Load
// rejects and a name with place "Load test" in two chapters, so no chapter silently overrides
// another.
func LoadChapters(paths ...string) (Targets, error) {
	all := Targets{values: map[string]string{}, meanings: map[string]string{}}
	from := map[string]string{}
	for _, path := range paths {
		t, err := Load(path)
		if err != nil {
			return Targets{}, err
		}
		for name, v := range t.values {
			if first, ok := from[name]; ok {
				return Targets{}, fmt.Errorf("report: %s has place %s in %s and in %s", name, placeLoadTest, filepath.Base(first), filepath.Base(path))
			}
			from[name] = path
			all.values[name] = v
			if m, ok := t.meanings[name]; ok {
				all.meanings[name] = m
			}
		}
	}
	return all, nil
}

// RepoRoot returns the root of the repository this package is compiled from.
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..")
}

// Load reads the rows with place "Load test" from the section whose heading ends in
// "Configuration values of this chapter" up to the next heading of the same or a higher level.
// The error reports a file that cannot be read or a chapter without that section.
func Load(path string) (Targets, error) {
	f, err := os.Open(path) // #nosec G304 -- the path of a chapter of this repository
	if err != nil {
		return Targets{}, fmt.Errorf("report: %w", err)
	}
	defer func() { _ = f.Close() }()
	t := Targets{values: map[string]string{}, meanings: map[string]string{}}
	level := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if h := headingLevel(line); h > 0 {
			switch {
			case level == 0 && strings.HasSuffix(strings.TrimSpace(line), sectionTitle):
				level = h
			case level > 0 && h <= level:
				return t, nil
			}
			continue
		}
		if level > 0 {
			t.addRow(line)
		}
	}
	if err := sc.Err(); err != nil {
		return Targets{}, fmt.Errorf("report: %w", err)
	}
	if level == 0 {
		return Targets{}, fmt.Errorf("report: %s has no section %q", path, sectionTitle)
	}
	return t, nil
}

// addRow keeps a table row of the section whose place is "Load test": name, value and meaning.
func (t Targets) addRow(line string) {
	cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	if len(cells) < 3 || strings.TrimSpace(cells[1]) != placeLoadTest {
		return
	}
	name := unquote(cells[0])
	t.values[name] = unquote(cells[2])
	if len(cells) > 3 {
		t.meanings[name] = strings.TrimSpace(cells[3])
	}
}

// headingLevel returns the number of leading '#' of a Markdown heading, 0 for other lines.
func headingLevel(line string) int {
	n := len(line) - len(strings.TrimLeft(line, "#"))
	if n == 0 || n == len(line) || line[n] != ' ' {
		return 0
	}
	return n
}

// unquote trims a table cell and its backticks.
func unquote(cell string) string {
	return strings.Trim(strings.TrimSpace(cell), "`")
}

// Text returns the value of name as written; the error reports a missing row.
func (t Targets) Text(name string) (string, error) {
	v, ok := t.values[name]
	if !ok {
		return "", fmt.Errorf("report: %s has no row with place %s", name, placeLoadTest)
	}
	return v, nil
}

// Minimum returns the value of name as a number; the error reports a missing row or a value that
// is not a plain number.
func (t Targets) Minimum(name string) (float64, error) {
	text, err := t.Text(name)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("report: %s = %q is not a number", name, text)
	}
	return v, nil
}

// errBelow is the error of a measurement below its threshold.
var errBelow = errors.New("below the threshold")

// CheckMinimum returns an error naming the target when measured is below threshold; at or above
// it passes.
func CheckMinimum(name string, measured, threshold float64) error {
	if measured < threshold {
		return fmt.Errorf("report: %s measured %.1f, %w %.1f", name, measured, errBelow, threshold)
	}
	return nil
}
