// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// LocalTime is a wall time of day in whole minutes after local midnight, 0 (00:00) to 1439
// (23:59): the columns start_local and end_local of quiet_hours and the HH:MM fields of the
// schema QuietHoursRule.
type LocalTime int

const (
	minutesPerDay  = 24 * 60
	secondsPerMin  = 60
	localTimeWidth = len("HH:MM")
	allWeekdays    = 1<<7 - 1
)

// ParseLocalTime parses exactly the HH:MM pattern of QuietHoursRule, 00:00 to 23:59: two digits,
// a colon, two digits. Errors: an internal_error of the kernel for any other text (the caller
// validated the request against the API schema before, so a mismatch here is a broken row).
//
// Example:
//
//	t, _ := domain.ParseLocalTime("22:30") // t == 1350, t.String() == "22:30"
func ParseLocalTime(s string) (LocalTime, error) {
	if len(s) != localTimeWidth || s[2] != ':' {
		return 0, invalidLocalTime()
	}
	digits := [4]byte{s[0], s[1], s[3], s[4]}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, invalidLocalTime()
		}
	}
	h := int(digits[0]-'0')*10 + int(digits[1]-'0')
	m := int(digits[2]-'0')*10 + int(digits[3]-'0')
	if h > 23 || m > 59 {
		return 0, invalidLocalTime()
	}
	return LocalTime(h*60 + m), nil
}

func invalidLocalTime() error {
	return errs.New(errs.CodeInternalError, "notify decision: local time is not HH:MM from 00:00 to 23:59")
}

// String writes t back as HH:MM; a value outside 0 to 1439 writes "invalid".
func (t LocalTime) String() string {
	if !t.Valid() {
		return "invalid"
	}
	return fmt.Sprintf("%02d:%02d", int(t)/60, int(t)%60)
}

// Valid reports whether t is within 0 to 1439.
func (t LocalTime) Valid() bool { return t >= 0 && t < minutesPerDay }

// QuietRule is one row of quiet_hours (03, 13 3.4).
type QuietRule struct {
	// Weekdays is the bit mask of the row, bit 0 = Monday to bit 6 = Sunday, 1 to 127.
	Weekdays uint8
	// Start is start_local.
	Start LocalTime
	// End is end_local; End <= Start crosses midnight, End == Start is 24 hours from Start.
	End LocalTime
	// Location is the zone of the row, or users.time_zone when the row's time_zone is NULL. The
	// caller loads it; this package never loads a zone.
	Location *time.Location
	// Enabled is the column enabled; disabled rules never match.
	Enabled bool
}

// InQuietHours reports whether any enabled rule holds at now (13 3.4, D-M3-17-4). For each rule,
// now is converted into the rule's Location, which gives the weekday (Monday is bit 0) and the
// wall time in seconds after midnight. With Start < End the rule holds if the weekday's bit is set
// and Start <= time < End. With End <= Start the window crosses midnight and belongs to the day it
// starts on: the rule holds if the weekday's bit is set and time >= Start, or if the previous
// weekday's bit is set and time < End (Monday's previous day is Sunday). So Start == End is a
// window of 24 hours from Start on each set weekday, which is how screen D9 "ganztags" is stored
// (00:00 to 00:00).
//
// No boundary is ever computed as an instant: a rule inside the hour skipped by a spring
// changeover matches nothing that day, and one inside the hour repeated in autumn matches twice.
// No rule, or only disabled ones, means not quiet.
//
// Errors: an internal_error of the kernel and false for a zero now and for any rule, enabled or
// not, with Weekdays outside 1 to 127, a time outside 0 to 1439 or a nil Location.
func InQuietHours(rules []QuietRule, now time.Time) (bool, error) {
	if err := validateRules(rules, now); err != nil {
		return false, err
	}
	for _, r := range rules {
		if r.Enabled && r.holds(now) {
			return true, nil
		}
	}
	return false, nil
}

// holds applies points 2 and 3 of 13 3.4 to one valid rule.
func (r QuietRule) holds(now time.Time) bool {
	local := now.In(r.Location)
	day := (int(local.Weekday()) + 6) % 7 // Monday 0 to Sunday 6
	prev := (day + 6) % 7
	sec := local.Hour()*3600 + local.Minute()*secondsPerMin + local.Second()
	start, end := int(r.Start)*secondsPerMin, int(r.End)*secondsPerMin
	if r.Start < r.End {
		return r.day(day) && start <= sec && sec < end
	}
	return (r.day(day) && sec >= start) || (r.day(prev) && sec < end)
}

func (r QuietRule) day(d int) bool { return r.Weekdays&(1<<d) != 0 }

func validateRules(rules []QuietRule, now time.Time) error {
	if now.IsZero() {
		return invalid("now")
	}
	for _, r := range rules {
		switch {
		case r.Weekdays < 1 || r.Weekdays > allWeekdays:
			return invalid("quiet_hours.weekdays")
		case !r.Start.Valid():
			return invalid("quiet_hours.start_local")
		case !r.End.Valid():
			return invalid("quiet_hours.end_local")
		case r.Location == nil:
			return invalid("quiet_hours.time_zone")
		}
	}
	return nil
}
