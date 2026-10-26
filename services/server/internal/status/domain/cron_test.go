// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/status/domain"
)

// at returns the UTC instant of the date and minute.
func at(year int, month time.Month, day, hour, minute int) time.Time {
	return time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
}

// mustSchedule parses expr or fails the test.
func mustSchedule(t *testing.T, expr string) domain.Schedule {
	t.Helper()
	s, err := domain.ParseSchedule(expr)
	if err != nil {
		t.Fatalf("ParseSchedule(%q): %v", expr, err)
	}
	return s
}

// TestCronDue holds the five-field parser, Matches, Next and DueKinds (docs/spec/16-operations.md
// 8.1, brief M7-02 design note 1): due times of the default schedules, the day rule of cron
// (day of month or day of week when both are restricted), a full and a differential due at the
// same minute or in one window run only the full, and nothing before the window is caught up.
func TestCronDue(t *testing.T) {
	t.Parallel()

	// 2026-10-18 is a Sunday.
	sunday := at(2026, 10, 18, 2, 0)
	if sunday.Weekday() != time.Sunday {
		t.Fatalf("fixture: %v is a %v", sunday, sunday.Weekday())
	}

	matches := []struct {
		expr string
		t    time.Time
		want bool
	}{
		{"0 2 * * 0", sunday, true},
		{"0 2 * * 7", sunday, true},
		{"0 2 * * 0", sunday.Add(time.Minute), false},
		{"0 2 * * 0", sunday.Add(24 * time.Hour), false},
		{"0 2 * * 1-6", sunday.Add(24 * time.Hour), true},
		{"0 2 * * 1-6", sunday, false},
		{"*/15 * * * *", at(2026, 10, 18, 7, 45), true},
		{"*/15 * * * *", at(2026, 10, 18, 7, 46), false},
		{"10-20/5 * * * *", at(2026, 10, 18, 7, 15), true},
		{"10-20/5 * * * *", at(2026, 10, 18, 7, 25), false},
		{"0 0,12 * * *", at(2026, 10, 18, 12, 0), true},
		{"0 0,12 * * *", at(2026, 10, 18, 6, 0), false},
		{"30 3 1 1 *", at(2027, 1, 1, 3, 30), true},
		{"30 3 1 2 *", at(2027, 1, 1, 3, 30), false},
		// Both day fields restricted: the 1st of the month or any Monday.
		{"0 0 1 * 1", at(2026, 10, 1, 0, 0), true},  // a Thursday, the 1st
		{"0 0 1 * 1", at(2026, 10, 19, 0, 0), true}, // a Monday
		{"0 0 1 * 1", at(2026, 10, 20, 0, 0), false},
		// Only the day of week restricted: the day of month does not widen it.
		{"0 0 * * 1", at(2026, 10, 1, 0, 0), false},
		// Seconds and the zone do not matter: the minute in UTC does.
		{"0 2 * * 0", sunday.Add(59 * time.Second), true},
		{"0 2 * * 0", time.Date(2026, 10, 18, 4, 0, 0, 0, time.FixedZone("plus2", 2*3600)), true},
	}
	for _, m := range matches {
		if got := mustSchedule(t, m.expr).Matches(m.t); got != m.want {
			t.Errorf("%q.Matches(%v) = %v, want %v", m.expr, m.t, got, m.want)
		}
	}

	nexts := []struct {
		expr  string
		after time.Time
		want  time.Time
	}{
		{"0 2 * * 0", sunday, at(2026, 10, 25, 2, 0)},
		{"0 2 * * 0", sunday.Add(-time.Second), sunday},
		{"0 2 * * 1-6", sunday, at(2026, 10, 19, 2, 0)},
		{"30 3 * * *", at(2026, 12, 31, 3, 30), at(2027, 1, 1, 3, 30)},
		{"0 0 29 2 *", at(2026, 10, 18, 0, 0), at(2028, 2, 29, 0, 0)},
		{"* * * * *", at(2026, 10, 18, 7, 0).Add(30 * time.Second), at(2026, 10, 18, 7, 1)},
		// A day that never exists has no next time.
		{"0 0 30 2 *", sunday, time.Time{}},
		{"0 0 31 4 *", sunday, time.Time{}},
	}
	for _, n := range nexts {
		if got := mustSchedule(t, n.expr).Next(n.after); !got.Equal(n.want) {
			t.Errorf("%q.Next(%v) = %v, want %v", n.expr, n.after, got, n.want)
		}
	}

	full := mustSchedule(t, "0 2 * * 0")
	diff := mustSchedule(t, "0 2 * * 1-6")
	all := mustSchedule(t, "0 2 * * *")
	logical := mustSchedule(t, "0 3 * * *")
	objects := mustSchedule(t, "30 3 * * *")
	defaults := map[domain.Kind]domain.Schedule{
		domain.KindPGFull: full, domain.KindPGDiff: diff, domain.KindLogical: logical, domain.KindObjects: objects,
	}
	due := []struct {
		name      string
		schedules map[domain.Kind]domain.Schedule
		from, to  time.Time
		want      []domain.Kind
	}{
		{"sunday full", defaults, sunday.Add(-time.Minute), sunday, []domain.Kind{domain.KindPGFull}},
		{"monday diff", defaults, sunday.Add(24*time.Hour - time.Minute), sunday.Add(24 * time.Hour), []domain.Kind{domain.KindPGDiff}},
		{"the window excludes from", defaults, sunday, sunday.Add(time.Minute), nil},
		{"a quiet minute", defaults, at(2026, 10, 18, 9, 0), at(2026, 10, 18, 9, 1), nil},
		{"logical and objects in one window", defaults, at(2026, 10, 18, 2, 59), at(2026, 10, 18, 3, 30),
			[]domain.Kind{domain.KindLogical, domain.KindObjects}},
		{"full and diff at the same minute", map[domain.Kind]domain.Schedule{domain.KindPGFull: all, domain.KindPGDiff: all},
			sunday.Add(-time.Minute), sunday.Add(24 * time.Hour), []domain.Kind{domain.KindPGFull}},
		{"full and diff in one window", defaults, at(2026, 10, 17, 12, 0), at(2026, 10, 18, 12, 0),
			[]domain.Kind{domain.KindPGFull, domain.KindLogical, domain.KindObjects}},
		{"diff alone in a window", defaults, at(2026, 10, 18, 12, 0), at(2026, 10, 19, 12, 0),
			[]domain.Kind{domain.KindPGDiff, domain.KindLogical, domain.KindObjects}},
		{"an empty window", defaults, sunday, sunday, nil},
		{"a reversed window", defaults, sunday, sunday.Add(-time.Hour), nil},
		{"a kind without schedule", map[domain.Kind]domain.Schedule{domain.KindLogical: logical},
			at(2026, 10, 18, 0, 0), at(2026, 10, 19, 0, 0), []domain.Kind{domain.KindLogical}},
	}
	for _, d := range due {
		if got := domain.DueKinds(d.schedules, d.from, d.to); !slices.Equal(got, d.want) {
			t.Errorf("%s: DueKinds = %v, want %v", d.name, got, d.want)
		}
	}
}

