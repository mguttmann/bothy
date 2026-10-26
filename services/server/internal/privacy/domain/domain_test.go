// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/privacy/domain"
)

func utc(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 30, 0, 0, time.UTC) }

// TestDueAtCalendarMonth: receipt plus one calendar month in UTC; a day missing in the target
// month moves to its last day (31 January, 29 February, end of year).
func TestDueAtCalendarMonth(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want time.Time }{
		{utc(2026, time.January, 31, 9), utc(2026, time.February, 28, 9)},
		{utc(2028, time.January, 31, 9), utc(2028, time.February, 29, 9)},
		{utc(2028, time.February, 29, 9), utc(2028, time.March, 29, 9)},
		{utc(2026, time.March, 31, 23), utc(2026, time.April, 30, 23)},
		{utc(2026, time.December, 31, 22), utc(2027, time.January, 31, 22)},
		{utc(2026, time.December, 15, 0), utc(2027, time.January, 15, 0)},
		{utc(2026, time.October, 10, 12), utc(2026, time.November, 10, 12)},
	} {
		if got := domain.DueAt(c.in); !got.Equal(c.want) {
			t.Errorf("DueAt(%s) = %s, want %s", c.in, got, c.want)
		}
	}
	// A receipt in another zone is computed in UTC: 1 February 00:30 in UTC+2 is 31 January UTC.
	in := time.Date(2026, time.February, 1, 0, 30, 0, 0, time.FixedZone("x", 2*3600))
	if got := domain.DueAt(in); !got.Equal(time.Date(2026, time.February, 28, 22, 30, 0, 0, time.UTC)) {
		t.Errorf("DueAt in UTC+2 = %s", got)
	}
	if got := domain.AddCalendarMonths(utc(2026, time.August, 31, 1), 2); !got.Equal(utc(2026, time.October, 31, 1)) {
		t.Errorf("two months = %s", got)
	}
}

// TestStateMachine: the transitions of 03; final states move nowhere.
func TestStateMachine(t *testing.T) {
	t.Parallel()
	allowed := map[[2]domain.State]bool{
		{domain.StateRequested, domain.StateApproved}: true, {domain.StateRequested, domain.StateRejected}: true,
		{domain.StateRequested, domain.StateCancelled}: true, {domain.StateApproved, domain.StateRunning}: true,
		{domain.StateApproved, domain.StateCompleted}: true, {domain.StateApproved, domain.StateCancelled}: true,
		{domain.StateRunning, domain.StateCompleted}: true, {domain.StateRunning, domain.StateFailed}: true,
	}
	states := []domain.State{domain.StateRequested, domain.StateApproved, domain.StateRunning, domain.StateCompleted,
		domain.StateFailed, domain.StateRejected, domain.StateCancelled}
	for _, from := range states {
		if !from.Valid() {
			t.Fatalf("%s not valid", from)
		}
		for _, to := range states {
			got, err := domain.Move(from, to)
			if allowed[[2]domain.State{from, to}] {
				if err != nil || got != to {
					t.Errorf("%s -> %s refused: %v", from, to, err)
				}
				continue
			}
			if err == nil || codeOf(err) != errs.CodeInvalidStateTransition {
				t.Errorf("%s -> %s allowed or wrong error %v", from, to, err)
			}
		}
	}
	if domain.State("x").Valid() || !domain.StateRunning.Open() || domain.StateFailed.Open() {
		t.Fatal("Valid or Open is wrong")
	}
}

// TestAutoApproved: only the export of one's own data without privacy.export_requires_approval.
func TestAutoApproved(t *testing.T) {
	t.Parallel()
	alex, robin := ids.New(), ids.New()
	if !domain.AutoApproved(domain.KindExport, alex, alex, false) {
		t.Fatal("own export not approved")
	}
	for name, ok := range map[string]bool{
		"needs approval": domain.AutoApproved(domain.KindExport, alex, alex, true),
		"for another":    domain.AutoApproved(domain.KindExport, alex, robin, false),
		"deletion":       domain.AutoApproved(domain.KindDeletion, alex, alex, false),
		"rectification":  domain.AutoApproved(domain.KindRectification, alex, alex, false),
	} {
		if ok {
			t.Errorf("%s: approved by the system", name)
		}
	}
}

