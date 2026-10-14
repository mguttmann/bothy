// SPDX-License-Identifier: Apache-2.0

// Package report reads the load test thresholds of make perf from the specification and compares
// measurements with them (docs/spec/02-architecture.md "Performance targets", ADR 0063). The values
// perf.* have the place Load test in the table "Configuration values of this chapter" of chapter 02
// and are never part of the server configuration schema, so the tests read them from that table
// instead of a literal. M1-72 extends the package with the modes and results.json of ADR 0063.
//
// Example:
//
//	targets, err := report.Load(report.ArchitectureChapter())
//	min, err := targets.Minimum("perf.event_commit_min_per_s")
//	err = report.CheckMinimum("perf.event_commit_min_per_s", measured, min)
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

// sectionTitle is the title of the table section in chapter 02.
const sectionTitle = "Configuration values of this chapter"

// placeLoadTest is the place of the load test values in that table.
const placeLoadTest = "Load test"

// Targets are the load test values of one chapter, by name, as written in the table.
type Targets struct {
	values map[string]string
}

// ArchitectureChapter returns the path of docs/spec/02-architecture.md in this repository.
func ArchitectureChapter() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "docs", "spec", "02-architecture.md")
}

// Load reads the rows with place Load test from the section whose heading ends in
// "Configuration values of this chapter" up to the next heading of the same or a higher level.
// The error reports a file that cannot be read or a chapter without that section.
func Load(path string) (Targets, error) {
	f, err := os.Open(path) // #nosec G304 -- the path of a chapter of this repository
	if err != nil {
		return Targets{}, fmt.Errorf("report: %w", err)
	}
	defer func() { _ = f.Close() }()
	values := map[string]string{}
	level := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if h := headingLevel(line); h > 0 {
			switch {
			case strings.HasSuffix(strings.TrimSpace(line), sectionTitle):
				level = h
			case level > 0 && h <= level:
				return Targets{values: values}, nil
			}
			continue
		}
		if level == 0 {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) < 3 || strings.TrimSpace(cells[1]) != placeLoadTest {
			continue
		}
		values[unquote(cells[0])] = unquote(cells[2])
	}
	if err := sc.Err(); err != nil {
		return Targets{}, fmt.Errorf("report: %w", err)
	}
	if level == 0 {
		return Targets{}, fmt.Errorf("report: %s has no section %q", path, sectionTitle)
	}
	return Targets{values: values}, nil
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
