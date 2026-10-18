// SPDX-License-Identifier: Apache-2.0

package report

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Comparison is how a measured value meets its threshold.
type Comparison string

// The comparisons of the meaning column: "target (minimum)" is AtLeast, "target (maximum)" and a
// plain "target" are AtMost.
const (
	AtLeast Comparison = ">="
	AtMost  Comparison = "<="
)

// ComparisonFromMeaning returns the comparison of a meaning cell of the 02 table. The error reports a meaning that is not a target (a load parameter or the profile).
func ComparisonFromMeaning(meaning string) (Comparison, error) {
	switch strings.ToLower(strings.TrimSpace(meaning)) {
	case "target (minimum)":
		return AtLeast, nil
	case "target (maximum)", "target":
		return AtMost, nil
	}
	return "", fmt.Errorf("report: meaning %q is not a target", meaning)
}

// Quantity is a value of the 02 table with its unit. Base is the value in the base unit of its
// kind: nanoseconds for durations, bytes for sizes, the number itself for counts and rates.
type Quantity struct {
	Value float64 // as written
	Unit  string  // us, ms, s, KiB, MiB, GiB, or "" for counts
	Base  float64
}

// unitFactors maps each unit to its factor towards the base unit.
var unitFactors = map[string]float64{
	"us": float64(time.Microsecond), "ms": float64(time.Millisecond), "s": float64(time.Second),
	"KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30, "": 1,
}

// quantityPattern is a non-negative decimal number with an optional unit suffix.
var quantityPattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)([A-Za-z]*)$`)

// ParseQuantity parses a value of the 02 table: a number with one of the units us, ms, s, KiB,
// MiB, GiB, or a plain number. The error reports anything else.
func ParseQuantity(text string) (Quantity, error) {
	m := quantityPattern.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return Quantity{}, fmt.Errorf("report: %q is not a number with a unit", text)
	}
	factor, ok := unitFactors[m[2]]
	if !ok {
		return Quantity{}, fmt.Errorf("report: %q has an unknown unit %q", text, m[2])
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return Quantity{}, fmt.Errorf("report: %q: %w", text, err)
	}
	return Quantity{Value: v, Unit: m[2], Base: v * factor}, nil
}

// FromBase converts a value in the base unit into the unit of q.
func (q Quantity) FromBase(base float64) float64 { return base / unitFactors[q.Unit] }

// Threshold is one target of the 02 table.
type Threshold struct {
	Name string
	Quantity
	Comparison Comparison
}

// Threshold returns the target name with its unit and comparison. The error reports a missing
// row, a value that does not parse, or a meaning that is not a target.
func (t Targets) Threshold(name string) (Threshold, error) {
	text, err := t.Text(name)
	if err != nil {
		return Threshold{}, err
	}
	q, err := ParseQuantity(text)
	if err != nil {
		return Threshold{}, fmt.Errorf("%s: %w", name, err)
	}
	cmp, err := ComparisonFromMeaning(t.meanings[name])
	if err != nil {
		return Threshold{}, fmt.Errorf("%s: %w", name, err)
	}
	return Threshold{Name: name, Quantity: q, Comparison: cmp}, nil
}

// Count returns a plain number of the table (a load parameter such as
// perf.sustained_messages_per_s). The error reports a missing row or a value with a unit.
func (t Targets) Count(name string) (int, error) {
	text, err := t.Text(name)
	if err != nil {
		return 0, err
	}
	q, err := ParseQuantity(text)
	if err != nil || q.Unit != "" || q.Value != math.Trunc(q.Value) {
		return 0, fmt.Errorf("report: %s = %q is not a plain integer", name, text)
	}
	return int(q.Value), nil
}

// Pass reports whether a measured value in the base unit meets the threshold.
func (th Threshold) Pass(base float64) bool {
	if th.Comparison == AtLeast {
		return base >= th.Base
	}
	return base <= th.Base
}

// Result returns the result of test for a measured value in the base unit over samples samples,
// with value and threshold in the unit of the threshold.
func (th Threshold) Result(test string, base float64, samples int) Result {
	return Result{
		Name: th.Name, Value: th.FromBase(base), Unit: th.Unit, Threshold: th.Value,
		Comparison: th.Comparison, Pass: th.Pass(base), Samples: samples, Test: test,
	}
}

// NearestRank returns the p-th percentile (0 < p <= 100) of samples by the nearest-rank method:
// the smallest sample with at least p percent of the samples at or below it. It does not change
// samples. The error reports no samples or p outside (0, 100].
func NearestRank(samples []float64, p float64) (float64, error) {
	if len(samples) == 0 {
		return 0, errors.New("report: no samples")
	}
	if p <= 0 || p > 100 {
		return 0, fmt.Errorf("report: percentile %v outside (0, 100]", p)
	}
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	return sorted[max(rank, 1)-1], nil
}

// DurationsP returns the p-th nearest-rank percentile of durations in nanoseconds.
func DurationsP(durations []time.Duration, p float64) (float64, error) {
	xs := make([]float64, len(durations))
	for i, d := range durations {
		xs[i] = float64(d)
	}
	return NearestRank(xs, p)
}
