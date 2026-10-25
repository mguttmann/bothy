// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
)

// AddCalendarMonths returns t plus n calendar months in UTC (docs/spec/20-privacy.md section 5.1,
// R-DP-05): the same day of the month n months later at the same time of day; a day that does not
// exist in the target month moves to the last day of that month (31 January plus one month is 28
// or 29 February). The rule follows Regulation (EEC, Euratom) No 1182/71 Art. 3(2)(c), which
// computes periods of EU law in months that way; time.AddDate would roll over into the next
// month instead. n must not be negative.
//
// Example:
//
//	AddCalendarMonths(time.Date(2026, 1, 31, 9, 0, 0, 0, time.UTC), 1) // 2026-02-28 09:00 UTC
func AddCalendarMonths(t time.Time, n int) time.Time {
	t = t.UTC()
	y, m, d := t.Date()
	target := time.Date(y, m+time.Month(n), 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	if last := daysIn(target.Year(), target.Month()); d > last {
		d = last
	}
	return time.Date(target.Year(), target.Month(), d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

// daysIn returns the number of days of month m of year y.
func daysIn(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// DueAt returns the statutory deadline of a request received at receipt: receipt plus one
// calendar month (Art. 12(3) GDPR). It is no configuration value (R-DP-05).
func DueAt(receipt time.Time) time.Time { return AddCalendarMonths(receipt, 1) }

// Extend returns the deadline of r after the extension by months (1 or 2) of Art. 12(3) GDPR,
// decided at now. The extension is allowed once and only before the original deadline, so the
// person learns of it within the first month.
//
// Errors: 422 validation_failed at /extend_months for a value other than 1 or 2; 409
// conflict (reason extension_not_allowed) once the deadline has passed or after an earlier
// extension.
func Extend(r Request, months int, now time.Time) (time.Time, error) {
	if months != 1 && months != 2 {
		return time.Time{}, fieldError("/extend_months", "invalid", "the extension is one or two months")
	}
	if r.Progress.ExtendedMonths != 0 || !now.Before(r.DueAt) {
		return time.Time{}, errs.New(errs.CodeConflict, "the deadline can be extended once and only before it expires").
			WithReason("extension_not_allowed")
	}
	return AddCalendarMonths(r.DueAt, months), nil
}

// ReminderDue reports whether the reminder gdpr.request_due is due for r at now with the lead
// privacy.request_reminder_lead: the request is open, no reminder went out yet and due_at minus
// lead has passed.
func ReminderDue(r Request, lead time.Duration, now time.Time) bool {
	return r.State.Open() && !r.Progress.ReminderSent && !now.Before(r.DueAt.Add(-lead))
}
