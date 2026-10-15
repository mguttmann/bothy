// SPDX-License-Identifier: Apache-2.0

package domain

import "github.com/OWNER/bothy/services/server/internal/kernel/errs"

// The errors[].code values of a refused quiet-hour rule (06 section 5 point 4).
const (
	// FieldOutOfRange marks weekdays outside 1 to 127.
	FieldOutOfRange = "out_of_range"
	// FieldInvalidFormat marks a local time that is not HH:MM from 00:00 to 23:59.
	FieldInvalidFormat = "invalid_format"
	// FieldUnknownTimeZone marks a time_zone that is not an IANA zone name (13 section 3.4).
	FieldUnknownTimeZone = "unknown_time_zone"
	// FieldLimitReached marks a list of rules whose quiet_hours.updated payload would exceed
	// sync.event_payload_max_bytes.
	FieldLimitReached = "limit_reached"
)

// localZoneName is the name time.LoadLocation maps to the zone of the host; a rule never uses it
// (the server's zone says nothing about the person).
const localZoneName = "Local"

// RuleInput is the body of createQuietHoursRule and putQuietHoursRule (schema
// QuietHoursRuleInput): Weekdays is the bit mask (bit 0 = Monday), StartLocal and EndLocal are
// HH:MM, TimeZone is nil when absent (the rule follows users.time_zone) and Enabled the switch.
type RuleInput struct {
	Weekdays             int
	StartLocal, EndLocal string
	TimeZone             *string
	Enabled              bool
}

// ValidateRuleInput checks the rules of 13 section 3.4 that need no I/O: weekdays from 1 to 127,
// start and end as ParseLocalTime reads them (start equal to end is allowed and means 24 hours
// from start), and a present time zone that is neither empty nor "Local". Whether a zone name
// loads is checked by the caller, because this package loads no zone.
//
// Errors: validation_failed with one field error per broken field: /weekdays out_of_range,
// /start_local or /end_local invalid_format, /time_zone unknown_time_zone.
//
// Example:
//
//	err := domain.ValidateRuleInput(domain.RuleInput{Weekdays: 31, StartLocal: "22:00", EndLocal: "07:00", Enabled: true})
func ValidateRuleInput(in RuleInput) error {
	var fields []errs.FieldError
	if in.Weekdays < 1 || in.Weekdays > allWeekdays {
		fields = append(fields, errs.FieldError{Pointer: "/weekdays", Code: FieldOutOfRange, Detail: "weekdays is a bit mask from 1 to 127"})
	}
	for _, f := range []struct{ ptr, v string }{{"/start_local", in.StartLocal}, {"/end_local", in.EndLocal}} {
		if _, err := ParseLocalTime(f.v); err != nil {
			fields = append(fields, errs.FieldError{Pointer: f.ptr, Code: FieldInvalidFormat, Detail: "a local time is HH:MM from 00:00 to 23:59"})
		}
	}
	if in.TimeZone != nil && (*in.TimeZone == "" || *in.TimeZone == localZoneName) {
		fields = append(fields, UnknownTimeZone())
	}
	if len(fields) > 0 {
		return errs.New(errs.CodeValidationFailed, "the quiet-hour rule is invalid").WithFields(fields...)
	}
	return nil
}

// UnknownTimeZone is the field error of a time_zone that is not an IANA zone name.
func UnknownTimeZone() errs.FieldError {
	return errs.FieldError{Pointer: "/time_zone", Code: FieldUnknownTimeZone, Detail: "time_zone is not an IANA time zone name"}
}

// RuleUnchanged reports whether writing in over the stored rule changes nothing: every field
// equal, an absent zone equal only to an absent zone. A put without change answers without event
// and keeps the version (brief M3-18 note 9 d).
func RuleUnchanged(stored, in RuleInput) bool {
	sameZone := (stored.TimeZone == nil) == (in.TimeZone == nil) && (stored.TimeZone == nil || *stored.TimeZone == *in.TimeZone)
	return sameZone && stored.Weekdays == in.Weekdays && stored.StartLocal == in.StartLocal && stored.EndLocal == in.EndLocal &&
		stored.Enabled == in.Enabled
}
