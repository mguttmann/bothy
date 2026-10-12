// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// at parses an RFC 3339 instant of a test table.
func at(t testing.TB, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

// quietAt runs InQuietHours and fails the test on an error.
func quietAt(t *testing.T, rules []domain.QuietRule, now time.Time) bool {
	t.Helper()
	q, err := domain.InQuietHours(rules, now)
	if err != nil {
		t.Fatalf("InQuietHours at %s: %v", now.Format(time.RFC3339), err)
	}
	return q
}

type instantCase struct {
	at    string
	quiet bool
}

func checkInstants(t *testing.T, name string, rules []domain.QuietRule, cases []instantCase) {
	t.Helper()
	for _, c := range cases {
		if got := quietAt(t, rules, at(t, c.at)); got != c.quiet {
			t.Errorf("%s at %s: quiet %v, want %v", name, c.at, got, c.quiet)
		}
	}
}

// quietMinutes counts the quiet minutes of the 24 hours from start (inclusive).
func quietMinutes(t *testing.T, rules []domain.QuietRule, start time.Time) (int, map[string]bool) {
	t.Helper()
	n := 0
	seen := map[string]bool{}
	for i := range 24 * 60 {
		now := start.Add(time.Duration(i) * time.Minute)
		if quietAt(t, rules, now) {
			n++
			seen[now.Format(time.RFC3339)] = true
		}
	}
	return n, seen
}

// TestInQuietHours_EuropeBerlinChangeoverDays holds 13 3.4 on both changeover days of 2026.
func TestInQuietHours_EuropeBerlinChangeoverDays(t *testing.T) {
	t.Parallel()
	rules := []domain.QuietRule{nightRule(t)}
	checkInstants(t, "spring", rules, []instantCase{
		{"2026-03-28T20:59:59Z", false},
		{"2026-03-28T21:00:00Z", true},
		{"2026-03-29T00:59:59Z", true},
		{"2026-03-29T01:00:00Z", true},
		{"2026-03-29T04:59:59Z", true},
		{"2026-03-29T05:00:00Z", false},
	})
	checkInstants(t, "autumn", rules, []instantCase{
		{"2026-10-24T19:59:59Z", false},
		{"2026-10-24T20:00:00Z", true},
		{"2026-10-25T00:30:00Z", true},
		{"2026-10-25T01:30:00Z", true},
		{"2026-10-25T05:59:59Z", true},
		{"2026-10-25T06:00:00Z", false},
	})
	if n, _ := quietMinutes(t, rules, at(t, "2026-03-28T12:00:00Z")); n != 480 {
		t.Errorf("spring: %d quiet minutes, want 480", n)
	}
	if n, _ := quietMinutes(t, rules, at(t, "2026-10-24T12:00:00Z")); n != 600 {
		t.Errorf("autumn: %d quiet minutes, want 600", n)
	}
}

// TestInQuietHours_RuleInsideChangeoverHour: a rule in the skipped hour matches nothing, one in
// the repeated hour matches twice, because no boundary is computed as an instant.
func TestInQuietHours_RuleInsideChangeoverHour(t *testing.T) {
	t.Parallel()
	rules := []domain.QuietRule{{Weekdays: 64, Start: 2*60 + 15, End: 2*60 + 45, Location: berlin(t), Enabled: true}}
	if n, _ := quietMinutes(t, rules, at(t, "2026-03-29T00:00:00Z")); n != 0 {
		t.Errorf("spring: %d quiet minutes, want 0", n)
	}
	n, seen := quietMinutes(t, rules, at(t, "2026-10-25T00:00:00Z"))
	if n != 60 {
		t.Errorf("autumn: %d quiet minutes, want 60", n)
	}
	for _, s := range []string{"2026-10-25T00:30:00Z", "2026-10-25T01:30:00Z"} {
		if !seen[s] {
			t.Errorf("autumn: %s not quiet", s)
		}
	}
	checkInstants(t, "autumn", rules, []instantCase{
		{"2026-10-25T00:45:00Z", false},
		{"2026-10-25T01:14:59Z", false},
	})
}

// TestInQuietHours_AcrossMidnightAndWeekdayBits: a window belongs to the day it starts on.
func TestInQuietHours_AcrossMidnightAndWeekdayBits(t *testing.T) {
	t.Parallel()
	loc := berlin(t)
	friday := []domain.QuietRule{{Weekdays: 16, Start: 22 * 60, End: 7 * 60, Location: loc, Enabled: true}}
	checkInstants(t, "friday night", friday, []instantCase{
		{"2026-10-23T21:30:00Z", true},
		{"2026-10-24T04:30:00Z", true},
		{"2026-10-24T05:00:00Z", false},
		{"2026-10-24T21:00:00Z", false},
		{"2026-10-23T04:30:00Z", false},
	})
	sunday := []domain.QuietRule{{Weekdays: 64, Start: 22 * 60, End: 7 * 60, Location: loc, Enabled: true}}
	checkInstants(t, "sunday night", sunday, []instantCase{
		{"2026-10-26T04:30:00Z", true},
		{"2026-10-27T04:30:00Z", false},
	})
	lunch := []domain.QuietRule{{Weekdays: 31, Start: 12 * 60, End: 13 * 60, Location: loc, Enabled: true}}
	checkInstants(t, "weekday lunch", lunch, []instantCase{
		{"2026-10-26T11:00:00Z", true},
		{"2026-10-26T11:59:59Z", true},
		{"2026-10-26T12:00:00Z", false},
		{"2026-10-24T10:30:00Z", false},
	})
}

// TestInQuietHours_WholeDayZonesAndDisabled: start_local = end_local is 24 hours (screen D9
// "ganztags"), the zone of the rule counts, disabled rules and no rules are never quiet.
func TestInQuietHours_WholeDayZonesAndDisabled(t *testing.T) {
	t.Parallel()
	weekend := domain.QuietRule{Weekdays: 96, Start: 0, End: 0, Location: berlin(t), Enabled: true}
	checkInstants(t, "weekend", []domain.QuietRule{weekend}, []instantCase{
		{"2026-10-23T21:59:59Z", false},
		{"2026-10-23T22:00:00Z", true},
		{"2026-10-25T22:59:59Z", true},
		{"2026-10-25T23:00:00Z", false},
	})
	inUTC := weekend
	inUTC.Location = time.UTC
	probe := at(t, "2026-10-23T22:30:00Z")
	if quietAt(t, []domain.QuietRule{inUTC}, probe) == quietAt(t, []domain.QuietRule{weekend}, probe) {
		t.Error("the same rule in UTC and in Europe/Berlin agrees at 2026-10-23T22:30:00Z")
	}
	disabled := weekend
	disabled.Enabled = false
	if quietAt(t, []domain.QuietRule{disabled}, at(t, "2026-10-24T10:00:00Z")) {
		t.Error("a disabled matching rule made the person quiet")
	}
	if quietAt(t, nil, at(t, "2026-10-24T10:00:00Z")) {
		t.Error("no rules gave quiet")
	}
	// Any enabled matching rule is enough.
	if !quietAt(t, []domain.QuietRule{disabled, weekend}, at(t, "2026-10-24T10:00:00Z")) {
		t.Error("an enabled matching rule after a disabled one did not make the person quiet")
	}
}

// TestInQuietHours_InvalidInput covers the rule cases of design note 10.
func TestInQuietHours_InvalidInput(t *testing.T) {
	t.Parallel()
	good := nightRule(t)
	mutate := func(f func(r *domain.QuietRule)) []domain.QuietRule {
		r := good
		f(&r)
		return []domain.QuietRule{r}
	}
	cases := map[string]struct {
		rules []domain.QuietRule
		now   time.Time
	}{
		"zero now":       {[]domain.QuietRule{good}, time.Time{}},
		"weekdays zero":  {mutate(func(r *domain.QuietRule) { r.Weekdays = 0 }), weekdayNoon},
		"weekdays 128":   {mutate(func(r *domain.QuietRule) { r.Weekdays = 128 }), weekdayNoon},
		"start negative": {mutate(func(r *domain.QuietRule) { r.Start = -1 }), weekdayNoon},
		"start 1440":     {mutate(func(r *domain.QuietRule) { r.Start = 1440 }), weekdayNoon},
		"end 1440":       {mutate(func(r *domain.QuietRule) { r.End = 1440 }), weekdayNoon},
		"nil location":   {mutate(func(r *domain.QuietRule) { r.Location = nil }), weekdayNoon},
		"disabled but nil location": {mutate(func(r *domain.QuietRule) {
			r.Location = nil
			r.Enabled = false
		}), weekdayNoon},
	}
	for name, c := range cases {
		q, err := domain.InQuietHours(c.rules, c.now)
		assertInternal(t, name, err)
		if q {
			t.Errorf("%s: quiet true with an error", name)
		}
	}
}

// TestParseLocalTime accepts exactly HH:MM from 00:00 to 23:59.
func TestParseLocalTime(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"00:00", "07:00", "23:59", "12:34"} {
		v, err := domain.ParseLocalTime(s)
		if err != nil {
			t.Errorf("ParseLocalTime(%q): %v", s, err)
			continue
		}
		if v.String() != s {
			t.Errorf("ParseLocalTime(%q).String() = %q", s, v.String())
		}
	}
	if v, _ := domain.ParseLocalTime("07:30"); v != 7*60+30 {
		t.Errorf("07:30 = %d minutes, want 450", v)
	}
	for _, s := range []string{"24:00", "7:00", "07:60", "07:00:00", "", "0a:00", "07-00", " 7:00", "+7:00", "-1:00"} {
		if _, err := domain.ParseLocalTime(s); err == nil {
			t.Errorf("ParseLocalTime(%q) passed", s)
		} else {
			assertInternal(t, "ParseLocalTime "+s, err)
		}
	}
}