// TestParseScheduleRejects holds the errors of ParseSchedule: they wrap ErrInvalidSchedule, name
// the field and never echo the expression.
func TestParseScheduleRejects(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ expr, field string }{
		{"", "five fields"},
		{"0 2 * *", "five fields"},
		{"0 2 * * * *", "five fields"},
		{"60 * * * *", "minute"},
		{"-1 * * * *", "minute"},
		{"* 24 * * *", "hour"},
		{"* * 0 * *", "day of month"},
		{"* * 32 * *", "day of month"},
		{"* * * 0 *", "month"},
		{"* * * 13 *", "month"},
		{"* * * * 8", "day of week"},
		{"* * * JAN *", "month"},
		{"* * * * MON", "day of week"},
		{"@daily", "five fields"},
		{"5-1 * * * *", "minute"},
		{"*/0 * * * *", "minute"},
		{"*/x * * * *", "minute"},
		{"1,,2 * * * *", "minute"},
		{"1- * * * *", "minute"},
		{"/5 * * * *", "minute"},
		{"1/5/2 * * * *", "minute"},
		{"*/99 * * * *", "minute"},
		{"+1 * * * *", "minute"},
		{"1/5 * * * *", "minute"},
	} {
		_, err := domain.ParseSchedule(c.expr)
		if !errors.Is(err, domain.ErrInvalidSchedule) {
			t.Errorf("ParseSchedule(%q) = %v, want ErrInvalidSchedule", c.expr, err)
			continue
		}
		if !strings.Contains(err.Error(), c.field) {
			t.Errorf("ParseSchedule(%q) = %q, want the field %q named", c.expr, err, c.field)
		}
		if c.expr != "" && c.expr != "@daily" && strings.Contains(err.Error(), c.expr) {
			t.Errorf("ParseSchedule(%q) = %q echoes the expression", c.expr, err)
		}
	}
	for _, ok := range []string{"0 2 * * 0", "0 2 * * 1-6", " 30  3 * * * ", "0-59/1 0-23 1-31 1-12 0-7", "5,10-12,*/20 * * * *"} {
		if _, err := domain.ParseSchedule(ok); err != nil {
			t.Errorf("ParseSchedule(%q): %v", ok, err)
		}
	}
}

// TestScheduleLongestGap: the longest time between two runs, the longest month for a monthly
// schedule, 0 for a schedule that never matches.
func TestScheduleLongestGap(t *testing.T) {
	t.Parallel()
	for expr, want := range map[string]time.Duration{
		"0 2 * * 0":    7 * 24 * time.Hour,
		"0 2 * * 1-6":  48 * time.Hour,
		"30 3 * * *":   24 * time.Hour,
		"0 2 1 * *":    31 * 24 * time.Hour,
		"*/15 * * * *": 15 * time.Minute,
		"0 0 30 2 *":   0,
	} {
		s, err := domain.ParseSchedule(expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got := s.LongestGap(); got != want {
			t.Errorf("%s: LongestGap = %v, want %v", expr, got, want)
		}
	}
}