// TestExtend: once, one or two months, only before the original deadline.
func TestExtend(t *testing.T) {
	t.Parallel()
	r := domain.Request{DueAt: utc(2026, time.January, 31, 9)}
	got, err := domain.Extend(r, 2, utc(2026, time.January, 10, 9))
	if err != nil || !got.Equal(utc(2026, time.March, 31, 9)) {
		t.Fatalf("Extend = %s, %v", got, err)
	}
	if _, err := domain.Extend(r, 3, utc(2026, time.January, 10, 9)); codeOf(err) != errs.CodeValidationFailed {
		t.Fatalf("three months: %v", err)
	}
	if _, err := domain.Extend(r, 1, r.DueAt); codeOf(err) != errs.CodeConflict {
		t.Fatalf("at the deadline: %v", err)
	}
	r.Progress.ExtendedMonths = 1
	if _, err := domain.Extend(r, 1, utc(2026, time.January, 10, 9)); codeOf(err) != errs.CodeConflict {
		t.Fatalf("second extension: %v", err)
	}
}

// TestReminderDue: open, not sent, lead reached.
func TestReminderDue(t *testing.T) {
	t.Parallel()
	due := utc(2026, time.March, 1, 0)
	lead := 72 * time.Hour
	r := domain.Request{State: domain.StateRequested, DueAt: due}
	if domain.ReminderDue(r, lead, due.Add(-lead-time.Second)) || !domain.ReminderDue(r, lead, due.Add(-lead)) {
		t.Fatal("lead boundary is wrong")
	}
	r.Progress.ReminderSent = true
	if domain.ReminderDue(r, lead, due) {
		t.Fatal("sent twice")
	}
	r.Progress.ReminderSent, r.State = false, domain.StateCompleted
	if domain.ReminderDue(r, lead, due) {
		t.Fatal("reminder for a closed request")
	}
}

// TestCheckReason: required, bounded, no control characters.
func TestCheckReason(t *testing.T) {
	t.Parallel()
	if err := domain.CheckReason("Request by the person", "/reason"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "  \n", "a\u202eb", strings.Repeat("x", domain.MaxReasonRunes+1)} {
		var e *errs.Error
		if err := domain.CheckReason(bad, "/reason"); !errors.As(err, &e) || e.Code != errs.CodeValidationFailed {
			t.Errorf("CheckReason(%q) = %v", bad, err)
		}
	}
}

// TestPackage: name, README language, progress round trip.
func TestPackage(t *testing.T) {
	t.Parallel()
	id := ids.MustParse("0192f4a0-7b1c-7277-8379-95fe87046d8a")
	if got := domain.PackageName(id, utc(2026, time.October, 10, 23)); got != "export-0192f4a0-7b1c-7277-8379-95fe87046d8a-2026-10-10.zip" {
		t.Fatal(got)
	}
	if domain.ReadmeLanguage("de-AT") != "de" || domain.ReadmeLanguage("en-GB") != "en" || domain.ReadmeLanguage("") != "en" ||
		domain.ReadmeLanguage("dexx") != "en" {
		t.Fatal("ReadmeLanguage is wrong")
	}
	if !strings.Contains(domain.Readme("de"), "manifest.json") || !strings.Contains(domain.Readme("en"), "audit.jsonl") {
		t.Fatal("README misses a file")
	}
	p := domain.Progress{LastStep: 3, ReminderSent: true, Content: "delete"}
	b, err := domain.MarshalProgress(p)
	if err != nil {
		t.Fatal(err)
	}
	back, err := domain.UnmarshalProgress(b)
	if err != nil || back != p {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
	if empty, err := domain.UnmarshalProgress([]byte("{}")); err != nil || empty != (domain.Progress{}) {
		t.Fatal("empty progress")
	}
}

// codeOf returns the code of a kernel/errs error, "" for any other.
func codeOf(err error) errs.Code {
	if e, ok := errs.As(err); ok {
		return e.Code
	}
	return ""
}
