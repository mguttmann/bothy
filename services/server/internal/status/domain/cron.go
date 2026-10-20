// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidSchedule reports a cron expression ParseSchedule rejects.
var ErrInvalidSchedule = errors.New("status: invalid cron schedule")

// nextHorizon bounds the search of Next in days. Every expression that matches at least once in
// four years matches within it (29 February included, except across a century year that is not a
// leap year), and an expression such as "0 0 30 2 *", which never matches, stops quickly.
const nextHorizon = 5 * 366

// field is one of the five fields: its name for errors and its range of values.
type field struct {
	name     string
	min, max int
}

// The five fields in the order of the expression. Day of week allows 7 as a second Sunday.
var fields = [5]field{
	{"minute", 0, 59},
	{"hour", 0, 23},
	{"day of month", 1, 31},
	{"month", 1, 12},
	{"day of week", 0, 7},
}

// Schedule is a parsed five-field cron expression (minute, hour, day of month, month, day of
// week), evaluated in UTC at minute resolution. The zero value matches nothing.
type Schedule struct {
	// sets holds one bit per allowed value of each field.
	sets [5]uint64
	// domStar and dowStar record an unrestricted day of month or day of week ("*" or "*/1"):
	// only when both are restricted does a day match if either matches (the rule of cron).
	domStar, dowStar bool
	valid            bool
}

// ParseSchedule parses a five-field cron expression of the keys backup.* (brief M7-02 design
// note 1): each field is "*", a number, a range "a-b", a step "*/n" or "a-b/n", or a list of
// these separated by commas. Minute 0-59, hour 0-23, day of month 1-31, month 1-12, day of week
// 0-7 (0 and 7 are Sunday). Names, macros such as "@daily" and the form "a/n" are not accepted.
// When both day of month and day of week are restricted, a day matches if either matches.
//
// Errors wrap ErrInvalidSchedule and name the field, never the expression, so the message can be
// logged next to the configuration key.
//
// Example:
//
//	s, err := domain.ParseSchedule("0 2 * * 1-6") // 02:00 UTC Monday to Saturday
func ParseSchedule(expr string) (Schedule, error) {
	parts := strings.Fields(expr)
	if len(parts) != len(fields) {
		return Schedule{}, fmt.Errorf("%w: want five fields (minute hour day-of-month month day-of-week)", ErrInvalidSchedule)
	}
	var s Schedule
	for i, p := range parts {
		set, star, ok := parseField(p, fields[i])
		if !ok {
			return Schedule{}, fmt.Errorf("%w: the field %s is not a number, range, step or list in %d-%d",
				ErrInvalidSchedule, fields[i].name, fields[i].min, fields[i].max)
		}
		s.sets[i] = set
		switch i {
		case 2:
			s.domStar = star
		case 4:
			s.dowStar = star
			// 7 is Sunday as well as 0.
			if set&(1<<7) != 0 {
				s.sets[i] |= 1
			}
		}
	}
	s.valid = true
	return s, nil
}

// parseField parses one field into its bit set. star reports "*" or "*/1", the unrestricted
// field the day rule needs.
func parseField(p string, f field) (set uint64, star, ok bool) {
	for item := range strings.SplitSeq(p, ",") {
		lo, hi, step, itemStar, okItem := parseItem(item, f)
		if !okItem {
			return 0, false, false
		}
		star = star || itemStar
		for v := lo; v <= hi; v += step {
			set |= 1 << uint(v)
		}
	}
	return set, star, true
}

// parseItem parses one item of a list: "*", "n", "a-b", "*/n" or "a-b/n". It returns the bounds,
// the step and whether the item is the unrestricted "*" or "*/1".
func parseItem(item string, f field) (lo, hi, step int, star, ok bool) {
	rng, stepText, hasStep := strings.Cut(item, "/")
	lo, hi, step = f.min, f.max, 1
	switch {
	case rng == "*":
	case strings.Contains(rng, "-"):
		a, b, _ := strings.Cut(rng, "-")
		var okA, okB bool
		lo, okA = number(a, f)
		hi, okB = number(b, f)
		if !okA || !okB || lo > hi {
			return 0, 0, 0, false, false
		}
	default:
		n, okN := number(rng, f)
		if !okN || hasStep {
			// "a/n" is not accepted: a step needs "*" or a range.
			return 0, 0, 0, false, false
		}
		lo, hi = n, n
	}
	if hasStep {
		n, err := strconv.Atoi(stepText)
		if err != nil || n < 1 || n > f.max || !digits(stepText) {
			return 0, 0, 0, false, false
		}
		step = n
	}
	return lo, hi, step, rng == "*" && step == 1, true
}

// number parses a decimal value of field f; signs, spaces and names are rejected.
func number(s string, f field) (int, bool) {
	if !digits(s) {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < f.min || n > f.max {
		return 0, false
	}
	return n, true
}

// digits reports a non-empty string of ASCII digits of at most two characters (every field value
// has at most two digits; the limit keeps Atoi far from overflow).
func digits(s string) bool {
	if s == "" || len(s) > 2 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// has reports whether value v is in field i.
func (s Schedule) has(i, v int) bool { return s.sets[i]&(1<<uint(v)) != 0 }

// dayMatches applies the day rule of cron to the date of t (UTC).
func (s Schedule) dayMatches(t time.Time) bool {
	if !s.has(3, int(t.Month())) {
		return false
	}
	dom := s.has(2, t.Day())
	dow := s.has(4, int(t.Weekday()))
	switch {
	case s.domStar && s.dowStar:
		return true
	case s.domStar:
		return dow
	case s.dowStar:
		return dom
	default:
		return dom || dow
	}
}

// Matches reports whether the minute of t, converted to UTC, matches s. Seconds are ignored.
func (s Schedule) Matches(t time.Time) bool {
	if !s.valid {
		return false
	}
	t = t.UTC()
	return s.dayMatches(t) && s.has(1, t.Hour()) && s.has(0, t.Minute())
}

// Next returns the first matching minute strictly after after, in UTC, or the zero time when
// none comes within about five years (an expression such as "0 0 30 2 *" never matches).
//
// Example:
//
//	s, _ := domain.ParseSchedule("0 2 * * 0")
//	s.Next(time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC)) // 2026-10-25 02:00 UTC
func (s Schedule) Next(after time.Time) time.Time {
	if !s.valid {
		return time.Time{}
	}
	start := after.UTC().Truncate(time.Minute).Add(time.Minute)
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	for range nextHorizon {
		if s.dayMatches(day) {
			if t, ok := s.firstMinute(day, start); ok {
				return t
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return time.Time{}
}

// firstMinute returns the first matching minute of day (midnight UTC) that is not before start.
func (s Schedule) firstMinute(day, start time.Time) (time.Time, bool) {
	for h := range 24 {
		if !s.has(1, h) {
			continue
		}
		for m := range 60 {
			t := day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
			if s.has(0, m) && !t.Before(start) {
				return t, true
			}
		}
	}
	return time.Time{}, false
}
