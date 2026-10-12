// SPDX-License-Identifier: Apache-2.0

package domain_test

import (
	"testing"
	"time"

	// The tests load Europe/Berlin from the zone database embedded in the Go toolchain, so they
	// pass the same way on hosts without a zone database or with an older one.
	_ "time/tzdata"

	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
	"github.com/OWNER/bothy/services/server/internal/kernel/ids"
	"github.com/OWNER/bothy/services/server/internal/notify/domain"
)

// The recipient of every hint in these tests is Alex. Sam wrote the message or started the run
// in the "not the recipient" cases; Robin receives the admin hints. Alex's devices are named by
// platform: phone (ios), tablet (android), desktop (macos), laptop (windows), console (web).
var (
	threadID      = testID(0x10)
	otherThreadID = testID(0x11)

	phoneID   = testID(0x20)
	tabletID  = testID(0x21)
	desktopID = testID(0x22)
	laptopID  = testID(0x23)
	consoleID = testID(0x24)

	// workstationID is a second windows device, used where a desktop must be active while the
	// laptop is not connected.
	workstationID = testID(0x25)

	phoneReg   = testID(0x30)
	tabletReg  = testID(0x31)
	desktopReg = testID(0x32)
	laptopReg  = testID(0x33)

	// weekdayNoon is Wednesday 2026-10-21, 12:00 in Berlin: outside every quiet-hour rule of the
	// tests.
	weekdayNoon = time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)

	// fixturePolicy holds fixture durations, not the configuration defaults.
	fixturePolicy = domain.Policy{CoalesceWindow: time.Minute, MobileDelay: 30 * time.Second}
)

// testID returns a non-zero ID whose last byte is n.
func testID(n byte) ids.ID {
	var id ids.ID
	id[6] = 0x70
	id[8] = 0x80
	id[15] = n
	return id
}

// berlin loads Europe/Berlin from the embedded zone database.
func berlin(t testing.TB) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("load Europe/Berlin: %v", err)
	}
	return loc
}

// nightRule is quiet hours every day from 22:00 to 07:00 in Berlin.
func nightRule(t testing.TB) domain.QuietRule {
	t.Helper()
	return domain.QuietRule{Weekdays: 127, Start: 22 * 60, End: 7 * 60, Location: berlin(t), Enabled: true}
}

func phone() domain.Device {
	return domain.Device{ID: phoneID, Platform: domain.PlatformIOS, Registrations: []ids.ID{phoneReg}}
}

func tablet() domain.Device {
	return domain.Device{ID: tabletID, Platform: domain.PlatformAndroid, Registrations: []ids.ID{tabletReg}}
}

func desktop() domain.Device {
	return domain.Device{ID: desktopID, Platform: domain.PlatformMacOS}
}

func laptop() domain.Device {
	return domain.Device{ID: laptopID, Platform: domain.PlatformWindows}
}

func console() domain.Device {
	return domain.Device{ID: consoleID, Platform: domain.PlatformWeb}
}

// connected returns d with a live connection focused on the given threads.
func connected(d domain.Device, idle bool, focus ...ids.ID) domain.Device {
	d.Connection = &domain.Connection{Focus: focus, Idle: idle}
	return d
}

// facts returns a hint of the given kind for Alex that a correct decision sends at once, without
// quiet hours, to the phone by push: Alex started the run, did not write the message, may read
// the thread and has the instance default all.
func facts(kind domain.Kind) domain.RecipientFacts {
	in := domain.RecipientFacts{
		Kind:       kind,
		Now:        weekdayNoon,
		Allowed:    true,
		RunStarter: true,
		Switches:   domain.Switches{AdminEnabled: true},
		Devices:    []domain.Device{phone(), desktop()},
		Policy:     fixturePolicy,
	}
	if kind.HasThread() {
		in.ThreadID = threadID
		in.Level = domain.LevelFacts{InstanceDefault: domain.LevelAll, DirectMessage: kind == domain.KindDM}
	}
	return in
}

// threadRow is an own notification_preferences row with the given level.
func threadRow(level domain.Level, mutedUntil time.Time) *domain.Preference {
	return &domain.Preference{Level: level, MutedUntil: mutedUntil}
}

// mustDecide runs Decide and fails the test on an error.
func mustDecide(t *testing.T, in domain.RecipientFacts) domain.Decision {
	t.Helper()
	d, err := domain.Decide(in)
	if err != nil {
		t.Fatalf("Decide(%s): %v", in.Kind, err)
	}
	return d
}

// assertInternal fails unless err is an internal_error of the kernel.
func assertInternal(t *testing.T, name string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: no error", name)
		return
	}
	e, ok := errs.As(err)
	if !ok || e.Code != errs.CodeInternalError {
		t.Errorf("%s: error %v is not internal_error", name, err)
	}
}

// threadKinds and otherKinds split Kinds() by HasThread as 13 2 does.
var (
	threadKinds = []domain.Kind{
		domain.KindMessage, domain.KindMention, domain.KindDM,
		domain.KindRunDone, domain.KindRunFailed, domain.KindInputNeeded,
	}
	otherKinds = []domain.Kind{domain.KindSecurity, domain.KindAdmin}
)
